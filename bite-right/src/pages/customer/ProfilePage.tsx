import { useState, useEffect } from 'react';
import { useAuthStore } from '@/store/auth';
import { userApi } from '@/lib/api/clients';
import { Profile } from '@/types/api';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';
import { Skeleton } from '@/components/ui/skeleton';
import { toast } from 'sonner';
import { Loader2 } from 'lucide-react';

const ProfilePage = () => {
  const { user, setDisplayName } = useAuthStore();
  const [profile, setProfile] = useState<Profile | null>(null);
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [name, setName] = useState('');
  const [avatarUrl, setAvatarUrl] = useState('');

  useEffect(() => {
    if (!user) return;
    userApi.get(`/users/${user.id}`).then(res => {
      setProfile(res.data);
      setName(res.data.name || '');
      setAvatarUrl(res.data.avatar_url || '');
    }).catch(() => {}).finally(() => setLoading(false));
  }, [user]);

  const handleSave = async () => {
    if (!user) return;
    setSaving(true);
    try {
      const res = await userApi.put(`/users/${user.id}`, { name, avatar_url: avatarUrl || undefined });
      setProfile(res.data);
      setDisplayName(name);
      toast.success('Profile updated');
    } catch { toast.error('Failed to update profile'); }
    finally { setSaving(false); }
  };

  if (loading) return <div className="container py-8 max-w-lg"><Skeleton className="h-64" /></div>;

  return (
    <div className="container py-8 max-w-lg">
      <Card className="shadow-card">
        <CardHeader>
          <CardTitle className="font-display">Profile</CardTitle>
        </CardHeader>
        <CardContent className="space-y-4">
          <div className="space-y-2">
            <Label>Email</Label>
            <Input value={user?.email || ''} disabled />
          </div>
          <div className="space-y-2">
            <Label>Phone</Label>
            <Input value={user?.phone || ''} disabled />
            <p className="text-xs text-muted-foreground">Phone is managed by authentication service</p>
          </div>
          <div className="space-y-2">
            <Label>Name</Label>
            <Input value={name} onChange={e => setName(e.target.value)} placeholder="Your name" />
          </div>
          <div className="space-y-2">
            <Label>Avatar URL</Label>
            <Input value={avatarUrl} onChange={e => setAvatarUrl(e.target.value)} placeholder="https://..." />
          </div>
          <Button className="w-full" onClick={handleSave} disabled={saving}>
            {saving ? <Loader2 className="w-4 h-4 animate-spin mr-2" /> : null}
            Save Changes
          </Button>
        </CardContent>
      </Card>
    </div>
  );
};

export default ProfilePage;
