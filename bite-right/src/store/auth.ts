import { create } from 'zustand';
import { persist } from 'zustand/middleware';
import { AuthUser } from '@/types/api';

interface AuthState {
  user: AuthUser | null;
  accessToken: string | null;
  driverId: number | null;
  isAuthenticated: boolean;
  displayName: string | null;
  setAuth: (user: AuthUser, accessToken: string) => void;
  setDriverId: (driverId: number) => void;
  setDisplayName: (name: string) => void;
  clearAuth: () => void;
}

export const useAuthStore = create<AuthState>()(
  persist(
    (set) => ({
      user: null,
      accessToken: null,
      driverId: null,
      isAuthenticated: false,
      displayName: null,
      setAuth: (user, accessToken) => set({ user, accessToken, isAuthenticated: true }),
      setDriverId: (driverId) => set({ driverId }),
      setDisplayName: (displayName) => set({ displayName }),
      clearAuth: () => {
        localStorage.removeItem('refresh_token');
        set({ user: null, accessToken: null, driverId: null, isAuthenticated: false, displayName: null });
      },
    }),
    {
      name: 'auth-storage',
      partialize: (state) => ({
        user: state.user,
        accessToken: state.accessToken,
        driverId: state.driverId,
        isAuthenticated: state.isAuthenticated,
        displayName: state.displayName,
      }),
    },
  ),
);
