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

func sharedHolders(ctx context.Context, q *store.Queries, self int64, keys []string) ([]store.SharedKeyHoldersRow, error) {
	if len(keys) == 0 {
		return nil, nil
	}
	return q.SharedKeyHolders(ctx, store.SharedKeyHoldersParams{SelfID: self, Keys: keys})
}

func confirmDistinct(ctx context.Context, q *store.Queries, prospectID, adminID int64, now time.Time, others []int64) error {
	if len(others) == 0 {
		return nil
	}
	return q.ConfirmDistinctProspects(ctx, store.ConfirmDistinctProspectsParams{ProspectID: prospectID, AdminID: adminID, Now: now, Others: others})
}

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

type taxIDOp int

const (
	opCreate taxIDOp = iota
	opUpdate
	opRestore
)

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
