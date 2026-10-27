package io.github.will413028.hawkr.media.service;

import io.github.will413028.hawkr.common.utils.code.AssetReferenceType;
import io.github.will413028.hawkr.media.entity.MediaAssetEntity;
import io.github.will413028.hawkr.media.exception.MediaServiceException;
import io.github.will413028.hawkr.media.repository.AssetReferenceRepository;
import io.github.will413028.hawkr.media.repository.MediaAssetRepository;
import org.junit.jupiter.api.DisplayName;
import org.junit.jupiter.api.Nested;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.api.extension.ExtendWith;
import org.mockito.InjectMocks;
import org.mockito.Mock;
import org.mockito.junit.jupiter.MockitoExtension;

import java.util.Collections;
import java.util.List;

import static org.junit.jupiter.api.Assertions.*;
import static org.mockito.ArgumentMatchers.*;
import static org.mockito.Mockito.*;

/**
 * AssetReferenceService 單元測試
 * <p>
 * 測試重點：
 * 1. LinkImages 的冪等行為
 * 2. UnlinkImages 的冪等行為
 * 3. 參數驗證
 */
@ExtendWith(MockitoExtension.class)
class AssetReferenceServiceTest {

    @Mock
    private AssetReferenceRepository assetReferenceRepository;

    @Mock
    private MediaAssetRepository mediaAssetRepository;

    @InjectMocks
    private AssetReferenceService assetReferenceService;

    private static final String REF_TYPE = AssetReferenceType.PRODUCT.value();
    private static final String REF_TYPE_CHAT = AssetReferenceType.CHAT_MESSAGE.value();
    private static final String REF_ID = "product-123";
    private static final Long ASSET_ID_1 = 1L;
    private static final Long ASSET_ID_2 = 2L;

    @Nested
    @DisplayName("LinkImages 測試")
    class LinkImagesTests {

        @Test
        @DisplayName("正常建立引用")
        void shouldLinkSuccessfully() {
            // Given
            List<String> assetIds = List.of(String.valueOf(ASSET_ID_1), String.valueOf(ASSET_ID_2));

            MediaAssetEntity asset1 = new MediaAssetEntity();
            asset1.setId(ASSET_ID_1);
            MediaAssetEntity asset2 = new MediaAssetEntity();
            asset2.setId(ASSET_ID_2);

            when(mediaAssetRepository.selectByIds(anyList())).thenReturn(List.of(asset1, asset2));
            when(assetReferenceRepository.insertIgnoreConflict(any(), anyString(), anyString())).thenReturn(1);

            // When
            int linkedCount = assetReferenceService.linkImages(REF_TYPE, REF_ID, assetIds);

            // Then
            assertEquals(2, linkedCount);
            verify(assetReferenceRepository, times(2)).insertIgnoreConflict(any(), eq(REF_TYPE), eq(REF_ID));
        }

        @Test
        @DisplayName("冪等：重複呼叫不報錯")
        void shouldBeIdempotent() {
            // Given
            List<String> assetIds = List.of(String.valueOf(ASSET_ID_1));

            MediaAssetEntity asset = new MediaAssetEntity();
            asset.setId(ASSET_ID_1);

            when(mediaAssetRepository.selectByIds(anyList())).thenReturn(List.of(asset));
            when(assetReferenceRepository.insertIgnoreConflict(any(), anyString(), anyString())).thenReturn(1);

            // When - 呼叫兩次
            int count1 = assetReferenceService.linkImages(REF_TYPE, REF_ID, assetIds);
            int count2 = assetReferenceService.linkImages(REF_TYPE, REF_ID, assetIds);

            // Then - 兩次都應成功
            assertEquals(1, count1);
            assertEquals(1, count2);
        }

        @Test
        @DisplayName("同一個 asset_id 出現兩次不得誤判為「部分資產不存在」")
        void shouldTolerateDuplicateAssetIds() {
            // Given - 呼叫端傳了同一個 assetId 兩次（同一個 staged_token 在一次 FinalizeImages
            // 裡出現兩次時，重入語意會讓兩個位置都回同一個 assetId）
            List<String> assetIds = List.of(String.valueOf(ASSET_ID_1), String.valueOf(ASSET_ID_1));

            MediaAssetEntity asset = new MediaAssetEntity();
            asset.setId(ASSET_ID_1);
            // selectByIds 走 SQL IN，天然去重 → 只會回 1 筆
            when(mediaAssetRepository.selectByIds(anyList())).thenReturn(List.of(asset));
            when(assetReferenceRepository.insertIgnoreConflict(any(), anyString(), anyString())).thenReturn(1);

            // When
            int linkedCount = assetReferenceService.linkImages(REF_TYPE, REF_ID, assetIds);

            // Then - 去重後為 1 筆，不得拋 notFound
            assertEquals(1, linkedCount);
            verify(assetReferenceRepository, times(1))
                .insertIgnoreConflict(eq(ASSET_ID_1), eq(REF_TYPE), eq(REF_ID));
        }

        @Test
        @DisplayName("ref_type 為空應拋出 INVALID_ARGUMENT")
        void shouldThrowWhenRefTypeEmpty() {
            // When & Then
            assertThrows(MediaServiceException.class, () -> {
                assetReferenceService.linkImages("", REF_ID, List.of(String.valueOf(ASSET_ID_1)));
            });
        }

        @Test
        @DisplayName("ref_id 為空應拋出 INVALID_ARGUMENT")
        void shouldThrowWhenRefIdEmpty() {
            // When & Then
            assertThrows(MediaServiceException.class, () -> {
                assetReferenceService.linkImages(REF_TYPE, "", List.of(String.valueOf(ASSET_ID_1)));
            });
        }

        @Test
        @DisplayName("asset_ids 為空應拋出 INVALID_ARGUMENT")
        void shouldThrowWhenAssetIdsEmpty() {
            // When & Then
            assertThrows(MediaServiceException.class, () -> {
                assetReferenceService.linkImages(REF_TYPE, REF_ID, Collections.emptyList());
            });
        }

        @Test
        @DisplayName("資產不存在應拋出 NOT_FOUND")
        void shouldThrowWhenAssetNotFound() {
            // Given
            List<String> assetIds = List.of(String.valueOf(ASSET_ID_1));
            when(mediaAssetRepository.selectByIds(anyList())).thenReturn(Collections.emptyList());

            // When & Then
            assertThrows(MediaServiceException.class, () -> {
                assetReferenceService.linkImages(REF_TYPE, REF_ID, assetIds);
            });
        }
    }

    @Nested
    @DisplayName("UnlinkImages 測試")
    class UnlinkImagesTests {

        @Test
        @DisplayName("正常解除引用")
        void shouldUnlinkSuccessfully() {
            // Given
            List<String> assetIds = List.of(String.valueOf(ASSET_ID_1), String.valueOf(ASSET_ID_2));
            when(assetReferenceRepository.batchDeleteReferences(eq(REF_TYPE), eq(REF_ID), anyList()))
                .thenReturn(2);

            // When
            int unlinkedCount = assetReferenceService.unlinkImages(REF_TYPE, REF_ID, assetIds);

            // Then
            assertEquals(2, unlinkedCount);
        }

        @Test
        @DisplayName("冪等：解除不存在的引用不報錯")
        void shouldBeIdempotentOnNonExistent() {
            // Given
            List<String> assetIds = List.of(String.valueOf(ASSET_ID_1));
            when(assetReferenceRepository.batchDeleteReferences(eq(REF_TYPE), eq(REF_ID), anyList()))
                .thenReturn(0); // 引用不存在，回傳 0

            // When
            int unlinkedCount = assetReferenceService.unlinkImages(REF_TYPE, REF_ID, assetIds);

            // Then - 不應拋出例外
            assertEquals(0, unlinkedCount);
        }

        @Test
        @DisplayName("ref_type 為空應拋出 INVALID_ARGUMENT")
        void shouldThrowWhenRefTypeEmpty() {
            // When & Then
            assertThrows(MediaServiceException.class, () -> {
                assetReferenceService.unlinkImages("", REF_ID, List.of(String.valueOf(ASSET_ID_1)));
            });
        }
    }

    @Nested
    @DisplayName("ReleaseReferences 測試")
    class ReleaseReferencesTests {

        @Test
        @DisplayName("正常釋放引用")
        void shouldReleaseSuccessfully() {
            // Given
            List<String> refIds = List.of("101", "102", "103");
            when(assetReferenceRepository.deleteByRefTypeAndRefIds(eq(REF_TYPE_CHAT), anyList(), anyList()))
                .thenReturn(3);

            // When
            int released = assetReferenceService.releaseReferences(REF_TYPE_CHAT, refIds, List.of());

            // Then
            assertEquals(3, released);
            verify(assetReferenceRepository)
                .deleteByRefTypeAndRefIds(eq(REF_TYPE_CHAT), eq(refIds), eq(List.of()));
        }

        @Test
        @DisplayName("keep_asset_ids 非空時轉成 Long 並下推給 SQL（換圖：刪舊留新同一交易）")
        void shouldPassKeepAssetIdsDownToRepository() {
            // Given
            List<String> refIds = List.of("2685914682");
            when(assetReferenceRepository.deleteByRefTypeAndRefIds(eq(REF_TYPE_CHAT), anyList(), anyList()))
                .thenReturn(1);

            // When
            int released = assetReferenceService.releaseReferences(
                REF_TYPE_CHAT, refIds, List.of(String.valueOf(ASSET_ID_1)));

            // Then
            assertEquals(1, released);
            verify(assetReferenceRepository).deleteByRefTypeAndRefIds(
                eq(REF_TYPE_CHAT), eq(refIds), eq(List.of(ASSET_ID_1)));
        }

        @Test
        @DisplayName("keep_asset_ids 為 null 視為不保留任何資產")
        void shouldTreatNullKeepListAsReleaseAll() {
            when(assetReferenceRepository.deleteByRefTypeAndRefIds(eq(REF_TYPE_CHAT), anyList(), anyList()))
                .thenReturn(2);

            assetReferenceService.releaseReferences(REF_TYPE_CHAT, List.of("1"), null);

            verify(assetReferenceRepository)
                .deleteByRefTypeAndRefIds(eq(REF_TYPE_CHAT), eq(List.of("1")), eq(List.of()));
        }

        @Test
        @DisplayName("keep_asset_ids 格式錯誤時拋出例外，不下推 SQL")
        void shouldThrowWhenKeepAssetIdMalformed() {
            assertThrows(MediaServiceException.class,
                () -> assetReferenceService.releaseReferences(REF_TYPE_CHAT, List.of("1"), List.of("abc")));
            verifyNoInteractions(assetReferenceRepository);
        }

        @Test
        @DisplayName("ref_ids 為空時拋出例外")
        void shouldThrowWhenRefIdsEmpty() {
            assertThrows(MediaServiceException.class,
                () -> assetReferenceService.releaseReferences(REF_TYPE_CHAT, Collections.emptyList(), List.of()));
            verifyNoInteractions(assetReferenceRepository);
        }

        @Test
        @DisplayName("無效 ref_type 時拋出例外")
        void shouldThrowWhenRefTypeInvalid() {
            assertThrows(MediaServiceException.class,
                () -> assetReferenceService.releaseReferences("not-a-valid-type", List.of("1"), List.of()));
            verifyNoInteractions(assetReferenceRepository);
        }
    }
}
