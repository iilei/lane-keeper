package main

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

const (
	awaitCommand    = "await"
	checkCommand    = "check"
	configFlag      = "--config"
	jsonFormat      = "json"
	outputFlag      = "--output"
	releaseWorkflow = "release"
	workflowFlag    = "--workflow"
)

type readinessRepositoryPaths struct {
	repositoryRoot string
	configPath     string
}

// assertRequiresTemplate exercises a rendering command against a workflow with no
// template configured and asserts it fails with the expected diagnostic.
func assertRequiresTemplate(
	t *testing.T,
	paths readinessRepositoryPaths,
	run func(ctx context.Context, args []string, stdout, stderr *bytes.Buffer) int,
	command, wantErrSubstring string,
) {
	t.Helper()
	var stdout, stderr bytes.Buffer
	exitCode := run(
		context.Background(),
		[]string{command, workflowFlag, releaseWorkflow, configFlag, paths.configPath},
		&stdout,
		&stderr,
	)
	if exitCode != usageExitCode {
		t.Fatalf("exit code = %d, want %d; stderr = %q", exitCode, usageExitCode, stderr.String())
	}
	if !strings.Contains(stderr.String(), wantErrSubstring) {
		t.Errorf("stderr = %q, want to contain %q", stderr.String(), wantErrSubstring)
	}
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

func TestRunReadinessCheckReportsReadyAsJSON(t *testing.T) {
	paths := readinessRepository(t, "succeed()")
	var stdout, stderr bytes.Buffer

	exitCode := runReadiness(
		context.Background(),
		[]string{checkCommand, workflowFlag, releaseWorkflow, configFlag, paths.configPath, outputFlag, jsonFormat},
		&stdout,
		&stderr,
		func() (string, error) { return paths.repositoryRoot, nil },
		noEnvironment,
	)
	if exitCode != 0 {
		t.Fatalf("runReadiness() exit code = %d, want 0; stderr = %q", exitCode, stderr.String())
	}
	if got, want := stdout.String(), `"status":"ready"`; !strings.Contains(got, want) {
		t.Errorf("stdout = %q, want to contain %q", got, want)
	}
	if got, want := stdout.String(), `"workflow":"release"`; !strings.Contains(got, want) {
		t.Errorf("stdout = %q, want to contain %q", got, want)
	}
}

func TestRunReadinessWarnsOnToolVersionMismatch(t *testing.T) {
	const testRunningVersion = "1.0.0"
	paths := readinessRepositoryWithToolsPin(t, "succeed()", "9.9.9")
	previousVersion := version
	version = testRunningVersion
	t.Cleanup(func() { version = previousVersion })

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
	if got, want := stderr.String(), "running version 1.0.0, but repository pins 9.9.9"; !strings.Contains(got, want) {
		t.Errorf("stderr = %q, want to contain %q", got, want)
	}
}

func TestRunReadinessSilentOnMatchingToolVersion(t *testing.T) {
	const testRunningVersion = "1.0.0"
	paths := readinessRepositoryWithToolsPin(t, "succeed()", testRunningVersion)
	previousVersion := version
	version = testRunningVersion
	t.Cleanup(func() { version = previousVersion })

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
	if strings.Contains(stderr.String(), "warning") {
		t.Errorf("stderr = %q, want no version warning", stderr.String())
	}
}

func TestRunReadinessRejectsUnknownOutputFormat(t *testing.T) {
	paths := readinessRepository(t, "succeed()")
	var stdout, stderr bytes.Buffer

	exitCode := runReadiness(
		context.Background(),
		[]string{checkCommand, workflowFlag, releaseWorkflow, configFlag, paths.configPath, outputFlag, "xml"},
		&stdout,
		&stderr,
		func() (string, error) { return paths.repositoryRoot, nil },
		noEnvironment,
	)
	if exitCode != usageExitCode {
		t.Fatalf("runReadiness() exit code = %d, want %d; stderr = %q", exitCode, usageExitCode, stderr.String())
	}
}

func TestRunReadinessCheckReadyGoldenText(t *testing.T) {
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
	assertGolden(t, "readiness_check_ready_text", stdout.String())
}

func TestRunReadinessCheckReadyGoldenJSON(t *testing.T) {
	paths := readinessRepository(t, "succeed()")
	var stdout, stderr bytes.Buffer

	exitCode := runReadiness(
		context.Background(),
		[]string{checkCommand, workflowFlag, releaseWorkflow, configFlag, paths.configPath, outputFlag, jsonFormat},
		&stdout,
		&stderr,
		func() (string, error) { return paths.repositoryRoot, nil },
		noEnvironment,
	)
	if exitCode != 0 {
		t.Fatalf("runReadiness() exit code = %d, want 0; stderr = %q", exitCode, stderr.String())
	}
	assertGolden(t, "readiness_check_ready_json", stdout.String())
}

func TestRunReadinessCheckNotReadyGoldenText(t *testing.T) {
	paths := readinessRepository(t, `fail("no baseline tag found", exit_code = 10)`)
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
	assertGolden(t, "readiness_check_not_ready_text", stdout.String())
}

func TestRunReadinessCheckNotReadyGoldenJSON(t *testing.T) {
	paths := readinessRepository(t, `fail("no baseline tag found", exit_code = 10)`)
	var stdout, stderr bytes.Buffer

	exitCode := runReadiness(
		context.Background(),
		[]string{checkCommand, workflowFlag, releaseWorkflow, configFlag, paths.configPath, outputFlag, jsonFormat},
		&stdout,
		&stderr,
		func() (string, error) { return paths.repositoryRoot, nil },
		noEnvironment,
	)
	if exitCode != 10 {
		t.Fatalf("runReadiness() exit code = %d, want 10; stderr = %q", exitCode, stderr.String())
	}
	assertGolden(t, "readiness_check_not_ready_json", stdout.String())
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

func TestRunReadinessAwaitTimesOutOnPersistentFailure(t *testing.T) {
	paths := readinessRepositoryWithAwait(t, `fail("not ready", exit_code = 10)`, "10ms", "40ms")
	var stdout, stderr bytes.Buffer

	exitCode := runReadiness(
		context.Background(),
		[]string{awaitCommand, workflowFlag, releaseWorkflow, configFlag, paths.configPath},
		&stdout,
		&stderr,
		func() (string, error) { return paths.repositoryRoot, nil },
		noEnvironment,
	)
	if exitCode != 10 {
		t.Fatalf("runReadiness() exit code = %d, want 10; stderr = %q", exitCode, stderr.String())
	}
	if !strings.Contains(stdout.String(), "readiness: not ready\n") {
		t.Errorf("stdout = %q, want not-ready status", stdout.String())
	}
}

func TestRunReadinessAwaitDelegatesToCanonicalCheckUntilReady(t *testing.T) {
	predicate := `
if git.latest_tag(workflow.target_branch) == None:
    fail("no baseline tag found", exit_code = 10)
succeed()
`
	paths := readinessRepositoryWithAwait(t, predicate, "10ms", "2s")
	tagCreated := make(chan error, 1)
	go func() {
		time.Sleep(30 * time.Millisecond)
		//nolint:gosec // test helper passes arguments directly to Git without a shell.
		tagCreated <- exec.CommandContext(context.Background(), "git", "-C", paths.repositoryRoot, "tag", "-m", "v1.0.0", "v1.0.0").Run()
	}()

	var stdout, stderr bytes.Buffer
	exitCode := runReadiness(
		context.Background(),
		[]string{awaitCommand, workflowFlag, releaseWorkflow, configFlag, paths.configPath},
		&stdout,
		&stderr,
		func() (string, error) { return paths.repositoryRoot, nil },
		noEnvironment,
	)
	if err := <-tagCreated; err != nil {
		t.Fatalf("create tag: %v", err)
	}
	if exitCode != 0 {
		t.Fatalf("runReadiness() exit code = %d, want 0; stderr = %q", exitCode, stderr.String())
	}
	if !strings.Contains(stdout.String(), "readiness: ready\n") {
		t.Errorf("stdout = %q, want ready status", stdout.String())
	}
}

func TestRunReadinessAwaitStopsPromptlyOnCancellation(t *testing.T) {
	paths := readinessRepositoryWithAwait(t, `fail("not ready", exit_code = 10)`, "1h", "1h")
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(20 * time.Millisecond)
		cancel()
	}()

	start := time.Now()
	var stdout, stderr bytes.Buffer
	exitCode := runReadiness(
		ctx,
		[]string{awaitCommand, workflowFlag, releaseWorkflow, configFlag, paths.configPath},
		&stdout,
		&stderr,
		func() (string, error) { return paths.repositoryRoot, nil },
		noEnvironment,
	)
	if elapsed := time.Since(start); elapsed > 500*time.Millisecond {
		t.Errorf("runReadiness() took %v, want prompt cancellation", elapsed)
	}
	if exitCode != 10 {
		t.Fatalf("runReadiness() exit code = %d, want 10; stderr = %q", exitCode, stderr.String())
	}
}

func readinessRepositoryWithAwait(t *testing.T, predicate, interval, timeout string) readinessRepositoryPaths {
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
predicate = """` + predicate + `"""

[_.lane-keeper.workflows.release]
checks = ["ready"]
target_branch = { resolve = "literal", value = "master" }

[_.lane-keeper.workflows.release.await]
interval = "` + interval + `"
timeout = "` + timeout + `"
`
	if err := os.WriteFile(configPath, []byte(content), 0o600); err != nil {
		t.Fatalf("WriteFile(): %v", err)
	}
	return readinessRepositoryPaths{repositoryRoot: repositoryRoot, configPath: configPath}
}

func readinessRepositoryWithToolsPin(t *testing.T, predicate, pinnedVersion string) readinessRepositoryPaths {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git is not available")
	}
	repositoryRoot := t.TempDir()
	runGit(t, repositoryRoot, "init", "--initial-branch=master")
	configPath := filepath.Join(repositoryRoot, "lane-keeper.toml")
	content := `
[tools]
lane-keeper = "` + pinnedVersion + `"

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
