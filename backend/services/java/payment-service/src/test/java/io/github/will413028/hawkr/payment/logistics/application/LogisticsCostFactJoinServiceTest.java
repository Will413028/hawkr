package io.github.will413028.hawkr.payment.logistics.application;

import static org.assertj.core.api.Assertions.assertThat;

import io.github.will413028.hawkr.common.utils.code.ResponsibilityCause;
import io.github.will413028.hawkr.payment.logistics.application.LogisticsCostFactJoinService.Outcome;
import io.github.will413028.hawkr.payment.logistics.domain.Bearer;
import io.github.will413028.hawkr.payment.logistics.domain.ResponsibilityCaseRules;
import io.github.will413028.hawkr.payment.logistics.domain.ResponsibilityRules;
import io.github.will413028.hawkr.payment.logistics.repository.FulfillmentExceptionCaseRepository;
import io.github.will413028.hawkr.payment.logistics.repository.LogisticsCostFactRepository;
import io.github.will413028.hawkr.payment.logistics.repository.LogisticsReceivableRepository;
import io.github.will413028.hawkr.payment.logistics.repository.entity.FulfillmentExceptionCaseEntity;
import io.github.will413028.hawkr.payment.logistics.repository.entity.LogisticsCostFactEntity;
import io.github.will413028.hawkr.payment.logistics.repository.entity.LogisticsReceivableEntity;
import java.time.LocalDateTime;
import java.util.ArrayList;
import java.util.LinkedHashMap;
import java.util.LinkedHashSet;
import java.util.List;
import java.util.Map;
import java.util.Optional;
import java.util.Set;
import org.junit.jupiter.api.DisplayName;
import org.junit.jupiter.api.Nested;
import org.junit.jupiter.api.Test;

/**
 * durable join 收斂點的行為（70 §3.1.4）。
 *
 * <h2>本檔取代 {@code ResponsibilitySettlementServiceTest}</h2>
 * 舊那一支測的是「責任側的私有處置」，而 Amendment 之後兩條路徑收斂到同一個方法
 * ——留兩份就是兩份真相。舊檔 15 支的<b>每一項保證都在本檔有對應</b>（見各 @DisplayName），
 * 另外補上 §21 要求的兩種抵達順序。
 *
 * <h2>🔴 替身真的維護狀態，而且鏡射 DB 的 UNIQUE</h2>
 * 只記 {@code calls} 的替身在這裡<b>零鑑別力</b>：它對任何寫入都回成功，
 * 於是「兩個事實同時抵達會建出兩筆應收」這種 bug 會全綠，
 * 而它在真 DB 上的症狀是<b>同一趟配送被收兩次錢</b>。
 *
 * <p>逐項鏡射：
 * <ul>
 *   <li>{@code uq_costfact_delivery_ref} ＋ {@code uq_costfact_event}（V16）</li>
 *   <li>{@code uq_logrecv_delivery_ref}（V15，<b>單欄</b>、不含 provider）</li>
 *   <li>{@code uq_exc_responsibility_subject}（V12，lifetime 唯一）</li>
 * </ul>
 */
@DisplayName("durable join：兩個獨立事實的收斂")
class LogisticsCostFactJoinServiceTest {

    private static final String REF = "del_abc";
    private static final String PROVIDER = "3003";
    private static final long STORE_ID = 2685914682L;
    private static final String ORDER_NO = "O-JOIN-1";
    private static final LocalDateTime NOW = LocalDateTime.of(2026, 9, 17, 12, 0);

    private FakeCostFacts costFacts;
    private FakeCases cases;
    private FakeReceivables receivables;
    private RecordingCollection collection;
    private LogisticsCostFactJoinService join;

    private LogisticsCostFactJoinService svc() {
        costFacts = new FakeCostFacts();
        cases = new FakeCases();
        receivables = new FakeReceivables();
        collection = new RecordingCollection(receivables);
        join = new LogisticsCostFactJoinService(costFacts, cases, receivables, collection);
        return join;
    }

    // =========================================================================
    // §21 A／B：兩種抵達順序，最終結果必須相同
    // =========================================================================

    @Nested
    @DisplayName("🔴 §21 抵達順序不影響最終結果")
    class ArrivalOrder {

        @Test
        @DisplayName("A. 成本先到 → 責任後到 ⇒ 恰好一筆應收")
        void costThenResponsibility() {
            LogisticsCostFactJoinService s = svc();

            // ① 成本先到：事實落庫，但責任未定 ⇒ 不得建應收
            costFacts.put(fact(REF, 115L));
            assertThat(s.converge(REF)).isEqualTo(Outcome.AWAITING_RESPONSIBILITY);
            assertThat(receivables.all())
                    .as("🔴 責任未 final 期間不得建立 Seller receivable（70 §3.1.1）")
                    .isEmpty();

            // ② 責任後到
            cases.resolve(REF, ResponsibilityCause.BUYER_CAUSED);
            assertThat(s.converge(REF)).isEqualTo(Outcome.RECEIVABLE_CREATED);

            assertThat(receivables.all()).hasSize(1);
            assertThat(receivables.all().get(0).getEstimatedCost())
                    .as("金額必須取自 cost fact，不得由責任側推導")
                    .isEqualTo(115L);
        }

        @Test
        @DisplayName("B. 責任先到 → 成本後到 ⇒ 恰好一筆應收（與 A 結果相同）")
        void responsibilityThenCost() {
            LogisticsCostFactJoinService s = svc();

            // ① 責任先到：案件已 RESOLVED，但成本未到 ⇒ 不得建應收
            cases.resolve(REF, ResponsibilityCause.BUYER_CAUSED);
            assertThat(s.converge(REF)).isEqualTo(Outcome.AWAITING_COST_FACT);
            assertThat(receivables.all())
                    .as("🔴 不得在這裡憑空估一個 estimated_cost（§14.1）")
                    .isEmpty();

            // ② 成本後到
            costFacts.put(fact(REF, 115L));
            assertThat(s.converge(REF)).isEqualTo(Outcome.RECEIVABLE_CREATED);

            assertThat(receivables.all()).hasSize(1);
            assertThat(receivables.all().get(0).getEstimatedCost()).isEqualTo(115L);
        }

        @Test
        @DisplayName("🔴 A 與 B 的最終狀態逐欄相同——這才是「順序不影響結果」的實質")
        void bothOrdersConvergeToTheSameRow() {
            LogisticsCostFactJoinService a = svc();
            costFacts.put(fact(REF, 115L));
            a.converge(REF);
            cases.resolve(REF, ResponsibilityCause.BUYER_CAUSED);
            a.converge(REF);
            LogisticsReceivableEntity viaA = receivables.all().get(0);

            LogisticsCostFactJoinService b = svc();
            cases.resolve(REF, ResponsibilityCause.BUYER_CAUSED);
            b.converge(REF);
            costFacts.put(fact(REF, 115L));
            b.converge(REF);
            LogisticsReceivableEntity viaB = receivables.all().get(0);

            assertThat(viaB.getEstimatedCost()).isEqualTo(viaA.getEstimatedCost());
            assertThat(viaB.getResponsibilityCause()).isEqualTo(viaA.getResponsibilityCause());
            assertThat(viaB.getProviderDeliveryRef()).isEqualTo(viaA.getProviderDeliveryRef());
            assertThat(viaB.getStoreId()).isEqualTo(viaA.getStoreId());
        }
    }

    // =========================================================================
    // §21 C／D／E：重複與並行
    // =========================================================================

    @Nested
    @DisplayName("§21 重複與並行")
    class Duplicates {

        @Test
        @DisplayName("C. 收斂重跑任意次 ⇒ 只有一筆應收（converge 冪等）")
        void convergeIsIdempotent() {
            LogisticsCostFactJoinService s = svc();
            costFacts.put(fact(REF, 115L));
            cases.resolve(REF, ResponsibilityCause.SELLER_CAUSED);

            assertThat(s.converge(REF)).isEqualTo(Outcome.RECEIVABLE_CREATED);
            assertThat(s.converge(REF)).isEqualTo(Outcome.RECEIVABLE_ALREADY_EXISTS);
            assertThat(s.converge(REF)).isEqualTo(Outcome.RECEIVABLE_ALREADY_EXISTS);

            assertThat(receivables.all()).hasSize(1);
            assertThat(collection.createCalls)
                    .as("🔴 第二次之後不得再呼叫 createReceivable——"
                            + "即使 DB 擋得住，多打一次也代表判斷錯了")
                    .isEqualTo(1);
        }

        @Test
        @DisplayName("D. 同一條 leg 的成本事實重複寫入 ⇒ 被 uq_costfact_delivery_ref 擋下")
        void duplicateCostFactIsRejected() {
            svc();
            assertThat(costFacts.insertIfAbsent(fact(REF, 115L))).isTrue();
            assertThat(costFacts.insertIfAbsent(fact(REF, 999L)))
                    .as("🔴 同一條 leg 只能有一筆成本事實；第二筆金額不同更是危險訊號")
                    .isFalse();
            assertThat(costFacts.findByProviderDeliveryRef(REF).orElseThrow().getEstimatedCost())
                    .as("先寫入的那筆不得被覆蓋——事實不會被改寫，後續費用走 true-up")
                    .isEqualTo(115L);
        }

        @Test
        @DisplayName("E. 並行：兩條路徑同時判定齊備 ⇒ UNIQUE 擋下第二筆，仍只有一筆應收")
        void concurrentConvergeCreatesOnlyOne() {
            LogisticsCostFactJoinService s = svc();
            costFacts.put(fact(REF, 115L));
            cases.resolve(REF, ResponsibilityCause.BUYER_CAUSED);

            // 模擬另一個 pod 已經搶先建好（fake 的 UNIQUE 會讓本次 create 回 false）
            receivables.seed(existingReceivable(REF, null));

            Outcome outcome = s.converge(REF);
            assertThat(outcome)
                    .as("並行敗者不得回 CONFLICT——那會讓一則正常訊息進 DLQ")
                    .isEqualTo(Outcome.RECEIVABLE_ALREADY_EXISTS);
            assertThat(receivables.all()).hasSize(1);
        }
    }

    // =========================================================================
    // 三軸分離（承接舊檔：Fault ≠ Settlement counterparty）
    // =========================================================================

    @Nested
    @DisplayName("三軸分離：Fault ≠ Settlement counterparty")
    class ThreeAxes {

        @Test
        @DisplayName("🔴 BUYER_CAUSED 的 counterparty 仍是 SELLER —— 全書最容易被讀反的一格")
        void buyerCausedStillSettlesWithSeller() {
            assertThat(ResponsibilityRules.bearerOf(ResponsibilityCause.BUYER_CAUSED))
                    .isEqualTo(Bearer.SELLER);

            LogisticsCostFactJoinService s = svc();
            costFacts.put(fact(REF, 115L));
            cases.resolve(REF, ResponsibilityCause.BUYER_CAUSED);

            assertThat(s.converge(REF))
                    .as("🔴 若有人把 counterparty shortcut 成 cause == SELLER_CAUSED，"
                            + "這一整類會漏建應收")
                    .isEqualTo(Outcome.RECEIVABLE_CREATED);
        }

        @Test
        @DisplayName("PROVIDER_CAUSED ⇒ DISPUTE，PLATFORM_CAUSED ⇒ PLATFORM（都不得向賣家收）")
        void nonSellerBearers() {
            assertThat(ResponsibilityRules.bearerOf(ResponsibilityCause.PROVIDER_CAUSED))
                    .isNotEqualTo(Bearer.SELLER);
            assertThat(ResponsibilityRules.bearerOf(ResponsibilityCause.PLATFORM_CAUSED))
                    .isNotEqualTo(Bearer.SELLER);
        }

        @Test
        @DisplayName("PROVIDER_CAUSED ＋ 成本已到 ⇒ 不建賣家應收（不是 AWAITING）")
        void providerCausedDoesNotCreate() {
            LogisticsCostFactJoinService s = svc();
            costFacts.put(fact(REF, 115L));
            cases.resolve(REF, ResponsibilityCause.PROVIDER_CAUSED);

            assertThat(s.converge(REF)).isEqualTo(Outcome.NO_SELLER_RECEIVABLE);
            assertThat(receivables.all()).isEmpty();
            assertThat(collection.createCalls).isZero();
        }

        @Test
        @DisplayName("PLATFORM_CAUSED ＋ 成本已到 ⇒ 不建賣家應收（平台吸收）")
        void platformCausedDoesNotCreate() {
            LogisticsCostFactJoinService s = svc();
            costFacts.put(fact(REF, 115L));
            cases.resolve(REF, ResponsibilityCause.PLATFORM_CAUSED);

            assertThat(s.converge(REF)).isEqualTo(Outcome.NO_SELLER_RECEIVABLE);
            assertThat(receivables.all()).isEmpty();
        }
    }

    // =========================================================================
    // 衝突（承接舊檔的陰性保證）
    // =========================================================================

    @Nested
    @DisplayName("🔴 衝突一律 fail-closed，不得靜默 ACK")
    class Conflicts {

        @Test
        @DisplayName("🔴 §21 G. PROVIDER_CAUSED 卻已有賣家應收 ⇒ CONFLICT（正在向不該付的人收錢）")
        void providerCausedWithExistingReceivableConflicts() {
            LogisticsCostFactJoinService s = svc();
            receivables.seed(existingReceivable(REF, null));
            cases.resolve(REF, ResponsibilityCause.PROVIDER_CAUSED);

            assertThat(s.converge(REF)).isEqualTo(Outcome.CONFLICT);
        }

        @Test
        @DisplayName("🔴 PLATFORM_CAUSED 卻已有賣家應收 ⇒ CONFLICT（不得轉嫁賣家）")
        void platformCausedWithExistingReceivableConflicts() {
            LogisticsCostFactJoinService s = svc();
            receivables.seed(existingReceivable(REF, null));
            cases.resolve(REF, ResponsibilityCause.PLATFORM_CAUSED);

            assertThat(s.converge(REF)).isEqualTo(Outcome.CONFLICT);
        }

        @Test
        @DisplayName("🔴 既有快照是**不同**成因 ⇒ CONFLICT，且不得覆蓋")
        void differentExistingSnapshotConflicts() {
            LogisticsCostFactJoinService s = svc();
            receivables.seed(existingReceivable(REF, ResponsibilityCause.SELLER_CAUSED.value()));
            costFacts.put(fact(REF, 115L));
            cases.resolve(REF, ResponsibilityCause.BUYER_CAUSED);

            assertThat(s.converge(REF)).isEqualTo(Outcome.CONFLICT);
            assertThat(receivables.all().get(0).getResponsibilityCause())
                    .as("🔴 不得覆蓋——覆蓋會讓「改判」看起來像正常更新")
                    .isEqualTo(ResponsibilityCause.SELLER_CAUSED.value());
        }

        @Test
        @DisplayName("重投同一個成因 ⇒ no-op，不是錯誤")
        void sameSnapshotIsIdempotent() {
            LogisticsCostFactJoinService s = svc();
            receivables.seed(existingReceivable(REF, ResponsibilityCause.BUYER_CAUSED.value()));
            costFacts.put(fact(REF, 115L));
            cases.resolve(REF, ResponsibilityCause.BUYER_CAUSED);

            assertThat(s.converge(REF)).isEqualTo(Outcome.RECEIVABLE_ALREADY_EXISTS);
        }

        @Test
        @DisplayName("🔴 同一 leg 命中多筆應收 ⇒ CONFLICT，不得取第一筆")
        void multipleReceivablesOnOneLegConflict() {
            LogisticsCostFactJoinService s = svc();
            receivables.seedIgnoringUniqueness(existingReceivable(REF, null));
            receivables.seedIgnoringUniqueness(existingReceivable(REF, null));
            cases.resolve(REF, ResponsibilityCause.BUYER_CAUSED);

            assertThat(s.converge(REF))
                    .as("取第一筆會讓帳對到錯的賣家身上")
                    .isEqualTo(Outcome.CONFLICT);
        }

        @Test
        @DisplayName("🔴🔴 案件已 RESOLVED 但 resolution_code 非法 ⇒ CONFLICT（**不是**等待）")
        void resolvedWithIllegalCauseIsConflictNotWaiting() {
            // 🔴 本檔最重要的一支。它與下面那支「IN_PROGRESS ⇒ 等待」是**一對**：
            //    兩者都「沒有合法成因」，但處置完全相反。
            //
            //    ⚠️ 本方法曾經回 AWAITING_RESPONSIBILITY——那是一個**偽裝成 fail-closed
            //    的 fail-open**：註解寫著 fail-closed、也有 log.error，
            //    但 AWAITING 會讓 consumer **正常 ACK** ⇒ 這筆已損壞的 financial finality
            //    就此無聲消失，而下游（退款／應收／KPI）全都以為「還在處理中」。
            //
            //    🔴 加 log.error 不算修好：那讓問題「被記錄」而不是「不發生」，
            //    而 ACK 之後沒有任何東西會回來看那一行 log。
            for (String illegal : new String[] {
                    "SYSTEM_CAUSED",              // 不存在的成因（平台原因一律歸 PLATFORM_CAUSED）
                    "RESPONSIBILITY_UNRESOLVED",  // 案件 category，不是成因
                    "seller_caused",              // 大小寫不符
                    "",                           // 空字串
                    null}) {                      // NULL
                LogisticsCostFactJoinService s = svc();
                costFacts.put(fact(REF, 115L));
                cases.resolveRaw(REF, illegal);

                assertThat(s.converge(REF))
                        .as("resolution_code=%s：案件已宣告 RESOLVED 卻沒有合法成因 "
                                + "⇒ financial finality 已損壞，必須 CONFLICT 讓 consumer 拋出進 DLQ", illegal)
                        .isEqualTo(Outcome.CONFLICT);

                // 🔴 陰性：不得有任何錯誤收款副作用
                assertThat(receivables.all())
                        .as("resolution_code=%s：不得建立應收", illegal)
                        .isEmpty();
                assertThat(collection.createCalls)
                        .as("resolution_code=%s：連 createReceivable 都不得被呼叫", illegal)
                        .isZero();
            }
        }

        @Test
        @DisplayName("對照：案件還沒 RESOLVED ⇒ 等待（證明上一支不是「什麼都回 CONFLICT」）")
        void notYetResolvedStillWaits() {
            // 🔴 這支是上一支的**對照組**：少了它，把 resolvedCauseOf 寫成「一律回 CONFLICT」
            //    也會讓上一支通過，而那會讓每一筆正常的等待都進 DLQ。
            LogisticsCostFactJoinService s = svc();
            costFacts.put(fact(REF, 115L));
            cases.inProgress(REF);

            assertThat(s.converge(REF)).isEqualTo(Outcome.AWAITING_RESPONSIBILITY);
            assertThat(receivables.all()).isEmpty();
        }
    }

    // =========================================================================
    // 邊界
    // =========================================================================

    @Nested
    @DisplayName("邊界")
    class Edges {

        @Test
        @DisplayName("providerDeliveryRef 為 null ⇒ 沒有 provider leg，等成本事實（不是錯誤）")
        void nullRefWaits() {
            LogisticsCostFactJoinService s = svc();
            assertThat(s.converge(null)).isEqualTo(Outcome.AWAITING_COST_FACT);
            assertThat(s.converge("")).isEqualTo(Outcome.AWAITING_COST_FACT);
        }

        @Test
        @DisplayName("案件存在但仍 IN_PROGRESS ⇒ 不算 financially final")
        void unresolvedCaseIsNotFinal() {
            LogisticsCostFactJoinService s = svc();
            costFacts.put(fact(REF, 115L));
            cases.inProgress(REF);

            assertThat(s.converge(REF)).isEqualTo(Outcome.AWAITING_RESPONSIBILITY);
            assertThat(receivables.all()).isEmpty();
        }

        // ⚠️ 原本這裡有一支 resolvedWithoutCodeIsNotFinal，斷言「RESOLVED 但 code 為空
        //    ⇒ AWAITING_RESPONSIBILITY」。**那個期望值本身就是被修掉的缺陷**：
        //    案件已宣告定案卻沒有合法成因 ＝ financial finality 損壞，不是「還沒判」。
        //    該情境（含 null 與空字串）已由 Conflicts 的
        //    resolvedWithIllegalCauseIsConflictNotWaiting 涵蓋並斷言 CONFLICT。

        @Test
        @DisplayName("正逆向兩條 leg 各自獨立收斂（各自一筆應收）")
        void forwardAndReturnLegsAreIndependent() {
            LogisticsCostFactJoinService s = svc();
            String forward = "del_fwd";
            String reverse = "del_ret";

            costFacts.put(fact(forward, 115L));
            cases.resolve(forward, ResponsibilityCause.BUYER_CAUSED);
            assertThat(s.converge(forward)).isEqualTo(Outcome.RECEIVABLE_CREATED);

            costFacts.put(fact(reverse, 115L));
            cases.resolve(reverse, ResponsibilityCause.BUYER_CAUSED);
            assertThat(s.converge(reverse))
                    .as("🔴 返程 leg 有自己的 provider_delivery_ref ⇒ 天然另一筆（70 §3.1.1 情形 4）")
                    .isEqualTo(Outcome.RECEIVABLE_CREATED);

            assertThat(receivables.all()).hasSize(2);
        }
    }

    // =========================================================================
    // fixtures ＋ 替身
    // =========================================================================

    private static LogisticsCostFactEntity fact(String ref, long cost) {
        LogisticsCostFactEntity f = new LogisticsCostFactEntity();
        f.setId(ref.hashCode() & 0xffffL);
        f.setProvider(PROVIDER);
        f.setProviderDeliveryRef(ref);
        f.setStoreId(STORE_ID);
        f.setOrderNo(ORDER_NO);
        f.setShipmentId(900_001L);
        f.setEstimatedCost(cost);
        f.setOccurredAt(NOW);
        f.setEventId("evt-" + ref);
        return f;
    }

    private static LogisticsReceivableEntity existingReceivable(String ref, String cause) {
        LogisticsReceivableEntity r = new LogisticsReceivableEntity();
        r.setId(700_000L + (ref.hashCode() & 0xffff));
        r.setStoreId(STORE_ID);
        r.setOrderNo(ORDER_NO);
        r.setProvider(PROVIDER);
        r.setProviderDeliveryRef(ref);
        r.setEstimatedCost(115L);
        r.setResponsibilityCause(cause);
        return r;
    }

    /** 鏡射 {@code uq_costfact_delivery_ref} ＋ {@code uq_costfact_event}（V16）。 */
    private static final class FakeCostFacts extends LogisticsCostFactRepository {
        private final Map<String, LogisticsCostFactEntity> byRef = new LinkedHashMap<>();
        private final Set<String> eventIds = new LinkedHashSet<>();

        FakeCostFacts() {
            super(null);
        }

        @Override
        public boolean insertIfAbsent(LogisticsCostFactEntity e) {
            if (!eventIds.add(e.getEventId())) {
                return false;
            }
            if (byRef.containsKey(e.getProviderDeliveryRef())) {
                return false;
            }
            byRef.put(e.getProviderDeliveryRef(), e);
            return true;
        }

        @Override
        public Optional<LogisticsCostFactEntity> findByProviderDeliveryRef(String ref) {
            return ref == null ? Optional.empty() : Optional.ofNullable(byRef.get(ref));
        }

        void put(LogisticsCostFactEntity e) {
            insertIfAbsent(e);
        }
    }

    /** 鏡射 {@code uq_exc_responsibility_subject}（V12，lifetime 唯一）。 */
    private static final class FakeCases extends FulfillmentExceptionCaseRepository {
        private final Map<String, FulfillmentExceptionCaseEntity> byRef = new LinkedHashMap<>();

        FakeCases() {
            super(null);
        }

        @Override
        public Optional<FulfillmentExceptionCaseEntity> findResponsibilityCase(
                String category, String subjectType, String subjectRef) {
            if (!ResponsibilityCaseRules.CATEGORY.equals(category)
                    || !ResponsibilityCaseRules.SUBJECT_TYPE.equals(subjectType)) {
                // 🔴 對照組：查詢條件寫錯時回 empty，而不是「碰巧也找得到」——
                //    否則把 category 打錯的 bug 會全綠。
                return Optional.empty();
            }
            return Optional.ofNullable(byRef.get(subjectRef));
        }

        void resolve(String ref, ResponsibilityCause cause) {
            resolveRaw(ref, cause.value());
        }

        void resolveRaw(String ref, String code) {
            FulfillmentExceptionCaseEntity c = base(ref);
            c.setStatus(ResponsibilityCaseRules.STATUS_RESOLVED);
            c.setResolutionCode(code);
            byRef.put(ref, c);
        }

        void inProgress(String ref) {
            FulfillmentExceptionCaseEntity c = base(ref);
            c.setStatus("IN_PROGRESS");
            byRef.put(ref, c);
        }

        private FulfillmentExceptionCaseEntity base(String ref) {
            FulfillmentExceptionCaseEntity c = new FulfillmentExceptionCaseEntity();
            c.setId(500_000L + (ref.hashCode() & 0xffff));
            c.setCategory(ResponsibilityCaseRules.CATEGORY);
            c.setSubjectType(ResponsibilityCaseRules.SUBJECT_TYPE);
            c.setSubjectRef(ref);
            return c;
        }
    }

    /** 鏡射 {@code uq_logrecv_delivery_ref}（V15，<b>單欄</b>、不含 provider）。 */
    private static final class FakeReceivables extends LogisticsReceivableRepository {
        private final List<LogisticsReceivableEntity> rows = new ArrayList<>();

        FakeReceivables() {
            super(null);
        }

        @Override
        public List<LogisticsReceivableEntity> findByProviderDeliveryRef(String ref) {
            return rows.stream().filter(r -> r.getProviderDeliveryRef().equals(ref)).toList();
        }

        @Override
        public boolean snapshotResponsibilityCause(long id, String cause) {
            for (LogisticsReceivableEntity r : rows) {
                if (r.getId() == id) {
                    r.setResponsibilityCause(cause);
                    return true;
                }
            }
            return false;
        }

        /** 走 UNIQUE（一條 leg 一筆）。 */
        boolean seed(LogisticsReceivableEntity r) {
            if (!findByProviderDeliveryRef(r.getProviderDeliveryRef()).isEmpty()) {
                return false;
            }
            rows.add(r);
            return true;
        }

        /** 🔴 繞過 UNIQUE，唯一合法用途：測「索引失效時的防禦分支」。 */
        void seedIgnoringUniqueness(LogisticsReceivableEntity r) {
            rows.add(r);
        }

        List<LogisticsReceivableEntity> all() {
            return List.copyOf(rows);
        }
    }

    /** 只負責把「建立應收」導向 fake 的 UNIQUE，並記錄呼叫次數。 */
    private static final class RecordingCollection extends LogisticsCollectionService {
        private final FakeReceivables receivables;
        int createCalls;

        RecordingCollection(FakeReceivables receivables) {
            super(null, null, null, null, null, false, 24, () -> 1L, () -> NOW);
            this.receivables = receivables;
        }

        @Override
        public boolean createReceivable(CostIncurred e) {
            createCalls++;
            LogisticsReceivableEntity r = new LogisticsReceivableEntity();
            r.setId(800_000L + (e.providerDeliveryRef().hashCode() & 0xffff));
            r.setStoreId(e.storeId());
            r.setOrderNo(e.orderNo());
            r.setProvider(e.provider());
            r.setProviderDeliveryRef(e.providerDeliveryRef());
            r.setEstimatedCost(e.estimatedCost());
            r.setIdempotencyKey(e.idempotencyKey());
            return receivables.seed(r);
        }
    }
}
