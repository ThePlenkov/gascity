# Yelena Markovic — Runtime State Migration Reviewer (Attempt 8, Independent DeepSeek V4 Flash Style)

**Verdict:** block

> **Lane:** Maintenance to Core runtime state migration, jsonl and archive state, spawn-storm ledgers, non-destructive markers, old/new binary concurrency, downgrade continuity.
>
> Reviewed against the Attempt 8 design document (`.gc/design-reviews/ga-1ekw9l/attempt-8/design-before.md`, 835 lines, `updated_at: 2026-06-09T13:20:59Z`) — specifically §"Doctor And Runtime-State Mutation Safety" (lines 392–408), §"Data And State" (lines 570–592), and §"Testing" → "Doctor and runtime-state tests" (lines 657–671).
>
> This independent review is produced using the DeepSeek V4 Flash persona, focusing specifically on first-principles trust boundaries, cross-document state consistency, and unstated runtime assumptions.

---

## Schema Conformance

Conforms to `gc.mayor.implementation-plan.v1`. Front matter carries the required keys with `phase: implementation-plan` and no `design_file`; the eight required top-level sections appear once each in the required order, and `Open Questions` is `None`. No appended attempt/review prose in the artifact.

---

## Top Strengths of the Design

- **Doctor-Owned, Lock-Gated Migration (Exclusive Domain):** Restricting mutation to `gc doctor --fix` and holding the city advisory lock (lines 394–397) guarantees that the state migration cannot run concurrently with other pack installations or import rewrites. Disallowing live controller startup paths from mutating runtime state ensures high stability.
- **Durable Recovery State Pre-Flighting:** Requiring that multi-file fixes write durable recovery state before publishing (lines 385–388) ensures fail-safety and determinism.
- **Non-Destructive Directory Preservation:** Leaving stale directories like `.gc/system/packs/maintenance` untouched (lines 598–601) guarantees that older binaries do not encounter panics on missing directories if rolled back.

---

## Critical Risks & Consensus Blockers (DeepSeek V4 Flash Style)

### 1. [Blocker] Unimplemented Old/New Path Ledger (Vague Table Placeholders)
- **The Risk:** The plan specifies that runtime-state migration moves JSONL archive state, spawn-storm ledgers, refs/remotes, etc. (lines 399–402), and asserts that the migration table is part of the implementation, not review prose (line 581). However, the table (lines 583–592) still uses abstract, non-implementable placeholders like `legacy archive directories`, `Core archive directory`, `old throttle ledger`, `Core throttle ledger`, and `old order keys`.
- **The Impact:** Without concrete, exact file paths and keys mapped out, implementers cannot write deterministic, correct migration code. For example, `GC_PACK_STATE_DIR` (defaulting to `.gc/runtime/packs/maintenance`), `jsonl-export-state.json`, `.gc/jsonl-archive`, and `.gc/spawn-storm-counts.json` are actual paths on disk that must be moved to Core-owned paths like `.gc/runtime/packs/core/jsonl-export-state.json` or `.gc/system/packs/core/jsonl-archive`. Conflating these with placeholders introduces high ambiguity and potential path collision bugs.
- **Required Resolution:** The migration table must be updated to map exact legacy paths/keys to their canonical Core-owned locations:
  - JSONL archives: from `.gc/jsonl-archive` to `.gc/system/packs/core/jsonl-archive`.
  - Spawn-storm ledgers: from `.gc/spawn-storm-counts.json` to `.gc/system/packs/core/spawn-storm-counts.json`.
  - JSONL export state: from `.gc/jsonl-export-state.json` to `.gc/system/packs/core/jsonl-export-state.json`.
  - Escalation/metadata fields: `pending_archive_push`, `consecutive_push_failures`, `push_failure_escalated`.
  - Order skip lists: legacy order skip tracking key to Core-owned order tracking key.

### 2. [Blocker] Vague and Unspecified Post-Marker Old-Binary Divergence Detection
- **The Risk:** The plan asserts that the migration marker records "old-binary post-marker write detection" (line 405), and that "retained legacy state is ignored unless the marker or digest checks show conflict" (lines 578–579). It does not define *which* legacy files are fingerprinted, how directories or git refs are compared (e.g., mtimes vs. content hashes), or when/how the check occurs.
- **The Impact:** If an old binary runs concurrently or post-rollback, writing legacy states, and the detection algorithm is vague, the new binary may fail to detect divergence or trigger false-positive version-skew blocks. Conversely, if it silently ignores post-marker writes, it can lead to silent data loss of operator work written during rollback.
- **Required Resolution:** Explicitly define the marker's schema and divergence detection algorithm. At minimum, the marker must record:
  - A cryptographic content hash (SHA-256) of each tracked legacy file (e.g., state and ledger files) immediately after migration.
  - The latest git commit hash of the migrated archive.
  On startup/reload, the new binary must compute the live legacy file digests and compare them with the marker. If a digest differs, the new binary must raise a typed `VersionSkewConflict` error and refuse behavior-changing operations until manual reconciliation or a deterministic re-upgrade is triggered.

### 3. [Major] Push-Cursor Race and Duplicate Commits on Shared Git Remotes
- **The Risk:** The legacy JSONL archive often pushes to a shared remote (e.g., origin). Copying the git refs/remotes (line 586) while copying the archive may preserve the push cursors and tracking branches.
- **The Impact:** If an operator rolls back to an old binary, both the old and new binaries may push to the same remote branch concurrently. Since they use separate local git folders (`.gc/jsonl-archive` vs. `.gc/system/packs/core/jsonl-archive`), their commit histories will diverge, causing force-push rejections, broken refs, or duplicated, interleaved commits on the remote server.
- **Required Resolution:** Specify that during copy-migration, the doctor must either disable the `origin` remote in the legacy repository (e.g., by renaming it to `retired-origin` or setting its URL to a dummy value) or clear the legacy `pending_archive_push` flag to prevent concurrent pushes from the legacy location post-upgrade.

### 4. [Major] Multi-process Copy-Migration Race (Destination-Absent Copier)
- **The Risk:** The plan requires staged copy digest checks (lines 402–403) and states that `gc doctor --fix` performs the migration (lines 394–395). However, it does not explicitly specify how the doctor prevents copy races if another process is actively writing to the legacy directory or state files while the copy is in-flight.
- **The Impact:** If `jsonl-export.sh` or a background writer is actively appending to the legacy JSONL state file or pushing to the git archive while the doctor copy is running, the copied state will be half-written or torn (a torn-write snapshot).
- **Required Resolution:** The doctor pre-flight checks must verify that no active script writers (e.g., PIDs for `jsonl-export.sh` or `spawn-storm-detect.sh`) are running in the system process table. If active writers are detected, the doctor must abort the migration with a clear diagnostic. Furthermore, the git archive copy must be staged in a temporary directory and atomically renamed (`os.Rename`) to the final Core-owned path once the copy and checksum validation succeed.

---

## Detailed Responses to Lane-Specific Questions

### Q1: Are jsonl state, archive remotes and refs, spawn-storm ledgers, order skip lists, and escalation fields migrated or preserved with non-destructive markers?

**Answer:**
Yes, the design successfully plans to preserve them (lines 399–402) and explicitly mandates non-destructive directory preservation (lines 598–601). However, the preservation is incomplete because the exact keys and paths are not mapped out in the ledger (as noted in Blocker 1). To fully satisfy this requirement, the doctor must:
1. Copy the JSONL state from legacy `jsonl-export-state.json` to the Core-owned location.
2. Reconcile the spawn-storm count ledger by copying the JSON counts.
3. Migrate the git archive, preserving remote configurations but neutralizing the legacy git repository's remotes to prevent duplicate pushing.
4. Record all source and destination file digests within a dedicated `.gc/migration-marker.json` file carrying a schema version.

---

### Q2: If an old binary writes legacy Maintenance state after the migration-completed marker, how does the new binary detect and handle divergence without data loss?

**Answer:**
The new binary detects divergence by checking the current SHA-256 digests of the legacy files against the digests captured in the `.gc/migration-marker.json` immediately after the migration succeeded.
If the live legacy digests differ from the marker's recorded digests:
- **Case A: Pure Rollback (Core-owned state is untouched):** If no writes have occurred to the new Core-owned state, the new binary can perform a deterministic "re-upgrade" by fast-forwarding the Core-owned paths to match the latest legacy state, updating the marker digests, and resuming operations.
- **Case B: Split-Brain Divergence (Both legacy and Core-owned states have written):** The new binary must raise a blocking version-skew diagnostic, transition to a degraded read-only state, and refuse behavior-changing operations until the operator manually resolves the conflict.

---

### Q3: What round-trip test covers interrupted archive copy, in-flight push state, duplicate writers, and rollback to an older binary?

**Answer:**
The testing plan mentions general post-marker write and downgrade tests (lines 665–667) but needs a dedicated, comprehensive integration test:
**`TestRuntimeStateMigrationRoundTrip`** (under `test/packcompat/`):
1. Set up a city with legacy JSONL files, active spawn-storm counts, and an in-flight `pending_archive_push=true` archive state.
2. Run `gc doctor --fix`, inject an interruption (SIGKILL) mid-archive copy, and verify that the incomplete directory is detected as corrupt/incomplete on the next run.
3. Resume and complete the migration; verify that the legacy remote is neutralized and that Core-owned paths now correctly hold the state and cursors.
4. Simulate a downgrade: run an older binary to write a new line to the legacy JSONL state.
5. Re-run the new binary; verify it detects the divergence, blocks with a `VersionSkewConflict` diagnostic, and refuses mutating commands.
6. Trigger a re-upgrade fast-forward (asserting the legacy change converges into the Core path), and verify the system resumes healthy operation.

---

## Evaluation Against Lane Anti-patterns

| Anti-pattern / Red Flag | Mitigation in Current Design | Status |
| :--- | :--- | :--- |
| **Destination-absent copy races two processes or treats half-copied archives as complete** | **Fail.** The design specifies staged archive-copy digest checks but does not enforce atomic renaming or process-table check for active writers. | **Blocker** |
| **Post-marker old-binary writes are silently ignored** | **Fail.** The design says legacy is ignored unless there's a conflict but does not specify how divergence is safely reconciled or fast-forwarded without silent data loss. | **Blocker** |
| **Renamed orders break existing skip lists or duplicate pending pushes** | **Pass.** The design preserves tracking/skip lists via aliases and copies pending push states. | **Pass** |

---

## Final Verdict: Block

The Attempt 8 design resolves some key high-level concerns, such as utilizing the doctor-owned advisory lock and planning non-destructive directory preservation. However, it still retains **consensus blockers** from previous iterations: the migration table relies on **vague placeholders** instead of exact canonical file paths/keys, the **post-marker old-binary divergence detection** is named but mechanically unspecified, and **downgrade continuity** fails to address the silent split-brain execution hazard of old binaries running on stale legacy paths. Consequently, I must **Block** the current plan. Defining concrete paths, detailing the marker SHA-256 fingerprint checks, neutralizing legacy remotes, and establishing the robust `TestRuntimeStateMigrationRoundTrip` integration test are necessary to secure this critical lane.
