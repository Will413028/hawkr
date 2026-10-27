// Package agent 是 agent 模組的公開 API：目前只有 vendor portal（供應商以邀請進入 RFQ 的存取模型）。
//
// 職責、擁有的資料與不變量見同目錄的 AGENTS.md。
package agent

import (
	"context"
	"embed"
	"io/fs"
	"time"
	"uuid"

	"github.com/riverqueue/river"

	"github.com/Will413028/hawkr/backend/internal/modules/agent/internal/model"
	"github.com/Will413028/hawkr/backend/internal/modules/agent/internal/vendorportal"
	"github.com/Will413028/hawkr/backend/internal/modules/seller"
	"github.com/Will413028/hawkr/backend/internal/platform/db"
)

//go:embed db/migrations/*.sql db/migrations/atlas.sum
var migrationFiles embed.FS

// Migrations 回傳本模組的 migration，給 cmd/hawkr 登記。
func Migrations() fs.FS {
	sub, err := fs.Sub(migrationFiles, "db/migrations")
	if err != nil {
		panic(err) // 只有 embed 的路徑寫錯才會發生，編譯後的 binary 不可能遇到
	}
	return sub
}

// Deps 是模組需要的外部依賴；clock 由組裝點注入。Prospects 是 seller 模組（*seller.Module）。NewID 預設 uuid.NewV7，
// 測試以外不給。
type Deps struct {
	DB        *db.DB
	Prospects Prospects
	Now       func() time.Time
	NewID     func() uuid.UUID
}

// Module 是 agent 模組。
type Module struct {
	vendorPortal *vendorportal.Service
}

// New 建立模組。
func New(d Deps) *Module {
	if d.DB == nil || d.Prospects == nil {
		panic("agent: missing dependency")
	}
	if d.Now == nil {
		panic("agent: Now is required")
	}
	if d.NewID == nil {
		d.NewID = uuid.NewV7
	}
	return &Module{vendorPortal: vendorportal.New(vendorportal.Deps{
		DB: d.DB, Contacts: contacts{d.Prospects}, Now: d.Now, NewID: d.NewID,
	})}
}

// contacts 把 seller 的 ProspectContact 轉成子領域最小的 port。
type contacts struct{ p Prospects }

func (c contacts) ProspectContact(ctx context.Context, prospectID int64) (vendorportal.Contact, error) {
	pc, err := c.p.ProspectContact(ctx, prospectID)
	if err != nil {
		return vendorportal.Contact{}, err
	}
	return vendorportal.Contact{Active: pc.Status == seller.ProspectStatusActive, Email: pc.Email}, nil
}

// 對外型別。
type (
	// EnsureInviteInput 是建立或重取邀請的輸入。
	EnsureInviteInput = model.EnsureInviteInput
	// IssuedInvite 是 EnsureInvite 的結果（明文 secret 只出現一次）。
	IssuedInvite = model.IssuedInvite
	// InvitePreview 是邀請的預覽。
	InvitePreview = model.InvitePreview
	// IssuedSession 是兌換邀請的結果（明文 token 只出現一次）。
	IssuedSession = model.IssuedSession
	// SessionInfo 是有效 session 的內容。
	SessionInfo = model.SessionInfo
	// Verification 是 session 的驗證紀錄。
	Verification = model.Verification
	// VerificationMethod 是驗證方式。
	VerificationMethod = model.VerificationMethod
	// RevokeReason 是邀請被撤銷的原因。
	RevokeReason = model.RevokeReason
	// PurgeResult 是清理刪除的列數。
	PurgeResult = model.PurgeResult
)

// VerificationGoogle 是 Google 登入；入口經 identity 驗證 Google token，模組只收已驗證的 email。
const VerificationGoogle = model.VerificationGoogle

// 撤銷原因。
const RevokeProspectArchived = model.RevokeProspectArchived

// VerificationMethodValues 回傳全部驗證方式。
func VerificationMethodValues() []VerificationMethod { return model.VerificationMethodValues() }

// RevokeReasonValues 回傳全部撤銷原因。
func RevokeReasonValues() []RevokeReason { return model.RevokeReasonValues() }

// EnsureInvite 的結果，用 errors.Is 判斷（legacy gRPC adapter 依分類映射）。
var (
	ErrInviteAlreadyDelivered = model.ErrInviteAlreadyDelivered
	ErrInviteRevoked          = model.ErrInviteRevoked
	ErrNoDeliverableEmail     = model.ErrNoDeliverableEmail
	ErrProspectNotActive      = model.ErrProspectNotActive
	ErrInviteSecretSuperseded = model.ErrInviteSecretSuperseded
)

// EnsureInvite 建立或重取一個 RFQ 收件人的邀請，回傳明文 secret（只此一次）。規則見 vendorportal.Service.EnsureInvite。
func (m *Module) EnsureInvite(ctx context.Context, in EnsureInviteInput) (IssuedInvite, error) {
	return m.vendorPortal.EnsureInvite(ctx, in)
}

// MarkInviteDelivered 記下邀請的那一把 secret（寄出的明文）已經寄出：只有它仍是目前的 secret 才標記；同一把 secret 重複
// 標記冪等；secret 已被換掉回 ErrInviteSecretSuperseded。
func (m *Module) MarkInviteDelivered(ctx context.Context, inviteID uuid.UUID, secret string) error {
	return m.vendorPortal.MarkDelivered(ctx, inviteID, secret)
}

// ExchangeInvite 以邀請 secret 換一個新的 session（邀請到期或撤銷之前可重複兌換）；不可用一律是同一個 NotFound。
func (m *Module) ExchangeInvite(ctx context.Context, secret string) (IssuedSession, error) {
	return m.vendorPortal.Exchange(ctx, secret)
}

// PreviewInvite 回傳遮罩後的 email 與到期時間，沒有副作用；不可用一律是同一個 NotFound。
func (m *Module) PreviewInvite(ctx context.Context, secret string) (InvitePreview, error) {
	return m.vendorPortal.Preview(ctx, secret)
}

// ValidateSession 驗證 session token 並滑動 idle 期限；不可用一律是同一個 Unauthenticated。
func (m *Module) ValidateSession(ctx context.Context, token string) (SessionInfo, error) {
	return m.vendorPortal.Validate(ctx, token)
}

// VerifySession 記下 session 已驗證業者持有邀請的信箱（verifiedEmail 由呼叫端先驗證過）；相同 email 重送冪等，
// 不符或不可用一律是同一個 PermissionDenied。
func (m *Module) VerifySession(ctx context.Context, token string, method VerificationMethod, verifiedEmail string) (SessionInfo, error) {
	return m.vendorPortal.Verify(ctx, token, method, verifiedEmail)
}

// OnProspectArchived 是 seller.ProspectArchived 的 handler，在 seller 的 transaction 內撤銷該 prospect 的邀請與 session。
func (m *Module) OnProspectArchived(ctx context.Context, e seller.ProspectArchived) error {
	return m.vendorPortal.RevokeProspect(ctx, e.ProspectID, RevokeProspectArchived)
}

// OnProspectMerged 是 seller.ProspectMerged 的 handler，在 seller 的 transaction 內把輸家（source）的邀請改掛贏家（target）：
// email 快照、secret、session 與驗證都不變，不撤銷（ADR「合併時的邀請」）。
func (m *Module) OnProspectMerged(ctx context.Context, e seller.ProspectMerged) error {
	return m.vendorPortal.RepointProspect(ctx, e.SourceProspectID, e.TargetProspectID)
}

// PurgeExpired 清理過期的 session 與邀請（連同 email），回傳刪除的列數；它是 agent.purge_vendor_portal job 的內容，給 worker
// 與測試呼叫。
func (m *Module) PurgeExpired(ctx context.Context) (PurgeResult, error) {
	return m.vendorPortal.Purge(ctx)
}

// AddWorkers 登記本模組的 River worker。
func (m *Module) AddWorkers(workers *river.Workers) {
	m.vendorPortal.AddWorkers(workers)
}

// PeriodicJobs 回傳本模組的排程 job：每天一次的過期清理（啟動即執行）。
func (m *Module) PeriodicJobs() []*river.PeriodicJob {
	return m.vendorPortal.PeriodicJobs()
}
