package usecase

import (
	"context"
	"strconv"
	"time"

	"github.com/Will413028/hawkr/invoice-service/internal/client/amego"
	"github.com/Will413028/hawkr/invoice-service/internal/domain"
	"go.uber.org/zap"
)

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

func remainingNumbers(t amego.Track) int64 {
	start, _ := strconv.ParseInt(t.Start, 10, 64)
	end, _ := strconv.ParseInt(t.End, 10, 64)
	if t.Now == "" {
		return end - start + 1
	}
	now, _ := strconv.ParseInt(t.Now, 10, 64)
	return end - now
}

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
	nextYear, nextPeriod := domain.AmegoPeriod(now.AddDate(0, 2, 0))

	var (
		remainingMin  int64 = -1
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
		out.exhausted = allExhausted || len(curTracks) == 0

		next, err := u.amego.ListTracks(ctx, cred, nextYear, nextPeriod)
		if err != nil {
			return err
		}
		var nextBooklets int64
		for _, t := range autoAssignedTracks(next) {
			nextBooklets += int64(t.TotalBooklet)
		}
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
