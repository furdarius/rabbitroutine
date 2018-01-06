package rabbitroutine

import (
	"context"

	"github.com/pkg/errors"
	"github.com/streadway/amqp"
)

// Publisher used to publish messages to RabbitMQ exchange.
type Publisher struct {
	pool *Pool
}

// NewPublisher return new instance of Publisher.
func NewPublisher(p *Pool) *Publisher {
	return &Publisher{p}
}

// EnsurePublish sends msg to an exchange on the RabbitMQ
// and wait to ensure that have successfully been received by the server.
func (p *Publisher) EnsurePublish(ctx context.Context, exchange, key string, msg amqp.Publishing) error {
	k, err := p.pool.ChannelWithConfirm(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to receive channel for publishing")
	}

	ch := k.Channel()

	err = ch.Publish(exchange, key, false, false, msg)
	if err != nil {
		return errors.Wrap(err, "failed to publish message")
	}

	select {
	case <-ctx.Done():
		return ctx.Err()
	case amqpErr := <-k.Error():
		return errors.Wrap(amqpErr, "failed to deliver a message")
	case <-k.Confirm():
		p.pool.Release(k)
		
		return nil
	}
}
