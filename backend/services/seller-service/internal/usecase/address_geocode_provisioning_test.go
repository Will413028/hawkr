package usecase

import (
	"context"
	"errors"
	"testing"

	pkgerrors "github.com/Will413028/hawkr/go-pkg/errors"
)

// backend#3329：地址查詢與金流商家建檔未開通時的行為契約。
//
// 期望值一律硬寫字面值（9700／9704 與訊息全文），不引用被測物的常數——
// 期望值跟著被測物一起變的話，這支測試什麼都驗不到。

// 🔴 陰性斷言：未開通時**不得**回「成功但空結果」。
func TestGeocodeDisabled_FailsClosedNotOpen(t *testing.T) {
	uc := NewAddressGeocodeUseCase(nil, nil)

	items, err := uc.Suggest(context.Background(), "臺北市", "大安區", "復興南路", 20)
	if err == nil || items != nil {
		t.Fatalf("未開通時 Suggest 必須 fail-closed，實際 items=%v err=%v", items, err)
	}
	assertSellerBizCode(t, err, 9700, "地址查詢服務尚未開通，請手動輸入完整地址")

	result, err := uc.Validate(context.Background(), "臺北市大安區復興南路一段1號")
	if err == nil || result != nil {
		t.Fatalf("未開通時 Validate 必須 fail-closed，實際 result=%v err=%v", result, err)
	}
	assertSellerBizCode(t, err, 9700, "地址查詢服務尚未開通，請手動輸入完整地址")
}

// 陽性對照：接上時不得被守門誤擋。少了這一格，「Suggest 永遠回 9700」也會全綠。
func TestGeocodeEnabled_NotBlockedByGuard(t *testing.T) {
	client := &fakeGeocodeClient{suggestResult: []Suggestion{{FullAddress: "臺北市大安區復興南路一段1號"}}}
	uc := NewAddressGeocodeUseCase(client, noopSellerGeocodeCache{})

	items, err := uc.Suggest(context.Background(), "臺北市", "大安區", "復興南路", 20)
	if err != nil {
		t.Fatalf("已開通時不該被守門擋下：%v", err)
	}
	if len(items) != 1 {
		t.Fatalf("已開通時應回 client 的結果，實際 %+v", items)
	}
}

// ErrPayuniMerchantDisabled 的碼與訊息是跨層契約（client 的停用實作回它、
// gateway 依 9704 映射 424），釘住字面值。
func TestPayuniMerchantDisabledError(t *testing.T) {
	assertSellerBizCode(t, ErrPayuniMerchantDisabled, 9704, "金流商家建檔服務尚未開通，請稍後再試")
}

func assertSellerBizCode(t *testing.T, err error, wantCode int, wantMsg string) {
	t.Helper()
	var de *pkgerrors.DomainError
	if !errors.As(err, &de) {
		t.Fatalf("必須是帶業務碼的 DomainError（否則 gateway 塌成 409 而不是 424），實際 %T：%v", err, err)
	}
	if got := de.DomainCode(); got != wantCode {
		t.Fatalf("業務碼應為 %d，實際 %d", wantCode, got)
	}
	if got := de.DomainMessage(); got != wantMsg {
		t.Fatalf("訊息應為 %q，實際 %q", wantMsg, got)
	}
}

type noopSellerGeocodeCache struct{}

func (noopSellerGeocodeCache) GetSuggestions(context.Context, string) ([]Suggestion, bool) {
	return nil, false
}
func (noopSellerGeocodeCache) SetSuggestions(context.Context, string, []Suggestion) {}
func (noopSellerGeocodeCache) GetValidation(context.Context, string) (*GeocodeValidation, bool) {
	return nil, false
}
func (noopSellerGeocodeCache) SetValidation(context.Context, string, *GeocodeValidation) {}
