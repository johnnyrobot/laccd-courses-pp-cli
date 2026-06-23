// Copyright 2026 johnnyrobot and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored novel command (standalone file, regen-safe).
// pp:data-source local

package cli

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"

	"laccd-courses-pp-cli/internal/store"

	"github.com/spf13/cobra"
)

type collegeStat struct {
	College      string `json:"college"`
	Tenant       string `json:"tenant"`
	Courses      int    `json:"courses"`
	Subjects     int    `json:"subjects"`
	Transferable int    `json:"transferable"`
}

type statsView struct {
	TotalCourses int           `json:"total_courses"`
	Colleges     int           `json:"colleges"`
	Subjects     int           `json:"subjects"`
	PerCollege   []collegeStat `json:"per_college"`
}

func newNovelStatsCmd(flags *rootFlags) *cobra.Command {
	var dbPath string

	cmd := &cobra.Command{
		Use:   "stats",
		Short: "District catalog stats: per-college course counts, subjects, and transferable share",
		Long: "Aggregate the synced LACCD course mirror into per-college counts: total courses,\n" +
			"distinct subjects, and how many courses are transferable (CSU/UC). Reads the local\n" +
			"mirror — run 'sync --resources courses' first. To check mirror completeness instead\n" +
			"of content distribution, use 'coverage'.",
		Example: "  laccd-courses-pp-cli stats\n  laccd-courses-pp-cli stats --agent",
		Annotations: map[string]string{
			"mcp:read-only":  "true",
			"pp:data-source": "local",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 && !flags.asJSON && !flags.agent {
				// stats takes no required input; only show help on a bare
				// interactive invocation, otherwise run.
			}
			if dryRunOK(flags) {
				return nil
			}
			if dbPath == "" {
				dbPath = defaultDBPath("laccd-courses-pp-cli")
			}
			if _, statErr := os.Stat(dbPath); os.IsNotExist(statErr) {
				fmt.Fprintf(cmd.ErrOrStderr(), "no local mirror at %s\nrun: laccd-courses-pp-cli sync --resources courses --db %s\n", dbPath, dbPath)
				if flags.asJSON || flags.agent {
					return printJSONFiltered(cmd.OutOrStdout(), statsView{PerCollege: []collegeStat{}}, flags)
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
			hintIfUnsynced(cmd, db, "courses")

			rows, err := db.DB().QueryContext(ctx, `SELECT tenant, subject, data FROM courses`)
			if err != nil {
				return fmt.Errorf("querying courses: %w", err)
			}
			defer rows.Close()

			perCollege := map[string]*collegeStat{}
			subjectsByCollege := map[string]map[string]struct{}{}
			allSubjects := map[string]struct{}{}
			total := 0
			for rows.Next() {
				var tenant, subject sql.NullString
				var data []byte
				if err := rows.Scan(&tenant, &subject, &data); err != nil {
					continue
				}
				total++
				host := tenant.String
				cs, ok := perCollege[host]
				if !ok {
					cs = &collegeStat{College: collegeCodeForHost(host), Tenant: host}
					perCollege[host] = cs
					subjectsByCollege[host] = map[string]struct{}{}
				}
				cs.Courses++
				if subject.String != "" {
					subjectsByCollege[host][subject.String] = struct{}{}
					allSubjects[subject.String] = struct{}{}
				}
				if courseIsTransferable(json.RawMessage(data)) {
					cs.Transferable++
				}
			}
			if err := rows.Err(); err != nil {
				return fmt.Errorf("reading courses: %w", err)
			}

			view := statsView{TotalCourses: total, Colleges: len(perCollege), Subjects: len(allSubjects)}
			for host, cs := range perCollege {
				cs.Subjects = len(subjectsByCollege[host])
				view.PerCollege = append(view.PerCollege, *cs)
			}
			sort.Slice(view.PerCollege, func(i, j int) bool {
				if view.PerCollege[i].Courses != view.PerCollege[j].Courses {
					return view.PerCollege[i].Courses > view.PerCollege[j].Courses
				}
				return view.PerCollege[i].College < view.PerCollege[j].College
			})

			if wantsHumanTable(cmd.OutOrStdout(), flags) {
				if total == 0 {
					fmt.Fprintln(cmd.OutOrStdout(), "local mirror is empty — run: laccd-courses-pp-cli sync --resources courses")
					return nil
				}
				fmt.Fprintf(cmd.OutOrStdout(), "%d courses across %d colleges, %d distinct subjects\n\n", view.TotalCourses, view.Colleges, view.Subjects)
				items := make([]map[string]any, 0, len(view.PerCollege))
				for _, c := range view.PerCollege {
					items = append(items, map[string]any{"college": c.College, "courses": c.Courses, "subjects": c.Subjects, "transferable": c.Transferable})
				}
				return printAutoTable(cmd.OutOrStdout(), items)
			}
			return printJSONFiltered(cmd.OutOrStdout(), view, flags)
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Local store path (default: per-user data dir)")
	return cmd
}

// courseIsTransferable reports whether a stored course is transferable to a
// 4-year system, based on its Outline-of-Record courseTransferStatus.
func courseIsTransferable(courseData json.RawMessage) bool {
	fci, ok := decodeFullCourseInfo(courseData)
	if !ok {
		return false
	}
	status := strings.ToUpper(extractTransferStatus(fci))
	if status == "" {
		return false
	}
	if strings.Contains(status, "NOT TRANSFER") || status == "NONE" || status == "NO" {
		return false
	}
	return strings.Contains(status, "CSU") || strings.Contains(status, "UC") || strings.Contains(status, "TRANSFER")
}
