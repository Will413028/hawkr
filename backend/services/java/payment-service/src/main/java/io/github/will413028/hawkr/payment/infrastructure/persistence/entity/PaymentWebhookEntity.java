package io.github.will413028.hawkr.payment.infrastructure.persistence.entity;

import com.baomidou.mybatisplus.annotation.*;
import io.github.will413028.hawkr.payment.infrastructure.persistence.typehandler.JsonbStringTypeHandler;
import lombok.Data;

import java.time.LocalDateTime;

/**
 * Webhook 記錄 Entity
 *
 * 記錄 MQ 消費的付款通知結果
 */
@Data
@TableName("payment_webhooks")
public class PaymentWebhookEntity {

    @TableId(type = IdType.AUTO)
    private Long id;

    /**
     * Webhook 類型（PAYUNI_PAYMENT）
     */
    private String webhookType;

    /**
     * 對應系統交易編號（付款 Webhook 使用）
     */
    private String tradeNo;

    /**
     * PayUni 交易編號（付款 Webhook 使用）
     */
    private String providerTradeNo;

    /**
     * 冪等鍵（依 webhook_type 組合不同欄位）
     */
    private String idempotencyKey;

    /**
     * 原始請求內容
     */
    private String rawRequest;

    /**
     * 解密後資料（JSONB）
     */
    @TableField(typeHandler = JsonbStringTypeHandler.class)
    private String decryptedPayload;

    /**
     * 驗證是否通過
     */
    private Boolean isValid;

    /**
     * 是否已處理
     */
    private Boolean isProcessed;

    /**
     * 處理狀態（PENDING, SUCCESS, FAILED, SKIPPED, VERIFY_FAILED, DECRYPT_FAILED）
     */
    private String processStatus;

    /**
     * 錯誤訊息
     */
    private String errorMessage;

    /**
     * 接收時間
     */
    @TableField(fill = FieldFill.INSERT)
    private LocalDateTime receivedAt;

    /**
     * 處理時間
     */
    private LocalDateTime processedAt;

    /**
     * 來源 IP
     */
    private String sourceIp;

    /**
     * User-Agent
     */
    private String userAgent;
}
