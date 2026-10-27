package usecase

import (
	"context"
	"fmt"

	"github.com/Will413028/hawkr/go-pkg/logger"
	"github.com/Will413028/hawkr/social-service/internal/domain"
	"go.uber.org/zap"
)

// resolveForumIdentity 解析討論區留言者的顯示名稱與頭像 objectKey。
//
// 鏡射 comment_snapshot.go 的 resolveCommentIdentity——刻意走 *MetaBatch 這組
// RPC，與 REST 列表的 enrichForumUserInfo 完全同一個來源，兩條路徑因此不可能
// 給出不同答案（同款 #2223 教訓，見 669bc471f 該次只修了 enrichCommentUserInfo，
// 沒有連帶修 enrichForumUserInfo／ForumUseCase，2026-08-08 補齊）。
//
// 查不到時重試一次（涵蓋 transient 失敗）；仍查不到才回空字串。
func (uc *ForumUseCase) resolveForumIdentity(ctx context.Context, actor Actor) (userName, avatarKey string) {
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
		logger.Warn(ctx, "forum identity lookup failed; entry carries empty name/avatar",
			zap.Int64("actorID", actor.ID), zap.Int16("actorKind", actor.Kind),
			// nil = 對方明確回「查無此人」；非 nil = 根本沒問到。
			zap.Error(lastErr))
		return "", ""
	}
	return info.Nickname, info.Avatar
}
