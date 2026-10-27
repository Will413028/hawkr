package mq

import (
	"errors"
	"fmt"
)

var ErrPermanent = errors.New("permanent failure")

const (
	failureClassPermanent      = "permanent"
	failureClassRetryExhausted = "retry_exhausted"
)

type permanentError struct {
	reason string
	err    error
}

func (e *permanentError) Error() string {
	if e.err == nil {
		return fmt.Sprintf("permanent failure (%s)", e.reason)
	}
	return fmt.Sprintf("permanent failure (%s): %v", e.reason, e.err)
}

func (e *permanentError) Unwrap() error { return e.err }

func (e *permanentError) Is(target error) bool { return target == ErrPermanent }

func Permanent(reason string, err error) error {
	return &permanentError{reason: reason, err: err}
}

func permanentReasonOf(err error) (string, bool) {
	var p *permanentError
	if errors.As(err, &p) {
		return p.reason, true
	}
	return "", false
}
