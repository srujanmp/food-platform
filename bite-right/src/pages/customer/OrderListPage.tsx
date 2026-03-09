import { useState, useEffect } from 'react';
import { Link } from 'react-router-dom';
import { orderApi } from '@/lib/api/clients';
import { useAuthStore } from '@/store/auth';
import { Order } from '@/types/api';
import { usePaymentStatus } from '@/hooks/use-payment-status';
import { PaymentStatus } from '@/types/payment';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Skeleton } from '@/components/ui/skeleton';
import { toast } from 'sonner';
import { ClipboardList } from 'lucide-react';

import { OrderRow } from './OrderRow';

export const statusColor: Record<string, string> = {
  PLACED: 'bg-info text-info-foreground',
  CONFIRMED: 'bg-info text-info-foreground',
  PREPARING: 'bg-warning text-warning-foreground',
  PREPARED: 'bg-warning text-warning-foreground',
  OUT_FOR_DELIVERY: 'bg-primary text-primary-foreground',
  DELIVERED: 'bg-success text-success-foreground',
  CANCELLED: 'bg-muted text-muted-foreground',
  FAILED: 'bg-destructive text-destructive-foreground',
};
export const paymentStatusColor: Record<PaymentStatus, string> = {
  CREATED: 'bg-muted text-muted-foreground',
  PENDING_PAYMENT: 'bg-warning text-warning-foreground',
  SUCCESS: 'bg-success text-success-foreground',
  FAILED: 'bg-destructive text-destructive-foreground',
  REFUNDED: 'bg-muted text-muted-foreground',
};

const OrderListPage = () => {
  const { user } = useAuthStore();
  const [orders, setOrders] = useState<Order[]>([]);
  const [loading, setLoading] = useState(true);


  useEffect(() => {
    if (!user) return;
    orderApi.get(`/orders/user/${user.id}`).then(res => {
      setOrders(res.data.orders || []);
    }).catch(() => toast.error('Failed to load orders'))
      .finally(() => setLoading(false));
  }, [user]);

  return (
    <div className="container py-8">
      <h1 className="text-3xl font-display font-bold mb-8">Your Orders</h1>
      {orders.length === 0 ? (
        <div className="text-center py-20">
          <ClipboardList className="w-12 h-12 mx-auto text-muted-foreground mb-4" />
          <p className="text-lg text-muted-foreground">No orders yet</p>
          <Button asChild className="mt-4"><Link to="/restaurants">Browse Restaurants</Link></Button>
        </div>
      ) : (
        <div className="space-y-4">
          {orders.map(order => (
            <OrderRow key={order.id} order={order} />
          ))}
        </div>
      )}
    </div>
  );
};

export default OrderListPage;
