package rabbitroutine

import (
	"context"
	"testing"
	"time"

	"github.com/pkg/errors"
	"github.com/streadway/amqp"
	"github.com/stretchr/testify/assert"
)

func TestEnsurePublisherRespectContext(t *testing.T) {
	defer time.AfterFunc(1*time.Second, func() { panic("EnsurePublisher don't respect context") }).Stop()

	conn := NewConnector(Config{})
	pool := NewPool(conn)

	pub := EnsurePublisher{pool}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := pub.Publish(ctx, "test", "test", amqp.Publishing{})
	assert.Error(t, err)
	assert.Equal(t, errors.Cause(err), ctx.Err())
}

func TestRetryPublisherRespectContext(t *testing.T) {
	defer time.AfterFunc(1*time.Second, func() { panic("RetryPublisher don't respect context") }).Stop()

	conn := NewConnector(Config{})
	pool := NewPool(conn)

	pub := RetryPublisher{&EnsurePublisher{pool}}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := pub.Publish(ctx, "test", "test", amqp.Publishing{})
	assert.Error(t, err)
	assert.Equal(t, errors.Cause(err), ctx.Err())
}

