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
		Attempts: 20,
		// How long wait between reconnect
		Wait: 2 * time.Second,
	})

	pool := rabbitroutine.NewPool(conn)
	ensurePub := rabbitroutine.NewEnsurePublisher(pool)
	pub := rabbitroutine.NewRetryPublisher(ensurePub)

	// nolint: errcheck
	go conn.Start(ctx)

	for i := 1; i <= 5000; i++ {
		err := pub.Publish(ctx, "myexch", "myqueue", amqp.Publishing{
			Body: []byte(fmt.Sprintf("message %d", i)),
		})
		if err != nil {
			log.Println("publish error:", err)
		}
	}
}
