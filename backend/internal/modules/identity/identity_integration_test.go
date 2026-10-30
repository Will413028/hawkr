//go:build integration

package identity_test

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/riverqueue/river"

	. "github.com/Will413028/hawkr/backend/internal/modules/identity"

	"github.com/Will413028/hawkr/backend/internal/platform/adminperm"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
	"github.com/Will413028/hawkr/backend/internal/platform/db"
	"github.com/Will413028/hawkr/backend/internal/platform/events"
	"github.com/Will413028/hawkr/backend/internal/platform/jobs"
	"github.com/Will413028/hawkr/backend/internal/platform/migrate"
	"github.com/Will413028/hawkr/backend/internal/platform/policy"
	"github.com/Will413028/hawkr/backend/internal/platform/testdb"
)

var pg *testdb.Server

func TestMain(m *testing.M) { testdb.Main(m, &pg) }

// token 是測試的後台 token：沒有簽章，只有 Authenticate 選 application 時讀的 iss 與 fakeAdmin 讀的 sub。
func token(issuer, sub string) string {
	b, _ := json.Marshal(map[string]string{"iss": issuer, "sub": sub})
	return "e30." + base64.RawURLEncoding.EncodeToString(b) + ".sig"
}

type fakeAdmin struct {
	issuer string
	users  map[string]AdminIdentity
}

func (a *fakeAdmin) Issuer() string { return a.issuer }

func (a *fakeAdmin) Verify(_ context.Context, tok string) (AdminIdentity, error) {
	var claims struct{ Iss, Sub string }
	parts := strings.Split(tok, ".")
	if len(parts) != 3 {
		return AdminIdentity{}, ErrInvalidToken
	}
	raw, _ := base64.RawURLEncoding.DecodeString(parts[1])
	if json.Unmarshal(raw, &claims) != nil || claims.Iss != a.issuer || claims.Sub == "bad" {
		return AdminIdentity{}, ErrInvalidToken
	}
	u, ok := a.users[claims.Sub]
	if !ok {
		return AdminIdentity{}, ErrInvalidToken
	}
	u.Subject = claims.Sub
	return u, nil
}

// fakeVerifier 以 token 查表回傳 Apple 的身分；"bad" 是無效的 token，"down" 表示 Apple 沒有回應。
type fakeVerifier struct{ ids map[string]Identity }

func (v *fakeVerifier) Verify(_ context.Context, tok string) (Identity, error) {
	switch tok {
	case "bad":
		return Identity{}, errors.Join(errors.New("簽章不符"), ErrInvalidToken)
	case "down":
		return Identity{}, errors.New("抓不到 JWKS")
	}
	id, ok := v.ids[tok]
	if !ok {
		return Identity{}, ErrInvalidToken
	}
	return id, nil
}

type revokeCall struct{ clientID, token string }

// fakeAppleTokens 以 code 換出 "rt-"+code；code 是 "fail" 時 Apple 拒絕（過期或已用過）；revokeErr 是撤銷的結果。
type fakeAppleTokens struct {
	mu        sync.Mutex
	exchanged []string
	revoked   []revokeCall
	revokeErr error
}

func (a *fakeAppleTokens) Exchange(_ context.Context, _, code string) (AppleGrant, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.exchanged = append(a.exchanged, code)
	if code == "fail" {
		return AppleGrant{}, errors.Join(errors.New("invalid_grant"), ErrRejected)
	}
	return AppleGrant{RefreshToken: "rt-" + code, IDToken: "id-" + code}, nil
}

func (a *fakeAppleTokens) Revoke(_ context.Context, clientID, tok string) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.revokeErr != nil {
		return a.revokeErr
	}
	a.revoked = append(a.revoked, revokeCall{clientID, tok})
	return nil
}

// fakeSealer 可逆地「加密」：前面加上標記，讓測試看得出排入 job 的不是明文。
type fakeSealer struct{}

func (fakeSealer) Seal(p, aad []byte) ([]byte, error) {
	return append([]byte("sealed:"+string(aad)+":"), p...), nil
}

func (fakeSealer) Open(c, aad []byte) ([]byte, error) {
	prefix := []byte("sealed:" + string(aad) + ":")
	if !bytes.HasPrefix(c, prefix) {
		return nil, errors.New("不是密文或脈絡不符")
	}
	return bytes.TrimPrefix(c, prefix), nil
}

// Has：fakeSealer 認得金鑰 "k1"（fakeSealer 的密文不帶金鑰 ID，啟動檢查只看資料庫參照的 ID）。
func (fakeSealer) Has(id string) bool { return id == "k1" }

const adminIssuer = "https://team.cloudflareaccess.test"

type fixture struct {
	m           *Module
	deps        Deps
	db          *db.DB
	now         time.Time
	emails      map[string]string // f.account 建帳時用的 email，鍵是使用者（登入身分的 subject）
	admins      *fakeAdmin
	apple       *fakeVerifier
	appleTokens *fakeAppleTokens
	deleted     []AccountDeleted
	createdMu   sync.Mutex
	sessionSeq  atomic.Int64
	created     []AccountCreated // 已 commit 與回滾的建帳事件都會記下；只在建帳成功時斷言
}

// setup 起一個只套用了本模組 migration 的空 database：integration 測試只跑在自己的 schema 上。
func setup(t *testing.T) *fixture {
	t.Helper()
	d := pg.NewDB(t, testdb.Jobs(), testdb.Migrations(migrate.Module{Name: "identity", Dir: Migrations()}))
	jc, err := jobs.NewInsertOnly(d)
	if err != nil {
		t.Fatal(err)
	}
	f := &fixture{
		db:          d,
		now:         time.Date(2026, 9, 27, 12, 0, 0, 0, time.UTC),
		emails:      map[string]string{},
		admins:      &fakeAdmin{issuer: adminIssuer, users: map[string]AdminIdentity{}},
		apple:       &fakeVerifier{ids: map[string]Identity{}},
		appleTokens: &fakeAppleTokens{},
	}
	bus := events.NewBus()
	events.Subscribe(bus, func(_ context.Context, e AccountDeleted) error {
		f.deleted = append(f.deleted, e)
		return nil
	})
	events.Subscribe(bus, func(_ context.Context, e AccountCreated) error {
		f.createdMu.Lock()
		defer f.createdMu.Unlock()
		f.created = append(f.created, e)
		return nil
	})
	f.deps = Deps{
		DB:     d,
		Events: bus,
		Jobs:   jc,
		Providers: Providers{
			Admin: f.admins,
			Apple: f.apple, AppleTokens: f.appleTokens, Sealer: fakeSealer{},
		},
		Now:    func() time.Time { return f.now },
		Logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
	}
	f.m = New(f.deps)
	return f
}

func wantKind(t *testing.T, err error, want apperr.Kind) {
	t.Helper()
	if got := apperr.KindOf(err); got != want || err == nil {
		t.Fatalf("err = %v（分類 %v），預期分類 %v", err, got, want)
	}
}

func wantReason(t *testing.T, err error, want apperr.Reason) {
	t.Helper()
	var ae *apperr.Error
	if !errors.As(err, &ae) || ae.Reason != want {
		t.Fatalf("err = %v，預期原因 %s", err, want)
	}
}

func ptr[T any](v T) *T { return &v }

// account 直接寫入一個帳號與它的登入身分（GOOGLE，subject 是 user；email 取自 f.emails）。買家不是角色（W5）：
// 建帳之後沒有任何角色。
func (f *fixture) account(t *testing.T, user string) int64 {
	t.Helper()
	var email *string
	if e := f.emails[user]; e != "" {
		email = &e
	}
	var id int64
	if err := f.db.Pool().QueryRow(context.Background(),
		`INSERT INTO identity.accounts (email, created_at, updated_at) VALUES ($1, $2, $2) RETURNING id`, email, f.now).Scan(&id); err != nil {
		t.Fatal(err)
	}
	if _, err := f.db.Pool().Exec(context.Background(),
		`INSERT INTO identity.account_identities (provider, subject, account_id, created_at, updated_at) VALUES ('GOOGLE', $1, $2, $3, $3)`,
		user, id, f.now); err != nil {
		t.Fatal(err)
	}
	return id
}

// sessionToken 為帳號直接寫入一個 App 種類的 session（時間取 f.now）並回傳 token，不經 Login。
func (f *fixture) sessionToken(t *testing.T, accountID int64) string {
	t.Helper()
	tok := fmt.Sprintf("%043d", f.sessionSeq.Add(1))
	hash := sha256.Sum256([]byte(tok))
	if _, err := f.db.Pool().Exec(context.Background(),
		`INSERT INTO identity.sessions (token_hash, account_id, client, created_at, last_used_at, absolute_expires_at, auth_time)
		 VALUES ($1, $2, 'APP', $3, $3, $3::timestamptz + interval '90 days', $3)`, hash[:], accountID, f.now); err != nil {
		t.Fatal(err)
	}
	return tok
}

// authenticate 認證登入身分 user 的帳號（沒有時建立）：為它寫一個 session 再認證，等於這個帳號帶著 session 打進來。
func (f *fixture) authenticate(t *testing.T, user string) (Authenticated, error) {
	t.Helper()
	var id int64
	err := f.db.Pool().QueryRow(context.Background(), `SELECT account_id FROM identity.account_identities WHERE provider = 'GOOGLE' AND subject = $1`, user).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		id = f.account(t, user)
	} else if err != nil {
		t.Fatal(err)
	}
	return f.m.Authenticate(context.Background(), f.sessionToken(t, id))
}

func (f *fixture) count(t *testing.T, query string, args ...any) int {
	t.Helper()
	var n int
	if err := f.db.Pool().QueryRow(context.Background(), query, args...).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

func (f *fixture) jobKinds(t *testing.T) []string {
	t.Helper()
	rows, err := f.db.Pool().Query(context.Background(), `SELECT kind FROM river.river_job ORDER BY id`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var k string
		if err := rows.Scan(&k); err != nil {
			t.Fatal(err)
		}
		out = append(out, k)
	}
	return out
}

// lastJobArgs 讀最新一個 kind 的 job 的參數。
func lastJobArgs[T any](t *testing.T, f *fixture, kind string) T {
	t.Helper()
	var raw []byte
	if err := f.db.Pool().QueryRow(context.Background(),
		`SELECT args FROM river.river_job WHERE kind = $1 ORDER BY id DESC LIMIT 1`, kind).Scan(&raw); err != nil {
		t.Fatalf("沒有排入 %s：%v", kind, err)
	}
	var args T
	if err := json.Unmarshal(raw, &args); err != nil {
		t.Fatal(err)
	}
	return args
}

type exchangeAppleCodeJobArgs struct {
	AccountID int64  `json:"account_id"`
	ClientID  string `json:"client_id"`
	Subject   string `json:"subject"`
	Code      []byte `json:"code"`
}

type revokeAppleTokenJobArgs struct {
	AccountID int64  `json:"account_id"`
	ClientID  string `json:"client_id"`
	Token     []byte `json:"token"`
}

// runWorkers 起真的 River worker，直到 ctx 結束；done 在每輪輪詢到條件成立時回 true。
func (f *fixture) runWorkers(t *testing.T, done func() bool) {
	t.Helper()
	workers := river.NewWorkers()
	f.m.AddWorkers(workers)
	c, err := jobs.NewWorker(f.db, workers, map[string]int{river.QueueDefault: 2}, nil)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	stopped := make(chan error, 1)
	go func() { stopped <- c.Run(ctx) }()
	defer func() {
		cancel()
		if err := <-stopped; err != nil {
			t.Errorf("worker 停止：%v", err)
		}
	}()
	deadline := time.Now().Add(30 * time.Second)
	for !done() {
		if time.Now().After(deadline) {
			t.Fatal("job 沒有在時限內完成")
		}
		time.Sleep(50 * time.Millisecond)
	}
}

func (f *fixture) storedAppleToken(t *testing.T, id int64) (clientID string, token []byte, ok bool) {
	t.Helper()
	err := f.db.Pool().QueryRow(context.Background(),
		`SELECT client_id, token FROM identity.apple_tokens WHERE account_id = $1`, id).Scan(&clientID, &token)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", nil, false
	}
	if err != nil {
		t.Fatal(err)
	}
	return clientID, token, true
}

func (f *fixture) appleCalls() (exchanged []string, revoked []revokeCall) {
	f.appleTokens.mu.Lock()
	defer f.appleTokens.mu.Unlock()
	return slices.Clone(f.appleTokens.exchanged), slices.Clone(f.appleTokens.revoked)
}

func TestAuthenticationErrors(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	for name, tok := range map[string]string{
		"沒有 token":      "",
		"不是 session":    "opaque",
		"長度對但不存在":       strings.Repeat("A", 43),
		"JWT 形式的 token": token("https://customers.example.test", "user_1"),
		"後台的無效 token":   token(adminIssuer, "bad"),
	} {
		_, err := f.m.Authenticate(ctx, tok)
		if apperr.KindOf(err) != apperr.KindUnauthenticated {
			t.Errorf("%s：err = %v，預期 unauthenticated", name, err)
		}
	}
	if n := f.count(t, `SELECT count(*) FROM identity.accounts`); n != 0 {
		t.Fatalf("失敗時建了 %d 個帳號", n)
	}
}

func TestSuspendedAccountStillAuthenticatesAndCarriesTheSuspensionTime(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	id := f.account(t, "user_1")
	if a, err := f.authenticate(t, "user_1"); err != nil || a.SuspendedAt != nil {
		t.Fatalf("未停權的帳號 = %+v, %v", a, err)
	}
	for range 2 { // 冪等：已經停權時不再寫第二筆
		if _, _, err := f.m.SuspendAccount(ctx, SuspendInput{AccountID: id, Category: policy.Other, Reason: "檢舉成立"}); err != nil {
			t.Fatal(err)
		}
	}
	a, err := f.authenticate(t, "user_1")
	if err != nil || a.Principal.ID != id || a.Principal.Kind != PrincipalAccount || a.SuspendedAt == nil || !a.SuspendedAt.Equal(f.now) {
		t.Fatalf("停權的帳號 = %+v, %v，預期認證成功並帶停權時間 %v", a, err, f.now)
	}
	if kinds := f.jobKinds(t); len(kinds) != 0 {
		t.Fatalf("停權排了 job：%v，停權不排任何 job", kinds)
	}
	if _, released, err := f.m.UnsuspendAccount(ctx, id, "申覆成立"); err != nil || !released {
		t.Fatalf("UnsuspendAccount = %v, %v", released, err)
	}
	if a, err := f.authenticate(t, "user_1"); err != nil || a.SuspendedAt != nil {
		t.Fatalf("解除後 = %+v, %v，預期停權時間清除", a, err)
	}
}

func TestDeletionLeavesATombstoneAndKeepsTheLoginIdentity(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	f.emails["user_1"] = "alice@example.com"
	id := f.account(t, "user_1")
	for range 2 { // 冪等
		if err := f.m.DeleteAccount(ctx, id); err != nil {
			t.Fatal(err)
		}
	}
	if len(f.deleted) != 1 || f.deleted[0].AccountID != id {
		t.Fatalf("AccountDeleted = %+v", f.deleted)
	}
	acc, err := f.m.Account(ctx, id)
	if err != nil || !acc.Deleted || acc.Email != "" || len(acc.Roles) != 0 {
		t.Fatalf("墓碑 = %+v, %v", acc, err)
	}
	// 同一個登入身分不會再建一個帳號：身分列留作墓碑。
	_, err = f.authenticate(t, "user_1")
	wantKind(t, err, apperr.KindUnauthenticated)
	if n := f.count(t, `SELECT count(*) FROM identity.accounts`); n != 1 {
		t.Fatalf("帳號數 = %d", n)
	}
	if n := f.count(t, `SELECT count(*) FROM identity.account_identities WHERE provider = 'GOOGLE' AND subject = 'user_1' AND account_id = $1`, id); n != 1 {
		t.Fatalf("登入身分的墓碑 = %d 列", n)
	}
	if kinds := f.jobKinds(t); len(kinds) != 0 {
		t.Fatalf("刪除排了 job：%v", kinds)
	}
	wantKind(t, f.m.DeleteAccount(ctx, 999), apperr.KindNotFound)
}

func TestRemoveRole(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	id := f.account(t, "user_1")

	// #3189：賣家重新入駐（來源 2）之後，遲到的舊撤銷（來源 1）不生效。
	if err := f.m.GrantRole(ctx, GrantRoleInput{AccountID: id, Role: RoleSeller, SourceID: ptr[int64](1)}); err != nil {
		t.Fatal(err)
	}
	if err := f.m.GrantRole(ctx, GrantRoleInput{AccountID: id, Role: RoleSeller, SourceID: ptr[int64](2)}); err != nil {
		t.Fatal(err)
	}
	if err := f.m.RemoveRole(ctx, RemoveRoleInput{AccountID: id, Role: RoleSeller, SourceID: ptr[int64](1)}); err != nil {
		t.Fatal(err)
	}
	a, _ := f.m.Account(ctx, id)
	if !slices.Equal(a.Roles, []Role{RoleSeller}) {
		t.Fatalf("柵欄沒擋下：角色 = %v", a.Roles)
	}
	// 角色的變更下一個請求就生效：每個請求都讀。
	if got, err := f.authenticate(t, "user_1"); err != nil || !slices.Equal(got.Principal.Roles, []Role{RoleSeller}) {
		t.Fatalf("認證的角色 = %+v, %v", got, err)
	}
	// 移除最後一個角色不刪帳號（W5：帳號活著就是買家，刪帳由 accountdeletion 編排），重送也一樣。
	for range 2 {
		if err := f.m.RemoveRole(ctx, RemoveRoleInput{AccountID: id, Role: RoleSeller, SourceID: ptr[int64](2)}); err != nil {
			t.Fatal(err)
		}
	}
	if got, err := f.authenticate(t, "user_1"); err != nil || len(got.Principal.Roles) != 0 {
		t.Fatalf("認證的角色 = %+v, %v", got, err)
	}
	if a, _ := f.m.Account(ctx, id); a.Deleted || len(a.Roles) != 0 || len(f.deleted) != 0 {
		t.Fatalf("帳號 = %+v，AccountDeleted %d 次", a, len(f.deleted))
	}
	// 買家不是角色：舊的 BUYER 值授予或移除都回 InvalidInput，不碰資料庫。
	wantKind(t, f.m.GrantRole(ctx, GrantRoleInput{AccountID: id, Role: "BUYER"}), apperr.KindInvalidInput)
	wantKind(t, f.m.RemoveRole(ctx, RemoveRoleInput{AccountID: id, Role: "BUYER"}), apperr.KindInvalidInput)
	// 已刪除的帳號：移除角色成功、不做任何事；授予回 NotFound。從來不存在的帳號回 NotFound。
	if err := f.m.DeleteAccount(ctx, id); err != nil {
		t.Fatal(err)
	}
	if err := f.m.RemoveRole(ctx, RemoveRoleInput{AccountID: id, Role: RoleSeller}); err != nil {
		t.Fatal(err)
	}
	if len(f.deleted) != 1 {
		t.Fatalf("AccountDeleted 發了 %d 次", len(f.deleted))
	}
	wantKind(t, f.m.RemoveRole(ctx, RemoveRoleInput{AccountID: 999, Role: RoleSeller}), apperr.KindNotFound)
	wantKind(t, f.m.GrantRole(ctx, GrantRoleInput{AccountID: id, Role: RoleSeller, SourceID: nil}), apperr.KindNotFound)
}

func TestAdminsAreCreatedForAccessSubjects(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	f.admins.users["user_ops"] = AdminIdentity{Email: "ops@gmail.com"}
	a, err := f.m.Authenticate(ctx, token(adminIssuer, "user_ops"))
	if err != nil || a.Principal.Kind != PrincipalAdmin || len(f.created) != 0 {
		t.Fatalf("= %+v, %v", a, err)
	}
	got, err := f.m.Admin(ctx, a.Principal.ID)
	want := Admin{ID: a.Principal.ID, Email: "ops@gmail.com"}
	if err != nil || got != want {
		t.Fatalf("管理員 = %+v, %v", got, err)
	}
	if again, err := f.m.Authenticate(ctx, token(adminIssuer, "user_ops")); err != nil || again.Principal.ID != a.Principal.ID {
		t.Fatalf("第二次 = %+v, %v", again, err)
	}
	if n := f.count(t, `SELECT count(*) FROM identity.admins`); n != 1 {
		t.Fatalf("管理員數 = %d", n)
	}
	// 後台的使用者與顧客的帳號各自發號：同號時不互相認證成對方。
	id := f.account(t, "user_ops")
	if id != a.Principal.ID {
		t.Fatalf("前提：兩邊都是第一筆（帳號 %d、管理員 %d）", id, a.Principal.ID)
	}
	if got, err := f.authenticate(t, "user_ops"); err != nil || got.Principal.Kind != PrincipalAccount {
		t.Fatalf("顧客的 token = %+v, %v", got, err)
	}
	_, err = f.m.Admin(ctx, 999)
	wantKind(t, err, apperr.KindNotFound)
}

// sealedToken 是 fakeSealer 對帳號保存的 token 的密文（內容與 AAD 都看得出來）。
func sealedToken(accountID int64, plain string) string {
	return "sealed:identity.apple_tokens|token|" + strconv.FormatInt(accountID, 10) + ":" + plain
}

func TestLinkAppleCredentialVerifiesTheIDToken(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	id := f.account(t, "user_1")
	f.apple.ids["a1"] = Identity{Subject: "apple-1", Audience: "io.github.will413028.hawkr.buyer"}
	link := func(tok, code string) error {
		return f.m.LinkAppleCredential(ctx, LinkAppleCredentialInput{AccountID: id, IDToken: tok, AuthorizationCode: code})
	}

	wantKind(t, link("bad", "code-1"), apperr.KindInvalidInput)
	wantKind(t, link("a1", ""), apperr.KindInvalidInput)
	wantKind(t, link("down", "code-1"), apperr.KindUnavailable)
	var queued int
	if err := f.db.Pool().QueryRow(ctx, `SELECT count(*) FROM river.river_job WHERE kind = 'identity.exchange_apple_code'`).Scan(&queued); err != nil || queued != 0 {
		t.Fatalf("被拒絕的請求排入了 %d 個 job（%v）", queued, err)
	}

	if err := link("a1", "code-1"); err != nil {
		t.Fatal(err)
	}
	if exchanged, _ := f.appleCalls(); len(exchanged) != 0 {
		t.Fatal("請求內不該呼叫 Apple")
	}
	args := lastJobArgs[exchangeAppleCodeJobArgs](t, f, "identity.exchange_apple_code")
	wantCode := "sealed:identity.exchange_apple_code|code|" + strconv.FormatInt(id, 10) + ":code-1"
	if args.AccountID != id || args.ClientID != "io.github.will413028.hawkr.buyer" || args.Subject != "apple-1" || string(args.Code) != wantCode {
		t.Fatalf("job 的參數 = %+v", args)
	}
	f.deps.Providers.AppleTokens = nil
	f.m = New(f.deps)
	wantKind(t, link("a1", "code-1"), apperr.KindUnavailable)
}

// 連結、換 token、保存（加密、取代舊的）、最終刪除時撤銷並刪列的完整流程。
func TestAppleTokenIsStoredReplacedAndRevokedOnDeletion(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	id := f.account(t, "user_1")
	f.apple.ids["a1"] = Identity{Subject: "apple-1", Audience: "io.github.will413028.hawkr.buyer"}
	f.apple.ids["id-code-1"] = f.apple.ids["a1"]
	f.apple.ids["id-code-2"] = f.apple.ids["a1"]
	link := func(code string) {
		t.Helper()
		if err := f.m.LinkAppleCredential(ctx, LinkAppleCredentialInput{AccountID: id, IDToken: "a1", AuthorizationCode: code}); err != nil {
			t.Fatal(err)
		}
	}
	storedIs := func(want string) func() bool {
		return func() bool {
			_, tok, ok := f.storedAppleToken(t, id)
			return ok && string(tok) == want
		}
	}

	link("code-1")
	f.runWorkers(t, storedIs(sealedToken(id, "rt-code-1")))
	if cid, _, _ := f.storedAppleToken(t, id); cid != "io.github.will413028.hawkr.buyer" {
		t.Fatalf("client_id = %q", cid)
	}
	link("code-2") // 重新連結取代
	f.runWorkers(t, storedIs(sealedToken(id, "rt-code-2")))
	var rows int
	if err := f.db.Pool().QueryRow(ctx, `SELECT count(*) FROM identity.apple_tokens`).Scan(&rows); err != nil || rows != 1 {
		t.Fatalf("保存了 %d 筆（%v）", rows, err)
	}
	if _, revoked := f.appleCalls(); len(revoked) != 0 {
		t.Fatalf("刪除之前不該撤銷：%+v", revoked)
	}

	// 刪除帳號：同一個 transaction 內刪列並排入撤銷；transaction 回滾時兩者都不發生。
	rollback := errors.New("rollback")
	err := f.db.InTx(ctx, func(ctx context.Context) error {
		if err := f.m.DeleteAccount(ctx, id); err != nil {
			return err
		}
		return rollback
	})
	if !errors.Is(err, rollback) {
		t.Fatal(err)
	}
	if _, _, ok := f.storedAppleToken(t, id); !ok {
		t.Fatal("回滾之後 token 應該還在")
	}
	if err := f.db.Pool().QueryRow(ctx, `SELECT count(*) FROM river.river_job WHERE kind = 'identity.revoke_apple_token'`).Scan(&rows); err != nil || rows != 0 {
		t.Fatalf("回滾之後排入了 %d 個撤銷 job（%v）", rows, err)
	}
	if err := f.m.DeleteAccount(ctx, id); err != nil {
		t.Fatal(err)
	}
	if _, _, ok := f.storedAppleToken(t, id); ok {
		t.Fatal("刪除之後不該留著 token")
	}
	queued := lastJobArgs[revokeAppleTokenJobArgs](t, f, "identity.revoke_apple_token")
	if queued.AccountID != id || queued.ClientID != "io.github.will413028.hawkr.buyer" || string(queued.Token) != sealedToken(id, "rt-code-2") {
		t.Fatalf("撤銷 job = %+v", queued)
	}
	f.runWorkers(t, func() bool { _, r := f.appleCalls(); return len(r) == 1 })
	if _, r := f.appleCalls(); r[0] != (revokeCall{"io.github.will413028.hawkr.buyer", "rt-code-2"}) {
		t.Fatalf("撤銷 = %+v", r)
	}
	// 再刪一次（冪等）不會再撤銷。
	if err := f.m.DeleteAccount(ctx, id); err != nil {
		t.Fatal(err)
	}
	if err := f.db.Pool().QueryRow(ctx, `SELECT count(*) FROM river.river_job WHERE kind = 'identity.revoke_apple_token'`).Scan(&rows); err != nil || rows != 1 {
		t.Fatalf("撤銷 job 共 %d 個（%v）", rows, err)
	}
}

// A 的 identity token 搭配 B 的 code：Apple 換 token 簽發的是 B 的 ID token，不保存，換得的 token 當場撤銷。
func TestExchangeWithSomeoneElsesCodeIsNotStored(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	id := f.account(t, "user_1")
	f.apple.ids["a1"] = Identity{Subject: "apple-A", Audience: "io.github.will413028.hawkr.buyer"}
	f.apple.ids["id-code-B"] = Identity{Subject: "apple-B", Audience: "io.github.will413028.hawkr.buyer"}
	if err := f.m.LinkAppleCredential(ctx, LinkAppleCredentialInput{AccountID: id, IDToken: "a1", AuthorizationCode: "code-B"}); err != nil {
		t.Fatal(err)
	}
	f.runWorkers(t, func() bool { _, r := f.appleCalls(); return len(r) == 1 })
	if _, _, ok := f.storedAppleToken(t, id); ok {
		t.Fatal("別人的授權不該存在這個帳號上")
	}
	if _, r := f.appleCalls(); r[0] != (revokeCall{"io.github.will413028.hawkr.buyer", "rt-code-B"}) {
		t.Fatalf("撤銷 = %+v", r)
	}
}

func TestDeletingAnAccountWithoutAppleCredentialEnqueuesNoRevocation(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	id := f.account(t, "user_1")
	if err := f.m.DeleteAccount(ctx, id); err != nil {
		t.Fatal(err)
	}
	var n int
	if err := f.db.Pool().QueryRow(ctx, `SELECT count(*) FROM river.river_job WHERE kind = 'identity.revoke_apple_token'`).Scan(&n); err != nil || n != 0 {
		t.Fatalf("撤銷 job %d 個（%v）", n, err)
	}
}

// 換 token 的 job 執行時帳號已經刪除：不保存，換得的 token 當場撤銷。
func TestExchangeAfterDeletionRevokesInsteadOfStoring(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	id := f.account(t, "user_1")
	f.apple.ids["a1"] = Identity{Subject: "apple-1", Audience: "io.github.will413028.hawkr.buyer"}
	f.apple.ids["id-code-1"] = f.apple.ids["a1"]
	if err := f.m.LinkAppleCredential(ctx, LinkAppleCredentialInput{AccountID: id, IDToken: "a1", AuthorizationCode: "code-1"}); err != nil {
		t.Fatal(err)
	}
	if err := f.m.DeleteAccount(ctx, id); err != nil {
		t.Fatal(err)
	}
	f.runWorkers(t, func() bool { _, r := f.appleCalls(); return len(r) == 1 })
	if _, _, ok := f.storedAppleToken(t, id); ok {
		t.Fatal("已刪除的帳號不該保存 token")
	}
}

func TestCheckAppleKeysRequiresEveryReferencedKey(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	id := f.account(t, "user_1")
	if err := f.m.CheckAppleKeys(ctx); err != nil {
		t.Fatalf("沒有保存的 token：%v", err)
	}
	insert := func(keyID string) {
		t.Helper()
		// 密文開頭：1 byte 的金鑰 ID 長度、金鑰 ID。
		if _, err := f.db.Pool().Exec(ctx, `INSERT INTO identity.apple_tokens (account_id, client_id, token) VALUES ($1, 'c', $2)
			ON CONFLICT (account_id) DO UPDATE SET token = EXCLUDED.token`, id, append([]byte{byte(len(keyID))}, keyID+"rest"...)); err != nil {
			t.Fatal(err)
		}
	}
	insert("k1")
	if err := f.m.CheckAppleKeys(ctx); err != nil {
		t.Fatalf("金鑰在金鑰環裡：%v", err)
	}
	insert("old")
	if err := f.m.CheckAppleKeys(ctx); err == nil || !strings.Contains(err.Error(), "old") {
		t.Fatalf("金鑰被拿掉要擋下：%v", err)
	}
	insert("k1")

	// 排隊或重試中的 job 參數裡的密文也要檢查；完成的 job 不管。
	job := func(kind, field, keyID, state string) {
		t.Helper()
		sealed := append([]byte{byte(len(keyID))}, keyID+"rest"...)
		args, _ := json.Marshal(map[string]any{field: sealed})
		if _, err := f.db.Pool().Exec(ctx, `INSERT INTO river.river_job (kind, args, state, max_attempts, finalized_at) VALUES ($1, $2, $3::text::river.river_job_state, 5, CASE WHEN $3::text = 'completed' THEN now() END)`, kind, args, state); err != nil {
			t.Fatal(err)
		}
	}
	job("identity.revoke_apple_token", "token", "k1", "available")
	job("identity.exchange_apple_code", "code", "k1", "retryable")
	job("identity.save_apple_token", "grant", "k1", "scheduled")
	job("identity.revoke_apple_token", "token", "gone", "completed")
	if err := f.m.CheckAppleKeys(ctx); err != nil {
		t.Fatalf("job 參數的金鑰都在，或 job 已完成：%v", err)
	}
	for _, c := range []struct{ kind, field string }{
		{"identity.exchange_apple_code", "code"}, {"identity.save_apple_token", "grant"}, {"identity.revoke_apple_token", "token"},
	} {
		job(c.kind, c.field, "retired-"+c.field, "retryable")
		if err := f.m.CheckAppleKeys(ctx); err == nil || !strings.Contains(err.Error(), "retired-"+c.field) {
			t.Fatalf("%s 的 job 參數用到被拿掉的金鑰要擋下：%v", c.kind, err)
		}
		if _, err := f.db.Pool().Exec(ctx, `DELETE FROM river.river_job WHERE args->>$1 IS NOT NULL AND kind = $2 AND state = 'retryable' AND convert_from(substring(decode(args->>$1,'base64') FROM 2 FOR get_byte(decode(args->>$1,'base64'),0)),'UTF8') = $3`, c.field, c.kind, "retired-"+c.field); err != nil {
			t.Fatal(err)
		}
	}
}

func TestCodeValueChecksMatchTheGoTypes(t *testing.T) {
	t.Parallel()
	f := setup(t)
	quoted := regexp.MustCompile(`'([^']+)'`)
	cases := []struct {
		constraint string
		want       []string
	}{
		{"account_roles_role_check", []string{string(RoleSeller)}},
		{"account_suspensions_kind_check", []string{string(SuspensionPermanent)}},
		{"account_suspensions_reason_category_check", categoryValues()},
		{"account_identities_provider_check", providerValues()},
		{"login_flows_provider_check", providerValues()},
		{"sessions_client_check", clientValues()},
		{"admin_roles_role_check", adminRoleValues()},
		{"admin_role_events_role_check", adminRoleValues()},
		{"admin_role_events_action_check", []string{string(AdminRoleActionGrant), string(AdminRoleActionRevoke)}},
		{"admin_role_events_source_check", []string{string(AdminRoleSourceCLI)}},
	}
	if len(cases[0].want) != len(Roles()) {
		t.Fatal("Roles() 與測試的值域不一致")
	}
	for _, c := range cases {
		var def string
		if err := f.db.Pool().QueryRow(context.Background(),
			`SELECT pg_get_constraintdef(oid) FROM pg_constraint WHERE conname = $1`, c.constraint).Scan(&def); err != nil {
			t.Fatalf("%s：%v", c.constraint, err)
		}
		var got []string
		for _, m := range quoted.FindAllStringSubmatch(def, -1) {
			got = append(got, m[1])
		}
		slices.Sort(got)
		slices.Sort(c.want)
		if !slices.Equal(got, c.want) {
			t.Errorf("%s 允許 %v，Go 的值域是 %v（%s）", c.constraint, got, c.want, strings.TrimSpace(def))
		}
	}
}

func categoryValues() []string {
	var out []string
	for _, c := range policy.Values() {
		out = append(out, string(c))
	}
	return out
}

func providerValues() []string {
	var out []string
	for _, p := range LoginProviders() {
		out = append(out, string(p))
	}
	return out
}

func clientValues() []string {
	var out []string
	for _, c := range ClientKinds() {
		out = append(out, string(c))
	}
	return out
}

func adminRoleValues() []string {
	var out []string
	for _, r := range adminperm.Roles() {
		out = append(out, string(r))
	}
	return out
}

func TestListAccounts(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	var ids []int64
	for i, email := range []string{"a_1@example.com", "b@example.com", "c@example.com"} {
		user := "user_" + strconv.Itoa(i)
		f.emails[user] = email
		ids = append(ids, f.account(t, user))
		f.now = f.now.Add(time.Minute)
	}
	if err := f.m.GrantRole(ctx, GrantRoleInput{AccountID: ids[1], Role: RoleSeller, SourceID: ptr[int64](5)}); err != nil {
		t.Fatal(err)
	}
	if err := f.m.DeleteAccount(ctx, ids[2]); err != nil {
		t.Fatal(err)
	}

	page, err := f.m.ListAccounts(ctx, ListAccountsInput{Filter: AccountFilter{}, After: nil, Size: 1})
	if err != nil || len(page.Accounts) != 1 || page.Accounts[0].ID != ids[1] || page.Next == nil || page.Total != 2 {
		t.Fatalf("第一頁 = %+v, %v（刪除的帳號不列，新到舊）", page, err)
	}
	page, err = f.m.ListAccounts(ctx, ListAccountsInput{Filter: AccountFilter{}, After: page.Next, Size: 1})
	if err != nil || len(page.Accounts) != 1 || page.Accounts[0].ID != ids[0] || page.Next != nil {
		t.Fatalf("第二頁 = %+v, %v", page, err)
	}
	page, err = f.m.ListAccounts(ctx, ListAccountsInput{Filter: AccountFilter{Role: RoleSeller}, After: nil, Size: 0})
	if err != nil || len(page.Accounts) != 1 || page.Accounts[0].ID != ids[1] || page.Total != 1 ||
		!slices.Equal(page.Accounts[0].Roles, []Role{RoleSeller}) {
		t.Fatalf("賣家 = %+v, %v", page, err)
	}
	// 使用者輸入的 _ 是字元本身，不是萬用字元：a_1 只找到 a_1，找不到其他。
	page, err = f.m.ListAccounts(ctx, ListAccountsInput{Filter: AccountFilter{Query: "A_1"}, After: nil, Size: 0})
	if err != nil || len(page.Accounts) != 1 || page.Accounts[0].ID != ids[0] {
		t.Fatalf("搜尋 email = %+v, %v", page, err)
	}
	page, err = f.m.ListAccounts(ctx, ListAccountsInput{Filter: AccountFilter{Query: "_"}, After: nil, Size: 0})
	if err != nil || len(page.Accounts) != 1 {
		t.Fatalf("搜尋 _ = %+v, %v", page, err)
	}
	_, err = f.m.ListAccounts(ctx, ListAccountsInput{Filter: AccountFilter{}, After: nil, Size: -1})
	wantKind(t, err, apperr.KindInvalidInput)
}

// email 篩選是完全相符（不分大小寫、去前後空白），不是包含：a_1@example.com 不會被 a_1@example.co 或 "a_1" 命中。
func TestListAccountsByExactEmail(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	var ids []int64
	for i, email := range []string{"Shop@Example.com", "myshop@example.com", "x_y@example.com", "xzy@example.com"} {
		user := "user_" + strconv.Itoa(i)
		f.emails[user] = email
		ids = append(ids, f.account(t, user))
		f.now = f.now.Add(time.Minute)
	}
	list := func(email string) AccountPage {
		t.Helper()
		page, err := f.m.ListAccounts(ctx, ListAccountsInput{Filter: AccountFilter{Email: email}, After: nil, Size: 0})
		if err != nil {
			t.Fatal(err)
		}
		return page
	}
	// 大小寫與前後空白不影響；"myshop@example.com" 包含 "shop@example.com" 但不是完全相符，不得命中。
	if page := list("  shop@EXAMPLE.com "); len(page.Accounts) != 1 || page.Accounts[0].ID != ids[0] || page.Total != 1 {
		t.Fatalf("完全相符 = %+v", page)
	}
	// _ 是字元本身，不是萬用字元。
	if page := list("x_y@example.com"); len(page.Accounts) != 1 || page.Accounts[0].ID != ids[2] || page.Total != 1 {
		t.Fatalf("含底線 = %+v", page)
	}
	for _, partial := range []string{"shop@example", "hop@example.com", "example.com", "nobody@example.com"} {
		if page := list(partial); len(page.Accounts) != 0 || page.Total != 0 {
			t.Fatalf("%q 不是完全相符，卻命中 %+v", partial, page)
		}
	}
	// 空字串不篩選。
	if page := list(""); page.Total != 4 {
		t.Fatalf("空 email 應列出全部 = %+v", page)
	}
	// 和其他篩選取交集。
	page, err := f.m.ListAccounts(ctx, ListAccountsInput{Filter: AccountFilter{Email: "shop@example.com", Role: RoleSeller}, After: nil, Size: 0})
	if err != nil || len(page.Accounts) != 0 {
		t.Fatalf("沒有賣家角色的帳號不該命中 = %+v, %v", page, err)
	}
}

// 帳號 ID 完全相符也找得到（舊的後台搜尋就支援）。email 不含這串數字，命中只可能來自 ID。
func TestListAccountsByID(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	f.emails["user_1"] = "x@example.com"
	id := f.account(t, "user_1")
	q := strconv.FormatInt(id, 10)
	page, err := f.m.ListAccounts(ctx, ListAccountsInput{Filter: AccountFilter{Query: " " + q + " "}, After: nil, Size: 0})
	if err != nil || len(page.Accounts) != 1 || page.Accounts[0].ID != id || page.Total != 1 {
		t.Fatalf("搜尋 ID = %+v, %v", page, err)
	}
	page, err = f.m.ListAccounts(ctx, ListAccountsInput{Filter: AccountFilter{Query: q + "9"}, After: nil, Size: 0})
	if err != nil || len(page.Accounts) != 0 || page.Total != 0 {
		t.Fatalf("ID 只比對完全相符 = %+v, %v", page, err)
	}
}

// 本機開發的 dev 登入：顧客是資料庫裡的 dev session（建帳、建 session），後台是 DevAuth 簽的 JWT。
func TestDevAuth(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	dev, err := NewDevAuth(nil)
	if err != nil {
		t.Fatal(err)
	}
	f.deps.Providers.Admin = dev.Admins()
	f.m = New(f.deps)

	id, buyer, err := f.m.DevNewSession(ctx)
	if err != nil {
		t.Fatal(err)
	}
	a, err := f.m.Authenticate(ctx, buyer)
	if err != nil || a.Principal.Kind != PrincipalAccount || a.Principal.ID != id {
		t.Fatalf("dev 的帳號 = %+v, %v", a, err)
	}
	if again, err := f.m.DevSession(ctx, id); err != nil || again == buyer {
		t.Fatalf("同一個 dev 帳號再建一個 session 要是新的 token：%v", err)
	}
	if n := f.count(t, `SELECT count(*) FROM identity.accounts`); n != 1 {
		t.Fatalf("帳號數 = %d", n)
	}
	if n := f.count(t, `SELECT count(*) FROM identity.account_identities`); n != 0 {
		t.Fatalf("dev 帳號不該有登入身分：%d 列", n)
	}
	_, err = f.m.DevSession(ctx, id+1000)
	wantKind(t, err, apperr.KindNotFound)
	admin, _ := dev.AdminToken("dev_ops")
	if adm, err := f.m.Authenticate(ctx, admin); err != nil || adm.Principal.Kind != PrincipalAdmin {
		t.Fatalf("dev 的管理員 = %+v, %v", adm, err)
	}
	if err := f.m.DeleteAccount(ctx, a.Principal.ID); err != nil {
		t.Fatal(err)
	}
	_, err = f.m.DevSession(ctx, id)
	wantKind(t, err, apperr.KindUnauthenticated)
	if n := f.count(t, `SELECT count(*) FROM identity.accounts`); n != 1 {
		t.Fatalf("已刪除的 dev 使用者又建了帳號：帳號數 = %d", n)
	}
}

// 生命週期 ACTIVE → PENDING_DELETION → DELETED：等待刪除的帳號仍可登入、可移除角色，但不能再授予角色；刪除時清掉
// 等待的狀態（accounts_deletion_pending_check）。
func TestDeletionPendingRefusesGrantRole(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	id := f.account(t, "user_1")
	if a, _ := f.m.Account(ctx, id); a.Lifecycle != LifecycleActive {
		t.Fatalf("lifecycle = %s", a.Lifecycle)
	}
	for range 2 { // 冪等
		if err := f.m.BeginDeletion(ctx, id); err != nil {
			t.Fatal(err)
		}
	}
	if a, _ := f.m.Account(ctx, id); a.Lifecycle != LifecyclePendingDeletion || a.Deleted {
		t.Fatalf("account = %+v", a)
	}
	wantKind(t, f.m.GrantRole(ctx, GrantRoleInput{AccountID: id, Role: RoleSeller}), apperr.KindFailedPrecondition)
	if a, _ := f.m.Account(ctx, id); len(a.Roles) != 0 {
		t.Fatalf("roles = %v", a.Roles)
	}
	if _, err := f.authenticate(t, "user_1"); err != nil {
		t.Fatalf("pending account must still authenticate: %v", err)
	}
	if err := f.m.DeleteAccount(ctx, id); err != nil {
		t.Fatal(err)
	}
	if a, _ := f.m.Account(ctx, id); a.Lifecycle != LifecycleDeleted || !a.Deleted {
		t.Fatalf("account = %+v", a)
	}
	if n := f.count(t, `SELECT count(*) FROM identity.accounts WHERE deletion_pending_at IS NOT NULL`); n != 0 {
		t.Fatalf("pending left on a tombstone: %d", n)
	}
	wantKind(t, f.m.BeginDeletion(ctx, id), apperr.KindNotFound)
	if _, err := f.db.Pool().Exec(ctx, `UPDATE identity.accounts SET deletion_pending_at = now() WHERE id = $1`, id); err == nil {
		t.Fatal("DB accepted a pending tombstone")
	}
}
