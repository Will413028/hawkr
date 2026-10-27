package io.github.will413028.hawkr.media.dto.request;

import lombok.Data;

/**
 * 完成上傳會話請求 DTO
 * 對應 HTTP API: POST /api/v1/app/media/upload_sessions/{id}/complete
 *
 * 註：目前後端不使用這些欄位，保留供未來擴展（如 ETag 驗證）
 */
@Data
public class CompleteUploadSessionRequest {

    /**
     * OSS 回傳的 ETag（可選，目前未使用）
     */
    private String etag;

    /**
     * MIME 類型（可選，目前未使用）
     */
    private String mimeType;

    /**
     * 檔案大小（位元組）（可選，目前未使用）
     */
    private Long sizeBytes;
}
