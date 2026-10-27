package io.github.will413028.hawkr.media.service;

import io.github.will413028.hawkr.common.grpc.PrincipalResolution;
import io.github.will413028.hawkr.common.utils.code.AssetReferenceType;
import io.github.will413028.hawkr.common.utils.code.MediaUploadSessionStatus;
import io.github.will413028.hawkr.media.client.SellerGrpcClient;
import io.github.will413028.hawkr.media.dto.StagedTokenClaims;
import io.github.will413028.hawkr.media.entity.UploadSessionEntity;
import io.github.will413028.hawkr.media.exception.MediaServiceException;
import io.github.will413028.hawkr.media.repository.ImportAssetRepository;
import io.github.will413028.hawkr.media.repository.MediaAssetRepository;
import io.github.will413028.hawkr.media.repository.UploadSessionRepository;
import io.github.will413028.hawkr.media.storage.StorageService;
import io.github.will413028.hawkr.common.obs.mq.ObsRabbitPublisher;
import io.github.will413028.hawkr.common.config.MediaUrlProperties;
import io.github.will413028.hawkr.common.safefetch.SafeFetcher;
import org.junit.jupiter.api.DisplayName;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.api.extension.ExtendWith;
import org.mockito.InjectMocks;
import org.mockito.Mock;
import org.mockito.junit.jupiter.MockitoExtension;
import org.springframework.test.util.ReflectionTestUtils;
import org.springframework.transaction.PlatformTransactionManager;

import java.time.LocalDateTime;
import java.util.Map;
import java.util.Optional;
import java.util.UUID;

import static org.assertj.core.api.Assertions.assertThat;
import static org.assertj.core.api.Assertions.assertThatThrownBy;
import static org.mockito.ArgumentMatchers.anyList;
import static org.mockito.ArgumentMatchers.anyLong;
import static org.mockito.ArgumentMatchers.eq;
import static org.mockito.Mockito.never;
import static org.mockito.Mockito.verify;
import static org.mockito.Mockito.verifyNoInteractions;
import static org.mockito.Mockito.when;

/**
 * BindStagedAsset 越權防呆（40 號設計書 §3.4/§4.3）必須以 store_id 解析回 raw
 * account_id 後再比對——product-import-service 呼叫時傳入的是賣家的 resolved
 * store_id（見 ItemMutationService.update: mediaClient.bindStagedAsset(...,
 * storeId, ...)），但 staged upload token 的 claims.accountId 是上傳當下
 * GrpcAuthContext.requireAccountId() 取得的 raw account_id。分岔賣家
 * （store_id != account_id）直接相等比對會誤判為越權，回應與「token 不存在」
 * 一致的 404，實際上是同一個賣家。
 */
@ExtendWith(MockitoExtension.class)
class ImportAssetServiceBindStagedAssetOwnershipTest {

    @Mock ImportAssetRepository importAssetRepository;
    @Mock MediaAssetRepository mediaAssetRepository;
    @Mock UploadSessionRepository uploadSessionRepository;
    @Mock StorageService storageService;
    @Mock SafeFetcher safeFetcher;
    @Mock StagedTokenService stagedTokenService;
    @Mock ObsRabbitPublisher obsRabbitPublisher;
    @Mock MediaUrlProperties mediaUrlProperties;
    @Mock PlatformTransactionManager transactionManager;
    @Mock SellerGrpcClient sellerGrpcClient;

    @InjectMocks ImportAssetService importAssetService;

    private static final Long DIVERGED_STORE_ID = 555L;
    private static final Long REAL_OWNER_ACCOUNT_ID = 100L;

    private StagedTokenClaims claimsWithAccount(Long accountId, boolean isSecure) {
        return claimsWithAccount(accountId, isSecure, UUID.randomUUID());
    }

    private StagedTokenClaims claimsWithAccount(Long accountId, boolean isSecure, UUID sessionId) {
        return StagedTokenClaims.builder()
                .sessionId(sessionId)
                .accountId(accountId)
                .expiresAt(LocalDateTime.now().plusMinutes(10))
                .isSecure(isSecure)
                .build();
    }

    @Test
    @DisplayName("分岔賣家：product-import-service 傳入的 store_id 解析回同一個 raw account_id → 通過越權檢查")
    void divergedSeller_resolvesToSameOwner_passesOwnershipCheck() {
        when(stagedTokenService.parseAndValidate("token"))
                .thenReturn(claimsWithAccount(REAL_OWNER_ACCOUNT_ID, true));
        when(sellerGrpcClient.resolvePrincipals(anyList()))
                .thenReturn(PrincipalResolution.allResolved(Map.of(DIVERGED_STORE_ID, REAL_OWNER_ACCOUNT_ID)));

        // isSecure=true 讓流程在越權檢查後的下一步丟出「機敏檔案不支援」，藉此觀察
        // 是否已通過越權檢查（若越權檢查誤判，會先丟出「staged_token 無效或不存在」）。
        assertThatThrownBy(() -> importAssetService.bindStagedAsset(
                "token", DIVERGED_STORE_ID, 1L, 1L, "7116"))
                .isInstanceOf(MediaServiceException.class)
                .hasMessageContaining("機敏檔案不支援");
    }

    @Test
    @DisplayName("真正越權：resolve 出的擁有者與 token 帳號不同 → 仍拒絕")
    void trueMismatch_stillRejected() {
        when(stagedTokenService.parseAndValidate("token"))
                .thenReturn(claimsWithAccount(999L, false)); // 999 不是 DIVERGED_STORE_ID 的真正擁有者
        when(sellerGrpcClient.resolvePrincipals(anyList()))
                .thenReturn(PrincipalResolution.allResolved(Map.of(DIVERGED_STORE_ID, REAL_OWNER_ACCOUNT_ID)));

        assertThatThrownBy(() -> importAssetService.bindStagedAsset(
                "token", DIVERGED_STORE_ID, 1L, 1L, "7116"))
                .isInstanceOf(MediaServiceException.class)
                .hasMessageContaining("staged_token 無效或不存在");
    }

    // ==================== upload_session 查詢鍵（#2054 孿生缺口）====================
    // 越權檢查通過之後，階段 3 還要用 session_id + 帳號查 upload_session。該表的
    // account_id 是上傳當下 GrpcAuthContext.requireAccountId() 寫入的 raw account_id，
    // 拿 store_id 去查對分岔賣家永遠零筆 —— 剛通過上面的 principal 比對，卻立刻收到
    // 「upload_session 不存在或無權存取」。查詢鍵必須是解析後的 principal。

    private static final UUID SESSION_ID = UUID.fromString("11111111-2222-4333-8444-555555555555");

    private UploadSessionEntity completedSessionOwnedBy(Long ownerAccountId) {
        UploadSessionEntity session = new UploadSessionEntity();
        session.setId(SESSION_ID);
        session.setAccountId(ownerAccountId);
        session.setObjectKey("upload/20260731/cabbage.jpg");
        session.setIsSecure(false);
        session.setContentType("image/jpeg");
        session.setFileSize(2048L);
        session.setStatus(MediaUploadSessionStatus.COMPLETED.value());
        session.setExpiresAt(LocalDateTime.now().plusDays(1));
        return session;
    }

    @Test
    @DisplayName("分岔賣家：upload_session 以 principal 查得到 → 整支 bind 走完並回傳新 import_asset")
    void divergedSeller_looksUpUploadSessionByPrincipal_bindSucceeds() {
        ReflectionTestUtils.setField(importAssetService, "tempPrefix", "tmp");
        when(stagedTokenService.parseAndValidate("token"))
                .thenReturn(claimsWithAccount(REAL_OWNER_ACCOUNT_ID, false, SESSION_ID));
        when(sellerGrpcClient.resolvePrincipals(anyList()))
                .thenReturn(PrincipalResolution.allResolved(Map.of(DIVERGED_STORE_ID, REAL_OWNER_ACCOUNT_ID)));
        // dedup 鍵刻意仍是 store_id：import_asset.store_id 全表存 store_id（見產品碼註解）
        when(importAssetRepository.findByStagedDedupKey(
                DIVERGED_STORE_ID, 42L, SESSION_ID.toString()))
                .thenReturn(Optional.empty());
        // 模擬真實 DB：upload_session.account_id 只認 raw account_id，用 store_id 查回零筆。
        // 產品碼若退回用 store_id 查，就會走到「upload_session 不存在或無權存取」——正是
        // 這個孿生缺口對分岔賣家造成的實際症狀。
        when(uploadSessionRepository.selectByIdAndAccountId(eq(SESSION_ID), anyLong()))
                .thenAnswer(inv -> REAL_OWNER_ACCOUNT_ID.equals(inv.getArgument(1))
                        ? completedSessionOwnedBy(REAL_OWNER_ACCOUNT_ID)
                        : null);

        ImportAssetService.BindResult result = importAssetService.bindStagedAsset(
                "token", DIVERGED_STORE_ID, 42L, 7L, AssetReferenceType.PRODUCT_IMPORT_PREVIEW.value());

        assertThat(result).isNotNull();
        assertThat(result.reused()).isFalse();
        assertThat(result.objectKey()).startsWith("tmp/").contains("/import/").endsWith(".jpg");
        // 查詢鍵必須是 principal，且絕不能用 store_id 再查一次
        verify(uploadSessionRepository).selectByIdAndAccountId(SESSION_ID, REAL_OWNER_ACCOUNT_ID);
        verify(uploadSessionRepository, never()).selectByIdAndAccountId(SESSION_ID, DIVERGED_STORE_ID);
    }

    @Test
    @DisplayName("他人越權：principal 不同 → 在越權關卡就擋掉，根本不會去查 upload_session")
    void differentSeller_rejectedBeforeUploadSessionLookup() {
        when(stagedTokenService.parseAndValidate("token"))
                .thenReturn(claimsWithAccount(999L, false, SESSION_ID));
        when(sellerGrpcClient.resolvePrincipals(anyList()))
                .thenReturn(PrincipalResolution.allResolved(Map.of(DIVERGED_STORE_ID, REAL_OWNER_ACCOUNT_ID)));

        assertThatThrownBy(() -> importAssetService.bindStagedAsset(
                "token", DIVERGED_STORE_ID, 42L, 7L, AssetReferenceType.PRODUCT_IMPORT_PREVIEW.value()))
                .isInstanceOf(MediaServiceException.class)
                .hasMessageContaining("staged_token 無效或不存在");

        verifyNoInteractions(uploadSessionRepository);
    }

    @Test
    @DisplayName("store_id 查無存活賣家（unresolved_ids）→ fail-closed 拒絕，不拿 store_id 冒充 account_id 比對")
    void unresolvedStore_isRejectedEvenWhenTheTokenAccountEqualsTheStoreId() {
        // 最壞的巧合：token 的帳號剛好等於 store_id。回吐 store_id 比對會因此放行。
        when(stagedTokenService.parseAndValidate("token"))
                .thenReturn(claimsWithAccount(DIVERGED_STORE_ID, false, SESSION_ID));
        when(sellerGrpcClient.resolvePrincipals(anyList()))
                .thenReturn(PrincipalResolution.of(Map.of(DIVERGED_STORE_ID, DIVERGED_STORE_ID), java.util.List.of(DIVERGED_STORE_ID)));

        assertThatThrownBy(() -> importAssetService.bindStagedAsset(
                "token", DIVERGED_STORE_ID, 42L, 7L, AssetReferenceType.PRODUCT_IMPORT_PREVIEW.value()))
                .isInstanceOf(MediaServiceException.class)
                .hasMessageContaining("staged_token 無效或不存在");

        verifyNoInteractions(uploadSessionRepository);
    }
}
