package service

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/food-platform/delivery-service/internal/models"
	"github.com/food-platform/delivery-service/internal/repository"
	"github.com/sony/gobreaker"
	"gorm.io/gorm"
)

var (
	ErrNotFound          = errors.New("not_found")
	ErrForbidden         = errors.New("forbidden")
	ErrInvalidTransition = errors.New("invalid_status_transition")
	ErrNoDriverAvailable = errors.New("no_driver_available")
	ErrCircuitOpen       = errors.New("service_unavailable")
)

// DeliveryService defines business operations used by handlers and consumers.
type DeliveryService interface {
	GetDriver(driverID uint) (*models.Driver, error)
	GetDriverByAuthID(authID uint) (*models.Driver, error)
	GetActiveOrderForDriver(driverID uint) (*models.Delivery, error)
	ListDriverOrders(driverID uint) ([]models.Delivery, error)
	UpdateLocation(authID uint, lat, lng float64) error
	UpdateDeliveryStatus(authID uint, orderID uint, status string) error
	GetTracking(orderID uint) (*models.TrackingResponse, error)
	AssignDriver(orderID uint, userID uint) error
	CreateDriver(authID uint, name, phone string) error
	EnqueueAssignment(orderID, userID uint) error
	RetryPendingAssignments() error
}

// deliveryService is the concrete implementation.
type deliveryService struct {
	driverRepo   repository.DriverRepository
	deliveryRepo repository.DeliveryRepository
	outboxRepo   repository.OutboxRepository
	pendingRepo  repository.PendingAssignmentRepository
	db           *gorm.DB
	httpClient   *http.Client
	cb           *gobreaker.CircuitBreaker
	orderSvcURL  string
}

// NewDeliveryService constructs a service with dependencies.
func NewDeliveryService(
	driverRepo repository.DriverRepository,
	deliveryRepo repository.DeliveryRepository,
	outboxRepo repository.OutboxRepository,
	pendingRepo repository.PendingAssignmentRepository,
	db *gorm.DB,
	orderSvcURL string,
) DeliveryService {
	cb := gobreaker.NewCircuitBreaker(gobreaker.Settings{
		Name:        "order-service-client",
		MaxRequests: 1,
		Interval:    60 * time.Second,
		Timeout:     30 * time.Second,
		ReadyToTrip: func(counts gobreaker.Counts) bool {
			return counts.ConsecutiveFailures >= 5
		},
	})
	return &deliveryService{
		driverRepo:   driverRepo,
		deliveryRepo: deliveryRepo,
		outboxRepo:   outboxRepo,
		pendingRepo:  pendingRepo,
		db:           db,
		httpClient:   &http.Client{Timeout: 3 * time.Second},
		cb:           cb,
		orderSvcURL:  orderSvcURL,
	}
}

func (s *deliveryService) GetDriver(driverID uint) (*models.Driver, error) {
	return s.driverRepo.GetByID(driverID)
}

func (s *deliveryService) GetDriverByAuthID(authID uint) (*models.Driver, error) {
	return s.driverRepo.GetByAuthID(authID)
}

func (s *deliveryService) GetActiveOrderForDriver(driverID uint) (*models.Delivery, error) {
	return s.deliveryRepo.GetActiveByDriver(driverID)
}

func (s *deliveryService) ListDriverOrders(driverID uint) ([]models.Delivery, error) {
	return s.deliveryRepo.ListByDriver(driverID)
}

func (s *deliveryService) UpdateLocation(authID uint, lat, lng float64) error {
	driver, err := s.driverRepo.GetByAuthID(authID)
	if err != nil {
		return err
	}
	if driver == nil {
		return ErrNotFound
	}
	driver.Latitude = lat
	driver.Longitude = lng
	return s.driverRepo.Update(s.db, driver)
}

// validTransitions defines which statuses a driver can set and from which preceding status.
var validTransitions = map[string]string{
	"OUT_FOR_DELIVERY": "ASSIGNED",
	"DELIVERED":        "OUT_FOR_DELIVERY",
	"FAILED":           "OUT_FOR_DELIVERY",
}

func (s *deliveryService) UpdateDeliveryStatus(authID uint, orderID uint, status string) error {
	requiredPrev, ok := validTransitions[status]
	if !ok {
		return ErrInvalidTransition
	}

	delivery, err := s.deliveryRepo.GetByOrderID(orderID)
	if err != nil {
		return err
	}
	if delivery == nil {
		return ErrNotFound
	}

	// Verify the driver owns this delivery
	driver, err := s.driverRepo.GetByAuthID(authID)
	if err != nil {
		return err
	}
	if driver == nil || driver.ID != delivery.DriverID {
		return ErrForbidden
	}

	if delivery.Status != requiredPrev {
		return ErrInvalidTransition
	}

	delivery.Status = status
	if status == "DELIVERED" {
		now := time.Now()
		delivery.DeliveredAt = &now
	}

	return s.db.Transaction(func(tx *gorm.DB) error {
		if err := s.deliveryRepo.Update(tx, delivery); err != nil {
			return err
		}

		// If delivered or failed, make driver available again
		if status == "DELIVERED" || status == "FAILED" {
			driver.IsAvailable = true
			if err := s.driverRepo.Update(tx, driver); err != nil {
				return err
			}
		}

		// Call order-service internal API to update order status
		orderStatus := status
		if err := s.updateOrderStatus(orderID, orderStatus); err != nil {
			return fmt.Errorf("failed to update order-service: %w", err)
		}

		// Write outbox event
		eventType := "ORDER_DELIVERED"
		if status == "FAILED" {
			eventType = "ORDER_FAILED"
		}
		if status == "OUT_FOR_DELIVERY" {
			// No outbox event needed for OUT_FOR_DELIVERY — already handled by order-service call
			return nil
		}

		e := &models.OutboxEvent{
			EventType: eventType,
			Payload: models.JSONMap{
				"order_id":  orderID,
				"driver_id": driver.ID,
				"user_id":   delivery.UserID,
				"status":    status,
			},
		}
		return s.outboxRepo.Create(tx, e)
	})
}

func (s *deliveryService) GetTracking(orderID uint) (*models.TrackingResponse, error) {
	delivery, err := s.deliveryRepo.GetByOrderID(orderID)
	if err != nil {
		return nil, err
	}
	if delivery == nil {
		return nil, ErrNotFound
	}

	driver, err := s.driverRepo.GetByID(delivery.DriverID)
	if err != nil {
		return nil, err
	}
	if driver == nil {
		return nil, ErrNotFound
	}

	return &models.TrackingResponse{
		OrderID:   orderID,
		Status:    delivery.Status,
		Latitude:  driver.Latitude,
		Longitude: driver.Longitude,
	}, nil
}

// AssignDriver is called by the ORDER_PREPARED consumer.
func (s *deliveryService) AssignDriver(orderID uint, userID uint) error {
	// Idempotency: if a delivery already exists for this order, skip.
	existing, err := s.deliveryRepo.GetByOrderID(orderID)
	if err != nil {
		return err
	}
	if existing != nil {
		// Clean up pending assignment if it exists.
		s.pendingRepo.Delete(orderID) //nolint:errcheck
		return nil
	}

	err = s.db.Transaction(func(tx *gorm.DB) error {
		driver, err := s.driverRepo.FindAvailable()
		if err != nil {
			return err
		}
		if driver == nil {
			return ErrNoDriverAvailable
		}

		delivery := &models.Delivery{
			OrderID:    orderID,
			DriverID:   driver.ID,
			UserID:     userID,
			Status:     "ASSIGNED",
			AssignedAt: time.Now(),
		}
		if err := s.deliveryRepo.Create(tx, delivery); err != nil {
			return err
		}

		driver.IsAvailable = false
		if err := s.driverRepo.Update(tx, driver); err != nil {
			return err
		}

		e := &models.OutboxEvent{
			EventType: "DRIVER_ASSIGNED",
			Payload: models.JSONMap{
				"order_id":    orderID,
				"driver_id":   driver.ID,
				"driver_name": driver.Name,
				"user_id":     userID,
			},
		}
		return s.outboxRepo.Create(tx, e)
	})
	if err == nil {
		s.pendingRepo.Delete(orderID) //nolint:errcheck
	}
	return err
}

// EnqueueAssignment persists an ORDER_PREPARED event so the poller can retry.
func (s *deliveryService) EnqueueAssignment(orderID, userID uint) error {
	return s.pendingRepo.Upsert(orderID, userID)
}

// RetryPendingAssignments is called periodically to assign drivers to pending orders.
func (s *deliveryService) RetryPendingAssignments() error {
	pending, err := s.pendingRepo.ListPending(20)
	if err != nil {
		return err
	}
	for _, pa := range pending {
		if err := s.AssignDriver(pa.OrderID, pa.UserID); err != nil {
			continue // will retry next tick
		}
	}
	return nil
}

// CreateDriver is called by the DRIVER_REGISTERED consumer.
func (s *deliveryService) CreateDriver(authID uint, name, phone string) error {
	existing, err := s.driverRepo.GetByAuthID(authID)
	if err != nil {
		return err
	}
	if existing != nil {
		return nil // idempotent
	}
	d := &models.Driver{
		AuthID:      authID,
		Name:        name,
		Phone:       phone,
		IsAvailable: true,
	}
	return s.driverRepo.Create(s.db, d)
}

// updateOrderStatus calls order-service internal API.
func (s *deliveryService) updateOrderStatus(orderID uint, status string) error {
	url := fmt.Sprintf("%s/api/v1/internal/orders/%d/status", s.orderSvcURL, orderID)
	body, err := json.Marshal(map[string]string{"status": status})
	if err != nil {
		return err
	}

	result, err := s.cb.Execute(func() (interface{}, error) {
		req, err := http.NewRequest(http.MethodPatch, url, bytes.NewReader(body))
		if err != nil {
			return nil, err
		}
		req.Header.Set("Content-Type", "application/json")
		resp, err := s.httpClient.Do(req)
		if err != nil {
			return nil, err
		}
		defer resp.Body.Close()
		respBody, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, err
		}
		if resp.StatusCode >= 400 {
			return nil, fmt.Errorf("order-service %d: %s", resp.StatusCode, string(respBody))
		}
		return respBody, nil
	})
	if err != nil {
		if errors.Is(err, gobreaker.ErrOpenState) || errors.Is(err, gobreaker.ErrTooManyRequests) {
			return ErrCircuitOpen
		}
		return err
	}
	_ = result
	return nil
}
