import { useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { useForm } from 'react-hook-form';
import { zodResolver } from '@hookform/resolvers/zod';
import { z } from 'zod';
import { restaurantApi } from '@/lib/api/clients';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { Card, CardContent, CardHeader, CardTitle, CardDescription } from '@/components/ui/card';
import { toast } from 'sonner';
import { Loader2 } from 'lucide-react';

const schema = z.object({
  name: z.string().min(1, 'Name is required'),
  address: z.string().optional(),
  cuisine: z.string().optional(),
  latitude: z.coerce.number().optional(),
  longitude: z.coerce.number().optional(),
});

type FormData = z.infer<typeof schema>;

const CreateRestaurantPage = () => {
  const navigate = useNavigate();
  const [loading, setLoading] = useState(false);
  const { register, handleSubmit, formState: { errors } } = useForm<FormData>({ resolver: zodResolver(schema) });

  const onSubmit = async (data: FormData) => {
    setLoading(true);
    try {
      await restaurantApi.post('/restaurants', data);
      toast.success('Restaurant created! Awaiting admin approval.');
      navigate('/owner/dashboard');
    } catch (err: any) {
      toast.error(err.response?.data?.message || 'Failed to create restaurant');
    } finally { setLoading(false); }
  };

  return (
    <div className="p-8 max-w-lg">
      <Card className="shadow-card">
        <CardHeader>
          <CardTitle className="font-display">Create Restaurant</CardTitle>
          <CardDescription>Your restaurant will be reviewed before appearing publicly.</CardDescription>
        </CardHeader>
        <CardContent>
          <form onSubmit={handleSubmit(onSubmit)} className="space-y-4">
            <div className="space-y-2"><Label>Restaurant Name *</Label><Input {...register('name')} />{errors.name && <p className="text-sm text-destructive">{errors.name.message}</p>}</div>
            <div className="space-y-2"><Label>Address</Label><Input {...register('address')} placeholder="Street, City" /></div>
            <div className="space-y-2"><Label>Cuisine</Label><Input {...register('cuisine')} placeholder="Indian, Chinese..." /></div>
            <div className="grid grid-cols-2 gap-4">
              <div className="space-y-2"><Label>Latitude</Label><Input type="number" step="any" {...register('latitude')} /></div>
              <div className="space-y-2"><Label>Longitude</Label><Input type="number" step="any" {...register('longitude')} /></div>
            </div>
            <Button type="submit" className="w-full" disabled={loading}>
              {loading ? <Loader2 className="w-4 h-4 animate-spin mr-2" /> : null} Create
            </Button>
          </form>
        </CardContent>
      </Card>
    </div>
  );
};

export default CreateRestaurantPage;
