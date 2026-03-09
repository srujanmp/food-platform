import { useState, useEffect } from 'react';
import { useAuthStore } from '@/store/auth';
import { orderApi, restaurantApi } from '@/lib/api/clients';
import { Order, Restaurant } from '@/types/api';
import { Button } from '@/components/ui/button';
import { Badge } from '@/components/ui/badge';
import { Skeleton } from '@/components/ui/skeleton';
import { toast } from 'sonner';
import { ClipboardList } from 'lucide-react';

const nextStatus: Record<string, string> = {
  PLACED: 'CONFIRMED',
  CONFIRMED: 'PREPARING',
  PREPARING: 'PREPARED',
};

const OwnerOrdersPage = () => {
  const { user } = useAuthStore();
  const [restaurant, setRestaurant] = useState<Restaurant | null>(null);
  const [orders, setOrders] = useState<Order[]>([]);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    const fetch = async () => {
      try {
        const res = await restaurantApi.get('/restaurants');
        const mine = (res.data.restaurants || []).find((r: Restaurant) => r.owner_id === user?.id);
        if (mine) {
          setRestaurant(mine);
          const ordersRes = await orderApi.get(`/orders/restaurant/${mine.id}`);
          setOrders(ordersRes.data.orders || []);
        }
      } catch { /* silent */ }
      finally { setLoading(false); }
    };
    fetch();
    const interval = setInterval(fetch, 15000);
    return () => clearInterval(interval);
  }, [user]);

  const updateStatus = async (orderId: number, status: string) => {
    if (!restaurant) return;
    try {
      await restaurantApi.patch(`/restaurants/${restaurant.id}/order-status`, { order_id: orderId, status });
      setOrders(prev => prev.map(o => o.id === orderId ? { ...o, status: status as any } : o));
      toast.success(`Order updated to ${status}`);
    } catch (err: any) {
      toast.error(err.response?.data?.message || 'Failed to update');
    }
  };

  if (loading) return <div className="p-8 space-y-4">{[1,2,3].map(i => <Skeleton key={i} className="h-24" />)}</div>;

  const activeOrders = orders.filter(o => ['PLACED', 'CONFIRMED', 'PREPARING', 'PREPARED'].includes(o.status));
  const completedOrders = orders.filter(o => ['DELIVERED', 'CANCELLED', 'FAILED', 'OUT_FOR_DELIVERY'].includes(o.status));

  return (
    <div className="p-8">
      <h1 className="text-2xl font-display font-bold mb-6">Orders</h1>

      <h2 className="font-display font-semibold text-lg mb-4">Active ({activeOrders.length})</h2>
      {activeOrders.length === 0 ? (
        <div className="text-center py-12 text-muted-foreground mb-8">
          <ClipboardList className="w-8 h-8 mx-auto mb-2" /><p>No active orders</p>
        </div>
      ) : (
        <div className="space-y-3 mb-8">
          {activeOrders.map(o => (
            <div key={o.id} className="p-4 rounded-xl bg-card shadow-card border border-border">
              <div className="flex items-center justify-between mb-2">
                <div>
                  <span className="font-medium">Order #{o.id}</span>
                  <Badge className="ml-2">{o.status.replace(/_/g, ' ')}</Badge>
                </div>
                {nextStatus[o.status] && (
                  <Button size="sm" className="gradient-hero text-primary-foreground border-0" onClick={() => updateStatus(o.id, nextStatus[o.status])}>
                    → {nextStatus[o.status]}
                  </Button>
                )}
              </div>
              <p className="text-sm text-muted-foreground">{o.item_name} — ₹{o.item_price.toFixed(2)}</p>
              <p className="text-xs text-muted-foreground">📍 {o.delivery_address}</p>
              {o.notes && <p className="text-xs text-muted-foreground">📝 {o.notes}</p>}
            </div>
          ))}
        </div>
      )}

      <h2 className="font-display font-semibold text-lg mb-4">Past Orders ({completedOrders.length})</h2>
      <div className="space-y-3">
        {completedOrders.slice(0, 10).map(o => (
          <div key={o.id} className="p-4 rounded-xl bg-card shadow-card border border-border flex items-center justify-between">
            <div>
              <span className="font-medium">Order #{o.id}</span>
              <p className="text-sm text-muted-foreground">{o.item_name} — ₹{o.item_price.toFixed(2)}</p>
            </div>
            <Badge variant="secondary">{o.status.replace(/_/g, ' ')}</Badge>
          </div>
        ))}
      </div>
    </div>
  );
};

export default OwnerOrdersPage;
