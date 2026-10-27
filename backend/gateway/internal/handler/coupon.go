package handler

import (
	"errors"
	"strconv"

	"github.com/Will413028/hawkr/gateway/internal/buyerid"
	"github.com/Will413028/hawkr/gateway/internal/middleware"
	"github.com/Will413028/hawkr/gateway/internal/response"
	"github.com/Will413028/hawkr/go-pkg/identity"
	pb "github.com/Will413028/hawkr/go-pkg/proto/coupon"
	"github.com/gin-gonic/gin"
)

// ==================== DTO ====================
type TargetAccountResponse struct {
	BuyerId  string `json:"buyerId"`
	Nickname string `json:"nickname"`
}

// CouponTemplateResponse 優惠券模板回應
type CouponTemplateResponse struct {
	TemplateId        string `json:"templateId"`
	Title             string `json:"title"`
	Type              string `json:"type"`              // 類型（1=現金券, 2=折扣券）
	TargetType        string `json:"targetType"`        // 發放目標（1=全體買家, 2=賣家會員, 3=指定買家, 4=補償券）
	ApplicableStoreId string `json:"applicableStoreId"` // 適用賣家 ID（空=全平台）
	SellerName        string `json:"sellerName"`        // 商家名稱（空=全平台券）
	// 賣家 canonical 短碼（sellers.store_code）；applicableStoreId 是內部代理鍵，
	// 賣家頁 canonical 網址走 storeCode。與 sellerName 同源，空=全平台券或聚合失敗。
	StoreCode            string                  `json:"storeCode"`
	LogoKey              string                  `json:"logoKey"` // 商家 logo objectKey（SSOT；49 §6.1；空=全平台券或無頭像）
	LogoUrl              string                  `json:"logoUrl"` // 商家 logo 顯示 URL（Gateway 組）
	IssuerType           string                  `json:"issuerType"`
	IssuerAccountId      string                  `json:"issuerAccountId"`
	IssuerStoreId        string                  `json:"issuerStoreId"`
	ThresholdAmount      string                  `json:"thresholdAmount"`
	DiscountAmount       string                  `json:"discountAmount"`
	DiscountPercent      string                  `json:"discountPercent"`
	MaxDiscountAmount    string                  `json:"maxDiscountAmount"`
	StartAt              string                  `json:"startAt"`
	EndAt                string                  `json:"endAt"`
	UsageLimitPerAccount string                  `json:"usageLimitPerAccount"`
	TotalIssueLimit      string                  `json:"totalIssueLimit"`
	IssuedCount          string                  `json:"issuedCount"`
	Status               string                  `json:"status"` // 模板狀態（0=草稿, 1=進行中, 2=已停用, 3=已過期, 4=已收回）
	CreatedAt            string                  `json:"createdAt"`
	TargetAccounts       []TargetAccountResponse `json:"targetAccounts,omitempty"` // targetType=3(指定買家)/4(補償券) 時回傳
}

// CouponInstanceResponse 優惠券實例回應
type CouponInstanceResponse struct {
	InstanceId        string `json:"instanceId"`
	TemplateId        string `json:"templateId"`
	Title             string `json:"title"`
	Type              string `json:"type"`              // 類型（1=現金券, 2=折扣券）
	ApplicableStoreId string `json:"applicableStoreId"` // 適用賣家 ID（空=全平台）
	SellerName        string `json:"sellerName"`        // 商家名稱（空=全平台券）
	StoreCode         string `json:"storeCode"`         // 賣家 canonical 短碼；見 CouponTemplateResponse.StoreCode
	LogoKey           string `json:"logoKey"`           // 商家 logo objectKey（SSOT；49 §6.1；空=全平台券或無頭像）
	LogoUrl           string `json:"logoUrl"`           // 商家 logo 顯示 URL（Gateway 組）
	ThresholdAmount   string `json:"thresholdAmount"`
	DiscountAmount    string `json:"discountAmount"`
	DiscountPercent   string `json:"discountPercent"`
	MaxDiscountAmount string `json:"maxDiscountAmount"`
	StartAt           string `json:"startAt"`
	EndAt             string `json:"endAt"`
	Status            string `json:"status"` // 實例狀態（0=未使用, 1=已鎖定, 2=已使用, 3=已過期, 4=已收回, 5=已作廢）
	CreatedAt         string `json:"createdAt"`
}

// CreateCouponTemplateRequest 賣家建立優惠券請求
type CreateCouponTemplateRequest struct {
	Title             string   `json:"title" binding:"required"`
	Type              string   `json:"type" binding:"required"`
	TargetType        string   `json:"targetType" binding:"required"` // 1=全體買家 / 2=賣家會員 / 3=指定買家
	ThresholdAmount   string   `json:"thresholdAmount"`
	DiscountAmount    string   `json:"discountAmount"`
	DiscountPercent   string   `json:"discountPercent"`
	MaxDiscountAmount string   `json:"maxDiscountAmount"`
	StartAt           string   `json:"startAt" binding:"required"`
	EndAt             string   `json:"endAt" binding:"required"`
	BuyerAccountIds   []string `json:"buyerAccountIds"`
	BuyerIds          []string `json:"buyerIds"`
}

// CouponTemplateDetailResponse 模板詳情回應（含統計）
type CouponTemplateDetailResponse struct {
	Template      *CouponTemplateResponse `json:"template"`
	UsedCount     string                  `json:"usedCount"`
	ReservedCount string                  `json:"reservedCount"`
}

// RevokeCouponTemplateRequest 收回模板請求
type RevokeCouponTemplateRequest struct {
	Reason string `json:"reason"`
}

// AdminCreateCouponTemplateRequest 管理員建立模板請求
type AdminCreateCouponTemplateRequest struct {
	Title                string `json:"title" binding:"required"`
	Type                 string `json:"type" binding:"required"`
	ApplicableStoreId    string `json:"applicableStoreId"`
	ThresholdAmount      string `json:"thresholdAmount"`
	DiscountAmount       string `json:"discountAmount"`
	DiscountPercent      string `json:"discountPercent"`
	MaxDiscountAmount    string `json:"maxDiscountAmount"`
	StartAt              string `json:"startAt" binding:"required"`
	EndAt                string `json:"endAt" binding:"required"`
	UsageLimitPerAccount string `json:"usageLimitPerAccount"`
	TotalIssueLimit      string `json:"totalIssueLimit"`
}

// AdminIssueCouponsRequest 管理員發行優惠券請求
type AdminIssueCouponsRequest struct {
	BuyerAccountIds     []string `json:"buyerAccountIds" binding:"required"`
	ImmediateDistribute bool     `json:"immediateDistribute"` // 是否立即代領（預設 false）
}

// AdminIssueCouponsResponse 管理員發行優惠券回應
type AdminIssueCouponsResponse struct {
	IssuedCount      string `json:"issuedCount"`
	DistributedCount string `json:"distributedCount,omitempty"` // 代領數量（僅 immediateDistribute=true）
}

// ClaimCouponResponse 買家領取優惠券回應
type ClaimCouponResponse struct {
	ClaimedAt string `json:"claimedAt,omitempty"` // ISO 8601
	Success   bool   `json:"success"`
}

// AdminDistributeTemplateResponse 管理員代領模板回應
type AdminDistributeTemplateResponse struct {
	DistributedCount string `json:"distributedCount"` // 本次代領數
}

// AdminGrantCompensationRequest 發放補償券請求
type AdminGrantCompensationRequest struct {
	BuyerAccountId    string `json:"buyerAccountId" binding:"required"`
	Amount            string `json:"amount" binding:"required"`
	RefOrderNo        string `json:"refOrderNo"`
	ApplicableStoreId string `json:"applicableStoreId" binding:"required"`
	Reason            string `json:"reason" binding:"required"`
}

// AdminVoidCouponRequest 作廢優惠券請求
type AdminVoidCouponRequest struct {
	Reason string `json:"reason" binding:"required"`
}

// AdminUpdateCouponQuotaRequest 更新配額請求
type AdminUpdateCouponQuotaRequest struct {
	MaxTemplateCount string `json:"maxTemplateCount" binding:"required"`
}

// ==================== 共用工具 ====================

func getAccountType(c *gin.Context) int {
	accountType, _ := c.Get("accountType")
	code, _ := accountType.(int)
	return code
}

func (h *Holder) resolveTargetBuyerID(c *gin.Context, buyerAccountIDRaw string) (int64, bool) {
	accountID, err := strconv.ParseInt(buyerAccountIDRaw, 10, 64)
	if err != nil {
		response.BadRequest(c, "invalid buyer_account_id")
		return 0, false
	}
	resolved, rerr := h.BuyerID.CurrentBuyerID(c.Request.Context(), identity.AccountID(accountID))
	if rerr != nil {
		if errors.Is(rerr, buyerid.ErrNoCurrentBuyer) {
			response.BadRequest(c, "buyer_account_id has no active buyer")
			return 0, false
		}
		response.GrpcErr(c, "buyer", rerr)
		return 0, false
	}
	return int64(resolved), true
}

func (h *Holder) resolveTargetBuyerIDs(c *gin.Context, raw []string) ([]string, bool) {
	if len(raw) == 0 {
		return raw, true
	}
	resolved := make([]string, 0, len(raw))
	for _, idStr := range raw {
		buyerID, ok := h.resolveTargetBuyerID(c, idStr)
		if !ok {
			return nil, false
		}
		resolved = append(resolved, strconv.FormatInt(buyerID, 10))
	}
	return resolved, true
}

func (h *Holder) resolveBuyerIDDirectly(c *gin.Context, raw string) (int64, bool) {
	id, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || id <= 0 {
		response.BadRequest(c, "invalid buyer_id")
		return 0, false
	}
	return id, true
}

func (h *Holder) validateBuyerIDs(c *gin.Context, raw []string) ([]string, bool) {
	validated := make([]string, 0, len(raw))
	for _, idStr := range raw {
		id, err := strconv.ParseInt(idStr, 10, 64)
		if err != nil || id <= 0 {
			response.BadRequest(c, "invalid buyer id")
			return nil, false
		}
		validated = append(validated, strconv.FormatInt(id, 10))
	}
	return validated, true
}

// mergeBuyerIDs 合併兩個獨立來源的目標買家清單（resolveTargetBuyerIDs
// 解析出來的、validateBuyerIDs 驗證過的），去重後回傳。
func mergeBuyerIDs(resolved, alreadyResolved []string) []string {
	seen := make(map[string]struct{}, len(resolved)+len(alreadyResolved))
	merged := make([]string, 0, len(resolved)+len(alreadyResolved))
	for _, id := range resolved {
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		merged = append(merged, id)
	}
	for _, id := range alreadyResolved {
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		merged = append(merged, id)
	}
	return merged
}

// ==================== Buyer API ====================

// GetCoupons 取得優惠券列表（根據 view 參數分流）
// @Summary      取得優惠券列表
// @Description  買家查詢自己的優惠券（預設），或賣家查詢買家持券（view=seller）
// @Tags         優惠券 (Coupon)
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        view          query  string  false  "檢視模式：空值=買家自己的券, seller=查詢買家持券"
// @Param        status        query  string  false  "篩選狀態（空字串=全部）"
// @Param        buyer_id      query  int64   false  "目標買家 buyer_id（view=seller 時必填）"
// @Param        page          query  string  false  "頁碼（預設 1）"
// @Param        page_size     query  string  false  "每頁筆數（預設 20）"
// @Success      200  {object}  object{code=int,data=object{records=[]CouponInstanceResponse,total=string,page=string,pageSize=string}}
// @Failure      400  {object}  object{code=int,message=string,traceId=string}
// @Failure      401  {object}  object{code=int,message=string,traceId=string}
// @Failure      403  {object}  object{code=int,message=string,traceId=string}
// @Router       /app/coupons [get]
func (h *Holder) GetCoupons(c *gin.Context) {
	accountID, ok := getAccountID(c)
	if !ok {
		response.Unauthorized(c)
		return
	}

	// §5.1 角色分離：買家/賣家使用獨立 RPC，Gateway 依 view 參數分發
	view := c.Query("view")
	accountType := getAccountType(c)

	if view == "seller" {
		// 賣家角色 + 停權能力檢查（CAP_ORDER_FULFILL）已由 sellerViewGuard 於路由層
		// 完成（46 §3.3 rule 2；IsSeller 判斷收斂進 authz package）。
		storeID, ok := h.resolveStoreIDFor(c, accountID)
		if !ok {
			return
		}
		h.handleListBuyerCouponsForSeller(c, storeID)
		return
	}

	// 預設：買家查詢自己的券
	if !middleware.IsBuyer(accountType) {
		response.Forbidden(c, "需要買家權限")
		return
	}
	h.handleListMyCoupons(c, accountID)
}

func (h *Holder) handleListMyCoupons(c *gin.Context, accountID int64) {
	buyerID, ok := h.resolveBuyerIDFor(c, accountID)
	if !ok {
		return
	}

	resp, err := h.Coupon.ListMyCoupons(c.Request.Context(), &pb.ListMyCouponsRequest{
		BuyerId:   int64(buyerID),
		AccountId: accountID,
		Status:    c.DefaultQuery("status", ""),
		Page:      c.DefaultQuery("page", defaultPageStr),
		PageSize:  c.DefaultQuery("page_size", defaultPageSizeStr),
	})
	if err != nil {
		response.GrpcErr(c, "coupon", err)
		return
	}

	if resp.Code != 0 {
		response.GrpcResult(c, resp.Code, resp.Message, nil)
		return
	}

	// 可用券（Template 資格計算結果）
	available := make([]gin.H, 0, len(resp.Records))
	for _, tpl := range resp.Records {
		available = append(available, templateInfoToMap(tpl, h.MediaBaseURL))
	}
	// 已使用券（Instance 紀錄）
	used := make([]gin.H, 0, len(resp.UsedRecords))
	for _, inst := range resp.UsedRecords {
		used = append(used, instanceInfoToMap(inst, h.MediaBaseURL))
	}
	response.OK(c, gin.H{
		"records":     available,
		"usedRecords": used,
		"total":       resp.Total,
		"page":        resp.Page,
		"pageSize":    resp.PageSize,
	})
}

func (h *Holder) handleListBuyerCouponsForSeller(c *gin.Context, storeID identity.StoreID) {
	buyerIDRaw := c.Query("buyer_id")
	if buyerIDRaw == "" {
		response.BadRequest(c, "buyer_id is required when view=seller")
		return
	}
	buyerID, ok := h.resolveBuyerIDDirectly(c, buyerIDRaw)
	if !ok {
		return
	}

	resp, err := h.Coupon.ListBuyerCouponsForSeller(c.Request.Context(), &pb.ListBuyerCouponsForSellerRequest{
		StoreId:  int64(storeID),
		BuyerId:  strconv.FormatInt(buyerID, 10),
		Status:   c.DefaultQuery("status", ""),
		Page:     c.DefaultQuery("page", defaultPageStr),
		PageSize: c.DefaultQuery("page_size", defaultPageSizeStr),
	})
	if err != nil {
		response.GrpcErr(c, "coupon", err)
		return
	}

	if resp.Code != 0 {
		response.GrpcResult(c, resp.Code, resp.Message, nil)
		return
	}

	available := make([]gin.H, 0, len(resp.Records))
	for _, tpl := range resp.Records {
		available = append(available, templateInfoToMap(tpl, h.MediaBaseURL))
	}
	used := make([]gin.H, 0, len(resp.UsedRecords))
	for _, inst := range resp.UsedRecords {
		used = append(used, instanceInfoToMap(inst, h.MediaBaseURL))
	}
	response.OK(c, gin.H{
		"records":     available,
		"usedRecords": used,
		"total":       resp.Total,
		"page":        resp.Page,
		"pageSize":    resp.PageSize,
	})
}

// ListAvailableCoupons 查詢可領取優惠券
// @Summary      查詢可領取優惠券
// @Description  買家查詢目前可領取的優惠券模板列表。帶 store_id 時僅回傳該商家券 + 全平台券（購物車場景）
// @Tags         優惠券 (Coupon)
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        page              query  string  false  "頁碼（預設 1）"
// @Param        page_size         query  string  false  "每頁筆數（預設 20）"
// @Param        store_id          query  string  false  "篩選商家 store_id（空 = 全部，有值 = 該商家券 + 全平台券）"
// @Success      200  {object}  object{code=int,data=object{records=[]CouponTemplateResponse,total=string,page=string,pageSize=string}}
// @Failure      400  {object}  object{code=int,message=string,traceId=string}
// @Failure      401  {object}  object{code=int,message=string,traceId=string}
// @Router       /app/coupons/available [get]
func (h *Holder) ListAvailableCoupons(c *gin.Context) {
	accountID, ok := getAccountID(c)
	if !ok {
		response.Unauthorized(c)
		return
	}

	buyerID, ok := h.resolveBuyerIDFor(c, accountID)
	if !ok {
		return
	}

	storeID := c.Query("store_id")
	orderAmount := c.Query("order_amount")
	if storeID == "" || orderAmount == "" {
		response.BadRequest(c, "store_id and order_amount are required")
		return
	}

	resp, err := h.Coupon.ListAvailableCoupons(c.Request.Context(), &pb.ListAvailableCouponsRequest{
		BuyerId:     int64(buyerID),
		Page:        c.DefaultQuery("page", defaultPageStr),
		PageSize:    c.DefaultQuery("page_size", defaultPageSizeStr),
		StoreId:     storeID,
		OrderAmount: orderAmount,
		AccountId:   accountID,
	})
	if err != nil {
		response.GrpcErr(c, "coupon", err)
		return
	}

	if resp.Code != 0 {
		response.GrpcResult(c, resp.Code, resp.Message, nil)
		return
	}

	records := make([]gin.H, 0, len(resp.Records))
	for _, item := range resp.Records {
		rec := gin.H{"estimatedDiscount": item.EstimatedDiscount}
		if item.Template != nil {
			rec["template"] = templateInfoToMap(item.Template, h.MediaBaseURL)
		}
		records = append(records, rec)
	}
	response.Page(c, records, resp.Total, resp.Page, resp.PageSize)
}

// ClaimCoupon 買家領取優惠券
// @Summary      領取優惠券
// @Description  買家主動領取指定模板的優惠券（限 status=1(進行中) 且 targetType=1(全體買家)/2(賣家會員) 的模板）
// @Tags         優惠券 (Coupon)
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        template_id  path  string  true  "模板 ID"
// @Success      200  {object}  object{code=int,data=ClaimCouponResponse}
// @Failure      400  {object}  object{code=int,message=string,traceId=string}
// @Failure      401  {object}  object{code=int,message=string,traceId=string}
// @Failure      403  {object}  object{code=int,message=string,traceId=string}
// @Router       /app/coupons/{template_id}/claim [post]
func (h *Holder) ClaimCoupon(c *gin.Context) {
	accountID, ok := getAccountID(c)
	if !ok {
		response.Unauthorized(c)
		return
	}

	accountType := getAccountType(c)
	if !middleware.IsBuyer(accountType) {
		response.Forbidden(c, "需要買家權限")
		return
	}

	buyerID, ok := h.resolveBuyerIDFor(c, accountID)
	if !ok {
		return
	}

	templateID := c.Param("template_id")
	if templateID == "" {
		response.BadRequest(c, "template_id is required")
		return
	}

	resp, err := h.Coupon.BuyerClaimCoupon(c.Request.Context(), &pb.BuyerClaimCouponRequest{
		BuyerId:    int64(buyerID),
		TemplateId: templateID,
		AccountId:  accountID,
	})
	if err != nil {
		response.GrpcErr(c, "coupon", err)
		return
	}

	if resp.Code != 0 {
		response.GrpcResult(c, resp.Code, resp.Message, nil)
		return
	}

	response.OK(c, gin.H{
		"success":   resp.Success,
		"claimedAt": resp.ClaimedAt,
	})
}

// ==================== Seller API ====================

// CreateCouponTemplate 賣家建立並發行優惠券
// @Summary      建立並發行優惠券
// @Description  賣家建立優惠券模板並立即發行給指定買家，需檢查配額上限
// @Tags         優惠券 (Coupon)
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        request  body  CreateCouponTemplateRequest  true  "建立優惠券請求"
// @Success      200  {object}  object{code=int,data=CouponTemplateResponse}
// @Failure      400  {object}  object{code=int,message=string,traceId=string}
// @Failure      403  {object}  object{code=int,message=string,traceId=string}
// @Router       /app/coupon_templates [post]
func (h *Holder) CreateCouponTemplate(c *gin.Context) {
	accountID, ok := getAccountID(c)
	if !ok {
		response.Unauthorized(c)
		return
	}
	storeID, ok := h.resolveStoreIDFor(c, accountID)
	if !ok {
		return
	}

	var req struct {
		Title             string   `json:"title" binding:"required"`
		Type              string   `json:"type" binding:"required"`
		TargetType        string   `json:"targetType" binding:"required"`
		ThresholdAmount   string   `json:"thresholdAmount"`
		DiscountAmount    string   `json:"discountAmount"`
		DiscountPercent   string   `json:"discountPercent"`
		MaxDiscountAmount string   `json:"maxDiscountAmount"`
		StartAt           string   `json:"startAt" binding:"required"`
		EndAt             string   `json:"endAt" binding:"required"`
		BuyerAccountIds   []string `json:"buyerAccountIds"`
		BuyerIds          []string `json:"buyerIds"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "invalid request: "+err.Error())
		return
	}

	resolvedBuyerIDs, ok := h.resolveTargetBuyerIDs(c, req.BuyerAccountIds)
	if !ok {
		return
	}
	validatedBuyerIds, ok := h.validateBuyerIDs(c, req.BuyerIds)
	if !ok {
		return
	}

	resp, err := h.Coupon.SellerCreateTemplate(c.Request.Context(), &pb.SellerCreateTemplateRequest{
		StoreId:           int64(storeID),
		Title:             req.Title,
		Type:              req.Type,
		TargetType:        req.TargetType,
		ThresholdAmount:   req.ThresholdAmount,
		DiscountAmount:    req.DiscountAmount,
		DiscountPercent:   req.DiscountPercent,
		MaxDiscountAmount: req.MaxDiscountAmount,
		StartAt:           req.StartAt,
		EndAt:             req.EndAt,
		BuyerIds:          mergeBuyerIDs(resolvedBuyerIDs, validatedBuyerIds),
	})
	if err != nil {
		response.GrpcErr(c, "coupon", err)
		return
	}

	if resp.Code != 0 {
		response.GrpcResult(c, resp.Code, resp.Message, nil)
		return
	}

	if resp.Data != nil {
		response.OK(c, templateInfoToMap(resp.Data, h.MediaBaseURL))
	} else {
		response.OK(c, nil)
	}
}

// ListCouponTemplates 賣家優惠券模板列表
// @Summary      取得賣家優惠券模板列表
// @Description  賣家查詢自己建立的優惠券模板列表
// @Tags         優惠券 (Coupon)
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        status     query  string  false  "篩選狀態（空字串=全部）"
// @Param        page       query  string  false  "頁碼（預設 1）"
// @Param        page_size  query  string  false  "每頁筆數（預設 20）"
// @Success      200  {object}  object{code=int,data=object{records=[]CouponTemplateResponse,total=string,page=string,pageSize=string}}
// @Failure      401  {object}  object{code=int,message=string,traceId=string}
// @Failure      403  {object}  object{code=int,message=string,traceId=string}
// @Router       /app/coupon_templates [get]
func (h *Holder) ListCouponTemplates(c *gin.Context) {
	accountID, ok := getAccountID(c)
	if !ok {
		response.Unauthorized(c)
		return
	}
	storeID, ok := h.resolveStoreIDFor(c, accountID)
	if !ok {
		return
	}

	resp, err := h.Coupon.SellerListTemplates(c.Request.Context(), &pb.SellerListTemplatesRequest{
		StoreId:  int64(storeID),
		Status:   c.DefaultQuery("status", ""),
		Page:     c.DefaultQuery("page", defaultPageStr),
		PageSize: c.DefaultQuery("page_size", defaultPageSizeStr),
	})
	if err != nil {
		response.GrpcErr(c, "coupon", err)
		return
	}

	if resp.Code != 0 {
		response.GrpcResult(c, resp.Code, resp.Message, nil)
		return
	}

	records := make([]gin.H, 0, len(resp.Records))
	for _, tpl := range resp.Records {
		records = append(records, templateInfoToMap(tpl, h.MediaBaseURL))
	}
	response.Page(c, records, resp.Total, resp.Page, resp.PageSize)
}

// GetCouponTemplateDetail 賣家取得模板詳情
// @Summary      取得優惠券模板詳情
// @Description  賣家查詢自己建立的優惠券模板詳情，包含已使用/未使用統計
// @Tags         優惠券 (Coupon)
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        template_id  path  string  true  "模板 ID"
// @Success      200  {object}  object{code=int,data=CouponTemplateDetailResponse}
// @Failure      401  {object}  object{code=int,message=string,traceId=string}
// @Failure      403  {object}  object{code=int,message=string,traceId=string}
// @Failure      404  {object}  object{code=int,message=string,traceId=string}
// @Router       /app/coupon_templates/{template_id} [get]
func (h *Holder) GetCouponTemplateDetail(c *gin.Context) {
	accountID, ok := getAccountID(c)
	if !ok {
		response.Unauthorized(c)
		return
	}
	storeID, ok := h.resolveStoreIDFor(c, accountID)
	if !ok {
		return
	}

	templateID := c.Param("template_id")
	if templateID == "" {
		response.BadRequest(c, "template_id is required")
		return
	}

	resp, err := h.Coupon.SellerGetTemplateDetail(c.Request.Context(), &pb.SellerGetTemplateDetailRequest{
		StoreId:    int64(storeID),
		TemplateId: templateID,
	})
	if err != nil {
		response.GrpcErr(c, "coupon", err)
		return
	}

	if resp.Code != 0 {
		response.GrpcResult(c, resp.Code, resp.Message, nil)
		return
	}

	data := gin.H{
		"usedCount":     resp.UsedCount,
		"reservedCount": resp.ReservedCount,
	}
	if resp.Data != nil {
		data["template"] = templateInfoToMap(resp.Data, h.MediaBaseURL)
	}
	response.OK(c, data)
}

// RevokeCouponTemplate 賣家收回優惠券模板
// @Summary      收回優惠券模板
// @Description  賣家收回指定的優惠券模板，同時收回所有已發行的未使用券
// @Tags         優惠券 (Coupon)
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        template_id  path  string  true  "模板 ID"
// @Param        request      body  RevokeCouponTemplateRequest  false  "收回原因"
// @Success      200  {object}  object{code=int,message=string}
// @Failure      401  {object}  object{code=int,message=string,traceId=string}
// @Failure      403  {object}  object{code=int,message=string,traceId=string}
// @Router       /app/coupon_templates/{template_id}/revoke [post]
func (h *Holder) RevokeCouponTemplate(c *gin.Context) {
	accountID, ok := getAccountID(c)
	if !ok {
		response.Unauthorized(c)
		return
	}
	storeID, ok := h.resolveStoreIDFor(c, accountID)
	if !ok {
		return
	}

	templateID := c.Param("template_id")
	if templateID == "" {
		response.BadRequest(c, "template_id is required")
		return
	}

	var req struct {
		Reason string `json:"reason"`
	}
	_ = c.ShouldBindJSON(&req)

	resp, err := h.Coupon.SellerRevokeTemplate(c.Request.Context(), &pb.SellerRevokeTemplateRequest{
		StoreId:    int64(storeID),
		TemplateId: templateID,
		Reason:     req.Reason,
	})
	if err != nil {
		response.GrpcErr(c, "coupon", err)
		return
	}

	response.GrpcResult(c, resp.Code, resp.Message, nil)
}

// ==================== Admin API ====================

// AdminCreateCouponTemplate 管理員建立優惠券模板
// @Summary      管理員建立優惠券模板
// @Description  管理員建立優惠券模板（status=0 草稿狀態，需另行上架）
// @Tags         優惠券 (Coupon)
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        request  body  AdminCreateCouponTemplateRequest  true  "建立模板請求"
// @Success      200  {object}  object{code=int,data=CouponTemplateResponse}
// @Failure      400  {object}  object{code=int,message=string,traceId=string}
// @Failure      403  {object}  object{code=int,message=string,traceId=string}
// @Router       /admin/coupon_templates [post]
func (h *Holder) AdminCreateCouponTemplate(c *gin.Context) {
	accountID, ok := getAccountID(c)
	if !ok {
		response.Unauthorized(c)
		return
	}

	var req struct {
		ClaimRequired     *bool    `json:"claimRequired"` // 是否需要領券（預設 true）
		Title             string   `json:"title" binding:"required"`
		Type              string   `json:"type" binding:"required"`
		TargetType        string   `json:"targetType" binding:"required"`
		ApplicableStoreId string   `json:"applicableStoreId"`
		ThresholdAmount   string   `json:"thresholdAmount"`
		DiscountAmount    string   `json:"discountAmount"`
		DiscountPercent   string   `json:"discountPercent"`
		MaxDiscountAmount string   `json:"maxDiscountAmount"`
		StartAt           string   `json:"startAt" binding:"required"`
		EndAt             string   `json:"endAt" binding:"required"`
		TotalIssueLimit   string   `json:"totalIssueLimit"`
		BuyerAccountIds   []string `json:"buyerAccountIds"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "invalid request: "+err.Error())
		return
	}

	// proto3 bool 預設為 false，但業務預設為 true（需要領券）
	// 使用指標判斷前端是否有傳值：nil=未傳（預設true）、false=明確免領券、true=需領券
	claimRequired := true
	if req.ClaimRequired != nil {
		claimRequired = *req.ClaimRequired
	}

	resolvedBuyerIDs, ok := h.resolveTargetBuyerIDs(c, req.BuyerAccountIds)
	if !ok {
		return
	}

	resp, err := h.Coupon.AdminCreateTemplate(c.Request.Context(), &pb.AdminCreateTemplateRequest{
		AccountId:         accountID,
		Title:             req.Title,
		Type:              req.Type,
		TargetType:        req.TargetType,
		ApplicableStoreId: req.ApplicableStoreId,
		ThresholdAmount:   req.ThresholdAmount,
		DiscountAmount:    req.DiscountAmount,
		DiscountPercent:   req.DiscountPercent,
		MaxDiscountAmount: req.MaxDiscountAmount,
		StartAt:           req.StartAt,
		EndAt:             req.EndAt,
		TotalIssueLimit:   req.TotalIssueLimit,
		BuyerIds:          resolvedBuyerIDs,
		ClaimRequired:     claimRequired,
	})
	if err != nil {
		response.GrpcErr(c, "coupon", err)
		return
	}

	if resp.Code != 0 {
		response.GrpcResult(c, resp.Code, resp.Message, nil)
		return
	}

	if resp.Data != nil {
		response.OK(c, templateInfoToMap(resp.Data, h.MediaBaseURL))
	} else {
		response.OK(c, nil)
	}
}

// AdminPublishCouponTemplate 管理員上架模板
// @Summary      上架優惠券模板
// @Description  管理員將 status=0(草稿) 的優惠券模板上架為 status=1(進行中)
// @Tags         優惠券 (Coupon)
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        template_id  path  string  true  "模板 ID"
// @Success      200  {object}  object{code=int,message=string}
// @Failure      403  {object}  object{code=int,message=string,traceId=string}
// @Failure      404  {object}  object{code=int,message=string,traceId=string}
// @Router       /admin/coupon_templates/{template_id}/publish [post]
func (h *Holder) AdminPublishCouponTemplate(c *gin.Context) {
	accountID, ok := getAccountID(c)
	if !ok {
		response.Unauthorized(c)
		return
	}

	templateID := c.Param("template_id")

	resp, err := h.Coupon.AdminPublishTemplate(c.Request.Context(), &pb.AdminPublishTemplateRequest{
		AccountId:  accountID,
		TemplateId: templateID,
	})
	if err != nil {
		response.GrpcErr(c, "coupon", err)
		return
	}

	response.GrpcResult(c, resp.Code, resp.Message, nil)
}

// AdminDisableCouponTemplate 管理員停用模板
// @Summary      停用優惠券模板
// @Description  管理員停用 status=1(進行中) 的優惠券模板
// @Tags         優惠券 (Coupon)
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        template_id  path  string  true  "模板 ID"
// @Success      200  {object}  object{code=int,message=string}
// @Failure      403  {object}  object{code=int,message=string,traceId=string}
// @Router       /admin/coupon_templates/{template_id}/disable [post]
func (h *Holder) AdminDisableCouponTemplate(c *gin.Context) {
	accountID, ok := getAccountID(c)
	if !ok {
		response.Unauthorized(c)
		return
	}

	templateID := c.Param("template_id")

	resp, err := h.Coupon.AdminDisableTemplate(c.Request.Context(), &pb.AdminDisableTemplateRequest{
		AccountId:  accountID,
		TemplateId: templateID,
	})
	if err != nil {
		response.GrpcErr(c, "coupon", err)
		return
	}

	response.GrpcResult(c, resp.Code, resp.Message, nil)
}

// AdminIssueCouponsHandler 管理員發行優惠券
// @Summary      管理員發行優惠券
// @Description  管理員對指定模板發行優惠券給指定買家列表
// @Tags         優惠券 (Coupon)
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        template_id  path  string  true  "模板 ID"
// @Param        request      body  AdminIssueCouponsRequest  true  "發行對象"
// @Success      200  {object}  object{code=int,data=AdminIssueCouponsResponse}
// @Failure      400  {object}  object{code=int,message=string,traceId=string}
// @Router       /admin/coupon_templates/{template_id}/issue [post]
func (h *Holder) AdminIssueCouponsHandler(c *gin.Context) {
	accountID, ok := getAccountID(c)
	if !ok {
		response.Unauthorized(c)
		return
	}

	templateID := c.Param("template_id")

	var req struct {
		BuyerAccountIds     []string `json:"buyerAccountIds" binding:"required"`
		ImmediateDistribute bool     `json:"immediateDistribute"` // 是否立即代領（預設 false）
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "invalid request: "+err.Error())
		return
	}

	resolvedBuyerIDs, ok := h.resolveTargetBuyerIDs(c, req.BuyerAccountIds)
	if !ok {
		return
	}

	resp, err := h.Coupon.AdminIssueCoupons(c.Request.Context(), &pb.AdminIssueCouponsRequest{
		AccountId:           accountID,
		TemplateId:          templateID,
		BuyerIds:            resolvedBuyerIDs,
		ImmediateDistribute: req.ImmediateDistribute,
	})
	if err != nil {
		response.GrpcErr(c, "coupon", err)
		return
	}

	if resp.Code != 0 {
		response.GrpcResult(c, resp.Code, resp.Message, nil)
		return
	}

	data := gin.H{"issuedCount": resp.IssuedCount}
	if req.ImmediateDistribute {
		data["distributedCount"] = resp.DistributedCount
	}
	response.OK(c, data)
}

// AdminDistributeTemplate 管理員代領模板
// @Summary      管理員代領模板
// @Description  管理員對指定模板執行代領，為所有已發行但未領取的目標帳號自動建立券實例
// @Tags         優惠券 (Coupon)
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        template_id  path  string  true  "模板 ID"
// @Success      200  {object}  object{code=int,data=AdminDistributeTemplateResponse}
// @Failure      400  {object}  object{code=int,message=string,traceId=string}
// @Failure      403  {object}  object{code=int,message=string,traceId=string}
// @Router       /admin/coupon_templates/{template_id}/distribute [post]
func (h *Holder) AdminDistributeTemplate(c *gin.Context) {
	accountID, ok := getAccountID(c)
	if !ok {
		response.Unauthorized(c)
		return
	}

	templateID := c.Param("template_id")

	resp, err := h.Coupon.AdminDistributeTemplate(c.Request.Context(), &pb.AdminDistributeTemplateRequest{
		AccountId:  accountID,
		TemplateId: templateID,
	})
	if err != nil {
		response.GrpcErr(c, "coupon", err)
		return
	}

	if resp.Code != 0 {
		response.GrpcResult(c, resp.Code, resp.Message, nil)
		return
	}

	response.OK(c, gin.H{"distributedCount": resp.DistributedCount})
}

// AdminGrantCompensation 管理員發放補償券
// @Summary      發放補償券
// @Description  管理員針對退款/糾紛發放補償優惠券給買家
// @Tags         優惠券 (Coupon)
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        request  body  AdminGrantCompensationRequest  true  "補償券請求"
// @Success      200  {object}  object{code=int,data=CouponInstanceResponse}
// @Failure      400  {object}  object{code=int,message=string,traceId=string}
// @Router       /admin/coupons/compensations [post]
func (h *Holder) AdminGrantCompensation(c *gin.Context) {
	accountID, ok := getAccountID(c)
	if !ok {
		response.Unauthorized(c)
		return
	}

	var req struct {
		BuyerAccountId string `json:"buyerAccountId" binding:"required"`
		Amount         string `json:"amount" binding:"required"`
		RefOrderNo     string `json:"refOrderNo"`
		// 必填，語意見 AdminGrantCompensationRequest。
		ApplicableStoreId string `json:"applicableStoreId" binding:"required"`
		Reason            string `json:"reason" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "invalid request: "+err.Error())
		return
	}

	buyerID, ok := h.resolveTargetBuyerID(c, req.BuyerAccountId)
	if !ok {
		return
	}

	resp, err := h.Coupon.AdminGrantCompensationCoupon(c.Request.Context(), &pb.AdminGrantCompensationCouponRequest{
		AccountId:         accountID,
		BuyerId:           strconv.FormatInt(buyerID, 10),
		Amount:            req.Amount,
		RefOrderNo:        req.RefOrderNo,
		ApplicableStoreId: req.ApplicableStoreId,
		Reason:            req.Reason,
	})
	if err != nil {
		response.GrpcErr(c, "coupon", err)
		return
	}

	if resp.Code != 0 {
		response.GrpcResult(c, resp.Code, resp.Message, nil)
		return
	}

	if resp.Data != nil {
		response.OK(c, templateInfoToMap(resp.Data, h.MediaBaseURL))
	} else {
		response.OK(c, nil)
	}
}

// AdminVoidCouponInstance 管理員作廢優惠券
// @Summary      作廢優惠券
// @Description  管理員作廢指定的優惠券實例（status=0(未使用)/1(已鎖定) 狀態）
// @Tags         優惠券 (Coupon)
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        instance_id  path  string  true  "券實例 ID"
// @Param        request      body  AdminVoidCouponRequest  true  "作廢原因"
// @Success      200  {object}  object{code=int,message=string}
// @Failure      400  {object}  object{code=int,message=string,traceId=string}
// @Router       /admin/coupons/{instance_id}/void [post]
func (h *Holder) AdminVoidCouponInstance(c *gin.Context) {
	accountID, ok := getAccountID(c)
	if !ok {
		response.Unauthorized(c)
		return
	}

	instanceID := c.Param("instance_id")

	var req struct {
		Reason string `json:"reason" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "invalid request: "+err.Error())
		return
	}

	resp, err := h.Coupon.AdminVoidInstance(c.Request.Context(), &pb.AdminVoidInstanceRequest{
		AccountId:  accountID,
		InstanceId: instanceID,
		Reason:     req.Reason,
	})
	if err != nil {
		response.GrpcErr(c, "coupon", err)
		return
	}

	response.GrpcResult(c, resp.Code, resp.Message, nil)
}

// AdminUpdateCouponQuota 管理員更新賣家配額
// @Summary      更新賣家優惠券配額
// @Description  管理員更新指定賣家的優惠券模板配額上限
// @Tags         優惠券 (Coupon)
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        store_id           path  string  true  "店家 ID (store_id)"
// @Param        request            body  AdminUpdateCouponQuotaRequest  true  "配額設定"
// @Success      200  {object}  object{code=int,message=string}
// @Failure      400  {object}  object{code=int,message=string,traceId=string}
// @Router       /admin/coupon_quotas/{store_id} [put]
func (h *Holder) AdminUpdateCouponQuota(c *gin.Context) {
	accountID, ok := getAccountID(c)
	if !ok {
		response.Unauthorized(c)
		return
	}

	storeID := c.Param("store_id")

	var req struct {
		MaxTemplateCount string `json:"maxTemplateCount" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "invalid request: "+err.Error())
		return
	}

	resp, err := h.Coupon.AdminUpdateSellerQuota(c.Request.Context(), &pb.AdminUpdateSellerQuotaRequest{
		AccountId:        accountID,
		StoreId:          storeID,
		MaxTemplateCount: req.MaxTemplateCount,
	})
	if err != nil {
		response.GrpcErr(c, "coupon", err)
		return
	}

	response.GrpcResult(c, resp.Code, resp.Message, nil)
}

// ==================== 轉換工具 ====================

func templateInfoToMap(tpl *pb.TemplateInfo, mediaBase string) gin.H {
	if tpl == nil {
		return gin.H{}
	}
	result := gin.H{
		"templateId":           tpl.TemplateId,
		"title":                tpl.Title,
		"type":                 tpl.Type,
		"targetType":           tpl.TargetType,
		"applicableStoreId":    tpl.ApplicableStoreId,
		"sellerName":           tpl.SellerName,
		"storeCode":            tpl.StoreCode,
		"logoKey":              tpl.SellerAvatarKey,                      // 商家 logo objectKey（SSOT；49 §6.1）
		"logoUrl":              buildURL(mediaBase, tpl.SellerAvatarKey), // 顯示 URL（Gateway 組）
		"issuerType":           tpl.IssuerType,
		"issuerAccountId":      tpl.IssuerAccountId,
		"issuerStoreId":        tpl.IssuerStoreId,
		"thresholdAmount":      tpl.ThresholdAmount,
		"discountAmount":       tpl.DiscountAmount,
		"discountPercent":      tpl.DiscountPercent,
		"maxDiscountAmount":    tpl.MaxDiscountAmount,
		"startAt":              tpl.StartAt,
		"endAt":                tpl.EndAt,
		"usageLimitPerAccount": tpl.UsageLimitPerAccount,
		"totalIssueLimit":      tpl.TotalIssueLimit,
		"issuedCount":          tpl.IssuedCount,
		"status":               tpl.Status,
		"createdAt":            tpl.CreatedAt,
		"claimRequired":        tpl.ClaimRequired,
		"claimed":              tpl.Claimed,
	}
	if len(tpl.TargetAccounts) > 0 {
		accounts := make([]gin.H, 0, len(tpl.TargetAccounts))
		for _, ta := range tpl.TargetAccounts {
			accounts = append(accounts, gin.H{
				"buyerId":  ta.BuyerId,
				"nickname": ta.Nickname,
				"phone":    ta.Phone,
			})
		}
		result["targetAccounts"] = accounts
	}
	return result
}

func instanceInfoToMap(inst *pb.InstanceInfo, mediaBase string) gin.H {
	if inst == nil {
		return gin.H{}
	}
	return gin.H{
		"instanceId":        inst.InstanceId,
		"templateId":        inst.TemplateId,
		"title":             inst.Title,
		"type":              inst.Type,
		"applicableStoreId": inst.ApplicableStoreId,
		"sellerName":        inst.SellerName,
		"storeCode":         inst.StoreCode,
		"logoKey":           inst.SellerAvatarKey,                      // 商家 logo objectKey（SSOT；49 §6.1）
		"logoUrl":           buildURL(mediaBase, inst.SellerAvatarKey), // 顯示 URL（Gateway 組）
		"thresholdAmount":   inst.ThresholdAmount,
		"discountAmount":    inst.DiscountAmount,
		"discountPercent":   inst.DiscountPercent,
		"maxDiscountAmount": inst.MaxDiscountAmount,
		"startAt":           inst.StartAt,
		"endAt":             inst.EndAt,
		"status":            inst.Status,
		"createdAt":         inst.CreatedAt,
	}
}
