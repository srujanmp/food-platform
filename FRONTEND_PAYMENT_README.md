# Frontend Razorpay Integration Guide (for the existing frontend tree)

This guide explains exactly what to change in your frontend so it works with the new Razorpay-enabled backend in `order-service`.

## New backend endpoints to use

Base: `http://localhost:8004/api/v1`

1. `POST /orders` (Bearer + `Idempotency-Key`)
   - Creates order in `PENDING_PAYMENT`
   - Creates Razorpay order internally

2. `POST /payments/verify` (Bearer)
   - Body:
   ```json
   {
     "order_id": 123,
     "razorpay_order_id": "order_XXXX",
     "razorpay_payment_id": "pay_XXXX",
     "razorpay_signature": "..."
   }
   ```
   - Marks payment `SUCCESS`, moves order to `PLACED`

3. `GET /payments/order/:orderId` (Bearer)
   - Returns current payment record/status for polling and UI state.

## Frontend flow changes

## 1) Create order request

When user clicks **Order Now**:
- Generate `Idempotency-Key` (`crypto.randomUUID()`)
- Call `POST /orders`
- Store returned `order.id`

## 2) Open Razorpay checkout

Use Razorpay web checkout script and pass:
- `key`: your Razorpay key id (public)
- `amount`: from menu/item amount in paise
- `order_id`: from backend payment record (`provider_order_id`) fetched via `GET /payments/order/:orderId`

## 3) Verify payment with backend

In Razorpay success handler:
- Send `POST /payments/verify` with `order_id`, `razorpay_order_id`, `razorpay_payment_id`, `razorpay_signature`.
- On success: redirect to order tracking page.

## 4) Poll payment status fallback

If verification call fails due to network/UI refresh:
- Poll `GET /payments/order/:orderId` every 3-5s for up to ~60s.
- Handle statuses:
  - `CREATED` / `PENDING_PAYMENT` UI pending
  - `SUCCESS` UI paid
  - `FAILED` show retry
  - `REFUNDED` show refunded

## 5) Retry UX

For failed payment:
- Keep order reference
- Re-open checkout for the same order by re-fetching payment state and provider order id (or call a retry endpoint if added later).

## Suggested file changes (based on your frontend structure)

- `src/lib/api/order.ts`
  - Keep `createOrder` with idempotency header
- `src/lib/api/payments.ts`
  - Add `verifyPayment(payload)`
  - Add `getPaymentByOrder(orderId)`
- `src/pages/customer/checkout/*`
  - Integrate Razorpay script + success/failure handlers
- `src/pages/customer/orders/*`
  - Payment status badges + polling fallback
- `src/types/payment.ts`
  - Add payment model with new fields (`status`, `provider_order_id`, `gateway_txn_id`, failure info)

## Environment variables for frontend

```env
VITE_ORDER_API_URL=http://localhost:8004/api/v1
VITE_RAZORPAY_KEY_ID=rzp_test_key
```

## Important notes

- Do **not** trust frontend signature verification. Always call backend `/payments/verify`.
- Keep `Idempotency-Key` for each order attempt to avoid duplicates.
- If webhook updates payment state later, rely on `GET /payments/order/:orderId` for source of truth.
