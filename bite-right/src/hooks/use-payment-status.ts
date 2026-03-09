import { useEffect, useState } from 'react';
import { getPaymentByOrder } from '@/lib/api/payments';
import { Payment } from '@/types/payment';

export function usePaymentStatus(orderId: number | undefined, poll: boolean = true) {
  const [payment, setPayment] = useState<Payment | null>(null);
  const [loading, setLoading] = useState(!!orderId);
  useEffect(() => {
    if (!orderId) return;
    let interval: NodeJS.Timeout | null = null;
    let attempts = 0;
    const fetchStatus = async () => {
      try {
        const res = await getPaymentByOrder(orderId);
        setPayment(res);
        if (["SUCCESS", "FAILED", "REFUNDED"].includes(res.status)) {
          if (interval) clearInterval(interval);
        }
      } catch {
        // ignore
      } finally {
        setLoading(false);
      }
    };
    fetchStatus();
    if (poll) {
      interval = setInterval(() => {
        attempts++;
        fetchStatus();
        if (attempts >= 20) interval && clearInterval(interval);
      }, 3000);
    }
    return () => { interval && clearInterval(interval); };
  }, [orderId, poll]);
  return { payment, loading };
}
