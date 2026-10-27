package io.github.will413028.hawkr.media.mq.event;

/**
 * Media 內部下載派工事件（08 號設計書 §7.2a）。
 *
 * <p>ImportDownloadAsset RPC handler 在 DB commit 之後 publish 此事件至內部 worker queue，
 * 由 {@link io.github.will413028.hawkr.media.mq.MediaImportAssetDownloadListener} 取出後執行 SafeFetch + OSS 上傳。</p>
 *
 * @param importAssetId import_asset.id（UUID）
 */
public record InternalDownloadTaskEvent(
    String importAssetId
) {
}
