package io.github.will413028.hawkr.shipping.entity;

import com.baomidou.mybatisplus.annotation.FieldFill;
import com.baomidou.mybatisplus.annotation.IdType;
import com.baomidou.mybatisplus.annotation.TableField;
import com.baomidou.mybatisplus.annotation.TableId;
import com.baomidou.mybatisplus.annotation.TableName;
import lombok.Data;

import java.math.BigDecimal;
import java.time.LocalDateTime;

/**
 * 分箱政策（版本化）。
 *
 * <p>三問：寫入＝維運／ADM 新增版本（append-only，🔴 <b>不 UPDATE 既有版本</b>）｜
 * 讀取＝報價時的分箱演算法 ＋ 歷史訂單以 {@code uber_quotes.packing_policy_version} 回溯｜
 * 推走＝🔴 <b>不推走、不刪除</b>——舊版本被 quote 指著，刪了「這張訂單當時用什麼規則算的」就沒有答案。
 *
 * <p>🔴 <b>政策版本進 quote 快照</b>：改政策<b>不回溯既有訂單</b>，否則同一張訂單在不同時間
 * 算出不同運費，而運費已經收過了。
 *
 * <p>⚠️ {@code version} 不作 PK；本 repo 的主鍵慣例是 TSID ⇒
 * {@code version} 落為 UNIQUE 業務鍵，語意完全等價。
 */
@Data
@TableName("packing_policy")
public class PackingPolicyEntity {

    @TableId(type = IdType.INPUT)
    private Long id;

    /** 政策版本號（UNIQUE）。{@code uber_quotes.packing_policy_version} 指向本欄。 */
    private Integer version;

    /**
     * 材積膨脹係數（🔴 <b>係數不是金額</b> ⇒ 全服務唯一使用 {@link BigDecimal} 的欄位；
     * DB 型別 {@code NUMERIC(4,2)}，預設 {@code 1.25}）。
     *
     * <p>⚠️ 它是<b>估計不是事實</b> ⇒ 體積檢查是「下界估計 × 係數」⇒ 失敗文案必須是
     * 「超過限制」而不是「箱子放不下」。
     * <p>DB CHECK 守住 {@code > 0}：{@code <= 0} 會讓體積檢查恆過，而且完全無聲。
     */
    private BigDecimal voidFactor;

    /** 每箱包材重量（公克，預設 150），計入 {@code package_plan.weight_gram}。 */
    private Integer packagingWeightGram;

    /** 單箱最大件數（預設 12）。 */
    private Integer maxItemsPerPackage;

    /** 本版本生效時間（UTC）。生效版本＝{@code effective_from <= NOW()} 中 version 最大者。 */
    private LocalDateTime effectiveFrom;

    @TableField(fill = FieldFill.INSERT)
    private LocalDateTime createdAt;

    @TableField(fill = FieldFill.INSERT_UPDATE)
    private LocalDateTime updatedAt;
}
