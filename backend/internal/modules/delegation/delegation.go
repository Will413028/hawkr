// Package delegation 是 delegation 模組的公開 API。
package delegation

import (
	"context"
	"embed"
	"fmt"
	"io/fs"
	"log/slog"
	"strconv"
	"time"
	"uuid"

	"github.com/Will413028/hawkr/backend/internal/modules/delegation/internal/audit"
	"github.com/Will413028/hawkr/backend/internal/modules/delegation/internal/authorization"
	"github.com/Will413028/hawkr/backend/internal/modules/delegation/internal/model"
	"github.com/Will413028/hawkr/backend/internal/modules/delegation/internal/session"
	"github.com/Will413028/hawkr/backend/internal/modules/identity"
	"github.com/Will413028/hawkr/backend/internal/modules/seller"
	"github.com/Will413028/hawkr/backend/internal/modules/social"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
	"github.com/Will413028/hawkr/backend/internal/platform/db"
	"github.com/Will413028/hawkr/backend/internal/platform/events"
)

//go:embed db/migrations/*.sql db/migrations/atlas.sum
var migrationFiles embed.FS

func Migrations() fs.FS {
	sub, err := fs.Sub(migrationFiles, "db/migrations")
	if err != nil {
		panic(err)
	}
	return sub
}

// Seller 是模組讀店家狀態需要的 seller 公開 API；*seller.Module 滿足它。
type Seller interface {
	Store(ctx context.Context, storeID int64) (seller.Store, error)
}

// Social 是模組取消排程需要的 social 公開 API；*social.Module 滿足它。
type Social interface {
	CancelScheduledBy(ctx context.Context, kind social.ScheduleKind, refs []string, reason social.CancelReason) ([]uuid.UUID, error)
	CancelDelegatedSchedulesOfStore(ctx context.Context, storeID int64, reason social.CancelReason) ([]uuid.UUID, error)
	PendingSchedulerRefs(ctx context.Context, kind social.ScheduleKind) ([]string, error)
}

// Deps 是模組需要的外部依賴，全部在組裝點提供。Social 必填（撤銷授權與資格時取消排程）；Seller 必填：seller 在同一個行程，每個子命令都組得出它，沒有「沒有 seller」的組態。
type Deps struct {
	DB       *db.DB
	Events   *events.Bus
	Identity *identity.Module
	Seller   Seller
	Social   Social
	NewID    func() uuid.UUID
	Now      func() time.Time
	Logger   *slog.Logger
}

type Module struct {
	authorization *authorization.Module
	audit         *audit.Module
	session       *session.Module
}

// storeStateOf 讀店家此刻的狀態：只有 seller 回 NotFound 時才是找不到（Found 為 false）；已關閉的店家存在，Found 為 true、Closed 為 true。可營運與原因
// 都來自 seller 的 StoreActionDelegate（Store.ActionUnavailableReasons，由 CapStorefrontWrite 缺的條件對應：已關閉、等待刪除、停權），句子是
// seller.StoreActionUnavailableText，清單上的原因與 StartDelegation 被拒時的錯誤是同一句；seller 的其他錯誤原樣回傳（seller 在行程內，
// 沒有跨服務的「暫時讀不到」），不當成拒絕。
func storeStateOf(s Seller) func(context.Context, int64) (model.StoreState, error) {
	return func(ctx context.Context, storeID int64) (model.StoreState, error) {
		st, err := s.Store(ctx, storeID)
		switch {
		case apperr.KindOf(err) == apperr.KindNotFound:
			return model.StoreState{StoreID: storeID}, nil
		case err != nil:
			return model.StoreState{}, err
		}
		out := model.StoreState{
			Found: true, StoreID: st.StoreID, SellerAccountID: st.AccountID, Name: st.StoreName, Closed: st.Status == seller.Closed,
		}
		reasons := st.ActionUnavailableReasons(seller.StoreActionDelegate)
		out.Operable = len(reasons) == 0
		if !out.Operable {
			out.UnavailableText = seller.StoreActionUnavailableText(reasons[0], seller.StoreActionDelegate).Text
		}
		return out, nil
	}
}

func New(d Deps) *Module {
	if d.Seller == nil {
		panic("delegation: missing dependency Seller")
	}
	if d.Social == nil {
		panic("delegation: missing dependency Social")
	}
	storeState := storeStateOf(d.Seller)
	s := session.New(session.Deps{
		DB: d.DB, Identity: d.Identity, StoreState: storeState,
		CancelSchedules: func(ctx context.Context, sessionIDs []int64) error {
			refs := make([]string, len(sessionIDs))
			for i, id := range sessionIDs {
				refs[i] = strconv.FormatInt(id, 10)
			}
			_, err := d.Social.CancelScheduledBy(ctx, social.ScheduleByDelegation, refs, social.CancelAuthorityLost)
			return err
		},
		PendingScheduleSessionIDs: func(ctx context.Context) ([]int64, error) {
			refs, err := d.Social.PendingSchedulerRefs(ctx, social.ScheduleByDelegation)
			if err != nil {
				return nil, err
			}
			ids := make([]int64, 0, len(refs))
			for _, r := range refs {
				if id, err := strconv.ParseInt(r, 10, 64); err == nil {
					ids = append(ids, id)
				}
			}
			return ids, nil
		},
		Now: d.Now, Logger: d.Logger,
	})
	return &Module{
		session: s,
		authorization: authorization.New(authorization.Deps{
			DB: d.DB, Identity: d.Identity, Session: s, StoreState: storeState, NewID: d.NewID, Now: d.Now,
			CancelSchedulesOfStore: func(ctx context.Context, storeID int64) error {
				_, err := d.Social.CancelDelegatedSchedulesOfStore(ctx, storeID, social.CancelAuthorityLost)
				return err
			},
		}),
		audit: audit.New(audit.Deps{
			DB: d.DB, Events: d.Events, Session: s, NewID: d.NewID, Now: d.Now,
		}),
	}
}

type AuditAction = model.AuditAction
type AdminEligibilityChange = model.AdminEligibilityChange
type AuditConfirmInput = model.AuditConfirmInput
type AuditIntent = model.AuditIntent
type AuditIntentInput = model.AuditIntentInput
type AuditIntentOutcome = model.AuditIntentOutcome
type AuditLog = model.AuditLog
type AuditLogFilter = model.AuditLogFilter
type AuditLogPage = model.AuditLogPage
type AuditOrigin = model.AuditOrigin
type AuditResourceType = model.AuditResourceType
type AuditResult = model.AuditResult
type AuthorizationHistoryPage = model.AuthorizationHistoryPage
type AuthorizationAction = model.AuthorizationAction
type AuthorizationSource = model.AuthorizationSource
type AuthorizationStatusFilter = model.AuthorizationStatusFilter
type Capability = model.Capability
type ChangedField = model.ChangedField
type DelegationActor = model.DelegationActor
type DelegationAuthorization = model.DelegationAuthorization
type DelegationAuthorizationChange = model.DelegationAuthorizationChange
type DelegationAuthorizationFilter = model.DelegationAuthorizationFilter
type DelegationAuthorizationPage = model.DelegationAuthorizationPage
type DelegationAuthorizationPanel = model.DelegationAuthorizationPanel
type DelegationAuthorizationRevocation = model.DelegationAuthorizationRevocation
type DelegationDenial = model.DelegationDenial
type DelegationError = model.DelegationError
type DelegationPaging = model.DelegationPaging
type DelegationPreflight = model.DelegationPreflight
type DelegationPreflightInput = model.DelegationPreflightInput
type DelegationReason = model.DelegationReason
type DelegationSession = model.DelegationSession
type DelegationStatus = model.DelegationStatus
type DelegationSessionFilter = model.DelegationSessionFilter
type DelegationSessionPage = model.DelegationSessionPage
type DelegationSummary = model.DelegationSummary
type DelegationTarget = model.DelegationTarget
type ListDelegationAuthorizationsInput = model.ListDelegationAuthorizationsInput
type DelegationAuthorizationHistoryInput = model.DelegationAuthorizationHistoryInput
type GrantedAuthorization = model.GrantedAuthorization
type GrantOutcome = model.GrantOutcome
type RedeemedDelegation = model.RedeemedDelegation
type RevokeDelegationInput = model.RevokeDelegationInput
type RevokedAuthorization = model.RevokedAuthorization
type StartDelegationInput = model.StartDelegationInput
type StartedDelegation = model.StartedDelegation
type RiskLevel = model.RiskLevel

const (
	CapabilityStorefrontRead  = model.CapabilityStorefrontRead
	CapabilityStorefrontWrite = model.CapabilityStorefrontWrite
	CapabilityProductsRead    = model.CapabilityProductsRead
	CapabilityProductsWrite   = model.CapabilityProductsWrite
	CapabilityProductsPublish = model.CapabilityProductsPublish
	CapabilityPostsRead       = model.CapabilityPostsRead
	CapabilityPostsWrite      = model.CapabilityPostsWrite
	CapabilityPostsPublish    = model.CapabilityPostsPublish
	CapabilityOrdersRead      = model.CapabilityOrdersRead
)

const (
	ReasonSellerRequest       = model.ReasonSellerRequest
	ReasonPlatformMaintenance = model.ReasonPlatformMaintenance
	ReasonContentGovernance   = model.ReasonContentGovernance
)

const (
	DelegationActive  = model.DelegationActive
	DelegationExpired = model.DelegationExpired
	DelegationRevoked = model.DelegationRevoked
)

const (
	AuthorizationContract       = model.AuthorizationContract
	AuthorizationTicket         = model.AuthorizationTicket
	AuthorizationPlatformPolicy = model.AuthorizationPlatformPolicy
)

const (
	AuthorizationGrant     = model.AuthorizationGrant
	AuthorizationReinstate = model.AuthorizationReinstate
	AuthorizationUpdate    = model.AuthorizationUpdate
	AuthorizationRevoke    = model.AuthorizationRevoke
)

const (
	AuthorizationAny         = model.AuthorizationAny
	AuthorizationEffective   = model.AuthorizationEffective
	AuthorizationIneffective = model.AuthorizationIneffective
)

const (
	GrantCreated    = model.GrantCreated
	GrantReinstated = model.GrantReinstated
)

const (
	AuditStorefrontUpdate      = model.AuditStorefrontUpdate
	AuditStorefrontLogoUpdate  = model.AuditStorefrontLogoUpdate
	AuditStorefrontCoverUpdate = model.AuditStorefrontCoverUpdate
	AuditProductCreate         = model.AuditProductCreate
	AuditProductUpdate         = model.AuditProductUpdate
	AuditProductPublish        = model.AuditProductPublish
	AuditProductUnpublish      = model.AuditProductUnpublish
	AuditProductBatchUnpublish = model.AuditProductBatchUnpublish
	AuditPostCreate            = model.AuditPostCreate
	AuditPostUpdate            = model.AuditPostUpdate
	AuditPostPublish           = model.AuditPostPublish
	AuditPostUnpublish         = model.AuditPostUnpublish
	AuditPostSchedule          = model.AuditPostSchedule
	AuditOrderPIIReveal        = model.AuditOrderPIIReveal
	AuditStorefrontRead        = model.AuditStorefrontRead
	AuditProductRead           = model.AuditProductRead
	AuditPostRead              = model.AuditPostRead
	AuditOrderRead             = model.AuditOrderRead
	AuditCorrection            = model.AuditCorrection
)

const (
	RiskLow    = model.RiskLow
	RiskMedium = model.RiskMedium
	RiskHigh   = model.RiskHigh
)

const (
	AuditPending   = model.AuditPending
	AuditSucceeded = model.AuditSucceeded
	AuditFailed    = model.AuditFailed
)

const (
	AuditInteractive = model.AuditInteractive
	AuditScheduled   = model.AuditScheduled
)

const (
	ResourceStorefront = model.ResourceStorefront
	ResourceProduct    = model.ResourceProduct
	ResourcePost       = model.ResourcePost
	ResourceOrder      = model.ResourceOrder
)

const DelegationMaxBatch = model.DelegationMaxBatch
const RevokeReasonAuthorizationRevoked = model.RevokeReasonAuthorizationRevoked
const RevokeReasonAdminRoleRevoked = model.RevokeReasonAdminRoleRevoked

const DenialActorBusy = model.DenialActorBusy
const DenialAdminNotAllowlisted = model.DenialAdminNotAllowlisted
const DenialAdminRequired = model.DenialAdminRequired
const DenialAuditIntentFailed = model.DenialAuditIntentFailed
const DenialAuthorizationAlreadyActive = model.DenialAuthorizationAlreadyActive
const DenialAuthorizationNotFound = model.DenialAuthorizationNotFound
const DenialAuthorizationRevoked = model.DenialAuthorizationRevoked
const DenialCapabilityDenied = model.DenialCapabilityDenied
const DenialCredentialInvalid = model.DenialCredentialInvalid
const DenialHandoffInvalid = model.DenialHandoffInvalid
const DenialIdempotencyKeyReused = model.DenialIdempotencyKeyReused
const DenialInvalidCommand = model.DenialInvalidCommand
const DenialSessionExpired = model.DenialSessionExpired
const DenialSessionNotFound = model.DenialSessionNotFound
const DenialSessionRevoked = model.DenialSessionRevoked
const DenialStoreBusy = model.DenialStoreBusy
const DenialStoreNotDelegable = model.DenialStoreNotDelegable
const DenialStoreNotOperable = model.DenialStoreNotOperable
const DenialStoreClosed = model.DenialStoreClosed

func DelegationDenialOf(err error) (*DelegationError, bool) { return model.DelegationDenialOf(err) }

func (m *Module) DelegationAuthorizations(ctx context.Context, in ListDelegationAuthorizationsInput) (DelegationAuthorizationPage, error) {
	return m.authorization.DelegationAuthorizations(ctx, in)
}

func (m *Module) DelegationAuthorizationOfStore(ctx context.Context, adminID, storeID int64) (DelegationAuthorizationPanel, error) {
	return m.authorization.DelegationAuthorizationOfStore(ctx, adminID, storeID)
}

func (m *Module) GrantDelegationAuthorization(ctx context.Context, in DelegationAuthorizationChange) (GrantedAuthorization, error) {
	return m.authorization.GrantDelegationAuthorization(ctx, in)
}

func (m *Module) UpdateDelegationAuthorization(ctx context.Context, in DelegationAuthorizationChange) (DelegationAuthorization, error) {
	return m.authorization.UpdateDelegationAuthorization(ctx, in)
}

func (m *Module) RevokeDelegationAuthorization(ctx context.Context, in DelegationAuthorizationRevocation) (RevokedAuthorization, error) {
	return m.authorization.RevokeDelegationAuthorization(ctx, in)
}

func (m *Module) DelegationAuthorizationHistory(ctx context.Context, in DelegationAuthorizationHistoryInput) (AuthorizationHistoryPage, error) {
	return m.authorization.DelegationAuthorizationHistory(ctx, in)
}

func (m *Module) DelegationSessionSummary(ctx context.Context, id int64) (DelegationSummary, error) {
	return m.session.DelegationSessionSummary(ctx, id)
}

func (m *Module) DelegationSessions(ctx context.Context, f DelegationSessionFilter, p DelegationPaging) (DelegationSessionPage, error) {
	return m.session.DelegationSessions(ctx, f, p)
}

func (m *Module) DelegationPreflight(ctx context.Context, in DelegationPreflightInput) (DelegationPreflight, error) {
	return m.session.DelegationPreflight(ctx, in)
}

func (m *Module) StartDelegation(ctx context.Context, in StartDelegationInput) (StartedDelegation, error) {
	return m.session.StartDelegation(ctx, in)
}

func (m *Module) RedeemDelegationHandoff(ctx context.Context, code string) (RedeemedDelegation, error) {
	return m.session.RedeemDelegationHandoff(ctx, code)
}

func (m *Module) ResolveDelegation(ctx context.Context, credential string, activity bool) (DelegationSession, error) {
	return m.session.ResolveDelegation(ctx, credential, activity)
}

func (m *Module) EndDelegation(ctx context.Context, credential string) error {
	return m.session.EndDelegation(ctx, credential)
}

func (m *Module) RevokeDelegation(ctx context.Context, in RevokeDelegationInput) (DelegationSession, error) {
	return m.session.RevokeDelegation(ctx, in)
}

func (m *Module) DelegationAuditLogs(ctx context.Context, f AuditLogFilter, p DelegationPaging) (AuditLogPage, error) {
	return m.audit.DelegationAuditLogs(ctx, f, p)
}

func (m *Module) RecordDelegationAuditIntent(ctx context.Context, in AuditIntentInput) (AuditIntent, error) {
	return m.audit.RecordDelegationAuditIntent(ctx, in)
}

func (m *Module) RecordDelegationAuditIntents(ctx context.Context, items []AuditIntentInput) ([]AuditIntentOutcome, error) {
	return m.audit.RecordDelegationAuditIntents(ctx, items)
}

func (m *Module) ConfirmDelegationAudit(ctx context.Context, in AuditConfirmInput) (AuditLog, error) {
	return m.audit.ConfirmDelegationAudit(ctx, in)
}

func (m *Module) ConfirmDelegationAudits(ctx context.Context, items []AuditConfirmInput) ([]AuditLog, error) {
	return m.audit.ConfirmDelegationAudits(ctx, items)
}

// SetAdminEligibility 開通或撤回管理員的代行資格。
func (m *Module) SetAdminEligibility(ctx context.Context, in AdminEligibilityChange) error {
	return m.session.SetAdminEligibility(ctx, in)
}

// OnAdminRoleRevoked 訂閱 identity 的 AdminRoleRevoked：在撤銷角色的同一個 transaction 內撤銷該管理員進行中的代行
// （原因 RevokeReasonAdminRoleRevoked）。組裝點以 events.Subscribe 訂閱。
func (m *Module) OnAdminRoleRevoked(ctx context.Context, e identity.AdminRoleRevoked) error {
	return m.session.OnAdminRoleRevoked(ctx, e)
}

// EndDelegationsOfAdmin 結束管理員進行中的代行（憑證立刻失效，不寫 REVOKED，發起的排程不取消）；後台登出時呼叫。
func (m *Module) EndDelegationsOfAdmin(ctx context.Context, adminID int64) error {
	return m.session.EndDelegationsOfAdmin(ctx, adminID)
}

// OnPostPublished 訂閱 social 的 PostPublished：代行發起的排程到點發布時，在發布的同一個 transaction 內寫一列 SUCCEEDED 的稽核（單段，冪等鍵
// schedule:<排程 ID>，重送不寫第二列）。其他來源的發布（直接發布、店主自己的排程）不屬於代行，略過。組裝點以 events.Subscribe 訂閱。
func (m *Module) OnPostPublished(ctx context.Context, e social.PostPublished) error {
	if e.Origin != social.OriginScheduled || e.ScheduledBy.Kind != social.ScheduleByDelegation {
		return nil
	}
	sessionID, err := strconv.ParseInt(e.ScheduledBy.Ref, 10, 64)
	if err != nil {
		return fmt.Errorf("delegation：排程 %s 的發起者 ref %q 不是代行 session ID：%w", e.ScheduleID, e.ScheduledBy.Ref, err)
	}
	return m.audit.RecordScheduledPublish(ctx, model.ScheduledPublishAuditInput{
		SessionID: sessionID, ScheduleID: e.ScheduleID, PostID: e.PostID, Preview: e.Preview,
	})
}

// scheduleAuthorizer 是 social 的排程授權 port 的 delegation 實作（發起者種類 DELEGATION，ref 是 session ID）。
type scheduleAuthorizer struct{ session *session.Module }

// AuthorizeScheduledPublish 在 social 的排程 job 內（貼文列已被鎖）重驗代行發起的排程：session 沒被撤銷、session 的店家就是排程的店家，加上開始代行用的
// 同一個授權述詞（資格、角色權限、店家可代行、授權有效且含 posts.publish）。全部是不鎖的讀取（鎖 identity 或 delegation 的列會倒退鎖順序）。
// 拒絕回 social.ErrScheduleDenied（job 取消排程為授權失效、不重試），讀取失敗原樣回傳（job 重試）。
func (a scheduleAuthorizer) AuthorizeScheduledPublish(ctx context.Context, p social.ScheduledPublish) error {
	sessionID, err := strconv.ParseInt(p.By.Ref, 10, 64)
	if err != nil {
		return fmt.Errorf("%w：發起者 ref %q 不是代行 session ID", social.ErrScheduleDenied, p.By.Ref)
	}
	ok, reason, err := a.session.SessionAuthority(ctx, sessionID, p.StoreID)
	if err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf("%w：%s", social.ErrScheduleDenied, reason)
	}
	return nil
}

// NewScheduleAuthorizer 建立 DELEGATION 排程的授權實作。它不需要 Deps.Social，所以組裝點先建它、再建 social、最後建 delegation 模組
// （social 要它，delegation 要 social）。
func NewScheduleAuthorizer(d Deps) social.ScheduleAuthorizer {
	if d.Seller == nil {
		panic("delegation: missing dependency Seller")
	}
	return scheduleAuthorizer{session: session.New(session.Deps{
		DB: d.DB, Identity: d.Identity, StoreState: storeStateOf(d.Seller), Now: d.Now, Logger: d.Logger,
	})}
}
