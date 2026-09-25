package agents

import (
	"context"
	"testing"
)

func TestClassify(t *testing.T) {
	cases := []struct {
		query string
		want  Intent
	}{
		{"hi", IntentGreeting},
		{"Hey there!", IntentGreeting},
		{"I want a white lace bralette", IntentDiscovery},         // "white" must not match "hi"
		{"Help me build an outfit for date night", IntentStyling}, // "outfit" must not match "fit"
		{"What size bra for 34D broad shoulders?", IntentSizing},
		{"Add it to my cart and checkout", IntentTransaction},
		{"Find me a wedding guest look", IntentStyling},
		{"recommend some pajamas", IntentDiscovery},
	}
	ia := NewIntentAgent(nil)
	for _, tc := range cases {
		got, err := ia.Classify(context.Background(), tc.query)
		if err != nil {
			t.Fatalf("Classify(%q): %v", tc.query, err)
		}
		if got != tc.want {
			t.Errorf("Classify(%q) = %s, want %s", tc.query, got, tc.want)
		}
	}
}
