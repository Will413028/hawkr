package hold

import (
	"context"
	"errors"
	"testing"
	"time"
	"uuid"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/Will413028/hawkr/backend/internal/modules/media/internal/model"
	"github.com/Will413028/hawkr/backend/internal/modules/media/internal/store"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
)

func TestClassifyHoldInsertMapsForeignKeyViolationToRetryableContention(t *testing.T) {
	fk := &pgconn.PgError{Code: foreignKeyViolation, ConstraintName: holdGateFKey}
	got := classifyHoldInsert(fk)
	if !errors.Is(got, model.ErrHoldContended) {
		t.Fatalf("23503 = %v，預期 ErrHoldContended", got)
	}
	if errors.Is(got, model.ErrAssetGone) {
		t.Fatal("23503 分不出閘門是被切成 DELETE 還是被撤掉，不得回 ErrAssetGone")
	}
	if k := apperr.KindOf(got); k != apperr.KindUnavailable {
		t.Fatalf("分類 = %v，預期 Unavailable", k)
	}
	if !IsContention(got) {
		t.Fatal("原始的 PgError 要保留在錯誤鏈上")
	}
}

func TestClassifyHoldInsertLeavesOtherErrorsAlone(t *testing.T) {
	if classifyHoldInsert(nil) != nil {
		t.Fatal("nil 要維持 nil")
	}
	other := &pgconn.PgError{Code: "23505"}
	if got := classifyHoldInsert(other); !errors.Is(got, other) || errors.Is(got, model.ErrHoldContended) {
		t.Fatalf("其他錯誤被改寫：%v", got)
	}
}

func TestOtherForeignKeyViolationsAreNotContention(t *testing.T) {
	other := &pgconn.PgError{Code: foreignKeyViolation, ConstraintName: "asset_gates_asset_fkey"}
	if got := classifyHoldInsert(other); errors.Is(got, model.ErrHoldContended) {
		t.Fatalf("其他外鍵違反被當成保全競爭：%v", got)
	}
	if IsContention(other) {
		t.Fatal("其他外鍵違反不是競爭")
	}
	for _, name := range []string{holdGateFKey, holdReleaseFKey} {
		if !IsContention(&pgconn.PgError{Code: foreignKeyViolation, ConstraintName: name}) {
			t.Fatalf("%s 的外鍵違反是競爭", name)
		}
	}
	if !IsContention(&pgconn.PgError{Code: lockNotAvailable}) {
		t.Fatal("55P03 是競爭")
	}
}

type failingExec struct{ err error }

func (f failingExec) Exec(context.Context, string, ...any) (pgconn.CommandTag, error) {
	return pgconn.CommandTag{}, f.err
}

func (failingExec) Query(context.Context, string, ...any) (pgx.Rows, error) { return nil, nil }

func (failingExec) QueryRow(context.Context, string, ...any) pgx.Row { return nil }

func TestReleaseMapsAConcurrentlyDeletedHoldToContention(t *testing.T) {
	fk := &pgconn.PgError{Code: foreignKeyViolation, ConstraintName: holdReleaseFKey}
	err := Release(t.Context(), store.New(failingExec{err: fk}), "src", uuid.New(), time.Now())
	if !errors.Is(err, model.ErrHoldContended) || apperr.KindOf(err) != apperr.KindUnavailable {
		t.Fatalf("Release 撞上被刪除的保全列 = %v，預期 Unavailable 的 ErrHoldContended", err)
	}
	other := errors.New("boom")
	if err := Release(t.Context(), store.New(failingExec{err: other}), "src", uuid.New(), time.Now()); !errors.Is(err, other) || errors.Is(err, model.ErrHoldContended) {
		t.Fatalf("其他錯誤被改寫：%v", err)
	}
}
