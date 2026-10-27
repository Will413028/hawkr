package handler

import (
	"net/http"

	"github.com/Will413028/hawkr/gateway/internal/response"
	"github.com/gin-gonic/gin"
)

// ========== gRPC 上傳會話相關 ==========

// CreateUploadSessionInput 建立上傳會話請求
type CreateUploadSessionInput struct {
	Filename    string `json:"filename" binding:"required"`
	ContentType string `json:"contentType" binding:"required"`
	FileSize    string `json:"fileSize" binding:"required"`
	IsSecure    bool   `json:"isSecure"`
}

// CreateUploadSessionOutput 建立上傳會話回應
type CreateUploadSessionOutput struct {
	SessionID string `json:"sessionId"`
	UploadURL string `json:"uploadUrl,omitempty"`
	ExpiresIn string `json:"expiresIn"`
	IsSecure  bool   `json:"isSecure"`
}

// CompleteUploadSessionOutput 完成上傳會話回應
type CompleteUploadSessionOutput struct {
	StagedToken string `json:"stagedToken"`
	PreviewUrl  string `json:"previewUrl"`
}

// CreateUploadSession 建立上傳會話（gRPC）
// @Summary      建立上傳會話
// @Description  建立媒體檔案的上傳會話，取得上傳 URL
// @Tags         媒體 (Media)
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        request  body      handler.CreateUploadSessionInput  true  "上傳會話請求資料"
// @Success      200      {object}  object{code=int,data=handler.CreateUploadSessionOutput}
// @Failure      400      {object}  object{code=int,message=string}
// @Failure      401      {object}  object{code=int,message=string}
// @Router       /app/media/upload_sessions [post]
func (h *Holder) CreateUploadSession(c *gin.Context) {
	// 從 JWT context 取得 accountId
	accountIDRaw, exists := c.Get("accountId")
	if !exists {
		response.Unauthorized(c)
		return
	}

	accountID, ok := accountIDRaw.(int64)
	if !ok {
		// 嘗試其他類型轉換
		if uid, ok := accountIDRaw.(float64); ok {
			accountID = int64(uid)
		} else {
			response.Err(c, http.StatusInternalServerError, 500, "invalid account id type")
			return
		}
	}

	// 解析請求
	var req CreateUploadSessionInput
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "invalid request: "+err.Error())
		return
	}

	// 呼叫 gRPC
	resp, err := h.Media.CreateUploadSession(
		c.Request.Context(),
		req.Filename,
		req.ContentType,
		req.FileSize,
		accountID,
		req.IsSecure,
	)
	if err != nil {
		response.GrpcErr(c, "media", err)
		return
	}

	if resp.Code != 0 {
		response.GrpcResult(c, resp.Code, resp.Message, nil)
		return
	}

	// 回傳結果
	response.OK(c, CreateUploadSessionOutput{
		SessionID: resp.SessionId,
		UploadURL: resp.UploadUrl,
		ExpiresIn: resp.ExpiresIn,
		IsSecure:  resp.IsSecure,
	})
}

// CompleteUploadSession 完成上傳會話（gRPC）
// @Summary      完成上傳會話
// @Description  完成指定的上傳會話，取得暫存 Token 與預覽 URL
// @Tags         媒體 (Media)
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id   path      string  true  "上傳會話 ID"
// @Success      200  {object}  object{code=int,data=CompleteUploadSessionOutput}
// @Failure      400  {object}  object{code=int,message=string}
// @Failure      401  {object}  object{code=int,message=string}
// @Failure      404  {object}  object{code=int,message=string}
// @Router       /app/media/upload_sessions/{id}/complete [post]
func (h *Holder) CompleteUploadSession(c *gin.Context) {
	// 從 JWT context 取得 accountId
	accountIDRaw, exists := c.Get("accountId")
	if !exists {
		response.Unauthorized(c)
		return
	}

	accountID, ok := accountIDRaw.(int64)
	if !ok {
		if uid, ok := accountIDRaw.(float64); ok {
			accountID = int64(uid)
		} else {
			response.Err(c, http.StatusInternalServerError, 500, "invalid account id type")
			return
		}
	}

	// 取得 session ID
	sessionID := c.Param("id")
	if sessionID == "" {
		response.BadRequest(c, "session id is required")
		return
	}

	// 呼叫 gRPC
	resp, err := h.Media.CompleteUploadSession(c.Request.Context(), sessionID, accountID)
	if err != nil {
		response.GrpcErr(c, "media", err)
		return
	}

	if resp.Code != 0 {
		response.GrpcResult(c, resp.Code, resp.Message, nil)
		return
	}

	// 回傳結果
	response.OK(c, gin.H{
		"stagedToken": resp.StagedToken,
		"previewUrl":  resp.PreviewUrl,
	})
}
