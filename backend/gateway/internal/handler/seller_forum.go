package handler

import (
	"strconv"

	"github.com/Will413028/hawkr/gateway/internal/response"
	"github.com/Will413028/hawkr/go-pkg/mediaurl"
	pb "github.com/Will413028/hawkr/go-pkg/proto/social"
	"github.com/gin-gonic/gin"
)

// ForumRecord 討論區留言項目
// 欄位保留 snake_case 與系統其他 proto-based 回應一致，避免破壞既有 Flutter app 契約。
type ForumRecord struct {
	UserName string `json:"userName"`
	// 49 §6.1 雙欄位：avatarKey = objectKey（DB 值，可能為 Google 外部 URL）；
	// avatarUrl = Gateway 組的顯示 URL（BuildURL 對 http(s) 存量直接 passthrough）。
	UserAvatarKey string `json:"avatarKey"`
	UserAvatar    string `json:"avatarUrl"`
	Content       string `json:"content"`
	ReplyToName   string `json:"replyToName,omitempty"`
	// id/seller_id/parent_id/reply_to_id 是 64-bit TSID（> JS 2^53）：proto 已
	// 將其定義為 string（§6 Track 1 B5），故為原生 string 序列化避免 web JSON.parse 精度
	// 丟失（對齊 notification #960、與留言 commentData 一致）。created_at 是毫秒 timestamp
	// （< 2^53）維持 number。
	ID         string `json:"id"`
	SellerID   string `json:"sellerId"`
	ParentID   string `json:"parentId,omitempty"`
	ReplyToID  string `json:"replyToId,omitempty"`
	CreatedAt  int64  `json:"createdAt"`
	ReplyCount int32  `json:"replyCount"`
	IsMine     bool   `json:"isMine"`
}

// toForumRecord 是讀寫共用的轉換點。寫入端（create/reply/update）以前直接
// response.OK(c, resp.Data) 吐 proto，於是同一筆留言在建立時叫 user_avatar_key、
// 在列表時叫 avatarKey——同資源兩種形狀（49 §6.6 R5）。
func toForumRecord(base string, e *pb.ForumEntryInfo) ForumRecord {
	return ForumRecord{
		ID:            e.GetId(),
		SellerID:      e.GetSellerId(),
		IsMine:        e.GetIsMine(),
		UserName:      e.GetUserName(),
		UserAvatarKey: e.GetUserAvatarKey(),
		UserAvatar:    mediaurl.BuildURL(base, e.GetUserAvatarKey()),
		Content:       e.GetContent(),
		ParentID:      e.GetParentId(),
		ReplyToID:     e.GetReplyToId(),
		ReplyToName:   e.GetReplyToName(),
		ReplyCount:    e.GetReplyCount(),
		CreatedAt:     e.GetCreatedAt(),
	}
}

func toForumRecordList(base string, items []*pb.ForumEntryInfo) []ForumRecord {
	result := make([]ForumRecord, 0, len(items))
	for _, e := range items {
		result = append(result, toForumRecord(base, e))
	}
	return result
}

// GetSellerForum 取得賣家討論區列表
// @Summary      取得賣家討論區列表
// @Description  公開取得指定賣家的討論區頂層留言，支援分頁
// @Tags         討論區 (Forum)
// @Accept       json
// @Produce      json
// @Param        id         path   string  true   "賣家 ID"
// @Param        page       query  int  false  "頁碼"      default(1)
// @Param        page_size  query  int  false  "每頁筆數"  default(10)
// @Success      200  {object}  object{code=int,data=object{records=[]ForumRecord,total=string,page=string,pageSize=string}}
// @Failure      400  {object}  object{code=int,message=string}
// @Failure      503  {object}  object{code=int,message=string}
// @Router       /public/sellers/{id}/forum [get]
func (h *Holder) GetSellerForum(c *gin.Context) {
	sellerID := c.Param("id")
	if sellerID == "" {
		response.BadRequest(c, "invalid seller id")
		return
	}
	page, pageSize := parsePagePageSize(c, defaultPageSize, maxPageSize)

	caller, ok := h.socialCaller(c, getOptionalAccountID(c))
	if !ok {
		return
	}

	resp, err := h.Social.GetSellerForum(c.Request.Context(), sellerID, caller, page, pageSize)
	if err != nil {
		response.GrpcErr(c, "social", err)
		return
	}
	if resp.Code != 0 {
		response.GrpcResult(c, resp.Code, resp.Message, nil)
		return
	}
	response.Page(c, toForumRecordList(h.MediaBaseURL, resp.Data), strconv.FormatInt(resp.Total, 10), strconv.Itoa(int(resp.Page)), strconv.Itoa(int(resp.PageSize)))
}

// GetForumReplies 取得討論區回覆列表
// @Summary      取得討論區回覆列表
// @Description  公開取得指定討論區留言的所有回覆，支援分頁
// @Tags         討論區 (Forum)
// @Accept       json
// @Produce      json
// @Param        id         path   string  true   "討論區留言 ID"
// @Param        page       query  int  false  "頁碼"           default(1)
// @Param        page_size  query  int  false  "每頁筆數"       default(10)
// @Success      200  {object}  object{code=int,data=object{records=[]ForumRecord,total=string,page=string,pageSize=string}}
// @Failure      400  {object}  object{code=int,message=string}
// @Failure      503  {object}  object{code=int,message=string}
// @Router       /public/sellers/forum/{id}/replies [get]
func (h *Holder) GetForumReplies(c *gin.Context) {
	parentID := c.Param("id")
	if parentID == "" {
		response.BadRequest(c, "invalid forum entry id")
		return
	}
	page, pageSize := parsePagePageSize(c, defaultPageSize, maxPageSize)

	caller, ok := h.socialCaller(c, getOptionalAccountID(c))
	if !ok {
		return
	}

	resp, err := h.Social.GetForumReplies(c.Request.Context(), parentID, caller, page, pageSize)
	if err != nil {
		response.GrpcErr(c, "social", err)
		return
	}
	if resp.Code != 0 {
		response.GrpcResult(c, resp.Code, resp.Message, nil)
		return
	}
	response.Page(c, toForumRecordList(h.MediaBaseURL, resp.Data), strconv.FormatInt(resp.Total, 10), strconv.Itoa(int(resp.Page)), strconv.Itoa(int(resp.PageSize)))
}

// CreateForumComment 在賣家討論區新增留言
// @Summary      在賣家討論區新增留言
// @Description  任何已登入用戶均可在指定賣家的討論區留言
// @Tags         討論區 (Forum)
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id       path  string                        true  "賣家 ID"
// @Param        request  body  object{content=string}     true  "留言內容"
// @Success      200  {object}  object{code=int,data=ForumRecord}
// @Failure      400  {object}  object{code=int,message=string}
// @Failure      401  {object}  object{code=int,message=string}
// @Failure      503  {object}  object{code=int,message=string}
// @Router       /app/sellers/{id}/forum [post]
func (h *Holder) CreateForumComment(c *gin.Context) {
	userID, ok := getAccountID(c)
	if !ok {
		response.Unauthorized(c)
		return
	}
	sellerID := c.Param("id")
	if sellerID == "" {
		response.BadRequest(c, "invalid seller id")
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

	resp, err := h.Social.CreateForumComment(c.Request.Context(), sellerID, caller, req.Content)
	if err != nil {
		response.GrpcErr(c, "social", err)
		return
	}
	if resp.Code != 0 {
		response.GrpcResult(c, resp.Code, resp.Message, nil)
		return
	}
	response.OK(c, toForumRecord(h.MediaBaseURL, resp.Data))
}

// ReplyForumComment 回覆討論區留言
// @Summary      回覆討論區留言
// @Description  任何已登入用戶均可回覆指定討論區留言，最多兩層（回覆的回覆自動歸到頂層）
// @Tags         討論區 (Forum)
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id       path  string                      true  "被回覆的討論區留言 ID"
// @Param        request  body  object{content=string}   true  "回覆內容"
// @Success      200  {object}  object{code=int,data=ForumRecord}
// @Failure      400  {object}  object{code=int,message=string}
// @Failure      401  {object}  object{code=int,message=string}
// @Failure      404  {object}  object{code=int,message=string}
// @Failure      503  {object}  object{code=int,message=string}
// @Router       /app/sellers/forum/{id}/reply [post]
func (h *Holder) ReplyForumComment(c *gin.Context) {
	userID, ok := getAccountID(c)
	if !ok {
		response.Unauthorized(c)
		return
	}
	parentID := c.Param("id")
	if parentID == "" {
		response.BadRequest(c, "invalid forum entry id")
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

	resp, err := h.Social.ReplyForumComment(c.Request.Context(), parentID, caller, req.Content)
	if err != nil {
		response.GrpcErr(c, "social", err)
		return
	}
	if resp.Code != 0 {
		response.GrpcResult(c, resp.Code, resp.Message, nil)
		return
	}
	response.OK(c, toForumRecord(h.MediaBaseURL, resp.Data))
}

// UpdateForumComment 編輯討論區留言
// @Summary      編輯討論區留言
// @Description  留言本人可編輯自己的討論區留言內容
// @Tags         討論區 (Forum)
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id       path  string                      true  "討論區留言 ID"
// @Param        request  body  object{content=string}   true  "留言內容"
// @Success      200  {object}  object{code=int,data=ForumRecord}
// @Failure      400  {object}  object{code=int,message=string}
// @Failure      401  {object}  object{code=int,message=string}
// @Failure      403  {object}  object{code=int,message=string}
// @Failure      404  {object}  object{code=int,message=string}
// @Failure      503  {object}  object{code=int,message=string}
// @Router       /app/sellers/forum/{id} [put]
func (h *Holder) UpdateForumComment(c *gin.Context) {
	userID, ok := getAccountID(c)
	if !ok {
		response.Unauthorized(c)
		return
	}
	entryID := c.Param("id")
	if entryID == "" {
		response.BadRequest(c, "invalid forum entry id")
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

	resp, err := h.Social.UpdateForumComment(c.Request.Context(), entryID, caller, req.Content)
	if err != nil {
		response.GrpcErr(c, "social", err)
		return
	}
	if resp.Code != 0 {
		response.GrpcResult(c, resp.Code, resp.Message, nil)
		return
	}
	response.OK(c, toForumRecord(h.MediaBaseURL, resp.Data))
}

// DeleteForumComment 刪除討論區留言
// @Summary      刪除討論區留言
// @Description  留言本人或賣家均可刪除討論區留言（軟刪除）
// @Tags         討論區 (Forum)
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id  path  string  true  "討論區留言 ID"
// @Success      200  {object}  object{code=int,message=string}
// @Failure      401  {object}  object{code=int,message=string}
// @Failure      403  {object}  object{code=int,message=string}
// @Failure      404  {object}  object{code=int,message=string}
// @Failure      503  {object}  object{code=int,message=string}
// @Router       /app/sellers/forum/{id} [delete]
func (h *Holder) DeleteForumComment(c *gin.Context) {
	userID, ok := getAccountID(c)
	if !ok {
		response.Unauthorized(c)
		return
	}
	entryID := c.Param("id")
	if entryID == "" {
		response.BadRequest(c, "invalid forum entry id")
		return
	}

	caller, ok := h.socialCaller(c, userID)
	if !ok {
		return
	}

	_, err := h.Social.DeleteForumComment(c.Request.Context(), entryID, caller)
	if err != nil {
		response.GrpcErr(c, "social", err)
		return
	}
	response.OK(c, nil)
}
