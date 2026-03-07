package service

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"time"

	"github.com/food-platform/order-service/internal/models"
	"github.com/food-platform/order-service/internal/repository"
	"github.com/sony/gobreaker"
	"gorm.io/gorm"
)

var (
	ErrNotFound          = errors.New("not found")
	ErrForbidden         = errors.New("forbidden")
	ErrInvalidStatus     = errors.New("invalid status transition")
	ErrIdempotencyExists = errors.New("idempotency_key_conflict")
	ErrRestaurantUnavail = errors.New("restaurant_unavailable")
	ErrMenuItemUnavail   = errors.New("menu_item_unavailable")
	ErrCircuitOpen       = errors.New("service_unavailable")
)

// OrderService defines business operations used by handlers.
type OrderService interface {
	PlaceOrder(userID uint, key string, req *models.PlaceOrderRequest) (*models.Order, error)
	GetOrder(id uint) (*models.Order, error)
	ListByUser(userID uint) ([]models.Order, error)
	ListByRestaurant(restID uint) ([]models.Order, error)
	CancelOrder(userID uint, orderID uint) error
	UpdateStatusByOwner(ownerUserID uint, orderID uint, status string) error
	UpdateStatus(orderID uint, status string) error
	GetStats() (*models.OrderStats, error)
}

// orderService is concrete implementation.
type orderService struct {
	repo          repository.OrderRepository
	payRepo       repository.PaymentRepository
	outboxRepo    repository.OutboxRepository
	db            *gorm.DB
	httpClient    *http.Client
	cb            *gobreaker.CircuitBreaker
	restaurantURL string
}

// NewOrderService constructs a service with dependencies.
func NewOrderService(repo repository.OrderRepository, payRepo repository.PaymentRepository, outboxRepo repository.OutboxRepository, db *gorm.DB, restaurantURL string) OrderService {
	cb := gobreaker.NewCircuitBreaker(gobreaker.Settings{
		Name:        "restaurant-client",
		MaxRequests: 1,
		Interval:    60 * time.Second,
		Timeout:     30 * time.Second,
		ReadyToTrip: func(counts gobreaker.Counts) bool {
			return counts.ConsecutiveFailures >= 5
		},
	})
	return &orderService{
		repo:          repo,
		payRepo:       payRepo,
		outboxRepo:    outboxRepo,
		db:            db,
		httpClient:    &http.Client{Timeout: 3 * time.Second},
		cb:            cb,
		restaurantURL: restaurantURL,
	}
}

// PlaceOrder implements the full order placement flow:
// 1. Idempotency check  2. Validate restaurant  3. Validate menu item + snapshot
// 4. Simulate payment   5. Atomic DB write (order + payment + outbox)
func (s *orderService) PlaceOrder(userID uint, key string, req *models.PlaceOrderRequest) (*models.Order, error) {
	// 1. idempotency check
	existing, err := s.payRepo.GetByIdempotencyKey(key)
	if err != nil {
		return nil, err
	}
	if existing != nil {
		return nil, ErrIdempotencyExists
	}

	// 2. validate restaurant via internal API
	restInfo, err := s.fetchRestaurant(req.RestaurantID)
	if err != nil {
		return nil, err
	}
	if !restInfo.IsApproved || !restInfo.IsOpen {
		return nil, ErrRestaurantUnavail
	}

	// 3. validate menu item + snapshot name/price
	menuInfo, err := s.fetchMenuItem(req.RestaurantID, req.MenuItemID)
	if err != nil {
		return nil, err
	}
	if !menuInfo.IsAvailable {
		return nil, ErrMenuItemUnavail
	}

	order := &models.Order{
		UserID:          userID,
		RestaurantID:    req.RestaurantID,
		MenuItemID:      req.MenuItemID,
		DeliveryAddress: req.DeliveryAddress,
		Notes:           req.Notes,
		ItemName:        menuInfo.Name,
		ItemPrice:       menuInfo.Price,
		Status:          "PLACED",
		IdempotencyKey:  key,
	}

	// 4. simulate payment
	payment := &models.Payment{
		UserID:         userID,
		Amount:         menuInfo.Price,
		Status:         "SUCCESS",
		Gateway:        "razorpay",
		GatewayTxnID:   fmt.Sprintf("txn_%d", rand.Int()),
		IdempotencyKey: key,
	}

	// 5. atomic transaction
	err = s.db.Transaction(func(tx *gorm.DB) error {
		if err := s.repo.Create(tx, order); err != nil {
			return err
		}
		payment.OrderID = order.ID
		if err := s.payRepo.Create(tx, payment); err != nil {
			return err
		}
		e := &models.OutboxEvent{
			EventType: "ORDER_PLACED",
			Payload: models.JSONMap{
				"order_id":            order.ID,
				"user_id":             userID,
				"restaurant_id":       order.RestaurantID,
				"item_name":           order.ItemName,
				"item_price":          order.ItemPrice,
				"amount":              payment.Amount,
				"restaurant_owner_id": restInfo.OwnerID,
			},
		}
		return s.outboxRepo.Create(tx, e)
	})
	if err != nil {
		return nil, err
	}
	return order, nil
}

// ── restaurant-service HTTP helpers ─────────────────────────────

type internalRestaurant struct {
	ID         uint   `json:"id"`
	Name       string `json:"name"`
	IsApproved bool   `json:"is_approved"`
	IsOpen     bool   `json:"is_open"`
	OwnerID    uint   `json:"owner_id"`
}

type internalMenuItem struct {
	ID          uint    `json:"id"`
	Name        string  `json:"name"`
	Price       float64 `json:"price"`
	IsAvailable bool    `json:"is_available"`
}

func (s *orderService) fetchRestaurant(id uint) (*internalRestaurant, error) {
	url := fmt.Sprintf("%s/api/v1/internal/restaurants/%d", s.restaurantURL, id)
	body, err := s.circuitGet(url)
	if err != nil {
		return nil, err
	}
	var r internalRestaurant
	if err := json.Unmarshal(body, &r); err != nil {
		return nil, fmt.Errorf("bad restaurant response: %w", err)
	}
	return &r, nil
}

func (s *orderService) fetchMenuItem(restID, itemID uint) (*internalMenuItem, error) {
	url := fmt.Sprintf("%s/api/v1/internal/restaurants/%d/menu/%d", s.restaurantURL, restID, itemID)
	body, err := s.circuitGet(url)
	if err != nil {
		return nil, err
	}
	var m internalMenuItem
	if err := json.Unmarshal(body, &m); err != nil {
		return nil, fmt.Errorf("bad menu item response: %w", err)
	}
	return &m, nil
}

func (s *orderService) circuitGet(url string) ([]byte, error) {
	result, err := s.cb.Execute(func() (interface{}, error) {
		resp, err := s.httpClient.Get(url)
		if err != nil {
			return nil, err
		}
		defer resp.Body.Close()
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, err
		}
		if resp.StatusCode >= 400 {
			return nil, fmt.Errorf("upstream %d: %s", resp.StatusCode, string(body))
		}
		return body, nil
	})
	if err != nil {
		if errors.Is(err, gobreaker.ErrOpenState) || errors.Is(err, gobreaker.ErrTooManyRequests) {
			return nil, ErrCircuitOpen
		}
		return nil, err
	}
	return result.([]byte), nil
}

func (s *orderService) GetOrder(id uint) (*models.Order, error) {
	return s.repo.GetByID(id)
}

func (s *orderService) ListByUser(userID uint) ([]models.Order, error) {
	return s.repo.ListByUser(userID)
}

func (s *orderService) ListByRestaurant(restID uint) ([]models.Order, error) {
	return s.repo.ListByRestaurant(restID)
}

func (s *orderService) CancelOrder(userID uint, orderID uint) error {
	order, err := s.repo.GetByID(orderID)
	if err != nil {
		return err
	}
	if order == nil {
		return ErrNotFound
	}
	if order.UserID != userID {
		return ErrForbidden
	}
	if order.Status != "PLACED" {
		return errors.New("cannot_cancel")
	}
	return s.db.Transaction(func(tx *gorm.DB) error {
		order.Status = "CANCELLED"
		if err := s.repo.Update(tx, order); err != nil {
			return err
		}
		pay, err := s.payRepo.GetByOrder(order.ID)
		if err != nil || pay == nil {
			return err
		}
		pay.Status = "REFUNDED"
		if err := s.payRepo.Update(tx, pay); err != nil {
			return err
		}
		e := &models.OutboxEvent{
			EventType: "ORDER_CANCELLED",
			Payload: models.JSONMap{
				"order_id": order.ID,
				"user_id":  order.UserID,
			},
		}
		return s.outboxRepo.Create(tx, e)
	})
}

// validOwnerTransitions defines which statuses a restaurant owner can set,
// and from which preceding status.
var validOwnerTransitions = map[string]string{
	"CONFIRMED": "PLACED",
	"PREPARING": "CONFIRMED",
	"PREPARED":  "PREPARING",
}

// validInternalTransitions defines which statuses the internal/delivery system can set.
var validInternalTransitions = map[string]string{
	"CONFIRMED":        "PLACED",
	"PREPARING":        "CONFIRMED",
	"PREPARED":         "PREPARING",
	"OUT_FOR_DELIVERY": "PREPARED",
	"DELIVERED":        "OUT_FOR_DELIVERY",
	"FAILED":           "", // allowed from any status
}

// UpdateStatusByOwner lets the restaurant owner move an order through
// PLACED → CONFIRMED → PREPARING → PREPARED.
func (s *orderService) UpdateStatusByOwner(ownerUserID uint, orderID uint, status string) error {
	requiredPrev, ok := validOwnerTransitions[status]
	if !ok {
		return ErrInvalidStatus
	}

	order, err := s.repo.GetByID(orderID)
	if err != nil {
		return err
	}
	if order == nil {
		return ErrNotFound
	}

	// verify the caller owns the restaurant
	rest, err := s.fetchRestaurant(order.RestaurantID)
	if err != nil {
		return fmt.Errorf("cannot verify ownership: %w", err)
	}
	if rest.OwnerID != ownerUserID {
		return ErrForbidden
	}

	if order.Status != requiredPrev {
		return ErrInvalidStatus
	}

	order.Status = status
	return s.db.Transaction(func(tx *gorm.DB) error {
		return s.repo.Update(tx, order)
	})
}

// UpdateStatus is the internal/system endpoint for delivery-side transitions:
// PREPARED → OUT_FOR_DELIVERY → DELIVERED, or any → FAILED.
func (s *orderService) UpdateStatus(orderID uint, status string) error {
	requiredPrev, ok := validInternalTransitions[status]
	if !ok {
		return ErrInvalidStatus
	}

	order, err := s.repo.GetByID(orderID)
	if err != nil {
		return err
	}
	if order == nil {
		return ErrNotFound
	}

	if requiredPrev != "" && order.Status != requiredPrev {
		return ErrInvalidStatus
	}

	order.Status = status
	return s.db.Transaction(func(tx *gorm.DB) error {
		return s.repo.Update(tx, order)
	})
}

func (s *orderService) GetStats() (*models.OrderStats, error) {
	return s.repo.GetStats()
}
