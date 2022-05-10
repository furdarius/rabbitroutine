package main

import (
	"context"
	"fmt"
	"log"
	"time"

	darkmq "github.com/sagleft/darkrmq"
	"github.com/streadway/amqp"
)

func main() {
	ctx := context.Background()

	url := "amqp://guest:guest@127.0.0.1:5672/"

	conn := darkmq.NewConnector(darkmq.Config{
		// Max reconnect attempts
		ReconnectAttempts: 20,
		// How long wait between reconnect
		Wait: 2 * time.Second,
	})

	pool := darkmq.NewPool(conn)
	ensurePub := darkmq.NewEnsurePublisher(pool)
	pub := darkmq.NewRetryPublisher(
		ensurePub,
		darkmq.PublishMaxAttemptsSetup(16),
		darkmq.PublishDelaySetup(darkmq.LinearDelay(10*time.Millisecond)),
	)

	go func() {
		err := conn.Dial(ctx, url)
		if err != nil {
			log.Fatalf("failed to establish RabbitMQ connection: %v", err)
		}
	}()

	exchangeName := "myexchange"
	queueName := "myqueue"

	ch, err := conn.Channel(ctx)
	if err != nil {
		log.Fatalf("failed to create channel: %v", err)
	}

	err = ch.ExchangeDeclare(exchangeName, "direct", true, false, false, false, nil)
	if err != nil {
		log.Fatalf("failed to declare exchange: %v", err)
	}

	_, err = ch.QueueDeclare(queueName, true, false, false, false, nil)
	if err != nil {
		log.Fatalf("failed to declare queue: %v", err)
	}

	err = ch.QueueBind(queueName, queueName, exchangeName, false, nil)
	if err != nil {
		log.Fatalf("failed to bind queue: %v", err)
	}

	err = ch.Close()
	if err != nil {
		log.Printf("failed to close channel: %v", err)
	}

	for i := 0; i < 500000; i++ {
		timeoutCtx, cancel := context.WithTimeout(ctx, 100*time.Millisecond)

		err := pub.Publish(timeoutCtx, exchangeName, queueName, amqp.Publishing{
			Body: []byte(fmt.Sprintf("message %d", i)),
		})
		if err != nil {
			log.Println("failed to publish:", err)
		}

		cancel()
	}
}
