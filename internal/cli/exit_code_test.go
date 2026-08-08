package cli

import (
	"errors"
	"fmt"
	"testing"

	"github.com/johnnyrobot/laccd-courses-pp-cli/internal/client"
)

// TestExitCode_ContractMatchesREADME pins the exit-code contract the README
// promises. Issue #1: command bodies mostly return fmt.Errorf("...: %w", err)
// around a client call rather than calling apiErr/notFoundErr by hand, so
// every API failure used to collapse to a bare 1 and callers had to sniff
// stderr for HTTP statuses.
func TestExitCode_ContractMatchesREADME(t *testing.T) {
	apiErrorAt := func(status int) error {
		return &client.APIError{Method: "GET", Path: "/courses/1", StatusCode: status, Body: "{}"}
	}

	cases := []struct {
		name string
		err  error
		want int
	}{
		{"nil-ish unclassified", errors.New("something went wrong"), 1},
		{"explicit usage", usageErr(errors.New("bad args")), 2},
		{"explicit not found", notFoundErr(errors.New("missing")), 3},
		{"explicit auth", authErr(errors.New("denied")), 4},
		{"explicit api", apiErr(errors.New("upstream")), 5},
		{"explicit rate limit", rateLimitErr(errors.New("slow down")), 7},
		{"explicit config", configErr(errors.New("bad config")), 10},

		// The cases issue #1 was actually about: an APIError reached
		// through an ordinary %w wrap, with no hand-written classification.
		{"wrapped 404", fmt.Errorf("fetching course: %w", apiErrorAt(404)), 3},
		{"wrapped 401", fmt.Errorf("fetching course: %w", apiErrorAt(401)), 4},
		{"wrapped 403", fmt.Errorf("fetching course: %w", apiErrorAt(403)), 4},
		{"wrapped 429", fmt.Errorf("fetching course: %w", apiErrorAt(429)), 7},
		{"wrapped 500", fmt.Errorf("fetching course: %w", apiErrorAt(500)), 5},
		{"wrapped 400", fmt.Errorf("fetching course: %w", apiErrorAt(400)), 5},
		{"doubly wrapped 404", fmt.Errorf("outer: %w", fmt.Errorf("inner: %w", apiErrorAt(404))), 3},

		// An explicit code always wins over status classification.
		{"cliError wins over APIError", usageErr(apiErrorAt(500)), 2},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := ExitCode(tc.err); got != tc.want {
				t.Errorf("ExitCode(%v) = %d, want %d", tc.err, got, tc.want)
			}
		})
	}
}
