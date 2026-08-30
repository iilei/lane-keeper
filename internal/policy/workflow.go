package policy

import (
	"context"
	"fmt"
	"strings"

	"github.com/iilei/lane-keeper/internal/workflow"
)

// WorkflowResult identifies the check that produced an aggregate readiness result.
type WorkflowResult struct {
	CheckName string
	Result    Result
}

// EvaluateWorkflow evaluates configured checks in order and stops at the first failure.
func EvaluateWorkflow(
	ctx context.Context,
	resolved *workflow.Resolved,
	input InputContext,
	host Host,
	limits Limits,
) (WorkflowResult, error) {
	workflowContext := WorkflowContext{
		Name:         resolved.Name,
		TargetBranch: resolved.TargetBranch,
		Remote:       resolved.Remote,
	}
	if strings.TrimSpace(resolved.SharedSource) != "" {
		sharedValue, err := CompileShared(ctx, resolved.SharedSource, limits)
		if err != nil {
			return WorkflowResult{}, fmt.Errorf("shared: %w", err)
		}
		host.Shared = sharedValue
	}
	for _, check := range resolved.Checks {
		result, err := Evaluate(ctx, check.Check.Predicate, workflowContext, input, host, limits)
		if err != nil {
			return WorkflowResult{}, fmt.Errorf("check %q: %w", check.Name, err)
		}
		if !result.Passed {
			return WorkflowResult{CheckName: check.Name, Result: result}, nil
		}
	}
	return WorkflowResult{Result: Result{Passed: true}}, nil
}
