package handler

import (
	"context"
	"io"
	"net/http"
	"strconv"
	"strings"

	grpcclient "github.com/Will413028/hawkr/gateway/internal/grpc"
	"github.com/Will413028/hawkr/gateway/internal/proxy"
	"github.com/Will413028/hawkr/gateway/internal/response"
	"github.com/Will413028/hawkr/go-pkg/mediaurl"
	pb "github.com/Will413028/hawkr/go-pkg/proto/ai"
	"github.com/gin-gonic/gin"
)

type AiStyleItem struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	Label        string `json:"label"`
	Prompt       string `json:"prompt"`
	ThumbnailKey string `json:"thumbnailKey"`
	ThumbnailURL string `json:"thumbnailUrl"`
	SortOrder    string `json:"sortOrder"`
}

type AiStyleListResponse struct {
	DefaultPrompt string        `json:"defaultPrompt"`
	WeeklyUsage   string        `json:"weeklyUsage"`
	WeeklyLimit   string        `json:"weeklyLimit"`
	Styles        []AiStyleItem `json:"styles"`
}

type AiStyleAdminItem struct {
	AiStyleItem
	CreatedAt string `json:"createdAt"`
	UpdatedAt string `json:"updatedAt"`
	Category  string `json:"category"`
	IsDefault bool   `json:"isDefault"`
	Enabled   bool   `json:"enabled"`
}

type CreateAiStyleRequest struct {
	Name                 string `json:"name"`
	Label                string `json:"label"`
	Prompt               string `json:"prompt"`
	ThumbnailStagedToken string `json:"thumbnailStagedToken,omitempty"`
	SortOrder            string `json:"sortOrder"`
	Category             string `json:"category"`
	IsDefault            bool   `json:"isDefault"`
	Enabled              bool   `json:"enabled"`
}

type UpdateAiStyleRequest struct {
	Label                *string `json:"label"`
	Prompt               *string `json:"prompt"`
	ThumbnailStagedToken *string `json:"thumbnailStagedToken,omitempty"`
	SortOrder            *string `json:"sortOrder"`
	Enabled              *bool   `json:"enabled"`
}

type AiTransformResponse struct {
	ImageBase64 string `json:"imageBase64"`
	MimeType    string `json:"mimeType"`
	WeeklyUsage string `json:"weeklyUsage"`
	WeeklyLimit string `json:"weeklyLimit"`
}

type AiQuotaResponse struct {
	StoreID     string `json:"storeId"`
	WeeklyLimit string `json:"weeklyLimit"`
	WeeklyUsage string `json:"weeklyUsage"`
	Memo        string `json:"memo"`
	UpdatedAt   string `json:"updatedAt"`
	IsCustom    bool   `json:"isCustom"`
}

type UpdateAiQuotaRequest struct {
	WeeklyLimit string `json:"weeklyLimit"`
	Memo        string `json:"memo"`
}

type assistantStreamRequest struct {
	SessionID string `json:"sessionId"`
	Message   string `json:"message"`
	ImageURL  string `json:"imageUrl"`
}

func (r assistantStreamRequest) toProto() *pb.AssistantRequest {
	return &pb.AssistantRequest{
		SessionId: r.SessionID,
		Message:   r.Message,
		ImageUrl:  r.ImageURL,
	}
}

type sellerProductAutofillRequestDTO struct {
	Text     string `json:"text"`
	ImageURL string `json:"imageUrl"`
}

type categoryCandidateDTO struct {
	CatalogID  string `json:"catalogId"`
	Path       string `json:"path"`
	Confidence string `json:"confidence"`
}

type skuDraftDTO struct {
	SkuName string `json:"skuName"`
	Price   string `json:"price"`
	Unit    string `json:"unit"`
}

type productDraftDTO struct {
	Name               string                 `json:"name"`
	Origin             string                 `json:"origin"`
	Description        string                 `json:"description"`
	CategoryCandidates []categoryCandidateDTO `json:"categoryCandidates"`
	Skus               []skuDraftDTO          `json:"skus"`
	Tags               []string               `json:"tags"`
	Warnings           []string               `json:"warnings"`
}

func toProductDraftDTO(d *pb.ProductDraft) productDraftDTO {
	out := productDraftDTO{
		Name:        d.GetName(),
		Origin:      d.GetOrigin(),
		Description: d.GetDescription(),
		Tags:        d.GetTags(),
		Warnings:    d.GetWarnings(),
	}
	for _, cc := range d.GetCategoryCandidates() {
		out.CategoryCandidates = append(out.CategoryCandidates, categoryCandidateDTO{
			CatalogID: cc.GetCatalogId(), Path: cc.GetPath(), Confidence: cc.GetConfidence(),
		})
	}
	for _, s := range d.GetSkus() {
		out.Skus = append(out.Skus, skuDraftDTO{SkuName: s.GetSkuName(), Price: s.GetPrice(), Unit: s.GetUnit()})
	}
	return out
}

type latestSessionMessageDTO struct {
	Role      string `json:"role"`
	Text      string `json:"text"`
	Timestamp string `json:"timestamp"`
}

type latestSessionResponseDTO struct {
	SessionID string                    `json:"sessionId"`
	Messages  []latestSessionMessageDTO `json:"messages"`
}

func toLatestSessionResponseDTO(r *pb.LatestSessionResponse) latestSessionResponseDTO {
	out := latestSessionResponseDTO{SessionID: r.GetSessionId()}
	for _, m := range r.GetMessages() {
		out.Messages = append(out.Messages, latestSessionMessageDTO{
			Role: m.GetRole(), Text: m.GetText(), Timestamp: m.GetTimestamp(),
		})
	}
	return out
}

// ProxyListAiStyles 取得 AI 修圖風格列表（App 用）
//
// @Summary      取得 AI 修圖風格列表
// @Description  回傳所有啟用的修圖風格 + defaultPrompt + 該帳號本週使用量
// @Tags         AI 修圖 (AI Image)
// @Produce      json
// @Security     BearerAuth
// @Success      200  {object}  object{code=int,data=AiStyleListResponse}
// @Failure      401  {object}  object{code=int,message=string}
// @Failure      502  {object}  object{code=int,message=string}
// @Router       /app/ai_styles [get]
func (h *Holder) ProxyListAiStyles(c *gin.Context) {
	storeID, ok := h.resolveStoreID(c)
	if !ok {
		return
	}
	category := c.Query("category")

	// #3519：storeID 直接交給 client，x-store-id metadata 由
	// AIClient.ListAiStyles 自己注入（見該方法註解——放在那裡才能讓漏帶身分
	// 變成編譯錯誤）。ai-service 從 metadata 解出 actor；request message 的
	// store_id 欄位已移除並 reserved，呼叫端無從指定「打誰的額度」。
	resp, err := h.AI.ListAiStyles(c.Request.Context(), storeID, category)
	if err != nil {
		response.GrpcErr(c, "ai", err)
		return
	}

	if resp.Code != 0 {
		response.GrpcResult(c, resp.Code, resp.Message, nil)
		return
	}

	var styles []AiStyleItem
	if resp.Data != nil {
		styles = make([]AiStyleItem, len(resp.Data.Styles))
		for i, s := range resp.Data.Styles {
			styles[i] = AiStyleItem{
				ID:           s.Id,
				Name:         s.Name,
				Label:        s.Label,
				Prompt:       s.Prompt,
				ThumbnailKey: s.ThumbnailKey,
				ThumbnailURL: mediaurl.BuildURL(h.MediaBaseURL, s.ThumbnailKey),
				SortOrder:    s.SortOrder,
			}
		}
	}

	response.OK(c, AiStyleListResponse{
		DefaultPrompt: resp.Data.GetDefaultPrompt(),
		WeeklyUsage:   resp.Data.GetWeeklyUsage(),
		WeeklyLimit:   resp.Data.GetWeeklyLimit(),
		Styles:        styles,
	})
}

// ProxyTransformImage AI 圖片轉換
//
// @Summary      AI 圖片轉換
// @Description  上傳圖片 + prompt，透過 Gemini AI 修圖。後端檢查額度，成功才扣。
// @Description
// @Description  失敗分類走業務碼（HTTP 409，讀 `code` 分流，勿依賴 HTTP status）：
// @Description  9250 = AI 這次沒產出圖片（可重試，後端已自動重試過一次）；
// @Description  9251 = 內容未通過安全性審查（不可重試，請換圖或改描述）；
// @Description  9252 = 產出與受版權保護內容過於相似（不可重試）；
// @Description  9253 = AI 服務暫時不可用（可稍後重試）。
// @Description  額度用完走 HTTP 429 + `code` 4291；限流的 429 則是 `code` 429，
// @Description  兩者只能靠 `code` 分辨，勿依賴 HTTP status。
// @Tags         AI 修圖 (AI Image)
// @Accept       multipart/form-data
// @Produce      json
// @Security     BearerAuth
// @Param        image   formData  file    true   "要轉換的圖片檔案"
// @Param        prompt  formData  string  true   "修圖提示詞（從 GET /app/ai_styles 取得）"
// @Success      200     {object}  object{code=int,data=AiTransformResponse}
// @Failure      400     {object}  object{code=int,message=string,traceId=string}
// @Failure      401     {object}  object{code=int,message=string,traceId=string}
// @Failure      409     {object}  object{code=int,message=string,messageZh=string,traceId=string}  "AI 修圖業務碼 9250~9253"
// @Failure      429     {object}  object{code=int,message=string,messageZh=string,traceId=string}  "本週額度已用完（code 4291）；限流則為 code 429"
// @Failure      503     {object}  object{code=int,message=string,traceId=string}
// @Router       /app/image_transformations [post]
func (h *Holder) ProxyTransformImage(c *gin.Context) {
	storeID, ok := h.resolveStoreID(c)
	if !ok {
		return
	}

	file, header, err := c.Request.FormFile("image")
	if err != nil {
		response.BadRequest(c, "image file is required")
		return
	}
	defer func() { _ = file.Close() }()

	imageData, err := io.ReadAll(file)
	if err != nil {
		response.BadRequest(c, "failed to read image")
		return
	}

	prompt := c.PostForm("prompt")
	if prompt == "" {
		response.BadRequest(c, "prompt is required")
		return
	}

	mimeType := header.Header.Get("Content-Type")
	if mimeType == "" {
		mimeType = "image/jpeg"
	}

	// #3519：同 ProxyListAiStyles——storeID 交給 client，metadata 由
	// AIClient.TransformImage 自己注入；request message 的 store_id 欄位
	// 同樣已移除並 reserved。
	resp, err := h.AI.TransformImage(c.Request.Context(), storeID, imageData, mimeType, prompt)
	if err != nil {
		response.GrpcDomainErr(c, "ai", err)
		return
	}

	response.OK(c, gin.H{
		"imageBase64": resp.Data.ImageBase64,
		"mimeType":    resp.Data.MimeType,
		"weeklyUsage": resp.Data.WeeklyUsage,
		"weeklyLimit": resp.Data.WeeklyLimit,
	})
}

// ProxyListAiStylesAdmin 取得全部 AI 修圖風格（Admin 用）
//
// @Summary      取得全部 AI 修圖風格（含停用）
// @Description  Admin 列出所有風格，含 disabled 與 default。
// @Tags         AI 修圖 (AI Image)
// @Produce      json
// @Security     BearerAuth
// @Success      200  {object}  object{code=int,data=[]AiStyleAdminItem}
// @Failure      401  {object}  object{code=int,message=string}
// @Failure      502  {object}  object{code=int,message=string}
// @Router       /admin/ai_styles [get]
func (h *Holder) ProxyListAiStylesAdmin(c *gin.Context) {
	category := c.Query("category")
	resp, err := h.AI.ListAiStylesForAdmin(c.Request.Context(), category)
	if err != nil {
		response.GrpcErr(c, "ai", err)
		return
	}

	if resp.Code != 0 {
		response.GrpcResult(c, resp.Code, resp.Message, nil)
		return
	}

	items := make([]AiStyleAdminItem, len(resp.Data))
	for i, s := range resp.Data {
		items[i] = AiStyleAdminItem{
			AiStyleItem: AiStyleItem{
				ID:           s.Id,
				Name:         s.Name,
				Label:        s.Label,
				Prompt:       s.Prompt,
				ThumbnailKey: s.ThumbnailKey,
				ThumbnailURL: mediaurl.BuildURL(h.MediaBaseURL, s.ThumbnailKey),
				SortOrder:    s.SortOrder,
			},
			CreatedAt: s.CreatedAt,
			UpdatedAt: s.UpdatedAt,
			Category:  s.Category,
			IsDefault: s.IsDefault,
			Enabled:   s.Enabled,
		}
	}

	response.OK(c, items)
}

// ProxyCreateAiStyle 新增 AI 修圖風格
//
// @Summary      新增 AI 修圖風格
// @Description  thumbnailStagedToken：透過 media service 上傳後取得的 staged token，後端會自動 finalize 並寫入 thumbnailUrl
// @Tags         AI 修圖 (AI Image)
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        request  body  CreateAiStyleRequest  true  "風格資料"
// @Success      200  {object}  object{code=int}
// @Failure      400  {object}  object{code=int,message=string}
// @Failure      401  {object}  object{code=int,message=string}
// @Router       /admin/ai_styles [post]
func (h *Holder) ProxyCreateAiStyle(c *gin.Context) {
	var req CreateAiStyleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	pbReq := &pb.CreateAiStyleRequest{
		Name:      req.Name,
		Label:     req.Label,
		Prompt:    req.Prompt,
		SortOrder: req.SortOrder,
		Category:  req.Category,
		IsDefault: req.IsDefault,
		Enabled:   req.Enabled,
	}
	if req.ThumbnailStagedToken != "" {
		pbReq.ThumbnailStagedToken = &req.ThumbnailStagedToken
	}

	resp, err := h.AI.CreateAiStyle(c.Request.Context(), pbReq)
	if err != nil {
		response.GrpcErr(c, "ai", err)
		return
	}

	response.GrpcResult(c, resp.Code, resp.Message, nil)
}

// ProxyUpdateAiStyle 修改 AI 修圖風格
//
// @Summary      修改 AI 修圖風格
// @Description  thumbnailStagedToken：透過 media service 上傳後取得的 staged token，後端會自動 finalize 並覆寫 thumbnailUrl；不傳則保留原有縮圖
// @Tags         AI 修圖 (AI Image)
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id       path  string                true  "風格 ID"
// @Param        request  body  UpdateAiStyleRequest  true  "修改內容"
// @Success      200  {object}  object{code=int}
// @Failure      400  {object}  object{code=int,message=string}
// @Failure      401  {object}  object{code=int,message=string}
// @Router       /admin/ai_styles/{id} [put]
func (h *Holder) ProxyUpdateAiStyle(c *gin.Context) {
	var req UpdateAiStyleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	pbReq := &pb.UpdateAiStyleRequest{Id: c.Param("id")}
	if req.Label != nil {
		pbReq.Label = *req.Label
		pbReq.HasLabel = true
	}
	if req.Prompt != nil {
		pbReq.Prompt = *req.Prompt
		pbReq.HasPrompt = true
	}
	if req.ThumbnailStagedToken != nil && *req.ThumbnailStagedToken != "" {
		pbReq.ThumbnailStagedToken = req.ThumbnailStagedToken
	}
	if req.SortOrder != nil {
		pbReq.SortOrder = *req.SortOrder
		pbReq.HasSortOrder = true
	}
	if req.Enabled != nil {
		pbReq.Enabled = *req.Enabled
		pbReq.HasEnabled = true
	}

	resp, err := h.AI.UpdateAiStyle(c.Request.Context(), pbReq)
	if err != nil {
		response.GrpcErr(c, "ai", err)
		return
	}

	response.GrpcResult(c, resp.Code, resp.Message, nil)
}

// ProxyDeleteAiStyle 刪除 AI 修圖風格
//
// @Summary      刪除 AI 修圖風格
// @Description  is_default=true 的風格不可刪除。
// @Tags         AI 修圖 (AI Image)
// @Produce      json
// @Security     BearerAuth
// @Param        id  path  string  true  "風格 ID"
// @Success      200  {object}  object{code=int}
// @Failure      400  {object}  object{code=int,message=string}
// @Failure      401  {object}  object{code=int,message=string}
// @Router       /admin/ai_styles/{id} [delete]
func (h *Holder) ProxyDeleteAiStyle(c *gin.Context) {
	resp, err := h.AI.DeleteAiStyle(c.Request.Context(), c.Param("id"))
	if err != nil {
		response.GrpcErr(c, "ai", err)
		return
	}

	response.GrpcResult(c, resp.Code, resp.Message, nil)
}

// ProxyGetAiQuota 查詢賣家 AI 修圖額度
//
// @Summary      查詢賣家 AI 修圖額度
// @Description  回傳賣家的每週額度、本週使用量、是否為自訂額度。
// @Tags         AI 修圖 (AI Image)
// @Produce      json
// @Security     BearerAuth
// @Param        store_id  path  string  true  "賣家 store_id"
// @Success      200  {object}  object{code=int,data=AiQuotaResponse}
// @Failure      401  {object}  object{code=int,message=string}
// @Router       /admin/ai_quotas/{store_id} [get]
func (h *Holder) ProxyGetAiQuota(c *gin.Context) {
	resp, err := h.AI.GetAiQuota(c.Request.Context(), c.Param("store_id"))
	if err != nil {
		response.GrpcErr(c, "ai", err)
		return
	}

	if resp.Code != 0 {
		response.GrpcResult(c, resp.Code, resp.Message, nil)
		return
	}

	response.OK(c, AiQuotaResponse{
		StoreID:     resp.Data.GetStoreId(),
		WeeklyLimit: resp.Data.GetWeeklyLimit(),
		WeeklyUsage: resp.Data.GetWeeklyUsage(),
		Memo:        resp.Data.GetMemo(),
		UpdatedAt:   resp.Data.GetUpdatedAt(),
		IsCustom:    resp.Data.GetIsCustom(),
	})
}

// ProxyUpdateAiQuota 設定賣家 AI 修圖額度
//
// @Summary      設定賣家 AI 修圖額度
// @Description  為指定賣家設定自訂每週額度（0=禁用，最高 1000）。
// @Tags         AI 修圖 (AI Image)
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        store_id  path  string                true  "賣家 store_id"
// @Param        request   body  UpdateAiQuotaRequest  true  "額度設定"
// @Success      200  {object}  object{code=int}
// @Failure      400  {object}  object{code=int,message=string}
// @Failure      401  {object}  object{code=int,message=string}
// @Router       /admin/ai_quotas/{store_id} [put]
func (h *Holder) ProxyUpdateAiQuota(c *gin.Context) {
	var req UpdateAiQuotaRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	resp, err := h.AI.UpdateAiQuota(c.Request.Context(), &pb.UpdateAiQuotaRequest{
		StoreId:     c.Param("store_id"),
		WeeklyLimit: req.WeeklyLimit,
		Memo:        req.Memo,
	})
	if err != nil {
		response.GrpcErr(c, "ai", err)
		return
	}

	response.GrpcResult(c, resp.Code, resp.Message, nil)
}

// ProxyDeleteAiQuota 刪除賣家自訂額度（恢復預設）
//
// @Summary      刪除賣家自訂額度
// @Description  移除賣家的自訂額度設定，恢復為系統預設（10 次/週）。
// @Tags         AI 修圖 (AI Image)
// @Produce      json
// @Security     BearerAuth
// @Param        store_id  path  string  true  "賣家 store_id"
// @Success      200  {object}  object{code=int}
// @Failure      401  {object}  object{code=int,message=string}
// @Router       /admin/ai_quotas/{store_id} [delete]
func (h *Holder) ProxyDeleteAiQuota(c *gin.Context) {
	resp, err := h.AI.DeleteAiQuota(c.Request.Context(), c.Param("store_id"))
	if err != nil {
		response.GrpcErr(c, "ai", err)
		return
	}

	response.GrpcResult(c, resp.Code, resp.Message, nil)
}

type SellerAssistantSSERequest struct {
	SessionID string `json:"sessionId,omitempty"`
	Message   string `json:"message"`
	ImageURL  string `json:"imageUrl,omitempty"`
}

// ProxyAIAssistantSSE proxies the seller SSE chat request to ai-service.
//
// @Summary      賣家 AI 小幫手 — 自由聊天（SSE）
// @Description  以 SSE 串流形式與 AI 小幫手自由對話，LLM 自行判斷意圖並選擇工具。
// @Tags         AI
// @Accept       json
// @Produce      text/event-stream
// @Security     BearerAuth
// @Param        request  body  SellerAssistantSSERequest  true  "聊天請求"
// @Success      200  {string}  string  "SSE stream"
// @Failure      401  {object}  object{code=int,message=string}
// @Failure      403  {object}  object{code=int,message=string}
// @Failure      502  {object}  object{code=int,message=string}
// @Router       /app/sellers/me/assistant/chat [post]
func (h *Holder) ProxyAIAssistantSSE(c *gin.Context) {
	var req assistantStreamRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	storeID, ok := h.resolveStoreID(c)
	if !ok {
		return
	}
	pbReq := req.toProto()
	proxy.ServeAIAssistantStream(c, req.SessionID, func(ctx context.Context) (grpcclient.AssistantStream, error) {
		return h.AIAssistant.SellerChat(grpcclient.WithStoreID(ctx, storeID), pbReq)
	})
}

// ProxySellerOrdersSSE proxies the seller orders query to ai-service.
//
// @Summary      賣家 AI 小幫手 — 訂單查詢（SSE）
// @Description  確定性呼叫訂單查詢工具，回傳訂單清單。
// @Tags         AI
// @Accept       json
// @Produce      text/event-stream
// @Security     BearerAuth
// @Param        request  body  SellerAssistantSSERequest  true  "查詢請求"
// @Success      200  {string}  string  "SSE stream"
// @Failure      401  {object}  object{code=int,message=string}
// @Failure      403  {object}  object{code=int,message=string}
// @Failure      502  {object}  object{code=int,message=string}
// @Router       /app/sellers/me/assistant/orders [post]
func (h *Holder) ProxySellerOrdersSSE(c *gin.Context) {
	var req assistantStreamRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	storeID, ok := h.resolveStoreID(c)
	if !ok {
		return
	}
	pbReq := req.toProto()
	proxy.ServeAIAssistantStream(c, req.SessionID, func(ctx context.Context) (grpcclient.AssistantStream, error) {
		return h.AIAssistant.SellerOrders(grpcclient.WithStoreID(ctx, storeID), pbReq)
	})
}

// ProxySellerInventorySSE proxies the seller inventory check to ai-service.
//
// @Summary      賣家 AI 小幫手 — 庫存檢查（SSE）
// @Description  確定性呼叫庫存檢查工具，比對庫存 vs 未出貨需求。
// @Tags         AI
// @Accept       json
// @Produce      text/event-stream
// @Security     BearerAuth
// @Param        request  body  SellerAssistantSSERequest  true  "查詢請求"
// @Success      200  {string}  string  "SSE stream"
// @Failure      401  {object}  object{code=int,message=string}
// @Failure      403  {object}  object{code=int,message=string}
// @Failure      502  {object}  object{code=int,message=string}
// @Router       /app/sellers/me/assistant/inventory [post]
func (h *Holder) ProxySellerInventorySSE(c *gin.Context) {
	var req assistantStreamRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	storeID, ok := h.resolveStoreID(c)
	if !ok {
		return
	}
	pbReq := req.toProto()
	proxy.ServeAIAssistantStream(c, req.SessionID, func(ctx context.Context) (grpcclient.AssistantStream, error) {
		return h.AIAssistant.SellerInventory(grpcclient.WithStoreID(ctx, storeID), pbReq)
	})
}

// ProxySellerPaymentSSE proxies the seller payment summary to ai-service.
//
// @Summary      賣家 AI 小幫手 — 收款對帳（SSE）
// @Description  確定性呼叫收款對帳工具，查詢已收款/待收款金額。
// @Tags         AI
// @Accept       json
// @Produce      text/event-stream
// @Security     BearerAuth
// @Param        request  body  SellerAssistantSSERequest  true  "查詢請求"
// @Success      200  {string}  string  "SSE stream"
// @Failure      401  {object}  object{code=int,message=string}
// @Failure      403  {object}  object{code=int,message=string}
// @Failure      502  {object}  object{code=int,message=string}
// @Router       /app/sellers/me/assistant/payment [post]
func (h *Holder) ProxySellerPaymentSSE(c *gin.Context) {
	var req assistantStreamRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	storeID, ok := h.resolveStoreID(c)
	if !ok {
		return
	}
	pbReq := req.toProto()
	proxy.ServeAIAssistantStream(c, req.SessionID, func(ctx context.Context) (grpcclient.AssistantStream, error) {
		return h.AIAssistant.SellerPayment(grpcclient.WithStoreID(ctx, storeID), pbReq)
	})
}

// ProxySellerPriceSSE proxies the seller seasonal price query to ai-service.
//
// @Summary      賣家 AI 小幫手 — 農產品行情（SSE）
// @Description  確定性呼叫農產品行情工具，查詢市場行情與定價建議。
// @Tags         AI
// @Accept       json
// @Produce      text/event-stream
// @Security     BearerAuth
// @Param        request  body  SellerAssistantSSERequest  true  "查詢請求"
// @Success      200  {string}  string  "SSE stream"
// @Failure      401  {object}  object{code=int,message=string}
// @Failure      403  {object}  object{code=int,message=string}
// @Failure      502  {object}  object{code=int,message=string}
// @Router       /app/sellers/me/assistant/price [post]
func (h *Holder) ProxySellerPriceSSE(c *gin.Context) {
	var req assistantStreamRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	storeID, ok := h.resolveStoreID(c)
	if !ok {
		return
	}
	pbReq := req.toProto()
	proxy.ServeAIAssistantStream(c, req.SessionID, func(ctx context.Context) (grpcclient.AssistantStream, error) {
		return h.AIAssistant.SellerPrice(grpcclient.WithStoreID(ctx, storeID), pbReq)
	})
}

// ProxyBuyerAssistantSSE proxies the buyer SSE chat request to ai-service.
//
// @Summary      買家 AI 小幫手 — 自由聊天（SSE）
// @Description  以 SSE 串流形式與 AI 小幫手自由對話。
// @Tags         AI
// @Accept       json
// @Produce      text/event-stream
// @Security     BearerAuth
// @Param        request  body  SellerAssistantSSERequest  true  "聊天請求"
// @Success      200  {string}  string  "SSE stream"
// @Failure      401  {object}  object{code=int,message=string}
// @Failure      403  {object}  object{code=int,message=string}
// @Failure      502  {object}  object{code=int,message=string}
// @Router       /app/buyers/me/assistant/chat [post]
func (h *Holder) ProxyBuyerAssistantSSE(c *gin.Context) {
	var req assistantStreamRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	buyerID, ok := h.resolveBuyerID(c)
	if !ok {
		return
	}
	pbReq := req.toProto()
	proxy.ServeAIAssistantStream(c, req.SessionID, func(ctx context.Context) (grpcclient.AssistantStream, error) {
		return h.AIAssistant.BuyerChat(grpcclient.WithBuyerID(ctx, buyerID), pbReq)
	})
}

// ProxyBuyerOrdersSSE proxies the buyer orders query to ai-service.
//
// @Summary      買家 AI 小幫手 — 訂單查詢（SSE）
// @Tags         AI
// @Accept       json
// @Produce      text/event-stream
// @Security     BearerAuth
// @Param        request  body  SellerAssistantSSERequest  true  "查詢請求"
// @Success      200  {string}  string  "SSE stream"
// @Failure      401  {object}  object{code=int,message=string}
// @Failure      502  {object}  object{code=int,message=string}
// @Router       /app/buyers/me/assistant/orders [post]
func (h *Holder) ProxyBuyerOrdersSSE(c *gin.Context) {
	var req assistantStreamRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	buyerID, ok := h.resolveBuyerID(c)
	if !ok {
		return
	}
	pbReq := req.toProto()
	proxy.ServeAIAssistantStream(c, req.SessionID, func(ctx context.Context) (grpcclient.AssistantStream, error) {
		return h.AIAssistant.BuyerOrders(grpcclient.WithBuyerID(ctx, buyerID), pbReq)
	})
}

// ProxyBuyerTrackingSSE proxies the buyer package tracking to ai-service.
//
// @Summary      買家 AI 小幫手 — 包裹追蹤（SSE）
// @Tags         AI
// @Accept       json
// @Produce      text/event-stream
// @Security     BearerAuth
// @Param        request  body  SellerAssistantSSERequest  true  "查詢請求"
// @Success      200  {string}  string  "SSE stream"
// @Failure      401  {object}  object{code=int,message=string}
// @Failure      502  {object}  object{code=int,message=string}
// @Router       /app/buyers/me/assistant/tracking [post]
func (h *Holder) ProxyBuyerTrackingSSE(c *gin.Context) {
	var req assistantStreamRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	buyerID, ok := h.resolveBuyerID(c)
	if !ok {
		return
	}
	pbReq := req.toProto()
	proxy.ServeAIAssistantStream(c, req.SessionID, func(ctx context.Context) (grpcclient.AssistantStream, error) {
		return h.AIAssistant.BuyerTracking(grpcclient.WithBuyerID(ctx, buyerID), pbReq)
	})
}

// ProxyBuyerReturnPolicySSE proxies the return-policy reply to ai-service, which
// streams its fixed text (agent.ReturnPolicyFacts) without calling an LLM.
//
// @Summary      買家 AI 小幫手 — 退貨流程說明（SSE）
// @Description  由 ai-service 以 SSE 回傳固定的退貨流程文案，不走 LLM。
// @Tags         AI
// @Accept       json
// @Produce      text/event-stream
// @Security     BearerAuth
// @Success      200  {string}  string  "SSE stream"
// @Failure      401  {object}  object{code=int,message=string}
// @Failure      502  {object}  object{code=int,message=string}
// @Router       /app/buyers/me/assistant/return-policy [post]
func (h *Holder) ProxyBuyerReturnPolicySSE(c *gin.Context) {
	var req assistantStreamRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	buyerID, ok := h.resolveBuyerID(c)
	if !ok {
		return
	}
	pbReq := req.toProto()
	proxy.ServeAIAssistantStream(c, req.SessionID, func(ctx context.Context) (grpcclient.AssistantStream, error) {
		return h.AIAssistant.BuyerReturnPolicy(grpcclient.WithBuyerID(ctx, buyerID), pbReq)
	})
}

// ProxyBuyerSeasonalSSE proxies the seasonal fruit recommendation to ai-service.
//
// @Summary      買家 AI 小幫手 — 當季水果推薦（SSE）
// @Tags         AI
// @Accept       json
// @Produce      text/event-stream
// @Security     BearerAuth
// @Param        request  body  SellerAssistantSSERequest  true  "查詢請求"
// @Success      200  {string}  string  "SSE stream"
// @Failure      401  {object}  object{code=int,message=string}
// @Failure      502  {object}  object{code=int,message=string}
// @Router       /app/buyers/me/assistant/seasonal [post]
func (h *Holder) ProxyBuyerSeasonalSSE(c *gin.Context) {
	var req assistantStreamRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	buyerID, ok := h.resolveBuyerID(c)
	if !ok {
		return
	}
	pbReq := req.toProto()
	proxy.ServeAIAssistantStream(c, req.SessionID, func(ctx context.Context) (grpcclient.AssistantStream, error) {
		return h.AIAssistant.BuyerSeasonal(grpcclient.WithBuyerID(ctx, buyerID), pbReq)
	})
}

// ProxyBuyerSellerMatchSSE proxies the AI seller-matching (candidate lead search) to ai-service.
//
// @Summary      買家 AI 小幫手 — 找供應商（SSE）
// @Tags         AI
// @Accept       json
// @Produce      text/event-stream
// @Security     BearerAuth
// @Param        request  body  SellerAssistantSSERequest  true  "查詢請求"
// @Success      200  {string}  string  "SSE stream"
// @Failure      401  {object}  object{code=int,message=string}
// @Failure      502  {object}  object{code=int,message=string}
// @Router       /app/buyers/me/assistant/seller-match [post]
func (h *Holder) ProxyBuyerSellerMatchSSE(c *gin.Context) {
	var req assistantStreamRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	buyerID, ok := h.resolveBuyerID(c)
	if !ok {
		return
	}
	pbReq := req.toProto()
	proxy.ServeAIAssistantStream(c, req.SessionID, func(ctx context.Context) (grpcclient.AssistantStream, error) {
		return h.AIAssistant.BuyerSellerMatch(grpcclient.WithBuyerID(ctx, buyerID), pbReq)
	})
}

type LatestSessionResponse struct {
	SessionID string        `json:"sessionId"`
	Messages  []MessageItem `json:"messages"`
}

type MessageItem struct {
	Role      string `json:"role"`
	Text      string `json:"text"`
	Timestamp string `json:"timestamp"`
}

// ProxySellerLatestSession proxies the seller latest session to ai-service.
//
// @Summary      賣家 AI 小幫手 — 取得最新對話歷史
// @Description  取得賣家最新的對話 session 及其完整歷史訊息（24 小時內）。無對話時回傳空。
// @Tags         AI
// @Produce      json
// @Security     BearerAuth
// @Success      200  {object}  object{code=int,data=LatestSessionResponse}
// @Failure      401  {object}  object{code=int,message=string}
// @Router       /app/sellers/me/assistant/sessions/latest [get]
func (h *Holder) ProxySellerLatestSession(c *gin.Context) {
	storeID, ok := h.resolveStoreID(c)
	if !ok {
		return
	}
	ctx := grpcclient.WithStoreID(c.Request.Context(), storeID)
	resp, err := h.AIAssistant.SellerLatestSession(ctx, &pb.LatestSessionRequest{})
	if err != nil {
		response.GrpcErr(c, "ai", err)
		return
	}
	response.OK(c, toLatestSessionResponseDTO(resp))
}

// ProxyBuyerLatestSession proxies the buyer latest session to ai-service.
//
// @Summary      買家 AI 小幫手 — 取得最新對話歷史
// @Description  取得買家最新的對話 session 及其完整歷史訊息（24 小時內）。無對話時回傳空。
// @Tags         AI
// @Produce      json
// @Security     BearerAuth
// @Success      200  {object}  object{code=int,data=LatestSessionResponse}
// @Failure      401  {object}  object{code=int,message=string}
// @Router       /app/buyers/me/assistant/sessions/latest [get]
func (h *Holder) ProxyBuyerLatestSession(c *gin.Context) {
	buyerID, ok := h.resolveBuyerID(c)
	if !ok {
		return
	}
	ctx := grpcclient.WithBuyerID(c.Request.Context(), buyerID)
	resp, err := h.AIAssistant.BuyerLatestSession(ctx, &pb.LatestSessionRequest{})
	if err != nil {
		response.GrpcErr(c, "ai", err)
		return
	}
	response.OK(c, toLatestSessionResponseDTO(resp))
}

// ProxyProcurementTurn proxies a procurement turn to ai-service.
//
// @Summary      Hawkr 幫找 — 提交需求描述
// @Description  自然語言需求輸入；回傳最新需求快照與仍缺少的欄位。degraded=true 表示解析暫時降級。
// @Tags         AI
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        request  body  ProcurementTurnRequest  true  "採購需求訊息"
// @Success      200  {object}  object{code=int,message=string,data=procurement.TurnResultDTO}
// @Failure      400  {object}  object{code=int,reason=string,message=string,traceId=string}
// @Failure      401  {object}  object{code=int,message=string,traceId=string}
// @Failure      422  {object}  object{code=int,reason=string,message=string,traceId=string}
// @Failure      429  {object}  object{code=int,reason=string,message=string,remaining=string,resetAt=string,traceId=string}  "reason=AGENT_MONTHLY_QUOTA_EXCEEDED 時額外帶 remaining（固定 \"0\"）與 resetAt（ISO 8601 UTC 重置時刻），doc 57 §4.7"
// @Router       /app/buyers/me/procurement/turns [post]
func (h *Holder) ProxyProcurementTurn(c *gin.Context) {
	buyerID, ok := h.resolveBuyerID(c)
	if !ok {
		return
	}

	var req ProcurementTurnRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		badProcurementRequest(c, "invalid request body")
		return
	}
	if req.ClientRequestID == "" {
		badProcurementRequest(c, "clientRequestId is required")
		return
	}
	if req.Message == "" {
		badProcurementRequest(c, "message is required")
		return
	}
	if len([]rune(req.Message)) > procurementMaxMessageRunes {
		writeProcurementErrorBody(c, http.StatusUnprocessableEntity, "INPUT_TOO_LONG", "單則訊息上限 2000 字")
		return
	}

	ctx := grpcclient.WithBuyerID(c.Request.Context(), buyerID)
	result, err := h.AIProcurement.SubmitTurn(ctx, &pb.TurnRequest{
		RequestId: req.RequestID, Message: req.Message, ClientRequestId: req.ClientRequestID,
	})
	if err != nil {
		writeProcurementError(c, err)
		return
	}
	response.OK(c, turnResultFromProto(result))
}

const procurementMaxMessageRunes = 2000

// ProxyProcurementTurnStream proxies the streaming procurement turn to ai-service.
//
// @Summary      Hawkr 幫找 — 提交需求描述（SSE 串流版）
// @Description  跟 /turns 同一個輸入契約，差別是以 AG-UI 事件串流邊執行邊推送進度（RUN_STARTED、STEP_STARTED/STEP_FINISHED、CUSTOM、TEXT_MESSAGE_START/CONTENT/END、STATE_SNAPSHOT；正常終止 RUN_FINISHED，失敗終止 RUN_ERROR）。連線建立後的失敗只會以 RUN_ERROR frame 呈現（HTTP 仍是 200）；該 frame 除了 code/message 外，帶 BusinessError 的失敗另有 reason，其中 reason=AGENT_MONTHLY_QUOTA_EXCEEDED 再加上 remaining 與 resetAt（doc 57 §4.7）。
// @Tags         AI
// @Accept       json
// @Produce      text/event-stream
// @Security     BearerAuth
// @Param        request  body  ProcurementTurnRequest  true  "採購需求訊息"
// @Success      200  {string}  string  "text/event-stream"
// @Failure      400  {object}  object{code=int,reason=string,message=string,traceId=string}
// @Failure      401  {object}  object{code=int,message=string,traceId=string}
// @Failure      422  {object}  object{code=int,reason=string,message=string,traceId=string}
// @Failure      429  {object}  object{code=int,reason=string,message=string,traceId=string}
// @Router       /app/buyers/me/procurement/turns/stream [post]
func (h *Holder) ProxyProcurementTurnStream(c *gin.Context) {
	buyerID, ok := h.resolveBuyerID(c)
	if !ok {
		return
	}

	var req ProcurementTurnRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		badProcurementRequest(c, "invalid request body")
		return
	}
	if req.ClientRequestID == "" {
		badProcurementRequest(c, "clientRequestId is required")
		return
	}
	if req.Message == "" {
		badProcurementRequest(c, "message is required")
		return
	}
	if len([]rune(req.Message)) > procurementMaxMessageRunes {
		writeProcurementErrorBody(c, http.StatusUnprocessableEntity, "INPUT_TOO_LONG", "單則訊息上限 2000 字")
		return
	}

	release, ok := acquireProcurementStreamSlot(int64(buyerID))
	if !ok {
		writeProcurementErrorBody(c, http.StatusTooManyRequests, "", "too many concurrent procurement turns, please wait and retry")
		return
	}
	defer release()

	proxy.ServeAIProcurementStream(c, req.RequestID, func(ctx context.Context) (grpcclient.AssistantStream, error) {
		return h.AIProcurement.SubmitTurnStream(grpcclient.WithBuyerID(ctx, buyerID), &pb.TurnRequest{
			RequestId: req.RequestID, Message: req.Message, ClientRequestId: req.ClientRequestID,
		})
	})
}

// ProxyProcurementSnapshot proxies a procurement snapshot read to ai-service.
//
// @Summary      Hawkr 幫找 — 取得需求快照
// @Tags         AI
// @Produce      json
// @Security     BearerAuth
// @Param        id  path  string  true  "需求 ID（TSID 字串）"
// @Success      200  {object}  object{code=int,message=string,data=ProcurementSnapshotResponseDTO}
// @Failure      404  {object}  object{code=int,reason=string,message=string,traceId=string}
// @Router       /app/buyers/me/procurement/requests/{id}/snapshot [get]
func (h *Holder) ProxyProcurementSnapshot(c *gin.Context) {
	buyerID, ok := h.resolveBuyerID(c)
	if !ok {
		return
	}
	ctx := grpcclient.WithBuyerID(c.Request.Context(), buyerID)
	result, err := h.AIProcurement.GetSnapshot(ctx, &pb.GetSnapshotRequest{RequestId: c.Param("id")})
	if err != nil {
		writeProcurementError(c, err)
		return
	}
	response.OK(c, snapshotResponseFromProto(result))
}

// ProxyProcurementCandidateSearch proxies a candidate-search run creation to ai-service.
//
// @Summary      Hawkr 幫找 — 建立候選搜尋
// @Description  需求就緒（REQUIREMENT_READY）後建立雙 lane 候選搜尋 run；同一需求版本重複呼叫回傳既有 run（冪等）。
// @Tags         AI
// @Produce      json
// @Security     BearerAuth
// @Param        id  path  string  true  "需求 ID（TSID 字串）"
// @Success      200  {object}  object{code=int,message=string,data=ProcurementStartCandidateSearchResponseDTO}
// @Failure      409  {object}  object{code=int,reason=string,message=string,traceId=string}
// @Router       /app/buyers/me/procurement/requests/{id}/candidate-searches [post]
func (h *Holder) ProxyProcurementCandidateSearch(c *gin.Context) {
	buyerID, ok := h.resolveBuyerID(c)
	if !ok {
		return
	}
	ctx := grpcclient.WithBuyerID(c.Request.Context(), buyerID)
	result, err := h.AIProcurement.StartCandidateSearch(ctx, &pb.StartCandidateSearchRequest{RequestId: c.Param("id")})
	if err != nil {
		writeProcurementError(c, err)
		return
	}
	response.OK(c, startCandidateSearchResponseFromProto(result))
}

// ProxyProcurementCandidateSearchGet proxies a candidate-search run read to ai-service.
//
// @Summary      Hawkr 幫找 — 取得候選搜尋結果
// @Tags         AI
// @Produce      json
// @Security     BearerAuth
// @Param        id     path  string  true  "需求 ID（TSID 字串）"
// @Param        runId  path  string  true  "候選搜尋 run ID（TSID 字串）；#2446 expand 波起亦接受 candidate set ID，兩者都解析得到"
// @Success      200  {object}  object{code=int,message=string,data=ProcurementCandidateSetResponseDTO}
// @Failure      404  {object}  object{code=int,reason=string,message=string,traceId=string}
// @Router       /app/buyers/me/procurement/requests/{id}/candidate-searches/{runId} [get]
func (h *Holder) ProxyProcurementCandidateSearchGet(c *gin.Context) {
	buyerID, ok := h.resolveBuyerID(c)
	if !ok {
		return
	}
	ctx := grpcclient.WithBuyerID(c.Request.Context(), buyerID)
	result, err := h.AIProcurement.GetCandidateSearch(ctx, &pb.GetCandidateSearchRequest{RequestId: c.Param("id"), RunId: c.Param("runId")})
	if err != nil {
		writeProcurementError(c, err)
		return
	}
	response.OK(c, candidateSetResponseFromProto(result, h.MediaBaseURL))
}

// ProxyProcurementSelection proxies a candidate selection toggle to ai-service.
//
// @Summary      Hawkr 幫找 — 勾選/取消勾選候選
// @Tags         AI
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id  path  string  true  "需求 ID（TSID 字串）"
// @Param        request  body  ProcurementSelectionRequest  true  "候選勾選"
// @Success      200  {object}  object{code=int,message=string,data=ProcurementSelectionResponseDTO}
// @Failure      400  {object}  object{code=int,reason=string,message=string,traceId=string}
// @Failure      409  {object}  object{code=int,reason=string,message=string,traceId=string}
// @Router       /app/buyers/me/procurement/requests/{id}/selections [post]
func (h *Holder) ProxyProcurementSelection(c *gin.Context) {
	buyerID, ok := h.resolveBuyerID(c)
	if !ok {
		return
	}
	var req ProcurementSelectionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		badProcurementRequest(c, "invalid request body")
		return
	}
	ctx := grpcclient.WithBuyerID(c.Request.Context(), buyerID)
	result, err := h.AIProcurement.ToggleSelection(ctx, &pb.ToggleSelectionRequest{
		RequestId: c.Param("id"), CandidateSetId: req.CandidateSetID, Lane: req.Lane, CandidateId: req.CandidateID, Selected: req.Selected,
	})
	if err != nil {
		writeProcurementError(c, err)
		return
	}
	response.OK(c, selectionResponseFromProto(result))
}

// ProxyProcurementAnswer proxies a clarify-question answer to ai-service.
//
// @Summary      Hawkr 幫找 — 引導式釐清答題
// @Tags         AI
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id  path  string  true  "需求 ID（TSID 字串）"
// @Param        request  body  ProcurementAnswerRequest  true  "釐清問題的回答"
// @Success      200  {object}  object{code=int,message=string,data=procurement.TurnResultDTO}
// @Failure      400  {object}  object{code=int,reason=string,message=string,traceId=string}
// @Failure      409  {object}  object{code=int,reason=string,message=string,traceId=string}
// @Failure      422  {object}  object{code=int,reason=string,message=string,traceId=string}
// @Router       /app/buyers/me/procurement/requests/{id}/answers [post]
func (h *Holder) ProxyProcurementAnswer(c *gin.Context) {
	buyerID, ok := h.resolveBuyerID(c)
	if !ok {
		return
	}
	var req ProcurementAnswerRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		badProcurementRequest(c, "invalid request body")
		return
	}
	if req.ClientRequestID == "" {
		badProcurementRequest(c, "clientRequestId is required")
		return
	}
	if req.SlotKey == "" {
		badProcurementRequest(c, "slotKey is required")
		return
	}
	isQuantityAnswer := req.SlotKey == "quantity" && !req.Skip
	isCropDisambiguationAnswer := req.SlotKey == "crop_disambiguation" && !req.Skip
	switch {
	case isQuantityAnswer:
		if req.OptionID != "" {
			badProcurementRequest(c, "quantity 不接受 optionId")
			return
		}
		if req.FreeText == "" && req.UnitOptionID == "" {
			badProcurementRequest(c, "freeText 或 unitOptionId 必須至少擇一")
			return
		}
	case isCropDisambiguationAnswer:
		if req.UnitOptionID != "" {
			badProcurementRequest(c, "unitOptionId 只能在回答 quantity 時使用")
			return
		}
		if req.OptionID == "" {
			badProcurementRequest(c, "crop_disambiguation 必須提供 optionId")
			return
		}
		if req.OptionID == "crop-other" {
			if req.FreeText == "" {
				badProcurementRequest(c, "crop_disambiguation 選擇 crop-other 時 freeText 為必填")
				return
			}
		} else if req.FreeText != "" {
			badProcurementRequest(c, "crop_disambiguation 選擇非 crop-other 選項時不可帶 freeText")
			return
		}
	default:
		if req.UnitOptionID != "" {
			badProcurementRequest(c, "unitOptionId 只能在回答 quantity 時使用")
			return
		}
		answerFieldsSet := 0
		if req.OptionID != "" {
			answerFieldsSet++
		}
		if req.FreeText != "" {
			answerFieldsSet++
		}
		if req.Skip {
			answerFieldsSet++
		}
		if answerFieldsSet == 0 {
			badProcurementRequest(c, "optionId, freeText 或 skip 必須擇一")
			return
		}
		if answerFieldsSet > 1 {
			badProcurementRequest(c, "optionId, freeText 和 skip 最多只能擇一")
			return
		}
	}

	ctx := grpcclient.WithBuyerID(c.Request.Context(), buyerID)
	result, err := h.AIProcurement.SubmitAnswer(ctx, &pb.SubmitAnswerRequest{
		RequestId: c.Param("id"), SlotKey: req.SlotKey, OptionId: req.OptionID, FreeText: req.FreeText,
		UnitOptionId: req.UnitOptionID, Skip: req.Skip, ExpectedRevisionId: req.ExpectedRevisionID,
		ClientRequestId: req.ClientRequestID, SelfEnteredPriceAmount: req.SelfEnteredPriceAmount,
	})
	if err != nil {
		writeProcurementError(c, err)
		return
	}
	response.OK(c, turnResultFromProto(result))
}

// ProxyProcurementRewind proxies a "rewind to slot" request to ai-service.
//
// @Summary      Hawkr 幫找 — 回到某欄位重新回答
// @Description  伺服器依 slotKey 從版本鏈解析要回溯到的版本，不需要 client 帶版本 ID；沒有 expectedRevisionId，不會回 409。
// @Tags         AI
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id  path  string  true  "需求 ID（TSID 字串）"
// @Param        request  body  ProcurementRewindRequest  true  "要重問的欄位"
// @Success      200  {object}  object{code=int,message=string,data=procurement.TurnResultDTO}
// @Failure      400  {object}  object{code=int,reason=string,message=string,traceId=string}
// @Failure      404  {object}  object{code=int,reason=string,message=string,traceId=string}
// @Failure      422  {object}  object{code=int,reason=string,message=string,traceId=string}
// @Router       /app/buyers/me/procurement/requests/{id}/rewinds [post]
func (h *Holder) ProxyProcurementRewind(c *gin.Context) {
	buyerID, ok := h.resolveBuyerID(c)
	if !ok {
		return
	}
	var req ProcurementRewindRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		badProcurementRequest(c, "invalid request body")
		return
	}
	if req.ClientRequestID == "" {
		badProcurementRequest(c, "clientRequestId is required")
		return
	}
	if req.SlotKey == "" {
		badProcurementRequest(c, "slotKey is required")
		return
	}

	ctx := grpcclient.WithBuyerID(c.Request.Context(), buyerID)
	result, err := h.AIProcurement.Rewind(ctx, &pb.RewindRequest{
		RequestId: c.Param("id"), SlotKey: req.SlotKey, ClientRequestId: req.ClientRequestID,
	})
	if err != nil {
		writeProcurementError(c, err)
		return
	}
	response.OK(c, turnResultFromProto(result))
}

// ProxyProcurementRfqPricePreview proxies a "preview RFQ price" request to
// ai-service (issue #2403 Slice 2).
//
// @Summary      Hawkr 幫找 — RFQ 報價預覽（Pricebook 參考區間）
// @Description  算出這次 RFQ 詢價可用的 Pricebook 參考價區間（`referencePriceRange`，pricingMode=`PRICEBOOK_GUIDED`），
// @Description  或在可靠映射缺失時改走買家自行出價（pricingMode=`BUYER_ENTERED_NO_REFERENCE`，`referencePriceRange` 為 `null`）。
// @Description  不建立 immutable RFQ confirmation、不通知任何供應商、除了寫入這筆 preview 本身沒有其他 side effect。
// @Description
// @Description  第一次呼叫可省略 `buyerRequestedRange`，Server 以參考區間下上界預填；帶了就原樣回顯，
// @Description  Server 不會把買家給的範圍 clamp 到參考區間內。
// @Description
// @Description  Idempotency：同一買家同一 `clientRequestId` 重送回同一筆既有結果（不重算、不重打 Pricebook）。
// @Description
// @Description  `referencePriceRange.basisQuantity`／`basisUnit` 說明三個金額是「每多少單位」的價格
// @Description  （GRAM 品項是每 1000 公克），值與 `buyerRequestedRange` 的同名欄位一致；client 不得自行換算。
// @Description
// @Description  `marketPriceBasis`（計價層級、涵蓋範圍、區域名、貢獻市場家數、資料截至、新鮮度）與
// @Description  `referencePriceRange` 同生同滅，`BUYER_ENTERED_NO_REFERENCE` 時為 `null`。
// @Description  `freshnessStatus` 只是揭露，不影響 `referenceAvailability`。
// @Description  `priceLayer`（`wholesale_auction`｜`farmgate`｜`live_animal`｜`retail`｜`wholesale_bulk`，
// @Description  空字串＝既有蔬果批發路徑）必須與金額一起顯示：毛豬回的是活體拍賣均價、不是豬肉分切零售價，
// @Description  兩者水平差數倍。`scope` 是 `REGION`／`NATIONAL`，用來區分「全國單一行情」與「區域比對失敗」。
// @Description
// @Description  `quoteTotalRange` 是伺服器依本次單價範圍與需求數量算出的詢價總額區間；需求數量是選填 slot，
// @Description  未填時為 `null`，client 應隱藏總額區塊而不是顯示 0。
// @Description
// @Description  422 `PRICE_BASIS_UNAVAILABLE`：currency/quantity/unit/conversion basis 無法 canonicalize 的 typed hard block，
// @Description  零 side effect、不建立 preview 列。
// @Description
// @Description  409 `STALE_REVISION`：`expectedRevisionId` 與目前的 active revision 不符（需求已被其他裝置改動），
// @Description  請重新讀取 snapshot 取得目前的 revisionId 再送。
// @Tags         AI
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id  path  string  true  "需求 ID（TSID 字串）"
// @Param        request  body  ProcurementRfqPricePreviewRequest  true  "RFQ 報價預覽輸入"
// @Success      200  {object}  object{code=int,message=string,data=ProcurementRfqPricePreviewResponseDTO}
// @Failure      400  {object}  object{code=int,reason=string,message=string,traceId=string}
// @Failure      404  {object}  object{code=int,reason=string,message=string,traceId=string}
// @Failure      409  {object}  object{code=int,reason=string,message=string,traceId=string}
// @Failure      422  {object}  object{code=int,reason=string,message=string,traceId=string}
// @Router       /app/buyers/me/procurement/requests/{id}/rfq-price-previews [post]
func (h *Holder) ProxyProcurementRfqPricePreview(c *gin.Context) {
	buyerID, ok := h.resolveBuyerID(c)
	if !ok {
		return
	}
	var req ProcurementRfqPricePreviewRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		badProcurementRequest(c, "invalid request body")
		return
	}

	ctx := grpcclient.WithBuyerID(c.Request.Context(), buyerID)
	result, err := h.AIProcurement.PreviewRfqPrice(ctx, req.toProto(c.Param("id")))
	if err != nil {
		writeProcurementError(c, err)
		return
	}
	response.OK(c, rfqPricePreviewResponseFromProto(result))
}

// ProxyProcurementRfqConfirmation proxies a "confirm RFQ snapshot" request to
// ai-service (issue #2403 Slice 3).
//
// @Summary      Hawkr 幫找 — RFQ 報價確認（產生不可變確認快照）
// @Description  買家對某次 `/rfq-price-previews` 結果按下確認：以 `pricePreviewId`／`pricePreviewVersion`／`payloadHash`
// @Description  做 CAS 檢查，通過後建立一筆不可變的 `confirmedRfqSnapshotId`。這一步只固定詢價規格本身，不通知任何供應商、
// @Description  不建立報價單邀請。
// @Description
// @Description  Idempotency：同一買家同一 `clientRequestId` 重送回同一筆既有結果（不重算、不重建）。
// @Description
// @Description  409 `STALE_REVISION`：`expectedRevisionId` 與目前的 active revision 不符，或 `pricePreviewVersion`／
// @Description  `payloadHash` 與目前的 price preview 不符（CAS 失敗）——兩者共用同一個 reason code，讀 `message` 分辨：
// @Description  前者請重新讀取 snapshot 取得目前的 revisionId 再送，後者請重新呼叫 `/rfq-price-previews` 取得最新結果再送。
// @Description
// @Description  404：`pricePreviewId` 不存在或不屬於這個買家（wrong-owner 與亂數 ID 回同一種形狀，不洩漏存在性）。
// @Tags         AI
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id  path  string  true  "需求 ID（TSID 字串）"
// @Param        request  body  ProcurementRfqConfirmationRequest  true  "RFQ 報價確認輸入"
// @Success      200  {object}  object{code=int,message=string,data=ProcurementRfqConfirmationResponseDTO}
// @Failure      400  {object}  object{code=int,reason=string,message=string,traceId=string}
// @Failure      404  {object}  object{code=int,reason=string,message=string,traceId=string}
// @Failure      409  {object}  object{code=int,reason=string,message=string,traceId=string}
// @Router       /app/buyers/me/procurement/requests/{id}/rfq-confirmations [post]
func (h *Holder) ProxyProcurementRfqConfirmation(c *gin.Context) {
	buyerID, ok := h.resolveBuyerID(c)
	if !ok {
		return
	}
	var req ProcurementRfqConfirmationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		badProcurementRequest(c, "invalid request body")
		return
	}

	ctx := grpcclient.WithBuyerID(c.Request.Context(), buyerID)
	result, err := h.AIProcurement.ConfirmRfqSnapshot(ctx, req.toProto(c.Param("id")))
	if err != nil {
		writeProcurementError(c, err)
		return
	}
	response.OK(c, rfqConfirmationResponseFromProto(result))
}

// ProxyProcurementRfqPublicationPreview proxies an "RFQ publication preview"
// request to ai-service (issue #2450 Slice 1).
//
// @Summary      Hawkr 幫找 — RFQ 發布預覽（產生不可變的發布核准）
// @Description  買家對一張已確認的詢價定案內容（`confirmedRfqSnapshotId`）決定「要發給誰」，伺服器編譯出賣家會看到的內容並留下一筆
// @Description  versioned、內容不可變的發布核准（`approvalId` ＋ `approvalVersion`）。
// @Description
// @Description  ⛔ 這一步**不會**建立發布、不會寄出任何邀請、不會通知任何賣家。它只固定「發什麼、發給誰」這個核准意圖；
// @Description  真正送出是後續的 execute（尚未開放）。
// @Description
// @Description  `approvalId` 與 `approvalVersion` **兩者都要收好**：execute 會拿這一對做樂觀鎖比對，這支回應是唯一的來源。
// @Description
// @Description  `audienceMode`：`WISH_POOL`（公開池，發給全站未停權賣家，不需要 `candidateSetId`／`selectedCandidateIds`）
// @Description  或 `DIRECT_RECIPIENTS`（指定對象，兩者必填；`selectedCandidateIds` 的**順序有意義**，換順序等於換受眾）。
// @Description  兩者混用（許願池帶了候選、或指定對象沒選任何候選）會回 400。
// @Description
// @Description  `sellerFacingPayloadJson` 是賣家會看到的完整內容，由伺服器從定案內容編譯，買家的可接受價格上下限不會出現在裡面。
// @Description
// @Description  Idempotency：同一買家同一 `previewClientRequestId` 重送回同一筆核准（不重算、不重建）。同一個 key 換內容則回
// @Description  409 `IDEMPOTENCY_KEY_CONFLICT`，且不會改動既有那一筆。
// @Description
// @Description  409 `STALE_REVISION`：`expectedRevisionId` 與目前的 active revision 不符，請重讀 snapshot 再送。
// @Description  409 `APPROVAL_STALE`：定案內容不屬於這個需求，或它是針對舊版需求確認的，請重新呼叫 `/rfq-confirmations`。
// @Description  400 `LANE_MISMATCH`：選到站外業者（EXTERNAL lane），目前還不能作為發布對象。
// @Description  404：`confirmedRfqSnapshotId` 不存在或不屬於這個買家（wrong-owner 與亂數 ID 回同一種形狀，不洩漏存在性）。
// @Tags         AI
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id  path  string  true  "需求 ID（TSID 字串）"
// @Param        request  body  ProcurementRfqPublicationPreviewRequest  true  "RFQ 發布預覽輸入"
// @Success      200  {object}  object{code=int,message=string,data=ProcurementRfqPublicationPreviewResponseDTO}
// @Failure      400  {object}  object{code=int,reason=string,message=string,traceId=string}
// @Failure      404  {object}  object{code=int,reason=string,message=string,traceId=string}
// @Failure      409  {object}  object{code=int,reason=string,message=string,traceId=string}
// @Router       /app/buyers/me/procurement/requests/{id}/rfq-publications/preview [post]
func (h *Holder) ProxyProcurementRfqPublicationPreview(c *gin.Context) {
	buyerID, ok := h.resolveBuyerID(c)
	if !ok {
		return
	}
	var req ProcurementRfqPublicationPreviewRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		badProcurementRequest(c, "invalid request body")
		return
	}

	ctx := grpcclient.WithBuyerID(c.Request.Context(), buyerID)
	result, err := h.AIProcurement.PreviewRfqPublication(ctx, req.toProto(c.Param("id")))
	if err != nil {
		writeProcurementError(c, err)
		return
	}
	response.OK(c, rfqPublicationPreviewResponseFromProto(result))
}

// ProxyProcurementRfqPublicationExecute proxies an "RFQ publication execute"
// request to ai-service (issue #2450 Slice 2a, Scope 4).
//
// @Summary      Hawkr 幫找 — RFQ 發布執行（把 preview 核准的內容真正發布）
// @Description  買家帶著 preview 回應的 `approvalId`／`approvalVersion`／`sellerFacingPayloadHash`（此處欄位名為 `payloadHash`）
// @Description  執行發布：伺服器對 `approvalId`＋`approvalVersion` 做樂觀鎖（CAS）比對，成功後把 preview 已固定好的內容
// @Description  寫成一筆 versioned 的 publication。
// @Description
// @Description  ⛔ body 只能帶參照與 CAS token（`approvalId`／`approvalVersion`／`expectedRevisionId`／`payloadHash`），
// @Description  不接受任何 browser-side 對「發什麼、發給誰」的 override——這些全部從 approval 讀（#2450 Scope 4）。
// @Description  `requestId` 一律取自 URL path，body 不接受、也不會被採用。
// @Description
// @Description  ⛔⛔ **`state = "PUBLISHED"` 只代表伺服器端交易完成，不代表 RFQ 已投遞、已進 Wish Pool 索引、
// @Description  或已被任何賣家看到。本 slice 刻意零對外送達**（DIRECT 的收件者快照／邀請／通知在 Slice 2b，
// @Description  WISH_POOL 的 projection／索引 outbox 在 Slice 2c）。前端文案必須使用「已建立發布，正在處理」，
// @Description  不得寫成「已送出」「已通知賣家」或任何暗示送達的字樣——這裡沒寫清楚，前端就會抄錯文案。
// @Description
// @Description  Idempotency：同一買家同一 `clientRequestId` 重送回同一筆 publication（`replayed: true`，不重建）。
// @Description  這是 execute 自己的 idempotency key，跟 preview 的 `previewClientRequestId` 是不同鍵。
// @Description  同一個 key 換 `approvalId` 回 409 `IDEMPOTENCY_KEY_CONFLICT`，且不會改動既有那一筆。
// @Description
// @Description  409 `APPROVAL_STALE`：`approvalId`／`approvalVersion` 對不上目前狀態（已被消費、已過期、
// @Description  或 `payloadHash` 跟 preview 當時編譯出來的不一致），請重新呼叫 `/rfq-publications/preview`。
// @Description  409 `STALE_REVISION`：`expectedRevisionId` 與目前的 active revision 不符，請重讀 snapshot 再送。
// @Description  422 `PRICE_BASIS_UNAVAILABLE`：沿用 preview 既有的映射，核准當下已經擋過一次，
// @Description  這裡是防禦性保留，理論上不會再被觸發。
// @Description  500：伺服器狀態不一致（`audience_mode` 落在值域外，或冪等回放讀不到本該同一交易內
// @Description  寫入的 publication），不是買家可修正的輸入問題，理論上不會被觸發。
// @Tags         AI
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id  path  string  true  "需求 ID（TSID 字串）"
// @Param        request  body  ProcurementRfqPublicationExecuteRequest  true  "RFQ 發布執行輸入"
// @Success      200  {object}  object{code=int,message=string,data=ProcurementRfqPublicationExecuteResponseDTO}
// @Failure      400  {object}  object{code=int,reason=string,message=string,traceId=string}
// @Failure      404  {object}  object{code=int,reason=string,message=string,traceId=string}
// @Failure      409  {object}  object{code=int,reason=string,message=string,traceId=string}
// @Failure      422  {object}  object{code=int,reason=string,message=string,traceId=string}
// @Failure      500  {object}  object{code=int,reason=string,message=string,traceId=string}
// @Router       /app/buyers/me/procurement/requests/{id}/rfq-publications [post]
func (h *Holder) ProxyProcurementRfqPublicationExecute(c *gin.Context) {
	buyerID, ok := h.resolveBuyerID(c)
	if !ok {
		return
	}
	var req ProcurementRfqPublicationExecuteRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		badProcurementRequest(c, "invalid request body")
		return
	}

	ctx := grpcclient.WithBuyerID(c.Request.Context(), buyerID)
	result, err := h.AIProcurement.ExecuteRfqPublication(ctx, req.toProto(c.Param("id")))
	if err != nil {
		writeProcurementError(c, err)
		return
	}
	response.OK(c, rfqPublicationExecuteResponseFromProto(result))
}

// ProxyProcurementConversations proxies the conversation list to ai-service.
//
// @Summary      Hawkr 幫找 — 對話列表
// @Tags         AI
// @Produce      json
// @Security     BearerAuth
// @Param        cursor_id  query  string  false  "上一頁回傳的 nextCursorId（不透明 cursor token，直接透傳即可，不代表任何特定欄位值）"
// @Param        page_size  query  string  false  "每頁筆數（預設 20，最大 50）"
// @Success      200  {object}  object{code=int,message=string,data=ProcurementListConversationsResponseDTO}
// @Failure      401  {object}  object{code=int,message=string,traceId=string}
// @Router       /app/buyers/me/procurement/conversations [get]
func (h *Holder) ProxyProcurementConversations(c *gin.Context) {
	buyerID, ok := h.resolveBuyerID(c)
	if !ok {
		return
	}
	ctx := grpcclient.WithBuyerID(c.Request.Context(), buyerID)
	result, err := h.AIProcurement.ListConversations(ctx, &pb.ListConversationsRequest{
		CursorId: c.Query("cursor_id"), PageSize: c.Query("page_size"),
	})
	if err != nil {
		writeProcurementError(c, err)
		return
	}
	response.OK(c, listConversationsResponseFromProto(result))
}

// ProxyProcurementConversationMessages proxies the conversation transcript to ai-service.
//
// @Summary      Hawkr 幫找 — 對話 transcript（深連結續聊）
// @Tags         AI
// @Produce      json
// @Security     BearerAuth
// @Param        id  path  string  true  "對話 ID（TSID 字串）"
// @Success      200  {object}  object{code=int,message=string,data=ProcurementConversationMessagesResponseDTO}
// @Failure      404  {object}  object{code=int,reason=string,message=string,traceId=string}
// @Router       /app/buyers/me/procurement/conversations/{id}/messages [get]
func (h *Holder) ProxyProcurementConversationMessages(c *gin.Context) {
	buyerID, ok := h.resolveBuyerID(c)
	if !ok {
		return
	}
	ctx := grpcclient.WithBuyerID(c.Request.Context(), buyerID)
	result, err := h.AIProcurement.GetConversationMessages(ctx, &pb.GetConversationMessagesRequest{ConversationId: c.Param("id")})
	if err != nil {
		writeProcurementError(c, err)
		return
	}
	response.OK(c, conversationMessagesResponseFromProto(result))
}

// ProxyProcurementConversationRuntimeSnapshot proxies the thread-rebuild projection to ai-service.
//
// @Summary      Hawkr 幫找 — 對話 runtime snapshot（重新整理後重建 thread）
// @Tags         AI
// @Produce      json
// @Security     BearerAuth
// @Param        id  path  string  true  "對話 ID（TSID 字串）"
// @Success      200  {object}  object{code=int,message=string,data=ProcurementRuntimeSnapshotResponseDTO}
// @Failure      404  {object}  object{code=int,reason=string,message=string,traceId=string}
// @Router       /app/buyers/me/procurement/conversations/{id}/runtime-snapshot [get]
func (h *Holder) ProxyProcurementConversationRuntimeSnapshot(c *gin.Context) {
	buyerID, ok := h.resolveBuyerID(c)
	if !ok {
		return
	}
	ctx := grpcclient.WithBuyerID(c.Request.Context(), buyerID)
	result, err := h.AIProcurement.GetRuntimeSnapshot(ctx, &pb.GetRuntimeSnapshotRequest{ConversationId: c.Param("id")})
	if err != nil {
		writeProcurementError(c, err)
		return
	}
	response.OK(c, runtimeSnapshotResponseFromProto(result))
}

// ProxyProcurementConversationRename proxies a conversation rename to ai-service.
//
// @Summary      Hawkr 幫找 — 對話改名
// @Tags         AI
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id  path  string  true  "對話 ID（TSID 字串）"
// @Param        request  body  ProcurementRenameConversationRequest  true  "新的對話標題"
// @Success      200  {object}  object{code=int,message=string}
// @Failure      400  {object}  object{code=int,reason=string,message=string,traceId=string}
// @Failure      404  {object}  object{code=int,reason=string,message=string,traceId=string}
// @Failure      422  {object}  object{code=int,reason=string,message=string,traceId=string}
// @Router       /app/buyers/me/procurement/conversations/{id} [patch]
func (h *Holder) ProxyProcurementConversationRename(c *gin.Context) {
	buyerID, ok := h.resolveBuyerID(c)
	if !ok {
		return
	}
	var req ProcurementRenameConversationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		badProcurementRequest(c, "invalid request body")
		return
	}
	ctx := grpcclient.WithBuyerID(c.Request.Context(), buyerID)
	if err := h.AIProcurement.RenameConversation(ctx, &pb.RenameConversationRequest{
		ConversationId: c.Param("id"), Title: req.Title, ClientRequestId: req.ClientRequestID,
	}); err != nil {
		writeProcurementError(c, err)
		return
	}
	response.OK(c, nil)
}

// ProxyProcurementConversationDelete proxies a conversation soft-delete to ai-service.
//
// @Summary      Hawkr 幫找 — 對話軟刪
// @Tags         AI
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id  path  string  true  "對話 ID（TSID 字串）"
// @Param        request  body  ProcurementDeleteConversationRequest  true  "冪等鍵"
// @Success      200  {object}  object{code=int,message=string}
// @Failure      400  {object}  object{code=int,reason=string,message=string,traceId=string}
// @Failure      404  {object}  object{code=int,reason=string,message=string,traceId=string}
// @Router       /app/buyers/me/procurement/conversations/{id} [delete]
func (h *Holder) ProxyProcurementConversationDelete(c *gin.Context) {
	buyerID, ok := h.resolveBuyerID(c)
	if !ok {
		return
	}
	var req ProcurementDeleteConversationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		badProcurementRequest(c, "invalid request body")
		return
	}
	ctx := grpcclient.WithBuyerID(c.Request.Context(), buyerID)
	if err := h.AIProcurement.DeleteConversation(ctx, &pb.DeleteConversationRequest{
		ConversationId: c.Param("id"), ClientRequestId: req.ClientRequestID,
	}); err != nil {
		writeProcurementError(c, err)
		return
	}
	response.OK(c, nil)
}

// ProxyProcurementRunEvents proxies Task 11's WatchRunEvents replay/live-subscribe
// RPC to the browser as AG-UI SSE frames. Unlike every other procurement route
// in this file, it is not nested under /requests/:id — a run outlives one
// request (SubmitAnswer／Rewind also produce TURN runs against the same
// run_id), so it is mounted as a top-level /runs/:runId/events route
// (routes_ai.go), same style as /conversations.
//
// @Summary      Hawkr 幫找 — Run ledger 事件重播／即時訂閱（SSE）
// @Description  以 `Last-Event-ID`（十進位字串 ledger seq）決定重播起點，缺省表示從頭重播。游標早於保留視窗回 410（REPLAY_CURSOR_EXPIRED，snapshot 已不可 replay，改用 GetSnapshot／GetRun）；游標高於目前 high-water mark 回 409（REPLAY_CURSOR_AHEAD）。兩者皆以 in-band 錯誤 frame 呈現，HTTP 層狀態碼恆為 200（SSE 慣例，串流已開始後無法再改 HTTP status）。
// @Tags         AI
// @Produce      text/event-stream
// @Security     BearerAuth
// @Param        runId  path  string  true  "Run ID（TSID 字串）"
// @Success      200  {string}  string  "SSE stream"
// @Failure      401  {object}  object{code=int,message=string}
// @Failure      403  {object}  object{code=int,message=string}
// @Failure      502  {object}  object{code=int,message=string}
// @Failure      503  {object}  object{code=int,reason=string,message=string,traceId=string}
// @Router       /app/buyers/me/procurement/runs/{runId}/events [get]
func (h *Holder) ProxyProcurementRunEvents(c *gin.Context) {
	if !h.ProcurementReplaySseEnabled {
		writeProcurementErrorBody(c, http.StatusServiceUnavailable, "CAPABILITY_UNAVAILABLE", "replay 端點尚未開放")
		return
	}
	buyerID, ok := h.resolveBuyerID(c)
	if !ok {
		return
	}

	runID := c.Param("runId")
	lastEventID := c.GetHeader("Last-Event-ID")

	proxy.ServeAIProcurementRunEventsStream(c, lastEventID, func(ctx context.Context, afterSeq string) (proxy.RunEventStream, error) {
		return h.AIProcurement.WatchRunEvents(grpcclient.WithBuyerID(ctx, buyerID), &pb.WatchRunEventsRequest{
			RunId: runID, AfterSeq: afterSeq,
		})
	})
}

// ProxyProcurementRunCancel proxies Task 14's CancelRun unary RPC to the
// browser (doc57 §5.4). Unlike ProxyProcurementRunEvents this is a plain
// unary JSON endpoint (not SSE) — mounted at the same top-level /runs/:runId
// level for the same reason (a run outlives one request).
//
// Cache-Control: private, no-store is applied by the procurement route
// group's existing middleware.PrivateNoStore() (routes_ai.go), not set again
// here — same as every other unary handler in this file.
//
// @Summary      Hawkr 幫找 — 取消 run
// @Description  cancelDisposition 值域固定 REQUESTED／ALREADY_CANCELLED／ALREADY_TERMINAL／TOO_LATE（doc57 §5.4，不得新增）；四者皆為成功結果。expectedRunVersion 是過期快照時回 409（APPROVAL_STALE）。
// @Tags         AI
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        runId  path  string  true  "Run ID（TSID 字串）"
// @Param        request  body  ProcurementRunCancelRequest  true  "取消命令"
// @Success      200  {object}  object{code=int,message=string,data=ProcurementRunCancelResponseDTO}
// @Failure      400  {object}  object{code=int,reason=string,message=string,traceId=string}
// @Failure      404  {object}  object{code=int,reason=string,message=string,traceId=string}
// @Failure      409  {object}  object{code=int,reason=string,message=string,traceId=string}
// @Router       /app/buyers/me/procurement/runs/{runId}/cancel [post]
func (h *Holder) ProxyProcurementRunCancel(c *gin.Context) {
	buyerID, ok := h.resolveBuyerID(c)
	if !ok {
		return
	}
	var req ProcurementRunCancelRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		badProcurementRequest(c, "invalid request body")
		return
	}
	ctx := grpcclient.WithBuyerID(c.Request.Context(), buyerID)
	result, err := h.AIProcurement.CancelRun(ctx, &pb.CancelRunRequest{
		RunId: c.Param("runId"), ClientRequestId: req.ClientRequestID,
		ExpectedRunVersion: req.ExpectedRunVersion, Reason: req.Reason,
	})
	if err != nil {
		writeProcurementError(c, err)
		return
	}
	response.OK(c, runCancelResponseFromProto(result))
}

// ProxyProcurementRunResume proxies Task 14's ResumeRun unary RPC to the
// browser (doc57 §5.5). Same transport shape as ProxyProcurementRunCancel
// above — see that handler's doc comment for the Cache-Control note.
//
// @Summary      Hawkr 幫找 — 恢復 run（回覆待處理 interrupt）
// @Description  對一個 WAITING_FOR_INPUT run 的 interrupt 提交回覆，建立一個新的 child run 繼續執行。已被處理過的 interrupt 回 409（APPROVAL_ALREADY_RESOLVED）。
// @Tags         AI
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        runId  path  string  true  "Run ID（TSID 字串，僅供路由對齊 REST 慣例，實際 resolve 依 interruptId）"
// @Param        request  body  ProcurementRunResumeRequest  true  "恢復命令"
// @Success      200  {object}  object{code=int,message=string,data=ProcurementRunResumeResponseDTO}
// @Failure      400  {object}  object{code=int,reason=string,message=string,traceId=string}
// @Failure      404  {object}  object{code=int,reason=string,message=string,traceId=string}
// @Failure      409  {object}  object{code=int,reason=string,message=string,traceId=string}
// @Router       /app/buyers/me/procurement/runs/{runId}/resume [post]
func (h *Holder) ProxyProcurementRunResume(c *gin.Context) {
	buyerID, ok := h.resolveBuyerID(c)
	if !ok {
		return
	}
	var req ProcurementRunResumeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		badProcurementRequest(c, "invalid request body")
		return
	}
	ctx := grpcclient.WithBuyerID(c.Request.Context(), buyerID)
	result, err := h.AIProcurement.ResumeRun(ctx, &pb.ResumeRunRequest{
		InterruptId: req.InterruptID, ClientRequestId: req.ClientRequestID,
		ResponsePayload: req.Payload,
	})
	if err != nil {
		writeProcurementError(c, err)
		return
	}
	response.OK(c, runResumeResponseFromProto(result))
}

type SellerProductAutofillRequest struct {
	Text     string `json:"text" example:"東部小農友善種植芭樂，3斤裝250元，5斤裝400元，產地台東"`
	ImageURL string `json:"imageUrl,omitempty" example:"https://cdn.example.com/image.jpg"`
}

type SellerProductAutofillResponse struct {
	Data    *SellerProductDraft `json:"data,omitempty"`
	Message string              `json:"message,omitempty"`
	TraceID string              `json:"traceId,omitempty"`
	Code    int                 `json:"code"`
}

type SellerProductDraft struct {
	Name               string                    `json:"name"`
	Origin             string                    `json:"origin,omitempty"`
	Description        string                    `json:"description,omitempty"`
	CategoryCandidates []SellerCategoryCandidate `json:"categoryCandidates"`
	Skus               []SellerSkuDraft          `json:"skus"`
	Tags               []string                  `json:"tags"`
	Warnings           []string                  `json:"warnings"`
}

type SellerCategoryCandidate struct {
	CatalogID  string `json:"catalogId"`
	Path       string `json:"path"`
	Confidence string `json:"confidence"`
}

type SellerSkuDraft struct {
	SkuName string `json:"skuName"`
	Price   string `json:"price"`
	Unit    string `json:"unit"`
}

// ProxySellerProductListingSSE proxies the seller product-listing chat request to ai-service.
//
// @Summary      賣家 AI 小幫手 — 對話式上架助手（SSE）
// @Description  多輪對話式上架引導，AI 透過 tool 呼叫把欄位累積到商品草稿。
// @Tags         AI
// @Accept       json
// @Produce      text/event-stream
// @Security     BearerAuth
// @Param        request  body  SellerAssistantSSERequest  true  "對話請求"
// @Success      200  {string}  string  "SSE stream"
// @Failure      401  {object}  object{code=int,message=string}
// @Failure      403  {object}  object{code=int,message=string}
// @Failure      502  {object}  object{code=int,message=string}
// @Router       /app/sellers/me/assistant/product-listing/chat [post]
func (h *Holder) ProxySellerProductListingSSE(c *gin.Context) {
	var req assistantStreamRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	storeID, ok := h.resolveStoreID(c)
	if !ok {
		return
	}
	pbReq := req.toProto()
	proxy.ServeAIAssistantStream(c, req.SessionID, func(ctx context.Context) (grpcclient.AssistantStream, error) {
		return h.AIAssistant.SellerProductListingChat(grpcclient.WithStoreID(ctx, storeID), pbReq)
	})
}

// ProxySellerProductAutofill proxies the seller product autofill request to ai-service.
//
// @Summary      賣家 AI 小幫手 — 商品表單預填
// @Description  賣家輸入商品自由文字描述（可附圖片 URL），由 AI 解析並回傳結構化商品草稿（非 SSE，一次性 JSON 回應）。
// @Tags         賣家助手 (Seller Assistant)
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        request  body  SellerProductAutofillRequest  true  "商品描述請求"
// @Success      200  {object}  SellerProductAutofillResponse
// @Failure      400  {object}  object{code=int,message=string}
// @Failure      401  {object}  object{code=int,message=string}
// @Failure      403  {object}  object{code=int,message=string}
// @Failure      500  {object}  object{code=int,message=string}
// @Failure      503  {object}  object{code=int,message=string}
// @Router       /app/sellers/me/assistant/product-autofill [post]
func (h *Holder) ProxySellerProductAutofill(c *gin.Context) {
	var req sellerProductAutofillRequestDTO
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	storeID, ok := h.resolveStoreID(c)
	if !ok {
		return
	}
	ctx := grpcclient.WithStoreID(c.Request.Context(), storeID)
	resp, err := h.AIAssistant.SellerProductAutofill(ctx, &pb.SellerProductAutofillRequest{
		Text:     req.Text,
		ImageUrl: req.ImageURL,
	})
	if err != nil {
		response.GrpcErr(c, "ai", err)
		return
	}
	response.OK(c, toProductDraftDTO(resp.GetData()))
}

func (h *Holder) resolveAssistantActorHeader(c *gin.Context, upstreamPath string) (header, value string, ok bool) {
	if strings.HasPrefix(upstreamPath, "/buyer/") {
		buyerID, ok := h.resolveBuyerID(c)
		if !ok {
			return "", "", false
		}
		return "X-Buyer-Id", strconv.FormatInt(int64(buyerID), 10), true
	}
	storeID, ok := h.resolveStoreID(c)
	if !ok {
		return "", "", false
	}
	return "X-Store-Id", strconv.FormatInt(int64(storeID), 10), true
}
