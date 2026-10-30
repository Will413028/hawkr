// Package login 是以 provider 身分登入、建帳、自動連結與手動連結（ADR 2026-10-30-identity-self-hosted-federated-login D1、D4）。
package login

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/Will413028/hawkr/backend/internal/modules/identity/internal/model"
	"github.com/Will413028/hawkr/backend/internal/modules/identity/internal/session"
	"github.com/Will413028/hawkr/backend/internal/modules/identity/internal/store"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
	"github.com/Will413028/hawkr/backend/internal/platform/db"
	"github.com/Will413028/hawkr/backend/internal/platform/events"
	"github.com/Will413028/hawkr/backend/internal/platform/trimtext"
)

type Deps struct {
	DB       *db.DB
	Events   *events.Bus
	Sessions *session.Service
	Now      func() time.Time
	Logger   *slog.Logger
}

type Service struct{ d Deps }

func New(d Deps) *Service { return &Service{d: d} }

func (s *Service) q(ctx context.Context) *store.Queries { return store.New(s.d.DB.Conn(ctx)) }

// errRace 表示同一個 (provider, subject) 的另一個登入先一步寫入了身分：放棄這個 transaction（帳號與事件一起 rollback），重做時會讀到它。
var errRace = errors.New("identity：並發的首次登入")

// maxAttempts 是並發首次登入的重試上限：第二次一定讀得到對方 commit 的身分。
const maxAttempts = 2

// gmailDomain 是 Google 保證 email 屬於該 Google 帳號本人的網域；其他網域要有 hd（Workspace 網域）才算。
const gmailDomain = "gmail.com"

// normalize 驗證並整理 provider 身分：provider 是已知的值、subject 非空；email 去前後空白，沒有 email 時 EmailVerified 一律是假。
func normalize(id model.ProviderIdentity) (model.ProviderIdentity, error) {
	if !id.Provider.Valid() {
		return id, apperr.InvalidInputOn([]string{"Provider"}, "不支援的登入方式 %q", id.Provider)
	}
	if id.Subject == "" {
		return id, apperr.InvalidInputOn([]string{"Subject"}, "缺少 provider 的使用者識別")
	}
	id.Email = trimtext.Trim(id.Email)
	id.HostedDomain = trimtext.Trim(id.HostedDomain)
	id.Name = trimtext.Trim(id.Name)
	if id.Email == "" {
		id.EmailVerified = false
	}
	return id, nil
}

// verifiedEmail 回傳可以寫進帳號寄件地址的 email：只有 provider 保證已驗證的。
func verifiedEmail(id model.ProviderIdentity) *string {
	if id.EmailVerified && id.Email != "" {
		return &id.Email
	}
	return nil
}

func nonEmpty(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

// AutoLinkEligible 回報 id 是不是可以用 email 自動連結到既有帳號（D4）：只有 Google、email 已驗證，而且 email 的網域是 gmail.com
// 或帶 hd；帶 hd 時 email 的網域必須等於 hd，不等就不符資格。LINE 沒有這個資格：它不保證 email 屬於本人。
func AutoLinkEligible(id model.ProviderIdentity) bool {
	if id.Provider != model.ProviderGoogle || !id.EmailVerified || id.Email == "" {
		return false
	}
	at := strings.LastIndexByte(id.Email, '@')
	if at < 0 {
		return false
	}
	domain := id.Email[at+1:]
	if id.HostedDomain != "" {
		return strings.EqualFold(domain, id.HostedDomain)
	}
	return strings.EqualFold(domain, gmailDomain)
}

// Login 以 provider 驗證過的身分登入，換發新 session（舊的登入前 token 同一個 transaction 刪除）：
//   - 已有這個身分：登入它的帳號；帳號已刪除（身分是墓碑）回 KindUnauthenticated，不建新帳號；
//   - 否則符合 AutoLinkEligible 且恰好一個存活帳號有符合資格的 Google 身分、其 email 等於這個已驗證 email：連結到該帳號；
//   - 否則建立新帳號（沒有 email 也照樣建）與身分，發出 AccountCreated。
//
// 每次登入以已驗證 email 更新帳號的寄件地址與身分記錄的 email。停權的帳號照樣登入（A7-D18）。
func (s *Service) Login(ctx context.Context, in model.LoginInput) (model.LoginResult, error) {
	id, err := normalize(in.Identity)
	if err != nil {
		return model.LoginResult{}, err
	}
	if !in.Client.Valid() {
		return model.LoginResult{}, apperr.InvalidInputOn([]string{"Client"}, "不支援的 client 種類 %q", in.Client)
	}
	in.Identity = id
	for range maxAttempts {
		var res model.LoginResult
		err = s.d.DB.InTx(ctx, func(ctx context.Context) error {
			var txErr error
			res, txErr = s.login(ctx, in)
			return txErr
		})
		if !errors.Is(err, errRace) {
			return res, err
		}
	}
	return model.LoginResult{}, fmt.Errorf("identity：登入連續撞到並發的首次登入：%w", err)
}

func (s *Service) login(ctx context.Context, in model.LoginInput) (model.LoginResult, error) {
	q := s.q(ctx)
	id := in.Identity
	now := s.d.Now()
	var res model.LoginResult

	existing, err := q.GetIdentity(ctx, store.GetIdentityParams{Provider: string(id.Provider), Subject: id.Subject})
	switch {
	case err == nil:
		if _, err := q.LockLiveAccount(ctx, existing.AccountID); errors.Is(err, pgx.ErrNoRows) {
			return res, apperr.Unauthenticated("這個登入身分的帳號已刪除")
		} else if err != nil {
			return res, fmt.Errorf("identity：鎖定身分的帳號：%w", err)
		}
		res.AccountID = existing.AccountID
		if err := q.UpdateIdentityClaims(ctx, store.UpdateIdentityClaimsParams{
			Provider: string(id.Provider), Subject: id.Subject, Email: nonEmpty(id.Email), EmailVerified: id.EmailVerified, AutoLinkEligible: AutoLinkEligible(id), Now: now,
		}); err != nil {
			return res, fmt.Errorf("identity：更新身分：%w", err)
		}
	case errors.Is(err, pgx.ErrNoRows):
		if res.AccountID, res.Linked, err = s.linkTarget(ctx, q, id); err != nil {
			return res, err
		}
		if !res.Linked {
			accountID, err := q.InsertFederatedAccount(ctx, store.InsertFederatedAccountParams{Email: verifiedEmail(id), Now: now})
			if err != nil {
				return res, fmt.Errorf("identity：建立帳號：%w", err)
			}
			res.AccountID, res.Created = accountID, true
		}
		if err := s.insertIdentity(ctx, q, res.AccountID, id); err != nil {
			return res, err
		}
		if res.Created {
			if err := events.Publish(ctx, s.d.Events, model.AccountCreated{AccountID: res.AccountID, Name: id.Name}); err != nil {
				return res, err
			}
		}
	default:
		return res, fmt.Errorf("identity：讀取身分：%w", err)
	}

	if email := verifiedEmail(id); email != nil && !res.Created {
		if _, err := q.UpdateAccountEmail(ctx, store.UpdateAccountEmailParams{ID: res.AccountID, Email: email, Now: now}); err != nil {
			return res, fmt.Errorf("identity：更新寄件地址：%w", err)
		}
	}
	if in.PreviousToken != "" {
		if err := s.d.Sessions.SignOut(ctx, in.PreviousToken); err != nil {
			return res, err
		}
	}
	issued, err := s.d.Sessions.Create(ctx, res.AccountID, in.Client)
	if err != nil {
		return res, err
	}
	res.Token, res.SessionID = issued.Token, issued.ID
	if res.Created {
		s.d.Logger.InfoContext(ctx, "identity：登入建立帳號", "account_id", res.AccountID, "provider", id.Provider)
	}
	return res, nil
}

// linkTarget 回傳可自動連結的既有帳號（已鎖定）；沒有、不唯一或不符合資格時 linked 是假。
func (s *Service) linkTarget(ctx context.Context, q *store.Queries, id model.ProviderIdentity) (accountID int64, linked bool, err error) {
	if !AutoLinkEligible(id) {
		return 0, false, nil
	}
	ids, err := q.FindAutoLinkAccountIDsByEmail(ctx, id.Email)
	if err != nil {
		return 0, false, fmt.Errorf("identity：以 email 找帳號：%w", err)
	}
	if len(ids) != 1 {
		return 0, false, nil
	}
	if _, err := q.LockLiveAccount(ctx, ids[0]); errors.Is(err, pgx.ErrNoRows) {
		return 0, false, nil
	} else if err != nil {
		return 0, false, fmt.Errorf("identity：鎖定帳號：%w", err)
	}
	return ids[0], true, nil
}

// insertIdentity 寫入身分；已被並發的登入先寫入時回 errRace。
func (s *Service) insertIdentity(ctx context.Context, q *store.Queries, accountID int64, id model.ProviderIdentity) error {
	_, err := q.InsertIdentity(ctx, store.InsertIdentityParams{
		Provider: string(id.Provider), Subject: id.Subject, AccountID: accountID, Email: nonEmpty(id.Email),
		EmailVerified: id.EmailVerified, AutoLinkEligible: AutoLinkEligible(id), Now: s.d.Now(),
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return errRace
	}
	if err != nil {
		return fmt.Errorf("identity：寫入身分：%w", err)
	}
	return nil
}

// Link 把 provider 身分手動連結到已登入的帳號。身分已連結到同一個帳號時成功（更新 email 記錄）；已屬於別的帳號回 AlreadyExists，
// 什麼都不改。帳號不存在或已刪除回 NotFound。
func (s *Service) Link(ctx context.Context, accountID int64, in model.ProviderIdentity) error {
	id, err := normalize(in)
	if err != nil {
		return err
	}
	return s.d.DB.InTx(ctx, func(ctx context.Context) error {
		q := s.q(ctx)
		if _, err := q.LockLiveAccount(ctx, accountID); errors.Is(err, pgx.ErrNoRows) {
			return apperr.NotFound("帳號 %d 不存在", accountID)
		} else if err != nil {
			return fmt.Errorf("identity：鎖定帳號：%w", err)
		}
		now := s.d.Now()
		switch err := s.insertIdentity(ctx, q, accountID, id); {
		case err == nil:
		case errors.Is(err, errRace):
			existing, err := q.GetIdentity(ctx, store.GetIdentityParams{Provider: string(id.Provider), Subject: id.Subject})
			if err != nil {
				return fmt.Errorf("identity：讀取身分：%w", err)
			}
			if existing.AccountID != accountID {
				return apperr.AlreadyExists("這個登入方式已連結到另一個帳號")
			}
			if err := q.UpdateIdentityClaims(ctx, store.UpdateIdentityClaimsParams{
				Provider: string(id.Provider), Subject: id.Subject, Email: nonEmpty(id.Email), EmailVerified: id.EmailVerified, AutoLinkEligible: AutoLinkEligible(id), Now: now,
			}); err != nil {
				return fmt.Errorf("identity：更新身分：%w", err)
			}
		default:
			return err
		}
		if email := verifiedEmail(id); email != nil {
			if _, err := q.UpdateAccountEmail(ctx, store.UpdateAccountEmailParams{ID: accountID, Email: email, Now: now}); err != nil {
				return fmt.Errorf("identity：更新寄件地址：%w", err)
			}
		}
		return nil
	})
}
