package io.github.will413028.hawkr.media.dto.response;

import lombok.Builder;
import lombok.Data;

/**
 * 建立上傳會話回應 DTO
 * 對應 HTTP API: POST /api/v1/app/media/upload_sessions
 */
@Data
@Builder
public class CreateUploadSessionResponse {

    /**
     * 會話 ID
     */
    private String sessionId;

    /**
     * OSS 上傳 URL（僅公開上傳時提供）
     */
    private String uploadUrl;

    /**
     * 有效期（秒）
     */
    private Integer expiresIn;

    /**
     * 是否為機敏檔案
     */
    private Boolean isSecure;
}
