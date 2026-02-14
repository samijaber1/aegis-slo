package sqlite

// Schema defines the SQLite database schema
const Schema = `
-- SLO definitions table
CREATE TABLE IF NOT EXISTS slo_definitions (
	id TEXT PRIMARY KEY,
	service TEXT NOT NULL,
	environment TEXT NOT NULL,
	objective REAL NOT NULL,
	compliance_window TEXT NOT NULL,
	evaluation_interval TEXT NOT NULL,
	spec_json TEXT NOT NULL,
	created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
	updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_slo_service_env ON slo_definitions(service, environment);

-- Evaluations audit table
CREATE TABLE IF NOT EXISTS evaluations (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	slo_id TEXT NOT NULL,
	service TEXT NOT NULL,
	environment TEXT NOT NULL,
	decision TEXT NOT NULL,
	sli REAL NOT NULL,
	error_rate REAL NOT NULL,
	budget_remaining REAL NOT NULL,
	is_stale BOOLEAN NOT NULL DEFAULT 0,
	has_no_traffic BOOLEAN NOT NULL DEFAULT 0,
	reasons_json TEXT NOT NULL,
	burn_rates_json TEXT NOT NULL,
	timestamp TIMESTAMP NOT NULL,
	created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
	FOREIGN KEY (slo_id) REFERENCES slo_definitions(id)
);

CREATE INDEX IF NOT EXISTS idx_evaluations_slo_id ON evaluations(slo_id);
CREATE INDEX IF NOT EXISTS idx_evaluations_service_env ON evaluations(service, environment);
CREATE INDEX IF NOT EXISTS idx_evaluations_decision ON evaluations(decision);
CREATE INDEX IF NOT EXISTS idx_evaluations_timestamp ON evaluations(timestamp DESC);
CREATE INDEX IF NOT EXISTS idx_evaluations_created_at ON evaluations(created_at DESC);

-- Latest state table (one row per SLO)
CREATE TABLE IF NOT EXISTS latest_state (
	slo_id TEXT PRIMARY KEY,
	service TEXT NOT NULL,
	environment TEXT NOT NULL,
	decision TEXT NOT NULL,
	sli REAL NOT NULL,
	error_rate REAL NOT NULL,
	budget_remaining REAL NOT NULL,
	is_stale BOOLEAN NOT NULL DEFAULT 0,
	has_no_traffic BOOLEAN NOT NULL DEFAULT 0,
	reasons_json TEXT NOT NULL,
	burn_rates_json TEXT NOT NULL,
	timestamp TIMESTAMP NOT NULL,
	updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
	FOREIGN KEY (slo_id) REFERENCES slo_definitions(id)
);

CREATE INDEX IF NOT EXISTS idx_latest_state_service_env ON latest_state(service, environment);
`
