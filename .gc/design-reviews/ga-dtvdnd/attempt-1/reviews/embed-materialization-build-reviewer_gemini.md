# Petra Novak - DeepSeek V4 Flash

**Verdict:** approve-with-risks

**Lane:** Builtinpacks registry, embed-path migration, materialization safety, Maintenance retirement, and downstream-reference closure.

Reviewed the **current** `plans/core-gastown-pack-migration/requirements.md` and `plans/core-gastown-pack-migration/implementation-plan.md` against `requirements.schema.md` (`gc.mayor.requirements.v1`), and re-verified every load-bearing code fact against the live tree this pass. The schema shape is conformant, and the progressive activation model from Level 0 to 8 is well-defined. 

However, looking at the build, embed, and materialization boundary, there are critical contradictions and subtle runtime edge cases that other reviewers might accept too quickly. The implementation must not proceed with source deletion or loader cutover until these gaps are closed.

---

## Live Code Base Inventory (Current Bad-State)
To ensure absolute alignment with the physical codebase, I re-verified the following build-seam locations today:
1. **Required Pack Hardcoding:** `cmd/gc/embed_builtin_packs.go` at line 237 hardcodes `required := []string{"core", "maintenance"}`, which unconditionally materializes and auto-includes Maintenance into every city.
2. **Builtin Pack Registry & Imports:** `internal/builtinpacks/registry.go` compile-imports `examples/gastown/packs/maintenance` at line 19, lists it in the `All()` slice at line 56, and recognizes `"gastown", "maintenance"` as public synthetic aliases at line 128. Core's embed source is pinned to `internal/bootstrap/packs/core` at line 53.
3. **Downstream Script Dependency:** `examples/dolt/assets/scripts/port_resolve.sh` at lines 4–6 sources `.gc/system/packs/maintenance/assets/scripts/dolt-target.sh`. `examples/dolt/port_resolve_test.go` at line 148 explicitly asserts against the maintenance copy.

---

## Consensus Strengths
- **Rigorous Schema Adherence (AC1):** The requirements now strictly adhere to `gc.mayor.requirements.v1` section order and meta-structures. It removes file-by-file assignments from the requirements document, avoiding premature implementation commitments.
- **Standalone Maintenance Retirement (AC5):** Explicitly states that Maintenance is no longer bundled, auto-included, materialized as active system pack, or chosen via lockfile resolution.
- **Configurable Maintenance Executor (AC9):** Decouples the Core maintenance execution from Go-side role assumptions. While `dog` can be defined as default pack configuration, the SDK is self-sufficient without it, satisfying Zero Framework Cognition (ZFC) principles.

---

## Critical Risks and Gaps (DeepSeek V4 Flash Focus)

While the proposed design is highly detailed, it contains critical build and materialization blind spots that must be addressed:

### 1. [Major] Direct Requirements-to-Design Mismatch on Core Relocation Path
- **Hazard:** `requirements.md` L30 states: *"For this migration, `internal/bootstrap/packs/core` is the sole canonical Gas City source root for the release-bundled Core pack."* This is reinforced by AC2 (L103). However, `implementation-plan.md` L17/L82 contradicts this by specifying: *"Move Gas City's required Core pack from `internal/bootstrap/packs/core` to `internal/packs/core`"*, and L467 deletes `internal/bootstrap/packs/core` entirely.
- **Consequence:** This is a hard requirements-to-design mismatch. The implementation will fail requirements-review validation if it deletes the bootstrap path, or the requirements document must be updated to formally authorize the relocation of Core to `internal/packs/core`.
- **Required Pin:** Align the two documents. The requirements should be updated to authorize the relocation to `internal/packs/core`, making `internal/packs/core` the sole canonical source authority, with compile-time deletion of the legacy bootstrap source tree serving as the proof of compile-time boundary enforcement.

### 2. [Major] The Upgrade-vs-Fresh Materialization Asymmetry (Stale Maintenance Leak)
- **Hazard:** `implementation-plan.md` L598 states: *"Do not delete stale `.gc/system/packs/maintenance`... directories during startup or `gc doctor --fix`. They are ignored by active discovery and reported as legacy state..."* 
- **Consequence:** If an existing city is upgraded, the stale `.gc/system/packs/maintenance/` directory will remain on disk. Because `examples/dolt/assets/scripts/port_resolve.sh` currently sources `.gc/system/packs/maintenance/assets/scripts/dolt-target.sh`, upgraded cities will continue to work *silently* because the stale file is left on disk. However, any **fresh** city initialized with the new binary will completely lack the `maintenance` pack on disk, meaning `port_resolve.sh` will instantly crash with a file-not-found error. This creates a split-reality bug that will pass local developer/upgrade tests but immediately break new installations in production.
- **Required Pin:** The design must enforce that **no** surviving script, support-pack file, or test may reference `.gc/system/packs/maintenance/` or `examples/gastown/packs/maintenance`. We must repoint `port_resolve.sh` to a surviving home (e.g. rehoming `dolt-target.sh` directly into the `dolt` support pack at `.gc/system/packs/dolt/assets/scripts/dolt-target.sh` or inlining it) and rewrite `port_resolve_test.go`. A static analysis scanner in `test/packlint` must verify that no occurrences of the retired path strings remain in active scripts or tests.

### 3. [Major] Directory-Level Materialization Safety & Concurrency
- **Hazard:** If this migration changes the materialization layouts under `.gc/system/packs/`, the extraction logic must be bulletproof. A naive implementation using direct `mkdir` and progressive copying is highly vulnerable to concurrent reader corruption (where another `gc` process reads a partially written directory) or failures mid-materialization.
- **Omission:** The updated requirements and design are completely silent on directory promotion safety during required pack materialization.
- **Required Pin:** The implementation must require **directory-level atomic materialization**:
  - Extract pack assets into a unique temporary sibling directory (e.g., `.gc/system/packs/.tmp-core-xyz`).
  - On successful extraction, perform an atomic directory rename (using `os.Rename` semantics) into the target path.
  - Clean up any stale/partial temporary staging directories on failure or startup.

### 4. [Minor] Go Bundling Seam and Registry Compile-Time Safety
- **Hazard:** The implementation plan specifies removing `maintenance` and `gastown` from the embedded set after the activation gate passes (Slice 6). However, because `internal/builtinpacks/registry.go` compile-imports `examples/gastown/packs/maintenance` (L19) and `gastown` (L18), the Go compiler will immediately fail if those directories are removed from the filesystem before the imports and `All()` registry entries are cleaned up.
- **Omission:** The steps do not explicitly coordinate the filesystem deletions in `examples/gastown/packs/` with the compile-time code changes in `registry.go` to prevent compile-time breakage during intermediate slices.
- **Required Pin:** The design must ensure that the filesystem deletion of `examples/gastown/packs/maintenance` and `packs/gastown` occurs in the *exact same change (Slice 6)* as the removal of their imports and registry entries in `internal/builtinpacks/registry.go` and `cmd/gc/embed_builtin_packs.go`.

### 5. [Minor] Transitive Diamond Conflicts on Public Imports
- **Hazard:** AC3 (L104) requires that duplicate public pack names or diamond conflicts with conflicting pins fail closed. However, the design doc is silent on how transitive dependencies are parsed during lock generation and resolution to detect diamond conflicts before downloading or caching.
- **Required Pin:** The design must clarify how `internal/systempacks` or the config resolution engine parses the transitive import graph to detect conflicting pins for public packs and prevent silent "last-one-wins" overwrites in the cache.

---

## Missing Evidence
1. **Dolt/Support Asset Ownership Destination:** The design lacks explicit rehoming destination mapping for `dolt-target.sh`. Since `dolt` is a surviving support pack, the design must state that `dolt-target.sh` rehomes directly to `examples/dolt/assets/scripts/dolt-target.sh` or is inlined.
2. **Wording/Path Static Scanner:** Explicit test specifications or command patterns for `test/packlint` proving that active code, scripts, or tests are scanned to ensure they do not reference retired paths.
3. **Atomic Materialization Invariants:** Code patterns or specifications illustrating staging-and-rename mechanics during required pack materialization under `internal/systempacks`.

---

## Required Changes
1. **Align Core Path Invariants:** Update `requirements.md` (L30, AC2) to permit the relocation of Core's embed source to `internal/packs/core` so that the implementation plan does not violate the requirements.
2. **Repoint and Rehome `dolt-target.sh`:** Formally rehome `dolt-target.sh` to the surviving `dolt` pack directory and update `port_resolve.sh` and `port_resolve_test.go` to point to the new location, completely eliminating the dependency on the retired `maintenance` pack path.
3. **Enforce Atomic Directory-Level Materialization:** Require that required pack materialization uses process-unique temporary sibling staging, atomic renaming, and fail-clean directory promotion.
4. **Coordinate Registry and FS Deletion:** Combine the registry compile-time cleanup and the filesystem deletion of retired pack directories into a single atomic slice (Slice 6) to avoid compile-time build failures.
5. **Add Path-String Scanner:** Add a lint check to `test/packlint` that fails if any active, non-allowed script or test file contains the literal string `.gc/system/packs/maintenance` or `examples/gastown/packs/maintenance`.

---

## Verdict Calibration / Rationale
I choose **approve-with-risks**. The overall architecture of `internal/systempacks`, `internal/doctorfix`, and the progressive activation slices are outstanding and highly robust. However, the "Stale Maintenance Leak" is a major, hidden runtime risk that will lead to extremely frustrating, non-reproducible bugs (passing on upgrades but crashing on new setups) unless we explicitly repoint the dolt-support scripts and add a path scanner. Aligning the canonical Core paths between requirements and design is also a prerequisite for successful gate verification.

---

## Questions
- **`dolt-target.sh` Ownership:** Should `dolt-target.sh` be owned directly by the `dolt` support pack, or is it better to merge its logic directly into `port_resolve.sh`?
- **Relative Imports in Gastown Template:** In the public Gastown pack, how is the legacy relative import `[imports.maintenance] source = "../maintenance"` resolved when Maintenance is not a public-source recognized pack? Will the public Gastown pack inline those behaviors?
