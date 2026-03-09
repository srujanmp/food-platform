import { useState, useEffect } from 'react';
import { Link } from 'react-router-dom';
import { restaurantApi } from '@/lib/api/clients';
import { Restaurant } from '@/types/api';
import { Input } from '@/components/ui/input';
import { Badge } from '@/components/ui/badge';
import { Skeleton } from '@/components/ui/skeleton';
import { Search, Star, MapPin } from 'lucide-react';

const RestaurantCard = ({ restaurant }: { restaurant: Restaurant }) => (
  <Link to={`/restaurants/${restaurant.id}`} className="group block rounded-xl bg-card shadow-card border border-border overflow-hidden hover:shadow-elevated transition-all">
    <div className="h-40 bg-muted flex items-center justify-center">
      <span className="text-4xl">🍽️</span>
    </div>
    <div className="p-4 space-y-2">
      <div className="flex items-center justify-between">
        <h3 className="font-display font-semibold text-foreground group-hover:text-primary transition-colors">{restaurant.name}</h3>
        <Badge variant={restaurant.is_open ? 'default' : 'secondary'} className={restaurant.is_open ? 'bg-success text-success-foreground' : ''}>
          {restaurant.is_open ? 'Open' : 'Closed'}
        </Badge>
      </div>
      {restaurant.cuisine && <p className="text-sm text-muted-foreground">{restaurant.cuisine}</p>}
      <div className="flex items-center gap-3 text-sm text-muted-foreground">
        {restaurant.avg_rating > 0 && (
          <span className="flex items-center gap-1"><Star className="w-3.5 h-3.5 text-warning fill-warning" /> {restaurant.avg_rating.toFixed(1)}</span>
        )}
        {restaurant.address && (
          <span className="flex items-center gap-1"><MapPin className="w-3.5 h-3.5" /> {restaurant.address}</span>
        )}
      </div>
    </div>
  </Link>
);

const RestaurantListPage = () => {
  const [restaurants, setRestaurants] = useState<Restaurant[]>([]);
  const [loading, setLoading] = useState(true);
  const [search, setSearch] = useState('');

  useEffect(() => {
    const fetch = async () => {
      setLoading(true);
      try {
        const endpoint = search ? `/restaurants/search?q=${encodeURIComponent(search)}` : '/restaurants';
        const res = await restaurantApi.get(endpoint);
        setRestaurants(res.data.restaurants || []);
      } catch { setRestaurants([]); }
      finally { setLoading(false); }
    };
    const debounce = setTimeout(fetch, search ? 300 : 0);
    return () => clearTimeout(debounce);
  }, [search]);

  return (
    <div className="container py-8">
      <div className="flex items-center justify-between mb-8">
        <h1 className="text-3xl font-display font-bold">Restaurants</h1>
        <div className="relative w-72">
          <Search className="absolute left-3 top-1/2 -translate-y-1/2 w-4 h-4 text-muted-foreground" />
          <Input className="pl-9" placeholder="Search restaurants..." value={search} onChange={e => setSearch(e.target.value)} />
        </div>
      </div>
      {loading ? (
        <div className="grid md:grid-cols-2 lg:grid-cols-3 gap-6">
          {[1,2,3,4,5,6].map(i => <Skeleton key={i} className="h-64 rounded-xl" />)}
        </div>
      ) : restaurants.length === 0 ? (
        <div className="text-center py-20 text-muted-foreground">
          <p className="text-lg">No restaurants found</p>
          <p className="text-sm mt-1">Try a different search term</p>
        </div>
      ) : (
        <div className="grid md:grid-cols-2 lg:grid-cols-3 gap-6">
          {restaurants.map(r => <RestaurantCard key={r.id} restaurant={r} />)}
        </div>
      )}
    </div>
  );
};

export default RestaurantListPage;
