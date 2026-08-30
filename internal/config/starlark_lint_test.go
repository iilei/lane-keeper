package config_test

import (
	"strings"
	"testing"

	"github.com/iilei/lane-keeper/internal/config"
)

func TestValidateStarlarkAcceptsPredicateControlFlow(t *testing.T) {
	t.Parallel()

	code := "if workflow.target_branch == \"main\":\n    succeed()\n"

	if err := config.ValidateStarlark(code); err != nil {
		t.Fatalf("ValidateStarlark() error = %v, want nil", err)
	}
}

func TestValidateStarlarkRejectsInvalidSyntax(t *testing.T) {
	t.Parallel()

	err := config.ValidateStarlark("succeed()\" \"\n")
	if err == nil {
		t.Fatal("ValidateStarlark() error = nil, want syntax error")
	}
	if !strings.Contains(err.Error(), "starlark syntax error") {
		t.Errorf("ValidateStarlark() error = %q, want starlark syntax error", err)
	}
}
