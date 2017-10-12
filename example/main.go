package main

import (
	"context"
	"errors"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/Furdarius/rabbitroutine"
	"golang.org/x/sync/errgroup"
)

var (
	ErrTermSig = errors.New("termination signal caught")
)

func main() {
	g, ctx := errgroup.WithContext(context.Background())

	conn := rabbitroutine.Do(ctx, rabbitroutine.Config{
		Host:     "127.0.0.1",
		Port:     5672,
		Username: "guest",
		Password: "guest",
		// Max reconnect attempts
		Attempts: 20,
		// How long wait between reconnect
		Wait: 5 * time.Second,
	})

	consumer := &Consumer{
		ExchangeName: "myexch",
		QueueName:    "myqueue",
	}

	g.Go(func() error {
		log.Println("consumers starting")
		defer log.Println("consumers finished")

		return conn.StartMultipleConsumers(ctx, consumer, 1)
	})

	g.Go(func() error {
		log.Println("conn.Wait starting")
		defer log.Println("conn.Wait finished")

		return conn.Wait()
	})

	g.Go(func() error {
		log.Println("signal trap starting")

		return TermSignalTrap(ctx)
	})

	if err := g.Wait(); err != nil && err != ErrTermSig {
		log.Fatal(
			"failed to wait goroutine group",
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
