package rabbitroutine

import (
	"github.com/streadway/amqp"
)

type Consumer interface {
	// Declare used to declare any rabbitmq entity.
	// Will be called once before Consume.
	Declare(ch *amqp.Channel) error
	// Consume used to consuming rabbitmq queue.
	// Can be called 1+ times (one goroutine per call).
	Consume(ch *amqp.Channel) error
}
