package config_test

import (
	"strings"
	"testing"
	"time"

	"github.com/iilei/lane-keeper/internal/config"
)

const (
	validLaneKeeperConfig = `
[tools]
go = "1.25"

[_.lane-keeper]
version = 1

[_.lane-keeper.defaults]
remote = "origin"
await_interval = "30s"
await_timeout = "30m"

[_.lane-keeper.checks.ready]
predicate = "succeed()"

[_.lane-keeper.templates.branch]
template = "{{ .shortSha }}"

[_.lane-keeper.templates.message]
title = "Ready"
body = "Body"

[_.lane-keeper.workflows.release]
checks = ["ready"]
target_branch = { resolve = "git-remote-head" }
branch_template = "branch"
merge_request_template = "message"

[_.lane-keeper.workflows.release.await]
interval = "10s"
timeout = "0s"
`
	longAwaitTimeout = "48h"
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

func TestParseIgnoresUnrelatedMiseConfiguration(t *testing.T) {
	t.Parallel()

	result, err := config.Parse("[tools]\ngo = \"1.25\"\n")
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	if result.Found {
		t.Errorf("Parse() result = %#v, want not found", result)
	}
}

func TestParseRejectsUnknownLaneKeeperField(t *testing.T) {
	t.Parallel()

	_, err := config.Parse("[_.lane-keeper]\nversion = 1\nunknown = true\n")
	if err == nil {
		t.Fatal("Parse() error = nil, want unknown-field error")
	}
}

func TestModelValidatesCompleteConfiguration(t *testing.T) {
	t.Parallel()

	result, err := config.Parse(validLaneKeeperConfig)
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	if errs := result.Model.Validate(); len(errs) > 0 {
		t.Fatalf("Validate() errors = %v, want none", errs)
	}
}

func TestModelReportsSemanticErrors(t *testing.T) {
	t.Parallel()

	content := strings.NewReplacer(
		"version = 1",
		"version = 2",
		"await_interval = \"30s\"",
		"await_interval = \"never\"\n\n[_.lane-keeper.defaults.template_date_formats]\nyyMMdd = \"20060102\"",
		"checks = [\"ready\"]",
		"checks = [\"missing\", \"missing\"]",
		"target_branch = { resolve = \"git-remote-head\" }",
		"target_branch = { resolve = \"literal\" }",
		"branch_template = \"branch\"",
		"branch_template = \"missing\"",
	).Replace(validLaneKeeperConfig)

	result, err := config.Parse(content)
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	errs := result.Model.Validate()
	for _, expected := range []string{
		"version must be 1",
		"invalid duration",
		"conflicts with a built-in layout",
		"unknown check",
		"duplicate check",
		"target_branch.value is required",
		"unknown branch template",
	} {
		if !errorsContain(errs, expected) {
			t.Errorf("Validate() errors = %v, want error containing %q", errs, expected)
		}
	}
}

func TestResolveAwaitSettingsUsesWorkflowValuesAndAllowsZeroTimeout(t *testing.T) {
	t.Parallel()

	result, err := config.Parse(validLaneKeeperConfig)
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	settings, err := result.Model.ResolveAwaitSettings("release", noEnvironment)
	if err != nil {
		t.Fatalf("ResolveAwaitSettings() error = %v", err)
	}
	if got, want := settings.Interval, 10*time.Second; got != want {
		t.Errorf("Interval = %s, want %s", got, want)
	}
	if got := settings.Timeout; got != 0 {
		t.Errorf("Timeout = %s, want 0", got)
	}
}

func TestResolveAwaitSettingsRequiresUnsafeMaximumForLongEnvironmentTimeout(t *testing.T) {
	t.Parallel()

	result, err := config.Parse(validLaneKeeperConfig)
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	environment := map[string]string{
		config.AwaitIntervalEnvironment:         "5s",
		config.AwaitTimeoutEnvironment:          longAwaitTimeout,
		config.AllowLongAwaitMaximumEnvironment: "172800",
	}
	settings, err := result.Model.ResolveAwaitSettings("release", mapEnvironment(environment))
	if err != nil {
		t.Fatalf("ResolveAwaitSettings() error = %v", err)
	}
	if got, want := settings.Interval, 5*time.Second; got != want {
		t.Errorf("Interval = %s, want %s", got, want)
	}
	if got, want := settings.Timeout, 48*time.Hour; got != want {
		t.Errorf("Timeout = %s, want %s", got, want)
	}
}

func TestResolveAwaitSettingsRejectsLongTimeoutWithoutUnsafeMaximum(t *testing.T) {
	t.Parallel()

	result, err := config.Parse(validLaneKeeperConfig)
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	_, err = result.Model.ResolveAwaitSettings("release", mapEnvironment(map[string]string{
		config.AwaitTimeoutEnvironment: longAwaitTimeout,
	}))
	if err == nil || !strings.Contains(err.Error(), "must not exceed 24h0m0s") {
		t.Fatalf("ResolveAwaitSettings() error = %v, want 24-hour maximum error", err)
	}
}

func TestResolveAwaitSettingsRejectsMalformedUnsafeMaximum(t *testing.T) {
	t.Parallel()

	result, err := config.Parse(validLaneKeeperConfig)
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	_, err = result.Model.ResolveAwaitSettings("release", mapEnvironment(map[string]string{
		config.AllowLongAwaitMaximumEnvironment: longAwaitTimeout,
	}))
	if err == nil || !strings.Contains(err.Error(), "positive integer number of seconds") {
		t.Fatalf("ResolveAwaitSettings() error = %v, want integer-seconds error", err)
	}
}

func errorsContain(errs []error, expected string) bool {
	for _, err := range errs {
		if strings.Contains(err.Error(), expected) {
			return true
		}
	}
	return false
}

func noEnvironment(string) (string, bool) {
	return "", false
}

func mapEnvironment(environment map[string]string) func(string) (string, bool) {
	return func(name string) (string, bool) {
		value, ok := environment[name]
		return value, ok
	}
}
