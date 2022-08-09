//go:build integration
// +build integration

package rabbitroutine

import (
	"context"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/pkg/errors"
	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/stretchr/testify/assert"
)

var (
	testURL = integrationURLFromEnv()
	testCfg = Config{
		ReconnectAttempts: 20000,
		Wait:              5 * time.Second,
	}
)

type FakeConsumer struct {
	declareFn func(ctx context.Context, ch *amqp.Channel) error
	consumeFn func(ctx context.Context, ch *amqp.Channel) error
}

func (c *FakeConsumer) Declare(ctx context.Context, ch *amqp.Channel) error {
	return c.declareFn(ctx, ch)
}

func (c *FakeConsumer) Consume(ctx context.Context, ch *amqp.Channel) error {
	return c.consumeFn(ctx, ch)
}

func TestIntegrationEnsurePublisher_PublishSuccess(t *testing.T) {
	ctx := context.Background()

	conn := NewConnector(testCfg)

	go func() {
		err := conn.Dial(ctx, testURL)
		if err != nil {
			panic(err)
		}
	}()

	ch, err := conn.Channel(ctx)
	assert.NoError(t, err, "failed to receive channel")
	assert.NotNil(t, ch, "nil channel received")
	defer ch.Close()

	testName := t.Name()
	testExchange := testName + "_Exchange"
	testQueue := testName + "_Queue"
	testMsg := testName + "_Message"

	err = ch.ExchangeDeclare(testExchange, "direct", false, true, false, false, nil)
	assert.NoError(t, err, "failed to declare exchange")

	_, err = ch.QueueDeclare(testQueue, false, false, false, false, nil)
	assert.NoError(t, err)

	err = ch.QueueBind(testQueue, testQueue, testExchange, false, nil)
	assert.NoError(t, err)

	pool := NewPool(conn)
	p := NewEnsurePublisher(pool)

	err = p.Publish(ctx, testExchange, testQueue, amqp.Publishing{Body: []byte(testMsg)})
	assert.NoError(t, err, "failed to publish")
}

func TestIntegrationRetryPublisher_PublishSuccess(t *testing.T) {
	ctx := context.Background()

	conn := NewConnector(testCfg)

	go func() {
		err := conn.Dial(ctx, testURL)
		if err != nil {
			panic(err)
		}
	}()

	ch, err := conn.Channel(ctx)
	assert.NoError(t, err, "failed to receive channel")
	assert.NotNil(t, ch, "nil channel received")
	defer ch.Close()

	testName := t.Name()
	testExchange := testName + "_Exchange"
	testQueue := testName + "_Queue"
	testMsg := testName + "_Message"

	err = ch.ExchangeDeclare(testExchange, "direct", false, true, false, false, nil)
	assert.NoError(t, err, "failed to declare exchange")

	_, err = ch.QueueDeclare(testQueue, false, false, false, false, nil)
	assert.NoError(t, err)

	err = ch.QueueBind(testQueue, testQueue, testExchange, false, nil)
	assert.NoError(t, err)

	pool := NewPool(conn)
	p := NewRetryPublisher(NewEnsurePublisher(pool))

	err = p.Publish(ctx, testExchange, testQueue, amqp.Publishing{Body: []byte(testMsg)})
	assert.NoError(t, err, "failed to publish")
}

func TestIntegrationRetryPublisher_MaxAttemptsError(t *testing.T) {
	ctx := context.Background()

	conn := NewConnector(testCfg)

	go func() {
		err := conn.Dial(ctx, testURL)
		if err != nil {
			panic(err)
		}
	}()

	ch, err := conn.Channel(ctx)
	assert.NoError(t, err, "failed to receive channel")
	assert.NotNil(t, ch, "nil channel received")
	defer ch.Close()

	testName := t.Name()
	testExchange := testName + "_Exchange"
	testQueue := testName + "_Queue"
	testMsg := testName + "_Message"

	err = ch.ExchangeDeclare(testExchange, "direct", false, true, false, false, nil)
	assert.NoError(t, err, "failed to declare exchange")

	pool := NewPool(conn)
	p := NewRetryPublisher(
		NewEnsurePublisher(pool),
		PublishMaxAttemptsSetup(2),
		PublishDelaySetup(LinearDelay(10*time.Millisecond)),
	)

	err = p.Publish(ctx, testExchange, testQueue, amqp.Publishing{Body: []byte(testMsg)})
	assert.Error(t, err)
	assert.Equal(t, ErrNoRoute, err)
}

func TestIntegrationRetryPublisher_PublishedReceivingSuccess(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	conn := NewConnector(testCfg)
	pool := NewPool(conn)
	pub := NewRetryPublisher(NewEnsurePublisher(pool))

	testName := t.Name()
	testExchange := testName + "_Exchange"
	testQueue := testName + "_Queue"
	testMsg := testName + "_Message"

	deliveriesCh := make(chan string)

	consumer := &FakeConsumer{
		declareFn: func(ctx context.Context, ch *amqp.Channel) error {
			return nil
		},
		consumeFn: func(ctx context.Context, ch *amqp.Channel) error {
			err := ch.Qos(1, 0, false)
			assert.NoError(t, err)

			msgs, err := ch.Consume(testQueue, "test_consumer", true, false, false, false, nil)
			assert.NoError(t, err)

			msg := <-msgs

			content := string(msg.Body)

			deliveriesCh <- content

			// Wait for test finishing
			<-ctx.Done()

			return nil
		},
	}

	go func() {
		err := conn.Dial(ctx, testURL)
		if err != nil && err != context.Canceled {
			panic(err)
		}
	}()

	ch, err := conn.Channel(ctx)
	assert.NoError(t, err, "failed to receive channel")
	assert.NotNil(t, ch, "nil channel received")
	defer ch.Close()

	err = ch.ExchangeDeclare(testExchange, "direct", false, true, false, false, nil)
	assert.NoError(t, err, "failed to declare exchange")

	_, err = ch.QueueDeclare(testQueue, false, false, false, false, nil)
	assert.NoError(t, err)

	err = ch.QueueBind(testQueue, testQueue, testExchange, false, nil)
	assert.NoError(t, err)

	err = pub.Publish(ctx, testExchange, testQueue, amqp.Publishing{Body: []byte(testMsg)})
	assert.NoError(t, err)

	go func() {
		err := conn.StartConsumer(ctx, consumer)
		if err != nil && err != context.Canceled {
			panic(err)
		}
	}()

	actualMsg := <-deliveriesCh
	assert.Equal(t, testMsg, actualMsg)
}

func TestIntegrationRetryPublisher_ConcurrentPublishingSuccess(t *testing.T) {
	ctx := context.Background()

	conn := NewConnector(testCfg)
	pool := NewPool(conn)
	pub := NewRetryPublisher(NewEnsurePublisher(pool))

	go func() {
		err := conn.Dial(ctx, testURL)
		if err != nil {
			panic(err)
		}
	}()

	testName := t.Name()
	testExchange := testName + "_Exchange"
	testQueue := testName + "_Queue"
	testMsg := testName + "_Message"

	ch, err := conn.Channel(ctx)
	assert.NoError(t, err, "failed to receive channel")
	assert.NotNil(t, ch, "nil channel received")
	defer ch.Close()

	err = ch.ExchangeDeclare(testExchange, "direct", false, true, false, false, nil)
	assert.NoError(t, err, "failed to declare exchange")

	_, err = ch.QueueDeclare(testQueue, false, false, false, false, nil)
	assert.NoError(t, err)

	err = ch.QueueBind(testQueue, testQueue, testExchange, false, nil)
	assert.NoError(t, err)

	// Number of goroutine for concurrent publishing
	N := 2

	var wg sync.WaitGroup
	wg.Add(N)

	for i := 0; i < N; i++ {
		go func() {
			err := pub.Publish(ctx, testExchange, testQueue, amqp.Publishing{Body: []byte(testMsg)})
			if err != nil {
				panic(err)
			}

			wg.Done()
		}()
	}

	wg.Wait()
}

func TestIntegrationRetryPublisherFireForget_ConcurrentPublishingSuccess(t *testing.T) {
	ctx := context.Background()

	conn := NewConnector(testCfg)
	pool := NewLightningPool(conn)
	pub := NewRetryPublisher(NewFireForgetPublisher(pool))

	go func() {
		err := conn.Dial(ctx, testURL)
		if err != nil {
			panic(err)
		}
	}()

	testName := t.Name()
	testExchange := testName + "_Exchange"
	testQueue := testName + "_Queue"
	testMsg := testName + "_Message"

	ch, err := conn.Channel(ctx)
	assert.NoError(t, err, "failed to receive channel")
	assert.NotNil(t, ch, "nil channel received")
	defer ch.Close()

	err = ch.ExchangeDeclare(testExchange, "direct", false, true, false, false, nil)
	assert.NoError(t, err, "failed to declare exchange")

	_, err = ch.QueueDeclare(testQueue, false, false, false, false, nil)
	assert.NoError(t, err)

	err = ch.QueueBind(testQueue, testQueue, testExchange, false, nil)
	assert.NoError(t, err)

	// Number of goroutine for concurrent publishing
	N := 2

	var wg sync.WaitGroup
	wg.Add(N)

	for i := 0; i < N; i++ {
		go func() {
			err := pub.Publish(ctx, testExchange, testQueue, amqp.Publishing{Body: []byte(testMsg)})
			if err != nil {
				panic(err)
			}

			wg.Done()
		}()
	}

	wg.Wait()
}

func TestIntegrationEnsurePublisher_PublishWithTimeoutError(t *testing.T) {
	ctx := context.Background()

	conn := NewConnector(testCfg)
	pool := NewPool(conn)
	pub := NewEnsurePublisher(pool)

	go func() {
		err := conn.Dial(ctx, testURL)
		if err != nil {
			panic(err)
		}
	}()

	testName := t.Name()
	testExchange := testName + "_Exchange"
	testQueue := testName + "_Queue"
	testMsg := testName + "_Message"

	ch, err := conn.Channel(ctx)
	assert.NoError(t, err, "failed to receive channel")
	assert.NotNil(t, ch, "nil channel received")
	defer ch.Close()

	err = ch.ExchangeDeclare(testExchange, "direct", false, true, false, false, nil)
	assert.NoError(t, err, "failed to declare exchange")

	_, err = ch.QueueDeclare(testQueue, false, false, false, false, nil)
	assert.NoError(t, err)

	err = ch.QueueBind(testQueue, testQueue, testExchange, false, nil)
	assert.NoError(t, err)

	timeoutCtx, cancel := context.WithTimeout(ctx, 0)
	defer cancel()

	err = pub.Publish(timeoutCtx, testExchange, testQueue, amqp.Publishing{Body: []byte(testMsg)})
	assert.Equal(t, errors.Cause(err), context.DeadlineExceeded)
}

func TestIntegrationEnsurePublisher_ConcurrentPublishWithTimeout(t *testing.T) {
	ctx := context.Background()

	conn := NewConnector(testCfg)
	pool := NewPool(conn)
	pub := NewEnsurePublisher(pool)

	go func() {
		err := conn.Dial(ctx, testURL)
		if err != nil {
			panic(err)
		}
	}()

	testName := t.Name()
	testExchange := testName + "_Exchange"
	testQueue := testName + "_Queue"
	testMsg := testName + "_Message"

	ch, err := conn.Channel(ctx)
	assert.NoError(t, err, "failed to receive channel")
	assert.NotNil(t, ch, "nil channel received")
	defer ch.Close()

	err = ch.ExchangeDeclare(testExchange, "direct", false, true, false, false, nil)
	assert.NoError(t, err, "failed to declare exchange")

	_, err = ch.QueueDeclare(testQueue, false, false, false, false, nil)
	assert.NoError(t, err)

	err = ch.QueueBind(testQueue, testQueue, testExchange, false, nil)
	assert.NoError(t, err)

	// Number of goroutine for concurrent publishing
	N := 10

	var wg sync.WaitGroup
	wg.Add(N)

	for i := 0; i < N; i++ {
		go func(gNum int) {
			timeout := time.Duration(gNum*2) * time.Millisecond
			timeoutCtx, cancel := context.WithTimeout(ctx, timeout)
			defer cancel()

			err := pub.Publish(timeoutCtx, testExchange, testQueue, amqp.Publishing{Body: []byte(testMsg)})
			if err != nil {
				if errors.Cause(err) != context.DeadlineExceeded {
					panic(err)
				}
			}

			wg.Done()
		}(i)
	}

	wg.Wait()
}

func TestIntegrationEnsurePublisher_PublishNoRouteError(t *testing.T) {
	ctx := context.Background()

	conn := NewConnector(testCfg)
	pool := NewPool(conn)
	pub := NewEnsurePublisher(pool)

	go func() {
		err := conn.Dial(ctx, testURL)
		if err != nil {
			panic(err)
		}
	}()

	testName := t.Name()
	testExchange := testName + "_Exchange"
	testQueue := testName + "_Queue"
	testMsg := testName + "_Message"

	ch, err := conn.Channel(ctx)
	assert.NoError(t, err, "failed to receive channel")
	assert.NotNil(t, ch, "nil channel received")
	defer ch.Close()

	err = ch.ExchangeDeclare(testExchange, "direct", false, true, false, false, nil)
	assert.NoError(t, err, "failed to declare exchange")

	timeoutCtx, cancel := context.WithTimeout(ctx, time.Second)
	defer cancel()

	err = pub.Publish(timeoutCtx, testExchange, "unbounded.key", amqp.Publishing{Body: []byte(testMsg)})
	assert.Error(t, err)
	assert.Equal(t, ErrNoRoute, err)

	_, err = ch.QueueDeclare(testQueue, false, false, false, false, nil)
	assert.NoError(t, err)

	err = ch.QueueBind(testQueue, testQueue, testExchange, false, nil)
	assert.NoError(t, err)

	err = pub.Publish(timeoutCtx, testExchange, "unbounded.key", amqp.Publishing{Body: []byte(testMsg)})
	assert.Error(t, err)
	assert.Equal(t, ErrNoRoute, err)
}

func integrationURLFromEnv() string {
	url := os.Getenv("AMQP_URL")
	if url == "" {
		url = "amqp://"
	}

	return url
}
