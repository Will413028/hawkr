package io.github.will413028.hawkr.aftersales.grpc.client;

import io.github.will413028.hawkr.common.obs.grpc.GrpcStubFactory;
import org.junit.jupiter.api.DisplayName;
import org.junit.jupiter.api.Test;

import static org.junit.jupiter.api.Assertions.assertThrows;
import static org.mockito.Mockito.mock;
import static org.mockito.Mockito.verifyNoInteractions;

/**
 * D4：補償券以 source_id 去重，空的 source_id 不能送出去（否則重送會重複發券）。
 */
class CouponGrpcClientTest {

    @Test
    @DisplayName("source_id 為 null、空字串或空白 → 在呼叫 coupon 之前拒絕")
    void blankSourceIdIsRejectedBeforeAnyCall() {
        GrpcStubFactory factory = mock(GrpcStubFactory.class);
        CouponGrpcClient client = new CouponGrpcClient(factory);

        for (String blank : new String[] {null, "", "  "}) {
            assertThrows(IllegalArgumentException.class, () ->
                    client.grantSellerCompensationCoupon("1", "2", "100", "600", "m", "A1", blank));
        }
        verifyNoInteractions(factory);
    }
}
