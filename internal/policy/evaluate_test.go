package policy_test

import (
	"context"
	"strings"
	"testing"
	"time"

	gitinspect "github.com/iilei/lane-keeper/internal/git"
	"github.com/iilei/lane-keeper/internal/policy"
)

const (
	mainBranchName = "main"
	originRemote   = "origin"
	releaseName    = "release"
	testTicket     = "ABC-123"
)

type fakeGitInspector struct {
	resolved string
	shortSHA string
	tag      gitinspect.TagResult
	diff     gitinspect.Diff
	block    bool
}

func TestEvaluateReturnsSuccessImmediately(t *testing.T) {
	t.Parallel()

	result, err := policy.Evaluate(
		context.Background(),
		"if workflow.target_branch == \"main\":\n    succeed()\nfail(\"unreachable\")\n",
		policy.WorkflowContext{Name: releaseName, TargetBranch: mainBranchName, Remote: originRemote},
		policy.InputContext{},
		policy.Host{},
		policy.DefaultLimits(),
	)
	if err != nil {
		t.Fatalf("Evaluate() error = %v", err)
	}
	if !result.Passed {
		t.Errorf("Evaluate() result = %#v, want passed", result)
	}
}

func TestEvaluateReturnsFailureReasonAndExitCode(t *testing.T) {
	t.Parallel()

	result, err := policy.Evaluate(
		context.Background(),
		"fail(\"not ready\", exit_code = 10)\n",
		policy.WorkflowContext{},
		policy.InputContext{},
		policy.Host{},
		policy.DefaultLimits(),
	)
	if err != nil {
		t.Fatalf("Evaluate() error = %v", err)
	}
	if result.Passed || result.Reason != "not ready" || result.ExitCode != 10 {
		t.Errorf("Evaluate() result = %#v, want not-ready exit code 10", result)
	}
}

func TestEvaluateExposesNullableInput(t *testing.T) {
	t.Parallel()

	result, err := policy.Evaluate(
		context.Background(),
		"if input.environment == None and input.ticket == \"ABC-123\":\n    succeed()\nfail(\"wrong input\")\n",
		policy.WorkflowContext{},
		policy.InputContext{Ticket: testTicket},
		policy.Host{},
		policy.DefaultLimits(),
	)
	if err != nil {
		t.Fatalf("Evaluate() error = %v", err)
	}
	if !result.Passed {
		t.Errorf("Evaluate() result = %#v, want passed", result)
	}
}

func TestEvaluateRejectsMissingResult(t *testing.T) {
	t.Parallel()

	_, err := policy.Evaluate(
		context.Background(),
		"value = 1\n",
		policy.WorkflowContext{},
		policy.InputContext{},
		policy.Host{},
		policy.DefaultLimits(),
	)
	if err == nil || !strings.Contains(err.Error(), "without calling succeed() or fail()") {
		t.Fatalf("Evaluate() error = %v, want missing result error", err)
	}
}

func TestEvaluateRejectsLoad(t *testing.T) {
	t.Parallel()

	_, err := policy.Evaluate(
		context.Background(),
		"load(\"module.star\", \"value\")\nsucceed()\n",
		policy.WorkflowContext{},
		policy.InputContext{},
		policy.Host{},
		policy.DefaultLimits(),
	)
	if err == nil {
		t.Fatal("Evaluate() error = nil, want load error")
	}
}

func TestEvaluateRejectsPrint(t *testing.T) {
	t.Parallel()

	_, err := policy.Evaluate(
		context.Background(),
		"print(\"side effect\")\nsucceed()\n",
		policy.WorkflowContext{},
		policy.InputContext{},
		policy.Host{},
		policy.DefaultLimits(),
	)
	if err == nil || !strings.Contains(err.Error(), "print() is unavailable") {
		t.Fatalf("Evaluate() error = %v, want print unavailable error", err)
	}
}

func TestEvaluateRejectsInvalidFailureResult(t *testing.T) {
	t.Parallel()

	for _, source := range []string{"fail(\"\")\n", "fail(\"reason\", exit_code = 251)\n"} {
		_, err := policy.Evaluate(
			context.Background(),
			source,
			policy.WorkflowContext{},
			policy.InputContext{},
			policy.Host{},
			policy.DefaultLimits(),
		)
		if err == nil {
			t.Errorf("Evaluate(%q) error = nil, want invalid result error", source)
		}
	}
}

func TestEvaluateEnforcesStepLimit(t *testing.T) {
	t.Parallel()

	_, err := policy.Evaluate(
		context.Background(),
		"values = [item for item in range(100000)]\nsucceed()\n",
		policy.WorkflowContext{},
		policy.InputContext{},
		policy.Host{},
		policy.Limits{MaxSteps: 100, MaxAllocs: 1024 * 1024, Deadline: time.Second},
	)
	if err == nil {
		t.Fatal("Evaluate() error = nil, want step limit error")
	}
}

func TestEvaluateEnforcesAllocationLimit(t *testing.T) {
	t.Parallel()

	_, err := policy.Evaluate(
		context.Background(),
		"succeed()\n",
		policy.WorkflowContext{Name: releaseName, TargetBranch: mainBranchName, Remote: originRemote},
		policy.InputContext{Environment: "staging", Ticket: testTicket},
		policy.Host{},
		policy.Limits{MaxSteps: 1000, MaxAllocs: 1, Deadline: time.Second},
	)
	if err == nil {
		t.Fatal("Evaluate() error = nil, want allocation limit error")
	}
}

func TestEvaluateExposesReadOnlyGitInspection(t *testing.T) {
	t.Parallel()

	inspector := &fakeGitInspector{
		resolved: "0123456789abcdef",
		shortSHA: "0123456",
		tag:      gitinspect.TagResult{Name: "v1.0.0", Found: true},
		diff:     gitinspect.Diff{Files: []string{"README.md", "docs/guide.md"}},
	}
	source := `
if git.resolve("HEAD") != "0123456789abcdef":
    fail("wrong resolved SHA")
if git.short_sha("HEAD") != "0123456":
    fail("wrong short SHA")
baseline = git.latest_tag(workflow.target_branch)
if baseline == None:
    fail("missing baseline")
diff = git.diff(baseline, workflow.target_branch)
if diff.is_empty or diff.files != ["README.md", "docs/guide.md"]:
    fail("wrong diff")
succeed()
`
	result, err := policy.Evaluate(
		context.Background(),
		source,
		policy.WorkflowContext{TargetBranch: mainBranchName},
		policy.InputContext{},
		policy.Host{Git: inspector},
		policy.DefaultLimits(),
	)
	if err != nil {
		t.Fatalf("Evaluate() error = %v", err)
	}
	if !result.Passed {
		t.Errorf("Evaluate() result = %#v, want passed", result)
	}
}

func TestEvaluateRepresentsMissingTagAsNone(t *testing.T) {
	t.Parallel()

	result, err := policy.Evaluate(
		context.Background(),
		"if git.latest_tag(\"HEAD\") == None:\n    succeed()\nfail(\"unexpected tag\")\n",
		policy.WorkflowContext{},
		policy.InputContext{},
		policy.Host{Git: &fakeGitInspector{}},
		policy.DefaultLimits(),
	)
	if err != nil {
		t.Fatalf("Evaluate() error = %v", err)
	}
	if !result.Passed {
		t.Errorf("Evaluate() result = %#v, want passed", result)
	}
}

func TestEvaluatePropagatesDeadlineToGitInspection(t *testing.T) {
	t.Parallel()

	_, err := policy.Evaluate(
		context.Background(),
		"git.resolve(\"HEAD\")\nsucceed()\n",
		policy.WorkflowContext{},
		policy.InputContext{},
		policy.Host{Git: &fakeGitInspector{block: true}},
		policy.Limits{MaxSteps: 1000, MaxAllocs: 1024 * 1024, Deadline: time.Millisecond},
	)
	if err == nil || !strings.Contains(err.Error(), context.DeadlineExceeded.Error()) {
		t.Fatalf("Evaluate() error = %v, want deadline exceeded", err)
	}
}

func TestCompileSharedExposesFunctionsAndData(t *testing.T) {
	t.Parallel()

	sharedValue, err := policy.CompileShared(
		context.Background(),
		"benign_patterns = [\"*.md\"]\n\ndef matches(name, patterns):\n    return name in patterns\n",
		policy.DefaultLimits(),
	)
	if err != nil {
		t.Fatalf("CompileShared() error = %v", err)
	}

	result, err := policy.Evaluate(
		context.Background(),
		"if shared.matches(\"*.md\", shared.benign_patterns):\n    succeed()\nfail(\"shared lookup failed\")\n",
		policy.WorkflowContext{},
		policy.InputContext{},
		policy.Host{Shared: sharedValue},
		policy.DefaultLimits(),
	)
	if err != nil {
		t.Fatalf("Evaluate() error = %v", err)
	}
	if !result.Passed {
		t.Errorf("Evaluate() result = %#v, want passed", result)
	}
}

func TestCompileSharedRejectsHostAPIReferences(t *testing.T) {
	t.Parallel()

	for _, source := range []string{"workflow.name", "git.resolve(\"HEAD\")", "succeed()", "fail(\"x\")"} {
		_, err := policy.CompileShared(context.Background(), source, policy.DefaultLimits())
		if err == nil {
			t.Errorf("CompileShared(%q) error = nil, want undefined-name error", source)
		}
	}
}

func (inspector *fakeGitInspector) Resolve(ctx context.Context, _ string) (string, error) {
	if inspector.block {
		<-ctx.Done()
		return "", ctx.Err()
	}
	return inspector.resolved, nil
}

func (inspector *fakeGitInspector) ShortSHA(context.Context, string) (string, error) {
	return inspector.shortSHA, nil
}

func (inspector *fakeGitInspector) LatestTag(context.Context, string) (gitinspect.TagResult, error) {
	return inspector.tag, nil
}

func (inspector *fakeGitInspector) Diff(context.Context, string, string) (gitinspect.Diff, error) {
	return inspector.diff, nil
}
