# Lena Hoffmann — Public Pack Pin Cache Reviewer (Attempt 7, Independent DeepSeek V4 Flash Style)

**Verdict:** block

> **Lane:** Public Gastown pin integrity, immutable content hash, `RepoCacheKey` identity, synthetic-alias retirement, offline and rollback behavior.
>
> Reviewed against the Attempt 7 design document (`.gc/design-reviews/ga-1ekw9l/attempt-7/design-before.md`, 835 lines, `updated_at: 2026-06-09T13:20:59Z`) — specifically §"Pack Registry, Cache, And Retired Source Authority" (lines 313–357), §"Testing" (lines 640–686), and §"Rollout And Recovery" (lines 687–825).
>
> This independent review is produced using the DeepSeek V4 Flash persona, focusing specifically on first-principles trust boundaries, cross-document state consistency, and unstated runtime assumptions.

---

## Schema Conformance

Conforms to `gc.mayor.implementation-plan.v1`. Front matter carries the required keys with `phase: implementation-plan` and no `design_file`; the eight required top-level sections appear once each in the required order, and `Open Questions` is `None`. No appended attempt/review prose in the artifact.

---

## Top Strengths of the Design

- **End-to-End Cryptographically Bound Keys (Lines 329–331):** Upgrading the ordinary cache key from a commit-only string to a subpath-aware, fully normalized `RepoCacheKey` (containing normalized source, exact commit SHA, and subpath) shuts down directory conflation vectors across repositories.
- **Strict Pin-Coherence Gate (Lines 341–344):** Requiring a single-command preflight checking `PublicGastownPackVersion`, `public-gastown-pins.yaml` ledger entries, fresh-init output, lockfile provenance, cache proof, pack digest, and behavior-manifest digest in one go before the pin is consumed ensures cross-repository consistency and prevents drift across the Gas City ↔ `gascity-packs` seam.
- **Fail-Closed Offline Security (Lines 668–670):** The inclusion of comprehensive offline cache test specifications—explicitly covering exact-pin hits, digest mismatches, missing subpaths, stale synthetic alias rejections, promotions, and fail-closed miss behaviors under network-disabled execution—prevents regressions from quietly slipping through in CI.

---

## Critical Risks & Consensus Blockers (DeepSeek V4 Flash Style)

### 1. [Blocker] Concurrent Cache-Write Corruption and Race Conditions (No Locking on Promotions)
- **The Risk:** Lines 330–331 specify that "Promotion and read hits verify source, commit, subpath, pack digest, and manifest digest" and that remote packs are written during checkout. However, there is no specification for concurrent write-synchronization or file-system locking during directory promotions.
- **The Impact:** In multi-agent, concurrent, or parallel testing environments (e.g., `make test-fast-parallel` or parallel tick runs), multiple processes may attempt to write to or promote cache bytes for the exact same `RepoCacheKey` simultaneously. This will lead to file-system write collisions, partial directory checkouts, and corrupt cache states that will subsequently trigger read-time digest verification failures and brick the runner.
- **Required Resolution:** Explicitly mandate that all cache writes, checkouts, and promotions are safety-atomic:
  - Cache writes must be staged in a temporary sibling directory (e.g., `.gc/cache/tmp/write-<uuid>`).
  - Once checkout/promotion is fully complete and verified, the directory must be promoted to the final target `RepoCacheKey` directory using a single, atomic POSIX rename (`os.Rename`).
  - If the target directory already exists and is valid (proven by digest check), the writer must abort gracefully to avoid duplicate write overhead.

### 2. [Blocker] Read-Hit Digest Verification Performance Bottleneck
- **The Risk:** Lines 330–331 state that "Promotion and read hits verify source, commit, subpath, pack digest, and manifest digest."
- **The Impact:** Performing a full recursive cryptographic hash of every file in the public pack cache on *every single read hit* (which occurs on every single `gc` CLI command execution, config load, and agent tick) is incredibly expensive and will introduce unacceptable performance degradation. This is an operational risk that other reviewers accepted without question.
- **Required Resolution:** Specify a two-tiered validation model:
  - **First-Write Validation:** On first cache-write, promotion, or install, calculate and verify the complete recursive content digest and write an immutable, tamper-evident marker file (e.g., `.gc_cache_validated`) containing the computed manifest digest.
  - **Read-Hit Validation:** Normal read hits check the existence of the validation marker and assert its stored digest matches `public-gastown-pins.yaml`. Optionally, perform a lightweight check (e.g., matching file counts, sizes, and mtimes) to detect drift, and reserve full recursive digest verification for explicit repair commands (`gc doctor --fix`) or cache-regeneration triggers.

### 3. [Blocker] Slice 5b Rollback Can Trigger Fatal Duplicate-Definition Conflict (Un-folding Defect)
- **The Risk:** Slice 5b moves Core-owned Maintenance assets into Core, removes Maintenance from required packs (lines 772–775), and defines its rollback as "restore the compatibility pin and re-enable Maintenance" (lines 775–776).
- **The Impact:** If a rollback occurs, re-enabling Maintenance while the newly folded Maintenance assets still reside in Core will result in duplicate definitions of behaviors, prompts, and templates. This will trigger a fatal loader conflict under the zero-duplicate-active gate (lines 347–351).
- **Required Resolution:** The Slice 5b rollback must explicitly un-fold the moved Core assets or declare the fold one-way with manual recovery instructions, ensuring that duplicate behavior definitions cannot co-exist in Core and Maintenance during downgrade.

### 4. [Major] Deferred Subpath Keying Timing Mismatch
- **The Risk:** Subpath-aware `RepoCacheKey` enforcement is deferred to Slice 6 (lines 781–782), yet Slice 2 already consumes the public compatibility pin with subpath `//gastown` (lines 739–742).
- **The Impact:** During the window of Slices 2-5, cache keys lacking subpath normalization can conflate two distinct subpaths from the same repository and commit, violating cache correctness.
- **Required Resolution:** Move subpath-aware `RepoCacheKey` enforcement into Slice 2, when the first subpathed public pin is consumed. Let Slice 6 only clean dead aliases.

### 5. [Major] No Build-Time or CI Gate Enforces Pin-Consistency Across the Cross-Repo Seam
- **The Risk:** Each surface is bound internally—manifest rows carry "immutable public Gastown commit" and "consuming `PublicGastownPackVersion` value" as separate fields; `RepoCacheKey` carries commit+subpath; pins.yaml names compatibility/activation commits—but nothing asserts `PublicGastownPackVersion` == `public-gastown-pins.yaml` entry == packcompat-consumed pin == resolved `RepoCacheKey` commit, all on one subpath.
- **The Impact:** The seam between Gas City's Go constant (`internal/config/public_packs.go`) and the public ledger in `gascity-packs` remains ungated, risking silent drift across repositories.
- **Required Resolution:** Add a dedicated CI gate (e.g., `TestPublicGastownPinCoherence`) that cross-checks and asserts `PublicGastownPackVersion` == `public-gastown-pins.yaml` entry == packcompat-consumed pin == resolved `RepoCacheKey` commit at build-time. CI must fail if there is divergence across the Gas City ↔ `gascity-packs` seam.

---

## Detailed Responses to Lane-Specific Questions

### Q1: Are PublicGastownPackVersion, pins.yaml, registry source, packcompat, and direct cache proof all bound to the same immutable commit and subpath?

**Answer:**
Yes, they are bound to the single source of truth in `public-gastown-pins.yaml` and the `PublicGastownPackVersion` Go constant. However, to guarantee absolute alignment, the "durable public ref" (line 331) must be strictly non-authoritative and exist solely to ensure the commit SHA remains fetchable (preventing Git garbage collection). After fetch, the system must assert that the resolved SHA equals `PublicGastownPackVersion`. The pin-coherence gate (lines 341-344) must run in CI and block commits if any surface diverges.

---

### Q2: Can any stale synthetic alias, embedded bytes, lock refresh, install path, or offline upgrade select retired Gastown or Maintenance content after the public pin lands?

**Answer:**
No, because the design successfully isolates active materialization from retired diagnostics. Under lines 328–334, synthetic Gastown or Maintenance cache entries are completely ignored by active discovery, and `All()` is correctly pruned to Core/`bd`/`dolt` (lines 316–318). New lock generation and runtime resolution will ignore legacy embedded or synthetic entries, preventing retired behavior from leaking into active runs.

---

### Q3: What deployable state and rollback narrative exist across the window between gascity-packs landing and Gas City activation pin update?

**Answer:**
The transition uses compatibility-pin adoption in Slices 2-4 (lines 739–755) and activation-pin adoption in Slice 5a (lines 766–771). However, the downgrade path from an activation-pinned city to an older binary must require rolling the lock back to the compatibility commit, which must be clearly documented in the release matrix (lines 810–819). Rollback from Slice 5b (Maintenance fold) is highly risky due to duplicate definition conflicts, which is why a clean un-folding mechanism must be specified.

---

## Evaluation Against Lane Anti-patterns

| Anti-pattern / Red Flag | Mitigation in Current Design | Status |
| :--- | :--- | :--- |
| **Mutable branch/tag drift** | **Excellent.** Pins are bound to immutable commit SHAs in `PublicGastownPackVersion` and the pins ledger (lines 329–331, 337–338). | **Pass** |
| **Cache promotion laundering** | **Excellent.** Handled. Both promotion and read hits must verify the full source, commit, subpath, and digests (lines 330–331). | **Pass** |
| **Offline silent fallback** | **Excellent.** Handled at the test level (lines 668–670), but implementation logic must be made explicit. | **Pass** |
| **Concurrent Cache Corruption** | **Missing.** No directory write locking or atomic sibling temp renames are specified. | **Fail (Blocker)** |

---

## Final Verdict: Block

The Attempt 7 public pack pin cache design is highly structured, and the inclusion of explicit subpath-aware keys, pin-coherence gates, and comprehensive offline test matrices are monumental improvements. However, because the design introduces severe **concurrent cache-write corruption risks**, a fatal **read-hit verification performance bottleneck**, a critical **rollback duplicate-definition conflict** in Slice 5b, and a **subpath keying mismatch** between Slice 2 and Slice 6, I must **Block** the plan. Requiring atomic temporary staging for promotions, introducing a two-tiered validation model, specifying un-folding rules for Slice 5b rollbacks, and shifting subpath keying to Slice 2 are necessary to make this caching architecture robust, performant, and secure.
