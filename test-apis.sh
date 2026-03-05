#!/bin/bash

# Colors for output
GREEN='\033[0;32m'
BLUE='\033[0;34m'
RED='\033[0;31m'
NC='\033[0m'

echo -e "${BLUE}=== Food Platform API Testing ===${NC}\n"

# quick health probes
echo -e "${BLUE}[INIT] Health checks${NC}"
curl -s http://localhost:8001/api/v1/auth/health | jq '.'
curl -s http://localhost:8002/api/v1/users/health | jq '.'
echo

# internal endpoint check (not exposed to public)
echo -e "${BLUE}[INIT] Internal ensure-profile endpoint${NC}"
curl -s -X POST http://localhost:8002/api/v1/internal/users/ensure \
  -H "Content-Type: application/json" \
  -d '{"auth_id":999999}' | jq '.'
echo


# Test 1: Register a user
echo -e "${BLUE}[TEST 1] Register New User${NC}"
TIMESTAMP=$(date +%s%N | cut -b1-13)
PHONE="+1555$(printf '%07d' $((RANDOM * 32768 + RANDOM)))"
REGISTER_RESPONSE=$(curl -s -X POST http://localhost:8001/api/v1/auth/register \
  -H "Content-Type: application/json" \
  -d '{
    "name":"Test User",
    "email":"'$TIMESTAMP'@test.com",
    "password":"Pass@12345",
    "phone":"'$PHONE'",
    "role":"customer"
  }')

echo "$REGISTER_RESPONSE" | jq '.'
USER_ID=$(echo "$REGISTER_RESPONSE" | jq -r '.user.id // empty')
ACCESS_TOKEN=$(echo "$REGISTER_RESPONSE" | jq -r '.access_token // empty')

if [ -z "$USER_ID" ] || [ -z "$ACCESS_TOKEN" ]; then
  echo -e "${RED}❌ Registration failed${NC}\n"
  exit 1
fi

echo -e "${GREEN}✓ User registered: ID=$USER_ID${NC}\n"

# Wait a bit for the event to be consumed
sleep 2

# store credentials for later tests
EMAIL="$TIMESTAMP@test.com"
PASSWORD="Pass@12345"

# Test 2: Login using credentials
echo -e "${BLUE}[TEST 2] Login with email/password${NC}"
LOGIN_RESPONSE=$(curl -s -X POST http://localhost:8001/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{"email":"'$EMAIL'","password":"'$PASSWORD'"}')

echo "$LOGIN_RESPONSE" | jq '.'
LOGIN_ACCESS=$(echo "$LOGIN_RESPONSE" | jq -r '.access_token // empty')
LOGIN_REFRESH=$(echo "$LOGIN_RESPONSE" | jq -r '.refresh_token // empty')
if [ -z "$LOGIN_ACCESS" ]; then
  echo -e "${RED}⚠ Login failed${NC}\n"
else
  echo -e "${GREEN}✓ Login succeeded${NC}\n"
fi

# Test 3: Refresh token
if [ -n "$LOGIN_REFRESH" ]; then
  echo -e "${BLUE}[TEST 3] Refreshing token${NC}"
  REFRESH_RESPONSE=$(curl -s -X POST http://localhost:8001/api/v1/auth/refresh \
    -H "Content-Type: application/json" \
    -d '{"refresh_token":"'$LOGIN_REFRESH'"}')
  echo "$REFRESH_RESPONSE" | jq '.'
  echo
fi

# Test 4: Send OTP to phone (should succeed)
echo -e "${BLUE}[TEST 4] Send OTP${NC}"
OTP_SEND=$(curl -s -X POST http://localhost:8001/api/v1/auth/otp/send \
  -H "Content-Type: application/json" \
  -d '{"phone":"'$PHONE'"}')
echo "$OTP_SEND" | jq '.'
echo

# fetch latest OTP code from DB
OTP_CODE=$(docker exec -i food-platform-postgres-auth-1 psql -U postgres -d auth_db -t -c "select code from otps order by id desc limit 1;" | tr -d '[:space:]')
if [ -n "$OTP_CODE" ]; then
  echo -e "${BLUE}[TEST 5] Verify OTP (code: $OTP_CODE)${NC}"
  OTP_VERIFY=$(curl -s -X POST http://localhost:8001/api/v1/auth/otp/verify \
    -H "Content-Type: application/json" \
    -d '{"phone":"'$PHONE'","code":"'$OTP_CODE'"}')
  echo "$OTP_VERIFY" | jq '.'
  echo
fi

# Test 6: Verify profile was created in user-service
echo -e "${BLUE}[TEST 6] Verify Profile Created via Event${NC}"
PROFILE_RESPONSE=$(curl -s -H "Authorization: Bearer $ACCESS_TOKEN" \
  http://localhost:8002/api/v1/users/$USER_ID)

echo "$PROFILE_RESPONSE" | jq '.'
if echo "$PROFILE_RESPONSE" | jq -e '.auth_id' > /dev/null 2>&1; then
  echo -e "${GREEN}✓ Profile found in user-service (event was consumed)${NC}\n"
else
  echo -e "${RED}⚠ Profile not found or error${NC}\n"
fi

# Test 7: Add an address
echo -e "${BLUE}[TEST 7] Add Address${NC}"
ADDRESS_RESPONSE=$(curl -s -X POST http://localhost:8002/api/v1/users/$USER_ID/addresses \
  -H "Authorization: Bearer $ACCESS_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "label":"Home",
    "line1":"123 Main Street",
    "city":"New York",
    "pincode":"10001",
    "latitude":40.7128,
    "longitude":-74.0060
  }')

echo "$ADDRESS_RESPONSE" | jq '.'
echo

# Test 8: List addresses
echo -e "${BLUE}[TEST 8] List Addresses${NC}"
LIST_RESPONSE=$(curl -s -H "Authorization: Bearer $ACCESS_TOKEN" \
  http://localhost:8002/api/v1/users/$USER_ID/addresses)

echo "$LIST_RESPONSE" | jq '.'
echo

# Test 9: Delete account (triggers USER_DELETED event)
echo -e "${BLUE}[TEST 9] Delete Account (triggers event)${NC}"
DELETE_RESPONSE=$(curl -s -X DELETE http://localhost:8001/api/v1/auth/account \
  -H "Authorization: Bearer $ACCESS_TOKEN")

echo "$DELETE_RESPONSE" | jq '.'
echo

# Wait for event processing
sleep 2

# Test 10: Verify profile was deleted
echo -e "${BLUE}[TEST 10] Verify Profile Deleted via Event${NC}"
GET_DELETED=$(curl -s -H "Authorization: Bearer $ACCESS_TOKEN" \
  http://localhost:8002/api/v1/users/$USER_ID)

echo "$GET_DELETED" | jq '.'
if echo "$GET_DELETED" | jq -e '.error' > /dev/null 2>&1; then
  echo -e "${GREEN}✓ Profile  deleted (event was consumed)${NC}\n"
else
  echo -e "${RED}⚠ Profile still exists${NC}\n"
fi

echo -e "${GREEN}=== Testing Complete ===${NC}"
