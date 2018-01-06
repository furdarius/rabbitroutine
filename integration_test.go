// +build integration

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

func TestIntegrationPublishing(t *testing.T) {
	ctx := context.Background()

	conn := NewConnector(testCfg)

	go func() {
		err := conn.Start(ctx)
		if err != nil {
			panic("failed to start")
		}
	}()

	ch, err := conn.Channel(ctx)
	assert.NoError(t, err, "failed to receive channel")
	assert.NotNil(t, ch, "nil channel received")
	defer ch.Close()

	exchange := "test-integration-exchange"
	queue := "test-integration-queue"
	msg := "test message"

	err = ch.ExchangeDeclare(
		exchange, // name
		"direct", // type
		false,    // durable
		true,     // auto-delete
		false,    // internal
		false,    // nowait
		nil,      // args
	)
	assert.NoError(t, err, "failed to declare exchange")

	pool := NewPool(conn)
	p := NewPublisher(pool)
	err = p.EnsurePublish(ctx, exchange, queue, amqp.Publishing{Body: []byte(msg)})
	assert.NoError(t, err, "failed to publish")
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
