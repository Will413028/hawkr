package handler

import (
	"context"
	"strconv"
	"strings"
	"time"

	"github.com/Will413028/hawkr/gateway/internal/response"
	"github.com/Will413028/hawkr/go-pkg/codemeta"
	"github.com/Will413028/hawkr/go-pkg/identity"
	"github.com/Will413028/hawkr/go-pkg/mediaurl"
	"github.com/Will413028/hawkr/go-pkg/pagination"
	pb "github.com/Will413028/hawkr/go-pkg/proto/social"
	"github.com/Will413028/hawkr/go-pkg/timeutil"
	"github.com/gin-gonic/gin"
)

// ==================== Post Handlers ====================

// GetLatestPosts 取得全站最新貼文列表
// @Summary      取得全站最新貼文列表
// @Description  公開取得所有商家的最新貼文，依建立時間倒序，支援分頁
// @Tags         貼文 (Post)
// @Accept       json
// @Produce      json
// @Param        page       query  int     false  "頁碼"          default(1)
// @Param        page_size  query  int     false  "每頁筆數"      default(20)
// @Param        tag        query  string  false  "單一 hashtag 篩選（不含 #；server normalize）"
// @Success      200  {object}  object{code=int,data=object{records=[]PostListItemData,total=string,page=string,pageSize=string}}
// @Failure      503  {object}  object{code=int,message=string}
// @Router       /public/posts [get]
func (h *Holder) GetLatestPosts(c *gin.Context) {
	page, pageSize := parsePagePageSize(c, 20, maxPostPageSize)
	tag := strings.TrimSpace(c.Query("tag"))
	buyerID, ok := h.socialBuyerID(c, getOptionalAccountID(c))
	if !ok {
		return
	}

	// sellerID="" means all sellers
	resp, err := h.Social.GetPostList(c.Request.Context(), buyerID, "", page, pageSize, tag)
	if err != nil {
		response.GrpcErr(c, "social", err)
		return
	}
	if resp.Code != 0 {
		response.GrpcResult(c, resp.Code, resp.Message, nil)
		return
	}
	items := toPostListItems(h.MediaBaseURL, resp.Data)
	h.enrichPostsWithSellerInfo(c.Request.Context(), items)
	response.Page(c, items, strconv.FormatInt(resp.Total, 10), strconv.Itoa(int(resp.Page)), strconv.Itoa(int(resp.PageSize)))
}

// GetSellerPosts 取得賣家貼文列表
// @Summary      取得賣家貼文列表
// @Description  公開取得指定賣家的貼文列表，支援分頁
// @Tags         貼文 (Post)
// @Accept       json
// @Produce      json
// @Param        id         path   string  true   "店家 ID（store_id）"
// @Param        page       query  int  false  "頁碼"      default(1)
// @Param        page_size  query  int  false  "每頁筆數"  default(20)
// @Success      200  {object}  object{code=int,data=object{records=[]PostListItemData,total=string,page=string,pageSize=string}}
// @Failure      400  {object}  object{code=int,message=string}
// @Failure      503  {object}  object{code=int,message=string}
// @Router       /public/sellers/{id}/posts [get]
func (h *Holder) GetSellerPosts(c *gin.Context) {
	storeID, ok := h.requireStore(c, c.Param("id"), gateExists)
	if !ok {
		return
	}
	page, pageSize := parsePagePageSize(c, 20, maxPostPageSize)
	buyerID, ok := h.socialBuyerID(c, getOptionalAccountID(c))
	if !ok {
		return
	}

	resp, err := h.Social.GetPostList(c.Request.Context(), buyerID, strconv.FormatInt(storeID, 10), page, pageSize, "")
	if err != nil {
		response.GrpcErr(c, "social", err)
		return
	}
	if resp.Code != 0 {
		response.GrpcResult(c, resp.Code, resp.Message, nil)
		return
	}
	items := toPostListItems(h.MediaBaseURL, resp.Data)
	h.enrichPostsWithSellerInfo(c.Request.Context(), items)
	response.Page(c, items, strconv.FormatInt(resp.Total, 10), strconv.Itoa(int(resp.Page)), strconv.Itoa(int(resp.PageSize)))
}

// GetPostDetail 取得貼文詳情
// @Summary      取得貼文詳情
// @Description  公開取得單篇貼文詳情，含圖片、商品、互動統計
// @Tags         貼文 (Post)
// @Accept       json
// @Produce      json
// @Param        id  path  string  true  "貼文 ID"
// @Success      200  {object}  object{code=int,data=PostDetailData}
// @Failure      400  {object}  object{code=int,message=string}
// @Failure      404  {object}  object{code=int,message=string}
// @Failure      503  {object}  object{code=int,message=string}
// @Router       /public/posts/{id} [get]
func (h *Holder) GetPostDetail(c *gin.Context) {
	postID := c.Param("id")
	if postID == "" {
		response.BadRequest(c, "invalid post id")
		return
	}
	buyerID, ok := h.socialBuyerID(c, getOptionalAccountID(c))
	if !ok {
		return
	}

	resp, err := h.Social.GetPostDetail(c.Request.Context(), buyerID, postID)
	if err != nil {
		response.GrpcErr(c, "social", err)
		return
	}
	if resp.Code != 0 {
		response.GrpcResult(c, resp.Code, resp.Message, nil)
		return
	}
	response.OK(c, toPostDetailData(h.MediaBaseURL, resp.Data))
}

// CreatePost 新增貼文
// @Summary      新增貼文
// @Description  賣家新增一篇貼文，可附一張圖片（staged_token）與一個商品（product_id）
// @Tags         貼文 (Post)
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        request  body  object{content=string,stagedToken=string,productId=string,tags=[]string}  true  "貼文內容"
// @Success      200  {object}  object{code=int,data=PostDetailData}
// @Failure      400  {object}  object{code=int,message=string}
// @Failure      401  {object}  object{code=int,message=string}
// @Failure      503  {object}  object{code=int,message=string}
// @Router       /app/sellers/posts [post]
func (h *Holder) CreatePost(c *gin.Context) {
	storeID, ok := h.resolveStoreID(c)
	if !ok {
		return
	}

	var req struct {
		ProductID   *string  `json:"productId"`
		Content     string   `json:"content"`
		StagedToken string   `json:"stagedToken"`
		Tags        []string `json:"tags"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	resp, err := h.Social.CreatePost(c.Request.Context(), storeID, req.Content, req.StagedToken, req.ProductID, req.Tags)
	if err != nil {
		// GrpcDomainErr parses [NNNN] prefix → biz code + messageZh (hashtag 3015-3018).
		response.GrpcDomainErr(c, "social", err)
		return
	}
	if resp.Code != 0 {
		response.GrpcResult(c, resp.Code, resp.Message, nil)
		return
	}
	response.OK(c, toPostDetailData(h.MediaBaseURL, resp.Data))
}

// UpdatePost 更新貼文
// @Summary      更新貼文
// @Description  賣家更新指定貼文，可替換圖片（keep_asset_id 保留舊圖、new_staged_token 換新圖）與商品
// @Tags         貼文 (Post)
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id       path  string                                                                                        true  "貼文 ID"
// @Param        request  body  object{content=string,keepAssetId=string,newStagedToken=string,productId=string,tags=[]string}  true  "更新內容（tags 採 PATCH 語意：未傳 = 不動；傳 [] 或 [...] = 替換）"
// @Success      200  {object}  object{code=int,data=PostDetailData}
// @Failure      400  {object}  object{code=int,message=string}
// @Failure      401  {object}  object{code=int,message=string}
// @Failure      403  {object}  object{code=int,message=string}
// @Failure      404  {object}  object{code=int,message=string}
// @Failure      503  {object}  object{code=int,message=string}
// @Router       /app/sellers/posts/{id} [put]
func (h *Holder) UpdatePost(c *gin.Context) {
	storeID, ok := h.resolveStoreID(c)
	if !ok {
		return
	}
	postID := c.Param("id")
	if postID == "" {
		response.BadRequest(c, "invalid post id")
		return
	}

	var req struct {
		ProductID      *string   `json:"productId"`
		Content        string    `json:"content"`
		KeepAssetID    string    `json:"keepAssetId"`
		NewStagedToken string    `json:"newStagedToken"`
		Tags           *[]string `json:"tags"` // nil = 不傳；非 nil（含空 slice） = 替換
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	var (
		tagsPresent bool
		tags        []string
	)
	if req.Tags != nil {
		tagsPresent = true
		tags = *req.Tags
	}

	resp, err := h.Social.UpdatePost(c.Request.Context(), postID, storeID, req.Content, req.KeepAssetID, req.NewStagedToken, req.ProductID, tagsPresent, tags)
	if err != nil {
		// GrpcDomainErr parses [NNNN] prefix → biz code + messageZh (hashtag 3015-3018).
		response.GrpcDomainErr(c, "social", err)
		return
	}
	if resp.Code != 0 {
		response.GrpcResult(c, resp.Code, resp.Message, nil)
		return
	}
	response.OK(c, toPostDetailData(h.MediaBaseURL, resp.Data))
}

// DeletePost 刪除貼文
// @Summary      刪除貼文
// @Description  賣家刪除自己的貼文（軟刪除）
// @Tags         貼文 (Post)
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id  path  string  true  "貼文 ID"
// @Success      200  {object}  object{code=int,message=string}
// @Failure      401  {object}  object{code=int,message=string}
// @Failure      403  {object}  object{code=int,message=string}
// @Failure      404  {object}  object{code=int,message=string}
// @Failure      503  {object}  object{code=int,message=string}
// @Router       /app/sellers/posts/{id} [delete]
func (h *Holder) DeletePost(c *gin.Context) {
	storeID, ok := h.resolveStoreID(c)
	if !ok {
		return
	}
	postID := c.Param("id")
	if postID == "" {
		response.BadRequest(c, "invalid post id")
		return
	}

	_, err := h.Social.DeletePost(c.Request.Context(), postID, storeID)
	if err != nil {
		response.GrpcErr(c, "social", err)
		return
	}
	response.OK(c, nil)
}

// PublishMyPost 賣家發布自己的貼文
// @Summary      發布貼文
// @Description  賣家把自己的未發布或已下架貼文轉為已發布，發布後才會出現在公開動態與 hashtag 列表
// @Tags         貼文 (Post)
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id  path  string  true  "貼文 ID"
// @Success      200  {object}  object{code=int,data=PostDetailData}
// @Failure      400  {object}  object{code=int,message=string}
// @Failure      401  {object}  object{code=int,message=string}
// @Failure      403  {object}  object{code=int,message=string}
// @Failure      404  {object}  object{code=int,message=string}
// @Failure      503  {object}  object{code=int,message=string}
// @Router       /app/sellers/posts/{id}/publish [post]
func (h *Holder) PublishMyPost(c *gin.Context) {
	h.updateMyPostPublishStatus(c, codemeta.PostPublishStatusPublished)
}

// UnpublishMyPost 賣家下架自己的貼文
// @Summary      下架貼文
// @Description  賣家把自己的已發布貼文轉為已下架，內容保留但不再出現在公開動態；與平台強制下架（hiddenAt）是兩件事
// @Tags         貼文 (Post)
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id  path  string  true  "貼文 ID"
// @Success      200  {object}  object{code=int,data=PostDetailData}
// @Failure      400  {object}  object{code=int,message=string}
// @Failure      401  {object}  object{code=int,message=string}
// @Failure      403  {object}  object{code=int,message=string}
// @Failure      404  {object}  object{code=int,message=string}
// @Failure      503  {object}  object{code=int,message=string}
// @Router       /app/sellers/posts/{id}/unpublish [post]
func (h *Holder) UnpublishMyPost(c *gin.Context) {
	h.updateMyPostPublishStatus(c, codemeta.PostPublishStatusUnpublished)
}

// updateMyPostPublishStatus 是賣家自己那條發布／下架路徑的共同體。
//
// 這裡沒有 expectedVersion：樂觀鎖是代行路徑的機制，用來擋「管理員看到的版本已經被
// 賣家改掉」；賣家對自己的貼文按下發布時沒有第二個寫入者要防，強收一個版本號只會讓
// 前端多一次詳情往返。上游對 expected_version 缺席的處理見 social-service 的
// SetPostPublishStatus（noVersionCAS sentinel）。
//
// scheduled_publish_at 同樣不開：social-service 要求排程必須落在代行工作階段內（它靠
// scheduled_by_session_id 重建授權），賣家路徑沒有 session 可指，帶了只會被拒。
func (h *Holder) updateMyPostPublishStatus(c *gin.Context, targetStatus int) {
	storeID, ok := h.resolveStoreID(c)
	if !ok {
		return
	}
	postID := c.Param("id")
	if postID == "" {
		response.BadRequest(c, "invalid post id")
		return
	}

	resp, err := h.Social.UpdateMyPostStatus(c.Request.Context(), postID, storeID, strconv.Itoa(targetStatus))
	if err != nil {
		response.GrpcDomainErr(c, "social", err)
		return
	}
	if resp.Code != 0 {
		response.GrpcResult(c, resp.Code, resp.Message, nil)
		return
	}
	response.OK(c, toPostDetailData(h.MediaBaseURL, resp.Data))
}

// TogglePostLike 切換貼文按讚
// @Summary      切換貼文按讚
// @Description  對指定貼文按讚或取消按讚（Toggle）
// @Tags         貼文 (Post)
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id  path  string  true  "貼文 ID"
// @Success      200  {object}  object{code=int,data=object{liked=bool}}
// @Failure      401  {object}  object{code=int,message=string}
// @Failure      404  {object}  object{code=int,message=string}
// @Failure      503  {object}  object{code=int,message=string}
// @Router       /app/posts/{id}/like [post]
func (h *Holder) TogglePostLike(c *gin.Context) {
	userID, ok := getAccountID(c)
	if !ok {
		response.Unauthorized(c)
		return
	}
	postID := c.Param("id")
	if postID == "" {
		response.BadRequest(c, "invalid post id")
		return
	}

	caller, ok := h.socialCaller(c, userID)
	if !ok {
		return
	}

	resp, err := h.Social.TogglePostLike(c.Request.Context(), caller.BuyerID, postID, identity.StoreID(caller.StoreID))
	if err != nil {
		response.GrpcErr(c, "social", err)
		return
	}
	if resp.Code != 0 {
		response.GrpcResult(c, resp.Code, resp.Message, nil)
		return
	}
	response.OK(c, gin.H{"liked": resp.Liked})
}

// SharePost 分享貼文
// @Summary      分享貼文
// @Description  記錄貼文分享，回傳最新分享次數
// @Tags         貼文 (Post)
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id       path  string                       true   "貼文 ID"
// @Param        request  body  object{platform=string}   false  "分享平台（選填）"
// @Success      200  {object}  object{code=int,data=object{shareCount=int}}
// @Failure      401  {object}  object{code=int,message=string}
// @Failure      404  {object}  object{code=int,message=string}
// @Failure      503  {object}  object{code=int,message=string}
// @Router       /app/posts/{id}/share [post]
func (h *Holder) SharePost(c *gin.Context) {
	userID, ok := getAccountID(c)
	if !ok {
		response.Unauthorized(c)
		return
	}
	postID := c.Param("id")
	if postID == "" {
		response.BadRequest(c, "invalid post id")
		return
	}

	// 刻意不解析身份：h.Social.SharePost 的 userID 參數在 social-service 端從未
	// 被讀取（server-side 只做 share_count+1，不寫入任何 identity 欄位——見
	// services/social-service/internal/usecase/post.go 的 SharePost(ctx, postID)
	// 簽章），解析身份只會平白讓沒有現任身分的帳號被擋在分享這個完全不需要
	// 身份識別的動作外。

	var req struct {
		Platform string `json:"platform"`
	}
	_ = c.ShouldBindJSON(&req)

	resp, err := h.Social.SharePost(c.Request.Context(), postID, userID, req.Platform)
	if err != nil {
		response.GrpcErr(c, "social", err)
		return
	}
	if resp.Code != 0 {
		response.GrpcResult(c, resp.Code, resp.Message, nil)
		return
	}
	response.OK(c, gin.H{"shareCount": resp.ShareCount})
}

// ReportPost 舉報貼文
// @Summary      舉報貼文
// @Description  買家對指定貼文進行舉報，同一買家對同一貼文重複舉報回傳 200（冪等）
// @Tags         貼文 (Post)
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id       path  string               true  "貼文 ID"
// @Param        request  body  ReportPostRequest  true  "舉報內容"
// @Success      200  {object}  object{code=int,data=object{reportId=string,createdAt=string}}
// @Failure      400  {object}  object{code=int,message=string}
// @Failure      401  {object}  object{code=int,message=string}
// @Failure      503  {object}  object{code=int,message=string}
// @Router       /app/sellers/posts/{id}/reports [post]
func (h *Holder) ReportPost(c *gin.Context) {
	userID, ok := getAccountID(c)
	if !ok {
		response.Unauthorized(c)
		return
	}
	postID := c.Param("id")
	if postID == "" {
		response.BadRequest(c, "invalid post id")
		return
	}

	userID, ok = h.resolveModerationUserID(c, userID)
	if !ok {
		return
	}

	var req ReportPostRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	resp, err := h.Social.ReportPost(c.Request.Context(), userID, postID, req.ReasonCode, req.Description)
	if err != nil {
		response.GrpcErr(c, "social", err)
		return
	}
	if resp.Code != 0 {
		response.GrpcResult(c, resp.Code, resp.Message, nil)
		return
	}
	response.OK(c, gin.H{
		"reportId":  resp.ReportId,
		"createdAt": resp.CreatedAt,
	})
}

// ListMyPosts 取得我的貼文列表（賣家）
// @Summary      取得我的貼文列表
// @Description  賣家取得自己所有貼文，可依狀態篩選，支援分頁；hidden 狀態貼文含下架原因；每筆帶 publishStatus 碼值以分辨未發布／已發布／已下架
// @Tags         貼文 (Post)
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        status     query  string  false  "貼文狀態篩選，留空回傳全部（published＝已發布且未被平台下架；hidden＝被平台下架）"  Enums(, published, hidden)
// @Param        page       query  int     false  "頁碼"                        default(1)
// @Param        page_size  query  int     false  "每頁筆數"                    default(20)
// @Success      200  {object}  object{code=int,data=object{records=[]MyPostListItemData,total=string,page=string,pageSize=string}}
// @Failure      401  {object}  object{code=int,message=string}
// @Failure      503  {object}  object{code=int,message=string}
// @Router       /app/sellers/me/posts [get]
func (h *Holder) ListMyPosts(c *gin.Context) {
	storeID, ok := h.resolveStoreID(c)
	if !ok {
		return
	}
	status := c.Query("status")
	page, pageSize := parsePagePageSize(c, 20, maxPostPageSize)

	resp, err := h.Social.ListMyPosts(c.Request.Context(), storeID, status, page, pageSize)
	if err != nil {
		response.GrpcErr(c, "social", err)
		return
	}
	if resp.Code != 0 {
		response.GrpcResult(c, resp.Code, resp.Message, nil)
		return
	}
	response.Page(c, toMyPostListItems(h.MediaBaseURL, resp.Data), strconv.FormatInt(resp.Total, 10), strconv.Itoa(int(resp.Page)), strconv.Itoa(int(resp.PageSize)))
}

// GetMyPostDetail 取得我的單篇貼文（賣家）：不分發布狀態與是否被平台下架（編輯頁使用），非自己的貼文回 404。
func (h *Holder) GetMyPostDetail(c *gin.Context) {
	storeID, ok := h.resolveStoreID(c)
	if !ok {
		return
	}
	postID := c.Param("id")
	if postID == "" {
		response.BadRequest(c, "invalid post id")
		return
	}

	resp, err := h.Social.GetMyPostDetail(c.Request.Context(), storeID, postID)
	if err != nil {
		response.GrpcErr(c, "social", err)
		return
	}
	if resp.Code != 0 {
		response.GrpcResult(c, resp.Code, resp.Message, nil)
		return
	}
	data, err := toMyPostDetailData(h.MediaBaseURL, resp.Data)
	if err != nil {
		response.ServiceUnavailable(c, "social")
		return
	}
	response.OK(c, data)
}

// AdminListPosts 管理後台取得貼文列表
// @Summary      管理後台取得貼文列表
// @Description  管理員取得所有貼文，可依狀態篩選，支援分頁，含舉報統計
// @Tags         管理 - 貼文 (Admin Post)
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        status     query  string  false  "貼文狀態篩選，留空回傳全部"  Enums(, published, hidden, deleted)
// @Param        page       query  int     false  "頁碼"                        default(1)
// @Param        page_size  query  int     false  "每頁筆數"                    default(20)
// @Success      200  {object}  object{code=int,data=object{records=[]AdminPostListItemData,total=string,page=string,pageSize=string}}
// @Failure      401  {object}  object{code=int,message=string}
// @Failure      503  {object}  object{code=int,message=string}
// @Router       /admin/posts [get]
func (h *Holder) AdminListPosts(c *gin.Context) {
	status := c.Query("status")
	page, pageSize := parsePagePageSize(c, 20, maxPostPageSize)

	resp, err := h.Social.AdminListPosts(c.Request.Context(), status, page, pageSize)
	if err != nil {
		response.GrpcErr(c, "social", err)
		return
	}
	if resp.Code != 0 {
		response.GrpcResult(c, resp.Code, resp.Message, nil)
		return
	}
	response.Page(c, toAdminPostListItems(h.MediaBaseURL, resp.Data), strconv.FormatInt(resp.Total, 10), strconv.Itoa(int(resp.Page)), strconv.Itoa(int(resp.PageSize)))
}

// AdminModeratePost 管理後台審核貼文
// @Summary      管理後台審核貼文
// @Description  管理員對貼文執行下架（HIDE）或恢復上架（RESTORE）
// @Tags         管理 - 貼文 (Admin Post)
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id       path  string                        true  "貼文 ID"
// @Param        request  body  AdminModeratePostRequest   true  "審核操作"
// @Success      200  {object}  object{code=int}
// @Failure      400  {object}  object{code=int,message=string}
// @Failure      401  {object}  object{code=int,message=string}
// @Failure      404  {object}  object{code=int,message=string}
// @Failure      503  {object}  object{code=int,message=string}
// @Router       /admin/posts/{id} [patch]
func (h *Holder) AdminModeratePost(c *gin.Context) {
	postID := c.Param("id")
	if postID == "" {
		response.BadRequest(c, "invalid post id")
		return
	}

	var req AdminModeratePostRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	operatorID, ok := getAccountID(c)
	if !ok {
		response.Unauthorized(c)
		return
	}
	resp, err := h.Social.AdminModeratePost(c.Request.Context(), postID, req.Action, req.Reason, req.ReasonCode, operatorID)
	if err != nil {
		response.GrpcErr(c, "social", err)
		return
	}
	if resp.Code != 0 {
		response.GrpcResult(c, resp.Code, resp.Message, nil)
		return
	}
	response.OK(c, nil)
}

// ==================== Comment Handlers ====================

// GetPostComments 取得貼文留言列表
// @Summary      取得貼文留言列表
// @Description  公開取得指定貼文的頂層留言，每則留言附帶最新 3 則回覆
// @Tags         留言 (Comment)
// @Accept       json
// @Produce      json
// @Param        id         path   string  true   "貼文 ID"
// @Param        page       query  int  false  "頁碼"      default(1)
// @Param        page_size  query  int  false  "每頁筆數"  default(20)
// @Success      200  {object}  object{code=int,data=object{records=[]object{id=string,postId=string,userName=string,avatarKey=string,avatarUrl=string,content=string,parentId=string,replyToId=string,replyToName=string,likeCount=int,replyCount=int,role=int,isMine=bool,isLiked=bool,createdAt=int,replies=[]object{id=string,postId=string,userName=string,avatarKey=string,avatarUrl=string,content=string,parentId=string,replyToId=string,replyToName=string,likeCount=int,role=int,isMine=bool,isLiked=bool,createdAt=int}},total=string,page=string,pageSize=string}}
// @Failure      400  {object}  object{code=int,message=string}
// @Failure      503  {object}  object{code=int,message=string}
// @Router       /public/posts/{id}/comments [get]
func (h *Holder) GetPostComments(c *gin.Context) {
	postID := c.Param("id")
	if postID == "" {
		response.BadRequest(c, "invalid post id")
		return
	}
	page, pageSize := parsePagePageSize(c, 20, maxPostPageSize)
	caller, ok := h.socialCaller(c, getOptionalAccountID(c))
	if !ok {
		return
	}

	resp, err := h.Social.GetCommentList(c.Request.Context(), postID, caller, page, pageSize)
	if err != nil {
		response.GrpcErr(c, "social", err)
		return
	}
	if resp.Code != 0 {
		response.GrpcResult(c, resp.Code, resp.Message, nil)
		return
	}
	response.Page(c, toCommentList(h.MediaBaseURL, resp.Data), strconv.FormatInt(resp.Total, 10), strconv.Itoa(int(resp.Page)), strconv.Itoa(int(resp.PageSize)))
}

// GetCommentReplies 取得留言回覆列表
// @Summary      取得留言回覆列表
// @Description  公開取得指定留言的所有回覆，支援分頁
// @Tags         留言 (Comment)
// @Accept       json
// @Produce      json
// @Param        id         path   string  true   "留言 ID"
// @Param        page       query  int  false  "頁碼"      default(1)
// @Param        page_size  query  int  false  "每頁筆數"  default(20)
// @Success      200  {object}  object{code=int,data=object{records=[]object{id=string,postId=string,userName=string,avatarKey=string,avatarUrl=string,content=string,parentId=string,replyToId=string,replyToName=string,likeCount=int,role=int,isMine=bool,isLiked=bool,createdAt=int},total=string,page=string,pageSize=string}}
// @Failure      400  {object}  object{code=int,message=string}
// @Failure      503  {object}  object{code=int,message=string}
// @Router       /public/posts/comments/{id}/replies [get]
func (h *Holder) GetCommentReplies(c *gin.Context) {
	commentID := c.Param("id")
	if commentID == "" {
		response.BadRequest(c, "invalid comment id")
		return
	}
	page, pageSize := parsePagePageSize(c, 20, maxPostPageSize)
	caller, ok := h.socialCaller(c, getOptionalAccountID(c))
	if !ok {
		return
	}

	resp, err := h.Social.GetCommentReplies(c.Request.Context(), "", caller, commentID, page, pageSize)
	if err != nil {
		response.GrpcErr(c, "social", err)
		return
	}
	if resp.Code != 0 {
		response.GrpcResult(c, resp.Code, resp.Message, nil)
		return
	}
	response.Page(c, toCommentList(h.MediaBaseURL, resp.Data), strconv.FormatInt(resp.Total, 10), strconv.Itoa(int(resp.Page)), strconv.Itoa(int(resp.PageSize)))
}

// CreatePostComment 新增貼文留言
// @Summary      新增貼文留言
// @Description  對指定貼文新增頂層留言（任何登入用戶均可）。可帶 clientRequestId 用於 SSE 推回時 dedup optimistic 樂觀更新（見 §41）
// @Tags         留言 (Comment)
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id       path  string                                                     true  "貼文 ID"
// @Param        request  body  object{content=string,clientRequestId=string} true  "留言內容；clientRequestId 為 UUID，用於 SSE event 回推時 dedup optimistic update。留言身分由伺服器依貼文歸屬推導（自己店的貼文是店家，其餘是本人）"
// @Success      200  {object}  object{code=int,data=object{id=string,postId=string,userName=string,avatarKey=string,avatarUrl=string,content=string,parentId=string,replyToId=string,replyToName=string,likeCount=int,replyCount=int,role=int,isMine=bool,isLiked=bool,createdAt=int}}
// @Failure      400  {object}  object{code=int,message=string}
// @Failure      401  {object}  object{code=int,message=string}
// @Failure      404  {object}  object{code=int,message=string}
// @Failure      503  {object}  object{code=int,message=string}
// @Router       /app/posts/{id}/comments [post]
func (h *Holder) CreatePostComment(c *gin.Context) {
	userID, ok := getAccountID(c)
	if !ok {
		response.Unauthorized(c)
		return
	}
	postID := c.Param("id")
	if postID == "" {
		response.BadRequest(c, "invalid post id")
		return
	}

	var req struct {
		Content         string `json:"content" binding:"required"`
		ClientRequestID string `json:"clientRequestId"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	caller, ok := h.socialCaller(c, userID)
	if !ok {
		return
	}

	resp, err := h.Social.CreateComment(c.Request.Context(), postID, caller, req.Content, "", "", req.ClientRequestID)
	if err != nil {
		response.GrpcErr(c, "social", err)
		return
	}
	if resp.Code != 0 {
		response.GrpcResult(c, resp.Code, resp.Message, nil)
		return
	}
	response.OK(c, toCommentData(h.MediaBaseURL, resp.Data))
}

// ReplyPostComment 回覆貼文留言
// @Summary      回覆貼文留言
// @Description  賣家回覆指定留言（僅貼文所屬賣家可操作）
// @Tags         留言 (Comment)
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id       path  string                                                                                  true  "被回覆的留言 ID"
// @Param        request  body  object{postId=string,content=string,replyToId=string,clientRequestId=string} true  "回覆內容；clientRequestId 為 UUID，用於 SSE event 回推時 dedup optimistic update。留言身分由伺服器依貼文歸屬推導（自己店的貼文是店家，其餘是本人）"
// @Success      200  {object}  object{code=int,data=object{id=string,postId=string,userName=string,avatarKey=string,avatarUrl=string,content=string,parentId=string,replyToId=string,replyToName=string,likeCount=int,replyCount=int,role=int,isMine=bool,isLiked=bool,createdAt=int}}
// @Failure      400  {object}  object{code=int,message=string}
// @Failure      401  {object}  object{code=int,message=string}
// @Failure      403  {object}  object{code=int,message=string}
// @Failure      404  {object}  object{code=int,message=string}
// @Failure      503  {object}  object{code=int,message=string}
// @Router       /app/posts/comments/{id}/replies [post]
func (h *Holder) ReplyPostComment(c *gin.Context) {
	userID, ok := getAccountID(c)
	if !ok {
		response.Unauthorized(c)
		return
	}
	commentID := c.Param("id")
	if commentID == "" {
		response.BadRequest(c, "invalid comment id")
		return
	}

	var req struct {
		Content         string `json:"content" binding:"required"`
		PostID          string `json:"postId" binding:"required"`
		ReplyToID       string `json:"replyToId"`
		ClientRequestID string `json:"clientRequestId"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	caller, ok := h.socialCaller(c, userID)
	if !ok {
		return
	}

	resp, err := h.Social.CreateComment(c.Request.Context(), req.PostID, caller, req.Content, commentID, req.ReplyToID, req.ClientRequestID)
	if err != nil {
		response.GrpcErr(c, "social", err)
		return
	}
	if resp.Code != 0 {
		response.GrpcResult(c, resp.Code, resp.Message, nil)
		return
	}
	response.OK(c, toCommentData(h.MediaBaseURL, resp.Data))
}

// UpdatePostComment 編輯留言
// @Summary      編輯留言
// @Description  編輯指定留言的內容，僅留言本人可操作
// @Tags         留言 (Comment)
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id       path  string                          true  "留言 ID"
// @Param        request  body  object{content=string}       true  "更新內容"
// @Success      200  {object}  object{code=int,data=object{id=string,postId=string,userName=string,avatarKey=string,avatarUrl=string,content=string,parentId=string,replyToId=string,replyToName=string,likeCount=int,replyCount=int,role=int,isMine=bool,isLiked=bool,createdAt=int}}
// @Failure      400  {object}  object{code=int,message=string}
// @Failure      401  {object}  object{code=int,message=string}
// @Failure      403  {object}  object{code=int,message=string}
// @Failure      404  {object}  object{code=int,message=string}
// @Failure      503  {object}  object{code=int,message=string}
// @Router       /app/posts/comments/{id} [put]
func (h *Holder) UpdatePostComment(c *gin.Context) {
	userID, ok := getAccountID(c)
	if !ok {
		response.Unauthorized(c)
		return
	}
	commentID := c.Param("id")
	if commentID == "" {
		response.BadRequest(c, "invalid comment id")
		return
	}

	var req struct {
		Content string `json:"content" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	caller, ok := h.socialCaller(c, userID)
	if !ok {
		return
	}

	resp, err := h.Social.UpdateComment(c.Request.Context(), commentID, caller, req.Content)
	if err != nil {
		response.GrpcErr(c, "social", err)
		return
	}
	if resp.Code != 0 {
		response.GrpcResult(c, resp.Code, resp.Message, nil)
		return
	}
	response.OK(c, toCommentData(h.MediaBaseURL, resp.Data))
}

// DeletePostComment 刪除留言
// @Summary      刪除留言
// @Description  刪除指定留言，留言本人或貼文賣家均可操作（軟刪除）
// @Tags         留言 (Comment)
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id  path  string  true  "留言 ID"
// @Success      200  {object}  object{code=int,message=string}
// @Failure      401  {object}  object{code=int,message=string}
// @Failure      403  {object}  object{code=int,message=string}
// @Failure      404  {object}  object{code=int,message=string}
// @Failure      503  {object}  object{code=int,message=string}
// @Router       /app/posts/comments/{id} [delete]
func (h *Holder) DeletePostComment(c *gin.Context) {
	userID, ok := getAccountID(c)
	if !ok {
		response.Unauthorized(c)
		return
	}
	commentID := c.Param("id")
	if commentID == "" {
		response.BadRequest(c, "invalid comment id")
		return
	}

	caller, ok := h.socialCaller(c, userID)
	if !ok {
		return
	}

	_, err := h.Social.DeleteComment(c.Request.Context(), commentID, caller)
	if err != nil {
		response.GrpcErr(c, "social", err)
		return
	}
	response.OK(c, nil)
}

// ToggleCommentLike 切換留言按讚
// @Summary      切換留言按讚
// @Description  對指定留言按讚或取消按讚（Toggle）
// @Tags         留言 (Comment)
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id  path  string  true  "留言 ID"
// @Success      200  {object}  object{code=int,data=object{liked=bool}}
// @Failure      401  {object}  object{code=int,message=string}
// @Failure      404  {object}  object{code=int,message=string}
// @Failure      503  {object}  object{code=int,message=string}
// @Router       /app/posts/comments/{id}/like [post]
func (h *Holder) ToggleCommentLike(c *gin.Context) {
	userID, ok := getAccountID(c)
	if !ok {
		response.Unauthorized(c)
		return
	}
	commentID := c.Param("id")
	if commentID == "" {
		response.BadRequest(c, "invalid comment id")
		return
	}

	caller, ok := h.socialCaller(c, userID)
	if !ok {
		return
	}

	resp, err := h.Social.ToggleCommentLike(c.Request.Context(), commentID, caller)
	if err != nil {
		response.GrpcErr(c, "social", err)
		return
	}
	if resp.Code != 0 {
		response.GrpcResult(c, resp.Code, resp.Message, nil)
		return
	}
	response.OK(c, gin.H{"liked": resp.Liked})
}

// ==================== DTOs ====================

// ReportPostRequest 舉報貼文請求
type ReportPostRequest struct {
	// 合法值：SPAM / SEXUAL / FRAUD / HARASSMENT / FOOD_SAFETY / POLITICAL / FAKE_NEWS / OTHER
	ReasonCode  string `json:"reasonCode" binding:"required"`
	Description string `json:"description"`
}

// MyPostListItemData 我的貼文列表項目（賣家自覽，含下架資訊）
type MyPostListItemData struct {
	ImageKeys     []string         `json:"imageKeys"` // 貼文圖 objectKey 陣列（49 §6.1）；無圖為 []
	Images        []string         `json:"imageUrls"` // 對應 imageKeys 的顯示 URL；無圖為 []
	Product       *PostProductData `json:"product"`
	SellerName    string           `json:"sellerName"`
	SellerLogoKey string           `json:"logoKey"` // 賣家 logo objectKey（49 §6.1）
	SellerLogoUrl string           `json:"logoUrl"` // 賣家 logo 顯示 URL（Gateway 組）
	Content       string           `json:"content"`
	HiddenReason  string           `json:"hiddenReason,omitempty"`
	// PostPublishStatus 碼值字串（CodeMeta；本檔不複製碼表）。omitempty 是刻意的：
	// 前端把「缺值」讀成「這條路徑的上游沒給」而不畫任何狀態標記，把它讀成「已發布」
	// 會是一個沒有根據的宣稱。上游恆帶值，所以實務上不會缺。
	PublishStatus string `json:"publishStatus,omitempty"`
	// ScheduledPublishAt 是代行者排定、尚未執行的發布時間（ISO 8601 UTC）；沒有排程時不輸出。
	// 排程不是發布狀態：貼文仍是未發布，到點才轉已發布。
	ScheduledPublishAt string   `json:"scheduledPublishAt,omitempty"`
	Tags               []string `json:"tags,omitempty"`
	ID                 string   `json:"id"`
	SellerID           string   `json:"sellerId"`
	CreatedAt          int64    `json:"createdAt"`
	HiddenAt           int64    `json:"hiddenAt,omitempty"`
	LikeCount          int32    `json:"likeCount"`
	CommentCount       int32    `json:"commentCount"`
	ViewCount          int32    `json:"viewCount"`
	ShareCount         int32    `json:"shareCount"`
}

// singleImageKeys 把單一 objectKey 包成陣列，跟 PostDetailData 的 ImageKeys shape 對齊。
//
// social 的 posts.image 是單一 TEXT 欄（DB 層就只存得下一張），所以列表與詳情的
// imageKeys 現階段恆為 0–1 element。列表曾同時吐 imageKey（單數）與 imageKeys
// （複數）兩組，前端編輯貼文時讀單數欄回送、複數欄那份就被當成「使用者刪掉了」
// 而清空 DB。單數欄已移除，複數欄是唯一形狀。
//
// 空字串回傳空陣列而非 nil：欄位無 omitempty，nil 會序列化成 null，前端得為
// null / [] 兩種空值各寫一次判斷。
func singleImageKeys(key string) []string {
	if key == "" {
		return []string{}
	}
	return []string{key}
}

func toMyPostListItems(base string, items []*pb.PostListItem) []MyPostListItemData {
	result := make([]MyPostListItemData, 0, len(items))
	for _, p := range items {
		var prod *PostProductData
		if p.Product != nil {
			prod = &PostProductData{
				ID:       p.Product.Id,
				Name:     p.Product.Name,
				ImageKey: p.Product.ImageKey,
				Image:    mediaurl.BuildURL(base, p.Product.ImageKey),
				Price:    p.Product.Price,
			}
		}
		imageKeys := singleImageKeys(p.ImageKey)
		result = append(result, MyPostListItemData{
			ID:            p.Id,
			ImageKeys:     imageKeys,
			Images:        buildURLsAlways(base, imageKeys),
			Content:       p.Content,
			SellerID:      p.SellerId,
			SellerName:    p.SellerName,
			SellerLogoKey: p.SellerLogoKey,
			SellerLogoUrl: mediaurl.BuildURL(base, p.SellerLogoKey),
			LikeCount:     p.LikeCount,
			CommentCount:  p.CommentCount,
			ViewCount:     p.ViewCount,
			ShareCount:    p.ShareCount,
			CreatedAt:     p.CreatedAt,
			HiddenAt:      p.HiddenAt,
			HiddenReason:  p.HiddenReason,
			PublishStatus: p.PublishStatus,
			Product:       prod,
			Tags:          p.Tags,

			ScheduledPublishAt: p.ScheduledPublishAt,
		})
	}
	return result
}

// AdminPostListItemData 管理後台貼文列表項目
type AdminPostListItemData struct {
	SellerName         string   `json:"sellerName"`
	Content            string   `json:"content"`
	Status             string   `json:"status"`
	HiddenReason       string   `json:"hiddenReason,omitempty"`
	LatestReportReason string   `json:"latestReportReason,omitempty"`
	ImageKeys          []string `json:"imageKeys"` // 貼文圖 objectKey 陣列（49 §6.1）；無圖為 []
	Images             []string `json:"imageUrls"` // 對應 imageKeys 的顯示 URL；無圖為 []
	PostID             string   `json:"postId"`
	StoreID            string   `json:"storeId"`
	CreatedAt          int64    `json:"createdAt"`
	LatestReportAt     int64    `json:"latestReportAt,omitempty"`
	ReportCount        int32    `json:"reportCount"`
}

func toAdminPostListItems(base string, items []*pb.AdminPostListItem) []AdminPostListItemData {
	result := make([]AdminPostListItemData, 0, len(items))
	for _, p := range items {
		result = append(result, AdminPostListItemData{
			PostID:             p.PostId,
			StoreID:            p.StoreId,
			SellerName:         p.SellerName,
			Content:            p.Content,
			ImageKeys:          emptyIfNil(p.ImageKeys),
			Images:             buildURLsAlways(base, p.ImageKeys),
			ReportCount:        p.ReportCount,
			LatestReportReason: p.LatestReportReason,
			LatestReportAt:     p.LatestReportAt,
			Status:             p.Status,
			CreatedAt:          p.CreatedAt,
			HiddenReason:       p.HiddenReason,
		})
	}
	return result
}

// AdminModeratePostRequest 審核貼文請求
type AdminModeratePostRequest struct {
	// 合法值：HIDE / RESTORE
	Action string `json:"action" binding:"required"`
	Reason string `json:"reason"`
	// HIDE 時的分類，合法值：SPAM / SEXUAL / FRAUD / HARASSMENT / FOOD_SAFETY / POLITICAL / FAKE_NEWS / OTHER
	// 選填（RESTORE 不需要）；空/非法值 social 端 fallback OTHER
	ReasonCode string `json:"reasonCode"`
}

// ==================== Internal helpers ====================

func parseIntQuery(c *gin.Context, key string, defaultVal int) int {
	s := c.Query(key)
	if s == "" {
		return defaultVal
	}
	v, err := strconv.Atoi(s)
	if err != nil || v <= 0 {
		return defaultVal
	}
	return v
}

// maxPostPageSize 限制單次貼文列表查詢筆數上限，避免 client 端一次請求整批
// 貼文觸發後端 per-item enrichment（product-service 批次查詢）成本失控。
const maxPostPageSize = 50

// parsePagePageSize parses the page/page_size query params with the same
// bounds pagination.NormalizePage applies: page<=0→1, pageSize<=0→defaultSize,
// pageSize>maxSize→maxSize. offset is discarded — this layer forwards
// page/pageSize downstream via gRPC; the offset (if any) is computed by the
// service that owns the query.
func parsePagePageSize(c *gin.Context, defaultSize, maxSize int32) (page, pageSize int32) {
	rawPage := parseIntQuery(c, "page", 0)
	rawPageSize := parseIntQuery(c, "page_size", 0)
	page, pageSize, _ = pagination.NormalizePage(int32(rawPage), int32(rawPageSize), defaultSize, maxSize)
	return page, pageSize
}

// PostListItemData 貼文列表項目
type PostListItemData struct {
	ImageKeys     []string         `json:"imageKeys"` // 貼文圖 objectKey 陣列（49 §6.1）；無圖為 []
	Images        []string         `json:"imageUrls"` // 對應 imageKeys 的顯示 URL；無圖為 []
	Product       *PostProductData `json:"product"`
	SellerName    string           `json:"sellerName"`
	SellerLogoKey string           `json:"logoKey"` // 賣家 logo objectKey（49 §6.1）
	SellerLogoUrl string           `json:"logoUrl"` // 賣家 logo 顯示 URL（Gateway 組）
	Content       string           `json:"content"`
	Tags          []string         `json:"tags,omitempty"`
	ID            string           `json:"id"`
	SellerID      string           `json:"sellerId"`
	// StoreCode 是賣家店面的 canonical 對外顯示碼，前端貼文卡片的「查看商家」
	// 連結一律用它組 /sellers/{store_code}。值由 enrichPostsWithSellerInfo 從
	// 既有的 GetSellerBatch 回應回填——social-service 只存 seller_id，沒有
	// store_code，但那支 batch RPC 為了店名/logo 本來就會呼叫，順手帶回來零成本。
	// 空字串代表該賣家在 seller 模組查不到（已關店/批次逾時），此時前端
	// 必須不渲染賣家連結，而不是退回 seller_id——store_id 定址已不再受支援。
	StoreCode    string `json:"storeCode"`
	CreatedAt    int64  `json:"createdAt"`
	LikeCount    int32  `json:"likeCount"`
	CommentCount int32  `json:"commentCount"`
	ViewCount    int32  `json:"viewCount"`
	ShareCount   int32  `json:"shareCount"`
	IsLiked      bool   `json:"isLiked"`
}

func toPostListItems(base string, items []*pb.PostListItem) []PostListItemData {
	result := make([]PostListItemData, 0, len(items))
	for _, p := range items {
		var prod *PostProductData
		if p.Product != nil {
			prod = &PostProductData{
				ID:       p.Product.Id,
				Name:     p.Product.Name,
				ImageKey: p.Product.ImageKey,
				Image:    mediaurl.BuildURL(base, p.Product.ImageKey),
				Price:    p.Product.Price,
			}
		}
		imageKeys := singleImageKeys(p.ImageKey)
		result = append(result, PostListItemData{
			ID:            p.Id,
			ImageKeys:     imageKeys,
			Images:        buildURLsAlways(base, imageKeys),
			Content:       p.Content,
			SellerID:      p.SellerId,
			SellerName:    p.SellerName,
			SellerLogoKey: p.SellerLogoKey,
			SellerLogoUrl: mediaurl.BuildURL(base, p.SellerLogoKey),
			LikeCount:     p.LikeCount,
			CommentCount:  p.CommentCount,
			ViewCount:     p.ViewCount,
			ShareCount:    p.ShareCount,
			IsLiked:       p.IsLiked,
			CreatedAt:     p.CreatedAt,
			Product:       prod,
			Tags:          p.Tags,
		})
	}
	return result
}

// PostProductData 貼文關聯商品
type PostProductData struct {
	Name string `json:"name"`
	// imageKey = 商品圖 objectKey（proto 值，S9 cascade 自 product）；imageUrl = Gateway 組的顯示 URL（49 §6.1）。
	ImageKey string `json:"imageKey"`
	Image    string `json:"imageUrl"`
	Price    string `json:"price"`
	ID       string `json:"id"`
}

// PostDetailData 貼文詳情
type PostDetailData struct {
	Products      []PostProductData `json:"products,omitempty"`
	SellerName    string            `json:"sellerName"`
	SellerLogoKey string            `json:"logoKey"` // 賣家 logo objectKey（49 §6.1）
	SellerLogoUrl string            `json:"logoUrl"` // 賣家 logo 顯示 URL（Gateway 組）
	Content       string            `json:"content"`
	ImageKeys     []string          `json:"imageKeys"` // 貼文圖 objectKey 陣列（49 §6.1）；無圖為 []
	Images        []string          `json:"imageUrls"` // 對應 imageKeys 的顯示 URL；無圖為 []
	Tags          []string          `json:"tags,omitempty"`
	ID            string            `json:"id"`
	SellerID      string            `json:"sellerId"`
	CreatedAt     int64             `json:"createdAt"`
	ShareCount    int32             `json:"shareCount"`
	ViewCount     int32             `json:"viewCount"`
	LikeCount     int32             `json:"likeCount"`
	CommentCount  int32             `json:"commentCount"`
	IsLiked       bool              `json:"isLiked"`
}

func toPostDetailData(base string, p *pb.PostDetailInfo) *PostDetailData {
	if p == nil {
		return nil
	}
	products := make([]PostProductData, 0, len(p.Products))
	for _, prod := range p.Products {
		products = append(products, PostProductData{
			ID:       prod.Id,
			Name:     prod.Name,
			ImageKey: prod.ImageKey,
			Image:    mediaurl.BuildURL(base, prod.ImageKey),
			Price:    prod.Price,
		})
	}
	return &PostDetailData{
		ID:            p.Id,
		Content:       p.Content,
		SellerID:      p.SellerId,
		SellerName:    p.SellerName,
		SellerLogoKey: p.SellerLogoKey,
		SellerLogoUrl: mediaurl.BuildURL(base, p.SellerLogoKey),
		ImageKeys:     emptyIfNil(p.ImageKeys),
		Images:        buildURLsAlways(base, p.ImageKeys),
		Products:      products,
		LikeCount:     p.LikeCount,
		CommentCount:  p.CommentCount,
		ViewCount:     p.ViewCount,
		ShareCount:    p.ShareCount,
		IsLiked:       p.IsLiked,
		CreatedAt:     p.CreatedAt,
		Tags:          p.Tags,
	}
}

// MyPostDetailData 店主讀自己的單篇貼文：公開詳情的欄位，加上店主才看得到的發布狀態與平台下架資訊
// （與 MyPostListItemData 同名同值域）。
type MyPostDetailData struct {
	PostDetailData
	PublishStatus string `json:"publishStatus,omitempty"`
	HiddenReason  string `json:"hiddenReason,omitempty"`
	HiddenAt      int64  `json:"hiddenAt,omitempty"`
}

func toMyPostDetailData(base string, p *pb.PostDetailInfo) (*MyPostDetailData, error) {
	if p == nil {
		return nil, nil
	}
	out := &MyPostDetailData{
		PostDetailData: *toPostDetailData(base, p),
		PublishStatus:  p.PublishStatus,
		HiddenReason:   p.HiddenReason,
	}
	if p.HiddenAt != "" {
		t, err := timeutil.ParseISO8601(p.HiddenAt)
		if err != nil {
			return nil, err
		}
		out.HiddenAt = t.Unix()
	}
	return out, nil
}

// commentData 留言項目（確保 is_liked 等零值欄位在 JSON 中出現）
type commentData struct {
	UserName string `json:"userName"`
	// 49 §6.1 雙欄位：avatarKey = objectKey（DB 值，可能為 Google 外部 URL）；
	// avatarUrl = Gateway 組的顯示 URL（BuildURL 對 http(s) 存量直接 passthrough）。
	UserAvatarKey string         `json:"avatarKey"`
	UserAvatar    string         `json:"avatarUrl"`
	Content       string         `json:"content"`
	ReplyToName   string         `json:"replyToName,omitempty"`
	Replies       []*commentData `json:"replies,omitempty"`
	// id 皆為 64-bit TSID（> JS 2^53），proto 已將其定義為 string（§6 Track 1 B5），
	// 故這些欄位為原生 string，避免 web JSON.parse 精度丟失，並與 SSE comment.created
	// （CommentSnapshot 已字串）型別一致（對齊 notification #960）。
	// created_at 是毫秒 timestamp（< 2^53）維持 number。
	ID         string `json:"id"`
	PostID     string `json:"postId"`
	ParentID   string `json:"parentId,omitempty"`
	ReplyToID  string `json:"replyToId,omitempty"`
	CreatedAt  int64  `json:"createdAt"`
	LikeCount  int32  `json:"likeCount"`
	ReplyCount int32  `json:"replyCount,omitempty"`
	Role       int32  `json:"role"`
	IsLiked    bool   `json:"isLiked"`
	IsMine     bool   `json:"isMine"`
}

func toCommentList(base string, items []*pb.CommentInfo) []commentData {
	result := make([]commentData, 0, len(items))
	for _, c := range items {
		result = append(result, toCommentData(base, c))
	}
	return result
}

func toCommentData(base string, c *pb.CommentInfo) commentData {
	var replies []*commentData
	if len(c.Replies) > 0 {
		replies = make([]*commentData, 0, len(c.Replies))
		for _, r := range c.Replies {
			rd := toCommentData(base, r)
			replies = append(replies, &rd)
		}
	}
	return commentData{
		ID:            c.Id,
		PostID:        c.PostId,
		IsMine:        c.IsMine,
		UserName:      c.UserName,
		UserAvatarKey: c.UserAvatarKey,
		UserAvatar:    mediaurl.BuildURL(base, c.UserAvatarKey),
		Content:       c.Content,
		ParentID:      c.ParentId,
		ReplyToID:     c.ReplyToId,
		ReplyToName:   c.ReplyToName,
		LikeCount:     c.LikeCount,
		ReplyCount:    c.ReplyCount,
		Role:          c.ActorKind,
		IsLiked:       c.IsLiked,
		CreatedAt:     c.CreatedAt,
		Replies:       replies,
	}
}

// enrichPostsWithSellerInfo 用 seller 模組的店名/logo 覆蓋貼文的賣家資訊。
// social-service 從 identity 模組取 nickname，但賣家的顯示名稱應該用 seller 模組的 store_name。
func (h *Holder) enrichPostsWithSellerInfo(ctx context.Context, posts []PostListItemData) {
	seen := make(map[int64]struct{})
	var ids []int64
	for _, p := range posts {
		sellerID, err := strconv.ParseInt(p.SellerID, 10, 64)
		if err != nil || sellerID == 0 {
			continue
		}
		if _, ok := seen[sellerID]; !ok {
			seen[sellerID] = struct{}{}
			ids = append(ids, sellerID)
		}
	}
	if len(ids) == 0 {
		return
	}

	batchCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	resp, err := h.Seller.GetSellerBatch(batchCtx, ids)
	if err != nil || resp == nil || resp.GetCode() != 0 {
		return
	}

	type sellerInfoView struct {
		name      string
		logoKey   string
		storeCode string
	}
	sellerMap := make(map[int64]sellerInfoView, len(resp.Data))
	for _, s := range resp.Data {
		storeID, parseErr := strconv.ParseInt(s.GetStoreId(), 10, 64)
		if parseErr != nil || storeID <= 0 {
			continue
		}
		sellerMap[storeID] = sellerInfoView{name: s.Name, logoKey: s.LogoKey, storeCode: s.StoreCode}
	}

	for i := range posts {
		sellerID, err := strconv.ParseInt(posts[i].SellerID, 10, 64)
		if err != nil {
			continue
		}
		if info, ok := sellerMap[sellerID]; ok {
			if info.name != "" {
				posts[i].SellerName = info.name
			}
			// 49 §6.1 GW組：seller 回 objectKey，logoKey 與 logoUrl 成對回填，
			// 避免快照 key 空時 URL 有值、key 空的不對稱。
			if info.logoKey != "" {
				posts[i].SellerLogoKey = info.logoKey
				posts[i].SellerLogoUrl = buildURL(h.MediaBaseURL, info.logoKey)
			}
			// canonical 店面定址碼（#2098 階段三）。social-service 不存 store_code，
			// 這支 batch RPC 本來就為了店名/logo 呼叫，順手帶回來零額外成本。
			posts[i].StoreCode = info.storeCode
		}
	}
}
