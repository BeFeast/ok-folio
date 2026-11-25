package gallery

import (
	"strings"
	"testing"
)

func TestMVPDecisionValidatesIssueCriteria(t *testing.T) {
	decision := MVPDecision()

	if err := ValidateDecision(decision); err != nil {
		t.Fatalf("expected MVP decision to validate: %v", err)
	}

	if decision.ChosenDirection != CustomMVPDirection {
		t.Fatalf("expected custom MVP direction, got %q", decision.ChosenDirection)
	}
	if len(decision.Options) != 3 {
		t.Fatalf("expected three compared options, got %d", len(decision.Options))
	}

	names := make(map[string]bool)
	for _, option := range decision.Options {
		names[strings.ToLower(option.Name)] = true
	}
	for _, name := range []string{"customize photoprism", "wrap photoprism", "custom ok folio gallery"} {
		if !names[name] {
			t.Fatalf("expected option %q to be compared", name)
		}
	}
}

func TestMVPDecisionDocumentsStorageAndRuntimeGuardrails(t *testing.T) {
	decision := MVPDecision()
	text := strings.ToLower(joinDecisionText(decision))

	for _, required := range []string{
		"read-only with respect to photoprism",
		"deploy and rollback runbooks",
		"/api/v1/gallery/decision",
		"./scripts/product-verifier.sh",
	} {
		if !strings.Contains(text, required) {
			t.Fatalf("expected decision text to include %q", required)
		}
	}
}

func TestValidateDecisionRejectsIncompleteDecision(t *testing.T) {
	decision := MVPDecision()
	decision.Storage = nil

	if err := ValidateDecision(decision); err == nil {
		t.Fatal("expected incomplete decision to fail validation")
	}
}
