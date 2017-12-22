package rabbitroutine

import (
	"context"

	"github.com/pkg/errors"
	"github.com/streadway/amqp"
)

// Publisher used to publish messages to RabbitMQ exchange.
type Publisher struct {
	conn *Connector
}

// NewPublisher return new instance of Publisher.
func NewPublisher(conn *Connector) *Publisher {
	return &Publisher{conn}
}

// EnsurePublish sends msg to an exchange on the RabbitMQ
// and wait to ensure that have successfully been received by the server.
func (p *Publisher) EnsurePublish(ctx context.Context, exchange, key string, msg amqp.Publishing) error {
	ch, err := p.conn.Channel(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to receive channel for publishing")
	}
	defer ch.Close()

	closeCh := ch.NotifyClose(make(chan *amqp.Error, 1))

	err = ch.Confirm(false)
	if err != nil {
		return errors.Wrap(err, "failed to setup confirm mode for channel")
	}

	publishCh := ch.NotifyPublish(make(chan amqp.Confirmation, 1))

	err = ch.Publish(exchange, key, false, false, msg)
	if err != nil {
		return errors.Wrap(err, "failed to publish message")
	}

	select {
	case <-ctx.Done():
		return ctx.Err()
	case amqpErr := <-closeCh:
		return errors.Wrap(amqpErr, "failed to deliver a message")
	case <-publishCh:
		return nil
	}
}


