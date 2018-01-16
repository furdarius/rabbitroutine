# 0.3.0
- Rename Attempts to ReconnectAttempts in Connector config
- Make zero value of ReconnectAttempts equal infinity.

# 0.2.1
- On error once close amqp channel.

# 0.2.0
- Dial and DialConfig were added. DialConfig used to configure RabbitMQ connection settings.
- Config stores only reconnect options.
- Start replaced with Dial.