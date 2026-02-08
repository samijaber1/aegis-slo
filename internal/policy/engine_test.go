package policy

import (
	"testing"

	"github.com/samijaber1/aegis-slo/internal/eval"
	"github.com/samijaber1/aegis-slo/internal/slo"
)

func TestEngine_Evaluate(t *testing.T) {
	engine := NewEngine()

	tests := []struct {
		name             string
		evalResult       *eval.EvaluationResult
		sloSpec          *slo.SLO
		expectedDecision Decision
	}{
		{
			name: "healthy - no rules triggered",
			evalResult: &eval.EvaluationResult{
				SLOID: "test",
				BurnRates: map[string]eval.BurnRateResult{
					"5m": {BurnRate: 1.0},
					"1h": {BurnRate: 1.0},
				},
			},
			sloSpec:          createTestSLO(),
			expectedDecision: DecisionALLOW,
		},
		{
			name: "fast burn - rule triggered",
			evalResult: &eval.EvaluationResult{
				SLOID: "test",
				BurnRates: map[string]eval.BurnRateResult{
					"5m": {BurnRate: 15.0},
					"1h": {BurnRate: 15.0},
				},
			},
			sloSpec:          createTestSLO(),
			expectedDecision: DecisionBLOCK,
		},
		{
			name: "only short window high - rule not triggered",
			evalResult: &eval.EvaluationResult{
				SLOID: "test",
				BurnRates: map[string]eval.BurnRateResult{
					"5m": {BurnRate: 15.0},
					"1h": {BurnRate: 1.0},
				},
			},
			sloSpec:          createTestSLO(),
			expectedDecision: DecisionALLOW,
		},
		{
			name: "stale data - warn",
			evalResult: &eval.EvaluationResult{
				SLOID:   "test",
				IsStale: true,
				BurnRates: map[string]eval.BurnRateResult{
					"5m": {BurnRate: 1.0},
					"1h": {BurnRate: 1.0},
				},
			},
			sloSpec:          createTestSLO(),
			expectedDecision: DecisionWARN,
		},
		{
			name: "insufficient data - warn",
			evalResult: &eval.EvaluationResult{
				SLOID:            "test",
				InsufficientData: true,
				BurnRates: map[string]eval.BurnRateResult{
					"5m": {BurnRate: 1.0},
					"1h": {BurnRate: 1.0},
				},
			},
			sloSpec:          createTestSLO(),
			expectedDecision: DecisionWARN,
		},
		{
			name: "stale + fast burn - block takes precedence",
			evalResult: &eval.EvaluationResult{
				SLOID:   "test",
				IsStale: true,
				BurnRates: map[string]eval.BurnRateResult{
					"5m": {BurnRate: 15.0},
					"1h": {BurnRate: 15.0},
				},
			},
			sloSpec:          createTestSLO(),
			expectedDecision: DecisionBLOCK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := engine.Evaluate(tt.sloSpec, tt.evalResult)

			if result.Decision != tt.expectedDecision {
				t.Errorf("expected decision %s, got %s (reasons: %v)",
					tt.expectedDecision, result.Decision, result.Reasons)
			}

			t.Logf("Decision: %s, Reasons: %v", result.Decision, result.Reasons)
		})
	}
}

func createTestSLO() *slo.SLO {
	return &slo.SLO{
		Metadata: slo.Metadata{
			ID: "test-slo",
		},
		Spec: slo.Spec{
			Objective: 0.999,
			BurnPolicy: slo.BurnPolicy{
				Rules: []slo.BurnRule{
					{
						Name:        "fast-burn",
						ShortWindow: "5m",
						LongWindow:  "1h",
						Threshold:   14.0,
						Action:      "BLOCK",
					},
				},
			},
		},
	}
}
