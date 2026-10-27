package postgres

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"time"

	"github.com/Will413028/hawkr/go-pkg/codemeta"
	"github.com/Will413028/hawkr/go-pkg/pgxutil"
	"github.com/Will413028/hawkr/pricebook-service/internal/domain"
	"github.com/Will413028/hawkr/pricebook-service/internal/repository/postgres/gen"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type marketOverviewRepository struct{ pool *pgxpool.Pool }

func NewMarketOverviewRepository(pool *pgxpool.Pool) domain.MarketOverviewRepository {
	return &marketOverviewRepository{pool: pool}
}

type overviewCandidate struct {
	CropID        int64   `json:"crop_id"`
	MarketID      *int64  `json:"market_id"`
	WatchTargetID *int64  `json:"watch_target_id"`
	CropCode      string  `json:"crop_code"`
	CropName      string  `json:"crop_name"`
	VariantName   *string `json:"variant_name"`
	MarketCode    *string `json:"market_code"`
	MarketName    *string `json:"market_name"`
	IsWatched     bool    `json:"is_watched"`
	Active        bool    `json:"active"`
}

func (r *marketOverviewRepository) LoadSnapshot(ctx context.Context, accountID int64, query domain.MarketOverviewQuery, cutoff time.Time) (domain.MarketOverviewSnapshot, error) {
	var empty domain.MarketOverviewSnapshot
	if cutoff.IsZero() {
		return empty, fmt.Errorf("market overview: cutoff must not be zero")
	}
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		return empty, err
	}
	defer func() { _ = tx.Rollback(context.WithoutCancel(ctx)) }()
	q := gen.New(tx)
	valid, err := q.OverviewMarketValid(ctx, query.MarketCode)
	if err != nil {
		return empty, err
	}
	if !valid {
		return empty, domain.ErrInvalidWatchTarget
	}
	// Trade dates are civil Taipei dates, represented as UTC-midnight SQL DATE
	// parameters. The revision cutoff remains the original absolute instant.
	taipei, err := time.LoadLocation("Asia/Taipei")
	if err != nil {
		return empty, err
	}
	local := cutoff.In(taipei)
	to := time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, time.UTC)
	from := to.AddDate(0, 0, 1-query.WindowDays)
	page, err := q.OverviewCandidates(ctx, gen.OverviewCandidatesParams{
		MarketCode: query.MarketCode, AccountID: accountID, Scope: query.Scope,
		AllScope: codemeta.PricebookOverviewScopeAll.Value, SearchQuery: query.Query,
		PageSize: int64(query.PageSize), PageOffset: int64(query.Page-1) * int64(query.PageSize),
		Cutoff: pgxutil.TimestamptzParam(cutoff), ToDate: pgxutil.DateParam(&to),
	})
	if err != nil {
		return empty, err
	}
	var candidates []overviewCandidate
	if err := json.Unmarshal([]byte(page.Items), &candidates); err != nil {
		return empty, fmt.Errorf("decode overview candidates: %w", err)
	}
	var options []struct {
		MarketCode string `json:"market_code"`
		Name       string `json:"name"`
	}
	if err := json.Unmarshal([]byte(page.MarketOptions), &options); err != nil {
		return empty, fmt.Errorf("decode overview markets: %w", err)
	}
	out := domain.MarketOverviewSnapshot{Cutoff: cutoff.UTC(), TotalItems: page.TotalItems,
		Items: make([]domain.MarketOverviewRow, len(candidates)), MarketOptions: make([]domain.MarketRef, 0, len(options))}
	for _, option := range options {
		out.MarketOptions = append(out.MarketOptions, domain.MarketRef{MarketCode: option.MarketCode, Name: option.Name})
	}
	cropIDs, marketIDs := make([]int64, len(candidates)), make([]int64, len(candidates))
	for i, c := range candidates {
		cropIDs[i] = c.CropID
		if c.MarketID != nil {
			marketIDs[i] = *c.MarketID
		}
		out.Items[i] = domain.MarketOverviewRow{WatchTargetID: c.WatchTargetID, CropCode: c.CropCode,
			CropName: c.CropName, VariantName: c.VariantName, MarketCode: c.MarketCode,
			MarketName: c.MarketName, IsWatched: c.IsWatched, Active: c.Active,
			Points: []domain.MarketOverviewPoint{}, SourceDigests: []string{}}
	}
	// Execute even for an empty page: query count stays independent of page size.
	observations, err := q.OverviewObservations(ctx, gen.OverviewObservationsParams{
		CropIds: cropIDs, MarketIds: marketIDs, FromDate: pgxutil.DateParam(&from),
		ToDate: pgxutil.DateParam(&to), Cutoff: pgxutil.TimestamptzParam(cutoff),
	})
	if err != nil {
		return empty, err
	}
	for _, observation := range observations {
		if observation.Ordinal < 1 || observation.Ordinal > int64(len(out.Items)) {
			return empty, fmt.Errorf("market overview: invalid result ordinal")
		}
		row := &out.Items[observation.Ordinal-1]
		price := NumericToPtr(observation.AvgPrice)
		if price == nil {
			return empty, fmt.Errorf("market overview: aggregate has no price")
		}
		actual := &domain.MarketOverviewActual{TradeDate: observation.TradeDate.Time, AvgPrice: *price, VolumeKg: NumericToPtr(observation.VolumeKg)}
		switch observation.DayRank {
		case 1:
			row.LatestActual = actual
		case 2:
			row.PreviousActual = actual
		}
		if observation.InWindow {
			row.Points = append(row.Points, domain.MarketOverviewPoint{TradeDate: actual.TradeDate, AvgPrice: *price})
		}
		if absolute := NumericToPtr(observation.AbsoluteChange); absolute != nil {
			direction := codemeta.PricebookChangeDirectionFlat.Value
			if observation.ChangeSign > 0 {
				direction = codemeta.PricebookChangeDirectionUp.Value
			} else if observation.ChangeSign < 0 {
				direction = codemeta.PricebookChangeDirectionDown.Value
			}
			row.Change = &domain.MarketOverviewChange{Absolute: *absolute, Percent: NumericToPtr(observation.PercentChange), Direction: direction}
		}
		row.SourceDigests = append(row.SourceDigests, observation.SourceDigests...)
		if observation.AvailableAt.Time.After(row.AvailableAt) {
			row.AvailableAt = observation.AvailableAt.Time.UTC()
		}
		row.VolumeWeightFallback = row.VolumeWeightFallback || observation.VolumeWeightFallback
	}
	for i := range out.Items {
		slices.Sort(out.Items[i].SourceDigests)
		out.Items[i].SourceDigests = slices.Compact(out.Items[i].SourceDigests)
	}
	if err := tx.Commit(ctx); err != nil {
		return empty, err
	}
	return out, nil
}
