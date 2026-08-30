// Package policy evaluates bounded, read-only Lane-Keeper Starlark predicates.
package policy

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/canonical/starlark/starlark"
	"github.com/canonical/starlark/starlarkstruct"
	"github.com/canonical/starlark/syntax"

	gitinspect "github.com/iilei/lane-keeper/internal/git"
)

const (
	defaultMaxSteps  = 100_000
	defaultMaxAllocs = 16 * 1024 * 1024
	defaultDeadline  = time.Second
	requiredSafety   = starlark.CPUSafe | starlark.MemSafe | starlark.TimeSafe | starlark.IOSafe
)

type (
	// WorkflowContext is the immutable workflow data visible to predicates.
	WorkflowContext struct {
		Name         string
		TargetBranch string
		Remote       string
	}

	// InputContext is invocation-specific data visible to predicates.
	InputContext struct {
		Environment string
		Ticket      string
	}

	// GitInspector is the read-only repository API available to predicates.
	GitInspector interface {
		Resolve(context.Context, string) (string, error)
		ShortSHA(context.Context, string) (string, error)
		LatestTag(context.Context, string) (gitinspect.TagResult, error)
		Diff(context.Context, string, string) (gitinspect.Diff, error)
	}

	// Host contains optional read-only services exposed to a predicate.
	Host struct {
		Git GitInspector
	}

	// Result is the terminal readiness decision returned by a predicate.
	Result struct {
		Passed   bool
		Reason   string
		ExitCode int
	}

	// Limits bounds one predicate evaluation.
	Limits struct {
		MaxSteps  int64
		MaxAllocs int64
		Deadline  time.Duration
	}

	resultSignal struct {
		result Result
	}
)

func (signal *resultSignal) Error() string {
	return "predicate returned a readiness result"
}

// DefaultLimits returns the built-in predicate execution limits.
func DefaultLimits() Limits {
	return Limits{MaxSteps: defaultMaxSteps, MaxAllocs: defaultMaxAllocs, Deadline: defaultDeadline}
}

// Evaluate executes one predicate until succeed() or fail() terminates it.
func Evaluate(
	ctx context.Context,
	source string,
	workflow WorkflowContext,
	input InputContext,
	host Host,
	limits Limits,
) (Result, error) {
	limits = normalizedLimits(limits)
	evaluationContext, cancel := context.WithTimeout(ctx, limits.Deadline)
	defer cancel()

	thread := &starlark.Thread{
		Name: "lane-keeper predicate",
		Print: func(thread *starlark.Thread, _ string) {
			thread.Cancel("print() is unavailable in readiness predicates")
		},
	}
	thread.SetParentContext(evaluationContext)
	thread.SetMaxSteps(limits.MaxSteps)
	thread.SetMaxAllocs(limits.MaxAllocs)
	defer thread.Cancel("evaluation complete")

	predeclared, err := predeclaredValues(thread, workflow, input, host)
	if err != nil {
		return Result{}, fmt.Errorf("build predicate context: %w", err)
	}
	thread.RequireSafety(requiredSafety)
	options := syntax.FileOptions{TopLevelControl: true}
	_, err = starlark.ExecFileOptions(&options, thread, "predicate.star", source, predeclared)
	if err == nil {
		return Result{}, errors.New("predicate completed without calling succeed() or fail()")
	}
	var signal *resultSignal
	if errors.As(err, &signal) {
		return signal.result, nil
	}
	return Result{}, fmt.Errorf("evaluate predicate: %w", err)
}

func predeclaredValues(
	thread *starlark.Thread,
	workflow WorkflowContext,
	input InputContext,
	host Host,
) (starlark.StringDict, error) {
	workflowValue, err := starlarkstruct.SafeFromStringDict(thread, starlark.String("workflow"), starlark.StringDict{
		"name":          starlark.String(workflow.Name),
		"target_branch": starlark.String(workflow.TargetBranch),
		"remote":        starlark.String(workflow.Remote),
	})
	if err != nil {
		return nil, err
	}
	inputValue, err := starlarkstruct.SafeFromStringDict(thread, starlark.String("input"), starlark.StringDict{
		"environment": nullableString(input.Environment),
		"ticket":      nullableString(input.Ticket),
	})
	if err != nil {
		return nil, err
	}
	predeclared := starlark.StringDict{
		"workflow": workflowValue,
		"input":    inputValue,
		"succeed":  starlark.NewBuiltinWithSafety("succeed", requiredSafety, succeed),
		"fail":     starlark.NewBuiltinWithSafety("fail", requiredSafety, fail),
	}
	if host.Git != nil {
		gitValue, err := newGitValue(thread, host.Git)
		if err != nil {
			return nil, err
		}
		predeclared["git"] = gitValue
	}
	return predeclared, nil
}

func succeed(
	_ *starlark.Thread,
	function *starlark.Builtin,
	args starlark.Tuple,
	kwargs []starlark.Tuple,
) (starlark.Value, error) {
	if err := starlark.UnpackArgs(function.Name(), args, kwargs); err != nil {
		return nil, err
	}
	return nil, &resultSignal{result: Result{Passed: true}}
}

func fail(
	_ *starlark.Thread,
	function *starlark.Builtin,
	args starlark.Tuple,
	kwargs []starlark.Tuple,
) (starlark.Value, error) {
	var reason string
	exitCode := 1
	if err := starlark.UnpackArgs(
		function.Name(),
		args,
		kwargs,
		"reason",
		&reason,
		"exit_code?",
		&exitCode,
	); err != nil {
		return nil, err
	}
	if reason == "" {
		return nil, fmt.Errorf("%s: reason must not be empty", function.Name())
	}
	if exitCode < 1 || exitCode > 250 {
		return nil, fmt.Errorf("%s: exit_code must be between 1 and 250", function.Name())
	}
	return nil, &resultSignal{result: Result{Reason: reason, ExitCode: exitCode}}
}

func nullableString(value string) starlark.Value {
	if value == "" {
		return starlark.None
	}
	return starlark.String(value)
}

func normalizedLimits(limits Limits) Limits {
	defaults := DefaultLimits()
	if limits.MaxSteps <= 0 {
		limits.MaxSteps = defaults.MaxSteps
	}
	if limits.MaxAllocs <= 0 {
		limits.MaxAllocs = defaults.MaxAllocs
	}
	if limits.Deadline <= 0 {
		limits.Deadline = defaults.Deadline
	}
	return limits
}
