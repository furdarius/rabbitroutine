package rabbitroutine

import (
	"time"

	"context"

	"fmt"
	"github.com/pkg/errors"
	"github.com/streadway/amqp"
	"golang.org/x/sync/errgroup"
)

// Connector do all rabbitmq failover routine for you.
type Connector struct {
	cfg       Config
	conn      *amqp.Connection
	errg      *errgroup.Group
	consumers []Consumer
	connCh    chan *amqp.Connection
}

// StartMultipleConsumers is used to start implementation of Consumer interface "count" times.
// Method Declare will be called once, and Consume will be called "count" times (one goroutine per call).
// It's blocking method.
func (c *Connector) StartMultipleConsumers(ctx context.Context, consumer Consumer, count int) error {
	for {
		// Use declareChannel only for consumer.Declare,
		// and close it after successful declaring.
		declareChannel, err := c.Channel(ctx)
		if err != nil {
			if contextDone(ctx) {
				return errors.WithMessage(err, "failed to get channel")
			}

			continue
		}

		err = consumer.Declare(ctx, declareChannel)
		if err != nil {
			if contextDone(ctx) {
				return errors.WithMessage(err, "failed to declare consumer")
			}

			continue
		}
		declareChannel.Close()

		var g errgroup.Group

		consumeCtx, cancel := context.WithCancel(ctx)

		for i := 0; i < count; i++ {
			// Allocate new channel for each consumer.
			consumeChannel, err := c.Channel(consumeCtx)
			if err != nil {
				// If we got error then stop all previously started consumers
				// and wait before they will be finished.
				cancel()

				break
			}

			closeCh := consumeChannel.NotifyClose(make(chan *amqp.Error, 1))

			// Start two goroutine: one for consume and second for close notification receiving.
			// When close notification received via closeCh, then consumeCtx and consumeChannel will be close.
			// In this case both goroutine must finish their work.

			g.Go(func() error {
				defer cancel()

				err := consumer.Consume(consumeCtx, consumeChannel)
				if err != nil {
					return err
				}

				closeErr := consumeChannel.Close()
				if closeErr != nil {
					return closeErr
				}

				return nil
			})

			g.Go(func() error {
				var err error

				select {
				case <-consumeCtx.Done():
					err = consumeCtx.Err()
				case amqpErr := <-closeCh:
					// On amqp error send stop signal to all consumer's goroutines.
					cancel()

					err = amqpErr
				}

				closeErr := consumeChannel.Close()
				if closeErr != nil {
					return closeErr
				}

				return err
			})
		}

		err = g.Wait()
		if err != nil && contextDone(ctx) {
			cancel()

			return err
		}

		cancel()
	}
}

// StartMultipleConsumers is used to start implementation of Consumer interface.
// It's blocking method.
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
// It's blocking method.
func (c *Connector) Wait() error {
	return c.errg.Wait()
}

// Channel allocate and return new amqp.Channel.
// On error new Channel should be opened.
func (c *Connector) Channel(ctx context.Context) (*amqp.Channel, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case conn := <-c.connCh:
		return conn.Channel()
	}
}

// dial attempts to connect to rabbitmq.
func (c *Connector) dial(ctx context.Context) error {
	var err error

	url := c.cfg.URL()

	for i := 0; i < c.cfg.Attempts; i++ {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			c.conn, err = amqp.Dial(url)
			if err != nil {
				time.Sleep(c.cfg.Wait)

				continue
			}

			return nil
		}
	}

	return err
}

// connBroadcast is used to synced return amqpConnection
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
// It's blocking method.
func (c *Connector) start(ctx context.Context) error {
	for {
		err := c.dial(ctx)
		if err != nil {
			return errors.WithMessage(err, "failed to dial")
		}

		fmt.Println("dialed")

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
		case <-closeCh:
			fmt.Println("connection close channel caught")

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
