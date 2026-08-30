package main

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

const (
	mrRenderCommand = "render"
	mrTitleTemplate = `{{ if .ticket }}{{ .ticket }}: {{ end }}Prepare {{ .environment }} contribution`
	mrBodyTemplate  = "Source commit: {{ .shortSha }}\nTarget branch: {{ .targetBranch }}"
)

func TestRunMRRenderRendersTitleAndBody(t *testing.T) {
	paths := mrRepository(t)
	var stdout, stderr bytes.Buffer

	exitCode := runMR(
		context.Background(),
		[]string{
			mrRenderCommand, workflowFlag, releaseWorkflow, configFlag, paths.configPath,
			ticketFlag, ticketValue, environmentFlag, stagingEnvironment,
		},
		&stdout,
		&stderr,
		func() (string, error) { return paths.repositoryRoot, nil },
		noEnvironment,
	)
	if exitCode != 0 {
		t.Fatalf("runMR() exit code = %d, want 0; stderr = %q", exitCode, stderr.String())
	}
	if got, want := stdout.String(), "title: ABC-123: Prepare staging contribution\n"; !strings.Contains(got, want) {
		t.Errorf("stdout = %q, want to contain %q", got, want)
	}
	if got, want := stdout.String(), "Target branch: master"; !strings.Contains(got, want) {
		t.Errorf("stdout = %q, want to contain %q", got, want)
	}
}

func TestRunMRRenderRendersJSON(t *testing.T) {
	paths := mrRepository(t)
	var stdout, stderr bytes.Buffer

	exitCode := runMR(
		context.Background(),
		[]string{
			mrRenderCommand, workflowFlag, releaseWorkflow, configFlag, paths.configPath,
			ticketFlag, ticketValue, environmentFlag, stagingEnvironment, outputFlag, jsonFormat,
		},
		&stdout,
		&stderr,
		func() (string, error) { return paths.repositoryRoot, nil },
		noEnvironment,
	)
	if exitCode != 0 {
		t.Fatalf("runMR() exit code = %d, want 0; stderr = %q", exitCode, stderr.String())
	}
	if got, want := stdout.String(), `"title":"ABC-123: Prepare staging contribution"`; !strings.Contains(got, want) {
		t.Errorf("stdout = %q, want to contain %q", got, want)
	}
}

func TestRunMRRenderGoldenText(t *testing.T) {
	paths := mrRepository(t)
	var stdout, stderr bytes.Buffer

	exitCode := runMR(
		context.Background(),
		[]string{
			mrRenderCommand, workflowFlag, releaseWorkflow, configFlag, paths.configPath,
			ticketFlag, ticketValue, environmentFlag, stagingEnvironment,
		},
		&stdout,
		&stderr,
		func() (string, error) { return paths.repositoryRoot, nil },
		noEnvironment,
	)
	if exitCode != 0 {
		t.Fatalf("runMR() exit code = %d, want 0; stderr = %q", exitCode, stderr.String())
	}
	assertGolden(t, "mr_render_text", normalizeGolden(stdout.String()))
}

func TestRunMRRenderGoldenJSON(t *testing.T) {
	paths := mrRepository(t)
	var stdout, stderr bytes.Buffer

	exitCode := runMR(
		context.Background(),
		[]string{
			mrRenderCommand, workflowFlag, releaseWorkflow, configFlag, paths.configPath,
			ticketFlag, ticketValue, environmentFlag, stagingEnvironment, outputFlag, jsonFormat,
		},
		&stdout,
		&stderr,
		func() (string, error) { return paths.repositoryRoot, nil },
		noEnvironment,
	)
	if exitCode != 0 {
		t.Fatalf("runMR() exit code = %d, want 0; stderr = %q", exitCode, stderr.String())
	}
	assertGolden(t, "mr_render_json", normalizeGolden(stdout.String()))
}

func TestRunMRRenderRequiresMergeRequestTemplate(t *testing.T) {
	paths := noTemplateRepository(t)
	assertRequiresTemplate(t, paths, func(ctx context.Context, args []string, stdout, stderr *bytes.Buffer) int {
		return runMR(
			ctx,
			args,
			stdout,
			stderr,
			func() (string, error) { return paths.repositoryRoot, nil },
			noEnvironment,
		)
	}, mrRenderCommand, "does not configure a merge-request template")
}

func TestRunMRRequiresWorkflow(t *testing.T) {
	var stdout, stderr bytes.Buffer
	exitCode := runMR(context.Background(), []string{mrRenderCommand}, &stdout, &stderr, os.Getwd, noEnvironment)
	if exitCode != usageExitCode {
		t.Errorf("runMR() exit code = %d, want %d", exitCode, usageExitCode)
	}
}

func mrRepository(t *testing.T) readinessRepositoryPaths {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git is not available")
	}
	repositoryRoot := t.TempDir()
	runGit(t, repositoryRoot, "init", "--initial-branch=master")
	runGit(t, repositoryRoot, "config", "user.email", "test@example.com")
	runGit(t, repositoryRoot, "config", "user.name", "Test")
	runGit(t, repositoryRoot, "commit", "--allow-empty", "-m", "init")
	configPath := filepath.Join(repositoryRoot, "lane-keeper.toml")
	content := `
[_.lane-keeper]
version = 1

[_.lane-keeper.defaults]
remote = "origin"

[_.lane-keeper.checks.ready]
predicate = """succeed()"""

[_.lane-keeper.templates.merge-request-message]
title = "` + mrTitleTemplate + `"
body = "` + strings.ReplaceAll(mrBodyTemplate, "\n", `\n`) + `"

[_.lane-keeper.workflows.release]
checks = ["ready"]
target_branch = { resolve = "literal", value = "master" }
merge_request_template = "merge-request-message"
`
	if err := os.WriteFile(configPath, []byte(content), 0o600); err != nil {
		t.Fatalf("WriteFile(): %v", err)
	}
	return readinessRepositoryPaths{repositoryRoot: repositoryRoot, configPath: configPath}
}
