package prometheus

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

func TestAdapter_QueryWindow(t *testing.T) {
	// Create a mock Prometheus server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		query := r.URL.Query().Get("query")

		// Check that {{window}} was substituted
		if query == "rate(requests[{{window}}])" {
			t.Error("window template not substituted")
		}

		// Return mock response
		resp := QueryResponse{
			Status: "success",
			Data: QueryData{
				ResultType: "vector",
				Result: []VectorResult{
					{
						Metric: map[string]string{"job": "test"},
						Value:  SamplePair{float64(time.Now().Unix()), "100.5"},
					},
				},
			},
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	// Create adapter
	config := DefaultConfig(server.URL)
	adapter := NewAdapter(config)

	// Test query
	result, err := adapter.QueryWindow("rate(requests[{{window}}])", "5m")
	if err != nil {
		t.Fatalf("query failed: %v", err)
	}

	if result.Window != "5m" {
		t.Errorf("expected window=5m, got %s", result.Window)
	}

	if result.Good != 100.5 {
		t.Errorf("expected value=100.5, got %f", result.Good)
	}

	if result.DataTimestamp == nil {
		t.Error("expected timestamp to be set")
	}
}

func TestAdapter_WindowSubstitution(t *testing.T) {
	tests := []struct {
		name     string
		query    string
		window   string
		expected string
	}{
		{
			name:     "single substitution",
			query:    "rate(metric[{{window}}])",
			window:   "5m",
			expected: "rate(metric[5m])",
		},
		{
			name:     "multiple substitutions",
			query:    "rate(good[{{window}}]) / rate(total[{{window}}])",
			window:   "1h",
			expected: "rate(good[1h]) / rate(total[1h])",
		},
		{
			name:     "no substitution needed",
			query:    "rate(metric[5m])",
			window:   "5m",
			expected: "rate(metric[5m])",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := substituteWindow(tt.query, tt.window)
			if result != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, result)
			}
		})
	}
}

func TestAdapter_Retry(t *testing.T) {
	var attempts int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		count := atomic.AddInt32(&attempts, 1)

		// Fail first attempt, succeed on second
		if count == 1 {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		resp := QueryResponse{
			Status: "success",
			Data: QueryData{
				ResultType: "vector",
				Result: []VectorResult{
					{
						Value: SamplePair{float64(time.Now().Unix()), "42"},
					},
				},
			},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	config := DefaultConfig(server.URL)
	config.RetryCount = 1
	config.RetryDelay = 10 * time.Millisecond
	adapter := NewAdapter(config)

	result, err := adapter.QueryWindow("test_metric", "5m")
	if err != nil {
		t.Fatalf("expected success after retry, got error: %v", err)
	}

	if result.Good != 42 {
		t.Errorf("expected value=42, got %f", result.Good)
	}

	if atomic.LoadInt32(&attempts) != 2 {
		t.Errorf("expected 2 attempts, got %d", attempts)
	}
}

func TestAdapter_Timeout(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Delay longer than timeout
		time.Sleep(200 * time.Millisecond)
		json.NewEncoder(w).Encode(QueryResponse{Status: "success"})
	}))
	defer server.Close()

	config := DefaultConfig(server.URL)
	config.Timeout = 50 * time.Millisecond
	config.RetryCount = 0
	adapter := NewAdapter(config)

	_, err := adapter.QueryWindow("test_metric", "5m")
	if err == nil {
		t.Error("expected timeout error, got nil")
	}
}

func TestAdapter_PrometheusError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := QueryResponse{
			Status: "error",
			Error:  "invalid query",
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	config := DefaultConfig(server.URL)
	adapter := NewAdapter(config)

	_, err := adapter.QueryWindow("invalid_query", "5m")
	if err == nil {
		t.Error("expected error, got nil")
	}

	if err != nil && err.Error() != "query failed after 2 attempts: prometheus error: invalid query" {
		t.Logf("got error: %v", err)
	}
}

func TestAdapter_Concurrency(t *testing.T) {
	var concurrent int32
	var maxConcurrent int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		current := atomic.AddInt32(&concurrent, 1)
		defer atomic.AddInt32(&concurrent, -1)

		// Track max concurrent requests
		for {
			max := atomic.LoadInt32(&maxConcurrent)
			if current <= max || atomic.CompareAndSwapInt32(&maxConcurrent, max, current) {
				break
			}
		}

		// Simulate some work
		time.Sleep(50 * time.Millisecond)

		resp := QueryResponse{
			Status: "success",
			Data: QueryData{
				ResultType: "vector",
				Result:     []VectorResult{{Value: SamplePair{float64(time.Now().Unix()), "1"}}},
			},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	config := DefaultConfig(server.URL)
	config.MaxConcurrency = 3
	config.Timeout = 5 * time.Second
	adapter := NewAdapter(config)

	// Launch 10 concurrent queries
	done := make(chan error, 10)
	for i := 0; i < 10; i++ {
		go func(id int) {
			_, err := adapter.QueryWindow(fmt.Sprintf("metric_%d", id), "5m")
			done <- err
		}(i)
	}

	// Wait for all queries
	for i := 0; i < 10; i++ {
		if err := <-done; err != nil {
			t.Errorf("query %d failed: %v", i, err)
		}
	}

	max := atomic.LoadInt32(&maxConcurrent)
	if max > int32(config.MaxConcurrency) {
		t.Errorf("max concurrent requests (%d) exceeded limit (%d)", max, config.MaxConcurrency)
	}

	t.Logf("Max concurrent requests: %d (limit: %d)", max, config.MaxConcurrency)
}

func TestAdapter_ZeroResults(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := QueryResponse{
			Status: "success",
			Data: QueryData{
				ResultType: "vector",
				Result:     []VectorResult{}, // No results
			},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	config := DefaultConfig(server.URL)
	adapter := NewAdapter(config)

	result, err := adapter.QueryWindow("missing_metric", "5m")
	if err != nil {
		t.Fatalf("query failed: %v", err)
	}

	if result.Good != 0 {
		t.Errorf("expected value=0 for no results, got %f", result.Good)
	}

	if result.DataTimestamp != nil {
		t.Error("expected nil timestamp for no results")
	}
}

func TestExtractScalarValue(t *testing.T) {
	tests := []struct {
		name     string
		response *QueryResponse
		expected float64
	}{
		{
			name: "single result",
			response: &QueryResponse{
				Data: QueryData{
					Result: []VectorResult{
						{Value: SamplePair{0, "42.5"}},
					},
				},
			},
			expected: 42.5,
		},
		{
			name: "multiple results summed",
			response: &QueryResponse{
				Data: QueryData{
					Result: []VectorResult{
						{Value: SamplePair{0, "10"}},
						{Value: SamplePair{0, "20"}},
						{Value: SamplePair{0, "30"}},
					},
				},
			},
			expected: 60,
		},
		{
			name: "no results",
			response: &QueryResponse{
				Data: QueryData{
					Result: []VectorResult{},
				},
			},
			expected: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractScalarValue(tt.response)
			if result != tt.expected {
				t.Errorf("expected %f, got %f", tt.expected, result)
			}
		})
	}
}
