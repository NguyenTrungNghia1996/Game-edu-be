package service

import (
	"errors"
	"fmt"
)

var (
	ErrValidation     = errors.New("validation error")
	ErrUnauthorized   = errors.New("unauthorized")
	ErrForbidden      = errors.New("forbidden")
	ErrNotFound       = errors.New("not found")
	ErrConflict       = errors.New("conflict")
	ErrAttemptExpired = errors.New("attempt expired or invalid")
)

func WrapErr(base error, msg string) error {
	return fmt.Errorf("%w: %s", base, msg)
}
