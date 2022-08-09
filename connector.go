package rabbitroutine

import (
	"context"
	"math"
	"sync"
	"time"

	"github.com/pkg/errors"
	amqp "github.com/rabbitmq/amqp091-go"
	"golang.org/x/sync/errgroup"
)

const (
	defaultHeartbeat = 10 * time.Second
	defaultLocale    = "en_US"
)

// Config stores reconnect options.
type Config struct {
	// ReconnectAttempts is a number that defines how many reconnect attempts would be made after the connection was broke off.
	// After a new connection have been established this number is reset.
	// So, when a next broke off happens there will be not less than ReconnectAttempts attempts to reconnect.
	// In case of maximum reconnect attempts exceeded* Dial or DialConfig func will just return error and that's it.
	// It's your turn to handle this situation.
	// But in generall it's better have unlimited ReconnectAttemts and log errors using Connector.AddRetriedListener (see examples dir)
	ReconnectAttempts uint
	// How long to wait between reconnect attempts.
	Wait time.Duration
}

// Connector implement RabbitMQ failover.
type Connector struct {
	cfg Config

	conn   *amqp.Connection
	connCh chan *amqp.Connection

	// retries has list of Retried event handlers.
	retries []func(Retried)
	// dials has list of Dialed event handlers.
	dials []func(Dialed)
	// amqpnotifies has list of AMQPNotified event handlers.
	amqpnotifies []func(AMQPNotified)
}

// NewConnector return a new instance of Connector.
func NewConnector(cfg Config) *Connector {
	if cfg.ReconnectAttempts == 0 {
		cfg.ReconnectAttempts = math.MaxUint32
	}

	return &Connector{
		cfg:    cfg,
		connCh: make(chan *amqp.Connection),
	}
}

// StartMultipleConsumers is used to start Consumer "count" times.
// Method Declare will be called once, and Consume will be called "count" times (one goroutine per call)
// so you can scale consumer horizontally.
// It's blocking method.
//
// NOTE: It's blocking method.
// nolint: gocyclo
func (c *Connector) StartMultipleConsumers(ctx context.Context, consumer Consumer, count int) error {
	var lastErr error

	for {
		if contextDone(ctx) {
			return lastErr
		}

		// On error wait for c.cfg.Wait time before consumer restart
		if lastErr != nil {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(c.cfg.Wait):
			}
		}

		// Use declareChannel only for consumer.Declare,
		// and close it after successful declaring.
		// nolint: vetshadow
		declareChannel, err := c.Channel(ctx)
		if err != nil {
			lastErr = errors.WithMessage(err, "failed to get channel")

			continue
		}

		err = consumer.Declare(ctx, declareChannel)
		if err != nil {
			lastErr = errors.WithMessage(err, "failed to declare consumer")

			continue
		}

		err = declareChannel.Close()
		if err != nil {
			lastErr = errors.WithMessage(err, "failed to close declareChannel")

			continue
		}

		var g errgroup.Group

		consumeCtx, cancel := context.WithCancel(ctx)

		for i := 0; i < count; i++ {
			// Allocate new channel for each consumer.
			// nolint: vetshadow
			consumeChannel, err := c.Channel(consumeCtx)
			if err != nil {
				// If we got error then stop all previously started consumers
				// and wait before they will be finished.
				cancel()

				break
			}

			var once sync.Once

			closeCh := consumeChannel.NotifyClose(make(chan *amqp.Error, 1))

			// Start two goroutine: one for consuming and second for close notification receiving.
			// When close notification received via closeCh, then all consumers get notification via consumeCtx.
			// In this case consuming must be finished and then goroutine will finish their work.

			g.Go(func() error {
				// On consume exit send stop signal to all consumer's goroutines.
				defer cancel()

				// nolint: vetshadow
				err := consumer.Consume(consumeCtx, consumeChannel)
				if err != nil {
					return errors.Wrap(err, "failed to consume")
				}

				var closeErr error
				once.Do(func() {
					closeErr = consumeChannel.Close()
				})

				if closeErr != nil && closeErr != amqp.ErrClosed {
					return errors.Wrap(closeErr, "failed to close amqp channel")
				}

				return nil
			})

			g.Go(func() error {
				// On amqp error send stop signal to all consumer's goroutines.
				defer cancel()

				var stopErr error

				select {
				case <-consumeCtx.Done():
					stopErr = consumeCtx.Err()
				case amqpErr := <-closeCh:
					c.emitAMQPNotified(AMQPNotified{amqpErr})

					stopErr = amqpErr
				}

				var closeErr error
				once.Do(func() {
					closeErr = consumeChannel.Close()
				})

				if closeErr != nil && closeErr != amqp.ErrClosed {
					return errors.Wrap(closeErr, "failed to close amqp channel")
				}

				return stopErr
			})
		}

		lastErr = g.Wait()

		cancel()
	}
}

// StartConsumer is used to start Consumer.
//
// NOTE: It's blocking method.
func (c *Connector) StartConsumer(ctx context.Context, consumer Consumer) error {
	return c.StartMultipleConsumers(ctx, consumer, 1)
}

// Channel allocate and return new amqp.Channel.
// On error new Channel should be opened.
//
// NOTE: It's blocking method. (It's waiting before connection will be established)
func (c *Connector) Channel(ctx context.Context) (*amqp.Channel, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case conn := <-c.connCh:
		return conn.Channel()
	}
}

// AddRetriedListener registers a event listener of
// connection establishing attempts.
//
// NOTE: not concurrency-safe.
func (c *Connector) AddRetriedListener(h func(Retried)) {
	c.retries = append(c.retries, h)
}

// emitRetry notify listeners about connection retry event.
func (c *Connector) emitRetried(r Retried) {
	for _, h := range c.retries {
		h(r)
	}
}

// AddDialedListener registers a event listener of
// connection successfully established.
//
// NOTE: not concurrency-safe.
func (c *Connector) AddDialedListener(h func(r Dialed)) {
	c.dials = append(c.dials, h)
}

// emitDialed notify listeners about dial event.
func (c *Connector) emitDialed(d Dialed) {
	for _, h := range c.dials {
		h(d)
	}
}

// AddAMQPNotifiedListener registers a event listener of
// AMQP error receiving.
//
// NOTE: not concurrency-safe.
func (c *Connector) AddAMQPNotifiedListener(h func(n AMQPNotified)) {
	c.amqpnotifies = append(c.amqpnotifies, h)
}

// emitAMQPNotified notify listeners about AMQPNotified event.
func (c *Connector) emitAMQPNotified(n AMQPNotified) {
	for _, h := range c.amqpnotifies {
		h(n)
	}
}

// dialWithIt try to connect to RabbitMQ.
// On error it will reconnect.
// If maximum retry count exceeded error will return.
func (c *Connector) dialWithIt(ctx context.Context, url string, config amqp.Config) error {
	var err error

	for i := uint(1); i <= c.cfg.ReconnectAttempts; i++ {
		c.conn, err = amqp.DialConfig(url, config)
		if err != nil {
			c.emitRetried(Retried{i, err})

			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(c.cfg.Wait):
				continue
			}
		}

		return nil
	}

	return errors.WithMessage(err, "maximum attempts exceeded")
}

// connBroadcast is used to send available connection to connCh.
func (c *Connector) connBroadcast(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case c.connCh <- c.conn:
		}
	}
}

// Dial will try to keep RabbitMQ connection active
// by catching and handling connection errors.
// It will return any error only if ctx was done.
//
// NOTE: It's blocking method.
func (c *Connector) Dial(ctx context.Context, url string) error {
	return c.DialConfig(ctx, url, amqp.Config{
		Heartbeat: defaultHeartbeat,
		Locale:    defaultLocale,
	})
}

// DialConfig used to configure RabbitMQ connection with amqp.Config.
// It will try to keep RabbitMQ connection active
// by catching and handling connection errors.
// It will return any error only if ctx was done.
//
// NOTE: It's blocking method.
func (c *Connector) DialConfig(ctx context.Context, url string, config amqp.Config) error {
	for {
		err := c.dialWithIt(ctx, url, config)
		if err != nil {
			return errors.WithMessage(err, "failed to dial")
		}

		// In the case of connection problems,
		// we will get an error from closeCh
		closeCh := c.conn.NotifyClose(make(chan *amqp.Error, 1))

		// After context cancellation we must wait for finishing of connBroadcast to avoid data race on c.conn.
		var wg sync.WaitGroup
		wg.Add(1)

		broadcastCtx, cancel := context.WithCancel(ctx)
		go func() {
			c.connBroadcast(broadcastCtx)

			wg.Done()
		}()

		c.emitDialed(Dialed{})

		select {
		case <-ctx.Done():
			cancel()
			wg.Wait()

			err = c.conn.Close()
			// It's not error if connection has already been closed.
			if err != nil && err != amqp.ErrClosed {
				return errors.WithMessage(err, "failed to close rabbitmq connection")
			}

			return ctx.Err()
		case amqpErr := <-closeCh:
			cancel()
			wg.Wait()

			c.emitAMQPNotified(AMQPNotified{amqpErr})
		}
	}
}

// contextDone used to check if ctx.Done() channel was closed.
func contextDone(ctx context.Context) bool {
	select {
	case <-ctx.Done():
		return true
	default:
	}

	return false
}
