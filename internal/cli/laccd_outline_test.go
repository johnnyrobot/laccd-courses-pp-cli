// Copyright 2026 johnnyrobot and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored tests for eLumen Outline-of-Record parsing.

package cli

import (
	"encoding/json"
	"testing"
)

// makeCourse builds a course record whose fullCourseInfo is a JSON-encoded
// string, exactly like the eLumen API returns it.
func makeCourse(t *testing.T, fci map[string]any) json.RawMessage {
	t.Helper()
	fciBytes, err := json.Marshal(fci)
	if err != nil {
		t.Fatalf("marshal fci: %v", err)
	}
	course := map[string]any{
		"id":             1,
		"subject":        "ACCTG",
		"number":         "001",
		"code":           "ACCTG001",
		"tenant":         "elac.elumenapp.com",
		"fullCourseInfo": string(fciBytes),
	}
	b, err := json.Marshal(course)
	if err != nil {
		t.Fatalf("marshal course: %v", err)
	}
	return b
}

func TestDecodeFullCourseInfo(t *testing.T) {
	t.Parallel()
	course := makeCourse(t, map[string]any{"subject": "ACCTG"})
	fci, ok := decodeFullCourseInfo(course)
	if !ok {
		t.Fatal("expected to decode fullCourseInfo")
	}
	if _, has := fci["subject"]; !has {
		t.Errorf("expected subject key in fci, got keys %v", keysOf(fci))
	}

	// No fullCourseInfo present.
	if _, ok := decodeFullCourseInfo(json.RawMessage(`{"id":1}`)); ok {
		t.Error("expected ok=false when fullCourseInfo absent")
	}
}

func TestParseRequisites(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name      string
		req       any
		wantCodes []string
	}{
		{
			name:      "empty block list",
			req:       map[string]any{"type": "OR", "blockList": []any{}},
			wantCodes: nil,
		},
		{
			name: "nested AND/OR with one course",
			req: map[string]any{"type": "OR", "blockList": []any{
				map[string]any{"type": "AND", "blockList": []any{
					map[string]any{"item": map[string]any{"code": "MATH261", "title": "Calculus I", "itemType": "Prerequisite", "isCourse": true}},
				}},
			}},
			wantCodes: []string{"MATH261"},
		},
		{
			name: "two requisites across branches",
			req: map[string]any{"type": "OR", "blockList": []any{
				map[string]any{"item": map[string]any{"code": "CS119", "title": "Programming in Python", "itemType": "Advisory"}},
				map[string]any{"item": map[string]any{"code": "MATH261", "title": "Calculus I", "itemType": "Co-Requisite"}},
			}},
			wantCodes: []string{"CS119", "MATH261"},
		},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			fci, ok := decodeFullCourseInfo(makeCourse(t, map[string]any{"requisites": tc.req}))
			if !ok {
				t.Fatal("decode failed")
			}
			got := parseRequisites(fci)
			if len(got) != len(tc.wantCodes) {
				t.Fatalf("got %d requisites, want %d (%v)", len(got), len(tc.wantCodes), got)
			}
			for i, code := range tc.wantCodes {
				if got[i].Code != code {
					t.Errorf("requisite[%d].Code = %q, want %q", i, got[i].Code, code)
				}
			}
		})
	}
}

func TestExtractUnits(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name     string
		profiles any
		wantMin  float64
		wantMax  float64
		wantOK   bool
	}{
		{
			name:     "default profile min/max",
			profiles: []any{map[string]any{"isDefault": true, "minimumCredit": 3, "maximumCredit": 4}},
			wantMin:  3, wantMax: 4, wantOK: true,
		},
		{
			name:     "credit string falls back to credit field",
			profiles: []any{map[string]any{"credit": "5"}},
			wantMin:  5, wantMax: 5, wantOK: true,
		},
		{
			name:     "no profiles",
			profiles: []any{},
			wantMin:  0, wantMax: 0, wantOK: false,
		},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			fci, _ := decodeFullCourseInfo(makeCourse(t, map[string]any{"creditsAndHours": tc.profiles}))
			u := extractUnits(fci)
			if u.OK != tc.wantOK || u.Min != tc.wantMin || u.Max != tc.wantMax {
				t.Errorf("extractUnits = {%g,%g,%v}, want {%g,%g,%v}", u.Min, u.Max, u.OK, tc.wantMin, tc.wantMax, tc.wantOK)
			}
		})
	}
}

func TestExtractTransferStatus(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name   string
		status any
		want   string
	}{
		{"object valueName", map[string]any{"valueName": "Transferable to both UC and CSU"}, "Transferable to both UC and CSU"},
		{"bare string", "Not Transferable", "Not Transferable"},
		{"absent", nil, ""},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			m := map[string]any{}
			if tc.status != nil {
				m["courseTransferStatus"] = tc.status
			}
			fci, _ := decodeFullCourseInfo(makeCourse(t, m))
			if got := extractTransferStatus(fci); got != tc.want {
				t.Errorf("extractTransferStatus = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestCourseIsTransferable(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name   string
		status any
		want   bool
	}{
		{"UC and CSU", map[string]any{"valueName": "Transferable to both UC and CSU"}, true},
		{"CSU only", map[string]any{"valueName": "Transferable to CSU only"}, true},
		{"not transferable", map[string]any{"valueName": "Not Transferable"}, false},
		{"absent", nil, false},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			m := map[string]any{}
			if tc.status != nil {
				m["courseTransferStatus"] = tc.status
			}
			if got := courseIsTransferable(makeCourse(t, m)); got != tc.want {
				t.Errorf("courseIsTransferable = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestNormalizeCourseNumber(t *testing.T) {
	t.Parallel()
	cases := []struct{ in, want string }{
		{"001", "1"},
		{"101", "101"},
		{"1", "1"},
		{" 003 ", "3"},
		{"NC260CE", "NC260CE"},
		{"nc260ce", "NC260CE"},
		{"", ""},
	}
	for _, tc := range cases {
		if got := normalizeCourseNumber(tc.in); got != tc.want {
			t.Errorf("normalizeCourseNumber(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func keysOf(m map[string]json.RawMessage) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}
