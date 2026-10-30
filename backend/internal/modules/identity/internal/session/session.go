// Package session 是登入 session 的建立、解析與撤銷，以及登入流程的暫存狀態（web 的 login_flows、App 的 nonce）。
//
// session 的規格：ADR 2026-09-27-identity-opaque-sessions D1–D3；聯合登入的流程：ADR 2026-10-30-identity-self-hosted-federated-login。
package session

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"log/slog"
	"time"
	"uuid"

	"github.com/jackc/pgx/v5"

	"github.com/Will413028/hawkr/backend/internal/modules/identity/internal/model"
	"github.com/Will413028/hawkr/backend/internal/modules/identity/internal/store"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
	"github.com/Will413028/hawkr/backend/internal/platform/db"
)

const (
	// WebIdle、AppIdle 是閒置逾時：距離 last_used_at 超過就失效（opaque-sessions ADR D2）。
	WebIdle = 7 * 24 * time.Hour
	AppIdle = 14 * 24 * time.Hour
	// Absolute 是絕對逾時：無論多常使用，建立後超過就失效。
	Absolute = 90 * 24 * time.Hour
	// RenewEvery 是 last_used_at 最多寫入一次的間隔。
	RenewEvery = 24 * time.Hour
	// FlowTTL 是 web 登入流程暫存列的壽命；AppNonceTTL 是 App 的 nonce 壽命。
	FlowTTL     = 10 * time.Minute
	AppNonceTTL = 5 * time.Minute

	tokenBytes = 32
	// tokenLen 是 32 bytes 的 base64url（無填充）長度；長度不符的 token 不必雜湊與查詢。
	tokenLen = 43
)

// Invalid 是認證失敗時給使用者的錯誤：沒有 token、token 無效或過期，或帳號已刪除。這幾種情況刻意不區分，client 一律回到登入。
func Invalid() error { return apperr.Unauthenticated("登入已失效，請重新登入") }

// Idle 回傳 client 的閒置逾時。
func Idle(c model.ClientKind) time.Duration {
	if c == model.ClientApp {
		return AppIdle
	}
	return WebIdle
}

type Deps struct {
	DB     *db.DB
	Now    func() time.Time
	Logger *slog.Logger
}

type Service struct{ d Deps }

func New(d Deps) *Service { return &Service{d: d} }

func (s *Service) q(ctx context.Context) *store.Queries { return store.New(s.d.DB.Conn(ctx)) }

// newSecret 回傳 32 bytes CSPRNG 的 base64url 字串。
func newSecret() (string, error) {
	b := make([]byte, tokenBytes)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("identity：產生隨機值：%w", err)
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

// Hash 是 token、state、nonce 等祕密值存進資料庫的形式。
func Hash(secret string) []byte {
	h := sha256.Sum256([]byte(secret))
	return h[:]
}

// Issued 是剛建立的 session：Token 是原文，只在這裡出現一次。
type Issued struct {
	Token string
	ID    uuid.UUID
}

// Create 為帳號建立新 session，並順手清掉這個帳號已逾時的 session。client 必須是已知的種類。登入建立 session 時由呼叫方包進同一個 transaction。
func (s *Service) Create(ctx context.Context, accountID int64, client model.ClientKind) (Issued, error) {
	if !client.Valid() {
		return Issued{}, fmt.Errorf("identity：未知的 client 種類 %q", client)
	}
	token, err := newSecret()
	if err != nil {
		return Issued{}, err
	}
	now := s.d.Now()
	q := s.q(ctx)
	if _, err := q.DeleteExpiredSessionsOfAccount(ctx, store.DeleteExpiredSessionsOfAccountParams{
		AccountID: accountID, Now: now, WebIdleBefore: now.Add(-WebIdle), AppIdleBefore: now.Add(-AppIdle),
	}); err != nil {
		return Issued{}, fmt.Errorf("identity：清除逾時的 session：%w", err)
	}
	id, err := q.InsertSession(ctx, store.InsertSessionParams{
		TokenHash: Hash(token), AccountID: accountID, Client: string(client), Now: now, AbsoluteExpiresAt: now.Add(Absolute),
	})
	if err != nil {
		return Issued{}, fmt.Errorf("identity：建立 session：%w", err)
	}
	return Issued{Token: token, ID: id}, nil
}

// Resolve 解析 token：雜湊查列，檢查閒置與絕對逾時，續期（一天最多寫一次 last_used_at），並讀出帳號的停權狀態與角色。
// 停權是授權問題，不是認證問題：停權帳號的 session 照樣解析成功，結果帶停權時間。token 無效、逾時或帳號已刪除回 KindUnauthenticated。
func (s *Service) Resolve(ctx context.Context, token string) (model.Authenticated, error) {
	if len(token) != tokenLen {
		return model.Authenticated{}, Invalid()
	}
	q := s.q(ctx)
	hash := Hash(token)
	row, err := q.ResolveSession(ctx, hash)
	if errors.Is(err, pgx.ErrNoRows) {
		return model.Authenticated{}, Invalid()
	}
	if err != nil {
		return model.Authenticated{}, fmt.Errorf("identity：讀取 session：%w", err)
	}
	now := s.d.Now()
	if row.DeletedAt != nil || !now.Before(row.AbsoluteExpiresAt) || !now.Before(row.LastUsedAt.Add(Idle(model.ClientKind(row.Client)))) {
		if _, err := q.DeleteSessionByTokenHash(ctx, hash); err != nil {
			s.d.Logger.WarnContext(ctx, "identity：刪除逾時的 session 失敗", "error", err)
		}
		return model.Authenticated{}, Invalid()
	}
	if !now.Before(row.LastUsedAt.Add(RenewEvery)) {
		if _, err := q.RenewSession(ctx, store.RenewSessionParams{ID: row.ID, Now: now, RenewBefore: now.Add(-RenewEvery)}); err != nil {
			return model.Authenticated{}, fmt.Errorf("identity：session 續期：%w", err)
		}
	}
	roles := make([]model.Role, len(row.Roles))
	for i, r := range row.Roles {
		roles[i] = model.Role(r)
	}
	return model.Authenticated{
		Principal:   model.Principal{Kind: model.PrincipalAccount, ID: row.AccountID, Roles: roles},
		SuspendedAt: row.SuspendedAt,
	}, nil
}

// SignOut 刪除 token 對應的 session 列；token 無效或已經不存在時成功（冪等）。
func (s *Service) SignOut(ctx context.Context, token string) error {
	if len(token) != tokenLen {
		return nil
	}
	if _, err := s.q(ctx).DeleteSessionByTokenHash(ctx, Hash(token)); err != nil {
		return fmt.Errorf("identity：登出：%w", err)
	}
	return nil
}

// List 回傳帳號所有 session，新到舊；包含已逾時但還沒清掉的列。
func (s *Service) List(ctx context.Context, accountID int64) ([]model.SessionInfo, error) {
	rows, err := s.q(ctx).ListSessionsOfAccount(ctx, accountID)
	if err != nil {
		return nil, fmt.Errorf("identity：列出 session：%w", err)
	}
	out := make([]model.SessionInfo, len(rows))
	for i, r := range rows {
		out[i] = model.SessionInfo{
			ID: r.ID, AccountID: r.AccountID, Client: model.ClientKind(r.Client), CreatedAt: r.CreatedAt,
			LastUsedAt: r.LastUsedAt, AbsoluteExpiresAt: r.AbsoluteExpiresAt, AuthTime: r.AuthTime,
		}
	}
	return out, nil
}

// Revoke 刪除帳號自己的一個 session；不存在或屬於別的帳號回 NotFound。重新驗證的判斷由呼叫方依 AuthTime 決定。
func (s *Service) Revoke(ctx context.Context, accountID int64, id uuid.UUID) error {
	n, err := s.q(ctx).DeleteSessionOfAccount(ctx, store.DeleteSessionOfAccountParams{ID: id, AccountID: accountID})
	if err != nil {
		return fmt.Errorf("identity：終止 session：%w", err)
	}
	if n == 0 {
		return apperr.NotFound("session 不存在")
	}
	return nil
}

// MarkAuthenticated 記下帳號的這個 session 剛重新以 provider 驗證過；不存在或屬於別的帳號回 NotFound。
func (s *Service) MarkAuthenticated(ctx context.Context, accountID int64, id uuid.UUID) error {
	q := s.q(ctx)
	n, err := q.SetSessionAuthTime(ctx, store.SetSessionAuthTimeParams{ID: id, AccountID: accountID, Now: s.d.Now()})
	if err != nil {
		return fmt.Errorf("identity：記錄重新驗證：%w", err)
	}
	if n == 0 {
		if _, err := q.GetSessionOfAccount(ctx, store.GetSessionOfAccountParams{ID: id, AccountID: accountID}); errors.Is(err, pgx.ErrNoRows) {
			return apperr.NotFound("session 不存在")
		} else if err != nil {
			return fmt.Errorf("identity：讀取 session：%w", err)
		}
	}
	return nil
}
