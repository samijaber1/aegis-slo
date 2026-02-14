package storage

import (
	"time"

	"github.com/samijaber1/aegis-slo/internal/eval"
	"github.com/samijaber1/aegis-slo/internal/policy"
	"github.com/samijaber1/aegis-slo/internal/slo"
)

// AuditStorage defines the interface for persisting evaluation results
type AuditStorage interface {
	// StoreSLODefinition persists an SLO definition
	StoreSLODefinition(slo *slo.SLO) error

	// StoreEvaluation persists an evaluation result
	StoreEvaluation(evalResult *eval.EvaluationResult, gateResult *policy.GateResult) error

	// UpdateLatestState updates the latest state for an SLO
	UpdateLatestState(sloID string, evalResult *eval.EvaluationResult, gateResult *policy.GateResult) error

	// QueryAudit retrieves audit records with optional filtering
	QueryAudit(filter AuditFilter) ([]AuditRecord, error)

	// GetLatestState retrieves the latest state for an SLO
	GetLatestState(sloID string) (*LatestState, error)

	// Close closes the storage connection
	Close() error
}

// AuditFilter defines filtering options for audit queries
type AuditFilter struct {
	SLOID       string
	Service     string
	Environment string
	Decision    string // ALLOW, BLOCK, WARN
	StartTime   *time.Time
	EndTime     *time.Time
	Limit       int
	Offset      int
}

// AuditRecord represents a single audit entry
type AuditRecord struct {
	ID              int64
	SLOID           string
	Service         string
	Environment     string
	Decision        string
	SLI             float64
	ErrorRate       float64
	BudgetRemaining float64
	IsStale         bool
	HasNoTraffic    bool
	Reasons         []string
	BurnRates       map[string]eval.BurnRateResult
	Timestamp       time.Time
	CreatedAt       time.Time
}

// LatestState represents the most recent evaluation state for an SLO
type LatestState struct {
	SLOID           string
	Service         string
	Environment     string
	Decision        string
	SLI             float64
	ErrorRate       float64
	BudgetRemaining float64
	IsStale         bool
	HasNoTraffic    bool
	Reasons         []string
	BurnRates       map[string]eval.BurnRateResult
	Timestamp       time.Time
	UpdatedAt       time.Time
}
