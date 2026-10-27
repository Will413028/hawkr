package io.github.will413028.hawkr.coupon.grpc;

import com.baomidou.mybatisplus.extension.plugins.pagination.Page;
import io.github.will413028.hawkr.common.grpc.GrpcAuthContext;
import io.github.will413028.hawkr.common.utils.code.CouponInstanceStatus;
import io.github.will413028.hawkr.coupon.entity.CouponInstanceEntity;
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
import io.github.will413028.hawkr.seller.grpc.SellerMeta;
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

import java.time.LocalDateTime;
import java.util.Collections;
import java.util.List;
import java.util.Map;

import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.mockito.ArgumentMatchers.any;
import static org.mockito.ArgumentMatchers.anyInt;
import static org.mockito.ArgumentMatchers.anyList;
import static org.mockito.ArgumentMatchers.anyLong;
import static org.mockito.ArgumentMatchers.eq;
import static org.mockito.Mockito.verify;
import static org.mockito.Mockito.when;

/**
 * #2137 — 優惠券卡片的賣家連結先前只有 applicable_store_id（內部代理鍵），前端連
 * /sellers/{store_id} 每次都要多吃一次 middleware 308。賣家頁 canonical 網址走
 * store_code，而這個值本來就在 batchGetSellerMeta 的同一份回應裡（sellerName 就是
 * 從那裡抄的），只是組 proto 時被丟掉——補齊零額外 RPC。
 *
 * 可用券（TemplateInfo）與已使用券（InstanceInfo）走兩支不同的轉換函式，各自都會漏，
 * 所以兩邊都釘。store_code 刻意設成與 store_id 不同：兩者相等的賣家（由已綁定的
 * PayUni merchant_code 回填）驗不出有沒有真的讀對欄位。
 */
@ExtendWith(MockitoExtension.class)
@MockitoSettings(strictness = Strictness.LENIENT)
class CouponGrpcServiceStoreCodeTest {

    private static final long STORE_ID = 2636840881L;
    private static final String STORE_CODE = "2649605557";

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

    @Test
    @DisplayName("listMyCoupons 的可用券與已使用券都帶出賣家 store_code")
    void listMyCoupons_carriesSellerStoreCode() {
        long buyerId = 3001L;
        long rawAccountId = 55L;

        CouponTemplateEntity template = new CouponTemplateEntity();
        template.setTemplateId(1L);
        template.setTitle("滿千折百");
        template.setType("1");
        template.setIssuerType("2");
        template.setIssuerStoreId(STORE_ID);
        template.setApplicableStoreId(STORE_ID);
        template.setThresholdAmount(1000L);
        template.setUsageLimitPerAccount(1);
        template.setTotalIssueLimit(100);
        template.setIssuedCount(0);
        template.setStatus("1");

        CouponTemplateEntity usedTemplate = new CouponTemplateEntity();
        usedTemplate.setTemplateId(2L);
        usedTemplate.setTitle("已用過的券");
        usedTemplate.setType("1");
        usedTemplate.setIssuerType("2");
        usedTemplate.setIssuerStoreId(STORE_ID);
        usedTemplate.setApplicableStoreId(STORE_ID);
        usedTemplate.setThresholdAmount(500L);

        CouponInstanceEntity used = new CouponInstanceEntity();
        used.setInstanceId(9L);
        used.setTemplateId(2L);
        used.setApplicableStoreId(STORE_ID);
        used.setStatus(CouponInstanceStatus.USED.value());
        used.setCreatedAt(LocalDateTime.of(2026, 8, 1, 0, 0));

        Page<CouponTemplateEntity> templatePage = new Page<>(1, 20);
        templatePage.setRecords(List.of(template));
        Page<CouponInstanceEntity> usedPage = new Page<>(1, 20);
        usedPage.setRecords(List.of(used));

        when(accountGrpcClient.getAccountCreatedAt(rawAccountId)).thenReturn(1700000000000L);
        when(orderGrpcClient.getBuyerMemberSellerIds(buyerId)).thenReturn(Collections.emptySet());
        when(templateService.listAvailableForBuyer(anyInt(), anyInt(), any(), eq(buyerId), anyLong(), any()))
                .thenReturn(templatePage);
        when(instanceService.listByOwner(eq(buyerId), any(), anyInt(), anyInt())).thenReturn(usedPage);
        when(claimService.getClaimedTemplateIds(buyerId)).thenReturn(Collections.emptySet());
        when(cacheService.getTemplate(2L)).thenReturn(usedTemplate);
        when(sellerGrpcClient.batchGetSellerMeta(anyList())).thenReturn(Map.of(
                STORE_ID, SellerMeta.newBuilder()
                        .setSellerName("Seller X")
                        .setLogoKey("logo.jpg")
                        .setStoreCode(STORE_CODE)
                        .build()));

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
        verify(responseObserver).onNext(captor.capture());
        ListMyCouponsResponse response = captor.getValue();

        assertEquals(1, response.getRecordsCount(), "應有一張可用券");
        assertEquals("Seller X", response.getRecords(0).getSellerName());
        assertEquals(STORE_CODE, response.getRecords(0).getStoreCode());
        assertEquals(String.valueOf(STORE_ID), response.getRecords(0).getApplicableStoreId());

        assertEquals(1, response.getUsedRecordsCount(), "應有一張已使用券");
        assertEquals("Seller X", response.getUsedRecords(0).getSellerName());
        assertEquals(STORE_CODE, response.getUsedRecords(0).getStoreCode());
    }

    @Test
    @DisplayName("賣家查不到時 store_code 與 sellerName 一起留空，前端據此不渲染連結")
    void listMyCoupons_leavesStoreCodeBlankWhenSellerLookupDegrades() {
        long buyerId = 3001L;
        long rawAccountId = 55L;

        CouponTemplateEntity template = new CouponTemplateEntity();
        template.setTemplateId(1L);
        template.setTitle("滿千折百");
        template.setType("1");
        template.setIssuerType("2");
        template.setIssuerStoreId(STORE_ID);
        template.setApplicableStoreId(STORE_ID);
        template.setThresholdAmount(1000L);
        template.setUsageLimitPerAccount(1);
        template.setTotalIssueLimit(100);
        template.setIssuedCount(0);
        template.setStatus("1");

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
        verify(responseObserver).onNext(captor.capture());
        assertEquals("", captor.getValue().getRecords(0).getStoreCode());
        assertEquals("", captor.getValue().getRecords(0).getSellerName());
    }
}
