package handler

import (
	"encoding/json"

	"github.com/Will413028/hawkr/gateway/internal/response"
	pb "github.com/Will413028/hawkr/go-pkg/proto/product"
	"github.com/gin-gonic/gin"
)

// ADM 端的商品認證審核（19 號設計書 §9.2 / §9.3）。
//
// 🔴 路徑用 /admin/product_certification_applications 而不是 /admin/certifications：
// 後者已經被商家實名認證（KYC，`docs/modules/seller.md` §13）佔用，兩者是完全不同的東西。
//
// 🔴 掛在 admin group（JWTAuth + requireAdmin），不是 adminNoAuth——那個 sibling group
// 是代行營運專用，掛錯會在 boot 斷言直接 panic。
//
// 🔴 reviewer 身分一律取 auth context 的 account_id，**不接受 request body 傳入**：
// 由請求體決定「誰批准的」等於讓稽核紀錄可被偽造。也不得抄 KYC 那個
// rid=1 / rname="Admin" 的 fallback（既有瑕疵），那會讓所有審核都記在同一個假身分上。

// AdminProductCertificationSummary 是同類型現行有效那筆的精簡投影（currentApproved）。
//
// 首次申請時整個物件是 null（proto3 message 未設）——ADM 據此知道「核准這一筆不會換掉
// 任何既有標章」。它不是裝飾：核准會讓 currentApproved 那筆被 SUPERSEDE，審核者必須先
// 看得到哪一張標章會被換掉（19 §9.2）。
type AdminProductCertificationSummary struct {
	ID                string  `json:"id" example:"843856384063471760"`
	CertificationType string  `json:"certificationType" example:"3701"`
	CertificateNo     string  `json:"certificateNo" example:"1-009-300022"`
	Status            string  `json:"status" example:"APPROVED"` // 恆為 APPROVED
	GovVerifyState    string  `json:"govVerifyState" example:"VERIFIED"`
	ReviewedAt        *string `json:"reviewedAt" example:"2026-08-26T04:12:07Z"`
}

func toAdminProductCertificationSummary(s *pb.ProductCertificationSummary) *AdminProductCertificationSummary {
	if s == nil {
		return nil
	}
	return &AdminProductCertificationSummary{
		ID:                s.GetId(),
		CertificationType: s.GetCertificationType(),
		CertificateNo:     s.GetCertificateNo(),
		Status:            s.GetStatus(),
		GovVerifyState:    s.GetGovVerifyState(),
		ReviewedAt:        nilIfEmptyStr(s.GetReviewedAt()),
	}
}

// AdminProductGovVerifyResult 是政府查驗結果的結構化投影（Admin-only，19 §9.2）。
//
// matchedCount 對外是字串：數字一律 string 是全專案契約（proto 那欄本身就宣告成 string），
// 前端照數字接會在 TSID 等級的值上踩到精度問題，也讓型別在三端不一致。
type AdminProductGovVerifyResult struct {
	CertType       string  `json:"certType" example:"3703"`
	CertNo         string  `json:"certNo" example:"010105"`
	QueriedAt      *string `json:"queriedAt" example:"2026-08-26T04:12:09Z"`
	Found          bool    `json:"found" example:"true"`
	Validity       string  `json:"validity" example:"VALID"` // VALID / EXPIRED / TERMINATED / SUSPENDED / NOT_FOUND / UNKNOWN
	StatusRaw      *string `json:"statusRaw"`                // 只有 3701 有
	ExpiresOn      *string `json:"expiresOn"`                // 3703 沒有
	HolderName     *string `json:"holderName"`
	HolderAddress  *string `json:"holderAddress"`
	IssuerName     *string `json:"issuerName"`
	ProductSummary *string `json:"productSummary"`
	MatchedCount   string  `json:"matchedCount" example:"173"` // 數字字串
	// rawExcerpt 是已修剪的政府原始回應（19 §8.4 防線 3 截斷後的殘餘）。
	// proto 載體是 JSON 字串，這裡還原成物件——讓 ADM 直接展開看，而不是逼前端再 parse
	// 一次字串。無法解析或未提供時是 null。
	RawExcerpt json.RawMessage `json:"rawExcerpt"`
}

func toAdminProductGovVerifyResult(r *pb.GovVerifyResult) *AdminProductGovVerifyResult {
	if r == nil {
		return nil
	}
	out := &AdminProductGovVerifyResult{
		CertType:       r.GetCertType(),
		CertNo:         r.GetCertNo(),
		QueriedAt:      nilIfEmptyStr(r.GetQueriedAt()),
		Found:          r.GetFound(),
		Validity:       r.GetValidity(),
		StatusRaw:      nilIfEmptyStr(r.GetStatusRaw()),
		ExpiresOn:      nilIfEmptyStr(r.GetExpiresOn()),
		HolderName:     nilIfEmptyStr(r.GetHolderName()),
		HolderAddress:  nilIfEmptyStr(r.GetHolderAddress()),
		IssuerName:     nilIfEmptyStr(r.GetIssuerName()),
		ProductSummary: nilIfEmptyStr(r.GetProductSummary()),
		MatchedCount:   r.GetMatchedCount(),
	}
	// json.Valid 這一關不能省：raw_excerpt_json 是上游組出來的字串，直接當
	// json.RawMessage 塞進回應會讓整個 envelope 在 Marshal 時失敗（HTTP 500、
	// 連 code 都送不出去），而壞掉的只是一個輔助欄位。
	if raw := r.GetRawExcerptJson(); raw != "" && json.Valid([]byte(raw)) {
		out.RawExcerpt = json.RawMessage(raw)
	}
	return out
}

// AdminProductCertificationApplicationItem 是 ADM 看到的一筆認證申請（列表與詳情共用）。
//
// 比賣家版多三塊：govVerifyPayload（政府原始資料）、internalNote（內部註記）、
// currentApproved（同類型現行有效那筆）。
type AdminProductCertificationApplicationItem struct {
	ID          string `json:"id" example:"843856384063471761"`
	ProductID   string `json:"productId" example:"843856384063471755"`
	ProductName string `json:"productName" example:"有機小松菜"`
	StoreID     string `json:"storeId" example:"2685914682"`
	// submittedByAccountId 是送件者的登入 principal（accounts PK），不是營運鍵。
	// 賣家換手（store-stint）後 storeId 不變而這個值會變，稽核要靠它才查得回
	// 「當時是誰送的」。已登記於 scripts/gateway-json-contract-baseline.json。
	SubmittedByAccountID *string `json:"submittedByAccountId" example:"2685914682"`
	CertificationType    string  `json:"certificationType" example:"3703"`
	CertificateNo        string  `json:"certificateNo" example:"010105"`
	Status               string  `json:"status" example:"SUBMITTED"`
	Source               string  `json:"source" example:"SELLER"`
	RequiresManualReview bool    `json:"requiresManualReview" example:"false"`
	GovVerifyState       string  `json:"govVerifyState" example:"VERIFIED"`
	GovVerifiedAt        *string `json:"govVerifiedAt" example:"2026-08-26T04:12:09Z"`
	SubmittedAt          *string `json:"submittedAt" example:"2026-08-26T04:12:05Z"`
	ReviewedAt           *string `json:"reviewedAt"`
	// reviewedByAccountId 同上，是裁決者的登入 principal；自動核准時為 null
	// （actor 由審核事件的 SYSTEM 標示）。
	ReviewedByAccountID *string                           `json:"reviewedByAccountId"`
	ReasonCode          *string                           `json:"reasonCode"`
	PublicReason        *string                           `json:"publicReason"`
	GovVerifyPayload    *AdminProductGovVerifyResult      `json:"govVerifyPayload"`
	CurrentApproved     *AdminProductCertificationSummary `json:"currentApproved"`
	InternalNote        *string                           `json:"internalNote"`
	Version             string                            `json:"version" example:"2"`
	// productPublicationState 是商品的公開生命週期（ProductPublicationState value）。
	//
	// 🔴 它不是裝飾：認證申請**結構上會指向已軟刪的商品**（V18 legacy migration 的 INSERT
	// 不濾 deleted_at、賣家送件後可以刪掉自己的商品），而 ADM 佇列列得出這些申請。只把
	// productName 補回來會讓審核者看到一個「看起來還在」的商品並可能核准它——#2818 之前
	// 那個空白商品名其實是唯一線索，補回名字之後必須有東西接手把「已移除」講明白。
	//
	// 商品列被實體刪除時為空字串（查不到 ⇒ 上游不設），ADM 至少還有 productId。
	ProductPublicationState string `json:"productPublicationState" example:"4014"`
}

func toAdminCertificationApplication(app *pb.ProductCertificationApplication) AdminProductCertificationApplicationItem {
	if app == nil {
		return AdminProductCertificationApplicationItem{}
	}
	return AdminProductCertificationApplicationItem{
		ID:                      app.GetId(),
		ProductID:               app.GetProductId(),
		ProductName:             app.GetProductName(),
		StoreID:                 app.GetStoreId(),
		SubmittedByAccountID:    nilIfEmptyStr(app.GetSubmittedByAccountId()),
		CertificationType:       app.GetCertificationType(),
		CertificateNo:           app.GetCertificateNo(),
		Status:                  app.GetStatus(),
		Source:                  app.GetSource(),
		RequiresManualReview:    app.GetRequiresManualReview(),
		GovVerifyState:          app.GetGovVerifyState(),
		GovVerifiedAt:           nilIfEmptyStr(app.GetGovVerifiedAt()),
		SubmittedAt:             nilIfEmptyStr(app.GetSubmittedAt()),
		ReviewedAt:              nilIfEmptyStr(app.GetReviewedAt()),
		ReviewedByAccountID:     nilIfEmptyStr(app.GetReviewedByAccountId()),
		ReasonCode:              nilIfEmptyStr(app.GetReasonCode()),
		PublicReason:            nilIfEmptyStr(app.GetPublicReason()),
		GovVerifyPayload:        toAdminProductGovVerifyResult(app.GetGovVerifyPayload()),
		CurrentApproved:         toAdminProductCertificationSummary(app.GetCurrentApproved()),
		InternalNote:            nilIfEmptyStr(app.GetInternalNote()),
		Version:                 app.GetVersion(),
		ProductPublicationState: app.GetProductPublicationState(),
	}
}

func toAdminCertificationApplications(apps []*pb.ProductCertificationApplication) []AdminProductCertificationApplicationItem {
	out := make([]AdminProductCertificationApplicationItem, 0, len(apps))
	for _, app := range apps {
		if app != nil {
			out = append(out, toAdminCertificationApplication(app))
		}
	}
	return out
}

// AdminProductCertificationAuditLogItem 是一筆審核事件（append-only 流水，19 §4.2）。
type AdminProductCertificationAuditLogItem struct {
	ID            string `json:"id"`
	ApplicationID string `json:"applicationId"`
	Action        string `json:"action"`     // ProductCertificationReviewAction
	FromStatus    string `json:"fromStatus"` // 建立事件為空字串
	ToStatus      string `json:"toStatus"`
	ActorRole     string `json:"actorRole"` // SYSTEM / SELLER / ADMIN
	// actorAccountId 是動作發起者的登入 principal；SYSTEM 動作為 null。
	// 已登記於 scripts/gateway-json-contract-baseline.json（理由同 submittedByAccountId）。
	ActorAccountID     *string `json:"actorAccountId"`
	CertificationType  string  `json:"certificationType"` // 事件當下的快照
	CertificateNo      string  `json:"certificateNo"`     // 同上
	ReasonCode         *string `json:"reasonCode"`
	PublicReason       *string `json:"publicReason"`
	InternalNote       *string `json:"internalNote"` // Admin-only
	ApplicationVersion string  `json:"applicationVersion"`
	TraceID            *string `json:"traceId"`
	CreatedAt          *string `json:"createdAt"`
}

func toAdminCertificationAuditLogs(events []*pb.ProductCertificationReviewEvent) []AdminProductCertificationAuditLogItem {
	out := make([]AdminProductCertificationAuditLogItem, 0, len(events))
	for _, e := range events {
		if e == nil {
			continue
		}
		out = append(out, AdminProductCertificationAuditLogItem{
			ID:                 e.GetId(),
			ApplicationID:      e.GetApplicationId(),
			Action:             e.GetAction(),
			FromStatus:         e.GetFromStatus(),
			ToStatus:           e.GetToStatus(),
			ActorRole:          e.GetActorRole(),
			ActorAccountID:     nilIfEmptyStr(e.GetActorAccountId()),
			CertificationType:  e.GetCertificationType(),
			CertificateNo:      e.GetCertificateNo(),
			ReasonCode:         nilIfEmptyStr(e.GetReasonCode()),
			PublicReason:       nilIfEmptyStr(e.GetPublicReason()),
			InternalNote:       nilIfEmptyStr(e.GetInternalNote()),
			ApplicationVersion: e.GetApplicationVersion(),
			TraceID:            nilIfEmptyStr(e.GetTraceId()),
			CreatedAt:          nilIfEmptyStr(e.GetCreatedAt()),
		})
	}
	return out
}

// ReviewProductCertificationRequest 是 approve / reject / revoke 共用的請求體。
//
// 動作由**路由**區分，不由 body 的 flag 分流（角色與動作分離，10 檔 §5.1）。
// reasonCode 對 reject / revoke 是必填，但**不在這裡用 binding 擋**：那是業務規則，
// 由 product-service 的收斂點以 3028 回答，Gateway 自己判會變成第二份規則。
type ReviewProductCertificationRequest struct {
	IdempotencyKey  string `json:"idempotencyKey" binding:"required" example:"cert-approve-2f0c1f2e"`
	ExpectedVersion string `json:"expectedVersion" binding:"required" example:"2"` // 樂觀鎖，數字字串
	ReasonCode      string `json:"reasonCode" example:"CERT_NOT_FOUND"`            // reject / revoke 必填
	PublicReason    string `json:"publicReason" example:"政府資料庫查無此編號，請確認後重新提交"`
	InternalNote    string `json:"internalNote" example:"已致電賣家確認"` // Admin-only，不會出現在賣家 DTO
}

// ReverifyProductCertificationRequest 是重新查驗的請求體。
//
// 沒有 expectedVersion：這一支不改 status，只覆寫 gov_verify_*（19 §9.3）。
// idempotencyKey 仍必填——它擋的是「連點兩次打兩次政府 API」，那是有配額的外部資源。
type ReverifyProductCertificationRequest struct {
	IdempotencyKey string `json:"idempotencyKey" binding:"required" example:"cert-reverify-2f0c1f2e"`
}

// adminCertificationApplicationID 取路由上的申請 ID。
func adminCertificationApplicationID(c *gin.Context) (string, bool) {
	id := c.Param("id")
	if id == "" {
		response.BadRequest(c, "invalid application id")
		return "", false
	}
	return id, true
}

// respondAdminCertificationApplication 是四支寫入端點共用的回應收斂點。
func respondAdminCertificationApplication(c *gin.Context, resp *pb.ProductCertificationApplicationAdminResponse, err error) {
	if err != nil {
		response.GrpcErr(c, "product", err)
		return
	}
	if resp.GetCode() != 0 {
		// BizStatus 而不是 GrpcResult：3025→404 / 3026→409 / 3027→409 / 3028→400 的
		// 分流靠 response 的登錄表，走 GrpcResult 會全部變成 HTTP 200，ADM 只能靠讀
		// body 才知道失敗了。未登錄的碼仍塌縮成 409。
		//
		// ⚠️ 這條分支在 production **幾乎不會被執行**：EnvelopeGuardInterceptor 會把
		// 「gRPC OK ＋ 非零 envelope code」轉成 gRPC error，所以業務碼是從上面那個
		// err 那側回來的（同一張登錄表由 response.GrpcErr 查，#2755 修的就是它漏查）。
		// 只有 envelopeCodePreserveData 的三個碼走得到這裡。保留它是為了不依賴
		// interceptor 的內部名單，但**不要**把它當成這組端點的 status 保證來源——
		// 用 mock proto client（無 *grpc.ClientConn）寫的測試只驗得到這條死路。
		response.BizStatus(c, resp.GetCode(), resp.GetMessage())
		return
	}
	response.OK(c, toAdminCertificationApplication(resp.GetData()))
}

// reviewAdminCertification 是 approve / reject / revoke 三支的共用軀幹：
// 差別只有要打哪一支 RPC，其餘（身分、參數、回應形狀）完全相同。
func (h *Holder) reviewAdminCertification(
	c *gin.Context,
	call func(*gin.Context, *pb.ReviewProductCertificationForAdminRequest) (*pb.ProductCertificationApplicationAdminResponse, error),
) {
	accountID, ok := getAccountID(c)
	if !ok {
		response.Unauthorized(c)
		return
	}
	applicationID, ok := adminCertificationApplicationID(c)
	if !ok {
		return
	}

	var req ReviewProductCertificationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "invalid request: "+err.Error())
		return
	}

	resp, err := call(c, &pb.ReviewProductCertificationForAdminRequest{
		ApplicationId:   applicationID,
		IdempotencyKey:  req.IdempotencyKey,
		ExpectedVersion: req.ExpectedVersion,
		ReasonCode:      req.ReasonCode,
		PublicReason:    req.PublicReason,
		InternalNote:    req.InternalNote,
		// 🔴 reviewer 一律取 auth context，不讀 body 的任何欄位。
		AccountId: accountID,
	})
	respondAdminCertificationApplication(c, resp, err)
}

// ListAdminProductCertificationApplications 認證審核佇列
// @Summary      認證申請審核佇列
// @Description  ADM 依狀態／類型／店家／關鍵字／送件時間區間查詢商品認證申請。keyword 比對商品名或證號；submitted_from / submitted_to 為 ISO 8601 UTC。分頁三欄 total/page/pageSize 皆為字串。
// @Tags         商品認證審核 (Product Certification)
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        status              query  string  false  "申請狀態（ProductCertificationStatus）"
// @Param        certification_type  query  string  false  "認證類型 CertificationType value" Enums(3701, 3702, 3703, 3704, 3705, 3706)
// @Param        store_id            query  string  false  "店家 store_id（數字字串）"
// @Param        keyword             query  string  false  "關鍵字（商品名或證號）"
// @Param        submitted_from      query  string  false  "送件時間下界（ISO 8601 UTC，如 2026-08-01T00:00:00Z）"
// @Param        submitted_to        query  string  false  "送件時間上界（ISO 8601 UTC）"
// @Param        page                query  string  false  "頁碼（數字字串，預設 1）"
// @Param        page_size           query  string  false  "每頁筆數（數字字串，預設 20）"
// @Success      200  {object}  object{code=int,message=string,data=object{records=[]AdminProductCertificationApplicationItem,total=string,page=string,pageSize=string}}
// @Failure      401  {object}  object{code=int,message=string,traceId=string}
// @Failure      403  {object}  object{code=int,message=string,traceId=string}
// @Router       /admin/product_certification_applications [get]
func (h *Holder) ListAdminProductCertificationApplications(c *gin.Context) {
	accountID, ok := getAccountID(c)
	if !ok {
		response.Unauthorized(c)
		return
	}

	resp, err := h.Product.ListProductCertificationApplicationsForAdmin(c.Request.Context(), &pb.ListProductCertificationApplicationsForAdminRequest{
		Status:            c.Query("status"),
		CertificationType: c.Query("certification_type"),
		StoreId:           c.Query("store_id"),
		Keyword:           c.Query("keyword"),
		SubmittedFrom:     c.Query("submitted_from"),
		SubmittedTo:       c.Query("submitted_to"),
		Page:              c.DefaultQuery("page", "1"),
		PageSize:          c.DefaultQuery("page_size", "20"),
		AccountId:         accountID,
	})
	if err != nil {
		response.GrpcErr(c, "product", err)
		return
	}
	if resp.GetCode() != 0 {
		response.BizStatus(c, resp.GetCode(), resp.GetMessage())
		return
	}

	response.Page(c,
		toAdminCertificationApplications(resp.GetRecords()),
		resp.GetTotal(), resp.GetPage(), resp.GetPageSize())
}

// GetAdminProductCertificationApplication 認證申請詳情
// @Summary      認證申請詳情
// @Description  ADM 查單筆認證申請的完整內容，比賣家版多 govVerifyPayload（政府原始查驗結果）、internalNote（內部註記）與 currentApproved（同類型現行有效那筆；首次申請為 null）。
// @Tags         商品認證審核 (Product Certification)
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id  path  string  true  "認證申請 ID（TSID 數字字串）"
// @Success      200  {object}  object{code=int,message=string,data=AdminProductCertificationApplicationItem}
// @Failure      401  {object}  object{code=int,message=string,traceId=string}
// @Failure      403  {object}  object{code=int,message=string,traceId=string}
// @Failure      404  {object}  object{code=int,message=string,traceId=string}
// @Router       /admin/product_certification_applications/{id} [get]
func (h *Holder) GetAdminProductCertificationApplication(c *gin.Context) {
	accountID, ok := getAccountID(c)
	if !ok {
		response.Unauthorized(c)
		return
	}
	applicationID, ok := adminCertificationApplicationID(c)
	if !ok {
		return
	}

	resp, err := h.Product.GetProductCertificationApplicationForAdmin(c.Request.Context(), &pb.GetProductCertificationApplicationForAdminRequest{
		ApplicationId: applicationID,
		AccountId:     accountID,
	})
	respondAdminCertificationApplication(c, resp, err)
}

// ListAdminProductCertificationAuditLogs 認證審核事件流水
// @Summary      認證審核事件流水
// @Description  ADM 查一筆認證申請的 append-only 審核事件（送出／自動核准／人工核准／駁回／撤銷／撤回／被取代），含 actor 與當下版本。分頁三欄皆為字串。
// @Tags         商品認證審核 (Product Certification)
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id         path   string  true   "認證申請 ID（TSID 數字字串）"
// @Param        page       query  string  false  "頁碼（數字字串，預設 1）"
// @Param        page_size  query  string  false  "每頁筆數（數字字串，預設 20）"
// @Success      200  {object}  object{code=int,message=string,data=object{records=[]AdminProductCertificationAuditLogItem,total=string,page=string,pageSize=string}}
// @Failure      401  {object}  object{code=int,message=string,traceId=string}
// @Failure      403  {object}  object{code=int,message=string,traceId=string}
// @Failure      404  {object}  object{code=int,message=string,traceId=string}
// @Router       /admin/product_certification_applications/{id}/audit_logs [get]
func (h *Holder) ListAdminProductCertificationAuditLogs(c *gin.Context) {
	accountID, ok := getAccountID(c)
	if !ok {
		response.Unauthorized(c)
		return
	}
	applicationID, ok := adminCertificationApplicationID(c)
	if !ok {
		return
	}

	resp, err := h.Product.ListProductCertificationAuditLogsForAdmin(c.Request.Context(), &pb.ListProductCertificationAuditLogsForAdminRequest{
		ApplicationId: applicationID,
		Page:          c.DefaultQuery("page", "1"),
		PageSize:      c.DefaultQuery("page_size", "20"),
		AccountId:     accountID,
	})
	if err != nil {
		response.GrpcErr(c, "product", err)
		return
	}
	if resp.GetCode() != 0 {
		response.BizStatus(c, resp.GetCode(), resp.GetMessage())
		return
	}

	response.Page(c,
		toAdminCertificationAuditLogs(resp.GetRecords()),
		resp.GetTotal(), resp.GetPage(), resp.GetPageSize())
}

// ApproveAdminProductCertification 核准認證申請
// @Summary      核准認證申請
// @Description  ADM 人工核准一筆認證申請。核准會讓同類型現行有效那筆（回應的 currentApproved）被標記為 SUPERSEDED 並重建商品的認證投影。reasonCode / publicReason 可省略。
// @Tags         商品認證審核 (Product Certification)
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id       path  string                             true  "認證申請 ID（TSID 數字字串）"
// @Param        request  body  ReviewProductCertificationRequest  true  "審核請求"
// @Success      200  {object}  object{code=int,message=string,data=AdminProductCertificationApplicationItem}
// @Failure      400  {object}  object{code=int,message=string,traceId=string}
// @Failure      401  {object}  object{code=int,message=string,traceId=string}
// @Failure      403  {object}  object{code=int,message=string,traceId=string}
// @Failure      404  {object}  object{code=int,message=string,traceId=string}
// @Failure      409  {object}  object{code=int,message=string,traceId=string}
// @Router       /admin/product_certification_applications/{id}/approve [post]
func (h *Holder) ApproveAdminProductCertification(c *gin.Context) {
	h.reviewAdminCertification(c, func(c *gin.Context, req *pb.ReviewProductCertificationForAdminRequest) (*pb.ProductCertificationApplicationAdminResponse, error) {
		return h.Product.ApproveProductCertificationForAdmin(c.Request.Context(), req)
	})
}

// RejectAdminProductCertification 駁回認證申請
// @Summary      駁回認證申請
// @Description  ADM 駁回一筆認證申請。reasonCode 必填（缺少回業務碼 3028）；publicReason 會展示給賣家，internalNote 不會。
// @Tags         商品認證審核 (Product Certification)
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id       path  string                             true  "認證申請 ID（TSID 數字字串）"
// @Param        request  body  ReviewProductCertificationRequest  true  "審核請求（reasonCode 必填）"
// @Success      200  {object}  object{code=int,message=string,data=AdminProductCertificationApplicationItem}
// @Failure      400  {object}  object{code=int,message=string,traceId=string}
// @Failure      401  {object}  object{code=int,message=string,traceId=string}
// @Failure      403  {object}  object{code=int,message=string,traceId=string}
// @Failure      404  {object}  object{code=int,message=string,traceId=string}
// @Failure      409  {object}  object{code=int,message=string,traceId=string}
// @Router       /admin/product_certification_applications/{id}/reject [post]
func (h *Holder) RejectAdminProductCertification(c *gin.Context) {
	h.reviewAdminCertification(c, func(c *gin.Context, req *pb.ReviewProductCertificationForAdminRequest) (*pb.ProductCertificationApplicationAdminResponse, error) {
		return h.Product.RejectProductCertificationForAdmin(c.Request.Context(), req)
	})
}

// RevokeAdminProductCertification 撤銷已核准的認證
// @Summary      撤銷已核准的認證
// @Description  ADM 撤銷一筆已核准的認證（例如事後發現證書失效）。reasonCode 必填（缺少回業務碼 3028）。撤銷會重建商品的認證投影，該標章立即從商品頁消失。
// @Tags         商品認證審核 (Product Certification)
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id       path  string                             true  "認證申請 ID（TSID 數字字串）"
// @Param        request  body  ReviewProductCertificationRequest  true  "審核請求（reasonCode 必填）"
// @Success      200  {object}  object{code=int,message=string,data=AdminProductCertificationApplicationItem}
// @Failure      400  {object}  object{code=int,message=string,traceId=string}
// @Failure      401  {object}  object{code=int,message=string,traceId=string}
// @Failure      403  {object}  object{code=int,message=string,traceId=string}
// @Failure      404  {object}  object{code=int,message=string,traceId=string}
// @Failure      409  {object}  object{code=int,message=string,traceId=string}
// @Router       /admin/product_certification_applications/{id}/revoke [post]
func (h *Holder) RevokeAdminProductCertification(c *gin.Context) {
	h.reviewAdminCertification(c, func(c *gin.Context, req *pb.ReviewProductCertificationForAdminRequest) (*pb.ProductCertificationApplicationAdminResponse, error) {
		return h.Product.RevokeProductCertificationForAdmin(c.Request.Context(), req)
	})
}

// ReverifyAdminProductCertification 重新查驗認證編號
// @Summary      重新查驗認證編號
// @Description  ADM 對一筆申請重新打政府開放資料平台查驗，寫回 govVerifyState / govVerifyPayload / govVerifiedAt 後回傳最新詳情。**只重新查驗、不改 status**——即使結果轉為 VERIFIED 也不自動核准，要核准仍走 approve。必須是 POST：它有副作用（消耗政府 API 配額並寫 DB），GET 會被瀏覽器與代理重放。
// @Tags         商品認證審核 (Product Certification)
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id       path  string                               true  "認證申請 ID（TSID 數字字串）"
// @Param        request  body  ReverifyProductCertificationRequest  true  "重新查驗請求"
// @Success      200  {object}  object{code=int,message=string,data=AdminProductCertificationApplicationItem}
// @Failure      400  {object}  object{code=int,message=string,traceId=string}
// @Failure      401  {object}  object{code=int,message=string,traceId=string}
// @Failure      403  {object}  object{code=int,message=string,traceId=string}
// @Failure      404  {object}  object{code=int,message=string,traceId=string}
// @Failure      409  {object}  object{code=int,message=string,traceId=string}
// @Router       /admin/product_certification_applications/{id}/reverify [post]
func (h *Holder) ReverifyAdminProductCertification(c *gin.Context) {
	accountID, ok := getAccountID(c)
	if !ok {
		response.Unauthorized(c)
		return
	}
	applicationID, ok := adminCertificationApplicationID(c)
	if !ok {
		return
	}

	var req ReverifyProductCertificationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "invalid request: "+err.Error())
		return
	}

	resp, err := h.Product.ReverifyProductCertificationForAdmin(c.Request.Context(), &pb.ReverifyProductCertificationForAdminRequest{
		ApplicationId:  applicationID,
		IdempotencyKey: req.IdempotencyKey,
		AccountId:      accountID,
	})
	respondAdminCertificationApplication(c, resp, err)
}
