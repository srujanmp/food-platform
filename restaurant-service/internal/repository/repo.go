package repository

import (
	"errors"
	"math"

	"github.com/food-platform/restaurant-service/internal/models"
	"gorm.io/gorm"
)

// ── Interfaces ─────────────────────────────────────────────────

type RestaurantRepository interface {
	Create(r *models.Restaurant) error
	GetByID(id uint) (*models.Restaurant, error)
	GetByIDWithMenu(id uint) (*models.Restaurant, error)
	ListAll() ([]models.Restaurant, error)
	ListApprovedOpen() ([]models.Restaurant, error)
	ListByOwner(ownerID uint) ([]models.Restaurant, error)
	Search(query string) ([]models.Restaurant, error)
	Nearby(lat, lng, radiusKm float64) ([]models.Restaurant, error)
	Update(r *models.Restaurant) error
	Delete(id uint) error
}

type MenuItemRepository interface {
	Create(item *models.MenuItem) error
	GetByID(id uint) (*models.MenuItem, error)
	ListByRestaurant(restaurantID uint) ([]models.MenuItem, error)
	Update(item *models.MenuItem) error
	Delete(id uint) error
}

type OutboxRepository interface {
	Create(tx *gorm.DB, event *models.OutboxEvent) error
	ListUnpublished(limit int) ([]models.OutboxEvent, error)
	MarkPublished(id uint) error
}

// ── Restaurant implementation ──────────────────────────────────

type restaurantRepo struct{ db *gorm.DB }

func NewRestaurantRepository(db *gorm.DB) RestaurantRepository {
	return &restaurantRepo{db}
}

func (r *restaurantRepo) Create(rest *models.Restaurant) error {
	return r.db.Create(rest).Error
}

func (r *restaurantRepo) GetByID(id uint) (*models.Restaurant, error) {
	var rest models.Restaurant
	err := r.db.First(&rest, id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	return &rest, err
}

func (r *restaurantRepo) GetByIDWithMenu(id uint) (*models.Restaurant, error) {
	var rest models.Restaurant
	err := r.db.Preload("MenuItems").First(&rest, id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	return &rest, err
}

func (r *restaurantRepo) ListAll() ([]models.Restaurant, error) {
	var restaurants []models.Restaurant
	err := r.db.Order("created_at DESC").Find(&restaurants).Error
	return restaurants, err
}

func (r *restaurantRepo) ListApprovedOpen() ([]models.Restaurant, error) {
	var restaurants []models.Restaurant
	err := r.db.Where("is_approved = ? AND is_open = ?", true, true).Order("created_at DESC").Find(&restaurants).Error
	return restaurants, err
}

func (r *restaurantRepo) ListByOwner(ownerID uint) ([]models.Restaurant, error) {
	var restaurants []models.Restaurant
	err := r.db.Where("owner_id = ?", ownerID).Order("created_at DESC").Find(&restaurants).Error
	return restaurants, err
}

func (r *restaurantRepo) Search(query string) ([]models.Restaurant, error) {
	var restaurants []models.Restaurant
	pattern := "%" + query + "%"
	err := r.db.Where("is_approved = ? AND (LOWER(name) LIKE LOWER(?) OR LOWER(cuisine) LIKE LOWER(?))", true, pattern, pattern).
		Order("created_at DESC").Find(&restaurants).Error
	return restaurants, err
}

func (r *restaurantRepo) Nearby(lat, lng, radiusKm float64) ([]models.Restaurant, error) {
	var all []models.Restaurant
	err := r.db.Where("is_approved = ? AND is_open = ?", true, true).Find(&all).Error
	if err != nil {
		return nil, err
	}
	var result []models.Restaurant
	for _, rest := range all {
		if haversine(lat, lng, rest.Latitude, rest.Longitude) <= radiusKm {
			result = append(result, rest)
		}
	}
	return result, nil
}

func (r *restaurantRepo) Update(rest *models.Restaurant) error {
	return r.db.Save(rest).Error
}

func (r *restaurantRepo) Delete(id uint) error {
	return r.db.Delete(&models.Restaurant{}, id).Error
}

// haversine returns the distance in km between two lat/lng points.
func haversine(lat1, lon1, lat2, lon2 float64) float64 {
	const R = 6371.0
	dLat := (lat2 - lat1) * math.Pi / 180
	dLon := (lon2 - lon1) * math.Pi / 180
	la1 := lat1 * math.Pi / 180
	la2 := lat2 * math.Pi / 180
	a := math.Sin(dLat/2)*math.Sin(dLat/2) + math.Cos(la1)*math.Cos(la2)*math.Sin(dLon/2)*math.Sin(dLon/2)
	c := 2 * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))
	return R * c
}

// ── MenuItem implementation ────────────────────────────────────

type menuItemRepo struct{ db *gorm.DB }

func NewMenuItemRepository(db *gorm.DB) MenuItemRepository {
	return &menuItemRepo{db}
}

func (r *menuItemRepo) Create(item *models.MenuItem) error {
	return r.db.Create(item).Error
}

func (r *menuItemRepo) GetByID(id uint) (*models.MenuItem, error) {
	var item models.MenuItem
	err := r.db.First(&item, id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	return &item, err
}

func (r *menuItemRepo) ListByRestaurant(restaurantID uint) ([]models.MenuItem, error) {
	var items []models.MenuItem
	err := r.db.Where("restaurant_id = ?", restaurantID).Order("category, name").Find(&items).Error
	return items, err
}

func (r *menuItemRepo) Update(item *models.MenuItem) error {
	return r.db.Save(item).Error
}

func (r *menuItemRepo) Delete(id uint) error {
	return r.db.Delete(&models.MenuItem{}, id).Error
}

// ── Outbox implementation ──────────────────────────────────────

type outboxRepo struct{ db *gorm.DB }

func NewOutboxRepository(db *gorm.DB) OutboxRepository {
	return &outboxRepo{db: db}
}

func (r *outboxRepo) Create(tx *gorm.DB, event *models.OutboxEvent) error {
	return tx.Create(event).Error
}

func (r *outboxRepo) ListUnpublished(limit int) ([]models.OutboxEvent, error) {
	var events []models.OutboxEvent
	err := r.db.Where("published = ?", false).Order("id ASC").Limit(limit).Find(&events).Error
	return events, err
}

func (r *outboxRepo) MarkPublished(id uint) error {
	return r.db.Model(&models.OutboxEvent{}).Where("id = ?", id).Update("published", true).Error
}
