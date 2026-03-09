import { Navigate, Outlet } from 'react-router-dom';
import { useAuthStore } from '@/store/auth';
import { UserRole } from '@/types/api';

interface ProtectedRouteProps {
  allowedRoles?: UserRole[];
  children?: React.ReactNode;
}

export const ProtectedRoute = ({ allowedRoles, children }: ProtectedRouteProps) => {
  const { isAuthenticated, user } = useAuthStore();

  if (!isAuthenticated) {
    return <Navigate to="/login" replace />;
  }

  if (allowedRoles && user && !allowedRoles.includes(user.role)) {
    return <Navigate to="/" replace />;
  }

  return children ? <>{children}</> : <Outlet />;
};

export const PublicRoute = ({ children }: { children: React.ReactNode }) => {
  const { isAuthenticated, user } = useAuthStore();

  if (isAuthenticated && user) {
    const redirectMap: Record<UserRole, string> = {
      USER: '/',
      RESTAURANT_OWNER: '/owner/dashboard',
      DRIVER: '/driver/dashboard',
      ADMIN: '/admin/dashboard',
    };
    return <Navigate to={redirectMap[user.role]} replace />;
  }

  return <>{children}</>;
};
