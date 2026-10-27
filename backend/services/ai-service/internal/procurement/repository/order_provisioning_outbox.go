package repository

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/Will413028/hawkr/ai-service/internal/procurement/repository/gen"
)

// ErrOrderProvisioningOutboxNotFound 代表這筆 acceptance 還沒有建單命令。
var ErrOrderProvisioningOutboxNotFound = errors.New("procurement: order provisioning outbox row not found")

// OrderProvisioningOutboxEventOrderShellRequested 是本表目前唯一的 event_type，
// 值域同時由 chk_propo_event_type 表達。新增值時兩邊一起改。
const OrderProvisioningOutboxEventOrderShellRequested = "RFQ_ORDER_SHELL_REQUESTED"

// OrderProvisioningOutbox 是 acceptance → order shell 的投遞任務。
type OrderProvisioningOutbox struct {
	ID           int64
	AcceptanceID int64
	EventType    string
	Payload      json.RawMessage
	Status       string
	AttemptCount int32
	CreatedAt    time.Time
	CompletedAt  *time.Time
}

func toOrderProvisioningOutbox(row gen.ProcurementOrderProvisioningOutbox) OrderProvisioningOutbox {
	return OrderProvisioningOutbox{
		ID:           row.ID,
		AcceptanceID: row.AcceptanceID,
		EventType:    row.EventType,
		Payload:      row.Payload,
		Status:       row.Status,
		AttemptCount: row.AttemptCount,
		CreatedAt:    row.CreatedAt.Time,
		CompletedAt:  row.CompletedAt,
	}
}

// InsertOrderProvisioningOutbox 為一筆 acceptance 排一個建單命令。
//
// ⛔ 必須與 acceptance 的寫入在**同一個交易**內（doc 64 §6.2 選項 B）：§2-4 真正要防的是
// 「接受成功卻永遠沒有訂單」，而那由「acceptance 落庫則命令必然落庫」＋ outbox 的重試與
// terminal 狀態一起守住。分兩個交易寫就是把那個保證換成一句祈禱。
//
// 重送同一把冪等鑰匙時 acceptance 走 ON CONFLICT 回放既有列，本支也會跟著再跑一次；
// uq_propo_acceptance_event 把它擋在 DB，靜默略過即可，不得產生第二筆建單命令。
func (r *Repo) InsertOrderProvisioningOutbox(
	ctx context.Context, acceptanceID int64, payload json.RawMessage, now time.Time,
) error {
	return r.q(ctx).InsertProcurementOrderProvisioningOutbox(ctx, gen.InsertProcurementOrderProvisioningOutboxParams{
		ID:            r.newID(),
		AcceptanceID:  acceptanceID,
		EventType:     OrderProvisioningOutboxEventOrderShellRequested,
		Payload:       payload,
		NextAttemptAt: timestamptzParam(now),
	})
}

// FindOrderProvisioningOutboxByAcceptance 回這筆 acceptance 的建單命令。
func (r *Repo) FindOrderProvisioningOutboxByAcceptance(
	ctx context.Context, acceptanceID int64,
) (OrderProvisioningOutbox, error) {
	row, err := r.q(ctx).FindProcurementOrderProvisioningOutboxByAcceptance(ctx, gen.FindProcurementOrderProvisioningOutboxByAcceptanceParams{
		AcceptanceID: acceptanceID,
		EventType:    OrderProvisioningOutboxEventOrderShellRequested,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return OrderProvisioningOutbox{}, ErrOrderProvisioningOutboxNotFound
		}
		return OrderProvisioningOutbox{}, err
	}
	return toOrderProvisioningOutbox(row), nil
}
