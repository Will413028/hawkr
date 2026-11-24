package io.github.will413028.hawkr.order.service;

import io.github.will413028.hawkr.order.checkout.client.CheckoutCouponGrpcClient;
import io.github.will413028.hawkr.order.repository.CouponConsumptionTaskRepository;
import org.junit.jupiter.api.DisplayName;
import org.junit.jupiter.api.Test;
import org.mockito.InOrder;

import static org.junit.jupiter.api.Assertions.assertDoesNotThrow;
import static org.junit.jupiter.api.Assertions.assertThrows;
import static org.mockito.ArgumentMatchers.anyInt;
import static org.mockito.ArgumentMatchers.anyLong;
import static org.mockito.ArgumentMatchers.anyString;
import static org.mockito.ArgumentMatchers.eq;
import static org.mockito.ArgumentMatchers.isNull;
import static org.mockito.Mockito.doThrow;
import static org.mockito.Mockito.inOrder;
import static org.mockito.Mockito.mock;
import static org.mockito.Mockito.never;
import static org.mockito.Mockito.verify;

/**
 * 釋券（D3）：任務在取消的 transaction 內寫入；提交後當場嘗試，成功才標完成，失敗不拋出、留給 worker 重試。
 */
class CouponReleaseServiceTest {

    private final CouponConsumptionTaskRepository repo = mock(CouponConsumptionTaskRepository.class);
    private final CheckoutCouponGrpcClient coupon = mock(CheckoutCouponGrpcClient.class);
    private final CouponReleaseService service = new CouponReleaseService(repo, coupon);

    @Test
    @DisplayName("enqueueInTx 寫入釋券任務（帶券 id），寫入失敗往外拋讓取消回滾")
    void enqueueInTxWritesTheTaskAndPropagatesFailure() {
        service.enqueueInTx("ORD-0", "V0");
        verify(repo).enqueueRelease(anyLong(), eq("ORD-0"), eq("V0"));

        doThrow(new RuntimeException("db down")).when(repo).enqueueRelease(anyLong(), eq("ORD-9"), anyString());
        assertThrows(RuntimeException.class, () -> service.enqueueInTx("ORD-9", "V9"));
    }

    @Test
    @DisplayName("當場釋放成功 → 呼叫 coupon（order_no 加券 id）、再標完成，不重寫任務")
    void successReleasesThenMarksDone() {
        service.release("ORD-1", "V1");

        InOrder order = inOrder(coupon, repo);
        order.verify(coupon).releaseCouponByOrderNo("ORD-1", "V1");
        order.verify(repo).markReleaseDone("ORD-1");
        verify(repo, never()).enqueueRelease(anyLong(), anyString(), anyString());
    }

    @Test
    @DisplayName("當場釋放失敗 → 不拋出、不標完成（任務留著給 worker 重試）")
    void failureLeavesTheTaskForTheWorker() {
        doThrow(new IllegalStateException("coupon down")).when(coupon).releaseCouponByOrderNo("ORD-2", "V2");

        assertDoesNotThrow(() -> service.release("ORD-2", "V2"));

        verify(repo, never()).markReleaseDone(anyString());
    }

    @Test
    @DisplayName("鎖券失敗補償 → 任務延後到 reserve deadline 之後，當場釋放成功也不標完成")
    void lateLockRecheckKeepsTheDelayedTaskPending() {
        service.releaseWithLateLockRecheck("ORD-3");

        verify(repo).enqueueReleaseAfter(anyLong(), eq("ORD-3"), isNull(), eq(CouponReleaseService.LATE_LOCK_RECHECK_DELAY_SEC));
        verify(coupon).releaseCouponByOrderNo("ORD-3", null);
        verify(repo, never()).markReleaseDone(anyString());
        org.junit.jupiter.api.Assertions.assertTrue(
                CouponReleaseService.LATE_LOCK_RECHECK_DELAY_SEC > CheckoutCouponGrpcClient.INTERNAL_READ_DEADLINE_SEC,
                "延後時間必須晚於 reserve 的 deadline");
    }

    @Test
    @DisplayName("鎖券失敗補償：寫任務失敗或當場釋放失敗都不拋出")
    void lateLockRecheckNeverThrows() {
        doThrow(new RuntimeException("db down")).when(repo).enqueueReleaseAfter(anyLong(), anyString(), isNull(), anyInt());
        doThrow(new IllegalStateException("coupon down")).when(coupon).releaseCouponByOrderNo("ORD-4", null);

        assertDoesNotThrow(() -> service.releaseWithLateLockRecheck("ORD-4"));

        verify(coupon).releaseCouponByOrderNo("ORD-4", null);
    }
}
