package eval

import (
	"math"
	"testing"
)

func TestComputeSLI(t *testing.T) {
	tests := []struct {
		name              string
		good              float64
		total             float64
		expectedSLI       float64
		expectedErrorRate float64
		insufficientData  bool
	}{
		{
			name:              "perfect availability",
			good:              100,
			total:             100,
			expectedSLI:       1.0,
			expectedErrorRate: 0.0,
		},
		{
			name:              "99.9% availability",
			good:              999,
			total:             1000,
			expectedSLI:       0.999,
			expectedErrorRate: 0.001,
		},
		{
			name:              "98% availability",
			good:              98,
			total:             100,
			expectedSLI:       0.98,
			expectedErrorRate: 0.02,
		},
		{
			name:             "zero traffic",
			good:             0,
			total:            0,
			insufficientData: true,
		},
		{
			name:              "all errors",
			good:              0,
			total:             100,
			expectedSLI:       0.0,
			expectedErrorRate: 1.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ComputeSLI(tt.good, tt.total)

			if result.InsufficientData != tt.insufficientData {
				t.Errorf("expected InsufficientData=%v, got %v",
					tt.insufficientData, result.InsufficientData)
			}

			if !tt.insufficientData {
				if math.Abs(result.Value-tt.expectedSLI) > 0.0001 {
					t.Errorf("expected SLI=%.4f, got %.4f",
						tt.expectedSLI, result.Value)
				}

				if math.Abs(result.ErrorRate-tt.expectedErrorRate) > 0.0001 {
					t.Errorf("expected ErrorRate=%.4f, got %.4f",
						tt.expectedErrorRate, result.ErrorRate)
				}
			}
		})
	}
}

func TestComputeBurnRate(t *testing.T) {
	tests := []struct {
		name             string
		errorRate        float64
		objective        float64
		expectedBurnRate float64
	}{
		{
			name:             "no errors",
			errorRate:        0.0,
			objective:        0.999,
			expectedBurnRate: 0.0,
		},
		{
			name:             "1x burn rate",
			errorRate:        0.001,
			objective:        0.999,
			expectedBurnRate: 1.0,
		},
		{
			name:             "14x burn rate",
			errorRate:        0.014,
			objective:        0.999,
			expectedBurnRate: 14.0,
		},
		{
			name:             "2% errors on 99% objective",
			errorRate:        0.02,
			objective:        0.99,
			expectedBurnRate: 2.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			burnRate := ComputeBurnRate(tt.errorRate, tt.objective)

			if math.Abs(burnRate-tt.expectedBurnRate) > 0.0001 {
				t.Errorf("expected burn rate=%.4f, got %.4f",
					tt.expectedBurnRate, burnRate)
			}
		})
	}
}

func TestComputeBudgetRemaining(t *testing.T) {
	tests := []struct {
		name                    string
		errorRate               float64
		objective               float64
		expectedBudgetRemaining float64
	}{
		{
			name:                    "full budget",
			errorRate:               0.0,
			objective:               0.999,
			expectedBudgetRemaining: 1.0,
		},
		{
			name:                    "half budget consumed",
			errorRate:               0.0005,
			objective:               0.999,
			expectedBudgetRemaining: 0.5,
		},
		{
			name:                    "budget exhausted",
			errorRate:               0.001,
			objective:               0.999,
			expectedBudgetRemaining: 0.0,
		},
		{
			name:                    "over budget",
			errorRate:               0.002,
			objective:               0.999,
			expectedBudgetRemaining: 0.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			remaining := ComputeBudgetRemaining(tt.errorRate, tt.objective)

			if math.Abs(remaining-tt.expectedBudgetRemaining) > 0.0001 {
				t.Errorf("expected budget remaining=%.4f, got %.4f",
					tt.expectedBudgetRemaining, remaining)
			}
		})
	}
}
