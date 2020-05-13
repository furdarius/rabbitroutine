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

	err := pub.Publish(ctx, "test", "test", amqp.Publishing{})
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

	err := pub.Publish(ctx, "test", "test", amqp.Publishing{})
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

	err := pub.Publish(ctx, "test", "test", amqp.Publishing{})
	assert.Error(t, err)
	assert.Equal(t, errors.Cause(err), ctx.Err())
}

func TestRetryPublisherDelaySetup(t *testing.T) {
	conn := NewConnector(Config{})
	pool := NewPool(conn)
	ensurePub := NewEnsurePublisher(pool)

	expected := 42 * time.Millisecond

	pub := NewRetryPublisher(ensurePub, PublishDelaySetup(ConstDelay(expected)))

	actual := pub.delayFn(1)
	assert.Equal(t, expected, actual)
}

func TestRetryPublisherMaxAttemptsSetup(t *testing.T) {
	conn := NewConnector(Config{})
	pool := NewPool(conn)
	ensurePub := NewEnsurePublisher(pool)

	expected := uint(42)

	pub := NewRetryPublisher(ensurePub, PublishMaxAttemptsSetup(expected))

	actual := pub.maxAttempts
	assert.Equal(t, expected, actual)
}

type retryAttemptCapturePublisher struct {
	attempts []interface{}
}

func (p *retryAttemptCapturePublisher) Publish(ctx context.Context, exchange, key string, msg amqp.Publishing) error {
	retryAttempt, ok := RetryAttempt(ctx)
	if ok {
		p.attempts = append(p.attempts, retryAttempt)
	}
	return errors.New("retryAttemptCapturePublisher.Publish always returns an error")
}

func TestRetryPublisherPassesRetryAttemptInContext(t *testing.T) {
	retryAttemptCapturePub := &retryAttemptCapturePublisher{}

	pub := NewRetryPublisher(retryAttemptCapturePub, PublishMaxAttemptsSetup(3))

	ctx := context.Background()
	err := pub.Publish(ctx, "test", "test", amqp.Publishing{})
	assert.Error(t, err, "retryAttemptCapturePublisher should always return an error")

	expected := []uint{1, 2, 3}
	assert.ElementsMatch(t, expected, retryAttemptCapturePub.attempts)
}

func TestRetryAttempt(t *testing.T) {
	ctx := context.Background()
	_, ok := RetryAttempt(ctx)
	assert.False(t, ok)

	retryAttempt := uint(3)
	ctx = context.WithValue(ctx, retryAttemptContextKey, retryAttempt)
	actual, ok := RetryAttempt(ctx)
	assert.True(t, ok)
	assert.Equal(t, retryAttempt, actual)
}

func TestConstDelay(t *testing.T) {
	tests := []struct {
		delayFn  RetryDelayFunc
		attempt  uint
		expected time.Duration
	}{
		{
			delayFn:  ConstDelay(10 * time.Millisecond),
			attempt:  1,
			expected: 10 * time.Millisecond,
		},
		{
			delayFn:  ConstDelay(10 * time.Millisecond),
			attempt:  5,
			expected: 10 * time.Millisecond,
		},
		{
			delayFn:  ConstDelay(120 * time.Millisecond),
			attempt:  52,
			expected: 120 * time.Millisecond,
		},
		{
			delayFn:  ConstDelay(time.Second),
			attempt:  99,
			expected: time.Second,
		},
	}

	for _, test := range tests {
		actual := test.delayFn(test.attempt)
		assert.Equal(t, test.expected, actual)
	}
}

func TestLinearDelay(t *testing.T) {
	tests := []struct {
		delayFn  RetryDelayFunc
		attempt  uint
		expected time.Duration
	}{
		{
			delayFn:  LinearDelay(10 * time.Millisecond),
			attempt:  1,
			expected: 10 * time.Millisecond,
		},
		{
			delayFn:  LinearDelay(10 * time.Millisecond),
			attempt:  5,
			expected: 50 * time.Millisecond,
		},
		{
			delayFn:  LinearDelay(120 * time.Millisecond),
			attempt:  52,
			expected: 6240 * time.Millisecond,
		},
		{
			delayFn:  LinearDelay(time.Second),
			attempt:  99,
			expected: 99 * time.Second,
		},
	}

	for _, test := range tests {
		actual := test.delayFn(test.attempt)
		assert.Equal(t, test.expected, actual)
	}
}
