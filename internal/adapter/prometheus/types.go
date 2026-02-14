package prometheus

import (
	"fmt"
	"time"
)

// QueryResponse represents a Prometheus query API response
type QueryResponse struct {
	Status string    `json:"status"`
	Data   QueryData `json:"data"`
	Error  string    `json:"error,omitempty"`
}

// QueryData contains the query result data
type QueryData struct {
	ResultType string         `json:"resultType"`
	Result     []VectorResult `json:"result"`
}

// VectorResult represents a single result from an instant vector query
type VectorResult struct {
	Metric map[string]string `json:"metric"`
	Value  SamplePair        `json:"value"`
}

// SamplePair is [timestamp, value]
type SamplePair [2]interface{}

// Timestamp returns the timestamp from the sample pair
func (sp SamplePair) Timestamp() time.Time {
	if len(sp) < 1 {
		return time.Time{}
	}
	if ts, ok := sp[0].(float64); ok {
		return time.Unix(int64(ts), 0)
	}
	return time.Time{}
}

// Value returns the value from the sample pair
func (sp SamplePair) Value() float64 {
	if len(sp) < 2 {
		return 0
	}
	if str, ok := sp[1].(string); ok {
		// Parse string to float
		var val float64
		_, _ = fmt.Sscanf(str, "%f", &val)
		return val
	}
	if val, ok := sp[1].(float64); ok {
		return val
	}
	return 0
}
