# `lane-keeper` — Implementation Specification

- **Status:** Draft implementation specification
- **Audience:** Human maintainers and coding agents
- **Primary design question:**
  > What makes the consuming repository easiest to understand and review?
- **Originating ADR:** *Shared Preflight and Idempotent Merge Request Creation Assistance via `mise`*
- **Scope note:** this specification supersedes the ADR's mutation scope; Lane-Keeper is read-only.

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

The design establishes that local and CI behavior must share one readiness implementation, that CI checks exactly once, and that local awaiting delegates to the same predicate.

### 1.1 Current implementation boundary

This document specifies the target design unless a section explicitly says
otherwise. The current executable implements `version` and
`config-introspection`; `readiness`, `branch`, and `mr` remain command stubs.

Current config introspection can:

- parse TOML;
- strictly decode `[_.lane-keeper]` while ignoring unrelated Mise keys;
- validate schema version, durations, date layouts, template shapes, workflow
  checks, target-branch resolver combinations, and template references;
- locate ordinary triple-quoted predicates beneath
  `[_.lane-keeper.checks.<name>]`;
- parse extracted predicates with Canonical Starlark without executing them;
- format extracted predicates through an external `buildifier` executable;
- validate and preview custom template date layouts.

It does not yet resolve workflows or Git state, execute Starlark, expose the
planned host API, evaluate readiness, or render branch and merge-request
templates. Predicate extraction is currently limited to the documented
ordinary triple-quoted representation; a TOML-structure-driven extractor
remains planned.

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
await   evaluate repeatedly until ready
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
- no additional runtime dependency for core commands or `--lint`.

`config-introspection --fmt` deliberately invokes external Buildifier. The
published formatting hook installs it in the hook's isolated environment.

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
await_interval = "30s"
await_timeout = "30m"

[_.lane-keeper.defaults.template_date_formats]
releaseStamp = "2006.01.02"

[_.lane-keeper.checks.main-ready]
description = "Whether the target branch is currently ready for this contribution"

predicate = """
target = workflow.target_branch
baseline = git.latest_tag(target)

if baseline == None:
    fail("no baseline tag found on %s" % target)

diff = git.diff(baseline, target)

if diff.is_empty:
    succeed()

succeed()
"""

[_.lane-keeper.templates.contribution-branch]
template = """
{{ if .ticket }}{{ .ticket }}-{{ end }}{{ .commitAuthorDate | date "yyMMdd" }}-{{ .environment }}-{{ .shortSha }}
"""

[_.lane-keeper.templates.merge-request-message]
title = "{{ if .ticket }}{{ .ticket }}: {{ end }}Prepare {{ .environment }} contribution"
body = """
Source commit: {{ .shortSha }}
Target branch: {{ .targetBranch }}
"""

[_.lane-keeper.workflows.deploy]
description = "Prepare a deployment-oriented contribution"
checks = ["main-ready"]
remote = "origin"
target_branch = { resolve = "git-remote-head" }
branch_template = "contribution-branch"
merge_request_template = "merge-request-message"

[_.lane-keeper.workflows.deploy.await]
interval = "30s"
timeout = "15m"

[tasks."check:readiness-main"]
run = "lane-keeper readiness check --workflow deploy"

[tasks."await:readiness-main"]
run = "lane-keeper readiness await --workflow deploy"

[tasks."tmpl:branch-name"]
run = "lane-keeper branch name --workflow deploy"

[tasks."tmpl:merge-req-message"]
run = "lane-keeper mr render --workflow deploy"
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

`predicate` must be inline Starlark in an ordinary TOML triple-quoted string:

```toml
predicate = """
target = workflow.target_branch

if git.latest_tag(target) == None:
    fail("no baseline tag found")

succeed()
"""
```

Language prefixes, file references, URLs, stdin, and other external predicate
sources are unsupported.

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

A `git.diff(...)` result exposes:

```python
diff.is_empty         # bool: True if no changes
diff.files            # list[str]: changed file paths
```

Example policy: filter changes by benign file patterns:

```python
diff = git.diff(baseline, target)

if diff.is_empty:
    fail("no changes since baseline")

for file in diff.files:
    if is_critical(file):  # application logic
        fail("non-benign change: " + file)

succeed()
```

### 8.5 Result functions

The predicate ends through:

```python
succeed()

fail("reason")
fail("reason", exit_code=2)
```

The `exit_code` parameter is optional and must be an integer in the range 1–250 if provided.

If omitted, the default exit code is 1 (predicate not ready).

Suggested semantic meanings (not enforced by the host):

```text
1   predicate not ready / expected workflow failure (default)
2   invocation or configuration error
3   repository state conflict
4   external service/API failure
```

The host converts the result into:

```text
status
human-readable message
exit code (1–250)
```

---

## 9. Command Surface

### 9.0 Config introspection (implemented)

```bash
lane-keeper config-introspection --lint <toml-files...>
lane-keeper config-introspection --fmt <toml-files...>
```

`--lint` validates TOML, parses extracted inline predicates as Starlark without
executing them, validates custom date layouts, and prints deterministic date
layout previews. It does not check Buildifier formatting.

`--fmt` performs the same validation, invokes the external `buildifier`
executable from `PATH`, replaces formatted predicate bodies in place, and
validates the resulting TOML. The flags are mutually exclusive. `config-check`
is currently accepted as a compatibility alias for `config-introspection`.

### 9.1 Check readiness

```bash
lane-keeper readiness check --workflow <workflow>
```

Example:

```bash
lane-keeper readiness check --workflow deploy \
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

### 9.2 Await readiness

```bash
lane-keeper readiness await --workflow <workflow>
```

Semantics:

```text
evaluate the same ordered checks
if not ready:
    sleep configured interval
    repeat
stop when ready or interrupted
```

`await` must not implement a second version of aggregate readiness evaluation.

This preserves the invariant that awaiting delegates to the canonical readiness logic.

### 9.3 Branch name

```bash
lane-keeper branch name --workflow <workflow>
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

### 9.4 Merge-request message

```bash
lane-keeper mr render --workflow <workflow>
```

This is a pure rendering operation. It must:

```text
resolve workflow context
render title and body as separate fields
print the rendered message
```

---

## 10. Built-in Defaults

Defaults should describe mechanics, not team-specific policy.

Reasonable defaults:

```text
remote                 origin
await interval         30s
await timeout          30m
branch template        [ticket-]date-environment-shortSha
```

Default branch template:

```text
{{ if .ticket }}{{ .ticket }}-{{ end }}{{ .commitAuthorDate | date "yyMMdd" }}-{{ .environment }}-{{ .shortSha }}
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
  "version": "1.42.0",
  "sha": "a83d0219...",
  "shortSha": "a83d021",
  "targetBranch": "main",
  "commitAuthorDate": "timestamp"
}
```

`commitAuthorDate` must derive from the immutable source commit's author date,
not wall-clock time.

This ensures retries against the same source commit produce the same branch identity.

TOML configuration keys and Starlark host properties use `snake_case`.
Template context properties use lower camel case, such as `shortSha` and
`targetBranch`.

Templates use Go `text/template`. The `date` function accepts the built-in
named layouts `yyMMdd`, `yyyyMMdd`, `HHmm`, `isoDate`, and `rfc3339`:

```gotemplate
{{ .commitAuthorDate | date "yyMMdd" }}
```

Advanced users may add named Go reference-time layouts:

```toml
[_.lane-keeper.defaults.template_date_formats]
releaseStamp = "2006.01.02"
```

Built-in names cannot be overridden and empty custom layouts are invalid.
Config introspection renders each custom layout with Go's reference time as an
informational preview; it does not infer whether the rendered content is useful.

### 11.1 Template precedence

```text
CLI override
> workflow-selected repository template
> built-in default
```

### 11.2 Validation

Validate the rendered branch name as a valid Git ref before printing it.

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
Target branch: {{ .targetBranch }}
"""
```

`lane-keeper mr render --workflow <workflow>` is a pure operation that renders
and prints the title and body. Repositories may expose it as:

```toml
[tasks."tmpl:merge-req-message"]
run = "lane-keeper mr render --workflow deploy"
```

The title and body MUST remain separate structured fields. Lane-Keeper MUST NOT
split one rendered template on blank lines: a Markdown body commonly contains
multiple paragraphs, so two consecutive newlines are not an unambiguous
delimiter.

---

## 12. Read-only operation boundary

Lane-Keeper inspects repository state and renders deterministic artifacts. It
MUST NOT create or update branches, push refs, create merge requests, or call a
remote mutation API. The invoking developer, shell, CI job, or dedicated VCS
tool owns those actions.

---

## 14. Workflow Evaluation

### 14.1 Target branch resolution

Each workflow MUST declare a `target_branch` inline table with a `resolve` field.
`resolve` is a closed Lane-Keeper API: configuration MUST NOT contain arbitrary
shell commands or scripts.

```toml
target_branch = { resolve = "literal", value = "master" }
target_branch = { resolve = "git-remote-head" }
```

`resolve = "literal"` requires a non-empty `value` containing a valid branch
name. `resolve = "git-remote-head"` forbids `value` and resolves the configured
remote's symbolic `HEAD` reference, such as `refs/remotes/origin/HEAD`, then
uses its branch component. This is the remote's default branch: the leading
branch proposed for a new merge request. An unknown resolver, an invalid field
combination, or resolution failure is a configuration/repository error;
Lane-Keeper MUST NOT silently substitute `main` or `master`.

The resolved branch becomes `workflow.target_branch` for Starlark evaluation.
This lets a repository use a stable declarative alias while retaining the
actual branch name in output and result metadata.

### 14.2 Workflow checks

Each workflow MUST declare a non-empty, ordered `checks` array. Each entry
names a configured check.

```toml
[_.lane-keeper.workflows.deploy]
checks = ["branch-ready", "required-tag-present"]
```

`readiness check` evaluates each named predicate once, in array order.
Evaluation stops at the first not-ready result or error. A workflow is ready
only when every check passes. `readiness await` MUST reuse this aggregate
evaluation; it must not implement a separate per-check execution path.

### 14.3 Await timing

Await timing resolves in this order:

```text
environment override
> workflow await setting
> repository default
> built-in default
```

The built-in interval is 30 seconds and the built-in timeout is 30 minutes.
`LANE_KEEPER_AWAIT_INTERVAL` and `LANE_KEEPER_AWAIT_TIMEOUT` provide
per-invocation overrides. A future CLI flag may take precedence over all four
sources when the readiness command is implemented.

The interval MUST be strictly positive. The timeout MUST be non-negative. A
timeout of zero permits the initial readiness evaluation and then returns its
result without sleeping or retrying. The timeout governs only sleeping and
retries; each predicate evaluation has its separate resource and cancellation
budget.

Configured and ordinary environment timeouts MUST NOT exceed 24 hours. A power
user may raise that ceiling by setting
`LANE_KEEPER_UNSAFE_ALLOW_LONG_AWAIT_MAXIMUM` to an integer number of seconds
greater than 86400. This unsafe value defines the effective ceiling and has no
additional policy maximum, though it must be representable as a Go duration.

## 15. Out of Scope

The following capabilities are explicitly **not** implemented on the foreseeable roadmap:

### 15.1 Global state and daemon

There is no system-wide state store, persistent daemon, or background worker.

- No cross-repository coordination or resource limits.
- `readiness await` remains attached to the invoking terminal or CI process.

This simplification preserves the principle that each repository owns its readiness state.

### 15.2 Notifications

There is no notification delivery mechanism.

- Readiness results are printed to stdout.
- External notification integrations (desktop notifications, Slack, email) belong to the invoking shell/IDE, not to `lane-keeper`.

This preserves the principle that `lane-keeper` is a local tool without external dependencies or side effects.

### 15.3 Retry or escalation logic

Predicates are stateless and have no memory between evaluations.

- No persistent failure counters.
- No exponential backoff on repeated failures.
- No escalation to human review or approval.

These behaviors belong to CI pipelines or higher-level orchestration, not to the readiness predicate.

---

## 16. `mise` Task Surface

The repository-facing task names may remain opinionated and team-specific even though the tool API is more generic.

Example:

```toml
[tasks."check:readiness-main"]
run = "lane-keeper readiness check --workflow deploy"

[tasks."await:readiness-main"]
run = "lane-keeper readiness await --workflow deploy"

[tasks."tmpl:branch-name"]
run = "lane-keeper branch name --workflow deploy"

[tasks."tmpl:merge-req-message"]
run = "lane-keeper mr render --workflow deploy"
```

This preserves the ADR's intended split between the team-owned tool and repository-owned task exposure.

---

## 17. GitLab CI Integration

GitLab CI should remain intentionally boring.

Example:

```yaml
readiness-main:
  script:
        - mise run check:readiness-main
```

The CI task must invoke:

```text
readiness check
```

exactly once.

It must never use:

```text
await
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

[_.lane-keeper]
version = 1
```

The tool must reject unsupported configuration schema versions.

For a globally installed local binary, a different tool version may produce an advisory warning if the repository pins another compatible version.

CI should use the pinned version.

---

## 19. Error Model

Predicates return exit codes in the range 0–250.

Reserved range: 251–255 (internal tool use only; predicates must not return these).

Normative exit codes:

```text
0   predicate passed / operation succeeded
1   predicate not ready / expected workflow failure (default)
2   invocation or configuration error
3   repository state conflict
4   external service/API failure
```

Predicates specify the exit code through the optional `exit_code` parameter to `fail()`:

```python
if git.latest_tag(target) == None:
    fail("no baseline tag found", exit_code=2)

if branch_already_exists:
    fail("conflicting branch state", exit_code=3)
```

If `exit_code` is omitted, the default is 1 (not ready).

Exit codes are stable and part of the versioned CLI contract. Once published in v1.0, they will not change.

The ADR explicitly treats task names, arguments, exit statuses, and machine-readable output as a versioned interface.

---

## 20. Output

Human-readable output should be the default.

Example:

```text
readiness: not ready
workflow: deploy
target: main
reason: no baseline tag found on main
```

Ready:

```text
readiness: ready
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
    succeed/fail

not allowed by default:
    arbitrary process execution
    arbitrary network requests
    arbitrary filesystem writes
    Git mutation
    GitLab mutation
    sleeping/polling
    notifications
```

Mutation belongs to the invoking VCS tool, shell, or CI job, not to Lane-Keeper.

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
4. read [_.lane-keeper] from mise.toml
5. validate config schema version
6. resolve one workflow
7. parse inline starlark: predicate
8. expose minimal read-only git API
9. implement succeed()/fail()
10. implement `readiness check`
11. stable human-readable output
12. stable initial exit-code contract
13. automated tests
```

Do not implement yet:

```text
await
workflow template rendering
```

Explicitly out of scope (see section 15):

```text
notifications (no delivery mechanism)
global state (no daemon, registry, or cross-repo coordination)
retry/escalation logic (predicates are stateless)
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

succeed()
"""

[_.lane-keeper.workflows.deploy]
checks = ["main-ready"]
target_branch = { resolve = "literal", value = "main" }
```

this command:

```bash
lane-keeper readiness check --workflow deploy
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
[tasks."check:readiness-main"]
run = "lane-keeper readiness check --workflow deploy"
```

must invoke exactly the same implementation locally and in GitLab CI.

This proves the ADR's central no-drift requirement.

---

## 24. Second Increment

After the first increment is proven:

```text
1. `readiness await`
2. configurable await interval and timeout
3. interrupt handling
4. environment timing overrides
5. tests proving await delegates to check/predicate implementation
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
5. commit author date extraction
6. branch ref validation
7. `branch name --workflow <workflow>`
```

Still no remote mutation required.

---

## 26. Fourth Increment

Add merge-request message rendering:

```text
1. structured title and body templates
2. resolved target branch in template context
3. `mr render --workflow <workflow>`
4. stable human-readable and machine-readable output
```

Still no remote mutation.

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
succeed/fail result conversion
Git API wrappers
template rendering
branch validation
```

### Integration tests

Use temporary Git repositories to verify:

```text
tag lookup
diff behavior
source SHA freezing
branch naming
```

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
    output/
```

Do not force this exact package layout if the implementation language or codebase suggests a simpler structure.

The important boundaries are:

```text
policy evaluation
repository mechanics
presentation
```

---

## 30. Core Invariants

The implementation must preserve all of the following:

1. Repository policy remains visible in the consuming repository.
2. Readiness has exactly one implementation per configured predicate.
3. CI evaluates each configured predicate once and never awaits.
4. Local await delegates to the same aggregate readiness evaluation.
5. All Lane-Keeper operations are read-only.
6. Readiness policy is read-only.
7. Local tooling is optional.
8. CI remains authoritative.
9. The source SHA is frozen before awaiting or rendering.
10. Branch naming is deterministic.
11. Branch and merge-request output is rendered but never applied remotely.
12. Notifications are out of scope (not implemented).
13. `lane-keeper` is the canonical executable.
14. `git lane-keep` is optional convenience.
15. `mise` is the preferred reproducibility layer, not a mandatory IDE integration layer.
16. Configuration must not evolve into a generic CI workflow language.
17. The consuming repository should remain easier to understand than the tool implementation.

---

## 31. Guiding Review Question

For every new feature, configuration field, Starlark API function, or built-in behavior, ask:

> Does this make the consuming repository easier to understand and review?

If the answer is no, prefer the simpler public model even if the `lane-keeper` implementation becomes more sophisticated internally.

That is the primary architectural criterion for this project.
