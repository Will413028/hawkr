package handler

import (
	"context"
	"strconv"
	"strings"
	"time"

	"github.com/Will413028/hawkr/gateway/internal/authz"
	"github.com/Will413028/hawkr/gateway/internal/handler/delegation"
	"github.com/Will413028/hawkr/gateway/internal/response"
	"github.com/Will413028/hawkr/go-pkg/codemeta"
	"github.com/Will413028/hawkr/go-pkg/errors/bizcode"
	commonpb "github.com/Will413028/hawkr/go-pkg/proto/common"
	socialpb "github.com/Will413028/hawkr/go-pkg/proto/social"
	"github.com/Will413028/hawkr/go-pkg/timeutil"
	"github.com/gin-gonic/gin"
)

// ==================== DTO ====================

// DelegatedPostListItem is one row of GET .../posts.
//
// 明確不含 likeCount／commentCount／viewCount／shareCount／isLiked：allow-list 逐項
// 列的是「貼文內容、圖片 objectKey、標籤、關聯商品 ID、發布狀態、排程時間、version」，
// 互動計數不在其中（deny-by-default）。也不含 sellerId／sellerName／logoKey——代行
// 工作區的 target store 是固定的，逐列重覆賣家識別資訊沒有消費者。
//
// ⚠️ 沒有 version：上游 ListMyPosts 的 PostListItem 不帶它（只有 GetPostDetail 的
// PostDetailInfo 有）。與其填一個空字串讓前端拿去當 expectedVersion，不如不宣告——
// 前端要編輯就先打詳情。
//
// publishStatus 則相反，必須帶：少了它，前端分不出一則貼文是草稿還是已發布 —— 兩者
// 在列表上長得一樣（草稿沒有任何狀態標記），而代行工作區的管理員正是靠列表判斷這家
// 店的經營狀態。碼與標籤都給，與詳情頁同口徑（前端不必自己維護第二份對照表）。
type DelegatedPostListItem struct {
	PostID             string   `json:"postId"`
	Content            string   `json:"content"`
	ImageKeys          []string `json:"imageKeys"`
	Tags               []string `json:"tags"`
	PublishStatus      string   `json:"publishStatus"`
	PublishStatusLabel string   `json:"publishStatusLabel"`
	CreatedAt          string   `json:"createdAt"`
	HiddenAt           *string  `json:"hiddenAt"`
	HiddenReason       string   `json:"hiddenReason"`
}

// DelegatedPostProduct is one element of the detail DTO's products[].
type DelegatedPostProduct struct {
	ProductID string `json:"productId"`
	Name      string `json:"name"`
	ImageKey  string `json:"imageKey"`
	Price     string `json:"price"`
}

// DelegatedPostDetail is GET .../posts/{postId}.
type DelegatedPostDetail struct {
	PostID             string                 `json:"postId"`
	Version            string                 `json:"version"`
	Content            string                 `json:"content"`
	ImageKeys          []string               `json:"imageKeys"`
	Tags               []string               `json:"tags"`
	PublishStatus      string                 `json:"publishStatus"`
	PublishStatusLabel string                 `json:"publishStatusLabel"`
	ScheduledPublishAt *string                `json:"scheduledPublishAt"`
	PublishedAt        *string                `json:"publishedAt"`
	CreatedAt          string                 `json:"createdAt"`
	HiddenAt           *string                `json:"hiddenAt"`
	HiddenReason       string                 `json:"hiddenReason"`
	Products           []DelegatedPostProduct `json:"products"`
}

// DelegatedPostCreateRequest is POST .../posts.
//
// 不接受 publishMode／scheduledPublishAt：建立一律是草稿。「建立即發布」是前端連續
// 呼叫兩支（POST .../posts 拿 postId，再 POST .../posts/{postId}/publish）。用 body
// 值決定要不要再驗 posts.publish，會讓 capability 變成 handler 內的條件分支——boot
// 斷言掃得到路由宣告的那一個 capability，掃不到藏在 if 裡的第二個。
type DelegatedPostCreateRequest struct {
	OperationReason string   `json:"operationReason"`
	NotifySeller    *bool    `json:"notifySeller"`
	Content         string   `json:"content"`
	StagedTokens    []string `json:"stagedTokens"`
	Tags            []string `json:"tags"`
	ProductIDs      []string `json:"productIds"`
}

// DelegatedPostUpdateRequest is PATCH .../posts/{postId}.
type DelegatedPostUpdateRequest struct {
	ExpectedVersion string   `json:"expectedVersion"`
	OperationReason string   `json:"operationReason"`
	NotifySeller    *bool    `json:"notifySeller"`
	Content         *string  `json:"content"`
	StagedTokens    []string `json:"stagedTokens"`
	Tags            []string `json:"tags"`
	ProductIDs      []string `json:"productIds"`
}

// DelegatedPostPublishRequest is POST .../posts/{postId}/publish.
type DelegatedPostPublishRequest struct {
	ExpectedVersion    string `json:"expectedVersion"`
	OperationReason    string `json:"operationReason"`
	NotifySeller       *bool  `json:"notifySeller"`
	PublishMode        string `json:"publishMode"`
	ScheduledPublishAt string `json:"scheduledPublishAt"`
}

// DelegatedPostUnpublishRequest is POST .../posts/{postId}/unpublish. It deliberately
// has no publishMode / scheduledPublishAt: unknown keys are rejected, so sending them
// is 9260 rather than a silently ignored schedule.
type DelegatedPostUnpublishRequest struct {
	ExpectedVersion string `json:"expectedVersion"`
	OperationReason string `json:"operationReason"`
	NotifySeller    *bool  `json:"notifySeller"`
}

// 排程邊界。下界 now()+5min：小於它的排程與立即發布沒有可觀察差別，卻要多付一整條
// 排程路徑的失敗面，同時它讓「排程建立後還來得及撤回」真的成立。上界 now()+90d：
// 排程列會佔著 pending 直到執行，而工作階段最長 30 分鐘。
//
// 只擋過去時間是不夠的：t <= now() 等於接受「1 秒後」與「10 年後」——前者製造與立即
// 發布競態的排程列、後者製造永久待執行列，兩者都不報錯，只會在很久以後才被發現。
const (
	delegatedScheduleMinLead = 5 * time.Minute
	delegatedScheduleMaxLead = 90 * 24 * time.Hour
)

// ==================== handlers ====================

// ListDelegatedPosts handles GET .../posts.
func (h *Holder) ListDelegatedPosts(c *gin.Context) {
	req, ok := resolveDelegatedRequest(c, false)
	if !ok {
		return
	}
	page, pageSize, ok := delegatedPaging(c)
	if !ok {
		return
	}
	if h.Social == nil {
		denyDelegated(c, upstreamDenial())
		return
	}

	orch := h.orchestrator()
	resp, err := h.Social.ListMyPostsForDelegation(c.Request.Context(), &socialpb.ListMyPostsRequest{
		SellerId:   req.StoreID,
		Status:     c.Query("status"),
		Page:       page,
		PageSize:   pageSize,
		Delegation: orch.Context(delegation.Spec{Session: req.Session}, authz.AdminCapPostsRead, ""),
	}, h.Delegation.GrpcDeadlineDomainReadMs)
	if err != nil {
		denyDelegated(c, denialFromGrpcErr(err))
		return
	}
	if resp.GetCode() != 0 {
		if resp.GetCode() == bizcode.DelegationResourceNotOwned {
			orch.RecordReadDenial(c.Request.Context(), req.readDenial("post", "", resp.GetCode()))
		}
		denyDelegated(c, delegationDenialFromCode(resp.GetCode(), resp.GetMessage()))
		return
	}

	records := make([]DelegatedPostListItem, 0, len(resp.GetData()))
	for _, p := range resp.GetData() {
		item := DelegatedPostListItem{
			PostID:             p.GetId(),
			Content:            p.GetContent(),
			ImageKeys:          nonNilStrings(singleKey(p.GetImageKey())),
			Tags:               nonNilStrings(p.GetTags()),
			PublishStatus:      p.GetPublishStatus(),
			PublishStatusLabel: codemeta.PostPublishStatusLabel(p.GetPublishStatus()),
			CreatedAt:          epochToISO8601(p.GetCreatedAt()),
			HiddenReason:       p.GetHiddenReason(),
		}
		if p.GetHiddenAt() != 0 {
			at := epochToISO8601(p.GetHiddenAt())
			item.HiddenAt = &at
		}
		records = append(records, item)
	}
	response.Page(c, records,
		strconv.FormatInt(resp.GetTotal(), 10),
		strconv.FormatInt(int64(resp.GetPage()), 10),
		strconv.FormatInt(int64(resp.GetPageSize()), 10))
}

// GetDelegatedPost handles GET .../posts/{postId}.
func (h *Holder) GetDelegatedPost(c *gin.Context) {
	req, ok := resolveDelegatedRequest(c, false)
	if !ok {
		return
	}
	postID := c.Param("post_id")
	orch := h.orchestrator()
	res, denial := h.readPost(c.Request.Context(), req.StoreID, postID,
		orch.Context(delegation.Spec{Session: req.Session}, authz.AdminCapPostsRead, ""))
	if denial != nil {
		if denial.Code == bizcode.DelegationResourceNotOwned {
			orch.RecordReadDenial(c.Request.Context(), req.readDenial("post", postID, denial.Code))
		}
		denyDelegated(c, denial)
		return
	}
	response.OK(c, res.Payload)
}

// CreateDelegatedPost handles POST .../posts.
func (h *Holder) CreateDelegatedPost(c *gin.Context) {
	req, ok := resolveDelegatedRequest(c, true)
	if !ok {
		return
	}
	var body DelegatedPostCreateRequest
	payloadHash, ok := bindDelegatedBody(c, &body)
	if !ok {
		return
	}
	reason, ok := validateOperationReason(c, body.OperationReason)
	if !ok {
		return
	}
	if !validatePostContent(c, &body.Content, true) ||
		!validatePostStagedTokens(c, body.StagedTokens) ||
		!validatePostTags(c, body.Tags) ||
		!validatePostProductIDs(c, body.ProductIDs) {
		return
	}

	orch := h.orchestrator()
	result, denial := orch.Orchestrate(c.Request.Context(), delegation.Spec{
		Session:      req.Session,
		Action:       "POST_CREATE",
		ResourceType: "post",
		ReadCap:      authz.AdminCapPostsRead,
		WriteCap:     authz.AdminCapPostsWrite,
		// 全表比較：建立類沒有 before，理由同 PRODUCT_CREATE。
		SnapshotFields: nil,
		// 建立類沒有 pre-image。貼文沒有名稱欄位，所以顯示名沿用 postResource 的
		// 同一條規則（內容前 60 字）—— 兩邊必須同規則，否則同一篇貼文的建立列與
		// 之後的編輯列在稽核主控台上看起來是兩個不同的資源。
		ResourceName:    truncateRunes(body.Content, 60),
		OperationReason: reason,
		NotifySeller:    notifySellerOrDefault(body.NotifySeller),
		IdempotencyKey:  req.IdempotencyKey,
		PayloadHash:     payloadHash,
		RequestID:       req.RequestID,
		CorrelationID:   req.CorrelationID,
		UserAgent:       req.UserAgent,
		Write: func(ctx context.Context, dctx *commonpb.DelegationContext) (*delegation.Resource, *delegation.Denial) {
			resp, err := h.Social.CreatePostForDelegation(ctx, &socialpb.CreatePostRequest{
				SellerId:       req.StoreID,
				Content:        body.Content,
				StagedTokens:   body.StagedTokens,
				ProductIds:     body.ProductIDs,
				Tags:           body.Tags,
				IdempotencyKey: req.IdempotencyKey,
				Delegation:     dctx,
			}, h.Delegation.GrpcDeadlineDomainWriteMs)
			if err != nil {
				return nil, denialFromGrpcErr(err)
			}
			if resp.GetCode() != 0 {
				return nil, delegationDenialFromCode(resp.GetCode(), resp.GetMessage())
			}
			return postResource(resp.GetData()), nil
		},
	})
	if denial != nil {
		denyDelegated(c, denial)
		return
	}
	respondDelegatedMutation(c, result)
}

// UpdateDelegatedPost handles PATCH .../posts/{postId}.
func (h *Holder) UpdateDelegatedPost(c *gin.Context) {
	req, ok := resolveDelegatedRequest(c, true)
	if !ok {
		return
	}
	postID := c.Param("post_id")
	var body DelegatedPostUpdateRequest
	payloadHash, ok := bindDelegatedBody(c, &body)
	if !ok {
		return
	}
	if !validateExpectedVersion(c, body.ExpectedVersion) {
		return
	}
	reason, ok := validateOperationReason(c, body.OperationReason)
	if !ok {
		return
	}
	if !validatePostContent(c, body.Content, false) ||
		!validatePostStagedTokens(c, body.StagedTokens) ||
		!validatePostTags(c, body.Tags) ||
		!validatePostProductIDs(c, body.ProductIDs) {
		return
	}

	var before *DelegatedPostDetail
	orch := h.orchestrator()
	result, denial := orch.Orchestrate(c.Request.Context(), delegation.Spec{
		Session:      req.Session,
		Action:       "POST_UPDATE",
		ResourceType: "post",
		ReadCap:      authz.AdminCapPostsRead,
		WriteCap:     authz.AdminCapPostsWrite,
		// 全表比較：UpdatePost 回完整 PostDetailInfo，兩側鍵集合對稱。
		SnapshotFields:  nil,
		OperationReason: reason,
		NotifySeller:    notifySellerOrDefault(body.NotifySeller),
		IdempotencyKey:  req.IdempotencyKey,
		PayloadHash:     payloadHash,
		RequestID:       req.RequestID,
		CorrelationID:   req.CorrelationID,
		UserAgent:       req.UserAgent,
		ResourceID:      postID,
		ReadBefore: func(ctx context.Context, dctx *commonpb.DelegationContext) (*delegation.Resource, *delegation.Denial) {
			res, denial := h.readPost(ctx, req.StoreID, postID, dctx)
			if denial != nil {
				return nil, denial
			}
			detail := res.Payload.(DelegatedPostDetail)
			before = &detail
			return res, nil
		},
		Write: func(ctx context.Context, dctx *commonpb.DelegationContext) (*delegation.Resource, *delegation.Denial) {
			updateReq := &socialpb.UpdatePostRequest{
				PostId:          postID,
				SellerId:        req.StoreID,
				NewStagedTokens: body.StagedTokens,
				ProductIds:      body.ProductIDs,
				ExpectedVersion: body.ExpectedVersion,
				IdempotencyKey:  req.IdempotencyKey,
				Delegation:      dctx,
			}
			if body.Content != nil {
				updateReq.Content = *body.Content
			} else if before != nil {
				updateReq.Content = before.Content
			}
			if body.Tags != nil {
				updateReq.Tags = body.Tags
				updateReq.TagsPresent = true
			}
			// 沒有送新圖時保留既有圖：social 的 keep_asset_ids 是「要留下來的」，
			// 空著等於清空。
			if len(body.StagedTokens) == 0 && before != nil {
				updateReq.KeepAssetIds = before.ImageKeys
			}
			resp, err := h.Social.UpdatePostForDelegation(ctx, updateReq, h.Delegation.GrpcDeadlineDomainWriteMs)
			if err != nil {
				return nil, denialFromGrpcErr(err)
			}
			if resp.GetCode() != 0 {
				return nil, delegationDenialFromCode(resp.GetCode(), resp.GetMessage())
			}
			return postResource(resp.GetData()), nil
		},
	})
	if denial != nil {
		denyDelegated(c, denial)
		return
	}
	respondDelegatedMutation(c, result)
}

// PublishDelegatedPost handles POST .../posts/{postId}/publish.
func (h *Holder) PublishDelegatedPost(c *gin.Context) {
	req, ok := resolveDelegatedRequest(c, true)
	if !ok {
		return
	}
	postID := c.Param("post_id")
	var body DelegatedPostPublishRequest
	payloadHash, ok := bindDelegatedBody(c, &body)
	if !ok {
		return
	}
	if !validateExpectedVersion(c, body.ExpectedVersion) {
		return
	}
	reason, ok := validateOperationReason(c, body.OperationReason)
	if !ok {
		return
	}
	scheduledAt, action, ok := resolvePublishMode(c, body)
	if !ok {
		return
	}

	orch := h.orchestrator()
	result, denial := orch.Orchestrate(c.Request.Context(), delegation.Spec{
		Session:      req.Session,
		Action:       action,
		ResourceType: "post",
		ReadCap:      authz.AdminCapPostsRead,
		WriteCap:     authz.AdminCapPostsPublish,
		// 只比發布三欄（publishStatus／scheduledPublishAt／publishedAt）。本端點的
		// body 動不到任何內容欄，收窄之後 content／tags／圖片數即使在寫入回應裡缺席
		// 或為零值，也不會被記成「被清空」。
		SnapshotFields:  delegation.ScopePostPublishState(),
		OperationReason: reason,
		NotifySeller:    notifySellerOrDefault(body.NotifySeller),
		IdempotencyKey:  req.IdempotencyKey,
		PayloadHash:     payloadHash,
		RequestID:       req.RequestID,
		CorrelationID:   req.CorrelationID,
		UserAgent:       req.UserAgent,
		ResourceID:      postID,
		ReadBefore: func(ctx context.Context, dctx *commonpb.DelegationContext) (*delegation.Resource, *delegation.Denial) {
			return h.readPost(ctx, req.StoreID, postID, dctx)
		},
		Write: func(ctx context.Context, dctx *commonpb.DelegationContext) (*delegation.Resource, *delegation.Denial) {
			return h.writePostStatus(ctx, req, postID, strconv.Itoa(codemeta.PostPublishStatusPublished),
				scheduledAt, body.ExpectedVersion, dctx)
		},
	})
	if denial != nil {
		denyDelegated(c, denial)
		return
	}
	respondDelegatedMutation(c, result)
}

// UnpublishDelegatedPost handles POST .../posts/{postId}/unpublish.
func (h *Holder) UnpublishDelegatedPost(c *gin.Context) {
	req, ok := resolveDelegatedRequest(c, true)
	if !ok {
		return
	}
	postID := c.Param("post_id")
	var body DelegatedPostUnpublishRequest
	payloadHash, ok := bindDelegatedBody(c, &body)
	if !ok {
		return
	}
	if !validateExpectedVersion(c, body.ExpectedVersion) {
		return
	}
	reason, ok := validateOperationReason(c, body.OperationReason)
	if !ok {
		return
	}

	orch := h.orchestrator()
	result, denial := orch.Orchestrate(c.Request.Context(), delegation.Spec{
		Session:      req.Session,
		Action:       "POST_UNPUBLISH",
		ResourceType: "post",
		ReadCap:      authz.AdminCapPostsRead,
		WriteCap:     authz.AdminCapPostsPublish,
		// 只比發布三欄，理由同 /publish。
		SnapshotFields:  delegation.ScopePostPublishState(),
		OperationReason: reason,
		NotifySeller:    notifySellerOrDefault(body.NotifySeller),
		IdempotencyKey:  req.IdempotencyKey,
		PayloadHash:     payloadHash,
		RequestID:       req.RequestID,
		CorrelationID:   req.CorrelationID,
		UserAgent:       req.UserAgent,
		ResourceID:      postID,
		ReadBefore: func(ctx context.Context, dctx *commonpb.DelegationContext) (*delegation.Resource, *delegation.Denial) {
			return h.readPost(ctx, req.StoreID, postID, dctx)
		},
		Write: func(ctx context.Context, dctx *commonpb.DelegationContext) (*delegation.Resource, *delegation.Denial) {
			return h.writePostStatus(ctx, req, postID, strconv.Itoa(codemeta.PostPublishStatusUnpublished),
				"", body.ExpectedVersion, dctx)
		},
	})
	if denial != nil {
		denyDelegated(c, denial)
		return
	}
	respondDelegatedMutation(c, result)
}

func (h *Holder) writePostStatus(ctx context.Context, req *delegatedRequest, postID, targetStatus, scheduledAt, expectedVersion string, dctx *commonpb.DelegationContext) (*delegation.Resource, *delegation.Denial) {
	resp, err := h.Social.UpdateSellerPostStatusForDelegation(ctx, &socialpb.UpdateSellerPostStatusRequest{
		PostId:             postID,
		SellerId:           req.StoreID,
		TargetStatus:       targetStatus,
		ScheduledPublishAt: scheduledAt,
		ExpectedVersion:    expectedVersion,
		IdempotencyKey:     req.IdempotencyKey,
		Delegation:         dctx,
	}, h.Delegation.GrpcDeadlineDomainWriteMs)
	if err != nil {
		return nil, denialFromGrpcErr(err)
	}
	if resp.GetCode() != 0 {
		return nil, delegationDenialFromCode(resp.GetCode(), resp.GetMessage())
	}
	return postResource(resp.GetData()), nil
}

// ==================== 讀取與轉換 ====================

func (h *Holder) readPost(ctx context.Context, storeID int64, postID string, dctx *commonpb.DelegationContext) (*delegation.Resource, *delegation.Denial) {
	if h.Social == nil {
		return nil, upstreamDenial()
	}
	resp, err := h.Social.GetPostDetailForDelegation(ctx, &socialpb.GetPostDetailRequest{
		PostId: postID,
		// owner_store_id 一定要填：GetPost 的 SQL 刻意不過濾 seller（留言／檢舉派工／
		// 隱藏／詳情四類路徑共用它作存在性查詢），缺這一欄 Q4 就無值可比，代行者
		// 讀得到任意賣家的貼文詳情，而讀取成功不落稽核 ⇒ 零痕跡。
		OwnerStoreId: storeID,
		Delegation:   dctx,
	}, h.Delegation.GrpcDeadlineDomainReadMs)
	if err != nil {
		return nil, denialFromGrpcErr(err)
	}
	if resp.GetCode() != 0 {
		return nil, delegationDenialFromCode(resp.GetCode(), resp.GetMessage())
	}
	return postResource(resp.GetData()), nil
}

func postResource(info *socialpb.PostDetailInfo) *delegation.Resource {
	products := make([]DelegatedPostProduct, 0, len(info.GetProducts()))
	productIDs := make([]string, 0, len(info.GetProducts()))
	for _, p := range info.GetProducts() {
		products = append(products, DelegatedPostProduct{
			ProductID: p.GetId(),
			Name:      p.GetName(),
			ImageKey:  p.GetImageKey(),
			Price:     p.GetPrice(),
		})
		productIDs = append(productIDs, p.GetId())
	}

	dto := DelegatedPostDetail{
		PostID:             info.GetId(),
		Version:            info.GetVersion(),
		Content:            info.GetContent(),
		ImageKeys:          nonNilStrings(info.GetImageKeys()),
		Tags:               nonNilStrings(info.GetTags()),
		PublishStatus:      info.GetPublishStatus(),
		PublishStatusLabel: codemeta.PostPublishStatusLabel(info.GetPublishStatus()),
		CreatedAt:          epochToISO8601(info.GetCreatedAt()),
		HiddenReason:       info.GetHiddenReason(),
		Products:           products,
	}
	if v := info.GetHiddenAt(); v != "" {
		dto.HiddenAt = &v
	}
	if v := info.GetScheduledPublishAt(); v != "" {
		dto.ScheduledPublishAt = &v
	}
	if v := info.GetPublishedAt(); v != "" {
		dto.PublishedAt = &v
	}

	imageKey := ""
	if len(dto.ImageKeys) > 0 {
		imageKey = dto.ImageKeys[0]
	}
	return &delegation.Resource{
		ID:       dto.PostID,
		Name:     truncateRunes(dto.Content, 60),
		ImageKey: imageKey,
		Version:  dto.Version,
		Fields: map[string]any{
			"content":            dto.Content,
			"tags":               joinForDisplay(dto.Tags),
			"productIds":         strconv.Itoa(len(productIDs)),
			"imageKeys":          strconv.Itoa(len(dto.ImageKeys)),
			"publishStatus":      dto.PublishStatus,
			"scheduledPublishAt": info.GetScheduledPublishAt(),
			"publishedAt":        info.GetPublishedAt(),
		},
		Payload: dto,
	}
}

// ==================== 值域 ====================

func validatePostContent(c *gin.Context, content *string, required bool) bool {
	if content == nil {
		if required {
			response.BizStatus(c, bizcode.DelegationInvalidCommand, "貼文內容為必填")
			return false
		}
		return true
	}
	trimmed := strings.TrimSpace(*content)
	if trimmed == "" || len([]rune(trimmed)) > 2000 {
		response.BizStatus(c, bizcode.DelegationInvalidCommand, "貼文內容需為 1–2000 字")
		return false
	}
	return true
}

func validatePostStagedTokens(c *gin.Context, tokens []string) bool {
	if len(tokens) > 1 {
		response.BizStatus(c, bizcode.DelegationInvalidCommand, "每則貼文只能有 1 張照片")
		return false
	}
	for _, t := range tokens {
		if t == "" || len(t) > 4096 {
			response.BizStatus(c, bizcode.DelegationInvalidCommand, "stagedToken 不正確")
			return false
		}
	}
	return true
}

// validatePostTags mirrors social-service's own limits (MaxCount = 10, MaxLength = 40
// code points). Gateway only checks the counts; canonicalisation and duplicate
// detection stay authoritative in the domain service.
func validatePostTags(c *gin.Context, tags []string) bool {
	if len(tags) > 10 {
		response.BizStatus(c, bizcode.DelegationInvalidCommand, "主題標籤最多 10 個")
		return false
	}
	for _, t := range tags {
		if len([]rune(t)) > 40 {
			response.BizStatus(c, bizcode.DelegationInvalidCommand, "單一主題標籤不得超過 40 字")
			return false
		}
	}
	return true
}

func validatePostProductIDs(c *gin.Context, ids []string) bool {
	for _, id := range ids {
		if !delegatedVersionRE.MatchString(id) {
			response.BizStatus(c, bizcode.DelegationInvalidCommand, "productIds 格式不正確")
			return false
		}
	}
	return true
}

// resolvePublishMode validates the two-value publishMode and the schedule window,
// and picks the audit action: a successfully created schedule is POST_SCHEDULE, not
// POST_PUBLISH. The actual publish later writes its own POST_PUBLISH row with
// origin='scheduled'.
func resolvePublishMode(c *gin.Context, body DelegatedPostPublishRequest) (scheduledAt string, action string, ok bool) {
	switch body.PublishMode {
	case "immediate":
		if body.ScheduledPublishAt != "" {
			response.BizStatus(c, bizcode.DelegationInvalidCommand, "立即發布不得帶排程時間")
			return "", "", false
		}
		return "", "POST_PUBLISH", true
	case "scheduled":
		if body.ScheduledPublishAt == "" {
			response.BizStatus(c, bizcode.DelegationInvalidCommand, "排程發布必須帶 scheduledPublishAt")
			return "", "", false
		}
		t, err := time.Parse(time.RFC3339, body.ScheduledPublishAt)
		if err != nil || !strings.HasSuffix(body.ScheduledPublishAt, "Z") || strings.Contains(body.ScheduledPublishAt, ".") {
			response.BizStatus(c, bizcode.DelegationInvalidCommand, "scheduledPublishAt 需為秒精度的 ISO 8601 UTC")
			return "", "", false
		}
		// 兩處各以自己收到請求當下的 now() 為基準，不採信任何由請求帶入的「現在時刻」。
		now := time.Now().UTC()
		if t.Before(now.Add(delegatedScheduleMinLead)) || t.After(now.Add(delegatedScheduleMaxLead)) {
			response.BizStatus(c, bizcode.DelegationInvalidCommand, "排程時間需介於 5 分鐘後與 90 天內")
			return "", "", false
		}
		return body.ScheduledPublishAt, "POST_SCHEDULE", true
	default:
		response.BizStatus(c, bizcode.DelegationInvalidCommand, "publishMode 需為 immediate 或 scheduled")
		return "", "", false
	}
}

// ==================== 小工具 ====================

// nonNilStrings keeps empty arrays as [] rather than null: the DTO promises an array
// and a null there is a second shape the front end has to handle.
func nonNilStrings(in []string) []string {
	if in == nil {
		return []string{}
	}
	return in
}

func singleKey(key string) []string {
	if key == "" {
		return nil
	}
	return []string{key}
}

// epochToISO8601 renders social's unix seconds as the contract's second-precision
// UTC string. Sub-second digits are not optional to trim: the front end's regex
// accepts at most three of them and rejects the whole response otherwise.
func epochToISO8601(sec int64) string {
	if sec == 0 {
		return ""
	}
	return timeutil.FormatISO8601(time.Unix(sec, 0).UTC().Truncate(time.Second))
}

func truncateRunes(s string, limit int) string {
	runes := []rune(s)
	if len(runes) <= limit {
		return s
	}
	return string(runes[:limit])
}
