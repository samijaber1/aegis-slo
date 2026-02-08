package policy

import (
	"fmt"

	"github.com/samijaber1/aegis-slo/internal/eval"
	"github.com/samijaber1/aegis-slo/internal/slo"
)

// Engine evaluates burn policies and produces gate decisions
type Engine struct{}

// NewEngine creates a new policy engine
func NewEngine() *Engine {
	return &Engine{}
}

// Evaluate applies burn policies and gating modifiers to produce a decision
func (e *Engine) Evaluate(sloSpec *slo.SLO, evalResult *eval.EvaluationResult) *GateResult {
	result := &GateResult{
		Decision:     DecisionALLOW,
		RuleResults:  []RuleResult{},
		Reasons:      []string{},
		IsStale:      evalResult.IsStale,
		HasNoTraffic: evalResult.InsufficientData,
	}

	// Apply gating modifiers first
	if evalResult.IsStale {
		result.Decision = DecisionWARN
		result.Reasons = append(result.Reasons, "data is stale")
	}

	if evalResult.InsufficientData {
		result.Decision = DecisionWARN
		result.Reasons = append(result.Reasons, "insufficient data (zero traffic)")
	}

	// Evaluate burn policy rules
	for _, rule := range sloSpec.Spec.BurnPolicy.Rules {
		ruleResult := e.evaluateRule(rule, evalResult)
		result.RuleResults = append(result.RuleResults, ruleResult)

		if ruleResult.Triggered {
			// Aggregate decisions: BLOCK > WARN > ALLOW
			if ruleResult.Action == DecisionBLOCK {
				result.Decision = DecisionBLOCK
			} else if ruleResult.Action == DecisionWARN && result.Decision != DecisionBLOCK {
				result.Decision = DecisionWARN
			}

			result.Reasons = append(result.Reasons, ruleResult.Reason)
		}
	}

	// If no specific reasons but decision is ALLOW, add positive reason
	if result.Decision == DecisionALLOW && len(result.Reasons) == 0 {
		result.Reasons = append(result.Reasons, "all burn rate checks passed")
	}

	return result
}

// evaluateRule evaluates a single burn rate rule
// Rule triggers if: burn_short >= threshold AND burn_long >= threshold
func (e *Engine) evaluateRule(rule slo.BurnRule, evalResult *eval.EvaluationResult) RuleResult {
	ruleResult := RuleResult{
		RuleName: rule.Name,
		Action:   Decision(rule.Action),
	}

	// Get burn rates for short and long windows
	shortBurn, shortExists := evalResult.BurnRates[rule.ShortWindow]
	longBurn, longExists := evalResult.BurnRates[rule.LongWindow]

	if !shortExists || !longExists {
		ruleResult.Triggered = false
		ruleResult.Reason = fmt.Sprintf("rule %s: missing window data", rule.Name)
		return ruleResult
	}

	ruleResult.ShortBurnRate = shortBurn.BurnRate
	ruleResult.LongBurnRate = longBurn.BurnRate
	ruleResult.Threshold = rule.Threshold

	// Check if both windows exceed threshold
	if shortBurn.BurnRate >= rule.Threshold && longBurn.BurnRate >= rule.Threshold {
		ruleResult.Triggered = true
		ruleResult.Reason = fmt.Sprintf(
			"rule %s triggered: short=%.2fx, long=%.2fx (threshold=%.2fx)",
			rule.Name,
			shortBurn.BurnRate,
			longBurn.BurnRate,
			rule.Threshold,
		)
	} else {
		ruleResult.Triggered = false
	}

	return ruleResult
}
