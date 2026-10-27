package io.github.will413028.hawkr.payment.infrastructure.persistence.entity;

import com.baomidou.mybatisplus.annotation.*;
import io.github.will413028.hawkr.common.mybatis.typehandler.JsonbMapTypeHandler;
import io.github.will413028.hawkr.payuni.http.PayUniAuditLogRow;
import lombok.Data;

import java.time.LocalDateTime;
import java.util.Map;

/**
 * PayUni 互動 audit Entity（payuni_audit_logs 專表）。
 * <p>
 * 承載出向（我方 → PayUni）與入向（PayUni → 我方）完整對話稽核，append-only、保留 7 年。
 * request_params / response_decrypted 以 opaque passthrough TypeHandler（裸 ObjectMapper，
 * 不套命名策略）忠實保存 PayUni 原生 PascalCase key（見 05 §11.2.1）。
 */
@Data
@TableName(value = "payuni_audit_logs", autoResultMap = true)
public class PayUniAuditLogEntity implements PayUniAuditLogRow {

    @TableId(type = IdType.AUTO)
    private Long id;

    /** OUTBOUND / INBOUND（CodeMeta PayUniAuditDirection，存 Value）。 */
    private String direction;

    /** 交易單號（出向自 request params MerTradeNo；查詢類 API 無單號 → null）。 */
    private String tradeNo;

    /** 退款單號（退款相關呼叫填）。 */
    private String refundNo;

    /** PayUni 商家編號（HAWK<account_id>）。 */
    private String merId;

    /** API 路徑（如 /api/trade/grant、入向 /webhook/payuni/notify）。 */
    private String apiPath;

    /** 出向業務參數明文（憑證欄位遮罩）；入向解密後 payload。opaque passthrough。 */
    @TableField(typeHandler = JsonbMapTypeHandler.class)
    private Map<String, Object> requestParams;

    /** 原始回應 body 全文（含加密原文 / HTML 全文）；不截斷、二進位不落內容。 */
    private String responseRaw;

    /** 解密後內層（PayUni 原生 PascalCase key）。opaque passthrough。 */
    @TableField(typeHandler = JsonbMapTypeHandler.class)
    private Map<String, Object> responseDecrypted;

    /** 外層 Status；parse/decrypt/hash 失敗記 PARSE_ERROR / HASH_INVALID。 */
    private String outerStatus;

    /** 外層 Message。 */
    private String outerMessage;

    /** 傳輸層錯誤碼（IOException / 非 200 / EMPTY response）。 */
    private String errorCode;

    /** 傳輸層錯誤訊息。 */
    private String errorMessage;

    /** HTTP 狀態碼（IO 錯誤時 0）。 */
    private Integer httpStatus;

    /** 呼叫耗時（毫秒）。 */
    private Long durationMs;

    /** 建立時間（MetaObjectHandler 自動填充，§21）。 */
    @TableField(fill = FieldFill.INSERT)
    private LocalDateTime createdAt;
}
