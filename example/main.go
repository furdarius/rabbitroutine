package main

import (
	"context"
	"errors"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/furdarius/rabbitroutine"
	"golang.org/x/sync/errgroup"
)

var (
	ErrTermSig = errors.New("termination signal caught")
)

func main() {
	g, ctx := errgroup.WithContext(context.Background())

	conn := rabbitroutine.New(rabbitroutine.Config{
		Host:     "127.0.0.1",
		Port:     5672,
		Username: "guest",
		Password: "guest",
		// Max reconnect attempts
		Attempts: 20,
		// How long wait between reconnect
		Wait: 2 * time.Second,
	})

	conn.AddRetriedListener(func(r rabbitroutine.Retried) {
		log.Printf("try to connect to RabbitMQ: attempt=%d, error=\"%v\"",
			r.Attempt, r.Error)
	})

	conn.AddDialedListener(func(_ rabbitroutine.Dialed) {
		log.Printf("RabbitMQ connection successfully established")
	})

	conn.AddAMQPNotifiedListener(func(n rabbitroutine.AMQPNotified) {
		log.Printf("RabbitMQ error received: %v", n.Error)
	})

	consumer := &Consumer{
		ExchangeName: "myexch",
		QueueName:    "myqueue",
	}

	g.Go(func() error {
		log.Println("conn.Start starting")
		defer log.Println("conn.Start finished")

		return conn.Start(ctx)
	})

	g.Go(func() error {
		log.Println("consumers starting")
		defer log.Println("consumers finished")

		return conn.StartMultipleConsumers(ctx, consumer, 5)
	})

	g.Go(func() error {
		log.Println("signal trap starting")
		defer log.Println("signal trap finished")

		return TermSignalTrap(ctx)
	})

	if err := g.Wait(); err != nil && err != ErrTermSig {
		log.Fatal(
			"failed to wait goroutine group: ",
			err)
	}
}

// TermSignalTrap used to catch termination signal from OS
// and return error to golang.org/x/sync/errgroup
func TermSignalTrap(ctx context.Context) error {
	sigc := make(chan os.Signal, 1)
	signal.Notify(sigc, syscall.SIGINT, os.Interrupt, syscall.SIGTERM)

	select {
	case <-sigc:
		log.Println("termination signal caught")
		return ErrTermSig
	case <-ctx.Done():
		return ctx.Err()
	}
}
