import { useState, useEffect } from 'react';
import { useAuthStore } from '@/store/auth';
import { restaurantApi } from '@/lib/api/clients';
import { Restaurant, MenuItem, CreateMenuItemRequest } from '@/types/api';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { Badge } from '@/components/ui/badge';
import { Dialog, DialogContent, DialogHeader, DialogTitle } from '@/components/ui/dialog';
import { Switch } from '@/components/ui/switch';
import { Skeleton } from '@/components/ui/skeleton';
import { toast } from 'sonner';
import { Plus, Trash2, Leaf, Drumstick, Loader2, ToggleLeft } from 'lucide-react';
import { useForm } from 'react-hook-form';
import { zodResolver } from '@hookform/resolvers/zod';
import { z } from 'zod';

const menuItemSchema = z.object({
  name: z.string().min(1, 'Required'),
  description: z.string().optional(),
  price: z.coerce.number().positive('Must be > 0'),
  category: z.string().optional(),
  is_veg: z.boolean({ required_error: 'Please select veg or non-veg' }),
  image_url: z.string().optional(),
});

const MenuPage = () => {
  const { user } = useAuthStore();
  const [restaurant, setRestaurant] = useState<Restaurant | null>(null);
  const [items, setItems] = useState<MenuItem[]>([]);
  const [loading, setLoading] = useState(true);
  const [showForm, setShowForm] = useState(false);
  const [saving, setSaving] = useState(false);

  const form = useForm<CreateMenuItemRequest>({ resolver: zodResolver(menuItemSchema) });

  useEffect(() => {
    const fetch = async () => {
      try {
        const res = await restaurantApi.get('/restaurants');
        const mine = (res.data.restaurants || []).find((r: Restaurant) => r.owner_id === user?.id);
        if (mine) {
          setRestaurant(mine);
          const menuRes = await restaurantApi.get(`/restaurants/${mine.id}/menu`);
          setItems(menuRes.data.menu_items || []);
        }
      } catch { /* silent */ }
      finally { setLoading(false); }
    };
    fetch();
  }, [user]);

  const addItem = async (data: CreateMenuItemRequest) => {
    if (!restaurant) return;
    setSaving(true);
    try {
      const res = await restaurantApi.post(`/restaurants/${restaurant.id}/menu`, data);
      setItems(prev => [...prev, res.data.menu_item]);
      setShowForm(false);
      form.reset();
      toast.success('Menu item added');
    } catch { toast.error('Failed to add item'); }
    finally { setSaving(false); }
  };

  const deleteItem = async (itemId: number) => {
    if (!restaurant) return;
    try {
      await restaurantApi.delete(`/restaurants/${restaurant.id}/menu/${itemId}`);
      setItems(prev => prev.filter(i => i.id !== itemId));
      toast.success('Item removed');
    } catch { toast.error('Failed to remove'); }
  };

  const toggleAvailability = async (itemId: number) => {
    if (!restaurant) return;
    try {
      const res = await restaurantApi.patch(`/restaurants/${restaurant.id}/menu/${itemId}/toggle`);
      setItems(prev => prev.map(i => i.id === itemId ? res.data.menu_item : i));
    } catch { toast.error('Failed to toggle'); }
  };

  if (loading) return <div className="p-8 space-y-4">{[1,2,3].map(i => <Skeleton key={i} className="h-20" />)}</div>;
  if (!restaurant) return <div className="p-8 text-center py-20 text-muted-foreground">Create a restaurant first</div>;

  return (
    <div className="p-8">
      <div className="flex items-center justify-between mb-6">
        <h1 className="text-2xl font-display font-bold">Menu Items</h1>
        <Button onClick={() => setShowForm(true)} size="sm"><Plus className="w-4 h-4 mr-1" /> Add Item</Button>
      </div>

      {items.length === 0 ? (
        <div className="text-center py-16 text-muted-foreground">No menu items yet. Add your first dish!</div>
      ) : (
        <div className="space-y-3">
          {items.map(item => (
            <div key={item.id} className="p-4 rounded-xl bg-card shadow-card border border-border flex items-center justify-between">
              <div className="flex items-center gap-3">
                {item.is_veg ? <Leaf className="w-4 h-4 text-success" /> : <Drumstick className="w-4 h-4 text-destructive" />}
                <div>
                  <div className="flex items-center gap-2">
                    <span className="font-medium">{item.name}</span>
                    <span className="font-display font-bold text-primary">₹{item.price.toFixed(2)}</span>
                    {item.category && <Badge variant="secondary" className="text-xs">{item.category}</Badge>}
                  </div>
                  {item.description && <p className="text-xs text-muted-foreground">{item.description}</p>}
                </div>
              </div>
              <div className="flex items-center gap-2">
                <Badge className={item.is_available ? 'bg-success text-success-foreground' : 'bg-muted text-muted-foreground'}>
                  {item.is_available ? 'Available' : 'Unavailable'}
                </Badge>
                <Button variant="ghost" size="icon" onClick={() => toggleAvailability(item.id)}>
                  <ToggleLeft className="w-4 h-4" />
                </Button>
                <Button variant="ghost" size="icon" onClick={() => deleteItem(item.id)}>
                  <Trash2 className="w-4 h-4 text-destructive" />
                </Button>
              </div>
            </div>
          ))}
        </div>
      )}

      <Dialog open={showForm} onOpenChange={setShowForm}>
        <DialogContent>
          <DialogHeader><DialogTitle className="font-display">Add Menu Item</DialogTitle></DialogHeader>
          <form onSubmit={form.handleSubmit(addItem)} className="space-y-4 mt-2">
            <div className="space-y-2"><Label>Name *</Label><Input {...form.register('name')} />{form.formState.errors.name && <p className="text-sm text-destructive">{form.formState.errors.name.message}</p>}</div>
            <div className="space-y-2"><Label>Description</Label><Input {...form.register('description')} /></div>
            <div className="grid grid-cols-2 gap-4">
              <div className="space-y-2"><Label>Price (₹) *</Label><Input type="number" step="0.01" {...form.register('price')} />{form.formState.errors.price && <p className="text-sm text-destructive">{form.formState.errors.price.message}</p>}</div>
              <div className="space-y-2"><Label>Category</Label><Input {...form.register('category')} /></div>
            </div>
            <div className="space-y-2">
              <Label>Veg / Non-Veg *</Label>
              <div className="flex gap-4">
                <Button type="button" variant={form.watch('is_veg') === true ? 'default' : 'outline'} size="sm" className={form.watch('is_veg') === true ? 'bg-success text-success-foreground' : ''} onClick={() => form.setValue('is_veg', true)}>
                  <Leaf className="w-4 h-4 mr-1" /> Veg
                </Button>
                <Button type="button" variant={form.watch('is_veg') === false ? 'default' : 'outline'} size="sm" className={form.watch('is_veg') === false ? 'bg-destructive text-destructive-foreground' : ''} onClick={() => form.setValue('is_veg', false)}>
                  <Drumstick className="w-4 h-4 mr-1" /> Non-Veg
                </Button>
              </div>
              {form.formState.errors.is_veg && <p className="text-sm text-destructive">{form.formState.errors.is_veg.message}</p>}
            </div>
            <div className="space-y-2"><Label>Image URL</Label><Input {...form.register('image_url')} /></div>
            <Button type="submit" className="w-full" disabled={saving}>
              {saving ? <Loader2 className="w-4 h-4 animate-spin mr-2" /> : null} Add Item
            </Button>
          </form>
        </DialogContent>
      </Dialog>
    </div>
  );
};

export default MenuPage;
