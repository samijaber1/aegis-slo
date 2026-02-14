package sqlite

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"

	_ "github.com/mattn/go-sqlite3"
	"github.com/samijaber1/aegis-slo/internal/eval"
	"github.com/samijaber1/aegis-slo/internal/policy"
	"github.com/samijaber1/aegis-slo/internal/slo"
	"github.com/samijaber1/aegis-slo/internal/storage"
)

// Store implements AuditStorage using SQLite
type Store struct {
	db *sql.DB
}

// NewStore creates a new SQLite storage with the given database path
func NewStore(dbPath string) (*Store, error) {
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Enable foreign keys
	if _, err := db.Exec("PRAGMA foreign_keys = ON"); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to enable foreign keys: %w", err)
	}

	// Run migrations
	if _, err := db.Exec(Schema); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to create schema: %w", err)
	}

	return &Store{db: db}, nil
}

// StoreSLODefinition persists an SLO definition
func (s *Store) StoreSLODefinition(sloSpec *slo.SLO) error {
	specJSON, err := json.Marshal(sloSpec.Spec)
	if err != nil {
		return fmt.Errorf("failed to marshal spec: %w", err)
	}

	query := `
		INSERT INTO slo_definitions (id, service, environment, objective, compliance_window, evaluation_interval, spec_json)
		VALUES (?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			service = excluded.service,
			environment = excluded.environment,
			objective = excluded.objective,
			compliance_window = excluded.compliance_window,
			evaluation_interval = excluded.evaluation_interval,
			spec_json = excluded.spec_json,
			updated_at = CURRENT_TIMESTAMP
	`

	_, err = s.db.Exec(query,
		sloSpec.Metadata.ID,
		sloSpec.Metadata.Service,
		sloSpec.Spec.Environment,
		sloSpec.Spec.Objective,
		sloSpec.Spec.ComplianceWindow,
		sloSpec.Spec.EvaluationInterval,
		string(specJSON),
	)
	if err != nil {
		return fmt.Errorf("failed to store SLO definition: %w", err)
	}

	return nil
}

// StoreEvaluation persists an evaluation result
func (s *Store) StoreEvaluation(evalResult *eval.EvaluationResult, gateResult *policy.GateResult) error {
	// Get SLO metadata from slo_definitions
	var service, environment string
	err := s.db.QueryRow("SELECT service, environment FROM slo_definitions WHERE id = ?", evalResult.SLOID).
		Scan(&service, &environment)
	if err != nil {
		return fmt.Errorf("failed to get SLO metadata: %w", err)
	}

	reasonsJSON, err := json.Marshal(gateResult.Reasons)
	if err != nil {
		return fmt.Errorf("failed to marshal reasons: %w", err)
	}

	burnRatesJSON, err := json.Marshal(evalResult.BurnRates)
	if err != nil {
		return fmt.Errorf("failed to marshal burn rates: %w", err)
	}

	hasNoTraffic := evalResult.InsufficientData

	query := `
		INSERT INTO evaluations (
			slo_id, service, environment, decision, sli, error_rate, budget_remaining,
			is_stale, has_no_traffic, reasons_json, burn_rates_json, timestamp
		)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`

	_, err = s.db.Exec(query,
		evalResult.SLOID,
		service,
		environment,
		string(gateResult.Decision),
		evalResult.SLI.Value,
		evalResult.SLI.ErrorRate,
		evalResult.BudgetRemaining,
		evalResult.IsStale,
		hasNoTraffic,
		string(reasonsJSON),
		string(burnRatesJSON),
		evalResult.Timestamp,
	)
	if err != nil {
		return fmt.Errorf("failed to store evaluation: %w", err)
	}

	return nil
}

// UpdateLatestState updates the latest state for an SLO
func (s *Store) UpdateLatestState(sloID string, evalResult *eval.EvaluationResult, gateResult *policy.GateResult) error {
	// Get SLO metadata
	var service, environment string
	err := s.db.QueryRow("SELECT service, environment FROM slo_definitions WHERE id = ?", sloID).
		Scan(&service, &environment)
	if err != nil {
		return fmt.Errorf("failed to get SLO metadata: %w", err)
	}

	reasonsJSON, err := json.Marshal(gateResult.Reasons)
	if err != nil {
		return fmt.Errorf("failed to marshal reasons: %w", err)
	}

	burnRatesJSON, err := json.Marshal(evalResult.BurnRates)
	if err != nil {
		return fmt.Errorf("failed to marshal burn rates: %w", err)
	}

	hasNoTraffic := evalResult.InsufficientData

	query := `
		INSERT INTO latest_state (
			slo_id, service, environment, decision, sli, error_rate, budget_remaining,
			is_stale, has_no_traffic, reasons_json, burn_rates_json, timestamp
		)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(slo_id) DO UPDATE SET
			service = excluded.service,
			environment = excluded.environment,
			decision = excluded.decision,
			sli = excluded.sli,
			error_rate = excluded.error_rate,
			budget_remaining = excluded.budget_remaining,
			is_stale = excluded.is_stale,
			has_no_traffic = excluded.has_no_traffic,
			reasons_json = excluded.reasons_json,
			burn_rates_json = excluded.burn_rates_json,
			timestamp = excluded.timestamp,
			updated_at = CURRENT_TIMESTAMP
	`

	_, err = s.db.Exec(query,
		sloID,
		service,
		environment,
		string(gateResult.Decision),
		evalResult.SLI.Value,
		evalResult.SLI.ErrorRate,
		evalResult.BudgetRemaining,
		evalResult.IsStale,
		hasNoTraffic,
		string(reasonsJSON),
		string(burnRatesJSON),
		evalResult.Timestamp,
	)
	if err != nil {
		return fmt.Errorf("failed to update latest state: %w", err)
	}

	return nil
}

// QueryAudit retrieves audit records with optional filtering
func (s *Store) QueryAudit(filter storage.AuditFilter) ([]storage.AuditRecord, error) {
	query := `
		SELECT id, slo_id, service, environment, decision, sli, error_rate, budget_remaining,
		       is_stale, has_no_traffic, reasons_json, burn_rates_json, timestamp, created_at
		FROM evaluations
		WHERE 1=1
	`
	args := []interface{}{}

	if filter.SLOID != "" {
		query += " AND slo_id = ?"
		args = append(args, filter.SLOID)
	}

	if filter.Service != "" {
		query += " AND service = ?"
		args = append(args, filter.Service)
	}

	if filter.Environment != "" {
		query += " AND environment = ?"
		args = append(args, filter.Environment)
	}

	if filter.Decision != "" {
		query += " AND decision = ?"
		args = append(args, filter.Decision)
	}

	if filter.StartTime != nil {
		query += " AND timestamp >= ?"
		args = append(args, filter.StartTime)
	}

	if filter.EndTime != nil {
		query += " AND timestamp <= ?"
		args = append(args, filter.EndTime)
	}

	query += " ORDER BY timestamp DESC"

	if filter.Limit > 0 {
		query += " LIMIT ?"
		args = append(args, filter.Limit)
	} else {
		query += " LIMIT 100" // Default limit
	}

	if filter.Offset > 0 {
		query += " OFFSET ?"
		args = append(args, filter.Offset)
	}

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query audit records: %w", err)
	}
	defer rows.Close()

	var records []storage.AuditRecord
	for rows.Next() {
		var record storage.AuditRecord
		var reasonsJSON, burnRatesJSON string

		err := rows.Scan(
			&record.ID,
			&record.SLOID,
			&record.Service,
			&record.Environment,
			&record.Decision,
			&record.SLI,
			&record.ErrorRate,
			&record.BudgetRemaining,
			&record.IsStale,
			&record.HasNoTraffic,
			&reasonsJSON,
			&burnRatesJSON,
			&record.Timestamp,
			&record.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan row: %w", err)
		}

		if err := json.Unmarshal([]byte(reasonsJSON), &record.Reasons); err != nil {
			return nil, fmt.Errorf("failed to unmarshal reasons: %w", err)
		}

		if err := json.Unmarshal([]byte(burnRatesJSON), &record.BurnRates); err != nil {
			return nil, fmt.Errorf("failed to unmarshal burn rates: %w", err)
		}

		records = append(records, record)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("row iteration error: %w", err)
	}

	return records, nil
}

// GetLatestState retrieves the latest state for an SLO
func (s *Store) GetLatestState(sloID string) (*storage.LatestState, error) {
	query := `
		SELECT slo_id, service, environment, decision, sli, error_rate, budget_remaining,
		       is_stale, has_no_traffic, reasons_json, burn_rates_json, timestamp, updated_at
		FROM latest_state
		WHERE slo_id = ?
	`

	var state storage.LatestState
	var reasonsJSON, burnRatesJSON string

	err := s.db.QueryRow(query, sloID).Scan(
		&state.SLOID,
		&state.Service,
		&state.Environment,
		&state.Decision,
		&state.SLI,
		&state.ErrorRate,
		&state.BudgetRemaining,
		&state.IsStale,
		&state.HasNoTraffic,
		&reasonsJSON,
		&burnRatesJSON,
		&state.Timestamp,
		&state.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get latest state: %w", err)
	}

	if err := json.Unmarshal([]byte(reasonsJSON), &state.Reasons); err != nil {
		return nil, fmt.Errorf("failed to unmarshal reasons: %w", err)
	}

	if err := json.Unmarshal([]byte(burnRatesJSON), &state.BurnRates); err != nil {
		return nil, fmt.Errorf("failed to unmarshal burn rates: %w", err)
	}

	return &state, nil
}

// Close closes the database connection
func (s *Store) Close() error {
	return s.db.Close()
}

// buildWhereClause is a helper to build WHERE clauses dynamically
func buildWhereClause(conditions []string, params []interface{}) (string, []interface{}) {
	if len(conditions) == 0 {
		return "", params
	}
	return " WHERE " + strings.Join(conditions, " AND "), params
}
