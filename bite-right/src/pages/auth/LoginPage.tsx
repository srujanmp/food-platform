import { useState } from 'react';
import { useNavigate, Link } from 'react-router-dom';
import { useForm } from 'react-hook-form';
import { zodResolver } from '@hookform/resolvers/zod';
import { z } from 'zod';
import { authApi } from '@/lib/api/clients';
import { userApi } from '@/lib/api/clients';
import { useAuthStore } from '@/store/auth';
import { AuthResponse } from '@/types/api';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card';
import { toast } from 'sonner';
import { UtensilsCrossed, Loader2 } from 'lucide-react';

const loginSchema = z.object({
  email: z.string().email('Invalid email address'),
  password: z.string().min(8, 'Password must be at least 8 characters'),
});

type LoginForm = z.infer<typeof loginSchema>;

const LoginPage = () => {
  const navigate = useNavigate();
  const { setAuth, setDisplayName, setDriverId } = useAuthStore();
  const [loading, setLoading] = useState(false);

  const { register, handleSubmit, formState: { errors } } = useForm<LoginForm>({
    resolver: zodResolver(loginSchema),
  });

  const onSubmit = async (data: LoginForm) => {
    setLoading(true);
    try {
      const res = await authApi.post<AuthResponse>('/auth/login', data);
      const { access_token, refresh_token, user } = res.data;
      
      setAuth(user, access_token);
      localStorage.setItem('refresh_token', refresh_token);

      // Fetch display name
      try {
        const profileRes = await userApi.get(`/users/${user.id}`, {
          headers: { Authorization: `Bearer ${access_token}` },
        });
        setDisplayName(profileRes.data.name);
      } catch { /* profile may not exist yet */ }

      // Resolve driver ID if needed
      if (user.role === 'DRIVER') {
        try {
          const driverRes = await import('@/lib/api/clients').then(m => 
            m.deliveryApi.get(`/delivery/driver/by-auth/${user.id}`, {
              headers: { Authorization: `Bearer ${access_token}` },
            })
          );
          setDriverId(driverRes.data.driver.id);
        } catch { /* driver record may not be ready yet */ }
      }

      toast.success('Welcome back!');

      const redirectMap = { USER: '/', RESTAURANT_OWNER: '/owner/dashboard', DRIVER: '/driver/dashboard', ADMIN: '/admin/dashboard' } as const;
      navigate(redirectMap[user.role] || '/');
    } catch (err: any) {
      const errorData = err.response?.data;
      if (err.response?.status === 401 && errorData?.error === 'account_deleted') {
        toast.error('This account has been deactivated.');
      } else if (err.response?.status === 401) {
        toast.error('Invalid email or password');
      } else if (err.response?.status === 409) {
        toast.error(errorData?.message || 'Account conflict');
      } else {
        toast.error('Login failed. Please try again.');
      }
    } finally {
      setLoading(false);
    }
  };

  return (
    <div className="min-h-screen flex items-center justify-center bg-background px-4">
      <Card className="w-full max-w-md shadow-elevated border-border">
        <CardHeader className="text-center space-y-2">
          <div className="mx-auto w-12 h-12 rounded-xl gradient-hero flex items-center justify-center mb-2">
            <UtensilsCrossed className="w-6 h-6 text-primary-foreground" />
          </div>
          <CardTitle className="text-2xl font-display font-bold">Welcome back</CardTitle>
          <CardDescription>Sign in to your account</CardDescription>
        </CardHeader>
        <CardContent>
          <form onSubmit={handleSubmit(onSubmit)} className="space-y-4">
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
            <Button type="submit" className="w-full" disabled={loading}>
              {loading ? <Loader2 className="w-4 h-4 animate-spin mr-2" /> : null}
              Sign In
            </Button>
          </form>
          <div className="mt-6 text-center text-sm text-muted-foreground">
            Don't have an account?{' '}
            <Link to="/register" className="text-primary font-medium hover:underline">Sign up</Link>
          </div>
          <div className="mt-2 text-center text-sm">
            <Link to="/otp" className="text-muted-foreground hover:text-primary">Sign in with OTP</Link>
          </div>
        </CardContent>
      </Card>
    </div>
  );
};

export default LoginPage;
