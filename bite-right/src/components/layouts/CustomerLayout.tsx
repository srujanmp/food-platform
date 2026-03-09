import { useState, useEffect } from 'react';
import { Link, Outlet, useNavigate } from 'react-router-dom';
import { useAuthStore } from '@/store/auth';
import { notificationApi, authApi } from '@/lib/api/clients';
import { Notification } from '@/types/api';
import { Button } from '@/components/ui/button';
import { Badge } from '@/components/ui/badge';
import { DropdownMenu, DropdownMenuContent, DropdownMenuItem, DropdownMenuTrigger } from '@/components/ui/dropdown-menu';
import { UtensilsCrossed, Bell, User, LogOut, MapPin, ClipboardList, Home } from 'lucide-react';
import { toast } from 'sonner';

const CustomerLayout = () => {
  const { user, displayName, clearAuth, accessToken } = useAuthStore();
  const navigate = useNavigate();
  const [notifications, setNotifications] = useState<Notification[]>([]);
  const unreadCount = notifications.filter(n => !n.is_read).length;

  useEffect(() => {
    if (!user) return;
    const fetchNotifications = async () => {
      try {
        const res = await notificationApi.get(`/notifications/user/${user.id}`);
        setNotifications(res.data.notifications || []);
      } catch { /* silent */ }
    };
    fetchNotifications();
    const interval = setInterval(fetchNotifications, 15000);
    return () => clearInterval(interval);
  }, [user]);

  const handleLogout = async () => {
    try {
      const refreshToken = localStorage.getItem('refresh_token');
      await authApi.post('/auth/logout', { refresh_token: refreshToken });
    } catch { /* silent */ }
    clearAuth();
    navigate('/login');
    toast.success('Logged out');
  };

  const markAsRead = async (id: number) => {
    try {
      await notificationApi.patch(`/notifications/${id}/read`);
      setNotifications(prev => prev.map(n => n.id === id ? { ...n, is_read: true } : n));
    } catch { /* silent */ }
  };

  return (
    <div className="min-h-screen bg-background">
      <header className="sticky top-0 z-50 border-b border-border bg-card/80 backdrop-blur-md">
        <div className="container flex h-16 items-center justify-between">
          <Link to="/" className="flex items-center gap-2">
            <div className="w-8 h-8 rounded-lg gradient-hero flex items-center justify-center">
              <UtensilsCrossed className="w-4 h-4 text-primary-foreground" />
            </div>
            <span className="font-display font-bold text-lg">FoodPlatform</span>
          </Link>

          <nav className="hidden md:flex items-center gap-6">
            <Link to="/" className="text-sm font-medium text-muted-foreground hover:text-foreground transition-colors flex items-center gap-1.5">
              <Home className="w-4 h-4" /> Home
            </Link>
            <Link to="/restaurants" className="text-sm font-medium text-muted-foreground hover:text-foreground transition-colors flex items-center gap-1.5">
              <MapPin className="w-4 h-4" /> Restaurants
            </Link>
            <Link to="/orders" className="text-sm font-medium text-muted-foreground hover:text-foreground transition-colors flex items-center gap-1.5">
              <ClipboardList className="w-4 h-4" /> Orders
            </Link>
          </nav>

          <div className="flex items-center gap-2">
            {/* Notification bell */}
            <DropdownMenu>
              <DropdownMenuTrigger asChild>
                <Button variant="ghost" size="icon" className="relative">
                  <Bell className="w-5 h-5" />
                  {unreadCount > 0 && (
                    <Badge className="absolute -top-1 -right-1 h-5 w-5 rounded-full p-0 flex items-center justify-center text-xs gradient-hero text-primary-foreground border-0">
                      {unreadCount}
                    </Badge>
                  )}
                </Button>
              </DropdownMenuTrigger>
              <DropdownMenuContent align="end" className="w-80">
                {notifications.length === 0 ? (
                  <div className="p-4 text-sm text-muted-foreground text-center">No notifications</div>
                ) : (
                  notifications.slice(0, 5).map(n => (
                    <DropdownMenuItem key={n.id} className="flex flex-col items-start gap-1 p-3 cursor-pointer" onClick={() => markAsRead(n.id)}>
                      <div className="flex items-center gap-2">
                        <span className="font-medium text-sm">{n.title}</span>
                        {!n.is_read && <span className="w-2 h-2 rounded-full bg-primary" />}
                      </div>
                      <span className="text-xs text-muted-foreground">{n.body}</span>
                    </DropdownMenuItem>
                  ))
                )}
                {notifications.length > 0 && (
                  <DropdownMenuItem asChild>
                    <Link to="/notifications" className="text-center text-sm text-primary w-full justify-center">View all</Link>
                  </DropdownMenuItem>
                )}
              </DropdownMenuContent>
            </DropdownMenu>

            {/* User menu */}
            <DropdownMenu>
              <DropdownMenuTrigger asChild>
                <Button variant="ghost" size="icon">
                  <User className="w-5 h-5" />
                </Button>
              </DropdownMenuTrigger>
              <DropdownMenuContent align="end">
                <div className="px-3 py-2 text-sm font-medium">{displayName || user?.email}</div>
                <DropdownMenuItem asChild><Link to="/profile">Profile</Link></DropdownMenuItem>
                <DropdownMenuItem asChild><Link to="/profile/addresses">Addresses</Link></DropdownMenuItem>
                <DropdownMenuItem onClick={handleLogout} className="text-destructive">
                  <LogOut className="w-4 h-4 mr-2" /> Logout
                </DropdownMenuItem>
              </DropdownMenuContent>
            </DropdownMenu>
          </div>
        </div>
      </header>
      <main><Outlet /></main>
    </div>
  );
};

export default CustomerLayout;
