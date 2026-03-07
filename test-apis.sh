#!/bin/bash

# Colors for output
GREEN='\033[0;32m'
BLUE='\033[0;34m'
RED='\033[0;31m'
YELLOW='\033[0;33m'
CYAN='\033[0;36m'
NC='\033[0m'

# Helper: log request details (method, url, auth, body)
log_req() {
  local method="$1" url="$2" auth="$3" body="$4"
  echo -e "${CYAN}→ $method $url${NC}"
  [ -n "$auth" ] && echo -e "${CYAN}  Auth: Bearer ${auth:0:20}...${NC}"
  [ -n "$body" ] && echo -e "${CYAN}  Body: $(echo "$body" | jq -c . 2>/dev/null || echo "$body")${NC}"
}

echo -e "${BLUE}=== Food Platform API Testing ===${NC}\n"

# quick health probes
echo -e "${BLUE}[INIT] Health checks${NC}"
curl -s http://localhost:8001/api/v1/auth/health | jq '.'
curl -s http://localhost:8002/api/v1/users/health | jq '.'
curl -s http://localhost:8003/api/v1/restaurants/health | jq '.'
curl -s http://localhost:8004/api/v1/orders/health | jq '.'
curl -s http://localhost:8005/api/v1/delivery/health | jq '.'
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
log_req "POST" "http://localhost:8001/api/v1/auth/register" "" '{"name":"Test User","email":"'$TIMESTAMP'@test.com","phone":"'$PHONE'","role":"USER"}'
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
log_req "POST" "http://localhost:8001/api/v1/auth/login" "" '{"email":"'$EMAIL'"}'
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
log_req "POST" "http://localhost:8002/api/v1/users/$USER_ID/addresses" "$ACCESS_TOKEN" '{"label":"Home","city":"New York","pincode":"10001"}'
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
log_req "POST" "http://localhost:8003/api/v1/restaurants" "$OWNER_TOKEN" '{"name":"Tandoori Nights","cuisine":"Indian"}'
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
log_req "PATCH" "http://localhost:8003/api/v1/restaurants/$RESTAURANT_ID/approve" "$ADMIN_TOKEN" ""
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
log_req "POST" "http://localhost:8003/api/v1/restaurants/$RESTAURANT_ID/menu" "$OWNER_TOKEN" '{"name":"Butter Chicken","price":350}'
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

# Re-create menu item for order tests
echo -e "${BLUE}[TEST 25b] Re-create Menu Item for Order Tests${NC}"
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
if [ -n "$MENU_ITEM_ID" ]; then
  echo -e "${GREEN}✓ Menu item recreated: ID=$MENU_ITEM_ID${NC}\n"
fi

# ── Order Service Tests ────────────────────────────────────────────────────

# Re-open restaurant (test 18 toggled it closed)
echo -e "${BLUE}[PREP] Re-open restaurant for order tests${NC}"
curl -s -X PATCH http://localhost:8003/api/v1/restaurants/$RESTAURANT_ID/status \
  -H "Authorization: Bearer $OWNER_TOKEN" | jq '.'
echo

# Test 26: Place Order (USER)
echo -e "${BLUE}[TEST 26] Place Order (USER)${NC}"
IDEMPOTENCY_KEY=$(uuidgen 2>/dev/null || echo "order-$(date +%s%N)")
log_req "POST" "http://localhost:8004/api/v1/orders" "$ACCESS_TOKEN" '{"restaurant_id":'$RESTAURANT_ID',"menu_item_id":'$MENU_ITEM_ID',"delivery_address":"'$DELIVERY_ADDRESS'","idempotency_key":"'$IDEMPOTENCY_KEY'"}'

# Use a stable address payload for order tests.
DELIVERY_ADDRESS="123 Main St, New York 10001"

ORDER_RESPONSE=$(curl -s -X POST http://localhost:8004/api/v1/orders \
  -H "Authorization: Bearer $ACCESS_TOKEN" \
  -H "Content-Type: application/json" \
  -H "Idempotency-Key: $IDEMPOTENCY_KEY" \
  -d '{
    "restaurant_id": '$RESTAURANT_ID',
    "menu_item_id": '$MENU_ITEM_ID',
    "delivery_address": "'"$DELIVERY_ADDRESS"'",
    "notes": "Extra spicy please"
  }')

echo "$ORDER_RESPONSE" | jq '.'
ORDER_ID=$(echo "$ORDER_RESPONSE" | jq -r '.order.id // .id // empty')

if [ -z "$ORDER_ID" ]; then
  echo -e "${RED}⚠ Order placement failed${NC}\n"
else
  echo -e "${GREEN}✓ Order placed: ID=$ORDER_ID${NC}"
  echo -e "${YELLOW}── Order after ORDER_PLACED ──${NC}"
  echo "$ORDER_RESPONSE" | jq '.order'
  echo
fi

# Test 27: Idempotency Check (should return 409)
echo -e "${BLUE}[TEST 27] Idempotency Check (duplicate key)${NC}"
DUPLICATE_STATUS=$(curl -s -o /dev/null -w '%{http_code}' -X POST http://localhost:8004/api/v1/orders \
  -H "Authorization: Bearer $ACCESS_TOKEN" \
  -H "Content-Type: application/json" \
  -H "Idempotency-Key: $IDEMPOTENCY_KEY" \
  -d '{
    "restaurant_id": '$RESTAURANT_ID',
    "menu_item_id": '$MENU_ITEM_ID',
    "delivery_address": "'"$DELIVERY_ADDRESS"'"
  }')

echo "HTTP status: $DUPLICATE_STATUS"
if [ "$DUPLICATE_STATUS" = "409" ]; then
  echo -e "${GREEN}✓ Idempotency key correctly rejected (409)${NC}\n"
else
  echo -e "${RED}⚠ Expected 409, got $DUPLICATE_STATUS${NC}\n"
fi

# Test 28: Get Order Details
if [ -n "$ORDER_ID" ]; then
  echo -e "${BLUE}[TEST 28] Get Order Details${NC}"
  curl -s -H "Authorization: Bearer $ACCESS_TOKEN" \
    http://localhost:8004/api/v1/orders/$ORDER_ID | jq '.'
  echo
fi

# Test 29: Get User Order History
echo -e "${BLUE}[TEST 29] Get User Order History${NC}"
if [ -n "$USER_ID" ]; then
  curl -s -H "Authorization: Bearer $ACCESS_TOKEN" \
    http://localhost:8004/api/v1/orders/user/$USER_ID | jq '.'
  echo
fi

# Test 30: Get Restaurant Orders (OWNER)
echo -e "${BLUE}[TEST 30] Get Restaurant Orders (OWNER)${NC}"
curl -s -H "Authorization: Bearer $OWNER_TOKEN" \
  http://localhost:8004/api/v1/orders/restaurant/$RESTAURANT_ID | jq '.'
echo

# Test 31: Restaurant Owner updates status (JWT-protected, owner token)
if [ -n "$ORDER_ID" ]; then
  echo -e "${BLUE}[TEST 31] Owner: Confirm Order${NC}"
  log_req "PATCH" "http://localhost:8004/api/v1/orders/$ORDER_ID/status" "$OWNER_TOKEN" '{"status":"CONFIRMED"}'
  curl -s -X PATCH http://localhost:8004/api/v1/orders/$ORDER_ID/status \
    -H "Authorization: Bearer $OWNER_TOKEN" \
    -H "Content-Type: application/json" \
    -d '{"status":"CONFIRMED"}' | jq '.'
  echo -e "${YELLOW}── Order after CONFIRMED ──${NC}"
  curl -s -H "Authorization: Bearer $ACCESS_TOKEN" \
    http://localhost:8004/api/v1/orders/$ORDER_ID | jq '.order'
  echo
  
  # Update to PREPARING
  echo -e "${BLUE}[TEST 31b] Owner: Mark PREPARING${NC}"
  log_req "PATCH" "http://localhost:8004/api/v1/orders/$ORDER_ID/status" "$OWNER_TOKEN" '{"status":"PREPARING"}'
  curl -s -X PATCH http://localhost:8004/api/v1/orders/$ORDER_ID/status \
    -H "Authorization: Bearer $OWNER_TOKEN" \
    -H "Content-Type: application/json" \
    -d '{"status":"PREPARING"}' | jq '.'
  echo -e "${YELLOW}── Order after PREPARING ──${NC}"
  curl -s -H "Authorization: Bearer $ACCESS_TOKEN" \
    http://localhost:8004/api/v1/orders/$ORDER_ID | jq '.order'
  echo
  
  # Update to PREPARED
  echo -e "${BLUE}[TEST 31c] Owner: Mark PREPARED${NC}"
  log_req "PATCH" "http://localhost:8004/api/v1/orders/$ORDER_ID/status" "$OWNER_TOKEN" '{"status":"PREPARED"}'
  curl -s -X PATCH http://localhost:8004/api/v1/orders/$ORDER_ID/status \
    -H "Authorization: Bearer $OWNER_TOKEN" \
    -H "Content-Type: application/json" \
    -d '{"status":"PREPARED"}' | jq '.'
  echo -e "${YELLOW}── Order after PREPARED ──${NC}"
  curl -s -H "Authorization: Bearer $ACCESS_TOKEN" \
    http://localhost:8004/api/v1/orders/$ORDER_ID | jq '.order'
  echo

  # Test 31d: Internal/Delivery system updates (no JWT)
  echo -e "${BLUE}[TEST 31d] Internal: OUT_FOR_DELIVERY${NC}"
  log_req "PATCH" "http://localhost:8004/api/v1/internal/orders/$ORDER_ID/status" "" '{"status":"OUT_FOR_DELIVERY"}'
  curl -s -X PATCH http://localhost:8004/api/v1/internal/orders/$ORDER_ID/status \
    -H "Content-Type: application/json" \
    -d '{"status":"OUT_FOR_DELIVERY"}' | jq '.'
  echo -e "${YELLOW}── Order after OUT_FOR_DELIVERY ──${NC}"
  curl -s -H "Authorization: Bearer $ACCESS_TOKEN" \
    http://localhost:8004/api/v1/orders/$ORDER_ID | jq '.order'
  echo

  echo -e "${BLUE}[TEST 31e] Internal: DELIVERED${NC}"
  log_req "PATCH" "http://localhost:8004/api/v1/internal/orders/$ORDER_ID/status" "" '{"status":"DELIVERED"}'
  curl -s -X PATCH http://localhost:8004/api/v1/internal/orders/$ORDER_ID/status \
    -H "Content-Type: application/json" \
    -d '{"status":"DELIVERED"}' | jq '.'
  echo -e "${YELLOW}── Order after DELIVERED ──${NC}"
  curl -s -H "Authorization: Bearer $ACCESS_TOKEN" \
    http://localhost:8004/api/v1/orders/$ORDER_ID | jq '.order'
  echo
fi

# Test 32: Place Another Order for Cancel Test
echo -e "${BLUE}[TEST 32] Place Order for Cancel Test${NC}"
CANCEL_IDEMPOTENCY_KEY=$(uuidgen 2>/dev/null || echo "order-cancel-$(date +%s%N)")
CANCEL_ORDER=$(curl -s -X POST http://localhost:8004/api/v1/orders \
  -H "Authorization: Bearer $ACCESS_TOKEN" \
  -H "Content-Type: application/json" \
  -H "Idempotency-Key: $CANCEL_IDEMPOTENCY_KEY" \
  -d '{
    "restaurant_id": '$RESTAURANT_ID',
    "menu_item_id": '$MENU_ITEM_ID',
    "delivery_address": "'"$DELIVERY_ADDRESS"'",
    "notes": "Order to be cancelled"
  }')

echo "$CANCEL_ORDER" | jq '.'
CANCEL_ORDER_ID=$(echo "$CANCEL_ORDER" | jq -r '.order.id // .id // empty')

if [ -n "$CANCEL_ORDER_ID" ]; then
  echo -e "${GREEN}✓ Order for cancel test placed: ID=$CANCEL_ORDER_ID${NC}\n"
  
  # Test 33: Cancel Order
  echo -e "${BLUE}[TEST 33] Cancel Order${NC}"
  log_req "PATCH" "http://localhost:8004/api/v1/orders/$CANCEL_ORDER_ID/cancel" "$ACCESS_TOKEN" ""
  curl -s -X PATCH http://localhost:8004/api/v1/orders/$CANCEL_ORDER_ID/cancel \
    -H "Authorization: Bearer $ACCESS_TOKEN" | jq '.'
  echo
  
  # Verify cancel worked
  echo -e "${BLUE}[TEST 33b] Verify Cancel (should be CANCELLED)${NC}"
  echo -e "${YELLOW}── Order after CANCELLED ──${NC}"
  curl -s -H "Authorization: Bearer $ACCESS_TOKEN" \
    http://localhost:8004/api/v1/orders/$CANCEL_ORDER_ID | jq '.order'
  echo
fi

# Test 34: Try to cancel already confirmed order (should fail)
if [ -n "$ORDER_ID" ]; then
  echo -e "${BLUE}[TEST 34] Try Cancel Confirmed Order (should 422)${NC}"
  CANCEL_CONFIRMED_STATUS=$(curl -s -o /dev/null -w '%{http_code}' -X PATCH \
    http://localhost:8004/api/v1/orders/$ORDER_ID/cancel \
    -H "Authorization: Bearer $ACCESS_TOKEN")
  
  echo "HTTP status: $CANCEL_CONFIRMED_STATUS"
  if [ "$CANCEL_CONFIRMED_STATUS" = "422" ]; then
    echo -e "${GREEN}✓ Cancel correctly rejected for confirmed order (422)${NC}\n"
  else
    echo -e "${RED}⚠ Expected 422, got $CANCEL_CONFIRMED_STATUS${NC}\n"
  fi
fi

# ── Delivery Service Tests ──────────────────────────────────────────────────

# Test 35: Register a DRIVER
echo -e "${BLUE}[TEST 35] Register DRIVER${NC}"
DRIVER_TS=$(date +%s%N | cut -b1-13)
DRIVER_PHONE="+1555$(printf '%07d' $((RANDOM * 32768 + RANDOM)))"
DRIVER_REG=$(curl -s -X POST http://localhost:8001/api/v1/auth/register \
  -H "Content-Type: application/json" \
  -d '{
    "name":"Test Driver",
    "email":"driver'$DRIVER_TS'@test.com",
    "password":"Pass@12345",
    "phone":"'$DRIVER_PHONE'",
    "role":"DRIVER"
  }')

echo "$DRIVER_REG" | jq '.'
DRIVER_TOKEN=$(echo "$DRIVER_REG" | jq -r '.access_token // empty')
DRIVER_AUTH_ID=$(echo "$DRIVER_REG" | jq -r '.user.id // empty')

if [ -z "$DRIVER_TOKEN" ]; then
  echo -e "${RED}❌ Driver registration failed${NC}\n"
else
  echo -e "${GREEN}✓ Driver registered: auth_id=$DRIVER_AUTH_ID${NC}\n"
fi

# Wait for DRIVER_REGISTERED event to be consumed
sleep 3

# Test 36: Delivery Health Check
echo -e "${BLUE}[TEST 36] Delivery Service Health${NC}"
curl -s http://localhost:8005/api/v1/delivery/health | jq '.'
echo

# Test 37: Get Driver Profile (look up by driver table ID)
echo -e "${BLUE}[TEST 37] Get Driver Profile${NC}"
# Fetch driver ID from delivery DB
DRIVER_ID=$(docker exec -i postgres-delivery psql -U postgres -d delivery_db -t -c \
  "SELECT id FROM drivers WHERE auth_id = $DRIVER_AUTH_ID LIMIT 1;" 2>/dev/null | tr -d '[:space:]')

if [ -n "$DRIVER_ID" ]; then
  DRIVER_PROFILE=$(curl -s -H "Authorization: Bearer $DRIVER_TOKEN" \
    http://localhost:8005/api/v1/delivery/driver/$DRIVER_ID)
  echo "$DRIVER_PROFILE" | jq '.'
  echo -e "${GREEN}✓ Driver profile found: id=$DRIVER_ID${NC}\n"
else
  echo -e "${RED}⚠ Driver not found in delivery_db (event may not be consumed yet)${NC}\n"
fi

# Test 38: Update Driver Location
echo -e "${BLUE}[TEST 38] Update Driver Location${NC}"
curl -s -X PATCH http://localhost:8005/api/v1/delivery/location \
  -H "Authorization: Bearer $DRIVER_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"latitude": 17.4000, "longitude": 78.5000}' | jq '.'
echo

# Test 39: Get Driver Orders (should be empty initially)
echo -e "${BLUE}[TEST 39] Get Driver Orders${NC}"
if [ -n "$DRIVER_ID" ]; then
  curl -s -H "Authorization: Bearer $DRIVER_TOKEN" \
    http://localhost:8005/api/v1/delivery/driver/$DRIVER_ID/orders | jq '.'
  echo
fi

# Test 40: Track Order (if a delivery was assigned)
echo -e "${BLUE}[TEST 40] Track Order${NC}"
if [ -n "$ORDER_ID" ]; then
  TRACK_RESP=$(curl -s -H "Authorization: Bearer $DRIVER_TOKEN" \
    http://localhost:8005/api/v1/delivery/track/$ORDER_ID)
  echo "$TRACK_RESP" | jq '.'
  if echo "$TRACK_RESP" | jq -e '.order_id' > /dev/null 2>&1; then
    echo -e "${GREEN}✓ Tracking data available${NC}\n"
  else
    echo -e "${YELLOW}⚠ No delivery assigned for this order yet${NC}\n"
  fi
fi

# ── Cleanup ─────────────────────────────────────────────────────────────────

# Test 41: Delete Menu Item before restaurant (FK constraint)
echo -e "${BLUE}[TEST 41] Delete Menu Item + Restaurant (owner)${NC}"
if [ -n "$MENU_ITEM_ID" ]; then
  curl -s -X DELETE http://localhost:8003/api/v1/restaurants/$RESTAURANT_ID/menu/$MENU_ITEM_ID \
    -H "Authorization: Bearer $OWNER_TOKEN" | jq '.'
fi
curl -s -X DELETE http://localhost:8003/api/v1/restaurants/$RESTAURANT_ID \
  -H "Authorization: Bearer $OWNER_TOKEN" | jq '.'
echo

# ── Auth Cleanup Tests ──────────────────────────────────────────────────────

# Test 42: Delete account (triggers USER_DELETED event)
echo -e "${BLUE}[TEST 42] Delete Account (triggers event)${NC}"
log_req "DELETE" "http://localhost:8001/api/v1/auth/account" "$ACCESS_TOKEN" ""
DELETE_RESPONSE=$(curl -s -X DELETE http://localhost:8001/api/v1/auth/account \
  -H "Authorization: Bearer $ACCESS_TOKEN")

echo "$DELETE_RESPONSE" | jq '.'
echo

# Wait for event processing
sleep 2

# Test 43: Verify profile was deleted and token rejected
echo -e "${BLUE}[TEST 43] Verify Profile Deleted via Event${NC}"
GET_DELETED=$(curl -s -o /dev/null -w '%{http_code}' -H "Authorization: Bearer $ACCESS_TOKEN" \
  http://localhost:8002/api/v1/users/$USER_ID)

echo "HTTP status: $GET_DELETED"
if [ "$GET_DELETED" = "401" ] || [ "$GET_DELETED" = "404" ]; then
  echo -e "${GREEN}✓ Profile access blocked after deletion (HTTP $GET_DELETED)${NC}\n"
else
  echo -e "${RED}⚠ Unexpected status: $GET_DELETED${NC}\n"
fi

# Test 44: Privilege escalation regression — ADMIN role must be rejected
echo -e "${BLUE}[TEST 44] Privilege Escalation Regression (ADMIN role)${NC}"
ADMIN_STATUS=$(curl -s -o /dev/null -w '%{http_code}' -X POST http://localhost:8001/api/v1/auth/register \
  -H 'Content-Type: application/json' \
  -d '{"name":"hacker","email":"hacker@x.com","password":"Pass@12345","phone":"+15559999999","role":"ADMIN"}')
echo "HTTP status: $ADMIN_STATUS"
if [ "$ADMIN_STATUS" = "400" ]; then
  echo -e "${GREEN}✓ ADMIN role correctly rejected (400)${NC}\n"
else
  echo -e "${RED}⚠ Expected 400, got $ADMIN_STATUS${NC}\n"
fi

# Test 45: Deleted user login regression — must be rejected
echo -e "${BLUE}[TEST 45] Deleted User Login Regression${NC}"
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
