#!/bin/bash
# AegisSLO API Usage Examples
# Run these commands against a running AegisSLO server

BASE_URL="${AEGIS_URL:-http://localhost:8080}"

echo "=== AegisSLO API Examples ==="
echo "Base URL: $BASE_URL"
echo ""

# 1. Health Check
echo "1. Health Check"
curl -s "$BASE_URL/healthz" | jq '.'
echo ""

# 2. Readiness Check
echo "2. Readiness Check"
curl -s "$BASE_URL/readyz" | jq '.'
echo ""

# 3. List All SLOs
echo "3. List All SLOs"
curl -s "$BASE_URL/v1/slo" | jq '.'
echo ""

# 4. Get Specific SLO Details
echo "4. Get SLO Details (replace 'test-slo' with actual ID)"
SLO_ID="test-slo"
curl -s "$BASE_URL/v1/slo/$SLO_ID" | jq '.'
echo ""

# 5. Gate Decision (Primary Use Case)
echo "5. Gate Decision - ALLOW/BLOCK/WARN"
curl -s -X POST "$BASE_URL/v1/gate/decision" \
  -H "Content-Type: application/json" \
  -d "{\"sloID\": \"$SLO_ID\", \"forceFresh\": false}" | jq '.'
echo ""

# 6. Force Fresh Evaluation (bypass cache)
echo "6. Force Fresh Evaluation"
curl -s -X POST "$BASE_URL/v1/gate/decision" \
  -H "Content-Type: application/json" \
  -d "{\"sloID\": \"$SLO_ID\", \"forceFresh\": true}" | jq '.'
echo ""

# 7. Query Audit Log - Recent Evaluations
echo "7. Audit Log - Last 10 Evaluations"
curl -s "$BASE_URL/v1/audit?limit=10" | jq '.records[] | {id, sloID, decision, timestamp}'
echo ""

# 8. Filter Audit by SLO ID
echo "8. Audit Log - Specific SLO"
curl -s "$BASE_URL/v1/audit?sloID=$SLO_ID&limit=5" | jq '.records[] | {decision, sli, timestamp}'
echo ""

# 9. Filter Audit by Decision (find all BLOCKs)
echo "9. Audit Log - BLOCK Decisions"
curl -s "$BASE_URL/v1/audit?decision=BLOCK&limit=5" | jq '.records[] | {sloID, reasons, timestamp}'
echo ""

# 10. Time Range Query
echo "10. Audit Log - Last Hour"
START_TIME=$(date -u -d '1 hour ago' +"%Y-%m-%dT%H:%M:%SZ" 2>/dev/null || date -u -v-1H +"%Y-%m-%dT%H:%M:%SZ")
END_TIME=$(date -u +"%Y-%m-%dT%H:%M:%SZ")
curl -s "$BASE_URL/v1/audit?startTime=$START_TIME&endTime=$END_TIME&limit=20" | jq '.records | length'
echo ""

# 11. Service State (all SLOs for a service/environment)
echo "11. Service State"
curl -s "$BASE_URL/v1/state/test-service/production" | jq '.'
echo ""

# 12. Parse Decision for CI/CD Pipeline
echo "12. CI/CD Integration - Extract Decision"
DECISION=$(curl -s -X POST "$BASE_URL/v1/gate/decision" \
  -H "Content-Type: application/json" \
  -d "{\"sloID\": \"$SLO_ID\"}" | jq -r '.decision')

echo "Decision: $DECISION"

if [ "$DECISION" = "ALLOW" ]; then
    echo "✅ Gate PASSED - Safe to deploy"
    exit 0
elif [ "$DECISION" = "WARN" ]; then
    echo "⚠️  Gate WARNING - Proceed with caution (stale data or no traffic)"
    exit 0  # or exit 1 if you want to block on warnings
elif [ "$DECISION" = "BLOCK" ]; then
    echo "❌ Gate BLOCKED - Do NOT deploy"
    exit 1
else
    echo "❓ Unknown decision: $DECISION"
    exit 2
fi
