package io.github.will413028.hawkr.media.dto;

import lombok.Builder;
import lombok.Data;

import java.time.LocalDateTime;
import java.util.UUID;

/**
 * Staged Token 解析後的 Claims
 * <p>
 * Token 格式依設計書 3.2.1： - session_id - account_id - expires_at - is_secure - scope
 */
@Data
@Builder
public class StagedTokenClaims {

    /**
     * 上傳會話 ID
     */
    private UUID sessionId;

    /**
     * 帳號 ID
     */
    private Long accountId;

    /**
     * 過期時間
     */
    private LocalDateTime expiresAt;

    /**
     * 是否為機敏檔案
     */
    private Boolean isSecure;

    /**
     * Token 範圍（scope）
     *
     * @see io.github.will413028.hawkr.common.utils.code.StagedTokenScope
     */
    private String scope;
}
