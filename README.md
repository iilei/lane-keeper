# Lane-Keeper

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

The ADR establishes that local and CI behavior must share one readiness implementation, that CI checks exactly once, and that local waiting delegates to the same predicate. fileciteturn0file0L28-L34 fileciteturn0file0L47-L94
