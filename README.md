<p align="center"><img src="https://habrastorage.org/webt/59/e2/71/59e271948a792190098780.png"></p>

[![GoDoc](https://godoc.org/github.com/furdarius/rabbitroutine?status.svg)](https://godoc.org/github.com/furdarius/rabbitroutine)

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

// Consumer declare your own RabbitMQ consumer
// implement rabbitroutine.Consumer interface.
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

pub := rabbitroutine.NewPublisher(conn)

ctx := context.Background()

go func() {
    err := conn.Start(ctx)
    if err != nil {
    	log.Println(err)
    }
}()

err := pub.EnsurePublish(ctx, "myexch", "myqueue", amqp.Publishing{
    Body: []byte("message"),
})

```

You can find more powerful examples in "[examples](https://github.com/furdarius/rabbitroutine/tree/master/examples)" directory.