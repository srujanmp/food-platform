https://my-go-guide.vercel.app/

# Food Platform Monorepo

A full-stack, event-driven food-ordering platform built as a **microservice system** with a React frontend.
This repository contains:

- 7 Go backend services (`auth`, `user`, `restaurant`, `order`, `delivery`, `notification`, `admin`)
- Shared infrastructure orchestration with Docker Compose
- A Vite + React frontend (`bite-right`)
- API/E2E shell scripts for integration validation

---

## 1) What is happening in this repository

At runtime, this platform behaves like a real multi-role food delivery product:

- **Users** register/login via `auth-service`.
- New user events are published to RabbitMQ and consumed by downstream services (for example profile/driver bootstrap).
- **Customers** browse restaurants and menu items, place orders, and verify payments.
- **Restaurant owners** manage restaurant data/menu and update order statuses.
- **Drivers** receive/track delivery workflow through `delivery-service`.
- **Notifications** are generated asynchronously from service events.
- **Admins** can ban users, approve restaurants, and see aggregate dashboard data through `admin-service`.

The architecture combines:

- **Synchronous APIs** (HTTP between frontend and services / service-to-service)
- **Asynchronous messaging** (RabbitMQ events)
- **Durable outbox relay loops** in multiple services to publish events reliably
- **Redis-backed rate limiting** and JWT-based auth/role checks

---

## 2) Repository structure

```txt
.
├── auth-service/
├── user-service/
├── restaurant-service/
├── order-service/
├── delivery-service/
├── notification-service/
├── admin-service/
├── bite-right/                 # React frontend (Vite + TypeScript)
├── docs/
├── docker-compose.yml          # Full stack (services + infra + pgAdmin)
├── docker-compose.infra.yml    # Infra-only compose
├── test-apis.sh                # Integration/API flow script
├── test-e2e.sh                 # End-to-end script
└── postman_collection.json
```

---

## 3) Service map (ports, purpose, key behavior)

| Service | Port | Purpose | Storage/Infra | Notes |
|---|---:|---|---|---|
| auth-service | 8001 | Registration, login, refresh, OTP, logout, account delete | Postgres + Redis + RabbitMQ | Seeds default admin from env; has internal ban endpoint |
| user-service | 8002 | User profiles + addresses | Postgres + Redis + RabbitMQ consumer | Exposes internal endpoints for profile ensure/list/ban |
| restaurant-service | 8003 | Restaurants + menus, owner/admin operations | Postgres + Redis + RabbitMQ publisher | Has public listing/search/nearby + internal menu lookups |
| order-service | 8004 | Order lifecycle + payment verification/webhook | Postgres + Redis + RabbitMQ publisher | Uses idempotency key; includes Razorpay verification hooks |
| delivery-service | 8005 | Driver/delivery tracking/status | Postgres + Redis + RabbitMQ consumer+publisher | Includes pending-assignment retry polling |
| notification-service | 8006 | User notification storage/read APIs | Postgres + Redis + RabbitMQ consumer | JWT-protected user notification endpoints |
| admin-service | 8007 | Admin aggregation/orchestration API | Redis + upstream service clients | ADMIN-only protected routes + health |
| pgAdmin | 5050 | DB UI | pgAdmin volume + server json | Preloaded with DB servers |
| RabbitMQ UI | 15672 | Queue broker management | RabbitMQ management plugin | Default guest/guest |

---

## 4) Infrastructure and platform mechanics

### Databases
Each core domain service has its own PostgreSQL instance/database, preserving service isolation.

### Redis
Used mainly for request throttling and selected auth/session-related behavior.

### RabbitMQ
Used for domain events across services. There are explicit publishers/consumers and outbox polling loops in services such as restaurant/order/delivery.

### Health and startup orchestration
`docker-compose.yml` uses health checks for PostgreSQL and RabbitMQ so dependent services wait for readiness.

---

## 5) Backend API domains (high-level)

All services are rooted under `/api/v1`.

- **Auth**: `/auth/register`, `/auth/login`, `/auth/refresh`, OTP endpoints, logout/account deletion, health.
- **User**: profile and address CRUD for authenticated users, internal profile ensure/list/ban endpoints.
- **Restaurant**:
  - Public: list/search/nearby/get/menu
  - Owner/Admin: create/update/delete/toggle/approve, menu management
  - Internal: restaurant/menu item lookups for upstream validation
- **Order**:
  - Protected: place order, verify payment, get payment by order, order reads/status/cancel
  - Internal: order stats/status updates + Razorpay webhook receiver
- **Delivery**: tracking + driver-only status/location/profile/order routes.
- **Notification**: list user notifications and mark as read.
- **Admin**: users/restaurants moderation and dashboard aggregation.

---

## 6) Frontend (`bite-right`) behavior

The frontend is a role-aware React app with pages for:

- **Auth**: register/login/OTP
- **Customer**: home, restaurants, order flow, tracking, profile, addresses, notifications
- **Owner**: restaurant setup/dashboard/menu/orders
- **Driver**: dashboard/history
- **Admin**: users/restaurants/dashboard

Key frontend integration details:

- Vite dev server runs on `8080`.
- Proxy routes (`/proxy/auth`, `/proxy/user`, etc.) map to backend ports 8001-8007 for local development.
- Axios clients inject JWT bearer token automatically and include refresh-token retry logic on 401.

---

## 7) Security and reliability patterns currently implemented

- JWT auth middleware across protected endpoints.
- Role guards for ADMIN / DRIVER / RESTAURANT_OWNER access control.
- Redis rate limiting on APIs.
- Graceful shutdown on services.
- Outbox relay loops to reduce event-loss risk.
- Idempotency key requirement on order placement.

---

## 8) Local development quick start

### Prerequisites
- Docker + Docker Compose
- (Optional) Go toolchain for local service development
- (Optional) Node 18+ for frontend local development

### Run full stack
```bash
docker compose up --build
```

### Useful URLs
- Frontend (if running locally with Vite): `http://localhost:8080`
- pgAdmin: `http://localhost:5050`
- RabbitMQ management: `http://localhost:15672`

### Run API script checks
```bash
bash test-apis.sh
bash test-e2e.sh
```

---

## 9) Environment/config notes

- Service env vars are largely defined in `docker-compose.yml` (ports, DB URLs, JWT secret, RabbitMQ URL, etc.).
- `order-service` additionally reads from `order-service/.env` through `env_file`.
- Default secrets in compose are development placeholders and must be hardened for production.

---

## 10) Operational intent

This repository is structured as a practical microservices learning/implementation platform that demonstrates:

- Domain-separated services
- Event-driven interactions
- Auth + role-based access
- Payment verification flow hooks
- Admin orchestration across services
- A frontend that consumes the entire backend mesh

In short: this is not just a code dump; it is a complete, runnable, multi-service food-ordering ecosystem with both synchronous APIs and asynchronous event processing.
