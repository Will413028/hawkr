package governance

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"
	"unicode/utf8"
	"uuid"

	"github.com/jackc/pgx/v5"

	"github.com/Will413028/hawkr/backend/internal/modules/media"
	"github.com/Will413028/hawkr/backend/internal/modules/seller/internal/model"
	"github.com/Will413028/hawkr/backend/internal/modules/seller/internal/stint"
	"github.com/Will413028/hawkr/backend/internal/modules/seller/internal/store"
	"github.com/Will413028/hawkr/backend/internal/modules/seller/internal/text"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
	"github.com/Will413028/hawkr/backend/internal/platform/events"
	"github.com/Will413028/hawkr/backend/internal/platform/uitext"
)


func errInquiryNotFound() error {
	m, _ := uitext.For(uitext.ZhTW).CommonError("NOT_FOUND")
	return apperr.NotFound("%s", m.Text)
}

func toInquiry(row store.SellerCaseInquiry) model.CaseInquiry {
	i := model.CaseInquiry{
		ID: row.ID, CaseID: row.CaseID, StoreID: row.StoreID, Kind: model.InquiryKind(row.Kind), Message: row.Message,
		CreatedAt: row.CreatedAt, CreatedByAdminID: row.CreatedByAdminID, DueAt: row.DueAt,
	}
	if row.ReplyReceivedAt != nil && row.ReplyBody != nil && row.ReplyChannel != nil && row.ReplyRecordedAt != nil {
		i.Reply = &model.InquiryReply{
			Body: *row.ReplyBody, Channel: model.InquiryReplyChannel(*row.ReplyChannel), ReceivedAt: *row.ReplyReceivedAt,
			RecordedAt: *row.ReplyRecordedAt,
		}
		if row.ReplyRecordedByAdminID != nil {
			i.Reply.RecordedByAdminID = *row.ReplyRecordedByAdminID
		}
		if row.RepliedByAccountID != nil {
			i.Reply.RepliedByAccountID = *row.RepliedByAccountID
		}
	}
	if row.WithdrawnAt != nil && row.WithdrawnByAdminID != nil && row.WithdrawCause != nil {
		i.Withdrawn = &model.InquiryWithdrawal{At: *row.WithdrawnAt, AdminID: *row.WithdrawnByAdminID, Cause: model.InquiryWithdrawCause(*row.WithdrawCause)}
	}
	return i
}

func adminInquiries(ctx context.Context, q *store.Queries, rows []store.SellerCaseInquiry) ([]model.AdminInquiry, error) {
	out := make([]model.AdminInquiry, len(rows))
	if len(rows) == 0 {
		return out, nil
	}
	ids := make([]uuid.UUID, 0, len(rows))
	index := make(map[uuid.UUID]int, len(rows))
	for i, r := range rows {
		inquiry := toInquiry(r)
		out[i] = model.AdminInquiry{CaseInquiry: inquiry, Availability: inquiry.Availability()}
		if inquiry.Reply != nil {
			ids = append(ids, r.ID)
			index[r.ID] = i
		}
	}
	if len(ids) == 0 {
		return out, nil
	}
	evidence, err := q.CaseInquiryEvidence(ctx, ids)
	if err != nil {
		return nil, err
	}
	for _, e := range evidence {
		reply := out[index[e.InquiryID]].Reply
		reply.Evidence = append(reply.Evidence, model.Evidence{AssetID: e.AssetID, ObjectKey: e.ObjectKey, Position: int(e.Position)})
	}
	return out, nil
}

func readInquiry(ctx context.Context, q *store.Queries, id uuid.UUID) (model.AdminInquiry, error) {
	row, err := q.CaseInquiryByID(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return model.AdminInquiry{}, errInquiryNotFound()
	}
	if err != nil {
		return model.AdminInquiry{}, err
	}
	views, err := adminInquiries(ctx, q, []store.SellerCaseInquiry{row})
	if err != nil {
		return model.AdminInquiry{}, err
	}
	return views[0], nil
}

func inquiryBlocked(blocks []model.InquiryBlock) error {
	if len(blocks) == 0 {
		return nil
	}
	return apperr.FailedPrecondition("%s", blocks[0].Text().Text)
}

func inquiryRequestHash(in model.CreateCaseInquiryInput, message string, due *time.Time) []byte {
	h := sha256.New()
	at := "-"
	if due != nil {
		at = due.UTC().Format(time.RFC3339Nano)
	}
	for _, p := range []string{in.CaseID.String(), string(in.Kind), message, at} {
		_, _ = fmt.Fprintf(h, "%d:%s|", len(p), p)
	}
	return h.Sum(nil)
}

// CreateInquiry 對案件的賣家發一筆詢問。冪等重送（帶 request_id）先於有狀態的驗證：同一位管理員、同一個 request_id、同樣的內容回傳原本那一筆詢問的目前狀態
// （AIP-155 的 stale success 例外），內容不同回 InvalidInput；鎖住案件之後再查一次，兩個並行的重送後到的回到查詢。
// 發詢問必須在 CaseAvailability.CreateInquiry 裡可做（案件進行中、店家的個資還沒清除、沒有還沒結束的詢問；只是關店不擋：帳號還在，條款第 11.3 條的義務與案件都還在）。
// 期限不設時是建立後 InquiryDefaultDue，設定時必須在建立後 InquiryMinDue 到 InquiryMaxDue 之間。同一個 transaction 讓案件的版本前進。在同一個 transaction 發
// CaseInquiryCreated（重送回放不發）。
// 鎖順序：店家列 → 案件列。
func (s *Service) CreateInquiry(ctx context.Context, in model.CreateCaseInquiryInput) (model.CreateCaseInquiryResult, error) {
	if err := model.Positive(in.AdminID); err != nil {
		return model.CreateCaseInquiryResult{}, err
	}
	if !slices.Contains(model.InquiryKindValues(), in.Kind) {
		return model.CreateCaseInquiryResult{}, invalidOn("Kind", text.InquiryKindRequired())
	}
	message := strings.TrimSpace(in.Message)
	if n := utf8.RuneCountInString(message); n == 0 || n > model.InquiryMessageMaxRunes {
		return model.CreateCaseInquiryResult{}, invalidOn("Message", text.InquiryMessageLength())
	}
	var due *time.Time
	if in.DueAt != nil {
		t := in.DueAt.UTC().Truncate(time.Microsecond)
		due = &t
	}
	var hash []byte
	if in.RequestID != nil {
		hash = inquiryRequestHash(in, message, due)
	}
	var result model.CreateCaseInquiryResult
	err := s.d.DB.InTx(ctx, func(ctx context.Context) error {
		q := store.New(s.d.DB.Conn(ctx))
		if replayed, ok, err := s.replayInquiry(ctx, q, in, hash); err != nil || ok {
			result = replayed
			return err
		}
		found, err := q.CaseByID(ctx, in.CaseID)
		if errors.Is(err, pgx.ErrNoRows) {
			return errCaseNotFound()
		}
		if err != nil {
			return err
		}
		storeRow, err := stint.LockAccountOf(ctx, q, found.StoreID)
		if err != nil {
			return err
		}
		locked, err := lockCase(ctx, q, in.CaseID, 0)
		if err != nil {
			return err
		}
		if replayed, ok, err := s.replayInquiry(ctx, q, in, hash); err != nil || ok {
			result = replayed
			return err
		}
		facts, err := caseFactsOf(ctx, q, locked.ID)
		if err != nil {
			return err
		}
		if err := caseBlocked(facts.Availability().CreateInquiry, model.CaseActionCreateInquiry); err != nil {
			return err
		}
		now := s.now()
		dueAt := now.Add(model.InquiryDefaultDue)
		if due != nil {
			dueAt = *due
			if d := dueAt.Sub(now); d < model.InquiryMinDue || d > model.InquiryMaxDue {
				return invalidOn("DueAt", text.InquiryDueRange())
			}
		}
		row, err := q.InsertCaseInquiry(ctx, store.InsertCaseInquiryParams{
			ID: uuid.NewV7(), CaseID: locked.ID, StoreID: locked.StoreID, Kind: string(in.Kind), Message: message, Now: now, AdminID: in.AdminID,
			DueAt: dueAt, RequestID: in.RequestID, RequestHash: hash,
		})
		if errors.Is(err, pgx.ErrNoRows) {
			replayed, ok, rerr := s.replayInquiry(ctx, q, in, hash)
			if rerr != nil {
				return rerr
			}
			if !ok {
				return fmt.Errorf("seller：詢問的 request_id %s 衝突，卻查不到原本那一筆", in.RequestID)
			}
			result = replayed
			return nil
		}
		if err != nil {
			return err
		}
		if err := q.BumpCase(ctx, store.BumpCaseParams{ID: locked.ID, Now: now}); err != nil {
			return err
		}
		if err := events.Publish(ctx, s.d.Events, model.CaseInquiryCreated{
			EventID: uuid.NewV7(), OccurredAt: now, InquiryID: row.ID, CaseID: locked.ID, StoreID: locked.StoreID,
			AccountID: storeRow.AccountID, Kind: in.Kind, DueAt: dueAt,
		}); err != nil {
			return err
		}
		inquiry, err := readInquiry(ctx, q, row.ID)
		if err != nil {
			return err
		}
		view, err := readCase(ctx, q, locked.ID, s.now())
		result = model.CreateCaseInquiryResult{Inquiry: inquiry, Case: view}
		return err
	})
	return result, err
}

func (s *Service) replayInquiry(ctx context.Context, q *store.Queries, in model.CreateCaseInquiryInput, hash []byte) (model.CreateCaseInquiryResult, bool, error) {
	if in.RequestID == nil {
		return model.CreateCaseInquiryResult{}, false, nil
	}
	row, err := q.CaseInquiryByRequest(ctx, store.CaseInquiryByRequestParams{AdminID: in.AdminID, RequestID: in.RequestID})
	if errors.Is(err, pgx.ErrNoRows) {
		return model.CreateCaseInquiryResult{}, false, nil
	}
	if err != nil {
		return model.CreateCaseInquiryResult{}, false, err
	}
	if !bytes.Equal(row.RequestHash, hash) {
		return model.CreateCaseInquiryResult{}, false, invalidOn("RequestID", text.InquiryRequestIdReused())
	}
	inquiry, err := readInquiry(ctx, q, row.ID)
	if err != nil {
		return model.CreateCaseInquiryResult{}, false, err
	}
	view, err := readCase(ctx, q, row.CaseID, s.now())
	return model.CreateCaseInquiryResult{Inquiry: inquiry, Case: view, Replayed: true}, true, err
}

// RecordInquiryReply 記下系統外收到的賣家回覆：管道、收到的時間（不早於詢問建立、不晚於現在；以秒比較下限，同一秒內的記為建立當下）、內容與最多 MaxEvidence 份附件，記下之後這筆詢問就結束。
// 必須在 transaction 外呼叫：附件（機敏上傳的 staged token，必須是機敏檔）先以管理員為上傳者轉正，轉正之後沒有寫入的資產由 media 的 GC 在建立滿 7 天後回收。
// 已經記過時回傳原本那一則（AlreadyRecorded，不比較內容）；詢問已撤回回 failed_precondition。不要求案件進行中：案件結案時詢問已被撤回，原因就是已撤回。
// 鎖順序：店家列 → 案件列 → 詢問列。
func (s *Service) RecordInquiryReply(ctx context.Context, in model.RecordCaseInquiryReplyInput) (model.RecordCaseInquiryReplyResult, error) {
	if err := model.Positive(in.AdminID); err != nil {
		return model.RecordCaseInquiryReplyResult{}, err
	}
	if !slices.Contains(model.AdminRecordableChannels(), in.Channel) {
		return model.RecordCaseInquiryReplyResult{}, invalidOn("Channel", text.InquiryChannelRequired())
	}
	if in.ReceivedAt.IsZero() {
		return model.RecordCaseInquiryReplyResult{}, invalidOn("ReceivedAt", text.InquiryReceiveTimeRange())
	}
	body := strings.TrimSpace(in.Body)
	if n := utf8.RuneCountInString(body); n == 0 || n > model.InquiryReplyMaxRunes {
		return model.RecordCaseInquiryReplyResult{}, invalidOn("Body", text.InquiryBodyLength())
	}
	if err := model.CheckEvidenceTokens(in.EvidenceTokens); err != nil {
		return model.RecordCaseInquiryReplyResult{}, err
	}
	received := in.ReceivedAt.UTC().Truncate(time.Microsecond)
	if _, err := store.New(s.d.DB.Conn(ctx)).CaseInquiryByID(ctx, in.InquiryID); errors.Is(err, pgx.ErrNoRows) {
		return model.RecordCaseInquiryReplyResult{}, errInquiryNotFound()
	} else if err != nil {
		return model.RecordCaseInquiryReplyResult{}, err
	}
	var result model.RecordCaseInquiryReplyResult
	err := s.d.DB.InTx(ctx, func(ctx context.Context) error {
		q := store.New(s.d.DB.Conn(ctx))
		locked, err := s.lockInquiry(ctx, q, in.InquiryID)
		if err != nil {
			return err
		}
		inquiry := toInquiry(locked)
		blocks := inquiry.Availability().RecordReply
		if len(blocks) > 0 && blocks[0] == model.InquiryBlockReplied {
			result, err = s.inquiryResult(ctx, q, inquiry)
			result.AlreadyRecorded = true
			return err
		}
		if err := inquiryBlocked(blocks); err != nil {
			return err
		}
		now := s.now()
		if received.Before(inquiry.CreatedAt.Truncate(time.Second)) || received.After(now) {
			return invalidOn("ReceivedAt", text.InquiryReceiveTimeRange())
		}
		if received.Before(inquiry.CreatedAt) {
			received = inquiry.CreatedAt
		}
		if _, err := q.RecordCaseInquiryReply(ctx, store.RecordCaseInquiryReplyParams{
			Body: body, Channel: string(in.Channel), ReceivedAt: received, Now: now, AdminID: in.AdminID, ID: inquiry.ID,
		}); err != nil {
			return err
		}
		evidence, err := model.ConfirmEvidence(ctx, s.d.Media, media.Uploader{ID: in.AdminID, Admin: true}, in.EvidenceTokens)
		if err != nil {
			return err
		}
		for i, e := range evidence {
			if err := q.InsertCaseInquiryEvidence(ctx, store.InsertCaseInquiryEvidenceParams{
				InquiryID: inquiry.ID, Position: int32(i), AssetID: e.AssetID, ObjectKey: e.ObjectKey, //nolint:gosec // 位置最多 MaxEvidence
			}); err != nil {
				return err
			}
		}
		if err := q.BumpCase(ctx, store.BumpCaseParams{ID: inquiry.CaseID, Now: now}); err != nil {
			return err
		}
		result, err = s.inquiryResult(ctx, q, inquiry)
		return err
	})
	return result, err
}

// WithdrawInquiry 撤回還沒回覆的詢問（賣家不必再回覆）。已經撤回時回傳原本的紀錄（AlreadyWithdrawn）；已回覆回 failed_precondition。
// 鎖順序：店家列 → 案件列 → 詢問列。
func (s *Service) WithdrawInquiry(ctx context.Context, in model.WithdrawCaseInquiryInput) (model.WithdrawCaseInquiryResult, error) {
	if err := model.Positive(in.AdminID); err != nil {
		return model.WithdrawCaseInquiryResult{}, err
	}
	var result model.WithdrawCaseInquiryResult
	err := s.d.DB.InTx(ctx, func(ctx context.Context) error {
		q := store.New(s.d.DB.Conn(ctx))
		locked, err := s.lockInquiry(ctx, q, in.InquiryID)
		if err != nil {
			return err
		}
		inquiry := toInquiry(locked)
		blocks := inquiry.Availability().Withdraw
		if len(blocks) > 0 && blocks[0] == model.InquiryBlockWithdrawn {
			view, err := s.inquiryResult(ctx, q, inquiry)
			result = model.WithdrawCaseInquiryResult{Inquiry: view.Inquiry, Case: view.Case, AlreadyWithdrawn: true}
			return err
		}
		if err := inquiryBlocked(blocks); err != nil {
			return err
		}
		now := s.now()
		if _, err := q.WithdrawCaseInquiry(ctx, store.WithdrawCaseInquiryParams{Now: now, AdminID: in.AdminID, ID: inquiry.ID}); err != nil {
			return err
		}
		if err := q.BumpCase(ctx, store.BumpCaseParams{ID: inquiry.CaseID, Now: now}); err != nil {
			return err
		}
		view, err := s.inquiryResult(ctx, q, inquiry)
		result = model.WithdrawCaseInquiryResult{Inquiry: view.Inquiry, Case: view.Case}
		return err
	})
	return result, err
}

func (s *Service) lockInquiry(ctx context.Context, q *store.Queries, id uuid.UUID) (store.SellerCaseInquiry, error) {
	found, err := q.CaseInquiryByID(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return found, errInquiryNotFound()
	}
	if err != nil {
		return found, err
	}
	if _, err := stint.LockAccountOf(ctx, q, found.StoreID); err != nil {
		return found, err
	}
	if _, err := lockCase(ctx, q, found.CaseID, 0); err != nil {
		return found, err
	}
	locked, err := q.LockCaseInquiry(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return locked, errInquiryNotFound()
	}
	return locked, err
}

func (s *Service) inquiryResult(ctx context.Context, q *store.Queries, inquiry model.CaseInquiry) (model.RecordCaseInquiryReplyResult, error) {
	view, err := readInquiry(ctx, q, inquiry.ID)
	if err != nil {
		return model.RecordCaseInquiryReplyResult{}, err
	}
	c, err := readCase(ctx, q, inquiry.CaseID, s.now())
	return model.RecordCaseInquiryReplyResult{Inquiry: view, Case: c}, err
}

func withdrawOpenInquiry(ctx context.Context, q *store.Queries, caseID uuid.UUID, adminID int64, now time.Time) error {
	_, err := q.WithdrawOpenCaseInquiry(ctx, store.WithdrawOpenCaseInquiryParams{Now: now, AdminID: adminID, CaseID: caseID})
	return err
}

// Inquiries 是案件的詢問清單，舊到新（keyset）；案件不存在回 NotFound。多讀一筆判斷有沒有下一頁。
func (s *Service) Inquiries(ctx context.Context, in model.CaseInquiryListInput) (model.AdminInquiryPage, error) {
	size, err := inquiryPageSize(in.PageSize)
	if err != nil {
		return model.AdminInquiryPage{}, err
	}
	params := store.CaseInquiriesParams{CaseID: in.CaseID, PageSize: size + 1}
	if in.After != nil {
		params.AfterCreatedAt, params.AfterID = &in.After.CreatedAt, &in.After.ID
	}
	var page model.AdminInquiryPage
	err = s.d.DB.InReadSnapshot(ctx, func(ctx context.Context) error {
		q := store.New(s.d.DB.Conn(ctx))
		if _, err := q.CaseByID(ctx, in.CaseID); errors.Is(err, pgx.ErrNoRows) {
			return errCaseNotFound()
		} else if err != nil {
			return err
		}
		rows, err := q.CaseInquiries(ctx, params)
		if err != nil {
			return err
		}
		more := len(rows) > int(size)
		if more {
			rows = rows[:size]
		}
		if page.Items, err = adminInquiries(ctx, q, rows); err != nil {
			return err
		}
		if more {
			last := page.Items[size-1]
			page.Next = &model.InquiryCursor{CreatedAt: last.CreatedAt, ID: last.ID}
		}
		return nil
	})
	return page, err
}
