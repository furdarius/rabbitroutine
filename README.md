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

### Publising

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

You can find more powerful examples in "[examples](https://github.com/furdarius/rabbitroutine/tree/master/examples)" directory.

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