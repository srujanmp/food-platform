import { useState, useEffect } from 'react';
import { useAuthStore } from '@/store/auth';
import { userApi } from '@/lib/api/clients';
import { Address, AddAddressRequest } from '@/types/api';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';
import { Dialog, DialogContent, DialogHeader, DialogTitle } from '@/components/ui/dialog';
import { Switch } from '@/components/ui/switch';
import { Skeleton } from '@/components/ui/skeleton';
import { toast } from 'sonner';
import { Plus, Trash2, MapPin, Loader2 } from 'lucide-react';

const AddressesPage = () => {
  const { user } = useAuthStore();
  const [addresses, setAddresses] = useState<Address[]>([]);
  const [loading, setLoading] = useState(true);
  const [showForm, setShowForm] = useState(false);
  const [saving, setSaving] = useState(false);
  const [form, setForm] = useState<AddAddressRequest>({ label: '', line1: '', city: '', pincode: '', is_default: false });

  const fetchAddresses = () => {
    if (!user) return;
    userApi.get(`/users/${user.id}/addresses`).then(res => {
      setAddresses(Array.isArray(res.data) ? res.data : (res.data.addresses || []));
    }).catch(() => {}).finally(() => setLoading(false));
  };

  useEffect(() => { fetchAddresses(); }, [user]);

  const addAddress = async () => {
    if (!user) return;
    setSaving(true);
    try {
      await userApi.post(`/users/${user.id}/addresses`, form);
      toast.success('Address added');
      setShowForm(false);
      setForm({ label: '', line1: '', city: '', pincode: '', is_default: false });
      fetchAddresses();
    } catch { toast.error('Failed to add address'); }
    finally { setSaving(false); }
  };

  const deleteAddress = async (id: number) => {
    try {
      await userApi.delete(`/users/addresses/${id}`);
      setAddresses(prev => prev.filter(a => a.id !== id));
      toast.success('Address removed');
    } catch { toast.error('Failed to delete'); }
  };

  if (loading) return <div className="container py-8 max-w-2xl space-y-4">{[1,2].map(i => <Skeleton key={i} className="h-24 rounded-xl" />)}</div>;

  return (
    <div className="container py-8 max-w-2xl">
      <div className="flex items-center justify-between mb-6">
        <h1 className="text-2xl font-display font-bold">Delivery Addresses</h1>
        <Button onClick={() => setShowForm(true)} size="sm"><Plus className="w-4 h-4 mr-1" /> Add</Button>
      </div>

      {addresses.length === 0 ? (
        <div className="text-center py-16">
          <MapPin className="w-12 h-12 mx-auto text-muted-foreground mb-4" />
          <p className="text-muted-foreground">No saved addresses</p>
        </div>
      ) : (
        <div className="space-y-3">
          {addresses.map(addr => (
            <Card key={addr.id} className="shadow-card">
              <CardContent className="flex items-center justify-between py-4">
                <div>
                  <div className="flex items-center gap-2">
                    <span className="font-medium">{addr.label}</span>
                    {addr.is_default && <span className="text-xs bg-primary/10 text-primary px-2 py-0.5 rounded-full">Default</span>}
                  </div>
                  <p className="text-sm text-muted-foreground">{addr.line1}, {addr.city} {addr.pincode}</p>
                </div>
                <Button variant="ghost" size="icon" onClick={() => deleteAddress(addr.id)}>
                  <Trash2 className="w-4 h-4 text-destructive" />
                </Button>
              </CardContent>
            </Card>
          ))}
        </div>
      )}

      <Dialog open={showForm} onOpenChange={setShowForm}>
        <DialogContent>
          <DialogHeader><DialogTitle className="font-display">Add Address</DialogTitle></DialogHeader>
          <div className="space-y-4 mt-2">
            <div className="space-y-2"><Label>Label</Label><Input placeholder="Home, Work..." value={form.label} onChange={e => setForm(f => ({ ...f, label: e.target.value }))} /></div>
            <div className="space-y-2"><Label>Address Line</Label><Input placeholder="123 Main St" value={form.line1} onChange={e => setForm(f => ({ ...f, line1: e.target.value }))} /></div>
            <div className="grid grid-cols-2 gap-4">
              <div className="space-y-2"><Label>City</Label><Input value={form.city} onChange={e => setForm(f => ({ ...f, city: e.target.value }))} /></div>
              <div className="space-y-2"><Label>Pincode</Label><Input value={form.pincode} onChange={e => setForm(f => ({ ...f, pincode: e.target.value }))} /></div>
            </div>
            <div className="flex items-center gap-2"><Switch checked={form.is_default} onCheckedChange={v => setForm(f => ({ ...f, is_default: v }))} /><Label>Default address</Label></div>
            <Button className="w-full" onClick={addAddress} disabled={saving}>
              {saving ? <Loader2 className="w-4 h-4 animate-spin mr-2" /> : null} Save Address
            </Button>
          </div>
        </DialogContent>
      </Dialog>
    </div>
  );
};

export default AddressesPage;
