package policy

// Decision represents a gate decision
type Decision string

const (
	DecisionALLOW Decision = "ALLOW"
	DecisionWARN  Decision = "WARN"
	DecisionBLOCK Decision = "BLOCK"
)

// RuleResult represents the result of evaluating a burn rule
type RuleResult struct {
	RuleName      string
	Triggered     bool
	Action        Decision
	ShortBurnRate float64
	LongBurnRate  float64
	Threshold     float64
	Reason        string
}

// GateResult represents the final gate decision
type GateResult struct {
	Decision     Decision
	RuleResults  []RuleResult
	Reasons      []string
	IsStale      bool
	HasNoTraffic bool
}
