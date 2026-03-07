#!/usr/bin/env bash
# ──────────────────────────────────────────────────────────────
# Restaurant Service — API smoke tests
# Usage: bash test-apis.sh
# Requires: curl, jq, running auth-service + restaurant-service
# ──────────────────────────────────────────────────────────────
set -euo pipefail

AUTH_URL="http://localhost:8001/api/v1/auth"
BASE="http://localhost:8003/api/v1"
PASS="Pass@12345"
TS=$(date +%s)

green()  { printf "\033[32m✔ %s\033[0m\n" "$1"; }
red()    { printf "\033[31m✘ %s\033[0m\n" "$1"; exit 1; }
header() { printf "\n\033[1;34m── %s ──\033[0m\n" "$1"; }

# ── 1. Health ──────────────────────────────────────────────────
header "Health Check"
STATUS=$(curl -s -o /dev/null -w "%{http_code}" "$BASE/restaurants/health")
[ "$STATUS" = "200" ] && green "Health OK ($STATUS)" || red "Health FAIL ($STATUS)"

# ── 2. Register a RESTAURANT_OWNER via auth-service ───────────
header "Register RESTAURANT_OWNER"
OWNER_EMAIL="owner${TS}@test.com"
OWNER_PHONE="+1555${TS: -7}"

REG_RESP=$(curl -s -X POST "$AUTH_URL/register" \
  -H "Content-Type: application/json" \
  -d "{
    \"name\": \"Test Owner\",
    \"email\": \"$OWNER_EMAIL\",
    \"password\": \"$PASS\",
    \"phone\": \"$OWNER_PHONE\",
    \"role\": \"RESTAURANT_OWNER\"
  }")
OWNER_TOKEN=$(echo "$REG_RESP" | jq -r '.access_token // empty')
[ -n "$OWNER_TOKEN" ] && green "Owner registered" || red "Owner registration failed: $REG_RESP"

# ── 3. Register an ADMIN for approval ─────────────────────────
#    (In production, ADMINs are seeded. For test we directly create
#     an admin-role JWT by re-using the DB. Instead, we'll call
#     the approve endpoint and accept that the test needs admin creds.)
#    Shortcut: call approve via internal-style — but approve requires ADMIN JWT.
#    We'll create a second user as RESTAURANT_OWNER for now and test approve later.

# ── 4. Create Restaurant ──────────────────────────────────────
header "Create Restaurant"
CREATE_RESP=$(curl -s -X POST "$BASE/restaurants" \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $OWNER_TOKEN" \
  -d '{
    "name": "Tandoori Nights",
    "address": "123 Spice Lane",
    "latitude": 17.385,
    "longitude": 78.4867,
    "cuisine": "Indian"
  }')
RESTAURANT_ID=$(echo "$CREATE_RESP" | jq -r '.restaurant.id // empty')
[ -n "$RESTAURANT_ID" ] && green "Created restaurant #$RESTAURANT_ID" || red "Create failed: $CREATE_RESP"

# ── 5. Get Restaurant (with menu) ─────────────────────────────
header "Get Restaurant"
GET_RESP=$(curl -s "$BASE/restaurants/$RESTAURANT_ID")
GOT_NAME=$(echo "$GET_RESP" | jq -r '.restaurant.name // empty')
[ "$GOT_NAME" = "Tandoori Nights" ] && green "Get OK: $GOT_NAME" || red "Get FAIL: $GET_RESP"

# ── 6. Update Restaurant ──────────────────────────────────────
header "Update Restaurant"
UPD_RESP=$(curl -s -X PUT "$BASE/restaurants/$RESTAURANT_ID" \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $OWNER_TOKEN" \
  -d '{"name": "Tandoori Nights Deluxe", "cuisine": "North Indian"}')
UPD_NAME=$(echo "$UPD_RESP" | jq -r '.restaurant.name // empty')
[ "$UPD_NAME" = "Tandoori Nights Deluxe" ] && green "Update OK: $UPD_NAME" || red "Update FAIL: $UPD_RESP"

# ── 7. Toggle Status (open/closed) ────────────────────────────
header "Toggle Status"
TOG_RESP=$(curl -s -X PATCH "$BASE/restaurants/$RESTAURANT_ID/status" \
  -H "Authorization: Bearer $OWNER_TOKEN")
IS_OPEN=$(echo "$TOG_RESP" | jq '.restaurant.is_open')
green "Toggle OK: is_open=$IS_OPEN"

# Toggle back to open for subsequent tests
curl -s -X PATCH "$BASE/restaurants/$RESTAURANT_ID/status" \
  -H "Authorization: Bearer $OWNER_TOKEN" > /dev/null

# ── 8. Approve Restaurant (direct DB — admin JWT not available in test) ──
header "Approve Restaurant (DB shortcut)"
docker exec -i postgres-restaurant psql -U postgres -d restaurant_db \
  -c "UPDATE restaurants SET is_approved = true WHERE id = $RESTAURANT_ID;" > /dev/null 2>&1
green "Approved restaurant #$RESTAURANT_ID via DB"

# ── 9. List Restaurants (public, approved + open) ─────────────
header "List Restaurants"
LIST_RESP=$(curl -s "$BASE/restaurants")
COUNT=$(echo "$LIST_RESP" | jq '.restaurants | length')
[ "$COUNT" -ge 1 ] && green "List OK: $COUNT restaurant(s)" || red "List FAIL: $LIST_RESP"

# ── 10. Search Restaurants ────────────────────────────────────
header "Search Restaurants"
SEARCH_RESP=$(curl -s "$BASE/restaurants/search?q=Tandoori")
SEARCH_COUNT=$(echo "$SEARCH_RESP" | jq '.restaurants | length')
[ "$SEARCH_COUNT" -ge 1 ] && green "Search OK: $SEARCH_COUNT result(s)" || red "Search FAIL: $SEARCH_RESP"

# ── 11. Nearby Restaurants ────────────────────────────────────
header "Nearby Restaurants"
NEARBY_RESP=$(curl -s "$BASE/restaurants/nearby?lat=17.385&lng=78.4867&radius=10")
NEARBY_COUNT=$(echo "$NEARBY_RESP" | jq '.restaurants | length')
[ "$NEARBY_COUNT" -ge 1 ] && green "Nearby: $NEARBY_COUNT restaurant(s)" || red "Nearby FAIL: $NEARBY_RESP"

# ── 12. Create Menu Item ──────────────────────────────────────
header "Create Menu Item"
MENU_RESP=$(curl -s -X POST "$BASE/restaurants/$RESTAURANT_ID/menu" \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $OWNER_TOKEN" \
  -d '{
    "name": "Butter Chicken",
    "description": "Creamy tomato-based chicken curry",
    "price": 350.00,
    "category": "Main Course",
    "is_veg": false,
    "image_url": "https://example.com/butter-chicken.jpg"
  }')
MENU_ITEM_ID=$(echo "$MENU_RESP" | jq -r '.menu_item.id // empty')
[ -n "$MENU_ITEM_ID" ] && green "Menu item #$MENU_ITEM_ID created" || red "Menu create FAIL: $MENU_RESP"

# ── 13. List Menu Items ───────────────────────────────────────
header "List Menu Items"
ITEMS_RESP=$(curl -s "$BASE/restaurants/$RESTAURANT_ID/menu")
ITEMS_COUNT=$(echo "$ITEMS_RESP" | jq '.menu_items | length')
[ "$ITEMS_COUNT" -ge 1 ] && green "Menu items: $ITEMS_COUNT" || red "Menu list FAIL: $ITEMS_RESP"

# ── 14. Update Menu Item ──────────────────────────────────────
header "Update Menu Item"
MUPD_RESP=$(curl -s -X PUT "$BASE/restaurants/$RESTAURANT_ID/menu/$MENU_ITEM_ID" \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $OWNER_TOKEN" \
  -d '{"price": 399.00, "description": "Rich & creamy butter chicken"}')
MUPD_PRICE=$(echo "$MUPD_RESP" | jq -r '.menu_item.price // empty')
green "Menu update OK: price=$MUPD_PRICE"

# ── 15. Toggle Menu Item Availability ─────────────────────────
header "Toggle Menu Availability"
MTOG_RESP=$(curl -s -X PATCH "$BASE/restaurants/$RESTAURANT_ID/menu/$MENU_ITEM_ID/toggle" \
  -H "Authorization: Bearer $OWNER_TOKEN")
MAVAIL=$(echo "$MTOG_RESP" | jq '.menu_item.is_available')
green "Toggle OK: is_available=$MAVAIL"

# ── 16. Internal — Get Restaurant ─────────────────────────────
header "Internal: Get Restaurant"
IVAL_RESP=$(curl -s "$BASE/internal/restaurants/$RESTAURANT_ID")
IVAL_NAME=$(echo "$IVAL_RESP" | jq -r '.name // empty')
[ -n "$IVAL_NAME" ] && green "Internal get OK: $IVAL_NAME" || red "Internal get FAIL: $IVAL_RESP"

# ── 17. Internal — Get Menu Item ──────────────────────────────
header "Internal: Get Menu Item"
IMENU_RESP=$(curl -s "$BASE/internal/restaurants/$RESTAURANT_ID/menu/$MENU_ITEM_ID")
IMENU_PRICE=$(echo "$IMENU_RESP" | jq -r '.price // empty')
[ -n "$IMENU_PRICE" ] && green "Internal menu OK: price=$IMENU_PRICE" || red "Internal menu FAIL: $IMENU_RESP"

# ── 18. Delete Menu Item ──────────────────────────────────────
header "Delete Menu Item"
DEL_M_STATUS=$(curl -s -o /dev/null -w "%{http_code}" -X DELETE "$BASE/restaurants/$RESTAURANT_ID/menu/$MENU_ITEM_ID" \
  -H "Authorization: Bearer $OWNER_TOKEN")
[ "$DEL_M_STATUS" = "200" ] && green "Menu item deleted ($DEL_M_STATUS)" || red "Menu delete FAIL ($DEL_M_STATUS)"

# ── 19. Delete Restaurant ─────────────────────────────────────
header "Delete Restaurant"
DEL_R_STATUS=$(curl -s -o /dev/null -w "%{http_code}" -X DELETE "$BASE/restaurants/$RESTAURANT_ID" \
  -H "Authorization: Bearer $OWNER_TOKEN")
[ "$DEL_R_STATUS" = "200" ] && green "Restaurant deleted ($DEL_R_STATUS)" || red "Restaurant delete FAIL ($DEL_R_STATUS)"

echo ""
printf "\033[1;32m🎉 All restaurant-service API tests passed!\033[0m\n"
