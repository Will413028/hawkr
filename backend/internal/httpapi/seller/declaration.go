package seller

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strconv"
	"time"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/reflect/protoreflect"

	pb "github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/seller/v1alpha1"
	"github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/seller/v1alpha1/sellerv1alpha1connect"
	"github.com/Will413028/hawkr/backend/internal/httpapi/rpc"
	model "github.com/Will413028/hawkr/backend/internal/modules/seller"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
)

// declaration 是 SellerAppService 一個方法的授權宣告（ADR 2026-09-29-seller-suspension-and-business-identity-in-monolith，
// 沿用 2026-09-18-seller-capability-authorization 的 D1「宣告＝掛載」）：要不然需要一個賣家能力，interceptor 以
// seller.ActingStore 檢查並把店家放進 ctx；要不然明列豁免與理由，interceptor 不查店家，方法也不要求賣家角色。
type declaration struct {
	capability model.Capability
	// exemptReason 非空表示豁免。
	exemptReason string
	// limit 非 nil 表示每個帳號對這個方法的呼叫有額度，由 interceptor 在能力檢查通過之後、交給 handler 之前計數。
	limit *accountLimit
}

// accountLimit 是每個帳號在一個時間窗內對一個方法的呼叫上限。
type accountLimit struct {
	quota  int
	window time.Duration
}

func requires(c model.Capability) declaration { return declaration{capability: c} }

func exempt(reason string) declaration { return declaration{exemptReason: reason} }

// limitedTo 為宣告加上每個帳號的額度：身分比對與對外查詢不能成為探測器，計數以帳號為身分（入口的 IP 限流擋不住
// 換位址的同一個帳號，也會誤傷共用位址的人）。
func (d declaration) limitedTo(quota int, window time.Duration) declaration {
	d.limit = &accountLimit{quota: quota, window: window}
	return d
}

// declarations 是每個 SellerAppService 方法的宣告，key 是生成的 procedure 常數。方法與宣告必須一一對應
// （checkDeclarations 在 NewInterceptor 時檢查，缺了或多了都不能啟動）。
var declarations = map[string]declaration{
	sellerv1alpha1connect.SellerAppServiceApplyStoreProcedure:                exempt("申請開店時帳號還沒有店家，也還不是賣家"),
	sellerv1alpha1connect.SellerAppServiceGetMyStoreOpeningProcedure:         exempt("開店入口在送出之前讀取，帳號還沒有店家，也還不是賣家；帳號的措施與生命週期由模組以 account ID 判斷"),
	sellerv1alpha1connect.SellerAppServiceGetMyStoreProcedure:                requires(model.CapAccountSelfRead),
	sellerv1alpha1connect.SellerAppServiceUpdateMyStoreProcedure:             requires(model.CapStorefrontWrite),
	sellerv1alpha1connect.SellerAppServiceCompleteMyStoreOnboardingProcedure: requires(model.CapStorefrontWrite),
	sellerv1alpha1connect.SellerAppServiceListMyFollowersProcedure:           requires(model.CapMarketing),
	sellerv1alpha1connect.SellerAppServiceGetMyDashboardProcedure:            requires(model.CapMarketing),
	sellerv1alpha1connect.SellerAppServiceCloseMyStoreProcedure:              requires(model.CapAccountClose),

	// KYC 與 PayUni：模組對同一個能力再檢查一次（深度防禦）。草稿、送出與取消是店面設定的一部分，停權與等待刪除時不能做；
	// 讀取自己的申請（含 PayUni 狀態）停權時仍可以。
	sellerv1alpha1connect.SellerAppServiceSaveKYCDraftProcedure: requires(model.CapStorefrontWrite),
	sellerv1alpha1connect.SellerAppServiceGetMyKYCProcedure:     requires(model.CapAccountSelfRead),
	sellerv1alpha1connect.SellerAppServiceCancelKYCProcedure:    requires(model.CapStorefrontWrite),
	// 身分比對是身分證號與統編存在與否的探測面（ADR 2026-10-19-seller-kyc-payuni-consolidated）：每個帳號每小時最多 5 次。
	// 一次正常的申請是填完才送出，被退回後修改重送，一個小時內不會超過；額度以帳號計，計數在這個程序的記憶體內：多個
	// instance 各自計數，monolith 跑超過一個 instance 之前要換成共用的計數（CARRYOVER.md「已知限制」；入口的 IP 限流不是它的補償）。
	sellerv1alpha1connect.SellerAppServiceSubmitKYCProcedure: requires(model.CapStorefrontWrite).limitedTo(5, time.Hour),
	// 統編查詢是 KYC 表單的輔助（先有店家才填表單），與 SaveKYCDraft 同一類；它代替使用者呼叫政府的公開 API，每個帳號每分鐘
	// 最多 10 次（輸入框打到第 8 碼才查，改錯重查仍在額度內）。
	sellerv1alpha1connect.SellerAppServiceLookupBusinessRegistrationProcedure: requires(model.CapStorefrontWrite).limitedTo(10, time.Minute),

	// 措施與申覆以帳號為範圍：措施的效果以帳號為單位、關店不解除，所有店家都已關閉的帳號照樣受處分，必須還能查措施與申覆，
	// 所以不要求賣家角色或店家（不呼叫 ActingStore），帳號的歸屬由模組以 account ID 檢查，別人的措施回 not_found。
	sellerv1alpha1connect.SellerAppServiceListMyMeasuresProcedure: exempt(appealReason),
	sellerv1alpha1connect.SellerAppServiceGetMyMeasureProcedure:   exempt(appealReason),
	sellerv1alpha1connect.SellerAppServiceSubmitAppealProcedure:   exempt(appealReason),

	sellerv1alpha1connect.SellerAppServiceRequestRemediationProcedure: exempt(appealReason),
}

const appealReason = "措施、申覆與解除請求以帳號為範圍，所有店家都已關閉的帳號仍受措施、必須還能申覆與請求解除"

// serviceDescriptor 是 SellerAppService 的 descriptor，宣告表以它為準。
func serviceDescriptor() protoreflect.ServiceDescriptor {
	return pb.File_hawkr_seller_v1alpha1_seller_proto.Services().ByName("SellerAppService")
}

// checkDeclarations 雙向對照宣告表與 service 的方法（rpc.CheckDeclarations），並檢查每個宣告恰好是一個存在的能力或一個
// 有理由的豁免。
func checkDeclarations(table map[string]declaration, svc protoreflect.ServiceDescriptor) error {
	return rpc.CheckDeclarations("httpapi/seller", table, []protoreflect.ServiceDescriptor{svc}, func(d declaration) string {
		switch {
		case (d.capability == "") == (d.exemptReason == ""):
			return "的宣告必須是一個能力或一個有理由的豁免"
		case d.capability == model.CapPublicListing:
			return "宣告了 " + string(d.capability) + "：它是店家的狀態不是動作，沒有任何方法可以需要它"
		case d.capability != "" && !slices.Contains(model.Capabilities(), d.capability):
			return "宣告了不存在的能力 " + string(d.capability)
		case d.limit != nil && (d.limit.quota <= 0 || d.limit.window <= 0):
			return "的額度必須是正數的次數與時間窗"
		}
		return ""
	})
}

// Acting 是 interceptor 讀取呼叫者目前店家的介面（seller.Module.ActingStore）。handler 不使用它：店家只從 ctx 取。
type Acting interface {
	ActingStore(ctx context.Context, accountID int64, c model.Capability) (model.Store, error)
}

type storeKey struct{}

// storeFrom 回傳 interceptor 為這個請求檢查過能力的店家。宣告為豁免的方法沒有店家；需要店家的 handler 拿不到時是
// 宣告與 handler 對不上，回沒有分類的錯誤（internal），不放行。
func storeFrom(ctx context.Context) (model.Store, error) {
	s, ok := ctx.Value(storeKey{}).(model.Store)
	if !ok {
		return model.Store{}, errors.New("httpapi/seller：這個方法沒有宣告需要賣家能力，ctx 裡沒有店家")
	}
	return s, nil
}

// Limit 是一次計數：Key 在 Window 內最多 Quota 次。同一個 Key 每次要用同一個時間窗。
type Limit struct {
	Key    string
	Quota  int
	Window time.Duration
}

// Limiter 是每個帳號的額度計數：一個請求的一或多個 Limit 全部在額度內才一起記一次，任何一個超過就都不記，回傳最久的等待時間。
type Limiter interface {
	Allow(limits ...Limit) (ok bool, wait time.Duration)
}

// NewInterceptor 建立只掛在 SellerAppService 的 interceptor，要放在 rpc.NewInterceptor 之後（內層），錯誤才會被轉換並
// 記 log。每個請求依宣告表處理：需要能力的方法以 acting.ActingStore 檢查呼叫者目前的店家（停權、等待刪除、沒有店家都
// 在這裡被擋下，fail-closed）並把店家放進 ctx；豁免的方法不查。有額度的方法在能力檢查通過之後以 limiter 依帳號計數，
// 超過回 resource_exhausted（帶 RetryInfo），handler 不會被呼叫。宣告表與 service 的方法對不上、或有額度的宣告卻沒有
// limiter 時回錯誤。
func NewInterceptor(acting Acting, limiter Limiter) (connect.Interceptor, error) {
	if err := checkDeclarations(declarations, serviceDescriptor()); err != nil {
		return nil, err
	}
	if limiter == nil {
		for procedure, d := range declarations {
			if d.limit != nil {
				return nil, fmt.Errorf("httpapi/seller：%s 宣告了每個帳號的額度，需要 limiter", procedure)
			}
		}
	}
	return &interceptor{acting: acting, limiter: limiter, table: declarations}, nil
}

type interceptor struct {
	acting  Acting
	limiter Limiter
	table   map[string]declaration
}

// enter 依 procedure 的宣告回傳要交給 handler 的 ctx。
func (i *interceptor) enter(ctx context.Context, procedure string) (context.Context, error) {
	d, ok := i.table[procedure]
	if !ok {
		return ctx, fmt.Errorf("httpapi/seller：%s 沒有授權宣告", procedure)
	}
	p, ok := rpc.PrincipalFrom(ctx)
	if d.exemptReason != "" {
		if d.limit != nil && !ok {
			return ctx, errNoPrincipal
		}
		return ctx, i.count(p.ID, procedure, d)
	}
	if !ok {
		return ctx, errNoPrincipal
	}
	s, err := i.acting.ActingStore(ctx, p.ID, d.capability)
	if err != nil {
		return ctx, err
	}
	if err := i.count(p.ID, procedure, d); err != nil {
		return ctx, err
	}
	return context.WithValue(ctx, storeKey{}, s), nil
}

// count 為有額度的方法計一次；沒有額度的方法不計。
func (i *interceptor) count(accountID int64, procedure string, d declaration) error {
	if d.limit == nil {
		return nil
	}
	key := "seller|" + procedure + "|" + strconv.FormatInt(accountID, 10)
	if ok, wait := i.limiter.Allow(Limit{Key: key, Quota: d.limit.quota, Window: d.limit.window}); !ok {
		return apperr.ResourceExhausted(wait, "操作太頻繁，請稍後再試")
	}
	return nil
}

func (i *interceptor) WrapUnary(next connect.UnaryFunc) connect.UnaryFunc {
	return func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
		ctx, err := i.enter(ctx, req.Spec().Procedure)
		if err != nil {
			return nil, err
		}
		return next(ctx, req)
	}
}

func (i *interceptor) WrapStreamingClient(next connect.StreamingClientFunc) connect.StreamingClientFunc {
	return next
}

func (i *interceptor) WrapStreamingHandler(next connect.StreamingHandlerFunc) connect.StreamingHandlerFunc {
	return func(ctx context.Context, conn connect.StreamingHandlerConn) error {
		ctx, err := i.enter(ctx, conn.Spec().Procedure)
		if err != nil {
			return err
		}
		return next(ctx, conn)
	}
}
