package usecase

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/Will413028/hawkr/go-pkg/codemeta"
	"github.com/Will413028/hawkr/pricebook-service/internal/config"
	"github.com/Will413028/hawkr/pricebook-service/internal/domain"
	"github.com/Will413028/hawkr/pricebook-service/internal/domain/forecast"
)

var ErrInvalidMarketOverviewInput = errors.New("invalid market overview input")

// MarketOverviewInput preserves optional query presence through validation.
type MarketOverviewInput struct {
	Scope, Query                           string
	WindowDays, Page, PageSize, MarketCode *string
}
type MarketOverviewItem struct {
	domain.MarketOverviewRow
	Status forecast.PredictionStatus
	Meta   DataMeta
}
type MarketOverviewResult struct {
	SnapshotID             string
	Query                  domain.MarketOverviewQuery
	GeneratedAt            time.Time
	SourceDataAsOf         *time.Time
	Source, Currency, Unit string
	MarketOptions          []domain.MarketRef
	Items                  []MarketOverviewItem
	TotalItems             int64
	HasNext                bool
}
type MarketOverview struct {
	repo      domain.MarketOverviewRepository
	freshness forecast.FreshnessConfig
}

func NewMarketOverview(repo domain.MarketOverviewRepository, cfg *config.Config) *MarketOverview {
	return &MarketOverview{repo: repo, freshness: freshnessConfigFrom(cfg.Inference)}
}
func positiveDecimal(value string) (int64, error) {
	if value == "" {
		return 0, ErrInvalidMarketOverviewInput
	}
	for _, c := range value {
		if c < '0' || c > '9' {
			return 0, ErrInvalidMarketOverviewInput
		}
	}
	n, err := strconv.ParseInt(value, 10, 64)
	if err != nil || n <= 0 {
		return 0, ErrInvalidMarketOverviewInput
	}
	return n, nil
}
func overviewNumber(value *string, fallback int) (int, error) {
	if value == nil {
		return fallback, nil
	}
	n, err := positiveDecimal(*value)
	return int(n), err
}
func validateMarketOverviewInput(in MarketOverviewInput) (domain.MarketOverviewQuery, error) {
	q := domain.MarketOverviewQuery{Scope: in.Scope, Query: in.Query}
	if q.Scope == "" {
		q.Scope = codemeta.PricebookOverviewScopeAll.Value
	}
	if q.Scope != codemeta.PricebookOverviewScopeAll.Value && q.Scope != codemeta.PricebookOverviewScopeWatchlist.Value {
		return q, ErrInvalidMarketOverviewInput
	}
	if in.MarketCode != nil {
		if q.Scope == codemeta.PricebookOverviewScopeWatchlist.Value {
			return q, ErrInvalidMarketOverviewInput
		}
		q.MarketCode = *in.MarketCode
	}
	var err error
	if q.WindowDays, err = overviewNumber(in.WindowDays, 7); err != nil {
		return q, err
	}
	if q.WindowDays != 7 && q.WindowDays != 14 && q.WindowDays != 30 {
		return q, ErrInvalidMarketOverviewInput
	}
	if q.Page, err = overviewNumber(in.Page, 1); err != nil {
		return q, err
	}
	if q.PageSize, err = overviewNumber(in.PageSize, 20); err != nil {
		return q, err
	}
	if q.PageSize > 100 {
		return q, ErrInvalidMarketOverviewInput
	}
	// Ensure OFFSET multiplication cannot overflow the repository's int64 boundary.
	if int64(q.Page-1) > int64(^uint64(0)>>1)/int64(q.PageSize) {
		return q, ErrInvalidMarketOverviewInput
	}
	return q, nil
}
func (u *MarketOverview) Get(ctx context.Context, accountID int64, in MarketOverviewInput, now time.Time) (MarketOverviewResult, error) {
	q, err := validateMarketOverviewInput(in)
	if err != nil {
		return MarketOverviewResult{}, err
	}
	if accountID <= 0 {
		return MarketOverviewResult{}, ErrInvalidMarketOverviewInput
	}
	snapshot, err := u.repo.LoadSnapshot(ctx, accountID, q, now)
	if err != nil {
		return MarketOverviewResult{}, err
	}
	out := MarketOverviewResult{Query: q, GeneratedAt: now, Source: dataMetaSource, Currency: dataMetaCurrency, Unit: dataMetaUnit, MarketOptions: snapshot.MarketOptions, Items: make([]MarketOverviewItem, 0, len(snapshot.Items)), TotalItems: snapshot.TotalItems}
	if out.MarketOptions == nil {
		out.MarketOptions = []domain.MarketRef{}
	}
	offset := int64(q.Page-1) * int64(q.PageSize)
	out.HasNext = offset < snapshot.TotalItems && int64(q.PageSize) < snapshot.TotalItems-offset
	for _, row := range snapshot.Items {
		item := MarketOverviewItem{MarketOverviewRow: row, Status: forecast.PredictionStatusUnavailable, Meta: unavailableDataMeta(now)}
		if row.Active && row.LatestActual != nil {
			date := forecast.DateFromTime(row.LatestActual.TradeDate)
			var availableAt *time.Time
			if !row.AvailableAt.IsZero() {
				availableAt = &row.AvailableAt
			}
			item.Meta = observedDataMeta(now, u.freshness, availableAt, &date, forecast.DatasetDigestFromSourceDigests(row.SourceDigests))
			item.Status = forecast.PredictionStatusReady
			if row.VolumeWeightFallback {
				item.Meta.QualityReasonCodes = append(item.Meta.QualityReasonCodes, forecast.QualityReasonVolumeWeightFallback)
			}
			if out.SourceDataAsOf == nil || item.Meta.SourceDataAsOf.After(*out.SourceDataAsOf) {
				out.SourceDataAsOf = item.Meta.SourceDataAsOf
			}
		} else {
			item.LatestActual = nil
			item.PreviousActual = nil
			item.Change = nil
			item.Points = nil
		}
		if item.Points == nil {
			item.Points = []domain.MarketOverviewPoint{}
		}
		out.Items = append(out.Items, item)
	}
	// Hash the complete account-scoped query and snapshot. No account ID is exposed.
	encoded, err := json.Marshal(struct {
		AccountID int64
		Query     domain.MarketOverviewQuery
		Snapshot  domain.MarketOverviewSnapshot
	}{accountID, q, snapshot})
	if err != nil {
		return MarketOverviewResult{}, fmt.Errorf("encode market snapshot: %w", err)
	}
	out.SnapshotID = fmt.Sprintf("sha256:%x", sha256.Sum256(encoded))
	return out, nil
}
