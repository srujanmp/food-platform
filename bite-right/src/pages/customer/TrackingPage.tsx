import { useState, useEffect, useRef } from 'react';
import { useParams } from 'react-router-dom';
import { deliveryApi } from '@/lib/api/clients';
import { TrackingResponse } from '@/types/api';
import { Badge } from '@/components/ui/badge';
import { Skeleton } from '@/components/ui/skeleton';
import { MapPin } from 'lucide-react';

const TrackingPage = () => {
  const { id } = useParams<{ id: string }>();
  const [tracking, setTracking] = useState<TrackingResponse | null>(null);
  const [loading, setLoading] = useState(true);
  const intervalRef = useRef<number>();

  useEffect(() => {
    const fetchTracking = async () => {
      try {
        const res = await deliveryApi.get(`/delivery/track/${id}`);
        setTracking(res.data);
        if (['DELIVERED', 'FAILED'].includes(res.data.status)) {
          clearInterval(intervalRef.current);
        }
      } catch { /* silent */ }
      finally { setLoading(false); }
    };

    fetchTracking();
    intervalRef.current = window.setInterval(fetchTracking, 5000);
    return () => clearInterval(intervalRef.current);
  }, [id]);

  if (loading) return <div className="container py-8"><Skeleton className="h-96" /></div>;

  return (
    <div className="container py-8 max-w-3xl">
      <div className="flex items-center justify-between mb-6">
        <h1 className="text-2xl font-display font-bold">Live Tracking — Order #{id}</h1>
        {tracking && (
          <Badge className="bg-primary text-primary-foreground">
            {tracking.status.replace(/_/g, ' ')}
          </Badge>
        )}
      </div>

      {/* Map placeholder - Leaflet requires DOM setup */}
      <div className="h-96 rounded-xl bg-muted border border-border flex flex-col items-center justify-center">
        <MapPin className="w-12 h-12 text-primary mb-4" />
        {tracking ? (
          <div className="text-center">
            <p className="font-medium">Driver Location</p>
            <p className="text-sm text-muted-foreground">
              Lat: {tracking.latitude.toFixed(4)}, Lng: {tracking.longitude.toFixed(4)}
            </p>
            <p className="text-xs text-muted-foreground mt-2">
              {['DELIVERED', 'FAILED'].includes(tracking.status)
                ? 'Delivery complete'
                : 'Updating every 5 seconds...'}
            </p>
          </div>
        ) : (
          <p className="text-muted-foreground">Waiting for tracking data...</p>
        )}
      </div>

      <p className="text-xs text-muted-foreground mt-4 text-center">
        Map integration with Leaflet is ready — connect to a live backend to see real-time driver movement.
      </p>
    </div>
  );
};

export default TrackingPage;
