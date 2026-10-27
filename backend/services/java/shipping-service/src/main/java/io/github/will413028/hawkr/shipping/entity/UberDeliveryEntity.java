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
 * Uber 配送單（66 §5.1.5）。
 *
 * <p>三問：寫入＝建單成功（🔴 <b>{@code ON CONFLICT DO NOTHING}，不是 catch
 * {@code DuplicateKeyException}</b>——PostgreSQL 交易內任一句失敗即整個交易 aborted，
 * catch 得到例外但下一句 SELECT 會炸成 500）／webhook consumer／P1 未終態回查排程｜
 * 讀取＝追蹤端點、狀態映射、POD 抓取、月結對帳｜
 * 推走＝webhook 與 P1 把 {@link #providerStatus} 推到終態（{@code delivered}／{@code canceled}／
 * {@code returned}）並寫 {@link #outcome}，🔴 <b>列不刪除</b>。
 *
 * <p>不變式：INV-U2（{@code quote_id} UNIQUE，違反＝兩張單重複計費）、
 * INV-U3（{@code shipment_id} UNIQUE，違反＝狀態寫不回去）。
 */
@Data
@TableName(value = "uber_deliveries", autoResultMap = true)
public class UberDeliveryEntity {

    @TableId(type = IdType.INPUT)
    private Long id;

    /**
     * Uber 配送單 id（{@code del_} 正向／{@code ret_} 逆向）。
     * <p>🔴 <b>前綴只用於讀取時的健全性檢查，不用於分辨 leg</b>（66 §4.5.3／陷阱 A7）——
     * 前綴是 provider 的實作細節，它改了我們不會知道。
     * 讀到「{@code ret_} 前綴但 {@code isReturn=false}」⇒ <b>建 case，不猜</b>。
     */
    private String deliveryId;

    private Long shipmentId;

    /** 對應 {@code uber_quotes.quote_id}；UNIQUE（INV-U2）。 */
    private String quoteId;

    /**
     * 🔴 <b>leg 的真相在這裡，不在前綴</b>（66 §5.1.5）。
     * <p>逆向 leg 是<b>另一筆 shipment ＋ 另一筆本表列</b>，不是在同一筆上加狀態。
     */
    private Boolean isReturn;

    /**
     * 正向 ↔ 逆向 leg 的關聯陣列（JSON 字串，66 N2／§7.3.3）。
     * <p>🔴 {@code relationship} 未知值 fail-closed：不建關聯 ＋ 建 case，<b>不做前綴比對、不猜</b>。
     */
    @TableField(typeHandler = JsonbStringTypeHandler.class)
    private String relatedDeliveries;

    /**
     * Uber 原生狀態值照抄（7 值，U-I）。正規化值在 {@code shipments.status}，兩者分開儲存。
     * <p>🔴 未知值一律 fail-closed（不寫入 ＋ counter ＋ 建 case），不得猜測式映射（INV-U7）。
     * <p>⚠️ 容忍值 {@code shopping_completed}（Pick &amp; Pack 專屬）<b>不寫入本欄</b>，只計 counter。
     */
    private String providerStatus;

    /**
     * 🔴 <b>boolean 欄位不是狀態</b>（U-I／66 §7.3.1）：它不映射任何 normalized 值。
     * 把它當第 8 個狀態會讓狀態機多出一條不存在的邊。
     */
    private Boolean courierImminent;

    private String courierName;

    /**
     * 🔴 存<b>遮罩後</b>的顯示值。Uber 給的是可撥打的臨時號碼 ⇒ 原值只透傳給前端，
     * <b>不落庫不落 log</b>（66 §5.1.5／§8.6）。
     */
    private String courierPhoneMasked;

    /** 外送員位置快照（JSON 字串）。來源是 {@code courier_update}（每 20 秒一發，U-E）。 */
    @TableField(typeHandler = JsonbStringTypeHandler.class)
    private String courierLocation;

    private LocalDateTime etaDropoffAt;

    /** Uber 原生值照抄（66 C.4 不納管）。是 {@code 33 ABNORMAL} 留痕的內容（66 §7.3.4）。 */
    private String undeliverableReason;

    /**
     * Uber 原生值照抄。
     * <p>🔴 取消<b>可能產生費用而該費用只出現在帳單</b>（U-R／66 §4.5.5）⇒ 本欄與
     * {@code shipments.uber_delivery_id} 是月結對帳的鍵；
     * <b>不得為取消費用另造 {@code shipments} 欄位</b>（沒有來源的欄會永遠是 null）。
     */
    private String cancelationReason;

    /**
     * {@code UberDeliveryOutcome}（66 C.2 ④）：{@code PROVIDER_UNDELIVERABLE}｜{@code PROVIDER_CANCELED}｜
     * {@code SELLER_CANCELED}｜{@code ADMIN_CANCELED}｜{@code CREATE_FAILED}（DB CHECK 守）。
     *
     * <p>🔴 它是「41 這一格的錢與責任歸誰」的唯一判準——{@code 41} 也可能來自賣家主動取消，
     * 兩者的錢與責任完全不同（66 §7.3.4）。
     */
    private String outcome;

    @TableField(fill = FieldFill.INSERT)
    private LocalDateTime createdAt;

    /** ⚠️ 本欄的最舊齡是 runbook 的 stale ETA 判準（66 §12.4）：{@code courier_update} 沒進來時它會停住。 */
    @TableField(fill = FieldFill.INSERT_UPDATE)
    private LocalDateTime updatedAt;
}
