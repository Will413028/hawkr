package io.github.will413028.hawkr.productimport.entity;

import com.baomidou.mybatisplus.annotation.*;
import lombok.Data;

import java.time.LocalDateTime;

/**
 * Row-level 解析 / 校驗錯誤明細（40 號設計書 §3.5）。BIGSERIAL 主鍵，純內部日誌表。
 *
 * <p>40 §3.5 / §4.3a wipe scope 新增欄位：
 * <ul>
 *   <li>{@code source} — 區分來源（L2_VALIDATOR / VALIDATE_PRODUCT_DRAFT / HTML_PARSE / CERT_VERIFY /
 *       SHIPPING_DOWNGRADE），讓 §5.5 save / §5.7 commit 階段精準清理 VALIDATE_PRODUCT_DRAFT 舊
 *       violations，避免賣家修正後 blocking 重疊計入。</li>
 *   <li>{@code existingProductId / existingProductName / existingProductStatusLabel} — 僅
 *       {@code error_type = DUPLICATE_NAME_VS_EXISTING} 時填值，供前端 DuplicateNameWarning 渲染；
 *       錯誤列表詳情頁讀此 row 即可，無 N+1。</li>
 * </ul>
 *
 * <p>{@code source} 為 NOT NULL DEFAULT 'L2_VALIDATOR'（DB 端 V6 migration 已設定），Entity 上以
 * fill 預設值避免 MyBatis-Plus INSERT 送 NULL 覆蓋 DB DEFAULT（docs/開發規範/Java 微服務開發規範.md §11.2 教訓）。
 */
@Data
@TableName("product_import_item_error")
public class ProductImportItemErrorEntity {

    /** 來源：L2_VALIDATOR — §5.2 CSV 解析階段的 L2 規則 / shipping / cert / duplicate-name 等。 */
    public static final String SOURCE_L2_VALIDATOR = "L2_VALIDATOR";
    /** 來源：VALIDATE_PRODUCT_DRAFT — §4.3a ValidateProductDraft RPC 回傳的 violations（§5.2 g / §5.5 2.7 / §5.7 4.5）。 */
    public static final String SOURCE_VALIDATE_PRODUCT_DRAFT = "VALIDATE_PRODUCT_DRAFT";
    /** 來源：HTML_PARSE — §12 HTML→JSON 轉換產生的 warning。 */
    public static final String SOURCE_HTML_PARSE = "HTML_PARSE";

    @TableId(type = IdType.AUTO)
    private Long id;

    private Long importItemId;
    private Long importJobId;
    private Integer csvLine;

    /** {@link io.github.will413028.hawkr.common.utils.code.ImportItemErrorType#value()} */
    private String errorType;
    /** ERROR | WARNING */
    private String severity;
    private String field;
    private String message;
    private String rawValue;

    /**
     * 來源（L2_VALIDATOR / VALIDATE_PRODUCT_DRAFT / HTML_PARSE / CERT_VERIFY / SHIPPING_DOWNGRADE）。
     * 預設 L2_VALIDATOR；INSERT 前若未顯式 set 由 default 帶；§5.5 / §5.7 對 VALIDATE_PRODUCT_DRAFT
     * 來源做 wipe scope 篩選用。
     */
    private String source;

    /** 同名既有商品 id — 僅 error_type=DUPLICATE_NAME_VS_EXISTING 時填值。 */
    private Long existingProductId;
    /** 同名既有商品 name — 同上。 */
    private String existingProductName;
    /** 同名既有商品 status 中文標籤（上架中 / 下架 / 草稿 / 待審核）— 同上。 */
    private String existingProductStatusLabel;

    @TableField(fill = FieldFill.INSERT)
    private LocalDateTime createdAt;
}
