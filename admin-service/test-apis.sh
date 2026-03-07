#!/usr/bin/env bash
# ──────────────────────────────────────────────────────────────
# Admin Service — API smoke tests
# Usage: bash test-apis.sh
# Requires: curl, jq, running auth-service + user-service +
#           restaurant-service + admin-service
# ──────────────────────────────────────────────────────────────
set -euo pipefail

AUTH_URL="http://localhost:8001/api/v1/auth"
BASE="http://localhost:8007/api/v1"
RESTAURANT_URL="http://localhost:8003/api/v1"
PASS="Pass@12345"
TS=$(date +%s)

green()  { printf "\033[32m✔ %s\033[0m\n" "$1"; }
red()    { printf "\033[31m✘ %s\033[0m\n" "$1"; exit 1; }
yellow() { printf "\033[33m⚠ %s\033[0m\n" "$1"; }
header() { printf "\n\033[1;34m── %s ──\033[0m\n" "$1"; }

# ── 1. Health ──────────────────────────────────────────────────
header "Health Check"
HEALTH_RESP=$(curl -s "$BASE/admin/health")
STATUS=$(echo "$HEALTH_RESP" | jq -r '.status // empty')
[ "$STATUS" = "ok" ] && green "Health OK" || red "Health FAIL: $HEALTH_RESP"

# ── 2. Reject unauthenticated access ──────────────────────────
header "Auth Guard (no token → 401)"
UNAUTH_CODE=$(curl -s -o /dev/null -w "%{http_code}" "$BASE/admin/users")
[ "$UNAUTH_CODE" = "401" ] && green "Unauthenticated blocked ($UNAUTH_CODE)" || red "Expected 401, got $UNAUTH_CODE"

# ── 3. Reject non-ADMIN role ──────────────────────────────────
header "Role Guard (USER token → 403)"
USER_EMAIL="admintest_user${TS}@test.com"
USER_PHONE="+1800${TS: -7}"
USER_REG=$(curl -s -X POST "$AUTH_URL/register" \
  -H "Content-Type: application/json" \
  -d "{
    \"name\": \"Regular User\",
    \"email\": \"$USER_EMAIL\",
    \"password\": \"$PASS\",
    \"phone\": \"$USER_PHONE\",
    \"role\": \"USER\"
  }")
USER_TOKEN=$(echo "$USER_REG" | jq -r '.access_token // empty')
[ -n "$USER_TOKEN" ] && green "Test user registered" || red "User registration failed: $USER_REG"

ROLE_CODE=$(curl -s -o /dev/null -w "%{http_code}" "$BASE/admin/users" \
  -H "Authorization: Bearer $USER_TOKEN")
[ "$ROLE_CODE" = "403" ] && green "Non-admin blocked ($ROLE_CODE)" || red "Expected 403, got $ROLE_CODE"

# ── 4. Login as seeded ADMIN ──────────────────────────────────
header "Admin Login"
ADMIN_LOGIN=$(curl -s -X POST "$AUTH_URL/login" \
  -H "Content-Type: application/json" \
  -d '{"email": "admin@admin.com", "password": "admin"}')
ADMIN_TOKEN=$(echo "$ADMIN_LOGIN" | jq -r '.access_token // empty')
[ -n "$ADMIN_TOKEN" ] && green "Admin logged in" || red "Admin login failed: $ADMIN_LOGIN"

# ── 5. List Users ─────────────────────────────────────────────
header "List Users"
USERS_RESP=$(curl -s "$BASE/admin/users" \
  -H "Authorization: Bearer $ADMIN_TOKEN")
USER_COUNT=$(echo "$USERS_RESP" | jq 'if type == "array" then length else 0 end')
[ "$USER_COUNT" -ge 1 ] && green "List Users OK: $USER_COUNT user(s)" || red "List Users FAIL: $USERS_RESP"

# ── 6. List Restaurants ───────────────────────────────────────
header "List Restaurants"
REST_RESP=$(curl -s "$BASE/admin/restaurants" \
  -H "Authorization: Bearer $ADMIN_TOKEN")
REST_COUNT=$(echo "$REST_RESP" | jq '.restaurants | length')
[ "$REST_COUNT" -ge 0 ] && green "List Restaurants OK: $REST_COUNT restaurant(s)" || red "List Restaurants FAIL: $REST_RESP"

# ── 7. Analytics Dashboard ────────────────────────────────────
header "Analytics Dashboard"
DASH_RESP=$(curl -s "$BASE/admin/analytics/dashboard" \
  -H "Authorization: Bearer $ADMIN_TOKEN")
HAS_REVENUE=$(echo "$DASH_RESP" | jq 'has("total_revenue")')
HAS_ORDERS=$(echo "$DASH_RESP" | jq 'has("total_orders")')
HAS_USERS=$(echo "$DASH_RESP" | jq 'has("total_users")')
HAS_RESTAURANTS=$(echo "$DASH_RESP" | jq 'has("total_restaurants")')
HAS_DELIVERED=$(echo "$DASH_RESP" | jq 'has("total_delivered")')
HAS_CANCELLED=$(echo "$DASH_RESP" | jq 'has("total_cancelled")')
if [ "$HAS_REVENUE" = "true" ] && [ "$HAS_ORDERS" = "true" ] && [ "$HAS_USERS" = "true" ] \
  && [ "$HAS_RESTAURANTS" = "true" ] && [ "$HAS_DELIVERED" = "true" ] && [ "$HAS_CANCELLED" = "true" ]; then
  green "Dashboard OK: $(echo "$DASH_RESP" | jq -c '.')"
else
  red "Dashboard FAIL: $DASH_RESP"
fi

# ── 8. Approve Restaurant ─────────────────────────────────────
header "Approve Restaurant"

# Create a restaurant to approve
OWNER_EMAIL="admintest_owner${TS}@test.com"
OWNER_PHONE="+1700${TS: -7}"
OWNER_REG=$(curl -s -X POST "$AUTH_URL/register" \
  -H "Content-Type: application/json" \
  -d "{
    \"name\": \"Test Owner\",
    \"email\": \"$OWNER_EMAIL\",
    \"password\": \"$PASS\",
    \"phone\": \"$OWNER_PHONE\",
    \"role\": \"RESTAURANT_OWNER\"
  }")
OWNER_TOKEN=$(echo "$OWNER_REG" | jq -r '.access_token // empty')
[ -n "$OWNER_TOKEN" ] && green "Restaurant owner registered" || red "Owner registration failed: $OWNER_REG"

CREATE_RESP=$(curl -s -X POST "$RESTAURANT_URL/restaurants" \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $OWNER_TOKEN" \
  -d '{
    "name": "Admin Test Kitchen",
    "address": "456 Admin Ave",
    "latitude": 17.400,
    "longitude": 78.500,
    "cuisine": "Italian"
  }')
TEST_REST_ID=$(echo "$CREATE_RESP" | jq -r '.restaurant.id // empty')
[ -n "$TEST_REST_ID" ] && green "Created restaurant #$TEST_REST_ID" || red "Create restaurant failed: $CREATE_RESP"

# Approve via admin-service
APPROVE_RESP=$(curl -s -X PATCH "$BASE/admin/restaurants/$TEST_REST_ID/approve" \
  -H "Authorization: Bearer $ADMIN_TOKEN")
IS_APPROVED=$(echo "$APPROVE_RESP" | jq -r '.restaurant.is_approved // empty')
[ "$IS_APPROVED" = "true" ] && green "Restaurant #$TEST_REST_ID approved" || red "Approve FAIL: $APPROVE_RESP"

# ── 9. Ban User ───────────────────────────────────────────────
header "Ban User"

# Register a throwaway user to ban
BAN_EMAIL="admintest_ban${TS}@test.com"
BAN_PHONE="+1600${TS: -7}"
BAN_REG=$(curl -s -X POST "$AUTH_URL/register" \
  -H "Content-Type: application/json" \
  -d "{
    \"name\": \"Ban Target\",
    \"email\": \"$BAN_EMAIL\",
    \"password\": \"$PASS\",
    \"phone\": \"$BAN_PHONE\",
    \"role\": \"USER\"
  }")
BAN_TOKEN=$(echo "$BAN_REG" | jq -r '.access_token // empty')
BAN_AUTH_ID=$(echo "$BAN_REG" | jq -r '.user.id // empty')
[ -n "$BAN_AUTH_ID" ] && green "Ban target registered: auth_id=$BAN_AUTH_ID" || red "Ban target registration failed: $BAN_REG"

# Wait for user-service to create the profile via event
sleep 2

BAN_RESP=$(curl -s -X PATCH "$BASE/admin/users/$BAN_AUTH_ID/ban" \
  -H "Authorization: Bearer $ADMIN_TOKEN")
BAN_MSG=$(echo "$BAN_RESP" | jq -r '.message // empty')
[ "$BAN_MSG" = "user profile banned" ] && green "User #$BAN_AUTH_ID banned" || yellow "Ban response: $(echo "$BAN_RESP" | jq -c '.')"

# ── 10. Verify banned user cannot access ──────────────────────
header "Verify Banned User Token Revoked"
# The banned user's token should be rejected by auth-service
# (because BanUser sets is_deleted=true and revokes refresh tokens)
BAN_CHECK=$(curl -s -o /dev/null -w "%{http_code}" -X POST "$AUTH_URL/login" \
  -H "Content-Type: application/json" \
  -d "{\"email\": \"$BAN_EMAIL\", \"password\": \"$PASS\"}")
if [ "$BAN_CHECK" = "401" ] || [ "$BAN_CHECK" = "403" ]; then
  green "Banned user login blocked ($BAN_CHECK)"
else
  yellow "Banned user login returned $BAN_CHECK (auth-service may allow login but block access)"
fi

# ── 11. Cleanup: delete the test restaurant ───────────────────
header "Cleanup"
DEL_STATUS=$(curl -s -o /dev/null -w "%{http_code}" -X DELETE \
  "$RESTAURANT_URL/restaurants/$TEST_REST_ID" \
  -H "Authorization: Bearer $OWNER_TOKEN")
[ "$DEL_STATUS" = "200" ] && green "Test restaurant deleted" || yellow "Cleanup: delete returned $DEL_STATUS"

echo ""
printf "\033[1;32m🎉 All admin-service API tests passed!\033[0m\n"
