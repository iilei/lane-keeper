package main

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const testConfigVersion = `
[_.lane-keeper]
version = 1
`

func TestIntrospectConfigFileLintCatchesNonTripleQuotedPredicateSyntaxError(t *testing.T) {
	t.Parallel()

	// A single-line basic string predicate: the legacy regex-based extractor
	// only recognized triple-quoted predicates and would have missed this.
	content := testConfigVersion + `
[_.lane-keeper.defaults]
remote = "origin"

[_.lane-keeper.checks.ready]
predicate = "def broken(:"

[_.lane-keeper.workflows.release]
checks = ["ready"]
target_branch = { resolve = "literal", value = "master" }
`
	tomlPath := writeTempConfig(t, content)

	result := introspectConfigFile(context.Background(), tomlPath, false)
	if len(result.Errors) == 0 {
		t.Fatal("introspectConfigFile() Errors = empty, want a predicate syntax error")
	}
	var found bool
	for _, err := range result.Errors {
		if strings.Contains(err.Error(), `check "ready"`) {
			found = true
		}
	}
	if !found {
		t.Errorf("introspectConfigFile() Errors = %v, want a check %q error", result.Errors, "ready")
	}
}

func TestIntrospectConfigFileLintAcceptsValidPredicate(t *testing.T) {
	t.Parallel()

	content := testConfigVersion + `
[_.lane-keeper.defaults]
remote = "origin"

[_.lane-keeper.checks.ready]
predicate = "succeed()"

[_.lane-keeper.workflows.release]
checks = ["ready"]
target_branch = { resolve = "literal", value = "master" }
`
	tomlPath := writeTempConfig(t, content)

	result := introspectConfigFile(context.Background(), tomlPath, false)
	if len(result.Errors) != 0 {
		t.Errorf("introspectConfigFile() Errors = %v, want none", result.Errors)
	}
}

func writeTempConfig(t *testing.T, content string) string {
	t.Helper()
	tomlPath := filepath.Join(t.TempDir(), "lane-keeper.toml")
	if err := os.WriteFile(tomlPath, []byte(content), 0o600); err != nil {
		t.Fatalf("WriteFile(): %v", err)
	}
	return tomlPath
}
