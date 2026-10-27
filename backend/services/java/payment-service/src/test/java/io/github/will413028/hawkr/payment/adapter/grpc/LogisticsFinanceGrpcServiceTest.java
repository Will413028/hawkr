package io.github.will413028.hawkr.payment.adapter.grpc;

import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.junit.jupiter.api.Assertions.assertFalse;
import static org.junit.jupiter.api.Assertions.assertNotNull;
import static org.junit.jupiter.api.Assertions.assertTrue;
import static org.mockito.ArgumentMatchers.any;
import static org.mockito.ArgumentMatchers.anyInt;
import static org.mockito.ArgumentMatchers.anyLong;
import static org.mockito.ArgumentMatchers.eq;
import static org.mockito.Mockito.mock;
import static org.mockito.Mockito.never;
import static org.mockito.Mockito.verify;
import static org.mockito.Mockito.when;

import io.github.will413028.hawkr.common.grpc.GrpcAuthContext;
import io.github.will413028.hawkr.common.utils.code.GrpcResponseCode;
import io.github.will413028.hawkr.payment.grpc.GetSellerLogisticsFinanceSummaryRequest;
import io.github.will413028.hawkr.payment.grpc.GetSellerLogisticsFinanceSummaryResponse;
import io.github.will413028.hawkr.payment.grpc.ListLogisticsReceivablesForAdminRequest;
import io.github.will413028.hawkr.payment.grpc.ListLogisticsReceivablesForAdminResponse;
import io.github.will413028.hawkr.payment.grpc.ListSellerLogisticsReceivablesRequest;
import io.github.will413028.hawkr.payment.grpc.ListSellerLogisticsReceivablesResponse;
import io.github.will413028.hawkr.payment.grpc.SellerLogisticsFinanceSummary;
import io.github.will413028.hawkr.payment.grpc.SellerLogisticsReceivableRecord;
import io.github.will413028.hawkr.payment.logistics.application.LogisticsFinanceQueryService;
import io.github.will413028.hawkr.payment.logistics.application.LogisticsFinanceQueryService.Paged;
import io.github.will413028.hawkr.payment.logistics.application.LogisticsFinanceQueryService.SellerFinanceSummary;
import io.github.will413028.hawkr.payment.logistics.application.LogisticsFinanceQueryService.SellerReceivableView;
import io.github.will413028.hawkr.payment.logistics.repository.entity.LogisticsCollectionAttemptEntity;
import io.github.will413028.hawkr.payment.logistics.repository.entity.LogisticsReceivableEntity;
import com.google.protobuf.Descriptors.Descriptor;
import io.grpc.Context;
import io.grpc.Status;
import io.grpc.StatusRuntimeException;
import io.grpc.stub.StreamObserver;
import java.time.LocalDateTime;
import java.util.List;
import org.junit.jupiter.api.DisplayName;
import org.junit.jupiter.api.Nested;
import org.junit.jupiter.api.Test;
import org.mockito.ArgumentCaptor;

/**
 * 物流財務 gRPC 出口的授權軸與 wire 契約。
 *
 * <p>單元測試環境無 gRPC Context ⇒ {@code isInternalCall()} 為 false，等同 Gateway 帶 JWT 呼叫。
 * account type code：1=BUYER、2=SELLER、3=SELLER_AND_BUYER、4=ADMIN。
 */
@DisplayName("物流財務 gRPC 出口")
class LogisticsFinanceGrpcServiceTest {

    private static final long CALLER_STORE = 2685914682L;
    private static final LocalDateTime NOW = LocalDateTime.of(2026, 9, 12, 3, 4, 5);

    private final LogisticsFinanceQueryService queryService = mock(LogisticsFinanceQueryService.class);

    private LogisticsFinanceGrpcService service() {
        // ⚠️ ObjectMapper 是真的不是 mock：它只用來解 impact_preview 的 JSON，
        //    而那正是本檔某些斷言要驗的輸出——mock 掉它等於驗不到解碼結果。
        return new LogisticsFinanceGrpcService(queryService, new com.fasterxml.jackson.databind.ObjectMapper());
    }

    private static Context ctx(String accountTypeCode) {
        return Context.current().withValue(GrpcAuthContext.ACCOUNT_TYPE, accountTypeCode);
    }

    private static SellerFinanceSummary summary(long receivableBalance, long credit) {
        return new SellerFinanceSummary(CALLER_STORE, null, null,
                1000L, 400L, 3L, 980L, 37L, 2L, 20L, 70L, -25L,
                List.of(), -15L, credit, receivableBalance, NOW.minusDays(45), 45L, true);
    }

    private GetSellerLogisticsFinanceSummaryResponse callSummary(Context context, long storeId) {
        @SuppressWarnings("unchecked")
        StreamObserver<GetSellerLogisticsFinanceSummaryResponse> observer = mock(StreamObserver.class);
        ArgumentCaptor<GetSellerLogisticsFinanceSummaryResponse> captor =
                ArgumentCaptor.forClass(GetSellerLogisticsFinanceSummaryResponse.class);
        Context previous = context.attach();
        try {
            service().getSellerLogisticsFinanceSummary(
                    GetSellerLogisticsFinanceSummaryRequest.newBuilder().setStoreId(storeId).build(),
                    observer);
        } finally {
            context.detach(previous);
        }
        verify(observer).onNext(captor.capture());
        return captor.getValue();
    }

    // =====================================================================
    // 授權軸
    // =====================================================================

    @Nested
    @DisplayName("授權")
    class Authorization {

        @Test
        @DisplayName("賣家面：store_id 未帶（Gateway 契約違反）→ 401，且不碰查詢層")
        void sellerWithoutStoreIdIsUnauthenticated() {
            GetSellerLogisticsFinanceSummaryResponse r = callSummary(ctx("2"), 0L);

            assertEquals(GrpcResponseCode.UNAUTHORIZED.code(), r.getCode(),
                    "🔴 必須 fail loud —— 回 code=0 ＋ 空 data 看起來就像「這家店沒有物流費」");
            verify(queryService, never()).sellerSummary(anyLong(), any(), any());
        }

        @Test
        @DisplayName("賣家面：買家（code 1）呼叫 → 403")
        void buyerCannotCallSellerFacingRpc() {
            GetSellerLogisticsFinanceSummaryResponse r = callSummary(ctx("1"), CALLER_STORE);
            assertEquals(GrpcResponseCode.PERMISSION_DENIED.code(), r.getCode());
            verify(queryService, never()).sellerSummary(anyLong(), any(), any());
        }

        @Test
        @DisplayName("賣家面：ADMIN（code 4）也不得走賣家 RPC（角色分離，不是「權限比較大就都能用」）")
        void adminCannotCallSellerFacingRpc() {
            GetSellerLogisticsFinanceSummaryResponse r = callSummary(ctx("4"), CALLER_STORE);
            assertEquals(GrpcResponseCode.PERMISSION_DENIED.code(), r.getCode());
            verify(queryService, never()).sellerSummary(anyLong(), any(), any());
        }

        @Test
        @DisplayName("賣家面：SELLER_AND_BUYER（code 3）可用（涵蓋 SELLER）")
        void dualRoleSellerIsAllowed() {
            when(queryService.sellerSummary(eq(CALLER_STORE), any(), any()))
                    .thenReturn(summary(300L, 120L));
            assertEquals(GrpcResponseCode.SUCCESS.code(), callSummary(ctx("3"), CALLER_STORE).getCode());
        }

        @Test
        @DisplayName("Admin 面：賣家（code 2）呼叫 → PERMISSION_DENIED，且不碰查詢層")
        void sellerCannotCallAdminRpc() {
            // ⚠️ Admin 面沿用既有 admin RPC 的慣例（PaymentGrpcService.listPaymentTransactionsForAdmin）：
            //    catch 直接交給 GrpcErrorHandler ⇒ 走 onError 而不是把碼塞進 envelope。
            //    賣家面則走 onNext ＋ envelope code（比照 getSellerPaymentHistory）。
            //    兩者不同是既有慣例，不是這一波的不一致——斷言因此各自對準各自的出口。
            @SuppressWarnings("unchecked")
            StreamObserver<ListLogisticsReceivablesForAdminResponse> observer = mock(StreamObserver.class);
            ArgumentCaptor<Throwable> captor = ArgumentCaptor.forClass(Throwable.class);
            Context c = ctx("2");
            Context previous = c.attach();
            try {
                service().listLogisticsReceivablesForAdmin(
                        ListLogisticsReceivablesForAdminRequest.newBuilder().build(), observer);
            } finally {
                c.detach(previous);
            }
            verify(observer, never()).onNext(any());
            verify(observer).onError(captor.capture());
            assertTrue(captor.getValue() instanceof StatusRuntimeException,
                    "應以 gRPC Status 拒絕，而不是包成一般例外（那會變成 500）");
            assertEquals(Status.Code.PERMISSION_DENIED,
                    ((StatusRuntimeException) captor.getValue()).getStatus().getCode());
            verify(queryService, never()).adminReceivables(any(), any(), any(), any(), any(),
                    any(), any(), org.mockito.ArgumentMatchers.anyBoolean(), anyLong(), anyInt(), anyInt());
        }
    }

    // =====================================================================
    // wire 契約
    // =====================================================================

    @Nested
    @DisplayName("wire 契約")
    class Wire {

        @Test
        @DisplayName("🔴 應收餘額與貸記餘額在 wire 上是兩個欄位，且值不被合成淨額")
        void receivableAndCreditAreTwoWireFields() {
            Descriptor d = SellerLogisticsFinanceSummary.getDescriptor();
            assertNotNull(d.findFieldByName("receivable_balance"),
                    "🔴 賣家欠平台的餘額欄位不見了 —— 兩個科目被合併成淨額了（ADR-008 §1）");
            assertNotNull(d.findFieldByName("seller_credit"),
                    "🔴 平台欠賣家的貸記欄位不見了 —— 兩個科目被合併成淨額了（ADR-008 §1）");

            when(queryService.sellerSummary(eq(CALLER_STORE), any(), any()))
                    .thenReturn(summary(300L, 120L));
            SellerLogisticsFinanceSummary s = callSummary(ctx("2"), CALLER_STORE).getData();

            assertEquals("300", s.getReceivableBalance());
            assertEquals("120", s.getSellerCredit());
            assertFalse("180".equals(s.getReceivableBalance()) || "180".equals(s.getSellerCredit()),
                    "🔴 兩個科目在 wire 上被合成淨額了");
        }

        @Test
        @DisplayName("🔴 Seller Subsidy（維度 6/11）永遠不設值：本服務不是它的來源")
        void sellerSubsidyIsNeverSet() {
            Descriptor d = SellerLogisticsFinanceSummary.getDescriptor();
            assertNotNull(d.findFieldByName("seller_subsidy"),
                    "欄位本身必須在契約裡（缺口要顯性），只是永遠不設值");
            assertNotNull(d.findFieldByName("platform_subsidy"),
                    "🔴 平台補貼與賣家補貼不得合併成一個「補貼」（W4-4）");

            when(queryService.sellerSummary(eq(CALLER_STORE), any(), any()))
                    .thenReturn(summary(300L, 120L));
            SellerLogisticsFinanceSummary s = callSummary(ctx("2"), CALLER_STORE).getData();

            assertFalse(s.hasSellerSubsidy(),
                    "🔴 填 0 會讓「payment 不是這個維度的來源」長得跟「賣家真的沒補貼」一樣");
            assertEquals("20", s.getPlatformSubsidy());
        }

        @Test
        @DisplayName("十一個維度在 wire 上各自有欄位（少一個就是漏交付）")
        void allElevenDimensionsExist() {
            Descriptor d = SellerLogisticsFinanceSummary.getDescriptor();
            // 🔴 硬寫字面值清單，不從 descriptor 反推——反推就是自己跟自己對。
            for (String f : List.of(
                    "estimated_provider_cost",    // 1/11
                    "actual_provider_cost",       // 2/11
                    "seller_logistics_charge",    // 3/11
                    "payuni_collection_fee",      // 4/11
                    "platform_subsidy",           // 5/11
                    "seller_subsidy",             // 6/11
                    "responsibility_adjustment",  // 7/11
                    "reconciliation_adjustment",  // 8/11
                    "collection_status_counts",   // 9/11
                    "collection_residual",        // 10/11
                    "seller_credit")) {           // 11/11
                assertNotNull(d.findFieldByName(f), "W6-3 的維度欄位缺了：" + f);
            }
        }

        @Test
        @DisplayName("🔴 NULL 不是 0：尚未對帳／尚未查詢的欄位一律維持未設")
        void nullIsNotZeroOnTheWire() {
            LogisticsReceivableEntity r = new LogisticsReceivableEntity();
            r.setId(11L);
            r.setStoreId(CALLER_STORE);
            r.setStatus("SUBMITTED");
            r.setEstimatedCost(100L);
            r.setPlatformSubsidy(0L);
            r.setTargetNet(100L);
            r.setCarriedResidual(0L);
            r.setActualCost(null);            // 尚未對帳
            r.setCreatedAt(NOW.minusDays(2));
            LogisticsCollectionAttemptEntity a = new LogisticsCollectionAttemptEntity();
            a.setId(21L);
            a.setReceivableId(11L);
            a.setGrossAmount(101L);
            a.setActualFee(null);             // 尚未查詢
            a.setResidual(null);

            when(queryService.sellerReceivables(eq(CALLER_STORE), any(), any(), any(), anyInt(), anyInt()))
                    .thenReturn(Paged.of(List.of(new SellerReceivableView(r, a, 0L, 0L)), 1L, 1, 20));
            when(queryService.agingDaysOf(any())).thenReturn(2L);

            @SuppressWarnings("unchecked")
            StreamObserver<ListSellerLogisticsReceivablesResponse> observer = mock(StreamObserver.class);
            ArgumentCaptor<ListSellerLogisticsReceivablesResponse> captor =
                    ArgumentCaptor.forClass(ListSellerLogisticsReceivablesResponse.class);
            Context c = ctx("2");
            Context previous = c.attach();
            try {
                service().listSellerLogisticsReceivables(
                        ListSellerLogisticsReceivablesRequest.newBuilder()
                                .setStoreId(CALLER_STORE).build(), observer);
            } finally {
                c.detach(previous);
            }
            verify(observer).onNext(captor.capture());
            SellerLogisticsReceivableRecord row = captor.getValue().getData().getRecords(0);

            assertFalse(row.hasActualProviderCost(), "🔴 尚未對帳 ≠ 成本是 0");
            assertFalse(row.hasPayuniCollectionFee(), "🔴 尚未查詢 ≠ 手續費是 0");
            assertFalse(row.hasCollectionResidual(), "🔴 尚未查詢 ≠ 正好收足");
            // 有值的欄位照常是數字字串（10 檔 §4）
            assertEquals("100", row.getEstimatedProviderCost());
            assertEquals("100", row.getSellerLogisticsCharge());
            assertEquals("2", row.getAgingDays());
        }

        @Test
        @DisplayName("狀態帶 CodeMeta 的中文 label（前端不必自己拼）")
        void statusLabelComesFromCodeMeta() {
            LogisticsReceivableEntity r = new LogisticsReceivableEntity();
            r.setId(12L);
            r.setStoreId(CALLER_STORE);
            r.setStatus("UNCONFIRMED");
            r.setEstimatedCost(1L);
            r.setPlatformSubsidy(0L);
            r.setTargetNet(1L);
            r.setCarriedResidual(0L);
            r.setCreatedAt(NOW);

            when(queryService.sellerReceivables(eq(CALLER_STORE), any(), any(), any(), anyInt(), anyInt()))
                    .thenReturn(Paged.of(List.of(new SellerReceivableView(r, null, 0L, 0L)), 1L, 1, 20));

            @SuppressWarnings("unchecked")
            StreamObserver<ListSellerLogisticsReceivablesResponse> observer = mock(StreamObserver.class);
            ArgumentCaptor<ListSellerLogisticsReceivablesResponse> captor =
                    ArgumentCaptor.forClass(ListSellerLogisticsReceivablesResponse.class);
            Context c = ctx("2");
            Context previous = c.attach();
            try {
                service().listSellerLogisticsReceivables(
                        ListSellerLogisticsReceivablesRequest.newBuilder()
                                .setStoreId(CALLER_STORE).build(), observer);
            } finally {
                c.detach(previous);
            }
            verify(observer).onNext(captor.capture());
            SellerLogisticsReceivableRecord row = captor.getValue().getData().getRecords(0);

            assertEquals("UNCONFIRMED", row.getCollectionStatus());
            assertEquals("收款結果未確認", row.getCollectionStatusLabel(),
                    "label 必須取自 CodeMeta，不得在 gateway 或前端另拼一份");
        }

        @Test
        @DisplayName("無效的 status 篩選 → 400，不得靜默回空清單")
        void invalidStatusFilterIsRejected() {
            @SuppressWarnings("unchecked")
            StreamObserver<ListSellerLogisticsReceivablesResponse> observer = mock(StreamObserver.class);
            ArgumentCaptor<ListSellerLogisticsReceivablesResponse> captor =
                    ArgumentCaptor.forClass(ListSellerLogisticsReceivablesResponse.class);
            Context c = ctx("2");
            Context previous = c.attach();
            try {
                service().listSellerLogisticsReceivables(
                        ListSellerLogisticsReceivablesRequest.newBuilder()
                                .setStoreId(CALLER_STORE).setStatus("PENDIGN").build(), observer);
            } finally {
                c.detach(previous);
            }
            verify(observer).onNext(captor.capture());
            assertEquals(GrpcResponseCode.INVALID_PARAM.code(), captor.getValue().getCode(),
                    "打錯字與『這個狀態沒有資料』必須分得開");
            assertTrue(captor.getValue().getMessage().contains("PENDIGN"));
        }
    }
}
