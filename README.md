<p align="center"><img src="https://habrastorage.org/webt/59/e2/71/59e271948a792190098780.png"></p>

[![GoDoc](https://godoc.org/github.com/furdarius/rabbitroutine?status.svg)](https://godoc.org/github.com/furdarius/rabbitroutine)
[![Build Status](https://travis-ci.org/furdarius/rabbitroutine.svg?branch=master)](https://travis-ci.org/furdarius/rabbitroutine)
[![Go Report Card](https://goreportcard.com/badge/github.com/furdarius/rabbitroutine)](https://goreportcard.com/report/github.com/furdarius/rabbitroutine)

# Rabbitmq Failover Routine For You

This is small library, that do RabbitMQ auto reconnect and publish retry routine for you.

## Install
```
go get github.com/furdarius/rabbitroutine
```

### Adding as dependency by "go dep"
```
$ dep ensure -add github.com/furdarius/rabbitroutine
```

## Usage


### Consuming
You need to implement [Consumer](https://godoc.org/github.com/furdarius/rabbitroutine#Consumer) and register
it with [StartConsumer](https://godoc.org/github.com/furdarius/rabbitroutine#Connector.StartConsumer)
or with [StartMultipleConsumers](https://godoc.org/github.com/furdarius/rabbitroutine#Connector.StartMultipleConsumers).
When connection is established (*at first time or after reconnect*) `Declare` method is called. It can be used to
declare required RabbitMQ entities ([consumer example](https://github.com/furdarius/rabbitroutine/blob/master/consumer_example_test.go)). 


Usage example:

```go

// Consumer declare your own RabbitMQ consumer implement rabbitroutine.Consumer interface.
type Consumer struct {}
func (c *Consumer) Declare(ctx context.Context, ch *amqp.Channel) error {}
func (c *Consumer) Consume(ctx context.Context, ch *amqp.Channel) error {}

conn := rabbitroutine.NewConnector(rabbitroutine.Config{
    Host:     "127.0.0.1",
    Port:     5672,
    Username: "guest",
    Password: "guest",
    // Max reconnect attempts
    Attempts: 20,
    // How long wait between reconnect
    Wait: 2 * time.Second,
})

ctx := context.Background()

go func() {
    err := conn.Start(ctx)
    if err != nil {
    	log.Println(err)
    }
}()

consumer := &Consumer{}
go func() {
    err := conn.StartConsumer(ctx, consumer)
    if err != nil {
        log.Println(err)
    }
}()
```

[Full example demonstrates messages consuming](https://github.com/furdarius/rabbitroutine/blob/master/consumer_example_test.go)


### Publising

For publishing [RetryPublisher](https://godoc.org/github.com/furdarius/rabbitroutine#RetryPublisher)
or [EnsurePublisher](https://godoc.org/github.com/furdarius/rabbitroutine#EnsurePublisher) implemented.
Both wait before receiving server approve that message was successfully delivered.
Or error `RetryPublisher` retry to publish message.

Usage example:
```go
ctx := context.Background()

conn := rabbitroutine.NewConnector(rabbitroutine.Config{
    Host:     "127.0.0.1",
    Port:     5672,
    Username: "guest",
    Password: "guest",
    // Max reconnect attempts
    Attempts: 20,
    // How long wait between reconnect
    Wait: 2 * time.Second,
})

pool := rabbitroutine.NewPool(conn)
ensurePub := rabbitroutine.NewEnsurePublisher(pool)
pub := rabbitroutine.NewRetryPublisher(ensurePub)

go conn.Start(ctx)

err := pub.Publish(ctx, "myexch", "myqueue", amqp.Publishing{Body: []byte("message")})
if err != nil {
    log.Println("publish error:", err)
}

```

[Full example demonstrates messages publishing](https://github.com/furdarius/rabbitroutine/blob/master/publisher_example_test.go)

## Contributing

Pull requests are very much welcomed.  Create your pull request on a non-master
branch, make sure a test or example is included that covers your change and
your commits represent coherent changes that include a reason for the change.

To run the integration tests, make sure you have RabbitMQ running on any host,
export the environment variable `AMQP_URL=amqp://host/` and run `go test -tags
integration`. As example:
```
AMQP_URL=amqp://guest:guest@127.0.0.1:5672/ go test -v -race -tags integration -timeout 2s
```

Use `gometalinter` to check code with linters:
```
gometalinter -t --vendor ./...
```

TravisCI will also run the integration tests and gometalinter.