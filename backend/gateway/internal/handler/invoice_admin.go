package handler

import (
	"net/url"

	"github.com/Will413028/hawkr/gateway/internal/middleware"
	"github.com/Will413028/hawkr/gateway/internal/response"
	pb "github.com/Will413028/hawkr/go-pkg/proto/invoice"
	"github.com/gin-gonic/gin"
)

// 58 號《電子發票服務》ADM 面 handler（§6.1.11a–§6.1.17）。
//
// 身分：getAccountID 取到的 account_id 僅作為稽核 actor 往下傳（§6.1.0(1)），不是
// 授權依據——授權由 admin group 上的 requireAdmin 承擔。

// GetInvoiceOverviewForAdmin ADM 發票營運總覽
// @Summary      ADM 發票營運總覽
// @Description  ADM 發票營運總覽（58 號設計書 §6.1 端點 37）
// @Tags         管理員 (Admin)
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Success      200  {object}  object{code=int,message=string,data=GetInvoiceOverviewForAdminData}
// @Failure      401  {object}  object{code=int,message=string}  "未登入或 JWT 無效"
// @Failure      403  {object}  object{code=int,message=string}  "非 admin 身分"
// @Failure      429  {object}  object{code=int,message=string}  "請求過於頻繁"
// @Failure      503  {object}  object{code=int,message=string}  "invoice-service 不可用或逾時"
// @Router       /admin/invoices/overview [get]
func (h *Holder) GetInvoiceOverviewForAdmin(c *gin.Context) {
	accountID, ok := getAccountID(c)
	if !ok {
		response.Unauthorized(c)
		return
	}

	resp, err := h.Invoice.GetInvoiceOverviewForAdmin(c.Request.Context(), &pb.GetInvoiceOverviewForAdminRequest{
		ActorAccountId: accountID,
	})
	if err != nil {
		response.GrpcErr(c, "invoice", err)
		return
	}

	data := convInvGetInvoiceOverviewForAdminData(resp.GetData())
	// preserveData 業務碼不會轉成 err（interceptor 對它們 return nil），必須自己看 code。
	if resp.GetCode() != 0 {
		response.Biz(c, resp.GetCode(), resp.GetMessage(), data)
		return
	}
	response.OK(c, data)
}

// ListTaxReviewsForAdmin ADM 稅務身分審核佇列
// @Summary      ADM 稅務身分審核佇列
// @Description  ADM 稅務身分審核佇列（58 號設計書 §6.1 端點 38）
// @Tags         管理員 (Admin)
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        filter  query  string  false  "審核佇列篩選 NEW/SUPPLEMENT_REQUIRED/UNDER_REVIEW/NEAR_DEADLINE/EXTENDED/EXPIRED"
// @Param        keyword  query  string  false  "關鍵字"
// @Param        page  query  string  false  "頁碼，數字字串 ≥ 1，預設 1"
// @Param        page_size  query  string  false  "每頁筆數，數字字串 1–50，預設 20（逾越一律 400，不夾住）"
// @Success      200  {object}  object{code=int,message=string,data=object{records=[]TaxReviewItem,total=string,page=string,pageSize=string}}
// @Failure      400  {object}  object{code=int,message=string}
// @Failure      401  {object}  object{code=int,message=string}  "未登入或 JWT 無效"
// @Failure      403  {object}  object{code=int,message=string}  "非 admin 身分"
// @Failure      429  {object}  object{code=int,message=string}  "請求過於頻繁"
// @Failure      503  {object}  object{code=int,message=string}  "invoice-service 不可用或逾時"
// @Router       /admin/invoices/tax_reviews [get]
func (h *Holder) ListTaxReviewsForAdmin(c *gin.Context) {
	accountID, ok := getAccountID(c)
	if !ok {
		response.Unauthorized(c)
		return
	}

	page, pageSize, ok := invoicePaging(c, invoiceAdminPageSizeStr)
	if !ok {
		return
	}

	resp, err := h.Invoice.ListTaxReviewsForAdmin(c.Request.Context(), &pb.ListTaxReviewsForAdminRequest{
		ActorAccountId: accountID,
		Filter:         c.Query("filter"),
		Keyword:        c.Query("keyword"),
		Page:           page,
		PageSize:       pageSize,
	})
	if err != nil {
		response.GrpcErr(c, "invoice", err)
		return
	}

	data := resp.GetData()
	records := make([]TaxReviewItem, 0, len(data.GetRecords()))
	for _, it := range data.GetRecords() {
		records = append(records, convInvTaxReviewItem(it))
	}
	// preserveData 業務碼不會轉成 err（interceptor 對它們 return nil），必須自己看 code。
	if resp.GetCode() != 0 {
		response.Biz(c, resp.GetCode(), resp.GetMessage(), response.PageData{
			Records:  records,
			Total:    data.GetTotal(),
			Page:     data.GetPage(),
			PageSize: data.GetPageSize(),
		})
		return
	}
	response.Page(c, records, data.GetTotal(), data.GetPage(), data.GetPageSize())
}

// GetTaxReviewForAdmin ADM 稅務身分審核詳情
// @Summary      ADM 稅務身分審核詳情
// @Description  ADM 稅務身分審核詳情（58 號設計書 §6.1 端點 39）
// @Tags         管理員 (Admin)
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        store_id  path  string  true  "賣家 store_id"
// @Success      200  {object}  object{code=int,message=string,data=SellerTaxProfileData}
// @Failure      401  {object}  object{code=int,message=string}  "未登入或 JWT 無效"
// @Failure      403  {object}  object{code=int,message=string}  "非 admin 身分"
// @Failure      404  {object}  object{code=int,message=string}  "9320"
// @Failure      429  {object}  object{code=int,message=string}  "請求過於頻繁"
// @Failure      503  {object}  object{code=int,message=string}  "invoice-service 不可用或逾時"
// @Router       /admin/invoices/tax_reviews/{store_id} [get]
func (h *Holder) GetTaxReviewForAdmin(c *gin.Context) {
	accountID, ok := getAccountID(c)
	if !ok {
		response.Unauthorized(c)
		return
	}

	storeId := c.Param("store_id")
	if storeId == "" {
		response.BadRequest(c, "store_id is required")
		return
	}

	resp, err := h.Invoice.GetTaxReviewForAdmin(c.Request.Context(), &pb.GetTaxReviewForAdminRequest{
		ActorAccountId: accountID,
		StoreId:        storeId,
	})
	if err != nil {
		response.GrpcErr(c, "invoice", err)
		return
	}

	data := convInvSellerTaxProfileData(resp.GetData())
	fillAdminTaxDocumentPaths(&data, storeId)
	// preserveData 業務碼不會轉成 err（interceptor 對它們 return nil），必須自己看 code。
	if resp.GetCode() != 0 {
		response.Biz(c, resp.GetCode(), resp.GetMessage(), data)
		return
	}
	response.OK(c, data)
}

// adminTaxDocumentContentPath 是 ADM 稅籍佐證下載的對外路徑（invoice-service 持有授權）。
// 與 router.AdminInvoiceTaxDocumentContent 同一條；handler 不能 import router（會循環），
// router 的 TestAdminTaxDocumentPathMatchesHandlerLiteral 釘住兩者一致。
func adminTaxDocumentContentPath(storeID, documentID string) string {
	return "/api/v1/admin/invoices/tax_reviews/" + url.PathEscape(storeID) +
		"/documents/" + url.PathEscape(documentID) + "/content"
}

// fillAdminTaxDocumentPaths 只給 ADM 回應填 fileDownloadPath；賣家回應不經過這裡。
func fillAdminTaxDocumentPaths(d *SellerTaxProfileData, storeID string) {
	for i := range d.Documents {
		if d.Documents[i].DocumentId != "" {
			d.Documents[i].FileDownloadPath = adminTaxDocumentContentPath(storeID, d.Documents[i].DocumentId)
		}
	}
}

// DecideTaxReviewForAdmin ADM 稅務身分審核裁決
// @Summary      ADM 稅務身分審核裁決
// @Description  ADM 稅務身分審核裁決（58 號設計書 §6.1 端點 40）
// @Tags         管理員 (Admin)
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        Idempotency-Key  header  string  true  "冪等鍵，^[A-Za-z0-9._:-]{8,128}$"
// @Param        store_id  path  string  true  "賣家 store_id"
// @Param        request  body  DecideTaxReviewForAdminRequest  true  "ADM 稅務身分審核裁決"
// @Success      200  {object}  object{code=int,message=string,data=DecideTaxReviewForAdminData}
// @Failure      400  {object}  object{code=int,message=string}  "含缺 confirmed"
// @Failure      401  {object}  object{code=int,message=string}  "未登入或 JWT 無效"
// @Failure      403  {object}  object{code=int,message=string}  "非 admin 身分"
// @Failure      404  {object}  object{code=int,message=string}  "9320"
// @Failure      409  {object}  object{code=int,message=string}  "9322 / 9348 決策與狀態不相容 / 9353 核定應開發票但缺統編 / 9354 核定免用發票但賣家尚未授權代製收據 / 9350 同鍵不同內容 / 9351 同鍵處理中"
// @Failure      429  {object}  object{code=int,message=string}  "請求過於頻繁"
// @Failure      503  {object}  object{code=int,message=string}  "invoice-service 不可用或逾時"
// @Router       /admin/invoices/tax_reviews/{store_id}/decisions [post]
func (h *Holder) DecideTaxReviewForAdmin(c *gin.Context) {
	accountID, ok := getAccountID(c)
	if !ok {
		response.Unauthorized(c)
		return
	}

	storeId := c.Param("store_id")
	if storeId == "" {
		response.BadRequest(c, "store_id is required")
		return
	}

	var body DecideTaxReviewForAdminRequest
	if err := c.ShouldBindJSON(&body); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	resp, err := h.Invoice.DecideTaxReviewForAdmin(c.Request.Context(), &pb.DecideTaxReviewForAdminRequest{
		ActorAccountId:       accountID,
		StoreId:              storeId,
		Decision:             body.Decision,
		Reason:               body.Reason,
		ApprovalValidUntil:   body.ApprovalValidUntil,
		Confirmed:            body.Confirmed,
		ExpectedVersion:      body.ExpectedVersion,
		ApprovalBasisKey:     body.ApprovalBasisKey,
		ApprovalBasisAssetId: body.ApprovalBasisAssetId,
		IdempotencyKey:       c.GetHeader(middleware.IdempotencyKeyHeader),
	})
	if err != nil {
		response.GrpcErr(c, "invoice", err)
		return
	}

	data := convInvDecideTaxReviewForAdminData(resp.GetData())
	// preserveData 業務碼不會轉成 err（interceptor 對它們 return nil），必須自己看 code。
	if resp.GetCode() != 0 {
		response.Biz(c, resp.GetCode(), resp.GetMessage(), data)
		return
	}
	response.OK(c, data)
}

// DecideTaxExtensionForAdmin ADM 稅務補件延長裁決
// @Summary      ADM 稅務補件延長裁決
// @Description  ADM 稅務補件延長裁決（58 號設計書 §6.1 端點 41）
// @Tags         管理員 (Admin)
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        Idempotency-Key  header  string  true  "冪等鍵，^[A-Za-z0-9._:-]{8,128}$"
// @Param        store_id  path  string  true  "賣家 store_id"
// @Param        request  body  DecideTaxExtensionForAdminRequest  true  "ADM 稅務補件延長裁決"
// @Success      200  {object}  object{code=int,message=string,data=DecideTaxReviewForAdminData}
// @Failure      400  {object}  object{code=int,message=string}
// @Failure      401  {object}  object{code=int,message=string}  "未登入或 JWT 無效"
// @Failure      403  {object}  object{code=int,message=string}  "非 admin 身分"
// @Failure      404  {object}  object{code=int,message=string}  "9320"
// @Failure      409  {object}  object{code=int,message=string}  "9322 / 9323 / 9350 同鍵不同內容 / 9351 同鍵處理中"
// @Failure      429  {object}  object{code=int,message=string}  "請求過於頻繁"
// @Failure      503  {object}  object{code=int,message=string}  "invoice-service 不可用或逾時"
// @Router       /admin/invoices/tax_reviews/{store_id}/extensions [post]
func (h *Holder) DecideTaxExtensionForAdmin(c *gin.Context) {
	accountID, ok := getAccountID(c)
	if !ok {
		response.Unauthorized(c)
		return
	}

	storeId := c.Param("store_id")
	if storeId == "" {
		response.BadRequest(c, "store_id is required")
		return
	}

	var body DecideTaxExtensionForAdminRequest
	if err := c.ShouldBindJSON(&body); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	resp, err := h.Invoice.DecideTaxExtensionForAdmin(c.Request.Context(), &pb.DecideTaxExtensionForAdminRequest{
		ActorAccountId:  accountID,
		StoreId:         storeId,
		Decision:        body.Decision,
		ExtendDays:      body.ExtendDays,
		Reason:          body.Reason,
		ExpectedVersion: body.ExpectedVersion,
		IdempotencyKey:  c.GetHeader(middleware.IdempotencyKeyHeader),
	})
	if err != nil {
		response.GrpcErr(c, "invoice", err)
		return
	}

	data := convInvDecideTaxReviewForAdminData(resp.GetData())
	// preserveData 業務碼不會轉成 err（interceptor 對它們 return nil），必須自己看 code。
	if resp.GetCode() != 0 {
		response.Biz(c, resp.GetCode(), resp.GetMessage(), data)
		return
	}
	response.OK(c, data)
}

// ListProviderAccountsForAdmin ADM AMEGO 帳號列表
// @Summary      ADM AMEGO 帳號列表
// @Description  ADM AMEGO 帳號列表（58 號設計書 §6.1 端點 42）
// @Tags         管理員 (Admin)
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        status  query  string  false  "狀態篩選"
// @Param        keyword  query  string  false  "關鍵字"
// @Param        page  query  string  false  "頁碼，數字字串 ≥ 1，預設 1"
// @Param        page_size  query  string  false  "每頁筆數，數字字串 1–50，預設 10（逾越一律 400，不夾住）"
// @Success      200  {object}  object{code=int,message=string,data=object{records=[]ProviderAccountItem,total=string,page=string,pageSize=string}}
// @Failure      400  {object}  object{code=int,message=string}
// @Failure      401  {object}  object{code=int,message=string}  "未登入或 JWT 無效"
// @Failure      403  {object}  object{code=int,message=string}  "非 admin 身分"
// @Failure      429  {object}  object{code=int,message=string}  "請求過於頻繁"
// @Failure      503  {object}  object{code=int,message=string}  "invoice-service 不可用或逾時"
// @Router       /admin/invoices/provider_accounts [get]
func (h *Holder) ListProviderAccountsForAdmin(c *gin.Context) {
	accountID, ok := getAccountID(c)
	if !ok {
		response.Unauthorized(c)
		return
	}

	page, pageSize, ok := invoicePaging(c, defaultPageSizeStr)
	if !ok {
		return
	}

	resp, err := h.Invoice.ListProviderAccountsForAdmin(c.Request.Context(), &pb.ListProviderAccountsForAdminRequest{
		ActorAccountId: accountID,
		Status:         c.Query("status"),
		Keyword:        c.Query("keyword"),
		Page:           page,
		PageSize:       pageSize,
	})
	if err != nil {
		response.GrpcErr(c, "invoice", err)
		return
	}

	data := resp.GetData()
	records := make([]ProviderAccountItem, 0, len(data.GetRecords()))
	for _, it := range data.GetRecords() {
		records = append(records, convInvProviderAccountItem(it))
	}
	// preserveData 業務碼不會轉成 err（interceptor 對它們 return nil），必須自己看 code。
	if resp.GetCode() != 0 {
		response.Biz(c, resp.GetCode(), resp.GetMessage(), response.PageData{
			Records:  records,
			Total:    data.GetTotal(),
			Page:     data.GetPage(),
			PageSize: data.GetPageSize(),
		})
		return
	}
	response.Page(c, records, data.GetTotal(), data.GetPage(), data.GetPageSize())
}

// GetProviderAccountForAdmin ADM AMEGO 帳號詳情
// @Summary      ADM AMEGO 帳號詳情
// @Description  ADM AMEGO 帳號詳情（58 號設計書 §6.1 端點 43）
// @Tags         管理員 (Admin)
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        store_id  path  string  true  "賣家 store_id"
// @Success      200  {object}  object{code=int,message=string,data=SellerProviderAccountData}
// @Failure      401  {object}  object{code=int,message=string}  "未登入或 JWT 無效"
// @Failure      403  {object}  object{code=int,message=string}  "非 admin 身分"
// @Failure      404  {object}  object{code=int,message=string}  "9330"
// @Failure      429  {object}  object{code=int,message=string}  "請求過於頻繁"
// @Failure      503  {object}  object{code=int,message=string}  "invoice-service 不可用或逾時"
// @Router       /admin/invoices/provider_accounts/{store_id} [get]
func (h *Holder) GetProviderAccountForAdmin(c *gin.Context) {
	accountID, ok := getAccountID(c)
	if !ok {
		response.Unauthorized(c)
		return
	}

	storeId := c.Param("store_id")
	if storeId == "" {
		response.BadRequest(c, "store_id is required")
		return
	}

	resp, err := h.Invoice.GetProviderAccountForAdmin(c.Request.Context(), &pb.GetProviderAccountForAdminRequest{
		ActorAccountId: accountID,
		StoreId:        storeId,
	})
	if err != nil {
		response.GrpcErr(c, "invoice", err)
		return
	}

	data := convInvSellerProviderAccountData(resp.GetData())
	// preserveData 業務碼不會轉成 err（interceptor 對它們 return nil），必須自己看 code。
	if resp.GetCode() != 0 {
		response.Biz(c, resp.GetCode(), resp.GetMessage(), data)
		return
	}
	response.OK(c, data)
}

// HealthCheckProviderAccountForAdmin ADM AMEGO 帳號健康檢查
// @Summary      ADM AMEGO 帳號健康檢查
// @Description  ADM AMEGO 帳號健康檢查（58 號設計書 §6.1 端點 44）
// @Tags         管理員 (Admin)
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        Idempotency-Key  header  string  true  "冪等鍵，^[A-Za-z0-9._:-]{8,128}$"
// @Param        store_id  path  string  true  "賣家 store_id"
// @Success      200  {object}  object{code=int,message=string,data=SellerProviderAccountData}
// @Failure      401  {object}  object{code=int,message=string}  "未登入或 JWT 無效"
// @Failure      403  {object}  object{code=int,message=string}  "非 admin 身分"
// @Failure      404  {object}  object{code=int,message=string}  "9330"
// @Failure      409  {object}  object{code=int,message=string}  "9332 provider 不可用 / 9350 同鍵不同內容 / 9351 同鍵處理中"
// @Failure      429  {object}  object{code=int,message=string}  "請求過於頻繁"
// @Failure      503  {object}  object{code=int,message=string}  "invoice-service 不可用或逾時"
// @Router       /admin/invoices/provider_accounts/{store_id}/health_checks [post]
func (h *Holder) HealthCheckProviderAccountForAdmin(c *gin.Context) {
	accountID, ok := getAccountID(c)
	if !ok {
		response.Unauthorized(c)
		return
	}

	storeId := c.Param("store_id")
	if storeId == "" {
		response.BadRequest(c, "store_id is required")
		return
	}

	resp, err := h.Invoice.HealthCheckProviderAccountForAdmin(c.Request.Context(), &pb.HealthCheckProviderAccountForAdminRequest{
		ActorAccountId: accountID,
		StoreId:        storeId,
		IdempotencyKey: c.GetHeader(middleware.IdempotencyKeyHeader),
	})
	if err != nil {
		response.GrpcErr(c, "invoice", err)
		return
	}

	data := convInvSellerProviderAccountData(resp.GetData())
	// preserveData 業務碼不會轉成 err（interceptor 對它們 return nil），必須自己看 code。
	if resp.GetCode() != 0 {
		response.Biz(c, resp.GetCode(), resp.GetMessage(), data)
		return
	}
	response.OK(c, data)
}

// DeactivateProviderAccountForAdmin ADM 停用 AMEGO 帳號
// @Summary      ADM 停用 AMEGO 帳號
// @Description  ADM 停用 AMEGO 帳號（58 號設計書 §6.1 端點 45）
// @Tags         管理員 (Admin)
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        Idempotency-Key  header  string  true  "冪等鍵，^[A-Za-z0-9._:-]{8,128}$"
// @Param        store_id  path  string  true  "賣家 store_id"
// @Param        request  body  DeactivateProviderAccountForAdminRequest  true  "ADM 停用 AMEGO 帳號"
// @Success      200  {object}  object{code=int,message=string,data=DeactivateProviderAccountForAdminData}
// @Failure      400  {object}  object{code=int,message=string}
// @Failure      401  {object}  object{code=int,message=string}  "未登入或 JWT 無效"
// @Failure      403  {object}  object{code=int,message=string}  "非 admin 身分"
// @Failure      404  {object}  object{code=int,message=string}  "9330"
// @Failure      409  {object}  object{code=int,message=string}  "9322 / 9350 同鍵不同內容 / 9351 同鍵處理中"
// @Failure      429  {object}  object{code=int,message=string}  "請求過於頻繁"
// @Failure      503  {object}  object{code=int,message=string}  "invoice-service 不可用或逾時"
// @Router       /admin/invoices/provider_accounts/{store_id}/deactivations [post]
func (h *Holder) DeactivateProviderAccountForAdmin(c *gin.Context) {
	accountID, ok := getAccountID(c)
	if !ok {
		response.Unauthorized(c)
		return
	}

	storeId := c.Param("store_id")
	if storeId == "" {
		response.BadRequest(c, "store_id is required")
		return
	}

	var body DeactivateProviderAccountForAdminRequest
	if err := c.ShouldBindJSON(&body); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	resp, err := h.Invoice.DeactivateProviderAccountForAdmin(c.Request.Context(), &pb.DeactivateProviderAccountForAdminRequest{
		ActorAccountId:  accountID,
		StoreId:         storeId,
		Reason:          body.Reason,
		Confirmed:       body.Confirmed,
		ExpectedVersion: body.ExpectedVersion,
		IdempotencyKey:  c.GetHeader(middleware.IdempotencyKeyHeader),
	})
	if err != nil {
		response.GrpcErr(c, "invoice", err)
		return
	}

	data := convInvDeactivateProviderAccountForAdminData(resp.GetData())
	// preserveData 業務碼不會轉成 err（interceptor 對它們 return nil），必須自己看 code。
	if resp.GetCode() != 0 {
		response.Biz(c, resp.GetCode(), resp.GetMessage(), data)
		return
	}
	response.OK(c, data)
}

// ListProviderCutoversForAdmin ADM AMEGO 切換紀錄
// @Summary      ADM AMEGO 切換紀錄
// @Description  ADM AMEGO 切換紀錄（58 號設計書 §6.1 端點 46）
// @Tags         管理員 (Admin)
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        store_id  path  string  true  "賣家 store_id"
// @Param        page  query  string  false  "頁碼，數字字串 ≥ 1，預設 1"
// @Param        page_size  query  string  false  "每頁筆數，數字字串 1–50，預設 10（逾越一律 400，不夾住）"
// @Success      200  {object}  object{code=int,message=string,data=object{records=[]ProviderCutoverItem,total=string,page=string,pageSize=string}}
// @Failure      400  {object}  object{code=int,message=string}  "參數格式錯誤"
// @Failure      401  {object}  object{code=int,message=string}  "未登入或 JWT 無效"
// @Failure      403  {object}  object{code=int,message=string}  "非 admin 身分"
// @Failure      404  {object}  object{code=int,message=string}  "9330"
// @Failure      429  {object}  object{code=int,message=string}  "請求過於頻繁"
// @Failure      503  {object}  object{code=int,message=string}  "invoice-service 不可用或逾時"
// @Router       /admin/invoices/provider_accounts/{store_id}/cutovers [get]
func (h *Holder) ListProviderCutoversForAdmin(c *gin.Context) {
	accountID, ok := getAccountID(c)
	if !ok {
		response.Unauthorized(c)
		return
	}

	storeId := c.Param("store_id")
	if storeId == "" {
		response.BadRequest(c, "store_id is required")
		return
	}

	page, pageSize, ok := invoicePaging(c, defaultPageSizeStr)
	if !ok {
		return
	}

	resp, err := h.Invoice.ListProviderCutoversForAdmin(c.Request.Context(), &pb.ListProviderCutoversForAdminRequest{
		ActorAccountId: accountID,
		StoreId:        storeId,
		Page:           page,
		PageSize:       pageSize,
	})
	if err != nil {
		response.GrpcErr(c, "invoice", err)
		return
	}

	data := resp.GetData()
	records := make([]ProviderCutoverItem, 0, len(data.GetRecords()))
	for _, it := range data.GetRecords() {
		records = append(records, convInvProviderCutoverItem(it))
	}
	// preserveData 業務碼不會轉成 err（interceptor 對它們 return nil），必須自己看 code。
	if resp.GetCode() != 0 {
		response.Biz(c, resp.GetCode(), resp.GetMessage(), response.PageData{
			Records:  records,
			Total:    data.GetTotal(),
			Page:     data.GetPage(),
			PageSize: data.GetPageSize(),
		})
		return
	}
	response.Page(c, records, data.GetTotal(), data.GetPage(), data.GetPageSize())
}

// ListInvoiceDocumentsForAdmin ADM 發票文件列表
// @Summary      ADM 發票文件列表
// @Description  ADM 發票文件列表（58 號設計書 §6.1 端點 47）
// @Tags         管理員 (Admin)
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        store_id  query  string  false  "賣家 store_id"
// @Param        order_no  query  string  false  "訂單編號"
// @Param        status  query  string  false  "狀態篩選"
// @Param        mode  query  string  false  "開票模式篩選"
// @Param        date_from  query  string  false  "起始日期 YYYY-MM-DD"
// @Param        date_to  query  string  false  "結束日期 YYYY-MM-DD"
// @Param        page  query  string  false  "頁碼，數字字串 ≥ 1，預設 1"
// @Param        page_size  query  string  false  "每頁筆數，數字字串 1–50，預設 20（逾越一律 400，不夾住）"
// @Success      200  {object}  object{code=int,message=string,data=object{records=[]SellerInvoiceDocumentItem,total=string,page=string,pageSize=string,shipmentStatusDegraded=bool}}
// @Failure      400  {object}  object{code=int,message=string}  "日期格式"
// @Failure      401  {object}  object{code=int,message=string}  "未登入或 JWT 無效"
// @Failure      403  {object}  object{code=int,message=string}  "非 admin 身分"
// @Failure      429  {object}  object{code=int,message=string}  "請求過於頻繁"
// @Failure      503  {object}  object{code=int,message=string}  "invoice-service 不可用或逾時"
// @Router       /admin/invoices/documents [get]
func (h *Holder) ListInvoiceDocumentsForAdmin(c *gin.Context) {
	accountID, ok := getAccountID(c)
	if !ok {
		response.Unauthorized(c)
		return
	}

	page, pageSize, ok := invoicePaging(c, invoiceAdminPageSizeStr)
	if !ok {
		return
	}

	resp, err := h.Invoice.ListInvoiceDocumentsForAdmin(c.Request.Context(), &pb.ListInvoiceDocumentsForAdminRequest{
		ActorAccountId: accountID,
		StoreId:        c.Query("store_id"),
		OrderNo:        c.Query("order_no"),
		Status:         c.Query("status"),
		Mode:           c.Query("mode"),
		DateFrom:       c.Query("date_from"),
		DateTo:         c.Query("date_to"),
		Page:           page,
		PageSize:       pageSize,
	})
	if err != nil {
		response.GrpcErr(c, "invoice", err)
		return
	}

	data := resp.GetData()
	records := make([]SellerInvoiceDocumentItem, 0, len(data.GetRecords()))
	for _, it := range data.GetRecords() {
		records = append(records, convInvSellerInvoiceDocumentItem(it))
	}
	// #3019：同端點 30，多帶 shipmentStatusDegraded（兩支共用 SellerInvoiceDocumentItem，
	// 降級語意必須一致，否則 A12 會靜默退回舊症狀）。
	pageData := InvoiceDocumentsPageData{
		PageData: response.PageData{
			Records:  records,
			Total:    data.GetTotal(),
			Page:     data.GetPage(),
			PageSize: data.GetPageSize(),
		},
		ShipmentStatusDegraded: data.GetShipmentStatusDegraded(),
	}
	// preserveData 業務碼不會轉成 err（interceptor 對它們 return nil），必須自己看 code。
	if resp.GetCode() != 0 {
		response.Biz(c, resp.GetCode(), resp.GetMessage(), pageData)
		return
	}
	response.OK(c, pageData)
}

// GetInvoiceDocumentForAdmin ADM 發票文件詳情
// @Summary      ADM 發票文件詳情
// @Description  ADM 發票文件詳情（58 號設計書 §6.1 端點 48）
// @Tags         管理員 (Admin)
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        document_id  path  string  true  "發票文件 ID"
// @Success      200  {object}  object{code=int,message=string,data=AdminInvoiceDocumentDetail}
// @Failure      401  {object}  object{code=int,message=string}  "未登入或 JWT 無效"
// @Failure      403  {object}  object{code=int,message=string}  "非 admin 身分"
// @Failure      404  {object}  object{code=int,message=string}  "9301"
// @Failure      429  {object}  object{code=int,message=string}  "請求過於頻繁"
// @Failure      503  {object}  object{code=int,message=string}  "invoice-service 不可用或逾時"
// @Router       /admin/invoices/documents/{document_id} [get]
func (h *Holder) GetInvoiceDocumentForAdmin(c *gin.Context) {
	accountID, ok := getAccountID(c)
	if !ok {
		response.Unauthorized(c)
		return
	}

	documentId := c.Param("document_id")
	if documentId == "" {
		response.BadRequest(c, "document_id is required")
		return
	}

	resp, err := h.Invoice.GetInvoiceDocumentForAdmin(c.Request.Context(), &pb.GetInvoiceDocumentForAdminRequest{
		ActorAccountId: accountID,
		DocumentId:     documentId,
	})
	if err != nil {
		response.GrpcErr(c, "invoice", err)
		return
	}

	data := convInvAdminInvoiceDocumentDetail(resp.GetData())
	// preserveData 業務碼不會轉成 err（interceptor 對它們 return nil），必須自己看 code。
	if resp.GetCode() != 0 {
		response.Biz(c, resp.GetCode(), resp.GetMessage(), data)
		return
	}
	response.OK(c, data)
}

// ResendInvoiceNotificationForAdmin ADM 重寄發票通知
// @Summary      ADM 重寄發票通知
// @Description  ADM 重寄發票通知（58 號設計書 §6.1 端點 49）
// @Tags         管理員 (Admin)
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        Idempotency-Key  header  string  true  "冪等鍵，^[A-Za-z0-9._:-]{8,128}$"
// @Param        document_id  path  string  true  "發票文件 ID"
// @Param        request  body  ResendInvoiceNotificationForAdminRequest  true  "ADM 重寄發票通知"
// @Success      200  {object}  object{code=int,message=string,data=ResendInvoiceNotificationData}
// @Failure      400  {object}  object{code=int,message=string}
// @Failure      401  {object}  object{code=int,message=string}  "未登入或 JWT 無效"
// @Failure      403  {object}  object{code=int,message=string}  "非 admin 身分"
// @Failure      404  {object}  object{code=int,message=string}  "9301"
// @Failure      409  {object}  object{code=int,message=string}  "9350 同鍵不同內容 / 9351 同鍵處理中"
// @Failure      429  {object}  object{code=int,message=string}  "9352"
// @Failure      503  {object}  object{code=int,message=string}  "invoice-service 不可用或逾時"
// @Router       /admin/invoices/documents/{document_id}/notification_resends [post]
func (h *Holder) ResendInvoiceNotificationForAdmin(c *gin.Context) {
	accountID, ok := getAccountID(c)
	if !ok {
		response.Unauthorized(c)
		return
	}

	documentId := c.Param("document_id")
	if documentId == "" {
		response.BadRequest(c, "document_id is required")
		return
	}

	var body ResendInvoiceNotificationForAdminRequest
	if err := c.ShouldBindJSON(&body); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	resp, err := h.Invoice.ResendInvoiceNotificationForAdmin(c.Request.Context(), &pb.ResendInvoiceNotificationForAdminRequest{
		ActorAccountId: accountID,
		DocumentId:     documentId,
		Reason:         body.Reason,
		IdempotencyKey: c.GetHeader(middleware.IdempotencyKeyHeader),
	})
	if err != nil {
		response.GrpcErr(c, "invoice", err)
		return
	}

	data := convInvResendInvoiceNotificationData(resp.GetData())
	// preserveData 業務碼不會轉成 err（interceptor 對它們 return nil），必須自己看 code。
	if resp.GetCode() != 0 {
		response.Biz(c, resp.GetCode(), resp.GetMessage(), data)
		return
	}
	response.OK(c, data)
}

// ListInvoiceJobsForAdmin ADM 發票工作佇列
// @Summary      ADM 發票工作佇列
// @Description  ADM 發票工作佇列（58 號設計書 §6.1 端點 50）
// @Tags         管理員 (Admin)
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        tab  query  string  false  "分頁籤"
// @Param        store_id  query  string  false  "賣家 store_id"
// @Param        order_no  query  string  false  "訂單編號"
// @Param        operation_type  query  string  false  "操作類型"
// @Param        upload_status  query  string  false  "財政部上傳狀態，白名單 PENDING/UPLOADED/NOT_FOUND/FAILED"
// @Param        manual_reason  query  string  false  "人工處理理由碼（CodeMeta InvoiceManualReason）"
// @Param        page  query  string  false  "頁碼，數字字串 ≥ 1，預設 1"
// @Param        page_size  query  string  false  "每頁筆數，數字字串 1–50，預設 20（逾越一律 400，不夾住）"
// @Success      200  {object}  object{code=int,message=string,data=object{records=[]InvoiceJobInfo,total=string,page=string,pageSize=string}}
// @Failure      400  {object}  object{code=int,message=string}
// @Failure      401  {object}  object{code=int,message=string}  "未登入或 JWT 無效"
// @Failure      403  {object}  object{code=int,message=string}  "非 admin 身分"
// @Failure      429  {object}  object{code=int,message=string}  "請求過於頻繁"
// @Failure      503  {object}  object{code=int,message=string}  "invoice-service 不可用或逾時"
// @Router       /admin/invoices/jobs [get]
func (h *Holder) ListInvoiceJobsForAdmin(c *gin.Context) {
	accountID, ok := getAccountID(c)
	if !ok {
		response.Unauthorized(c)
		return
	}

	page, pageSize, ok := invoicePaging(c, invoiceAdminPageSizeStr)
	if !ok {
		return
	}

	resp, err := h.Invoice.ListInvoiceJobsForAdmin(c.Request.Context(), &pb.ListInvoiceJobsForAdminRequest{
		ActorAccountId: accountID,
		Tab:            c.Query("tab"),
		StoreId:        c.Query("store_id"),
		OrderNo:        c.Query("order_no"),
		OperationType:  c.Query("operation_type"),
		// §6.1.14 的兩個額外 query。upload_status 是「發票開出來了、財政部端沒收到」
		// 這一類的唯一入口——那些 job 落 COMPLETED、與真正完成的混在同一頁。
		UploadStatus: c.Query("upload_status"),
		ManualReason: c.Query("manual_reason"),
		Page:         page,
		PageSize:     pageSize,
	})
	if err != nil {
		response.GrpcErr(c, "invoice", err)
		return
	}

	data := resp.GetData()
	records := make([]InvoiceJobInfo, 0, len(data.GetRecords()))
	for _, it := range data.GetRecords() {
		records = append(records, convInvInvoiceJobInfo(it))
	}
	// preserveData 業務碼不會轉成 err（interceptor 對它們 return nil），必須自己看 code。
	if resp.GetCode() != 0 {
		response.Biz(c, resp.GetCode(), resp.GetMessage(), response.PageData{
			Records:  records,
			Total:    data.GetTotal(),
			Page:     data.GetPage(),
			PageSize: data.GetPageSize(),
		})
		return
	}
	response.Page(c, records, data.GetTotal(), data.GetPage(), data.GetPageSize())
}

// GetInvoiceJobForAdmin ADM 發票工作詳情
// @Summary      ADM 發票工作詳情
// @Description  ADM 發票工作詳情（58 號設計書 §6.1 端點 51）
// @Tags         管理員 (Admin)
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        job_id  path  string  true  "發票工作 ID"
// @Success      200  {object}  object{code=int,message=string,data=AdminInvoiceJobDetail}
// @Failure      401  {object}  object{code=int,message=string}  "未登入或 JWT 無效"
// @Failure      403  {object}  object{code=int,message=string}  "非 admin 身分"
// @Failure      404  {object}  object{code=int,message=string}  "9340"
// @Failure      429  {object}  object{code=int,message=string}  "請求過於頻繁"
// @Failure      503  {object}  object{code=int,message=string}  "invoice-service 不可用或逾時"
// @Router       /admin/invoices/jobs/{job_id} [get]
func (h *Holder) GetInvoiceJobForAdmin(c *gin.Context) {
	accountID, ok := getAccountID(c)
	if !ok {
		response.Unauthorized(c)
		return
	}

	jobId := c.Param("job_id")
	if jobId == "" {
		response.BadRequest(c, "job_id is required")
		return
	}

	resp, err := h.Invoice.GetInvoiceJobForAdmin(c.Request.Context(), &pb.GetInvoiceJobForAdminRequest{
		ActorAccountId: accountID,
		JobId:          jobId,
	})
	if err != nil {
		response.GrpcErr(c, "invoice", err)
		return
	}

	data := convInvAdminInvoiceJobDetail(resp.GetData())
	// preserveData 業務碼不會轉成 err（interceptor 對它們 return nil），必須自己看 code。
	if resp.GetCode() != 0 {
		response.Biz(c, resp.GetCode(), resp.GetMessage(), data)
		return
	}
	response.OK(c, data)
}

// RetryInvoiceJobForAdmin ADM 安全重試發票工作
// @Summary      ADM 安全重試發票工作
// @Description  ADM 安全重試發票工作（58 號設計書 §6.1 端點 52）
// @Tags         管理員 (Admin)
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        Idempotency-Key  header  string  true  "冪等鍵，^[A-Za-z0-9._:-]{8,128}$"
// @Param        job_id  path  string  true  "發票工作 ID"
// @Param        request  body  RetryInvoiceJobForAdminRequest  true  "ADM 安全重試發票工作"
// @Success      200  {object}  object{code=int,message=string,data=InvoiceJobMutationData}
// @Failure      400  {object}  object{code=int,message=string}  "含缺 fundingDecision"
// @Failure      401  {object}  object{code=int,message=string}  "未登入或 JWT 無效"
// @Failure      403  {object}  object{code=int,message=string}  "非 admin 身分"
// @Failure      404  {object}  object{code=int,message=string}  "9340"
// @Failure      409  {object}  object{code=int,message=string}  "9341 不可安全重試 / 9343 版本衝突 / 9350 同鍵不同內容 / 9351 同鍵處理中"
// @Failure      429  {object}  object{code=int,message=string}  "請求過於頻繁"
// @Failure      503  {object}  object{code=int,message=string}  "invoice-service 不可用或逾時"
// @Router       /admin/invoices/jobs/{job_id}/retries [post]
func (h *Holder) RetryInvoiceJobForAdmin(c *gin.Context) {
	accountID, ok := getAccountID(c)
	if !ok {
		response.Unauthorized(c)
		return
	}

	jobId := c.Param("job_id")
	if jobId == "" {
		response.BadRequest(c, "job_id is required")
		return
	}

	var body RetryInvoiceJobForAdminRequest
	if err := c.ShouldBindJSON(&body); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	resp, err := h.Invoice.RetryInvoiceJobForAdmin(c.Request.Context(), &pb.RetryInvoiceJobForAdminRequest{
		ActorAccountId:  accountID,
		JobId:           jobId,
		Reason:          body.Reason,
		ExpectedVersion: body.ExpectedVersion,
		IdempotencyKey:  c.GetHeader(middleware.IdempotencyKeyHeader),
		FundingDecision: body.FundingDecision,
	})
	if err != nil {
		response.GrpcErr(c, "invoice", err)
		return
	}

	data := convInvInvoiceJobMutationData(resp.GetData())
	// preserveData 業務碼不會轉成 err（interceptor 對它們 return nil），必須自己看 code。
	if resp.GetCode() != 0 {
		response.Biz(c, resp.GetCode(), resp.GetMessage(), data)
		return
	}
	response.OK(c, data)
}

// ResumeInvoiceDocumentForAdmin ADM 將待人工處理的發票文件送回自動線
// @Summary      ADM 將待人工處理的發票文件送回自動線
// @Description  ADM 將待人工處理、且尚無任何開票作業的發票文件送回自動線（58 號設計書 §6.1 端點 73）。自動線落 AWAITING_MANUAL 時不建 operation，那些文件沒有 job_id 可用，端點 52／53／56 全部組不出請求；本端點是它們唯一的入口，成功後會補建一件開票作業並回傳其 jobId。
// @Tags         管理員 (Admin)
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        Idempotency-Key  header  string  true  "冪等鍵，^[A-Za-z0-9._:-]{8,128}$"
// @Param        document_id  path  string  true  "發票文件 ID"
// @Param        request  body  ResumeInvoiceDocumentForAdminRequest  true  "ADM 將待人工處理的發票文件送回自動線"
// @Success      200  {object}  object{code=int,message=string,data=InvoiceJobMutationData}
// @Failure      400  {object}  object{code=int,message=string}  "含缺 fundingDecision"
// @Failure      401  {object}  object{code=int,message=string}  "未登入或 JWT 無效"
// @Failure      403  {object}  object{code=int,message=string}  "非 admin 身分"
// @Failure      404  {object}  object{code=int,message=string}  "9301"
// @Failure      409  {object}  object{code=int,message=string}  "9341 文件不在待人工處理狀態或已有開票作業 / 9343 版本衝突 / 9350 同鍵不同內容 / 9351 同鍵處理中"
// @Failure      429  {object}  object{code=int,message=string}  "請求過於頻繁"
// @Failure      503  {object}  object{code=int,message=string}  "invoice-service 不可用或逾時"
// @Router       /admin/invoices/documents/{document_id}/manual_resolutions [post]
func (h *Holder) ResumeInvoiceDocumentForAdmin(c *gin.Context) {
	accountID, ok := getAccountID(c)
	if !ok {
		response.Unauthorized(c)
		return
	}

	documentId := c.Param("document_id")
	if documentId == "" {
		response.BadRequest(c, "document_id is required")
		return
	}

	var body ResumeInvoiceDocumentForAdminRequest
	if err := c.ShouldBindJSON(&body); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	resp, err := h.Invoice.ResumeInvoiceDocumentForAdmin(c.Request.Context(), &pb.ResumeInvoiceDocumentForAdminRequest{
		ActorAccountId:  accountID,
		DocumentId:      documentId,
		Reason:          body.Reason,
		ExpectedVersion: body.ExpectedVersion,
		IdempotencyKey:  c.GetHeader(middleware.IdempotencyKeyHeader),
		FundingDecision: body.FundingDecision,
	})
	if err != nil {
		response.GrpcErr(c, "invoice", err)
		return
	}

	data := convInvInvoiceJobMutationData(resp.GetData())
	// preserveData 業務碼不會轉成 err（interceptor 對它們 return nil），必須自己看 code。
	if resp.GetCode() != 0 {
		response.Biz(c, resp.GetCode(), resp.GetMessage(), data)
		return
	}
	response.OK(c, data)
}

// CancelInvoiceJobForAdmin ADM 受控取消發票工作
// @Summary      ADM 受控取消發票工作
// @Description  ADM 受控取消發票工作（58 號設計書 §6.1 端點 53）
// @Tags         管理員 (Admin)
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        Idempotency-Key  header  string  true  "冪等鍵，^[A-Za-z0-9._:-]{8,128}$"
// @Param        job_id  path  string  true  "發票工作 ID"
// @Param        request  body  CancelInvoiceJobForAdminRequest  true  "ADM 受控取消發票工作"
// @Success      200  {object}  object{code=int,message=string,data=InvoiceJobMutationData}
// @Failure      400  {object}  object{code=int,message=string}
// @Failure      401  {object}  object{code=int,message=string}  "未登入或 JWT 無效"
// @Failure      403  {object}  object{code=int,message=string}  "非 admin 身分"
// @Failure      404  {object}  object{code=int,message=string}  "9340"
// @Failure      409  {object}  object{code=int,message=string}  "9343 / 9350 同鍵不同內容 / 9351 同鍵處理中"
// @Failure      429  {object}  object{code=int,message=string}  "請求過於頻繁"
// @Failure      503  {object}  object{code=int,message=string}  "invoice-service 不可用或逾時"
// @Router       /admin/invoices/jobs/{job_id}/cancellations [post]
func (h *Holder) CancelInvoiceJobForAdmin(c *gin.Context) {
	accountID, ok := getAccountID(c)
	if !ok {
		response.Unauthorized(c)
		return
	}

	jobId := c.Param("job_id")
	if jobId == "" {
		response.BadRequest(c, "job_id is required")
		return
	}

	var body CancelInvoiceJobForAdminRequest
	if err := c.ShouldBindJSON(&body); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	resp, err := h.Invoice.CancelInvoiceJobForAdmin(c.Request.Context(), &pb.CancelInvoiceJobForAdminRequest{
		ActorAccountId:               accountID,
		JobId:                        jobId,
		ReasonCode:                   body.ReasonCode,
		ReasonDetail:                 body.ReasonDetail,
		AllowManualAfterCancellation: body.AllowManualAfterCancellation,
		ExpectedVersion:              body.ExpectedVersion,
		IdempotencyKey:               c.GetHeader(middleware.IdempotencyKeyHeader),
	})
	if err != nil {
		response.GrpcErr(c, "invoice", err)
		return
	}

	data := convInvInvoiceJobMutationData(resp.GetData())
	// §6.1.0(13) 的 preserveData 碼：EnvelopeGuardInterceptor 對它們不轉 error，
	// data 必須原樣透傳（HTTP 恆 200），否則前端拿不到導向用的 id／時間。
	if resp.GetCode() != 0 {
		response.Biz(c, resp.GetCode(), resp.GetMessage(), data)
		return
	}
	response.OK(c, data)
}

// RequestManualUnlockForAdmin ADM 申請人工出口解鎖
// @Summary      ADM 申請人工出口解鎖
// @Description  ADM 申請人工出口解鎖（58 號設計書 §6.1 端點 54）。allowed-from 有兩個：CANCELLED_BY_ADMIN（不帶證據）與 CANCEL_BLOCKED_UNKNOWN（cancelEvidenceKind 必填，覆核時同一交易內走 J28→J29）。
// @Tags         管理員 (Admin)
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        Idempotency-Key  header  string  true  "冪等鍵，^[A-Za-z0-9._:-]{8,128}$"
// @Param        job_id  path  string  true  "發票工作 ID"
// @Param        request  body  RequestManualUnlockForAdminRequest  true  "ADM 申請人工出口解鎖"
// @Success      200  {object}  object{code=int,message=string,data=ManualUnlockInfo}
// @Failure      400  {object}  object{code=int,message=string}
// @Failure      401  {object}  object{code=int,message=string}  "未登入或 JWT 無效"
// @Failure      403  {object}  object{code=int,message=string}  "非 admin 身分"
// @Failure      404  {object}  object{code=int,message=string}  "9340"
// @Failure      409  {object}  object{code=int,message=string}  "9343 / 9350 同鍵不同內容 / 9351 同鍵處理中"
// @Failure      429  {object}  object{code=int,message=string}  "請求過於頻繁"
// @Failure      503  {object}  object{code=int,message=string}  "invoice-service 不可用或逾時"
// @Router       /admin/invoices/jobs/{job_id}/manual_unlocks [post]
//
// 200 ＋ 9344：已有一個等待覆核的解鎖請求，`data.unlockId` 是**既有那一列**的 id
// （preserveData，§6.1.0(13)）。
func (h *Holder) RequestManualUnlockForAdmin(c *gin.Context) {
	accountID, ok := getAccountID(c)
	if !ok {
		response.Unauthorized(c)
		return
	}

	jobId := c.Param("job_id")
	if jobId == "" {
		response.BadRequest(c, "job_id is required")
		return
	}

	var body RequestManualUnlockForAdminRequest
	if err := c.ShouldBindJSON(&body); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	resp, err := h.Invoice.RequestManualUnlockForAdmin(c.Request.Context(), &pb.RequestManualUnlockForAdminRequest{
		ActorAccountId:            accountID,
		JobId:                     jobId,
		ProviderCheckEvidence:     body.ProviderCheckEvidence,
		ProviderCheckedAt:         body.ProviderCheckedAt,
		Reason:                    body.Reason,
		ExpectedVersion:           body.ExpectedVersion,
		CancelEvidenceKind:        body.CancelEvidenceKind,
		CancelEvidenceObjectKey:   body.CancelEvidenceObjectKey,
		CancelEvidenceStagedToken: body.CancelEvidenceStagedToken,
		IdempotencyKey:            c.GetHeader(middleware.IdempotencyKeyHeader),
	})
	if err != nil {
		response.GrpcErr(c, "invoice", err)
		return
	}

	data := convInvManualUnlockInfo(resp.GetData())
	// §6.1.0(13) 的 preserveData 碼：EnvelopeGuardInterceptor 對它們不轉 error，
	// data 必須原樣透傳（HTTP 恆 200）。端點 54 會在「已經有一個等待覆核的請求」時
	// 回 9344 ＋ 既有那一列的 unlockId —— 少了這一段，非零碼會被 response.OK 壓成 0，
	// ADM 前端以為自己剛剛建立了新請求（實際上什麼都沒建立），也拿不到導向覆核
	// 對話框的 unlockId。
	if resp.GetCode() != 0 {
		response.Biz(c, resp.GetCode(), resp.GetMessage(), data)
		return
	}
	response.OK(c, data)
}

// ApproveManualUnlockForAdmin ADM 核准人工出口解鎖
// @Summary      ADM 核准人工出口解鎖
// @Description  ADM 核准人工出口解鎖（58 號設計書 §6.1 端點 55）
// @Tags         管理員 (Admin)
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        Idempotency-Key  header  string  true  "冪等鍵，^[A-Za-z0-9._:-]{8,128}$"
// @Param        job_id  path  string  true  "發票工作 ID"
// @Param        unlock_id  path  string  true  "人工出口解鎖 ID"
// @Param        request  body  ApproveManualUnlockForAdminRequest  true  "ADM 核准人工出口解鎖"
// @Success      200  {object}  object{code=int,message=string,data=ManualUnlockInfo}
// @Failure      400  {object}  object{code=int,message=string}
// @Failure      401  {object}  object{code=int,message=string}  "未登入或 JWT 無效"
// @Failure      403  {object}  object{code=int,message=string}  "非 admin 身分"
// @Failure      404  {object}  object{code=int,message=string}  "9340"
// @Failure      409  {object}  object{code=int,message=string}  "9343 / 9345 不得同一人 / 9350 同鍵不同內容 / 9351 同鍵處理中"
// @Failure      429  {object}  object{code=int,message=string}  "請求過於頻繁"
// @Failure      503  {object}  object{code=int,message=string}  "invoice-service 不可用或逾時"
// @Router       /admin/invoices/jobs/{job_id}/manual_unlocks/{unlock_id}/approvals [post]
func (h *Holder) ApproveManualUnlockForAdmin(c *gin.Context) {
	accountID, ok := getAccountID(c)
	if !ok {
		response.Unauthorized(c)
		return
	}

	jobId := c.Param("job_id")
	if jobId == "" {
		response.BadRequest(c, "job_id is required")
		return
	}
	unlockId := c.Param("unlock_id")
	if unlockId == "" {
		response.BadRequest(c, "unlock_id is required")
		return
	}

	var body ApproveManualUnlockForAdminRequest
	if err := c.ShouldBindJSON(&body); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	resp, err := h.Invoice.ApproveManualUnlockForAdmin(c.Request.Context(), &pb.ApproveManualUnlockForAdminRequest{
		ActorAccountId:  accountID,
		JobId:           jobId,
		UnlockId:        unlockId,
		Approved:        body.Approved,
		Reason:          body.Reason,
		ExpectedVersion: body.ExpectedVersion,
		IdempotencyKey:  c.GetHeader(middleware.IdempotencyKeyHeader),
	})
	if err != nil {
		response.GrpcErr(c, "invoice", err)
		return
	}

	data := convInvManualUnlockInfo(resp.GetData())
	// §6.1.0(13) 的 preserveData 碼：EnvelopeGuardInterceptor 對它們不轉 error，
	// data 必須原樣透傳（HTTP 恆 200），否則前端拿不到導向用的 id／時間。
	if resp.GetCode() != 0 {
		response.Biz(c, resp.GetCode(), resp.GetMessage(), data)
		return
	}
	response.OK(c, data)
}

// ReportManualResultForAdmin ADM 回報人工處理結果
// @Summary      ADM 回報人工處理結果
// @Description  ADM 回報人工處理結果（58 號設計書 §6.1 端點 56）
// @Tags         管理員 (Admin)
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        Idempotency-Key  header  string  true  "冪等鍵，^[A-Za-z0-9._:-]{8,128}$"
// @Param        job_id  path  string  true  "發票工作 ID"
// @Param        request  body  ReportManualResultForAdminRequest  true  "ADM 回報人工處理結果"
// @Success      200  {object}  object{code=int,message=string,data=InvoiceJobMutationData}
// @Failure      400  {object}  object{code=int,message=string}
// @Failure      401  {object}  object{code=int,message=string}  "未登入或 JWT 無效"
// @Failure      403  {object}  object{code=int,message=string}  "非 admin 身分"
// @Failure      404  {object}  object{code=int,message=string}  "9340"
// @Failure      409  {object}  object{code=int,message=string}  "9343 / 9350 同鍵不同內容 / 9351 同鍵處理中"
// @Failure      429  {object}  object{code=int,message=string}  "請求過於頻繁"
// @Failure      503  {object}  object{code=int,message=string}  "invoice-service 不可用或逾時"
// @Router       /admin/invoices/jobs/{job_id}/manual_results [post]
func (h *Holder) ReportManualResultForAdmin(c *gin.Context) {
	accountID, ok := getAccountID(c)
	if !ok {
		response.Unauthorized(c)
		return
	}

	jobId := c.Param("job_id")
	if jobId == "" {
		response.BadRequest(c, "job_id is required")
		return
	}

	var body ReportManualResultForAdminRequest
	if err := c.ShouldBindJSON(&body); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	resp, err := h.Invoice.ReportManualResultForAdmin(c.Request.Context(), &pb.ReportManualResultForAdminRequest{
		ActorAccountId:  accountID,
		JobId:           jobId,
		InvoiceNo:       body.InvoiceNo,
		InvoiceDate:     body.InvoiceDate,
		TotalAmount:     body.TotalAmount,
		Note:            body.Note,
		ExpectedVersion: body.ExpectedVersion,
		IdempotencyKey:  c.GetHeader(middleware.IdempotencyKeyHeader),
	})
	if err != nil {
		response.GrpcErr(c, "invoice", err)
		return
	}

	data := convInvInvoiceJobMutationData(resp.GetData())
	// preserveData 業務碼不會轉成 err（interceptor 對它們 return nil），必須自己看 code。
	if resp.GetCode() != 0 {
		response.Biz(c, resp.GetCode(), resp.GetMessage(), data)
		return
	}
	response.OK(c, data)
}

// CreateManualIssuanceForAdmin ADM 手動開立發票
// @Summary      ADM 手動開立發票
// @Description  ADM 手動開立發票（58 號設計書 §6.1 端點 57）
// @Tags         管理員 (Admin)
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        Idempotency-Key  header  string  true  "冪等鍵，^[A-Za-z0-9._:-]{8,128}$"
// @Param        request  body  CreateManualIssuanceForAdminRequest  true  "ADM 手動開立發票"
// @Success      200  {object}  object{code=int,message=string,data=CreateManualIssuanceForAdminData}
// @Failure      400  {object}  object{code=int,message=string}  "含缺 confirmed"
// @Failure      401  {object}  object{code=int,message=string}  "未登入或 JWT 無效"
// @Failure      403  {object}  object{code=int,message=string}  "非 admin 身分"
// @Failure      409  {object}  object{code=int,message=string}  "9332 / 9346 issuer / recipient 組合不合法 / 9350 同鍵不同內容 / 9351 同鍵處理中"
// @Failure      429  {object}  object{code=int,message=string}  "請求過於頻繁"
// @Failure      503  {object}  object{code=int,message=string}  "invoice-service 不可用或逾時"
// @Router       /admin/invoices/issuances [post]
func (h *Holder) CreateManualIssuanceForAdmin(c *gin.Context) {
	accountID, ok := getAccountID(c)
	if !ok {
		response.Unauthorized(c)
		return
	}

	var body CreateManualIssuanceForAdminRequest
	if err := c.ShouldBindJSON(&body); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	resp, err := h.Invoice.CreateManualIssuanceForAdmin(c.Request.Context(), &pb.CreateManualIssuanceForAdminRequest{
		ActorAccountId: accountID,
		Issuer:         toPbInvIssuanceIssuer(body.Issuer),
		Recipient:      toPbInvIssuanceRecipient(body.Recipient),
		Source:         toPbInvIssuanceSource(body.Source),
		DocumentKind:   body.DocumentKind,
		Reason:         body.Reason,
		Confirmed:      body.Confirmed,
		IdempotencyKey: c.GetHeader(middleware.IdempotencyKeyHeader),
	})
	if err != nil {
		response.GrpcErr(c, "invoice", err)
		return
	}

	data := convInvCreateManualIssuanceForAdminData(resp.GetData())
	// §6.1.0(13) 的 preserveData 碼：EnvelopeGuardInterceptor 對它們不轉 error，
	// data 必須原樣透傳（HTTP 恆 200），否則前端拿不到導向用的 id／時間。
	if resp.GetCode() != 0 {
		response.Biz(c, resp.GetCode(), resp.GetMessage(), data)
		return
	}
	response.OK(c, data)
}

// ListInvoiceComplaintsForAdmin ADM 申訴列表
// @Summary      ADM 申訴列表
// @Description  ADM 申訴列表（58 號設計書 §6.1 端點 58）
// @Tags         管理員 (Admin)
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        tab  query  string  false  "分頁籤"
// @Param        store_id  query  string  false  "賣家 store_id"
// @Param        order_no  query  string  false  "訂單編號（依訂單反查申訴）"
// @Param        sort  query  string  false  "排序（逾期案件優先等）"
// @Param        page  query  string  false  "頁碼，數字字串 ≥ 1，預設 1"
// @Param        page_size  query  string  false  "每頁筆數，數字字串 1–50，預設 20（逾越一律 400，不夾住）"
// @Success      200  {object}  object{code=int,message=string,data=object{records=[]InvoiceComplaintInfo,total=string,page=string,pageSize=string}}
// @Failure      400  {object}  object{code=int,message=string}
// @Failure      401  {object}  object{code=int,message=string}  "未登入或 JWT 無效"
// @Failure      403  {object}  object{code=int,message=string}  "非 admin 身分"
// @Failure      429  {object}  object{code=int,message=string}  "請求過於頻繁"
// @Failure      503  {object}  object{code=int,message=string}  "invoice-service 不可用或逾時"
// @Router       /admin/invoices/complaints [get]
func (h *Holder) ListInvoiceComplaintsForAdmin(c *gin.Context) {
	accountID, ok := getAccountID(c)
	if !ok {
		response.Unauthorized(c)
		return
	}

	page, pageSize, ok := invoicePaging(c, invoiceAdminPageSizeStr)
	if !ok {
		return
	}

	resp, err := h.Invoice.ListInvoiceComplaintsForAdmin(c.Request.Context(), &pb.ListInvoiceComplaintsForAdminRequest{
		ActorAccountId: accountID,
		Tab:            c.Query("tab"),
		StoreId:        c.Query("store_id"),
		OrderNo:        c.Query("order_no"),
		Sort:           c.Query("sort"),
		Page:           page,
		PageSize:       pageSize,
	})
	if err != nil {
		response.GrpcErr(c, "invoice", err)
		return
	}

	data := resp.GetData()
	records := make([]InvoiceComplaintInfo, 0, len(data.GetRecords()))
	for _, it := range data.GetRecords() {
		records = append(records, convInvInvoiceComplaintInfo(it))
	}
	// preserveData 業務碼不會轉成 err（interceptor 對它們 return nil），必須自己看 code。
	if resp.GetCode() != 0 {
		response.Biz(c, resp.GetCode(), resp.GetMessage(), response.PageData{
			Records:  records,
			Total:    data.GetTotal(),
			Page:     data.GetPage(),
			PageSize: data.GetPageSize(),
		})
		return
	}
	response.Page(c, records, data.GetTotal(), data.GetPage(), data.GetPageSize())
}

// GetInvoiceComplaintForAdmin ADM 申訴詳情
// @Summary      ADM 申訴詳情
// @Description  ADM 申訴詳情（58 號設計書 §6.1 端點 59）
// @Tags         管理員 (Admin)
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        complaint_id  path  string  true  "申訴案件 ID"
// @Success      200  {object}  object{code=int,message=string,data=InvoiceComplaintInfo}
// @Failure      401  {object}  object{code=int,message=string}  "未登入或 JWT 無效"
// @Failure      403  {object}  object{code=int,message=string}  "非 admin 身分"
// @Failure      404  {object}  object{code=int,message=string}  "9309"
// @Failure      429  {object}  object{code=int,message=string}  "請求過於頻繁"
// @Failure      503  {object}  object{code=int,message=string}  "invoice-service 不可用或逾時"
// @Router       /admin/invoices/complaints/{complaint_id} [get]
func (h *Holder) GetInvoiceComplaintForAdmin(c *gin.Context) {
	accountID, ok := getAccountID(c)
	if !ok {
		response.Unauthorized(c)
		return
	}

	complaintId := c.Param("complaint_id")
	if complaintId == "" {
		response.BadRequest(c, "complaint_id is required")
		return
	}

	resp, err := h.Invoice.GetInvoiceComplaintForAdmin(c.Request.Context(), &pb.GetInvoiceComplaintForAdminRequest{
		ActorAccountId: accountID,
		ComplaintId:    complaintId,
	})
	if err != nil {
		response.GrpcErr(c, "invoice", err)
		return
	}

	data := convInvInvoiceComplaintInfo(resp.GetData())
	// preserveData 業務碼不會轉成 err（interceptor 對它們 return nil），必須自己看 code。
	if resp.GetCode() != 0 {
		response.Biz(c, resp.GetCode(), resp.GetMessage(), data)
		return
	}
	response.OK(c, data)
}

// ActOnInvoiceComplaintForAdmin ADM 申訴處理動作
// @Summary      ADM 申訴處理動作
// @Description  ADM 申訴處理動作（58 號設計書 §6.1 端點 60）
// @Tags         管理員 (Admin)
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        Idempotency-Key  header  string  true  "冪等鍵，^[A-Za-z0-9._:-]{8,128}$"
// @Param        complaint_id  path  string  true  "申訴案件 ID"
// @Param        request  body  ActOnInvoiceComplaintForAdminRequest  true  "ADM 申訴處理動作"
// @Success      200  {object}  object{code=int,message=string,data=ActOnInvoiceComplaintForAdminData}
// @Failure      400  {object}  object{code=int,message=string}  "含缺 confirmed"
// @Failure      401  {object}  object{code=int,message=string}  "未登入或 JWT 無效"
// @Failure      403  {object}  object{code=int,message=string}  "非 admin 身分"
// @Failure      404  {object}  object{code=int,message=string}  "9309"
// @Failure      409  {object}  object{code=int,message=string}  "9322 / 9336 / 9350 同鍵不同內容 / 9351 同鍵處理中"
// @Failure      429  {object}  object{code=int,message=string}  "請求過於頻繁"
// @Failure      503  {object}  object{code=int,message=string}  "invoice-service 不可用或逾時"
// @Router       /admin/invoices/complaints/{complaint_id}/actions [post]
func (h *Holder) ActOnInvoiceComplaintForAdmin(c *gin.Context) {
	accountID, ok := getAccountID(c)
	if !ok {
		response.Unauthorized(c)
		return
	}

	complaintId := c.Param("complaint_id")
	if complaintId == "" {
		response.BadRequest(c, "complaint_id is required")
		return
	}

	var body ActOnInvoiceComplaintForAdminRequest
	if err := c.ShouldBindJSON(&body); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	resp, err := h.Invoice.ActOnInvoiceComplaintForAdmin(c.Request.Context(), &pb.ActOnInvoiceComplaintForAdminRequest{
		ActorAccountId:  accountID,
		ComplaintId:     complaintId,
		Action:          body.Action,
		Reason:          body.Reason,
		Confirmed:       body.Confirmed,
		ExpectedVersion: body.ExpectedVersion,
		IdempotencyKey:  c.GetHeader(middleware.IdempotencyKeyHeader),
	})
	if err != nil {
		response.GrpcErr(c, "invoice", err)
		return
	}

	data := convInvActOnInvoiceComplaintForAdminData(resp.GetData())
	// preserveData 業務碼不會轉成 err（interceptor 對它們 return nil），必須自己看 code。
	if resp.GetCode() != 0 {
		response.Biz(c, resp.GetCode(), resp.GetMessage(), data)
		return
	}
	response.OK(c, data)
}

// ListInvoiceAuditLogsForAdmin ADM 發票稽核紀錄
// @Summary      ADM 發票稽核紀錄
// @Description  ADM 發票稽核紀錄（58 號設計書 §6.1 端點 61）
// @Tags         管理員 (Admin)
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        actor_account_id  query  string  false  "被稽核的操作者 account_id"
// @Param        store_id  query  string  false  "賣家 store_id"
// @Param        order_no  query  string  false  "訂單編號"
// @Param        document_id  query  string  false  "發票文件 ID"
// @Param        job_id  query  string  false  "發票工作 ID"
// @Param        operation_type  query  string  false  "操作類型"
// @Param        date_from  query  string  false  "起始日期 YYYY-MM-DD"
// @Param        date_to  query  string  false  "結束日期 YYYY-MM-DD"
// @Param        page  query  string  false  "頁碼，數字字串 ≥ 1，預設 1"
// @Param        page_size  query  string  false  "每頁筆數，數字字串 1–50，預設 20（逾越一律 400，不夾住）"
// @Success      200  {object}  object{code=int,message=string,data=object{records=[]InvoiceAuditLogItem,total=string,page=string,pageSize=string}}
// @Failure      400  {object}  object{code=int,message=string}  "日期格式"
// @Failure      401  {object}  object{code=int,message=string}  "未登入或 JWT 無效"
// @Failure      403  {object}  object{code=int,message=string}  "非 admin 身分"
// @Failure      429  {object}  object{code=int,message=string}  "請求過於頻繁"
// @Failure      503  {object}  object{code=int,message=string}  "invoice-service 不可用或逾時"
// @Router       /admin/invoices/audit_logs [get]
func (h *Holder) ListInvoiceAuditLogsForAdmin(c *gin.Context) {
	accountID, ok := getAccountID(c)
	if !ok {
		response.Unauthorized(c)
		return
	}

	page, pageSize, ok := invoicePaging(c, invoiceAdminPageSizeStr)
	if !ok {
		return
	}

	resp, err := h.Invoice.ListInvoiceAuditLogsForAdmin(c.Request.Context(), &pb.ListInvoiceAuditLogsForAdminRequest{
		ActorAccountId:       accountID,
		TargetActorAccountId: c.Query("actor_account_id"),
		StoreId:              c.Query("store_id"),
		OrderNo:              c.Query("order_no"),
		DocumentId:           c.Query("document_id"),
		JobId:                c.Query("job_id"),
		OperationType:        c.Query("operation_type"),
		DateFrom:             c.Query("date_from"),
		DateTo:               c.Query("date_to"),
		Page:                 page,
		PageSize:             pageSize,
	})
	if err != nil {
		response.GrpcErr(c, "invoice", err)
		return
	}

	data := resp.GetData()
	records := make([]InvoiceAuditLogItem, 0, len(data.GetRecords()))
	for _, it := range data.GetRecords() {
		records = append(records, convInvInvoiceAuditLogItem(it))
	}
	// preserveData 業務碼不會轉成 err（interceptor 對它們 return nil），必須自己看 code。
	if resp.GetCode() != 0 {
		response.Biz(c, resp.GetCode(), resp.GetMessage(), response.PageData{
			Records:  records,
			Total:    data.GetTotal(),
			Page:     data.GetPage(),
			PageSize: data.GetPageSize(),
		})
		return
	}
	response.Page(c, records, data.GetTotal(), data.GetPage(), data.GetPageSize())
}

// ReactivateProviderAccountForAdmin ADM 恢復 AMEGO 帳號
// @Summary      ADM 恢復 AMEGO 帳號
// @Description  ADM 恢復 AMEGO 帳號（58 號設計書 §6.1 端點 66）
// @Tags         管理員 (Admin)
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        Idempotency-Key  header  string  true  "冪等鍵，^[A-Za-z0-9._:-]{8,128}$"
// @Param        store_id  path  string  true  "賣家 store_id"
// @Param        request  body  ReactivateProviderAccountForAdminRequest  true  "ADM 恢復 AMEGO 帳號"
// @Success      200  {object}  object{code=int,message=string,data=SellerProviderAccountData}
// @Failure      400  {object}  object{code=int,message=string}
// @Failure      401  {object}  object{code=int,message=string}  "未登入或 JWT 無效"
// @Failure      403  {object}  object{code=int,message=string}  "非 admin 身分"
// @Failure      404  {object}  object{code=int,message=string}  "9330"
// @Failure      409  {object}  object{code=int,message=string}  "9322 / 9332 / 9336 非 SUSPENDED / 9350 同鍵不同內容 / 9351 同鍵處理中"
// @Failure      429  {object}  object{code=int,message=string}  "請求過於頻繁"
// @Failure      503  {object}  object{code=int,message=string}  "invoice-service 不可用或逾時"
// @Router       /admin/invoices/provider_accounts/{store_id}/reactivations [post]
func (h *Holder) ReactivateProviderAccountForAdmin(c *gin.Context) {
	accountID, ok := getAccountID(c)
	if !ok {
		response.Unauthorized(c)
		return
	}

	storeId := c.Param("store_id")
	if storeId == "" {
		response.BadRequest(c, "store_id is required")
		return
	}

	var body ReactivateProviderAccountForAdminRequest
	if err := c.ShouldBindJSON(&body); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	resp, err := h.Invoice.ReactivateProviderAccountForAdmin(c.Request.Context(), &pb.ReactivateProviderAccountForAdminRequest{
		ActorAccountId:  accountID,
		StoreId:         storeId,
		Reason:          body.Reason,
		Confirmed:       body.Confirmed,
		ExpectedVersion: body.ExpectedVersion,
		IdempotencyKey:  c.GetHeader(middleware.IdempotencyKeyHeader),
	})
	if err != nil {
		response.GrpcErr(c, "invoice", err)
		return
	}

	data := convInvSellerProviderAccountData(resp.GetData())
	// §6.1.0(13) 的 preserveData 碼：EnvelopeGuardInterceptor 對它們不轉 error，
	// data 必須原樣透傳（HTTP 恆 200），否則前端拿不到導向用的 id／時間。
	if resp.GetCode() != 0 {
		response.Biz(c, resp.GetCode(), resp.GetMessage(), data)
		return
	}
	response.OK(c, data)
}

// RevokeProviderAccountForAdmin ADM 撤銷 AMEGO 帳號
// @Summary      ADM 撤銷 AMEGO 帳號
// @Description  ADM 撤銷 AMEGO 帳號（58 號設計書 §6.1 端點 67）
// @Tags         管理員 (Admin)
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        Idempotency-Key  header  string  true  "冪等鍵，^[A-Za-z0-9._:-]{8,128}$"
// @Param        store_id  path  string  true  "賣家 store_id"
// @Param        request  body  RevokeProviderAccountForAdminRequest  true  "ADM 撤銷 AMEGO 帳號"
// @Success      200  {object}  object{code=int,message=string,data=RevokeProviderAccountForAdminData}
// @Failure      400  {object}  object{code=int,message=string}
// @Failure      401  {object}  object{code=int,message=string}  "未登入或 JWT 無效"
// @Failure      403  {object}  object{code=int,message=string}  "非 admin 身分"
// @Failure      404  {object}  object{code=int,message=string}  "9330"
// @Failure      409  {object}  object{code=int,message=string}  "9322 / 9336 當前狀態不允許 / 9350 同鍵不同內容 / 9351 同鍵處理中"
// @Failure      429  {object}  object{code=int,message=string}  "請求過於頻繁"
// @Failure      503  {object}  object{code=int,message=string}  "invoice-service 不可用或逾時"
// @Router       /admin/invoices/provider_accounts/{store_id}/revocations [post]
func (h *Holder) RevokeProviderAccountForAdmin(c *gin.Context) {
	accountID, ok := getAccountID(c)
	if !ok {
		response.Unauthorized(c)
		return
	}

	storeId := c.Param("store_id")
	if storeId == "" {
		response.BadRequest(c, "store_id is required")
		return
	}

	var body RevokeProviderAccountForAdminRequest
	if err := c.ShouldBindJSON(&body); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	resp, err := h.Invoice.RevokeProviderAccountForAdmin(c.Request.Context(), &pb.RevokeProviderAccountForAdminRequest{
		ActorAccountId:  accountID,
		StoreId:         storeId,
		Reason:          body.Reason,
		Confirmed:       body.Confirmed,
		ExpectedVersion: body.ExpectedVersion,
		IdempotencyKey:  c.GetHeader(middleware.IdempotencyKeyHeader),
	})
	if err != nil {
		response.GrpcErr(c, "invoice", err)
		return
	}

	data := convInvRevokeProviderAccountForAdminData(resp.GetData())
	// preserveData 業務碼不會轉成 err（interceptor 對它們 return nil），必須自己看 code。
	if resp.GetCode() != 0 {
		response.Biz(c, resp.GetCode(), resp.GetMessage(), data)
		return
	}
	response.OK(c, data)
}
