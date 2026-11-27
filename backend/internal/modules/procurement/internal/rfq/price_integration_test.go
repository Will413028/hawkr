//go:build integration

package rfq_test

import (
	"context"
	"math"
	"testing"

	"uuid"

	"github.com/Will413028/hawkr/backend/internal/modules/procurement/internal/store"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
)

func TestPreviewPriceComputesTheTotalAndWritesNothing(t *testing.T) {
	f := newFixture(t)
	before := f.count("procurement.rfq_publication_approvals")
	p, err := f.service().PreviewPrice(context.Background(), f.priceIn())
	must(t, err)
	if p.Total == nil || p.Total.LowerBoundAmount != 300 || p.Total.UpperBoundAmount != 600 || p.Total.Quantity != 3 || p.Total.Unit != "KG" {
		t.Fatalf("總額 = %+v", p.Total)
	}
	if p.LowerBoundAmount != 100 || p.UpperBoundAmount != 200 || p.BasisQuantity != 1 || p.BasisUnit != "KG" {
		t.Fatalf("單價區間或計價基準不對：%+v", p)
	}
	if after := f.count("procurement.rfq_publication_approvals"); after != before {
		t.Fatalf("純計算不寫入核准：%d → %d", before, after)
	}
	for _, gone := range []string{"procurement.price_previews", "procurement.confirmed_rfq_snapshots"} {
		var reg *string
		if err := f.d.Pool().QueryRow(context.Background(), `SELECT to_regclass($1)::text`, gone).Scan(&reg); err != nil || reg != nil {
			t.Fatalf("%s 已刪除，to_regclass = %v err = %v", gone, reg, err)
		}
	}
}

func TestPreviewPriceBasisAndTotalByRequirement(t *testing.T) {
	f := newFixture(t)
	f.revise(func(p *store.InsertRevisionParams) { p.Unit, p.Quantity = ptr("GRAM"), ptr(int32(250)) })
	gram, err := f.service().PreviewPrice(context.Background(), f.priceIn())
	must(t, err)
	if gram.BasisQuantity != 1000 || gram.BasisUnit != "GRAM" || gram.Total == nil || gram.Total.LowerBoundAmount != 25 || gram.Total.UpperBoundAmount != 50 {
		t.Fatalf("GRAM 要以每 1000 計價：%+v", gram)
	}
	f.revise(func(p *store.InsertRevisionParams) { p.Quantity = nil })
	none, err := f.service().PreviewPrice(context.Background(), f.priceIn())
	must(t, err)
	if none.Total != nil {
		t.Fatalf("需求沒有數量時不設總額：%+v", none.Total)
	}
}

func TestPreviewPriceBranches(t *testing.T) {
	ctx := context.Background()
	f := newFixture(t)

	stale := f.priceIn()
	stale.ExpectedRevisionID = uuid.NewV7()
	_, err := f.service().PreviewPrice(ctx, stale)
	wantConflict(t, err, "request_revision")

	for _, r := range map[string][2]int64{"zero lower": {0, 10}, "zero upper": {10, 0}, "negative": {-1, 10}, "inverted": {20, 10}} {
		in := f.priceIn()
		in.LowerBoundAmount, in.UpperBoundAmount = r[0], r[1]
		_, err := f.service().PreviewPrice(ctx, in)
		wantInvalid(t, err, "LowerBoundAmount")
	}
	both := stale
	both.LowerBoundAmount, both.UpperBoundAmount = 20, 10
	_, err = f.service().PreviewPrice(ctx, both)
	wantInvalid(t, err, "UpperBoundAmount")

	other := f.sibling(buyer + 1)
	foreign := f.priceIn()
	foreign.RequestID, foreign.ExpectedRevisionID = other.request, other.revision
	_, err = f.service().PreviewPrice(ctx, foreign)
	wantKind(t, err, apperr.KindNotFound)

	f.exec(`UPDATE procurement.conversations SET deleted_at = $1 WHERE request_id = $2`, f.now, f.request)
	_, err = f.service().PreviewPrice(ctx, f.priceIn())
	wantKind(t, err, apperr.KindNotFound)
}

func TestPreviewPriceRequiresAReadyOrMatchingRequest(t *testing.T) {
	f := newFixture(t)
	for _, status := range []string{"CLARIFYING"} {
		f.setStatus(status)
		_, err := f.service().PreviewPrice(context.Background(), f.priceIn())
		wantKind(t, err, apperr.KindFailedPrecondition)
		wantReason(t, err, apperr.ReasonRequirementNotReady, true)
	}
	f.setStatus("MATCHING")
	if _, err := f.service().PreviewPrice(context.Background(), f.priceIn()); err != nil {
		t.Fatalf("MATCHING 要收：%v", err)
	}
}

func TestPreviewPriceRequirementWithoutAUnitIsNotReady(t *testing.T) {
	f := newFixture(t)
	f.revise(func(p *store.InsertRevisionParams) { p.Unit = nil })
	_, err := f.service().PreviewPrice(context.Background(), f.priceIn())
	wantKind(t, err, apperr.KindFailedPrecondition)
	wantReason(t, err, apperr.ReasonRequirementNotReady, true)
}

func TestPreviewPriceOverflowIsAFailedPreconditionWithoutAReason(t *testing.T) {
	f := newFixture(t)
	in := f.priceIn()
	in.LowerBoundAmount, in.UpperBoundAmount = 1, math.MaxInt64
	_, err := f.service().PreviewPrice(context.Background(), in)
	wantKind(t, err, apperr.KindFailedPrecondition)
	wantReason(t, err, apperr.ReasonRequirementNotReady, false)
}
