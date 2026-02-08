package synthetic

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/samijaber1/aegis-slo/internal/eval"
)

// MetricFixture represents a metric fixture file format
type MetricFixture struct {
	Windows map[string]WindowData `json:"windows"`
}

// WindowData represents metrics for a specific window
type WindowData struct {
	Good          float64    `json:"good"`
	Total         float64    `json:"total"`
	DataTimestamp *time.Time `json:"dataTimestamp,omitempty"`
}

// Adapter is a synthetic metrics adapter that reads from JSON fixtures
type Adapter struct {
	fixtures map[string]*MetricFixture
}

// NewAdapter creates a new synthetic adapter
func NewAdapter() *Adapter {
	return &Adapter{
		fixtures: make(map[string]*MetricFixture),
	}
}

// LoadFixture loads a metric fixture from a JSON file
func (a *Adapter) LoadFixture(name string, path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("failed to read fixture: %w", err)
	}

	var fixture MetricFixture
	if err := json.Unmarshal(data, &fixture); err != nil {
		return fmt.Errorf("failed to parse fixture: %w", err)
	}

	a.fixtures[name] = &fixture
	return nil
}

// SetFixture directly sets a fixture (useful for testing)
func (a *Adapter) SetFixture(name string, fixture *MetricFixture) {
	a.fixtures[name] = fixture
}

// QueryWindow implements the MetricsAdapter interface
// Query format: "fixture:name" where name is the fixture identifier
func (a *Adapter) QueryWindow(query string, window string) (eval.WindowMetrics, error) {
	// Parse query to extract fixture name
	fixtureName := a.parseQuery(query)
	if fixtureName == "" {
		return eval.WindowMetrics{}, fmt.Errorf("invalid query format: %s", query)
	}

	// Get fixture
	fixture, exists := a.fixtures[fixtureName]
	if !exists {
		return eval.WindowMetrics{}, fmt.Errorf("fixture not found: %s", fixtureName)
	}

	// Get window data
	windowData, exists := fixture.Windows[window]
	if !exists {
		return eval.WindowMetrics{}, fmt.Errorf("window not found in fixture: %s", window)
	}

	return eval.WindowMetrics{
		Window:        window,
		Good:          windowData.Good,
		Total:         windowData.Total,
		DataTimestamp: windowData.DataTimestamp,
	}, nil
}

// parseQuery extracts the fixture name from a query string
// Expected format: "sum(rate(...))" -> extract any identifier, or just use the whole query
// For simplicity, we'll use a convention: queries contain the fixture name as a comment or label
// Or we can use a simple format: just the fixture name itself
func (a *Adapter) parseQuery(query string) string {
	// For synthetic adapter, we expect the query to be just the fixture name
	// or to contain a fixture reference like "fixture:name"
	if strings.HasPrefix(query, "fixture:") {
		return strings.TrimPrefix(query, "fixture:")
	}

	// Otherwise, use the query itself as the fixture name (simplified)
	// This allows tests to just use fixture names directly
	return query
}
