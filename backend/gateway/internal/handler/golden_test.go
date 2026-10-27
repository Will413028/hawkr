package handler

import (
	"bytes"
	"encoding/json"
	"flag"
	"os"
	"path/filepath"
	"testing"
)

var updateGolden = flag.Bool("update-golden", false, "rewrite golden snapshot files from current output")

// assertGolden pins an endpoint's exact JSON wire shape. Run with
// `-update-golden` to (re)generate after an INTENDED wire change; the diff in
// review is the audit of expected (field case / TSID number→string / zero-value
// appearance / relocation) vs unexpected (dropped field, wrong type) changes.
func assertGolden(t *testing.T, name string, got []byte) {
	t.Helper()
	path := filepath.Join("testdata", "golden", name+".json")
	norm := normalizeJSON(t, got)
	if *updateGolden {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, norm, 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read golden %s (regenerate with `go test ./internal/handler/ -run %s -update-golden`): %v", path, t.Name(), err)
	}
	if !bytes.Equal(bytes.TrimSpace(want), norm) {
		t.Errorf("golden mismatch %s\n got: %s\nwant: %s", name, norm, bytes.TrimSpace(want))
	}
}

// normalizeJSON re-encodes through a map so key ordering / whitespace never
// causes spurious diffs; only structural/value changes show up.
func normalizeJSON(t *testing.T, b []byte) []byte {
	t.Helper()
	var v interface{}
	if err := json.Unmarshal(b, &v); err != nil {
		t.Fatalf("golden input is not valid JSON: %v; raw=%s", err, b)
	}
	out, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return out
}
