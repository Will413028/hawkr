package rfq

import (
	"errors"
	"strings"
	"testing"
	"time"

	"uuid"

	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
)

func directRecipients() PreviewPublicationInput {
	set := uuid.NewV7()
	return PreviewPublicationInput{LowerBoundAmount: 100, UpperBoundAmount: 200, CandidateSetID: set, SelectedCandidateIDs: []uuid.UUID{uuid.NewV7()}}
}

func TestCheckInputReportsEachInvalidInputOnItsField(t *testing.T) {
	for _, tc := range []struct {
		name  string
		edit  func(*PreviewPublicationInput)
		field string // 空字串表示合法
	}{
		{"valid", func(*PreviewPublicationInput) {}, ""},
		{"zero lower", func(i *PreviewPublicationInput) { i.LowerBoundAmount = 0 }, "LowerBoundAmount"},
		{"inverted", func(i *PreviewPublicationInput) { i.LowerBoundAmount, i.UpperBoundAmount = 20, 10 }, "LowerBoundAmount"},
		{"note at the limit", func(i *PreviewPublicationInput) { i.BuyerNote = strings.Repeat("字", MaxBuyerNoteRunes) }, ""},
		{"note over the limit", func(i *PreviewPublicationInput) { i.BuyerNote = strings.Repeat("字", MaxBuyerNoteRunes+1) }, "BuyerNote"},
		{"note only the padding pushes it over", func(i *PreviewPublicationInput) { i.BuyerNote = "  " + strings.Repeat("字", MaxBuyerNoteRunes) + "  " }, ""},
		{"no candidate set", func(i *PreviewPublicationInput) { i.CandidateSetID = uuid.UUID{} }, "CandidateSetID"},
	} {
		in := directRecipients()
		tc.edit(&in)
		_, err := checkInput(in)
		if tc.field == "" {
			if err != nil {
				t.Errorf("%s：不該有錯：%v", tc.name, err)
			}
			continue
		}
		var ae *apperr.Error
		if !errors.As(err, &ae) || ae.Kind() != apperr.KindInvalidInput || ae.FieldViolations()[0].Field != tc.field {
			t.Errorf("%s：要 INVALID_ARGUMENT 指向 %s，得到 %v", tc.name, tc.field, err)
		}
	}
}

func TestCheckInputReturnsTheTrimmedNote(t *testing.T) {
	in := directRecipients()
	in.BuyerNote = "  請盡快 \n"
	note, err := checkInput(in)
	if err != nil || note != "請盡快" {
		t.Fatalf("note = %q err = %v", note, err)
	}
}

// 期限以台北的今天為界：台北 00:30（UTC 前一天 16:30）時，UTC 的昨天就是台北的今天。
func TestDeadlinePassedUsesTheTaipeiCalendarDay(t *testing.T) {
	day := func(m time.Month, d int) time.Time { return time.Date(2026, m, d, 0, 0, 0, 0, time.UTC) }
	for _, tc := range []struct {
		name     string
		deadline time.Time
		now      time.Time
		want     bool
	}{
		{"today", day(11, 23), time.Date(2026, 11, 23, 3, 0, 0, 0, time.UTC), false},
		{"yesterday", day(11, 22), time.Date(2026, 11, 23, 3, 0, 0, 0, time.UTC), true},
		{"tomorrow", day(11, 24), time.Date(2026, 11, 23, 3, 0, 0, 0, time.UTC), false},
		{"taipei already the 23rd while utc is the 22nd: the 22nd is past", day(11, 22), time.Date(2026, 11, 22, 16, 30, 0, 0, time.UTC), true},
		{"taipei already the 23rd while utc is the 22nd: the 23rd is today", day(11, 23), time.Date(2026, 11, 22, 16, 30, 0, 0, time.UTC), false},
		{"last minute of the taipei day", day(11, 23), time.Date(2026, 11, 23, 15, 59, 59, 0, time.UTC), false},
		{"first second of the next taipei day", day(11, 23), time.Date(2026, 11, 23, 16, 0, 0, 0, time.UTC), true},
		{"deadline carrying a time of day is compared by its date", time.Date(2026, 11, 23, 18, 0, 0, 0, time.UTC), time.Date(2026, 11, 23, 3, 0, 0, 0, time.UTC), false},
	} {
		if got := DeadlinePassed(tc.deadline, tc.now); got != tc.want {
			t.Errorf("%s：DeadlinePassed = %v", tc.name, got)
		}
	}
}
