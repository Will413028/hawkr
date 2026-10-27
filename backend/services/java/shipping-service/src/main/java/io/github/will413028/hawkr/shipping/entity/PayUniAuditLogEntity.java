package io.github.will413028.hawkr.shipping.entity;

import com.baomidou.mybatisplus.annotation.*;
import io.github.will413028.hawkr.common.mybatis.typehandler.JsonbMapTypeHandler;
import io.github.will413028.hawkr.payuni.http.PayUniAuditLogRow;
import lombok.Data;

import java.time.LocalDateTime;
import java.util.Map;

/**
 * PayUni 互動 audit Entity（payuni_audit_logs 專表，與 payment-service 同構）。
 * <p>
 * 承載出向物流互動稽核（建單 / print_label / cat_file_download / query 等），append-only、保留 7 年。
 * request_params / response_decrypted 以 opaque passthrough TypeHandler 忠實保存 PayUni 原生 PascalCase key
 * （見 05 §11.2.1）。收件人 PII 明文保留、憑證欄位遮罩（見 06 §10.5）。
 */
@Data
@TableName(value = "payuni_audit_logs", autoResultMap = true)
public class PayUniAuditLogEntity implements PayUniAuditLogRow {

    @TableId(type = IdType.AUTO)
    private Long id;

    /** OUTBOUND / INBOUND（CodeMeta PayUniAuditDirection，存 Value）。 */
    private String direction;

    /** 交易單號（出向自 request params ShipTradeNo / MerTradeNo；query 類無單號 → null）。 */
    private String tradeNo;

    /** 退貨物流單號。 */
    private String refundNo;

    /** PayUni 商家編號。 */
    private String merId;

    /** API 路徑（如 /api/logistics/print_label、cat_file_download）。 */
    private String apiPath;

    /** 出向業務參數（憑證欄位遮罩；收件人 PII 明文保留）。opaque passthrough。 */
    @TableField(typeHandler = JsonbMapTypeHandler.class)
    private Map<String, Object> requestParams;

    /** 原始回應全文（含 HTML form 全文、加密原文）；不截斷、二進位（PDF）不落內容。 */
    private String responseRaw;

    /** 解密後內層（PayUni 原生 PascalCase key）。opaque passthrough。 */
    @TableField(typeHandler = JsonbMapTypeHandler.class)
    private Map<String, Object> responseDecrypted;

    /** 外層 Status；parse/decrypt/hash 失敗記 PARSE_ERROR / HASH_INVALID。 */
    private String outerStatus;

    /** 外層 Message；二進位下載記 metadata（型別/bytes）。 */
    private String outerMessage;

    /** 傳輸層錯誤碼。 */
    private String errorCode;

    /** 傳輸層錯誤訊息。 */
    private String errorMessage;

    /** HTTP 狀態碼。 */
    private Integer httpStatus;

    /** 呼叫耗時（毫秒）。 */
    private Long durationMs;

    /** 建立時間（MetaObjectHandler 自動填充，§21）。 */
    @TableField(fill = FieldFill.INSERT)
    private LocalDateTime createdAt;
}
