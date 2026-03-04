package repository

import (
	"errors"
	"time"

	"github.com/food-platform/auth-service/internal/models"
	"gorm.io/gorm"
)

// AuthRepository defines the DB operations the service layer needs.
// Keeping it as an interface makes it easy to mock in unit tests.
type AuthRepository interface {
	CreateUser(user *models.User) error
	FindUserByEmail(email string) (*models.User, error)
	FindUserByID(id uint) (*models.User, error)
	FindUserByPhone(phone string) (*models.User, error)

	CreateOTP(otp *models.OTP) error
	FindValidOTP(phone, code string) (*models.OTP, error)
	MarkOTPUsed(id uint) error
}

type authRepository struct {
	db *gorm.DB
}

// New returns a concrete AuthRepository backed by PostgreSQL via GORM.
func New(db *gorm.DB) AuthRepository {
	return &authRepository{db: db}
}

// ─── User operations ──────────────────────────────────────────────────────────

func (r *authRepository) CreateUser(user *models.User) error {
	return r.db.Create(user).Error
}

func (r *authRepository) FindUserByEmail(email string) (*models.User, error) {
	var user models.User
	err := r.db.Where("email = ?", email).First(&user).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil // caller checks for nil
	}
	return &user, err
}

func (r *authRepository) FindUserByID(id uint) (*models.User, error) {
	var user models.User
	err := r.db.First(&user, id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	return &user, err
}

func (r *authRepository) FindUserByPhone(phone string) (*models.User, error) {
	var user models.User
	err := r.db.Where("phone = ?", phone).First(&user).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	return &user, err
}

// ─── OTP operations ───────────────────────────────────────────────────────────

func (r *authRepository) CreateOTP(otp *models.OTP) error {
	// Invalidate any previous OTPs for this phone number before creating a new one.
	r.db.Model(&models.OTP{}).
		Where("phone = ? AND used = false", otp.Phone).
		Update("used", true)
	return r.db.Create(otp).Error
}

func (r *authRepository) FindValidOTP(phone, code string) (*models.OTP, error) {
	var otp models.OTP
	err := r.db.Where(
		"phone = ? AND code = ? AND used = false AND expires_at > ?",
		phone, code, time.Now(),
	).First(&otp).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	return &otp, err
}

func (r *authRepository) MarkOTPUsed(id uint) error {
	return r.db.Model(&models.OTP{}).Where("id = ?", id).Update("used", true).Error
}