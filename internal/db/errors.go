package db

import "errors"

const (
	ErrCodeNotFound      = "NOT_FOUND"
	ErrCodeInvalidParent = "INVALID_PARENT"
	ErrCodeTaskBlocked   = "TASK_BLOCKED"
	ErrCodeInvalidState  = "INVALID_STATE"
	ErrCodeDuplicateDep  = "DUPLICATE_DEP"
	ErrCodeCycleDetected = "CYCLE_DETECTED"
	ErrCodeSessionStale  = "SESSION_STALE"
	ErrCodeInvalidRef    = "INVALID_REF"
)

type CodedError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func (e *CodedError) Error() string {
	return e.Message
}

func codedError(code, msg string) error {
	return &CodedError{Code: code, Message: msg}
}

func AsCodedError(err error) *CodedError {
	var ce *CodedError
	if errors.As(err, &ce) {
		return ce
	}
	return nil
}
