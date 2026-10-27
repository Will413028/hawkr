package io.github.will413028.hawkr.payment.logistics.repository.entity;

import com.baomidou.mybatisplus.annotation.IdType;
import com.baomidou.mybatisplus.annotation.TableId;
import com.baomidou.mybatisplus.annotation.TableName;
import java.time.LocalDateTime;
import lombok.Data;

/**
 * {@code logistics_cost_facts}（V16）—— provider 成本事實。
 *
 * <p>70 §3.1.4 durable join 的<b>成本側載體</b>：SD-15 逐字要求「成本已發生的事實
 * <b>先記錄</b>，歸屬未定」，而在 V16 之前那句話沒有任何載體。
 *
 * <p>🔴 <b>它不是 receivable 的替身，也不是第二套收款引擎</b>（§14.1／§3.5.1 的射程）：
 * append-only、<b>沒有狀態機、沒有收款嘗試、不參與任何收款流程</b>。
 * 應收的唯一建立路徑仍然只有一條——{@code cost incurred} <b>AND</b> {@code counterparty = SELLER}。
 *
 * <p>⚠️ <b>本表沒有 {@code updated_at}</b>：它是事實，不是狀態。事實不會被改寫——
 * 同一條 leg 之後出現的費用走 true-up（§3.1.3），改的是 receivable 的 {@code actual_cost}，
 * 不是回頭改這一列。
 */
@Data
@TableName(value = "logistics_cost_facts", autoResultMap = true)
public class LogisticsCostFactEntity {

    /** TSID。 */
    @TableId(type = IdType.INPUT)
    private Long id;

    /** {@code ShippingMethod} 碼值（{@code '3003'}…）。 */
    private String provider;

    /**
     * 🔴 identity 的錨（§3.1.2）。{@code uq_costfact_delivery_ref} 保證一條 leg 一筆事實，
     * 與 {@code logistics_receivables.uq_logrecv_delivery_ref} <b>同口徑</b>（單欄、不含 provider）
     * ——兩張表對「什麼是同一條 leg」必須是同一個答案，否則 join 會對到錯的列。
     */
    private String providerDeliveryRef;

    private Long storeId;

    private String orderNo;

    /**
     * ⚠️ <b>可為 {@code null}</b>：取消可能發生在 shipment 建立之前，這個事實真的可能沒有
     * shipment（70 §7.1）。🔴 正因為它可以是 null，<b>它不能當 identity</b>。
     */
    private Long shipmentId;

    /** 🔴 新台幣元整數，恆為正（{@code chk_costfact_amount}）。0 ＝「沒有成本」，那種情形不該發事件。 */
    private Long estimatedCost;

    /**
     * 🔴 事實發生的<b>業務時間</b>，不得用收到事件的時間冒充——兩者在重投時會差很多，
     * 而它是 {@code charge_basis_frozen_at} 的來源（計費依據凍結在事實發生的那一刻）。
     */
    private LocalDateTime occurredAt;

    /** L1 去重：broker 重投同一則事件時認得出來（{@code uq_costfact_event}）。 */
    private String eventId;

    private LocalDateTime createdAt;
}
