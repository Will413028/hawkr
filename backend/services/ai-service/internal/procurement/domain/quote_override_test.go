package domain

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestValidateQuoteDeadline(t *testing.T) {
	now := time.Date(2026, 8, 15, 12, 0, 0, 0, time.UTC)

	t.Run("valid future date", func(t *testing.T) {
		v, err := ValidateQuoteDeadline("2026-08-20", now)
		require.NoError(t, err)
		require.Equal(t, "2026-08-20", v)
	})

	t.Run("today is valid", func(t *testing.T) {
		_, err := ValidateQuoteDeadline("2026-08-15", now)
		require.NoError(t, err)
	})

	t.Run("earlier than today is rejected", func(t *testing.T) {
		_, err := ValidateQuoteDeadline("2026-08-14", now)
		require.Error(t, err)
	})

	t.Run("malformed date is rejected", func(t *testing.T) {
		_, err := ValidateQuoteDeadline("2026/08/20", now)
		require.Error(t, err)
	})
}

func TestValidateBuyerNote(t *testing.T) {
	t.Run("within limit", func(t *testing.T) {
		v, err := ValidateBuyerNote("  急件，優先出貨  ")
		require.NoError(t, err)
		require.Equal(t, "急件，優先出貨", v)
	})

	t.Run("empty after trim is valid", func(t *testing.T) {
		v, err := ValidateBuyerNote("   ")
		require.NoError(t, err)
		require.Equal(t, "", v)
	})

	t.Run("exceeds MaxBuyerNoteLen is rejected", func(t *testing.T) {
		tooLong := make([]rune, MaxBuyerNoteLen+1)
		for i := range tooLong {
			tooLong[i] = '字'
		}
		_, err := ValidateBuyerNote(string(tooLong))
		require.Error(t, err)
	})

	t.Run("exactly MaxBuyerNoteLen is valid", func(t *testing.T) {
		exact := make([]rune, MaxBuyerNoteLen)
		for i := range exact {
			exact[i] = '字'
		}
		_, err := ValidateBuyerNote(string(exact))
		require.NoError(t, err)
	})
}
