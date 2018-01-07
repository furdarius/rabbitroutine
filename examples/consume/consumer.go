package main

import (
	"context"
	"fmt"
	"log"

	"github.com/pkg/errors"
	"github.com/streadway/amqp"
)

// Consumer implement rabbitroutine.Consumer interface.
type Consumer struct {
	ExchangeName string
	QueueName    string
}

// Declare implement rabbitroutine.Consumer.(Declare) interface method.
func (c *Consumer) Declare(ctx context.Context, ch *amqp.Channel) error {
	err := ch.ExchangeDeclare(
		c.ExchangeName, // name
		"direct",       // type
		true,           // durable
		false,          // auto-deleted
		false,          // internal
		false,          // no-wait
		nil,            // arguments
	)
	if err != nil {
		return errors.WithMessage(err, "failed to declare "+c.ExchangeName)
	}

	_, err = ch.QueueDeclare(
		c.QueueName, // name
		true,        // durable
		false,       // delete when unused
		false,       // exclusive
		false,       // no-wait
		nil,         // arguments
	)
	if err != nil {
		return errors.WithMessage(err, "failed to declare "+c.QueueName)
	}

	err = ch.QueueBind(
		c.QueueName,    // queue name
		c.QueueName,    // routing key
		c.ExchangeName, // exchange
		false,          // no-wait
		nil,            // arguments
	)
	if err != nil {
		return errors.WithMessage(err, "failed to bind "+c.QueueName+" to "+c.ExchangeName)
	}

	return nil
}

// Consume implement rabbitroutine.Consumer.(Consume) interface method.
func (c *Consumer) Consume(ctx context.Context, ch *amqp.Channel) error {
	defer log.Println("consume method finished")

	err := ch.Qos(
		1,     // prefetch count
		0,     // prefetch size
		false, // global
	)
	if err != nil {
		return errors.WithMessage(err, "failed to set qos")
	}

	msgs, err := ch.Consume(
		c.QueueName,  // queue
		"myconsumer", // consumer name
		false,        // auto-ack
		false,        // exclusive
		false,        // no-local
		false,        // no-wait
		nil,          // args
	)
	if err != nil {
		return errors.WithMessage(err, "failed to consume "+c.QueueName)
	}

	for {
		select {
		case msg, ok := <-msgs:
			if !ok {
				return amqp.ErrClosed
			}

			content := string(msg.Body)

			fmt.Println("New message:", content)

			err := msg.Ack(false)
			if err != nil {
				log.Printf("failed to Ack message: %v", err)
			}
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}
