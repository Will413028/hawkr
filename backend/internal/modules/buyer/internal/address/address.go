// Package address 提供地址與預設地址管理。
package address

import (
	"context"
	"errors"
	"github.com/jackc/pgx/v5"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/Will413028/hawkr/backend/internal/modules/buyer/internal/model"
	"github.com/Will413028/hawkr/backend/internal/modules/buyer/internal/stint"
	"github.com/Will413028/hawkr/backend/internal/modules/buyer/internal/store"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
	"github.com/Will413028/hawkr/backend/internal/platform/db"
	"github.com/Will413028/hawkr/backend/internal/platform/pagination"
	"github.com/Will413028/hawkr/backend/internal/platform/phone"
	"github.com/Will413028/hawkr/backend/internal/platform/twaddr"
)

// Deps 外部依賴；clock 由組裝點注入。
type Deps struct {
	DB  *db.DB
	Now func() time.Time
}

// Service 此子領域的 use case 容器。
type Service struct {
	deps Deps
}

// New 建立服務，外部依賴由組裝點提供。
func New(deps Deps) *Service { return &Service{deps: deps} }

// Validate 驗證提供的欄位並正規化電話。
func Validate(input *model.AddressInput) error {
	input.ReceiverName = strings.TrimSpace(input.ReceiverName)
	count := 0
	for _, character := range input.ReceiverName {
		if character <= 127 {
			count++
		} else {
			count += 2
		}
	}
	if count < 4 || count > 30 {
		return apperr.InvalidInputOn([]string{"ReceiverName"}, "收件人姓名長度必須為 4 到 30")
	}
	input.ReceiverPhone = phone.NormalizeToLocal(input.ReceiverPhone)
	if !phone.Valid(input.ReceiverPhone) {
		return apperr.InvalidInputOn([]string{"ReceiverPhone"}, "收件人電話不合法")
	}
	if _, ok := twaddr.TownByCode(input.TownCode); !ok {
		return apperr.InvalidInputOn([]string{"TownCode"}, "鄉鎮市區代碼不合法")
	}
	input.AddressDetail = strings.TrimSpace(input.AddressDetail)
	if n := utf8.RuneCountInString(input.AddressDetail); n < 1 || n > 255 {
		return apperr.InvalidInputOn([]string{"AddressDetail"}, "地址必填且不可超過 255 字")
	}
	return nil
}

func toAddress(addressRow store.BuyerShippingAddress) model.Address {
	return model.Address{
		AddressID:     addressRow.AddressID,
		BuyerID:       addressRow.BuyerID,
		Version:       addressRow.Version,
		ReceiverName:  addressRow.ReceiverName,
		ReceiverPhone: addressRow.ReceiverPhone,
		TownCode:      addressRow.TownCode,
		AddressDetail: addressRow.AddressDetail,
		IsDefault:     addressRow.IsDefault,
		CreatedAt:     addressRow.CreatedAt,
		UpdatedAt:     addressRow.UpdatedAt,
	}
}

// List 列出買家的全部地址，預設地址優先。
func (service *Service) List(ctx context.Context, buyerID int64) ([]model.Address, error) {
	if err := model.Positive(buyerID); err != nil {
		return nil, err
	}
	rows, err := store.New(service.deps.DB.Conn(ctx)).AllAddresses(ctx, buyerID)
	if err != nil {
		return nil, err
	}
	result := make([]model.Address, 0, len(rows))
	for _, addressRow := range rows {
		result = append(result, toAddress(addressRow))
	}
	return result, err
}

// Page 依預設標記、更新時間及地址 ID 取一頁。
func (service *Service) Page(ctx context.Context, input model.AddressListInput) (model.AddressPage, error) {
	if err := model.Positive(input.BuyerID); err != nil {
		return model.AddressPage{}, err
	}
	pageSize, err := pagination.PageSize(input.PageSize, model.MaxAddresses, 100)
	if err != nil {
		return model.AddressPage{}, err
	}
	params := store.AddressesParams{BuyerID: input.BuyerID, PageLimit: pageSize + 1}
	if input.After != nil {
		params.HasCursor = true
		params.CursorDefault = input.After.Default
		params.CursorTime = input.After.Time
		params.CursorID = input.After.ID
	}
	rows, err := store.New(service.deps.DB.Conn(ctx)).Addresses(ctx, params)
	if err != nil {
		return model.AddressPage{}, err
	}
	page := model.AddressPage{Items: make([]model.Address, 0, min(len(rows), int(pageSize)))}
	for _, row := range rows[:min(len(rows), int(pageSize))] {
		page.Items = append(page.Items, toAddress(row))
	}
	if len(rows) > int(pageSize) {
		last := page.Items[len(page.Items)-1]
		page.Next = &model.AddressCursor{Default: last.IsDefault, Time: last.UpdatedAt, ID: last.AddressID}
	}
	return page, nil
}

// Get 讀取買家擁有的地址；不存在或不屬於該買家均回 NotFound。
func (service *Service) Get(ctx context.Context, buyerID, addressID int64) (model.Address, error) {
	if err := model.Positive(buyerID, addressID); err != nil {
		return model.Address{}, err
	}
	addressRow, err := store.New(service.deps.DB.Conn(ctx)).Address(ctx, store.AddressParams{BuyerID: buyerID, AddressID: addressID})
	if err != nil {
		return model.Address{}, addressError(err)
	}
	return toAddress(addressRow), nil
}

// Add 新增地址，第一筆自動成為預設。
func (service *Service) Add(ctx context.Context, input model.AddressInput) (model.Address, error) {
	if err := model.Positive(input.BuyerID); err != nil {
		return model.Address{}, err
	}
	if err := Validate(&input); err != nil {
		return model.Address{}, err
	}
	var result store.BuyerShippingAddress
	err := service.deps.DB.InTx(ctx, func(ctx context.Context) error {
		queries := store.New(service.deps.DB.Conn(ctx))
		if _, err := stint.Require(ctx, queries, input.BuyerID, false); err != nil {
			return err
		}
		rows, err := queries.AllAddresses(ctx, input.BuyerID)
		if err != nil {
			return err
		}
		// 已鎖住買家，筆數不會在檢查與寫入之間改變。上限是要等狀態被修正（先刪掉一筆）的前提條件，不是輸入錯誤
		// （〈API 設計規範（Connect）〉§八）。
		if len(rows) >= model.MaxAddresses {
			return apperr.FailedPrecondition("收貨地址最多 %d 筆，請先刪除不用的地址", model.MaxAddresses)
		}
		def := input.IsDefault || len(rows) == 0
		if def {
			if err := queries.ClearDefault(ctx, store.ClearDefaultParams{BuyerID: input.BuyerID, UpdatedAt: service.deps.Now().UTC()}); err != nil {
				return err
			}
		}
		result, err = queries.AddAddress(ctx, store.AddAddressParams{
			BuyerID:       input.BuyerID,
			ReceiverName:  input.ReceiverName,
			ReceiverPhone: input.ReceiverPhone,
			TownCode:      input.TownCode,
			AddressDetail: input.AddressDetail,
			IsDefault:     def,
			CreatedAt:     service.deps.Now().UTC(),
		})
		return err
	})
	if err != nil {
		return model.Address{}, addressError(err)
	}
	return toAddress(result), nil
}

// Update 修改地址；原預設不因 IsDefault=false 而清空。
func (service *Service) Update(ctx context.Context, input model.AddressInput) (model.Address, error) {
	if err := model.Positive(input.BuyerID, input.AddressID); err != nil {
		return model.Address{}, err
	}
	if input.ExpectedVersion <= 0 {
		return model.Address{}, apperr.InvalidInputOn([]string{"ExpectedVersion"}, "必須提供讀取時的版本")
	}
	if err := Validate(&input); err != nil {
		return model.Address{}, err
	}
	var result store.BuyerShippingAddress
	err := service.deps.DB.InTx(ctx, func(ctx context.Context) error {
		queries := store.New(service.deps.DB.Conn(ctx))
		if _, err := stint.Require(ctx, queries, input.BuyerID, false); err != nil {
			return err
		}
		old, err := queries.Address(ctx, store.AddressParams{BuyerID: input.BuyerID, AddressID: input.AddressID})
		if err != nil {
			return addressError(err)
		}
		if old.Version != input.ExpectedVersion {
			return apperr.VersionConflict("地址已被修改")
		}
		def := input.IsDefault || old.IsDefault
		changed := old.ReceiverName != input.ReceiverName ||
			old.ReceiverPhone != input.ReceiverPhone ||
			old.TownCode != input.TownCode ||
			old.AddressDetail != input.AddressDetail ||
			old.IsDefault != def
		if !changed {
			result = old
			return nil
		}
		if def && !old.IsDefault {
			if err := queries.ClearDefault(ctx, store.ClearDefaultParams{BuyerID: input.BuyerID, AddressID: input.AddressID, UpdatedAt: service.deps.Now().UTC()}); err != nil {
				return err
			}
		}
		result, err = queries.UpdateAddress(ctx, store.UpdateAddressParams{
			BuyerID:       input.BuyerID,
			AddressID:     input.AddressID,
			ReceiverName:  input.ReceiverName,
			ReceiverPhone: input.ReceiverPhone,
			TownCode:      input.TownCode,
			AddressDetail: input.AddressDetail,
			IsDefault:     def,
			UpdatedAt:     service.deps.Now().UTC(),
			Version:       input.ExpectedVersion,
		})
		if errors.Is(err, pgx.ErrNoRows) {
			return apperr.VersionConflict("地址已被修改")
		}
		return err
	})
	if err != nil {
		return model.Address{}, addressError(err)
	}
	return toAddress(result), nil
}

// Delete 刪除地址；刪除預設時以最近更新的剩餘地址遞補。
func (service *Service) Delete(ctx context.Context, buyerID, addressID, expectedVersion int64) error {
	if err := model.Positive(buyerID, addressID); err != nil {
		return err
	}
	return service.deps.DB.InTx(ctx, func(ctx context.Context) error {
		queries := store.New(service.deps.DB.Conn(ctx))
		if _, err := stint.Require(ctx, queries, buyerID, false); err != nil {
			return err
		}
		_, err := queries.Address(ctx, store.AddressParams{BuyerID: buyerID, AddressID: addressID})
		if err != nil {
			return addressError(err)
		}
		if _, err := queries.DeleteAddress(ctx, store.DeleteAddressParams{BuyerID: buyerID, AddressID: addressID, CheckVersion: expectedVersion != 0, ExpectedVersion: expectedVersion}); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return apperr.VersionConflict("地址已被修改")
			}
			return err
		}
		return queries.PromoteDefault(ctx, store.PromoteDefaultParams{BuyerID: buyerID, UpdatedAt: service.deps.Now().UTC()})
	})
}

func addressError(err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return apperr.NotFound("找不到地址")
	}
	return err
}
