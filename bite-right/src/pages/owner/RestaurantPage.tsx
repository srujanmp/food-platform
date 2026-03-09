import { useState, useEffect } from 'react';
import { useAuthStore } from '@/store/auth';
import { restaurantApi } from '@/lib/api/clients';
import { Restaurant } from '@/types/api';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';
import { Badge } from '@/components/ui/badge';
import { Skeleton } from '@/components/ui/skeleton';
import { toast } from 'sonner';
import { Loader2, AlertTriangle } from 'lucide-react';
import { Link } from 'react-router-dom';

const RestaurantPage = () => {
  const { user } = useAuthStore();
  const [restaurant, setRestaurant] = useState<Restaurant | null>(null);
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [name, setName] = useState('');
  const [address, setAddress] = useState('');
  const [cuisine, setCuisine] = useState('');

  useEffect(() => {
    const fetch = async () => {
      try {
        const res = await restaurantApi.get('/restaurants');
        const mine = (res.data.restaurants || []).find((r: Restaurant) => r.owner_id === user?.id);
        if (mine) {
          // Fetch full details
          const full = await restaurantApi.get(`/restaurants/${mine.id}`);
          const r = full.data.restaurant;
          setRestaurant(r);
          setName(r.name); setAddress(r.address || ''); setCuisine(r.cuisine || '');
        }
      } catch { /* no restaurant */ }
      finally { setLoading(false); }
    };
    fetch();
  }, [user]);

  if (loading) return <div className="p-8"><Skeleton className="h-64" /></div>;
  if (!restaurant) return (
    <div className="p-8 text-center py-20">
      <p className="text-muted-foreground mb-4">You don't have a restaurant yet.</p>
      <Button asChild><Link to="/owner/restaurant/create">Create Restaurant</Link></Button>
    </div>
  );

  const toggleOpen = async () => {
    try {
      const res = await restaurantApi.patch(`/restaurants/${restaurant.id}/status`);
      setRestaurant(res.data.restaurant);
      toast.success(`Restaurant is now ${res.data.restaurant.is_open ? 'open' : 'closed'}`);
    } catch { toast.error('Failed to update status'); }
  };

  const saveChanges = async () => {
    setSaving(true);
    try {
      const res = await restaurantApi.put(`/restaurants/${restaurant.id}`, { name, address, cuisine });
      setRestaurant(res.data.restaurant);
      toast.success('Updated');
    } catch { toast.error('Failed to update'); }
    finally { setSaving(false); }
  };

  return (
    <div className="p-8 max-w-2xl">
      {!restaurant.is_approved && (
        <div className="mb-6 p-4 rounded-xl bg-warning/10 border border-warning/20 flex items-center gap-3">
          <AlertTriangle className="w-5 h-5 text-warning" />
          <p className="text-sm font-medium">Pending admin approval</p>
        </div>
      )}

      <Card className="shadow-card">
        <CardHeader className="flex flex-row items-center justify-between">
          <CardTitle className="font-display">My Restaurant</CardTitle>
          <div className="flex gap-2 items-center">
            <Badge className={restaurant.is_open ? 'bg-success text-success-foreground' : ''}>
              {restaurant.is_open ? 'Open' : 'Closed'}
            </Badge>
            <Button variant="outline" size="sm" onClick={toggleOpen}>
              {restaurant.is_open ? 'Close' : 'Open'}
            </Button>
          </div>
        </CardHeader>
        <CardContent className="space-y-4">
          <div className="space-y-2"><Label>Name</Label><Input value={name} onChange={e => setName(e.target.value)} /></div>
          <div className="space-y-2"><Label>Address</Label><Input value={address} onChange={e => setAddress(e.target.value)} /></div>
          <div className="space-y-2"><Label>Cuisine</Label><Input value={cuisine} onChange={e => setCuisine(e.target.value)} /></div>
          <Button onClick={saveChanges} disabled={saving}>
            {saving ? <Loader2 className="w-4 h-4 animate-spin mr-2" /> : null} Save Changes
          </Button>
        </CardContent>
      </Card>
    </div>
  );
};

export default RestaurantPage;
