package darkmq

import (
	"context"

	"github.com/streadway/amqp"
)

// Consumer interface provides functionality of rabbit entity Declaring and queue consuming.
type Consumer interface {
	// Declare used to declare required RabbitMQ entities.
	// Will be called once before Consume (even when StartMultipleConsumers called).
	// On any problems with connection or channel RabbitMQ entities will be redeclared.
	Declare(ctx context.Context, ch *amqp.Channel) error
	// Consume used to consuming RabbitMQ queue.
	// Can be called 1+ times if you register it with StartMultipleConsumers.
	Consume(ctx context.Context, ch *amqp.Channel) error
}
