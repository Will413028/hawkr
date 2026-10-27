// Package fake is an in-memory mail.Sender for tests.
package fake

import (
	"context"
	"sync"

	"github.com/Will413028/hawkr/go-pkg/mail"
)

// Sender records every message it is handed and returns canned results.
type Sender struct {
	mu sync.Mutex

	// Sent records every call, including ones that returned Err — a test
	// asserting "we tried to send X and it failed" needs the attempt.
	Sent []mail.Message

	// Err, when non-nil, is returned by every Send.
	Err error

	// Receipt is returned when Err is nil.
	Receipt mail.Receipt
}

func (s *Sender) Send(_ context.Context, m mail.Message) (mail.Receipt, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Sent = append(s.Sent, m)
	if s.Err != nil {
		return mail.Receipt{}, s.Err
	}
	return s.Receipt, nil
}

// Reset clears recorded calls between sub-tests.
func (s *Sender) Reset() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Sent = nil
}
