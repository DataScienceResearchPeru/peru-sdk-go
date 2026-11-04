package sunat

import "fmt"

// Error exposes stable categories without including credentials or remote bodies.
type Error struct {
	Kind       string
	Operation  string
	HTTPStatus int
	Code       string
}

func (e *Error) Error() string {
	return fmt.Sprintf("%s: %s (HTTP %d)", e.Operation, e.Kind, e.HTTPStatus)
}
