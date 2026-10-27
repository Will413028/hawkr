package io.github.will413028.hawkr.aftersales.entity;

import com.baomidou.mybatisplus.annotation.*;
import lombok.Data;

import java.time.LocalDateTime;

@Data
@TableName("aftersale_cases")
public class AfterSaleCaseEntity {

    @TableId(type = IdType.INPUT)
    private Long id;

    private String caseNo;

    private String orderNo;

    private Long buyerId;

    private Long storeId;

    private String status;

    private String reason;

    private String description;

    private Long refundableAmount;

    private Long shippingFee;

    private String deliveryTemp;

    private String buyerName;

    private String buyerPhone;

    private String buyerCity;

    private String buyerDistrict;

    private String buyerAddressDetail;

    private String buyerPostalCode;

    private String transactionId;

    private String paymentMethod;

    private String merId;

    private String shippingMethod;

    private String returnShipmentNo;

    private String returnShippingMethod;

    private String shipTradeNo;

    private String partnerId;

    private String odno;

    private String validationNo;

    /**
     * 7-11 退貨代碼自動重試的「已確認觸發次數」預算（0 = 尚未觸發、1 = 已成功觸發一次、
     * 2 = 已耗盡待 ops 介入）。
     *
     * <p><b>issue #2294 L7</b>：這<b>不是</b>搶佔閘門——閘門是 {@link #labelRetryAt} 租約。
     * 本欄只在 shipping {@code RetryReturnPrintLabel} <b>確認成功之後</b>才寫，
     * 所以一次 gRPC 失敗不會消耗掉這個案件唯一一次自動重試。
     */
    private Short labelRetryCount;

    /**
     * issue #2294 L7：7-11 退貨代碼自動重試的<b>租約到期時間</b>（V9 新增）。
     *
     * <p><b>三態</b>（互斥且完整，兩個消費述詞都靠它）：
     * <ul>
     *   <li>{@code NULL} ＝ <b>從未嘗試過</b>（新案件，或 ops 重置）——不算耗盡</li>
     *   <li><b>過去</b>時間 ＝ 嘗試過但目前無人持有（可立即搶佔，且已留下「試過」的證據）</li>
     *   <li><b>未來</b>時間 ＝ 正在嘗試中（租約有效，不可搶佔）</li>
     * </ul>
     * <p>⚠️ 這是租約不是「已重試旗標」：持有者在 claim 與 gRPC 之間消失時，
     * 租約到期該列自動回到候選集合（規範《多副本排程搶佔規範》§二 第 4 步）。
     * <p>⚠️ <b>不可挪用 {@code label_retry_count}</b> 當閘門：它是業務預算，
     * CAS 成功即永久離開候選集合、無時間欄可讓它回來 —— 那正是本 issue 修掉的缺陷。
     * <p>🚨 <b>一旦被 claim 過就不得再回到 {@code NULL}</b>（唯一例外＝ops 的
     * {@code resetLabelRetryCount}）：耗盡掃描以 {@code IS NOT NULL} 當「試過」的證據，
     * 失敗釋放若還原成 {@code NULL}，{@code LABEL_RETRY_EXHAUSTED} 告警會實質失效。
     */
    @TableField("label_retry_at")
    private LocalDateTime labelRetryAt;

    private LocalDateTime returnDeliveredAt;

    /** §5.3.10 自動確認收貨截止時間（= return_delivered_at + AUTO_CONFIRM_RECEIPT_HOURS；AutoConfirmReceiptScheduler 掃此欄位） */
    private LocalDateTime autoConfirmAt;

    /** §5.3.11 PayUni 退貨物流原始狀態碼（status=28 RETURN_FAILED 時填入；2026-05-22 起碼 55 賣家未取也會 stamp 但不變狀態） */
    private String returnProviderStatus;

    /** §5.3.11 PayUni 解密後錯誤訊息原文（status=28 時填入，回傳前端顯示） */
    private String returnProviderMessage;

    // ==================== 2026-05-22 逆物流退貨政策修訂 ====================

    /** §5.3.10.2 超商退貨到店時間（PayUni 碼 32 觸發 25→26 流轉時 stamp） */
    private LocalDateTime storeArrivedAt;

    /** §5.3.10.2 超商待取逾時截止（= store_arrived_at + STORE_PICKUP_TIMEOUT_HOURS；StorePickupTimeoutScheduler 掃此欄位） */
    private LocalDateTime storePickupDeadlineAt;

    /** §5.3.10.2 超商賣家實際取貨時間（PayUni 碼 11 觸發 26→27 流轉時 stamp） */
    private LocalDateTime sellerPickedUpAt;

    /** §5.3.10 系統實際自動完成時間戳（區別於 deadline 欄位的「截止時間」語意；status=31/32 時必填） */
    private LocalDateTime autoCompleteAtStamp;

    /**
     * §5.3.10.3 超商待取剩 24h 預警 FCM 已發時間戳（<b>事後記錄，非冪等閘門</b>；issue #2294 A1 例外 B：
     * publish 成功後才寫，故 NULL <b>不保證未發</b> —— 冪等由 notification 端
     * {@code notifications_inbox_dedup_idx} 承擔）。候選述詞仍讀它，作用是「已成功記錄過的不必再撈」。
     */
    @TableField("warned_store_pickup_24h_at")
    private LocalDateTime warnedStorePickup24hAt;

    /** §5.3.10.3 確認收貨剩 24h 預警 FCM 已發時間戳（<b>事後記錄，非冪等閘門</b>；同上，見 issue #2294 A2） */
    @TableField("warned_confirm_receipt_24h_at")
    private LocalDateTime warnedConfirmReceipt24hAt;

    // ==================== 2026-06-15 待寄回逾時政策修訂 ====================

    /** §5.3.8 買家寄回退貨逾時截止（= NOW + return.timeout-days；於 0/65 → 20 流轉當下 stamp；ReturnTimeoutWorker 與 ReturnTwentyFourHourWarnScheduler 掃此欄位） */
    private LocalDateTime shipBackDeadlineAt;

    /**
     * §5.3.10.3 待寄回剩 24h 預警 FCM 已發時間戳（<b>事後記錄，非冪等閘門</b>；同 warnedStorePickup24hAt，
     * 見 issue #2294 A3）。
     */
    @TableField("warned_ship_back_24h_at")
    private LocalDateTime warnedShipBack24hAt;

    /**
     * §5.21 issue #2294 L6：{@code ReturnStuckAlertScheduler}（RETURN_STUCK 卡死告警）的<b>租約欄</b>，
     * <b>不是「已告警旗標」</b> —— 值 = 本次搶到的 pod 持有租約到期時間（= now + {@code returnStuckDays}，
     * 與告警冷卻同值）。{@code NULL} 或已過期即代表可被重新搶佔。
     *
     * <p>⚠️ <b>不可挪用 {@code updated_at}</b>：那是本排程的候選判定欄（{@code updated_at < NOW()-Nd}），
     * 寫它會改變候選判定本身。
     * <p>⚠️ 卡死告警<b>不得走例外 B</b>：狀態沒解除時再發一次對收件人仍有新資訊（沒人處理就該一直吵），
     * 所以必須 lease、必須會重發（與 24h 預警方向相反，見 09 §8.5）。
     */
    @TableField("return_stuck_alerted_at")
    private LocalDateTime returnStuckAlertedAt;

    private LocalDateTime deliveredAt;

    private LocalDateTime closedAt;

    // §10 申訴與平台裁決
    private String appealState;

    private LocalDateTime appealSubmittedAt;

    private LocalDateTime appealResolvedAt;

    private LocalDateTime lastRejectedAt;

    // §10.6.6 申訴內容結構化欄位（供 Admin 詳情 API 直讀，SubmitAppeal 時分別寫入）
    private String appealReason;

    private String appealDescription;

    // §5.10 賣家處理期限自動升級
    private LocalDateTime processDeadline;

    private Boolean autoEscalated;

    private LocalDateTime escalatedAt;

    @TableField(fill = FieldFill.INSERT)
    private LocalDateTime createdAt;

    @TableField(fill = FieldFill.INSERT_UPDATE)
    private LocalDateTime updatedAt;
}
