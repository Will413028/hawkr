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
	for _, f := range model.LockedIdentityFields() {
		if _, ok := fieldDiffs[f]; !ok {
			t.Errorf("身分欄位 %s 沒有 diff", f)
		}
	}
}

func TestDiffRowsReportsOnlyTheFieldsThatChanged(t *testing.T) {
	name, other, code := "王小明", "王大明", "004"
	base := store.SellerKycApplication{SellerType: "INDIVIDUAL", RealName: &name, BankCode: &code, BankAccountEnc: []byte("x")}
	var p protector
	changed := func(next store.SellerKycApplication) (fields []string, identity bool) {
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
		return fields, d.identity()
	}
	if f, id := changed(base); len(f) != 0 || id {
		t.Errorf("沒改：%v %v", f, id)
	}
	next := base
	next.RealName = &other
	if f, id := changed(next); !slices.Equal(f, []string{"real_name"}) || !id {
		t.Errorf("改姓名：%v %v", f, id)
	}
	next = base
	newCode := "005"
	next.BankCode = &newCode
	if f, id := changed(next); !slices.Equal(f, []string{"bank_code"}) || id {
		t.Errorf("只改銀行代碼（密文沒變）：%v %v", f, id)
	}
	next = base
	next.BankAccountEnc = nil
	if f, _ := changed(next); !slices.Equal(f, []string{"bank_account"}) {
		t.Errorf("清掉銀行帳號：%v", f)
	}
	next = base
	next.SellerType = "COMPANY"
	if f, id := changed(next); len(f) != 0 || !id {
		t.Errorf("改賣家型態：%v %v", f, id)
	}
}
