#!/bin/bash
# scripts/stop.sh

# Colors for output
GREEN='\033[0;32m'
RED='\033[0;31m'
NC='\033[0m'

echo -e "${GREEN}Stopping Naxos Job Orchestration System...${NC}"

# Stop all services
docker compose down

echo -e "${GREEN}All services stopped successfully!${NC}"