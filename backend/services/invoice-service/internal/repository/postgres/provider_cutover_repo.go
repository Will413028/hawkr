package postgres

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Will413028/hawkr/invoice-service/internal/domain"
	"github.com/Will413028/hawkr/invoice-service/internal/repository/postgres/gen"
)

// ProviderCutoverRepository 是 invoice_provider_cutovers（§5.1.15）的唯一存取面。
//
// 🔴 寫入的兩支（CloseOpen／Create）必須與 invoice_provider_accounts 的那一句 CAS
// **同一交易**：分兩個交易寫，歷史與現值會不一致，而端點 46 是唯一會讀到本表的地方
// ——沒有任何東西會發現。
type ProviderCutoverRepository struct {
	baseRepo
}

// NewProviderCutoverRepository 建立 repository。
func NewProviderCutoverRepository(pool *pgxpool.Pool) *ProviderCutoverRepository {
	return &ProviderCutoverRepository{baseRepo: baseRepo{pool: pool}}
}

var _ domain.ProviderCutoverRepository = (*ProviderCutoverRepository)(nil)

// CloseOpen 關掉目前有效的那一列，回被改到的列數。
//
// ⚠️ rows=0 **不是守衛不成立**：第一次設定切點時本來就沒有前一列。呼叫端不得把它
// 當成 CAS 失敗（那會讓每個賣家的第一次啟用都被回絕）。
func (r *ProviderCutoverRepository) CloseOpen(ctx context.Context, providerAccountID int64, now time.Time) (int64, error) {
	return r.q(ctx).CloseOpenProviderCutover(ctx, gen.CloseOpenProviderCutoverParams{
		ProviderAccountID: providerAccountID,
		Now:               tp(now),
	})
}

// Create 新增一列切點歷史。
func (r *ProviderCutoverRepository) Create(ctx context.Context, in domain.NewProviderCutover) (domain.ProviderCutover, error) {
	row, err := r.q(ctx).InsertProviderCutover(ctx, gen.InsertProviderCutoverParams{
		ProviderAccountID:  in.ProviderAccountID,
		IssuerID:           in.IssuerID,
		AccountVersion:     in.AccountVersion,
		CutoverAt:          ts(in.CutoverAt),
		CreatedByAccountID: in.CreatedByAccountID,
		Reason:             nilIfBlank(in.Reason),
		Now:                ts(in.Now),
	})
	if err != nil {
		return domain.ProviderCutover{}, err
	}
	return toProviderCutover(row), nil
}

// ListForAdmin 回一個帳戶的切點歷史（新到舊）。
func (r *ProviderCutoverRepository) ListForAdmin(ctx context.Context, providerAccountID int64, page domain.Page) ([]domain.ProviderCutover, error) {
	rows, err := r.q(ctx).ListProviderCutoversForAdmin(ctx, gen.ListProviderCutoversForAdminParams{
		ProviderAccountID: providerAccountID,
		RowLimit:          page.Limit,
		RowOffset:         page.Offset,
	})
	if err != nil {
		return nil, err
	}
	out := make([]domain.ProviderCutover, 0, len(rows))
	for _, row := range rows {
		out = append(out, toProviderCutover(row))
	}
	return out, nil
}

// CountForAdmin 回總筆數（端點 46 的 total）。
func (r *ProviderCutoverRepository) CountForAdmin(ctx context.Context, providerAccountID int64) (int64, error) {
	return r.q(ctx).CountProviderCutoversForAdmin(ctx, providerAccountID)
}

// toProviderCutover 把生成列映成 domain 型別。
func toProviderCutover(row gen.InvoiceProviderCutover) domain.ProviderCutover {
	return domain.ProviderCutover{
		ID:                 row.ID,
		ProviderAccountID:  row.ProviderAccountID,
		IssuerID:           row.IssuerID,
		AccountVersion:     row.AccountVersion,
		CutoverAt:          row.CutoverAt.Time,
		DeactivatedAt:      row.DeactivatedAt,
		Mode:               row.Mode,
		CreatedByAccountID: row.CreatedByAccountID,
		Reason:             row.Reason,
		CreatedAt:          row.CreatedAt.Time,
	}
}
