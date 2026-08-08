// Copyright 2026 johnnyrobot and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored novel command (standalone file, regen-safe).
// pp:data-source auto

package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/johnnyrobot/laccd-courses-pp-cli/internal/store"

	"github.com/spf13/cobra"
)

type prereqView struct {
	CourseID   string           `json:"course_id"`
	Code       string           `json:"code"`
	Name       string           `json:"name"`
	College    string           `json:"college"`
	Count      int              `json:"count"`
	Requisites []requisiteEntry `json:"requisites"`
}

func newNovelCoursesPrereqsCmd(flags *rootFlags) *cobra.Command {
	var dbPath string

	cmd := &cobra.Command{
		Use:   "prereqs <course_id>",
		Short: "Print a course's prerequisite / advisory / corequisite chain",
		Long: "Walk a course's structured requisite blocks (from its Outline of Record) and print\n" +
			"every prerequisite, advisory, and corequisite with its course code and title. Takes a\n" +
			"numeric course id (the 'id' from a search result). Fetches live by default and falls\n" +
			"back to the local mirror when offline.",
		Example: "  laccd-courses-pp-cli courses prereqs 40830\n  laccd-courses-pp-cli courses prereqs 40830 --agent",
		Annotations: map[string]string{
			"mcp:read-only":  "true",
			"pp:data-source": "auto",
			"pp:happy-args":  "course_id=40830",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			if len(args) < 1 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("prereqs needs a course id, e.g. 'prereqs 40830'"))
			}
			courseID := strings.TrimSpace(args[0])
			// Validate numeric id before it reaches the URL path or store query —
			// prevents path traversal (e.g. '../configurations') into other endpoints.
			if _, perr := strconv.ParseInt(courseID, 10, 64); perr != nil {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("course id must be numeric, e.g. 'prereqs 40830'"))
			}

			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			var data json.RawMessage
			var err error
			if flags.dataSource == "local" {
				data, err = prereqFromStore(ctx, cmd, dbPath, courseID)
			} else {
				data, err = prereqFromLive(ctx, courseID, flags)
				if err != nil && isNetworkError(err) {
					fmt.Fprintln(cmd.ErrOrStderr(), "live API unreachable; falling back to local mirror")
					data, err = prereqFromStore(ctx, cmd, dbPath, courseID)
				}
			}
			if err != nil {
				return err
			}
			if len(data) == 0 {
				fmt.Fprintf(cmd.ErrOrStderr(), "course %s not found\n", courseID)
				if flags.asJSON || flags.agent {
					fmt.Fprintln(cmd.OutOrStdout(), "{}")
				}
				return nil
			}

			var base struct {
				Code   string `json:"code"`
				Name   string `json:"name"`
				Tenant string `json:"tenant"`
			}
			_ = json.Unmarshal(data, &base)
			view := prereqView{CourseID: courseID, Code: base.Code, Name: base.Name, College: collegeCodeForHost(base.Tenant), Requisites: []requisiteEntry{}}
			if fci, ok := decodeFullCourseInfo(data); ok {
				if reqs := parseRequisites(fci); len(reqs) > 0 {
					view.Requisites = reqs
				}
			}
			view.Count = len(view.Requisites)

			if wantsHumanTable(cmd.OutOrStdout(), flags) {
				fmt.Fprintf(cmd.OutOrStdout(), "%s — %s (%s)\n", view.Code, view.Name, view.College)
				if view.Count == 0 {
					fmt.Fprintln(cmd.OutOrStdout(), "No prerequisites, advisories, or corequisites on record.")
					return nil
				}
				items := make([]map[string]any, 0, view.Count)
				for _, r := range view.Requisites {
					kind := r.Type
					if kind == "" {
						kind = "Requisite"
					}
					items = append(items, map[string]any{"type": kind, "code": r.Code, "title": r.Title})
				}
				return printAutoTable(cmd.OutOrStdout(), items)
			}
			return printJSONFiltered(cmd.OutOrStdout(), view, flags)
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Local store path (used with --data-source local or offline fallback)")
	return cmd
}

func prereqFromLive(ctx context.Context, courseID string, flags *rootFlags) (json.RawMessage, error) {
	c, err := flags.newClient()
	if err != nil {
		return nil, err
	}
	data, err := c.Get(ctx, "/public/courses/"+courseID, nil)
	if err != nil {
		return nil, err
	}
	return data, nil
}

func prereqFromStore(ctx context.Context, cmd *cobra.Command, dbPath, courseID string) (json.RawMessage, error) {
	if dbPath == "" {
		dbPath = defaultDBPath("laccd-courses-pp-cli")
	}
	if _, statErr := os.Stat(dbPath); os.IsNotExist(statErr) {
		fmt.Fprintf(cmd.ErrOrStderr(), "no local mirror at %s\nrun: laccd-courses-pp-cli sync --resources courses --db %s\n", dbPath, dbPath)
		return nil, nil
	}
	db, err := store.OpenWithContext(ctx, dbPath)
	if err != nil {
		return nil, fmt.Errorf("opening local store: %w", err)
	}
	defer db.Close()
	row := db.DB().QueryRowContext(ctx, `SELECT data FROM courses WHERE id = ?`, courseID)
	var data []byte
	if err := row.Scan(&data); err != nil {
		return nil, nil // not found in store
	}
	return json.RawMessage(data), nil
}
