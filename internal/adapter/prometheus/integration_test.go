package prometheus_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/samijaber1/aegis-slo/internal/adapter/prometheus"
	"github.com/samijaber1/aegis-slo/internal/eval"
	"github.com/samijaber1/aegis-slo/internal/policy"
	"github.com/samijaber1/aegis-slo/internal/slo"
)

func TestPrometheusAdapter_Integration(t *testing.T) {
	// Create a mock Prometheus server that returns metrics matching healthy scenario
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		query := r.URL.Query().Get("query")

		var value string
		if query == "sum(rate(good[5m]))" || query == "sum(rate(good[1h]))" || query == "sum(rate(good[30d]))" {
			value = "99950" // Good requests
		} else if query == "sum(rate(total[5m]))" || query == "sum(rate(total[1h]))" || query == "sum(rate(total[30d]))" {
			value = "100000" // Total requests
		} else {
			value = "0"
		}

		resp := prometheus.QueryResponse{
			Status: "success",
			Data: prometheus.QueryData{
				ResultType: "vector",
				Result: []prometheus.VectorResult{
					{
						Metric: map[string]string{},
						Value:  prometheus.SamplePair{float64(time.Now().Unix()), value},
					},
				},
			},
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	// Create SLO spec (similar to checkout-availability)
	sloSpec := &slo.SLO{
		Metadata: slo.Metadata{
			ID:      "test-slo",
			Service: "test",
		},
		Spec: slo.Spec{
			Objective:        0.999,
			ComplianceWindow: "30d",
			SLI: slo.SLI{
				Type: "ratio",
				Good: slo.QueryRef{
					PrometheusQuery: "sum(rate(good[{{window}}]))",
				},
				Total: slo.QueryRef{
					PrometheusQuery: "sum(rate(total[{{window}}]))",
				},
			},
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
			Gating: slo.Gating{
				MinDataPoints:  1,
				StalenessLimit: "120s",
			},
		},
	}

	// Create Prometheus adapter
	config := prometheus.DefaultConfig(server.URL)
	adapter := prometheus.NewAdapter(config)

	// Create evaluator
	evaluator := eval.NewEvaluator(adapter)

	// Evaluate SLO
	now := time.Now()
	evalResult, err := evaluator.Evaluate(sloSpec, now)
	if err != nil {
		t.Fatalf("evaluation failed: %v", err)
	}

	// Verify evaluation results
	if evalResult.SLI.Value < 0.999 {
		t.Errorf("expected SLI >= 0.999, got %f", evalResult.SLI.Value)
	}

	if evalResult.InsufficientData {
		t.Error("expected sufficient data")
	}

	if evalResult.IsStale {
		t.Error("expected fresh data")
	}

	// Create policy engine and evaluate
	policyEngine := policy.NewEngine()
	gateResult := policyEngine.Evaluate(sloSpec, evalResult)

	// Should be ALLOW for healthy metrics
	if gateResult.Decision != policy.DecisionALLOW {
		t.Errorf("expected ALLOW decision, got %s (reasons: %v)",
			gateResult.Decision, gateResult.Reasons)
	}

	t.Logf("✓ Integration test passed: Decision=%s, SLI=%.4f, Reasons=%v",
		gateResult.Decision, evalResult.SLI.Value, gateResult.Reasons)
}

func TestPrometheusAdapter_QueryFailure_ReturnsWarn(t *testing.T) {
	// Create a server that always fails
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
		w.Write([]byte("Prometheus unavailable"))
	}))
	defer server.Close()

	sloSpec := &slo.SLO{
		Metadata: slo.Metadata{ID: "test-slo"},
		Spec: slo.Spec{
			Objective:        0.999,
			ComplianceWindow: "30d",
			SLI: slo.SLI{
				Good:  slo.QueryRef{PrometheusQuery: "good"},
				Total: slo.QueryRef{PrometheusQuery: "total"},
			},
			BurnPolicy: slo.BurnPolicy{
				Rules: []slo.BurnRule{
					{
						Name:        "test",
						ShortWindow: "5m",
						LongWindow:  "1h",
						Threshold:   14.0,
						Action:      "BLOCK",
					},
				},
			},
			Gating: slo.Gating{StalenessLimit: "120s"},
		},
	}

	config := prometheus.DefaultConfig(server.URL)
	config.RetryCount = 0 // No retries for faster test
	adapter := prometheus.NewAdapter(config)

	evaluator := eval.NewEvaluator(adapter)

	// Evaluation should fail when Prometheus is unavailable
	_, err := evaluator.Evaluate(sloSpec, time.Now())
	if err == nil {
		t.Error("expected error when Prometheus is unavailable, got nil")
	}

	// In a real system, this would trigger a WARN decision
	// For now, we verify that query failures are properly propagated
	t.Logf("✓ Query failure properly detected: %v", err)
}
