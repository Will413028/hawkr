// Package db 提供單體唯一的資料庫連線池與 transaction 邊界（ADR D3、D4）。
//
// 規則：transaction 由最上層的 use case 用 InTx 開（需要一致快照的讀取用 InReadSnapshot）；被呼叫的模組經由 Conn 取得同一個
// transaction，所以跨模組的寫入一起 commit 或一起 rollback。repository 不自己開 transaction。
package db

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// DBTX 是 sqlc 生成碼需要的介面；*pgxpool.Pool 與 pgx.Tx 都滿足它。
type DBTX interface {
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
	CopyFrom(ctx context.Context, table pgx.Identifier, columns []string, src pgx.CopyFromSource) (int64, error)
}

// Config 是連線池的設定。
type Config struct {
	URL      string
	MaxConns int32
	// ConnectRetryBudget 是啟動時等待資料庫可連線的總時間。
	// 部署時 pod 可能比資料庫先起來，所以不在第一次連線失敗就放棄。
	ConnectRetryBudget time.Duration
}

// DB 是連線池加上 transaction 邊界。
type DB struct {
	pool *pgxpool.Pool
}

// Open 建立連線池，並在 ConnectRetryBudget 內等到資料庫可以連線。
//
// 使用 pgx 預設的 statement 快取。若之後在前面加上 transaction pooling 的 PgBouncer，
// 要改成 QueryExecModeExec，否則 prepared statement 會落在別的 backend 連線上。
func Open(ctx context.Context, cfg Config) (*DB, error) {
	pc, err := pgxpool.ParseConfig(cfg.URL)
	if err != nil {
		return nil, fmt.Errorf("db：解析連線字串：%w", err)
	}
	if cfg.MaxConns > 0 {
		pc.MaxConns = cfg.MaxConns
	}
	pool, err := pgxpool.NewWithConfig(ctx, pc)
	if err != nil {
		return nil, fmt.Errorf("db：建立連線池：%w", err)
	}
	if err := pingWithRetry(ctx, pool, cfg.ConnectRetryBudget); err != nil {
		pool.Close()
		return nil, err
	}
	return &DB{pool: pool}, nil
}

func pingWithRetry(ctx context.Context, pool *pgxpool.Pool, budget time.Duration) error {
	deadline := time.Now().Add(budget)
	wait := 200 * time.Millisecond
	for {
		pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		err := pool.Ping(pingCtx)
		cancel()
		if err == nil {
			return nil
		}
		if time.Now().Add(wait).After(deadline) {
			return fmt.Errorf("db：%s 內無法連線：%w", budget, err)
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(wait):
		}
		wait = min(wait*2, 5*time.Second)
	}
}

// Close 關閉連線池。
func (d *DB) Close() { d.pool.Close() }

// Pool 回傳底層連線池，給 River driver 這類需要直接操作連線池的元件用。
func (d *DB) Pool() *pgxpool.Pool { return d.pool }

// Ping 確認資料庫可以連線，給 readiness check 用。
func (d *DB) Ping(ctx context.Context) error { return d.pool.Ping(ctx) }

type txKey struct{}

// TxFrom 回傳 ctx 裡的 transaction。
func TxFrom(ctx context.Context) (pgx.Tx, bool) {
	tx, ok := ctx.Value(txKey{}).(pgx.Tx)
	return tx, ok
}

// Conn 回傳 ctx 裡的 transaction；沒有時回傳連線池。repository 一律經由它執行 SQL。
func (d *DB) Conn(ctx context.Context) DBTX {
	if tx, ok := TxFrom(ctx); ok {
		return tx
	}
	return d.pool
}

// InTx 在 transaction 內執行 fn：fn 回傳錯誤或 panic 就 rollback，否則 commit。
// ctx 裡已經有 transaction 時直接加入它，由最外層決定 commit 或 rollback。
func (d *DB) InTx(ctx context.Context, fn func(ctx context.Context) error) (err error) {
	if _, ok := TxFrom(ctx); ok {
		return fn(ctx)
	}
	tx, err := d.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("db：開始 transaction：%w", err)
	}
	defer func() {
		if p := recover(); p != nil {
			_ = tx.Rollback(context.WithoutCancel(ctx))
			panic(p)
		}
		if err != nil {
			if rbErr := tx.Rollback(context.WithoutCancel(ctx)); rbErr != nil && !errors.Is(rbErr, pgx.ErrTxClosed) {
				err = errors.Join(err, fmt.Errorf("db：rollback：%w", rbErr))
			}
		}
	}()
	if err := fn(context.WithValue(ctx, txKey{}, tx)); err != nil {
		return err
	}
	if err = tx.Commit(ctx); err != nil {
		return fmt.Errorf("db：commit：%w", err)
	}
	return nil
}

// ErrSnapshotInTx 是在已有 transaction 的 ctx 上開讀取快照時回傳的錯誤。
var ErrSnapshotInTx = errors.New("db：讀取快照不能開在 transaction 內")

// InReadSnapshot 在一個唯讀的 REPEATABLE READ transaction 內執行 fn：fn 裡所有 Conn(ctx) 讀到同一份快照（多個語句
// 之間別的 transaction 提交的改動不會被看見），用在「幾個語句要拼成一致的結果」的讀取（跨頁、主表加附表）。
// 這個 transaction 不能寫入；fn 回傳錯誤或 panic 就 rollback，否則 commit。
//
// ctx 已經在 transaction 內時回傳 ErrSnapshotInTx，不加入外層：外層的隔離層級與讀寫模式已經決定，隔離層級不能在
// 查詢之後再提高，靜默加入會讓呼叫端以為拿到快照。呼叫端必須是最上層的讀取 use case。
func (d *DB) InReadSnapshot(ctx context.Context, fn func(ctx context.Context) error) (err error) {
	if _, ok := TxFrom(ctx); ok {
		return ErrSnapshotInTx
	}
	tx, err := d.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		return fmt.Errorf("db：開始讀取快照：%w", err)
	}
	defer func() {
		if p := recover(); p != nil {
			_ = tx.Rollback(context.WithoutCancel(ctx))
			panic(p)
		}
		if err != nil {
			if rbErr := tx.Rollback(context.WithoutCancel(ctx)); rbErr != nil && !errors.Is(rbErr, pgx.ErrTxClosed) {
				err = errors.Join(err, fmt.Errorf("db：rollback：%w", rbErr))
			}
		}
	}()
	if err := fn(context.WithValue(ctx, txKey{}, tx)); err != nil {
		return err
	}
	if err = tx.Commit(ctx); err != nil {
		return fmt.Errorf("db：commit：%w", err)
	}
	return nil
}
