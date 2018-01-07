package rabbitroutine

import (
	"context"
	"testing"
	"time"

	"github.com/pkg/errors"
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

func TestStartIsBlocking(t *testing.T) {
	conn := NewConnector(Config{
		Attempts: 10,
		Wait:     10 * time.Second,
	})

	// nolint: errcheck
	go func() {
		conn.Start(context.Background())

		panic("Start is not blocking")
	}()

	<-time.After(5 * time.Millisecond)
}

func TestStartReturnErrorOnFailedReconnect(t *testing.T) {
	conn := NewConnector(Config{
		Attempts: 1,
		Wait:     time.Millisecond,
	})

	err := conn.Start(context.Background())
	assert.Error(t, err)
}

func TestStartRespectContext(t *testing.T) {
	conn := NewConnector(Config{
		Attempts: 100,
		Wait:     5 * time.Minute,
	})

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := conn.Start(ctx)
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

func TestDialRespectContext(t *testing.T) {
	defer time.AfterFunc(1*time.Second, func() { panic("dial don't respect context") }).Stop()

	conn := NewConnector(Config{
		Attempts: 100,
		Wait:     5 * time.Minute,
	})

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := conn.dial(ctx)
	assert.Error(t, err)
	assert.Equal(t, err, ctx.Err())
}

func TestDialRetryFailed(t *testing.T) {
	defer time.AfterFunc(1*time.Second, func() { panic("dial don't respect context") }).Stop()

	conn := NewConnector(Config{
		Attempts: 3,
		Wait:     1 * time.Millisecond,
	})

	err := conn.dial(context.Background())
	assert.Error(t, err)
	assert.Nil(t, conn.conn)
}

func TestChannelRespectContext(t *testing.T) {
	defer time.AfterFunc(1*time.Second, func() { panic("Channel don't respect context") }).Stop()

	conn := NewConnector(Config{
		Attempts: 100,
		Wait:     5 * time.Minute,
	})

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := conn.Channel(ctx)
	assert.Error(t, err)
	assert.Equal(t, err, ctx.Err())
}
