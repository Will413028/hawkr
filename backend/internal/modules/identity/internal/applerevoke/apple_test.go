package applerevoke

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"strings"
	"testing"

	"github.com/riverqueue/river"
	"github.com/riverqueue/river/rivertype"

	"github.com/Will413028/hawkr/backend/internal/modules/identity/internal/model"
)

type appleCall struct{ clientID, value string }

// testAppleTokens 以 code 換出 refresh token "rt-<code>"，ID token 是 "id-<code>"；code 是 "fail" 時 Apple 拒絕。
type testAppleTokens struct {
	exchanged []appleCall
	revoked   []appleCall
	revokeErr error
}

func (a *testAppleTokens) Exchange(_ context.Context, clientID, code string) (model.AppleGrant, error) {
	if code == "fail" {
		return model.AppleGrant{}, model.ErrRejected
	}
	a.exchanged = append(a.exchanged, appleCall{clientID, code})
	return model.AppleGrant{RefreshToken: "rt-" + code, IDToken: "id-" + code}, nil
}

func (a *testAppleTokens) Revoke(_ context.Context, clientID, token string) error {
	if a.revokeErr != nil {
		return a.revokeErr
	}
	a.revoked = append(a.revoked, appleCall{clientID, token})
	return nil
}

// testVerifier 把 "id-<code>" 對應到擁有那個 code 的 Apple 身分；down 表示連不到 Apple 的公鑰。
type testVerifier struct {
	ids  map[string]model.Identity
	down *bool
}

func (v testVerifier) Verify(_ context.Context, tok string) (model.Identity, error) {
	if *v.down {
		return model.Identity{}, errors.New("抓不到 JWKS")
	}
	id, ok := v.ids[tok]
	if !ok {
		return model.Identity{}, model.ErrInvalidToken
	}
	return id, nil
}

type testSealer struct{}

func (testSealer) Seal(p, aad []byte) ([]byte, error) {
	return append([]byte("sealed:"+string(aad)+":"), p...), nil
}

func (testSealer) Open(c, aad []byte) ([]byte, error) {
	prefix := []byte("sealed:" + string(aad) + ":")
	if !bytes.HasPrefix(c, prefix) {
		return nil, errors.New("aad 不符")
	}
	return c[len(prefix):], nil
}

func (testSealer) Has(string) bool { return true }

type testSaver struct {
	saved []string
	err   error
	fails int // 前幾次保存失敗
}

func (s *testSaver) save(_ context.Context, _ int64, _ string, sealed []byte) error {
	if s.fails > 0 {
		s.fails--
		return errors.New("db down")
	}
	if s.err != nil {
		return s.err
	}
	s.saved = append(s.saved, string(sealed))
	return nil
}

const buyerClient = "io.github.will413028.hawkr.buyer"

type testJobs struct {
	args []river.JobArgs
	err  error
}

func (j *testJobs) Enqueue(_ context.Context, args river.JobArgs, _ *river.InsertOpts) error {
	if j.err != nil {
		return j.err
	}
	j.args = append(j.args, args)
	return nil
}

type exchangeFixture struct {
	m      *Service
	tokens *testAppleTokens
	saver  *testSaver
	jobs   *testJobs
	down   *bool
	logs   *bytes.Buffer
}

func newExchangeFixture() *exchangeFixture {
	f := &exchangeFixture{tokens: &testAppleTokens{}, saver: &testSaver{}, jobs: &testJobs{}, down: new(bool), logs: &bytes.Buffer{}}
	verifier := testVerifier{down: f.down, ids: map[string]model.Identity{
		"id-code-A": {Subject: "apple-A", Audience: buyerClient},
		"id-code-B": {Subject: "apple-B", Audience: buyerClient},
		"id-code-W": {Subject: "apple-A", Audience: "com.other"},
	}}
	f.m = New(Deps{Apple: verifier, AppleTokens: f.tokens, Sealer: testSealer{}, Jobs: f.jobs, Logger: slog.New(slog.NewTextHandler(f.logs, nil))})
	f.m.saver = f.saver
	return f
}

// work 執行換 token 的 job：code 是 Apple 端的 code，linkedSub 是連結時驗證的 identity token 的 sub。
func (f *exchangeFixture) work(code, linkedSub string) error {
	args := exchangeAppleCodeArgs{AccountID: 7, ClientID: buyerClient, Subject: linkedSub, Code: append([]byte("sealed:"+string(codeAAD(7))+":"), code...)}
	return (&exchangeAppleCodeWorker{m: f.m}).Work(context.Background(), &river.Job[exchangeAppleCodeArgs]{JobRow: &rivertype.JobRow{Attempt: 1, MaxAttempts: 4}, Args: args})
}

func isCancel(err error) bool {
	var c *river.JobCancelError
	return errors.As(err, &c)
}

func TestExchangeStoresTheTokenWhenTheIDTokenMatchesTheLinkedIdentity(t *testing.T) {
	f := newExchangeFixture()
	if err := f.work("code-A", "apple-A"); err != nil {
		t.Fatal(err)
	}
	want := "sealed:" + string(tokenAAD(7)) + ":rt-code-A"
	if len(f.saver.saved) != 1 || f.saver.saved[0] != want || len(f.tokens.revoked) != 0 {
		t.Fatalf("saved=%q revoked=%v", f.saver.saved, f.tokens.revoked)
	}
}

// A 的 identity token 搭配 B 的 code：Apple 換 token 簽發的是 B 的 ID token，與連結時驗證的 A 不同，token 不保存並撤銷。
func TestExchangeRevokesWhenTheCodeBelongsToSomeoneElse(t *testing.T) {
	for name, tc := range map[string]struct{ code, linked, reason string }{
		"另一個人的 code":  {"code-B", "apple-A", "binding_mismatch"},
		"audience 不同": {"code-W", "apple-A", "binding_mismatch"},
		"ID token 無效": {"code-X", "apple-A", "binding_mismatch"},
	} {
		t.Run(name, func(t *testing.T) {
			f := newExchangeFixture()
			if err := f.work(tc.code, tc.linked); !isCancel(err) {
				t.Fatalf("err = %v，要取消 job", err)
			}
			if len(f.saver.saved) != 0 || len(f.tokens.revoked) != 1 || f.tokens.revoked[0] != (appleCall{buyerClient, "rt-" + tc.code}) {
				t.Fatalf("saved=%q revoked=%v", f.saver.saved, f.tokens.revoked)
			}
			logs := f.logs.String()
			if !strings.Contains(logs, "level=ERROR") || !strings.Contains(logs, "reason="+tc.reason) || !strings.Contains(logs, "account_id=7") ||
				strings.Contains(logs, "rt-"+tc.code) || strings.Contains(logs, tc.code+" ") {
				t.Fatalf("log = %s", logs)
			}
		})
	}
}

// 帳號已刪除：換得的 token 當場撤銷（刪除的 transaction 沒有看到它）。
func TestExchangeRevokesWhenTheAccountIsGone(t *testing.T) {
	f := newExchangeFixture()
	f.saver.err = errAccountGone
	if err := f.work("code-A", "apple-A"); !isCancel(err) {
		t.Fatalf("err = %v", err)
	}
	if len(f.tokens.revoked) != 1 || f.tokens.revoked[0] != (appleCall{buyerClient, "rt-code-A"}) || len(f.jobs.args) != 0 {
		t.Fatalf("revoked=%v jobs=%v", f.tokens.revoked, f.jobs.args)
	}
	if !strings.Contains(f.logs.String(), "level=ERROR") || !strings.Contains(f.logs.String(), "reason=account_deleted") {
		t.Fatalf("log = %s", f.logs.String())
	}
	f = newExchangeFixture()
	f.saver.err = errAccountGone
	f.tokens.revokeErr = errors.New("503")
	if err := f.work("code-A", "apple-A"); !isCancel(err) || !strings.Contains(f.logs.String(), "discard_revoke_failed") {
		t.Fatalf("撤銷也失敗要記 error：err=%v log=%s", err, f.logs.String())
	}
}

// 暫時性失敗（連不到 Apple 的公鑰、保存失敗）不撤銷（那會撤掉使用者剛給的授權），改排保存的 job，帶加密的 token 回應。
func TestExchangeDefersSavingOnTransientFailures(t *testing.T) {
	for name, tc := range map[string]struct {
		setup    func(*exchangeFixture)
		reason   string
		verified bool
	}{
		"連不到公鑰": {func(f *exchangeFixture) { *f.down = true }, "binding_unverifiable", false},
		"保存失敗":  {func(f *exchangeFixture) { f.saver.err = errors.New("db down") }, "persist_failed", true},
	} {
		t.Run(name, func(t *testing.T) {
			f := newExchangeFixture()
			tc.setup(f)
			if err := f.work("code-A", "apple-A"); !isCancel(err) {
				t.Fatalf("err = %v", err)
			}
			if len(f.tokens.revoked) != 0 || len(f.saver.saved) != 0 || len(f.jobs.args) != 1 {
				t.Fatalf("revoked=%v saved=%v jobs=%v", f.tokens.revoked, f.saver.saved, f.jobs.args)
			}
			save, ok := f.jobs.args[0].(saveAppleTokenArgs)
			if !ok || save.AccountID != 7 || save.Subject != "apple-A" || save.ClientID != buyerClient || save.Verified != tc.verified {
				t.Fatalf("job = %+v", f.jobs.args[0])
			}
			raw, err := (testSealer{}).Open(save.Grant, grantAAD(7))
			if err != nil || !strings.Contains(string(raw), "rt-code-A") {
				t.Fatalf("grant = %q, %v", raw, err)
			}
			if !strings.Contains(f.logs.String(), "level=ERROR") || !strings.Contains(f.logs.String(), "reason="+tc.reason) || strings.Contains(f.logs.String(), "rt-code-A") {
				t.Fatalf("log = %s", f.logs.String())
			}
		})
	}
	// 連排入保存的 job 也失敗：放棄，不撤銷。
	f := newExchangeFixture()
	f.saver.err = errors.New("db down")
	f.jobs.err = errors.New("db down")
	if err := f.work("code-A", "apple-A"); !isCancel(err) || len(f.tokens.revoked) != 0 || !strings.Contains(f.logs.String(), "defer_save_failed") {
		t.Fatalf("err=%v revoked=%v log=%s", err, f.tokens.revoked, f.logs.String())
	}
}

func (f *exchangeFixture) workSave(blob grantBlob, verified bool, attempt int) error {
	raw, _ := json.Marshal(blob)
	sealed, _ := (testSealer{}).Seal(raw, grantAAD(7))
	args := saveAppleTokenArgs{AccountID: 7, ClientID: buyerClient, Subject: "apple-A", Verified: verified, Grant: sealed}
	return (&saveAppleTokenWorker{m: f.m}).Work(context.Background(), &river.Job[saveAppleTokenArgs]{JobRow: &rivertype.JobRow{Attempt: attempt, MaxAttempts: saveAppleTokenMaxAttempts}, Args: args})
}

func TestSaveJobRetriesThenGivesUpWithoutRevoking(t *testing.T) {
	good := grantBlob{RefreshToken: "rt-code-A", IDToken: "id-code-A"}

	f := newExchangeFixture() // 重新驗證綁定後保存
	if err := f.workSave(good, false, 1); err != nil || len(f.saver.saved) != 1 {
		t.Fatalf("err=%v saved=%v", err, f.saver.saved)
	}
	f = newExchangeFixture() // 已驗證過：不再驗證（ID token 已過期也可保存）
	if err := f.workSave(grantBlob{RefreshToken: "rt-code-A", IDToken: "expired"}, true, 1); err != nil || len(f.saver.saved) != 1 {
		t.Fatalf("err=%v saved=%v", err, f.saver.saved)
	}

	f = newExchangeFixture() // 保存失敗：重試，成功後保存
	f.saver.fails = 1
	if err := f.workSave(good, true, 1); err == nil || isCancel(err) {
		t.Fatalf("要重試：%v", err)
	}
	if err := f.workSave(good, true, 2); err != nil || len(f.saver.saved) != 1 {
		t.Fatalf("err=%v saved=%v", err, f.saver.saved)
	}

	f = newExchangeFixture() // 最後一次仍失敗：error log、放棄、不撤銷
	f.saver.err = errors.New("db down")
	if err := f.workSave(good, true, saveAppleTokenMaxAttempts-1); err == nil || isCancel(err) {
		t.Fatalf("還沒到最後一次要重試：%v", err)
	}
	if err := f.workSave(good, true, saveAppleTokenMaxAttempts); !isCancel(err) || len(f.tokens.revoked) != 0 {
		t.Fatalf("err=%v revoked=%v", err, f.tokens.revoked)
	}
	if !strings.Contains(f.logs.String(), "level=ERROR") || !strings.Contains(f.logs.String(), "reason=persist_failed") || !strings.Contains(f.logs.String(), "account_id=7") {
		t.Fatalf("log = %s", f.logs.String())
	}

	f = newExchangeFixture() // 公鑰一直連不到：同樣放棄、不撤銷
	*f.down = true
	if err := f.workSave(good, false, saveAppleTokenMaxAttempts); !isCancel(err) || len(f.tokens.revoked) != 0 || len(f.saver.saved) != 0 {
		t.Fatalf("err=%v revoked=%v saved=%v", err, f.tokens.revoked, f.saver.saved)
	}

	f = newExchangeFixture() // 重新驗證發現身分不符：撤銷
	if err := f.workSave(grantBlob{RefreshToken: "rt-code-B", IDToken: "id-code-B"}, false, 1); !isCancel(err) || len(f.tokens.revoked) != 1 || len(f.saver.saved) != 0 {
		t.Fatalf("err=%v revoked=%v", err, f.tokens.revoked)
	}

	f = newExchangeFixture() // 帳號已刪除：撤銷
	f.saver.err = errAccountGone
	if err := f.workSave(good, true, 1); !isCancel(err) || len(f.tokens.revoked) != 1 {
		t.Fatalf("err=%v revoked=%v", err, f.tokens.revoked)
	}
}

func TestExchangeCancelsAndLogsWhenAppleRejectsTheCode(t *testing.T) {
	f := newExchangeFixture()
	if err := f.work("fail", "apple-A"); !isCancel(err) || len(f.saver.saved) != 0 {
		t.Fatalf("err = %v", err)
	}
	if !strings.Contains(f.logs.String(), "level=ERROR") || !strings.Contains(f.logs.String(), "reason=exchange_rejected") {
		t.Fatalf("log = %s", f.logs.String())
	}
}

func TestExchangeCodeFromAnotherAccountCannotBeOpened(t *testing.T) {
	f := newExchangeFixture()
	args := exchangeAppleCodeArgs{AccountID: 8, ClientID: buyerClient, Subject: "apple-A", Code: append([]byte("sealed:"+string(codeAAD(7))+":"), "code-A"...)}
	err := (&exchangeAppleCodeWorker{m: f.m}).Work(context.Background(), &river.Job[exchangeAppleCodeArgs]{JobRow: &rivertype.JobRow{}, Args: args})
	if !isCancel(err) || len(f.tokens.exchanged) != 0 {
		t.Fatalf("err=%v exchanged=%v", err, f.tokens.exchanged)
	}
}

func TestExchangeWithoutAppleConfigurationCancels(t *testing.T) {
	m := New(Deps{Logger: slog.New(slog.NewTextHandler(io.Discard, nil))})
	err := (&exchangeAppleCodeWorker{m: m}).Work(context.Background(), &river.Job[exchangeAppleCodeArgs]{JobRow: &rivertype.JobRow{}, Args: exchangeAppleCodeArgs{AccountID: 1}})
	if !isCancel(err) {
		t.Fatalf("err = %v", err)
	}
}

func TestRevokeTokenWorkerLogsFailuresWithoutTheToken(t *testing.T) {
	logs := &bytes.Buffer{}
	tokens := &testAppleTokens{}
	m := New(Deps{AppleTokens: tokens, Sealer: testSealer{}, Logger: slog.New(slog.NewTextHandler(logs, nil))})
	work := func(m *Service, attempt int, token string) error {
		args := revokeAppleTokenArgs{AccountID: 7, ClientID: buyerClient, Token: []byte(token)}
		return (&revokeAppleTokenWorker{m: m}).Work(context.Background(), &river.Job[revokeAppleTokenArgs]{JobRow: &rivertype.JobRow{Attempt: attempt, MaxAttempts: revokeAppleTokenMaxAttempts}, Args: args})
	}
	good := "sealed:" + string(tokenAAD(7)) + ":rt-secret"
	check := func(reason string) {
		t.Helper()
		l := logs.String()
		if !strings.Contains(l, "level=ERROR") || !strings.Contains(l, "reason="+reason) || !strings.Contains(l, "account_id=7") || strings.Contains(l, "rt-secret") {
			t.Fatalf("reason %s：log = %s", reason, l)
		}
		logs.Reset()
	}
	if err := work(m, 1, "garbage"); !isCancel(err) {
		t.Fatal(err)
	}
	check("open_token")
	tokens.revokeErr = errors.New("503")
	if err := work(m, 1, good); err == nil || logs.Len() != 0 {
		t.Fatalf("中途的暫時失敗只重試，不記 error：%v %s", err, logs.String())
	}
	if err := work(m, revokeAppleTokenMaxAttempts, good); err == nil {
		t.Fatal("最後一次仍回錯誤")
	}
	check("revoke_unavailable")
	tokens.revokeErr = model.ErrRejected
	_ = work(m, 1, good)
	check("revoke_rejected")
	if err := work(New(Deps{Logger: m.d.Logger}), 1, good); err == nil {
		t.Fatal("沒有設定 Apple 要回錯誤")
	}
	check("apple_not_configured")
}

func TestRevokeTokenWorker(t *testing.T) {
	tokens := &testAppleTokens{}
	m := New(Deps{AppleTokens: tokens, Sealer: testSealer{}, Logger: slog.New(slog.NewTextHandler(io.Discard, nil))})
	args := revokeAppleTokenArgs{AccountID: 7, ClientID: buyerClient, Token: []byte("sealed:" + string(tokenAAD(7)) + ":rt-code-1")}
	work := func(a revokeAppleTokenArgs) error {
		return (&revokeAppleTokenWorker{m: m}).Work(context.Background(), &river.Job[revokeAppleTokenArgs]{JobRow: &rivertype.JobRow{}, Args: a})
	}
	tokens.revokeErr = errors.New("503")
	if err := work(args); err == nil || isCancel(err) {
		t.Fatalf("transient failure must retry: %v", err)
	}
	tokens.revokeErr = nil
	if err := work(args); err != nil || len(tokens.revoked) != 1 || tokens.revoked[0] != (appleCall{buyerClient, "rt-code-1"}) {
		t.Fatalf("revoke = %v, %v", tokens.revoked, err)
	}
	other := args
	other.AccountID = 8
	if err := work(other); !isCancel(err) {
		t.Fatalf("token 搬到別的帳號必須解不開：%v", err)
	}
	tokens.revokeErr = model.ErrRejected
	if err := work(args); !isCancel(err) {
		t.Fatalf("rejected revoke = %v", err)
	}
}
