package io.github.will413028.hawkr.media.service;

import io.github.will413028.hawkr.media.exception.MediaServiceException;
import io.github.will413028.hawkr.media.storage.StorageObjectStat;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.DisplayName;
import org.junit.jupiter.api.Test;
import org.springframework.test.util.ReflectionTestUtils;

import static org.junit.jupiter.api.Assertions.assertDoesNotThrow;
import static org.junit.jupiter.api.Assertions.assertThrows;

/**
 * {@link MediaUploadValidator} 單元測試（49 號 §8.1）。
 * 覆蓋宣告驗證（MIME 白名單 / size 上限）與 finalize HEAD 回查（超限 / 與宣告不符 / 物件不存在）。
 */
class MediaUploadValidatorTest {

    private MediaUploadValidator validator;

    @BeforeEach
    void setUp() {
        validator = new MediaUploadValidator();
        ReflectionTestUtils.setField(validator, "maxImageSizeBytes", 10_485_760L);
        ReflectionTestUtils.setField(validator, "maxSecureSizeBytes", 104_857_600L);
    }

    @Test
    @DisplayName("宣告：合法圖片 MIME + size 通過")
    void declaredValidImage() {
        assertDoesNotThrow(() -> validator.validateDeclared("image/jpeg", 1024L, false));
        assertDoesNotThrow(() -> validator.validateDeclared("image/webp", 1024L, false));
    }

    @Test
    @DisplayName("宣告：非白名單 MIME → 拒絕")
    void declaredRejectsUnknownMime() {
        assertThrows(MediaServiceException.class,
            () -> validator.validateDeclared("application/zip", 1024L, false));
    }

    @Test
    @DisplayName("宣告：PDF 僅機敏場景允許（公開場景拒絕）")
    void declaredPdfOnlyForSecure() {
        assertThrows(MediaServiceException.class,
            () -> validator.validateDeclared("application/pdf", 1024L, false));
        assertDoesNotThrow(() -> validator.validateDeclared("application/pdf", 1024L, true));
    }

    @Test
    @DisplayName("宣告：超過圖片上限 → 拒絕")
    void declaredRejectsOversize() {
        assertThrows(MediaServiceException.class,
            () -> validator.validateDeclared("image/png", 10_485_761L, false));
    }

    @Test
    @DisplayName("宣告：size 非正數 → 拒絕")
    void declaredRejectsNonPositiveSize() {
        assertThrows(MediaServiceException.class,
            () -> validator.validateDeclared("image/png", 0L, false));
        assertThrows(MediaServiceException.class,
            () -> validator.validateDeclared("image/png", null, false));
    }

    @Test
    @DisplayName("HEAD 回查：物件不存在（stat=null）→ 拒絕")
    void finalizedRejectsMissingObject() {
        assertThrows(MediaServiceException.class,
            () -> validator.validateFinalizedObject(null, "image/jpeg", 1024L, false));
    }

    @Test
    @DisplayName("HEAD 回查：實際超限 → 拒絕")
    void finalizedRejectsActualOversize() {
        StorageObjectStat stat = new StorageObjectStat(10_485_761L, "image/jpeg");
        assertThrows(MediaServiceException.class,
            () -> validator.validateFinalizedObject(stat, "image/jpeg", null, false));
    }

    @Test
    @DisplayName("HEAD 回查：實際大小與宣告不符 → 拒絕")
    void finalizedRejectsSizeMismatch() {
        StorageObjectStat stat = new StorageObjectStat(2048L, "image/jpeg");
        assertThrows(MediaServiceException.class,
            () -> validator.validateFinalizedObject(stat, "image/jpeg", 1024L, false));
    }

    @Test
    @DisplayName("HEAD 回查：實際 Content-Type 竄改為非白名單 → 拒絕")
    void finalizedRejectsActualMimeNotWhitelisted() {
        StorageObjectStat stat = new StorageObjectStat(1024L, "application/x-msdownload");
        assertThrows(MediaServiceException.class,
            () -> validator.validateFinalizedObject(stat, "image/jpeg", 1024L, false));
    }

    @Test
    @DisplayName("HEAD 回查：實際 Content-Type 與宣告不符 → 拒絕")
    void finalizedRejectsMimeMismatch() {
        StorageObjectStat stat = new StorageObjectStat(1024L, "image/png");
        assertThrows(MediaServiceException.class,
            () -> validator.validateFinalizedObject(stat, "image/jpeg", 1024L, false));
    }

    @Test
    @DisplayName("HEAD 回查：實際與宣告一致且未超限 → 通過")
    void finalizedPasses() {
        StorageObjectStat stat = new StorageObjectStat(1024L, "image/jpeg");
        assertDoesNotThrow(
            () -> validator.validateFinalizedObject(stat, "image/jpeg", 1024L, false));
    }

    @Test
    @DisplayName("HEAD 回查：isSecure 物件實際大小 = 宣告 + GCM_TAG_BYTES，實際 Content-Type 為密文固定值 → 通過")
    void finalizedSecurePassesWithGcmTagOverhead() {
        // SecureMediaController.secureUpload 寫入 R2 時對密文一律硬編碼
        // "application/octet-stream"（與宣告的原始檔案 MIME 無關）——isSecure
        // 物件不比對 Content-Type，只比對加回 GCM tag 後的大小。
        StorageObjectStat stat = new StorageObjectStat(
            1024L + io.github.will413028.hawkr.media.crypto.AesStreamCipher.GCM_TAG_BYTES, "application/octet-stream");
        assertDoesNotThrow(
            () -> validator.validateFinalizedObject(stat, "image/jpeg", 1024L, true));
    }

    @Test
    @DisplayName("HEAD 回查：isSecure 物件實際大小 = 宣告值本身（未加密）→ 拒絕")
    void finalizedSecureRejectsWhenMissingGcmTag() {
        StorageObjectStat stat = new StorageObjectStat(1024L, "application/octet-stream");
        assertThrows(MediaServiceException.class,
            () -> validator.validateFinalizedObject(stat, "image/jpeg", 1024L, true));
    }

    @Test
    @DisplayName("HEAD 回查：isSecure 物件不比對 Content-Type——即使宣告 MIME 與密文固定值不符也通過")
    void finalizedSecureSkipsContentTypeCheck() {
        StorageObjectStat stat = new StorageObjectStat(
            1024L + io.github.will413028.hawkr.media.crypto.AesStreamCipher.GCM_TAG_BYTES, "application/octet-stream");
        assertDoesNotThrow(
            () -> validator.validateFinalizedObject(stat, "application/pdf", 1024L, true));
    }
}
