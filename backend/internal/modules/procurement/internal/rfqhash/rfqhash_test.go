package rfqhash

import "testing"

func ptr[T any](v T) *T { return &v }

func base() Input {
	return Input{
		ConfirmedRfqSnapshotID: "s1", RequestRevisionID: "r1", PricePreviewID: "p1", BuyerNote: "請盡快",
		Requirement: Slots{Item: ptr("外套"), Quantity: ptr(int32(3)), Unit: ptr("件"), BudgetPerUnit: ptr(int64(500)), DeliveryDate: ptr("2026-12-01")},
		Price:       Price{RangeLowerAmount: 400, RangeUpperAmount: 600, BasisQuantity: 1, BasisUnit: "件", TotalLowerAmount: ptr(int64(1200)), TotalUpperAmount: ptr(int64(1800)), TotalQuantity: ptr(int32(3)), TotalUnit: ptr("件")},
	}
}

func TestSnapshotIsDeterministicAndPinned(t *testing.T) {
	a, err := Snapshot(base())
	if err != nil {
		t.Fatal(err)
	}
	b, _ := Snapshot(base())
	if a != b || len(a) != 64 {
		t.Fatalf("同輸入要同 hash：%q %q", a, b)
	}
	const golden = "0c5928787576017f44953621ecccae9df578d3b1be4c3e90d1bad0ca42b05035"
	if a != golden {
		t.Fatalf("golden 變了：%s", a)
	}
}

func TestEveryFieldChangesTheHash(t *testing.T) {
	want, _ := Snapshot(base())
	mutate := map[string]func(*Input){
		"snapshot id": func(i *Input) { i.ConfirmedRfqSnapshotID = "x" },
		"revision id": func(i *Input) { i.RequestRevisionID = "x" },
		"preview id":  func(i *Input) { i.PricePreviewID = "x" },
		"note":        func(i *Input) { i.BuyerNote = "x" },
		"item":        func(i *Input) { i.Requirement.Item = ptr("x") },
		"quantity":    func(i *Input) { i.Requirement.Quantity = ptr(int32(4)) },
		"unit":        func(i *Input) { i.Requirement.Unit = ptr("x") },
		"packaging":   func(i *Input) { i.Requirement.Packaging = ptr("x") },
		"budget":      func(i *Input) { i.Requirement.BudgetPerUnit = ptr(int64(1)) },
		"city":        func(i *Input) { i.Requirement.DeliveryCity = ptr("x") },
		"date":        func(i *Input) { i.Requirement.DeliveryDate = ptr("2026-12-02") },
		"exact":       func(i *Input) { i.Requirement.ExactMatchRequired = ptr(true) },
		"lower":       func(i *Input) { i.Price.RangeLowerAmount++ },
		"upper":       func(i *Input) { i.Price.RangeUpperAmount++ },
		"basis q":     func(i *Input) { i.Price.BasisQuantity++ },
		"basis u":     func(i *Input) { i.Price.BasisUnit = "x" },
		"total lower": func(i *Input) { i.Price.TotalLowerAmount = nil },
		"total upper": func(i *Input) { i.Price.TotalUpperAmount = ptr(int64(1)) },
		"total q":     func(i *Input) { i.Price.TotalQuantity = nil },
		"total u":     func(i *Input) { i.Price.TotalUnit = ptr("x") },
	}
	for name, f := range mutate {
		in := base()
		f(&in)
		got, _ := Snapshot(in)
		if got == want {
			t.Errorf("改 %s，hash 沒變", name)
		}
	}
}
