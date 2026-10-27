package io.github.will413028.hawkr.order.grpc;

import io.github.will413028.hawkr.common.config.MediaUrlProperties;
import io.github.will413028.hawkr.common.grpc.GrpcAuthContext;
import io.github.will413028.hawkr.common.utils.TsidUtil;
import io.github.will413028.hawkr.common.utils.code.GrpcResponseCode;
import io.github.will413028.hawkr.common.utils.code.InvoiceCarrierSelectionMode;
import io.github.will413028.hawkr.common.utils.code.InvoiceDocumentKind;
import io.github.will413028.hawkr.order.checkout.CheckoutService;
import io.github.will413028.hawkr.order.checkout.client.CheckoutCouponGrpcClient;
import io.github.will413028.hawkr.order.checkout.client.SellerGrpcClient;
import io.github.will413028.hawkr.order.entity.OrderInvoiceSnapshotEntity;
import io.github.will413028.hawkr.order.grpc.client.BuyerGrpcClient;
import io.github.will413028.hawkr.order.grpc.client.MediaGrpcClient;
import io.github.will413028.hawkr.order.grpc.client.PaymentGrpcClient;
import io.github.will413028.hawkr.order.grpc.client.ShippingGrpcClient;
import io.github.will413028.hawkr.order.mq.OrderEventPublisher;
import io.github.will413028.hawkr.order.repository.GrantPaymentTaskRepository;
import io.github.will413028.hawkr.order.repository.OrderInvoiceSnapshotRepository;
import io.github.will413028.hawkr.order.repository.OrderItemRepository;
import io.github.will413028.hawkr.order.repository.OrderRepository;
import io.github.will413028.hawkr.order.service.OrderService;
import io.grpc.Context;
import io.grpc.Status;
import io.grpc.StatusRuntimeException;
import io.grpc.stub.StreamObserver;
import org.junit.jupiter.api.BeforeAll;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.DisplayName;
import org.junit.jupiter.api.Test;
import org.mockito.ArgumentCaptor;
import org.springframework.dao.DuplicateKeyException;

import java.util.concurrent.atomic.AtomicReference;

import static org.assertj.core.api.Assertions.assertThat;
import static org.mockito.ArgumentMatchers.any;
import static org.mockito.Mockito.mock;
import static org.mockito.Mockito.never;
import static org.mockito.Mockito.verify;
import static org.mockito.Mockito.when;

/**
 * {@code ReviseOrderInvoiceSnapshotInternal}（58 號 §6.2.5、§7.3.5 R03）。
 *
 * <p>四件事在守，每一件漏掉都<b>不會報錯</b>：
 * <ol>
 *   <li><b>append-only</b>：成功時 INSERT v(n+1)，絕不 UPDATE 既有列。原地改會讓
 *       「下單當下宣告了什麼」這個事實消失，而那是爭議時唯一的憑據。</li>
 *   <li><b>冪等</b>：同 {@code idempotency_key} 重送回同一個版本、不新增列。沒有它，
 *       invoice-service 的 gRPC 重試會每次遞增版本，而版本號是開票冪等鍵的組成之一
 *       ⇒ 重複開票的風險從這裡進來。</li>
 *   <li><b>樂觀鎖</b>：兩筆並行更正時後寫的靜默覆蓋前者，而兩筆都會顯示成功。</li>
 *   <li><b>{@code notify_email} 不是可更正欄位</b>（§6.2.5 定版 7）：它不在發票上，
 *       讓它遞增內容軸會使「同內容重開」與「內容真的變了」在冪等鍵上無法區分。</li>
 * </ol>
 */
class OrderInvoiceSnapshotReviseTest {

    private static final String ORDER_NO = "O20260831001";
    private static final String IDEM_KEY = "correction-7788";
    private static final String CARRIED_EMAIL = "buyer@example.com";

    private OrderInvoiceSnapshotRepository invoiceSnapshotRepository;

    /**
     * handler 內用 {@code TsidUtil.generate()} 產新列的主鍵；沒有 Spring context 就得自己 init
     * （node-id base 300 ＝ order-service，見 {@code 10-engineering-contracts.md} §7.2）。
     *
     * <p>{@code init} 是一次性的，而同一個 JVM 會跑多個測試類 ⇒ 已初始化就當成成功。
     * 這裡只吞「已經初始化」這一種，其餘照拋——吞掉全部會讓真的設定錯誤變成無聲。
     */
    @BeforeAll
    static void initTsid() {
        try {
            TsidUtil.init(300, 0);
        } catch (IllegalStateException alreadyInitialized) {
            // 同 JVM 的其他測試類先跑過了，維持原設定即可
        }
    }

    @BeforeEach
    void setUp() {
        invoiceSnapshotRepository = mock(OrderInvoiceSnapshotRepository.class);
    }

    private OrderGrpcService buildService() {
        return new OrderGrpcService(
                mock(OrderService.class), mock(CheckoutService.class), mock(CheckoutCouponGrpcClient.class),
                mock(OrderRepository.class), mock(OrderItemRepository.class), mock(ShippingGrpcClient.class),
                mock(PaymentGrpcClient.class), mock(SellerGrpcClient.class), mock(BuyerGrpcClient.class),
                mock(MediaGrpcClient.class), mock(MediaUrlProperties.class),
                mock(GrantPaymentTaskRepository.class), invoiceSnapshotRepository,
                mock(OrderEventPublisher.class), "");
    }

    private static OrderInvoiceSnapshotEntity v1() {
        OrderInvoiceSnapshotEntity s = new OrderInvoiceSnapshotEntity();
        s.setId(2649605557L);
        s.setOrderNo(ORDER_NO);
        s.setSnapshotVersion(1);
        s.setInvoiceKind(InvoiceDocumentKind.B2C.value());
        s.setCarrierSelectionMode(InvoiceCarrierSelectionMode.NONE.value());
        s.setNotifyEmail(CARRIED_EMAIL);
        return s;
    }

    private static ReviseOrderInvoiceSnapshotInternalRequest.Builder req(String expectedVersion) {
        return ReviseOrderInvoiceSnapshotInternalRequest.newBuilder()
                .setOrderNo(ORDER_NO)
                .setExpectedSnapshotVersion(expectedVersion)
                .setInvoiceKind(InvoiceDocumentKind.B2B.value())
                .setCarrierSelectionMode(InvoiceCarrierSelectionMode.NONE.value())
                .setBuyerTaxId("12345678")
                .setBuyerTitle("範例生鮮股份有限公司")
                .setIdempotencyKey(IDEM_KEY);
    }

    private record Result(ReviseOrderInvoiceSnapshotInternalResponse response, Throwable error) {}

    private Result call(OrderGrpcService svc, ReviseOrderInvoiceSnapshotInternalRequest request, boolean internal) {
        AtomicReference<ReviseOrderInvoiceSnapshotInternalResponse> resp = new AtomicReference<>();
        AtomicReference<Throwable> err = new AtomicReference<>();
        StreamObserver<ReviseOrderInvoiceSnapshotInternalResponse> observer = new StreamObserver<>() {
            @Override public void onNext(ReviseOrderInvoiceSnapshotInternalResponse value) { resp.set(value); }
            @Override public void onError(Throwable t) { err.set(t); }
            @Override public void onCompleted() { }
        };
        Context ctx = Context.current().withValue(GrpcAuthContext.INTERNAL_CALL, internal);
        ctx.run(() -> {
            try {
                svc.reviseOrderInvoiceSnapshotInternal(request, observer);
            } catch (Throwable t) {
                err.set(t);
            }
        });
        return new Result(resp.get(), err.get());
    }

    @Test
    @DisplayName("非內部呼叫 → PERMISSION_DENIED（少了這道閘門，任何持有 JWT 的人都能改別人訂單的發票抬頭）")
    void rejectsNonInternalCallers() {
        Result r = call(buildService(), req("1").build(), false);

        assertThat(r.response()).isNull();
        assertThat(Status.fromThrowable(r.error()).getCode()).isEqualTo(Status.Code.PERMISSION_DENIED);
        verify(invoiceSnapshotRepository, never()).insert(any());
    }

    @Test
    @DisplayName("成功 → INSERT v2（不是 UPDATE v1），且 notify_email 沿用前一版")
    void appendsNewVersionAndCarriesNotifyEmail() {
        when(invoiceSnapshotRepository.findByRevisionIdempotencyKey(ORDER_NO, IDEM_KEY)).thenReturn(null);
        when(invoiceSnapshotRepository.findLatestByOrderNo(ORDER_NO)).thenReturn(v1());

        Result r = call(buildService(), req("1").build(), true);

        assertThat(r.response().getCode()).isZero();
        assertThat(r.response().getSnapshotVersion())
                .as("回傳的新版本；invoice-service 只能從這裡複製（I-50）").isEqualTo("2");

        ArgumentCaptor<OrderInvoiceSnapshotEntity> captor =
                ArgumentCaptor.forClass(OrderInvoiceSnapshotEntity.class);
        verify(invoiceSnapshotRepository).insert(captor.capture());
        OrderInvoiceSnapshotEntity written = captor.getValue();

        assertThat(written.getSnapshotVersion()).isEqualTo(2);
        assertThat(written.getOrderNo()).isEqualTo(ORDER_NO);
        assertThat(written.getId()).as("TSID 必須是新的一列，不是沿用 v1 的 id").isNotEqualTo(2649605557L);
        assertThat(written.getInvoiceKind()).isEqualTo(InvoiceDocumentKind.B2B.value());
        assertThat(written.getBuyerTaxId()).isEqualTo("12345678");
        assertThat(written.getRevisionIdempotencyKey())
                .as("沒有寫入冪等鍵，下一次重送就會再 INSERT 一列").isEqualTo(IDEM_KEY);
        assertThat(written.getNotifyEmail())
                .as("§6.2.5 定版 7：通知信箱不是內容軸，原樣沿用前一版").isEqualTo(CARRIED_EMAIL);
    }

    @Test
    @DisplayName("冪等重送 → 回同一個版本且完全不 INSERT（重試每次遞增版本＝重複開票的入口）")
    void idempotentReplayReturnsSameVersionWithoutInsert() {
        OrderInvoiceSnapshotEntity existing = v1();
        existing.setSnapshotVersion(2);
        existing.setRevisionIdempotencyKey(IDEM_KEY);
        when(invoiceSnapshotRepository.findByRevisionIdempotencyKey(ORDER_NO, IDEM_KEY)).thenReturn(existing);

        Result r = call(buildService(), req("1").build(), true);

        assertThat(r.response().getCode()).isZero();
        assertThat(r.response().getSnapshotVersion()).isEqualTo("2");
        verify(invoiceSnapshotRepository, never()).insert(any());
        verify(invoiceSnapshotRepository, never()).findLatestByOrderNo(any());
    }

    @Test
    @DisplayName("expected 版本與最新版不符 → 9308 且不寫入（否則後寫的靜默覆蓋前者，兩筆都顯示成功）")
    void versionMismatchReturnsConflict() {
        when(invoiceSnapshotRepository.findByRevisionIdempotencyKey(ORDER_NO, IDEM_KEY)).thenReturn(null);
        OrderInvoiceSnapshotEntity latest = v1();
        latest.setSnapshotVersion(3);
        when(invoiceSnapshotRepository.findLatestByOrderNo(ORDER_NO)).thenReturn(latest);

        Result r = call(buildService(), req("1").build(), true);

        assertThat(r.response().getCode())
                .isEqualTo(GrpcResponseCode.INVOICE_SNAPSHOT_VERSION_CONFLICT.code());
        assertThat(r.response().getSnapshotVersion()).isEmpty();
        verify(invoiceSnapshotRepository, never()).insert(any());
    }

    @Test
    @DisplayName("並行撞唯一鍵 → 9308（不得吞掉：吞掉就是「回報成功但什麼都沒寫」）")
    void duplicateKeyBecomesConflictNotSuccess() {
        when(invoiceSnapshotRepository.findByRevisionIdempotencyKey(ORDER_NO, IDEM_KEY)).thenReturn(null);
        when(invoiceSnapshotRepository.findLatestByOrderNo(ORDER_NO)).thenReturn(v1());
        when(invoiceSnapshotRepository.insert(any())).thenThrow(new DuplicateKeyException("uk_order_invoice_snapshots"));

        Result r = call(buildService(), req("1").build(), true);

        assertThat(r.response().getCode())
                .isEqualTo(GrpcResponseCode.INVOICE_SNAPSHOT_VERSION_CONFLICT.code());
        assertThat(r.response().getSnapshotVersion()).isEmpty();
    }

    @Test
    @DisplayName("查無快照 → NOT_FOUND（不得靜默補一列：從未做過發票選擇的舊訂單會被自動開票）")
    void missingSnapshotReturnsNotFound() {
        when(invoiceSnapshotRepository.findByRevisionIdempotencyKey(ORDER_NO, IDEM_KEY)).thenReturn(null);
        when(invoiceSnapshotRepository.findLatestByOrderNo(ORDER_NO)).thenReturn(null);

        Result r = call(buildService(), req("1").build(), true);

        assertThat(r.response().getCode()).isEqualTo(GrpcResponseCode.NOT_FOUND.code());
        verify(invoiceSnapshotRepository, never()).insert(any());
    }

    @Test
    @DisplayName("order_no 或冪等鍵為空 → INVALID_PARAM（空冪等鍵會讓每次重送都變成一次新的更正）")
    void blankIdempotencyKeyIsRejected() {
        Result r = call(buildService(), req("1").setIdempotencyKey("").build(), true);

        assertThat(r.response().getCode()).isEqualTo(GrpcResponseCode.INVALID_PARAM.code());
        verify(invoiceSnapshotRepository, never()).insert(any());
    }
}
