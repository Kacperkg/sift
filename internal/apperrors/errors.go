// Package apperrors defines application-wide error categories shared by the
// repository and service layers. It deliberately has no project dependencies.
package apperrors

import "errors"

var (
	ErrNotFound     = errors.New("not found")
	ErrConflict     = errors.New("conflict")
	ErrUnauthorized = errors.New("unauthorized")
	ErrInvalidInput = errors.New("invalid input")
)
