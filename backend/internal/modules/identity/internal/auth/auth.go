package auth

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/Will413028/hawkr/backend/internal/modules/identity/internal/model"
	"github.com/Will413028/hawkr/backend/internal/modules/identity/internal/session"
	"github.com/Will413028/hawkr/backend/internal/modules/identity/internal/store"
	"github.com/Will413028/hawkr/backend/internal/platform/adminperm"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
	"github.com/Will413028/hawkr/backend/internal/platform/db"
)

// errSessionInvalid 是認證失敗時給使用者的錯誤，與 session 無效時相同：client 一律回到登入。
func errSessionInvalid() error { return session.Invalid() }

// Authenticate 驗證顧客的 session token 或後台 Access/dev token，回傳呼叫者。
//
// 後台 token 是 JWT，以 iss 精確比對選到後台 verifier；其他一律當顧客的 opaque session token，由 session 解析（閒置與絕對逾時、
// 續期、帳號的停權與角色，每個請求都讀）。管理員第一次認證時建列（零角色）。停權是授權問題，不是認證問題：停權的帳號照樣認證成功，
// 結果帶停權時間（Authenticated.SuspendedAt），由入口依動作的性質決定能做什麼（A7-D17、A7-D18）。
//
// 錯誤：token 無效、逾時或帳號刪除回 KindUnauthenticated；問不到公鑰或資料庫時回其他錯誤。
func (m *Service) Authenticate(ctx context.Context, token string) (model.Authenticated, error) {
	if token == "" {
		return model.Authenticated{}, errSessionInvalid()
	}
	if a := m.d.Admin; a != nil {
		if iss := unverifiedIssuer(token); iss != "" && a.Issuer() == iss {
			identity, err := a.Verify(ctx, token)
			if err != nil {
				if errors.Is(err, model.ErrInvalidToken) {
					return model.Authenticated{}, apperr.WithCause(errSessionInvalid(), err)
				}
				return model.Authenticated{}, fmt.Errorf("identity：驗證後台 token：%w", err)
			}
			return m.authenticateAdmin(ctx, identity)
		}
	}
	return m.d.Sessions.Resolve(ctx, token)
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

type Deps struct {
	DB       *db.DB
	Sessions *session.Service
	Admin    model.AdminVerifier
	Google   model.IDTokenVerifier
	Now      func() time.Time
	Logger   *slog.Logger
}

type Service struct{ d Deps }

func New(d Deps) *Service { return &Service{d: d} }

func (m *Service) q(ctx context.Context) *store.Queries { return store.New(m.d.DB.Conn(ctx)) }
