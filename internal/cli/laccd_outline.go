// Copyright 2026 johnnyrobot and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored eLumen Outline-of-Record parsing (standalone file, regen-safe).

package cli

import (
	"encoding/json"
	"strconv"
	"strings"
)

// decodeFullCourseInfo returns the parsed fullCourseInfo object for a course.
// eLumen serializes fullCourseInfo as a JSON string inside the course record
// (both in list and detail responses), so we unmarshal twice. It also tolerates
// fullCourseInfo already being an object.
func decodeFullCourseInfo(courseData json.RawMessage) (map[string]json.RawMessage, bool) {
	var top map[string]json.RawMessage
	if json.Unmarshal(courseData, &top) != nil {
		return nil, false
	}
	raw, ok := top["fullCourseInfo"]
	if !ok || len(raw) == 0 {
		return nil, false
	}
	// Case 1: fullCourseInfo is a JSON-encoded string.
	var asString string
	if json.Unmarshal(raw, &asString) == nil && asString != "" {
		var fci map[string]json.RawMessage
		if json.Unmarshal([]byte(asString), &fci) == nil {
			return fci, true
		}
		return nil, false
	}
	// Case 2: fullCourseInfo is already an object.
	var fci map[string]json.RawMessage
	if json.Unmarshal(raw, &fci) == nil {
		return fci, true
	}
	return nil, false
}

// requisiteEntry is one resolved requisite (a course or condition) pulled from
// the nested requisites tree.
type requisiteEntry struct {
	Code     string `json:"code"`
	Title    string `json:"title"`
	Type     string `json:"type"` // Prerequisite | Advisory | Corequisite | Limitation
	IsCourse bool   `json:"is_course"`
}

// parseRequisites walks the eLumen requisites tree and returns a flat list of
// requisite entries. The tree is nested AND/OR blocks: a node has either an
// `item` (a leaf) or a `blockList` (children) — sometimes both.
func parseRequisites(fci map[string]json.RawMessage) []requisiteEntry {
	raw, ok := fci["requisites"]
	if !ok || len(raw) == 0 {
		return nil
	}
	var entries []requisiteEntry
	walkRequisiteNode(raw, &entries)
	return entries
}

func walkRequisiteNode(raw json.RawMessage, out *[]requisiteEntry) {
	var node struct {
		Item      json.RawMessage   `json:"item"`
		BlockList []json.RawMessage `json:"blockList"`
	}
	if json.Unmarshal(raw, &node) != nil {
		return
	}
	if len(node.Item) > 0 {
		var item struct {
			Code     string `json:"code"`
			Title    string `json:"title"`
			ItemType string `json:"itemType"`
			IsCourse bool   `json:"isCourse"`
		}
		if json.Unmarshal(node.Item, &item) == nil && (item.Code != "" || item.Title != "") {
			*out = append(*out, requisiteEntry{
				Code:     item.Code,
				Title:    item.Title,
				Type:     item.ItemType,
				IsCourse: item.IsCourse,
			})
		}
	}
	for _, child := range node.BlockList {
		walkRequisiteNode(child, out)
	}
}

// courseUnits holds the minimum/maximum credit range for a course, pulled from
// the default creditsAndHours profile.
type courseUnits struct {
	Min float64 `json:"min"`
	Max float64 `json:"max"`
	OK  bool    `json:"-"`
}

// extractUnits returns the credit range from the default (or first)
// creditsAndHours profile.
func extractUnits(fci map[string]json.RawMessage) courseUnits {
	raw, ok := fci["creditsAndHours"]
	if !ok || len(raw) == 0 {
		return courseUnits{}
	}
	var profiles []struct {
		Credit        jsonNumber `json:"credit"`
		MinimumCredit jsonNumber `json:"minimumCredit"`
		MaximumCredit jsonNumber `json:"maximumCredit"`
		IsDefault     bool       `json:"isDefault"`
	}
	if json.Unmarshal(raw, &profiles) != nil || len(profiles) == 0 {
		return courseUnits{}
	}
	pick := profiles[0]
	for _, p := range profiles {
		if p.IsDefault {
			pick = p
			break
		}
	}
	minC := pick.MinimumCredit.float()
	maxC := pick.MaximumCredit.float()
	if minC == 0 && maxC == 0 {
		c := pick.Credit.float()
		minC, maxC = c, c
	}
	return courseUnits{Min: minC, Max: maxC, OK: true}
}

// extractTransferStatus returns the human-readable courseTransferStatus, or "".
// eLumen encodes this as an object {valueName, valueCode, status, ...} but
// sometimes as a bare string, so we handle both.
func extractTransferStatus(fci map[string]json.RawMessage) string {
	return attributeValueName(fci, "courseTransferStatus")
}

// extractGeneralEducationStatus returns courseGeneralEducationStatus, or "".
func extractGeneralEducationStatus(fci map[string]json.RawMessage) string {
	return attributeValueName(fci, "courseGeneralEducationStatus")
}

// attributeValueName reads an eLumen curriculum-attribute field that may be a
// {valueName: ...} object or a bare string.
func attributeValueName(fci map[string]json.RawMessage, key string) string {
	raw, ok := fci[key]
	if !ok || len(raw) == 0 {
		return ""
	}
	var s string
	if json.Unmarshal(raw, &s) == nil {
		return strings.TrimSpace(s)
	}
	var obj struct {
		ValueName string `json:"valueName"`
	}
	if json.Unmarshal(raw, &obj) == nil {
		return strings.TrimSpace(obj.ValueName)
	}
	return ""
}

// jsonNumber decodes a value that may be a JSON number or a JSON-encoded
// numeric string (eLumen mixes both for credit fields).
type jsonNumber struct{ v float64 }

func (n *jsonNumber) UnmarshalJSON(b []byte) error {
	s := strings.Trim(strings.TrimSpace(string(b)), "\"")
	if s == "" || s == "null" {
		n.v = 0
		return nil
	}
	f, err := strconv.ParseFloat(s, 64)
	if err != nil {
		n.v = 0
		return nil // tolerate junk; treat as zero
	}
	n.v = f
	return nil
}

func (n jsonNumber) float() float64 { return n.v }

// normalizeCourseNumber strips surrounding whitespace and leading zeros so a
// user-typed "101" matches a stored "101" and "1" matches "001". Non-numeric
// course numbers (e.g. "NC260CE") compare case-insensitively as-is.
func normalizeCourseNumber(s string) string {
	t := strings.TrimSpace(strings.ToUpper(s))
	if t == "" {
		return ""
	}
	if _, err := strconv.Atoi(t); err == nil {
		trimmed := strings.TrimLeft(t, "0")
		if trimmed == "" {
			return "0" // "0"/"000" must not collapse to the empty string
		}
		return trimmed
	}
	return t
}
