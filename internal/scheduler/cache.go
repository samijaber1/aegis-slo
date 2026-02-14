package scheduler

import (
	"sync"
	"time"

	"github.com/samijaber1/aegis-slo/internal/eval"
	"github.com/samijaber1/aegis-slo/internal/policy"
)

// EvaluationState represents the cached evaluation state for an SLO
type EvaluationState struct {
	EvalResult *eval.EvaluationResult
	GateResult *policy.GateResult
	UpdatedAt  time.Time
	TTL        time.Duration
}

// IsStale returns true if the cached state is older than its TTL
func (s *EvaluationState) IsStale(now time.Time) bool {
	return now.Sub(s.UpdatedAt) > s.TTL
}

// StateCache is a thread-safe cache for SLO evaluation states
type StateCache struct {
	mu     sync.RWMutex
	states map[string]*EvaluationState
}

// NewStateCache creates a new state cache
func NewStateCache() *StateCache {
	return &StateCache{
		states: make(map[string]*EvaluationState),
	}
}

// Get retrieves cached state for an SLO
func (c *StateCache) Get(sloID string) (*EvaluationState, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	state, exists := c.states[sloID]
	return state, exists
}

// Set stores evaluation state for an SLO
func (c *StateCache) Set(sloID string, state *EvaluationState) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.states[sloID] = state
}

// GetAll returns all cached states
func (c *StateCache) GetAll() map[string]*EvaluationState {
	c.mu.RLock()
	defer c.mu.RUnlock()

	// Create a copy to avoid external modifications
	snapshot := make(map[string]*EvaluationState, len(c.states))
	for k, v := range c.states {
		snapshot[k] = v
	}

	return snapshot
}

// Delete removes a cached state
func (c *StateCache) Delete(sloID string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	delete(c.states, sloID)
}

// Clear removes all cached states
func (c *StateCache) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.states = make(map[string]*EvaluationState)
}

// Size returns the number of cached states
func (c *StateCache) Size() int {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return len(c.states)
}
