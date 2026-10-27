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
	"github.com/Will413028/hawkr/backend/internal/modules/seller/internal/mediaref"
	"github.com/Will413028/hawkr/backend/internal/modules/seller/internal/model"
	"github.com/Will413028/hawkr/backend/internal/modules/seller/internal/stint"
	"github.com/Will413028/hawkr/backend/internal/modules/seller/internal/store"
	"github.com/Will413028/hawkr/backend/internal/modules/seller/internal/text"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
	"github.com/Will413028/hawkr/backend/internal/platform/uitext"
)

// 案件的詢問（step10e-reports-brief R4、設計 step10e-3cb-design §3.2）。鎖順序：店家列 → 案件列 → 詢問列（同一家店的結案、發詢問與記下回覆因此串行：
// 結案先鎖店家，所以不會出現結案之後才寫進回覆的詢問）。詢問只加不改案件的成員，但建立、記下回覆、撤回都讓案件的版本與 updated_at 前進
// （它們改變「可以決定」與決定的依據）；指令本身不帶 etag。

// errInquiryNotFound 是詢問不存在：不另加伺服器句子，文字是共用文案目錄 Connect code NOT_FOUND 的預設文字（common.error.NOT_FOUND，與傳輸層的 rpc.CommonText 同一個來源）。
// apperr 錯誤的 LocalizedMessage 一律標 zh-TW（rpc 的 errorLocale，I-server 隨 apperr 一起處理），所以這裡也固定取 zh-TW：文字與標籤一致，
// 不隨請求語系取到別的語系的文字再被標成 zh-TW。目錄裡缺這個 key 時 TestInquiryNotFoundTextIsInTheCatalog 失敗（不會在執行期悄悄變成空白）。
func errInquiryNotFound() error {
	m, _ := uitext.For(uitext.ZhTW).CommonError("NOT_FOUND")
	return apperr.NotFound("%s", m.Text)
}

func toInquiry(row store.SellerCaseInquiry) model.CaseInquiry {
	i := model.CaseInquiry{
		ID: row.ID, CaseID: row.CaseID, StoreID: row.StoreID, Kind: model.InquiryKind(row.Kind), Message: row.Message,
		CreatedAt: row.CreatedAt, CreatedByAdminID: row.CreatedByAdminID, DueAt: row.DueAt,
	}
	if row.ReplyReceivedAt != nil && row.ReplyBody != nil && row.ReplyChannel != nil && row.ReplyRecordedAt != nil && row.ReplyRecordedByAdminID != nil {
		i.Reply = &model.InquiryReply{
			Body: *row.ReplyBody, Channel: model.InquiryReplyChannel(*row.ReplyChannel), ReceivedAt: *row.ReplyReceivedAt,
			RecordedAt: *row.ReplyRecordedAt, RecordedByAdminID: *row.ReplyRecordedByAdminID,
		}
	}
	if row.WithdrawnAt != nil && row.WithdrawnByAdminID != nil && row.WithdrawCause != nil {
		i.Withdrawn = &model.InquiryWithdrawal{At: *row.WithdrawnAt, AdminID: *row.WithdrawnByAdminID, Cause: model.InquiryWithdrawCause(*row.WithdrawCause)}
	}
	return i
}

// adminInquiries 把一批詢問列變成後台的檢視：已回覆的載入附件（每批一支固定查詢），並附上此刻可以做的動作。
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

// readInquiry 讀一筆詢問的後台檢視；在 q 的連線上執行，所以可以在指令的 transaction 裡讀自己寫的。
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

// inquiryBlocked 是詢問不能做動作的錯誤：訊息是第一個原因的句子；沒有原因回 nil。
func inquiryBlocked(blocks []model.InquiryBlock) error {
	if len(blocks) == 0 {
		return nil
	}
	return apperr.FailedPrecondition("%s", blocks[0].Text().Text)
}

// inquiryRequestHash 是正規化後的建立輸入（不含 request_id；時間已截到微秒）以固定欄位順序編碼的 SHA-256：冪等重送比對「內容是否相同」的唯一定義，
// 同 measure 套件的 requestHash。沒有指定期限時編碼 "-"（重送時「現在」已經不同，預設期限不能進 hash）。
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
// 期限不設時是建立後 InquiryDefaultDue，設定時必須在建立後 InquiryMinDue 到 InquiryMaxDue 之間。同一個 transaction 讓案件的版本前進。不發事件：
// 還沒有消費者（A8 要通知賣家時，依 CARRYOVER「消費者出現時再加它需要的事件」加上）。
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
		if _, err := stint.LockAccountOf(ctx, q, found.StoreID); err != nil {
			return err
		}
		locked, err := lockCase(ctx, q, in.CaseID, 0)
		if err != nil {
			return err
		}
		// 兩個同一個 request_id 的並行重送：後到的在這裡等到先到的 commit，再查一次就找到它。
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
			// 同一個 (管理員, request_id) 在上面兩次查詢之後才出現：不可能發生在已鎖住案件列之後，除非它屬於別的案件（內容不同）。
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

// replayInquiry 查 (管理員, request_id)：命中且內容相同回傳詢問目前的狀態與案件；內容不同回 InvalidInput；沒帶 request_id 或沒命中回 false。
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
	if !slices.Contains(model.InquiryReplyChannelValues(), in.Channel) {
		return model.RecordCaseInquiryReplyResult{}, invalidOn("Channel", text.InquiryChannelRequired())
	}
	if in.ReceivedAt.IsZero() {
		return model.RecordCaseInquiryReplyResult{}, invalidOn("ReceivedAt", text.InquiryReceiveTimeRange())
	}
	body := strings.TrimSpace(in.Body)
	if n := utf8.RuneCountInString(body); n == 0 || n > model.InquiryReplyMaxRunes {
		return model.RecordCaseInquiryReplyResult{}, invalidOn("Body", text.InquiryBodyLength())
	}
	if err := mediaref.EvidenceTokens(in.EvidenceTokens); err != nil {
		return model.RecordCaseInquiryReplyResult{}, err
	}
	received := in.ReceivedAt.UTC().Truncate(time.Microsecond)
	if _, err := store.New(s.d.DB.Conn(ctx)).CaseInquiryByID(ctx, in.InquiryID); errors.Is(err, pgx.ErrNoRows) {
		return model.RecordCaseInquiryReplyResult{}, errInquiryNotFound()
	} else if err != nil {
		return model.RecordCaseInquiryReplyResult{}, err
	}
	evidence, err := mediaref.FinalizeEvidence(ctx, s.d.Media, media.Uploader{ID: in.AdminID, Admin: true}, in.EvidenceTokens)
	if err != nil {
		return model.RecordCaseInquiryReplyResult{}, err
	}
	var result model.RecordCaseInquiryReplyResult
	err = s.d.DB.InTx(ctx, func(ctx context.Context) error {
		q := store.New(s.d.DB.Conn(ctx))
		locked, err := s.lockInquiry(ctx, q, in.InquiryID)
		if err != nil {
			return err
		}
		inquiry := toInquiry(locked)
		blocks := inquiry.Availability().RecordReply
		if len(blocks) > 0 && blocks[0] == model.InquiryBlockReplied {
			// 先前已記過：回原本那一則，這次輸入的內容沒有寫入。
			result, err = s.inquiryResult(ctx, q, inquiry)
			result.AlreadyRecorded = true
			return err
		}
		if err := inquiryBlocked(blocks); err != nil {
			return err
		}
		now := s.now()
		// 收到的時間是管理員讀到的一個時刻，精確度是秒（後台預設填入的現在也是秒）；詢問的建立時間有微秒。下限以秒比較：與詢問建立同一秒的收到時間
		// 視為建立當下記下（資料庫保證收到不早於建立），所以剛建立就記下的回覆不會因為精確度的差而被擋。
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
		for i, e := range evidence {
			if err := q.InsertCaseInquiryEvidence(ctx, store.InsertCaseInquiryEvidenceParams{
				InquiryID: inquiry.ID, Position: int32(i), AssetID: e.AssetID, ObjectKey: e.ObjectKey, //nolint:gosec // 位置最多 MaxEvidence
			}); err != nil {
				return err
			}
		}
		if len(evidence) > 0 {
			if err := mediaref.Changed(ctx, q, s.d.Jobs, now, mediaref.EvidenceRef(inquiry.ID)); err != nil {
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

// lockInquiry 依鎖順序鎖住詢問：先讀詢問（店家與案件不會變）→ 店家列 → 案件列 → 詢問列；回傳鎖住之後的詢問列。
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

// inquiryResult 讀詢問與它的案件現在的檢視（指令回應用）。
func (s *Service) inquiryResult(ctx context.Context, q *store.Queries, inquiry model.CaseInquiry) (model.RecordCaseInquiryReplyResult, error) {
	view, err := readInquiry(ctx, q, inquiry.ID)
	if err != nil {
		return model.RecordCaseInquiryReplyResult{}, err
	}
	c, err := readCase(ctx, q, inquiry.CaseID, s.now())
	return model.RecordCaseInquiryReplyResult{Inquiry: view, Case: c}, err
}

// withdrawOpenInquiry 在案件結案（決定，或通報都移走）的 transaction 內，由系統撤回案件還沒結束的詢問（最多一筆）；作者是結案的管理員。呼叫方已鎖住店家列與案件列，
// 這個 UPDATE 同時鎖住詢問列（鎖順序：店家列 → 案件列 → 詢問列）。
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
