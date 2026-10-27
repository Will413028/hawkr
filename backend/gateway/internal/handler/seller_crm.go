package handler

import (
	"context"
	"errors"
	"fmt"
	"strconv"

	"github.com/Will413028/hawkr/gateway/internal/buyerid"
	"github.com/Will413028/hawkr/gateway/internal/response"
	"github.com/Will413028/hawkr/go-pkg/httputil"
	"github.com/Will413028/hawkr/go-pkg/identity"
	couponpb "github.com/Will413028/hawkr/go-pkg/proto/coupon"
	crmpb "github.com/Will413028/hawkr/go-pkg/proto/crm"
	orderpb "github.com/Will413028/hawkr/go-pkg/proto/order"
	"github.com/gin-gonic/gin"
)

// sellerHasBuyerRelationshipByBuyerID 回報買家（以 buyer_id 指名）是否與此賣家
// 有訂單關聯。CRM 客戶記錄由 order.completed 事件自動建檔，因此「在此賣家有
// 訂單」是涵蓋 CRM 客戶、最根本的關聯訊號。回傳買家 PII（手機 / 暱稱 / 頭像）
// 前必須通過此檢查，否則任一賣家可越權查詢全平台任意買家（B-198 IDOR）。
// order-service 訂單持久化的就是 buyer_id，所以這裡不需要任何解析。
func (h *Holder) sellerHasBuyerRelationshipByBuyerID(ctx context.Context, storeID identity.StoreID, buyerID identity.BuyerID) (bool, error) {
	resp, err := h.Order.GetBuyerOrdersForSeller(ctx, &orderpb.GetBuyerOrdersForSellerRequest{
		BuyerId: int64(buyerID),
		StoreId: int64(storeID),
		Page:    "1",
		Size:    "1",
	})
	if err != nil {
		return false, err
	}
	if resp.Code != 0 {
		return false, nil
	}
	total, _ := strconv.ParseInt(resp.Total, 10, 64)
	return total > 0, nil
}

func (h *Holder) sellerHasBuyerRelationship(ctx context.Context, storeID identity.StoreID, rawBuyerAccountID identity.AccountID) (identity.BuyerID, bool, error) {
	buyerID, err := h.BuyerID.CurrentBuyerID(ctx, rawBuyerAccountID)
	if err != nil {
		if errors.Is(err, buyerid.ErrNoCurrentBuyer) {
			return 0, false, nil
		}
		return 0, false, err
	}
	related, err := h.sellerHasBuyerRelationshipByBuyerID(ctx, storeID, buyerID)
	return buyerID, related, err
}

func (h *Holder) resolveCustomerBuyerID(c *gin.Context) (identity.BuyerID, bool) {
	rawBuyerID, err := strconv.ParseInt(c.Param("buyerId"), 10, 64)
	if err != nil || rawBuyerID <= 0 {
		response.BadRequest(c, "invalid customer id")
		return 0, false
	}
	return identity.BuyerID(rawBuyerID), true
}

// BuyerLookupResponse 買家精準查詢回應
type BuyerLookupResponse struct {
	UID       string `json:"uid"`
	BuyerName string `json:"buyerName"`
	Phone     string `json:"phone"`
	AvatarKey string `json:"avatarKey"` // 買家頭像 objectKey（SSOT；49 §6.1）
	AvatarURL string `json:"avatarUrl"` // 買家頭像顯示 URL（Gateway 組；http 外部頭像 passthrough）
}

// LookupBuyer 依手機號或 buyer_id 精準查詢買家基本資訊。
// @Summary      精準查詢買家
// @Description  依手機號或 buyer_id 查詢買家基本資訊（暱稱、手機、頭像），phone/buyer_id 二者擇一提供
// @Tags         商家CRM (Seller CRM)
// @Produce      json
// @Security     BearerAuth
// @Param        phone       query  string  false  "買家手機號（與 buyer_id 互斥）"
// @Param        buyer_id    query  int64   false  "買家 buyer_id（與 phone 互斥，已是現任值免解析）"
// @Success      200  {object}  object{code=int,data=BuyerLookupResponse}
// @Failure      400  {object}  object{code=int,message=string}
// @Failure      404  {object}  object{code=int,message=string}
// @Failure      503  {object}  object{code=int,message=string}
// @Router       /app/sellers/me/customers/lookup [get]
func (h *Holder) LookupBuyer(c *gin.Context) {
	storeID, ok := h.resolveStoreID(c)
	if !ok {
		return
	}

	phone := c.Query("phone")
	buyerIDRaw := c.Query("buyer_id")
	hasPhone := phone != ""
	hasBuyerID := buyerIDRaw != ""

	switch {
	case !hasPhone && !hasBuyerID:
		response.BadRequest(c, "phone or buyer_id is required")
		return
	case hasPhone && hasBuyerID:
		response.BadRequest(c, "phone and buyer_id are mutually exclusive")
		return
	}

	ctx := c.Request.Context()
	var buyerPhone string
	var resolvedBuyerID identity.BuyerID
	var related bool
	var err error

	// 授權：僅當買家與此賣家有訂單關聯時才可回傳 PII。無關聯時統一回 404
	// （與「買家不存在」同碼），避免賣家用手機號列舉平台用戶。
	//
	// buyer_id 來源值已是現任 buyer_id（CRM/CustomerLookup 解析過），直接查
	// 關聯即可；phone 來源值是 raw account_id（GetAccountByPhone 回傳），需經
	// sellerHasBuyerRelationship 解析成現任 buyer_id 再查——不可不分來源一律
	// 當 raw account_id 二次解析（同 coupon.go handleListBuyerCouponsForSeller
	// 的教訓）。
	switch {
	case hasPhone:
		resp, aerr := h.Account.GetAccountByPhone(ctx, phone)
		if aerr != nil {
			response.GrpcErr(c, "account", aerr)
			return
		}
		if resp.Data == nil {
			response.NotFound(c, "buyer not found")
			return
		}
		buyerPhone = resp.Data.Phone
		resolvedBuyerID, related, err = h.sellerHasBuyerRelationship(ctx, storeID, identity.AccountID(resp.Data.AccountId))
	default: // hasBuyerID
		id, perr := strconv.ParseInt(buyerIDRaw, 10, 64)
		if perr != nil || id <= 0 {
			response.BadRequest(c, "invalid buyer_id")
			return
		}
		resolvedBuyerID = identity.BuyerID(id)
		related, err = h.sellerHasBuyerRelationshipByBuyerID(ctx, storeID, resolvedBuyerID)
	}
	if err != nil {
		response.GrpcErr(c, "order", err)
		return
	}
	if !related {
		response.NotFound(c, "buyer not found")
		return
	}

	// 取得買家暱稱 + 頭像 CDN URL（resolvedBuyerID 已是解析後的 buyer_id/PK，
	// 必須走 ByBuyerID 版本查詢——buyer_id 設計上永不等於 account_id）
	meta, err := h.Buyer.GetBuyerMetaByBuyerID(ctx, resolvedBuyerID)
	if err != nil {
		response.GrpcErr(c, "buyer", err)
		return
	}
	if meta == nil {
		response.NotFound(c, "buyer not found")
		return
	}

	// buyer_id 查詢時，從 buyer profile 取得手機號
	if buyerPhone == "" {
		profile, err := h.Buyer.GetProfileByBuyerID(ctx, int64(resolvedBuyerID))
		if err != nil {
			response.GrpcErr(c, "buyer", err)
			return
		}
		if profile != nil {
			buyerPhone = profile.Phone
		}
	}

	response.OK(c, BuyerLookupResponse{
		UID:       fmt.Sprintf("%d", int64(resolvedBuyerID)),
		BuyerName: meta.Nickname,
		Phone:     buyerPhone,
		AvatarKey: meta.AvatarKey,
		AvatarURL: buildURL(h.MediaBaseURL, meta.AvatarKey),
	})
}

// GetBuyerMeta returns basic buyer info (nickname + avatar) by buyer_id.
// The buyer must have an order relationship with the caller seller; otherwise
// 404 is returned (B-198 IDOR fix — previously any seller could read any
// buyer's nickname/avatar).
//
// 2026-07-26: {buyerId} 從「account_id，內部解析成現任 buyer_id」改成直接收
// buyer_id。這支端點原本是全家族裡唯一自己 inline parse 的——path 收 account_id
// 卻在回應吐 buyerId，同一個 token 在請求與回應各指一條軸。現在兩邊同軸：收
// 什麼 buyer_id 就回什麼 buyer_id。
// @Summary      查詢買家基本資訊
// @Description  取得與此賣家有訂單關聯之買家的暱稱與頭像；無關聯回 404
// @Tags         商家CRM (Seller CRM)
// @Produce      json
// @Security     BearerAuth
// @Param        buyerId  path  int  true  "客戶的 buyer_id（顧客列表回傳的 buyerId；非帳號 ID）"
// @Success      200  {object}  object{code=int,data=object{buyerId=int,nickname=string,avatarKey=string,avatarUrl=string}}
// @Failure      400  {object}  object{code=int,message=string}
// @Failure      404  {object}  object{code=int,message=string}
// @Failure      503  {object}  object{code=int,message=string}
// @Router       /app/sellers/me/customers/{buyerId}/meta [get]
func (h *Holder) GetBuyerMeta(c *gin.Context) {
	storeID, ok := h.resolveStoreID(c)
	if !ok {
		return
	}

	// path param 就是 buyer_id，走全家族共用的單一入口（#1519 的修復）。
	buyerID, ok := h.resolveCustomerBuyerID(c)
	if !ok {
		return
	}

	// 授權：僅限與此賣家有訂單關聯的買家，否則回 404（見 LookupBuyer 說明）。
	related, err := h.sellerHasBuyerRelationshipByBuyerID(c.Request.Context(), storeID, buyerID)
	if err != nil {
		response.GrpcErr(c, "order", err)
		return
	}
	if !related {
		response.NotFound(c, "buyer not found")
		return
	}

	meta, err := h.Buyer.GetBuyerMetaByBuyerID(c.Request.Context(), buyerID)
	if err != nil {
		response.GrpcErr(c, "buyer", err)
		return
	}
	if meta == nil {
		response.NotFound(c, "buyer not found")
		return
	}

	response.OK(c, gin.H{
		"buyerId":   strconv.FormatInt(meta.BuyerId, 10),
		"nickname":  meta.Nickname,
		"avatarKey": meta.AvatarKey,                           // 買家頭像 objectKey（SSOT；49 §6.1）
		"avatarUrl": buildURL(h.MediaBaseURL, meta.AvatarKey), // 顯示 URL（Gateway 組；http 外部頭像 passthrough）
	})
}

// crmTagListItem 是 CRM 標籤在顧客列表回應中的形狀。
// storeId 是賣家的店家代理鍵（標籤的擁有者），不是客戶。
type crmTagListItem struct {
	ID      string `json:"id,omitempty"`
	StoreID string `json:"storeId,omitempty"`
	Name    string `json:"name,omitempty"`
}

// sellerTagResp 是三支「標籤」端點（ListMerchantTags / CreateSellerTag /
// GetCustomerTags）的對外形狀。
//
// ⚠️ 這三支原本把 *crmpb.Tag 直接丟給 response.GrpcResult，而 gin 的 c.JSON 走
// encoding/json —— proto 的 struct tag 就這樣直接變成公開 wire key。於是 proto
// 欄位 account_id 正名成 store_id 時，這三支的回應 key 也跟著從 "account_id"
// 靜默變成 "store_id"，中間沒有任何 mapping 層吸收。這個 struct 就是那層：
// 對外 key 由這裡決定，proto 之後再怎麼改名都不會漏出去。
//
// key 現為 camelCase 的 storeId，與 crmTagListItem / crmTagDetailItem 一致——
// 顧客列表／詳情裡的 tags[] 與這三支端點回的是同一種東西，不該有兩套 key 風格。
type sellerTagResp struct {
	ID      string `json:"id,omitempty"`
	StoreID string `json:"storeId,omitempty"`
	Name    string `json:"name,omitempty"`
}

// toSellerTagResp 把單一 proto Tag 轉成對外形狀；nil 進 nil 出，維持原本
// passthrough 在 Data 為 nil 時的行為。
func toSellerTagResp(t *crmpb.Tag) *sellerTagResp {
	if t == nil {
		return nil
	}
	return &sellerTagResp{
		ID:      t.GetId(),
		StoreID: t.GetStoreId(),
		Name:    t.GetName(),
	}
}

// toSellerTagRespList 對標籤列表做同樣轉換。nil slice 回 nil（而非空 slice），
// 維持原本直接透傳 proto slice 的 JSON 輸出。
func toSellerTagRespList(tags []*crmpb.Tag) []*sellerTagResp {
	if tags == nil {
		return nil
	}
	out := make([]*sellerTagResp, 0, len(tags))
	for _, t := range tags {
		out = append(out, toSellerTagResp(t))
	}
	return out
}

// crmCustomerListItem 是顧客列表單筆客戶的對外形狀（camelCase，與顧客詳情
// 一致）。
//
// 識別碼語意（2026-07-26 正名）：
//   - storeId — 賣家的店家代理鍵，**不是**客戶
//   - buyerId — 客戶的 buyers.buyer_id（per-stint PK，永不等於 account_id）。
//     這個值可以直接回頭打 /customers/{buyerId} 系列端點。
//   - id      — crm row 主鍵，別當任何人的 ID 用
//
// ⚠️ 這批欄位名是 breaking change：舊名 accountId 裝的是賣家 store surrogate、
// 舊名 customerAccountId 裝的是買家 buyer_id——同一個 JSON 物件裡兩個名字都在
// 說謊。賣家 App 端同批改成只讀 buyerId / storeId，兩邊必須一起上線。
//
// ⚠️ 前端一律只讀 buyerId，不得補 fallback chain。2026-07-26 的教訓：多層 ??
// 在 wire contract 上不是韌性，是把 breaking change 從 fail-fast 降級成
// silent-wrong（舊的 `customer_account_id ?? account_id ?? id` 讓「客戶 ID
// 靜默變成賣家自己的 store surrogate」藏了很久）。
//
// avatarKey/avatarUrl 成對是 49 §6.6 S9 的要求：crm-service 透傳 objectKey、
// Gateway 是唯一 URL 組裝點。
type crmCustomerListItem struct {
	ID           string           `json:"id,omitempty"`
	StoreID      string           `json:"storeId,omitempty"`
	BuyerID      string           `json:"buyerId,omitempty"`
	CustomerType string           `json:"customerType,omitempty"`
	TotalSpent   string           `json:"totalSpent,omitempty"`
	OrderCount   string           `json:"orderCount,omitempty"`
	CouponCount  string           `json:"couponCount,omitempty"`
	Nickname     string           `json:"nickname,omitempty"`
	AvatarKey    string           `json:"avatarKey,omitempty"` // 買家頭像 objectKey（SSOT；49 §6.1）
	AvatarURL    string           `json:"avatarUrl,omitempty"` // 顯示 URL（Gateway 組；http 外部頭像 passthrough）
	Phone        string           `json:"phone,omitempty"`
	Tags         []crmTagListItem `json:"tags,omitempty"`
}

// toCrmCustomerListItem 將 crm-service 的 CrmCustomer 轉成列表對外形狀。
func (h *Holder) toCrmCustomerListItem(cu *crmpb.CrmCustomer) crmCustomerListItem {
	item := crmCustomerListItem{
		ID:           cu.GetId(),
		StoreID:      cu.GetStoreId(),
		BuyerID:      cu.GetBuyerId(),
		CustomerType: cu.GetCustomerType(),
		TotalSpent:   cu.GetTotalSpent(),
		OrderCount:   cu.GetOrderCount(),
		CouponCount:  cu.GetCouponCount(),
		Nickname:     cu.GetNickname(),
		AvatarKey:    cu.GetAvatarKey(),
		AvatarURL:    buildURL(h.MediaBaseURL, cu.GetAvatarKey()),
		Phone:        cu.GetPhone(),
	}
	if tags := cu.GetTags(); len(tags) > 0 {
		item.Tags = make([]crmTagListItem, 0, len(tags))
		for _, t := range tags {
			item.Tags = append(item.Tags, crmTagListItem{
				ID:      t.GetId(),
				StoreID: t.GetStoreId(),
				Name:    t.GetName(),
			})
		}
	}
	return item
}

// crmTagDetailItem 是 CRM 標籤在顧客詳情回應中的形狀（camelCase）。
// storeId 是賣家的店家代理鍵（標籤的擁有者），不是客戶。
type crmTagDetailItem struct {
	ID      string `json:"id"`
	StoreID string `json:"storeId"`
	Name    string `json:"name"`
}

// crmCustomerDetail 是顧客詳情的對外形狀（零值一律輸出，維持原 protojson
// passthrough 的行為），只多一個 Gateway 組的 avatarUrl——protojson 直接吐
// proto 就沒有插入組裝結果的位置，而 49 §6.6 S9 要求 key/url 成對對外。
//
// 識別碼語意與 crmCustomerListItem 完全一致（storeId = 賣家店家代理鍵、
// buyerId = 客戶的 buyers.buyer_id、id = crm row 主鍵）；同樣是 2026-07-26 的
// breaking rename，前端只讀 buyerId、不得補 fallback chain。理由見
// crmCustomerListItem 的註解。
type crmCustomerDetail struct {
	ID                 string             `json:"id"`
	StoreID            string             `json:"storeId"`
	BuyerID            string             `json:"buyerId"`
	CustomerType       string             `json:"customerType"`
	TotalSpent         string             `json:"totalSpent"`
	OrderCount         string             `json:"orderCount"`
	CouponCount        string             `json:"couponCount"`
	Nickname           string             `json:"nickname"`
	AvatarKey          string             `json:"avatarKey"` // 買家頭像 objectKey（SSOT；49 §6.1）
	AvatarURL          string             `json:"avatarUrl"` // 顯示 URL（Gateway 組；http 外部頭像 passthrough）
	Phone              string             `json:"phone"`
	Address            string             `json:"address"`
	Email              string             `json:"email"`
	FrequentCategories []string           `json:"frequentCategories"`
	Tags               []crmTagDetailItem `json:"tags"`
}

// toCrmCustomerDetail 將 crm-service 的 CrmCustomer 轉成詳情對外形狀。
// slice 一律初始化成空 slice（非 nil），維持原 protojson EmitUnpopulated 的
// `[]` 輸出，不讓前端在「無標籤」時突然收到 null。
func (h *Holder) toCrmCustomerDetail(cu *crmpb.CrmCustomer) crmCustomerDetail {
	detail := crmCustomerDetail{
		ID:                 cu.GetId(),
		StoreID:            cu.GetStoreId(),
		BuyerID:            cu.GetBuyerId(),
		CustomerType:       cu.GetCustomerType(),
		TotalSpent:         cu.GetTotalSpent(),
		OrderCount:         cu.GetOrderCount(),
		CouponCount:        cu.GetCouponCount(),
		Nickname:           cu.GetNickname(),
		AvatarKey:          cu.GetAvatarKey(),
		AvatarURL:          buildURL(h.MediaBaseURL, cu.GetAvatarKey()),
		Phone:              cu.GetPhone(),
		Address:            cu.GetAddress(),
		Email:              cu.GetEmail(),
		FrequentCategories: make([]string, 0, len(cu.GetFrequentCategories())),
		Tags:               make([]crmTagDetailItem, 0, len(cu.GetTags())),
	}
	detail.FrequentCategories = append(detail.FrequentCategories, cu.GetFrequentCategories()...)
	for _, t := range cu.GetTags() {
		detail.Tags = append(detail.Tags, crmTagDetailItem{
			ID:      t.GetId(),
			StoreID: t.GetStoreId(),
			Name:    t.GetName(),
		})
	}
	return detail
}

// GetCustomerList returns list of customers for a seller
// @Summary      取得顧客列表
// @Description  取得商家的顧客列表，支援分頁、狀態篩選、關鍵字搜尋及標籤篩選
// @Tags         商家CRM (Seller CRM)
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        page        query  int     false  "頁碼（預設為 1）"
// @Param        page_size   query  int     false  "每頁筆數（預設為 20）"
// @Param        customer_type  query  string  false  "顧客分類篩選（” = 全部, normal / vip / blocklist）"
// @Param        keyword     query  string  false  "搜尋關鍵字"
// @Param        tag_id      query  string  false  "標籤 ID 篩選"
// @Param        sort_by     query  string  false  "排序欄位（預設為 created_at）"
// @Param        sort_order  query  string  false  "排序方向（預設為 desc）"
// @Success      200  {object}  object{code=int,message=string,data=object{total=int,list=[]object{id=int,storeId=int,buyerId=int,customerType=string,totalSpent=int,orderCount=int,couponCount=int,nickname=string,avatarKey=string,avatarUrl=string,phone=string,tags=[]object{id=int,storeId=int,name=string}}}}
// @Failure      401  {object}  object{code=int,message=string}
// @Failure      500  {object}  object{code=int,message=string}
// @Failure      503  {object}  object{code=int,message=string}
// @Router       /app/sellers/me/customers [get]
func (h *Holder) GetCustomerList(c *gin.Context) {
	storeID, ok := h.resolveStoreID(c)
	if !ok {
		return
	}

	page := httputil.ParsePageParam(c.Query("page"), 1)
	pageSize := httputil.ParsePageParam(c.Query("page_size"), 10)
	status := c.Query("customer_type") // "" = 全部
	keyword := c.Query("keyword")
	tagID := c.Query("tag_id")
	sortBy := c.DefaultQuery("sort_by", "created_at")
	sortOrder := c.DefaultQuery("sort_order", "desc")

	resp, err := h.Crm.GetCustomerList(c.Request.Context(), storeID, int32(page), int32(pageSize), status, keyword, tagID, sortBy, sortOrder)
	if err != nil {
		response.GrpcErr(c, "crm", err)
		return
	}
	if resp.Code != 0 {
		response.GrpcResult(c, resp.Code, resp.Message, nil)
		return
	}

	list := make([]crmCustomerListItem, 0, len(resp.Data.GetList()))
	for _, cu := range resp.Data.GetList() {
		list = append(list, h.toCrmCustomerListItem(cu))
	}

	response.OK(c, gin.H{
		"total": resp.Data.GetTotal(),
		"list":  list,
	})
}

// GetCustomerStats returns stats for a specific customer
// @Summary      取得顧客詳細資料
// @Description  取得指定買家在此賣家下的完整資料，包含：
// @Description  - 基本資料：buyerId、暱稱（nickname）、頭像（avatar）
// @Description  - 消費統計：消費總金額（total_spent）、訂單數（order_count）
// @Description  - 聯絡資訊：電話（phone）、地址（address）、Email（email）
// @Description  - 優惠券：此賣家發給該買家的優惠券數量（coupon_count）
// @Description  - 標籤：賣家給予此買家的標籤列表（tags）
// @Description  - CRM 分類：normal / vip / blocklist
// @Tags         商家CRM (Seller CRM)
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        buyerId  path  int  true  "客戶的 buyer_id（顧客列表回傳的 buyerId；非帳號 ID）"
// @Success      200  {object}  object{code=int,message=string,data=object{id=int,storeId=int,buyerId=int,customerType=string,totalSpent=int,orderCount=int,couponCount=int,nickname=string,avatarKey=string,avatarUrl=string,phone=string,address=string,email=string,frequentCategories=[]string,tags=[]object{id=int,storeId=int,name=string}}}
// @Failure      400  {object}  object{code=int,message=string}
// @Failure      401  {object}  object{code=int,message=string}
// @Failure      404  {object}  object{code=int,message=string}  "顧客不存在"
// @Failure      500  {object}  object{code=int,message=string}
// @Failure      503  {object}  object{code=int,message=string}
// @Router       /app/sellers/me/customers/{buyerId} [get]
func (h *Holder) GetCustomerStats(c *gin.Context) {
	storeID, ok := h.resolveStoreID(c)
	if !ok {
		return
	}

	customerID, ok := h.resolveCustomerBuyerID(c)
	if !ok {
		return
	}

	resp, err := h.Crm.GetCustomerStats(c.Request.Context(), storeID, customerID)
	if err != nil {
		response.GrpcErr(c, "crm", err)
		return
	}
	if resp.Code != 0 {
		response.GrpcResult(c, resp.Code, resp.Message, nil)
		return
	}

	response.OK(c, h.toCrmCustomerDetail(resp.Data))
}

// UpdateCustomerStatus updates customer status (e.g. blacklist)
// @Summary      更新顧客狀態
// @Description  更新顧客的狀態（例如加入黑名單），需提供狀態值及原因
// @Tags         商家CRM (Seller CRM)
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        buyerId  path  int                                 true  "客戶的 buyer_id（顧客列表回傳的 buyerId；非帳號 ID）"
// @Param        request  body  object{customerType=string}  true  "更新顧客分類請求（normal / vip / blocklist）"
// @Success      200  {object}  object{code=int,message=string}
// @Failure      400  {object}  object{code=int,message=string}
// @Failure      401  {object}  object{code=int,message=string}
// @Failure      404  {object}  object{code=int,message=string}  "顧客不存在"
// @Failure      500  {object}  object{code=int,message=string}
// @Failure      503  {object}  object{code=int,message=string}
// @Router       /app/sellers/me/customers/{buyerId} [patch]
func (h *Holder) UpdateCustomerStatus(c *gin.Context) {
	storeID, ok := h.resolveStoreID(c)
	if !ok {
		return
	}

	customerID, ok := h.resolveCustomerBuyerID(c)
	if !ok {
		return
	}

	var req struct {
		CustomerType string `json:"customerType" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "invalid request body")
		return
	}

	resp, err := h.Crm.UpdateCustomerStatus(c.Request.Context(), storeID, customerID, req.CustomerType)
	if err != nil {
		response.GrpcErr(c, "crm", err)
		return
	}

	response.GrpcResult(c, resp.Code, resp.Message, nil)
}

// ListMerchantTags lists all tags for a merchant
// @Summary      取得商家標籤列表
// @Description  取得商家所有的自訂標籤
// @Tags         商家CRM (Seller CRM)
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Success      200  {object}  object{code=int,message=string,data=object}
// @Failure      401  {object}  object{code=int,message=string}
// @Failure      500  {object}  object{code=int,message=string}
// @Failure      503  {object}  object{code=int,message=string}
// @Router       /app/sellers/me/tags [get]
func (h *Holder) ListMerchantTags(c *gin.Context) {
	storeID, ok := h.resolveStoreID(c)
	if !ok {
		return
	}

	resp, err := h.Crm.ListTags(c.Request.Context(), storeID)
	if err != nil {
		response.GrpcErr(c, "crm", err)
		return
	}

	response.GrpcResult(c, resp.Code, resp.Message, toSellerTagRespList(resp.Data))
}

// CreateSellerTag creates a new tag
// @Summary      建立商家標籤
// @Description  建立新的商家自訂標籤，name 為必填
// @Tags         商家CRM (Seller CRM)
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        request  body  object{name=string}  true  "建立標籤請求"
// @Success      200  {object}  object{code=int,message=string,data=object}
// @Failure      400  {object}  object{code=int,message=string}
// @Failure      401  {object}  object{code=int,message=string}
// @Failure      409  {object}  object{code=int,message=string}  "標籤名稱已存在"
// @Failure      500  {object}  object{code=int,message=string}
// @Failure      503  {object}  object{code=int,message=string}
// @Router       /app/sellers/me/tags [post]
func (h *Holder) CreateSellerTag(c *gin.Context) {
	storeID, ok := h.resolveStoreID(c)
	if !ok {
		return
	}

	var req struct {
		Name string `json:"name" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "invalid request body")
		return
	}

	resp, err := h.Crm.CreateTag(c.Request.Context(), storeID, req.Name)
	if err != nil {
		response.GrpcErr(c, "crm", err)
		return
	}

	response.GrpcResult(c, resp.Code, resp.Message, toSellerTagResp(resp.Data))
}

// UpdateSellerTag updates a tag
// @Summary      更新商家標籤
// @Description  根據標籤 ID 更新商家自訂標籤的名稱
// @Tags         商家CRM (Seller CRM)
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        tagId    path  string                                                true  "標籤 ID"
// @Param        request  body  object{name=string}   true  "更新標籤請求（name 為必填）"
// @Success      200  {object}  object{code=int,message=string}
// @Failure      400  {object}  object{code=int,message=string}
// @Failure      401  {object}  object{code=int,message=string}
// @Failure      404  {object}  object{code=int,message=string}  "標籤不存在"
// @Failure      500  {object}  object{code=int,message=string}
// @Failure      503  {object}  object{code=int,message=string}
// @Router       /app/sellers/me/tags/{tagId} [patch]
func (h *Holder) UpdateSellerTag(c *gin.Context) {
	storeID, ok := h.resolveStoreID(c)
	if !ok {
		return
	}

	tagID := c.Param("tagId")
	if tagID == "" {
		response.BadRequest(c, "invalid tag id")
		return
	}

	var req struct {
		Name string `json:"name" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "invalid request body")
		return
	}

	resp, err := h.Crm.UpdateTag(c.Request.Context(), storeID, tagID, req.Name)
	if err != nil {
		response.GrpcErr(c, "crm", err)
		return
	}

	response.GrpcResult(c, resp.Code, resp.Message, nil)
}

// DeleteMerchantTag deletes a tag
// @Summary      刪除商家標籤
// @Description  根據標籤 ID 刪除商家自訂標籤
// @Tags         商家CRM (Seller CRM)
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        tagId  path  string  true  "標籤 ID"
// @Success      200  {object}  object{code=int,message=string}
// @Failure      400  {object}  object{code=int,message=string}
// @Failure      401  {object}  object{code=int,message=string}
// @Failure      404  {object}  object{code=int,message=string}  "標籤不存在"
// @Failure      500  {object}  object{code=int,message=string}
// @Failure      503  {object}  object{code=int,message=string}
// @Router       /app/sellers/me/tags/{tagId} [delete]
func (h *Holder) DeleteMerchantTag(c *gin.Context) {
	storeID, ok := h.resolveStoreID(c)
	if !ok {
		return
	}

	tagID := c.Param("tagId")
	if tagID == "" {
		response.BadRequest(c, "invalid tag id")
		return
	}

	resp, err := h.Crm.DeleteTag(c.Request.Context(), storeID, tagID)
	if err != nil {
		response.GrpcErr(c, "crm", err)
		return
	}

	response.GrpcResult(c, resp.Code, resp.Message, nil)
}

// AddCustomerTags adds tags to a customer
// @Summary      為顧客新增標籤
// @Description  為指定顧客批次新增標籤
// @Tags         商家CRM (Seller CRM)
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        buyerId  path  int                        true  "客戶的 buyer_id（顧客列表回傳的 buyerId；非帳號 ID）"
// @Param        request  body  object{tagNames=[]string}  true  "標籤名稱列表（find-or-create）"
// @Success      200  {object}  object{code=int,message=string}
// @Failure      400  {object}  object{code=int,message=string}
// @Failure      401  {object}  object{code=int,message=string}
// @Failure      404  {object}  object{code=int,message=string}  "顧客不存在"
// @Failure      500  {object}  object{code=int,message=string}
// @Failure      503  {object}  object{code=int,message=string}
// @Router       /app/sellers/me/customers/{buyerId}/tags [post]
func (h *Holder) AddCustomerTags(c *gin.Context) {
	storeID, ok := h.resolveStoreID(c)
	if !ok {
		return
	}

	customerID, ok := h.resolveCustomerBuyerID(c)
	if !ok {
		return
	}

	var req struct {
		TagNames []string `json:"tagNames" binding:"required,min=1"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "invalid request body")
		return
	}

	resp, err := h.Crm.AddCustomerTags(c.Request.Context(), storeID, customerID, req.TagNames)
	if err != nil {
		response.GrpcErr(c, "crm", err)
		return
	}

	response.GrpcResult(c, resp.Code, resp.Message, nil)
}

// RemoveCustomerTag removes a tag from a customer
// @Summary      移除顧客標籤
// @Description  移除指定顧客的某個標籤
// @Tags         商家CRM (Seller CRM)
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        buyerId  path  int  true  "客戶的 buyer_id（顧客列表回傳的 buyerId；非帳號 ID）"
// @Param        tagId    path  int  true  "標籤 ID"
// @Success      200  {object}  object{code=int,message=string}
// @Failure      400  {object}  object{code=int,message=string}
// @Failure      401  {object}  object{code=int,message=string}
// @Failure      404  {object}  object{code=int,message=string}  "顧客或標籤不存在"
// @Failure      500  {object}  object{code=int,message=string}
// @Failure      503  {object}  object{code=int,message=string}
// @Router       /app/sellers/me/customers/{buyerId}/tags/{tagId} [delete]
func (h *Holder) RemoveCustomerTag(c *gin.Context) {
	storeID, ok := h.resolveStoreID(c)
	if !ok {
		return
	}

	customerID, ok := h.resolveCustomerBuyerID(c)
	if !ok {
		return
	}

	tagID, err := strconv.ParseInt(c.Param("tagId"), 10, 64)
	if err != nil {
		response.BadRequest(c, "invalid tag id")
		return
	}

	resp, err := h.Crm.RemoveCustomerTag(c.Request.Context(), storeID, customerID, tagID)
	if err != nil {
		response.GrpcErr(c, "crm", err)
		return
	}

	response.GrpcResult(c, resp.Code, resp.Message, nil)
}

// GetCustomerTags returns tags for a specific customer
// @Summary      取得顧客的標籤列表
// @Description  取得指定顧客目前擁有的所有標籤
// @Tags         商家CRM (Seller CRM)
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        buyerId  path  int  true  "客戶的 buyer_id（顧客列表回傳的 buyerId；非帳號 ID）"
// @Success      200  {object}  object{code=int,message=string,data=object}
// @Failure      400  {object}  object{code=int,message=string}
// @Failure      401  {object}  object{code=int,message=string}
// @Failure      404  {object}  object{code=int,message=string}  "顧客不存在"
// @Failure      500  {object}  object{code=int,message=string}
// @Failure      503  {object}  object{code=int,message=string}
// @Router       /app/sellers/me/customers/{buyerId}/tags [get]
func (h *Holder) GetCustomerTags(c *gin.Context) {
	storeID, ok := h.resolveStoreID(c)
	if !ok {
		return
	}

	customerID, ok := h.resolveCustomerBuyerID(c)
	if !ok {
		return
	}

	resp, err := h.Crm.GetCustomerTags(c.Request.Context(), storeID, customerID)
	if err != nil {
		response.GrpcErr(c, "crm", err)
		return
	}

	response.GrpcResult(c, resp.Code, resp.Message, toSellerTagRespList(resp.Data))
}

// GetCustomerOrders returns order history for a specific customer
// @Summary      取得顧客消費紀錄
// @Description  取得指定買家在此賣家的訂單列表（訂單編號、時間、金額）
// @Tags         商家CRM (Seller CRM)
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        buyerId    path   int     true   "客戶的 buyer_id（顧客列表回傳的 buyerId；非帳號 ID）"
// @Param        page       query  int     false  "頁碼（預設為 1）"
// @Param        page_size  query  int     false  "每頁筆數（預設為 20）"
// @Success      200  {object}  object{code=int,message=string,data=object{records=[]object{orderNo=string,createTime=string,actualAmount=string,discountAmount=string,refundAmount=string},total=string,page=string,pageSize=string}}
// @Failure      400  {object}  object{code=int,message=string}
// @Failure      401  {object}  object{code=int,message=string}
// @Failure      500  {object}  object{code=int,message=string}
// @Failure      503  {object}  object{code=int,message=string}
// @Router       /app/sellers/me/customers/{buyerId}/orders [get]
func (h *Holder) GetCustomerOrders(c *gin.Context) {
	storeID, ok := h.resolveStoreID(c)
	if !ok {
		return
	}

	// ② 第三方目標買家（賣家指定的 buyerId path 參數）就是 buyer_id 本身，經
	// 共用 resolveCustomerBuyerID 取出後原值送 order-service——訂單持久化的正
	// 是 buyer_id，翻成現任 stint 反而會讓輪替前的訂單歷史整段消失。
	buyerID, ok := h.resolveCustomerBuyerID(c)
	if !ok {
		return
	}

	resp, err := h.Order.GetBuyerOrdersForSeller(c.Request.Context(), &orderpb.GetBuyerOrdersForSellerRequest{
		BuyerId: int64(buyerID),
		StoreId: int64(storeID),
		Page:    c.DefaultQuery("page", defaultPageStr),
		Size:    c.DefaultQuery("page_size", defaultPageSizeStr),
	})
	if err != nil {
		response.GrpcErr(c, "order", err)
		return
	}

	if resp.Code != 0 {
		response.GrpcResult(c, resp.Code, resp.Message, nil)
		return
	}

	records := make([]gin.H, 0, len(resp.Records))
	for _, r := range resp.Records {
		items := make([]gin.H, 0, len(r.Items))
		for _, item := range r.Items {
			items = append(items, gin.H{
				"id":              item.Id,
				"productId":       item.ProductId,
				"productName":     item.ProductName,
				"skuId":           item.SkuId,
				"skuName":         item.SkuName,
				"quantity":        item.Quantity,
				"unitPrice":       item.UnitPrice,
				"listPrice":       item.ListPrice,
				"actualPayAmount": item.ActualPayAmount,
				"refundStatus":    item.RefundStatus,
				"shareDiscount":   item.ShareDiscount,
				"imageKey":        item.ImageKey,
				"imageUrl":        buildURL(h.MediaBaseURL, item.ImageKey),
			})
		}
		records = append(records, gin.H{
			"orderNo":        r.OrderNo,
			"createTime":     r.CreateTime,
			"actualAmount":   r.ActualAmount,
			"discountAmount": r.DiscountAmount,
			// 這條清單自 2026-08-05 起也含 PARTIALLY_REFUNDED(60) 的訂單，所以
			// actualAmount 單獨看會高估賣家實收——refundAmount 必須跟著一起給，
			// 否則賣家看到的是「原價成交」這種假象。淨額 = actualAmount - refundAmount。
			"refundAmount":    r.RefundAmount,
			"items":           items,
			"shippingAddress": r.ShippingAddress,
			"recipientPhone":  r.RecipientPhone,
			"shippingFee":     r.ShippingFee,
		})
	}
	response.Page(c, records, resp.Total, resp.Page, resp.PageSize)
}

// GetCustomerCoupons returns coupons held by a specific customer
// @Summary      取得顧客優惠券列表
// @Description  取得指定買家持有的此賣家優惠券
// @Tags         商家CRM (Seller CRM)
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        buyerId    path   int     true   "客戶的 buyer_id（顧客列表回傳的 buyerId；非帳號 ID）"
// @Param        page       query  int     false  "頁碼（預設為 1）"
// @Param        page_size  query  int     false  "每頁筆數（預設為 20）"
// @Param        status     query  string  false  "篩選狀態（空字串 = 全部）"
// @Success      200  {object}  object{code=int,message=string,data=object{records=[]object{instanceId=string,templateId=string,title=string,type=string,applicableStoreId=string,issuerStoreId=string,sellerName=string,storeCode=string,thresholdAmount=int,discountAmount=int,discountPercent=int,maxDiscountAmount=int,startAt=string,endAt=string,status=string,createdAt=string},total=string,page=string,pageSize=string}}
// @Failure      401  {object}  object{code=int,message=string}
// @Failure      500  {object}  object{code=int,message=string}
// @Failure      503  {object}  object{code=int,message=string}
// @Router       /app/sellers/me/customers/{buyerId}/coupons [get]
func (h *Holder) GetCustomerCoupons(c *gin.Context) {
	storeID, ok := h.resolveStoreID(c)
	if !ok {
		return
	}

	// ② 第三方目標買家（賣家指定的 buyerId path 參數）就是 buyer_id，經共用
	// resolveCustomerBuyerID 取出後原值送 coupon-service，跟本檔其餘 7 個
	// /customers/{buyerId}/... 手足一致。coupon_instances.owner_buyer_id 是
	// ClaimCoupon 當下由 gateway 送出的 buyer_id 寫入，coupon-service 端只有
	// 直接 eq 比對、無二次解析——所以這裡送的必須是 crm 列上掛的那個
	// buyer_id。翻成現任 stint 會讓任何買家角色曾 delete → re-apply 的顧客
	// 查不到自己已持有的優惠券（#1519 同類，只是換成券軸）。
	buyerID, ok := h.resolveCustomerBuyerID(c)
	if !ok {
		return
	}

	resp, err := h.Coupon.ListBuyerCouponsForSeller(c.Request.Context(), &couponpb.ListBuyerCouponsForSellerRequest{
		StoreId:  int64(storeID),
		BuyerId:  strconv.FormatInt(int64(buyerID), 10),
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

// SearchCustomersByTags searches customers by multiple tags (AND logic)
// @Summary      依標籤搜尋目標客戶
// @Description  依多個標籤篩選客戶（AND 邏輯，需同時擁有所有選取的標籤）
// @Tags         商家CRM (Seller CRM)
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        request  body  SearchCustomersByTagsRequest  true  "搜尋請求"
// @Success      200  {object}  SearchCustomersByTagsResponse
// @Failure      400  {object}  object{code=int,message=string}
// @Failure      401  {object}  object{code=int,message=string}
// @Failure      500  {object}  object{code=int,message=string}
// @Failure      503  {object}  object{code=int,message=string}
// @Router       /app/sellers/me/customers/search-by-tags [post]
func (h *Holder) SearchCustomersByTags(c *gin.Context) {
	storeID, ok := h.resolveStoreID(c)
	if !ok {
		return
	}

	var req SearchCustomersByTagsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "invalid request body")
		return
	}

	if len(req.TagIDs) == 0 {
		response.BadRequest(c, "tag_ids is required")
		return
	}

	resp, err := h.Crm.SearchCustomersByTags(c.Request.Context(), storeID, req.TagIDs)
	if err != nil {
		response.GrpcErr(c, "crm", err)
		return
	}

	if resp.Code != 0 {
		response.GrpcResult(c, resp.Code, resp.Message, nil)
		return
	}
	customers := make([]gin.H, 0, len(resp.Data.Customers))
	for _, cu := range resp.Data.Customers {
		customers = append(customers, gin.H{
			"buyerId":   cu.BuyerId,
			"nickname":  cu.Nickname,
			"phone":     cu.Phone,
			"avatarKey": cu.AvatarKey,                           // 買家頭像 objectKey（SSOT；49 §6.1）
			"avatarUrl": buildURL(h.MediaBaseURL, cu.AvatarKey), // 顯示 URL（Gateway 組；http 外部頭像 passthrough）
		})
	}

	response.OK(c, gin.H{
		"total":     resp.Data.Total,
		"customers": customers,
	})
}

// SearchCustomersByTagsRequest 搜尋目標客戶請求
type SearchCustomersByTagsRequest struct {
	TagIDs []string `json:"tagIds" binding:"required,min=1"` // 標籤 ID 列表（AND 邏輯）
}

// SearchCustomersByTagsResponse 搜尋目標客戶回應
type SearchCustomersByTagsResponse struct {
	Message string                        `json:"message"`
	TraceID string                        `json:"traceId,omitempty"`
	Data    SearchCustomersByTagsDataResp `json:"data"`
	Code    int                           `json:"code"`
}

// SearchCustomersByTagsDataResp 搜尋結果
type SearchCustomersByTagsDataResp struct {
	Total     string                `json:"total"` // 整數字串
	Customers []MatchedCustomerResp `json:"customers"`
}

// MatchedCustomerResp 匹配的客戶
type MatchedCustomerResp struct {
	BuyerID   string `json:"buyerId"`   // 買家 buyer_id（buyers.buyer_id；不是 account_id）
	Nickname  string `json:"nickname"`  // 暱稱
	Phone     string `json:"phone"`     // 電話
	AvatarKey string `json:"avatarKey"` // 買家頭像 objectKey（SSOT；49 §6.1）
	AvatarURL string `json:"avatarUrl"` // 買家頭像顯示 URL（Gateway 組）
}
