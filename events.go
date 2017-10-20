package rabbitroutine

// Retried is fired when connection retrying occurs.
// The event will be emitted only if the connection was not established.
// If connection was successfully established Dialed event emitted.
type Retried struct {
	Attempt int
	Error   error
}

// Dialed is fired when connection was successfully established.
type Dialed struct {}