package io.github.will413028.hawkr.aftersales.entity;

import com.baomidou.mybatisplus.annotation.*;
import lombok.Data;

import java.time.LocalDate;
import java.time.LocalDateTime;

@Data
@TableName("refund_tasks")
public class RefundTaskEntity {

    @TableId(type = IdType.INPUT)
    private Long id;

    private String caseNo;

    private Long cancelId;

    private String orderNo;

    private String transactionId;

    private Long refundAmount;

    private String paymentMethod;

    private String merId;

    /**
     * 退款 outcome：FULL_REFUND / PARTIAL_REFUND（{@link io.github.will413028.hawkr.common.utils.code.AfterSalePlatformDecisionOutcome} value）。
     * case_no path 必填（由 V3 CHECK constraint 保證）；cancel_id path 可 NULL（cancel.completed event 下游不需 outcome）。
     * 由 5 處建 task 入口（AfterSaleResolutionService / PlatformDecisionService / ReturnReceiptService）依語意 normalize 後寫入。
     */
    private String refundOutcome;

    private String status;

    private String externalRefId;

    private String lastError;

    private Integer retryCount;

    private Integer maxRetryCount;

    private LocalDateTime nextRetryTime;

    private String payuniStatus;

    private String payuniMessage;

    private LocalDate estimatedRefundDate;

    private LocalDate bankDate;

    private String clientType;

    private String appType;

    /**
     * 當前對 PayUni 發 RFD 的 attempt 序號。
     * §2.2 業務 pivot (2026-05-14)：每次 ATM 退款失敗退回 AWAITING_BUYER_REFUND_PAGE(5) 時 +1，
     * 供 idempotency_key 構成 {@code :{attempt}} 後綴，避免 buyer 重填時命中既有 payment_refunds row 而不發新 RFD。
     */
    private Short currentAttempt;

    /**
     * issue #2294 L5：{@code RefundSyncWorker}（補發遺漏 MQ 通知）的<b>租約欄</b>，
     * <b>不是「已補發旗標」</b> —— 值 = 本次搶到的 pod 持有租約到期時間（= now + 3 小時 = 排程週期 × 3）。
     * {@code NULL} 或已過期即代表可被重新搶佔。
     *
     * <p>⚠️ <b>不可挪用 {@code updated_at} 當租約欄</b>：它由 {@code MetaObjectHandler} 統一維護，
     * 任何寫入都會動它，而 {@code RefundSyncWorker} 的候選述詞正是 {@code updated_at < NOW()-1h}
     * —— 拿它當租約會讓「租約」被無關的更新重置、也會改變候選判定本身。
     */
    private LocalDateTime resyncAt;

    @TableField(fill = FieldFill.INSERT)
    private LocalDateTime createdAt;

    @TableField(fill = FieldFill.INSERT_UPDATE)
    private LocalDateTime updatedAt;
}
