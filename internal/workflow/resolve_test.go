package workflow_test

import (
	"context"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/iilei/lane-keeper/internal/config"
	"github.com/iilei/lane-keeper/internal/workflow"
)

const (
	firstCheckName   = "first"
	mainBranchName   = "main"
	masterBranchName = "master"
	originRemoteName = "origin"
	secondCheckName  = "second"
	successPredicate = "succeed()"
)

func TestResolveLiteralWorkflowMergesDefaultsAndPreservesCheckOrder(t *testing.T) {
	t.Parallel()

	model := validModel()
	configured := model.Workflows["release"]
	configured.TargetBranch = config.TargetBranch{Resolve: "literal", Value: masterBranchName}
	model.Workflows["release"] = configured

	resolved, err := workflow.Resolve(context.Background(), model, "release", noEnvironment, nil)
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
	if got, want := resolved.Remote, originRemoteName; got != want {
		t.Errorf("Remote = %q, want %q", got, want)
	}
	if got, want := resolved.TargetBranch, masterBranchName; got != want {
		t.Errorf("TargetBranch = %q, want %q", got, want)
	}
	if got, want := resolved.Checks[0].Name, firstCheckName; got != want {
		t.Errorf("Checks[0].Name = %q, want %q", got, want)
	}
	if got, want := resolved.Checks[1].Name, secondCheckName; got != want {
		t.Errorf("Checks[1].Name = %q, want %q", got, want)
	}
	if got, want := resolved.Await.Interval, 30*time.Second; got != want {
		t.Errorf("Await.Interval = %s, want %s", got, want)
	}
}

func TestGitRemoteHeadResolvesSymbolicRemoteReference(t *testing.T) {
	t.Parallel()

	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git is not available")
	}
	repositoryRoot := t.TempDir()
	runGit(t, repositoryRoot, "init")
	runGit(t, repositoryRoot, "symbolic-ref", "refs/remotes/origin/HEAD", "refs/remotes/origin/master")

	branch, err := workflow.GitRemoteHead(repositoryRoot)(context.Background(), originRemoteName)
	if err != nil {
		t.Fatalf("GitRemoteHead() error = %v", err)
	}
	if got, want := branch, masterBranchName; got != want {
		t.Errorf("branch = %q, want %q", got, want)
	}
}

func TestResolveUsesGitRemoteHead(t *testing.T) {
	t.Parallel()

	model := validModel()
	resolved, err := workflow.Resolve(
		context.Background(),
		model,
		"release",
		noEnvironment,
		func(_ context.Context, remote string) (string, error) {
			if got, want := remote, originRemoteName; got != want {
				t.Errorf("remote = %q, want %q", got, want)
			}
			return mainBranchName, nil
		},
	)
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
	if got, want := resolved.TargetBranch, mainBranchName; got != want {
		t.Errorf("TargetBranch = %q, want %q", got, want)
	}
}

func validModel() *config.Model {
	return &config.Model{
		Version: 1,
		Defaults: config.Defaults{
			Remote:        originRemoteName,
			AwaitInterval: "30s",
			AwaitTimeout:  "30m",
		},
		Checks: map[string]config.Check{
			firstCheckName:  {Predicate: successPredicate},
			secondCheckName: {Predicate: successPredicate},
		},
		Workflows: map[string]config.Workflow{
			"release": {
				Checks:       []string{firstCheckName, secondCheckName},
				TargetBranch: config.TargetBranch{Resolve: "git-remote-head"},
			},
		},
	}
}

func noEnvironment(string) (string, bool) {
	return "", false
}

func runGit(t *testing.T, repositoryRoot string, args ...string) {
	t.Helper()
	commandArgs := append([]string{"-C", filepath.Clean(repositoryRoot)}, args...)
	//nolint:gosec // test helper passes arguments directly to Git without a shell.
	if output, err := exec.CommandContext(context.Background(), "git", commandArgs...).CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v: %s", args, err, output)
	}
}
