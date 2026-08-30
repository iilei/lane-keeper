package git_test

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	gitinspect "github.com/iilei/lane-keeper/internal/git"
)

const (
	baselineTag = "v1.0.0"
	masterRef   = "master"
)

func TestInspectorReadsRepositoryState(t *testing.T) {
	t.Parallel()

	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git is not available")
	}
	repositoryRoot := initializeRepository(t)
	inspector := gitinspect.NewInspector(repositoryRoot)
	ctx := context.Background()

	resolved, err := inspector.Resolve(ctx, "HEAD")
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
	shortSHA, err := inspector.ShortSHA(ctx, "HEAD")
	if err != nil {
		t.Fatalf("ShortSHA() error = %v", err)
	}
	if !strings.HasPrefix(resolved, shortSHA) || len(shortSHA) >= len(resolved) {
		t.Errorf("ShortSHA() = %q, want a shorter prefix of %q", shortSHA, resolved)
	}

	tag, err := inspector.LatestTag(ctx, masterRef)
	if err != nil {
		t.Fatalf("LatestTag() error = %v", err)
	}
	if !tag.Found || tag.Name != baselineTag {
		t.Errorf("LatestTag() = %#v, want Name %q and Found true", tag, baselineTag)
	}

	diff, err := inspector.Diff(ctx, baselineTag, "HEAD")
	if err != nil {
		t.Fatalf("Diff() error = %v", err)
	}
	slices.Sort(diff.Files)
	wantFiles := []string{"docs/guide.md", "notes with spaces.txt"}
	if !slices.Equal(diff.Files, wantFiles) {
		t.Errorf("Diff().Files = %q, want %q", diff.Files, wantFiles)
	}
	if diff.IsEmpty {
		t.Error("Diff().IsEmpty = true, want false")
	}

	emptyDiff, err := inspector.Diff(ctx, "HEAD", "HEAD")
	if err != nil {
		t.Fatalf("Diff(HEAD, HEAD) error = %v", err)
	}
	if len(emptyDiff.Files) != 0 {
		t.Errorf("Diff(HEAD, HEAD).Files = %q, want empty", emptyDiff.Files)
	}
	if !emptyDiff.IsEmpty {
		t.Error("Diff(HEAD, HEAD).IsEmpty = false, want true")
	}
}

func TestInspectorReadsCommitDatesAndValidatesRefFormat(t *testing.T) {
	t.Parallel()

	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git is not available")
	}
	repositoryRoot := initializeRepository(t)
	inspector := gitinspect.NewInspector(repositoryRoot)
	ctx := context.Background()

	authorDate, err := inspector.AuthorDate(ctx, "HEAD")
	if err != nil {
		t.Fatalf("AuthorDate() error = %v", err)
	}
	if authorDate.IsZero() {
		t.Error("AuthorDate() = zero time, want a resolved timestamp")
	}

	commitDate, err := inspector.CommitDate(ctx, "HEAD")
	if err != nil {
		t.Fatalf("CommitDate() error = %v", err)
	}
	if commitDate.IsZero() {
		t.Error("CommitDate() = zero time, want a resolved timestamp")
	}

	if err := inspector.CheckRefFormat(ctx, "feature/example-123"); err != nil {
		t.Errorf("CheckRefFormat(valid) error = %v", err)
	}
	if err := inspector.CheckRefFormat(ctx, "feature//bad"); err == nil {
		t.Error("CheckRefFormat(invalid) error = nil, want error")
	}
}

func TestInspectorReportsMissingReachableTag(t *testing.T) {
	t.Parallel()

	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git is not available")
	}
	repositoryRoot := t.TempDir()
	runGit(t, repositoryRoot, "init", "--initial-branch="+masterRef)
	configureIdentity(t, repositoryRoot)
	writeFile(t, repositoryRoot, "README.md", "initial\n")
	runGit(t, repositoryRoot, "add", "README.md")
	runGit(t, repositoryRoot, "commit", "-m", "initial")

	tag, err := gitinspect.NewInspector(repositoryRoot).LatestTag(context.Background(), "HEAD")
	if err != nil {
		t.Fatalf("LatestTag() error = %v", err)
	}
	if tag.Found || tag.Name != "" {
		t.Errorf("LatestTag() = %#v, want empty name and Found false", tag)
	}
}

func initializeRepository(t *testing.T) string {
	t.Helper()
	repositoryRoot := t.TempDir()
	runGit(t, repositoryRoot, "init", "--initial-branch="+masterRef)
	configureIdentity(t, repositoryRoot)
	writeFile(t, repositoryRoot, "README.md", "initial\n")
	runGit(t, repositoryRoot, "add", "README.md")
	runGit(t, repositoryRoot, "commit", "-m", "initial")
	runGit(t, repositoryRoot, "tag", "-a", baselineTag, "-m", "baseline")

	writeFile(t, repositoryRoot, "docs/guide.md", "guide\n")
	writeFile(t, repositoryRoot, "notes with spaces.txt", "notes\n")
	runGit(t, repositoryRoot, "add", "docs/guide.md", "notes with spaces.txt")
	runGit(t, repositoryRoot, "commit", "-m", "documentation")
	return repositoryRoot
}

func configureIdentity(t *testing.T, repositoryRoot string) {
	t.Helper()
	runGit(t, repositoryRoot, "config", "user.name", "Lane Keeper Test")
	runGit(t, repositoryRoot, "config", "user.email", "lane-keeper@example.invalid")
}

func writeFile(t *testing.T, repositoryRoot, name, content string) {
	t.Helper()
	path := filepath.Join(repositoryRoot, name)
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		t.Fatalf("MkdirAll(%q): %v", name, err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("WriteFile(%q): %v", name, err)
	}
}

func runGit(t *testing.T, repositoryRoot string, args ...string) {
	t.Helper()
	commandArgs := append([]string{"-C", filepath.Clean(repositoryRoot)}, args...)
	//nolint:gosec // test helper passes arguments directly to Git without a shell.
	command := exec.CommandContext(context.Background(), "git", commandArgs...)
	command.Env = isolatedGitEnvironment()
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v: %s", args, err, output)
	}
}

func isolatedGitEnvironment() []string {
	environment := make([]string, 0, len(os.Environ())+2)
	for _, variable := range os.Environ() {
		name, _, _ := strings.Cut(variable, "=")
		if !strings.HasPrefix(name, "GIT_") {
			environment = append(environment, variable)
		}
	}
	return append(environment, "GIT_CONFIG_GLOBAL="+os.DevNull, "GIT_CONFIG_NOSYSTEM=1")
}
