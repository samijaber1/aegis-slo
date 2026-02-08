package eval

import "time"

// WindowMetrics represents metrics for a specific time window
type WindowMetrics struct {
	Window        string
	Good          float64
	Total         float64
	DataTimestamp *time.Time // Optional: for staleness checking
}

// SLIResult represents the computed SLI value
type SLIResult struct {
	Value            float64
	ErrorRate        float64
	InsufficientData bool
	Reason           string
}

// BurnRateResult represents burn rate computation for a window
type BurnRateResult struct {
	Window    string
	BurnRate  float64
	SLI       float64
	ErrorRate float64
}

// EvaluationResult represents the complete evaluation of an SLO
type EvaluationResult struct {
	SLOID            string
	SLI              SLIResult
	BurnRates        map[string]BurnRateResult // keyed by window
	BudgetRemaining  float64
	InsufficientData bool
	IsStale          bool
	Timestamp        time.Time
}
