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
	branchNameCommand  = "name"
	environmentFlag    = "--environment"
	stagingEnvironment = "staging"
	ticketFlag         = "--ticket"
	ticketValue        = "ABC-123"
)

func TestRunBranchNameRendersAndValidatesBranch(t *testing.T) {
	paths := branchRepository(t, `{{ if .ticket }}{{ .ticket }}-{{ end }}{{ .environment }}-{{ .shortSha }}`)
	var stdout, stderr bytes.Buffer

	exitCode := runBranch(
		context.Background(),
		[]string{
			branchNameCommand, workflowFlag, releaseWorkflow, configFlag, paths.configPath,
			ticketFlag, ticketValue, environmentFlag, stagingEnvironment,
		},
		&stdout,
		&stderr,
		func() (string, error) { return paths.repositoryRoot, nil },
		noEnvironment,
	)
	if exitCode != 0 {
		t.Fatalf("runBranch() exit code = %d, want 0; stderr = %q", exitCode, stderr.String())
	}
	if got, want := strings.TrimSpace(stdout.String()), "ABC-123-staging-"; !strings.HasPrefix(got, want) {
		t.Errorf("stdout = %q, want prefix %q", got, want)
	}
}

func TestRunBranchNameRendersJSON(t *testing.T) {
	paths := branchRepository(t, `{{ if .ticket }}{{ .ticket }}-{{ end }}{{ .environment }}-{{ .shortSha }}`)
	var stdout, stderr bytes.Buffer

	exitCode := runBranch(
		context.Background(),
		[]string{
			branchNameCommand, workflowFlag, releaseWorkflow, configFlag, paths.configPath,
			ticketFlag, ticketValue, environmentFlag, stagingEnvironment, outputFlag, jsonFormat,
		},
		&stdout,
		&stderr,
		func() (string, error) { return paths.repositoryRoot, nil },
		noEnvironment,
	)
	if exitCode != 0 {
		t.Fatalf("runBranch() exit code = %d, want 0; stderr = %q", exitCode, stderr.String())
	}
	if got, want := stdout.String(), `"branchName":"ABC-123-staging-`; !strings.Contains(got, want) {
		t.Errorf("stdout = %q, want to contain %q", got, want)
	}
}

func TestRunBranchNameGoldenText(t *testing.T) {
	paths := branchRepository(t, `{{ if .ticket }}{{ .ticket }}-{{ end }}{{ .environment }}-{{ .shortSha }}`)
	var stdout, stderr bytes.Buffer

	exitCode := runBranch(
		context.Background(),
		[]string{
			branchNameCommand, workflowFlag, releaseWorkflow, configFlag, paths.configPath,
			ticketFlag, ticketValue, environmentFlag, stagingEnvironment,
		},
		&stdout,
		&stderr,
		func() (string, error) { return paths.repositoryRoot, nil },
		noEnvironment,
	)
	if exitCode != 0 {
		t.Fatalf("runBranch() exit code = %d, want 0; stderr = %q", exitCode, stderr.String())
	}
	assertGolden(t, "branch_name_text", normalizeGolden(stdout.String()))
}

func TestRunBranchNameGoldenJSON(t *testing.T) {
	paths := branchRepository(t, `{{ if .ticket }}{{ .ticket }}-{{ end }}{{ .environment }}-{{ .shortSha }}`)
	var stdout, stderr bytes.Buffer

	exitCode := runBranch(
		context.Background(),
		[]string{
			branchNameCommand, workflowFlag, releaseWorkflow, configFlag, paths.configPath,
			ticketFlag, ticketValue, environmentFlag, stagingEnvironment, outputFlag, jsonFormat,
		},
		&stdout,
		&stderr,
		func() (string, error) { return paths.repositoryRoot, nil },
		noEnvironment,
	)
	if exitCode != 0 {
		t.Fatalf("runBranch() exit code = %d, want 0; stderr = %q", exitCode, stderr.String())
	}
	assertGolden(t, "branch_name_json", normalizeGolden(stdout.String()))
}

func TestRunBranchNameRejectsInvalidRenderedRef(t *testing.T) {
	paths := branchRepository(t, `feature/{{ .environment }}//bad`)
	var stdout, stderr bytes.Buffer

	exitCode := runBranch(
		context.Background(),
		[]string{
			branchNameCommand,
			workflowFlag,
			releaseWorkflow,
			configFlag,
			paths.configPath,
			environmentFlag,
			stagingEnvironment,
		},
		&stdout,
		&stderr,
		func() (string, error) { return paths.repositoryRoot, nil },
		noEnvironment,
	)
	if exitCode != usageExitCode {
		t.Fatalf("runBranch() exit code = %d, want %d; stderr = %q", exitCode, usageExitCode, stderr.String())
	}
	if !strings.Contains(stderr.String(), "not a valid Git branch name") {
		t.Errorf("stderr = %q, want invalid branch name error", stderr.String())
	}
}

func TestRunBranchNameRequiresBranchTemplate(t *testing.T) {
	paths := noTemplateRepository(t)
	assertRequiresTemplate(t, paths, func(ctx context.Context, args []string, stdout, stderr *bytes.Buffer) int {
		return runBranch(
			ctx,
			args,
			stdout,
			stderr,
			func() (string, error) { return paths.repositoryRoot, nil },
			noEnvironment,
		)
	}, branchNameCommand, "does not configure a branch template")
}

func TestRunBranchRequiresWorkflow(t *testing.T) {
	var stdout, stderr bytes.Buffer
	exitCode := runBranch(context.Background(), []string{branchNameCommand}, &stdout, &stderr, os.Getwd, noEnvironment)
	if exitCode != usageExitCode {
		t.Errorf("runBranch() exit code = %d, want %d", exitCode, usageExitCode)
	}
}

func branchRepository(t *testing.T, branchTemplate string) readinessRepositoryPaths {
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
version = 1

[defaults]
remote = "origin"

[checks.ready]
predicate = """succeed()"""

[templates.contribution-branch]
template = "` + branchTemplate + `"

[workflows.release]
checks = ["ready"]
target_branch = { resolve = "literal", value = "master" }
branch_template = "contribution-branch"
`
	if err := os.WriteFile(configPath, []byte(content), 0o600); err != nil {
		t.Fatalf("WriteFile(): %v", err)
	}
	return readinessRepositoryPaths{repositoryRoot: repositoryRoot, configPath: configPath}
}

// noTemplateRepository returns a valid workflow with no branch or merge-request template configured.
func noTemplateRepository(t *testing.T) readinessRepositoryPaths {
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
version = 1

[defaults]
remote = "origin"

[checks.ready]
predicate = """succeed()"""

[workflows.release]
checks = ["ready"]
target_branch = { resolve = "literal", value = "master" }
`
	if err := os.WriteFile(configPath, []byte(content), 0o600); err != nil {
		t.Fatalf("WriteFile(): %v", err)
	}
	return readinessRepositoryPaths{repositoryRoot: repositoryRoot, configPath: configPath}
}
