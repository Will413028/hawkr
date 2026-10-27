package io.github.will413028.hawkr.payment.logistics.scheduler;

import io.github.will413028.hawkr.common.obs.log.ObsLogger;
import io.github.will413028.hawkr.payment.logistics.application.LogisticsCollectionService;
import io.github.will413028.hawkr.payment.logistics.domain.ReceivableStatusRules;
import io.github.will413028.hawkr.payment.logistics.repository.LogisticsCollectionAttemptRepository;
import io.github.will413028.hawkr.payment.logistics.repository.LogisticsReceivableRepository;
import io.github.will413028.hawkr.payment.logistics.repository.entity.LogisticsCollectionAttemptEntity;
import io.github.will413028.hawkr.payment.logistics.repository.entity.LogisticsReceivableEntity;
import io.micrometer.core.instrument.Gauge;
import io.micrometer.core.instrument.MeterRegistry;
import java.util.List;
import java.util.Optional;
import org.springframework.beans.factory.annotation.Value;
import org.springframework.boot.autoconfigure.condition.ConditionalOnProperty;
import org.springframework.scheduling.annotation.Scheduled;
import org.springframework.stereotype.Component;

/**
 * 線 B 收款的兩支租約 worker：<b>提交</b>（PENDING → 送出）與<b>查詢</b>（已送出 → 入帳）。
 *
 * <h2>🔴 為什麼一定是 lease 而不是永久旗標（10 檔 §29）</h2>
 * 沒有租約的後果很具體：兩個 pod 同時撈到同一列 PENDING ⇒ 各自生一個 TSID {@code trade_no}
 * ⇒ 各自 pre-commit ⇒ <b>各自呼叫 PayUni</b> ⇒ 同一筆應收被收兩次錢。
 * <ul>
 *   <li>⚠️ {@code uq_logcoll_trade_no} 擋不住：兩個 pod 的 trade_no 本來就不同。</li>
 *   <li>⚠️ {@code PENDING → SUBMITTED} 的 CAS 也擋不住：它發生在呼叫 PayUni <b>之後</b>。</li>
 * </ul>
 * payment 今天是 {@code replicas: 2}（10 檔 §29 的五支 overlay 之一）⇒ 這不是預防性設計，
 * 是 live 條件。
 *
 * <h2>🔴 租約長度的上下界（§29 要求逐支驗算，算式如下）</h2>
 *
 * <h3>單筆最壞耗時</h3>
 * <pre>
 *   PayUni 同步呼叫  = connect 30s + write 30s + read 30s = 90s
 *                      （PayUniHttpClient 的 OkHttp 三個 timeout，實查該檔）
 *   前後 DB 寫入     ≈ 10s（pre-commit 1 句 ＋ 收尾交易 2~5 句，含連線等待）
 *   對第三方友善的 sleep = perItemSleepMs（預設 100ms）
 *   ───────────────────────────────────────────────
 *   單筆最壞 W       ≈ 100s   （= WORST_CASE_ITEM_SECONDS）
 * </pre>
 *
 * <h3>下界：租約 &gt; 批次總耗時（不是單筆）</h3>
 * ⚠️ 這一條是<b>實測出來的</b>，不是理論：{@code NOW()} 在 PostgreSQL 是<b>交易開始時間</b>，
 * 所以同一次 claim 撈到的整批列<b>共用同一個租約起點</b>（資料層 mapper 的 javadoc 記了
 * {@code batchSize=10} 撈 2 筆、兩筆 {@code *_lease_until} 位元相同的實測）。
 * ⇒ 不變式不是「單筆最壞耗時 &lt; 租約」而是
 * <pre>    batchSize × W  &lt;  leaseSeconds</pre>
 * 預設值代入：{@code 5 × 100 = 500 &lt; 600} ✅（餘裕 100s）。
 * 🔴 這條不變式在 {@link #validateLeaseBounds} 內以<b>開機即拋</b>的形式守住，
 * 不是靠這段註解——調大 {@code batch-size} 卻忘了調租約，會在啟動時就現形，
 * 而不是在 prod 上變成一筆重複扣款。
 *
 * <h3>上界：租約 &lt; 候選窗口的剩餘壽命</h3>
 * <ul>
 *   <li><b>提交</b>：候選述詞是 {@code status='PENDING' AND (lease IS NULL OR lease &lt;= NOW())}。
 *       一列 PENDING <b>不會自己離開候選集合</b>（沒有任何時間欄讓它過期）⇒ 候選窗口壽命無上界
 *       ⇒ 這個上界<b>恆成立</b>，不是被忽略。
 *       真正的上界因此是維運選擇：一個崩掉的 pod 最多可以把一筆應收擋多久。
 *       取 {@code MAX_LEASE_SECONDS = 3600}（1 小時），同樣在開機時守。</li>
 *   <li><b>查詢</b>：候選述詞是 {@code fund_date IS NULL AND submitted_at IS NOT NULL AND ...}，
 *       同樣不會自己過期；但這裡有一個<b>更緊</b>的窗口——
 *       {@code expected_fund_date + fundDateGraceHours} 之後就該判 UNCONFIRMED，
 *       租約若接近寬限期，一個崩掉的持有者會實質延後那個判定。
 *       ⇒ 要求 {@code queryLease ≪ grace}；預設 {@code 600s / 86400s = 0.7%}。</li>
 * </ul>
 *
 * <h2>retry 的 backoff ＝ 租約本身</h2>
 * 一筆在送出途中崩掉（或拋例外）的列<b>不歸還租約</b> ⇒ 它要等租約自然到期（預設 600s）
 * 才會再次進入候選集合。這就是退避間隔，不需要第二個計時欄
 * （V9 也沒有 {@code retry_at} 這種欄位，加一個等於改 frozen 的 DDL）。
 * ⚠️ 因此 {@code releaseSubmitLease}／{@code releaseQueryLease} 在本 worker 內<b>刻意不呼叫</b>：
 * 提前歸還會把退避從 600 秒變成 0 秒，對一個剛剛打爆的第三方接口連續重打。
 *
 * <h2>⚠️ 誰把每個狀態推走（{@code 06-no-small-tails.md} §3.13 第 ② 問）</h2>
 * <table border="1">
 *   <caption>本波的執行者盤點——<b>答不出執行者的格子寫在這裡，不寫在回報裡就會消失</b></caption>
 *   <tr><th>狀態</th><th>誰推進來</th><th>誰推走</th></tr>
 *   <tr><td>PENDING</td><td>送達事件 consumer（{@code createReceivable}）</td>
 *       <td>{@link #runSubmit}</td></tr>
 *   <tr><td>SUBMITTED</td><td>{@link #runSubmit}</td><td>{@link #runQuery}</td></tr>
 *   <tr><td>SETTLED</td><td>{@link #runQuery}</td>
 *       <td>🔴 <b>本波沒有</b>——出邊只有 {@code → CLOSED}（true-up），屬 provider 對帳那一波。
 *           SETTLED <b>不是終態</b>，這一格空著是預期的、不是遺漏</td></tr>
 *   <tr><td>UNCONFIRMED</td><td>{@link #runQuery}</td>
 *       <td>{@link #runQuery}（查證為真 → SETTLED）；另兩條出邊
 *           （→ PENDING 開新 attempt、→ WRITTEN_OFF）是 ADM 人工，本波無端點</td></tr>
 *   <tr><td>FAILED</td><td>{@code markRejectedInTx}</td>
 *       <td>🔴 <b>今天沒有人</b>。{@link ReceivableStatusRules} 允許 {@code FAILED → SUBMITTED}
 *           （退避重送），但 repository 只暴露 {@code claimPendingForSubmit}，
 *           而它寫死 PENDING；V9 的 {@code idx_logrecv_submit_lease} 也是
 *           {@code WHERE status = 'PENDING'} 的 partial index。
 *           ⇒ 明確被 PayUni 拒絕的應收會<b>停在 FAILED 不再重試</b>，
 *           只能靠 ADM 例外佇列（{@code listByStatus}）人工處理。
 *           修法是資料層的 {@code claimForSubmitByStatus} 開放給 FAILED ＋ 索引覆蓋，
 *           不在本波的檔案範圍內</td></tr>
 * </table>
 *
 * <p>⚠️ 另一個已知的殘留：ADM 把一筆 UNCONFIRMED 退回 PENDING 去開新 attempt 之後，
 * <b>舊的那一列 attempt</b> 仍然滿足 {@code claimForQuery} 的述詞
 * （{@code fund_date IS NULL AND submitted_at IS NOT NULL}）⇒ 每個租約週期會被 claim 一次，
 * 由 {@link LogisticsCollectionService.QueryResult#SKIPPED_NOT_QUERYABLE} 擋下。
 * <b>不會打 PayUni</b>（狀態檢查在呼叫之前），代價是每個租約週期兩次 DB 往返。
 * 要根治需要 attempt 側的「已放棄」欄位，而 V9 已凍結。
 *
 * <h2>🔴 兩道 Enablement Gate，關著時<b>連 claim 都不做</b></h2>
 * <ol>
 *   <li>費率政策未查證（EV-P6）⇒ {@code auto-submit-enabled=false}（預設）</li>
 *   <li>{@link LogisticsCollectionService.SellerMerchantIdResolver} 沒有 bean
 *       ⇒ 解析不出賣家 MerID</li>
 * </ol>
 * 關著時直接 return，<b>不是</b>先 claim 再跳過：後者會把每一列的租約無謂地壓住 600 秒，
 * 讓「gate 打開的那一刻」之後還要再等一輪租約。
 */
@Component
@ConditionalOnProperty(prefix = "hawkr.payment.logistics.collection",
        name = "enabled", havingValue = "true", matchIfMissing = true)
public class LogisticsCollectionScheduler {

    private static final ObsLogger log = ObsLogger.forClass(LogisticsCollectionScheduler.class);

    /**
     * 單筆最壞耗時（秒），見類別 javadoc 的算式。
     *
     * <p>🔴 這個常數是租約下界的<b>輸入</b>：改它就是在改不變式，必須同步覆核
     * {@code PayUniHttpClient} 的三個 timeout。
     */
    static final int WORST_CASE_ITEM_SECONDS = 100;

    /** 租約上界（秒）：一個崩掉的 pod 最多把一列擋住多久。 */
    static final int MAX_LEASE_SECONDS = 3600;

    /** metric 名。Prometheus 對 Counter 自動補 {@code _total}。 */
    static final String METRIC_OUTCOME = "hawkr_payment_logistics_collection_outcome";

    /** 🔴 兩道 gate 是否都開著（1／0）。worker「完全沒在跑」靠它才看得見，log 不會主動說。 */
    static final String METRIC_GATE_OPEN = "hawkr_payment_logistics_collection_gate_open";

    private final LogisticsReceivableRepository receivables;
    private final LogisticsCollectionAttemptRepository attempts;
    private final LogisticsCollectionService collection;
    private final Optional<LogisticsCollectionService.SellerMerchantIdResolver> merchantIds;
    private final MeterRegistry meterRegistry;

    private final int batchSize;
    private final int submitLeaseSeconds;
    private final int queryLeaseSeconds;
    private final long perItemSleepMs;

    public LogisticsCollectionScheduler(
            LogisticsReceivableRepository receivables,
            LogisticsCollectionAttemptRepository attempts,
            LogisticsCollectionService collection,
            Optional<LogisticsCollectionService.SellerMerchantIdResolver> merchantIds,
            MeterRegistry meterRegistry,
            @Value("${hawkr.payment.logistics.collection.batch-size:5}") int batchSize,
            @Value("${hawkr.payment.logistics.collection.submit-lease-seconds:600}") int submitLeaseSeconds,
            @Value("${hawkr.payment.logistics.collection.query-lease-seconds:600}") int queryLeaseSeconds,
            @Value("${hawkr.payment.logistics.collection.fund-date-grace-hours:24}") int fundDateGraceHours,
            @Value("${hawkr.payment.logistics.collection.per-item-sleep-ms:100}") long perItemSleepMs) {
        validateLeaseBounds(batchSize, submitLeaseSeconds, queryLeaseSeconds, fundDateGraceHours);
        this.receivables = receivables;
        this.attempts = attempts;
        this.collection = collection;
        this.merchantIds = merchantIds;
        this.meterRegistry = meterRegistry;
        this.batchSize = batchSize;
        this.submitLeaseSeconds = submitLeaseSeconds;
        this.queryLeaseSeconds = queryLeaseSeconds;
        this.perItemSleepMs = perItemSleepMs;
        Gauge.builder(METRIC_GATE_OPEN, this, s -> s.gatesOpen() ? 1.0d : 0.0d)
                .description("線 B 自動收款的兩道 Enablement Gate 是否都開著（費率已查證 ＋ MerID resolver 已接線）")
                .register(meterRegistry);
    }

    /**
     * 🔴 §29 的不變式做成開機即拋，而不是註解裡的承諾。
     *
     * <p>它擋的是最容易發生的一種漂移：為了吞吐量把 {@code batch-size} 從 5 調到 50，
     * 而租約沒動——那一刻起，批次跑到第 6 筆之後租約就過期了，另一個 pod 會<b>合法</b>搶走
     * 同一列並再打一次 PayUni。那個失效<b>沒有任何錯誤訊息</b>。
     */
    static void validateLeaseBounds(int batchSize, int submitLeaseSeconds,
                                    int queryLeaseSeconds, int fundDateGraceHours) {
        if (batchSize < 1) {
            throw new IllegalStateException("logistics.collection.batch-size 必須 >= 1，got " + batchSize);
        }
        long required = (long) batchSize * WORST_CASE_ITEM_SECONDS;
        requireLowerBound("submit-lease-seconds", submitLeaseSeconds, required, batchSize);
        requireLowerBound("query-lease-seconds", queryLeaseSeconds, required, batchSize);
        requireUpperBound("submit-lease-seconds", submitLeaseSeconds);
        requireUpperBound("query-lease-seconds", queryLeaseSeconds);

        // 查詢側的第二個（更緊的）上界：租約若接近寬限期，崩掉的持有者會實質延後 UNCONFIRMED 判定。
        long graceSeconds = (long) fundDateGraceHours * 3600L;
        if (queryLeaseSeconds >= graceSeconds) {
            throw new IllegalStateException(String.format(
                    "logistics.collection.query-lease-seconds=%d 必須遠小於 fund-date-grace-hours=%d（%d 秒）"
                            + "：租約接近寬限期時，一個崩掉的持有者會實質延後 UNCONFIRMED 判定",
                    queryLeaseSeconds, fundDateGraceHours, graceSeconds));
        }
    }

    private static void requireLowerBound(String key, int leaseSeconds, long required, int batchSize) {
        if (leaseSeconds <= required) {
            throw new IllegalStateException(String.format(
                    "logistics.collection.%s=%d 不足：批次共用同一個租約起點（NOW() 是交易開始時間），"
                            + "不變式是 batchSize(%d) × 單筆最壞耗時(%ds) = %ds < 租約。"
                            + "調大 batch-size 就必須同步調大租約，否則另一個 pod 會合法搶走仍在打 PayUni 的那一列",
                    key, leaseSeconds, batchSize, WORST_CASE_ITEM_SECONDS, required));
        }
    }

    private static void requireUpperBound(String key, int leaseSeconds) {
        if (leaseSeconds > MAX_LEASE_SECONDS) {
            throw new IllegalStateException(String.format(
                    "logistics.collection.%s=%d 超過上界 %ds：崩掉的 pod 會把該列擋住整段租約",
                    key, leaseSeconds, MAX_LEASE_SECONDS));
        }
    }

    /** 兩道 Enablement Gate 是否都開著。 */
    boolean gatesOpen() {
        return collection.isAutoCollectionEnabled() && merchantIds.isPresent();
    }

    // =========================================================================
    // 提交 worker
    // =========================================================================

    @Scheduled(fixedDelayString = "${hawkr.payment.logistics.collection.submit.fixed-delay-ms:60000}")
    public void runSubmit() {
        if (!gatesOpen()) {
            // 🔴 設計上的停用，安靜跳過不告警——把它寫成告警會讓真正的失敗被埋在噪音裡。
            //    「worker 沒在跑」靠 METRIC_GATE_OPEN 這個 gauge 看，不靠 log。
            return;
        }
        List<LogisticsReceivableEntity> batch =
                receivables.claimPendingForSubmit(submitLeaseSeconds, batchSize);
        if (batch.isEmpty()) {
            return;
        }

        for (LogisticsReceivableEntity r : batch) {
            Optional<String> merId = merchantIds.get().resolve(r.getStoreId());
            if (merId.isEmpty()) {
                // 🔴 解析不出 MerID 就跳過，**不得退回任何預設值**：猜錯會從別人的商店扣錢，
                //    而每一層 log 都會顯示成功。不歸還租約 ⇒ 下一次重試自動退避一個租約。
                count("submit", "merchant_unresolved");
                log.warn("LogisticsCollectionScheduler.submit.merchantUnresolved",
                        "解析不出賣家 MerID，跳過", "storeId", String.valueOf(r.getStoreId()));
                continue;
            }
            try {
                LogisticsCollectionService.SubmitResult result = collection.submitOne(r, merId.get());
                count("submit", result.name().toLowerCase());
            } catch (Exception e) {
                // 🔴 不歸還租約：退避 = 一個租約長度。歸還會讓下一 tick 立刻對剛剛失敗的
                //    第三方接口再打一次。
                count("submit", "error");
                log.error("LogisticsCollectionScheduler.submit.failed",
                        "送出單筆拋例外 receivableId=" + r.getId(), e);
            }
            if (!sleepBetweenItems()) {
                break;
            }
        }
    }

    // =========================================================================
    // 查詢 worker
    // =========================================================================

    @Scheduled(fixedDelayString = "${hawkr.payment.logistics.collection.query.fixed-delay-ms:300000}")
    public void runQuery() {
        if (!gatesOpen()) {
            return;
        }
        List<LogisticsCollectionAttemptEntity> batch =
                attempts.claimForQuery(queryLeaseSeconds, batchSize);
        if (batch.isEmpty()) {
            return;
        }

        for (LogisticsCollectionAttemptEntity a : batch) {
            Optional<LogisticsReceivableEntity> receivable = receivables.findById(a.getReceivableId());
            if (receivable.isEmpty()) {
                // FK 是 ON DELETE RESTRICT ⇒ 這裡為空只可能是資料被手寫 SQL 動過。fail-loud 到 metric。
                count("query", "receivable_missing");
                log.error("LogisticsCollectionScheduler.query.receivableMissing",
                        "attempt 指向不存在的應收 attemptId=" + a.getId());
                continue;
            }
            LogisticsReceivableEntity r = receivable.get();
            Optional<String> merId = merchantIds.get().resolve(r.getStoreId());
            if (merId.isEmpty()) {
                count("query", "merchant_unresolved");
                log.warn("LogisticsCollectionScheduler.query.merchantUnresolved",
                        "解析不出賣家 MerID，跳過", "storeId", String.valueOf(r.getStoreId()));
                continue;
            }
            try {
                LogisticsCollectionService.QueryResult result = collection.queryOne(r, a, merId.get());
                count("query", result.name().toLowerCase());
            } catch (Exception e) {
                count("query", "error");
                log.error("LogisticsCollectionScheduler.query.failed",
                        "查詢單筆拋例外 attemptId=" + a.getId(), e);
            }
            if (!sleepBetweenItems()) {
                break;
            }
        }
    }

    // =========================================================================

    /**
     * 對第三方友善的間隔。
     *
     * @return {@code false} ＝ 執行緒被中斷，呼叫端應中止整批
     */
    private boolean sleepBetweenItems() {
        if (perItemSleepMs <= 0L) {
            return true;
        }
        try {
            Thread.sleep(perItemSleepMs);
            return true;
        } catch (InterruptedException ie) {
            Thread.currentThread().interrupt();
            return false;
        }
    }

    private void count(String phase, String outcome) {
        meterRegistry.counter(METRIC_OUTCOME, "phase", phase, "outcome", outcome).increment();
    }
}
