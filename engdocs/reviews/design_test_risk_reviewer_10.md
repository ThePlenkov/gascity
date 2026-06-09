# Architectural Risk and Testing Gap Assessment

**Review ID:** design-test-risk-reviewer-10  
**Date:** 2026-06-09  
**Target Areas:** `internal/session/` and `internal/runtime/`  
**Status:** Completed  

---

## Executive Summary

This document provides an evidence-based assessment of the key architectural risks, concurrency issues, and test coverage gaps identified within Gas City's session management (`internal/session/`) and runtime providers (`internal/runtime/`).

### Key Findings
1. **PID & Lock File Usage Violation (High Risk)**: `internal/session/submit.go` implements lock and PID files to track nudge pollers. This is a direct violation of Gas City's core design principle: *"No status files — query live state."*
2. **Global Mutex and State Shared Across Provider Instances (Medium-High Risk)**: `internal/runtime/t3bridge` uses a global mutex (`authMu`) and global package variables to cache WebSocket tokens, risking races and cross-tenant credential/URL pollution.
3. **Background Goroutine Context Leak (Medium Risk)**: Background event watchers in `internal/runtime/t3bridge` are spawned using a context derived from `context.Background()` instead of propagating a system-wide or provider-lifecycle context.
4. **Low Test Coverage in Core Providers (Medium Risk)**:
   - `internal/runtime/t3bridge/provider.go` has **46.4%** statement coverage, with key session operations (`Stop`, `Interrupt`, `Nudge`, `Attach`) having **0%** coverage.
   - `internal/runtime/tmux/` has **28.8%** statement coverage, leaving the primary local development provider significantly untested.

---

## Detailed Risk Assessments

### 1. PID & Lock File Usage in Nudge Poller
* **File Path**: [internal/session/submit.go](../../internal/session/submit.go#L602-L650)
* **Risk Class**: Violation of architectural invariants, stale state risk.
* **Line Range**: Lines 602–650

#### Code Evidence
```go
func sessionSubmitPollerPIDPath(cityPath, sessionName string) string {
	return citylayout.RuntimePath(cityPath, "nudges", "pollers", sessionName+".pid")
}
...
func withSessionSubmitPollerPIDLock(pidPath string, fn func() error) error {
	lockPath := pidPath + ".lock"
	if err := os.MkdirAll(filepath.Dir(pidPath), 0o755); err != nil {
		return fmt.Errorf("creating nudge poller dir: %w", err)
	}
	lockFile, err := os.OpenFile(lockPath, os.O_CREATE|os.O_RDWR, 0o600)
...
```

#### Detailed Description
The nudge poller tracks running processes by writing `.pid` and `.pid.lock` files to the `.gc` state directory. Gas City's design invariants explicitly prohibit the use of lock or PID status files because they easily go stale on unexpected orchestrator/system crashes, producing false negatives or false positives. The process table or live process checks (e.g. process signal 0 or direct OS table queries) must always serve as the single source of truth.

#### Recommendations
1. Deprecate `withSessionSubmitPollerPIDLock` and the generation of `.pid` / `.lock` files.
2. Utilize live process table scans or standard process signal-0 checks (`os.FindProcess` + `Process.Signal(syscall.Signal(0))`) to identify running pollers.
3. Track running poller subprocesses internally using a manager registry if managed within the same execution lifespan.

---

### 2. Global Mutex & Shared Mutable State in WebSocket Token Cache
* **File Path**: [internal/runtime/t3bridge/provider.go](../../internal/runtime/t3bridge/provider.go#L30-L35)
* **Risk Class**: Concurrency, multitenancy / test isolation hazards.
* **Line Range**: Lines 30–35

#### Code Evidence
```go
var (
	authMu                       sync.Mutex
	cachedBridgeWSToken          string
	cachedBridgeWSTokenBaseURL   string
	cachedBridgeWSTokenExpiresAt time.Time
)
```

#### Detailed Description
The WebSocket token cache in the T3 bridge provider is stored in package-global variables, protected by a global mutex `authMu`. When multiple `Provider` instances are instantiated concurrently (such as in parallel integration tests, multi-city orchestrations, or multi-tenant agent pools) utilizing different endpoints or credentials, they overwrite each other's cached token and base URL. This can cause websocket connections to route to incorrect base URLs or use invalid tokens.

#### Recommendations
1. Move `cachedBridgeWSToken`, `cachedBridgeWSTokenBaseURL`, and `cachedBridgeWSTokenExpiresAt` to fields on the `Provider` struct.
2. Replace `authMu` with an instance-level mutex or encapsulate token management in a dedicated, mockable `TokenManager` type.

---

### 3. Background Goroutine Context Leak in Event Watcher
* **File Path**: [internal/runtime/t3bridge/provider.go](../../internal/runtime/t3bridge/provider.go#L1837-L1847)
* **Risk Class**: Resource leak (leaking goroutines).
* **Line Range**: Lines 1837–1847

#### Code Evidence
```go
func (p *Provider) ensureEventWatcher(name string, cfg runtime.Config, binding threadBinding, envelope StartupEnvelope, providerName string) {
	p.mu.Lock()
	if cancel, ok := p.watchers[name]; ok {
		cancel()
	}
	ctx, cancel := context.WithCancel(context.Background())
	p.watchers[name] = cancel
	p.mu.Unlock()

	go p.runEventWatcher(ctx, name, cfg, binding, envelope, providerName)
}
```

#### Detailed Description
The event watcher is initialized using `context.WithCancel(context.Background())`. Because this context has no parent and is not tied to the lifecycle of the parent provider, controller, or orchestrator, any watcher goroutines that are not explicitly cancelled via `stopEventWatcher` will leak and run forever (or block on file reads/channel receives) after the parent provider or controller is shut down or reloaded.

#### Recommendations
1. Pass a root or lifecycle context (e.g. `p.ctx` or `controller.Context()`) to `ensureEventWatcher` rather than starting from `context.Background()`.
2. Ensure that any teardown or stop methods in the parent lifecycle gracefully cancel and wait for the completion of all active watcher goroutines.

---

### 4. Significant Test Coverage Gaps in Core Provider Implementations
* **File Paths**:
  - `internal/runtime/t3bridge/provider.go`
  - `internal/runtime/tmux/`
* **Risk Class**: Maintainability, regression risk, untargeted behavior.

#### Description
An analysis of test coverage reveals massive gaps in core provider code paths:
1. **T3 Bridge**: Statement coverage stands at **46.4%**. Crucial runtime interaction and teardown methods are completely untested (0.0% coverage):
   - `Stop` (line 2204) — 0.0%
   - `Interrupt` (line 2246) — 0.0%
   - `Attach` (line 2273) — 0.0%
   - `Nudge` (line 2302) — 0.0%
   - `GetLastActivity` (line 2469) — 0.0%
   - `ClearScrollback` (line 2495) — 0.0%
   - `SendKeys` (line 2559) — 0.0%
   - `RunLive` (line 2564) — 0.0%
   - `Capabilities` (line 2569) — 0.0%
   - `SleepCapability` (line 2576) — 0.0%
2. **Tmux Provider**: Statement coverage is only **28.8%**. Since tmux is the primary local execution environment for local development, leaving almost 70% of tmux provider execution logic untested represents a severe risk of silent regressions.

#### Recommendations
1. **Mock WebSocket Testing**: Create a robust mock WebSocket server in `provider_test.go` to simulate T3 server responses and verify all untested `t3bridge.Provider` methods under happy paths and failure/timeout scenarios.
2. **Refactor Tmux Provider for Mockability**: Deconstruct high-complexity functions in `internal/runtime/tmux/tmux.go` into testable unit helpers, and mock out the direct `tmux` CLI calls to boost tmux statement coverage past 75%.
