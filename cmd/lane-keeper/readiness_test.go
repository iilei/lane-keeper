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
	checkCommand    = "check"
	configFlag      = "--config"
	releaseWorkflow = "release"
	workflowFlag    = "--workflow"
)

type readinessRepositoryPaths struct {
	repositoryRoot string
	configPath     string
}

func TestRunReadinessCheckReportsReady(t *testing.T) {
	paths := readinessRepository(t, "succeed()")
	var stdout, stderr bytes.Buffer

	exitCode := runReadiness(
		context.Background(),
		[]string{checkCommand, workflowFlag, releaseWorkflow, configFlag, paths.configPath},
		&stdout,
		&stderr,
		func() (string, error) { return paths.repositoryRoot, nil },
		noEnvironment,
	)
	if exitCode != 0 {
		t.Fatalf("runReadiness() exit code = %d, want 0; stderr = %q", exitCode, stderr.String())
	}
	if got, want := stdout.String(), "readiness: ready\nworkflow: release\ntarget: master\n"; got != want {
		t.Errorf("stdout = %q, want %q", got, want)
	}
}

func TestRunReadinessCheckPropagatesPredicateFailure(t *testing.T) {
	paths := readinessRepository(t, `fail("not ready", exit_code = 10)`)
	var stdout, stderr bytes.Buffer

	exitCode := runReadiness(
		context.Background(),
		[]string{checkCommand, workflowFlag, releaseWorkflow, configFlag, paths.configPath},
		&stdout,
		&stderr,
		func() (string, error) { return paths.repositoryRoot, nil },
		noEnvironment,
	)
	if exitCode != 10 {
		t.Fatalf("runReadiness() exit code = %d, want 10; stderr = %q", exitCode, stderr.String())
	}
	wantLines := []string{
		"readiness: not ready",
		"workflow: release",
		"target: master",
		"check: ready",
		"reason: not ready",
	}
	for _, line := range wantLines {
		if !strings.Contains(stdout.String(), line+"\n") {
			t.Errorf("stdout = %q, want line %q", stdout.String(), line)
		}
	}
}

func TestRunReadinessRequiresWorkflow(t *testing.T) {
	var stdout, stderr bytes.Buffer
	exitCode := runReadiness(context.Background(), []string{checkCommand}, &stdout, &stderr, os.Getwd, noEnvironment)
	if exitCode != usageExitCode {
		t.Errorf("runReadiness() exit code = %d, want %d", exitCode, usageExitCode)
	}
}

func readinessRepository(t *testing.T, predicate string) readinessRepositoryPaths {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git is not available")
	}
	repositoryRoot := t.TempDir()
	runGit(t, repositoryRoot, "init", "--initial-branch=master")
	configPath := filepath.Join(repositoryRoot, "lane-keeper.toml")
	content := `
[_.lane-keeper]
version = 1

[_.lane-keeper.defaults]
remote = "origin"

[_.lane-keeper.checks.ready]
predicate = """` + predicate + `"""

[_.lane-keeper.workflows.release]
checks = ["ready"]
target_branch = { resolve = "literal", value = "master" }
`
	if err := os.WriteFile(configPath, []byte(content), 0o600); err != nil {
		t.Fatalf("WriteFile(): %v", err)
	}
	return readinessRepositoryPaths{repositoryRoot: repositoryRoot, configPath: configPath}
}

func runGit(t *testing.T, repositoryRoot string, args ...string) {
	t.Helper()
	commandArgs := append([]string{"-C", repositoryRoot}, args...)
	//nolint:gosec // test helper passes arguments directly to Git without a shell.
	if output, err := exec.CommandContext(context.Background(), "git", commandArgs...).CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v: %s", args, err, output)
	}
}

func noEnvironment(string) (string, bool) {
	return "", false
}
