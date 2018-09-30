package rabbitroutine

import (
	"context"
	"testing"
	"time"

	"github.com/pkg/errors"
	"github.com/streadway/amqp"
	"github.com/stretchr/testify/assert"
)

func TestFireForgetPublisherImplementPublisher(t *testing.T) {
	assert.Implements(t, (*Publisher)(nil), new(FireForgetPublisher))
}

func TestEnsurePublisherImplementPublisher(t *testing.T) {
	assert.Implements(t, (*Publisher)(nil), new(EnsurePublisher))
}

func TestRetryPublisherImplementPublisher(t *testing.T) {
	assert.Implements(t, (*Publisher)(nil), new(RetryPublisher))
}

func TestFireForgetPublisherRespectContext(t *testing.T) {
	defer time.AfterFunc(1*time.Second, func() { panic("FireForgetPublisher don't respect context") }).Stop()

	conn := NewConnector(Config{})
	pool := NewLightningPool(conn)

	pub := FireForgetPublisher{pool}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := pub.Publish(ctx, "test", "test", false, false, amqp.Publishing{})
	assert.Error(t, err)
	assert.Equal(t, errors.Cause(err), ctx.Err())
}

func TestEnsurePublisherRespectContext(t *testing.T) {
	defer time.AfterFunc(1*time.Second, func() { panic("EnsurePublisher don't respect context") }).Stop()

	conn := NewConnector(Config{})
	pool := NewPool(conn)

	pub := EnsurePublisher{pool}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := pub.Publish(ctx, "test", "test", false, false, amqp.Publishing{})
	assert.Error(t, err)
	assert.Equal(t, errors.Cause(err), ctx.Err())
}

func TestRetryPublisherRespectContext(t *testing.T) {
	defer time.AfterFunc(1*time.Second, func() { panic("RetryPublisher don't respect context") }).Stop()

	conn := NewConnector(Config{})
	pool := NewPool(conn)
	ensurePub := NewEnsurePublisher(pool)
	pub := NewRetryPublisher(ensurePub)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := pub.Publish(ctx, "test", "test", false, false, amqp.Publishing{})
	assert.Error(t, err)
	assert.Equal(t, errors.Cause(err), ctx.Err())
}

func TestRetryPublisherDelaySetup(t *testing.T) {
	conn := NewConnector(Config{})
	pool := NewPool(conn)
	ensurePub := NewEnsurePublisher(pool)

	expected := 999 * time.Millisecond
	pub := NewRetryPublisherWithDelay(ensurePub, expected)
	assert.Equal(t, expected, pub.delay)
}
