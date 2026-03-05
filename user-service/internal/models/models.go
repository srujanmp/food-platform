package models

import (
	"time"

	"gorm.io/gorm"
)

// ────────────────────────────────────────────────────────────
// DB models
// ────────────────────────────────────────────────────────────

type Profile struct {
	ID        uint           `gorm:"primaryKey" json:"id"`
	AuthID    uint           `gorm:"uniqueIndex;not null" json:"auth_id"`
	Name      string         `gorm:"size:100" json:"name"`
	AvatarURL string         `gorm:"size:255" json:"avatar_url"`
	CreatedAt time.Time      `json:"created_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"` // soft-delete
}

type Address struct {
	ID        uint    `gorm:"primaryKey" json:"id"`
	UserID    uint    `gorm:"not null;index" json:"user_id"`
	Label     string  `gorm:"size:50" json:"label"`   // Home | Work | Other
	Line1     string  `gorm:"type:text" json:"line1"` //nolint
	City      string  `gorm:"size:100" json:"city"`
	Pincode   string  `gorm:"size:10" json:"pincode"`
	Latitude  float64 `gorm:"type:decimal(10,8)" json:"latitude"`
	Longitude float64 `gorm:"type:decimal(11,8)" json:"longitude"`
	IsDefault bool    `gorm:"default:false" json:"is_default"`
}

// ────────────────────────────────────────────────────────────
// Request / Response DTOs
// ────────────────────────────────────────────────────────────

type UpdateProfileRequest struct {
	Name      string `json:"name"`
	AvatarURL string `json:"avatar_url"`
}

type AddAddressRequest struct {
	Label     string  `json:"label" binding:"required"`
	Line1     string  `json:"line1" binding:"required"`
	City      string  `json:"city" binding:"required"`
	Pincode   string  `json:"pincode" binding:"required"`
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
	IsDefault bool    `json:"is_default"`
}

type UpdateAddressRequest struct {
	Label     string  `json:"label"`
	Line1     string  `json:"line1"`
	City      string  `json:"city"`
	Pincode   string  `json:"pincode"`
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
	IsDefault bool    `json:"is_default"`
}

type ErrorResponse struct {
	Error string `json:"error"`
}

type MessageResponse struct {
	Message string `json:"message"`
}
