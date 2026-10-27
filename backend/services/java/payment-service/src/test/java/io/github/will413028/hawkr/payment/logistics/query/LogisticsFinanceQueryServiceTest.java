package io.github.will413028.hawkr.payment.logistics.query;

import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.junit.jupiter.api.Assertions.assertNotEquals;
import static org.junit.jupiter.api.Assertions.assertThrows;
import static org.junit.jupiter.api.Assertions.assertTrue;
import static org.mockito.ArgumentMatchers.any;
import static org.mockito.ArgumentMatchers.anyInt;
import static org.mockito.ArgumentMatchers.anyLong;
import static org.mockito.ArgumentMatchers.anyString;
import static org.mockito.ArgumentMatchers.eq;
import static org.mockito.Mockito.lenient;
import static org.mockito.Mockito.mock;
import static org.mockito.Mockito.when;

import io.github.will413028.hawkr.common.utils.code.LogisticsReceivableStatus;
import io.github.will413028.hawkr.payment.logistics.application.LogisticsFinanceQueryService;
import io.github.will413028.hawkr.payment.logistics.application.LogisticsFinanceQueryService.Paged;
import io.github.will413028.hawkr.payment.logistics.application.LogisticsFinanceQueryService.SellerFinanceSummary;
import io.github.will413028.hawkr.payment.logistics.application.LogisticsFinanceQueryService.SellerReceivableView;
import io.github.will413028.hawkr.payment.logistics.repository.FulfillmentCaseResolutionRepository;
import io.github.will413028.hawkr.payment.logistics.repository.FulfillmentExceptionCaseRepository;
import io.github.will413028.hawkr.payment.logistics.repository.LedgerRepository;
import io.github.will413028.hawkr.payment.logistics.repository.LogisticsFinanceQueryRepository;
import io.github.will413028.hawkr.payment.logistics.repository.LogisticsReceivableRepository;
import io.github.will413028.hawkr.payment.logistics.repository.SellerLogisticsAccountRepository;
import io.github.will413028.hawkr.payment.logistics.repository.entity.LogisticsAdjustmentEntity;
import io.github.will413028.hawkr.payment.logistics.repository.entity.LogisticsCollectionAttemptEntity;
import io.github.will413028.hawkr.payment.logistics.repository.entity.LogisticsReceivableEntity;
import io.github.will413028.hawkr.payment.logistics.repository.entity.SellerLogisticsAccountEntity;
import io.github.will413028.hawkr.payment.logistics.repository.projection.LogisticsFinanceProjections.AdjustmentKindTotal;
import io.github.will413028.hawkr.payment.logistics.repository.projection.LogisticsFinanceProjections.CollectionAggregate;
import io.github.will413028.hawkr.payment.logistics.repository.projection.LogisticsFinanceProjections.ReceivableAggregate;
import io.github.will413028.hawkr.payment.logistics.repository.projection.LogisticsFinanceProjections.ReceivableStatusCount;
import java.lang.reflect.RecordComponent;
import java.time.LocalDateTime;
import java.util.Arrays;
import java.util.List;
import java.util.Optional;
import org.junit.jupiter.api.DisplayName;
import org.junit.jupiter.api.Nested;
import org.junit.jupiter.api.Test;

/**
 * 物流財務讀取模型的行為契約。
 *
 * <p>🔴 期望值一律硬寫字面值，不從被測物反推（{@code 300} / {@code 120} / {@code 180} 這三個
 * 數字就是整組測試的鑑別力所在：只要有人把兩個科目合成淨額，斷言就會看到 180）。
 */
@DisplayName("物流財務讀取模型")
class LogisticsFinanceQueryServiceTest {

    private static final long CALLER_STORE = 2685914682L;
    /** 🔴 刻意是不同數字（非對稱 ID 原則）：兩者相同的話「有沒有真的比對 store」零鑑別力。 */
    private static final long OTHER_STORE = 2632049831L;
    private static final LocalDateTime NOW = LocalDateTime.of(2026, 9, 12, 0, 0, 0);

    private final LogisticsFinanceQueryRepository query = mock(LogisticsFinanceQueryRepository.class);
    private final LogisticsReceivableRepository receivables = mock(LogisticsReceivableRepository.class);
    private final SellerLogisticsAccountRepository accounts = mock(SellerLogisticsAccountRepository.class);
    private final LedgerRepository ledger = mock(LedgerRepository.class);
    private final FulfillmentExceptionCaseRepository cases = mock(FulfillmentExceptionCaseRepository.class);
    private final FulfillmentCaseResolutionRepository resolutions =
            mock(FulfillmentCaseResolutionRepository.class);

    private LogisticsFinanceQueryService service() {
        return new LogisticsFinanceQueryService(query, receivables, accounts, ledger, cases, resolutions,
                () -> NOW);
    }

    private static LogisticsReceivableEntity receivable(long id, long storeId, String status) {
        LogisticsReceivableEntity r = new LogisticsReceivableEntity();
        r.setId(id);
        r.setStoreId(storeId);
        r.setStatus(status);
        r.setEstimatedCost(100L);
        r.setPlatformSubsidy(0L);
        r.setTargetNet(100L);
        r.setCarriedResidual(0L);
        r.setCreatedAt(NOW.minusDays(10));
        r.setUpdatedAt(NOW);
        return r;
    }

    private static SellerLogisticsAccountEntity account(long storeId, long receivableBalance, long credit) {
        SellerLogisticsAccountEntity a = new SellerLogisticsAccountEntity();
        a.setStoreId(storeId);
        a.setReceivableBalance(receivableBalance);
        a.setCreditBalance(credit);
        a.setLastActivityAt(NOW.minusDays(45));
        return a;
    }

    private void stubEmptySummary() {
        lenient().when(query.sellerReceivableAggregate(anyLong(), any(), any()))
                .thenReturn(ReceivableAggregate.empty());
        lenient().when(query.sellerCollectionAggregate(anyLong(), any(), any()))
                .thenReturn(CollectionAggregate.empty());
        lenient().when(query.sellerAdjustmentTotals(anyLong(), any(), any())).thenReturn(List.of());
        lenient().when(query.sellerStatusCounts(anyLong(), any(), any())).thenReturn(List.of());
    }

    // =====================================================================
    // 賣家範圍
    // =====================================================================

    @Nested
    @DisplayName("賣家範圍")
    class Scope {

        @Test
        @DisplayName("store_id <= 0 一律丟例外，不得塌成空結果")
        void missingStoreIdFailsLoud() {
            stubEmptySummary();
            LogisticsFinanceQueryService s = service();
            for (long bad : new long[] {0L, -1L}) {
                assertThrows(IllegalArgumentException.class, () -> s.sellerSummary(bad, null, null),
                        "store_id=" + bad + " 必須 fail loud —— 空結果看起來就像「這家店沒有物流費」");
                assertThrows(IllegalArgumentException.class,
                        () -> s.sellerReceivables(bad, List.of(), null, null, 1, 20));
                assertThrows(IllegalArgumentException.class, () -> s.sellerLedger(bad, 1, 20));
            }
        }

        @Test
        @DisplayName("收斂點守衛：查詢回了別家店的列 → 丟例外（不是靜默過濾掉）")
        void foreignStoreRowIsRejectedNotFiltered() {
            // 模擬「SQL 的 store 述詞壞了」：repository 回了一筆屬於別家店的應收。
            when(query.sellerListReceivables(eq(CALLER_STORE), any(), any(), any(), anyInt(), anyInt()))
                    .thenReturn(List.of(receivable(1L, OTHER_STORE, "PENDING")));

            IllegalStateException e = assertThrows(IllegalStateException.class,
                    () -> service().sellerReceivables(CALLER_STORE, List.of(), null, null, 1, 20));
            assertTrue(e.getMessage().contains(String.valueOf(OTHER_STORE)),
                    "錯誤訊息要指得出是哪一家店的列溢出來了，否則排查時只知道「壞了」");
        }

        @Test
        @DisplayName("查詢確實以呼叫端的 store 發問（不是某個寫死或撿來的值）")
        void queryIsIssuedWithCallerStore() {
            when(query.sellerListReceivables(eq(CALLER_STORE), any(), any(), any(), anyInt(), anyInt()))
                    .thenReturn(List.of(receivable(1L, CALLER_STORE, "PENDING")));
            when(query.sellerCountReceivables(eq(CALLER_STORE), any(), any(), any())).thenReturn(1L);
            when(query.latestAttemptsByReceivableIds(any())).thenReturn(List.of());
            when(query.adjustmentsByReceivableIds(any())).thenReturn(List.of());

            Paged<SellerReceivableView> paged =
                    service().sellerReceivables(CALLER_STORE, List.of(), null, null, 1, 20);

            assertEquals(1, paged.records().size());
            assertEquals(CALLER_STORE, paged.records().get(0).receivable().getStoreId());
        }
    }

    // =====================================================================
    // 十一個維度不合併
    // =====================================================================

    @Nested
    @DisplayName("十一個維度")
    class Dimensions {

        @Test
        @DisplayName("🔴 應收餘額與貸記餘額是兩個科目，永不合成淨額")
        void receivableAndCreditNeverNetted() {
            stubEmptySummary();
            when(accounts.find(CALLER_STORE)).thenReturn(Optional.of(account(CALLER_STORE, 300L, 120L)));

            SellerFinanceSummary s = service().sellerSummary(CALLER_STORE, null, null);

            assertEquals(300L, s.receivableBalance(), "賣家欠平台 300");
            assertEquals(120L, s.sellerCredit(), "平台欠賣家 120");
            // 這一句是本測試的核心：任何「合成一個帶正負的淨額」都會讓某一欄變成 180。
            assertNotEquals(180L, s.receivableBalance(),
                    "🔴 兩個科目被合成淨額了 —— ADR-008 第 1 條（永久可追蹤）失效");
            assertNotEquals(180L, s.sellerCredit(),
                    "🔴 兩個科目被合成淨額了 —— ADR-008 第 1 條（永久可追蹤）失效");
        }

        @Test
        @DisplayName("🔴 責任調整與對帳調整分兩個維度，不加總成一個「調整」")
        void responsibilityAndReconciliationStaySeparate() {
            stubEmptySummary();
            when(query.sellerAdjustmentTotals(eq(CALLER_STORE), any(), any())).thenReturn(List.of(
                    new AdjustmentKindTotal("RESPONSIBILITY", 70L, 2L),
                    new AdjustmentKindTotal("RECONCILIATION", -25L, 1L)));

            SellerFinanceSummary s = service().sellerSummary(CALLER_STORE, null, null);

            assertEquals(70L, s.responsibilityAdjustment(), "維度 7/11：這筆錢算誰的");
            assertEquals(-25L, s.reconciliationAdjustment(), "維度 8/11：帳單與暫估差多少（可負）");
            assertNotEquals(45L, s.responsibilityAdjustment(), "🔴 兩個維度被加總了");
        }

        @Test
        @DisplayName("🔴 讀取模型沒有 sellerSubsidy 欄位（本服務不是它的來源）")
        void sellerSubsidyHasNoFieldAtAll() {
            List<String> names = Arrays.stream(SellerFinanceSummary.class.getRecordComponents())
                    .map(RecordComponent::getName).toList();
            assertTrue(names.contains("platformSubsidy"), "平台補貼必須在（維度 5/11）");
            assertTrue(names.stream().noneMatch(n -> n.toLowerCase().contains("sellersubsidy")),
                    "🔴 賣家補貼（維度 6/11）在 payment-service 沒有來源。放一個恆為 0 的欄位會讓"
                            + "「查不到」長得跟「真的是 0」一樣 —— 缺口必須顯性（proto 以 optional 不設值表達）");
        }

        @Test
        @DisplayName("actual_cost 為 NULL 的筆數以 unreconciledCount 顯性表達，不當成 0 併進成本")
        void unreconciledIsExplicitNotZero() {
            when(query.sellerReceivableAggregate(eq(CALLER_STORE), any(), any()))
                    .thenReturn(new ReceivableAggregate(1000L, 400L, 3L, 980L, 20L, 0L, 5L));
            when(query.sellerCollectionAggregate(anyLong(), any(), any()))
                    .thenReturn(CollectionAggregate.empty());
            when(query.sellerAdjustmentTotals(anyLong(), any(), any())).thenReturn(List.of());
            when(query.sellerStatusCounts(anyLong(), any(), any())).thenReturn(List.of());

            SellerFinanceSummary s = service().sellerSummary(CALLER_STORE, null, null);

            assertEquals(400L, s.actualProviderCost(), "只含已對帳的部分");
            assertEquals(3L, s.unreconciledCount(), "尚未對帳的筆數必須看得到");
            assertEquals(1000L, s.estimatedProviderCost());
        }

        @Test
        @DisplayName("手續費同理：尚未查到 FundDate 的筆數以 pendingFeeQueryCount 顯性表達")
        void pendingFeeQueryIsExplicit() {
            stubEmptySummary();
            when(query.sellerCollectionAggregate(eq(CALLER_STORE), any(), any()))
                    .thenReturn(new CollectionAggregate(37L, 2L, -15L, 15L, 963L));

            SellerFinanceSummary s = service().sellerSummary(CALLER_STORE, null, null);

            assertEquals(37L, s.payuniCollectionFee(), "維度 4/11：實際手續費，不得用預估值冒充");
            assertEquals(2L, s.pendingFeeQueryCount());
            assertEquals(-15L, s.collectionResidual(), "維度 10/11：負值代表平台多收了");
        }

        @Test
        @DisplayName("狀態分佈帶 CodeMeta 的中文 label（不是在這裡硬寫一份碼表）")
        void statusCountsCarryCodeMetaLabel() {
            stubEmptySummary();
            when(query.sellerStatusCounts(eq(CALLER_STORE), any(), any())).thenReturn(List.of(
                    new ReceivableStatusCount("SETTLED", 4L),
                    new ReceivableStatusCount("UNCONFIRMED", 1L)));

            SellerFinanceSummary s = service().sellerSummary(CALLER_STORE, null, null);

            assertEquals(2, s.collectionStatusCounts().size());
            assertEquals(LogisticsReceivableStatus.SETTLED, s.collectionStatusCounts().get(0).status());
            assertEquals("已收款", s.collectionStatusCounts().get(0).status().label());
            assertEquals(4L, s.collectionStatusCounts().get(0).count());
        }

        @Test
        @DisplayName("DB 出現不在 CodeMeta 的狀態值 → FAIL 而不是靜默略過")
        void unknownStatusFailsLoud() {
            stubEmptySummary();
            when(query.sellerStatusCounts(eq(CALLER_STORE), any(), any()))
                    .thenReturn(List.of(new ReceivableStatusCount("TOTALLY_MADE_UP", 1L)));

            assertThrows(IllegalStateException.class,
                    () -> service().sellerSummary(CALLER_STORE, null, null),
                    "略過的話 CodeMeta 與 V9 的漂移永遠不會被發現");
        }
    }

    // =====================================================================
    // 單筆維度
    // =====================================================================

    @Nested
    @DisplayName("單筆應收")
    class PerReceivable {

        private SellerReceivableView oneRow(Long residual, String kind, long amount) {
            LogisticsReceivableEntity r = receivable(11L, CALLER_STORE, "SETTLED");
            LogisticsCollectionAttemptEntity a = new LogisticsCollectionAttemptEntity();
            a.setId(21L);
            a.setReceivableId(11L);
            a.setGrossAmount(101L);
            a.setResidual(residual);
            a.setActualFee(residual == null ? null : 1L);

            LogisticsAdjustmentEntity adj = new LogisticsAdjustmentEntity();
            adj.setId(31L);
            adj.setReceivableId(11L);
            adj.setStoreId(CALLER_STORE);
            adj.setKind(kind);
            adj.setAmount(amount);

            when(query.sellerListReceivables(eq(CALLER_STORE), any(), any(), any(), anyInt(), anyInt()))
                    .thenReturn(List.of(r));
            when(query.sellerCountReceivables(eq(CALLER_STORE), any(), any(), any())).thenReturn(1L);
            when(query.latestAttemptsByReceivableIds(any())).thenReturn(List.of(a));
            when(query.adjustmentsByReceivableIds(any())).thenReturn(List.of(adj));

            return service().sellerReceivables(CALLER_STORE, List.of(), null, null, 1, 20)
                    .records().get(0);
        }

        @Test
        @DisplayName("residual < 0（平台多收）→ 換算成正數的貸記本金")
        void negativeResidualBecomesCredit() {
            SellerReceivableView v = oneRow(-40L, "RECONCILIATION", -40L);
            assertEquals(40L, v.sellerCredit(), "維度 11/11：多收的錢要還，不得變成 platform margin");
            assertEquals(-40L, v.reconciliationAdjustment());
            assertEquals(0L, v.responsibilityAdjustment(), "另一個維度必須保持 0，不得被串到");
        }

        @Test
        @DisplayName("residual > 0（未收足）→ 沒有貸記")
        void positiveResidualHasNoCredit() {
            assertEquals(0L, oneRow(60L, "RESPONSIBILITY", 60L).sellerCredit());
        }

        @Test
        @DisplayName("residual 為 NULL（尚未查詢）→ 貸記為 0，且 latestAttempt 的欄位維持 null")
        void nullResidualIsNotZeroResidual() {
            SellerReceivableView v = oneRow(null, "RESPONSIBILITY", 5L);
            assertEquals(0L, v.sellerCredit());
            assertEquals(null, v.latestAttempt().getResidual(),
                    "🔴 尚未查詢必須維持 null —— 填 0 之後「還沒查」與「正好收足」就分不開了");
            assertEquals(null, v.latestAttempt().getActualFee());
        }
    }

    // =====================================================================
    // 帳齡
    // =====================================================================

    @Nested
    @DisplayName("帳齡（W6-4 的 overdue 格）")
    class Aging {

        @Test
        @DisplayName("非終態：自 created_at 起算")
        void nonTerminalAges() {
            assertEquals(10L, service().agingDaysOf(receivable(1L, CALLER_STORE, "PENDING")));
        }

        @Test
        @DisplayName("🔴 終態一律 0（CLOSED／WRITTEN_OFF 不是逾期，只是老）")
        void terminalNeverAges() {
            assertEquals(0L, service().agingDaysOf(receivable(1L, CALLER_STORE, "CLOSED")));
            assertEquals(0L, service().agingDaysOf(receivable(1L, CALLER_STORE, "WRITTEN_OFF")));
        }

        @Test
        @DisplayName("賣家帳戶的帳齡錨是 last_activity_at，不是「餘額為正」")
        void accountAgingUsesLastActivity() {
            assertEquals(45L, service().agingDaysOfAccount(account(CALLER_STORE, 0L, 0L)),
                    "餘額為 0 也可能已經 45 天沒動——錨是活動時間不是餘額");
        }
    }
}
