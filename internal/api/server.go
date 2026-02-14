package api

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/samijaber1/aegis-slo/internal/scheduler"
	"github.com/samijaber1/aegis-slo/internal/storage"
)

// Server is the HTTP API server
type Server struct {
	scheduler *scheduler.Scheduler
	server    *http.Server
}

// NewServer creates a new API server
func NewServer(sched *scheduler.Scheduler, addr string) *Server {
	s := &Server{
		scheduler: sched,
	}

	mux := http.NewServeMux()

	// Health endpoints
	mux.HandleFunc("/healthz", s.handleHealth)
	mux.HandleFunc("/readyz", s.handleReady)

	// SLO endpoints
	mux.HandleFunc("/v1/slo", s.handleSLOList)
	mux.HandleFunc("/v1/slo/", s.handleSLOGet)

	// State endpoint
	mux.HandleFunc("/v1/state/", s.handleState)

	// Gate decision endpoint
	mux.HandleFunc("/v1/gate/decision", s.handleGateDecision)

	// Audit endpoint
	mux.HandleFunc("/v1/audit", s.handleAudit)

	s.server = &http.Server{
		Addr:         addr,
		Handler:      loggingMiddleware(mux),
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
	}

	return s
}

// Start starts the HTTP server
func (s *Server) Start() error {
	log.Printf("Starting API server on %s", s.server.Addr)
	if err := s.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return err
	}
	return nil
}

// Shutdown gracefully shuts down the server
func (s *Server) Shutdown(ctx context.Context) error {
	log.Println("Shutting down API server...")
	return s.server.Shutdown(ctx)
}

// handleHealth handles GET /healthz
func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	respondJSON(w, http.StatusOK, HealthResponse{Status: "ok"})
}

// handleReady handles GET /readyz
func (s *Server) handleReady(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	slos := s.scheduler.GetSLOs()
	cacheSize := s.scheduler.GetCache().Size()

	ready := len(slos) > 0
	reasons := []string{}

	if len(slos) == 0 {
		reasons = append(reasons, "no SLOs loaded")
	}

	if cacheSize == 0 {
		reasons = append(reasons, "no evaluations cached yet")
	}

	status := http.StatusOK
	if !ready {
		status = http.StatusServiceUnavailable
	}

	respondJSON(w, status, ReadyResponse{
		Ready:      ready,
		SLOsLoaded: len(slos),
		Reasons:    reasons,
	})
}

// handleSLOList handles GET /v1/slo
func (s *Server) handleSLOList(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	slos := s.scheduler.GetSLOs()

	summaries := make([]SLOSummary, 0, len(slos))
	for _, sloWithFile := range slos {
		summaries = append(summaries, SLOSummary{
			ID:          sloWithFile.SLO.Metadata.ID,
			Service:     sloWithFile.SLO.Metadata.Service,
			Environment: sloWithFile.SLO.Spec.Environment,
			Objective:   sloWithFile.SLO.Spec.Objective,
		})
	}

	respondJSON(w, http.StatusOK, SLOListResponse{SLOs: summaries})
}

// handleSLOGet handles GET /v1/slo/{id}
func (s *Server) handleSLOGet(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Extract ID from path
	id := strings.TrimPrefix(r.URL.Path, "/v1/slo/")
	if id == "" {
		respondError(w, http.StatusBadRequest, "SLO ID required")
		return
	}

	slos := s.scheduler.GetSLOs()
	for _, sloWithFile := range slos {
		if sloWithFile.SLO.Metadata.ID == id {
			respondJSON(w, http.StatusOK, sloWithFile.SLO)
			return
		}
	}

	respondError(w, http.StatusNotFound, fmt.Sprintf("SLO not found: %s", id))
}

// handleState handles GET /v1/state/{service}/{env}
func (s *Server) handleState(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Extract service and environment from path
	path := strings.TrimPrefix(r.URL.Path, "/v1/state/")
	parts := strings.Split(path, "/")
	if len(parts) != 2 {
		respondError(w, http.StatusBadRequest, "invalid path format, expected /v1/state/{service}/{env}")
		return
	}

	service := parts[0]
	env := parts[1]

	// Find matching SLOs
	slos := s.scheduler.GetSLOs()
	cache := s.scheduler.GetCache()

	matchingSLOs := []string{}
	decisions := make(map[string]string)
	var lastUpdated time.Time

	for _, sloWithFile := range slos {
		if sloWithFile.SLO.Metadata.Service == service && sloWithFile.SLO.Spec.Environment == env {
			id := sloWithFile.SLO.Metadata.ID
			matchingSLOs = append(matchingSLOs, id)

			if state, ok := cache.Get(id); ok {
				decisions[id] = string(state.GateResult.Decision)
				if state.UpdatedAt.After(lastUpdated) {
					lastUpdated = state.UpdatedAt
				}
			}
		}
	}

	if len(matchingSLOs) == 0 {
		respondError(w, http.StatusNotFound, fmt.Sprintf("no SLOs found for service=%s, env=%s", service, env))
		return
	}

	respondJSON(w, http.StatusOK, StateResponse{
		Service:     service,
		Environment: env,
		SLOs:        matchingSLOs,
		Decisions:   decisions,
		LastUpdated: lastUpdated,
	})
}

// handleGateDecision handles POST /v1/gate/decision
func (s *Server) handleGateDecision(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req DecisionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, fmt.Sprintf("invalid request: %v", err))
		return
	}

	if req.SLOID == "" {
		respondError(w, http.StatusBadRequest, "sloID required")
		return
	}

	// Force fresh evaluation if requested
	if req.ForceFresh {
		if err := s.scheduler.EvaluateNow(req.SLOID); err != nil {
			respondError(w, http.StatusInternalServerError, fmt.Sprintf("evaluation failed: %v", err))
			return
		}
	}

	// Get cached state
	cache := s.scheduler.GetCache()
	state, ok := cache.Get(req.SLOID)
	if !ok {
		respondError(w, http.StatusNotFound, fmt.Sprintf("no evaluation found for SLO: %s", req.SLOID))
		return
	}

	// Build response
	burnRates := make(map[string]BurnRateInfo)
	for window, br := range state.EvalResult.BurnRates {
		burnRates[window] = BurnRateInfo{
			BurnRate: br.BurnRate,
		}
	}

	// Add thresholds from triggered rules
	for _, rr := range state.GateResult.RuleResults {
		if rr.Triggered {
			// Add threshold to corresponding windows
			// Note: This is simplified - in production you'd match windows to rules more precisely
			for window := range burnRates {
				info := burnRates[window]
				info.Threshold = rr.Threshold
				burnRates[window] = info
			}
		}
	}

	response := DecisionResponse{
		Decision:  string(state.GateResult.Decision),
		SLOID:     state.EvalResult.SLOID,
		Timestamp: state.EvalResult.Timestamp,
		TTL:       int(state.TTL.Seconds()),
		SLI: SLIInfo{
			Value:           state.EvalResult.SLI.Value,
			ErrorRate:       state.EvalResult.SLI.ErrorRate,
			BudgetRemaining: state.EvalResult.BudgetRemaining,
		},
		Reasons:      state.GateResult.Reasons,
		BurnRates:    burnRates,
		IsStale:      state.GateResult.IsStale,
		HasNoTraffic: state.GateResult.HasNoTraffic,
	}

	respondJSON(w, http.StatusOK, response)
}

// handleAudit handles GET /v1/audit
func (s *Server) handleAudit(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Get audit storage from scheduler
	auditStorage := s.scheduler.GetAuditStorage()
	if auditStorage == nil {
		respondError(w, http.StatusServiceUnavailable, "audit storage not configured")
		return
	}

	// Parse query parameters
	query := r.URL.Query()
	filter := storage.AuditFilter{
		SLOID:       query.Get("sloID"),
		Service:     query.Get("service"),
		Environment: query.Get("environment"),
		Decision:    query.Get("decision"),
	}

	if limitStr := query.Get("limit"); limitStr != "" {
		if limit, err := strconv.Atoi(limitStr); err == nil {
			filter.Limit = limit
		}
	}

	if offsetStr := query.Get("offset"); offsetStr != "" {
		if offset, err := strconv.Atoi(offsetStr); err == nil {
			filter.Offset = offset
		}
	}

	if startTimeStr := query.Get("startTime"); startTimeStr != "" {
		if startTime, err := time.Parse(time.RFC3339, startTimeStr); err == nil {
			filter.StartTime = &startTime
		}
	}

	if endTimeStr := query.Get("endTime"); endTimeStr != "" {
		if endTime, err := time.Parse(time.RFC3339, endTimeStr); err == nil {
			filter.EndTime = &endTime
		}
	}

	// Query audit records
	records, err := auditStorage.QueryAudit(filter)
	if err != nil {
		respondError(w, http.StatusInternalServerError, fmt.Sprintf("failed to query audit: %v", err))
		return
	}

	// Convert to response format
	responseRecords := make([]AuditRecordResponse, len(records))
	for i, record := range records {
		burnRates := make(map[string]BurnRateInfo)
		for window, br := range record.BurnRates {
			burnRates[window] = BurnRateInfo{
				BurnRate: br.BurnRate,
			}
		}

		responseRecords[i] = AuditRecordResponse{
			ID:              record.ID,
			SLOID:           record.SLOID,
			Service:         record.Service,
			Environment:     record.Environment,
			Decision:        record.Decision,
			SLI:             record.SLI,
			ErrorRate:       record.ErrorRate,
			BudgetRemaining: record.BudgetRemaining,
			IsStale:         record.IsStale,
			HasNoTraffic:    record.HasNoTraffic,
			Reasons:         record.Reasons,
			BurnRates:       burnRates,
			Timestamp:       record.Timestamp,
			CreatedAt:       record.CreatedAt,
		}
	}

	response := AuditResponse{
		Records: responseRecords,
		Total:   len(responseRecords),
	}

	respondJSON(w, http.StatusOK, response)
}

// Helper functions

func respondJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

func respondError(w http.ResponseWriter, status int, message string) {
	respondJSON(w, status, ErrorResponse{Error: message})
}

func loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		log.Printf("%s %s %s", r.Method, r.URL.Path, time.Since(start))
	})
}
