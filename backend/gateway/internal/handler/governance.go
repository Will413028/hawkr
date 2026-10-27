package handler

import (
	"strconv"

	"github.com/Will413028/hawkr/gateway/internal/response"
	pb "github.com/Will413028/hawkr/go-pkg/proto/seller"
	"github.com/gin-gonic/gin"
)

// B-169 Governance HTTP handlers — issue #722.

// ========== Reporter side ==========

type SubmitViolationReportRequest struct {
	TargetSellerID   string   `json:"targetSellerId"   binding:"required"`
	TargetProductID  string   `json:"targetProductId"`
	TargetPostID     string   `json:"targetPostId"`
	TargetOrderNo    string   `json:"targetOrderNo"`
	Category         string   `json:"category"           binding:"required"`
	Severity         string   `json:"severity"           binding:"required"`
	Description      string   `json:"description"        binding:"required"`
	EvidenceAssetIDs []string `json:"evidenceAssetIds"`
}

type ViolationReportDTO struct {
	ID                     string   `json:"id"`
	TargetSellerID         string   `json:"targetSellerId"`
	TargetProductID        string   `json:"targetProductId"`
	TargetPostID           string   `json:"targetPostId"`
	TargetOrderNo          string   `json:"targetOrderNo"`
	ReporterID             string   `json:"reporterId"`
	Category               string   `json:"category"`
	Severity               string   `json:"severity"`
	Description            string   `json:"description"`
	EvidenceAssetIDs       []string `json:"evidenceAssetIds"`
	Status                 string   `json:"status"`
	InvestigatingAtMs      string   `json:"investigatingAtMs"`
	ResolvedAtMs           string   `json:"resolvedAtMs"`
	AdminResolutionSummary string   `json:"adminResolutionSummary"`
	CreatedAtMs            string   `json:"createdAtMs"`
}

func toViolationReportDTO(report *pb.SellerViolationReport) ViolationReportDTO {
	if report == nil {
		return ViolationReportDTO{EvidenceAssetIDs: []string{}}
	}
	evidence := report.GetEvidenceAssetIds()
	if evidence == nil {
		evidence = []string{}
	}
	return ViolationReportDTO{
		ID:                     report.GetId(),
		TargetSellerID:         report.GetTargetSellerId(),
		TargetProductID:        report.GetTargetProductId(),
		TargetPostID:           report.GetTargetPostId(),
		TargetOrderNo:          report.GetTargetOrderNo(),
		ReporterID:             report.GetReporterAccountId(),
		Category:               report.GetCategory(),
		Severity:               report.GetSeverity(),
		Description:            report.GetDescription(),
		EvidenceAssetIDs:       evidence,
		Status:                 report.GetStatus(),
		InvestigatingAtMs:      strconv.FormatInt(report.GetInvestigatingAtMs(), 10),
		ResolvedAtMs:           strconv.FormatInt(report.GetResolvedAtMs(), 10),
		AdminResolutionSummary: report.GetAdminResolutionSummary(),
		CreatedAtMs:            strconv.FormatInt(report.GetCreatedAtMs(), 10),
	}
}

func toViolationReportDTOs(reports []*pb.SellerViolationReport) []ViolationReportDTO {
	out := make([]ViolationReportDTO, 0, len(reports))
	for _, report := range reports {
		if report != nil {
			out = append(out, toViolationReportDTO(report))
		}
	}
	return out
}

type PublicViolationDTO struct {
	ID            string `json:"id"`
	Category      string `json:"category"`
	Severity      string `json:"severity"`
	PenaltyType   string `json:"penaltyType"`
	PublicSummary string `json:"publicSummary"`
	EffectiveAtMs string `json:"effectiveAtMs"`
	ExpiresAtMs   string `json:"expiresAtMs"`
}

func toPublicViolationDTOs(violations []*pb.PublicSellerViolation) []PublicViolationDTO {
	out := make([]PublicViolationDTO, 0, len(violations))
	for _, violation := range violations {
		if violation == nil {
			continue
		}
		out = append(out, PublicViolationDTO{
			ID:            violation.GetId(),
			Category:      violation.GetCategory(),
			Severity:      violation.GetSeverity(),
			PenaltyType:   violation.GetPenaltyType(),
			PublicSummary: violation.GetPublicSummary(),
			EffectiveAtMs: strconv.FormatInt(violation.GetEffectiveAtMs(), 10),
			ExpiresAtMs:   strconv.FormatInt(violation.GetExpiresAtMs(), 10),
		})
	}
	return out
}

// SubmitViolationReport — 買家/賣家對賣家提交違規檢舉
// @Summary      提交商家違規檢舉
// @Description  買家或賣家對特定商家提交違規檢舉（食安、假認證、誤導、刷單等）。回應含 duplicate 旗標，重複 24h 內同 reporter+target 會以 code=4090 回。
// @Tags         商家治理 (Governance)
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        body  body  handler.SubmitViolationReportRequest  true  "檢舉內容"
// @Success      200  {object}  object{code=int,data=object{reportId=string,duplicate=bool}}
// @Failure      400  {object}  object{code=int,message=string,traceId=string}
// @Failure      401  {object}  object{code=int,message=string,traceId=string}
// @Failure      409  {object}  object{code=int,message=string,traceId=string}  "duplicate report (4090)"
// @Failure      503  {object}  object{code=int,message=string,traceId=string}
// @Router       /app/violation_reports [post]
func (h *Holder) SubmitViolationReport(c *gin.Context) {
	userID, ok := getAccountID(c)
	if !ok {
		response.Unauthorized(c)
		return
	}
	var req SubmitViolationReportRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	resp, err := h.Seller.CreateViolationReport(c.Request.Context(), &pb.CreateViolationReportRequest{
		ReporterAccountId: userID,
		TargetSellerId:    req.TargetSellerID,
		TargetProductId:   req.TargetProductID,
		TargetPostId:      req.TargetPostID,
		TargetOrderNo:     req.TargetOrderNo,
		Category:          req.Category,
		Severity:          req.Severity,
		Description:       req.Description,
		EvidenceAssetIds:  req.EvidenceAssetIDs,
	})
	if err != nil {
		response.GrpcErr(c, "seller", err)
		return
	}
	response.GrpcResult(c, resp.Code, resp.Message, gin.H{
		"reportId":  resp.ReportId,
		"duplicate": resp.Duplicate,
	})
}

// ListMyViolationReports — 查詢我提交的檢舉與處理進度
// @Summary      我提交的檢舉列表
// @Description  回傳當前登入者提交的違規檢舉，含 status / admin_resolution_summary 處理進度。
// @Tags         商家治理 (Governance)
// @Produce      json
// @Security     BearerAuth
// @Param        limit   query  int  false  "每頁筆數"  default(20)
// @Param        offset  query  int  false  "偏移量"    default(0)
// @Success      200  {object}  object{code=int,data=object{records=array}}
// @Failure      401  {object}  object{code=int,message=string,traceId=string}
// @Failure      503  {object}  object{code=int,message=string,traceId=string}
// @Router       /app/violation_reports/me [get]
func (h *Holder) ListMyViolationReports(c *gin.Context) {
	userID, ok := getAccountID(c)
	if !ok {
		response.Unauthorized(c)
		return
	}
	limit := int32(parseIntDefault(c.Query("limit"), 20))
	offset := int32(parseIntDefault(c.Query("offset"), 0))
	resp, err := h.Seller.ListMyViolationReports(c.Request.Context(), &pb.ListMyViolationReportsRequest{
		ReporterAccountId: userID,
		Limit:             limit,
		Offset:            offset,
	})
	if err != nil {
		response.GrpcErr(c, "seller", err)
		return
	}
	response.GrpcResult(c, resp.Code, resp.Message, gin.H{"records": toViolationReportDTOs(resp.Reports)})
}

// ========== Public Food Safety ==========

type SubmitFoodSafetyReportRequest struct {
	TargetSellerID  string `json:"targetSellerId"`
	TargetProductID string `json:"targetProductId"`
	TargetOrderNo   string `json:"targetOrderNo"`
	ReporterPhone   string `json:"reporterPhone"     binding:"required"`
	ReporterEmail   string `json:"reporterEmail"`
	ReporterName    string `json:"reporterName"`
	Description     string `json:"description"        binding:"required"`
	// issue #1527：匿名食安通報不接受佐證附件（deny-by-default）。刻意不宣告 evidence_asset_ids —
	// 前端帶入會被忽略（第一層）；繞 Gateway 直打 gRPC 則由 seller-service domain Validate 拒（第二層）。
}

// SubmitFoodSafetyReport — 公開無登入食安通報入口
//
// @Summary      食安緊急通報（公開無登入）
// @Description  公開食安通報入口，無需登入。reporter_phone 必填用於回追，severity 後端自動設 critical，4h 內 admin 必須 ack。本入口另有路由專屬雙層限流（10 分鐘 5 次、1 小時 10 次），超出回 429；額度算在限流身分上（一般訪客即其來源 IP，SSR／CI 等共用出口另有獨立身分軸）。本入口不接受佐證附件（匿名端 deny-by-default，issue #1527）；需附佐證圖請登入改走 POST /app/violation_reports（category=food_safety）。
// @Tags         商家治理 (Governance)
// @Accept       json
// @Produce      json
// @Param        body  body  handler.SubmitFoodSafetyReportRequest  true  "食安通報內容"
// @Success      200  {object}  object{code=int,data=object{reportId=string,ackEstimatedAt=string}}
// @Failure      400  {object}  object{code=int,message=string,traceId=string}
// @Failure      429  {object}  object{code=int,message=string,traceId=string}  "rate limit exceeded（10 分鐘 5 次 或 1 小時 10 次）"
// @Failure      503  {object}  object{code=int,message=string,traceId=string}
// @Router       /public/food_safety_reports [post]
func (h *Holder) SubmitFoodSafetyReport(c *gin.Context) {
	var req SubmitFoodSafetyReportRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	resp, err := h.Seller.CreateFoodSafetyReport(c.Request.Context(), &pb.CreateFoodSafetyReportRequest{
		TargetSellerId:  req.TargetSellerID,
		TargetProductId: req.TargetProductID,
		TargetOrderNo:   req.TargetOrderNo,
		ReporterPhone:   req.ReporterPhone,
		ReporterEmail:   req.ReporterEmail,
		ReporterName:    req.ReporterName,
		Description:     req.Description,
	})
	if err != nil {
		response.GrpcErr(c, "seller", err)
		return
	}
	response.GrpcResult(c, resp.Code, resp.Message, gin.H{
		"reportId":       resp.ReportId,
		"ackEstimatedAt": resp.AckEstimatedAt,
	})
}

// ========== Public Seller Page Violations ==========

// GetPublicSellerViolations — 公開賣家頁查違規記錄（透明化）
// @Summary      公開賣家違規記錄
// @Description  公開查詢指定賣家過去 90 天 high+ severity 違規記錄（D8 cache）。買家在賣家頁可見，無需登入。
// @Tags         商家治理 (Governance)
// @Produce      json
// @Param        id  path  int  true  "賣家 ID"
// @Success      200  {object}  object{code=int,data=object{records=array}}
// @Failure      400  {object}  object{code=int,message=string,traceId=string}
// @Failure      404  {object}  object{code=int,message=string,traceId=string}
// @Failure      503  {object}  object{code=int,message=string,traceId=string}
// @Router       /public/sellers/{id}/violations/public [get]
func (h *Holder) GetPublicSellerViolations(c *gin.Context) {
	sellerID := c.Param("id")
	if sellerID == "" {
		response.BadRequest(c, "invalid seller id")
		return
	}
	resp, err := h.Seller.GetPublicSellerViolations(c.Request.Context(), &pb.GetPublicSellerViolationsRequest{
		SellerId: sellerID,
	})
	if err != nil {
		response.GrpcErr(c, "seller", err)
		return
	}
	response.GrpcResult(c, resp.Code, resp.Message, gin.H{"records": toPublicViolationDTOs(resp.Violations)})
}
