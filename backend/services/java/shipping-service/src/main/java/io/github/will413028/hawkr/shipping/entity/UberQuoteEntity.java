package io.github.will413028.hawkr.shipping.entity;

import com.baomidou.mybatisplus.annotation.FieldFill;
import com.baomidou.mybatisplus.annotation.IdType;
import com.baomidou.mybatisplus.annotation.TableField;
import com.baomidou.mybatisplus.annotation.TableId;
import com.baomidou.mybatisplus.annotation.TableName;
import io.github.will413028.hawkr.shipping.config.typehandler.JsonbStringTypeHandler;
import lombok.Data;

import java.time.LocalDateTime;

/**
 * Uber 報價（66 §5.1.4）。
 *
 * <p>三問：寫入＝報價端點／checkout preview｜
 * 讀取＝checkout confirm（🔴 <b>INV-U1 的收斂點</b>）／建單／帳務與客訴追溯｜
 * 推走＝{@code CHECKOUT_ACCEPTED}（結帳確認）・{@code CONSUMED}（建單成功）・
 * {@code EXPIRED}（TTL 掃描）・{@code SUPERSEDED}（同 hash 重報）。
 *
 * <p>🔴 <b>{@code CONSUMED} 的 allowed-from 只有 {@code CHECKOUT_ACCEPTED}</b>——
 * from {@code CREATED} 代表跳過了 66 §4.1 ② 的結帳確認。
 *
 * <p>🔴 <b>INV-U1</b>：訂單的 Uber 運費恆等於被 {@code CHECKOUT_ACCEPTED} 的那筆的 {@link #feeAmount}，
 * <b>永不補收</b>。收斂點不是「不要重算」而是「沒有重算的路徑」——confirm 的程式碼裡
 * <b>不得存在</b>呼叫 {@code shippingFeeCache.lookupFee("3003", …)} 的分支。
 *
 * <p>⚠️ {@code EXPIRED} 只改 status <b>不刪列</b>（帳務與客訴要查得到報價歷史），保留 90 天。
 * <p>⚠️ 卡住偵測：{@code CHECKOUT_ACCEPTED} 超過 30 分未 {@code CONSUMED} ＝<b>收了錢沒建單</b>。
 */
@Data
@TableName(value = "uber_quotes", autoResultMap = true)
public class UberQuoteEntity {

    @TableId(type = IdType.INPUT)
    private Long id;

    /** Uber 回傳的報價 id（{@code dqt_} 前綴）。UNIQUE。 */
    private String quoteId;

    /** 建單後回填；報價當下為 null（66 §5.1.4）。 */
    private String orderNo;

    private Long buyerId;

    /** 🔴 賣家業務鍵 {@code store_id}，不是 account_id。 */
    private Long sellerStoreId;

    private String externalStoreId;

    /**
     * 報價運費，新台幣<b>元整數</b>（禁 BigDecimal／Double）。
     * <p>🔴 INV-U1 的那個值：永不補收。
     */
    private Long feeAmount;

    private String currency;

    private LocalDateTime expiresAt;

    /** 取件資訊快照（JSON 字串，key 一律 camelCase）。 */
    @TableField(typeHandler = JsonbStringTypeHandler.class)
    private String pickupJson;

    /** 收件資訊快照（JSON 字串，key 一律 camelCase）。 */
    @TableField(typeHandler = JsonbStringTypeHandler.class)
    private String dropoffJson;

    /** 品項快照（JSON 字串，key 一律 camelCase）。 */
    @TableField(typeHandler = JsonbStringTypeHandler.class)
    private String itemsJson;

    /**
     * {@code SHA256( external_store_id ‖ canonical(dropoff) ‖ canonical(items 依 sku_id 排序)
     * ‖ packing_policy_version )}——順序固定，缺一即 66 A9（換了取件門市但 hash 相同
     * ⇒ 用舊報價建新門市的單）。
     *
     * <p>🔴 <b>由伺服端算，前端不得傳</b>——前端傳 hash 等於讓前端決定「這是不是同一筆購物車」。
     */
    private String payloadHash;

    /**
     * 🔴 政策版本進 quote <b>快照</b>：改政策不回溯既有訂單，否則同一張訂單在不同時間算出不同運費，
     * 而運費已經收過了（66 §5.1.8）。指向 {@code packing_policy.version}。
     */
    private Integer packingPolicyVersion;

    /**
     * {@code CREATED}｜{@code CHECKOUT_ACCEPTED}｜{@code EXPIRED}｜{@code CONSUMED}｜{@code SUPERSEDED}
     * （DB CHECK {@code chk_uber_quotes_status} 守）。
     * <p>終態＝{@code CONSUMED}／{@code EXPIRED}／{@code SUPERSEDED}；
     * 非終態＝{@code CREATED}／{@code CHECKOUT_ACCEPTED}（66 §7.3.5）。
     */
    private String status;

    @TableField(fill = FieldFill.INSERT)
    private LocalDateTime createdAt;

    @TableField(fill = FieldFill.INSERT_UPDATE)
    private LocalDateTime updatedAt;
}
