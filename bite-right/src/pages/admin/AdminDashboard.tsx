import { useState, useEffect } from 'react';
import { adminApi } from '@/lib/api/clients';
import { AdminDashboard as DashboardType } from '@/types/api';
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';
import { Skeleton } from '@/components/ui/skeleton';
import { AlertTriangle, Package, DollarSign, Truck, XCircle, Users, Store } from 'lucide-react';

const AdminDashboard = () => {
  const [data, setData] = useState<DashboardType | null>(null);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    adminApi.get('/admin/analytics/dashboard').then(res => setData(res.data))
      .catch(() => {}).finally(() => setLoading(false));
  }, []);

  if (loading) return <div className="p-8 grid grid-cols-3 gap-6">{[1,2,3,4,5,6].map(i => <Skeleton key={i} className="h-32" />)}</div>;

  const allZero = data && Object.values(data).every(v => v === 0);

  const stats = data ? [
    { label: 'Total Orders', value: data.total_orders, icon: Package, color: 'text-primary' },
    { label: 'Total Revenue', value: `₹${data.total_revenue.toLocaleString()}`, icon: DollarSign, color: 'text-success' },
    { label: 'Delivered', value: data.total_delivered, icon: Truck, color: 'text-success' },
    { label: 'Cancelled', value: data.total_cancelled, icon: XCircle, color: 'text-destructive' },
    { label: 'Total Users', value: data.total_users, icon: Users, color: 'text-info' },
    { label: 'Total Restaurants', value: data.total_restaurants, icon: Store, color: 'text-warning' },
  ] : [];

  return (
    <div className="p-8">
      <h1 className="text-3xl font-display font-bold mb-8">Admin Dashboard</h1>
      
      {allZero && (
        <div className="mb-6 p-4 rounded-xl bg-warning/10 border border-warning/20 flex items-center gap-3">
          <AlertTriangle className="w-5 h-5 text-warning" />
          <p className="text-sm">All values are zero — some upstream services may be unavailable.</p>
        </div>
      )}

      <div className="grid md:grid-cols-3 gap-6">
        {stats.map(({ label, value, icon: Icon, color }) => (
          <Card key={label} className="shadow-card">
            <CardHeader className="flex flex-row items-center justify-between pb-2">
              <CardTitle className="text-sm text-muted-foreground">{label}</CardTitle>
              <Icon className={`w-5 h-5 ${color}`} />
            </CardHeader>
            <CardContent>
              <p className="text-3xl font-display font-bold">{value}</p>
            </CardContent>
          </Card>
        ))}
      </div>
    </div>
  );
};

export default AdminDashboard;
