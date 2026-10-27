package usecase

import (
	"context"
	"fmt"

	"github.com/Will413028/hawkr/go-pkg/logger"
	pkgmq "github.com/Will413028/hawkr/go-pkg/mq"
	"github.com/Will413028/hawkr/go-pkg/timeutil"
	"github.com/Will413028/hawkr/social-service/internal/domain"
	"github.com/Will413028/hawkr/social-service/internal/repository/postgres/gen"
	"go.uber.org/zap"
)

// buildCommentSnapshot 將 cms_comment row 轉成 fat event 用的 CommentSnapshot。
// 41 號設計書 §6.1 — fat event payload nested DTO，避免 gateway 二次 gRPC 查詢。
//
// userName / avatarKey 由呼叫端以 resolveCommentIdentity 解析後傳入，**不從 row 讀**
// （#2223）：`cms_comment.user_name` / `user_avatar` 是建立當下寫下的反正規化快照，
// 解析失敗時會是 NULL，而 REST 列表走 enrichCommentUsers 於讀取時現查、下一次重拉就
// 自癒——SSE 只有這一幀機會，讀 row 等於把「當下查不到」永久凍進推播裡（實測：同一則
// 留言 SSE avatarKey="" 而 REST 回 objectKey）。
func buildCommentSnapshot(c *gen.CmsComment, userName, avatarKey string) *pkgmq.CommentSnapshot {
	if c == nil {
		return nil
	}
	snap := &pkgmq.CommentSnapshot{
		CommentID:  c.ID,
		PostID:     c.PostID,
		UserID:     c.UserID,
		Role:       int32(c.Role),
		ParentID:   c.ParentID,
		ReplyToID:  c.ReplyToID,
		LikeCount:  c.LikeCount,
		ReplyCount: c.ReplyCount,
		UserName:   userName,
		AvatarKey:  avatarKey,
	}
	if c.Content != nil {
		snap.Content = *c.Content
	}
	if c.ReplyToName != nil {
		snap.ReplyToName = *c.ReplyToName
	}
	if c.CreateTime.Valid {
		snap.CreatedAt = timeutil.FormatISO8601(c.CreateTime.Time)
	}
	if c.UpdateTime.Valid {
		snap.UpdatedAt = timeutil.FormatISO8601(c.UpdateTime.Time)
	}
	return snap
}

// resolveCommentIdentity 解析留言者的顯示名稱與頭像 objectKey。
//
// 刻意走 *MetaBatch 這組 RPC——與 REST 列表的 enrichCommentUsers 完全同一個來源，
// 兩條路徑因此不可能給出不同答案（#2223 的成因正是各查各的）。批次版另有一個單筆版
// 沒有的性質：**map 有沒有這個 key 可以區分「查不到」與「查到但沒設頭像」**，
// 單筆版把兩者都攤成空字串，於是無法只在真正失敗時重試。
//
// 查不到時重試一次（涵蓋 transient 失敗）；仍查不到才回空字串。
func (uc *CommentUseCase) resolveCommentIdentity(ctx context.Context, actor Actor) (userName, avatarKey string) {
	// lastErr 保留最後一次的失敗原因：`ok=false` 同時涵蓋「查無此人」與
	// 「RPC 失敗」，兩者的處置一樣（重試後回空字串），但排查方式完全不同 ——
	// 少了原因的 Warn 只能叫人去猜（#2830 群組 5 同一類）。
	var lastErr error
	lookup := func() (*AccountInfo, bool) {
		switch actor.Kind {
		case domain.ActorKindStore:
			m, err := uc.sellerClient.GetSellerMetaBatch(ctx, []int64{actor.ID})
			lastErr = err
			info, ok := m[actor.ID]
			return info, ok && info != nil
		case domain.ActorKindBuyer:
			m, err := uc.buyerClient.GetBuyerMetaBatch(ctx, []int64{actor.ID})
			lastErr = err
			info, ok := m[actor.ID]
			return info, ok && info != nil
		}
		lastErr = fmt.Errorf("unknown actor kind %d", actor.Kind)
		return nil, false
	}

	info, ok := lookup()
	if !ok {
		info, ok = lookup()
	}
	if !ok {
		// Warn 而非 Debug：這一則留言的推播與列表會顯示成無名無頭像，是使用者看得到的
		// 降級，不該只活在 debug log 裡（既有的單筆 client 就是 Debug，等同無聲）。
		logger.Warn(ctx, "comment identity lookup failed; snapshot carries empty name/avatar",
			zap.Int64("actorID", actor.ID), zap.Int16("actorKind", actor.Kind),
			// nil = 對方明確回「查無此人」；非 nil = 根本沒問到。
			zap.Error(lastErr))
		return "", ""
	}
	return info.Nickname, info.Avatar
}
