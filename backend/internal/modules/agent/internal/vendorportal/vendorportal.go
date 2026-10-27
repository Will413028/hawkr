// Package vendorportal 是 vendor portal（供應商從 RFQ 邀請信進來看詢價、寫草稿、正式送出）的存取模型：以「邀請」為單位的
// 存取、可重複兌換的連結、server-side session 與 email 驗證紀錄（ADR 2026-10-03-vendor-portal-invite-scoped-access）。來源是
// seller-service 的 vendor_portal_*（遷移計畫 A3b 步驟 7d）；沿用與改動的逐項判斷在模組的 CARRYOVER.md。
//
// 存取範圍就是一張邀請（一個 rfq_recipient_id）：session 只屬於那張邀請，驗證也綁那張邀請，所以同一個業者的兩張 RFQ 互不影響。
// 秘密（邀請 secret、session token）都是 32 bytes 亂數、base64url 編碼，資料庫只存原始 bytes 的 SHA-256；查詢一律以 hash
// 為鍵，不需要常數時間比較。呼叫端拿到的都是不透明字串。
package vendorportal

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"time"
	"uuid"

	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
	"github.com/Will413028/hawkr/backend/internal/platform/db"
)

// Contact 是 prospect 的聯絡方式與狀態：Active 表示業者還在進行中；Email 沒有時是 nil。
type Contact struct {
	Active bool
	Email  *string
}

// Contacts 是 prospect 的聯絡方式 port（由 seller 的 ProspectContact 實作，根套件轉接）。prospect 不存在回 NotFound。實作必須
// 在呼叫方的 transaction 內對 prospect 列取共享鎖（讀到 Active 之後到呼叫方提交之前，封存與合併被擋住），Ensure 靠這點與
// 封存序列化。
type Contacts interface {
	ProspectContact(ctx context.Context, prospectID int64) (Contact, error)
}

// Deps 外部依賴；clock 與 ID 產生器由組裝點注入。
type Deps struct {
	DB       *db.DB
	Contacts Contacts
	Now      func() time.Time
	NewID    func() uuid.UUID
}

// Service 是 vendor portal 的 use case。
type Service struct{ d Deps }

// New 建立服務。
func New(d Deps) *Service { return &Service{d: d} }

// 對外一律不分原因的錯誤：猜 secret 的人不能從回應分辨「不存在」「過期」「撤銷」。
func errInviteNotUsable() error { return apperr.NotFound("邀請無法使用") }
func errSessionInvalid() error  { return apperr.Unauthenticated("登入已失效") }
func errVerifyDenied() error    { return apperr.PermissionDenied("無法驗證這個 session") }

const secretBytes = 32

var encoding = base64.RawURLEncoding.Strict()

// newSecret 產生 32 bytes 的 crypto/rand 秘密，回傳 base64url（無 padding）字串與要存的 SHA-256。
func newSecret() (string, []byte, error) {
	raw := make([]byte, secretBytes)
	if _, err := rand.Read(raw); err != nil {
		return "", nil, err
	}
	sum := sha256.Sum256(raw)
	return encoding.EncodeToString(raw), sum[:], nil
}

// hashSecret 解開呼叫端給的字串並回傳 SHA-256；不是剛好 32 bytes 的標準編碼就回 false（不必查資料庫）。
func hashSecret(s string) ([]byte, bool) {
	raw, err := encoding.DecodeString(s)
	if err != nil || len(raw) != secretBytes {
		return nil, false
	}
	sum := sha256.Sum256(raw)
	return sum[:], true
}

func requirePositive(name string, v int64) error {
	if v <= 0 {
		return apperr.InvalidInputOn([]string{name}, "%s 必須是正整數", name)
	}
	return nil
}
