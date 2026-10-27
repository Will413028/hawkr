package io.github.will413028.hawkr.productimport.entity;

import com.baomidou.mybatisplus.annotation.*;
import lombok.Data;

import java.time.LocalDateTime;

/**
 * Media callback event 去重 inbox（40 號設計書 §3.6a）。
 *
 * <p>輕量 inbox（單表 PK 去重）— Listener 開頭 INSERT ... ON CONFLICT DO NOTHING，
 * 影響筆數=0 即重複事件，直接 ACK 跳過。
 */
@Data
@TableName("product_import_media_callback_inbox")
public class ProductImportMediaCallbackInboxEntity {

    /** UUID（由 Media 端產生）。 */
    @TableId(type = IdType.INPUT)
    private String eventId;

    private String importAssetId;
    /** UPLOADED | FAILED */
    private String resultStatus;

    @TableField(fill = FieldFill.INSERT)
    private LocalDateTime processedAt;
}
