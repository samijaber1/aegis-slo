package sqlite

import (
	"os"
	"testing"
	"time"

	"github.com/samijaber1/aegis-slo/internal/eval"
	"github.com/samijaber1/aegis-slo/internal/policy"
	"github.com/samijaber1/aegis-slo/internal/slo"
	"github.com/samijaber1/aegis-slo/internal/storage"
)

func setupTestDB(t *testing.T) (*Store, func()) {
	t.Helper()

	// Create temp database file
	tmpfile, err := os.CreateTemp("", "test-*.db")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	tmpfile.Close()

	store, err := NewStore(tmpfile.Name())
	if err != nil {
		os.Remove(tmpfile.Name())
		t.Fatalf("failed to create store: %v", err)
	}

	cleanup := func() {
		store.Close()
		os.Remove(tmpfile.Name())
	}

	return store, cleanup
}

func TestStore_StoreSLODefinition(t *testing.T) {
	store, cleanup := setupTestDB(t)
	defer cleanup()

	sloSpec := &slo.SLO{
		Metadata: slo.Metadata{
			ID:      "test-slo",
			Service: "test-service",
		},
		Spec: slo.Spec{
			Environment:        "production",
			Objective:          0.995,
			ComplianceWindow:   "30d",
			EvaluationInterval: "5m",
		},
	}

	err := store.StoreSLODefinition(sloSpec)
	if err != nil {
		t.Fatalf("failed to store SLO definition: %v", err)
	}

	// Verify it was stored by trying to retrieve it
	// (This would require adding a GetSLODefinition method)
}

func TestStore_StoreEvaluation(t *testing.T) {
	store, cleanup := setupTestDB(t)
	defer cleanup()

	// First store an SLO definition
	sloSpec := &slo.SLO{
		Metadata: slo.Metadata{
			ID:      "test-slo",
			Service: "test-service",
		},
		Spec: slo.Spec{
			Environment:        "production",
			Objective:          0.995,
			ComplianceWindow:   "30d",
			EvaluationInterval: "5m",
		},
	}

	if err := store.StoreSLODefinition(sloSpec); err != nil {
		t.Fatalf("failed to store SLO definition: %v", err)
	}

	// Create evaluation result
	evalResult := &eval.EvaluationResult{
		SLOID: "test-slo",
		SLI: eval.SLIResult{
			Value:     0.999,
			ErrorRate: 0.001,
		},
		BudgetRemaining: 0.8,
		BurnRates: map[string]eval.BurnRateResult{
			"5m": {
				Window:    "5m",
				BurnRate:  1.0,
				SLI:       0.999,
				ErrorRate: 0.001,
			},
		},
		IsStale:   false,
		Timestamp: time.Now(),
	}

	gateResult := &policy.GateResult{
		Decision: policy.DecisionALLOW,
		Reasons:  []string{"all burn rate checks passed"},
	}

	err := store.StoreEvaluation(evalResult, gateResult)
	if err != nil {
		t.Fatalf("failed to store evaluation: %v", err)
	}
}

func TestStore_QueryAudit(t *testing.T) {
	store, cleanup := setupTestDB(t)
	defer cleanup()

	// Store SLO definition
	sloSpec := &slo.SLO{
		Metadata: slo.Metadata{
			ID:      "test-slo",
			Service: "test-service",
		},
		Spec: slo.Spec{
			Environment:        "production",
			Objective:          0.995,
			ComplianceWindow:   "30d",
			EvaluationInterval: "5m",
		},
	}
	store.StoreSLODefinition(sloSpec)

	// Store multiple evaluations
	for i := 0; i < 3; i++ {
		evalResult := &eval.EvaluationResult{
			SLOID: "test-slo",
			SLI: eval.SLIResult{
				Value:     0.999,
				ErrorRate: 0.001,
			},
			BudgetRemaining: 0.8,
			BurnRates:       map[string]eval.BurnRateResult{},
			Timestamp:       time.Now().Add(time.Duration(i) * time.Minute),
		}

		gateResult := &policy.GateResult{
			Decision: policy.DecisionALLOW,
			Reasons:  []string{"test reason"},
		}

		store.StoreEvaluation(evalResult, gateResult)
	}

	// Query all evaluations
	records, err := store.QueryAudit(storage.AuditFilter{
		Limit: 10,
	})
	if err != nil {
		t.Fatalf("failed to query audit: %v", err)
	}

	if len(records) != 3 {
		t.Errorf("expected 3 records, got %d", len(records))
	}

	// Query by SLO ID
	records, err = store.QueryAudit(storage.AuditFilter{
		SLOID: "test-slo",
		Limit: 10,
	})
	if err != nil {
		t.Fatalf("failed to query audit by SLO ID: %v", err)
	}

	if len(records) != 3 {
		t.Errorf("expected 3 records for test-slo, got %d", len(records))
	}

	// Query by service
	records, err = store.QueryAudit(storage.AuditFilter{
		Service: "test-service",
		Limit:   10,
	})
	if err != nil {
		t.Fatalf("failed to query audit by service: %v", err)
	}

	if len(records) != 3 {
		t.Errorf("expected 3 records for test-service, got %d", len(records))
	}
}

func TestStore_UpdateLatestState(t *testing.T) {
	store, cleanup := setupTestDB(t)
	defer cleanup()

	// Store SLO definition
	sloSpec := &slo.SLO{
		Metadata: slo.Metadata{
			ID:      "test-slo",
			Service: "test-service",
		},
		Spec: slo.Spec{
			Environment:        "production",
			Objective:          0.995,
			ComplianceWindow:   "30d",
			EvaluationInterval: "5m",
		},
	}
	store.StoreSLODefinition(sloSpec)

	// Update latest state
	evalResult := &eval.EvaluationResult{
		SLOID: "test-slo",
		SLI: eval.SLIResult{
			Value:     0.999,
			ErrorRate: 0.001,
		},
		BudgetRemaining: 0.8,
		BurnRates:       map[string]eval.BurnRateResult{},
		Timestamp:       time.Now(),
	}

	gateResult := &policy.GateResult{
		Decision: policy.DecisionALLOW,
		Reasons:  []string{"test reason"},
	}

	err := store.UpdateLatestState("test-slo", evalResult, gateResult)
	if err != nil {
		t.Fatalf("failed to update latest state: %v", err)
	}

	// Get latest state
	state, err := store.GetLatestState("test-slo")
	if err != nil {
		t.Fatalf("failed to get latest state: %v", err)
	}

	if state == nil {
		t.Fatal("expected state to be non-nil")
	}

	if state.SLOID != "test-slo" {
		t.Errorf("expected SLOID=test-slo, got %s", state.SLOID)
	}

	if state.Decision != string(policy.DecisionALLOW) {
		t.Errorf("expected decision=ALLOW, got %s", state.Decision)
	}

	if state.SLI != 0.999 {
		t.Errorf("expected SLI=0.999, got %f", state.SLI)
	}
}

func TestStore_GetLatestState_NotFound(t *testing.T) {
	store, cleanup := setupTestDB(t)
	defer cleanup()

	state, err := store.GetLatestState("nonexistent")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if state != nil {
		t.Error("expected nil state for nonexistent SLO")
	}
}
