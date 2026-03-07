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
	Create(p *models.Profile) error
	Update(p *models.Profile) error
	SoftDelete(authID uint) error
	ListAll() ([]models.Profile, error)
}

type AddressRepository interface {
	ListByAuthID(authID uint) ([]models.Address, error)
	GetByID(id uint) (*models.Address, error)
	Create(a *models.Address) error
	Update(a *models.Address) error
	Delete(id uint) error
	ClearDefault(authID uint) error
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

func (r *profileRepo) Create(p *models.Profile) error {
	return r.db.Create(p).Error
}

func (r *profileRepo) Update(p *models.Profile) error {
	return r.db.Save(p).Error
}

func (r *profileRepo) SoftDelete(authID uint) error {
	return r.db.Model(&models.Profile{}).Where("auth_id = ?", authID).Delete(&models.Profile{}).Error
}

func (r *profileRepo) ListAll() ([]models.Profile, error) {
	var profiles []models.Profile
	err := r.db.Find(&profiles).Error
	return profiles, err
}

// ────────────────────────────────────────────────────────────
// Address implementation
// ────────────────────────────────────────────────────────────

type addressRepo struct{ db *gorm.DB }

func NewAddressRepository(db *gorm.DB) AddressRepository { return &addressRepo{db} }

func (r *addressRepo) ListByAuthID(authID uint) ([]models.Address, error) {
	var addrs []models.Address
	err := r.db.Where("auth_id = ?", authID).Find(&addrs).Error
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
func (r *addressRepo) ClearDefault(authID uint) error {
	return r.db.Model(&models.Address{}).
		Where("auth_id = ? AND is_default = true", authID).
		Update("is_default", false).Error
}
