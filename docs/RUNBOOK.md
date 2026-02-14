# AegisSLO Runbook

Operational guide for deploying, monitoring, and troubleshooting AegisSLO in production.

## Table of Contents

- [Deployment](#deployment)
- [Monitoring](#monitoring)
- [Troubleshooting](#troubleshooting)
- [Maintenance](#maintenance)
- [Disaster Recovery](#disaster-recovery)

## Deployment

### Prerequisites

- **Metrics Backend**: Prometheus server with historical data (>=30d for monthly SLOs)
- **Infrastructure**: Docker + Docker Compose or Kubernetes
- **Database**: Persistent volume for SQLite (or shared filesystem for replicas)
- **Network**: Connectivity to Prometheus API

### Production Deployment (Docker Compose)

1. **Create Production SLO Definitions**

```bash
mkdir -p /opt/aegis-slo/slos
# Add your SLO YAML files to /opt/aegis-slo/slos/
```

2. **Configure docker-compose.yml**

```yaml
version: '3.8'

services:
  aegis-slo:
    image: aegis-slo:v0.1.0
    restart: unless-stopped
    ports:
      - "8080:8080"
    environment:
      - TZ=UTC
    volumes:
      - /opt/aegis-slo/slos:/app/slos:ro
      - aegis-data:/data
    command: >
      /app/aegis-server
      --port=8080
      --slo-dir=/app/slos
      --adapter=prometheus
      --prometheus-url=http://prometheus:9090
      --db=/data/aegis.db
    healthcheck:
      test: ["CMD", "wget", "--quiet", "--tries=1", "--spider", "http://localhost:8080/healthz"]
      interval: 30s
      timeout: 5s
      retries: 3
      start_period: 10s
    logging:
      driver: "json-file"
      options:
        max-size: "10m"
        max-file: "3"

volumes:
  aegis-data:
    driver: local
```

3. **Deploy**

```bash
docker-compose up -d
docker-compose logs -f aegis-slo
```

4. **Verify**

```bash
# Check health
curl http://localhost:8080/healthz

# Check readiness
curl http://localhost:8080/readyz

# List loaded SLOs
curl http://localhost:8080/v1/slo
```

### Kubernetes Deployment

See `examples/kubernetes/` for manifests including:
- Deployment with resource limits
- ConfigMap for SLO definitions
- PersistentVolumeClaim for database
- Service and Ingress
- HorizontalPodAutoscaler (read replicas)

### High Availability Setup

**Option 1: Active-Passive** (Recommended for v0.1.0)
- Single writer instance
- Passive replica with read-only database mount
- Automatic failover via Kubernetes or Docker Swarm

**Option 2: Active-Active** (Requires shared storage)
- Multiple instances sharing SQLite database over NFS/GlusterFS
- **Warning**: SQLite locking may cause contention
- Consider PostgreSQL adapter (planned v0.2.0)

**Load Balancer Configuration:**
```nginx
upstream aegis_slo {
    server aegis-1:8080;
    server aegis-2:8080 backup;  # Passive replica
}

server {
    listen 80;

    location / {
        proxy_pass http://aegis_slo;
        proxy_http_version 1.1;
        proxy_set_header Connection "";

        # Health check bypass
        proxy_next_upstream error timeout http_503;
    }
}
```

## Monitoring

### Key Metrics to Track

1. **Service Health**
   - Endpoint: `GET /healthz`
   - Frequency: Every 10s
   - Alert: Consecutive failures (>3)

2. **Service Readiness**
   - Endpoint: `GET /readyz`
   - Check: `ready: true`, `slosLoaded > 0`
   - Alert: Not ready for >5min

3. **Gate Decision Distribution**
   - Query: `SELECT decision, COUNT(*) FROM evaluations GROUP BY decision`
   - Track: ALLOW/BLOCK/WARN ratio over time
   - Alert: BLOCK rate >10% for >15min

4. **Evaluation Latency**
   - Parse server logs: "Evaluated SLO" lines
   - Track: Time between evaluations
   - Alert: Evaluation delayed >2x interval

5. **Database Size**
   - Monitor: SQLite database file size
   - Alert: Growth >1GB/day (indicates retention issue)

### Prometheus Metrics Export

AegisSLO does not yet export Prometheus metrics. Use log parsing or query the audit database:

```bash
# Example: Count decisions in last hour
sqlite3 aegis.db <<EOF
SELECT decision, COUNT(*) as count
FROM evaluations
WHERE created_at > datetime('now', '-1 hour')
GROUP BY decision;
EOF
```

### Log Monitoring

**Critical Log Patterns:**

```bash
# Evaluation errors
grep "Error evaluating SLO" /var/log/aegis-slo.log

# Query failures
grep "query failed" /var/log/aegis-slo.log

# Database errors
grep "failed to store" /var/log/aegis-slo.log
```

**Alert Conditions:**
- `Error evaluating SLO` for same SLO >3 times in 5min → Prometheus query issue
- `query failed after N attempts` → Prometheus unreachable
- `failed to store evaluation` → Database corruption or disk full

### Sample Grafana Dashboard

```json
{
  "panels": [
    {
      "title": "Gate Decisions (Last 24h)",
      "query": "SELECT decision, COUNT(*) FROM evaluations WHERE created_at > datetime('now', '-1 day') GROUP BY decision"
    },
    {
      "title": "SLO Evaluation Frequency",
      "query": "SELECT slo_id, COUNT(*) FROM evaluations WHERE created_at > datetime('now', '-1 hour') GROUP BY slo_id"
    },
    {
      "title": "Blocked SLOs",
      "query": "SELECT slo_id, service, environment FROM latest_state WHERE decision = 'BLOCK'"
    }
  ]
}
```

## Troubleshooting

### Common Issues

#### 1. "No SLOs loaded" / Readiness Fails

**Symptoms:**
- `GET /readyz` returns `503`
- `slosLoaded: 0`
- Reason: "no SLOs loaded"

**Diagnosis:**
```bash
docker exec -it aegis-slo ls -la /app/slos
docker logs aegis-slo | grep "Failed to load SLOs"
```

**Common Causes:**
- SLO directory empty or incorrect path
- Invalid YAML syntax in SLO files
- Schema validation failures
- File permissions (container user cannot read)

**Resolution:**
```bash
# Validate SLO files locally
docker run --rm -v $(pwd)/slos:/slos aegis-slo:v0.1.0 \
  /app/aegis validate --dir /slos

# Check container filesystem
docker exec aegis-slo cat /app/slos/my-slo.yaml

# Restart after fixing
docker-compose restart aegis-slo
```

#### 2. "Query failed" / Prometheus Adapter Errors

**Symptoms:**
- Log: `Error evaluating SLO X: query good metrics (window=5m): query failed after 1 attempts`
- Decision: WARN (not BLOCK)

**Diagnosis:**
```bash
# Test Prometheus connectivity from container
docker exec aegis-slo wget -O- http://prometheus:9090/api/v1/query?query=up

# Check Prometheus logs
docker logs prometheus | grep ERROR
```

**Common Causes:**
- Prometheus URL incorrect or unreachable
- Prometheus under heavy load (timeout)
- Query syntax error (check `{{window}}` substitution)
- Missing metrics (series not scraped)

**Resolution:**
```bash
# Test query manually
curl -G http://prometheus:9090/api/v1/query \
  --data-urlencode 'query=sum(rate(http_requests_total[5m]))'

# Increase timeout (future: add --prometheus-timeout flag)
# For now, check Prometheus performance

# Verify metric exists
curl -G http://prometheus:9090/api/v1/series \
  --data-urlencode 'match[]=http_requests_total'
```

#### 3. Database Locked / Write Failures

**Symptoms:**
- Log: `failed to store evaluation: database is locked`
- Audit endpoint returns old data

**Diagnosis:**
```bash
# Check for multiple writers
ps aux | grep aegis-server
lsof /data/aegis.db

# Check database integrity
sqlite3 /data/aegis.db "PRAGMA integrity_check;"
```

**Resolution:**
```bash
# Ensure single writer
docker-compose scale aegis-slo=1

# If corrupted, restore from backup
cp /backup/aegis.db.backup /data/aegis.db
docker-compose restart aegis-slo
```

#### 4. Stale Data Warnings

**Symptoms:**
- Decision: WARN
- `isStale: true`
- Reason: "data is stale"

**Diagnosis:**
Check SLO's `stalenessLimit` vs Prometheus scrape interval:

```bash
# Get last metric timestamp
curl -G http://prometheus:9090/api/v1/query \
  --data-urlencode 'query=http_requests_total' \
  | jq '.data.result[0].value[0]'

# Compare to current time
date +%s
```

**Resolution:**
- Increase `stalenessLimit` in SLO spec if scrape interval is long
- Fix Prometheus scraping if metrics genuinely stale
- Check network latency between AegisSLO and Prometheus

#### 5. Zero Traffic / Insufficient Data

**Symptoms:**
- Decision: WARN
- `hasNoTraffic: true`
- `sli.value: 0` (no errors, but also no requests)

**Diagnosis:**
```bash
# Check if metric has data
curl -G http://prometheus:9090/api/v1/query \
  --data-urlencode 'query=sum(rate(http_requests_total[5m]))' \
  | jq '.data.result[0].value[1]'
```

**Resolution:**
- Verify application is receiving traffic
- Check metric label filters (status codes, paths, etc.)
- Confirm Prometheus is scraping target

## Maintenance

### Routine Tasks

#### Daily
- [ ] Check `/readyz` and `/healthz` endpoints
- [ ] Review last 24h gate decisions (`/v1/audit?limit=100`)
- [ ] Monitor database size growth

#### Weekly
- [ ] Review BLOCK decisions for false positives
- [ ] Validate SLO configurations against production traffic patterns
- [ ] Check for evaluation lag (logs: time between "Evaluated SLO" messages)

#### Monthly
- [ ] Audit database cleanup (see Audit Retention below)
- [ ] Review and tune burn rate thresholds
- [ ] Update SLO objectives based on actual reliability

### SLO Updates

**To add/modify SLOs:**

1. Update YAML files in SLO directory
2. Validate locally:
   ```bash
   aegis validate --dir ./slos
   ```
3. Restart server:
   ```bash
   docker-compose restart aegis-slo
   ```
4. Verify SLO loaded:
   ```bash
   curl http://localhost:8080/v1/slo/new-slo-id
   ```

**Note:** v0.1.0 requires restart. Hot-reload planned for v0.2.0.

### Audit Retention

**Manual Cleanup:**

```sql
-- Delete evaluations older than 90 days
DELETE FROM evaluations
WHERE created_at < datetime('now', '-90 days');

-- Vacuum to reclaim space
VACUUM;
```

**Automated Cleanup (cron):**

```bash
# /etc/cron.daily/aegis-audit-cleanup
#!/bin/bash
sqlite3 /data/aegis.db <<EOF
DELETE FROM evaluations WHERE created_at < datetime('now', '-90 days');
VACUUM;
EOF
echo "Cleaned up evaluations older than 90 days"
```

### Database Backups

**Daily Backup Script:**

```bash
#!/bin/bash
DATE=$(date +%Y%m%d)
BACKUP_DIR=/backup/aegis-slo
mkdir -p $BACKUP_DIR

# Backup database (using .backup for consistency)
sqlite3 /data/aegis.db ".backup $BACKUP_DIR/aegis-$DATE.db"

# Keep last 30 days
find $BACKUP_DIR -name "aegis-*.db" -mtime +30 -delete

echo "Backup completed: aegis-$DATE.db"
```

**Restore from Backup:**

```bash
# Stop server
docker-compose stop aegis-slo

# Restore database
cp /backup/aegis-slo/aegis-20240115.db /data/aegis.db

# Start server
docker-compose start aegis-slo
```

## Disaster Recovery

### Scenario 1: Complete Database Loss

**Impact:** Loss of audit trail, but SLO evaluations continue from cache.

**Recovery:**
1. Server continues operating (evaluations from Prometheus)
2. Deploy with empty database:
   ```bash
   rm /data/aegis.db
   docker-compose restart aegis-slo
   ```
3. New audit trail starts accumulating
4. Historical decisions lost (consider Prometheus query logs for forensics)

**Prevention:** Automated daily backups to external storage.

### Scenario 2: Prometheus Unavailable

**Impact:** All gate decisions return WARN (query failures).

**Immediate Action:**
- Evaluations fail → decisions = WARN
- Deployments should treat WARN as blocking (configure in CI/CD)
- Restore Prometheus or point to backup instance

**Temporary Workaround:**
```bash
# Switch to synthetic adapter for testing
docker-compose stop aegis-slo
# Edit docker-compose.yml: --adapter=synthetic
docker-compose up -d aegis-slo
```

### Scenario 3: Incorrect SLO Configuration

**Impact:** False BLOCK decisions, blocking legitimate deployments.

**Emergency Rollback:**
```bash
# Revert SLO files
git checkout HEAD~1 slos/

# Restart
docker-compose restart aegis-slo

# Verify
curl http://localhost:8080/v1/slo
```

**Hotfix:**
- Temporarily increase burn rate thresholds
- Update affected SLO's `threshold` values
- Restart and monitor

### Scenario 4: SLO Evaluation Stuck

**Symptoms:**
- Stale `timestamp` in `/v1/gate/decision` responses
- No recent entries in `evaluations` table

**Diagnosis:**
```bash
# Check scheduler goroutines
docker logs aegis-slo | grep "Evaluated SLO"

# Check for panics
docker logs aegis-slo | grep "panic"
```

**Resolution:**
```bash
# Restart scheduler
docker-compose restart aegis-slo

# If persists, check resource limits (CPU/memory)
docker stats aegis-slo
```

## Appendix

### Useful SQL Queries

```sql
-- Top 10 most frequently blocked SLOs
SELECT slo_id, COUNT(*) as blocks
FROM evaluations
WHERE decision = 'BLOCK' AND created_at > datetime('now', '-7 days')
GROUP BY slo_id
ORDER BY blocks DESC
LIMIT 10;

-- SLO reliability over time (hourly buckets)
SELECT
    strftime('%Y-%m-%d %H:00', timestamp) as hour,
    AVG(sli) as avg_sli,
    AVG(budget_remaining) as avg_budget
FROM evaluations
WHERE slo_id = 'api-availability'
    AND timestamp > datetime('now', '-24 hours')
GROUP BY hour
ORDER BY hour;

-- Audit trail for specific deployment decision
SELECT *
FROM evaluations
WHERE slo_id = 'api-availability'
    AND timestamp BETWEEN '2024-01-15 10:00:00' AND '2024-01-15 10:05:00'
ORDER BY timestamp DESC;
```

### Configuration Checklist

- [ ] SLO YAML files validated (`aegis validate`)
- [ ] Prometheus URL reachable from container network
- [ ] Database volume persistent and backed up
- [ ] Health check endpoints accessible
- [ ] Logging configured (log rotation, retention)
- [ ] Monitoring alerts configured
- [ ] Resource limits set (CPU, memory)
- [ ] TLS/HTTPS configured (if exposed publicly)
- [ ] Authentication/authorization via reverse proxy
- [ ] Burn rate thresholds tuned for your traffic patterns

### Support

- **Documentation**: [README.md](../README.md), [PED.md](./PED.md)
- **Issues**: https://github.com/samijaber1/aegis-slo/issues
- **Logs**: Check Docker logs for detailed error messages
- **Debug Mode**: Set `LOG_LEVEL=debug` environment variable (future)

---

**Last Updated:** v0.1.0 (2024-01-15)
