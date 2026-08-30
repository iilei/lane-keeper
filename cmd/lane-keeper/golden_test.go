package main

import (
	"os"
	"path/filepath"
	"regexp"
	"testing"
)

// updateGoldenEnv, when set to any non-empty value, causes assertGolden to
// (re)write the expected golden file instead of comparing against it.
const updateGoldenEnv = "UPDATE_GOLDEN"

var (
	goldenShortSHAPattern = regexp.MustCompile(`\b[0-9a-f]{7,40}\b`)
	goldenDatePattern     = regexp.MustCompile(`\d{4}-\d{2}-\d{2}(T\d{2}:\d{2}:\d{2}[^\s"]*)?`)
)

// normalizeGolden replaces non-deterministic substrings (commit SHAs, dates)
// with stable placeholders so golden files remain reproducible across runs
// against freshly created temporary Git repositories.
func normalizeGolden(output string) string {
	output = goldenShortSHAPattern.ReplaceAllString(output, "SHA")
	output = goldenDatePattern.ReplaceAllString(output, "DATE")
	return output
}

// assertGolden compares got (already normalized by the caller if needed)
// against testdata/<name>.golden. Set UPDATE_GOLDEN=1 to write/refresh the
// golden file instead of asserting.
func assertGolden(t *testing.T, name, got string) {
	t.Helper()
	goldenPath := filepath.Join("testdata", name+".golden")
	if os.Getenv(updateGoldenEnv) != "" {
		if err := os.MkdirAll(filepath.Dir(goldenPath), 0o750); err != nil {
			t.Fatalf("MkdirAll(): %v", err)
		}
		if err := os.WriteFile(goldenPath, []byte(got), 0o600); err != nil {
			t.Fatalf("WriteFile(%s): %v", goldenPath, err)
		}
		return
	}
	want, err := os.ReadFile(goldenPath) //nolint:gosec // fixed testdata path under this package.
	if err != nil {
		t.Fatalf("ReadFile(%s): %v (run with UPDATE_GOLDEN=1 to create it)", goldenPath, err)
	}
	if got != string(want) {
		t.Errorf("output does not match %s\ngot:\n%s\nwant:\n%s", goldenPath, got, string(want))
	}
}
