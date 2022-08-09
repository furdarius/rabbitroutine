package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/furdarius/rabbitroutine"
	amqp "github.com/rabbitmq/amqp091-go"
)

func main() {
	ctx := context.Background()

	url := "amqp://guest:guest@127.0.0.1:5672/"

	conn := rabbitroutine.NewConnector(rabbitroutine.Config{
		// Max reconnect attempts
		ReconnectAttempts: 20,
		// How long wait between reconnect
		Wait: 2 * time.Second,
	})

	pool := rabbitroutine.NewPool(conn)
	ensurePub := rabbitroutine.NewEnsurePublisher(pool)
	pub := rabbitroutine.NewRetryPublisher(
		ensurePub,
		rabbitroutine.PublishMaxAttemptsSetup(16),
		rabbitroutine.PublishDelaySetup(rabbitroutine.LinearDelay(10*time.Millisecond)),
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
