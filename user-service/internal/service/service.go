package service

import (
	"errors"

	"github.com/food-platform/user-service/internal/models"
	"github.com/food-platform/user-service/internal/repository"
)

var (
	ErrNotFound   = errors.New("not found")
	ErrForbidden  = errors.New("forbidden")
	ErrBadRequest = errors.New("bad request")
)

// ────────────────────────────────────────────────────────────
// Profile service
// ────────────────────────────────────────────────────────────

type ProfileService interface {
	EnsureProfile(authID uint, name string) (*models.Profile, error)
	GetProfile(authID uint, callerID uint, callerRole string) (*models.Profile, error)
	UpdateProfile(authID uint, callerID uint, callerRole string, req *models.UpdateProfileRequest) (*models.Profile, error)
	DeleteProfile(authID uint, callerID uint, callerRole string) error
	ListAllProfiles() ([]models.Profile, error)
	BanProfile(authID uint) error
}

type profileService struct {
	profileRepo repository.ProfileRepository
}

func NewProfileService(pr repository.ProfileRepository) ProfileService {
	return &profileService{profileRepo: pr}
}

// EnsureProfile creates a profile if one does not exist for this authID.
// Idempotent — safe to call multiple times.
func (s *profileService) EnsureProfile(authID uint, name string) (*models.Profile, error) {
	existing, err := s.profileRepo.GetByAuthID(authID)
	if err != nil {
		return nil, err
	}
	if existing != nil {
		return existing, nil
	}
	p := &models.Profile{AuthID: authID, Name: name}
	if err := s.profileRepo.Create(p); err != nil {
		return nil, err
	}
	return p, nil
}

// GetProfile — :id in the URL is the auth_id (value from JWT user_id claim).
func (s *profileService) GetProfile(authID uint, callerID uint, callerRole string) (*models.Profile, error) {
	p, err := s.profileRepo.GetByAuthID(authID)
	if err != nil {
		return nil, err
	}
	if p == nil {
		return nil, ErrNotFound
	}
	if callerRole != "ADMIN" && callerID != p.AuthID {
		return nil, ErrForbidden
	}
	return p, nil
}

// UpdateProfile — :id in the URL is the auth_id.
func (s *profileService) UpdateProfile(authID uint, callerID uint, callerRole string, req *models.UpdateProfileRequest) (*models.Profile, error) {
	p, err := s.profileRepo.GetByAuthID(authID)
	if err != nil {
		return nil, err
	}
	if p == nil {
		return nil, ErrNotFound
	}
	if callerRole != "ADMIN" && callerID != p.AuthID {
		return nil, ErrForbidden
	}
	if req.Name != "" {
		p.Name = req.Name
	}
	if req.AvatarURL != "" {
		p.AvatarURL = req.AvatarURL
	}
	if err := s.profileRepo.Update(p); err != nil {
		return nil, err
	}
	return p, nil
}

// DeleteProfile — :id in the URL is the auth_id.
func (s *profileService) DeleteProfile(authID uint, callerID uint, callerRole string) error {
	p, err := s.profileRepo.GetByAuthID(authID)
	if err != nil {
		return err
	}
	if p == nil {
		return ErrNotFound
	}
	if callerRole != "ADMIN" && callerID != p.AuthID {
		return ErrForbidden
	}
	return s.profileRepo.SoftDelete(p.AuthID)
}

// ListAllProfiles returns all profiles (for admin internal use).
func (s *profileService) ListAllProfiles() ([]models.Profile, error) {
	return s.profileRepo.ListAll()
}

// BanProfile soft-deletes a profile by auth_id (for admin internal use).
func (s *profileService) BanProfile(authID uint) error {
	p, err := s.profileRepo.GetByAuthID(authID)
	if err != nil {
		return err
	}
	if p == nil {
		return ErrNotFound
	}
	return s.profileRepo.SoftDelete(p.AuthID)
}

// ────────────────────────────────────────────────────────────
// Address service
// ────────────────────────────────────────────────────────────

type AddressService interface {
	ListAddresses(authID uint, callerID uint, callerRole string) ([]models.Address, error)
	AddAddress(authID uint, callerID uint, callerRole string, req *models.AddAddressRequest) (*models.Address, error)
	UpdateAddress(addressID uint, callerID uint, callerRole string, req *models.UpdateAddressRequest) (*models.Address, error)
	DeleteAddress(addressID uint, callerID uint, callerRole string) error
}

type addressService struct {
	addressRepo repository.AddressRepository
	profileRepo repository.ProfileRepository
}

func NewAddressService(ar repository.AddressRepository, pr repository.ProfileRepository) AddressService {
	return &addressService{addressRepo: ar, profileRepo: pr}
}

// getProfileByAuthID resolves auth_id → profile, returns ErrNotFound if missing.
func (s *addressService) getProfileByAuthID(authID uint) (*models.Profile, error) {
	p, err := s.profileRepo.GetByAuthID(authID)
	if err != nil {
		return nil, err
	}
	if p == nil {
		return nil, ErrNotFound
	}
	return p, nil
}

func (s *addressService) ListAddresses(authID uint, callerID uint, callerRole string) ([]models.Address, error) {
	p, err := s.getProfileByAuthID(authID)
	if err != nil {
		return nil, err
	}
	if callerRole != "ADMIN" && callerID != p.AuthID {
		return nil, ErrForbidden
	}
	return s.addressRepo.ListByAuthID(p.AuthID)
}

func (s *addressService) AddAddress(authID uint, callerID uint, callerRole string, req *models.AddAddressRequest) (*models.Address, error) {
	p, err := s.getProfileByAuthID(authID)
	if err != nil {
		return nil, err
	}
	if callerRole != "ADMIN" && callerID != p.AuthID {
		return nil, ErrForbidden
	}
	if req.IsDefault {
		_ = s.addressRepo.ClearDefault(p.AuthID)
	}
	a := &models.Address{
		AuthID:    p.AuthID,
		Label:     req.Label,
		Line1:     req.Line1,
		City:      req.City,
		Pincode:   req.Pincode,
		Latitude:  req.Latitude,
		Longitude: req.Longitude,
		IsDefault: req.IsDefault,
	}
	if err := s.addressRepo.Create(a); err != nil {
		return nil, err
	}
	return a, nil
}

func (s *addressService) UpdateAddress(addressID uint, callerID uint, callerRole string, req *models.UpdateAddressRequest) (*models.Address, error) {
	a, err := s.addressRepo.GetByID(addressID)
	if err != nil || a == nil {
		return nil, ErrNotFound
	}
	// Resolve profile to get auth_id for ownership check
	p, err := s.profileRepo.GetByAuthID(a.AuthID)
	if err != nil || p == nil {
		return nil, ErrNotFound
	}
	if callerRole != "ADMIN" && callerID != p.AuthID {
		return nil, ErrForbidden
	}
	if req.Label != "" {
		a.Label = req.Label
	}
	if req.Line1 != "" {
		a.Line1 = req.Line1
	}
	if req.City != "" {
		a.City = req.City
	}
	if req.Pincode != "" {
		a.Pincode = req.Pincode
	}
	if req.Latitude != 0 {
		a.Latitude = req.Latitude
	}
	if req.Longitude != 0 {
		a.Longitude = req.Longitude
	}
	if req.IsDefault {
		_ = s.addressRepo.ClearDefault(a.AuthID)
		a.IsDefault = true
	}
	if err := s.addressRepo.Update(a); err != nil {
		return nil, err
	}
	return a, nil
}

func (s *addressService) DeleteAddress(addressID uint, callerID uint, callerRole string) error {
	a, err := s.addressRepo.GetByID(addressID)
	if err != nil || a == nil {
		return ErrNotFound
	}
	p, err := s.profileRepo.GetByAuthID(a.AuthID)
	if err != nil || p == nil {
		return ErrNotFound
	}
	if callerRole != "ADMIN" && callerID != p.AuthID {
		return ErrForbidden
	}
	return s.addressRepo.Delete(addressID)
}
