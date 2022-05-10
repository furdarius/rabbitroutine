package darkmq

import (
	"context"
	"testing"
	"time"

	"github.com/pkg/errors"
	"github.com/streadway/amqp"
	"github.com/stretchr/testify/assert"
)

func TestContextDoneIsCorrectAndNotBlocking(t *testing.T) {
	defer time.AfterFunc(1*time.Second, func() { panic("contextDone deadlock") }).Stop()

	tests := []struct {
		ctx      func() context.Context
		expected bool
	}{
		{
			func() context.Context {
				return context.Background()
			},
			false,
		},
		{
			func() context.Context {
				ctx, cancel := context.WithCancel(context.Background())
				cancel()
				return ctx
			},
			true,
		},
	}

	for _, test := range tests {
		actual := contextDone(test.ctx())

		assert.Equal(t, test.expected, actual)
	}
}

func TestDialIsBlocking(t *testing.T) {
	conn := NewConnector(Config{
		ReconnectAttempts: 10,
		Wait:              10 * time.Second,
	})

	go func() {
		_ = conn.Dial(context.Background(), "")

		panic("Dial is not blocking")
	}()

	<-time.After(5 * time.Millisecond)
}

func TestDialReturnErrorOnFailedReconnect(t *testing.T) {
	conn := NewConnector(Config{
		ReconnectAttempts: 1,
		Wait:              time.Millisecond,
	})

	err := conn.Dial(context.Background(), "")
	assert.Error(t, err)
}

func TestDialRespectContext(t *testing.T) {
	conn := NewConnector(Config{
		ReconnectAttempts: 100,
		Wait:              5 * time.Minute,
	})

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := conn.Dial(ctx, "")
	assert.Error(t, err)
	assert.Equal(t, errors.Cause(err), ctx.Err())
}

func TestConnBroadcastRespectContext(t *testing.T) {
	defer time.AfterFunc(1*time.Second, func() { panic("connBroadcast don't respect context") }).Stop()

	conn := NewConnector(Config{})

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	conn.connBroadcast(ctx)
}

func TestDialWithItRespectContext(t *testing.T) {
	defer time.AfterFunc(1*time.Second, func() { panic("dialWithIt don't respect context") }).Stop()

	conn := NewConnector(Config{
		ReconnectAttempts: 100,
		Wait:              5 * time.Minute,
	})

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := conn.dialWithIt(ctx, "", amqp.Config{})
	assert.Error(t, err)
	assert.Equal(t, err, ctx.Err())
}

func TestDialWithItRetryFailed(t *testing.T) {
	defer time.AfterFunc(1*time.Second, func() { panic("dialWithIt don't respect context") }).Stop()

	conn := NewConnector(Config{
		ReconnectAttempts: 3,
		Wait:              1 * time.Millisecond,
	})

	err := conn.dialWithIt(context.Background(), "", amqp.Config{})
	assert.Error(t, err)
	assert.Nil(t, conn.conn)
}

func TestChannelRespectContext(t *testing.T) {
	defer time.AfterFunc(1*time.Second, func() { panic("Channel don't respect context") }).Stop()

	conn := NewConnector(Config{
		ReconnectAttempts: 100,
		Wait:              5 * time.Minute,
	})

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := conn.Channel(ctx)
	assert.Error(t, err)
	assert.Equal(t, err, ctx.Err())
}

func TestConnectorCreationWithEmptyConfig(t *testing.T) {
	conn := NewConnector(Config{})

	assert.NotEqual(t, 0, conn.cfg.ReconnectAttempts)
}
