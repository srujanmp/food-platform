import { getToken } from '../utils/getToken';

const API_URL = import.meta.env.VITE_ORDER_API_URL;

export async function verifyPayment(payload: {
  OrderID: number;
  RazorpayOrderID: string;
  RazorpayPaymentID: string;
  RazorpaySignature: string;
}) {
  const res = await fetch(`${API_URL}/payments/verify`, {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
      Authorization: `Bearer ${getToken()}`,
    },
    body: JSON.stringify(payload),
  });
  if (!res.ok) throw new Error('Payment verification failed');
  return res.json();
}

export async function getPaymentByOrder(orderId: number) {
  const res = await fetch(`${API_URL}/payments/order/${orderId}`, {
    headers: {
      Authorization: `Bearer ${getToken()}`,
    },
  });
  if (!res.ok) throw new Error('Failed to fetch payment status');
  return res.json();
}
