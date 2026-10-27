package model

import (
	"slices"
	"testing"
	"time"
)

func TestPublicStoreUpdateTimeIsTheLatestOfEverySource(t *testing.T) {
	at := func(h int) *time.Time {
		v := time.Date(2026, 10, 1, h, 0, 0, 0, time.UTC)
		return &v
	}
	cases := []struct {
		name             string
		updated          time.Time
		first, announced *time.Time
		want             time.Time
	}{
		{"only updated_at", *at(1), nil, nil, *at(1)},
		{"first exposure later", *at(1), at(2), nil, *at(2)},
		{"announcement latest", *at(1), at(2), at(5), *at(5)},
		{"older sources are ignored", *at(6), at(2), at(3), *at(6)},
	}
	for _, c := range cases {
		if got := PublicStoreUpdateTime(c.updated, c.first, c.announced); !got.Equal(c.want) {
			t.Errorf("%s：%v，want %v", c.name, got, c.want)
		}
	}
}

// MissingExposure 只把讀出來的條件對應到 Requirement（條件本身的述詞在 SQL，由 integration 的資料集測試鎖住）：每一項條件不滿足時
// 只缺它自己那一項，多項時依 Requirements 的順序；停權讀 Sanctions；全部滿足時為空。
func TestMissingExposureMapsEachConditionToItsRequirement(t *testing.T) {
	full := fullStore(Active, false)
	if got := MissingExposure(full); len(got) != 0 {
		t.Fatalf("full store missing %v", got)
	}
	cases := map[Requirement]func(*Store){
		RequirementActive:             func(s *Store) { s.Conditions.Active = false },
		RequirementNotPendingDeletion: func(s *Store) { s.Conditions.NotPendingDeletion = false },
		RequirementNotSuspended:       func(s *Store) { s.Sanctions = sanctionsOf(true) },
		RequirementKYCApproved:        func(s *Store) { s.Conditions.KYCApproved = false },
		RequirementStoreName:          func(s *Store) { s.Conditions.StoreName = false },
		RequirementLogo:               func(s *Store) { s.Conditions.Logo = false },
		RequirementTownCode:           func(s *Store) { s.Conditions.TownCode = false },
	}
	if len(cases) != len(Requirements()) {
		t.Fatalf("cases cover %d of %d requirements", len(cases), len(Requirements()))
	}
	all := full
	for r, mutate := range cases {
		s := full
		mutate(&s)
		mutate(&all)
		if got := MissingExposure(s); !slices.Equal(got, []Requirement{r}) {
			t.Errorf("%s: missing=%v", r, got)
		}
	}
	if got := MissingExposure(all); !slices.Equal(got, Requirements()) {
		t.Fatalf("every condition unmet: missing=%v want %v", got, Requirements())
	}
	// 生命週期欄位（Status）與店名等欄位不參與：只看 Conditions。
	s := full
	s.Status, s.StoreName = Closed, ""
	if got := MissingExposure(s); len(got) != 0 {
		t.Fatalf("Go must not re-derive conditions from fields: missing=%v", got)
	}
}

func TestBatchIDs(t *testing.T) {
	if err := BatchIDs(nil); err != nil {
		t.Fatalf("empty: %v", err)
	}
	if err := BatchIDs([]int64{1, 0}); err == nil {
		t.Fatal("non-positive id accepted")
	}
	if err := BatchIDs(make([]int64, MaxBatchSize+1)); err == nil {
		t.Fatal("oversized batch accepted")
	}
	ids := make([]int64, MaxBatchSize)
	for i := range ids {
		ids[i] = int64(i + 1)
	}
	if err := BatchIDs(ids); err != nil {
		t.Fatalf("max batch: %v", err)
	}
}
