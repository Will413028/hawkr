package io.github.will413028.hawkr.media.storage;

/**
 * 物件的 HEAD 回查結果（大小 + Content-Type）。
 *
 * <p>finalize 時以此比對 upload_session 宣告值，堵住 presigned PUT 繞過宣告的
 * 超限 / MIME 竄改（49 號設計書 §8.1）。
 */
public record StorageObjectStat(long contentLength, String contentType) {
}
