package rabbitroutine

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/streadway/amqp"
	"github.com/stretchr/testify/assert"
)

var testCfg = integrationConfig()

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
		{
			// nolint: vet
			func() context.Context {
				ctx, _ := context.WithTimeout(context.Background(), 10*time.Second)
				return ctx
			},
			false,
		},
	}

	for _, test := range tests {
		actual := contextDone(test.ctx())

		assert.Equal(t, test.expected, actual)
	}
}

func TestStartIsBlocking(t *testing.T) {
	conn := New(testCfg)

	// nolint: errcheck
	go func() {
		conn.Start(context.Background())

		panic("Start is not blocking")
	}()

	<-time.After(5 * time.Millisecond)
}

func TestStartReturnErrorOnFailedReconnect(t *testing.T) {
	conn := New(Config{
		Attempts: 1,
		Wait:     time.Millisecond,
	})

	err := conn.Start(context.Background())
	assert.Error(t, err)
}

func integrationURLFromEnv() string {
	url := os.Getenv("AMQP_URL")
	if url == "" {
		url = "amqp://"
	}

	return url
}

func integrationConfig() Config {
	uri, err := amqp.ParseURI(integrationURLFromEnv())
	if err != nil {
		panic("failed to parse AMQP_URL")
	}

	return Config{
		Host:     uri.Host,
		Port:     uri.Port,
		Username: uri.Username,
		Password: uri.Password,
		Attempts: 20000,
		Wait:     5 * time.Second,
	}
}
