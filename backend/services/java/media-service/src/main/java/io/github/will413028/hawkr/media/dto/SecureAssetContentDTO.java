package io.github.will413028.hawkr.media.dto;

/**
 * 機敏資產的原始內容（已解密）。
 *
 * <p>{@code ServerReadSecureAsset} 的 Service 層回傳值：把 bytes 還給上傳它的那個服務。
 * 「誰能看」是呼叫端的事，media 只負責「呼叫端說得出的三元組是否真的指向這份資產」。
 *
 * @param fileData    解密後的原始位元組
 * @param contentType 資產落庫的 MIME 類型
 * @param filename    由 object key 尾段推得的檔名
 */
public record SecureAssetContentDTO(byte[] fileData, String contentType, String filename) {
}
