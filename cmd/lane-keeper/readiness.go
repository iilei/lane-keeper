package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/iilei/lane-keeper/internal/config"
	gitinspect "github.com/iilei/lane-keeper/internal/git"
	"github.com/iilei/lane-keeper/internal/policy"
	"github.com/iilei/lane-keeper/internal/workflow"
)

func readiness(args []string) int {
	return runReadiness(context.Background(), args, os.Stdout, os.Stderr, os.Getwd, os.LookupEnv)
}

func runReadiness(
	ctx context.Context,
	args []string,
	stdout, stderr io.Writer,
	getwd func() (string, error),
	lookupEnv func(string) (string, bool),
) int {
	if len(args) == 0 || args[0] != "check" {
		_, _ = fmt.Fprintln(stderr, "usage: lane-keeper readiness check --workflow <name> [--config <path>]")
		return usageExitCode
	}

	flags := flag.NewFlagSet("readiness check", flag.ContinueOnError)
	flags.SetOutput(stderr)
	workflowName := flags.String("workflow", "", "Workflow to evaluate")
	configPath := flags.String("config", "", "Path to a Mise TOML file")
	environment := flags.String("environment", "", "Optional environment input")
	ticket := flags.String("ticket", "", "Optional ticket input")
	if err := flags.Parse(args[1:]); err != nil {
		return usageExitCode
	}
	if *workflowName == "" || flags.NArg() != 0 {
		_, _ = fmt.Fprintln(stderr, "usage: lane-keeper readiness check --workflow <name> [--config <path>]")
		return usageExitCode
	}

	resolved, inspector, err := prepareReadiness(ctx, *workflowName, *configPath, getwd, lookupEnv)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "readiness: error: %v\n", err)
		return usageExitCode
	}
	result, err := policy.EvaluateWorkflow(
		ctx,
		&resolved,
		policy.InputContext{Environment: *environment, Ticket: *ticket},
		policy.Host{Git: inspector},
		policy.DefaultLimits(),
	)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "readiness: error: %v\n", err)
		return usageExitCode
	}
	printReadiness(stdout, &resolved, result)
	if result.Result.Passed {
		return 0
	}
	return result.Result.ExitCode
}

func prepareReadiness(
	ctx context.Context,
	workflowName, configPath string,
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
	parsed, err := config.Parse(string(content))
	if err != nil {
		return workflow.Resolved{}, nil, err
	}
	if !parsed.Found {
		return workflow.Resolved{}, nil, fmt.Errorf("config %q does not contain [_.lane-keeper]", configPath)
	}
	inspector := gitinspect.NewInspector(repositoryRoot)
	resolved, err := workflow.Resolve(
		ctx,
		parsed.Model,
		workflowName,
		lookupEnv,
		workflow.GitRemoteHead(repositoryRoot),
	)
	if err != nil {
		return workflow.Resolved{}, nil, err
	}
	return resolved, inspector, nil
}

func printReadiness(output io.Writer, resolved *workflow.Resolved, result policy.WorkflowResult) {
	status := "ready"
	if !result.Result.Passed {
		status = "not ready"
	}
	_, _ = fmt.Fprintf(
		output,
		"readiness: %s\nworkflow: %s\ntarget: %s\n",
		status,
		resolved.Name,
		resolved.TargetBranch,
	)
	if result.CheckName != "" {
		_, _ = fmt.Fprintf(output, "check: %s\n", result.CheckName)
	}
	if result.Result.Reason != "" {
		_, _ = fmt.Fprintf(output, "reason: %s\n", result.Result.Reason)
	}
}
