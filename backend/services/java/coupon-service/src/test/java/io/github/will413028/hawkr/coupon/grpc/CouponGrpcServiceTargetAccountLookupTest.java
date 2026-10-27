package io.github.will413028.hawkr.coupon.grpc;

import io.github.will413028.hawkr.common.grpc.PrincipalResolution;
import io.github.will413028.hawkr.account.grpc.AccountInfo;
import io.github.will413028.hawkr.common.grpc.GrpcAuthContext;
import io.github.will413028.hawkr.common.utils.code.CouponTargetType;
import io.github.will413028.hawkr.common.utils.code.CouponTemplateStatus;
import io.github.will413028.hawkr.common.utils.code.IssuerType;
import io.github.will413028.hawkr.buyer.grpc.BuyerDisplay;
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

import java.util.List;
import java.util.Map;

import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.mockito.ArgumentMatchers.any;
import static org.mockito.ArgumentMatchers.anyString;
import static org.mockito.ArgumentMatchers.eq;
import static org.mockito.Mockito.never;
import static org.mockito.Mockito.verify;
import static org.mockito.Mockito.when;

/**
 * buyer_id rekey Phase 5 補丁 — sellerGetTemplateDetail 的 DESIGNATED/COMPENSATION 目標帳號
 * 聚合，證實/修正一個既有缺口（見 coupon.proto TargetAccountInfo 註解：「傳入的是 buyer_id
 * 而非 raw account_id，未 reverse-resolve」）：{@code targetAccountMapper.selectBuyerIdsByTemplateId}
 * 回傳的是 buyer_id（stint-scoped），但 {@code accountGrpcClient.batchGetAccountInfo} 是用
 * raw account_id 當主鍵查 accounts 表 —— 若某買家 buyer_id 已與其目前登入 account_id 分岔
 * （回鍋買家），舊碼直接把 buyer_id 丟給 batchGetAccountInfo 會查不到人，nickname/phone
 * 靜默回空字串。
 *
 * <p>修法比照 {@code CouponExpiringWarningJob} 已用過的 {@link BuyerGrpcClient#resolvePrincipals}
 * 慣例：批次反查 buyer_id -> 登入 account_id，用反查後的 account_id 查帳號資訊，
 * 再用反查 map 把結果對回原始 buyer_id（response 的 buyer_id 欄位維持不變，只有
 * nickname/phone 的查詢鍵改對）。
 */
@ExtendWith(MockitoExtension.class)
@MockitoSettings(strictness = Strictness.LENIENT)
class CouponGrpcServiceTargetAccountLookupTest {

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

    @Mock private StreamObserver<SellerGetTemplateDetailResponse> detailResponseObserver;

    private static final long STORE_ID = 555L;
    private static final long TEMPLATE_ID = 999L;

    private CouponGrpcService newService() {
        return new CouponGrpcService(templateService, instanceService, claimService,
                quotaService, cacheService, sellerGrpcClient, orderGrpcClient, accountGrpcClient,
                targetAccountMapper, buyerGrpcClient);
    }

    private CouponTemplateEntity designatedTemplate() {
        CouponTemplateEntity template = new CouponTemplateEntity();
        template.setTemplateId(TEMPLATE_ID);
        template.setIssuerStoreId(STORE_ID);
        template.setIssuerType(IssuerType.SELLER.value());
        template.setTargetType(CouponTargetType.DESIGNATED.value());
        template.setTitle("週年慶指定優惠");
        template.setType("CASH");
        template.setStatus(CouponTemplateStatus.ACTIVE.value());
        return template;
    }

    private SellerGetTemplateDetailResponse invoke(CouponGrpcService service) {
        GrpcAuthContext.setSkipAccountTypeCheck(true);
        Context ctx = Context.current().withValue(GrpcAuthContext.ACCOUNT_ID, STORE_ID);
        Context previous = ctx.attach();
        try {
            SellerGetTemplateDetailRequest request = SellerGetTemplateDetailRequest.newBuilder()
                    .setStoreId(STORE_ID)
                    .setTemplateId(String.valueOf(TEMPLATE_ID))
                    .build();
            service.sellerGetTemplateDetail(request, detailResponseObserver);
        } finally {
            ctx.detach(previous);
            GrpcAuthContext.setSkipAccountTypeCheck(false);
        }
        ArgumentCaptor<SellerGetTemplateDetailResponse> captor =
                ArgumentCaptor.forClass(SellerGetTemplateDetailResponse.class);
        verify(detailResponseObserver).onNext(captor.capture());
        return captor.getValue();
    }

    @Test
    @DisplayName("sellerGetTemplateDetail：分岔買家（buyer_id != 目前 account_id）用反查後的 account_id 查帳號資訊，不是用 raw buyer_id")
    void sellerGetTemplateDetail_resolvesDivergedBuyerIdToCurrentAccountId_beforeAccountLookup() {
        long divergedBuyerId = 7484139444629864823L;
        long resolvedAccountId = 2619321233L;

        when(templateService.getActiveTemplate(TEMPLATE_ID)).thenReturn(designatedTemplate());
        when(instanceService.countByTemplateAndStatus(eq(TEMPLATE_ID), anyString())).thenReturn(0);
        when(targetAccountMapper.selectBuyerIdsByTemplateId(TEMPLATE_ID)).thenReturn(List.of(divergedBuyerId));
        when(buyerGrpcClient.resolvePrincipals(List.of(divergedBuyerId)))
                .thenReturn(PrincipalResolution.allResolved(Map.of(divergedBuyerId, resolvedAccountId)));

        AccountInfo resolvedInfo = AccountInfo.newBuilder()
                .setAccountId(resolvedAccountId)
                .setPhone("0912345678")
                .build();
        when(accountGrpcClient.batchGetAccountInfo(List.of(resolvedAccountId)))
                .thenReturn(Map.of(resolvedAccountId, resolvedInfo));
        // 名字走 buyer-service（#2830）：accounts 表沒有 nickname 欄位，
        // AccountInfo.nickname 在 production 對每一位收券人都是空字串。
        when(buyerGrpcClient.batchGetBuyerDisplayByBuyerIds(List.of(divergedBuyerId)))
                .thenReturn(Map.of(divergedBuyerId, BuyerDisplay.newBuilder()
                        .setDisplayName("阿明").build()));

        SellerGetTemplateDetailResponse response = invoke(newService());

        // 核心斷言：查帳號資訊必須用反查後的 account_id，不能用原始 buyer_id 當主鍵
        verify(accountGrpcClient).batchGetAccountInfo(List.of(resolvedAccountId));
        verify(accountGrpcClient, never()).batchGetAccountInfo(List.of(divergedBuyerId));

        assertEquals(1, response.getData().getTargetAccountsList().size());
        TargetAccountInfo target = response.getData().getTargetAccounts(0);
        // buyer_id 欄位維持原樣（這部分既有邏輯已正確）
        assertEquals(String.valueOf(divergedBuyerId), target.getBuyerId());
        // nickname/phone 必須來自反查後的帳號資訊，不能因主鍵對不上而靜默回空字串
        assertEquals("阿明", target.getNickname());
        assertEquals("0912345678", target.getPhone());
    }

    @Test
    @DisplayName("sellerGetTemplateDetail：buyer_id 查無存活買家（unresolved_ids）不拿 buyer_id 冒充 account_id 查電話")
    void sellerGetTemplateDetail_doesNotLookUpAccountByBuyerId_whenUnresolved() {
        long buyerId = 3001L;

        when(templateService.getActiveTemplate(TEMPLATE_ID)).thenReturn(designatedTemplate());
        when(instanceService.countByTemplateAndStatus(eq(TEMPLATE_ID), anyString())).thenReturn(0);
        when(targetAccountMapper.selectBuyerIdsByTemplateId(TEMPLATE_ID)).thenReturn(List.of(buyerId));
        // 舊 buyer-service 會把輸入回填進 principals，unresolved_ids 才是真相：兩種形狀都不得查帳號。
        when(buyerGrpcClient.resolvePrincipals(List.of(buyerId)))
                .thenReturn(PrincipalResolution.of(Map.of(buyerId, buyerId), List.of(buyerId)));
        // 若實作拿 buyer_id 去查，這個 account 剛好存在，電話就會洩漏給別人。
        when(accountGrpcClient.batchGetAccountInfo(List.of(buyerId)))
                .thenReturn(Map.of(buyerId, AccountInfo.newBuilder().setAccountId(buyerId).setPhone("0987654321").build()));
        when(buyerGrpcClient.batchGetBuyerDisplayByBuyerIds(List.of(buyerId)))
                .thenReturn(Map.of(buyerId, BuyerDisplay.newBuilder().setDisplayName("小華").build()));

        SellerGetTemplateDetailResponse response = invoke(newService());

        verify(accountGrpcClient, never()).batchGetAccountInfo(any());
        TargetAccountInfo target = response.getData().getTargetAccounts(0);
        assertEquals(String.valueOf(buyerId), target.getBuyerId());
        assertEquals("小華", target.getNickname());
        assertEquals("", target.getPhone());
    }

    @Test
    @DisplayName("sellerGetTemplateDetail：BuyerGrpcClient RPC 拋例外時不整個失敗，也不拿 buyer_id 查帳號資訊（電話留空）")
    void sellerGetTemplateDetail_doesNotLookUpAccountByBuyerId_whenResolvePrincipalsThrows() {
        long buyerId = 3001L;

        when(templateService.getActiveTemplate(TEMPLATE_ID)).thenReturn(designatedTemplate());
        when(instanceService.countByTemplateAndStatus(eq(TEMPLATE_ID), anyString())).thenReturn(0);
        when(targetAccountMapper.selectBuyerIdsByTemplateId(TEMPLATE_ID)).thenReturn(List.of(buyerId));
        when(buyerGrpcClient.resolvePrincipals(List.of(buyerId)))
                .thenThrow(new RuntimeException("buyer-service unavailable"));
        when(accountGrpcClient.batchGetAccountInfo(List.of(buyerId)))
                .thenReturn(Map.of(buyerId, AccountInfo.newBuilder().setAccountId(buyerId).setPhone("0987654321").build()));
        when(buyerGrpcClient.batchGetBuyerDisplayByBuyerIds(List.of(buyerId)))
                .thenReturn(Map.of(buyerId, BuyerDisplay.newBuilder().setDisplayName("小華").build()));

        SellerGetTemplateDetailResponse response = invoke(newService());

        assertEquals(0, response.getCode());
        verify(buyerGrpcClient).resolvePrincipals(List.of(buyerId));
        verify(accountGrpcClient, never()).batchGetAccountInfo(any());
        TargetAccountInfo target = response.getData().getTargetAccounts(0);
        assertEquals("小華", target.getNickname());
        assertEquals("", target.getPhone());
    }

    @Test
    @DisplayName("sellerGetTemplateDetail：收券人名字來自 buyer-service，已註銷者顯示墓碑字串而非空白（#2830）")
    void sellerGetTemplateDetail_deletedBuyerShowsTombstone_notBlank() {
        long deletedBuyerId = 4002L;

        when(templateService.getActiveTemplate(TEMPLATE_ID)).thenReturn(designatedTemplate());
        when(instanceService.countByTemplateAndStatus(eq(TEMPLATE_ID), anyString())).thenReturn(0);
        when(targetAccountMapper.selectBuyerIdsByTemplateId(TEMPLATE_ID)).thenReturn(List.of(deletedBuyerId));
        // 已註銷買家：不是存活的 stint，沒有解析到帳號。
        when(buyerGrpcClient.resolvePrincipals(List.of(deletedBuyerId)))
                .thenReturn(PrincipalResolution.of(Map.of(), List.of(deletedBuyerId)));
        // 但 buyer-service 的 Display RPC 仍解得出來，回墓碑字串。
        when(buyerGrpcClient.batchGetBuyerDisplayByBuyerIds(List.of(deletedBuyerId)))
                .thenReturn(Map.of(deletedBuyerId, BuyerDisplay.newBuilder()
                        .setDisplayName("已刪除帳號").setDeleted(true).build()));

        SellerGetTemplateDetailResponse response = invoke(newService());

        TargetAccountInfo target = response.getData().getTargetAccounts(0);
        assertEquals(String.valueOf(deletedBuyerId), target.getBuyerId());
        assertEquals("已刪除帳號", target.getNickname(),
                "已註銷收券人必須顯示墓碑字串，不能是空白列");
        assertEquals("", target.getPhone(), "已註銷帳號的電話不得回傳");
    }

    @Test
    @DisplayName("sellerGetTemplateDetail：名字不得取自 AccountInfo.nickname（accounts 表沒有該欄位，production 恆空）")
    void sellerGetTemplateDetail_nicknameMustNotComeFromAccountInfo() {
        long buyerId = 4003L;

        when(templateService.getActiveTemplate(TEMPLATE_ID)).thenReturn(designatedTemplate());
        when(instanceService.countByTemplateAndStatus(eq(TEMPLATE_ID), anyString())).thenReturn(0);
        when(targetAccountMapper.selectBuyerIdsByTemplateId(TEMPLATE_ID)).thenReturn(List.of(buyerId));
        long accountId = 8803L;
        when(buyerGrpcClient.resolvePrincipals(List.of(buyerId))).thenReturn(PrincipalResolution.allResolved(Map.of(buyerId, accountId)));

        // 刻意讓兩邊給出**不同**的名字：只有讀對來源才會拿到 buyer-service 那個。
        // 舊碼讀 AccountInfo.nickname —— 那個欄位在 production 從不被填
        // （accounts 表無此欄，domain.Account 無此欄，toAccountInfo 不設它），
        // 但單元測試的 mock 填得出來，於是「名字會顯示」這個假象被測試釘住了。
        when(accountGrpcClient.batchGetAccountInfo(List.of(accountId))).thenReturn(Map.of(accountId,
                AccountInfo.newBuilder().setAccountId(accountId)
                        .setNickname("這個值在 production 永遠不存在")
                        .setPhone("0911222333").build()));
        when(buyerGrpcClient.batchGetBuyerDisplayByBuyerIds(List.of(buyerId)))
                .thenReturn(Map.of(buyerId, BuyerDisplay.newBuilder().setDisplayName("真的名字").build()));

        SellerGetTemplateDetailResponse response = invoke(newService());

        TargetAccountInfo target = response.getData().getTargetAccounts(0);
        assertEquals("真的名字", target.getNickname(), "名字必須來自 buyer-service");
        assertEquals("0911222333", target.getPhone(), "電話仍來自 account-service");
    }
}
