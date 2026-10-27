package io.github.will413028.hawkr.productimport.storage;

/**
 * Product Import CSV 儲存抽象（49 號設計書：第二個 storage SDK 接入點）。
 *
 * <p>唯一實作為 {@link ProductImportR2Storage}（Cloudflare R2，S3 相容）。與 media-service
 * 共用同一 bucket，走獨立的 {@code tmp/product-import/} prefix（40 號設計書 §5.1 / §5.9）。
 */
public interface ProductImportStorage {

    /** 上傳 CSV bytes 到 {@code tmp/product-import/{storeId}/{jobId}.csv}，回傳 object key。 */
    String uploadCsv(Long storeId, Long jobId, byte[] content);

    /** 下載 CSV bytes；null 表示未啟用 / 物件不存在。 */
    byte[] downloadCsv(String objectKey);

    /** GC 用：刪除過期 CSV 物件。 */
    void deleteObject(String objectKey);
}
