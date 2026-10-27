package io.github.will413028.hawkr.coupon.scheduler;

import io.github.will413028.hawkr.common.grpc.PrincipalResolution;
import io.github.will413028.hawkr.coupon.entity.projection.CouponExpiringRef;
import io.github.will413028.hawkr.coupon.event.CouponEventPublisher;
import io.github.will413028.hawkr.coupon.event.CouponExpiringEvent;
import io.github.will413028.hawkr.coupon.grpc.client.BuyerGrpcClient;
import io.github.will413028.hawkr.coupon.mapper.CouponInstanceMapper;
import org.junit.jupiter.api.DisplayName;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.api.extension.ExtendWith;
import org.mockito.ArgumentCaptor;
import org.mockito.Mock;
import org.mockito.junit.jupiter.MockitoExtension;
import org.mockito.junit.jupiter.MockitoSettings;
import org.mockito.quality.Strictness;
import org.springframework.test.util.ReflectionTestUtils;

import java.time.LocalDateTime;
import java.util.List;
import java.util.Map;

import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.junit.jupiter.api.Assertions.assertNotEquals;
import static org.mockito.ArgumentMatchers.any;
import static org.mockito.ArgumentMatchers.anyInt;
import static org.mockito.ArgumentMatchers.eq;
import static org.mockito.Mockito.verify;
import static org.mockito.Mockito.when;

/**
 * buyer_id rekey Phase 5 — 鏡像 order-service Phase 1 對 notify_buyer_account_id 的
 * discriminating test 精神：buyerId（業務值 buyer_id）與 notifyBuyerAccountId
 * （reverse-resolve 後的登入 account_id）刻意設為不同值，證明發布的事件帶的是「解析後」
 * 的值，不是原始 owner_buyer_id 直接複製。
 */
@ExtendWith(MockitoExtension.class)
@MockitoSettings(strictness = Strictness.LENIENT)
class CouponExpiringWarningJobTest {

    @Mock private CouponInstanceMapper instanceMapper;
    @Mock private CouponEventPublisher publisher;
    @Mock private BuyerGrpcClient buyerGrpcClient;

    private CouponExpiringWarningJob newJob() {
        CouponExpiringWarningJob job = new CouponExpiringWarningJob(instanceMapper, publisher, buyerGrpcClient);
        ReflectionTestUtils.setField(job, "enabled", true);
        ReflectionTestUtils.setField(job, "warningDays", 1);
        ReflectionTestUtils.setField(job, "batchSize", 500);
        return job;
    }

    @Test
    @DisplayName("execute() 發布事件帶 reverse-resolve 後的 notifyBuyerAccountId（分岔情境，不等於原始 buyer_id）")
    void execute_publishesResolvedNotifyBuyerAccountId_whenDiverged() {
        long ownerBuyerId = 3001L;
        long resolvedAccountId = 55L;

        CouponExpiringRef ref = new CouponExpiringRef(9001L, ownerBuyerId, "週末優惠", LocalDateTime.now().plusHours(12));
        when(instanceMapper.scanExpiringUnused(any(), any(), eq(0L), anyInt()))
                .thenReturn(List.of(ref));
        when(instanceMapper.scanExpiringUnused(any(), any(), eq(9001L), anyInt()))
                .thenReturn(List.of());
        when(buyerGrpcClient.resolvePrincipals(List.of(ownerBuyerId)))
                .thenReturn(PrincipalResolution.allResolved(Map.of(ownerBuyerId, resolvedAccountId)));

        newJob().execute();

        ArgumentCaptor<CouponExpiringEvent> captor = ArgumentCaptor.forClass(CouponExpiringEvent.class);
        verify(publisher).publishCouponExpiring(captor.capture());
        CouponExpiringEvent published = captor.getValue();
        assertEquals(String.valueOf(ownerBuyerId), published.buyerId());
        assertEquals(String.valueOf(resolvedAccountId), published.notifyBuyerAccountId());
        assertNotEquals(published.buyerId(), published.notifyBuyerAccountId());
    }

    @Test
    @DisplayName("execute()：BuyerGrpcClient 拋例外時該批次 notifyBuyerAccountId 帶 0（不 fallback 回原始 buyer_id）")
    void execute_publishesZeroNotifyBuyerAccountId_whenResolveThrows() {
        long ownerBuyerId = 3001L;

        CouponExpiringRef ref = new CouponExpiringRef(9001L, ownerBuyerId, "週末優惠", LocalDateTime.now().plusHours(12));
        when(instanceMapper.scanExpiringUnused(any(), any(), eq(0L), anyInt()))
                .thenReturn(List.of(ref));
        when(instanceMapper.scanExpiringUnused(any(), any(), eq(9001L), anyInt()))
                .thenReturn(List.of());
        when(buyerGrpcClient.resolvePrincipals(List.of(ownerBuyerId)))
                .thenThrow(new RuntimeException("buyer-service unavailable"));

        newJob().execute();

        ArgumentCaptor<CouponExpiringEvent> captor = ArgumentCaptor.forClass(CouponExpiringEvent.class);
        verify(publisher).publishCouponExpiring(captor.capture());
        assertEquals("0", captor.getValue().notifyBuyerAccountId());
    }

    @Test
    @DisplayName("execute()：buyer_id 查無存活買家（unresolved_ids）時 notifyBuyerAccountId 帶 0，不回吐 buyer_id")
    void execute_publishesZeroNotifyBuyerAccountId_whenBuyerIsUnresolved() {
        long ownerBuyerId = 3001L;
        CouponExpiringRef ref = new CouponExpiringRef(9001L, ownerBuyerId, "週末優惠", LocalDateTime.now().plusHours(12));
        when(instanceMapper.scanExpiringUnused(any(), any(), eq(0L), anyInt())).thenReturn(List.of(ref));
        when(instanceMapper.scanExpiringUnused(any(), any(), eq(9001L), anyInt())).thenReturn(List.of());
        when(buyerGrpcClient.resolvePrincipals(List.of(ownerBuyerId)))
                .thenReturn(PrincipalResolution.of(Map.of(), List.of(ownerBuyerId)));

        newJob().execute();

        ArgumentCaptor<CouponExpiringEvent> captor = ArgumentCaptor.forClass(CouponExpiringEvent.class);
        verify(publisher).publishCouponExpiring(captor.capture());
        assertEquals("0", captor.getValue().notifyBuyerAccountId());
    }
}
