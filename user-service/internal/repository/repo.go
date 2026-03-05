package repository

import (
	"errors"

	"github.com/food-platform/user-service/internal/models"
	"gorm.io/gorm"
)

// ────────────────────────────────────────────────────────────
// Interfaces
// ────────────────────────────────────────────────────────────

type ProfileRepository interface {
	GetByAuthID(authID uint) (*models.Profile, error)
	GetByID(id uint) (*models.Profile, error)
	Create(p *models.Profile) error
	Update(p *models.Profile) error
	SoftDelete(id uint) error
}

type AddressRepository interface {
	ListByUserID(userID uint) ([]models.Address, error)
	GetByID(id uint) (*models.Address, error)
	Create(a *models.Address) error
	Update(a *models.Address) error
	Delete(id uint) error
	ClearDefault(userID uint) error
}

// ────────────────────────────────────────────────────────────
// Profile implementation
// ────────────────────────────────────────────────────────────

type profileRepo struct{ db *gorm.DB }

func NewProfileRepository(db *gorm.DB) ProfileRepository { return &profileRepo{db} }

func (r *profileRepo) GetByAuthID(authID uint) (*models.Profile, error) {
	var p models.Profile
	err := r.db.Where("auth_id = ?", authID).First(&p).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	return &p, err
}

func (r *profileRepo) GetByID(id uint) (*models.Profile, error) {
	var p models.Profile
	err := r.db.First(&p, id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	return &p, err
}

func (r *profileRepo) Create(p *models.Profile) error {
	return r.db.Create(p).Error
}

func (r *profileRepo) Update(p *models.Profile) error {
	return r.db.Save(p).Error
}

func (r *profileRepo) SoftDelete(id uint) error {
	return r.db.Delete(&models.Profile{}, id).Error
}

// ────────────────────────────────────────────────────────────
// Address implementation
// ────────────────────────────────────────────────────────────

type addressRepo struct{ db *gorm.DB }

func NewAddressRepository(db *gorm.DB) AddressRepository { return &addressRepo{db} }

func (r *addressRepo) ListByUserID(userID uint) ([]models.Address, error) {
	var addrs []models.Address
	err := r.db.Where("user_id = ?", userID).Find(&addrs).Error
	return addrs, err
}

func (r *addressRepo) GetByID(id uint) (*models.Address, error) {
	var a models.Address
	err := r.db.First(&a, id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	return &a, err
}

func (r *addressRepo) Create(a *models.Address) error {
	return r.db.Create(a).Error
}

func (r *addressRepo) Update(a *models.Address) error {
	return r.db.Save(a).Error
}

func (r *addressRepo) Delete(id uint) error {
	return r.db.Delete(&models.Address{}, id).Error
}

// ClearDefault unsets is_default for all addresses of a user before setting a new default.
func (r *addressRepo) ClearDefault(userID uint) error {
	return r.db.Model(&models.Address{}).
		Where("user_id = ? AND is_default = true", userID).
		Update("is_default", false).Error
}
