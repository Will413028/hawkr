package auth

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/Will413028/hawkr/backend/internal/modules/identity/internal/model"
	"github.com/Will413028/hawkr/backend/internal/modules/identity/internal/store"
	"github.com/Will413028/hawkr/backend/internal/platform/adminperm"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
	"github.com/Will413028/hawkr/backend/internal/platform/db"
)

// errSessionInvalid 是認證失敗時給使用者的錯誤：沒有 token、token 無效或過期，或帳號已刪除。這幾種情況刻意
// 不區分，client 一律回到登入。
func errSessionInvalid() error { return apperr.Unauthenticated("登入已失效，請重新登入") }

// Authenticate 驗證顧客 Clerk 或後台 Access/dev token，回傳呼叫者。
//
// token 的 issuer 決定它屬於顧客還是後台。顧客第一次認證時建立帳號（授予買家角色），管理員第一次認證時建列。
// 停權、角色與刪除以本模組的資料為準，每個請求都讀：token 在 Clerk
// 撤銷之後的效期內仍驗得過，但帳號刪除時回 unauthenticated。停權是授權問題，不是認證問題：停權的帳號照樣認證成功，
// 結果帶停權時間（Authenticated.SuspendedAt），由入口依動作的性質決定能做什麼（A7-D17、A7-D18）。
//
// 錯誤：token 無效或帳號刪除回 KindUnauthenticated；問不到公鑰或資料庫時回其他錯誤。
func (m *Service) Authenticate(ctx context.Context, token string) (model.Authenticated, error) {
	if token == "" {
		return model.Authenticated{}, errSessionInvalid()
	}
	iss := unverifiedIssuer(token)
	if c := clerkFor(m.d.Customers, iss); c != nil {
		userID, err := verifyToken(ctx, c, token)
		if err != nil {
			return model.Authenticated{}, err
		}
		return m.authenticateAccount(ctx, userID)
	}
	if a := m.d.Admin; a != nil && iss != "" && a.Issuer() == iss {
		identity, err := a.Verify(ctx, token)
		if err != nil {
			if errors.Is(err, model.ErrInvalidToken) {
				return model.Authenticated{}, apperr.WithCause(errSessionInvalid(), err)
			}
			return model.Authenticated{}, fmt.Errorf("identity：驗證後台 token：%w", err)
		}
		return m.authenticateAdmin(ctx, identity)
	}
	return model.Authenticated{}, errSessionInvalid()
}

// unverifiedIssuer 讀出 JWT 的 iss，只用來選擇驗證它的 application；簽章與 issuer 由那個 application 驗證。
func unverifiedIssuer(token string) string {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return ""
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return ""
	}
	var c struct {
		Issuer string `json:"iss"`
	}
	if json.Unmarshal(payload, &c) != nil {
		return ""
	}
	return c.Issuer
}

func clerkFor(apps []model.Clerk, issuer string) model.Clerk {
	if issuer == "" {
		return nil
	}
	i := slices.IndexFunc(apps, func(c model.Clerk) bool { return c.Issuer() == issuer })
	if i < 0 {
		return nil
	}
	return apps[i]
}

func verifyToken(ctx context.Context, c model.Clerk, token string) (model.ClerkIdentity, error) {
	userID, err := c.Verify(ctx, token)
	switch {
	case errors.Is(err, model.ErrInvalidToken):
		return model.ClerkIdentity{}, apperr.WithCause(errSessionInvalid(), err)
	case err != nil:
		return model.ClerkIdentity{}, fmt.Errorf("identity：驗證 session token：%w", err)
	}
	return userID, nil
}

// authenticateAccount 回傳 Clerk 使用者對應的帳號；還沒有帳號時建立。
func (m *Service) authenticateAccount(ctx context.Context, u model.ClerkIdentity) (model.Authenticated, error) {
	q := m.q(ctx)
	row, err := q.ResolveAccount(ctx, u.Subject)
	switch {
	case err == nil:
		if err := accountStatus(row.DeletedAt); err != nil {
			return model.Authenticated{}, err
		}
		return model.Authenticated{Principal: model.Principal{Kind: model.PrincipalAccount, ID: row.ID, Roles: toRoles(row.Roles)}, SuspendedAt: row.SuspendedAt}, nil
	case !errors.Is(err, pgx.ErrNoRows):
		return model.Authenticated{}, fmt.Errorf("identity：讀取帳號：%w", err)
	}
	id, created, err := m.d.Account.CreateForClerk(ctx, u.Subject, u.Email, u.Name)
	if err != nil {
		return model.Authenticated{}, err
	}
	if !created {
		row, err := m.q(ctx).ResolveAccount(ctx, u.Subject)
		if err != nil {
			return model.Authenticated{}, fmt.Errorf("identity：讀取剛建立的帳號：%w", err)
		}
		if err := accountStatus(row.DeletedAt); err != nil {
			return model.Authenticated{}, err
		}
		return model.Authenticated{Principal: model.Principal{Kind: model.PrincipalAccount, ID: row.ID, Roles: toRoles(row.Roles)}, SuspendedAt: row.SuspendedAt}, nil
	}
	m.d.Logger.InfoContext(ctx, "identity：建立帳號", "account_id", id)
	row, err = m.q(ctx).ResolveAccount(ctx, u.Subject)
	if err != nil {
		return model.Authenticated{}, fmt.Errorf("identity：讀取剛建立的帳號角色：%w", err)
	}
	return model.Authenticated{Principal: model.Principal{Kind: model.PrincipalAccount, ID: id, Roles: toRoles(row.Roles)}}, nil
}

// authenticateAdmin 回傳 Access subject 對應的管理員與他目前的角色；還沒有管理員時建立（零角色）。角色每個請求讀一次，不快取
// （ADR 2026-10-04-admin-roles）：撤銷在下一個請求生效。
func (m *Service) authenticateAdmin(ctx context.Context, a model.AdminIdentity) (model.Authenticated, error) {
	row, err := m.q(ctx).ResolveAdmin(ctx, a.Subject)
	switch {
	case err == nil:
		return adminAuthenticated(row.ID, row.Roles), nil
	case !errors.Is(err, pgx.ErrNoRows):
		return model.Authenticated{}, fmt.Errorf("identity：讀取管理員：%w", err)
	}
	id, err := m.q(ctx).InsertAdmin(ctx, store.InsertAdminParams{
		AccessSubject: a.Subject, Email: a.Email, Now: m.d.Now(),
	})
	if errors.Is(err, pgx.ErrNoRows) {
		// 同一位管理員同時的另一個請求剛建好
		if row, err = m.q(ctx).ResolveAdmin(ctx, a.Subject); err != nil {
			return model.Authenticated{}, fmt.Errorf("identity：讀取剛建立的管理員：%w", err)
		}
		return adminAuthenticated(row.ID, row.Roles), nil
	}
	if err != nil {
		return model.Authenticated{}, fmt.Errorf("identity：建立管理員：%w", err)
	}
	m.d.Logger.InfoContext(ctx, "identity：建立管理員", "admin_id", id)
	return adminAuthenticated(id, nil), nil
}

func adminAuthenticated(id int64, roles []string) model.Authenticated {
	adminRoles := make([]adminperm.Role, len(roles))
	for i, r := range roles {
		adminRoles[i] = adminperm.Role(r)
	}
	return model.Authenticated{Principal: model.Principal{Kind: model.PrincipalAdmin, ID: id, AdminRoles: adminRoles}}
}

func toRoles(s []string) []model.Role {
	roles := make([]model.Role, len(s))
	for i, r := range s {
		roles[i] = model.Role(r)
	}
	return roles
}

type AccountCreator interface {
	CreateForClerk(ctx context.Context, userID, email, name string) (id int64, created bool, err error)
}

type Deps struct {
	DB        *db.DB
	Account   AccountCreator
	Customers []model.Clerk
	Admin     model.AdminVerifier
	Google    model.IDTokenVerifier
	Now       func() time.Time
	Logger    *slog.Logger
}

type Service struct{ d Deps }

func New(d Deps) *Service { return &Service{d: d} }

func (m *Service) q(ctx context.Context) *store.Queries { return store.New(m.d.DB.Conn(ctx)) }

func accountStatus(deleted *time.Time) error {
	if deleted != nil {
		return errSessionInvalid()
	}
	return nil
}
