package rabbitroutine

import (
	"fmt"
	"time"
)

// Config store all data required to dial with rabbitmq.
type Config struct {
	Host        string
	Port        int
	Username    string
	Password    string
	VirtualHost string
	// Max reconnect attempts.
	Attempts int
	// How long wait between reconnect.
	Wait time.Duration
}

// URL return a string in the AMQP URI format.
func (c *Config) URL() string {
	return fmt.Sprintf(
		"amqp://%s:%s@%s:%d/%s",
		c.Username,
		c.Password,
		c.Host,
		c.Port,
		c.VirtualHost,
	)
}
