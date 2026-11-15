// Package delegation 是 `<模組>DelegatedService` 共用的傳輸層（API 設計規範（Connect）§二的 Delegated）：把請求標頭 X-HK-Operation-Session 的代行 credential 解成 session，
// 依該 service 自己的能力宣告表授權並稽核被拒絕的請求，再把驗證過的代行放進 ctx。每個 service 掛一支自己的 interceptor、帶自己的宣告表；資源的讀寫與回應組裝在資源模組的
// httpapi 套件（社群是 httpapi/social）。
package delegation

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/reflect/protoreflect"

	"github.com/Will413028/hawkr/backend/internal/httpapi/rpc"
	moddelegation "github.com/Will413028/hawkr/backend/internal/modules/delegation"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
)

// OperationSessionHeader 是代行 session 的憑證所在的請求標頭（website 的 BFF 轉送）。
const OperationSessionHeader = "X-HK-Operation-Session"

// Sessions 是 interceptor 需要的 delegation 公開 API（*delegation.Module）：以憑證解析代行 session（activity 為真時順延閒置逾時；interceptor 先唯讀解析，授權通過才以 activity 為真再解析一次），與授權（被拒絕時寫 FAILED 稽核）。
type Sessions interface {
	ResolveDelegation(ctx context.Context, credential string, activity bool) (moddelegation.DelegationSession, error)
	AuthorizeDelegated(ctx context.Context, c moddelegation.DelegatedCaller) error
}

// Declaration 是一個 `<模組>DelegatedService` 的能力宣告：service 的 descriptor，與每個方法（key 是生成的 procedure 常數）需要的代行能力、稽核動作與資源種類。
// 方法與宣告必須一一對應（NewInterceptor 時檢查，缺了或多了都不能啟動）。沒有豁免：這個類別的每個方法都以代行 session 為憑證。
type Declaration struct {
	Service protoreflect.ServiceDescriptor
	Methods map[string]moddelegation.DelegatedMethod
}

func (d Declaration) check() error {
	return rpc.CheckDeclarations("httpapi/delegation", d.Methods, []protoreflect.ServiceDescriptor{d.Service}, func(m moddelegation.DelegatedMethod) string {
		switch {
		case !m.Capability.Valid():
			return "宣告了不存在的能力 " + string(m.Capability)
		case m.Action == "" || m.Resource == "":
			return "必須宣告稽核的動作與資源種類"
		}
		return ""
	})
}

// Caller 是 interceptor 為這個請求驗證過的代行：session、被呼叫的方法宣告與寫進稽核列的請求資料。
type Caller = moddelegation.DelegatedCaller

type callerKey struct{}

var errNoCaller = errors.New("httpapi/delegation：ctx 裡沒有代行 session，DelegatedService 的 handler 沒有掛 interceptor")

// CallerFrom 回傳 interceptor 放進 ctx 的代行；沒有放（handler 被掛在沒有它的地方）回沒有分類的錯誤（internal），不放行。
func CallerFrom(ctx context.Context) (Caller, error) {
	c, ok := ctx.Value(callerKey{}).(Caller)
	if !ok {
		return Caller{}, errNoCaller
	}
	return c, nil
}

// NewInterceptor 建立只掛在 decl 那個 service 的 interceptor，要放在 rpc.NewInterceptor 之內，錯誤才會被轉換並記 log。流程：讀 X-HK-Operation-Session，
// 以 ResolveDelegation 解析（唯讀，不算活動）；解析不到 session（缺少或無效的憑證）回 unauthenticated，不寫稽核列（沒有 session 可歸屬），只記一筆結構化的安全 log
// （procedure 與 trace，不含憑證）；解析到之後以 AuthorizeDelegated 授權：session 不是 ACTIVE（到期、已結束、已撤銷）或沒有該方法的能力被拒絕，拒絕另寫一列 FAILED 稽核；
// 通過才再以 ResolveDelegation 順延閒置逾時（被拒絕的請求不算活動），並把代行放進 ctx。目標店家只來自 session，請求沒有 store_id 可以指向別家。
func NewInterceptor(sessions Sessions, logger *slog.Logger, decl Declaration) (connect.Interceptor, error) {
	if sessions == nil {
		return nil, errors.New("httpapi/delegation：NewInterceptor 需要 Sessions")
	}
	if logger == nil {
		return nil, errors.New("httpapi/delegation：NewInterceptor 需要 Logger")
	}
	if err := decl.check(); err != nil {
		return nil, err
	}
	return &interceptor{table: decl.Methods, sessions: sessions, logger: logger}, nil
}

type interceptor struct {
	table    map[string]moddelegation.DelegatedMethod
	sessions Sessions
	logger   *slog.Logger
}

func (i *interceptor) enter(ctx context.Context, procedure string, header http.Header) (context.Context, error) {
	method, ok := i.table[procedure]
	if !ok {
		return ctx, fmt.Errorf("httpapi/delegation：%s 沒有能力宣告", procedure)
	}
	credential := header.Get(OperationSessionHeader)
	s, err := i.sessions.ResolveDelegation(ctx, credential, false)
	if err != nil {
		if apperr.KindOf(err) == apperr.KindUnauthenticated {
			i.logger.WarnContext(ctx, "代行憑證解析不到 session", "security_event", "delegation_credential_unresolved",
				"procedure", procedure, "credential_present", credential != "", "trace_id", rpc.TraceID(ctx), "user_agent", header.Get("User-Agent"))
		}
		return ctx, err
	}
	caller := Caller{Session: s, Method: method, CorrelationID: rpc.TraceID(ctx), UserAgent: header.Get("User-Agent")}
	if err := i.sessions.AuthorizeDelegated(ctx, caller); err != nil {
		return ctx, err
	}
	// 授權通過才算活動：被拒絕的請求不順延閒置逾時。
	if _, err := i.sessions.ResolveDelegation(ctx, credential, true); err != nil {
		return ctx, err
	}
	return context.WithValue(ctx, callerKey{}, caller), nil
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

// WrapStreamingHandler 拒絕串流：Delegated service 只有 unary。
func (i *interceptor) WrapStreamingHandler(connect.StreamingHandlerFunc) connect.StreamingHandlerFunc {
	return func(_ context.Context, conn connect.StreamingHandlerConn) error {
		return fmt.Errorf("httpapi/delegation：%s 不支援串流", conn.Spec().Procedure)
	}
}
