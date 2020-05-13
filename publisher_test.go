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
	attempts               []interface{}
	retryAttemptContextKey string
}

func (p *retryAttemptCapturePublisher) Publish(ctx context.Context, exchange, key string, msg amqp.Publishing) error {
	p.attempts = append(p.attempts, ctx.Value(p.retryAttemptContextKey))
	return errors.New("retryAttemptCapturePublisher.Publish always returns an error")
}

func TestRetryPublisherRetryAttemptContextKeySetup(t *testing.T) {
	conn := NewConnector(Config{})
	pool := NewPool(conn)
	ensurePub := NewEnsurePublisher(pool)

	expected := "retry-attempt"

	pub := NewRetryPublisher(ensurePub, PublishRetryAttemptContextKeySetup(expected))

	actual := pub.retryAttemptContextKey
	assert.Equal(t, expected, actual)
}

func TestRetryPublisherPassesAttemptWhenContextKeySet(t *testing.T) {
	retries := 3
	retryAttemptContextKey := "retry-attempt"
	retryAttemptCapturePub := &retryAttemptCapturePublisher{
		retryAttemptContextKey: retryAttemptContextKey,
	}

	pub := NewRetryPublisher(retryAttemptCapturePub, PublishRetryAttemptContextKeySetup(retryAttemptContextKey), PublishMaxAttemptsSetup(uint(retries)))

	ctx := context.Background()
	err := pub.Publish(ctx, "test", "test", amqp.Publishing{})
	assert.Error(t, err, "retryAttemptCapturePublisher should always return an error")

	expected := []uint{1, 2, 3}
	assert.ElementsMatch(t, expected, retryAttemptCapturePub.attempts)
}

func TestRetryPublisherDoesntPassAttemptWhenContextKeyNotSet(t *testing.T) {
	retries := 3
	retryAttemptCapturePub := &retryAttemptCapturePublisher{}

	pub := NewRetryPublisher(retryAttemptCapturePub, PublishMaxAttemptsSetup(uint(retries)))

	ctx := context.Background()
	err := pub.Publish(ctx, "test", "test", amqp.Publishing{})
	assert.Error(t, err, "retryAttemptCapturePublisher should always return an error")

	expected := []interface{}{nil, nil, nil}
	assert.ElementsMatch(t, expected, retryAttemptCapturePub.attempts)
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
