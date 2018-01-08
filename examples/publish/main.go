package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/furdarius/rabbitroutine"
	"github.com/streadway/amqp"
)

func main() {
	ctx := context.Background()

	conn := rabbitroutine.NewConnector(rabbitroutine.Config{
		Host:     "127.0.0.1",
		Port:     5672,
		Username: "guest",
		Password: "guest",
		// Max reconnect attempts
		Attempts: 20000,
		// How long wait between reconnect
		Wait: 2 * time.Second,
	})

	pool := rabbitroutine.NewPool(conn)
	ensurePub := rabbitroutine.NewEnsurePublisher(pool)
	pub := rabbitroutine.NewRetryPublisher(ensurePub)

	go func() {
		err := conn.Start(ctx)
		if err != nil {
			log.Println("failed to establish RabbitMQ connection:", err)
		}
	}()

	for i := 0; i < 5000; i++ {
		timeoutCtx, cancel := context.WithTimeout(ctx, 100 * time.Millisecond)

		err := pub.Publish(timeoutCtx, "myexch", "myqueue", amqp.Publishing{
			Body: []byte(fmt.Sprintf("message %d", i)),
		})
		if err != nil {
			log.Println("failed to publish:", err)
		}

		cancel()
	}
}
