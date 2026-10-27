package handler

import (
	"net/http"

	"github.com/Will413028/hawkr/gateway/internal/response"
	pb "github.com/Will413028/hawkr/go-pkg/proto/shipping"
	"github.com/gin-gonic/gin"
	"google.golang.org/grpc/codes"
	grpcstatus "google.golang.org/grpc/status"
)

// ==================== DTO ====================

// GetStoreMapParamsReq 取得門市地圖參數請求
type GetStoreMapParamsReq struct {
	AppType   string `json:"appType" binding:"required"`
	MobileTag string `json:"mobileTag"`
}

// StoreMapFormDataResp 門市地圖表單資料回應
type StoreMapFormDataResp struct {
	FormAction  string `json:"formAction"`
	MerId       string `json:"merId"`
	Version     string `json:"version"`
	EncryptInfo string `json:"encryptInfo"`
	HashInfo    string `json:"hashInfo"`
	RedirectUrl string `json:"redirectUrl"`
}

// StoreMapResultResp 門市選擇結果回應
type StoreMapResultResp struct {
	StoreId     string `json:"storeId"`
	StoreName   string `json:"storeName"`
	Address     string `json:"address"`
	InsularArea string `json:"insularArea"`
	StoreType   string `json:"storeType"`
}

// ==================== Handler ====================

// GetStoreMapParams 取得超商門市地圖加密參數
// @Summary      取得超商門市地圖參數
// @Description  取得 PayUni 門市地圖 Form POST 所需的加密參數，前端用 redirectUrl 開啟 WebView（App）或 popup（Web）
// @Tags         超商門市地圖 (Store Map)
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        body  body  GetStoreMapParamsReq  true  "appType: buyer/seller/web（決定回調 302 目標：buyer/seller→App deep link，web→{WEB_FRONTEND_URL}/store-map/return），mobileTag: Y(手機版)/N(PC版)，預設 Y；web 建議傳 N"
// @Success      200  {object}  object{code=int,data=StoreMapFormDataResp}
// @Failure      400  {object}  object{code=int,message=string}
// @Failure      401  {object}  object{code=int,message=string}
// @Failure      503  {object}  object{code=int,message=string}
// @Router       /app/store_map_sessions [post]
func (h *Holder) GetStoreMapParams(c *gin.Context) {
	accountID, ok := getAccountID(c)
	if !ok || accountID == 0 {
		response.Unauthorized(c)
		return
	}

	var req GetStoreMapParamsReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "invalid request body: appType is required")
		return
	}

	if req.AppType != "buyer" && req.AppType != "seller" && req.AppType != "web" {
		response.BadRequest(c, "appType must be 'buyer', 'seller' or 'web'")
		return
	}

	resp, err := h.Shipping.GetStoreMapParams(c.Request.Context(), &pb.GetStoreMapParamsRequest{
		AccountId: accountID,
		AppType:   req.AppType,
		MobileTag: req.MobileTag,
	})
	if err != nil {
		response.GrpcErr(c, "shipping", err)
		return
	}

	if resp.Code != 0 {
		response.GrpcResult(c, resp.Code, resp.Message, nil)
		return
	}

	// 透傳微服務回傳值（redirectUrl 由 Shipping Service 組裝）
	data := StoreMapFormDataResp{
		FormAction:  resp.Data.GetFormAction(),
		MerId:       resp.Data.GetMerId(),
		Version:     resp.Data.GetVersion(),
		EncryptInfo: resp.Data.GetEncryptInfo(),
		HashInfo:    resp.Data.GetHashInfo(),
		RedirectUrl: resp.Data.GetRedirectUrl(),
	}
	response.GrpcResult(c, resp.Code, resp.Message, data)
}

// StoreMapRedirect 重導向到 PayUni 門市地圖頁
// GET /store_map/redirect?id={opaqueId}
// 此端點供 APP WebView 直接開啟，會自動 Form POST 到 PayUni 門市地圖。
// #1394：改讀 opaque id → gRPC 取回後端 canonical 表單 → html/template 安全 render
// （舊 base64 `?data=` 反射 XSS 路徑已物理刪除）。
// @Summary      重導向到 PayUni 門市地圖頁
// @Description  以後端產生的一次性 opaque id 取回 canonical 表單資料並自動 Form POST 到 PayUni 門市地圖，供 APP WebView 直接開啟
// @Tags         超商門市地圖 (Store Map)
// @Produce      html
// @Param        id  query  string  true  "後端產生的一次性 redirect id"
// @Success      200  {string}  string  "自動提交到 PayUni 的 HTML 頁面"
// @Failure      400  {string}  string  "缺少參數"
// @Failure      404  {string}  string  "連結已失效或不存在"
// @Failure      503  {string}  string  "服務不可用"
// @Router       /store_map/redirect [get]
func (h *Holder) StoreMapRedirect(c *gin.Context) {
	redirectID := c.Query("id")
	if redirectID == "" {
		renderPayUniRedirectError(c, http.StatusBadRequest, "缺少必要參數")
		return
	}

	resp, err := h.Shipping.GetStoreMapRedirectForm(c.Request.Context(), redirectID)
	if err != nil {
		if grpcstatus.Code(err) == codes.NotFound {
			renderPayUniRedirectError(c, http.StatusNotFound, "門市地圖連結已失效或不存在，請重新操作")
			return
		}
		renderPayUniRedirectError(c, http.StatusServiceUnavailable, "物流服務暫時無法使用")
		return
	}

	renderPayUniRedirect(c, payUniRedirectForm{
		Title:       "正在開啟門市地圖",
		FormID:      "storeMapForm",
		ActionURL:   resp.GetFormAction(),
		MerID:       resp.GetMerId(),
		Version:     resp.GetVersion(),
		EncryptInfo: resp.GetEncryptInfo(),
		HashInfo:    resp.GetHashInfo(),
		IsPlatForm:  "1", // 門市地圖固定平台模式（原 hardcoded IsPlatForm=1）
	})
}

// StoreMapReturn 門市選擇結果回調
// POST /store_map/return?app=buyer
// PayUni 門市地圖選擇完成後以 Form POST 導回此端點
// Gateway 呼叫 Shipping Service 解密門市資料，302 redirect 到 App Deep Link
// @Summary      門市選擇結果回調
// @Description  PayUni 門市地圖選店完成後的回調，解密門市資訊後 302 redirect 到 Deep Link 導回 App
// @Tags         超商門市地圖 (Store Map)
// @Param        app  query  string  false  "APP 類型（buyer/seller），預設 buyer"
// @Success      302  "Redirect to Deep Link"
// @Failure      400  {string}  string  "參數錯誤"
// @Failure      503  {string}  string  "服務不可用"
// @Router       /store_map/return [post]
func (h *Holder) StoreMapReturn(c *gin.Context) {
	// PayUni 門市地圖以 Form POST 回傳加密結果
	encryptInfo := c.PostForm("EncryptInfo")
	hashInfo := c.PostForm("HashInfo")
	appType := c.DefaultQuery("app", "buyer")

	if encryptInfo == "" || hashInfo == "" {
		c.String(http.StatusBadRequest, "缺少加密參數")
		return
	}

	resp, err := h.Shipping.DecryptStoreMapResult(c.Request.Context(), &pb.DecryptStoreMapResultRequest{
		EncryptInfo: encryptInfo,
		HashInfo:    hashInfo,
		AppType:     appType,
	})
	if err != nil {
		c.String(http.StatusServiceUnavailable, "解密門市資訊失敗")
		return
	}

	// 透傳 Shipping Service 組裝的 Deep Link URL，302 redirect 讓 WebView 攔截 URL Scheme
	c.Redirect(http.StatusFound, resp.GetRedirectUrl())
}
