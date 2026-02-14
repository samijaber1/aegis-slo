package scheduler

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/samijaber1/aegis-slo/internal/eval"
	"github.com/samijaber1/aegis-slo/internal/policy"
	"github.com/samijaber1/aegis-slo/internal/slo"
	"github.com/samijaber1/aegis-slo/internal/storage"
)

// Scheduler manages periodic SLO evaluations
type Scheduler struct {
	evaluator    *eval.Evaluator
	policyEngine *policy.Engine
	cache        *StateCache
	sloDirectory string
	slos         []slo.SLOWithFile
	audit        storage.AuditStorage
	cancel       context.CancelFunc
	wg           sync.WaitGroup
	mu           sync.RWMutex
	running      bool
}

// NewScheduler creates a new scheduler
func NewScheduler(evaluator *eval.Evaluator, policyEngine *policy.Engine, sloDirectory string) *Scheduler {
	return &Scheduler{
		evaluator:    evaluator,
		policyEngine: policyEngine,
		cache:        NewStateCache(),
		sloDirectory: sloDirectory,
	}
}

// SetAuditStorage sets the audit storage backend (optional)
func (s *Scheduler) SetAuditStorage(audit storage.AuditStorage) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.audit = audit
}

// LoadSLOs loads SLOs from the configured directory
func (s *Scheduler) LoadSLOs() error {
	sloFiles, errors := slo.LoadFromDirectory(s.sloDirectory)
	if len(errors) > 0 {
		return fmt.Errorf("failed to load SLOs: %d errors", len(errors))
	}

	if len(sloFiles) == 0 {
		return fmt.Errorf("no SLOs found in %s", s.sloDirectory)
	}

	// Validate all SLOs
	validator, err := slo.NewValidator("schemas/slo_v1.json")
	if err != nil {
		return fmt.Errorf("failed to create validator: %w", err)
	}

	validationErrors := validator.ValidateDirectory(s.sloDirectory)
	if len(validationErrors) > 0 {
		return fmt.Errorf("SLO validation failed: %d errors", len(validationErrors))
	}

	s.mu.Lock()
	s.slos = sloFiles
	audit := s.audit
	s.mu.Unlock()

	// Persist SLO definitions to audit storage if available
	if audit != nil {
		for _, sloWithFile := range sloFiles {
			if err := audit.StoreSLODefinition(sloWithFile.SLO); err != nil {
				log.Printf("Warning: failed to store SLO definition %s: %v", sloWithFile.SLO.Metadata.ID, err)
			}
		}
	}

	log.Printf("Loaded %d SLOs", len(sloFiles))
	return nil
}

// Start begins the evaluation scheduler
func (s *Scheduler) Start() error {
	s.mu.Lock()
	if s.running {
		s.mu.Unlock()
		return fmt.Errorf("scheduler already running")
	}

	if len(s.slos) == 0 {
		s.mu.Unlock()
		return fmt.Errorf("no SLOs loaded, call LoadSLOs() first")
	}

	ctx, cancel := context.WithCancel(context.Background())
	s.cancel = cancel
	s.running = true
	slos := s.slos
	s.mu.Unlock()

	// Start one goroutine per SLO
	for _, sloWithFile := range slos {
		s.wg.Add(1)
		go s.evaluateLoop(ctx, sloWithFile.SLO)
	}

	log.Printf("Started scheduler for %d SLOs", len(slos))
	return nil
}

// Stop stops the scheduler and waits for all evaluations to complete
func (s *Scheduler) Stop() {
	s.mu.Lock()
	if !s.running {
		s.mu.Unlock()
		return
	}

	s.cancel()
	s.running = false
	s.mu.Unlock()

	log.Println("Stopping scheduler...")
	s.wg.Wait()
	log.Println("Scheduler stopped")
}

// evaluateLoop runs periodic evaluations for a single SLO
func (s *Scheduler) evaluateLoop(ctx context.Context, sloSpec *slo.SLO) {
	defer s.wg.Done()

	// Parse evaluation interval
	interval, err := slo.ParseDuration(sloSpec.Spec.EvaluationInterval)
	if err != nil {
		log.Printf("Error parsing evaluation interval for SLO %s: %v", sloSpec.Metadata.ID, err)
		return
	}

	// Initial evaluation
	s.evaluateOnce(sloSpec, interval)

	// Periodic evaluations
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.evaluateOnce(sloSpec, interval)
		}
	}
}

// evaluateOnce performs a single evaluation of an SLO
func (s *Scheduler) evaluateOnce(sloSpec *slo.SLO, interval time.Duration) {
	now := time.Now()

	// Evaluate SLO
	evalResult, err := s.evaluator.Evaluate(sloSpec, now)
	if err != nil {
		log.Printf("Error evaluating SLO %s: %v", sloSpec.Metadata.ID, err)
		return
	}

	// Apply policy
	gateResult := s.policyEngine.Evaluate(sloSpec, evalResult)

	// Cache the result
	state := &EvaluationState{
		EvalResult: evalResult,
		GateResult: gateResult,
		UpdatedAt:  now,
		TTL:        interval,
	}

	s.cache.Set(sloSpec.Metadata.ID, state)

	// Persist to audit storage if available
	s.mu.RLock()
	audit := s.audit
	s.mu.RUnlock()

	if audit != nil {
		// Store evaluation record
		if err := audit.StoreEvaluation(evalResult, gateResult); err != nil {
			log.Printf("Warning: failed to store evaluation for SLO %s: %v", sloSpec.Metadata.ID, err)
		}

		// Update latest state
		if err := audit.UpdateLatestState(sloSpec.Metadata.ID, evalResult, gateResult); err != nil {
			log.Printf("Warning: failed to update latest state for SLO %s: %v", sloSpec.Metadata.ID, err)
		}
	}

	log.Printf("Evaluated SLO %s: decision=%s, SLI=%.4f",
		sloSpec.Metadata.ID, gateResult.Decision, evalResult.SLI.Value)
}

// GetCache returns the state cache
func (s *Scheduler) GetCache() *StateCache {
	return s.cache
}

// GetAuditStorage returns the audit storage backend
func (s *Scheduler) GetAuditStorage() storage.AuditStorage {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.audit
}

// GetSLOs returns the loaded SLOs
func (s *Scheduler) GetSLOs() []slo.SLOWithFile {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// Return a copy
	result := make([]slo.SLOWithFile, len(s.slos))
	copy(result, s.slos)
	return result
}

// SetSLOsForTest sets SLOs directly (for testing only)
func (s *Scheduler) SetSLOsForTest(slos []slo.SLOWithFile) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.slos = slos
}

// EvaluateNow forces immediate evaluation of a specific SLO
func (s *Scheduler) EvaluateNow(sloID string) error {
	s.mu.RLock()
	var targetSLO *slo.SLO
	for _, sloWithFile := range s.slos {
		if sloWithFile.SLO.Metadata.ID == sloID {
			targetSLO = sloWithFile.SLO
			break
		}
	}
	s.mu.RUnlock()

	if targetSLO == nil {
		return fmt.Errorf("SLO not found: %s", sloID)
	}

	interval, err := slo.ParseDuration(targetSLO.Spec.EvaluationInterval)
	if err != nil {
		return fmt.Errorf("invalid evaluation interval: %w", err)
	}

	s.evaluateOnce(targetSLO, interval)
	return nil
}
