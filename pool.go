package rabbitroutine

import (
	"context"
	"sync"

	"github.com/pkg/errors"
	amqp "github.com/rabbitmq/amqp091-go"
)

// ChannelKeeper stores AMQP Channel with Confirmation and Close chans.
type ChannelKeeper struct {
	msgCh     *amqp.Channel
	errorCh   chan *amqp.Error
	confirmCh chan amqp.Confirmation
	returnCh  chan amqp.Return
}

// Channel returns an amqp.Channel stored in ChannelKeeper.
func (k *ChannelKeeper) Channel() *amqp.Channel {
	return k.msgCh
}

// Error returns a channel that will receive amqp.Error when it occurs.
func (k *ChannelKeeper) Error() <-chan *amqp.Error {
	return k.errorCh
}

// Confirm returns a channel that will receive amqp.Confirmation when it occurs.
func (k *ChannelKeeper) Confirm() <-chan amqp.Confirmation {
	return k.confirmCh
}

// Return returns a channel that will receive amqp.Return when it occurs.
func (k *ChannelKeeper) Return() <-chan amqp.Return {
	return k.returnCh
}

// Close closes RabbitMQ channel stored in ChannelKeeper.
func (k *ChannelKeeper) Close() error {
	return k.msgCh.Close()
}

// Pool is a set of AMQP Channels that may be individually saved and retrieved.
type Pool struct {
	conn *Connector
	mx   sync.Mutex
	set  []ChannelKeeper
}

// NewPool returns a new instance of Pool.
func NewPool(conn *Connector) *Pool {
	return &Pool{
		conn: conn,
	}
}

// ChannelWithConfirm returns a ChannelKeeper with AMQP Channel into confirm mode.
func (p *Pool) ChannelWithConfirm(ctx context.Context) (ChannelKeeper, error) {
	var (
		k   ChannelKeeper
		err error
	)

	p.mx.Lock()
	last := len(p.set) - 1
	if last >= 0 {
		k = p.set[last]
		p.set = p.set[:last]
	}
	p.mx.Unlock()

	// If pool is empty create new channel
	if last < 0 {
		k, err = p.new(ctx)
		if err != nil {
			return k, errors.Wrap(err, "failed to create new")
		}
	}

	return k, nil
}

// Release adds k to the pool.
func (p *Pool) Release(k ChannelKeeper) {
	p.mx.Lock()
	p.set = append(p.set, k)
	p.mx.Unlock()
}

// new returns a ChannelKeeper with new amqp.Channel into confirm mode.
func (p *Pool) new(ctx context.Context) (ChannelKeeper, error) {
	ch, err := p.conn.Channel(ctx)
	if err != nil {
		return ChannelKeeper{}, errors.Wrap(err, "failed to receive channel from connection")
	}

	errorCh := ch.NotifyClose(make(chan *amqp.Error, 1))

	err = ch.Confirm(false)
	if err != nil {
		_ = ch.Close() //nolint: gosec

		return ChannelKeeper{}, errors.Wrap(err, "failed to setup confirm mode for channel")
	}

	confirmCh := ch.NotifyPublish(make(chan amqp.Confirmation, 1))

	returnCh := ch.NotifyReturn(make(chan amqp.Return, 1))

	return ChannelKeeper{ch, errorCh, confirmCh, returnCh}, nil
}

// Size returns current pool size.
// note: not thread-safe operation.
func (p *Pool) Size() int {
	return len(p.set)
}

// LightningPool stores AMQP Channels without confirm mode, so they will be used without delivery guarantees.
type LightningPool struct {
	conn *Connector
	mx   sync.Mutex
	set  []*amqp.Channel
}

// NewLightningPool return a new instance of LightningPool.
func NewLightningPool(conn *Connector) *LightningPool {
	return &LightningPool{
		conn: conn,
	}
}

// Channel return AMQP Channel.
func (p *LightningPool) Channel(ctx context.Context) (*amqp.Channel, error) {
	var (
		k   *amqp.Channel
		err error
	)

	p.mx.Lock()
	last := len(p.set) - 1
	if last >= 0 {
		k = p.set[last]
		p.set = p.set[:last]
	}
	p.mx.Unlock()

	// If pool is empty create new channel
	if last < 0 {
		k, err = p.new(ctx)
		if err != nil {
			return k, errors.Wrap(err, "failed to create new")
		}
	}

	return k, nil
}

// Release adds k to the pool.
func (p *LightningPool) Release(k *amqp.Channel) {
	p.mx.Lock()
	p.set = append(p.set, k)
	p.mx.Unlock()
}

// new returns new amqp.Channel.
func (p *LightningPool) new(ctx context.Context) (*amqp.Channel, error) {
	return p.conn.Channel(ctx)
}

// Size returns current pool size.
// note: not thread-safe operation.
func (p *LightningPool) Size() int {
	return len(p.set)
}
