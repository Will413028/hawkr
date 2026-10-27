package io.github.will413028.hawkr.media.entity;

import com.baomidou.mybatisplus.annotation.FieldFill;
import com.baomidou.mybatisplus.annotation.IdType;
import com.baomidou.mybatisplus.annotation.TableField;
import com.baomidou.mybatisplus.annotation.TableId;
import com.baomidou.mybatisplus.annotation.TableName;
import com.baomidou.mybatisplus.annotation.Version;
import lombok.Data;

import java.io.Serializable;
import java.time.LocalDateTime;

/**
 * Product Import 委派下載資產表 Entity（08 號設計書 §7.1）。
 *
 * <p>生命週期：DOWNLOADING → UPLOADED → FINALIZED（搬到正式路徑）/ FAILED / EXPIRED。
 * 與既有 {@link UploadSessionEntity}（前端上傳）/ {@link MediaAssetEntity}（正式發布）並排。</p>
 */
@Data
@TableName(value = "import_asset", autoResultMap = true)
public class ImportAssetEntity implements Serializable {

    private static final long serialVersionUID = 1L;

    /** UUID（Media 自管，非 TSID） */
    @TableId(type = IdType.INPUT)
    private String id;

    private Long storeId;
    private Long importJobId;
    private Long importItemId;
    private Long callbackCorrelationId;

    /** {@link #SOURCE_TYPE_URL}（外部下載）/ {@link #SOURCE_TYPE_STAGED}（賣家本地上傳 staged_token 綁定）。 */
    private String sourceType;

    /** {@code source_type} 值域：外部 URL 下載。§8 禁止字面值散落。 */
    public static final String SOURCE_TYPE_URL = "URL";

    /**
     * {@code source_type} 值域：賣家本地上傳後以 staged_token 綁定。
     *
     * <p>⚠️ 這個值域<b>沒有</b>對應的 CodeMeta enum——它不跨服務，只在 media-service
     * 內部區分兩條匯入來源。SQL 內另有一處字面值（{@code ImportAssetRepository} 的
     * 三元組查詢），改它要動方法簽章，不在本次範圍。
     */
    public static final String SOURCE_TYPE_STAGED = "STAGED";

    private String sourceUrl;
    private String sourceUrlHash;

    /** source_type=STAGED 時必填，對應 upload_session.id。 */
    private String stagedSessionId;

    /** PRODUCT_IMPORT_PREVIEW | PRODUCT_IMPORT_PREVIEW_DESC */
    private String refType;

    /** 9401 DOWNLOADING / 9402 UPLOADED / 9403 FINALIZED / 9404 FAILED / 9405 EXPIRED */
    private String status;

    private String objectKey;
    private String contentType;
    private Long fileSize;
    private String previewUrl;

    private Long finalizedAssetId;
    private String finalizedObjectKey;
    private String finalizedRefType;

    private String failureReason;
    private String failureDetail;

    private LocalDateTime expiresAt;
    private LocalDateTime finalizedAt;
    private LocalDateTime expiredAt;

    @Version
    private Integer version;

    @TableField(fill = FieldFill.INSERT)
    private LocalDateTime createdAt;

    @TableField(fill = FieldFill.INSERT_UPDATE)
    private LocalDateTime updatedAt;
}
