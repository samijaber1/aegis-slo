package scheduler

import (
	"sync"
	"testing"
	"time"

	"github.com/samijaber1/aegis-slo/internal/eval"
	"github.com/samijaber1/aegis-slo/internal/policy"
)

func TestStateCache_Basics(t *testing.T) {
	cache := NewStateCache()

	// Initially empty
	if cache.Size() != 0 {
		t.Errorf("expected empty cache, got size %d", cache.Size())
	}

	// Set and get
	state := &EvaluationState{
		EvalResult: &eval.EvaluationResult{SLOID: "test-slo"},
		GateResult: &policy.GateResult{Decision: policy.DecisionALLOW},
		UpdatedAt:  time.Now(),
		TTL:        30 * time.Second,
	}

	cache.Set("test-slo", state)

	if cache.Size() != 1 {
		t.Errorf("expected size 1, got %d", cache.Size())
	}

	retrieved, ok := cache.Get("test-slo")
	if !ok {
		t.Fatal("expected to retrieve state")
	}

	if retrieved.EvalResult.SLOID != "test-slo" {
		t.Errorf("expected SLOID=test-slo, got %s", retrieved.EvalResult.SLOID)
	}

	// Delete
	cache.Delete("test-slo")
	if cache.Size() != 0 {
		t.Errorf("expected size 0 after delete, got %d", cache.Size())
	}

	_, ok = cache.Get("test-slo")
	if ok {
		t.Error("expected not to find deleted state")
	}
}

func TestStateCache_GetAll(t *testing.T) {
	cache := NewStateCache()

	// Add multiple states
	for i := 0; i < 3; i++ {
		state := &EvaluationState{
			EvalResult: &eval.EvaluationResult{SLOID: string(rune('a' + i))},
			UpdatedAt:  time.Now(),
		}
		cache.Set(string(rune('a'+i)), state)
	}

	all := cache.GetAll()
	if len(all) != 3 {
		t.Errorf("expected 3 states, got %d", len(all))
	}
}

func TestStateCache_Clear(t *testing.T) {
	cache := NewStateCache()

	cache.Set("slo1", &EvaluationState{})
	cache.Set("slo2", &EvaluationState{})

	cache.Clear()

	if cache.Size() != 0 {
		t.Errorf("expected size 0 after clear, got %d", cache.Size())
	}
}

func TestStateCache_IsStale(t *testing.T) {
	now := time.Now()
	state := &EvaluationState{
		UpdatedAt: now.Add(-1 * time.Minute),
		TTL:       30 * time.Second,
	}

	if !state.IsStale(now) {
		t.Error("expected state to be stale")
	}

	state.UpdatedAt = now.Add(-10 * time.Second)
	if state.IsStale(now) {
		t.Error("expected state to not be stale")
	}
}

func TestStateCache_Concurrency(t *testing.T) {
	cache := NewStateCache()
	var wg sync.WaitGroup

	// Concurrent writes
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			state := &EvaluationState{
				EvalResult: &eval.EvaluationResult{SLOID: string(rune('a' + id))},
			}
			cache.Set(string(rune('a'+id%26)), state)
		}(i)
	}

	// Concurrent reads
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			cache.Get(string(rune('a' + id%26)))
		}(i)
	}

	wg.Wait()

	// Should not panic and have some entries
	if cache.Size() == 0 {
		t.Error("expected some entries after concurrent operations")
	}
}
