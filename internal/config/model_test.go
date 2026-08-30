package config_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/iilei/lane-keeper/internal/config"
)

const readyCheckName = "ready"

// TestParseAtQualifierReadsExampleMiseTOML proves .example.mise.toml is
// directly usable via --config with the default "_.lane-keeper" qualifier,
// since it retains the mise.toml-embedded nesting.
func TestParseAtQualifierReadsExampleMiseTOML(t *testing.T) {
	t.Parallel()

	repositoryRoot, err := filepath.Abs("../..")
	if err != nil {
		t.Fatalf("Abs() error = %v", err)
	}
	//nolint:gosec // fixed test-relative path, not user input.
	content, err := os.ReadFile(filepath.Join(repositoryRoot, ".example.mise.toml"))
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	model, found, err := config.ParseAtQualifier(string(content), "_.lane-keeper")
	if err != nil {
		t.Fatalf("ParseAtQualifier() error = %v", err)
	}
	if !found {
		t.Fatal("ParseAtQualifier() found = false, want true")
	}
	if errs := model.Validate(); len(errs) > 0 {
		t.Errorf("Validate() errors = %v, want none", errs)
	}
}

func TestModelValidateAcceptsValidSharedSource(t *testing.T) {
	t.Parallel()

	model := config.Model{
		Version: 1,
		Shared:  config.Shared{Source: "value = 1\n"},
	}
	for _, err := range model.Validate() {
		if strings.Contains(err.Error(), "shared") {
			t.Errorf("Validate() error = %v, want no shared error", err)
		}
	}
}

func TestModelValidateRejectsInvalidSharedSyntax(t *testing.T) {
	t.Parallel()

	model := config.Model{
		Version: 1,
		Shared:  config.Shared{Source: "def broken(:\n"},
	}
	var found bool
	for _, err := range model.Validate() {
		if strings.Contains(err.Error(), "shared") {
			found = true
		}
	}
	if !found {
		t.Fatal("Validate() did not report a shared error for invalid syntax")
	}
}

func TestModelValidateAcceptsValidCheckPredicate(t *testing.T) {
	t.Parallel()

	model := config.Model{
		Version: 1,
		Checks:  map[string]config.Check{readyCheckName: {Predicate: "succeed()\n"}},
	}
	for _, err := range model.Validate() {
		if strings.Contains(err.Error(), `check "ready"`) {
			t.Errorf("Validate() error = %v, want no check error", err)
		}
	}
}

// TestModelValidateRejectsInvalidCheckPredicateSyntax proves predicate syntax
// is validated structurally from the parsed model, regardless of how the TOML
// value was quoted (here: an ordinary single-line basic string, not the
// triple-quoted form the legacy regex-based extractor required).
func TestModelValidateRejectsInvalidCheckPredicateSyntax(t *testing.T) {
	t.Parallel()

	model := config.Model{
		Version: 1,
		Checks:  map[string]config.Check{readyCheckName: {Predicate: "def broken(:"}},
	}
	var found bool
	for _, err := range model.Validate() {
		if strings.Contains(err.Error(), `check "ready"`) {
			found = true
		}
	}
	if !found {
		t.Fatal("Validate() did not report a check predicate syntax error")
	}
}

func TestModelValidateRejectsInvalidCheckPredicateSyntaxViaParse(t *testing.T) {
	t.Parallel()

	content := `
[_.lane-keeper]
version = 1

[_.lane-keeper.checks.ready]
predicate = "def broken(:"
`
	model, found, err := config.ParseAtQualifier(content, "_.lane-keeper")
	if err != nil {
		t.Fatalf("ParseAtQualifier() error = %v", err)
	}
	if !found {
		t.Fatal("ParseAtQualifier() found = false, want true")
	}
	var errFound bool
	for _, validationErr := range model.Validate() {
		if strings.Contains(validationErr.Error(), `check "ready"`) {
			errFound = true
		}
	}
	if !errFound {
		t.Fatal("Validate() did not report a syntax error for a single-line basic-string predicate")
	}
}

func TestPinnedToolVersionReadsToolsTable(t *testing.T) {
	t.Parallel()

	pin, err := config.PinnedToolVersion("[tools]\nlane-keeper = \"0.4.2\"\n")
	if err != nil {
		t.Fatalf("PinnedToolVersion() error = %v", err)
	}
	if !pin.Found || pin.Version != "0.4.2" {
		t.Errorf("PinnedToolVersion() = %#v, want Version \"0.4.2\" and Found true", pin)
	}
}

func TestPinnedToolVersionMissingIsNotOK(t *testing.T) {
	t.Parallel()

	for _, content := range []string{
		"",
		"[tools]\ngo = \"1.25\"\n",
		"[tools.lane-keeper]\nversion = \"0.4.2\"\n",
	} {
		pin, err := config.PinnedToolVersion(content)
		if err != nil {
			t.Fatalf("PinnedToolVersion(%q) error = %v", content, err)
		}
		if pin.Found {
			t.Errorf("PinnedToolVersion(%q) Found = true, want false", content)
		}
	}
}

func TestPinnedToolVersionRejectsMalformedTOML(t *testing.T) {
	t.Parallel()

	if _, err := config.PinnedToolVersion("[tools\n"); err == nil {
		t.Fatal("PinnedToolVersion() error = nil, want toml parse error")
	}
}
