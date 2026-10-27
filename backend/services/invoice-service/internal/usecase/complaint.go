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

// 本檔是 B07「未收到發票／收據申訴」的寫入面（§6.1.6 端點 13／63／64／65 ＝ §7.3.5 的
// K01／K07／K11／K13）。讀取面（端點 14／15）在 query_read.go 與 query.go。
//
// 四條貫穿全檔的約束：
//
//	① **Tier A（§8.1.4(b)）**：buyerID 一律進 SQL 的 WHERE——讀用 GetForBuyer、
//	   三支轉移的 CAS 自己也帶 complainant_buyer_id。越權與查無共用 9309、同一段
//	   message、同一行程式碼。**不得**改成 GetByID ＋ Go 端比對：那會多出一個可以
//	   回不同 code／message 的分支，而所有測試照樣綠（§8.6 T-10）。
//	② **狀態轉移一律 CAS，rows=0 不是成功**：三支轉移的 allowed-from ＋ 版本 ＋
//	   （K13 的）次數與冷卻全部寫在 SQL 的 WHERE，本層只判 ok 並回 9336。
//	③ **一次動作 ＝ 一個交易**：狀態轉移 ＋ 時間軸事件 ＋（K01 的）outbox 必須同進同退。
//	   少了交易，會出現「案件建了但賣家永遠收不到通知」或「狀態轉了但時間軸沒有那一筆」，
//	   兩者都不報錯。
//	④ **工作天曆是唯一的期限來源**：sla_due_at 由 BusinessCalendar 在建案當下算好落庫
//	   （§5.1.14），前端禁止自算（§6.1.6 第 1 條）。

const (
	// complaintTextMaxRunes 是三個自由文字欄的長度上限（58A B07：`maxLength={500}`）。
	//
	// 在這裡擋而不是讓 DB 擋：description／message 都是 text 欄，DB 根本不會擋——
	// 少了本檢查，前端的 maxLength 是唯一防線，而那是 client-side 的。
	complaintTextMaxRunes = 500

	// complaintWithdrawQuota 是 K01 守衛②：同一 document 已 WITHDRAWN 的案件數必須 < 2。
	//
	// WITHDRAWN 不在 uq_invcomp_open_per_document 的述詞內 ⇒ 撤回之後可以對同一份
	// document 重新申訴（誤開案要的正是這個行為），但它同時是一條繞過 reopen_count ≤ 2
	// 上限的路 ⇒ 配額由本常數守，不成立回 9336（§7.3.5 K11 下方紅字）。
	complaintWithdrawQuota = 2

	// complaintCalendarLookahead／Lookbehind 是讀鏡像時的窗口。
	//
	// 前看必須 ≥ ComplaintSlaHardCapDays，否則硬上限那一段的假日查不到；後看讓
	// 「已逾期 X 個工作天」也算得出來。
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
	// issuers 只為了一件事：把 invoice_complaints.issuer_id（PK）翻成
	// invoice_issuers.issuer_ref_id（＝ sellers.store_id）填進出向事件。
	// 少了它，P8 的 store_id 會是 TSID ⇒ 賣家收件人恆解不出來 ⇒ 靜默 0 投遞。
	issuers     domain.IssuerRepository
	outbox      domain.OutboxRepository
	businessDay domain.BusinessDayRepository
	// audit 是 I7 的寫入面（#3109，2026-09-04 補）。
	//
	// 🔴 **AppendEvent 不能代替它**：invoice_complaint_events 是**買家可見的時間軸**
	// （K01–K13 的事件型別），invoice_status_history 是**稽核**（§5.1.12，七個 aggregate
	// 共用一張表，A10 與 AuditTrailDrawer 讀它）。兩者用途不同、值域不同、保存義務不同。
	// 在這之前申訴軸只寫前者 ⇒ dev 上 subject_type='COMPLAINT' 的歷程只有 1 列，
	// 而那一列是 K09 違規計數專用的，K01–K13 的狀態轉移一列都沒有。
	audit domain.AuditRepository

	newID func() int64
	now   func() time.Time
	log   *zap.Logger
}

// NewComplaint 組出 B07 的寫入 usecase。
// subjectComplaint 是 invoice_status_history.subject_type 的申訴軸
// （chk_invhist_subject 七值之一，§5.1.12）。
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
		// Tier A：別人的訂單得到空清單 ⇒ 與「這張訂單還沒有發票文件」共用 9301。
		if len(docs) == 0 {
			return documentNotFound()
		}

		now := u.now()
		// 9311 時間閘（§6.1.6 第 3 條）：不設這個閘 ⇒ 申訴變成催單管道 ⇒ 灌爆賣家的
		// 3 工作天 SLA ⇒ A07 誤判違規 ⇒ 觸發 C1 停權門檻。
		if now.Before(complaintOpensAt(docs)) {
			return complaintBiz(bizcode.InvoiceComplaintNotOpenYet)
		}

		cal, err := u.calendarAround(ctx, now)
		if err != nil {
			return err
		}

		// 申訴掛在**最新一份**主文件上（作廢重開鏈的最後一份）：uq_invcomp_open_per_document
		// 與撤回配額都是 document-scoped。
		target := docs[0]

		// 守衛①：已有未結案案件 ⇒ 不是錯誤，導向既有那一件。
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

		// 守衛②：撤回額度。放在建案端（可數、可回明確業務碼），不放進唯一索引。
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
			// 守衛①與 INSERT 之間仍有一個極窄的窗口（別的分頁同時送出），
			// uq_invcomp_open_per_document 擋下它 ⇒ 走與守衛①相同的出口。
			existing, found, err := findOpenComplaint(ctx, u.complaints, in.BuyerID, in.OrderNo)
			if err != nil {
				return err
			}
			if !found {
				// 索引擋下了，卻查不到那一件 ⇒ 述詞與索引述詞已經漂移。
				// 回錯讓整筆 rollback：放行會讓買家看到一個空的成功回應。
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
			// action 留空：InvoiceComplaintAction 的 6 值是「賣家／ADM 點了哪一顆」，
			// 買家的動作不進那個軸（§7.3.5 逐字）。
			ActorType:      complaintActorBuyer,
			ActorAccountID: in.BuyerID,
			Message:        in.Description,
			Now:            now,
		})
		if err != nil {
			return err
		}

		// 🔴 store_id 走 invoice_issuers.issuer_ref_id，不是 row.IssuerID（那是 PK）。
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
		// 重開把案件推回 OPEN ⇒ SLA 重新起算（與 K10 同理）。不重置的話這一列一進 OPEN
		// 就落回逾期掃描的候選集合，賣家連看都還沒看到就被標成 OVERDUE。
		apply: func(ctx context.Context, cal domain.BusinessCalendar, now time.Time) (bool, error) {
			return u.complaints.Reopen(ctx, in.ComplaintID, in.BuyerID, in.ExpectedVersion,
				u.slaDueAt(cal, now), now)
		},
	})
}

// complaintTransition 是三支買家動作的共通形狀。
type complaintTransition struct {
	eventType string
	apply     func(ctx context.Context, cal domain.BusinessCalendar, now time.Time) (bool, error)
}

// transition 跑「讀 → 驗版本 → CAS → 寫時間軸 → 回轉移後的單筆」這條共通路徑。
//
// 一律回**轉移後**的完整單筆（§6.1.6 共通形狀）：前端據此直接覆蓋 query cache，
// 不必再打一次端點 15；少了它，兩次請求之間的窗口會讓畫面顯示轉移前的狀態。
func (u *Complaint) transition(ctx context.Context, in ComplaintActionInput, t complaintTransition) (ComplaintView, error) {
	var out ComplaintView
	err := u.tx.WithTx(ctx, func(ctx context.Context) error {
		// 🔴 GetForBuyer 不是 GetByID：擁有者述詞在 SQL 的 WHERE，越權與查無由
		// 同一行程式碼回同一個 9309（§8.1.4(b)）。
		c, err := u.complaints.GetForBuyer(ctx, in.ComplaintID, in.BuyerID)
		if err != nil {
			return err
		}
		// 第二段述詞走同一個出口，不另開碼。
		if in.OrderNo != "" && derefString(c.OrderNo) != in.OrderNo {
			return complaintNotFound()
		}
		// 版本不符回 9322（§6.1.6 共通形狀）。CAS 仍然帶 version ⇒ 本判斷是為了給出
		// 可分辨的訊息，不是正確性防線；真正的防線在 SQL 的 WHERE。
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
		// 🔴 rows=0 ＝ 守衛不成立（allowed-from 不符／版本被搶先／重開次數用完／冷卻未到），
		// 不得當成功往下走——那正是靜默失敗的形狀。三種情境共用 9336，訊息由 CodeMeta 給。
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

		// 重讀而不是就地拼裝：version、sla_due_at、reopen_count 這些欄位的新值由 SQL
		// 算出（version+1、reopen_count+1），Go 端拼一份等於維護第二套算式。
		updated, err := u.complaints.GetForBuyer(ctx, in.ComplaintID, in.BuyerID)
		if err != nil {
			return err
		}

		// I7：每一次 status 轉移都要有一列 invoice_status_history（#3109）。
		//
		// 放在重讀之後：to 取**實際落地的值**而不是 t.eventType 推導的期望值——
		// K13 重開的目標狀態由 SQL 依 reopen_count 決定，Go 端猜一份就是第二套算式。
		// 自轉移不寫（狀態沒變的動作由 complaint_events 表達）。
		if u.audit != nil && updated.Status != c.Status {
			// actor 是**買家**：硬寫 SYSTEM 會讓稽核看不出這一步是誰按的。
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

// ---------------------------------------------------------------------------
// 工作天曆
// ---------------------------------------------------------------------------

// calendarAround 讀一次鏡像，組出涵蓋「倒數 ＋ 硬上限」所需區間的行事曆快照。
//
// 一個請求只讀一次：sla_due_at 與 remainingBusinessDays 用的是同一份快照，
// 分兩次讀會讓兩者在鏡像剛好被 INV.gov 覆寫的那一瞬間互相矛盾。
func (u *Complaint) calendarAround(ctx context.Context, now time.Time) (domain.BusinessCalendar, error) {
	return loadBusinessCalendar(ctx, u.businessDay,
		now.AddDate(0, 0, -complaintCalendarLookbehind),
		now.AddDate(0, 0, complaintCalendarLookahead))
}

// slaDueAt 算出一件申訴的 sla_due_at（K01 建案與 K13 重開共用）。
func (u *Complaint) slaDueAt(cal domain.BusinessCalendar, now time.Time) time.Time {
	if !cal.Covers(now, now.AddDate(0, 0, domain.ComplaintSlaHardCapDays)) {
		// 鏡像過期或從未同步 ⇒ 未涵蓋的日期一律視為假日、落在硬上限（偏晚）。
		// 這裡只留一行 WARN：告警的資料面是 max(synced_at)，由 INV.gov 的
		// invoice_business_calendar_stale 負責，不在請求路徑上重造一套。
		u.logger().Warn("invoice.complaint.business_calendar_stale")
	}
	return cal.ComplaintSlaDueAt(now)
}

// loadBusinessCalendar 由鏡像組出一份行事曆快照。
//
// 兩端各外擴一天：ComplaintSlaDueAt 是從「起算日的次一天」開始推的，區間切齊會讓
// 邊界那一天的假日查不到而被當成工作天（偏早）。
func loadBusinessCalendar(ctx context.Context, repo domain.BusinessDayRepository, from, to time.Time) (domain.BusinessCalendar, error) {
	if from.After(to) {
		from, to = to, from
	}
	holidays, err := repo.ListHolidays(ctx, from.AddDate(0, 0, -1), to.AddDate(0, 0, 1))
	if err != nil {
		return domain.BusinessCalendar{}, err
	}
	syncedAt, err := repo.SyncedAt(ctx)
	if err != nil {
		return domain.BusinessCalendar{}, err
	}
	return domain.NewBusinessCalendar(holidays, syncedAt), nil
}

// viewOf 補上時間軸與工作天倒數。
func (u *Complaint) viewOf(ctx context.Context, c domain.Complaint, cal domain.BusinessCalendar, now time.Time) (ComplaintView, error) {
	events, err := u.complaints.ListEvents(ctx, c.ID)
	if err != nil {
		return ComplaintView{}, err
	}
	return viewWith(c, events, cal, now), nil
}

// viewWith 是不再打 DB 的那一半（建案路徑手上已經有事件列）。
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

// ---------------------------------------------------------------------------
// 共用小工具
// ---------------------------------------------------------------------------

// complaintActorBuyer 是 invoice_complaint_events.actor_type 的買家值
// （chk_invcompev_actor 四值之一）。
const complaintActorBuyer = "BUYER"

// outboxAggregateComplaint 是 invoice_outbox_events.aggregate_type 的申訴軸
// （§23／36 號：一張表以 aggregate_type 區分 DOCUMENT／ADJUSTMENT／COMPLAINT／TAX_PROFILE）。
const outboxAggregateComplaint = "COMPLAINT"

// complaintReasonNotReceived 是 Phase 1 唯一的申訴原因（§6.1.6／proto 逐字）。
//
// ⚠️ **CodeMeta 沒有這個值域**（`invoice_complaints.reason_code` 只有 DB 的
// DEFAULT 'NOT_RECEIVED'，沒有 CHECK、沒有生成 enum）⇒ 這裡是全服務唯一的字面值。
// Phase 2 增加第二個原因時正解是先補 CodeMeta 再回來刪掉它。缺口登記在交付回報。
const complaintReasonNotReceived = "NOT_RECEIVED"

// normalizeComplaintReasonCode 把空字串補成 Phase 1 的唯一值，其餘一律拒絕。
//
// 不猜也不放行：reason_code 會落進 DB 並出現在 ADM 的篩選器，收一個值域外的字串
// 等於讓那個篩選器從此少一類案件而沒有人會發現。
func normalizeComplaintReasonCode(code string) (string, error) {
	if code == "" || code == complaintReasonNotReceived {
		return complaintReasonNotReceived, nil
	}
	return "", ErrComplaintReasonCodeInvalid
}

// errComplaintIndexDrift 是「唯一索引擋下了，但用同一組條件查不到那一件」。
//
// 它是故障不是業務結果 ⇒ 刻意不是 DomainError，走 codes.Internal。
var errComplaintIndexDrift = errors.New("invoice: complaint insert was rejected by the open-per-document index but no open case was found")

// complaintOpensAt 回申訴入口的開放時刻（§6.1.6 第 3 條的 9311 時間閘）。
//
// 以**最早**一份文件的建立時間起算：作廢重開不該讓買家的申訴窗口重新倒數。
// 🔴 端點 8 的 complaintOpensAt 與端點 13 的閘門必須是同一個算式——兩份實作漂移的
// 症狀是「按鈕可以按，按下去回 9311」，而畫面上沒有任何線索。
func complaintOpensAt(docs []domain.Document) time.Time {
	return docs[len(docs)-1].CreatedAt.Add(complaintOpenDelay)
}

// findOpenComplaint 找這張訂單底下這位買家的進行中申訴。
//
// buyerID 是 ListForBuyerOrder 的述詞之一（SQL 的 WHERE）⇒ 別人的案件在這裡查不到。
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

// applyComplaintCountdown 補上 remaining／overdue 兩個工作天欄位。
//
// 算不出來時**保持 nil**（未計算）：回 0 會被前端讀成「今天到期」，而用自然日代替
// 會偏早 ⇒ 賣家被誤判逾期 ⇒ 違規計數 +1（§5.1.14 表格逐字）。
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

// complaintVersionConflict 借 9322（碼名是稅務側的）。
//
// ⚠️ 碼名與語意不一致是設計書指定的（§6.1.6 共通形狀「版本不符 9322」、58A 錯誤表
// 逐字「碼名是稅務側的，但申訴動作的 expectedVersion 衝突共用它」）。正解是新增一個
// 通用的樂觀鎖碼，登記在交付回報。
func complaintVersionConflict() error {
	return pkgerrors.NewBusiness(bizcode.SellerTaxProfileVersionConflict,
		bizcode.Message(bizcode.SellerTaxProfileVersionConflict))
}

// complaintCreatedEvent 是 invoice.complaint.created 的 payload。
//
// payload 邊界（36 號逐字）：禁止放載具識別碼、買受人統編、email、電話、隨機碼與任何
// provider credential。本事件只放路由與顯示所需的識別碼與期限。
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
		// 冪等鍵形狀取自 36 號的 `INVCPL:{complaint_id}:{phase}`：少了 phase，
		// 同一件案子的 created／responded 兩則通知會被 notification-service 去重掉一則。
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

// ---------------------------------------------------------------------------
// K05：申訴逾 3 工作天未回覆（P10，#3055 的橫向稽查）
// ---------------------------------------------------------------------------

// complaintOverdueEvent 是 P10（`invoice.complaint.overdue`，58 §6.3.9 定版 13 列第 11 列）。
//
// 該模板刻意**沒有** refType／refId（同 P9，它指向的是申訴列表不是單一憑證），
// 也刻意不用 overdue_hours ——「少一個必填 key，解除條件就少一個可能對不上的地方」是
// migration 檔頭的原話。這裡仍然帶齊業務欄位，模板用不到的欄位不會造成任何問題。
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
		// 冪等鍵的 phase 是 `overdue`（36 號的 `INVCPL:{complaint_id}:{phase}`）：
		// 與同一件案子的 created／responded 分開，否則三則會被去重成一則。
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
			// I7：K04 逾期轉態也要留痕（#3109）。
			//
			// 🔴 from 寫死 OPEN 而不是 c.Status：MarkOverdue 回的是**已更新**的列，
			// c.Status 此刻已經是 OVERDUE，拿它當 from 會寫出一條 OVERDUE → OVERDUE
			// 的自轉移。K04 的 allowed-from 在 SQL 裡就是 OPEN 單值。
			if u.audit != nil {
				if err := appendStatusHistory(ctx, u.audit, subjectComplaint, c.ID,
					codemeta.InvoiceComplaintStatusOpen.Value, c.Status,
					codemeta.InvoiceComplaintEventTypeSlaOverdue.Value, "", now); err != nil {
					return err
				}
			}
			// issuer 讀不到就整批 rollback：用 issuer_id（PK）頂替 store_id 的症狀是
			// 下游解析不到賣家、通知靜默不送，而該筆已經離開候選集合 ⇒ 永遠不會再試。
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
			// from 寫死 OVERDUE 而不是 c.Status，理由與 K04 那一段逐字相同：
			// EscalateOverdue 回的是**已更新**的列，c.Status 此刻已經是 ESCALATED_TO_ADM，
			// 拿它當 from 會寫出一條自轉移。K05 的 allowed-from 在 SQL 裡是 OVERDUE 單值。
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
			// from 寫死 SELLER_RESPONDED，理由與 K04／K05 逐字相同：rows 回的是**已更新**
			// 的列，c.Status 此刻已是 RESOLVED。K06 的 allowed-from 在 SQL 裡是單值。
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

// reopenCooldownWindow 是 K06 結案後的重開冷卻（端點 65 的守衛與 B07 重開鍵讀
// reopen_allowed_after）。數字取自 ResolveExpiredBuyerReviews 的 SQL 註解「now + 24h」。
const reopenCooldownWindow = 24 * time.Hour

// reasonBuyerReviewExpired 是 K06 在**稽核軸**（invoice_status_history.reason_code，
// 無值域約束）上的理由碼。刻意不借用 InvoiceComplaintEventType 的任何值——見
// ResolveExpiredBuyerReviews 的註解。
const reasonBuyerReviewExpired = "BUYER_REVIEW_EXPIRED"

// escalateOverdueWindow 是 K05 的等待期：進 OVERDUE 之後再給賣家 24 小時。
//
// 數字取自 `EscalateOverdueComplaints` 的 SQL 註解（「OVERDUE 且 now >= overdue_at + 24h」）
// 與 CodeMeta `InvoiceComplaintStatusOverdue` 的描述（「24 小時後自動升級 ADM」）——
// 三處必須同值，改一處就要一起改。
const escalateOverdueWindow = 24 * time.Hour

// overdueSweepBatch 是一輪最多標幾筆。
//
// 與 INV.dl 的 tick（5 分鐘）搭配：逾期是**時間到就成立**的事實，不會突然湧入幾千筆；
// 而批次上限的作用是讓單輪耗時有上界（整批在同一個交易內，見上面的註解）。
const overdueSweepBatch int32 = 100
