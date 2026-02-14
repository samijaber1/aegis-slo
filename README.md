# AegisSLO

**SLO-driven release gate and error-budget control plane for reliability-aware deployments.**

AegisSLO is a lightweight, opinionated SLO evaluation engine that provides automated gate decisions based on multi-window burn rate analysis. It helps teams make data-driven deployment decisions by answering: *"Is it safe to deploy right now?"*

## Features

- **📊 Multi-Window Burn Rate Analysis**: Evaluate SLI across multiple time windows to detect both fast and slow burns
- **🎯 Policy-Based Gating**: Configurable burn rate policies with short/long window thresholds
- **🔄 Dual Adapter Support**: Works with Prometheus or synthetic metrics for testing
- **📝 Audit Logging**: SQLite-based audit trail of all evaluation decisions
- **🐳 Docker Ready**: Production-ready Docker image with health checks
- **🔌 HTTP API**: RESTful API for gate decisions, SLO listing, and audit queries

## Quick Start

### Using Docker Compose

```bash
# Clone the repository
git clone https://github.com/samijaber1/aegis-slo.git
cd aegis-slo

# Start the server with synthetic adapter
docker-compose up

# Check health
curl http://localhost:8080/healthz

# Get gate decision
curl -X POST http://localhost:8080/v1/gate/decision \
  -H "Content-Type: application/json" \
  -d '{"sloID": "api-availability"}'
```

### Building from Source

**Requirements:**
- Go 1.23 or later
- CGO enabled (for SQLite support)
- GCC (for SQLite compilation)

```bash
# Build the server
CGO_ENABLED=1 go build -o aegis-server ./cmd/aegis-server

# Build the CLI validator
go build -o aegis ./cmd/aegis-cli

# Validate SLO definitions
./aegis validate --dir ./fixtures/slo/valid

# Run the server
./aegis-server \
  --slo-dir ./fixtures/slo/valid \
  --adapter synthetic \
  --db aegis.db \
  --port 8080
```

## SLO Definition

SLOs are defined in YAML following the [schema](schemas/slo_v1.json):

```yaml
apiVersion: aegis.slo/v1
kind: SLO
metadata:
  id: api-availability
  service: api-gateway
  owner: platform-team

spec:
  environment: production
  objective: 0.995  # 99.5% availability
  complianceWindow: 30d
  evaluationInterval: 5m

  sli:
    type: availability
    good:
      prometheusQuery: |
        sum(rate(http_requests_total{status!~"5.."}[{{window}}]))
    total:
      prometheusQuery: |
        sum(rate(http_requests_total[{{window}}]))

  burnPolicy:
    rules:
      - name: fast-burn
        shortWindow: 5m
        longWindow: 1h
        threshold: 14.0

      - name: slow-burn
        shortWindow: 1h
        longWindow: 6h
        threshold: 7.0

  gating:
    minDataPoints: 10
    stalenessLimit: 10m
```

## HTTP API

### Gate Decision

Get an ALLOW/BLOCK/WARN decision for a deployment gate:

```bash
curl -X POST http://localhost:8080/v1/gate/decision \
  -H "Content-Type: application/json" \
  -d '{
    "sloID": "api-availability",
    "forceFresh": false
  }'
```

**Response:**
```json
{
  "decision": "ALLOW",
  "sloID": "api-availability",
  "timestamp": "2024-01-15T10:30:00Z",
  "ttl": 300,
  "sli": {
    "value": 0.9995,
    "errorRate": 0.0005,
    "budgetRemaining": 0.9
  },
  "reasons": ["all burn rate checks passed"],
  "burnRates": {
    "5m": {"burnRate": 1.0, "threshold": 14.0},
    "1h": {"burnRate": 1.0, "threshold": 14.0},
    "6h": {"burnRate": 1.0, "threshold": 7.0}
  },
  "isStale": false,
  "hasNoTraffic": false
}
```

**Decisions:**
- `ALLOW`: All burn rate checks pass, safe to deploy
- `BLOCK`: One or more burn rate thresholds exceeded
- `WARN`: Stale data or insufficient traffic (non-blocking)

### List SLOs

```bash
curl http://localhost:8080/v1/slo
```

### Get SLO Details

```bash
curl http://localhost:8080/v1/slo/api-availability
```

### Query Audit Log

```bash
# Get recent evaluations
curl "http://localhost:8080/v1/audit?limit=10"

# Filter by SLO
curl "http://localhost:8080/v1/audit?sloID=api-availability&limit=20"

# Filter by decision
curl "http://localhost:8080/v1/audit?decision=BLOCK&limit=10"

# Time range query
curl "http://localhost:8080/v1/audit?startTime=2024-01-15T00:00:00Z&endTime=2024-01-15T23:59:59Z"
```

### Service State

```bash
curl http://localhost:8080/v1/state/api-gateway/production
```

### Health & Readiness

```bash
curl http://localhost:8080/healthz
curl http://localhost:8080/readyz
```

## Configuration

### Command-Line Flags

| Flag | Default | Description |
|------|---------|-------------|
| `--port` | `8080` | HTTP server port |
| `--host` | `0.0.0.0` | HTTP server host |
| `--slo-dir` | - | Directory containing SLO YAML files (required) |
| `--adapter` | `synthetic` | Metrics adapter: `prometheus` or `synthetic` |
| `--prometheus-url` | - | Prometheus server URL (required if adapter=prometheus) |
| `--synthetic-fixtures` | - | Directory with synthetic metric fixtures |
| `--db` | `aegis.db` | SQLite database file for audit logging |

### Environment Variables

Configuration can also be set via environment variables:
- `PORT`
- `HOST`
- `SLO_DIR`
- `ADAPTER_TYPE`
- `PROMETHEUS_URL`
- `DB_PATH`

## Architecture

```
┌─────────────────────────────────────────────────────┐
│                  HTTP API Server                     │
│  /v1/gate/decision  /v1/audit  /v1/slo  /healthz    │
└─────────────────┬───────────────────────────────────┘
                  │
         ┌────────▼────────┐
         │   Scheduler     │  Per-SLO evaluation loops
         │                 │  (interval: 5m, 1m, etc.)
         └────┬───────┬────┘
              │       │
      ┌───────▼─┐   ┌▼────────────┐
      │ Policy  │   │  Evaluator  │
      │ Engine  │   │             │
      └─────────┘   └──┬──────────┘
                       │
            ┌──────────▼──────────┐
            │  Metrics Adapter    │
            ├─────────────────────┤
            │ Prometheus          │
            │ Synthetic (testing) │
            └─────────────────────┘
```

## Burn Rate Math

AegisSLO uses Google SRE's multi-window burn rate approach:

**Error Rate:**
```
error_rate = (total - good) / total
```

**Burn Rate:**
```
burn_rate = error_rate / error_budget
where error_budget = 1 - objective
```

**Example:**
- Objective: 99.5% (error budget: 0.5%)
- Current error rate: 7% over 5m window
- Burn rate: 7% / 0.5% = **14x**

A burn rate of 14x means you're consuming error budget 14× faster than allowed over the compliance window (30d). At this rate, you'd exhaust your monthly budget in ~2 days.

## Known Limitations

### v0.1.0 Limitations

1. **Single Adapter Instance**
   - Only one metrics adapter (Prometheus or synthetic) per server instance
   - Cannot query multiple Prometheus instances
   - *Workaround*: Run multiple AegisSLO instances with different adapters

2. **No Persistent SLO Storage**
   - SLO definitions must be loaded from YAML files on startup
   - Changes require server restart
   - *Planned*: Hot-reload via file watcher (v0.2.0)

3. **Limited Staleness Detection**
   - Staleness only checks metric timestamp age
   - Does not verify metric collection continuity
   - *Improvement*: Gap detection in time series (future)

4. **No Built-in Alerting**
   - Server only provides API responses
   - Does not send alerts on BLOCK decisions
   - *Workaround*: Poll `/v1/audit` or `/v1/state` from external alerting system

5. **SQLite Limitations**
   - Single-writer limitation (not suitable for multi-replica deployments)
   - Audit database can grow unbounded
   - *Planned*: Audit retention policies and PostgreSQL adapter (v0.2.0+)

6. **No Query Optimization**
   - Prometheus queries run serially per SLO
   - No query result caching beyond evaluation TTL
   - *Improvement*: Parallel queries with rate limiting (future)

7. **Basic Authentication Only**
   - No built-in API authentication or authorization
   - Relies on network-level security
   - *Planned*: API key support (v0.2.0)

8. **Windows Build Challenges**
   - CGO requirement makes Windows builds complex
   - Requires MinGW or similar C compiler
   - *Recommendation*: Use Docker on Windows

### Production Considerations

- **High Availability**: Run multiple read-only replicas behind a load balancer; use shared database or accept eventual consistency
- **Metric Freshness**: Configure `stalenessLimit` appropriate to your Prometheus scrape interval + query latency
- **Burn Rate Tuning**: Start conservative (high thresholds), tune based on false positive/negative rates
- **Database Backups**: Regular backups of SQLite database for audit trail preservation

## Development

### Running Tests

```bash
# All tests (requires CGO for SQLite tests)
CGO_ENABLED=1 go test ./...

# Without SQLite tests
go test $(go list ./... | grep -v storage/sqlite)

# Specific package
go test ./internal/policy -v
```

### Project Structure

```
aegis-slo/
├── cmd/
│   ├── aegis-cli/        # SLO validator CLI
│   └── aegis-server/     # HTTP API server
├── internal/
│   ├── adapter/          # Metrics adapters
│   │   ├── prometheus/   # Prometheus HTTP client
│   │   └── synthetic/    # Fixture-based testing
│   ├── api/              # HTTP handlers
│   ├── config/           # Configuration
│   ├── eval/             # SLI/burn rate evaluation
│   ├── policy/           # Policy engine (ALLOW/BLOCK/WARN)
│   ├── scheduler/        # Periodic evaluation loops
│   ├── slo/              # SLO loading & validation
│   └── storage/          # Audit persistence
│       └── sqlite/       # SQLite implementation
├── schemas/              # JSON schemas
├── fixtures/             # Test data
│   ├── slo/              # Example SLO definitions
│   └── metrics/          # Synthetic metric fixtures
└── docs/                 # Design docs

```

## Contributing

See [docs/TASKS.md](docs/TASKS.md) for the development roadmap and milestone plan.

## License

MIT License - see LICENSE file for details.

## Credits

Built with inspiration from:
- [Google SRE Book - Chapter 5: Eliminating Toil](https://sre.google/sre-book/eliminating-toil/)
- [Google SRE Workbook - Chapter 2: Implementing SLOs](https://sre.google/workbook/implementing-slos/)
- [Sloth - SLO generator](https://github.com/slok/sloth)
