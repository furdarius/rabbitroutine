package rabbitroutine

import (
	"context"
	"fmt"
	"math"
	"time"

	"github.com/pkg/errors"
	"github.com/streadway/amqp"
)

var (
	// ErrNotFound indicates that RabbitMQ entity doesn't exist.
	ErrNotFound = errors.New("rabbitmq entity not found")
	// ErrNoRoute indicates that queue is bound that matches the routing key.
	// @see: https://www.rabbitmq.com/amqp-0-9-1-errata.html#section_17
	ErrNoRoute = errors.New("queue not bound")
)

// Publisher interface provides functionality of publishing to RabbitMQ.
type Publisher interface {
	// Publish used to send msg to RabbitMQ exchange.
	Publish(ctx context.Context, exchange, key string, msg amqp.Publishing) error
}

// EnsurePublisher implements Publisher interface and guarantees delivery of the message to the server.
// When EnsurePublisher used, publishing confirmation is enabled, so we have delivery guarantees.
// @see http://www.rabbitmq.com/blog/2011/02/10/introducing-publisher-confirms/
// @see https://www.rabbitmq.com/amqp-0-9-1-errata.html#section_17
type EnsurePublisher struct {
	pool *Pool
}

// NewEnsurePublisher returns a new instance of EnsurePublisher.
func NewEnsurePublisher(p *Pool) *EnsurePublisher {
	return &EnsurePublisher{pool: p}
}

// Publish sends msg to an exchange on the RabbitMQ and wait to ensure
// that msg have been successfully received by the server.
// Returns error if no queue is bound that matches the routing key.
// It will blocks until is either message is successfully delivered, context has cancelled or error received.
func (p *EnsurePublisher) Publish(ctx context.Context, exchange, key string, msg amqp.Publishing) error {
	k, err := p.pool.ChannelWithConfirm(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to receive channel for publishing")
	}

	err = p.publish(ctx, k, exchange, key, msg)

	if err == ErrNoRoute {
		p.pool.Release(k)

		return err
	}

	if err != nil {
		_ = k.Close() //nolint: gosec

		return err
	}

	p.pool.Release(k)

	return nil
}

func (p *EnsurePublisher) publish(ctx context.Context, k ChannelKeeper, exchange, key string, msg amqp.Publishing) error {
	ch := k.Channel()

	mandatory := true
	immediate := false
	err := ch.Publish(exchange, key, mandatory, immediate, msg)
	if err != nil {
		return errors.Wrap(err, "failed to publish message")
	}

	select {
	case <-ctx.Done():
		return ctx.Err()
	case amqpErr := <-k.Error():
		if amqpErr.Code == amqp.NotFound {
			return ErrNotFound
		}

		return errors.Wrap(amqpErr, "failed to deliver a message")
	case amqpRet := <-k.Return():
		if amqpRet.ReplyCode == amqp.NoRoute {
			// Unroutable mandatory or immediate messages are acknowledged immediately after
			// any Channel.NotifyReturn listeners have been notified.
			<-k.Confirm()

			return ErrNoRoute
		}

		return fmt.Errorf("failed to deliver a message: %s", amqpRet.ReplyText)
	case <-k.Confirm():
		return nil
	}
}

// FireForgetPublisher implements Publisher interface and used to publish messages to RabbitMQ exchange without delivery guarantees.
// When FireForgetPublisher used, publishing confirmation is not enabled, so we haven't delivery guarantees.
// @see http://www.rabbitmq.com/blog/2011/02/10/introducing-publisher-confirms/
type FireForgetPublisher struct {
	pool *LightningPool
}

// NewFireForgetPublisher returns a new instance of FireForgetPublisher.
func NewFireForgetPublisher(p *LightningPool) *FireForgetPublisher {
	return &FireForgetPublisher{p}
}

// Publish sends msg to an exchange on the RabbitMQ.
func (p *FireForgetPublisher) Publish(ctx context.Context, exchange, key string, msg amqp.Publishing) error {
	ch, err := p.pool.Channel(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to receive channel for publishing")
	}

	mandatory := false
	immediate := false
	err = ch.Publish(exchange, key, mandatory, immediate, msg)
	if err != nil {
		_ = ch.Close() //nolint: gosec

		return errors.Wrap(err, "failed to publish message")
	}

	p.pool.Release(ch)

	return nil
}

// RetryPublisherOption describes a functional option for configuring RetryPublisher.
type RetryPublisherOption func(*RetryPublisher)

// RetryDelayFunc returns how long to wait before retry.
type RetryDelayFunc func(attempt uint) time.Duration

// RetryPublisher retries to publish message before context done.
type RetryPublisher struct {
	Publisher

	// maxAttempts is limit of publish attempts.
	maxAttempts uint
	// delayFn returns how long to wait before next retry
	delayFn RetryDelayFunc
}

// NewRetryPublisher returns a new instance of RetryPublisherOption.
func NewRetryPublisher(p Publisher, opts ...RetryPublisherOption) *RetryPublisher {
	pub := &RetryPublisher{
		Publisher:   p,
		maxAttempts: math.MaxUint32,
		delayFn:     ConstDelay(10 * time.Millisecond),
	}

	for _, option := range opts {
		option(pub)
	}

	return pub
}

// Publish is used to send msg to RabbitMQ exchange.
// It will block until is either message is delivered or context has cancelled.
// Error returned only if context was done.
func (p *RetryPublisher) Publish(ctx context.Context, exchange, key string, msg amqp.Publishing) error {
	var err error

	for attempt := uint(1); attempt <= p.maxAttempts; attempt++ {
		err = p.Publisher.Publish(ctx, exchange, key, msg)
		if err != nil {
			select {
			case <-time.After(p.delayFn(attempt)):
				continue
			case <-ctx.Done():
				return ctx.Err()
			}
		}

		return nil
	}

	return err
}

// PublishDelaySetup sets function for publish delay time.Duration receiving.
func PublishDelaySetup(fn RetryDelayFunc) RetryPublisherOption {
	return func(pub *RetryPublisher) {
		pub.delayFn = fn
	}
}

// PublishMaxAttemptsSetup sets limit of publish attempts.
func PublishMaxAttemptsSetup(maxAttempts uint) RetryPublisherOption {
	return func(pub *RetryPublisher) {
		pub.maxAttempts = maxAttempts
	}
}

// ConstDelay returns constant delay value.
func ConstDelay(delay time.Duration) RetryDelayFunc {
	fn := func(_ uint) time.Duration {
		return delay
	}

	return fn
}

// LinearDelay returns delay value increases linearly depending on the current attempt.
func LinearDelay(delay time.Duration) RetryDelayFunc {
	fn := func(attempt uint) time.Duration {
		return time.Duration(attempt) * delay
	}

	return fn
}
