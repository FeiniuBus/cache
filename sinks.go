package cache

// A Sink receives data from a Get call.
type Sink interface {
	SetString(s string) error

	SetBytes(v []byte) error
}
