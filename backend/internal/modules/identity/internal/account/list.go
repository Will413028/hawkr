package account

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/Will413028/hawkr/backend/internal/modules/identity/internal/model"
	"github.com/Will413028/hawkr/backend/internal/modules/identity/internal/store"
	"github.com/Will413028/hawkr/backend/internal/platform/pagination"
)

// 帳號清單每頁的筆數（Google AIP-158）。
const (
	defaultAccountPageSize = 10
	maxAccountPageSize     = 100
)

// ListAccounts 列出存活的帳號，新到舊（後台的會員頁）。After 為 nil 時從第一頁開始。
func (m *Service) ListAccounts(ctx context.Context, in model.ListAccountsInput) (model.AccountPage, error) {
	size, err := pagination.PageSize(in.Size, defaultAccountPageSize, maxAccountPageSize)
	if err != nil {
		return model.AccountPage{}, err
	}
	in.Size = size
	if in.Filter.Role != "" && in.Filter.Role != model.RoleSeller {
		return model.AccountPage{}, fmt.Errorf("identity：未知的角色 %q", in.Filter.Role)
	}
	pattern, accountID := "", int64(0)
	if q := strings.TrimSpace(in.Filter.Query); q != "" {
		pattern = "%" + escapeLike(q) + "%"
		// 不是數字（或超出範圍）時是 0：ID 從 1 開始，不會命中。
		accountID, _ = strconv.ParseInt(q, 10, 64)
	}
	email := strings.ToLower(strings.TrimSpace(in.Filter.Email))
	params := store.ListAccountsParams{Role: string(in.Filter.Role), Pattern: pattern, AccountID: accountID, Email: email, MaxRows: in.Size + 1}
	if in.After != nil {
		params.HasCursor, params.AfterCreated, params.AfterID = true, in.After.CreatedAt, in.After.ID
	}
	q := m.q(ctx)
	rows, err := q.ListAccounts(ctx, params)
	if err != nil {
		return model.AccountPage{}, fmt.Errorf("identity：列出帳號：%w", err)
	}
	total, err := q.CountAccounts(ctx, store.CountAccountsParams{Role: string(in.Filter.Role), Pattern: pattern, AccountID: accountID, Email: email})
	if err != nil {
		return model.AccountPage{}, fmt.Errorf("identity：計算帳號數：%w", err)
	}
	page := model.AccountPage{Total: total}
	if len(rows) > int(in.Size) {
		rows = rows[:in.Size]
		last := rows[len(rows)-1]
		page.Next = &model.AccountCursor{CreatedAt: last.CreatedAt, ID: last.ID}
	}
	ids := make([]int64, len(rows))
	for i, r := range rows {
		ids[i] = r.ID
	}
	accounts, err := m.Accounts(ctx, ids)
	if err != nil {
		return model.AccountPage{}, err
	}
	byID := make(map[int64]model.Account, len(accounts))
	for _, a := range accounts {
		byID[a.ID] = a
	}
	for _, r := range rows {
		page.Accounts = append(page.Accounts, byID[r.ID])
	}
	return page, nil
}

// escapeLike 讓使用者輸入的 %、_ 與 \ 當成字元本身（LIKE 預設的跳脫字元是 \）。
func escapeLike(s string) string {
	return strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(s)
}
