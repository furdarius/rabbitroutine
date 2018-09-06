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
// When EnsurePublisher used, publishing confirmation is enabled, so we have delivery guarantees.
// @see http://www.rabbitmq.com/blog/2011/02/10/introducing-publisher-confirms/
type EnsurePublisher struct {
	pool *Pool
}

// NewEnsurePublisher return a new instance of EnsurePublisher.
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
		_ = k.Close() //nolint: gosec

		return errors.Wrap(err, "failed to publish message")
	}

	select {
	case <-ctx.Done():
		// Do not return to pool, because old confirmation will be waited.
		_ = k.Close() //nolint: gosec

		return ctx.Err()
	case amqpErr := <-k.Error():
		_ = k.Close() //nolint: gosec

		return errors.Wrap(amqpErr, "failed to deliver a message")
	case <-k.Confirm():
		p.pool.Release(k)

		return nil
	}
}

// FireForgetPublisher implement Publisher interface and used to publish messages to RabbitMQ exchange without delivery guarantees.
// When FireForgetPublisher used, publishing confirmation is not enabled, so we haven't delivery guarantees.
// @see http://www.rabbitmq.com/blog/2011/02/10/introducing-publisher-confirms/
type FireForgetPublisher struct {
	pool *LightningPool
}

// NewFireForgetPublisher return a new instance of FireForgetPublisher.
func NewFireForgetPublisher(p *LightningPool) *FireForgetPublisher {
	return &FireForgetPublisher{p}
}

// Publish sends msg to an exchange on the RabbitMQ.
func (p *FireForgetPublisher) Publish(ctx context.Context, exchange, key string, msg amqp.Publishing) error {
	ch, err := p.pool.Channel(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to receive channel for publishing")
	}

	err = ch.Publish(exchange, key, false, false, msg)
	if err != nil {
		_ = ch.Close() //nolint: gosec

		return errors.Wrap(err, "failed to publish message")
	}

	p.pool.Release(ch)

	return nil
}

// RetryPublisher implement Publisher interface and used to publish messages to RabbitMQ exchange.
// It will block until is either message is successfully delivered or context has cancelled.
// On error publisher will retry to publish msg.
type RetryPublisher struct {
	Publisher
	// delay define how long to wait before retry
	delay time.Duration
}

// NewRetryPublisher return a new instance of RetryPublisher.
func NewRetryPublisher(p Publisher) *RetryPublisher {
	return &RetryPublisher{p, 10 * time.Millisecond}
}

// NewRetryPublisherWithDelay return a new instance of RetryPublisher with defined delay between retries.
func NewRetryPublisherWithDelay(p Publisher, delay time.Duration) *RetryPublisher {
	return &RetryPublisher{p, delay}
}

// Publish is used to send msg to RabbitMQ exchange.
// It will block until is either message is delivered or context has cancelled.
// Error returned only if context was done.
func (p *RetryPublisher) Publish(ctx context.Context, exchange, key string, msg amqp.Publishing) error {
	for {
		err := p.Publisher.Publish(ctx, exchange, key, msg)
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
