package agent

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"net/http"
	"strconv"

	"connectrpc.com/connect"

	"github.com/Will413028/hawkr/backend/internal/httpapi/rpc"
	model "github.com/Will413028/hawkr/backend/internal/modules/agent"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
)

// Sessions 是驗證 session 的介面（agent.Module.ValidateSession）。
type Sessions interface {
	ValidateSession(ctx context.Context, token string) (model.SessionInfo, error)
}

// Session 是 interceptor 驗證過的 vendor portal session：Token 是 cookie 的明文，交給需要它的方法（VerifySession 以它定位 session）。
type Session struct {
	Token string
	Info  model.SessionInfo
}

type sessionKey struct{}

// sessionFrom 回傳 interceptor 為這個請求驗證過的 session。interceptor 沒有放進去（handler 被掛在沒有它的地方）回沒有分類的錯誤
// （internal），不放行。
func sessionFrom(ctx context.Context) (Session, error) {
	s, ok := ctx.Value(sessionKey{}).(Session)
	if !ok {
		return Session{}, errNoSession
	}
	return s, nil
}

var errNoSession = errors.New("httpapi/agent：ctx 裡沒有 session，InviteeService 的 handler 沒有掛 interceptor")

// googleNoncePrefix 是 nonce 的用途前綴：與 agent 模組存 token 的雜湊（原始 bytes 的 SHA-256）不同的輸入，nonce 與資料庫裡的
// token_hash 不能互相推導。
const googleNoncePrefix = "hawkr vendor-portal google nonce\x00"

// GoogleNonce 是 session 的 Google 登入 nonce（OIDC Core §15.5.2：以 session cookie 值的雜湊推導）：用途前綴加 session token 的
// SHA-256，base64url（無 padding）。不儲存，每次由 token 算。
func GoogleNonce(token string) string {
	sum := sha256.Sum256([]byte(googleNoncePrefix + token))
	return base64.RawURLEncoding.EncodeToString(sum[:])
}

// parseRecipientID 解析請求或路徑裡的 rfq_recipient_id：只接受正整數的十進位寫法（與 cookie 名稱同一種）。
func parseRecipientID(s string) (int64, bool) {
	id, err := strconv.ParseInt(s, 10, 64)
	if err != nil || id <= 0 || strconv.FormatInt(id, 10) != s {
		return 0, false
	}
	return id, true
}

// SessionOf 驗證請求裡 rfqRecipientID 那張邀請的 cookie，給入口轉 gateway 的 RFQ 路由用（id 由入口從路徑解析）：沒有 cookie 或
// session 不可用，或 session 不是這個收件人的，回 (零值, false, nil)；其他錯誤（資料庫等）原樣回傳。
func SessionOf(sessions Sessions) func(*http.Request, int64) (model.SessionInfo, bool, error) {
	return func(r *http.Request, rfqRecipientID int64) (model.SessionInfo, bool, error) {
		token := rpc.VendorPortalCookieToken(r.Header, rfqRecipientID)
		if token == "" {
			return model.SessionInfo{}, false, nil
		}
		info, err := sessions.ValidateSession(r.Context(), token)
		if apperr.KindOf(err) == apperr.KindUnauthenticated {
			return model.SessionInfo{}, false, nil
		}
		if err != nil {
			return model.SessionInfo{}, false, err
		}
		if info.RfqRecipientID != rfqRecipientID {
			return model.SessionInfo{}, false, nil
		}
		return info, true, nil
	}
}

// recipientRequest 是 VendorPortalInviteeService 的每個請求都有的欄位：它指名要用哪一張邀請的 cookie。
type recipientRequest interface {
	GetRfqRecipientId() string
}

var recipientPaths = rpc.FieldPaths{"RfqRecipientId": "rfq_recipient_id"}

// NewInterceptor 建立只掛在 VendorPortalInviteeService 的 interceptor，要放在 rpc.NewInterceptor 之後（內層），錯誤才會被轉換並記
// log。授權跟著 service 的邊界：這個 service 的每個方法都需要 session，所以沒有逐方法的宣告。以請求的 rfq_recipient_id 選
// cookie（rpc.VendorPortalCookieToken），以 agent.ValidateSession 驗證，session 必須屬於這個收件人，並把 Session 放進 ctx；沒有
// cookie、session 不可用或屬於別的收件人一律回同一個 unauthenticated。請求沒有 rfq_recipient_id 欄位（新增了不帶它的方法）回
// internal，不放行；欄位空白或格式不對回 invalid_input。
func NewInterceptor(sessions Sessions) (connect.Interceptor, error) {
	if sessions == nil {
		return nil, errors.New("httpapi/agent：NewInterceptor 需要 Sessions")
	}
	return &interceptor{sessions: sessions}, nil
}

type interceptor struct{ sessions Sessions }

// errSessionInvalid 是沒有 cookie、session 不可用與 session 屬於別的收件人共用的錯誤（與 agent.ValidateSession 的訊息一致）。
func errSessionInvalid() error { return apperr.Unauthenticated("登入已失效") }

func (i *interceptor) enter(ctx context.Context, procedure string, header http.Header, msg any) (context.Context, error) {
	named, ok := msg.(recipientRequest)
	if !ok {
		return ctx, fmt.Errorf("httpapi/agent：%s 的請求沒有 rfq_recipient_id", procedure)
	}
	id, ok := parseRecipientID(named.GetRfqRecipientId())
	if !ok {
		return ctx, rpc.WithFieldPaths(apperr.InvalidInputOn([]string{"RfqRecipientId"}, "rfq_recipient_id 必須是正整數的十進位字串"), recipientPaths)
	}
	token := rpc.VendorPortalCookieToken(header, id)
	if token == "" {
		return ctx, errSessionInvalid()
	}
	info, err := i.sessions.ValidateSession(ctx, token)
	if err != nil {
		return ctx, err
	}
	if info.RfqRecipientID != id {
		return ctx, errSessionInvalid()
	}
	return context.WithValue(ctx, sessionKey{}, Session{Token: token, Info: info}), nil
}

func (i *interceptor) WrapUnary(next connect.UnaryFunc) connect.UnaryFunc {
	return func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
		ctx, err := i.enter(ctx, req.Spec().Procedure, req.Header(), req.Any())
		if err != nil {
			return nil, err
		}
		return next(ctx, req)
	}
}

func (i *interceptor) WrapStreamingClient(next connect.StreamingClientFunc) connect.StreamingClientFunc {
	return next
}

// WrapStreamingHandler 拒絕串流：這個 service 只有 unary，請求訊息要先讀到才知道選哪張邀請的 cookie。
func (i *interceptor) WrapStreamingHandler(connect.StreamingHandlerFunc) connect.StreamingHandlerFunc {
	return func(_ context.Context, conn connect.StreamingHandlerConn) error {
		return fmt.Errorf("httpapi/agent：%s 不支援串流", conn.Spec().Procedure)
	}
}
