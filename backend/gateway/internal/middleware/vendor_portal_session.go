package middleware

import (
	"strconv"
	"time"

	"github.com/Will413028/hawkr/gateway/internal/response"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// 入口（單體）驗過 vendor portal session 後寫在請求上的 header。入口會先刪掉
// 用戶端送來的所有 X-Vendor-Portal-*，所以 gateway 只信任這裡的值。
const (
	VendorPortalRecipientIDHeader        = "X-Vendor-Portal-Rfq-Recipient-Id"
	VendorPortalSessionIDHeader          = "X-Vendor-Portal-Session-Id"
	VendorPortalVerificationMethodHeader = "X-Vendor-Portal-Verification-Method"
	VendorPortalVerifiedEmailHeader      = "X-Vendor-Portal-Verified-Email"
	VendorPortalVerifiedAtHeader         = "X-Vendor-Portal-Verified-At"
)

const (
	vendorPortalRecipientIDKey = "vendorPortalRfqRecipientId"
	vendorPortalSessionKey     = "vendorPortalSession"
)

// VendorPortalVerificationMethodGoogle 是入口以 Google ID token 證明業者持有
// 受邀信箱的驗證方式。
const VendorPortalVerificationMethodGoogle = "GOOGLE"

// VendorPortalVerification 是入口附上的驗證紀錄；四個欄位一起存在。
type VendorPortalVerification struct {
	Method     string
	Email      string
	VerifiedAt time.Time
	SessionID  string
}

type vendorPortalSessionContext struct {
	sessionID    string
	verification *VendorPortalVerification
}

// VendorPortalSession 讀入口寫上的 header，把收件人與驗證紀錄放進 gin context 供
// handler 讀取。沒有收件人 header ＝ 沒有有效的 session；任何 header 的值不合格式
// 也一律 401。
func VendorPortalSession() gin.HandlerFunc {
	return func(c *gin.Context) {
		recipientID, sess, ok := parseVendorPortalHeaders(c)
		if !ok {
			response.AbortUnauthorized(c, "unauthorized")
			return
		}
		c.Set(vendorPortalRecipientIDKey, recipientID)
		c.Set(vendorPortalSessionKey, sess)
		c.Next()
	}
}

func parseVendorPortalHeaders(c *gin.Context) (string, vendorPortalSessionContext, bool) {
	raw := c.GetHeader(VendorPortalRecipientIDHeader)
	id, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || id <= 0 || strconv.FormatInt(id, 10) != raw {
		return "", vendorPortalSessionContext{}, false
	}
	sessionID, err := uuid.Parse(c.GetHeader(VendorPortalSessionIDHeader))
	if err != nil || sessionID == uuid.Nil {
		return "", vendorPortalSessionContext{}, false
	}
	sess := vendorPortalSessionContext{sessionID: sessionID.String()}

	method := c.GetHeader(VendorPortalVerificationMethodHeader)
	email := c.GetHeader(VendorPortalVerifiedEmailHeader)
	at := c.GetHeader(VendorPortalVerifiedAtHeader)
	if method == "" && email == "" && at == "" {
		return raw, sess, true
	}
	verifiedAt, err := time.Parse(time.RFC3339, at)
	if method != VendorPortalVerificationMethodGoogle || email == "" || err != nil {
		return "", vendorPortalSessionContext{}, false
	}
	sess.verification = &VendorPortalVerification{
		Method: method, Email: email, VerifiedAt: verifiedAt.UTC(), SessionID: sess.sessionID,
	}
	return raw, sess, true
}

// VendorPortalRecipientIDFrom 回入口驗過的收件人 ID；VendorPortalSession 沒有先執行時
// 回空字串。
func VendorPortalRecipientIDFrom(c *gin.Context) string {
	raw, _ := c.Get(vendorPortalRecipientIDKey)
	id, _ := raw.(string)
	return id
}

// VendorPortalSessionIDFrom 回入口驗過的 session ID；VendorPortalSession 沒有先執行時
// 回空字串。
func VendorPortalSessionIDFrom(c *gin.Context) string {
	raw, _ := c.Get(vendorPortalSessionKey)
	sess, _ := raw.(vendorPortalSessionContext)
	return sess.sessionID
}

// VendorPortalVerificationFrom 回這個請求的驗證紀錄；session 沒有驗證過（或
// VendorPortalSession 沒有先執行）時 ok 為 false。
func VendorPortalVerificationFrom(c *gin.Context) (VendorPortalVerification, bool) {
	raw, _ := c.Get(vendorPortalSessionKey)
	sess, _ := raw.(vendorPortalSessionContext)
	if sess.verification == nil {
		return VendorPortalVerification{}, false
	}
	return *sess.verification, true
}

// RequireVendorPortalGoogleVerified 擋住還沒有 Google 驗證紀錄的 session。
//
// 🔴 **只掛在「正式寫入」那幾條上，不掛草稿**：草稿可覆寫，用邀請連結那一級即可；
// 正式送出產生的 sellerResponseVersion 是 immutable、買家接受 exact version 即商務
// award ⇒ 不可撤回的商務承諾不能只靠一條七天有效的邀請連結（拿到信的任何人都能代
// 該業者承諾價格）。
//
// ⚠️ 它讀的是 VendorPortalSession 放進 context 的值，所以**必須掛在它後面**。
// 單獨掛的話 context 沒有驗證紀錄 ⇒ 擋下——fail-closed 是結構性的，不靠順序
// 寫對。`TestRequireVendorPortalGoogleVerified_DeniesWithoutSessionMiddleware` 守著它。
func RequireVendorPortalGoogleVerified() gin.HandlerFunc {
	return func(c *gin.Context) {
		v, ok := VendorPortalVerificationFrom(c)
		if !ok || v.Method != VendorPortalVerificationMethodGoogle {
			// ⚠️ 403 而不是 401：session 本身是有效的，只是還沒有驗證紀錄。回 401 會
			// 讓前端把人踢回兌換邀請連結那一步，而那一步不會產生驗證紀錄，於是形成迴圈。
			response.AbortForbidden(c, "vendor portal step-up required")
			return
		}
		c.Next()
	}
}
