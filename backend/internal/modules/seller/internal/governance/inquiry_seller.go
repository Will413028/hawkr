package governance

import (
	"context"
	"errors"
	"strings"
	"unicode/utf8"
	"uuid"

	"github.com/jackc/pgx/v5"

	"github.com/Will413028/hawkr/backend/internal/modules/media"
	"github.com/Will413028/hawkr/backend/internal/modules/seller/internal/mediaref"
	"github.com/Will413028/hawkr/backend/internal/modules/seller/internal/model"
	"github.com/Will413028/hawkr/backend/internal/modules/seller/internal/store"
	"github.com/Will413028/hawkr/backend/internal/modules/seller/internal/text"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
	"github.com/Will413028/hawkr/backend/internal/platform/uitext"
)

// 賣家端的詢問以帳號為範圍：詢問屬於這個帳號任何一個 stint（含已關閉的店）才讀得到、回覆得了；關店不收回回覆的義務與機會
// （詢問開放中就能回覆，案件結案時系統撤回），所以不要求店家營運中。別人的與不存在的詢問一律 NotFound。

// ownsInquiry 是賣家端授權的唯一判斷：詢問所在的店家（storeAccountID）屬於呼叫的帳號。
func ownsInquiry(storeAccountID, accountID int64) bool {
	return accountID > 0 && storeAccountID == accountID
}

// ownedInquiry 讀一筆呼叫帳號擁有的詢問列；不擁有與不存在都回 NotFound。
func ownedInquiry(ctx context.Context, q *store.Queries, accountID int64, id uuid.UUID) (store.SellerCaseInquiry, error) {
	row, err := q.CaseInquiryByID(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return row, errInquiryNotFound()
	}
	if err != nil {
		return row, err
	}
	owner, err := q.StoreByID(ctx, row.StoreID)
	if err != nil {
		return row, err
	}
	if !ownsInquiry(owner.AccountID, accountID) {
		return store.SellerCaseInquiry{}, errInquiryNotFound()
	}
	return row, nil
}

// sellerInquiryBlockText 是賣家回覆被擋下的句子（語系 l）；不認得的原因回零值。
func sellerInquiryBlockText(l uitext.Locale, b model.InquiryBlock) uitext.Message {
	switch b {
	case model.InquiryBlockWithdrawn:
		return text.InquirySellerReplyUnavailableWithdrawn(l)
	case model.InquiryBlockReplied:
		return text.InquirySellerReplyUnavailableReplied(l)
	}
	return uitext.Message{}
}

func sellerReplyBlocked(l uitext.Locale, blocks []model.InquiryBlock) error {
	if len(blocks) == 0 {
		return nil
	}
	return apperr.FailedPrecondition("%s", sellerInquiryBlockText(l, blocks[0]).Text)
}

// sellerInquiries 把一批詢問列變成賣家的檢視：附件只留數量（賣家不讀回自己上傳的附件，也不輸出物件鍵）。
func sellerInquiries(ctx context.Context, q *store.Queries, rows []store.SellerCaseInquiry) ([]model.SellerInquiry, error) {
	views, err := adminInquiries(ctx, q, rows)
	if err != nil {
		return nil, err
	}
	out := make([]model.SellerInquiry, len(views))
	for i, v := range views {
		out[i] = model.SellerInquiry{CaseInquiry: v.CaseInquiry, CanReply: len(v.Availability.RecordReply) == 0}
		if r := out[i].Reply; r != nil {
			out[i].AttachmentCount = len(r.Evidence)
			r.Evidence = nil
		}
	}
	return out, nil
}

// AccountInquiries 是賣家帳號全部店家上的詢問，新到舊（keyset），含已回覆（含管理員代記的系統外回覆）與已撤回的唯讀紀錄；
// PendingReplyCount 是同一個快照裡待回覆的總數，不受篩選與分頁影響。
func (s *Service) AccountInquiries(ctx context.Context, in model.AccountInquiryListInput) (model.SellerInquiryPage, error) {
	if err := model.Positive(in.AccountID); err != nil {
		return model.SellerInquiryPage{}, err
	}
	size, err := inquiryPageSize(in.PageSize)
	if err != nil {
		return model.SellerInquiryPage{}, err
	}
	params := store.AccountCaseInquiriesParams{AccountID: in.AccountID, PageSize: size + 1}
	if in.State != nil {
		if *in.State != model.InquiryStateOpen && *in.State != model.InquiryStateReplied && *in.State != model.InquiryStateWithdrawn {
			return model.SellerInquiryPage{}, apperr.InvalidInputOn([]string{"State"}, "%s", text.ListStateInvalid().String())
		}
		st := string(*in.State)
		params.State = &st
	}
	if in.After != nil {
		params.AfterCreatedAt, params.AfterID = &in.After.CreatedAt, &in.After.ID
	}
	var page model.SellerInquiryPage
	err = s.d.DB.InReadSnapshot(ctx, func(ctx context.Context) error {
		q := store.New(s.d.DB.Conn(ctx))
		rows, err := q.AccountCaseInquiries(ctx, params)
		if err != nil {
			return err
		}
		more := len(rows) > int(size)
		if more {
			rows = rows[:size]
		}
		if page.Items, err = sellerInquiries(ctx, q, rows); err != nil {
			return err
		}
		if more {
			last := page.Items[size-1]
			page.Next = &model.InquiryCursor{CreatedAt: last.CreatedAt, ID: last.ID}
		}
		page.PendingReplyCount, err = q.CountAccountOpenCaseInquiries(ctx, in.AccountID)
		return err
	})
	return page, err
}

// AccountInquiry 讀賣家帳號的一筆詢問。
func (s *Service) AccountInquiry(ctx context.Context, accountID int64, id uuid.UUID) (model.SellerInquiry, error) {
	if err := model.Positive(accountID); err != nil {
		return model.SellerInquiry{}, err
	}
	var out model.SellerInquiry
	err := s.d.DB.InReadSnapshot(ctx, func(ctx context.Context) error {
		q := store.New(s.d.DB.Conn(ctx))
		row, err := ownedInquiry(ctx, q, accountID, id)
		if err != nil {
			return err
		}
		views, err := sellerInquiries(ctx, q, []store.SellerCaseInquiry{row})
		if err != nil {
			return err
		}
		out = views[0]
		return nil
	})
	return out, err
}

// ReplyToInquiry 賣家在賣家中心回覆自己店家的詢問（內容加最多 MaxEvidence 份附件），回覆之後詢問結束。一則詢問一次回覆：已回覆（含管理員記下的）
// 與已撤回回 failed_precondition，重送也一樣（不寫入）。逾期後仍可回覆，直到撤回或案件結案（逾期標記照舊留給 admin）。收到與記下是同一個時刻，案件的
// 「待決定」從此刻重新起算。必須在 transaction 外呼叫：附件（機敏上傳的 staged token）先以賣家帳號為上傳者轉正，擁有者與狀態先預檢（不屬於這個帳號的詢問不轉正附件），
// 轉正之後沒有寫入的資產由 media 的 GC 在建立滿 7 天後回收。鎖順序：店家列 → 案件列 → 詢問列。
func (s *Service) ReplyToInquiry(ctx context.Context, in model.ReplyToInquiryInput) (model.SellerInquiry, error) {
	if err := model.Positive(in.AccountID); err != nil {
		return model.SellerInquiry{}, err
	}
	l := uitext.LocaleFrom(ctx)
	body := strings.TrimSpace(in.Body)
	if n := utf8.RuneCountInString(body); n == 0 || n > model.InquiryReplyMaxRunes {
		return model.SellerInquiry{}, invalidOn("Body", text.InquirySellerReplyBodyLength(l))
	}
	if err := mediaref.EvidenceTokens(in.EvidenceTokens); err != nil {
		return model.SellerInquiry{}, err
	}
	pre, err := ownedInquiry(ctx, store.New(s.d.DB.Conn(ctx)), in.AccountID, in.InquiryID)
	if err != nil {
		return model.SellerInquiry{}, err
	}
	if err := sellerReplyBlocked(l, toInquiry(pre).Availability().RecordReply); err != nil {
		return model.SellerInquiry{}, err
	}
	evidence, err := mediaref.FinalizeEvidence(ctx, s.d.Media, media.Uploader{ID: in.AccountID}, in.EvidenceTokens)
	if err != nil {
		return model.SellerInquiry{}, err
	}
	var out model.SellerInquiry
	err = s.d.DB.InTx(ctx, func(ctx context.Context) error {
		q := store.New(s.d.DB.Conn(ctx))
		locked, err := s.lockInquiry(ctx, q, in.InquiryID)
		if err != nil {
			return err
		}
		inquiry := toInquiry(locked)
		if err := sellerReplyBlocked(l, inquiry.Availability().RecordReply); err != nil {
			return err
		}
		now := s.now()
		if _, err := q.ReplyCaseInquiryBySeller(ctx, store.ReplyCaseInquiryBySellerParams{Body: body, Now: now, AccountID: in.AccountID, ID: inquiry.ID}); err != nil {
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
		row, err := q.CaseInquiryByID(ctx, inquiry.ID)
		if err != nil {
			return err
		}
		views, err := sellerInquiries(ctx, q, []store.SellerCaseInquiry{row})
		if err != nil {
			return err
		}
		out = views[0]
		return nil
	})
	return out, err
}
