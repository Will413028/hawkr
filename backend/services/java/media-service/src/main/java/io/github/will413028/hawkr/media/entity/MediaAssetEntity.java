package io.github.will413028.hawkr.media.entity;

import com.baomidou.mybatisplus.annotation.*;
import lombok.Data;

import java.io.Serializable;
import java.time.LocalDateTime;

/**
 * 資源資產表 Entity
 * 對應設計書 2.2 media_asset 表
 * <p>
 * 正式的檔案紀錄
 */
@Data
@TableName(value = "media_asset", autoResultMap = true)
public class MediaAssetEntity implements Serializable {
    private static final long serialVersionUID = 1L;

    /**
     * Asset ID（對外流通的 ID）
     * 使用 IdType.INPUT 由服務層手動生成 TSID
     */
    @TableId(type = IdType.INPUT)
    private Long id;

    /**
     * 真實登入帳號 ID（一般買/賣家/後台上傳者），透過 createMediaAsset 寫入。
     * 系統直傳（serverUploadAndFinalize）為 NULL。
     */
    private Long uploaderAccountId;

    /**
     * issue #3149：原始檔名（顯示用，非定址鍵）。
     *
     * <p>一般上傳於 Finalize 時由 upload_session 複製；兩支後端直傳取 RPC 參數。NULL 時讀取端回退至
     * object_key 尾段。
     */
    private String originalFilename;

    /**
     * OSS 最終路徑（於 Finalize 後固定，不可變更）
     */
    private String objectKey;

    /**
     * 資源分類（Finalize 時設定），對應 AssetReferenceType
     */
    private String refType;

    /**
     * 是否為機敏檔案
     */
    private Boolean isSecure;

    /**
     * 機敏檔案專用：流式加密所使用的金鑰（加密存儲）
     */
    private String encryptionKey;

    /**
     * MIME 類型，例如 image/jpeg
     */
    private String mimeType;

    /**
     * 檔案大小（位元組）
     */
    private Long sizeBytes;

    /**
     * 建立時間
     */
    @TableField(fill = FieldFill.INSERT)
    private LocalDateTime createdAt;

    /**
     * issue #2294 L8：GC 排程的<b>租約到期時間</b>（非「已 GC 旗標」）。
     * NULL 或 &lt;= NOW() 表示可搶佔；由 {@code claimAssetForGc} 推進。
     */
    private LocalDateTime gcLeaseUntil;
}
