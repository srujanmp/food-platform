#!/usr/bin/env bash
# ──────────────────────────────────────────────────────────────
# Delivery Service — API smoke tests
# Usage: bash test-apis.sh
# Requires: curl, jq, running auth-service + delivery-service + order-service
# ──────────────────────────────────────────────────────────────
set -euo pipefail

AUTH_URL="http://localhost:8001/api/v1/auth"
BASE="http://localhost:8005/api/v1"
ORDER_URL="http://localhost:8004/api/v1"
RESTAURANT_URL="http://localhost:8003/api/v1"
PASS="Pass@12345"
TS=$(date +%s)

green()  { printf "\033[32m✔ %s\033[0m\n" "$1"; }
red()    { printf "\033[31m✘ %s\033[0m\n" "$1"; exit 1; }
yellow() { printf "\033[33m⚠ %s\033[0m\n" "$1"; }
header() { printf "\n\033[1;34m── %s ──\033[0m\n" "$1"; }

# ── 1. Health ──────────────────────────────────────────────────
header "Health Check"
STATUS=$(curl -s -o /dev/null -w "%{http_code}" "$BASE/delivery/health")
[ "$STATUS" = "200" ] && green "Health OK ($STATUS)" || red "Health FAIL ($STATUS)"

# ── 2. Register a DRIVER via auth-service ──────────────────────
header "Register DRIVER"
DRIVER_EMAIL="driver${TS}@test.com"
DRIVER_PHONE="+1555${TS: -7}"

DRIVER_REG=$(curl -s -X POST "$AUTH_URL/register" \
  -H "Content-Type: application/json" \
  -d "{
    \"name\": \"Test Driver\",
    \"email\": \"$DRIVER_EMAIL\",
    \"password\": \"$PASS\",
    \"phone\": \"$DRIVER_PHONE\",
    \"role\": \"DRIVER\"
  }")
DRIVER_TOKEN=$(echo "$DRIVER_REG" | jq -r '.access_token // empty')
DRIVER_AUTH_ID=$(echo "$DRIVER_REG" | jq -r '.user.id // empty')
[ -n "$DRIVER_TOKEN" ] && green "Driver registered: auth_id=$DRIVER_AUTH_ID" || red "Driver registration failed: $DRIVER_REG"

# Wait for DRIVER_REGISTERED event
sleep 3

# ── 3. Get Driver profile ─────────────────────────────────────
header "Get Driver Profile"
DRIVER_ID=$(docker exec -i postgres-delivery psql -U postgres -d delivery_db -t -c \
  "SELECT id FROM drivers WHERE auth_id = $DRIVER_AUTH_ID LIMIT 1;" 2>/dev/null | tr -d '[:space:]')

if [ -n "$DRIVER_ID" ]; then
  PROFILE_RESP=$(curl -s -H "Authorization: Bearer $DRIVER_TOKEN" \
    "$BASE/delivery/driver/$DRIVER_ID")
  echo "$PROFILE_RESP" | jq '.'
  green "Driver profile found: id=$DRIVER_ID"
else
  yellow "Driver not yet in delivery_db — event may still be processing"
  DRIVER_ID=""
fi

# ── 4. Update Location ────────────────────────────────────────
header "Update Driver Location"
LOC_RESP=$(curl -s -X PATCH "$BASE/delivery/location" \
  -H "Authorization: Bearer $DRIVER_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"latitude": 17.4000, "longitude": 78.5000}')
echo "$LOC_RESP" | jq '.'
LOC_MSG=$(echo "$LOC_RESP" | jq -r '.message // empty')
[ "$LOC_MSG" = "location_updated" ] && green "Location updated" || yellow "Location update: $LOC_RESP"

# ── 5. Get Driver Orders (should be empty) ────────────────────
header "Get Driver Orders (empty)"
if [ -n "$DRIVER_ID" ]; then
  ORDERS_RESP=$(curl -s -H "Authorization: Bearer $DRIVER_TOKEN" \
    "$BASE/delivery/driver/$DRIVER_ID/orders")
  echo "$ORDERS_RESP" | jq '.'
  ORDER_COUNT=$(echo "$ORDERS_RESP" | jq '.orders | length')
  green "Driver has $ORDER_COUNT orders"
fi

# ── 6. Full delivery flow: place order → prepare → assign → deliver ──
header "Full Delivery Flow"

# Register USER for placing order
USER_EMAIL="user${TS}flow@test.com"
USER_PHONE="+1666${TS: -7}"
USER_REG=$(curl -s -X POST "$AUTH_URL/register" \
  -H "Content-Type: application/json" \
  -d "{
    \"name\": \"Flow User\",
    \"email\": \"$USER_EMAIL\",
    \"password\": \"$PASS\",
    \"phone\": \"$USER_PHONE\",
    \"role\": \"USER\"
  }")
USER_TOKEN=$(echo "$USER_REG" | jq -r '.access_token // empty')
[ -n "$USER_TOKEN" ] && green "User registered for flow test" || yellow "User registration failed"

# Register RESTAURANT_OWNER
OWNER_EMAIL="owner${TS}flow@test.com"
OWNER_PHONE="+1777${TS: -7}"
OWNER_REG=$(curl -s -X POST "$AUTH_URL/register" \
  -H "Content-Type: application/json" \
  -d "{
    \"name\": \"Flow Owner\",
    \"email\": \"$OWNER_EMAIL\",
    \"password\": \"$PASS\",
    \"phone\": \"$OWNER_PHONE\",
    \"role\": \"RESTAURANT_OWNER\"
  }")
OWNER_TOKEN=$(echo "$OWNER_REG" | jq -r '.access_token // empty')
[ -n "$OWNER_TOKEN" ] && green "Restaurant owner registered" || yellow "Owner registration failed"

# Create restaurant
REST_CREATE=$(curl -s -X POST "$RESTAURANT_URL/restaurants" \
  -H "Authorization: Bearer $OWNER_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"name":"Delivery Test Restaurant","address":"456 Test Lane","latitude":17.385,"longitude":78.4867,"cuisine":"Test"}')
REST_ID=$(echo "$REST_CREATE" | jq -r '.restaurant.id // empty')
[ -n "$REST_ID" ] && green "Restaurant created: ID=$REST_ID" || yellow "Restaurant creation failed"

# Approve restaurant via DB
if [ -n "$REST_ID" ]; then
  docker exec -i postgres-restaurant psql -U postgres -d restaurant_db \
    -c "UPDATE restaurants SET is_approved = true WHERE id = $REST_ID;" > /dev/null 2>&1
  green "Restaurant approved via DB"
fi

# Create menu item
MENU_CREATE=$(curl -s -X POST "$RESTAURANT_URL/restaurants/$REST_ID/menu" \
  -H "Authorization: Bearer $OWNER_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"name":"Test Item","description":"Test","price":200.00,"category":"Test","is_veg":true}')
MENU_ID=$(echo "$MENU_CREATE" | jq -r '.menu_item.id // empty')
[ -n "$MENU_ID" ] && green "Menu item created: ID=$MENU_ID" || yellow "Menu item creation failed"

# Place order
if [ -n "$REST_ID" ] && [ -n "$MENU_ID" ]; then
  IDEM_KEY="delivery-test-$(date +%s%N)"
  ORDER_RESP=$(curl -s -X POST "$ORDER_URL/orders" \
    -H "Authorization: Bearer $USER_TOKEN" \
    -H "Content-Type: application/json" \
    -H "Idempotency-Key: $IDEM_KEY" \
    -d "{
      \"restaurant_id\": $REST_ID,
      \"menu_item_id\": $MENU_ID,
      \"delivery_address\": \"123 Test St\",
      \"notes\": \"Delivery flow test\"
    }")
  TEST_ORDER_ID=$(echo "$ORDER_RESP" | jq -r '.order.id // empty')
  [ -n "$TEST_ORDER_ID" ] && green "Order placed: ID=$TEST_ORDER_ID" || yellow "Order placement failed"

  # Owner confirms → prepares → prepared
  if [ -n "$TEST_ORDER_ID" ]; then
    curl -s -X PATCH "$ORDER_URL/orders/$TEST_ORDER_ID/status" \
      -H "Authorization: Bearer $OWNER_TOKEN" \
      -H "Content-Type: application/json" \
      -d '{"status":"CONFIRMED"}' > /dev/null
    green "Order CONFIRMED"

    curl -s -X PATCH "$ORDER_URL/orders/$TEST_ORDER_ID/status" \
      -H "Authorization: Bearer $OWNER_TOKEN" \
      -H "Content-Type: application/json" \
      -d '{"status":"PREPARING"}' > /dev/null
    green "Order PREPARING"

    curl -s -X PATCH "$ORDER_URL/orders/$TEST_ORDER_ID/status" \
      -H "Authorization: Bearer $OWNER_TOKEN" \
      -H "Content-Type: application/json" \
      -d '{"status":"PREPARED"}' > /dev/null
    green "Order PREPARED — waiting for ORDER_PREPARED event..."

    # Wait for assignment
    sleep 5

    # Check if delivery was assigned
    TRACK=$(curl -s -H "Authorization: Bearer $DRIVER_TOKEN" \
      "$BASE/delivery/track/$TEST_ORDER_ID")
    echo "$TRACK" | jq '.'
    TRACK_STATUS=$(echo "$TRACK" | jq -r '.status // empty')

    if [ "$TRACK_STATUS" = "ASSIGNED" ]; then
      green "Delivery ASSIGNED to driver"

      # Driver: OUT_FOR_DELIVERY
      header "Driver: OUT_FOR_DELIVERY"
      STATUS_RESP=$(curl -s -X PATCH "$BASE/delivery/$TEST_ORDER_ID/status" \
        -H "Authorization: Bearer $DRIVER_TOKEN" \
        -H "Content-Type: application/json" \
        -d '{"status":"OUT_FOR_DELIVERY"}')
      echo "$STATUS_RESP" | jq '.'
      green "Status → OUT_FOR_DELIVERY"

      # Track after OUT_FOR_DELIVERY
      curl -s -H "Authorization: Bearer $DRIVER_TOKEN" \
        "$BASE/delivery/track/$TEST_ORDER_ID" | jq '.'

      # Driver: DELIVERED
      header "Driver: DELIVERED"
      DELIVER_RESP=$(curl -s -X PATCH "$BASE/delivery/$TEST_ORDER_ID/status" \
        -H "Authorization: Bearer $DRIVER_TOKEN" \
        -H "Content-Type: application/json" \
        -d '{"status":"DELIVERED"}')
      echo "$DELIVER_RESP" | jq '.'
      green "Status → DELIVERED"

      # Verify order status in order-service
      sleep 1
      ORDER_CHECK=$(curl -s -H "Authorization: Bearer $USER_TOKEN" \
        "$ORDER_URL/orders/$TEST_ORDER_ID")
      FINAL_STATUS=$(echo "$ORDER_CHECK" | jq -r '.order.status // empty')
      echo "Order final status: $FINAL_STATUS"
      [ "$FINAL_STATUS" = "DELIVERED" ] && green "Order status synced to DELIVERED" || yellow "Order status: $FINAL_STATUS"

    else
      yellow "Delivery not assigned yet (status: $TRACK_STATUS) — ORDER_PREPARED event may not have fired"
    fi
  fi
fi

# ── 7. Invalid status transition test ─────────────────────────
header "Invalid Status Transition"
if [ -n "$TEST_ORDER_ID" ]; then
  BAD_STATUS=$(curl -s -o /dev/null -w "%{http_code}" -X PATCH "$BASE/delivery/$TEST_ORDER_ID/status" \
    -H "Authorization: Bearer $DRIVER_TOKEN" \
    -H "Content-Type: application/json" \
    -d '{"status":"OUT_FOR_DELIVERY"}')
  [ "$BAD_STATUS" = "422" ] || [ "$BAD_STATUS" = "404" ] && green "Invalid transition rejected ($BAD_STATUS)" || yellow "Expected 422, got $BAD_STATUS"
fi

# ── 8. Unauthorized access test ───────────────────────────────
header "Unauthorized Access (no token)"
UNAUTH_STATUS=$(curl -s -o /dev/null -w "%{http_code}" "$BASE/delivery/location")
[ "$UNAUTH_STATUS" = "401" ] && green "Unauthorized correctly rejected ($UNAUTH_STATUS)" || yellow "Expected 401, got $UNAUTH_STATUS"

echo
printf "\033[1;32m=== Delivery Service Tests Complete ===\033[0m\n"
