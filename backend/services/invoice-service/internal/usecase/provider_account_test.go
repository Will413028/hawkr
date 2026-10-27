package usecase

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/Will413028/hawkr/go-pkg/codemeta"
	"github.com/Will413028/hawkr/go-pkg/errors/bizcode"
	"github.com/Will413028/hawkr/invoice-service/internal/client/amego"
	"github.com/Will413028/hawkr/invoice-service/internal/domain"
	"go.uber.org/zap"
)

// 本檔的每一支陰性測試都在驗同一件事：**守衛不成立時什麼都不能發生**。
// 只斷言「回了哪個碼」不夠——rows=0 的危險形狀是「回了錯誤，但前面已經寫了一半」。
// 所以每一支陰性都額外數 repo 的呼叫次數。
//
// 🔴 另有一組**明文旁路**斷言（TestProviderAccount_AppKeyNeverLeaves…）：AppKey 是
// write-only 的憑證，它只能出現在 repository 的寫入參數裡。回應、狀態歷程、ADM 稽核
// 任何一處出現它都是外洩，而那三處都是持久化且保存期以年計。

var providerNow = time.Date(2026, 8, 30, 6, 0, 0, 0, time.UTC)

const testAppKey = "amego-app-key-SECRET-9f2c"

// providerAdjs 記下 P14／P16 有沒有把該 issuer 的未送出調整一起收掉（#3073）。
type providerAdjs struct {
	domain.AdjustmentRepository
	calls   int
	issuers []int64
	reasons []string
	rows    int64
}

func (f *providerAdjs) CancelUnsentForIssuer(
	_ context.Context, issuerID int64, reasonCode string, _ time.Time,
) (int64, error) {
	f.calls++
	f.issuers = append(f.issuers, issuerID)
	f.reasons = append(f.reasons, reasonCode)
	return f.rows, nil
}

type providerFixture struct {
	uc       *ProviderAccount
	adjs     *providerAdjs
	issuers  *fakeIssuerRepo
	profiles *fakeProfileRepo
	accounts *fakeProviderAccountRepo
	ops      *fakeOpRepo
	audit    *fakeAudit
	provider *fakeProvider
	cutovers *fakeCutoverRepo
}

func newProviderFixture(account domain.ProviderAccount) *providerFixture {
	issuers := &fakeIssuerRepo{issuer: domain.Issuer{ID: 900, IssuerType: "SELLER", IssuerRefID: 2685914682}}
	taxID := "12345678"
	profiles := &fakeProfileRepo{profile: domain.SellerTaxProfile{
		ID: 500, IssuerID: 900, TaxID: &taxID,
		ReviewStatus:      codemeta.SellerTaxReviewStatusApprovedInvoiceRequired.Value,
		PendingOrderCount: 3,
	}}
	accounts := newFakeProviderAccountRepo(account)
	ops := &fakeOpRepo{openCount: 7}
	audit := &fakeAudit{}
	provider := &fakeProvider{verify: fakeProviderVerify{
		tracks: []amego.Track{{Code: "AB", TrackAPICode: "api-code-1"}},
	}}
	cutovers := &fakeCutoverRepo{}
	adjs := &providerAdjs{}
	uc := NewProviderAccount(directTx{}, issuers, profiles, accounts, ops, adjs, audit, cutovers, provider,
		func() int64 { return 4242 }, zap.NewNop()).
		WithClock(func() time.Time { return providerNow })
	return &providerFixture{uc: uc, adjs: adjs, issuers: issuers, profiles: profiles, accounts: accounts,
		ops: ops, audit: audit, provider: provider, cutovers: cutovers}
}

func accountAt(status string, version int64) domain.ProviderAccount {
	verified := providerNow.Add(-24 * time.Hour)
	return domain.ProviderAccount{
		ID: 3001, IssuerID: 900, Provider: "AMEGO", ProviderTaxID: "12345678",
		Status: status, Version: version, CredentialLast4: "9f2c", VerifiedAt: &verified,
	}
}

// ---------------------------------------------------------------------------
// 端點 24 憑證寫入
// ---------------------------------------------------------------------------

func TestProviderAccount_PutCredentials_FirstBind(t *testing.T) {
	fx := newProviderFixture(domain.ProviderAccount{})
	fx.accounts.getErr = notFoundErr(bizcode.InvoiceProviderAccountNotFound)

	got, err := fx.uc.PutCredentials(context.Background(), PutProviderCredentialsInput{
		StoreID: 2685914682, SellerTaxID: "12345678", AppKey: testAppKey,
		ConsentVersion: "v1", ExpectedVersion: 0, IdempotencyKey: "idem-24",
	})
	if err != nil {
		t.Fatalf("PutCredentials 失敗：%v", err)
	}
	if len(fx.accounts.created) != 1 {
		t.Fatalf("應建列一次，實得 %d", len(fx.accounts.created))
	}
	// 落地即 CREDENTIAL_VERIFYING（§6.1.8 端點 24）：CREDENTIAL_SUBMITTED 是不對外呈現
	// 的中間態，賣家按下儲存後看到的下一格必須是「驗證中」。
	if fx.accounts.created[0].Status != codemeta.InvoiceProviderAccountStatusCredentialVerifying.Value {
		t.Fatalf("建列狀態應為 CREDENTIAL_VERIFYING，實得 %q", fx.accounts.created[0].Status)
	}
	if got.Account.Status != codemeta.InvoiceProviderAccountStatusCredentialVerifying.Value {
		t.Fatalf("回應狀態應為 CREDENTIAL_VERIFYING，實得 %q", got.Account.Status)
	}
	if len(fx.audit.entries) != 1 || fx.audit.entries[0].SubjectType != providerSubjectType {
		t.Fatalf("應寫一列 PROVIDER_ACCOUNT 狀態歷程，實得 %+v", fx.audit.entries)
	}
}

// TestProviderAccount_PutCredentials_TaxIDMismatch 驗 9334。
//
// 這一格必須在**寫入之前**擋：開錯抬頭的發票不能改只能作廢重開，而作廢有法定窗口。
func TestProviderAccount_PutCredentials_TaxIDMismatch(t *testing.T) {
	fx := newProviderFixture(domain.ProviderAccount{})
	fx.accounts.getErr = notFoundErr(bizcode.InvoiceProviderAccountNotFound)

	_, err := fx.uc.PutCredentials(context.Background(), PutProviderCredentialsInput{
		StoreID: 2685914682, SellerTaxID: "87654321", AppKey: testAppKey, ConsentVersion: "v1",
	})
	assertBiz(t, err, bizcode.InvoiceProviderTaxidMismatch)
	if len(fx.accounts.created) != 0 || len(fx.accounts.seenAppKeys) != 0 {
		t.Fatalf("統編不符時不得有任何寫入，created=%d seenAppKeys=%d",
			len(fx.accounts.created), len(fx.accounts.seenAppKeys))
	}
}

// TestProviderAccount_PutCredentials_NotApproved 驗 9321（稅務身分未核准）。
func TestProviderAccount_PutCredentials_NotApproved(t *testing.T) {
	fx := newProviderFixture(domain.ProviderAccount{})
	fx.accounts.getErr = notFoundErr(bizcode.InvoiceProviderAccountNotFound)
	fx.profiles.profile.ReviewStatus = "SUBMITTED"

	_, err := fx.uc.PutCredentials(context.Background(), PutProviderCredentialsInput{
		StoreID: 2685914682, SellerTaxID: "12345678", AppKey: testAppKey, ConsentVersion: "v1",
	})
	assertBiz(t, err, bizcode.SellerTaxProfileStateInvalid)
	if len(fx.accounts.created) != 0 {
		t.Fatalf("未核准時不得建列")
	}
}

// TestProviderAccount_PutCredentials_ActiveRejected 驗 allowed-from：ACTIVE 的換 key
// 必須走端點 27 輪替。fake 以 resubmitOK=false 模擬 SQL 的 rows=0。
func TestProviderAccount_PutCredentials_ActiveRejected(t *testing.T) {
	fx := newProviderFixture(accountAt("ACTIVE", 5))
	fx.accounts.resubmitOK = false

	_, err := fx.uc.PutCredentials(context.Background(), PutProviderCredentialsInput{
		StoreID: 2685914682, SellerTaxID: "12345678", AppKey: testAppKey,
		ConsentVersion: "v1", ExpectedVersion: 5,
	})
	assertBiz(t, err, bizcode.InvoiceProviderStateInvalid)
	if len(fx.audit.entries) != 0 {
		t.Fatalf("CAS 不成立時不得寫狀態歷程，實得 %d 列", len(fx.audit.entries))
	}
}

// TestProviderAccount_PutCredentials_FingerprintTaken 驗 9337。
//
// 🔴 **不得回 9331**：那會把賣家引導去改一把其實正確的金鑰，而正解是「這組憑證已被
// 別人綁走」。兩者的處置完全相反。
func TestProviderAccount_PutCredentials_FingerprintTaken(t *testing.T) {
	fx := newProviderFixture(accountAt("INVALID_CREDENTIAL", 5))
	fx.accounts.taken = true

	_, err := fx.uc.PutCredentials(context.Background(), PutProviderCredentialsInput{
		StoreID: 2685914682, SellerTaxID: "12345678", AppKey: testAppKey,
		ConsentVersion: "v1", ExpectedVersion: 5,
	})
	assertBiz(t, err, bizcode.InvoiceProviderCredentialTaken)
	// 交易在 PostgreSQL 端已 aborted ⇒ 之後不得再下任何一句 SQL。
	if len(fx.audit.entries) != 0 {
		t.Fatalf("撞唯一鍵之後不得再寫任何一列，實得 %d", len(fx.audit.entries))
	}
}

func TestProviderAccount_PutCredentials_VersionConflict(t *testing.T) {
	fx := newProviderFixture(accountAt("INVALID_CREDENTIAL", 5))

	_, err := fx.uc.PutCredentials(context.Background(), PutProviderCredentialsInput{
		StoreID: 2685914682, SellerTaxID: "12345678", AppKey: testAppKey,
		ConsentVersion: "v1", ExpectedVersion: 4,
	})
	assertBiz(t, err, bizcode.SellerTaxProfileVersionConflict)
	if len(fx.accounts.resubmit) != 0 {
		t.Fatalf("版本不符時不得下 CAS")
	}
}

// TestProviderAccount_AppKeyNeverLeavesRepository 是明文旁路的專屬斷言。
//
// AppKey 只能出現在 repository 的寫入參數裡。回應（ProviderAccountView）、狀態歷程
// （append-only）與 ADM 稽核（5 年保存）任何一處出現它都是外洩。
func TestProviderAccount_AppKeyNeverLeavesRepository(t *testing.T) {
	fx := newProviderFixture(domain.ProviderAccount{})
	fx.accounts.getErr = notFoundErr(bizcode.InvoiceProviderAccountNotFound)

	view, err := fx.uc.PutCredentials(context.Background(), PutProviderCredentialsInput{
		StoreID: 2685914682, SellerTaxID: "12345678", AppKey: testAppKey,
		ConsentVersion: "v1", IdempotencyKey: "idem-24",
	})
	if err != nil {
		t.Fatalf("PutCredentials 失敗：%v", err)
	}

	// ① 明文確實有到 repository（否則本測試會因為「根本沒寫」而假綠）。
	if len(fx.accounts.seenAppKeys) != 1 || fx.accounts.seenAppKeys[0] != testAppKey {
		t.Fatalf("repository 應收到一次明文 AppKey，實得 %v", fx.accounts.seenAppKeys)
	}
	// ② 回應的每一個字串欄位都不得包含它。
	for name, v := range map[string]string{
		"Provider": view.Account.Provider, "ProviderTaxID": view.Account.ProviderTaxID,
		"Status": view.Account.Status, "CredentialLast4": view.Account.CredentialLast4,
	} {
		if strings.Contains(v, testAppKey) {
			t.Fatalf("回應欄位 %s 洩漏 AppKey：%q", name, v)
		}
	}
	if view.Account.StatusReason != nil && strings.Contains(*view.Account.StatusReason, testAppKey) {
		t.Fatalf("StatusReason 洩漏 AppKey")
	}
	// ③ 狀態歷程不得包含它。
	for _, e := range fx.audit.entries {
		if e.ReasonDetail != nil && strings.Contains(*e.ReasonDetail, testAppKey) {
			t.Fatalf("狀態歷程 reason_detail 洩漏 AppKey：%q", *e.ReasonDetail)
		}
	}
	// ④ ADM 稽核不得包含它（本路徑本來就不寫，但值域一旦擴大就會有人加上去）。
	for _, e := range fx.audit.admin {
		if strings.Contains(string(e.AfterState), testAppKey) || strings.Contains(e.ReasonDetail, testAppKey) {
			t.Fatalf("ADM 稽核洩漏 AppKey")
		}
	}
}

// ---------------------------------------------------------------------------
// 端點 25 測試連線
// ---------------------------------------------------------------------------

func TestProviderAccount_Verify_Success(t *testing.T) {
	fx := newProviderFixture(accountAt("CREDENTIAL_SUBMITTED", 2))

	view, err := fx.uc.Verify(context.Background(), VerifyProviderAccountInput{
		StoreID: 2685914682, ExpectedVersion: 2, IdempotencyKey: "idem-25",
	})
	if err != nil {
		t.Fatalf("Verify 失敗：%v", err)
	}
	if len(fx.accounts.startVerify) != 1 {
		t.Fatalf("應先推進 CREDENTIAL_VERIFYING 一次，實得 %d", len(fx.accounts.startVerify))
	}
	if len(fx.accounts.landed) != 1 {
		t.Fatalf("應落地一次，實得 %d", len(fx.accounts.landed))
	}
	landed := fx.accounts.landed[0]
	if landed.NextStatus != codemeta.InvoiceProviderAccountStatusActive.Value {
		t.Fatalf("三項全過應落 ACTIVE，實得 %q", landed.NextStatus)
	}
	// 🔴 落 ACTIVE 必須同時寫 verified_at：chk_ipa_active_verified 是 DB 端的等式，
	// 少寫它整句 UPDATE 會 23514。
	if landed.VerifiedAt == nil || !landed.VerifiedAt.Equal(providerNow) {
		t.Fatalf("落 ACTIVE 應寫 verified_at=now，實得 %v", landed.VerifiedAt)
	}
	// CAS 帶的是**推進之後**的版本（2 → 3）。拿舊版本去落地會 rows=0。
	if landed.ExpectedVersion != 3 {
		t.Fatalf("落地 CAS 應帶 version=3，實得 %d", landed.ExpectedVersion)
	}
	if view.Account.Status != "ACTIVE" {
		t.Fatalf("回應狀態應為 ACTIVE，實得 %q", view.Account.Status)
	}
	// 三項的第一項與第三項各打一次唯讀端點；**沒有任何寫入類外呼**。
	if len(fx.provider.verify.banCalls) != 1 || len(fx.provider.verify.trackArgs) != 1 {
		t.Fatalf("應各打一次 ban_query／track_all，實得 %d／%d",
			len(fx.provider.verify.banCalls), len(fx.provider.verify.trackArgs))
	}
	if len(fx.provider.requests) != 0 {
		t.Fatalf("驗證不得產生任何發票，實得 %d 次 f0401", len(fx.provider.requests))
	}
}

// TestProviderAccount_Verify_InvalidCredential 驗 P04：code 16 ⇒ INVALID_CREDENTIAL ＋ 9331。
func TestProviderAccount_Verify_InvalidCredential(t *testing.T) {
	fx := newProviderFixture(accountAt("CREDENTIAL_SUBMITTED", 2))
	fx.provider.verify.banErr = &amego.APIError{Code: "16", Class: amego.ClassPermanent}

	view, err := fx.uc.Verify(context.Background(), VerifyProviderAccountInput{
		StoreID: 2685914682, ExpectedVersion: 2,
	})
	assertBiz(t, err, bizcode.InvoiceProviderCredentialInvalid)
	// 🔴 狀態必須**先落地**才回碼：少了它 S03 顯示「驗證失敗」而狀態還停在驗證中，
	// 賣家看不出下一步是重打 AppKey 還是等一下再試。
	if view.Account.Status != codemeta.InvoiceProviderAccountStatusInvalidCredential.Value {
		t.Fatalf("code 16 應落 INVALID_CREDENTIAL，實得 %q", view.Account.Status)
	}
	// 失敗時 verified_at 必須沿用原值：清成 NULL 會讓已設切點的列 23514。
	if fx.accounts.landed[0].VerifiedAt == nil {
		t.Fatalf("落失敗狀態時 verified_at 不得被清空")
	}
	// track_all 不該被打（第一項就掛了）。
	if len(fx.provider.verify.trackArgs) != 0 {
		t.Fatalf("簽章檢查失敗後不得續打 track_all")
	}
}

// TestProviderAccount_Verify_ProviderRejected 驗 P05：code 13 ⇒ PROVIDER_REJECTED。
func TestProviderAccount_Verify_ProviderRejected(t *testing.T) {
	fx := newProviderFixture(accountAt("INVALID_CREDENTIAL", 2))
	fx.provider.verify.banErr = &amego.APIError{Code: "13", Class: amego.ClassManual}

	view, _ := fx.uc.Verify(context.Background(), VerifyProviderAccountInput{
		StoreID: 2685914682, ExpectedVersion: 2,
	})
	if view.Account.Status != codemeta.InvoiceProviderAccountStatusProviderRejected.Value {
		t.Fatalf("code 13 應落 PROVIDER_REJECTED，實得 %q", view.Account.Status)
	}
}

// TestProviderAccount_Verify_Unavailable 驗 P06：code 10 ⇒ PROVIDER_UNAVAILABLE ＋ 9332。
//
// 🔴 9331 與 9332 必須分得開：一個要賣家改 AppKey，一個要賣家**不要**改 AppKey。
func TestProviderAccount_Verify_Unavailable(t *testing.T) {
	fx := newProviderFixture(accountAt("CREDENTIAL_SUBMITTED", 2))
	fx.provider.verify.banErr = &amego.APIError{Code: "10", Class: amego.ClassRetryable}

	view, err := fx.uc.Verify(context.Background(), VerifyProviderAccountInput{
		StoreID: 2685914682, ExpectedVersion: 2,
	})
	assertBiz(t, err, bizcode.InvoiceProviderUnavailable)
	if view.Account.Status != codemeta.InvoiceProviderAccountStatusProviderUnavailable.Value {
		t.Fatalf("code 10 應落 PROVIDER_UNAVAILABLE，實得 %q", view.Account.Status)
	}
}

// TestProviderAccount_Verify_NoUsableTrack 驗 P03 的第三項。
//
// 少了它，賣家會通過全部檢查、看到綠色「已啟用」，然後每一筆開票都回 3040111 字軌不足
// ——那是重試多久都不會好的錯誤，而失敗高峰正好落在期初。
func TestProviderAccount_Verify_NoUsableTrack(t *testing.T) {
	fx := newProviderFixture(accountAt("CREDENTIAL_SUBMITTED", 2))
	// 有字軌列，但沒有任何一組是「API 配號」（TrackApiCode 空）。
	fx.provider.verify.tracks = []amego.Track{{Code: "AB", TrackAPICode: ""}}

	view, err := fx.uc.Verify(context.Background(), VerifyProviderAccountInput{
		StoreID: 2685914682, ExpectedVersion: 2,
	})
	assertBiz(t, err, bizcode.InvoiceProviderCredentialInvalid)
	if view.Account.Status != codemeta.InvoiceProviderAccountStatusProviderRejected.Value {
		t.Fatalf("沒有可用字軌應落 PROVIDER_REJECTED，實得 %q", view.Account.Status)
	}
}

// TestProviderAccount_Verify_EnvGuard 驗環境閘門：**status 完全不動**。
//
// ENV_GUARD 是我方的環境設定問題，不是這位賣家的憑證問題。把它記成賣家的帳號狀態，
// 會在環境修好之後留下一個假的錯誤狀態，而沒有人會回來清。
func TestProviderAccount_Verify_EnvGuard(t *testing.T) {
	fx := newProviderFixture(accountAt("ACTIVE", 4))
	fx.provider.verify.banErr = &amego.APIError{Code: amego.CodeEnvGuard, Class: amego.ClassManual}

	view, err := fx.uc.Verify(context.Background(), VerifyProviderAccountInput{
		StoreID: 2685914682, ExpectedVersion: 4,
	})
	assertBiz(t, err, bizcode.InvoiceProviderUnavailable)
	if view.Account.Status != "ACTIVE" {
		t.Fatalf("ENV_GUARD 不得改動 status，實得 %q", view.Account.Status)
	}
	if len(fx.accounts.failedActive) != 0 {
		t.Fatalf("ENV_GUARD 不得落成憑證錯誤狀態")
	}
	if len(fx.accounts.touched) != 1 {
		t.Fatalf("應只寫一次健康檢查事實，實得 %d", len(fx.accounts.touched))
	}
}

// TestProviderAccount_Verify_StateNotAllowed 驗 rows=0 ⇒ 9333，且不得續打 AMEGO。
func TestProviderAccount_Verify_StateNotAllowed(t *testing.T) {
	fx := newProviderFixture(accountAt("REVOKED", 9))
	fx.accounts.startVerifyOK = false

	_, err := fx.uc.Verify(context.Background(), VerifyProviderAccountInput{
		StoreID: 2685914682, ExpectedVersion: 9,
	})
	assertBiz(t, err, bizcode.InvoiceProviderStateInvalid)
	if len(fx.provider.verify.banCalls) != 0 {
		t.Fatalf("狀態不允許時不得打 AMEGO，實得 %d 次", len(fx.provider.verify.banCalls))
	}
	if len(fx.accounts.landed) != 0 {
		t.Fatalf("狀態不允許時不得落地")
	}
}

// ---------------------------------------------------------------------------
// 端點 26 確認啟用切點
// ---------------------------------------------------------------------------

func TestProviderAccount_Activate(t *testing.T) {
	fx := newProviderFixture(accountAt("ACTIVE", 6))
	cutover := providerNow.Add(2 * time.Hour)

	got, err := fx.uc.Activate(context.Background(), ActivateProviderAccountInput{
		StoreID: 2685914682, CutoverAt: cutover, ExpectedVersion: 6, IdempotencyKey: "idem-26",
	})
	if err != nil {
		t.Fatalf("Activate 失敗：%v", err)
	}
	if len(fx.accounts.cutovers) != 1 || fx.accounts.cutovers[0].Version != 6 {
		t.Fatalf("應以 version=6 下一次 CAS，實得 %+v", fx.accounts.cutovers)
	}
	if got.Account.AutomationCutoverAt == nil || !got.Account.AutomationCutoverAt.Equal(cutover) {
		t.Fatalf("切點未寫入")
	}
}

// TestProviderAccount_Activate_Backdated 驗硬規則③：不接受回溯。
//
// 把新設定套用到歷史訂單，等於對一批已經以別的方式處理過的訂單重新開票。
func TestProviderAccount_Activate_Backdated(t *testing.T) {
	fx := newProviderFixture(accountAt("ACTIVE", 6))

	_, err := fx.uc.Activate(context.Background(), ActivateProviderAccountInput{
		StoreID: 2685914682, CutoverAt: providerNow.Add(-time.Second), ExpectedVersion: 6,
	})
	if !errors.Is(err, ErrProviderCutoverInPast) {
		t.Fatalf("回溯切點應回 400，實得 %v", err)
	}
	if len(fx.accounts.cutovers) != 0 {
		t.Fatalf("回溯切點不得下 CAS")
	}
}

// TestProviderAccount_Activate_NotVerified 驗硬規則①：未先驗證通過 ⇒ 9333。
// fake 以 cutoverOK=false 模擬 SQL 的 `AND status='ACTIVE'` rows=0。
func TestProviderAccount_Activate_NotVerified(t *testing.T) {
	fx := newProviderFixture(accountAt("CREDENTIAL_VERIFYING", 6))
	fx.accounts.cutoverOK = false

	_, err := fx.uc.Activate(context.Background(), ActivateProviderAccountInput{
		StoreID: 2685914682, CutoverAt: providerNow.Add(time.Hour), ExpectedVersion: 6,
	})
	assertBiz(t, err, bizcode.InvoiceProviderStateInvalid)
	if len(fx.audit.entries) != 0 {
		t.Fatalf("CAS 不成立時不得寫狀態歷程")
	}
}

// ---------------------------------------------------------------------------
// 端點 27 憑證輪替
// ---------------------------------------------------------------------------

func TestProviderAccount_Rotate_Success(t *testing.T) {
	fx := newProviderFixture(accountAt("ACTIVE", 8))

	_, err := fx.uc.RotateCredentials(context.Background(), RotateProviderCredentialsInput{
		StoreID: 2685914682, AppKey: "new-app-key", ExpectedVersion: 8, IdempotencyKey: "idem-27",
	})
	if err != nil {
		t.Fatalf("RotateCredentials 失敗：%v", err)
	}
	if len(fx.accounts.rotationStarted) != 1 {
		t.Fatalf("應先進 ROTATION_PENDING（舊 key 保留），實得 %d", len(fx.accounts.rotationStarted))
	}
	if len(fx.accounts.promoted) != 1 {
		t.Fatalf("驗證通過應扶正一次，實得 %d", len(fx.accounts.promoted))
	}
	// 扶正 CAS 帶的是 ROTATION_PENDING 之後的版本（8 → 9）。
	if fx.accounts.promoted[0].ExpectedVersion != 9 {
		t.Fatalf("扶正應帶 version=9，實得 %d", fx.accounts.promoted[0].ExpectedVersion)
	}
	if len(fx.accounts.aborted) != 0 {
		t.Fatalf("成功時不得走 P13")
	}
}

// TestProviderAccount_Rotate_VerifyFails 驗 P13：**保留原 key、不覆蓋**。
//
// 覆蓋後才驗證失敗 ＝ 該賣家從那一刻起每一筆開票都失敗，而 UI 上 status 仍是 ACTIVE。
func TestProviderAccount_Rotate_VerifyFails(t *testing.T) {
	fx := newProviderFixture(accountAt("ACTIVE", 8))
	fx.provider.verify.banErr = &amego.APIError{Code: "16", Class: amego.ClassPermanent}

	view, err := fx.uc.RotateCredentials(context.Background(), RotateProviderCredentialsInput{
		StoreID: 2685914682, AppKey: "new-app-key", ExpectedVersion: 8,
	})
	assertBiz(t, err, bizcode.InvoiceProviderCredentialInvalid)
	if len(fx.accounts.promoted) != 0 {
		t.Fatalf("驗證失敗時**絕不可**扶正，實得 %d 次", len(fx.accounts.promoted))
	}
	if len(fx.accounts.aborted) != 1 {
		t.Fatalf("應走 P13 還原一次，實得 %d", len(fx.accounts.aborted))
	}
	if view.Account.Status != "ACTIVE" {
		t.Fatalf("P13 之後狀態應回 ACTIVE，實得 %q", view.Account.Status)
	}
}

// TestProviderAccount_Rotate_SameKey 驗 U3：與現行憑證相同 ⇒ 什麼都不做。
//
// 放行的後果是賣家以為換過了，而外洩的那一把還在用。
func TestProviderAccount_Rotate_SameKey(t *testing.T) {
	fx := newProviderFixture(accountAt("ACTIVE", 8))
	fx.accounts.matches = true

	_, err := fx.uc.RotateCredentials(context.Background(), RotateProviderCredentialsInput{
		StoreID: 2685914682, AppKey: testAppKey, ExpectedVersion: 8,
	})
	assertBiz(t, err, bizcode.InvoiceProviderStateInvalid)
	if len(fx.accounts.rotationStarted) != 0 || len(fx.provider.verify.banCalls) != 0 {
		t.Fatalf("同一把 key 不得寫任何欄位、也不得打 AMEGO")
	}
}

// ---------------------------------------------------------------------------
// 端點 44 ADM 主動連線檢查
// ---------------------------------------------------------------------------

// TestProviderAccount_HealthCheck_ActiveStaysActive 驗定版③：ACTIVE 通過 ⇒ status 完全不動。
func TestProviderAccount_HealthCheck_ActiveStaysActive(t *testing.T) {
	fx := newProviderFixture(accountAt("ACTIVE", 4))

	view, err := fx.uc.HealthCheckForAdmin(context.Background(), AdminProviderHealthCheckInput{
		ActorAccountID: 42, StoreID: 2685914682, IdempotencyKey: "idem-44",
	})
	if err != nil {
		t.Fatalf("HealthCheckForAdmin 失敗：%v", err)
	}
	if len(fx.accounts.startVerify) != 0 {
		t.Fatalf("ACTIVE 不得被推進 CREDENTIAL_VERIFYING（§7.3.4 沒有這條邊）")
	}
	if len(fx.accounts.touched) != 1 {
		t.Fatalf("應寫一次健康檢查事實，實得 %d", len(fx.accounts.touched))
	}
	if view.Account.Status != "ACTIVE" {
		t.Fatalf("狀態不得改變，實得 %q", view.Account.Status)
	}
	if len(fx.audit.admin) != 1 || fx.audit.admin[0].Action != providerAuditActionHealthCheck {
		t.Fatalf("應寫一列 ADM 稽核，實得 %+v", fx.audit.admin)
	}
}

// TestProviderAccount_HealthCheck_UnavailableDoesNotTouchStatus 驗定版③ 的核心那一句。
func TestProviderAccount_HealthCheck_UnavailableDoesNotTouchStatus(t *testing.T) {
	fx := newProviderFixture(accountAt("ACTIVE", 4))
	fx.provider.verify.banErr = &amego.APIError{Code: "18", Class: amego.ClassRetryable}

	view, err := fx.uc.HealthCheckForAdmin(context.Background(), AdminProviderHealthCheckInput{
		ActorAccountID: 42, StoreID: 2685914682,
	})
	assertBiz(t, err, bizcode.InvoiceProviderUnavailable)
	if view.Account.Status != "ACTIVE" {
		t.Fatalf("PROVIDER_UNAVAILABLE 不得改動 status，實得 %q", view.Account.Status)
	}
	if len(fx.accounts.failedActive) != 0 || len(fx.accounts.landed) != 0 {
		t.Fatalf("上游不可用不得落成任何錯誤狀態")
	}
}

// TestProviderAccount_HealthCheck_CredentialFaultLandsError 驗定版③ 的另一半。
func TestProviderAccount_HealthCheck_CredentialFaultLandsError(t *testing.T) {
	fx := newProviderFixture(accountAt("ACTIVE", 4))
	fx.provider.verify.banErr = &amego.APIError{Code: "12", Class: amego.ClassManual}

	view, err := fx.uc.HealthCheckForAdmin(context.Background(), AdminProviderHealthCheckInput{
		ActorAccountID: 42, StoreID: 2685914682,
	})
	assertBiz(t, err, bizcode.InvoiceProviderCredentialInvalid)
	if len(fx.accounts.failedActive) != 1 {
		t.Fatalf("憑證真的壞了應落成賣家修得掉的狀態，實得 %d", len(fx.accounts.failedActive))
	}
	if view.Account.Status != codemeta.InvoiceProviderAccountStatusInvalidCredential.Value {
		t.Fatalf("應落 INVALID_CREDENTIAL，實得 %q", view.Account.Status)
	}
}

// ---------------------------------------------------------------------------
// 端點 45 停用／端點 66 恢復／端點 67 撤銷
// ---------------------------------------------------------------------------

func TestProviderAccount_Deactivate(t *testing.T) {
	fx := newProviderFixture(accountAt("ACTIVE", 4))

	got, err := fx.uc.DeactivateForAdmin(context.Background(), AdminProviderActionInput{
		ActorAccountID: 42, StoreID: 2685914682, Reason: "AppKey 外洩",
		Confirmed: true, ExpectedVersion: 4, IdempotencyKey: "idem-45",
	})
	if err != nil {
		t.Fatalf("DeactivateForAdmin 失敗：%v", err)
	}
	// 🔴 同一交易內把該 issuer 全部未終態 operation 轉終態（§7.4 C-8）。
	if len(fx.ops.failOpen) != 1 {
		t.Fatalf("應集合式轉終態一次，實得 %d", len(fx.ops.failOpen))
	}
	if fx.ops.failOpen[0].CancelReasonCode != codemeta.InvoiceCancelReasonProviderSuspendedByAdmin.Value {
		t.Fatalf("cancel_reason_code 應為 PROVIDER_SUSPENDED_BY_ADMIN，實得 %q",
			fx.ops.failOpen[0].CancelReasonCode)
	}
	// 🔴 同一交易內也要收掉該 issuer 還沒送出去的 adjustment（#3073）。
	//
	// 少了這一句，operation 被打成 FAILED_PERMANENT 而 adjustment 留在 PENDING／SENDING
	// ——沒有執行者、沒有告警（在 invoice_adjustment_stuck_rows 出現之前），
	// dev 上實測 8 筆卡了三天，last_error 全是這條路徑寫的。
	if fx.adjs.calls != 1 {
		t.Fatalf("應收掉未送出的 adjustment 一次，實得 %d —— 它們會停在 PENDING／SENDING 而沒有執行者",
			fx.adjs.calls)
	}
	if fx.adjs.issuers[0] != 900 {
		t.Errorf("收的是 issuer %d，want 900", fx.adjs.issuers[0])
	}
	if fx.adjs.reasons[0] != codemeta.InvoiceCancelReasonProviderSuspendedByAdmin.Value {
		t.Errorf("cancel_reason_code = %q，want PROVIDER_SUSPENDED_BY_ADMIN", fx.adjs.reasons[0])
	}
	if got.PendingOrderCount != 7 {
		t.Fatalf("pendingOrderCount 應與判定同一交易讀出（7），實得 %d", got.PendingOrderCount)
	}
	if got.AffectedScope == "" {
		t.Fatalf("affectedScopeNotice 由後端產生，不得為空")
	}
	if len(fx.audit.admin) != 1 || fx.audit.admin[0].Action != providerAuditActionDeactivate {
		t.Fatalf("應寫一列 ADM 稽核")
	}
	// 稽核 before/after 只放狀態欄：那張表保存 5 年。
	if strings.Contains(string(fx.audit.admin[0].AfterState), "9f2c") {
		t.Fatalf("ADM 稽核不得含 credential_last4")
	}
}

// TestProviderAccount_Deactivate_NotConfirmed 驗 confirmed=true 是硬守衛。
func TestProviderAccount_Deactivate_NotConfirmed(t *testing.T) {
	fx := newProviderFixture(accountAt("ACTIVE", 4))

	_, err := fx.uc.DeactivateForAdmin(context.Background(), AdminProviderActionInput{
		ActorAccountID: 42, StoreID: 2685914682, Reason: "AppKey 外洩",
		Confirmed: false, ExpectedVersion: 4,
	})
	if !errors.Is(err, ErrProviderConfirmRequired) {
		t.Fatalf("confirmed=false 應回 400，實得 %v", err)
	}
	if len(fx.accounts.suspended) != 0 || len(fx.ops.failOpen) != 0 {
		t.Fatalf("未二次確認時不得有任何寫入")
	}
}

// TestProviderAccount_Deactivate_ReasonRequired 驗必填理由。
//
// chk_invadmaudit_reason 要求 length ≥ 1；空理由的稽核列等於沒有稽核。
func TestProviderAccount_Deactivate_ReasonRequired(t *testing.T) {
	fx := newProviderFixture(accountAt("ACTIVE", 4))

	_, err := fx.uc.DeactivateForAdmin(context.Background(), AdminProviderActionInput{
		ActorAccountID: 42, StoreID: 2685914682, Reason: "   ",
		Confirmed: true, ExpectedVersion: 4,
	})
	if !errors.Is(err, ErrProviderReasonRequired) {
		t.Fatalf("空白理由應回 400，實得 %v", err)
	}
	if len(fx.accounts.suspended) != 0 {
		t.Fatalf("理由不合法時不得下 CAS")
	}
}

func TestProviderAccount_Reactivate(t *testing.T) {
	fx := newProviderFixture(accountAt("SUSPENDED", 5))

	view, err := fx.uc.ReactivateForAdmin(context.Background(), AdminProviderActionInput{
		ActorAccountID: 42, StoreID: 2685914682, Reason: "已確認憑證仍有效",
		Confirmed: true, ExpectedVersion: 5, IdempotencyKey: "idem-66",
	})
	if err != nil {
		t.Fatalf("ReactivateForAdmin 失敗：%v", err)
	}
	// 🔴 恢復前強制重驗：必須先推進 CREDENTIAL_VERIFYING，不得直接回 ACTIVE。
	if len(fx.accounts.startVerify) != 1 {
		t.Fatalf("恢復前應強制重驗，實得 %d 次推進", len(fx.accounts.startVerify))
	}
	if len(fx.provider.verify.banCalls) != 1 {
		t.Fatalf("恢復必須真的打一次 AMEGO")
	}
	if view.Account.Status != "ACTIVE" {
		t.Fatalf("重驗通過後應為 ACTIVE，實得 %q", view.Account.Status)
	}
	// 被端點 45 打成 FAILED_PERMANENT 的那批**不自動復活**。
	if len(fx.ops.failOpen) != 0 {
		t.Fatalf("恢復不得碰 operation")
	}
}

// TestProviderAccount_Reactivate_WrongSource 驗 P15 的 allowed-from 只有 SUSPENDED。
func TestProviderAccount_Reactivate_WrongSource(t *testing.T) {
	fx := newProviderFixture(accountAt("ACTIVE", 5))

	_, err := fx.uc.ReactivateForAdmin(context.Background(), AdminProviderActionInput{
		ActorAccountID: 42, StoreID: 2685914682, Reason: "x", Confirmed: true, ExpectedVersion: 5,
	})
	assertBiz(t, err, bizcode.InvoiceProviderStateInvalid)
	if len(fx.provider.verify.banCalls) != 0 {
		t.Fatalf("來源不是 SUSPENDED 時不得打 AMEGO")
	}
}

func TestProviderAccount_Revoke_FromActive(t *testing.T) {
	fx := newProviderFixture(accountAt("ACTIVE", 4))

	got, err := fx.uc.RevokeForAdmin(context.Background(), AdminProviderActionInput{
		ActorAccountID: 42, StoreID: 2685914682, Reason: "賣家離場",
		Confirmed: true, ExpectedVersion: 4, IdempotencyKey: "idem-67",
	})
	if err != nil {
		t.Fatalf("RevokeForAdmin 失敗：%v", err)
	}
	if got.Status != codemeta.InvoiceProviderAccountStatusRevoked.Value {
		t.Fatalf("狀態應為 REVOKED，實得 %q", got.Status)
	}
	// 來源非 SUSPENDED ⇒ 比照 P14 打掉在途 job。
	if len(fx.ops.failOpen) != 1 {
		t.Fatalf("來源為 ACTIVE 時應集合式轉終態一次，實得 %d", len(fx.ops.failOpen))
	}
}

// TestProviderAccount_Revoke_FromSuspendedSkipsFailOpen 驗「不再打一次」。
//
// 那批 operation 已在端點 45 被打成 FAILED_PERMANENT，再打一次會把 completed_at
// 往後推、洗掉「什麼時候被停的」。
func TestProviderAccount_Revoke_FromSuspendedSkipsFailOpen(t *testing.T) {
	fx := newProviderFixture(accountAt("SUSPENDED", 4))

	if _, err := fx.uc.RevokeForAdmin(context.Background(), AdminProviderActionInput{
		ActorAccountID: 42, StoreID: 2685914682, Reason: "賣家離場",
		Confirmed: true, ExpectedVersion: 4,
	}); err != nil {
		t.Fatalf("RevokeForAdmin 失敗：%v", err)
	}
	if len(fx.ops.failOpen) != 0 {
		t.Fatalf("來源已是 SUSPENDED 時不得再打一次，實得 %d", len(fx.ops.failOpen))
	}
}

// TestProviderAccount_Revoke_AlreadyRevoked 是破壞性防護②：
// 已 REVOKED 的列撤不了第二次，而且**不會靜默成功**。
func TestProviderAccount_Revoke_AlreadyRevoked(t *testing.T) {
	fx := newProviderFixture(accountAt("REVOKED", 9))
	fx.accounts.revokeOK = false // SQL 的 allowed-from 不含 REVOKED ⇒ rows=0

	_, err := fx.uc.RevokeForAdmin(context.Background(), AdminProviderActionInput{
		ActorAccountID: 42, StoreID: 2685914682, Reason: "再撤一次",
		Confirmed: true, ExpectedVersion: 9,
	})
	assertBiz(t, err, bizcode.InvoiceProviderStateInvalid)
	if len(fx.ops.failOpen) != 0 || len(fx.audit.admin) != 0 {
		t.Fatalf("CAS 不成立時不得有任何副作用：failOpen=%d audit=%d",
			len(fx.ops.failOpen), len(fx.audit.admin))
	}
}

// TestProviderAccount_Revoke_NotFound 是破壞性防護①：
// **沒有「查不到就當可以撤銷」的分支**。
func TestProviderAccount_Revoke_NotFound(t *testing.T) {
	fx := newProviderFixture(domain.ProviderAccount{})
	fx.accounts.getErr = notFoundErr(bizcode.InvoiceProviderAccountNotFound)

	_, err := fx.uc.RevokeForAdmin(context.Background(), AdminProviderActionInput{
		ActorAccountID: 42, StoreID: 2685914682, Reason: "賣家離場",
		Confirmed: true, ExpectedVersion: 0,
	})
	assertBiz(t, err, bizcode.InvoiceProviderAccountNotFound)
	if len(fx.accounts.revoked) != 0 || len(fx.ops.failOpen) != 0 {
		t.Fatalf("查無帳號時不得有任何寫入")
	}
}

// TestProviderAccount_Revoke_VersionConflict 是破壞性防護③。
func TestProviderAccount_Revoke_VersionConflict(t *testing.T) {
	fx := newProviderFixture(accountAt("ACTIVE", 4))

	_, err := fx.uc.RevokeForAdmin(context.Background(), AdminProviderActionInput{
		ActorAccountID: 42, StoreID: 2685914682, Reason: "賣家離場",
		Confirmed: true, ExpectedVersion: 3,
	})
	assertBiz(t, err, bizcode.SellerTaxProfileVersionConflict)
	if len(fx.accounts.revoked) != 0 {
		t.Fatalf("版本不符時不得下 CAS")
	}
}

// ---------------------------------------------------------------------------
// Tier A
// ---------------------------------------------------------------------------

// TestProviderAccount_TierA_UnknownStoreIsIndistinguishable 驗越權與查無共用 9330。
//
// store_id 是 GetByRef 的述詞之一 ⇒ 別的 store_id 解出別的 issuer、拿不到這一列。
func TestProviderAccount_TierA_UnknownStoreIsIndistinguishable(t *testing.T) {
	fx := newProviderFixture(accountAt("ACTIVE", 4))
	fx.issuers.err = notFoundErr(bizcode.InvoiceProviderAccountNotFound)

	for _, tc := range []struct {
		name string
		run  func() error
	}{
		{"verify", func() error {
			_, err := fx.uc.Verify(context.Background(), VerifyProviderAccountInput{StoreID: 1, ExpectedVersion: 4})
			return err
		}},
		{"activate", func() error {
			_, err := fx.uc.Activate(context.Background(), ActivateProviderAccountInput{
				StoreID: 1, CutoverAt: providerNow.Add(time.Hour), ExpectedVersion: 4})
			return err
		}},
		{"rotate", func() error {
			_, err := fx.uc.RotateCredentials(context.Background(), RotateProviderCredentialsInput{
				StoreID: 1, AppKey: "k", ExpectedVersion: 4})
			return err
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			assertBiz(t, tc.run(), bizcode.InvoiceProviderAccountNotFound)
		})
	}
}

func assertBiz(t *testing.T, err error, code int32) {
	t.Helper()
	if err == nil {
		t.Fatalf("應回業務碼 %d，實得 nil", code)
	}
	if !isBizCode(err, code) {
		t.Fatalf("應回業務碼 %d，實得 %v", code, err)
	}
}

// ---------------------------------------------------------------------------
// 切點歷史（§5.1.15，端點 46 的唯一資料來源）
// ---------------------------------------------------------------------------

// 端點 26 必須在**同一交易**內留下一列切點歷史，否則端點 46 永遠是空清單，
// 而空清單看起來就像「這個賣家還沒設過切點」。
func TestProviderAccount_Activate_RecordsCutoverHistory(t *testing.T) {
	fx := newProviderFixture(accountAt("ACTIVE", 6))
	cutover := providerNow.Add(2 * time.Hour)

	if _, err := fx.uc.Activate(context.Background(), ActivateProviderAccountInput{
		StoreID: 2685914682, CutoverAt: cutover, ExpectedVersion: 6,
	}); err != nil {
		t.Fatalf("Activate 失敗：%v", err)
	}
	if len(fx.cutovers.rows) != 1 {
		t.Fatalf("應留下一列切點歷史，實得 %d", len(fx.cutovers.rows))
	}
	row := fx.cutovers.rows[0]
	if !row.CutoverAt.Equal(cutover) {
		t.Fatalf("切點時間不符：want %v got %v", cutover, row.CutoverAt)
	}
	// 記的是**設定之後**的版本：ADM 拿 ProviderCutoverItem.version 要對得回帳戶當時那一版。
	if row.AccountVersion != 7 {
		t.Fatalf("account_version 應為設定後的 7，實得 %d", row.AccountVersion)
	}
	// 賣家路徑的 actor 是 store surrogate（§6 身分分離的既有慣例）。
	if row.CreatedByAccountID != 2685914682 {
		t.Fatalf("created_by 應為 store surrogate，實得 %d", row.CreatedByAccountID)
	}
	if row.DeactivatedAt != nil {
		t.Fatal("剛建立的那一列必須是現行有效（deactivated_at 為 NULL）")
	}
}

// 第二次設定切點必須**先關舊列**：uq_invpc_open_per_account 只允許一列現行有效，
// 少了 CloseOpen 會撞 23505（對賣家是一句 500，不是任何業務碼）。
func TestProviderAccount_Activate_ClosesPreviousCutoverFirst(t *testing.T) {
	fx := newProviderFixture(accountAt("ACTIVE", 6))
	if _, err := fx.uc.Activate(context.Background(), ActivateProviderAccountInput{
		StoreID: 2685914682, CutoverAt: providerNow.Add(2 * time.Hour), ExpectedVersion: 6,
	}); err != nil {
		t.Fatalf("第一次 Activate 失敗：%v", err)
	}

	fx.accounts.account.Version = 7
	if _, err := fx.uc.Activate(context.Background(), ActivateProviderAccountInput{
		StoreID: 2685914682, CutoverAt: providerNow.Add(5 * time.Hour), ExpectedVersion: 7,
	}); err != nil {
		t.Fatalf("第二次 Activate 失敗：%v", err)
	}

	if len(fx.cutovers.rows) != 2 {
		t.Fatalf("應有兩列歷史，實得 %d", len(fx.cutovers.rows))
	}
	open := 0
	for _, r := range fx.cutovers.rows {
		if r.DeactivatedAt == nil {
			open++
		}
	}
	if open != 1 {
		t.Fatalf("同時只能有一列現行有效，實得 %d 列", open)
	}
	if fx.cutovers.rows[0].DeactivatedAt == nil {
		t.Fatal("舊列必須被關掉")
	}
}

// 端點 67 撤銷是 M4 終態：自動開立就此停止 ⇒ 現行那一列切點必須被關。
// 少了它，端點 46 會顯示一個「至今仍有效」的切點，而連線早就被撤銷了。
func TestProviderAccount_Revoke_ClosesOpenCutover(t *testing.T) {
	fx := newProviderFixture(accountAt("ACTIVE", 6))
	if _, err := fx.uc.Activate(context.Background(), ActivateProviderAccountInput{
		StoreID: 2685914682, CutoverAt: providerNow.Add(2 * time.Hour), ExpectedVersion: 6,
	}); err != nil {
		t.Fatalf("Activate 失敗：%v", err)
	}
	fx.accounts.account.Version = 7

	if _, err := fx.uc.RevokeForAdmin(context.Background(), AdminProviderActionInput{
		ActorAccountID: 90001, StoreID: 2685914682, Reason: "金鑰永久作廢",
		Confirmed: true, ExpectedVersion: 7,
	}); err != nil {
		t.Fatalf("RevokeForAdmin 失敗：%v", err)
	}
	for _, r := range fx.cutovers.rows {
		if r.DeactivatedAt == nil {
			t.Fatal("撤銷之後不得留下現行有效的切點列")
		}
	}
}

// 反面：端點 45 停用是**可逆的**（P15 可恢復）⇒ 不得關掉切點列。
// 關掉再開會在歷史上造出一段假的「重新設定」，而賣家從未按過端點 26。
func TestProviderAccount_Deactivate_KeepsOpenCutover(t *testing.T) {
	fx := newProviderFixture(accountAt("ACTIVE", 6))
	if _, err := fx.uc.Activate(context.Background(), ActivateProviderAccountInput{
		StoreID: 2685914682, CutoverAt: providerNow.Add(2 * time.Hour), ExpectedVersion: 6,
	}); err != nil {
		t.Fatalf("Activate 失敗：%v", err)
	}
	fx.accounts.account.Version = 7

	if _, err := fx.uc.DeactivateForAdmin(context.Background(), AdminProviderActionInput{
		ActorAccountID: 90001, StoreID: 2685914682, Reason: "暫停串接",
		Confirmed: true, ExpectedVersion: 7,
	}); err != nil {
		t.Fatalf("DeactivateForAdmin 失敗：%v", err)
	}
	if fx.cutovers.rows[0].DeactivatedAt != nil {
		t.Fatal("停用是可逆的，不得關掉切點列")
	}
}

// TestProviderOutcomeFromError_ManualCodesAreNotReportedAsTransient 守「先看 Class 再談重試」。
//
// 🔴 事故形狀（2026-09-01 dev 實測）：AMEGO 對 /json/ban_query 回
// `{"code":99,"msg":"第1筆 統一編號格式錯誤"}`，而 99 不在明確碼清單裡 ⇒ 落 default
// ⇒ 賣家看到「暫時無法連線，請稍後再試（**請勿更改 AppKey**）」。兩句話都把他推向
// 錯的下一步：這個錯誤重試一萬次也不會好，而該改的正是憑證裡的統編。
//
// 這條 production 走得到：端點 24 對統編只驗 `^[0-9]{8}$`、不驗檢查碼。
func TestProviderOutcomeFromError_ManualCodesAreNotReportedAsTransient(t *testing.T) {
	for _, tc := range []struct {
		name          string
		err           error
		wantStatus    string
		wantTransient bool // Reason 是否該說「請稍後再試」
	}{
		{
			name: "code 99（ClassManual，一碼多義）不得說成暫時性",
			err: &amego.APIError{
				Endpoint: "/json/ban_query", Code: "99",
				Msg: "第1筆 統一編號格式錯誤", Class: amego.ClassManual, KnownCode: true,
			},
			wantStatus:    codemeta.InvoiceProviderAccountStatusProviderRejected.Value,
			wantTransient: false,
		},
		{
			name: "code 23（ClassPermanent，我方送錯形狀）同樣不得說成暫時性",
			err: &amego.APIError{
				Endpoint: "/json/track_all", Code: "23",
				Msg: "data 應為陣列", Class: amego.ClassPermanent, KnownCode: true,
			},
			wantStatus:    codemeta.InvoiceProviderAccountStatusProviderRejected.Value,
			wantTransient: false,
		},
		{
			name: "code 21（ClassRetryable，人數過多）才是真的等一下再試",
			err: &amego.APIError{
				Endpoint: "/json/ban_query", Code: "21",
				Msg: "人數過多", Class: amego.ClassRetryable, KnownCode: true,
			},
			wantStatus:    codemeta.InvoiceProviderAccountStatusProviderUnavailable.Value,
			wantTransient: true,
		},
		{
			name: "TRANSPORT（連不上）也是暫時性",
			err: &amego.APIError{
				Endpoint: "/json/ban_query", Code: "TRANSPORT",
				Class: amego.ClassReconcile, Kind: amego.ErrorKindConn, KnownCode: true,
			},
			wantStatus:    codemeta.InvoiceProviderAccountStatusProviderUnavailable.Value,
			wantTransient: true,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := providerOutcomeFromError(tc.err)
			if got.NextStatus != tc.wantStatus {
				t.Fatalf("NextStatus = %q，期望 %q", got.NextStatus, tc.wantStatus)
			}
			isTransient := strings.Contains(got.Reason, "請稍後再試")
			if isTransient != tc.wantTransient {
				t.Fatalf("Reason=%q 的暫時性語氣 = %v，期望 %v", got.Reason, isTransient, tc.wantTransient)
			}
			// 非暫時性的那幾條必須把 AMEGO 的原文帶出來 —— 我方無法把「一碼多義」的 99
			// 翻成固定文案，猜一句就是把賣家推向錯的下一步。
			if !tc.wantTransient {
				var apiErr *amego.APIError
				if errors.As(tc.err, &apiErr) && apiErr.Msg != "" &&
					!strings.Contains(got.Reason, apiErr.Msg) {
					t.Fatalf("Reason=%q 應含服務商原文 %q", got.Reason, apiErr.Msg)
				}
			}
			// ACTIVE 帳號不得因為一個分不出歸屬的碼被降級。
			if got.CredentialFault {
				t.Fatalf("CredentialFault 應為 false（分不出是賣家憑證還是我方參數），實得 true")
			}
		})
	}
}
