package model

import (
	"reflect"
	"strings"
	"testing"

	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
)

// CD1：值域只有 NORMAL 與 VIP；BLOCKLIST（含舊的小寫）、空字串與任意字串都不是分類。
func TestCustomerTypeDomain(t *testing.T) {
	t.Parallel()
	for _, ok := range []CustomerType{TypeNormal, TypeVIP} {
		if !ok.Valid() {
			t.Errorf("%q 應合法", ok)
		}
	}
	for _, bad := range []CustomerType{"", "BLOCKLIST", "blocklist", "normal", "vip", "Vip", "SUPER", " VIP"} {
		if bad.Valid() {
			t.Errorf("%q 不應合法", bad)
		}
	}
}

// CD8：Customer 型別沒有任何聯絡或個資欄位（A6-D8：CRM 只顯示暱稱與訂單統計；暱稱由 httpapi 經 buyer 補）。
func TestCustomerHasNoContactFields(t *testing.T) {
	t.Parallel()
	typ := reflect.TypeOf(Customer{})
	want := []string{"StoreID", "BuyerID", "Type", "TotalSpent", "OrderCount", "FirstOrderAt", "Tags"}
	var got []string
	for i := 0; i < typ.NumField(); i++ {
		got = append(got, typ.Field(i).Name)
		n := strings.ToLower(typ.Field(i).Name)
		for _, banned := range []string{"phone", "email", "address", "nickname", "name"} {
			if strings.Contains(n, banned) {
				t.Errorf("Customer 出現個資欄位 %s", typ.Field(i).Name)
			}
		}
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Customer 欄位 %v，預期 %v（新增欄位要先確認不是個資，再改這個清單）", got, want)
	}
}

func TestSortAndOutcomeDomain(t *testing.T) {
	t.Parallel()
	if !SortFirstOrderAt.Valid() || !SortTotalSpent.Valid() || !SortOrderCount.Valid() || SortBy(3).Valid() || SortBy(-1).Valid() {
		t.Fatal("SortBy 值域錯誤")
	}
	if !RefundPartial.Valid() || !RefundFull.Valid() || RefundOutcome("").Valid() || RefundOutcome("REFUND").Valid() {
		t.Fatal("RefundOutcome 值域錯誤")
	}
}

func TestReasonOfSurvivesWrapping(t *testing.T) {
	t.Parallel()
	err := WithReason(apperr.NotFound("x"), ReasonTagNotFound)
	if ReasonOf(err) != ReasonTagNotFound || apperr.KindOf(err) != apperr.KindNotFound {
		t.Fatalf("reason=%q kind=%v", ReasonOf(err), apperr.KindOf(err))
	}
	if ReasonOf(apperr.NotFound("x")) != "" {
		t.Fatal("沒有原因時應為空字串")
	}
}
