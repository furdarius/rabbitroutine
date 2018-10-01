# 0.4.1 | [Pull request](https://github.com/furdarius/rabbitroutine/pull/6)
- Data race fixed on c.conn when connection error occurs.

# 0.4.0 | [Pull request](https://github.com/furdarius/rabbitroutine/pull/3)
- [LightningPool](https://godoc.org/github.com/furdarius/rabbitroutine#LightningPool) added.
- [FireForgetPublisher](https://godoc.org/github.com/furdarius/rabbitroutine#FireForgetPublisher) added.
- Possible deadlock on channel receiving from Dialed event listener fixed.
- [RetryPublisher](https://godoc.org/github.com/furdarius/rabbitroutine#RetryPublisher) accepts [Publisher](https://godoc.org/github.com/furdarius/rabbitroutine#Publisher) interface now.

# 0.3.1
- On error wait for c.cfg.Wait time before consumer restart.

# 0.3.0
- Rename Attempts to ReconnectAttempts in Connector config
- Make zero value of ReconnectAttempts equal infinity.
- Return error from ChannelKeeper Close method.

# 0.2.1
- On error once close amqp channel.

# 0.2.0
- Dial and DialConfig were added. DialConfig used to configure RabbitMQ connection settings.
- Config stores only reconnect options.
- Start replaced with Dial.