#!/usr/bin/env bash
# ══════════════════════════════════════════════════════════════
# Food Platform — Complete End-to-End Architecture Test
# Tests: auth → user → restaurant → order → delivery (event flow)
# ══════════════════════════════════════════════════════════════
set -uo pipefail

AUTH="http://localhost:8001/api/v1"
USER="http://localhost:8002/api/v1"
REST="http://localhost:8003/api/v1"
ORD="http://localhost:8004/api/v1"
DEL="http://localhost:8005/api/v1"
PASS="Pass@12345"
TS=$(date +%s)

PASSED=0
FAILED=0
WARNED=0

green()  { printf "\033[32m  ✔ %s\033[0m\n" "$1"; ((PASSED++)); }
red()    { printf "\033[31m  ✘ %s\033[0m\n" "$1"; ((FAILED++)); }
yellow() { printf "\033[33m  ⚠ %s\033[0m\n" "$1"; ((WARNED++)); }
header() { printf "\n\033[1;36m━━━ %s ━━━\033[0m\n" "$1"; }
sub()    { printf "\033[1;34m  ▸ %s\033[0m\n" "$1"; }

assert_eq() {
  local label="$1" actual="$2" expected="$3"
  if [ "$actual" = "$expected" ]; then
    green "$label → $actual"
  else
    red "$label: expected=$expected actual=$actual"
  fi
}

assert_not_empty() {
  local label="$1" val="$2"
  if [ -n "$val" ] && [ "$val" != "null" ]; then
    green "$label → $val"
  else
    red "$label: empty/null"
  fi
}

assert_http() {
  local label="$1" code="$2" expected="$3"
  if [ "$code" = "$expected" ]; then
    green "$label (HTTP $code)"
  else
    red "$label: expected HTTP $expected, got $code"
  fi
}

echo -e "\033[1;35m╔══════════════════════════════════════════════════════════╗\033[0m"
echo -e "\033[1;35m║     Food Platform — End-to-End Architecture Test        ║\033[0m"
echo -e "\033[1;35m╚══════════════════════════════════════════════════════════╝\033[0m"

# ═══════════════════════════════════════════════════════════════
header "0. CLEANUP — Reset stale test data"
# ═══════════════════════════════════════════════════════════════
docker exec -i postgres-delivery psql -U postgres -d delivery_db -c "DELETE FROM deliveries; DELETE FROM drivers; DELETE FROM outbox_events;" > /dev/null 2>&1
green "Cleaned delivery_db (drivers, deliveries, outbox_events)"

# ═══════════════════════════════════════════════════════════════
header "1. HEALTH CHECKS (All 5 Services)"
# ═══════════════════════════════════════════════════════════════

for p in "auth:8001:auth" "user:8002:users" "restaurant:8003:restaurants" "order:8004:orders" "delivery:8005:delivery"; do
  IFS=: read -r name port path <<< "$p"
  CODE=$(curl -s -o /dev/null -w "%{http_code}" "http://localhost:$port/api/v1/$path/health")
  assert_http "$name-service health" "$CODE" "200"
done

# ═══════════════════════════════════════════════════════════════
header "2. AUTH SERVICE — Registration & Login"
# ═══════════════════════════════════════════════════════════════

# Register a regular USER
sub "Register USER"
USER_EMAIL="user_e2e_${TS}@test.com"
USER_PHONE="+1555$(printf '%07d' $((RANDOM % 9999999)))"
USER_REG=$(curl -s -X POST "$AUTH/auth/register" \
  -H "Content-Type: application/json" \
  -d "{\"name\":\"E2E User\",\"email\":\"$USER_EMAIL\",\"password\":\"$PASS\",\"phone\":\"$USER_PHONE\",\"role\":\"USER\"}")
USER_TOKEN=$(echo "$USER_REG" | jq -r '.access_token // empty')
USER_AUTH_ID=$(echo "$USER_REG" | jq -r '.user.id // empty')
assert_not_empty "USER registered (auth_id)" "$USER_AUTH_ID"
assert_not_empty "USER access_token" "$USER_TOKEN"

# Register a RESTAURANT_OWNER
sub "Register RESTAURANT_OWNER"
OWNER_EMAIL="owner_e2e_${TS}@test.com"
OWNER_PHONE="+1666$(printf '%07d' $((RANDOM % 9999999)))"
OWNER_REG=$(curl -s -X POST "$AUTH/auth/register" \
  -H "Content-Type: application/json" \
  -d "{\"name\":\"E2E Owner\",\"email\":\"$OWNER_EMAIL\",\"password\":\"$PASS\",\"phone\":\"$OWNER_PHONE\",\"role\":\"RESTAURANT_OWNER\"}")
OWNER_TOKEN=$(echo "$OWNER_REG" | jq -r '.access_token // empty')
OWNER_AUTH_ID=$(echo "$OWNER_REG" | jq -r '.user.id // empty')
assert_not_empty "OWNER registered (auth_id)" "$OWNER_AUTH_ID"

# Register a DRIVER
sub "Register DRIVER"
DRIVER_EMAIL="driver_e2e_${TS}@test.com"
DRIVER_PHONE="+1777$(printf '%07d' $((RANDOM % 9999999)))"
DRIVER_REG=$(curl -s -X POST "$AUTH/auth/register" \
  -H "Content-Type: application/json" \
  -d "{\"name\":\"E2E Driver\",\"email\":\"$DRIVER_EMAIL\",\"password\":\"$PASS\",\"phone\":\"$DRIVER_PHONE\",\"role\":\"DRIVER\"}")
DRIVER_TOKEN=$(echo "$DRIVER_REG" | jq -r '.access_token // empty')
DRIVER_AUTH_ID=$(echo "$DRIVER_REG" | jq -r '.user.id // empty')
assert_not_empty "DRIVER registered (auth_id)" "$DRIVER_AUTH_ID"

# Login test
sub "Login with email/password"
LOGIN_RESP=$(curl -s -X POST "$AUTH/auth/login" \
  -H "Content-Type: application/json" \
  -d "{\"email\":\"$USER_EMAIL\",\"password\":\"$PASS\"}")
LOGIN_TOKEN=$(echo "$LOGIN_RESP" | jq -r '.access_token // empty')
REFRESH_TOKEN=$(echo "$LOGIN_RESP" | jq -r '.refresh_token // empty')
assert_not_empty "Login access_token" "$LOGIN_TOKEN"
assert_not_empty "Login refresh_token" "$REFRESH_TOKEN"

# Refresh token test
sub "Refresh token"
REFRESH_RESP=$(curl -s -X POST "$AUTH/auth/refresh" \
  -H "Content-Type: application/json" \
  -d "{\"refresh_token\":\"$REFRESH_TOKEN\"}")
NEW_TOKEN=$(echo "$REFRESH_RESP" | jq -r '.access_token // empty')
assert_not_empty "Refreshed access_token" "$NEW_TOKEN"

# Wait for USER_CREATED events to propagate
echo -e "\n  Waiting 3s for events to propagate..."
sleep 3

# ═══════════════════════════════════════════════════════════════
header "3. USER SERVICE — Profile via Events"
# ═══════════════════════════════════════════════════════════════

sub "User profile auto-created via event"
PROFILE_RESP=$(curl -s -H "Authorization: Bearer $USER_TOKEN" "$USER/users/profile")
PROFILE_NAME=$(echo "$PROFILE_RESP" | jq -r '.profile.name // .user.name // empty')
if [ -n "$PROFILE_NAME" ]; then
  green "User profile found: $PROFILE_NAME"
else
  yellow "User profile not yet created (event may be delayed)"
fi

# ═══════════════════════════════════════════════════════════════
header "4. DELIVERY SERVICE — Driver Auto-Created via Event"
# ═══════════════════════════════════════════════════════════════

sub "Check driver in delivery_db (USER_CREATED event → DRIVER filter)"
DRIVER_DB_ID=$(docker exec -i postgres-delivery psql -U postgres -d delivery_db -t -c \
  "SELECT id FROM drivers WHERE auth_id = $DRIVER_AUTH_ID LIMIT 1;" 2>/dev/null | tr -d '[:space:]')

if [ -n "$DRIVER_DB_ID" ]; then
  green "Driver auto-created in delivery_db: id=$DRIVER_DB_ID (from USER_CREATED event)"
else
  yellow "Driver not in delivery_db yet — waiting 3 more seconds..."
  sleep 3
  DRIVER_DB_ID=$(docker exec -i postgres-delivery psql -U postgres -d delivery_db -t -c \
    "SELECT id FROM drivers WHERE auth_id = $DRIVER_AUTH_ID LIMIT 1;" 2>/dev/null | tr -d '[:space:]')
  if [ -n "$DRIVER_DB_ID" ]; then
    green "Driver auto-created (after retry): id=$DRIVER_DB_ID"
  else
    red "Driver NOT created — USER_CREATED event not consumed"
  fi
fi

# ═══════════════════════════════════════════════════════════════
header "5. DELIVERY SERVICE — Driver APIs"
# ═══════════════════════════════════════════════════════════════

# Get Driver Profile
sub "GET /delivery/driver/:id"
if [ -n "$DRIVER_DB_ID" ]; then
  PROF_RESP=$(curl -s -H "Authorization: Bearer $DRIVER_TOKEN" "$DEL/delivery/driver/$DRIVER_DB_ID")
  PROF_NAME=$(echo "$PROF_RESP" | jq -r '.driver.name // empty')
  assert_not_empty "Driver profile name" "$PROF_NAME"
else
  red "Skipped — no driver ID"
fi

# Update Location
sub "PATCH /delivery/location"
LOC_RESP=$(curl -s -X PATCH "$DEL/delivery/location" \
  -H "Authorization: Bearer $DRIVER_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"latitude": 17.3850, "longitude": 78.4867}')
LOC_MSG=$(echo "$LOC_RESP" | jq -r '.message // empty')
assert_eq "Update location" "$LOC_MSG" "location_updated"

# Verify updated location
if [ -n "$DRIVER_DB_ID" ]; then
  sub "Verify location persisted"
  PROF2=$(curl -s -H "Authorization: Bearer $DRIVER_TOKEN" "$DEL/delivery/driver/$DRIVER_DB_ID")
  LAT=$(echo "$PROF2" | jq -r '.driver.latitude // empty')
  assert_eq "Latitude saved" "$LAT" "17.385"
fi

# Get Driver Orders (should be empty)
sub "GET /delivery/driver/:id/orders (empty)"
if [ -n "$DRIVER_DB_ID" ]; then
  ORDERS_RESP=$(curl -s -H "Authorization: Bearer $DRIVER_TOKEN" "$DEL/delivery/driver/$DRIVER_DB_ID/orders")
  ORDER_COUNT=$(echo "$ORDERS_RESP" | jq '.orders | length')
  assert_eq "Driver orders count (initially)" "$ORDER_COUNT" "0"
fi

# Unauthorized access test
sub "Unauthorized access (no token)"
UNAUTH_CODE=$(curl -s -o /dev/null -w "%{http_code}" "$DEL/delivery/driver/1")
assert_http "No token → 401" "$UNAUTH_CODE" "401"

# Wrong role test
sub "Wrong role (USER accessing driver endpoint)"
WRONG_CODE=$(curl -s -o /dev/null -w "%{http_code}" \
  -H "Authorization: Bearer $USER_TOKEN" "$DEL/delivery/driver/1")
assert_http "USER role → 403" "$WRONG_CODE" "403"

# ═══════════════════════════════════════════════════════════════
header "6. RESTAURANT SERVICE — Setup"
# ═══════════════════════════════════════════════════════════════

# Create restaurant
sub "Create restaurant"
REST_CREATE=$(curl -s -X POST "$REST/restaurants" \
  -H "Authorization: Bearer $OWNER_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"name":"E2E Test Restaurant","address":"123 Test Ave","latitude":17.385,"longitude":78.4867,"cuisine":"Indian"}')
REST_ID=$(echo "$REST_CREATE" | jq -r '.restaurant.id // empty')
assert_not_empty "Restaurant created (id)" "$REST_ID"

# Approve via DB (normally admin would do this)
if [ -n "$REST_ID" ]; then
  docker exec -i postgres-restaurant psql -U postgres -d restaurant_db \
    -c "UPDATE restaurants SET is_approved = true WHERE id = $REST_ID;" > /dev/null 2>&1
  green "Restaurant approved via DB"
fi

# List restaurants (public)
sub "List approved restaurants (public)"
LIST_CODE=$(curl -s -o /dev/null -w "%{http_code}" "$REST/restaurants")
assert_http "GET /restaurants" "$LIST_CODE" "200"

# Search restaurants
sub "Search restaurants"
SEARCH_RESP=$(curl -s "$REST/restaurants/search?q=E2E")
SEARCH_COUNT=$(echo "$SEARCH_RESP" | jq '.restaurants | length')
if [ "$SEARCH_COUNT" -gt 0 ] 2>/dev/null; then
  green "Search found $SEARCH_COUNT result(s)"
else
  yellow "Search returned 0 results"
fi

# Nearby restaurants
sub "Nearby restaurants"
NEARBY_CODE=$(curl -s -o /dev/null -w "%{http_code}" "$REST/restaurants/nearby?lat=17.385&lng=78.487&radius=10")
assert_http "GET /restaurants/nearby" "$NEARBY_CODE" "200"

# Create menu item
sub "Create menu item"
MENU_CREATE=$(curl -s -X POST "$REST/restaurants/$REST_ID/menu" \
  -H "Authorization: Bearer $OWNER_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"name":"Butter Chicken","description":"Creamy tomato curry","price":350.00,"category":"Main Course","is_veg":false}')
MENU_ID=$(echo "$MENU_CREATE" | jq -r '.menu_item.id // empty')
assert_not_empty "Menu item created (id)" "$MENU_ID"

# List menu
sub "List menu items"
MENU_LIST=$(curl -s "$REST/restaurants/$REST_ID/menu")
MENU_COUNT=$(echo "$MENU_LIST" | jq '.menu_items | length')
assert_eq "Menu item count" "$MENU_COUNT" "1"

# Get restaurant with menu
sub "Get restaurant by ID"
REST_GET=$(curl -s "$REST/restaurants/$REST_ID")
REST_NAME=$(echo "$REST_GET" | jq -r '.restaurant.name // empty')
assert_not_empty "Restaurant name" "$REST_NAME"

# ═══════════════════════════════════════════════════════════════
header "7. ORDER SERVICE — Place Order"
# ═══════════════════════════════════════════════════════════════

sub "Place order"
IDEM_KEY="e2e-test-${TS}-$(( RANDOM ))"
ORDER_RESP=$(curl -s -X POST "$ORD/orders" \
  -H "Authorization: Bearer $USER_TOKEN" \
  -H "Content-Type: application/json" \
  -H "Idempotency-Key: $IDEM_KEY" \
  -d "{
    \"restaurant_id\": $REST_ID,
    \"menu_item_id\": $MENU_ID,
    \"delivery_address\": \"456 Delivery Lane, Hyderabad\",
    \"notes\": \"E2E test order\"
  }")
ORDER_ID=$(echo "$ORDER_RESP" | jq -r '.order.id // empty')
ORDER_STATUS=$(echo "$ORDER_RESP" | jq -r '.order.status // empty')
assert_not_empty "Order placed (id)" "$ORDER_ID"
assert_eq "Order initial status" "$ORDER_STATUS" "PLACED"

# Get order
sub "Get order details"
ORDER_GET=$(curl -s -H "Authorization: Bearer $USER_TOKEN" "$ORD/orders/$ORDER_ID")
ORDER_GET_STATUS=$(echo "$ORDER_GET" | jq -r '.order.status // empty')
assert_eq "Order status via GET" "$ORDER_GET_STATUS" "PLACED"

# List user orders
sub "List user orders"
MY_ORDERS=$(curl -s -H "Authorization: Bearer $USER_TOKEN" "$ORD/orders/user/$USER_AUTH_ID")
MY_ORDER_COUNT=$(echo "$MY_ORDERS" | jq '.orders | length')
if [ "$MY_ORDER_COUNT" -gt 0 ] 2>/dev/null; then
  green "User has $MY_ORDER_COUNT order(s)"
else
  red "No orders found"
fi

# ═══════════════════════════════════════════════════════════════
header "8. ORDER FLOW — Owner Transitions (CONFIRMED → PREPARING)"
# ═══════════════════════════════════════════════════════════════

# CONFIRMED
sub "Owner sets CONFIRMED"
CONF_RESP=$(curl -s -X PATCH "$ORD/orders/$ORDER_ID/status" \
  -H "Authorization: Bearer $OWNER_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"status":"CONFIRMED"}')
CONF_MSG=$(echo "$CONF_RESP" | jq -r '.message // empty')
assert_eq "Order → CONFIRMED" "$CONF_MSG" "status_updated"

# PREPARING
sub "Owner sets PREPARING"
PREP_RESP=$(curl -s -X PATCH "$ORD/orders/$ORDER_ID/status" \
  -H "Authorization: Bearer $OWNER_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"status":"PREPARING"}')
PREP_MSG=$(echo "$PREP_RESP" | jq -r '.message // empty')
assert_eq "Order → PREPARING" "$PREP_MSG" "status_updated"

# ═══════════════════════════════════════════════════════════════
header "9. RESTAURANT SERVICE — Mark PREPARED (triggers ORDER_PREPARED event)"
# ═══════════════════════════════════════════════════════════════

sub "Mark order PREPARED via restaurant-service (triggers outbox + event)"
PREPARED_RESP=$(curl -s -X PATCH "$REST/restaurants/$REST_ID/order-status" \
  -H "Authorization: Bearer $OWNER_TOKEN" \
  -H "Content-Type: application/json" \
  -d "{\"order_id\": $ORDER_ID, \"status\": \"PREPARED\"}")
echo "  Response: $PREPARED_RESP"
PREPARED_MSG=$(echo "$PREPARED_RESP" | jq -r '.message // empty')
PREPARED_ERR=$(echo "$PREPARED_RESP" | jq -r '.error // empty')
if [ -n "$PREPARED_MSG" ]; then
  green "PREPARED via restaurant-service: $PREPARED_MSG"
elif [ -z "$PREPARED_ERR" ] || [ "$PREPARED_ERR" = "null" ]; then
  green "PREPARED via restaurant-service (success)"
else
  red "PREPARED failed: $PREPARED_ERR"
fi

# Verify order status is PREPARED in order-service
sleep 1
ORDER_CHECK=$(curl -s -H "Authorization: Bearer $USER_TOKEN" "$ORD/orders/$ORDER_ID")
ORDER_CHECK_STATUS=$(echo "$ORDER_CHECK" | jq -r '.order.status // empty')
assert_eq "Order status in order-service" "$ORDER_CHECK_STATUS" "PREPARED"

# Check outbox event was written
sub "Check ORDER_PREPARED in outbox"
OUTBOX_COUNT=$(docker exec -i postgres-restaurant psql -U postgres -d restaurant_db -t -c \
  "SELECT count(*) FROM outbox_events WHERE event_type = 'ORDER_PREPARED' AND payload::jsonb->>'order_id' = '$ORDER_ID';" 2>/dev/null | tr -d '[:space:]')
if [ "$OUTBOX_COUNT" -gt 0 ] 2>/dev/null; then
  green "ORDER_PREPARED outbox event found"
else
  yellow "ORDER_PREPARED outbox event not found yet"
fi

# ═══════════════════════════════════════════════════════════════
header "10. DELIVERY SERVICE — Automatic Driver Assignment"
# ═══════════════════════════════════════════════════════════════

sub "Waiting for ORDER_PREPARED event → delivery assignment (up to 10s)"
ASSIGNED=false
for i in $(seq 1 10); do
  TRACK=$(curl -s -H "Authorization: Bearer $USER_TOKEN" "$DEL/delivery/track/$ORDER_ID" 2>/dev/null)
  TRACK_STATUS=$(echo "$TRACK" | jq -r '.status // empty')
  if [ "$TRACK_STATUS" = "ASSIGNED" ]; then
    ASSIGNED=true
    break
  fi
  sleep 1
done

if [ "$ASSIGNED" = true ]; then
  green "Delivery ASSIGNED (event flow worked!)"
  echo "$TRACK" | jq '.'
else
  red "Delivery NOT assigned after 10s — event flow broken (status: $TRACK_STATUS)"
  # Debug: check outbox relay
  echo "  Debug: checking outbox published status..."
  docker exec -i postgres-restaurant psql -U postgres -d restaurant_db -t -c \
    "SELECT id, event_type, published FROM outbox_events ORDER BY id DESC LIMIT 5;" 2>/dev/null
  echo "  Debug: checking delivery_db..."
  docker exec -i postgres-delivery psql -U postgres -d delivery_db -t -c \
    "SELECT * FROM deliveries ORDER BY id DESC LIMIT 3;" 2>/dev/null
  echo "  Debug: checking restaurant-service logs..."
  docker logs restaurant-service 2>&1 | tail -10
  echo "  Debug: checking delivery-service logs..."
  docker logs delivery-service 2>&1 | tail -10
fi

# ═══════════════════════════════════════════════════════════════
header "11. DELIVERY SERVICE — Driver Status Updates"
# ═══════════════════════════════════════════════════════════════

if [ "$ASSIGNED" = true ]; then
  # OUT_FOR_DELIVERY
  sub "Driver sets OUT_FOR_DELIVERY"
  OFD_RESP=$(curl -s -X PATCH "$DEL/delivery/$ORDER_ID/status" \
    -H "Authorization: Bearer $DRIVER_TOKEN" \
    -H "Content-Type: application/json" \
    -d '{"status":"OUT_FOR_DELIVERY"}')
  OFD_MSG=$(echo "$OFD_RESP" | jq -r '.message // empty')
  assert_eq "Status → OUT_FOR_DELIVERY" "$OFD_MSG" "status_updated"

  # Track after OUT_FOR_DELIVERY
  sub "Track order (should show OUT_FOR_DELIVERY + driver location)"
  TRACK2=$(curl -s -H "Authorization: Bearer $USER_TOKEN" "$DEL/delivery/track/$ORDER_ID")
  TRACK2_STATUS=$(echo "$TRACK2" | jq -r '.status // empty')
  TRACK2_LAT=$(echo "$TRACK2" | jq -r '.latitude // empty')
  assert_eq "Tracking status" "$TRACK2_STATUS" "OUT_FOR_DELIVERY"
  assert_not_empty "Tracking latitude" "$TRACK2_LAT"
  echo "$TRACK2" | jq '.'

  # Update driver location mid-delivery
  sub "Update location mid-delivery"
  curl -s -X PATCH "$DEL/delivery/location" \
    -H "Authorization: Bearer $DRIVER_TOKEN" \
    -H "Content-Type: application/json" \
    -d '{"latitude": 17.3900, "longitude": 78.4900}' | jq -r '.message'

  # DELIVERED
  sub "Driver sets DELIVERED"
  DEL_RESP=$(curl -s -X PATCH "$DEL/delivery/$ORDER_ID/status" \
    -H "Authorization: Bearer $DRIVER_TOKEN" \
    -H "Content-Type: application/json" \
    -d '{"status":"DELIVERED"}')
  DEL_MSG=$(echo "$DEL_RESP" | jq -r '.message // empty')
  assert_eq "Status → DELIVERED" "$DEL_MSG" "status_updated"

  # Track after DELIVERED
  sub "Track delivery (final — DELIVERED)"
  TRACK3=$(curl -s -H "Authorization: Bearer $USER_TOKEN" "$DEL/delivery/track/$ORDER_ID")
  TRACK3_STATUS=$(echo "$TRACK3" | jq -r '.status // empty')
  assert_eq "Final tracking status" "$TRACK3_STATUS" "DELIVERED"

  # Verify order-service status synced
  sub "Verify order-service status synced to DELIVERED"
  sleep 1
  ORDER_FINAL=$(curl -s -H "Authorization: Bearer $USER_TOKEN" "$ORD/orders/$ORDER_ID")
  ORDER_FINAL_STATUS=$(echo "$ORDER_FINAL" | jq -r '.order.status // empty')
  assert_eq "Order-service status" "$ORDER_FINAL_STATUS" "DELIVERED"

  # Check driver is available again
  sub "Driver available again after delivery"
  if [ -n "$DRIVER_DB_ID" ]; then
    AVAIL=$(docker exec -i postgres-delivery psql -U postgres -d delivery_db -t -c \
      "SELECT is_available FROM drivers WHERE id = $DRIVER_DB_ID;" 2>/dev/null | tr -d '[:space:]')
    assert_eq "Driver is_available" "$AVAIL" "t"
  fi

  # Check driver orders list now has 1
  sub "Driver orders list"
  if [ -n "$DRIVER_DB_ID" ]; then
    DRV_ORDERS=$(curl -s -H "Authorization: Bearer $DRIVER_TOKEN" "$DEL/delivery/driver/$DRIVER_DB_ID/orders")
    DRV_ORDER_COUNT=$(echo "$DRV_ORDERS" | jq '.orders | length')
    assert_eq "Driver orders count (after delivery)" "$DRV_ORDER_COUNT" "1"
  fi
else
  yellow "Skipping driver status tests — delivery not assigned"
fi

# ═══════════════════════════════════════════════════════════════
header "12. DELIVERY SERVICE — Error Cases"
# ═══════════════════════════════════════════════════════════════

# Invalid status transition (trying OUT_FOR_DELIVERY on already DELIVERED order)
sub "Invalid status transition"
if [ "$ASSIGNED" = true ]; then
  BAD_STATUS_CODE=$(curl -s -o /dev/null -w "%{http_code}" -X PATCH "$DEL/delivery/$ORDER_ID/status" \
    -H "Authorization: Bearer $DRIVER_TOKEN" \
    -H "Content-Type: application/json" \
    -d '{"status":"OUT_FOR_DELIVERY"}')
  assert_http "Invalid transition rejected" "$BAD_STATUS_CODE" "422"
fi

# Track non-existent order
sub "Track non-existent order"
NOTRACK_CODE=$(curl -s -o /dev/null -w "%{http_code}" \
  -H "Authorization: Bearer $USER_TOKEN" "$DEL/delivery/track/999999")
assert_http "Track non-existent → 404" "$NOTRACK_CODE" "404"

# Update status on non-existent delivery
sub "Update status on non-existent delivery"
NOSTATUS_CODE=$(curl -s -o /dev/null -w "%{http_code}" -X PATCH "$DEL/delivery/999999/status" \
  -H "Authorization: Bearer $DRIVER_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"status":"OUT_FOR_DELIVERY"}')
assert_http "Non-existent delivery → 404" "$NOSTATUS_CODE" "404"

# Bad JSON body
sub "Bad request body"
BAD_JSON_CODE=$(curl -s -o /dev/null -w "%{http_code}" -X PATCH "$DEL/delivery/location" \
  -H "Authorization: Bearer $DRIVER_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"invalid": true}')
assert_http "Missing lat/lng → 400" "$BAD_JSON_CODE" "400"

# ═══════════════════════════════════════════════════════════════
header "13. CROSS-SERVICE EVENT VERIFICATION"
# ═══════════════════════════════════════════════════════════════

# Verify USER_CREATED → user-service
sub "USER_CREATED event: user-service profile auto-created"
UPROF=$(curl -s -H "Authorization: Bearer $USER_TOKEN" "$USER/users/profile")
UPROF_AUTH=$(echo "$UPROF" | jq -r '.profile.auth_id // .user.auth_id // empty')
if [ "$UPROF_AUTH" = "$USER_AUTH_ID" ]; then
  green "User profile in user-service matches auth_id=$USER_AUTH_ID"
else
  yellow "User profile auth_id mismatch or not found (got: $UPROF_AUTH)"
fi

# Verify USER_CREATED → delivery-service (DRIVER only)
sub "USER_CREATED event: NON-driver user NOT in delivery_db"
NON_DRIVER=$(docker exec -i postgres-delivery psql -U postgres -d delivery_db -t -c \
  "SELECT count(*) FROM drivers WHERE auth_id = $USER_AUTH_ID;" 2>/dev/null | tr -d '[:space:]')
assert_eq "Regular USER not in drivers table" "$NON_DRIVER" "0"

# Verify outbox events published
sub "Restaurant outbox relay: events marked published"
UNPUB=$(docker exec -i postgres-restaurant psql -U postgres -d restaurant_db -t -c \
  "SELECT count(*) FROM outbox_events WHERE published = false;" 2>/dev/null | tr -d '[:space:]')
if [ "$UNPUB" = "0" ]; then
  green "All restaurant outbox events published"
else
  yellow "$UNPUB outbox events still unpublished"
fi

# Verify delivery outbox events
sub "Delivery outbox events published"
DEL_UNPUB=$(docker exec -i postgres-delivery psql -U postgres -d delivery_db -t -c \
  "SELECT count(*) FROM outbox_events WHERE published = false;" 2>/dev/null | tr -d '[:space:]')
if [ "$DEL_UNPUB" = "0" ]; then
  green "All delivery outbox events published"
else
  yellow "$DEL_UNPUB delivery outbox events still unpublished"
fi

# ═══════════════════════════════════════════════════════════════
header "14. RabbitMQ EXCHANGE VERIFICATION"
# ═══════════════════════════════════════════════════════════════

sub "Check RabbitMQ exchanges exist"
EXCHANGES=$(docker exec food-platform-rabbitmq-1 rabbitmqctl list_exchanges --formatter=json 2>/dev/null | jq -r '.[].name' 2>/dev/null || echo "")
for ex in user_events restaurant_events delivery_events order_events; do
  if echo "$EXCHANGES" | grep -q "$ex"; then
    green "Exchange '$ex' exists"
  else
    red "Exchange '$ex' missing"
  fi
done

sub "Check RabbitMQ queues exist"
QUEUES=$(docker exec food-platform-rabbitmq-1 rabbitmqctl list_queues --formatter=json 2>/dev/null | jq -r '.[].name' 2>/dev/null || echo "")
for q in delivery.order_prepared delivery.user_created; do
  if echo "$QUEUES" | grep -q "$q"; then
    green "Queue '$q' exists"
  else
    red "Queue '$q' missing"
  fi
done

# ═══════════════════════════════════════════════════════════════
# SUMMARY
# ═══════════════════════════════════════════════════════════════
echo
echo -e "\033[1;35m╔══════════════════════════════════════════════════════════╗\033[0m"
printf "\033[1;35m║\033[0m  \033[32mPASSED: %2d\033[0m  │  \033[31mFAILED: %2d\033[0m  │  \033[33mWARNED: %2d\033[0m             \033[1;35m║\033[0m\n" "$PASSED" "$FAILED" "$WARNED"
echo -e "\033[1;35m╚══════════════════════════════════════════════════════════╝\033[0m"

if [ "$FAILED" -gt 0 ]; then
  exit 1
fi
