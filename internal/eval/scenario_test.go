package eval_test

import (
	"testing"
	"time"

	"github.com/samijaber1/aegis-slo/internal/adapter/synthetic"
	"github.com/samijaber1/aegis-slo/internal/eval"
	"github.com/samijaber1/aegis-slo/internal/policy"
	"github.com/samijaber1/aegis-slo/internal/slo"
)

func TestScenarios(t *testing.T) {
	// Load the SLO spec
	sloSpec := loadCheckoutSLO(t)

	tests := []struct {
		name             string
		fixtureFile      string
		expectedDecision policy.Decision
		checkStale       bool
		checkNoTraffic   bool
	}{
		{
			name:             "healthy",
			fixtureFile:      "../../fixtures/metrics/healthy.json",
			expectedDecision: policy.DecisionALLOW,
		},
		{
			name:             "fast-burn",
			fixtureFile:      "../../fixtures/metrics/fast-burn.json",
			expectedDecision: policy.DecisionBLOCK,
		},
		{
			name:             "slow-burn",
			fixtureFile:      "../../fixtures/metrics/slow-burn.json",
			expectedDecision: policy.DecisionBLOCK,
		},
		{
			name:             "stale",
			fixtureFile:      "../../fixtures/metrics/stale.json",
			expectedDecision: policy.DecisionWARN,
			checkStale:       true,
		},
		{
			name:             "zero-traffic",
			fixtureFile:      "../../fixtures/metrics/zero-traffic.json",
			expectedDecision: policy.DecisionWARN,
			checkNoTraffic:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create synthetic adapter
			adapter := synthetic.NewAdapter()
			if err := adapter.LoadFixture("checkout", tt.fixtureFile); err != nil {
				t.Fatalf("failed to load fixture: %v", err)
			}

			// Create evaluator
			evaluator := eval.NewEvaluator(adapter)

			// Evaluate SLO
			now := time.Now()
			evalResult, err := evaluator.Evaluate(sloSpec, now)
			if err != nil {
				t.Fatalf("evaluation failed: %v", err)
			}

			// Create policy engine
			policyEngine := policy.NewEngine()

			// Evaluate policy
			gateResult := policyEngine.Evaluate(sloSpec, evalResult)

			// Check decision
			if gateResult.Decision != tt.expectedDecision {
				t.Errorf("expected decision %s, got %s (reasons: %v)",
					tt.expectedDecision, gateResult.Decision, gateResult.Reasons)
			}

			// Additional checks
			if tt.checkStale && !gateResult.IsStale {
				t.Error("expected IsStale to be true")
			}

			if tt.checkNoTraffic && !gateResult.HasNoTraffic {
				t.Error("expected HasNoTraffic to be true")
			}

			t.Logf("Decision: %s, Reasons: %v", gateResult.Decision, gateResult.Reasons)
			for _, rr := range gateResult.RuleResults {
				if rr.Triggered {
					t.Logf("  Rule %s triggered: %s", rr.RuleName, rr.Reason)
				}
			}
		})
	}
}

func loadCheckoutSLO(t *testing.T) *slo.SLO {
	t.Helper()

	// Load the checkout-availability SLO
	sloFiles, errors := slo.LoadFromDirectory("../../fixtures/slo/valid")
	if len(errors) > 0 {
		t.Fatalf("failed to load SLO: %v", errors)
	}

	if len(sloFiles) == 0 {
		t.Fatal("no SLOs loaded")
	}

	// Find checkout-availability
	for _, sf := range sloFiles {
		if sf.SLO.Metadata.ID == "checkout-availability" {
			// Modify queries to use fixture names for synthetic adapter
			sf.SLO.Spec.SLI.Good.PrometheusQuery = "checkout"
			sf.SLO.Spec.SLI.Total.PrometheusQuery = "checkout"
			return sf.SLO
		}
	}

	t.Fatal("checkout-availability SLO not found")
	return nil
}
