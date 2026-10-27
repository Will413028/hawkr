package handler

import (
	"github.com/Will413028/hawkr/gateway/internal/response"
	pb "github.com/Will413028/hawkr/go-pkg/proto/payment"
	"github.com/gin-gonic/gin"
)

// ==================== DTO（Apple Pay Web Merchant Validation）====================

// ValidateApplePayMerchantRequest Apple Pay on Web merchant validation 請求
// 對應前端 ApplePaySession.onvalidatemerchant / W3C PaymentRequest.onmerchantvalidation 取得 validationURL 後送出
type ValidateApplePayMerchantRequest struct {
	// Apple 提供的一次性 validationURL（後端校驗 host 必須以 .apple.com 結尾、path 必須以 /paymentservices/ 開頭、scheme=https、port=443、不含 userInfo、長度 ≤ 2048）
	ValidationUrl string `json:"validationURL" binding:"required" example:"https://apple-pay-gateway-cert.apple.com/paymentservices/startSession"`
	// 前端發起付款的網域（後端依 APPLE_PAY_DOMAIN_ALLOWLIST 校驗、長度 ≤ 253）
	DomainName string `json:"domainName" binding:"required" example:"web-dev.example.com"`
	// Apple Pay sheet 顯示的商家名稱（一般為商品名稱或品牌名）
	DisplayName string `json:"displayName" binding:"required" example:"Hawkr"`
}

// ValidateApplePayMerchantResponse Apple Pay on Web merchant validation 回應
type ValidateApplePayMerchantResponse struct {
	// Apple 回應原文 JSON，前端原樣傳入 ApplePaySession.completeMerchantValidation() / event.complete()
	MerchantSessionJson string `json:"merchantSessionJson"`
}

// ValidateApplePayMerchant Apple Pay on Web merchant 驗證
// @Summary      Apple Pay on Web merchant 驗證
// @Description  Apple Pay 付款 sheet 啟動時，前端 onvalidatemerchant / onmerchantvalidation callback 會拿到 Apple 提供的一次性 validationURL；前端把這個 URL 送來後端後，由後端使用 Apple Pay Merchant Identity Certificate 進行 mTLS POST，原樣回傳 merchantSession JSON 給前端調用 completeMerchantValidation。
// @Description
// @Description  **安全規則**：
// @Description  - `validationURL` 必須 scheme=https、host 以 `.apple.com` 結尾、path 以 `/paymentservices/` 開頭、port=443（或省略）、不含 userInfo、長度 ≤ 2048；URL 經 `normalize()` 後再檢查 path 防 traversal
// @Description  - `domainName` 必須在後端 `APPLE_PAY_DOMAIN_ALLOWLIST` 內、長度 ≤ 253
// @Description  - 違反任一規則一律回業務碼 8028 `APPLE_PAY_VALIDATION_URL_REJECTED`
// @Description
// @Description  **業務錯誤碼**（PaymentBusinessException 既有架構：HTTP 409 + envelope `message` 內帶業務碼 `[8028]` / `[8029]` / `[8030]`）：
// @Description  - `[8028] APPLE_PAY_VALIDATION_URL_REJECTED` 驗證網址不符合白名單 / 網域不被允許 / 含 userInfo / 非 https / port 非 443 / 長度超限
// @Description  - `[8029] APPLE_PAY_CERT_LOAD_FAILED` Apple Pay 憑證載入失敗（檔不存在、PKCS#1 不支援、cert/key 不配對）
// @Description  - `[8030] APPLE_PAY_MERCHANT_VALIDATION_FAILED` Apple 端 mTLS 驗證失敗（非 2xx 或網路 / TLS 錯）
// @Description  - 詳見 [05 號設計書 §5.11.7](../docs/詳細設計書/05-支付服務設計書.md)
// @Description
// @Description  **前端建議**：依 envelope `message` 開頭的 `[<code>]` 業務碼做分流；不要單看 HTTP status，因為 409 同時涵蓋三種業務情境。文案策略見設計書 §5.11.7。
// @Tags         支付 (Payment)
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        request  body  ValidateApplePayMerchantRequest  true  "Apple Pay merchant validation 請求"
// @Success      200  {object}  object{code=int,message=string,data=ValidateApplePayMerchantResponse}  "成功取得 merchantSession"
// @Failure      400  {object}  object{code=int,message=string}  "request body 格式錯誤"
// @Failure      401  {object}  object{code=int,message=string}  "JWT 缺失 / 失效"
// @Failure      409  {object}  object{code=int,message=string}  "[8028] 驗證網址 / 網域不符 ; [8029] 憑證載入失敗 ; [8030] Apple mTLS 失敗"
// @Failure      503  {object}  object{code=int,message=string}  "payment-service 不可用 / gRPC 連線失敗"
// @Router       /app/payments/apple_pay/validate_merchant [post]
func (h *Holder) ValidateApplePayMerchant(c *gin.Context) {
	var req ValidateApplePayMerchantRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "invalid request: "+err.Error())
		return
	}

	resp, err := h.Payment.ValidateApplePayMerchant(c.Request.Context(), &pb.ValidateApplePayMerchantRequest{
		ValidationUrl: req.ValidationUrl,
		DomainName:    req.DomainName,
		DisplayName:   req.DisplayName,
	})
	if err != nil {
		response.GrpcErr(c, "payment", err)
		return
	}

	response.GrpcResult(c, resp.Code, resp.Message, ValidateApplePayMerchantResponse{
		MerchantSessionJson: resp.MerchantSessionJson,
	})
}
