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
 * 賣家出貨門市 ↔ Uber external_store 對應（66 §5.1.3）。
 *
 * <p>三問：寫入＝賣家設定端點／ADM（A-U4）／建單 {@code customer_blocked} 403 的 N6 事後反推｜
 * 讀取＝Quote 與 Create <b>兩處</b>｜
 * 推走＝{@code active=false}（賣家關門市）；{@code blocked} 由 ADM 端點<b>或</b>下一次成功建單自動解除
 * ——🔴 <b>兩個出口都要有</b>，只留 ADM 的話賣家會卡到有人去按。
 *
 * <p>不變式 INV-U6（DB CHECK {@code chk_uber_store_mapping_active_not_blocked}）：
 * {@code active=true ⇒ blocked=false}。違反的症狀是「對已封鎖門市持續建單，每筆 403」。
 */
@Data
@TableName(value = "uber_store_mapping", autoResultMap = true)
public class UberStoreMappingEntity {

    @TableId(type = IdType.INPUT)
    private Long id;

    /** 🔴 賣家業務鍵 {@code store_id}（sellers PK），不是 account_id（66 §2.4）。 */
    private Long sellerStoreId;

    /**
     * 🔴 Quote 與 Create <b>兩處都要帶</b>（66 §5.1.3）。
     * <p>⚠️ 它是<b>憑證等級</b>的值（66 §8.6：洩漏 ＋ token 洩漏 ＝ 可代賣家叫車）⇒ log 必須遮罩。
     */
    private String externalStoreId;

    /** Uber customer id（{@code cus_} 前綴）。 */
    private String customerId;

    /**
     * 取件地址／聯絡人／電話／備註的<b>快照</b>（JSON 字串，key 一律 camelCase）。
     *
     * <p>🔴 <b>快照不是參照</b>：賣家改地址後已建立的 delivery 不得跟著變，
     * 否則對帳時「這一趟從哪裡取件」會有兩個答案。
     * <p>⚠️ 但 quote 用的是<b>當下值</b> ⇒ {@code payload_hash} 必須含 {@code external_store_id}（66 A9）。
     */
    @TableField(typeHandler = JsonbStringTypeHandler.class)
    private String pickupSnapshot;

    /**
     * 此門市支援的溫層（{@code DeliveryTemp} value 的 JSON 陣列字串）。
     * <p>🔴 預設只有常溫 {@code ["3201"]}——Uber API <b>完全沒有溫層欄位</b>（U-M），檢查 100% 在我方。
     */
    @TableField(typeHandler = JsonbStringTypeHandler.class)
    private String supportedTemps;

    /** 件數上限（M-4：30 件硬擋，無 warning 層）。 */
    private Integer maxPackages;

    /** 備貨時間，進 quote 的 {@code pickup_ready_dt}（預設 15 分）。 */
    private Integer prepSlaMinutes;

    private Boolean active;

    /**
     * Uber {@code customer_blocked} 403 的事後反推結果（66 N6／§4.5.4）。
     * <p>⚠️ 它沒有主動查詢的來源，只能由一次失敗的建單反推 ⇒ 解除同樣要有自動出口，不能只靠人。
     */
    private Boolean blocked;

    /** 同 {@link #blocked}，由 {@code customer_blocked} 403 反推（賣家需完成稅務資料）。 */
    private Boolean taxFormRequired;

    /** {@code HEALTHY}｜{@code DEGRADED}｜{@code BLOCKED}｜{@code UNKNOWN}（DB CHECK 守）。 */
    private String healthStatus;

    @TableField(fill = FieldFill.INSERT)
    private LocalDateTime createdAt;

    @TableField(fill = FieldFill.INSERT_UPDATE)
    private LocalDateTime updatedAt;
}
