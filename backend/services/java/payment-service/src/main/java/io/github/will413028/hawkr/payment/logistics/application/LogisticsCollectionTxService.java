package io.github.will413028.hawkr.payment.logistics.application;

import io.github.will413028.hawkr.common.utils.DateTimeUtils;
import io.github.will413028.hawkr.common.utils.TsidUtil;
import io.github.will413028.hawkr.common.utils.code.LogisticsReceivableStatus;
import io.github.will413028.hawkr.payment.logistics.domain.LedgerTxn;
import io.github.will413028.hawkr.payment.logistics.domain.LogisticsAmounts;
import io.github.will413028.hawkr.payment.logistics.domain.LogisticsLedgerBuilders;
import io.github.will413028.hawkr.payment.logistics.domain.ReceivableStatusRules;
import io.github.will413028.hawkr.payment.logistics.repository.LedgerRepository;
import io.github.will413028.hawkr.payment.logistics.repository.LogisticsCollectionAttemptRepository;
import io.github.will413028.hawkr.payment.logistics.repository.LogisticsReceivableRepository;
import io.github.will413028.hawkr.payment.logistics.repository.SellerLogisticsAccountRepository;
import io.github.will413028.hawkr.payment.logistics.repository.entity.LogisticsReceivableEntity;
import io.github.will413028.hawkr.payment.logistics.repository.entity.SellerLogisticsAccountEntity;
import java.time.LocalDate;
import java.time.LocalDateTime;
import java.time.format.DateTimeFormatter;
import java.util.function.Supplier;
import org.springframework.beans.factory.annotation.Autowired;
import org.springframework.stereotype.Service;
import org.springframework.transaction.annotation.Transactional;

/**
 * 線 B 收款鏈裡<b>需要原子性</b>的那幾組寫入。
 *
 * <h2>為什麼是一個獨立的 bean（兩個理由，缺一都不成立）</h2>
 * <ol>
 *   <li><b>self-invocation 會讓 {@code @Transactional} 失效</b>（10 檔 §11.1）：在
 *       {@link LogisticsCollectionService} 內以 {@code this.} 呼叫自家 {@code @Transactional}
 *       方法不經 Spring proxy，事務<b>完全不存在</b>，各語句 auto-commit。
 *       payment-service 已有同形的先例：{@code ExpiredTransactionCloseService}。</li>
 *   <li><b>例外必須穿過 proxy 才會 rollback</b>：把交易掛在編排方法上、又在同一個方法內
 *       per-item catch 掉例外，事務不會被標 rollback-only ⇒ 前半段照樣 commit。
 *       跨 bean 的短交易讓「任一步失敗 ⇒ 這一筆整組不落地 ⇒ 下一輪重來」真的成立。</li>
 * </ol>
 *
 * <h2>🔴 交易內零網路呼叫</h2>
 * 本類別<b>只有 DB 寫入</b>。PayUni 的呼叫留在 {@link LogisticsCollectionService}
 * 的非交易區段（那邊標 {@code Propagation.NEVER}）。混進來的話會變成一條 90 秒的長交易
 * 持著連線，而且 attempt 的 pre-commit 不再是「送出前已落地」。
 *
 * <h2>⚠️ 命名</h2>
 * payment-service <b>沒有</b> {@code *TxHandler} 這個命名，它用
 * {@code *Service} / {@code *CloseService} ＋ 跨 bean 呼叫（{@code ExpiredTransactionCloseService}），
 * 或就地 {@code TransactionTemplate.execute}（{@code MembershipBillingApplicationService}）。
 * 本類別沿用前者——因為這裡要的正是它那條「例外穿過 proxy ⇒ 該筆 rollback」的保證，
 * 不是第三種寫法。
 */
@Service
public class LogisticsCollectionTxService {

    /** {@code settlement_period} 的格式：UTC 曆月 {@code YYYYMM}（DDL 是 {@code char(6)}）。 */
    private static final DateTimeFormatter SETTLEMENT_PERIOD = DateTimeFormatter.ofPattern("yyyyMM");

    private final LogisticsReceivableRepository receivables;
    private final LogisticsCollectionAttemptRepository attempts;
    private final SellerLogisticsAccountRepository accounts;
    private final LedgerRepository ledger;
    private final Supplier<Long> ids;
    private final Supplier<LocalDateTime> nowUtc;

    @Autowired
    public LogisticsCollectionTxService(LogisticsReceivableRepository receivables,
                                        LogisticsCollectionAttemptRepository attempts,
                                        SellerLogisticsAccountRepository accounts,
                                        LedgerRepository ledger) {
        this(receivables, attempts, accounts, ledger, TsidUtil::generate, DateTimeUtils::nowUtc);
    }

    /** 測試用：注入 id 產生器與時鐘。 */
    public LogisticsCollectionTxService(LogisticsReceivableRepository receivables,
                                        LogisticsCollectionAttemptRepository attempts,
                                        SellerLogisticsAccountRepository accounts,
                                        LedgerRepository ledger,
                                        Supplier<Long> ids,
                                        Supplier<LocalDateTime> nowUtc) {
        this.receivables = receivables;
        this.attempts = attempts;
        this.accounts = accounts;
        this.ledger = ledger;
        this.ids = ids;
        this.nowUtc = nowUtc;
    }

    /**
     * 應收成立的結果。
     *
     * @param created         {@code false} ＝ 冪等鍵衝突（送達事件重放）。🔴 不是錯誤
     * @param carriedResidual 本筆結轉進來的前期殘差（可負）
     */
    public record CreateOutcome(boolean created, long receivableId, long targetNet, long carriedResidual) {
    }

    /**
     * 建立應收、認列帳本、推進賣家餘額，<b>同一個交易</b>。
     *
     * <p>🔴 <b>結轉來自賣家帳戶的既有餘額</b>，而那個餘額就是前期未收足的部分。
     * ⚠️ 它<b>不是「上期的手續費」</b>——是「上期預估與實際的差」，費率估準時為 0。
     * 把它讀成手續費，就會做出 ADR-011 初版那個 fee-on-fee 遞迴。
     *
     * <p>🔴 帳本<b>只認列本期新增</b>那一塊（{@code accrual}），<b>不含 carried</b>：
     * 結轉是前期已認列、尚未收足的餘額，再認列一次就是重複認列收入——帳上多一筆不存在的收入，
     * 而現金並沒有多進來。
     */
    @Transactional
    public CreateOutcome createReceivableInTx(long receivableId,
                                              LogisticsCollectionService.CostIncurred e,
                                              long accrual) {
        accounts.ensureExists(e.storeId());
        long carried = accounts.find(e.storeId())
                .map(SellerLogisticsAccountEntity::getReceivableBalance)
                .orElseThrow(() -> new IllegalStateException(
                        // ensureExists 剛跑過，找不到只可能是同交易內被別的東西刪了 ⇒ fail-loud。
                        "logistics: seller_logistics_accounts 缺 store_id=" + e.storeId()));
        long targetNet = LogisticsAmounts.targetNet(accrual, carried);

        LogisticsReceivableEntity entity = new LogisticsReceivableEntity();
        entity.setId(receivableId);
        entity.setStoreId(e.storeId());
        entity.setOrderNo(e.orderNo());
        entity.setShipmentId(e.shipmentId());
        entity.setProvider(e.provider());
        entity.setProviderDeliveryRef(e.providerDeliveryRef());
        entity.setEstimatedCost(e.estimatedCost());
        entity.setPlatformSubsidy(e.platformSubsidy());
        entity.setTargetNet(targetNet);
        entity.setCarriedResidual(carried);
        // 🔴 計費依據凍結在**事件的業務時間**，不是 now()：之後政策改了不回溯本列。
        entity.setChargeBasisFrozenAt(e.occurredAt());
        entity.setIdempotencyKey(e.idempotencyKey());

        if (!receivables.createIfAbsent(entity)) {
            // 冪等鍵衝突 ＝ 事件重放。什麼都不做（連帳本都不寫）。
            return new CreateOutcome(false, receivableId, targetNet, carried);
        }

        if (accrual > 0L) {
            LedgerTxn txn = LogisticsLedgerBuilders.logisticsReceivableAccrual(
                    new LogisticsLedgerBuilders.Input(
                            e.storeId(), e.orderNo(), period(e.occurredAt()), "ORDER", e.idempotencyKey()),
                    accrual);
            post(txn, e.occurredAt());
            requireBalanceApplied(accounts.addReceivable(e.storeId(), accrual),
                    "addReceivable", e.storeId());
        }
        return new CreateOutcome(true, receivableId, targetNet, carried);
    }

    /**
     * 送出被受理（或未取得回應）：寫回 attempt 的受理結果 ＋ 應收推進 SUBMITTED。
     *
     * <p>🔴 {@code grossAmount} 寫回的是 <b>adapter 實際送出</b>的金額。
     * ⚠️ 本波它與 pre-commit 當下寫進去的值<b>相同</b>（gross 在 HTTP 呼叫之前就解算並落地了），
     * 這一句因此是冗餘而不是修正——保留它是為了讓「回應帶回不同 gross」這種未來變化不會靜默錯位。
     */
    @Transactional
    public void markSubmittedInTx(long receivableId, long attemptId,
                                  String submitStatus, String submitMessage, long grossAmount,
                                  LocalDateTime submittedAt, LocalDate expectedFundDate) {
        requireAttemptWritten(
                attempts.markSubmitted(attemptId, submitStatus, submitMessage, grossAmount,
                        submittedAt, expectedFundDate),
                attemptId, "markSubmitted");
        requireTransition(receivableId, LogisticsReceivableStatus.SUBMITTED);
    }

    /**
     * 送出被<b>明確拒絕</b>：寫回 attempt ＋ 應收推進 FAILED。
     *
     * <p>🔴 與「未取得回應」是兩件事：明確拒絕代表沒有受理、不存在「可能已經扣了」的窗口。
     * ⚠️ {@code expected_fund_date} 刻意留 {@code null}——沒有受理就沒有期望撥款日，
     * 填一個進去會讓查詢 worker 把它當成一筆在等撥款的收款。
     */
    @Transactional
    public void markRejectedInTx(long receivableId, long attemptId,
                                 String submitStatus, String submitMessage, long grossAmount,
                                 LocalDateTime submittedAt) {
        requireAttemptWritten(
                attempts.markSubmitted(attemptId, submitStatus, submitMessage, grossAmount,
                        submittedAt, null),
                attemptId, "markSubmitted(rejected)");
        requireTransition(receivableId, LogisticsReceivableStatus.FAILED);
    }

    /**
     * 確認入帳的完整帳務處理。
     *
     * @param residual {@code target_net − net_proceeds}，可為負（平台多收）
     */
    public record SettledCommand(long receivableId,
                                 long storeId,
                                 String orderNo,
                                 long attemptId,
                                 String tradeNo,
                                 LocalDate fundDate,
                                 long actualFee,
                                 long netProceeds,
                                 long residual) {
    }

    /**
     * 寫回查詢結果、推進 SETTLED、認列實收、必要時發貸記——<b>全部同一個交易</b>。
     *
     * <p>🔴 帳本記的是 <b>{@code net_proceeds}</b> 不是 gross：手續費是 PayUni 從撥款裡扣掉的，
     * 平台從來沒收到那筆錢。未收足的部分自然留在應收餘額裡，那就是要結轉的東西。
     *
     * <p>🔴 多收（{@code residual < 0}）⇒ 產生 Seller Logistics Credit，
     * <b>不得</b>變成 platform margin。
     */
    @Transactional
    public void recordSettledInTx(SettledCommand c) {
        // 🔴 rows=0 ⇒ 另一個持有者已經寫過 fund_date。**必須 fail-loud 並 rollback**：
        //    放行會讓下面的帳本與餘額被記第二次，而帳本的 idempotency_key 擋得住重複 txn、
        //    擋不住 addReceivable 這種 delta 累加。
        if (!attempts.recordQueryResult(c.attemptId(), c.fundDate(), c.actualFee(),
                c.netProceeds(), c.residual())) {
            throw new IllegalStateException(
                    "logistics: attempt " + c.attemptId() + " 已有 fund_date，拒絕重複入帳");
        }
        requireTransition(c.receivableId(), LogisticsReceivableStatus.SETTLED);

        String period = period(c.fundDate().atStartOfDay());
        if (c.netProceeds() > 0L) {
            LedgerTxn settled = LogisticsLedgerBuilders.logisticsCollectionSettled(
                    new LogisticsLedgerBuilders.Input(
                            c.storeId(), c.orderNo(), period, "PAYOUT", c.tradeNo()),
                    c.netProceeds());
            post(settled, nowUtc.get());
            requireBalanceApplied(accounts.addReceivable(c.storeId(), -c.netProceeds()),
                    "addReceivable(-net)", c.storeId());
        }

        long credit = LogisticsAmounts.creditFromResidual(c.residual());
        if (credit > 0L) {
            LedgerTxn issue = LogisticsLedgerBuilders.logisticsCreditIssue(
                    new LogisticsLedgerBuilders.Input(
                            c.storeId(), c.orderNo(), period, "ADMIN", c.tradeNo() + ":credit"),
                    credit);
            post(issue, nowUtc.get());
            requireBalanceApplied(accounts.addCredit(c.storeId(), credit),
                    "addCredit", c.storeId());
        }
    }

    // =========================================================================

    /** 🔴 表頭與分錄必須同交易（{@code trg_ledger_balanced} 在 COMMIT 當下才求值）——跨 bean 呼叫。 */
    private void post(LedgerTxn txn, LocalDateTime postedAt) {
        // 回 false ＝ 這筆 txn 已認列過（重放），不是錯誤：帳本以 idempotency_key 去重。
        ledger.post(ids.get(), txn, ledger.nextTxnNo(), postedAt);
    }

    /**
     * 🔴 CAS rows=0 一律 fail-loud。
     *
     * <p>它<b>不是</b>「別人先做過了」：它代表這筆已經不在 allowed-from 裡，而那是狀態機
     * 或呼叫順序有問題。靜默放行的症狀是「應收永遠停在某一格，而所有 log 都顯示處理成功」。
     */
    private void requireTransition(long receivableId, LogisticsReceivableStatus to) {
        if (!receivables.transition(receivableId, to)) {
            throw new IllegalStateException(String.format(
                    "logistics: receivable %d 無法推進到 %s（allowed-from=%s 不含它的當前狀態）",
                    receivableId, to.value(), ReceivableStatusRules.allowedFromValues(to)));
        }
    }

    /** attempt 的寫回 rows=0 ⇒ 那一列不存在或條件不符，同樣 fail-loud。 */
    private void requireAttemptWritten(boolean written, long attemptId, String what) {
        if (!written) {
            throw new IllegalStateException(
                    "logistics: attempt " + attemptId + " 的 " + what + " 影響 0 列");
        }
    }

    /**
     * 餘額 delta 影響 0 列 ⇒ 帳戶不存在。
     *
     * <p>🔴 <b>不得</b>當成「沒事」：那代表這筆錢沒有落到任何賣家帳上，而帳本那一側已經記了
     * ——兩邊從此對不起來，且沒有任何東西會叫。
     */
    private void requireBalanceApplied(boolean applied, String what, long storeId) {
        if (!applied) {
            throw new IllegalStateException(
                    "logistics: " + what + " 影響 0 列，seller_logistics_accounts 缺 store_id=" + storeId);
        }
    }

    private static String period(LocalDateTime utc) {
        return utc.format(SETTLEMENT_PERIOD);
    }
}
