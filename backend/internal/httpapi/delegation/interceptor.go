package delegation

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"connectrpc.com/connect"

	"github.com/Will413028/hawkr/backend/internal/httpapi/rpc"
	moddelegation "github.com/Will413028/hawkr/backend/internal/modules/delegation"
)

// OperationSessionHeader 是代行 session 的憑證所在的請求標頭（website 的 BFF 轉送）。
const OperationSessionHeader = "X-HK-Operation-Session"

// Sessions 是以憑證解析代行 session 的來源（*delegation.Module）；activity 為真時順延閒置逾時。
type Sessions interface {
	ResolveDelegation(ctx context.Context, credential string, activity bool) (moddelegation.DelegationSession, error)
}

// Caller 是 interceptor 為這個請求驗證過的代行：session 與寫進稽核列的請求資料。
type Caller = moddelegation.DelegatedCaller

type callerKey struct{}

var errNoCaller = errors.New("httpapi/delegation：ctx 裡沒有代行 session，DelegationDelegatedService 的 handler 沒有掛 interceptor")

// callerFrom 回傳 interceptor 放進 ctx 的代行；沒有放（handler 被掛在沒有它的地方）回沒有分類的錯誤（internal），不放行。
func callerFrom(ctx context.Context) (Caller, error) {
	c, ok := ctx.Value(callerKey{}).(Caller)
	if !ok {
		return Caller{}, errNoCaller
	}
	return c, nil
}

// NewInterceptor 建立只掛在 DelegationDelegatedService 的 interceptor，要放在 rpc.NewInterceptor 之內，錯誤才會被轉換並記 log。流程：讀 X-HK-Operation-Session，
// 以 delegation.ResolveDelegation 解析（路由算活動，順延閒置逾時）；session 不是 ACTIVE（到期、已結束、已撤銷）拒絕；依 declarations 檢查 session 帶有該方法的能力；
// 通過才把 session 放進 ctx。憑證缺少或無效是 unauthenticated；其餘拒絕是 delegation 模組的代行拒絕（permission_denied 或 failed_precondition）。
// 目標店家只來自 session，請求沒有 store_id 可以指向別家。宣告與 service 的方法對不上時回錯誤，不能啟動。
func NewInterceptor(sessions Sessions) (connect.Interceptor, error) {
	return newInterceptor(declarations, sessions)
}

// newInterceptor 的宣告表由參數給（測試用缺一列的表驗證啟動檢查）。
func newInterceptor(table map[string]moddelegation.Capability, sessions Sessions) (connect.Interceptor, error) {
	if sessions == nil {
		return nil, errors.New("httpapi/delegation：NewInterceptor 需要 Sessions")
	}
	if err := checkDeclarations(table, serviceDescriptor()); err != nil {
		return nil, err
	}
	return &interceptor{table: table, sessions: sessions}, nil
}

type interceptor struct {
	table    map[string]moddelegation.Capability
	sessions Sessions
}

func (i *interceptor) enter(ctx context.Context, procedure string, header http.Header) (context.Context, error) {
	required, ok := i.table[procedure]
	if !ok {
		return ctx, fmt.Errorf("httpapi/delegation：%s 沒有能力宣告", procedure)
	}
	s, err := i.sessions.ResolveDelegation(ctx, header.Get(OperationSessionHeader), true)
	if err != nil {
		return ctx, err
	}
	if err := moddelegation.CheckDelegationSession(s, required); err != nil {
		return ctx, err
	}
	return context.WithValue(ctx, callerKey{}, Caller{Session: s, CorrelationID: rpc.TraceID(ctx), UserAgent: header.Get("User-Agent")}), nil
}

func (i *interceptor) WrapUnary(next connect.UnaryFunc) connect.UnaryFunc {
	return func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
		ctx, err := i.enter(ctx, req.Spec().Procedure, req.Header())
		if err != nil {
			return nil, err
		}
		return next(ctx, req)
	}
}

func (i *interceptor) WrapStreamingClient(next connect.StreamingClientFunc) connect.StreamingClientFunc {
	return next
}

// WrapStreamingHandler 拒絕串流：這個 service 只有 unary。
func (i *interceptor) WrapStreamingHandler(connect.StreamingHandlerFunc) connect.StreamingHandlerFunc {
	return func(_ context.Context, conn connect.StreamingHandlerConn) error {
		return fmt.Errorf("httpapi/delegation：%s 不支援串流", conn.Spec().Procedure)
	}
}
