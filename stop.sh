#!/bin/bash

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

echo -e "${YELLOW}Stopping Authentication Microservices...${NC}"

# Function to stop a service
stop_service() {
    local service_name=$1
    local pid_file=$2
    
    if [ -f "$pid_file" ]; then
        local pid=$(cat "$pid_file")
        if kill -0 "$pid" 2>/dev/null; then
            echo -e "${YELLOW}Stopping $service_name (PID: $pid)...${NC}"
            kill "$pid"
            
            # Wait for process to stop
            local count=0
            while kill -0 "$pid" 2>/dev/null && [ $count -lt 10 ]; do
                sleep 1
                count=$((count + 1))
            done
            
            if kill -0 "$pid" 2>/dev/null; then
                echo -e "${RED}Force killing $service_name...${NC}"
                kill -9 "$pid"
            fi
            
            echo -e "${GREEN}$service_name stopped successfully${NC}"
        else
            echo -e "${YELLOW}$service_name is not running${NC}"
        fi
        rm -f "$pid_file"
    else
        echo -e "${YELLOW}No PID file found for $service_name${NC}"
    fi
}

# Stop services
if [ -d "logs" ]; then
    stop_service "Auth Backend Service" "logs/auth-service.pid"
    stop_service "Auth Frontend Service" "logs/frontend.pid"
else
    echo -e "${YELLOW}No logs directory found. Services may not be running.${NC}"
fi

# Also try to kill any remaining go processes on these ports
echo -e "${YELLOW}Checking for any remaining processes on ports 8080 and 8081...${NC}"

# Kill processes on port 8080
if lsof -ti:8080 >/dev/null 2>&1; then
    echo -e "${YELLOW}Killing processes on port 8080...${NC}"
    lsof -ti:8080 | xargs kill -9 2>/dev/null
fi

# Kill processes on port 8081
if lsof -ti:8081 >/dev/null 2>&1; then
    echo -e "${YELLOW}Killing processes on port 8081...${NC}"
    lsof -ti:8081 | xargs kill -9 2>/dev/null
fi

echo -e "${GREEN}"
echo "=========================================="
echo "  Authentication Services Stopped!"
echo "=========================================="
echo -e "${NC}"