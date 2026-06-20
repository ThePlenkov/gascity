package builtin

import (
	"testing"
)

func TestBuiltinProvidersAndOrder(t *testing.T) {
	providers := BuiltinProviders()
	order := BuiltinProviderOrder()

	if len(providers) != 18 {
		t.Fatalf("len(BuiltinProviders()) = %d, want 18", len(providers))
	}
	if len(order) != 18 {
		t.Fatalf("len(BuiltinProviderOrder()) = %d, want 18", len(order))
	}

	for _, name := range order {
		spec, ok := providers[name]
		if !ok {
			t.Fatalf("BuiltinProviders() missing %q", name)
		}
		if spec.Command == "" {
			t.Fatalf("provider %q has empty Command", name)
		}
		if spec.DisplayName == "" {
			t.Fatalf("provider %q has empty DisplayName", name)
		}
	}
}

func TestBuiltinProviderMimoCodeSpec(t *testing.T) {
	providers := BuiltinProviders()
	spec, ok := providers["mimocode"]
	if !ok {
		t.Fatal("BuiltinProviders() missing mimocode")
	}
	if spec.Command != "mimo" {
		t.Errorf("mimocode Command = %q, want %q", spec.Command, "mimo")
	}
	if spec.DisplayName != "MiMo Code" {
		t.Errorf("mimocode DisplayName = %q, want %q", spec.DisplayName, "MiMo Code")
	}
	if len(spec.Args) != 1 || spec.Args[0] != "--never-ask-questions" {
		t.Errorf("mimocode Args = %v, want [--never-ask-questions]", spec.Args)
	}
	if spec.PromptMode != "flag" || spec.PromptFlag != "--prompt" {
		t.Errorf("mimocode prompt = (%q, %q), want (flag, --prompt)", spec.PromptMode, spec.PromptFlag)
	}
	if !spec.SupportsACP || !spec.SupportsHooks {
		t.Errorf("mimocode SupportsACP=%v SupportsHooks=%v, want both true", spec.SupportsACP, spec.SupportsHooks)
	}
	if spec.ResumeFlag != "--session" || spec.ResumeStyle != "flag" {
		t.Errorf("mimocode resume = (%q, %q), want (--session, flag)", spec.ResumeFlag, spec.ResumeStyle)
	}
	if len(spec.ACPArgs) != 1 || spec.ACPArgs[0] != "acp" {
		t.Errorf("mimocode ACPArgs = %v, want [acp]", spec.ACPArgs)
	}
	if spec.InstructionsFile != "AGENTS.md" {
		t.Errorf("mimocode InstructionsFile = %q, want AGENTS.md", spec.InstructionsFile)
	}

	order := BuiltinProviderOrder()
	opencodeIdx, mimocodeIdx := -1, -1
	for i, name := range order {
		switch name {
		case "opencode":
			opencodeIdx = i
		case "mimocode":
			mimocodeIdx = i
		}
	}
	if mimocodeIdx == -1 {
		t.Fatal("BuiltinProviderOrder() missing mimocode")
	}
	// kilo now sits between opencode and mimocode; mimocode must be
	// somewhere after opencode but not necessarily immediately after.
	if mimocodeIdx <= opencodeIdx {
		t.Errorf("mimocode order index = %d, want strictly after opencode (%d)", mimocodeIdx, opencodeIdx)
	}
}

// TestBuiltinProviderKiloSpec verifies that the kilo CLI is registered
// as a builtin runtime provider with the opencode-derived profile shape.
func TestBuiltinProviderKiloSpec(t *testing.T) {
	providers := BuiltinProviders()
	spec, ok := providers["kilo"]
	if !ok {
		t.Fatal("BuiltinProviders() missing kilo")
	}
	if spec.Command != "kilo" {
		t.Errorf("kilo Command = %q, want %q", spec.Command, "kilo")
	}
	if spec.DisplayName != "Kilo Code" {
		t.Errorf("kilo DisplayName = %q, want %q", spec.DisplayName, "Kilo Code")
	}
	if spec.PromptMode != "flag" || spec.PromptFlag != "--prompt" {
		t.Errorf("kilo prompt = (%q, %q), want (flag, --prompt)", spec.PromptMode, spec.PromptFlag)
	}
	if !spec.SupportsACP || !spec.SupportsHooks {
		t.Errorf("kilo SupportsACP=%v SupportsHooks=%v, want both true", spec.SupportsACP, spec.SupportsHooks)
	}
	if spec.ResumeFlag != "--session" || spec.ResumeStyle != "flag" {
		t.Errorf("kilo resume = (%q, %q), want (--session, flag)", spec.ResumeFlag, spec.ResumeStyle)
	}
	if len(spec.ACPArgs) != 1 || spec.ACPArgs[0] != "acp" {
		t.Errorf("kilo ACPArgs = %v, want [acp]", spec.ACPArgs)
	}
	if spec.InstructionsFile != "AGENTS.md" {
		t.Errorf("kilo InstructionsFile = %q, want AGENTS.md", spec.InstructionsFile)
	}

	order := BuiltinProviderOrder()
	opencodeIdx, kiloIdx := -1, -1
	for i, name := range order {
		switch name {
		case "opencode":
			opencodeIdx = i
		case "kilo":
			kiloIdx = i
		}
	}
	if kiloIdx == -1 {
		t.Fatal("BuiltinProviderOrder() missing kilo")
	}
	if kiloIdx != opencodeIdx+1 {
		t.Errorf("kilo order index = %d, want immediately after opencode (%d)", kiloIdx, opencodeIdx)
	}
}

func TestBuiltinProvidersReturnClonedData(t *testing.T) {
	a := BuiltinProviders()
	b := BuiltinProviders()

	a["claude"] = BuiltinProviderSpec{Command: "mutated"}
	if b["claude"].Command == "mutated" {
		t.Fatal("BuiltinProviders() should return a cloned map")
	}

	claude := a["codex"]
	claude.ProcessNames[0] = "mutated"
	a["codex"] = claude
	if b["codex"].ProcessNames[0] == "mutated" {
		t.Fatal("BuiltinProviders() should clone nested slices")
	}
}
