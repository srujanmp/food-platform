import { useState, useEffect } from 'react';
import { useAuthStore } from '@/store/auth';
import { notificationApi } from '@/lib/api/clients';
import { Notification } from '@/types/api';
import { Button } from '@/components/ui/button';
import { Badge } from '@/components/ui/badge';
import { Skeleton } from '@/components/ui/skeleton';
import { Bell, Check } from 'lucide-react';

const NotificationsPage = () => {
  const { user } = useAuthStore();
  const [notifications, setNotifications] = useState<Notification[]>([]);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    if (!user) return;
    notificationApi.get(`/notifications/user/${user.id}`).then(res => {
      setNotifications(res.data.notifications || []);
    }).catch(() => {}).finally(() => setLoading(false));
  }, [user]);

  const markRead = async (id: number) => {
    try {
      await notificationApi.patch(`/notifications/${id}/read`);
      setNotifications(prev => prev.map(n => n.id === id ? { ...n, is_read: true } : n));
    } catch { /* silent */ }
  };

  if (loading) return <div className="container py-8 max-w-2xl space-y-4">{[1,2,3].map(i => <Skeleton key={i} className="h-16 rounded-xl" />)}</div>;

  return (
    <div className="container py-8 max-w-2xl">
      <h1 className="text-2xl font-display font-bold mb-6">Notifications</h1>
      {notifications.length === 0 ? (
        <div className="text-center py-20">
          <Bell className="w-12 h-12 mx-auto text-muted-foreground mb-4" />
          <p className="text-muted-foreground">No notifications yet</p>
        </div>
      ) : (
        <div className="space-y-3">
          {notifications.map(n => (
            <div key={n.id} className={`p-4 rounded-xl border transition-colors ${n.is_read ? 'bg-card border-border' : 'bg-secondary/50 border-primary/20'}`}>
              <div className="flex items-start justify-between">
                <div>
                  <div className="flex items-center gap-2 mb-1">
                    <span className="font-medium text-sm">{n.title}</span>
                    {!n.is_read && <span className="w-2 h-2 rounded-full bg-primary" />}
                  </div>
                  <p className="text-sm text-muted-foreground">{n.body}</p>
                  <p className="text-xs text-muted-foreground mt-1">{new Date(n.created_at).toLocaleString()}</p>
                </div>
                {!n.is_read && (
                  <Button variant="ghost" size="icon" onClick={() => markRead(n.id)}>
                    <Check className="w-4 h-4" />
                  </Button>
                )}
              </div>
            </div>
          ))}
        </div>
      )}
    </div>
  );
};

export default NotificationsPage;
