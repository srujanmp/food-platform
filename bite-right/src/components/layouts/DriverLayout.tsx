import { Link, Outlet, useNavigate, useLocation } from 'react-router-dom';
import { useAuthStore } from '@/store/auth';
import { authApi } from '@/lib/api/clients';
import { Button } from '@/components/ui/button';
import { UtensilsCrossed, LayoutDashboard, History, LogOut } from 'lucide-react';
import { toast } from 'sonner';
import { cn } from '@/lib/utils';

const navItems = [
  { to: '/driver/dashboard', label: 'Dashboard', icon: LayoutDashboard },
  { to: '/driver/history', label: 'Delivery History', icon: History },
];

const DriverLayout = () => {
  const { clearAuth } = useAuthStore();
  const navigate = useNavigate();
  const location = useLocation();

  const handleLogout = async () => {
    try {
      const rt = localStorage.getItem('refresh_token');
      await authApi.post('/auth/logout', { refresh_token: rt });
    } catch { /* silent */ }
    clearAuth();
    navigate('/login');
    toast.success('Logged out');
  };

  return (
    <div className="min-h-screen flex">
      <aside className="w-64 bg-sidebar text-sidebar-foreground border-r border-sidebar-border flex flex-col">
        <div className="p-4 flex items-center gap-2 border-b border-sidebar-border">
          <div className="w-8 h-8 rounded-lg gradient-hero flex items-center justify-center">
            <UtensilsCrossed className="w-4 h-4 text-primary-foreground" />
          </div>
          <span className="font-display font-bold">Driver Portal</span>
        </div>
        <nav className="flex-1 p-3 space-y-1">
          {navItems.map(item => (
            <Link key={item.to} to={item.to}
              className={cn(
                'flex items-center gap-3 px-3 py-2.5 rounded-lg text-sm font-medium transition-colors',
                location.pathname === item.to ? 'bg-sidebar-accent text-sidebar-accent-foreground' : 'text-sidebar-foreground/70 hover:bg-sidebar-accent/50'
              )}>
              <item.icon className="w-4 h-4" />
              {item.label}
            </Link>
          ))}
        </nav>
        <div className="p-3 border-t border-sidebar-border">
          <Button variant="ghost" className="w-full justify-start text-sidebar-foreground/70 hover:text-sidebar-foreground" onClick={handleLogout}>
            <LogOut className="w-4 h-4 mr-2" /> Logout
          </Button>
        </div>
      </aside>
      <main className="flex-1 overflow-auto"><Outlet /></main>
    </div>
  );
};

export default DriverLayout;
