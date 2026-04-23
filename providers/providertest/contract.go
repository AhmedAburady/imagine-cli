// Package providertest provides a reusable contract test suite every
// provider should pass. Each provider gets a one-line test file:
//
//	func TestContract(t *testing.T) { providertest.Contract(t, "myprovider") }
//
// The Contract battery checks invariants that are knowable without making
// any network calls: Info well-formedness, alias round-trips, BindFlags
// idempotency, ReadFlags default-state validity, and cross-consistency
// between declarations (Bundle.SupportedFlags, ModelInfo.SupportedFlags,
// and the flags actually registered by BindFlags).
//
// The harness exists because the Provider interface is the contract every
// provider shares — and the contract only holds if it's checked. When the
// framework's rules change (e.g. model-level enforcement lands), adding a
// test to this file makes every provider's suite enforce it at once.
package providertest

import (
	"testing"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"github.com/AhmedAburady/imagine-cli/providers"
)

// Contract runs the full provider-contract battery against the registered
// provider named `name`. Intended to be invoked from the provider's own
// package test file so import cycles don't arise.
func Contract(t *testing.T, name string) {
	t.Helper()

	bundle, ok := providers.Get(name)
	if !ok {
		t.Fatalf("provider %q is not registered — ensure its package is blank-imported", name)
	}

	t.Run("InfoWellFormed", func(t *testing.T) { checkInfoWellFormed(t, bundle) })
	t.Run("InfoNameMatchesRegistration", func(t *testing.T) { checkInfoNameMatches(t, name, bundle) })
	t.Run("DefaultModelResolvable", func(t *testing.T) { checkDefaultModelResolvable(t, bundle) })
	t.Run("EmptyInputResolvesToDefault", func(t *testing.T) { checkEmptyResolvesToDefault(t, bundle) })
	t.Run("AliasesRoundTrip", func(t *testing.T) { checkAliasesRoundTrip(t, bundle) })
	t.Run("CanonicalIDsRoundTrip", func(t *testing.T) { checkCanonicalIDsRoundTrip(t, bundle) })
	t.Run("NoDuplicateModelIDs", func(t *testing.T) { checkNoDuplicateModelIDs(t, bundle) })
	t.Run("ModelSupportedFlagsSubsetOfBundle", func(t *testing.T) { checkModelFlagsSubset(t, bundle) })
	t.Run("MaxBatchNValid", func(t *testing.T) { checkMaxBatchN(t, bundle) })
	t.Run("BindFlagsIdempotent", func(t *testing.T) { checkBindFlagsIdempotent(t, bundle) })
	t.Run("ReadFlagsDefaultsSucceed", func(t *testing.T) { checkReadFlagsDefaults(t, bundle) })
	t.Run("SupportedFlagsRegisteredByBindFlags", func(t *testing.T) { checkSupportedFlagsRegistered(t, bundle) })
}

// --- individual invariants --------------------------------------------------

func checkInfoWellFormed(t *testing.T, b providers.Bundle) {
	if b.Info.Name == "" {
		t.Error("Info.Name is empty")
	}
	if b.Info.DisplayName == "" {
		t.Error("Info.DisplayName is empty")
	}
	if b.Info.DefaultModel == "" {
		t.Error("Info.DefaultModel is empty")
	}
	if len(b.Info.Models) == 0 {
		t.Error("Info.Models is empty — provider must advertise at least one model")
	}
	for i, m := range b.Info.Models {
		if m.ID == "" {
			t.Errorf("Info.Models[%d].ID is empty", i)
		}
	}
}

func checkInfoNameMatches(t *testing.T, name string, b providers.Bundle) {
	if b.Info.Name != name {
		t.Errorf("Info.Name = %q, want %q (should match registration key)", b.Info.Name, name)
	}
}

func checkDefaultModelResolvable(t *testing.T, b providers.Bundle) {
	got, err := b.Info.ResolveModel(b.Info.DefaultModel)
	if err != nil {
		t.Fatalf("ResolveModel(DefaultModel=%q): %v", b.Info.DefaultModel, err)
	}
	if got != b.Info.DefaultModel {
		t.Errorf("ResolveModel(%q) = %q, want itself", b.Info.DefaultModel, got)
	}
}

func checkEmptyResolvesToDefault(t *testing.T, b providers.Bundle) {
	got, err := b.Info.ResolveModel("")
	if err != nil {
		t.Fatalf("ResolveModel(\"\"): %v", err)
	}
	if got != b.Info.DefaultModel {
		t.Errorf("ResolveModel(\"\") = %q, want DefaultModel %q", got, b.Info.DefaultModel)
	}
}

func checkAliasesRoundTrip(t *testing.T, b providers.Bundle) {
	for _, m := range b.Info.Models {
		for _, alias := range m.Aliases {
			got, err := b.Info.ResolveModel(alias)
			if err != nil {
				t.Errorf("ResolveModel(alias=%q): %v", alias, err)
				continue
			}
			if got != m.ID {
				t.Errorf("ResolveModel(%q) = %q, want canonical ID %q", alias, got, m.ID)
			}
		}
	}
}

func checkCanonicalIDsRoundTrip(t *testing.T, b providers.Bundle) {
	for _, m := range b.Info.Models {
		got, err := b.Info.ResolveModel(m.ID)
		if err != nil {
			t.Errorf("ResolveModel(ID=%q): %v", m.ID, err)
			continue
		}
		if got != m.ID {
			t.Errorf("ResolveModel(%q) = %q, want itself", m.ID, got)
		}
	}
}

func checkNoDuplicateModelIDs(t *testing.T, b providers.Bundle) {
	seen := make(map[string]bool, len(b.Info.Models))
	for _, m := range b.Info.Models {
		if seen[m.ID] {
			t.Errorf("duplicate model ID %q", m.ID)
		}
		seen[m.ID] = true
	}
}

func checkModelFlagsSubset(t *testing.T, b providers.Bundle) {
	allowed := make(map[string]bool, len(b.SupportedFlags))
	for _, f := range b.SupportedFlags {
		allowed[f] = true
	}
	for _, m := range b.Info.Models {
		for _, f := range m.SupportedFlags {
			if !allowed[f] {
				t.Errorf("model %q declares flag %q which isn't in Bundle.SupportedFlags %v",
					m.ID, f, b.SupportedFlags)
			}
		}
	}
}

func checkMaxBatchN(t *testing.T, b providers.Bundle) {
	if b.Info.Capabilities.MaxBatchN < 1 {
		t.Errorf("Capabilities.MaxBatchN = %d, must be >= 1 (orchestrator divides by it)",
			b.Info.Capabilities.MaxBatchN)
	}
}

func checkBindFlagsIdempotent(t *testing.T, b providers.Bundle) {
	if b.BindFlags == nil {
		return // provider has no private flags
	}
	cmd := &cobra.Command{Use: "test", Run: func(*cobra.Command, []string) {}}
	func() {
		defer func() {
			if r := recover(); r != nil {
				t.Fatalf("BindFlags panicked on first call: %v", r)
			}
		}()
		b.BindFlags(cmd)
	}()
	beforeCount := countFlags(cmd)
	func() {
		defer func() {
			if r := recover(); r != nil {
				t.Fatalf("BindFlags panicked on second call: %v", r)
			}
		}()
		b.BindFlags(cmd)
	}()
	afterCount := countFlags(cmd)
	if beforeCount != afterCount {
		t.Errorf("BindFlags not idempotent: %d flags after first call, %d after second", beforeCount, afterCount)
	}
}

func checkReadFlagsDefaults(t *testing.T, b providers.Bundle) {
	if b.ReadFlags == nil {
		return
	}
	cmd := &cobra.Command{Use: "test", Run: func(*cobra.Command, []string) {}}
	if b.BindFlags != nil {
		b.BindFlags(cmd)
	}
	cmd.SetArgs(nil)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("parsing defaults: %v", err)
	}
	out, err := b.ReadFlags(cmd)
	if err != nil {
		t.Errorf("ReadFlags with default state errored: %v", err)
	}
	if out == nil {
		t.Error("ReadFlags returned nil options for default state")
	}
}

func checkSupportedFlagsRegistered(t *testing.T, b providers.Bundle) {
	if b.BindFlags == nil {
		return
	}
	cmd := &cobra.Command{Use: "test", Run: func(*cobra.Command, []string) {}}
	b.BindFlags(cmd)
	for _, name := range b.SupportedFlags {
		if cmd.Flags().Lookup(name) == nil {
			t.Errorf("SupportedFlags lists %q but BindFlags didn't register it", name)
		}
	}
}

// --- helpers ----------------------------------------------------------------

func countFlags(cmd *cobra.Command) int {
	n := 0
	cmd.Flags().VisitAll(func(_ *pflag.Flag) { n++ })
	return n
}
