package io.github.will413028.hawkr.payment.logistics.repository.entity;

import com.baomidou.mybatisplus.annotation.IdType;
import com.baomidou.mybatisplus.annotation.TableId;
import com.baomidou.mybatisplus.annotation.TableName;
import java.time.LocalDateTime;
import lombok.Data;

/**
 * {@code logistics_receivables}（V9）—— 賣家物流應收的持久化列。
 *
 * <p>🔴 <b>欄位集合逐欄對應 V9 的 DDL，不對應 Go 那份 struct</b>：兩者曾有落差，而
 * DDL 才是契約。守門是 {@code LogisticsSchemaContractTest}——本波零程式碼消費者，
 * 欄名打錯不會有任何東西編譯失敗。
 *
 * <p>🔴 幾個最容易讀錯的欄位，把語意釘在欄位旁邊而不是只寫在設計書裡：
 * <ul>
 *   <li>{@code actualCost} <b>NULL ＝ 尚未對帳，不是 0</b>。與 {@code statementLineId} 是
 *       雙向等式（{@code chk_logrecv_recon_pair}）：兩者必須同時寫。</li>
 *   <li>{@code carriedResidual} <b>不是「上期的手續費」</b>，是「上期預估與實際的差」，
 *       費率估準時為 0。讀成手續費就會做出 fee-on-fee 遞迴。</li>
 *   <li>{@code submitLeaseUntil} 是提交 worker 的<b>租約</b>，<b>不是「已送出」旗標</b>
 *       （10 檔 §29）。它會過期並自動讓該列回到候選集合。</li>
 *   <li>{@code storeId} 是賣家業務鍵，<b>不是 account_id</b>。</li>
 * </ul>
 */
@Data
@TableName(value = "logistics_receivables", autoResultMap = true)
public class LogisticsReceivableEntity {

    /** TSID（V9 是 {@code bigint}，非 bigserial）⇒ 由呼叫端帶入。 */
    @TableId(type = IdType.INPUT)
    private Long id;

    private Long storeId;
    private String orderNo;
    private Long shipmentId;
    private String provider;
    private String providerDeliveryRef;
    private Long estimatedCost;

    /** 🔴 NULL ＝ 尚未對帳（不是 0）。 */
    private Long actualCost;

    private Long platformSubsidy;
    private Long targetNet;

    /** 🔴 可為負：本筆 target_net 內含的前期收款殘差。 */
    private Long carriedResidual;

    /** {@code LogisticsReceivableStatus} 的 {@code value()}。 */
    private String status;

    private LocalDateTime chargeBasisFrozenAt;
    private String idempotencyKey;
    private Long invoiceDocumentId;

    /** 🔴 租約欄，不是旗標。 */
    private LocalDateTime submitLeaseUntil;

    private Long statementLineId;

    /**
     * 建立本列<b>當下</b>的 resolved responsibility audit snapshot（V15）。
     *
     * <p>🔴 <b>它不是 fulfillment responsibility 的 SSOT</b>（70 §3.6.2）：
     * {@code PROVIDER_CAUSED}／{@code PLATFORM_CAUSED} <b>可以沒有</b>賣家應收，
     * 責任未判定時<b>明確沒有</b> ⇒ 應收不能成為 responsibility classification 存在的前提。
     * 未建立應收時的權威落點是 {@code fulfillment_exception_cases} ＋
     * {@code fulfillment_case_resolutions}。
     *
     * <p>🔴 值域<b>只有四個 resolved 值</b>（{@code chk_logrecv_responsibility_cause}），
     * 而 classification 本身是 5 值——那是<b>分工不是矛盾</b>：應收只在 resolved 之後才建立。
     *
     * <p>⚠️ 它的職責是 audit evidence／receivable origin／ADM 對帳／（未來）KPI 輸入；
     * 🔴 <b>不是</b> financial counterparty、<b>不是</b> payment collector、
     * <b>不是</b> {@code Bearer}。
     *
     * <p>⚠️ {@code null} ＝ 尚未寫入快照，<b>不是</b>「沒有責任」。
     */
    private String responsibilityCause;

    private LocalDateTime createdAt;
    private LocalDateTime updatedAt;
}
