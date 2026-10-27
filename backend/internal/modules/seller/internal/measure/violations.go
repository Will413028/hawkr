package measure

import (
	"context"
	"errors"
	"uuid"

	"github.com/jackc/pgx/v5"

	"github.com/Will413028/hawkr/backend/internal/modules/seller/internal/model"
	"github.com/Will413028/hawkr/backend/internal/modules/seller/internal/store"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
)

// Violation 讀取一筆認定（admin，只讀）；不存在回 NotFound。
func (s *Service) Violation(ctx context.Context, violationID uuid.UUID) (model.Violation, error) {
	q := store.New(s.d.DB.Conn(ctx))
	row, err := q.ViolationByID(ctx, violationID)
	if errors.Is(err, pgx.ErrNoRows) {
		return model.Violation{}, apperr.NotFound("找不到這筆認定")
	}
	if err != nil {
		return model.Violation{}, err
	}
	out, err := toViolations(ctx, q, []store.SellerViolation{row})
	if err != nil {
		return model.Violation{}, err
	}
	return out[0], nil
}

// Violations 是 admin 的認定列表，新到舊，可依店家、帳號、狀態篩選。
func (s *Service) Violations(ctx context.Context, in model.ViolationListInput) (model.ViolationPage, error) {
	size, err := pageSize(in.PageSize)
	if err != nil {
		return model.ViolationPage{}, err
	}
	if in.StoreID < 0 || in.AccountID < 0 {
		return model.ViolationPage{}, apperr.InvalidInput("ID 必須為正數")
	}
	params := store.AdminViolationsParams{PageSize: size + 1}
	if in.StoreID > 0 {
		params.StoreID = &in.StoreID
	}
	if in.AccountID > 0 {
		params.AccountID = &in.AccountID
	}
	if in.Status != nil {
		if err := model.OneOf(*in.Status, model.ViolationStatusValues(), "Status", "認定狀態不合法"); err != nil {
			return model.ViolationPage{}, err
		}
		params.Status = (*string)(in.Status)
	}
	if in.After != nil {
		params.AfterCreatedAt, params.AfterID = &in.After.CreatedAt, &in.After.ID
	}
	q := store.New(s.d.DB.Conn(ctx))
	rows, err := q.AdminViolations(ctx, params)
	if err != nil {
		return model.ViolationPage{}, err
	}
	items, err := toViolations(ctx, q, rows)
	if err != nil {
		return model.ViolationPage{}, err
	}
	page := model.ViolationPage{Items: items}
	if len(page.Items) > int(size) {
		page.Items = page.Items[:size]
		last := page.Items[size-1]
		page.Next = &model.ViolationCursor{CreatedAt: last.CreatedAt, ID: last.ID}
	}
	return page, nil
}
