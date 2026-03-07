package events

import (
	"encoding/json"
	"fmt"

	"github.com/food-platform/delivery-service/internal/models"
	"github.com/rabbitmq/amqp091-go"
)

// Publisher publishes delivery-related events.
type Publisher interface {
	Publish(ev *models.OutboxEvent) error
	Close() error
}

type rabbitmqPublisher struct {
	ch *amqp091.Channel
}

// NewPublisher declares the delivery_events exchange and returns a Publisher.
func NewPublisher(ch *amqp091.Channel) (Publisher, error) {
	if err := ch.ExchangeDeclare(
		"delivery_events",
		"topic",
		true,
		false,
		false,
		false,
		nil,
	); err != nil {
		return nil, fmt.Errorf("failed to declare exchange: %w", err)
	}
	return &rabbitmqPublisher{ch: ch}, nil
}

func (p *rabbitmqPublisher) Publish(ev *models.OutboxEvent) error {
	body, err := json.Marshal(ev.Payload)
	if err != nil {
		return err
	}
	return p.ch.Publish("delivery_events", ev.EventType, false, false, amqp091.Publishing{
		ContentType: "application/json",
		Body:        body,
	})
}

func (p *rabbitmqPublisher) Close() error {
	return p.ch.Close()
}
