package service

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/food-platform/restaurant-service/internal/models"
	"github.com/food-platform/restaurant-service/internal/repository"
	"gorm.io/gorm"
)

var (
	ErrNotFound  = errors.New("not found")
	ErrForbidden = errors.New("forbidden")
	ErrBadStatus = errors.New("invalid order status transition")
)

// ── Restaurant service ─────────────────────────────────────────

type RestaurantService interface {
	Create(ownerID uint, req *models.CreateRestaurantRequest) (*models.Restaurant, error)
	GetByID(id uint) (*models.Restaurant, error)
	GetByIDWithMenu(id uint) (*models.Restaurant, error)
	ListApprovedOpen() ([]models.Restaurant, error)
	Search(query string) ([]models.Restaurant, error)
	Nearby(lat, lng, radiusKm float64) ([]models.Restaurant, error)
	Update(id uint, callerID uint, callerRole string, req *models.UpdateRestaurantRequest) (*models.Restaurant, error)
	Delete(id uint, callerID uint, callerRole string) error
	ToggleStatus(id uint, callerID uint, callerRole string) (*models.Restaurant, error)
	Approve(id uint) (*models.Restaurant, error)
	UpdateOrderStatus(restaurantID uint, callerID uint, callerRole string, req *models.OrderStatusRequest) error
}

type restaurantService struct {
	repo            repository.RestaurantRepository
	outboxRepo      repository.OutboxRepository
	db              *gorm.DB
	orderServiceURL string
}

func NewRestaurantService(repo repository.RestaurantRepository, outboxRepo repository.OutboxRepository, db *gorm.DB, orderServiceURL string) RestaurantService {
	return &restaurantService{repo: repo, outboxRepo: outboxRepo, db: db, orderServiceURL: orderServiceURL}
}

func (s *restaurantService) Create(ownerID uint, req *models.CreateRestaurantRequest) (*models.Restaurant, error) {
	r := &models.Restaurant{
		OwnerID:    ownerID,
		Name:       req.Name,
		Address:    req.Address,
		Latitude:   req.Latitude,
		Longitude:  req.Longitude,
		Cuisine:    req.Cuisine,
		IsApproved: false,
		IsOpen:     true,
	}
	if err := s.repo.Create(r); err != nil {
		return nil, err
	}
	return r, nil
}

func (s *restaurantService) GetByID(id uint) (*models.Restaurant, error) {
	r, err := s.repo.GetByID(id)
	if err != nil {
		return nil, err
	}
	if r == nil {
		return nil, ErrNotFound
	}
	return r, nil
}

func (s *restaurantService) GetByIDWithMenu(id uint) (*models.Restaurant, error) {
	r, err := s.repo.GetByIDWithMenu(id)
	if err != nil {
		return nil, err
	}
	if r == nil {
		return nil, ErrNotFound
	}
	return r, nil
}

func (s *restaurantService) ListApprovedOpen() ([]models.Restaurant, error) {
	return s.repo.ListApprovedOpen()
}

func (s *restaurantService) Search(query string) ([]models.Restaurant, error) {
	return s.repo.Search(query)
}

func (s *restaurantService) Nearby(lat, lng, radiusKm float64) ([]models.Restaurant, error) {
	return s.repo.Nearby(lat, lng, radiusKm)
}

func (s *restaurantService) Update(id uint, callerID uint, callerRole string, req *models.UpdateRestaurantRequest) (*models.Restaurant, error) {
	r, err := s.repo.GetByID(id)
	if err != nil {
		return nil, err
	}
	if r == nil {
		return nil, ErrNotFound
	}
	if callerRole != "ADMIN" && callerID != r.OwnerID {
		return nil, ErrForbidden
	}
	if req.Name != "" {
		r.Name = req.Name
	}
	if req.Address != "" {
		r.Address = req.Address
	}
	if req.Latitude != 0 {
		r.Latitude = req.Latitude
	}
	if req.Longitude != 0 {
		r.Longitude = req.Longitude
	}
	if req.Cuisine != "" {
		r.Cuisine = req.Cuisine
	}
	if err := s.repo.Update(r); err != nil {
		return nil, err
	}
	return r, nil
}

func (s *restaurantService) Delete(id uint, callerID uint, callerRole string) error {
	r, err := s.repo.GetByID(id)
	if err != nil {
		return err
	}
	if r == nil {
		return ErrNotFound
	}
	if callerRole != "ADMIN" && callerID != r.OwnerID {
		return ErrForbidden
	}
	return s.repo.Delete(id)
}

func (s *restaurantService) ToggleStatus(id uint, callerID uint, callerRole string) (*models.Restaurant, error) {
	r, err := s.repo.GetByID(id)
	if err != nil {
		return nil, err
	}
	if r == nil {
		return nil, ErrNotFound
	}
	if callerRole != "ADMIN" && callerID != r.OwnerID {
		return nil, ErrForbidden
	}
	r.IsOpen = !r.IsOpen
	if err := s.repo.Update(r); err != nil {
		return nil, err
	}
	return r, nil
}

func (s *restaurantService) Approve(id uint) (*models.Restaurant, error) {
	r, err := s.repo.GetByID(id)
	if err != nil {
		return nil, err
	}
	if r == nil {
		return nil, ErrNotFound
	}
	r.IsApproved = true
	if err := s.repo.Update(r); err != nil {
		return nil, err
	}
	return r, nil
}

// UpdateOrderStatus calls order-service internal API and fires ORDER_PREPARED via outbox.
func (s *restaurantService) UpdateOrderStatus(restaurantID uint, callerID uint, callerRole string, req *models.OrderStatusRequest) error {
	r, err := s.repo.GetByID(restaurantID)
	if err != nil {
		return err
	}
	if r == nil {
		return ErrNotFound
	}
	if callerRole != "ADMIN" && callerID != r.OwnerID {
		return ErrForbidden
	}

	// Call order-service internal API: PATCH /api/v1/internal/orders/:id/status
	url := fmt.Sprintf("%s/api/v1/internal/orders/%d/status", s.orderServiceURL, req.OrderID)
	body := fmt.Sprintf(`{"status":"%s"}`, req.Status)
	httpReq, err := http.NewRequest(http.MethodPatch, url, strings.NewReader(body))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(httpReq)
	if err != nil {
		return fmt.Errorf("order-service unreachable: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return fmt.Errorf("order-service returned %d", resp.StatusCode)
	}

	// If PREPARED, write outbox event in a transaction
	if req.Status == "PREPARED" {
		payload, _ := json.Marshal(map[string]interface{}{
			"order_id":      req.OrderID,
			"restaurant_id": restaurantID,
		})
		return s.db.Transaction(func(tx *gorm.DB) error {
			return s.outboxRepo.Create(tx, &models.OutboxEvent{
				EventType: "ORDER_PREPARED",
				Payload:   string(payload),
			})
		})
	}
	return nil
}

// ── Menu item service ──────────────────────────────────────────

type MenuItemService interface {
	Create(restaurantID uint, callerID uint, callerRole string, req *models.CreateMenuItemRequest) (*models.MenuItem, error)
	GetByID(id uint) (*models.MenuItem, error)
	ListByRestaurant(restaurantID uint) ([]models.MenuItem, error)
	Update(restaurantID uint, itemID uint, callerID uint, callerRole string, req *models.UpdateMenuItemRequest) (*models.MenuItem, error)
	Delete(restaurantID uint, itemID uint, callerID uint, callerRole string) error
	ToggleAvailability(restaurantID uint, itemID uint, callerID uint, callerRole string) (*models.MenuItem, error)
}

type menuItemService struct {
	itemRepo       repository.MenuItemRepository
	restaurantRepo repository.RestaurantRepository
}

func NewMenuItemService(ir repository.MenuItemRepository, rr repository.RestaurantRepository) MenuItemService {
	return &menuItemService{itemRepo: ir, restaurantRepo: rr}
}

func (s *menuItemService) Create(restaurantID uint, callerID uint, callerRole string, req *models.CreateMenuItemRequest) (*models.MenuItem, error) {
	rest, err := s.restaurantRepo.GetByID(restaurantID)
	if err != nil {
		return nil, err
	}
	if rest == nil {
		return nil, ErrNotFound
	}
	if callerRole != "ADMIN" && callerID != rest.OwnerID {
		return nil, ErrForbidden
	}
	isVeg := true
	if req.IsVeg != nil {
		isVeg = *req.IsVeg
	}
	item := &models.MenuItem{
		RestaurantID: restaurantID,
		Name:         req.Name,
		Description:  req.Description,
		Price:        req.Price,
		Category:     req.Category,
		IsVeg:        isVeg,
		IsAvailable:  true,
		ImageURL:     req.ImageURL,
	}
	if err := s.itemRepo.Create(item); err != nil {
		return nil, err
	}
	return item, nil
}

func (s *menuItemService) GetByID(id uint) (*models.MenuItem, error) {
	item, err := s.itemRepo.GetByID(id)
	if err != nil {
		return nil, err
	}
	if item == nil {
		return nil, ErrNotFound
	}
	return item, nil
}

func (s *menuItemService) ListByRestaurant(restaurantID uint) ([]models.MenuItem, error) {
	return s.itemRepo.ListByRestaurant(restaurantID)
}

func (s *menuItemService) Update(restaurantID uint, itemID uint, callerID uint, callerRole string, req *models.UpdateMenuItemRequest) (*models.MenuItem, error) {
	rest, err := s.restaurantRepo.GetByID(restaurantID)
	if err != nil || rest == nil {
		return nil, ErrNotFound
	}
	if callerRole != "ADMIN" && callerID != rest.OwnerID {
		return nil, ErrForbidden
	}
	item, err := s.itemRepo.GetByID(itemID)
	if err != nil || item == nil || item.RestaurantID != restaurantID {
		return nil, ErrNotFound
	}
	if req.Name != "" {
		item.Name = req.Name
	}
	if req.Description != "" {
		item.Description = req.Description
	}
	if req.Price > 0 {
		item.Price = req.Price
	}
	if req.Category != "" {
		item.Category = req.Category
	}
	if req.IsVeg != nil {
		item.IsVeg = *req.IsVeg
	}
	if req.ImageURL != "" {
		item.ImageURL = req.ImageURL
	}
	if err := s.itemRepo.Update(item); err != nil {
		return nil, err
	}
	return item, nil
}

func (s *menuItemService) Delete(restaurantID uint, itemID uint, callerID uint, callerRole string) error {
	rest, err := s.restaurantRepo.GetByID(restaurantID)
	if err != nil || rest == nil {
		return ErrNotFound
	}
	if callerRole != "ADMIN" && callerID != rest.OwnerID {
		return ErrForbidden
	}
	item, err := s.itemRepo.GetByID(itemID)
	if err != nil || item == nil || item.RestaurantID != restaurantID {
		return ErrNotFound
	}
	return s.itemRepo.Delete(itemID)
}

func (s *menuItemService) ToggleAvailability(restaurantID uint, itemID uint, callerID uint, callerRole string) (*models.MenuItem, error) {
	rest, err := s.restaurantRepo.GetByID(restaurantID)
	if err != nil || rest == nil {
		return nil, ErrNotFound
	}
	if callerRole != "ADMIN" && callerID != rest.OwnerID {
		return nil, ErrForbidden
	}
	item, err := s.itemRepo.GetByID(itemID)
	if err != nil || item == nil || item.RestaurantID != restaurantID {
		return nil, ErrNotFound
	}
	item.IsAvailable = !item.IsAvailable
	if err := s.itemRepo.Update(item); err != nil {
		return nil, err
	}
	return item, nil
}
