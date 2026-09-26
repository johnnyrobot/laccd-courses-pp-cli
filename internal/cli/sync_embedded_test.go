// Copyright 2026 johnnyrobot and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"testing"
)

// eLumen paginates with a HAL envelope. An exhausted page still carries
// _embedded/_links/pagination with an empty item array; if that is not read as
// an empty page, syncResource falls through to the single-object path and the
// whole resource dies with "missing id for courses". That is reachable in
// normal use whenever a stored cursor points past the end of a filtered result
// set, e.g. re-running sync with --resource-param courses:query=ACCTG.
func TestIsEmptyPageResponseHandlesHALEmbedded(t *testing.T) {
	for _, tc := range []struct {
		name string
		body string
		want bool
	}{
		{
			name: "exhausted page",
			body: `{"_embedded":{"courses":[]},"_links":{"self":{"href":"/public/courses?page=4"}},"pagination":{"page":4}}`,
			want: true,
		},
		{
			name: "page with results",
			body: `{"_embedded":{"courses":[{"id":40830,"name":"Introductory Accounting I"}]},"_links":{},"pagination":{"page":1}}`,
			want: false,
		},
		{
			name: "programs share the shape",
			body: `{"_embedded":{"programs":[]},"_links":{},"pagination":{"page":9}}`,
			want: true,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := isEmptyPageResponse(json.RawMessage(tc.body)); got != tc.want {
				t.Errorf("isEmptyPageResponse(%s) = %v, want %v", tc.name, got, tc.want)
			}
		})
	}
}

// A HAL page must still yield its items, so adding _embedded to the envelope
// unwrap list cannot silently turn a full page into an empty one.
func TestExtractPageItemsHandlesHALEmbedded(t *testing.T) {
	body := `{"_embedded":{"courses":[{"id":1},{"id":2}]},"_links":{},"pagination":{"page":1}}`
	items, _, _ := extractPageItems(json.RawMessage(body), "page")
	if len(items) != 2 {
		t.Fatalf("extractPageItems returned %d items, want 2", len(items))
	}
}
