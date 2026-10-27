package io.github.will413028.hawkr.media.dto.request;

import jakarta.validation.constraints.NotBlank;
import jakarta.validation.constraints.NotNull;
import jakarta.validation.constraints.Positive;
import lombok.AllArgsConstructor;
import lombok.Builder;
import lombok.Data;
import lombok.NoArgsConstructor;

/**
 * 建立上傳會話請求 DTO
 * 對應 HTTP API: POST /api/v1/app/media/upload_sessions
 */
@Data
@Builder
@NoArgsConstructor
@AllArgsConstructor
public class CreateUploadSessionRequest {

    /**
     * 檔案名稱
     */
    @NotBlank(message = "檔案名稱不可為空")
    private String filename;

    /**
     * 內容類型（MIME type）
     */
    @NotBlank(message = "內容類型不可為空")
    private String contentType;

    /**
     * 檔案大小（位元組）
     */
    @NotNull(message = "檔案大小不可為空")
    @Positive(message = "檔案大小必須為正數")
    private Long fileSize;

    /**
     * 是否為機敏檔案（採流式加密上傳）
     */
    private Boolean isSecure = false;
}
