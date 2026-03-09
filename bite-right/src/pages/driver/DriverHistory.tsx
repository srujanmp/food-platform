import { useState, useEffect } from 'react';
import { useAuthStore } from '@/store/auth';
import { deliveryApi } from '@/lib/api/clients';
import { Delivery } from '@/types/api';
import { Badge } from '@/components/ui/badge';
import { Skeleton } from '@/components/ui/skeleton';
import { History } from 'lucide-react';

const DriverHistory = () => {
  const { driverId } = useAuthStore();
  const [orders, setOrders] = useState<Delivery[]>([]);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    if (!driverId) return;
    deliveryApi.get(`/delivery/driver/${driverId}/orders`).then(res => {
      setOrders(res.data.orders || []);
    }).catch(() => {}).finally(() => setLoading(false));
  }, [driverId]);

  if (loading) return <div className="p-8 space-y-4">{[1,2,3].map(i => <Skeleton key={i} className="h-20" />)}</div>;

  return (
    <div className="p-8">
      <h1 className="text-2xl font-display font-bold mb-6">Delivery History</h1>
      {orders.length === 0 ? (
        <div className="text-center py-20">
          <History className="w-12 h-12 mx-auto text-muted-foreground mb-4" />
          <p className="text-muted-foreground">No deliveries yet</p>
        </div>
      ) : (
        <div className="space-y-3">
          {orders.map(o => (
            <div key={o.id} className="p-4 rounded-xl bg-card shadow-card border border-border flex items-center justify-between">
              <div>
                <span className="font-medium">Order #{o.order_id}</span>
                <p className="text-xs text-muted-foreground mt-1">
                  {o.delivered_at ? `Delivered: ${new Date(o.delivered_at).toLocaleString()}` : `Assigned: ${new Date(o.assigned_at).toLocaleString()}`}
                </p>
              </div>
              <Badge variant="secondary">{o.status}</Badge>
            </div>
          ))}
        </div>
      )}
    </div>
  );
};

export default DriverHistory;
