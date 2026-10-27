package handler

import (
	"github.com/Will413028/hawkr/gateway/internal/middleware"
	"github.com/Will413028/hawkr/gateway/internal/response"
	pb "github.com/Will413028/hawkr/go-pkg/proto/invoice"
	"github.com/gin-gonic/gin"
)

// 58 號《電子發票服務》買家面 handler（§6.1.1–§6.1.6）。
//
// 身分：一律 h.resolveBuyerID（44 號的營運鍵 buyer_id，不是 account_id）。越權與查無
// 走同一個出口（Tier A，§6.1.0(5)）——那個判定在 invoice-service，Gateway 只負責把
// 正確的 buyer_id 送下去；送 account_id 下去就等於把述詞建在錯的鍵上。

// ListBuyerInvoiceProfiles 買家常用發票資料列表
// @Summary      買家常用發票資料列表
// @Description  買家常用發票資料列表（58 號設計書 §6.1 端點 1）
// @Tags         電子發票 (Invoice)
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        page  query  string  false  "頁碼，數字字串 ≥ 1，預設 1"
// @Param        page_size  query  string  false  "每頁筆數，數字字串 1–50，預設 10（逾越一律 400，不夾住）"
// @Success      200  {object}  object{code=int,message=string,data=object{records=[]InvoiceProfileInfo,total=string,page=string,pageSize=string}}
// @Failure      400  {object}  object{code=int,message=string}  "參數格式錯誤"
// @Failure      401  {object}  object{code=int,message=string}  "未登入或 JWT 無效"
// @Failure      429  {object}  object{code=int,message=string}  "請求過於頻繁"
// @Failure      503  {object}  object{code=int,message=string}  "invoice-service 不可用或逾時"
// @Router       /app/buyers/me/invoice_profiles [get]
func (h *Holder) ListBuyerInvoiceProfiles(c *gin.Context) {
	buyerID, ok := h.resolveBuyerID(c)
	if !ok {
		return
	}

	page, pageSize, ok := invoicePaging(c, defaultPageSizeStr)
	if !ok {
		return
	}

	resp, err := h.Invoice.ListBuyerInvoiceProfiles(c.Request.Context(), &pb.ListBuyerInvoiceProfilesRequest{
		BuyerId:  int64(buyerID),
		Page:     page,
		PageSize: pageSize,
	})
	if err != nil {
		response.GrpcErr(c, "invoice", err)
		return
	}

	data := resp.GetData()
	records := make([]InvoiceProfileInfo, 0, len(data.GetRecords()))
	for _, it := range data.GetRecords() {
		records = append(records, convInvInvoiceProfileInfo(it))
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

// CreateBuyerInvoiceProfile 新增買家常用發票資料
// @Summary      新增買家常用發票資料
// @Description  新增買家常用發票資料（58 號設計書 §6.1 端點 2）
// @Tags         電子發票 (Invoice)
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        Idempotency-Key  header  string  true  "冪等鍵，^[A-Za-z0-9._:-]{8,128}$"
// @Param        request  body  CreateBuyerInvoiceProfileRequest  true  "新增買家常用發票資料"
// @Success      200  {object}  object{code=int,message=string,data=InvoiceProfileInfo}
// @Failure      400  {object}  object{code=int,message=string}
// @Failure      401  {object}  object{code=int,message=string}  "未登入或 JWT 無效"
// @Failure      409  {object}  object{code=int,message=string}  "9303 筆數上限 / 9305 格式 / 9306 互斥 / 9350 同鍵不同內容 / 9351 同鍵處理中"
// @Failure      429  {object}  object{code=int,message=string}  "請求過於頻繁"
// @Failure      503  {object}  object{code=int,message=string}  "invoice-service 不可用或逾時"
// @Router       /app/buyers/me/invoice_profiles [post]
func (h *Holder) CreateBuyerInvoiceProfile(c *gin.Context) {
	buyerID, ok := h.resolveBuyerID(c)
	if !ok {
		return
	}

	var body CreateBuyerInvoiceProfileRequest
	if err := c.ShouldBindJSON(&body); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	resp, err := h.Invoice.CreateBuyerInvoiceProfile(c.Request.Context(), &pb.CreateBuyerInvoiceProfileRequest{
		BuyerId:        int64(buyerID),
		CarrierType:    body.CarrierType,
		Value:          body.Value,
		BuyerTaxId:     body.BuyerTaxId,
		BuyerName:      body.BuyerName,
		Label:          body.Label,
		SetDefault:     body.SetDefault,
		IdempotencyKey: c.GetHeader(middleware.IdempotencyKeyHeader),
	})
	if err != nil {
		response.GrpcErr(c, "invoice", err)
		return
	}

	data := convInvInvoiceProfileInfo(resp.GetData())
	// preserveData 業務碼不會轉成 err（interceptor 對它們 return nil），必須自己看 code。
	if resp.GetCode() != 0 {
		response.Biz(c, resp.GetCode(), resp.GetMessage(), data)
		return
	}
	response.OK(c, data)
}

// UpdateBuyerInvoiceProfile 更新買家常用發票資料
// @Summary      更新買家常用發票資料
// @Description  更新買家常用發票資料（58 號設計書 §6.1 端點 3）
// @Tags         電子發票 (Invoice)
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        Idempotency-Key  header  string  true  "冪等鍵，^[A-Za-z0-9._:-]{8,128}$"
// @Param        profile_id  path  string  true  "常用發票資料 ID"
// @Param        request  body  UpdateBuyerInvoiceProfileRequest  true  "更新買家常用發票資料"
// @Success      200  {object}  object{code=int,message=string,data=InvoiceProfileInfo}
// @Failure      400  {object}  object{code=int,message=string}
// @Failure      401  {object}  object{code=int,message=string}  "未登入或 JWT 無效"
// @Failure      404  {object}  object{code=int,message=string}  "9302（Tier A）"
// @Failure      409  {object}  object{code=int,message=string}  "9305 / 9306 / 9350 同鍵不同內容 / 9351 同鍵處理中"
// @Failure      429  {object}  object{code=int,message=string}  "請求過於頻繁"
// @Failure      503  {object}  object{code=int,message=string}  "invoice-service 不可用或逾時"
// @Router       /app/buyers/me/invoice_profiles/{profile_id} [put]
func (h *Holder) UpdateBuyerInvoiceProfile(c *gin.Context) {
	buyerID, ok := h.resolveBuyerID(c)
	if !ok {
		return
	}

	profileId := c.Param("profile_id")
	if profileId == "" {
		response.BadRequest(c, "profile_id is required")
		return
	}

	var body UpdateBuyerInvoiceProfileRequest
	if err := c.ShouldBindJSON(&body); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	resp, err := h.Invoice.UpdateBuyerInvoiceProfile(c.Request.Context(), &pb.UpdateBuyerInvoiceProfileRequest{
		BuyerId:        int64(buyerID),
		ProfileId:      profileId,
		Value:          body.Value,
		BuyerTaxId:     body.BuyerTaxId,
		BuyerName:      body.BuyerName,
		Label:          body.Label,
		IdempotencyKey: c.GetHeader(middleware.IdempotencyKeyHeader),
	})
	if err != nil {
		response.GrpcErr(c, "invoice", err)
		return
	}

	data := convInvInvoiceProfileInfo(resp.GetData())
	// preserveData 業務碼不會轉成 err（interceptor 對它們 return nil），必須自己看 code。
	if resp.GetCode() != 0 {
		response.Biz(c, resp.GetCode(), resp.GetMessage(), data)
		return
	}
	response.OK(c, data)
}

// DeleteBuyerInvoiceProfile 刪除買家常用發票資料
// @Summary      刪除買家常用發票資料
// @Description  刪除買家常用發票資料（58 號設計書 §6.1 端點 4）
// @Tags         電子發票 (Invoice)
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        Idempotency-Key  header  string  true  "冪等鍵，^[A-Za-z0-9._:-]{8,128}$"
// @Param        profile_id  path  string  true  "常用發票資料 ID"
// @Success      200  {object}  object{code=int,message=string,data=object{deleted=bool}}
// @Failure      401  {object}  object{code=int,message=string}  "未登入或 JWT 無效"
// @Failure      404  {object}  object{code=int,message=string}  "9302（Tier A）"
// @Failure      409  {object}  object{code=int,message=string}  "9304 須先指定新預設 / 9350 同鍵不同內容 / 9351 同鍵處理中"
// @Failure      429  {object}  object{code=int,message=string}  "請求過於頻繁"
// @Failure      503  {object}  object{code=int,message=string}  "invoice-service 不可用或逾時"
// @Router       /app/buyers/me/invoice_profiles/{profile_id} [delete]
func (h *Holder) DeleteBuyerInvoiceProfile(c *gin.Context) {
	buyerID, ok := h.resolveBuyerID(c)
	if !ok {
		return
	}

	profileId := c.Param("profile_id")
	if profileId == "" {
		response.BadRequest(c, "profile_id is required")
		return
	}

	resp, err := h.Invoice.DeleteBuyerInvoiceProfile(c.Request.Context(), &pb.DeleteBuyerInvoiceProfileRequest{
		BuyerId:        int64(buyerID),
		ProfileId:      profileId,
		IdempotencyKey: c.GetHeader(middleware.IdempotencyKeyHeader),
	})
	if err != nil {
		response.GrpcErr(c, "invoice", err)
		return
	}

	data := DeleteBuyerInvoiceProfileData{Deleted: resp.GetDeleted()}
	// preserveData 業務碼不會轉成 err（interceptor 對它們 return nil），必須自己看 code。
	if resp.GetCode() != 0 {
		response.Biz(c, resp.GetCode(), resp.GetMessage(), data)
		return
	}
	response.OK(c, data)
}

// SetDefaultBuyerInvoiceProfile 設為預設的常用發票資料
// @Summary      設為預設的常用發票資料
// @Description  設為預設的常用發票資料（58 號設計書 §6.1 端點 5）
// @Tags         電子發票 (Invoice)
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        Idempotency-Key  header  string  true  "冪等鍵，^[A-Za-z0-9._:-]{8,128}$"
// @Param        profile_id  path  string  true  "常用發票資料 ID"
// @Success      200  {object}  object{code=int,message=string,data=InvoiceProfileInfo}
// @Failure      401  {object}  object{code=int,message=string}  "未登入或 JWT 無效"
// @Failure      404  {object}  object{code=int,message=string}  "9302（Tier A）"
// @Failure      409  {object}  object{code=int,message=string}  "9350 同鍵不同內容 / 9351 同鍵處理中"
// @Failure      429  {object}  object{code=int,message=string}  "請求過於頻繁"
// @Failure      503  {object}  object{code=int,message=string}  "invoice-service 不可用或逾時"
// @Router       /app/buyers/me/invoice_profiles/{profile_id}/default [post]
func (h *Holder) SetDefaultBuyerInvoiceProfile(c *gin.Context) {
	buyerID, ok := h.resolveBuyerID(c)
	if !ok {
		return
	}

	profileId := c.Param("profile_id")
	if profileId == "" {
		response.BadRequest(c, "profile_id is required")
		return
	}

	resp, err := h.Invoice.SetDefaultBuyerInvoiceProfile(c.Request.Context(), &pb.SetDefaultBuyerInvoiceProfileRequest{
		BuyerId:        int64(buyerID),
		ProfileId:      profileId,
		IdempotencyKey: c.GetHeader(middleware.IdempotencyKeyHeader),
	})
	if err != nil {
		response.GrpcErr(c, "invoice", err)
		return
	}

	data := convInvInvoiceProfileInfo(resp.GetData())
	// preserveData 業務碼不會轉成 err（interceptor 對它們 return nil），必須自己看 code。
	if resp.GetCode() != 0 {
		response.Biz(c, resp.GetCode(), resp.GetMessage(), data)
		return
	}
	response.OK(c, data)
}

// GetOrderInvoice 查訂單的發票總覽
// @Summary      查訂單的發票總覽
// @Description  查訂單的發票總覽（58 號設計書 §6.1 端點 8）
// @Tags         電子發票 (Invoice)
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        order_no  path  string  true  "訂單編號"
// @Success      200  {object}  object{code=int,message=string,data=OrderInvoiceData}
// @Failure      401  {object}  object{code=int,message=string}  "未登入或 JWT 無效"
// @Failure      404  {object}  object{code=int,message=string}  "9301（Tier A）"
// @Failure      429  {object}  object{code=int,message=string}  "請求過於頻繁"
// @Failure      503  {object}  object{code=int,message=string}  "invoice-service 不可用或逾時"
// @Router       /app/orders/{order_no}/invoice [get]
func (h *Holder) GetOrderInvoice(c *gin.Context) {
	buyerID, ok := h.resolveBuyerID(c)
	if !ok {
		return
	}

	orderNo := c.Param("order_no")
	if orderNo == "" {
		response.BadRequest(c, "order_no is required")
		return
	}

	resp, err := h.Invoice.GetOrderInvoice(c.Request.Context(), &pb.GetOrderInvoiceRequest{
		BuyerId: int64(buyerID),
		OrderNo: orderNo,
	})
	if err != nil {
		response.GrpcErr(c, "invoice", err)
		return
	}

	data := convInvOrderInvoiceData(resp.GetData())
	// preserveData 業務碼不會轉成 err（interceptor 對它們 return nil），必須自己看 code。
	if resp.GetCode() != 0 {
		response.Biz(c, resp.GetCode(), resp.GetMessage(), data)
		return
	}
	response.OK(c, data)
}

// GetOrderInvoiceDocument 查訂單的發票文件詳情
// @Summary      查訂單的發票文件詳情
// @Description  查訂單的發票文件詳情（58 號設計書 §6.1 端點 9）
// @Tags         電子發票 (Invoice)
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        order_no  path  string  true  "訂單編號"
// @Param        document_id  path  string  true  "發票文件 ID"
// @Success      200  {object}  object{code=int,message=string,data=InvoiceDocumentDetail}
// @Failure      401  {object}  object{code=int,message=string}  "未登入或 JWT 無效"
// @Failure      404  {object}  object{code=int,message=string}  "9301（Tier A）"
// @Failure      429  {object}  object{code=int,message=string}  "請求過於頻繁"
// @Failure      503  {object}  object{code=int,message=string}  "invoice-service 不可用或逾時"
// @Router       /app/orders/{order_no}/invoice_documents/{document_id} [get]
func (h *Holder) GetOrderInvoiceDocument(c *gin.Context) {
	buyerID, ok := h.resolveBuyerID(c)
	if !ok {
		return
	}

	orderNo := c.Param("order_no")
	if orderNo == "" {
		response.BadRequest(c, "order_no is required")
		return
	}
	documentId := c.Param("document_id")
	if documentId == "" {
		response.BadRequest(c, "document_id is required")
		return
	}

	resp, err := h.Invoice.GetInvoiceDocument(c.Request.Context(), &pb.GetInvoiceDocumentRequest{
		BuyerId:    int64(buyerID),
		OrderNo:    orderNo,
		DocumentId: documentId,
	})
	if err != nil {
		response.GrpcErr(c, "invoice", err)
		return
	}

	data := convInvInvoiceDocumentDetail(resp.GetData())
	// preserveData 業務碼不會轉成 err（interceptor 對它們 return nil），必須自己看 code。
	if resp.GetCode() != 0 {
		response.Biz(c, resp.GetCode(), resp.GetMessage(), data)
		return
	}
	response.OK(c, data)
}

// GetOrderInvoiceDocumentFile 取得訂單發票 PDF 的下載資訊
// @Summary      取得訂單發票 PDF 的下載資訊
// @Description  取得訂單發票 PDF 的下載資訊（58 號設計書 §6.1 端點 10）
// @Tags         電子發票 (Invoice)
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        order_no  path  string  true  "訂單編號"
// @Param        document_id  path  string  true  "發票文件 ID"
// @Param        style  query  string  false  "PDF 樣式，白名單 0/1/2/3/5"
// @Success      200  {object}  object{code=int,message=string,data=InvoiceFileData}
// @Failure      400  {object}  object{code=int,message=string}  "參數格式錯誤"
// @Failure      401  {object}  object{code=int,message=string}  "未登入或 JWT 無效"
// @Failure      404  {object}  object{code=int,message=string}  "9301（Tier A）"
// @Failure      409  {object}  object{code=int,message=string}  "9312 載具未中獎 / 9313 逾查詢期限"
// @Failure      429  {object}  object{code=int,message=string}  "請求過於頻繁"
// @Failure      503  {object}  object{code=int,message=string}  "invoice-service 不可用或逾時"
// @Router       /app/orders/{order_no}/invoice_documents/{document_id}/file [get]
func (h *Holder) GetOrderInvoiceDocumentFile(c *gin.Context) {
	buyerID, ok := h.resolveBuyerID(c)
	if !ok {
		return
	}

	orderNo := c.Param("order_no")
	if orderNo == "" {
		response.BadRequest(c, "order_no is required")
		return
	}
	documentId := c.Param("document_id")
	if documentId == "" {
		response.BadRequest(c, "document_id is required")
		return
	}

	resp, err := h.Invoice.GetInvoiceDocumentFile(c.Request.Context(), &pb.GetInvoiceDocumentFileRequest{
		BuyerId:    int64(buyerID),
		OrderNo:    orderNo,
		DocumentId: documentId,
		Style:      c.Query("style"),
	})
	if err != nil {
		response.GrpcErr(c, "invoice", err)
		return
	}

	data := convInvInvoiceFileData(resp.GetData())
	// preserveData 業務碼不會轉成 err（interceptor 對它們 return nil），必須自己看 code。
	if resp.GetCode() != 0 {
		response.Biz(c, resp.GetCode(), resp.GetMessage(), data)
		return
	}
	response.OK(c, data)
}

// ListBuyerInvoiceDocuments 買家發票中心列表
// @Summary      買家發票中心列表
// @Description  買家發票中心列表（58 號設計書 §6.1 端點 68）
// @Tags         電子發票 (Invoice)
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        date_from  query  string  false  "起始日期 YYYY-MM-DD"
// @Param        date_to  query  string  false  "結束日期 YYYY-MM-DD"
// @Param        status  query  string  false  "狀態篩選"
// @Param        document_kind  query  string  false  "文件種類 B2C/B2B/RECEIPT"
// @Param        page  query  string  false  "頁碼，數字字串 ≥ 1，預設 1"
// @Param        page_size  query  string  false  "每頁筆數，數字字串 1–50，預設 10（逾越一律 400，不夾住）"
// @Success      200  {object}  object{code=int,message=string,data=object{records=[]BuyerInvoiceDocumentItem,total=string,page=string,pageSize=string}}
// @Failure      400  {object}  object{code=int,message=string}  "日期格式"
// @Failure      401  {object}  object{code=int,message=string}  "未登入或 JWT 無效"
// @Failure      429  {object}  object{code=int,message=string}  "請求過於頻繁"
// @Failure      503  {object}  object{code=int,message=string}  "invoice-service 不可用或逾時"
// @Router       /app/invoices/documents [get]
func (h *Holder) ListBuyerInvoiceDocuments(c *gin.Context) {
	buyerID, ok := h.resolveBuyerID(c)
	if !ok {
		return
	}

	page, pageSize, ok := invoicePaging(c, defaultPageSizeStr)
	if !ok {
		return
	}

	resp, err := h.Invoice.ListBuyerInvoiceDocuments(c.Request.Context(), &pb.ListBuyerInvoiceDocumentsRequest{
		BuyerId:      int64(buyerID),
		DateFrom:     c.Query("date_from"),
		DateTo:       c.Query("date_to"),
		Status:       c.Query("status"),
		DocumentKind: c.Query("document_kind"),
		Page:         page,
		PageSize:     pageSize,
	})
	if err != nil {
		response.GrpcErr(c, "invoice", err)
		return
	}

	data := resp.GetData()
	records := make([]BuyerInvoiceDocumentItem, 0, len(data.GetRecords()))
	for _, it := range data.GetRecords() {
		records = append(records, convInvBuyerInvoiceDocumentItem(it))
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

// GetBuyerInvoiceDocument 查發票文件詳情（依 document_id）
// @Summary      查發票文件詳情（依 document_id）
// @Description  查發票文件詳情（依 document_id）（58 號設計書 §6.1 端點 69）
// @Tags         電子發票 (Invoice)
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        document_id  path  string  true  "發票文件 ID"
// @Success      200  {object}  object{code=int,message=string,data=InvoiceDocumentDetail}
// @Failure      401  {object}  object{code=int,message=string}  "未登入或 JWT 無效"
// @Failure      404  {object}  object{code=int,message=string}  "9301（Tier A）"
// @Failure      429  {object}  object{code=int,message=string}  "請求過於頻繁"
// @Failure      503  {object}  object{code=int,message=string}  "invoice-service 不可用或逾時"
// @Router       /app/invoices/documents/{document_id} [get]
func (h *Holder) GetBuyerInvoiceDocument(c *gin.Context) {
	buyerID, ok := h.resolveBuyerID(c)
	if !ok {
		return
	}

	documentId := c.Param("document_id")
	if documentId == "" {
		response.BadRequest(c, "document_id is required")
		return
	}

	resp, err := h.Invoice.GetBuyerInvoiceDocument(c.Request.Context(), &pb.GetBuyerInvoiceDocumentRequest{
		BuyerId:    int64(buyerID),
		DocumentId: documentId,
	})
	if err != nil {
		response.GrpcErr(c, "invoice", err)
		return
	}

	data := convInvInvoiceDocumentDetail(resp.GetData())
	// preserveData 業務碼不會轉成 err（interceptor 對它們 return nil），必須自己看 code。
	if resp.GetCode() != 0 {
		response.Biz(c, resp.GetCode(), resp.GetMessage(), data)
		return
	}
	response.OK(c, data)
}

// GetBuyerInvoiceDocumentFile 取得發票 PDF 的下載資訊（依 document_id）
// @Summary      取得發票 PDF 的下載資訊（依 document_id）
// @Description  取得發票 PDF 的下載資訊（依 document_id）（58 號設計書 §6.1 端點 70）
// @Tags         電子發票 (Invoice)
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        document_id  path  string  true  "發票文件 ID"
// @Param        style  query  string  false  "PDF 樣式，白名單 0/1/2/3/5"
// @Success      200  {object}  object{code=int,message=string,data=InvoiceFileData}
// @Failure      400  {object}  object{code=int,message=string}  "參數格式錯誤"
// @Failure      401  {object}  object{code=int,message=string}  "未登入或 JWT 無效"
// @Failure      404  {object}  object{code=int,message=string}  "9301"
// @Failure      409  {object}  object{code=int,message=string}  "9312 / 9313"
// @Failure      429  {object}  object{code=int,message=string}  "請求過於頻繁"
// @Failure      503  {object}  object{code=int,message=string}  "invoice-service 不可用或逾時"
// @Router       /app/invoices/documents/{document_id}/file [get]
func (h *Holder) GetBuyerInvoiceDocumentFile(c *gin.Context) {
	buyerID, ok := h.resolveBuyerID(c)
	if !ok {
		return
	}

	documentId := c.Param("document_id")
	if documentId == "" {
		response.BadRequest(c, "document_id is required")
		return
	}

	resp, err := h.Invoice.GetInvoiceDocumentFile(c.Request.Context(), &pb.GetInvoiceDocumentFileRequest{
		BuyerId:    int64(buyerID),
		OrderNo:    "", /* 端點 70 是 document-scoped：路由上沒有 order_no，共用的 GetInvoiceDocumentFile 卻是 order-scoped。服務端必須改以 buyer_id × document_id 判定擁有者 */
		DocumentId: documentId,
		Style:      c.Query("style"),
	})
	if err != nil {
		response.GrpcErr(c, "invoice", err)
		return
	}

	data := convInvInvoiceFileData(resp.GetData())
	// preserveData 業務碼不會轉成 err（interceptor 對它們 return nil），必須自己看 code。
	if resp.GetCode() != 0 {
		response.Biz(c, resp.GetCode(), resp.GetMessage(), data)
		return
	}
	response.OK(c, data)
}

// ResendBuyerInvoiceNotification 買家重寄發票通知
// @Summary      買家重寄發票通知
// @Description  買家重寄發票通知（58 號設計書 §6.1 端點 71）
// @Tags         電子發票 (Invoice)
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        Idempotency-Key  header  string  true  "冪等鍵，^[A-Za-z0-9._:-]{8,128}$"
// @Param        document_id  path  string  true  "發票文件 ID"
// @Success      200  {object}  object{code=int,message=string,data=ResendInvoiceNotificationData}
// @Failure      400  {object}  object{code=int,message=string}  "參數格式錯誤"
// @Failure      401  {object}  object{code=int,message=string}  "未登入或 JWT 無效"
// @Failure      404  {object}  object{code=int,message=string}  "9301"
// @Failure      409  {object}  object{code=int,message=string}  "9350 同鍵不同內容 / 9351 同鍵處理中"
// @Failure      429  {object}  object{code=int,message=string}  "9352 十分鐘內已寄送過"
// @Failure      503  {object}  object{code=int,message=string}  "invoice-service 不可用或逾時"
// @Router       /app/invoices/documents/{document_id}/notification_resends [post]
func (h *Holder) ResendBuyerInvoiceNotification(c *gin.Context) {
	buyerID, ok := h.resolveBuyerID(c)
	if !ok {
		return
	}

	documentId := c.Param("document_id")
	if documentId == "" {
		response.BadRequest(c, "document_id is required")
		return
	}

	// 買家專屬 RPC（§5.1 角色分離）。賣家版帶 store_id、本版帶 buyer_id——
	// 兩者的擁有者述詞不同，而 §8.1.4 的 Tier A 要求擁有者必須進服務端的 WHERE。
	resp, err := h.Invoice.ResendBuyerInvoiceNotification(c.Request.Context(), &pb.ResendBuyerInvoiceNotificationRequest{
		BuyerId:        int64(buyerID),
		DocumentId:     documentId,
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

// CreateInvoiceCorrection 買家申請修改發票資料
// @Summary      買家申請修改發票資料
// @Description  買家申請修改發票資料（58 號設計書 §6.1 端點 11）
// @Tags         電子發票 (Invoice)
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        Idempotency-Key  header  string  true  "冪等鍵，^[A-Za-z0-9._:-]{8,128}$"
// @Param        order_no  path  string  true  "訂單編號"
// @Param        request  body  CreateInvoiceCorrectionRequest  true  "買家申請修改發票資料"
// @Success      200  {object}  object{code=int,message=string,data=InvoiceCorrectionData}
// @Failure      400  {object}  object{code=int,message=string}
// @Failure      401  {object}  object{code=int,message=string}  "未登入或 JWT 無效"
// @Failure      404  {object}  object{code=int,message=string}  "9301"
// @Failure      409  {object}  object{code=int,message=string}  "9305 / 9306 / 9307 / 9308 / 9351 同鍵處理中 / 9350 同鍵不同內容"
// @Failure      429  {object}  object{code=int,message=string}  "請求過於頻繁"
// @Failure      503  {object}  object{code=int,message=string}  "invoice-service 不可用或逾時"
// @Router       /app/orders/{order_no}/invoice_corrections [post]
func (h *Holder) CreateInvoiceCorrection(c *gin.Context) {
	buyerID, ok := h.resolveBuyerID(c)
	if !ok {
		return
	}

	orderNo := c.Param("order_no")
	if orderNo == "" {
		response.BadRequest(c, "order_no is required")
		return
	}

	var body CreateInvoiceCorrectionRequest
	if err := c.ShouldBindJSON(&body); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	resp, err := h.Invoice.CreateInvoiceCorrection(c.Request.Context(), &pb.CreateInvoiceCorrectionRequest{
		BuyerId:                 int64(buyerID),
		OrderNo:                 orderNo,
		ExpectedSnapshotVersion: body.ExpectedSnapshotVersion,
		NewInvoice:              toPbInvInvoiceInputSpec(body.NewInvoice),
		Reason:                  body.Reason,
		IdempotencyKey:          c.GetHeader(middleware.IdempotencyKeyHeader),
	})
	if err != nil {
		response.GrpcErr(c, "invoice", err)
		return
	}

	data := convInvInvoiceCorrectionData(resp.GetData())
	// preserveData 業務碼不會轉成 err（interceptor 對它們 return nil），必須自己看 code。
	if resp.GetCode() != 0 {
		response.Biz(c, resp.GetCode(), resp.GetMessage(), data)
		return
	}
	response.OK(c, data)
}

// GetInvoiceCorrection 查發票修改申請
// @Summary      查發票修改申請
// @Description  查發票修改申請（58 號設計書 §6.1 端點 12）
// @Tags         電子發票 (Invoice)
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        order_no  path  string  true  "訂單編號"
// @Param        correction_id  path  string  true  "發票修改申請 ID"
// @Success      200  {object}  object{code=int,message=string,data=InvoiceCorrectionData}
// @Failure      401  {object}  object{code=int,message=string}  "未登入或 JWT 無效"
// @Failure      404  {object}  object{code=int,message=string}  "9301"
// @Failure      429  {object}  object{code=int,message=string}  "請求過於頻繁"
// @Failure      503  {object}  object{code=int,message=string}  "invoice-service 不可用或逾時"
// @Router       /app/orders/{order_no}/invoice_corrections/{correction_id} [get]
func (h *Holder) GetInvoiceCorrection(c *gin.Context) {
	buyerID, ok := h.resolveBuyerID(c)
	if !ok {
		return
	}

	orderNo := c.Param("order_no")
	if orderNo == "" {
		response.BadRequest(c, "order_no is required")
		return
	}
	correctionId := c.Param("correction_id")
	if correctionId == "" {
		response.BadRequest(c, "correction_id is required")
		return
	}

	resp, err := h.Invoice.GetInvoiceCorrection(c.Request.Context(), &pb.GetInvoiceCorrectionRequest{
		BuyerId:      int64(buyerID),
		OrderNo:      orderNo,
		CorrectionId: correctionId,
	})
	if err != nil {
		response.GrpcErr(c, "invoice", err)
		return
	}

	data := convInvInvoiceCorrectionData(resp.GetData())
	// preserveData 業務碼不會轉成 err（interceptor 對它們 return nil），必須自己看 code。
	if resp.GetCode() != 0 {
		response.Biz(c, resp.GetCode(), resp.GetMessage(), data)
		return
	}
	response.OK(c, data)
}

// CreateInvoiceComplaint 買家提出「未收到發票」申訴
// @Summary      買家提出「未收到發票」申訴
// @Description  買家提出「未收到發票」申訴（58 號設計書 §6.1 端點 13）
// @Tags         電子發票 (Invoice)
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        Idempotency-Key  header  string  true  "冪等鍵，^[A-Za-z0-9._:-]{8,128}$"
// @Param        order_no  path  string  true  "訂單編號"
// @Param        request  body  CreateInvoiceComplaintRequest  true  "買家提出「未收到發票」申訴"
// @Success      200  {object}  object{code=int,message=string,data=InvoiceComplaintInfo}
// @Failure      400  {object}  object{code=int,message=string}
// @Failure      401  {object}  object{code=int,message=string}  "未登入或 JWT 無效"
// @Failure      404  {object}  object{code=int,message=string}  "9301"
// @Failure      409  {object}  object{code=int,message=string}  "9311 尚未開放 / 9350 同鍵不同內容 / 9351 同鍵處理中"
// @Failure      429  {object}  object{code=int,message=string}  "請求過於頻繁"
// @Failure      503  {object}  object{code=int,message=string}  "invoice-service 不可用或逾時"
// @Router       /app/orders/{order_no}/invoice_complaints [post]
func (h *Holder) CreateInvoiceComplaint(c *gin.Context) {
	buyerID, ok := h.resolveBuyerID(c)
	if !ok {
		return
	}

	orderNo := c.Param("order_no")
	if orderNo == "" {
		response.BadRequest(c, "order_no is required")
		return
	}

	var body CreateInvoiceComplaintRequest
	if err := c.ShouldBindJSON(&body); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	resp, err := h.Invoice.CreateInvoiceComplaint(c.Request.Context(), &pb.CreateInvoiceComplaintRequest{
		BuyerId:        int64(buyerID),
		OrderNo:        orderNo,
		ReasonCode:     body.ReasonCode,
		Description:    body.Description,
		IdempotencyKey: c.GetHeader(middleware.IdempotencyKeyHeader),
	})
	if err != nil {
		response.GrpcErr(c, "invoice", err)
		return
	}

	data := convInvInvoiceComplaintInfo(resp.GetData())
	// §6.1.0(13) 的 preserveData 碼：EnvelopeGuardInterceptor 對它們不轉 error，
	// data 必須原樣透傳（HTTP 恆 200），否則前端拿不到導向用的 id／時間。
	if resp.GetCode() != 0 {
		response.Biz(c, resp.GetCode(), resp.GetMessage(), data)
		return
	}
	response.OK(c, data)
}

// ListOrderInvoiceComplaints 查本訂單的申訴列表
// @Summary      查本訂單的申訴列表
// @Description  查本訂單的申訴列表（58 號設計書 §6.1 端點 14）
// @Tags         電子發票 (Invoice)
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        order_no  path  string  true  "訂單編號"
// @Param        page  query  string  false  "頁碼，數字字串 ≥ 1，預設 1"
// @Param        page_size  query  string  false  "每頁筆數，數字字串 1–50，預設 10（逾越一律 400，不夾住）"
// @Success      200  {object}  object{code=int,message=string,data=object{records=[]InvoiceComplaintInfo,total=string,page=string,pageSize=string}}
// @Failure      400  {object}  object{code=int,message=string}  "參數格式錯誤"
// @Failure      401  {object}  object{code=int,message=string}  "未登入或 JWT 無效"
// @Failure      404  {object}  object{code=int,message=string}  "9301"
// @Failure      429  {object}  object{code=int,message=string}  "請求過於頻繁"
// @Failure      503  {object}  object{code=int,message=string}  "invoice-service 不可用或逾時"
// @Router       /app/orders/{order_no}/invoice_complaints [get]
func (h *Holder) ListOrderInvoiceComplaints(c *gin.Context) {
	buyerID, ok := h.resolveBuyerID(c)
	if !ok {
		return
	}

	orderNo := c.Param("order_no")
	if orderNo == "" {
		response.BadRequest(c, "order_no is required")
		return
	}

	page, pageSize, ok := invoicePaging(c, defaultPageSizeStr)
	if !ok {
		return
	}

	resp, err := h.Invoice.ListOrderInvoiceComplaints(c.Request.Context(), &pb.ListOrderInvoiceComplaintsRequest{
		BuyerId:  int64(buyerID),
		OrderNo:  orderNo,
		Page:     page,
		PageSize: pageSize,
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

// GetInvoiceComplaint 查申訴詳情
// @Summary      查申訴詳情
// @Description  查申訴詳情（58 號設計書 §6.1 端點 15）
// @Tags         電子發票 (Invoice)
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        order_no  path  string  true  "訂單編號"
// @Param        complaint_id  path  string  true  "申訴案件 ID"
// @Success      200  {object}  object{code=int,message=string,data=InvoiceComplaintInfo}
// @Failure      401  {object}  object{code=int,message=string}  "未登入或 JWT 無效"
// @Failure      404  {object}  object{code=int,message=string}  "9309（Tier A）"
// @Failure      429  {object}  object{code=int,message=string}  "請求過於頻繁"
// @Failure      503  {object}  object{code=int,message=string}  "invoice-service 不可用或逾時"
// @Router       /app/orders/{order_no}/invoice_complaints/{complaint_id} [get]
func (h *Holder) GetInvoiceComplaint(c *gin.Context) {
	buyerID, ok := h.resolveBuyerID(c)
	if !ok {
		return
	}

	orderNo := c.Param("order_no")
	if orderNo == "" {
		response.BadRequest(c, "order_no is required")
		return
	}
	complaintId := c.Param("complaint_id")
	if complaintId == "" {
		response.BadRequest(c, "complaint_id is required")
		return
	}

	resp, err := h.Invoice.GetInvoiceComplaint(c.Request.Context(), &pb.GetInvoiceComplaintRequest{
		BuyerId:     int64(buyerID),
		OrderNo:     orderNo,
		ComplaintId: complaintId,
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

// RejectInvoiceComplaintResponse 買家駁回賣家的申訴回覆
// @Summary      買家駁回賣家的申訴回覆
// @Description  買家駁回賣家的申訴回覆（58 號設計書 §6.1 端點 63）
// @Tags         電子發票 (Invoice)
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        Idempotency-Key  header  string  true  "冪等鍵，^[A-Za-z0-9._:-]{8,128}$"
// @Param        order_no  path  string  true  "訂單編號"
// @Param        complaint_id  path  string  true  "申訴案件 ID"
// @Param        request  body  RejectInvoiceComplaintResponseRequest  true  "買家駁回賣家的申訴回覆"
// @Success      200  {object}  object{code=int,message=string,data=InvoiceComplaintInfo}
// @Failure      400  {object}  object{code=int,message=string}  "reason 缺或超長"
// @Failure      401  {object}  object{code=int,message=string}  "未登入或 JWT 無效"
// @Failure      404  {object}  object{code=int,message=string}  "9309（Tier A）"
// @Failure      409  {object}  object{code=int,message=string}  "9322 / 9336 案件當前狀態不允許 / 9350 同鍵不同內容 / 9351 同鍵處理中"
// @Failure      429  {object}  object{code=int,message=string}  "請求過於頻繁"
// @Failure      503  {object}  object{code=int,message=string}  "invoice-service 不可用或逾時"
// @Router       /app/orders/{order_no}/invoice_complaints/{complaint_id}/rejections [post]
func (h *Holder) RejectInvoiceComplaintResponse(c *gin.Context) {
	buyerID, ok := h.resolveBuyerID(c)
	if !ok {
		return
	}

	orderNo := c.Param("order_no")
	if orderNo == "" {
		response.BadRequest(c, "order_no is required")
		return
	}
	complaintId := c.Param("complaint_id")
	if complaintId == "" {
		response.BadRequest(c, "complaint_id is required")
		return
	}

	var body RejectInvoiceComplaintResponseRequest
	if err := c.ShouldBindJSON(&body); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	resp, err := h.Invoice.RejectInvoiceComplaintResponse(c.Request.Context(), &pb.RejectInvoiceComplaintResponseRequest{
		BuyerId:         int64(buyerID),
		OrderNo:         orderNo,
		ComplaintId:     complaintId,
		Reason:          body.Reason,
		ExpectedVersion: body.ExpectedVersion,
		IdempotencyKey:  c.GetHeader(middleware.IdempotencyKeyHeader),
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

// WithdrawInvoiceComplaint 買家撤回申訴
// @Summary      買家撤回申訴
// @Description  買家撤回申訴（58 號設計書 §6.1 端點 64）
// @Tags         電子發票 (Invoice)
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        Idempotency-Key  header  string  true  "冪等鍵，^[A-Za-z0-9._:-]{8,128}$"
// @Param        order_no  path  string  true  "訂單編號"
// @Param        complaint_id  path  string  true  "申訴案件 ID"
// @Param        request  body  WithdrawInvoiceComplaintRequest  true  "買家撤回申訴"
// @Success      200  {object}  object{code=int,message=string,data=InvoiceComplaintInfo}
// @Failure      400  {object}  object{code=int,message=string}  "參數格式錯誤"
// @Failure      401  {object}  object{code=int,message=string}  "未登入或 JWT 無效"
// @Failure      404  {object}  object{code=int,message=string}  "9309（Tier A）"
// @Failure      409  {object}  object{code=int,message=string}  "9322 / 9336 案件當前狀態不允許 / 9350 同鍵不同內容 / 9351 同鍵處理中"
// @Failure      429  {object}  object{code=int,message=string}  "請求過於頻繁"
// @Failure      503  {object}  object{code=int,message=string}  "invoice-service 不可用或逾時"
// @Router       /app/orders/{order_no}/invoice_complaints/{complaint_id}/withdrawals [post]
func (h *Holder) WithdrawInvoiceComplaint(c *gin.Context) {
	buyerID, ok := h.resolveBuyerID(c)
	if !ok {
		return
	}

	orderNo := c.Param("order_no")
	if orderNo == "" {
		response.BadRequest(c, "order_no is required")
		return
	}
	complaintId := c.Param("complaint_id")
	if complaintId == "" {
		response.BadRequest(c, "complaint_id is required")
		return
	}

	var body WithdrawInvoiceComplaintRequest
	if err := c.ShouldBindJSON(&body); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	resp, err := h.Invoice.WithdrawInvoiceComplaint(c.Request.Context(), &pb.WithdrawInvoiceComplaintRequest{
		BuyerId:         int64(buyerID),
		OrderNo:         orderNo,
		ComplaintId:     complaintId,
		ExpectedVersion: body.ExpectedVersion,
		IdempotencyKey:  c.GetHeader(middleware.IdempotencyKeyHeader),
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

// ReopenInvoiceComplaint 買家重新開啟申訴
// @Summary      買家重新開啟申訴
// @Description  買家重新開啟申訴（58 號設計書 §6.1 端點 65）
// @Tags         電子發票 (Invoice)
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        Idempotency-Key  header  string  true  "冪等鍵，^[A-Za-z0-9._:-]{8,128}$"
// @Param        order_no  path  string  true  "訂單編號"
// @Param        complaint_id  path  string  true  "申訴案件 ID"
// @Param        request  body  ReopenInvoiceComplaintRequest  true  "買家重新開啟申訴"
// @Success      200  {object}  object{code=int,message=string,data=InvoiceComplaintInfo}
// @Failure      400  {object}  object{code=int,message=string}  "reason 超長"
// @Failure      401  {object}  object{code=int,message=string}  "未登入或 JWT 無效"
// @Failure      404  {object}  object{code=int,message=string}  "9309（Tier A）"
// @Failure      409  {object}  object{code=int,message=string}  "9322 / 9336 非 RESOLVED／已達 2 次上限／冷卻未到 / 9350 同鍵不同內容 / 9351 同鍵處理中"
// @Failure      429  {object}  object{code=int,message=string}  "請求過於頻繁"
// @Failure      503  {object}  object{code=int,message=string}  "invoice-service 不可用或逾時"
// @Router       /app/orders/{order_no}/invoice_complaints/{complaint_id}/reopenings [post]
func (h *Holder) ReopenInvoiceComplaint(c *gin.Context) {
	buyerID, ok := h.resolveBuyerID(c)
	if !ok {
		return
	}

	orderNo := c.Param("order_no")
	if orderNo == "" {
		response.BadRequest(c, "order_no is required")
		return
	}
	complaintId := c.Param("complaint_id")
	if complaintId == "" {
		response.BadRequest(c, "complaint_id is required")
		return
	}

	var body ReopenInvoiceComplaintRequest
	if err := c.ShouldBindJSON(&body); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	resp, err := h.Invoice.ReopenInvoiceComplaint(c.Request.Context(), &pb.ReopenInvoiceComplaintRequest{
		BuyerId:         int64(buyerID),
		OrderNo:         orderNo,
		ComplaintId:     complaintId,
		Reason:          body.Reason,
		ExpectedVersion: body.ExpectedVersion,
		IdempotencyKey:  c.GetHeader(middleware.IdempotencyKeyHeader),
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
