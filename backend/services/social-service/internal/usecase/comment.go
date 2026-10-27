package usecase

import (
	"context"
	"fmt"
	"strings"

	"github.com/Will413028/hawkr/go-pkg/moderation"
	"github.com/Will413028/hawkr/go-pkg/pagination"
	"github.com/Will413028/hawkr/go-pkg/timeutil"
	"github.com/Will413028/hawkr/social-service/internal/domain"
	"github.com/Will413028/hawkr/social-service/internal/repository/postgres/gen"
)

type CommentUseCase struct {
	commentRepo    domain.CommentRepository
	postRepo       domain.PostRepository
	buyerClient    BuyerClient
	sellerClient   SellerClient
	eventPublisher ContentEventPublisher
	tsid           func() int64
}

func NewCommentUseCase(
	commentRepo domain.CommentRepository,
	postRepo domain.PostRepository,
	buyerClient BuyerClient,
	sellerClient SellerClient,
	eventPublisher ContentEventPublisher,
	tsid func() int64,
) *CommentUseCase {
	return &CommentUseCase{
		commentRepo:    commentRepo,
		postRepo:       postRepo,
		buyerClient:    buyerClient,
		sellerClient:   sellerClient,
		eventPublisher: eventPublisher,
		tsid:           tsid,
	}
}

func (uc *CommentUseCase) ListTopLevel(ctx context.Context, postID int64, page, pageSize int32) ([]*gen.CmsComment, int64, error) {
	page, pageSize, _ = pagination.NormalizePage(page, pageSize, domain.DefaultPageSize, domain.MaxPageSize)
	offset := int64(page-1) * int64(pageSize)
	comments, err := uc.commentRepo.ListTopLevel(ctx, postID, int64(pageSize), offset)
	if err != nil {
		return nil, 0, err
	}
	total, err := uc.commentRepo.CountTopLevel(ctx, postID)
	if err != nil {
		return nil, 0, err
	}
	return comments, total, nil
}

func (uc *CommentUseCase) ListReplies(ctx context.Context, parentID int64, page, pageSize int32) ([]*gen.CmsComment, int64, error) {
	page, pageSize, _ = pagination.NormalizePage(page, pageSize, domain.DefaultPageSize, domain.MaxPageSize)
	offset := int64(page-1) * int64(pageSize)
	replies, err := uc.commentRepo.ListReplies(ctx, parentID, int64(pageSize), offset)
	if err != nil {
		return nil, 0, err
	}
	total, err := uc.commentRepo.CountReplies(ctx, parentID)
	if err != nil {
		return nil, 0, err
	}
	return replies, total, nil
}

func (uc *CommentUseCase) GetLatestReplies(ctx context.Context, parentID int64, limit int32) ([]*gen.CmsComment, error) {
	return uc.commentRepo.GetLatestReplies(ctx, parentID, int64(limit))
}

// BatchGetLatestReplies fetches latest N replies for multiple parents in one query.
// Returns a map of parentID → []*CmsComment.
func (uc *CommentUseCase) BatchGetLatestReplies(ctx context.Context, parentIDs []int64, limit int32) map[int64][]*gen.CmsComment {
	if len(parentIDs) == 0 {
		return nil
	}
	replies, err := uc.commentRepo.BatchGetLatestReplies(ctx, parentIDs, limit)
	if err != nil {
		return nil
	}
	m := make(map[int64][]*gen.CmsComment, len(parentIDs))
	for _, r := range replies {
		r2 := r
		m[r2.ParentID] = append(m[r2.ParentID], r2)
	}
	return m
}

// ViewerForPost 回傳呼叫者在這則貼文底下的互動身分（讚的個人化、封鎖過濾用）：貼文屬於呼叫者的店 →
// 店家，否則買家，兩者都沒有 → 零值。
func (uc *CommentUseCase) ViewerForPost(ctx context.Context, postID int64, caller Caller) Actor {
	return caller.ViewerActor(uc.postOwnerStoreID(ctx, postID))
}

// ViewerForComment 與 ViewerForPost 相同，目標是留言所在的貼文。
func (uc *CommentUseCase) ViewerForComment(ctx context.Context, commentID int64, caller Caller) Actor {
	comment, err := uc.commentRepo.GetByID(ctx, commentID)
	if err != nil || comment == nil {
		return caller.ViewerActor(0)
	}
	return uc.ViewerForPost(ctx, comment.PostID, caller)
}

// postOwnerStoreID 回傳貼文所屬的店；貼文不存在或已刪除回 0（沒有人擁有它）。
func (uc *CommentUseCase) postOwnerStoreID(ctx context.Context, postID int64) int64 {
	post, err := uc.postRepo.GetByID(ctx, postID)
	if err != nil || post == nil {
		return 0
	}
	return post.SellerID
}

func (uc *CommentUseCase) IsLiked(ctx context.Context, commentID int64, viewer Actor) bool {
	if viewer.IsZero() {
		return false
	}
	_, err := uc.commentRepo.GetLike(ctx, commentID, viewer.Kind, viewer.ID)
	return err == nil
}

// BatchIsLiked returns a set of commentIDs that the viewer has liked.
func (uc *CommentUseCase) BatchIsLiked(ctx context.Context, commentIDs []int64, viewer Actor) map[int64]bool {
	if viewer.IsZero() || len(commentIDs) == 0 {
		return nil
	}
	ids, err := uc.commentRepo.BatchGetLikedCommentIDs(ctx, commentIDs, viewer.Kind, viewer.ID)
	if err != nil {
		return nil
	}
	m := make(map[int64]bool, len(ids))
	for _, id := range ids {
		m[id] = true
	}
	return m
}

// principalOf 把互動者解析成通知收件匣的身分空間：店家 → account_id，買家 → buyer_id 原值。
func (uc *CommentUseCase) principalOf(ctx context.Context, kind int16, id int64) int64 {
	switch kind {
	case domain.ActorKindStore:
		return uc.sellerClient.ResolvePrincipal(ctx, id)
	case domain.ActorKindBuyer:
		return id
	}
	return 0 // 未知的 kind 不猜身分空間
}

// principalSpaceOf 回傳互動者種類對應的身分空間名稱（消費端靠它分流）與收件匣身分值；
// 與 principalOf 共用同一個 kind 對照，未知的 kind 回 ok=false，呼叫端不得當成買家。
func (uc *CommentUseCase) principalSpaceOf(ctx context.Context, kind int16, id int64) (space string, principal int64, ok bool) {
	switch kind {
	case domain.ActorKindStore:
		return "seller", uc.principalOf(ctx, kind, id), true
	case domain.ActorKindBuyer:
		return "buyer", uc.principalOf(ctx, kind, id), true
	}
	return "", 0, false
}

func (uc *CommentUseCase) CreateComment(ctx context.Context, postID int64, caller Caller, content string, parentID, replyToID int64, clientRequestID string, sellerSuspended bool) (*gen.CmsComment, error) {
	if strings.TrimSpace(content) == "" {
		return nil, ErrBadRequest
	}

	// B-135: profanity dictionary
	if hit, _ := moderation.Check(content); hit {
		return nil, ErrModerationRejected
	}

	post, err := uc.postRepo.GetByID(ctx, postID)
	if err != nil || post == nil {
		return nil, ErrNotFound
	}

	if parentID > 0 {
		parent, perr := uc.commentRepo.GetByID(ctx, parentID)
		if perr != nil || parent == nil || parent.DeletedAt != nil || parent.PostID != postID {
			return nil, ErrNotFound
		}
		if parent.ParentID > 0 {
			parentID = parent.ParentID
		}
	}

	actor, err := caller.DeriveActor(post.SellerID)
	if err != nil {
		return nil, err
	}
	// 回覆只限貼文所屬的店；店家身分在自己的貼文上停權時，留言與回覆都擋，不退回本人身分。
	if parentID > 0 && actor.Kind != domain.ActorKindStore {
		return nil, ErrForbidden
	}
	if sellerSuspended && actor.Kind == domain.ActorKindStore {
		return nil, ErrSellerSuspendedOperationBlocked
	}

	userName, userAvatar := uc.resolveCommentIdentity(ctx, actor)

	// 取 replyToName
	var replyToName string
	if replyToID > 0 {
		if replyTo, err := uc.commentRepo.GetByID(ctx, replyToID); err == nil && replyTo != nil {
			replyToName = stringOrEmpty(replyTo.UserName)
		}
	}

	comment, err := uc.commentRepo.Create(ctx, uc.tsid(), postID, actor.ID, userName, userAvatar, content, parentID, replyToID, replyToName, actor.Kind)
	if err != nil {
		return nil, err
	}
	// counter 更新改為非同步（見 docs/superpowers/specs/2026-07-31-social-counter-async-consistency-design.md），
	// 不再同步呼叫 Incr*，避免熱門貼文的 row lock 競爭卡在使用者請求路徑上。
	if uc.eventPublisher != nil {
		if parentID > 0 {
			_ = uc.eventPublisher.PublishCounterDelta(ctx, "comment", parentID, "reply_count", 1)
		} else {
			// comment_count 只計頂層留言，對齊留言列表 API 的 total（#1171）
			_ = uc.eventPublisher.PublishCounterDelta(ctx, "post", postID, "comment_count", 1)
		}
	}

	// 41 號設計書 §3.3.1:publisher 一律 publish,self-author skip 由 notification consumer
	// (template_handler.go) 自己判斷,publisher 不擋。確保 gateway SSE 不漏推給留言者
	// 的其他 tab / 裝置,以及看同一貼文的其他 viewer(§一.3 設計目標)。
	if uc.eventPublisher != nil && parentID == 0 {
		preview := content
		if len(preview) > domain.PreviewMaxLength {
			preview = preview[:domain.PreviewMaxLength]
		}
		// 貼文作者恆為店家（cms_post.seller_id 是 store_id），不跨身分空間探測。
		const postAuthorType = "seller"
		snapshot := buildCommentSnapshot(comment, userName, userAvatar)
		resolvedPostAuthorID := uc.sellerClient.ResolvePrincipal(ctx, post.SellerID)

		// commenterPrincipalId：notification-service 的自我通知抑制拿它跟 postAuthorId 比對，
		// 兩者必須落在同一個身分空間。貼文作者永遠是店家：留言者是這家店 (kind, id) 相同時，
		// 直接複用貼文作者側算好的值，兩個值必然同源（ResolvePrincipal 失敗時原樣回傳輸入，
		// 各解析各的會一成一敗而漏掉 skip）。
		var commenterPrincipalID int64
		if actor.Kind == domain.ActorKindStore && actor.ID == post.SellerID {
			commenterPrincipalID = resolvedPostAuthorID
		} else {
			commenterPrincipalID = uc.principalOf(ctx, actor.Kind, actor.ID)
		}
		_ = uc.eventPublisher.PublishCommentCreated(ctx, comment.ID, postID, actor.ID, commenterPrincipalID, userName, resolvedPostAuthorID, post.SellerID, postAuthorType, preview, parentID, clientRequestID, snapshot)
	}

	// 41 號設計書 §3.3.1:reply 分支同樣 publisher 一律 publish。replierID 加入 payload
	// 供 notification consumer 做 self-reply skip。
	if uc.eventPublisher != nil && replyToID > 0 {
		if replyTo, rerr := uc.commentRepo.GetByID(ctx, replyToID); rerr == nil && replyTo != nil {
			// notifications.user_id 的讀取端是不對稱的（gateway resolveInboxUserID）：賣家
			// 收件匣以 account_id 查、買家收件匣以 buyer_id 查。replyToUserId 同時被
			// comment.replied.seller / comment.replied.buyer 兩張 template 讀，靠 replyToUserType
			// 分流，所以依被回覆者的身分種類填對應身分空間的值。
			replyToUserType, replyToUserID, kindOK := uc.principalSpaceOf(ctx, replyTo.Role, replyTo.UserID)
			if !kindOK {
				return comment, nil // 未知的身分種類：不猜收件匣，不發通知
			}
			preview := content
			if len(preview) > domain.PreviewMaxLength {
				preview = preview[:domain.PreviewMaxLength]
			}
			snapshot := buildCommentSnapshot(comment, userName, userAvatar)

			// replierPrincipalId：self-skip 的 actor 側，與 replyToUserID 同身分空間；回覆自己的
			// 留言（(kind, id) 相同）時複用被回覆者側的值，理由同 commenterPrincipalId。
			var replierPrincipalID int64
			if actor.Kind == replyTo.Role && actor.ID == replyTo.UserID {
				replierPrincipalID = replyToUserID
			} else {
				replierPrincipalID = uc.principalOf(ctx, actor.Kind, actor.ID)
			}
			_ = uc.eventPublisher.PublishCommentReplied(ctx, replyToUserID, actor.ID, replierPrincipalID, replyTo.UserID, replyToUserType, userName, postID, comment.ID, preview, parentID, clientRequestID, snapshot)
		}
	}

	return comment, nil
}

func (uc *CommentUseCase) UpdateComment(ctx context.Context, commentID int64, caller Caller, content string) (*gen.CmsComment, error) {
	if strings.TrimSpace(content) == "" {
		return nil, ErrBadRequest
	}

	comment, err := uc.commentRepo.GetByID(ctx, commentID)
	if err != nil || comment.DeletedAt != nil {
		return nil, ErrNotFound
	}

	if !caller.IsAuthor(comment.Role, comment.UserID) {
		return nil, ErrForbidden
	}

	updated, err := uc.commentRepo.UpdateContent(ctx, commentID, content)
	if err != nil {
		return nil, err
	}

	// 41 號設計書 §3.3 / §6.1：best-effort publish comment.updated 給 gateway SSE。
	if uc.eventPublisher != nil && updated != nil {
		updatedAt := ""
		if updated.UpdateTime.Valid {
			updatedAt = timeutil.FormatISO8601(updated.UpdateTime.Time)
		}
		_ = uc.eventPublisher.PublishCommentUpdated(ctx, updated.ID, updated.PostID, content, updatedAt)
	}

	return updated, nil
}

func (uc *CommentUseCase) DeleteComment(ctx context.Context, commentID int64, caller Caller) error {
	comment, err := uc.commentRepo.GetByID(ctx, commentID)
	if err != nil {
		return ErrNotFound
	}

	isOwner := caller.IsAuthor(comment.Role, comment.UserID)
	var isPostOwner bool
	if !isOwner {
		post, err := uc.postRepo.GetByID(ctx, comment.PostID)
		isPostOwner = err == nil && post != nil && caller.OwnsStore(post.SellerID)
	}
	if !isOwner && !isPostOwner {
		return ErrForbidden
	}

	if err := uc.commentRepo.SoftDelete(ctx, commentID); err != nil {
		return err
	}
	// 非同步 counter 更新，理由同 CreateComment。
	if uc.eventPublisher != nil {
		if comment.ParentID > 0 {
			_ = uc.eventPublisher.PublishCounterDelta(ctx, "comment", comment.ParentID, "reply_count", -1)
		} else {
			// comment_count 只計頂層留言（#1171）
			_ = uc.eventPublisher.PublishCounterDelta(ctx, "post", comment.PostID, "comment_count", -1)
		}
	}

	// 41 號設計書 §3.3 / §6.1：best-effort publish comment.deleted 給 gateway SSE。
	if uc.eventPublisher != nil {
		_ = uc.eventPublisher.PublishCommentDeleted(ctx, commentID, comment.PostID, comment.ParentID)
	}

	return nil
}

func (uc *CommentUseCase) ToggleLike(ctx context.Context, commentID int64, caller Caller) (liked bool, err error) {
	// 存在性檢查：comment 不存在或已軟刪除 → 回 404，避免對不存在的留言寫入孤兒 like row
	// (GetComment 未過濾 deleted_at，故需顯式檢查 DeletedAt，同 UpdateComment 慣例)（#1485）。
	existing, getErr := uc.commentRepo.GetByID(ctx, commentID)
	if getErr != nil || existing == nil || existing.DeletedAt != nil {
		return false, ErrNotFound
	}

	// 留言所在的貼文屬於呼叫者的店 → 店家的讚；否則本人的讚。
	actor, err := caller.DeriveActor(uc.postOwnerStoreID(ctx, existing.PostID))
	if err != nil {
		return false, err
	}

	// commentAuthorId 是 comment.like_changed 唯一一張 template（target_user_type='seller'）的
	// target_user_field，而賣家收件匣是以 account_id 查的（gateway resolveInboxUserID），所以
	// 店家作者解析成 account_id；買家作者維持 buyer_id 原樣。
	//
	// commentAuthorType 隨 commentAuthorID 一起發（同 comment.replied 的 replyToUserType）：
	// 解析後的 commentAuthorId 不是單一身分空間的值，消費端靠它分流（#1626）。
	commentAuthorType, commentAuthorID, kindOK := uc.principalSpaceOf(ctx, existing.Role, existing.UserID)
	if !kindOK {
		return false, fmt.Errorf("comment %d has unknown author kind %d", commentID, existing.Role)
	}

	// likerPrincipalId：notification-service 的自我通知抑制拿它跟 commentAuthorId 比對，
	// 兩者必須落在同一個身分空間。按讚者與作者是同一個 (kind, id) 時直接複用作者側算好的值，
	// 兩個值必然同源（ResolvePrincipal 失敗時原樣回傳輸入，各解析各的會一成一敗而漏掉 skip）。
	var likerPrincipalID int64
	if actor.Kind == existing.Role && actor.ID == existing.UserID {
		likerPrincipalID = commentAuthorID
	} else {
		likerPrincipalID = uc.principalOf(ctx, actor.Kind, actor.ID)
	}

	_, err = uc.commentRepo.GetLike(ctx, commentID, actor.Kind, actor.ID)
	if err != nil {
		// LIKE 分支
		if dbErr := uc.commentRepo.CreateLike(ctx, commentID, actor.Kind, actor.ID); dbErr != nil {
			return false, dbErr
		}
		// like_count 改非同步更新（2026-07-31 design）。

		if uc.eventPublisher != nil {
			_ = uc.eventPublisher.PublishCounterDelta(ctx, "comment", commentID, "like_count", 1)

			// 41 號設計書 §6.1：publish comment.like_changed action=LIKE
			// notification consumer 過濾 LIKE 才推播；gateway SSE 推 like count 變化。
			// likeCount 用 existing.LikeCount+1（樂觀值）——不能重查 DB，counter 現在是
			// 非同步更新，重查會讀到還沒套用增量的舊值。
			likerName := uc.likerName(ctx, actor)
			_ = uc.eventPublisher.PublishCommentLikeChanged(ctx, commentID, existing.PostID, existing.LikeCount+1, "LIKE", actor.ID, int32(actor.Kind), commentAuthorID, existing.UserID, likerPrincipalID, likerName, commentAuthorType)
		}

		return true, nil
	}
	// UNLIKE 分支
	if dbErr := uc.commentRepo.DeleteLike(ctx, commentID, actor.Kind, actor.ID); dbErr != nil {
		return false, dbErr
	}
	// like_count 改非同步更新（2026-07-31 design）。

	if uc.eventPublisher != nil {
		_ = uc.eventPublisher.PublishCounterDelta(ctx, "comment", commentID, "like_count", -1)

		// 41 號設計書 §6.1：publish comment.like_changed action=UNLIKE
		// notification consumer 過濾 LIKE 才推播（UNLIKE 不發通知）；gateway SSE 仍推 like count 變化。
		// likeCount 用 existing.LikeCount-1（樂觀值），理由同上。counter 現在非同步更新，rapid
		// like→unlike（雙擊）可能在 consumer 套用前一個 +1 之前就讀到 existing.LikeCount==0，
		// 算出 -1 會原樣送到 SSE client（gateway 端無 clamp）。DB 側 DecrCommentLikeCount SQL 本身
		// 有 GREATEST(0, ...) 防線，這裡的樂觀值也要鏡射同一條下限，否則 SSE 會顯示負數。
		newLikeCount := existing.LikeCount - 1
		if newLikeCount < 0 {
			newLikeCount = 0
		}
		_ = uc.eventPublisher.PublishCommentLikeChanged(ctx, commentID, existing.PostID, newLikeCount, "UNLIKE", actor.ID, int32(actor.Kind), commentAuthorID, existing.UserID, likerPrincipalID, "", commentAuthorType)
	}

	return false, nil
}

// likerName 依身分種類查按讚者的顯示名稱：買家查買家資料、店家查店家資料。
func (uc *CommentUseCase) likerName(ctx context.Context, actor Actor) string {
	switch actor.Kind {
	case domain.ActorKindStore:
		if info, err := uc.sellerClient.GetSellerInfo(ctx, actor.ID); err == nil && info != nil {
			return info.Nickname
		}
	case domain.ActorKindBuyer:
		if info, err := uc.buyerClient.GetBuyerProfile(ctx, actor.ID); err == nil && info != nil {
			return info.Nickname
		}
	}
	return ""
}
