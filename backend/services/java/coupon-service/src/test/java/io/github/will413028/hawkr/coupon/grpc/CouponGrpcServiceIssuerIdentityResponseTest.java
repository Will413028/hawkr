package io.github.will413028.hawkr.coupon.grpc;

import com.baomidou.mybatisplus.extension.plugins.pagination.Page;
import io.github.will413028.hawkr.common.grpc.GrpcAuthContext;
import io.github.will413028.hawkr.common.utils.code.IssuerType;
import io.github.will413028.hawkr.coupon.entity.CouponTemplateEntity;
import io.github.will413028.hawkr.coupon.grpc.client.AccountGrpcClient;
import io.github.will413028.hawkr.coupon.grpc.client.BuyerGrpcClient;
import io.github.will413028.hawkr.coupon.grpc.client.OrderGrpcClient;
import io.github.will413028.hawkr.coupon.grpc.client.SellerGrpcClient;
import io.github.will413028.hawkr.coupon.mapper.CouponTargetAccountMapper;
import io.github.will413028.hawkr.coupon.service.CouponCacheService;
import io.github.will413028.hawkr.coupon.service.CouponClaimService;
import io.github.will413028.hawkr.coupon.service.CouponInstanceService;
import io.github.will413028.hawkr.coupon.service.CouponQuotaService;
import io.github.will413028.hawkr.coupon.service.CouponTemplateService;
import io.grpc.Context;
import io.grpc.stub.StreamObserver;
import org.junit.jupiter.api.DisplayName;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.api.extension.ExtendWith;
import org.mockito.ArgumentCaptor;
import org.mockito.Mock;
import org.mockito.junit.jupiter.MockitoExtension;
import org.mockito.junit.jupiter.MockitoSettings;
import org.mockito.quality.Strictness;

import java.util.Collections;
import java.util.List;

import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.mockito.ArgumentMatchers.any;
import static org.mockito.ArgumentMatchers.anyInt;
import static org.mockito.ArgumentMatchers.anyList;
import static org.mockito.ArgumentMatchers.anyLong;
import static org.mockito.ArgumentMatchers.eq;
import static org.mockito.Mockito.when;

/**
 * docs/superpowers/specs/2026-08-06-coupon-issuer-identity-schema-split-design.md §4/§5 —
 * gRPC response 必須依 entity 哪個欄位有值分別輸出 issuer_account_id / issuer_store_id，
 * 不能像拆分前那樣把 issuer_store_id 無條件塞進 issuer_account_id。
 */
@ExtendWith(MockitoExtension.class)
@MockitoSettings(strictness = Strictness.LENIENT)
class CouponGrpcServiceIssuerIdentityResponseTest {

    @Mock private CouponTemplateService templateService;
    @Mock private CouponInstanceService instanceService;
    @Mock private CouponClaimService claimService;
    @Mock private CouponQuotaService quotaService;
    @Mock private CouponCacheService cacheService;
    @Mock private SellerGrpcClient sellerGrpcClient;
    @Mock private OrderGrpcClient orderGrpcClient;
    @Mock private AccountGrpcClient accountGrpcClient;
    @Mock private CouponTargetAccountMapper targetAccountMapper;
    @Mock private BuyerGrpcClient buyerGrpcClient;

    @Mock private StreamObserver<ListMyCouponsResponse> responseObserver;

    private CouponGrpcService newService() {
        return new CouponGrpcService(templateService, instanceService, claimService,
                quotaService, cacheService, sellerGrpcClient, orderGrpcClient, accountGrpcClient,
                targetAccountMapper, buyerGrpcClient);
    }

    private ListMyCouponsResponse invokeListMyCoupons(CouponTemplateEntity template) {
        long buyerId = 3001L;
        long rawAccountId = 55L;

        Page<CouponTemplateEntity> templatePage = new Page<>(1, 20);
        templatePage.setRecords(List.of(template));

        when(accountGrpcClient.getAccountCreatedAt(rawAccountId)).thenReturn(1700000000000L);
        when(orderGrpcClient.getBuyerMemberSellerIds(buyerId)).thenReturn(Collections.emptySet());
        when(templateService.listAvailableForBuyer(anyInt(), anyInt(), any(), eq(buyerId), anyLong(), any()))
                .thenReturn(templatePage);
        when(instanceService.listByOwner(eq(buyerId), any(), anyInt(), anyInt())).thenReturn(new Page<>(1, 20));
        when(claimService.getClaimedTemplateIds(buyerId)).thenReturn(Collections.emptySet());
        when(sellerGrpcClient.batchGetSellerMeta(anyList())).thenReturn(Collections.emptyMap());

        CouponGrpcService service = newService();
        GrpcAuthContext.setSkipAccountTypeCheck(true);
        Context ctx = Context.current().withValue(GrpcAuthContext.ACCOUNT_ID, rawAccountId);
        Context previous = ctx.attach();
        try {
            service.listMyCoupons(ListMyCouponsRequest.newBuilder()
                    .setBuyerId(buyerId)
                    .setAccountId(rawAccountId)
                    .setPage("1")
                    .setPageSize("20")
                    .build(), responseObserver);
        } finally {
            ctx.detach(previous);
            GrpcAuthContext.setSkipAccountTypeCheck(false);
        }

        ArgumentCaptor<ListMyCouponsResponse> captor = ArgumentCaptor.forClass(ListMyCouponsResponse.class);
        org.mockito.Mockito.verify(responseObserver).onNext(captor.capture());
        return captor.getValue();
    }

    @Test
    @DisplayName("ADMIN 模板：response 帶 issuer_account_id，issuer_store_id 空字串")
    void adminTemplate_populatesIssuerAccountId_leavesIssuerStoreIdEmpty() {
        CouponTemplateEntity template = new CouponTemplateEntity();
        template.setTemplateId(1L);
        template.setTitle("平台活動券");
        template.setType("1");
        template.setIssuerType(IssuerType.ADMIN.value());
        template.setIssuerAccountId(777L);
        template.setThresholdAmount(0L);
        template.setUsageLimitPerAccount(1);
        template.setTotalIssueLimit(-1);
        template.setIssuedCount(0);
        template.setStatus("1");

        ListMyCouponsResponse response = invokeListMyCoupons(template);

        assertEquals("777", response.getRecords(0).getIssuerAccountId());
        assertEquals("", response.getRecords(0).getIssuerStoreId());
    }

    @Test
    @DisplayName("SELLER 模板：response 帶 issuer_store_id，issuer_account_id 空字串")
    void sellerTemplate_populatesIssuerStoreId_leavesIssuerAccountIdEmpty() {
        CouponTemplateEntity template = new CouponTemplateEntity();
        template.setTemplateId(2L);
        template.setTitle("週末優惠");
        template.setType("1");
        template.setIssuerType(IssuerType.SELLER.value());
        template.setIssuerStoreId(555L);
        template.setThresholdAmount(0L);
        template.setUsageLimitPerAccount(1);
        template.setTotalIssueLimit(-1);
        template.setIssuedCount(0);
        template.setStatus("1");

        ListMyCouponsResponse response = invokeListMyCoupons(template);

        assertEquals("555", response.getRecords(0).getIssuerStoreId());
        assertEquals("", response.getRecords(0).getIssuerAccountId());
    }
}
