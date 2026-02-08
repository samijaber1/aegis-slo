package slo

// SLO represents the parsed SLO definition
type SLO struct {
	APIVersion string   `yaml:"apiVersion"`
	Kind       string   `yaml:"kind"`
	Metadata   Metadata `yaml:"metadata"`
	Spec       Spec     `yaml:"spec"`
}

// Metadata contains SLO metadata
type Metadata struct {
	ID          string `yaml:"id"`
	Service     string `yaml:"service"`
	Owner       string `yaml:"owner,omitempty"`
	Description string `yaml:"description,omitempty"`
}

// Spec contains SLO specification
type Spec struct {
	Environment        string     `yaml:"environment"`
	Objective          float64    `yaml:"objective"`
	ComplianceWindow   string     `yaml:"complianceWindow"`
	EvaluationInterval string     `yaml:"evaluationInterval"`
	SLI                SLI        `yaml:"sli"`
	BurnPolicy         BurnPolicy `yaml:"burnPolicy"`
	Gating             Gating     `yaml:"gating"`
}

// SLI defines the Service Level Indicator
type SLI struct {
	Type        string   `yaml:"type"`
	ThresholdMs *int     `yaml:"thresholdMs,omitempty"`
	Good        QueryRef `yaml:"good"`
	Total       QueryRef `yaml:"total"`
}

// QueryRef contains the Prometheus query
type QueryRef struct {
	PrometheusQuery string `yaml:"prometheusQuery"`
}

// BurnPolicy defines burn rate policies
type BurnPolicy struct {
	Rules []BurnRule `yaml:"rules"`
}

// BurnRule defines a single burn rate rule
type BurnRule struct {
	Name        string  `yaml:"name"`
	ShortWindow string  `yaml:"shortWindow"`
	LongWindow  string  `yaml:"longWindow"`
	Threshold   float64 `yaml:"threshold"`
	Action      string  `yaml:"action"`
}

// Gating defines gating configuration
type Gating struct {
	MinDataPoints  int    `yaml:"minDataPoints"`
	StalenessLimit string `yaml:"stalenessLimit"`
}

// SLOWithFile pairs an SLO with its source file path
type SLOWithFile struct {
	SLO  *SLO
	File string
}

// ValidationError represents a validation error for a specific file
type ValidationError struct {
	File    string
	Path    string
	Message string
}

// Error implements the error interface
func (e ValidationError) Error() string {
	if e.Path != "" {
		return e.File + ": " + e.Path + ": " + e.Message
	}
	return e.File + ": " + e.Message
}
