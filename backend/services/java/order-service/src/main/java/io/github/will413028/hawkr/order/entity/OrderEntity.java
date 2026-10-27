package io.github.will413028.hawkr.order.entity;

import com.baomidou.mybatisplus.annotation.*;
import io.github.will413028.hawkr.order.config.typehandler.ReviewSnapshotTypeHandler;
import io.github.will413028.hawkr.order.config.typehandler.StringListTypeHandler;
import lombok.Data;

import java.time.LocalDateTime;
import java.util.List;

/**
 * 訂單主表 Entity
 */
@Data
@TableName("orders")
public class OrderEntity {

    @TableId(type = IdType.INPUT)
    private Long id;

    private String orderNo;

    private Long buyerId;

    private Long storeId;

    private Long totalOriginalAmount;

    private Long totalActualAmount;

    private String status;

    private String paymentMethod;

    private String transactionId;

    private String checkoutId;

    /** 配送溫層 Enum value（如 3201、3202） */
    private String deliveryTemp;

    /** 運送方式 Enum value（如 3001） */
    private String shippingMethod;

    /** 運費（新台幣元） */
    private Long shippingFee;

    /** 運費負擔方 Enum value（如 3102） */
    private String shippingFeePayer;

    /** 收件人姓名 */
    private String recipientName;

    /** 收件人電話 */
    private String recipientPhone;

    /** 收件地址（宅配） */
    private String shippingAddress;

    /** 配送備註 */
    private String shippingNote;

    /** 訂單備註（買家下單時留言給賣家，選填） */
    private String orderRemark;

    /** 超商門市代號（店到店；與賣家 store_id surrogate 無關，見 Plan C Phase 2 collision note） */
    private String cvsStoreId;

    /** 超商門市名稱（店到店） */
    private String cvsStoreName;

    /** ATM 銀行代碼（建單時快照，僅 ATM 付款） */
    private String atmBankCode;

    /** ATM 虛擬帳號（建單時快照，僅 ATM 付款） */
    private String atmVirtualAccount;

    /** ATM 繳費期限（建單時快照，僅 ATM 付款，ISO 8601） */
    private String atmExpireDate;

    /** 材積總和（cm³，用於宅配 Spec 計算） */
    private Integer dimensionSum;

    /** 配送區域 Enum value（如 3401、3402） */
    private String shippingArea;

    /** 出貨城市（建單時從 Shipping Service 快照） */
    private String senderCity;

    /** 出貨地區（建單時從 Shipping Service 快照） */
    private String senderDistrict;

    /** PayUni 商家編號 */
    private String merId;

    // ─── 政策快照五欄（65 Foundation，V14）────────────────────────────────
    // 訂單成立後不因賣家後續修改物流／收款設定而改變既有訂單的履約、收款或結算語意。
    // ⚠️ 第 5 個維度（provider）不在這五欄內：shippingMethod 已經承載它，再開一欄就是第二個真相。

    /**
     * 商流模式快照：{@code SELLER_DIRECT} / {@code PLATFORM_CONSIGNMENT}
     * （DB CHECK {@code chk_orders_commerce_model}；存量列為 {@code SELLER_DIRECT}）。
     * <p>撥款資格的軸守門：非 {@code PLATFORM_CONSIGNMENT} 一律 {@code NOT_APPLICABLE}。
     */
    private String commerceModel;

    /**
     * 收款模式快照：{@code SELLER_MERCHANT} / {@code PLATFORM_AGENT_MERCHANT} /
     * {@code PROVIDER_COLLECTED} / {@code EXTERNAL_PREPAID}
     * （DB CHECK {@code chk_orders_collection_model}）。
     * <p>🔴 {@code resolvePaymentMerId} 必須由本欄驅動，不得再讀 buyer request 透傳的 consolidated。
     * <p>DB CHECK {@code chk_orders_policy_matrix} 綁死本欄與 {@link #commerceModel} 的對應：
     * {@code PLATFORM_CONSIGNMENT} ⇔ {@code PLATFORM_AGENT_MERCHANT}。
     */
    private String collectionModel;

    /**
     * 履約模式快照：{@code SELLER_MANAGED} / {@code PLATFORM_LOGISTICS}
     * （DB CHECK {@code chk_orders_fulfillment_mode}）。
     * <p>與 {@link #shippingMethod} 不完全正交：{@code 3005}（商家自行配送）必然是
     * {@code SELLER_MANAGED}（DB CHECK {@code chk_orders_fulfillment_mode_consistent}）。
     */
    private String fulfillmentMode;

    /**
     * 開票模式快照：{@code SELLER_TO_BUYER} / {@code PLATFORM_TO_BUYER}
     * （DB CHECK {@code chk_orders_invoice_model}）。
     * <p>開票 issuer 必須與收款主體一致：DB CHECK {@code chk_orders_invoice_collection} 綁死
     * {@code PLATFORM_TO_BUYER} ⇔ {@link #collectionModel} = {@code PLATFORM_AGENT_MERCHANT}。
     */
    private String invoiceModel;

    /** 政策版本快照；存量列回填為 {@code v0-legacy}。 */
    private String policyVersion;

    /**
     * EXTERNAL lane RFQ 成交的 buyer acceptance id（ai-service 的
     * {@code procurement_buyer_quote_acceptances.id}）；非 RFQ 訂單為 NULL。
     *
     * <p>這是建單的冪等鍵：命令由 ai-service 的 outbox worker 投遞，重試必然發生，
     * {@code uk_orders_acceptance_id}（偏索引）保證一筆成交至多一張訂單。
     * ⛔ 不得改用 {@code checkoutId} 承載：它非空就會觸發購物車鎖。64 號 §6.4.7。
     *
     * <p>⚠️ 只有 EXTERNAL lane 有值。PLATFORM lane 的 RFQ 成交走現行 createOrder
     * （有真 SKU 與真庫存，賣方就是那個平台賣家）。
     */
    private Long acceptanceId;

    /** 優惠券**實例** ID（reserveCoupon 後寫入；非 template id） */
    private String couponInstanceId;

    /** 優惠券折扣金額（新台幣元） */
    private Long discountAmount;

    /**
     * 賣家負擔的商品折扣（新台幣<b>元</b>的整數；禁止 BigDecimal / Double）。費基公式的減項。
     * <p>{@link #discountAmount} 是單一總額、沒有 bearer 維度 ⇒ 沒有本欄就算不出 fee_base，
     * 而症狀是「算出一個看起來合理但錯的數字」，不是報錯。
     * <p>⚠️ 連帶契約：coupon-service 必須在核銷回應中回傳 bearer 拆分，否則 order-service 沒有值可寫
     * （本波為 open item，存量與新單皆落 0；DB CHECK 只保證 &gt;= 0）。
     */
    private Long sellerBorneDiscountAmount;

    /**
     * 累計已完成退款金額（新台幣元，03 §3.1）。由 aftersales.refund.completed（PARTIAL outcome）
     * 消費時同事務累加；=0 表示無已完成退款。完成訂單時撥款任務分流依據（03 §3.11.1 / §3.11.2）。
     */
    private Long refundTotalAmount;

    private String shipmentNo;

    /** 物流狀態碼（ShippingStatus value） */
    private String shippingStatus;

    /** 物流狀態描述（PayUni ShipStatusDesc 或 ShippingStatus.displayName） */
    private String shippingStatusDesc;

    /** 宅配託運單 PDF CDN URL（Shipping Service MQ 事件快照，僅宅配有值） */
    private String labelUrl;
    /** 宅配託運單 PDF 下載期限（取號時 NOW()+24h，僅前端按鈕判斷用） */
    private LocalDateTime labelExpireAt;

    /** 超商出貨便代碼（PartnerId，超商取貨用） */
    private String partnerId;

    /** 超商訂單編號（odno，超商取貨用） */
    private String odno;

    /** 超商驗證碼（validationNo，超商取貨用） */
    private String validationNo;

    /** 出貨照片 objectKey 陣列（Shipping Service MQ 事件快照，JSONB；顯示 URL 由 Gateway 組，49 §5.3/§6.2） */
    @TableField(typeHandler = StringListTypeHandler.class)
    private List<String> shipmentPhotoKeys;

    /** 買家確認收貨上傳照片 objectKey 陣列（ConfirmReceipt 寫入，JSONB，選填；顯示 URL 由 Gateway 組） */
    @TableField(typeHandler = StringListTypeHandler.class)
    private List<String> receiptPhotoKeys;

    /**
     * 訂單評論快照（JSONB；NULL = 未評論）。取該訂單最近一筆商品評論，
     * 來源 review.created MQ 事件，評論不可改/刪 → 凍結快照（03 設計書 §8.9）。
     */
    @TableField(typeHandler = ReviewSnapshotTypeHandler.class)
    private ReviewSnapshot review;

    /** 確認收貨時間（買家手動 / 自動確認時回填） */
    private LocalDateTime confirmedAt;

    /** 確認收貨方式：MANUAL（買家手動）/ AUTO（排程自動）。E 軸落地後降為 {@link #receiptStatus} 的附屬事實欄。 */
    private String confirmType;

    /**
     * E 軸（收貨確認）狀態：{@code PENDING} / {@code BUYER_CONFIRMED} / {@code AUTO_CONFIRMED} /
     * {@code REVIEW_PENDING} / {@code ADMIN_CONFIRMED} / {@code ADMIN_REJECTED} / {@code DISPUTED}
     * （DB CHECK {@code chk_orders_receipt_status}）。
     *
     * <p>🔴 <b>NULL 的語意是「這條軸還沒開始」，不是一個狀態值</b> ⇒ aging 指標必須排除 NULL 列，
     * 否則所有未送達訂單都會被算成「卡在 receipt 軸」。
     *
     * <p>DB 側兩條不變式：{@code chk_orders_receipt_axis_present}（{@link #deliveredAt} 非 NULL
     * ⇒ 本欄必須非 NULL）、{@code chk_orders_receipt_entered_pair}（本欄與
     * {@link #receiptStatusEnteredAt} 必須同時為 NULL 或同時非 NULL）。
     * ⚠️ 寫 {@code delivered_at} 的 SQL 若不同句寫本欄，會直接 23514 失敗。
     */
    private String receiptStatus;

    /**
     * 進入當前 {@link #receiptStatus} 的時間。與 {@link #receiptStatus} 成對
     * （DB CHECK {@code chk_orders_receipt_entered_pair}）。
     * <p>E 軸的 aging 一律錨在本欄，不得用 {@code updated_at}（同 shipments.status_entered_at 的理由）。
     */
    private LocalDateTime receiptStatusEnteredAt;

    /**
     * 人工裁決期限。{@link #receiptStatus} = {@code REVIEW_PENDING} 且本欄到期者為排程候選
     * （部分索引 {@code idx_orders_receipt_review_due}）。NULL = 不需人工裁決。
     */
    private LocalDateTime receiptReviewDueAt;

    /** 付款期限（超過此時間自動取消訂單） */
    private LocalDateTime paymentDeadline;

    /** 付款嘗試次數 */
    private Integer paymentAttempts;

    private LocalDateTime payTime;

    private LocalDateTime shipTime;

    /** 自動完成時間（送達後 60 小時；遇 AfterSales case 活躍會延後，03 §5.8） */
    private LocalDateTime autoCompleteAt;

    /** 倒數暫停時刻（NULL=倒數中；NOT NULL=已暫停，scheduler 跳過）；由 AfterSales MQ 事件觸發（03 §5.8） */
    private LocalDateTime countdownPausedAt;

    /** 累計暫停次數（觀測用） */
    private Integer countdownPauseCount;

    /** 賣家處理期限快照（UTC，09 §5.9）；shipment.created 寫入、shipment.processing.cleared 清空。NULL = Uber/未付款/已處理/已取消 */
    private LocalDateTime processingDeadline;

    /** 付款後建單重試次數（Backstop，03 §8.11） */
    private Integer shipmentCreateAttempts;

    /** 下次建單重試時間（backoff，Backstop，03 §8.11） */
    private LocalDateTime shipmentCreateNextRetryAt;

    /** 取消原因 value（OrderCancelReason，91xx 系統 / 92xx 賣家；buyer 自願取消為 NULL，03 §5.3.1） */
    private String cancelReason;

    /** 取消備註（賣家自填，僅 OTHER / SUGGEST_SPLIT 場景使用） */
    private String cancelRemark;

    /** 取消者角色（OrderCancelByRole：SYSTEM / SELLER / BUYER；03 §5.3.1） */
    private String cancelByRole;

    /** 取消時間（三支取消 CAS 於狀態變更同句寫入；NULL = 未取消，或取消於 V11 之前無法回填） */
    private LocalDateTime cancelledAt;

    /** 送達時間（訂單轉 DELIVERED 時記錄，供 AfterSales 時效驗證） */
    private LocalDateTime deliveredAt;

    /** ATM 退款等待買家前往 PayUni 頁面（MQ 事件驅動 TRUE/FALSE） */
    private Boolean needRefundPage;

    /** 任一商品項目已評論即為 TRUE（由 review.created MQ 事件更新） */
    private Boolean isReviewed;

    @TableField(fill = FieldFill.INSERT)
    private LocalDateTime createdAt;

    @TableField(fill = FieldFill.INSERT_UPDATE)
    private LocalDateTime updatedAt;
}
