# AegisSLO — Product Engineering Document (PED)

## 1. Overview

### 1.1 Product name

**AegisSLO**  
*SLO-driven release gate and error-budget control plane*

### 1.2 One-sentence purpose

AegisSLO is a control-plane service that continuously evaluates Service Level Objectives (SLOs), computes error-budget burn rates, and returns deterministic deployment gate decisions (ALLOW / WARN / BLOCK) for CI/CD pipelines.

### 1.3 Problem statement

Teams often deploy changes without an objective view of service reliability. When error budgets are already being consumed rapidly, further changes increase risk and can trigger cascading failures.

AegisSLO provides:

- Quantitative SLO evaluation
- Error-budget accounting
- Multi-window burn-rate analysis
- Automated deployment gating

This system becomes the single source of truth for release safety.

---

## 2. Goals and non-goals

### 2.1 Goals

- Parse and validate SLO definitions from YAML
- Evaluate SLIs using Prometheus or synthetic metrics
- Compute error-budget burn rates
- Apply multi-window burn policies
- Provide deployment gate decisions via HTTP API
- Maintain an audit log of decisions
- Provide CLI tools for validation and simulation

### 2.2 Non-goals

- Enterprise authentication/authorization
- GUI dashboards
- Pager/on-call integrations
- Multi-region distributed storage
- Full incident management tooling

---

## 3. Users

- **SRE / Platform engineers** — define and govern SLOs
- **Service teams** — integrate deployment gating
- **Oncall engineers** — inspect decision reasoning

---

## 4. Core concepts

### 4.1 SLI (Service Level Indicator)

A measurable signal of service behavior:

- Availability: good requests / total requests
- Latency compliance: fast requests / total requests
- Correctness: successful operations / total operations

### 4.2 SLO (Service Level Objective)

A reliability target over a window:

Example: 99.9% availability over 30 days.

### 4.3 Error budget

```
error_budget = 1 - objective
```

99.9% objective → 0.1% allowable error.

### 4.4 Burn rate

```
error_rate = 1 - SLI
burn_rate = error_rate / error_budget
```

Burn rate expresses how fast reliability budget is being consumed relative to time.

### 4.5 Multi-window burn analysis

Evaluate burn rate across short and long windows to detect sustained degradation.

---

## 5. System architecture

### 5.1 Major components

- SLO loader & validator
- Evaluation engine
- Metrics adapter (Prometheus + synthetic)
- Policy engine
- API server
- Audit storage
- CLI tooling

### 5.2 Evaluation flow

1. Load SLO spec
2. Query metrics for evaluation windows
3. Compute SLIs
4. Compute burn rates
5. Apply burn policies
6. Store evaluation result
7. Serve gate decision

---

## 6. Functional requirements

### 6.1 SLO spec loading

- Load YAML SLO files from configured directory
- Validate against JSON schema
- Reject invalid specs without crashing system
- Reload periodically or on signal

---

### 6.2 Supported SLI types (v1)

#### Ratio SLI

```
SLI = good / total
```

Used for availability/correctness.

#### Latency threshold SLI

```
SLI = fast_requests / total
```

Requests below threshold count as good.

---

### 6.3 Metrics adapters

#### Prometheus adapter

- Query Prometheus HTTP API
- Support window templating
- Handle query failures gracefully

#### Synthetic adapter

- Fixture-driven simulated metrics
- Deterministic testing support

---

### 6.4 Burn policy engine

For each rule:

```
if burn_short >= threshold AND burn_long >= threshold
    trigger rule
```

Actions:

- ALLOW
- WARN
- BLOCK

Aggregation:

- Any BLOCK → BLOCK
- Else any WARN → WARN
- Else ALLOW

---

### 6.5 Audit logging

Store:

- timestamp
- SLO ID
- windows evaluated
- SLI values
- burn rates
- decision
- reasoning

---

### 6.6 HTTP API

Required endpoints:

```
GET  /healthz
GET  /readyz
GET  /v1/slo
GET  /v1/slo/{id}
GET  /v1/state/{service}/{env}
POST /v1/gate/decision
GET  /v1/audit
```

Decision response includes:

- decision
- TTL
- reasons
- evidence snapshot

---

### 6.7 CLI

Commands:

```
aegis validate
aegis eval
aegis replay
aegis explain
```

---

## 7. SLO specification format

SLOs are defined in YAML validated against `schemas/slo_v1.json`.

Fields include:

- metadata
- objective
- windows
- SLI query definitions
- burn policies
- gating rules

Template variables:

```
{{window}}
```

used in Prometheus queries.

---

## 8. Reliability math rules

For each window:

```
E = max(0, 1 - SLI)
B = 1 - objective
burn_rate = E / B
```

If total requests = 0:

- mark insufficient data
- decision escalates to WARN

Compliance window evaluates budget remaining:

```
remaining_budget = 1 - consumed_fraction
```

---

## 9. Decision algorithm

For each SLO:

1. Evaluate all windows
2. Compute burn rates
3. Apply burn rules
4. Apply gating modifiers:
   - stale data → WARN
   - insufficient data → WARN

Aggregate across SLOs:

- Any BLOCK → BLOCK
- Else any WARN → WARN
- Else ALLOW

---

## 10. Data storage

SQLite tables:

- slo_definitions
- evaluations
- latest_state

Store structured evidence JSON for reproducibility.

---

## 11. Observability of AegisSLO

Expose Prometheus metrics:

- evaluation latency
- decision counters
- adapter errors
- SLO load failures

Structured logs include correlation IDs.

---

## 12. Performance constraints

- Support ~50 SLOs
- Evaluation interval ≥ 30s
- Prometheus query concurrency capped
- Cache window results per evaluation cycle

---

## 13. Security model (minimal)

Optional shared-token header for decision endpoints.

---

## 14. Failure handling

- Prometheus unavailable → WARN decisions
- Partial data → WARN
- Never ALLOW when data is stale or unknown

---

## 15. Test strategy

### Unit tests

- schema validation
- burn-rate math
- rule evaluation

### Integration tests

- synthetic adapter flows
- API contract validation
- persistence checks

### Scenario tests

- healthy system
- fast burn
- slow burn
- stale data
- zero traffic

---

## 16. Repository structure

```
cmd/
  aegis-server/
  aegis-cli/

internal/
  slo/
  eval/
  policy/
  api/
  prometheus/
  storage/
  logging/
  config/

schemas/
fixtures/
docs/
deployments/
```

---

## 17. Implementation language

Primary implementation: **Go**

Rationale:

- concurrency model
- production tooling ecosystem
- SRE alignment

---

## 18. Milestones

### Milestone 1

Schema + YAML validation engine.

### Milestone 2

SLI math + burn-rate evaluation + synthetic adapter.

### Milestone 3

Prometheus adapter.

### Milestone 4

API server + gating logic.

### Milestone 5

SQLite audit + persistence.

---

## 19. Definition of done

System is complete when:

- SLOs validate correctly
- Burn policies trigger deterministically
- API returns correct decisions
- Synthetic scenarios reproduce expected outcomes
- Tests pass reliably
- Audit evidence is stored and queryable

---

## 20. Engineering principles

- Deterministic decision logic
- Explicit failure handling
- Observable behavior
- Testable components
- Spec-driven implementation
- No hidden heuristics

---

End of PED.
