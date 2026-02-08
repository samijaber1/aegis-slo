package eval

import (
	"fmt"
	"time"

	"github.com/samijaber1/aegis-slo/internal/slo"
)

// MetricsAdapter defines the interface for fetching metrics.
// QueryWindow returns WindowMetrics for the given query+window. For synthetic fixtures,
// this should return deterministic values. For later Prometheus adapter, this will
// execute a query with {{window}} substituted.
type MetricsAdapter interface {
	QueryWindow(query string, window string) (WindowMetrics, error)
}

// Evaluator handles SLO evaluation.
type Evaluator struct {
	adapter MetricsAdapter
}

// NewEvaluator creates a new evaluator with the given metrics adapter.
func NewEvaluator(adapter MetricsAdapter) *Evaluator {
	return &Evaluator{adapter: adapter}
}

// Evaluate performs a complete SLO evaluation for a single SLO spec.
func (e *Evaluator) Evaluate(sloSpec *slo.SLO, now time.Time) (*EvaluationResult, error) {
	if sloSpec == nil {
		return nil, fmt.Errorf("nil sloSpec")
	}

	result := &EvaluationResult{
		SLOID:     sloSpec.Metadata.ID,
		BurnRates: make(map[string]BurnRateResult),
		Timestamp: now,
	}

	// Collect all unique windows required (compliance + burn policy windows)
	windows := e.collectWindows(sloSpec)

	// Parse staleness limit once
	var stalenessLimit time.Duration
	var haveStalenessLimit bool
	if sloSpec.Spec.Gating.StalenessLimit != "" {
		d, err := slo.ParseDuration(sloSpec.Spec.Gating.StalenessLimit)
		if err == nil {
			stalenessLimit = d
			haveStalenessLimit = true
		}
	}

	// Query metrics for each window
	windowMetrics := make(map[string]WindowMetrics, len(windows))
	for _, window := range windows {
		// Query good events
		goodMetrics, err := e.adapter.QueryWindow(sloSpec.Spec.SLI.Good.PrometheusQuery, window)
		if err != nil {
			return nil, fmt.Errorf("query good metrics (window=%s): %w", window, err)
		}

		// Query total events
		totalMetrics, err := e.adapter.QueryWindow(sloSpec.Spec.SLI.Total.PrometheusQuery, window)
		if err != nil {
			return nil, fmt.Errorf("query total metrics (window=%s): %w", window, err)
		}

		// Choose the best timestamp available for staleness checks:
		// Prefer the newest timestamp among good/total to avoid marking stale due to one missing/older ts.
		var chosenTS *time.Time
		if goodMetrics.DataTimestamp != nil && totalMetrics.DataTimestamp != nil {
			if goodMetrics.DataTimestamp.After(*totalMetrics.DataTimestamp) {
				chosenTS = goodMetrics.DataTimestamp
			} else {
				chosenTS = totalMetrics.DataTimestamp
			}
		} else if goodMetrics.DataTimestamp != nil {
			chosenTS = goodMetrics.DataTimestamp
		} else if totalMetrics.DataTimestamp != nil {
			chosenTS = totalMetrics.DataTimestamp
		}

		windowMetrics[window] = WindowMetrics{
			Window:        window,
			Good:          goodMetrics.Good,
			Total:         totalMetrics.Total,
			DataTimestamp: chosenTS,
		}

		// Staleness gating modifier: if any required window is stale -> result.IsStale = true
		if haveStalenessLimit && chosenTS != nil {
			age := now.Sub(*chosenTS)
			if age > stalenessLimit {
				result.IsStale = true
			}
		}
	}

	// Compliance window must exist (collectWindows includes it)
	complianceWindow := sloSpec.Spec.ComplianceWindow
	complianceMetrics, ok := windowMetrics[complianceWindow]
	if !ok {
		return nil, fmt.Errorf("missing metrics for compliance window %q", complianceWindow)
	}

	// Compute SLI for compliance window (used for budget remaining)
	result.SLI = ComputeSLI(complianceMetrics.Good, complianceMetrics.Total)

	// Compute burn rates + per-window SLI/error rate
	for window, metrics := range windowMetrics {
		sliResult := ComputeSLI(metrics.Good, metrics.Total)
		burnRate := ComputeBurnRate(sliResult.ErrorRate, sloSpec.Spec.Objective)

		result.BurnRates[window] = BurnRateResult{
			Window:    window,
			BurnRate:  burnRate,
			SLI:       sliResult.Value,
			ErrorRate: sliResult.ErrorRate,
		}

		// Insufficient data modifier: if ANY window has total==0, treat evaluation as insufficient
		if sliResult.InsufficientData {
			result.InsufficientData = true
		}
	}

	// Budget remaining is defined over the compliance window (per PED)
	result.BudgetRemaining = ComputeBudgetRemaining(result.SLI.ErrorRate, sloSpec.Spec.Objective)

	return result, nil
}

// collectWindows extracts all unique windows from burn policy rules.
func (e *Evaluator) collectWindows(sloSpec *slo.SLO) []string {
	windowSet := make(map[string]struct{})

	// Add compliance window
	windowSet[sloSpec.Spec.ComplianceWindow] = struct{}{}

	// Add all burn policy windows
	for _, rule := range sloSpec.Spec.BurnPolicy.Rules {
		windowSet[rule.ShortWindow] = struct{}{}
		windowSet[rule.LongWindow] = struct{}{}
	}

	// Convert to slice
	windows := make([]string, 0, len(windowSet))
	for w := range windowSet {
		windows = append(windows, w)
	}
	return windows
}
