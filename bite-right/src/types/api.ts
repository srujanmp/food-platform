// API Types for Food Ordering Platform

export interface AuthResponse {
  access_token: string;
  refresh_token: string;
  user: AuthUser;
}

export interface AuthUser {
  id: number;
  email: string;
  phone: string;
  role: UserRole;
}

export type UserRole = 'USER' | 'RESTAURANT_OWNER' | 'DRIVER' | 'ADMIN';
export type RegisterableRole = Exclude<UserRole, 'ADMIN'>;

export interface LoginRequest {
  email: string;
  password: string;
}

export interface RegisterRequest {
  name: string;
  email: string;
  password: string;
  phone: string;
  role?: RegisterableRole;
}

export interface OtpSendRequest {
  phone: string;
}

export interface OtpVerifyRequest {
  phone: string;
  code: string;
}

export interface Profile {
  auth_id: number;
  name: string;
  avatar_url: string;
  created_at: string;
}

export interface Address {
  id: number;
  auth_id: number;
  label: string;
  line1: string;
  city: string;
  pincode: string;
  latitude?: number;
  longitude?: number;
  is_default: boolean;
  created_at: string;
  updated_at: string;
}

export interface AddAddressRequest {
  label: string;
  line1: string;
  city: string;
  pincode: string;
  latitude?: number;
  longitude?: number;
  is_default?: boolean;
}

export interface Restaurant {
  id: number;
  owner_id: number;
  name: string;
  address: string;
  latitude: number;
  longitude: number;
  cuisine: string;
  avg_rating: number;
  is_open: boolean;
  is_approved: boolean;
  created_at: string;
  updated_at: string;
  menu_items?: MenuItem[];
}

export interface MenuItem {
  id: number;
  restaurant_id: number;
  name: string;
  description: string;
  price: number;
  category: string;
  is_veg: boolean;
  is_available: boolean;
  image_url: string;
  created_at: string;
  updated_at: string;
}

export interface CreateRestaurantRequest {
  name: string;
  address?: string;
  latitude?: number;
  longitude?: number;
  cuisine?: string;
}

export interface CreateMenuItemRequest {
  name: string;
  description?: string;
  price: number;
  category?: string;
  is_veg: boolean;
  image_url?: string;
}

export type OrderStatus = 'PLACED' | 'CONFIRMED' | 'PREPARING' | 'PREPARED' | 'OUT_FOR_DELIVERY' | 'DELIVERED' | 'CANCELLED' | 'FAILED';

export interface Order {
  id: number;
  user_id: number;
  restaurant_id: number;
  menu_item_id: number;
  item_name: string;
  item_price: number;
  status: OrderStatus;
  delivery_address: string;
  notes: string;
  created_at: string;
  updated_at: string;
}

export interface PlaceOrderRequest {
  restaurant_id: number;
  menu_item_id: number;
  delivery_address: string;
  notes?: string;
}

export interface Driver {
  id: number;
  auth_id: number;
  name: string;
  phone: string;
  latitude: number;
  longitude: number;
  is_available: boolean;
  created_at: string;
  updated_at: string;
}

export interface Delivery {
  id: number;
  order_id: number;
  driver_id: number;
  user_id: number;
  status: string;
  assigned_at: string;
  delivered_at: string | null;
  created_at: string;
  updated_at: string;
  driver?: Driver | null;
}

export interface DriverResponse {
  driver: Driver;
  active_order: Delivery | null;
}

export interface TrackingResponse {
  order_id: number;
  status: OrderStatus;
  latitude: number;
  longitude: number;
}

export interface UpdateLocationRequest {
  latitude: number;
  longitude: number;
}

export type NotificationEventType = 'ORDER_PLACED' | 'DRIVER_ASSIGNED' | 'ORDER_DELIVERED' | 'ORDER_FAILED' | 'ORDER_CANCELLED';

export interface Notification {
  id: number;
  user_id: number;
  event_type: NotificationEventType;
  title: string;
  body: string;
  is_read: boolean;
  created_at: string;
}

export interface NotificationListResponse {
  notifications: Notification[];
}

export interface AdminDashboard {
  total_orders: number;
  total_revenue: number;
  total_delivered: number;
  total_cancelled: number;
  total_users: number;
  total_restaurants: number;
}

export interface ApiError {
  error: string;
  message?: string;
}

export interface HealthResponse {
  service: string;
  status: string;
  uptime?: string;
}
