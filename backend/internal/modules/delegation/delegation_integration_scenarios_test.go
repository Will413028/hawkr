//go:build integration

package delegation

import (
	"context"
	"encoding/json"
	"errors"
	"regexp"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"
	"uuid"

	"github.com/Will413028/hawkr/backend/internal/modules/delegation/internal/model"
	"github.com/Will413028/hawkr/backend/internal/modules/seller"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
)

func wantKind(t *testing.T, err error, want apperr.Kind) {
	t.Helper()
	if got := apperr.KindOf(err); got != want || err == nil {
		t.Fatalf("err = %v（分類 %v），預期分類 %v", err, got, want)
	}
}

func wantMessage(t *testing.T, err error, want string) {
	t.Helper()
	var ae *apperr.Error
	if !errors.As(err, &ae) || ae.Message != want {
		t.Fatalf("err = %v，預期訊息 %q", err, want)
	}
}

func sameCapabilitySet(a, b []Capability) bool {
	x, y := slices.Clone(a), slices.Clone(b)
	slices.Sort(x)
	slices.Sort(y)
	return slices.Equal(x, y)
}

func fromCapabilities(caps []Capability) []string {
	out := make([]string, len(caps))
	for i, cap := range caps {
		out[i] = string(cap)
	}
	return out
}

// 代行（設計書 56）的 integration 測試。時間由 fixture 的 f.now 控制，不用 sleep。

const testStore = int64(2685914682)

// delegationAdmin 建立一位管理員；enabled 時開通代行。
func (f *fixture) delegationAdmin(t *testing.T, email string, enabled bool) int64 {
	t.Helper()
	ctx := context.Background()
	id := f.admin(t, email)
	if enabled {
		f.grantOwner(t, id)
	}
	if err := f.m.SetAdminEligibility(ctx, AdminEligibilityChange{AdminID: id, OperatorAdminID: id, Enabled: enabled}); err != nil {
		t.Fatal(err)
	}
	return id
}

func storeOf(id int64, status seller.Status, suspended bool) seller.Store {
	var st seller.Store
	st.StoreID, st.AccountID, st.StoreName, st.Status, st.Sanctions = id, id+7, "梨山盛花園", status, sanctionsOf(suspended)
	// seller 讀店家時由 SQL 帶上的條件（Store.Conditions）：生命週期照 status，其餘都滿足。
	st.Conditions = seller.Conditions{
		Derived: true, Active: status != seller.Closed, NotPendingDeletion: status != seller.PendingDeletion, KYCApproved: true,
		StoreName: true, Logo: true, TownCode: true,
	}
	return st
}

func liveFn(id int64) (seller.Store, error) {
	return storeOf(id, seller.Active, false), nil
}

func missingFn(int64) (seller.Store, error) {
	return seller.Store{}, apperr.NotFound("店家不存在")
}

var errSellerDown = errors.New("seller 沒有回應")

func downFn(int64) (seller.Store, error) { return seller.Store{}, errSellerDown }

// wantSellerError：seller 的錯誤原樣傳出（不是拒絕，也不改成 Unavailable：seller 在行程內，Internal 就是 Internal）。
func wantSellerError(t *testing.T, err error) {
	t.Helper()
	if !errors.Is(err, errSellerDown) || apperr.KindOf(err) != apperr.KindInternal {
		t.Fatalf("err = %v（kind %v），預期原樣的 seller 錯誤、Internal", err, apperr.KindOf(err))
	}
}

func fixedFn(status seller.Status, suspended bool) func(int64) (seller.Store, error) {
	return func(id int64) (seller.Store, error) { return storeOf(id, status, suspended), nil }
}

func (f *fixture) grant(t *testing.T, admin, storeID int64, caps ...Capability) GrantedAuthorization {
	t.Helper()
	if len(caps) == 0 {
		caps = []Capability{model.CapabilityProductsRead, CapabilityProductsWrite, model.CapabilityPostsPublish, model.CapabilityOrdersRead}
	}
	g, err := f.m.GrantDelegationAuthorization(context.Background(), DelegationAuthorizationChange{
		AdminID: admin, StoreID: storeID, Source: model.AuthorizationContract, SourceReference: "CT-2026-0927",
		Capabilities: caps, OperationReason: "合約簽署",
	})
	if err != nil {
		t.Fatal(err)
	}
	return g
}

func startInput(admin, storeID int64, key string, caps ...Capability) StartDelegationInput {
	if len(caps) == 0 {
		caps = []Capability{model.CapabilityProductsRead, CapabilityProductsWrite}
	}
	return StartDelegationInput{
		AdminID: admin, StoreID: storeID, Reason: model.ReasonSellerRequest,
		CaseReference: ptr("HK-2026-0927"), Capabilities: caps, TTLSeconds: 1800, IdempotencyKey: key,
		RequestID: "req-" + key, CorrelationID: "corr-" + key, UserAgent: "integration-test",
	}
}

func (f *fixture) create(t *testing.T, in StartDelegationInput) StartedDelegation {
	t.Helper()
	s, err := f.m.StartDelegation(context.Background(), in)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

type issuedSession struct {
	StartedDelegation
	Credential string
}

func (f *fixture) start(t *testing.T, in StartDelegationInput) issuedSession {
	t.Helper()
	s := f.create(t, in)
	r, err := f.m.RedeemDelegationHandoff(context.Background(), s.HandoffCode)
	if err != nil {
		t.Fatal(err)
	}
	return issuedSession{StartedDelegation: s, Credential: r.Credential}
}

// wantDenial 確認 err 是代行的拒絕 d（對外的業務碼由 internal/legacy 對應）。
func wantDenial(t *testing.T, err error, d DelegationDenial) {
	t.Helper()
	got, ok := DelegationDenialOf(err)
	if !ok || got.Denial != d {
		t.Fatalf("err = %v，預期拒絕 %d", err, d)
	}
}

func TestDelegationLifecycle(t *testing.T) {
	f := setup(t)
	ctx := context.Background()
	admin := f.delegationAdmin(t, "ops@hawkr.example", true)
	f.grant(t, admin, testStore)

	pre, err := f.m.DelegationPreflight(ctx, DelegationPreflightInput{AdminID: admin, StoreID: testStore})
	if err != nil || !pre.Allowed || !pre.TargetKnown || pre.Target.StoreName != "梨山盛花園" || len(pre.Capabilities) != 4 ||
		pre.TTL != 30*time.Minute || pre.IdleTimeout != 15*time.Minute || pre.Actor.DisplayName != "ops@hawkr.example" {
		t.Fatalf("preflight = %+v, %v", pre, err)
	}
	started := f.create(t, startInput(admin, testStore, "ik-1"))
	s := started.Session
	if !strings.HasPrefix(started.HandoffCode, "dhof_") ||
		s.Status != model.DelegationActive || s.Revision != 0 || !regexp.MustCompile(`^OPS-[0-9A-Z]{13}$`).MatchString(s.DisplayCode) ||
		s.Target.SellerAccountID != testStore+7 || s.ReasonLabel != "賣家請求（管理員記錄）" || s.ExpiresAt.Sub(s.IssuedAt) != 30*time.Minute {
		t.Fatalf("建立 = %+v", started)
	}
	if n := f.count(t, `SELECT count(*) FROM delegation.delegated_operation_sessions WHERE credential_hash IS NULL AND handoff_code_hash = $1`,
		tokenHash(started.HandoffCode)); n != 1 {
		t.Fatalf("建立時未簽發 credential：%d 列", n)
	}
	r, err := f.m.RedeemDelegationHandoff(ctx, started.HandoffCode)
	if err != nil || !strings.HasPrefix(r.Credential, "dops_") || len(r.Credential) != 48 || r.Session.ID != s.ID {
		t.Fatalf("兌換 = %+v, %v", r, err)
	}
	if n := f.count(t, `SELECT count(*) FROM delegation.delegated_operation_sessions WHERE credential_hash = $1 AND handoff_code_hash IS NULL`, tokenHash(r.Credential)); n != 1 {
		t.Fatalf("只有兌換時的 hash 落庫：%d 列", n)
	}

	// 輪詢不寫入：連續兩次讀到逐位元相同的摘要。
	a, err := f.m.ResolveDelegation(ctx, r.Credential, false)
	if err != nil {
		t.Fatal(err)
	}
	f.now = f.now.Add(10 * time.Second)
	b, _ := f.m.ResolveDelegation(ctx, r.Credential, false)
	if a.ID != s.ID || a.Revision != b.Revision || !a.ExpiresAt.Equal(b.ExpiresAt) || a.Status != b.Status {
		t.Fatalf("兩次讀取不同：%+v %+v", a, b)
	}

	// 同一把 handoff code 只能兌換一次。
	_, err = f.m.RedeemDelegationHandoff(ctx, started.HandoffCode)
	wantDenial(t, err, DenialHandoffInvalid)
	_, err = f.m.RedeemDelegationHandoff(ctx, "garbage")
	wantDenial(t, err, DenialHandoffInvalid)

	// 結束：立刻到期、revision 加一；再結束一次、對不到的 credential 都成功。
	f.now = f.now.Add(50 * time.Second)
	if err := f.m.EndDelegation(ctx, r.Credential); err != nil {
		t.Fatal(err)
	}
	ended, err := f.m.ResolveDelegation(ctx, r.Credential, false)
	if err != nil || ended.Status != model.DelegationExpired || ended.Revision != 1 || ended.EndedAt == nil ||
		!ended.ExpiresAt.Equal(s.IssuedAt.Add(time.Minute)) {
		t.Fatalf("結束後 = %+v, %v", ended, err)
	}
	for _, c := range []string{r.Credential, "dops_unknown", ""} {
		if err := f.m.EndDelegation(ctx, c); err != nil {
			t.Fatalf("結束 %q = %v", c, err)
		}
	}
	_, err = f.m.ResolveDelegation(ctx, "dops_unknown", true)
	wantDenial(t, err, DenialCredentialInvalid)
	_, err = f.m.ResolveDelegation(ctx, "", false)
	wantDenial(t, err, DenialCredentialInvalid)
}

func TestEndingInTheFirstSecondKeepsExpiresAfterIssued(t *testing.T) {
	f := setup(t)
	ctx := context.Background()
	admin := f.delegationAdmin(t, "ops@hawkr.example", true)
	f.grant(t, admin, testStore)
	started := f.start(t, startInput(admin, testStore, "ik-1"))
	f.now = f.now.Add(300 * time.Millisecond)
	if err := f.m.EndDelegation(ctx, started.Credential); err != nil {
		t.Fatal(err)
	}
	s, _ := f.m.ResolveDelegation(ctx, started.Credential, false)
	if !s.ExpiresAt.After(s.IssuedAt) {
		t.Fatalf("expires_at %v 必須晚於 issued_at %v（前端要求差大於零）", s.ExpiresAt, s.IssuedAt)
	}
}

func TestPreflightDenials(t *testing.T) {
	f := setup(t)
	ctx := context.Background()
	admin := f.delegationAdmin(t, "ops@hawkr.example", true)
	other := f.delegationAdmin(t, "other@hawkr.example", true)
	outsider := f.delegationAdmin(t, "outsider@hawkr.example", false)
	preflight := func(adminID, storeID int64) DelegationPreflight {
		t.Helper()
		p, err := f.m.DelegationPreflight(ctx, DelegationPreflightInput{AdminID: adminID, StoreID: storeID})
		if err != nil {
			t.Fatal(err)
		}
		return p
	}
	wantDenied := func(p DelegationPreflight, d DelegationDenial) {
		t.Helper()
		// 不允許時選項與能力仍然非空（前端在看 allowed 之前就檢查）。
		if p.Allowed || p.Denial != d || p.DeniedLabel == "" || len(p.ReasonOptions) != 3 || len(p.Capabilities) == 0 {
			t.Fatalf("= %+v，預期拒絕 %d", p, d)
		}
	}

	// 沒有開通的管理員：在讀店家之前就拒絕，讀不到店家也一樣。
	f.storeFn = downFn
	p := preflight(outsider, testStore)
	wantDenied(p, DenialAdminNotAllowlisted)
	if p.TargetKnown || p.Target.StoreID != testStore || len(p.Capabilities) != 9 {
		t.Fatalf("還沒讀到店家 = %+v", p)
	}
	// 讀不到店家不是拒絕。
	_, err := f.m.DelegationPreflight(ctx, DelegationPreflightInput{AdminID: admin, StoreID: testStore})
	wantSellerError(t, err)
	f.storeFn = missingFn
	wantDenied(preflight(admin, testStore), DenialStoreNotDelegable)
	// 已關閉的店家存在：新拒絕 DenialStoreClosed（舊碼 9265 只在 legacy 轉接層），標籤是 seller 給的句子（N2），而且知道是哪家店（顯示店名與授權的集合）。
	f.storeFn = fixedFn(seller.Closed, false)
	p = preflight(admin, testStore)
	wantDenied(p, DenialStoreClosed)
	if p.DeniedLabel != "店家已關閉，無法代行。" || !p.TargetKnown || p.Target.StoreName == "" {
		t.Fatalf("已關閉 = %+v", p)
	}
	f.storeFn = liveFn
	wantDenied(preflight(admin, testStore), DenialAuthorizationNotFound)

	f.grant(t, admin, testStore, model.CapabilityPostsRead)
	for name, tc := range map[string]struct {
		fn    func(int64) (seller.Store, error)
		label string
	}{
		"停權":   {fixedFn(seller.Active, true), "帳號停權中，無法代行。"},
		"等待刪除": {fixedFn(seller.PendingDeletion, false), "店家等待刪除，無法代行。"},
	} {
		f.storeFn = tc.fn
		p = preflight(admin, testStore)
		wantDenied(p, DenialStoreNotOperable)
		if p.DeniedLabel != tc.label {
			t.Fatalf("%s：標籤 %q，預期 seller 給的句子 %q", name, p.DeniedLabel, tc.label)
		}
		if !slices.Equal(p.Capabilities, []Capability{model.CapabilityPostsRead}) {
			t.Fatalf("%s：不允許時回「會核發的集合」：%v", name, p.Capabilities)
		}
	}
	f.storeFn = liveFn
	if p := preflight(admin, testStore); !p.Allowed {
		t.Fatalf("= %+v", p)
	}

	// 已到期與已撤回都是 9264（與從來沒有開通的 9286 分開）。
	f.now = f.now.Add(time.Minute)
	until := f.now.Add(time.Hour)
	if _, err := f.m.UpdateDelegationAuthorization(ctx, DelegationAuthorizationChange{
		AdminID: admin, StoreID: testStore, Source: model.AuthorizationContract, SourceReference: "CT-2026-0927",
		Capabilities: []Capability{model.CapabilityPostsRead}, EffectiveUntil: &until, OperationReason: "合約期限",
	}); err != nil {
		t.Fatal(err)
	}
	f.now = until
	wantDenied(preflight(admin, testStore), DenialAuthorizationRevoked)

	// 管理員或店家已經在代行：9268、9269。
	const store2 = testStore + 1
	f.grant(t, admin, store2)
	f.start(t, startInput(admin, store2, "ik-1"))
	wantDenied(preflight(admin, store2), DenialActorBusy)
	wantDenied(preflight(other, store2), DenialStoreBusy)

	// 全域列：沒有店家層級的列時才用它；店家層級的列存在就遮蔽它（即使已撤回）。
	const store3, store4 = testStore + 2, testStore + 3
	if _, err := f.db.Pool().Exec(ctx, `INSERT INTO delegation.seller_delegation_authorizations
		(id, target_store_id, source, source_reference, granted_capabilities, effective_from, approved_by_admin_id, approved_at)
		VALUES ($1, NULL, 'platform_policy', 'POLICY-2026-1', '{posts.read}', $2, $3, $2)`, uuid.NewV7(), f.now, admin); err != nil {
		t.Fatal(err)
	}
	if p := preflight(other, store3); !p.Allowed || !slices.Equal(p.Capabilities, []Capability{model.CapabilityPostsRead}) {
		t.Fatalf("全域列 = %+v", p)
	}
	f.grant(t, admin, store4)
	if _, err := f.m.RevokeDelegationAuthorization(ctx, DelegationAuthorizationRevocation{AdminID: admin, StoreID: store4, Reason: "合約到期"}); err != nil {
		t.Fatal(err)
	}
	wantDenied(preflight(other, store4), DenialAuthorizationRevoked)
}

func TestStartDelegationValidatesAndReevaluatesTheGrant(t *testing.T) {
	f := setup(t)
	ctx := context.Background()
	admin := f.delegationAdmin(t, "ops@hawkr.example", true)

	_, err := f.m.StartDelegation(ctx, startInput(admin, testStore, "ik-1"))
	wantDenial(t, err, DenialAuthorizationNotFound)
	wantMessage(t, err, "此店家尚未開通代行營運")

	f.grant(t, admin, testStore, model.CapabilityProductsRead)
	invalid := map[string]func(*StartDelegationInput){
		"沒有冪等鍵":    func(in *StartDelegationInput) { in.IdempotencyKey = "" },
		"不認得的理由":   func(in *StartDelegationInput) { in.Reason = "SELLER_REQUEST" },
		"沒有案件編號":   func(in *StartDelegationInput) { in.CaseReference = nil },
		"案件編號太短":   func(in *StartDelegationInput) { in.CaseReference = ptr("HK1") },
		"沒有能力":     func(in *StartDelegationInput) { in.Capabilities = nil },
		"不認得的能力":   func(in *StartDelegationInput) { in.Capabilities = []Capability{"orders.refund"} },
		"重複的能力":    func(in *StartDelegationInput) { in.Capabilities = []Capability{"products.read", "products.read"} },
		"TTL 是零":   func(in *StartDelegationInput) { in.TTLSeconds = 0 },
		"TTL 超過上限": func(in *StartDelegationInput) { in.TTLSeconds = 1801 },
	}
	for name, mut := range invalid {
		in := startInput(admin, testStore, "ik-1", model.CapabilityProductsRead)
		mut(&in)
		_, err := f.m.StartDelegation(ctx, in)
		if d, ok := DelegationDenialOf(err); !ok || d.Denial != DenialInvalidCommand {
			t.Errorf("%s = %v，預期 9260", name, err)
		}
	}
	// 能力由伺服端重新評估：不在授權內的能力是 9262，不會被悄悄丟掉。
	_, err = f.m.StartDelegation(ctx, startInput(admin, testStore, "ik-1", model.CapabilityProductsRead, CapabilityProductsWrite))
	wantDenial(t, err, DenialCapabilityDenied)
	if n := f.count(t, `SELECT count(*) FROM delegation.delegated_operation_sessions`); n != 0 {
		t.Fatalf("拒絕的建立留下 %d 列", n)
	}
	s := f.start(t, startInput(admin, testStore, "ik-1", model.CapabilityProductsRead))
	if s.Session.Actor.DisplayName != "ops@hawkr.example" {
		t.Fatalf("沒有帶顯示名時用 email：%+v", s.Session.Actor)
	}
}

func TestStartDelegationReplay(t *testing.T) {
	f := setup(t)
	ctx := context.Background()
	admin := f.delegationAdmin(t, "ops@hawkr.example", true)
	f.grant(t, admin, testStore)
	in := startInput(admin, testStore, "ik-1")
	first := f.create(t, in)

	// 同一把 key：同一個 session，handoff code 換新、舊的當場失效。
	f.now = f.now.Add(5 * time.Second)
	in.Capabilities = []Capability{CapabilityProductsWrite, model.CapabilityProductsRead} // 順序不是內容
	again := f.create(t, in)
	if again.Session.ID != first.Session.ID || again.HandoffCode == first.HandoffCode {
		t.Fatalf("重放 = %+v", again)
	}
	_, err := f.m.RedeemDelegationHandoff(ctx, first.HandoffCode)
	wantDenial(t, err, DenialHandoffInvalid)
	redeemed, err := f.m.RedeemDelegationHandoff(ctx, again.HandoffCode)
	if err != nil || !strings.HasPrefix(redeemed.Credential, "dops_") {
		t.Fatalf("新的 handoff = %+v, %v", redeemed, err)
	}
	_, err = f.m.RedeemDelegationHandoff(ctx, again.HandoffCode)
	wantDenial(t, err, DenialHandoffInvalid)
	// 同一把 key、不同的內容。
	changed := startInput(admin, testStore, "ik-1")
	changed.CaseReference = ptr("HK-2026-0928")
	_, err = f.m.StartDelegation(ctx, changed)
	wantDenial(t, err, DenialIdempotencyKeyReused)
	// 不同的 key：管理員已經在代行（不回既有的 session）。
	_, err = f.m.StartDelegation(ctx, startInput(admin, testStore, "ik-2"))
	wantDenial(t, err, DenialActorBusy)

	// 已兌換後重放只回同一個 session，不再發出 handoff code。
	replayed := f.create(t, startInput(admin, testStore, "ik-1"))
	if replayed.Session.ID != first.Session.ID || replayed.HandoffCode != "" {
		t.Fatalf("已兌換後重放 = %+v", replayed)
	}
	_, err = f.m.RedeemDelegationHandoff(ctx, replayed.HandoffCode)
	wantDenial(t, err, DenialHandoffInvalid)
	if n := f.count(t, `SELECT count(*) FROM delegation.delegated_operation_sessions WHERE id = $1 AND credential_hash = $2 AND handoff_code_hash IS NULL`, first.Session.ID, tokenHash(redeemed.Credential)); n != 1 {
		t.Fatalf("重放後原憑證 hash 的列數 = %d", n)
	}
	if _, err := f.m.ResolveDelegation(ctx, redeemed.Credential, false); err != nil {
		t.Fatal(err)
	}
}

func TestConcurrentStartsWithOneKeyShareASession(t *testing.T) {
	f := setup(t)
	ctx := context.Background()
	admin := f.delegationAdmin(t, "ops@hawkr.example", true)
	f.grant(t, admin, testStore)
	const n = 5
	var wg sync.WaitGroup
	results := make([]StartedDelegation, n)
	errs := make([]error, n)
	for i := range n {
		wg.Go(func() { results[i], errs[i] = f.m.StartDelegation(ctx, startInput(admin, testStore, "ik-1")) })
	}
	wg.Wait()
	for i := range n {
		if errs[i] != nil || results[i].Session.ID != results[0].Session.ID {
			t.Fatalf("第 %d 個 = %+v, %v", i, results[i], errs[i])
		}
	}
	if c := f.count(t, `SELECT count(*) FROM delegation.delegated_operation_sessions`); c != 1 {
		t.Fatalf("%d 列，預期 1", c)
	}
}

func TestIdleTimeout(t *testing.T) {
	f := setup(t)
	ctx := context.Background()
	admin := f.delegationAdmin(t, "ops@hawkr.example", true)
	f.grant(t, admin, testStore)
	const store2 = testStore + 1
	f.grant(t, admin, store2)
	other := f.delegationAdmin(t, "other@hawkr.example", true)

	// 輪詢（activity=false）不算活動：15 分鐘後閒置逾時。
	polled := f.start(t, startInput(admin, testStore, "ik-1"))
	// 代行資源的路由（activity=true）每次都順延。
	active := f.start(t, startInput(other, store2, "ik-2"))
	for range 5 {
		f.now = f.now.Add(4 * time.Minute)
		if _, err := f.m.ResolveDelegation(ctx, polled.Credential, false); err != nil {
			t.Fatal(err)
		}
		if _, err := f.m.ResolveDelegation(ctx, active.Credential, true); err != nil {
			t.Fatal(err)
		}
	}
	if s, _ := f.m.ResolveDelegation(ctx, polled.Credential, false); s.Status != model.DelegationExpired {
		t.Fatalf("輪詢不得順延閒置逾時：%s", s.Status)
	}
	if s, _ := f.m.ResolveDelegation(ctx, active.Credential, false); s.Status != model.DelegationActive || s.Revision != 0 {
		t.Fatalf("代行資源的路由要順延，而且不動 revision：%+v", s)
	}
	// 已經閒置逾時的 session 不會被下一個請求救回來。
	s, err := f.m.ResolveDelegation(ctx, polled.Credential, true)
	if err != nil || s.Status != model.DelegationExpired {
		t.Fatalf("= %+v, %v", s, err)
	}
	// 閒置逾時、還沒結束的列不是衝突：preflight 允許，建立時在同一個 transaction 內把它收掉。
	if p, err := f.m.DelegationPreflight(ctx, DelegationPreflightInput{AdminID: admin, StoreID: testStore}); err != nil || !p.Allowed {
		t.Fatalf("preflight = %+v, %v", p, err)
	}
	if n := f.count(t, `SELECT count(*) FROM delegation.delegated_operation_sessions WHERE id = $1 AND ended_at IS NULL AND expires_at > $2`,
		polled.Session.ID, f.now); n != 1 {
		t.Fatal("fixture：這一列要還沒 hard 到期、也還沒結束")
	}
	again := f.start(t, startInput(admin, testStore, "ik-3"))
	if again.Session.ID == polled.Session.ID {
		t.Fatal("要建立新的 session")
	}
	if n := f.count(t, `SELECT count(*) FROM delegation.delegated_operation_sessions WHERE id = $1 AND ended_at IS NOT NULL`, polled.Session.ID); n != 1 {
		t.Fatal("閒置逾時的列要被結束，否則它繼續佔著 partial unique index")
	}
	// hard 到期不順延：一直有活動的 session 在 30 分鐘時照樣到期。
	f.now = active.Session.ExpiresAt
	if s, _ := f.m.ResolveDelegation(ctx, active.Credential, true); s.Status != model.DelegationExpired {
		t.Fatalf("hard 到期 = %s", s.Status)
	}
}

func TestRedeemHandoffRefusesAnExpiredSession(t *testing.T) {
	f := setup(t)
	ctx := context.Background()
	admin := f.delegationAdmin(t, "ops@hawkr.example", true)
	f.grant(t, admin, testStore)
	in := startInput(admin, testStore, "ik-1")
	in.TTLSeconds = 10
	s := f.create(t, in)
	// session 已經 hard 到期，handoff code 本身還在 60 秒內。
	f.now = f.now.Add(20 * time.Second)
	_, err := f.m.RedeemDelegationHandoff(ctx, s.HandoffCode)
	wantDenial(t, err, DenialHandoffInvalid)
	// 只有四個述詞的版本會兌換成功：到期的兩個述詞不可省略的證明。
	if n := f.count(t, `WITH r AS (UPDATE delegation.delegated_operation_sessions SET handoff_code_hash = NULL
		WHERE handoff_code_hash = $1 AND handoff_expires_at > $2 AND status = 'active' AND ended_at IS NULL RETURNING id)
		SELECT count(*) FROM r`, tokenHash(s.HandoffCode), f.now); n != 1 {
		t.Fatalf("四述詞版本 %d 列", n)
	}
}

func TestRevokeDelegation(t *testing.T) {
	f := setup(t)
	ctx := context.Background()
	admin := f.delegationAdmin(t, "ops@hawkr.example", true)
	// 撤銷不看 allowlist：它只會讓權限變少，也是緊急剎車。
	governor := f.delegationAdmin(t, "governor@hawkr.example", false)
	f.grant(t, admin, testStore)
	s := f.start(t, startInput(admin, testStore, "ik-1"))

	_, err := f.m.RevokeDelegation(ctx, RevokeDelegationInput{SessionID: s.Session.ID, AdminID: governor, Reason: ""})
	wantDenial(t, err, DenialInvalidCommand)
	_, err = f.m.RevokeDelegation(ctx, RevokeDelegationInput{SessionID: 999999999, AdminID: governor, Reason: "異常"})
	wantDenial(t, err, DenialSessionNotFound)

	f.now = f.now.Add(90 * time.Second)
	revoked, err := f.m.RevokeDelegation(ctx, RevokeDelegationInput{SessionID: s.Session.ID, AdminID: governor, Reason: "賣家要求中止"})
	if err != nil || revoked.Status != DelegationRevoked || revoked.Revision != 1 || revoked.RevokedAt == nil || revoked.EndedAt == nil ||
		*revoked.RevokedByAdminID != governor || *revoked.RevokeReason != "賣家要求中止" {
		t.Fatalf("撤銷 = %+v, %v", revoked, err)
	}
	// 重放：第一次的撤銷時刻，revision 不再加一。
	f.now = f.now.Add(time.Minute)
	again, err := f.m.RevokeDelegation(ctx, RevokeDelegationInput{SessionID: s.Session.ID, AdminID: admin, Reason: "再撤一次"})
	if err != nil || !again.RevokedAt.Equal(*revoked.RevokedAt) || again.Revision != 1 || *again.RevokeReason != "賣家要求中止" {
		t.Fatalf("重放 = %+v, %v", again, err)
	}
	// 撤銷寫 ended_at：摘要的時長不隨查詢時刻長大。
	first, err := f.m.DelegationSessionSummary(ctx, s.Session.ID)
	if err != nil || first.Duration != 90*time.Second || first.Session.RevokedByDisplayName == nil ||
		*first.Session.RevokedByDisplayName != "governor@hawkr.example" {
		t.Fatalf("摘要 = %+v, %v", first, err)
	}
	f.now = f.now.Add(10 * time.Minute)
	if second, _ := f.m.DelegationSessionSummary(ctx, s.Session.ID); second.Duration != first.Duration {
		t.Fatalf("時長 %v → %v", first.Duration, second.Duration)
	}
	// 代行資源的路由也救不回撤銷的 session。
	if r, err := f.m.ResolveDelegation(ctx, s.Credential, true); err != nil || r.Status != DelegationRevoked {
		t.Fatalf("= %+v, %v", r, err)
	}
}

func TestRevokeKeepsAnEarlierEnd(t *testing.T) {
	f := setup(t)
	ctx := context.Background()
	admin := f.delegationAdmin(t, "ops@hawkr.example", true)
	f.grant(t, admin, testStore)
	s := f.start(t, startInput(admin, testStore, "ik-1"))
	f.now = f.now.Add(2 * time.Minute)
	if err := f.m.EndDelegation(ctx, s.Credential); err != nil {
		t.Fatal(err)
	}
	endedAt := f.now
	f.now = f.now.Add(time.Hour)
	r, err := f.m.RevokeDelegation(ctx, RevokeDelegationInput{SessionID: s.Session.ID, AdminID: admin, Reason: "事後補治理痕跡"})
	if err != nil || r.EndedAt == nil || !r.EndedAt.Equal(endedAt) {
		t.Fatalf("撤銷已經結束的 session 保留原本的結束時刻：%+v, %v", r, err)
	}
}

func TestSummaryDurationStopsAtIdleTimeout(t *testing.T) {
	f := setup(t)
	ctx := context.Background()
	admin := f.delegationAdmin(t, "ops@hawkr.example", true)
	f.grant(t, admin, testStore)
	s := f.start(t, startInput(admin, testStore, "ik-1"))
	f.now = f.now.Add(time.Minute)
	if _, err := f.m.ResolveDelegation(ctx, s.Credential, true); err != nil {
		t.Fatal(err)
	}
	f.now = f.now.Add(20 * time.Minute)
	first, err := f.m.DelegationSessionSummary(ctx, s.Session.ID)
	if err != nil || first.Session.Status != model.DelegationExpired || first.Session.EndedAt != nil || first.Duration != time.Minute {
		t.Fatalf("右端是最後活動（1 分鐘），不是現在也不是最後活動加閒置：%+v, %v", first, err)
	}
	// 閒置逾時之後才撤銷：三個候選取最早的，仍然停在最後活動。
	if _, err := f.m.RevokeDelegation(ctx, RevokeDelegationInput{SessionID: s.Session.ID, AdminID: admin, Reason: "閒置之後撤銷"}); err != nil {
		t.Fatal(err)
	}
	if after, _ := f.m.DelegationSessionSummary(ctx, s.Session.ID); after.Duration != time.Minute || after.Session.EndedAt == nil {
		t.Fatalf("= %+v", after)
	}
}

func TestAdminSignOutRevokesItsDelegations(t *testing.T) {
	f := setup(t)
	ctx := context.Background()
	admin := f.delegationAdmin(t, "ops@hawkr.example", true)
	other := f.delegationAdmin(t, "other@hawkr.example", true)
	f.grant(t, admin, testStore)
	f.grant(t, admin, testStore+1)
	mine := f.start(t, startInput(admin, testStore, "ik-1"))
	theirs := f.start(t, startInput(other, testStore+1, "ik-2"))

	if err := f.m.RevokeDelegationsOfAdmin(ctx, admin); err != nil {
		t.Fatal(err)
	}
	s, _ := f.m.ResolveDelegation(ctx, mine.Credential, false)
	if s.Status != DelegationRevoked || *s.RevokeReason != "admin_logout" || *s.RevokedByAdminID != admin {
		t.Fatalf("登出的管理員的代行 = %+v", s)
	}
	if s, _ := f.m.ResolveDelegation(ctx, theirs.Credential, false); s.Status != model.DelegationActive {
		t.Fatalf("別人的代行不受影響：%+v", s)
	}
	// 冪等：再登出一次沒有事。
	if err := f.m.RevokeDelegationsOfAdmin(ctx, admin); err != nil {
		t.Fatal(err)
	}
}

func TestDisablingAdminEligibilityRevokesSessionsAtomically(t *testing.T) {
	f := setup(t)
	ctx := context.Background()
	admin := f.delegationAdmin(t, "ops@hawkr.example", true)
	f.grant(t, admin, testStore)
	started := f.start(t, startInput(admin, testStore, "ik-disable"))

	_, err := f.db.Pool().Exec(ctx, `
		CREATE FUNCTION delegation.reject_revoke() RETURNS trigger LANGUAGE plpgsql AS $$
		BEGIN
			IF NEW.status = 'revoked' THEN RAISE EXCEPTION 'reject revoke'; END IF;
			RETURN NEW;
		END $$`)
	if err != nil {
		t.Fatal(err)
	}
	_, err = f.db.Pool().Exec(ctx, `CREATE TRIGGER reject_revoke BEFORE UPDATE ON delegation.delegated_operation_sessions
		FOR EACH ROW EXECUTE FUNCTION delegation.reject_revoke()`)
	if err != nil {
		t.Fatal(err)
	}
	if err := f.m.SetAdminEligibility(ctx, AdminEligibilityChange{AdminID: admin, OperatorAdminID: admin}); err == nil {
		t.Fatal("撤銷 session 失敗時，資格撤回必須一起回滾")
	}
	eligible, err := f.m.session.AdminEligible(ctx, admin)
	if err != nil || !eligible {
		t.Fatalf("回滾後資格 = %v, %v", eligible, err)
	}
	session, err := f.m.ResolveDelegation(ctx, started.Credential, false)
	if err != nil || session.Status != DelegationActive {
		t.Fatalf("回滾後 session = %+v, %v", session, err)
	}
	if _, err := f.db.Pool().Exec(ctx, `DROP TRIGGER reject_revoke ON delegation.delegated_operation_sessions`); err != nil {
		t.Fatal(err)
	}
	if err := f.m.SetAdminEligibility(ctx, AdminEligibilityChange{AdminID: admin, OperatorAdminID: admin}); err != nil {
		t.Fatal(err)
	}
	eligible, err = f.m.session.AdminEligible(ctx, admin)
	if err != nil || eligible {
		t.Fatalf("撤回後資格 = %v, %v", eligible, err)
	}
	session, err = f.m.ResolveDelegation(ctx, started.Credential, false)
	if err != nil || session.Status != DelegationRevoked || session.RevokeReason == nil || *session.RevokeReason != "delegation_disabled" {
		t.Fatalf("撤回後 session = %+v, %v", session, err)
	}
}

// ---- 稽核 ----

func intentInput(sessionID int64, key string) AuditIntentInput {
	return AuditIntentInput{
		SessionID: sessionID, Capability: CapabilityProductsWrite, Action: AuditProductUpdate, ResourceType: ResourceProduct,
		ResourceID: "2701445588392017", ResourceName: "梨山水蜜桃", OperationReason: "賣家請求修正商品說明",
		IdempotencyKey: key, RequestID: "req-" + key, CorrelationID: "corr-" + key, NotifySeller: true, Origin: model.AuditInteractive,
	}
}

func (f *fixture) intent(t *testing.T, in AuditIntentInput) AuditIntent {
	t.Helper()
	i, err := f.m.RecordDelegationAuditIntent(context.Background(), in)
	if err != nil {
		t.Fatal(err)
	}
	return i
}

func (f *fixture) auditRow(t *testing.T, id uuid.UUID) AuditLog {
	t.Helper()
	p, err := f.m.DelegationAuditLogs(context.Background(), AuditLogFilter{}, DelegationPaging{PageSize: 50})
	if err != nil {
		t.Fatal(err)
	}
	for _, l := range p.Logs {
		if l.ID == id {
			return l
		}
	}
	t.Fatalf("找不到稽核 %s", id)
	return AuditLog{}
}

func TestAuditIntentAndConfirm(t *testing.T) {
	f := setup(t)
	ctx := context.Background()
	admin := f.delegationAdmin(t, "ops@hawkr.example", true)
	f.grant(t, admin, testStore)
	s := f.start(t, startInput(admin, testStore, "ik-1"))

	in := intentInput(s.Session.ID, "ik-notify")
	in.BeforeSnapshot = json.RawMessage(`{"title":"舊的"}`)
	in.ResourceVersionBefore = ptr(int64(12))
	first := f.intent(t, in)
	if first.Replayed || first.Result != model.AuditPending {
		t.Fatalf("= %+v", first)
	}
	// 同一把 key：回既有的列。
	if again := f.intent(t, in); !again.Replayed || again.AuditLogID != first.AuditLogID {
		t.Fatalf("重放 = %+v", again)
	}

	f.now = f.now.Add(3 * time.Second)
	confirmed, err := f.m.ConfirmDelegationAudit(ctx, AuditConfirmInput{
		AuditLogID: first.AuditLogID, Result: model.AuditSucceeded, ResourceVersionAfter: ptr(int64(13)),
		AfterSnapshot: json.RawMessage(`{"title":"新的"}`), ChangedFields: []ChangedField{{Field: "title", Label: "標題", Before: "舊的", After: "新的"}},
	})
	if err != nil || confirmed.Result != model.AuditSucceeded || confirmed.NotifiedAt == nil || confirmed.ConfirmedAt == nil ||
		len(confirmed.ChangedFields) != 1 || *confirmed.ResourceID != "2701445588392017" || *confirmed.ResourceVersionAfter != 13 {
		t.Fatalf("確認 = %+v, %v", confirmed, err)
	}
	// 完成事件在同一個 transaction 內發出，帶寫入當下的快照。
	if len(f.completed) != 1 {
		t.Fatalf("事件 %d 則", len(f.completed))
	}
	e := f.completed[0]
	if e.AuditLogID != first.AuditLogID || e.SessionID != s.Session.ID || e.SessionDisplayCode != s.Session.DisplayCode ||
		e.ActionLabel != "編輯商品" || *e.ResourceName != "梨山水蜜桃" || e.ActorDisplayName != "ops@hawkr.example" ||
		e.TargetSellerAccountID != testStore+7 || *e.OperationReason != "賣家請求修正商品說明" || *e.CaseReference != "HK-2026-0927" ||
		e.CorrelationID != "corr-ik-notify" || e.RiskLevel != RiskMedium || !e.NotifiedAt.Equal(*confirmed.NotifiedAt) {
		t.Fatalf("事件 = %+v", e)
	}
	// 再確認一次：既有的結論不被覆蓋，也不再發事件。
	again, err := f.m.ConfirmDelegationAudit(ctx, AuditConfirmInput{AuditLogID: first.AuditLogID, Result: model.AuditFailed, BizCode: 9280})
	if err != nil || again.Result != model.AuditSucceeded || again.BizCode != nil || len(f.completed) != 1 {
		t.Fatalf("第二次確認 = %+v, %v", again, err)
	}

	// 沒有勾選通知：稽核照寫、沒有事件（漏填的 gateway 在這裡沒有錯誤訊號，這是契約）。
	silent := intentInput(s.Session.ID, "ik-silent")
	silent.NotifySeller = false
	i := f.intent(t, silent)
	if l, err := f.m.ConfirmDelegationAudit(ctx, AuditConfirmInput{AuditLogID: i.AuditLogID, Result: model.AuditSucceeded}); err != nil || l.NotifiedAt != nil {
		t.Fatalf("= %+v, %v", l, err)
	}
	// 失敗的列記業務碼、不通知。
	failed := f.intent(t, intentInput(s.Session.ID, "ik-failed"))
	if l, err := f.m.ConfirmDelegationAudit(ctx, AuditConfirmInput{AuditLogID: failed.AuditLogID, Result: model.AuditFailed, BizCode: 9266}); err != nil ||
		*l.BizCode != 9266 || l.NotifiedAt != nil {
		t.Fatalf("= %+v, %v", l, err)
	}
	if len(f.completed) != 1 {
		t.Fatalf("事件 %d 則", len(f.completed))
	}
	_, err = f.m.ConfirmDelegationAudit(ctx, AuditConfirmInput{AuditLogID: failed.AuditLogID, Result: model.AuditPending})
	wantDenial(t, err, DenialInvalidCommand)
	_, err = f.m.ConfirmDelegationAudit(ctx, AuditConfirmInput{AuditLogID: uuid.NewV7(), Result: model.AuditSucceeded})
	wantDenial(t, err, DenialAuditIntentFailed)
}

func TestPIIRevealCannotNotifyTheSeller(t *testing.T) {
	f := setup(t)
	ctx := context.Background()
	admin := f.delegationAdmin(t, "ops@hawkr.example", true)
	f.grant(t, admin, testStore)
	s := f.start(t, startInput(admin, testStore, "ik-1", model.CapabilityOrdersRead))
	i := f.intent(t, AuditIntentInput{
		SessionID: s.Session.ID, Capability: model.CapabilityOrdersRead, Action: model.AuditOrderPIIReveal, ResourceType: model.ResourceOrder,
		ResourceID: "O123456789", OperationReason: "客服確認收件地址", RequestID: "req-pii", CorrelationID: "corr-pii",
		NotifySeller: true, Origin: model.AuditInteractive,
	})
	l, err := f.m.ConfirmDelegationAudit(ctx, AuditConfirmInput{AuditLogID: i.AuditLogID, Result: model.AuditSucceeded})
	if err != nil || l.NotifySeller || l.NotifiedAt != nil || len(f.completed) != 0 {
		t.Fatalf("PII 揭露恆不通知賣家：%+v, %v", l, err)
	}
	// 訂單以訂單號指稱，原樣存進 resource_id（原本的 bigint 存不進去而是 NULL，這一列指不回它的訂單）。
	if l.ResourceID == nil || *l.ResourceID != "O123456789" || l.ResourceName != nil || l.RiskLevel != model.RiskHigh {
		t.Fatalf("= %+v", l)
	}
}

func TestConfirmRollsBackWhenTheEventIsNotQueued(t *testing.T) {
	f := setup(t)
	ctx := context.Background()
	admin := f.delegationAdmin(t, "ops@hawkr.example", true)
	f.grant(t, admin, testStore)
	s := f.start(t, startInput(admin, testStore, "ik-1"))
	i := f.intent(t, intentInput(s.Session.ID, "ik-notify"))
	f.completedErr = errors.New("排入失敗")
	if _, err := f.m.ConfirmDelegationAudit(ctx, AuditConfirmInput{AuditLogID: i.AuditLogID, Result: model.AuditSucceeded}); err == nil {
		t.Fatal("事件排不進去時確認要失敗")
	}
	if l := f.auditRow(t, i.AuditLogID); l.Result != model.AuditPending || l.NotifiedAt != nil {
		t.Fatalf("稽核的事實與送出通知的承諾一起 commit 或 rollback：%+v", l)
	}
	f.completedErr = nil
	if _, err := f.m.ConfirmDelegationAudit(ctx, AuditConfirmInput{AuditLogID: i.AuditLogID, Result: model.AuditSucceeded}); err != nil || len(f.completed) != 1 {
		t.Fatalf("重試 = %v，事件 %d 則", err, len(f.completed))
	}
}

func TestConfirmChecksTheCapabilityOnlyOnSuccess(t *testing.T) {
	f := setup(t)
	ctx := context.Background()
	admin := f.delegationAdmin(t, "ops@hawkr.example", true)
	f.grant(t, admin, testStore)
	s := f.start(t, startInput(admin, testStore, "ik-1"))
	in := intentInput(s.Session.ID, "ik-posts")
	in.Capability, in.Action, in.ResourceType = model.CapabilityPostsPublish, model.AuditPostPublish, model.ResourcePost
	i := f.intent(t, in)
	_, err := f.m.ConfirmDelegationAudit(ctx, AuditConfirmInput{AuditLogID: i.AuditLogID, Result: model.AuditSucceeded})
	wantDenial(t, err, DenialCapabilityDenied)
	// 失敗的列正是「要了沒有授予的能力」的證據。
	if l, err := f.m.ConfirmDelegationAudit(ctx, AuditConfirmInput{AuditLogID: i.AuditLogID, Result: model.AuditFailed, BizCode: 9262}); err != nil || l.Result != model.AuditFailed {
		t.Fatalf("= %+v, %v", l, err)
	}
	// 封閉值域外的能力連 intent 都寫不進去（DB 的 CHECK 是機械兜底）。
	bad := intentInput(s.Session.ID, "ik-refund")
	bad.Capability = "orders.refund"
	if _, err := f.m.RecordDelegationAuditIntent(ctx, bad); err == nil || apperr.KindOf(err) != apperr.KindInternal {
		t.Fatalf("= %v", err)
	}
}

func TestIntentRequiresAUsableSessionAndAValidInput(t *testing.T) {
	f := setup(t)
	ctx := context.Background()
	admin := f.delegationAdmin(t, "ops@hawkr.example", true)
	f.grant(t, admin, testStore)
	f.grant(t, admin, testStore+1)
	s := f.start(t, startInput(admin, testStore, "ik-1"))

	invalid := map[string]func(*AuditIntentInput){
		"不認得的動作":  func(in *AuditIntentInput) { in.Action = "ORDER_REFUND_APPROVE" },
		"資源種類":    func(in *AuditIntentInput) { in.ResourceType = "Product" },
		"origin":  func(in *AuditIntentInput) { in.Origin = "cron" },
		"沒有能力":    func(in *AuditIntentInput) { in.Capability = "" },
		"寫入類沒有理由": func(in *AuditIntentInput) { in.OperationReason = "" },
		"理由超過一百字": func(in *AuditIntentInput) { in.OperationReason = strings.Repeat("理", 101) },
	}
	for name, mut := range invalid {
		in := intentInput(s.Session.ID, "ik-"+name)
		mut(&in)
		_, err := f.m.RecordDelegationAuditIntent(ctx, in)
		if d, ok := DelegationDenialOf(err); !ok || d.Denial != DenialInvalidCommand {
			t.Errorf("%s = %v", name, err)
		}
	}
	// 被拒絕的讀取可以沒有理由。
	read := intentInput(s.Session.ID, "")
	read.Action, read.Capability, read.OperationReason = model.AuditProductRead, model.CapabilityProductsRead, ""
	f.intent(t, read)

	_, err := f.m.RecordDelegationAuditIntent(ctx, intentInput(999999999, "ik-x"))
	wantDenial(t, err, DenialSessionNotFound)

	// 到期：互動的 intent 拒絕，排程執行照常；撤銷兩者都拒絕。
	f.now = s.Session.ExpiresAt
	_, err = f.m.RecordDelegationAuditIntent(ctx, intentInput(s.Session.ID, "ik-late"))
	wantDenial(t, err, DenialSessionExpired)
	f.now = f.now.Add(time.Minute)
	live := f.start(t, startInput(admin, testStore+1, "ik-2"))
	if _, err := f.m.RevokeDelegation(ctx, RevokeDelegationInput{SessionID: live.Session.ID, AdminID: admin, Reason: "中止"}); err != nil {
		t.Fatal(err)
	}
	_, err = f.m.RecordDelegationAuditIntent(ctx, intentInput(live.Session.ID, "ik-revoked"))
	wantDenial(t, err, DenialSessionRevoked)
	scheduled := intentInput(live.Session.ID, "schedule:1")
	scheduled.Origin, scheduled.Action, scheduled.ResourceType = model.AuditScheduled, model.AuditPostPublish, model.ResourcePost
	_, err = f.m.RecordDelegationAuditIntent(ctx, scheduled)
	wantDenial(t, err, DenialSessionRevoked)
}

func TestScheduledIntentInheritsTheReason(t *testing.T) {
	f := setup(t)
	ctx := context.Background()
	admin := f.delegationAdmin(t, "ops@hawkr.example", true)
	f.grant(t, admin, testStore)
	s := f.start(t, startInput(admin, testStore, "ik-1", model.CapabilityPostsPublish))
	const postID, orphanID, reason = "2701445588392020", "2701445588392021", "配合到貨日排定發布"
	schedule := AuditIntentInput{
		SessionID: s.Session.ID, Capability: model.CapabilityPostsPublish, Action: model.AuditPostSchedule, ResourceType: model.ResourcePost,
		ResourceID: postID, OperationReason: reason, IdempotencyKey: "ik-schedule", RequestID: "req", CorrelationID: "corr",
		NotifySeller: true, Origin: model.AuditInteractive,
	}
	i := f.intent(t, schedule)
	if _, err := f.m.ConfirmDelegationAudit(ctx, AuditConfirmInput{AuditLogID: i.AuditLogID, Result: model.AuditSucceeded}); err != nil {
		t.Fatal(err)
	}

	// 排程執行時 session 早就到期；呼叫方給的理由不採用。
	f.now = s.Session.ExpiresAt.Add(time.Hour)
	run := AuditIntentInput{
		SessionID: s.Session.ID, Capability: model.CapabilityPostsPublish, Action: model.AuditPostPublish, ResourceType: model.ResourcePost,
		ResourceID: postID, OperationReason: "捏造的理由", IdempotencyKey: "schedule:" + postID + ":1760000000",
		CorrelationID: "corr-worker", NotifySeller: true, Origin: model.AuditScheduled,
	}
	published := f.intent(t, run)
	if l := f.auditRow(t, published.AuditLogID); l.OperationReason == nil || *l.OperationReason != reason || l.Origin != model.AuditScheduled {
		t.Fatalf("繼承的理由 = %+v", l)
	}
	// 找不到可以繼承的理由：拒絕，不留下任何稽核列。
	run.ResourceID, run.IdempotencyKey = orphanID, "schedule:"+orphanID+":1760000000"
	_, err := f.m.RecordDelegationAuditIntent(ctx, run)
	wantDenial(t, err, DenialAuditIntentFailed)
	if n := f.count(t, `SELECT count(*) FROM delegation.admin_operation_audit_logs WHERE session_id = $1`, s.Session.ID); n != 2 {
		t.Fatalf("%d 列", n)
	}
}

func TestAuditBatches(t *testing.T) {
	f := setup(t)
	ctx := context.Background()
	admin := f.delegationAdmin(t, "ops@hawkr.example", true)
	f.grant(t, admin, testStore)
	s := f.start(t, startInput(admin, testStore, "ik-1"))

	bad := intentInput(s.Session.ID, "ik-b")
	bad.Action = "ORDER_REFUND_APPROVE"
	outcomes, err := f.m.RecordDelegationAuditIntents(ctx, []AuditIntentInput{
		intentInput(s.Session.ID, "ik-a"), bad, intentInput(s.Session.ID, "ik-c"),
	})
	if err != nil || len(outcomes) != 3 || outcomes[0].Err != nil || outcomes[2].Err != nil {
		t.Fatalf("逐項成敗 = %+v, %v", outcomes, err)
	}
	wantDenial(t, outcomes[1].Err, DenialInvalidCommand)
	for _, n := range []int{0, DelegationMaxBatch + 1} {
		_, err := f.m.RecordDelegationAuditIntents(ctx, make([]AuditIntentInput, n))
		wantDenial(t, err, DenialInvalidCommand)
	}

	// confirm 的批次全有或全無。
	a, c := outcomes[0].Intent.AuditLogID, outcomes[2].Intent.AuditLogID
	_, err = f.m.ConfirmDelegationAudits(ctx, []AuditConfirmInput{{AuditLogID: a, Result: model.AuditSucceeded}, {AuditLogID: c, Result: "DONE"}})
	wantDenial(t, err, DenialInvalidCommand)
	if l := f.auditRow(t, a); l.Result != model.AuditPending || l.NotifiedAt != nil {
		t.Fatal("失敗的批次不得留下部分確認")
	}
	f.completed = nil // 訂閱者在 rollback 的 transaction 內收到過第一項；真正的訂閱者排入的 job 跟著 rollback
	logs, err := f.m.ConfirmDelegationAudits(ctx, []AuditConfirmInput{{AuditLogID: a, Result: model.AuditSucceeded}, {AuditLogID: c, Result: model.AuditSucceeded}})
	if err != nil || len(logs) != 2 || logs[0].ID != a || logs[1].Result != model.AuditSucceeded || len(f.completed) != 2 {
		t.Fatalf("= %+v, %v，事件 %d 則", logs, err, len(f.completed))
	}
}

func TestAuditLogsAndSummary(t *testing.T) {
	f := setup(t)
	ctx := context.Background()
	admin := f.delegationAdmin(t, "ops@hawkr.example", true)
	other := f.delegationAdmin(t, "other@hawkr.example", true)
	f.grant(t, admin, testStore)
	f.grant(t, admin, testStore+1)
	s := f.start(t, startInput(admin, testStore, "ik-1"))
	o := f.start(t, startInput(other, testStore+1, "ik-2"))

	i := f.intent(t, intentInput(s.Session.ID, "ik-a"))
	if _, err := f.m.ConfirmDelegationAudit(ctx, AuditConfirmInput{AuditLogID: i.AuditLogID, Result: model.AuditSucceeded}); err != nil {
		t.Fatal(err)
	}
	f.now = f.now.Add(time.Second)
	publish := intentInput(s.Session.ID, "ik-b")
	publish.Action = model.AuditProductPublish
	p := f.intent(t, publish)
	if _, err := f.m.ConfirmDelegationAudit(ctx, AuditConfirmInput{AuditLogID: p.AuditLogID, Result: model.AuditSucceeded}); err != nil {
		t.Fatal(err)
	}
	f.intent(t, intentInput(o.Session.ID, "ik-c"))

	page, err := f.m.DelegationAuditLogs(ctx, AuditLogFilter{SessionID: &s.Session.ID}, DelegationPaging{})
	if err != nil || page.Total != 2 || len(page.Logs) != 2 || page.Logs[0].ID != p.AuditLogID || page.Paging.PageSize != 20 {
		t.Fatalf("時間線（新到舊）= %+v, %v", page, err)
	}
	if page, _ := f.m.DelegationAuditLogs(ctx, AuditLogFilter{SessionID: &s.Session.ID, Action: model.AuditProductPublish}, DelegationPaging{}); page.Total != 1 {
		t.Fatalf("篩選動作 = %+v", page)
	}
	if page, _ := f.m.DelegationAuditLogs(ctx, AuditLogFilter{AdminID: &other}, DelegationPaging{}); page.Total != 1 || page.Logs[0].Actor.AdminID != other {
		t.Fatalf("跨 session 以管理員篩選 = %+v", page)
	}
	if page, _ := f.m.DelegationAuditLogs(ctx, AuditLogFilter{Result: model.AuditPending}, DelegationPaging{Page: 1, PageSize: 1}); page.Total != 1 || len(page.Logs) != 1 {
		t.Fatalf("= %+v", page)
	}
	from := f.now.Add(time.Hour)
	if page, _ := f.m.DelegationAuditLogs(ctx, AuditLogFilter{From: &from}, DelegationPaging{}); page.Total != 0 {
		t.Fatalf("時間區間 = %+v", page)
	}
	for name, filter := range map[string]AuditLogFilter{
		"動作": {Action: "SHIP"}, "資源": {ResourceType: "sku"}, "結果": {Result: "DONE"}, "店家": {StoreID: ptr(int64(0))},
		"區間": {From: &from, To: ptr(f.now)},
	} {
		_, err := f.m.DelegationAuditLogs(ctx, filter, DelegationPaging{})
		if d, ok := DelegationDenialOf(err); !ok || d.Denial != DenialInvalidCommand {
			t.Errorf("%s = %v", name, err)
		}
	}
	missing := int64(999999999)
	_, err = f.m.DelegationAuditLogs(ctx, AuditLogFilter{SessionID: &missing}, DelegationPaging{})
	wantDenial(t, err, DenialSessionNotFound)

	sum, err := f.m.DelegationSessionSummary(ctx, s.Session.ID)
	if err != nil || sum.OperationCount != 2 || sum.RiskOperationCount != 1 || sum.PendingAuditCount != 0 || !sum.NotifiedSeller ||
		sum.FulfillmentActionCount != 0 || sum.FinanceActionCount != 0 || sum.AuditRetentionDays != 730 || sum.Session.Status != model.DelegationActive {
		t.Fatalf("摘要 = %+v, %v", sum, err)
	}
	// 沒有登錄的動作寫不進稽核表（設計書 56 §6.5.3），履約、金流的計數因此結構上恆為 0。
	insert := func(action, result string) error {
		_, err := f.db.Pool().Exec(ctx, `INSERT INTO delegation.admin_operation_audit_logs
			(id, session_id, actor_admin_id, actor_display_name, target_store_id, target_seller_account_id, target_store_name,
			 origin, capability, action, resource_type, result, request_id, correlation_id, risk_level, notify_seller,
			 created_at, confirmed_at, biz_code)
			VALUES ($1, $2, $3, 'x', $4, 0, 'x', 'interactive', 'orders.read', $5, 'order', $6, 'r', 'c', 'high', false,
			        $7, CASE WHEN $6 = 'PENDING' THEN NULL ELSE $7::timestamptz END, CASE WHEN $6 = 'FAILED' THEN 9280 END)`,
			uuid.NewV7(), s.Session.ID, admin, testStore, action, result, f.now)
		return err
	}
	if err := insert("ORDER_SHIPMENT_CREATE", "SUCCEEDED"); err == nil || !strings.Contains(err.Error(), "admin_operation_audit_logs_action_check") {
		t.Fatalf("沒有登錄的動作 = %v", err)
	}
	// 但計數是數出來的，不是寫死的零：模擬封鎖被打穿（拿掉 CHECK）時，那些列要被數到，PENDING 的也算。
	if _, err := f.db.Pool().Exec(ctx, `ALTER TABLE delegation.admin_operation_audit_logs DROP CONSTRAINT admin_operation_audit_logs_action_check`); err != nil {
		t.Fatal(err)
	}
	for _, r := range []struct{ action, result string }{
		{"ORDER_SHIPMENT_CREATE", "SUCCEEDED"}, {"ORDER_REFUND_APPROVE", "FAILED"}, {"ORDER_FULFILLMENT_CONFIRM", "PENDING"},
	} {
		if err := insert(r.action, r.result); err != nil {
			t.Fatal(err)
		}
	}
	sum, _ = f.m.DelegationSessionSummary(ctx, s.Session.ID)
	if sum.FulfillmentActionCount != 2 || sum.FinanceActionCount != 1 || sum.OperationCount != 3 || sum.PendingAuditCount != 1 {
		t.Fatalf("= %+v", sum)
	}
	_, err = f.m.DelegationSessionSummary(ctx, missing)
	wantDenial(t, err, DenialSessionNotFound)
}

func TestListDelegationSessions(t *testing.T) {
	f := setup(t)
	ctx := context.Background()
	admin := f.delegationAdmin(t, "ops@hawkr.example", true)
	other := f.delegationAdmin(t, "other@hawkr.example", true)
	f.grant(t, admin, testStore)
	f.grant(t, admin, testStore+1)
	a := f.start(t, startInput(admin, testStore, "ik-1"))
	f.now = f.now.Add(time.Second)
	b := f.start(t, startInput(other, testStore+1, "ik-2"))
	if _, err := f.m.RevokeDelegation(ctx, RevokeDelegationInput{SessionID: a.Session.ID, AdminID: other, Reason: "中止"}); err != nil {
		t.Fatal(err)
	}

	page, err := f.m.DelegationSessions(ctx, DelegationSessionFilter{}, DelegationPaging{})
	if err != nil || page.Total != 2 || page.Sessions[0].ID != b.Session.ID || page.Sessions[1].Status != DelegationRevoked ||
		*page.Sessions[1].RevokedByDisplayName != "other@hawkr.example" || page.Sessions[0].RevokedByDisplayName != nil {
		t.Fatalf("= %+v, %v", page, err)
	}
	if page, _ := f.m.DelegationSessions(ctx, DelegationSessionFilter{DisplayCode: b.Session.DisplayCode}, DelegationPaging{}); page.Total != 1 {
		t.Fatalf("以代號 = %+v", page)
	}
	if page, _ := f.m.DelegationSessions(ctx, DelegationSessionFilter{StoreID: ptr(testStore)}, DelegationPaging{}); page.Total != 1 || page.Sessions[0].ID != a.Session.ID {
		t.Fatalf("以店家 = %+v", page)
	}
	_, err = f.m.DelegationSessions(ctx, DelegationSessionFilter{DisplayCode: "OPS-123"}, DelegationPaging{})
	wantDenial(t, err, DenialInvalidCommand)
}

func TestMissingRevokerDoesNotBreakSessionReads(t *testing.T) {
	f := setup(t)
	ctx := context.Background()
	admin := f.delegationAdmin(t, "ops@hawkr.example", true)
	revoker := f.delegationAdmin(t, "revoker@hawkr.example", true)
	f.grant(t, admin, testStore)
	s := f.start(t, startInput(admin, testStore, "ik-1"))
	if _, err := f.m.RevokeDelegation(ctx, RevokeDelegationInput{SessionID: s.Session.ID, AdminID: revoker, Reason: "中止"}); err != nil {
		t.Fatal(err)
	}
	f.deleteAdmin(t, revoker)
	summary, err := f.m.DelegationSessionSummary(ctx, s.Session.ID)
	if err != nil || summary.Session.RevokedByDisplayName != nil {
		t.Fatalf("摘要 = %+v, %v", summary, err)
	}
	page, err := f.m.DelegationSessions(ctx, DelegationSessionFilter{}, DelegationPaging{})
	if err != nil || page.Total != 1 || page.Sessions[0].RevokedByDisplayName != nil {
		t.Fatalf("清單 = %+v, %v", page, err)
	}
}

// ---- 授權的治理 ----

func (f *fixture) history(t *testing.T, admin, storeID int64) AuthorizationHistoryPage {
	t.Helper()
	h, err := f.m.DelegationAuthorizationHistory(context.Background(), DelegationAuthorizationHistoryInput{AdminID: admin, StoreID: storeID, Paging: DelegationPaging{}})
	if err != nil {
		t.Fatal(err)
	}
	return h
}

func change(admin, storeID int64, key string, caps ...Capability) DelegationAuthorizationChange {
	return DelegationAuthorizationChange{
		AdminID: admin, StoreID: storeID, Source: model.AuthorizationContract, SourceReference: "CT-2026-0927",
		Capabilities: caps, OperationReason: "合約簽署", IdempotencyKey: key, RequestID: "req", CorrelationID: "corr",
	}
}

func TestGrantAndReinstateUseOneRow(t *testing.T) {
	f := setup(t)
	ctx := context.Background()
	admin := f.delegationAdmin(t, "ops@hawkr.example", true)
	first, err := f.m.GrantDelegationAuthorization(ctx, change(admin, testStore, "g1", model.CapabilityProductsRead, CapabilityProductsWrite))
	if err != nil || first.Outcome != model.GrantCreated || !first.Authorization.Effective || first.Authorization.ApprovedByDisplayName != "ops@hawkr.example" {
		t.Fatalf("開通 = %+v, %v", first, err)
	}
	// 已經有有效的授權：9285，而且 transaction 沒有中止（不是接唯一鍵的錯誤）。
	_, err = f.m.GrantDelegationAuthorization(ctx, change(admin, testStore, "g2", model.CapabilityPostsRead))
	wantDenial(t, err, DenialAuthorizationAlreadyActive)
	if p, _ := f.m.DelegationAuthorizationOfStore(ctx, admin, testStore); !p.Exists || !slices.Equal(p.Authorization.Capabilities, first.Authorization.Capabilities) {
		t.Fatalf("= %+v", p)
	}

	f.now = f.now.Add(time.Minute)
	if _, err := f.m.RevokeDelegationAuthorization(ctx, DelegationAuthorizationRevocation{AdminID: admin, StoreID: testStore, Reason: "合約到期", IdempotencyKey: "r1"}); err != nil {
		t.Fatal(err)
	}
	f.now = f.now.Add(time.Minute)
	second, err := f.m.GrantDelegationAuthorization(ctx, change(admin, testStore, "g3", model.CapabilityPostsRead))
	if err != nil || second.Outcome != model.GrantReinstated || second.Authorization.ID != first.Authorization.ID ||
		second.Authorization.RevokedAt != nil || second.Authorization.RevokeReason != nil || !second.Authorization.EffectiveFrom.Equal(f.now) {
		t.Fatalf("重新開通是同一列、撤回的欄位清空、生效時間重設：%+v, %v", second, err)
	}
	h := f.history(t, admin, testStore)
	actions := []model.AuthorizationAction{}
	for _, e := range h.Entries {
		actions = append(actions, e.Action)
	}
	if !slices.Equal(actions, []model.AuthorizationAction{model.AuthorizationReinstate, model.AuthorizationRevoke, model.AuthorizationGrant}) {
		t.Fatalf("歷史 = %v", actions)
	}
	if h.Entries[2].CapabilitiesBefore != nil || !slices.Equal(h.Entries[0].CapabilitiesBefore, []Capability{model.CapabilityProductsRead, CapabilityProductsWrite}) ||
		h.Entries[1].Source != nil || *h.Entries[0].Source != model.AuthorizationContract || h.Entries[0].ActorDisplayName != "ops@hawkr.example" {
		t.Fatalf("歷史 = %+v", h.Entries)
	}
}

func TestMissingApproverDoesNotRollBackAuthorizationWrites(t *testing.T) {
	f := setup(t)
	ctx := context.Background()
	approver := f.delegationAdmin(t, "approver@hawkr.example", true)
	governor := f.delegationAdmin(t, "governor@hawkr.example", true)
	f.grant(t, approver, testStore)
	f.deleteAdmin(t, approver)
	updated, err := f.m.UpdateDelegationAuthorization(ctx, change(governor, testStore, "update", model.CapabilityPostsRead))
	if err != nil || updated.ApprovedByDisplayName != "" {
		t.Fatalf("更新 = %+v, %v", updated, err)
	}
	page, err := f.m.DelegationAuthorizations(ctx, ListDelegationAuthorizationsInput{AdminID: governor})
	if err != nil || page.Total != 1 || page.Authorizations[0].ApprovedByDisplayName != "" {
		t.Fatalf("清單 = %+v, %v", page, err)
	}
	if got := f.count(t, `SELECT count(*) FROM delegation.delegation_authorization_audit_logs WHERE target_store_id = $1`, testStore); got != 2 {
		t.Fatalf("治理歷史 %d 列，預期 2", got)
	}
}

func TestAuthorizationReplays(t *testing.T) {
	f := setup(t)
	ctx := context.Background()
	admin := f.delegationAdmin(t, "ops@hawkr.example", true)
	in := change(admin, testStore, "g1", model.CapabilityProductsRead)
	first, err := f.m.GrantDelegationAuthorization(ctx, in)
	if err != nil {
		t.Fatal(err)
	}
	// 同一把 key 重放：第一次的答案（不是 9285），不多寫一列歷史。
	in.OperationReason = "換個說法重試" // 理由不是內容
	if replay, err := f.m.GrantDelegationAuthorization(ctx, in); err != nil || replay.Outcome != model.GrantCreated || replay.Authorization.ID != first.Authorization.ID {
		t.Fatalf("重放 = %+v, %v", replay, err)
	}
	// 同一把 key、不同的內容：9267。
	_, err = f.m.GrantDelegationAuthorization(ctx, change(admin, testStore, "g1", model.CapabilityPostsRead))
	wantDenial(t, err, DenialIdempotencyKeyReused)

	// 調整的重放不再套用一次（updated_at 不動、不多一列歷史）。
	f.now = f.now.Add(time.Minute)
	upd := change(admin, testStore, "u1", model.CapabilityOrdersRead, model.CapabilityPostsRead)
	updated, err := f.m.UpdateDelegationAuthorization(ctx, upd)
	if err != nil || !sameCapabilitySet(updated.Capabilities, upd.Capabilities) {
		t.Fatalf("調整 = %+v, %v", updated, err)
	}
	f.now = f.now.Add(time.Minute)
	if replay, err := f.m.UpdateDelegationAuthorization(ctx, upd); err != nil || !replay.UpdatedAt.Equal(updated.UpdatedAt) {
		t.Fatalf("調整的重放 = %+v, %v", replay, err)
	}
	// 用過開通的 key 來調整：9267，整個調整 rollback。
	_, err = f.m.UpdateDelegationAuthorization(ctx, change(admin, testStore, "g1", model.CapabilityOrdersRead))
	wantDenial(t, err, DenialIdempotencyKeyReused)
	// 再調整一次（新的 key）：歷史的 before 是真的變更前。
	if _, err := f.m.UpdateDelegationAuthorization(ctx, change(admin, testStore, "u2", model.CapabilityOrdersRead)); err != nil {
		t.Fatal(err)
	}
	h := f.history(t, admin, testStore)
	if h.Total != 3 || !sameCapabilitySet(h.Entries[0].CapabilitiesBefore, upd.Capabilities) ||
		!slices.Equal(h.Entries[1].CapabilitiesBefore, []Capability{model.CapabilityProductsRead}) {
		t.Fatalf("歷史 = %+v", h)
	}

	// 撤回的重放：已經撤回、保留第一次、只有一列 REVOKE。
	f.now = f.now.Add(time.Minute)
	r1, err := f.m.RevokeDelegationAuthorization(ctx, DelegationAuthorizationRevocation{AdminID: admin, StoreID: testStore, Reason: "合約到期", IdempotencyKey: "r1"})
	if err != nil || r1.AlreadyRevoked || r1.RevokedSessions == nil || len(r1.RevokedSessions) != 0 {
		t.Fatalf("撤回 = %+v, %v", r1, err)
	}
	f.now = f.now.Add(time.Minute)
	r2, err := f.m.RevokeDelegationAuthorization(ctx, DelegationAuthorizationRevocation{AdminID: admin, StoreID: testStore, Reason: "另一個理由", IdempotencyKey: "r2"})
	if err != nil || !r2.AlreadyRevoked || !r2.Authorization.RevokedAt.Equal(*r1.Authorization.RevokedAt) || *r2.Authorization.RevokeReason != "合約到期" ||
		*r2.Authorization.RevokedByDisplayName != "ops@hawkr.example" {
		t.Fatalf("第二次撤回 = %+v, %v", r2, err)
	}
	if h := f.history(t, admin, testStore); h.Total != 4 {
		t.Fatalf("歷史 %d 列", h.Total)
	}
}

func TestUpdateTellsMissingFromRevoked(t *testing.T) {
	f := setup(t)
	ctx := context.Background()
	admin := f.delegationAdmin(t, "ops@hawkr.example", true)
	_, err := f.m.UpdateDelegationAuthorization(ctx, change(admin, testStore, "", model.CapabilityPostsRead))
	wantDenial(t, err, DenialAuthorizationNotFound)
	f.grant(t, admin, testStore)
	past := f.now.Add(-time.Hour)
	in := change(admin, testStore, "", model.CapabilityPostsRead)
	in.EffectiveUntil = &past
	_, err = f.m.UpdateDelegationAuthorization(ctx, in)
	wantDenial(t, err, DenialInvalidCommand)
	if _, err := f.m.RevokeDelegationAuthorization(ctx, DelegationAuthorizationRevocation{AdminID: admin, StoreID: testStore, Reason: "撤回"}); err != nil {
		t.Fatal(err)
	}
	_, err = f.m.UpdateDelegationAuthorization(ctx, change(admin, testStore, "", model.CapabilityPostsRead))
	wantDenial(t, err, DenialAuthorizationRevoked)
}

func TestRevokingTheAuthorizationEndsRunningSessions(t *testing.T) {
	f := setup(t)
	ctx := context.Background()
	admin := f.delegationAdmin(t, "ops@hawkr.example", true)
	governor := f.delegationAdmin(t, "governor@hawkr.example", true)
	f.grant(t, admin, testStore)
	live := f.start(t, startInput(admin, testStore, "ik-1"))
	if p, _ := f.m.DelegationAuthorizationOfStore(ctx, governor, testStore); p.ActiveSessions != 1 {
		t.Fatalf("= %+v", p)
	}
	f.now = f.now.Add(90 * time.Second)
	out, err := f.m.RevokeDelegationAuthorization(ctx, DelegationAuthorizationRevocation{AdminID: governor, StoreID: testStore, Reason: "賣家終止合作"})
	if err != nil || len(out.RevokedSessions) != 1 || out.RevokedSessions[0] != live.Session.ID {
		t.Fatalf("= %+v, %v", out, err)
	}
	s, _ := f.m.ResolveDelegation(ctx, live.Credential, false)
	if s.Status != DelegationRevoked || *s.RevokeReason != "authorization_revoked" || *s.RevokedByAdminID != governor || s.EndedAt == nil {
		t.Fatalf("連帶撤銷 = %+v", s)
	}
	// 由 session 找回是哪一次治理動作：同一個時刻。
	if n := f.count(t, `SELECT count(*) FROM delegation.delegation_authorization_audit_logs
		WHERE target_store_id = $1 AND action = 'REVOKE' AND date_trunc('second', created_at) = $2`, testStore, *s.RevokedAt); n != 1 {
		t.Fatal("撤回的歷史與 session 的 revoked_at 要是同一個時刻")
	}
	if p, _ := f.m.DelegationAuthorizationOfStore(ctx, governor, testStore); p.ActiveSessions != 0 || !p.Exists || p.Authorization.Effective {
		t.Fatalf("= %+v", p)
	}
}

func TestAuthorizationAndItsHistoryCommitTogether(t *testing.T) {
	f := setup(t)
	ctx := context.Background()
	admin := f.delegationAdmin(t, "ops@hawkr.example", true)
	for _, ddl := range []string{
		`CREATE FUNCTION delegation.refuse_history() RETURNS trigger LANGUAGE plpgsql AS $$
		 BEGIN RAISE EXCEPTION 'audit insert refused by test'; END $$`,
		`CREATE TRIGGER refuse_history BEFORE INSERT ON delegation.delegation_authorization_audit_logs
		 FOR EACH ROW EXECUTE FUNCTION delegation.refuse_history()`,
	} {
		if _, err := f.db.Pool().Exec(ctx, ddl); err != nil {
			t.Fatal(err)
		}
	}
	_, err := f.m.GrantDelegationAuthorization(ctx, change(admin, testStore, "g1", model.CapabilityPostsRead))
	if err == nil || !strings.Contains(err.Error(), "audit insert refused by test") {
		t.Fatalf("= %v", err)
	}
	if n := f.count(t, `SELECT count(*) FROM delegation.seller_delegation_authorizations`); n != 0 {
		t.Fatal("歷史寫不進去時授權不得留下")
	}
}

func TestGovernanceNeedsAnAllowlistedAdmin(t *testing.T) {
	f := setup(t)
	ctx := context.Background()
	outsider := f.delegationAdmin(t, "outsider@hawkr.example", false)
	calls := map[string]error{}
	_, calls["list"] = f.m.DelegationAuthorizations(ctx, ListDelegationAuthorizationsInput{AdminID: outsider, Filter: DelegationAuthorizationFilter{}, Paging: DelegationPaging{}})
	_, calls["get"] = f.m.DelegationAuthorizationOfStore(ctx, outsider, testStore)
	_, calls["grant"] = f.m.GrantDelegationAuthorization(ctx, change(outsider, testStore, "", model.CapabilityPostsRead))
	_, calls["update"] = f.m.UpdateDelegationAuthorization(ctx, change(outsider, testStore, "", model.CapabilityPostsRead))
	_, calls["revoke"] = f.m.RevokeDelegationAuthorization(ctx, DelegationAuthorizationRevocation{AdminID: outsider, StoreID: testStore, Reason: "x"})
	_, calls["history"] = f.m.DelegationAuthorizationHistory(ctx, DelegationAuthorizationHistoryInput{AdminID: outsider, StoreID: testStore, Paging: DelegationPaging{}})
	for name, err := range calls {
		if d, ok := DelegationDenialOf(err); !ok || d.Denial != DenialAdminNotAllowlisted {
			t.Errorf("%s = %v，預期 9288", name, err)
		}
	}
	_, err := f.m.DelegationAuthorizations(ctx, ListDelegationAuthorizationsInput{AdminID: 0, Filter: DelegationAuthorizationFilter{}, Paging: DelegationPaging{}})
	wantDenial(t, err, DenialAdminRequired)
}

func TestGrantNeedsAStoreThatExists(t *testing.T) {
	f := setup(t)
	ctx := context.Background()
	admin := f.delegationAdmin(t, "ops@hawkr.example", true)
	f.storeFn = missingFn
	_, err := f.m.GrantDelegationAuthorization(ctx, change(admin, testStore, "", model.CapabilityPostsRead))
	wantDenial(t, err, DenialStoreNotDelegable)
	// 已關閉：開通授權也是 DenialStoreClosed，訊息是同一句（N2）；店家不存在才是 DenialStoreNotDelegable。
	f.storeFn = fixedFn(seller.Closed, false)
	_, err = f.m.GrantDelegationAuthorization(ctx, change(admin, testStore, "", model.CapabilityPostsRead))
	wantDenial(t, err, DenialStoreClosed)
	if d, _ := DelegationDenialOf(err); d == nil || d.Message() != "店家已關閉，無法代行。" {
		t.Fatalf("開通授權對已關閉的店家：%v", err)
	}
	f.storeFn = downFn
	_, err = f.m.GrantDelegationAuthorization(ctx, change(admin, testStore, "", model.CapabilityPostsRead))
	wantSellerError(t, err)
	f.storeFn = liveFn
	past := f.now
	in := change(admin, testStore, "", model.CapabilityPostsRead)
	in.EffectiveUntil = &past
	_, err = f.m.GrantDelegationAuthorization(ctx, in)
	wantDenial(t, err, DenialInvalidCommand)
	_, err = f.m.DelegationAuthorizationHistory(ctx, DelegationAuthorizationHistoryInput{AdminID: admin, StoreID: testStore, Paging: DelegationPaging{}})
	wantDenial(t, err, DenialAuthorizationNotFound)
	if p, err := f.m.DelegationAuthorizationOfStore(ctx, admin, testStore); err != nil || p.Exists {
		t.Fatalf("沒有開通不是錯誤：%+v, %v", p, err)
	}
}

func TestStatusFilterIsATotalPartition(t *testing.T) {
	f := setup(t)
	ctx := context.Background()
	admin := f.delegationAdmin(t, "ops@hawkr.example", true)
	f.grant(t, admin, testStore)   // 有效
	f.grant(t, admin, testStore+1) // 撤回
	until := f.now.Add(time.Hour)  // 到期
	in := change(admin, testStore+2, "", model.CapabilityPostsRead)
	in.EffectiveUntil = &until
	if _, err := f.m.GrantDelegationAuthorization(ctx, in); err != nil {
		t.Fatal(err)
	}
	if _, err := f.m.RevokeDelegationAuthorization(ctx, DelegationAuthorizationRevocation{AdminID: admin, StoreID: testStore + 1, Reason: "撤回"}); err != nil {
		t.Fatal(err)
	}
	// 生效時間在未來的列（維運 SQL 仍可能建）：兩分法不能漏掉它。
	if _, err := f.db.Pool().Exec(ctx, `INSERT INTO delegation.seller_delegation_authorizations
		(id, target_store_id, source, source_reference, granted_capabilities, effective_from, approved_by_admin_id, approved_at)
		VALUES ($1, $2, 'ticket', 'TICKET-2026-1', '{posts.read}', $3, $4, $3)`, uuid.NewV7(), testStore+3, f.now.Add(24*time.Hour), admin); err != nil {
		t.Fatal(err)
	}
	f.now = until
	list := func(s AuthorizationStatusFilter) DelegationAuthorizationPage {
		p, err := f.m.DelegationAuthorizations(ctx, ListDelegationAuthorizationsInput{AdminID: admin, Filter: DelegationAuthorizationFilter{Status: s}, Paging: DelegationPaging{}})
		if err != nil {
			t.Fatal(err)
		}
		return p
	}
	all, effective, ineffective := list(model.AuthorizationAny), list(model.AuthorizationEffective), list(model.AuthorizationIneffective)
	if all.Total != 4 || effective.Total != 1 || ineffective.Total != 3 || effective.Authorizations[0].StoreID != testStore {
		t.Fatalf("全部 %d、有效 %d、無效 %d", all.Total, effective.Total, ineffective.Total)
	}
	for _, a := range all.Authorizations {
		if a.Effective != (a.StoreID == testStore) {
			t.Errorf("店家 %d 的 Effective = %v 與篩選不一致", a.StoreID, a.Effective)
		}
	}
	_, err := f.m.DelegationAuthorizations(ctx, ListDelegationAuthorizationsInput{AdminID: admin, Filter: DelegationAuthorizationFilter{Status: "active"}, Paging: DelegationPaging{}})
	wantDenial(t, err, DenialInvalidCommand)
	if p, _ := f.m.DelegationAuthorizations(ctx, ListDelegationAuthorizationsInput{AdminID: admin, Filter: DelegationAuthorizationFilter{StoreID: ptr(testStore + 1)}, Paging: DelegationPaging{}}); p.Total != 1 {
		t.Fatalf("以店家 = %+v", p)
	}
}

func TestDelegationCodeValuesMatchTheChecks(t *testing.T) {
	f := setup(t)
	quoted := regexp.MustCompile(`'([^']+)'`)
	var reasons []string
	for _, o := range model.DelegationReasonOptions() {
		reasons = append(reasons, string(o.Code))
	}
	cases := []struct {
		constraint string
		want       []string
	}{
		{"delegated_operation_sessions_status_check", []string{string(model.DelegationActive), string(DelegationRevoked)}},
		{"delegated_operation_sessions_reason_code_check", reasons},
		{"delegated_operation_sessions_capabilities_check", fromCapabilities(model.Capabilities())},
		{"seller_delegation_authorizations_capabilities_check", fromCapabilities(model.Capabilities())},
		{"admin_operation_audit_logs_capability_check", fromCapabilities(model.Capabilities())},
		{"admin_operation_audit_logs_result_check", []string{string(model.AuditPending), string(model.AuditSucceeded), string(model.AuditFailed)}},
		{"admin_operation_audit_logs_origin_check", []string{string(model.AuditInteractive), string(model.AuditScheduled)}},
		{"admin_operation_audit_logs_resource_type_check", []string{string(model.ResourceStorefront), string(ResourceProduct), string(model.ResourcePost), string(model.ResourceOrder)}},
		{"admin_operation_audit_logs_risk_level_check", []string{string(model.RiskLow), string(RiskMedium), string(model.RiskHigh)}},
		{"seller_delegation_authorizations_source_check", []string{string(model.AuthorizationContract), string(model.AuthorizationTicket), string(model.AuthorizationPlatformPolicy)}},
		{"admin_operation_audit_logs_action_check", auditActionStrings()},
		{"delegation_authorization_audit_logs_action_check", []string{string(model.AuthorizationGrant), string(model.AuthorizationReinstate), string(model.AuthorizationUpdate), string(model.AuthorizationRevoke)}},
	}
	for _, c := range cases {
		var def string
		if err := f.db.Pool().QueryRow(context.Background(),
			`SELECT pg_get_constraintdef(oid) FROM pg_constraint WHERE conname = $1`, c.constraint).Scan(&def); err != nil {
			t.Fatalf("%s：%v", c.constraint, err)
		}
		var got []string
		for _, m := range quoted.FindAllStringSubmatch(def, -1) {
			if !slices.Contains(got, m[1]) {
				got = append(got, m[1])
			}
		}
		slices.Sort(got)
		want := slices.Clone(c.want)
		slices.Sort(want)
		if !slices.Equal(got, want) {
			t.Errorf("%s 允許 %v，Go 的值域是 %v", c.constraint, got, want)
		}
	}
}

func auditActionStrings() []string {
	var out []string
	for _, a := range model.AuditActions() {
		out = append(out, string(a))
	}
	return out
}

func TestAuditIntentKeyIsBoundToTheCommandTheResourceAndThePayload(t *testing.T) {
	f := setup(t)
	ctx := context.Background()
	admin := f.delegationAdmin(t, "ops@hawkr.example", true)
	f.grant(t, admin, testStore)
	s := f.start(t, startInput(admin, testStore, "ik-1"))
	in := intentInput(s.Session.ID, "ik-a")
	in.PayloadHash = "hash-1"
	first := f.intent(t, in)

	// 設計書 56 §6.6.1：同一把鍵用在不同的動作、資源或內容上是 9267，不是重放。
	for name, mut := range map[string]func(*AuditIntentInput){
		"不同的內容": func(in *AuditIntentInput) { in.PayloadHash = "hash-2" },
		"沒有內容":  func(in *AuditIntentInput) { in.PayloadHash = "" },
		"不同的動作": func(in *AuditIntentInput) { in.Action = model.AuditProductPublish },
		"不同的資源": func(in *AuditIntentInput) { in.ResourceID = "2701445588392099" },
	} {
		other := in
		mut(&other)
		_, err := f.m.RecordDelegationAuditIntent(ctx, other)
		if d, ok := DelegationDenialOf(err); !ok || d.Denial != DenialIdempotencyKeyReused {
			t.Errorf("%s = %v，預期 9267", name, err)
		}
	}
	if again := f.intent(t, in); !again.Replayed || again.AuditLogID != first.AuditLogID {
		t.Fatalf("同一個請求 = %+v", again)
	}

	// 建立類：intent 沒有資源的 ID，confirm 補上之後，同一個請求的重放照樣是重放。
	create := intentInput(s.Session.ID, "ik-create")
	create.Action, create.ResourceID, create.PayloadHash = model.AuditProductCreate, "", "hash-create"
	c := f.intent(t, create)
	if _, err := f.m.ConfirmDelegationAudit(ctx, AuditConfirmInput{AuditLogID: c.AuditLogID, Result: model.AuditSucceeded, ResourceID: ptr("2701445588392100")}); err != nil {
		t.Fatal(err)
	}
	if again := f.intent(t, create); !again.Replayed || again.AuditLogID != c.AuditLogID || again.Result != model.AuditSucceeded {
		t.Fatalf("建立類的重放 = %+v", again)
	}
}

func sanctionsOf(suspended bool) *seller.Sanctions {
	if !suspended {
		return seller.NoSanctions()
	}
	ground := seller.RiskFraud
	return seller.SanctionedBy(seller.EffectiveMeasure{Kind: seller.MeasureSuspension, MeasureID: uuid.NewV7(), Basis: seller.BasisRisk, RiskGround: &ground})
}
