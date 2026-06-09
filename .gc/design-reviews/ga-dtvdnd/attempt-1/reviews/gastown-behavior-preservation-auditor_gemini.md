# Oleg Marchetti — DeepSeek V4 Flash Perspective Independent Review (Iteration 2 / Attempt 1)

**Verdict:** approve-with-risks

**Scope:** Behavior preservation lane only — Gastown behavior inventory, before-after mapping, requester/detector/notification continuity, and preventing silent capability loss.

---

## Executive Summary

This review evaluates the Iteration 2 / Attempt 1 draft of the Core/Gastown Split implementation plan against the approved requirements, prior audit feedback, and current codebase behaviors. The updated design represents a monumental leap in rigor, incorporating a machine-readable **Source-Derived Behavior Manifest (AC6/AC7)**, a robust **System Pack Loader (`internal/systempacks`)**, and a **Strict Behavior Witness Floor** to defend against silent regressions.

However, from the strict, empirical perspective of **Behavior Preservation Auditing**, several highly subtle, cross-document inconsistencies and critical edge cases remain unaddressed. These gaps—ranging from CI generator traps to multi-host concurrency blindness—could cause silent operational failures or repository state corruption if accepted too quickly.

---

## Evaluation of the Three Key Questions

### 1. Does every generalized Core asset have a corresponding external Gastown home for stripped role-specific behavior?
**Auditor Finding: Yes.** The implementation plan establishes a strict multi-repo rollout sequence (Slice 1 prerequisite) ensuring that no source deletion or de-roling of the SDK occurs until Gastown-owned formulas, orders, scripts, prompts, and overlays are safely landed in `gascity-packs/gastown` at immutable commits.

### 2. Does the before-and-after inventory cover formulas, orders, scripts, prompts, template variables, and notification paths rather than only file moves?
**Auditor Finding: Yes.** The "Source-Derived Behavior Manifest" (§133–167) explicitly tracks behavior at the logical level—capturing triggers, requesters, detectors, mail/nudge targets, prompt fragments, and script branches. This directly answers prior concerns about file-level mapping opacity.

### 3. What artifact proves supported Gastown workflows still resolve and trigger after the split?
**Auditor Finding:** The canonical machine-readable **`plans/core-gastown-pack-migration/behavior-manifest.generated.yaml`** and the `test/packcompat` suite represent the definitive, auditable proof. The packcompat gate executes moved scripts, composes molecules, and validates configured recipients from the public pin in a clean environment.

---

## Critical Risks & Missing Edge Cases (Auditor Findings)

### 1. The CI Generator Freshness Trap (Self-Defeating Manifest Validation)
* **The Risk:** The plan specifies that CI will fail if the behavior manifest is stale, and that the generator dynamically "walks old Gas City behavior-bearing sources" to ensure complete coverage (§154–160).
* **The Gap:** Once Slice 7 deletes the legacy source directories (`internal/bootstrap/packs/core` and `examples/gastown/packs/maintenance`), any dynamic generator scan in CI on subsequent commits/PRs will find exactly **zero** old files. The generator will either crash, report empty rows, or fail to validate against the "after" state.
* **The Fix:** The implementation plan must explicitly detail how the generator handles deleted legacy paths. It must either:
  1. Fetch the "before" state from a historic Git ref (baseline commit) dynamically during the CI run, or
  2. Validate against a frozen, cryptographically hashed or digest-verified snapshot of the "before" state checked into the repository, ensuring manual tampering of the manifest is blocked without requiring legacy folders to remain in the tree.

### 2. Distributed/Network Shared-Disk Blindness (Multi-Host Concurrency Bypass)
* **The Risk:** The Doctor's mutation coordinator uses process table checks (`ps`, `lsof`) to discover if a controller for the same city is running, refusing automatic fixes if one is active (§255–258).
* **The Gap:** This check is strictly local. In enterprise, Kubernetes, or clustered environments where the city's workspace resides on a shared network disk (e.g., NFS, AWS EFS, or Kubernetes ReadWriteMany volumes), a controller running on Host A is completely invisible to `gc doctor --fix` running on Host B.
* **The Consequence:** Concurrently running the Doctor's mutation coordinator while a remote controller is actively reading/writing will lead to catastrophic data corruption of the Task Store (Beads) or configuration files.
* **The Fix:** The mutation coordinator must utilize a filesystem-level exclusive lock (such as `flock`/`fcntl` on a dedicated lock file under `.gc/` or a lock in the persistent DB/dolt backend) to guarantee multi-host concurrency protection, overriding the "no status files" rule for this exceptional, non-reentrant system mutation.

### 3. In-Flight Session Path Stalls (Silent Pass on Open Question 4)
* **The Risk:** Requirements Open Question 4 asks: *"For existing cities with in-flight sessions using prompts or formulas from retired paths, should the migration allow those sessions to finish with old materialized content, require an immediate restart after repair, or expose a separate operator decision?"*
* **The Gap:** The implementation plan confidently declares "Open Questions: None," yet it completely fails to define a concrete engineering solution for in-flight sessions or beads referencing deleted paths.
* **The Consequence:** When an old city is upgraded, active sessions referencing deleted template/formula file paths will suddenly crash or hang on their next step execution because those files (e.g., `examples/gastown/packs/maintenance/assets/scripts/reaper.sh`) have been deleted in Slice 7.
* **The Fix:** The plan must explicitly resolve Open Question 4. We recommend that the mutation coordinator either:
  1. Refuses to fix a city that contains active, unresolved in-flight sessions, or
  2. Embeds a rewiring shim in the runtime-state migration layer that dynamically redirects legacy path references to their new Core/Gastown locations during session adoption.

### 4. Dynamic Recipient Preflight and Silent Execution Failures in Generalized Scripts
* **The Risk:** Scripts like `reaper.sh` and `jsonl-export.sh` are generalized to consume recipients dynamically from formula/order metadata (§133–167).
* **The Gap:** If a recipient target is left unconfigured, is empty, or evaluates to `/`, executing commands like `gc mail send ""` or `gc mail send /` inside the shell scripts will cause unhandled script crashes.
* **The Fix:** All generalized shell scripts must perform preflight verification of their recipient variables. If the target is empty or invalid, the script must gracefully log an audit warning to `stderr` and skip mail execution (exiting with code `0`) rather than crashing the entire workflow.

### 5. Lack of Simulated Failure/Escalation Witness Fixtures in `test/packcompat`
* **The Risk:** `test/packcompat` verifies that Gastown workflows load and trigger (happy path).
* **The Gap:** Warning and escalation pathways (e.g., mail alerts to `mayor/` on reaper anomalies) represent the highest-risk operational code. Happy-path testing alone does not prove these paths are preserved.
* **The Fix:** Mandate that `test/packcompat` include behavioral-trigger fixtures that inject simulated errors/timeouts to explicitly force and verify warning/escalation pathways.

---

## Required Changes for Finalization

1. **CI Generator Baseline Strategy:** Amend §133–167 to specify how the behavior manifest generator validates against a frozen/baseline historical reference after the legacy folders are physically deleted.
2. **Network Lock Protection:** Update §245–274 to require a file-system level advisory lock (e.g., flock) to prevent multi-host mutation conflicts on shared storage.
3. **Resolve Open Question 4 (In-Flight Sessions):** Define the exact pre-upgrade gate or session rewiring behavior for active sessions referencing deleted legacy paths.
4. **Script Recipient Guard:** Mandate that generalized scripts handle empty/slash recipients by logging a warning and skipping execution gracefully.
5. **Add Failure-Path Witness Fixtures:** Update §414–428 to require testing of simulated failure/escalation paths under `test/packcompat`.
