package io.github.will413028.hawkr.media.service;

import io.github.will413028.hawkr.common.grpc.PrincipalResolution;
import io.github.will413028.hawkr.common.config.MediaUrlProperties;
import io.github.will413028.hawkr.common.obs.mq.ObsRabbitPublisher;
import io.github.will413028.hawkr.common.safefetch.SafeFetcher;
import io.github.will413028.hawkr.common.utils.code.AssetReferenceType;
import io.github.will413028.hawkr.common.utils.code.ImportAssetMediaStatus;
import io.github.will413028.hawkr.media.client.SellerGrpcClient;
import io.github.will413028.hawkr.media.entity.ImportAssetEntity;
import io.github.will413028.hawkr.media.exception.MediaServiceException;
import io.github.will413028.hawkr.media.repository.ImportAssetRepository;
import io.github.will413028.hawkr.media.repository.MediaAssetRepository;
import io.github.will413028.hawkr.media.repository.UploadSessionRepository;
import io.github.will413028.hawkr.media.storage.StorageService;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.DisplayName;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.api.extension.ExtendWith;
import org.mockito.InjectMocks;
import org.mockito.Mock;
import org.mockito.junit.jupiter.MockitoExtension;
import org.springframework.test.util.ReflectionTestUtils;
import org.springframework.transaction.PlatformTransactionManager;
import org.springframework.transaction.support.TransactionTemplate;

import java.util.Map;
import java.util.Optional;

import static org.assertj.core.api.Assertions.assertThatThrownBy;
import static org.mockito.Mockito.verifyNoInteractions;
import static org.mockito.Mockito.when;

/**
 * FinalizeImportAsset 的擁有者比對必須以 principal 為準（#2054）。
 *
 * <p>{@code import_asset.store_id} 是「下載當下」的賣家 store_id，Finalize 傳入的
 * 是「現在」的 store_id。賣家在 import job 存活期（預設 7 天）內刪店重開，兩者就會
 * 分岔——直接相等比對會把同一個賣家誤判成越權，回「import_asset 不存在」卡死匯入
 * 最終提交。正面對照組是同一支 service 的 {@code bindStagedAsset}
 * （{@link ImportAssetServiceBindStagedAssetOwnershipTest}）。</p>
 *
 * <p>本測試刻意用「不相等」的 store_id 值：grandfather 賣家 store_id == account_id，
 * 用相等值測等於什麼都沒測，正是這個 bug 長期沒被發現的原因。</p>
 */
@ExtendWith(MockitoExtension.class)
class ImportAssetServiceFinalizeOwnershipTest {

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

    private static final String IMPORT_ASSET_ID = "8f1a1b2c-0000-4000-8000-000000000001";
    /** 下載當下的 store_id（賣家第一屆店）。 */
    private static final Long OLD_STORE_ID = 2683382637L;
    /** 刪店重開後的現任 store_id（同一個 account_id 的第二屆店）。 */
    private static final Long NEW_STORE_ID = 2685914682L;
    /** 兩屆店共同的登入 principal。 */
    private static final Long OWNER_ACCOUNT_ID = 2623900520L;
    /** 完全不相干的另一位賣家的 store_id / account_id。 */
    private static final Long OTHER_STORE_ID = 2688637900L;
    private static final Long OTHER_ACCOUNT_ID = 2639226113L;

    @BeforeEach
    void setUp() {
        // txTemplate 平時由 @PostConstruct verifyConfig() 建；純 Mockito 測試不跑
        // Spring 生命週期，直接注入等價實例（PlatformTransactionManager 是 mock，
        // getTransaction/commit/rollback 皆 no-op，例外照樣往外拋）。
        ReflectionTestUtils.setField(importAssetService, "txTemplate",
            new TransactionTemplate(transactionManager));
    }

    private ImportAssetEntity uploadedAssetOwnedBy(Long ownerId) {
        ImportAssetEntity entity = new ImportAssetEntity();
        entity.setId(IMPORT_ASSET_ID);
        entity.setStoreId(ownerId);
        // 刻意停在 DOWNLOADING：擁有者檢查是 prepareFinalize 的第一道關卡，通過後
        // 下一道狀態檢查會丟出可辨識的 failedPrecondition 訊息，藉此觀察「有沒有
        // 走過越權關卡」，不必把整條 OSS 搬移鏈路都 mock 起來。
        entity.setStatus(ImportAssetMediaStatus.DOWNLOADING.value());
        return entity;
    }

    @Test
    @DisplayName("分岔賣家：舊 store_id 擁有的 import_asset，用新 store_id 提交仍通過越權檢查")
    void divergedStoreStint_resolvesToSamePrincipal_passesOwnershipCheck() {
        when(importAssetRepository.findByIdForUpdate(IMPORT_ASSET_ID))
            .thenReturn(Optional.of(uploadedAssetOwnedBy(OLD_STORE_ID)));
        when(sellerGrpcClient.resolvePrincipals(java.util.List.of(OLD_STORE_ID, NEW_STORE_ID)))
            .thenReturn(PrincipalResolution.allResolved(Map.of(OLD_STORE_ID, OWNER_ACCOUNT_ID, NEW_STORE_ID, OWNER_ACCOUNT_ID)));

        assertThatThrownBy(() -> importAssetService.finalizeImportAsset(
            IMPORT_ASSET_ID, AssetReferenceType.PRODUCT.value(), NEW_STORE_ID, 777L))
            .isInstanceOf(MediaServiceException.class)
            // 走到狀態檢查 = 已通過越權檢查；若誤判越權會是「import_asset 不存在」
            .hasMessageContaining("尚未下載完成");
    }

    @Test
    @DisplayName("真正越權：另一位賣家的 store_id 解出不同 principal → 仍拒絕")
    void differentSeller_stillRejected() {
        when(importAssetRepository.findByIdForUpdate(IMPORT_ASSET_ID))
            .thenReturn(Optional.of(uploadedAssetOwnedBy(OLD_STORE_ID)));
        when(sellerGrpcClient.resolvePrincipals(java.util.List.of(OLD_STORE_ID, OTHER_STORE_ID)))
            .thenReturn(PrincipalResolution.allResolved(Map.of(OLD_STORE_ID, OWNER_ACCOUNT_ID, OTHER_STORE_ID, OTHER_ACCOUNT_ID)));

        assertThatThrownBy(() -> importAssetService.finalizeImportAsset(
            IMPORT_ASSET_ID, AssetReferenceType.PRODUCT.value(), OTHER_STORE_ID, 777L))
            .isInstanceOf(MediaServiceException.class)
            .hasMessageContaining("import_asset 不存在");
    }

    @Test
    @DisplayName("同一屆店（快路徑）：兩值相同時不打 seller-service")
    void sameStoreStint_takesFastPath_withoutRpc() {
        when(importAssetRepository.findByIdForUpdate(IMPORT_ASSET_ID))
            .thenReturn(Optional.of(uploadedAssetOwnedBy(NEW_STORE_ID)));

        assertThatThrownBy(() -> importAssetService.finalizeImportAsset(
            IMPORT_ASSET_ID, AssetReferenceType.PRODUCT.value(), NEW_STORE_ID, 777L))
            .isInstanceOf(MediaServiceException.class)
            .hasMessageContaining("尚未下載完成");

        verifyNoInteractions(sellerGrpcClient);
    }

    // ==================== 未解析 principal ⇒ fail-closed（issue #2789 階段 B1） ====================
    //
    // seller-service 對「查無在架列」的 store_id（帳號刪除時寫入 deleted_at，或 id 根本不存在）
    // 回 code=0 並把輸入<b>原樣回吐</b>進 principals。舊寫法 getOrDefault(id, id) 於是拿
    // 一個 store_id 空間的值去跟另一側 account_id 空間的值比相等 —— 那已經不是身分比對，
    // 而這一關放行的後果是拿得到別人的 import_asset。

    private static PrincipalResolution echoedBack(java.util.Map<Long, Long> principals, Long... unresolved) {
        return PrincipalResolution.of(principals, java.util.List.of(unresolved));
    }

    @Test
    @DisplayName("擁有者 store_id 未解析、回吐值恰好等於呼叫者 principal → 舊碼會放行，fail-closed 後拒絕")
    void ownerUnresolved_echoCollidesWithCallerPrincipal_deniedAfterFailClosed() {
        // 這是 echo-back 在<b>今天</b>唯一會放行「非快路徑」比對的形狀，也是本組唯一
        // 對「換回 getOrDefault」有鑑別力的一支（實測：換回去只有這一支轉紅）：
        // owner 側未解析 ⇒ 回吐 OLD_STORE_ID；caller 側真的解析到、而其 account_id
        // 恰好就是 OLD_STORE_ID 這個數值（grandfather 期間 store_id == account_id，
        // 兩個 id 空間是重疊的）。舊碼 ownerPrincipal(=OLD_STORE_ID).equals(callerPrincipal)
        // 成立 ⇒ 放行；fail-closed 之後一律拒絕。
        when(importAssetRepository.findByIdForUpdate(IMPORT_ASSET_ID))
            .thenReturn(Optional.of(uploadedAssetOwnedBy(OLD_STORE_ID)));
        when(sellerGrpcClient.resolvePrincipals(java.util.List.of(OLD_STORE_ID, OTHER_STORE_ID)))
            .thenReturn(echoedBack(Map.of(OLD_STORE_ID, OLD_STORE_ID, OTHER_STORE_ID, OLD_STORE_ID),
                OLD_STORE_ID));

        assertThatThrownBy(() -> importAssetService.finalizeImportAsset(
            IMPORT_ASSET_ID, AssetReferenceType.PRODUCT.value(), OTHER_STORE_ID, 777L))
            .isInstanceOf(MediaServiceException.class)
            // 越權防枚舉：與「不存在」同一個回應，不洩漏線索
            .hasMessageContaining("import_asset 不存在");
    }

    /**
     * 契約釘樁（非鑑別力測試）：把產品碼換回 echo-back 這一支<b>仍然綠</b>——兩個不同的
     * store_id 各自回吐後本來就不相等。它擋的是另一個方向的回歸：若日後把未解析改成
     * {@code return 0}（通知類站點的取捨），雙 0 會讓 {@code 0 == 0} 成立 ⇒ 兩個毫不相干
     * 的賣家被判成同一人 ⇒ 真正的越權放行。這一支就是那條路的攔截線。
     */
    @Test
    @DisplayName("兩個 store_id 同時未解析 → 拒絕（不拿兩個未驗證的回吐值互比）")
    void bothStoresUnresolved_denied() {
        when(importAssetRepository.findByIdForUpdate(IMPORT_ASSET_ID))
            .thenReturn(Optional.of(uploadedAssetOwnedBy(OLD_STORE_ID)));
        when(sellerGrpcClient.resolvePrincipals(java.util.List.of(OLD_STORE_ID, NEW_STORE_ID)))
            .thenReturn(echoedBack(Map.of(OLD_STORE_ID, OLD_STORE_ID, NEW_STORE_ID, NEW_STORE_ID),
                OLD_STORE_ID, NEW_STORE_ID));

        assertThatThrownBy(() -> importAssetService.finalizeImportAsset(
            IMPORT_ASSET_ID, AssetReferenceType.PRODUCT.value(), NEW_STORE_ID, 777L))
            .isInstanceOf(MediaServiceException.class)
            .hasMessageContaining("import_asset 不存在");
    }

    /**
     * 契約釘樁（非鑑別力測試）：echo-back 與 {@code return 0} 兩種舊寫法在這個輸入下都會
     * 拒絕，本支只是把「未解析一律拒絕」寫成可執行的契約，避免日後有人為了讓分岔賣家
     * 通過而放寬成「只要有一側解析得到就比對」。
     */
    @Test
    @DisplayName("呼叫者 store_id 未解析（擁有者正常）→ 拒絕")
    void callerUnresolved_denied() {
        when(importAssetRepository.findByIdForUpdate(IMPORT_ASSET_ID))
            .thenReturn(Optional.of(uploadedAssetOwnedBy(OLD_STORE_ID)));
        when(sellerGrpcClient.resolvePrincipals(java.util.List.of(OLD_STORE_ID, NEW_STORE_ID)))
            .thenReturn(echoedBack(Map.of(OLD_STORE_ID, OWNER_ACCOUNT_ID, NEW_STORE_ID, NEW_STORE_ID),
                NEW_STORE_ID));

        assertThatThrownBy(() -> importAssetService.finalizeImportAsset(
            IMPORT_ASSET_ID, AssetReferenceType.PRODUCT.value(), NEW_STORE_ID, 777L))
            .isInstanceOf(MediaServiceException.class)
            .hasMessageContaining("import_asset 不存在");
    }
}
