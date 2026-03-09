# Backend Architecture + Payment Integration Analysis

## Current backend architecture (payment-relevant)

- The system is split into microservices (`auth`, `user`, `restaurant`, `order`, `delivery`, `notification`, `admin`) and wired with Postgres + Redis + RabbitMQ in `docker-compose.yml`.
- `order-service` is where payment records are created and updated.
- Payment persistence exists (`payments` table), but there is **no external gateway integration yet** (the payment is simulated as successful at order placement).
- Event-driven flow is implemented with an outbox pattern from order-service to RabbitMQ.

## How payments work today

1. Client calls `POST /api/v1/orders` with `Idempotency-Key`.
2. `order-service` checks existing payment by idempotency key.
3. It validates restaurant and menu item through restaurant-service internal endpoints.
4. It creates:
   - an `orders` row with status `PLACED`
   - a `payments` row with status `SUCCESS`, gateway `razorpay`, and a random transaction id
   - an outbox event `ORDER_PLACED`
5. On cancel (`PATCH /orders/:id/cancel`) it sets payment status to `REFUNDED`.

## Gaps in current payment implementation

- No payment intent/authorization/capture lifecycle.
- No webhook endpoint to reconcile asynchronous gateway events.
- No signature verification.
- No payment failure/retry flow exposed to client.
- No payment read API for frontend (order response intentionally excludes payment details).
- Monetary type uses `float64` in code, which is risky for currency precision.

## Backend changes required for real payments

### 1) Data model & migrations (order-service)

- Extend `payments` with fields such as:
  - `provider_payment_id`
  - `provider_order_id`
  - `provider_signature`
  - `status` expanded to enum-like values (`PENDING`, `AUTHORIZED`, `CAPTURED`, `FAILED`, `REFUNDED`, `PARTIAL_REFUND`)
  - `failure_code`, `failure_reason`
  - `captured_at`, `refunded_at`
- Consider changing `amount` to integer minor units (`amount_paise`) to avoid floating precision issues.

### 2) Service layer changes

- Replace simulated payment creation in `PlaceOrder` with:
  - create local order in `PENDING_PAYMENT` (or keep draft order)
  - create provider order/payment intent
  - persist provider identifiers + pending status
- Add use-cases:
  - `CreatePaymentIntent(orderID)`
  - `VerifyPayment(orderID, payload, signature)`
  - `HandleWebhook(event)`
  - `RefundPayment(orderID, reason)`

### 3) API endpoints

Add routes under order-service (or a dedicated payment-service):

- `POST /payments/intent` (create gateway intent/order)
- `POST /payments/verify` (client confirmation + signature)
- `POST /payments/webhook` (gateway callback)
- `GET /payments/:orderId` (payment status for UI)
- `POST /payments/:orderId/refund` (admin/system)

### 4) Eventing updates

- Publish explicit payment events:
  - `PAYMENT_AUTHORIZED`
  - `PAYMENT_CAPTURED`
  - `PAYMENT_FAILED`
  - `PAYMENT_REFUNDED`
- Make order-status progression dependent on `PAYMENT_CAPTURED` (e.g., prevent `PLACED` finalization when payment fails).

### 5) Security and reliability

- Verify webhook signatures.
- Add idempotency key handling for webhook/event processing.
- Store raw webhook payload for audit/debug.
- Add timeout/retry strategy for gateway API calls.
- Add structured error mapping from provider to user-safe messages.

## Frontend changes required

> There is no frontend source code in this repo currently. The repository includes a frontend requirements document only.

For a Next.js frontend, add:

1. **Checkout flow**
   - Create payment intent/order via backend.
   - Open provider checkout widget / redirect flow.
   - On success, send verification payload to backend.

2. **Order state handling**
   - Show payment states: `pending`, `processing`, `paid`, `failed`, `refunded`.
   - Retry CTA for failed payments.
   - Poll or subscribe for payment status updates after checkout return.

3. **API client updates**
   - Add `payments` client module.
   - Include idempotency key generation for order placement.

4. **Role-specific UI**
   - Customer: payment status + receipt + retry payment.
   - Owner/Admin: revenue and settlement-aware views (distinguish paid vs refunded).

5. **Web UX details**
   - Prevent double-submit while creating order/payment intent.
   - Persist pending payment state across reloads.
   - Handle webhook lag gracefully via optimistic + polling fallback.

## File-level change plan

### Backend files likely to change

- `order-service/internal/models/models.go` (payment model fields)
- `order-service/migrations/000002_create_payments.up.sql` (schema evolution via new migration)
- `order-service/internal/repository/repo.go` (queries by provider IDs/status)
- `order-service/internal/service/service.go` (replace simulated payment logic)
- `order-service/internal/handlers/handlers.go` (new payment endpoints)
- `order-service/internal/events/publisher.go` (new payment routing keys/events)
- `order-service/cmd/main.go` (register new routes and dependencies)
- `docker-compose.yml` / env config (payment provider keys, webhook secret)

### Frontend files likely to be added/changed (in a separate frontend repo/app)

- `src/lib/api/payments.ts`
- `src/lib/api/orders.ts`
- `src/features/checkout/*`
- `src/features/orders/*`
- `src/stores/payment-store.ts`
- `.env.local` (payment-related public keys, backend base URLs)

## Recommended rollout sequence

1. Backend schema + service abstraction for provider.
2. Payment intent + verify endpoints.
3. Webhook ingestion + reconciliation.
4. Frontend checkout integration.
5. Refund/admin reporting hardening.
6. End-to-end tests for success/failure/refund/idempotent retries.
