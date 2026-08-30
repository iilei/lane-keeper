package policy_test

import (
	"context"
	"testing"

	"github.com/iilei/lane-keeper/internal/config"
	"github.com/iilei/lane-keeper/internal/policy"
	"github.com/iilei/lane-keeper/internal/workflow"
)

const (
	firstCheckName  = "first"
	releaseWorkflow = "release"
	secondCheckName = "second"
	succeedSource   = "succeed()\n"
)

func TestEvaluateWorkflowStopsAtFirstFailure(t *testing.T) {
	t.Parallel()

	resolved := workflow.Resolved{
		Name:         releaseWorkflow,
		TargetBranch: mainBranchName,
		Remote:       originRemote,
		Checks: []workflow.NamedCheck{
			{Name: firstCheckName, Check: config.Check{Predicate: "fail(\"not ready\", exit_code = 10)\n"}},
			{Name: secondCheckName, Check: config.Check{Predicate: "fail(\"must not run\")\n"}},
		},
	}
	result, err := policy.EvaluateWorkflow(
		context.Background(),
		&resolved,
		policy.InputContext{},
		policy.Host{},
		policy.DefaultLimits(),
	)
	if err != nil {
		t.Fatalf("EvaluateWorkflow() error = %v", err)
	}
	if got, want := result.CheckName, firstCheckName; got != want {
		t.Errorf("CheckName = %q, want %q", got, want)
	}
	if result.Result.Passed || result.Result.ExitCode != 10 {
		t.Errorf("Result = %#v, want not-ready exit code 10", result.Result)
	}
}

func TestEvaluateWorkflowPassesAfterAllChecks(t *testing.T) {
	t.Parallel()

	resolved := workflow.Resolved{
		Checks: []workflow.NamedCheck{
			{Name: firstCheckName, Check: config.Check{Predicate: succeedSource}},
			{Name: secondCheckName, Check: config.Check{Predicate: succeedSource}},
		},
	}
	result, err := policy.EvaluateWorkflow(
		context.Background(),
		&resolved,
		policy.InputContext{},
		policy.Host{},
		policy.DefaultLimits(),
	)
	if err != nil {
		t.Fatalf("EvaluateWorkflow() error = %v", err)
	}
	if !result.Result.Passed || result.CheckName != "" {
		t.Errorf("EvaluateWorkflow() = %#v, want aggregate success", result)
	}
}

func TestEvaluateWorkflowSharesCompiledSharedAcrossChecks(t *testing.T) {
	t.Parallel()

	const checksSharedValue = "if shared.value == \"shared-value\":\n    succeed()\nfail(\"missing shared value\")\n"
	resolved := workflow.Resolved{
		SharedSource: "value = \"shared-value\"\n",
		Checks: []workflow.NamedCheck{
			{Name: firstCheckName, Check: config.Check{Predicate: checksSharedValue}},
			{Name: secondCheckName, Check: config.Check{Predicate: checksSharedValue}},
		},
	}
	result, err := policy.EvaluateWorkflow(
		context.Background(),
		&resolved,
		policy.InputContext{},
		policy.Host{},
		policy.DefaultLimits(),
	)
	if err != nil {
		t.Fatalf("EvaluateWorkflow() error = %v", err)
	}
	if !result.Result.Passed || result.CheckName != "" {
		t.Errorf("EvaluateWorkflow() = %#v, want aggregate success", result)
	}
}

func TestEvaluateWorkflowRejectsSharedHostAPIReferences(t *testing.T) {
	t.Parallel()

	resolved := workflow.Resolved{
		SharedSource: "succeed()\n",
		Checks: []workflow.NamedCheck{
			{Name: firstCheckName, Check: config.Check{Predicate: succeedSource}},
		},
	}
	_, err := policy.EvaluateWorkflow(
		context.Background(),
		&resolved,
		policy.InputContext{},
		policy.Host{},
		policy.DefaultLimits(),
	)
	if err == nil {
		t.Fatal("EvaluateWorkflow() error = nil, want shared compile error")
	}
}
