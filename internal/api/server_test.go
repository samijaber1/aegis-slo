package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/samijaber1/aegis-slo/internal/adapter/synthetic"
	"github.com/samijaber1/aegis-slo/internal/eval"
	"github.com/samijaber1/aegis-slo/internal/policy"
	"github.com/samijaber1/aegis-slo/internal/scheduler"
	"github.com/samijaber1/aegis-slo/internal/slo"
)

func setupTestServer(t *testing.T) (*Server, *scheduler.Scheduler) {
	t.Helper()

	adapter := synthetic.NewAdapter()
	evaluator := eval.NewEvaluator(adapter)
	policyEngine := policy.NewEngine()
	sched := scheduler.NewScheduler(evaluator, policyEngine, "../../fixtures/slo/valid")

	// Manually populate cache for testing
	cache := sched.GetCache()
	cache.Set("test-slo", &scheduler.EvaluationState{
		EvalResult: &eval.EvaluationResult{
			SLOID: "test-slo",
			SLI: eval.SLIResult{
				Value:     0.999,
				ErrorRate: 0.001,
			},
			BudgetRemaining: 0.5,
			BurnRates: map[string]eval.BurnRateResult{
				"5m": {BurnRate: 1.0},
				"1h": {BurnRate: 1.0},
			},
			Timestamp: time.Now(),
		},
		GateResult: &policy.GateResult{
			Decision: policy.DecisionALLOW,
			Reasons:  []string{"all burn rate checks passed"},
		},
		UpdatedAt: time.Now(),
		TTL:       30 * time.Second,
	})

	server := NewServer(sched, ":0")
	return server, sched
}

func TestHealthEndpoint(t *testing.T) {
	server, _ := setupTestServer(t)

	req := httptest.NewRequest("GET", "/healthz", nil)
	w := httptest.NewRecorder()

	server.handleHealth(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	var resp HealthResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp.Status != "ok" {
		t.Errorf("expected status=ok, got %s", resp.Status)
	}
}

func TestReadyEndpoint(t *testing.T) {
	tests := []struct {
		name           string
		loadSLOs       bool
		expectedStatus int
		expectedReady  bool
	}{
		{
			name:           "ready with SLOs",
			loadSLOs:       true,
			expectedStatus: http.StatusOK,
			expectedReady:  true,
		},
		{
			name:           "not ready without SLOs",
			loadSLOs:       false,
			expectedStatus: http.StatusServiceUnavailable,
			expectedReady:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create fresh server for each test case
			server, sched := setupTestServer(t)

			if tt.loadSLOs {
				// Populate test SLOs directly
				testSLO := &slo.SLO{
					Metadata: slo.Metadata{
						ID:      "test-slo",
						Service: "test-service",
					},
					Spec: slo.Spec{
						Objective: 0.995,
					},
				}
				sched.SetSLOsForTest([]slo.SLOWithFile{
					{SLO: testSLO, File: "test.yaml"},
				})
			}

			req := httptest.NewRequest("GET", "/readyz", nil)
			w := httptest.NewRecorder()

			server.handleReady(w, req)

			if w.Code != tt.expectedStatus {
				t.Errorf("expected status %d, got %d", tt.expectedStatus, w.Code)
			}

			var resp ReadyResponse
			if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
				t.Fatalf("failed to decode response: %v", err)
			}

			if resp.Ready != tt.expectedReady {
				t.Errorf("expected ready=%v, got %v", tt.expectedReady, resp.Ready)
			}
		})
	}
}

func TestGateDecisionEndpoint(t *testing.T) {
	server, _ := setupTestServer(t)

	tests := []struct {
		name           string
		request        DecisionRequest
		expectedStatus int
		expectedDec    string
	}{
		{
			name: "valid decision request",
			request: DecisionRequest{
				SLOID: "test-slo",
			},
			expectedStatus: http.StatusOK,
			expectedDec:    "ALLOW",
		},
		{
			name: "missing SLO ID",
			request: DecisionRequest{
				SLOID: "",
			},
			expectedStatus: http.StatusBadRequest,
		},
		{
			name: "SLO not found",
			request: DecisionRequest{
				SLOID: "nonexistent",
			},
			expectedStatus: http.StatusNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body, _ := json.Marshal(tt.request)
			req := httptest.NewRequest("POST", "/v1/gate/decision", bytes.NewReader(body))
			w := httptest.NewRecorder()

			server.handleGateDecision(w, req)

			if w.Code != tt.expectedStatus {
				t.Errorf("expected status %d, got %d", tt.expectedStatus, w.Code)
			}

			if tt.expectedStatus == http.StatusOK {
				var resp DecisionResponse
				if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
					t.Fatalf("failed to decode response: %v", err)
				}

				if resp.Decision != tt.expectedDec {
					t.Errorf("expected decision=%s, got %s", tt.expectedDec, resp.Decision)
				}

				if resp.SLOID != tt.request.SLOID {
					t.Errorf("expected SLOID=%s, got %s", tt.request.SLOID, resp.SLOID)
				}

				if resp.SLI.Value != 0.999 {
					t.Errorf("expected SLI value=0.999, got %f", resp.SLI.Value)
				}

				if len(resp.Reasons) == 0 {
					t.Error("expected reasons to be present")
				}

				if len(resp.BurnRates) == 0 {
					t.Error("expected burn rates to be present")
				}
			}
		})
	}
}

func TestMethodNotAllowed(t *testing.T) {
	server, _ := setupTestServer(t)

	tests := []struct {
		path   string
		method string
	}{
		{"/healthz", "POST"},
		{"/readyz", "POST"},
		{"/v1/slo", "POST"},
		{"/v1/gate/decision", "GET"},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, tt.path, nil)
			w := httptest.NewRecorder()

			mux := http.NewServeMux()
			mux.HandleFunc("/healthz", server.handleHealth)
			mux.HandleFunc("/readyz", server.handleReady)
			mux.HandleFunc("/v1/slo", server.handleSLOList)
			mux.HandleFunc("/v1/gate/decision", server.handleGateDecision)

			mux.ServeHTTP(w, req)

			if w.Code != http.StatusMethodNotAllowed {
				t.Errorf("expected status 405, got %d", w.Code)
			}
		})
	}
}
