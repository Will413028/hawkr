package handler

import (
	"github.com/Will413028/hawkr/gateway/internal/middleware"
	"github.com/Will413028/hawkr/gateway/internal/response"
	pb "github.com/Will413028/hawkr/go-pkg/proto/invoice"
	"github.com/gin-gonic/gin"
)

// 58 號《電子發票服務》賣家面 handler（§6.1.7–§6.1.11）。
//
// 身分：h.resolveStoreID（store surrogate）。能力層（requireSeller(cap)）在 router
// 掛載，這裡不再自行判斷停權——§46 規範明令停權判斷只准出現在 authz package。

// GetSellerInvoiceOverview 賣家發票總覽
// @Summary      賣家發票總覽
// @Description  賣家發票總覽（58 號設計書 §6.1 端點 16）
// @Tags         電子發票 (Invoice)
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Success      200  {object}  object{code=int,message=string,data=SellerInvoiceOverviewData}
// @Failure      401  {object}  object{code=int,message=string}  "未登入或 JWT 無效"
// @Failure      403  {object}  object{code=int,message=string}  "非賣家身分"
// @Failure      409  {object}  object{code=int,message=string}  "9240 賣家停權"
// @Failure      429  {object}  object{code=int,message=string}  "請求過於頻繁"
// @Failure      503  {object}  object{code=int,message=string}  "invoice-service 不可用或逾時"
// @Router       /app/seller/invoices/overview [get]
func (h *Holder) GetSellerInvoiceOverview(c *gin.Context) {
	storeID, ok := h.resolveStoreID(c)
	if !ok {
		return
	}

	resp, err := h.Invoice.GetSellerInvoiceOverview(c.Request.Context(), &pb.GetSellerInvoiceOverviewRequest{
		StoreId: int64(storeID),
	})
	if err != nil {
		response.GrpcErr(c, "invoice", err)
		return
	}

	data := convInvSellerInvoiceOverviewData(resp.GetData())
	// preserveData 業務碼不會轉成 err（interceptor 對它們 return nil），必須自己看 code。
	if resp.GetCode() != 0 {
		response.Biz(c, resp.GetCode(), resp.GetMessage(), data)
		return
	}
	response.OK(c, data)
}

// GetSellerTaxProfile 查賣家稅務資格
// @Summary      查賣家稅務資格
// @Description  查賣家稅務資格（58 號設計書 §6.1 端點 17）
// @Tags         電子發票 (Invoice)
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Success      200  {object}  object{code=int,message=string,data=SellerTaxProfileData}
// @Failure      401  {object}  object{code=int,message=string}  "未登入或 JWT 無效"
// @Failure      403  {object}  object{code=int,message=string}  "非賣家身分"
// @Failure      404  {object}  object{code=int,message=string}  "9320"
// @Failure      429  {object}  object{code=int,message=string}  "請求過於頻繁"
// @Failure      503  {object}  object{code=int,message=string}  "invoice-service 不可用或逾時"
// @Router       /app/seller/invoices/tax_profile [get]
func (h *Holder) GetSellerTaxProfile(c *gin.Context) {
	storeID, ok := h.resolveStoreID(c)
	if !ok {
		return
	}

	resp, err := h.Invoice.GetSellerTaxProfile(c.Request.Context(), &pb.GetSellerTaxProfileRequest{
		StoreId: int64(storeID),
	})
	if err != nil {
		response.GrpcErr(c, "invoice", err)
		return
	}

	data := convInvSellerTaxProfileData(resp.GetData())
	// preserveData 業務碼不會轉成 err（interceptor 對它們 return nil），必須自己看 code。
	if resp.GetCode() != 0 {
		response.Biz(c, resp.GetCode(), resp.GetMessage(), data)
		return
	}
	response.OK(c, data)
}

// SaveSellerTaxProfileDraft 儲存稅務資格草稿
// @Summary      儲存稅務資格草稿
// @Description  儲存稅務資格草稿（58 號設計書 §6.1 端點 18）
// @Tags         電子發票 (Invoice)
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        Idempotency-Key  header  string  true  "冪等鍵，^[A-Za-z0-9._:-]{8,128}$"
// @Param        request  body  SaveSellerTaxProfileDraftRequest  true  "儲存稅務資格草稿"
// @Success      200  {object}  object{code=int,message=string,data=SellerTaxProfileData}
// @Failure      400  {object}  object{code=int,message=string}
// @Failure      401  {object}  object{code=int,message=string}  "未登入或 JWT 無效"
// @Failure      403  {object}  object{code=int,message=string}  "非賣家身分"
// @Failure      409  {object}  object{code=int,message=string}  "9321 狀態不允許 / 9322 版本衝突 / 9350 同鍵不同內容 / 9351 同鍵處理中"
// @Failure      429  {object}  object{code=int,message=string}  "請求過於頻繁"
// @Failure      503  {object}  object{code=int,message=string}  "invoice-service 不可用或逾時"
// @Router       /app/seller/invoices/tax_profile/draft [put]
func (h *Holder) SaveSellerTaxProfileDraft(c *gin.Context) {
	storeID, ok := h.resolveStoreID(c)
	if !ok {
		return
	}
	// 稅籍資料被誰改過屬保存 5 年的稽核軌跡（§8.1.3）；一個店有多個可登入帳號時，
	// 只有 account 層答得出人（同端點 19／22 的形狀，backend#3048）。
	accountID, actorOK := getAccountID(c)
	if !actorOK {
		response.Unauthorized(c)
		return
	}

	var body SaveSellerTaxProfileDraftRequest
	if err := c.ShouldBindJSON(&body); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	resp, err := h.Invoice.SaveSellerTaxProfileDraft(c.Request.Context(), &pb.SaveSellerTaxProfileDraftRequest{
		StoreId:              int64(storeID),
		TaxIdentity:          toPbInvSellerTaxIdentity(body.TaxIdentity),
		EvidenceObjectKeys:   body.EvidenceObjectKeys,
		ExpectedVersion:      body.ExpectedVersion,
		IdempotencyKey:       c.GetHeader(middleware.IdempotencyKeyHeader),
		EvidenceStagedTokens: body.EvidenceStagedTokens,
		SavedByAccountId:     accountID,
	})
	if err != nil {
		response.GrpcErr(c, "invoice", err)
		return
	}

	data := convInvSellerTaxProfileData(resp.GetData())
	// preserveData 業務碼不會轉成 err（interceptor 對它們 return nil），必須自己看 code。
	if resp.GetCode() != 0 {
		response.Biz(c, resp.GetCode(), resp.GetMessage(), data)
		return
	}
	response.OK(c, data)
}

// SubmitSellerTaxProfile 送出稅務資格審核
// @Summary      送出稅務資格審核
// @Description  送出稅務資格審核（58 號設計書 §6.1 端點 19）。body 的 acceptedTerms 必須為 true：送審與「同意自行開票責任條款」是同一個動作（裁決 C），未同意回 9325。
// @Tags         電子發票 (Invoice)
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        Idempotency-Key  header  string  true  "冪等鍵，^[A-Za-z0-9._:-]{8,128}$"
// @Param        request  body  SubmitSellerTaxProfileRequest  true  "送出稅務資格審核"
// @Success      200  {object}  object{code=int,message=string,data=SellerTaxProfileData}
// @Failure      400  {object}  object{code=int,message=string}
// @Failure      401  {object}  object{code=int,message=string}  "未登入或 JWT 無效"
// @Failure      403  {object}  object{code=int,message=string}  "非賣家身分"
// @Failure      409  {object}  object{code=int,message=string}  "9321 / 9322 / 9325 未接受條款 / 9350 同鍵不同內容 / 9351 同鍵處理中"
// @Failure      429  {object}  object{code=int,message=string}  "請求過於頻繁"
// @Failure      503  {object}  object{code=int,message=string}  "invoice-service 不可用或逾時"
// @Router       /app/seller/invoices/tax_profile/submissions [post]
func (h *Holder) SubmitSellerTaxProfile(c *gin.Context) {
	storeID, ok := h.resolveStoreID(c)
	if !ok {
		return
	}
	// 裁決 C 之後送審**同時是條款簽署**，而簽署要留下「是誰按的」——那是 account 層的
	// 事實不是 store 層的（同端點 22 的形狀）。缺它，這份「責任在賣家」的書面憑據
	// 查不到自然人。
	accountID, actorOK := getAccountID(c)
	if !actorOK {
		response.Unauthorized(c)
		return
	}

	var body SubmitSellerTaxProfileRequest
	if err := c.ShouldBindJSON(&body); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	resp, err := h.Invoice.SubmitSellerTaxProfile(c.Request.Context(), &pb.SubmitSellerTaxProfileRequest{
		StoreId:             int64(storeID),
		ExpectedVersion:     body.ExpectedVersion,
		AcceptedTerms:       body.AcceptedTerms,
		AcceptedByAccountId: accountID,
		IdempotencyKey:      c.GetHeader(middleware.IdempotencyKeyHeader),
	})
	if err != nil {
		response.GrpcErr(c, "invoice", err)
		return
	}

	data := convInvSellerTaxProfileData(resp.GetData())
	// §6.1.0(13) 的 preserveData 碼：EnvelopeGuardInterceptor 對它們不轉 error，
	// data 必須原樣透傳（HTTP 恆 200），否則前端拿不到導向用的 id／時間。
	if resp.GetCode() != 0 {
		response.Biz(c, resp.GetCode(), resp.GetMessage(), data)
		return
	}
	response.OK(c, data)
}

// RequestSellerTaxExtension 申請稅務補件期限延長
// @Summary      申請稅務補件期限延長
// @Description  申請稅務補件期限延長（58 號設計書 §6.1 端點 20）
// @Tags         電子發票 (Invoice)
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        Idempotency-Key  header  string  true  "冪等鍵，^[A-Za-z0-9._:-]{8,128}$"
// @Param        request  body  RequestSellerTaxExtensionRequest  true  "申請稅務補件期限延長"
// @Success      200  {object}  object{code=int,message=string,data=SellerTaxProfileData}
// @Failure      400  {object}  object{code=int,message=string}
// @Failure      401  {object}  object{code=int,message=string}  "未登入或 JWT 無效"
// @Failure      403  {object}  object{code=int,message=string}  "非賣家身分"
// @Failure      409  {object}  object{code=int,message=string}  "9321 / 9322 / 9323 已延長過 / 9350 同鍵不同內容 / 9351 同鍵處理中"
// @Failure      429  {object}  object{code=int,message=string}  "請求過於頻繁"
// @Failure      503  {object}  object{code=int,message=string}  "invoice-service 不可用或逾時"
// @Router       /app/seller/invoices/tax_profile/extension_requests [post]
func (h *Holder) RequestSellerTaxExtension(c *gin.Context) {
	storeID, ok := h.resolveStoreID(c)
	if !ok {
		return
	}
	// 延長會把平台補件期限往後推（一次最多 30 天、只能一次）⇒「是誰推的」必須留痕
	// （backend#3048）。
	accountID, actorOK := getAccountID(c)
	if !actorOK {
		response.Unauthorized(c)
		return
	}

	var body RequestSellerTaxExtensionRequest
	if err := c.ShouldBindJSON(&body); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	resp, err := h.Invoice.RequestSellerTaxExtension(c.Request.Context(), &pb.RequestSellerTaxExtensionRequest{
		StoreId:              int64(storeID),
		Reason:               body.Reason,
		ExpectedVersion:      body.ExpectedVersion,
		IdempotencyKey:       c.GetHeader(middleware.IdempotencyKeyHeader),
		RequestedByAccountId: accountID,
	})
	if err != nil {
		response.GrpcErr(c, "invoice", err)
		return
	}

	data := convInvSellerTaxProfileData(resp.GetData())
	// preserveData 業務碼不會轉成 err（interceptor 對它們 return nil），必須自己看 code。
	if resp.GetCode() != 0 {
		response.Biz(c, resp.GetCode(), resp.GetMessage(), data)
		return
	}
	response.OK(c, data)
}

// GetSelfIssueAttestation 查自行開票責任條款
// @Summary      查自行開票責任條款
// @Description  查自行開票責任條款（58 號設計書 §6.1 端點 21）
// @Tags         電子發票 (Invoice)
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Success      200  {object}  object{code=int,message=string,data=SelfIssueAttestationInfo}
// @Failure      401  {object}  object{code=int,message=string}  "未登入或 JWT 無效"
// @Failure      403  {object}  object{code=int,message=string}  "非賣家身分"
// @Failure      429  {object}  object{code=int,message=string}  "請求過於頻繁"
// @Failure      503  {object}  object{code=int,message=string}  "invoice-service 不可用或逾時"
// @Router       /app/seller/invoices/self_issue_attestations [get]
func (h *Holder) GetSelfIssueAttestation(c *gin.Context) {
	storeID, ok := h.resolveStoreID(c)
	if !ok {
		return
	}

	resp, err := h.Invoice.GetSelfIssueAttestation(c.Request.Context(), &pb.GetSelfIssueAttestationRequest{
		StoreId: int64(storeID),
	})
	if err != nil {
		response.GrpcErr(c, "invoice", err)
		return
	}

	data := convInvSelfIssueAttestationInfo(resp.GetData())
	// preserveData 業務碼不會轉成 err（interceptor 對它們 return nil），必須自己看 code。
	if resp.GetCode() != 0 {
		response.Biz(c, resp.GetCode(), resp.GetMessage(), data)
		return
	}
	response.OK(c, data)
}

// AcceptSelfIssueAttestation 同意自行開票責任條款
// @Summary      同意自行開票責任條款
// @Description  同意自行開票責任條款（58 號設計書 §6.1 端點 22）
// @Tags         電子發票 (Invoice)
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        Idempotency-Key  header  string  true  "冪等鍵，^[A-Za-z0-9._:-]{8,128}$"
// @Param        request  body  AcceptSelfIssueAttestationRequest  true  "同意自行開票責任條款"
// @Success      200  {object}  object{code=int,message=string,data=SelfIssueAttestationInfo}
// @Failure      400  {object}  object{code=int,message=string}
// @Failure      401  {object}  object{code=int,message=string}  "未登入或 JWT 無效"
// @Failure      403  {object}  object{code=int,message=string}  "非賣家身分"
// @Failure      409  {object}  object{code=int,message=string}  "9350 同鍵不同內容 / 9351 同鍵處理中"
// @Failure      429  {object}  object{code=int,message=string}  "請求過於頻繁"
// @Failure      503  {object}  object{code=int,message=string}  "invoice-service 不可用或逾時"
// @Router       /app/seller/invoices/self_issue_attestations [post]
func (h *Holder) AcceptSelfIssueAttestation(c *gin.Context) {
	storeID, ok := h.resolveStoreID(c)
	if !ok {
		return
	}
	// 條款同意要留下「是誰按的」，那是 account 層的事實，不是 store 層的。
	accountID, actorOK := getAccountID(c)
	if !actorOK {
		response.Unauthorized(c)
		return
	}

	var body AcceptSelfIssueAttestationRequest
	if err := c.ShouldBindJSON(&body); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	resp, err := h.Invoice.AcceptSelfIssueAttestation(c.Request.Context(), &pb.AcceptSelfIssueAttestationRequest{
		StoreId:             int64(storeID),
		AcceptedByAccountId: accountID,
		TermsVersion:        body.TermsVersion,
		Accepted:            body.Accepted,
		IdempotencyKey:      c.GetHeader(middleware.IdempotencyKeyHeader),
	})
	if err != nil {
		response.GrpcErr(c, "invoice", err)
		return
	}

	data := convInvSelfIssueAttestationInfo(resp.GetData())
	// preserveData 業務碼不會轉成 err（interceptor 對它們 return nil），必須自己看 code。
	if resp.GetCode() != 0 {
		response.Biz(c, resp.GetCode(), resp.GetMessage(), data)
		return
	}
	response.OK(c, data)
}

// GetSellerProviderAccount 查 AMEGO 憑證帳號
// @Summary      查 AMEGO 憑證帳號
// @Description  查 AMEGO 憑證帳號（58 號設計書 §6.1 端點 23）
// @Tags         電子發票 (Invoice)
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Success      200  {object}  object{code=int,message=string,data=SellerProviderAccountData}
// @Failure      401  {object}  object{code=int,message=string}  "未登入或 JWT 無效"
// @Failure      403  {object}  object{code=int,message=string}  "非賣家身分"
// @Failure      404  {object}  object{code=int,message=string}  "9330"
// @Failure      409  {object}  object{code=int,message=string}  "9240 賣家停權"
// @Failure      429  {object}  object{code=int,message=string}  "請求過於頻繁"
// @Failure      503  {object}  object{code=int,message=string}  "invoice-service 不可用或逾時"
// @Router       /app/seller/invoices/provider_account [get]
func (h *Holder) GetSellerProviderAccount(c *gin.Context) {
	storeID, ok := h.resolveStoreID(c)
	if !ok {
		return
	}

	resp, err := h.Invoice.GetSellerProviderAccount(c.Request.Context(), &pb.GetSellerProviderAccountRequest{
		StoreId: int64(storeID),
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

// PutSellerProviderCredentials 設定 AMEGO 憑證
// @Summary      設定 AMEGO 憑證
// @Description  設定 AMEGO 憑證（58 號設計書 §6.1 端點 24）
// @Tags         電子發票 (Invoice)
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        Idempotency-Key  header  string  true  "冪等鍵，^[A-Za-z0-9._:-]{8,128}$"
// @Param        request  body  PutSellerProviderCredentialsRequest  true  "設定 AMEGO 憑證"
// @Success      200  {object}  object{code=int,message=string,data=SellerProviderAccountData}
// @Failure      400  {object}  object{code=int,message=string}
// @Failure      401  {object}  object{code=int,message=string}  "未登入或 JWT 無效"
// @Failure      403  {object}  object{code=int,message=string}  "非賣家身分"
// @Failure      409  {object}  object{code=int,message=string}  "9240 / 9321 稅籍未核准 / 9322 / 9334 統編不符 / 9350 同鍵不同內容 / 9351 同鍵處理中"
// @Failure      429  {object}  object{code=int,message=string}  "請求過於頻繁"
// @Failure      503  {object}  object{code=int,message=string}  "invoice-service 不可用或逾時"
// @Router       /app/seller/invoices/provider_account/credentials [put]
func (h *Holder) PutSellerProviderCredentials(c *gin.Context) {
	storeID, ok := h.resolveStoreID(c)
	if !ok {
		return
	}

	var body PutSellerProviderCredentialsRequest
	if err := c.ShouldBindJSON(&body); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	resp, err := h.Invoice.PutSellerProviderCredentials(c.Request.Context(), &pb.PutSellerProviderCredentialsRequest{
		StoreId:         int64(storeID),
		Provider:        body.Provider,
		SellerTaxId:     body.SellerTaxId,
		AppKey:          body.AppKey,
		ConsentVersion:  body.ConsentVersion,
		ExpectedVersion: body.ExpectedVersion,
		IdempotencyKey:  c.GetHeader(middleware.IdempotencyKeyHeader),
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

// VerifySellerProviderAccount 驗證 AMEGO 憑證連線
// @Summary      驗證 AMEGO 憑證連線
// @Description  驗證 AMEGO 憑證連線（58 號設計書 §6.1 端點 25）
// @Tags         電子發票 (Invoice)
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        Idempotency-Key  header  string  true  "冪等鍵，^[A-Za-z0-9._:-]{8,128}$"
// @Param        request  body  VerifySellerProviderAccountRequest  true  "驗證 AMEGO 憑證連線"
// @Success      200  {object}  object{code=int,message=string,data=SellerProviderAccountData}
// @Failure      400  {object}  object{code=int,message=string}
// @Failure      401  {object}  object{code=int,message=string}  "未登入或 JWT 無效"
// @Failure      403  {object}  object{code=int,message=string}  "非賣家身分"
// @Failure      409  {object}  object{code=int,message=string}  "9240 / 9332 provider 不可用 / 9333 狀態不允許 / 9350 同鍵不同內容 / 9351 同鍵處理中"
// @Failure      429  {object}  object{code=int,message=string}  "請求過於頻繁"
// @Failure      503  {object}  object{code=int,message=string}  "invoice-service 不可用或逾時"
// @Router       /app/seller/invoices/provider_account/verifications [post]
func (h *Holder) VerifySellerProviderAccount(c *gin.Context) {
	storeID, ok := h.resolveStoreID(c)
	if !ok {
		return
	}

	var body VerifySellerProviderAccountRequest
	if err := c.ShouldBindJSON(&body); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	resp, err := h.Invoice.VerifySellerProviderAccount(c.Request.Context(), &pb.VerifySellerProviderAccountRequest{
		StoreId:         int64(storeID),
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

// ActivateSellerProviderAccount 啟用 AMEGO 開票（指定切換時點）
// @Summary      啟用 AMEGO 開票（指定切換時點）
// @Description  啟用 AMEGO 開票（指定切換時點）（58 號設計書 §6.1 端點 26）
// @Tags         電子發票 (Invoice)
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        Idempotency-Key  header  string  true  "冪等鍵，^[A-Za-z0-9._:-]{8,128}$"
// @Param        request  body  ActivateSellerProviderAccountRequest  true  "啟用 AMEGO 開票（指定切換時點）"
// @Success      200  {object}  object{code=int,message=string,data=SellerProviderAccountData}
// @Failure      400  {object}  object{code=int,message=string}  "含 cutoverAt < now"
// @Failure      401  {object}  object{code=int,message=string}  "未登入或 JWT 無效"
// @Failure      403  {object}  object{code=int,message=string}  "非賣家身分"
// @Failure      409  {object}  object{code=int,message=string}  "9240 / 9322 / 9333 未先驗證 / 9350 同鍵不同內容 / 9351 同鍵處理中"
// @Failure      429  {object}  object{code=int,message=string}  "請求過於頻繁"
// @Failure      503  {object}  object{code=int,message=string}  "invoice-service 不可用或逾時"
// @Router       /app/seller/invoices/provider_account/activations [post]
func (h *Holder) ActivateSellerProviderAccount(c *gin.Context) {
	storeID, ok := h.resolveStoreID(c)
	if !ok {
		return
	}

	var body ActivateSellerProviderAccountRequest
	if err := c.ShouldBindJSON(&body); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	resp, err := h.Invoice.ActivateSellerProviderAccount(c.Request.Context(), &pb.ActivateSellerProviderAccountRequest{
		StoreId:         int64(storeID),
		CutoverAt:       body.CutoverAt,
		ExpectedVersion: body.ExpectedVersion,
		IdempotencyKey:  c.GetHeader(middleware.IdempotencyKeyHeader),
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

// RotateSellerProviderCredentials 輪替 AMEGO 憑證
// @Summary      輪替 AMEGO 憑證
// @Description  輪替 AMEGO 憑證（58 號設計書 §6.1 端點 27）
// @Tags         電子發票 (Invoice)
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        Idempotency-Key  header  string  true  "冪等鍵，^[A-Za-z0-9._:-]{8,128}$"
// @Param        request  body  RotateSellerProviderCredentialsRequest  true  "輪替 AMEGO 憑證"
// @Success      200  {object}  object{code=int,message=string,data=SellerProviderAccountData}
// @Failure      400  {object}  object{code=int,message=string}
// @Failure      401  {object}  object{code=int,message=string}  "未登入或 JWT 無效"
// @Failure      403  {object}  object{code=int,message=string}  "非賣家身分"
// @Failure      409  {object}  object{code=int,message=string}  "9240 / 9322 / 9333 / 9350 同鍵不同內容 / 9351 同鍵處理中"
// @Failure      429  {object}  object{code=int,message=string}  "請求過於頻繁"
// @Failure      503  {object}  object{code=int,message=string}  "invoice-service 不可用或逾時"
// @Router       /app/seller/invoices/provider_account/rotations [post]
func (h *Holder) RotateSellerProviderCredentials(c *gin.Context) {
	storeID, ok := h.resolveStoreID(c)
	if !ok {
		return
	}

	var body RotateSellerProviderCredentialsRequest
	if err := c.ShouldBindJSON(&body); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	resp, err := h.Invoice.RotateSellerProviderCredentials(c.Request.Context(), &pb.RotateSellerProviderCredentialsRequest{
		StoreId:         int64(storeID),
		AppKey:          body.AppKey,
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

// GetSellerReceiptAuthorization 查免用統一發票的收據代製授權
// @Summary      查免用統一發票的收據代製授權
// @Description  查免用統一發票的收據代製授權（58 號設計書 §6.1 端點 28）
// @Tags         電子發票 (Invoice)
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Success      200  {object}  object{code=int,message=string,data=SellerReceiptAuthorizationData}
// @Failure      401  {object}  object{code=int,message=string}  "未登入或 JWT 無效"
// @Failure      403  {object}  object{code=int,message=string}  "非賣家身分"
// @Failure      409  {object}  object{code=int,message=string}  "9240"
// @Failure      429  {object}  object{code=int,message=string}  "請求過於頻繁"
// @Failure      503  {object}  object{code=int,message=string}  "invoice-service 不可用或逾時"
// @Router       /app/seller/invoices/receipt_authorization [get]
func (h *Holder) GetSellerReceiptAuthorization(c *gin.Context) {
	storeID, ok := h.resolveStoreID(c)
	if !ok {
		return
	}

	resp, err := h.Invoice.GetSellerReceiptAuthorization(c.Request.Context(), &pb.GetSellerReceiptAuthorizationRequest{
		StoreId: int64(storeID),
	})
	if err != nil {
		response.GrpcErr(c, "invoice", err)
		return
	}

	data := convInvSellerReceiptAuthorizationData(resp.GetData())
	// preserveData 業務碼不會轉成 err（interceptor 對它們 return nil），必須自己看 code。
	if resp.GetCode() != 0 {
		response.Biz(c, resp.GetCode(), resp.GetMessage(), data)
		return
	}
	response.OK(c, data)
}

// ConfirmSellerReceiptAuthorization 確認收據代製授權
// @Summary      確認收據代製授權
// @Description  確認收據代製授權（58 號設計書 §6.1 端點 29）
// @Tags         電子發票 (Invoice)
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        Idempotency-Key  header  string  true  "冪等鍵，^[A-Za-z0-9._:-]{8,128}$"
// @Param        request  body  ConfirmSellerReceiptAuthorizationRequest  true  "確認收據代製授權"
// @Success      200  {object}  object{code=int,message=string,data=SellerReceiptAuthorizationData}
// @Failure      400  {object}  object{code=int,message=string}
// @Failure      401  {object}  object{code=int,message=string}  "未登入或 JWT 無效"
// @Failure      403  {object}  object{code=int,message=string}  "非賣家身分"
// @Failure      409  {object}  object{code=int,message=string}  "9240 / 9322 / 9335 稅籍審核狀態不在 allowed-from / 9350 同鍵不同內容 / 9351 同鍵處理中"
// @Failure      429  {object}  object{code=int,message=string}  "請求過於頻繁"
// @Failure      503  {object}  object{code=int,message=string}  "invoice-service 不可用或逾時"
// @Router       /app/seller/invoices/receipt_authorization/confirmations [post]
func (h *Holder) ConfirmSellerReceiptAuthorization(c *gin.Context) {
	storeID, ok := h.resolveStoreID(c)
	if !ok {
		return
	}

	var body ConfirmSellerReceiptAuthorizationRequest
	if err := c.ShouldBindJSON(&body); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	resp, err := h.Invoice.ConfirmSellerReceiptAuthorization(c.Request.Context(), &pb.ConfirmSellerReceiptAuthorizationRequest{
		StoreId:         int64(storeID),
		ConsentVersion:  body.ConsentVersion,
		Authorized:      body.Authorized,
		ExpectedVersion: body.ExpectedVersion,
		IdempotencyKey:  c.GetHeader(middleware.IdempotencyKeyHeader),
	})
	if err != nil {
		response.GrpcErr(c, "invoice", err)
		return
	}

	data := convInvSellerReceiptAuthorizationData(resp.GetData())
	// preserveData 業務碼不會轉成 err（interceptor 對它們 return nil），必須自己看 code。
	if resp.GetCode() != 0 {
		response.Biz(c, resp.GetCode(), resp.GetMessage(), data)
		return
	}
	response.OK(c, data)
}

// ListSellerInvoiceDocuments 賣家發票紀錄列表
// @Summary      賣家發票紀錄列表
// @Description  賣家發票紀錄列表（58 號設計書 §6.1 端點 30）
// @Tags         電子發票 (Invoice)
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        date_from  query  string  false  "起始日期 YYYY-MM-DD"
// @Param        date_to  query  string  false  "結束日期 YYYY-MM-DD"
// @Param        mode  query  string  false  "開票模式篩選"
// @Param        status  query  string  false  "狀態篩選"
// @Param        abnormal_only  query  string  false  "只看異常，true/false"
// @Param        order_no  query  string  false  "訂單編號"
// @Param        page  query  string  false  "頁碼，數字字串 ≥ 1，預設 1"
// @Param        page_size  query  string  false  "每頁筆數，數字字串 1–50，預設 10（逾越一律 400，不夾住）"
// @Success      200  {object}  object{code=int,message=string,data=object{records=[]SellerInvoiceDocumentItem,total=string,page=string,pageSize=string,shipmentStatusDegraded=bool}}
// @Failure      400  {object}  object{code=int,message=string}  "日期格式"
// @Failure      401  {object}  object{code=int,message=string}  "未登入或 JWT 無效"
// @Failure      403  {object}  object{code=int,message=string}  "非賣家身分"
// @Failure      409  {object}  object{code=int,message=string}  "9240"
// @Failure      429  {object}  object{code=int,message=string}  "請求過於頻繁"
// @Failure      503  {object}  object{code=int,message=string}  "invoice-service 不可用或逾時"
// @Router       /app/seller/invoices/documents [get]
func (h *Holder) ListSellerInvoiceDocuments(c *gin.Context) {
	storeID, ok := h.resolveStoreID(c)
	if !ok {
		return
	}

	page, pageSize, ok := invoicePaging(c, defaultPageSizeStr)
	if !ok {
		return
	}

	abnormalOnly, ok := invoiceBoolQuery(c, "abnormal_only")
	if !ok {
		return
	}

	resp, err := h.Invoice.ListSellerInvoiceDocuments(c.Request.Context(), &pb.ListSellerInvoiceDocumentsRequest{
		StoreId:      int64(storeID),
		DateFrom:     c.Query("date_from"),
		DateTo:       c.Query("date_to"),
		Mode:         c.Query("mode"),
		Status:       c.Query("status"),
		AbnormalOnly: abnormalOnly,
		OrderNo:      c.Query("order_no"),
		Page:         page,
		PageSize:     pageSize,
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
	// #3019：分頁 data 多帶一個 shipmentStatusDegraded ⇒ 不能走 response.Page
	// （它只組四個欄位）。內嵌 response.PageData 讓四個 key 逐字沿用同一份 tag。
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

// GetSellerInvoiceDocument 賣家發票明細
// @Summary      賣家發票明細
// @Description  賣家發票明細（58 號設計書 §6.1 端點 31）
// @Tags         電子發票 (Invoice)
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        document_id  path  string  true  "發票文件 ID"
// @Success      200  {object}  object{code=int,message=string,data=SellerInvoiceDocumentDetail}
// @Failure      401  {object}  object{code=int,message=string}  "未登入或 JWT 無效"
// @Failure      403  {object}  object{code=int,message=string}  "非賣家身分"
// @Failure      404  {object}  object{code=int,message=string}  "9301（Tier A）"
// @Failure      409  {object}  object{code=int,message=string}  "9240"
// @Failure      429  {object}  object{code=int,message=string}  "請求過於頻繁"
// @Failure      503  {object}  object{code=int,message=string}  "invoice-service 不可用或逾時"
// @Router       /app/seller/invoices/documents/{document_id} [get]
func (h *Holder) GetSellerInvoiceDocument(c *gin.Context) {
	storeID, ok := h.resolveStoreID(c)
	if !ok {
		return
	}

	documentId := c.Param("document_id")
	if documentId == "" {
		response.BadRequest(c, "document_id is required")
		return
	}

	resp, err := h.Invoice.GetSellerInvoiceDocument(c.Request.Context(), &pb.GetSellerInvoiceDocumentRequest{
		StoreId:    int64(storeID),
		DocumentId: documentId,
	})
	if err != nil {
		response.GrpcErr(c, "invoice", err)
		return
	}

	data := convInvSellerInvoiceDocumentDetail(resp.GetData())
	// preserveData 業務碼不會轉成 err（interceptor 對它們 return nil），必須自己看 code。
	if resp.GetCode() != 0 {
		response.Biz(c, resp.GetCode(), resp.GetMessage(), data)
		return
	}
	response.OK(c, data)
}

// ListSellerReceivedInvoiceDocuments 賣家進項發票列表
// @Summary      賣家進項發票列表
// @Description  賣家進項發票列表（58 號設計書 §6.1 端點 72）
// @Tags         電子發票 (Invoice)
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        date_from  query  string  false  "起始日期 YYYY-MM-DD"
// @Param        date_to  query  string  false  "結束日期 YYYY-MM-DD"
// @Param        status  query  string  false  "狀態篩選"
// @Param        page  query  string  false  "頁碼，數字字串 ≥ 1，預設 1"
// @Param        page_size  query  string  false  "每頁筆數，數字字串 1–50，預設 10（逾越一律 400，不夾住）"
// @Success      200  {object}  object{code=int,message=string,data=object{records=[]SellerReceivedInvoiceItem,total=string,page=string,pageSize=string}}
// @Failure      400  {object}  object{code=int,message=string}  "日期格式"
// @Failure      401  {object}  object{code=int,message=string}  "未登入或 JWT 無效"
// @Failure      403  {object}  object{code=int,message=string}  "非賣家身分"
// @Failure      429  {object}  object{code=int,message=string}  "請求過於頻繁"
// @Failure      503  {object}  object{code=int,message=string}  "invoice-service 不可用或逾時"
// @Router       /app/seller/invoices/received_documents [get]
func (h *Holder) ListSellerReceivedInvoiceDocuments(c *gin.Context) {
	storeID, ok := h.resolveStoreID(c)
	if !ok {
		return
	}

	page, pageSize, ok := invoicePaging(c, defaultPageSizeStr)
	if !ok {
		return
	}

	resp, err := h.Invoice.ListSellerReceivedInvoiceDocuments(c.Request.Context(), &pb.ListSellerReceivedInvoiceDocumentsRequest{
		StoreId:  int64(storeID),
		DateFrom: c.Query("date_from"),
		DateTo:   c.Query("date_to"),
		Status:   c.Query("status"),
		Page:     page,
		PageSize: pageSize,
	})
	if err != nil {
		response.GrpcErr(c, "invoice", err)
		return
	}

	data := resp.GetData()
	records := make([]SellerReceivedInvoiceItem, 0, len(data.GetRecords()))
	for _, it := range data.GetRecords() {
		records = append(records, convInvSellerReceivedInvoiceItem(it))
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

// ResendSellerInvoiceNotification 賣家重寄發票通知
// @Summary      賣家重寄發票通知
// @Description  賣家重寄發票通知（58 號設計書 §6.1 端點 32）
// @Tags         電子發票 (Invoice)
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        Idempotency-Key  header  string  true  "冪等鍵，^[A-Za-z0-9._:-]{8,128}$"
// @Param        document_id  path  string  true  "發票文件 ID"
// @Success      200  {object}  object{code=int,message=string,data=ResendInvoiceNotificationData}
// @Failure      401  {object}  object{code=int,message=string}  "未登入或 JWT 無效"
// @Failure      403  {object}  object{code=int,message=string}  "非賣家身分"
// @Failure      404  {object}  object{code=int,message=string}  "9301（Tier A）"
// @Failure      409  {object}  object{code=int,message=string}  "9350 同鍵不同內容 / 9351 同鍵處理中"
// @Failure      429  {object}  object{code=int,message=string}  "9352 十分鐘內已寄送過"
// @Failure      503  {object}  object{code=int,message=string}  "invoice-service 不可用或逾時"
// @Router       /app/seller/invoices/documents/{document_id}/notification_resends [post]
func (h *Holder) ResendSellerInvoiceNotification(c *gin.Context) {
	storeID, ok := h.resolveStoreID(c)
	if !ok {
		return
	}

	documentId := c.Param("document_id")
	if documentId == "" {
		response.BadRequest(c, "document_id is required")
		return
	}

	resp, err := h.Invoice.ResendInvoiceNotification(c.Request.Context(), &pb.ResendInvoiceNotificationRequest{
		StoreId:        int64(storeID),
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

// GetSellerInvoiceJob 賣家發票異常詳情
// @Summary      賣家發票異常詳情
// @Description  賣家發票異常詳情（58 號設計書 §6.1 端點 33）
// @Tags         電子發票 (Invoice)
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        job_id  path  string  true  "發票工作 ID"
// @Success      200  {object}  object{code=int,message=string,data=InvoiceJobInfo}
// @Failure      401  {object}  object{code=int,message=string}  "未登入或 JWT 無效"
// @Failure      403  {object}  object{code=int,message=string}  "非賣家身分"
// @Failure      404  {object}  object{code=int,message=string}  "9340（Tier A）"
// @Failure      409  {object}  object{code=int,message=string}  "9240"
// @Failure      429  {object}  object{code=int,message=string}  "請求過於頻繁"
// @Failure      503  {object}  object{code=int,message=string}  "invoice-service 不可用或逾時"
// @Router       /app/seller/invoices/jobs/{job_id} [get]
func (h *Holder) GetSellerInvoiceJob(c *gin.Context) {
	storeID, ok := h.resolveStoreID(c)
	if !ok {
		return
	}

	jobId := c.Param("job_id")
	if jobId == "" {
		response.BadRequest(c, "job_id is required")
		return
	}

	resp, err := h.Invoice.GetSellerInvoiceJob(c.Request.Context(), &pb.GetSellerInvoiceJobRequest{
		StoreId: int64(storeID),
		JobId:   jobId,
	})
	if err != nil {
		response.GrpcErr(c, "invoice", err)
		return
	}

	data := convInvInvoiceJobInfo(resp.GetData())
	// preserveData 業務碼不會轉成 err（interceptor 對它們 return nil），必須自己看 code。
	if resp.GetCode() != 0 {
		response.Biz(c, resp.GetCode(), resp.GetMessage(), data)
		return
	}
	response.OK(c, data)
}

// ListSellerInvoiceComplaints 賣家申訴列表
// @Summary      賣家申訴列表
// @Description  賣家申訴列表（58 號設計書 §6.1 端點 34）
// @Tags         電子發票 (Invoice)
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        status  query  string  false  "狀態篩選"
// @Param        sort  query  string  false  "排序"
// @Param        page  query  string  false  "頁碼，數字字串 ≥ 1，預設 1"
// @Param        page_size  query  string  false  "每頁筆數，數字字串 1–50，預設 10（逾越一律 400，不夾住）"
// @Success      200  {object}  object{code=int,message=string,data=object{records=[]InvoiceComplaintInfo,total=string,page=string,pageSize=string}}
// @Failure      400  {object}  object{code=int,message=string}
// @Failure      401  {object}  object{code=int,message=string}  "未登入或 JWT 無效"
// @Failure      403  {object}  object{code=int,message=string}  "非賣家身分"
// @Failure      429  {object}  object{code=int,message=string}  "請求過於頻繁"
// @Failure      503  {object}  object{code=int,message=string}  "invoice-service 不可用或逾時"
// @Router       /app/seller/invoices/complaints [get]
func (h *Holder) ListSellerInvoiceComplaints(c *gin.Context) {
	storeID, ok := h.resolveStoreID(c)
	if !ok {
		return
	}

	page, pageSize, ok := invoicePaging(c, defaultPageSizeStr)
	if !ok {
		return
	}

	resp, err := h.Invoice.ListSellerInvoiceComplaints(c.Request.Context(), &pb.ListSellerInvoiceComplaintsRequest{
		StoreId:  int64(storeID),
		Status:   c.Query("status"),
		Sort:     c.Query("sort"),
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

// GetSellerInvoiceComplaint 賣家申訴詳情
// @Summary      賣家申訴詳情
// @Description  賣家申訴詳情（58 號設計書 §6.1 端點 35）
// @Tags         電子發票 (Invoice)
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        complaint_id  path  string  true  "申訴案件 ID"
// @Success      200  {object}  object{code=int,message=string,data=InvoiceComplaintInfo}
// @Failure      401  {object}  object{code=int,message=string}  "未登入或 JWT 無效"
// @Failure      403  {object}  object{code=int,message=string}  "非賣家身分"
// @Failure      404  {object}  object{code=int,message=string}  "9309（Tier A）"
// @Failure      429  {object}  object{code=int,message=string}  "請求過於頻繁"
// @Failure      503  {object}  object{code=int,message=string}  "invoice-service 不可用或逾時"
// @Router       /app/seller/invoices/complaints/{complaint_id} [get]
func (h *Holder) GetSellerInvoiceComplaint(c *gin.Context) {
	storeID, ok := h.resolveStoreID(c)
	if !ok {
		return
	}

	complaintId := c.Param("complaint_id")
	if complaintId == "" {
		response.BadRequest(c, "complaint_id is required")
		return
	}

	resp, err := h.Invoice.GetSellerInvoiceComplaint(c.Request.Context(), &pb.GetSellerInvoiceComplaintRequest{
		StoreId:     int64(storeID),
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

// RespondSellerInvoiceComplaint 賣家回覆申訴
// @Summary      賣家回覆申訴
// @Description  賣家回覆申訴（58 號設計書 §6.1 端點 36）
// @Tags         電子發票 (Invoice)
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        Idempotency-Key  header  string  true  "冪等鍵，^[A-Za-z0-9._:-]{8,128}$"
// @Param        complaint_id  path  string  true  "申訴案件 ID"
// @Param        request  body  RespondSellerInvoiceComplaintRequest  true  "賣家回覆申訴"
// @Success      200  {object}  object{code=int,message=string,data=InvoiceComplaintInfo}
// @Failure      400  {object}  object{code=int,message=string}
// @Failure      401  {object}  object{code=int,message=string}  "未登入或 JWT 無效"
// @Failure      403  {object}  object{code=int,message=string}  "非賣家身分"
// @Failure      404  {object}  object{code=int,message=string}  "9309（Tier A）"
// @Failure      409  {object}  object{code=int,message=string}  "9322 / 9336 案件當前狀態不允許 / 9350 同鍵不同內容 / 9351 同鍵處理中"
// @Failure      429  {object}  object{code=int,message=string}  "請求過於頻繁"
// @Failure      503  {object}  object{code=int,message=string}  "invoice-service 不可用或逾時"
// @Router       /app/seller/invoices/complaints/{complaint_id}/responses [post]
func (h *Holder) RespondSellerInvoiceComplaint(c *gin.Context) {
	storeID, ok := h.resolveStoreID(c)
	if !ok {
		return
	}

	complaintId := c.Param("complaint_id")
	if complaintId == "" {
		response.BadRequest(c, "complaint_id is required")
		return
	}

	var body RespondSellerInvoiceComplaintRequest
	if err := c.ShouldBindJSON(&body); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	resp, err := h.Invoice.RespondSellerInvoiceComplaint(c.Request.Context(), &pb.RespondSellerInvoiceComplaintRequest{
		StoreId:         int64(storeID),
		ComplaintId:     complaintId,
		Action:          body.Action,
		Note:            body.Note,
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

// ReportExternalIssuance 賣家回報自行開立的發票號碼
// @Summary      賣家回報自行開立的發票號碼
// @Description  賣家回報自行開立的發票號碼（58 號設計書 §6.1 端點 62）
// @Tags         電子發票 (Invoice)
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        Idempotency-Key  header  string  true  "冪等鍵，^[A-Za-z0-9._:-]{8,128}$"
// @Param        job_id  path  string  true  "發票工作 ID"
// @Param        request  body  ReportExternalIssuanceRequest  true  "賣家回報自行開立的發票號碼"
// @Success      200  {object}  object{code=int,message=string,data=InvoiceJobInfo}
// @Failure      400  {object}  object{code=int,message=string}  "號碼／日期格式、invoiceDate 早於付款日"
// @Failure      401  {object}  object{code=int,message=string}  "未登入或 JWT 無效"
// @Failure      403  {object}  object{code=int,message=string}  "非賣家身分"
// @Failure      404  {object}  object{code=int,message=string}  "9340（Tier A）"
// @Failure      409  {object}  object{code=int,message=string}  "9343 版本衝突 / 9349 尚未解鎖人工處理 / 9350 冪等鍵 payload 不符 / 9351 同鍵處理中"
// @Failure      429  {object}  object{code=int,message=string}  "請求過於頻繁"
// @Failure      503  {object}  object{code=int,message=string}  "invoice-service 不可用或逾時"
// @Router       /app/seller/invoices/jobs/{job_id}/external_issuances [post]
func (h *Holder) ReportExternalIssuance(c *gin.Context) {
	storeID, ok := h.resolveStoreID(c)
	if !ok {
		return
	}

	jobId := c.Param("job_id")
	if jobId == "" {
		response.BadRequest(c, "job_id is required")
		return
	}

	var body ReportExternalIssuanceRequest
	if err := c.ShouldBindJSON(&body); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	resp, err := h.Invoice.ReportExternalIssuance(c.Request.Context(), &pb.ReportExternalIssuanceRequest{
		StoreId:           int64(storeID),
		JobId:             jobId,
		InvoiceNumber:     body.InvoiceNumber,
		InvoiceDate:       body.InvoiceDate,
		EvidenceAssetKeys: body.EvidenceAssetKeys,
		Note:              body.Note,
		ExpectedVersion:   body.ExpectedVersion,
		IdempotencyKey:    c.GetHeader(middleware.IdempotencyKeyHeader),
	})
	if err != nil {
		response.GrpcErr(c, "invoice", err)
		return
	}

	data := convInvInvoiceJobInfo(resp.GetData())
	// preserveData 業務碼不會轉成 err（interceptor 對它們 return nil），必須自己看 code。
	if resp.GetCode() != 0 {
		response.Biz(c, resp.GetCode(), resp.GetMessage(), data)
		return
	}
	response.OK(c, data)
}

// ListOrderExternalIssuances 查本訂單可回報／已回報的自行開立案件
// @Summary      查本訂單可回報／已回報的自行開立案件
// @Description  查本訂單可回報／已回報的自行開立案件（58 號設計書 §6.1 端點 62R）
// @Tags         電子發票 (Invoice)
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        order_no  path  string  true  "訂單編號"
// @Param        page  query  string  false  "頁碼，數字字串 ≥ 1，預設 1"
// @Param        page_size  query  string  false  "每頁筆數，數字字串 1–50，預設 10（逾越一律 400，不夾住）"
// @Success      200  {object}  object{code=int,message=string,data=object{records=[]InvoiceJobInfo,total=string,page=string,pageSize=string}}
// @Failure      400  {object}  object{code=int,message=string}  "參數格式錯誤"
// @Failure      401  {object}  object{code=int,message=string}  "未登入或 JWT 無效"
// @Failure      403  {object}  object{code=int,message=string}  "非賣家身分"
// @Failure      429  {object}  object{code=int,message=string}  "請求過於頻繁"
// @Failure      503  {object}  object{code=int,message=string}  "invoice-service 不可用或逾時"
// @Router       /app/seller/invoices/orders/{order_no}/external_issuances [get]
func (h *Holder) ListOrderExternalIssuances(c *gin.Context) {
	storeID, ok := h.resolveStoreID(c)
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

	resp, err := h.Invoice.ListOrderExternalIssuances(c.Request.Context(), &pb.ListOrderExternalIssuancesRequest{
		StoreId:  int64(storeID),
		OrderNo:  orderNo,
		Page:     page,
		PageSize: pageSize,
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
