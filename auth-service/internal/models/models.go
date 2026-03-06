package models

import "time"

// ─── DB Models ───────────────────────────────────────────────────────────────

// User maps to the `users` table in auth_db.
type User struct {
	ID         uint       `gorm:"primaryKey;autoIncrement" json:"id"`
	Email      string     `gorm:"uniqueIndex;not null"     json:"email"`
	Password   string     `gorm:"not null"                 json:"-"` // never serialise
	Phone      string     `gorm:"uniqueIndex"              json:"phone"`
	Role       string     `gorm:"default:USER"             json:"role"`
	IsVerified bool       `gorm:"default:false"            json:"is_verified"`
	IsDeleted  bool       `gorm:"default:false;index"      json:"is_deleted"`
	DeletedAt  *time.Time `gorm:"index"                    json:"deleted_at,omitempty"`
	CreatedAt  time.Time  `                                json:"created_at"`
	UpdatedAt  time.Time  `                                json:"updated_at"`
}

// OTP maps to the `otps` table.
type OTP struct {
	ID        uint      `gorm:"primaryKey;autoIncrement"`
	Phone     string    `gorm:"not null"`
	Code      string    `gorm:"not null"`
	ExpiresAt time.Time `gorm:"not null"`
	Used      bool      `gorm:"default:false"`
}

// ─── Request Bodies ───────────────────────────────────────────────────────────

type RegisterRequest struct {
	Name     string `json:"name"     binding:"required"`
	Email    string `json:"email"    binding:"required,email"`
	Password string `json:"password" binding:"required,min=8"`
	Phone    string `json:"phone"    binding:"required"`
	Role     string `json:"role"` // optional, defaults to USER
}

type LoginRequest struct {
	Email    string `json:"email"    binding:"required,email"`
	Password string `json:"password" binding:"required"`
}

type RefreshRequest struct {
	RefreshToken string `json:"refresh_token" binding:"required"`
}

type SendOTPRequest struct {
	Phone string `json:"phone" binding:"required"`
}

type VerifyOTPRequest struct {
	Phone string `json:"phone" binding:"required"`
	Code  string `json:"code"  binding:"required,len=6"`
}

// ─── Response Bodies ──────────────────────────────────────────────────────────

type AuthResponse struct {
	AccessToken  string       `json:"access_token"`
	RefreshToken string       `json:"refresh_token"`
	User         UserResponse `json:"user"`
}

type UserResponse struct {
	ID    uint   `json:"id"`
	Email string `json:"email"`
	Phone string `json:"phone"`
	Role  string `json:"role"`
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
	Uptime  string `json:"uptime"`
}

// ─── Event Models ─────────────────────────────────────────────────────────────

// UserCreatedEvent published when a user registers successfully.
type UserCreatedEvent struct {
	Event     string    `json:"event"`
	UserID    uint      `json:"user_id"`
	Name      string    `json:"name"`
	Email     string    `json:"email"`
	Phone     string    `json:"phone,omitempty"`
	Role      string    `json:"role"`
	Timestamp time.Time `json:"timestamp"`
}

// UserDeletedEvent published when a user account is deleted.
type UserDeletedEvent struct {
	Event     string    `json:"event"`
	UserID    uint      `json:"user_id"`
	Email     string    `json:"email"`
	Timestamp time.Time `json:"timestamp"`
}
