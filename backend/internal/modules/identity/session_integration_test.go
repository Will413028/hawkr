//go:build integration

package identity_test

import (
	"context"
	"strings"
	"sync"
	"testing"
	"time"

	. "github.com/Will413028/hawkr/backend/internal/modules/identity"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
	"github.com/Will413028/hawkr/backend/internal/platform/policy"
)

// who 是登入的 provider 身分，測試用最少的欄位描述。
type who struct {
	provider Provider
	subject  string
	email    string
	verified bool
	hd       string
}

func (w who) identity() ProviderIdentity {
	return ProviderIdentity{Provider: w.provider, Subject: w.subject, Email: w.email, EmailVerified: w.verified, HostedDomain: w.hd, Name: w.subject}
}

func (f *fixture) loginAs(t *testing.T, w who, client ClientKind) LoginResult {
	t.Helper()
	res, err := f.m.Login(context.Background(), LoginInput{Identity: w.identity(), Client: client})
	if err != nil {
		t.Fatalf("登入 %+v：%v", w, err)
	}
	return res
}

func (f *fixture) authenticated(t *testing.T, token string) Authenticated {
	t.Helper()
	a, err := f.m.Authenticate(context.Background(), token)
	if err != nil {
		t.Fatalf("Authenticate：%v", err)
	}
	return a
}

func (f *fixture) lastUsed(t *testing.T, token string) time.Time {
	t.Helper()
	var at time.Time
	if err := f.db.Pool().QueryRow(context.Background(),
		`SELECT last_used_at FROM identity.sessions WHERE token_hash = sha256($1::bytea)`, []byte(token)).Scan(&at); err != nil {
		t.Fatal(err)
	}
	return at
}

func (f *fixture) accountEmail(t *testing.T, id int64) string {
	t.Helper()
	acc, err := f.m.Account(context.Background(), id)
	if err != nil {
		t.Fatal(err)
	}
	return acc.Email
}

// failOn 讓對 table 的 op（INSERT、UPDATE 或 DELETE）一律丟錯，用來逼一個 transaction 的後一句失敗。
func (f *fixture) failOn(t *testing.T, op, table string) {
	t.Helper()
	ctx := context.Background()
	if _, err := f.db.Pool().Exec(ctx, `CREATE FUNCTION identity.force_fail() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'forced failure'; END $$`); err != nil {
		t.Fatal(err)
	}
	if _, err := f.db.Pool().Exec(ctx, `CREATE TRIGGER force_fail BEFORE `+op+` ON identity.`+table+` FOR EACH ROW EXECUTE FUNCTION identity.force_fail()`); err != nil {
		t.Fatal(err)
	}
}

var (
	googleAlice = who{provider: ProviderGoogle, subject: "google-alice", email: "alice@gmail.com", verified: true}
	lineBob     = who{provider: ProviderLine, subject: "line-bob"}
)

func TestLoginCreatesAnAccountForEachProvider(t *testing.T) {
	t.Parallel()
	f := setup(t)

	g := f.loginAs(t, googleAlice, ClientWeb)
	if !g.Created || g.Linked || g.Token == "" {
		t.Fatalf("Google 第一次登入 = %+v", g)
	}
	a := f.authenticated(t, g.Token)
	if a.Principal.Kind != PrincipalAccount || a.Principal.ID != g.AccountID || len(a.Principal.Roles) != 0 || a.SuspendedAt != nil {
		t.Fatalf("session 解析 = %+v", a)
	}
	// 已驗證的 email 成為寄件地址；建帳事件帶姓名（買家模組以它建立 profile）。
	if got := f.accountEmail(t, g.AccountID); got != "alice@gmail.com" {
		t.Fatalf("寄件地址 = %q", got)
	}
	if len(f.created) != 1 || f.created[0] != (AccountCreated{AccountID: g.AccountID, Name: "google-alice"}) {
		t.Fatalf("建帳事件 = %+v", f.created)
	}

	// LINE 沒有 email 照樣建帳，也沒有寄件地址。
	l := f.loginAs(t, lineBob, ClientApp)
	if !l.Created || l.AccountID == g.AccountID {
		t.Fatalf("LINE 第一次登入 = %+v", l)
	}
	if got := f.accountEmail(t, l.AccountID); got != "" {
		t.Fatalf("LINE 帳號的寄件地址 = %q，沒有 email", got)
	}
	if f.authenticated(t, l.Token).Principal.ID != l.AccountID {
		t.Fatal("LINE 的 session 解析到別的帳號")
	}

	// 同一個身分再登入是同一個帳號，不再建帳，而且是新的 session。
	again := f.loginAs(t, googleAlice, ClientWeb)
	if again.Created || again.Linked || again.AccountID != g.AccountID || again.Token == g.Token || again.SessionID == g.SessionID {
		t.Fatalf("第二次登入 = %+v（第一次 %+v）", again, g)
	}
	if n := f.count(t, `SELECT count(*) FROM identity.accounts`); n != 2 {
		t.Fatalf("帳號數 = %d", n)
	}
	// 登入拿到的是原文 token，資料庫只存 SHA-256。
	if n := f.count(t, `SELECT count(*) FROM identity.sessions WHERE token_hash = sha256(convert_to($1, 'UTF8'))`, g.Token); n != 1 {
		t.Fatalf("以 SHA-256 找到 %d 個 session", n)
	}
	if n := f.count(t, `SELECT count(*) FROM identity.sessions WHERE token_hash = convert_to($1, 'UTF8')`, g.Token); n != 0 {
		t.Fatal("資料庫存了原文 token")
	}
}

func TestLoginRejectsUnknownProviderAndClient(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	for name, in := range map[string]LoginInput{
		"空的 provider":  {Identity: ProviderIdentity{Subject: "s"}, Client: ClientWeb},
		"未知的 provider": {Identity: ProviderIdentity{Provider: "UNKNOWN", Subject: "s"}, Client: ClientWeb},
		"沒有 subject":   {Identity: ProviderIdentity{Provider: ProviderGoogle}, Client: ClientWeb},
		"空的 client":    {Identity: ProviderIdentity{Provider: ProviderGoogle, Subject: "s"}},
		"未知的 client":   {Identity: ProviderIdentity{Provider: ProviderGoogle, Subject: "s"}, Client: "UNKNOWN"},
	} {
		_, err := f.m.Login(ctx, in)
		if apperr.KindOf(err) != apperr.KindInvalidInput {
			t.Errorf("%s：err = %v，預期 InvalidInput", name, err)
		}
	}
	if n := f.count(t, `SELECT count(*) FROM identity.accounts`); n != 0 {
		t.Fatalf("被拒絕的登入建了 %d 個帳號", n)
	}
}

// 自動連結只在 Google 回報 email 已驗證、而且是 gmail.com 或帶 hd 時，且恰好一個既有帳號的寄件地址相符。
func TestAutoLinking(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name     string
		login    who
		existing []string // 既有帳號的寄件地址
		linked   bool
	}{
		{"Google 已驗證的 gmail 連結到既有帳號", who{provider: ProviderGoogle, subject: "g1", email: "shared@gmail.com", verified: true}, []string{"shared@gmail.com"}, true},
		{"email 大小寫不分", who{provider: ProviderGoogle, subject: "g1", email: "Shared@Gmail.com", verified: true}, []string{"shared@gmail.com"}, true},
		{"Google Workspace（有 hd）連結", who{provider: ProviderGoogle, subject: "g1", email: "a@corp.example", verified: true, hd: "corp.example"}, []string{"a@corp.example"}, true},
		{"Google 未驗證不連結", who{provider: ProviderGoogle, subject: "g1", email: "shared@gmail.com"}, []string{"shared@gmail.com"}, false},
		{"Google 非 gmail 且沒有 hd 不連結", who{provider: ProviderGoogle, subject: "g1", email: "a@corp.example", verified: true}, []string{"a@corp.example"}, false},
		{"LINE 同 email 不連結", who{provider: ProviderLine, subject: "l1", email: "shared@gmail.com"}, []string{"shared@gmail.com"}, false},
		{"LINE 就算標為已驗證也不連結", who{provider: ProviderLine, subject: "l1", email: "shared@gmail.com", verified: true}, []string{"shared@gmail.com"}, false},
		{"兩個帳號有同一個寄件地址時不猜", who{provider: ProviderGoogle, subject: "g1", email: "shared@gmail.com", verified: true}, []string{"shared@gmail.com", "shared@gmail.com"}, false},
		{"沒有符合的帳號就建新帳", who{provider: ProviderGoogle, subject: "g1", email: "shared@gmail.com", verified: true}, []string{"other@gmail.com"}, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			f := setup(t)
			var existing []int64
			for i, email := range c.existing {
				user := "user_" + string(rune('a'+i))
				f.customers.users[user] = ClerkIdentity{Email: email}
				existing = append(existing, f.account(t, user))
			}
			res := f.loginAs(t, c.login, ClientWeb)
			if res.Linked != c.linked || res.Created == c.linked {
				t.Fatalf("結果 = %+v，預期 linked=%v", res, c.linked)
			}
			if c.linked && res.AccountID != existing[0] {
				t.Fatalf("連結到帳號 %d，預期 %d", res.AccountID, existing[0])
			}
			if !c.linked {
				for _, id := range existing {
					if res.AccountID == id {
						t.Fatalf("不該連結卻登入了既有帳號 %d", id)
					}
				}
			}
			// 連結之後同一個身分以後直接登入該帳號，不再比對 email。
			if next := f.loginAs(t, c.login, ClientWeb); next.AccountID != res.AccountID || next.Created || next.Linked {
				t.Fatalf("再登入 = %+v，預期仍是帳號 %d", next, res.AccountID)
			}
			if n := f.count(t, `SELECT count(*) FROM identity.account_identities WHERE provider = $1 AND subject = $2`, string(c.login.provider), c.login.subject); n != 1 {
				t.Fatalf("身分列 %d 筆", n)
			}
		})
	}
}

func TestEachLoginUpdatesTheSenderAddressFromVerifiedEmailsOnly(t *testing.T) {
	t.Parallel()
	f := setup(t)
	first := f.loginAs(t, googleAlice, ClientWeb)
	changed := googleAlice
	changed.email = "alice.new@gmail.com"
	if res := f.loginAs(t, changed, ClientWeb); res.AccountID != first.AccountID {
		t.Fatalf("同一個 subject 換 email 仍是同一個帳號：%+v", res)
	}
	if got := f.accountEmail(t, first.AccountID); got != "alice.new@gmail.com" {
		t.Fatalf("寄件地址 = %q，預期跟上已驗證的新 email", got)
	}
	// 未驗證的 email 不動寄件地址，但身分記錄照實記下。
	unverified := changed
	unverified.email, unverified.verified = "typo@gmail.com", false
	f.loginAs(t, unverified, ClientWeb)
	if got := f.accountEmail(t, first.AccountID); got != "alice.new@gmail.com" {
		t.Fatalf("未驗證的 email 改了寄件地址：%q", got)
	}
	if n := f.count(t, `SELECT count(*) FROM identity.account_identities WHERE email = 'typo@gmail.com' AND NOT email_verified`); n != 1 {
		t.Fatalf("身分記錄沒有記下最近一次回報：%d", n)
	}
}

func TestLinkIdentityConflict(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	a := f.loginAs(t, googleAlice, ClientWeb)
	b := f.loginAs(t, lineBob, ClientWeb)

	// 把 Bob 的 LINE 身分連到 Alice：已屬於 Bob 的帳號，回錯誤，什麼都不改。
	err := f.m.LinkIdentity(ctx, a.AccountID, lineBob.identity())
	wantKind(t, err, apperr.KindAlreadyExists)
	var owner int64
	if err := f.db.Pool().QueryRow(ctx, `SELECT account_id FROM identity.account_identities WHERE provider = 'LINE' AND subject = 'line-bob'`).Scan(&owner); err != nil || owner != b.AccountID {
		t.Fatalf("身分的擁有者 = %d, %v，預期仍是 %d", owner, err, b.AccountID)
	}
	// 連結一個新的身分：之後用它登入就是 Alice 的帳號。
	second := who{provider: ProviderLine, subject: "line-alice"}
	if err := f.m.LinkIdentity(ctx, a.AccountID, second.identity()); err != nil {
		t.Fatal(err)
	}
	if res := f.loginAs(t, second, ClientApp); res.AccountID != a.AccountID || res.Created {
		t.Fatalf("以連結的 LINE 登入 = %+v", res)
	}
	// 重複連結同一個帳號：成功、不重複。
	if err := f.m.LinkIdentity(ctx, a.AccountID, second.identity()); err != nil {
		t.Fatal(err)
	}
	if n := f.count(t, `SELECT count(*) FROM identity.account_identities WHERE account_id = $1`, a.AccountID); n != 2 {
		t.Fatalf("Alice 的身分數 = %d", n)
	}
	wantKind(t, f.m.LinkIdentity(ctx, 9999, who{provider: ProviderLine, subject: "x"}.identity()), apperr.KindNotFound)
	wantKind(t, f.m.LinkIdentity(ctx, a.AccountID, ProviderIdentity{Provider: "UNKNOWN", Subject: "x"}), apperr.KindInvalidInput)
}

func TestConcurrentFirstLoginsShareOneAccount(t *testing.T) {
	t.Parallel()
	f := setup(t)
	const n = 8
	var wg sync.WaitGroup
	results := make([]LoginResult, n)
	errs := make([]error, n)
	for i := range n {
		wg.Add(1)
		go func() {
			defer wg.Done()
			results[i], errs[i] = f.m.Login(context.Background(), LoginInput{Identity: googleAlice.identity(), Client: ClientWeb})
		}()
	}
	wg.Wait()
	created := 0
	for i := range n {
		if errs[i] != nil || results[i].AccountID != results[0].AccountID {
			t.Fatalf("第 %d 個 = %+v, %v", i, results[i], errs[i])
		}
		if results[i].Created {
			created++
		}
	}
	if created != 1 {
		t.Fatalf("%d 個登入宣稱建了帳號，預期 1", created)
	}
	if got := f.count(t, `SELECT count(*) FROM identity.accounts`); got != 1 {
		t.Fatalf("建立了 %d 個帳號", got)
	}
	if got := f.count(t, `SELECT count(*) FROM identity.sessions`); got != n {
		t.Fatalf("session 數 = %d，預期每個登入一個", got)
	}
}

// 登入建帳、寫身分與建 session 是同一個 transaction（R4）：最後一句失敗時前面的都不留。
func TestLoginIsAtomic(t *testing.T) {
	t.Parallel()
	f := setup(t)
	f.failOn(t, "INSERT", "sessions")
	if _, err := f.m.Login(context.Background(), LoginInput{Identity: googleAlice.identity(), Client: ClientWeb}); err == nil {
		t.Fatal("建 session 失敗時登入要失敗")
	}
	for _, table := range []string{"accounts", "account_identities", "sessions"} {
		if n := f.count(t, `SELECT count(*) FROM identity.`+table); n != 0 {
			t.Errorf("%s 留下了 %d 列", table, n)
		}
	}
}

func TestLoginIssuesAFreshSessionAndRevokesThePreviousOne(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	old := f.loginAs(t, googleAlice, ClientWeb)
	res, err := f.m.Login(ctx, LoginInput{Identity: googleAlice.identity(), Client: ClientWeb, PreviousToken: old.Token})
	if err != nil {
		t.Fatal(err)
	}
	if res.Token == old.Token {
		t.Fatal("登入沒有換發新 token")
	}
	if _, err := f.m.Authenticate(ctx, old.Token); apperr.KindOf(err) != apperr.KindUnauthenticated {
		t.Fatalf("舊 token 還能用：%v", err)
	}
	f.authenticated(t, res.Token)
	if n := f.count(t, `SELECT count(*) FROM identity.sessions`); n != 1 {
		t.Fatalf("session 數 = %d", n)
	}
}

func TestSignOutDeletesOnlyThatSession(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	web := f.loginAs(t, googleAlice, ClientWeb)
	app := f.loginAs(t, googleAlice, ClientApp)
	if err := f.m.SignOut(ctx, web.Token); err != nil {
		t.Fatal(err)
	}
	if _, err := f.m.Authenticate(ctx, web.Token); apperr.KindOf(err) != apperr.KindUnauthenticated {
		t.Fatalf("登出後 = %v", err)
	}
	f.authenticated(t, app.Token)
	for _, tok := range []string{web.Token, "", "junk", strings.Repeat("x", 43)} { // 冪等，亂給的 token 也成功
		if err := f.m.SignOut(ctx, tok); err != nil {
			t.Fatalf("SignOut(%q) = %v", tok, err)
		}
	}
	f.authenticated(t, app.Token)
}

func TestSessionsCanBeListedAndRevokedOnlyByTheirOwner(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	a1 := f.loginAs(t, googleAlice, ClientWeb)
	f.now = f.now.Add(time.Minute)
	a2 := f.loginAs(t, googleAlice, ClientApp)
	b := f.loginAs(t, lineBob, ClientApp)

	list, err := f.m.ListSessions(ctx, a1.AccountID)
	if err != nil || len(list) != 2 || list[0].ID != a2.SessionID || list[1].ID != a1.SessionID || list[0].Client != ClientApp || list[1].Client != ClientWeb {
		t.Fatalf("列表 = %+v, %v，預期新到舊的兩個", list, err)
	}
	if !list[1].AuthTime.Equal(list[1].CreatedAt) || !list[1].AbsoluteExpiresAt.Equal(list[1].CreatedAt.Add(90*24*time.Hour)) {
		t.Fatalf("auth_time／絕對到期 = %+v", list[1])
	}
	// 別人的 session 不能終止，也不能標記重新驗證。
	wantKind(t, f.m.RevokeSession(ctx, a1.AccountID, b.SessionID), apperr.KindNotFound)
	wantKind(t, f.m.MarkSessionAuthenticated(ctx, a1.AccountID, b.SessionID), apperr.KindNotFound)
	f.authenticated(t, b.Token)

	f.now = f.now.Add(10 * time.Minute)
	if err := f.m.MarkSessionAuthenticated(ctx, a1.AccountID, a1.SessionID); err != nil {
		t.Fatal(err)
	}
	list, _ = f.m.ListSessions(ctx, a1.AccountID)
	if !list[1].AuthTime.Equal(f.now) {
		t.Fatalf("重新驗證後 auth_time = %v，預期 %v", list[1].AuthTime, f.now)
	}
	if err := f.m.RevokeSession(ctx, a1.AccountID, a1.SessionID); err != nil {
		t.Fatal(err)
	}
	if _, err := f.m.Authenticate(ctx, a1.Token); apperr.KindOf(err) != apperr.KindUnauthenticated {
		t.Fatalf("被終止的 session = %v", err)
	}
	wantKind(t, f.m.RevokeSession(ctx, a1.AccountID, a1.SessionID), apperr.KindNotFound)
}

func TestSessionIdleAndAbsoluteExpiry(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	day := 24 * time.Hour
	unauthenticated := func(tok string) bool {
		_, err := f.m.Authenticate(ctx, tok)
		return apperr.KindOf(err) == apperr.KindUnauthenticated
	}

	// 閒置逾時：web 7 天、App 14 天；差一秒還有效，到期那一刻失效，失效的列同時被清掉。
	web := f.loginAs(t, googleAlice, ClientWeb)
	app := f.loginAs(t, lineBob, ClientApp)
	f.now = f.now.Add(7*day - time.Second)
	f.authenticated(t, web.Token)
	f.authenticated(t, app.Token)
	f.now = f.now.Add(time.Second) // 距 web 上次使用（上一行的續期）還不到 7 天
	f.authenticated(t, web.Token)
	idleWeb := f.loginAs(t, who{provider: ProviderGoogle, subject: "idle-web"}, ClientWeb)
	f.now = f.now.Add(7 * day)
	if !unauthenticated(idleWeb.Token) {
		t.Fatal("web session 閒置滿 7 天要失效")
	}
	if n := f.count(t, `SELECT count(*) FROM identity.sessions WHERE id = $1`, idleWeb.SessionID); n != 0 {
		t.Fatal("逾時的 session 沒有被刪掉")
	}
	idleApp := f.loginAs(t, who{provider: ProviderLine, subject: "idle-app"}, ClientApp)
	f.now = f.now.Add(14*day - time.Second)
	f.authenticated(t, idleApp.Token)
	f.now = f.now.Add(14 * day)
	if !unauthenticated(idleApp.Token) {
		t.Fatal("App session 閒置滿 14 天要失效")
	}

	// 絕對逾時：每 10 天使用一次（永遠沒閒置逾時），第 90 天還是失效。
	start := f.now
	forever := f.loginAs(t, who{provider: ProviderLine, subject: "forever"}, ClientApp)
	for i := 1; i <= 8; i++ {
		f.now = start.Add(time.Duration(i) * 10 * day)
		f.authenticated(t, forever.Token)
	}
	f.now = start.Add(90*day - time.Second)
	f.authenticated(t, forever.Token)
	f.now = start.Add(90 * day)
	if !unauthenticated(forever.Token) {
		t.Fatal("建立滿 90 天要失效，不論多常使用")
	}
}

func TestSessionRenewalWritesAtMostOncePerDay(t *testing.T) {
	t.Parallel()
	f := setup(t)
	t0 := f.now
	res := f.loginAs(t, googleAlice, ClientWeb)
	f.now = t0.Add(time.Hour)
	f.authenticated(t, res.Token)
	if got := f.lastUsed(t, res.Token); !got.Equal(t0) {
		t.Fatalf("一小時內續期寫了 last_used_at = %v", got)
	}
	f.now = t0.Add(24 * time.Hour)
	f.authenticated(t, res.Token)
	if got := f.lastUsed(t, res.Token); !got.Equal(t0.Add(24 * time.Hour)) {
		t.Fatalf("滿一天後 last_used_at = %v，預期續期", got)
	}
	f.now = t0.Add(25 * time.Hour)
	f.authenticated(t, res.Token)
	if got := f.lastUsed(t, res.Token); !got.Equal(t0.Add(24 * time.Hour)) {
		t.Fatalf("續期後一小時又寫了 last_used_at = %v", got)
	}
}

func TestSessionReadsAccountStatusAndRolesOnEveryRequest(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	res := f.loginAs(t, googleAlice, ClientWeb)
	if err := f.m.GrantRole(ctx, GrantRoleInput{AccountID: res.AccountID, Role: RoleSeller, SourceID: ptr[int64](1)}); err != nil {
		t.Fatal(err)
	}
	if a := f.authenticated(t, res.Token); len(a.Principal.Roles) != 1 || a.Principal.Roles[0] != RoleSeller {
		t.Fatalf("授予角色後 = %+v", a)
	}
	if err := f.m.RemoveRole(ctx, RemoveRoleInput{AccountID: res.AccountID, Role: RoleSeller, SourceID: ptr[int64](1)}); err != nil {
		t.Fatal(err)
	}
	if a := f.authenticated(t, res.Token); len(a.Principal.Roles) != 0 {
		t.Fatalf("移除角色後 = %+v", a)
	}
}

func TestSuspensionEndsSessionsAndTheNextLoginSeesTheSuspension(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	res := f.loginAs(t, googleAlice, ClientWeb)
	other := f.loginAs(t, lineBob, ClientWeb)
	link := who{provider: ProviderLine, subject: "line-alice"}
	if err := f.m.LinkIdentity(ctx, res.AccountID, link.identity()); err != nil {
		t.Fatal(err)
	}
	second := f.loginAs(t, link, ClientApp)

	for range 2 { // 冪等：已經停權時不再動 session
		if _, _, err := f.m.SuspendAccount(ctx, SuspendInput{AccountID: res.AccountID, Category: policy.Other, Reason: "檢舉成立"}); err != nil {
			t.Fatal(err)
		}
	}
	for _, tok := range []string{res.Token, second.Token} {
		if _, err := f.m.Authenticate(ctx, tok); apperr.KindOf(err) != apperr.KindUnauthenticated {
			t.Fatalf("停權後原本的 session = %v，預期失效", err)
		}
	}
	f.authenticated(t, other.Token) // 別的帳號不受影響
	// 停權者可以重新登入（A7-D18），session 解析時讀到停權狀態。
	again := f.loginAs(t, googleAlice, ClientWeb)
	if a := f.authenticated(t, again.Token); a.SuspendedAt == nil || !a.SuspendedAt.Equal(f.now) || a.Principal.ID != res.AccountID {
		t.Fatalf("停權後重新登入 = %+v", a)
	}
	// 重複停權不會把停權後新建的 session 刪掉。
	if _, created, err := f.m.SuspendAccount(ctx, SuspendInput{AccountID: res.AccountID, Category: policy.Other}); err != nil || created {
		t.Fatalf("重複停權 = %v, %v", created, err)
	}
	f.authenticated(t, again.Token)
}

// 停權寫停權紀錄、accounts.suspended_at 與刪 session 是同一個 transaction（R4）：刪 session 失敗時前面的都不留。
func TestSuspensionAndSessionDeletionAreAtomic(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	res := f.loginAs(t, googleAlice, ClientWeb)
	f.failOn(t, "DELETE", "sessions")
	if _, _, err := f.m.SuspendAccount(ctx, SuspendInput{AccountID: res.AccountID, Category: policy.Other, Reason: "x"}); err == nil {
		t.Fatal("刪 session 失敗時停權要失敗")
	}
	if f.suspendedAt(t, res.AccountID) {
		t.Fatal("停權的 transaction 沒有 rollback：suspended_at 還在")
	}
	if all, _ := f.suspensionRows(t, res.AccountID); all != 0 {
		t.Fatalf("留下了 %d 筆停權紀錄", all)
	}
	f.authenticated(t, res.Token)
}

func TestDeletionEndsSessionsAndLeavesIdentityTombstones(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	res := f.loginAs(t, googleAlice, ClientWeb)
	app := f.loginAs(t, googleAlice, ClientApp)
	if err := f.m.DeleteAccount(ctx, res.AccountID); err != nil {
		t.Fatal(err)
	}
	for _, tok := range []string{res.Token, app.Token} {
		if _, err := f.m.Authenticate(ctx, tok); apperr.KindOf(err) != apperr.KindUnauthenticated {
			t.Fatalf("刪帳後的 session = %v，預期失效", err)
		}
	}
	if n := f.count(t, `SELECT count(*) FROM identity.sessions WHERE account_id = $1`, res.AccountID); n != 0 {
		t.Errorf("刪帳後 sessions 還有 %d 列", n)
	}
	if n := f.count(t, `SELECT count(*) FROM identity.account_identities WHERE provider = 'GOOGLE' AND subject = 'google-alice' AND account_id = $1 AND email IS NULL AND NOT email_verified`, res.AccountID); n != 1 {
		t.Errorf("墓碑身分列 = %d，預期保留 provider 與 subject、清掉 email", n)
	}
	if acc, err := f.m.Account(ctx, res.AccountID); err != nil || !acc.Deleted || acc.Email != "" {
		t.Fatalf("墓碑 = %+v, %v", acc, err)
	}
	accounts := f.count(t, `SELECT count(*) FROM identity.accounts`)
	for _, client := range []ClientKind{ClientWeb, ClientApp} {
		_, err := f.m.Login(ctx, LoginInput{Identity: googleAlice.identity(), Client: client})
		wantKind(t, err, apperr.KindUnauthenticated)
	}
	if n := f.count(t, `SELECT count(*) FROM identity.accounts`); n != accounts {
		t.Fatalf("墓碑身分再登入後帳號數 %d -> %d，預期不建新帳號", accounts, n)
	}
	if n := f.count(t, `SELECT count(*) FROM identity.sessions`); n != 0 {
		t.Fatalf("被拒的登入留下 %d 個 session", n)
	}
}

// 已刪帳號的身分不能被連到另一個帳號：自動連結只看存活帳號，手動連結回 AlreadyExists 且不改資料。
func TestDeletedAccountsIdentityCannotBeReusedOrLinked(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	gone := f.loginAs(t, lineBob, ClientWeb)
	other := f.loginAs(t, googleAlice, ClientWeb)
	if err := f.m.DeleteAccount(ctx, gone.AccountID); err != nil {
		t.Fatal(err)
	}
	wantKind(t, f.m.LinkIdentity(ctx, other.AccountID, lineBob.identity()), apperr.KindAlreadyExists)
	if n := f.count(t, `SELECT count(*) FROM identity.account_identities WHERE provider = 'LINE' AND subject = 'line-bob' AND account_id = $1`, gone.AccountID); n != 1 {
		t.Fatalf("手動連結失敗後墓碑身分 = %d，預期仍屬於已刪帳號", n)
	}
	_, err := f.m.Login(ctx, LoginInput{Identity: lineBob.identity(), Client: ClientWeb})
	wantKind(t, err, apperr.KindUnauthenticated)
}

// 刪帳的墓碑、刪 session 與清身分個資是同一個 transaction（R4）：任何一句失敗，前面的都不留。
func TestDeletionIsAtomicAcrossSessionsAndIdentities(t *testing.T) {
	for _, c := range []struct{ op, table string }{{"DELETE", "sessions"}, {"UPDATE", "account_identities"}} {
		t.Run(c.table, func(t *testing.T) {
			t.Parallel()
			f := setup(t)
			ctx := context.Background()
			res := f.loginAs(t, googleAlice, ClientWeb)
			f.failOn(t, c.op, c.table)
			if err := f.m.DeleteAccount(ctx, res.AccountID); err == nil {
				t.Fatal("刪除失敗時刪帳要失敗")
			}
			if acc, err := f.m.Account(ctx, res.AccountID); err != nil || acc.Deleted || acc.Email != "alice@gmail.com" {
				t.Fatalf("帳號 = %+v, %v，預期沒有變成墓碑", acc, err)
			}
			if n := f.count(t, `SELECT count(*) FROM identity.sessions WHERE account_id = $1`, res.AccountID); n != 1 {
				t.Errorf("sessions 剩 %d 列，預期 1", n)
			}
			if n := f.count(t, `SELECT count(*) FROM identity.account_identities WHERE account_id = $1 AND email = 'alice@gmail.com'`, res.AccountID); n != 1 {
				t.Errorf("身分的 email 被清掉了（%d 列保留），預期整筆回滾", n)
			}
			f.authenticated(t, res.Token)
		})
	}
}

func TestLoginFlowIsSingleUse(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	begin := func(provider Provider) LoginFlowStart {
		t.Helper()
		s, err := f.m.BeginLoginFlow(ctx, LoginFlowInput{Provider: provider, ReturnPath: "/orders?x=1"})
		if err != nil {
			t.Fatal(err)
		}
		return s
	}
	s := begin(ProviderGoogle)
	if s.State == "" || s.Nonce == "" || s.CodeVerifier == "" || s.Binding == "" || s.State == s.Nonce || s.State == s.Binding || s.Nonce == s.CodeVerifier {
		t.Fatalf("start = %+v，四個值要各自隨機", s)
	}
	flow, err := f.m.ConsumeLoginFlow(ctx, s.State, s.Binding, ProviderGoogle)
	if err != nil || flow.Provider != ProviderGoogle || flow.Nonce != s.Nonce || flow.CodeVerifier != s.CodeVerifier || flow.ReturnPath != "/orders?x=1" || flow.LinkAccountID != nil {
		t.Fatalf("callback = %+v, %v", flow, err)
	}
	// state 重放被拒。
	_, err = f.m.ConsumeLoginFlow(ctx, s.State, s.Binding, ProviderGoogle)
	wantKind(t, err, apperr.KindUnauthenticated)
	if n := f.count(t, `SELECT count(*) FROM identity.login_flows`); n != 0 {
		t.Fatalf("留下 %d 列", n)
	}
	// 驗證失敗也作廢：之後即使帶對的值也不行。
	for name, bad := range map[string]func(LoginFlowStart) (string, string, Provider){
		"provider 與 start 不同（mix-up）": func(s LoginFlowStart) (string, string, Provider) { return s.State, s.Binding, ProviderLine },
		"登入前 cookie 不符（login CSRF）":   func(s LoginFlowStart) (string, string, Provider) { return s.State, "forged", ProviderGoogle },
		"沒有登入前 cookie":                func(s LoginFlowStart) (string, string, Provider) { return s.State, "", ProviderGoogle },
	} {
		s := begin(ProviderGoogle)
		state, binding, provider := bad(s)
		_, err := f.m.ConsumeLoginFlow(ctx, state, binding, provider)
		if apperr.KindOf(err) != apperr.KindUnauthenticated {
			t.Errorf("%s：err = %v", name, err)
		}
		if _, err := f.m.ConsumeLoginFlow(ctx, s.State, s.Binding, ProviderGoogle); apperr.KindOf(err) != apperr.KindUnauthenticated {
			t.Errorf("%s：失敗之後 state 還能用：%v", name, err)
		}
	}
	// 逾時（10 分鐘）無效，而且那一列被清掉。
	s = begin(ProviderLine)
	f.now = f.now.Add(10*time.Minute - time.Second)
	s2 := begin(ProviderLine)
	if _, err := f.m.ConsumeLoginFlow(ctx, s2.State, s2.Binding, ProviderLine); err != nil {
		t.Fatalf("差一秒到期還有效：%v", err)
	}
	f.now = f.now.Add(time.Second)
	_, err = f.m.ConsumeLoginFlow(ctx, s.State, s.Binding, ProviderLine)
	wantKind(t, err, apperr.KindUnauthenticated)
	// 手動連結的流程帶要連結的帳號。
	acc := f.loginAs(t, googleAlice, ClientWeb)
	ls, err := f.m.BeginLoginFlow(ctx, LoginFlowInput{Provider: ProviderLine, ReturnPath: "/", LinkAccountID: &acc.AccountID})
	if err != nil {
		t.Fatal(err)
	}
	if lf, err := f.m.ConsumeLoginFlow(ctx, ls.State, ls.Binding, ProviderLine); err != nil || lf.LinkAccountID == nil || *lf.LinkAccountID != acc.AccountID {
		t.Fatalf("連結流程 = %+v, %v", lf, err)
	}
}

func TestLoginFlowRefusesUnsafeReturnPathsAndUnknownProviders(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	for _, p := range []string{"", "//evil.example", "/\\evil.example", "https://evil.example/", "evil", "/a\r\nb"} {
		_, err := f.m.BeginLoginFlow(ctx, LoginFlowInput{Provider: ProviderGoogle, ReturnPath: p})
		if apperr.KindOf(err) != apperr.KindInvalidInput {
			t.Errorf("返回網址 %q：err = %v，預期 InvalidInput", p, err)
		}
	}
	for _, p := range []Provider{"", "UNKNOWN", "google"} {
		_, err := f.m.BeginLoginFlow(ctx, LoginFlowInput{Provider: p, ReturnPath: "/"})
		if apperr.KindOf(err) != apperr.KindInvalidInput {
			t.Errorf("provider %q：err = %v，預期 InvalidInput", p, err)
		}
	}
	if n := f.count(t, `SELECT count(*) FROM identity.login_flows`); n != 0 {
		t.Fatalf("被拒絕的 start 留下 %d 列", n)
	}
	// 資料庫也擋：就算程式漏檢，返回網址不是站內相對路徑的列寫不進去。
	_, err := f.db.Pool().Exec(ctx, `INSERT INTO identity.login_flows (state_hash, provider, code_verifier, nonce, return_path, binding_hash, created_at, expires_at)
		VALUES (sha256('a'), 'GOOGLE', 'v', 'n', '//evil.example', sha256('b'), now(), now() + interval '1 minute')`)
	if err == nil {
		t.Fatal("資料庫收了 //evil.example")
	}
}

func TestAppNonceIsSingleUseAndShortLived(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	nonce, expires, err := f.m.IssueAppNonce(ctx)
	if err != nil || nonce == "" || !expires.Equal(f.now.Add(5*time.Minute)) {
		t.Fatalf("IssueAppNonce = %q, %v, %v", nonce, expires, err)
	}
	if n := f.count(t, `SELECT count(*) FROM identity.app_login_nonces WHERE nonce_hash = convert_to($1, 'UTF8')`, nonce); n != 0 {
		t.Fatal("資料庫存了原文 nonce")
	}
	if err := f.m.ConsumeAppNonce(ctx, nonce); err != nil {
		t.Fatal(err)
	}
	wantKind(t, f.m.ConsumeAppNonce(ctx, nonce), apperr.KindUnauthenticated) // 重放
	wantKind(t, f.m.ConsumeAppNonce(ctx, "junk"), apperr.KindUnauthenticated)
	wantKind(t, f.m.ConsumeAppNonce(ctx, strings.Repeat("A", 43)), apperr.KindUnauthenticated)

	late, _, err := f.m.IssueAppNonce(ctx)
	if err != nil {
		t.Fatal(err)
	}
	f.now = f.now.Add(5 * time.Minute)
	wantKind(t, f.m.ConsumeAppNonce(ctx, late), apperr.KindUnauthenticated)
	if n := f.count(t, `SELECT count(*) FROM identity.app_login_nonces`); n != 0 {
		t.Fatalf("逾時的 nonce 沒有被清掉：%d", n)
	}
}

func TestSessionTokensComeOnlyFromLogin(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	res := f.loginAs(t, googleAlice, ClientWeb)
	// session ID、截短、加長的值都不是憑證。
	for name, tok := range map[string]string{
		"雜湊的十六進位":    strings.Repeat("a", 43),
		"session ID": res.SessionID.String(),
		"空白補長":       res.Token[:42] + " ",
		"截短":         res.Token[:42],
		"加長":         res.Token + "A",
	} {
		if _, err := f.m.Authenticate(ctx, tok); apperr.KindOf(err) != apperr.KindUnauthenticated {
			t.Errorf("%s：err = %v，預期 unauthenticated", name, err)
		}
	}
}
