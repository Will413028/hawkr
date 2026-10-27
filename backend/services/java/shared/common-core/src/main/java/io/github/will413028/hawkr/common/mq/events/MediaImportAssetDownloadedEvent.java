package io.github.will413028.hawkr.common.mq.events;

/**
 * Media → Product Import Service 跨服務 callback 事件（08 號設計書 §7.2a / 36 號 §3.6b / 40 號 §6.5a）。
 *
 * <p>由 Media 內部下載 worker 完成（UPLOADED 或 FAILED）後 publish 至
 * {@code hawkr.media} exchange + {@code import.asset.downloaded} routing key，
 * 由 product-import-service 的 {@code MediaCallbackListener} 消費。</p>
 *
 * <p>本 record 為兩端共用 SSOT，集中於 common-core 以杜絕兩服務各自定義導致 schema 漂移
 * （docs/開發規範/Java 微服務開發規範.md §15 / §10、docs/開發規範/MQ 與事件管理規範.md）。</p>
 *
 * @param eventId                    UUID v4，Media 端產生（dedup 用）
 * @param importAssetId              Media 端 import_asset.id
 * @param importJobId                import_job.id（從 import_asset 反查回填，方便 callback 端路由）
 * @param importItemId               import_item.id
 * @param productImportItemAssetId   Import 端 product_import_item_asset.id（即 RPC 階段傳入的 callback_correlation_id，原樣 echo）
 * @param resultStatus               對齊 {@code ImportAssetMediaStatus.value()}：{@code 9402}=UPLOADED / {@code 9404}=FAILED
 * @param objectKey                  UPLOADED 時填（OSS tmp 路徑）
 * @param previewUrl                 UPLOADED 時填（OSS 簽名預覽 URL，可選）
 * @param contentType                UPLOADED 時填
 * @param fileSize                   UPLOADED 時填（位元組）
 * @param failureReason              FAILED 時填，值對齊 {@code ImportAssetFailureReason}
 * @param failureDetail              FAILED 時填，自由格式 error message
 * @param completedAt                ISO 8601 UTC 完成時間
 */
public record MediaImportAssetDownloadedEvent(
    String eventId,
    String importAssetId,
    Long importJobId,
    Long importItemId,
    Long productImportItemAssetId,
    String resultStatus,
    String objectKey,
    String previewUrl,
    String contentType,
    Long fileSize,
    String failureReason,
    String failureDetail,
    String completedAt
) {
}
