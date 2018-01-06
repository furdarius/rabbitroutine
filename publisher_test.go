package rabbitroutine

import (
	"context"
	"testing"
	"time"

	"github.com/pkg/errors"
	"github.com/streadway/amqp"
	"github.com/stretchr/testify/assert"
)

func TestEnsurePublishRespectContext(t *testing.T) {
	defer time.AfterFunc(1*time.Second, func() { panic("EnsurePublish don't respect context") }).Stop()

	conn := NewConnector(Config{})
	pool := NewPool(conn)

	pub := Publisher{pool}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := pub.EnsurePublish(ctx, "test", "test", amqp.Publishing{})
	assert.Error(t, err)
	assert.Equal(t, errors.Cause(err), ctx.Err())
}
