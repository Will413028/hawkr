package io.github.will413028.hawkr.media.service;

import io.github.will413028.hawkr.common.obs.log.ObsLogger;
import io.github.will413028.hawkr.media.crypto.AesStreamCipher;
import io.github.will413028.hawkr.media.exception.MediaServiceException;
import io.github.will413028.hawkr.media.storage.StorageObjectStat;
import org.springframework.beans.factory.annotation.Value;
import org.springframework.stereotype.Component;

import java.util.Set;

/**
 * 上傳驗證（49 號設計書 §8.1，堵盤點缺口）。
 *
 * <p>兩道關卡：
 * <ol>
 *   <li>宣告驗證（CreateUploadSession）：MIME 白名單 + fileSize 上限宣告。</li>
 *   <li>強制點——finalize 時 HEAD 回查 R2 實際 size/Content-Type：超限或與宣告不符即 reject
 *       （呼叫端負責刪除 tmp 物件）。SigV4 presigned PUT 無法表達 size 範圍條件，故一律由此兜底。</li>
 * </ol>
 *
 * <p>公開圖片限 image/*（jpeg/png/webp/gif）；PDF 僅限機敏/標籤場景（isSecure）。
 */
@Component
public class MediaUploadValidator {

    private static final ObsLogger log = ObsLogger.forClass(MediaUploadValidator.class);

    /** 公開圖片允許的 MIME。 */
    private static final Set<String> IMAGE_MIME_TYPES = Set.of(
        "image/jpeg", "image/png", "image/webp", "image/gif");

    /** 機敏/標籤場景額外允許的 MIME。 */
    private static final String PDF_MIME_TYPE = "application/pdf";

    /** 一般圖片大小上限（bytes），與前端一致。 */
    @Value("${hawkr.media.upload.max-image-size-bytes:10485760}")
    private long maxImageSizeBytes;

    /** 機敏檔案大小上限（bytes）。 */
    @Value("${hawkr.media.upload.max-secure-size-bytes:104857600}")
    private long maxSecureSizeBytes;

    /**
     * CreateUploadSession 宣告驗證。
     *
     * @param contentType 前端宣告的 MIME
     * @param declaredSize 前端宣告的檔案大小
     * @param isSecure 是否機敏檔案（PDF 僅此場景允許）
     */
    public void validateDeclared(String contentType, Long declaredSize, boolean isSecure) {
        if (contentType == null || contentType.isBlank()) {
            throw MediaServiceException.invalidArgument("內容類型不可為空");
        }
        String normalized = contentType.trim().toLowerCase();
        if (!isAllowedMime(normalized, isSecure)) {
            throw MediaServiceException.invalidArgument("不支援的檔案類型: " + contentType);
        }
        long limit = sizeLimit(isSecure);
        if (declaredSize == null || declaredSize <= 0) {
            throw MediaServiceException.invalidArgument("檔案大小必須為正數");
        }
        if (declaredSize > limit) {
            throw MediaServiceException.invalidArgument(
                "檔案大小超過上限 " + limit + " bytes（宣告 " + declaredSize + "）");
        }
    }

    /**
     * finalize HEAD 回查驗證（強制點）。stat 為 null（物件不存在）或超限 / 與宣告不符即拋出。
     *
     * @param stat HEAD 回查結果（null 表示物件不存在）
     * @param declaredContentType upload_session 宣告的 MIME
     * @param declaredSize upload_session 宣告的檔案大小
     * @param isSecure 是否機敏檔案
     */
    public void validateFinalizedObject(StorageObjectStat stat,
                                        String declaredContentType,
                                        Long declaredSize,
                                        boolean isSecure) {
        if (stat == null) {
            throw MediaServiceException.invalidArgument("上傳物件不存在，無法轉正");
        }
        long limit = sizeLimit(isSecure);
        if (stat.contentLength() > limit) {
            log.warn("upload.head.oversize", "實際檔案超過上限",
                "actual_limit", stat.contentLength() + "/" + limit);
            throw MediaServiceException.invalidArgument(
                "實際檔案大小超過上限 " + limit + " bytes（實際 " + stat.contentLength() + "）");
        }
        // 與宣告不符：大小。isSecure 物件在 R2 上存的是 AES-GCM 密文，
        // 密文大小 = 明文大小（宣告值）+ GCM_TAG_BYTES，比對時需加回這個 offset。
        if (declaredSize != null && declaredSize > 0) {
            long expectedSize = isSecure ? declaredSize + AesStreamCipher.GCM_TAG_BYTES : declaredSize;
            if (stat.contentLength() != expectedSize) {
                log.warn("upload.head.size_mismatch", "實際大小與宣告不符",
                    "actual_expected", stat.contentLength() + "/" + expectedSize);
                throw MediaServiceException.invalidArgument(
                    "實際檔案大小與宣告不符（宣告 " + declaredSize + "，實際 " + stat.contentLength() + "）");
            }
        }
        // isSecure 物件的實際 Content-Type 永遠是 application/octet-stream
        // （SecureMediaController.secureUpload 寫入 R2 時對密文一律硬編碼此值，
        // 見該類別 storageService.uploadStream 呼叫）——跟宣告的原始檔案 MIME
        // 無關，比對只對公開（非加密）上傳有意義。原始 MIME 已在 validateDeclared
        // （CreateUploadSession 時）驗過白名單，這裡對 isSecure 物件不重複比對。
        if (!isSecure) {
            // 實際 Content-Type 必須在白名單
            String actualType = stat.contentType() == null ? "" : stat.contentType().trim().toLowerCase();
            if (!isAllowedMime(actualType, isSecure)) {
                log.warn("upload.head.mime_reject", "實際 Content-Type 不在白名單",
                    "actual", actualType);
                throw MediaServiceException.invalidArgument("實際檔案類型不支援: " + stat.contentType());
            }
            // 與宣告不符：MIME 家族
            if (declaredContentType != null && !declaredContentType.isBlank()) {
                String declaredType = declaredContentType.trim().toLowerCase();
                if (!declaredType.equals(actualType)) {
                    log.warn("upload.head.mime_mismatch", "實際 Content-Type 與宣告不符",
                        "actual_declared", actualType + "/" + declaredType);
                    throw MediaServiceException.invalidArgument(
                        "實際檔案類型與宣告不符（宣告 " + declaredContentType + "，實際 " + stat.contentType() + "）");
                }
            }
        }
    }

    private boolean isAllowedMime(String normalizedMime, boolean isSecure) {
        if (IMAGE_MIME_TYPES.contains(normalizedMime)) {
            return true;
        }
        return isSecure && PDF_MIME_TYPE.equals(normalizedMime);
    }

    private long sizeLimit(boolean isSecure) {
        return isSecure ? maxSecureSizeBytes : maxImageSizeBytes;
    }
}
