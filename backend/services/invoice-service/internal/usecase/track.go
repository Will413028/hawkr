package usecase

import (
	"context"
	"strconv"
	"time"

	"github.com/Will413028/hawkr/invoice-service/internal/client/amego"
	"github.com/Will413028/hawkr/invoice-service/internal/domain"
	"go.uber.org/zap"
)

// ---------------------------------------------------------------------------
// INV.trk —— 字軌餘量巡檢（§4.5.10、§7.5.3、§8.2.3 M9／M10／M21／M24）
//
// 這支是「向第三方取回我方沒有的資訊」（§乙類），**不是** reconciliation：它不比對
// 兩份資料、不回填、不修正，只把光貿那側的字軌狀態變成我方的指標。
//
// 🔴 它先前**完全不存在**，而 SQL、repository 方法、config env、告警規則全都寫好了
// ⇒ 三條告警（InvoiceTrackBalanceLow／Exhausted／NextPeriodInsufficient）從上線那天
// 起就結構上不可能觸發（#2969／#2959）。
//
// 為什麼沒有候選集合租約（§4.3.3 line 850 逐字）：它掃的是 invoice_provider_accounts
// 中 status='ACTIVE' 的**全集**，不帶租約欄。§7.5.4 給的 ClaimLease=10min 避免的是
// 「同一輪掃描被重複啟動」，**不承擔正確性** —— 多副本同時跑最多是多打幾次 AMEGO，
// 指標是 Set 不是 Add，不會疊加。
// ---------------------------------------------------------------------------

// 一本字軌 50 張（財政部配號單位）。M24 的「下期本數 × 50」逐字用它。
const numbersPerBooklet = 50

// TrackScanner 是 INV.trk 需要的出站面。用窄介面讓測試不必架整個 amego client。
type TrackScanner interface {
	ListTracks(ctx context.Context, c amego.Credential, year, period int) ([]amego.Track, error)
}

// TrackAccountSource 是 INV.trk 需要的 repository 面。
type TrackAccountSource interface {
	ListActive(ctx context.Context) ([]domain.ProviderAccount, error)
	WithDecryptedCredential(ctx context.Context, id int64, fn func(providerTaxID, appKey string) error) error
}

// TrackObserver 收下一輪掃描的結果。worker 那側把它接到 Prometheus gauge。
//
// 用介面而不是直接寫 gauge：usecase 不該 import promauto（那會讓每個 usecase 測試
// 都在全域 registry 上註冊一次），而且這樣測試能斷言「算出來的四個數對不對」，
// 不必去讀 /metrics。
type TrackObserver interface {
	ObserveTrackScan(remainingMin, lowSellers, exhaustedSellers, nextPeriodInsufficient int64)
}

// TrackOperationPromoter 是 J33 的出向面（SELLER_ACTION_REQUIRED → READY）。
//
// 🔴 窄介面而不是整個 domain.OperationRepository：INV.trk 只需要「這位 issuer 的
// 字軌回來了，把他卡住的票放行」這一件事，收下整個 repository 會讓每個 Track 測試
// 都得實作三十幾支方法（而那些替身多半會寫成無條件回 true ——
// 「替身沒鏡射真實守衛」正是本輪要清的另一個缺陷類）。
type TrackOperationPromoter interface {
	PromoteSellerActionRequired(ctx context.Context, issuerID int64, now time.Time) (int64, error)
}

// Track 是 INV.trk 的 usecase。
type Track struct {
	accounts TrackAccountSource
	amego    TrackScanner
	obs      TrackObserver
	ops      TrackOperationPromoter
	log      *zap.Logger
	now      func() time.Time

	warnThreshold  int64
	growthHeadroom float64
}

// NewTrack 建 INV.trk usecase。
func NewTrack(
	accounts TrackAccountSource,
	scanner TrackScanner,
	obs TrackObserver,
	ops TrackOperationPromoter,
	log *zap.Logger,
	now func() time.Time,
	warnThreshold int64,
	growthHeadroom float64,
) *Track {
	return &Track{
		accounts: accounts, amego: scanner, obs: obs, ops: ops, log: orNop(log), now: now,
		warnThreshold: warnThreshold, growthHeadroom: growthHeadroom,
	}
}

// remainingNumbers 逐字取自 §4.5.10 的規格碼。
//
// `now == ""` 代表**尚未配過號**（實測會出現），此時整段都還沒用 ⇒ end-start+1。
// 已配過號時 `now` 是「下一張要用的號碼」的前一張 ⇒ 剩餘是 end-now。
//
// 🔴 不要改成用 TotalBooklet-UsedBooklet 推算：那兩欄在 track_all 的 layer 3 上
// 不保證有值（官方的平坦欄位表沒有表達巢狀結構），而 Start/End/Now 是必有的。
func remainingNumbers(t amego.Track) int64 {
	start, _ := strconv.ParseInt(t.Start, 10, 64)
	end, _ := strconv.ParseInt(t.End, 10, 64)
	if t.Now == "" {
		return end - start + 1
	}
	now, _ := strconv.ParseInt(t.Now, 10, 64)
	return end - now
}

// autoAssignedTracks 從 track_all 的三層巢狀結構裡挑出「自動配號」那一類。
//
// 三層是 1 財政部配給 → 2 給光貿用 → 3 發票字軌列表；自動配號的字軌是
// **layer 3 且 category == 1**（§4.5.10）。官方的平坦欄位表沒有表達這個結構，
// 直接把頂層當清單用會把「財政部配給總量」誤當成「可用字軌」。
func autoAssignedTracks(roots []amego.Track) []amego.Track {
	var out []amego.Track
	var walk func([]amego.Track)
	walk = func(ts []amego.Track) {
		for _, t := range ts {
			if t.Layer == 3 && t.Category == 1 {
				out = append(out, t)
			}
			if len(t.Data) > 0 {
				walk(t.Data)
			}
		}
	}
	walk(roots)
	return out
}

// exhausted 判定當期字軌已用盡（M21）：剩餘為 0，或字軌 status ∈ {3 過期, 9 用畢}。
func exhausted(t amego.Track) bool {
	return remainingNumbers(t) <= 0 || t.Status == 3 || t.Status == 9
}

// RunOnce 跑一輪字軌巡檢。
//
// 🔴 **單一賣家失敗不中斷整輪**：一家的憑證壞掉或 AMEGO 對該家回錯，不該讓其餘
// 賣家的餘量變成「量不到」。失敗的那家不計入任何一個計數器 —— 這是刻意的：把它
// 算成「餘量 0」會產生假的 InvoiceTrackExhausted，算成「餘量充足」會遮住真問題。
// 失敗本身由 invoice_amego_response_code_total（M3）與該家的健康檢查負責。
func (u *Track) RunOnce(ctx context.Context) error {
	accounts, err := u.accounts.ListActive(ctx)
	if err != nil {
		return err
	}

	now := u.now()
	curYear, curPeriod := domain.AmegoPeriod(now)
	// 下一期：往後推兩個月再換算，避開自己做 period+1 的跨年進位。
	nextYear, nextPeriod := domain.AmegoPeriod(now.AddDate(0, 2, 0))

	var (
		remainingMin  int64 = -1 // -1 ＝ 還沒有任何一家成功量到
		low           int64
		exhaustedCnt  int64
		nextInsuff    int64
		scanned, fail int
		promoted      int64
	)

	for _, acct := range accounts {
		res, err := u.scanOne(ctx, acct, curYear, curPeriod, nextYear, nextPeriod)
		if err != nil {
			fail++
			u.log.Warn("invoice.trk.seller_scan_failed",
				zap.Int64("provider_account_id", acct.ID),
				zap.Int64("issuer_id", acct.IssuerID),
				zap.Error(err))
			continue
		}
		scanned++
		if remainingMin < 0 || res.remaining < remainingMin {
			remainingMin = res.remaining
		}
		if res.remaining < u.warnThreshold {
			low++
		}
		if res.exhausted {
			exhaustedCnt++
		}
		if res.nextInsufficient {
			nextInsuff++
		}
		// J33：這位 issuer 的字軌餘量恢復了 ⇒ 把他卡在 SELLER_ACTION_REQUIRED 的
		// 票推回 READY（#3089 A 組第 3 項：在此之前這條邊零執行者）。
		//
		// 🔴 判準是「**現在**量得到餘量」而不是「這次掃描沒失敗」：掃描失敗那一支已經
		// `continue` 掉了，走到這裡代表 remaining 是真的量出來的數（memory
		// `feedback_no_blind_measurement`：量不到不等於量到 0）。
		//
		// **不需要 claim／lease**：集合式 UPDATE、無外部副作用（AMEGO 的字軌查詢是
		// scanOne 本職、已經做完），推出候選集合的寫入就是狀態轉移本身 ⇒ 第二個副本
		// rows=0。《多副本排程搶佔規範》例外 A，形態同 J05／J09。
		promoted += u.promoteRecovered(ctx, acct.IssuerID, res, now)
	}

	if remainingMin < 0 {
		remainingMin = 0
	}
	u.obs.ObserveTrackScan(remainingMin, low, exhaustedCnt, nextInsuff)
	u.log.Info("invoice.trk.scanned",
		zap.Int("accounts", len(accounts)),
		zap.Int("ok", scanned),
		zap.Int("failed", fail),
		zap.Int64("remaining_min", remainingMin),
		zap.Int64("low_sellers", low),
		zap.Int64("exhausted_sellers", exhaustedCnt),
		zap.Int64("next_period_insufficient_sellers", nextInsuff),
		zap.Int64("promoted_seller_action_required", promoted))
	return nil
}

// promoteRecovered 執行 J33。回推回去的列數（0 是常態：多數賣家從來沒卡過）。
//
// 🔴 **失敗不中止整輪掃描**：J33 是附加的復原動作，它壞掉不該讓字軌監控（本 worker
// 的主職）連帶停擺。但**必須留痕**，否則「票沒有被放行」在下一次發生時無從診斷。
func (u *Track) promoteRecovered(ctx context.Context, issuerID int64, res trackScanResult, now time.Time) int64 {
	if u.ops == nil || res.exhausted || res.remaining <= 0 {
		return 0
	}
	rows, err := u.ops.PromoteSellerActionRequired(ctx, issuerID, now)
	if err != nil {
		u.log.Warn("invoice.trk.promote_failed",
			zap.Int64("issuer_id", issuerID),
			zap.Int64("remaining", res.remaining),
			zap.Error(err))
		return 0
	}
	if rows > 0 {
		u.log.Info("invoice.trk.seller_action_required_promoted",
			zap.Int64("issuer_id", issuerID),
			zap.Int64("rows", rows),
			zap.Int64("remaining", res.remaining))
	}
	return rows
}

type trackScanResult struct {
	remaining        int64
	exhausted        bool
	nextInsufficient bool
}

// scanOne 打兩次 track_all（本期與下期），算出這一家的三個判定。
//
// 🔴 AppKey 明文的活動範圍只到 WithDecryptedCredential 的 callback：不回傳、不 log、
// 不進任何 struct 欄位（同 submit.go 的紀律）。
func (u *Track) scanOne(
	ctx context.Context, acct domain.ProviderAccount,
	curYear, curPeriod, nextYear, nextPeriod int,
) (trackScanResult, error) {
	var out trackScanResult
	err := u.accounts.WithDecryptedCredential(ctx, acct.ID, func(providerTaxID, appKey string) error {
		cred := amego.Credential{IssuerID: acct.IssuerID, Invoice: providerTaxID, AppKey: appKey}

		cur, err := u.amego.ListTracks(ctx, cred, curYear, curPeriod)
		if err != nil {
			return err
		}
		curTracks := autoAssignedTracks(cur)

		// 本期剩餘＝所有自動配號字軌的剩餘和；已用盡＝**每一段都用盡**。
		// 用 all 而不是 any：一家可以同時持有多段字軌，其中一段用完不代表開不了票。
		var remaining int64
		allExhausted := len(curTracks) > 0
		for _, t := range curTracks {
			r := remainingNumbers(t)
			if r > 0 {
				remaining += r
			}
			if !exhausted(t) {
				allExhausted = false
			}
		}
		out.remaining = remaining
		// 一段都沒有 ⇒ 也是用盡（而且是更糟的一種：連配號都沒有）。
		out.exhausted = allExhausted || len(curTracks) == 0

		next, err := u.amego.ListTracks(ctx, cred, nextYear, nextPeriod)
		if err != nil {
			return err
		}
		var nextBooklets int64
		for _, t := range autoAssignedTracks(next) {
			nextBooklets += int64(t.TotalBooklet)
		}
		// M24：下期本數 × 50 < 本期實際開立 ×(1＋headroom)。
		// 本期實際開立＝本期配到的總量 − 剩餘。
		var curTotal int64
		for _, t := range curTracks {
			s, _ := strconv.ParseInt(t.Start, 10, 64)
			e, _ := strconv.ParseInt(t.End, 10, 64)
			curTotal += e - s + 1
		}
		used := curTotal - remaining
		if used < 0 {
			used = 0
		}
		projected := float64(used) * (1 + u.growthHeadroom)
		out.nextInsufficient = float64(nextBooklets*numbersPerBooklet) < projected
		return nil
	})
	return out, err
}
