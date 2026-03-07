package repository

import (
	"errors"

	"github.com/food-platform/delivery-service/internal/models"
	"gorm.io/gorm"
)

// DriverRepository handles CRUD for drivers.
type DriverRepository interface {
	Create(tx *gorm.DB, d *models.Driver) error
	GetByID(id uint) (*models.Driver, error)
	GetByAuthID(authID uint) (*models.Driver, error)
	Update(tx *gorm.DB, d *models.Driver) error
	FindAvailable() (*models.Driver, error)
}

// DeliveryRepository handles CRUD for deliveries.
type DeliveryRepository interface {
	Create(tx *gorm.DB, d *models.Delivery) error
	GetByOrderID(orderID uint) (*models.Delivery, error)
	GetActiveByDriver(driverID uint) (*models.Delivery, error)
	ListByDriver(driverID uint) ([]models.Delivery, error)
	Update(tx *gorm.DB, d *models.Delivery) error
}

// OutboxRepository for writing and scanning outbox events.
type OutboxRepository interface {
	Create(tx *gorm.DB, e *models.OutboxEvent) error
	ListUnpublished(limit int) ([]models.OutboxEvent, error)
	MarkPublished(id uint) error
}

// PendingAssignmentRepository for tracking unassigned ORDER_PREPARED events.
type PendingAssignmentRepository interface {
	Upsert(orderID, userID uint) error
	ListPending(limit int) ([]models.PendingAssignment, error)
	Delete(orderID uint) error
}

// ── GORM implementations ────────────────────────────────────────────────────

// driverRepo implements DriverRepository.
type driverRepo struct{ db *gorm.DB }

func NewDriverRepository(db *gorm.DB) DriverRepository {
	return &driverRepo{db: db}
}

func (r *driverRepo) Create(tx *gorm.DB, d *models.Driver) error {
	return tx.Create(d).Error
}

func (r *driverRepo) GetByID(id uint) (*models.Driver, error) {
	var d models.Driver
	err := r.db.First(&d, id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	return &d, err
}

func (r *driverRepo) GetByAuthID(authID uint) (*models.Driver, error) {
	var d models.Driver
	err := r.db.Where("auth_id = ?", authID).First(&d).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	return &d, err
}

func (r *driverRepo) Update(tx *gorm.DB, d *models.Driver) error {
	return tx.Save(d).Error
}

func (r *driverRepo) FindAvailable() (*models.Driver, error) {
	var d models.Driver
	err := r.db.Where("is_available = ?", true).Order("id").First(&d).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	return &d, err
}

// deliveryRepo implements DeliveryRepository.
type deliveryRepo struct{ db *gorm.DB }

func NewDeliveryRepository(db *gorm.DB) DeliveryRepository {
	return &deliveryRepo{db: db}
}

func (r *deliveryRepo) Create(tx *gorm.DB, d *models.Delivery) error {
	return tx.Create(d).Error
}

func (r *deliveryRepo) GetByOrderID(orderID uint) (*models.Delivery, error) {
	var d models.Delivery
	err := r.db.Preload("Driver").Where("order_id = ?", orderID).First(&d).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	return &d, err
}

func (r *deliveryRepo) GetActiveByDriver(driverID uint) (*models.Delivery, error) {
	var d models.Delivery
	err := r.db.Where("driver_id = ? AND status IN ?", driverID, []string{"ASSIGNED", "OUT_FOR_DELIVERY"}).First(&d).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	return &d, err
}

func (r *deliveryRepo) ListByDriver(driverID uint) ([]models.Delivery, error) {
	var list []models.Delivery
	err := r.db.Where("driver_id = ?", driverID).Order("created_at DESC").Find(&list).Error
	return list, err
}

func (r *deliveryRepo) Update(tx *gorm.DB, d *models.Delivery) error {
	return tx.Save(d).Error
}

// outboxRepo implements OutboxRepository.
type outboxRepo struct{ db *gorm.DB }

func NewOutboxRepository(db *gorm.DB) OutboxRepository {
	return &outboxRepo{db: db}
}

func (r *outboxRepo) Create(tx *gorm.DB, e *models.OutboxEvent) error {
	return tx.Create(e).Error
}

func (r *outboxRepo) ListUnpublished(limit int) ([]models.OutboxEvent, error) {
	var events []models.OutboxEvent
	err := r.db.Where("published = ?", false).Order("id").Limit(limit).Find(&events).Error
	return events, err
}

func (r *outboxRepo) MarkPublished(id uint) error {
	return r.db.Model(&models.OutboxEvent{}).Where("id = ?", id).Update("published", true).Error
}

// pendingAssignmentRepo implements PendingAssignmentRepository.
type pendingAssignmentRepo struct{ db *gorm.DB }

func NewPendingAssignmentRepository(db *gorm.DB) PendingAssignmentRepository {
	return &pendingAssignmentRepo{db: db}
}

func (r *pendingAssignmentRepo) Upsert(orderID, userID uint) error {
	pa := models.PendingAssignment{OrderID: orderID, UserID: userID}
	result := r.db.Where("order_id = ?", orderID).FirstOrCreate(&pa)
	return result.Error
}

func (r *pendingAssignmentRepo) ListPending(limit int) ([]models.PendingAssignment, error) {
	var list []models.PendingAssignment
	err := r.db.Order("id").Limit(limit).Find(&list).Error
	return list, err
}

func (r *pendingAssignmentRepo) Delete(orderID uint) error {
	return r.db.Where("order_id = ?", orderID).Delete(&models.PendingAssignment{}).Error
}
