package io.github.will413028.hawkr.payment.logistics.application;

import io.github.will413028.hawkr.common.obs.log.ObsLogger;
import io.github.will413028.hawkr.common.utils.TsidUtil;
import io.github.will413028.hawkr.common.utils.code.CodeMetaValidator;
import io.github.will413028.hawkr.common.utils.code.ProviderFeeType;
import io.github.will413028.hawkr.common.utils.code.ResponsibilityCause;
import io.github.will413028.hawkr.common.utils.code.ShippingMethod;
import io.github.will413028.hawkr.payment.logistics.repository.ProviderSettlementLineRepository;
import io.github.will413028.hawkr.payment.logistics.repository.entity.ProviderSettlementLineEntity;
import java.util.ArrayList;
import java.util.LinkedHashMap;
import java.util.LinkedHashSet;
import java.util.List;
import java.util.Map;
import java.util.Set;
import java.util.function.Supplier;
import org.springframework.beans.factory.annotation.Autowired;
import org.springframework.beans.factory.annotation.Value;
import org.springframework.stereotype.Service;

/**
 * provider 帳單的<b>抽象層 ＋ 人工匯入路徑 ＋ 對帳引擎入口</b>（W4-5，70 §6.1）。
 *
 * <h2>🔴 {@code PROVIDER AUTOMATIC STATEMENT ENABLEMENT = DISABLED}</h2>
 * Uber 是否提供 statement／invoice API <b>完全沒有證據</b>（{@code statement}／{@code invoice}
 * 在 Uber 總覽各 <b>0 命中</b>，EV-U1）。⇒ <b>自動抓取那一半刻意只留一個旗標與一個會拋的入口</b>
 * （{@link #fetchFromProvider}），而 statement abstraction／manual import path／reconciliation
 * engine 三件<b>全部做完</b>。
 * <p>⚠️ 這不是偷懶也不是暫代：<b>沒有來源就不要造排程</b>（同《後端服務責任邊界規範》
 * 禁止 {@code *ReconciliationJob}／drift scanner 進 application code）。
 * 旗標預設 {@code false}，真的拿到 API 那天才有東西可以打開。
 *
 * <h2>🔴 為什麼批次迴圈在這裡，而單筆 true-up 在另一個 bean</h2>
 * {@link LogisticsReconciliationService#reconcileOne} 是 {@code @Transactional} 的<b>單筆</b>
 * 原子單位。在同一個 class 內以 {@code this.} 呼叫自家 {@code @Transactional} 方法
 * <b>不經 Spring proxy ⇒ 事務完全不存在</b>（10 檔 §11.1），各語句 auto-commit；
 * 而那一筆要寫五處（應收回填／帳本／調整單／餘額／狀態），少任何一處落地帳就對不起來。
 * ⇒ 迴圈必須<b>跨 bean</b> 呼叫它，所以住在這裡。
 * <p>⚠️ 同理，本迴圈<b>不得</b>被外層交易包住：一筆失敗不該讓整份帳單回滾
 * （那會讓「一列壞資料擋住整份帳單」）。per-item catch ＋ 逐筆短交易才是對的粒度。
 *
 * <h2>⚠️ 彙總列的去重缺口（誠實標註，不是可以靠 {@code ON CONFLICT} 解掉的）</h2>
 * {@code uq_provsettle_line} 含 {@code provider_shipment_ref}，而 PostgreSQL 的 NULL
 * <b>互不相等</b> ⇒ <b>彙總列（該欄為 NULL）不會被那個 UNIQUE 去重</b>，重複匯入會產生重複列。
 * ⇒ 本類別在匯入層擋：<b>只有這份帳單（{@code provider} ＋ {@code statement_ref}）一列都還沒有時，
 * 才收彙總列</b>；否則整批彙總列一律拒收並記一次 warn。
 */
@Service
public class ProviderStatementImporter {

    private static final ObsLogger log = ObsLogger.forClass(ProviderStatementImporter.class);

    /**
     * 🔴 W4-5 的旗標，<b>逐字</b>。它同時是 log 與回報裡的那一行字，
     * 讓「自動抓取沒有做」在輸出面而不是只在註解裡看得見。
     */
    public static final String PROVIDER_AUTOMATIC_STATEMENT_ENABLEMENT_DISABLED =
            "PROVIDER AUTOMATIC STATEMENT ENABLEMENT = DISABLED";

    /** 匯入時的初始 {@code recon_status}：一律未比對，🔴 匯入<b>不做任何帳務判斷</b>。 */
    static final String RECON_UNMATCHED = LogisticsReconciliationService.RECON_UNMATCHED;

    private final ProviderSettlementLineRepository lines;
    private final LogisticsReconciliationService reconciliation;
    private final Supplier<Long> ids;

    /**
     * 🔴 自動抓取帳單的總開關，<b>預設 false，而那是設計不是保守起見</b>（W4-5／EV-U1）。
     *
     * <p>⚠️ 呼叫端遇到停用應該<b>安靜跳過</b>而不是告警：把設計上的停用寫成告警，
     * 會讓真正的失敗被埋在噪音裡（同 {@code LogisticsCollectionService} 的收款閘門）。
     */
    private final boolean automaticStatementEnabled;

    @Autowired
    public ProviderStatementImporter(
            ProviderSettlementLineRepository lines,
            LogisticsReconciliationService reconciliation,
            @Value("${hawkr.payment.logistics.statement.automatic-fetch-enabled:false}")
            boolean automaticStatementEnabled) {
        this(lines, reconciliation, automaticStatementEnabled, TsidUtil::generate);
    }

    /** 測試用：注入 id 產生器。 */
    public ProviderStatementImporter(ProviderSettlementLineRepository lines,
                                     LogisticsReconciliationService reconciliation,
                                     boolean automaticStatementEnabled,
                                     Supplier<Long> ids) {
        this.lines = lines;
        this.reconciliation = reconciliation;
        this.automaticStatementEnabled = automaticStatementEnabled;
        this.ids = ids;
    }

    // =========================================================================
    // 1. Statement abstraction（W4-5）
    // =========================================================================

    /**
     * 帳單的<b>一列明細</b>——本抽象刻意停在這個層次。
     *
     * <p>🔴 它<b>不含</b>任何我方的判斷欄位（{@code recon_status}／{@code ledger_txn_no}／
     * {@code exception_case_id}）：那些是對帳的<b>產物</b>，讓匯入格式帶著它們，
     * 等於允許匯入端直接宣告「這一列已經對過帳了」。
     *
     * @param provider            ShippingMethod <b>value</b>（{@code '3003'}）
     * @param statementRef        一份帳單的識別（同一期可以有多份）
     * @param statementPeriod     {@code 'YYYY-MM'}
     * @param providerShipmentRef 對帳關聯鍵；🔴 <b>彙總列為 null</b>
     * @param feeType             CodeMeta {@link ProviderFeeType}
     * @param amount              元，🔴 恆為正（{@code chk_provsettle_amount}）；
     *                            正負由 {@code fee_type} 與對帳結果表達，不由金額表達
     */
    public record StatementLine(String provider,
                                String statementRef,
                                String statementPeriod,
                                String providerShipmentRef,
                                String orderNo,
                                String feeType,
                                long amount) {

        /** 🔴 彙總列：{@code uq_provsettle_line} 對它<b>不生效</b>（NULL 互不相等）。 */
        public boolean isAggregate() {
            return providerShipmentRef == null || providerShipmentRef.isBlank();
        }
    }

    /** 一列的匯入結果。三格的後續處置不同，所以不合併成布林。 */
    public enum LineImportStatus {
        /** 新建。 */
        IMPORTED,
        /** {@code uq_provsettle_line} 命中 ⇒ 這一列匯過了。<b>不是錯誤。</b> */
        DUPLICATE,
        /** 🔴 彙總列 ＋ 這份帳單已有明細 ⇒ 拒收（那個 UNIQUE 擋不住它，只能在這裡擋）。 */
        REJECTED_AGGREGATE_REIMPORT
    }

    /**
     * @param lineId {@code IMPORTED} 時是新建那一列的 id；其餘為 {@code null}
     */
    public record LineImportResult(StatementLine line, Long lineId, LineImportStatus status) {
    }

    public record ImportReport(List<LineImportResult> results) {

        public ImportReport {
            results = List.copyOf(results);
        }

        public long imported() {
            return count(LineImportStatus.IMPORTED);
        }

        public long duplicates() {
            return count(LineImportStatus.DUPLICATE);
        }

        public long rejectedAggregates() {
            return count(LineImportStatus.REJECTED_AGGREGATE_REIMPORT);
        }

        private long count(LineImportStatus s) {
            return results.stream().filter(r -> r.status() == s).count();
        }
    }

    // =========================================================================
    // 2. Manual import path（W4-5）
    // =========================================================================

    /**
     * 人工匯入一份帳單的明細列。
     *
     * <p>🔴 <b>匯入不做帳務</b>：不 post 分錄、不動應收、不判責任。一切都留給
     * {@link #reconcileStatement}——70 §6.2 逐字「不自動改金額、不自動補列」。
     *
     * @param importedBy ADM actor 的 <b>account_id</b>（⚠️ 不是 store_id）
     * @throws IllegalArgumentException 值域不合法（provider／feeType／amount／期別），fail-loud
     */
    public ImportReport importManual(List<StatementLine> statementLines, long importedBy) {
        if (statementLines == null || statementLines.isEmpty()) {
            throw new IllegalArgumentException("logistics: 匯入的帳單明細不得為空");
        }
        if (importedBy <= 0L) {
            // imported_by 是 NOT NULL 且語意是「誰匯的」——填 0 等於匯入者不明。
            throw new IllegalArgumentException("logistics: importedBy 必須是有效的 account_id");
        }
        statementLines.forEach(ProviderStatementImporter::validate);

        // 🔴 「這份帳單已經有列了嗎」必須在**整批開始之前**取一次快照。
        //    邊匯邊查的話，同一批裡的明細列會把後面的彙總列判成「重複匯入」——
        //    而那是<b>第一次</b>匯入，彙總列會在完全正常的情況下被拒收。
        Set<String> statementsWithExistingLines = new LinkedHashSet<>();
        for (StatementLine line : statementLines) {
            String key = line.provider() + "\u0000" + line.statementRef();
            if (!statementsWithExistingLines.contains(key) && statementAlreadyHasLines(line)) {
                statementsWithExistingLines.add(key);
            }
        }

        List<LineImportResult> results = new ArrayList<>(statementLines.size());
        for (StatementLine line : statementLines) {
            if (line.isAggregate()
                    && statementsWithExistingLines.contains(
                            line.provider() + "\u0000" + line.statementRef())) {
                // 🔴 那個 UNIQUE 擋不住彙總列（NULL 互不相等）⇒ 只能在這裡擋，而且要出聲。
                log.warn("ProviderStatementImporter.import.aggregateRejected",
                        "彙總列拒收：這份帳單已有明細，重複匯入會產生重複列",
                        "statementRef", line.statementRef());
                results.add(new LineImportResult(line, null,
                        LineImportStatus.REJECTED_AGGREGATE_REIMPORT));
                continue;
            }
            long id = ids.get();
            ProviderSettlementLineEntity e = new ProviderSettlementLineEntity();
            e.setId(id);
            e.setProvider(line.provider());
            e.setStatementRef(line.statementRef());
            e.setStatementPeriod(line.statementPeriod());
            e.setProviderShipmentRef(line.isAggregate() ? null : line.providerShipmentRef());
            e.setOrderNo(line.orderNo());
            e.setFeeType(line.feeType());
            e.setAmount(line.amount());
            e.setReconStatus(RECON_UNMATCHED);
            // 🔴 chk_provsettle_ledger 是雙向等式：非 MATCHED ⇒ ledger_txn_no 必須為 NULL。
            e.setLedgerTxnNo(null);
            e.setImportedBy(importedBy);

            boolean created = lines.importIfAbsent(e);
            results.add(new LineImportResult(line, created ? id : null,
                    created ? LineImportStatus.IMPORTED : LineImportStatus.DUPLICATE));
        }
        ImportReport report = new ImportReport(results);
        log.info("ProviderStatementImporter.import.done",
                String.format("帳單匯入 imported=%d duplicates=%d rejectedAggregates=%d",
                        report.imported(), report.duplicates(), report.rejectedAggregates()),
                "statementRef", statementLines.get(0).statementRef());
        return report;
    }

    /**
     * 🔴 自動抓取帳單——<b>本波沒有</b>，這裡只留一個會拋的入口。
     *
     * <p>留一個「回空清單」的實作比拋還糟：呼叫端會把「沒有這個能力」讀成「這期沒有帳單」，
     * 而那正是 {@code 06-no-small-tails.md} §3.10 第 ④ 問說的——對「目標不存在」是 SKIP 而非 FAIL，
     * 目標被搬走的那天它會靜默轉綠。
     *
     * @throws IllegalStateException 恆拋，訊息逐字帶 {@link #PROVIDER_AUTOMATIC_STATEMENT_ENABLEMENT_DISABLED}
     */
    public List<StatementLine> fetchFromProvider(String provider, String statementPeriod) {
        // 🔴 **旗標打開也還是拋**：旗標不是實作。讓「把開關扳開」以為就能自動抓，
        //    比停用本身危險得多——那會變成一個看起來已經接好、實際不存在的能力。
        throw new IllegalStateException(PROVIDER_AUTOMATIC_STATEMENT_ENABLEMENT_DISABLED
                + "（EV-U1：Uber 是否提供 statement／invoice API 完全沒有證據）"
                + " provider=" + provider + " period=" + statementPeriod
                + " enabledFlag=" + automaticStatementEnabled
                + " ⇒ 請走 importManual(...)");
    }

    /** {@link #fetchFromProvider} 能不能用。🔴 預設 false 是設計（W4-5），不是故障。 */
    public boolean isAutomaticStatementEnabled() {
        return automaticStatementEnabled;
    }

    // =========================================================================
    // 3. Reconciliation engine（W4-1 ~ W4-4）
    // =========================================================================

    /**
     * 責任成因與補貼拆分的<b>輸入</b>來源。
     *
     * <h2>🔴 為什麼是輸入而不是本服務推導</h2>
     * 帳單的真實欄位<b>未知</b>（EV-U2）⇒ 這個系統今天沒有原料可以判責任。
     * 造一個「依 fee_type 猜成因」的規則出來，會讓一個猜測長得像判定
     * ——而 {@code ResponsibilityRules} 的規則是<b>依成因</b>判定、<b>不依費用名稱</b>
     * （同一筆取消費在賣家改地址與外送員失聯兩種情況下歸屬完全相反）。
     * <p>⚠️ 判不出來時呼叫端應落 {@link ResponsibilityCause#PROVIDER_CAUSED}（先進爭議），
     * <b>不是</b>落 null——null 的語意是「沒有例外事件，單純暫估誤差」，那會直接開單向賣家收。
     */
    @FunctionalInterface
    public interface ReconciliationPolicy {

        /** @param providerDeliveryRef 對帳關聯鍵 */
        Attribution attributionOf(String providerDeliveryRef);

        /**
         * @param cause           可為 null（沒有例外事件）
         * @param platformSubsidy 🔴 平台吸收多少，<b>會</b>減少賣家要補付的金額
         * @param sellerSubsidy   🔴 賣家對<b>買家</b>的運費補貼（線 A），
         *                        <b>不得</b>減少賣家欠平台的物流成本（W4-4）
         */
        record Attribution(ResponsibilityCause cause, long platformSubsidy, long sellerSubsidy) {

            /** 沒有例外事件、沒有補貼：單純的暫估誤差，依 W4-1 由賣家負擔。 */
            public static final Attribution NONE = new Attribution(null, 0L, 0L);
        }

        /** 一律「沒有例外事件、沒有補貼」。批次重跑時的預設。 */
        static ReconciliationPolicy none() {
            return ref -> Attribution.NONE;
        }
    }

    /** 一趟配送的對帳結果。 */
    public record DeliveryReconcileResult(String providerDeliveryRef,
                                          List<Long> statementLineIds,
                                          long actualCost,
                                          LogisticsReconciliationService.ReconcileResult result,
                                          String failure) {
    }

    public record ReconcileReport(String statementRef, List<DeliveryReconcileResult> deliveries) {

        public ReconcileReport {
            deliveries = List.copyOf(deliveries);
        }

        public long countOf(LogisticsReconciliationService.ReconcileOutcome outcome) {
            return deliveries.stream()
                    .filter(d -> d.result() != null && d.result().outcome() == outcome)
                    .count();
        }

        /** 🔴 逐筆 catch 之後仍然要數得出來：失敗筆數為 0 才是「整份帳單對完了」。 */
        public long failures() {
            return deliveries.stream().filter(d -> d.failure() != null).count();
        }
    }

    /**
     * 對一份帳單的 {@code UNMATCHED} 明細列做 true-up。
     *
     * <p>🔴 <b>以 {@code provider_shipment_ref} 分組</b>：一趟配送可以有
     * DELIVERY ＋ WAITING ＋ CANCELLATION 好幾列，{@code actual_cost} 是它們的<b>合計</b>。
     * 逐列各對一次的話，第一列就把 {@code actual_cost} 寫死（write-once），
     * 其餘幾列<b>永遠停在 {@code UNMATCHED}</b>。
     *
     * <p>⚠️ <b>彙總列（{@code provider_shipment_ref} 為 NULL）不進對帳</b>：它對不到任何一趟
     * 配送。留在 {@code UNMATCHED} 由 ADM 處理——自動把它攤到某幾筆應收上就是「自動補列」。
     *
     * <p>⚠️ per-item catch：一列壞資料不得擋住整份帳單。失敗筆數由
     * {@link ReconcileReport#failures()} 數得出來，🔴 <b>不是</b>吞掉。
     */
    public ReconcileReport reconcileStatement(String provider, String statementRef,
                                              ReconciliationPolicy policy, long actorAccountId) {
        CodeMetaValidator.requireValidValue(ShippingMethod.class, provider, "provider");
        ReconciliationPolicy effective = policy == null ? ReconciliationPolicy.none() : policy;

        Map<String, List<ProviderSettlementLineEntity>> groups = new LinkedHashMap<>();
        for (ProviderSettlementLineEntity line : lines.listByStatementRef(provider, statementRef)) {
            if (!RECON_UNMATCHED.equals(line.getReconStatus())) {
                continue;
            }
            String ref = line.getProviderShipmentRef();
            if (ref == null || ref.isBlank()) {
                // 彙總列：留在 UNMATCHED，不猜它該攤給誰。
                continue;
            }
            groups.computeIfAbsent(ref, k -> new ArrayList<>()).add(line);
        }

        List<DeliveryReconcileResult> out = new ArrayList<>(groups.size());
        for (Map.Entry<String, List<ProviderSettlementLineEntity>> g : groups.entrySet()) {
            String deliveryRef = g.getKey();
            List<Long> lineIds = g.getValue().stream()
                    .map(ProviderSettlementLineEntity::getId).toList();
            long actualCost = 0L;
            String period = null;
            for (ProviderSettlementLineEntity line : g.getValue()) {
                actualCost = Math.addExact(actualCost, line.getAmount());
                period = line.getStatementPeriod();
            }
            try {
                // 🔴 policy 的呼叫**必須在 try 之內**：它是每一筆各自的判定原料，
                //    一筆的原料壞掉不得擋住整份帳單（把它放在外面就是「一列壞資料擋住全部」）。
                ReconciliationPolicy.Attribution a = effective.attributionOf(deliveryRef);
                LogisticsReconciliationService.ReconcileCommand cmd =
                        new LogisticsReconciliationService.ReconcileCommand(
                                lineIds, provider, statementRef, period, deliveryRef, actualCost,
                                a.cause(), a.platformSubsidy(), a.sellerSubsidy(), actorAccountId);
                // 🔴 跨 bean ⇒ @Transactional 真的生效：這一筆的五處寫入同進同退。
                LogisticsReconciliationService.ReconcileResult r = reconciliation.reconcileOne(cmd);
                out.add(new DeliveryReconcileResult(deliveryRef, lineIds, actualCost, r, null));
            } catch (RuntimeException ex) {
                // per-item catch：這一筆整組 rollback（交易在被呼叫端那一層），下一筆照跑。
                log.error("ProviderStatementImporter.reconcile.itemFailed",
                        "對帳失敗 deliveryRef=" + deliveryRef + "：" + ex.getMessage(),
                        "statementRef", statementRef, ex);
                out.add(new DeliveryReconcileResult(deliveryRef, lineIds, actualCost, null,
                        ex.getClass().getSimpleName() + ": " + ex.getMessage()));
            }
        }

        ReconcileReport report = new ReconcileReport(statementRef, out);
        log.info("ProviderStatementImporter.reconcile.done",
                String.format("帳單對帳 deliveries=%d reconciled=%d disputed=%d failures=%d（%s）",
                        out.size(),
                        report.countOf(LogisticsReconciliationService.ReconcileOutcome.RECONCILED),
                        report.countOf(LogisticsReconciliationService.ReconcileOutcome.DISPUTED),
                        report.failures(),
                        PROVIDER_AUTOMATIC_STATEMENT_ENABLEMENT_DISABLED),
                "statementRef", statementRef);
        return report;
    }

    // =========================================================================

    private boolean statementAlreadyHasLines(StatementLine line) {
        return !lines.listByStatementRef(line.provider(), line.statementRef()).isEmpty();
    }

    /**
     * 值域檢查，<b>在任何一列落地之前全部跑完</b>。
     *
     * <p>🔴 先全批驗再寫：混著寫會留下「前半匯進去了、後半因為第 7 列格式錯而中止」的半套帳單，
     * 而彙總列的去重閘門（{@link #statementAlreadyHasLines}）從此對這份帳單永久關閉
     * ——重匯的人會發現彙總列再也進不來。
     */
    static void validate(StatementLine line) {
        CodeMetaValidator.requireValidValue(ShippingMethod.class, line.provider(), "provider");
        CodeMetaValidator.requireValidCode(ProviderFeeType.class, line.feeType(), "feeType");
        if (line.statementRef() == null || line.statementRef().isBlank()) {
            throw new IllegalArgumentException("logistics: statementRef 為必填");
        }
        // 'YYYY-MM'；轉得出 'YYYYMM' 才進得了帳本的 settlement_period。
        LogisticsReconciliationService.toSettlementPeriod(line.statementPeriod());
        if (line.amount() <= 0L) {
            // chk_provsettle_amount：金額恆為正。負數的「退款列」要用 fee_type=ADJUSTMENT 表達，
            // 🔴 不得用負金額——一張表兩種正負慣例，彙總時必然有人算錯一邊。
            throw new IllegalArgumentException(
                    "logistics: 帳單金額必須為正（chk_provsettle_amount），got " + line.amount());
        }
    }

    /** 診斷用：這份帳單目前有幾列。 */
    public int lineCount(String provider, String statementRef) {
        return lines.listByStatementRef(provider, statementRef).size();
    }
}
