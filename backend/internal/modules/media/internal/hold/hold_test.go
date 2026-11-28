package hold

import (
	"errors"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"

	"github.com/Will413028/hawkr/backend/internal/modules/media/internal/model"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
)

func TestClassifyHoldInsertMapsForeignKeyViolationToRetryableContention(t *testing.T) {
	fk := &pgconn.PgError{Code: foreignKeyViolation}
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
