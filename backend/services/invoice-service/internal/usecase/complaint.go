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
	pkgerrors "github.com/Will413028/hawkr/go-pkg/errors"
	"github.com/Will413028/hawkr/go-pkg/errors/bizcode"
	pkgmq "github.com/Will413028/hawkr/go-pkg/mq"
	"github.com/Will413028/hawkr/go-pkg/timeutil"
	"github.com/Will413028/hawkr/invoice-service/internal/domain"
	"github.com/Will413028/hawkr/invoice-service/internal/eventwire"
	"go.uber.org/zap"
)


const (
	complaintTextMaxRunes = 500

	complaintWithdrawQuota = 2

	complaintCalendarLookahead  = domain.ComplaintSlaHardCapDays + 1
	complaintCalendarLookbehind = 30
)

// 四個型別邊界錯誤。它們是**契約違規不是業務結果**（§6.1 端點表逐字的「400」）
// ⇒ 呼叫端映成 InvalidArgument，不是業務碼——沒有任何一個業務碼描述「你的字太長」，
// 借一個近似的碼會讓前端把它顯示成別的意思。
var (
	ErrComplaintDescriptionTooLong = errors.New("invoice: complaint description exceeds 500 characters")
	ErrComplaintReasonRequired     = errors.New("invoice: rejection reason is required")
	ErrComplaintReasonTooLong      = errors.New("invoice: reason exceeds 500 characters")
	ErrComplaintReasonCodeInvalid  = errors.New("invoice: reason_code is outside the Phase 1 value set")
)

// Complaint 是 B07 的寫入 usecase。
//
// 它持有 TxManager（約束③）與 outbox（K01 要通知賣家），而 Query 兩者都沒有——
// 那正是兩者分開的理由：把寫入掛進 Query，「唯讀查詢面」這句話就變成假的。
type Complaint struct {
	tx         database.TxManager
	complaints domain.ComplaintRepository
	documents  domain.DocumentRepository
	issuers     domain.IssuerRepository
	outbox      domain.OutboxRepository
	businessDay domain.BusinessDayRepository
	audit domain.AuditRepository

	newID func() int64
	now   func() time.Time
	log   *zap.Logger
}

const subjectComplaint = "COMPLAINT"

func NewComplaint(
	tx database.TxManager,
	complaints domain.ComplaintRepository,
	documents domain.DocumentRepository,
	issuers domain.IssuerRepository,
	outbox domain.OutboxRepository,
	businessDay domain.BusinessDayRepository,
	audit domain.AuditRepository,
	newID func() int64,
	log *zap.Logger,
) *Complaint {
	return &Complaint{
		tx:          tx,
		complaints:  complaints,
		documents:   documents,
		issuers:     issuers,
		outbox:      outbox,
		businessDay: businessDay,
		audit:       audit,
		newID:       newID,
		now:         func() time.Time { return time.Now().UTC() },
		log:         log,
	}
}

// WithClock 注入時間來源（測試用）。
func (u *Complaint) WithClock(now func() time.Time) *Complaint {
	if now != nil {
		u.now = now
	}
	return u
}

// FileComplaintInput 是端點 13 的輸入。
type FileComplaintInput struct {
	BuyerID     int64
	OrderNo     string
	ReasonCode  string
	Description string
}

// ComplaintActionInput 是端點 63／64／65 的共通輸入。
//
// OrderNo 是 Tier A 的第二段述詞：路徑上的訂單與案件不匹配時走**同一個** 9309 出口
// （§8.1.4(c)：不得為它另開一個碼，那等於在回應裡放一個「這件案子存在，只是不在
// 這張訂單下」的旗標）。
type ComplaintActionInput struct {
	BuyerID         int64
	OrderNo         string
	ComplaintID     int64
	ExpectedVersion int64
	Reason          string
}

// FileComplaintResult 是端點 13 的輸出。
//
// Duplicate=true 代表這張訂單已有進行中的案件：回應是 **9310 ＋ 既有案件的完整單筆**
// （HTTP 200，§6.1.6 第 2 條），不是錯誤——前端關掉 Dialog 並把畫面捲到既有案件。
type FileComplaintResult struct {
	View      ComplaintView
	Duplicate bool
}

// File 建立一件申訴（端點 13 ＝ K01）。
//
// 順序是刻意的：格式驗證 → 開交易 → 文件與時間閘 → 兩個守衛 → 算 SLA → 寫入。
// 格式錯的請求不該開交易；而兩個守衛必須與寫入同交易，否則「先查再寫」的窗口會讓
// 同一張訂單建出第二件案子（DB 的 partial unique 是最後一道，但它只回 23505，
// 前端分不出是重複申訴還是撤回額度用盡）。
func (u *Complaint) File(ctx context.Context, in FileComplaintInput) (FileComplaintResult, error) {
	reasonCode, err := normalizeComplaintReasonCode(in.ReasonCode)
	if err != nil {
		return FileComplaintResult{}, err
	}
	if utf8.RuneCountInString(in.Description) > complaintTextMaxRunes {
		return FileComplaintResult{}, ErrComplaintDescriptionTooLong
	}

	var out FileComplaintResult
	err = u.tx.WithTx(ctx, func(ctx context.Context) error {
		docs, err := u.documents.ListOrderDocumentsForBuyer(ctx, in.OrderNo, in.BuyerID)
		if err != nil {
			return err
		}
		if len(docs) == 0 {
			return documentNotFound()
		}

		now := u.now()
		if now.Before(complaintOpensAt(docs)) {
			return complaintBiz(bizcode.InvoiceComplaintNotOpenYet)
		}

		cal, err := u.calendarAround(ctx, now)
		if err != nil {
			return err
		}

		target := docs[0]

		if existing, found, err := findOpenComplaint(ctx, u.complaints, in.BuyerID, in.OrderNo); err != nil {
			return err
		} else if found {
			view, err := u.viewOf(ctx, existing, cal, now)
			if err != nil {
				return err
			}
			out = FileComplaintResult{View: view, Duplicate: true}
			return nil
		}

		withdrawn, err := u.complaints.CountWithdrawnByDocument(ctx, target.ID)
		if err != nil {
			return err
		}
		if withdrawn >= complaintWithdrawQuota {
			return complaintBiz(bizcode.InvoiceComplaintActionNotAllowed)
		}

		slaDueAt := u.slaDueAt(cal, now)

		id := u.newID()
		row, inserted, err := u.complaints.Create(ctx, domain.NewComplaint{
			ID:                 id,
			DocumentID:         target.ID,
			IssuerID:           target.IssuerID,
			OrderNo:            in.OrderNo,
			ComplainantBuyerID: in.BuyerID,
			ReasonCode:         reasonCode,
			Description:        in.Description,
			SlaDueAt:           slaDueAt,
			Now:                now,
		})
		if err != nil {
			return err
		}
		if !inserted {
			existing, found, err := findOpenComplaint(ctx, u.complaints, in.BuyerID, in.OrderNo)
			if err != nil {
				return err
			}
			if !found {
				return errComplaintIndexDrift
			}
			view, err := u.viewOf(ctx, existing, cal, now)
			if err != nil {
				return err
			}
			out = FileComplaintResult{View: view, Duplicate: true}
			return nil
		}

		event, err := u.complaints.AppendEvent(ctx, domain.NewComplaintEvent{
			ComplaintID: row.ID,
			EventType:   codemeta.InvoiceComplaintEventTypeCreated.Value,
			ActorType:      complaintActorBuyer,
			ActorAccountID: in.BuyerID,
			Message:        in.Description,
			Now:            now,
		})
		if err != nil {
			return err
		}

		issuer, err := u.issuers.GetByID(ctx, row.IssuerID)
		if err != nil {
			return err
		}
		payload, err := complaintCreatedPayload(issuer, row, target, now)
		if err != nil {
			return err
		}
		if _, err := u.outbox.Insert(ctx, domain.NewOutboxEvent{
			AggregateType: outboxAggregateComplaint,
			AggregateID:   row.ID,
			EventType:     string(pkgmq.EventTypeInvoiceComplaintCreated),
			Exchange:      pkgmq.ExchangeInvoice,
			RoutingKey:    pkgmq.RoutingKeyInvoiceComplaintCreated,
			Payload:       payload,
			NextAttemptAt: now,
			Now:           now,
		}); err != nil {
			return err
		}

		out = FileComplaintResult{View: viewWith(row, []domain.ComplaintEvent{event}, cal, now)}
		return nil
	})
	if err != nil {
		return FileComplaintResult{}, err
	}
	return out, nil
}

// Reject 是端點 63（K07）：買家不接受賣家的回覆，案件轉由平台判定。
//
// 沒有時間守衛是刻意的（§6.1.6 端點 63 逐字）：buyer_review_due_at 到期時該列已被
// K06 推到 RESOLVED，狀態 CAS 本身就隱含了它。多寫一條時間述詞會出現
// 「狀態還在 SELLER_RESPONDED 卻說你來不及了」。
func (u *Complaint) Reject(ctx context.Context, in ComplaintActionInput) (ComplaintView, error) {
	if in.Reason == "" {
		return ComplaintView{}, ErrComplaintReasonRequired
	}
	if utf8.RuneCountInString(in.Reason) > complaintTextMaxRunes {
		return ComplaintView{}, ErrComplaintReasonTooLong
	}
	return u.transition(ctx, in, complaintTransition{
		eventType: codemeta.InvoiceComplaintEventTypeEscalated.Value,
		apply: func(ctx context.Context, _ domain.BusinessCalendar, now time.Time) (bool, error) {
			return u.complaints.EscalateByBuyer(ctx, in.ComplaintID, in.BuyerID, in.ExpectedVersion, now)
		},
	})
}

// Withdraw 是端點 64（K11）：買家撤回。
//
// **沒有 reason 欄**（§7 K11 無此守衛）：強制買家為「我開錯單了」寫一段話只會逼出
// 一堆「.」。端點層不自行加嚴——看到別的動作有 reason 就順手補一個，等於改了 §7。
func (u *Complaint) Withdraw(ctx context.Context, in ComplaintActionInput) (ComplaintView, error) {
	return u.transition(ctx, in, complaintTransition{
		eventType: codemeta.InvoiceComplaintEventTypeWithdrawn.Value,
		apply: func(ctx context.Context, _ domain.BusinessCalendar, now time.Time) (bool, error) {
			return u.complaints.Withdraw(ctx, in.ComplaintID, in.BuyerID, in.ExpectedVersion, now)
		},
	})
}

// Reopen 是端點 65（K13）：買家重開一件已解決的案件。
//
// 三條守衛（狀態、reopen_count < 2、冷卻已過）全部在 SQL 的同一句 WHERE 讀舊值，
// 本層**不重複判**：重複判會多出一條「Go 說可以、SQL 說不行」的分歧，而且兩個分頁
// 同時按下時應用層的 if 各自讀到 0 再各自寫 1。
func (u *Complaint) Reopen(ctx context.Context, in ComplaintActionInput) (ComplaintView, error) {
	if utf8.RuneCountInString(in.Reason) > complaintTextMaxRunes {
		return ComplaintView{}, ErrComplaintReasonTooLong
	}
	return u.transition(ctx, in, complaintTransition{
		eventType: codemeta.InvoiceComplaintEventTypeReopened.Value,
		apply: func(ctx context.Context, cal domain.BusinessCalendar, now time.Time) (bool, error) {
			return u.complaints.Reopen(ctx, in.ComplaintID, in.BuyerID, in.ExpectedVersion,
				u.slaDueAt(cal, now), now)
		},
	})
}

type complaintTransition struct {
	eventType string
	apply     func(ctx context.Context, cal domain.BusinessCalendar, now time.Time) (bool, error)
}

func (u *Complaint) transition(ctx context.Context, in ComplaintActionInput, t complaintTransition) (ComplaintView, error) {
	var out ComplaintView
	err := u.tx.WithTx(ctx, func(ctx context.Context) error {
		c, err := u.complaints.GetForBuyer(ctx, in.ComplaintID, in.BuyerID)
		if err != nil {
			return err
		}
		if in.OrderNo != "" && derefString(c.OrderNo) != in.OrderNo {
			return complaintNotFound()
		}
		if c.Version != in.ExpectedVersion {
			return complaintVersionConflict()
		}

		now := u.now()
		cal, err := u.calendarAround(ctx, now)
		if err != nil {
			return err
		}
		ok, err := t.apply(ctx, cal, now)
		if err != nil {
			return err
		}
		if !ok {
			return complaintBiz(bizcode.InvoiceComplaintActionNotAllowed)
		}

		if _, err := u.complaints.AppendEvent(ctx, domain.NewComplaintEvent{
			ComplaintID:    c.ID,
			EventType:      t.eventType,
			ActorType:      complaintActorBuyer,
			ActorAccountID: in.BuyerID,
			Message:        in.Reason,
			Now:            now,
		}); err != nil {
			return err
		}

		updated, err := u.complaints.GetForBuyer(ctx, in.ComplaintID, in.BuyerID)
		if err != nil {
			return err
		}

		if u.audit != nil && updated.Status != c.Status {
			buyerID := in.BuyerID
			if err := appendStatusHistoryActor(ctx, u.audit, subjectComplaint, c.ID,
				c.Status, updated.Status, t.eventType, in.Reason,
				complaintActorBuyer, &buyerID, now); err != nil {
				return err
			}
		}
		out, err = u.viewOf(ctx, updated, cal, now)
		return err
	})
	if err != nil {
		return ComplaintView{}, err
	}
	return out, nil
}


func (u *Complaint) calendarAround(ctx context.Context, now time.Time) (domain.BusinessCalendar, error) {
	return loadBusinessCalendar(ctx, u.businessDay,
		now.AddDate(0, 0, -complaintCalendarLookbehind),
		now.AddDate(0, 0, complaintCalendarLookahead))
}

func (u *Complaint) slaDueAt(cal domain.BusinessCalendar, now time.Time) time.Time {
	if !cal.Covers(now, now.AddDate(0, 0, domain.ComplaintSlaHardCapDays)) {
		u.logger().Warn("invoice.complaint.business_calendar_stale")
	}
	return cal.ComplaintSlaDueAt(now)
}

func loadBusinessCalendar(ctx context.Context, repo domain.BusinessDayRepository, from, to time.Time) (domain.BusinessCalendar, error) {
	if from.After(to) {
		from, to = to, from
	}
	holidays, coverage, err := repo.Snapshot(ctx, from.AddDate(0, 0, -1), to.AddDate(0, 0, 1))
	if err != nil {
		return domain.BusinessCalendar{}, err
	}
	return domain.NewBusinessCalendar(holidays, coverage), nil
}

func (u *Complaint) viewOf(ctx context.Context, c domain.Complaint, cal domain.BusinessCalendar, now time.Time) (ComplaintView, error) {
	events, err := u.complaints.ListEvents(ctx, c.ID)
	if err != nil {
		return ComplaintView{}, err
	}
	return viewWith(c, events, cal, now), nil
}

func viewWith(c domain.Complaint, events []domain.ComplaintEvent, cal domain.BusinessCalendar, now time.Time) ComplaintView {
	view := ComplaintView{Complaint: c, Events: events}
	applyComplaintCountdown(&view, cal, now)
	return view
}

func (u *Complaint) logger() *zap.Logger {
	if u.log == nil {
		return zap.NewNop()
	}
	return u.log
}


const complaintActorBuyer = "BUYER"

const outboxAggregateComplaint = "COMPLAINT"

const complaintReasonNotReceived = "NOT_RECEIVED"

func normalizeComplaintReasonCode(code string) (string, error) {
	if code == "" || code == complaintReasonNotReceived {
		return complaintReasonNotReceived, nil
	}
	return "", ErrComplaintReasonCodeInvalid
}

var errComplaintIndexDrift = errors.New("invoice: complaint insert was rejected by the open-per-document index but no open case was found")

func complaintOpensAt(docs []domain.Document) time.Time {
	return docs[len(docs)-1].CreatedAt.Add(complaintOpenDelay)
}

func findOpenComplaint(ctx context.Context, repo domain.ComplaintRepository, buyerID int64, orderNo string) (domain.Complaint, bool, error) {
	rows, err := repo.ListForBuyerOrder(ctx, domain.BuyerComplaintQuery{
		BuyerID: buyerID,
		OrderNo: orderNo,
		Page:    domain.Page{Limit: orderComplaintScanLimit},
	})
	if err != nil {
		return domain.Complaint{}, false, err
	}
	for _, c := range rows {
		if containsString(openComplaintStatuses, c.Status) {
			return c, true, nil
		}
	}
	return domain.Complaint{}, false, nil
}

func applyComplaintCountdown(v *ComplaintView, cal domain.BusinessCalendar, now time.Time) {
	remaining, ok := cal.RemainingBusinessDays(now, v.Complaint.SlaDueAt)
	if !ok {
		return
	}
	r := remaining
	v.RemainingBusinessDays = &r
	overdue := int32(0)
	if remaining < 0 {
		overdue = -remaining
	}
	v.OverdueBusinessDays = &overdue
}

func complaintBiz(code int32) error {
	return pkgerrors.NewBusiness(code, bizcode.Message(code))
}

func complaintVersionConflict() error {
	return pkgerrors.NewBusiness(bizcode.SellerTaxProfileVersionConflict,
		bizcode.Message(bizcode.SellerTaxProfileVersionConflict))
}

type complaintCreatedEvent struct {
	NotificationRefID string            `json:"notification_ref_id"`
	ComplaintID       string            `json:"complaint_id"`
	InvoiceDocumentID string            `json:"invoice_document_id"`
	OrderNo           string            `json:"order_no"`
	BuyerID           eventwire.BuyerID `json:"buyer_id"`
	StoreID           eventwire.StoreID `json:"store_id"`
	ReasonCode        string            `json:"reason_code"`
	SlaDueAt          string            `json:"sla_due_at"`
	OccurredAt        string            `json:"occurred_at"`
}

func complaintCreatedPayload(issuer domain.Issuer, c domain.Complaint, doc domain.Document, now time.Time) (json.RawMessage, error) {
	return json.Marshal(complaintCreatedEvent{
		NotificationRefID: "INVCPL:" + strconv.FormatInt(c.ID, 10) + ":created",
		ComplaintID:       strconv.FormatInt(c.ID, 10),
		InvoiceDocumentID: strconv.FormatInt(doc.ID, 10),
		OrderNo:           derefString(c.OrderNo),
		BuyerID:           eventwire.BuyerIDOfComplaint(c),
		StoreID:           eventwire.StoreIDOf(issuer),
		ReasonCode:        c.ReasonCode,
		SlaDueAt:          timeutil.FormatISO8601(c.SlaDueAt),
		OccurredAt:        timeutil.FormatISO8601(now),
	})
}


type complaintOverdueEvent struct {
	NotificationRefID string            `json:"notification_ref_id"`
	ComplaintID       string            `json:"complaint_id"`
	InvoiceDocumentID string            `json:"invoice_document_id"`
	OrderNo           string            `json:"order_no"`
	BuyerID           eventwire.BuyerID `json:"buyer_id"`
	StoreID           eventwire.StoreID `json:"store_id"`
	SlaDueAt          string            `json:"sla_due_at"`
	OccurredAt        string            `json:"occurred_at"`
}

func complaintOverduePayload(issuer domain.Issuer, c domain.Complaint, now time.Time) (json.RawMessage, error) {
	return json.Marshal(complaintOverdueEvent{
		NotificationRefID: "INVCPL:" + strconv.FormatInt(c.ID, 10) + ":overdue",
		ComplaintID:       strconv.FormatInt(c.ID, 10),
		InvoiceDocumentID: strconv.FormatInt(c.DocumentID, 10),
		OrderNo:           derefString(c.OrderNo),
		BuyerID:           eventwire.BuyerIDOfComplaint(c),
		StoreID:           eventwire.StoreIDOf(issuer),
		SlaDueAt:          timeutil.FormatISO8601(c.SlaDueAt),
		OccurredAt:        timeutil.FormatISO8601(now),
	})
}

// SweepOverdueComplaints 是 **K04** 的執行者：把逾 SLA 的申訴標成 OVERDUE 並通知賣家。
//
// ⚠️ 本行原本寫 K05，那是誤標（2026-09-08 更正）：K05 是 OVERDUE → ESCALATED_TO_ADM
// （`EscalateOverdueComplaints`，見下方 EscalateOverdueComplaints）。誤標的代價是實質的
// ——K05 在 2026-09-08 之前零呼叫端，而這行字讓人以為它已經有執行者了。
//
// 🔴 這支之前**不存在**：`ComplaintRepository.MarkOverdue` 實作好了卻零呼叫端
// （#3055 的橫向稽查實查），於是逾期的申訴永遠停在 OPEN、賣家不會被催、
// ADM 的 OVERDUE 分頁恆空，而 SLA 是本模組對買家的承諾。
//
// 🔴 **標記與事件在同一個交易內**：分開做的話，標記成功而事件寫入失敗時，該筆已經
// 離開 MarkOverdue 的候選集合（status 不再是 OPEN）⇒ 通知永遠不會補發，而且完全無聲。
//
// 回傳這一輪真的標到的筆數，讓呼叫端記 log；掃不到東西不是錯誤。
func (u *Complaint) SweepOverdueComplaints(ctx context.Context, now time.Time) (int, error) {
	swept := 0
	err := u.tx.WithTx(ctx, func(ctx context.Context) error {
		rows, err := u.complaints.MarkOverdue(ctx, now, overdueSweepBatch)
		if err != nil {
			return err
		}
		for _, c := range rows {
			if u.audit != nil {
				if err := appendStatusHistory(ctx, u.audit, subjectComplaint, c.ID,
					codemeta.InvoiceComplaintStatusOpen.Value, c.Status,
					codemeta.InvoiceComplaintEventTypeSlaOverdue.Value, "", now); err != nil {
					return err
				}
			}
			issuer, ierr := u.issuers.GetByID(ctx, c.IssuerID)
			if ierr != nil {
				return ierr
			}
			payload, perr := complaintOverduePayload(issuer, c, now)
			if perr != nil {
				return perr
			}
			if _, oerr := u.outbox.Insert(ctx, domain.NewOutboxEvent{
				AggregateType: outboxAggregateComplaint,
				AggregateID:   c.ID,
				EventType:     string(pkgmq.EventTypeInvoiceComplaintOverdue),
				Exchange:      pkgmq.ExchangeInvoice,
				RoutingKey:    pkgmq.RoutingKeyInvoiceComplaintOverdue,
				Payload:       payload,
				NextAttemptAt: now,
				Now:           now,
			}); oerr != nil {
				return oerr
			}
			swept++
		}
		return nil
	})
	if err != nil {
		return 0, err
	}
	return swept, nil
}

// EscalateOverdueComplaints 是 **K05** 的執行者：`OVERDUE` 滿 24 小時仍沒有人回覆
// ⇒ `ESCALATED_TO_ADM`，案件交由平台判定。
//
// 🔴 這支在 2026-09-08 之前**不存在**：`ComplaintRepository.EscalateOverdue` 從介面、
// adapter 到 sqlc 生成物一應俱全，卻零呼叫端（全服務零呼叫端稽核實查）。後果是
// `OVERDUE` 的三條出邊裡，**唯一一條不需要外人動作的那條從來不會發生**：
//
//	K04 → SELLER_RESPONDED   要賣家自己來回覆
//	→   WITHDRAWN            要買家自己撤回
//	K05 → ESCALATED_TO_ADM   平台接手 ←← 沒有執行者
//
// 賣家不理、買家不撤回 ⇒ 申訴永遠停在 OVERDUE，平台永遠不會接手，而 SLA 是本模組
// 對買家的承諾。dev 上 7500699963840528384 因此卡了 5 天（sla_due_at 09-03）。
//
// ⚠️ 這是「實作好了卻沒有人呼叫」的**同一個 class 在同一個介面上的第二次**：相鄰的
// `MarkOverdue`（K04）在 #3055 被補過一次，補的人沒有回頭看隔壁那支。
//
// **刻意不發事件**：`ESCALATED_TO_ADM` 對兩邊都已經可見——ADM 讀清單（該狀態本來就是
// 它的分頁之一），買家端 B07 讀 gRPC 顯示「案件已交由平台客服處理」。為它新增一條
// routing key 會連帶動 36 號 catalog 與 topology，而沒有任何新的收件人。
//
// 留痕用 `ESCALATED` ＋ actor `SYSTEM`：CodeMeta 對該值的描述逐字寫著
// 「SYSTEM（逾期 24 小時自動升級，K05）」，本支就是那句話的實作。
func (u *Complaint) EscalateOverdueComplaints(ctx context.Context, now time.Time) (int, error) {
	escalated := 0
	err := u.tx.WithTx(ctx, func(ctx context.Context) error {
		rows, err := u.complaints.EscalateOverdue(ctx, now, now.Add(-escalateOverdueWindow), overdueSweepBatch)
		if err != nil {
			return err
		}
		for _, c := range rows {
			if u.audit != nil {
				if err := appendStatusHistory(ctx, u.audit, subjectComplaint, c.ID,
					codemeta.InvoiceComplaintStatusOverdue.Value, c.Status,
					codemeta.InvoiceComplaintEventTypeEscalated.Value, "", now); err != nil {
					return err
				}
			}
			escalated++
		}
		return nil
	})
	if err != nil {
		return 0, err
	}
	return escalated, nil
}

// ResolveExpiredBuyerReviews 是 **K06** 的執行者：賣家已回覆、買家的異議期屆滿仍未表態
// ⇒ `RESOLVED`（逾期視為接受）。
//
// 🔴 這支在 2026-09-08 之前**不存在**。`ComplaintRepository.ResolveExpiredBuyerReviews`
// 的 SQL 註解逐字寫著「**這是 K06 唯一的觸發者**」，而那個唯一的觸發者零呼叫端
// ⇒ 賣家回覆之後案件永遠停在 `SELLER_RESPONDED`：買家沒有異議也結不了案、
// 賣家的案件數永遠降不下來、`InvoiceComplaintUnresolvedAging` 把它當成未結案。
// dev 實例：7500462677014085632 的 buyer_review_due_at 是 2026-09-04，卡了 4 天。
//
// ⚠️ 它在第一輪零呼叫端稽核中**被漏掉**：adapter 的方法名與 sqlc 生成的方法名完全
// 相同（`r.q(ctx).ResolveExpiredBuyerReviews(...)`），而那一行被 grep 誤算成業務
// 呼叫端。同形的還有 K05 與 FetchApproachingStatutoryDeadlines —— 稽核腳本要排除
// `internal/repository/postgres/` 內的 `.q(ctx).` 自轉呼叫才看得到它們。
//
// **留痕只寫 invoice_status_history，不寫 invoice_complaint_events**：後者是買家 B07／
// 賣家 S10／ADM A07 三個畫面共用的**前端可見**時間軸，`InvoiceComplaintEventType` 的
// 7 個值沒有一個對應「逾期視為接受」，硬借一個（例如 PLATFORM_DECISION）會讓買家在
// 時間軸上讀到一句沒有發生過的事。要新增碼值就得同時決定三個前端的顯示文案，
// 那是產品決定（#2911 正是這一類：買家面不得出現「違規」二字）。
//
// resolution_code 取 NO_VIOLATION：賣家已依規回覆、買家未提異議 ⇒ 不計入違規窗口
// （CountConfirmedViolationOrders 的述詞是 status='VIOLATION_CONFIRMED' 單值）。
func (u *Complaint) ResolveExpiredBuyerReviews(ctx context.Context, now time.Time) (int, error) {
	resolved := 0
	err := u.tx.WithTx(ctx, func(ctx context.Context) error {
		rows, err := u.complaints.ResolveExpiredBuyerReviews(ctx, now,
			now.Add(reopenCooldownWindow), complaintResolutionNoViolation, overdueSweepBatch)
		if err != nil {
			return err
		}
		for _, c := range rows {
			if u.audit != nil {
				if err := appendStatusHistory(ctx, u.audit, subjectComplaint, c.ID,
					codemeta.InvoiceComplaintStatusSellerResponded.Value, c.Status,
					reasonBuyerReviewExpired, "", now); err != nil {
					return err
				}
			}
			resolved++
		}
		return nil
	})
	if err != nil {
		return 0, err
	}
	return resolved, nil
}

const reopenCooldownWindow = 24 * time.Hour

const reasonBuyerReviewExpired = "BUYER_REVIEW_EXPIRED"

const escalateOverdueWindow = 24 * time.Hour

const overdueSweepBatch int32 = 100
