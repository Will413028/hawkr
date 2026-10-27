package io.github.will413028.hawkr.media.entity;

import com.baomidou.mybatisplus.annotation.*;
import lombok.Data;

import java.io.Serializable;
import java.time.LocalDateTime;
import java.util.UUID;

/**
 * 上傳會話表 Entity 對應設計書 2.1 upload_session 表
 * <p>
 * 記錄前端請求上傳的臨時狀態
 */
@Data
@TableName(value = "upload_session", autoResultMap = true)
public class UploadSessionEntity implements Serializable {

    private static final long serialVersionUID = 1L;

    /**
     * Session ID (Token 內含此 ID)
     * 使用 IdType.INPUT 由服務層手動生成 UUID
     */
    @TableId(type = IdType.INPUT)
    private UUID id;

    /**
     * 帳號 ID（上傳者）
     */
    private Long accountId;

    /**
     * issue #3149：使用者上傳時宣告的原始檔名（經 {@code AssetFilename.sanitize} 消毒）。
     *
     * <p>V9 之前的列為 NULL，讀取端回退至 object_key 尾段（設計書 08 §4.9.4）。
     * 這是<b>顯示用</b>的補充資料，不是定址鍵——定址一律用 {@link #objectKey}。
     */
    private String originalFilename;

    /**
     * OSS 上的路徑，例如 tmp/20260129/uuid.jpg
     */
    private String objectKey;

    /**
     * 是否為機敏檔案（採流式加密上傳，禁止直傳 OSS）
     */
    private Boolean isSecure;

    /**
     * MIME 類型（建立時由前端提供）
     */
    private String contentType;

    /**
     * 檔案大小（位元組，建立時由前端提供）
     */
    private Long fileSize;

    /**
     * 機敏檔案專用：AES-GCM 加密金鑰（以主密鑰加密存儲）
     */
    private String encryptionKey;

    /**
     * Finalize 成功後回填的 media_asset ID，用於重入查詢
     */
    private Long assetId;

    /**
     * 狀態碼（使用 MediaUploadSessionStatus.value()） INIT=7001, COMPLETED=7002,
     * FINALIZED=7003, EXPIRED=7004
     *
     * @see io.github.will413028.hawkr.common.utils.code.MediaUploadSessionStatus
     */
    private String status;

    /**
     * Token 有效期
     */
    private LocalDateTime expiresAt;

    /**
     * issue #2294 L8：GC 排程的<b>租約到期時間</b>（非「已 GC 旗標」）。
     * NULL 或 &lt;= NOW() 表示可搶佔；由 {@code claimSessionForGc} 推進。
     */
    private LocalDateTime gcLeaseUntil;

    /**
     * 建立時間
     */
    @TableField(fill = FieldFill.INSERT)
    private LocalDateTime createdAt;

    /**
     * 更新時間
     */
    @TableField(fill = FieldFill.INSERT_UPDATE)
    private LocalDateTime updatedAt;
}
