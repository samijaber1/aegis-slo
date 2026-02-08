# AegisSLO — Tasks & Milestone Plan

## Principles
- Build milestone-by-milestone; no scope creep.
- Every milestone ends with: tests passing + CI green + documentation updated.
- Small PRs only (one theme per PR).

---

## Milestone 0 — Repo hygiene (PR-0)
### Tasks
- [ ] Add docs/PED.md
- [ ] Add schemas/slo_v1.json
- [ ] Initialize Go module
- [ ] Create skeleton directories per PED
- [ ] Add GitHub Actions CI (fmt/vet/test)
- [ ] Add basic README scaffold + badges

### Acceptance criteria
- CI runs on push/PR and is green
- `go test ./...` passes on a clean clone

---

## Milestone 1 — SLO loading + schema validation (PR-1..PR-2)
### Tasks
- [ ] Implement SLO file discovery from a directory (glob *.yaml/*.yml)
- [ ] Parse YAML into raw bytes + metadata extraction
- [ ] Validate YAML against schemas/slo_v1.json (JSON Schema draft 2020-12)
- [ ] Implement extra validation:
  - [ ] metadata.id uniqueness across loaded SLOs
  - [ ] complianceWindow >= max(policy windows) (duration compare)
  - [ ] objective in (0,1) (schema enforces; keep as guard)
- [ ] Return structured validation errors (file, field, message)
- [ ] Unit tests for validator
- [ ] Add fixtures for valid + invalid examples

### Acceptance criteria
- `aegis validate --dir ./fixtures/slo` reports errors deterministically
- Invalid files don’t crash the loader; valid ones still load
- Unit tests cover at least: missing required fields, invalid duration, duplicate IDs

---

## Milestone 2 — Evaluation core (math + synthetic adapter) (PR-3..PR-5)
### Tasks
- [ ] Implement duration parsing utilities (s/m/h/d)
- [ ] Implement SLI computation:
  - [ ] ratio SLI
  - [ ] latency_threshold SLI
- [ ] Implement burn rate math + budget remaining over complianceWindow
- [ ] Implement multi-window burn rule evaluation (short+long >= threshold)
- [ ] Implement gating modifiers:
  - [ ] stale data -> WARN
  - [ ] insufficient data (total==0) -> WARN
- [ ] Implement synthetic metrics adapter:
  - [ ] load windowed values from JSON fixtures
  - [ ] support returning good/total per window
- [ ] Scenario tests: healthy, fast-burn, slow-burn, stale, zero-traffic

### Acceptance criteria
- `aegis eval --adapter synthetic` produces expected decisions for all scenarios
- Tests prove rule triggering is correct and deterministic
- No Prometheus code yet

---

## Milestone 3 — Prometheus adapter (PR-6..PR-7)
### Tasks
- [ ] Implement Prometheus HTTP client + query execution
- [ ] Window templating: replace {{window}} in queries
- [ ] Concurrency limit (e.g., semaphore max 10)
- [ ] Retry policy (simple: 1 retry) + timeout per query
- [ ] Adapter-level tests using mocked HTTP server

### Acceptance criteria
- Prometheus adapter returns identical results to synthetic fixtures when mocked
- Query failures yield WARN (not ALLOW)

---

## Milestone 4 — API server + scheduler (PR-8..PR-10)
### Tasks
- [ ] Implement scheduler loop per SLO evaluationInterval
- [ ] Store latest evaluation in memory cache
- [ ] Implement endpoints:
  - [ ] /healthz, /readyz
  - [ ] /v1/slo, /v1/slo/{id}
  - [ ] /v1/state/{service}/{env}
  - [ ] POST /v1/gate/decision (read cached; optional force_fresh)
- [ ] Implement evidence payload format per PED
- [ ] Contract tests for API responses

### Acceptance criteria
- Running server evaluates SLOs on schedule and serves decisions
- Decision responses include reasons + evidence windows
- Readyz fails if no valid SLOs loaded or adapter unavailable

---

## Milestone 5 — SQLite audit + docker compose demo (PR-11..PR-12)
### Tasks
- [ ] Implement SQLite storage + migrations
- [ ] Persist slo_definitions, evaluations, latest_state
- [ ] Add /v1/audit endpoint with filters
- [ ] Add docker-compose demo (server + prometheus optional)
- [ ] Add docs: DESIGN, RUNBOOK, API usage

### Acceptance criteria
- Decisions are persisted and queryable via /v1/audit
- Demo environment reproducible from README instructions

---

## Release v0.1.0
### Tasks
- [ ] Tag release v0.1.0
- [ ] Ensure docs complete and examples exist
- [ ] Add “Known limitations” section
