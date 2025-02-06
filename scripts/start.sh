#!/bin/bash
# scripts/start.sh

# Colors for output
GREEN='\033[0;32m'
RED='\033[0;31m'
NC='\033[0m'

echo -e "${GREEN}Starting Naxos Job Orchestration System...${NC}"


# Check if Docker is running
if ! docker info > /dev/null 2>&1; then
    echo -e "${RED}Docker is not running. Please start Docker first.${NC}"
    exit 1
fi

# Check if docker-compose is installed
if ! command -v docker compose > /dev/null 2>&1; then
    echo -e "${RED}docker compose is not installed. Please install it first.${NC}"
    exit 1
fi

# Build the images
echo "Building Docker images..."
docker compose build

# Start the services
echo "Starting services..."
docker compose up -d

# Wait for services to be healthy
echo "Waiting for services to be healthy..."
sleep 10

# Check service status
echo "Checking service status..."
if docker compose ps | grep -q "Exit"; then
    echo -e "${RED}Some services failed to start. Check logs with 'docker compose logs'${NC}"
    exit 1
fi

echo -e "${GREEN}All services are up and running!${NC}"
echo ""
echo "Available endpoints:"
echo "- API Server: http://localhost:8080"
echo "- RabbitMQ Management: http://localhost:15672 (username: naxos, password: naxos123)"
echo ""
echo "To check logs:"
echo "- API Server: docker compose logs api"
echo "- Runner 1: docker compose logs runner1"
echo "- Runner 2: docker compose logs runner2"
echo ""
echo "To stop the system:"
echo "./scripts/stop.sh"