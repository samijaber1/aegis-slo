package prometheus

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/samijaber1/aegis-slo/internal/eval"
	"golang.org/x/sync/semaphore"
)

// Config holds Prometheus adapter configuration
type Config struct {
	URL            string
	Timeout        time.Duration
	MaxConcurrency int64
	RetryCount     int
	RetryDelay     time.Duration
}

// DefaultConfig returns default configuration
func DefaultConfig(prometheusURL string) Config {
	return Config{
		URL:            prometheusURL,
		Timeout:        10 * time.Second,
		MaxConcurrency: 10,
		RetryCount:     1,
		RetryDelay:     100 * time.Millisecond,
	}
}

// Adapter is a Prometheus metrics adapter
type Adapter struct {
	config Config
	client *http.Client
	sem    *semaphore.Weighted
}

// NewAdapter creates a new Prometheus adapter
func NewAdapter(config Config) *Adapter {
	return &Adapter{
		config: config,
		client: &http.Client{
			Timeout: config.Timeout,
		},
		sem: semaphore.NewWeighted(config.MaxConcurrency),
	}
}

// QueryWindow implements the MetricsAdapter interface
// It executes a Prometheus instant query with {{window}} substituted
func (a *Adapter) QueryWindow(query string, window string) (eval.WindowMetrics, error) {
	// Substitute {{window}} with actual window value
	instantQuery := substituteWindow(query, window)

	// Acquire semaphore to limit concurrency
	ctx, cancel := context.WithTimeout(context.Background(), a.config.Timeout)
	defer cancel()

	if err := a.sem.Acquire(ctx, 1); err != nil {
		return eval.WindowMetrics{}, fmt.Errorf("semaphore acquire: %w", err)
	}
	defer a.sem.Release(1)

	// Execute query with retry
	var lastErr error
	for attempt := 0; attempt <= a.config.RetryCount; attempt++ {
		if attempt > 0 {
			time.Sleep(a.config.RetryDelay)
		}

		result, err := a.executeQuery(ctx, instantQuery)
		if err == nil {
			// Extract scalar value from result
			value := extractScalarValue(result)
			timestamp := extractTimestamp(result)

			return eval.WindowMetrics{
				Window:        window,
				Good:          value,
				Total:         value, // For instant queries, good=total (caller queries separately)
				DataTimestamp: timestamp,
			}, nil
		}

		lastErr = err
	}

	return eval.WindowMetrics{}, fmt.Errorf("query failed after %d attempts: %w", a.config.RetryCount+1, lastErr)
}

// executeQuery performs a single Prometheus query
func (a *Adapter) executeQuery(ctx context.Context, query string) (*QueryResponse, error) {
	// Build query URL
	queryURL := fmt.Sprintf("%s/api/v1/query", strings.TrimSuffix(a.config.URL, "/"))

	// Add query parameter
	params := url.Values{}
	params.Add("query", query)

	fullURL := queryURL + "?" + params.Encode()

	// Create request
	req, err := http.NewRequestWithContext(ctx, "GET", fullURL, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	// Execute request
	resp, err := a.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("http request: %w", err)
	}
	defer resp.Body.Close()

	// Read response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	// Check HTTP status
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("http status %d: %s", resp.StatusCode, string(body))
	}

	// Parse JSON response
	var result QueryResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("parse response: %w", err)
	}

	// Check Prometheus status
	if result.Status != "success" {
		return nil, fmt.Errorf("prometheus error: %s", result.Error)
	}

	return &result, nil
}

// substituteWindow replaces {{window}} placeholder with actual window value
func substituteWindow(query string, window string) string {
	return strings.ReplaceAll(query, "{{window}}", window)
}

// extractScalarValue extracts a scalar value from query response
// Aggregates all results by summing them
func extractScalarValue(resp *QueryResponse) float64 {
	if resp == nil || len(resp.Data.Result) == 0 {
		return 0
	}

	var sum float64
	for _, result := range resp.Data.Result {
		sum += result.Value.Value()
	}

	return sum
}

// extractTimestamp extracts the most recent timestamp from query response
func extractTimestamp(resp *QueryResponse) *time.Time {
	if resp == nil || len(resp.Data.Result) == 0 {
		return nil
	}

	var latest time.Time
	for _, result := range resp.Data.Result {
		ts := result.Value.Timestamp()
		if ts.After(latest) {
			latest = ts
		}
	}

	if latest.IsZero() {
		return nil
	}

	return &latest
}
