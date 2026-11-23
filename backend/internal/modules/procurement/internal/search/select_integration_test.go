//go:build integration

package search_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"uuid"

	"github.com/Will413028/hawkr/backend/internal/modules/procurement/internal/candidate"
	"github.com/Will413028/hawkr/backend/internal/modules/procurement/internal/search"
	"github.com/Will413028/hawkr/backend/internal/modules/procurement/internal/store"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
)

func (f *fixture) selector() search.Selector {
	return search.Selector{DB: f.d, Now: func() time.Time { return now }}
}

func (f *fixture) toggle(buyerID int64, set, candidate uuid.UUID, selected bool) error {
	return f.selector().Select(context.Background(), search.SelectInput{BuyerID: buyerID, RequestID: f.request, CandidateSetID: set, CandidateID: candidate, Selected: selected})
}

// searched 跑完一次成功的搜尋（店 10、20），回候選集與它的第一個候選。
func (f *fixture) searched() (uuid.UUID, store.ProcurementCandidate) {
	f.t.Helper()
	f.fakes.pages = map[int]candidate.Page{1: hits(10, 20)}
	s := f.mustStart()
	if err := f.dispatch(s.RunID); err != nil {
		f.t.Fatal(err)
	}
	return s.CandidateSetID, f.candidates(s.CandidateSetID)[0]
}

func wantConflictTarget(t *testing.T, err error, target string) {
	t.Helper()
	var ae *apperr.Error
	if !errors.As(err, &ae) || ae.Kind() != apperr.KindVersionConflict || ae.Reason != apperr.ReasonVersionConflict || ae.Metadata["conflictTarget"] != target {
		t.Fatalf("要 VERSION_CONFLICT conflictTarget=%s，得到 %v", target, err)
	}
}

func wantKind(t *testing.T, err error, kind apperr.Kind) {
	t.Helper()
	if apperr.KindOf(err) != kind {
		t.Fatalf("要 %v，得到 %v", kind, err)
	}
}

func TestToggleSelectionSetsTheTargetStateOnTheEffectiveSetAndIsIdempotent(t *testing.T) {
	f := newFixture(t, "REQUIREMENT_READY", "CATEGORY_CONFIRMED")
	set, c := f.searched()
	for range 2 {
		must(t, f.toggle(buyer, set, c.ID, true))
		if got := f.candidates(set)[0]; !got.Selected || got.SelectedAt == nil || !got.SelectedAt.Equal(now) {
			t.Fatalf("勾選後 = %+v", got)
		}
	}
	for range 2 {
		must(t, f.toggle(buyer, set, c.ID, false))
		if got := f.candidates(set)[0]; got.Selected || got.SelectedAt != nil {
			t.Fatalf("取消後 = %+v", got)
		}
	}
	if got := f.candidates(set)[1]; got.Selected {
		t.Fatal("只動指定的候選")
	}
}

func TestToggleSelectionOnASupersededSetIsAbortedAfterAReSearchSucceeds(t *testing.T) {
	f := newFixture(t, "REQUIREMENT_READY", "CATEGORY_CONFIRMED")
	oldSet, oldCand := f.searched()
	newSet, newCand := f.searched()
	wantConflictTarget(t, f.toggle(buyer, oldSet, oldCand.ID, true), "candidate_set")
	if f.candidates(oldSet)[0].Selected {
		t.Fatal("過時的候選集不寫入")
	}
	must(t, f.toggle(buyer, newSet, newCand.ID, true))
}

func TestToggleSelectionOnASetWhoseSearchHasNotSucceededIsAborted(t *testing.T) {
	f := newFixture(t, "REQUIREMENT_READY", "CATEGORY_CONFIRMED")
	set, c := f.searched()
	pending := f.mustStart()
	must(t, f.toggle(buyer, set, c.ID, true))
	wantConflictTarget(t, f.toggle(buyer, pending.CandidateSetID, uuid.NewV7(), true), "candidate_set")
}

func TestToggleSelectionIsAbortedOnceTheRequestLeavesMatching(t *testing.T) {
	f := newFixture(t, "REQUIREMENT_READY", "CATEGORY_CONFIRMED")
	set, c := f.searched()
	b := buyer
	if n, err := store.New(f.d.Conn(context.Background())).SetRequestStatus(context.Background(), store.SetRequestStatusParams{Status: "CLARIFYING", Now: now, ID: f.request, BuyerID: &b, FromStatus: "MATCHING"}); err != nil || n != 1 {
		t.Fatalf("退回 CLARIFYING n=%d err=%v", n, err)
	}
	wantConflictTarget(t, f.toggle(buyer, set, c.ID, true), "candidate_set")
	if f.candidates(set)[0].Selected {
		t.Fatal("離開 MATCHING 之後不寫入")
	}
}

func TestToggleSelectionNotFoundBranches(t *testing.T) {
	f := newFixture(t, "REQUIREMENT_READY", "CATEGORY_CONFIRMED")
	set, c := f.searched()
	wantKind(t, f.toggle(buyer+1, set, c.ID, true), apperr.KindNotFound)
	wantKind(t, f.toggle(buyer, uuid.NewV7(), c.ID, true), apperr.KindNotFound)
	wantKind(t, f.toggle(buyer, set, uuid.NewV7(), true), apperr.KindNotFound)
	other := newFixture(t, "REQUIREMENT_READY", "CATEGORY_CONFIRMED")
	otherSet, otherCand := other.searched()
	wantKind(t, f.toggle(buyer, otherSet, otherCand.ID, true), apperr.KindNotFound)
	f.deleteConversation()
	wantKind(t, f.toggle(buyer, set, c.ID, true), apperr.KindNotFound)
}
