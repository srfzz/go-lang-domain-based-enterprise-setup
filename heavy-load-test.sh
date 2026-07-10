#!/bin/bash

echo "🔥 HEAVY LOAD TEST - Like Goravel"
echo "=================================="
echo ""

# Colors
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
RED='\033[0;31m'
NC='\033[0m'

# Get token
TOKEN=$(curl -s -X POST http://localhost:8080/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{"email":"admin@enterprise.com","password":"Admin123!"}' \
  | jq -r '.access_token')

echo -e "${GREEN}✅ Token obtained${NC}"
echo ""

# Test 1: Like Goravel - 10,000 users, 30 seconds
echo -e "${BLUE}Test 1: 10,000 Concurrent Users, 30 seconds${NC}"
echo "This is what Goravel did: 25,573 RPS"
echo "----------------------------------------"
echo -e "${YELLOW}WARNING: This will stress your system!${NC}"
read -p "Continue? (y/n): " -n 1 -r
echo ""
if [[ $REPLY =~ ^[Yy]$ ]]; then
    hey -z 30s -c 10000 -q 5000 http://localhost:8080/health 2>&1 | grep -E "Requests/sec|Total|Average|Status code"
else
    echo -e "${YELLOW}Skipped${NC}"
fi
echo ""

# Test 2: 5,000 users, 60 seconds
echo -e "${BLUE}Test 2: 5,000 Concurrent Users, 60 seconds${NC}"
echo "----------------------------------------"
read -p "Continue? (y/n): " -n 1 -r
echo ""
if [[ $REPLY =~ ^[Yy]$ ]]; then
    hey -z 60s -c 5000 -q 3000 http://localhost:8080/health 2>&1 | grep -E "Requests/sec|Total|Average"
else
    echo -e "${YELLOW}Skipped${NC}"
fi
echo ""

# Test 3: Admin API with 1,000 users
echo -e "${BLUE}Test 3: Admin API, 1,000 Concurrent Users${NC}"
echo "----------------------------------------"
hey -z 30s -c 1000 -q 2000 -H "Authorization: Bearer $TOKEN" http://localhost:8080/api/v1/admin/users 2>&1 | grep -E "Requests/sec|Total|Average"
echo ""

echo -e "${GREEN}✅ Heavy load test complete!${NC}"
