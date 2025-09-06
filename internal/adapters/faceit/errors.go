package faceit

import "fmt"

var ErrNotFound = fmt.Errorf("not found")

type APIError struct {
	Status int
	Body   string
}

func (e *APIError) Error() string {
	return fmt.Sprintf("faceit api status %d: %s", e.Status, e.Body)
}
