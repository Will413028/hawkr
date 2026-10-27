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

// baseRepo 是本 package 每個 repository 的唯一共用部分。
//
// 🔴 它刻意只持有 pool 而沒有 *gen.Queries 欄位：querier 必須**每次呼叫現取**，
// 因為 database.GetDBTX 會在 ctx 帶有交易時回那個交易、否則回 pool。把 querier 存成
// 欄位 ＝ 永遠拿 pool ＝ usecase 用 TxManager.WithTx 包起來的多段寫入各自 auto-commit，
// 而症狀只在併發或中途失敗時才出現（§4.2.2「不自己 Begin/Commit」）。
type baseRepo struct {
	pool *pgxpool.Pool
}

func (r baseRepo) q(ctx context.Context) *gen.Queries {
	return gen.New(database.GetDBTX(ctx, r.pool))
}

// biz 把業務碼包成 DomainError。訊息一律取自 bizcode.Message，不在此重打字串——
// 重打的那一份不會隨 CodeMeta 的 .md 更新，而沒有任何 gate 比對它們。
func biz(code int32) error {
	return pkgerrors.NewBusiness(code, bizcode.Message(code))
}

// notFound 把 pgx.ErrNoRows 換成指定的業務碼，其餘錯誤原樣包裝往上。
//
// 為什麼「查無列」與「越權」共用同一個回傳：§8.1.4 要求 Tier A 越權回 404 且與
// 「不存在」不可區分——授權述詞寫在 SQL 的 WHERE 裡，兩者在這一層本來就長得一樣。
func notFound(err error, code int32) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return biz(code)
	}
	return err
}

// nilIfBlank 把空字串折成 NULL。
//
// 這不是美觀問題：多個 partial unique 與 CHECK（chk_invadmaudit_reason、
// uq_invadmaudit_client_request）對「空字串」與「NULL」的判定相反，而 Go 的零值是前者。
func nilIfBlank(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

// isUniqueViolation 回報這是不是 PostgreSQL 的 23505（unique_violation）。
//
// 🔴 它只用在**無法改寫成 ON CONFLICT** 的那些語句（UPDATE 沒有 ON CONFLICT 子句）。
// 一旦它為真，這個交易在 PostgreSQL 端已經 aborted（25P02）⇒ 呼叫端只能 rollback，
// 不得再下任何一句 SQL，也不得「catch 之後繼續查」。
func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}
