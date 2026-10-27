package io.github.will413028.hawkr.coupon.grpc;

import io.github.will413028.hawkr.common.grpc.GrpcAuthContext;
import io.github.will413028.hawkr.common.utils.code.CouponInstanceStatus;
import io.github.will413028.hawkr.common.utils.code.CouponTemplateStatus;
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
import com.baomidou.mybatisplus.extension.plugins.pagination.Page;
import io.grpc.Context;
import io.grpc.stub.StreamObserver;
import org.junit.jupiter.api.DisplayName;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.api.extension.ExtendWith;
import org.mockito.Mock;
import org.mockito.junit.jupiter.MockitoExtension;
import org.mockito.junit.jupiter.MockitoSettings;
import org.mockito.quality.Strictness;

import java.time.LocalDateTime;
import java.util.Collections;
import java.util.List;
import java.util.Set;

import static org.mockito.ArgumentMatchers.any;
import static org.mockito.ArgumentMatchers.anyInt;
import static org.mockito.ArgumentMatchers.anyLong;
import static org.mockito.ArgumentMatchers.eq;
import static org.mockito.Mockito.never;
import static org.mockito.Mockito.verify;
import static org.mockito.Mockito.when;

@ExtendWith(MockitoExtension.class)
@MockitoSettings(strictness = Strictness.LENIENT)
class CouponGrpcServiceBuyerIdentityTest {

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

    @Mock private StreamObserver<ListMyCouponsResponse> listMyCouponsResponseObserver;
    @Mock private StreamObserver<ListAvailableCouponsResponse> listAvailableCouponsResponseObserver;
    @Mock private StreamObserver<BuyerClaimCouponResponse> buyerClaimCouponResponseObserver;

    private CouponGrpcService newService() {
        return new CouponGrpcService(templateService, instanceService, claimService,
                quotaService, cacheService, sellerGrpcClient, orderGrpcClient, accountGrpcClient,
                targetAccountMapper, buyerGrpcClient);
    }

    @Test
    @DisplayName("listMyCoupons 讀 request.buyer_id/account_id，忽略 GrpcAuthContext 原始 JWT accountId")
    void listMyCoupons_usesRequestFields_notGrpcAuthContext() {
        long staleJwtAccountId = 100L;
        long resolvedBuyerId = 3001L;
        long rawAccountId = 55L;

        when(instanceService.listByOwner(eq(resolvedBuyerId), any(), anyInt(), anyInt()))
                .thenReturn(new Page<>(1, 20));
        when(claimService.getClaimedTemplateIds(resolvedBuyerId)).thenReturn(Collections.emptySet());

        CouponGrpcService service = newService();
        GrpcAuthContext.setSkipAccountTypeCheck(true);
        Context ctx = Context.current().withValue(GrpcAuthContext.ACCOUNT_ID, staleJwtAccountId);
        Context previous = ctx.attach();
        try {
            ListMyCouponsRequest request = ListMyCouponsRequest.newBuilder()
                    .setBuyerId(resolvedBuyerId)
                    .setAccountId(rawAccountId)
                    .setStatus(CouponInstanceStatus.USED.value())
                    .setPage("1")
                    .setPageSize("20")
                    .build();

            service.listMyCoupons(request, listMyCouponsResponseObserver);
        } finally {
            ctx.detach(previous);
            GrpcAuthContext.setSkipAccountTypeCheck(false);
        }

        verify(instanceService).listByOwner(eq(resolvedBuyerId), eq(CouponInstanceStatus.USED.value()), anyInt(), anyInt());
        verify(instanceService, never()).listByOwner(eq(staleJwtAccountId), any(), anyInt(), anyInt());
    }

    @Test
    @DisplayName("listAvailableCoupons 分流 account_id -> getAccountCreatedAt、buyer_id -> getBuyerMemberSellerIds")
    void listAvailableCoupons_splitsAccountIdAndBuyerIdToCorrectDownstreamClients() {
        long staleJwtAccountId = 100L;
        long resolvedBuyerId = 3001L;
        long rawAccountId = 55L;

        when(accountGrpcClient.getAccountCreatedAt(rawAccountId)).thenReturn(1700000000000L);
        when(orderGrpcClient.getBuyerMemberSellerIds(resolvedBuyerId)).thenReturn(Set.of(9001L));
        when(templateService.listAvailableForBuyer(anyInt(), anyInt(), any(), eq(resolvedBuyerId), anyLong(), any()))
                .thenReturn(new Page<>(1, 20));
        when(instanceService.listByOwner(eq(resolvedBuyerId), any(), anyInt(), anyInt()))
                .thenReturn(new Page<>(1, 9999));
        when(claimService.getClaimedTemplateIds(resolvedBuyerId)).thenReturn(Collections.emptySet());

        CouponGrpcService service = newService();
        GrpcAuthContext.setSkipAccountTypeCheck(true);
        Context ctx = Context.current().withValue(GrpcAuthContext.ACCOUNT_ID, staleJwtAccountId);
        Context previous = ctx.attach();
        try {
            ListAvailableCouponsRequest request = ListAvailableCouponsRequest.newBuilder()
                    .setBuyerId(resolvedBuyerId)
                    .setAccountId(rawAccountId)
                    .setPage("1")
                    .setPageSize("20")
                    .setStoreId("9001")
                    .setOrderAmount("500")
                    .build();

            service.listAvailableCoupons(request, listAvailableCouponsResponseObserver);
        } finally {
            ctx.detach(previous);
            GrpcAuthContext.setSkipAccountTypeCheck(false);
        }

        verify(accountGrpcClient).getAccountCreatedAt(rawAccountId);
        verify(accountGrpcClient, never()).getAccountCreatedAt(resolvedBuyerId);
        verify(orderGrpcClient).getBuyerMemberSellerIds(resolvedBuyerId);
        verify(orderGrpcClient, never()).getBuyerMemberSellerIds(rawAccountId);
    }

    @Test
    @DisplayName("buyerClaimCoupon 讀 request.buyer_id/account_id，忽略 GrpcAuthContext 原始 JWT accountId")
    void buyerClaimCoupon_usesRequestFields_notGrpcAuthContext() {
        long staleJwtAccountId = 100L;
        long resolvedBuyerId = 3001L;
        long rawAccountId = 55L;

        CouponTemplateEntity template = new CouponTemplateEntity();
        template.setTemplateId(999L);
        template.setStatus(CouponTemplateStatus.ACTIVE.value());
        template.setStartAt(LocalDateTime.now().minusDays(1));
        template.setEndAt(LocalDateTime.now().plusDays(1));
        template.setClaimRequired(true);
        template.setTotalIssueLimit(-1);
        // targetType 留 null：isEligible 對 null targetType 直接回傳 true（舊資料相容分支），
        // 讓本測試專注在身分來源判別，不需要額外經營 buyerCreatedAt/memberSellerIds/
        // designatedTemplateIds 三者的資格判定細節（那是 isEligible 自己的既有覆蓋範圍）。
        when(templateService.getActiveTemplate(999L)).thenReturn(template);
        when(accountGrpcClient.getAccountCreatedAt(rawAccountId)).thenReturn(1700000000000L);
        when(orderGrpcClient.getBuyerMemberSellerIds(resolvedBuyerId)).thenReturn(Collections.emptySet());
        when(targetAccountMapper.selectTemplateIdsByBuyerId(resolvedBuyerId)).thenReturn(List.of());
        when(claimService.claimCoupon(999L, resolvedBuyerId)).thenReturn(LocalDateTime.now());

        CouponGrpcService service = newService();
        GrpcAuthContext.setSkipAccountTypeCheck(true);
        Context ctx = Context.current().withValue(GrpcAuthContext.ACCOUNT_ID, staleJwtAccountId);
        Context previous = ctx.attach();
        try {
            BuyerClaimCouponRequest request = BuyerClaimCouponRequest.newBuilder()
                    .setBuyerId(resolvedBuyerId)
                    .setAccountId(rawAccountId)
                    .setTemplateId("999")
                    .build();

            service.buyerClaimCoupon(request, buyerClaimCouponResponseObserver);
        } finally {
            ctx.detach(previous);
            GrpcAuthContext.setSkipAccountTypeCheck(false);
        }

        verify(accountGrpcClient).getAccountCreatedAt(rawAccountId);
        verify(accountGrpcClient, never()).getAccountCreatedAt(staleJwtAccountId);
        verify(orderGrpcClient).getBuyerMemberSellerIds(resolvedBuyerId);
        verify(targetAccountMapper).selectTemplateIdsByBuyerId(resolvedBuyerId);
        verify(claimService).claimCoupon(999L, resolvedBuyerId);
    }
}
