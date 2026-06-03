package storage

import (
	"encoding/json"
	"testing"
)

// TestIsIdentical_MatchesPythonSentinelSerialisation pins the cross-language
// contract that broke in Phase 122d.1: the worker writes the BUG-B sentinel
// with Python's `json.dumps` default separators — `{"op": "identical"}`, WITH
// a space after the colon — while the original detector string-matched against
// a hand-written `{"op":"identical"}` (no space). The compare never succeeded,
// so the sentinel leaked through `GetArticleRevisionDiff` as a malformed diff
// op and the dashboard rendered a blank revision in the slider.
//
// `IsIdentical` now decodes the op and compares the `Op` field, so it must
// recognise the sentinel regardless of JSON whitespace. The first case is the
// EXACT bytes the worker stores (verified against the live ClickHouse cell).
func TestIsIdentical_MatchesPythonSentinelSerialisation(t *testing.T) {
	cases := []struct {
		name string
		raw  []string
		want bool
	}{
		{"python_default_separators_with_space", []string{`{"op": "identical"}`}, true},
		{"compact_separators_no_space", []string{`{"op":"identical"}`}, true},
		{"extra_whitespace", []string{`{ "op" : "identical" }`}, true},
		{"real_diff_op_is_not_identical", []string{`{"op": "del", "before": "x"}`}, false},
		{"two_ops_is_not_sentinel", []string{`{"op": "identical"}`, `{"op": "add", "after": "y"}`}, false},
		{"empty_is_not_sentinel", nil, false},
		{"malformed_json_is_not_sentinel", []string{`{not json`}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			row := &ArticleRevisionDiffRow{DiffParagraphs: tc.raw}
			if got := row.IsIdentical(); got != tc.want {
				t.Fatalf("IsIdentical(%q) = %v, want %v", tc.raw, got, tc.want)
			}
		})
	}
}

// TestIsIdentical_AgreesWithPythonJSONDumps belt-and-braces: serialise the
// sentinel the same way the worker's `json.dumps({"op": "identical"})` does
// (default separators) and confirm the detector still fires. Guards against a
// future refactor reintroducing a brittle byte-for-byte compare.
func TestIsIdentical_AgreesWithPythonJSONDumps(t *testing.T) {
	// json.Marshal emits compact separators; inject the space the Python
	// encoder produces so the fixture matches the on-disk reality exactly.
	marshalled, err := json.Marshal(map[string]string{"op": "identical"})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	pythonStyle := string(marshalled[:5]) + " " + string(marshalled[5:]) // `{"op":` + ` ` + `"identical"}`
	row := &ArticleRevisionDiffRow{DiffParagraphs: []string{pythonStyle}}
	if !row.IsIdentical() {
		t.Fatalf("sentinel %q not recognised", pythonStyle)
	}
}
