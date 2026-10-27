// Package profile 提供買家 profile 與頭像操作。
package profile

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"log/slog"
	"math/big"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/Will413028/hawkr/backend/internal/modules/buyer/internal/mediaref"
	"github.com/Will413028/hawkr/backend/internal/modules/buyer/internal/model"
	"github.com/Will413028/hawkr/backend/internal/modules/buyer/internal/stint"
	"github.com/Will413028/hawkr/backend/internal/modules/buyer/internal/store"
	"github.com/Will413028/hawkr/backend/internal/modules/media"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
	"github.com/Will413028/hawkr/backend/internal/platform/db"
	"github.com/Will413028/hawkr/backend/internal/platform/jobs"
	"github.com/Will413028/hawkr/backend/internal/platform/phone"
)

// Deps 外部依賴；clock 由組裝點注入。
type Deps struct {
	DB             *db.DB
	Media          media.Media
	Jobs           jobs.Enqueuer
	Now            func() time.Time
	NicknameNumber func() int
	Logger         *slog.Logger
}

// Service 此子領域的 use case 容器。
type Service struct {
	deps Deps
}

// New 建立服務，外部依賴由組裝點提供。
func New(deps Deps) *Service {
	if deps.NicknameNumber == nil {
		deps.NicknameNumber = func() int {
			count, err := rand.Int(rand.Reader, big.NewInt(10000))
			if err != nil {
				panic(err)
			}
			return int(count.Int64())
		}
	}
	return &Service{deps: deps}
}

// Create 在建帳的 transaction 內建立帳號唯一的買家（ADR 2026-09-30-account-created-provisions-buyer）。買家不是
// identity 的角色，不授予任何角色。一個帳號一生只有一個買家：同一帳號再建立違反 buyers_account，回錯誤使建帳整筆
// 回滾，不重讀、不補建。
func (service *Service) Create(ctx context.Context, accountID int64, suggested string) error {
	if err := model.Positive(accountID); err != nil {
		return err
	}
	name := strings.TrimSpace(suggested)
	if name == "" {
		name = fmt.Sprintf("Hawkr 用戶%04d", service.deps.NicknameNumber())
	}
	if utf8.RuneCountInString(name) > 100 {
		name = string([]rune(name)[:100])
	}
	_, err := store.New(service.deps.DB.Conn(ctx)).CreateBuyer(ctx, store.CreateBuyerParams{
		AccountID:   accountID,
		DisplayName: &name,
		CreatedAt:   service.deps.Now().UTC(),
	})
	if err != nil {
		return model.Error(fmt.Errorf("buyer：建立買家：%w", err))
	}
	return nil
}

// Profile 讀取帳號目前存活的 stint。
func (service *Service) Profile(ctx context.Context, id int64) (model.Profile, error) {
	if err := model.Positive(id); err != nil {
		return model.Profile{}, err
	}
	buyerRow, err := store.New(service.deps.DB.Conn(ctx)).Current(ctx, id)
	if err != nil {
		return model.Profile{}, model.Error(err)
	}
	return stint.Profile(buyerRow), nil
}

// CurrentBuyerID 只讀取現任 stint 的業務鍵。
func (service *Service) CurrentBuyerID(ctx context.Context, accountID int64) (int64, error) {
	if err := model.Positive(accountID); err != nil {
		return 0, err
	}
	id, err := store.New(service.deps.DB.Conn(ctx)).CurrentBuyerID(ctx, accountID)
	if err != nil {
		return 0, model.Error(err)
	}
	return id, nil
}

// ProfileByBuyerID 讀取存活 profile；closed stint 回 NotFound。
func (service *Service) ProfileByBuyerID(ctx context.Context, id int64) (model.Profile, error) {
	if err := model.Positive(id); err != nil {
		return model.Profile{}, err
	}
	buyerRow, err := store.New(service.deps.DB.Conn(ctx)).ByID(ctx, id)
	if err == nil && (buyerRow.Status == string(model.Closed) || buyerRow.PiiClearedAt != nil) {
		return model.Profile{}, apperr.NotFound("買家已關閉")
	}
	if err != nil {
		return model.Profile{}, model.Error(err)
	}
	return stint.Profile(buyerRow), nil
}

// LockActive 在呼叫方的 transaction 內以共享鎖確認 stint 是營運中；沒有 transaction 是呼叫端的錯誤。
func (service *Service) LockActive(ctx context.Context, id int64) error {
	if err := model.Positive(id); err != nil {
		return err
	}
	if _, ok := db.TxFrom(ctx); !ok {
		return errors.New("buyer：LockActive 必須在 transaction 內呼叫")
	}
	return stint.RequireActiveShared(ctx, store.New(service.deps.DB.Conn(ctx)), id)
}

// AccountIDByBuyerID 只讀取 stint 的帳號鍵，包含已關閉的歷史 stint。
func (service *Service) AccountIDByBuyerID(ctx context.Context, id int64) (int64, error) {
	if err := model.Positive(id); err != nil {
		return 0, err
	}
	row, err := store.New(service.deps.DB.Conn(ctx)).ByID(ctx, id)
	if err != nil {
		return 0, model.Error(err)
	}
	return row.AccountID, nil
}

// BatchDisplay 批次讀取顯示資料，CLOSED stint 回 Deleted 且不回個資。
func (service *Service) BatchDisplay(ctx context.Context, ids []int64) ([]model.Display, error) {
	if err := model.BatchIDs(ids); err != nil {
		return nil, err
	}
	rows, err := store.New(service.deps.DB.Conn(ctx)).BatchDisplay(ctx, ids)
	if err != nil {
		return nil, err
	}
	result := make([]model.Display, 0, len(rows))
	for _, row := range rows {
		if row.Deleted {
			row.DisplayName = nil
			row.Avatar = nil
		}
		result = append(result, model.Display{
			BuyerID:     row.BuyerID,
			DisplayName: row.DisplayName,
			Avatar:      row.Avatar,
			Deleted:     row.Deleted,
		})
	}
	return result, err
}

// BatchMeta 批次讀取存活 profile；未找到的 ID 略過。
func (service *Service) BatchMeta(ctx context.Context, ids []int64) ([]model.Profile, error) {
	if err := model.BatchIDs(ids); err != nil {
		return nil, err
	}
	rows, err := store.New(service.deps.DB.Conn(ctx)).BatchProfiles(ctx, ids)
	if err != nil {
		return nil, err
	}
	result := make([]model.Profile, 0, len(rows))
	for _, row := range rows {
		result = append(result, stint.Profile(row))
	}
	return result, err
}

// ResolvePrincipals 回傳存活 buyer_id 到 account_id 的映射；未回傳的 ID 為 unresolved。
func (service *Service) ResolvePrincipals(ctx context.Context, ids []int64) (map[int64]int64, error) {
	if err := model.BatchIDs(ids); err != nil {
		return nil, err
	}
	rows, err := store.New(service.deps.DB.Conn(ctx)).Principals(ctx, ids)
	if err != nil {
		return nil, err
	}
	result := map[int64]int64{}
	for _, row := range rows {
		result[row.BuyerID] = row.AccountID
	}
	return result, err
}

// BuyerIDsByAccount 列出帳號所有目前與歷史 stint。
func (service *Service) BuyerIDsByAccount(ctx context.Context, id int64) ([]int64, error) {
	if err := model.Positive(id); err != nil {
		return nil, err
	}
	result, err := store.New(service.deps.DB.Conn(ctx)).BuyerIDs(ctx, id)
	if err != nil {
		return nil, err
	}
	return result, nil
}

// StatsForAdmin 計算目前存活買家數量。
func (service *Service) StatsForAdmin(ctx context.Context) (model.AdminStats, error) {
	count, err := store.New(service.deps.DB.Conn(ctx)).Stats(ctx)
	return model.AdminStats{Total: count}, err
}

// Validate 驗證提供的 profile 欄位，並正規化聯絡電話。
func Validate(input *model.UpdateProfileInput, now time.Time) error {
	if input.DisplayName != nil {
		name := strings.TrimSpace(*input.DisplayName)
		if name == "" || utf8.RuneCountInString(name) > 100 {
			return apperr.InvalidInputOn([]string{"DisplayName"}, "暱稱長度不合法")
		}
		input.DisplayName = &name
	}
	if input.Gender != nil && *input.Gender != "" && *input.Gender != model.Male && *input.Gender != model.Female && *input.Gender != model.Other {
		return apperr.InvalidInputOn([]string{"Gender"}, "性別不合法")
	}
	if input.Phone != nil {
		normalized := phone.NormalizeToLocal(*input.Phone)
		if normalized != "" && !phone.Valid(normalized) {
			return apperr.InvalidInputOn([]string{"Phone"}, "電話不合法")
		}
		input.Phone = &normalized
	}
	if input.BirthDate != nil && !input.BirthDate.IsZero() && (input.BirthDate.Year() < 1 || input.BirthDate.After(now)) {
		return apperr.InvalidInputOn([]string{"BirthDate"}, "生日不合法")
	}
	for _, value := range []*string{input.City, input.Job} {
		if value != nil && utf8.RuneCountInString(*value) > 100 {
			return apperr.InvalidInput("欄位過長")
		}
	}
	return nil
}

func nullableText(value string) *string {
	if value == "" {
		return nil
	}
	return &value
}

// UpdateProfile 在買家列鎖內合併提供的欄位；未提供的欄位保留原值。
// ExpectedVersion 必填；沒有任何欄位改變、也沒有換頭像時不寫入，版本不變。
// AvatarStagedToken 帶上傳完成的 staged token 時換頭像：先在 transaction 外預檢（版本與合併後的文字驗證）
// 並轉正圖片，再於同一個版本條件的 transaction 內寫入文字欄位與頭像、把被換掉的頭像記成退場並排入同步（mediaref）。
func (service *Service) UpdateProfile(ctx context.Context, input model.UpdateProfileInput) (model.Profile, error) {
	if err := model.Positive(input.BuyerID); err != nil {
		return model.Profile{}, err
	}
	if input.ExpectedVersion <= 0 {
		return model.Profile{}, apperr.InvalidInputOn([]string{"ExpectedVersion"}, "必須提供讀取時的版本")
	}
	var hasAvatar bool
	var avatarToken string
	if input.AvatarStagedToken != nil && *input.AvatarStagedToken != "" {
		if strings.TrimSpace(*input.AvatarStagedToken) == "" {
			return model.Profile{}, apperr.InvalidInputOn([]string{"AvatarStagedToken"}, "必須提供圖片")
		}
		hasAvatar = true
		avatarToken = strings.TrimSpace(*input.AvatarStagedToken)
	}

	var asset media.Asset
	if hasAvatar {
		if _, ok := db.TxFrom(ctx); ok {
			return model.Profile{}, apperr.FailedPrecondition("換圖必須在 transaction 外開始")
		}
		current, err := service.ProfileByBuyerID(ctx, input.BuyerID)
		if err != nil {
			return model.Profile{}, err
		}
		if current.Version != input.ExpectedVersion {
			return model.Profile{}, apperr.VersionConflict("買家資料已被修改")
		}
		if _, _, err := service.mergeChecked(current, input); err != nil {
			return model.Profile{}, err
		}
		ref := mediaref.AvatarRef(input.BuyerID)
		asset, err = media.FinalizeField(ctx, service.deps.Media, media.Uploader{ID: current.AccountID}, media.Field{Name: "AvatarStagedToken"}, avatarToken, ref.Type)
		if err != nil {
			return model.Profile{}, err
		}
	}

	var updated store.BuyerBuyer
	err := service.deps.DB.InTx(ctx, func(ctx context.Context) error {
		queries := store.New(service.deps.DB.Conn(ctx))
		current, err := stint.Require(ctx, queries, input.BuyerID, false)
		if err != nil {
			return err
		}
		if current.Version != input.ExpectedVersion {
			return apperr.VersionConflict("買家資料已被修改")
		}
		merged, textChanged, err := service.mergeChecked(stint.Profile(current), input)
		if err != nil {
			return err
		}
		if !textChanged && !hasAvatar {
			updated = current
			return nil
		}

		row := current
		version := input.ExpectedVersion
		now := service.deps.Now().UTC()
		if textChanged {
			var birthDate pgtype.Date
			if merged.BirthDate != nil {
				birthDate = pgtype.Date{Time: *merged.BirthDate, Valid: true}
			}
			var genderStr *string
			if merged.Gender != nil {
				s := string(*merged.Gender)
				genderStr = &s
			}
			row, err = queries.UpdateProfile(ctx, store.UpdateProfileParams{
				BuyerID:     input.BuyerID,
				DisplayName: &merged.DisplayName,
				Phone:       merged.Phone,
				Gender:      genderStr,
				BirthDate:   birthDate,
				City:        merged.City,
				Job:         merged.Job,
				UpdatedAt:   now,
				Version:     version,
			})
			if errors.Is(err, pgx.ErrNoRows) {
				return apperr.VersionConflict("買家資料已被修改")
			}
			if err != nil {
				return err
			}
			version = row.Version
		}

		if hasAvatar {
			ref := mediaref.AvatarRef(input.BuyerID)
			if err := mediaref.Reject(ctx, queries, ref, media.SyncCandidate{Field: "AvatarStagedToken", AssetID: asset.ID}); err != nil {
				return err
			}
			row, err = queries.UpdateAvatar(ctx, store.UpdateAvatarParams{
				BuyerID:       input.BuyerID,
				Avatar:        &asset.ObjectKey,
				AvatarAssetID: &asset.ID,
				UpdatedAt:     now,
				Version:       version,
			})
			if errors.Is(err, pgx.ErrNoRows) {
				return apperr.VersionConflict("買家資料已被修改")
			}
			if err != nil {
				return err
			}
			var retired []string
			if current.AvatarAssetID != nil && *current.AvatarAssetID != asset.ID {
				retired = append(retired, *current.AvatarAssetID)
			}
			if err := mediaref.Changed(ctx, queries, service.deps.Jobs, now, ref, retired...); err != nil {
				return err
			}
		}

		updated = row
		return nil
	})
	if err != nil {
		return model.Profile{}, model.Error(err)
	}
	return stint.Profile(updated), nil
}

func (service *Service) mergeChecked(before model.Profile, input model.UpdateProfileInput) (model.Profile, bool, error) {
	candidate := before
	if input.DisplayName != nil {
		candidate.DisplayName = *input.DisplayName
	}
	if input.Phone != nil {
		candidate.Phone = nullableText(*input.Phone)
	}
	if input.Gender != nil {
		g := model.Gender(strings.TrimSpace(string(*input.Gender)))
		if g == "" {
			candidate.Gender = nil
		} else {
			candidate.Gender = &g
		}
	}
	if input.City != nil {
		candidate.City = nullableText(*input.City)
	}
	if input.Job != nil {
		candidate.Job = nullableText(*input.Job)
	}
	if input.BirthDate != nil {
		if input.BirthDate.IsZero() {
			candidate.BirthDate = nil
		} else {
			candidate.BirthDate = input.BirthDate
		}
	}
	mergedInput := model.UpdateProfileInput{
		DisplayName: &candidate.DisplayName,
		Phone:       candidate.Phone,
		Gender:      candidate.Gender,
		BirthDate:   candidate.BirthDate,
		City:        candidate.City,
		Job:         candidate.Job,
	}
	now := service.deps.Now().UTC()
	if err := Validate(&mergedInput, now); err != nil {
		return model.Profile{}, false, err
	}
	candidate.DisplayName = *mergedInput.DisplayName
	if mergedInput.Phone != nil {
		candidate.Phone = nullableText(*mergedInput.Phone)
	}

	textChanged := before.DisplayName != candidate.DisplayName ||
		!equalNullableString(before.Phone, candidate.Phone) ||
		!equalGender(before.Gender, candidate.Gender) ||
		!equalNullableString(before.City, candidate.City) ||
		!equalNullableString(before.Job, candidate.Job) ||
		!equalBirthDate(before.BirthDate, candidate.BirthDate)

	return candidate, textChanged, nil
}

func equalNullableString(a, b *string) bool {
	if a == nil || b == nil {
		return a == b
	}
	return *a == *b
}

func equalGender(a, b *model.Gender) bool {
	if a == nil || b == nil {
		return a == b
	}
	return *a == *b
}

func equalBirthDate(a, b *time.Time) bool {
	if a == nil || b == nil {
		return a == b
	}
	return a.Equal(*b)
}
