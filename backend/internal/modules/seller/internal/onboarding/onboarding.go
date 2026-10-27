// Package onboarding 是申請開店：驗證、建立 stint、授予 SELLER 與寫入 profile 在同一個 transaction。
package onboarding

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"math/big"
	"time"
	"uuid"

	"github.com/jackc/pgx/v5"

	"github.com/Will413028/hawkr/backend/internal/modules/identity"
	"github.com/Will413028/hawkr/backend/internal/modules/seller/internal/agreement"
	"github.com/Will413028/hawkr/backend/internal/modules/seller/internal/model"
	"github.com/Will413028/hawkr/backend/internal/modules/seller/internal/stint"
	"github.com/Will413028/hawkr/backend/internal/modules/seller/internal/store"
	"github.com/Will413028/hawkr/backend/internal/platform/db"
	"github.com/Will413028/hawkr/backend/internal/platform/events"
	"github.com/Will413028/hawkr/backend/internal/platform/jobs"
	"github.com/Will413028/hawkr/backend/internal/platform/uitext"
)

// Identity 是開店需要的 identity 公開 API：先鎖帳號列（鎖順序：帳號列先於店家列），再授予 SELLER。
type Identity interface {
	LockAccount(context.Context, int64) error
	GrantRole(context.Context, identity.GrantRoleInput) error
	Account(context.Context, int64) (identity.Account, error)
}

// Deps 外部依賴；clock 與 store_code 產生器由組裝點注入。
type Deps struct {
	DB        *db.DB
	Events    *events.Bus
	Jobs      jobs.Enqueuer
	Identity  Identity
	Now       func() time.Time
	StoreCode func(now time.Time) (int64, error)
}

// Service 是開店的 use case。
type Service struct{ d Deps }

// maxStoreCodeAttempts 是 store_code 撞號時的重試次數。
const maxStoreCodeAttempts = 5

// New 建立服務；StoreCode 沒給時用 RandomStoreCode。
func New(d Deps) *Service {
	if d.StoreCode == nil {
		d.StoreCode = RandomStoreCode
	}
	return &Service{d: d}
}

// RandomStoreCode 產生 store_code：西元年的後兩碼乘以 10^8，加上 8 位密碼學亂數（PayUni 的 MerID 是「HAWK」加上它）。
func RandomStoreCode(now time.Time) (int64, error) {
	n, err := rand.Int(rand.Reader, big.NewInt(100_000_000))
	if err != nil {
		return 0, fmt.Errorf("seller：產生 store_code：%w", err)
	}
	return int64(now.UTC().Year()%100)*100_000_000 + n.Int64(), nil
}

// Apply 申請開店。所有輸入先驗證，任何一欄不合法就不建店、不授予角色；通過後在同一個 transaction 內鎖帳號、
// 確認沒有目前的 stint、帳號沒有停權（停權以帳號為單位，關店不會解除，所以先前 stint 上未解除的停權也算；用的是
// 讀取時的同一個推導 stint.Effective，處分一到期就不算，不必等到期 job；生效中的終止合作與停權回不同的 reason，見 model.OpeningBlock）、建立 stint 與 profile、授予 SELLER（帳號等待
// 刪除時 identity 拒絕，整筆回滾），並發出 StoreOpened。帳號之前關過店時建立新的 stint；舊 stint 上處分已到期、期間還沒
// 被 job 結束的那一段，由之後第一次 Recompute 結束（不會跟到新的 stint）。
//
// 同意的文件（AcceptedAgreements）與 profile 一同先驗證：種類不對回 InvalidInput，版本不是現行版本回 FailedPrecondition
// （AGREEMENT_OUTDATED），都不建店；通過後同意紀錄與建店在同一個 transaction。
func (s *Service) Apply(ctx context.Context, in model.ApplyInput) (model.Store, error) {
	if err := model.Positive(in.AccountID); err != nil {
		return model.Store{}, err
	}
	profile := model.Profile{
		StoreName:     in.StoreName,
		Description:   model.Text(in.Description),
		ContactPhone:  model.Text(in.ContactPhone),
		TownCode:      model.Text(in.TownCode),
		AddressDetail: model.Text(in.AddressDetail),
		Prize:         model.Text(in.Prize),
	}
	if err := model.Normalize(&profile); err != nil {
		return model.Store{}, err
	}
	if err := model.CheckAgreements(model.ApplyAgreements, in.AcceptedAgreements); err != nil {
		return model.Store{}, err
	}
	var created store.SellerStore
	err := s.d.DB.InTx(ctx, func(ctx context.Context) error {
		if err := s.d.Identity.LockAccount(ctx, in.AccountID); err != nil {
			return err
		}
		q := store.New(s.d.DB.Conn(ctx))
		now := s.d.Now().UTC()
		state, err := s.openingState(ctx, q, in.AccountID, now)
		if err != nil {
			return err
		}
		if err := model.OpeningBlock(uitext.LocaleFrom(ctx), state); err != nil {
			return err
		}
		if created, err = s.insert(ctx, q, in.AccountID, profile, now); err != nil {
			return err
		}
		if err := agreement.Record(ctx, q, created.StoreID, in.AccountID, in.AcceptedAgreements, now, agreement.Subject{},
			agreement.Source{ClientIP: in.ClientIP, UserAgent: in.UserAgent}); err != nil {
			return err
		}
		if err := s.d.Identity.GrantRole(ctx, identity.GrantRoleInput{
			AccountID: in.AccountID, Role: identity.RoleSeller, SourceID: &created.StoreID,
		}); err != nil {
			return err
		}
		if err := events.Publish(ctx, s.d.Events, model.StoreOpened{
			EventID: uuid.NewV7(), OccurredAt: now,
			StoreID: created.StoreID, StoreCode: created.StoreCode, AccountID: created.AccountID,
		}); err != nil {
			return err
		}
		return stint.Invalidate(ctx, s.d.Jobs, created)
	})
	if err != nil {
		return model.Store{}, err
	}
	return stint.Load(ctx, store.New(s.d.DB.Conn(ctx)), created, s.d.Now().UTC())
}

// openingState 讀出判斷能不能開店的全部輸入（開店用 model.OpeningBlock、入口的可用性用 model.OpeningBlockers，都只看這個狀態）：目前的店家、帳號
// 的生命週期、生效中的措施（讀取時的同一個推導 stint.Effective，處分一到期就不算，不必等到期 job）。
func (s *Service) openingState(ctx context.Context, q *store.Queries, accountID int64, now time.Time) (model.OpeningState, error) {
	hasStore := true
	if _, err := q.CurrentStore(ctx, accountID); errors.Is(err, pgx.ErrNoRows) {
		hasStore = false
	} else if err != nil {
		return model.OpeningState{}, err
	}
	account, err := s.d.Identity.Account(ctx, accountID)
	if err != nil {
		return model.OpeningState{}, err
	}
	sanctions, err := stint.Effective(ctx, q, accountID, now)
	if err != nil {
		return model.OpeningState{}, err
	}
	return model.OpeningState{HasStore: hasStore, AccountActive: account.Lifecycle == identity.LifecycleActive, Sanctions: sanctions}, nil
}

// Opening 回答帳號此刻能不能申請開店與不能的原因（依優先序），與 Apply 的前置條件是同一個狀態與同一個判斷（model.OpeningBlockers）；唯讀，不鎖。
func (s *Service) Opening(ctx context.Context, accountID int64) (model.StoreOpening, error) {
	if err := model.Positive(accountID); err != nil {
		return model.StoreOpening{}, err
	}
	state, err := s.openingState(ctx, store.New(s.d.DB.Conn(ctx)), accountID, s.d.Now().UTC())
	if err != nil {
		return model.StoreOpening{}, err
	}
	blockers := model.OpeningBlockers(state)
	return model.StoreOpening{CanApply: len(blockers) == 0, Unavailable: model.OpeningUnavailabilities(uitext.LocaleFrom(ctx), blockers)}, nil
}

// insert 建立 stint；store_code 撞號時換一個重試（ON CONFLICT DO NOTHING 不會讓 transaction 失效）。
func (s *Service) insert(ctx context.Context, q *store.Queries, accountID int64, p model.Profile, now time.Time) (store.SellerStore, error) {
	for range maxStoreCodeAttempts {
		code, err := s.d.StoreCode(now)
		if err != nil {
			return store.SellerStore{}, err
		}
		row, err := q.CreateStore(ctx, store.CreateStoreParams{
			StoreCode: code, AccountID: accountID, StoreName: p.StoreName, Description: p.Description,
			ContactPhone: p.ContactPhone, TownCode: p.TownCode, AddressDetail: p.AddressDetail,
			Prize: p.Prize, Now: now,
		})
		if errors.Is(err, pgx.ErrNoRows) {
			continue
		}
		if err != nil {
			return row, err
		}
		// 公告列與店家列同一個 transaction 建立：之後每次公告都只鎖它、不插入（version 0：還沒公告過）。
		return row, q.InsertStoreExposure(ctx, row.StoreID)
	}
	return store.SellerStore{}, fmt.Errorf("seller：store_code 連續 %d 次撞號", maxStoreCodeAttempts)
}
