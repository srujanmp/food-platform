import { useState, useEffect, useRef } from 'react';
import { useParams } from 'react-router-dom';
import { restaurantApi, userApi, orderApi } from '@/lib/api/clients';
import { getPaymentByOrder, verifyPayment } from '@/lib/api/payments';
import { Restaurant, MenuItem, Address } from '@/types/api';
import { useAuthStore } from '@/store/auth';
import { Button } from '@/components/ui/button';
import { Badge } from '@/components/ui/badge';
import { Skeleton } from '@/components/ui/skeleton';
import { Dialog, DialogContent, DialogHeader, DialogTitle, DialogDescription } from '@/components/ui/dialog';
import { Label } from '@/components/ui/label';
import { Textarea } from '@/components/ui/textarea';
import { Input } from '@/components/ui/input';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';
import { toast } from 'sonner';
import { Payment } from '@/types/payment';
// Razorpay script loader
function loadRazorpayScript() {
  return new Promise((resolve, reject) => {
    if (document.getElementById('razorpay-sdk')) return resolve(true);
    const script = document.createElement('script');
    script.id = 'razorpay-sdk';
    script.src = 'https://checkout.razorpay.com/v1/checkout.js';
    script.onload = () => resolve(true);
    script.onerror = () => reject('Razorpay SDK failed to load');
    document.body.appendChild(script);
  });
}
import { Star, MapPin, Leaf, Drumstick, Loader2 } from 'lucide-react';

const formatAddress = (addr: Address): string =>
  `${addr.line1}, ${addr.city} ${addr.pincode}`.trim();

const RestaurantDetailPage = () => {
    const [payment, setPayment] = useState<Payment | null>(null);
    const [showPayment, setShowPayment] = useState(false);
    const paymentPollRef = useRef<NodeJS.Timeout | null>(null);
  const { id } = useParams<{ id: string }>();
  const { user } = useAuthStore();
  const [restaurant, setRestaurant] = useState<Restaurant | null>(null);
  const [loading, setLoading] = useState(true);
  const [orderModal, setOrderModal] = useState<MenuItem | null>(null);
  const [addresses, setAddresses] = useState<Address[]>([]);
  const [selectedAddress, setSelectedAddress] = useState('');
  const [customAddress, setCustomAddress] = useState('');
  const [notes, setNotes] = useState('');
  const [ordering, setOrdering] = useState(false);

  useEffect(() => {
    const fetch = async () => {
      try {
        const res = await restaurantApi.get(`/restaurants/${id}`);
        setRestaurant(res.data.restaurant);
      } catch { toast.error('Restaurant not found'); }
      finally { setLoading(false); }
    };
    fetch();
  }, [id]);

  useEffect(() => {
    if (!user) return;
    userApi.get(`/users/${user.id}/addresses`).then(res => {
      const addrs = Array.isArray(res.data) ? res.data : (res.data.addresses || []);
      setAddresses(addrs);
      const def = addrs.find((a: Address) => a.is_default);
      if (def) setSelectedAddress(String(def.id));
    }).catch(() => {});
  }, [user]);

  const placeOrder = async () => {
    if (!orderModal || !user || !restaurant) return;
    let deliveryAddress = '';
    if (selectedAddress === 'custom') {
      deliveryAddress = customAddress;
    } else {
      const addrObj = addresses.find(a => String(a.id) === selectedAddress);
      if (!addrObj) {
        toast.error('Please select a valid address');
        return;
      }
      deliveryAddress = formatAddress(addrObj);
    }

    if (!deliveryAddress.trim()) {
      toast.error('Please provide a delivery address');
      return;
    }

    setOrdering(true);
    try {
      const idempotencyKey = crypto.randomUUID();
      const orderRes = await orderApi.post('/orders', {
        restaurant_id: restaurant.id,
        menu_item_id: orderModal.id,
        delivery_address: deliveryAddress,
        notes: notes || undefined,
      }, { headers: { 'Idempotency-Key': idempotencyKey } });
      const orderId = orderRes.data?.order?.id || orderRes.data?.id;
      if (!orderId) throw new Error('Order creation failed');

      // Fetch payment info
      const paymentRes = await getPaymentByOrder(orderId);
      setPayment(paymentRes);
      setShowPayment(true);
      // Open Razorpay checkout
      await loadRazorpayScript();
      // @ts-ignore
      const rzp = new (window as any).Razorpay({
        key: import.meta.env.VITE_RAZORPAY_KEY_ID,
        amount: paymentRes.amount,
        currency: 'INR',
        name: restaurant.name,
        description: orderModal.name,
        order_id: paymentRes.provider_order_id,
        handler: async function (response: any) {
          try {
            console.log('Razorpay handler response:', response);
            const payload = {
              order_id: orderId,
              razorpay_order_id: response.razorpay_order_id,
              razorpay_payment_id: response.razorpay_payment_id,
              razorpay_signature: response.razorpay_signature,
            };
            if (!payload.razorpay_order_id || !payload.razorpay_payment_id || !payload.razorpay_signature) {
              toast.error('Payment failed: Missing Razorpay fields.');
              pollPaymentStatus(orderId);
              return;
            }
            console.log('verifyPayment payload:', payload);
            await verifyPayment(payload);
            toast.success('Payment successful!');
            setShowPayment(false);
            setOrderModal(null);
            setNotes('');
            // Optionally redirect to tracking page
          } catch (err) {
            toast.error('Payment verification failed.');
            pollPaymentStatus(orderId);
          }
        },
        modal: {
          ondismiss: function () {
            pollPaymentStatus(orderId);
          }
        },
        prefill: {
          name: user?.email, // fallback to email, since AuthUser has no name
          email: user?.email,
        },
        theme: { color: '#6366f1' },
      });
      rzp.open();
    } catch (err: any) {
      if (err.response?.status === 409 && err.response?.data?.error === 'duplicate_request') {
        // silently ignore
      } else if (err.response?.status === 422) {
        toast.error(err.response.data?.message || 'Unable to place order');
      } else {
        toast.error('Failed to place order');
      }
    } finally {
      setOrdering(false);
    }
  };

  // Poll payment status fallback
  const pollPaymentStatus = (orderId: number) => {
    let attempts = 0;
    if (paymentPollRef.current) clearInterval(paymentPollRef.current);
    paymentPollRef.current = setInterval(async () => {
      attempts++;
      try {
        const paymentRes = await getPaymentByOrder(orderId);
        setPayment(paymentRes);
        if (["SUCCESS", "FAILED", "REFUNDED"].includes(paymentRes.status)) {
          clearInterval(paymentPollRef.current!);
          if (paymentRes.status === 'SUCCESS') {
            toast.success('Payment successful!');
            setShowPayment(false);
            setOrderModal(null);
            setNotes('');
          } else if (paymentRes.status === 'FAILED') {
            toast.error('Payment failed. Please try again.');
          } else if (paymentRes.status === 'REFUNDED') {
            toast.info('Payment refunded.');
          }
        }
      } catch {
        // ignore
      }
      if (attempts >= 20) {
        clearInterval(paymentPollRef.current!);
      }
    }, 3000);
  };

  if (loading) return <div className="container py-8"><Skeleton className="h-96" /></div>;
  if (!restaurant) return <div className="container py-20 text-center text-muted-foreground">Restaurant not found</div>;

  const menu = restaurant.menu_items || [];

  return (
    <div className="container py-8">
      {/* Restaurant header */}
      <div className="mb-8 p-6 rounded-xl bg-card shadow-card border border-border">
        <div className="flex items-start justify-between">
          <div>
            <h1 className="text-3xl font-display font-bold mb-2">{restaurant.name}</h1>
            <div className="flex items-center gap-4 text-sm text-muted-foreground">
              {restaurant.cuisine && <span>{restaurant.cuisine}</span>}
              {restaurant.avg_rating > 0 && (
                <span className="flex items-center gap-1"><Star className="w-4 h-4 text-warning fill-warning" /> {restaurant.avg_rating.toFixed(1)}</span>
              )}
              {restaurant.address && <span className="flex items-center gap-1"><MapPin className="w-4 h-4" /> {restaurant.address}</span>}
            </div>
          </div>
          <Badge variant={restaurant.is_open ? 'default' : 'secondary'} className={restaurant.is_open ? 'bg-success text-success-foreground' : ''}>
            {restaurant.is_open ? 'Open' : 'Closed'}
          </Badge>
        </div>
      </div>

      {/* Menu */}
      <h2 className="text-2xl font-display font-bold mb-6">Menu</h2>
      {menu.length === 0 ? (
        <p className="text-muted-foreground">No menu items yet.</p>
      ) : (
        <div className="grid md:grid-cols-2 gap-4">
          {menu.map(item => (
            <div key={item.id} className="p-4 rounded-xl bg-card shadow-card border border-border flex justify-between items-start gap-4">
              <div className="flex-1">
                <div className="flex items-center gap-2 mb-1">
                  {item.is_veg ? (
                    <Leaf className="w-4 h-4 text-success" />
                  ) : (
                    <Drumstick className="w-4 h-4 text-destructive" />
                  )}
                  <h3 className="font-semibold">{item.name}</h3>
                </div>
                {item.description && <p className="text-sm text-muted-foreground mb-2">{item.description}</p>}
                <p className="font-display font-bold text-primary">₹{item.price.toFixed(2)}</p>
                {item.category && <Badge variant="secondary" className="mt-1 text-xs">{item.category}</Badge>}
              </div>
              <Button
                size="sm"
                disabled={!item.is_available || !restaurant.is_open}
                onClick={() => setOrderModal(item)}
                className="gradient-hero text-primary-foreground border-0"
              >
                {item.is_available ? 'Order Now' : 'Unavailable'}
              </Button>
            </div>
          ))}
        </div>
      )}

      {/* Order Modal */}
      <Dialog open={!!orderModal} onOpenChange={() => setOrderModal(null)}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle className="font-display">Confirm Order</DialogTitle>
            <DialogDescription>
              {orderModal?.name} — ₹{orderModal?.price.toFixed(2)}
            </DialogDescription>
          </DialogHeader>
          <div className="space-y-4 mt-2">
            <div className="space-y-2">
              <Label>Delivery Address</Label>
              {addresses.length > 0 ? (
                <Select value={selectedAddress} onValueChange={setSelectedAddress}>
                  <SelectTrigger><SelectValue placeholder="Select an address" /></SelectTrigger>
                  <SelectContent>
                    {addresses.map(a => (
                      <SelectItem key={a.id} value={String(a.id)}>
                        {a.label}: {formatAddress(a)}
                      </SelectItem>
                    ))}
                    <SelectItem value="custom">Enter manually</SelectItem>
                  </SelectContent>
                </Select>
              ) : (
                <Input placeholder="Enter delivery address" value={customAddress} onChange={e => setCustomAddress(e.target.value)} />
              )}
              {selectedAddress === 'custom' && (
                <Input placeholder="Enter delivery address" value={customAddress} onChange={e => setCustomAddress(e.target.value)} className="mt-2" />
              )}
            </div>
            <div className="space-y-2">
              <Label>Notes (optional)</Label>
              <Textarea placeholder="Extra spicy, no onions..." value={notes} onChange={e => setNotes(e.target.value)} />
            </div>
            <Button className="w-full gradient-hero text-primary-foreground border-0" onClick={placeOrder} disabled={ordering}>
              {ordering ? <Loader2 className="w-4 h-4 animate-spin mr-2" /> : null}
              Place Order
            </Button>
          </div>
        </DialogContent>
      </Dialog>
    </div>
  );
};

export default RestaurantDetailPage;
