# `lane-keeper` — Implementation Specification

- **Status:** Draft implementation specification
- **Audience:** Human maintainers and coding agents
- **Primary design question:**
  > What makes the consuming repository easiest to understand and review?
- **Primary source:** ADR *Shared Preflight and Idempotent Merge Request Creation Assistance via `mise`*

---

## 1. Purpose

`lane-keeper` is a small repository workflow tool intended to make readiness checks, waiting, branch naming, and merge-request creation consistent between local development and GitLab CI.

Its primary goal is **not** to hide repository policy inside tooling.

Its primary goal is to provide reusable mechanics while keeping the consuming repository's workflow policy:

- visible;
- reviewable;
- close to the repository configuration;
- understandable without reading the `lane-keeper` implementation;
- identical in local use and CI.

The first adopting repository has a complex multi-environment GitLab CI setup. A merge-request pipeline may predictably fail while the leading target branch is not yet in a required repository state. That behavior is intentional and must remain authoritative in CI.

`lane-keeper` provides a local and automation-friendly way to evaluate the same precondition before filing a merge request, without adding another dynamic CI layer.

The ADR establishes that local and CI behavior must share one readiness implementation, that CI checks exactly once, and that local waiting delegates to the same predicate.

---

## 2. Design Principles

### 2.1 Consuming repository readability wins

When choosing between:

```text
simpler lane-keeper internals
```

and:

```text
clearer policy in the consuming repository
```

prefer the latter.

Implementation complexity belongs in the `lane-keeper` repository.

Repository-specific policy belongs in the consuming repository and should be obvious during code review.

### 2.2 One predicate, multiple execution policies

There is exactly one readiness predicate.

It can be consumed as:

```text
check   evaluate once
wait    evaluate repeatedly until ready
watch   evaluate asynchronously and notify on state transition
```

The predicate itself never sleeps, retries, mutates Git state, creates branches, creates merge requests, or sends notifications.

### 2.3 CI remains authoritative

Local tooling is advisory.

A successful local check means:

> The prerequisite is satisfied now, so creating the merge request is currently appropriate.

It does not guarantee that CI will remain green later.

GitLab CI performs the authoritative check at pipeline execution time.

### 2.4 Local tooling is optional

A developer must still be able to contribute without using `lane-keeper` locally.

The repository's GitLab CI remains the correctness boundary.

This follows the ADR's explicit choice to keep hooks and local tooling optional.

### 2.5 Do not build another workflow engine

`lane-keeper` is not a CI system.

Do not introduce:

- arbitrary DAGs;
- generic step arrays;
- generic shell pipelines;
- arbitrary mutation from policy scripts;
- workflow-language features that duplicate GitLab CI.

The tool owns a small number of well-defined repository workflow operations.

---

## 3. Tool Identity

### 3.1 Project name

```text
lane-keeper
```

### 3.2 Canonical executable

```text
lane-keeper
```

### 3.3 Optional Git convenience executable

The release may additionally install:

```text
git-lane-keep
```

When this executable is on `PATH`, Git exposes:

```bash
git lane-keep ...
```

The canonical namespace is still `lane-keeper`.

`git lane-keep` is convenience only and must not be required for correctness or IDE integration.

---

## 4. Distribution

`lane-keeper` should be implemented as a precompiled executable.

The implementation language is an internal concern of the tool repository. Go is a reasonable choice because it produces portable single binaries and is suitable for embedding Starlark.

The ADR requires:

- versioned distribution;
- explicit version pinning;
- Windows, macOS, and Linux support;
- platform-specific artifacts;
- SHA-256 checksums;
- no additional runtime dependency for users.

### 4.1 Preferred installation

The preferred consuming-repository installation is through `mise`.

Example:

The complete copy/merge template is maintained in
`.example.mise.toml`.

```toml
[tools]
lane-keeper = "0.1.0"
```

### 4.2 Local installation flexibility

Developers may also install `lane-keeper` globally or through another package mechanism.

Repository behavior must not depend on how the executable was installed.

CI should use the repository-pinned `mise` version.

---

## 5. Repository Configuration Location

The preferred repository configuration lives directly in:

```text
mise.toml
```

under Mise's reserved metadata table and Lane-Keeper's dedicated namespace:

```toml
[_]

[_.lane-keeper]
version = 1
```

`lane-keeper` must parse only the `[_.lane-keeper]` subtree and ignore
unrelated `mise.toml` configuration. Mise ignores the `[_]` metadata table, so
this does not produce unknown-field warnings when developers run Mise tasks.

This keeps:

```text
tool version
repository policy
workflow configuration
project-facing mise tasks
```

visible in one reviewable file.

### 5.1 Config lookup precedence

Recommended precedence:

```text
1. explicit --config path
2. repository mise.toml containing [_.lane-keeper]
3. built-in defaults
```

A dedicated `.lane-keeper.toml` is not needed for the first version.

---

## 6. Configuration Shape

The initial configuration model consists of:

```text
defaults
checks
templates
workflows
```

Example:

```toml
[tools]
lane-keeper = "0.1.0"

[_]

[_.lane-keeper]
version = 1

[_.lane-keeper.defaults]
remote = "origin"
wait_interval = "30s"

[_.lane-keeper.checks.main-ready]
description = "Whether the target branch is currently ready for this contribution"

predicate = """
starlark:
target = workflow.target_branch
baseline = git.latest_tag(target)

if baseline == None:
    fail("no baseline tag found on " + target)

diff = git.diff(baseline, target)

if diff.is_empty:
    fail("no relevant changes since " + baseline)

pass({
    "baseline": baseline,
    "target": target,
})
"""

[_.lane-keeper.templates.contribution-branch]
template = """
{{ if .ticket }}{{ .ticket }}-{{ end }}{{ .yyMMdd }}-{{ .environment }}-{{ .shortSha }}
"""

[_.lane-keeper.templates.merge-request-message]
title = "{{ if .ticket }}{{ .ticket }}: {{ end }}Prepare {{ .environment }} contribution"
body = """
Source commit: {{ .shortSha }}
Target branch: {{ .target_branch }}
"""

[_.lane-keeper.workflows.deploy]
description = "Prepare a deployment-oriented contribution"
checks = ["main-ready"]
remote = "origin"
target_branch_resolver = "git-remote-head"
branch_template = "contribution-branch"
merge_request_template = "merge-request-message"

[_.lane-keeper.workflows.deploy.wait]
interval = "30s"

[_.lane-keeper.workflows.deploy.watch]
notify = true
identity = [
  "repository",
  "workflow",
  "source_sha",
  "environment",
]
notify_on = [
  "not_ready->ready",
  "not_ready->error",
]

[tasks."check:preflight-main"]
run = "lane-keeper preflight check deploy"

[tasks."wait:preflight-main"]
run = "lane-keeper preflight wait deploy"

[tasks."tmpl:branch-name"]
run = "lane-keeper branch name deploy"

[tasks."tmpl:merge-req-message"]
run = "lane-keeper mr render deploy"

[tasks."git:create-mr-when-ready"]
run = "lane-keeper mr create-when-ready deploy"
```

---

## 7. Starlark as Repository Policy Notation

Repository-specific readiness logic should be expressed in embedded Starlark.

The purpose of Starlark is:

```text
make policy obvious in the consuming repository
```

not:

```text
turn lane-keeper into a generic scripting host
```

### 7.1 Why Starlark

Starlark is suitable because it is:

- Python-like and readable;
- intentionally constrained;
- embeddable in the compiled tool;
- controllable by the host;
- suitable for small policy predicates.

A developer reviewing the consuming repository should be able to understand the readiness rule without reading Go code.

### 7.2 Predicate field type

`predicate` must always be a TOML string.

Supported source forms:

```text
starlark:<inline source>
starlark+file:<path>
```

Inline example:

```toml
predicate = """
starlark:
target = workflow.target_branch

if git.latest_tag(target) == None:
    fail("no baseline tag found")

pass()
"""
```

File-backed example:

```toml
predicate = "starlark+file:.lane-keep/main-ready.star"
```

This intentionally borrows the useful idea of a source-locator scheme while keeping the TOML type stable.

### 7.3 No arbitrary OS access

Starlark must not receive unrestricted access to:

- operating-system commands;
- arbitrary filesystem access;
- network access;
- environment mutation;
- Git mutation;
- GitLab mutation.

The host exposes only deliberate read-only repository primitives.

---

## 8. Starlark Execution Context

The first useful execution context should remain small.

### 8.1 `workflow`

Read-only workflow configuration:

```python
workflow.name
workflow.target_branch
workflow.remote
```

### 8.2 `input`

Invocation-specific input:

```python
input.environment
input.ticket
```

Nullable values should be represented as:

```python
None
```

### 8.3 `git`

Read-only repository inspection API.

Initial primitives:

```python
git.resolve(ref)
git.short_sha(ref)
git.latest_tag(ref)
git.diff(from_ref, to_ref)
```

The exact API should grow only in response to real repository policy needs.

### 8.4 Diff result

A `git.diff(...)` result should initially expose only what is required.

Example:

```python
diff.is_empty
```

Possible later additions:

```python
diff.files
diff.commit_count
```

Do not add them until required.

### 8.5 Result functions

The predicate ends through:

```python
pass()
pass({...})

fail("reason")
fail("reason", {...})
```

The host converts the result into:

```text
status
human-readable message
structured metadata
exit code
```

---

## 9. Command Surface

### 9.1 Check

```bash
lane-keeper preflight check <workflow>
```

Example:

```bash
lane-keeper preflight check deploy \
  --environment staging \
  --ticket ABC-123
```

Semantics:

```text
resolve config
resolve workflow
resolve invocation inputs
evaluate each configured predicate once, in order
print result
return exit status
```

This is the CI-safe operation.

### 9.2 Wait

```bash
lane-keeper preflight wait <workflow>
```

Semantics:

```text
evaluate the same ordered checks
if not ready:
    sleep configured interval
    repeat
stop when ready or interrupted
```

`wait` must not implement a second version of the aggregate preflight.

This preserves the ADR invariant that waiting delegates to the canonical readiness logic.

### 9.3 Watch

```bash
lane-keeper preflight watch <workflow> --notify
```

Semantics:

```text
resolve immutable invocation context
calculate watch identity
reuse equivalent active watch if present
otherwise dispatch background watcher
return promptly

background watcher:
  evaluate same ordered checks
    sleep between attempts
    detect state transitions
    notify on configured transition
```

The first notification transition of interest is:

```text
NOT_READY -> READY
```

`watch` is intended as an optional bridge for IDE-first developers.

It must not require an IDE plugin.

### 9.4 Branch name

```bash
lane-keeper branch name <workflow>
```

This is a pure operation.

It must:

```text
resolve source commit
build template context
render template
validate Git ref
print canonical branch name
```

No remote mutation.

### 9.5 Create MR when ready

```bash
lane-keeper mr create-when-ready <workflow>
```

This is the high-level orchestration command.

Conceptual flow:

```text
freeze source SHA
wait for preflight
render canonical branch name
ensure remote branch
ensure matching MR
return MR URL
```

The ADR requires the source SHA to be frozen at invocation start so a later change to `HEAD` does not alter the contribution identity while waiting.

---

## 10. Built-in Defaults

Defaults should describe mechanics, not team-specific policy.

Reasonable defaults:

```text
remote                 origin
wait interval          30s
notification backend   system
branch template        [ticket-]date-environment-shortSha
```

Default branch template:

```text
{{ if .ticket }}{{ .ticket }}-{{ end }}{{ .yyMMdd }}-{{ .environment }}-{{ .shortSha }}
```

The ADR already establishes that the branch identity should be deterministic and should include source SHA by default.

Do not hardcode:

```text
main
staging
specific ticket regex
specific tag format
specific deployment semantics
```

unless they are explicit workflow configuration.

---

## 11. Template Context

At minimum:

```json
{
  "ticket": "ABC-123",
  "environment": "staging",
  "sha": "a83d0219...",
  "shortSha": "a83d021",
  "yyMMdd": "260829",
  "HHmm": "0610"
}
```

The timestamp must derive from the immutable source commit's committer timestamp, not wall-clock time.

This ensures retries against the same source commit produce the same branch identity.

### 11.1 Template precedence

```text
CLI override
> workflow-selected repository template
> built-in default
```

### 11.2 Validation

Before remote mutation, validate the rendered branch name as a valid Git ref.

### 11.3 Merge-request message templates

A workflow may select a repository message template through
`merge_request_template`. A message template has separate `title` and `body`
fields; both are rendered with the template context. The context additionally
includes `target_branch` after target-branch resolution.

```toml
[_.lane-keeper.templates.merge-request-message]
title = "{{ .ticket }}: prepare contribution"
body = """
Source commit: {{ .shortSha }}
Target branch: {{ .target_branch }}
"""
```

`lane-keeper mr render <workflow>` is a pure operation that renders and prints
the title and body before any remote mutation. Repositories may expose it as:

```toml
[tasks."tmpl:merge-req-message"]
run = "lane-keeper mr render deploy"
```

The title and body MUST remain separate structured fields. Lane-Keeper MUST NOT
split one rendered template on blank lines: a Markdown body commonly contains
multiple paragraphs, so two consecutive newlines are not an unambiguous
delimiter.

---

## 12. Branch Idempotency

Branch creation must use PUT-like semantics even if the GitLab API itself is POST-based.

Given:

```text
branch name
source SHA
```

behavior must be:

```text
branch absent
    -> create branch at source SHA
    -> success

branch exists at expected SHA
    -> reuse
    -> success

branch exists at different SHA
    -> explicit conflict
    -> failure
```

The tool must never silently move an existing branch.

This behavior is required by the ADR.

---

## 13. Merge Request Idempotency

MR creation must also be retry-safe.

Behavior:

```text
matching open MR absent
    -> create MR

matching open MR exists
    -> reuse MR
    -> return existing URL
```

Repeated invocation by:

```text
human
LLM
retrying shell
automation
```

must not create duplicate branches or duplicate MRs.

---

## 14. Watch Idempotency

Repeated IDE actions should not create duplicate background pollers.

Default logical watch identity:

```text
repository
workflow
source SHA
environment
```

The exact fields may be configurable:

```toml
[lane-keeper.workflows.deploy.watch]
identity = [
  "repository",
  "workflow",
  "source_sha",
  "environment",
]
```

### 6.1 Target branch resolution

Each workflow MUST declare exactly one target-branch field:

```text
target_branch_literal
target_branch_resolver
```

`target_branch_literal` contains a valid literal branch name.

`target_branch_resolver` selects a Lane-Keeper-defined resolver. Initially,
`git-remote-head` resolves the configured remote's symbolic `HEAD` reference,
such as `refs/remotes/origin/HEAD`, then uses its branch component. This is the
remote's default branch: the leading branch proposed for a new merge request.
Resolution failure is a configuration/repository error; Lane-Keeper MUST NOT
silently substitute `main` or `master`.

Configuring both fields, or neither field, is invalid. Resolver names are a
closed Lane-Keeper API: configuration MUST NOT contain arbitrary shell commands
or scripts.

The resolved branch becomes `workflow.target_branch` for Starlark evaluation.
This lets a repository use a stable declarative alias while retaining the
actual branch name in output and result metadata.

### 6.2 Workflow checks

Each workflow MUST declare a non-empty, ordered `checks` array. Each entry
names a configured check.

```toml
[_.lane-keeper.workflows.deploy]
checks = ["branch-ready", "required-tag-present"]
```

Preflight evaluates each named predicate once, in array order. Evaluation stops
at the first not-ready result or error. A workflow is ready only when every
check passes. `wait` and `watch` MUST reuse this aggregate preflight evaluation;
they must not implement a separate per-check execution path.

Equivalent active watch:

```text
reuse/report existing watch
```

No equivalent active watch:

```text
create watcher
```

A persistent daemon is not required initially.

Prefer:

```text
one detached process per watch
```

until real usage demonstrates a need for a daemon.

---

## 15. Notifications

Notifications are best-effort convenience.

They must never alter predicate semantics or CI status.

Initial backend:

```text
system
```

Initial transition:

```text
NOT_READY -> READY
```

Optional:

```text
NOT_READY -> ERROR
```

Repeated unchanged states do not notify.

Example:

```text
lane-keeper

staging contribution is ready

main now satisfies the configured preflight condition.
```

---

## 16. `mise` Task Surface

The repository-facing task names may remain opinionated and team-specific even though the tool API is more generic.

Example:

```toml
[tasks."check:preflight-main"]
run = "lane-keeper preflight check deploy"

[tasks."wait:preflight-main"]
run = "lane-keeper preflight wait deploy"

[tasks."tmpl:branch-name"]
run = "lane-keeper branch name deploy"

[tasks."git:create-mr-when-ready"]
run = "lane-keeper mr create-when-ready deploy"
```

This preserves the ADR's intended split between the team-owned tool and repository-owned task exposure.

---

## 17. GitLab CI Integration

GitLab CI should remain intentionally boring.

Example:

```yaml
preflight-main:
  script:
    - mise run check:preflight-main
```

The CI task must invoke:

```text
preflight check
```

exactly once.

It must never use:

```text
wait
watch
notification dispatch
```

The CI invocation and the local troubleshooting invocation must reach the same Starlark predicate.

---

## 18. Versioning

Two versions matter:

```text
tool version
config schema version
```

Example:

```toml
[tools]
lane-keeper = "0.4.2"

[lane-keeper]
version = 1
```

The tool must reject unsupported configuration schema versions.

For a globally installed local binary, a different tool version may produce an advisory warning if the repository pins another compatible version.

CI should use the pinned version.

---

## 19. Error Model

Use stable exit statuses.

Suggested initial contract:

```text
0   predicate passed / operation succeeded
1   predicate not ready / expected workflow failure
2   invocation or configuration error
3   repository state conflict
4   external service/API failure
```

The exact values may change before v1, but once published they become part of the versioned CLI contract.

The ADR explicitly treats task names, arguments, exit statuses, and machine-readable output as a versioned interface.

---

## 20. Output

Human-readable output should be the default.

Example:

```text
preflight: not ready
workflow: deploy
target: main
reason: no baseline tag found on main
```

Ready:

```text
preflight: ready
workflow: deploy
target: main
baseline: v1.42.0
```

A machine-readable mode should be planned:

```bash
lane-keeper ... --output json
```

Example:

```json
{
  "status": "ready",
  "workflow": "deploy",
  "target": "main",
  "data": {
    "baseline": "v1.42.0"
  }
}
```

Do not expose unstable internal implementation details.

---

## 21. Security Boundaries

The Starlark runtime is repository policy evaluation, not general scripting.

Therefore:

```text
allowed:
    read-only Git inspection
    workflow data
    invocation data
    pass/fail

not allowed by default:
    arbitrary process execution
    arbitrary network requests
    arbitrary filesystem writes
    Git mutation
    GitLab mutation
    sleeping/polling
    notifications
```

Mutation belongs to compiled `lane-keeper` operations after policy evaluation.

---

## 22. Non-goals

Do not implement in the initial product:

- a generic CI DSL;
- arbitrary workflow graphs;
- arbitrary shell execution from Starlark;
- IDE plugins;
- a permanent daemon;
- Git hook enforcement;
- mandatory local use;
- automatic modification of `main`;
- policy hidden in opaque built-ins when it can reasonably remain in repository config.

---

## 23. First Implementation Increment

The first increment should validate the architectural premise with minimal scope.

Implement:

```text
1. precompiled lane-keeper executable
2. CLI parser
3. locate repository root
4. read [lane-keeper] from mise.toml
5. validate config schema version
6. resolve one workflow
7. parse inline starlark: predicate
8. expose minimal read-only git API
9. implement pass()/fail()
10. implement `preflight check`
11. stable human-readable output
12. stable initial exit-code contract
13. automated tests
```

Do not implement yet:

```text
wait
watch
notifications
templates
branch creation
GitLab API
MR creation
git-lane-keep alias
```

### 23.1 Acceptance criteria

Given:

```toml
[_]

[_.lane-keeper]
version = 1

[_.lane-keeper.checks.main-ready]
predicate = """
target = workflow.target_branch

if git.latest_tag(target) == None:
    fail("no baseline tag")

pass()
"""

[_.lane-keeper.workflows.deploy]
checks = ["main-ready"]
target_branch_literal = "main"
```

this command:

```bash
lane-keeper preflight check deploy
```

must:

```text
- load the repository config;
- resolve workflow.deploy;
- expose workflow.target_branch to Starlark;
- expose git.latest_tag(ref);
- evaluate each configured predicate once;
- return 0 on pass;
- return 1 on not-ready;
- print the fail reason when not-ready;
- perform no mutation.
```

A repository `mise` task:

```toml
[tasks."check:preflight-main"]
run = "lane-keeper preflight check deploy"
```

must invoke exactly the same implementation locally and in GitLab CI.

This proves the ADR's central no-drift requirement.

---

## 24. Second Increment

After the first increment is proven:

```text
1. preflight wait
2. configurable wait interval
3. interrupt handling
4. tests proving wait delegates to check/predicate implementation
```

No second predicate implementation is allowed.

---

## 25. Third Increment

Add branch identity:

```text
1. template config
2. built-in default template
3. template context generation
4. immutable source SHA resolution
5. committer timestamp extraction
6. branch ref validation
7. `branch name`
```

Still no remote mutation required.

---

## 26. Fourth Increment

Add GitLab mutation:

```text
1. ensure remote branch
2. explicit SHA conflict detection
3. ensure matching MR
4. return MR URL
5. `mr create-when-ready`
```

Preserve the ADR's idempotent semantics.

---

## 27. Fifth Increment

Add asynchronous developer convenience:

```text
1. preflight watch
2. watch identity
3. detached process
4. minimal watch state
5. native system notification
6. transition-based notification
```

No IDE plugin required.

---

## 28. Testing Strategy

### Unit tests

Cover:

```text
config parsing
schema-version rejection
workflow resolution
predicate source parsing
Starlark context
pass/fail result conversion
Git API wrappers
template rendering
branch validation
watch identity
```

### Integration tests

Use temporary Git repositories to verify:

```text
tag lookup
diff behavior
source SHA freezing
branch naming
idempotent branch behavior
```

### GitLab API tests

Abstract GitLab transport behind a small interface.

Test:

```text
branch absent
branch same SHA
branch wrong SHA
MR absent
MR existing
API failure
```

without requiring live GitLab for normal test runs.

### Golden tests

Use golden output for:

```text
human-readable diagnostics
JSON output
```

Keep messages deliberate because policy visibility is part of the product.

---

## 29. Implementation Boundaries

A suggested internal decomposition:

```text
cmd/
    lane-keeper/

internal/
    config/
    repo/
    policy/
    starlark/
    git/
    template/
    workflow/
    gitlab/
    watch/
    notify/
    output/
```

Do not force this exact package layout if the implementation language or codebase suggests a simpler structure.

The important boundaries are:

```text
policy evaluation
repository mechanics
mutation/orchestration
presentation
```

---

## 30. Core Invariants

The implementation must preserve all of the following:

1. Repository policy remains visible in the consuming repository.
2. Readiness has exactly one implementation per configured predicate.
3. CI evaluates each configured predicate once and never waits.
4. Local wait delegates to the same aggregate preflight.
5. Watch delegates to the same aggregate preflight.
6. Preflight policy is read-only.
7. Local tooling is optional.
8. CI remains authoritative.
9. The source SHA is frozen before waiting/orchestration.
10. Branch naming is deterministic.
11. Existing branch at wrong SHA is a conflict.
12. MR creation is idempotent.
13. Notification delivery is non-authoritative.
14. `lane-keeper` is the canonical executable.
15. `git lane-keep` is optional convenience.
16. `mise` is the preferred reproducibility layer, not a mandatory IDE integration layer.
17. Configuration must not evolve into a generic CI workflow language.
18. The consuming repository should remain easier to understand than the tool implementation.

---

## 31. Guiding Review Question

For every new feature, configuration field, Starlark API function, or built-in behavior, ask:

> Does this make the consuming repository easier to understand and review?

If the answer is no, prefer the simpler public model even if the `lane-keeper` implementation becomes more sophisticated internally.

That is the primary architectural criterion for this project.
