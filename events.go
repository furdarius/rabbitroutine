package rabbitroutine

// Retry is fired when connection retrying occurs.
type Retry struct {
	Attempt int
	Success bool
	Error   error
}
