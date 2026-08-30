# Lane-Keeper

[![codecov](https://codecov.io/gh/iilei/lane-keeper/graph/badge.svg?token=JB6LMYL16Z)](https://codecov.io/gh/iilei/lane-keeper)

## 1. Purpose

`lane-keeper` is a small, read-only repository workflow tool intended to make readiness checks, awaiting readiness, branch naming, and merge-request message rendering consistent between local development and GitLab CI.

Its primary goal is **not** to hide repository policy inside tooling.

Its primary goal is to provide reusable mechanics while keeping the consuming repository's workflow policy:

The first adopting repository has a complex multi-environment GitLab CI setup. A merge-request pipeline may predictably fail while the leading target branch is not yet in a required repository state. That behavior is intentional and must remain authoritative in CI.

`lane-keeper` provides a local and automation-friendly way to evaluate the same precondition before filing a merge request, without adding another dynamic CI layer.

The design establishes that local and CI behavior must share one readiness implementation, that CI checks exactly once, and that local awaiting delegates to the same predicate.

## Implementation Status

The current executable implements:

- `lane-keeper version`;
- `lane-keeper config-introspection --lint <toml-files...>` for TOML validation,
  typed Lane-Keeper schema and reference validation, structural Starlark syntax
  validation of every configured check predicate (regardless of TOML string
  quoting style), and custom date-layout previews;
- `lane-keeper config-introspection --fmt <toml-files...>` for in-place
  Buildifier formatting of embedded predicates (limited to the documented
  ordinary triple-quoted representation, since splicing requires literal text
  positions);
- the `git-keep-lane` forwarding executable;
- internal template context and named date-layout primitives;
- typed workflow lookup, default merging, ordered check resolution, and
  `literal`/`git-remote-head` target-branch resolution;
- read-only Git inspection for commit resolution, Git-native abbreviated SHAs,
  newest reachable tags by creator date, author/committer dates, and NUL-safe
  changed file paths;
- bounded inline Starlark execution with immutable workflow/input contexts,
  read-only Git host functions, a namespaced `shared` block for reusable
  predicate helpers, and terminating `succeed()`/`fail()` results;
- `lane-keeper readiness check --workflow <name>` and
  `lane-keeper readiness await --workflow <name>` with repository config
  discovery, ordered check evaluation, and predicate exit-code propagation;
- `lane-keeper branch name --workflow <name>` and
  `lane-keeper mr render --workflow <name>` for deterministic branch and
  merge-request template rendering;
- human-readable text output (default) or stable JSON via `--output json` on
  all four `readiness`/`branch`/`mr` commands.

`--lint` is self-contained. `--fmt` requires the external `buildifier`
executable on `PATH`; the published formatting hook installs it in its isolated
Go hook environment.

## Proposed Configuration

Copy or merge [`.example.mise.toml`](.example.mise.toml) into the consuming
repository's `mise.toml`. Lane-Keeper configuration lives beneath
`[_.lane-keeper]`, which Mise ignores as project metadata.

Each workflow names its target branch through the built-in `target_branch` resolver:

```toml
[_.lane-keeper.workflows.release]
target_branch = { resolve = "git-remote-head" }
# target_branch = { resolve = "literal", value = "master" }
```

`resolve = "git-remote-head"` resolves the configured remote's symbolic `HEAD`,
such as `refs/remotes/origin/HEAD`. Use `resolve = "literal"` with a non-empty
`value` when the workflow must always evaluate a named branch. Resolver values
are a closed Lane-Keeper API.

Await behavior has repository defaults and optional workflow overrides:

```toml
[_.lane-keeper.defaults]
await_interval = "30s"
await_timeout = "30m"

[_.lane-keeper.workflows.release.await]
interval = "10s"
timeout = "15m"
```

An await timeout of `0s` always permits the initial readiness evaluation but no
sleep or retry. Intervals must be positive; timeouts must be non-negative and
normally cannot exceed 24 hours.

Per-invocation environment overrides are `LANE_KEEPER_AWAIT_INTERVAL` and
`LANE_KEEPER_AWAIT_TIMEOUT`. A power user may raise the 24-hour ceiling by
setting `LANE_KEEPER_UNSAFE_ALLOW_LONG_AWAIT_MAXIMUM` to a positive integer
number of seconds greater than 86400. The supplied ceiling has no additional
policy maximum, but it must fit Go's duration representation.

## Explicit Config File

Every `readiness`/`branch`/`mr` command accepts `--config <path>` to read a
dedicated Lane-Keeper TOML file instead of discovering `mise.toml`. Because
Mise never parses this file, its configuration fields live at the document
root, without the `[_.lane-keeper]` wrapper:

```toml
version = 1

[defaults]
remote = "origin"

[checks.main-ready]
predicate = """
succeed()
"""

[workflows.deploy]
checks = ["main-ready"]
target_branch = { resolve = "literal", value = "main" }
```

The two config shapes are not interchangeable: the implicit `mise.toml`
lookup always requires the `[_.lane-keeper]` nesting, and an explicit
`--config` file always uses the root-level shape above.

## Tool Version Advisory

When the repository's `mise.toml` pins a `lane-keeper` version under
`[tools]` that differs from the running binary's version, every command
prints a non-fatal warning to stderr and continues:

```text
lane-keeper: warning: running version 1.0.0, but repository pins 9.9.9
```

This check only applies to the implicit `mise.toml` lookup; `[tools]` is a
Mise concept with no meaning in an explicit `--config` file. Local `dev`
builds are never checked.
