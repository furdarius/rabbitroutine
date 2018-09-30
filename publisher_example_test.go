package rabbitroutine_test

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/furdarius/rabbitroutine"
	"github.com/streadway/amqp"
)

// This example demonstrates publishing messages in RabbitMQ exchange using FireForgetPublisher.
func ExampleFireForgetPublisher() {
	ctx := context.Background()

	url := "amqp://guest:guest@127.0.0.1:5672/"

	conn := rabbitroutine.NewConnector(rabbitroutine.Config{
		// Max reconnect attempts
		ReconnectAttempts: 20000,
		// How long wait between reconnect
		Wait: 2 * time.Second,
	})

	pool := rabbitroutine.NewLightningPool(conn)
	pub := rabbitroutine.NewFireForgetPublisher(pool)

	go func() {
		err := conn.Dial(ctx, url)
		if err != nil {
			log.Println("failed to establish RabbitMQ connection:", err)
		}
	}()

	for i := 0; i < 5000; i++ {
		timeoutCtx, cancel := context.WithTimeout(ctx, 100*time.Millisecond)

		err := pub.Publish(timeoutCtx, "myexch", "myqueue", false, false, amqp.Publishing{
			Body: []byte(fmt.Sprintf("message %d", i)),
		})
		if err != nil {
			log.Println("failed to publish:", err)
		}

		cancel()
	}
}

// This example demonstrates publishing messages in RabbitMQ exchange delivery guarantees by EnsurePublisher
// and publishing retries by RetryPublisher.
func ExampleEnsurePublisher() {
	ctx := context.Background()

	url := "amqp://guest:guest@127.0.0.1:5672/"

	conn := rabbitroutine.NewConnector(rabbitroutine.Config{
		// Max reconnect attempts
		ReconnectAttempts: 20000,
		// How long wait between reconnect
		Wait: 2 * time.Second,
	})

	pool := rabbitroutine.NewPool(conn)
	ensurePub := rabbitroutine.NewEnsurePublisher(pool)
	pub := rabbitroutine.NewRetryPublisher(ensurePub)

	go func() {
		err := conn.Dial(ctx, url)
		if err != nil {
			log.Println("failed to establish RabbitMQ connection:", err)
		}
	}()

	for i := 0; i < 5000; i++ {
		timeoutCtx, cancel := context.WithTimeout(ctx, 100*time.Millisecond)

		err := pub.Publish(timeoutCtx, "myexch", "myqueue", false, false, amqp.Publishing{
			Body: []byte(fmt.Sprintf("message %d", i)),
		})
		if err != nil {
			log.Println("failed to publish:", err)
		}

		cancel()
	}
}
