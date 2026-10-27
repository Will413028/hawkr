package io.github.will413028.hawkr.media.service;

import io.github.will413028.hawkr.common.config.MediaUrlProperties;
import io.github.will413028.hawkr.common.utils.TsidUtil;
import io.github.will413028.hawkr.common.utils.code.MediaUploadSessionStatus;
import io.github.will413028.hawkr.media.dto.FinalizedAssetDTO;
import io.github.will413028.hawkr.media.dto.StagedTokenClaims;
import io.github.will413028.hawkr.media.entity.MediaAssetEntity;
import io.github.will413028.hawkr.media.entity.UploadSessionEntity;
import io.github.will413028.hawkr.media.exception.MediaServiceException;
import io.github.will413028.hawkr.media.repository.MediaAssetRepository;
import io.github.will413028.hawkr.media.repository.UploadSessionRepository;
import org.junit.jupiter.api.BeforeAll;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.DisplayName;
import org.junit.jupiter.api.Nested;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.api.extension.ExtendWith;
import org.mockito.InjectMocks;
import org.mockito.Mock;
import org.mockito.Spy;
import org.mockito.junit.jupiter.MockitoExtension;

import io.github.will413028.hawkr.common.utils.code.AssetReferenceType;
import io.github.will413028.hawkr.common.utils.code.StagedTokenScope;
import io.github.will413028.hawkr.media.storage.StorageObjectStat;

import io.github.will413028.hawkr.common.utils.DateTimeUtils;
import java.util.List;
import java.util.UUID;

import static org.junit.jupiter.api.Assertions.*;
import static org.mockito.ArgumentMatchers.*;
import static org.mockito.Mockito.*;

/**
 * MediaAssetService 單元測試
 * <p>
 * 測試重點：
 * 1. FinalizeImages 的重入/冪等行為
 * 2. 狀態機轉換驗證
 * 3. Owner 權限驗證
 */
@ExtendWith(MockitoExtension.class)
class MediaAssetServiceTest {

    /**
     * TsidUtil 是靜態單例，MediaAssetService 直接呼叫 TsidUtil.generate()（未透過 Spring
     * context，本測試也不啟 Spring context）。全 reactor 跑法下若本測試是本模組 JVM 第一個
     * 觸碰 TsidUtil 的測試就會因未初始化而炸；顯式在此 init 一次，並用 isInitialized() 防止
     * 重複初始化拋例外（surefire 同 fork 內多個測試類共用同一 JVM）。
     */
    @BeforeAll
    static void initTsid() {
        if (!TsidUtil.isInitialized()) {
            TsidUtil.init(100, 0);
        }
    }

    @Mock
    private MediaAssetRepository mediaAssetRepository;

    @Mock
    private UploadSessionRepository uploadSessionRepository;

    @Mock
    private StagedTokenService stagedTokenService;

    @Mock
    private io.github.will413028.hawkr.media.storage.StorageService storageService;

    @Mock
    private io.github.will413028.hawkr.media.converter.MediaAssetConverter mediaAssetConverter;

    @Mock
    private MediaUrlProperties mediaUrlProperties;

    @Mock
    private MediaUploadValidator uploadValidator;

    /**
     * 機敏檔的加解密（{@code serverUploadSecureAndFinalize} / {@code serverReadSecureAsset}）。
     *
     * <p>🔴 **用真的、帶主密鑰的實例，不用 @Mock**（#3024）：mock 的
     * {@code isAvailable()} 預設回 false，而上傳路徑在主密鑰不可用時是 fail loud
     * ——本檔兩支機敏上傳測試會因此炸在一個與它們要驗的事完全無關的地方。
     * 真的 cipher 也讓「上傳的到底是不是密文」變成可驗的（那一格在
     * {@code ServerUploadSecureEncryptionTest}）。
     */
    @Spy
    private io.github.will413028.hawkr.media.crypto.AesStreamCipher aesStreamCipher =
            new io.github.will413028.hawkr.media.crypto.AesStreamCipher(
                    java.util.Base64.getEncoder().encodeToString(
                            "0123456789abcdef0123456789abcdef".getBytes(java.nio.charset.StandardCharsets.UTF_8)));

    /**
     * 轉正流程把儲存端操作切到事務外（見 MediaAssetService 類別註解），各階段自行以
     * TransactionTemplate 控制事務；mock 的 PlatformTransactionManager 讓 execute 直通 callback。
     */
    /** 機敏檔直傳要在同一次呼叫內建立 asset_reference（58 §8.6 T-03）。 */
    @Mock
    private AssetReferenceService assetReferenceService;

    @Mock
    private org.springframework.transaction.PlatformTransactionManager transactionManager;

    @InjectMocks
    private MediaAssetService mediaAssetService;

    private static final Long ACCOUNT_ID = 12345L;
    private static final UUID SESSION_ID = UUID.randomUUID();
    private static final Long ASSET_ID = 123456789L;
    private static final String OBJECT_KEY = "tmp/20260129/test.jpg";
    private static final String FINAL_KEY = "public/product/12345/20260129/test.jpg";
    private static final String STAGED_TOKEN = "test-staged-token";
    private static final String REF_TYPE = AssetReferenceType.PRODUCT.value();

    @Nested
    @DisplayName("FinalizeImages 測試")
    class FinalizeImagesTests {

        private UploadSessionEntity session;
        private StagedTokenClaims claims;

        @BeforeEach
        void setUp() {
            session = new UploadSessionEntity();
            session.setId(SESSION_ID);
            session.setAccountId(ACCOUNT_ID);
            session.setObjectKey(OBJECT_KEY);
            session.setIsSecure(false);
            session.setStatus(MediaUploadSessionStatus.COMPLETED.value());
            session.setExpiresAt(DateTimeUtils.nowUtc().plusMinutes(30));

            claims = StagedTokenClaims.builder()
                .sessionId(SESSION_ID)
                .accountId(ACCOUNT_ID)
                .isSecure(false)
                .scope(StagedTokenScope.MEDIA_UPLOAD_SESSION.value())
                .expiresAt(DateTimeUtils.nowUtc().plusMinutes(30))
                .build();
        }

        @Test
        @DisplayName("正常流程：COMPLETED → FINALIZED")
        void shouldFinalizeSuccessfully() {
            // Given
            when(stagedTokenService.parseAndValidate(STAGED_TOKEN)).thenReturn(claims);
            when(uploadSessionRepository.selectById(SESSION_ID)).thenReturn(session);
            when(uploadSessionRepository.updateStatusWithCondition(
                eq(SESSION_ID),
                eq(MediaUploadSessionStatus.COMPLETED.value()),
                eq(MediaUploadSessionStatus.FINALIZED.value())
            )).thenReturn(1);
            when(mediaAssetRepository.insert(any())).thenReturn(1);

            FinalizedAssetDTO expectedDto = FinalizedAssetDTO.builder()
                .stagedToken(STAGED_TOKEN)
                .assetId(String.valueOf(ASSET_ID))
                .objectKey(OBJECT_KEY)
                .isSecure(false)
                .build();
            when(mediaAssetConverter.toFinalizedDTO(any(), eq(STAGED_TOKEN))).thenReturn(expectedDto);

            // When
            List<FinalizedAssetDTO> results = mediaAssetService.finalizeImages(
                List.of(STAGED_TOKEN),
                ACCOUNT_ID,
                REF_TYPE
            );

            // Then
            assertNotNull(results);
            assertEquals(1, results.size());
            assertEquals(STAGED_TOKEN, results.get(0).getStagedToken());

            verify(uploadSessionRepository).updateStatusWithCondition(
                SESSION_ID,
                MediaUploadSessionStatus.COMPLETED.value(),
                MediaUploadSessionStatus.FINALIZED.value()
            );
            verify(mediaAssetRepository).insert(any());
        }

        /**
         * #3149 W1：轉正時 {@code media_asset.original_filename} 必須從 session 複製過來。
         * 漏了這一行，整條保存鏈在最後一步斷掉——session 存了、asset 沒有，而讀取端讀的是
         * asset ⇒ 使用者看到的仍是 objectKey 尾段那串 uuid。
         */
        @Test
        @DisplayName("#3149：轉正時 asset.original_filename 來自 session（不再 sanitize 第二次）")
        void copiesOriginalFilenameFromSession() {
            // 與 objectKey 尾段（test.jpg）刻意不同；session 裡的值建立會話時已消毒過
            session.setOriginalFilename("使用者的照片.jpg");

            when(stagedTokenService.parseAndValidate(STAGED_TOKEN)).thenReturn(claims);
            when(uploadSessionRepository.selectById(SESSION_ID)).thenReturn(session);
            when(uploadSessionRepository.updateStatusWithCondition(
                eq(SESSION_ID),
                eq(MediaUploadSessionStatus.COMPLETED.value()),
                eq(MediaUploadSessionStatus.FINALIZED.value())
            )).thenReturn(1);

            mediaAssetService.finalizeImages(List.of(STAGED_TOKEN), ACCOUNT_ID, REF_TYPE);

            org.mockito.ArgumentCaptor<MediaAssetEntity> inserted =
                    org.mockito.ArgumentCaptor.forClass(MediaAssetEntity.class);
            verify(mediaAssetRepository).insert(inserted.capture());
            assertEquals("使用者的照片.jpg", inserted.getValue().getOriginalFilename());
        }

        @Test
        @DisplayName("#3149：session 的 original_filename 為 NULL 時 asset 亦為 NULL（讀取端回退）")
        void leavesOriginalFilenameNullWhenSessionHasNone() {
            session.setOriginalFilename(null);

            when(stagedTokenService.parseAndValidate(STAGED_TOKEN)).thenReturn(claims);
            when(uploadSessionRepository.selectById(SESSION_ID)).thenReturn(session);
            when(uploadSessionRepository.updateStatusWithCondition(
                eq(SESSION_ID),
                eq(MediaUploadSessionStatus.COMPLETED.value()),
                eq(MediaUploadSessionStatus.FINALIZED.value())
            )).thenReturn(1);

            mediaAssetService.finalizeImages(List.of(STAGED_TOKEN), ACCOUNT_ID, REF_TYPE);

            org.mockito.ArgumentCaptor<MediaAssetEntity> inserted =
                    org.mockito.ArgumentCaptor.forClass(MediaAssetEntity.class);
            verify(mediaAssetRepository).insert(inserted.capture());
            // NULL 是合法終態，不是待修復的狀態（§4.9.7）——不得在這裡塞一個空字串或 uuid
            assertNull(inserted.getValue().getOriginalFilename());
        }

        @Test
        @DisplayName("重入語意：已 FINALIZED 的 token 不報錯，回傳既有資產")
        void shouldReturnExistingAssetWhenAlreadyFinalized() {
            // Given - session 已經是 FINALIZED 狀態
            session.setStatus(MediaUploadSessionStatus.FINALIZED.value());

            when(stagedTokenService.parseAndValidate(STAGED_TOKEN)).thenReturn(claims);
            when(uploadSessionRepository.selectById(SESSION_ID)).thenReturn(session);

            MediaAssetEntity existingAsset = new MediaAssetEntity();
            existingAsset.setId(ASSET_ID);
            existingAsset.setUploaderAccountId(ACCOUNT_ID);
            existingAsset.setObjectKey(OBJECT_KEY);
            existingAsset.setIsSecure(false);

            when(mediaAssetRepository.selectList(any())).thenReturn(List.of(existingAsset));

            FinalizedAssetDTO expectedDto = FinalizedAssetDTO.builder()
                .stagedToken(STAGED_TOKEN)
                .assetId(String.valueOf(ASSET_ID))
                .objectKey(OBJECT_KEY)
                .isSecure(false)
                .build();
            when(mediaAssetConverter.toFinalizedDTO(existingAsset, STAGED_TOKEN)).thenReturn(expectedDto);

            // When
            List<FinalizedAssetDTO> results = mediaAssetService.finalizeImages(
                List.of(STAGED_TOKEN),
                ACCOUNT_ID,
                REF_TYPE
            );

            // Then
            assertNotNull(results);
            assertEquals(1, results.size());
            assertEquals(String.valueOf(ASSET_ID), results.get(0).getAssetId());

            // 不應該嘗試更新狀態或新增資產
            verify(uploadSessionRepository, never()).updateStatusWithCondition(any(), any(), any());
            verify(mediaAssetRepository, never()).insert(any());
        }

        @Test
        @DisplayName("Owner 不匹配應拋出 PERMISSION_DENIED")
        void shouldThrowPermissionDeniedWhenOwnerMismatch() {
            // Given
            when(stagedTokenService.parseAndValidate(STAGED_TOKEN)).thenReturn(claims);

            // When & Then
            Long wrongOwnerId = 99999L;
            assertThrows(MediaServiceException.class, () -> {
                mediaAssetService.finalizeImages(
                    List.of(STAGED_TOKEN),
                    wrongOwnerId,
                    REF_TYPE
                );
            });
        }

        @Test
        @DisplayName("Session 不存在應拋出 INVALID_ARGUMENT")
        void shouldThrowInvalidArgumentWhenSessionNotFound() {
            // Given
            when(stagedTokenService.parseAndValidate(STAGED_TOKEN)).thenReturn(claims);
            when(uploadSessionRepository.selectById(SESSION_ID)).thenReturn(null);

            // When & Then
            assertThrows(MediaServiceException.class, () -> {
                mediaAssetService.finalizeImages(
                    List.of(STAGED_TOKEN),
                    ACCOUNT_ID,
                    REF_TYPE
                );
            });
        }

        @Test
        @DisplayName("Session 已過期應拋出 INVALID_ARGUMENT")
        void shouldThrowInvalidArgumentWhenSessionExpired() {
            // Given
            session.setExpiresAt(DateTimeUtils.nowUtc().minusMinutes(1)); // 已過期
            claims = StagedTokenClaims.builder()
                .sessionId(SESSION_ID)
                .accountId(ACCOUNT_ID)
                .isSecure(false)
                .scope(StagedTokenScope.MEDIA_UPLOAD_SESSION.value())
                .expiresAt(DateTimeUtils.nowUtc().minusMinutes(1))
                .build();

            when(stagedTokenService.parseAndValidate(STAGED_TOKEN)).thenReturn(claims);
            when(uploadSessionRepository.selectById(SESSION_ID)).thenReturn(session);

            // When & Then
            assertThrows(MediaServiceException.class, () -> {
                mediaAssetService.finalizeImages(
                    List.of(STAGED_TOKEN),
                    ACCOUNT_ID,
                    REF_TYPE
                );
            });
        }

        @Test
        @DisplayName("Session 狀態為 INIT 應拋出 INVALID_ARGUMENT")
        void shouldThrowInvalidArgumentWhenSessionStatusIsInit() {
            // Given
            session.setStatus(MediaUploadSessionStatus.INIT.value());

            when(stagedTokenService.parseAndValidate(STAGED_TOKEN)).thenReturn(claims);
            when(uploadSessionRepository.selectById(SESSION_ID)).thenReturn(session);

            // When & Then
            assertThrows(MediaServiceException.class, () -> {
                mediaAssetService.finalizeImages(
                    List.of(STAGED_TOKEN),
                    ACCOUNT_ID,
                    REF_TYPE
                );
            });
        }

        @Test
        @DisplayName("#1396：token scope 與 session isSecure 不一致 → 拒絕（防枚舉 notFound），不搬移不建資產")
        void shouldRejectWhenClaimsScopeMismatchSession() {
            // Given - session 是公開上傳，但 token 帶機敏 scope（偽造/竄改）
            claims = StagedTokenClaims.builder()
                .sessionId(SESSION_ID)
                .accountId(ACCOUNT_ID)
                .isSecure(true)
                .scope(StagedTokenScope.MEDIA_SECURE_UPLOAD.value())
                .expiresAt(DateTimeUtils.nowUtc().plusMinutes(30))
                .build();
            when(stagedTokenService.parseAndValidate(STAGED_TOKEN)).thenReturn(claims);
            when(uploadSessionRepository.selectById(SESSION_ID)).thenReturn(session);

            // When & Then
            assertThrows(MediaServiceException.class, () ->
                mediaAssetService.finalizeImages(List.of(STAGED_TOKEN), ACCOUNT_ID, REF_TYPE));

            verify(storageService, never()).copyObject(any(), any());
            verify(mediaAssetRepository, never()).insert(any());
        }

        @Test
        @DisplayName("#1396：session.account ≠ claims.account（caller 與 claims 一致）→ 拒絕（防枚舉 notFound），不搬移不建資產")
        void shouldRejectWhenSessionAccountDiffersFromClaims() {
            // Given - token account 與 caller 相符（過 step 2），但 server-side session 屬於他人（step 3.5 交叉檢查應攔）
            session.setAccountId(88888L);
            when(stagedTokenService.parseAndValidate(STAGED_TOKEN)).thenReturn(claims);
            when(uploadSessionRepository.selectById(SESSION_ID)).thenReturn(session);

            // When & Then - caller == claims.account == ACCOUNT_ID，但 session.account 不同
            assertThrows(MediaServiceException.class, () ->
                mediaAssetService.finalizeImages(List.of(STAGED_TOKEN), ACCOUNT_ID, REF_TYPE));

            verify(storageService, never()).copyObject(any(), any());
            verify(mediaAssetRepository, never()).insert(any());
        }

        @Test
        @DisplayName("49 §8.1：HEAD 回查不通過 → 刪除 tmp、拋出、不搬移不建資產")
        void shouldRejectAndCleanupWhenHeadCheckFails() {
            // Given - storage 已配置，HEAD 回查（validator）判定不合規
            when(stagedTokenService.parseAndValidate(STAGED_TOKEN)).thenReturn(claims);
            when(uploadSessionRepository.selectById(SESSION_ID)).thenReturn(session);
            when(storageService.isEnabled()).thenReturn(true);
            when(storageService.statObject(OBJECT_KEY))
                .thenReturn(new StorageObjectStat(999_999_999L, "image/jpeg"));
            doThrow(MediaServiceException.invalidArgument("實際檔案大小超過上限"))
                .when(uploadValidator).validateFinalizedObject(any(), any(), any(), anyBoolean());

            // When & Then
            assertThrows(MediaServiceException.class, () ->
                mediaAssetService.finalizeImages(List.of(STAGED_TOKEN), ACCOUNT_ID, REF_TYPE));

            // 清理暫存物件 + 未搬移 + 未建資產
            verify(storageService).deleteObject(OBJECT_KEY);
            verify(storageService, never()).copyObject(any(), any());
            verify(mediaAssetRepository, never()).insert(any());
        }

        @Test
        @DisplayName("正常流程（storage 啟用）：HEAD 回查通過 → 搬移 + 建資產")
        void shouldFinalizeWhenHeadCheckPasses() {
            // Given
            when(stagedTokenService.parseAndValidate(STAGED_TOKEN)).thenReturn(claims);
            when(uploadSessionRepository.selectById(SESSION_ID)).thenReturn(session);
            when(storageService.isEnabled()).thenReturn(true);
            when(storageService.statObject(OBJECT_KEY))
                .thenReturn(new StorageObjectStat(1024L, "image/jpeg"));
            when(storageService.generateFinalObjectKey(any(), anyBoolean(), any(), any()))
                .thenReturn("public/product/12345/20260129/test.jpg");
            when(uploadSessionRepository.updateStatusWithCondition(
                eq(SESSION_ID),
                eq(MediaUploadSessionStatus.COMPLETED.value()),
                eq(MediaUploadSessionStatus.FINALIZED.value())
            )).thenReturn(1);
            when(mediaAssetRepository.insert(any())).thenReturn(1);
            FinalizedAssetDTO expectedDto = FinalizedAssetDTO.builder()
                .stagedToken(STAGED_TOKEN).assetId(String.valueOf(ASSET_ID))
                .objectKey(OBJECT_KEY).isSecure(false).build();
            when(mediaAssetConverter.toFinalizedDTO(any(), eq(STAGED_TOKEN))).thenReturn(expectedDto);

            // When
            List<FinalizedAssetDTO> results = mediaAssetService.finalizeImages(
                List.of(STAGED_TOKEN), ACCOUNT_ID, REF_TYPE);

            // Then
            assertEquals(1, results.size());
            verify(uploadValidator).validateFinalizedObject(any(), any(), any(), anyBoolean());
            verify(storageService).copyObject(eq(OBJECT_KEY), any());
            verify(mediaAssetRepository).insert(any());
        }

        @Test
        @DisplayName("暫存物件在 DB commit 之後才刪：copy 與 delete 不再同一步")
        void shouldDeleteTmpOnlyAfterDbCommit() {
            when(stagedTokenService.parseAndValidate(STAGED_TOKEN)).thenReturn(claims);
            when(uploadSessionRepository.selectById(SESSION_ID)).thenReturn(session);
            when(storageService.generateFinalObjectKey(any(), anyBoolean(), any(), any()))
                .thenReturn(FINAL_KEY);
            when(uploadSessionRepository.updateStatusWithCondition(any(), any(), any())).thenReturn(1);
            when(mediaAssetRepository.insert(any())).thenReturn(1);
            when(mediaAssetConverter.toFinalizedDTO(any(), eq(STAGED_TOKEN)))
                .thenReturn(FinalizedAssetDTO.builder().stagedToken(STAGED_TOKEN).build());

            mediaAssetService.finalizeImages(List.of(STAGED_TOKEN), ACCOUNT_ID, REF_TYPE);

            org.mockito.InOrder inOrder = inOrder(storageService, mediaAssetRepository);
            inOrder.verify(storageService).copyObject(OBJECT_KEY, FINAL_KEY);
            inOrder.verify(mediaAssetRepository).insert(any());
            inOrder.verify(storageService).deleteObject(OBJECT_KEY);
        }

        @Test
        @DisplayName("並發輸掉 CAS 時不得再 INSERT —— 舊版會留下第二列指向同一 object_key")
        void shouldNotInsertWhenLosingStatusCas() {
            session.setAssetId(ASSET_ID);
            when(stagedTokenService.parseAndValidate(STAGED_TOKEN)).thenReturn(claims);
            when(uploadSessionRepository.selectById(SESSION_ID)).thenReturn(session);
            when(storageService.generateFinalObjectKey(any(), anyBoolean(), any(), any()))
                .thenReturn(FINAL_KEY);
            // CAS 落敗：另一個請求已搶先轉正
            when(uploadSessionRepository.updateStatusWithCondition(any(), any(), any())).thenReturn(0);

            MediaAssetEntity winner = new MediaAssetEntity();
            winner.setId(ASSET_ID);
            winner.setObjectKey(FINAL_KEY);
            when(mediaAssetRepository.selectById(ASSET_ID)).thenReturn(winner);
            when(mediaAssetConverter.toFinalizedDTO(winner, STAGED_TOKEN))
                .thenReturn(FinalizedAssetDTO.builder()
                    .stagedToken(STAGED_TOKEN).assetId(String.valueOf(ASSET_ID)).build());

            List<FinalizedAssetDTO> results =
                mediaAssetService.finalizeImages(List.of(STAGED_TOKEN), ACCOUNT_ID, REF_TYPE);

            assertEquals(1, results.size());
            assertEquals(String.valueOf(ASSET_ID), results.get(0).getAssetId());
            verify(mediaAssetRepository, never()).insert(any());
            // 贏家與自己複製到同一個 finalObjectKey，補償刪除會刪掉贏家的物件，故不得刪
            verify(storageService, never()).deleteObject(FINAL_KEY);
        }

        @Test
        @DisplayName("多 token 中途失敗：已複製到正式路徑的物件必須補償刪除，不得留成 R2 孤兒")
        void shouldCompensateAlreadyCopiedObjectsWhenLaterTokenFails() {
            UUID sessionId2 = UUID.randomUUID();
            String token2 = "second-staged-token";
            String objectKey2 = "tmp/20260129/second.jpg";
            String finalKey2 = "public/product/12345/20260129/second.jpg";

            UploadSessionEntity session2 = new UploadSessionEntity();
            session2.setId(sessionId2);
            session2.setAccountId(ACCOUNT_ID);
            session2.setObjectKey(objectKey2);
            session2.setIsSecure(false);
            session2.setStatus(MediaUploadSessionStatus.COMPLETED.value());
            session2.setExpiresAt(DateTimeUtils.nowUtc().plusMinutes(30));

            StagedTokenClaims claims2 = StagedTokenClaims.builder()
                .sessionId(sessionId2)
                .accountId(ACCOUNT_ID)
                .isSecure(false)
                .scope(StagedTokenScope.MEDIA_UPLOAD_SESSION.value())
                .expiresAt(DateTimeUtils.nowUtc().plusMinutes(30))
                .build();

            when(stagedTokenService.parseAndValidate(STAGED_TOKEN)).thenReturn(claims);
            when(stagedTokenService.parseAndValidate(token2)).thenReturn(claims2);
            when(uploadSessionRepository.selectById(SESSION_ID)).thenReturn(session);
            when(uploadSessionRepository.selectById(sessionId2)).thenReturn(session2);
            when(storageService.generateFinalObjectKey(eq(OBJECT_KEY), anyBoolean(), any(), any()))
                .thenReturn(FINAL_KEY);
            when(storageService.generateFinalObjectKey(eq(objectKey2), anyBoolean(), any(), any()))
                .thenReturn(finalKey2);
            // 第一張搬移成功、第二張失敗（兩組參數都要 stub，否則嚴格模式會在第一次呼叫就判參數不符）
            doNothing().when(storageService).copyObject(OBJECT_KEY, FINAL_KEY);
            doThrow(new RuntimeException("R2 copy failed"))
                .when(storageService).copyObject(objectKey2, finalKey2);
            when(mediaAssetRepository.countByObjectKey(FINAL_KEY)).thenReturn(0);

            assertThrows(RuntimeException.class, () ->
                mediaAssetService.finalizeImages(List.of(STAGED_TOKEN, token2), ACCOUNT_ID, REF_TYPE));

            // 第一張已經複製到正式路徑 → 必須刪掉，否則就是 media_asset 認不得的 R2 孤兒
            verify(storageService).deleteObject(FINAL_KEY);
            // 暫存不刪（留給重試 / staging GC），也沒有任何資產落庫
            verify(storageService, never()).deleteObject(OBJECT_KEY);
            verify(mediaAssetRepository, never()).insert(any());
        }

        @Test
        @DisplayName("補償刪除前先確認沒有其他 media_asset 列指向同一物件（並發贏家保護）")
        void shouldSkipCompensationWhenAnotherAssetOwnsTheObject() {
            when(stagedTokenService.parseAndValidate(STAGED_TOKEN)).thenReturn(claims);
            when(uploadSessionRepository.selectById(SESSION_ID)).thenReturn(session);
            when(storageService.generateFinalObjectKey(any(), anyBoolean(), any(), any()))
                .thenReturn(FINAL_KEY);
            when(uploadSessionRepository.updateStatusWithCondition(any(), any(), any()))
                .thenThrow(new RuntimeException("db down"));
            when(mediaAssetRepository.countByObjectKey(FINAL_KEY)).thenReturn(1);

            assertThrows(RuntimeException.class, () ->
                mediaAssetService.finalizeImages(List.of(STAGED_TOKEN), ACCOUNT_ID, REF_TYPE));

            verify(storageService, never()).deleteObject(FINAL_KEY);
        }
    }

    @Nested
    @DisplayName("ServerUploadAndFinalize 測試")
    class ServerUploadAndFinalizeTests {

        @Test
        @DisplayName("建列失敗 → 補償刪除剛上傳的物件")
        void shouldDeleteUploadedObjectWhenInsertFails() {
            when(mediaAssetRepository.insert(any())).thenThrow(new RuntimeException("insert failed"));

            assertThrows(RuntimeException.class, () -> mediaAssetService.serverUploadAndFinalize(
                new byte[]{1, 2, 3}, "S1.pdf", "application/pdf", AssetReferenceType.SHIPPING.value()));

            verify(storageService).uploadStream(any(), any(), any(), anyLong());
            verify(storageService).deleteObject(any());
        }

        /**
         * #3149 §4.9.3〈作用範圍〉：同一個 filename 在這一支有<b>兩個不同的去處</b>——
         * object_key 末段吃<b>原值</b>，只有 original_filename 走消毒。
         *
         * <p>🔴 用乾淨的短檔名（例如 {@code S1.pdf}）測這一格是零鑑別力的：消毒前後相同，
         * 兩個斷言就都變成「等於自己」。這裡刻意傳一個 sanitize 會改變的檔名。
         */
        @Test
        @DisplayName("#3149：original_filename 消毒、object_key 末段吃原值（兩者刻意分歧）")
        void persistsSanitizedFilenameButKeepsRawInObjectKey() {
            // 含引號＋控制字元：引號那一半是 #3149 的引號迴歸
            // （builder 的 charset 多載在 spring-web 6.1.2 上不逃逸引號，防線只在寫入側）
            // 控制字元一律用轉義序列寫，不在原碼裡嵌真的 0x01
            String raw = "標籤\"x\u0001\r\n.pdf";

            mediaAssetService.serverUploadAndFinalize(
                    new byte[]{1, 2, 3}, raw, "application/pdf", AssetReferenceType.SHIPPING.value());

            org.mockito.ArgumentCaptor<MediaAssetEntity> inserted =
                    org.mockito.ArgumentCaptor.forClass(MediaAssetEntity.class);
            verify(mediaAssetRepository).insert(inserted.capture());
            MediaAssetEntity asset = inserted.getValue();

            // A：顯示值消毒（控制字元與引號全清）
            assertEquals("標籤x.pdf", asset.getOriginalFilename());
            assertFalse(asset.getOriginalFilename().contains("\""),
                    "引號必須在寫入側就被清掉——讀取側的 ContentDisposition builder 不逃逸它");

            // B：🔴 object_key 末段是**未消毒**的原值。它是既有資料的定址鍵——
            // selectByObjectKey、GC 的補償刪除、uk_media_asset_object_key 全部以它為準，
            // 改變組法會讓既有物件與新 key 規則不一致。沒有這一格，「不小心把 objectKey
            // 一起改成消毒後的值」不會轉紅。
            assertTrue(asset.getObjectKey().endsWith("/" + raw),
                    "object_key 末段必須是未消毒的原值，實得 " + asset.getObjectKey());
            assertFalse(asset.getObjectKey().endsWith("/標籤x.pdf"),
                    "object_key 不得吃消毒後的值，實得 " + asset.getObjectKey());
            assertTrue(asset.getObjectKey().contains("\""),
                    "object_key 末段是原值，引號應原樣保留（§4.9.3 兩個值不可互相取代）");
        }
    }

    @Nested
    @DisplayName("ServerUploadSecureAndFinalize（機敏檔後端直傳，58 §8.1.5）")
    class ServerUploadSecureTests {

        private static final String INVOICE_REF_TYPE = "7120"; // AssetReferenceType.INVOICE_DOCUMENT
        private static final String DOC_ID = "7499573277841424384";

        @Test
        @DisplayName("objectKey 落 secure/ 前綴、is_secure=true、同一次呼叫建立 asset_reference")
        void uploadsToSecurePrefixAndLinksReference() {
            when(transactionManager.getTransaction(any())).thenReturn(
                    new org.springframework.transaction.support.SimpleTransactionStatus());

            MediaAssetEntity asset = mediaAssetService.serverUploadSecureAndFinalize(
                    "%PDF-1.4 test".getBytes(java.nio.charset.StandardCharsets.UTF_8),
                    "ZA10068421.pdf", "application/pdf", INVOICE_REF_TYPE, DOC_ID);

            // 🔴 secure/ 前綴：既有的 serverUploadAndFinalize 硬寫 public/，
            // 而 public/ 的授權述詞對 uploaderAccountId=NULL 的資產兩個分支都不通（§165）。
            assertTrue(asset.getObjectKey().startsWith("secure/"),
                    "機敏檔必須落 secure/ 前綴，實得 " + asset.getObjectKey());
            assertTrue(Boolean.TRUE.equals(asset.getIsSecure()), "is_secure 必須是 true");
            assertEquals(INVOICE_REF_TYPE, asset.getRefType());

            // 🔴 §8.6 T-03：建資產與建引用必須是同一個動作。分開做的話 media GC 第 2 階段
            // 會在第 8 天把 media_asset 連同 R2 物件刪除，而發票文件列還記著 pdf_object_key
            // ⇒ **下載時才 404**，且 AMEGO 的 invoice_file 只查得到 180 天內。
            verify(assetReferenceService).linkImages(
                    org.mockito.ArgumentMatchers.eq(INVOICE_REF_TYPE),
                    org.mockito.ArgumentMatchers.eq(DOC_ID),
                    org.mockito.ArgumentMatchers.eq(java.util.List.of(String.valueOf(asset.getId()))));
        }

        @Test
        @DisplayName("建列或建引用失敗時補償刪除剛上傳的物件")
        void compensatesWhenPersistFails() {
            when(transactionManager.getTransaction(any())).thenReturn(
                    new org.springframework.transaction.support.SimpleTransactionStatus());
            org.mockito.Mockito.doThrow(new RuntimeException("boom"))
                    .when(mediaAssetRepository).insert(any());

            assertThrows(RuntimeException.class, () -> mediaAssetService.serverUploadSecureAndFinalize(
                    "%PDF-1.4 test".getBytes(java.nio.charset.StandardCharsets.UTF_8),
                    "ZA10068421.pdf", "application/pdf", INVOICE_REF_TYPE, DOC_ID));

            // 少了補償，R2 會留下一個沒有任何一張表認得的物件。
            verify(storageService).deleteObject(any());
        }

        /** 同 {@code serverUploadAndFinalize}：兩個去處必須分歧（#3149 §4.9.3〈作用範圍〉）。 */
        @Test
        @DisplayName("#3149：original_filename 消毒、object_key 末段吃原值（兩者刻意分歧）")
        void persistsSanitizedFilenameButKeepsRawInObjectKey() {
            when(transactionManager.getTransaction(any())).thenReturn(
                    new org.springframework.transaction.support.SimpleTransactionStatus());
            String raw = "發票\"x\u0001\r\n.pdf";

            MediaAssetEntity asset = mediaAssetService.serverUploadSecureAndFinalize(
                    "%PDF-1.4 test".getBytes(java.nio.charset.StandardCharsets.UTF_8),
                    raw, "application/pdf", INVOICE_REF_TYPE, DOC_ID);

            // A：顯示值消毒（控制字元與引號全清）
            assertEquals("發票x.pdf", asset.getOriginalFilename());
            assertFalse(asset.getOriginalFilename().contains("\""),
                    "引號必須在寫入側就被清掉");

            // B：🔴 object_key 末段是未消毒的原值——這一格才是守住 §4.9.3 邊界的東西
            assertTrue(asset.getObjectKey().endsWith("/" + raw),
                    "object_key 末段必須是未消毒的原值，實得 " + asset.getObjectKey());
            assertFalse(asset.getObjectKey().endsWith("/發票x.pdf"),
                    "object_key 不得吃消毒後的值，實得 " + asset.getObjectKey());
        }
    }
}
