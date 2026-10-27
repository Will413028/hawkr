package usecase

import (
	"context"
	"errors"
	"sort"
	"testing"
	"time"

	pkgerrors "github.com/Will413028/hawkr/go-pkg/errors"
	"github.com/Will413028/hawkr/social-service/internal/repository/postgres/gen"
)

func newTestHashtagUC(repo *mockHashtagRepo, ids ...int64) *HashtagUseCase {
	i := 0
	tsidFn := func() int64 {
		if i < len(ids) {
			v := ids[i]
			i++
			return v
		}
		return int64(99000 + i)
	}
	return NewHashtagUseCase(repo, tsidFn)
}

func TestHashtagUC_Apply_CreatesAllWhenNoCurrent(t *testing.T) {
	repo := &mockHashtagRepo{
		listByPostIDFn: func(_ context.Context, _ gen.DBTX, _ int64) ([]gen.CmsPostHashtag, error) {
			return nil, nil
		},
	}
	uc := newTestHashtagUC(repo, 100, 200, 300)
	result, err := uc.Apply(context.Background(), nil, 1, []string{"食物", "Food", "宅配"}, true /*visible*/)
	if err != nil {
		t.Fatalf("Apply err = %v", err)
	}
	if len(result.Added) != 3 {
		t.Errorf("added = %d, want 3", len(result.Added))
	}
	if len(result.Removed) != 0 {
		t.Errorf("removed = %d, want 0", len(result.Removed))
	}
	// counter delta should be +1 for each added, ASC order
	if len(repo.adjustCalls) != 3 {
		t.Fatalf("adjust calls = %d, want 3", len(repo.adjustCalls))
	}
	prev := int64(-1)
	for _, c := range repo.adjustCalls {
		if c.delta != 1 {
			t.Errorf("delta = %d, want 1", c.delta)
		}
		if c.hashtagID <= prev {
			t.Errorf("adjust order not ASC: %v", repo.adjustCalls)
		}
		prev = c.hashtagID
	}
}

func TestHashtagUC_Apply_DiffAddAndRemove(t *testing.T) {
	// Current: id=100 食物 (pos=0), id=200 Food (pos=1)
	// Target:  食物, 宅配  → removes 200 (Food), adds new for 宅配
	repo := &mockHashtagRepo{
		listByPostIDFn: func(_ context.Context, _ gen.DBTX, _ int64) ([]gen.CmsPostHashtag, error) {
			return []gen.CmsPostHashtag{
				{PostID: 1, HashtagID: 100, DisplayName: "食物", Position: 0},
				{PostID: 1, HashtagID: 200, DisplayName: "Food", Position: 1},
			}, nil
		},
		upsertByNameFn: func(_ context.Context, _ gen.DBTX, id int64, canonical string) (int64, error) {
			switch canonical {
			case "食物":
				return 100, nil
			case "宅配":
				return 300, nil
			}
			return id, nil
		},
	}
	uc := newTestHashtagUC(repo, 0, 300)
	result, err := uc.Apply(context.Background(), nil, 1, []string{"食物", "宅配"}, true)
	if err != nil {
		t.Fatalf("Apply err = %v", err)
	}
	if !equalInt64Slice(result.Added, []int64{300}) {
		t.Errorf("added = %v, want [300]", result.Added)
	}
	if !equalInt64Slice(result.Removed, []int64{200}) {
		t.Errorf("removed = %v, want [200]", result.Removed)
	}
	// Expect: +1 to 300, -1 to 200
	hasAdd, hasRemove := false, false
	for _, c := range repo.adjustCalls {
		if c.hashtagID == 300 && c.delta == 1 {
			hasAdd = true
		}
		if c.hashtagID == 200 && c.delta == -1 {
			hasRemove = true
		}
	}
	if !hasAdd || !hasRemove {
		t.Errorf("counter delta missing: %v", repo.adjustCalls)
	}
}

func TestHashtagUC_Apply_NotVisibleNoCounterDelta(t *testing.T) {
	repo := &mockHashtagRepo{
		listByPostIDFn: func(_ context.Context, _ gen.DBTX, _ int64) ([]gen.CmsPostHashtag, error) {
			return nil, nil
		},
	}
	uc := newTestHashtagUC(repo, 100)
	_, err := uc.Apply(context.Background(), nil, 1, []string{"食物"}, false /* not visible */)
	if err != nil {
		t.Fatalf("Apply err = %v", err)
	}
	if len(repo.adjustCalls) != 0 {
		t.Errorf("adjust calls = %d, want 0 (not visible)", len(repo.adjustCalls))
	}
}

func TestHashtagUC_Apply_PropagatesValidationError(t *testing.T) {
	repo := &mockHashtagRepo{}
	uc := newTestHashtagUC(repo)
	_, err := uc.Apply(context.Background(), nil, 1, []string{"#has space"}, true)
	if err == nil {
		t.Fatal("expected validation error, got nil")
	}
}

func TestHashtagUC_AdjustCountersForVisibilityChange(t *testing.T) {
	// 3 bindings on post 1 → -1 for hide, ASC order
	repo := &mockHashtagRepo{
		listHashtagIDsByPostFn: func(_ context.Context, _ gen.DBTX, _ int64) ([]int64, error) {
			return []int64{100, 200, 300}, nil // already ASC from SQL ORDER BY
		},
	}
	uc := newTestHashtagUC(repo)
	if _, err := uc.AdjustCountersForVisibilityChange(context.Background(), nil, 1, -1); err != nil {
		t.Fatalf("err = %v", err)
	}
	if len(repo.adjustCalls) != 3 {
		t.Fatalf("adjust calls = %d, want 3", len(repo.adjustCalls))
	}
	for i, c := range repo.adjustCalls {
		if c.delta != -1 {
			t.Errorf("[%d] delta = %d, want -1", i, c.delta)
		}
		want := []int64{100, 200, 300}[i]
		if c.hashtagID != want {
			t.Errorf("[%d] hashtagID = %d, want %d (ASC)", i, c.hashtagID, want)
		}
	}
}

func TestHashtagUC_AdjustCountersForVisibilityChange_ZeroDeltaNoop(t *testing.T) {
	repo := &mockHashtagRepo{
		listHashtagIDsByPostFn: func(_ context.Context, _ gen.DBTX, _ int64) ([]int64, error) {
			t.Fatal("ListHashtagIDsByPostID should not be called when delta=0")
			return nil, nil
		},
	}
	uc := newTestHashtagUC(repo)
	if _, err := uc.AdjustCountersForVisibilityChange(context.Background(), nil, 1, 0); err != nil {
		t.Fatalf("err = %v", err)
	}
}

func TestHashtagUC_Apply_EmptyRawTagsRemovesAllExisting(t *testing.T) {
	// PATCH "tags": [] semantics — clear all bindings.
	repo := &mockHashtagRepo{
		listByPostIDFn: func(_ context.Context, _ gen.DBTX, _ int64) ([]gen.CmsPostHashtag, error) {
			return []gen.CmsPostHashtag{
				{PostID: 1, HashtagID: 100, DisplayName: "美食", Position: 0},
				{PostID: 1, HashtagID: 200, DisplayName: "宅配", Position: 1},
			}, nil
		},
	}
	uc := newTestHashtagUC(repo)
	result, err := uc.Apply(context.Background(), nil, 1, []string{}, true)
	if err != nil {
		t.Fatalf("Apply err = %v", err)
	}
	if len(result.Added) != 0 {
		t.Errorf("added = %v, want []", result.Added)
	}
	if !equalInt64Slice(result.Removed, []int64{100, 200}) {
		t.Errorf("removed = %v, want [100, 200]", result.Removed)
	}
	// Counter delta: -1 for each removed, ASC order.
	if len(repo.adjustCalls) != 2 {
		t.Fatalf("adjust calls = %d, want 2", len(repo.adjustCalls))
	}
	for i, c := range repo.adjustCalls {
		want := []int64{100, 200}[i]
		if c.hashtagID != want || c.delta != -1 {
			t.Errorf("[%d] adjust = {id=%d delta=%d}, want {id=%d delta=-1}", i, c.hashtagID, c.delta, want)
		}
	}
}

func TestHashtagUC_Apply_KeptTagPositionReorder(t *testing.T) {
	// current=[100@pos0, 200@pos1]; target reorders to [200, 100] (positions swap).
	// Both rows are kept — expect UpdateBinding for both with new positions,
	// no Insert/Delete, no counter delta.
	var inserts, deletes int
	updateCalls := []struct {
		hashtagID int64
		position  int16
	}{}
	repo := &mockHashtagRepo{
		listByPostIDFn: func(_ context.Context, _ gen.DBTX, _ int64) ([]gen.CmsPostHashtag, error) {
			return []gen.CmsPostHashtag{
				{PostID: 1, HashtagID: 100, DisplayName: "美食", Position: 0},
				{PostID: 1, HashtagID: 200, DisplayName: "宅配", Position: 1},
			}, nil
		},
		upsertByNameFn: func(_ context.Context, _ gen.DBTX, _ int64, canonical string) (int64, error) {
			switch canonical {
			case "宅配":
				return 200, nil
			case "美食":
				return 100, nil
			}
			return 0, nil
		},
		insertBindingFn: func(_ context.Context, _ gen.DBTX, _, _ int64, _ string, _ int16) error {
			inserts++
			return nil
		},
		deleteBindingsFn: func(_ context.Context, _ gen.DBTX, _ int64, _ []int64) error {
			deletes++
			return nil
		},
		updateBindingFn: func(_ context.Context, _ gen.DBTX, _, hashtagID int64, _ string, position int16) error {
			updateCalls = append(updateCalls, struct {
				hashtagID int64
				position  int16
			}{hashtagID, position})
			return nil
		},
	}
	uc := newTestHashtagUC(repo)
	_, err := uc.Apply(context.Background(), nil, 1, []string{"宅配", "美食"}, true)
	if err != nil {
		t.Fatalf("Apply err = %v", err)
	}
	if inserts != 0 {
		t.Errorf("inserts = %d, want 0 (both kept)", inserts)
	}
	if deletes != 0 {
		t.Errorf("deletes = %d, want 0 (both kept)", deletes)
	}
	if len(updateCalls) != 2 {
		t.Fatalf("update calls = %d, want 2 (both positions changed)", len(updateCalls))
	}
	// Update order follows target iteration: 200@0, 100@1.
	if updateCalls[0].hashtagID != 200 || updateCalls[0].position != 0 {
		t.Errorf("update[0] = %+v, want {hashtagID=200 position=0}", updateCalls[0])
	}
	if updateCalls[1].hashtagID != 100 || updateCalls[1].position != 1 {
		t.Errorf("update[1] = %+v, want {hashtagID=100 position=1}", updateCalls[1])
	}
	if len(repo.adjustCalls) != 0 {
		t.Errorf("adjust calls = %d, want 0 (kept-only, no counter delta)", len(repo.adjustCalls))
	}
}

func TestHashtagUC_Apply_CounterDeltaMergedASCAcrossAddedAndRemoved(t *testing.T) {
	// Regression guard for spec §4.2: lock acquisition MUST be ASC across the
	// UNION of added ∪ removed, not two independent ASC passes (which would
	// deadlock with an inverted concurrent Apply).
	// current=[3, 7] (will be removed); target maps to [5, 10] (will be added)
	repo := &mockHashtagRepo{
		listByPostIDFn: func(_ context.Context, _ gen.DBTX, _ int64) ([]gen.CmsPostHashtag, error) {
			return []gen.CmsPostHashtag{
				{PostID: 1, HashtagID: 3, DisplayName: "a", Position: 0},
				{PostID: 1, HashtagID: 7, DisplayName: "b", Position: 1},
			}, nil
		},
		upsertByNameFn: func(_ context.Context, _ gen.DBTX, _ int64, canonical string) (int64, error) {
			switch canonical {
			case "c":
				return 5, nil
			case "d":
				return 10, nil
			}
			return 0, nil
		},
	}
	uc := newTestHashtagUC(repo)
	_, err := uc.Apply(context.Background(), nil, 1, []string{"c", "d"}, true)
	if err != nil {
		t.Fatalf("Apply err = %v", err)
	}
	// Expect merged ASC: [3 -1, 5 +1, 7 -1, 10 +1] — NOT [5 +1, 10 +1, 3 -1, 7 -1].
	wantOrder := []adjustCall{
		{3, -1},
		{5, 1},
		{7, -1},
		{10, 1},
	}
	if len(repo.adjustCalls) != len(wantOrder) {
		t.Fatalf("adjust calls = %d, want %d", len(repo.adjustCalls), len(wantOrder))
	}
	for i, want := range wantOrder {
		got := repo.adjustCalls[i]
		if got != want {
			t.Errorf("[%d] adjust = %+v, want %+v", i, got, want)
		}
	}
}

func TestHashtagUC_Search_PrefixMode(t *testing.T) {
	called := false
	repo := &mockHashtagRepo{
		searchByPrefixFn: func(_ context.Context, canonical string, limit int32) ([]gen.SearchHashtagsByPrefixRow, error) {
			called = true
			if canonical != "美" {
				t.Errorf("canonical = %q, want %q", canonical, "美")
			}
			if limit != 10 {
				t.Errorf("limit = %d, want 10 (default)", limit)
			}
			return []gen.SearchHashtagsByPrefixRow{{Name: "美食", PostCount: 5}}, nil
		},
	}
	uc := newTestHashtagUC(repo)
	out, err := uc.Search(context.Background(), "#美", SearchModePrefix, 0)
	if err != nil {
		t.Fatalf("Search err = %v", err)
	}
	if !called {
		t.Fatal("expected prefix search to be called")
	}
	if len(out) != 1 || out[0].Name != "美食" || out[0].PostCount != 5 {
		t.Errorf("out = %+v, want [{美食 5}]", out)
	}
}

func TestHashtagUC_Search_ContainMode(t *testing.T) {
	called := false
	repo := &mockHashtagRepo{
		searchByContainFn: func(_ context.Context, _ string, _ int32) ([]gen.SearchHashtagsByContainRow, error) {
			called = true
			return nil, nil
		},
	}
	uc := newTestHashtagUC(repo)
	_, err := uc.Search(context.Background(), "美", SearchModeContain, 0)
	if err != nil || !called {
		t.Errorf("Contain mode not dispatched correctly: called=%v err=%v", called, err)
	}
}

func TestHashtagUC_Search_LimitCaps(t *testing.T) {
	var gotLimit int32
	repo := &mockHashtagRepo{
		searchByPrefixFn: func(_ context.Context, _ string, limit int32) ([]gen.SearchHashtagsByPrefixRow, error) {
			gotLimit = limit
			return nil, nil
		},
	}
	uc := newTestHashtagUC(repo)
	_, _ = uc.Search(context.Background(), "a", SearchModePrefix, 100)
	if gotLimit != 20 {
		t.Errorf("limit = %d, want 20 (capped)", gotLimit)
	}
}

func TestHashtagUC_Search_EmptyOrInvalidQReturnsNil(t *testing.T) {
	repo := &mockHashtagRepo{}
	uc := newTestHashtagUC(repo)
	out, err := uc.Search(context.Background(), "", SearchModePrefix, 0)
	if err != nil || out != nil {
		t.Errorf("empty q: out=%v err=%v", out, err)
	}
	out, err = uc.Search(context.Background(), "###", SearchModePrefix, 0)
	if err != nil || out != nil {
		t.Errorf("hash-only q: out=%v err=%v", out, err)
	}
}

func TestHashtagUC_Popular_DefaultRange(t *testing.T) {
	var gotStart, gotEnd time.Time
	var gotLimit int32
	repo := &mockHashtagRepo{
		popularInRangeFn: func(_ context.Context, start, end time.Time, limit int32) ([]gen.SelectPopularHashtagsInRangeRow, error) {
			gotStart, gotEnd, gotLimit = start, end, limit
			return []gen.SelectPopularHashtagsInRangeRow{{Hashtag: "美食", Count: 7}}, nil
		},
	}
	uc := newTestHashtagUC(repo)
	out, err := uc.Popular(context.Background(), "", "", 0)
	if err != nil {
		t.Fatalf("Popular err = %v", err)
	}
	if gotLimit != 20 {
		t.Errorf("limit = %d, want 20 (default)", gotLimit)
	}
	// end should be after start, range ≈ 1 month + 1 day (end-exclusive)
	if !gotEnd.After(gotStart) {
		t.Errorf("end %v not after start %v", gotEnd, gotStart)
	}
	diff := gotEnd.Sub(gotStart)
	if diff < 28*24*time.Hour || diff > 33*24*time.Hour {
		t.Errorf("range diff = %v, want ~1 month +1 day", diff)
	}
	if len(out) != 1 || out[0].Hashtag != "美食" || out[0].Count != 7 {
		t.Errorf("out = %+v", out)
	}
}

func TestHashtagUC_Popular_ExplicitRange(t *testing.T) {
	var gotStart, gotEnd time.Time
	repo := &mockHashtagRepo{
		popularInRangeFn: func(_ context.Context, start, end time.Time, _ int32) ([]gen.SelectPopularHashtagsInRangeRow, error) {
			gotStart, gotEnd = start, end
			return nil, nil
		},
	}
	uc := newTestHashtagUC(repo)
	_, err := uc.Popular(context.Background(), "2026-01-01", "2026-01-31", 5)
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	// Verify left-closed right-open: start_at = 2026-01-01 00:00 UTC, end_at = 2026-02-01 00:00 UTC.
	wantStart := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	wantEnd := time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC)
	if !gotStart.Equal(wantStart) {
		t.Errorf("start = %v, want %v", gotStart, wantStart)
	}
	if !gotEnd.Equal(wantEnd) {
		t.Errorf("end = %v, want %v", gotEnd, wantEnd)
	}
}

func TestHashtagUC_Popular_RejectsInvalidDateFormat(t *testing.T) {
	uc := newTestHashtagUC(&mockHashtagRepo{})
	_, err := uc.Popular(context.Background(), "2026/01/01", "", 0)
	if err == nil {
		t.Fatal("expected error for invalid date format")
	}
	// Verify error type so Task 11 handler can errors.As to translate to gateway 400.
	var de *pkgerrors.DomainError
	if !errors.As(err, &de) || de.DomainCode() != int(pkgerrors.CodeInvalidInput) {
		t.Errorf("err = %v, want *DomainError(CodeInvalidInput)", err)
	}
}

func TestHashtagUC_Popular_RejectsRangeOver1Year(t *testing.T) {
	uc := newTestHashtagUC(&mockHashtagRepo{})
	_, err := uc.Popular(context.Background(), "2024-01-01", "2026-01-02", 0)
	if err == nil {
		t.Fatal("expected error for range > 1 year")
	}
}

func TestHashtagUC_Popular_RangeBoundary(t *testing.T) {
	// Exact 1 year (start.AddDate(1,0,0) == end) — should PASS (the check is After, not Before|Equal).
	repo := &mockHashtagRepo{}
	uc := newTestHashtagUC(repo)
	if _, err := uc.Popular(context.Background(), "2025-01-01", "2026-01-01", 0); err != nil {
		t.Errorf("exactly 1 year should pass, got %v", err)
	}
	// 1 year + 1 day — should FAIL.
	if _, err := uc.Popular(context.Background(), "2025-01-01", "2026-01-02", 0); err == nil {
		t.Error("1 year + 1 day should be rejected")
	}
}

func TestHashtagUC_Popular_RejectsStartAfterEnd(t *testing.T) {
	uc := newTestHashtagUC(&mockHashtagRepo{})
	_, err := uc.Popular(context.Background(), "2026-03-01", "2026-02-01", 0)
	if err == nil {
		t.Fatal("expected error for start > end")
	}
}

func TestResolvePopularRange_AllBranches(t *testing.T) {
	// Fixed "now" for deterministic assertions.
	now := time.Date(2026, 5, 16, 12, 0, 0, 0, time.UTC)
	today := time.Date(2026, 5, 16, 0, 0, 0, 0, time.UTC)

	t.Run("both empty → last 1 month, ending today", func(t *testing.T) {
		start, end, err := resolvePopularRange("", "", now)
		if err != nil {
			t.Fatal(err)
		}
		if !end.Equal(today) {
			t.Errorf("end = %v, want %v", end, today)
		}
		wantStart := today.AddDate(0, -1, 0) // 2026-04-16
		if !start.Equal(wantStart) {
			t.Errorf("start = %v, want %v", start, wantStart)
		}
	})

	t.Run("only start → end = start + 1y when within today", func(t *testing.T) {
		// start far in past so start+1y < today
		start, end, err := resolvePopularRange("2024-01-01", "", now)
		if err != nil {
			t.Fatal(err)
		}
		wantEnd := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
		if !end.Equal(wantEnd) {
			t.Errorf("end = %v, want %v (start+1y < today)", end, wantEnd)
		}
		if !start.Equal(time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)) {
			t.Errorf("start = %v", start)
		}
	})

	t.Run("only start → end clamped to today when start+1y > today", func(t *testing.T) {
		// start = today - 30 days → start+1y is in future → clamp to today
		startDate := today.AddDate(0, 0, -30)
		start, end, err := resolvePopularRange(startDate.Format(popularDateFormat), "", now)
		if err != nil {
			t.Fatal(err)
		}
		if !end.Equal(today) {
			t.Errorf("end = %v, want %v (today clamp)", end, today)
		}
		if !start.Equal(startDate) {
			t.Errorf("start = %v", start)
		}
	})

	t.Run("only end (in past) → start = end - 1y", func(t *testing.T) {
		start, end, err := resolvePopularRange("", "2026-03-15", now)
		if err != nil {
			t.Fatal(err)
		}
		wantStart := time.Date(2025, 3, 15, 0, 0, 0, 0, time.UTC)
		if !start.Equal(wantStart) {
			t.Errorf("start = %v, want %v", start, wantStart)
		}
		if !end.Equal(time.Date(2026, 3, 15, 0, 0, 0, 0, time.UTC)) {
			t.Errorf("end = %v", end)
		}
	})

	t.Run("only end (future) → end clamped to today, start = today - 1y", func(t *testing.T) {
		start, end, err := resolvePopularRange("", "2099-01-01", now)
		if err != nil {
			t.Fatal(err)
		}
		if !end.Equal(today) {
			t.Errorf("end = %v, want %v (future clamp)", end, today)
		}
		wantStart := today.AddDate(-1, 0, 0)
		if !start.Equal(wantStart) {
			t.Errorf("start = %v, want %v", start, wantStart)
		}
	})

	t.Run("both given", func(t *testing.T) {
		start, end, err := resolvePopularRange("2026-01-01", "2026-01-31", now)
		if err != nil {
			t.Fatal(err)
		}
		if !start.Equal(time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)) {
			t.Errorf("start = %v", start)
		}
		if !end.Equal(time.Date(2026, 1, 31, 0, 0, 0, 0, time.UTC)) {
			t.Errorf("end = %v", end)
		}
	})

	t.Run("invalid start format", func(t *testing.T) {
		_, _, err := resolvePopularRange("2026/01/01", "", now)
		if err == nil {
			t.Fatal("expected error")
		}
		var de *pkgerrors.DomainError
		if !errors.As(err, &de) || de.DomainCode() != int(pkgerrors.CodeInvalidInput) {
			t.Errorf("err = %v, want *DomainError(CodeInvalidInput)", err)
		}
	})

	t.Run("start after end", func(t *testing.T) {
		_, _, err := resolvePopularRange("2026-03-01", "2026-02-01", now)
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("range exceeds 1 year", func(t *testing.T) {
		_, _, err := resolvePopularRange("2024-01-01", "2025-06-01", now)
		if err == nil {
			t.Fatal("expected error")
		}
	})
}

func equalInt64Slice(a, b []int64) bool {
	if len(a) != len(b) {
		return false
	}
	a2 := append([]int64(nil), a...)
	b2 := append([]int64(nil), b...)
	sort.Slice(a2, func(i, j int) bool { return a2[i] < a2[j] })
	sort.Slice(b2, func(i, j int) bool { return b2[i] < b2[j] })
	for i := range a2 {
		if a2[i] != b2[i] {
			return false
		}
	}
	return true
}
