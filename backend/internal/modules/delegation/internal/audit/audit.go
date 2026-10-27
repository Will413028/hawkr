package audit

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"time"
	"unicode/utf8"

	"github.com/Will413028/hawkr/backend/internal/modules/delegation/internal/model"

	"github.com/jackc/pgx/v5"

	"github.com/Will413028/hawkr/backend/internal/modules/delegation/internal/store"
	"github.com/Will413028/hawkr/backend/internal/platform/events"
)

func toAuditLog(l store.DelegationAdminOperationAuditLog) model.AuditLog {
	return model.AuditLog{
		ID:                    l.ID,
		SessionID:             l.SessionID,
		Actor:                 model.DelegationActor{AdminID: l.ActorAdminID, DisplayName: l.ActorDisplayName},
		Target:                model.DelegationTarget{StoreID: l.TargetStoreID, SellerAccountID: l.TargetSellerAccountID, StoreName: l.TargetStoreName},
		Origin:                model.AuditOrigin(l.Origin),
		Capability:            model.Capability(l.Capability),
		Action:                model.AuditAction(l.Action),
		ResourceType:          model.AuditResourceType(l.ResourceType),
		ResourceID:            l.ResourceID,
		ResourceName:          l.ResourceName,
		Result:                model.AuditResult(l.Result),
		BizCode:               l.BizCode,
		OperationReason:       l.OperationReason,
		CaseReference:         l.CaseReference,
		RequestID:             l.RequestID,
		CorrelationID:         l.CorrelationID,
		IdempotencyKey:        l.IdempotencyKey,
		ResourceVersionBefore: l.ResourceVersionBefore,
		ResourceVersionAfter:  l.ResourceVersionAfter,
		ChangedFields:         decodeChangedFields(l.ChangedFields),
		UserAgent:             l.UserAgent,
		RiskLevel:             model.RiskLevel(l.RiskLevel),
		NotifySeller:          l.NotifySeller,
		NotifiedAt:            l.NotifiedAt,
		CreatedAt:             l.CreatedAt,
		ConfirmedAt:           l.ConfirmedAt,
	}
}

// storedChangedField 是 changed_fields 的 JSON 形狀（每個鍵都保留，空的 before 也是「這個欄位原本是空的」）。
type storedChangedField struct {
	Field  string `json:"field"`
	Label  string `json:"label"`
	Before string `json:"before"`
	After  string `json:"after"`
}

func encodeChangedFields(fs []model.ChangedField) ([]byte, error) {
	if len(fs) == 0 {
		return nil, nil
	}
	out := make([]storedChangedField, len(fs))
	for i, f := range fs {
		out[i] = storedChangedField(f)
	}
	return json.Marshal(out)
}

// decodeChangedFields 讀回變更的欄位。解析不了的列回空的（一列壞掉的資料不能讓整個稽核查詢失敗），沒有欄位名的
// 項目略過。
func decodeChangedFields(raw []byte) []model.ChangedField {
	if len(raw) == 0 {
		return nil
	}
	var stored []storedChangedField
	if err := json.Unmarshal(raw, &stored); err != nil {
		return nil
	}
	var out []model.ChangedField
	for _, f := range stored {
		if f.Field != "" {
			out = append(out, model.ChangedField(f))
		}
	}
	return out
}

// RecordDelegationAuditIntent 寫入 PENDING 的稽核列（intent）。session 必須還能用：已撤銷、已到期的 session 寫不進
// intent（排程執行例外：它的授權事實是 session ID 加 origin=scheduled，session 早就結束是預期的；撤銷仍然擋下）。
func (m *Module) RecordDelegationAuditIntent(ctx context.Context, in model.AuditIntentInput) (model.AuditIntent, error) {
	now := m.d.Now()
	q := m.q(ctx)
	s, err := m.d.Session.SessionForAudit(ctx, model.AuditSessionInput{ID: in.SessionID, Origin: in.Origin, Now: now})
	if err != nil {
		return model.AuditIntent{}, err
	}
	if err := validateIntent(in); err != nil {
		return model.AuditIntent{}, err
	}
	if in.IdempotencyKey != "" {
		if intent, ok, err := replayedIntent(ctx, q, in); err != nil || ok {
			return intent, err
		}
	}
	reason := model.Optional(in.OperationReason)
	if in.Origin == model.AuditScheduled {
		if reason, err = inheritedReason(ctx, q, in); err != nil {
			return model.AuditIntent{}, err
		}
	}
	var before []byte
	if len(in.BeforeSnapshot) > 0 {
		before = in.BeforeSnapshot
	}
	row, err := q.InsertDelegationAuditLog(ctx, store.InsertDelegationAuditLogParams{
		ID:                    m.d.NewID(),
		SessionID:             s.ID,
		ActorAdminID:          s.Actor.AdminID,
		ActorDisplayName:      s.Actor.DisplayName,
		TargetStoreID:         s.Target.StoreID,
		TargetSellerAccountID: s.Target.SellerAccountID,
		TargetStoreName:       s.Target.StoreName,
		Origin:                string(in.Origin),
		Capability:            string(in.Capability),
		Action:                string(in.Action),
		ResourceType:          string(in.ResourceType),
		ResourceID:            model.Optional(in.ResourceID),
		ResourceName:          model.Optional(truncateRunes(in.ResourceName, model.ResourceNameMaxRunes)),
		OperationReason:       reason,
		CaseReference:         s.CaseReference,
		RequestID:             in.RequestID,
		CorrelationID:         in.CorrelationID,
		IdempotencyKey:        model.Optional(in.IdempotencyKey),
		PayloadHash:           model.Optional(in.PayloadHash),
		BeforeSnapshot:        before,
		ResourceVersionBefore: in.ResourceVersionBefore,
		UserAgent:             model.Optional(in.UserAgent),
		RiskLevel:             string(in.Action.RiskLevel()),
		// notify_seller 只在 intent 寫入（confirm 只讀）；不能通知的動作在這裡強制為 false。
		NotifySeller: in.NotifySeller && in.Action.MayNotifySeller(),
		Now:          now,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		// 同一把鍵的另一個請求剛寫入：它的列就是答案。
		intent, ok, err := replayedIntent(ctx, q, in)
		if err == nil && !ok {
			err = errors.New("delegation：稽核 intent 撞到冪等鍵，卻讀不到那一列")
		}
		return intent, err
	}
	if err != nil {
		return model.AuditIntent{}, fmt.Errorf("delegation：寫入稽核 intent：%w", err)
	}
	return model.AuditIntent{AuditLogID: row.ID, Result: model.AuditResult(row.Result)}, nil
}

// replayedIntent 回傳同一把冪等鍵既有的列（ok 為 true）。冪等鍵綁定 (session、動作、資源、payload hash)（設計書 56
// §6.6.1；§5.2 的 uq_aoal_session_idempotency「重放時比對 payload_hash，不同即回衝突碼」）：同一把鍵用在不同的動作、
// 資源或內容上是 9267，不是重放。
func replayedIntent(ctx context.Context, q *store.Queries, in model.AuditIntentInput) (model.AuditIntent, bool, error) {
	l, err := q.GetDelegationAuditLogByIdempotencyKey(ctx, store.GetDelegationAuditLogByIdempotencyKeyParams{
		SessionID: in.SessionID, IdempotencyKey: model.Optional(in.IdempotencyKey),
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return model.AuditIntent{}, false, nil
	}
	if err != nil {
		return model.AuditIntent{}, false, fmt.Errorf("delegation：讀取稽核的冪等鍵：%w", err)
	}
	if !sameIntent(l, in) {
		return model.AuditIntent{}, false, model.Deny(model.DenialIdempotencyKeyReused, "冪等鍵已用於不同的請求內容")
	}
	return model.AuditIntent{AuditLogID: l.ID, Replayed: true, Result: model.AuditResult(l.Result)}, true, nil
}

// sameIntent 回傳既有的稽核列是不是同一個請求寫的。建立類在 intent 時沒有資源的 ID（confirm 才補上），重放時也不帶，
// 所以只有帶了才比。
func sameIntent(l store.DelegationAdminOperationAuditLog, in model.AuditIntentInput) bool {
	if l.Action != string(in.Action) || l.ResourceType != string(in.ResourceType) {
		return false
	}
	if in.ResourceID != "" && (l.ResourceID == nil || *l.ResourceID != in.ResourceID) {
		return false
	}
	hash := model.Optional(in.PayloadHash)
	return (l.PayloadHash == nil) == (hash == nil) && (hash == nil || *l.PayloadHash == *hash)
}

// usableDelegationSession 是 intent 的前提：session 存在而且還能用。
func validateIntent(in model.AuditIntentInput) error {
	switch {
	case !in.Action.Known():
		return model.Deny(model.DenialInvalidCommand, "action 不在已登錄的稽核動作內")
	case !in.ResourceType.Valid():
		return model.Deny(model.DenialInvalidCommand, "resourceType 不正確")
	case in.Origin != model.AuditInteractive && in.Origin != model.AuditScheduled:
		return model.Deny(model.DenialInvalidCommand, "origin 不正確")
	case in.Capability == "":
		return model.Deny(model.DenialInvalidCommand, "capability 不得為空")
	case in.OperationReason == "" && !in.Action.ReadDenial() && in.Origin != model.AuditScheduled:
		// 空字串存成 NULL 之後，理由就再也補不回來。
		return model.Deny(model.DenialInvalidCommand, "operationReason 為必填")
	case utf8.RuneCountInString(in.OperationReason) > 100:
		return model.Deny(model.DenialInvalidCommand, "operationReason 不得超過 100 字")
	}
	return nil
}

// inheritedReason 是排程執行的操作理由：同一個 session、同一個資源、建立排程（POST_SCHEDULE）那一列的理由。執行時
// 沒有請求可以帶理由，而寫入類的理由必填；找不到就拒絕（9279），不寫一列沒有理由的寫入。排程只有貼文；其他資源的
// 排程是沒有定義的路徑，同樣拒絕。
func inheritedReason(ctx context.Context, q *store.Queries, in model.AuditIntentInput) (*string, error) {
	if in.ResourceType != model.ResourcePost {
		return nil, model.Deny(model.DenialInvalidCommand, "origin=scheduled 不支援此 resourceType")
	}
	if in.ResourceID == "" {
		return nil, model.Deny(model.DenialInvalidCommand, "origin=scheduled 必須帶 resourceId")
	}
	reason, err := q.LatestDelegationAuditReason(ctx, store.LatestDelegationAuditReasonParams{
		SessionID: in.SessionID, ResourceID: in.ResourceID, Action: string(model.AuditPostSchedule),
	})
	if errors.Is(err, pgx.ErrNoRows) || (err == nil && reason == nil) {
		return nil, model.Deny(model.DenialAuditIntentFailed, "找不到可繼承操作理由的排程稽核紀錄")
	}
	if err != nil {
		return nil, fmt.Errorf("delegation：讀取排程的操作理由：%w", err)
	}
	return reason, nil
}

// truncateRunes 把 s 截到 n 個字。
func truncateRunes(s string, n int) string {
	if r := []rune(s); len(r) > n {
		return string(r[:n])
	}
	return s
}

// RecordDelegationAuditIntents 是批次的 intent，逐項成敗：每一項獨立寫入，一項失敗不影響其他項（語意等於呼叫 N 次
// 單筆；存在的理由是 gateway 的呼叫次數不能與筆數成正比）。
func (m *Module) RecordDelegationAuditIntents(ctx context.Context, items []model.AuditIntentInput) ([]model.AuditIntentOutcome, error) {
	if len(items) == 0 || len(items) > model.DelegationMaxBatch {
		return nil, model.Deny(model.DenialInvalidCommand, "items 必須為 1–%d 筆", model.DelegationMaxBatch)
	}
	out := make([]model.AuditIntentOutcome, len(items))
	for i, it := range items {
		out[i].Intent, out[i].Err = m.RecordDelegationAuditIntent(ctx, it)
	}
	return out, nil
}

// ConfirmDelegationAudit 把 PENDING 改成 SUCCEEDED 或 FAILED。已經確認過時回傳既有的列、不覆蓋（冪等）。成功而且
// intent 要求通知賣家時，同一個 transaction 內發出 DelegationOperationCompleted（原本寫 admin_operation_outbox）：
// 稽核的事實與送出通知的承諾一起 commit。
func (m *Module) ConfirmDelegationAudit(ctx context.Context, in model.AuditConfirmInput) (model.AuditLog, error) {
	var out model.AuditLog
	err := m.d.DB.InTx(ctx, func(ctx context.Context) error {
		var err error
		out, err = m.confirmAudit(ctx, m.q(ctx), in, m.d.Now())
		return err
	})
	return out, err
}

// ConfirmDelegationAudits 是批次的 confirm，全有或全無：一項失敗，所有列都留在 PENDING（合法的終態），沒有「部分
// 確認」的狀態。
func (m *Module) ConfirmDelegationAudits(ctx context.Context, items []model.AuditConfirmInput) ([]model.AuditLog, error) {
	if len(items) == 0 || len(items) > model.DelegationMaxBatch {
		return nil, model.Deny(model.DenialInvalidCommand, "items 必須為 1–%d 筆", model.DelegationMaxBatch)
	}
	var out []model.AuditLog
	err := m.d.DB.InTx(ctx, func(ctx context.Context) error {
		out = out[:0]
		q, now := m.q(ctx), m.d.Now()
		for _, it := range items {
			l, err := m.confirmAudit(ctx, q, it, now)
			if err != nil {
				return err
			}
			out = append(out, l)
		}
		return nil
	})
	return out, err
}

func (m *Module) confirmAudit(ctx context.Context, q *store.Queries, in model.AuditConfirmInput, now time.Time) (model.AuditLog, error) {
	if in.Result != model.AuditSucceeded && in.Result != model.AuditFailed {
		return model.AuditLog{}, model.Deny(model.DenialInvalidCommand, "result 必須是 SUCCEEDED 或 FAILED")
	}
	existing, err := q.GetDelegationAuditLog(ctx, in.AuditLogID)
	if errors.Is(err, pgx.ErrNoRows) {
		return model.AuditLog{}, model.Deny(model.DenialAuditIntentFailed, "找不到對應的稽核紀錄")
	}
	if err != nil {
		return model.AuditLog{}, fmt.Errorf("delegation：讀取稽核：%w", err)
	}
	// 成功的列，能力必須在 session 的快照內。失敗的列不檢查：它們記的是「有人要了沒有授予的能力」，正是安全稽核要的
	// 證據。
	var session model.DelegationSession
	if in.Result == model.AuditSucceeded {
		if session, err = m.d.Session.Session(ctx, existing.SessionID); err != nil {
			return model.AuditLog{}, fmt.Errorf("delegation：讀取代行：%w", err)
		}
		if !slices.Contains(session.Capabilities, model.Capability(existing.Capability)) {
			return model.AuditLog{}, model.Deny(model.DenialCapabilityDenied, "稽核列的 capability 不在該工作階段的授權快照內")
		}
	}
	changed, err := encodeChangedFields(in.ChangedFields)
	if err != nil {
		return model.AuditLog{}, fmt.Errorf("delegation：編碼變更的欄位：%w", err)
	}
	var bizCode *int32
	if in.Result == model.AuditFailed && in.BizCode != 0 {
		bizCode = &in.BizCode
	}
	var after []byte
	if len(in.AfterSnapshot) > 0 {
		after = in.AfterSnapshot
	}
	row, err := q.ConfirmDelegationAuditLog(ctx, store.ConfirmDelegationAuditLogParams{
		Result: string(in.Result), ResourceID: in.ResourceID, AfterSnapshot: after, ChangedFields: changed,
		ResourceVersionAfter: in.ResourceVersionAfter, BizCode: bizCode, Now: now, ID: in.AuditLogID,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		// 已經確認過：既有的結論不被覆蓋。
		if existing, err = q.GetDelegationAuditLog(ctx, in.AuditLogID); err != nil {
			return model.AuditLog{}, fmt.Errorf("delegation：讀取稽核：%w", err)
		}
		return toAuditLog(existing), nil
	}
	if err != nil {
		return model.AuditLog{}, fmt.Errorf("delegation：確認稽核：%w", err)
	}
	if row.NotifiedAt != nil {
		if err := events.Publish(ctx, m.d.Events, delegationCompleted(row, session)); err != nil {
			return model.AuditLog{}, err
		}
	}
	return toAuditLog(row), nil
}

func delegationCompleted(l store.DelegationAdminOperationAuditLog, s model.DelegationSession) model.DelegationOperationCompleted {
	action := model.AuditAction(l.Action)
	return model.DelegationOperationCompleted{
		AuditLogID:            l.ID,
		SessionID:             l.SessionID,
		SessionDisplayCode:    s.DisplayCode,
		ActorAdminID:          l.ActorAdminID,
		ActorDisplayName:      l.ActorDisplayName,
		TargetStoreID:         l.TargetStoreID,
		TargetSellerAccountID: l.TargetSellerAccountID,
		Capability:            model.Capability(l.Capability),
		Action:                action,
		ActionLabel:           action.Label(),
		RiskLevel:             model.RiskLevel(l.RiskLevel),
		ResourceType:          model.AuditResourceType(l.ResourceType),
		ResourceID:            l.ResourceID,
		ResourceName:          l.ResourceName,
		OperationReason:       l.OperationReason,
		CaseReference:         l.CaseReference,
		CorrelationID:         l.CorrelationID,
		OccurredAt:            l.CreatedAt,
		NotifiedAt:            *l.NotifiedAt,
	}
}
