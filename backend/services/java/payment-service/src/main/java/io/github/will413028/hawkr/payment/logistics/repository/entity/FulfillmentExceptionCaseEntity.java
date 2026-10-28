package io.github.will413028.hawkr.payment.logistics.repository.entity;

import com.baomidou.mybatisplus.annotation.IdType;
import com.baomidou.mybatisplus.annotation.TableField;
import com.baomidou.mybatisplus.annotation.TableId;
import com.baomidou.mybatisplus.annotation.TableName;
import io.github.will413028.hawkr.payment.infrastructure.persistence.typehandler.JsonbStringTypeHandler;
import java.time.LocalDateTime;
import lombok.Data;

/**
 * {@code fulfillment_exception_cases}（V9）—— 通用維運例外案件。
 *
 * <p>⚠️ 它<b>完全不綁 commerce_model</b>：{@code source_service} 三值、{@code subject_type} 四值，
 * 與商流無關。它是 Uber refund_request 的 compensation case 落點。
 *
 * <p>🔴 兩個 JSONB 欄位掛 {@link JsonbStringTypeHandler} ⇒ 本類別<b>必須</b>
 * {@code autoResultMap = true}；任何自訂 {@code @Select} 另需
 * {@code @ResultMap("mybatis-plus_FulfillmentExceptionCaseEntity")}（10 檔 §11.1）。
 *
 * <p>🔴 {@code category} 在 DB 層<b>刻意沒有 CHECK</b>：該 enum 本波尚未落進 CodeMeta，
 * 在 DB 硬寫一份值域清單 ＝ 在 SSOT 之外造第二份碼表。值域由寫入路徑的
 * {@code CodeMetaValidator} 守。
 *
 * <p>🔴 {@code ownerAccountId} 是「有沒有人負責」的機械保證（{@code chk_exc_owner}）：
 * OPEN ⇒ 必須為 NULL；離開 OPEN ⇒ 必須有值且有 {@code ownerAssignedAt}。
 * <b>它不是排他鎖</b>——兩個人同時開同一張案件是可接受的（由 {@code uq_exc_open_dedup} 去重），
 * 兩個人同時「處理中」而沒人負責才不可接受。
 */
@Data
@TableName(value = "fulfillment_exception_cases", autoResultMap = true)
public class FulfillmentExceptionCaseEntity {

    /** TSID。 */
    @TableId(type = IdType.INPUT)
    private Long id;

    /** {@code 'EX' + yyyyMMdd + LPAD(nextval('exception_case_no_seq'),8,'0')}。 */
    private String caseNo;

    /** {@code shipping} / {@code order} / {@code payment}（2026-09-12 合併時收斂）。 */
    private String sourceService;

    private String category;

    /** {@code P0} / {@code P1} / {@code P2}。 */
    private String severity;

    /** {@code ORDER} / {@code SHIPMENT} / {@code SHIPMENT_BATCH} / {@code CONFIG}。 */
    private String subjectType;

    private String subjectRef;
    private Long storeId;

    /** {@code OPEN} / {@code IN_PROGRESS} / {@code RESOLVED} / {@code DISMISSED}。 */
    private String status;

    private LocalDateTime statusEnteredAt;

    /** 🔴 OPEN ⇒ NULL；非 OPEN ⇒ 必有值（{@code chk_exc_owner}）。 */
    private Long ownerAccountId;

    private LocalDateTime ownerAssignedAt;

    /** {@code '{category}:{subject_type}:{subject_ref}'}；partial UNIQUE 只覆蓋未結案的兩個狀態。 */
    private String dedupKey;

    /** JSONB object。 */
    @TableField(typeHandler = JsonbStringTypeHandler.class)
    private String detail;

    /** JSONB array；🔴 只存 CodeMeta 碼，不存端點或 URL（URL 會腐化，碼不會）。 */
    @TableField(typeHandler = JsonbStringTypeHandler.class)
    private String proposedActionCodes;

    private String resolutionCode;
    private String resolutionNote;
    private Long resolvedByAccountId;
    private LocalDateTime resolvedAt;

    /**
     * 🔴 <b>Immutable event context</b>（V14）——建案當下凍結，之後任何路徑都不得改。
     *
     * <p>N17（建案）與 N16（定案）的 {@code order_no} <b>都從這一欄取值</b>。
     * 在此之前兩者由呼叫端各傳一次、中間隔著數小時～數天與一次人工覆核，
     * 而 server 端沒有任何一行比對 ⇒ 兩筆事件可以帶不同的 order_no。
     * 那不是小事：{@code order_no} 是 order-service 把訂單從 {@code SHIPPING_ABNORMAL(35)}
     * 推到 {@code DELIVERY_FAILED_NO_REFUND(54)} 的 <b>CAS 錨</b>（36 §3.3.1）
     * ⇒ 對不上時那道 CAS 永遠 {@code rows=0}，Order 永遠停在 35，<b>全鏈路無訊號</b>。
     *
     * <p>🔴 責任判定案件必填（{@code chk_exc_responsibility_context}）；generic 案件可為 null。
     */
    private String orderNo;

    /**
     * 🔴 Immutable event context（V14）。<b>責任軸也允許 null</b>：
     * 責任判定可能在 shipment 建立之前就產生（建單失敗 {@code CREATE_FAILED(990)}
     * 本身就是一種履約失敗）⇒ 強制非空會讓最需要判責任的那一類案件建不出來。
     * <p>⚠️ 內部原生 {@code Long}（TSID），只在組 payload 的單一邊界轉字串。
     */
    private Long shipmentId;

    /**
     * 🔴 Immutable event context（V14）。ShippingMethod 碼值（{@code '3003'}／{@code '3004'}），
     * 與 {@code logistics_receivables.provider} <b>同一個碼域</b>、不同欄名——
     * 本欄存在的唯一理由是餵 N16／N17 的 {@code shipping_method}（36 §3.3.1 的 wire key）。
     * <p>🔴 責任判定案件必填（{@code chk_exc_responsibility_context}）。
     */
    private String shippingMethod;

    /** 🔴 來源端發生時間（事件帶進來），<b>不可與 {@code createdAt} 混用</b>。 */
    private LocalDateTime occurredAt;

    private LocalDateTime createdAt;
    private LocalDateTime updatedAt;
}
