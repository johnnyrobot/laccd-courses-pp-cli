// Copyright 2026 johnnyrobot and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored novel command (standalone file, regen-safe).
// pp:data-source local

package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/johnnyrobot/laccd-courses-pp-cli/internal/store"

	"github.com/spf13/cobra"
)

// compareEntry is one college's version of the compared course.
type compareEntry struct {
	College   string  `json:"college"`
	Tenant    string  `json:"tenant"`
	ID        int64   `json:"id"`
	Code      string  `json:"code"`
	Name      string  `json:"name"`
	MinUnits  float64 `json:"min_units"`
	MaxUnits  float64 `json:"max_units"`
	Transfer  string  `json:"transfer_status"`
	Prereqs   int     `json:"prerequisites"`
	signature string
}

type compareView struct {
	Subject   string         `json:"subject"`
	Number    string         `json:"number"`
	Found     int            `json:"colleges_found"`
	Identical bool           `json:"identical"`
	Summary   string         `json:"summary"`
	Courses   []compareEntry `json:"courses"`
}

func newNovelCoursesCompareCmd(flags *rootFlags) *cobra.Command {
	var flagDiffOnly bool
	var dbPath string

	cmd := &cobra.Command{
		Use:   "compare <subject> <number>",
		Short: "Compare one course across LACCD colleges — units, transfer status, prerequisites",
		Long: "Align a single subject+number across every LACCD college that has it synced locally:\n" +
			"units, transfer status, and prerequisite count side by side — the rows that break a\n" +
			"transfer. --diff-only suppresses the rows entirely when every college matches, so a\n" +
			"scripted check stays quiet until something actually diverges; when they differ you\n" +
			"still get every college back, not just the odd ones out.\n\n" +
			"Reads the local mirror. Sync the subject first, e.g.\n" +
			"  laccd-courses-pp-cli sync --resources courses --resource-param courses:query=ACCTG\n" +
			"For free-text discovery use 'courses search'; for one course's full record use 'courses get'.",
		Example: "  laccd-courses-pp-cli courses compare ACCTG 001\n  laccd-courses-pp-cli courses compare ACCTG 001 --diff-only --agent",
		Annotations: map[string]string{
			"mcp:read-only":  "true",
			"pp:data-source": "local",
			"pp:happy-args":  "subject=ENGLISH;number=101",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if len(args) < 2 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("compare needs a subject and a course number, e.g. 'compare ENGLISH 101'"))
			}
			if dryRunOK(flags) {
				return nil
			}
			subject := strings.ToUpper(strings.TrimSpace(args[0]))
			number := strings.TrimSpace(args[1])

			if dbPath == "" {
				dbPath = defaultDBPath("laccd-courses-pp-cli")
			}
			if _, statErr := os.Stat(dbPath); os.IsNotExist(statErr) {
				printSyncSubjectHint(cmd, subject, dbPath)
				if flags.asJSON || flags.agent {
					return printJSONFiltered(cmd.OutOrStdout(), compareView{Subject: subject, Number: number, Courses: []compareEntry{}}, flags)
				}
				return nil
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			db, err := store.OpenWithContext(ctx, dbPath)
			if err != nil {
				return fmt.Errorf("opening local store: %w", err)
			}
			defer db.Close()

			rows, err := db.DB().QueryContext(ctx, `SELECT data FROM courses WHERE subject = ? COLLATE NOCASE`, subject)
			if err != nil {
				return fmt.Errorf("querying courses: %w", err)
			}
			defer rows.Close()
			var records []json.RawMessage
			for rows.Next() {
				var data []byte
				if err := rows.Scan(&data); err != nil {
					continue
				}
				records = append(records, json.RawMessage(append([]byte(nil), data...)))
			}
			if err := rows.Err(); err != nil {
				return fmt.Errorf("reading courses: %w", err)
			}
			records = filterBySubjectNumber(records, subject, number)

			view := buildCompareView(subject, number, records)
			if view.Found == 0 {
				printSyncSubjectHint(cmd, subject, dbPath)
			}
			if flagDiffOnly && view.Identical {
				view.Courses = nil
			}

			if wantsHumanTable(cmd.OutOrStdout(), flags) {
				if view.Found == 0 {
					return nil
				}
				fmt.Fprintf(cmd.OutOrStdout(), "%s\n\n", view.Summary)
				if flagDiffOnly && view.Identical {
					return nil
				}
				items := make([]map[string]any, 0, len(view.Courses))
				for _, c := range view.Courses {
					items = append(items, map[string]any{
						"college": c.College, "code": c.Code, "units": unitsLabel(c.MinUnits, c.MaxUnits),
						"transfer": c.Transfer, "prerequisites": c.Prereqs,
					})
				}
				return printAutoTable(cmd.OutOrStdout(), items)
			}
			return printJSONFiltered(cmd.OutOrStdout(), view, flags)
		},
	}
	cmd.Flags().BoolVar(&flagDiffOnly, "diff-only", false, "Collapse identical colleges; show output only when they differ")
	cmd.Flags().StringVar(&dbPath, "db", "", "Local store path (default: per-user data dir)")
	return cmd
}

func printSyncSubjectHint(cmd *cobra.Command, subject, dbPath string) {
	fmt.Fprintf(cmd.ErrOrStderr(),
		"no synced %s courses to compare.\nsync the subject first:\n  laccd-courses-pp-cli sync --resources courses --resource-param courses:query=%s --db %s\n",
		subject, subject, dbPath)
}

// filterBySubjectNumber keeps only courses whose subject+number match the
// request (number compared with leading-zero normalization).
func filterBySubjectNumber(courses []json.RawMessage, subject, number string) []json.RawMessage {
	wantNum := normalizeCourseNumber(number)
	var out []json.RawMessage
	for _, raw := range courses {
		var c struct {
			Subject string `json:"subject"`
			Number  string `json:"number"`
		}
		if json.Unmarshal(raw, &c) != nil {
			continue
		}
		if !strings.EqualFold(strings.TrimSpace(c.Subject), subject) {
			continue
		}
		if normalizeCourseNumber(c.Number) != wantNum {
			continue
		}
		out = append(out, raw)
	}
	return out
}

func buildCompareView(subject, number string, records []json.RawMessage) compareView {
	byTenant := map[string]compareEntry{}
	for _, raw := range records {
		var base struct {
			ID     int64  `json:"id"`
			Code   string `json:"code"`
			Name   string `json:"name"`
			Tenant string `json:"tenant"`
		}
		if json.Unmarshal(raw, &base) != nil {
			continue
		}
		entry := compareEntry{
			College: collegeCodeForHost(base.Tenant),
			Tenant:  base.Tenant,
			ID:      base.ID,
			Code:    base.Code,
			Name:    base.Name,
		}
		if fci, ok := decodeFullCourseInfo(raw); ok {
			u := extractUnits(fci)
			entry.MinUnits, entry.MaxUnits = u.Min, u.Max
			entry.Transfer = extractTransferStatus(fci)
			entry.Prereqs = len(parseRequisites(fci))
		}
		entry.signature = fmt.Sprintf("%g|%g|%s|%d", entry.MinUnits, entry.MaxUnits, strings.ToUpper(entry.Transfer), entry.Prereqs)
		// Keep the highest-id (most recent) version per college.
		if prev, ok := byTenant[base.Tenant]; !ok || entry.ID > prev.ID {
			byTenant[base.Tenant] = entry
		}
	}

	view := compareView{Subject: subject, Number: number, Found: len(byTenant), Courses: []compareEntry{}}
	sigs := map[string]struct{}{}
	for _, e := range byTenant {
		view.Courses = append(view.Courses, e)
		sigs[e.signature] = struct{}{}
	}
	sort.Slice(view.Courses, func(i, j int) bool { return view.Courses[i].College < view.Courses[j].College })
	view.Identical = view.Found > 1 && len(sigs) == 1
	switch {
	case view.Found == 0:
		view.Summary = fmt.Sprintf("No synced LACCD college has %s %s.", subject, number)
	case view.Found == 1:
		e := view.Courses[0]
		view.Summary = fmt.Sprintf("%s %s found at 1 college (%s): units %s, transfer %q, %d prerequisite block(s).",
			subject, number, e.College, unitsLabel(e.MinUnits, e.MaxUnits), e.Transfer, e.Prereqs)
	case view.Identical:
		e := view.Courses[0]
		view.Summary = fmt.Sprintf("All %d colleges match %s %s: units %s, transfer %q, %d prerequisite block(s).",
			view.Found, subject, number, unitsLabel(e.MinUnits, e.MaxUnits), e.Transfer, e.Prereqs)
	default:
		view.Summary = fmt.Sprintf("%s %s differs across %d colleges (%d distinct unit/transfer/prereq profiles).",
			subject, number, view.Found, len(sigs))
	}
	return view
}

func unitsLabel(minU, maxU float64) string {
	if minU == maxU {
		return fmt.Sprintf("%g", minU)
	}
	return fmt.Sprintf("%g-%g", minU, maxU)
}
