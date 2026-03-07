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
curl -s http://localhost:8003/api/v1/restaurants/health | jq '.'
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
    "role":"USER"
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
OTP_CODE=$(docker exec -i postgres-auth psql -U postgres -d auth_db -t -c "select code from otps order by id desc limit 1;" | tr -d '[:space:]')
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

# ── Restaurant Service Tests ────────────────────────────────────────────────

# Test 9: Login as ADMIN (seeded admin@admin.com / admin)
echo -e "${BLUE}[TEST 9] Login as ADMIN${NC}"
ADMIN_LOGIN=$(curl -s -X POST http://localhost:8001/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{"email":"admin@admin.com","password":"admin"}')

echo "$ADMIN_LOGIN" | jq '.'
ADMIN_TOKEN=$(echo "$ADMIN_LOGIN" | jq -r '.access_token // empty')

if [ -z "$ADMIN_TOKEN" ]; then
  echo -e "${RED}❌ Admin login failed${NC}\n"
  exit 1
fi
echo -e "${GREEN}✓ Admin login succeeded${NC}\n"

# Test 10: Register a RESTAURANT_OWNER
echo -e "${BLUE}[TEST 10] Register RESTAURANT_OWNER${NC}"
OWNER_TS=$(date +%s%N | cut -b1-13)
OWNER_PHONE="+1555$(printf '%07d' $((RANDOM * 32768 + RANDOM)))"
OWNER_REG=$(curl -s -X POST http://localhost:8001/api/v1/auth/register \
  -H "Content-Type: application/json" \
  -d '{
    "name":"Restaurant Owner",
    "email":"owner'$OWNER_TS'@test.com",
    "password":"Pass@12345",
    "phone":"'$OWNER_PHONE'",
    "role":"RESTAURANT_OWNER"
  }')

echo "$OWNER_REG" | jq '.'
OWNER_TOKEN=$(echo "$OWNER_REG" | jq -r '.access_token // empty')
OWNER_ID=$(echo "$OWNER_REG" | jq -r '.user.id // empty')

if [ -z "$OWNER_TOKEN" ]; then
  echo -e "${RED}❌ Owner registration failed${NC}\n"
  exit 1
fi
echo -e "${GREEN}✓ Owner registered: ID=$OWNER_ID${NC}\n"

# Test 11: Create Restaurant
echo -e "${BLUE}[TEST 11] Create Restaurant${NC}"
CREATE_REST=$(curl -s -X POST http://localhost:8003/api/v1/restaurants \
  -H "Authorization: Bearer $OWNER_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "name":"Tandoori Nights",
    "address":"123 Spice Lane",
    "latitude":17.385,
    "longitude":78.4867,
    "cuisine":"Indian"
  }')

echo "$CREATE_REST" | jq '.'
RESTAURANT_ID=$(echo "$CREATE_REST" | jq -r '.restaurant.id // empty')

if [ -z "$RESTAURANT_ID" ]; then
  echo -e "${RED}❌ Restaurant creation failed${NC}\n"
  exit 1
fi
echo -e "${GREEN}✓ Restaurant created: ID=$RESTAURANT_ID${NC}\n"

# Test 12: Approve Restaurant (ADMIN)
echo -e "${BLUE}[TEST 12] Approve Restaurant (ADMIN)${NC}"
APPROVE_RESP=$(curl -s -X PATCH http://localhost:8003/api/v1/restaurants/$RESTAURANT_ID/approve \
  -H "Authorization: Bearer $ADMIN_TOKEN")
echo "$APPROVE_RESP" | jq '.'
IS_APPROVED=$(echo "$APPROVE_RESP" | jq '.restaurant.is_approved')
if [ "$IS_APPROVED" = "true" ]; then
  echo -e "${GREEN}✓ Restaurant approved${NC}\n"
else
  echo -e "${RED}❌ Restaurant approval failed${NC}\n"
fi

# Test 13: List Restaurants (public, no auth — only approved+open)
echo -e "${BLUE}[TEST 13] List Restaurants (Public)${NC}"
LIST_REST=$(curl -s http://localhost:8003/api/v1/restaurants)
echo "$LIST_REST" | jq '.'
LIST_COUNT=$(echo "$LIST_REST" | jq '.restaurants | length')
if [ "$LIST_COUNT" -gt 0 ] 2>/dev/null; then
  echo -e "${GREEN}✓ Listed $LIST_COUNT restaurant(s)${NC}\n"
else
  echo -e "${RED}⚠ No restaurants listed (expected at least 1)${NC}\n"
fi

# Test 14: Get Restaurant with Menu (public, no auth)
echo -e "${BLUE}[TEST 14] Get Restaurant with Menu (Public)${NC}"
curl -s http://localhost:8003/api/v1/restaurants/$RESTAURANT_ID | jq '.'
echo

# Test 15: Search Restaurants (public, no auth)
echo -e "${BLUE}[TEST 15] Search Restaurants (Public)${NC}"
SEARCH_RESP=$(curl -s "http://localhost:8003/api/v1/restaurants/search?q=Tandoori")
echo "$SEARCH_RESP" | jq '.'
SEARCH_COUNT=$(echo "$SEARCH_RESP" | jq '.restaurants | length')
if [ "$SEARCH_COUNT" -gt 0 ] 2>/dev/null; then
  echo -e "${GREEN}✓ Search found $SEARCH_COUNT restaurant(s)${NC}\n"
else
  echo -e "${RED}⚠ Search returned empty (expected at least 1)${NC}\n"
fi

# Test 16: Nearby Restaurants (public, no auth)
echo -e "${BLUE}[TEST 16] Nearby Restaurants (Public)${NC}"
NEARBY_RESP=$(curl -s "http://localhost:8003/api/v1/restaurants/nearby?lat=17.385&lng=78.4867&radius=10")
echo "$NEARBY_RESP" | jq '.'
NEARBY_COUNT=$(echo "$NEARBY_RESP" | jq '.restaurants | length')
if [ "$NEARBY_COUNT" -gt 0 ] 2>/dev/null; then
  echo -e "${GREEN}✓ Nearby found $NEARBY_COUNT restaurant(s)${NC}\n"
else
  echo -e "${RED}⚠ Nearby returned empty (expected at least 1)${NC}\n"
fi

# Test 17: Update Restaurant (owner)
echo -e "${BLUE}[TEST 17] Update Restaurant${NC}"
curl -s -X PUT http://localhost:8003/api/v1/restaurants/$RESTAURANT_ID \
  -H "Authorization: Bearer $OWNER_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"name":"Tandoori Nights Deluxe","cuisine":"North Indian"}' | jq '.'
echo

# Test 18: Toggle Status (owner)
echo -e "${BLUE}[TEST 18] Toggle Restaurant Status${NC}"
curl -s -X PATCH http://localhost:8003/api/v1/restaurants/$RESTAURANT_ID/status \
  -H "Authorization: Bearer $OWNER_TOKEN" | jq '.'
echo

# Test 19: Create Menu Item (owner)
echo -e "${BLUE}[TEST 19] Create Menu Item${NC}"
CREATE_MENU=$(curl -s -X POST http://localhost:8003/api/v1/restaurants/$RESTAURANT_ID/menu \
  -H "Authorization: Bearer $OWNER_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "name":"Butter Chicken",
    "description":"Creamy tomato-based chicken curry",
    "price":350.00,
    "category":"Main Course",
    "is_veg":false,
    "image_url":"https://example.com/butter-chicken.jpg"
  }')

echo "$CREATE_MENU" | jq '.'
MENU_ITEM_ID=$(echo "$CREATE_MENU" | jq -r '.menu_item.id // empty')

if [ -z "$MENU_ITEM_ID" ]; then
  echo -e "${RED}⚠ Menu item creation failed${NC}\n"
else
  echo -e "${GREEN}✓ Menu item created: ID=$MENU_ITEM_ID${NC}\n"
fi

# Test 20: List Menu Items (public, no auth)
echo -e "${BLUE}[TEST 20] List Menu Items (Public)${NC}"
curl -s http://localhost:8003/api/v1/restaurants/$RESTAURANT_ID/menu | jq '.'
echo

# Test 21: Update Menu Item (owner)
if [ -n "$MENU_ITEM_ID" ]; then
  echo -e "${BLUE}[TEST 21] Update Menu Item${NC}"
  curl -s -X PUT http://localhost:8003/api/v1/restaurants/$RESTAURANT_ID/menu/$MENU_ITEM_ID \
    -H "Authorization: Bearer $OWNER_TOKEN" \
    -H "Content-Type: application/json" \
    -d '{"price":399.00,"description":"Rich & creamy butter chicken"}' | jq '.'
  echo
fi

# Test 22: Toggle Menu Item Availability (owner)
if [ -n "$MENU_ITEM_ID" ]; then
  echo -e "${BLUE}[TEST 22] Toggle Menu Item Availability${NC}"
  curl -s -X PATCH http://localhost:8003/api/v1/restaurants/$RESTAURANT_ID/menu/$MENU_ITEM_ID/toggle \
    -H "Authorization: Bearer $OWNER_TOKEN" | jq '.'
  echo
fi

# Test 23: Internal - Get Restaurant
echo -e "${BLUE}[TEST 23] Internal - Get Restaurant${NC}"
curl -s http://localhost:8003/api/v1/internal/restaurants/$RESTAURANT_ID | jq '.'
echo

# Test 24: Internal - Get Menu Item
if [ -n "$MENU_ITEM_ID" ]; then
  echo -e "${BLUE}[TEST 24] Internal - Get Menu Item${NC}"
  curl -s http://localhost:8003/api/v1/internal/restaurants/$RESTAURANT_ID/menu/$MENU_ITEM_ID | jq '.'
  echo
fi

# Test 25: Delete Menu Item (owner)
if [ -n "$MENU_ITEM_ID" ]; then
  echo -e "${BLUE}[TEST 25] Delete Menu Item${NC}"
  curl -s -X DELETE http://localhost:8003/api/v1/restaurants/$RESTAURANT_ID/menu/$MENU_ITEM_ID \
    -H "Authorization: Bearer $OWNER_TOKEN" | jq '.'
  echo
fi

# Test 26: Delete Restaurant (owner)
echo -e "${BLUE}[TEST 26] Delete Restaurant${NC}"
curl -s -X DELETE http://localhost:8003/api/v1/restaurants/$RESTAURANT_ID \
  -H "Authorization: Bearer $OWNER_TOKEN" | jq '.'
echo

# ── Auth Cleanup Tests ──────────────────────────────────────────────────────

# Test 27: Delete account (triggers USER_DELETED event)
echo -e "${BLUE}[TEST 27] Delete Account (triggers event)${NC}"
DELETE_RESPONSE=$(curl -s -X DELETE http://localhost:8001/api/v1/auth/account \
  -H "Authorization: Bearer $ACCESS_TOKEN")

echo "$DELETE_RESPONSE" | jq '.'
echo

# Wait for event processing
sleep 2

# Test 28: Verify profile was deleted and token rejected
echo -e "${BLUE}[TEST 28] Verify Profile Deleted via Event${NC}"
GET_DELETED=$(curl -s -o /dev/null -w '%{http_code}' -H "Authorization: Bearer $ACCESS_TOKEN" \
  http://localhost:8002/api/v1/users/$USER_ID)

echo "HTTP status: $GET_DELETED"
if [ "$GET_DELETED" = "401" ] || [ "$GET_DELETED" = "404" ]; then
  echo -e "${GREEN}✓ Profile access blocked after deletion (HTTP $GET_DELETED)${NC}\n"
else
  echo -e "${RED}⚠ Unexpected status: $GET_DELETED${NC}\n"
fi

# Test 29: Privilege escalation regression — ADMIN role must be rejected
echo -e "${BLUE}[TEST 29] Privilege Escalation Regression (ADMIN role)${NC}"
ADMIN_STATUS=$(curl -s -o /dev/null -w '%{http_code}' -X POST http://localhost:8001/api/v1/auth/register \
  -H 'Content-Type: application/json' \
  -d '{"name":"hacker","email":"hacker@x.com","password":"Pass@12345","phone":"+15559999999","role":"ADMIN"}')
echo "HTTP status: $ADMIN_STATUS"
if [ "$ADMIN_STATUS" = "400" ]; then
  echo -e "${GREEN}✓ ADMIN role correctly rejected (400)${NC}\n"
else
  echo -e "${RED}⚠ Expected 400, got $ADMIN_STATUS${NC}\n"
fi

# Test 30: Deleted user login regression — must be rejected
echo -e "${BLUE}[TEST 30] Deleted User Login Regression${NC}"
DELETED_LOGIN_STATUS=$(curl -s -o /dev/null -w '%{http_code}' -X POST http://localhost:8001/api/v1/auth/login \
  -H 'Content-Type: application/json' \
  -d '{"email":"'$EMAIL'","password":"'$PASSWORD'"}')
echo "HTTP status: $DELETED_LOGIN_STATUS"
if [ "$DELETED_LOGIN_STATUS" = "401" ]; then
  echo -e "${GREEN}✓ Deleted user login correctly rejected (401)${NC}\n"
else
  echo -e "${RED}⚠ Expected 401, got $DELETED_LOGIN_STATUS${NC}\n"
fi

echo -e "${GREEN}=== Testing Complete ===${NC}"
