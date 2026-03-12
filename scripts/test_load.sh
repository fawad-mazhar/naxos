#!/bin/bash
# scripts/test_load.sh
# Load test: submits 150 jobs to the API and tracks execution

set -o pipefail

API_URL="${1:-http://localhost:8080}"
NUM_JOBS="${2:-150}"
JOB_DEF_ID="sample-1"

GREEN='\033[0;32m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
NC='\033[0m'

echo -e "${GREEN}=== Naxos NATS JetStream Load Test ===${NC}"
echo "API: $API_URL"
echo "Jobs to submit: $NUM_JOBS"
echo "Job definition: $JOB_DEF_ID"
echo ""

# Check API is healthy
echo "Checking API health..."
if ! curl -sf "$API_URL/health" > /dev/null 2>&1; then
    echo -e "${RED}API is not responding. Make sure docker-compose is running.${NC}"
    exit 1
fi
echo -e "${GREEN}API is healthy${NC}"
echo ""

# Submit jobs
echo "Submitting $NUM_JOBS jobs..."
JOB_IDS=()
FAILED=0
for i in $(seq 1 "$NUM_JOBS"); do
    RESPONSE=$(curl -sf -X POST "$API_URL/api/v1/jobs/$JOB_DEF_ID/execute" \
        -H "Content-Type: application/json" \
        -d "{\"jobNumber\": $i}")

    if [ $? -eq 0 ]; then
        EXEC_ID=$(echo "$RESPONSE" | python3 -c "import sys, json; print(json.load(sys.stdin)['executionId'])" 2>/dev/null)
        if [ -n "$EXEC_ID" ]; then
            JOB_IDS+=("$EXEC_ID")
        fi
    else
        FAILED=$((FAILED + 1))
    fi

    # Progress indicator every 25 jobs
    if [ $((i % 25)) -eq 0 ]; then
        echo "  Submitted $i/$NUM_JOBS jobs..."
    fi
done

SUBMITTED=${#JOB_IDS[@]}
echo -e "${GREEN}Submitted $SUBMITTED jobs successfully ($FAILED failed)${NC}"
echo ""

# Wait for jobs to complete
echo "Waiting for jobs to complete..."
echo "(sample-1 = 3 sequential tasks × ~1s each ≈ 3s/job, with 4 runners ≈ $((NUM_JOBS * 3 / 4))s total)"
echo ""

COMPLETED=0
FAILED_JOBS=0
RUNNING=0
PENDING=0
MAX_WAIT=600  # 10 minutes max
START_TIME=$(date +%s)

while true; do
    COMPLETED=0
    FAILED_JOBS=0
    RUNNING=0
    PENDING=0

    for EXEC_ID in "${JOB_IDS[@]}"; do
        STATUS=$(curl -sf "$API_URL/api/v1/jobs/$EXEC_ID/status" 2>/dev/null | python3 -c "import sys, json; print(json.load(sys.stdin)['status'])" 2>/dev/null)
        case "$STATUS" in
            COMPLETED) COMPLETED=$((COMPLETED + 1)) ;;
            FAILED)    FAILED_JOBS=$((FAILED_JOBS + 1)) ;;
            RUNNING)   RUNNING=$((RUNNING + 1)) ;;
            *)         PENDING=$((PENDING + 1)) ;;
        esac
    done

    ELAPSED=$(( $(date +%s) - START_TIME ))
    echo -e "  [${ELAPSED}s] Completed: $COMPLETED | Running: $RUNNING | Pending: $PENDING | Failed: $FAILED_JOBS"

    DONE=$((COMPLETED + FAILED_JOBS))
    if [ "$DONE" -ge "$SUBMITTED" ]; then
        break
    fi

    if [ "$ELAPSED" -ge "$MAX_WAIT" ]; then
        echo -e "${RED}Timed out after ${MAX_WAIT}s${NC}"
        break
    fi

    sleep 5
done

TOTAL_TIME=$(( $(date +%s) - START_TIME ))

echo ""
echo -e "${GREEN}=== Results ===${NC}"
echo "Total jobs submitted: $SUBMITTED"
echo "Completed: $COMPLETED"
echo "Failed: $FAILED_JOBS"
echo "Total time: ${TOTAL_TIME}s"
if [ "$COMPLETED" -gt 0 ]; then
    THROUGHPUT=$(echo "scale=2; $COMPLETED / $TOTAL_TIME" | bc 2>/dev/null || echo "N/A")
    echo "Throughput: ~${THROUGHPUT} jobs/s"
fi
echo ""

# Get system status
echo -e "${YELLOW}=== System Status ===${NC}"
curl -sf "$API_URL/api/v1/system/status" | python3 -m json.tool 2>/dev/null
echo ""
