import { useState, useEffect } from 'react';
import { adminApi } from '@/lib/api/clients';
import { Profile } from '@/types/api';
import { Button } from '@/components/ui/button';
import { Badge } from '@/components/ui/badge';
import { Skeleton } from '@/components/ui/skeleton';
import { toast } from 'sonner';
import { Users, Ban } from 'lucide-react';

const AdminUsersPage = () => {
  const [users, setUsers] = useState<Profile[]>([]);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    adminApi.get('/admin/users').then(res => {
      setUsers(Array.isArray(res.data) ? res.data : []);
    }).catch(() => toast.error('Failed to load users'))
      .finally(() => setLoading(false));
  }, []);

  const banUser = async (authId: number) => {
    try {
      await adminApi.patch(`/admin/users/${authId}/ban`);
      setUsers(prev => prev.filter(u => u.auth_id !== authId));
      toast.success('User banned');
    } catch { toast.error('Failed to ban user'); }
  };

  if (loading) return <div className="p-8 space-y-4">{[1,2,3].map(i => <Skeleton key={i} className="h-16" />)}</div>;

  return (
    <div className="p-8">
      <h1 className="text-2xl font-display font-bold mb-6">Users ({users.length})</h1>
      {users.length === 0 ? (
        <div className="text-center py-20">
          <Users className="w-12 h-12 mx-auto text-muted-foreground mb-4" />
          <p className="text-muted-foreground">No users found</p>
        </div>
      ) : (
        <div className="space-y-3">
          {users.map(u => (
            <div key={u.auth_id} className="p-4 rounded-xl bg-card shadow-card border border-border flex items-center justify-between">
              <div>
                <span className="font-medium">{u.name || 'Unnamed'}</span>
                <p className="text-xs text-muted-foreground">ID: {u.auth_id} • Joined: {new Date(u.created_at).toLocaleDateString()}</p>
              </div>
              <Button variant="destructive" size="sm" onClick={() => banUser(u.auth_id)}>
                <Ban className="w-4 h-4 mr-1" /> Ban
              </Button>
            </div>
          ))}
        </div>
      )}
    </div>
  );
};

export default AdminUsersPage;
