package io.github.will413028.hawkr.media.dto;

/**
 * 資產中介資料（#3031）。**不含 bytes**——需要內容的走 ServerReadSecureAsset。
 *
 * <p>全欄字串：對外 response 的數字與時間一律 string（10 檔 §4）。
 */
public record AssetMetadataDTO(
    String objectKey,
    String assetId,
    String fileName,
    String sizeBytes,
    String uploadedAt,
    String mimeType
) {
}
