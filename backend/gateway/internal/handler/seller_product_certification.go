package handler

import (
	"github.com/Will413028/hawkr/gateway/internal/response"
	pb "github.com/Will413028/hawkr/go-pkg/proto/product"
	"github.com/gin-gonic/gin"
)

// 賣家端的商品認證申請（19 號設計書 §9.1）。
//
// 🔴 為什麼是獨立檔案而不是併進 seller_product.go：scripts/seller-identity-ratchet.sh
// 對 seller_product.go 的 `getAccountID(c)` 出現次數只准下降，把這三支寫進那個檔會讓
// 計數上升、CI 直接紅。賣家歸屬鍵一律走 resolveStoreIDFor 解析出來的 store surrogate，
// account_id 只作為登入 principal 傳給下游（product.proto §10.3 例外第 1 條）。
//
// 🔴 賣家 DTO 不含 internalNote、不含 govVerifyPayload 原文、不含 submittedByAccountId：
// 那是 ADM 內部註記與政府原始資料，隔離靠**兩個型別**而不是執行期清空欄位——後者會讓
// swagger 宣告與實際輸出對不上，而且下一個複製這段 code 的人不會知道要清。

// SellerProductCertificationApplicationItem 是賣家看到的一筆認證申請。
//
// 時間與原因欄位用 *string：19 §9.1 的回應範例明寫 "reasonCode": null，而空字串與 null
// 在前端是兩件事（前者會被渲染成一個空白的原因欄）。
type SellerProductCertificationApplicationItem struct {
	ID                string  `json:"id" example:"843856384063471760"`
	ProductID         string  `json:"productId" example:"843856384063471755"`
	CertificationType string  `json:"certificationType" example:"3701"` // CertificationType value（3701–3706）
	CertificateNo     string  `json:"certificateNo" example:"1-009-300022"`
	Status            string  `json:"status" example:"APPROVED"`         // ProductCertificationStatus
	GovVerifyState    string  `json:"govVerifyState" example:"VERIFIED"` // ProductCertificationGovVerifyState
	GovVerifiedAt     *string `json:"govVerifiedAt" example:"2026-08-26T04:12:07Z"`
	SubmittedAt       *string `json:"submittedAt" example:"2026-08-26T04:12:05Z"`
	ReviewedAt        *string `json:"reviewedAt"`
	ReasonCode        *string `json:"reasonCode"`   // ProductCertificationRejectReason
	PublicReason      *string `json:"publicReason"` // 可直接展示給賣家的駁回原因
	Version           string  `json:"version" example:"1"`
}

// toSellerCertificationApplication 把 proto 投影成賣家 DTO（Admin-only 欄位一律不映射）。
func toSellerCertificationApplication(app *pb.ProductCertificationApplication) SellerProductCertificationApplicationItem {
	if app == nil {
		return SellerProductCertificationApplicationItem{}
	}
	return SellerProductCertificationApplicationItem{
		ID:                app.GetId(),
		ProductID:         app.GetProductId(),
		CertificationType: app.GetCertificationType(),
		CertificateNo:     app.GetCertificateNo(),
		Status:            app.GetStatus(),
		GovVerifyState:    app.GetGovVerifyState(),
		GovVerifiedAt:     nilIfEmptyStr(app.GetGovVerifiedAt()),
		SubmittedAt:       nilIfEmptyStr(app.GetSubmittedAt()),
		ReviewedAt:        nilIfEmptyStr(app.GetReviewedAt()),
		ReasonCode:        nilIfEmptyStr(app.GetReasonCode()),
		PublicReason:      nilIfEmptyStr(app.GetPublicReason()),
		Version:           app.GetVersion(),
	}
}

// nilIfEmptyStr 把 proto 的 string 欄位還原成可選輸出（空字串 = 未設定 = JSON null）。
func nilIfEmptyStr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func toSellerCertificationApplications(apps []*pb.ProductCertificationApplication) []SellerProductCertificationApplicationItem {
	out := make([]SellerProductCertificationApplicationItem, 0, len(apps))
	for _, app := range apps {
		if app != nil {
			out = append(out, toSellerCertificationApplication(app))
		}
	}
	return out
}

// SubmitProductCertificationRequest 是賣家送出單筆認證申請的請求體。
//
// ⚠️ certificationType / certificateNo 刻意**不掛值域 binding tag**：本 repo 的 binding
// 失敗只有一條出口（400），而 19 §6.2.2 明令非法認證輸入不得回 4xx。這一支是賣家主動
// 送出的單筆申請，值域由 product-service 的收斂點權威判定並以業務碼回覆。
type SubmitProductCertificationRequest struct {
	CertificationType string `json:"certificationType" binding:"required" example:"3701"`
	CertificateNo     string `json:"certificateNo" binding:"required" example:"1-009-300022"`
	IdempotencyKey    string `json:"idempotencyKey" binding:"required" example:"cert-2f0c1f2e"`
}

// WithdrawProductCertificationRequest 是賣家撤回一筆待審申請的請求體。
type WithdrawProductCertificationRequest struct {
	ExpectedVersion string `json:"expectedVersion" binding:"required" example:"2"` // 樂觀鎖，數字字串
	IdempotencyKey  string `json:"idempotencyKey" binding:"required" example:"cert-wd-2f0c1f2e"`
}

// sellerCertificationIdentity 解析出「登入 principal ＋ 當前 store surrogate」。
// 兩者分開：account_id 是 accounts PK，store_id 才是賣家的營運鍵（44 號設計書）。
func (h *Holder) sellerCertificationIdentity(c *gin.Context) (accountID int64, storeID int64, ok bool) {
	accountID, ok = getAccountID(c)
	if !ok {
		response.Unauthorized(c)
		return 0, 0, false
	}
	store, ok := h.resolveStoreIDFor(c, accountID)
	if !ok {
		return 0, 0, false
	}
	return accountID, int64(store), true
}

// sellerProductIDParam 取路由上的商品 ID（app 路由用 :id）。
func sellerProductIDParam(c *gin.Context) (string, bool) {
	productID := c.Param("product_id")
	if productID == "" {
		productID = c.Param("id")
	}
	if productID == "" {
		response.BadRequest(c, "invalid product id")
		return "", false
	}
	return productID, true
}

// ListSellerProductCertificationApplications 取得商品的認證申請列表
// @Summary      取得商品認證申請列表
// @Description  列出指定商品的認證申請與審核狀態。僅限商品擁有者存取（非本人商品一律 404，避免以商品 ID 枚舉他人資料）。分頁三欄 total/page/pageSize 皆為字串。
// @Tags         商家商品 (Seller Product)
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id         path   string  true   "商品 ID（TSID 數字字串）"
// @Param        status     query  string  false  "依申請狀態篩選（ProductCertificationStatus）；省略＝不篩"
// @Param        page       query  string  false  "頁碼（數字字串，預設 1）"
// @Param        page_size  query  string  false  "每頁筆數（數字字串，預設 20）"
// @Success      200  {object}  object{code=int,message=string,data=object{records=[]SellerProductCertificationApplicationItem,total=string,page=string,pageSize=string}}
// @Failure      400  {object}  object{code=int,message=string,traceId=string}
// @Failure      401  {object}  object{code=int,message=string,traceId=string}
// @Failure      403  {object}  object{code=int,message=string,traceId=string}
// @Failure      404  {object}  object{code=int,message=string,traceId=string}
// @Router       /app/products/{id}/certification_applications [get]
func (h *Holder) ListSellerProductCertificationApplications(c *gin.Context) {
	accountID, storeID, ok := h.sellerCertificationIdentity(c)
	if !ok {
		return
	}
	productID, ok := sellerProductIDParam(c)
	if !ok {
		return
	}

	resp, err := h.Product.ListProductCertificationApplications(c.Request.Context(), &pb.ListProductCertificationApplicationsRequest{
		ProductId: productID,
		Status:    c.Query("status"),
		Page:      c.DefaultQuery("page", "1"),
		PageSize:  c.DefaultQuery("page_size", "20"),
		AccountId: accountID,
		StoreId:   storeID,
	})
	if err != nil {
		response.GrpcErr(c, "product", err)
		return
	}
	if resp.GetCode() != 0 {
		// BizStatus 而不是 GrpcResult：3025（不存在／非本人商品）必須是 404，靠 200＋碼
		// 會讓前端把「查不到」畫成一個空列表。未登錄的碼仍塌縮成 409，與既有行為一致。
		//
		// ⚠️ 與 admin_product_certification.go 同註：EnvelopeGuardInterceptor 讓業務碼
		// 從 err 那側回來，這條分支只有 envelopeCodePreserveData 的碼走得到（#2755）。
		response.BizStatus(c, resp.GetCode(), resp.GetMessage())
		return
	}

	response.Page(c,
		toSellerCertificationApplications(resp.GetRecords()),
		resp.GetTotal(), resp.GetPage(), resp.GetPageSize())
}

// SubmitSellerProductCertification 送出商品認證申請
// @Summary      送出商品認證申請
// @Description  為指定商品送出一筆農業認證申請。同步快取命中「有效」且該類型允許自動核准時會直接核准，否則落審核中並排背景查驗。重送同一組 (類型, 編號) 為冪等 no-op；同類型已有待審且編號不同時回 3029。
// @Tags         商家商品 (Seller Product)
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id       path  string                             true  "商品 ID（TSID 數字字串）"
// @Param        request  body  SubmitProductCertificationRequest  true  "認證申請請求"
// @Success      200  {object}  object{code=int,message=string,data=SellerProductCertificationApplicationItem}
// @Failure      400  {object}  object{code=int,message=string,traceId=string}
// @Failure      401  {object}  object{code=int,message=string,traceId=string}
// @Failure      403  {object}  object{code=int,message=string,traceId=string}
// @Failure      404  {object}  object{code=int,message=string,traceId=string}
// @Failure      409  {object}  object{code=int,message=string,traceId=string}
// @Router       /app/products/{id}/certification_applications [post]
func (h *Holder) SubmitSellerProductCertification(c *gin.Context) {
	accountID, storeID, ok := h.sellerCertificationIdentity(c)
	if !ok {
		return
	}
	productID, ok := sellerProductIDParam(c)
	if !ok {
		return
	}

	var req SubmitProductCertificationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "invalid request: "+err.Error())
		return
	}

	resp, err := h.Product.SubmitProductCertification(c.Request.Context(), &pb.SubmitProductCertificationRequest{
		ProductId:         productID,
		CertificationType: req.CertificationType,
		CertificateNo:     req.CertificateNo,
		IdempotencyKey:    req.IdempotencyKey,
		AccountId:         accountID,
		StoreId:           storeID,
	})
	if err != nil {
		response.GrpcErr(c, "product", err)
		return
	}
	if resp.GetCode() != 0 {
		response.BizStatus(c, resp.GetCode(), resp.GetMessage())
		return
	}

	response.OK(c, toSellerCertificationApplication(resp.GetData()))
}

// WithdrawSellerProductCertification 撤回商品認證申請
// @Summary      撤回商品認證申請
// @Description  撤回一筆尚未裁決的認證申請。expectedVersion 不符或該筆已被裁決回 3026；狀態不允許撤回回 3027。
// @Tags         商家商品 (Seller Product)
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id              path  string                               true  "商品 ID（TSID 數字字串）"
// @Param        application_id  path  string                               true  "認證申請 ID（TSID 數字字串）"
// @Param        request         body  WithdrawProductCertificationRequest  true  "撤回請求"
// @Success      200  {object}  object{code=int,message=string,data=SellerProductCertificationApplicationItem}
// @Failure      400  {object}  object{code=int,message=string,traceId=string}
// @Failure      401  {object}  object{code=int,message=string,traceId=string}
// @Failure      403  {object}  object{code=int,message=string,traceId=string}
// @Failure      404  {object}  object{code=int,message=string,traceId=string}
// @Failure      409  {object}  object{code=int,message=string,traceId=string}
// @Router       /app/products/{id}/certification_applications/{application_id}/withdraw [post]
func (h *Holder) WithdrawSellerProductCertification(c *gin.Context) {
	accountID, storeID, ok := h.sellerCertificationIdentity(c)
	if !ok {
		return
	}
	productID, ok := sellerProductIDParam(c)
	if !ok {
		return
	}
	applicationID := c.Param("application_id")
	if applicationID == "" {
		response.BadRequest(c, "invalid application id")
		return
	}

	var req WithdrawProductCertificationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "invalid request: "+err.Error())
		return
	}

	resp, err := h.Product.WithdrawProductCertification(c.Request.Context(), &pb.WithdrawProductCertificationRequest{
		ProductId:       productID,
		ApplicationId:   applicationID,
		ExpectedVersion: req.ExpectedVersion,
		IdempotencyKey:  req.IdempotencyKey,
		AccountId:       accountID,
		StoreId:         storeID,
	})
	if err != nil {
		response.GrpcErr(c, "product", err)
		return
	}
	if resp.GetCode() != 0 {
		response.BizStatus(c, resp.GetCode(), resp.GetMessage())
		return
	}

	response.OK(c, toSellerCertificationApplication(resp.GetData()))
}
