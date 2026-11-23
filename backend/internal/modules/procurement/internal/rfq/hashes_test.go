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

func revisionRow() store.ProcurementRequestRevision {
	return store.ProcurementRequestRevision{
		ID: uuid.NewV7(), Item: ptr("小白菜"), ItemResolutionLabel: ptr("小白菜"), Quantity: ptr(int32(3)), Unit: ptr("KG"), Packaging: ptr("箱"), BudgetPerUnit: ptr(int64(90)),
		DeliveryCity: ptr("台北市"), DeliveryDate: ptr(day), ExactMatchRequired: ptr(false),
	}
}

func approvalRow() store.ProcurementRfqPublicationApproval {
	return store.ProcurementRfqPublicationApproval{
		ID: uuid.NewV7(), SchemaVersion: rfqhash.SchemaV1, AudienceMode: model.AudienceWishPool, AudiencePolicyID: ptr("P"), AudiencePolicyVersion: ptr(int32(1)),
		RangeLowerAmount: 100, RangeUpperAmount: 200, BuyerNote: "請盡快", ResponseDeadline: ptr(day.AddDate(0, 0, -3)),
	}
}

// 賣家看得到的每個欄位，不論來自核准凍結的欄位或需求版本，都進 hash：少一個欄位的對照，就能偷偷改掉那個欄位而 hash 不變。
func TestAssembledSellerHashCoversEveryApprovalAndRevisionField(t *testing.T) {
	base, err := assemble(approvalRow(), revisionRow(), compiled{})
	if err != nil {
		t.Fatal(err)
	}
	for name, mutate := range map[string]func(*store.ProcurementRfqPublicationApproval, *store.ProcurementRequestRevision){
		"approval lower": func(a *store.ProcurementRfqPublicationApproval, _ *store.ProcurementRequestRevision) {
			a.RangeLowerAmount++
		},
		"approval upper": func(a *store.ProcurementRfqPublicationApproval, _ *store.ProcurementRequestRevision) {
			a.RangeUpperAmount++
		},
		"approval note": func(a *store.ProcurementRfqPublicationApproval, _ *store.ProcurementRequestRevision) {
			a.BuyerNote = "x"
		},
		"approval deadline": func(a *store.ProcurementRfqPublicationApproval, _ *store.ProcurementRequestRevision) {
			a.ResponseDeadline = nil
		},
		"revision item": func(_ *store.ProcurementRfqPublicationApproval, r *store.ProcurementRequestRevision) {
			r.Item = ptr("x")
		},
		"revision label": func(_ *store.ProcurementRfqPublicationApproval, r *store.ProcurementRequestRevision) {
			r.ItemResolutionLabel = ptr("x")
		},
		"revision quantity": func(_ *store.ProcurementRfqPublicationApproval, r *store.ProcurementRequestRevision) {
			r.Quantity = ptr(int32(4))
		},
		"revision unit": func(_ *store.ProcurementRfqPublicationApproval, r *store.ProcurementRequestRevision) {
			r.Unit = ptr("GRAM")
		},
		"revision packaging": func(_ *store.ProcurementRfqPublicationApproval, r *store.ProcurementRequestRevision) {
			r.Packaging = nil
		},
		"revision city": func(_ *store.ProcurementRfqPublicationApproval, r *store.ProcurementRequestRevision) {
			r.DeliveryCity = ptr("x")
		},
		"revision date": func(_ *store.ProcurementRfqPublicationApproval, r *store.ProcurementRequestRevision) {
			r.DeliveryDate = ptr(day.AddDate(0, 0, 1))
		},
	} {
		a, r := approvalRow(), revisionRow()
		mutate(&a, &r)
		got, err := assemble(a, r, compiled{})
		if err != nil {
			t.Fatalf("%s：%v", name, err)
		}
		if got.sellerHash == base.sellerHash {
			t.Errorf("改 %s，賣家可見內容的 hash 沒變", name)
		}
	}
}

func TestAssembledSellerHashIgnoresFieldsTheSellerNeverSees(t *testing.T) {
	base, _ := assemble(approvalRow(), revisionRow(), compiled{})
	a, r := approvalRow(), revisionRow()
	r.BudgetPerUnit, r.ExactMatchRequired = ptr(int64(1)), ptr(true)
	a.ID, a.ExpiresAt = uuid.NewV7(), day
	got, _ := assemble(a, r, compiled{})
	if got.sellerHash != base.sellerHash {
		t.Error("買家自己的預算與精確比對、核准的 ID 與到期時間賣家看不到，不該進賣家可見內容的 hash")
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
