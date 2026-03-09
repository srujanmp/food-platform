export type PaymentStatus =
  | 'CREATED'
  | 'PENDING_PAYMENT'
  | 'SUCCESS'
  | 'FAILED'
  | 'REFUNDED';

export interface Payment {
  id: number;
  order_id: number;
  status: PaymentStatus;
  provider_order_id: string;
  gateway_txn_id: string;
  failure_reason?: string;
  failure_code?: string;
  created_at: string;
  updated_at: string;
}
