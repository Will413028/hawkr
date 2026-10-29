// Package identity 是 identity 模組的公開 API：認證（顧客 Clerk、後台 Cloudflare Access）、帳號與管理員、角色，帳號的停權與刪除。
//
// 職責、擁有的資料與不變量見同目錄的 AGENTS.md；身分驗證的決策見 ADR 2026-09-27-identity-clerk。
package identity

import (
	"context"
	"embed"
	"io/fs"
	"log/slog"
	"time"

	"github.com/riverqueue/river"

	"github.com/Will413028/hawkr/backend/internal/modules/identity/internal/account"
	"github.com/Will413028/hawkr/backend/internal/modules/identity/internal/admin"
	"github.com/Will413028/hawkr/backend/internal/modules/identity/internal/applerevoke"
	"github.com/Will413028/hawkr/backend/internal/modules/identity/internal/auth"
	"github.com/Will413028/hawkr/backend/internal/modules/identity/internal/model"
	"github.com/Will413028/hawkr/backend/internal/platform/adminperm"
	"github.com/Will413028/hawkr/backend/internal/platform/db"
	"github.com/Will413028/hawkr/backend/internal/platform/events"
	"github.com/Will413028/hawkr/backend/internal/platform/jobs"
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

// Deps 是模組需要的外部依賴，全部在組裝點提供。
type Deps struct {
	DB     *db.DB
	Events *events.Bus
	Jobs   jobs.Enqueuer
	// Providers 是外部的 Clerk、Access、Google 與 Apple；沒有設定的欄位是 nil，對應的功能回 unavailable。
	Providers Providers
	// Now 是目前時間；測試注入固定值。
	Now    func() time.Time
	Logger *slog.Logger
}

// Role 是帳號角色，值與資料庫的 CHECK 相同。
type Role = model.Role

// PrincipalKind 區分帳號與管理員。
type PrincipalKind = model.PrincipalKind

// Principal 是帶類型與角色的登入者。
type Principal = model.Principal

// Authenticated 是驗證後的登入者及 Clerk token 的姓名。
type Authenticated = model.Authenticated

// Clerk 是顧客 Clerk application 的 client。
type Clerk = model.Clerk

// ClerkIdentity 是已驗證的 Clerk session claims。
type ClerkIdentity = model.ClerkIdentity

// AdminIdentity 是驗證後的後台身分。
type AdminIdentity = model.AdminIdentity

// AdminVerifier 驗證後台 Access 或 dev token。
type AdminVerifier = model.AdminVerifier

// Identity 是驗證過的 Google 或 Apple ID token 資料。
type Identity = model.Identity

// IDTokenVerifier 驗證 Google 或 Apple 的 ID token。
type IDTokenVerifier = model.IDTokenVerifier

// AppleTokens 交換並撤銷 Sign in with Apple token。
type AppleTokens = model.AppleTokens

// AppleGrant 是 authorization code 換得的 token。
type AppleGrant = model.AppleGrant

// Sealer 加密排入 job 的第三方憑證。
type Sealer = model.Sealer

// Providers 是模組的外部身分與 Apple provider。
type Providers = model.Providers

// Account 是帳號，已刪除時只保留墓碑。
type Account = model.Account

// SuspendInput 是停權帳號的輸入。
type SuspendInput = model.SuspendInput

// AccountSuspension 是帳號的一筆停權紀錄。
type AccountSuspension = model.AccountSuspension

// SuspensionKind 是停權的型別，目前只有永久。
type SuspensionKind = model.SuspensionKind

// ReleaseReasonAccountDeleted 是帳號刪除時解除未解除紀錄的 release_reason。
const ReleaseReasonAccountDeleted = model.ReleaseReasonAccountDeleted

// RemoveRoleInput 是移除角色的輸入。
type RemoveRoleInput = model.RemoveRoleInput

// Lifecycle 是帳號的生命週期。
type Lifecycle = model.Lifecycle

// Admin 是管理員。
type Admin = model.Admin

// AdminRoleChangeInput 是授予或撤銷管理員角色的輸入。
type AdminRoleChangeInput = model.AdminRoleChangeInput

// AdminRoleChange 是已完成的管理員角色變更。
type AdminRoleChange = model.AdminRoleChange

// AdminRoleAction 是管理員角色的一次變更：授予或撤銷。
type AdminRoleAction = model.AdminRoleAction

// AdminRoleSource 是變更管理員角色的來源。
type AdminRoleSource = model.AdminRoleSource

// AccountFilter 是後台帳號清單的篩選。
type AccountFilter = model.AccountFilter

// AccountCursor 是帳號清單上一頁的排序鍵。
type AccountCursor = model.AccountCursor

// AccountPage 是一頁帳號。
type AccountPage = model.AccountPage

// GrantRoleInput 是授予帳號角色的輸入。
type GrantRoleInput = model.GrantRoleInput

// ListAccountsInput 是帳號清單的篩選與分頁輸入。
type ListAccountsInput = model.ListAccountsInput

// LinkAppleCredentialInput 是交出 Apple 憑證的輸入。
type LinkAppleCredentialInput = model.LinkAppleCredentialInput

const (
	RoleSeller               = model.RoleSeller
	LifecycleActive          = model.LifecycleActive
	LifecyclePendingDeletion = model.LifecyclePendingDeletion
	LifecycleDeleted         = model.LifecycleDeleted
	PrincipalAccount         = model.PrincipalAccount
	PrincipalAdmin           = model.PrincipalAdmin
	AdminRoleActionGrant     = model.AdminRoleActionGrant
	AdminRoleActionRevoke    = model.AdminRoleActionRevoke
	AdminRoleSourceCLI       = model.AdminRoleSourceCLI
	SuspensionPermanent      = model.SuspensionPermanent
)

// ErrInvalidToken 表示 token 的簽章、issuer、audience 或期限無效。
var ErrInvalidToken = model.ErrInvalidToken

// ErrRejected 表示 provider 拒絕且重試不會成功。
var ErrRejected = model.ErrRejected

// Roles 回傳所有帳號角色（只有另外申請的角色；每個帳號都是買家）。
func Roles() []Role { return model.Roles() }

// Module 是 identity 模組。
type Module struct {
	account     *account.Service
	admin       *admin.Service
	auth        *auth.Service
	applerevoke *applerevoke.Service
}

// New 建立模組。
func New(d Deps) *Module {
	ar := applerevoke.New(applerevoke.Deps{
		DB: d.DB, Jobs: d.Jobs, Apple: d.Providers.Apple,
		AppleTokens: d.Providers.AppleTokens, Sealer: d.Providers.Sealer, Now: d.Now, Logger: d.Logger,
	})
	a := account.New(account.Deps{
		DB: d.DB, Events: d.Events, Jobs: d.Jobs, Customers: d.Providers.Customers, AppleRevoker: ar,
		Now: d.Now, Logger: d.Logger,
	})
	return &Module{
		account: a,
		admin:   admin.New(admin.Deps{DB: d.DB, Events: d.Events, Now: d.Now, Logger: d.Logger}),
		auth: auth.New(auth.Deps{
			DB: d.DB, Account: a, Customers: d.Providers.Customers, Admin: d.Providers.Admin,
			Google: d.Providers.Google, Now: d.Now, Logger: d.Logger,
		}),
		applerevoke: ar,
	}
}

// Account 回傳帳號，含已刪除的墓碑。
func (m *Module) Account(ctx context.Context, id int64) (Account, error) {
	return m.account.Account(ctx, id)
}

// Accounts 回傳存在的帳號，依 ID 排序；不存在的 ID 略過。
func (m *Module) Accounts(ctx context.Context, ids []int64) ([]Account, error) {
	return m.account.Accounts(ctx, ids)
}

// GrantRole 授予角色（SELLER）；已有同角色時更新來源。帳號不存在或已刪除時回 NotFound，等待刪除時回 FailedPrecondition，
// 不是角色的值回 InvalidInput。
func (m *Module) GrantRole(ctx context.Context, in GrantRoleInput) error {
	return m.account.GrantRole(ctx, in)
}

// RemoveRole 移除角色（SELLER），永遠不刪帳號。已刪帳可重試。
func (m *Module) RemoveRole(ctx context.Context, in RemoveRoleInput) error {
	return m.account.RemoveRole(ctx, in)
}

// BeginDeletion 讓帳號進入等待刪除（Lifecycle 變成 PENDING_DELETION），之後 GrantRole 回 FailedPrecondition；已在等待時
// 成功。只由 accountdeletion 在受理會等待未結訂單的申請的 transaction 內呼叫；帳號已刪除或不存在時回 NotFound。
func (m *Module) BeginDeletion(ctx context.Context, accountID int64) error {
	return m.account.BeginDeletion(ctx, accountID)
}

// LockAccount 在呼叫者的 transaction 內鎖住存活的帳號，與授予、移除角色及刪帳號序列化；帳號已刪除時不鎖，
// 從未存在時回 NotFound。
func (m *Module) LockAccount(ctx context.Context, id int64) error {
	return m.account.LockAccount(ctx, id)
}

// SuspendAccount 永久停權帳號：停權紀錄與 suspended_at 在同一個 transaction 寫入，並排入 Clerk 停用 job。已停權時不寫第二筆紀錄，
// 回傳原本的紀錄與 created=false。只由 accountsuspension 呼叫（internal/arch 的 TestSuspensionEntriesAreOnlyCalledByAccountsuspension）：
// 擁有商店的帳號不能停權的檢查在那裡，這裡不查。類別不是政策分類回 InvalidInput，帳號不存在或已刪除回 NotFound。
func (m *Module) SuspendAccount(ctx context.Context, in SuspendInput) (AccountSuspension, bool, error) {
	return m.account.SuspendAccount(ctx, in)
}

// UnsuspendAccount 手動解除停權：清 suspended_at 並把解除時間與原因記在目前的紀錄上（同一個 transaction）。沒有停權時成功、
// 回傳 released=false。reason 必填。只由 accountsuspension 呼叫。
func (m *Module) UnsuspendAccount(ctx context.Context, accountID int64, reason string) (AccountSuspension, bool, error) {
	return m.account.UnsuspendAccount(ctx, accountID, reason)
}

// CurrentSuspension 回傳帳號目前未解除的停權紀錄；沒有停權時 ok 是 false，帳號不存在回 NotFound。給 A7-2 的「讀自己目前的停權」。
func (m *Module) CurrentSuspension(ctx context.Context, accountID int64) (AccountSuspension, bool, error) {
	return m.account.CurrentSuspension(ctx, accountID)
}

// DeleteAccount 刪除帳號、解除未解除的停權紀錄並排入 Clerk 刪除 job；已刪除時成功，從未存在時回 NotFound。
func (m *Module) DeleteAccount(ctx context.Context, accountID int64) error {
	return m.account.DeleteAccount(ctx, accountID)
}

// ListAccounts 列出存活帳號，新到舊；After 為 nil 時從第一頁開始。
func (m *Module) ListAccounts(ctx context.Context, in ListAccountsInput) (AccountPage, error) {
	return m.account.ListAccounts(ctx, in)
}

// Admin 回傳管理員。
func (m *Module) Admin(ctx context.Context, id int64) (Admin, error) {
	return m.admin.Admin(ctx, id)
}

// Admins 以一次查詢回傳存在的管理員；不存在的 ID 不在結果中。
func (m *Module) Admins(ctx context.Context, ids []int64) (map[int64]Admin, error) {
	return m.admin.Admins(ctx, ids)
}

// GrantAdminRole 授予已存在的管理員一個角色（管理員要先以 Access 登入一次），角色與事件在同一個 transaction 內寫入。
// 管理員不存在回 NotFound，已經有這個角色回 AlreadyExists，不寫事件。
func (m *Module) GrantAdminRole(ctx context.Context, in AdminRoleChangeInput) (AdminRoleChange, error) {
	return m.admin.GrantRole(ctx, in)
}

// RevokeAdminRole 撤銷管理員的一個角色，在同一個 transaction 內寫事件並發出 AdminRoleRevoked；管理員沒有這個角色回
// FailedPrecondition，不寫事件。撤掉最後一個 OWNER 是允許的，結果的 HolderRemains 供呼叫方警告。
func (m *Module) RevokeAdminRole(ctx context.Context, in AdminRoleChangeInput) (AdminRoleChange, error) {
	return m.admin.RevokeRole(ctx, in)
}

// AdminAllows 在呼叫方的 transaction 內以 FOR SHARE 讀管理員的角色並回答是否有權限 perm；與撤銷互相等待，所以通過
// 的檢查在呼叫方的 transaction 結束之前不會被撤銷。不在 transaction 內呼叫回錯誤。
func (m *Module) AdminAllows(ctx context.Context, adminID int64, perm adminperm.Permission) (bool, error) {
	return m.admin.AdminAllows(ctx, adminID, perm)
}

// Authenticate 依 issuer 驗證顧客 Clerk 或後台 Access/dev token。首次顧客認證建立帳號與買家角色；刪除或
// 無效 token 回 KindUnauthenticated，停權回 KindPermissionDenied，provider 或資料庫故障回其他錯誤。
func (m *Module) Authenticate(ctx context.Context, token string) (Authenticated, error) {
	return m.auth.Authenticate(ctx, token)
}

// VerifyGoogle 驗證 vendor portal step-up 的 Google ID token，不登入或寫入；單體入口的 VerifySession 呼叫它。token 的 nonce 必須等於
// wantNonce（必填，空字串是程式錯誤，回沒有分類的錯誤），iat 不得早於 10 分鐘前。回傳的 Identity 帶 EmailVerified，這裡不強制它：
// 以 email 授權的呼叫方必須自己檢查。無效 token、nonce 不符與 token 太舊回 KindUnauthenticated；未設定 provider 或讀不到公鑰回 KindUnavailable。
func (m *Module) VerifyGoogle(ctx context.Context, idToken, wantNonce string) (Identity, error) {
	return m.auth.VerifyGoogle(ctx, idToken, wantNonce)
}

// LinkAppleCredential 接收 App 原生登入 Apple 後交來的憑證（App Store 5.1.1(v)）：驗證 identity token，把它的 sub、
// audience 與加密的 authorization code 排入 job；job 換 refresh token，Apple 同一次簽發的 ID token 與連結時驗證的身分相同
// 才加密保存。帳號最終刪除時在同一個 transaction 內排入撤銷（單體 ADR D5：外部呼叫在 transaction 外）。
func (m *Module) LinkAppleCredential(ctx context.Context, in LinkAppleCredentialInput) error {
	return m.applerevoke.LinkAppleCredential(ctx, in)
}

// CheckAppleKeys 是啟動檢查：保存的 Apple token 密文用到的每一把金鑰都要在 IDENTITY_TOKEN_KEYS 的金鑰環裡（serve 與
// worker 啟動時呼叫）；沒有設定 Apple 時略過。
func (m *Module) CheckAppleKeys(ctx context.Context) error {
	return m.applerevoke.CheckKeys(ctx)
}

// AddWorkers 登記本模組的 job worker。
func (m *Module) AddWorkers(w *river.Workers) {
	m.account.AddWorkers(w)
	m.applerevoke.AddWorkers(w)
}

// PeriodicJobs 回傳本模組的排程 job；目前沒有。
func (m *Module) PeriodicJobs() []*river.PeriodicJob { return nil }
