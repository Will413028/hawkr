package io.github.will413028.hawkr.order.worker;

import io.github.will413028.hawkr.order.checkout.client.CheckoutCouponGrpcClient;
import io.github.will413028.hawkr.order.entity.CartCleanupRetryTaskEntity;
import io.github.will413028.hawkr.order.entity.CouponConsumptionTaskEntity;
import io.github.will413028.hawkr.order.grpc.client.CartGrpcClient;
import io.github.will413028.hawkr.order.repository.CartCleanupRetryTaskRepository;
import io.github.will413028.hawkr.order.repository.CouponConsumptionTaskRepository;
import org.junit.jupiter.api.DisplayName;
import org.junit.jupiter.api.Test;
import org.mockito.ArgumentCaptor;
import org.mockito.InOrder;

import java.time.LocalDateTime;
import java.util.List;

import static org.junit.jupiter.api.Assertions.assertTrue;
import static org.mockito.ArgumentMatchers.any;
import static org.mockito.ArgumentMatchers.anyInt;
import static org.mockito.ArgumentMatchers.anyLong;
import static org.mockito.ArgumentMatchers.anyString;
import static org.mockito.ArgumentMatchers.eq;
import static org.mockito.Mockito.inOrder;
import static org.mockito.Mockito.mock;
import static org.mockito.Mockito.never;
import static org.mockito.Mockito.verify;
import static org.mockito.Mockito.verifyNoInteractions;
import static org.mockito.Mockito.when;

/**
 * issue #2294 C3 / C5：{@code CartCleanupRetryWorker} 與 {@code CouponConsumptionWorker}
 * 的 claim 由「{@code SET status=PROCESSING} 永久旗標」改為
 * 「<b>lease on {@code next_retry_time}</b>」。
 *
 * <p>兩支原本<b>零測試</b>。C5 更曾被 #2294 §1-4 的「不納入」清單明文判為「已正確」——
 * 理由是「閘門在副作用之前」＋「副作用是 gRPC 不是 publish」，兩者都不成立
 * （閘門位置正確 ≠ 搶佔語意正確；C2/C3 的副作用同樣是 gRPC 卻被納入）。
 */
class TaskWorkerClaimLeaseTest {

    // ==================== C3 CartCleanupRetryWorker ====================

    private static CartCleanupRetryTaskEntity cartTask() {
        CartCleanupRetryTaskEntity t = new CartCleanupRetryTaskEntity();
        t.setTaskId(1001L);
        t.setBuyerId(3001L);
        t.setCheckoutId("CK1");
        t.setRetryCount(0);
        t.setMaxRetryCount(3);
        return t;
    }

    /**
     * ⚠️ 鑑別點：Mockito 對 {@code int} 預設回 <b>0</b>，
     * 「忘記 stub claim」與「claim 真的回 0」長得一樣。沒有本測試，
     * 把 claim 閘門整個拿掉不會有任何測試轉紅。
     */
    @Test
    @DisplayName("C3 claim 回 0 → 不打 cart gRPC、不寫任何狀態")
    void cart_claimLost_doesNothing() {
        CartCleanupRetryTaskRepository repo = mock(CartCleanupRetryTaskRepository.class);
        CartGrpcClient cart = mock(CartGrpcClient.class);
        when(repo.findRetryable(anyInt())).thenReturn(List.of(cartTask()));
        when(repo.claimTask(anyLong(), any())).thenReturn(0);

        new CartCleanupRetryWorker(repo, cart).process();

        verifyNoInteractions(cart);
        verify(repo, never()).markDone(anyLong());
        verify(repo, never()).updateFailed(anyLong(), anyString(), anyString(), anyInt());
    }

    @Test
    @DisplayName("C3 claim 回 1 → 才打 cart gRPC，且 claim 先於 gRPC；claim 帶未來租約（lease）")
    void cart_claimWon_callsGrpcWithFutureLease() {
        CartCleanupRetryTaskRepository repo = mock(CartCleanupRetryTaskRepository.class);
        CartGrpcClient cart = mock(CartGrpcClient.class);
        when(repo.findRetryable(anyInt())).thenReturn(List.of(cartTask()));
        when(repo.claimTask(anyLong(), any())).thenReturn(1);

        LocalDateTime before = io.github.will413028.hawkr.common.utils.DateTimeUtils.nowUtc();
        new CartCleanupRetryWorker(repo, cart).process();

        InOrder order = inOrder(repo, cart);
        ArgumentCaptor<LocalDateTime> lease = ArgumentCaptor.forClass(LocalDateTime.class);
        order.verify(repo).claimTask(eq(1001L), lease.capture());
        order.verify(cart).removeItemsByCheckoutId(3001L, "CK1");
        assertTrue(!lease.getValue().isBefore(before.plusSeconds(CartCleanupRetryWorker.CLAIM_LEASE_SEC)),
                "claim 必須寫 now + CLAIM_LEASE_SEC 的未來時間（lease），不是永久旗標");
        verify(repo).markDone(1001L);
    }

    // ==================== C5 CouponConsumptionWorker ====================

    private static CouponConsumptionTaskEntity couponTask() {
        CouponConsumptionTaskEntity t = new CouponConsumptionTaskEntity();
        t.setTaskId(2002L);
        t.setBuyerId(3001L);
        t.setCouponInstanceId("CI1");
        t.setOrderNo("ORD1");
        t.setRetryCount(0);
        t.setMaxRetryCount(3);
        return t;
    }

    @Test
    @DisplayName("C5 claim 回 0 → 不打 coupon gRPC、不寫任何狀態")
    void coupon_claimLost_doesNothing() {
        CouponConsumptionTaskRepository repo = mock(CouponConsumptionTaskRepository.class);
        CheckoutCouponGrpcClient coupon = mock(CheckoutCouponGrpcClient.class);
        when(repo.findRetryable(anyInt())).thenReturn(List.of(couponTask()));
        when(repo.claimTask(anyLong(), any())).thenReturn(0);

        new CouponConsumptionWorker(repo, coupon).process();

        verifyNoInteractions(coupon);
        verify(repo, never()).markDone(anyLong());
        verify(repo, never()).updateFailed(anyLong(), anyString(), anyString(), anyInt());
    }

    @Test
    @DisplayName("C5 claim 回 1 → 才打 coupon gRPC，且 claim 先於 gRPC；claim 帶未來租約（lease）")
    void coupon_claimWon_callsGrpcWithFutureLease() {
        CouponConsumptionTaskRepository repo = mock(CouponConsumptionTaskRepository.class);
        CheckoutCouponGrpcClient coupon = mock(CheckoutCouponGrpcClient.class);
        when(repo.findRetryable(anyInt())).thenReturn(List.of(couponTask()));
        when(repo.claimTask(anyLong(), any())).thenReturn(1);

        LocalDateTime before = io.github.will413028.hawkr.common.utils.DateTimeUtils.nowUtc();
        new CouponConsumptionWorker(repo, coupon).process();

        InOrder order = inOrder(repo, coupon);
        ArgumentCaptor<LocalDateTime> lease = ArgumentCaptor.forClass(LocalDateTime.class);
        order.verify(repo).claimTask(eq(2002L), lease.capture());
        order.verify(coupon).consumeCoupon("CI1", 3001L, "ORD1");
        assertTrue(!lease.getValue().isBefore(before.plusSeconds(CouponConsumptionWorker.CLAIM_LEASE_SEC)),
                "claim 必須寫 now + CLAIM_LEASE_SEC 的未來時間（lease），不是永久旗標");
        verify(repo).markDone(2002L);
    }
}
