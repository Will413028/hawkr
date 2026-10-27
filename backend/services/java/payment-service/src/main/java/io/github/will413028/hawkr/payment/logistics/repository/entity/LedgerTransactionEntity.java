package io.github.will413028.hawkr.payment.logistics.repository.entity;

import com.baomidou.mybatisplus.annotation.IdType;
import com.baomidou.mybatisplus.annotation.TableId;
import com.baomidou.mybatisplus.annotation.TableName;
import java.time.LocalDateTime;
import lombok.Data;

/**
 * {@code ledger_transactions}（V9）—— 雙分錄帳本的表頭。
 *
 * <p>🔴 本表 <b>append-only</b>：零 UPDATE、零 DELETE，所以 V9 <b>沒有 {@code updated_at}</b>。
 * 更正一律開一筆反向的 {@code MANUAL_ADJUSTMENT}，不改原列。
 * ⇒ 本 entity 刻意<b>沒有</b> updatedAt 欄位；mapper 也不提供任何 UPDATE。
 *
 * <p>🔴 {@code idempotencyKey}（{@code '{txn_type}:{source_type}:{source_ref}'}）的 UNIQUE 是
 * 「同一個業務事件只認列一次」的唯一保證。衝突時回零列 ⇒ 呼叫端據此判定「已經認列過」，
 * 那<b>不是錯誤</b>（MQ 至少一次投遞讓重放是常態）。
 *
 * <p>⚠️ {@code commerceModel} × {@code txnType} 有表級互斥 CHECK（{@code chk_ledgtxn_model_type}）。
 * 程式側的同一份表在 {@code LedgerTxn.validateModelTxnTypeCompatibility}——<b>兩道防線方向一致，
 * 不互相取代</b>。
 */
@Data
@TableName(value = "ledger_transactions", autoResultMap = true)
public class LedgerTransactionEntity {

    /** TSID。 */
    @TableId(type = IdType.INPUT)
    private Long id;

    /** {@code 'LT' + yyyyMMdd + LPAD(nextval('ledger_txn_no_seq'),8,'0')}。 */
    private String txnNo;

    /** CodeMeta {@code LedgerTxnType}。 */
    private String txnType;

    /** store surrogate（非 account_id）。 */
    private Long storeId;

    private String orderNo;

    /** CodeMeta {@code CommerceModel}。 */
    private String commerceModel;

    /** {@code 'YYYYMM'}（UTC 曆月）。 */
    private String settlementPeriod;

    private String sourceType;

    /** INV-L3：每個減項都要指得回一張單據 ⇒ NOT NULL。 */
    private String sourceRef;

    private String idempotencyKey;

    /** 入帳的業務時間（≠ {@code createdAt}）。 */
    private LocalDateTime postedAt;

    private LocalDateTime createdAt;
}
