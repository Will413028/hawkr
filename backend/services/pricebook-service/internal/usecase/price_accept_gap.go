package usecase

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"go.uber.org/zap"

	"github.com/Will413028/hawkr/go-pkg/database"
	"github.com/Will413028/hawkr/go-pkg/pgxutil"
	"github.com/Will413028/hawkr/pricebook-service/internal/client/moa"
	"github.com/Will413028/hawkr/pricebook-service/internal/domain"
	"github.com/Will413028/hawkr/pricebook-service/internal/repository/postgres/gen"
)

type AcceptGapOutcome int

const (
	AcceptGapAccepted AcceptGapOutcome = iota + 1

	AcceptGapUpstreamHasData

	AcceptGapRestDay
)

func (o AcceptGapOutcome) String() string {
	switch o {
	case AcceptGapAccepted:
		return "accepted_gap"
	case AcceptGapUpstreamHasData:
		return "complete"
	case AcceptGapRestDay:
		return "rest_day"
	default:
		return "unknown"
	}
}

var ErrPartitionNotAbandoned = errors.New("分區不在 abandoned 狀態，不可接受")

var ErrPartitionNotFound = errors.New("找不到這一格分區")

func (u *PriceIngest) AcceptGap(
	ctx context.Context,
	marketCode string,
	tradeDate time.Time,
	acceptedBy string,
	reason string,
) (AcceptGapOutcome, error) {
	acceptedBy = strings.TrimSpace(acceptedBy)
	reason = strings.TrimSpace(reason)
	if acceptedBy == "" {
		return 0, fmt.Errorf("acceptedBy 不可為空")
	}
	if reason == "" {
		return 0, fmt.Errorf("reason 不可為空")
	}

	day := tradeDate.UTC().Truncate(24 * time.Hour)
	q := gen.New(database.GetDBTX(ctx, u.pool))

	marketID, err := q.GetMarketIDByCode(ctx, marketCode)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return 0, fmt.Errorf("找不到市場代碼 %s", marketCode)
		}
		return 0, fmt.Errorf("查市場失敗（market=%s）: %w", marketCode, err)
	}

	current, err := q.GetPartition(ctx, gen.GetPartitionParams{
		MarketID: marketID, TradeDate: pgxutil.DateParam(&day),
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return 0, fmt.Errorf("%w（market=%s, date=%s）",
				ErrPartitionNotFound, marketCode, day.Format("2006-01-02"))
		}
		return 0, fmt.Errorf("查分區失敗: %w", err)
	}
	if current.Status != domain.PartitionStatusAbandoned {
		return 0, fmt.Errorf("%w：目前 status=%d（market=%s, date=%s）",
			ErrPartitionNotAbandoned, current.Status, marketCode, day.Format("2006-01-02"))
	}

	records, _, err := u.fetchAllWithEmptyRetry(ctx, q, day, day, marketCode)
	if err != nil {
		return 0, fmt.Errorf("重新向上游查證失敗（market=%s, date=%s）: %w",
			marketCode, day.Format("2006-01-02"), err)
	}

	var trades []moa.TradeRecord
	sawSentinel := false
	for _, r := range records {
		parsed, perr := moa.ParseROCDate(r.TransDate)
		if perr != nil || !parsed.Equal(day) {

			continue
		}

		if _, reason := r.Validate(); reason != "" && !reason.IsDataDefect() {
			sawSentinel = true
			continue
		}
		trades = append(trades, r)
	}

	switch {
	case len(trades) > 0:
		return u.completeInsteadOfAccepting(ctx, q, marketID, day, marketCode, trades)
	case sawSentinel:
		return u.restDayInsteadOfAccepting(ctx, q, marketID, day, marketCode)
	}

	rows, err := q.AcceptPartitionGap(ctx, gen.AcceptPartitionGapParams{
		MarketID:        marketID,
		TradeDate:       pgxutil.DateParam(&day),
		AcceptedStatus:  domain.PartitionStatusAcceptedGap,
		AbandonedStatus: domain.PartitionStatusAbandoned,
		AcceptedAt:      timePtr(time.Now().UTC()),
		AcceptedBy:      &acceptedBy,
		AcceptedReason:  &reason,
	})
	if err != nil {
		return 0, fmt.Errorf("標記接受缺口失敗: %w", err)
	}
	if rows != 1 {

		return 0, fmt.Errorf("%w：更新影響 %d 列（可能已被其他程序改動）",
			ErrPartitionNotAbandoned, rows)
	}

	zap.L().Info("price_ingest: 已接受缺口（上游確認無資料）",
		zap.String("market_code", marketCode),
		zap.String("trade_date", day.Format("2006-01-02")),
		zap.String("accepted_by", acceptedBy),
		zap.String("reason", reason))
	return AcceptGapAccepted, nil
}

func (u *PriceIngest) completeInsteadOfAccepting(
	ctx context.Context,
	q *gen.Queries,
	marketID int64,
	day time.Time,
	marketCode string,
	trades []moa.TradeRecord,
) (AcceptGapOutcome, error) {
	cache := &upsertCache{
		groupIDs:  make(map[string]int64),
		cropIDs:   make(map[string]int64),
		marketIDs: make(map[string]int64),
	}
	availableAt := pgxutil.TimestamptzParam(time.Now().UTC())

	rejected := newRejectedRegister(sourceMOATrades)
	valid := make([]moa.ValidTradeRecord, 0, len(trades))
	for _, r := range trades {
		v, reason := r.Validate()
		switch {
		case reason == "":
			valid = append(valid, v)
		case reason.IsDataDefect():
			rejected.add(r.CropCode, r.MarketCode, r.TransDate, reason)
		}
	}
	if _, _, _, err := u.ingestBatch(ctx, q, cache, valid, availableAt); err != nil {
		return 0, fmt.Errorf("寫入重查到的行情失敗（market=%s）: %w", marketCode, err)
	}
	var rejectStats IngestStats
	if err := rejected.flush(ctx, q, &rejectStats); err != nil {
		return 0, err
	}

	outcome := newPartitionOutcome()
	outcome.markComplete(partitionKey{marketID: marketID, date: day})
	if err := u.persistPartitionOutcome(ctx, q, outcome); err != nil {
		return 0, err
	}

	zap.L().Warn("price_ingest: 拒絕接受缺口——上游現在有資料，已寫入並標記完成",
		zap.String("market_code", marketCode),
		zap.String("trade_date", day.Format("2006-01-02")),
		zap.Int("record_count", len(trades)))
	return AcceptGapUpstreamHasData, nil
}

func (u *PriceIngest) restDayInsteadOfAccepting(
	ctx context.Context,
	q *gen.Queries,
	marketID int64,
	day time.Time,
	marketCode string,
) (AcceptGapOutcome, error) {
	rows, err := q.ResolveAbandonedPartitionAsRestDay(ctx, gen.ResolveAbandonedPartitionAsRestDayParams{
		MarketID:        marketID,
		TradeDate:       pgxutil.DateParam(&day),
		RestDayStatus:   domain.PartitionStatusRestDay,
		AbandonedStatus: domain.PartitionStatusAbandoned,
	})
	if err != nil {
		return 0, fmt.Errorf("標記休市失敗: %w", err)
	}
	if rows != 1 {
		return 0, fmt.Errorf("%w：更新影響 %d 列（可能已被其他程序改動）",
			ErrPartitionNotAbandoned, rows)
	}

	zap.L().Warn("price_ingest: 拒絕接受缺口——上游回的是休市哨兵，已標記休市",
		zap.String("market_code", marketCode),
		zap.String("trade_date", day.Format("2006-01-02")))
	return AcceptGapRestDay, nil
}

func timePtr(t time.Time) *time.Time { return &t }
