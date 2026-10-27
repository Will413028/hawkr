package io.github.will413028.hawkr.payment.infrastructure.persistence.entity;

import com.baomidou.mybatisplus.annotation.*;
import lombok.Data;

import java.time.LocalDateTime;

/**
 * 支付交易 Entity
 */
@Data
@TableName("payment_transactions")
public class PaymentTransactionEntity {

    @TableId(type = IdType.INPUT)
    private Long id;

    /**
     * 系統交易編號 = {orderNo}{3位序號}（如 O382716459001），即 PayUni MerTradeNo
     */
    private String tradeNo;

    /**
     * 原始訂單編號（如 O382716459），同一訂單的所有 row 共用
     */
    private String orderId;

    /**
     * 同一訂單的第 N 次付款嘗試（從 1 開始遞增）
     */
    private Integer attemptSeq;

    /**
     * 商家編號（選填）
     */
    private String merId;

    /**
     * 買家 buyer_id（2026-08-02 由 account_id 改名，值本來就是買家身份）
     */
    private String buyerId;

    /**
     * 支付方式 (1:信用卡, 2:ApplePay, 3:GooglePay, 4:ATM, 6:LinePay)
     */
    private String paymentType;

    /**
     * 交易金額（新台幣元）
     */
    private Long amount;

    /**
     * 商品名稱
     */
    private String productName;

    /**
     * 交易狀態 (0:INIT, 1:PAID, 2:FAILED, 3:EXPIRED, 4:CONFLICT, 5:CLOSED)
     */
    private String status;

    /**
     * PayUni 交易編號
     */
    private String providerTradeNo;

    /**
     * PayUni TradeStatus
     */
    private String providerStatus;

    /**
     * PayUni 回傳訊息
     */
    private String providerMessage;

    /**
     * PayUni 撥款成功時間（里程碑）
     * 遵循 docs/開發規範/PayUni 錯誤回應收斂規範.md §6.2 撥款欄位封閉規則：僅 Admin API 可暴露。
     */
    private LocalDateTime grantedAt;

    /**
     * PayUni /api/merchant/grant 回應狀態碼（Admin only）
     */
    private String grantStatus;

    /**
     * PayUni 撥款回應訊息（Admin only）
     */
    private String grantMessage;

    /**
     * ATM 銀行代碼
     */
    private String atmBankCode;

    /**
     * ATM 虛擬帳戶
     */
    private String atmVirtualAccount;

    /**
     * ATM 繳費期限
     */
    private java.time.LocalDate atmExpireDate;

    /**
     * 付款完成時間
     */
    private LocalDateTime payTime;

    /**
     * 關閉時間
     */
    private LocalDateTime closedAt;

    /**
     * 逾期時間
     */
    private LocalDateTime expireAt;

    /**
     * L2 補救路徑：最近一次 reconcile 處理時間
     */
    private LocalDateTime lastReconciledAt;

    /**
     * L2 補救路徑：rescue 退款觸發時間
     */
    private LocalDateTime rescuedAt;

    /**
     * L2 補救路徑：rescue 動作結果（REFUNDED / REFUND_PENDING_MANUAL）
     */
    private String rescueAction;

    /** §2A.6 是否為重複付款被退的副 row */
    private Boolean isDuplicate;
    /** §2A.6 指向「主」row 的 id */
    private Long duplicateOfPaymentId;
    /** §2A.6 同 order_id 第幾次重複（1, 2, ...） */
    private Integer duplicateSequence;

    /** §10.5 PayUni 認為交易建立的時間 */
    private LocalDateTime providerCreateAt;
    /** §10.5 PayUni 認為使用者真實付款時間 */
    private LocalDateTime providerPaidAt;

    /** §2A.5.4 ATM row 最後一次復活時間 */
    private LocalDateTime reactivatedAt;

    /** §5.9 請求端類型（CodeMeta ClientType value：MOBILE_APP / WEB_BROWSER） */
    private String clientType;

    /** §5.9 App 身份（CodeMeta AppType value：BUYER / SELLER） */
    private String appType;

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

    /**
     * 版本控制（樂觀鎖）
     */
    @Version
    private Integer version;
}
