package turn

import (
	"errors"
	"slices"
	"testing"

	"uuid"

	"github.com/Will413028/hawkr/backend/internal/modules/procurement/internal/model"
	"github.com/Will413028/hawkr/backend/internal/modules/procurement/internal/slots"
	"github.com/Will413028/hawkr/backend/internal/modules/procurement/internal/store"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
	"github.com/Will413028/hawkr/backend/internal/platform/uitext"
)

// chainOf 依序建立版本：每一版的內容祖先是 parents 指的那一版（-1 是沒有祖先）。city 是縣市（空字串是沒填）。
func chainOf(cities []string, parents []int) []store.ProcurementRequestRevision {
	var out []store.ProcurementRequestRevision
	for i, c := range cities {
		r := store.ProcurementRequestRevision{ID: uuid.NewV7(), RevisionNo: int32(i + 1), Item: sp("蘋果")}
		if c != "" {
			r.DeliveryCity = sp(c)
		}
		if parents[i] >= 0 {
			id := out[parents[i]].ID
			r.ParentRevisionID = &id
		}
		out = append(out, r)
	}
	return out
}

func wantNothingToRewind(t *testing.T, err error) {
	t.Helper()
	var ae *apperr.Error
	if err == nil || !errors.As(err, &ae) || ae.Kind() != apperr.KindFailedPrecondition || ae.Reason.String() != "" {
		t.Fatalf("要沒有 reason 的 FAILED_PRECONDITION，得到 %v", err)
	}
}

func TestRewindTargetIsTheParentOfTheNearestRevisionThatChangedTheSlot(t *testing.T) {
	revs := chainOf([]string{"臺北市", "臺中市", "高雄市"}, []int{-1, 0, 1})
	got, err := rewindTarget(revs, revs[2], slots.DeliveryCity)
	if err != nil || got.ID != revs[1].ID {
		t.Fatalf("target = %v %v，要第二版", got, err)
	}
}

func TestRewindTargetSkipsRevisionsThatLeftTheSlotAlone(t *testing.T) {
	revs := chainOf([]string{"臺北市", "臺中市", "臺中市", "臺中市"}, []int{-1, 0, 1, 2})
	got, err := rewindTarget(revs, revs[3], slots.DeliveryCity)
	if err != nil || got.ID != revs[0].ID {
		t.Fatalf("target = %v %v，要改動縣市的那一版（第二版）的祖先，也就是第一版", got, err)
	}
}

func TestRewindTargetStepsFurtherBackAfterARewindAndThenRunsOut(t *testing.T) {
	// 1 臺北市 → 2 臺中市 → 3 高雄市；倒回得到第 4 版（內容同第 2 版，祖先是第 2 版）；再倒回得到第 5 版（內容同第 1 版）。
	revs := chainOf([]string{"臺北市", "臺中市", "高雄市", "臺中市"}, []int{-1, 0, 1, 1})
	got, err := rewindTarget(revs, revs[3], slots.DeliveryCity)
	if err != nil || got.ID != revs[0].ID {
		t.Fatalf("第二次倒回要再退一步到第一版，而不是回到高雄市：%v %v", got, err)
	}
	revs = append(revs, chainOf([]string{"臺北市"}, []int{-1})[0])
	revs[4].RevisionNo = 5
	revs[4].ParentRevisionID = &revs[0].ID
	_, err = rewindTarget(revs, revs[4], slots.DeliveryCity)
	wantNothingToRewind(t, err)
}

func TestRewindTargetCanLandOnAVersionWhereTheSlotWasUnset(t *testing.T) {
	revs := chainOf([]string{"", "臺北市"}, []int{-1, 0})
	got, err := rewindTarget(revs, revs[1], slots.DeliveryCity)
	if err != nil || got.ID != revs[0].ID || got.DeliveryCity != nil {
		t.Fatalf("target = %+v %v，要欄位還沒填的第一版", got, err)
	}
	// 倒回之後（第 3 版內容同第 1 版）欄位沒有值，沒有東西可倒回。
	revs = append(revs, store.ProcurementRequestRevision{ID: uuid.NewV7(), RevisionNo: 3, Item: sp("蘋果"), ParentRevisionID: &revs[0].ID})
	_, err = rewindTarget(revs, revs[2], slots.DeliveryCity)
	wantNothingToRewind(t, err)
}

func TestRewindTargetNeverWrittenSlotHasNothingToRewind(t *testing.T) {
	revs := chainOf([]string{"臺北市"}, []int{-1})
	_, err := rewindTarget(revs, revs[0], slots.Packaging)
	wantNothingToRewind(t, err)
}

func TestRewindTargetOfTheFirstValueHasNoParentToLandOn(t *testing.T) {
	revs := chainOf([]string{"臺北市"}, []int{-1})
	_, err := rewindTarget(revs, revs[0], slots.DeliveryCity)
	wantNothingToRewind(t, err)
}

func TestRewindTargetQuantityCoversTheUnit(t *testing.T) {
	a := store.ProcurementRequestRevision{ID: uuid.NewV7(), RevisionNo: 1, Item: sp("蘋果"), Quantity: new(int32), Unit: sp("KG")}
	*a.Quantity = 3
	b := a
	b.ID, b.RevisionNo, b.ParentRevisionID, b.Unit = uuid.NewV7(), 2, &a.ID, sp("CATTY")
	got, err := rewindTarget([]store.ProcurementRequestRevision{a, b}, b, slots.Quantity)
	if err != nil || got.ID != a.ID {
		t.Fatalf("只改單位也算改了數量這一格：%v %v", got, err)
	}
}

func TestRewindTargetItemIgnoresTheServerFilledResolution(t *testing.T) {
	a := store.ProcurementRequestRevision{ID: uuid.NewV7(), RevisionNo: 1, Item: sp("梨")}
	b := store.ProcurementRequestRevision{ID: uuid.NewV7(), RevisionNo: 2, ParentRevisionID: &a.ID, Item: sp("蘋果")}
	c := b
	kind, label, cat := "CATEGORY_CONFIRMED", "蘋果", int64(21)
	c.ID, c.RevisionNo, c.ParentRevisionID, c.ItemResolutionKind, c.ItemResolutionLabel, c.ItemResolutionCategoryID = uuid.NewV7(), 3, &b.ID, &kind, &label, &cat
	got, err := rewindTarget([]store.ProcurementRequestRevision{a, b, c}, c, slots.Item)
	if err != nil || got.ID != a.ID {
		t.Fatalf("品項倒回要越過只補上解析的版本，落在第一版：%v %v", got, err)
	}
}

func TestRewindDecisionAsksTheRewoundSlotWithZeroStreakAndForgetsItsSkip(t *testing.T) {
	prior := readyPrior()
	prior.prevTurn = &prevTurn{optionalStreak: 2, skipped: []model.SlotKey{slots.Packaging, slots.BudgetPerUnit}}
	next := prior.current
	d := rewindDecision(uitext.ZhTW, prior, next, slots.Packaging)
	if d.question == nil || d.question.Slot != slots.Packaging {
		t.Fatalf("question = %+v", d.question)
	}
	if d.plan.OptionalStreak != 0 || !slices.Equal(d.plan.Skipped, []model.SlotKey{slots.BudgetPerUnit}) {
		t.Errorf("streak = %d skipped = %v", d.plan.OptionalStreak, d.plan.Skipped)
	}
}

func TestRewindDecisionRegressesMatchingOnlyWhenTheItemResolutionChanges(t *testing.T) {
	prior := readyPrior()
	prior.requestOf = statusMatching
	next := prior.current
	next.DeliveryCity = nil
	if d := rewindDecision(uitext.ZhTW, prior, next, slots.DeliveryCity); d.status != statusMatching {
		t.Errorf("縣市不影響搜尋，status = %s", d.status)
	}
	next = prior.current
	next.Resolution = nil
	if d := rewindDecision(uitext.ZhTW, prior, next, slots.Item); d.status != statusClarifying {
		t.Errorf("品項解析變了要退回，status = %s", d.status)
	}
}
