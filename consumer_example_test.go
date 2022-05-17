package darkmq_test

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	darkmq "github.com/sagleft/darkrmq"
	"github.com/streadway/amqp"
)

// Consumer implement darkmq.Consumer interface.
type Consumer struct {
	ExchangeName string
	QueueName    string
	Tag          string
}

// Declare implement darkmq.Consumer.(Declare) interface method.
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
		log.Printf("failed to declare exchange %v: %v", c.ExchangeName, err)

		return err
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
		log.Printf("failed to declare queue %v: %v", c.QueueName, err)

		return err
	}

	err = ch.QueueBind(
		c.QueueName,    // queue name
		c.QueueName,    // routing key
		c.ExchangeName, // exchange
		false,          // no-wait
		nil,            // arguments
	)
	if err != nil {
		log.Printf("failed to bind queue %v: %v", c.QueueName, err)

		return err
	}

	return nil
}

// Consume implement darkmq.Consumer.(Consume) interface method.
func (c *Consumer) Consume(ctx context.Context, ch *amqp.Channel) error {
	defer log.Println("consume method finished")

	err := ch.Qos(
		1,     // prefetch count
		0,     // prefetch size
		false, // global
	)
	if err != nil {
		log.Printf("failed to set qos: %v", err)

		return err
	}

	msgs, err := ch.Consume(
		c.QueueName, // queue
		c.Tag,       // consumer name
		false,       // auto-ack
		false,       // exclusive
		false,       // no-local
		false,       // no-wait
		nil,         // args
	)
	if err != nil {
		log.Printf("failed to consume %v: %v", c.QueueName, err)

		return err
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

func (c *Consumer) GetTag() string {
	return c.Tag
}

// This example demonstrates consuming messages from RabbitMQ queue.
func ExampleConsumer() {
	ctx := context.Background()

	url := "amqp://guest:guest@127.0.0.1:5672/"

	conn := darkmq.NewConnector(darkmq.Config{
		// Max reconnect attempts
		ReconnectAttempts: 20,
		// How long wait between reconnect
		Wait: 2 * time.Second,
	})

	conn.AddRetriedListener(func(r darkmq.Retried) {
		log.Printf("try to connect to RabbitMQ: attempt=%d, error=\"%v\"",
			r.ReconnectAttempt, r.Error)
	})

	conn.AddDialedListener(func(_ darkmq.Dialed) {
		log.Printf("RabbitMQ connection successfully established")
	})

	conn.AddAMQPNotifiedListener(func(n darkmq.AMQPNotified) {
		log.Printf("RabbitMQ error received: %v", n.Error)
	})

	consumer := &Consumer{
		ExchangeName: "myexch",
		QueueName:    "myqueue",
		Tag:          "myconsumer",
	}

	go func() {
		err := conn.Dial(ctx, url)
		if err != nil {
			log.Println("failed to establish RabbitMQ connection:", err)
		}
	}()

	go func() {
		err := conn.StartMultipleConsumers(ctx, consumer, 5)
		if err != nil {
			log.Println("failed to start consumer:", err)
		}
	}()

	sigc := make(chan os.Signal, 1)
	signal.Notify(sigc, syscall.SIGINT, os.Interrupt, syscall.SIGTERM)

	// Wait for OS termination signal
	<-sigc
}
