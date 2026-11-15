package audit

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"

	"github.com/Will413028/hawkr/backend/internal/modules/delegation/internal/model"
	"github.com/Will413028/hawkr/backend/internal/modules/delegation/internal/store"
)

// RecordScheduledPublish 在貼文排程發布的同一個 transaction 內寫一列 SUCCEEDED 的稽核（origin SCHEDULED、POST_PUBLISH、posts.publish）：發布
// 的事實就是結果，不走 intent 與 confirm 兩段（那是 gateway 的路徑，不動）。冪等鍵 schedule:<排程 ID>；同一個 session 同一把鍵已經有列時
// 當成功，不寫第二列。操作理由繼承同一個 session、同一篇貼文最近一筆 POST_SCHEDULE 的理由，找不到時留空（稽核列的理由欄可以是空的，
// 找不到不該讓已經發生的發布 rollback）。session 的快照（管理員、店家、案件編號）來自 session 列，不是排程。
func (m *Module) RecordScheduledPublish(ctx context.Context, in model.ScheduledPublishAuditInput) error {
	s, err := m.d.Session.Session(ctx, in.SessionID)
	if err != nil {
		return fmt.Errorf("delegation：讀取排程發布的代行：%w", err)
	}
	q := m.q(ctx)
	postID := in.PostID.String()
	reason, err := q.LatestDelegationAuditReason(ctx, store.LatestDelegationAuditReasonParams{
		SessionID: s.ID, ResourceID: postID, Action: string(model.AuditPostSchedule),
	})
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return fmt.Errorf("delegation：讀取排程的操作理由：%w", err)
	}
	key := model.ScheduledPublishIdempotencyKey(in.ScheduleID)
	_, err = q.InsertScheduledPublishAuditLog(ctx, store.InsertScheduledPublishAuditLogParams{
		ID:                    m.d.NewID(),
		SessionID:             s.ID,
		ActorAdminID:          s.Actor.AdminID,
		ActorDisplayName:      s.Actor.DisplayName,
		TargetStoreID:         s.Target.StoreID,
		TargetSellerAccountID: s.Target.SellerAccountID,
		TargetStoreName:       s.Target.StoreName,
		ResourceID:            postID,
		ResourceName:          model.Optional(truncateRunes(in.Preview, model.ResourceNameMaxRunes)),
		OperationReason:       reason,
		CaseReference:         s.CaseReference,
		RequestID:             key,
		CorrelationID:         key,
		IdempotencyKey:        key,
		RiskLevel:             string(model.AuditPostPublish.RiskLevel()),
		Now:                   m.d.Now(),
	})
	if errors.Is(err, pgx.ErrNoRows) {
		// 事件重送：同一把鍵的列已經在，它就是答案。
		return nil
	}
	if err != nil {
		return fmt.Errorf("delegation：寫入排程發布的稽核：%w", err)
	}
	return nil
}
