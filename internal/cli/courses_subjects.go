// Copyright 2026 johnnyrobot and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored novel command (standalone file, regen-safe).
// pp:data-source local

package cli

import (
	"context"
	"database/sql"
	"fmt"
	"os"

	"laccd-courses-pp-cli/internal/store"

	"github.com/spf13/cobra"
)

type subjectRow struct {
	Subject  string `json:"subject"`
	Courses  int    `json:"courses"`
	Colleges int    `json:"colleges"`
}

type subjectCourseRow struct {
	Code    string `json:"code"`
	Name    string `json:"name"`
	Number  string `json:"number"`
	College string `json:"college"`
	Tenant  string `json:"tenant"`
}

func newNovelCoursesSubjectsCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	var tenant string
	var limit int

	cmd := &cobra.Command{
		Use:   "subjects [subject]",
		Short: "List subject prefixes with course counts, or all courses in one subject",
		Long: "Browse the synced LACCD catalog by subject.\n" +
			"With no argument, lists every distinct subject prefix and its district-wide course count.\n" +
			"With a subject (e.g. 'subjects ENGL'), lists all synced courses in that subject across colleges.\n" +
			"Reads the local mirror — run 'sync --resources courses' or a 'courses search' first.",
		Example: "  laccd-courses-pp-cli courses subjects\n  laccd-courses-pp-cli courses subjects ENGL --tenant ELAC",
		Annotations: map[string]string{
			"mcp:read-only":  "true",
			"pp:data-source": "local",
			// Any positional is a valid subject name; an unknown subject yields
			// an empty result (exit 0), not an error.
			"pp:no-error-path-probe": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			if dbPath == "" {
				dbPath = defaultDBPath("laccd-courses-pp-cli")
			}
			host := resolveTenant(tenant)
			if _, statErr := os.Stat(dbPath); os.IsNotExist(statErr) {
				fmt.Fprintf(cmd.ErrOrStderr(), "no local mirror at %s\nrun: laccd-courses-pp-cli sync --resources courses --db %s\n", dbPath, dbPath)
				if flags.asJSON || flags.agent {
					fmt.Fprintln(cmd.OutOrStdout(), "[]")
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

			if len(args) == 0 {
				return runSubjectsList(ctx, cmd, db.DB(), host, flags)
			}
			return runSubjectCourses(ctx, cmd, db.DB(), args[0], host, limit, flags)
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Local store path (default: per-user data dir)")
	cmd.Flags().StringVar(&tenant, "tenant", "", "Limit to one college (code ELAC, or host elac.elumenapp.com)")
	cmd.Flags().IntVar(&limit, "limit", 200, "Maximum courses to list when a subject is given")
	return cmd
}

func runSubjectsList(ctx context.Context, cmd *cobra.Command, db *sql.DB, host string, flags *rootFlags) error {
	query := `SELECT subject, COUNT(*) AS n, COUNT(DISTINCT tenant) AS colleges
	          FROM courses WHERE subject IS NOT NULL AND subject != ''`
	var rowsArgs []any
	if host != "" {
		query += " AND tenant = ?"
		rowsArgs = append(rowsArgs, host)
	}
	query += " GROUP BY subject ORDER BY n DESC, subject ASC"
	rows, err := db.QueryContext(ctx, query, rowsArgs...)
	if err != nil {
		return fmt.Errorf("querying subjects: %w", err)
	}
	defer rows.Close()
	out := make([]subjectRow, 0)
	for rows.Next() {
		var subj sql.NullString
		var n, colleges sql.NullInt64
		if err := rows.Scan(&subj, &n, &colleges); err != nil {
			continue
		}
		out = append(out, subjectRow{Subject: subj.String, Courses: int(n.Int64), Colleges: int(colleges.Int64)})
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("reading subjects: %w", err)
	}
	if wantsHumanTable(cmd.OutOrStdout(), flags) {
		if len(out) == 0 {
			fmt.Fprintln(cmd.OutOrStdout(), "no subjects in local mirror yet — run: laccd-courses-pp-cli sync --resources courses")
			return nil
		}
		items := make([]map[string]any, 0, len(out))
		for _, r := range out {
			items = append(items, map[string]any{"subject": r.Subject, "courses": r.Courses, "colleges": r.Colleges})
		}
		return printAutoTable(cmd.OutOrStdout(), items)
	}
	return printJSONFiltered(cmd.OutOrStdout(), out, flags)
}

func runSubjectCourses(ctx context.Context, cmd *cobra.Command, db *sql.DB, subject, host string, limit int, flags *rootFlags) error {
	query := `SELECT code, name, number, tenant FROM courses WHERE subject = ? COLLATE NOCASE`
	rowsArgs := []any{subject}
	if host != "" {
		query += " AND tenant = ?"
		rowsArgs = append(rowsArgs, host)
	}
	query += " ORDER BY tenant, number LIMIT ?"
	rowsArgs = append(rowsArgs, limit)
	rows, err := db.QueryContext(ctx, query, rowsArgs...)
	if err != nil {
		return fmt.Errorf("querying courses: %w", err)
	}
	defer rows.Close()
	out := make([]subjectCourseRow, 0)
	for rows.Next() {
		var code, name, number, tenant sql.NullString
		if err := rows.Scan(&code, &name, &number, &tenant); err != nil {
			continue
		}
		out = append(out, subjectCourseRow{
			Code:    code.String,
			Name:    name.String,
			Number:  number.String,
			Tenant:  tenant.String,
			College: collegeCodeForHost(tenant.String),
		})
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("reading courses: %w", err)
	}
	if wantsHumanTable(cmd.OutOrStdout(), flags) {
		if len(out) == 0 {
			fmt.Fprintf(cmd.OutOrStdout(), "no synced courses for subject %q — run: laccd-courses-pp-cli sync --resources courses\n", subject)
			return nil
		}
		items := make([]map[string]any, 0, len(out))
		for _, r := range out {
			items = append(items, map[string]any{"code": r.Code, "name": r.Name, "college": r.College})
		}
		return printAutoTable(cmd.OutOrStdout(), items)
	}
	return printJSONFiltered(cmd.OutOrStdout(), out, flags)
}
