<p align="center"><img src="logo.jpg"></p>

[![PkgGoDev](https://pkg.go.dev/badge/github.com/furdarius/darkmq)](https://pkg.go.dev/github.com/furdarius/darkmq)
[![Build Status](https://travis-ci.org/furdarius/darkmq.svg?branch=master)](https://travis-ci.org/furdarius/darkmq)
[![Go Report Card](https://goreportcard.com/badge/github.com/furdarius/darkmq)](https://goreportcard.com/report/github.com/furdarius/darkmq)

# Rabbitmq Failover Routine

Lightweight library that handles RabbitMQ auto-reconnect and publishing retry routine for you.
The library is designed to save the developer from the headache when working with RabbitMQ.

**darkmq** solves your RabbitMQ reconnection problems:
* Handles connection errors and channels errors separately.
* Takes into account the need to [re-declare](https://godoc.org/github.com/furdarius/darkmq#Consumer) entities in RabbitMQ after reconnection.
* Notifies of [errors](https://godoc.org/github.com/furdarius/darkmq#Connector.AddAMQPNotifiedListener) and connection [retry attempts](https://godoc.org/github.com/furdarius/darkmq#Connector.AddRetriedListener).
* Supports [FireAndForgetPublisher](https://godoc.org/github.com/furdarius/darkmq#FireForgetPublisher) and [EnsurePublisher](https://godoc.org/github.com/furdarius/darkmq#EnsurePublisher), that can be wrapped with [RetryPublisher](https://godoc.org/github.com/furdarius/darkmq#RetryPublisher).
* Supports pool of channels used for publishing.
* Provides channels [pool size](https://godoc.org/github.com/furdarius/darkmq#Pool.Size) statistics.

**Stop to do wrappers, do features!**

## Install
```
go get github.com/sagleft/darkrmq
```

### Adding as dependency by "go dep"
```
$ dep ensure -add github.com/sagleft/darkrmq
```

## Usage


### Consuming
You need to implement [Consumer](https://godoc.org/github.com/furdarius/darkmq#Consumer) and register
it with [StartConsumer](https://godoc.org/github.com/furdarius/darkmq#Connector.StartConsumer)
or with [StartMultipleConsumers](https://godoc.org/github.com/furdarius/darkmq#Connector.StartMultipleConsumers).
When connection is established (*at first time or after reconnect*) `Declare` method is called. It can be used to
declare required RabbitMQ entities ([consumer example](https://github.com/furdarius/darkmq/blob/master/consumer_example_test.go)). 


Usage example:

```go

// Consumer declares your own RabbitMQ consumer implementing darkmq.Consumer interface.
type Consumer struct {}
func (c *Consumer) Declare(ctx context.Context, ch *amqp.Channel) error {}
func (c *Consumer) Consume(ctx context.Context, ch *amqp.Channel) error {}

url := "amqp://guest:guest@127.0.0.1:5672/"

conn := darkmq.NewConnector(darkmq.Config{
    // How long to wait between reconnect
    Wait: 2 * time.Second,
})

ctx := context.Background()

go func() {
    err := conn.Dial(ctx, url)
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

[Full example demonstrates messages consuming](https://github.com/furdarius/darkmq/blob/master/consumer_example_test.go)


### Publishing

For publishing [FireForgetPublisher](https://godoc.org/github.com/furdarius/darkmq#FireForgetPublisher)
and [EnsurePublisher](https://godoc.org/github.com/furdarius/darkmq#EnsurePublisher) implemented.
Both of them can be wrapped with [RetryPublisher](https://godoc.org/github.com/furdarius/darkmq#RetryPublisher)
to repeat publishing on errors and mitigate short-term network problems.

Usage example:
```go
ctx := context.Background()

url := "amqp://guest:guest@127.0.0.1:5672/"

conn := darkmq.NewConnector(darkmq.Config{
    // How long wait between reconnect
    Wait: 2 * time.Second,
})

pool := darkmq.NewPool(conn)
ensurePub := darkmq.NewEnsurePublisher(pool)
pub := darkmq.NewRetryPublisher(
    ensurePub,
    darkmq.PublishMaxAttemptsSetup(16),
    darkmq.PublishDelaySetup(darkmq.LinearDelay(10*time.Millisecond)),
)

go conn.Dial(ctx, url)

err := pub.Publish(ctx, "myexch", "myqueue", amqp.Publishing{Body: []byte("message")})
if err != nil {
    log.Println("publish error:", err)
}

```

[Full example demonstrates messages publishing](https://github.com/furdarius/darkmq/blob/master/publisher_example_test.go)

## Contributing

Pull requests are very much welcomed.  Create your pull request, make sure a test or example is included that covers your change and
your commits represent coherent changes that include a reason for the change.

To run the integration tests, make sure you have RabbitMQ running on any host (e.g with `docker run --net=host -it --rm rabbitmq`), then
export the environment variable `AMQP_URL=amqp://host/` and run `go test -tags
integration`. As example:
```
AMQP_URL=amqp://guest:guest@127.0.0.1:5672/ go test -v -race -cpu=1,2 -tags integration -timeout 5s
```

Use [golangci-lint](https://github.com/golangci/golangci-lint) to check code with linters:
```
golangci-lint run ./...
```

TravisCI will also run the integration tests and golangci-lint.
