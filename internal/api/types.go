package api

import (
	"time"
)

// DecisionRequest represents a gate decision request
type DecisionRequest struct {
	SLOID      string `json:"sloID"`
	ForceFresh bool   `json:"forceFresh,omitempty"`
}

// DecisionResponse represents a gate decision response
type DecisionResponse struct {
	Decision     string                  `json:"decision"`
	SLOID        string                  `json:"sloID"`
	Timestamp    time.Time               `json:"timestamp"`
	TTL          int                     `json:"ttl"` // seconds
	SLI          SLIInfo                 `json:"sli"`
	Reasons      []string                `json:"reasons"`
	BurnRates    map[string]BurnRateInfo `json:"burnRates"`
	IsStale      bool                    `json:"isStale"`
	HasNoTraffic bool                    `json:"hasNoTraffic"`
}

// SLIInfo contains SLI metrics
type SLIInfo struct {
	Value           float64 `json:"value"`
	ErrorRate       float64 `json:"errorRate"`
	BudgetRemaining float64 `json:"budgetRemaining"`
}

// BurnRateInfo contains burn rate information for a window
type BurnRateInfo struct {
	BurnRate  float64 `json:"burnRate"`
	Threshold float64 `json:"threshold,omitempty"`
}

// SLOListResponse represents a list of SLOs
type SLOListResponse struct {
	SLOs []SLOSummary `json:"slos"`
}

// SLOSummary contains summary information about an SLO
type SLOSummary struct {
	ID          string  `json:"id"`
	Service     string  `json:"service"`
	Environment string  `json:"environment"`
	Objective   float64 `json:"objective"`
}

// StateResponse represents the evaluation state for a service/environment
type StateResponse struct {
	Service     string            `json:"service"`
	Environment string            `json:"environment"`
	SLOs        []string          `json:"slos"`
	Decisions   map[string]string `json:"decisions"`
	LastUpdated time.Time         `json:"lastUpdated"`
}

// HealthResponse represents health check response
type HealthResponse struct {
	Status string `json:"status"`
}

// ReadyResponse represents readiness check response
type ReadyResponse struct {
	Ready      bool     `json:"ready"`
	SLOsLoaded int      `json:"slosLoaded"`
	Reasons    []string `json:"reasons,omitempty"`
}

// ErrorResponse represents an error response
type ErrorResponse struct {
	Error string `json:"error"`
}
