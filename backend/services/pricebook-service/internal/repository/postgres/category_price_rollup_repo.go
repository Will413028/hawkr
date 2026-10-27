package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Will413028/hawkr/go-pkg/database"
	"github.com/Will413028/hawkr/go-pkg/pgxutil"
	"github.com/Will413028/hawkr/pricebook-service/internal/domain"
	"github.com/Will413028/hawkr/pricebook-service/internal/repository/postgres/gen"
)

type categoryPriceRollupRepo struct{ pool *pgxpool.Pool }

func NewCategoryPriceRollupRepository(pool *pgxpool.Pool) domain.CategoryPriceRollupRepository {
	return &categoryPriceRollupRepo{pool: pool}
}

func (r *categoryPriceRollupRepo) Upsert(ctx context.Context, m domain.CategoryPriceRollup) error {
	q := gen.New(database.GetDBTX(ctx, r.pool))

	latest, err := nullableFloatNumeric(m.LatestPrice)
	if err != nil {
		return fmt.Errorf("latest_price 轉換失敗（category_id=%s）: %w", m.CategoryID, err)
	}
	prev, err := nullableFloatNumeric(m.PrevPrice)
	if err != nil {
		return fmt.Errorf("prev_price 轉換失敗（category_id=%s）: %w", m.CategoryID, err)
	}
	var points []byte
	if len(m.Points) > 0 {
		points, err = json.Marshal(m.Points)
		if err != nil {
			return fmt.Errorf("points 序列化失敗（category_id=%s）: %w", m.CategoryID, err)
		}
	}
	var forecastPoints []byte
	if len(m.ForecastPoints) > 0 {
		forecastPoints, err = json.Marshal(m.ForecastPoints)
		if err != nil {
			return fmt.Errorf("forecast_points 序列化失敗（category_id=%s）: %w", m.CategoryID, err)
		}
	}

	return q.UpsertCategoryPriceRollup(ctx, gen.UpsertCategoryPriceRollupParams{
		CategoryID:       m.CategoryID,
		Level:            m.Level,
		Status:           string(m.Status),
		LatestPrice:      latest,
		PrevPrice:        prev,
		Points:           points,
		ForecastPoints:   forecastPoints,
		SourceGroupCount: m.SourceGroupCount,
		PriceLayer:       priceLayerParam(m.PriceLayer),
		Unit:             m.Unit,
		ComputedAt:       pgxutil.TimestamptzParam(m.ComputedAt),
	})
}

func (r *categoryPriceRollupRepo) Get(ctx context.Context, categoryID string) (*domain.CategoryPriceRollup, error) {
	q := gen.New(database.GetDBTX(ctx, r.pool))
	row, err := q.GetCategoryPriceRollup(ctx, categoryID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	out := domain.CategoryPriceRollup{
		CategoryID:       row.CategoryID,
		Level:            row.Level,
		Status:           domain.RollupStatus(row.Status),
		SourceGroupCount: row.SourceGroupCount,
		Unit:             row.Unit,
		ComputedAt:       row.ComputedAt.Time,
	}
	if row.PriceLayer != nil {
		l := domain.PriceLayer(*row.PriceLayer)
		out.PriceLayer = &l
	}
	if row.LatestPrice.Valid {
		f, err := numericToFloat(row.LatestPrice)
		if err != nil {
			return nil, err
		}
		out.LatestPrice = &f
	}
	if row.PrevPrice.Valid {
		f, err := numericToFloat(row.PrevPrice)
		if err != nil {
			return nil, err
		}
		out.PrevPrice = &f
	}
	if len(row.Points) > 0 {
		if err := json.Unmarshal(row.Points, &out.Points); err != nil {
			return nil, fmt.Errorf("points 反序列化失敗（category_id=%s）: %w", categoryID, err)
		}
	}
	if len(row.ForecastPoints) > 0 {
		if err := json.Unmarshal(row.ForecastPoints, &out.ForecastPoints); err != nil {
			return nil, fmt.Errorf("forecast_points 反序列化失敗（category_id=%s）: %w", categoryID, err)
		}
	}
	return &out, nil
}

// nullableFloatNumeric converts a possibly-nil *float64 to pgtype.Numeric,
// nil becoming SQL NULL — same shape as usecase.nullableForecastNumeric
// (internal/usecase/forecast_batch.go:226-231), duplicated here rather than
// exported cross-package because it's a two-line pgtype conversion, not
// shared business logic.
func nullableFloatNumeric(f *float64) (pgtype.Numeric, error) {
	if f == nil {
		return pgtype.Numeric{}, nil
	}
	return NumericFromString(fmt.Sprintf("%.4f", *f))
}

// numericToFloat converts a pgtype.Numeric known to be Valid (caller checks
// .Valid first) to float64 via pgx v5's own Float64Value() — preferred over
// hand-parsing the numeric's internal Int/Exp representation.
func numericToFloat(n pgtype.Numeric) (float64, error) {
	f, err := n.Float64Value()
	if err != nil {
		return 0, fmt.Errorf("numeric to float64: %w", err)
	}
	return f.Float64, nil
}

func priceLayerParam(l *domain.PriceLayer) *string {
	if l == nil {
		return nil
	}
	s := string(*l)
	return &s
}
