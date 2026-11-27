package postgres

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Will413028/hawkr/go-pkg/database"
	pkgerrors "github.com/Will413028/hawkr/go-pkg/errors"
	"github.com/Will413028/hawkr/go-pkg/errors/bizcode"
	"github.com/Will413028/hawkr/invoice-service/internal/repository/postgres/gen"
)

type baseRepo struct {
	pool *pgxpool.Pool
}

func (r baseRepo) q(ctx context.Context) *gen.Queries {
	return gen.New(database.GetDBTX(ctx, r.pool))
}

func biz(code int32) error {
	return pkgerrors.NewBusiness(code, bizcode.Message(code))
}

func notFound(err error, code int32) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return biz(code)
	}
	return err
}

func nilIfBlank(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}
