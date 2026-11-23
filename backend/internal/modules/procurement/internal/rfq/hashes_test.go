package rfq

import (
	"errors"
	"testing"
	"time"

	"uuid"

	"github.com/Will413028/hawkr/backend/internal/modules/procurement/internal/model"
	"github.com/Will413028/hawkr/backend/internal/modules/procurement/internal/rfqhash"
	"github.com/Will413028/hawkr/backend/internal/modules/procurement/internal/store"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
)

func ptr[T any](v T) *T { return &v }

var day = time.Date(2026, 12, 1, 0, 0, 0, 0, time.UTC)

func previewRow() store.ProcurementPricePreview {
	return store.ProcurementPricePreview{
		ID: uuid.NewV7(), RequestRevisionID: uuid.NewV7(), RangeLowerAmount: 100, RangeUpperAmount: 200, BasisQuantity: 1, BasisUnit: "KG",
		TotalLowerAmount: ptr(int64(300)), TotalUpperAmount: ptr(int64(600)), TotalQuantity: ptr(int32(3)), TotalUnit: ptr("KG"),
	}
}

func revisionRow() store.ProcurementRequestRevision {
	return store.ProcurementRequestRevision{
		ID: uuid.NewV7(), Item: ptr("小白菜"), Quantity: ptr(int32(3)), Unit: ptr("KG"), Packaging: ptr("箱"), BudgetPerUnit: ptr(int64(90)),
		DeliveryCity: ptr("台北市"), DeliveryDate: ptr(day), ExactMatchRequired: ptr(false),
	}
}

func snapshotRow() store.ProcurementConfirmedRfqSnapshot {
	return store.ProcurementConfirmedRfqSnapshot{ID: uuid.NewV7(), BuyerNote: "請盡快"}
}

// 每個存下的欄位都進 hash：少一個欄位，改那個欄位的值 hash 不變，測試就紅。
func TestPreviewHashCoversEveryStoredField(t *testing.T) {
	base := previewRow()
	want, err := PreviewHash(base)
	if err != nil {
		t.Fatal(err)
	}
	for name, mutate := range map[string]func(*store.ProcurementPricePreview){
		"id":          func(p *store.ProcurementPricePreview) { p.ID = uuid.NewV7() },
		"revision id": func(p *store.ProcurementPricePreview) { p.RequestRevisionID = uuid.NewV7() },
		"lower":       func(p *store.ProcurementPricePreview) { p.RangeLowerAmount++ },
		"upper":       func(p *store.ProcurementPricePreview) { p.RangeUpperAmount++ },
		"basis q":     func(p *store.ProcurementPricePreview) { p.BasisQuantity = 1000 },
		"basis u":     func(p *store.ProcurementPricePreview) { p.BasisUnit = "GRAM" },
		"total lower": func(p *store.ProcurementPricePreview) { p.TotalLowerAmount = ptr(int64(301)) },
		"total upper": func(p *store.ProcurementPricePreview) { p.TotalUpperAmount = ptr(int64(601)) },
		"total q":     func(p *store.ProcurementPricePreview) { p.TotalQuantity = ptr(int32(4)) },
		"total u":     func(p *store.ProcurementPricePreview) { p.TotalUnit = ptr("GRAM") },
	} {
		p := base
		mutate(&p)
		if got, _ := PreviewHash(p); got == want {
			t.Errorf("改 %s，預覽的 hash 沒變", name)
		}
	}
	if again, _ := PreviewHash(base); again != want {
		t.Error("同一列要同一個 hash")
	}
}

func TestSnapshotHashCoversEveryStoredField(t *testing.T) {
	snap, rev, prev := snapshotRow(), revisionRow(), previewRow()
	want, err := SnapshotHash(snap, rev, prev, rfqhash.SchemaV1)
	if err != nil {
		t.Fatal(err)
	}
	type rows struct {
		snap store.ProcurementConfirmedRfqSnapshot
		rev  store.ProcurementRequestRevision
		prev store.ProcurementPricePreview
	}
	for name, mutate := range map[string]func(*rows){
		"snapshot id": func(r *rows) { r.snap.ID = uuid.NewV7() },
		"buyer note":  func(r *rows) { r.snap.BuyerNote = "x" },
		"revision id": func(r *rows) { r.rev.ID = uuid.NewV7() },
		"item":        func(r *rows) { r.rev.Item = ptr("x") },
		"quantity":    func(r *rows) { r.rev.Quantity = ptr(int32(9)) },
		"unit":        func(r *rows) { r.rev.Unit = ptr("GRAM") },
		"packaging":   func(r *rows) { r.rev.Packaging = ptr("x") },
		"budget":      func(r *rows) { r.rev.BudgetPerUnit = ptr(int64(1)) },
		"city":        func(r *rows) { r.rev.DeliveryCity = ptr("x") },
		"date":        func(r *rows) { r.rev.DeliveryDate = ptr(day.AddDate(0, 0, 1)) },
		"exact match": func(r *rows) { r.rev.ExactMatchRequired = ptr(true) },
		"preview id":  func(r *rows) { r.prev.ID = uuid.NewV7() },
		"range lower": func(r *rows) { r.prev.RangeLowerAmount++ },
		"range upper": func(r *rows) { r.prev.RangeUpperAmount++ },
		"basis":       func(r *rows) { r.prev.BasisQuantity = 1000 },
		"total":       func(r *rows) { r.prev.TotalLowerAmount = nil },
	} {
		r := rows{snap, rev, prev}
		mutate(&r)
		if got, _ := SnapshotHash(r.snap, r.rev, r.prev, rfqhash.SchemaV1); got == want {
			t.Errorf("改 %s，快照的 hash 沒變", name)
		}
	}
}

func sellerBase() model.SellerFacing {
	return model.SellerFacing{
		Item: "小白菜", ItemLabel: "小白菜", Quantity: ptr(int32(3)), Unit: ptr("KG"), Packaging: ptr("箱"), DeliveryCity: "台北市",
		DeliveryDate: ptr(day), ResponseDate: ptr(day.AddDate(0, 0, -3)), BuyerNote: "請盡快",
		BasisQuantity: 1, BasisUnit: "KG", LowerBoundAmount: 100, UpperBoundAmount: 200,
	}
}

// 賣家看得到的每個欄位都進 hash：核准與執行用同一個對照，少一個欄位就能偷偷改掉那個欄位而 hash 不變。
func TestSellerHashCoversEveryPayloadField(t *testing.T) {
	want, err := rfqhash.SellerFacing(sellerInput(rfqhash.SchemaV1, sellerBase()))
	if err != nil {
		t.Fatal(err)
	}
	for name, mutate := range map[string]func(*model.SellerFacing){
		"item":        func(p *model.SellerFacing) { p.Item = "x" },
		"label":       func(p *model.SellerFacing) { p.ItemLabel = "x" },
		"quantity":    func(p *model.SellerFacing) { p.Quantity = nil },
		"unit":        func(p *model.SellerFacing) { p.Unit = ptr("GRAM") },
		"packaging":   func(p *model.SellerFacing) { p.Packaging = nil },
		"city":        func(p *model.SellerFacing) { p.DeliveryCity = "x" },
		"date":        func(p *model.SellerFacing) { p.DeliveryDate = nil },
		"deadline":    func(p *model.SellerFacing) { p.ResponseDate = ptr(day) },
		"note":        func(p *model.SellerFacing) { p.BuyerNote = "x" },
		"basis q":     func(p *model.SellerFacing) { p.BasisQuantity = 1000 },
		"basis u":     func(p *model.SellerFacing) { p.BasisUnit = "GRAM" },
		"lower bound": func(p *model.SellerFacing) { p.LowerBoundAmount++ },
		"upper bound": func(p *model.SellerFacing) { p.UpperBoundAmount++ },
	} {
		p := sellerBase()
		mutate(&p)
		if got, _ := rfqhash.SellerFacing(sellerInput(rfqhash.SchemaV1, p)); got == want {
			t.Errorf("改 %s，賣家可見內容的 hash 沒變", name)
		}
	}
}

func TestAudienceHashCoversEveryApprovalField(t *testing.T) {
	base := store.ProcurementRfqPublicationApproval{SchemaVersion: rfqhash.SchemaV1, AudienceMode: model.AudienceWishPool, AudiencePolicyID: ptr("P"), AudiencePolicyVersion: ptr(int32(1))}
	want, err := rfqhash.Audience(audienceInput(base, nil))
	if err != nil {
		t.Fatal(err)
	}
	for name, mutate := range map[string]func(*store.ProcurementRfqPublicationApproval){
		"mode":           func(a *store.ProcurementRfqPublicationApproval) { a.AudienceMode = model.AudienceDirectRecipients },
		"policy id":      func(a *store.ProcurementRfqPublicationApproval) { a.AudiencePolicyID = ptr("Q") },
		"policy version": func(a *store.ProcurementRfqPublicationApproval) { a.AudiencePolicyVersion = ptr(int32(2)) },
	} {
		a := base
		mutate(&a)
		if got, _ := rfqhash.Audience(audienceInput(a, nil)); got == want {
			t.Errorf("改 %s，受眾的 hash 沒變", name)
		}
	}
	if got, _ := rfqhash.Audience(audienceInput(base, []int64{1})); got == want {
		t.Error("多一家店，受眾的 hash 沒變")
	}
}

func TestAudienceShape(t *testing.T) {
	set, a, b := uuid.NewV7(), uuid.NewV7(), uuid.NewV7()
	zero := time.Time{}
	for _, tc := range []struct {
		name  string
		in    PreviewPublicationInput
		field string // 空字串表示合法
	}{
		{"wish pool", PreviewPublicationInput{AudienceMode: model.AudienceWishPool}, ""},
		{"wish pool with a set", PreviewPublicationInput{AudienceMode: model.AudienceWishPool, CandidateSetID: &set}, "CandidateSetID"},
		{"wish pool with candidates", PreviewPublicationInput{AudienceMode: model.AudienceWishPool, SelectedCandidateIDs: []uuid.UUID{a}}, "SelectedCandidateIDs"},
		{"direct", PreviewPublicationInput{AudienceMode: model.AudienceDirectRecipients, CandidateSetID: &set, SelectedCandidateIDs: []uuid.UUID{a, b}}, ""},
		{"direct without a set", PreviewPublicationInput{AudienceMode: model.AudienceDirectRecipients, SelectedCandidateIDs: []uuid.UUID{a}}, "CandidateSetID"},
		{"direct without candidates", PreviewPublicationInput{AudienceMode: model.AudienceDirectRecipients, CandidateSetID: &set}, "SelectedCandidateIDs"},
		{"direct with a duplicate", PreviewPublicationInput{AudienceMode: model.AudienceDirectRecipients, CandidateSetID: &set, SelectedCandidateIDs: []uuid.UUID{a, b, a}}, "SelectedCandidateIDs[2]"},
		{"unknown mode", PreviewPublicationInput{AudienceMode: "ALL"}, "AudienceMode"},
		{"empty mode", PreviewPublicationInput{}, "AudienceMode"},
		{"zero deadline", PreviewPublicationInput{AudienceMode: model.AudienceWishPool, ResponseDeadline: &zero}, "ResponseDeadline"},
	} {
		err := checkAudienceShape(tc.in)
		if tc.field == "" {
			if err != nil {
				t.Errorf("%s：不該有錯：%v", tc.name, err)
			}
			continue
		}
		var ae *apperr.Error
		if !errors.As(err, &ae) || ae.Kind() != apperr.KindInvalidInput || len(ae.FieldViolations()) != 1 || ae.FieldViolations()[0].Field != tc.field {
			t.Errorf("%s：要 INVALID_ARGUMENT 指向 %s，得到 %v", tc.name, tc.field, err)
		}
	}
}

func TestSameDate(t *testing.T) {
	later := day.Add(5 * time.Hour)
	for _, tc := range []struct {
		name       string
		stored, in *time.Time
		want       bool
	}{
		{"both nil", nil, nil, true}, {"stored only", &day, nil, false}, {"input only", nil, &day, false},
		{"same day other hour", &day, &later, true}, {"other day", &day, ptr(day.AddDate(0, 0, 1)), false},
	} {
		if got := sameDate(tc.stored, tc.in); got != tc.want {
			t.Errorf("%s：sameDate = %v", tc.name, got)
		}
	}
}
