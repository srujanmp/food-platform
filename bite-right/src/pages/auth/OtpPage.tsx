import { useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { useForm } from 'react-hook-form';
import { zodResolver } from '@hookform/resolvers/zod';
import { z } from 'zod';
import { authApi } from '@/lib/api/clients';
import { useAuthStore } from '@/store/auth';
import { AuthResponse } from '@/types/api';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card';
import { toast } from 'sonner';
import { Smartphone, Loader2 } from 'lucide-react';

const phoneSchema = z.object({ phone: z.string().min(1, 'Phone is required') });
const otpSchema = z.object({ code: z.string().length(6, 'OTP must be 6 digits') });

const OtpPage = () => {
  const navigate = useNavigate();
  const { setAuth } = useAuthStore();
  const [step, setStep] = useState<'phone' | 'verify'>('phone');
  const [phone, setPhone] = useState('');
  const [loading, setLoading] = useState(false);

  const phoneForm = useForm({ resolver: zodResolver(phoneSchema) });
  const otpForm = useForm({ resolver: zodResolver(otpSchema) });

  const sendOtp = async (data: { phone: string }) => {
    setLoading(true);
    try {
      await authApi.post('/auth/otp/send', data);
      setPhone(data.phone);
      setStep('verify');
      toast.success('OTP sent to your phone');
    } catch {
      toast.error('Failed to send OTP');
    } finally {
      setLoading(false);
    }
  };

  const verifyOtp = async (data: { code: string }) => {
    setLoading(true);
    try {
      const res = await authApi.post<AuthResponse>('/auth/otp/verify', { phone, code: data.code });
      const { access_token, refresh_token, user } = res.data;
      setAuth(user, access_token);
      localStorage.setItem('refresh_token', refresh_token);
      toast.success('Verified!');
      const redirectMap = { USER: '/', RESTAURANT_OWNER: '/owner/dashboard', DRIVER: '/driver/dashboard', ADMIN: '/admin/dashboard' } as const;
      navigate(redirectMap[user.role] || '/');
    } catch (err: any) {
      if (err.response?.status === 401 && err.response?.data?.error === 'account_deleted') {
        toast.error('This account has been deactivated.');
      } else {
        toast.error('Invalid OTP');
      }
    } finally {
      setLoading(false);
    }
  };

  return (
    <div className="min-h-screen flex items-center justify-center bg-background px-4">
      <Card className="w-full max-w-md shadow-elevated">
        <CardHeader className="text-center space-y-2">
          <div className="mx-auto w-12 h-12 rounded-xl bg-secondary flex items-center justify-center mb-2">
            <Smartphone className="w-6 h-6 text-primary" />
          </div>
          <CardTitle className="text-2xl font-display font-bold">
            {step === 'phone' ? 'Phone Login' : 'Enter OTP'}
          </CardTitle>
          <CardDescription>
            {step === 'phone' ? 'We\'ll send you a verification code' : `Code sent to ${phone}`}
          </CardDescription>
        </CardHeader>
        <CardContent>
          {step === 'phone' ? (
            <form onSubmit={phoneForm.handleSubmit(sendOtp)} className="space-y-4">
              <div className="space-y-2">
                <Label htmlFor="phone">Phone Number</Label>
                <Input id="phone" placeholder="+91 9876543210" {...phoneForm.register('phone')} />
                {phoneForm.formState.errors.phone && (
                  <p className="text-sm text-destructive">{phoneForm.formState.errors.phone.message as string}</p>
                )}
              </div>
              <Button type="submit" className="w-full" disabled={loading}>
                {loading ? <Loader2 className="w-4 h-4 animate-spin mr-2" /> : null}
                Send OTP
              </Button>
            </form>
          ) : (
            <form onSubmit={otpForm.handleSubmit(verifyOtp)} className="space-y-4">
              <div className="space-y-2">
                <Label htmlFor="code">Verification Code</Label>
                <Input id="code" placeholder="000000" maxLength={6} {...otpForm.register('code')} />
                {otpForm.formState.errors.code && (
                  <p className="text-sm text-destructive">{otpForm.formState.errors.code.message as string}</p>
                )}
              </div>
              <Button type="submit" className="w-full" disabled={loading}>
                {loading ? <Loader2 className="w-4 h-4 animate-spin mr-2" /> : null}
                Verify
              </Button>
              <Button type="button" variant="ghost" className="w-full" onClick={() => setStep('phone')}>
                Change phone number
              </Button>
            </form>
          )}
        </CardContent>
      </Card>
    </div>
  );
};

export default OtpPage;
