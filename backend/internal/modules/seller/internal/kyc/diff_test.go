package kyc

import (
	"slices"
	"testing"

	"github.com/Will413028/hawkr/backend/internal/modules/seller/internal/model"
	"github.com/Will413028/hawkr/backend/internal/modules/seller/internal/store"
)

// 寫入路徑能回報的欄位 = KYCFieldNames 扣掉文件類的欄位：新增 KYCField 沒接上 diff，這個測試就紅。
func TestFieldDiffsCoverEveryNonDocumentField(t *testing.T) {
	var want []string
	for _, f := range model.KYCFieldNames() {
		if !model.IsDocumentField(f) {
			want = append(want, f)
		}
	}
	var got []string
	for f := range fieldDiffs {
		got = append(got, f)
	}
	slices.Sort(want)
	slices.Sort(got)
	if !slices.Equal(got, want) {
		t.Fatalf("diff 回報的欄位 %v，預期 %v", got, want)
	}
}

func TestDiffRowsReportsOnlyTheFieldsThatChanged(t *testing.T) {
	name, other, tax, otherTax := "範例小農有限公司", "範例大農有限公司", "12345678", "23456781"
	base := store.SellerKycApplication{SellerType: "COMPANY", CompanyName: &name, TaxID: &tax}
	changed := func(next store.SellerKycApplication) (fields []string) {
		for f, c := range diffRows(base, next).fields {
			if c {
				fields = append(fields, f)
			}
		}
		slices.Sort(fields)
		return fields
	}
	if f := changed(base); len(f) != 0 {
		t.Errorf("沒改：%v", f)
	}
	next := base
	next.CompanyName = &other
	if f := changed(next); !slices.Equal(f, []string{"business.company_name"}) {
		t.Errorf("改公司名稱：%v", f)
	}
	// 統編直接比明文（D42）：改值算有變，清掉也算。
	next = base
	next.TaxID = &otherTax
	if f := changed(next); !slices.Equal(f, []string{"business.tax_id"}) {
		t.Errorf("改統編：%v", f)
	}
	next = base
	next.TaxID = nil
	if f := changed(next); !slices.Equal(f, []string{"business.tax_id"}) {
		t.Errorf("清掉統編：%v", f)
	}
}
