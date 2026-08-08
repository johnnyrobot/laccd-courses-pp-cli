// Copyright 2026 johnnyrobot and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored novel command (standalone file, regen-safe).
// pp:data-source auto

package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sync"

	"github.com/johnnyrobot/laccd-courses-pp-cli/internal/cliutil"
	"github.com/johnnyrobot/laccd-courses-pp-cli/internal/store"

	"github.com/spf13/cobra"
)

type coverageRow struct {
	College  string `json:"college"`
	Tenant   string `json:"tenant"`
	Local    int    `json:"local"`
	APITotal int    `json:"api_total"` // -1 when not probed
	Missing  int    `json:"missing"`   // api_total - local (>=0); -1 when api_total unknown
}

type coverageView struct {
	CheckedAPI bool          `json:"checked_api"`
	Note       string        `json:"note,omitempty"`
	Rows       []coverageRow `json:"per_college"`
}

func newNovelCoverageCmd(flags *rootFlags) *cobra.Command {
	var dbPath string

	cmd := &cobra.Command{
		Use:   "coverage",
		Short: "Check how complete the local catalog mirror is per college",
		Long: "Report, per LACCD college, how many courses you have synced locally. Add\n" +
			"--data-source live to also fetch each college's live API total and see what's\n" +
			"missing — so an empty offline answer means 'not in the catalog', not 'not synced\n" +
			"yet'. The live comparison probes all 9 colleges and is slower. For catalog content\n" +
			"distribution (subjects, transferable share) use 'stats' instead.",
		Example: "  laccd-courses-pp-cli coverage\n  laccd-courses-pp-cli coverage --data-source live",
		Annotations: map[string]string{
			"mcp:read-only":  "true",
			"pp:data-source": "auto",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if dbPath == "" {
				dbPath = defaultDBPath("laccd-courses-pp-cli")
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			// Local counts (store may not exist yet — treat as zero).
			localByHost := map[string]int{}
			if _, statErr := os.Stat(dbPath); statErr == nil {
				if db, err := store.OpenWithContext(ctx, dbPath); err == nil {
					defer db.Close()
					rows, qerr := db.DB().QueryContext(ctx, `SELECT tenant, COUNT(*) FROM courses GROUP BY tenant`)
					if qerr == nil {
						defer rows.Close()
						for rows.Next() {
							var tenant string
							var n int
							if rows.Scan(&tenant, &n) == nil {
								localByHost[tenant] = n
							}
						}
						if rerr := rows.Err(); rerr != nil {
							fmt.Fprintf(cmd.ErrOrStderr(), "warning: error reading local counts (results may be partial): %v\n", rerr)
						}
					}
				}
			}

			// API-total comparison requires one (slow) call per college, so it
			// is opt-in via --data-source live. The default shows synced counts
			// instantly.
			checkAPI := flags.dataSource == "live"
			var c clientGetter
			if checkAPI {
				cl, err := flags.newClient()
				if err != nil {
					return err
				}
				c = cl
			} else if wantsHumanTable(cmd.OutOrStdout(), flags) {
				fmt.Fprintln(cmd.ErrOrStderr(), "showing local mirror counts; add --data-source live to compare against live API totals (probes all 9 colleges, slower)")
			}
			// The eLumen list endpoint is slow; cap live probes under dogfood
			// so the live matrix fits its per-command timeout.
			probeLimit := len(laccdColleges)
			if checkAPI && cliutil.IsDogfoodEnv() {
				probeLimit = 1
			}

			// Probe colleges concurrently — the eLumen backend is slow and 9
			// sequential probes blow past interactive/sample timeouts. The
			// client's rate limiter still throttles outbound requests.
			totals := make([]int, len(laccdColleges))
			for i := range totals {
				totals[i] = -1
			}
			if checkAPI {
				var wg sync.WaitGroup
				for i := 0; i < probeLimit && i < len(laccdColleges); i++ {
					wg.Add(1)
					go func(idx int) {
						defer wg.Done()
						if total, ok := probeAPITotal(ctx, c, laccdColleges[idx].Host); ok {
							totals[idx] = total
						}
					}(i)
				}
				wg.Wait()
			}

			view := coverageView{CheckedAPI: checkAPI}
			if !checkAPI {
				view.Note = "api_total/missing not probed (shown as -1); pass --data-source live to compare against live API totals"
			}
			for i, college := range laccdColleges {
				row := coverageRow{College: college.Code, Tenant: college.Host, Local: localByHost[college.Host], APITotal: totals[i], Missing: -1}
				if totals[i] >= 0 {
					if totals[i] >= row.Local {
						row.Missing = totals[i] - row.Local
					} else {
						row.Missing = 0
					}
				}
				view.Rows = append(view.Rows, row)
			}

			if wantsHumanTable(cmd.OutOrStdout(), flags) {
				items := make([]map[string]any, 0, len(view.Rows))
				for _, r := range view.Rows {
					apiCol := "—"
					missCol := "—"
					if r.APITotal >= 0 {
						apiCol = fmt.Sprintf("%d", r.APITotal)
						missCol = fmt.Sprintf("%d", r.Missing)
					}
					items = append(items, map[string]any{"college": r.College, "local": r.Local, "api_total": apiCol, "missing": missCol})
				}
				if checkAPI && probeLimit < len(laccdColleges) {
					fmt.Fprintf(cmd.ErrOrStderr(), "note: probed API totals for %d of %d colleges (curtailed)\n", probeLimit, len(laccdColleges))
				}
				return printAutoTable(cmd.OutOrStdout(), items)
			}
			return printJSONFiltered(cmd.OutOrStdout(), view, flags)
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Local store path (default: per-user data dir)")
	return cmd
}

// clientGetter is the subset of the generated client coverage needs; declaring
// it keeps probeAPITotal independently testable.
type clientGetter interface {
	Get(ctx context.Context, path string, params map[string]string) (json.RawMessage, error)
}

// probeAPITotal fetches a 1-row page for a college and reads pagination.total.
func probeAPITotal(ctx context.Context, c clientGetter, host string) (int, bool) {
	data, err := c.Get(ctx, "/public/courses", map[string]string{
		"tenant":   host,
		"status":   "approved",
		"page":     "1",
		"pageSize": "1",
	})
	if err != nil {
		return 0, false
	}
	return paginationTotal(data)
}

// paginationTotal extracts pagination.total from an eLumen HAL list envelope.
func paginationTotal(data json.RawMessage) (int, bool) {
	var env struct {
		Pagination struct {
			Total int `json:"total"`
		} `json:"pagination"`
	}
	if json.Unmarshal(data, &env) != nil {
		return 0, false
	}
	return env.Pagination.Total, true
}
