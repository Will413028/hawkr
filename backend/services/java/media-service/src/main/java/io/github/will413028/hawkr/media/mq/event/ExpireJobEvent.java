package io.github.will413028.hawkr.media.mq.event;

/**
 * Product Import Service GC 觸發的 ImportJob 過期事件（40 號設計書 §6 / 08 號設計書 §7.5）。
 *
 * <p>由 ProductImportGcJob publish 至 {@code hawkr.product.import} exchange +
 * {@code gc.expire_job} routing key；Media 端 {@link io.github.will413028.hawkr.media.mq.MediaImportGcListener}
 * 消費後將該 import_job_id 下所有非終態 import_asset 標 EXPIRED 並刪 OSS tmp 物件。</p>
 *
 * @param importJobId import_job.id
 */
public record ExpireJobEvent(
    Long importJobId
) {
}
