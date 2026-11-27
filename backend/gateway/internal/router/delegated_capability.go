package router

import (
	"context"
	"sort"
	"strconv"
	"sync"

	"github.com/Will413028/hawkr/gateway/internal/authz"
	"github.com/Will413028/hawkr/gateway/internal/handler"
	"github.com/Will413028/hawkr/gateway/internal/handler/delegation"
	"github.com/Will413028/hawkr/gateway/internal/middleware"
	"github.com/Will413028/hawkr/gateway/internal/response"
	pkgdelegation "github.com/Will413028/hawkr/go-pkg/delegation"
	"github.com/Will413028/hawkr/go-pkg/envelope"
	"github.com/Will413028/hawkr/go-pkg/errors/bizcode"
	pkglogger "github.com/Will413028/hawkr/go-pkg/logger"
	accountpb "github.com/Will413028/hawkr/go-pkg/proto/account"
	sellerpb "github.com/Will413028/hawkr/go-pkg/proto/seller"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	grpcstatus "google.golang.org/grpc/status"
)

const headerOperationCredential = handler.HeaderOperationCredential

var (
	adminNoAuthMu       sync.Mutex
	adminNoAuthRegistry = map[string]struct{}{}
)

func registerAdminNoAuth(method, path string) {
	adminNoAuthMu.Lock()
	adminNoAuthRegistry[method+" "+path] = struct{}{}
	adminNoAuthMu.Unlock()
}

func registeredAdminNoAuth() []string {
	adminNoAuthMu.Lock()
	out := make([]string, 0, len(adminNoAuthRegistry))
	for k := range adminNoAuthRegistry {
		out = append(out, k)
	}
	adminNoAuthMu.Unlock()
	sort.Strings(out)
	return out
}

type delegationSessionResolver interface {
	GetCurrentDelegationSession(ctx context.Context, credential string, countsAsActivity bool, deadlineMs int) (*accountpb.GetCurrentSessionResponse, error)
}

type delegationStoreStateResolver interface {
	GetStoreStateForGuard(ctx context.Context, storeID int64, deadlineMs int) (*sellerpb.SellerResponse, error)
}

type delegationSessionRevoker interface {
	RevokeDelegationSession(ctx context.Context, req *accountpb.RevokeSessionRequest, deadlineMs int) (*accountpb.RevokeSessionResponse, error)
}

type delegationDeps struct {
	resolver             delegationSessionResolver
	stores               delegationStoreStateResolver
	revoker              delegationSessionRevoker
	deadlineMs           int
	storeStateDeadlineMs int
	revokeDeadlineMs     int
	audit *delegation.Orchestrator
}

func newDelegationDeps(h *handler.Holder) delegationDeps {
	d := delegationDeps{}
	if h != nil {
		if h.Account != nil {
			d.resolver = h.Account
			d.revoker = h.Account
			d.audit = &delegation.Orchestrator{Audit: h.Account, Cfg: h.Delegation}
		}
		if h.Seller != nil {
			d.stores = h.Seller
		}
		d.deadlineMs = h.Delegation.GrpcDeadlineCurrentMs
		d.storeStateDeadlineMs = h.Delegation.GrpcDeadlineStoreStateMs
		d.revokeDeadlineMs = h.Delegation.GrpcDeadlineCreateMs
	}
	return d
}

func delegationDenyMessage(code int32) string {
	switch code {
	case bizcode.DelegationCredentialInvalid:
		return "代行憑證無效，請重新建立工作階段"
	case bizcode.DelegationSessionExpired:
		return "代行工作階段已到期"
	case bizcode.DelegationSessionRevoked:
		return "代行工作階段已撤銷"
	case bizcode.DelegationTargetMismatch:
		return "此工作階段的代行對象不是這家店家"
	case bizcode.DelegationCapabilityDenied:
		return "此工作階段未取得執行該操作的能力"
	case bizcode.DelegationStoreNotOperable:
		return "此店家已停權或關閉，無法代行"
	case bizcode.DelegationStoreNotDelegable:
		return "此店家目前不可代行"
	case bizcode.DelegationUpstreamUnavailable:
		return "代行服務暫時不可用，請稍後再試"
	default:
		return "代行請求遭拒"
	}
}

func resolveDelegationSession(c *gin.Context, deps delegationDeps, countsAsActivity bool) (*accountpb.OperationSessionSummary, bool) {
	credential := c.GetHeader(headerOperationCredential)
	c.Request.Header.Del(headerOperationCredential)
	if credential == "" {
		response.AbortBizStatus(c, bizcode.DelegationCredentialInvalid, delegationDenyMessage(bizcode.DelegationCredentialInvalid))
		return nil, false
	}
	if deps.resolver == nil {
		zap.L().Error("Delegation.dependency.unwired", zap.String("dep", "resolver"))
		response.AbortBizStatus(c, bizcode.DelegationInternalContractMismatch, delegationDenyMessage(bizcode.DelegationInternalContractMismatch))
		return nil, false
	}

	resp, err := deps.resolver.GetCurrentDelegationSession(c.Request.Context(), credential, countsAsActivity, deps.deadlineMs)
	if err != nil {
		// 先問一句：這個 err 是「上游真的不可用」，還是「上游明確拒絕了這把憑證」？
		//
		// delegation 模組對無效憑證回的是 gRPC OK ＋ 非零 envelope code（9274 等），
		// 而 EnvelopeGuardInterceptor 會把非零碼轉成 gRPC error 並在 message 前面加上
		// "[9274] " 前綴（gateway/internal/grpc/envelope_guard_interceptor.go:104）。
		// 少了這一步還原，下面那個 resp.GetCode() != 0 分支永遠進不去（碼在到達前就
		// 變成 err 了），憑證無效會一律回 9280/424。
		//
		// 那個差別對使用者是實質的：424「代行服務暫時不可用，請稍後再試」會讓前端
		// 顯示可重試的錯誤，而重試永遠不會好——憑證不會因為再試一次就變有效。
		// 401 才會把人導回 ADM 重新建立工作階段。
		if code, bizMsg := envelope.ParseBizMessage(grpcstatus.Convert(err).Message()); code > 0 {
			if bizMsg == "" {
				bizMsg = delegationDenyMessage(code)
			}
			response.AbortBizStatus(c, code, bizMsg)
			return nil, false
		}
		response.AbortBizStatus(c, bizcode.DelegationUpstreamUnavailable, delegationDenyMessage(bizcode.DelegationUpstreamUnavailable))
		return nil, false
	}
	if resp.GetCode() != 0 {
		msg := resp.GetMessage()
		if msg == "" {
			msg = delegationDenyMessage(resp.GetCode())
		}
		response.AbortBizStatus(c, resp.GetCode(), msg)
		return nil, false
	}
	data := resp.GetData()
	if data == nil {
		response.AbortBizStatus(c, bizcode.DelegationCredentialInvalid, delegationDenyMessage(bizcode.DelegationCredentialInvalid))
		return nil, false
	}
	return data, true
}

const revokeReasonStoreNotOperable = pkgdelegation.RevokeReasonStoreNotOperable

func enforceStoreOperable(c *gin.Context, deps delegationDeps, session *accountpb.OperationSessionSummary, storeID int64) bool {
	if deps.stores == nil {
		zap.L().Error("Delegation.dependency.unwired", zap.String("dep", "stores"))
		response.AbortBizStatus(c, bizcode.DelegationInternalContractMismatch, delegationDenyMessage(bizcode.DelegationInternalContractMismatch))
		return false
	}

	resp, err := deps.stores.GetStoreStateForGuard(c.Request.Context(), storeID, deps.storeStateDeadlineMs)

	var denied int32
	switch {
	case err != nil:
		if !storeStateIsBusinessVerdict(err) {
			pkglogger.Warn(c.Request.Context(), "delegation.store_state.unavailable",
				zap.Int64("store_id", storeID), zap.Error(err))
			response.AbortBizStatus(c, bizcode.DelegationUpstreamUnavailable, delegationDenyMessage(bizcode.DelegationUpstreamUnavailable))
			return false
		}
		denied = bizcode.DelegationStoreNotDelegable
	case resp.GetData() == nil:
		denied = bizcode.DelegationStoreNotDelegable
	default:
		if ok, code := pkgdelegation.StoreOperable(resp.GetData().GetLifecycleStatus(), false); !ok {
			denied = code
		}
	}
	if denied == 0 {
		return true
	}

	revokeForStoreNotOperable(c, deps, session)
	response.AbortBizStatus(c, denied, delegationDenyMessage(denied))
	return false
}

func storeStateIsBusinessVerdict(err error) bool {
	switch grpcstatus.Code(err) {
	case codes.NotFound, codes.FailedPrecondition:
		return true
	default:
		return false
	}
}

func revokeForStoreNotOperable(c *gin.Context, deps delegationDeps, session *accountpb.OperationSessionSummary) {
	if deps.revoker == nil {
		zap.L().Error("Delegation.dependency.unwired", zap.String("dep", "revoker"))
		return
	}
	sessionID := session.GetSessionId()
	resp, err := deps.revoker.RevokeDelegationSession(c.Request.Context(), &accountpb.RevokeSessionRequest{
		SessionId: sessionID,
		ActorAdminId:   session.GetActor().GetAdminId(),
		RevokeReason:   revokeReasonStoreNotOperable,
		IdempotencyKey: revokeReasonStoreNotOperable + ":" + sessionID,
		RequestId:      c.GetString("request_id"),
	}, deps.revokeDeadlineMs)
	if err != nil {
		pkglogger.Warn(c.Request.Context(), "delegation.store_not_operable.revoke_failed",
			zap.String("session_id", sessionID), zap.Error(err))
		return
	}
	if resp.GetCode() != 0 {
		pkglogger.Warn(c.Request.Context(), "delegation.store_not_operable.revoke_denied",
			zap.String("session_id", sessionID), zap.Int32("code", resp.GetCode()))
	}
}

func recordGuardDenial(c *gin.Context, deps delegationDeps, session *accountpb.OperationSessionSummary, cap authz.AdminCapability, denied int32) {
	if denied != bizcode.DelegationTargetMismatch && denied != bizcode.DelegationCapabilityDenied {
		return
	}
	if deps.audit == nil {
		return
	}
	face := delegatedFace(cap)
	if face == "" {
		return
	}
	deps.audit.RecordReadDenial(c.Request.Context(), delegation.ReadDenial{
		Session:      session,
		ResourceType: face,
		ResourceID:   guardResourceID(c, face),
		Capability:    cap,
		BizCode:       denied,
		RequestID:     c.GetString("request_id"),
		CorrelationID: c.GetString("trace_id"),
		UserAgent:     c.GetHeader("User-Agent"),
	})
}

func delegatedFace(cap authz.AdminCapability) string {
	switch cap {
	case authz.AdminCapStorefrontRead, authz.AdminCapStorefrontWrite:
		return "storefront"
	case authz.AdminCapProductsRead, authz.AdminCapProductsWrite, authz.AdminCapProductsPublish:
		return "product"
	case authz.AdminCapOrdersRead:
		return "order"
	default:
		return ""
	}
}

func guardResourceID(c *gin.Context, face string) string {
	switch face {
	case "product":
		return c.Param("product_id")
	case "order":
		return c.Param("order_no")
	default:
		return ""
	}
}

func requireDelegated(cap authz.AdminCapability, deps delegationDeps) gin.HandlerFunc {
	return func(c *gin.Context) {
		session, ok := resolveDelegationSession(c, deps, true)
		if !ok {
			return
		}

		targetStoreID, _ := strconv.ParseInt(session.GetTarget().GetStoreId(), 10, 64)
		requestedStoreID, _ := strconv.ParseInt(c.Param("store_id"), 10, 64)

		caps := make([]authz.AdminCapability, 0, len(session.GetCapabilities()))
		for _, s := range session.GetCapabilities() {
			caps = append(caps, authz.AdminCapability(s))
		}

		if _, denied := authz.GrantedAdminCapabilities(authz.AdminDelegationContext{
			SessionStatus:      session.GetStatus(),
			Capabilities:       caps,
			TargetStoreID:      targetStoreID,
			RequestedStoreID:   requestedStoreID,
			RequiredCapability: cap,
		}); denied != 0 {
			recordGuardDenial(c, deps, session, cap, denied)
			response.AbortBizStatus(c, denied, delegationDenyMessage(denied))
			return
		}

		if !enforceStoreOperable(c, deps, session, targetStoreID) {
			return
		}

		middleware.SetDelegationSession(c, session)
		c.Next()
	}
}



func adminHandle(g *gin.RouterGroup, method, relPath string, h gin.HandlerFunc) {
	middleware.RegisterDelegatedRoute(method, ginJoinPath(g.BasePath(), relPath))
	g.Handle(method, relPath, h)
}

func credentialHandle(g *gin.RouterGroup, method, relPath string, h gin.HandlerFunc) {
	full := ginJoinPath(g.BasePath(), relPath)
	registerAdminNoAuth(method, full)
	middleware.RegisterDelegatedRoute(method, full)
	g.Handle(method, relPath, h)
}

func delegatedHandle(g *gin.RouterGroup, method, relPath string, cap authz.AdminCapability, deps delegationDeps, h gin.HandlerFunc) {
	full := ginJoinPath(g.BasePath(), relPath)
	authz.RegisterAdmin(method, full, cap)
	registerAdminNoAuth(method, full)
	middleware.RegisterDelegatedRoute(method, full)
	g.Handle(method, relPath, requireDelegated(cap, deps), h)
}
