# Design Notes: Parsing Git Command Output Safely

- **Status:** Adopted
- **Applies to:** `internal/git`
- **Origin:** practices carried over from a sibling project (`nopii`) that parses
  Git commit streams and repeatedly had to defend against Git output edge cases.

## Context

`lane-keeper` shells out to `git` and parses its stdout. Naive parsing
(splitting on newlines, trusting a fixed field order without validation) is
fragile against real-world Git output: commit messages and file paths can
contain any byte except NUL, repositories can contain paths with embedded
newlines, and multi-field `--pretty=format` output has no built-in field
delimiter safe against attacker- or author-controlled content.

## Practices

### 1. Prefer NUL-based delimiters over newlines

File paths and commit message bodies may legitimately contain newlines but
never NUL. Wherever Git supports a `-z`/NUL-terminated output mode, prefer it
over line-based parsing.

`internal/git/inspect.go`'s `Diff` already does this (`splitNUL` on `git diff
--name-only -z`). Apply the same default to any future multi-path or
multi-record Git query added to this package.

### 2. Use an explicit, unlikely-to-collide field separator for multi-field records

When one `git` invocation must return several structured fields per record
(e.g. hash, parents, author identity, timestamps, body), use
`--pretty=format:` with an explicit separator byte such as `\x1f` (ASCII Unit
Separator) between fields and `\x00` between records, rather than relying on
a human-readable delimiter that could plausibly appear inside a commit
message or author name.

Example pattern (from a sibling project's `internal/stream/git.go`):

```text
format:MAGIC%x1f%H%x1f%P%x1f"%an" <%ae>%x1f%at%x00
```

A short magic prefix at the start of the format lets a parser distinguish
this structured record from arbitrary preceding output (for example, injected
annotation lines from a signing backend) before attempting to split fields.

### 3. Validate field count before indexing

After splitting a record on the chosen separator, check the resulting field
count against the expected count and fail with a clear error identifying the
malformed record, rather than indexing directly and risking an
out-of-bounds panic or silently misaligned fields if Git's output format ever
changes shape.

### 4. Parse structured sub-fields (e.g. mailbox identities) with explicit validation

Fields like Git's `"Name" <email@example.com>` author/committer identity
format should be parsed with explicit boundary checks (quote and angle
bracket positions) and a descriptive error on mismatch, not a permissive
regex that silently accepts malformed input.

## Application to `lane-keeper`

`internal/git/inspect.go` already follows practice #1 for `Diff`. When adding
new Git accessors (for example, a commit author-date lookup for
`branch name` template rendering), prefer:

- NUL-terminated or `-z` output where the underlying Git subcommand supports it;
- a `\x1f`-delimited `--pretty=format:` record with a magic prefix when more
  than one field is needed from a single invocation, to avoid ambiguity with
  commit message content;
- explicit field-count validation before parsing rather than optimistic
  indexing.

Single-field queries (e.g. `git rev-parse`, `git for-each-ref` with one
`--format` field) do not need the multi-field record pattern; `TrimSpace` on
the single line of output remains sufficient, as already used throughout
`internal/git/inspect.go`.
