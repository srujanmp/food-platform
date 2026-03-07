# Next.js Frontend Design Prompt — Food Ordering Platform

> **How to use this document:** Copy the entire prompt below and paste it into your AI coding assistant (Cursor, GitHub Copilot Chat, ChatGPT, Claude, etc.). The prompt is self-contained and tells the assistant everything it needs to know about the backend before generating a single line of frontend code.

---

## Prompt

You are building a **production-grade food ordering web application** frontend using the **latest stable version of Next.js** (use App Router, React Server Components, and Server Actions where appropriate). Before writing any code, **perform a web search for the latest Next.js documentation** to ensure you are using the most current APIs, conventions, and best practices (e.g. `next/navigation`, `next/image`, metadata API, `use client`, `use server`, route handlers, etc.).

---

### 1 — Tech Stack Requirements

| Layer | Choice | Notes |
|---|---|---|
| Framework | **Next.js (latest)** | App Router only; read latest docs via web search |
| Language | **TypeScript** (strict mode) | All files `.tsx` / `.ts` |
| Styling | **Tailwind CSS** + **shadcn/ui** | Use shadcn components for all UI primitives |
| Data fetching | **TanStack Query v5** | For all client-side data; RSC fetch for static/initial loads |
| State | **Zustand** | Auth store, cart-like ephemeral UI state |
| Forms | **React Hook Form** + **Zod** | Validation mirrors backend constraints below |
| Maps | **Leaflet** or **react-map-gl** | For nearby restaurants and live delivery tracking |
| HTTP client | **axios** instance per service | Base URLs defined in `src/lib/api/` |
| Auth storage | **httpOnly cookies** (preferred) or `localStorage` | Access token + refresh token; never expose to JS if possible |

---

### 2 — Backend Overview (Read This Before Writing a Single Line)

The backend is **7 independent Go microservices**. They all run locally. There is **no API gateway** in the local development setup — the frontend calls each service directly on its own port.

#### 2.1 Service Base URLs (localhost)

```
AUTH_SERVICE        = http://localhost:8001/api/v1
USER_SERVICE        = http://localhost:8002/api/v1
RESTAURANT_SERVICE  = http://localhost:8003/api/v1
ORDER_SERVICE       = http://localhost:8004/api/v1
DELIVERY_SERVICE    = http://localhost:8005/api/v1
NOTIFICATION_SERVICE = http://localhost:8006/api/v1
ADMIN_SERVICE       = http://localhost:8007/api/v1
```

Store these in `src/lib/constants/endpoints.ts` and in `.env.local`:

```env
NEXT_PUBLIC_AUTH_URL=http://localhost:8001/api/v1
NEXT_PUBLIC_USER_URL=http://localhost:8002/api/v1
NEXT_PUBLIC_RESTAURANT_URL=http://localhost:8003/api/v1
NEXT_PUBLIC_ORDER_URL=http://localhost:8004/api/v1
NEXT_PUBLIC_DELIVERY_URL=http://localhost:8005/api/v1
NEXT_PUBLIC_NOTIFICATION_URL=http://localhost:8006/api/v1
NEXT_PUBLIC_ADMIN_URL=http://localhost:8007/api/v1
```

#### 2.2 Authentication — Critical Details

- The backend issues **JWT HS256 access tokens** (default TTL: **15 minutes**) and **refresh tokens** (TTL: **7 days**, stored in Redis on the server).

- On login/register the backend returns:

  ```json
  {
    "access_token": "<jwt>",
    "refresh_token": "<opaque-string>",
    "user": { "id": 1, "email": "...", "phone": "...", "role": "USER" }
  }
  ```

  > ⚠️ The auth response does **not** include a `name` field. After login, make a separate call to `GET /users/:id` (user-service) to fetch the user's display name for the UI.

- Store `access_token` in memory (Zustand) and `refresh_token` in an httpOnly cookie (set via a Next.js Route Handler acting as a BFF proxy, or in localStorage for simplicity).

- All protected endpoints require: `Authorization: Bearer <access_token>`.

- **Token refresh:** call `POST /api/v1/auth/refresh` with body `{ "refresh_token": "..." }` before the access token expires. Implement a **silent refresh** (axios interceptor that catches 401 → refreshes → retries the original request).

- **Logout:** `POST /api/v1/auth/logout` with Bearer token + body `{ "refresh_token": "..." }`. This revokes the refresh token from Redis. Both the Bearer token and the refresh_token body field are required.

- **Roles:** `USER`, `RESTAURANT_OWNER`, `DRIVER`, `ADMIN`. The `ADMIN` role **cannot** be self-registered — it is seeded in the database. Use the `role` field from the decoded JWT (or from the login response) to gate UI sections.

- **Driver auto-registration:** When a user registers with `role=DRIVER`, the auth-service publishes a `USER_CREATED` event. The delivery-service consumes this event and **automatically creates a `Driver` record** in its database. There is no separate driver registration step — the frontend only needs to register with `role=DRIVER` and the backend handles driver record creation. The driver record will be available via `GET /delivery/driver/by-auth/:authId` after a short delay (event propagation).

#### 2.3 User Roles and What Each Role Can Do

| Role | Portal | Key Capabilities |
|---|---|---|
| `USER` | Customer app | Browse restaurants, view menus, place orders, track delivery, view notifications |
| `RESTAURANT_OWNER` | Owner dashboard | Create/manage restaurant, manage menu items, process incoming orders (confirm → preparing → prepared) |
| `DRIVER` | Driver app | View assigned orders, update GPS location, update delivery status (out for delivery → delivered/failed) |
| `ADMIN` | Admin panel | List/ban users, approve restaurants, view analytics dashboard |

Build **4 distinct portal layouts** gated by role, all within the same Next.js app using route groups: `(customer)`, `(owner)`, `(driver)`, `(admin)`.

---

### 3 — Complete API Reference

#### 3.1 Auth Service — `http://localhost:8001/api/v1/auth`

| Method | Path | Auth | Request Body | Response | Notes |
|---|---|---|---|---|---|
| POST | `/register` | None | `{ name, email, password (min 8), phone, role? }` | `{ access_token, refresh_token, user: { id, email, phone, role } }` | role defaults to `USER`; `ADMIN` is rejected with 400 |
| POST | `/login` | None | `{ email, password }` | `{ access_token, refresh_token, user }` | 401 if credentials wrong or account soft-deleted |
| POST | `/refresh` | None | `{ refresh_token }` | `{ access_token, refresh_token, user }` | 401 if token invalid/expired |
| POST | `/logout` | Bearer | `{ refresh_token }` | `{ message: "logged out successfully" }` | Revokes refresh token from Redis |
| POST | `/otp/send` | None | `{ phone }` | `{ message: "OTP sent successfully" }` | Sends OTP to phone |
| POST | `/otp/verify` | None | `{ phone, code (6 digits) }` | `{ access_token, refresh_token, user }` | Sets `is_verified=true` |
| DELETE | `/account` | Bearer | — | `{ message: "account deleted successfully" }` | Soft-delete; fires USER_DELETED event |
| GET | `/health` | None | — | `{ service, status, uptime }` | Health check |

**Validation rules to mirror in Zod:**

- `email`: valid email format
- `password`: minimum 8 characters
- `phone`: required for registration
- `name`: required for registration
- `role`: one of `USER`, `RESTAURANT_OWNER`, `DRIVER` (never `ADMIN`)
- OTP `code`: exactly 6 characters

---

#### 3.2 User Service — `http://localhost:8002/api/v1`

The `:id` parameter in all user endpoints is the **auth service user ID** (`auth_db.users.id`), not a separate profile ID.

| Method | Path | Auth | Request Body | Response | Notes |
|---|---|---|---|---|---|
| GET | `/users/:id` | Bearer + Owner or ADMIN | — | `{ auth_id, name, avatar_url, created_at }` | Only the profile owner or an ADMIN can view. Returns 403 for other authenticated users. |
| PUT | `/users/:id` | Bearer + Owner or ADMIN | `{ name?, avatar_url? }` | Updated profile object | Only the owning user (or ADMIN). **Note: `phone` is stored in auth-service only — it cannot be updated here.** |
| GET | `/users/:id/addresses` | Bearer + Owner or ADMIN | — | `[address, address, ...]` | Returns a **bare JSON array**, not wrapped in an object. Lists all addresses. |

> ⚠️ There is **no `DELETE /users/:id` endpoint**. Account deletion is handled **exclusively** through `DELETE /auth/account` on the auth-service (section 3.1). When called, the auth-service fires a `USER_DELETED` event, and the user-service consumes that event to soft-delete the profile asynchronously. The frontend must **never** attempt to call `DELETE /users/:id` — it does not exist.
| POST | `/users/:id/addresses` | Bearer + Owner or ADMIN | `{ label, line1, city, pincode, latitude?, longitude?, is_default? }` | Created address object | |
| PUT | `/users/addresses/:addressId` | Bearer + Owner or ADMIN | `{ label?, line1?, city?, pincode?, latitude?, longitude?, is_default? }` | Updated address object | |
| DELETE | `/users/addresses/:addressId` | Bearer + Owner or ADMIN | — | `{ message: "address deleted" }` | **Soft delete** (uses GORM's `DeletedAt` mechanism — the row is marked deleted, not physically removed) |
| GET | `/users/health` | None | — | `{ service, status }` | |

**Profile response shape:**

```json
{
  "auth_id": 42,
  "name": "John Doe",
  "avatar_url": "https://example.com/avatar.jpg",
  "created_at": "2025-01-15T10:30:00Z"
}
```

> ⚠️ There is **no `id` field** — `auth_id` is the primary key. The profile response does **not** include `phone` (phone is stored in auth-service only).

**Address object shape:**

```json
{
  "id": 1,
  "auth_id": 42,
  "label": "Home",
  "line1": "123 Main St",
  "city": "Mumbai",
  "pincode": "400001",
  "latitude": 19.076,
  "longitude": 72.877,
  "is_default": true,
  "created_at": "...",
  "updated_at": "..."
}
```

---

#### 3.3 Restaurant Service — `http://localhost:8003/api/v1`

##### Restaurants

| Method | Path | Auth | Request Body | Response | Notes |
|---|---|---|---|---|---|
| GET | `/restaurants` | None | — | `{ restaurants: [...] }` | Only `is_approved=true` + `is_open=true` |
| GET | `/restaurants/:id` | None | — | `{ restaurant: { ...fields, menu_items: [...] } }` | Full details including menu |
| GET | `/restaurants/search?q=` | None | — | `{ restaurants: [...] }` | Search by name or cuisine (approved only) |
| GET | `/restaurants/nearby?lat=&lng=&radius=` | None | — | `{ restaurants: [...] }` | radius in km; approved + open only |
| POST | `/restaurants` | Bearer + RESTAURANT_OWNER | `{ name, address?, latitude?, longitude?, cuisine? }` | `{ restaurant: {...} }` | Starts as `is_approved=false` |
| PUT | `/restaurants/:id` | Bearer + Owner | `{ name?, address?, latitude?, longitude?, cuisine? }` | `{ restaurant: {...} }` | |
| DELETE | `/restaurants/:id` | Bearer + Owner | — | `{ message: "restaurant deactivated" }` | |
| PATCH | `/restaurants/:id/status` | Bearer + Owner | — | `{ restaurant: {...} }` | Toggles `is_open` |
| PATCH | `/restaurants/:id/order-status` | Bearer + Owner | `{ order_id, status }` | `{ message: "order status updated to ..." }` | status: `CONFIRMED`, `PREPARING`, `PREPARED` |

**Restaurant object shape:**

```json
{
  "id": 1,
  "owner_id": 42,
  "name": "Spice Garden",
  "address": "MG Road, Bangalore",
  "latitude": 12.9716,
  "longitude": 77.5946,
  "cuisine": "Indian",
  "avg_rating": 4.2,
  "is_open": true,
  "is_approved": true,
  "created_at": "...",
  "updated_at": "...",
  "menu_items": []
}
```

##### Menu Items

| Method | Path | Auth | Request Body | Response | Notes |
|---|---|---|---|---|---|
| GET | `/restaurants/:id/menu` | None | — | `{ menu_items: [...] }` | |
| POST | `/restaurants/:id/menu` | Bearer + Owner | `{ name, description?, price (>0), category?, is_veg?, image_url? }` | `{ menu_item: {...} }` | ⚠️ `is_veg` defaults to `true` if omitted — always send this field explicitly in the form |
| PUT | `/restaurants/:id/menu/:itemId` | Bearer + Owner | `{ name?, description?, price?, category?, is_veg?, image_url? }` | `{ menu_item: {...} }` | |
| DELETE | `/restaurants/:id/menu/:itemId` | Bearer + Owner | — | `{ message: "menu item removed" }` | |
| PATCH | `/restaurants/:id/menu/:itemId/toggle` | Bearer + Owner | — | `{ menu_item: {...} }` | Toggles `is_available` |

**MenuItem object shape:**

```json
{
  "id": 10,
  "restaurant_id": 1,
  "name": "Butter Chicken",
  "description": "Creamy tomato gravy",
  "price": 280.00,
  "category": "Main",
  "is_veg": false,
  "is_available": true,
  "image_url": "https://...",
  "created_at": "...",
  "updated_at": "..."
}
```

---

#### 3.4 Order Service — `http://localhost:8004/api/v1`

**Important:** There is **no cart**. Users place a direct single-item order via "Order Now". Payment is captured inline (simulated gateway — no real Razorpay/Stripe keys needed in dev).

**Important:** Every `POST /orders` call MUST include an `Idempotency-Key` header (a UUID generated by the client, e.g. `crypto.randomUUID()`). Retrying with the same key returns 409 Conflict instead of placing a duplicate order.

| Method | Path | Auth | Request Body / Headers | Response | Notes |
|---|---|---|---|---|---|
| POST | `/orders` | Bearer | Header: `Idempotency-Key: <uuid>`<br>Body: `{ restaurant_id, menu_item_id, delivery_address, notes? }` | `{ order: {...} }` | 201 Created; publishes ORDER_PLACED event |
| GET | `/orders/:id` | Bearer | — | `{ order: {...} }` | Order details only (no payment info included). **Note:** No ownership check — any authenticated user can fetch any order by ID. The frontend should only link to orders belonging to the current user. |
| GET | `/orders/user/:userId` | Bearer + Owner | — | `{ orders: [...] }` | Order history, sorted by `created_at DESC` |
| GET | `/orders/restaurant/:restaurantId` | Bearer + Owner | — | `{ orders: [...] }` | Orders for restaurant owners |
| PATCH | `/orders/:id/cancel` | Bearer | — | `{ message: "cancelled" }` | Only if `status=PLACED`; sets payment to REFUNDED |
| PATCH | `/orders/:id/status` | Bearer + RESTAURANT_OWNER | `{ status }` | `{ message: "status_updated" }` | **Direct route exists** but do NOT use from the frontend — use the restaurant-service `PATCH /restaurants/:id/order-status` instead (section 12.3), which also triggers the `ORDER_PREPARED` outbox event for driver assignment |

**Order object shape:**

```json
{
  "id": 99,
  "user_id": 42,
  "restaurant_id": 1,
  "menu_item_id": 10,
  "item_name": "Butter Chicken",
  "item_price": 280.00,
  "status": "PLACED",
  "delivery_address": "123 Main St, Mumbai",
  "notes": "Extra spicy",
  "created_at": "...",
  "updated_at": "..."
}
```

> ⚠️ The order response does **not** include payment information. Payment is handled internally by the backend and is not exposed via any public API endpoint.

**Order Status Flow (frontend must respect this):**

```
PLACED → CONFIRMED → PREPARING → PREPARED → OUT_FOR_DELIVERY → DELIVERED
  └── CANCELLED  (only from PLACED, before restaurant confirms)
      FAILED     (terminal, set by driver)
```

Show a visual status stepper on the order tracking page using this exact sequence.

---

#### 3.5 Delivery Service — `http://localhost:8005/api/v1`

| Method | Path | Auth | Request Body | Response | Notes |
|---|---|---|---|---|---|
| GET | `/delivery/driver/by-auth/:authId` | Bearer + DRIVER | — | `{ driver: {...}, active_order: {...} \| null }` | **Resolves internal driver ID from auth ID.** Use this after DRIVER login to get `driver.id` for subsequent calls. |
| GET | `/delivery/driver/:driverId` | Bearer + DRIVER | — | `{ driver: {...}, active_order: {...} \| null }` | Driver profile + active order |
| GET | `/delivery/driver/:driverId/orders` | Bearer + DRIVER | — | `{ orders: [...] }` | Driver's delivery history |
| PATCH | `/delivery/location` | Bearer + DRIVER | `{ latitude, longitude }` | `{ message: "location_updated" }` | Called on a timer (every 5–10s) from driver app. **The driver is identified from the JWT `user_id` claim** — no `driverId` in the path or body. The backend resolves the driver record internally via `auth_id`. |
| PATCH | `/delivery/:orderId/status` | Bearer + DRIVER | `{ status }` | `{ message: "status_updated" }` | status: `OUT_FOR_DELIVERY` or `DELIVERED` or `FAILED`. Only the assigned driver can update. |
| GET | `/delivery/track/:orderId` | Bearer | — | `{ order_id, status, latitude, longitude }` | Live tracking; poll every 5s. **Response is flat — no nested driver object.** **Note:** No ownership check — any authenticated user can track any order. The frontend should only expose tracking links for the current user's own orders. |
| GET | `/delivery/health` | None | — | `{ service, status }` | |

**Driver object shape (returned by both `/driver/:driverId` and `/driver/by-auth/:authId`):**

```json
{
  "driver": {
    "id": 5,
    "auth_id": 42,
    "name": "Ravi Kumar",
    "phone": "+919876543210",
    "latitude": 12.9716,
    "longitude": 77.5946,
    "is_available": true,
    "created_at": "...",
    "updated_at": "..."
  },
  "active_order": {
    "id": 10,
    "order_id": 99,
    "driver_id": 5,
    "user_id": 42,
    "status": "ASSIGNED",
    "assigned_at": "...",
    "delivered_at": null,
    "created_at": "...",
    "updated_at": "...",
    "driver": null
  }
}
```

> `active_order` is `null` if the driver has no current delivery assignment. When present, the `active_order.driver` field is always `null` (GORM artifact — ignore it).

**Delivery status values:** `ASSIGNED`, `OUT_FOR_DELIVERY`, `DELIVERED`, `FAILED`

**Valid driver status transitions:**
- `ASSIGNED` → `OUT_FOR_DELIVERY`
- `OUT_FOR_DELIVERY` → `DELIVERED`
- `OUT_FOR_DELIVERY` → `FAILED`

**Tracking response shape:**

```json
{
  "order_id": 99,
  "status": "OUT_FOR_DELIVERY",
  "latitude": 12.9720,
  "longitude": 77.5950
}
```

> ⚠️ Driver name and phone are **not** available from the tracking endpoint. Do not render driver contact info on the live tracking page.

---

#### 3.6 Notification Service — `http://localhost:8006/api/v1`

| Method | Path | Auth | Request Body | Response | Notes |
|---|---|---|---|---|---|
| GET | `/notifications/user/:userId` | Bearer | — | `{ notifications: [...] }` | Persistent inbox; newest first. **Response is wrapped — use `response.notifications`, not the root array** |
| PATCH | `/notifications/:id/read` | Bearer | — | `{ message: "marked_as_read" }` | Mark single notification read (note: underscore-separated) |
| GET | `/notifications/health` | None | — | `{ service, status }` | |

**Notification object shape:**

```json
{
  "id": 7,
  "user_id": 42,
  "event_type": "ORDER_PLACED",
  "title": "Order Placed",
  "body": "Order placed and payment captured. Waiting for restaurant to confirm.",
  "is_read": false,
  "created_at": "..."
}
```

**Events and their notification text (for UI display):**

| Event | Notification to Customer |
|---|---|
| `ORDER_PLACED` | "Order placed and payment captured. Waiting for restaurant to confirm." |
| `DRIVER_ASSIGNED` | "Driver [name] has been assigned and is on the way." |
| `ORDER_DELIVERED` | "Your order has been delivered. Enjoy your meal!" |
| `ORDER_FAILED` | "Delivery failed. A refund has been initiated." |
| `ORDER_CANCELLED` | "Order cancelled. Refund will appear in 3–5 business days." |

Poll `GET /notifications/user/:userId` every 15 seconds to show a live notification badge in the navbar.

---

#### 3.7 Admin Service — `http://localhost:8007/api/v1`

All endpoints require `role=ADMIN` in the JWT. The admin panel is a separate route group.

| Method | Path | Auth | Request Body | Response | Notes |
|---|---|---|---|---|---|
| GET | `/admin/users` | Bearer + ADMIN | — | `[{...}, {...}, ...]` | **Bare JSON array** of user profile objects (not wrapped). **Proxied from user-service** — if user-service is down, returns 503 or proxy error. Handle gracefully in the UI. |
| PATCH | `/admin/users/:id/ban` | Bearer + ADMIN | — | `{ message: "user profile banned" }` | Revokes all tokens via auth-service AND soft-deletes profile via user-service |
| GET | `/admin/restaurants` | Bearer + ADMIN | — | `{ restaurants: [...] }` | **Wrapped object.** Returns **all** restaurants including unapproved and closed ones. |
| PATCH | `/admin/restaurants/:id/approve` | Bearer + ADMIN | — | `{ restaurant: {...} }` | **Proxied to restaurant-service** `PATCH /restaurants/:id/approve` with the admin's JWT. Errors originate from restaurant-service upstream (e.g., 404 if restaurant not found). |
| GET | `/admin/analytics/dashboard` | Bearer + ADMIN | — | `{ total_orders, total_revenue, total_delivered, total_cancelled, total_users, total_restaurants }` | Use exact field names — no `revenue` or `orders` shorthand. **Graceful degradation:** if any upstream service is down, the affected fields return `0` instead of an error. The frontend should be aware that zero values may indicate "no data" OR "service unavailable" — consider showing a subtle warning if all values are zero. |
| GET | `/admin/health` | None | — | `{ service, status, uptime }` | |

**Admin dashboard response shape:**

```json
{
  "total_orders": 150,
  "total_revenue": 42500.00,
  "total_delivered": 120,
  "total_cancelled": 10,
  "total_users": 85,
  "total_restaurants": 12
}
```

---

### 4 — Error Handling

All services return errors in this shape:

```json
{ "error": "snake_case_code", "message": "optional human-readable detail" }
```

**Common HTTP status → UI mapping:**

| Status | Error Code | UI Action |
|---|---|---|
| 400 | `validation_failed` | Show inline field errors from `message` |
| 401 | `invalid_credentials` / `invalid_or_expired_token` | Redirect to `/login`; clear tokens |
| 403 | `not_restaurant_owner` / `forbidden` / `insufficient_permissions` | Show "Access denied" toast |
| 404 | `*_not_found` | Show empty state / 404 page |
| 409 | `duplicate_request` | Silently ignore (idempotency replay) |
| 409 | `user_with_this_email_already_exists` / `user_with_this_phone_already_exists` | Show specific registration error: "An account with this email already exists" or "An account with this phone already exists" |
| 422 | `invalid_status_transition` / `restaurant_unavailable` / `menu_item_unavailable` | Show specific error toast |
| 429 | `rate_limit_exceeded` | Show "Too many requests" toast; respect `Retry-After` header |
| 503 | `service_unavailable` | Show "Service temporarily unavailable" banner |
| 500 | `internal_server_error` | Show generic error toast; log to console |

Implement a global Axios response interceptor that:

1. Catches 401 → attempts silent token refresh → retries original request once.
2. If refresh also fails → clears all auth state → redirects to `/login`.
3. All other errors → dispatches to a global error boundary / toast system.

---

### 5 — Page & Route Architecture

Use Next.js App Router with the following route groups:

```
src/app/
├── (auth)/
│   ├── login/page.tsx
│   ├── register/page.tsx
│   └── otp/page.tsx
│
├── (customer)/
│   ├── layout.tsx                  ← Navbar, notification bell, footer
│   ├── page.tsx                    ← Home: hero, featured restaurants
│   ├── restaurants/
│   │   ├── page.tsx                ← List all approved+open restaurants
│   │   └── [id]/page.tsx           ← Restaurant detail + menu
│   ├── orders/
│   │   ├── page.tsx                ← Order history list
│   │   └── [id]/
│   │       ├── page.tsx            ← Order detail + status stepper
│   │       └── track/page.tsx      ← Live delivery map (polls /delivery/track/:id)
│   ├── profile/
│   │   ├── page.tsx                ← View/edit profile
│   │   └── addresses/page.tsx      ← Manage delivery addresses
│   └── notifications/page.tsx      ← Full notification inbox
│
├── (owner)/
│   ├── layout.tsx                  ← Owner sidebar nav
│   ├── dashboard/page.tsx          ← Stats overview
│   ├── restaurant/
│   │   ├── page.tsx                ← My restaurant details + edit
│   │   ├── create/page.tsx         ← Create restaurant form (pending approval)
│   │   └── menu/page.tsx           ← Menu items CRUD
│   └── orders/page.tsx             ← Incoming orders list + status update buttons
│
├── (driver)/
│   ├── layout.tsx                  ← Driver sidebar
│   ├── dashboard/page.tsx          ← Active order + map
│   └── history/page.tsx            ← Past deliveries
│
├── (admin)/
│   ├── layout.tsx                  ← Admin sidebar
│   ├── dashboard/page.tsx          ← Analytics dashboard
│   ├── users/page.tsx              ← User list + ban
│   └── restaurants/page.tsx        ← Restaurant list + approve
│
└── api/                            ← Next.js Route Handlers (BFF layer)
    └── auth/
        └── refresh/route.ts        ← Proxy for silent token refresh
```

---

### 6 — State Management

#### Auth Store (Zustand)

```typescript
interface AuthState {
  user: { id: number; email: string; phone: string; role: string } | null;
  accessToken: string | null;
  driverId: number | null;          // resolved after driver login (see section 12.4)
  isAuthenticated: boolean;
  setAuth: (user, accessToken) => void;
  setDriverId: (driverId: number) => void;
  clearAuth: () => void;
}
```

Persist the access token in memory only. Persist the refresh token in an httpOnly cookie using a Next.js Route Handler.

#### Key Data Flows

- **Order placement:** `crypto.randomUUID()` → store idempotency key → `POST /orders` with `Idempotency-Key` header → invalidate `orders` query on success.
- **Real-time tracking:** use `setInterval` (5s) inside a `useEffect` on the tracking page to poll `GET /delivery/track/:orderId`. Clear interval on unmount. Stop polling when status reaches `DELIVERED` or `FAILED`.
- **Notification badge:** poll `GET /notifications/user/:userId` every 15s; count `is_read=false` items.
- **Driver location:** `setInterval` (10s) inside a `useEffect` on the driver dashboard to call `PATCH /delivery/location` with the browser's Geolocation API result.

---

### 7 — Key UI Components to Build

| Component | Description |
|---|---|
| `<RestaurantCard>` | Image, name, cuisine, rating, open/closed badge |
| `<MenuItemCard>` | Image, name, description, price, veg/non-veg badge, "Order Now" button |
| `<OrderNowModal>` | Address selector (from saved addresses), notes field, confirm button |
| `<OrderStatusStepper>` | Visual horizontal stepper showing all 6 statuses; current highlighted |
| `<LiveTrackingMap>` | Leaflet map with driver marker; updates on poll |
| `<NotificationBell>` | Navbar icon with unread count badge; dropdown with latest 5 |
| `<RestaurantOwnerOrderCard>` | Order details + action buttons (Confirm / Preparing / Prepared) |
| `<DriverOrderCard>` | Order + customer address + action buttons (Out for delivery / Delivered / Failed) |
| `<AdminRestaurantRow>` | Restaurant info + "Approve" button (disabled if already approved) |
| `<AdminUserRow>` | User info + "Ban" button |

---

### 8 — TypeScript Types (derive from the API shapes above)

Generate a `src/types/api.ts` file that includes:

- `AuthResponse`, `UserResponse`, `LoginRequest`, `RegisterRequest`
- `Profile` — **shape: `{ auth_id: number; name: string; avatar_url: string; created_at: string }`** (no `id` field; `auth_id` is the primary key)
- `Address`, `AddAddressRequest`
- `Restaurant`, `MenuItem`, `CreateRestaurantRequest`, `CreateMenuItemRequest`
- `Order`, `PlaceOrderRequest`, `OrderStatus` (union type of all statuses: `PLACED | CONFIRMED | PREPARING | PREPARED | OUT_FOR_DELIVERY | DELIVERED | CANCELLED | FAILED`)
- `Driver` — **shape: `{ id: number; auth_id: number; name: string; phone: string; latitude: number; longitude: number; is_available: boolean; created_at: string; updated_at: string }`**
- `Delivery` — **shape: `{ id: number; order_id: number; driver_id: number; user_id: number; status: string; assigned_at: string; delivered_at: string | null; created_at: string; updated_at: string; driver?: Driver | null }`** (the `driver` field is a GORM artifact and will always be `null` — ignore it)
- `DriverResponse` — **shape: `{ driver: Driver; active_order: Delivery | null }`**
- `TrackingResponse` — **shape: `{ order_id: number; status: OrderStatus; latitude: number; longitude: number }`** (flat, no nested driver object)
- `UpdateLocationRequest`
- `Notification`, `NotificationEventType` (union of all event type strings: `ORDER_PLACED | DRIVER_ASSIGNED | ORDER_DELIVERED | ORDER_FAILED | ORDER_CANCELLED`)
- `NotificationListResponse` — **shape: `{ notifications: Notification[] }`** (wrapped object)
- `AdminDashboard` — **exact shape: `{ total_orders: number; total_revenue: number; total_delivered: number; total_cancelled: number; total_users: number; total_restaurants: number }`**
- `ApiError` (shape: `{ error: string; message?: string }`)

> ⚠️ Do **not** generate a `Payment` type — payment data is not exposed by any public API endpoint. Payment is handled internally by the order-service.

---

### 9 — Specific Implementation Requirements

1. **No cart service exists.** "Add to cart" patterns are wrong. The UI shows a single "Order Now" button per menu item that opens a modal to confirm address and notes, then immediately places the order.

2. **Idempotency is mandatory.** Every call to `POST /orders` MUST include a fresh `Idempotency-Key: <uuid>` header. If the user accidentally submits twice (double-click), the second call returns 409 — handle this silently (don't show an error).

3. **Price is snapshotted at order time.** The order response contains `item_name` and `item_price` as they were at placement time. Do not try to look up the current menu item price for historical orders — always use the snapshotted values from the order object.

4. **ADMIN role cannot register.** The registration form must not offer "Admin" as a role option. The backend will return 400 if tried.

5. **Restaurants start unapproved.** After a `RESTAURANT_OWNER` creates a restaurant, it starts with `is_approved=false`. Show a "Pending approval" banner in the owner dashboard until `is_approved=true`. The public listing (`GET /restaurants`) will never show it until approved.

6. **Order cancellation window.** A customer can only cancel an order when `status=PLACED`. Once the restaurant confirms, cancellation is no longer allowed. Show the "Cancel Order" button only when `status=PLACED`.

7. **Driver GPS polling.** The driver dashboard must use the browser `navigator.geolocation.watchPosition` API and push location updates to `PATCH /delivery/location` every 10 seconds. Stop the watch on component unmount.

8. **Live tracking polling.** The customer tracking page polls `GET /delivery/track/:orderId` every 5 seconds via `setInterval`. Stop polling when status reaches `DELIVERED` or `FAILED`.

9. **Soft-deleted accounts.** If login **or OTP verification** returns 401 with `error: "account_deleted"`, show a specific message: "This account has been deactivated." Do not show the generic "Invalid credentials" message. Handle this on both the login page and the OTP verification page.

10. **Role-based routing.** After login, redirect based on role:
    - `USER` → `/restaurants`
    - `RESTAURANT_OWNER` → `/owner/dashboard`
    - `DRIVER` → `/driver/dashboard`
    - `ADMIN` → `/admin/dashboard`

---

### 10 — CORS Note

The Go backends run with CORS enabled for development (allow all origins). If you hit CORS errors in the browser, add a Next.js `next.config.ts` `rewrites` rule to proxy all backend calls through `/api/proxy/*` to avoid CORS issues entirely:

```typescript
// next.config.ts
const nextConfig = {
  async rewrites() {
    return [
      { source: '/api/proxy/auth/:path*',          destination: 'http://localhost:8001/api/v1/:path*' },
      { source: '/api/proxy/users/:path*',          destination: 'http://localhost:8002/api/v1/:path*' },
      { source: '/api/proxy/restaurants/:path*',    destination: 'http://localhost:8003/api/v1/:path*' },
      { source: '/api/proxy/orders/:path*',         destination: 'http://localhost:8004/api/v1/:path*' },
      { source: '/api/proxy/delivery/:path*',       destination: 'http://localhost:8005/api/v1/:path*' },
      { source: '/api/proxy/notifications/:path*',  destination: 'http://localhost:8006/api/v1/:path*' },
      { source: '/api/proxy/admin/:path*',          destination: 'http://localhost:8007/api/v1/:path*' },
    ];
  },
};

export default nextConfig;
```

---

### 11 — Deliverables

Generate the following (in order):

1. `npx create-next-app@latest food-platform-web --typescript --tailwind --app --src-dir` scaffold
2. Install dependencies: `shadcn/ui`, `@tanstack/react-query`, `zustand`, `react-hook-form`, `zod`, `axios`, `leaflet`, `@types/leaflet`, `uuid`
3. `src/types/api.ts` — all TypeScript types
4. `src/lib/api/` — axios instances for each service + interceptor
5. `src/store/auth.ts` — Zustand auth store
6. All page components listed in section 5
7. All reusable UI components listed in section 7
8. `next.config.ts` with rewrites proxy

**Before generating any code, search the web for the latest Next.js App Router documentation** to confirm you are using current APIs for routing, data fetching, layouts, and server components.

---

## 12 — Additional Notes & Corrections

### 12.1 — Notification Service: Restaurant Owner Notifications

The notification service sends ORDER_PLACED to **both** the customer AND the restaurant owner. The owner dashboard must also poll for notifications.

Add to section 3.6 — poll `GET /notifications/user/:ownerId` on the owner dashboard every 15 seconds (same pattern as customer).

Additional notification the owner receives:

| Event | Notification to Restaurant Owner |
|---|---|
| `ORDER_PLACED` | "New order #[order_id] received." |

In the owner dashboard navbar, show the same `<NotificationBell>` component used in the customer layout, initialized with the owner's `auth_id`.

---

### 12.2 — Delivery Tracking Response Shape

The backend response from `GET /delivery/track/:orderId` is **flat**, not nested. Use this shape:

```json
{
  "order_id": 5,
  "status": "OUT_FOR_DELIVERY",
  "latitude": 17.385,
  "longitude": 78.4867
}
```

Update the `TrackingResponse` type in `src/types/api.ts` accordingly:

```typescript
export interface TrackingResponse {
  order_id: number;
  status: OrderStatus;
  latitude: number;
  longitude: number;
}
```

On the `<LiveTrackingMap>` component, read `response.latitude` and `response.longitude` directly — there is no nested `driver` object.

Driver name and phone are not available from the tracking endpoint.

---

### 12.3 — Restaurant Owner Order Status Endpoint

Restaurant owners must call `PATCH /restaurants/:id/order-status` on the **restaurant-service** (port 8003), NOT order-service directly.

- The restaurant-service then calls order-service internally.
- The `PREPARED` status triggers the `ORDER_PREPARED` RabbitMQ event via the restaurant-service outbox, which kicks off driver assignment automatically.

Correct request for owner status updates:

```
PATCH http://localhost:8003/api/v1/restaurants/:restaurantId/order-status
Authorization: Bearer <token>
Content-Type: application/json

{ "order_id": 99, "status": "CONFIRMED" | "PREPARING" | "PREPARED" }
```

Response: `{ "message": "order status updated to CONFIRMED" }`

> ⚠️ The restaurant-service validates that the caller's `user_id` from the JWT matches the restaurant's `owner_id`. If the caller is not the restaurant owner (and not an ADMIN), the backend returns **403** with error code `not_restaurant_owner`. Handle this in the UI with an "Access denied" toast.

Update the `<RestaurantOwnerOrderCard>` component to call this endpoint, not the order-service status endpoint.

---

### 12.4 — Driver ID Resolution After Login

Driver endpoints use `driverId` which is the **delivery_db internal ID** (`drivers.id`), NOT the `auth_id` from the JWT.

After a DRIVER logs in, the JWT contains `user_id` which is the `auth_id`. You must resolve the internal driver ID before calling any driver endpoint:

1. DRIVER logs in → get `auth_id` from JWT `user_id` field
2. Call `GET /delivery/driver/by-auth/:authId` → response: `{ driver: {...}, active_order: {...} | null }`
3. Store the resolved `driver.id` in the Zustand auth store as `driverId`

Update the Zustand auth store to include `driverId`:

```typescript
interface AuthState {
  user: { id: number; email: string; phone: string; role: string } | null;
  accessToken: string | null;
  driverId: number | null;          // resolved after driver login
  isAuthenticated: boolean;
  setAuth: (user, accessToken) => void;
  setDriverId: (driverId: number) => void;
  clearAuth: () => void;
}
```

On the driver dashboard mount, if `driverId` is null, call `GET /delivery/driver/by-auth/:authId` to resolve it. Show a loading state until `driverId` is available — do not render driver-specific UI until then.

---

### 12.5 — Order Placement: Delivery Address Formatting

`POST /orders` expects `delivery_address` as a **plain string**, not an object. When the user selects a saved address in `<OrderNowModal>`, you must stringify it before sending:

```typescript
const formatAddress = (address: Address): string =>
  `${address.line1}, ${address.city} ${address.pincode}`.trim();
```

In `<OrderNowModal>`:

- Show a dropdown/radio list of the user's saved addresses fetched from `GET /users/:id/addresses`
- Also allow a free-text fallback input if the user has no saved addresses
- Pass the formatted string as `delivery_address` in the order request body

The order response will echo back this string as `delivery_address` — it is never parsed back into an address object by the backend.

---

### 12.6 — Owner's Restaurant Not Visible Until Approved

There is no `GET /restaurants?owner_id=X` endpoint. The public listing only returns approved + open restaurants. If an owner's restaurant is unapproved, it will not appear in any listing endpoint.

**Frontend implication:** After a `RESTAURANT_OWNER` creates a restaurant:

1. Store the newly created restaurant object returned from `POST /restaurants` locally in React state or cache.
2. Use that stored object to render the owner dashboard — do not rely on re-fetching from `GET /restaurants` (it will be missing until approved).
3. Show a persistent "Pending approval" banner until `is_approved=true`.
4. The owner can still fetch their own restaurant via `GET /restaurants/:id` using the known ID — this endpoint returns any restaurant regardless of approval status.

---

### 12.7 — Menu Item `is_veg` Must Always Be Sent Explicitly

The backend defaults `is_veg` to `true` if the field is omitted on `POST /restaurants/:id/menu`. This means any menu item created without explicitly setting `is_veg: false` will be marked as vegetarian regardless of what the UI shows.

**Frontend requirement:** The `<MenuItemForm>` (used in create and edit flows) must render a required toggle/radio for veg vs. non-veg with **no default selected state** — force the user to make an explicit choice before the form can be submitted. The Zod schema must mark `is_veg` as required (`z.boolean()`) for the create form.

---

### 12.8 — Rate Limiting

All services implement Redis-based rate limiting:

| Service | Rate Limit |
|---|---|
| Auth-service (auth endpoints) | 30 requests/minute per IP |
| All other services | 100 requests/minute per IP |

When rate-limited, the backend returns HTTP 429 with a `Retry-After` header. Handle this in the global axios interceptor by showing a "Too many requests" toast and optionally backing off polling intervals.
