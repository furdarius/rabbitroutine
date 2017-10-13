package main

import (
	"fmt"

	"github.com/pkg/errors"
	"github.com/streadway/amqp"
)

type Consumer struct {
	ExchangeName string
	QueueName    string
}

func (c *Consumer) Declare(ch *amqp.Channel) error {
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

func (c *Consumer) Consume(ch *amqp.Channel) error {
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

	for msg := range msgs {
		content := string(msg.Body)

		fmt.Println(content)

		msg.Ack(false)
	}

	return nil
}
