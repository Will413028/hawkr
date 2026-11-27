// Package adminaudit 是後台動作的平台紀錄（D62；ADR 2026-11-14-platform-admin-audit-log）：platform 擁有 schema adminaudit（db/schema.sql），
// 每個後台寫入 RPC 的 use case 在領域變更的同一個 transaction 內呼叫 Record 寫一筆（誰、什麼動作、對誰、什麼時候），讀取端是 cmd/hawkr 的營運子命令（List）。
//
// 只增不改：本套件只有 Record 與 List，沒有改寫或刪除的路徑。Record 不在 transaction 內呼叫會失敗（ErrNoTransaction）：紀錄與領域變更同生同滅，
// 領域變更 rollback 時紀錄跟著消失（TestRecordRollsBackWithTheCallersTransaction）。detail 只放小的結構化內容，不放 secret、個資或原始 payload。
package adminaudit

import (
	"context"
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"regexp"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
	"github.com/Will413028/hawkr/backend/internal/platform/db"
)

//go:embed db/migrations/*.sql db/migrations/atlas.sum
var migrationFiles embed.FS

// Schema 是 adminaudit 的 schema 名稱。
const Schema = "adminaudit"

// Migrations 回傳 adminaudit schema 的 migration，給 cmd/hawkr 登記。
func Migrations() fs.FS {
	sub, err := fs.Sub(migrationFiles, "db/migrations")
	if err != nil {
		panic(err)
	}
	return sub
}

// MaxDetailBytes 是 detail 的大小上限，量法只有一種：資料庫存的 jsonb 的文字輸出（octet_length(detail::text)，schema 的 records_detail_check）。
// jsonb 的文字輸出（鍵排序、冒號與逗號後有空白）與 Go 的 json.Marshal 不同長，Go 端無法精確預測，所以大小只由約束把關，Record 把違反它的錯誤對映成 InvalidInput。
const MaxDetailBytes = 2048

const detailCheckConstraint = "records_detail_check"

// ErrNoTransaction 表示 Record 在 transaction 外呼叫：紀錄與領域變更必須同一個 transaction。
var ErrNoTransaction = errors.New("adminaudit: Record must run inside a transaction")

var targetTypePattern = regexp.MustCompile(`^[a-z][a-z0-9_]{0,39}$`)

// Actor 是做這個動作的管理員與動作：Action 是後台 RPC 的 procedure 名稱（handler 從 req.Spec().Procedure 取），use case 原樣帶進 Record。
type Actor struct {
	AdminID int64
	Action  string
}

// Entry 是要記錄的一筆：Actor、對象（TargetType 是小寫底線的種類，例如 payment_account；TargetID 是對象在它自己模組的 ID 文字）與小的結構化 Detail。
type Entry struct {
	Actor      Actor
	TargetType string
	TargetID   string
	Detail     map[string]any
}

// Record 是讀回的一筆。
type Record struct {
	ID         int64
	OccurredAt time.Time
	AdminID    int64
	Action     string
	TargetType string
	TargetID   string
	Detail     json.RawMessage
}

// Log 是後台動作紀錄。
type Log struct {
	db  *db.DB
	now func() time.Time
}

// New 建立 Log；clock 由組裝點注入。
func New(d *db.DB, now func() time.Time) *Log {
	if d == nil || now == nil {
		panic("adminaudit: missing dependency")
	}
	return &Log{db: d, now: now}
}

const insertSQL = `INSERT INTO adminaudit.records (occurred_at, admin_id, action, target_type, target_id, detail)
VALUES ($1, $2, $3, $4, $5, $6)`

// Record 在呼叫方的 transaction 內寫一筆紀錄；不在 transaction 內回 ErrNoTransaction。欄位不合（管理員 ID 非正、沒有動作、對象種類或 ID 不合、detail 超過 MaxDetailBytes）回 InvalidInput，
// 呼叫方的 transaction 隨之失敗：沒有紀錄的後台寫入不成立。
func (l *Log) Record(ctx context.Context, e Entry) error {
	if _, ok := db.TxFrom(ctx); !ok {
		return ErrNoTransaction
	}
	if err := e.validate(); err != nil {
		return err
	}
	detail := []byte("{}")
	if len(e.Detail) > 0 {
		var err error
		if detail, err = json.Marshal(e.Detail); err != nil {
			return apperr.InvalidInput("後台紀錄的 detail 不是有效的 JSON")
		}
	}
	_, err := l.db.Conn(ctx).Exec(ctx, insertSQL, l.now().UTC(), e.Actor.AdminID, e.Actor.Action, e.TargetType, e.TargetID, detail)
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23514" && pgErr.ConstraintName == detailCheckConstraint {
		return apperr.InvalidInput("後台紀錄的 detail 超過 %d 位元組", MaxDetailBytes)
	}
	return err
}

func (e Entry) validate() error {
	switch {
	case e.Actor.AdminID <= 0:
		return apperr.InvalidInput("後台紀錄需要管理員 ID")
	case e.Actor.Action == "" || len(e.Actor.Action) > 200:
		return apperr.InvalidInput("後台紀錄需要動作（1 到 200 位元組）")
	case !targetTypePattern.MatchString(e.TargetType):
		return apperr.InvalidInput("後台紀錄的對象種類不合")
	case e.TargetID == "" || len(e.TargetID) > 255:
		return apperr.InvalidInput("後台紀錄需要對象 ID（1 到 255 位元組）")
	}
	return nil
}

// Filter 是 List 的條件；零值的欄位不篩選。Since 含、Until 不含。
type Filter struct {
	AdminID    int64
	Action     string
	TargetType string
	TargetID   string
	Since      time.Time
	Until      time.Time
	// Limit 是最多回幾筆（預設 100，上限 1000）。
	Limit int
}

const (
	defaultLimit = 100
	maxLimit     = 1000
)

const listSQL = `SELECT id, occurred_at, admin_id, action, target_type, target_id, detail
FROM adminaudit.records
WHERE ($1::bigint = 0 OR admin_id = $1)
  AND ($2::text = '' OR action = $2)
  AND ($3::text = '' OR target_type = $3)
  AND ($4::text = '' OR target_id = $4)
  AND ($5::timestamptz IS NULL OR occurred_at >= $5)
  AND ($6::timestamptz IS NULL OR occurred_at < $6)
ORDER BY occurred_at DESC, id DESC
LIMIT $7`

// List 依條件讀回紀錄，新的在前。
func (l *Log) List(ctx context.Context, f Filter) ([]Record, error) {
	limit := f.Limit
	switch {
	case limit < 0:
		return nil, apperr.InvalidInput("limit 不得為負")
	case limit == 0:
		limit = defaultLimit
	case limit > maxLimit:
		return nil, apperr.InvalidInput("limit 不得超過 %d", maxLimit)
	}
	rows, err := l.db.Conn(ctx).Query(ctx, listSQL, f.AdminID, f.Action, f.TargetType, f.TargetID, nullTime(f.Since), nullTime(f.Until), limit)
	if err != nil {
		return nil, err
	}
	out, err := pgx.CollectRows(rows, func(r pgx.CollectableRow) (Record, error) {
		var rec Record
		err := r.Scan(&rec.ID, &rec.OccurredAt, &rec.AdminID, &rec.Action, &rec.TargetType, &rec.TargetID, &rec.Detail)
		return rec, err
	})
	if err != nil {
		return nil, fmt.Errorf("adminaudit: list: %w", err)
	}
	return out, nil
}

func nullTime(t time.Time) *time.Time {
	if t.IsZero() {
		return nil
	}
	return &t
}
