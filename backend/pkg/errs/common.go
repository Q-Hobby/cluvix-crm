package errs

import "errors"

// LocalizedError carries an i18n key + optional format args.
// Services return this instead of fmt.Errorf with hardcoded strings.
// HandleError detects the type and calls i18n.TF(c, key, args...).
type LocalizedError struct {
	Key     string
	Args    []interface{}
	wrapped error // optional: allows errors.Is against a sentinel
}

func (e *LocalizedError) Error() string { return e.Key }
func (e *LocalizedError) Unwrap() error { return e.wrapped }

func NewLocalizedError(key string, args ...interface{}) error {
	return &LocalizedError{Key: key, Args: args}
}

// NewLocalizedErrorWrapping creates a LocalizedError that also wraps a sentinel error,
// so errors.Is(err, sentinel) returns true while the i18n message carries extra context.
func NewLocalizedErrorWrapping(key string, sentinel error, args ...interface{}) error {
	return &LocalizedError{Key: key, Args: args, wrapped: sentinel}
}

var ErrVersionConflict = errors.New("version_conflict")
var ErrNotFound = errors.New("not_found")
var ErrConcurrentBusy = errors.New("concurrent_request_busy")
