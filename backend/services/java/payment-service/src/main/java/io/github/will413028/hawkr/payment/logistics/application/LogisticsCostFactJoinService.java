package io.github.will413028.hawkr.payment.logistics.application;

import io.github.will413028.hawkr.common.obs.log.ObsLogger;
import io.github.will413028.hawkr.common.utils.code.ResponsibilityCause;
import io.github.will413028.hawkr.payment.logistics.domain.ResponsibilityCaseRules;
import io.github.will413028.hawkr.payment.logistics.domain.Bearer;
import io.github.will413028.hawkr.payment.logistics.domain.ResponsibilityRules;
import io.github.will413028.hawkr.payment.logistics.repository.FulfillmentExceptionCaseRepository;
import io.github.will413028.hawkr.payment.logistics.repository.LogisticsCostFactRepository;
import io.github.will413028.hawkr.payment.logistics.repository.LogisticsReceivableRepository;
import io.github.will413028.hawkr.payment.logistics.repository.entity.FulfillmentExceptionCaseEntity;
import io.github.will413028.hawkr.payment.logistics.repository.entity.LogisticsCostFactEntity;
import io.github.will413028.hawkr.payment.logistics.repository.entity.LogisticsReceivableEntity;
import java.util.List;
import java.util.Optional;
import org.springframework.stereotype.Service;

/**
 * 🔴 <b>durable join 的收斂點</b>（70 §3.1.4）——兩個獨立事實在這裡匯合。
 *
 * <h2>為什麼兩條路徑必須呼叫同一個方法</h2>
 * {@code logistics.cost.incurred}（成本）與 {@code N16}（責任終局）是<b>兩個獨立的
 * business fact</b>，抵達順序不定。若各自實作一份「齊備了就建應收」的判斷，
 * 那就是<b>兩份真相</b>——而它們漂移時的症狀是「某個抵達順序下應收建不出來」，
 * 沒有任何測試會自動發現，因為每條路徑自己都是綠的。
 *
 * <p>⇒ 先到的那一方<b>只負責把自己的事實持久化</b>，判斷一律交給 {@link #converge}。
 * 因此最終結果與抵達順序無關。
 *
 * <h2>🔴 收斂述詞（§3.1.1，連言）</h2>
 * <pre>
 *   provider cost incurred   AND   bearerOf(responsibilityCause) = SELLER
 * </pre>
 * 兩個條件都成立才建立應收；只有其中一個 ⇒ <b>durably wait</b>（什麼都不做，
 * 事實留在各自的表裡等對方）。
 *
 * <h2>⚠️ 禁止的捷徑</h2>
 * <ul>
 *   <li>🔴 <b>不得</b>把 counterparty 判定 shortcut 成 {@code cause == SELLER_CAUSED}
 *       ——{@code BUYER_CAUSED} 的 counterparty <b>也是</b> SELLER（§3.1.1 情形 3），
 *       shortcut 會讓那一整類漏建。三軸分離見 §3.6.1。</li>
 *   <li>🔴 <b>不得</b>在責任側憑空估一個 {@code estimated_cost}：金額的權威在 shipping domain，
 *       N16 不帶金額。憑空造一個正是 §14.1 禁止的那條路徑換個形狀，而且更糟——數字是編的。</li>
 *   <li>🔴 <b>不得</b>在成本先到時先建一筆「等責任」的 receivable：§3.1.1 逐字
 *       「未 financially final 期間不得建立 Seller logistics receivable」，
 *       而且 {@code PENDING} 是提交 worker 的 claim 述詞 ⇒ 那一列會被撈去向賣家收錢。</li>
 * </ul>
 */
@Service
public class LogisticsCostFactJoinService {

    private static final ObsLogger log = ObsLogger.forClass(LogisticsCostFactJoinService.class);

    /** 收斂的結果。🔴 每一格的後續處置都不同，所以不合併成布林。 */
    public enum Outcome {
        /** 成本事實還沒到 ⇒ durably wait（責任側已持久化在案件表）。 */
        AWAITING_COST_FACT,
        /** 責任判定還沒 financially final ⇒ durably wait（成本側已持久化在 cost fact 表）。 */
        AWAITING_RESPONSIBILITY,
        /** 兩者齊備且 counterparty ＝ SELLER ⇒ 這次真的建了應收。 */
        RECEIVABLE_CREATED,
        /** 兩者齊備、counterparty ＝ SELLER，但應收已存在 ⇒ 重投，idempotent no-op。 */
        RECEIVABLE_ALREADY_EXISTS,
        /** 兩者齊備但 counterparty ≠ SELLER ⇒ 正確地不建賣家應收（§6.3）。 */
        NO_SELLER_RECEIVABLE,
        /**
         * 🔴 牴觸，<b>不得靜默 ACK</b>：
         * <ul>
         *   <li>counterparty ≠ SELLER，卻已經有一筆賣家應收（正在向不該付的人收錢）</li>
         *   <li>同一條 leg 命中多筆應收（identity 不變式壞了）</li>
         *   <li>案件的 {@code resolution_code} 不是四個 resolved 值之一</li>
         * </ul>
         */
        CONFLICT
    }

    private final LogisticsCostFactRepository costFacts;
    private final FulfillmentExceptionCaseRepository cases;
    private final LogisticsReceivableRepository receivables;
    private final LogisticsCollectionService collection;

    public LogisticsCostFactJoinService(LogisticsCostFactRepository costFacts,
                                        FulfillmentExceptionCaseRepository cases,
                                        LogisticsReceivableRepository receivables,
                                        LogisticsCollectionService collection) {
        this.costFacts = costFacts;
        this.cases = cases;
        this.receivables = receivables;
        this.collection = collection;
    }

    /**
     * 對某一條 provider leg 求值收斂述詞。
     *
     * <p>🔴 <b>本方法是冪等的</b>，兩條路徑都可以隨時呼叫任意次：它只讀兩張表的<b>當前事實</b>，
     * 不依賴「誰先呼叫」。重投、crash 後重試、兩個事件同時抵達，結果都一樣。
     *
     * <p>⚠️ <b>並行安全不靠這裡的讀取，靠 DB 的 UNIQUE</b>：兩個 pod 同時判定「齊備」時，
     * 兩者都會走到建立應收，而 {@code uq_logrecv_delivery_ref} ＋ {@code uq_logrecv_idem}
     * 保證只有一筆寫得進去，另一邊拿到 {@code false} ⇒ {@link Outcome#RECEIVABLE_ALREADY_EXISTS}。
     * 🔴 <b>不得改用「先查再寫」當唯一保證</b>——那中間的窗口正是重複計費的來源。
     */
    public Outcome converge(String providerDeliveryRef) {
        if (providerDeliveryRef == null || providerDeliveryRef.isBlank()) {
            // 沒有 provider leg ＝ 沒有 provider cost incurred（§3.1.2）⇒ 條件 ① 永遠不成立。
            return Outcome.AWAITING_COST_FACT;
        }

        List<LogisticsReceivableEntity> existing =
                receivables.findByProviderDeliveryRef(providerDeliveryRef);
        if (existing.size() > 1) {
            // 🔴 uq_logrecv_delivery_ref 應使其不可能發生 ⇒ 走到這裡代表 identity 不變式壞了。
            //    不得取「第一筆」——那會讓帳對到錯的賣家身上。
            log.error("LogisticsCostFactJoinService.legIdentityBroken",
                    "同一條 provider leg 命中多筆應收，identity 不變式已損壞 ref="
                            + providerDeliveryRef + " count=" + existing.size());
            return Outcome.CONFLICT;
        }

        Optional<LogisticsCostFactEntity> fact =
                costFacts.findByProviderDeliveryRef(providerDeliveryRef);
        CauseLookup lookup = resolvedCauseOf(providerDeliveryRef);

        if (lookup instanceof CauseLookup.Corrupt corrupt) {
            // 🔴 案件宣告 RESOLVED 卻沒有合法成因 ⇒ financial finality 已損壞。
            //    **絕不能走 AWAITING**：那會讓訊息被正常 ACK，而這筆損壞就此無聲消失。
            //    走 CONFLICT ⇒ consumer 拋出 ⇒ 既有 retry／DLQ ⇒ 有人看得到。
            log.error("LogisticsCostFactJoinService.corruptFinality",
                    "責任案件已 RESOLVED 但 resolution_code 非法，financial finality 已損壞"
                            + " ref=" + providerDeliveryRef
                            + " resolutionCode=" + corrupt.rawCode()
                            + "（合法值僅四個 resolved ResponsibilityCause；"
                            + "RESPONSIBILITY_UNRESOLVED 是案件 category 不是成因，"
                            + "SYSTEM_CAUSED 不存在）");
            return Outcome.CONFLICT;
        }

        if (lookup instanceof CauseLookup.NotFinal) {
            // 責任尚未 financially final（案件不存在或未 RESOLVED）。
            // 🔴 此時 counterparty **尚未 resolved**，不是「先暫時算成賣家」（§3.1.1）。
            return fact.isPresent() ? Outcome.AWAITING_RESPONSIBILITY : Outcome.AWAITING_COST_FACT;
        }

        ResponsibilityCause resolved = ((CauseLookup.Final) lookup).cause();

        // 🔴 純推導，不落庫（§3.6.1 B 軸）。bearerOf 是沒有 default 的 switch expression：
        //    CodeMeta 新增成因而未指派負擔方時，這裡編譯期就會失敗，不會靜默落到預設值。
        Bearer counterparty = ResponsibilityRules.bearerOf(resolved);

        if (counterparty != Bearer.SELLER) {
            if (!existing.isEmpty()) {
                // 🔴 最重要的陰性保證：leg 上已有賣家應收，而責任判定說這筆成本不該由賣家承擔
                //    ⇒ **正在向不該付的人收錢**。必須被人看到，不能 log.warn 之後 ACK。
                log.error("LogisticsCostFactJoinService.counterpartyConflict",
                        "counterparty=" + counterparty + "（不得向賣家收），但該 leg 已存在賣家應收 ref="
                                + providerDeliveryRef + " receivableId=" + existing.get(0).getId()
                                + " cause=" + resolved.value());
                return Outcome.CONFLICT;
            }
            return Outcome.NO_SELLER_RECEIVABLE;
        }

        if (fact.isEmpty()) {
            // 責任已定但成本未到 ⇒ durably wait。
            // 🔴 **不得在這裡估一個金額**：權威在 shipping domain，N16 不帶金額。
            return Outcome.AWAITING_COST_FACT;
        }

        if (!existing.isEmpty()) {
            // 應收已存在 ⇒ 補上責任成因快照（若尚未有），這一步本身冪等。
            return snapshotIfAbsent(existing.get(0), resolved);
        }

        // 兩個條件都成立 ⇒ 建立應收。金額取自 cost fact，成因取自案件。
        LogisticsCostFactEntity f = fact.get();
        boolean created = collection.createReceivable(new LogisticsCollectionService.CostIncurred(
                f.getStoreId(),
                f.getOrderNo(),
                f.getShipmentId(),
                f.getProvider(),
                f.getProviderDeliveryRef(),
                f.getEstimatedCost(),
                0L,
                f.getOccurredAt()));

        if (!created) {
            // 並行：另一個 pod 先寫進去了。DB 的 UNIQUE 擋住重複計費，這裡只是認出來。
            log.info("LogisticsCostFactJoinService.receivableRace",
                    "應收已由並行路徑建立（UNIQUE 擋下重複）ref=" + providerDeliveryRef,
                    "cause", resolved.value());
            return receivables.findByProviderDeliveryRef(providerDeliveryRef).stream()
                    .findFirst()
                    .map(r -> snapshotIfAbsent(r, resolved))
                    .orElse(Outcome.RECEIVABLE_ALREADY_EXISTS);
        }

        Outcome snapshot = receivables.findByProviderDeliveryRef(providerDeliveryRef).stream()
                .findFirst()
                .map(r -> snapshotIfAbsent(r, resolved))
                .orElse(Outcome.CONFLICT);
        if (snapshot == Outcome.CONFLICT) {
            log.error("LogisticsCostFactJoinService.createdButNotFound",
                    "剛建立的應收查不回來 ref=" + providerDeliveryRef);
            return Outcome.CONFLICT;
        }
        log.info("LogisticsCostFactJoinService.receivableCreated",
                "兩個事實齊備，應收成立 ref=" + providerDeliveryRef
                        + " cost=" + f.getEstimatedCost(),
                "cause", resolved.value());
        return Outcome.RECEIVABLE_CREATED;
    }

    /**
     * 責任側查詢的<b>三態</b>結果。
     *
     * <h3>🔴 為什麼不能用 {@code Optional}</h3>
     * {@code Optional.empty()} 只能表達「沒有值」，而這裡有<b>兩種完全不同的沒有值</b>：
     * <ul>
     *   <li><b>還沒定案</b>（案件不存在／未 {@code RESOLVED}）—— 正常，等就好</li>
     *   <li><b>定案了但資料壞了</b>（{@code RESOLVED} 卻帶著非法 {@code resolution_code}）
     *       —— 🔴 <b>財務終局性已損壞</b>，絕不能當成「等一下」</li>
     * </ul>
     * 把後者壓成 {@code empty}，呼叫端只會看到 {@code AWAITING_RESPONSIBILITY}
     * ⇒ <b>訊息被正常 ACK</b> ⇒ 一筆已損壞的 financial finality 就此無聲消失。
     *
     * <p>⚠️ <b>光加 {@code log.error} 不算修好</b>：那讓問題「被記錄」而不是「不發生」，
     * 而 ACK 之後沒有任何東西會回來看那一行 log。
     */
    private sealed interface CauseLookup {
        /** 案件不存在、或尚未 {@code RESOLVED} ⇒ 正常等待。 */
        record NotFinal() implements CauseLookup {
        }

        /** 已 {@code RESOLVED} 且是四個合法 resolved cause 之一。 */
        record Final(ResponsibilityCause cause) implements CauseLookup {
        }

        /** 🔴 已 {@code RESOLVED}，但 {@code resolution_code} 非法 ⇒ CONFLICT。 */
        record Corrupt(String rawCode) implements CauseLookup {
        }
    }

    /**
     * 取該 leg 的 <b>financially final</b> 責任成因。
     *
     * <p>🔴 三個條件缺一不可，而且每一個都擋掉一種「看起來已經判了、其實還會變」的狀態：
     * <ul>
     *   <li>案件存在且 {@code category = RESPONSIBILITY_UNRESOLVED}——其他 category 的案件
     *       與責任判定無關（例如對帳不符）</li>
     *   <li>{@code status = RESOLVED}——{@code IN_PROGRESS} 代表還在補證據</li>
     *   <li>{@code resolution_code} 是四個 <b>resolved</b> 成因之一——
     *       🔴 {@code RESPONSIBILITY_UNRESOLVED} 是案件 category <b>不是</b>成因，
     *       {@code SYSTEM_CAUSED} 不存在（平台／系統原因一律歸 {@code PLATFORM_CAUSED}）</li>
     * </ul>
     *
     * <p>🔴 <b>前兩個條件不成立 ＝ 還沒定案（可以等）；第三個不成立 ＝ 資料壞了（不能等）。</b>
     * 兩者的處置完全相反，所以回傳型別必須分得出來。
     *
     * <p>⚠️ <b>不依賴 V12 的 {@code chk_exc_responsibility_resolved_cause}</b>：
     * 那道 CHECK 擋的是<b>寫入</b>，而本方法讀的可能是 CHECK 加上之前就存在的舊列，
     * 也可能是一次繞過應用層的維運 SQL 寫進去的。<b>CHECK 存在不等於歷史資料都通過它。</b>
     */
    private CauseLookup resolvedCauseOf(String providerDeliveryRef) {
        // 🔴 category／subject_type／RESOLVED 三個常數一律取自 ResponsibilityCaseRules，
        //    不在這裡重打一份——兩份必然漂移，而漂移的症狀是「查不到案件」⇒ 應收永遠建不出來，
        //    且每一步都「照規則成功」、沒有任何錯誤訊號。
        Optional<FulfillmentExceptionCaseEntity> c = cases.findResponsibilityCase(
                ResponsibilityCaseRules.CATEGORY,
                ResponsibilityCaseRules.SUBJECT_TYPE,
                providerDeliveryRef);
        if (c.isEmpty()) {
            return new CauseLookup.NotFinal();
        }
        FulfillmentExceptionCaseEntity row = c.get();
        if (!ResponsibilityCaseRules.STATUS_RESOLVED.equals(row.getStatus())) {
            return new CauseLookup.NotFinal();
        }

        String code = row.getResolutionCode();
        if (code == null || code.isBlank() || !ResponsibilityCause.isValidValue(code)) {
            // 🔴 案件已經宣告 RESOLVED，卻沒有一個合法的成因 ⇒ financial finality 本身是壞的。
            //    它**不是**「還沒判」——已經有人（或某條路徑）把它標成判完了。
            //    當成等待 ⇒ 正常 ACK ⇒ 這筆損壞就此無聲消失，而下游的退款／應收／KPI
            //    全都以為「還在處理中」。
            return new CauseLookup.Corrupt(code == null ? "<null>" : code);
        }
        return new CauseLookup.Final(ResponsibilityCause.fromValue(code));
    }

    private Outcome snapshotIfAbsent(LogisticsReceivableEntity r, ResponsibilityCause cause) {
        String current = r.getResponsibilityCause();
        if (current != null && !current.isBlank()) {
            if (!current.equals(cause.value())) {
                // 🔴 既有快照與本次結論不同 ⇒ finality 不變式壞了（一條 leg 只能有一個
                //    financially final 的判定）。不得覆蓋——覆蓋會讓「改判」看起來像正常更新。
                log.error("LogisticsCostFactJoinService.causeConflict",
                        "既有責任成因快照與本次結論不同 receivableId=" + r.getId()
                                + " existing=" + current + " incoming=" + cause.value());
                return Outcome.CONFLICT;
            }
            return Outcome.RECEIVABLE_ALREADY_EXISTS;
        }
        receivables.snapshotResponsibilityCause(r.getId(), cause.value());
        return Outcome.RECEIVABLE_ALREADY_EXISTS;
    }
}
