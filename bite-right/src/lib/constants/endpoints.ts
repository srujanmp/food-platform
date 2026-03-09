export const ENDPOINTS = {
  AUTH: import.meta.env.VITE_AUTH_URL || '/proxy/auth/api/v1',
  USER: import.meta.env.VITE_USER_URL || '/proxy/user/api/v1',
  RESTAURANT: import.meta.env.VITE_RESTAURANT_URL || '/proxy/restaurant/api/v1',
  ORDER: import.meta.env.VITE_ORDER_API_URL || import.meta.env.VITE_ORDER_URL || '/proxy/order/api/v1',
  DELIVERY: import.meta.env.VITE_DELIVERY_URL || '/proxy/delivery/api/v1',
  NOTIFICATION: import.meta.env.VITE_NOTIFICATION_URL || '/proxy/notification/api/v1',
  ADMIN: import.meta.env.VITE_ADMIN_URL || '/proxy/admin/api/v1',
} as const;
