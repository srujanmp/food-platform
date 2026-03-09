import { Toaster } from "@/components/ui/toaster";
import { Toaster as Sonner } from "@/components/ui/sonner";
import { TooltipProvider } from "@/components/ui/tooltip";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { BrowserRouter, Routes, Route } from "react-router-dom";
import { ProtectedRoute, PublicRoute } from "@/components/auth/ProtectedRoute";
import CustomerLayout from "@/components/layouts/CustomerLayout";
import OwnerLayout from "@/components/layouts/OwnerLayout";
import DriverLayout from "@/components/layouts/DriverLayout";
import AdminLayout from "@/components/layouts/AdminLayout";

// Auth pages
import LoginPage from "@/pages/auth/LoginPage";
import RegisterPage from "@/pages/auth/RegisterPage";
import OtpPage from "@/pages/auth/OtpPage";

// Customer pages
import HomePage from "@/pages/customer/HomePage";
import RestaurantListPage from "@/pages/customer/RestaurantListPage";
import RestaurantDetailPage from "@/pages/customer/RestaurantDetailPage";
import OrderListPage from "@/pages/customer/OrderListPage";
import OrderDetailPage from "@/pages/customer/OrderDetailPage";
import TrackingPage from "@/pages/customer/TrackingPage";
import ProfilePage from "@/pages/customer/ProfilePage";
import AddressesPage from "@/pages/customer/AddressesPage";
import NotificationsPage from "@/pages/customer/NotificationsPage";

// Owner pages
import OwnerDashboard from "@/pages/owner/OwnerDashboard";
import CreateRestaurantPage from "@/pages/owner/CreateRestaurantPage";
import RestaurantPage from "@/pages/owner/RestaurantPage";
import MenuPage from "@/pages/owner/MenuPage";
import OwnerOrdersPage from "@/pages/owner/OwnerOrdersPage";

// Driver pages
import DriverDashboard from "@/pages/driver/DriverDashboard";
import DriverHistory from "@/pages/driver/DriverHistory";

// Admin pages
import AdminDashboardPage from "@/pages/admin/AdminDashboard";
import AdminUsersPage from "@/pages/admin/AdminUsersPage";
import AdminRestaurantsPage from "@/pages/admin/AdminRestaurantsPage";

import NotFound from "./pages/NotFound";

const queryClient = new QueryClient();

const App = () => (
  <QueryClientProvider client={queryClient}>
    <TooltipProvider>
      <Toaster />
      <Sonner />
      <BrowserRouter>
        <Routes>
          {/* Auth routes */}
          <Route path="/login" element={<PublicRoute><LoginPage /></PublicRoute>} />
          <Route path="/register" element={<PublicRoute><RegisterPage /></PublicRoute>} />
          <Route path="/otp" element={<PublicRoute><OtpPage /></PublicRoute>} />

          {/* Customer routes */}
          <Route element={<ProtectedRoute allowedRoles={['USER']} />}>
            <Route element={<CustomerLayout />}>
              <Route path="/" element={<HomePage />} />
              <Route path="/restaurants" element={<RestaurantListPage />} />
              <Route path="/restaurants/:id" element={<RestaurantDetailPage />} />
              <Route path="/orders" element={<OrderListPage />} />
              <Route path="/orders/:id" element={<OrderDetailPage />} />
              <Route path="/orders/:id/track" element={<TrackingPage />} />
              <Route path="/profile" element={<ProfilePage />} />
              <Route path="/profile/addresses" element={<AddressesPage />} />
              <Route path="/notifications" element={<NotificationsPage />} />
            </Route>
          </Route>

          {/* Owner routes */}
          <Route element={<ProtectedRoute allowedRoles={['RESTAURANT_OWNER']} />}>
            <Route element={<OwnerLayout />}>
              <Route path="/owner/dashboard" element={<OwnerDashboard />} />
              <Route path="/owner/restaurant" element={<RestaurantPage />} />
              <Route path="/owner/restaurant/create" element={<CreateRestaurantPage />} />
              <Route path="/owner/restaurant/menu" element={<MenuPage />} />
              <Route path="/owner/orders" element={<OwnerOrdersPage />} />
            </Route>
          </Route>

          {/* Driver routes */}
          <Route element={<ProtectedRoute allowedRoles={['DRIVER']} />}>
            <Route element={<DriverLayout />}>
              <Route path="/driver/dashboard" element={<DriverDashboard />} />
              <Route path="/driver/history" element={<DriverHistory />} />
            </Route>
          </Route>

          {/* Admin routes */}
          <Route element={<ProtectedRoute allowedRoles={['ADMIN']} />}>
            <Route element={<AdminLayout />}>
              <Route path="/admin/dashboard" element={<AdminDashboardPage />} />
              <Route path="/admin/users" element={<AdminUsersPage />} />
              <Route path="/admin/restaurants" element={<AdminRestaurantsPage />} />
            </Route>
          </Route>

          <Route path="*" element={<NotFound />} />
        </Routes>
      </BrowserRouter>
    </TooltipProvider>
  </QueryClientProvider>
);

export default App;
