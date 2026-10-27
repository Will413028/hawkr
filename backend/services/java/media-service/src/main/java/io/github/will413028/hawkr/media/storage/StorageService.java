package io.github.will413028.hawkr.media.storage;

import java.io.InputStream;

/**
 * 物件儲存抽象（49 號設計書 §4.1/§4.2）。
 *
 * <p>唯一實作為 {@link R2StorageService}（Cloudflare R2，S3 相容 API）。objectKey 結構、
 * 上傳三段式流程、staged token、finalize、GC 邏輯全部與 backend 無關，故收斂於此介面。
 *
 * <p>方法面涵蓋 presign / put / get / copy / delete，另有 {@link #statObject(String)}
 * （finalize HEAD 回查）與 {@link #isEnabled()}（dev 未配置時走 mock/skip fallback，
 * 回 false 以跳過需真實物件的檢查）。
 */
public interface StorageService {

    /** 生成暫存 object key（{visibility}/{yyyyMMdd}/{uuid}{ext}）。 */
    String generateObjectKey(String filename, boolean isSecure);

    /**
     * 生成轉正後的最終 object key（{visibility}/{category}/{ownerId}/{date}/{uuid}{ext}）。
     *
     * <p>{@code ownerId} 刻意是**軸中立**的：一般上傳路徑
     * （{@code MediaAssetService.finalizeSingleImage}）傳的是真登入 account_id，
     * CSV/URL 匯入轉正路徑（{@code ImportAssetService.prepareFinalize}）傳的是賣家
     * store_id。此值只作路徑分桶字串，不參與任何身份判斷，故不綁定單一 id 軸；
     * 但也因此**不可以**叫 accountId——那會讓一半的呼叫端與名字不符。</p>
     */
    String generateFinalObjectKey(String originalKey, boolean isSecure, String refType, Long ownerId);

    /** 生成預簽名上傳 URL（PUT）。 */
    String generatePresignedUploadUrl(String objectKey, String contentType, int expiresSec);

    /** 生成預簽名下載 URL（GET）。 */
    String generatePresignedDownloadUrl(String objectKey, int expiresSec);

    /** 串流上傳（機敏檔案 / 後端直傳）。 */
    void uploadStream(String objectKey, InputStream inputStream, String contentType, long size);

    /** 取得物件串流（機敏檔案下載）。 */
    InputStream getObjectStream(String objectKey);

    /** 刪除物件。 */
    void deleteObject(String objectKey);

    /** 複製物件（finalize 暫存 → 正式路徑）。 */
    void copyObject(String sourceKey, String destinationKey);

    /**
     * HEAD 回查物件實際大小 / Content-Type。物件不存在回 {@code null}。
     * 僅在 {@link #isEnabled()} 為 true 時有意義。
     */
    StorageObjectStat statObject(String objectKey);

    /**
     * 儲存後端是否已配置就緒。dev 未配置時回 false（走 mock/skip fallback），
     * 呼叫端據此跳過需要真實物件存在的檢查（如 finalize HEAD 回查）。
     */
    boolean isEnabled();
}
