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
AUTH_SERVICE      = http://localhost:8001/api/v1
USER_SERVICE      = http://localhost:8002/api/v1
RESTAURANT_SERVICE = http://localhost:8003/api/v1
ORDER_SERVICE     = http://localhost:8004/api/v1
DELIVERY_SERVICE  = http://localhost:8005/api/v1
NOTIFICATION_SERVICE = http://localhost:8006/api/v1
ADMIN_SERVICE     = http://localhost:8007/api/v1
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
- Store `access_token` in memory (Zustand) and `refresh_token` in an httpOnly cookie (set via a Next.js Route Handler acting as a BFF proxy, or in localStorage for simplicity).
- All protected endpoints require: `Authorization: Bearer <access_token>`.
- **Token refresh:** call `POST /api/v1/auth/refresh` with body `{ "refresh_token": "..." }` before the access token expires. Implement a **silent refresh** (axios interceptor that catches 401 → refreshes → retries the original request).
- **Logout:** `POST /api/v1/auth/logout` with Bearer token + body `{ "refresh_token": "..." }`. This blacklists both tokens.
- **Roles:** `USER`, `RESTAURANT_OWNER`, `DRIVER`, `ADMIN`. The `ADMIN` role **cannot** be self-registered — it is seeded in the database. Use the `role` field from the decoded JWT (or from the login response) to gate UI sections.

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
| POST | `/logout` | Bearer | `{ refresh_token }` | `{ message: "logged out successfully" }` | Blacklists both tokens |
| POST | `/otp/send` | None | `{ phone }` | `{ message: "OTP sent successfully" }` | Sends OTP to phone |
| POST | `/otp/verify` | None | `{ phone, code (6 digits) }` | `{ access_token, refresh_token, user }` | Sets `is_verified=true` |
| DELETE | `/account` | Bearer | — | `{ message: "account deleted successfully" }` | Soft-delete; fires USER_DELETED event |
| GET | `/health` | None | — | `{ service, status, uptime }` | Health check |

**Validation rules to mirror in Zod:**
- `email`: valid email format
- `password`: minimum 8 characters
- `phone`: required for registration
- `role`: one of `USER`, `RESTAURANT_OWNER`, `DRIVER` (never `ADMIN`)
- OTP `code`: exactly 6 characters

---

#### 3.2 User Service — `http://localhost:8002/api/v1`

The `:id` parameter in all user endpoints is the **auth service user ID** (`auth_db.users.id`), not a separate profile ID.

| Method | Path | Auth | Request Body | Response | Notes |
|---|---|---|---|---|---|
| GET | `/users/:id` | Bearer | — | `{ id, auth_id, name, avatar_url, created_at }` | Any authenticated user can view; only owner can edit |
| PUT | `/users/:id` | Bearer + Owner | `{ name?, phone?, avatar_url? }` | Updated profile object | Only the owning user (or ADMIN) |
| GET | `/users/:id/addresses` | Bearer + Owner | — | Array of address objects | Lists all non-deleted addresses |
| POST | `/users/:id/addresses` | Bearer + Owner | `{ label, line1, city, pincode, latitude?, longitude?, is_default? }` | Created address object | |
| PUT | `/users/addresses/:addressId` | Bearer + Owner | `{ label?, line1?, city?, pincode?, latitude?, longitude?, is_default? }` | Updated address object | |
| DELETE | `/users/addresses/:addressId` | Bearer + Owner | — | `{ message: "address deleted" }` | Soft-delete |
| GET | `/users/health` | None | — | `{ service, status }` | |

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
| GET | `/restaurants/search?q=` | None | — | `{ restaurants: [...] }` | Search by name or cuisine |
| GET | `/restaurants/nearby?lat=&lng=&radius=` | None | — | `{ restaurants: [...] }` | radius in km |
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
| POST | `/restaurants/:id/menu` | Bearer + Owner | `{ name, description?, price (>0), category?, is_veg?, image_url? }` | `{ menu_item: {...} }` | |
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
| GET | `/orders/:id` | Bearer | — | `{ order: {...} }` | Includes payment status |
| GET | `/orders/user/:userId` | Bearer + Owner | — | `{ orders: [...] }` | Order history |
| GET | `/orders/restaurant/:restaurantId` | Bearer + Owner | — | `{ orders: [...] }` | Orders for restaurant owners |
| PATCH | `/orders/:id/cancel` | Bearer | — | `{ message: "cancelled" }` | Only if `status=PLACED` |

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
| GET | `/delivery/driver/:driverId` | Bearer + DRIVER | — | `{ driver: {...}, active_order: {...} \| null }` | Driver profile + active order |
| GET | `/delivery/driver/:driverId/orders` | Bearer + DRIVER | — | `{ orders: [...] }` | Driver's order history |
| PATCH | `/delivery/location` | Bearer + DRIVER | `{ latitude, longitude }` | `{ message: "location_updated" }` | Called on a timer (every 5–10s) from driver app |
| PATCH | `/delivery/:orderId/status` | Bearer + DRIVER | `{ status }` | `{ message: "status_updated" }` | status: `OUT_FOR_DELIVERY` or `DELIVERED` or `FAILED` |
| GET | `/delivery/track/:orderId` | Bearer | — | `{ driver_lat, driver_lng, status, assigned_at, delivered_at? }` | Live tracking; poll every 5s |
| GET | `/delivery/health` | None | — | `{ service, status }` | |

**Tracking response shape:**
```json
{
  "order_id": 99,
  "status": "OUT_FOR_DELIVERY",
  "driver": {
    "id": 5,
    "name": "Ravi Kumar",
    "phone": "+919876543210",
    "latitude": 12.9720,
    "longitude": 77.5950
  },
  "assigned_at": "...",
  "delivered_at": null
}
```

---

#### 3.6 Notification Service — `http://localhost:8006/api/v1`

| Method | Path | Auth | Request Body | Response | Notes |
|---|---|---|---|---|---|
| GET | `/notifications/user/:userId` | Bearer | — | Array of notification objects | Persistent inbox; newest first |
| PATCH | `/notifications/:id/read` | Bearer | — | `{ message: "marked as read" }` | Mark single notification read |
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

| Event | Notification to User |
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
| GET | `/admin/users` | Bearer + ADMIN | — | Array of user objects | Calls user-service internally |
| PATCH | `/admin/users/:id/ban` | Bearer + ADMIN | — | `{ message: "user banned" }` | Revokes all tokens for that user |
| GET | `/admin/restaurants` | Bearer + ADMIN | — | Array of restaurant objects | All restaurants, not just approved |
| PATCH | `/admin/restaurants/:id/approve` | Bearer + ADMIN | — | `{ restaurant: {...} }` | Sets `is_approved=true` |
| GET | `/admin/analytics/dashboard` | Bearer + ADMIN | — | `{ revenue, orders, users, ... }` | Summary analytics |
| GET | `/admin/health` | None | — | `{ service, status }` | |

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
| 403 | `not_restaurant_owner` / `forbidden` | Show "Access denied" toast |
| 404 | `*_not_found` | Show empty state / 404 page |
| 409 | `duplicate_request` | Silently ignore (idempotency replay) |
| 422 | `invalid_status_transition` / `restaurant_unavailable` | Show specific error toast |
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
  isAuthenticated: boolean;
  setAuth: (user, accessToken) => void;
  clearAuth: () => void;
}
```

Persist the access token in memory only. Persist the refresh token in an httpOnly cookie using a Next.js Route Handler.

#### Key Data Flows
- **Order placement:** `crypto.randomUUID()` → store idempotency key → `POST /orders` with `Idempotency-Key` header → invalidate `orders` query on success.
- **Real-time tracking:** use `setInterval` (5s) inside a `useEffect` on the tracking page to poll `GET /delivery/track/:orderId`. Clear interval on unmount.
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
- `Profile`, `Address`, `AddAddressRequest`
- `Restaurant`, `MenuItem`, `CreateRestaurantRequest`, `CreateMenuItemRequest`
- `Order`, `Payment`, `PlaceOrderRequest`, `OrderStatus` (union type of all statuses)
- `Delivery`, `TrackingResponse`, `UpdateLocationRequest`
- `Notification`, `NotificationEventType` (union of all event type strings)
- `ApiError` (shape: `{ error: string; message?: string }`)

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

9. **Soft-deleted accounts.** If login returns 401 with `error: "account_deleted"`, show a specific message: "This account has been deactivated." Do not show the generic "Invalid credentials" message.

10. **Role-based routing.** After login, redirect based on role:
    - `USER` → `/restaurants`
    - `RESTAURANT_OWNER` → `/owner/dashboard`
    - `DRIVER` → `/driver/dashboard`
    - `ADMIN` → `/admin/dashboard`

---

### 10 — CORS Note

The Go backends run with CORS enabled for development. If you hit CORS errors in the browser, add a Next.js `next.config.ts` `rewrites` rule to proxy all backend calls through `/api/proxy/*` to avoid CORS issues entirely:

```typescript
// next.config.ts
const nextConfig = {
  async rewrites() {
    return [
      { source: '/api/proxy/auth/:path*',         destination: 'http://localhost:8001/api/v1/:path*' },
      { source: '/api/proxy/users/:path*',        destination: 'http://localhost:8002/api/v1/:path*' },
      { source: '/api/proxy/restaurants/:path*',  destination: 'http://localhost:8003/api/v1/:path*' },
      { source: '/api/proxy/orders/:path*',       destination: 'http://localhost:8004/api/v1/:path*' },
      { source: '/api/proxy/delivery/:path*',     destination: 'http://localhost:8005/api/v1/:path*' },
      { source: '/api/proxy/notifications/:path*',destination: 'http://localhost:8006/api/v1/:path*' },
      { source: '/api/proxy/admin/:path*',        destination: 'http://localhost:8007/api/v1/:path*' },
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
