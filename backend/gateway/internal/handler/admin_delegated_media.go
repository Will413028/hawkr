package handler

import (
	"strconv"
	"strings"

	"github.com/Will413028/hawkr/gateway/internal/middleware"
	"github.com/Will413028/hawkr/gateway/internal/response"
	"github.com/Will413028/hawkr/go-pkg/errors/bizcode"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// 代行的媒體上傳（56 §6.1.13）。三步：① 建 upload session ② 瀏覽器 PUT 直送 R2
// （不經任何 BFF，預簽 URL 自帶授權）③ complete 換 stagedToken。這裡是 ①③。
//
// 為什麼不沿用 /api/v1/app/media/upload_sessions：原定版假設工作區在
// hawkr-admin（那裡的瀏覽器持有 ADM session cookie）；工作區移到
// hawkr-website 之後，www 上的瀏覽器沒有任何 ADM 憑證（__Host- cookie 是
// host-only），它唯一持有的是 operation credential ⇒ 照原文施工的後果是代行工作區裡
// 一張圖都上傳不了，而症狀是 401，看起來像權限設定錯誤而不是設計缺口。
//
// guard 是 requireDelegatedSessionOnly：只驗 credential 有效 ＋ session 未終態，
// 不驗 capability、不驗 target（upload session 不寫業務資料、不碰 target store，
// 沒有可授權的對象）⇒ 九值封閉列舉不擴充。
//
// 「不挑 capability」與「算不算活動」是兩件事：這兩支一律 counts_as_activity = true
// （由 guard 決定，不是這裡）。換一批商品圖正是最容易停留超過一個 idle 窗口、又不打
// 任何 delegatedHandle 路由的動作——步 ② 直送 R2，一次 gateway 都沒打。
//
// 上傳本身不落稽核：一次沒有被掛上去的上傳什麼都沒動。被記下來的是引用它的那次
// mutation（changed_fields 記 logoKey／coverKey／images，值是 objectKey）。

// DelegatedMediaUploadSessionRequest is the body of POST .../media/upload_sessions.
type DelegatedMediaUploadSessionRequest struct {
	Filename    string `json:"filename"`
	ContentType string `json:"contentType"`
	FileSize    string `json:"fileSize"`
}

// DelegatedMediaUploadSessionData is what the browser needs for step ②.
type DelegatedMediaUploadSessionData struct {
	SessionID string `json:"sessionId"`
	UploadURL string `json:"uploadUrl"`
	ExpiresIn string `json:"expiresIn"`
}

// DelegatedMediaCompleteData carries the staged token plus the preview URL.
//
// stagedToken is a JWT-shaped short-lived credential: never logged, never in a
// snapshot, never in changed_fields.
//
// previewUrl is not optional decoration — the shared browser client
// (website `lib/media/upload.ts`) fail-fasts when either field is missing, so
// omitting it made every delegated upload (logo／cover／貼文附圖／商品圖) fail at
// the last step with 「上傳完成回應缺少 previewUrl 或 stagedToken」, after the bytes
// were already in R2. 賣家自助那支（media.go）一直都回兩個欄位；這支漏掉一個，
// 兩邊對同一個 client 給出不同形狀的回應。
type DelegatedMediaCompleteData struct {
	StagedToken string `json:"stagedToken"`
	PreviewURL  string `json:"previewUrl"`
}

// CreateDelegatedMediaUploadSession handles POST .../delegated-operations/media/upload_sessions.
func (h *Holder) CreateDelegatedMediaUploadSession(c *gin.Context) {
	adminID, ok := delegatedMediaOwner(c)
	if !ok {
		return
	}
	var body DelegatedMediaUploadSessionRequest
	if _, ok := bindDelegatedBody(c, &body); !ok {
		return
	}
	if strings.TrimSpace(body.Filename) == "" || strings.TrimSpace(body.ContentType) == "" {
		response.BizStatus(c, bizcode.DelegationInvalidCommand, "filename 與 contentType 為必填")
		return
	}
	if body.FileSize != "" && !nonNegativeInteger(body.FileSize) {
		response.BizStatus(c, bizcode.DelegationInvalidCommand, "fileSize 格式不正確")
		return
	}
	// 9289 而不是 9280（#2744）：依賴未接線代表**這個 Gateway 版本**沒把它 wire 起來，
	// 重試一萬次都一樣，要改 code 重新部署。9280 說「稍後再試」，於是外部看起來與上游
	// 抖動完全相同：操作者一直重試、值班的人看 424 上升卻在上游查不到任何異常。
	// 同一份 code 的 admin_delegated_common.go:370 早就寫著「把永久失敗講成可重試是
	// 不可接受的」，那條紀律只被套用在上游拒絕的翻譯，沒有套用在自己的接線檢查上。
	if h.Media == nil {
		zap.L().Error("Delegation.dependency.unwired", zap.String("dep", "Media"), zap.String("op", "CreateUploadSession"))
		response.BizStatus(c, bizcode.DelegationInternalContractMismatch, "代行營運服務設定異常，重試無效")
		return
	}

	resp, err := h.Media.CreateUploadSession(c.Request.Context(), body.Filename, body.ContentType, body.FileSize,
		adminID,
		// 機敏檔一律不走代行：代行只上傳店面／商品／貼文的公開圖。
		false)
	if err != nil {
		// media-service 的業務碼與領域服務走同一條還原路徑：翻譯成 9280 會把
		// 「檔案類型不支援」這種可恢復的拒絕講成「服務暫時不可用，請稍後再試」。
		denyDelegated(c, denialFromGrpcErr(err))
		return
	}
	if resp.GetCode() != 0 {
		response.BizStatus(c, resp.GetCode(), resp.GetMessage())
		return
	}
	response.OK(c, DelegatedMediaUploadSessionData{
		SessionID: resp.GetSessionId(),
		UploadURL: resp.GetUploadUrl(),
		ExpiresIn: resp.GetExpiresIn(),
	})
}

// CompleteDelegatedMediaUploadSession handles POST
// .../delegated-operations/media/upload_sessions/:session_id/complete.
func (h *Holder) CompleteDelegatedMediaUploadSession(c *gin.Context) {
	adminID, ok := delegatedMediaOwner(c)
	if !ok {
		return
	}
	sessionID := c.Param("session_id")
	if sessionID == "" {
		response.BizStatus(c, bizcode.DelegationInvalidCommand, "缺少上傳工作階段代號")
		return
	}
	// 同上：未接線是永久失敗，不是上游抖動（#2744）。
	if h.Media == nil {
		zap.L().Error("Delegation.dependency.unwired", zap.String("dep", "Media"), zap.String("op", "CompleteUploadSession"))
		response.BizStatus(c, bizcode.DelegationInternalContractMismatch, "代行營運服務設定異常，重試無效")
		return
	}

	resp, err := h.Media.CompleteUploadSession(c.Request.Context(), sessionID, adminID)
	if err != nil {
		// media-service 的業務碼與領域服務走同一條還原路徑：翻譯成 9280 會把
		// 「檔案類型不支援」這種可恢復的拒絕講成「服務暫時不可用，請稍後再試」。
		denyDelegated(c, denialFromGrpcErr(err))
		return
	}
	if resp.GetCode() != 0 {
		response.BizStatus(c, resp.GetCode(), resp.GetMessage())
		return
	}
	response.OK(c, DelegatedMediaCompleteData{
		StagedToken: resp.GetStagedToken(),
		PreviewURL:  resp.GetPreviewUrl(),
	})
}

// delegatedMediaOwner resolves the upload owner: the ADM operator's admin_accounts.id.
//
// 歸屬判定是「同一位 ADM 操作者」，不是「同一個工作階段」：media 沒有工作階段概念、
// upload_session 也沒有 store 欄位，那兩個寫法都不可實作。同一位 ADM 上傳、同一位
// ADM 掛載 ⇒ 轉正通過；換一位 ADM ⇒ 失敗（media 回 NOT_FOUND）。
//
// 出站身分不在這裡填：credential-only 群組沒有 JWT、沒有 ADM cookie，applyIdentity
// 從未跑過，x-account-id 一路是空的，而**掛載那一跳也需要同一個身分**（領域服務代打
// FinalizeImages，比對的是 upload_session.account_id == caller 的 x-account-id）。
// 所以注入點統一在 middleware.SetDelegationSession——這兩支自己再填一次會變成第二個
// 機制，兩份必然漂移。本函式只負責把 admin id 交給 request payload 的 accountId 欄。
func delegatedMediaOwner(c *gin.Context) (int64, bool) {
	session, ok := middleware.DelegationSession(c)
	if !ok {
		response.BizStatus(c, bizcode.DelegationCredentialInvalid, "代行憑證無效，請重新建立工作階段")
		return 0, false
	}
	adminID, err := strconv.ParseInt(session.GetActor().GetAdminId(), 10, 64)
	if err != nil || adminID <= 0 {
		response.BizStatus(c, bizcode.DelegationCredentialInvalid, "代行憑證無效，請重新建立工作階段")
		return 0, false
	}
	return adminID, true
}
