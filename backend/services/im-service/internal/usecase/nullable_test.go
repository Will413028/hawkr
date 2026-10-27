package usecase

import (
	"testing"
	"time"
)

func TestTimeValue(t *testing.T) {
	if got := timeValue(nil); !got.IsZero() {
		t.Fatalf("timeValue(nil) = %v, want zero time", got)
	}

	want := time.Date(2026, time.August, 26, 12, 34, 56, 0, time.UTC)
	if got := timeValue(&want); !got.Equal(want) {
		t.Fatalf("timeValue(%v) = %v, want original time", want, got)
	}
}
