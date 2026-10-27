package io.github.will413028.hawkr.coupon.event;

import io.github.will413028.hawkr.common.grpc.PrincipalResolution;
import io.github.will413028.hawkr.coupon.grpc.client.BuyerGrpcClient;
import org.junit.jupiter.api.DisplayName;
import org.junit.jupiter.api.Test;
import org.mockito.ArgumentCaptor;
import org.springframework.amqp.rabbit.core.RabbitTemplate;

import java.util.List;
import java.util.Map;

import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.mockito.ArgumentMatchers.any;
import static org.mockito.ArgumentMatchers.anyString;
import static org.mockito.Mockito.mock;
import static org.mockito.Mockito.verify;
import static org.mockito.Mockito.when;

class CouponEventPublisherTest {

    @Test
    @DisplayName("coupon.issued：解析到存活買家時帶解析後的 account_id")
    void publishCouponIssued_carriesTheResolvedAccountId() {
        RabbitTemplate rabbit = mock(RabbitTemplate.class);
        BuyerGrpcClient buyers = mock(BuyerGrpcClient.class);
        when(buyers.resolvePrincipals(List.of(3001L))).thenReturn(PrincipalResolution.allResolved(Map.of(3001L, 55L)));

        new CouponEventPublisher(rabbit, buyers).publishCouponIssued(3001L, 1L, "券", null, "2026-12-31");

        assertEquals("55", String.valueOf(captured(rabbit).getNotifyBuyerAccountId()));
    }

    @Test
    @DisplayName("coupon.issued：buyer_id 查無存活買家（unresolved_ids）時 notifyBuyerAccountId 帶 0，不回吐 buyer_id")
    void publishCouponIssued_unresolvedBuyerIsNotEchoedAsAnAccount() {
        RabbitTemplate rabbit = mock(RabbitTemplate.class);
        BuyerGrpcClient buyers = mock(BuyerGrpcClient.class);
        // 舊 buyer-service 會回填 principals；unresolved_ids 才是真相。
        when(buyers.resolvePrincipals(List.of(3001L))).thenReturn(PrincipalResolution.of(Map.of(3001L, 3001L), List.of(3001L)));

        new CouponEventPublisher(rabbit, buyers).publishCouponIssued(3001L, 1L, "券", null, "2026-12-31");

        assertEquals("0", String.valueOf(captured(rabbit).getNotifyBuyerAccountId()));
    }

    private static CouponIssuedEvent captured(RabbitTemplate rabbit) {
        ArgumentCaptor<Object> captor = ArgumentCaptor.forClass(Object.class);
        verify(rabbit).convertAndSend(anyString(), anyString(), captor.capture());
        return (CouponIssuedEvent) captor.getValue();
    }
}
