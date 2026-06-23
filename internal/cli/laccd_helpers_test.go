// Copyright 2026 johnnyrobot and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored tests for college mapping, HAL envelope, compare, and coverage logic.

package cli

import (
	"encoding/json"
	"testing"
)

func TestResolveTenant(t *testing.T) {
	t.Parallel()
	cases := []struct{ in, want string }{
		{"ELAC", "elac.elumenapp.com"},
		{"elac", "elac.elumenapp.com"},
		{"LAMC", "lamission.elumenapp.com"}, // non-obvious mapping
		{"LAPC", "pierce.elumenapp.com"},    // non-obvious mapping
		{"pierce", "pierce.elumenapp.com"},
		{"pierce.elumenapp.com", "pierce.elumenapp.com"},
		{"", ""},
		{"unknown.example.com", "unknown.example.com"}, // passthrough
	}
	for _, tc := range cases {
		if got := resolveTenant(tc.in); got != tc.want {
			t.Errorf("resolveTenant(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestCollegeCodeForHost(t *testing.T) {
	t.Parallel()
	if got := collegeCodeForHost("lamission.elumenapp.com"); got != "LAMC" {
		t.Errorf("collegeCodeForHost(lamission) = %q, want LAMC", got)
	}
	if got := collegeCodeForHost("other.example.com"); got != "other.example.com" {
		t.Errorf("unknown host should pass through, got %q", got)
	}
}

func TestNormalizeListPayload(t *testing.T) {
	t.Parallel()
	// HAL envelope -> bare array
	hal := json.RawMessage(`{"_embedded":{"courses":[{"id":1},{"id":2}]},"_links":{},"pagination":{"total":2}}`)
	var arr []json.RawMessage
	if err := json.Unmarshal(normalizeListPayload(hal), &arr); err != nil {
		t.Fatalf("normalized HAL not an array: %v", err)
	}
	if len(arr) != 2 {
		t.Errorf("expected 2 items from HAL envelope, got %d", len(arr))
	}
	// Bare array passes through.
	bare := json.RawMessage(`[{"id":1}]`)
	if string(normalizeListPayload(bare)) != string(bare) {
		t.Error("bare array should pass through unchanged")
	}
	// Detail object (no _embedded) passes through.
	detail := json.RawMessage(`{"id":1,"code":"ACCTG001"}`)
	if string(normalizeListPayload(detail)) != string(detail) {
		t.Error("detail object should pass through unchanged")
	}
}

func TestPaginationTotal(t *testing.T) {
	t.Parallel()
	total, ok := paginationTotal(json.RawMessage(`{"pagination":{"total":587},"_embedded":{"courses":[]}}`))
	if !ok || total != 587 {
		t.Errorf("paginationTotal = (%d,%v), want (587,true)", total, ok)
	}
	if _, ok := paginationTotal(json.RawMessage(`not json`)); ok {
		t.Error("expected ok=false on invalid JSON")
	}
}

func TestFilterBySubjectNumber(t *testing.T) {
	t.Parallel()
	courses := []json.RawMessage{
		json.RawMessage(`{"subject":"ACCTG","number":"001","tenant":"elac.elumenapp.com"}`),
		json.RawMessage(`{"subject":"ACCTG","number":"002","tenant":"elac.elumenapp.com"}`),
		json.RawMessage(`{"subject":"acctg","number":"1","tenant":"wlac.elumenapp.com"}`),  // case + zero-pad
		json.RawMessage(`{"subject":"MATH","number":"001","tenant":"elac.elumenapp.com"}`), // wrong subject
	}
	got := filterBySubjectNumber(courses, "ACCTG", "001")
	if len(got) != 2 {
		t.Fatalf("filterBySubjectNumber matched %d, want 2", len(got))
	}
}

func TestBuildCompareView(t *testing.T) {
	t.Parallel()
	mk := func(t *testing.T, tenant string, id int, prereqs int) json.RawMessage {
		t.Helper()
		req := map[string]any{"type": "OR", "blockList": []any{}}
		blocks := []any{}
		for i := 0; i < prereqs; i++ {
			blocks = append(blocks, map[string]any{"item": map[string]any{"code": "PRE", "title": "x", "itemType": "Prerequisite"}})
		}
		req["blockList"] = blocks
		fci := map[string]any{
			"creditsAndHours":      []any{map[string]any{"isDefault": true, "minimumCredit": 5, "maximumCredit": 5}},
			"courseTransferStatus": map[string]any{"valueName": "Transferable to both UC and CSU"},
			"requisites":           req,
		}
		fb, _ := json.Marshal(fci)
		c := map[string]any{"id": id, "subject": "ACCTG", "number": "001", "code": "ACCTG001", "tenant": tenant, "fullCourseInfo": string(fb)}
		b, _ := json.Marshal(c)
		return b
	}

	// Two colleges, identical profiles.
	identical := []json.RawMessage{mk(t, "elac.elumenapp.com", 1, 0), mk(t, "wlac.elumenapp.com", 2, 0)}
	v := buildCompareView("ACCTG", "001", identical)
	if v.Found != 2 || !v.Identical {
		t.Errorf("identical case: Found=%d Identical=%v, want 2/true", v.Found, v.Identical)
	}

	// Two colleges, differing prereq counts.
	differ := []json.RawMessage{mk(t, "elac.elumenapp.com", 1, 0), mk(t, "lasc.elumenapp.com", 2, 3)}
	v2 := buildCompareView("ACCTG", "001", differ)
	if v2.Found != 2 || v2.Identical {
		t.Errorf("differ case: Found=%d Identical=%v, want 2/false", v2.Found, v2.Identical)
	}

	// Highest-id version wins per college.
	dupes := []json.RawMessage{mk(t, "elac.elumenapp.com", 1, 0), mk(t, "elac.elumenapp.com", 9, 2)}
	v3 := buildCompareView("ACCTG", "001", dupes)
	if v3.Found != 1 {
		t.Fatalf("dupe collapse: Found=%d, want 1", v3.Found)
	}
	if v3.Courses[0].ID != 9 || v3.Courses[0].Prereqs != 2 {
		t.Errorf("expected highest-id version (id=9, prereqs=2), got id=%d prereqs=%d", v3.Courses[0].ID, v3.Courses[0].Prereqs)
	}
}
