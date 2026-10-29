package applerevoke

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/riverqueue/river"

	"github.com/Will413028/hawkr/backend/internal/modules/identity/internal/model"
	"github.com/Will413028/hawkr/backend/internal/modules/identity/internal/store"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
	"github.com/Will413028/hawkr/backend/internal/platform/db"
	"github.com/Will413028/hawkr/backend/internal/platform/jobs"
)

type Deps struct {
	DB          *db.DB
	Jobs        jobs.Enqueuer
	Apple       model.IDTokenVerifier
	AppleTokens model.AppleTokens
	Sealer      model.Sealer
	Now         func() time.Time
	Logger      *slog.Logger
}

type Service struct {
	d     Deps
	saver tokenSaver
}

func New(d Deps) *Service {
	m := &Service{d: d}
	m.saver = dbSaver{m}
	return m
}

func (m *Service) q(ctx context.Context) *store.Queries { return store.New(m.d.DB.Conn(ctx)) }

func (m *Service) configured() bool {
	return m.d.Apple != nil && m.d.AppleTokens != nil && m.d.Sealer != nil
}

// 密文認證的脈絡（AAD）：用途、表與欄、帳號 ID。密文搬到別的帳號或別的用途就解不開。
func codeAAD(accountID int64) []byte {
	return []byte("identity.exchange_apple_code|code|" + strconv.FormatInt(accountID, 10))
}

// grantAAD 是保存的 job 帶的整包 token 回應的 AAD。
func grantAAD(accountID int64) []byte {
	return []byte("identity.save_apple_token|grant|" + strconv.FormatInt(accountID, 10))
}

func tokenAAD(accountID int64) []byte {
	return []byte("identity.apple_tokens|token|" + strconv.FormatInt(accountID, 10))
}

// LinkAppleCredential 驗證 App 原生登入 Apple 後交來的 identity token（簽章、issuer、audience），把它的 sub 與 audience
// 連同加密的 authorization code 排入換 refresh token 的 job；job 會確認 Apple 換 token 時簽發的 ID token 與這裡驗證的
// 是同一個身分。呼叫者是已認證的 session，保存的就是他自己的 Apple 授權。換 token 與保存在背景執行；重複呼叫只是再換一次、
// 取代保存的 token。Apple 沒有設定（本機開發）時回 unavailable。
func (m *Service) LinkAppleCredential(ctx context.Context, in model.LinkAppleCredentialInput) error {
	if !m.configured() {
		return apperr.Unavailable("目前無法連結 Apple 的授權，請稍後再試")
	}
	if in.IDToken == "" || in.AuthorizationCode == "" {
		return apperr.InvalidInput("缺少 Apple 的 identity token 或 authorization code")
	}
	id, err := m.d.Apple.Verify(ctx, in.IDToken)
	switch {
	case errors.Is(err, model.ErrInvalidToken):
		return apperr.WithCause(apperr.InvalidInput("Apple 的驗證已失效，請重新登入"), err)
	case err != nil:
		return apperr.WithCause(apperr.Unavailable("目前無法連結 Apple 的授權，請稍後再試"), err)
	}
	sealed, err := m.d.Sealer.Seal([]byte(in.AuthorizationCode), codeAAD(in.AccountID))
	if err != nil {
		return fmt.Errorf("identity：加密 Apple 的 authorization code：%w", err)
	}
	args := exchangeAppleCodeArgs{AccountID: in.AccountID, ClientID: id.Audience, Subject: id.Subject, Code: sealed}
	if err := m.d.Jobs.Enqueue(ctx, args, &river.InsertOpts{MaxAttempts: exchangeAppleCodeMaxAttempts}); err != nil {
		return fmt.Errorf("identity：排入換 Apple 的 token：%w", err)
	}
	m.d.Logger.InfoContext(ctx, "identity：排入換 Apple 的 token", "account_id", in.AccountID)
	return nil
}

// RevokeStored 在呼叫方的 transaction 內取走帳號保存的 Apple refresh token，排入撤銷的 job（job 與刪列同一個
// transaction）。沒有保存的 token 時什麼都不做。
func (m *Service) RevokeStored(ctx context.Context, accountID int64) error {
	row, err := m.q(ctx).TakeAppleToken(ctx, accountID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("identity：取走 Apple 的 token：%w", err)
	}
	args := revokeAppleTokenArgs{AccountID: accountID, ClientID: row.ClientID, Token: row.Token}
	if err := m.d.Jobs.Enqueue(ctx, args, &river.InsertOpts{MaxAttempts: revokeAppleTokenMaxAttempts}); err != nil {
		return fmt.Errorf("identity：排入撤銷 Apple 的 token：%w", err)
	}
	return nil
}

// CheckKeys 是啟動檢查：保存的密文、以及排隊或重試中的 job 參數裡的密文（換 token、保存、撤銷）用到的每一把金鑰都要在
// 金鑰環裡，否則舊金鑰被拿掉時，要到刪帳號撤銷或 job 重試才發現解不開。沒有設定 Apple（本機開發）時略過。
func (m *Service) CheckKeys(ctx context.Context) error {
	if m.d.Sealer == nil {
		return nil
	}
	ids, err := m.q(ctx).ReferencedAppleKeyIDs(ctx)
	if err != nil {
		return fmt.Errorf("identity：讀取資料庫參照的金鑰：%w", err)
	}
	// river 的表不屬於本模組的 schema（sqlc 讀不到），以原生查詢讀 job 參數裡密文欄位開頭的金鑰 ID。
	for _, f := range []struct{ kind, field string }{
		{exchangeAppleCodeArgs{}.Kind(), "code"},
		{saveAppleTokenArgs{}.Kind(), "grant"},
		{revokeAppleTokenArgs{}.Kind(), "token"},
	} {
		rows, err := m.d.DB.Conn(ctx).Query(ctx, `
			SELECT DISTINCT convert_from(substring(b FROM 2 FOR get_byte(b, 0)), 'UTF8')
			FROM (SELECT decode(args->>$2, 'base64') AS b FROM river.river_job
			      WHERE kind = $1 AND state NOT IN ('completed', 'cancelled', 'discarded')) t
			WHERE b IS NOT NULL AND length(b) > 0`, f.kind, f.field)
		if err != nil {
			return fmt.Errorf("identity：讀取 job 參數參照的金鑰：%w", err)
		}
		for rows.Next() {
			var id string
			if err := rows.Scan(&id); err != nil {
				rows.Close()
				return fmt.Errorf("identity：讀取 job 參數參照的金鑰：%w", err)
			}
			ids = append(ids, id)
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			return fmt.Errorf("identity：讀取 job 參數參照的金鑰：%w", err)
		}
	}
	var missing []string
	for _, id := range ids {
		if !m.d.Sealer.Has(id) && !slices.Contains(missing, id) {
			missing = append(missing, id)
		}
	}
	if len(missing) > 0 {
		return fmt.Errorf("identity：資料庫或排隊中的 job 參照的金鑰不在設定的金鑰環裡（IDENTITY_TOKEN_KEYS）：%s", strings.Join(missing, ", "))
	}
	return nil
}

// exchangeAppleCodeMaxAttempts：authorization code 5 分鐘內有效、只能換一次；River 預設的退避下第 4 次大約在第
// 1 分半。
const exchangeAppleCodeMaxAttempts = 4

// exchangeAppleCodeArgs 以 authorization code 換 refresh token 並保存。Subject 與 ClientID（audience）是連結時驗證過的
// identity token 的身分；Code 是加密過的。
type exchangeAppleCodeArgs struct {
	AccountID int64  `json:"account_id"`
	ClientID  string `json:"client_id"`
	Subject   string `json:"subject"`
	Code      []byte `json:"code"`
}

func (exchangeAppleCodeArgs) Kind() string { return "identity.exchange_apple_code" }

type exchangeAppleCodeWorker struct {
	river.WorkerDefaults[exchangeAppleCodeArgs]
	m *Service
}

// errAccountGone 表示換 token 的 job 執行時帳號已經刪除。
var errAccountGone = errors.New("identity：帳號已刪除")

// tokenSaver 在 transaction 內保存加密的 refresh token（取代舊的）。
type tokenSaver interface {
	save(ctx context.Context, accountID int64, clientID string, sealed []byte) error
}

type dbSaver struct{ m *Service }

func (s dbSaver) save(ctx context.Context, accountID int64, clientID string, sealed []byte) error {
	return s.m.d.DB.InTx(ctx, func(ctx context.Context) error {
		q := s.m.q(ctx)
		if _, err := q.LockLiveAccount(ctx, accountID); errors.Is(err, pgx.ErrNoRows) {
			return errAccountGone
		} else if err != nil {
			return fmt.Errorf("identity：鎖定帳號：%w", err)
		}
		if err := q.UpsertAppleToken(ctx, store.UpsertAppleTokenParams{AccountID: accountID, ClientID: clientID, Token: sealed, Now: s.m.d.Now()}); err != nil {
			return fmt.Errorf("identity：保存 Apple 的 token：%w", err)
		}
		return nil
	})
}

// Work 以 authorization code 換 refresh token。換得之後確認 Apple 簽發的 ID token 與連結時驗證的身分（sub、audience）相同：
//   - 不同、或沒有 ID token：不保存，當場撤銷換得的 token（別人的授權不能存在這個帳號上）。
//   - 帳號已經刪除：不保存，當場撤銷（刪除的 transaction 沒有看到這個 token）。
//   - 暫時性失敗（驗證 ID token 時連不到 Apple 的公鑰、加密或資料庫保存失敗）：不撤銷（撤銷會連使用者剛授權的 Apple 授權一起撤掉），
//     改排入保存的 job，把換得的 token 加密後帶著重試；code 只能用一次，所以不能靠重試這個 job。
//
// 每一種失敗都以 error 等級記 log（帶原因與帳號 ID，不含 code 與 token），讓系統性的失敗在伺服器端看得到。
func (w *exchangeAppleCodeWorker) Work(ctx context.Context, job *river.Job[exchangeAppleCodeArgs]) error {
	m, args := w.m, job.Args
	log := m.d.Logger.With("account_id", args.AccountID, "client_id", args.ClientID)
	if !m.configured() {
		log.ErrorContext(ctx, "identity：換 Apple 的 token 失敗", "reason", "apple_not_configured")
		return river.JobCancel(errors.New("identity：沒有設定 Apple，無法換 token"))
	}
	code, err := m.d.Sealer.Open(args.Code, codeAAD(args.AccountID))
	if err != nil {
		log.ErrorContext(ctx, "identity：換 Apple 的 token 失敗", "reason", "open_code", "error", err)
		return river.JobCancel(fmt.Errorf("identity：解密 Apple 的 authorization code：%w", err))
	}
	grant, err := m.d.AppleTokens.Exchange(ctx, args.ClientID, string(code))
	if errors.Is(err, model.ErrRejected) {
		log.ErrorContext(ctx, "identity：換 Apple 的 token 失敗", "reason", "exchange_rejected", "error", err)
		return river.JobCancel(err)
	}
	if err != nil {
		if job.Attempt >= job.MaxAttempts {
			log.ErrorContext(ctx, "identity：換 Apple 的 token 失敗", "reason", "exchange_unavailable", "error", err)
		}
		return fmt.Errorf("identity：換 Apple 的 token：%w", err)
	}
	verified := false
	switch reason, err := m.bind(ctx, args.Subject, args.ClientID, grant.IDToken); {
	case reason == "binding_unverifiable":
		return w.deferSave(ctx, log, args, grant, false, reason, err)
	case err != nil:
		return m.discard(ctx, log, args.ClientID, grant.RefreshToken, reason, err)
	default:
		verified = true
	}
	return w.persist(ctx, log, args, grant, verified)
}

// persist 保存已驗證綁定的 token；帳號已刪除就撤銷，暫時性的保存失敗改排保存的 job。
func (w *exchangeAppleCodeWorker) persist(ctx context.Context, log *slog.Logger, args exchangeAppleCodeArgs, grant model.AppleGrant, verified bool) error {
	sealed, err := w.m.d.Sealer.Seal([]byte(grant.RefreshToken), tokenAAD(args.AccountID))
	if err != nil {
		return w.deferSave(ctx, log, args, grant, verified, "seal_token", err)
	}
	err = w.m.saver.save(ctx, args.AccountID, args.ClientID, sealed)
	switch {
	case err == nil:
		return nil
	case errors.Is(err, errAccountGone):
		return w.m.discard(ctx, log, args.ClientID, grant.RefreshToken, "account_deleted", err)
	}
	return w.deferSave(ctx, log, args, grant, verified, "persist_failed", err)
}

// deferSave 把換得的 token 加密後排入保存的 job；連排入也失敗（資料庫不可用）時只能記 error 並放棄，
// 下一次原生登入會再交一個 code。
func (w *exchangeAppleCodeWorker) deferSave(ctx context.Context, log *slog.Logger, args exchangeAppleCodeArgs, grant model.AppleGrant, verified bool, reason string, cause error) error {
	log.ErrorContext(ctx, "identity：換得的 Apple token 暫時無法保存，改排保存的 job", "reason", reason, "error", cause)
	blob, err := json.Marshal(grantBlob{RefreshToken: grant.RefreshToken, IDToken: grant.IDToken}) //nolint:gosec // G117：grantBlob 在 Sealer.Seal 加密後才保存，refresh_token 欄位名是 Apple 的 token 名，不是寫死的憑證
	if err == nil {
		blob, err = w.m.d.Sealer.Seal(blob, grantAAD(args.AccountID))
	}
	if err == nil {
		save := saveAppleTokenArgs{AccountID: args.AccountID, ClientID: args.ClientID, Subject: args.Subject, Verified: verified, Grant: blob}
		err = w.m.d.Jobs.Enqueue(ctx, save, &river.InsertOpts{MaxAttempts: saveAppleTokenMaxAttempts})
	}
	if err != nil {
		log.ErrorContext(ctx, "identity：排入保存 Apple token 的 job 失敗，放棄（不撤銷，下一次原生登入會再交）", "reason", "defer_save_failed", "error", err)
	}
	return river.JobCancel(cause)
}

// bind 確認 Apple 換 token 時簽發的 ID token 與連結時驗證的 identity token 是同一個 sub 與 audience：避免 A 的 identity
// token 搭配 B 的 code，把 B 的授權存在 A 的帳號上。連不到 Apple 的公鑰（暫時性）回 binding_unverifiable。
func (m *Service) bind(ctx context.Context, subject, clientID, idToken string) (string, error) {
	if idToken == "" {
		return "binding_missing_id_token", errors.New("identity：Apple 的回應沒有 ID token")
	}
	id, err := m.d.Apple.Verify(ctx, idToken)
	switch {
	case errors.Is(err, model.ErrInvalidToken):
		return "binding_mismatch", fmt.Errorf("identity：Apple 換 token 回傳的 ID token 無效：%w", err)
	case err != nil:
		return "binding_unverifiable", fmt.Errorf("identity：驗證 Apple 換 token 回傳的 ID token：%w", err)
	}
	if id.Subject != subject || id.Audience != clientID {
		return "binding_mismatch", errors.New("identity：Apple 換 token 回傳的身分與連結時驗證的不同")
	}
	return "", nil
}

// discard 當場撤銷剛換得但不能保存的 refresh token（只用在身分不符與帳號已刪除），job 取消。
func (m *Service) discard(ctx context.Context, log *slog.Logger, clientID, refreshToken, reason string, cause error) error {
	log.ErrorContext(ctx, "identity：換得的 Apple token 不保存，撤銷", "reason", reason, "error", cause)
	if err := m.d.AppleTokens.Revoke(ctx, clientID, refreshToken); err != nil {
		log.ErrorContext(ctx, "identity：撤銷不保存的 Apple token 失敗，使用者需在 Apple 的設定裡移除授權", "reason", "discard_revoke_failed", "error", err)
	}
	return river.JobCancel(cause)
}

// grantBlob 是保存的 job 帶的 Apple token 回應，整包加密。
type grantBlob struct {
	RefreshToken string `json:"refresh_token"`
	IDToken      string `json:"id_token"`
}

// saveAppleTokenMaxAttempts：River 預設的退避下，第 10 次大約在四小時後。
const saveAppleTokenMaxAttempts = 10

// saveAppleTokenArgs 保存已換得的 refresh token。Grant 是加密的 [grantBlob]。Verified 為 true 表示綁定已經驗證過
// （ID token 很快過期，重試時不再驗證）；false 時 job 重新驗證 ID token 與 Subject、ClientID（audience）的綁定。
type saveAppleTokenArgs struct {
	AccountID int64  `json:"account_id"`
	ClientID  string `json:"client_id"`
	Subject   string `json:"subject"`
	Verified  bool   `json:"verified"`
	Grant     []byte `json:"grant"`
}

func (saveAppleTokenArgs) Kind() string { return "identity.save_apple_token" }

type saveAppleTokenWorker struct {
	river.WorkerDefaults[saveAppleTokenArgs]
	m *Service
}

// Work 重新驗證綁定（尚未驗證過時）並保存。身分不符或帳號已刪除就撤銷；其他失敗重試，最後一次失敗以 error 等級記 log
// 並放棄，不撤銷（撤銷會連使用者剛授權的 Apple 授權一起撤掉；下一次原生登入會再交）。
func (w *saveAppleTokenWorker) Work(ctx context.Context, job *river.Job[saveAppleTokenArgs]) error {
	m, args := w.m, job.Args
	log := m.d.Logger.With("account_id", args.AccountID, "client_id", args.ClientID)
	final := job.Attempt >= job.MaxAttempts
	fail := func(reason string, cause error) error {
		if final {
			log.ErrorContext(ctx, "identity：保存 Apple token 失敗，放棄（不撤銷，下一次原生登入會再交）", "reason", reason, "error", cause)
			return river.JobCancel(cause)
		}
		return cause
	}
	if !m.configured() {
		log.ErrorContext(ctx, "identity：保存 Apple token 失敗", "reason", "apple_not_configured")
		return river.JobCancel(errors.New("identity：沒有設定 Apple，無法保存 token"))
	}
	raw, err := m.d.Sealer.Open(args.Grant, grantAAD(args.AccountID))
	var grant grantBlob
	if err == nil {
		err = json.Unmarshal(raw, &grant)
	}
	if err != nil {
		log.ErrorContext(ctx, "identity：保存 Apple token 失敗", "reason", "open_grant", "error", err)
		return river.JobCancel(fmt.Errorf("identity：解開保存的 Apple token：%w", err))
	}
	if !args.Verified {
		switch reason, err := m.bind(ctx, args.Subject, args.ClientID, grant.IDToken); {
		case reason == "binding_unverifiable":
			return fail(reason, err)
		case err != nil:
			return m.discard(ctx, log, args.ClientID, grant.RefreshToken, reason, err)
		}
	}
	sealed, err := m.d.Sealer.Seal([]byte(grant.RefreshToken), tokenAAD(args.AccountID))
	if err != nil {
		return fail("seal_token", err)
	}
	err = m.saver.save(ctx, args.AccountID, args.ClientID, sealed)
	switch {
	case err == nil:
		return nil
	case errors.Is(err, errAccountGone):
		return m.discard(ctx, log, args.ClientID, grant.RefreshToken, "account_deleted", err)
	}
	return fail("persist_failed", err)
}

// revokeAppleTokenMaxAttempts：River 預設的退避下，第 10 次大約在四小時後；都失敗時 job 留在 discarded，要人工處理。
const revokeAppleTokenMaxAttempts = 10

// revokeAppleTokenArgs 撤銷一個 Apple 的 refresh token。Token 是加密過的。
type revokeAppleTokenArgs struct {
	AccountID int64  `json:"account_id"`
	ClientID  string `json:"client_id"`
	Token     []byte `json:"token"`
}

func (revokeAppleTokenArgs) Kind() string { return "identity.revoke_apple_token" }

type revokeAppleTokenWorker struct {
	river.WorkerDefaults[revokeAppleTokenArgs]
	m *Service
}

func (w *revokeAppleTokenWorker) Work(ctx context.Context, job *river.Job[revokeAppleTokenArgs]) error {
	m, args := w.m, job.Args
	log := m.d.Logger.With("account_id", args.AccountID, "client_id", args.ClientID)
	if m.d.AppleTokens == nil || m.d.Sealer == nil {
		log.ErrorContext(ctx, "identity：撤銷 Apple 的授權失敗", "reason", "apple_not_configured")
		return errors.New("identity：沒有設定 Apple，無法撤銷 token")
	}
	token, err := m.d.Sealer.Open(args.Token, tokenAAD(args.AccountID))
	if err != nil {
		log.ErrorContext(ctx, "identity：撤銷 Apple 的授權失敗", "reason", "open_token", "error", err)
		return river.JobCancel(fmt.Errorf("identity：解密 Apple 的 refresh token：%w", err))
	}
	err = m.d.AppleTokens.Revoke(ctx, args.ClientID, string(token))
	if errors.Is(err, model.ErrRejected) {
		// 設定錯誤（client 或金鑰不對）：重試不會成功。
		log.ErrorContext(ctx, "identity：撤銷 Apple 的授權失敗", "reason", "revoke_rejected", "error", err)
		return river.JobCancel(fmt.Errorf("identity：Apple 拒絕撤銷：%w", err))
	}
	if err != nil {
		if job.Attempt >= job.MaxAttempts {
			log.ErrorContext(ctx, "identity：撤銷 Apple 的授權失敗，放棄（使用者需在 Apple 的設定裡移除授權）", "reason", "revoke_unavailable", "error", err)
		}
		return fmt.Errorf("identity：撤銷 Apple 的 token：%w", err)
	}
	log.InfoContext(ctx, "identity：已撤銷 Apple 的授權")
	return nil
}

func (m *Service) AddWorkers(w *river.Workers) {
	river.AddWorker(w, &exchangeAppleCodeWorker{m: m})
	river.AddWorker(w, &saveAppleTokenWorker{m: m})
	river.AddWorker(w, &revokeAppleTokenWorker{m: m})
}
