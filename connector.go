package rabbitroutine

import (
	"context"
	"time"

	"github.com/pkg/errors"
	"github.com/streadway/amqp"
	"golang.org/x/sync/errgroup"
)

// Connector do all rabbitmq failover routine for you.
type Connector struct {
	cfg    Config
	conn   *amqp.Connection
	errg   *errgroup.Group
	connCh chan *amqp.Connection

	// retries has list of Retried event handlers.
	retries []func(Retried)
	// dials has list of Dialed event handlers.
	dials []func(Dialed)
	// amqpnotifies has list of AMQPNotified event handlers.
	amqpnotifies []func(AMQPNotified)
}

// StartMultipleConsumers is used to start Consumer "count" times.
// Method Declare will be called once, and Consume will be called "count" times (one goroutine per call).
// It's blocking method.
//
// NOTE: It's blocking method.
// nolint: gocyclo
func (c *Connector) StartMultipleConsumers(ctx context.Context, consumer Consumer, count int) error {
	var startErr error

	for {
		if contextDone(ctx) {
			return startErr
		}

		// Use declareChannel only for consumer.Declare,
		// and close it after successful declaring.
		// nolint: vetshadow
		declareChannel, err := c.Channel(ctx)
		if err != nil {
			startErr = errors.WithMessage(err, "failed to get channel")

			continue
		}

		err = consumer.Declare(ctx, declareChannel)
		if err != nil {
			startErr = errors.WithMessage(err, "failed to declare consumer")

			continue
		}

		err = declareChannel.Close()
		if err != nil {
			startErr = errors.WithMessage(err, "failed to close declareChannel")

			continue
		}

		var g errgroup.Group

		// nolint: vetshadow
		ctx, cancel := context.WithCancel(ctx)

		for i := 0; i < count; i++ {
			// Allocate new channel for each consumer.
			// nolint: vetshadow
			consumeChannel, err := c.Channel(ctx)
			if err != nil {
				// If we got error then stop all previously started consumers
				// and wait before they will be finished.
				cancel()

				break
			}

			closeCh := consumeChannel.NotifyClose(make(chan *amqp.Error, 1))

			// Start two goroutine: one for consuming and second for close notification receiving.
			// When close notification received via closeCh, then all consumers get notification via consumeCtx.
			// In this case consuming must be finished and then goroutine will finish their work.

			g.Go(func() error {
				// On consume exit send stop signal to all consumer's goroutines.
				defer cancel()

				// nolint: vetshadow
				err := consumer.Consume(ctx, consumeChannel)
				if err != nil {
					return err
				}

				return consumeChannel.Close()
			})

			g.Go(func() error {
				// On amqp error send stop signal to all consumer's goroutines.
				defer cancel()

				var stopErr error

				select {
				case <-ctx.Done():
					stopErr = ctx.Err()
				case amqpErr := <-closeCh:
					c.emitAMQPNotified(AMQPNotified{amqpErr})

					stopErr = amqpErr
				}

				err := consumeChannel.Close()
				if err != nil {
					return err
				}

				return stopErr
			})
		}

		startErr = g.Wait()

		// go vet notify: the cancel function is not used on all paths (possible context leak)
		cancel()
	}
}

// StartConsumer is used to start Consumer.
//
// NOTE: It's blocking method.
func (c *Connector) StartConsumer(ctx context.Context, consumer Consumer) error {
	return c.StartMultipleConsumers(ctx, consumer, 1)
}

// Do open connection with rabbitmq and
// start routine to keep it active.
func Do(ctx context.Context, cfg Config) *Connector {
	group, ctx := errgroup.WithContext(ctx)

	c := &Connector{
		cfg:    cfg,
		errg:   group,
		connCh: make(chan *amqp.Connection),
	}

	group.Go(func() error {
		return c.start(ctx)
	})

	return c
}

// Wait is used to catch an error from Connector.
//
// NOTE: It's blocking method.
func (c *Connector) Wait() error {
	return c.errg.Wait()
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

// dial attempts to connect to rabbitmq.
func (c *Connector) dial(ctx context.Context) error {
	var err error

	url := c.cfg.URL()

	for i := 1; i <= c.cfg.Attempts; i++ {
		c.conn, err = amqp.Dial(url)
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

// Start try to keep rabbitmq connection active
// by catching and handling connection errors.
//
// NOTE: It's blocking method.
func (c *Connector) start(ctx context.Context) error {
	for {
		err := c.dial(ctx)
		if err != nil {
			return errors.WithMessage(err, "failed to dial")
		}

		c.emitDialed(Dialed{})

		// In the case of connection problems,
		// we will get an error from closeCh
		closeCh := c.conn.NotifyClose(make(chan *amqp.Error, 1))

		brctx, cancel := context.WithCancel(ctx)
		go c.connBroadcast(brctx)

		select {
		case <-ctx.Done():
			cancel()

			err = c.conn.Close()
			// It's not error if connection has already been closed.
			if err != nil && err != amqp.ErrClosed {
				return errors.WithMessage(err, "failed to close rabbitmq connection")
			}

			return ctx.Err()
		case amqpErr := <-closeCh:
			c.emitAMQPNotified(AMQPNotified{amqpErr})

			cancel()
		}
	}
}

// contextDone used to check was ctx.Done() channel closed.
func contextDone(ctx context.Context) bool {
	select {
	case <-ctx.Done():
		return true
	default:
	}

	return false
}
