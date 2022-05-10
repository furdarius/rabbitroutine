package darkmq

import (
	"context"
	"testing"
	"time"

	"github.com/pkg/errors"
	"github.com/streadway/amqp"
	"github.com/stretchr/testify/assert"
)

func TestPoolChannelWithConfirmRespectContext(t *testing.T) {
	defer time.AfterFunc(1*time.Second, func() { panic("Pool.ChannelWithConfirm don't respect context") }).Stop()

	conn := NewConnector(Config{})
	pool := NewPool(conn)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := pool.ChannelWithConfirm(ctx)
	assert.Error(t, err)
	assert.Equal(t, errors.Cause(err), ctx.Err())
}

func TestPoolEmptyOnStart(t *testing.T) {
	conn := NewConnector(Config{})
	pool := NewPool(conn)

	assert.Empty(t, pool.set)
}

func TestPoolReleaseSuccess(t *testing.T) {
	conn := NewConnector(Config{})
	pool := NewPool(conn)

	pool.Release(ChannelKeeper{})
	assert.Len(t, pool.set, 1)
}

func TestPoolAcquiringFromNonEmpty(t *testing.T) {
	conn := NewConnector(Config{})
	pool := NewPool(conn)

	pool.Release(ChannelKeeper{
		errorCh:   make(chan *amqp.Error, 1),
		confirmCh: make(chan amqp.Confirmation, 1),
	})
	assert.Len(t, pool.set, 1)

	k, err := pool.ChannelWithConfirm(context.Background())
	assert.NoError(t, err)
	assert.Empty(t, pool.set)
	assert.NotNil(t, k.errorCh)
	assert.NotNil(t, k.confirmCh)
}

func TestLightningPoolChannelRespectContext(t *testing.T) {
	defer time.AfterFunc(1*time.Second, func() { panic("LightningPool.Channel don't respect context") }).Stop()

	conn := NewConnector(Config{})
	pool := NewLightningPool(conn)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := pool.Channel(ctx)
	assert.Error(t, err)
	assert.Equal(t, errors.Cause(err), ctx.Err())
}

func TestLightningPoolEmptyOnStart(t *testing.T) {
	conn := NewConnector(Config{})
	pool := NewLightningPool(conn)

	assert.Empty(t, pool.set)
}

func TestLightningPoolReleaseSuccess(t *testing.T) {
	conn := NewConnector(Config{})
	pool := NewLightningPool(conn)

	pool.Release(&amqp.Channel{})
	assert.Len(t, pool.set, 1)
}

func TestLightningPoolAcquiringFromNonEmpty(t *testing.T) {
	conn := NewConnector(Config{})
	pool := NewLightningPool(conn)

	pool.Release(&amqp.Channel{})
	assert.Len(t, pool.set, 1)

	_, err := pool.Channel(context.Background())
	assert.NoError(t, err)
	assert.Empty(t, pool.set)
}
