# Design Decision: Inline-Only Starlark Policy in `mise.toml`

- **Status:** Proposed
- **Date:** 2026-08-30
- **Applies to:** `lane-keeper`
- **Originating ADR:** *Shared Preflight and Idempotent Merge Request Creation Assistance via `mise`*
- **Scope note:** the current Lane-Keeper design is read-only and does not implement the ADR's mutation operations.
- **Primary design question:**
  > What makes the consuming repository easiest to understand and review?

## Context

`lane-keeper` needs repository-specific readiness logic that can be evaluated identically:

- locally by developers;
- by automation and LLM-based agents;
- in GitLab CI.

The ADR requires one implementation of the readiness predicate, with CI evaluating it exactly once and local awaiting delegating to the same predicate. It also requires readiness evaluation to remain read-only against the leading target branch.

Two approaches were considered for expressing repository policy:

1. a small custom declarative DSL;
2. embedded Starlark.

A custom DSL provides a very narrow security surface, but would require `lane-keeper` to define and maintain its own syntax, parser, evaluation semantics, expression rules, diagnostics, variable model, and future extension strategy.

Starlark is more expressive and familiar-looking, but embedded scripting can raise reasonable security concerns if policy code may be loaded from arbitrary files, remote sources, dynamically resolved modules, or host APIs with broad capabilities.

The goal is therefore to retain the readability advantage of Starlark while making its execution boundary explicit and narrow.

## Decision

`lane-keeper` will support **inline Starlark predicates only**.

### Current implementation status

The current `config-introspection` command extracts ordinary triple-quoted
predicate values, parses them with Canonical Starlark without execution, and
can format them through external Buildifier. `readiness check` executes ordered
predicates with finite step, allocation, and deadline limits; immutable
workflow/input contexts; read-only Git inspection; and terminating
`succeed()`/`fail()` results. `readiness await` remains planned work.

The Starlark source MUST be obtained directly from the repository-owned
`mise.toml`, from a configured field under the `[_.lane-keeper]` metadata
namespace.

Example:

```toml
[_]

[_.lane-keeper]
version = 1

[_.lane-keeper.checks.main-ready]
description = "Whether the target branch is ready for this contribution"

predicate = """
target = workflow.target_branch
baseline = git.latest_tag(target)

if baseline == None:
    fail("no baseline tag found on " + target)

if git.diff(baseline, target).is_empty:
    fail("no relevant changes since " + baseline)

succeed()
"""
```

The `predicate` field is defined by the configuration schema to contain Starlark source.

No language prefix is required.

The following forms are intentionally unsupported:

```text
starlark+file:
file:
http:
https:
git:
exec:
stdin:
```

There is no external predicate source resolution.

## Source-of-Policy Boundary

The only permitted source of executable repository policy is:

```text
repository mise.toml
    |
    +-- [_]
        |
        +-- lane-keeper
            |
            +-- checks.<name>.predicate
```

`lane-keeper` MUST NOT:

- load predicate code from arbitrary files;
- follow predicate references to another repository;
- download predicate code;
- evaluate code from environment variables;
- accept predicate source through stdin;
- accept arbitrary predicate source through normal CLI flags;
- dynamically resolve additional Starlark modules.

An explicit testing-only mechanism may exist internally, but it MUST NOT become part of the normal repository-facing execution contract unless separately decided.

## No Starlark Module Loading

Starlark `load(...)` functionality MUST NOT be exposed.

A predicate is self-contained.

Example of unsupported policy:

```python
load("//policy:helpers.star", "is_ready")
```

The absence of module loading ensures that the complete readiness policy remains visible in the repository configuration being reviewed.

This is a deliberate readability and security property.

## Host API Boundary

Starlark receives only a fixed host-provided API.

The initial API is read-only.

### Workflow context

```python
workflow.name
workflow.target_branch
workflow.remote
```

### Invocation context

```python
input.environment
input.ticket
```

Nullable values are represented as:

```python
None
```

### Git inspection

Initial functions may include:

```python
git.resolve(ref)
git.short_sha(ref)
git.latest_tag(ref)
git.diff(from_ref, to_ref)
```

Returned objects expose only deliberate read-only properties.

Example:

```python
diff = git.diff(baseline, target)

if diff.is_empty:
    fail("no relevant changes")

# Later extension: filter changes by file pattern
for file in diff.files:
    if is_environment_critical(file):
        fail("non-benign change: " + file)
```

The `diff.files` property is required because the initial repository policy
must classify changed paths as benign or environment-determining.

The host API SHOULD grow only when a concrete repository policy requires another primitive.

## Explicitly Forbidden Capabilities

The Starlark environment MUST NOT provide:

```text
arbitrary process execution
shell execution
network access
HTTP clients
arbitrary filesystem reads
filesystem writes
environment mutation
Git mutation
GitLab mutation
sleeping
polling
background execution
notification delivery
dynamic module loading
```

These operations belong to the compiled `lane-keeper` implementation where applicable.

The predicate's responsibility is only:

```text
                                         / ... ok: pass, exit 0
(repository state) + (workflow input) }-{
                                         \ not ok: fail, exit !0
```

Whilst workflow input may or may not imply waiting for a desired state.

### Result Contract

Predicates terminate through host-provided result functions:

```python
succeed()

fail("reason")
fail("reason", exit_code=2)
```

Examples:

```python
if baseline == None:
    fail("no baseline tag found")

if not ready:
    fail("state not ready", exit_code=1)

succeed()
```

The host converts this result into:

```text
readiness state
human-readable diagnostics
stable process exit status
```

Interpreter errors fail closed and are treated as configuration/policy errors, not as readiness success.

## Execution Policy Is Outside Starlark

Starlark defines the readiness predicate only.

It MUST NOT know whether it is being consumed by:

```text
check
await
CI
human
LLM
```

The compiled tool owns execution policy.

### Check

```text
evaluate once
return result
```

### Await

```text
evaluate
if not ready:
    sleep
    evaluate again
```

Both operations use the exact same ordered predicate set.

This preserves the ADR requirement that local and CI behavior must not drift.

## Read-only Boundary

Readiness Starlark is strictly read-only.

Lane-Keeper may await readiness and render deterministic branch names or
merge-request messages. It does not create branches, push refs, create merge
requests, or invoke remote mutation APIs.

This preserves the invariant that readiness inspects repository state without modifying it.

## Resource Limits

The embedded Starlark evaluator MUST use bounded execution.

Every evaluation thread MUST require all of Canonical Starlark's safety flags:

```text
CPUSafe
MemSafe
TimeSafe
IOSafe
```

The evaluator MUST set finite step and allocation budgets before executing a
predicate. It MUST also have a cancellation deadline. A predicate exceeding a
budget or cancellation deadline MUST fail as a policy/configuration error. It
MUST NOT block indefinitely.

Every Lane-Keeper-provided builtin or Starlark value MUST declare the safety
flags it satisfies. Host APIs are read-only and therefore must be `IOSafe`:
they may inspect only the repository data explicitly provided by the evaluator;
they must not execute processes, open arbitrary files, access the network, or
mutate Git state.

Builtin implementations MUST account for work and memory that scales with
Starlark-controlled input:

```text
use thread.AddSteps or thread.CheckSteps before proportional CPU work
use thread.AddAllocs or thread.CheckAllocs before persistent or significant
transient allocations
use SafeInt arithmetic for resource calculations
use SafeStringBuilder for constructed strings
use SafeIterate for Starlark-controlled iteration and check its final error
use SafeAppender for Starlark-controlled slice growth
```

`CheckAllocs` alone is insufficient for overlapping transient allocations: an
implementation MUST temporarily reserve their combined worst-case size with
`AddAllocs`, then release that reservation after the operation.

Each builtin or custom Starlark value MUST have safety tests using Canonical
Starlark's `startest` package. The tests MUST require the relevant safety flags
and exercise input sizes proportional to the test scale so under-counted CPU or
memory usage fails the test.

These requirements are a best-effort safety contract, not a substitute for
process isolation. Lane-Keeper's initial host API remains deliberately small so
the contract can be audited and tested as it grows.

## Reviewability

The main reason for choosing inline Starlark rather than a custom DSL is repository reviewability.

A reviewer should be able to inspect:

```toml
[_.lane-keeper.checks.main-ready]
predicate = """
...
"""
```

and answer:

```text
What repository state is being checked?
Why does the check pass?
Why does it fail?
What data does it inspect?
```

without navigating into the `lane-keeper` implementation repository.

The complete executable policy must be visible in the same repository change that changes the workflow.

## Security Model

The security model is intentionally simple to state:

> `lane-keeper` evaluates only inline Starlark stored under its namespace in the repository's `mise.toml`. The interpreter exposes a fixed, read-only host API and cannot load modules, execute processes, access the network, mutate Git state, or retrieve additional code.

This is preferable to a generic scripting-source model whose effective policy may be distributed across multiple files or dynamically retrieved locations.

## Trust Model

The repository-owned `mise.toml` is already code-reviewed project configuration.

Changing a `lane-keeper` predicate therefore has the same repository governance boundary as changing:

```text
mise tasks
GitLab CI configuration
repository scripts
tool version pins
```

This decision does not claim that repository configuration is inherently trusted.

It makes the executable-policy origin explicit and reviewable.

CI and local executions must evaluate the predicate from the checked-out repository state that is relevant to that invocation.

## Why Not a Custom DSL

A small DSL remains a viable alternative.

It was not selected because supporting meaningful repository checks would likely introduce, over time:

```text
named values
bindings
boolean composition
comparison operators
Git-specific functions
error messages
types
expression semantics
source locations
parser diagnostics
```

That would effectively create a new language that contributors must learn and maintainers must specify.

Starlark provides these basic language mechanics while `lane-keeper` retains control over all security-sensitive capabilities through its host API.

## Why Not External Starlark Files

External `.star` files were rejected for the initial design.

Although they could improve organization for large policies, they weaken the primary reviewability property:

```text
open mise.toml
-> see complete readiness policy
```

They also introduce source resolution and module-boundary questions that are unnecessary for the expected small predicates.

If predicates later become large enough that inline configuration is clearly harmful to readability, external policy files require a separate design decision.

## Why Not Shell Commands

A configuration such as:

```toml
command = "./scripts/check-main-ready"
```

would be simple to implement but provides a much broader execution capability and makes the actual policy less visible in `mise.toml`.

It also introduces platform, quoting, runtime, and process-environment concerns.

Therefore arbitrary command execution is not part of the readiness predicate model.

## Consequences

### Positive

- complete readiness policy is visible in `mise.toml`;
- no custom expression language needs to be invented;
- no external policy loading exists;
- no arbitrary shell execution is required;
- host capabilities are explicit and auditable;
- local and CI evaluation use the same source;
- predicates remain readable to developers without knowledge of the `lane-keeper` implementation language;
- review of workflow policy remains local to the consuming repository.

### Negative

- `lane-keeper` embeds and maintains a Starlark runtime;
- the implementation must enforce execution/resource limits;
- maintainers must carefully control the host API;
- security reviewers must understand the constrained Starlark execution model;
- inline policies may become unwieldy if they grow too large.

## Invariants

1. Predicate source comes only from `[_.lane-keeper]` configuration in repository `mise.toml`.
2. Predicates are inline Starlark strings.
3. External policy-source schemes are unsupported.
4. `load(...)` is unavailable.
5. No arbitrary process execution is exposed.
6. No network access is exposed.
7. No arbitrary filesystem access is exposed.
8. No Git or GitLab mutation is exposed to predicates.
9. Predicates cannot sleep, poll, watch, or notify.
10. `readiness check` and `readiness await` evaluate the same ordered predicate set.
11. CI evaluates the predicate exactly once.
12. Interpreter errors fail closed.
13. Predicate execution is resource-bounded.
14. The complete repository policy remains visible during review of `mise.toml`.
15. New Starlark host functions require deliberate review as additions to the tool's security surface.

## Example Complete Policy

```toml
[_]

[_.lane-keeper]
version = 1

[_.lane-keeper.checks.main-ready]
description = "Determine whether the target branch is ready for the deployment contribution"

predicate = """
target = workflow.target_branch
baseline = git.latest_tag(target)

if baseline == None:
    fail("no baseline tag found on %s" % target)

changes = git.diff(baseline, target)

if changes.is_empty:
    succeed()

succeed()
"""

[_.lane-keeper.workflows.deploy]
checks = ["main-ready"]
target_branch = { resolve = "literal", value = "main" }
```

The intended reviewer experience is:

```text
I can read the repository configuration
and understand the complete readiness rule
without opening the lane-keeper source code.
```

That is the deciding criterion for this design.
