package usecase

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"go.uber.org/zap"

	"github.com/Will413028/hawkr/go-pkg/database"
	"github.com/Will413028/hawkr/go-pkg/pgxutil"
	"github.com/Will413028/hawkr/pricebook-service/internal/client/moa"
	"github.com/Will413028/hawkr/pricebook-service/internal/domain"
	"github.com/Will413028/hawkr/pricebook-service/internal/repository/postgres/gen"
)

const DeclareLookbackDays = 7

func (u *PriceIngest) DeclareDue(ctx context.Context, from, through time.Time) error {
	if through.Before(from) {
		return fmt.Errorf("宣告區間反轉：from=%s 晚於 through=%s",
			from.Format("2006-01-02"), through.Format("2006-01-02"))
	}

	q := gen.New(database.GetDBTX(ctx, u.pool))

	markets, err := q.ListActiveMarkets(ctx)
	if err != nil {
		return fmt.Errorf("取得 active 市場清單失敗: %w", err)
	}
	if len(markets) == 0 {

		return fmt.Errorf("market 表沒有任何 active 市場，無法宣告攝取分區")
	}

	restDays, err := q.ListRestDaysInRange(ctx, gen.ListRestDaysInRangeParams{
		FromDate: pgxutil.DateParam(&from),
		ToDate:   pgxutil.DateParam(&through),
	})
	if err != nil {
		return fmt.Errorf("取得休市日失敗: %w", err)
	}
	rest := make(map[partitionKey]struct{}, len(restDays))
	for _, r := range restDays {
		rest[partitionKey{marketID: r.MarketID, date: r.RestDate.Time}] = struct{}{}
	}

	var (
		marketIDs  []int64
		tradeDates []pgtype.Date
		statuses   []int16
	)
	for _, m := range markets {
		for d := from; !d.After(through); d = d.AddDate(0, 0, 1) {
			day := d
			status := domain.PartitionStatusPending
			if _, isRest := rest[partitionKey{marketID: m.ID, date: day}]; isRest {
				status = domain.PartitionStatusRestDay
			}
			marketIDs = append(marketIDs, m.ID)
			tradeDates = append(tradeDates, pgxutil.DateParam(&day))
			statuses = append(statuses, status)
		}
	}

	if err := q.DeclarePartitions(ctx, gen.DeclarePartitionsParams{
		MarketIds:  marketIDs,
		TradeDates: tradeDates,
		Statuses:   statuses,
		DeclaredAt: pgxutil.TimestamptzParam(time.Now().UTC()),
	}); err != nil {
		return fmt.Errorf("宣告攝取分區失敗: %w", err)
	}
	return nil
}

type partitionKey struct {
	marketID int64
	date     time.Time
}

func (u *PriceIngest) HandleDue(ctx context.Context) (IngestStats, error) {
	var stats IngestStats

	q := gen.New(database.GetDBTX(ctx, u.pool))

	pending, err := q.ListPendingPartitions(ctx, domain.PartitionStatusPending)
	if err != nil {
		return stats, fmt.Errorf("取得待補分區失敗: %w", err)
	}
	if len(pending) == 0 {
		return stats, nil
	}

	markets, err := q.ListActiveMarkets(ctx)
	if err != nil {
		return stats, fmt.Errorf("取得 active 市場清單失敗: %w", err)
	}
	codeByID := make(map[int64]string, len(markets))
	for _, m := range markets {
		codeByID[m.ID] = m.MarketCode
	}

	datesByMarket := make(map[int64][]time.Time)
	for _, p := range pending {
		datesByMarket[p.MarketID] = append(datesByMarket[p.MarketID], p.TradeDate.Time)
	}

	availableAt := pgxutil.TimestamptzParam(time.Now().UTC())
	cache := &upsertCache{
		groupIDs:  make(map[string]int64),
		cropIDs:   make(map[string]int64),
		marketIDs: make(map[string]int64),
	}
	batchSize := u.batchSize
	if batchSize <= 0 {
		batchSize = ingestBatchSize
	}

	outcome := newPartitionOutcome()
	rejected := newRejectedRegister(sourceMOATrades)

	for marketID, dates := range datesByMarket {
		marketCode, ok := codeByID[marketID]
		if !ok {

			zap.L().Warn("price_ingest: 待補分區的市場已非 active，本輪跳過",
				zap.Int64("market_id", marketID), zap.Int("date_count", len(dates)))
			continue
		}

		for _, seg := range mergeConsecutiveDates(dates) {
			records, _, err := u.fetchAllWithEmptyRetry(ctx, q, seg.start, seg.end, marketCode)
			if err != nil {
				return stats, fmt.Errorf("抓取行情失敗（market=%s, %s~%s）: %w",
					marketCode, seg.start.Format("2006-01-02"), seg.end.Format("2006-01-02"), err)
			}
			stats.Fetched += len(records)

			var valid []moa.ValidTradeRecord
			for _, r := range records {
				day, parseErr := moa.ParseROCDate(r.TransDate)
				if parseErr != nil {

					stats.Skipped++
					continue
				}
				key := partitionKey{marketID: marketID, date: day}
				v, reason := r.Validate()
				switch {
				case reason == "":
					valid = append(valid, v)
					outcome.markComplete(key)
				case reason.IsDataDefect():

					rejected.add(r.CropCode, r.MarketCode, r.TransDate, reason)
					outcome.markComplete(key)
				default:
					outcome.markRestDay(key)
				}
			}

			for i := 0; i < len(valid); i += batchSize {
				hi := i + batchSize
				if hi > len(valid) {
					hi = len(valid)
				}
				inserted, revised, skipped, err := u.ingestBatch(ctx, q, cache, valid[i:hi], availableAt)
				if err != nil {
					return stats, fmt.Errorf("批次寫入行情失敗（market=%s）: %w", marketCode, err)
				}
				stats.Inserted += inserted
				stats.Revised += revised
				stats.Skipped += skipped
			}
		}
	}

	for _, p := range pending {
		key := partitionKey{marketID: p.MarketID, date: p.TradeDate.Time}
		if _, ok := codeByID[p.MarketID]; !ok {
			continue
		}
		stats.PartitionsAttempted++
		outcome.markMissingIfUntouched(key)
	}

	if err := rejected.flush(ctx, q, &stats); err != nil {
		return stats, err
	}
	if err := u.persistPartitionOutcome(ctx, q, outcome); err != nil {
		return stats, err
	}
	return stats, nil
}

type partitionOutcome struct {
	complete map[partitionKey]struct{}
	restDay  map[partitionKey]struct{}
	missing  map[partitionKey]struct{}
}

func newPartitionOutcome() *partitionOutcome {
	return &partitionOutcome{
		complete: make(map[partitionKey]struct{}),
		restDay:  make(map[partitionKey]struct{}),
		missing:  make(map[partitionKey]struct{}),
	}
}

func (o *partitionOutcome) markComplete(k partitionKey) {
	o.complete[k] = struct{}{}
	delete(o.restDay, k)
}

func (o *partitionOutcome) markRestDay(k partitionKey) {
	if _, done := o.complete[k]; done {
		return
	}
	o.restDay[k] = struct{}{}
}

func (o *partitionOutcome) markMissingIfUntouched(k partitionKey) {
	if _, done := o.complete[k]; done {
		return
	}
	if _, rest := o.restDay[k]; rest {
		return
	}
	o.missing[k] = struct{}{}
}

func (u *PriceIngest) persistPartitionOutcome(ctx context.Context, q *gen.Queries, o *partitionOutcome) error {
	now := time.Now().UTC()

	if ids, dates := splitKeys(o.complete); len(ids) > 0 {
		if err := q.MarkPartitionsComplete(ctx, gen.MarkPartitionsCompleteParams{
			MarketIds: ids, TradeDates: dates,
			CompleteStatus: domain.PartitionStatusComplete,
			CompletedAt:    &now,
		}); err != nil {
			return fmt.Errorf("標記分區完成失敗: %w", err)
		}
	}

	if ids, dates := splitKeys(o.restDay); len(ids) > 0 {
		if err := q.MarkPartitionsRestDay(ctx, gen.MarkPartitionsRestDayParams{
			MarketIds: ids, TradeDates: dates,
			RestDayStatus: domain.PartitionStatusRestDay,
			PendingStatus: domain.PartitionStatusPending,
		}); err != nil {
			return fmt.Errorf("標記分區休市失敗: %w", err)
		}
	}

	if ids, dates := splitKeys(o.missing); len(ids) > 0 {
		lastErr := "上游未回傳這一天的任何資料"
		if err := q.MarkPartitionsAttempted(ctx, gen.MarkPartitionsAttemptedParams{
			MarketIds: ids, TradeDates: dates,
			AttemptedAt:     &now,
			LastError:       &lastErr,
			MaxAttempts:     domain.MaxIngestAttempts,
			AbandonedStatus: domain.PartitionStatusAbandoned,
		}); err != nil {
			return fmt.Errorf("標記分區嘗試失敗: %w", err)
		}
	}
	return nil
}

func splitKeys(set map[partitionKey]struct{}) ([]int64, []pgtype.Date) {
	if len(set) == 0 {
		return nil, nil
	}
	keys := make([]partitionKey, 0, len(set))
	for k := range set {
		keys = append(keys, k)
	}
	sortPartitionKeys(keys)

	ids := make([]int64, 0, len(keys))
	dates := make([]pgtype.Date, 0, len(keys))
	for _, k := range keys {
		day := k.date
		ids = append(ids, k.marketID)
		dates = append(dates, pgxutil.DateParam(&day))
	}
	return ids, dates
}

func sortPartitionKeys(keys []partitionKey) {
	sort.Slice(keys, func(i, j int) bool {
		if keys[i].marketID != keys[j].marketID {
			return keys[i].marketID < keys[j].marketID
		}
		return keys[i].date.Before(keys[j].date)
	})
}
