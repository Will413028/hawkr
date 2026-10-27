package usecase

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"

	"github.com/Will413028/hawkr/go-pkg/database"
	"github.com/Will413028/hawkr/go-pkg/pgxutil"
	"github.com/Will413028/hawkr/pricebook-service/internal/client/moa"
	"github.com/Will413028/hawkr/pricebook-service/internal/repository/postgres/gen"
)

type RestDayIngest struct {
	client *moa.Client
	pool   *pgxpool.Pool
	tsid   func() int64
	logger *zap.Logger
}

func NewRestDayIngest(c *moa.Client, pool *pgxpool.Pool, tsid func() int64, logger *zap.Logger) *RestDayIngest {
	return &RestDayIngest{client: c, pool: pool, tsid: tsid, logger: logger}
}

func restDayMarketReject(d moa.RestDay) moa.RejectReason {
	switch {
	case strings.TrimSpace(d.MarketCode) == "":
		return moa.RejectMissingMarketCode
	case strings.TrimSpace(d.MarketName) == "":
		return moa.RejectMissingMarketName
	}
	return ""
}

func (u *RestDayIngest) Handle(ctx context.Context) (int, error) {
	days, err := u.client.FetchRestDays(ctx)
	if err != nil {
		return 0, fmt.Errorf("抓取休市日曆失敗: %w", err)
	}

	if len(days) == 0 {
		return 0, fmt.Errorf("休市日曆 API 未回傳任何資料")
	}

	q := gen.New(database.GetDBTX(ctx, u.pool))
	fetchedAt := pgxutil.TimestamptzParam(time.Now().UTC())

	marketIDs := make(map[string]int64, len(days))
	rejected := newRejectedRegister(sourceMOARestDays)
	skipped := make(map[string]struct{})
	written := 0
	for _, d := range days {
		if _, bad := skipped[d.MarketCode]; bad {
			continue
		}
		marketID, ok := marketIDs[d.MarketCode]
		if !ok {

			if reason := restDayMarketReject(d); reason != "" {
				rejected.add("", d.MarketCode, moa.ToROCDate(d.Date), reason)
				skipped[d.MarketCode] = struct{}{}
				continue
			}
			marketID, err = q.UpsertMarket(ctx, gen.UpsertMarketParams{
				ID:         u.tsid(),
				MarketCode: d.MarketCode,
				Name:       d.MarketName,
			})
			if err != nil {
				return written, fmt.Errorf("upsert market %q 失敗: %w", d.MarketCode, err)
			}
			marketIDs[d.MarketCode] = marketID
		}

		date := d.Date
		if err := q.UpsertMarketRestDay(ctx, gen.UpsertMarketRestDayParams{
			ID:         u.tsid(),
			MarketID:   marketID,
			MarketType: stringOrNil(d.MarketType),
			RestDate:   pgxutil.DateParam(&date),
			FetchedAt:  fetchedAt,
		}); err != nil {
			return written, fmt.Errorf("upsert market_rest_day 失敗（market=%s, type=%s, date=%s）: %w",
				d.MarketCode, d.MarketType, d.Date.Format("2006-01-02"), err)
		}
		written++
	}

	var rejectStats IngestStats
	if err := rejected.flush(ctx, q, &rejectStats); err != nil {
		return written, err
	}

	deleted, err := q.DeleteLegacyRestDaysWithoutMarketType(ctx)
	if err != nil {
		u.logger.Error("清理無 market_type 的舊休市列失敗，不影響本輪已寫入的新列",
			zap.Error(err))
	} else if deleted > 0 {

		u.logger.Info("清理無 market_type 的舊休市列", zap.Int64("deleted", deleted))
	}
	return written, nil
}
