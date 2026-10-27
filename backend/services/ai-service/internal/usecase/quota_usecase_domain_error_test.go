package usecase

import (
	"context"
	"errors"
	"testing"

	"github.com/Will413028/hawkr/ai-service/internal/repository/postgres/gen"
	"github.com/Will413028/hawkr/ai-service/internal/usecase/dto"
	"github.com/Will413028/hawkr/go-pkg/identity"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// #3519 slice B Task 4 fix round 1：quota_usecase.go 的 weeklyLimit 驗證訊息
// 改成 pkgerrors.NewBusiness(400, ...)。fakeStyleDBTX／assertDomainBusinessError／
// assertUnclassifiedError 定義在 style_usecase_domain_error_test.go（同一個
// package，不重複定義）。

// fakeQuotaDBTX implements gen.DBTX；UpdateQuota／DeleteQuota 都只用 Exec
// （sqlc :exec，不讀回傳列），QueryRow 本檔用不到。
type fakeQuotaDBTX struct {
	execErr error
}

func (f *fakeQuotaDBTX) Exec(_ context.Context, _ string, _ ...any) (pgconn.CommandTag, error) {
	if f.execErr != nil {
		return pgconn.CommandTag{}, f.execErr
	}
	return pgconn.NewCommandTag("UPDATE 1"), nil
}

func (f *fakeQuotaDBTX) Query(_ context.Context, _ string, _ ...any) (pgx.Rows, error) {
	return nil, errors.New("fakeQuotaDBTX: Query not implemented")
}

func (f *fakeQuotaDBTX) QueryRow(_ context.Context, _ string, _ ...any) pgx.Row {
	return nil
}

func TestQuotaUseCase_UpdateQuota_WeeklyLimitOutOfRange_IsClassifiedBusinessError(t *testing.T) {
	uc := &QuotaUseCase{q: gen.New(&fakeQuotaDBTX{})}
	err := uc.UpdateQuota(context.Background(), identity.StoreID(1), dto.UpdateQuotaRequest{WeeklyLimit: "9999"})
	assertDomainBusinessError(t, err, 400, "weeklyLimit 必須為 0~1000 的整數")
}

// 真正的 UpsertAiQuota 失敗必須維持未歸類，不能被本輪修法誤判成業務錯誤。
func TestQuotaUseCase_UpdateQuota_RealUpsertFailure_StaysUnclassified(t *testing.T) {
	uc := &QuotaUseCase{q: gen.New(&fakeQuotaDBTX{execErr: errors.New("sentinel-quota-upsert-failure-2e9d")})}
	err := uc.UpdateQuota(context.Background(), identity.StoreID(1), dto.UpdateQuotaRequest{WeeklyLimit: "50", Memo: "ok"})
	assertUnclassifiedError(t, err, "sentinel-quota-upsert-failure-2e9d")
}

// DeleteQuota 沒有本地驗證，err 只可能來自 DB——確認它維持未歸類。
func TestQuotaUseCase_DeleteQuota_RealDeleteFailure_StaysUnclassified(t *testing.T) {
	uc := &QuotaUseCase{q: gen.New(&fakeQuotaDBTX{execErr: errors.New("sentinel-quota-delete-failure-1a7f")})}
	err := uc.DeleteQuota(context.Background(), identity.StoreID(1))
	assertUnclassifiedError(t, err, "sentinel-quota-delete-failure-1a7f")
}
