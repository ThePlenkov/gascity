# Ingrid Holm — DeepSeek V4 Flash (Independent Review, Attempt 17)

**Verdict:** block

**Persona:** Ingrid Holm, Operability, Performance, and Diagnostics Reviewer. Lane: decision observability, trace and doctor diagnostics, fact read cost, and event fan-out load.

**Reviewed against:** `internal/session/DESIGN.md` (Attempt 17, response to Attempt 16), `internal/session/REQUIREMENTS.md`, `internal/session/AGENTS.md`, and current checkout source.

---

## Overview

As the **10-operability-performance-diagnostics** lane reviewer (Ingrid Holm), this independent review evaluates the **Attempt 17 response revision** of `internal/session/DESIGN.md`.

The Attempt 17 design makes outstanding structural progress by formalizing the 8-step target precedence resolver, establishing a strict ban on flat optional envelopes for target-classification outputs, isolating Mail and Extmsg as "Characterization only" to prevent early scope-creep, and requiring an explicit `DIAGNOSTICS_MANIFEST.yaml` for trace mappings.

However, from an **operability, performance, and diagnostics** perspective, the design remains a **BLOCK**. It continues to defer critical performance-safety choices (such as the physical indexing of all-session scans, the caching of reconciler facts, or the decoupling of synchronous event emissions) to individual implementation slices. Furthermore, it leaves the execution and non-blocking recovery lifecycle of read-path repairs completely undefined, presenting a severe risk of host CPU exhaustion and process table starvation under real production workloads.

---

## Top Strengths

1. **Clean Read-Path Repair Separation (`DESIGN.md:283-288`):**
   Prohibiting `RepairEmptyType` from silently executing write side-effects on read-only target classification paths and instead returning an explicit `repair-needed` result kind is a major operability win. It preserves classifier purity and keeps write boundaries clean.
2. **Explicit Trace Mappings in `DIAGNOSTICS_MANIFEST.yaml` (`DESIGN.md:678-693`):**
   Requiring every diagnostic row to explicitly map to a `gc trace` site/reason/outcome record ensures that shifting logic to pure functions does not create a diagnostic signal blackout. This prevents silent failures and preserves operator-visible traces.
3. **Structured Boundary Fresh/Stale Rules (`DESIGN.md:580-595`):**
   Adding explicit freshness, unknown, stale, and provider-error fact-handling requirements to `BOUNDARY_MATRIX.yaml` ensures that the reconciler and session deciders have defined, predictable failure-handling behaviors rather than relying on ad-hoc error propagation.

---

## Critical Risks & Blockers

### 1. [Blocker - Performance] Unresolved All-Session Scan Hazard on first-adopter Path (`resolveLiveSessionByPathAlias`)
* **Evidence:** `DESIGN.md:581` lists `resolveLiveSessionByPathAlias` under "Required budget rows before delegation" with:
  > `Decision to index, remove, or keep with explicit scan budget. If kept, budget must name maximum session rows scanned and prove newest-created tiebreaker behavior on a large fixture.`
* **Why it matters:** Deferring this critical architectural decision to individual implementation slices is a severe hazard. `resolveLiveSessionByPathAlias` (invoked on the first-adopter API query path) currently executes an unindexed, full `beads.Store` scan, filtering every session bead in memory by `Title`.
  
  In the production `BdStore` backend, this triggers **two process forks (`bd list`)** per invocation. On any large city containing thousands of historical or inactive sessions, this unindexed all-session scan will saturate host CPU and disk I/O. Leaving this choice open means the first-adopter slice can legally ship with an unindexed full scan simply by "budgeting" it, violating our core performance-safety invariants.
* **Required Change:** The design must make a final architectural decision: **either remove path-alias resolution from the first adopter's scope entirely, or mandate a proper database-level index on `Title` before delegation.** Simply budgeting a process-forking all-session scan on an API query hot path is unacceptable.

### 2. [Blocker - Operability] Unspecified Execution and non-blocking Lifecycle for `repair-needed` Reads
* **Evidence:** `DESIGN.md:283-288` states that when target classification encounters an empty-type session bead, the read path returns `repair-needed` and delegates the write to a separate, audited repair command.
* **Why it matters:** The design completely avoids specifying **how, when, and by whom the repair command is triggered**:
  1. If the API handler synchronously triggers the repair command, the read path is no longer side-effect-free, which violates the target classifier's fundamental contract (`DESIGN.md:247`).
  2. If the API handler rejects the query and returns a `404` or `500` until an external cron or human operator triggers the repair, this introduces a severe availability regression for previously readable sessions.
  3. If the repair runs asynchronously in the background, the design fails to specify what prevents a split-brain write race if a mutating command (like `gc wake`) targets that same unrepaired session bead concurrently.
* **Required Change:** Authoritatively define the repair execution lifecycle: specify whether repairs are triggered via non-blocking asynchronous worker queues, reconciler ticks, or background handlers, and detail the concurrency gates that protect the bead while a repair is pending.

### 3. [Blocker - Performance] Lack of concrete Caching or batching Mechanisms for Reconciler hot loops
* **Evidence:** `DESIGN.md:582` requires a budget row for Reconciler Fact Compilation:
  > `Store query count, subprocess count, runtime probe count, maximum session/work rows scanned, partial-snapshot behavior, proof command, and owner.`
* **Why it matters:** Every reconciler tick compiles facts by querying the store. Because `BdStore` list queries fork a `bd` subprocess, executing repeated queries per tick in a hot reconciler loop will lead to extreme CPU starvation and process table exhaustion on the host.
  
  Similar to the path-alias scan, the design lists this budget row as a *requirement* but **does not specify any caching, incremental fact compilation, or bulk read mechanism** at the architectural level. Defining a budget does not physically prevent process fork fatigue; the design must provide the structural mechanism to meet that budget.
* **Required Change:** Mandate a specific, shared fact-caching or snapshot-read mechanism (such as an in-memory TTL cache, a bulk session state reader, or an incremental fact compilation adapter) that reconciler ticks must use to prevent process fork fatigue.

### 4. [Major - Diagnostics] Collapse of granular WakeCauses/Blockers in Decider Diagnostics
* **Evidence:** The structured diagnostic result fields (`DESIGN.md:666-676`) list only a generic "reason code," and the Target Classification result schema (`DESIGN.md:302-311`) covers target-resolution kinds, not lifecycle wake-cause or blocker.
* **Why it matters:** `ProjectLifecycle` already produces a typed `WakeCause` enum (`lifecycle_projection.go:120-138`: `pending-create`, `pin`, `attached`, `pending`, `named-always`, `work`, `scale-demand`, `explicit`) and typed blockers (`held`, `quarantined`, `missing-config`, `identity-conflict`, `duplicate-canonical`). 
  
  Collapsing these highly descriptive, granular, operator-facing "whys" into a generic string "reason code" regresses operator explainability from machine-readable diagnostics back to opaque prose, making automated diagnostics and log parsing difficult.
* **Required Change:** Require the diagnostic result structure to carry the typed `WakeCause` and blocker enums rather than a flattened reason string, and require parity tests to prove that operator explainability does not regress.

### 5. [Major - Performance] Unbudgeted Synchronous Event Emission Serialized Behind Cross-Process Locks
* **Evidence:** `DESIGN.md:636-645` inventories the `session.*` events, but the budget table (`DESIGN.md:708-715`) only rows "Event recovery scans" (a durable scan), leaving the synchronous emission path unbudgeted.
* **Why it matters:** In the current implementation, events are emitted via `rec.Record(events.Event{...})` inline at 9+ reconciler sites. `FileRecorder.Record` acquires a cross-process advisory `flock` with a 250ms bounded wait per call. Under high contention or in a large city, per-session emissions can serialize behind this lock, stalling the reconciler pass.
* **Required Change:** Add an event-emission budget row to the table (distinct from "Event recovery scans") that names the current synchronous flock `Record` cost and proves that the reconciler hot loop does not serialize N per-session emissions behind a blocking cross-process lock.

### 6. [Major - Separation of Concerns] Inappropriate Mixing of Reconciler-Specific requirements into Session Slice 0
* **Evidence:** `DESIGN.md:196-202` requires that Session Slice 0 must repair or owner-retire evidence for `SESSION-RECON-002`, `SESSION-RECON-003`, `SESSION-RECON-006`, and `SESSION-RECON-007` before a later slice cites those rows.
* **Why it matters:** Under `DESIGN.md:550-563` (the Session/Reconciler split), pool scaling, provider health gates, and progress-aware thresholds are explicitly reconciler/pool behaviors that live *outside* `internal/session`.
  
  Forcing the Session-specific Slice 0 gate to block on, validate, and repair reconciler-specific requirement evidence violates the separation of concerns. It unnecessarily delays the delivery of non-mutating Session Slice 0 code by coupling it to complex, unrelated reconciler state machines.
* **Required Change:** Remove the reconciler-specific requirements (`SESSION-RECON-*`) from the Session Slice 0 entry criteria, moving them to their respective reconciler or pool-focused implementation backlogs.

---

## Answers to Persona Questions

### 1. Can operators explain why a session was blocked, woken, drained, or closed from decider output and trace evidence alone?
* **Answer:** With the current design, **no, not completely**. Because the design collapses the granular, typed `WakeCause` and `Blocker` enums into a flat, generic diagnostic "reason code," operators will lose the exact machine-readable "why" behind transitions. While `DIAGNOSTICS_MANIFEST.yaml` maps reasons to `gc trace` sites, a flattened reason string regresses operator observability compared to the current typed projection model.

### 2. What do gc trace, conflicts, and event logs show when a decision is rejected or an event is missed?
* **Answer:** When a decision is rejected, `DIAGNOSTICS_MANIFEST.yaml` maps the structured `diagnostic_code` onto a `gc trace` site/reason/outcome record, ensuring the rejection is visible in `gc trace` and `doctor` surfaces.
  
  When an event is missed, the system relies on the **Durable Scan Contract** (`DESIGN.md:628-634`). Critical actions (such as work release and close) are designed to converge from durable facts scanned periodically by a "durable scan owner," ensuring recovery even when in-process event delivery is completely lost. However, the design does not specify whether these recovery scans are visible to operators via `trace` or `doctor`, meaning recovery actions may happen silently in the background.

### 3. What is the reconciler cost of materializing facts and emitting subscriber events across a large city?
* **Answer:** **Extremely high and unsustainably expensive.** Because each store query in `BdStore` forks an OS subprocess, compiling facts per-session on every reconciler tick will quickly saturate host resources. While `DESIGN.md:582` mandates a budget row for reconciler fact compilation, the design lacks concrete caching or incremental compilation mechanisms, making the real-world operational cost of fact materialization unsafe for large cities.

---

## Consistency & Parity Report

* **Requirements Alignment:** Under `REQUIREMENTS.md`, exact target resolution precedence must be preserved. While the classifier precedence matrix in `DESIGN.md:257-281` matches this perfectly, the performance penalty of path-alias lookups (`resolveLiveSessionByPathAlias`) introduces a severe operational regression that violates system stability.
* **Reviewer Interlock:** This review directly aligns with **Takeshi Yamamoto's** (Decider Atomicity Enforcer) focus on decider purity and the removal of local wall-clock reads, and **Ravi Krishnamurthy's** (Migration Coexistence Strategist) call for standardizing a global cross-process concurrency primitive instead of deferring it to individual slices.

---

## Required Changes Before Approval

1. **Eliminate Path-Alias Performance Hazard:** Completely remove `resolveLiveSessionByPathAlias` (and Title-based all-session scanning) from the Slice 1 API lookup scope, or mandate a proper database-level index on `Title` before delegation. Do not allow a process-forking all-session scan to be approved via a budget row alone.
2. **Define `repair-needed` Execution Lifecycle:** Detail the non-blocking execution model for the audited repair command when the read path encounters a `repair-needed` result. Specify how the repair is scheduled (e.g. background job, reconciler tick) and how concurrent writes are fenced while a repair is pending.
3. **Mandate Reconciler Fact Caching:** Add a concrete, reusable caching or bulk-reading mechanism to the Reconciler Fact Compilation design to prevent host process-fork exhaustion during ticks.
4. **Preserve Typed Wake/Blocker Observability:** Require the diagnostic result structure to carry the typed `WakeCause` and blocker enums rather than a flattened reason string.
5. **Budget Synchronous Event Emission:** Add a dedicated event-emission budget row to the table and specify non-blocking async buffers or a maximum flock serialization limit.
6. **Decouple Reconciler requirements from Session Slice 0:** Remove `SESSION-RECON-002`, `SESSION-RECON-003`, `SESSION-RECON-006`, and `SESSION-RECON-007` from the Session Slice 0 block list, keeping Slice 0 focused strictly on session-specific boundaries and inventories.

---

## Questions

1. If the API read-only classifier returns `repair-needed` for a session bead, will the API handler block the caller's request while it triggers an asynchronous background repair, or will it return a 404/not-found and depend on an external agent to resolve it?
2. To prevent reconciler hot loops from saturating host resources, can we introduce a bulk session status reader in `beads.Store` that returns all active session statuses in a single query rather than running per-session queries?
3. Why are reconciler-owned pool scaling and health-gate requirements (`SESSION-RECON-*`) placed as blocking gates for Session Slice 0, when they violate the Session/Reconciler separation of concerns?
4. How will operator-visible trace surfaces show that a critical reaction converged via a background "durable scan" rather than a synchronous event delivery?
