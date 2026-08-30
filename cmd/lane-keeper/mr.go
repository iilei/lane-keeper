package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/iilei/lane-keeper/internal/output"
	"github.com/iilei/lane-keeper/internal/template"
)

func mr(args []string) int {
	return runMR(context.Background(), args, os.Stdout, os.Stderr, os.Getwd, os.LookupEnv)
}

func runMR(
	ctx context.Context,
	args []string,
	stdout, stderr io.Writer,
	getwd func() (string, error),
	lookupEnv func(string) (string, bool),
) int {
	if len(args) == 0 || args[0] != "render" {
		_, _ = fmt.Fprintln(stderr, "usage: lane-keeper mr render --workflow <name> [--config <path>]")
		return usageExitCode
	}

	flags := flag.NewFlagSet("mr render", flag.ContinueOnError)
	flags.SetOutput(stderr)
	workflowName := flags.String("workflow", "", "Workflow to evaluate")
	configPath := flags.String("config", "", "Path to a Mise TOML file")
	environment := flags.String("environment", "", "Optional environment input")
	ticket := flags.String("ticket", "", "Optional ticket input")
	renderVersion := flags.String("version", "", "Optional version input")
	outputFormat := flags.String("output", output.FormatText, "Output format: text or json")
	if err := flags.Parse(args[1:]); err != nil {
		return usageExitCode
	}
	if *workflowName == "" || flags.NArg() != 0 {
		_, _ = fmt.Fprintln(stderr, "usage: lane-keeper mr render --workflow <name> [--config <path>]")
		return usageExitCode
	}
	format, err := output.ParseFormat(*outputFormat)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "mr: error: %v\n", err)
		return usageExitCode
	}

	resolved, inspector, err := prepareReadiness(ctx, *workflowName, *configPath, getwd, lookupEnv)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "mr: error: %v\n", err)
		return usageExitCode
	}
	if strings.TrimSpace(resolved.MergeRequestTemplateName) == "" {
		_, _ = fmt.Fprintf(
			stderr,
			"mr: error: workflow %q does not configure a merge-request template\n",
			*workflowName,
		)
		return usageExitCode
	}

	commit, err := resolveCommitContext(ctx, inspector)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "mr: error: %v\n", err)
		return usageExitCode
	}

	templateContext := template.Context{
		Ticket:           *ticket,
		Environment:      *environment,
		Version:          *renderVersion,
		SHA:              commit.sha,
		ShortSHA:         commit.shortSHA,
		TargetBranch:     resolved.TargetBranch,
		CommitAuthorDate: commit.authorDate,
		CommitDate:       commit.commitDate,
	}
	title, err := templateContext.Render(
		resolved.MergeRequestTemplateName+"-title",
		resolved.MergeRequestTemplate.Title,
		resolved.TemplateDateFormats,
	)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "mr: error: %v\n", err)
		return usageExitCode
	}
	body, err := templateContext.Render(
		resolved.MergeRequestTemplateName+"-body",
		resolved.MergeRequestTemplate.Body,
		resolved.TemplateDateFormats,
	)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "mr: error: %v\n", err)
		return usageExitCode
	}
	title = strings.TrimSpace(title)
	body = strings.TrimSpace(body)

	if format == output.FormatJSON {
		result := output.Result{
			Status:   "ok",
			Workflow: *workflowName,
			Target:   resolved.TargetBranch,
			Data:     map[string]any{"title": title, "body": body},
		}
		if err := result.WriteJSON(stdout); err != nil {
			_, _ = fmt.Fprintf(stderr, "mr: error: %v\n", err)
			return usageExitCode
		}
		return 0
	}

	_, _ = fmt.Fprintf(stdout, "title: %s\nbody: %s\n", title, body)
	return 0
}
