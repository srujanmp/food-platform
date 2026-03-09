import { useAuthStore } from '@/store/auth';

export function getToken() {
  // This must be called inside a React component or hook
  return useAuthStore.getState().accessToken;
}
