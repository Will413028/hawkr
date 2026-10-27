package worker

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"time"

	"go.uber.org/zap"

	pkgerrors "github.com/Will413028/hawkr/go-pkg/errors"
	"github.com/Will413028/hawkr/go-pkg/errors/bizcode"
	"github.com/Will413028/hawkr/go-pkg/revalidate"
	"github.com/Will413028/hawkr/go-pkg/timeutil"
	"github.com/Will413028/hawkr/social-service/internal/config"
	"github.com/Will413028/hawkr/social-service/internal/domain"
	"github.com/Will413028/hawkr/social-service/internal/observability"
	"github.com/Will413028/hawkr/social-service/internal/repository/postgres/gen"
	"github.com/Will413028/hawkr/social-service/internal/usecase"
)

// postSchedulePublisher 是 56 §7.6 的排程發布 worker。
//
// 形狀是《多副本排程搶佔規範》所說的「批次讀候選 → 逐筆做副作用」，因此五個不變式
// 逐條落在這支：① 用租約不用永久旗標 ② 租約 > 單筆最壞耗時（由 Config.Validate 在
// 啟動時斷言）③ 租約 < 候選窗口（同上）④ 租約基準逐筆取 ⑤ 終寫也是 CAS。
//
// prod 現況 social-service 是單副本，所以今天跨 pod 重複還不會發生；但規範第 ⑧ 條要求
// 調高 replicas 之前排程必須已有合格搶佔——沒有閘門的排程器是擴容那一刻才引爆的地雷，
// 而擴容通常是為了應付流量、沒有人會在那時想到排程器。
type postSchedulePublisher struct {
	cfg      config.ScheduledPostPublishConfig
	repo     domain.ScheduledPostRepository
	hashtags *usecase.HashtagUseCase
	audit    usecase.DelegationAuditClient
	reval    *revalidate.Client
	logger   *zap.Logger

	// owner 只是觀測值：正確性由 schedule_lease_expires_at 的 CAS 承擔，不由這個字串
	// 的唯一性承擔。兩個 pod 若碰巧同名，CAS 仍然只會有一個贏。
	owner string
	// now 讓測試控制時間；生產一律 time.Now().UTC()。
	now func() time.Time
}

func newPostSchedulePublisher(
	cfg config.ScheduledPostPublishConfig,
	repo domain.ScheduledPostRepository,
	hashtags *usecase.HashtagUseCase,
	audit usecase.DelegationAuditClient,
	reval *revalidate.Client,
	logger *zap.Logger,
) *postSchedulePublisher {
	host, err := os.Hostname()
	if err != nil || host == "" {
		host = "unknown-host"
	}
	return &postSchedulePublisher{
		cfg:      cfg,
		repo:     repo,
		hashtags: hashtags,
		audit:    audit,
		reval:    reval,
		logger:   logger,
		owner:    fmt.Sprintf("%s/%d", host, os.Getpid()),
		now:      func() time.Time { return time.Now().UTC() },
	}
}

// tick 的順序是固定的：三條收斂 → 候選 SELECT → 逐列 claim。
//
// 收斂不能省。少了它，失效的列會停在 pending 被每個 tick 重撈、CAS 恆為 rows = 0
// （tick 30 秒 × 24 小時 ＝ 每列 2,880 次無效 CAS），最後由第三條轉 expired ⇒ 逾期告警
// 會為一個完全預期、無需處置的情形而響，而一條開始響假訊號的告警等於作廢。
func (p *postSchedulePublisher) tick(ctx context.Context) {
	p.converge(ctx)

	now := p.now()
	ids, err := p.repo.ListDue(ctx, now, p.cfg.WindowHours, p.cfg.BatchSize)
	if err != nil {
		p.logger.Error("scheduled post: list due failed", zap.Error(err))
		return
	}
	for _, id := range ids {
		if ctx.Err() != nil {
			return
		}
		p.publishOne(ctx, id)
	}
}

// converge 跑三條收斂語句。順序固定：刪除 → 下架 → 窗口。
//
// 三條都不帶租約守衛（見 SQL 註解），因此它們會與正被持有的租約互相覆寫——那是設計的
// 一部分：收斂把某列轉終態時，持有租約的 pod 到了終寫會因 CAS 的 schedule_state =
// 'pending' 述詞拿到 rows = 0 而放棄發布。fail-closed 由終寫承擔，不由收斂的述詞承擔。
func (p *postSchedulePublisher) converge(ctx context.Context) {
	now := p.now()

	if n, err := p.repo.ConvergeDeleted(ctx, now); err != nil {
		p.logger.Error("scheduled post: converge deleted failed", zap.Error(err))
	} else if n > 0 {
		observability.DelegationScheduledPostCancelledTotal.
			WithLabelValues(observability.ScheduleCancelReasonDeletion).Add(float64(n))
		p.logger.Info("scheduled post: cancelled by deletion", zap.Int64("rows", n))
	}

	if n, err := p.repo.ConvergeHidden(ctx, now); err != nil {
		p.logger.Error("scheduled post: converge hidden failed", zap.Error(err))
	} else if n > 0 {
		observability.DelegationScheduledPostCancelledTotal.
			WithLabelValues(observability.ScheduleCancelReasonGovernance).Add(float64(n))
		p.logger.Info("scheduled post: cancelled by governance", zap.Int64("rows", n))
	}

	if n, err := p.repo.ConvergeExpired(ctx, now, p.cfg.WindowHours); err != nil {
		p.logger.Error("scheduled post: converge expired failed", zap.Error(err))
	} else if n > 0 {
		observability.DelegationScheduledPostExpiredTotal.
			WithLabelValues(observability.ScheduleExpiredReasonWindowExceeded).Add(float64(n))
		p.logger.Warn("scheduled post: expired past the candidate window", zap.Int64("rows", n))
	}
}

func (p *postSchedulePublisher) publishOne(ctx context.Context, id int64) {
	// 不變式 4：租約基準在**這一列**的 claim 當下取。在批次迴圈外算一次，不變式就從
	// 「單筆最壞耗時 < 租約」退化成「批次總耗時 < 租約」，隨 batch-size 一起破。
	claimAt := p.now()
	leaseUntil := claimAt.Add(time.Duration(p.cfg.LeaseSeconds) * time.Second)

	row, err := p.repo.Claim(ctx, id, p.owner, claimAt, leaseUntil, p.cfg.WindowHours)
	if err != nil {
		p.logger.Error("scheduled post: claim failed", zap.Int64("post_id", id), zap.Error(err))
		return
	}
	if row == nil {
		// 沒搶到，或該列在候選 SELECT 之後已不合資格。兩者都不是錯誤。
		return
	}

	log := p.logger.With(zap.Int64("post_id", id), zap.String("lease_owner", p.owner))

	if row.ScheduledBySessionID == nil || row.ScheduledPublishAt == nil {
		// CHECK 約束保證 pending 必有 scheduled_publish_at；session id 則是排程建立路徑
		// 一定會寫的值。走到這裡代表有人繞過那條路徑寫了 pending，稽核歸屬無從建立 ⇒
		// 不發布。走失敗釋放而不是直接跳過，是為了讓它最後落進逾期告警而不是每個 tick
		// 靜默重試到永遠。
		log.Error("scheduled post: row is pending without a schedule owner or time")
		p.releaseAfterFailure(ctx, row, "", log)
		return
	}

	scheduledAt := *row.ScheduledPublishAt
	// 冪等鍵的第二段取被 claim 的那一列的 scheduled_publish_at，不是 now()、也不是
	// worker 進 tick 的時間：用時間點以外的任何東西當第二段，重試就會各自算出不同的
	// 鍵，整條冪等保證失效，一則貼文重試三次留下三列假 PENDING 稽核。
	idempotencyKey := fmt.Sprintf("schedule:%d:%d", row.ID, scheduledAt.Unix())
	log = log.With(zap.String("idempotency_key", idempotencyKey))

	intent, err := p.recordIntent(ctx, row, idempotencyKey)
	if err != nil {
		// 9271 ＝ 工作階段已被撤銷。撤銷有三個來源（人工撤銷／ADM 登出連動／目標店家
		// 轉為不可代行），worker 端只有這一條分支；要分辨得讀 session 列的 revoke_reason。
		// fail-closed 發生在執行時，不做任何跨服務推送——那會變成補償流程。
		if pkgerrors.IsCode(err, bizcode.DelegationSessionRevoked) {
			p.cancelByRevocation(ctx, row, log)
			return
		}
		log.Warn("scheduled post: audit intent failed", zap.Error(err))
		p.releaseAfterFailure(ctx, row, "", log)
		return
	}

	publishCtx, cancel := context.WithTimeout(ctx, msDuration(p.cfg.TxBudgetMs))
	defer cancel()

	publishedAt := p.now()
	var bustTags []int64
	published, err := p.repo.Publish(publishCtx, row.ID, p.owner, domain.PostPublishStatusPublished, publishedAt,
		func(txCtx context.Context, q gen.DBTX) error {
			// hashtag 計數的維護時點是「轉為已發布」而不是「建立」：草稿不計數，否則
			// 只被代行草稿用過的標籤會出現在公開自動完成清單裡（述詞是 post_count > 0）。
			// 與狀態轉換同一個交易，否則中間崩潰會留下一則已發布但計數少一的貼文，
			// 而本專案沒有、也不允許有對帳程序去發現它。
			adjusted, cerr := p.hashtags.AdjustCountersForVisibilityChange(txCtx, q, row.ID, 1)
			if cerr != nil {
				return cerr
			}
			bustTags = adjusted
			return nil
		})
	if err != nil {
		log.Error("scheduled post: publish transaction failed", zap.Error(err))
		p.releaseAfterFailure(ctx, row, intent.AuditLogID, log)
		return
	}
	if !published {
		// 終寫 CAS 的 rows = 0。兩件事仍然不做：
		//  ① 不累加取消／逾期指標——該列的終態由收斂語句或接手的 pod 負責寫，計數歸
		//     它們；重複記會讓 reason 分佈失真。
		//  ② 不清租約、不推遲下次嘗試——租約若已被別人持有，動它就是清別人的租約。
		//
		// 稽核則必須依成因分流。rows = 0 的兩個成因在這裡完全同形，處置卻相反：
		//  ⓐ 租約被接手 ⇒ 接手者用同一個冪等鍵拿到同一列 auditLogId，而 confirm 的述詞
		//     是 WHERE result='PENDING' 先到先贏 ⇒ 本 pod 搶著寫 FAILED 會擋掉它隨後的
		//     SUCCEEDED，稽核記成失敗、outbox 一列不建、賣家收不到通知，而貼文其實已經
		//     發出去了。這一側維持不寫。
		//  ⓑ 租約仍在我手上 ⇒ 沒有接手者，rows = 0 必然是另外三個述詞之一不成立（已刪除
		//     ／已下架／已收斂轉態），該列不會再回到候選集合 ⇒ **沒有任何人會再來關這列
		//     稽核**。原本的「一律不寫」在這一側讓它永久停在 PENDING，而 PENDING 的語意
		//     是「不知道寫入成不成立」——這裡明確知道它不成立，寫得出 FAILED。
		//
		// 探針失敗時退回不寫：誤寫 FAILED 會擋掉真實結論（ⓐ 的傷害），漏寫只是留一列
		// PENDING（ⓑ 的傷害）。兩者不對等，所以不確定時選後者。
		heldByMe, err := p.repo.IsLeaseHeldBy(ctx, row.ID, p.owner)
		if err != nil {
			log.Warn("scheduled post: lease ownership probe failed, leaving audit row PENDING",
				zap.String("audit_log_id", intent.AuditLogID), zap.Error(err))
			return
		}
		if !heldByMe {
			log.Info("scheduled post: terminal CAS lost to another holder, leaving audit row for them")
			return
		}
		log.Info("scheduled post: terminal CAS lost while still holding the lease, closing audit row as failed")
		p.confirmFailure(ctx, intent.AuditLogID, bizcode.DelegationScheduledPostNotPublishable, log)
		return
	}

	log.Info("scheduled post: published", zap.Time("published_at", publishedAt))

	if err := p.confirm(ctx, intent.AuditLogID, row, publishedAt); err != nil {
		// 貼文已經發出去了，稽核列留在 PENDING（§7.3 的視窗 A）。這裡不能把它改寫成
		// 別的狀態，也不能回滾發布——已經對買家可見的東西收不回來。
		log.Error("scheduled post: audit confirm failed, audit row stays PENDING",
			zap.String("audit_log_id", intent.AuditLogID), zap.Error(err))
	}

	p.bust(ctx, row.ID, bustTags)
}

// recordIntent 寫下 PENDING 稽核列。origin=scheduled 讓 account-service 改用排程的授權
// 條件（status <> 'revoked'，允許已到期）——到期的是操作者的操作權，不是已授權動作的
// 效力。它是提權欄位，account-service 只在內部授權簽章驗證通過且 serviceName ==
// social-service 時才接受這個值。
func (p *postSchedulePublisher) recordIntent(ctx context.Context, row *gen.CmsPost, idempotencyKey string) (*usecase.DelegationAuditIntentResult, error) {
	ctx, cancel := context.WithTimeout(ctx, msDuration(p.cfg.AuditIntentDeadlineMs))
	defer cancel()

	before, err := json.Marshal(map[string]string{
		"publishStatus":      strconv.Itoa(int(row.PublishStatus)),
		"scheduledPublishAt": timeutil.FormatISO8601(*row.ScheduledPublishAt),
	})
	if err != nil {
		return nil, err
	}

	return p.audit.RecordAuditIntent(ctx, usecase.DelegationAuditIntent{
		SessionID:    strconv.FormatInt(*row.ScheduledBySessionID, 10),
		Capability:   domain.DelegationCapabilityPosts,
		Action:       domain.DelegationActionPostPublish,
		ResourceType: domain.DelegationResourceTypePost,
		ResourceID:   strconv.FormatInt(row.ID, 10),
		// OperationReason 留空：排程執行沒有 HTTP 請求可帶理由，而理由的 SSOT 是建立
		// 排程當下寫下的那一列 POST_SCHEDULE 稽核（同 session、同貼文），它在
		// account-service 自己的資料庫裡——由該端在 intent 內繼承過來（§5.1.3 三態表
		// 的 ⚠️ 段），這一端送什麼都會被丟棄。由這一端猜一句話填進去等於偽造稽核，
		// 而繼承不到時該端一律拒絕，本 worker 因此走失敗釋放、最後落進逾期告警。
		OperationReason: "",
		// 排程執行只改 publish_status 與 published_at，不覆蓋任何內容欄位、也不 bump
		// version ⇒ 前後版本相等。記錄的是「發布時處於哪個版本」，不宣稱這次改了版本。
		ResourceVersionBefore: strconv.FormatInt(row.Version, 10),
		BeforeSnapshotJSON:    string(before),
		IdempotencyKey:        idempotencyKey,
		// 沒有 HTTP 請求，因此沒有 request id；correlation 取冪等鍵，它在同一則貼文的
		// 每次重試之間都相同，log 上因此撈得出「這一次排程發布」的完整歷程。
		CorrelationID: idempotencyKey,
		NotifySeller:  true,
		Origin:        domain.DelegationOriginScheduled,
	})
}

// confirm 在總預算內重試 ConfirmAuditResult。
//
// 逐次 deadline 與總預算是兩個不同的東西：前者防單次呼叫 hang 住，後者是租約下界不等式
// 右邊的那一項——沒有總預算，最壞耗時就沒有上界，租約長度也就無從斷言。
func (p *postSchedulePublisher) confirm(ctx context.Context, auditLogID string, row *gen.CmsPost, publishedAt time.Time) error {
	total, cancelTotal := context.WithTimeout(ctx, msDuration(p.cfg.AuditConfirmTotalBudgetMs))
	defer cancelTotal()

	after, err := json.Marshal(map[string]string{
		"publishStatus": strconv.Itoa(int(domain.PostPublishStatusPublished)),
		"publishedAt":   timeutil.FormatISO8601(publishedAt),
	})
	if err != nil {
		return err
	}

	in := usecase.DelegationAuditConfirm{
		AuditLogID:           auditLogID,
		Result:               "SUCCEEDED",
		ResourceID:           strconv.FormatInt(row.ID, 10),
		ResourceVersionAfter: strconv.FormatInt(row.Version, 10),
		AfterSnapshotJSON:    string(after),
		ChangedFields: []usecase.DelegationChangedField{{
			Field:  "publishStatus",
			Label:  "發布狀態",
			Before: strconv.Itoa(int(row.PublishStatus)),
			After:  strconv.Itoa(int(domain.PostPublishStatusPublished)),
		}},
	}

	var lastErr error
	for attempt := 1; attempt <= p.cfg.AuditConfirmMaxAttempts; attempt++ {
		callCtx, cancel := context.WithTimeout(total, msDuration(p.cfg.AuditConfirmDeadlineMs))
		err := p.audit.ConfirmAuditResult(callCtx, in)
		cancel()
		if err == nil {
			return nil
		}
		lastErr = err
		if total.Err() != nil {
			break
		}
	}
	return lastErr
}

// confirmFailure 把稽核列關成 FAILED。與 confirm 共用同一組 deadline／預算語意，差別只在
// 沒有 after 快照與 changed_fields —— 寫入沒有發生，這兩者留空才是誠實的。
//
// 失敗只記一行 Warn 就走：這個呼叫點本身就是「已經沒有別人會來關這一列」的收尾，再往上
// 拋沒有人接得住；而重試已經在 confirm 那一層做過（同一組總預算），這裡再包一層只會把
// tick 拖長。留下來的 PENDING 由告警負責，那正是它存在的理由。
func (p *postSchedulePublisher) confirmFailure(ctx context.Context, auditLogID string, bizCode int32, log *zap.Logger) {
	total, cancelTotal := context.WithTimeout(ctx, msDuration(p.cfg.AuditConfirmTotalBudgetMs))
	defer cancelTotal()

	in := usecase.DelegationAuditConfirm{
		AuditLogID: auditLogID,
		Result:     "FAILED",
		BizCode:    bizCode,
	}

	var lastErr error
	for attempt := 1; attempt <= p.cfg.AuditConfirmMaxAttempts; attempt++ {
		callCtx, cancel := context.WithTimeout(total, msDuration(p.cfg.AuditConfirmDeadlineMs))
		err := p.audit.ConfirmAuditResult(callCtx, in)
		cancel()
		if err == nil {
			return
		}
		lastErr = err
		if total.Err() != nil {
			break
		}
	}
	log.Error("scheduled post: audit confirm(FAILED) failed, audit row stays PENDING",
		zap.String("audit_log_id", auditLogID), zap.Int32("biz_code", bizCode), zap.Error(lastErr))
}

// releaseAfterFailure 還原自己的租約並排下一次嘗試；重試用盡則轉 expired。
//
// 述詞帶 schedule_lease_owner = 自己（見 SQL）：釋放是「還原我自己的租約」，動到別人的
// 就是搶。這與「三條收斂不帶守衛」不矛盾，兩者方向相反。rows = 0 不是錯誤。
// auditLogID 為空表示這一次失敗發生在 intent 成立之前（沒有 owner／時間、或 intent 本身
// 就失敗），沒有稽核列可關。
func (p *postSchedulePublisher) releaseAfterFailure(ctx context.Context, row *gen.CmsPost, auditLogID string, log *zap.Logger) {
	now := p.now()

	if int(row.ScheduleRetryCount)+1 >= p.cfg.MaxRetry {
		ok, err := p.repo.ExhaustRetries(ctx, row.ID, p.owner, now)
		if err != nil {
			log.Error("scheduled post: exhaust retries failed", zap.Error(err))
			return
		}
		if ok {
			observability.DelegationScheduledPostExpiredTotal.
				WithLabelValues(observability.ScheduleExpiredReasonRetryExhausted).Inc()
			log.Warn("scheduled post: expired after exhausting retries",
				zap.Int32("retry_count", row.ScheduleRetryCount+1))
			// ok = true ⇒ 述詞帶 owner 且成立 ⇒ 租約在我手上、且這一列剛被我推進 expired
			// 終態。沒有接手者會來關這列稽核，也不會再有下一次重試 —— 這裡是最後一個
			// 知道它失敗的地方。不寫就永久 PENDING。
			//
			// ok = false 則相反：租約已不在我手上或該列已被收斂，終態由別人寫，稽核也歸
			// 它們，這裡沉默是對的。
			if auditLogID != "" {
				p.confirmFailure(ctx, auditLogID, bizcode.DelegationScheduledPostRetryExhausted, log)
			}
		}
		return
	}

	next := now.Add(p.backoff(row.ScheduleRetryCount))
	if _, err := p.repo.ReleaseLease(ctx, row.ID, p.owner, now, next); err != nil {
		log.Error("scheduled post: release lease failed", zap.Error(err))
		return
	}
	log.Info("scheduled post: lease released for retry", zap.Time("next_attempt_at", next))
}

// backoff 對應 Config.Validate 用的那一條序列：delay(k) = LEAST(base × 2^(k-1), cap)，
// 其中 k 是第幾次失敗。retryCount 是這一次失敗之前已經失敗過的次數，因此 k-1 = retryCount。
// 兩處的算式必須一致，否則啟動時斷言「退避走得完候選窗口」，實際跑的卻是另一條序列。
func (p *postSchedulePublisher) backoff(retryCount int32) time.Duration {
	delay := p.cfg.BackoffBaseSeconds
	for i := int32(0); i < retryCount; i++ {
		if delay >= p.cfg.BackoffMaxSeconds {
			break
		}
		delay *= 2
	}
	if delay > p.cfg.BackoffMaxSeconds {
		delay = p.cfg.BackoffMaxSeconds
	}
	return time.Duration(delay) * time.Second
}

func (p *postSchedulePublisher) cancelByRevocation(ctx context.Context, row *gen.CmsPost, log *zap.Logger) {
	ok, err := p.repo.CancelByRevocation(ctx, row.ID, p.owner, p.now())
	if err != nil {
		log.Error("scheduled post: cancel by revocation failed", zap.Error(err))
		return
	}
	if ok {
		observability.DelegationScheduledPostCancelledTotal.
			WithLabelValues(observability.ScheduleCancelReasonRevocation).Inc()
		// 取消一律不通知賣家、不寫代行稽核（與另外兩種取消對齊）；可解釋性由這一支
		// counter 與稽核查詢承擔。
		log.Info("scheduled post: cancelled, delegation session was revoked")
	}
}

// bust 讓官網把這則貼文與計數變動過的標籤重新取一次。
// 只送 entity tag；domain-collection tag 是 web 邊界的職責。best-effort，未設定即 no-op。
func (p *postSchedulePublisher) bust(ctx context.Context, postID int64, hashtagIDs []int64) {
	if p.reval == nil || !p.reval.Enabled() {
		return
	}
	tags := append([]string{revalidate.PostTag(postID)}, revalidate.HashtagTags(hashtagIDs...)...)
	p.reval.BustAsync(ctx, tags)
}

func msDuration(ms int) time.Duration {
	return time.Duration(ms) * time.Millisecond
}
