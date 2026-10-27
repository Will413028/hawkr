package io.github.will413028.hawkr.media.dto;

import lombok.Builder;
import lombok.Data;

/**
 * 已轉正資產 DTO
 * 用於 Service 層傳遞，對應 proto FinalizedAsset
 */
@Data
@Builder
public class FinalizedAssetDTO {

    /**
     * 原始 Staged Token
     */
    private String stagedToken;

    /**
     * 正式資產 ID
     */
    private String assetId;

    /**
     * OSS object key
     */
    private String objectKey;

    /**
     * 是否為機敏檔案
     */
    private Boolean isSecure;
}
