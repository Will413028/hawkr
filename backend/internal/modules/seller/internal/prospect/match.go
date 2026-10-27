package prospect

import (
	"context"
	"errors"
	"slices"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/Will413028/hawkr/backend/internal/modules/seller/internal/model"
	"github.com/Will413028/hawkr/backend/internal/modules/seller/internal/store"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
	"github.com/Will413028/hawkr/backend/internal/platform/db"
)

// loadContacts 讀業者的聯絡方式（原值），同類型主要的在前。
func loadContacts(ctx context.Context, q *store.Queries, prospectID int64) ([]model.ContactPoint, error) {
	rows, err := q.ProspectContacts(ctx, prospectID)
	if err != nil {
		return nil, err
	}
	out := make([]model.ContactPoint, len(rows))
	for i, r := range rows {
		out[i] = model.ContactPoint{Kind: model.ContactKind(r.Kind), Value: r.Value, Primary: r.IsPrimary}
	}
	return out, nil
}

// replaceContacts 整批替換業者的聯絡方式（contacts 已正規化，每個類型恰好一個主要的）；比對鍵由解析器推導，沒有鍵的寫成 NULL。
// 每個類型恰好一個主要的由 deferred 外鍵在 commit 時再檢查一次。
func replaceContacts(ctx context.Context, q *store.Queries, prospectID int64, contacts []model.ContactPoint) error {
	if err := q.ClearProspectContacts(ctx, prospectID); err != nil {
		return err
	}
	return addContacts(ctx, q, prospectID, contacts)
}

func addContacts(ctx context.Context, q *store.Queries, prospectID int64, contacts []model.ContactPoint) error {
	if len(contacts) == 0 {
		return nil
	}
	p := store.AddProspectContactsParams{ProspectID: prospectID}
	for _, c := range contacts {
		p.Kinds = append(p.Kinds, string(c.Kind))
		p.Values = append(p.Values, c.Value)
		p.MatchKeys = append(p.MatchKeys, model.ContactMatchKey(c.Kind, c.Value))
		p.Primaries = append(p.Primaries, c.Primary)
	}
	return q.AddProspectContacts(ctx, p)
}

// resolveMatches 檢查這些比對鍵有沒有被其他未封存的業者持有（沒有標記為與 self 不是同一家的；self 為 0 是建立之前）。沒有未處理的
// 相符回 nil；相符的業者全部都在 confirmed 裡，回傳要記下「不是同一家」的業者 ID（呼叫方在同一個 transaction 內記）；
// 否則回 PROSPECT_MATCH_FOUND（model.NewProspectMatchFound），列出全部相符的業者。confirmed 裡沒有相符的 ID 不會被記下。
func resolveMatches(ctx context.Context, q *store.Queries, self int64, keys []string, confirmed []int64) ([]int64, error) {
	holders, err := sharedHolders(ctx, q, self, keys)
	if err != nil {
		return nil, err
	}
	ids := make([]int64, 0, len(holders))
	for _, h := range holders {
		if len(ids) == 0 || ids[len(ids)-1] != h.ProspectID {
			ids = append(ids, h.ProspectID)
		}
	}
	for _, id := range ids {
		if !slices.Contains(confirmed, id) {
			return nil, model.NewProspectMatchFound(ids)
		}
	}
	return ids, nil
}

// sharedHolders 回傳持有這些比對鍵的其他未封存業者與命中的鍵（依業者、鍵排序）；沒有鍵時不查。
func sharedHolders(ctx context.Context, q *store.Queries, self int64, keys []string) ([]store.SharedKeyHoldersRow, error) {
	if len(keys) == 0 {
		return nil, nil
	}
	return q.SharedKeyHolders(ctx, store.SharedKeyHoldersParams{SelfID: self, Keys: keys})
}

// confirmDistinct 記下 prospectID 與 others 不是同一家（較小的 ID 在前，已記的略過）。
func confirmDistinct(ctx context.Context, q *store.Queries, prospectID, adminID int64, now time.Time, others []int64) error {
	if len(others) == 0 {
		return nil
	}
	return q.ConfirmDistinctProspects(ctx, store.ConfirmDistinctProspectsParams{ProspectID: prospectID, AdminID: adminID, Now: now, Others: others})
}

// taxIDHolder 回傳持有這個統編的其他未封存業者（進行中或已入駐）與它的狀態；沒有統編或沒有人持有時 ok 為 false。self 是自己的
// ID（建立時為 0）。在 transaction 內先以統編取 advisory lock，同一個統編的寫入因此排隊，取得鎖之後查到的持有者就是權威。
func taxIDHolder(ctx context.Context, q *store.Queries, self int64, taxID *string) (holder store.LiveProspectByTaxIDRow, ok bool, err error) {
	if taxID == nil {
		return store.LiveProspectByTaxIDRow{}, false, nil
	}
	if _, inTx := db.TxFrom(ctx); inTx {
		if err := q.LockProspectTaxID(ctx, *taxID); err != nil {
			return store.LiveProspectByTaxIDRow{}, false, err
		}
	}
	holder, err = q.LiveProspectByTaxID(ctx, taxID)
	switch {
	case errors.Is(err, pgx.ErrNoRows):
		return store.LiveProspectByTaxIDRow{}, false, nil
	case err != nil:
		return store.LiveProspectByTaxIDRow{}, false, err
	}
	return holder, holder.ProspectID != self, nil
}

// taxIDOp 是撞到統編的操作；句子依操作與持有者的狀態而定。
type taxIDOp int

const (
	opCreate taxIDOp = iota
	opUpdate
	opRestore
)

// errTaxIDHeld 是統編被別的未封存業者持有，一律帶 PROSPECT_TAX_ID_HELD 與 metadata 的 prospectIds（持有者的 ID）：建立回
// AlreadyExists，修改與還原回 FailedPrecondition。修改時持有者是進行中的，該做的是合併；持有者已入駐不能合併，是同一家就封存這筆；
// 還原時是同一家就不需要還原。
func errTaxIDHeld(holder store.LiveProspectByTaxIDRow, op taxIDOp) error {
	id := strconv.FormatInt(holder.ProspectID, 10)
	var e *apperr.Error
	switch {
	case op == opCreate:
		e = apperr.AlreadyExists("統一編號已由業者 %d 使用", holder.ProspectID)
	case op == opRestore:
		e = apperr.FailedPrecondition("統一編號已由業者 %d 使用，是同一家的話不需要還原", holder.ProspectID)
	case model.ProspectStatus(holder.Status) == model.ProspectStatusConverted:
		e = apperr.FailedPrecondition("統一編號已由已入駐的業者 %d 使用；是同一家的話請封存這筆", holder.ProspectID)
	default:
		e = apperr.FailedPrecondition("統一編號已由業者 %d 使用，該做的是合併", holder.ProspectID)
	}
	return e.WithReason(apperr.ReasonProspectTaxIDHeld).WithMetadata("prospectIds", id)
}
