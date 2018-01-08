package rabbitroutine

import (
	"context"
	"time"

	"github.com/pkg/errors"
	"github.com/streadway/amqp"
)

// Publisher interface provides functionality of publishing to RabbitMQ.
type Publisher interface {
	// Publish used to send msg to RabbitMQ exchange.
	Publish(ctx context.Context, exchange, key string, msg amqp.Publishing) error
}

// EnsurePublisher implement Publisher interface and used to publish messages to RabbitMQ exchange.
// It will block until is either message is successfully delivered, context has cancelled or error received.
type EnsurePublisher struct {
	pool *Pool
}

// NewEnsurePublisher return new instance of EnsurePublisher.
func NewEnsurePublisher(p *Pool) *EnsurePublisher {
	return &EnsurePublisher{p}
}

// Publish sends msg to an exchange on the RabbitMQ
// and wait to ensure that msg have successfully been received by the server.
// It will block until is either message is successfully delivered, context has cancelled or error received.
func (p *EnsurePublisher) Publish(ctx context.Context, exchange, key string, msg amqp.Publishing) error {
	k, err := p.pool.ChannelWithConfirm(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to receive channel for publishing")
	}

	ch := k.Channel()

	err = ch.Publish(exchange, key, false, false, msg)
	if err != nil {
		k.Close()

		return errors.Wrap(err, "failed to publish message")
	}

	select {
	case <-ctx.Done():
		// Do not release amqp Channel, because old confirmation will be waited.
		k.Close()

		return ctx.Err()
	case amqpErr := <-k.Error():
		k.Close()

		return errors.Wrap(amqpErr, "failed to deliver a message")
	case <-k.Confirm():
		p.pool.Release(k)

		return nil
	}
}

// RetryPublisher implement Publisher interface and used to publish messages to RabbitMQ exchange.
// It will block until is either message is successfully delivered or context has cancelled.
// On error publisher will retry to publish msg.
type RetryPublisher struct {
	*EnsurePublisher
	// delay define how long to wait before retry
	delay time.Duration
}

// NewRetryPublisher return new instance of RetryPublisher.
func NewRetryPublisher(p *EnsurePublisher) *RetryPublisher {
	return &RetryPublisher{p, 10 * time.Millisecond}
}

// NewRetryPublisherWithDelay return new instance of RetryPublisher with defined delay between retries.
func NewRetryPublisherWithDelay(p *EnsurePublisher, delay time.Duration) *RetryPublisher {
	return &RetryPublisher{p, delay}
}

// Publish is used to send msg to RabbitMQ exchange.
// It will block until is either message is delivered or context has cancelled.
// Error returned only if context was done.
func (p *RetryPublisher) Publish(ctx context.Context, exchange, key string, msg amqp.Publishing) error {
	for {
		err := p.EnsurePublisher.Publish(ctx, exchange, key, msg)
		if err != nil {
			select {
			case <-time.After(p.delay):
				continue
			case <-ctx.Done():
				return ctx.Err()
			}
		}

		return nil
	}
}
