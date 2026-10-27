package domain

import "testing"

func TestIsTerminalRunStatus(t *testing.T) {
	cases := map[string]bool{
		RunStatusQueued():    false,
		RunStatusRunning():   false,
		RunStatusWaiting():   false,
		RunStatusCancelReq(): false,
		RunStatusSucceeded(): true,
		RunStatusFailed():    true,
		RunStatusCancelled(): true,
		RunStatusResumed():   true,
		RunStatusExpired():   true,
		"UNKNOWN":            false,
	}
	for status, want := range cases {
		if got := IsTerminalRunStatus(status); got != want {
			t.Errorf("IsTerminalRunStatus(%q) = %v, want %v", status, got, want)
		}
	}
}
