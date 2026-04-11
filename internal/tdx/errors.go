package tdx

import (
	"errors"
	"fmt"
)

// ErrUnauthorized is returned when TD responds with 401.
var ErrUnauthorized = errors.New("tdx: unauthorized")

// APIError wraps non-2xx responses with structured detail.
type APIError struct {
	Status  int
	Message string
}

func (e *APIError) Error() string {
	return fmt.Sprintf("tdx api: %d: %s", e.Status, e.Message)
}
