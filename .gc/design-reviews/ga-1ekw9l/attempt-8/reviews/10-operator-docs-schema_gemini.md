# Claire Dubois — Operator Docs and Schema Review (Attempt 8, Independent DeepSeek V4 Flash Style)

**Verdict:** approve-with-risks

> **Re-grounding & Verdict Evolution Note:** The prior Gemini review at this path **blocked** the design on (1) schema non-conformance and (2) tutorial under-enumeration. The author has delivered a 835-line schema-conforming rewrite (`implementation-plan.md`) that resolves the layout blockers, incorporates a consolidated canonical vocabulary, names the correct tutorials, and establishes a robust mutation coordinator. Consequently, the structural **block is lifted**. However, this independent DeepSeek V4 Flash analysis identifies critical operational, sequencing, and data-safety risks that the compression introduced, which other reviewers have accepted too quickly.

**Lane:** Operator upgrade DX, terminology matrix, docs/schema generated artifacts, doctor messages, tutorial integrity. Reviewed `implementation-plan.md` against `requirements.md` in the current workspace.

---

## Schema Conformance

Conforms to `gc.mayor.implementation-plan.v1`. Front matter carries the required keys with `phase: implementation-plan`, a valid `requirements_file`, and no `design_file`. All seven required top-level body sections (Summary, Current System, Proposed Implementation, Data And State, Testing, Rollout And Recovery, Open Questions) are present, correctly named, and in the correct order. The Summary is a concise 4-sentence overview, and `Open Questions` is explicitly `None`. No appended attempt/review summaries or external implementation assignments exist within the artifact body. 

One source-grounding defect remains within the plan: the Current System section (line 74) cites `docs/reference/system-packs.md` as currently asserting the legacy shape, but this file does not exist in the repository's docs tree. Line 508 correctly acknowledges that the file does not exist and says "the slice creates it", which directly contradicts the update claims on lines 74 and 521. This is noted as a risk below.

---

## Top Strengths of the Design

- **Explicit Terminology Matrix Integration (Lines 504–507 & 512–516):** 
  The updated plan incorporates a concrete terminology matrix (`terminology-matrix.yaml`) categorizing token classes (Core, provider host packs, retired Maintenance, valid lowercase/store maintenance). This prevents blunt denylists from causing massive false positives and subsequent wholesale allowlist suppression.
- **Defeating the "Lint-Stale-Output" Race Condition (Lines 517–520, 676–678):** 
  The plan now explicitly sequences generation commands (OpenAPI, dashboard types, schemas, help references, tutorial transcripts, and doctor output goldens) *before* the wording linter runs. This ensures that CI is scanning fresh, current generated assets rather than stale artifacts.
- **Structured Mutation Coordinator and Lock-Guarded Execution (Lines 361–390, 570–573):** 
  Replacing legacy direct mutation with a multi-file transactional `FixIntent` + coordinator API ensures that half-migrated/corrupt cities are prevented. The city-level advisory lock prevents concurrent binary conflicts during automated operations.
- **Ignores Legacy Paths and Protects Operator Edits (Lines 353–356, 598–601):** 
  Stale `.gc/system/packs/maintenance`, `.gc/system/packs/gastown`, and `.gc/runtime/packs/maintenance` directories are explicitly ignored by active discovery rather than deleted, protecting potential operator custom edits and manual work.

---

## Critical Risks & Architectural Inconsistencies

### 1. [Major] Phantom File Reference: `docs/reference/system-packs.md` Still Targeted for "Update" (Lines 74, 521)
- **The Risk:** The plan lists `docs/reference/system-packs.md` for "Update" (line 521) and states it "currently assert[s] the legacy shape" (line 74). However, this file **does not exist** in the repository's docs tree. Line 508 correctly acknowledges that the file does not exist and says "the slice creates it", which directly contradicts the update claims on lines 74 and 521. A decomposer attempting to update a non-existent file will face a direct blocker or produce a broken path.
- **The Impact:** Unactionable decomposition steps and broken file references.
- **Recommended Action:** Mark `docs/reference/system-packs.md` as "Create" rather than "Update" in all instances and reconcile the contradiction.

### 2. [Major] Absence of Headless/Non-Interactive Flag for `gc doctor --fix`
- **The Risk:** While the requirements (AC10, AC11) explicitly mandate `gc doctor --fix --non-interactive` as the canonical mutating repair surface for headless, automated recovery and CI pipelines, the proposed CLI details (Lines 355, 394, 659, 715) only define a bare `gc doctor --fix` and do not specify the `--non-interactive` flag or its behavior.
- **The Impact:** Without a documented non-interactive flag, headless scripts may hang indefinitely waiting on stdin confirmation.
- **Recommended Action:** Explicitly define and specify the `--non-interactive` (or `--yes`) flag for `gc doctor --fix` in the Proposed CLI and testing sections to guarantee non-interactive safety.

### 3. [Major] Undefined "Deterministic Re-Upgrade Flow" for Post-Marker Writes (Lines 405–407, 577–578)
- **The Risk:** The plan specifies that if an old binary writes to legacy paths after the migration marker, the new binary reports a version-skew diagnostic and requires a "deterministic re-upgrade flow" (lines 405–407). However, the mechanics of this flow and how old-binary writes are actually detected are completely unspecified.
- **The Impact:** Operators facing post-migration writes will have no safe or documented way to merge the old binary's stale state back into the new Core-owned paths, creating a split-brain data corruption hazard.
- **Recommended Action:** Specify the exact detection mechanism (e.g. comparing file modification times or epoch numbers) and document the re-upgrade merge rules or declare it a strict one-way boundary with explicit warnings.

### 4. [Minor] Ambiguous Location and Absence of the Terminology Matrix in Support Set (Lines 556–562)
- **The Risk:** The Proposed Implementation section names `terminology-matrix.yaml` as "the vocabulary authority" (line 505) and describes its schema (lines 512–514), but the required support set (lines 556–562) completely omits `terminology-matrix.yaml`.
- **The Impact:** Potential misalignment of deliverables across teams during decomposition/implementation.
- **Recommended Action:** Reconcile the discrepancy by explicitly adding `terminology-matrix.yaml` to the required support set list.

### 5. [Minor] Lack of Tmux Isolation Guards for Local/CI Testing (Lines 456–457)
- **The Risk:** The plan moves role-theme and tmux behavior, and states that "Tmux cleanup examples and tests must target isolated sockets only and must never use a default-server kill" (lines 456–457). However, it fails to specify a strict testing check or linter to enforce this.
- **The Impact:** If tests are misconfigured, they could destroy active developer sessions or CI host environments.
- **Recommended Action:** Specify an automated check (e.g., AST/path scanner or a linter gate) to reject bare `tmux kill-server` and enforce isolated sockets (`tmux -L <socket>`) in all test-cleanup paths.

---

## Detailed Responses to Lane-Specific Questions

### Q1: Do docs, doctor output, CLI help, generated references, tutorials, and schema artifacts use the same Core, Gastown, retired Maintenance, and store-maintenance vocabulary?
**Answer:** **Yes, conceptually, but the mechanical enforcement is fragile.** The plan consolidates vocabulary on lines 526–530, which is excellent. However, the plan omits explicit listings of JSON schemas (`pack-schema.json`, `city-schema.json`) and generated TypeScript files from the wording targets. Stale references in these auto-complete files will directly contradict the documentation. The linter *must* validate all generated dashboard types and JSON schemas.

### Q2: Is the wording matrix executable enough to distinguish retired pack references from legitimate lowercase maintenance or Dolt/store maintenance terms?
**Answer:** **Yes, with the proper parser/classifier.** The introduction of `internal/packsource` as the sole classifier (lines 320–327) resolves the risk of blunt keyword matching. By returning typed states (e.g., `active bundled`, `stale cache`, `retired custom/fork`), the classifier can distinguish the retired system pack from legitimate store settings (`[maintenance.dolt]`) or the Core `maintenance_worker`. Plain text grep is replaced by semantic token classification.

### Q3: Can a new or upgrading operator follow one narrative from missing orders or stale packs through doctor diagnostics, public pin verification, and recovery?
**Answer:** **Yes, but the upgrade/recovery narrative remains slightly scattered.** The plan links doctor output, `FixHint` objects, and version-skew diagnostics. However, because it lacks a single canonical "Operator Upgrade and Troubleshooting Guide" markdown file, the operator has to look at `troubleshooting.md`, `system-packs.md`, and CLI output separately.

---

## Evaluation Against Lane Anti-patterns

| Anti-pattern / Red Flag | Mitigation in Current Design | Status |
| :--- | :--- | :--- |
| **Operator-facing behavior changes ship with stale docs or non-release docs debt** | **Excellent.** Section sequencing places wording lint and artifact generation in the same slice as behavioral changes. | **Pass** |
| **Wording lint creates noisy false positives and gets suppressed wholesale** | **Excellent.** Correctly mitigated by the hierarchical structure of `terminology-matrix.yaml` and semantic token contexts. | **Pass** |
| **Doctor diagnostics, tutorials, or generated schema still point operators to Maintenance or in-tree Gastown** | **Excellent.** Stale paths are ignored, and tutorials are targeted for comprehensive role-neutral reframing. | **Pass** |

---

## Missing Evidence

1. **Circular Dependency Proof:** The concrete CLI commands and targets showing that generators run before the wording scanner in CI.
2. **Deterministic Re-upgrade Protocol:** The exact merging strategy or warnings presented when a post-migration write by an old binary is detected.
3. **Wording Matrix Schema with Token Classes:** Structured matrix detailing Core, provider host packs, retired Maintenance, valid lowercase/store maintenance, and stale generated paths.
4. **Doctor-Output Goldens:** Verification golden fixtures for legacy import-state, stale generated-pack, missing public pin/cache, and no-Maintenance loading.
5. **Tutorial 05/07 Disposition:** The behavior-evidence authority mapping `mol-dog-*` orders and formulas (whether they stay Core-only or require Dolt/Gastown provider behavior).

---

## Required Changes

1. **Terminology Matrix & Classification:** Restore a positive and negative term matrix for operator docs and generated artifacts. Require the wording scanner to classify hits by token and context to allow legitimate store/lowercase maintenance and reject retired pack references.
2. **CI Sequencing & Generated-Artifact Freshness:** Define generated-artifact ownership and freshness gates by artifact class (including exact commands and CI order). CI must regenerate OpenAPI, dashboard types, docs/schemas, CLI help, tutorial transcripts, doctor output, and public companion docs before wording lint.
3. **Operator Docs Inventory & Recovery Guide:** Clarify if `docs/reference/system-packs.md` is a new page. Provide a single recovery guide/guide section that threads `gc doctor`, public pack pin verification, ignored legacy directories, manual cleanup, and stale-pack diagnostics.
4. **Doctor-Output Golden Fixtures:** Add doctor-output golden fixtures for legacy import-state diagnostics, stale generated-pack diagnostics, missing public pin/cache, and successful no-Maintenance loading.
5. **Tutorial 05/07 Transcript Goldens & Disposition:** State the tutorial 05/07 order/formula disposition using the behavior-evidence manifest as authority. Require transcript goldens for tutorials 05 and 07 in both minimal and Gastown-template cities.
6. **Non-Interactive Flag:** Define the exact CLI flag (e.g., `--non-interactive`) used to run `gc doctor --fix` in automated, non-interactive environments.
7. **Rollback Warning and Boundaries:** Update the Runtime-State Migration section to explicitly state whether downgrade to an old binary is a one-way boundary, and specify the exact text of the warning printed when a post-marker write is detected.
8. **Ledger Path & Tmux safety:** Name the exact path (`plans/core-gastown-pack-migration/artifacts/asset-migration-ledger.yaml`), verify it in `packlint`, and explicitly restrict tmux test scripts to isolated sockets.
