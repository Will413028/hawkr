package repository

import (
	"context"
	"encoding/json"
	"errors"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/Will413028/hawkr/ai-service/internal/procurement/domain"
	"github.com/Will413028/hawkr/ai-service/internal/procurement/repository/gen"
	"github.com/Will413028/hawkr/go-pkg/codemeta"
	"github.com/Will413028/hawkr/go-pkg/identity"
)

// ErrInviteNotFound 是 GetPublicationInviteSellerView 找不到列時的哨兵錯誤。
var ErrInviteNotFound = errors.New("procurement: rfq publication invite not found")

// delivery_state 的四個值。值域同時由 chk_prpi_delivery_state 與 CodeMeta
// ProcurementRfqInviteDeliveryState 表達；這裡是 Go 側的單一來源，不要在別處寫字面值。
// 值直接取自生成的 codemeta.ProcurementRfqInviteDeliveryStateXxx.Value，而非常數表達式，
// 因此這裡必須用 var（不能是 const）。
var (
	InviteDeliveryStateNotSent = codemeta.ProcurementRfqInviteDeliveryStateNotSent.Value
	InviteDeliveryStateQueued  = codemeta.ProcurementRfqInviteDeliveryStateQueued.Value
	InviteDeliveryStateSent    = codemeta.ProcurementRfqInviteDeliveryStateSent.Value
	InviteDeliveryStateFailed  = codemeta.ProcurementRfqInviteDeliveryStateFailed.Value
)

// PublicationInvite 是一位收件人的不可變身分快照 ＋ 可變的投遞狀態。
type PublicationInvite struct {
	ID              int64
	PublicationID   int64
	BuyerID         int64
	Position        int
	Lane            domain.QuoteLane
	StoreID         int64
	ProspectID      int64
	DeliveryState   string
	DeliveryStateAt time.Time
}

type InsertPublicationInvitesInput struct {
	PublicationID int64
	BuyerID       identity.BuyerID
	// Recipients 的切片索引即 position——順序是契約的一部分（同
	// InsertPublicationApproval 對 approval recipients 的處理）。
	Recipients []domain.PublicationRecipient
	Now        time.Time
}

// InsertPublicationInvites 為每一位收件人寫一列 invite，起始 delivery_state 一律
// QUEUED。
//
// ⛔ 這裡不吸收唯一違反：撞到 uq_prpi_publication_store／_prospect 代表上游的去重壞了
// （candidate set 的 unique index 或 compileAudience），那是要讓整筆 execute rollback
// 的資料不變式違反，不是可以回放的冪等命中。
func (r *Repo) InsertPublicationInvites(ctx context.Context, in InsertPublicationInvitesInput) ([]PublicationInvite, error) {
	out := make([]PublicationInvite, 0, len(in.Recipients))
	q := r.q(ctx)
	for i, recipient := range in.Recipients {
		storeID, prospectID, err := recipientIdentityParams(recipient)
		if err != nil {
			return nil, err
		}
		row, err := q.InsertProcurementRfqPublicationInvite(ctx, gen.InsertProcurementRfqPublicationInviteParams{
			ID:              r.newID(),
			PublicationID:   in.PublicationID,
			BuyerID:         int64(in.BuyerID),
			Position:        int32(i),
			Lane:            string(recipient.Lane),
			StoreID:         storeID,
			ProspectID:      prospectID,
			DeliveryState:   InviteDeliveryStateQueued,
			DeliveryStateAt: timestamptzParam(in.Now),
		})
		if err != nil {
			return nil, err
		}
		out = append(out, toPublicationInviteEntity(row))
	}
	return out, nil
}

// ListPublicationInvites 依 position 讀回一個 publication 的收件人清單。
func (r *Repo) ListPublicationInvites(ctx context.Context, publicationID int64) ([]PublicationInvite, error) {
	rows, err := r.q(ctx).ListProcurementRfqPublicationInvites(ctx, publicationID)
	if err != nil {
		return nil, err
	}
	out := make([]PublicationInvite, 0, len(rows))
	for _, row := range rows {
		out = append(out, toPublicationInviteEntity(row))
	}
	return out, nil
}

func toPublicationInviteEntity(row gen.ProcurementRfqPublicationInvite) PublicationInvite {
	inv := PublicationInvite{
		ID:              row.ID,
		PublicationID:   row.PublicationID,
		BuyerID:         row.BuyerID,
		Position:        int(row.Position),
		Lane:            domain.QuoteLane(row.Lane),
		DeliveryState:   row.DeliveryState,
		DeliveryStateAt: row.DeliveryStateAt.Time,
	}
	if row.StoreID != nil {
		inv.StoreID = *row.StoreID
	}
	if row.ProspectID != nil {
		inv.ProspectID = *row.ProspectID
	}
	return inv
}

// InviteSellerView 是站外賣家看得到的 RFQ 摘要。
type InviteSellerView struct {
	QualityKnowledgeState string
	Item                  string
	ItemLabel             string
	Quantity              string
	Unit                  string
	Packaging             string
	DeliveryCity          string
	DeliveryDate          string
	ResponseDeadline      string
	BuyerNote             string
	BuyerRequestedRange   *domain.SellerFacingRequestedRange

	// PublicationVersion 是十進位字串（"" ＝ 未知）。站外業者正式送出報價時原樣帶
	// 回來做 CAS：把報價綁在「業者當時看到的那一版 RFQ」上。
	//
	// ⚠️ 它是**白名單的第 13 欄**，不是順手加的。判準：業者需要它才能安全地送出，
	// 而它本身只說「你看到的是第幾版」——不洩漏任何買家內容。
	// ⛔ seller_facing_payload_hash **不**加進來：那是我方的內部完整性摘要，交給
	// 站外的不受信任方只會多一個可被比對／重放的內部值，而它的驗證者本來就該是
	// server。代價是 CAS 退化成 version-only——擋得住「買家改版後才送出」，擋不住
	// 「payload 變了但 version 沒 bump」，而後者本身是 producer 端的 bug。
	PublicationVersion string
}

// GetPublicationInviteSellerView 依 invite id 讀回賣家可見的 RFQ 摘要。
func (r *Repo) GetPublicationInviteSellerView(ctx context.Context, inviteID int64) (InviteSellerView, error) {
	row, err := r.q(ctx).GetPublicationInviteSellerView(ctx, inviteID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return InviteSellerView{}, ErrInviteNotFound
		}
		return InviteSellerView{}, err
	}

	return toInviteSellerView(row.SellerFacingPayloadJson, row.ResponseDeadline, row.PublicationVersion)
}

// toInviteSellerView 是 seller_facing_payload_json ＋ response_deadline 到白名單的
// 唯一轉換。具名共用而不是各呼叫端抄一次：賣家詳情與賣家清單看到的必須是同一份
// 內容，兩份實作可以各自漂移。
func toInviteSellerView(payloadJSON json.RawMessage, responseDeadline pgtype.Date, publicationVersion int32) (InviteSellerView, error) {
	var payload domain.SellerFacingPayload
	if err := json.Unmarshal(payloadJSON, &payload); err != nil {
		return InviteSellerView{}, err
	}
	return InviteSellerView{
		QualityKnowledgeState: payload.QualityKnowledgeState,
		Item:                  payload.Item,
		ItemLabel:             payload.ItemLabel,
		Quantity:              intPtrValue(payload.Quantity),
		Unit:                  stringValue(payload.Unit),
		Packaging:             stringValue(payload.Packaging),
		DeliveryCity:          stringValue(payload.DeliveryCity),
		DeliveryDate:          stringValue(payload.DeliveryDate),
		ResponseDeadline:      stringValue(dateValue(responseDeadline)),
		BuyerNote:             stringValue(payload.BuyerNote),
		BuyerRequestedRange:   payload.BuyerRequestedRange,
		PublicationVersion:    strconv.FormatInt(int64(publicationVersion), 10),
	}, nil
}

func intPtrValue(value *int) string {
	if value == nil {
		return ""
	}
	return strconv.Itoa(*value)
}
