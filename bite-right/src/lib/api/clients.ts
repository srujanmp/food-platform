import axios, { AxiosError, InternalAxiosRequestConfig } from 'axios';
import { ENDPOINTS } from '@/lib/constants/endpoints';
import { useAuthStore } from '@/store/auth';
import { ApiError } from '@/types/api';
import { toast } from 'sonner';

const createApiClient = (baseURL: string) => {
  const client = axios.create({ baseURL, headers: { 'Content-Type': 'application/json' } });

  client.interceptors.request.use((config: InternalAxiosRequestConfig) => {
    const token = useAuthStore.getState().accessToken;
    if (token) {
      config.headers.Authorization = `Bearer ${token}`;
    }
    return config;
  });

  let isRefreshing = false;
  let failedQueue: Array<{ resolve: (v: unknown) => void; reject: (e: unknown) => void }> = [];

  const processQueue = (error: unknown, token: string | null = null) => {
    failedQueue.forEach(prom => {
      if (error) prom.reject(error);
      else prom.resolve(token);
    });
    failedQueue = [];
  };

  client.interceptors.response.use(
    (response) => response,
    async (error: AxiosError<ApiError>) => {
      const originalRequest = error.config as InternalAxiosRequestConfig & { _retry?: boolean };
      
      if (error.response?.status === 401 && !originalRequest._retry) {
        if (error.response.data?.error === 'account_deleted') {
          toast.error('This account has been deactivated.');
          useAuthStore.getState().clearAuth();
          window.location.href = '/login';
          return Promise.reject(error);
        }

        if (isRefreshing) {
          return new Promise((resolve, reject) => {
            failedQueue.push({ resolve, reject });
          }).then(token => {
            originalRequest.headers.Authorization = `Bearer ${token}`;
            return client(originalRequest);
          });
        }

        originalRequest._retry = true;
        isRefreshing = true;

        try {
          const refreshToken = localStorage.getItem('refresh_token');
          if (!refreshToken) throw new Error('No refresh token');
          
          const res = await axios.post(`${ENDPOINTS.AUTH}/auth/refresh`, { refresh_token: refreshToken });
          const { access_token, refresh_token, user } = res.data;
          
          useAuthStore.getState().setAuth(user, access_token);
          localStorage.setItem('refresh_token', refresh_token);
          
          processQueue(null, access_token);
          originalRequest.headers.Authorization = `Bearer ${access_token}`;
          return client(originalRequest);
        } catch (refreshError) {
          processQueue(refreshError, null);
          useAuthStore.getState().clearAuth();
          localStorage.removeItem('refresh_token');
          window.location.href = '/login';
          return Promise.reject(refreshError);
        } finally {
          isRefreshing = false;
        }
      }

      // Handle other errors
      if (error.response) {
        const status = error.response.status;
        const data = error.response.data;
        
        if (status === 403) {
          toast.error('Access denied');
        } else if (status === 409 && data?.error === 'duplicate_request') {
          // Silently ignore idempotency replays
        } else if (status === 429) {
          toast.error('Too many requests. Please wait a moment.');
        } else if (status === 503) {
          toast.error('Service temporarily unavailable');
        } else if (status >= 500) {
          toast.error('Something went wrong. Please try again.');
        }
      }

      return Promise.reject(error);
    }
  );

  return client;
};

export const authApi = createApiClient(ENDPOINTS.AUTH);
export const userApi = createApiClient(ENDPOINTS.USER);
export const restaurantApi = createApiClient(ENDPOINTS.RESTAURANT);
export const orderApi = createApiClient(ENDPOINTS.ORDER);
export const deliveryApi = createApiClient(ENDPOINTS.DELIVERY);
export const notificationApi = createApiClient(ENDPOINTS.NOTIFICATION);
export const adminApi = createApiClient(ENDPOINTS.ADMIN);
