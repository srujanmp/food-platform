import { useState } from 'react';
import { useNavigate, Link } from 'react-router-dom';
import { useForm } from 'react-hook-form';
import { zodResolver } from '@hookform/resolvers/zod';
import { z } from 'zod';
import { authApi, userApi } from '@/lib/api/clients';
import { useAuthStore } from '@/store/auth';
import { AuthResponse, RegisterableRole } from '@/types/api';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';
import { toast } from 'sonner';
import { UtensilsCrossed, Loader2 } from 'lucide-react';

const registerSchema = z.object({
  name: z.string().min(1, 'Name is required').max(100),
  email: z.string().email('Invalid email').max(255),
  password: z.string().min(8, 'Password must be at least 8 characters'),
  phone: z.string().min(1, 'Phone is required'),
  role: z.enum(['USER', 'RESTAURANT_OWNER', 'DRIVER'] as const),
});

type RegisterForm = z.infer<typeof registerSchema>;

const RegisterPage = () => {
  const navigate = useNavigate();
  const { setAuth, setDisplayName } = useAuthStore();
  const [loading, setLoading] = useState(false);

  const { register, handleSubmit, setValue, watch, formState: { errors } } = useForm<RegisterForm>({
    resolver: zodResolver(registerSchema),
    defaultValues: { role: 'USER' },
  });

  const onSubmit = async (data: RegisterForm) => {
    setLoading(true);
    try {
      const res = await authApi.post<AuthResponse>('/auth/register', data);
      const { access_token, refresh_token, user } = res.data;
      
      setAuth(user, access_token);
      localStorage.setItem('refresh_token', refresh_token);
      setDisplayName(data.name);

      toast.success('Account created successfully!');

      const redirectMap = { USER: '/', RESTAURANT_OWNER: '/owner/dashboard', DRIVER: '/driver/dashboard', ADMIN: '/admin/dashboard' } as const;
      navigate(redirectMap[user.role] || '/');
    } catch (err: any) {
      const errorData = err.response?.data;
      if (err.response?.status === 409) {
        if (errorData?.error?.includes('email')) {
          toast.error('An account with this email already exists');
        } else if (errorData?.error?.includes('phone')) {
          toast.error('An account with this phone already exists');
        } else {
          toast.error(errorData?.message || 'Account already exists');
        }
      } else {
        toast.error(errorData?.message || 'Registration failed');
      }
    } finally {
      setLoading(false);
    }
  };

  const roleLabels: Record<RegisterableRole, string> = {
    USER: 'Customer',
    RESTAURANT_OWNER: 'Restaurant Owner',
    DRIVER: 'Delivery Driver',
  };

  return (
    <div className="min-h-screen flex items-center justify-center bg-background px-4 py-8">
      <Card className="w-full max-w-md shadow-elevated border-border">
        <CardHeader className="text-center space-y-2">
          <div className="mx-auto w-12 h-12 rounded-xl gradient-hero flex items-center justify-center mb-2">
            <UtensilsCrossed className="w-6 h-6 text-primary-foreground" />
          </div>
          <CardTitle className="text-2xl font-display font-bold">Create account</CardTitle>
          <CardDescription>Get started with FoodPlatform</CardDescription>
        </CardHeader>
        <CardContent>
          <form onSubmit={handleSubmit(onSubmit)} className="space-y-4">
            <div className="space-y-2">
              <Label htmlFor="name">Full Name</Label>
              <Input id="name" placeholder="John Doe" {...register('name')} />
              {errors.name && <p className="text-sm text-destructive">{errors.name.message}</p>}
            </div>
            <div className="space-y-2">
              <Label htmlFor="email">Email</Label>
              <Input id="email" type="email" placeholder="you@example.com" {...register('email')} />
              {errors.email && <p className="text-sm text-destructive">{errors.email.message}</p>}
            </div>
            <div className="space-y-2">
              <Label htmlFor="password">Password</Label>
              <Input id="password" type="password" placeholder="••••••••" {...register('password')} />
              {errors.password && <p className="text-sm text-destructive">{errors.password.message}</p>}
            </div>
            <div className="space-y-2">
              <Label htmlFor="phone">Phone</Label>
              <Input id="phone" placeholder="+91 9876543210" {...register('phone')} />
              {errors.phone && <p className="text-sm text-destructive">{errors.phone.message}</p>}
            </div>
            <div className="space-y-2">
              <Label>I am a</Label>
              <Select value={watch('role')} onValueChange={(v) => setValue('role', v as RegisterableRole)}>
                <SelectTrigger><SelectValue /></SelectTrigger>
                <SelectContent>
                  {(Object.keys(roleLabels) as RegisterableRole[]).map(role => (
                    <SelectItem key={role} value={role}>{roleLabels[role]}</SelectItem>
                  ))}
                </SelectContent>
              </Select>
              {errors.role && <p className="text-sm text-destructive">{errors.role.message}</p>}
            </div>
            <Button type="submit" className="w-full" disabled={loading}>
              {loading ? <Loader2 className="w-4 h-4 animate-spin mr-2" /> : null}
              Create Account
            </Button>
          </form>
          <div className="mt-6 text-center text-sm text-muted-foreground">
            Already have an account?{' '}
            <Link to="/login" className="text-primary font-medium hover:underline">Sign in</Link>
          </div>
        </CardContent>
      </Card>
    </div>
  );
};

export default RegisterPage;
