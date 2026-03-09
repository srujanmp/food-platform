import { useState, useEffect } from 'react';
import { useAuthStore } from '@/store/auth';
import { restaurantApi, orderApi } from '@/lib/api/clients';
import { Restaurant, Order } from '@/types/api';
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';
import { Badge } from '@/components/ui/badge';
import { Skeleton } from '@/components/ui/skeleton';
import { Store, ClipboardList, AlertTriangle } from 'lucide-react';
import { Link } from 'react-router-dom';
import { Button } from '@/components/ui/button';

const OwnerDashboard = () => {
  const { user } = useAuthStore();
  const [restaurant, setRestaurant] = useState<Restaurant | null>(null);
  const [orders, setOrders] = useState<Order[]>([]);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    const fetch = async () => {
      try {
        // Try to find owner's restaurant from all restaurants
        const res = await restaurantApi.get('/restaurants');
        const myRestaurant = (res.data.restaurants || []).find((r: Restaurant) => r.owner_id === user?.id);
        if (myRestaurant) {
          setRestaurant(myRestaurant);
          const ordersRes = await orderApi.get(`/orders/restaurant/${myRestaurant.id}`);
          setOrders(ordersRes.data.orders || []);
        }
      } catch { /* might not have restaurant yet */ }
      finally { setLoading(false); }
    };
    fetch();
  }, [user]);

  if (loading) return <div className="p-8 space-y-4"><Skeleton className="h-32" /><Skeleton className="h-64" /></div>;

  if (!restaurant) {
    return (
      <div className="p-8">
        <div className="text-center py-20">
          <Store className="w-16 h-16 mx-auto text-muted-foreground mb-4" />
          <h2 className="text-2xl font-display font-bold mb-2">No Restaurant Yet</h2>
          <p className="text-muted-foreground mb-6">Create your restaurant to start receiving orders.</p>
          <Button asChild className="gradient-hero text-primary-foreground border-0">
            <Link to="/owner/restaurant/create">Create Restaurant</Link>
          </Button>
        </div>
      </div>
    );
  }

  const pendingOrders = orders.filter(o => ['PLACED', 'CONFIRMED', 'PREPARING'].includes(o.status));

  return (
    <div className="p-8">
      <h1 className="text-3xl font-display font-bold mb-8">Dashboard</h1>

      {!restaurant.is_approved && (
        <div className="mb-6 p-4 rounded-xl bg-warning/10 border border-warning/20 flex items-center gap-3">
          <AlertTriangle className="w-5 h-5 text-warning" />
          <p className="text-sm font-medium">Your restaurant is pending approval. It won't appear in public listings until approved.</p>
        </div>
      )}

      <div className="grid md:grid-cols-3 gap-6 mb-8">
        <Card className="shadow-card">
          <CardHeader className="pb-2"><CardTitle className="text-sm text-muted-foreground">Total Orders</CardTitle></CardHeader>
          <CardContent><p className="text-3xl font-display font-bold">{orders.length}</p></CardContent>
        </Card>
        <Card className="shadow-card">
          <CardHeader className="pb-2"><CardTitle className="text-sm text-muted-foreground">Active Orders</CardTitle></CardHeader>
          <CardContent><p className="text-3xl font-display font-bold text-primary">{pendingOrders.length}</p></CardContent>
        </Card>
        <Card className="shadow-card">
          <CardHeader className="pb-2"><CardTitle className="text-sm text-muted-foreground">Status</CardTitle></CardHeader>
          <CardContent>
            <Badge className={restaurant.is_open ? 'bg-success text-success-foreground' : 'bg-muted text-muted-foreground'}>
              {restaurant.is_open ? 'Open' : 'Closed'}
            </Badge>
          </CardContent>
        </Card>
      </div>

      <h2 className="text-xl font-display font-semibold mb-4">Recent Orders</h2>
      {pendingOrders.length === 0 ? (
        <div className="text-center py-12 text-muted-foreground">
          <ClipboardList className="w-8 h-8 mx-auto mb-2" />
          <p>No active orders</p>
        </div>
      ) : (
        <div className="space-y-3">
          {pendingOrders.slice(0, 5).map(o => (
            <div key={o.id} className="p-4 rounded-xl bg-card shadow-card border border-border flex items-center justify-between">
              <div>
                <span className="font-medium">Order #{o.id}</span>
                <p className="text-sm text-muted-foreground">{o.item_name} — ₹{o.item_price.toFixed(2)}</p>
              </div>
              <Badge>{o.status.replace(/_/g, ' ')}</Badge>
            </div>
          ))}
        </div>
      )}
    </div>
  );
};

export default OwnerDashboard;
