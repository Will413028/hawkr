package usecase

import (
	"context"
	"encoding/json"
	"errors"
	"strconv"
	"time"
	"unicode/utf8"

	"github.com/Will413028/hawkr/go-pkg/codemeta"
	"github.com/Will413028/hawkr/go-pkg/database"
	"github.com/Will413028/hawkr/go-pkg/errors/bizcode"
	pkgmq "github.com/Will413028/hawkr/go-pkg/mq"
	"github.com/Will413028/hawkr/go-pkg/timeutil"
	"github.com/Will413028/hawkr/invoice-service/internal/domain"
	"github.com/Will413028/hawkr/invoice-service/internal/eventwire"
	"go.uber.org/zap"
)


const (
	complaintActorSeller = "SELLER"

	complaintBuyerReviewWindow = 72 * time.Hour
)

// ErrComplaintActionInvalid 是 action 不在賣家可選三值內。
//
// 它是**契約違規不是業務結果** ⇒ 400，不借 9336：9336 的語意是「這個動作在目前狀態
// 下不允許」，而值域外的字串連「是哪個動作」都答不出來。
var ErrComplaintActionInvalid = errors.New(
	"invoice: action must be one of RESENT / REISSUED / NEED_PLATFORM_HELP")

// SellerComplaint 是 S10 的寫入 usecase。
//
// 它與 Complaint（買家側）分開的理由不是「賣家 vs 買家」而是**歸屬鍵不同**：買家側的
// 述詞是 complainant_buyer_id、賣家側是經 invoice_issuers 解析的 store_id。共用一個
// 型別會讓兩個歸屬鍵在同一組方法上流動，而那正是 Tier A 最容易破的地方。
type SellerComplaint struct {
	tx         database.TxManager
	complaints domain.ComplaintRepository
	issuers domain.IssuerRepository
	outbox  domain.OutboxRepository
	audit       domain.AuditRepository
	businessDay domain.BusinessDayRepository

	now func() time.Time
	log *zap.Logger
}

// NewSellerComplaint 組出 S10 的寫入 usecase。
func NewSellerComplaint(
	tx database.TxManager,
	complaints domain.ComplaintRepository,
	issuers domain.IssuerRepository,
	outbox domain.OutboxRepository,
	businessDay domain.BusinessDayRepository,
	audit domain.AuditRepository,
	log *zap.Logger,
) *SellerComplaint {
	return &SellerComplaint{
		tx: tx, complaints: complaints, issuers: issuers, outbox: outbox, businessDay: businessDay,
		audit: audit,
		now:   func() time.Time { return time.Now().UTC() },
		log:   orNop(log),
	}
}

// WithClock 注入時間來源（測試用）。
func (u *SellerComplaint) WithClock(now func() time.Time) *SellerComplaint {
	if now != nil {
		u.now = now
	}
	return u
}

// RespondComplaintInput 是端點 36 的輸入。
type RespondComplaintInput struct {
	StoreID         int64
	ComplaintID     int64
	Action          string
	Note            string
	ExpectedVersion int64
	IdempotencyKey  string
}

// Respond 讓賣家回覆一件申訴（端點 36）。
//
// 🔴 NEED_PLATFORM_HELP **必須是合法選項**（端點 36 定版②）：它把案件推進
// ESCALATED_TO_ADM，在賣家側的 S09 歸入「已回覆」分頁。把它做成「不算回覆」的後果是
// 賣家請求平台協助之後仍被 SLA 計為逾期。
//
// 🔴 NEED_PLATFORM_HELP 必填說明（§7.3.5 K03）；另外兩個動作的 note 可空。
func (u *SellerComplaint) Respond(ctx context.Context, in RespondComplaintInput) (ComplaintView, error) {
	action, err := normalizeSellerComplaintAction(in.Action)
	if err != nil {
		return ComplaintView{}, err
	}
	if utf8.RuneCountInString(in.Note) > complaintTextMaxRunes {
		return ComplaintView{}, ErrComplaintReasonTooLong
	}
	escalate := action == codemeta.InvoiceComplaintActionNeedPlatformHelp.Value
	if escalate && isBlank(in.Note) {
		return ComplaintView{}, ErrComplaintReasonRequired
	}

	var out ComplaintView
	err = u.tx.WithTx(ctx, func(ctx context.Context) error {
		c, err := u.complaints.GetForSeller(ctx, in.ComplaintID, in.StoreID)
		if err != nil {
			return err
		}
		if c.Version != in.ExpectedVersion {
			return complaintVersionConflict()
		}

		now := u.now()
		var (
			ok         bool
			nextStatus string
		)
		if escalate {
			ok, err = u.complaints.EscalateBySeller(ctx, c.ID, in.StoreID, in.ExpectedVersion, now)
			nextStatus = codemeta.InvoiceComplaintStatusEscalatedToAdm.Value
		} else {
			ok, err = u.complaints.MarkSellerResponded(ctx, c.ID, in.StoreID, in.ExpectedVersion,
				now.Add(complaintBuyerReviewWindow), now)
			nextStatus = codemeta.InvoiceComplaintStatusSellerResponded.Value
		}
		if err != nil {
			return err
		}
		if !ok {
			return complaintBiz(bizcode.InvoiceComplaintActionNotAllowed)
		}

		if u.audit != nil && nextStatus != c.Status {
			storeID := in.StoreID
			if err := appendStatusHistoryActor(ctx, u.audit, subjectComplaint, c.ID,
				c.Status, nextStatus, action, in.Note,
				complaintActorSeller, &storeID, now); err != nil {
				return err
			}
		}

		eventType := codemeta.InvoiceComplaintEventTypeSellerReply.Value
		if escalate {
			eventType = codemeta.InvoiceComplaintEventTypeEscalated.Value
		}
		if _, aerr := u.complaints.AppendEvent(ctx, domain.NewComplaintEvent{
			ComplaintID:    c.ID,
			EventType:      eventType,
			Action:         action,
			ActorType:      complaintActorSeller,
			ActorAccountID: in.StoreID,
			Message:        in.Note,
			Now:            now,
		}); aerr != nil {
			return aerr
		}

		issuer, ierr := u.issuers.GetByID(ctx, c.IssuerID)
		if ierr != nil {
			return ierr
		}
		payload, perr := sellerComplaintRespondedPayload(issuer, c, action, nextStatus, now)
		if perr != nil {
			return perr
		}
		if _, oerr := u.outbox.Insert(ctx, domain.NewOutboxEvent{
			AggregateType: outboxAggregateComplaint,
			AggregateID:   c.ID,
			EventType:     string(pkgmq.EventTypeInvoiceComplaintResponded),
			Exchange:      pkgmq.ExchangeInvoice,
			RoutingKey:    pkgmq.RoutingKeyInvoiceComplaintResponded,
			Payload:       payload,
			NextAttemptAt: now,
			Now:           now,
		}); oerr != nil {
			return oerr
		}

		updated, err := u.complaints.GetForSeller(ctx, in.ComplaintID, in.StoreID)
		if err != nil {
			return err
		}
		cal, err := u.calendarAround(ctx, now)
		if err != nil {
			return err
		}
		events, err := u.complaints.ListEvents(ctx, updated.ID)
		if err != nil {
			return err
		}
		out = viewWith(updated, events, cal, now)
		return nil
	})
	if err != nil {
		return ComplaintView{}, err
	}
	return out, nil
}

func (u *SellerComplaint) calendarAround(ctx context.Context, now time.Time) (domain.BusinessCalendar, error) {
	return loadBusinessCalendar(ctx, u.businessDay,
		now.AddDate(0, 0, -complaintCalendarLookbehind),
		now.AddDate(0, 0, complaintCalendarLookahead))
}

func normalizeSellerComplaintAction(action string) (string, error) {
	switch action {
	case codemeta.InvoiceComplaintActionResent.Value,
		codemeta.InvoiceComplaintActionReissued.Value,
		codemeta.InvoiceComplaintActionNeedPlatformHelp.Value:
		return action, nil
	default:
		return "", ErrComplaintActionInvalid
	}
}

type complaintRespondedEvent struct {
	NotificationRefID string            `json:"notification_ref_id"`
	ComplaintID       string            `json:"complaint_id"`
	InvoiceDocumentID string            `json:"invoice_document_id"`
	OrderNo           string            `json:"order_no"`
	BuyerID           eventwire.BuyerID `json:"buyer_id"`
	StoreID           eventwire.StoreID `json:"store_id"`
	Action            string            `json:"action"`
	Status            string            `json:"status"`
	OccurredAt        string            `json:"occurred_at"`
}

func sellerComplaintRespondedPayload(issuer domain.Issuer, c domain.Complaint, action, status string, now time.Time) (json.RawMessage, error) {
	return json.Marshal(complaintRespondedEvent{
		NotificationRefID: "INVCPL:" + strconv.FormatInt(c.ID, 10) + ":responded",
		ComplaintID:       strconv.FormatInt(c.ID, 10),
		InvoiceDocumentID: strconv.FormatInt(c.DocumentID, 10),
		OrderNo:           derefString(c.OrderNo),
		BuyerID:           eventwire.BuyerIDOfComplaint(c),
		StoreID:           eventwire.StoreIDOf(issuer),
		Action:            action,
		Status:            status,
		OccurredAt:        timeutil.FormatISO8601(now),
	})
}
