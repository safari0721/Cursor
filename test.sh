#!/bin/bash

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

echo -e "${GREEN}Testing Authentication Microservices...${NC}"

# Base URLs
AUTH_SERVICE="http://localhost:8081"
FRONTEND_SERVICE="http://localhost:8080"

# Test function
test_endpoint() {
    local method=$1
    local url=$2
    local data=$3
    local expected_status=$4
    local description=$5
    
    echo -e "${YELLOW}Testing: $description${NC}"
    
    if [ -n "$data" ]; then
        response=$(curl -s -w "\n%{http_code}" -X $method -H "Content-Type: application/json" -d "$data" "$url")
    else
        response=$(curl -s -w "\n%{http_code}" -X $method "$url")
    fi
    
    # Extract status code (last line)
    status_code=$(echo "$response" | tail -n1)
    # Extract response body (all but last line)
    body=$(echo "$response" | head -n -1)
    
    if [ "$status_code" -eq "$expected_status" ]; then
        echo -e "${GREEN}✓ PASS${NC} - Status: $status_code"
        if [ -n "$body" ]; then
            echo "  Response: $body"
        fi
    else
        echo -e "${RED}✗ FAIL${NC} - Expected: $expected_status, Got: $status_code"
        if [ -n "$body" ]; then
            echo "  Response: $body"
        fi
    fi
    echo ""
}

# Wait for services to be ready
echo -e "${YELLOW}Waiting for services to be ready...${NC}"
sleep 2

# Test 1: Health check
test_endpoint "GET" "$AUTH_SERVICE/health" "" 200 "Auth service health check"

# Test 2: Frontend service accessibility
test_endpoint "GET" "$FRONTEND_SERVICE/" "" 200 "Frontend service accessibility"

# Test 3: User signup
test_endpoint "POST" "$AUTH_SERVICE/signup" '{"username":"testuser","password":"testpass123","email":"test@example.com"}' 200 "User signup"

# Test 4: User login
test_endpoint "POST" "$AUTH_SERVICE/login" '{"username":"testuser","password":"testpass123"}' 200 "User login"

# Test 5: User login with wrong password
test_endpoint "POST" "$AUTH_SERVICE/login" '{"username":"testuser","password":"wrongpass"}' 401 "User login with wrong password"

# Test 6: Change password
test_endpoint "POST" "$AUTH_SERVICE/change-password" '{"username":"testuser","old_password":"testpass123","new_password":"newpass123"}' 200 "Change password"

# Test 7: Login with new password
test_endpoint "POST" "$AUTH_SERVICE/login" '{"username":"testuser","password":"newpass123"}' 200 "Login with new password"

# Test 8: Duplicate signup
test_endpoint "POST" "$AUTH_SERVICE/signup" '{"username":"testuser","password":"testpass123","email":"test2@example.com"}' 409 "Duplicate user signup"

echo -e "${GREEN}Testing completed!${NC}"