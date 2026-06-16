# Release Gate: sling no-formula propagation

Date: 2026-06-16

Deploy bead: ga-nxnkdz
Source bead: ga-mkkhig
PR: https://github.com/gastownhall/gascity/pull/3545
Branch: builder/ga-frh27v
Reviewed source commit: 4a5809a36e53bee4c2638301ae6c372a7901b24d
Local origin/main at gate time: 057b8884c2ca43d94829e20884a2bf5cef47fa5e

Note: docs/PROJECT_MANIFEST.md is not present in this worktree. This gate uses
the deployer release criteria from the active Gas City role prompt and the test
commands documented in TESTING.md.

## Scope

This change preserves `gc sling --no-formula` when routing through the convoy
expansion and plain bead routing paths. `RouteOpts` now carries the NoFormula
flag, `ExpandConvoy` forwards it into `SlingOpts`, and `RouteBead` forwards it
into `SlingOpts` so agents with a `DefaultSlingFormula` can still receive raw
bead routes when the caller explicitly suppresses formula attachment.

Changed files:

- `cmd/gc/cmd_sling.go`
- `internal/sling/sling.go`
- `internal/sling/sling_test.go`

## Gate Checklist

| # | Criterion | Result | Evidence |
|---|-----------|--------|----------|
| 1 | Review PASS present | PASS | Source review bead ga-mkkhig is closed with close reason `pass` and notes contain `PASS (gascity/reviewer, 2026-06-16)`. |
| 2 | Acceptance criteria met | PASS | The implementation covers all reviewed propagation sites: `doSlingBatchWithJSON` sets `RouteOpts.NoFormula`, `ExpandConvoy` forwards `RouteOpts.NoFormula` to `SlingOpts`, and `RouteBead` forwards `RouteOpts.NoFormula` to `SlingOpts`. Targeted tests cover both `ExpandConvoy` and `RouteBead` with a default formula configured. |
| 3 | Tests pass | PASS | `go test ./internal/sling -run 'TestSling(ExpandConvoy|RouteBead)_NoFormulaPreventsDefaultFormulaAttachment' -count=1` passed; `make test-fast-parallel` passed all fast jobs; `go vet ./...` completed cleanly; `git diff --check origin/main...HEAD` completed cleanly. |
| 4 | No high-severity review findings open | PASS | Review notes for ga-mkkhig state `Verdict: PASS - no blockers` and list no security or high-severity findings. |
| 5 | Final branch is clean | PASS | Gate worktree was clean before this gate file was added; `.githooks` is active via `core.hooksPath=.githooks`. Final clean status is verified after the gate commit. |
| 6 | Branch diverges cleanly from main | PASS | Branch is 2 commits behind and 2 commits ahead of current `origin/main`; `git merge-tree --write-tree origin/main HEAD` completed successfully with tree `d413bb149012402af1d6c3d4a6986772930b0b52`. The deployer did not rebase. |
| 7 | Single feature theme | PASS | The commit set touches one subsystem and one behavior: preserving explicit `--no-formula` suppression across sling routing paths. |

## Change Set

| Commit | Subject | Paths |
|--------|---------|-------|
| e23f0ed7c05af2c6155ac9264cfd51571d3f1a47 | fix(sling): propagate --no-formula through ExpandConvoy to DoSlingBatch | `cmd/gc/cmd_sling.go`, `internal/sling/sling.go`, `internal/sling/sling_test.go` |
| 4a5809a36e53bee4c2638301ae6c372a7901b24d | fix(sling): propagate NoFormula through RouteBead to DoSling | `internal/sling/sling.go`, `internal/sling/sling_test.go` |

## Local Commands

```text
git diff --check origin/main...HEAD
git merge-tree --write-tree origin/main HEAD
go test ./internal/sling -run 'TestSling(ExpandConvoy|RouteBead)_NoFormulaPreventsDefaultFormulaAttachment' -count=1
go vet ./...
make test-fast-parallel
```
