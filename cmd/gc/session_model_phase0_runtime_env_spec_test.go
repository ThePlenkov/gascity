package main

import (
	"io"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gastownhall/gascity/internal/beads"
	"github.com/gastownhall/gascity/internal/config"
	"github.com/gastownhall/gascity/internal/fsys"
	"github.com/gastownhall/gascity/internal/runtime"
)

// Phase 0 spec coverage from engdocs/design/session-model-unification.md:
// - Runtime Environment
// - GC_TEMPLATE/GC_AGENT/GC_SESSION_ORIGIN contracts

func TestPhase0RuntimeEnv_TemplateResolutionSetsOriginAndPublicHandle(t *testing.T) {
	params := &agentBuildParams{
		cityName:   "phase0-city",
		cityPath:   t.TempDir(),
		workspace:  &config.Workspace{Provider: "test-agent"},
		providers:  map[string]config.ProviderSpec{"test-agent": {DisplayName: "Test Agent", Command: "true"}},
		lookPath:   func(string) (string, error) { return filepath.Join("/usr/bin", "true"), nil },
		fs:         fsys.OSFS{},
		beaconTime: time.Unix(0, 0),
		beadNames:  make(map[string]string),
		stderr:     io.Discard,
	}
	agentCfg := &config.Agent{
		Name:     "worker",
		Provider: "test-agent",
		WorkDir:  filepath.Join(".gc", "agents", "phase0"),
	}

	tp, err := resolveTemplate(params, agentCfg, agentCfg.QualifiedName(), nil)
	if err != nil {
		t.Fatalf("resolveTemplate(worker): %v", err)
	}
	if got := tp.Env["GC_TEMPLATE"]; got != "worker" {
		t.Fatalf("GC_TEMPLATE = %q, want worker", got)
	}
	if got := tp.Env["GC_SESSION_ORIGIN"]; got == "" {
		t.Fatal("GC_SESSION_ORIGIN = empty, want explicit origin")
	}
	if got := tp.Env["GC_AGENT"]; got != tp.Env["GC_SESSION_NAME"] {
		t.Fatalf("GC_AGENT = %q, want public-handle compatibility value %q", got, tp.Env["GC_SESSION_NAME"])
	}
}

func TestPhase0RuntimeEnv_TemplateResolutionDoesNotPublishLifecycleBeadsWrapper(t *testing.T) {
	t.Setenv("GC_BEADS", "bd")

	params := &agentBuildParams{
		cityName:   "phase0-city",
		cityPath:   t.TempDir(),
		workspace:  &config.Workspace{Provider: "test-agent"},
		providers:  map[string]config.ProviderSpec{"test-agent": {DisplayName: "Test Agent", Command: "true"}},
		lookPath:   func(string) (string, error) { return filepath.Join("/usr/bin", "true"), nil },
		fs:         fsys.OSFS{},
		beaconTime: time.Unix(0, 0),
		beadNames:  make(map[string]string),
		stderr:     io.Discard,
	}
	agentCfg := &config.Agent{
		Name:     "mayor",
		Provider: "test-agent",
	}

	tp, err := resolveTemplate(params, agentCfg, agentCfg.QualifiedName(), nil)
	if err != nil {
		t.Fatalf("resolveTemplate(mayor): %v", err)
	}
	if got := tp.Env["GC_BEADS"]; strings.Contains(got, "gc-beads-bd") {
		t.Fatalf("GC_BEADS = %q, want data-path provider value, not lifecycle wrapper", got)
	}
}

// TestPhase0RuntimeEnv_EphemeralPoolAgentBEADSActorMatchesSessionName guards
// the pool/ephemeral invariant that BEADS_ACTOR equals GC_SESSION_NAME and
// GC_AGENT. See ga-dre: the pack prompt's tier-1 recovery query assumes
// `bd update --claim` without an explicit `--assignee` stores an actor that
// other queries can match; the pool path already satisfies this because
// template_resolve.go sets all three to the tmux-safe sessName together.
// This test locks that contract so a future refactor doesn't silently
// diverge the three keys.
func TestPhase0RuntimeEnv_EphemeralPoolAgentBEADSActorMatchesSessionName(t *testing.T) {
	params := &agentBuildParams{
		cityName:   "phase0-city",
		cityPath:   t.TempDir(),
		workspace:  &config.Workspace{Provider: "test-agent"},
		providers:  map[string]config.ProviderSpec{"test-agent": {DisplayName: "Test Agent", Command: "true"}},
		lookPath:   func(string) (string, error) { return filepath.Join("/usr/bin", "true"), nil },
		fs:         fsys.OSFS{},
		beaconTime: time.Unix(0, 0),
		beadNames:  make(map[string]string),
		stderr:     io.Discard,
	}
	agentCfg := &config.Agent{
		Name:     "worker",
		Provider: "test-agent",
		WorkDir:  filepath.Join(".gc", "agents", "phase0"),
	}

	tp, err := resolveTemplate(params, agentCfg, agentCfg.QualifiedName(), nil)
	if err != nil {
		t.Fatalf("resolveTemplate(worker): %v", err)
	}
	if got, want := tp.Env["BEADS_ACTOR"], tp.Env["GC_SESSION_NAME"]; got != want {
		t.Fatalf("BEADS_ACTOR = %q, want GC_SESSION_NAME %q", got, want)
	}
	if got, want := tp.Env["BEADS_ACTOR"], tp.Env["GC_AGENT"]; got != want {
		t.Fatalf("BEADS_ACTOR = %q, want GC_AGENT %q", got, want)
	}
}

// TestPhase0RuntimeEnv_NamedSingletonBEADSActorMatchesIdentity asserts the
// fix from ga-6fo: in a named-singleton session, BEADS_ACTOR must equal the
// raw identity (GC_AGENT / GC_TEMPLATE), not the sanitized sessName. Without
// this, `bd update --claim` (no explicit --assignee) stores the cryptic
// sanitized session name and breaks tier-1 recovery queries keyed on
// GC_TEMPLATE.
func TestPhase0RuntimeEnv_NamedSingletonBEADSActorMatchesIdentity(t *testing.T) {
	cityPath := t.TempDir()
	store := beads.NewMemStore()
	// Use a rig-qualified identity ("gascity/mayor") so the sanitized sessName
	// ("gascity--mayor") diverges from the raw identity. Without the divergence
	// the test passes trivially — sanitization only fires when the identity
	// contains a slash.
	cfg := &config.City{
		Workspace: config.Workspace{Name: "phase0-city"},
		Agents: []config.Agent{{
			Name:              "mayor",
			Dir:               "gascity",
			StartCommand:      "true",
			MaxActiveSessions: intPtr(1),
			WorkQuery:         "printf ''",
		}},
		NamedSessions: []config.NamedSession{{
			Template: "mayor",
			Dir:      "gascity",
			Mode:     "always",
		}},
	}

	dsResult := buildDesiredState("phase0-city", cityPath, time.Now().UTC(), cfg, runtime.NewFake(), store, io.Discard)
	var namedTP TemplateParams
	found := false
	for _, tp := range dsResult.State {
		if tp.Env["GC_SESSION_ORIGIN"] == "named" {
			namedTP = tp
			found = true
			break
		}
	}
	if !found {
		t.Fatal("buildDesiredState produced no named-session TemplateParams; cannot assert BEADS_ACTOR")
	}
	identity := namedTP.ConfiguredNamedIdentity
	if identity == "" {
		t.Fatalf("named TemplateParams missing ConfiguredNamedIdentity: %+v", namedTP)
	}
	if got := namedTP.Env["BEADS_ACTOR"]; got != identity {
		t.Fatalf("BEADS_ACTOR = %q, want identity %q", got, identity)
	}
	if got, want := namedTP.Env["BEADS_ACTOR"], namedTP.Env["GC_AGENT"]; got != want {
		t.Fatalf("BEADS_ACTOR = %q, want GC_AGENT %q", got, want)
	}
	if got, want := namedTP.Env["BEADS_ACTOR"], namedTP.Env["GC_TEMPLATE"]; got != want {
		t.Fatalf("BEADS_ACTOR = %q, want GC_TEMPLATE %q (singleton named)", got, want)
	}
}
