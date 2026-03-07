package events

import (
	"encoding/json"
	"fmt"

	"github.com/food-platform/delivery-service/internal/service"
	"github.com/rabbitmq/amqp091-go"
	"go.uber.org/zap"
)

// Consumer listens to RabbitMQ queues for order and auth events.
type Consumer struct {
	ch     *amqp091.Channel
	svc    service.DeliveryService
	logger *zap.Logger
}

// NewConsumer sets up exchanges, queues, and bindings.
func NewConsumer(ch *amqp091.Channel, svc service.DeliveryService, logger *zap.Logger) (*Consumer, error) {
	// Declare exchanges we subscribe to (idempotent — safe if already exists)
	for _, exchange := range []string{"restaurant_events", "user_events"} {
		if err := ch.ExchangeDeclare(exchange, "topic", true, false, false, false, nil); err != nil {
			return nil, fmt.Errorf("failed to declare exchange %s: %w", exchange, err)
		}
	}

	// Queue: delivery.order_prepared — bound to restaurant_events / ORDER_PREPARED
	if _, err := ch.QueueDeclare("delivery.order_prepared", true, false, false, false, nil); err != nil {
		return nil, fmt.Errorf("failed to declare queue delivery.order_prepared: %w", err)
	}
	if err := ch.QueueBind("delivery.order_prepared", "ORDER_PREPARED", "restaurant_events", false, nil); err != nil {
		return nil, fmt.Errorf("failed to bind delivery.order_prepared: %w", err)
	}

	// Queue: delivery.user_created — bound to user_events / user.created
	// (auth-service publishes USER_CREATED for all roles; we filter DRIVER in handler)
	if _, err := ch.QueueDeclare("delivery.user_created", true, false, false, false, nil); err != nil {
		return nil, fmt.Errorf("failed to declare queue delivery.user_created: %w", err)
	}
	if err := ch.QueueBind("delivery.user_created", "user.created", "user_events", false, nil); err != nil {
		return nil, fmt.Errorf("failed to bind delivery.user_created: %w", err)
	}

	return &Consumer{ch: ch, svc: svc, logger: logger}, nil
}

// Start begins consuming messages from both queues (blocking goroutines).
func (c *Consumer) Start() {
	go c.consumeOrderPrepared()
	go c.consumeUserCreated()
}

func (c *Consumer) consumeOrderPrepared() {
	msgs, err := c.ch.Consume("delivery.order_prepared", "", false, false, false, false, nil)
	if err != nil {
		c.logger.Fatal("failed to consume delivery.order_prepared", zap.Error(err))
	}
	c.logger.Info("consumer started: delivery.order_prepared")

	for msg := range msgs {
		c.handleOrderPrepared(msg)
	}
}

func (c *Consumer) handleOrderPrepared(msg amqp091.Delivery) {
	var payload struct {
		OrderID uint `json:"order_id"`
		UserID  uint `json:"user_id"`
	}
	if err := json.Unmarshal(msg.Body, &payload); err != nil {
		c.logger.Error("ORDER_PREPARED: bad payload", zap.Error(err))
		msg.Nack(false, false) //nolint:errcheck
		return
	}

	c.logger.Info("ORDER_PREPARED received", zap.Uint("order_id", payload.OrderID))

	// Persist to pending_assignments so a poller can retry if assignment fails now.
	if err := c.svc.EnqueueAssignment(payload.OrderID, payload.UserID); err != nil {
		c.logger.Error("ORDER_PREPARED: failed to persist pending assignment", zap.Error(err), zap.Uint("order_id", payload.OrderID))
		msg.Nack(false, true) //nolint:errcheck
		return
	}

	// Try immediate assignment; failures are fine — the poller will retry.
	if err := c.svc.AssignDriver(payload.OrderID, payload.UserID); err != nil {
		c.logger.Warn("ORDER_PREPARED: assign driver deferred", zap.Error(err), zap.Uint("order_id", payload.OrderID))
	} else {
		c.logger.Info("ORDER_PREPARED handled", zap.Uint("order_id", payload.OrderID))
	}

	msg.Ack(false) //nolint:errcheck
}

func (c *Consumer) consumeUserCreated() {
	msgs, err := c.ch.Consume("delivery.user_created", "", false, false, false, false, nil)
	if err != nil {
		c.logger.Fatal("failed to consume delivery.user_created", zap.Error(err))
	}
	c.logger.Info("consumer started: delivery.user_created")

	for msg := range msgs {
		c.handleUserCreated(msg)
	}
}

func (c *Consumer) handleUserCreated(msg amqp091.Delivery) {
	// AUTH-service publishes USER_CREATED for every role.
	// We only care about DRIVER registrations.
	var payload struct {
		Event  string `json:"event"`
		UserID uint   `json:"user_id"`
		Name   string `json:"name"`
		Phone  string `json:"phone"`
		Role   string `json:"role"`
	}
	if err := json.Unmarshal(msg.Body, &payload); err != nil {
		c.logger.Error("USER_CREATED: bad payload", zap.Error(err))
		msg.Nack(false, false) //nolint:errcheck
		return
	}

	// Skip non-DRIVER registrations
	if payload.Role != "DRIVER" {
		msg.Ack(false) //nolint:errcheck
		return
	}

	c.logger.Info("DRIVER registration received", zap.Uint("auth_id", payload.UserID), zap.String("name", payload.Name))

	if err := c.svc.CreateDriver(payload.UserID, payload.Name, payload.Phone); err != nil {
		c.logger.Error("DRIVER creation failed", zap.Error(err))
		msg.Nack(false, true) //nolint:errcheck
		return
	}

	msg.Ack(false) //nolint:errcheck
	c.logger.Info("DRIVER created", zap.Uint("auth_id", payload.UserID))
}
