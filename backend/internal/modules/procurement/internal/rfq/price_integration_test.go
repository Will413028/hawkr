//go:build integration

package rfq_test

import (
	"context"
	"math"
	"strings"
	"testing"
	"time"

	"uuid"

	"github.com/Will413028/hawkr/backend/internal/modules/procurement/internal/rfq"
	"github.com/Will413028/hawkr/backend/internal/modules/procurement/internal/rfqhash"
	"github.com/Will413028/hawkr/backend/internal/modules/procurement/internal/store"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
)

func TestPreviewPriceStoresTheTotalAndItsHashIsRecomputableFromTheRow(t *testing.T) {
	f := newFixture(t)
	p := f.preview()
	if p.Total == nil || p.Total.LowerBoundAmount != 300 || p.Total.UpperBoundAmount != 600 || p.Total.Quantity != 3 || p.Total.Unit != "KG" {
		t.Fatalf("總額 = %+v", p.Total)
	}
	if p.BasisQuantity != 1 || p.BasisUnit != "KG" || !p.ExpiresAt.Equal(f.now.Add(rfq.PricePreviewTTL)) {
		t.Fatalf("計價基準或到期時間不對：%+v", p)
	}
	row, err := store.New(f.d.Conn(context.Background())).GetPricePreview(context.Background(), store.GetPricePreviewParams{ID: p.ID, RequestID: f.request, BuyerID: &f.buyer})
	must(t, err)
	want, err := rfq.PreviewHash(row)
	must(t, err)
	if p.PayloadHash != want || len(want) != 64 {
		t.Fatalf("hash %q，由列重算 %q", p.PayloadHash, want)
	}
}

func TestPreviewPriceBasisAndTotalByRequirement(t *testing.T) {
	f := newFixture(t)
	f.revise(func(p *store.InsertRevisionParams) { p.Unit, p.Quantity = ptr("GRAM"), ptr(int32(250)) })
	gram := f.preview()
	if gram.BasisQuantity != 1000 || gram.BasisUnit != "GRAM" || gram.Total == nil || gram.Total.LowerBoundAmount != 25 || gram.Total.UpperBoundAmount != 50 {
		t.Fatalf("GRAM 要以每 1000 計價：%+v", gram)
	}
	f.revise(func(p *store.InsertRevisionParams) { p.Quantity = nil })
	if p := f.preview(); p.Total != nil {
		t.Fatalf("需求沒有數量時不設總額：%+v", p.Total)
	}
}

func TestPreviewPriceReplaysTheFirstResultAndRejectsDifferentContent(t *testing.T) {
	f := newFixture(t)
	in := f.priceIn()
	first, err := f.service().PreviewPrice(context.Background(), in)
	must(t, err)
	f.now = f.now.Add(time.Minute)
	again, err := f.service().PreviewPrice(context.Background(), in)
	must(t, err)
	if again.ID != first.ID || again.PayloadHash != first.PayloadHash || !again.ExpiresAt.Equal(first.ExpiresAt) {
		t.Fatalf("重送要回第一次的預覽：%+v vs %+v", again, first)
	}
	if n := f.count("procurement.price_previews"); n != 1 {
		t.Fatalf("price_previews %d 列", n)
	}
	for name, edit := range map[string]func(*rfq.PreviewPriceInput){
		"range":    func(i *rfq.PreviewPriceInput) { i.UpperBoundAmount++ },
		"revision": func(i *rfq.PreviewPriceInput) { i.ExpectedRevisionID = uuid.NewV7() },
		"request":  func(i *rfq.PreviewPriceInput) { i.RequestID = uuid.NewV7() },
	} {
		changed := in
		edit(&changed)
		if _, err := f.service().PreviewPrice(context.Background(), changed); err == nil {
			t.Errorf("同一個鍵、不同的%s要回錯誤", name)
		} else {
			wantInvalid(t, err, "client_request_id")
		}
	}
}

func TestPreviewPriceBranches(t *testing.T) {
	ctx := context.Background()
	f := newFixture(t)

	stale := f.priceIn()
	stale.ExpectedRevisionID = uuid.NewV7()
	_, err := f.service().PreviewPrice(ctx, stale)
	wantConflict(t, err, "request_revision")

	for name, r := range map[string][2]int64{"zero lower": {0, 10}, "zero upper": {10, 0}, "negative": {-1, 10}, "inverted": {20, 10}} {
		in := f.priceIn()
		in.LowerBoundAmount, in.UpperBoundAmount = r[0], r[1]
		_, err := f.service().PreviewPrice(ctx, in)
		if apperr.KindOf(err) != apperr.KindInvalidInput {
			t.Errorf("%s：要 INVALID_ARGUMENT，得到 %v", name, err)
		}
	}
	if n := f.count("procurement.price_previews"); n != 0 {
		t.Fatalf("被拒的請求不寫入，price_previews %d 列", n)
	}

	other := f.sibling(buyer + 1)
	foreign := f.priceIn()
	foreign.RequestID, foreign.ExpectedRevisionID = other.request, other.revision
	_, err = f.service().PreviewPrice(ctx, foreign)
	wantKind(t, err, apperr.KindNotFound)

	f.exec(`UPDATE procurement.conversations SET deleted_at = $1 WHERE request_id = $2`, f.now, f.request)
	_, err = f.service().PreviewPrice(ctx, f.priceIn())
	wantKind(t, err, apperr.KindNotFound)
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
	if n := f.count("procurement.price_previews"); n != 0 {
		t.Fatalf("溢位不寫入，price_previews %d 列", n)
	}
}

func TestConfirmSnapshotStoresTheNoteAndItsHashIsRecomputableFromTheRows(t *testing.T) {
	f := newFixture(t)
	p := f.preview()
	in := f.confirmIn(p)
	in.BuyerNote = "  請盡快  "
	snap, err := f.service().ConfirmSnapshot(context.Background(), in)
	must(t, err)
	if !snap.ExpiresAt.Equal(f.now.Add(rfq.SnapshotTTL)) {
		t.Fatalf("到期時間 %v", snap.ExpiresAt)
	}
	q := store.New(f.d.Conn(context.Background()))
	row, err := q.GetConfirmedSnapshot(context.Background(), store.GetConfirmedSnapshotParams{ID: snap.ID, RequestID: f.request, BuyerID: &f.buyer})
	must(t, err)
	if row.BuyerNote != "請盡快" {
		t.Fatalf("補充說明要去頭尾空白：%q", row.BuyerNote)
	}
	rev, err := q.GetRevision(context.Background(), store.GetRevisionParams{ID: f.revision, RequestID: f.request, BuyerID: &f.buyer})
	must(t, err)
	prev, err := q.GetPricePreview(context.Background(), store.GetPricePreviewParams{ID: p.ID, RequestID: f.request, BuyerID: &f.buyer})
	must(t, err)
	want, err := rfq.SnapshotHash(row, rev, prev, rfqhash.SchemaV1)
	must(t, err)
	if snap.PayloadHash != want {
		t.Fatalf("hash %q，由列重算 %q", snap.PayloadHash, want)
	}
	if snap.PayloadHash == p.PayloadHash {
		t.Fatal("快照的 hash 與預覽的 hash 是兩種東西")
	}
}

func TestConfirmSnapshotNoteLimit(t *testing.T) {
	f := newFixture(t)
	p := f.preview()
	ok := f.confirmIn(p)
	ok.BuyerNote = strings.Repeat("字", rfq.MaxBuyerNoteRunes)
	if _, err := f.service().ConfirmSnapshot(context.Background(), ok); err != nil {
		t.Fatalf("剛好上限要收：%v", err)
	}
	long := f.confirmIn(p)
	long.BuyerNote = strings.Repeat("字", rfq.MaxBuyerNoteRunes+1)
	_, err := f.service().ConfirmSnapshot(context.Background(), long)
	wantInvalid(t, err, "BuyerNote")
	blank := f.confirmIn(p)
	blank.BuyerNote = "   "
	snap, err := f.service().ConfirmSnapshot(context.Background(), blank)
	must(t, err)
	if row, err := store.New(f.d.Conn(context.Background())).GetConfirmedSnapshot(context.Background(), store.GetConfirmedSnapshotParams{ID: snap.ID, RequestID: f.request, BuyerID: &f.buyer}); err != nil || row.BuyerNote != "" {
		t.Fatalf("只有空白的補充說明存成空字串：%+v %v", row, err)
	}
}

func TestConfirmSnapshotReplaysTheFirstResultAndRejectsDifferentContent(t *testing.T) {
	f := newFixture(t)
	p := f.preview()
	in := f.confirmIn(p)
	in.BuyerNote = "a"
	first, err := f.service().ConfirmSnapshot(context.Background(), in)
	must(t, err)
	f.now = f.now.Add(rfq.PricePreviewTTL + time.Minute) // 預覽已過期，重送仍回第一次的快照
	again, err := f.service().ConfirmSnapshot(context.Background(), in)
	must(t, err)
	if again != first {
		t.Fatalf("重送要回第一次的快照：%+v vs %+v", again, first)
	}
	if n := f.count("procurement.confirmed_rfq_snapshots"); n != 1 {
		t.Fatalf("confirmed_rfq_snapshots %d 列", n)
	}
	for name, edit := range map[string]func(*rfq.ConfirmInput){
		"note":     func(i *rfq.ConfirmInput) { i.BuyerNote = "b" },
		"preview":  func(i *rfq.ConfirmInput) { i.PricePreviewID = uuid.NewV7() },
		"revision": func(i *rfq.ConfirmInput) { i.ExpectedRevisionID = uuid.NewV7() },
		"request":  func(i *rfq.ConfirmInput) { i.RequestID = uuid.NewV7() },
	} {
		changed := in
		edit(&changed)
		_, err := f.service().ConfirmSnapshot(context.Background(), changed)
		if err == nil {
			t.Errorf("同一個鍵、不同的%s要回錯誤", name)
			continue
		}
		wantInvalid(t, err, "client_request_id")
	}
}

func TestConfirmSnapshotStaleBranches(t *testing.T) {
	ctx := context.Background()
	f := newFixture(t)
	p := f.preview()

	stale := f.confirmIn(p)
	stale.ExpectedRevisionID = uuid.NewV7()
	_, err := f.service().ConfirmSnapshot(ctx, stale)
	wantConflict(t, err, "request_revision")

	hash := f.confirmIn(p)
	hash.PayloadHash = p.PayloadHash[:63] + "0"
	if hash.PayloadHash == p.PayloadHash {
		hash.PayloadHash = p.PayloadHash[:63] + "1"
	}
	_, err = f.service().ConfirmSnapshot(ctx, hash)
	wantConflict(t, err, "price_preview")

	missing := f.confirmIn(p)
	missing.PricePreviewID = uuid.NewV7()
	_, err = f.service().ConfirmSnapshot(ctx, missing)
	wantKind(t, err, apperr.KindNotFound)

	foreign := f.sibling(buyer + 1)
	theirs := foreign.preview()
	notMine := f.confirmIn(theirs)
	_, err = f.service().ConfirmSnapshot(ctx, notMine)
	wantKind(t, err, apperr.KindNotFound)

	again := f.sibling(buyer) // 同一位買家的另一張需求的預覽：過時，不是 NOT_FOUND
	otherRequest := again.preview()
	crossed := f.confirmIn(otherRequest)
	_, err = f.service().ConfirmSnapshot(ctx, crossed)
	wantConflict(t, err, "price_preview")

	f.now = f.now.Add(rfq.PricePreviewTTL + time.Second)
	_, err = f.service().ConfirmSnapshot(ctx, f.confirmIn(p))
	wantConflict(t, err, "price_preview")
	if n := f.count("procurement.confirmed_rfq_snapshots"); n != 0 {
		t.Fatalf("被拒的請求不寫入，confirmed_rfq_snapshots %d 列", n)
	}
}

func TestConfirmSnapshotRejectsAPreviewMadeForAnEarlierRevision(t *testing.T) {
	f := newFixture(t)
	p := f.preview()
	f.revise(func(*store.InsertRevisionParams) {}) // 預覽之後需求又改了一版，買家帶著新版本與舊預覽來確認
	_, err := f.service().ConfirmSnapshot(context.Background(), f.confirmIn(p))
	wantConflict(t, err, "price_preview")
}
