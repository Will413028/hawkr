package io.github.will413028.hawkr.order.entity;

import com.baomidou.mybatisplus.annotation.FieldFill;
import com.baomidou.mybatisplus.annotation.IdType;
import com.baomidou.mybatisplus.annotation.TableField;
import com.baomidou.mybatisplus.annotation.TableId;
import com.baomidou.mybatisplus.annotation.TableName;
import lombok.Data;

import java.time.LocalDateTime;

/**
 * 訂單發票開立選擇快照（58 號《電子發票服務》§5.4.3，Flyway V12）。
 *
 * <p><b>不可變</b>：由 {@code CheckoutConfirm} 在建單交易內寫入一次（{@code snapshot_version = 1}），
 * 之後 order-service 沒有、也不得新增任何 UPDATE 路徑——invoice-service 的
 * {@code operation_key} 含 {@code snapshot_version}，快照變動等於同一張訂單可以開出第二張發票
 * （58 號 I-50）。
 *
 * <p><b>零 PII</b>：載具明碼（手機條碼 3J0002／自然人憑證 CQ0001）一律不進本表，
 * 密文留在 invoice-service 的 {@code buyer_invoice_preferences}；本表只帶
 * {@link #buyerInvoicePreferenceId} 這個指標。統編與捐贈碼是公開登記／公告代碼，非 PII。
 */
@Data
@TableName("order_invoice_snapshots")
public class OrderInvoiceSnapshotEntity {

    @TableId(type = IdType.INPUT)
    private Long id;

    private String orderNo;

    /** 恆為 1（本服務不遞增；值域下界由 {@code chk_ois_version} 守）。 */
    private Integer snapshotVersion;

    /** {@code InvoiceDocumentKind} 的 value，本表只用 B2C／B2B（{@code chk_ois_kind}）。 */
    private String invoiceKind;

    /** {@code InvoiceCarrierSelectionMode} 的 value（{@code chk_ois_mode}）。 */
    private String carrierSelectionMode;

    /** invoice-service {@code buyer_invoice_preferences.id}；跨服務不建 FK。 */
    private Long buyerInvoicePreferenceId;

    /** 公司統編（8 碼數字）；僅 B2B 有值。 */
    private String buyerTaxId;

    /** 買方抬頭；僅 B2B 有值（AMEGO BuyerName 上限 60 字元）。 */
    private String buyerTitle;

    /** 捐贈碼（3–7 碼數字）。 */
    private String donationCode;

    /**
     * 發票開立通知信箱。§5.4.3 原 DDL 漏列，但 §6.2.4 order-service ⑤ 與 §7.1.2 步驟 1
     * 都要求採集，且 proto {@code OrderInvoiceSnapshotData.notify_email} 已定版
     * ⇒ 缺了它 {@code invoice_documents.recipient_email} 恆為空、開票通知寄不出去而不報錯。
     */
    private String notifyEmail;

    /**
     * B06／R03 更正的冪等鍵＝invoice-service 的 {@code correction_id}（58 號 §6.2.5 定版 3）。
     *
     * <p>CheckoutConfirm 寫的 v1 恆為 {@code null}——它的冪等由結帳流程自己的機制保證，
     * 而 {@code uk_ois_revision_idem} 是 partial unique index（{@code WHERE ... IS NOT NULL}），
     * NULL 不參與唯一性。
     */
    private String revisionIdempotencyKey;

    @TableField(fill = FieldFill.INSERT)
    private LocalDateTime createdAt;
}
