package hold

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"time"
	"uuid"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/Will413028/hawkr/backend/internal/modules/media/internal/model"
	"github.com/Will413028/hawkr/backend/internal/modules/media/internal/store"
)

const (
	gateHold   = "HOLD"
	gateDelete = "DELETE"

	lockNotAvailable    = "55P03"
	foreignKeyViolation = "23503"
	holdGateFKey        = "asset_holds_gate_fkey"
	holdReleaseFKey     = "asset_hold_releases_hold_fkey"
	gateAttempts        = 3
)

const LockTimeout = 100 * time.Millisecond

type execer interface {
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
}

func SetLockTimeout(ctx context.Context, c execer) error {
	if _, err := c.Exec(ctx, fmt.Sprintf("SET LOCAL lock_timeout = '%dms'", LockTimeout.Milliseconds())); err != nil {
		return fmt.Errorf("media：設定鎖逾時：%w", err)
	}
	return nil
}

func isHoldForeignKey(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == foreignKeyViolation && (pgErr.ConstraintName == holdGateFKey || pgErr.ConstraintName == holdReleaseFKey)
}

func classifyHoldInsert(err error) error {
	if isHoldForeignKey(err) {
		return fmt.Errorf("%w：%w", model.ErrHoldContended, err)
	}
	return err
}

func IsContention(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == lockNotAvailable || isHoldForeignKey(err)
}

func sortedIDs(assetIDs []uuid.UUID) []uuid.UUID {
	ids := slices.Clone(assetIDs)
	slices.SortFunc(ids, func(a, b uuid.UUID) int { return slices.Compare(a[:], b[:]) })
	return slices.Compact(ids)
}

func Place(ctx context.Context, q *store.Queries, assetIDs []uuid.UUID, source string, sourceID uuid.UUID, until, now time.Time, secureOnly bool) error {
	if source == "" {
		return fmt.Errorf("media：保全的來源不得為空")
	}
	if !until.After(now) {
		return fmt.Errorf("media：保全期限 %s 不在現在 %s 之後", until, now)
	}
	ids := sortedIDs(assetIDs)
	if len(ids) == 0 {
		return nil
	}
	states, err := q.ListAssetStates(ctx, ids)
	if err != nil {
		return fmt.Errorf("media：讀取要保全的資產：%w", err)
	}
	if len(states) != len(ids) {
		return model.ErrAssetNotFound
	}
	for _, r := range states {
		if !r.Confirmed {
			return model.ErrAssetNotFound
		}
	}
	if secureOnly {
		for _, r := range states {
			if model.Visibility(r.Visibility) != model.VisibilitySecure {
				return model.ErrNotSecure
			}
		}
	}
	for _, r := range states {
		if model.AssetState(r.State) != model.AssetStaged {
			return model.ErrAssetGone
		}
	}
	for range gateAttempts {
		if err := q.InsertHoldGates(ctx, store.InsertHoldGatesParams{Ids: ids, Now: now}); err != nil {
			return fmt.Errorf("media：登記保全閘門：%w", err)
		}
		gates, err := q.ListAssetGates(ctx, ids)
		if err != nil {
			return fmt.Errorf("media：讀取保全閘門：%w", err)
		}
		for _, g := range gates {
			if g.Kind == gateDelete {
				return model.ErrAssetGone
			}
		}
		if len(gates) != len(ids) {
			continue
		}
		if err := classifyHoldInsert(q.InsertHolds(ctx, store.InsertHoldsParams{Ids: ids, Source: source, SourceID: sourceID, Until: until, Now: now})); err != nil {
			return fmt.Errorf("media：登記保全：%w", err)
		}
		return nil
	}
	return model.ErrHoldContended
}

func Extend(ctx context.Context, q *store.Queries, source string, sourceID uuid.UUID, until, now time.Time) error {
	live, err := q.IsHoldSourceLive(ctx, store.IsHoldSourceLiveParams{Source: source, SourceID: sourceID, Now: now})
	if err != nil {
		return fmt.Errorf("media：查保全來源：%w", err)
	}
	if !live {
		return model.ErrHoldNotFound
	}
	if err := classifyHoldInsert(q.ExtendHolds(ctx, store.ExtendHoldsParams{Source: source, SourceID: sourceID, Until: until, Now: now})); err != nil {
		return fmt.Errorf("media：延長保全：%w", err)
	}
	return nil
}

func Release(ctx context.Context, q *store.Queries, source string, sourceID uuid.UUID, now time.Time) error {
	if err := classifyHoldInsert(q.ReleaseHolds(ctx, store.ReleaseHoldsParams{Source: source, SourceID: sourceID, Now: now})); err != nil {
		return fmt.Errorf("media：解除保全：%w", err)
	}
	return nil
}

func Settle(ctx context.Context, q *store.Queries, assetID uuid.UUID, now time.Time) (int, error) {
	if _, err := q.LockHoldGate(ctx, assetID); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return 0, nil
		}
		return 0, fmt.Errorf("media：鎖定資產 %s 的保全閘門：%w", assetID, err)
	}
	removed, err := q.DeleteDeadHolds(ctx, store.DeleteDeadHoldsParams{AssetID: assetID, Now: now})
	if err != nil {
		return 0, fmt.Errorf("media：清除資產 %s 到期或解除的保全：%w", assetID, err)
	}
	if _, err := q.DeleteIdleHoldGate(ctx, assetID); err != nil {
		return 0, fmt.Errorf("media：撤除資產 %s 沒有保全的閘門：%w", assetID, err)
	}
	return int(removed), nil
}

func Claim(ctx context.Context, q *store.Queries, assetID uuid.UUID, now time.Time) (bool, error) {
	if _, err := Settle(ctx, q, assetID, now); err != nil {
		return false, err
	}
	if err := q.InsertDeleteGate(ctx, store.InsertDeleteGateParams{AssetID: assetID, Now: now}); err != nil {
		return false, fmt.Errorf("media：登記資產 %s 的刪除閘門：%w", assetID, err)
	}
	kind, err := q.GetGateKind(ctx, assetID)
	if errors.Is(err, pgx.ErrNoRows) {
		return true, nil
	}
	if err != nil {
		return false, fmt.Errorf("media：讀取資產 %s 的閘門：%w", assetID, err)
	}
	return kind == gateDelete, nil
}

func ClaimAll(ctx context.Context, q *store.Queries, assetIDs []uuid.UUID, now time.Time) (bool, error) {
	for _, id := range sortedIDs(assetIDs) {
		won, err := Claim(ctx, q, id, now)
		if err != nil || !won {
			return false, err
		}
	}
	return true, nil
}

func ReleaseSource(ctx context.Context, q *store.Queries, source string, sourceID uuid.UUID) error {
	ids, err := q.ListAssetsOfHoldSource(ctx, store.ListAssetsOfHoldSourceParams{Source: source, SourceID: sourceID})
	if err != nil {
		return fmt.Errorf("media：列出來源 %s 的保全：%w", source, err)
	}
	for _, id := range ids {
		if _, err := q.LockHoldGate(ctx, id); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				continue
			}
			return fmt.Errorf("media：鎖定資產 %s 的保全閘門：%w", id, err)
		}
		if _, err := q.DeleteHoldsOfSource(ctx, store.DeleteHoldsOfSourceParams{AssetID: id, Source: source, SourceID: sourceID}); err != nil {
			return fmt.Errorf("media：清除資產 %s 的來源保全：%w", id, err)
		}
		if _, err := q.DeleteIdleHoldGate(ctx, id); err != nil {
			return fmt.Errorf("media：撤除資產 %s 沒有保全的閘門：%w", id, err)
		}
	}
	return nil
}
