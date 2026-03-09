import { Link } from 'react-router-dom';
import { Button } from '@/components/ui/button';
import { UtensilsCrossed, MapPin, Clock, Star } from 'lucide-react';

const HomePage = () => {
  return (
    <div>
      {/* Hero */}
      <section className="relative overflow-hidden">
        <div className="absolute inset-0 gradient-warm" />
        <div className="container relative py-24 md:py-32">
          <div className="max-w-2xl space-y-6">
            <div className="inline-flex items-center gap-2 px-3 py-1.5 rounded-full bg-secondary text-secondary-foreground text-sm font-medium">
              <UtensilsCrossed className="w-4 h-4" />
              Fresh food, delivered fast
            </div>
            <h1 className="text-4xl md:text-6xl font-display font-extrabold tracking-tight text-foreground leading-tight">
              Delicious meals,<br />
              <span className="text-primary">delivered to you</span>
            </h1>
            <p className="text-lg text-muted-foreground max-w-lg">
              Explore restaurants near you and order your favorite dishes with real-time delivery tracking.
            </p>
            <div className="flex gap-3">
              <Button asChild size="lg" className="gradient-hero text-primary-foreground border-0 shadow-elevated hover:opacity-90 transition-opacity">
                <Link to="/restaurants">Browse Restaurants</Link>
              </Button>
            </div>
          </div>
        </div>
      </section>

      {/* Features */}
      <section className="container py-20">
        <div className="grid md:grid-cols-3 gap-8">
          {[
            { icon: MapPin, title: 'Nearby Restaurants', desc: 'Find the best food places around you with our location-based search.' },
            { icon: Clock, title: 'Real-time Tracking', desc: 'Track your delivery live on the map from restaurant to your doorstep.' },
            { icon: Star, title: 'Top Rated', desc: 'Browse community-rated restaurants and discover new favorites.' },
          ].map(({ icon: Icon, title, desc }) => (
            <div key={title} className="p-6 rounded-xl bg-card shadow-card border border-border hover:shadow-elevated transition-shadow">
              <div className="w-10 h-10 rounded-lg bg-secondary flex items-center justify-center mb-4">
                <Icon className="w-5 h-5 text-primary" />
              </div>
              <h3 className="font-display font-semibold text-lg mb-2">{title}</h3>
              <p className="text-sm text-muted-foreground">{desc}</p>
            </div>
          ))}
        </div>
      </section>
    </div>
  );
};

export default HomePage;
