package io.github.will413028.hawkr.payment.logistics.repository.entity;

import com.baomidou.mybatisplus.annotation.IdType;
import com.baomidou.mybatisplus.annotation.TableId;
import com.baomidou.mybatisplus.annotation.TableName;
import java.time.LocalDateTime;
import lombok.Data;

/**
 * {@code provider_settlement_lines}（V9）—— provider 帳單的明細列。
 *
 * <p>🔴 本表<b>沒有 {@code created_at}／{@code updated_at}</b>：時間軸是 {@code imported_at}
 * （V9 的 DDL 就是如此）。逐欄對 DDL 才看得出來，照抄兄弟表會多出兩個不存在的欄位。
 *
 * <p>🔴 {@code reconStatus = MATCHED} ⇔ {@code ledgerTxnNo IS NOT NULL}
 * （{@code chk_provsettle_ledger} 是<b>雙向</b>等式）：單向會讓「已比對但指不出分錄」溜過去。
 *
 * <p>⚠️ {@code importedBy} 是 ADM actor 的 <b>account_id</b>，不是 store_id。
 */
@Data
@TableName(value = "provider_settlement_lines", autoResultMap = true)
public class ProviderSettlementLineEntity {

    /** TSID。 */
    @TableId(type = IdType.INPUT)
    private Long id;

    /** ShippingMethod 碼值 {@code '3003'}／{@code '3004'}（同 ledger {@code owner_ref_id} 口徑）。 */
    private String provider;

    private String statementRef;

    /** {@code 'YYYY-MM'}。 */
    private String statementPeriod;

    /** 彙總列可為 NULL。 */
    private String providerShipmentRef;

    private String orderNo;

    /** CodeMeta {@code ProviderFeeType}（5 值）。 */
    private String feeType;

    /** 🔴 元，恆為正（{@code chk_provsettle_amount}）。 */
    private Long amount;

    /** {@code UNMATCHED} / {@code MATCHED} / {@code DISPUTED} / {@code WRITTEN_OFF}。 */
    private String reconStatus;

    /** 🔴 與 {@code reconStatus = MATCHED} 是雙向等式。 */
    private String ledgerTxnNo;

    private Long exceptionCaseId;

    /** ⚠️ ADM actor 的 account_id。 */
    private Long importedBy;

    private LocalDateTime importedAt;
}
