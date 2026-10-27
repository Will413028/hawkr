package usecase

import (
	"context"
	"time"

	"github.com/Will413028/hawkr/go-pkg/codemeta"
	"github.com/Will413028/hawkr/invoice-service/internal/client/amego"
	"github.com/Will413028/hawkr/invoice-service/internal/domain"
	"go.uber.org/zap"
)

// ---------------------------------------------------------------------------
// INV.hc —— provider 帳戶健康檢查（§7.3.4 P08–P10、§7.5.3、§4.3.3 第 6 組）
//
// 存在理由（§乙類逐字）：把發現「憑證失效」的時點從「下一張發票開失敗」提前到
// 「巡檢時」。法定 48 小時的開票期限裡，這個提前量就是能不能救回來的差別。
//
// 🔴 它先前**完全不存在**，而候選 SQL、claim CAS、結果寫回、租約欄、索引全都在
// ⇒ `DEGRADED` 的帳戶**沒有任何東西**能讓它自動回 `ACTIVE`（#2969）。
//
// 打哪一支端點（§4.5.2 硬規則）：**不得為健康檢查新增第 16 條 nginx 白名單**，
// 只能打 Client 介面已涵蓋的 15 支裡唯讀且需簽章的那幾支。預設 `/json/track_all`
// （ListTracks）——它是唯一能在一次呼叫內同時證明「AppKey 有效」與「統編綁定正確」
// 且**零副作用**的端點。
//
// 🔴 租約欄是 `health_check_next_at`，**不是** `last_health_checked_at`：後者是
// 事實時間戳（最後一次真的打了 AMEGO），被 claim 推到未來就變成謊言，而它同時是
// 「這個帳戶多久沒被檢查過」那條告警的來源（§4.6 W-02）。
// ---------------------------------------------------------------------------

// healthCheckInterval 是巡檢週期（§7.5.3）。
//
// 🔴 **不論成敗都要寫 `health_check_next_at = now + 15min`**（生成檔 SQL 註解逐字）：
// 沒有這一句，claim 推上去的 120s 租約會變成事實上的巡檢週期，呼叫量瞬間變 7.5 倍。
// 週期是**容量決策**不是即時性偏好：每個 ACTIVE／DEGRADED 憑證 4×24 ＝ 96 次／日，
// 掛在賣家數 S 而非訂單數 D 上，零開票的日子照打（§9.2.1）。改它要回頭重算呼叫量。
const healthCheckInterval = 15 * time.Minute

// healthCheckLease 是 claim 推到未來的租約（§7.5.4，下界 10.3s ＝ 一次唯讀呼叫 10s ＋ DB 0.3s）。
const healthCheckLease = 120 * time.Second

// healthCheckBatch 是單輪撈幾筆（§7.5.3 FetchBatch = 50）。
//
// 放 usecase 不放 worker：它決定的是「一輪要處理多少工作」，與「多久 tick 一次」
// 是兩件事。放兩處各寫一次會漂移，而漂移的那天沒有任何東西會轉紅。
const healthCheckBatch = 50

// halfOpenProbesToClose 是 half-open 連續成功幾次才回 ACTIVE（P10 ＝ 3）。
const halfOpenProbesToClose = 3

// halfOpenAfter 是 open 持續多久才進 half-open（§7.3.4「DEGRADED 的三個條件」＝ 60 秒）。
const halfOpenAfter = 60 * time.Second

// providerUnavailableReverifyWindow 是 §7.3.4 P06→P08 的「排入 health-check 自動重驗，
// **最多 24 小時**」。超過這個窗口的 PROVIDER_UNAVAILABLE 帳戶不再自動重驗，只留 ADM
// 端點 44 手動——否則一個永久壞掉的憑證會每 15 分鐘打一次 AMEGO、直到有人發現為止。
//
// 起算點是 breaker_opened_at（進入 PROVIDER_UNAVAILABLE 的時刻），不是 updated_at：
// 後者每輪 claim 都會被推 ⇒ 上界永遠不到（見 invoice_provider_accounts.sql 的註解）。
const providerUnavailableReverifyWindow = 24 * time.Hour

// HealthProbe 是 INV.hc 需要的出站面。刻意只要 ListTracks 一支——
// 介面越窄，「這支 worker 會打哪些 AMEGO 端點」在型別上就越說得清楚。
type HealthProbe interface {
	ListTracks(ctx context.Context, c amego.Credential, year, period int) ([]amego.Track, error)
}

// HealthAccountStore 是 INV.hc 需要的 repository 面。
type HealthAccountStore interface {
	FetchDueHealthChecks(ctx context.Context, now, unavailableSinceFloor time.Time, limit int32) ([]domain.ProviderAccount, error)
	ClaimHealthCheck(ctx context.Context, id int64, now, leaseUntil, unavailableSinceFloor time.Time) (bool, error)
	RecordHealthCheckResult(ctx context.Context, in domain.RecordHealthCheckResult) error
	WithDecryptedCredential(ctx context.Context, id int64, fn func(providerTaxID, appKey string) error) error
}

// HealthCheck 是 INV.hc 的 usecase。
type HealthCheck struct {
	accounts HealthAccountStore
	probe    HealthProbe
	log      *zap.Logger
	now      func() time.Time
	batch    int32
}

// NewHealthCheck 建 INV.hc usecase。
func NewHealthCheck(accounts HealthAccountStore, probe HealthProbe, log *zap.Logger, now func() time.Time, batch int32) *HealthCheck {
	return &HealthCheck{accounts: accounts, probe: probe, log: orNop(log), now: now, batch: batch}
}

// RunOnce 跑一輪健康巡檢。
//
// 🔴 單一帳戶失敗不中斷整輪：一家的憑證壞掉正是要被記錄的事實，不該讓其餘帳戶
// 整輪不被檢查。
func (u *HealthCheck) RunOnce(ctx context.Context) error {
	now := u.now()
	due, err := u.accounts.FetchDueHealthChecks(ctx, now, now.Add(-providerUnavailableReverifyWindow), u.batch)
	if err != nil {
		return err
	}
	var checked, skipped, recovered, degraded int
	for _, acct := range due {
		// 🔴 claim 的基準逐筆在當下取，不在迴圈外算一次：算一次會讓不變式從
		// 「單筆最壞耗時 < 租約」退化成「整批總耗時 < 租約」（《多副本排程搶佔規範》）。
		at := u.now()
		// 上界基準同樣逐筆取：與 fetch 用同一個時鐘來源，兩處述詞才真的鏡射。
		ok, err := u.accounts.ClaimHealthCheck(ctx, acct.ID, at, at.Add(healthCheckLease),
			at.Add(-providerUnavailableReverifyWindow))
		if err != nil {
			u.log.Error("invoice.hc.claim_failed", zap.Int64("account_id", acct.ID), zap.Error(err))
			continue
		}
		if !ok {
			// 沒搶到＝別的副本先到，或述詞已不成立。不是錯誤。
			skipped++
			continue
		}
		res := u.checkOne(ctx, acct)
		if err := u.accounts.RecordHealthCheckResult(ctx, res.record); err != nil {
			u.log.Error("invoice.hc.record_failed", zap.Int64("account_id", acct.ID), zap.Error(err))
			continue
		}
		checked++
		if res.recovered {
			recovered++
		}
		if res.reopened {
			degraded++
		}
	}
	u.log.Info("invoice.hc.scanned",
		zap.Int("due", len(due)), zap.Int("checked", checked), zap.Int("skipped", skipped),
		zap.Int("recovered_to_active", recovered), zap.Int("breaker_reopened", degraded))
	return nil
}

type healthOutcome struct {
	record    domain.RecordHealthCheckResult
	recovered bool // DEGRADED → ACTIVE（P10）
	reopened  bool // half-open 探測失敗 ⇒ 回 open
}

// checkOne 對單一帳戶打一次唯讀探測並算出要寫回的狀態。
//
// 🔴 AppKey 明文的活動範圍只到 WithDecryptedCredential 的 callback（同 submit.go）。
func (u *HealthCheck) checkOne(ctx context.Context, acct domain.ProviderAccount) healthOutcome {
	at := u.now()
	year, period := domain.AmegoPeriod(at)

	var probeErr error
	if err := u.accounts.WithDecryptedCredential(ctx, acct.ID, func(providerTaxID, appKey string) error {
		_, probeErr = u.probe.ListTracks(ctx,
			amego.Credential{IssuerID: acct.IssuerID, Invoice: providerTaxID, AppKey: appKey},
			year, period)
		return nil
	}); err != nil {
		// 連憑證都解不開：這本身就是健康問題，記進 status_reason。
		probeErr = err
	}

	return decideHealthOutcome(acct, at, probeErr)
}

// decideHealthOutcome 是純函式，把「這一次探測的結果」映射成要寫回的欄位。
//
// 抽成純函式的理由：狀態機的四格（ACTIVE/DEGRADED × 成功/失敗）是這支 worker
// 的全部語意，值得被單獨測到——而它不需要 context、DB 或 AMEGO。
//
// 四格逐字對應 §7.3.4：
//
//	ACTIVE   ＋ 成功 → 維持 ACTIVE，清掉 status_reason
//	ACTIVE   ＋ 失敗 → **維持 ACTIVE**，只記 status_reason（見下方 ⚠️）
//	DEGRADED ＋ 成功 → half-open 探測 +1；連續達 3 次 → ACTIVE 並清斷路器（P10）
//	DEGRADED ＋ 失敗 → 探測歸零、breaker_opened_at 重設為現在（回 open）
//
// ⚠️ **ACTIVE ＋ 失敗刻意不降級**：§7.3.4 定義進入 DEGRADED 的條件是「該 store 連續
// **5 次** AMEGO 呼叫失敗」，而那個計數器在 amego client 的 gobreaker 裡、不在本層。
// 單次巡檢失敗就降級會與那條規則衝突（一次失敗 ≠ 連續五次），所以這裡只記錄事實。
// ⚠️ 已知缺口：client 現在的 gobreaker 是**全域**一顆（`Name: "amego"`），設計書要的
// per-store 斷路器尚不存在 ⇒ 「連續 5 次」目前是跨賣家算的。缺口登記在 #2969。
//
// 🔴 DEGRADED 且尚未到 half-open 窗口（open 未滿 60 秒）時**不動探測計數**：
// 那段時間的成功不算 half-open 探測，把它算進去會讓冷卻形同虛設。
func decideHealthOutcome(acct domain.ProviderAccount, at time.Time, probeErr error) healthOutcome {
	out := healthOutcome{record: domain.RecordHealthCheckResult{
		ID:                  acct.ID,
		CheckedAt:           at,
		NextCheckAt:         at.Add(healthCheckInterval),
		NextStatus:          acct.Status,
		StatusReason:        acct.StatusReason,
		BreakerOpenedAt:     acct.BreakerOpenedAt,
		BreakerProbeSuccess: acct.BreakerProbeSuccess,
	}}

	if acct.Status == codemeta.InvoiceProviderAccountStatusProviderUnavailable.Value {
		// PROVIDER_UNAVAILABLE 分支 —— §7.3.4 P08 的自動重驗。
		//
		// 探測成功即落 ACTIVE，**不經 CREDENTIAL_VERIFYING**：規格的
		// `PROVIDER_UNAVAILABLE --[INV.hc]--> CREDENTIAL_VERIFYING` 講的是同步 verify
		// 內的過渡態（該狀態沒有任何排程撿它，見 invoice_provider_accounts.sql 的
		// UpsertCredentials 註解「中間態不對外呈現」）——真的把列停在那裡就是卡死。
		//
		// 敢直接落 ACTIVE 的理由：HealthProbe.ListTracks 是**帶該帳戶真實憑證**打 AMEGO
		// （credential 由 WithDecryptedCredential 供給）⇒ 成功同時證明 provider 可達
		// 與憑證有效，正是 CREDENTIAL_VERIFYING 要驗的那兩件事。
		//
		// 失敗則 status 完全不動，與 ADM 端點 44 的第 ③ 條定版一致；breaker_opened_at
		// 保持不動，24 小時上界的時鐘才會繼續走（重設它＝上界永遠不到）。
		if probeErr == nil {
			out.record.NextStatus = codemeta.InvoiceProviderAccountStatusActive.Value
			out.record.StatusReason = nil
			out.record.BreakerOpenedAt = nil
			out.record.BreakerProbeSuccess = 0
			out.recovered = true
			return out
		}
		reason := probeErr.Error()
		out.record.StatusReason = &reason
		return out
	}

	if acct.Status != codemeta.InvoiceProviderAccountStatusDegraded.Value {
		// ACTIVE 分支
		if probeErr == nil {
			out.record.StatusReason = nil
			out.record.BreakerProbeSuccess = 0
			return out
		}
		reason := probeErr.Error()
		out.record.StatusReason = &reason
		return out
	}

	// DEGRADED 分支
	inHalfOpen := acct.BreakerOpenedAt == nil || !at.Before(acct.BreakerOpenedAt.Add(halfOpenAfter))
	if !inHalfOpen {
		// 冷卻中：這一輪只更新時間戳，不動探測計數。
		return out
	}
	if probeErr != nil {
		reason := probeErr.Error()
		out.record.StatusReason = &reason
		out.record.BreakerProbeSuccess = 0
		opened := at
		out.record.BreakerOpenedAt = &opened
		out.reopened = true
		return out
	}
	next := acct.BreakerProbeSuccess + 1
	if int(next) >= halfOpenProbesToClose {
		out.record.NextStatus = codemeta.InvoiceProviderAccountStatusActive.Value
		out.record.StatusReason = nil
		out.record.BreakerOpenedAt = nil
		out.record.BreakerProbeSuccess = 0
		out.recovered = true
		return out
	}
	out.record.BreakerProbeSuccess = next
	return out
}
