package store

import (
	"errors"
	"fmt"
)

// ErrNotFound is returned when a requested resource does not exist.
var ErrNotFound = errors.New("not found")

// NotFoundError wraps ErrNotFound with contextual information.
type NotFoundError struct {
	Resource string
	ID       string
}

func (e *NotFoundError) Error() string {
	return fmt.Sprintf("%s not found: %s", e.Resource, e.ID)
}

func (e *NotFoundError) Unwrap() error { return ErrNotFound }

// NewNotFoundError creates a new NotFoundError.
func NewNotFoundError(resource, id string) error {
	return &NotFoundError{Resource: resource, ID: id}
}
