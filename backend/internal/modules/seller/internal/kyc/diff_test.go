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
		if _, document := model.KYCFieldDocumentSlot(f); !document {
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
	name, other, code := "王小明", "王大明", "004"
	base := store.SellerKycApplication{SellerType: "INDIVIDUAL", RealName: &name, BankCode: &code, BankAccountEnc: []byte("x")}
	var p protector
	changed := func(next store.SellerKycApplication) (fields []string) {
		d, err := p.diffRows(base, next)
		if err != nil {
			t.Fatal(err)
		}
		for f, c := range d.fields {
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
	next.RealName = &other
	if f := changed(next); !slices.Equal(f, []string{"real_name"}) {
		t.Errorf("改姓名：%v", f)
	}
	next = base
	newCode := "005"
	next.BankCode = &newCode
	if f := changed(next); !slices.Equal(f, []string{"bank_code"}) {
		t.Errorf("只改銀行代碼（密文沒變）：%v", f)
	}
	next = base
	next.BankAccountEnc = nil
	if f := changed(next); !slices.Equal(f, []string{"bank_account"}) {
		t.Errorf("清掉銀行帳號：%v", f)
	}
}
