package usecase

import (
	"errors"
	"testing"
	"time"

	"github.com/Will413028/hawkr/go-pkg/codemeta"
	"github.com/Will413028/hawkr/invoice-service/internal/domain"
)

// TestDecideHealthOutcome 釘住 §7.3.4 的 provider 狀態機（INV.hc 那一半）。
//
// 五格：ACTIVE×{成功,失敗} ／ DEGRADED×{冷卻中, half-open 成功, half-open 失敗}。
func TestDecideHealthOutcome(t *testing.T) {
	at := time.Date(2026, 8, 31, 12, 0, 0, 0, time.UTC)
	boom := errors.New("amego: 401")
	ptr := func(d time.Duration) *time.Time { v := at.Add(d); return &v }
	active := codemeta.InvoiceProviderAccountStatusActive.Value
	degraded := codemeta.InvoiceProviderAccountStatusDegraded.Value
	reason := "先前的原因"

	cases := []struct {
		name            string
		acct            domain.ProviderAccount
		err             error
		wantStatus      string
		wantProbe       int16
		wantReasonNil   bool
		wantBreakerNil  bool
		wantRecovered   bool
		wantReopened    bool
		wantBreakerAtIs *time.Time // nil ＝ 不檢查
	}{
		{
			name:          "ACTIVE ＋ 成功 → 維持 ACTIVE 並清掉 status_reason",
			acct:          domain.ProviderAccount{ID: 1, Status: active, StatusReason: &reason},
			wantStatus:    active,
			wantProbe:     0,
			wantReasonNil: true,
		},
		{
			// 🔴 刻意**不降級**：§7.3.4 進入 DEGRADED 的條件是「連續 5 次失敗」，
			// 而那個計數器在 amego client 的 gobreaker 裡不在本層。單次巡檢失敗
			// 就降級會與那條規則衝突。
			name:       "ACTIVE ＋ 失敗 → 仍是 ACTIVE，只記 status_reason",
			acct:       domain.ProviderAccount{ID: 2, Status: active},
			err:        boom,
			wantStatus: active,
		},
		{
			// open 未滿 60 秒 ⇒ 還沒進 half-open，這一輪的成功不算探測。
			// 算進去的話冷卻形同虛設，DEGRADED 會在 open 後三輪內就回 ACTIVE。
			name:       "DEGRADED ＋ 冷卻中（open 才 10 秒）＋ 成功 → 探測不加",
			acct:       domain.ProviderAccount{ID: 3, Status: degraded, BreakerOpenedAt: ptr(-10 * time.Second), BreakerProbeSuccess: 1},
			wantStatus: degraded,
			wantProbe:  1,
		},
		{
			name:       "DEGRADED ＋ half-open ＋ 成功（第 2 次）→ 探測 +1，仍 DEGRADED",
			acct:       domain.ProviderAccount{ID: 4, Status: degraded, BreakerOpenedAt: ptr(-90 * time.Second), BreakerProbeSuccess: 1},
			wantStatus: degraded,
			wantProbe:  2,
		},
		{
			name:           "DEGRADED ＋ half-open ＋ 成功（第 3 次）→ P10 回 ACTIVE 並清斷路器",
			acct:           domain.ProviderAccount{ID: 5, Status: degraded, BreakerOpenedAt: ptr(-90 * time.Second), BreakerProbeSuccess: 2},
			wantStatus:     active,
			wantProbe:      0,
			wantReasonNil:  true,
			wantBreakerNil: true,
			wantRecovered:  true,
		},
		{
			name:            "DEGRADED ＋ half-open ＋ 失敗 → 探測歸零、回 open",
			acct:            domain.ProviderAccount{ID: 6, Status: degraded, BreakerOpenedAt: ptr(-90 * time.Second), BreakerProbeSuccess: 2},
			err:             boom,
			wantStatus:      degraded,
			wantProbe:       0,
			wantReopened:    true,
			wantBreakerAtIs: &at,
		},
	}

	for _, c := range cases {
		got := decideHealthOutcome(c.acct, at, c.err)
		r := got.record

		// 🔴 不論走哪一格，巡檢週期一律寫回 now+15min。少了它，claim 推上去的
		// 120s 租約會變成事實上的巡檢週期，呼叫量瞬間變 7.5 倍（§9.2.1 的容量前提）。
		if want := at.Add(15 * time.Minute); !r.NextCheckAt.Equal(want) {
			t.Errorf("%s: NextCheckAt=%v want %v", c.name, r.NextCheckAt, want)
		}
		if !r.CheckedAt.Equal(at) {
			t.Errorf("%s: CheckedAt=%v want %v", c.name, r.CheckedAt, at)
		}
		if r.NextStatus != c.wantStatus {
			t.Errorf("%s: NextStatus=%s want %s", c.name, r.NextStatus, c.wantStatus)
		}
		if r.BreakerProbeSuccess != c.wantProbe {
			t.Errorf("%s: BreakerProbeSuccess=%d want %d", c.name, r.BreakerProbeSuccess, c.wantProbe)
		}
		if c.wantReasonNil && r.StatusReason != nil {
			t.Errorf("%s: StatusReason 應為 nil，實得 %q", c.name, *r.StatusReason)
		}
		if c.err != nil && !c.wantReasonNil && (r.StatusReason == nil || *r.StatusReason == "") {
			t.Errorf("%s: 失敗時 StatusReason 不該是空的", c.name)
		}
		if c.wantBreakerNil && r.BreakerOpenedAt != nil {
			t.Errorf("%s: BreakerOpenedAt 應為 nil", c.name)
		}
		if c.wantBreakerAtIs != nil && (r.BreakerOpenedAt == nil || !r.BreakerOpenedAt.Equal(*c.wantBreakerAtIs)) {
			t.Errorf("%s: BreakerOpenedAt 應重設為 %v", c.name, *c.wantBreakerAtIs)
		}
		if got.recovered != c.wantRecovered {
			t.Errorf("%s: recovered=%v want %v", c.name, got.recovered, c.wantRecovered)
		}
		if got.reopened != c.wantReopened {
			t.Errorf("%s: reopened=%v want %v", c.name, got.reopened, c.wantReopened)
		}
	}
}

// TestDecideHealthOutcome_NilBreakerOpenedAtCountsAsHalfOpen 釘住一個邊界：
// DEGRADED 但 breaker_opened_at 是 NULL（例如由 ADM 手動設成 DEGRADED）。
//
// 判定成 half-open 而不是「永遠冷卻中」——後者會讓那筆帳戶再也回不到 ACTIVE，
// 正是本 worker 要消滅的那種靜默終點。
func TestDecideHealthOutcome_NilBreakerOpenedAtCountsAsHalfOpen(t *testing.T) {
	at := time.Date(2026, 8, 31, 12, 0, 0, 0, time.UTC)
	acct := domain.ProviderAccount{
		ID:     9,
		Status: codemeta.InvoiceProviderAccountStatusDegraded.Value,
		// BreakerOpenedAt 為 nil
		BreakerProbeSuccess: 2,
	}
	got := decideHealthOutcome(acct, at, nil)
	if !got.recovered || got.record.NextStatus != codemeta.InvoiceProviderAccountStatusActive.Value {
		t.Fatalf("breaker_opened_at 為 NULL 時應視為 half-open 並在第 3 次成功回 ACTIVE，實得 status=%s recovered=%v",
			got.record.NextStatus, got.recovered)
	}
}

// TestDecideHealthOutcome_ProviderUnavailableReverify 釘住 §7.3.4 P08 的自動重驗分支。
//
// 這一格在 2026-08-31 之前**不存在執行者**：FetchDueHealthChecks／ClaimHealthCheck 的述詞
// 只列 ACTIVE／DEGRADED，而 P06 寫好的 health_check_next_at 沒有任何東西會撈 ⇒ 規格寫的
// 「自動重驗，最多 24 小時」是一句沒有實作的話（backend#2977 ②）。
//
// 🔴 兩件事各自獨立，缺一即靜默失效，所以兩件都要有斷言：
//
//	① 探測成功 → ACTIVE（而不是停在 CREDENTIAL_VERIFYING —— 那個狀態沒有排程撿它）
//	② 探測失敗 → status 完全不動，且 **breaker_opened_at 不被重設**
//	   （重設＝24 小時上界每輪往後推 ⇒ 上界永遠不到 ⇒ 無限重驗）
func TestDecideHealthOutcome_ProviderUnavailableReverify(t *testing.T) {
	at := time.Date(2026, 8, 31, 12, 0, 0, 0, time.UTC)
	unavailable := codemeta.InvoiceProviderAccountStatusProviderUnavailable.Value
	active := codemeta.InvoiceProviderAccountStatusActive.Value
	openedAt := at.Add(-3 * time.Hour)
	reason := "電子發票服務商暫時無法連線"

	t.Run("探測成功 → 直接落 ACTIVE 並清掉不可用的錶", func(t *testing.T) {
		out := decideHealthOutcome(domain.ProviderAccount{
			ID: 7, Status: unavailable, StatusReason: &reason, BreakerOpenedAt: &openedAt,
		}, at, nil)

		if out.record.NextStatus != active {
			t.Fatalf("NextStatus = %q, want %q（停在其他狀態＝沒有排程會再撿它）", out.record.NextStatus, active)
		}
		if out.record.StatusReason != nil {
			t.Errorf("StatusReason = %v, want nil", *out.record.StatusReason)
		}
		if out.record.BreakerOpenedAt != nil {
			t.Errorf("BreakerOpenedAt = %v, want nil（恢復後錶要清掉，否則下次不可用時上界當場已過）", *out.record.BreakerOpenedAt)
		}
		if !out.recovered {
			t.Error("recovered = false, want true（M 指標靠它計數）")
		}
	})

	t.Run("探測失敗 → status 不動，且不重設 24 小時上界的錨", func(t *testing.T) {
		out := decideHealthOutcome(domain.ProviderAccount{
			ID: 7, Status: unavailable, BreakerOpenedAt: &openedAt,
		}, at, errors.New("amego: connection refused"))

		if out.record.NextStatus != unavailable {
			t.Fatalf("NextStatus = %q, want %q（失敗不改 status，同端點 44 定版③）", out.record.NextStatus, unavailable)
		}
		if out.record.BreakerOpenedAt == nil || !out.record.BreakerOpenedAt.Equal(openedAt) {
			t.Errorf("BreakerOpenedAt = %v, want %v（被重設成 now 的話，每 15 分鐘往後推一次 ⇒ 24 小時上界永遠不到）",
				out.record.BreakerOpenedAt, openedAt)
		}
		if out.record.StatusReason == nil {
			t.Error("StatusReason = nil, want 非空（失敗原因要留給 ADM 看）")
		}
		if out.recovered {
			t.Error("recovered = true, want false")
		}
	})
}
