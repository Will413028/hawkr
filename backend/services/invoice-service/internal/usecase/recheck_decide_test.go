package usecase

import (
	"errors"
	"testing"
	"time"

	"github.com/Will413028/hawkr/go-pkg/codemeta"
	"github.com/Will413028/hawkr/invoice-service/internal/client/amego"
	"github.com/Will413028/hawkr/invoice-service/internal/domain"
)

// TestDecideRecheck 釘住 §7.3 的重查狀態機（J17／J23–J26／J31）。
func TestDecideRecheck(t *testing.T) {
	now := time.Date(2026, 8, 31, 12, 0, 0, 0, time.UTC)
	ago := func(d time.Duration) *time.Time { v := now.Add(-d); return &v }

	found := RecheckProbe{Found: true}
	notIssued := RecheckProbe{NotIssued: true, WithinProviderRetention: true}
	notIssuedStale := RecheckProbe{NotIssued: true, WithinProviderRetention: false}
	unclear := RecheckProbe{}

	cases := []struct {
		name       string
		op         domain.Operation
		probe      RecheckProbe
		wantKind   RecheckOutcomeKind
		wantStatus string
		wantEvid   bool
	}{
		{
			name:       "J23 查到已開立 → SUCCEEDED（即使有取消在途，取消被回絕）",
			op:         domain.Operation{ID: 1, UnknownSince: ago(time.Hour), CancelRequestedAt: ago(time.Minute)},
			probe:      found,
			wantKind:   RecheckResolved,
			wantStatus: codemeta.InvoiceOperationStatusSucceeded.Value,
		},
		{
			name:       "J24 code=71 ＋ 有 ADM 取消在途 → CANCELLED_BY_ADMIN ＋ 證據",
			op:         domain.Operation{ID: 2, UnknownSince: ago(time.Hour), CancelRequestedAt: ago(time.Minute)},
			probe:      notIssued,
			wantKind:   RecheckResolved,
			wantStatus: codemeta.InvoiceOperationStatusCancelledByAdmin.Value,
			wantEvid:   true,
		},
		{
			name:       "J25 code=71 ＋ 無取消 ＋ 180 天內 → RETRY_SCHEDULED",
			op:         domain.Operation{ID: 3, UnknownSince: ago(time.Hour)},
			probe:      notIssued,
			wantKind:   RecheckResolved,
			wantStatus: codemeta.InvoiceOperationStatusRetryScheduled.Value,
		},
		{
			// 🔴 這一格是重複開票的分水嶺：超過 180 天時 AMEGO 對「開過的發票」
			// 也回 71，把它當成「確定沒開」而補送 ⇒ 同一筆訂單開出第二張發票。
			name:       "J25 邊界：code=71 但超過 180 天 → 不得補送，退回重查迴圈",
			op:         domain.Operation{ID: 4, UnknownSince: ago(time.Hour)},
			probe:      notIssuedStale,
			wantKind:   RecheckStillWaiting,
			wantStatus: codemeta.InvoiceOperationStatusWaitingConfirmation.Value,
		},
		{
			name:       "J26 仍不確定 ＋ 有 ADM 取消在途 → CANCEL_BLOCKED_UNKNOWN",
			op:         domain.Operation{ID: 5, UnknownSince: ago(time.Hour), CancelRequestedAt: ago(time.Minute)},
			probe:      unclear,
			wantKind:   RecheckUnresolvedTerminal,
			wantStatus: codemeta.InvoiceOperationStatusCancelBlockedUnknown.Value,
		},
		{
			name:       "J17 仍不確定 ＋ 無取消 ＋ 超過 24h → FAILED_UNKNOWN",
			op:         domain.Operation{ID: 6, UnknownSince: ago(25 * time.Hour)},
			probe:      unclear,
			wantKind:   RecheckUnresolvedTerminal,
			wantStatus: codemeta.InvoiceOperationStatusFailedUnknown.Value,
		},
		{
			name:       "J31 仍不確定 ＋ 無取消 ＋ 未超過 24h → WAITING_CONFIRMATION",
			op:         domain.Operation{ID: 7, UnknownSince: ago(23 * time.Hour)},
			probe:      unclear,
			wantKind:   RecheckStillWaiting,
			wantStatus: codemeta.InvoiceOperationStatusWaitingConfirmation.Value,
		},
		{
			// 已確認的取消不算「在途」——前置 ② 逐字用兩個時間戳判別，
			// 只看 cancel_requested_at 會讓已結案的取消永遠把 job 卡在 J26。
			name:       "取消已確認（confirmed 非空）不算在途 → 走無取消分支",
			op:         domain.Operation{ID: 8, UnknownSince: ago(time.Hour), CancelRequestedAt: ago(time.Hour), CancelConfirmedAt: ago(time.Minute)},
			probe:      notIssued,
			wantKind:   RecheckResolved,
			wantStatus: codemeta.InvoiceOperationStatusRetryScheduled.Value,
		},
	}

	for _, c := range cases {
		got := decideRecheck(c.op, c.probe, now)
		if got.Kind != c.wantKind {
			t.Errorf("%s: Kind=%d want %d", c.name, got.Kind, c.wantKind)
		}
		if got.NextStatus != c.wantStatus {
			t.Errorf("%s: NextStatus=%s want %s", c.name, got.NextStatus, c.wantStatus)
		}
		if c.wantEvid && got.CancelEvidenceKind == nil {
			t.Errorf("%s: 應帶 cancel_evidence_kind（chk_invop_admin_cancel_needs_confirm 會擋）", c.name)
		}
		if !c.wantEvid && got.CancelEvidenceKind != nil {
			t.Errorf("%s: 不該帶 cancel_evidence_kind，實得 %q", c.name, *got.CancelEvidenceKind)
		}
	}
}

// TestRecheckBackoff 釘住 §7.5.3 的階梯 30s→1m→2m→5m→10m→30m。
//
// 🔴 階梯位置由 `now − unknown_since` 推導，不是由 attempt_count（前置 ①）。
// 計數器在「claim 成功但副本崩潰」之後會失真而時間戳不會——失真的方向是
// 計數停住、時間繼續走 ⇒ 那一列用最短間隔重查到天荒地老。
func TestRecheckBackoff(t *testing.T) {
	cases := []struct {
		elapsed time.Duration
		want    time.Duration
	}{
		{0, 30 * time.Second},
		{119 * time.Second, 30 * time.Second},
		{2 * time.Minute, time.Minute},
		{5 * time.Minute, 2 * time.Minute},
		{15 * time.Minute, 5 * time.Minute},
		{45 * time.Minute, 10 * time.Minute},
		{2 * time.Hour, 30 * time.Minute},
		{23 * time.Hour, 30 * time.Minute}, // 封頂，不會再拉長
	}
	for _, c := range cases {
		if got := recheckBackoff(c.elapsed); got != c.want {
			t.Errorf("recheckBackoff(%v)=%v want %v", c.elapsed, got, c.want)
		}
	}
}

// TestDecideRecheck_NextAttemptUsesLadder 確認排程時間真的用了階梯而不是固定值。
func TestDecideRecheck_NextAttemptUsesLadder(t *testing.T) {
	now := time.Date(2026, 8, 31, 12, 0, 0, 0, time.UTC)
	early := now.Add(-time.Minute)  // elapsed 1m ⇒ 30s
	late := now.Add(-3 * time.Hour) // elapsed 3h ⇒ 30m
	got1 := decideRecheck(domain.Operation{ID: 1, UnknownSince: &early}, RecheckProbe{}, now)
	got2 := decideRecheck(domain.Operation{ID: 2, UnknownSince: &late}, RecheckProbe{}, now)
	if !got1.NextAttemptAt.Equal(now.Add(30 * time.Second)) {
		t.Errorf("elapsed 1m 應排 +30s，實得 %v", got1.NextAttemptAt.Sub(now))
	}
	if !got2.NextAttemptAt.Equal(now.Add(30 * time.Minute)) {
		t.Errorf("elapsed 3h 應排 +30m，實得 %v", got2.NextAttemptAt.Sub(now))
	}
}

// TestUploadNextAt 釘住候選集合②的離場條件。
//
// 🔴 終態回 nil ＝ 離開候選集合。少了這一格，91／-1 的列會**永久滯留**在
// upload_recheck 的候選裡，每 60 秒被 claim 一次、打一次 AMEGO，直到天荒地老
// ——而它們的上傳結果已經不會再變（§5.2.3 第 3 組警告的正是這個形狀）。
func TestUploadNextAt(t *testing.T) {
	now := time.Date(2026, 8, 31, 12, 0, 0, 0, time.UTC)
	issued := now.Add(-3 * time.Hour)
	doc := domain.Document{ID: 1, ProviderIssuedAt: &issued}
	s := func(v int16) *int16 { return &v }

	for _, term := range []int16{99, 91, -1} {
		if got := uploadNextAt(doc, s(term), nil, now); got != nil {
			t.Errorf("status=%d 是終態，應回 nil（離開候選集合），實得 %v", term, *got)
		}
	}
	// 還在飛的值要排下一次；elapsed 3h ⇒ 階梯封頂 30 分鐘。
	for _, flying := range []int16{1, 2, 3, 31, 32} {
		got := uploadNextAt(doc, s(flying), nil, now)
		if got == nil {
			t.Fatalf("status=%d 還在飛，應排下一次覆查", flying)
		}
		if !got.Equal(now.Add(30 * time.Minute)) {
			t.Errorf("status=%d 應排 +30m（elapsed 3h 封頂），實得 %v", flying, got.Sub(now))
		}
	}
	// 查不到狀態（nil）且是**暫時性**失敗 ⇒ 仍要排下一次，不能因為這一輪沒問到就讓它離場。
	if got := uploadNextAt(doc, nil, nil, now); got == nil {
		t.Error("status 為 nil（本輪沒查到）時仍應排下一次，否則那份文件永遠不再被覆查")
	}
}

// #3014：這個迴圈原本**沒有任何終止條件**。
//
// 唯一的出邊是「拿得到 status 且為終態」，於是 status 永遠拿不到的那一列就永遠留在
// 候選集合裡：dev 實測 attempts 累到 29 仍在排程，對 AMEGO 的呼叫 92% 是這個迴圈、
// 真正的開票只佔 0.4%。
//
// 🔴 三格的方向必須同時成立，只驗其中一格會讓修法退化：
//   - 只驗「PERMANENT 停」⇒ 寫成「一律不重試」也會過，而那會讓暫時性故障永久放棄
//   - 只驗「RETRYABLE 續」⇒ 等於沒修
//   - 只驗次數上限 ⇒ PERMANENT 仍會白跑 48 次
func TestUploadNextAt_TerminationConditions(t *testing.T) {
	now := time.Date(2026, 8, 31, 12, 0, 0, 0, time.UTC)
	issued := now.Add(-3 * time.Hour)
	doc := domain.Document{ID: 1, ProviderIssuedAt: &issued}

	permanent := &amego.APIError{
		Endpoint: "/json/invoice_query", Code: "71",
		Class: amego.ClassPermanent, Kind: amego.ErrorKindProvider, Msg: "查無資料",
	}
	retryable := &amego.APIError{
		Endpoint: "/json/invoice_query", Code: "500",
		Class: amego.ClassRetryable, Kind: amego.ErrorKindProvider, Msg: "暫時性錯誤",
	}

	t.Run("PERMANENT ⇒ 立刻停（重試 48 次也不會變）", func(t *testing.T) {
		if got := uploadNextAt(doc, nil, permanent, now); got != nil {
			t.Fatalf("PERMANENT 仍排下一次（%v）—— 這就是 #3014 的無限迴圈", got)
		}
	})

	t.Run("RETRYABLE ⇒ 仍要重試（修法不能是一律不重試）", func(t *testing.T) {
		if got := uploadNextAt(doc, nil, retryable, now); got == nil {
			t.Fatal("RETRYABLE 被停掉 —— 一次暫時性故障就讓這張票永遠不再覆查")
		}
	})

	t.Run("非 AMEGO 來源的錯誤 ⇒ 當暫時性（一次金鑰輪替不該終止全部覆查）", func(t *testing.T) {
		if got := uploadNextAt(doc, nil, errors.New("decrypt credential failed"), now); got == nil {
			t.Fatal("非 AMEGO 錯誤被當成 PERMANENT")
		}
	})

	t.Run("次數用盡 ⇒ 停（SQL 註解逐字的 48）", func(t *testing.T) {
		atCap := domain.Document{ID: 1, ProviderIssuedAt: &issued,
			UploadRecheckAttempts: uploadRecheckMaxAttempts - 1}
		// -1 是因為本輪的 ClaimUploadRecheck 已經 +1 過，doc 是 claim 之前讀到的。
		if got := uploadNextAt(atCap, nil, nil, now); got != nil {
			t.Fatalf("attempts 已達上限仍排下一次（%v）", got)
		}
		justUnder := domain.Document{ID: 1, ProviderIssuedAt: &issued,
			UploadRecheckAttempts: uploadRecheckMaxAttempts - 2}
		if got := uploadNextAt(justUnder, nil, nil, now); got == nil {
			t.Fatal("還沒到上限就停了 —— 上限提前一輪生效")
		}
	})

	t.Run("終態 status 優先於一切（即使同時有 PERMANENT 錯誤也一樣是離場）", func(t *testing.T) {
		s := int16(99)
		if got := uploadNextAt(doc, &s, permanent, now); got != nil {
			t.Fatalf("終態仍排下一次（%v）", got)
		}
	})
}
