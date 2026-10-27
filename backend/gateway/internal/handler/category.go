package handler

import (
	"net/http"

	"github.com/Will413028/hawkr/gateway/internal/response"
	pkglogger "github.com/Will413028/hawkr/go-pkg/logger"
	pb "github.com/Will413028/hawkr/go-pkg/proto/category"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// Category represents a product category (API response format)
type Category struct {
	ID         string      `json:"id"`
	Name       string      `json:"name"`
	ParentID   string      `json:"parentId"`
	ParentName string      `json:"parentName,omitempty"`
	Status     string      `json:"status,omitempty"`
	DeletedAt  string      `json:"deletedAt,omitempty"`
	Children   []*Category `json:"children,omitempty"`
}

// CreateCategoryRequest is the request body for creating a category
type CreateCategoryRequest struct {
	ParentID string `json:"parentId"`
	Name     string `json:"name" binding:"required"`
}

// UpdateCategoryRequest is the request body for updating a category name
type UpdateCategoryRequest struct {
	Name string `json:"name" binding:"required"`
}

// BatchSoftDeleteCategoryRequest is the request body for batch soft-delete
type BatchSoftDeleteCategoryRequest struct {
	CategoryIDs []string `json:"categoryIds" binding:"required"`
}

// BatchChangeProductCategoryRequest is the request body for batch changing product category
type BatchChangeProductCategoryRequest struct {
	SourceCategoryID string   `json:"sourceCategoryId,omitempty"`
	TargetCategoryID string   `json:"targetCategoryId" binding:"required"`
	ProductIDs       []string `json:"productIds,omitempty"`
}

// GetCategoryList
// @Summary 取得分類列表（扁平結構）
// @Description 取得所有啟用中的商品分類，以扁平列表格式回傳。公開 API，僅返回 ACTIVE 狀態的分類。
// @Tags 分類 (Category)
// @Accept json
// @Produce json
// @Success 200 {object} object{code=int,data=[]Category} "成功回傳分類列表"
// @Failure 404 {object} object{code=int,message=string} "找不到該分類"
// @Router /public/categories [get]
func (h *Holder) GetCategoryList(c *gin.Context) {
	resp, err := h.Category.GetCategoryTree(c.Request.Context(), "active", false)
	if err != nil {
		pkglogger.Info(c.Request.Context(), "Failed to get categories", zap.Error(err))
		response.GrpcErr(c, "product", err)
		return
	}
	if resp == nil || resp.Code != 0 {
		writeBackendError(c, resp)
		return
	}
	response.OK(c, flattenCategoryTree(resp.Categories))
}

// GetCategoryTree
// @Summary 取得分類樹狀結構
// @Description 取得所有啟用中的商品分類，以巢狀樹狀結構回傳。公開 API，僅返回 ACTIVE 狀態的分類。
// @Tags 分類 (Category)
// @Accept json
// @Produce json
// @Success 200 {object} object{code=int,data=[]Category} "成功回傳分類樹"
// @Router /public/categories/tree [get]
func (h *Holder) GetCategoryTree(c *gin.Context) {
	resp, err := h.Category.GetCategoryTree(c.Request.Context(), "active", false)
	if err != nil {
		pkglogger.Info(c.Request.Context(), "Failed to get category tree", zap.Error(err))
		response.GrpcErr(c, "product", err)
		return
	}
	if resp == nil || resp.Code != 0 {
		writeBackendError(c, resp)
		return
	}
	response.OK(c, convertCategoryNodeTree(resp.Categories))
}

// GetSellerCategoryTree
// @Summary 取得賣家分類樹狀結構
// @Description 取得賣家可見的商品分類樹（僅 ACTIVE 狀態，2026-04-25 新政策後不再含 self-pending）。
// @Tags 分類 (Category)
// @Accept json
// @Produce json
// @Security BearerAuth
// @Success 200 {object} object{code=int,data=[]Category} "成功回傳賣家分類樹"
// @Failure 401 {object} object{code=int,message=string} "未授權"
// @Router /app/categories/tree [get]
func (h *Holder) GetSellerCategoryTree(c *gin.Context) {
	if _, ok := getAccountID(c); !ok {
		response.Err(c, http.StatusUnauthorized, 401, "未授權：缺少賣家身份")
		return
	}
	// 新政策：賣家分類樹僅 ACTIVE，取消 self-pending 邏輯
	resp, err := h.Category.GetCategoryTree(c.Request.Context(), "active", false)
	if err != nil {
		pkglogger.Info(c.Request.Context(), "Failed to get seller category tree", zap.Error(err))
		response.GrpcErr(c, "product", err)
		return
	}
	if resp == nil || resp.Code != 0 {
		writeBackendError(c, resp)
		return
	}
	response.OK(c, convertCategoryNodeTree(resp.Categories))
}

// GetCategoryByID
// @Summary 依 ID 取得單一分類
// @Description 公開 API，僅返回 ACTIVE 狀態的分類。
// @Tags 分類 (Category)
// @Accept json
// @Produce json
// @Param id path string true "分類 ID"
// @Success 200 {object} object{code=int,data=Category} "成功回傳分類資訊"
// @Failure 400 {object} object{code=int,message=string} "無效的分類 ID"
// @Failure 404 {object} object{code=int,message=string} "找不到該分類"
// @Router /public/categories/{id} [get]
func (h *Holder) GetCategoryByID(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		response.BadRequest(c, "invalid category id")
		return
	}
	treeResp, err := h.Category.GetCategoryTree(c.Request.Context(), "active", false)
	if err != nil {
		response.GrpcErr(c, "product", err)
		return
	}
	cat := findCategoryInTree(treeResp.Categories, id)
	if cat == nil {
		response.GrpcResult(c, 404, "category not found", nil)
		return
	}
	response.OK(c, Category{
		ID: cat.Id, Name: cat.Name, ParentID: cat.ParentId,
		ParentName: cat.ParentName, Status: cat.Status,
	})
}

// GetCategoryChildren
// @Summary 取得分類的直接子分類
// @Description 公開 API，僅返回 ACTIVE 狀態的子分類。
// @Tags 分類 (Category)
// @Accept json
// @Produce json
// @Param id path string true "父分類 ID"
// @Success 200 {object} object{code=int,data=[]Category} "成功回傳子分類列表"
// @Failure 400 {object} object{code=int,message=string} "無效的分類 ID"
// @Router /public/categories/{id}/children [get]
func (h *Holder) GetCategoryChildren(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		response.BadRequest(c, "invalid category id")
		return
	}
	resp, err := h.Category.GetChildCategories(c.Request.Context(), id)
	if err != nil {
		response.GrpcErr(c, "product", err)
		return
	}
	if resp == nil || resp.Code != 0 {
		writeBackendError(c, resp)
		return
	}
	categories := make([]Category, len(resp.Categories))
	for i, cat := range resp.Categories {
		categories[i] = Category{
			ID: cat.Id, Name: cat.Name, ParentID: cat.ParentId,
			ParentName: cat.ParentName, Status: cat.Status,
		}
	}
	response.OK(c, categories)
}

// GetCategoryPath
// @Summary 取得分類路徑
// @Description 公開 API，僅返回 ACTIVE 狀態分類的路徑。
// @Tags 分類 (Category)
// @Accept json
// @Produce json
// @Param id path string true "分類 ID"
// @Success 200 {object} object{code=int,data=[]Category} "從根到指定分類的路徑"
// @Failure 404 {object} object{code=int,message=string} "找不到該分類"
// @Router /public/categories/{id}/path [get]
func (h *Holder) GetCategoryPath(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		response.BadRequest(c, "invalid category id")
		return
	}
	treeResp, err := h.Category.GetCategoryTree(c.Request.Context(), "active", false)
	if err != nil {
		response.GrpcErr(c, "product", err)
		return
	}
	path := findCategoryPath(treeResp.Categories, id, nil)
	if path == nil {
		response.NotFound(c, "category not found")
		return
	}
	result := make([]Category, len(path))
	for i, cat := range path {
		result[i] = Category{
			ID: cat.Id, Name: cat.Name, ParentID: cat.ParentId,
			ParentName: cat.ParentName, Status: cat.Status,
		}
	}
	response.OK(c, result)
}

// CreateCategory
// @Summary 賣家新增分類
// @Description 新增分類（任何層級皆可，2026-04-25 新政策）。同名同父若已存在 ACTIVE 分類，會自動套用既有 ID 透明回傳；撞到軟刪除同名則回 CATEGORY_NAME_RESERVED。
// @Tags 分類 (Category)
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param request body CreateCategoryRequest true "新增分類請求"
// @Success 200 {object} object{code=int,data=Category} "新增成功"
// @Failure 400 {object} object{code=int,message=string} "無效的請求參數"
// @Failure 401 {object} object{code=int,message=string} "未授權"
// @Router /app/categories [post]
func (h *Holder) CreateCategory(c *gin.Context) {
	createdBy, ok := getAccountID(c)
	if !ok {
		response.Err(c, http.StatusUnauthorized, 401, "未授權：缺少用戶身份")
		return
	}
	var req CreateCategoryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "無效的請求參數: "+err.Error())
		return
	}
	resp, err := h.Category.CreateCategory(c.Request.Context(), req.ParentID, req.Name, createdBy)
	if err != nil {
		response.GrpcErr(c, "product", err)
		return
	}
	if resp == nil || resp.Code != 0 {
		writeBackendError(c, resp)
		return
	}
	response.OK(c, protoToCategory(resp.Category))
}

// GetAdminCategoryTree
// @Summary 管理員取得分類樹
// @Description 預設僅顯示 ACTIVE 分類；加 query `?include=deleted` 顯示含軟刪除分類。需要管理員權限。
// @Tags 管理員 (Admin)
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param include query string false "包含已軟刪除分類，傳 'deleted' 顯示" Enums(deleted)
// @Success 200 {object} object{code=int,data=[]Category} "分類樹"
// @Failure 403 {object} object{code=int,message=string} "禁止訪問"
// @Router /admin/categories/tree [get]
func (h *Holder) GetAdminCategoryTree(c *gin.Context) {
	includeDeleted := c.Query("include") == "deleted"
	status := "active"
	if includeDeleted {
		status = ""
	}
	resp, err := h.Category.GetCategoryTree(c.Request.Context(), status, includeDeleted)
	if err != nil {
		response.GrpcErr(c, "product", err)
		return
	}
	if resp == nil || resp.Code != 0 {
		writeBackendError(c, resp)
		return
	}
	response.OK(c, convertCategoryNodeTree(resp.Categories))
}

// CreateCategoryAdmin
// @Summary 管理員新增分類
// @Description 任何層級皆可新增；同名同父自動套用，撞軟刪除回 CATEGORY_NAME_RESERVED。
// @Tags 管理員 (Admin)
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param request body CreateCategoryRequest true "新增分類請求"
// @Success 200 {object} object{code=int,data=Category} "新增成功"
// @Router /admin/categories [post]
func (h *Holder) CreateCategoryAdmin(c *gin.Context) {
	createdBy, ok := getAccountID(c)
	if !ok {
		response.Err(c, http.StatusUnauthorized, 401, "未授權")
		return
	}
	var req CreateCategoryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	resp, err := h.Category.CreateCategoryForAdmin(c.Request.Context(), req.ParentID, req.Name, createdBy)
	if err != nil {
		response.GrpcErr(c, "product", err)
		return
	}
	if resp == nil || resp.Code != 0 {
		writeBackendError(c, resp)
		return
	}
	response.OK(c, protoToCategory(resp.Category))
}

// UpdateCategoryAdmin
// @Summary 管理員修改分類名稱
// @Description 僅可改 name，不可改 parent_id。改父類別請走「批次更換商品分類」 API。
// @Tags 管理員 (Admin)
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path string true "分類 ID"
// @Param request body UpdateCategoryRequest true "新名稱"
// @Success 200 {object} object{code=int,data=Category} "修改成功"
// @Failure 404 {object} object{code=int,message=string} "分類不存在"
// @Router /admin/categories/{id} [patch]
func (h *Holder) UpdateCategoryAdmin(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		response.BadRequest(c, "invalid id")
		return
	}
	updatedBy, ok := getAccountID(c)
	if !ok {
		response.Err(c, http.StatusUnauthorized, 401, "未授權")
		return
	}
	var req UpdateCategoryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	resp, err := h.Category.UpdateCategoryForAdmin(c.Request.Context(), id, req.Name, updatedBy)
	if err != nil {
		response.GrpcErr(c, "product", err)
		return
	}
	if resp == nil || resp.Code != 0 {
		writeBackendError(c, resp)
		return
	}
	response.OK(c, protoToCategory(resp.Category))
}

// BatchSoftDeleteCategoryAdmin
// @Summary 管理員批次軟刪除分類
// @Description 按拓樸排序執行（L3→L2→L1）；底下無商品+無 ACTIVE 子分類才能刪；任一筆失敗整批 rollback。
// @Tags 管理員 (Admin)
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param request body BatchSoftDeleteCategoryRequest true "批次刪除請求"
// @Success 200 {object} object{code=int,data=object{deletedIds=[]string}} "刪除成功"
// @Failure 400 {object} object{code=int,message=string} "驗證失敗（如有商品/有子分類）"
// @Router /admin/categories [delete]
func (h *Holder) BatchSoftDeleteCategoryAdmin(c *gin.Context) {
	updatedBy, ok := getAccountID(c)
	if !ok {
		response.Err(c, http.StatusUnauthorized, 401, "未授權")
		return
	}
	var req BatchSoftDeleteCategoryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	resp, err := h.Category.BatchSoftDeleteCategoryForAdmin(c.Request.Context(), req.CategoryIDs, updatedBy)
	if err != nil {
		response.GrpcErr(c, "product", err)
		return
	}
	if resp == nil || resp.Code != 0 {
		writeBackendError(c, resp)
		return
	}
	response.OK(c, map[string]interface{}{"deletedIds": resp.DeletedIds})
}

// RestoreCategoryAdmin
// @Summary 管理員還原已軟刪除分類
// @Description 還原 status=9003 的分類；若同名同父已存在 ACTIVE 或父分類已軟刪除則拒絕。
// @Tags 管理員 (Admin)
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path string true "分類 ID"
// @Success 200 {object} object{code=int,data=Category} "還原成功"
// @Failure 400 {object} object{code=int,message=string} "驗證失敗"
// @Router /admin/categories/{id}/restore [post]
func (h *Holder) RestoreCategoryAdmin(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		response.BadRequest(c, "invalid id")
		return
	}
	updatedBy, ok := getAccountID(c)
	if !ok {
		response.Err(c, http.StatusUnauthorized, 401, "未授權")
		return
	}
	resp, err := h.Category.RestoreCategoryForAdmin(c.Request.Context(), id, updatedBy)
	if err != nil {
		response.GrpcErr(c, "product", err)
		return
	}
	if resp == nil || resp.Code != 0 {
		writeBackendError(c, resp)
		return
	}
	response.OK(c, protoToCategory(resp.Category))
}

// ==================== 輔助方法 ====================

func writeBackendError(c *gin.Context, resp interface {
	GetCode() int32
	GetMessage() string
}) {
	if resp == nil {
		response.GrpcResult(c, 500, "unknown error", nil)
		return
	}
	msg := resp.GetMessage()
	if msg == "" {
		msg = "unknown error"
	}
	response.GrpcResult(c, resp.GetCode(), msg, nil)
}

func protoToCategory(item *pb.CategoryItem) Category {
	if item == nil {
		return Category{}
	}
	return Category{
		ID: item.Id, Name: item.Name, ParentID: item.ParentId,
		ParentName: item.ParentName, Status: item.Status, DeletedAt: item.DeletedAt,
	}
}

// convertCategoryNodeTree converts protobuf CategoryNode to Category
func convertCategoryNodeTree(nodes []*pb.CategoryNode) []*Category {
	result := make([]*Category, len(nodes))
	for i, node := range nodes {
		result[i] = &Category{
			ID:         node.Id,
			Name:       node.Name,
			ParentID:   node.ParentId,
			ParentName: node.ParentName,
			Status:     node.Status,
			DeletedAt:  node.DeletedAt,
			Children:   convertCategoryNodeTree(node.Children),
		}
	}
	return result
}

// flattenCategoryTree 將樹狀結構扁平化
func flattenCategoryTree(nodes []*pb.CategoryNode) []Category {
	result := make([]Category, 0)
	var flatten func([]*pb.CategoryNode)
	flatten = func(nodes []*pb.CategoryNode) {
		for _, node := range nodes {
			result = append(result, Category{
				ID:         node.Id,
				Name:       node.Name,
				ParentID:   node.ParentId,
				ParentName: node.ParentName,
				Status:     node.Status,
				DeletedAt:  node.DeletedAt,
			})
			if node.Children != nil {
				flatten(node.Children)
			}
		}
	}
	flatten(nodes)
	return result
}

// findCategoryInTree 在樹狀結構中尋找指定 ID 的分類
func findCategoryInTree(nodes []*pb.CategoryNode, id string) *pb.CategoryNode {
	for _, node := range nodes {
		if node.Id == id {
			return node
		}
		if node.Children != nil {
			found := findCategoryInTree(node.Children, id)
			if found != nil {
				return found
			}
		}
	}
	return nil
}

// findCategoryPath 在樹狀結構中尋找從根到指定 ID 的路徑
func findCategoryPath(nodes []*pb.CategoryNode, targetID string, currentPath []*pb.CategoryNode) []*pb.CategoryNode {
	for _, node := range nodes {
		// 對 sibling 各自 clone 一份 path，避免共用 underlying array 互相覆蓋
		newPath := make([]*pb.CategoryNode, len(currentPath), len(currentPath)+1)
		copy(newPath, currentPath)
		newPath = append(newPath, node)
		if node.Id == targetID {
			return newPath
		}
		if node.Children != nil {
			found := findCategoryPath(node.Children, targetID, newPath)
			if found != nil {
				return found
			}
		}
	}
	return nil
}
