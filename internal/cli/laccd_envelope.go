// Copyright 2026 johnnyrobot and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored eLumen HAL helpers (kept in a standalone file so regen preserves them).

package cli

import "encoding/json"

// normalizeListPayload unwraps eLumen's HAL list envelope
// ({"_embedded": {"<resource>": [...]}, "_links": ..., "pagination": ...})
// into a bare JSON array so live list responses display and cache exactly like
// local/array responses. Bare arrays (e.g. /public/tenants/) and detail objects
// (e.g. /public/courses/{id}) pass through unchanged.
func normalizeListPayload(data json.RawMessage) json.RawMessage {
	if len(data) == 0 {
		return data
	}
	// Already a bare array — nothing to unwrap.
	var arr []json.RawMessage
	if json.Unmarshal(data, &arr) == nil {
		return data
	}
	var obj map[string]json.RawMessage
	if json.Unmarshal(data, &obj) != nil {
		return data // not a JSON object
	}
	// Only rewrite HAL list envelopes; leave other detail objects alone.
	if _, ok := obj["_embedded"]; !ok {
		return data
	}
	items, _, _ := extractPageItems(data, "page")
	out, err := json.Marshal(items)
	if err != nil {
		return data
	}
	return out
}
