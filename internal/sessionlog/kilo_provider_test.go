package sessionlog

import "testing"

// TestProviderFamilyKiloMapsToOpencode verifies that the kilo CLI provider
// family maps to the opencode family. kilo is a fork of opencode and shares
// the same transcript export/mirror surface, so the opencode reader, finder,
// and escape policy apply to kilo sessions without duplicating them.
func TestProviderFamilyKiloMapsToOpencode(t *testing.T) {
	tests := []struct {
		provider string
		want     string
	}{
		{provider: "kilo", want: "opencode"},
		{provider: "kilo/tmux-cli", want: "opencode"},
		{provider: "kilo-cli", want: "opencode"},
		{provider: "my-kilo", want: "opencode"},
		{provider: "KILO", want: "opencode"},
	}
	for _, tt := range tests {
		if got := ProviderFamily(tt.provider); got != tt.want {
			t.Errorf("ProviderFamily(%q) = %q, want %q", tt.provider, got, tt.want)
		}
	}
}