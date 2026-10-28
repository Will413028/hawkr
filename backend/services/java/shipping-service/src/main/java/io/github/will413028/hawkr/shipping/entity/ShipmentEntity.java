package io.github.will413028.hawkr.shipping.entity;

import com.baomidou.mybatisplus.annotation.FieldFill;
import com.baomidou.mybatisplus.annotation.IdType;
import com.baomidou.mybatisplus.annotation.TableField;
import com.baomidou.mybatisplus.annotation.TableId;
import com.baomidou.mybatisplus.annotation.TableName;
import io.github.will413028.hawkr.shipping.config.typehandler.ShipmentPhotoListTypeHandler;
import lombok.Data;

import java.io.Serializable;
import java.time.LocalDate;
import java.time.LocalDateTime;
import java.util.List;

@Data
@TableName(value = "shipments", autoResultMap = true)
public class ShipmentEntity {

    @TableId(type = IdType.INPUT)
    private Long id;

    private String shipmentNo;
    private String orderNo;
    private Long storeId;
    private Long buyerId;
    private String shippingMethod;
    private String merId;
    private String shipTradeNo;
    private String uberDeliveryId;
    private String status;
    /**
     * 本筆<b>進入當前 {@code status} 的時間</b>；aging 指標的唯一錨（65）。
     * <p>🔴 禁用 {@code updated_at} 當 aging 基準——任何無關欄位的 UPDATE（標籤重印、
     * 租約續期）都會重置它，於是「卡在某狀態多久」會被一次次洗掉。
     * <p>DB 為 {@code NOT NULL DEFAULT NOW()}：INSERT 留 null 由 DB 補；
     * 任何改 {@code status} 的 SQL 必須<b>同句</b>推進它（見 {@code ShipmentMapper} 三支狀態寫入）。
     */
    private LocalDateTime statusEnteredAt;
    /**
     * 物流商回傳的<b>原始貨態碼</b>（正常軌跡證據，含本系統尚未 mapping 的未知值）。
     *
     * <p>🔴 不得以「有值」判定異常。異常的 provider 側判準是 {@link #providerErrorCode} 非 null
     * （V12 拆欄前兩者混寫在同一欄，使 ADM 異常清單把正常單一併撈出）。
     */
    private String providerStatus;
    private String providerMessage;
    private LocalDateTime providerStatusTime;

    /**
     * 物流商回傳的<b>失敗代碼</b>（建單/取號/叫件失敗）。非 null 即為 provider 層失敗。
     *
     * <p>唯一寫入者：{@code ShipmentMapper.markLabelPrintFailed} /
     * {@code markTcatCallFailed} / 建單失敗殼（{@code ShippingService.persistFailedShipmentShell}）。
     * 貨態寫入路徑<b>不得</b>碰這三欄。
     */
    private String providerErrorCode;
    private String providerErrorMessage;
    private LocalDateTime providerErrorAt;
    private String shipType;
    private String lgsType;
    private String goodsType;
    private String serviceType;
    private Long tradeAmt;
    private String senderName;
    private String senderPhone;
    private String senderCity;
    private String senderDistrict;
    private String senderAddress;
    private String senderZipCode;
    private String recipientName;
    private String recipientPhone;
    private String recipientAddress;
    private String cvsStoreCode;
    private String storeName;
    private String partnerId;
    private String validationNo;
    private String spec;
    private String deliveryTimeTag;
    /** 列印狀態機 value（LabelPrintStatus，§2.4.5.1）：4101 NONE / 4102 REQUESTED / 4103 SUCCEEDED / 4104 FAILED */
    private String labelPrintStatus;
    /** 最近一次 transition 到 REQUESTED 的時間，reconciliation 卡死判定 */
    private LocalDateTime labelPrintRequestedAt;
    /** 黑貓叫車狀態機 value（TcatCallStatus，§2.4.5.2）：4201 NONE / 4202 REQUESTED / 4203 SUCCEEDED / 4204 FAILED */
    private String tcatCallStatus;
    /** 最近一次 transition 到 REQUESTED 的時間，ops 告警判定 */
    private LocalDateTime tcatCallRequestedAt;
    private LocalDateTime tcatCallTime;
    private String labelObjectKey;
    private Long labelAssetId;
    private String prodDesc;
    private String odno;
    /** 託運單 PDF 下載期限（取號時 NOW()+24h，僅前端按鈕判斷用） */
    private LocalDateTime labelExpireAt;
    /** 訂單已確認收貨（由 Order Service MQ 事件同步） */
    private Boolean receiptConfirmed;
    /** 是否需要事先電聯 Y/N（generate_home_label 時填入） */
    private String isContact;
    /** 是否需要推車 Y/N（generate_home_label 時填入） */
    private String isTrolley;
    /** 是否為退貨物流單 */
    private Boolean isReturn;
    /** 上次主動向 PayUni 查詢貨態的時間（成功與否皆寫入），用於 §5.19 reconcile 排程退避 */
    private LocalDateTime lastReconciledAt;
    /**
     * issue #2294 L3：§5.19.3.B 列印補收排程的<b>租約到期時間</b>（非「已補收旗標」）。
     * NULL 或 &lt;= NOW() 表示可搶佔；由 {@code claimLabelReconcile} 推進。
     */
    private LocalDateTime labelReconcileNextAt;
    /**
     * issue #2294 L4：§5.19.3.C 黑貓卡死告警的<b>冷卻租約到期時間</b>（非「已告警旗標」）。
     * 卡死狀態沒解除就該持續有聲音，所以租約到期後會再次告警；NULL 或 &lt;= NOW() 表示可搶佔。
     */
    private LocalDateTime tcatStuckAlertedAt;
    /**
     * 65 Foundation：建單結果回查（991 CREATE_UNKNOWN）的<b>租約到期時間</b>。
     * <p>🔴 <b>不是「已查過旗標」</b>：NULL 或 &lt;= NOW() 表示可搶佔。pod 在副作用中途崩潰時，
     * 租約到期會讓該筆自動回到候選集合；永久旗標則會讓它卡死且無告警。
     */
    private LocalDateTime createQueryNextAt;
    /**
     * 65 Foundation：取消確認輪詢（992 CANCEL_PENDING）的<b>租約到期時間</b>。
     * <p>🔴 <b>不是「已確認旗標」</b>：NULL 或 &lt;= NOW() 表示可搶佔。語意與約束同
     * {@link #createQueryNextAt}。
     */
    private LocalDateTime cancelConfirmNextAt;
    /**
     * 66 P1：Uber 未終態回查（tick 5 分／租約 5 分）的<b>租約到期時間</b>。
     * <p>🔴 <b>不是「已回查旗標」</b>：NULL 或 &lt;= NOW() 表示可搶佔。語意與約束同
     * {@link #createQueryNextAt}；V13 刻意不回填既有列（回填成 NOW() 會讓它們沉默一個租約週期）。
     * <p>⚠️ P1 不是 reconciliation job：webhook 只重試 3 次（窗口約 100 秒），
     * 回查是我方唯一的收斂手段，屬業務流程的一部分。
     */
    private LocalDateTime uberStatusReconcileNextAt;
    /**
     * 取消申請受理時間（992 CANCEL_PENDING 的事實欄）。
     * <p>992 的語意是「取消申請已受理，待 provider 確認」，它是唯一允許回退到非終態的狀態。
     */
    private LocalDateTime cancelRequestedAt;
    /**
     * 取消申請發起者：{@code BUYER} / {@code SELLER} / {@code ADMIN} / {@code SYSTEM}。
     * <p>值域由 DB CHECK {@code chk_shipments_cancel_actor} 守；NULL = 未申請取消。
     */
    private String cancelRequestActor;
    /** 賣家處理期限（UTC），逾期由 §5.9 排程自動取消。NULL = 無期限（Uber）/ 已通知 */
    private LocalDateTime processingDeadline;
    /** 給宅配人員備註（generate_home_label 時填入，最長 100 位） */
    private String memo;

    /**
     * §5.21 DELIVERY_OVERDUE 異常偵測來源。
     * 黑貓宅配：建單時 = processing_deadline + 3 工作日。
     * 超商：Webhook ShipStatus=22 已到店時 = NOW() + 7 天（應取貨期限）。
     * 歷史 shipment 為 NULL，不參與偵測（§5.21.8）。
     */
    @TableField("expected_delivery_date")
    private LocalDate expectedDeliveryDate;

    @com.baomidou.mybatisplus.annotation.TableField(typeHandler = io.github.will413028.hawkr.shipping.config.typehandler.JsonbStringTypeHandler.class)
    private String senderSnapshot;

    @com.baomidou.mybatisplus.annotation.TableField(typeHandler = io.github.will413028.hawkr.shipping.config.typehandler.JsonbStringTypeHandler.class)
    private String receiverSnapshot;

    @TableField(typeHandler = ShipmentPhotoListTypeHandler.class)
    private List<ShipmentPhoto> shipmentPhotos;

    @TableField(fill = FieldFill.INSERT)
    private LocalDateTime createdAt;

    @TableField(fill = FieldFill.INSERT_UPDATE)
    private LocalDateTime updatedAt;

    /**
     * 出貨照片（JSONB 內部結構）
     */
    @Data
    public static class ShipmentPhoto implements Serializable {
        private String assetId;
        private String objectKey;
    }
}
