package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/iilei/lane-keeper/internal/config"
	gitinspect "github.com/iilei/lane-keeper/internal/git"
	"github.com/iilei/lane-keeper/internal/output"
	"github.com/iilei/lane-keeper/internal/policy"
	"github.com/iilei/lane-keeper/internal/workflow"
)

const (
	checkMode = "check"
	awaitMode = "await"

	// defaultConfigQualifier is the dot-separated table path Lane-Keeper
	// configuration is nested under inside a repository's mise.toml, so Mise
	// ignores it as project metadata. See ParseAtQualifier for how an empty
	// qualifier instead reads configuration fields from the document root.
	defaultConfigQualifier = "_.lane-keeper"
)

func readiness(args []string) int {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	return runReadiness(ctx, args, os.Stdout, os.Stderr, os.Getwd, os.LookupEnv)
}

func runReadiness(
	ctx context.Context,
	args []string,
	stdout, stderr io.Writer,
	getwd func() (string, error),
	lookupEnv func(string) (string, bool),
) int {
	if len(args) == 0 || (args[0] != checkMode && args[0] != awaitMode) {
		_, _ = fmt.Fprintln(stderr, "usage: lane-keeper readiness check|await --workflow <name> [--config <path>]")
		return usageExitCode
	}
	mode := args[0]

	flags := flag.NewFlagSet("readiness "+mode, flag.ContinueOnError)
	flags.SetOutput(stderr)
	workflowName := flags.String("workflow", "", "Workflow to evaluate")
	configPath := flags.String("config", "", "Path to a Mise TOML file")
	cfgQualifier := flags.String(
		"cfg-qualifier",
		defaultConfigQualifier,
		"Dot-separated table path containing Lane-Keeper config (empty for document root)",
	)
	environment := flags.String("environment", "", "Optional environment input")
	ticket := flags.String("ticket", "", "Optional ticket input")
	outputFormat := flags.String("output", output.FormatText, "Output format: text or json")
	if err := flags.Parse(args[1:]); err != nil {
		return usageExitCode
	}
	if *workflowName == "" || flags.NArg() != 0 {
		_, _ = fmt.Fprintln(stderr, "usage: lane-keeper readiness check|await --workflow <name> [--config <path>]")
		return usageExitCode
	}
	format, err := output.ParseFormat(*outputFormat)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "readiness: error: %v\n", err)
		return usageExitCode
	}

	resolved, inspector, err := prepareReadiness(
		ctx,
		*workflowName,
		*configPath,
		*cfgQualifier,
		stderr,
		getwd,
		lookupEnv,
	)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "readiness: error: %v\n", err)
		return usageExitCode
	}
	input := policy.InputContext{Environment: *environment, Ticket: *ticket}
	host := policy.Host{Git: inspector}
	limits := policy.DefaultLimits()

	var result policy.WorkflowResult
	if mode == awaitMode {
		result, err = awaitWorkflow(ctx, &resolved, input, host, limits)
	} else {
		result, err = policy.EvaluateWorkflow(ctx, &resolved, input, host, limits)
	}
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "readiness: error: %v\n", err)
		return usageExitCode
	}
	if err := printReadiness(stdout, format, &resolved, result); err != nil {
		_, _ = fmt.Fprintf(stderr, "readiness: error: %v\n", err)
		return usageExitCode
	}
	if result.Result.Passed {
		return 0
	}
	return result.Result.ExitCode
}

// awaitWorkflow evaluates the same canonical aggregate readiness check,
// sleeping and re-evaluating until ready, timeout, or interruption. The await
// timeout is tracked independently of ctx so an expiring wall-clock deadline
// never cancels a predicate evaluation already in progress; only sleeping
// between evaluations is interrupted by ctx or the timeout.
func awaitWorkflow(
	ctx context.Context,
	resolved *workflow.Resolved,
	input policy.InputContext,
	host policy.Host,
	limits policy.Limits,
) (policy.WorkflowResult, error) {
	hasDeadline := resolved.Await.Timeout > 0
	var deadline time.Time
	if hasDeadline {
		deadline = time.Now().Add(resolved.Await.Timeout)
	}
	for {
		result, err := policy.EvaluateWorkflow(ctx, resolved, input, host, limits)
		if err != nil {
			return policy.WorkflowResult{}, err
		}
		if result.Result.Passed || !hasDeadline {
			return result, nil
		}
		wait := resolved.Await.Interval
		if remaining := time.Until(deadline); remaining <= 0 {
			return result, nil
		} else if remaining < wait {
			wait = remaining
		}
		timer := time.NewTimer(wait)
		select {
		case <-ctx.Done():
			timer.Stop()
			return result, nil
		case <-timer.C:
		}
	}
}

func prepareReadiness(
	ctx context.Context,
	workflowName, configPath, cfgQualifier string,
	stderr io.Writer,
	getwd func() (string, error),
	lookupEnv func(string) (string, bool),
) (workflow.Resolved, *gitinspect.Inspector, error) {
	workingDirectory, err := getwd()
	if err != nil {
		return workflow.Resolved{}, nil, fmt.Errorf("get working directory: %w", err)
	}
	repositoryRoot, err := gitinspect.Root(ctx, workingDirectory)
	if err != nil {
		return workflow.Resolved{}, nil, err
	}
	if configPath == "" {
		configPath = filepath.Join(repositoryRoot, "mise.toml")
	}
	content, err := os.ReadFile(configPath) //nolint:gosec // caller explicitly selects repository configuration.
	if err != nil {
		return workflow.Resolved{}, nil, fmt.Errorf("read config %q: %w", configPath, err)
	}

	// The [tools] version pin is a Mise concept; it is a no-op when absent,
	// so this is safe to attempt regardless of --config/--cfg-qualifier.
	warnOnVersionMismatch(stderr, string(content))
	model, found, err := config.ParseAtQualifier(string(content), cfgQualifier)
	if err != nil {
		return workflow.Resolved{}, nil, fmt.Errorf("config %q: %w", configPath, err)
	}
	if !found {
		return workflow.Resolved{}, nil, fmt.Errorf("config %q does not contain [%s]", configPath, cfgQualifier)
	}
	inspector := gitinspect.NewInspector(repositoryRoot)
	resolved, err := workflow.Resolve(
		ctx,
		&model,
		workflowName,
		lookupEnv,
		workflow.GitRemoteHead(repositoryRoot),
	)
	if err != nil {
		return workflow.Resolved{}, nil, err
	}
	return resolved, inspector, nil
}

// warnOnVersionMismatch prints a non-fatal advisory to stderr when the running
// binary's version differs from the repository's [tools] lane-keeper pin. A
// "dev" build (unset via ldflags) is assumed to be a local build, not a
// mismatched release, and is never warned about.
func warnOnVersionMismatch(stderr io.Writer, content string) {
	if version == "dev" {
		return
	}
	pin, err := config.PinnedToolVersion(content)
	if err != nil || !pin.Found || pin.Version == version {
		return
	}
	_, _ = fmt.Fprintf(
		stderr,
		"lane-keeper: warning: running version %s, but repository pins %s\n",
		version,
		pin.Version,
	)
}

func printReadiness(writer io.Writer, format string, resolved *workflow.Resolved, result policy.WorkflowResult) error {
	status := "ready"
	if !result.Result.Passed {
		status = "not ready"
	}
	if format == output.FormatJSON {
		data := map[string]any{}
		if result.CheckName != "" {
			data["check"] = result.CheckName
		}
		if result.Result.Reason != "" {
			data["reason"] = result.Result.Reason
		}
		return output.Result{
			Status:   status,
			Workflow: resolved.Name,
			Target:   resolved.TargetBranch,
			Data:     data,
		}.WriteJSON(writer)
	}
	_, _ = fmt.Fprintf(
		writer,
		"readiness: %s\nworkflow: %s\ntarget: %s\n",
		status,
		resolved.Name,
		resolved.TargetBranch,
	)
	if result.CheckName != "" {
		_, _ = fmt.Fprintf(writer, "check: %s\n", result.CheckName)
	}
	if result.Result.Reason != "" {
		_, _ = fmt.Fprintf(writer, "reason: %s\n", result.Result.Reason)
	}
	return nil
}
