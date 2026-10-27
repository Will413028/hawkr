package handler

import (
	aipb "github.com/Will413028/hawkr/go-pkg/proto/ai"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/Will413028/hawkr/gateway/internal/middleware"
	"github.com/Will413028/hawkr/gateway/internal/response"
	"github.com/gin-gonic/gin"
)

// vendorPortalVerificationProto 把入口附上的驗證紀錄原樣帶給 ai。context 裡沒有紀錄
// 時回 nil，由 ai 以 PermissionDenied 擋下（fail-closed 在 ai 端也成立）。
func vendorPortalVerificationProto(c *gin.Context) *aipb.VendorPortalVerification {
	v, ok := middleware.VendorPortalVerificationFrom(c)
	if !ok {
		return nil
	}
	return &aipb.VendorPortalVerification{
		Method:     v.Method,
		Email:      v.Email,
		VerifiedAt: timestamppb.New(v.VerifiedAt),
		SessionId:  v.SessionID,
	}
}

// 站外業者的三支**正式寫入**。
//
// 🔴 這三條路由掛 `middleware.RequireVendorPortalGoogleVerified()`，草稿那兩條不掛。
// 判準是「可覆寫」與「不可撤回」的差別：草稿可以一直改；正式送出產生的
// `sellerResponseVersion` 是 immutable，買家接受 exact version 即商務 award ⇒ 不可
// 撤回的商務承諾不能只靠一條七天有效的邀請連結（拿到信的任何人都能代該業者承諾價格）。
//
// ⚠️ 逐欄語意與賣家版相同，DTO 也共用（`ProcurementSellerResponseVersionDTO`／
// `ProcurementSellerResponseClosureDTO`）——兩側看到的內容必須一致，這是 doc 64 §89
// 「兩側投影必須相同」的同一條要求。

// ⛔ **沒有 expectedSellerFacingPayloadHash**（賣家版有）。那個 hash 不對站外方
// 暴露，站外這條 lane 的 CAS 是 version-only，理由見 ai.proto。留一個永遠是空字串
// 的欄位只會誤導呼叫端以為它有作用。
type VendorPortalSubmitResponseRequest struct {
	ExpectedPublicationVersion    string                      `json:"expectedPublicationVersion"`
	ExpectedSellerResponseVersion string                      `json:"expectedSellerResponseVersion"`
	Quote                         VendorPortalQuotePayloadDTO `json:"quote"`
}

type VendorPortalWithdrawResponseRequest struct {
	ExpectedSellerResponseVersion string `json:"expectedSellerResponseVersion"`
	ReasonCode                    string `json:"reasonCode"`
}

type VendorPortalDeclineRequest struct {
	ReasonCode string `json:"reasonCode"`
}

// VendorPortalSubmitResponse 站外業者正式送出回價。
//
// @Summary      Hawkr 幫找 — 站外業者送出回價
// @Description  正式送出，產生 immutable 的 sellerResponseVersion；買家接受 exact version 即商務 award。與草稿不同，這裡會驗金額恆等式。
// @Description  🔴 另需 session 已有 Google 驗證紀錄（單體的 VerifySession），驗證紀錄隨請求寫入送出的版本。穩定冪等鍵走 Idempotency-Key header。
// @Tags         VendorPortal
// @Accept       json
// @Produce      json
// @Param        rfq_recipient_id  path    string  true  "邀請收件人 ID（TSID 字串）"
// @Param        Idempotency-Key   header  string  true  "穩定冪等鍵"
// @Param        request  body  VendorPortalSubmitResponseRequest  true  "回價內容與 CAS 欄位"
// @Success      200  {object}  object{code=int,message=string,data=ProcurementSellerResponseVersionDTO}
// @Failure      400  {object}  object{code=int,reason=string,message=string,traceId=string}
// @Failure      403  {object}  object{code=int,message=string,traceId=string}
// @Failure      404  {object}  object{code=int,message=string,traceId=string}
// @Failure      409  {object}  object{code=int,reason=string,message=string,traceId=string}
// @Router       /public/vendor_portal/rfqs/{rfq_recipient_id}/response_submissions [post]
func (h *Holder) VendorPortalSubmitResponse(c *gin.Context) {
	recipientID, ok := vendorPortalBoundRecipientID(c)
	if !ok {
		return
	}
	key, ok := requireIdempotencyKey(c)
	if !ok {
		return
	}
	var req VendorPortalSubmitResponseRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		badProcurementRequest(c, "invalid request body")
		return
	}
	result, err := h.AIProcurement.SubmitVendorPortalResponse(c.Request.Context(),
		&aipb.SubmitVendorPortalResponseRequest{
			RfqRecipientId:                recipientID,
			ClientRequestId:               key,
			ExpectedPublicationVersion:    req.ExpectedPublicationVersion,
			ExpectedSellerResponseVersion: req.ExpectedSellerResponseVersion,
			Quote:                         req.Quote.toProto(),
			Verification:                  vendorPortalVerificationProto(c),
		})
	if err != nil {
		writeProcurementError(c, err)
		return
	}
	response.OK(c, sellerResponseVersionFromProto(result))
}

// VendorPortalWithdrawResponse 站外業者撤回已送出的回價。
//
// @Summary      Hawkr 幫找 — 站外業者撤回回價
// @Description  撤回追加成指令紀錄，舊版本 payload 不改寫。🔴 另需 session 已有 Google 驗證紀錄（單體的 VerifySession），驗證紀錄隨請求寫入送出的版本或撤回、婉拒的指令紀錄。穩定冪等鍵走 Idempotency-Key header。
// @Tags         VendorPortal
// @Accept       json
// @Produce      json
// @Param        rfq_recipient_id    path    string  true  "邀請收件人 ID（TSID 字串）"
// @Param        seller_response_id  path    string  true  "回價身分 ID（TSID 字串）"
// @Param        Idempotency-Key     header  string  true  "穩定冪等鍵"
// @Param        request  body  VendorPortalWithdrawResponseRequest  true  "CAS 欄位與理由"
// @Success      200  {object}  object{code=int,message=string,data=ProcurementSellerResponseClosureDTO}
// @Failure      403  {object}  object{code=int,message=string,traceId=string}
// @Failure      409  {object}  object{code=int,reason=string,message=string,traceId=string}
// @Router       /public/vendor_portal/rfqs/{rfq_recipient_id}/seller_responses/{seller_response_id}/withdrawals [post]
func (h *Holder) VendorPortalWithdrawResponse(c *gin.Context) {
	recipientID, ok := vendorPortalBoundRecipientID(c)
	if !ok {
		return
	}
	key, ok := requireIdempotencyKey(c)
	if !ok {
		return
	}
	var req VendorPortalWithdrawResponseRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		badProcurementRequest(c, "invalid request body")
		return
	}
	out, err := h.AIProcurement.WithdrawVendorPortalResponse(c.Request.Context(),
		&aipb.WithdrawVendorPortalResponseRequest{
			RfqRecipientId:                recipientID,
			SellerResponseId:              c.Param("seller_response_id"),
			ClientRequestId:               key,
			ExpectedSellerResponseVersion: req.ExpectedSellerResponseVersion,
			ReasonCode:                    req.ReasonCode,
			Verification:                  vendorPortalVerificationProto(c),
		})
	if err != nil {
		writeProcurementError(c, err)
		return
	}
	response.OK(c, sellerResponseClosureFromProto(out))
}

// VendorPortalDeclineRfq 站外業者婉拒這次詢價。
//
// @Summary      Hawkr 幫找 — 站外業者婉拒詢價
// @Description  婉拒後這張 publication 對該收件人寫入關閉。🔴 另需 session 已有 Google 驗證紀錄（單體的 VerifySession），驗證紀錄隨請求寫入送出的版本或撤回、婉拒的指令紀錄。穩定冪等鍵走 Idempotency-Key header。
// @Tags         VendorPortal
// @Accept       json
// @Produce      json
// @Param        rfq_recipient_id  path    string  true  "邀請收件人 ID（TSID 字串）"
// @Param        Idempotency-Key   header  string  true  "穩定冪等鍵"
// @Param        request  body  VendorPortalDeclineRequest  true  "婉拒理由"
// @Success      200  {object}  object{code=int,message=string,data=ProcurementSellerResponseClosureDTO}
// @Failure      403  {object}  object{code=int,message=string,traceId=string}
// @Failure      409  {object}  object{code=int,reason=string,message=string,traceId=string}
// @Router       /public/vendor_portal/rfqs/{rfq_recipient_id}/declines [post]
func (h *Holder) VendorPortalDeclineRfq(c *gin.Context) {
	recipientID, ok := vendorPortalBoundRecipientID(c)
	if !ok {
		return
	}
	key, ok := requireIdempotencyKey(c)
	if !ok {
		return
	}
	var req VendorPortalDeclineRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		badProcurementRequest(c, "invalid request body")
		return
	}
	out, err := h.AIProcurement.DeclineVendorPortalRfqPublication(c.Request.Context(),
		&aipb.DeclineVendorPortalRfqPublicationRequest{
			RfqRecipientId:  recipientID,
			ClientRequestId: key,
			ReasonCode:      req.ReasonCode,
			Verification:    vendorPortalVerificationProto(c),
		})
	if err != nil {
		writeProcurementError(c, err)
		return
	}
	response.OK(c, sellerResponseClosureFromProto(out))
}
