package seller

import (
	"context"
	"errors"
	"maps"
	"slices"
	"strings"
	"testing"
	"time"

	"connectrpc.com/connect"

	pb "github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/seller/v1alpha1"
	"github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/seller/v1alpha1/sellerv1alpha1connect"
	"github.com/Will413028/hawkr/backend/internal/httpapi/rpc"
	"github.com/Will413028/hawkr/backend/internal/modules/identity"
	model "github.com/Will413028/hawkr/backend/internal/modules/seller"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
)

// 宣告表與 service 的方法一一對應；少一個宣告或多一個指向不存在的方法的宣告，都不能啟動。
func TestDeclarationsMatchTheServiceDescriptor(t *testing.T) {
	svc := serviceDescriptor()
	if err := checkDeclarations(declarations, svc); err != nil {
		t.Fatal(err)
	}
	if _, err := NewInterceptor(nil, &limiterFake{}); err != nil {
		t.Fatal(err)
	}
	if len(declarations) != svc.Methods().Len() {
		t.Fatalf("declarations=%d methods=%d", len(declarations), svc.Methods().Len())
	}

	for procedure := range declarations {
		table := maps.Clone(declarations)
		delete(table, procedure)
		if err := checkDeclarations(table, svc); err == nil || !strings.Contains(err.Error(), procedure+" 沒有授權宣告") {
			t.Errorf("移除 %s 的宣告：err=%v", procedure, err)
		}
	}

	stale := maps.Clone(declarations)
	stale["/hawkr.seller.v1alpha1.SellerAppService/NoSuchMethod"] = requires(model.CapStorefrontWrite)
	if err := checkDeclarations(stale, svc); err == nil || !strings.Contains(err.Error(), "NoSuchMethod 有宣告，但 service 沒有這個方法") {
		t.Errorf("多出不存在的方法：err=%v", err)
	}
}

// 每個宣告恰好是一個存在的能力或一個有理由的豁免。
func TestEachDeclarationIsExactlyOneCapabilityOrAnExemption(t *testing.T) {
	svc := serviceDescriptor()
	procedure := sellerv1alpha1connect.SellerAppServiceGetMyStoreProcedure
	for name, d := range map[string]declaration{
		"空宣告":     {},
		"能力又豁免":   {capability: model.CapAccountSelfRead, exemptReason: "理由"},
		"不存在的能力":  requires("CAP_NO_SUCH"),
		"店家的狀態":   requires(model.CapPublicListing),
		"沒有理由的豁免": exempt(""),
	} {
		table := maps.Clone(declarations)
		table[procedure] = d
		if err := checkDeclarations(table, svc); err == nil {
			t.Errorf("%s 通過了檢查", name)
		}
	}
}

type actingFake struct {
	calls []model.Capability
	store model.Store
	err   error
}

func (a *actingFake) ActingStore(_ context.Context, accountID int64, c model.Capability) (model.Store, error) {
	a.calls = append(a.calls, c)
	s := a.store
	s.AccountID = accountID
	return s, a.err
}

type fakeRequest struct {
	connect.AnyRequest
	procedure string
}

func (r fakeRequest) Spec() connect.Spec { return connect.Spec{Procedure: r.procedure} }

func accountContext(id int64) context.Context {
	return rpc.WithPrincipal(context.Background(), identity.Principal{Kind: identity.PrincipalAccount, ID: id})
}

// 需要能力的方法由 interceptor 以宣告的能力呼叫 ActingStore 並把店家放進 ctx；豁免的方法不查、ctx 裡沒有店家。
func TestInterceptorEnforcesEachDeclaration(t *testing.T) {
	for procedure, d := range declarations {
		t.Run(procedure, func(t *testing.T) {
			acting := &actingFake{store: model.Store{StoreID: 11}}
			limiter := &limiterFake{}
			i, err := NewInterceptor(acting, limiter)
			if err != nil {
				t.Fatal(err)
			}
			var seen *model.Store
			next := func(ctx context.Context, _ connect.AnyRequest) (connect.AnyResponse, error) {
				if s, err := storeFrom(ctx); err == nil {
					seen = &s
				}
				return nil, nil
			}
			if _, err := i.WrapUnary(next)(accountContext(7), fakeRequest{procedure: procedure}); err != nil {
				t.Fatal(err)
			}
			if d.exemptReason != "" {
				if len(acting.calls) != 0 || seen != nil {
					t.Fatalf("豁免的方法查了店家：calls=%v store=%v", acting.calls, seen)
				}
				return
			}
			if !slices.Equal(acting.calls, []model.Capability{d.capability}) || seen == nil || seen.StoreID != 11 || seen.AccountID != 7 {
				t.Fatalf("calls=%v store=%v", acting.calls, seen)
			}
		})
	}
}

// limiterFake 記下每次計數；deny 為 true 時一律拒絕。
type limiterFake struct {
	calls []limiterCall
	deny  bool
}

type limiterCall struct {
	key    string
	limit  int
	window time.Duration
}

func (l *limiterFake) Allow(limits ...Limit) (bool, time.Duration) {
	for _, lim := range limits {
		l.calls = append(l.calls, limiterCall{lim.Key, lim.Quota, lim.Window})
	}
	if l.deny {
		return false, 42 * time.Second
	}
	return true, 0
}

// 只有身分比對與對外查詢有每個帳號的額度，額度寫在宣告上；其他方法不計數。
func TestOnlyIdentityMatchingAndTheExternalLookupHaveAnAccountLimit(t *testing.T) {
	want := map[string]accountLimit{
		sellerv1alpha1connect.SellerAppServiceSubmitKYCProcedure:                  {5, time.Hour},
		sellerv1alpha1connect.SellerAppServiceLookupBusinessRegistrationProcedure: {10, time.Minute},
	}
	for procedure, d := range declarations {
		got := d.limit
		w, limited := want[procedure]
		if limited != (got != nil) || (limited && *got != w) {
			t.Errorf("%s 的額度 = %+v，預期 %+v", procedure, got, w)
		}
	}
}

// 有額度的宣告必須有 limiter，否則不能啟動；額度必須是正數。
func TestLimitedDeclarationsNeedALimiterAndPositiveQuotas(t *testing.T) {
	if _, err := NewInterceptor(&actingFake{}, nil); err == nil || !strings.Contains(err.Error(), "需要 limiter") {
		t.Fatalf("err=%v", err)
	}
	svc := serviceDescriptor()
	for name, d := range map[string]declaration{
		"次數為零":  requires(model.CapStorefrontWrite).limitedTo(0, time.Hour),
		"時間窗為零": requires(model.CapStorefrontWrite).limitedTo(1, 0),
	} {
		table := maps.Clone(declarations)
		table[sellerv1alpha1connect.SellerAppServiceSubmitKYCProcedure] = d
		if err := checkDeclarations(table, svc); err == nil {
			t.Errorf("%s 通過了檢查", name)
		}
	}
}

// 額度以帳號與方法計數，在能力檢查通過之後、handler 之前：超過回 resource_exhausted（帶 RetryInfo 的 wait），handler 不被
// 呼叫；沒有通過能力檢查的請求不消耗額度。
func TestInterceptorCountsPerAccountAfterTheCapabilityCheck(t *testing.T) {
	procedure := sellerv1alpha1connect.SellerAppServiceSubmitKYCProcedure
	acting, limiter := &actingFake{}, &limiterFake{}
	i, err := NewInterceptor(acting, limiter)
	if err != nil {
		t.Fatal(err)
	}
	called := 0
	next := func(context.Context, connect.AnyRequest) (connect.AnyResponse, error) {
		called++
		return nil, nil
	}
	if _, err := i.WrapUnary(next)(accountContext(7), fakeRequest{procedure: procedure}); err != nil {
		t.Fatal(err)
	}
	if _, err := i.WrapUnary(next)(accountContext(8), fakeRequest{procedure: procedure}); err != nil {
		t.Fatal(err)
	}
	want := []limiterCall{
		{"seller|" + procedure + "|7", 5, time.Hour},
		{"seller|" + procedure + "|8", 5, time.Hour},
	}
	if !slices.Equal(limiter.calls, want) || called != 2 {
		t.Fatalf("calls=%+v called=%d", limiter.calls, called)
	}

	limiter.deny = true
	_, err = i.WrapUnary(next)(accountContext(7), fakeRequest{procedure: procedure})
	var exhausted *apperr.Error
	if !errors.As(err, &exhausted) || apperr.KindOf(err) != apperr.KindResourceExhausted || called != 2 {
		t.Fatalf("err=%v called=%d", err, called)
	}
	if exhausted.RetryAfter != 42*time.Second {
		t.Fatalf("wait=%v", exhausted.RetryAfter)
	}

	// 能力檢查沒有通過：回能力的錯誤，額度不被消耗。
	denied := errors.New("suspended")
	blocked, counted := &actingFake{err: denied}, &limiterFake{}
	j, err := NewInterceptor(blocked, counted)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := j.WrapUnary(next)(accountContext(7), fakeRequest{procedure: procedure}); !errors.Is(err, denied) || len(counted.calls) != 0 {
		t.Fatalf("err=%v counted=%+v", err, counted.calls)
	}

	// 沒有額度的方法不計數。
	if _, err := i.WrapUnary(next)(accountContext(7), fakeRequest{procedure: sellerv1alpha1connect.SellerAppServiceGetMyKYCProcedure}); err != nil || len(limiter.calls) != 3 {
		t.Fatalf("err=%v calls=%d", err, len(limiter.calls))
	}
}

// 措施與申覆宣告為豁免：不呼叫 ActingStore，所以沒有店家、沒有賣家角色的帳號（所有店家都已關閉）照樣到得了 handler。
func TestAppealMethodsAreExemptFromTheSellerRoleAndTheStore(t *testing.T) {
	for _, procedure := range []string{
		sellerv1alpha1connect.SellerAppServiceListMyMeasuresProcedure,
		sellerv1alpha1connect.SellerAppServiceGetMyMeasureProcedure,
		sellerv1alpha1connect.SellerAppServiceSubmitAppealProcedure,
		sellerv1alpha1connect.SellerAppServiceRequestRemediationProcedure,
	} {
		d, ok := declarations[procedure]
		if !ok || d.exemptReason == "" || d.capability != "" {
			t.Errorf("%s 的宣告 = %+v，預期豁免", procedure, d)
			continue
		}
		acting := &actingFake{err: errors.New("沒有店家的帳號不該被查")}
		i, err := NewInterceptor(acting, &limiterFake{})
		if err != nil {
			t.Fatal(err)
		}
		called := false
		next := func(context.Context, connect.AnyRequest) (connect.AnyResponse, error) {
			called = true
			return nil, nil
		}
		if _, err := i.WrapUnary(next)(accountContext(7), fakeRequest{procedure: procedure}); err != nil || !called || len(acting.calls) != 0 {
			t.Errorf("%s：err=%v called=%v acting=%v", procedure, err, called, acting.calls)
		}
	}
}

// ActingStore 的錯誤原樣交回（由外層 interceptor 轉換），handler 不會被呼叫。
func TestInterceptorRejectsWithoutCallingTheHandler(t *testing.T) {
	boom := errors.New("denied")
	i, err := NewInterceptor(&actingFake{err: boom}, &limiterFake{})
	if err != nil {
		t.Fatal(err)
	}
	called := false
	next := func(context.Context, connect.AnyRequest) (connect.AnyResponse, error) {
		called = true
		return nil, nil
	}
	if _, err := i.WrapUnary(next)(accountContext(7), fakeRequest{procedure: sellerv1alpha1connect.SellerAppServiceUpdateMyStoreProcedure}); !errors.Is(err, boom) || called {
		t.Fatalf("err=%v called=%v", err, called)
	}
	// 沒有呼叫者（rpc 的授權已經擋下；這裡是授權鏈壞了）：不查店家，沒有分類的錯誤（internal），不是給使用者的訊息。
	if _, err := i.WrapUnary(next)(context.Background(), fakeRequest{procedure: sellerv1alpha1connect.SellerAppServiceGetMyStoreProcedure}); apperr.KindOf(err) != apperr.KindInternal || called {
		t.Fatalf("err=%v called=%v", err, called)
	}
	// 表裡沒有的 procedure 一律拒絕。
	if _, err := i.WrapUnary(next)(accountContext(7), fakeRequest{procedure: "/hawkr.seller.v1alpha1.SellerAppService/Nope"}); err == nil || called {
		t.Fatalf("err=%v called=%v", err, called)
	}
}

// 契約的列舉與模組的值域一一對應。
func TestEnumsMatchTheModule(t *testing.T) {
	all := model.AllCapabilities()
	names := make([]string, len(all))
	for i, c := range all {
		names[i] = strings.TrimPrefix(string(c), "CAP_")
	}
	if mismatches := rpc.EnumMismatches[pb.SellerCapability](names); len(mismatches) > 0 {
		t.Errorf("SellerCapability：%v", mismatches)
	}
	// 順序：模組依契約的數字順序列出（刪除的數字保留不重用，所以數字可以不連續，不能用位置推數字）。
	for i := 1; i < len(all); i++ {
		if capabilityPB(all[i-1]) >= capabilityPB(all[i]) {
			t.Errorf("能力 %s 與 %s 的順序和契約的數字順序不一致", all[i-1], all[i])
		}
	}
	for i, r := range model.Requirements() {
		if got := rpc.EnumToProto[pb.SellerRequirement](r); got != pb.SellerRequirement(i+1) {
			t.Errorf("條件 %s 對應到 %s，預期數字 %d（模組的順序就是契約的數字順序）", r, got, i+1)
		}
	}
	if got := len(pb.SellerRequirement_name) - 1; got != len(model.Requirements()) {
		t.Errorf("契約有 %d 個條件，模組有 %d 個", got, len(model.Requirements()))
	}
	for name, mismatches := range map[string][]string{
		"StoreStatus":       rpc.EnumMismatches[pb.StoreStatus](model.StatusValues()),
		"SellerRequirement": rpc.EnumMismatches[pb.SellerRequirement](model.Requirements()),
		"AgreementKind":     rpc.EnumMismatches[pb.AgreementKind](model.AgreementKinds()),
	} {
		if len(mismatches) > 0 {
			t.Errorf("%s：%v", name, mismatches)
		}
	}
}
