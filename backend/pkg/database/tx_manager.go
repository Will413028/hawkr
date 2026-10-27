package database

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// txCtxKey is the context key for storing transactions.
type txCtxKey struct{}

// TxManager provides transaction management functionality.
type TxManager interface {
	// WithTx executes the given function within a transaction.
	// If the function returns an error, the transaction is rolled back.
	// If the function returns nil, the transaction is committed.
	WithTx(ctx context.Context, fn func(ctx context.Context) error) error
}

// pgxTxManager implements TxManager using pgx.
type pgxTxManager struct {
	pool *pgxpool.Pool
}

// NewTxManager creates a new TxManager with the given connection pool.
func NewTxManager(pool *pgxpool.Pool) TxManager {
	return &pgxTxManager{pool: pool}
}

// WithTx executes the function within a transaction.
// Supports nested transactions by reusing an existing transaction from context.
func (m *pgxTxManager) WithTx(ctx context.Context, fn func(ctx context.Context) error) error {
	// Check if already in a transaction (nested call)
	if _, ok := ctx.Value(txCtxKey{}).(pgx.Tx); ok {
		// Execute within existing transaction
		return fn(ctx)
	}

	// Start a new transaction
	tx, err := m.pool.Begin(ctx)
	if err != nil {
		return err
	}

	// Store transaction in context
	txCtx := context.WithValue(ctx, txCtxKey{}, tx)

	// Execute the function
	if err := fn(txCtx); err != nil {
		// Rollback on error, using a context that won't be cancelled
		rollbackCtx := context.WithoutCancel(ctx)
		_ = tx.Rollback(rollbackCtx)
		return err
	}

	// Commit the transaction
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
