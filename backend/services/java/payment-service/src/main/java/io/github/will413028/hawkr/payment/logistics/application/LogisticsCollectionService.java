package io.github.will413028.hawkr.payment.logistics.application;

import io.github.will413028.hawkr.common.obs.log.ObsLogger;
import io.github.will413028.hawkr.common.utils.DateTimeUtils;
import io.github.will413028.hawkr.common.utils.TsidUtil;
import io.github.will413028.hawkr.common.utils.code.LogisticsReceivableStatus;
import io.github.will413028.hawkr.payment.agentfee.AgentFeeGrossUp;
import io.github.will413028.hawkr.payment.agentfee.AgentFeeService;
import io.github.will413028.hawkr.payment.logistics.domain.LogisticsAmounts;
import io.github.will413028.hawkr.payment.logistics.domain.ReceivableStatusRules;
import io.github.will413028.hawkr.payment.logistics.repository.LogisticsCollectionAttemptRepository;
import io.github.will413028.hawkr.payment.logistics.repository.LogisticsReceivableRepository;
import io.github.will413028.hawkr.payment.logistics.repository.entity.LogisticsCollectionAttemptEntity;
import io.github.will413028.hawkr.payment.logistics.repository.entity.LogisticsReceivableEntity;
import java.time.DayOfWeek;
import java.time.LocalDate;
import java.time.LocalDateTime;
import java.util.Optional;
import java.util.function.Supplier;
import org.springframework.beans.factory.annotation.Autowired;
import org.springframework.beans.factory.annotation.Value;
import org.springframework.stereotype.Service;
import org.springframework.transaction.annotation.Propagation;
import org.springframework.transaction.annotation.Transactional;

/**
 * 線 B（平台向賣家收物流服務費）的收款編排：<b>應收成立 → 送出 → 查詢 → 入帳</b>。
 *
 * <p>本類別是 2026-09-12 架構裁決 DEC-20260912-184025 之後，原 {@code settlement-service}
 * {@code internal/usecase/collection.go} 的 Java 對應物，落 {@code io.github.will413028.hawkr.payment.logistics}。
 *
 * <h2>🔴 本層不知道費率（ADR-011）</h2>
 * domain（{@link LogisticsAmounts}）只算「要淨收多少」（{@code target_net}）；
 * 送出多少由 {@link AgentFeeGrossUp} 依 verified fee policy 反推。這條界線是 ADR-005
 * 「不得把 1% 寫成商業常數」的實際形狀。本類別因此<b>只</b>把 target_net 交給 gross-up，
 * 自己不做任何費率算術。
 *
 * <h2>🔴 三條不可違反的規則（裁決第 7、12 條）</h2>
 * <ol>
 *   <li><b>gross 在送出 PayUni 之前就與 attempt 一起持久化</b>。
 *       {@code agent_fee_query} 的查詢鍵是 {@code (MerID, TradeNo, Amt)} <b>三元組</b>，
 *       而 {@code Amt} 是 gross 不是 target net ⇒ 「送出成功、回應遺失」時，
 *       若 gross 沒先落地，後續 query 帶不出正確 Amt ⇒ 一筆已經在對方系統裡的收款，
 *       在我方永遠停在不確定狀態。
 *       ⚠️ 順序是 <b>先 {@code solve()} 再建列</b>：{@code solve()} 在費率未查證時會拋
 *       （fail-closed），先建列的話會留下一筆 gross 不明的 attempt，而
 *       {@code chk_logcoll_gross >= 1} 根本寫不進去。
 *       ⚠️ 也<b>不用</b>「先落佔位值、回應後覆寫」那種兩段式——gross 在 HTTP 呼叫之前就算得出來，
 *       同進程直接拿得到，可以一次寫對。</li>
 *   <li><b>不自己 call 自己的 gRPC</b>：直接注入 {@link AgentFeeService}（同進程 {@code @Service}），
 *       不透過 {@code PaymentGrpcService}。</li>
 *   <li><b>hot path 隔離</b>：本模組<b>不得</b>被 Buyer payment 的同步路徑引用。本類別的入口只有
 *       送達事件 consumer（{@link #createReceivable}）與排程 worker
 *       （{@link #submitOne} / {@link #queryOne}）。</li>
 * </ol>
 *
 * <h2>🔴 交易邊界：送出／查詢<b>不得</b>包在交易內</h2>
 * {@link #submitOne} 與 {@link #queryOne} 標 {@link Propagation#NEVER}。
 * 這不是風格，是正確性：
 * <ul>
 *   <li>attempt 的 pre-commit <b>必須真的 commit</b> 才呼叫 PayUni。被外層交易包住的話，
 *       那一列直到 PayUni 回來（或逾時）都還沒 commit ⇒ 「先落地才呼叫」整條保證失效，
 *       而且<b>完全沒有訊號</b>。</li>
 *   <li>PayUni 的同步呼叫最壞 90 秒（OkHttp connect/write/read 各 30 秒），
 *       包在交易裡就是一條 90 秒的長交易持著連線。</li>
 * </ul>
 * {@code NEVER} 讓「呼叫端不小心包了交易」從靜默失效變成開機即現形的例外
 * （fail-closed，不是 fail-open）。
 */
@Service
public class LogisticsCollectionService {

    private static final ObsLogger log = ObsLogger.forClass(LogisticsCollectionService.class);

    /**
     * PayUni {@code agent_fee} 的費用類別碼：物流 ＝ 4。
     *
     * <p>⚠️ 這是 <b>PayUni 協定欄位</b>不是本平台的共享業務碼 —— 它沒有、也不該有 CodeMeta 條目
     * （CodeMeta 是跨服務共享代碼值的 SSOT，第三方協定數字放進去會讓兩個值域混在一起）。
     * gRPC adapter {@code PaymentGrpcService.submitAgentFee} 同樣是把呼叫端給的數字直接透傳。
     */
    static final int FEE_TYPE_LOGISTICS = 4;

    /** PayUni {@code agent_fee} 的帳務方向：1 ＝ 正向（向賣家收）。2 ＝ 負向（退還賣家）走貸記路徑。 */
    static final int BAL_TYPE_COLLECT = 1;

    /**
     * {@code trade_no} 的前綴。
     *
     * <p>🔴 <b>我方生成並保證唯一</b>（{@code uq_logcoll_trade_no}）：PayUni 官方三處逐字標
     * {@code TradeNo}「可重複」，且 {@code agent_fee_query} 回<b>單筆結構非陣列</b>
     * ⇒ 真的送重複了，用官方 API 也對不出有幾筆。
     */
    static final String TRADE_NO_PREFIX = "LF";

    /** PayUni {@code TradeNo} 長度上限。超長會被截斷或拒絕，而截斷會讓去重鍵失真。 */
    static final int TRADE_NO_MAX_LENGTH = 20;

    private final LogisticsReceivableRepository receivables;
    private final LogisticsCollectionAttemptRepository attempts;
    private final LogisticsCollectionTxService txService;
    private final AgentFeeService agentFeeService;
    private final AgentFeeGrossUp grossUp;
    private final Supplier<Long> ids;
    private final Supplier<LocalDateTime> nowUtc;

    /**
     * 🔴 自動收款總開關，<b>預設 false，而那是設計不是保守起見</b>。
     *
     * <p>ADR-011 的 gross-up 需要 verified fee policy 當輸入（EV-P6）；缺了它，
     * 送出的每一筆金額都是猜的。所以「停用」是 <b>Enablement Gate</b> 而不是故障
     * ——schema／狀態機／對帳／residual 機制全部可以先做完，缺的只是這個開關。
     *
     * <p>⚠️ 呼叫端（worker）遇到停用應該<b>安靜跳過</b>而不是告警：把設計上的停用寫成告警，
     * 會讓真正的失敗被埋在噪音裡。
     */
    private final boolean autoCollectionEnabled;

    /**
     * {@code expected_fund_date} 之後還要等多久，才把一筆查不到撥款的收款判成 UNCONFIRMED。
     *
     * <p>🔴 這個寬限期同時吸收兩件事：① PayUni 的次日撥款本來就有時間差；
     * ② {@link #nextBusinessDay} 只跳過週末、<b>沒有國定假日表</b>，連假時期望日會偏早。
     */
    private final int fundDateGraceHours;

    @Autowired
    public LogisticsCollectionService(
            LogisticsReceivableRepository receivables,
            LogisticsCollectionAttemptRepository attempts,
            LogisticsCollectionTxService txService,
            AgentFeeService agentFeeService,
            AgentFeeGrossUp grossUp,
            @Value("${hawkr.payment.logistics.collection.auto-submit-enabled:false}")
            boolean autoCollectionEnabled,
            @Value("${hawkr.payment.logistics.collection.fund-date-grace-hours:24}")
            int fundDateGraceHours) {
        this(receivables, attempts, txService, agentFeeService, grossUp,
                autoCollectionEnabled, fundDateGraceHours, TsidUtil::generate, DateTimeUtils::nowUtc);
    }

    /** 測試用：注入 id 產生器與時鐘（10 檔 §21：時間敏感邏輯注入 clock，不用 sleep 或本機時區）。 */
    public LogisticsCollectionService(
            LogisticsReceivableRepository receivables,
            LogisticsCollectionAttemptRepository attempts,
            LogisticsCollectionTxService txService,
            AgentFeeService agentFeeService,
            AgentFeeGrossUp grossUp,
            boolean autoCollectionEnabled,
            int fundDateGraceHours,
            Supplier<Long> ids,
            Supplier<LocalDateTime> nowUtc) {
        this.receivables = receivables;
        this.attempts = attempts;
        this.txService = txService;
        this.agentFeeService = agentFeeService;
        this.grossUp = grossUp;
        this.autoCollectionEnabled = autoCollectionEnabled;
        this.fundDateGraceHours = fundDateGraceHours;
        this.ids = ids;
        this.nowUtc = nowUtc;
    }

    public boolean isAutoCollectionEnabled() {
        return autoCollectionEnabled;
    }

    // =========================================================================
    // 1. 應收成立
    // =========================================================================

    /**
     * provider 成本事實（{@code logistics.cost.incurred} 的領域輸入，契約 ＝ 70 §7.1）。
     *
     * <h3>🔴 觸發不是「送達完成」</h3>
     * 條件是 {@code provider cost incurred} <b>AND</b> {@code settlement counterparty = SELLER}
     * （70 §3.1.1）——涵蓋<b>四種</b>情形：正常送達／賣家可歸責的取消／買家可歸責的無法投遞／
     * 返程 leg。
     *
     * <p>⚠️ <b>本段原本寫「應收只在送達完成成立 ⇒ 取消／建單失敗／退回一律不建應收，
     * 那條錢走 {@code logistics_adjustments}（無 receivable 母單）」——該說法已被 70 §6.3 取代</b>，
     * 兩個理由各自獨立成立：
     * <ol>
     *   <li>觸發條件本來就不是送達：provider 成本已經發生、結算對手是賣家 ⇒ 該建就要建。</li>
     *   <li>🔴 {@code logistics_adjustments} <b>不得當 collection mother record</b>（70 §3.5.1）
     *       ——而「掛在 {@code store_id ＋ provider_delivery_ref} 上、無母單」正是「當母單」的定義。
     *       DB 層也不允許：{@code logistics_collection_attempts.receivable_id} 是 {@code NOT NULL}
     *       ＋ FK ⇒ 那條路徑<b>永遠收不到錢</b>，只會產生一張沒有人去收的調整單。</li>
     * </ol>
     *
     * <h3>🔴 本 record 不帶 responsibility cause</h3>
     * Cost Fact ≠ Responsibility Decision（70 §3.1.4）：責任終局與 settlement counterparty
     * 的唯一 authority 是 N16。成本先到時 {@code responsibility_cause} 留空，
     * 由 durable join 在責任定案後補上。
     *
     * @param shipmentId 🔴 <b>可為 {@code null}</b>——取消可能發生在 shipment 建立之前，
     *                   這個事件真的可能沒有 shipment（70 §7.1）。
     *                   ⚠️ 正因為它可以是 null，<b>它不能當冪等鍵</b>。
     * @param occurredAt 事件的<b>業務時間</b>，用作 {@code charge_basis_frozen_at}。
     *                   🔴 不是 {@code now()}：計費依據凍結在事實發生的那一刻，之後政策改了不回溯本列
     */
    public record CostIncurred(long storeId,
                               String orderNo,
                               Long shipmentId,
                               String provider,
                               String providerDeliveryRef,
                               long estimatedCost,
                               long platformSubsidy,
                               LocalDateTime occurredAt) {

        /**
         * {@code logistics_receivable:{provider}:{provider_delivery_ref}}。
         *
         * <p>🔴 <b>錨在 provider fulfillment leg，不是 shipment</b>（70 §3.1.2）。
         * 它的 UNIQUE（{@code uq_logrecv_idem}）是「事件重放不會重複計費」的唯一保證
         * ——MQ 至少一次投遞，沒有它就會對同一條 leg 建兩筆應收。
         *
         * <p>⚠️ <b>本鍵曾經是 {@code logistics_receivable:{shipment_id}:{leg}}，那是錯的</b>，
         * 兩個方向都會出事且症狀相反：
         * <ul>
         *   <li>provider 建單結果不明後重建、或人工 cancel + recreate ⇒ 同一個 {@code shipment_id}
         *       先後對應兩個 delivery ⇒ 兩套 identity 分岔，而 UNIQUE <b>抓不到</b> ⇒ <b>重複計費</b></li>
         *   <li>反過來，正逆向兩條 leg 的 {@code shipment_id} 相同 ⇒ 第二筆真實成本被誤判成重複
         *       而擋下 ⇒ <b>漏收</b></li>
         * </ul>
         *
         * <p>🔴 <b>本格式由 DB 強制</b>：{@code chk_logrecv_idem_anchor}（V15）逐字要求
         * {@code provider_delivery_ref} 非空時 {@code idempotency_key} 必須等於本組成。
         * 用別的格式寫入會在 INSERT 當下 {@code 23514}——那是刻意的，讓錯誤的錨立刻現形，
         * 而不是等到對帳時才發現金額對不上。
         */
        public String idempotencyKey() {
            return "logistics_receivable:" + provider + ":" + providerDeliveryRef;
        }
    }

    /**
     * 依成本事實建立應收，並在<b>同一個交易</b>內認列帳本與更新賣家餘額。
     *
     * @return {@code true} ＝ 這次真的建了；{@code false} ＝ 這則事件已經處理過（冪等鍵衝突）。
     *         🔴 {@code false} <b>不是錯誤</b>，MQ 呼叫端應該 ACK 而不是進 DLQ。
     * @throws IllegalArgumentException 金額不合法（fail-loud，不靜默夾到 0）
     */
    public boolean createReceivable(CostIncurred e) {
        // 🔴 純算術先做、在交易之外：它會拋（補貼超過成本之類），而那種錯誤不該
        //    先開一個交易再 rollback。domain 只算「本期認列」，不含 carried（重複認列收入）。
        long accrual = LogisticsAmounts.accrualAmount(e.estimatedCost(), e.platformSubsidy(), 0L);

        LogisticsCollectionTxService.CreateOutcome outcome =
                txService.createReceivableInTx(ids.get(), e, accrual);

        if (!outcome.created()) {
            log.info("LogisticsCollectionService.receivable.replay",
                    "送達事件重放，已存在應收（ACK 不進 DLQ）", "idempotencyKey", e.idempotencyKey());
            return false;
        }
        log.info("LogisticsCollectionService.receivable.created",
                String.format("應收成立 accrual=%d carried=%d targetNet=%d",
                        accrual, outcome.carriedResidual(), outcome.targetNet()),
                "receivableId", String.valueOf(outcome.receivableId()));
        return true;
    }

    // =========================================================================
    // 2. 送出
    // =========================================================================

    /** {@link #submitOne} 的結果。每一格的<b>後續處置都不同</b>，所以不合併成布林。 */
    public enum SubmitResult {
        /** 自動收款未啟用（EV-P6）。🔴 設計上的停用，呼叫端安靜跳過、<b>不要告警</b>。 */
        DISABLED,
        /** PayUni 明確受理 ⇒ 應收已進 SUBMITTED，等次日。 */
        SUBMITTED,
        /**
         * PayUni <b>明確拒絕</b>（有回應、非 SUCCESS）⇒ 沒有受理、不存在「可能已經扣了」的窗口
         * ⇒ 可自動退避重送。應收已進 FAILED。
         */
        REJECTED,
        /**
         * 🔴 <b>未取得回應</b>（逾時／連線失敗）⇒ PayUni <b>可能已經受理</b>。
         * 不重送、推進 SUBMITTED，交給查詢 worker 用這個 {@code trade_no} 去問對方到底有沒有受理。
         */
        UNKNOWN,
        /**
         * {@code target_net < 1} ⇒ 平台上期多收超過本期應收。
         * 🔴 該產生貸記，<b>不是</b>送出一筆必失敗的請求（PayUni {@code Amt} 值域 1~9999999）。
         */
        SKIPPED_ZERO_TARGET
    }

    /**
     * 對一筆 PENDING 應收送出 {@code agent_fee}。
     *
     * <p>🔴 <b>順序是硬性的</b>：{@code solve()} → pre-commit（含 gross）→ 才呼叫 PayUni。
     * 顛倒的後果：HTTP 逾時 ⇒ 我方沒有紀錄（或有紀錄但不知道送了多少）
     * ⇒ 重試產生第二個 {@code trade_no} ⇒ <b>重複扣款且事後查不出來</b>。
     *
     * @param merId 被收款的合作商店 MerID。🔴 由呼叫端解析後傳入——本服務<b>不猜</b>它
     *              （賣家 MerID 的真實來源在 seller 側，見 {@link SellerMerchantIdResolver}）
     * @throws IllegalStateException 應收不在 PENDING（呼叫順序或狀態機出問題），或 CAS rows=0
     */
    @Transactional(propagation = Propagation.NEVER)
    public SubmitResult submitOne(LogisticsReceivableEntity r, String merId) {
        if (!autoCollectionEnabled) {
            return SubmitResult.DISABLED;
        }
        String status = r.getStatus();
        if (!LogisticsReceivableStatus.PENDING.value().equals(status)) {
            throw new IllegalStateException(
                    "logistics: 只有 PENDING 可以送出，receivable=" + r.getId() + " status=" + status);
        }
        long targetNet = r.getTargetNet() == null ? 0L : r.getTargetNet();
        if (targetNet < 1L) {
            // 🔴 不送出一筆必失敗的請求：那種失敗看起來像通道問題，會讓真正的通道問題被埋掉。
            log.warn("LogisticsCollectionService.submit.zeroTarget",
                    "target_net=" + targetNet + " 不得送出，應走貸記路徑",
                    "receivableId", String.valueOf(r.getId()));
            return SubmitResult.SKIPPED_ZERO_TARGET;
        }

        // ── 步驟 1：先 solve。費率未查證時它在這裡就拋，還沒有任何列落地。 ──────────
        AgentFeeGrossUp.Result solved = grossUp.solve(targetNet);

        long attemptId = ids.get();
        String tradeNo = TRADE_NO_PREFIX + attemptId;
        if (tradeNo.length() > TRADE_NO_MAX_LENGTH) {
            throw new IllegalStateException(
                    "logistics: trade_no " + tradeNo + " 超過 PayUni 長度上限 " + TRADE_NO_MAX_LENGTH);
        }

        // ── 步驟 2：pre-commit。🔴 gross 與 attempt **同一個 INSERT**、且在呼叫之前 commit。──
        //    本方法是 Propagation.NEVER ⇒ 這一句自成交易並立即 commit，
        //    「送出前已落地」因此是機械保證而不是註解裡的承諾。
        attempts.preCommit(attemptId, r.getId(), tradeNo, solved.grossAmount());

        // ── 步驟 3：才呼叫 PayUni。────────────────────────────────────────────────
        AgentFeeService.SubmitOutcome res =
                agentFeeService.submit(merId, tradeNo, solved, FEE_TYPE_LOGISTICS, BAL_TYPE_COLLECT);

        LocalDateTime now = nowUtc.get();
        if (!res.providerResponded()) {
            // 🔴 **未取得回應 ≠ 失敗**：PayUni 可能已經受理了。
            //    不重送——留在 PENDING 會讓下一輪再送一次同一筆（新 trade_no），
            //    而那正是「把一次不確定變成一次確定的重複扣款」。
            //    ⇒ 推到 SUBMITTED，讓查詢 worker 拿這個 trade_no 去問對方。
            //    ⚠️ gross 在這一格是**真值不是佔位值**：它在 HTTP 呼叫之前就算好並落地了，
            //       所以即使「PayUni 逾時」這一種也查得到（查詢鍵三元組完整）。
            txService.markSubmittedInTx(r.getId(), attemptId, "", "呼叫未取得回應",
                    solved.grossAmount(), now, nextBusinessDay(now));
            log.warn("LogisticsCollectionService.submit.unknown",
                    "agent_fee 未取得回應，推進 SUBMITTED 交查詢 worker 判定", "tradeNo", tradeNo);
            return SubmitResult.UNKNOWN;
        }

        if (res.rejected()) {
            // 明確拒絕 ⇒ 沒有受理、不存在「可能已經扣了」的窗口 ⇒ 可自動退避重送。
            txService.markRejectedInTx(r.getId(), attemptId, res.providerStatus(),
                    res.providerMessage(), res.grossAmount(), now);
            log.warn("LogisticsCollectionService.submit.rejected",
                    "agent_fee 明確拒絕 status=" + res.providerStatus(), "tradeNo", tradeNo);
            return SubmitResult.REJECTED;
        }

        txService.markSubmittedInTx(r.getId(), attemptId, res.providerStatus(),
                res.providerMessage(), res.grossAmount(), now, nextBusinessDay(now));
        log.info("LogisticsCollectionService.submit.accepted",
                "agent_fee 已受理，等次日撥款", "tradeNo", tradeNo);
        return SubmitResult.SUBMITTED;
    }

    // =========================================================================
    // 3. 查詢與入帳
    // =========================================================================

    /** {@link #queryOne} 的結果。 */
    public enum QueryResult {
        /** 已確認入帳並完成帳務處理（attempt ＋ 應收狀態 ＋ 帳本 ＋ 餘額，同一交易）。 */
        SETTLED,
        /** 還在寬限期內、尚未撥款。什麼都不做，下一輪再查。 */
        PENDING_FUND,
        /** 🔴 超過期望撥款日 ＋ 寬限仍查不到 ⇒ 進 ADM 人工，<b>不自動重送</b>。 */
        UNCONFIRMED,
        /** 已撥款但 PayUni 沒給 Fee ⇒ 算不出實收。🔴 不得用預估值冒充，留在原狀下一輪再查。 */
        SETTLED_WITHOUT_FEE,
        /** 應收已不在可查詢的狀態（ADM 已核銷／已退回開新 attempt／已結清）⇒ 這一筆 attempt 不再追。 */
        SKIPPED_NOT_QUERYABLE
    }

    /**
     * 對一筆已送出的收款查詢撥款結果。
     *
     * <p>🔴 <b>兩個哨兵值必須分開判</b>：
     * <ul>
     *   <li>{@code found=false}（PayUni 沒有這筆）⇒ 送出其實沒受理，或 trade_no／amt 帶錯</li>
     *   <li>{@code found=true} 且 {@code fundDate} 為空（尚未撥款）⇒ 受理了但還沒執行</li>
     * </ul>
     * 混在一起的話，呼叫端會把「沒受理」誤讀成「還沒到次日」而一直等下去。
     *
     * <p>⚠️ <b>與 Go 版的一處刻意差異</b>：Go 版要求 {@code status == SUBMITTED}，否則回錯。
     * 這裡<b>也接受 UNCONFIRMED</b>——{@link ReceivableStatusRules} 的 {@code SETTLED}
     * allowed-from 逐字含 UNCONFIRMED，而它是 UNCONFIRMED 三條出邊裡最可能的一條。
     * 不接受它的話，一筆「其實收到了」的 UNCONFIRMED 只能靠人工發現。
     *
     * @param merId 🔴 與送出時同一個 MerID（查詢鍵是 {@code (MerID, TradeNo, Amt)} 三元組）
     */
    @Transactional(propagation = Propagation.NEVER)
    public QueryResult queryOne(LogisticsReceivableEntity r, LogisticsCollectionAttemptEntity a, String merId) {
        String status = r.getStatus();
        boolean queryable = LogisticsReceivableStatus.SUBMITTED.value().equals(status)
                || LogisticsReceivableStatus.UNCONFIRMED.value().equals(status);
        if (!queryable) {
            log.info("LogisticsCollectionService.query.skipped",
                    "應收已不在可查詢狀態 status=" + status, "tradeNo", a.getTradeNo());
            return QueryResult.SKIPPED_NOT_QUERYABLE;
        }

        // 🔴 帶的是 gross 不是 target_net：查詢鍵三元組的第三格就是當初送出的 Amt。
        AgentFeeService.QueryOutcome res =
                agentFeeService.query(merId, a.getTradeNo(), a.getGrossAmount());

        if (res.settled()) {
            return recordSettled(r, a, res);
        }

        // 尚未撥款（含查無此筆）。判「是不是已經等太久」。
        if (pastGrace(a)) {
            // 🔴 **UNCONFIRMED 不自動重送，這是整個狀態機最重要的一格。**
            //    理由是 PayUni 的實際能力：TradeNo 官方標「可重複」、查詢回單筆結構非陣列
            //    ⇒ 真的送重複了，用官方 API 也對不出有幾筆。
            //    自動重送的期望值是「把一次不確定變成一次確定的重複扣款」⇒ **必須由人看**。
            if (LogisticsReceivableStatus.UNCONFIRMED.value().equals(status)) {
                // 已經在 UNCONFIRMED 了：本輪只是再問一次有沒有收到，沒收到就維持原狀。
                return QueryResult.UNCONFIRMED;
            }
            requireTransition(r, LogisticsReceivableStatus.UNCONFIRMED);
            log.warn("LogisticsCollectionService.query.unconfirmed",
                    "超過期望撥款日＋寬限仍查不到撥款，進 ADM 人工（providerFound=" + res.found() + "）",
                    "tradeNo", a.getTradeNo());
            return QueryResult.UNCONFIRMED;
        }

        // 還在寬限期內：什麼都不做，下一輪再查。
        return QueryResult.PENDING_FUND;
    }

    /**
     * 把「確認入帳」寫進 attempt、應收狀態與帳本，<b>全部在同一個交易內</b>。
     *
     * <p>🔴 <b>residual ≠ 0 不得標記已收訖</b>：正的代表平台尚未收足（缺口併入下一期的
     * {@code target_net}），負的代表平台多收（必須產生 Seller Logistics Credit，
     * <b>不得</b>變成 platform margin）。兩者都不是 CLOSED——所以本方法推的是 SETTLED，
     * 而 SETTLED <b>不是終態</b>。
     */
    private QueryResult recordSettled(LogisticsReceivableEntity r,
                                      LogisticsCollectionAttemptEntity a,
                                      AgentFeeService.QueryOutcome res) {
        Optional<Long> actualFee = res.actualFee();
        if (actualFee.isEmpty()) {
            // 🔴 已撥款但沒有 Fee ⇒ 我們算不出實收。**不得用預估值冒充**
            //    （維度 5 的唯一來源是 agent_fee_query 的 Fee）。留在原狀讓下一輪再查。
            log.warn("LogisticsCollectionService.query.settledWithoutFee",
                    "已撥款但無 Fee，不得用預估值冒充", "tradeNo", a.getTradeNo());
            return QueryResult.SETTLED_WITHOUT_FEE;
        }

        long netProceeds = LogisticsAmounts.netProceeds(a.getGrossAmount(), actualFee.get());
        long residual = LogisticsAmounts.residual(r.getTargetNet(), netProceeds);
        LocalDate fundDate = LocalDate.parse(res.fundDate().orElseThrow());

        txService.recordSettledInTx(new LogisticsCollectionTxService.SettledCommand(
                r.getId(), r.getStoreId(), r.getOrderNo(), a.getId(), a.getTradeNo(),
                fundDate, actualFee.get(), netProceeds, residual));

        log.info("LogisticsCollectionService.query.settled",
                String.format("入帳完成 gross=%d actualFee=%d net=%d residual=%d",
                        a.getGrossAmount(), actualFee.get(), netProceeds, residual),
                "tradeNo", a.getTradeNo());
        return QueryResult.SETTLED;
    }

    // =========================================================================
    // 4. 純判定
    // =========================================================================

    /**
     * 是否已超過期望撥款日 ＋ 寬限期。
     *
     * <p>🔴 沒有這個判定，「還沒到次日」與「次日執行失敗」<b>永遠分不開</b>——兩者在 PayUni
     * 的回應上長得一模一樣（{@code FundDate = '0000-00-00'}）。
     *
     * <p>⚠️ {@code expected_fund_date} 為空時回 {@code false}（判不了就不推進，留給下一輪或人工）。
     */
    boolean pastGrace(LogisticsCollectionAttemptEntity a) {
        LocalDate expected = a.getExpectedFundDate();
        if (expected == null) {
            return false;
        }
        return nowUtc.get().isAfter(expected.atStartOfDay().plusHours(fundDateGraceHours));
    }

    /**
     * {@code expected_fund_date} ＝ 送出日 ＋ 1 個工作日。
     *
     * <p>🔴 它存在的唯一理由是 PayUni 分不出「還沒到次日」與「次日執行失敗」。沒有這個錨，
     * 那兩件事永遠分不開。
     * ⚠️ 這裡只跳過週末，<b>沒有國定假日表</b>：那會讓寬限期在連假時偏短，
     * 由 {@code fund-date-grace-hours} 吸收。
     */
    static LocalDate nextBusinessDay(LocalDateTime from) {
        LocalDate d = from.toLocalDate().plusDays(1);
        while (d.getDayOfWeek() == DayOfWeek.SATURDAY || d.getDayOfWeek() == DayOfWeek.SUNDAY) {
            d = d.plusDays(1);
        }
        return d;
    }

    /**
     * CAS 推進狀態並 fail-loud。
     *
     * <p>🔴 rows=0 <b>不得</b>讀成「別人先做過了」就靜默放行：它代表這筆已經不在
     * allowed-from 裡，而那是狀態機或呼叫順序有問題。靜默放行的症狀是
     * 「應收永遠停在某一格，而所有 log 都顯示處理成功」。
     */
    private void requireTransition(LogisticsReceivableEntity r, LogisticsReceivableStatus to) {
        if (!receivables.transition(r.getId(), to)) {
            throw new IllegalStateException(String.format(
                    "logistics: receivable %d 無法從 %s 推進到 %s（allowed-from=%s 不含它）",
                    r.getId(), r.getStatus(), to.value(),
                    ReceivableStatusRules.allowedFromValues(to)));
        }
    }

    /**
     * 把 {@code store_id} 解析成該賣家的 PayUni 子商店 MerID。
     *
     * <h2>🔴 為什麼是一個介面而不是 {@code "HAWK" + storeId}</h2>
     * 賣家 MerID 的<b>真實來源在 seller 側</b>（結帳鏈走的是
     * {@code CheckoutService.resolvePaymentMerId → sellerMeta.getPayuniMerId()}）。
     * 在 payment-service 裡按格式拼一份出來，就是第二個真實來源——而它們一旦不一致，
     * 錢會從錯的商店被扣走，<b>而且每一層 log 都顯示成功</b>。
     *
     * <h2>⚠️ 本波沒有實作（誠實標註）</h2>
     * payment-service 今天<b>沒有</b> seller 的 gRPC client（{@code adapter/grpc/client}
     * 只有 {@code OrderGrpcClient}），接上它需要 NetworkPolicy ＋ env 一起改（10 檔 §14.5.0c）
     * ⇒ 不在本波範圍。因此這是<b>第二道 Enablement Gate</b>，與費率政策（EV-P6）並列：
     * 沒有 bean ⇒ {@code LogisticsCollectionScheduler} 的送出／查詢 tick 直接跳過並記一次 log，
     * <b>不是</b>猜一個 MerID 送出去。
     */
    @FunctionalInterface
    public interface SellerMerchantIdResolver {

        /**
         * @return 該賣家的 PayUni 子商店 MerID；{@code Optional.empty()} ＝ 解析不出來。
         *         🔴 呼叫端遇到 empty 必須<b>跳過這一筆</b>，不得退回任何預設值
         */
        Optional<String> resolve(long storeId);
    }
}
