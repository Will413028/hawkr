package model

import (
	"context"
	"errors"
	"net/http"
	"slices"
	"time"
	"uuid"

	"github.com/Will413028/hawkr/backend/internal/platform/adminperm"
	"github.com/Will413028/hawkr/backend/internal/platform/policy"
)

// Role 是帳號另外申請的角色。值等於 account_roles.role 的 CHECK（TestCodeValueChecksMatchTheGoTypes）。買家不是角色：
// 帳號活著就是買家（Will 2026-10-01 W5，ADR 2026-09-30-account-created-provisions-buyer），舊契約需要的買家位元由
// internal/legacy 在邊界合成。
type Role string

const RoleSeller Role = "SELLER"

// Roles 回傳所有角色。
func Roles() []Role { return []Role{RoleSeller} }

// PrincipalKind 是 principal 的類型：帳號（買家、賣家）或管理員。兩者的 ID 各自發號，要連同類型才能識別。
type PrincipalKind int

const (
	PrincipalAccount PrincipalKind = iota + 1
	PrincipalAdmin
)

// Principal 是登入的使用者。
type Principal struct {
	Kind PrincipalKind
	ID   int64
	// Roles 是帳號另外申請的角色，依字母排序；管理員不使用它。帳號（Kind 是 PrincipalAccount）一定是買家，不列在這裡。
	Roles []Role
	// AdminRoles 是管理員目前的角色（identity.admin_roles），依字母排序，認證時每個請求都讀；零個表示沒有任何權限。帳號不使用它。
	AdminRoles []adminperm.Role
}

// HasRole 回傳帳號有沒有角色 r。
func (p Principal) HasRole(r Role) bool { return slices.Contains(p.Roles, r) }

// AdminAllows 回傳 principal 是不是管理員、而且角色允許權限 perm。
func (p Principal) AdminAllows(perm adminperm.Permission) bool {
	return p.Kind == PrincipalAdmin && adminperm.Allows(p.AdminRoles, perm)
}

// Authenticated 是認證過的請求的呼叫者。
type Authenticated struct {
	Principal Principal
}

// ErrInvalidToken 表示 token 無效：簽章、issuer、audience 或期限不對。其他錯誤表示驗證時 provider 沒有回應。
var ErrInvalidToken = errors.New("identity：token 無效")

// Clerk 是顧客 Clerk application 的 client。
type Clerk interface {
	// Issuer 是這個 application 簽的 session token 的 issuer。
	Issuer() string
	// OwnsUser 回傳使用者的 ID 是不是這個 application 發的。
	OwnsUser(id string) bool
	// Verify 驗證 session token，回傳使用者的 subject 與建帳資料。token 無效時回傳包著 ErrInvalidToken 的錯誤；其他錯誤表示問不到公鑰。
	Verify(ctx context.Context, token string) (ClerkIdentity, error)
	// BanUser 停用使用者：撤銷他所有的 session，之後不能再登入。使用者不存在時成功。
	BanUser(ctx context.Context, id string) error
	// DeleteUser 刪除使用者。已經不存在時成功。
	DeleteUser(ctx context.Context, id string) error
}

// ClerkIdentity 是驗證過的 Clerk session claims。
type ClerkIdentity struct {
	Subject string
	// Email 是 session 的 email claim；沒有時是空字串（例如 LINE 沒有給 email）。
	Email string
	// Name 是姓名；沒有時是空字串。
	Name string
}

// AdminIdentity 是已驗證的後台身分。email 只供顯示，識別以 subject 為準。
type AdminIdentity struct {
	Subject string
	Email   string
}

// AdminVerifier 驗證 Access 或本機 dev admin token。
type AdminVerifier interface {
	Issuer() string
	Verify(ctx context.Context, token string) (AdminIdentity, error)
}

// Identity 是驗證過的 Google 或 Apple ID token 的內容。
type Identity struct {
	Subject       string
	Email         string
	EmailVerified bool
	// Audience 是 token 簽給哪個 client（aud）：Apple 以它換 token 與撤銷。
	Audience string
	// Nonce 是 token 的 nonce claim，沒有時是空字串；IssuedAt 是 iat，沒有時是零值。
	Nonce    string
	IssuedAt time.Time
}

// IDTokenVerifier 驗證 Google 或 Apple 的 ID token。token 無效時回傳包著 ErrInvalidToken 的錯誤。
type IDTokenVerifier interface {
	Verify(ctx context.Context, idToken string) (Identity, error)
}

// AppleTokens 以 Sign in with Apple 的 authorization code 換 refresh token，並撤銷它。clientID 是 ID token 的 audience。
type AppleTokens interface {
	// Exchange 回傳 refresh token 與 Apple 同一次簽發的 ID token。
	Exchange(ctx context.Context, clientID, authorizationCode string) (AppleGrant, error)
	Revoke(ctx context.Context, clientID, refreshToken string) error
}

// AppleGrant 是 authorization code 換得的 token。
type AppleGrant struct {
	RefreshToken string
	IDToken      string
}

// Sealer 加密排入 job 與保存的第三方憑證（Apple 的 authorization code 與 refresh token）。aad 是密文認證的脈絡（用途與帳號），
// 解密時必須給同一份，密文搬到別的列、欄或帳號就解不開。密文自帶金鑰 ID，換金鑰之後舊的密文仍解得開。
type Sealer interface {
	Seal(plaintext, aad []byte) ([]byte, error)
	Open(ciphertext, aad []byte) ([]byte, error)
	// Has 回報金鑰環有沒有這個金鑰 ID。
	Has(keyID string) bool
}

// Providers 是外部的 provider。沒有設定的欄位是 nil 或空的。
type Providers struct {
	// Customers 是顧客的 Clerk application；Admin 是後台的 Access 或本機 dev verifier。
	Customers []Clerk
	Admin     AdminVerifier
	// Google 驗證 vendor portal step-up 的 ID token（單體入口的 VerifySession）；nil 時回 unavailable。
	Google IDTokenVerifier
	// Apple、AppleTokens 與 Sealer 給 Sign in with Apple 的授權碼交換、保存與刪帳時撤銷（LinkAppleCredential）；
	// 任一個是 nil 時 LinkAppleCredential 回 unavailable，刪帳照常（沒有保存的 token 就沒有東西可撤銷）。
	Apple       IDTokenVerifier
	AppleTokens AppleTokens
	Sealer      Sealer
}

// Account 是帳號。存活的帳號都是買家；刪除的帳號只留墓碑：沒有 email 與角色。
type Account struct {
	ID int64
	// Email 是建立帳號時 Clerk 回報的已驗證 email；沒有時是空字串。只給後台顯示，不是識別。
	Email string
	// Roles 是另外申請的角色，依字母排序。
	Roles     []Role
	Suspended bool
	// Lifecycle 是帳號的生命週期；Deleted 等於 Lifecycle == LifecycleDeleted。
	Lifecycle Lifecycle
	Deleted   bool
	CreatedAt time.Time
}

// Lifecycle 是帳號的生命週期：ACTIVE → PENDING_DELETION → DELETED。
type Lifecycle string

const (
	// LifecycleActive 存活的帳號。
	LifecycleActive Lifecycle = "ACTIVE"
	// LifecyclePendingDeletion 已受理刪除申請、等待未結訂單結束；仍可登入，但不能再授予角色。
	LifecyclePendingDeletion Lifecycle = "PENDING_DELETION"
	// LifecycleDeleted 已刪除，只留墓碑。
	LifecycleDeleted Lifecycle = "DELETED"
)

// SuspensionKind 是停權的型別。值等於 account_suspensions.kind 的 CHECK。
type SuspensionKind string

// SuspensionPermanent 是永久停權，只能手動解除；A7 沒有定期停權（A7-D16）。
const SuspensionPermanent SuspensionKind = "PERMANENT"

// SuspendInput 是停權帳號的輸入。
type SuspendInput struct {
	AccountID int64
	// Category 是停權的理由類別，必須是 policy.Values() 的值。
	Category policy.Category
	// Reason 是理由的說明，選填，規則同 platform/auditreason。
	Reason string
	// SourceRef 是來源（例如 moderation 的案件）的不透明參照，identity 不解讀它；選填，去掉前後空白後 1 到 200 字。
	SourceRef string
}

// AccountSuspension 是帳號的一筆停權紀錄。
type AccountSuspension struct {
	ID        uuid.UUID
	AccountID int64
	Kind      SuspensionKind
	Category  policy.Category
	Reason    string
	// SourceRef 沒有來源時是空字串。
	SourceRef   string
	SuspendedAt time.Time
	// ReleasedAt 與 ReleaseReason 在未解除時是 nil 與空字串。
	ReleasedAt    *time.Time
	ReleaseReason string
}

// ReleaseReasonAccountDeleted 是帳號刪除時解除未解除紀錄的 release_reason；手動解除的原因由呼叫方說明，不會是這個值。
const ReleaseReasonAccountDeleted = "ACCOUNT_DELETED"

// RemoveRoleInput 是移除角色的輸入。移除角色不會刪除帳號：帳號活著就是買家，刪帳由 accountdeletion 編排。
type RemoveRoleInput struct {
	AccountID int64
	Role      Role
	// SourceID 是要撤銷的那次授予的來源；目前的授予屬於另一個（較新的）來源時不移除（遲到的撤銷
	// 不能蓋掉重新入駐後的授予，#3189）。
	SourceID *int64
}

// GrantRoleInput 是授予帳號角色的輸入。
type GrantRoleInput struct {
	AccountID int64
	Role      Role
	SourceID  *int64
}

// Admin 是管理員。
type Admin struct {
	ID    int64
	Email string
}

// AdminRoleAction 是管理員角色的一次變更：授予或撤銷。值等於 admin_role_events.action 的 CHECK。
type AdminRoleAction string

const (
	AdminRoleActionGrant  AdminRoleAction = "GRANT"
	AdminRoleActionRevoke AdminRoleAction = "REVOKE"
)

// AdminRoleSource 是變更管理員角色的來源。值等於 admin_role_events.source 的 CHECK。目前只有 CLI：CLI 沒有經過驗證的
// 操作者，經驗證的操作者在授予搬進後台 UI 時才有（ADR 2026-10-04-admin-roles）。
type AdminRoleSource string

const AdminRoleSourceCLI AdminRoleSource = "CLI"

// AdminRoleChangeInput 是授予或撤銷管理員角色的輸入。
type AdminRoleChangeInput struct {
	// AdminID 與 Email 擇一：指出已經存在的管理員。
	AdminID int64
	Email   string
	Role    adminperm.Role
	// Reason 是必填的理由，規則同 platform/auditreason。
	Reason string
	Source AdminRoleSource
	// OSUser 是執行者的作業系統使用者，未經驗證、只供參考；取不到時是空字串。
	OSUser string
}

// AdminRoleChange 是已完成的角色變更。
type AdminRoleChange struct {
	AdminID int64
	Role    adminperm.Role
	Action  AdminRoleAction
	// HolderRemains 只對撤銷有意義：變更之後還有沒有任何管理員持有這個角色。
	HolderRemains bool
}

// AdminRoleRevoked 在撤銷管理員角色的 transaction 內發出（platform/events）：訂閱者的處理與撤銷一起 commit 或 rollback。
type AdminRoleRevoked struct {
	AdminID   int64
	Role      adminperm.Role
	RevokedAt time.Time
}

// AccountFilter 是後台帳號清單的篩選。
type AccountFilter struct {
	// Role 只列出有這個角色的帳號；空字串是全部。
	Role Role
	// Query 是帳號 ID（完全相符），或 email 包含的文字（不分大小寫）；空字串不篩選。
	Query string
	// Email 只列出 email 完全相符的帳號（不分大小寫，比對前去掉前後空白）；空字串不篩選。
	Email string
}

// AccountCursor 是帳號清單上一頁最後一筆的排序鍵。
type AccountCursor struct {
	CreatedAt time.Time
	ID        int64
}

// AccountPage 是帳號清單的一頁。
type AccountPage struct {
	Accounts []Account
	// Next 是下一頁的起點；nil 表示這是最後一頁。
	Next *AccountCursor
	// Total 是符合篩選的帳號總數（後台以它顯示頁數）。
	Total int64
}

// ListAccountsInput 是帳號清單的篩選與分頁輸入。
type ListAccountsInput struct {
	Filter AccountFilter
	After  *AccountCursor
	Size   int32
}

// LinkAppleCredentialInput 是交出 Apple 憑證的輸入。
type LinkAppleCredentialInput struct {
	AccountID         int64
	IDToken           string
	AuthorizationCode string
}

// AccountCreated 在建帳的 transaction 內發出，訂閱者以相同 transaction 建立業務身分。
type AccountCreated struct {
	AccountID int64
	Name      string
}

// AccountDeleted 在帳號刪除的 transaction 內發出（platform/events）：訂閱者的處理與刪除一起 commit 或
// rollback。過渡期由 internal/legacy 排入 job，送到舊服務訂閱的 RabbitMQ（account.deleted）。
type AccountDeleted struct {
	AccountID int64
	DeletedAt time.Time
}

// ClerkConfig 是一個 Clerk application 的設定。
type ClerkConfig struct {
	PublishableKey string
	SecretKey      string
	JWTKey         string
	// AuthorizedParties 是 web 的 origin：web 取得的 token 帶 azp，必須是其中之一。
	AuthorizedParties []string
}

// AppleConfig 是 Apple 開發者帳號的 Sign in with Apple 金鑰。
type AppleConfig struct {
	TeamID        string
	KeyID         string
	PrivateKeyPEM string
	// RedirectURIs 是 web 的 Services ID 授權時用的 redirect URI（換 token 時要帶同一個）。
	RedirectURIs map[string]string
}

// AccessConfig 設定後台 Access application 的 JWT 驗證。
type AccessConfig struct {
	TeamDomain string
	Audience   string
	HTTPClient *http.Client
	Context    context.Context
}

// ErrRejected 表示 provider 拒絕請求，且重試不會成功。
var ErrRejected = errors.New("identity：provider 拒絕，重試不會成功")
