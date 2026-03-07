package models

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"time"
)

// JSONMap is a map[string]any that implements sql.Scanner / driver.Valuer
// so GORM can read/write JSONB columns correctly.
type JSONMap map[string]any

func (j *JSONMap) Scan(src any) error {
	if src == nil {
		*j = nil
		return nil
	}
	var bs []byte
	switch v := src.(type) {
	case []byte:
		bs = v
	case string:
		bs = []byte(v)
	default:
		return fmt.Errorf("JSONMap.Scan: unsupported type %T", src)
	}
	return json.Unmarshal(bs, j)
}

func (j JSONMap) Value() (driver.Value, error) {
	if j == nil {
		return nil, nil
	}
	return json.Marshal(j)
}

// --- DB models -----------------------------------------------------------

// Order represents a row in the orders table.
type Order struct {
	ID              uint      `gorm:"primaryKey;autoIncrement" json:"id"`
	UserID          uint      `gorm:"not null" json:"user_id"`
	RestaurantID    uint      `gorm:"not null" json:"restaurant_id"`
	MenuItemID      uint      `gorm:"not null" json:"menu_item_id"`
	ItemName        string    `gorm:"not null" json:"item_name"`
	ItemPrice       float64   `gorm:"not null" json:"item_price"`
	Status          string    `gorm:"not null;default:PLACED" json:"status"`
	DeliveryAddress string    `gorm:"not null" json:"delivery_address"`
	Notes           string    `json:"notes,omitempty"`
	IdempotencyKey  string    `gorm:"uniqueIndex;not null" json:"-"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}

// Payment represents a row in the payments table.
type Payment struct {
	ID             uint      `gorm:"primaryKey;autoIncrement" json:"id"`
	OrderID        uint      `gorm:"uniqueIndex;not null" json:"order_id"`
	UserID         uint      `gorm:"not null" json:"user_id"`
	Amount         float64   `gorm:"not null" json:"amount"`
	Currency       string    `gorm:"default:INR" json:"currency"`
	Status         string    `gorm:"not null" json:"status"`
	Gateway        string    `json:"gateway,omitempty"`
	GatewayTxnID   string    `gorm:"uniqueIndex" json:"gateway_txn_id,omitempty"`
	IdempotencyKey string    `gorm:"uniqueIndex;not null" json:"-"`
	CreatedAt      time.Time `json:"created_at"`
}

// OutboxEvent mirrors the outbox_events table.
type OutboxEvent struct {
	ID        uint      `gorm:"primaryKey;autoIncrement" json:"id"`
	EventType string    `json:"event_type"`
	Payload   JSONMap   `gorm:"type:jsonb" json:"payload"`
	Published bool      `gorm:"default:false" json:"published"`
	CreatedAt time.Time `json:"created_at"`
}

// --- Request/response DTOs ------------------------------------------------

type PlaceOrderRequest struct {
	RestaurantID    uint   `json:"restaurant_id" binding:"required"`
	MenuItemID      uint   `json:"menu_item_id" binding:"required"`
	DeliveryAddress string `json:"delivery_address" binding:"required"`
	Notes           string `json:"notes"`
}

// CancelOrderRequest is empty; path param used instead.
type CancelOrderRequest struct{}

// OwnerUpdateStatusRequest for restaurant-owner status updates.
type OwnerUpdateStatusRequest struct {
	Status string `json:"status" binding:"required,oneof=CONFIRMED PREPARING PREPARED"`
}

// InternalUpdateStatusRequest for internal/delivery status updates.
type InternalUpdateStatusRequest struct {
	Status string `json:"status" binding:"required,oneof=CONFIRMED PREPARING PREPARED OUT_FOR_DELIVERY DELIVERED FAILED"`
}

// standard response wrappers
type ErrorResponse struct {
	Error   string `json:"error"`
	Message string `json:"message,omitempty"`
}

type SuccessResponse struct {
	Message string `json:"message"`
}

// OrderStats holds aggregated order metrics for the admin dashboard.
type OrderStats struct {
	TotalOrders    int64   `json:"total_orders"`
	TotalRevenue   float64 `json:"total_revenue"`
	TotalDelivered int64   `json:"total_delivered"`
	TotalCancelled int64   `json:"total_cancelled"`
}

// Health check response
type HealthResponse struct {
	Service string `json:"service"`
	Status  string `json:"status"`
}
