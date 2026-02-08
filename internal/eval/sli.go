package eval

import "math"

// ComputeSLI calculates the SLI value from good and total metrics
// SLI = good / total
func ComputeSLI(good, total float64) SLIResult {
	// Handle zero traffic
	if total == 0 {
		return SLIResult{
			Value:            0,
			ErrorRate:        0,
			InsufficientData: true,
			Reason:           "no traffic (total=0)",
		}
	}

	// Ensure good <= total
	if good > total {
		good = total
	}

	// Compute SLI
	sli := good / total

	// Compute error rate: E = max(0, 1 - SLI)
	errorRate := math.Max(0, 1-sli)

	return SLIResult{
		Value:            sli,
		ErrorRate:        errorRate,
		InsufficientData: false,
		Reason:           "",
	}
}

// ComputeBurnRate calculates the burn rate from error rate and error budget
// burn_rate = error_rate / error_budget
// where error_budget = 1 - objective
func ComputeBurnRate(errorRate, objective float64) float64 {
	errorBudget := 1 - objective
	if errorBudget <= 0 {
		return 0
	}
	return errorRate / errorBudget
}

// ComputeBudgetRemaining calculates remaining error budget
// remaining_budget = 1 - (consumed_errors / allowed_errors)
func ComputeBudgetRemaining(errorRate, objective float64) float64 {
	errorBudget := 1 - objective
	if errorBudget <= 0 {
		return 0
	}
	if errorRate < 0 {
		errorRate = 0
	}
	consumed := errorRate / errorBudget
	remaining := 1 - consumed
	if remaining < 0 {
		return 0
	}
	if remaining > 1 {
		return 1
	}
	return remaining
}

