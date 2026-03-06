package events

import (
	"context"
	"encoding/json"
	"fmt"
	"log"

	"github.com/food-platform/user-service/internal/models"
	"github.com/food-platform/user-service/internal/service"
	"github.com/rabbitmq/amqp091-go"
)

// Consumer handles consuming events from RabbitMQ.
type Consumer interface {
	Start(ctx context.Context) error
	Stop() error
}

type rabbitmqConsumer struct {
	ch             *amqp091.Channel
	conn           *amqp091.Connection
	profileService service.ProfileService
}

// NewConsumer creates a new RabbitMQ event consumer.
func NewConsumer(
	conn *amqp091.Connection,
	ch *amqp091.Channel,
	profileService service.ProfileService,
) Consumer {
	return &rabbitmqConsumer{
		ch:             ch,
		conn:           conn,
		profileService: profileService,
	}
}

// Start begins consuming events from RabbitMQ.
func (c *rabbitmqConsumer) Start(ctx context.Context) error {
	// Declare the exchange
	if err := c.ch.ExchangeDeclare(
		"user_events", // name
		"topic",       // kind
		true,          // durable
		false,         // auto-delete
		false,         // internal
		false,         // no-wait
		nil,           // args
	); err != nil {
		return fmt.Errorf("failed to declare exchange: %w", err)
	}

	// Declare dead-letter exchange
	if err := c.ch.ExchangeDeclare(
		"user_events.dlx", // name
		"topic",           // kind
		true,              // durable
		false,             // auto-delete
		false,             // internal
		false,             // no-wait
		nil,               // args
	); err != nil {
		return fmt.Errorf("failed to declare DLX exchange: %w", err)
	}

	// Declare the queue for USER_CREATED events with dead-letter exchange
	createdArgs := amqp091.Table{
		"x-dead-letter-exchange":    "user_events.dlx",
		"x-dead-letter-routing-key": "user.created.dlq",
	}
	createdQueue, err := c.ch.QueueDeclare(
		"user.created.queue", // name
		true,                 // durable
		false,                // delete when unused
		false,                // exclusive
		false,                // no-wait
		createdArgs,          // args
	)
	if err != nil {
		return fmt.Errorf("failed to declare created queue: %w", err)
	}

	// Bind USER_CREATED queue
	if err := c.ch.QueueBind(
		createdQueue.Name, // queue name
		"user.created",    // routing key
		"user_events",     // exchange name
		false,             // no-wait
		nil,               // args
	); err != nil {
		return fmt.Errorf("failed to bind created queue: %w", err)
	}

	// Declare the queue for USER_DELETED events with dead-letter exchange
	deletedArgs := amqp091.Table{
		"x-dead-letter-exchange":    "user_events.dlx",
		"x-dead-letter-routing-key": "user.deleted.dlq",
	}
	deletedQueue, err := c.ch.QueueDeclare(
		"user.deleted.queue", // name
		true,                 // durable
		false,                // delete when unused
		false,                // exclusive
		false,                // no-wait
		deletedArgs,          // args
	)
	if err != nil {
		return fmt.Errorf("failed to declare deleted queue: %w", err)
	}

	// Bind USER_DELETED queue
	if err := c.ch.QueueBind(
		deletedQueue.Name, // queue name
		"user.deleted",    // routing key
		"user_events",     // exchange name
		false,             // no-wait
		nil,               // args
	); err != nil {
		return fmt.Errorf("failed to bind deleted queue: %w", err)
	}

	// Start consuming USER_CREATED messages (manual ack)
	createdMsgs, err := c.ch.Consume(
		createdQueue.Name, // queue
		"",                // consumer
		false,             // auto-ack = false (manual ack)
		false,             // exclusive
		false,             // no-local
		false,             // no-wait
		nil,               // args
	)
	if err != nil {
		return fmt.Errorf("failed to consume from created queue: %w", err)
	}

	// Start consuming USER_DELETED messages (manual ack)
	deletedMsgs, err := c.ch.Consume(
		deletedQueue.Name, // queue
		"",                // consumer
		false,             // auto-ack = false (manual ack)
		false,             // exclusive
		false,             // no-local
		false,             // no-wait
		nil,               // args
	)
	if err != nil {
		return fmt.Errorf("failed to consume from deleted queue: %w", err)
	}

	// Start goroutines to handle messages
	go c.handleCreatedMessages(ctx, createdMsgs)
	go c.handleDeletedMessages(ctx, deletedMsgs)

	log.Println("Event consumer started")
	return nil
}

// handleCreatedMessages processes USER_CREATED events.
func (c *rabbitmqConsumer) handleCreatedMessages(ctx context.Context, msgs <-chan amqp091.Delivery) {
	for {
		select {
		case <-ctx.Done():
			return
		case msg := <-msgs:
			if msg.Body == nil {
				continue
			}

			var event models.UserCreatedEvent
			if err := json.Unmarshal(msg.Body, &event); err != nil {
				log.Printf("Failed to unmarshal USER_CREATED event: %v", err)
				_ = msg.Nack(false, false) // send to DLQ
				continue
			}

			// Create profile for the new user (pass name from event)
			_, err := c.profileService.EnsureProfile(event.UserID, event.Name)
			if err != nil {
				log.Printf("Failed to create profile for user %d: %v", event.UserID, err)
				_ = msg.Nack(false, false) // send to DLQ
				continue
			}

			_ = msg.Ack(false)
			log.Printf("Profile created for user %d (email: %s)", event.UserID, event.Email)
		}
	}
}

// handleDeletedMessages processes USER_DELETED events.
func (c *rabbitmqConsumer) handleDeletedMessages(ctx context.Context, msgs <-chan amqp091.Delivery) {
	for {
		select {
		case <-ctx.Done():
			return
		case msg := <-msgs:
			if msg.Body == nil {
				continue
			}

			var event models.UserDeletedEvent
			if err := json.Unmarshal(msg.Body, &event); err != nil {
				log.Printf("Failed to unmarshal USER_DELETED event: %v", err)
				_ = msg.Nack(false, false) // send to DLQ
				continue
			}

			// Delete profile for the user
			err := c.profileService.DeleteProfile(event.UserID, event.UserID, "SYSTEM")
			if err != nil {
				log.Printf("Failed to delete profile for user %d: %v", event.UserID, err)
				_ = msg.Nack(false, false) // send to DLQ
				continue
			}

			_ = msg.Ack(false)
			log.Printf("Profile deleted for user %d (email: %s)", event.UserID, event.Email)
		}
	}
}

// Stop stops the consumer.
func (c *rabbitmqConsumer) Stop() error {
	if err := c.ch.Close(); err != nil {
		return fmt.Errorf("failed to close channel: %w", err)
	}
	if err := c.conn.Close(); err != nil {
		return fmt.Errorf("failed to close connection: %w", err)
	}
	return nil
}
