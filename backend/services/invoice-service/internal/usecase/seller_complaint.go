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

// 本檔是 S10「賣家回覆買家申訴」的寫入面（§6.1.11 端點 36 ＝ §7.3.5 的 K02 與 K03）。
// 讀取面（端點 34／35）在 query_read.go。
//
// 五條約束：
//
//	① **回覆採追加事件、不覆蓋前次**（端點 36 定版①）：狀態轉移那一句一個內容欄都不碰，
//	   內容一律 append 進 invoice_complaint_events。所以它是 POST .../responses（建立子資源）
//	   而不是 PUT .../response。
//	② **三個 action 分成兩條邊**：RESENT／REISSUED 走 K02（→ SELLER_RESPONDED），
//	   NEED_PLATFORM_HELP 走 K03（→ ESCALATED_TO_ADM）。兩條邊的 allowed-from 不同
//	   （K02 收 OPEN／OVERDUE，K03 只收 OPEN），合成一句 2 值 allowed-from 會讓
//	   「逾期後才請求平台協助」靜默成功而狀態沒變。
//	③ **Tier A**：storeID 是 GetForSeller 的述詞之一（經 invoice_issuers 解析），越權與
//	   查無由同一行程式碼回同一個 9309。**不得**改成 GetByID ＋ Go 端比對。
//	④ **這一支對停權賣家放行**（§6.1.11）：申訴有 3 工作天 SLA、逾期即計違規、90 天內
//	   3 次即停權 ⇒ 鎖住它會製造「因為停權所以無法回覆，因為無法回覆所以繼續累積違規」
//	   的死結。放行由 Gateway 的能力層決定，本層不重複判斷，但也不得自行加嚴。
//	⑤ **不收任何佐證檔欄位**（端點 36 定版③）：request 沒有 attachment 欄，這是刻意的
//	   不是遺漏。時間軸表雖有 attachment_object_keys，本路徑一律寫空陣列。

const (
	// complaintActorSeller 是 invoice_complaint_events.actor_type 的賣家值
	// （chk_invcompev_actor 四值之一）。
	complaintActorSeller = "SELLER"

	// complaintBuyerReviewWindow 是 K02 寫進 buyer_review_due_at 的長度（§7.3.5 的 72 小時）。
	//
	// 🔴 少寫這一欄，該案永遠不會被 K06 自動結案，**而且沒有任何錯誤**：那一列只是從
	// 「逾期視為接受」的候選集合裡消失。它同時是 idx_invcomp_buyer_review_due 的鍵。
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
	// issuers 的用途與買家側 Complaint 相同：出向事件的 store_id 必須來自
	// invoice_issuers.issuer_ref_id。
	//
	// ⚠️ 不用 in.StoreID 就地填是刻意的：值今天相同（GetForSeller 的述詞就是
	// issuer_ref_id = store_id），但「事件上的 store_id 由哪張表決定」必須只有一個答案
	// —— 兩個來源就是兩個真相，而不一致時沒有任何東西會報錯。
	issuers domain.IssuerRepository
	outbox  domain.OutboxRepository
	// audit 是 I7 的寫入面（#3109）。賣家回覆／請求協助都是 status 轉移，
	// 而 complaint_events 是時間軸不是稽核表——兩者不可互相代替。
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
		// 🔴 GetForSeller 不是 GetByID：擁有者述詞在 SQL 的 WHERE，越權與查無由同一行
		// 程式碼回同一個 9309（§8.1.4(b)）。
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
			// K03：OPEN → ESCALATED_TO_ADM。allowed-from 只有 OPEN（在 SQL）。
			ok, err = u.complaints.EscalateBySeller(ctx, c.ID, in.StoreID, in.ExpectedVersion, now)
			nextStatus = codemeta.InvoiceComplaintStatusEscalatedToAdm.Value
		} else {
			// K02：OPEN／OVERDUE → SELLER_RESPONDED。
			// buyer_review_due_at 由本層算：它是 K06 唯一的觸發依據（見常數註解）。
			ok, err = u.complaints.MarkSellerResponded(ctx, c.ID, in.StoreID, in.ExpectedVersion,
				now.Add(complaintBuyerReviewWindow), now)
			nextStatus = codemeta.InvoiceComplaintStatusSellerResponded.Value
		}
		if err != nil {
			return err
		}
		// 🔴 rows=0 ＝ 守衛不成立（狀態不允許回覆／版本被搶先），不得當成功往下走。
		// §6.1.11 端點 36 逐字：狀態不允許回覆 → 9336。
		if !ok {
			return complaintBiz(bizcode.InvoiceComplaintActionNotAllowed)
		}

		// I7：留痕（#3109）。放在 CAS 成功之後、事件之前；nextStatus 是本層算出的
		// 目標值而不是重讀——K02／K03 的目標狀態由分支決定，SQL 不會另外算一個。
		if u.audit != nil && nextStatus != c.Status {
			// actor 是**賣家**，鍵沿用同檔 AppendEvent 的 in.StoreID（見 helpers 的紅字）。
			storeID := in.StoreID
			if err := appendStatusHistoryActor(ctx, u.audit, subjectComplaint, c.ID,
				c.Status, nextStatus, action, in.Note,
				complaintActorSeller, &storeID, now); err != nil {
				return err
			}
		}

		// 追加事件（約束①）。event_type 恆為 SELLER_REPLY——**含 NEED_PLATFORM_HELP**：
		// 那三個值是 InvoiceComplaintAction（「賣家點了哪一顆」），與 event_type
		// （「發生了什麼」）是同一張表的兩個正交欄位。
		//
		// ⚠️ 升級那一格的 event_type 取 ESCALATED 而不是 SELLER_REPLY：CodeMeta 對
		// ESCALATED 的描述逐字寫「actor_type = SELLER（請求協助，K03）」，它就是為這一格
		// 存在的；寫成 SELLER_REPLY 會讓 S09／A07 的時間軸看不出案件是在這一步離開賣家的。
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

		// P9 `invoice.complaint.responded`：通知買家「賣家回覆了，去看」。
		//
		// 🔴 payload 不放賣家填的自由文字（§6.3.3.3 P8/P9/P10 逐字）：那一欄可能含個資與
		// 情緒性內容，而通知只需要「有一則新回覆」。
		// 🔴 store_id 走 invoice_issuers.issuer_ref_id，不是 c.IssuerID（那是 PK）。
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

		// 重讀而不是就地拼裝：version、buyer_review_due_at、first_responded_at 的新值
		// 由 SQL 算出（version+1、COALESCE 保留第一次），Go 端拼一份等於維護第二套算式。
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

// calendarAround 讀一次鏡像，組出「已逾期 X 個工作天」所需區間的行事曆快照。
//
// 一個請求只讀一次（與 Complaint.calendarAround 同一條理由）：分兩次讀會讓倒數與逾期
// 天數在鏡像剛好被 INV.gov 覆寫的那一瞬間互相矛盾。
func (u *SellerComplaint) calendarAround(ctx context.Context, now time.Time) (domain.BusinessCalendar, error) {
	return loadBusinessCalendar(ctx, u.businessDay,
		now.AddDate(0, 0, -complaintCalendarLookbehind),
		now.AddDate(0, 0, complaintCalendarLookahead))
}

// normalizeSellerComplaintAction 只收賣家可選的三個值。
//
// 🔴 正面列舉三值而不是「不是 ADM 的三值就放行」：InvoiceComplaintAction 是 6 值的共用
// 碼表，日後長胖時放行式的寫法會把新的 ADM 動作靜默收進賣家端點。
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

// complaintRespondedEvent 是 invoice.complaint.responded 的 payload。
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
		// 冪等鍵形狀取自 36 號的 `INVCPL:{complaint_id}:{phase}`：少了 phase，
		// 同一件案子的 created／responded 兩則通知會被 notification-service 去重掉一則。
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
