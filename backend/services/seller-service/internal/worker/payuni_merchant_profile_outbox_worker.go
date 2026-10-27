// payuni_merchant_profile_outbox_worker.go: 把商店地址變更送到 PayUni 的
// outbox worker。與 merchant_verification_outbox_worker.go 同為單階段——一筆
// row 一次副作用——但那支的副作用是發 MQ，這支是對外的 update_merchant 寫入
// 呼叫，所以 claim lease 長得多。
package worker

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"github.com/Will413028/hawkr/go-pkg/logger"
	"github.com/Will413028/hawkr/go-pkg/outbox"
	"github.com/Will413028/hawkr/seller-service/internal/domain"
	"github.com/Will413028/hawkr/seller-service/internal/usecase"
	"go.uber.org/zap"
)

const (
	pmpoMaxAttempts = 5
	pmpoBaseBackoff = 1 * time.Minute
	pmpoMaxBackoff  = 1 * time.Hour
	pmpoFetchBatch  = 50
	// pmpoClaimLease 比 merchant_verification 的 1 分鐘長：這裡要等一次對外
	// HTTP 呼叫（PayUni update_merchant）加上它自己的 advisory lock 等待，
	// 不是純本地發布（多副本排程搶佔規範 §三：lease 必須大於單筆最壞處理時間）。
	pmpoClaimLease = 3 * time.Minute
)

// merchantProfileUpdater 是呼叫 PayUni 的接縫，*usecase.PayuniUseCase 滿足它。
// 刻意複用既有 usecase 而不是自己組 update_merchant 請求：advisory lock、
// idempotency claim、審計列、7 欄回寫都在那裡面，重寫一份等於多一條會漂移的
// 路徑。
type merchantProfileUpdater interface {
	UpdateMerchantProfile(ctx context.Context, req *usecase.UpdateMerchantProfileRequest) (*usecase.UpdateMerchantProfileResult, error)
}

// merchantProfileVersionLookup 讀取當前的 merchant_profile_version。
//
// 版本現讀而不是投遞時存進 payload：賣家可能在這之間從 wallet 頁改過聯絡資料，
// 那條路徑會推進版本。存舊版本只會讓每一次重試都撞 CAS 而永遠送不出去。
type merchantProfileVersionLookup func(ctx context.Context, storeID int64) (int64, error)

// PayuniMerchantProfileOutboxWorker 消化 payuni_merchant_profile_outbox 的
// pending 列，逐筆呼叫 PayUni update_merchant。RunOnce 是 scheduler 每個
// tick 的唯一入口。
type PayuniMerchantProfileOutboxWorker struct {
	inner *outbox.Worker[*domain.PayuniMerchantProfileOutbox]
}

// NewPayuniMerchantProfileOutboxWorker 建構 worker。metrics 不可為 nil。
func NewPayuniMerchantProfileOutboxWorker(
	repo domain.PayuniMerchantProfileOutboxRepository,
	updater merchantProfileUpdater,
	version merchantProfileVersionLookup,
	metrics *outbox.Metrics,
) *PayuniMerchantProfileOutboxWorker {
	if metrics == nil {
		panic("worker.NewPayuniMerchantProfileOutboxWorker: metrics must not be nil")
	}

	process := func(ctx context.Context, row *domain.PayuniMerchantProfileOutbox) (any, error) {
		var p domain.PayuniMerchantProfilePayload
		if err := json.Unmarshal(row.Payload, &p); err != nil {
			return nil, fmt.Errorf("unmarshal payload: %w", err)
		}
		v, err := version(ctx, row.StoreID)
		if err != nil {
			return nil, fmt.Errorf("read merchant profile version: %w", err)
		}

		// 只填地址兩欄。其餘 5 欄留空是刻意的，兩端都把空字串當「不要動」：
		// PayUni 的 wire struct 全帶 omitempty（pkg/payuni/client.go），本地
		// 回寫是 COALESCE(NULLIF(...), 既有值)（UpdatePayuniMerchantProfile）。
		//
		// IdempotencyKey 用 outbox row id：重試時不能變，否則同一筆變動會被
		// 當成兩次不同的請求。不變才讓「PayUni 已成功但 MarkDone 失敗」的下
		// 一輪走 SUCCEEDED replay，而不是再打一次 PayUni。
		_, err = updater.UpdateMerchantProfile(ctx, &usecase.UpdateMerchantProfileRequest{
			StoreID:         row.StoreID,
			AccountID:       row.AccountID,
			IdempotencyKey:  strconv.FormatInt(row.ID, 10),
			ExpectedVersion: v,
			MerAddr:         p.MerAddr,
			MerAddrPost:     p.MerAddrPost,
		})
		if err != nil {
			return nil, fmt.Errorf("update merchant profile: %w", err)
		}
		return nil, nil
	}

	hooks := outbox.Hooks[*domain.PayuniMerchantProfileOutbox]{
		OnAttemptFailed: func(ctx context.Context, row *domain.PayuniMerchantProfileOutbox, cause string, err error) {
			logger.Warn(ctx, "payuni-merchant-profile-outbox: attempt failed",
				zap.Int64("id", row.ID), zap.Int64("storeID", row.StoreID),
				zap.String("cause", cause), zap.Error(err))
			metrics.AttemptFailedTotal(cause).Inc()
		},
		OnTerminalFailed: func(ctx context.Context, row *domain.PayuniMerchantProfileOutbox, newAttempt int, lastErr string, markFailedErr error) {
			if markFailedErr != nil {
				logger.Warn(ctx, "payuni-merchant-profile-outbox: MarkFailed error (degraded)",
					zap.Int64("id", row.ID), zap.Int64("storeID", row.StoreID),
					zap.Int("attempt", newAttempt), zap.String("lastErr", lastErr), zap.Error(markFailedErr))
				return
			}
			// 終結的 row 代表本地地址與 PayUni 登記值已經分歧，且系統不會再自動
			// 收斂——需要人工用代理商後台或 payuni-api 工具補送。
			logger.Warn(ctx, "payuni-merchant-profile-outbox: row exhausted all retries → failed",
				zap.Int64("id", row.ID), zap.Int64("storeID", row.StoreID),
				zap.Int("attempt", newAttempt), zap.String("lastErr", lastErr))
			metrics.FailedTotal().Inc()
		},
	}

	cfg := outbox.Config{
		MaxAttempts: pmpoMaxAttempts,
		BaseBackoff: pmpoBaseBackoff,
		MaxBackoff:  pmpoMaxBackoff,
		FetchBatch:  pmpoFetchBatch,
		ClaimLease:  pmpoClaimLease,
	}

	return &PayuniMerchantProfileOutboxWorker{
		inner: outbox.New[*domain.PayuniMerchantProfileOutbox](repo, process, cfg, hooks),
	}
}

// RunOnce 處理一批 pending 且到期的列後回傳。
func (w *PayuniMerchantProfileOutboxWorker) RunOnce(ctx context.Context) error {
	return w.inner.RunOnce(ctx)
}
