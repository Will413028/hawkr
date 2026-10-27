package io.github.will413028.hawkr.productimport.publisher;

/**
 * Product Import 領域所有 MQ event payload records（40 號設計書 §6.5a）。
 *
 * <p>集中於此檔以利對齊 Java / Go 兩端 schema。對應 36 號設計書 §3.6b。
 */
public final class ImportEvents {

    private ImportEvents() {}

    // ============================================================================
    // 內部解耦 events
    // ============================================================================

    public record CsvParseEvent(Long importJobId, Long storeId) {}

    public record HtmlFetchEvent(Long importItemId,
                                 Long importJobId,
                                 Long storeId,
                                 String sourceUrl,
                                 Integer commitRevision) {}

    public record AssetDownloadEvent(Long importItemId,
                                     Long importJobId,
                                     Long storeId,
                                     Long assetId,
                                     String sourceUrl,
                                     /** GALLERY | DESCRIPTION */
                                     String usage,
                                     Integer commitRevision) {}

    public record CommitItemEvent(Long importItemId,
                                  Long importJobId,
                                  Long storeId,
                                  Integer commitRevision,
                                  String targetStatus,
                                  String idempotencyKey) {}

    // ============================================================================
    // 跨服務 / 跨團隊 events
    // ============================================================================

    /** Product Import → Media（GC 通知）。 */
    public record ExpireJobEvent(Long importJobId) {}

    /** Product Import → notification-service（解析+下載完成推播，一次性）。 */
    public record ImportJobParsedEvent(String eventId,
                                       Long importJobId,
                                       Long storeId,
                                       Integer totalItems,
                                       Integer readyItems,
                                       Integer parseFailedItems,
                                       Integer warningItems,
                                       String parsedAt) {}

    /** Product Import → notification-service（commit 完成推播，每輪 commit 都發）。 */
    public record ImportJobCommitFinishedEvent(String eventId,
                                               Long importJobId,
                                               Long storeId,
                                               /** COMMITTED | READY */
                                               String finalJobStatus,
                                               Integer committedItems,
                                               Integer commitFailedItems,
                                               Integer skippedItems,
                                               String finishedAt) {}

    // ============================================================================
    // 跨服務消費（Media publish，本服務 consume）
    //
    // MediaImportAssetDownloadedEvent 由 common-core 集中定義
    // (io.github.will413028.hawkr.common.mq.events.MediaImportAssetDownloadedEvent)，
    // 兩端共用同一 record 以避免 schema 漂移（docs/開發規範/Java 微服務開發規範.md §15、docs/開發規範/MQ 與事件管理規範.md）。
    // ============================================================================
}
