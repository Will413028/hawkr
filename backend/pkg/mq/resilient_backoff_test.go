package mq

import (
	"testing"
	"time"
)

// reconnectBackoff is defined in resilient.go and, as of this change, also
// drives Publisher.watchReconnect's retry delay. It had no direct test
// coverage before; this backfills it so both reconnect loops share a
// verified backoff curve: 1s, 2s, 4s, 8s, 16s, capped at 30s.
func TestReconnectBackoff(t *testing.T) {
	cases := []struct {
		attempt int
		want    time.Duration
	}{
		{0, time.Second},
		{1, time.Second},
		{2, 2 * time.Second},
		{3, 4 * time.Second},
		{4, 8 * time.Second},
		{5, 16 * time.Second},
		{6, 30 * time.Second},
		{100, 30 * time.Second},
	}
	for _, c := range cases {
		if got := reconnectBackoff(c.attempt); got != c.want {
			t.Errorf("reconnectBackoff(%d) = %v, want %v", c.attempt, got, c.want)
		}
	}
}
