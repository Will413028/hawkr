package io.github.will413028.hawkr.coupon.grpc;

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
import com.baomidou.mybatisplus.extension.plugins.pagination.Page;
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

import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.junit.jupiter.api.Assertions.assertNotEquals;
import static org.mockito.ArgumentMatchers.any;
import static org.mockito.ArgumentMatchers.anyInt;
import static org.mockito.ArgumentMatchers.anyString;
import static org.mockito.ArgumentMatchers.eq;
import static org.mockito.Mockito.atLeastOnce;
import static org.mockito.Mockito.never;
import static org.mockito.Mockito.verify;
import static org.mockito.Mockito.when;

/**
 * Plan C Phase 1 Task 4 — 5 個賣家 RPC + 1 個 admin RPC 讀取身分的來源必須是
 * request 已解析的 store_id，不可讀 GrpcAuthContext 的原始 JWT accountId
 * （帳號重開店後兩者會分岔）。比照 product-service
 * {@code ProductGrpcServiceGetProductOwnerTest} 的既有慣例：Context.current()
 * .withValue(GrpcAuthContext.ACCOUNT_ID, staleId).attach() + 驗證下游呼叫參數。
 */
@ExtendWith(MockitoExtension.class)
@MockitoSettings(strictness = Strictness.LENIENT)
class CouponGrpcServiceSellerIdentityTest {

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

    @Mock private StreamObserver<SellerCreateTemplateResponse> createResponseObserver;
    @Mock private StreamObserver<SellerListTemplatesResponse> listResponseObserver;
    @Mock private StreamObserver<SellerGetTemplateDetailResponse> detailResponseObserver;
    @Mock private StreamObserver<SellerRevokeTemplateResponse> revokeResponseObserver;
    @Mock private StreamObserver<ListBuyerCouponsForSellerResponse> listBuyerResponseObserver;
    @Mock private StreamObserver<AdminUpdateSellerQuotaResponse> adminQuotaResponseObserver;

    private CouponGrpcService newService() {
        return new CouponGrpcService(templateService, instanceService, claimService,
                quotaService, cacheService, sellerGrpcClient, orderGrpcClient, accountGrpcClient,
                targetAccountMapper, buyerGrpcClient);
    }

    @Test
    @DisplayName("sellerCreateTemplate 讀 request.store_id，忽略 GrpcAuthContext 原始 JWT accountId")
    void sellerCreateTemplate_usesRequestStoreId_notGrpcAuthContext() {
        long staleJwtAccountId = 100L;
        long resolvedStoreId = 555L;

        CouponTemplateEntity created = new CouponTemplateEntity();
        created.setTemplateId(999L);
        when(templateService.createSellerTemplate(eq(resolvedStoreId), anyString(), anyString(), anyString(),
                any(), any(), any(), any(), any(), any())).thenReturn(created);

        CouponGrpcService service = newService();
        GrpcAuthContext.setSkipAccountTypeCheck(true);
        Context ctx = Context.current().withValue(GrpcAuthContext.ACCOUNT_ID, staleJwtAccountId);
        Context previous = ctx.attach();
        try {
            SellerCreateTemplateRequest request = SellerCreateTemplateRequest.newBuilder()
                    .setStoreId(resolvedStoreId)
                    .setTitle("週末優惠")
                    .setType("CASH")
                    .setTargetType("1") // ALL
                    .setDiscountAmount("100")
                    .setStartAt("2026-04-01T00:00:00Z")
                    .setEndAt("2026-04-07T23:59:59Z")
                    .build();

            service.sellerCreateTemplate(request, createResponseObserver);
        } finally {
            ctx.detach(previous);
            GrpcAuthContext.setSkipAccountTypeCheck(false);
        }

        verify(quotaService).checkQuota(resolvedStoreId);
        ArgumentCaptor<Long> storeIdCaptor = ArgumentCaptor.forClass(Long.class);
        verify(templateService).createSellerTemplate(storeIdCaptor.capture(), anyString(), anyString(), anyString(),
                any(), any(), any(), any(), any(), any());
        assertEquals(resolvedStoreId, storeIdCaptor.getValue());
    }

    @Test
    @DisplayName("sellerListTemplates 讀 request.store_id，忽略 GrpcAuthContext 原始 JWT accountId")
    void sellerListTemplates_usesRequestStoreId_notGrpcAuthContext() {
        long staleJwtAccountId = 100L;
        long resolvedStoreId = 555L;

        when(templateService.listSellerTemplates(eq(resolvedStoreId), anyString(), anyInt(), anyInt()))
                .thenReturn(new Page<>(1, 20));
        when(quotaService.getMaxTemplateCount(resolvedStoreId)).thenReturn(5);
        when(templateService.countSellerActiveTemplates(resolvedStoreId)).thenReturn(0L);

        CouponGrpcService service = newService();
        GrpcAuthContext.setSkipAccountTypeCheck(true);
        Context ctx = Context.current().withValue(GrpcAuthContext.ACCOUNT_ID, staleJwtAccountId);
        Context previous = ctx.attach();
        try {
            SellerListTemplatesRequest request = SellerListTemplatesRequest.newBuilder()
                    .setStoreId(resolvedStoreId)
                    .setPage("1")
                    .setPageSize("20")
                    .build();

            service.sellerListTemplates(request, listResponseObserver);
        } finally {
            ctx.detach(previous);
            GrpcAuthContext.setSkipAccountTypeCheck(false);
        }

        verify(templateService).listSellerTemplates(eq(resolvedStoreId), anyString(), anyInt(), anyInt());
        verify(quotaService).getMaxTemplateCount(resolvedStoreId);
        verify(templateService).countSellerActiveTemplates(resolvedStoreId);
    }

    @Test
    @DisplayName("sellerGetTemplateDetail 用 request.store_id 比對 issuerStoreId，忽略 GrpcAuthContext 原始 JWT accountId")
    void sellerGetTemplateDetail_usesRequestStoreId_notGrpcAuthContext() {
        long staleJwtAccountId = 100L;
        long resolvedStoreId = 555L;

        CouponTemplateEntity template = new CouponTemplateEntity();
        template.setTemplateId(999L);
        template.setIssuerStoreId(resolvedStoreId); // 這張模板屬於 555，不是 100
        template.setIssuerType(IssuerType.SELLER.value());
        template.setTargetType("1");
        template.setTitle("週末優惠");
        template.setType("CASH");
        template.setStatus("1");
        when(templateService.getActiveTemplate(999L)).thenReturn(template);
        when(instanceService.countByTemplateAndStatus(eq(999L), anyString())).thenReturn(0);

        CouponGrpcService service = newService();
        GrpcAuthContext.setSkipAccountTypeCheck(true);
        Context ctx = Context.current().withValue(GrpcAuthContext.ACCOUNT_ID, staleJwtAccountId);
        Context previous = ctx.attach();
        try {
            SellerGetTemplateDetailRequest request = SellerGetTemplateDetailRequest.newBuilder()
                    .setStoreId(resolvedStoreId)
                    .setTemplateId("999")
                    .build();

            service.sellerGetTemplateDetail(request, detailResponseObserver);
        } finally {
            ctx.detach(previous);
            GrpcAuthContext.setSkipAccountTypeCheck(false);
        }

        // 若回歸讀 GrpcAuthContext（100），擁有權比對會用 100 != 555 → COUPON_NOT_FOUND，
        // 就不會走到 instanceService 查詢，也不會回 code=0。
        // 注意：sellerGetTemplateDetail 正常流程本就會呼叫 countByTemplateAndStatus 兩次
        // （USED 與 RESERVED 各一次），brief 原給的 verify(instanceService)（隱含 times(1)）
        // 與此不符，改用 atLeastOnce() 驗證「有沒有走到這段」而非精確次數。
        verify(instanceService, atLeastOnce()).countByTemplateAndStatus(eq(999L), anyString());
        ArgumentCaptor<SellerGetTemplateDetailResponse> captor = ArgumentCaptor.forClass(SellerGetTemplateDetailResponse.class);
        verify(detailResponseObserver).onNext(captor.capture());
        assertEquals(0, captor.getValue().getCode());
    }

    @Test
    @DisplayName("sellerGetTemplateDetail：issuer_type=ADMIN 模板即使 issuerStoreId 巧合等於 request.store_id 也拒絕（未涵蓋於 brief，補 issuer_type 缺口的判別測試）")
    void sellerGetTemplateDetail_rejectsAdminIssuedTemplate_evenWhenIssuerStoreIdCoincidentallyMatches() {
        long resolvedStoreId = 555L;

        // 這張模板是 admin 建立的（issuer_type=ADMIN），issuer_store_id 欄位存的其實是
        // admin 的 account_id，只是數值巧合跟賣家目前的 store_id 相同。沒有 issuer_type
        // 過濾的話，純數值比對會誤判成「這是我的模板」。
        CouponTemplateEntity adminTemplate = new CouponTemplateEntity();
        adminTemplate.setTemplateId(999L);
        adminTemplate.setIssuerStoreId(resolvedStoreId);
        adminTemplate.setIssuerType(IssuerType.ADMIN.value());
        adminTemplate.setTargetType("1");
        when(templateService.getActiveTemplate(999L)).thenReturn(adminTemplate);

        CouponGrpcService service = newService();
        GrpcAuthContext.setSkipAccountTypeCheck(true);
        Context ctx = Context.current().withValue(GrpcAuthContext.ACCOUNT_ID, resolvedStoreId);
        Context previous = ctx.attach();
        try {
            SellerGetTemplateDetailRequest request = SellerGetTemplateDetailRequest.newBuilder()
                    .setStoreId(resolvedStoreId)
                    .setTemplateId("999")
                    .build();

            service.sellerGetTemplateDetail(request, detailResponseObserver);
        } finally {
            ctx.detach(previous);
            GrpcAuthContext.setSkipAccountTypeCheck(false);
        }

        // 未過濾 issuer_type 的舊邏輯會誤判擁有權成立，一路查到 instanceService 並回 code=0。
        verify(instanceService, never()).countByTemplateAndStatus(any(), anyString());
        ArgumentCaptor<SellerGetTemplateDetailResponse> captor = ArgumentCaptor.forClass(SellerGetTemplateDetailResponse.class);
        verify(detailResponseObserver).onNext(captor.capture());
        assertNotEquals(0, captor.getValue().getCode());
    }

    @Test
    @DisplayName("sellerRevokeTemplate 讀 request.store_id，忽略 GrpcAuthContext 原始 JWT accountId")
    void sellerRevokeTemplate_usesRequestStoreId_notGrpcAuthContext() {
        long staleJwtAccountId = 100L;
        long resolvedStoreId = 555L;

        when(templateService.revokeTemplate(eq(999L), eq(resolvedStoreId), anyString())).thenReturn(true);
        when(instanceService.revokeByTemplate(eq(999L), anyString())).thenReturn(3);

        CouponGrpcService service = newService();
        GrpcAuthContext.setSkipAccountTypeCheck(true);
        Context ctx = Context.current().withValue(GrpcAuthContext.ACCOUNT_ID, staleJwtAccountId);
        Context previous = ctx.attach();
        try {
            SellerRevokeTemplateRequest request = SellerRevokeTemplateRequest.newBuilder()
                    .setStoreId(resolvedStoreId)
                    .setTemplateId("999")
                    .setReason("測試收回")
                    .build();

            service.sellerRevokeTemplate(request, revokeResponseObserver);
        } finally {
            ctx.detach(previous);
            GrpcAuthContext.setSkipAccountTypeCheck(false);
        }

        verify(templateService).revokeTemplate(eq(999L), eq(resolvedStoreId), anyString());
    }

    @Test
    @DisplayName("listBuyerCouponsForSeller 讀 request.store_id，忽略 GrpcAuthContext 原始 JWT accountId")
    void listBuyerCouponsForSeller_usesRequestStoreId_notGrpcAuthContext() {
        long staleJwtAccountId = 100L;
        long resolvedStoreId = 555L;

        when(instanceService.listBuyerCouponsForSeller(eq(resolvedStoreId), eq(700L), anyString(), anyInt(), anyInt()))
                .thenReturn(new Page<>(1, 20));

        CouponGrpcService service = newService();
        GrpcAuthContext.setSkipAccountTypeCheck(true);
        Context ctx = Context.current().withValue(GrpcAuthContext.ACCOUNT_ID, staleJwtAccountId);
        Context previous = ctx.attach();
        try {
            ListBuyerCouponsForSellerRequest request = ListBuyerCouponsForSellerRequest.newBuilder()
                    .setStoreId(resolvedStoreId)
                    .setBuyerId("700")
                    .setPage("1")
                    .setPageSize("20")
                    .build();

            service.listBuyerCouponsForSeller(request, listBuyerResponseObserver);
        } finally {
            ctx.detach(previous);
            GrpcAuthContext.setSkipAccountTypeCheck(false);
        }

        verify(instanceService).listBuyerCouponsForSeller(eq(resolvedStoreId), eq(700L), anyString(), anyInt(), anyInt());
    }

    @Test
    @DisplayName("adminUpdateSellerQuota 讀 request.store_id（proto 欄位已從 seller_account_id 改名）")
    void adminUpdateSellerQuota_readsStoreIdField() {
        CouponGrpcService service = newService();
        GrpcAuthContext.setSkipAccountTypeCheck(true);
        Context ctx = Context.current().withValue(GrpcAuthContext.ACCOUNT_ID, 1L);
        Context previous = ctx.attach();
        try {
            AdminUpdateSellerQuotaRequest request = AdminUpdateSellerQuotaRequest.newBuilder()
                    .setAccountId(1L)
                    .setStoreId("555")
                    .setMaxTemplateCount("10")
                    .build();

            service.adminUpdateSellerQuota(request, adminQuotaResponseObserver);
        } finally {
            ctx.detach(previous);
            GrpcAuthContext.setSkipAccountTypeCheck(false);
        }

        verify(quotaService).updateQuota(555L, 10);
    }
}
