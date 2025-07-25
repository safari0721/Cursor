#!/bin/bash

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

echo -e "${GREEN}Starting Authentication Microservices...${NC}"

# Function to check if a port is in use
check_port() {
    if lsof -Pi :$1 -sTCP:LISTEN -t >/dev/null ; then
        echo -e "${RED}Port $1 is already in use. Please stop the service using that port.${NC}"
        exit 1
    fi
}

# Check if ports are available
echo -e "${YELLOW}Checking if ports 8080 and 8081 are available...${NC}"
check_port 8080
check_port 8081

# Create log directory
mkdir -p logs

# Start auth service in background
echo -e "${YELLOW}Starting Auth Backend Service on port 8081...${NC}"
cd auth-service
go mod tidy > ../logs/auth-service-setup.log 2>&1
if [ $? -ne 0 ]; then
    echo -e "${RED}Failed to install auth service dependencies. Check logs/auth-service-setup.log${NC}"
    exit 1
fi

nohup go run main.go > ../logs/auth-service.log 2>&1 &
AUTH_PID=$!
cd ..

# Wait a bit for auth service to start
sleep 3

# Check if auth service is running
if ! curl -s http://localhost:8081/health > /dev/null; then
    echo -e "${RED}Auth service failed to start. Check logs/auth-service.log${NC}"
    kill $AUTH_PID 2>/dev/null
    exit 1
fi

echo -e "${GREEN}Auth Backend Service started successfully (PID: $AUTH_PID)${NC}"

# Start frontend service
echo -e "${YELLOW}Starting Auth Frontend Service on port 8080...${NC}"
go mod tidy > logs/frontend-setup.log 2>&1
if [ $? -ne 0 ]; then
    echo -e "${RED}Failed to install frontend dependencies. Check logs/frontend-setup.log${NC}"
    kill $AUTH_PID 2>/dev/null
    exit 1
fi

export AUTH_SERVICE_URL=http://localhost:8081
nohup go run main.go > logs/frontend.log 2>&1 &
FRONTEND_PID=$!

# Wait a bit for frontend service to start
sleep 3

# Check if frontend service is running
if ! curl -s http://localhost:8080/ > /dev/null; then
    echo -e "${RED}Frontend service failed to start. Check logs/frontend.log${NC}"
    kill $AUTH_PID $FRONTEND_PID 2>/dev/null
    exit 1
fi

echo -e "${GREEN}Auth Frontend Service started successfully (PID: $FRONTEND_PID)${NC}"

# Save PIDs for stop script
echo $AUTH_PID > logs/auth-service.pid
echo $FRONTEND_PID > logs/frontend.pid

echo -e "${GREEN}"
echo "=========================================="
echo "  Authentication Services Started!"
echo "=========================================="
echo -e "${NC}"
echo "Frontend Service: http://localhost:8080"
echo "Backend Service:  http://localhost:8081"
echo ""
echo "Logs are available in the logs/ directory:"
echo "  - logs/auth-service.log"
echo "  - logs/frontend.log"
echo ""
echo "To stop the services, run: ./stop.sh"
echo ""
echo -e "${YELLOW}Services are running in the background...${NC}"