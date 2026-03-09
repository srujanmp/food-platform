import { useState, useEffect } from 'react';
import { useParams, Link } from 'react-router-dom';
import { orderApi } from '@/lib/api/clients';
import { Order, OrderStatus } from '@/types/api';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Skeleton } from '@/components/ui/skeleton';
import { toast } from 'sonner';
import { cn } from '@/lib/utils';
import { CheckCircle2, Circle, MapPin } from 'lucide-react';

const ORDER_STEPS: OrderStatus[] = ['PLACED', 'CONFIRMED', 'PREPARING', 'PREPARED', 'OUT_FOR_DELIVERY', 'DELIVERED'];
const TERMINAL = ['CANCELLED', 'FAILED'];

const OrderDetailPage = () => {
  const { id } = useParams<{ id: string }>();
  const [order, setOrder] = useState<Order | null>(null);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    orderApi.get(`/orders/${id}`).then(res => setOrder(res.data.order || res.data))
      .catch(() => toast.error('Order not found'))
      .finally(() => setLoading(false));
  }, [id]);

  if (loading) return <div className="container py-8"><Skeleton className="h-64" /></div>;
  if (!order) return <div className="container py-20 text-center text-muted-foreground">Order not found</div>;

  const currentIdx = ORDER_STEPS.indexOf(order.status as OrderStatus);
  const isTerminal = TERMINAL.includes(order.status);

  return (
    <div className="container py-8 max-w-2xl">
      <h1 className="text-3xl font-display font-bold mb-2">Order #{order.id}</h1>
      <p className="text-muted-foreground mb-8">{new Date(order.created_at).toLocaleString()}</p>

      {/* Status Stepper */}
      {isTerminal ? (
        <div className="mb-8 p-4 rounded-xl bg-destructive/10 border border-destructive/20 text-center">
          <Badge className="bg-destructive text-destructive-foreground text-sm px-4 py-1">{order.status}</Badge>
          <p className="text-sm text-muted-foreground mt-2">
            {order.status === 'CANCELLED' ? 'Refund will appear in 3–5 business days.' : 'Delivery failed. A refund has been initiated.'}
          </p>
        </div>
      ) : (
        <div className="mb-8">
          <div className="flex items-center justify-between">
            {ORDER_STEPS.map((step, i) => {
              const done = i <= currentIdx;
              const active = i === currentIdx;
              return (
                <div key={step} className="flex flex-col items-center flex-1">
                  <div className={cn(
                    'w-8 h-8 rounded-full flex items-center justify-center text-xs font-bold transition-all',
                    done ? 'gradient-hero text-primary-foreground' : 'bg-muted text-muted-foreground',
                    active && 'ring-4 ring-primary/20'
                  )}>
                    {done ? <CheckCircle2 className="w-4 h-4" /> : <Circle className="w-4 h-4" />}
                  </div>
                  <span className={cn('text-[10px] mt-1 text-center', done ? 'text-foreground font-medium' : 'text-muted-foreground')}>
                    {step.replace(/_/g, ' ')}
                  </span>
                </div>
              );
            })}
          </div>
        </div>
      )}

      {/* Order details */}
      <div className="p-6 rounded-xl bg-card shadow-card border border-border space-y-4">
        <div className="flex justify-between">
          <span className="text-muted-foreground">Item</span>
          <span className="font-medium">{order.item_name}</span>
        </div>
        <div className="flex justify-between">
          <span className="text-muted-foreground">Price</span>
          <span className="font-display font-bold text-primary">₹{order.item_price.toFixed(2)}</span>
        </div>
        <div className="flex justify-between">
          <span className="text-muted-foreground">Delivery Address</span>
          <span className="text-sm text-right max-w-[60%]">{order.delivery_address}</span>
        </div>
        {order.notes && (
          <div className="flex justify-between">
            <span className="text-muted-foreground">Notes</span>
            <span className="text-sm text-right max-w-[60%]">{order.notes}</span>
          </div>
        )}
      </div>

      <div className="flex gap-3 mt-6">
        {['OUT_FOR_DELIVERY', 'PREPARED', 'PREPARING', 'CONFIRMED'].includes(order.status) && (
          <Button asChild className="gradient-hero text-primary-foreground border-0">
            <Link to={`/orders/${order.id}/track`}><MapPin className="w-4 h-4 mr-2" /> Track Delivery</Link>
          </Button>
        )}
        {order.status === 'PLACED' && (
          <Button variant="destructive" onClick={async () => {
            try {
              await orderApi.patch(`/orders/${order.id}/cancel`);
              setOrder({ ...order, status: 'CANCELLED' });
              toast.success('Order cancelled');
            } catch { toast.error('Cannot cancel'); }
          }}>Cancel Order</Button>
        )}
      </div>
    </div>
  );
};

export default OrderDetailPage;
