package models

import (
	"time"
)

// ── Restaurant — matches restaurant_db.restaurants exactly ─────

type Restaurant struct {
	ID         uint       `gorm:"primaryKey" json:"id"`
	OwnerID    uint       `gorm:"not null;index" json:"owner_id"`
	Name       string     `gorm:"not null;size:255" json:"name"`
	Address    string     `json:"address"`
	Latitude   float64    `gorm:"type:decimal(10,8)" json:"latitude"`
	Longitude  float64    `gorm:"type:decimal(11,8)" json:"longitude"`
	Cuisine    string     `gorm:"size:100" json:"cuisine"`
	AvgRating  float64    `gorm:"type:decimal(3,2);default:0.0" json:"avg_rating"`
	IsOpen     bool       `gorm:"default:true" json:"is_open"`
	IsApproved bool       `gorm:"default:false" json:"is_approved"`
	CreatedAt  time.Time  `json:"created_at"`
	UpdatedAt  time.Time  `json:"updated_at"`
	MenuItems  []MenuItem `gorm:"foreignKey:RestaurantID" json:"menu_items,omitempty"`
}

// ── MenuItem — matches restaurant_db.menu_items exactly ────────

type MenuItem struct {
	ID           uint      `gorm:"primaryKey" json:"id"`
	RestaurantID uint      `gorm:"not null;index" json:"restaurant_id"`
	Name         string    `gorm:"not null;size:255" json:"name"`
	Description  string    `json:"description"`
	Price        float64   `gorm:"not null;type:decimal(10,2)" json:"price"`
	Category     string    `gorm:"size:100" json:"category"`
	IsVeg        bool      `gorm:"default:true" json:"is_veg"`
	IsAvailable  bool      `gorm:"default:true" json:"is_available"`
	ImageURL     string    `gorm:"size:255" json:"image_url"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// ── OutboxEvent — transactional outbox pattern ─────────────────

type OutboxEvent struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	EventType string    `gorm:"size:100;not null" json:"event_type"`
	Payload   string    `gorm:"type:jsonb;not null" json:"payload"`
	Published bool      `gorm:"default:false" json:"published"`
	CreatedAt time.Time `gorm:"default:now()" json:"created_at"`
}

// ── Request DTOs ───────────────────────────────────────────────

type CreateRestaurantRequest struct {
	Name      string  `json:"name" binding:"required"`
	Address   string  `json:"address"`
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
	Cuisine   string  `json:"cuisine"`
}

type UpdateRestaurantRequest struct {
	Name      string  `json:"name"`
	Address   string  `json:"address"`
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
	Cuisine   string  `json:"cuisine"`
}

type CreateMenuItemRequest struct {
	Name        string  `json:"name" binding:"required"`
	Description string  `json:"description"`
	Price       float64 `json:"price" binding:"required,gt=0"`
	Category    string  `json:"category"`
	IsVeg       *bool   `json:"is_veg"`
	ImageURL    string  `json:"image_url"`
}

type UpdateMenuItemRequest struct {
	Name        string  `json:"name"`
	Description string  `json:"description"`
	Price       float64 `json:"price"`
	Category    string  `json:"category"`
	IsVeg       *bool   `json:"is_veg"`
	ImageURL    string  `json:"image_url"`
}

type OrderStatusRequest struct {
	OrderID uint   `json:"order_id" binding:"required"`
	Status  string `json:"status" binding:"required,oneof=CONFIRMED PREPARING PREPARED"`
}

// ── Response DTOs ──────────────────────────────────────────────

type ErrorResponse struct {
	Error string `json:"error"`
}

type MessageResponse struct {
	Message string `json:"message"`
}
