package rfqhash

import "testing"

func previewBase() PricePreviewInput {
	return PricePreviewInput{SchemaVersion: SchemaV1, PricePreviewID: "p1", RequestRevisionID: "r1", Price: base().Price}
}

func audienceBase() AudienceInput {
	return AudienceInput{SchemaVersion: SchemaV1, Mode: "DIRECT_RECIPIENTS", StoreIDs: []int64{7, 3}}
}

func sellerBase() SellerFacingInput {
	return SellerFacingInput{
		SchemaVersion: SchemaV1, Item: "外套", ItemLabel: "冬季外套", Quantity: ptr(int32(3)), Unit: ptr("KG"), Packaging: ptr("箱"),
		DeliveryCity: "台北市", DeliveryDate: ptr("2026-12-01"), ResponseDeadline: ptr("2026-11-30"), BuyerNote: "請盡快",
		Range: SellerRange{BasisQuantity: 1, BasisUnit: "KG", LowerBoundAmount: 400, UpperBoundAmount: 600},
	}
}

func TestPartsAreDeterministicAndPinned(t *testing.T) {
	for name, tc := range map[string]struct {
		got    func() (string, error)
		golden string
	}{
		"preview":  {func() (string, error) { return PricePreview(previewBase()) }, "7278b637860ab59d72bde91ef7955235877482fde01d7a8e64bf97e07abb3eef"},
		"audience": {func() (string, error) { return Audience(audienceBase()) }, "582d828451c11b3f941bf1bb7c80ff980113b5e6785e756a2553e7fa234f707d"},
		"seller":   {func() (string, error) { return SellerFacing(sellerBase()) }, "0a8a0889f0272ec086b0962e56dd5f86e271c6b490083f6430a8d0485762bcc2"},
	} {
		a, err := tc.got()
		if err != nil {
			t.Fatalf("%s：%v", name, err)
		}
		if b, _ := tc.got(); a != b || len(a) != 64 {
			t.Fatalf("%s：同輸入要同 hash：%q %q", name, a, b)
		}
		if a != tc.golden {
			t.Errorf("%s：golden 變了：%s", name, a)
		}
	}
}

func TestPartsRejectUnknownSchemaVersions(t *testing.T) {
	for _, v := range []string{"", "v0", "v2", "V1"} {
		p, a, s := previewBase(), audienceBase(), sellerBase()
		p.SchemaVersion, a.SchemaVersion, s.SchemaVersion = v, v, v
		if got, err := PricePreview(p); err == nil {
			t.Errorf("PricePreview %q 要回錯誤，得到 %s", v, got)
		}
		if got, err := Audience(a); err == nil {
			t.Errorf("Audience %q 要回錯誤，得到 %s", v, got)
		}
		if got, err := SellerFacing(s); err == nil {
			t.Errorf("SellerFacing %q 要回錯誤，得到 %s", v, got)
		}
	}
}

func TestEveryPreviewFieldChangesTheHash(t *testing.T) {
	want, _ := PricePreview(previewBase())
	for name, f := range map[string]func(*PricePreviewInput){
		"preview id":  func(i *PricePreviewInput) { i.PricePreviewID = "x" },
		"revision id": func(i *PricePreviewInput) { i.RequestRevisionID = "x" },
		"lower":       func(i *PricePreviewInput) { i.Price.RangeLowerAmount++ },
		"upper":       func(i *PricePreviewInput) { i.Price.RangeUpperAmount++ },
		"basis q":     func(i *PricePreviewInput) { i.Price.BasisQuantity++ },
		"basis u":     func(i *PricePreviewInput) { i.Price.BasisUnit = "x" },
		"total lower": func(i *PricePreviewInput) { i.Price.TotalLowerAmount = nil },
		"total upper": func(i *PricePreviewInput) { i.Price.TotalUpperAmount = ptr(int64(1)) },
		"total q":     func(i *PricePreviewInput) { i.Price.TotalQuantity = nil },
		"total u":     func(i *PricePreviewInput) { i.Price.TotalUnit = ptr("x") },
	} {
		in := previewBase()
		f(&in)
		if got, _ := PricePreview(in); got == want {
			t.Errorf("改 %s，hash 沒變", name)
		}
	}
}

func TestEveryAudienceFieldChangesTheHash(t *testing.T) {
	want, _ := Audience(audienceBase())
	for name, f := range map[string]func(*AudienceInput){
		"mode":           func(i *AudienceInput) { i.Mode = "WISH_POOL" },
		"policy id":      func(i *AudienceInput) { i.PolicyID = ptr("p") },
		"policy version": func(i *AudienceInput) { i.PolicyVersion = ptr(int32(1)) },
		"order":          func(i *AudienceInput) { i.StoreIDs = []int64{3, 7} },
		"one fewer":      func(i *AudienceInput) { i.StoreIDs = []int64{7} },
		"one more":       func(i *AudienceInput) { i.StoreIDs = []int64{7, 3, 9} },
		"another store":  func(i *AudienceInput) { i.StoreIDs = []int64{7, 4} },
	} {
		in := audienceBase()
		f(&in)
		if got, _ := Audience(in); got == want {
			t.Errorf("改 %s，hash 沒變", name)
		}
	}
	empty, nilIDs := audienceBase(), audienceBase()
	empty.StoreIDs, nilIDs.StoreIDs = []int64{}, nil
	a, _ := Audience(empty)
	b, _ := Audience(nilIDs)
	if a != b {
		t.Error("沒有對象時 nil 與空 slice 要同 hash")
	}
}

func TestEverySellerFacingFieldChangesTheHash(t *testing.T) {
	want, _ := SellerFacing(sellerBase())
	for name, f := range map[string]func(*SellerFacingInput){
		"item":      func(i *SellerFacingInput) { i.Item = "x" },
		"label":     func(i *SellerFacingInput) { i.ItemLabel = "x" },
		"quantity":  func(i *SellerFacingInput) { i.Quantity = ptr(int32(4)) },
		"unit":      func(i *SellerFacingInput) { i.Unit = nil },
		"packaging": func(i *SellerFacingInput) { i.Packaging = ptr("x") },
		"city":      func(i *SellerFacingInput) { i.DeliveryCity = "x" },
		"date":      func(i *SellerFacingInput) { i.DeliveryDate = ptr("2026-12-02") },
		"deadline":  func(i *SellerFacingInput) { i.ResponseDeadline = nil },
		"note":      func(i *SellerFacingInput) { i.BuyerNote = "x" },
		"basis q":   func(i *SellerFacingInput) { i.Range.BasisQuantity = 1000 },
		"basis u":   func(i *SellerFacingInput) { i.Range.BasisUnit = "x" },
		"lower":     func(i *SellerFacingInput) { i.Range.LowerBoundAmount++ },
		"upper":     func(i *SellerFacingInput) { i.Range.UpperBoundAmount++ },
	} {
		in := sellerBase()
		f(&in)
		if got, _ := SellerFacing(in); got == want {
			t.Errorf("改 %s，hash 沒變", name)
		}
	}
}

func TestHashKindsDoNotCollide(t *testing.T) {
	p, _ := PricePreview(previewBase())
	a, _ := Audience(audienceBase())
	s, _ := SellerFacing(sellerBase())
	n, _ := Snapshot(base())
	seen := map[string]bool{}
	for _, h := range []string{p, a, s, n} {
		if seen[h] {
			t.Fatalf("不同種的 hash 撞了：%s", h)
		}
		seen[h] = true
	}
}
