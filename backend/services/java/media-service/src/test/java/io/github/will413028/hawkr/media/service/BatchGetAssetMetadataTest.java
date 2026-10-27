package io.github.will413028.hawkr.media.service;

import io.github.will413028.hawkr.common.utils.code.AssetReferenceType;
import io.github.will413028.hawkr.media.converter.MediaAssetConverter;
import io.github.will413028.hawkr.media.crypto.AesStreamCipher;
import io.github.will413028.hawkr.media.dto.AssetMetadataDTO;
import io.github.will413028.hawkr.media.entity.MediaAssetEntity;
import io.github.will413028.hawkr.media.repository.MediaAssetRepository;
import io.github.will413028.hawkr.media.repository.UploadSessionRepository;
import io.github.will413028.hawkr.media.storage.StorageService;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.DisplayName;
import org.junit.jupiter.api.Test;
import org.springframework.transaction.PlatformTransactionManager;

import java.time.LocalDateTime;
import java.util.List;

import static org.assertj.core.api.Assertions.assertThat;
import static org.mockito.Mockito.mock;
import static org.mockito.Mockito.when;

/**
 * {@code batchGetAssetMetadata}（#3031）。
 *
 * <p>🔴 <b>查無與越權必須無法分辨</b>：兩者都是「不出現在結果裡」。回一個
 * 「這個 key 存在但你沒權限」的訊號，等於把這支 RPC 變成可枚舉的 oracle——
 * 攻擊者拿一批猜的 object_key 掃，靠回應差異就能問出哪些存在。
 */
class BatchGetAssetMetadataTest {

    private static final String REF_TYPE = AssetReferenceType.CERTIFICATION.value();
    private static final String REF_ID = "7001";
    private static final String MINE = "secure/tax/mine.pdf";
    private static final String OTHERS = "secure/tax/others.pdf";
    private static final String GONE = "secure/tax/gone.pdf";

    private MediaAssetRepository repo;
    private AssetReferenceService references;
    private MediaAssetService service;

    @BeforeEach
    void setUp() {
        repo = mock(MediaAssetRepository.class);
        references = mock(AssetReferenceService.class);
        service = new MediaAssetService(
                repo,
                mock(UploadSessionRepository.class),
                mock(StagedTokenService.class),
                mock(StorageService.class),
                mock(MediaAssetConverter.class),
                mock(MediaUploadValidator.class),
                new AesStreamCipher(""),
                references,
                mock(PlatformTransactionManager.class));
    }

    private static MediaAssetEntity asset(long id, String key) {
        MediaAssetEntity a = new MediaAssetEntity();
        a.setId(id);
        a.setObjectKey(key);
        a.setMimeType("application/pdf");
        a.setSizeBytes(10240L);
        // 帶奈秒的輸入：不截斷的話會印出 "…T07:28:15.123456789Z"，違反秒精度契約。
        a.setCreatedAt(LocalDateTime.of(2026, 9, 1, 7, 28, 15, 123456789));
        return a;
    }

    @Test
    @DisplayName("通過授權的回完整中介資料；越權與查無一律不出現（兩者逐字相同）")
    void returnsOnlyAuthorizedAssetsAndCannotDistinguishDenialFromMissing() {
        when(repo.selectByObjectKey(MINE)).thenReturn(asset(881L, MINE));
        when(repo.selectByObjectKey(OTHERS)).thenReturn(asset(882L, OTHERS));
        // GONE：repo 回 null（Mockito 預設）＝ 查無
        // 唯一正解才 true：填錯任何一欄，Mockito 預設回 false。
        when(references.hasReference(881L, REF_TYPE, REF_ID)).thenReturn(true);

        List<AssetMetadataDTO> got = service.batchGetAssetMetadata(
                List.of(MINE, OTHERS, GONE), REF_TYPE, REF_ID);

        assertThat(got).hasSize(1);
        AssetMetadataDTO m = got.get(0);
        assertThat(m.objectKey()).isEqualTo(MINE);
        // 🔴 asset_id 是 Gateway 組下載路徑的唯一輸入。
        assertThat(m.assetId()).isEqualTo("881");
        assertThat(m.fileName()).isEqualTo("mine.pdf");
        assertThat(m.sizeBytes()).isEqualTo("10240");
        // 🔴 秒精度（58 §6.2.0 硬慣例 3）：次秒必須被截掉。
        assertThat(m.uploadedAt()).isEqualTo("2026-09-01T07:28:15Z");

        // 越權（OTHERS）與查無（GONE）在結果裡**同樣是缺席**，無從分辨。
        assertThat(got).extracting(AssetMetadataDTO::objectKey)
                .doesNotContain(OTHERS, GONE);
    }

    @Test
    @DisplayName("#3149：original_filename 存在時優先於 objectKey 尾段（兩值刻意不同）")
    void originalFilenameTakesPrecedenceOverObjectKeyTail() {
        MediaAssetEntity a = asset(881L, MINE);
        // 🔴 兩個值必須不同：objectKey 尾段是 "mine.pdf"，原始檔名是中文。
        // 相同的話「原始優先」壞掉（永遠回尾段）也會全綠。
        a.setOriginalFilename("稅籍證明_2026.pdf");
        when(repo.selectByObjectKey(MINE)).thenReturn(a);
        when(references.hasReference(881L, REF_TYPE, REF_ID)).thenReturn(true);

        List<AssetMetadataDTO> got = service.batchGetAssetMetadata(List.of(MINE), REF_TYPE, REF_ID);

        assertThat(got).hasSize(1);
        assertThat(got.get(0).fileName()).isEqualTo("稅籍證明_2026.pdf");
        // 這一行才是本格的本體：賣家上傳 3 份稅籍證明時看到三串一模一樣的 uuid，
        // 正是 #3149 的原始症狀。
        assertThat(got.get(0).fileName()).isNotEqualTo("mine.pdf");
    }

    @Test
    @DisplayName("#3149：original_filename 為 NULL（V9 之前的列）→ 回退到 objectKey 尾段")
    void fallsBackToObjectKeyTailForLegacyRows() {
        // V9 之前建立的列真的沒有原始檔名（R2 的 object metadata 也沒存，無從回填）
        MediaAssetEntity a = asset(881L, MINE);
        a.setOriginalFilename(null);
        when(repo.selectByObjectKey(MINE)).thenReturn(a);
        when(references.hasReference(881L, REF_TYPE, REF_ID)).thenReturn(true);

        List<AssetMetadataDTO> got = service.batchGetAssetMetadata(List.of(MINE), REF_TYPE, REF_ID);

        assertThat(got.get(0).fileName()).isEqualTo("mine.pdf");
    }

    @Test
    @DisplayName("空輸入回空清單，不查 repository")
    void emptyInputIsANoop() {
        assertThat(service.batchGetAssetMetadata(List.of(), REF_TYPE, REF_ID)).isEmpty();
        assertThat(service.batchGetAssetMetadata(null, REF_TYPE, REF_ID)).isEmpty();
    }
}
