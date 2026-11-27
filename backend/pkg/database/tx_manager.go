package database

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

type txCtxKey struct{}

// TxManager provides transaction management functionality.
type TxManager interface {
	// WithTx executes the given function within a transaction.
	// If the function returns an error, the transaction is rolled back.
	// If the function returns nil, the transaction is committed.
	// If the function panics or calls runtime.Goexit, the transaction is rolled back and the panic propagates.
	WithTx(ctx context.Context, fn func(ctx context.Context) error) error
}

type pgxTxManager struct {
	pool txBeginner
}

type txBeginner interface {
	Begin(ctx context.Context) (pgx.Tx, error)
}

// NewTxManager creates a new TxManager with the given connection pool.
func NewTxManager(pool *pgxpool.Pool) TxManager {
	return &pgxTxManager{pool: pool}
}

// WithTx executes the function within a transaction.
// Supports nested transactions by reusing an existing transaction from context.
func (m *pgxTxManager) WithTx(ctx context.Context, fn func(ctx context.Context) error) error {
	if _, ok := ctx.Value(txCtxKey{}).(pgx.Tx); ok {
		return fn(ctx)
	}

	tx, err := m.pool.Begin(ctx)
	if err != nil {
		return err
	}

	txCtx := context.WithValue(ctx, txCtxKey{}, tx)

	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback(context.WithoutCancel(ctx))
		}
	}()

	if err := fn(txCtx); err != nil {
		return err
	}

	committed = true
	return tx.Commit(ctx)
}

// TxFromContext extracts the transaction from context, if present.
// Returns nil if no transaction is found.
func TxFromContext(ctx context.Context) pgx.Tx {
	if tx, ok := ctx.Value(txCtxKey{}).(pgx.Tx); ok {
		return tx
	}
	return nil
}

// DBTX represents a database connection or transaction.
// This interface is compatible with sqlc generated code.
type DBTX interface {
	Exec(ctx context.Context, sql string, arguments ...any) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
	CopyFrom(ctx context.Context, tableName pgx.Identifier, columnNames []string, rowSrc pgx.CopyFromSource) (int64, error)
}

// GetDBTX returns the transaction from context if present, otherwise returns the pool.
// Use this in repository implementations to automatically use transactions when available.
func GetDBTX(ctx context.Context, pool *pgxpool.Pool) DBTX {
	if tx := TxFromContext(ctx); tx != nil {
		return tx
	}
	return pool
}
