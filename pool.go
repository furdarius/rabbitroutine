package rabbitroutine

import (
	"context"
	"sync"

	"github.com/pkg/errors"
	"github.com/streadway/amqp"
)

// ChannelKeeper store AMQP Channel with Confirmation and Close chans.
type ChannelKeeper struct {
	msgCh     *amqp.Channel
	errorCh   chan *amqp.Error
	confirmCh chan amqp.Confirmation
}

// Channel returns a amqp.Channel stored in ChannelKeeper.
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

// Pool is a set of AMQP Channels that may be individually saved and retrieved.
type Pool struct {
	conn *Connector
	mx   sync.Mutex
	set  []ChannelKeeper
}

// NewPool return new instance of Pool.
func NewPool(conn *Connector) *Pool {
	return &Pool{
		conn: conn,
	}
}

// ChannelWithConfirm return ChannelKeeper with AMQP Channel into confirm mode.
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

// new return ChannelKeeper with new amqp.Channel into confirm mode.
func (p *Pool) new(ctx context.Context) (ChannelKeeper, error) {
	var keep ChannelKeeper

	ch, err := p.conn.Channel(ctx)
	if err != nil {
		return keep, errors.Wrap(err, "failed to receive channel")
	}

	closeCh := ch.NotifyClose(make(chan *amqp.Error, 1))

	err = ch.Confirm(false)
	if err != nil {
		_ = ch.Close()

		return keep, errors.Wrap(err, "failed to setup confirm mode for channel")
	}

	publishCh := ch.NotifyPublish(make(chan amqp.Confirmation, 1))

	return ChannelKeeper{ch, closeCh, publishCh}, nil
}
