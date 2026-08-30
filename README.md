# Lane-Keeper

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
  typed Lane-Keeper schema and reference validation, Canonical Starlark syntax
  parsing, and custom date-layout previews;
- `lane-keeper config-introspection --fmt <toml-files...>` for in-place
  Buildifier formatting of embedded predicates;
- the `git-keep-lane` forwarding executable;
- internal template context and named date-layout primitives.

`readiness`, `branch`, and `mr` are currently command stubs. Workflow loading,
target-branch resolution, Starlark execution and host APIs, readiness evaluation,
and template rendering remain planned work. The example configuration below
documents the intended public contract, not a fully implemented workflow.

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
