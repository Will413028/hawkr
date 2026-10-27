package handler

import (
	"errors"
	"strconv"

	"github.com/Will413028/hawkr/gateway/internal/buyerid"
	"github.com/Will413028/hawkr/gateway/internal/response"
	"github.com/Will413028/hawkr/gateway/internal/storeid"
	"github.com/Will413028/hawkr/go-pkg/identity"
	pkglogger "github.com/Will413028/hawkr/go-pkg/logger"
	pb "github.com/Will413028/hawkr/go-pkg/proto/social"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// ==================== B-135: Reporting & Blocking ====================

// resolveModerationUserID maps the caller's login principal (account_id) to the
// identity social-service records in reports.reporter_id / user_blocks.blocker_id.
//
// That id space is "a live store_id OR a live buyer_id" — social-service says so
// itself (BlockUseCase.resolvePrincipal) and since #2789 it refuses, fail-closed,
// any id neither role service claims. A raw account_id only belongs to that space
// by accident: a grandfathered stint has store_id == buyer_id == account_id, so it
// resolves today and stops resolving the day that account is re-keyed (設計書 44,
// re-key W2/W3) — at which point the caller cannot block anyone (#2867).
//
// So every arm that produces an identity produces it from a stint resolver. When
// the active role's axis reports no stint the other axis is consulted before
// giving up: accountType is a bitmask, so "active as buyer, no buyer stint" does
// not imply "no store", and vice versa. Only an account with no live stint on
// either axis reaches the raw account_id, and there it is a placeholder for "this
// account has no social identity at all" rather than a value we believe in —
// social-service rejects it on its own terms.
//
// Never 403: which stint an account currently holds is not an authorization
// question (that policy predates the seller branch and applies to both axes). The
// two secondary lookups are best-effort for the same reason — a role service being
// down must not turn this arm into a failure mode it never had, and the
// downstream fail-closed check still catches the value it produces.
//
// loginRole==0 is not "never picked a role" — login clamps to a role the account
// holds (sanitizeActiveRole), so it means the role held at login has since been
// revoked. There is no stint axis to resolve against, hence 400 rather than a
// fallback (537d12dfe).
func (h *Holder) resolveModerationUserID(c *gin.Context, accountID int64) (int64, bool) {
	loginRoleRaw, _ := c.Get("loginRole")
	loginRole, _ := loginRoleRaw.(int)
	switch loginRole {
	case 1:
		resolved, err := h.BuyerID.CurrentBuyerID(c.Request.Context(), identity.AccountID(accountID))
		if err != nil {
			if errors.Is(err, buyerid.ErrNoCurrentBuyer) {
				return h.moderationStoreStintOrRaw(c, accountID), true
			}
			response.GrpcErr(c, "buyer", err)
			return 0, false
		}
		return int64(resolved), true
	case 2:
		resolved, err := h.Store.CurrentStoreID(c.Request.Context(), identity.AccountID(accountID))
		if err != nil {
			if errors.Is(err, storeid.ErrNoCurrentStore) {
				return h.moderationBuyerStintOrRaw(c, accountID), true
			}
			response.GrpcErr(c, "seller", err)
			return 0, false
		}
		return int64(resolved), true
	default:
		response.BadRequest(c, "active role required")
		return 0, false
	}
}

// moderationStoreStintOrRaw is resolveModerationUserID's secondary lookup for a
// buyer-active caller whose buyer axis reported no stint: the same account may
// still hold a live store, and that store_id is the identity social-service can
// actually resolve. Writes no response — returns accountID unchanged when the
// seller axis cannot answer either.
func (h *Holder) moderationStoreStintOrRaw(c *gin.Context, accountID int64) int64 {
	resolved, err := h.Store.CurrentStoreID(c.Request.Context(), identity.AccountID(accountID))
	if err != nil {
		if !errors.Is(err, storeid.ErrNoCurrentStore) {
			pkglogger.Warn(c.Request.Context(),
				"moderation: seller axis unavailable for a no-buyer-stint account; sending raw account_id",
				zap.Int64("accountId", accountID), zap.Error(err))
		}
		return accountID
	}
	return int64(resolved)
}

// moderationBuyerStintOrRaw is the mirror of moderationStoreStintOrRaw for a
// seller-active caller whose seller axis reported no current store.
func (h *Holder) moderationBuyerStintOrRaw(c *gin.Context, accountID int64) int64 {
	resolved, err := h.BuyerID.CurrentBuyerID(c.Request.Context(), identity.AccountID(accountID))
	if err != nil {
		if !errors.Is(err, buyerid.ErrNoCurrentBuyer) {
			pkglogger.Warn(c.Request.Context(),
				"moderation: buyer axis unavailable for a no-store-stint account; sending raw account_id",
				zap.Int64("accountId", accountID), zap.Error(err))
		}
		return accountID
	}
	return int64(resolved)
}

// SubmitReportRequest is the JSON body for POST /app/reports.
// evidenceKeys 承載裸 objectKey（49 §6.1／10 檔 §25），下游 social-service 存進
// reports.evidence JSONB、proto 欄位即 evidence_keys；不接受完整 URL。
type SubmitReportRequest struct {
	TargetType   string   `json:"targetType"   binding:"required"`
	TargetID     string   `json:"targetId"     binding:"required"`
	ReasonCode   string   `json:"reasonCode"   binding:"required"`
	Description  string   `json:"description"`
	EvidenceKeys []string `json:"evidenceKeys"`
}

type BlockRecordDTO struct {
	ID           string `json:"id"`
	BlockedID    string `json:"blockedId"`
	CreateTimeMs string `json:"createTimeMs"`
}

func toBlockRecordDTOs(blocks []*pb.UserBlock) []BlockRecordDTO {
	out := make([]BlockRecordDTO, 0, len(blocks))
	for _, block := range blocks {
		if block == nil {
			continue
		}
		out = append(out, BlockRecordDTO{
			ID:           block.GetId(),
			BlockedID:    block.GetBlockedId(),
			CreateTimeMs: strconv.FormatInt(block.GetCreateTimeMs(), 10),
		})
	}
	return out
}

// SubmitReport
// @Summary      提交檢舉
// @Description  買家或賣家對貼文/留言/討論區留言/使用者提交檢舉（B-135 / Apple Guideline 1.2 / Issue #645）
// @Tags         檢舉與封鎖 (Moderation)
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        request  body  SubmitReportRequest  true  "檢舉內容"
// @Success      200  {object}  object{code=int,message=string,data=object{reportId=string,status=string}}
// @Failure      400  {object}  object{code=int,message=string,traceId=string}
// @Failure      401  {object}  object{code=int,message=string,traceId=string}
// @Failure      503  {object}  object{code=int,message=string,traceId=string}
// @Router       /app/reports [post]
func (h *Holder) SubmitReport(c *gin.Context) {
	userID, ok := getAccountID(c)
	if !ok {
		response.Unauthorized(c)
		return
	}
	userID, ok = h.resolveModerationUserID(c, userID)
	if !ok {
		return
	}
	var req SubmitReportRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	// 前端可控寫入路徑一律只收 objectKey（10 檔 §25）：剝殼本平台顯示 URL、拒絕外來域名
	// 與非 public/ 前綴，避免完整 URL 落進 reports.evidence。
	evidenceKeys := make([]string, 0, len(req.EvidenceKeys))
	for _, raw := range req.EvidenceKeys {
		key, kerr := resolveInputObjectKey(h.MediaBaseURL, raw, "", "public/")
		if kerr != nil {
			response.BadRequest(c, kerr.Error())
			return
		}
		if key == "" {
			continue
		}
		evidenceKeys = append(evidenceKeys, key)
	}
	resp, err := h.Social.SubmitReport(c.Request.Context(), userID, req.TargetType, req.TargetID, req.ReasonCode, req.Description, evidenceKeys)
	if err != nil {
		response.GrpcErr(c, "social", err)
		return
	}
	response.OK(c, gin.H{
		"reportId": resp.ReportId,
		"status":   resp.Status,
	})
}

// ListMyReports
// @Summary      列出我送出的檢舉
// @Description  分頁查詢當前登入者送出的檢舉紀錄（B-135 / Apple Guideline 1.2 / Issue #645）
// @Tags         檢舉與封鎖 (Moderation)
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        page       query  int  false  "頁碼 (預設 1)"
// @Param        page_size  query  int  false  "每頁筆數 (預設 20)"
// @Success      200  {object}  object{code=int,message=string,data=object{records=[]object}}
// @Failure      401  {object}  object{code=int,message=string,traceId=string}
// @Failure      503  {object}  object{code=int,message=string,traceId=string}
// @Router       /app/reports/me [get]
func (h *Holder) ListMyReports(c *gin.Context) {
	userID, ok := getAccountID(c)
	if !ok {
		response.Unauthorized(c)
		return
	}
	// ② buyer store-stint re-key：見 SubmitReport 同款註解（reporter_id stint-scoped）。
	userID, ok = h.resolveModerationUserID(c, userID)
	if !ok {
		return
	}
	page := parseIntDefault(c.Query("page"), 1)
	pageSize := parseIntDefault(c.Query("page_size"), 20)
	resp, err := h.Social.ListMyReports(c.Request.Context(), userID, int32(page), int32(pageSize))
	if err != nil {
		response.GrpcErr(c, "social", err)
		return
	}
	response.OK(c, gin.H{"records": toReportList(resp.Reports)})
}

// BlockUser
// @Summary      封鎖使用者
// @Description  將指定 userId 加入當前登入者的封鎖名單（B-135 / Apple Guideline 1.2 / Issue #645）
// @Tags         檢舉與封鎖 (Moderation)
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        userId  path  string  true  "要封鎖的使用者 ID"
// @Success      200  {object}  object{code=int,message=string,data=object}
// @Failure      400  {object}  object{code=int,message=string,traceId=string}
// @Failure      401  {object}  object{code=int,message=string,traceId=string}
// @Failure      503  {object}  object{code=int,message=string,traceId=string}
// @Router       /app/blocks/{userId} [post]
func (h *Holder) BlockUser(c *gin.Context) {
	userID, ok := getAccountID(c)
	if !ok {
		response.Unauthorized(c)
		return
	}
	// ② buyer store-stint re-key：blocker_id stint-scoped；見 SubmitReport 同款註解。
	userID, ok = h.resolveModerationUserID(c, userID)
	if !ok {
		return
	}
	blockedID := c.Param("userId")
	if blockedID == "" {
		response.BadRequest(c, "invalid user id")
		return
	}
	if _, err := h.Social.BlockUser(c.Request.Context(), userID, blockedID); err != nil {
		response.GrpcErr(c, "social", err)
		return
	}
	response.OK(c, gin.H{})
}

// UnblockUser
// @Summary      解除封鎖使用者
// @Description  從當前登入者的封鎖名單中移除指定 userId
// @Tags         檢舉與封鎖 (Moderation)
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        userId  path  string  true  "要解除封鎖的使用者 ID"
// @Success      200  {object}  object{code=int,message=string,data=object}
// @Failure      400  {object}  object{code=int,message=string,traceId=string}
// @Failure      401  {object}  object{code=int,message=string,traceId=string}
// @Failure      503  {object}  object{code=int,message=string,traceId=string}
// @Router       /app/blocks/{userId} [delete]
func (h *Holder) UnblockUser(c *gin.Context) {
	userID, ok := getAccountID(c)
	if !ok {
		response.Unauthorized(c)
		return
	}
	// ② buyer store-stint re-key：blocker_id stint-scoped；見 SubmitReport 同款註解。
	userID, ok = h.resolveModerationUserID(c, userID)
	if !ok {
		return
	}
	blockedID := c.Param("userId")
	if blockedID == "" {
		response.BadRequest(c, "invalid user id")
		return
	}
	if _, err := h.Social.UnblockUser(c.Request.Context(), userID, blockedID); err != nil {
		response.GrpcErr(c, "social", err)
		return
	}
	response.OK(c, gin.H{})
}

// ListBlocks
// @Summary      列出我的封鎖名單
// @Description  分頁查詢當前登入者封鎖的使用者清單（B-135 / Apple Guideline 1.2 / Issue #645）
// @Tags         檢舉與封鎖 (Moderation)
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        page       query  int  false  "頁碼 (預設 1)"
// @Param        page_size  query  int  false  "每頁筆數 (預設 20)"
// @Success      200  {object}  object{code=int,message=string,data=object{records=[]object}}
// @Failure      401  {object}  object{code=int,message=string,traceId=string}
// @Failure      503  {object}  object{code=int,message=string,traceId=string}
// @Router       /app/blocks [get]
func (h *Holder) ListBlocks(c *gin.Context) {
	userID, ok := getAccountID(c)
	if !ok {
		response.Unauthorized(c)
		return
	}
	// ② buyer store-stint re-key：blocker_id stint-scoped；見 SubmitReport 同款註解。
	userID, ok = h.resolveModerationUserID(c, userID)
	if !ok {
		return
	}
	page := parseIntDefault(c.Query("page"), 1)
	pageSize := parseIntDefault(c.Query("page_size"), 20)
	resp, err := h.Social.ListBlocks(c.Request.Context(), userID, int32(page), int32(pageSize))
	if err != nil {
		response.GrpcErr(c, "social", err)
		return
	}
	response.OK(c, gin.H{"records": toBlockRecordDTOs(resp.Blocks)})
}

// AdminListReports
// @Summary      管理員：列出檢舉紀錄
// @Description  依 status/target_type/reason_code 篩選並分頁查詢檢舉紀錄
// @Tags         檢舉與封鎖 Admin (Admin Moderation)
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        status       query  string  false  "狀態篩選（PENDING/RESOLVED_*/DISMISSED/APPEALED）"
// @Param        target_type  query  string  false  "檢舉對象類型篩選"
// @Param        reason_code  query  string  false  "檢舉原因代碼篩選"
// @Param        page         query  int     false  "頁碼 (預設 1)"
// @Param        page_size    query  int     false  "每頁筆數 (預設 20)"
// @Success      200  {object}  object{code=int,message=string,data=object{records=[]object}}
// @Failure      401  {object}  object{code=int,message=string,traceId=string}
// @Failure      503  {object}  object{code=int,message=string,traceId=string}
// @Router       /admin/reports [get]
func (h *Holder) AdminListReports(c *gin.Context) {
	page := parseIntDefault(c.Query("page"), 1)
	pageSize := parseIntDefault(c.Query("page_size"), 20)
	resp, err := h.Social.AdminListReports(c.Request.Context(), c.Query("status"), c.Query("target_type"), c.Query("reason_code"), int32(page), int32(pageSize))
	if err != nil {
		response.GrpcErr(c, "social", err)
		return
	}
	response.OK(c, gin.H{"records": toReportList(resp.Reports)})
}

// AdminGetReport
// @Summary      管理員：取得單筆檢舉
// @Description  依 report id 取回完整檢舉紀錄（含對象、理由、證據、處理狀態）
// @Tags         檢舉與封鎖 Admin (Admin Moderation)
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id  path  string  true  "檢舉紀錄 ID"
// @Success      200  {object}  object{code=int,message=string,data=object}
// @Failure      400  {object}  object{code=int,message=string,traceId=string}
// @Failure      401  {object}  object{code=int,message=string,traceId=string}
// @Failure      403  {object}  object{code=int,message=string,traceId=string}
// @Failure      404  {object}  object{code=int,message=string,traceId=string}
// @Failure      503  {object}  object{code=int,message=string,traceId=string}
// @Router       /admin/reports/{id} [get]
func (h *Holder) AdminGetReport(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		response.BadRequest(c, "invalid report id")
		return
	}
	resp, err := h.Social.AdminGetReport(c.Request.Context(), id)
	if err != nil {
		response.GrpcErr(c, "social", err)
		return
	}
	response.OK(c, toReportData(resp.Report))
}

// AdminResolveReportRequest is the JSON body for POST /admin/reports/:id/resolve.
type AdminResolveReportRequest struct {
	Action string `json:"action" binding:"required"` // HIDE_TARGET / WARN_USER / BAN_USER / DISMISS
	Note   string `json:"note"`
}

// AdminResolveReport
// @Summary      管理員：處理檢舉
// @Description  對檢舉做裁決，action 可為 HIDE_TARGET / WARN_USER / BAN_USER / DISMISS；note 為內部備註
// @Tags         檢舉與封鎖 Admin (Admin Moderation)
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id       path  string  true  "檢舉紀錄 ID"
// @Param        request  body  object{action=string,note=string}  true  "處理動作與備註"
// @Success      200  {object}  object{code=int,message=string,data=object}
// @Failure      400  {object}  object{code=int,message=string,traceId=string}
// @Failure      401  {object}  object{code=int,message=string,traceId=string}
// @Failure      403  {object}  object{code=int,message=string,traceId=string}
// @Failure      404  {object}  object{code=int,message=string,traceId=string}
// @Failure      503  {object}  object{code=int,message=string,traceId=string}
// @Router       /admin/reports/{id}/resolve [post]
func (h *Holder) AdminResolveReport(c *gin.Context) {
	adminID, ok := getAccountID(c)
	if !ok {
		response.Unauthorized(c)
		return
	}
	id := c.Param("id")
	if id == "" {
		response.BadRequest(c, "invalid report id")
		return
	}
	var req AdminResolveReportRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	resp, err := h.Social.AdminResolveReport(c.Request.Context(), id, adminID, req.Action, req.Note)
	if err != nil {
		response.GrpcErr(c, "social", err)
		return
	}
	response.OK(c, toReportData(resp.Report))
}

// AdminReopenReportRequest is the JSON body for POST /admin/reports/:id/reopen.
type AdminReopenReportRequest struct {
	Note string `json:"note"`
}

// AdminReopenReport
// @Summary      管理員：重新開啟已處理的檢舉（申訴）
// @Description  把 RESOLVED_*/DISMISSED 的檢舉轉為 APPEALED，可再次呼叫 resolve 端點裁決
//
// CODEMETA_SUBSET_OK：這裡刻意只列 ReportStatus 的四個終態，不含 PENDING——
// 沒被裁決過的檢舉沒有「重新開啟」這件事可做，述詞在
// report_repository.go 的 reopenableStatuses 同樣排除它。
// @Tags         檢舉與封鎖 Admin (Admin Moderation)
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id       path  string  true  "檢舉紀錄 ID"
// @Param        request  body  object{note=string}  true  "重開原因"
// @Success      200  {object}  object{code=int,message=string,data=object}
// @Failure      400  {object}  object{code=int,message=string,traceId=string}
// @Failure      401  {object}  object{code=int,message=string,traceId=string}
// @Failure      404  {object}  object{code=int,message=string,traceId=string}
// @Router       /admin/reports/{id}/reopen [post]
func (h *Holder) AdminReopenReport(c *gin.Context) {
	adminID, ok := getAccountID(c)
	if !ok {
		response.Unauthorized(c)
		return
	}
	id := c.Param("id")
	if id == "" {
		response.BadRequest(c, "invalid report id")
		return
	}
	var req AdminReopenReportRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	resp, err := h.Social.AdminReopenReport(c.Request.Context(), id, adminID, req.Note)
	if err != nil {
		response.GrpcErr(c, "social", err)
		return
	}
	response.OK(c, toReportData(resp.Report))
}

func parseIntDefault(s string, def int) int {
	if s == "" {
		return def
	}
	v, err := strconv.Atoi(s)
	if err != nil {
		return def
	}
	return v
}

// reportData 是檢舉紀錄（B-135）的 REST 回應 DTO，取代直接序列化 *pb.Report。
// id 皆為 64-bit TSID（> JS 2^53）：proto 已將其定義為 string（§6 Track 1 B5），
// 故為原生 string 序列化，避免 web / admin 端 JSON.parse 精度丟失（對齊 notification
// #960 / comment）。resolved_by 為 admin id，暫維持 ,string。*_ms 是毫秒 timestamp
// （< 2^53）維持 number。欄位名比照 proto 的 snake_case json tag，確保前端既有契約
// 不變——故不用 protojson（會把欄位名改成 lowerCamelCase）。
type reportData struct {
	TargetType        string `json:"targetType,omitempty"`
	ReasonCode        string `json:"reasonCode,omitempty"`
	Description       string `json:"description,omitempty"`
	Status            string `json:"status,omitempty"`
	ResolvedAction    string `json:"resolvedAction,omitempty"`
	ID                string `json:"id,omitempty"`
	ReporterID        string `json:"reporterId,omitempty"`
	TargetID          string `json:"targetId,omitempty"`
	ResolvedBy        int64  `json:"resolvedBy,string,omitempty"`
	ResolvedAtMs      int64  `json:"resolvedAtMs,omitempty"`
	CreateTimeMs      int64  `json:"createTimeMs,omitempty"`
	RecentActionCount string `json:"recentActionCount,omitempty"`
}

func toReportData(r *pb.Report) *reportData {
	if r == nil {
		return nil
	}
	return &reportData{
		ID:                r.Id,
		ReporterID:        r.ReporterId,
		TargetType:        r.TargetType,
		TargetID:          r.TargetId,
		ReasonCode:        r.ReasonCode,
		Description:       r.Description,
		Status:            r.Status,
		ResolvedAction:    r.ResolvedAction,
		ResolvedBy:        r.ResolvedBy,
		ResolvedAtMs:      r.ResolvedAtMs,
		CreateTimeMs:      r.CreateTimeMs,
		RecentActionCount: r.RecentActionCount,
	}
}

func toReportList(items []*pb.Report) []reportData {
	result := make([]reportData, 0, len(items))
	for _, r := range items {
		if r == nil {
			continue
		}
		result = append(result, *toReportData(r))
	}
	return result
}
