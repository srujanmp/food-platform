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

// ── DB Models ────────────────────────────────────────────────────────────────

// Driver maps to the `drivers` table.
type Driver struct {
	ID          uint      `gorm:"primaryKey;autoIncrement" json:"id"`
	AuthID      uint      `gorm:"uniqueIndex;not null"     json:"auth_id"`
	Name        string    `gorm:"size:100"                 json:"name"`
	Phone       string    `gorm:"size:20"                  json:"phone"`
	Latitude    float64   `gorm:"type:decimal(10,8)"       json:"latitude"`
	Longitude   float64   `gorm:"type:decimal(11,8)"       json:"longitude"`
	IsAvailable bool      `gorm:"default:true"             json:"is_available"`
	CreatedAt   time.Time `                                json:"created_at"`
	UpdatedAt   time.Time `                                json:"updated_at"`
}

// Delivery maps to the `deliveries` table.
type Delivery struct {
	ID          uint       `gorm:"primaryKey;autoIncrement" json:"id"`
	OrderID     uint       `gorm:"uniqueIndex;not null"     json:"order_id"`
	DriverID    uint       `gorm:"not null"                 json:"driver_id"`
	UserID      uint       `gorm:"not null;default:0"       json:"user_id"`
	Status      string     `gorm:"size:30;not null;default:ASSIGNED" json:"status"`
	AssignedAt  time.Time  `gorm:"not null;default:now()"   json:"assigned_at"`
	DeliveredAt *time.Time `                                json:"delivered_at,omitempty"`
	CreatedAt   time.Time  `                                json:"created_at"`
	UpdatedAt   time.Time  `                                json:"updated_at"`
	Driver      *Driver    `gorm:"foreignKey:DriverID"      json:"driver,omitempty"`
}

// OutboxEvent mirrors the outbox_events table.
type OutboxEvent struct {
	ID        uint      `gorm:"primaryKey;autoIncrement" json:"id"`
	EventType string    `json:"event_type"`
	Payload   JSONMap   `gorm:"type:jsonb" json:"payload"`
	Published bool      `gorm:"default:false" json:"published"`
	CreatedAt time.Time `json:"created_at"`
}

// PendingAssignment tracks ORDER_PREPARED events that haven't been assigned yet.
type PendingAssignment struct {
	ID        uint      `gorm:"primaryKey;autoIncrement"`
	OrderID   uint      `gorm:"uniqueIndex;not null"`
	UserID    uint      `gorm:"not null"`
	CreatedAt time.Time `gorm:"default:now()"`
}

// ── Request/Response DTOs ────────────────────────────────────────────────────

type UpdateLocationRequest struct {
	Latitude  float64 `json:"latitude"  binding:"required"`
	Longitude float64 `json:"longitude" binding:"required"`
}

type UpdateStatusRequest struct {
	Status string `json:"status" binding:"required,oneof=OUT_FOR_DELIVERY DELIVERED FAILED"`
}

type ErrorResponse struct {
	Error   string `json:"error"`
	Message string `json:"message,omitempty"`
}

type SuccessResponse struct {
	Message string `json:"message"`
}

type HealthResponse struct {
	Service string `json:"service"`
	Status  string `json:"status"`
}

type TrackingResponse struct {
	OrderID   uint    `json:"order_id"`
	Status    string  `json:"status"`
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
}
