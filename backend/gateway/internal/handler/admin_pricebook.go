package handler

import (
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/Will413028/hawkr/gateway/internal/response"
	pb "github.com/Will413028/hawkr/go-pkg/proto/pricebook"
)

// upsertCategoryMappingRequest is the HTTP body for UpsertCategoryMapping.
// It deliberately has no field for the operator identity (updatedBy /
// accountId): the operator comes exclusively from getAccountID(c), which
// reads the JWT-verified account set by the admin group's middleware — an
// admin must never be able to forge who made a mapping decision by putting
// a different identity in the request body.
type upsertCategoryMappingRequest struct {
	CategoryID string  `json:"categoryId" binding:"required"`
	Status     string  `json:"status" binding:"required,oneof=resolved unmapped ambiguous"`
	GroupCode  *string `json:"groupCode"`
	SourceNote *string `json:"sourceNote"`
}

// UpsertCategoryMapping writes an admin-curated categoryId→crop/group
// mapping.
// @Summary      新增／更新 categoryId 對應的 Pricebook 品項
// @Tags         後台市場行情分類 (Admin Pricebook Category Mapping)
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        body  body  upsertCategoryMappingRequest  true  "mapping 內容"
// @Success      200  {object}  object{code=int,data=object}
// @Failure      400  {object}  object{code=int,message=string}
// @Failure      404  {object}  object{code=int,message=string}
// @Router       /admin/pricebook/category-mapping [post]
func (h *Holder) UpsertCategoryMapping(c *gin.Context) {
	accountID, ok := getAccountID(c)
	if !ok {
		response.Unauthorized(c)
		return
	}
	var req upsertCategoryMappingRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	pbReq := &pb.UpsertCategoryMappingRequest{
		CategoryId: req.CategoryID, Status: req.Status,
		GroupCode: req.GroupCode, SourceNote: req.SourceNote,
		UpdatedBy: accountID,
	}
	resp, err := h.Pricebook.UpsertCategoryMapping(c.Request.Context(), pbReq)
	if err != nil {
		response.GrpcErr(c, "pricebook", err)
		return
	}
	response.OKProto(c, resp)
}

// ListCategoryMappingWorklist lists categories still needing an admin
// mapping decision.
// @Summary      待處理 categoryId mapping 清單
// @Tags         後台市場行情分類 (Admin Pricebook Category Mapping)
// @Produce      json
// @Security     BearerAuth
// @Param        status  query  string  false  "篩選：unmapped｜ambiguous，留空回全部"
// @Success      200  {object}  object{code=int,data=object}
// @Failure      400  {object}  object{code=int,message=string}
// @Router       /admin/pricebook/category-mapping [get]
func (h *Holder) ListCategoryMappingWorklist(c *gin.Context) {
	if _, ok := getAccountID(c); !ok {
		response.Unauthorized(c)
		return
	}
	statusFilter := strings.TrimSpace(c.Query("status"))
	// proto 契約寫明 status 只能是空字串｜unmapped｜ambiguous。未驗證前，
	// ?status=resolved 或 ?status=xxx 會靜默回空清單，讓操作者誤以為真的沒有
	// 待處理項目——那是一個看起來成功的錯誤答案，比 400 難查得多。
	switch statusFilter {
	case "", "unmapped", "ambiguous":
	default:
		response.BadRequest(c, "status 只能是 unmapped 或 ambiguous，留空表示不篩選")
		return
	}
	resp, err := h.Pricebook.ListCategoryMappingWorklist(c.Request.Context(), statusFilter)
	if err != nil {
		response.GrpcErr(c, "pricebook", err)
		return
	}
	response.OKProto(c, resp)
}
