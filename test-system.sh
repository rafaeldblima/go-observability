#!/bin/bash

echo "🚀 Testing Go Observability System"
echo "=================================="

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Base URL
BASE_URL="http://localhost:8080"

# Function to test endpoint
test_endpoint() {
    local description="$1"
    local method="$2"
    local url="$3"
    local data="$4"
    local expected_status="$5"
    
    echo -e "\n${YELLOW}Testing: $description${NC}"
    echo "Request: $method $url"
    if [ ! -z "$data" ]; then
        echo "Data: $data"
    fi
    
    if [ "$method" = "POST" ]; then
        response=$(curl -s -w "\n%{http_code}" -X POST "$url" \
            -H "Content-Type: application/json" \
            -d "$data")
    else
        response=$(curl -s -w "\n%{http_code}" "$url")
    fi
    
    # Split response and status code
    status_code=$(echo "$response" | tail -n1)
    body=$(echo "$response" | head -n -1)
    
    echo "Status Code: $status_code"
    echo "Response: $body"
    
    if [ "$status_code" = "$expected_status" ]; then
        echo -e "${GREEN}✅ PASSED${NC}"
    else
        echo -e "${RED}❌ FAILED - Expected $expected_status, got $status_code${NC}"
    fi
}

# Wait for services to be ready
echo "⏳ Waiting for services to be ready..."
sleep 5

# Test 1: Health check Service A
test_endpoint "Service A Health Check" "GET" "$BASE_URL/health" "" "200"

# Test 2: Health check Service B
test_endpoint "Service B Health Check" "GET" "http://localhost:8081/health" "" "200"

# Test 3: Valid CEP (São Paulo)
test_endpoint "Valid CEP - São Paulo" "POST" "$BASE_URL/" '{"cep": "01310100"}' "200"

# Test 4: Invalid CEP - wrong format
test_endpoint "Invalid CEP - Wrong Format" "POST" "$BASE_URL/" '{"cep": "123"}' "422"

# Test 5: Invalid CEP - letters
test_endpoint "Invalid CEP - Contains Letters" "POST" "$BASE_URL/" '{"cep": "abcd1234"}' "422"

# Test 6: Invalid CEP - too long
test_endpoint "Invalid CEP - Too Long" "POST" "$BASE_URL/" '{"cep": "123456789"}' "422"

# Test 7: Non-existent CEP
test_endpoint "Non-existent CEP" "POST" "$BASE_URL/" '{"cep": "99999999"}' "404"

# Test 8: Valid CEP (Rio de Janeiro)
test_endpoint "Valid CEP - Rio de Janeiro" "POST" "$BASE_URL/" '{"cep": "20040020"}' "200"

# Test 9: Missing CEP field
test_endpoint "Missing CEP Field" "POST" "$BASE_URL/" '{}' "422"

# Test 10: Empty CEP
test_endpoint "Empty CEP" "POST" "$BASE_URL/" '{"cep": ""}' "422"

echo -e "\n🏁 Testing completed!"
echo -e "\n📊 Check Zipkin traces at: ${YELLOW}http://localhost:9411${NC}"
echo -e "🔍 View service traces and performance metrics"
