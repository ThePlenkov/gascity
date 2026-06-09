# Ingrid Kovac — ZFC & Role-Neutrality Guardian Perspective Independent Review (Iteration 18 / Attempt 1)

**Verdict:** BLOCK

**Scope:** Zero hardcoded roles, Core role neutrality, `dog` exception containment, SDK self-sufficiency, Go-source migration guard coverage, cross-document consistency, and architectural coherence.

This independent review evaluates the Iteration 18 / Attempt 1 draft of the Core and Gastown Pack Split design (`.gc/design-review-inputs/core-gastown-pack-migration/design.md`) against `requirements.md` and the live codebase at the `rig_root` (`/data/projects/gascity`).

---

## Executive Summary

As Ingrid Kovac, the **ZFC and Role-Neutrality Guardian**, I am issuing a strict **Verdict: BLOCK** for Iteration 18 / Attempt 1 of the Core and Gastown Pack Split design.

While the current design snapshot introduces exceptional architectural advancements—specifically the new **Provider-Pack Continuity and Rewrite Exceptions** ledger (§2370–2391), **Strict Preflight Recipient Validation** (§1378–1381), and **Hermetic Offline Testing** (§1495, §2009, §3108)—it fails to resolve key internal contradictions and implementation-blocking loopholes that threaten our core design principle: **Zero Framework Cognition (ZFC) and Role-Neutrality**.

The design contains critical unresolved contradictions and edge cases that other reviewers might accept too quickly:
1. **The Dashboard `crew` Wire Vocabulary Contradiction (§2509 vs §3370):** The design specifies replacing the OpenAPI/dashboard `crew` vocabulary with neutral session grouping and regenerating TypeScript schemas. However, under Testing, it asserts: *"This migration should not require dashboard changes."* If wire types are regenerated and the `crew` or `agent_kind` fields change, the dashboard UI code (consuming these fields) will break unless it is updated. This remains an active inconsistency.
2. **Go-Source Comments inside Scanner Scope (§2489 vs Live Code):** Comments are explicitly within the scanner's matching scope. However, live code files such as `internal/api/handler_agents.go` at line 553 explicitly mention `mayor, witness` as role examples. Unless these comments are updated, removed, or allowlisted, they will trigger fatal scanner failures under the new rule.
3. **Core Asset Classification Specifics:** The design fails to provide a clear, exhaustive classification of Core maintenance assets into `controller_owned` (required SDK infrastructure) or `optional_core_maintenance_worker` (worker-bound). Without this explicit mapping, required SDK controller logic can easily be assigned to the optional maintenance worker, violating SDK self-sufficiency.

---

## Top Strengths of current design

* **`dog` Containerization via Bindings (§204–215):** The transition to the `[gc.bindings.maintenance_worker]` table with default `"dog"` successfully contains role logic in configuration. This ensures SDK self-sufficiency when renamed or omitted, and prevents `"dog"` from becoming a hardcoded Go special case.
* **Provider-Pack Continuity Ledger (§2595):** Resolves the prior contradiction regarding `bd` and `dolt` assets. It explicitly defines which files can be rewritten, how they map to `core.maintenance_worker` symbolic bindings, and what tests must prove their continuity.
* **Strict Preflight Recipient Validation (§1378–1381):** Enforcing that required recipient fields fail preflight if empty or `/` eliminates the risk of silent alert failures, securing Gastown's warning pathways.
* **Hermetic Offline Testing (§2169–2172, §2227):** Mandating that `test/packcompat` can run offline using local fixtures and remote caches prevents network dependency failures in air-gapped CI environments.
* **Table-Driven Participation Resolution (§2429–2436):** Typing `RequiredSystemPackParticipation` and returning explicit records for required host packs ensures resolver-driven participation is fully verified before any behavior discovery occurs.

---

## Critical Risks & Gaps

### 1. Dashboard `crew` Wire Vocabulary Contradiction
* **The Risk:** Wire-level fields like `crew` or `agent_kind` must be de-roled (§2509). Yet, the design asserts "this migration should not require dashboard changes" (§3370).
* **The Gap:** The live dashboard filters on `session.agent_kind === "crew"` (`cmd/gc/dashboard/web/src/panels/crew.ts:45`) and reads the OpenAPI schema for `agent_kind` (`docs/schema/openapi.json`). Changing the Go wire vocabulary without modifying the dashboard will immediately break API compatibility.
* **Recommendation:** Resolve this contradiction. Either authorize dashboard API updates in the rollout plan (including running `make dashboard-check`), or formally allowlist the specific Go wire/JSON struct tags required for backward compatibility with the current dashboard version.

### 2. Go-Source Comments inside Scanner Scope
* **The Risk:** The scanner is described as scanning "comments" and "prompt prose", rejecting sub-identifier surfaces of hardcoded roles (`mayor`, `witness`, etc.) unless a row allows them.
* **The Gap:** Active code files such as `internal/api/handler_agents.go` at line 553 contains the comment:
  `//   - "role" otherwise — a singleton agent (e.g. mayor, witness) that lives`
  This comment resides in Gas City Core Go code. Under Attempt 17's strict rules, this will trigger a fatal role-neutrality scanner violation when the Go files are scanned.
* **Recommendation:** Explicitly mandate in the design's Go migration plan that such comments must be updated or cleaned up to use neutral terms (e.g., "singleton agent" instead of "e.g. mayor, witness").

### 3. Core Asset Classification Missing
* **The Risk:** Omitting the Core maintenance worker disables worker-bound tasks while leaving controller-owned operations active.
* **The Gap:** The design lists generic infrastructure orders and scripts such as gate sweep, blocker-close cascade, stale cleanup/reaper, spawn storm detection, and binary doctor checks. However, it never classifies which assets are controller-owned versus worker-bound. Without a per-asset classification table, an implementation can easily break controller self-sufficiency by assigning a required SDK operation to the optional `core.maintenance_worker` binding.
* **Recommendation:** Extend the behavior manifest or role-surface table with a mandatory per-asset classification: `controller_owned`, `optional_core_maintenance_worker`, `public_gastown`, or `retired`. Mandate negative tests showing that when `maintenance_worker = ""` is configured, all `controller_owned` operations execute successfully.

---

## Evaluation of the Three Key Questions

### 1. Does any Go change introduce role-conditional logic or a literal role name outside tests, migration docs, or pack configuration?
**No, but active Go code today still has these role names, which must be systematically removed.**
The design asserts that absolute role neutrality will govern all Go source code (§2484–2514). Active role-name violations exist in the production tree today:
* **The Tmux Special Themes (`internal/runtime/tmux/theme.go:33,39,43`):** `MayorTheme()`, `DeaconTheme()`, and `DogTheme()` return hardcoded role names.
* **The Tmux Status Format Map (`internal/runtime/tmux/tmux.go:80–89`):** `roleEmoji` explicitly maps `"mayor"`, `"deacon"`, `"witness"`, `"refinery"`, `"crew"`, and `"polecat"` to emojis.
* **Warmup Defaults (`cmd/gc/cmd_start_warmup.go:33`):** `defaultWarmupMailTo = "mayor"` is hardcoded.

The Go migration table correctly targets these for replacement/deletion, but the scanner must be configured to split camelCase/PascalCase sub-identifiers (or execute substring searches) to ensure these are caught.

### 2. Does the Core role-name guard scan every asset type including scripts, overlays, orders, template fragments, doctor checks, metadata, and prompt snippets?
**Yes, but the scanner's matching rules must explicitly exclude the audit tables themselves.**
The scanner described in §2484–2518 correctly spans all asset types. However:
* **Audit Manifest Exception:** The role-surface table (`role-surface.generated.yaml`) and the behavior manifest (`behavior-manifest.generated.yaml`) *must* record literal role names for tracking purposes. The design must explicitly exempt these generated YAML metadata files from self-collision, otherwise the scanner will fail on its own audit tables.

### 3. Can Core infrastructure still run when the default maintenance agent is removed or renamed by configuration?
**Yes, but only if the boundary between controller-owned and worker-bound code is strictly enforced.**
The symbolic binding model (`[gc.bindings.maintenance_worker]`) is sound (§204–215). However, to prevent a regression where a developer writes a required controller-side SDK feature (like gate-sweeps or blocker-close cascades) to depend on the maintenance worker, a rigid per-asset classification and test asserting full functionality with an omitted worker are required.

---

## Required Changes for Finalization (Actionable Gates)

To lift this block, the design document must be updated to resolve these issues with the following concrete, actionable gates:

1. **Resolve Dashboard Wire Alignment:** Explicitly authorize the dashboard API and generated type updates in the rollout plan under a validated gate, or formally allowlist/preserve the exact JSON struct tags required for backward compatibility with the current dashboard version.
2. **Clean/Neutralize Go Comments:** Mandate the cleanup of hardcoded role references in Core Go comments (such as `internal/api/handler_agents.go:553`) prior to running the role-neutrality scanner.
3. **Classify Core Assets:** Provide an exhaustive, per-asset table classifying moved/retired Maintenance assets into `controller_owned` vs. `optional_core_maintenance_worker`. Require omitted-worker negative tests for all `controller_owned` assets.
4. **Self-Referential & Pack Exemptions:** Formally exempt the generated metadata audit tables (`role-surface.generated.yaml`, `behavior-manifest.generated.yaml`) from role-name rejection checks. Clarify that public Gastown assets are scanned for preservation inventory, not role-name rejection.
