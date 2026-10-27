package usecase

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/Will413028/hawkr/ai-service/internal/cache"
	"github.com/Will413028/hawkr/ai-service/internal/repository/postgres/gen"
	"github.com/Will413028/hawkr/ai-service/internal/usecase/dto"
	pkgerrors "github.com/Will413028/hawkr/go-pkg/errors"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
)

// #3519 slice B Task 4 fix round 1: quota_usecase.go／style_usecase.go 的幾個
// deliberate 業務錯誤（"style not found"／"cannot delete default style"／
// "sort order out of range: ..."）改成 pkgerrors.NewBusiness(400, ...)，讓 grpc
// handler 的 domainMessage() 判為已分類、原樣透傳，不再被「未歸類 fallback」的
// genericFailureMessage 吃掉。本檔直接在 usecase 層驗證這個分類是否正確，並確認
// 真正的 DB 錯誤（非本次要保留的業務訊息）仍然維持未歸類、不會被誤判成業務錯誤。
//
// 用 gen.DBTX（sqlc 生成的介面：Exec／Query／QueryRow）造假，不需要真的
// Postgres——僅涵蓋 StyleUseCase.Update／Delete 的「非交易」分支：
// Update() 在 req.IsDefault==true 時走 uc.txManager.WithTx，其閉包內部用
// gen.New(database.GetDBTX(txCtx, uc.pool)) 重新從 uc.pool（型別是具體的
// *pgxpool.Pool，不是介面）取得 Queries，會繞過這裡注入的假 uc.q，因此
// setsIsDefaultTrue 分支的「style not found」（沿用同一個轉換、同一個判定）
// 仍然只能靠 dbtest／docker 覆蓋，見套件內其他測試檔的既有先例
// （dbtest_test.go，`//go:build integration`）。

// fakeStyleRow 是 GetAiStyleByID 的假 pgx.Row：dest 的型別與順序對應
// gen.(*Queries).GetAiStyleByID 的 Scan 呼叫（style.sql.go），改動該生成檔的
// SELECT 欄位順序時這裡也要跟著改。
type fakeStyleRow struct {
	err   error
	style gen.AiStyle
}

func (r fakeStyleRow) Scan(dest ...any) error {
	if r.err != nil {
		return r.err
	}
	*dest[0].(*int64) = r.style.ID
	*dest[1].(*string) = r.style.Name
	*dest[2].(*string) = r.style.Label
	*dest[3].(*string) = r.style.Prompt
	*dest[4].(*string) = r.style.ThumbnailKey
	*dest[5].(*int32) = r.style.SortOrder
	*dest[6].(*bool) = r.style.IsDefault
	*dest[7].(*bool) = r.style.Enabled
	*dest[8].(*string) = r.style.Category
	*dest[9].(*pgtype.Timestamptz) = r.style.CreatedAt
	*dest[10].(*pgtype.Timestamptz) = r.style.UpdatedAt
	return nil
}

// fakeStyleDBTX implements gen.DBTX：Exec 給單一腳本化結果（affected rows 或
// error），QueryRow 回傳腳本化的 fakeStyleRow，Query 本檔用不到。
type fakeStyleDBTX struct {
	queryRow fakeStyleRow
	execN    int64
	execErr  error
}

func (f *fakeStyleDBTX) Exec(_ context.Context, _ string, _ ...any) (pgconn.CommandTag, error) {
	if f.execErr != nil {
		return pgconn.CommandTag{}, f.execErr
	}
	return pgconn.NewCommandTag(fmt.Sprintf("UPDATE %d", f.execN)), nil
}

func (f *fakeStyleDBTX) Query(_ context.Context, _ string, _ ...any) (pgx.Rows, error) {
	return nil, errors.New("fakeStyleDBTX: Query not implemented")
}

func (f *fakeStyleDBTX) QueryRow(_ context.Context, _ string, _ ...any) pgx.Row {
	return f.queryRow
}

// noopTxManager 只滿足 StyleUseCase.txManager 的欄位型別；本檔測試的都是
// Update()／Delete() 不進交易的分支，不會被呼叫到。
type noopTxManager struct{}

func (noopTxManager) WithTx(ctx context.Context, fn func(ctx context.Context) error) error {
	return fn(ctx)
}

func newTestStyleUseCase(db gen.DBTX) *StyleUseCase {
	return &StyleUseCase{
		q:         gen.New(db),
		txManager: noopTxManager{},
		cache:     cache.New(time.Minute),
		genID:     func() int64 { return 1 },
	}
}

// assertDomainBusinessError 斷言 err 是一個 DomainCoder，且 Code／Message 與
// NewBusiness(400, wantMsg) 逐字元一致（含 Error() 沒有被串上任何 Cause）。
func assertDomainBusinessError(t *testing.T, err error, wantCode int, wantMsg string) {
	t.Helper()
	if err == nil {
		t.Fatal("expected an error, got nil")
	}
	var dc pkgerrors.DomainCoder
	if !errors.As(err, &dc) {
		t.Fatalf("expected a DomainCoder (classified business error), got %T: %v", err, err)
	}
	if dc.DomainCode() != wantCode {
		t.Errorf("DomainCode() = %d, want %d", dc.DomainCode(), wantCode)
	}
	if dc.DomainMessage() != wantMsg {
		t.Errorf("DomainMessage() = %q, want %q", dc.DomainMessage(), wantMsg)
	}
	if err.Error() != wantMsg {
		t.Errorf("Error() = %q, want %q (NewBusiness must carry no Cause)", err.Error(), wantMsg)
	}
}

// assertUnclassifiedError 斷言 err 不是 DomainCoder（不會被 grpc handler 的
// domainMessage()／GetCode 誤判成業務錯誤），且原文包含 sentinel——證明真正的 DB
// 失敗仍然會落到「未歸類 fallback」（genericFailureMessage + log），不會被本輪
// fix round 1 的分類意外吃掉。
func assertUnclassifiedError(t *testing.T, err error, sentinel string) {
	t.Helper()
	if err == nil {
		t.Fatal("expected an error, got nil")
	}
	var dc pkgerrors.DomainCoder
	if errors.As(err, &dc) {
		t.Fatalf("expected a plain (non-DomainCoder) error for a real DB failure, got classified: %v", err)
	}
	if !strings.Contains(err.Error(), sentinel) {
		t.Fatalf("expected error to contain sentinel %q, got %q", sentinel, err.Error())
	}
}

func TestStyleUseCase_Delete_LookupFailure_IsClassifiedStyleNotFound(t *testing.T) {
	uc := newTestStyleUseCase(&fakeStyleDBTX{queryRow: fakeStyleRow{err: errors.New("sentinel-lookup-9f3a")}})
	err := uc.Delete(context.Background(), 1)
	assertDomainBusinessError(t, err, 400, "style not found")
}

func TestStyleUseCase_Delete_DefaultStyle_IsClassifiedCannotDelete(t *testing.T) {
	uc := newTestStyleUseCase(&fakeStyleDBTX{queryRow: fakeStyleRow{style: gen.AiStyle{ID: 1, IsDefault: true}}})
	err := uc.Delete(context.Background(), 1)
	assertDomainBusinessError(t, err, 400, "cannot delete default style")
}

// 真正的 DELETE 失敗（GetAiStyleByID 成功、DeleteAiStyle 本身出錯）必須維持
// 未歸類——不能因為本輪修法而被誤判成 "style not found"／"cannot delete
// default style" 之類的業務錯誤。
func TestStyleUseCase_Delete_RealDeleteFailure_StaysUnclassified(t *testing.T) {
	uc := newTestStyleUseCase(&fakeStyleDBTX{
		queryRow: fakeStyleRow{style: gen.AiStyle{ID: 1, IsDefault: false}},
		execErr:  errors.New("sentinel-delete-db-failure-9f3a"),
	})
	err := uc.Delete(context.Background(), 1)
	assertUnclassifiedError(t, err, "sentinel-delete-db-failure-9f3a")
}

func TestStyleUseCase_Update_ZeroRowsAffected_IsClassifiedStyleNotFound(t *testing.T) {
	uc := newTestStyleUseCase(&fakeStyleDBTX{execN: 0})
	label := "new label"
	err := uc.Update(context.Background(), 1, dto.UpdateStyleRequest{Label: &label})
	assertDomainBusinessError(t, err, 400, "style not found")
}

// 真正的 UPDATE 失敗（Exec 本身出錯，不是 affected==0）必須維持未歸類。
func TestStyleUseCase_Update_RealUpdateFailure_StaysUnclassified(t *testing.T) {
	uc := newTestStyleUseCase(&fakeStyleDBTX{execErr: errors.New("sentinel-update-db-failure-4c2a")})
	label := "new label"
	err := uc.Update(context.Background(), 1, dto.UpdateStyleRequest{Label: &label})
	assertUnclassifiedError(t, err, "sentinel-update-db-failure-4c2a")
}

// SortOrder 溢位在碰 DB 之前就短路，fakeStyleDBTX 給零值都不會被用到。
func TestStyleUseCase_Update_SortOrderOutOfRange_IsClassifiedBusinessError(t *testing.T) {
	uc := newTestStyleUseCase(&fakeStyleDBTX{})
	sortOrder := "99999999999" // > math.MaxInt32
	err := uc.Update(context.Background(), 1, dto.UpdateStyleRequest{SortOrder: &sortOrder})
	assertDomainBusinessError(t, err, 400, "sort order out of range: 99999999999")
}
