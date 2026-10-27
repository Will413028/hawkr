package membership

import (
	"testing"
	"time"
)

func TestPeriodClampAndBoundary(t *testing.T) {
	anchor := time.Date(2024, 1, 31, 12, 0, 0, 0, time.UTC)
	for _, tt := range []struct{ now, start, end string }{{"2024-02-01T12:00:00Z", "2024-01-31T12:00:00Z", "2024-02-29T12:00:00Z"}, {"2024-02-29T12:00:00Z", "2024-02-29T12:00:00Z", "2024-03-29T12:00:00Z"}, {"2024-03-31T12:00:00Z", "2024-03-29T12:00:00Z", "2024-04-29T12:00:00Z"}, {"2025-03-31T12:00:00Z", "2025-03-28T12:00:00Z", "2025-04-28T12:00:00Z"}} {
		now, err := time.Parse(time.RFC3339, tt.now)
		if err != nil {
			t.Fatal(err)
		}
		start, end := Period(anchor, now)
		if start.Format(time.RFC3339) != tt.start || end.Format(time.RFC3339) != tt.end {
			t.Fatalf("now=%s got [%s,%s)", tt.now, start, end)
		}
	}
}
