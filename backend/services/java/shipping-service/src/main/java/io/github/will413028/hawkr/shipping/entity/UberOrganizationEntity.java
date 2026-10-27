package io.github.will413028.hawkr.shipping.entity;

import com.baomidou.mybatisplus.annotation.FieldFill;
import com.baomidou.mybatisplus.annotation.IdType;
import com.baomidou.mybatisplus.annotation.TableField;
import com.baomidou.mybatisplus.annotation.TableId;
import com.baomidou.mybatisplus.annotation.TableName;
import lombok.Data;

import java.time.LocalDateTime;

/**
 * 賣家 ↔ Uber 組織對應（66 §5.1.2）。
 *
 * <p>三問：寫入＝賣家開通流程／ADM｜讀取＝報價與建單前解析組織｜
 * 推走＝賣家關店 ⇒ {@code status='SUSPENDED'}，🔴 <b>不刪列</b>（帳務要追溯）。
 */
@Data
@TableName("uber_organization")
public class UberOrganizationEntity {

    @TableId(type = IdType.INPUT)
    private Long id;

    private Long uberAccountId;

    /**
     * 🔴 <b>賣家業務鍵 {@code store_id}（sellers PK），不是 {@code account_id}</b>
     * （66 §5.1.2 逐字；身分分離見設計書 44）。
     * <p>欄名必須誠實：這裡放 account_id 會讓整條賣家軸在 re-key 後指向錯的賣家。
     */
    private Long storeId;

    private String organizationId;

    /** {@code PENDING}｜{@code ACTIVE}｜{@code SUSPENDED}（值域由 DB CHECK {@code chk_uber_organization_status} 守）。 */
    private String status;

    @TableField(fill = FieldFill.INSERT)
    private LocalDateTime createdAt;

    @TableField(fill = FieldFill.INSERT_UPDATE)
    private LocalDateTime updatedAt;
}
