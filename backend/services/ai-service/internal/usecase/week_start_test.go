package usecase

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestWeekStartFor_TaipeiBoundary(t *testing.T) {
	tpe, err := time.LoadLocation("Asia/Taipei")
	require.NoError(t, err)

	// Taipei Monday 00:30 → that Monday's date (was previous week under UTC).
	mon := time.Date(2026, 7, 6, 0, 30, 0, 0, tpe)
	require.Equal(t, "2026-07-06", weekStartFor(mon))

	// Taipei Sunday 23:00 → still the PREVIOUS Monday (2026-06-29..07-05 week).
	sun := time.Date(2026, 7, 5, 23, 0, 0, 0, tpe)
	require.Equal(t, "2026-06-29", weekStartFor(sun))
}
