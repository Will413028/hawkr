package governance

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
	"uuid"

	"github.com/jackc/pgx/v5"

	"github.com/Will413028/hawkr/backend/internal/modules/seller/internal/model"
	"github.com/Will413028/hawkr/backend/internal/modules/seller/internal/stint"
	"github.com/Will413028/hawkr/backend/internal/modules/seller/internal/store"
	"github.com/Will413028/hawkr/backend/internal/modules/seller/internal/text"
	"github.com/Will413028/hawkr/backend/internal/platform/twtime"
)

// 食安案件的轉報紀錄（step10e-reports-brief R5、設計 step10e-3cb-design §3.2）：記錄轉報、標記不轉報與記下主管機關的回覆。三個指令都不帶 etag，
// 鎖順序是店家列 → 案件列，成功時讓案件的版本與 updated_at 前進。轉報與不轉報是同一列（case_referrals 一案一列），所以互斥由主鍵保證，
// 可用性（CaseFacts.Availability）先擋並給句子。

const (
	// referralTextMaxRunes 是不轉報的原因與主管機關的回覆內容的字數上限（與 DB 的 CHECK 一致）。
	referralTextMaxRunes = 2000
)

// civilDay 把 t 的年月日（t 所在時區的日曆日）取成 UTC 午夜的 time.Time；零值仍是零值。client 傳來的日期已經是這個形狀，這裡只是不信任呼叫端。
func civilDay(t time.Time) time.Time {
	if t.IsZero() {
		return t
	}
	y, m, d := t.Date()
	return time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
}

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

// RecordFDAReferral 記下食安案件已轉報主管機關（轉報本身在系統外以人工完成）：每個案件一筆，記過之後不能修改；已經記過時回傳原本的紀錄（AlreadyRecorded），不覆寫、
// 不動案件的版本。只限平台判定類別是食安、有通報的案件；已標記不轉報是錯誤（不是 already）。不要求先決定（轉報可能先於決定，例如嚴重事件先通報主管機關）。
// 追蹤日期不設時是轉報日（台北）加 ReferralFollowUpDays 天，設定時不早於今天（台北）。不帶 etag。鎖順序：店家列 → 案件列。
func (s *Service) RecordFDAReferral(ctx context.Context, in model.RecordFDAReferralInput) (model.RecordFDAReferralResult, error) {
	if err := model.Positive(in.AdminID); err != nil {
		return model.RecordFDAReferralResult{}, err
	}
	if !slices.Contains(model.FDAChannelValues(), in.Channel) {
		return model.RecordFDAReferralResult{}, invalidOn("Channel", text.ReferralChannelRequired())
	}
	referenceNo := model.Text(in.ReferenceNo)
	if referenceNo != nil && len([]rune(*referenceNo)) > 100 {
		return model.RecordFDAReferralResult{}, invalidOn("ReferenceNo", text.ReferralReferenceNoLength())
	}
	var result model.RecordFDAReferralResult
	err := s.d.DB.InTx(ctx, func(ctx context.Context) error {
		q := store.New(s.d.DB.Conn(ctx))
		locked, facts, err := s.lockedCaseFacts(ctx, q, in.CaseID)
		if err != nil {
			return err
		}
		blocks := facts.Availability().RecordReferral
		if len(blocks) > 0 && blocks[0] != model.CaseBlockReferralRecorded {
			return caseBlocked(blocks, model.CaseActionRecordReferral)
		}
		if len(blocks) > 0 {
			// 先前已記過：回原本的紀錄，這次輸入的內容沒有寫入。
			view, err := readCase(ctx, q, locked.ID, s.now())
			result = model.RecordFDAReferralResult{Case: view, AlreadyRecorded: true}
			return err
		}
		now := s.now()
		followUp := model.DefaultReferralFollowUp(now)
		if in.FollowUpOn != nil {
			followUp = civilDay(*in.FollowUpOn)
			if followUp.IsZero() || followUp.Before(twtime.Day(now)) {
				return invalidOn("FollowUpOn", text.ReferralFollowUpPast())
			}
		}
		channel := string(in.Channel)
		if _, err := q.InsertCaseReferral(ctx, store.InsertCaseReferralParams{
			CaseID: locked.ID, Category: locked.Category, Channel: &channel, ReferenceNo: referenceNo, FollowUpOn: &followUp, Now: now, AdminID: in.AdminID,
		}); err != nil {
			return err
		}
		if err := q.BumpCase(ctx, store.BumpCaseParams{ID: locked.ID, Now: now}); err != nil {
			return err
		}
		view, err := readCase(ctx, q, locked.ID, s.now())
		result = model.RecordFDAReferralResult{Case: view}
		return err
	})
	return result, err
}

// WaiveReferral 標記食安案件不轉報（例如主管機關已經介入），必須附原因。它和轉報是同一列：標記之後不能再記錄轉報，記錄轉報之後也不能標記不轉報（錯誤）；
// 已經標記過時回傳原本的紀錄（AlreadyWaived），不覆寫、不動案件的版本。不轉報讓案件離開 3d 的「待轉報」（判定式是沒有轉報紀錄）。不帶 etag。鎖順序：店家列 → 案件列。
func (s *Service) WaiveReferral(ctx context.Context, in model.WaiveFDAReferralInput) (model.WaiveFDAReferralResult, error) {
	if err := model.Positive(in.AdminID); err != nil {
		return model.WaiveFDAReferralResult{}, err
	}
	reason := strings.TrimSpace(in.Reason)
	if n := utf8.RuneCountInString(reason); n == 0 || n > referralTextMaxRunes {
		return model.WaiveFDAReferralResult{}, invalidOn("Reason", text.ReferralWaiverReasonLength())
	}
	var result model.WaiveFDAReferralResult
	err := s.d.DB.InTx(ctx, func(ctx context.Context) error {
		q := store.New(s.d.DB.Conn(ctx))
		locked, facts, err := s.lockedCaseFacts(ctx, q, in.CaseID)
		if err != nil {
			return err
		}
		blocks := facts.Availability().WaiveReferral
		if len(blocks) > 0 && blocks[0] != model.CaseBlockReferralWaived {
			return caseBlocked(blocks, model.CaseActionWaiveReferral)
		}
		if len(blocks) > 0 {
			view, err := readCase(ctx, q, locked.ID, s.now())
			result = model.WaiveFDAReferralResult{Case: view, AlreadyWaived: true}
			return err
		}
		now := s.now()
		if _, err := q.InsertCaseReferralWaiver(ctx, store.InsertCaseReferralWaiverParams{
			CaseID: locked.ID, Category: locked.Category, Reason: &reason, Now: now, AdminID: in.AdminID,
		}); err != nil {
			return err
		}
		if err := q.BumpCase(ctx, store.BumpCaseParams{ID: locked.ID, Now: now}); err != nil {
			return err
		}
		view, err := readCase(ctx, q, locked.ID, s.now())
		result = model.WaiveFDAReferralResult{Case: view}
		return err
	})
	return result, err
}

// referralReplyRequestHash 是正規化後的回覆輸入（不含 request_id）以固定欄位順序編碼的 SHA-256：冪等重送比對「內容是否相同」的唯一定義，同 inquiryRequestHash。
func referralReplyRequestHash(in model.RecordFDAReferralReplyInput, body string, received time.Time, next *time.Time) []byte {
	h := sha256.New()
	nextText := "-"
	if next != nil {
		nextText = next.Format(time.DateOnly)
	}
	for _, p := range []string{in.CaseID.String(), body, received.Format(time.DateOnly), nextText, strconv.FormatBool(in.ClearFollowUp)} {
		_, _ = fmt.Fprintf(h, "%d:%s|", len(p), p)
	}
	return h.Sum(nil)
}

// RecordReferralReply 記下主管機關對轉報的回覆（只新增；移送通知與結果通知可以是兩則）。只限已記錄轉報的案件：沒有轉報紀錄與已標記不轉報都是錯誤
// （DB 也保證，回覆只能掛在 REFERRED 的列）。收到的日期（台北）不早於轉報日、不晚於今天；追蹤日期依 NextFollowUpOn（取代，不早於今天）與 ClearFollowUp（清掉）更新，
// 兩者都沒有時不變；更新時記下設定的時間（追蹤逾期只看它之後的回覆）。不帶 etag（只新增）；RequestID 選填（AIP-155，範圍是（管理員, request_id））：
// 鎖住案件之後先查重送，同樣的內容回傳案件目前的狀態、不多記一則，內容不同回 invalid_argument。鎖順序：店家列 → 案件列。
func (s *Service) RecordReferralReply(ctx context.Context, in model.RecordFDAReferralReplyInput) (model.AdminCase, error) {
	if err := model.Positive(in.AdminID); err != nil {
		return model.AdminCase{}, err
	}
	body := strings.TrimSpace(in.Body)
	if n := utf8.RuneCountInString(body); n == 0 || n > referralTextMaxRunes {
		return model.AdminCase{}, invalidOn("Body", text.ReferralReplyBodyLength())
	}
	if in.NextFollowUpOn != nil && in.ClearFollowUp {
		return model.AdminCase{}, invalidOn("ClearFollowUp", text.ReferralFollowUpConflict())
	}
	received := civilDay(in.ReceivedOn)
	var nextInput *time.Time
	if in.NextFollowUpOn != nil {
		d := civilDay(*in.NextFollowUpOn)
		nextInput = &d
	}
	var hash []byte
	if in.RequestID != nil {
		hash = referralReplyRequestHash(in, body, received, nextInput)
	}
	var view model.AdminCase
	err := s.d.DB.InTx(ctx, func(ctx context.Context) error {
		q := store.New(s.d.DB.Conn(ctx))
		locked, facts, err := s.lockedCaseFacts(ctx, q, in.CaseID)
		if err != nil {
			return err
		}
		if in.RequestID != nil {
			prior, err := q.CaseReferralReplyByRequest(ctx, store.CaseReferralReplyByRequestParams{AdminID: in.AdminID, RequestID: in.RequestID})
			switch {
			case err == nil && !bytes.Equal(prior.RequestHash, hash):
				return invalidOn("RequestID", text.InquiryRequestIdReused())
			case err == nil:
				view, err = readCase(ctx, q, locked.ID, s.now())
				return err
			case !errors.Is(err, pgx.ErrNoRows):
				return err
			}
		}
		if err := caseBlocked(facts.Availability().RecordReferralReply, model.CaseActionRecordReferralReply); err != nil {
			return err
		}
		now := s.now()
		today := twtime.Day(now)
		if received.IsZero() || received.After(today) || received.Before(twtime.Day(facts.Case.Referral.FDA.ReportedAt)) {
			return invalidOn("ReceivedOn", text.ReferralReceiveDateRange())
		}
		if nextInput != nil && (nextInput.IsZero() || nextInput.Before(today)) {
			return invalidOn("NextFollowUpOn", text.ReferralFollowUpPast())
		}
		n, err := q.InsertCaseReferralReply(ctx, store.InsertCaseReferralReplyParams{
			ID: uuid.NewV7(), CaseID: locked.ID, Body: body, ReceivedOn: received, AdminID: in.AdminID, Now: now, RequestID: in.RequestID, RequestHash: hash,
		})
		if err != nil {
			return err
		}
		if n != 1 {
			return fmt.Errorf("seller：主管機關的回覆的 request_id %v 已存在，鎖住案件之後的查詢卻沒有找到", in.RequestID)
		}
		if nextInput != nil || in.ClearFollowUp {
			if err := q.SetReferralFollowUp(ctx, store.SetReferralFollowUpParams{CaseID: locked.ID, FollowUpOn: nextInput, Now: now}); err != nil {
				return err
			}
		}
		if err := q.BumpCase(ctx, store.BumpCaseParams{ID: locked.ID, Now: now}); err != nil {
			return err
		}
		view, err = readCase(ctx, q, locked.ID, now)
		return err
	})
	return view, err
}

// lockedCaseFacts 是轉報三個指令共同的開頭：讀案件、鎖店家列、鎖案件列（不比對版本）、讀鎖住之後的事實。案件不存在回 not_found。
func (s *Service) lockedCaseFacts(ctx context.Context, q *store.Queries, caseID uuid.UUID) (store.SellerCase, model.CaseFacts, error) {
	found, err := q.CaseByID(ctx, caseID)
	if errors.Is(err, pgx.ErrNoRows) {
		return store.SellerCase{}, model.CaseFacts{}, errCaseNotFound()
	}
	if err != nil {
		return store.SellerCase{}, model.CaseFacts{}, err
	}
	if _, err := stint.LockAccountOf(ctx, q, found.StoreID); err != nil {
		return store.SellerCase{}, model.CaseFacts{}, err
	}
	locked, err := lockCase(ctx, q, caseID, 0)
	if err != nil {
		return store.SellerCase{}, model.CaseFacts{}, err
	}
	facts, err := caseFactsOf(ctx, q, locked.ID)
	return locked, facts, err
}
