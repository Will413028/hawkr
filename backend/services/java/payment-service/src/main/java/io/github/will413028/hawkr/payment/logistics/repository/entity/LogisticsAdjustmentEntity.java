package io.github.will413028.hawkr.payment.logistics.repository.entity;

import com.baomidou.mybatisplus.annotation.IdType;
import com.baomidou.mybatisplus.annotation.TableId;
import com.baomidou.mybatisplus.annotation.TableName;
import java.time.LocalDateTime;
import lombok.Data;

/**
 * {@code logistics_adjustments}（V9）—— 責任／對帳調整單。
 *
 * <p>🔴 {@code receivableId} <b>可為 NULL</b>：未送達的情形不建 receivable（ADR-003 應收只在
 * 送達完成成立），但 provider 的取消費仍然發生 ⇒ 掛在 {@code storeId} ＋ {@code providerDeliveryRef}
 * 上，無母單。⚠️「未送達不收」講的是<b>價金</b>，不是「取消費沒人付」。
 *
 * <p>⚠️ {@code amount} <b>允許負數</b>（與 {@code ledger_entries.amount} 恆正相反）：這裡是業務
 * 調整單，正負是它的語意；帳本那邊由 {@code direction} 表達。<b>兩層的規則不同是刻意的。</b>
 * 唯一被擋掉的是 0（{@code chk_logadj_amount_nonzero}）。
 *
 * <p>🔴 本表<b>沒有 {@code updated_at}</b>（V9 的 DDL 就是如此）。
 */
@Data
@TableName(value = "logistics_adjustments", autoResultMap = true)
public class LogisticsAdjustmentEntity {

    /** TSID。 */
    @TableId(type = IdType.INPUT)
    private Long id;

    /** 🔴 可為 NULL（無母單的取消費）。 */
    private Long receivableId;

    private Long storeId;
    private String providerDeliveryRef;

    /** {@code RESPONSIBILITY} / {@code RECONCILIATION}。 */
    private String kind;

    /** ⚠️ 允許負數，禁止 0。 */
    private Long amount;

    /** {@code ResponsibilityCause} 的 {@code value()}；{@code RESPONSIBILITY} 時必填。 */
    private String responsibilityCause;

    private Long statementLineId;

    /** ADM account_id；系統產生時為 NULL。 */
    private Long createdBy;

    private String reason;
    private LocalDateTime createdAt;
}
