package io.github.will413028.hawkr.payment.logistics.application;

import io.github.will413028.hawkr.common.obs.log.ObsLogger;
import io.github.will413028.hawkr.common.utils.DateTimeUtils;
import io.github.will413028.hawkr.common.utils.TsidUtil;
import io.github.will413028.hawkr.common.utils.code.LogisticsReceivableStatus;
import io.github.will413028.hawkr.common.utils.code.ResponsibilityCause;
import io.github.will413028.hawkr.common.utils.code.ShippingMethod;
import io.github.will413028.hawkr.payment.logistics.domain.Bearer;
import io.github.will413028.hawkr.payment.logistics.domain.LedgerTxn;
import io.github.will413028.hawkr.payment.logistics.domain.LogisticsAmounts;
import io.github.will413028.hawkr.payment.logistics.domain.LogisticsLedgerBuilders;
import io.github.will413028.hawkr.payment.logistics.domain.ReceivableStatusRules;
import io.github.will413028.hawkr.payment.logistics.domain.ResponsibilityRules;
import io.github.will413028.hawkr.payment.logistics.repository.FulfillmentExceptionCaseRepository;
import io.github.will413028.hawkr.payment.logistics.repository.LedgerRepository;
import io.github.will413028.hawkr.payment.logistics.repository.LogisticsAdjustmentRepository;
import io.github.will413028.hawkr.payment.logistics.repository.LogisticsReceivableRepository;
import io.github.will413028.hawkr.payment.logistics.repository.ProviderSettlementLineRepository;
import io.github.will413028.hawkr.payment.logistics.repository.SellerLogisticsAccountRepository;
import io.github.will413028.hawkr.payment.logistics.repository.entity.FulfillmentExceptionCaseEntity;
import io.github.will413028.hawkr.payment.logistics.repository.entity.LogisticsAdjustmentEntity;
import io.github.will413028.hawkr.payment.logistics.repository.entity.LogisticsReceivableEntity;
import java.time.LocalDateTime;
import java.util.List;
import java.util.function.Supplier;
import org.springframework.beans.factory.annotation.Autowired;
import org.springframework.stereotype.Service;
import org.springframework.transaction.annotation.Transactional;

/**
 * 線 C → 線 B 的 true-up 編排（W4，70 §6.2）：<b>暫估 provider 成本 → 帳單實際成本</b>的差額結算。
 *
 * <h2>🔴 兩個方向都要有分錄，不得只記一邊</h2>
 * <table border="1">
 *   <caption>W4-1／W4-2 的程式形式</caption>
 *   <tr><th>情形</th><th>賣家軸</th><th>帳本</th></tr>
 *   <tr><td>{@code actual > estimated}</td>
 *       <td>{@code logistics_adjustments{kind=RECONCILIATION, amount=+diff}} ⇒ 併入下一筆應收</td>
 *       <td>{@link LogisticsLedgerBuilders#logisticsTrueUp} （DEBIT 應收／CREDIT 收入）</td></tr>
 *   <tr><td>{@code actual < estimated}</td>
 *       <td>🔴 <b>Seller Credit，不得吃掉</b>（{@code credit_balance += |diff|}）</td>
 *       <td>{@link LogisticsLedgerBuilders#logisticsCreditIssue}（DEBIT 收入／CREDIT 貸記）</td></tr>
 * </table>
 *
 * <h2>🔴 為什麼「多收」不走 {@code logisticsTrueUp} 的負向分支（那一支在本路徑<b>刻意沒有呼叫端</b>）</h2>
 * true-up 的合法起點只有 {@code SETTLED}（{@link ReceivableStatusRules} 的
 * {@code CLOSED} allowed-from ＝ {@code {SETTLED}}）⇒ <b>錢已經收進來了</b>，該筆應收餘額已歸零。
 * 此時再記一筆 {@code DEBIT 收入／CREDIT 應收} 會把應收<b>推成負數</b>，而
 * {@code ADR-008} 第 1 條逐字要求「應收與貸記是<b>兩個科目</b>，不是一個科目的正負」
 * —— 負的應收就是把兩者合成淨額。若兩筆都記（負向 true-up ＋ 貸記發生），
 * 同一筆多收會被沖銷<b>兩次</b>：收入少認列 2×diff，而賣家在「下期 target_net 變小」
 * 與「貸記餘額」兩處各拿回一次。
 * <p>⚠️ 這是「兩條規則各自正確、組合起來互相抵銷」的形狀（{@code 02-judgment.md} §2.1）：
 * §3.8 的科目對照表（含負向 true-up）與 §6.2 的對帳處置（貸記）各自都對，
 * <b>同時執行</b>才是錯的。本路徑採 §6.2，因為它才是對帳這條軸的規格。
 * 負向 {@code logisticsTrueUp} 留給「帳單早於收款抵達」的未來路徑（今天沒有執行者）。
 *
 * <h2>🔴 責任歸屬用 {@link ResponsibilityRules#bearerOf}，不在這裡重寫一份</h2>
 * 四格中最容易讀反的是 {@code BUYER_CAUSED} —— 它<b>仍歸賣家</b>的物流成本
 * （買家已完成一次付款，平台不得二次收買家，{@code ADR-004}）。
 * {@code PROVIDER_CAUSED} ⇒ {@link Bearer#DISPUTE} ⇒ 🔴 <b>不得自動向賣家收</b>，
 * 走爭議／對帳，並且<b>不回填 {@code actual_cost}</b>（見 {@link #reconcileOne}）。
 *
 * <h2>交易邊界</h2>
 * 本類別的公開方法都是 {@code @Transactional} 的<b>單筆</b>原子單位；批次迴圈住在
 * {@link ProviderStatementImporter}（<b>另一個 bean</b>）。理由是 10 檔 §11.1：
 * 在同一個 class 內以 {@code this.} 呼叫自家 {@code @Transactional} 方法不經 Spring proxy，
 * 事務<b>完全不存在</b>，各語句 auto-commit；而本方法一次要寫五處
 * （應收回填／帳本／調整單／餘額／狀態），少任何一處落地都會讓帳對不起來。
 */
@Service
public class LogisticsReconciliationService {

    private static final ObsLogger log = ObsLogger.forClass(LogisticsReconciliationService.class);

    // ── provider_settlement_lines.recon_status ────────────────────────────────
    // 🔴 誠實標註：這四個值住在 V9 的 chk_provsettle_recon，**沒有** CodeMeta enum
    //    （實查 common-core/utils/code 對 UNMATCHED／DISPUTED 零命中）。
    //    在這裡定具名常數，是為了讓字面值只出現一次；**不是**在 CodeMeta 之外另造碼表——
    //    真要收斂進 CodeMeta 得改 docs/代碼管理（CodeMeta）/*.md ＋ 跑 generate.sh ＋ 同步四份
    //    前端拷貝，那是跨 repo 的動作，不在本波（W4）的檔案範圍內。
    static final String RECON_UNMATCHED = "UNMATCHED";
    static final String RECON_MATCHED = "MATCHED";
    static final String RECON_DISPUTED = "DISPUTED";

    /**
     * {@code logistics_adjustments.kind}。
     *
     * <p>🔴 true-up 產生的調整單一律 {@code RECONCILIATION}（70 §6.2 逐字），
     * <b>即使</b>它帶著 {@code responsibility_cause}：{@code RESPONSIBILITY} 留給
     * §6.3 那條「未送達、無應收母單的取消費」。兩者的載體不同是刻意的。
     */
    static final String ADJ_KIND_RECONCILIATION = "RECONCILIATION";

    /**
     * {@code UNMATCHED → MATCHED／DISPUTED} 的 allowed-from。
     *
     * <p>⚠️ {@code provider_settlement_lines} 今天<b>沒有</b> {@code *Rules} helper
     * （{@link ReceivableStatusRules} 只管應收那一條軸），所以 allowed-from 由呼叫端傳入
     * （見 {@code ProviderSettlementLineMapper} 的同一段說明）。這是缺口不是設計。
     */
    private static final List<String> RECON_ALLOWED_FROM_UNMATCHED = List.of(RECON_UNMATCHED);

    // ── fulfillment_exception_cases ──────────────────────────────────────────
    // 🔴 同上：category 的 enum 本波尚未落進 CodeMeta，V9 因此**刻意沒有** chk_exc_category
    //    （「在 DB 硬寫一份值域清單 ＝ 在 SSOT 之外造第二份碼表」）。
    static final String CASE_SOURCE_SERVICE = "payment";
    static final String CASE_SUBJECT_TYPE = "SHIPMENT";
    static final String CASE_STATUS_OPEN = "OPEN";
    static final String CASE_SEVERITY_P1 = "P1";
    /** 帳單列對不到唯一應收（0 筆或多筆）。 */
    static final String CASE_CATEGORY_STATEMENT_UNMATCHED = "LOGISTICS_STATEMENT_UNMATCHED";
    /** {@code PROVIDER_CAUSED} 的超額成本 ⇒ 先進爭議，🔴 不得自動向賣家收。 */
    static final String CASE_CATEGORY_PROVIDER_DISPUTE = "LOGISTICS_PROVIDER_COST_DISPUTE";

    private final LogisticsReceivableRepository receivables;
    private final ProviderSettlementLineRepository lines;
    private final LogisticsAdjustmentRepository adjustments;
    private final SellerLogisticsAccountRepository accounts;
    private final LedgerRepository ledger;
    private final FulfillmentExceptionCaseRepository cases;
    private final Supplier<Long> ids;
    private final Supplier<LocalDateTime> nowUtc;

    @Autowired
    public LogisticsReconciliationService(LogisticsReceivableRepository receivables,
                                          ProviderSettlementLineRepository lines,
                                          LogisticsAdjustmentRepository adjustments,
                                          SellerLogisticsAccountRepository accounts,
                                          LedgerRepository ledger,
                                          FulfillmentExceptionCaseRepository cases) {
        this(receivables, lines, adjustments, accounts, ledger, cases,
                TsidUtil::generate, DateTimeUtils::nowUtc);
    }

    /** 測試用：注入 id 產生器與時鐘（10 檔 §21）。 */
    public LogisticsReconciliationService(LogisticsReceivableRepository receivables,
                                          ProviderSettlementLineRepository lines,
                                          LogisticsAdjustmentRepository adjustments,
                                          SellerLogisticsAccountRepository accounts,
                                          LedgerRepository ledger,
                                          FulfillmentExceptionCaseRepository cases,
                                          Supplier<Long> ids,
                                          Supplier<LocalDateTime> nowUtc) {
        this.receivables = receivables;
        this.lines = lines;
        this.adjustments = adjustments;
        this.accounts = accounts;
        this.ledger = ledger;
        this.cases = cases;
        this.ids = ids;
        this.nowUtc = nowUtc;
    }

    // =========================================================================
    // 1. 純判定（W4-1 / W4-2 / W4-3 / W4-4）
    // =========================================================================

    /**
     * true-up 的輸入。
     *
     * <p>🔴 <b>{@code platformSubsidy} 與 {@code sellerSubsidy} 是兩個欄位，永遠不合成一個數
     * （W4-4）</b>。它們的語意完全不同：
     * <ul>
     *   <li>{@code platformSubsidy} —— 平台自己吸收多少，<b>會</b>減少賣家要補付的金額</li>
     *   <li>{@code sellerSubsidy} —— 賣家對<b>買家</b>的運費補貼（線 A），
     *       🔴 <b>不得</b>減少賣家欠平台的物流成本。賣家替買家出運費，並不會讓
     *       provider 少收平台一毛錢</li>
     * </ul>
     * 🔴 <b>{@code Buyer Shipping Charge = 0} 不代表 Platform 負擔</b>（W4-4 逐字）：
     * 本 record <b>刻意沒有</b> {@code buyerShippingCharge} 欄位 —— 沒有那個輸入，
     * 「從買家運費 0 推導平台補貼」在結構上就不可能發生。
     *
     * @param cause {@code null} ＝ 沒有責任判定（單純的暫估誤差），依 W4-1 由賣家負擔
     */
    public record TrueUpInput(long estimatedCost,
                              long actualCost,
                              ResponsibilityCause cause,
                              long platformSubsidy,
                              long sellerSubsidy) {
    }

    /**
     * true-up 的判定結果。<b>四個去向互斥且加總必等於 {@code |diff|}</b>，
     * 所以「差額被吃掉」在結構上是可偵測的（見 {@link #assertFullyAllocated}）。
     *
     * @param diff             {@code actual − estimated}（可正可負）
     * @param sellerChargeable &gt;0 ⇒ 補收，進賣家下一筆應收
     * @param sellerCredit     &gt;0 ⇒ 退還賣家，🔴 走貸記<b>不得吃掉</b>
     * @param platformAbsorbed 平台吸收（{@code PLATFORM_CAUSED} 或 platform subsidy）
     * @param disputed         🔴 {@code PROVIDER_CAUSED} ⇒ 先進爭議，<b>不得自動向賣家收</b>
     * @param bearer           {@link ResponsibilityRules#bearerOf} 的結果；{@code cause} 為 null 時為 SELLER
     * @param sellerSubsidy    原封帶出，供憑據記錄；🔴 它<b>沒有</b>參與上面任何一格的計算
     */
    public record TrueUpDecision(long diff,
                                 long sellerChargeable,
                                 long sellerCredit,
                                 long platformAbsorbed,
                                 long disputed,
                                 Bearer bearer,
                                 long sellerSubsidy) {

        /** 這次 true-up 需不需要動任何東西。 */
        public boolean isNoop() {
            return diff == 0L;
        }
    }

    /**
     * W4-1 ~ W4-4 的判定表，<b>純函式</b>。
     *
     * <pre>
     * diff &gt; 0（平台少收）  bearer=SELLER   → 先扣 platform subsidy，餘額補收賣家
     *                        bearer=PLATFORM → 全額平台吸收（PLATFORM_CAUSED，不得轉嫁）
     *                        bearer=DISPUTE  → 全額進爭議（PROVIDER_CAUSED，不得自動向賣家收）
     * diff &lt; 0（平台多收）  🔴 一律 Seller Credit，**不看責任、不看補貼**
     *                        （多收的錢本來就不是平台的；PLATFORM_CAUSED 也不能留著）
     * diff = 0              什麼都不做
     * </pre>
     *
     * @throws IllegalArgumentException 金額為負（fail-loud，不靜默夾到 0）
     */
    public static TrueUpDecision decide(TrueUpInput in) {
        requireNonNegative(in.estimatedCost(), "estimatedCost");
        requireNonNegative(in.actualCost(), "actualCost");
        requireNonNegative(in.platformSubsidy(), "platformSubsidy");
        requireNonNegative(in.sellerSubsidy(), "sellerSubsidy");

        long diff = Math.subtractExact(in.actualCost(), in.estimatedCost());
        Bearer bearer = in.cause() == null ? Bearer.SELLER : ResponsibilityRules.bearerOf(in.cause());

        if (diff == 0L) {
            return allocated(diff, 0L, 0L, 0L, 0L, bearer, in.sellerSubsidy());
        }
        if (diff < 0L) {
            // 🔴 W4-2：少收（實際 < 暫估）⇒ 建立 Seller Credit，**不得吃掉**。
            //    用 domain 的 creditFromResidual 而不是自己 negate：那一支同時守著
            //    「貸記本金不得被通道手續費侵蝕」（負向撥款是足額給賣家，通道成本由平台承擔）。
            long credit = LogisticsAmounts.creditFromResidual(diff);
            return allocated(diff, 0L, credit, 0L, 0L, bearer, in.sellerSubsidy());
        }

        // diff > 0：W4-1「建立 additional receivable，**除非**責任是 platform／provider／補貼」
        return switch (bearer) {
            // 🔴 PLATFORM_CAUSED：平台負擔，不得轉嫁賣家。
            case PLATFORM -> allocated(diff, 0L, 0L, diff, 0L, bearer, in.sellerSubsidy());
            // 🔴 PROVIDER_CAUSED：先進爭議／對帳，**不得自動向賣家收**。
            case DISPUTE -> allocated(diff, 0L, 0L, 0L, diff, bearer, in.sellerSubsidy());
            case SELLER -> {
                // 🔴 只有 platformSubsidy 抵得掉；sellerSubsidy 一毛都不參與（W4-4）。
                long absorbed = Math.min(diff, in.platformSubsidy());
                yield allocated(diff, Math.subtractExact(diff, absorbed), 0L, absorbed, 0L,
                        bearer, in.sellerSubsidy());
            }
        };
    }

    /**
     * 🔴 差額<b>必須</b>被四個去向完全吸收：{@code |diff| = chargeable + credit + absorbed + disputed}。
     *
     * <p>沒有這一句，「少算一格」的症狀是<b>一筆錢從帳上消失而每一層 log 都顯示成功</b>
     * —— 那正是 W4-2「不得吃掉」要防的東西。放在建構點而不是呼叫端，
     * 任何未來新增的分支都逃不掉。
     */
    private static TrueUpDecision allocated(long diff, long chargeable, long credit,
                                            long absorbed, long disputed,
                                            Bearer bearer, long sellerSubsidy) {
        TrueUpDecision d = new TrueUpDecision(diff, chargeable, credit, absorbed, disputed,
                bearer, sellerSubsidy);
        assertFullyAllocated(d);
        return d;
    }

    private static void assertFullyAllocated(TrueUpDecision d) {
        long allocated = d.sellerChargeable() + d.sellerCredit() + d.platformAbsorbed() + d.disputed();
        long magnitude = Math.abs(d.diff());
        if (allocated != magnitude) {
            throw new IllegalStateException(String.format(
                    "logistics: true-up 差額未被完全分配（diff=%d, chargeable=%d, credit=%d,"
                            + " absorbed=%d, disputed=%d）—— 差額被吃掉是 W4-2 明令禁止的",
                    d.diff(), d.sellerChargeable(), d.sellerCredit(),
                    d.platformAbsorbed(), d.disputed()));
        }
    }

    private static void requireNonNegative(long v, String what) {
        if (v < 0L) {
            throw new IllegalArgumentException("logistics: " + what + " 不得為負，got " + v);
        }
    }

    // =========================================================================
    // 2. 單筆對帳（原子）
    // =========================================================================

    /**
     * 一次 true-up 的完整輸入。
     *
     * @param statementLineIds    這趟配送在帳單上的<b>全部</b>明細列 id。
     *                            🔴 <b>不是單一列</b>：一趟配送可以有 DELIVERY ＋ WAITING ＋
     *                            CANCELLATION 好幾列，只對其中一列會讓其餘幾列<b>永遠停在
     *                            {@code UNMATCHED}</b>——那是一條沒有執行者的邊
     *                            （{@code 06-no-small-tails.md} §3.13 第 ② 問）
     * @param provider            ShippingMethod <b>value</b>（{@code '3003'}），同 ledger owner_ref_id 口徑
     * @param statementPeriod     {@code 'YYYY-MM'}（帳單口徑；帳本的 {@code settlement_period} 由它轉成 {@code 'YYYYMM'}）
     * @param providerDeliveryRef 對帳關聯鍵（{@code uber_deliveries.delivery_id}）
     * @param actualCost          該趟配送在帳單上的<b>實際</b>成本合計（＝上列各列金額之和）
     * @param cause               責任成因，可為 null（單純暫估誤差 ⇒ 依 W4-1 由賣家負擔）。
     *                            🔴 它是<b>輸入</b>不是本服務推導出來的：帳單的真實欄位未知（EV-U2），
     *                            判不出來時由呼叫端落 {@code PROVIDER_CAUSED} 先進爭議
     * @param actorAccountId      ADM 操作者的 <b>account_id</b>（⚠️ 不是 store_id）
     */
    public record ReconcileCommand(List<Long> statementLineIds,
                                   String provider,
                                   String statementRef,
                                   String statementPeriod,
                                   String providerDeliveryRef,
                                   long actualCost,
                                   ResponsibilityCause cause,
                                   long platformSubsidy,
                                   long sellerSubsidy,
                                   long actorAccountId) {

        public ReconcileCommand {
            if (statementLineIds == null || statementLineIds.isEmpty()) {
                throw new IllegalArgumentException("logistics: statementLineIds 不得為空");
            }
            statementLineIds = List.copyOf(statementLineIds);
        }

        /**
         * 憑據欄（{@code logistics_receivables.statement_line_id}／
         * {@code logistics_adjustments.statement_line_id}）只放得下一個 id ⇒ 取<b>最小</b>的那個。
         * 🔴 取最小而不是「第一個」：呼叫端的順序不保證穩定，而這個值會被寫進 DB，
         * 不穩定的話同一份帳單重跑會指到不同列。
         */
        public long primaryLineId() {
            return statementLineIds.stream().mapToLong(Long::longValue).min().orElseThrow();
        }
    }

    /** 每一格的後續處置都不同，所以不合併成布林。 */
    public enum ReconcileOutcome {
        /** true-up 完成，應收推進 {@code CLOSED}（含 diff=0／補收／平台吸收／貸記）。 */
        RECONCILED,
        /**
         * 🔴 進爭議：{@code PROVIDER_CAUSED} 的超額成本，或帳單列對不到<b>唯一</b>應收。
         * <b>不動賣家任何一毛錢</b>，也不回填 {@code actual_cost}。
         */
        DISPUTED,
        /**
         * 同一列重跑（該趟配送已結清、且這幾列都已離開 {@code UNMATCHED}）⇒ 什麼都不做。
         * <b>不是錯誤。</b>
         */
        ALREADY_RECONCILED,
        /**
         * 應收還不在 {@code SETTLED}（帳單早於收款抵達）⇒ 這一輪不處理，帳單列留在
         * {@code UNMATCHED} 等下一輪。🔴 <b>不是</b>錯誤，也<b>不得</b>當成「對完了」。
         */
        NOT_YET_SETTLED
    }

    /**
     * @param decision {@code null} 代表還沒走到判定（對不到應收／狀態不對／重放）
     */
    public record ReconcileResult(ReconcileOutcome outcome, TrueUpDecision decision, Long receivableId) {
    }

    /**
     * 對一筆帳單明細做 true-up。
     *
     * <p>順序是硬性的：
     * <ol>
     *   <li><b>先配對</b>（{@code provider_delivery_ref}）。0 筆或多筆 ⇒ 爭議，🔴 不猜。</li>
     *   <li><b>狀態必須是 {@code SETTLED}</b>：true-up 的唯一合法起點
     *       （{@code CLOSED} 的 allowed-from ＝ {@code {SETTLED}}）。</li>
     *   <li><b>判定</b>。{@code disputed > 0} ⇒ 立刻走爭議，
     *       🔴 <b>不回填 {@code actual_cost}</b> —— {@code applyTrueUp} 是 write-once
     *       （{@code AND actual_cost IS NULL}），把一個還在爭議中的數字寫進去，
     *       爭議結果出來時<b>改不回來</b>，而 {@code chk_logrecv_recon_pair} 又要求
     *       它與 {@code statement_line_id} 同生同滅。</li>
     *   <li><b>回填 {@code actual_cost} ＋ {@code statement_line_id}</b>（同一句，雙向等式）。
     *       這一句同時是<b>冪等閘</b>：回 false ＝ 已對過帳 ⇒ 後面一步都不做。</li>
     *   <li>帳本（provider 成本應計 → 帳單列 {@code MATCHED} → 賣家軸 true-up）、
     *       調整單、餘額、狀態。</li>
     * </ol>
     */
    @Transactional
    public ReconcileResult reconcileOne(ReconcileCommand c) {
        List<LogisticsReceivableEntity> matched =
                receivables.findByProviderDeliveryRef(c.providerDeliveryRef());
        if (matched.size() != 1) {
            // 🔴 0 筆＝帳單有我方無紀錄的費用；多筆＝關聯鍵不唯一。兩者都**不自動改金額、不自動補列**。
            openCase(CASE_CATEGORY_STATEMENT_UNMATCHED, c, null,
                    "帳單列對不到唯一應收（命中 " + matched.size() + " 筆）");
            markDisputed(c);
            log.warn("LogisticsReconciliationService.reconcile.unmatched",
                    "帳單列對不到唯一應收，命中 " + matched.size() + " 筆 ⇒ 進爭議",
                    "providerDeliveryRef", c.providerDeliveryRef());
            return new ReconcileResult(ReconcileOutcome.DISPUTED, null, null);
        }

        LogisticsReceivableEntity r = matched.get(0);

        // 🔴 終態（CLOSED／WRITTEN_OFF）與「還沒到 SETTLED」是**兩件事**，處置相反。
        //    合成一格的症狀：一列事後才來的費用被當成「等下一輪」，而下一輪它<b>永遠</b>等不到
        //    —— 應收不會再離開終態，那是一條沒有執行者的邊。
        if (ReceivableStatusRules.isTerminal(LogisticsReceivableStatus.fromValue(r.getStatus()))) {
            List<Long> stillUnmatched = c.statementLineIds().stream()
                    .filter(id -> lines.findById(id)
                            .map(l -> RECON_UNMATCHED.equals(l.getReconStatus()))
                            .orElse(false))
                    .toList();
            if (stillUnmatched.isEmpty()) {
                // 同一列重跑（帳單重對）：什麼都不做。
                log.info("LogisticsReconciliationService.reconcile.replay",
                        "該趟配送已結清且帳單列都已處理過，略過（不是錯誤）",
                        "receivableId", String.valueOf(r.getId()));
                return new ReconcileResult(ReconcileOutcome.ALREADY_RECONCILED, null, r.getId());
            }
            // 🔴 事後才來的費用：{@code actual_cost} 是 write-once，併不進已結清的那個數字
            //    ⇒ 一律進爭議由 ADM 處置。**不得**靜默丟掉，也不得偷偷再開一筆應收。
            openCase(CASE_CATEGORY_STATEMENT_UNMATCHED, c, r.getStoreId(),
                    "應收已結清（status=" + r.getStatus() + "）後又出現帳單費用 "
                            + c.actualCost() + " ⇒ 進爭議，不得併入已結清的 actual_cost");
            markDisputed(stillUnmatched);
            log.warn("LogisticsReconciliationService.reconcile.lateLine",
                    "應收已結清後才出現的帳單費用 ⇒ 進爭議",
                    "receivableId", String.valueOf(r.getId()));
            return new ReconcileResult(ReconcileOutcome.DISPUTED, null, r.getId());
        }

        if (!LogisticsReceivableStatus.SETTLED.value().equals(r.getStatus())) {
            // 帳單比收款早到。留在 UNMATCHED，下一輪再對——帳單列的 aging index 看得到它。
            log.warn("LogisticsReconciliationService.reconcile.notSettled",
                    "應收尚未 SETTLED（status=" + r.getStatus() + "）⇒ 本輪不對帳",
                    "receivableId", String.valueOf(r.getId()));
            return new ReconcileResult(ReconcileOutcome.NOT_YET_SETTLED, null, r.getId());
        }

        TrueUpDecision d = decide(new TrueUpInput(
                r.getEstimatedCost(), c.actualCost(), c.cause(),
                c.platformSubsidy(), c.sellerSubsidy()));

        if (d.disputed() > 0L) {
            // 🔴 PROVIDER_CAUSED：不得自動向賣家收，也不得把爭議中的金額寫成 actual_cost。
            openCase(CASE_CATEGORY_PROVIDER_DISPUTE, c, r.getStoreId(), String.format(
                    "PROVIDER_CAUSED 超額 %d（暫估 %d、帳單 %d）⇒ 先進爭議，不得自動向賣家收",
                    d.disputed(), r.getEstimatedCost(), c.actualCost()));
            markDisputed(c);
            log.warn("LogisticsReconciliationService.reconcile.providerDispute",
                    "PROVIDER_CAUSED 超額 " + d.disputed() + " ⇒ 爭議，賣家應收不動",
                    "receivableId", String.valueOf(r.getId()));
            return new ReconcileResult(ReconcileOutcome.DISPUTED, d, r.getId());
        }

        // 🔴 冪等閘：write-once 的 actual_cost。回 false ＝ 已對過帳，後面一步都不做。
        if (!receivables.applyTrueUp(r.getId(), c.actualCost(), c.primaryLineId())) {
            log.info("LogisticsReconciliationService.reconcile.replay",
                    "actual_cost 已回填過，略過（不是錯誤）",
                    "receivableId", String.valueOf(r.getId()));
            return new ReconcileResult(ReconcileOutcome.ALREADY_RECONCILED, d, r.getId());
        }

        String period = toSettlementPeriod(c.statementPeriod());
        LogisticsLedgerBuilders.Input in = new LogisticsLedgerBuilders.Input(
                r.getStoreId(), r.getOrderNo(), period, "ADMIN", sourceRef(c));

        // ── 線 C：provider 成本應計。帳單列的 ledger_txn_no 指的就是它。 ─────────────
        // 🔴 chk_provsettle_ledger 是雙向等式（MATCHED ⇔ ledger_txn_no 非 NULL）⇒
        //    即使 diff = 0（沒有任何賣家軸分錄），也必須有一筆分錄可指
        //    —— 那筆就是「平台這個月要付 provider 多少」，它本來就該在帳上。
        String costTxnNo = postCostAccrual(in, c, r.getStoreId());
        // 🔴 這趟配送的**每一列**都要 MATCHED，漏掉的那幾列會永遠停在 UNMATCHED。
        for (long lineId : c.statementLineIds()) {
            requireLineMatched(lineId, costTxnNo);
        }

        // ── 線 B：賣家軸 ────────────────────────────────────────────────────────
        if (d.sellerChargeable() > 0L) {
            // 憑據：diff 全額一列、平台吸收一列。🔴 **兩列分開**才看得出「平台吸收了多少」，
            //      合成一列淨額之後，補貼與成本誤差在事後永遠分不開（W4-4）。
            recordAdjustment(c, r, d.diff(), c.cause(),
                    String.format("true-up 補收：帳單 %d − 暫估 %d = %+d",
                            c.actualCost(), r.getEstimatedCost(), d.diff()));
            if (d.platformAbsorbed() > 0L) {
                recordAdjustment(c, r, -d.platformAbsorbed(), c.cause(),
                        String.format("平台補貼吸收 %d（sellerSubsidy=%d 不計入賣家應收）",
                                d.platformAbsorbed(), d.sellerSubsidy()));
            }
            LedgerTxn trueUp = LogisticsLedgerBuilders.logisticsTrueUp(in, d.sellerChargeable());
            ledger.post(ids.get(), trueUp, ledger.nextTxnNo(), nowUtc.get());
            requireBalanceApplied(accounts.addReceivable(r.getStoreId(), d.sellerChargeable()),
                    "addReceivable(true-up)", r.getStoreId());

        } else if (d.sellerCredit() > 0L) {
            // 🔴 W4-2：多收一律回賣家，**不得吃掉、不得變成 platform margin**。
            recordAdjustment(c, r, d.diff(), c.cause(),
                    String.format("true-up 退還：帳單 %d − 暫估 %d = %d ⇒ 轉為賣家貸記 %d",
                            c.actualCost(), r.getEstimatedCost(), d.diff(), d.sellerCredit()));
            LedgerTxn issue = LogisticsLedgerBuilders.logisticsCreditIssue(in, d.sellerCredit());
            ledger.post(ids.get(), issue, ledger.nextTxnNo(), nowUtc.get());
            requireBalanceApplied(accounts.addCredit(r.getStoreId(), d.sellerCredit()),
                    "addCredit(true-up)", r.getStoreId());

        } else if (d.platformAbsorbed() > 0L) {
            // 全額由平台吸收（PLATFORM_CAUSED，或補貼 >= diff）。
            // 🔴 賣家軸一毛不動，但**必須留痕**：「不得默默由平台吸收」講的是不得無聲，
            //    不是不得吸收。兩列相消的調整單讓「吸收了多少、憑什麼」在事後查得到。
            recordAdjustment(c, r, d.diff(), c.cause(),
                    String.format("true-up 差額 %+d（帳單 %d − 暫估 %d）",
                            d.diff(), c.actualCost(), r.getEstimatedCost()));
            recordAdjustment(c, r, -d.platformAbsorbed(), c.cause(),
                    String.format("平台全額吸收 %d（bearer=%s，sellerSubsidy=%d 不計入賣家應收）",
                            d.platformAbsorbed(), d.bearer(), d.sellerSubsidy()));
        }

        requireTransition(r, LogisticsReceivableStatus.CLOSED);

        log.info("LogisticsReconciliationService.reconcile.done",
                String.format("true-up 完成 diff=%d chargeable=%d credit=%d absorbed=%d bearer=%s",
                        d.diff(), d.sellerChargeable(), d.sellerCredit(), d.platformAbsorbed(), d.bearer()),
                "receivableId", String.valueOf(r.getId()));
        return new ReconcileResult(ReconcileOutcome.RECONCILED, d, r.getId());
    }

    // =========================================================================
    // 3. 內部
    // =========================================================================

    /**
     * {@code LOGISTICS_COST_ACCRUAL}：DEBIT {@code PLATFORM_LOGISTICS_COST}／
     * CREDIT {@code PROVIDER_PAYABLE}。
     *
     * @return 這筆分錄的 {@code txn_no}（帳單列要指回它）
     */
    private String postCostAccrual(LogisticsLedgerBuilders.Input in, ReconcileCommand c, long storeId) {
        long providerRefId = providerRefId(c.provider());
        LedgerTxn cost = LogisticsLedgerBuilders.logisticsCostAccrual(in, providerRefId, c.actualCost());
        String txnNo = ledger.nextTxnNo();
        if (ledger.post(ids.get(), cost, txnNo, nowUtc.get())) {
            return txnNo;
        }
        // 已認列過（重放）⇒ 取回原來那一筆的 txn_no。
        // 🔴 不能沿用剛才 nextTxnNo() 拿到的號碼：那個號碼沒有對應的表頭，
        //    寫進 ledger_txn_no 之後「指得回一筆分錄」就變成假的。
        return ledger.findByIdempotencyKey(cost.idempotencyKey())
                .map(t -> t.getTxnNo())
                .orElseThrow(() -> new IllegalStateException(
                        "logistics: 成本應計回重放但查不到原交易，idempotencyKey="
                                + cost.idempotencyKey() + " store=" + storeId));
    }

    /** 帳單列 {@code UNMATCHED → MATCHED}，與 {@code ledger_txn_no} 同一句寫（雙向等式）。 */
    private void requireLineMatched(long statementLineId, String ledgerTxnNo) {
        if (!lines.markMatched(statementLineId, RECON_MATCHED, ledgerTxnNo,
                RECON_ALLOWED_FROM_UNMATCHED)) {
            // 🔴 rows=0 不得讀成「別人先做過了」：它代表這一列已經不是 UNMATCHED
            //    （已配對到別的應收／已進爭議／已核銷），而那是對帳邏輯出問題。
            throw new IllegalStateException(String.format(
                    "logistics: 帳單列 %d 無法從 %s 推進到 %s（allowed-from 不含它的當前狀態）",
                    statementLineId, RECON_UNMATCHED, RECON_MATCHED));
        }
    }

    private void markDisputed(ReconcileCommand c) {
        markDisputed(c.statementLineIds());
    }

    private void markDisputed(List<Long> statementLineIds) {
        for (long statementLineId : statementLineIds) {
            if (!lines.markUnmatchedOutcome(statementLineId, RECON_DISPUTED,
                    RECON_ALLOWED_FROM_UNMATCHED)) {
                throw new IllegalStateException(String.format(
                        "logistics: 帳單列 %d 無法從 %s 推進到 %s",
                        statementLineId, RECON_UNMATCHED, RECON_DISPUTED));
            }
        }
    }

    private void recordAdjustment(ReconcileCommand c, LogisticsReceivableEntity r,
                                  long amount, ResponsibilityCause cause, String reason) {
        if (amount == 0L) {
            // chk_logadj_amount_nonzero：0 元的調整單沒有語意，只會讓對帳報表多一列雜訊。
            throw new IllegalStateException("logistics: 調整單金額不得為 0（" + reason + "）");
        }
        LogisticsAdjustmentEntity e = new LogisticsAdjustmentEntity();
        e.setId(ids.get());
        e.setReceivableId(r.getId());
        e.setStoreId(r.getStoreId());
        e.setProviderDeliveryRef(c.providerDeliveryRef());
        e.setKind(ADJ_KIND_RECONCILIATION);
        e.setAmount(amount);
        e.setResponsibilityCause(cause == null ? null : cause.value());
        e.setStatementLineId(c.primaryLineId());
        e.setCreatedBy(c.actorAccountId() > 0L ? c.actorAccountId() : null);
        e.setReason(reason);
        adjustments.create(e);
    }

    private void openCase(String category, ReconcileCommand c, Long storeId, String detail) {
        FulfillmentExceptionCaseEntity e = new FulfillmentExceptionCaseEntity();
        e.setId(ids.get());
        e.setCaseNo(cases.nextCaseNo());
        e.setSourceService(CASE_SOURCE_SERVICE);
        e.setCategory(category);
        e.setSeverity(CASE_SEVERITY_P1);
        e.setSubjectType(CASE_SUBJECT_TYPE);
        e.setSubjectRef(c.providerDeliveryRef());
        e.setStoreId(storeId);
        // 🔴 chk_exc_owner：OPEN ⇒ owner_account_id 必須為 NULL。開案不指派承辦人。
        e.setStatus(CASE_STATUS_OPEN);
        e.setDedupKey(category + ":" + CASE_SUBJECT_TYPE + ":" + c.providerDeliveryRef());
        e.setDetail(caseDetailJson(c, detail));
        e.setProposedActionCodes("[]");
        e.setOccurredAt(nowUtc.get());
        // 回 false ＝ 已有未結案的同 dedup_key 案件 ⇒ 不重複開（不是錯誤）。
        cases.openIfAbsent(e);
    }

    /**
     * {@code detail} 的 JSONB object。
     *
     * <p>⚠️ 逐欄手寫 JSON 而不是丟一個 map 給 Jackson：這裡的值全部是我方產生的
     * 數字與碼值，唯一可能含奇怪字元的是 {@code statementRef}／{@code providerDeliveryRef}，
     * 所以只對字串做最小轉義。{@code chk_exc_detail_object} 會擋住非 object。
     */
    private String caseDetailJson(ReconcileCommand c, String detail) {
        return "{"
                + "\"statementLineIds\":\"" + c.statementLineIds() + "\","
                + "\"statementRef\":\"" + jsonEscape(c.statementRef()) + "\","
                + "\"statementPeriod\":\"" + jsonEscape(c.statementPeriod()) + "\","
                + "\"provider\":\"" + jsonEscape(c.provider()) + "\","
                + "\"providerDeliveryRef\":\"" + jsonEscape(c.providerDeliveryRef()) + "\","
                + "\"actualCost\":\"" + c.actualCost() + "\","
                + "\"responsibilityCause\":" + (c.cause() == null ? "null"
                        : "\"" + c.cause().value() + "\"") + ","
                + "\"detail\":\"" + jsonEscape(detail) + "\""
                + "}";
    }

    private static String jsonEscape(String s) {
        if (s == null) {
            return "";
        }
        StringBuilder sb = new StringBuilder(s.length() + 8);
        for (int i = 0; i < s.length(); i++) {
            char ch = s.charAt(i);
            switch (ch) {
                case '"' -> sb.append("\\\"");
                case '\\' -> sb.append("\\\\");
                case '\n' -> sb.append("\\n");
                case '\r' -> sb.append("\\r");
                case '\t' -> sb.append("\\t");
                default -> {
                    if (ch < 0x20) {
                        sb.append(String.format("\\u%04x", (int) ch));
                    } else {
                        sb.append(ch);
                    }
                }
            }
        }
        return sb.toString();
    }

    /**
     * {@code source_ref}：指得回帳單的哪一趟配送（INV-L3）。
     *
     * <p>🔴 錨在 {@code statement_ref + provider_delivery_ref} 而不是某一列的 id：
     * 它同時是帳本 {@code idempotency_key} 的尾段，而一趟配送可能有多列
     * —— 用列 id 會讓同一趟配送因為「這次挑到不同的代表列」而重複認列成本。
     */
    private static String sourceRef(ReconcileCommand c) {
        return c.statementRef() + ":" + c.providerDeliveryRef();
    }

    /**
     * {@code 'YYYY-MM'}（帳單口徑）→ {@code 'YYYYMM'}（{@code chk_ledgtxn_period}）。
     *
     * <p>🔴 格式不符一律拋，<b>不猜也不補</b>：settlement_period 決定這筆分錄算進哪一期，
     * 猜錯的症狀是某個月的對帳差 N 元而事後無從得知是哪一筆。
     */
    static String toSettlementPeriod(String statementPeriod) {
        if (statementPeriod == null || statementPeriod.length() != 7
                || statementPeriod.charAt(4) != '-') {
            throw new IllegalArgumentException(
                    "logistics: statement_period 必須是 'YYYY-MM'，got " + statementPeriod);
        }
        String compact = statementPeriod.substring(0, 4) + statementPeriod.substring(5, 7);
        for (int i = 0; i < 6; i++) {
            char ch = compact.charAt(i);
            if (ch < '0' || ch > '9') {
                throw new IllegalArgumentException(
                        "logistics: statement_period 必須是 'YYYY-MM'，got " + statementPeriod);
            }
        }
        return compact;
    }

    /**
     * {@code provider}（ShippingMethod <b>value</b>，例 {@code '3003'}）→ ledger 的
     * {@code owner_ref_id}。
     *
     * <p>🔴 先過 {@code ShippingMethod} 的值域再 parse：直接 parse 會讓任何數字字串通過，
     * 而 {@code chk_ledgentry_ref} 只檢查「PROVIDER ⇒ ref &gt; 0」——一個不存在的 provider
     * 碼值會靜默落庫。
     */
    static long providerRefId(String provider) {
        if (!ShippingMethod.isValidValue(provider)) {
            throw new IllegalArgumentException(
                    "logistics: 未知的 provider 碼值 " + provider + "（應為 ShippingMethod 的 value）");
        }
        return Long.parseLong(provider);
    }

    private void requireTransition(LogisticsReceivableEntity r, LogisticsReceivableStatus to) {
        if (!receivables.transition(r.getId(), to)) {
            throw new IllegalStateException(String.format(
                    "logistics: receivable %d 無法從 %s 推進到 %s（allowed-from=%s 不含它）",
                    r.getId(), r.getStatus(), to.value(),
                    ReceivableStatusRules.allowedFromValues(to)));
        }
    }

    private void requireBalanceApplied(boolean applied, String what, long storeId) {
        if (!applied) {
            throw new IllegalStateException(
                    "logistics: " + what + " 影響 0 列，seller_logistics_accounts 缺 store_id=" + storeId);
        }
    }
}
