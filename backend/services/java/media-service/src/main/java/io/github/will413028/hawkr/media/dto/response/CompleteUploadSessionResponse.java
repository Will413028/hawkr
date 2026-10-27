package io.github.will413028.hawkr.media.dto.response;

import lombok.Builder;
import lombok.Data;

/**
 * 完成上傳會話回應 DTO
 * 對應 HTTP API: POST /api/v1/app/media/upload_sessions/{id}/complete
 */
@Data
@Builder
public class CompleteUploadSessionResponse {

    /**
     * Staged Token（JWT 格式）
     * 用於後續 FinalizeImages 呼叫
     */
    private String stagedToken;

    /**
     * 預覽 URL（OSS 簽名 URL）
     * 供前端在提交前預覽圖片，具時效性，不落盤
     */
    private String previewUrl;
}
