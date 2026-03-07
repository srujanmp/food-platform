package events

import (
	"fmt"

	"github.com/rabbitmq/amqp091-go"
)

// Publisher publishes restaurant events to RabbitMQ.
type Publisher struct {
	ch *amqp091.Channel
}

// NewPublisher declares the restaurant_events exchange and returns a Publisher.
func NewPublisher(ch *amqp091.Channel) (*Publisher, error) {
	if err := ch.ExchangeDeclare(
		"restaurant_events",
		"topic",
		true,
		false,
		false,
		false,
		nil,
	); err != nil {
		return nil, fmt.Errorf("failed to declare exchange: %w", err)
	}
	return &Publisher{ch: ch}, nil
}

// Publish sends a message to the restaurant_events exchange with the given routing key.
func (p *Publisher) Publish(routingKey string, body []byte) error {
	return p.ch.Publish("restaurant_events", routingKey, false, false, amqp091.Publishing{
		ContentType: "application/json",
		Body:        body,
	})
}
