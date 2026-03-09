import { useState, useEffect } from 'react';
import { adminApi } from '@/lib/api/clients';
import { Restaurant } from '@/types/api';
import { Button } from '@/components/ui/button';
import { Badge } from '@/components/ui/badge';
import { Skeleton } from '@/components/ui/skeleton';
import { toast } from 'sonner';
import { Store, CheckCircle } from 'lucide-react';

const AdminRestaurantsPage = () => {
  const [restaurants, setRestaurants] = useState<Restaurant[]>([]);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    adminApi.get('/admin/restaurants').then(res => {
      setRestaurants(res.data.restaurants || []);
    }).catch(() => toast.error('Failed to load'))
      .finally(() => setLoading(false));
  }, []);

  const approve = async (id: number) => {
    try {
      const res = await adminApi.patch(`/admin/restaurants/${id}/approve`);
      setRestaurants(prev => prev.map(r => r.id === id ? { ...r, is_approved: true } : r));
      toast.success('Restaurant approved');
    } catch { toast.error('Failed to approve'); }
  };

  if (loading) return <div className="p-8 space-y-4">{[1,2,3].map(i => <Skeleton key={i} className="h-20" />)}</div>;

  return (
    <div className="p-8">
      <h1 className="text-2xl font-display font-bold mb-6">Restaurants ({restaurants.length})</h1>
      {restaurants.length === 0 ? (
        <div className="text-center py-20">
          <Store className="w-12 h-12 mx-auto text-muted-foreground mb-4" />
          <p className="text-muted-foreground">No restaurants</p>
        </div>
      ) : (
        <div className="space-y-3">
          {restaurants.map(r => (
            <div key={r.id} className="p-4 rounded-xl bg-card shadow-card border border-border flex items-center justify-between">
              <div>
                <div className="flex items-center gap-2 mb-1">
                  <span className="font-medium">{r.name}</span>
                  <Badge className={r.is_approved ? 'bg-success text-success-foreground' : 'bg-warning text-warning-foreground'}>
                    {r.is_approved ? 'Approved' : 'Pending'}
                  </Badge>
                  <Badge variant="secondary">{r.is_open ? 'Open' : 'Closed'}</Badge>
                </div>
                <p className="text-xs text-muted-foreground">{r.cuisine} • {r.address} • Owner: {r.owner_id}</p>
              </div>
              {!r.is_approved && (
                <Button size="sm" className="bg-success text-success-foreground" onClick={() => approve(r.id)}>
                  <CheckCircle className="w-4 h-4 mr-1" /> Approve
                </Button>
              )}
            </div>
          ))}
        </div>
      )}
    </div>
  );
};

export default AdminRestaurantsPage;
