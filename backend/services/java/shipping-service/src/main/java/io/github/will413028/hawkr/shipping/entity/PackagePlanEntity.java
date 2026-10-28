package io.github.will413028.hawkr.shipping.entity;

import com.baomidou.mybatisplus.annotation.FieldFill;
import com.baomidou.mybatisplus.annotation.IdType;
import com.baomidou.mybatisplus.annotation.TableField;
import com.baomidou.mybatisplus.annotation.TableId;
import com.baomidou.mybatisplus.annotation.TableName;
import lombok.Data;

import java.time.LocalDateTime;

/**
 * 分箱結果。
 *
 * <p>三問：寫入＝報價時的分箱演算法（與 {@code uber_quotes} 同一次寫入）｜
 * 讀取＝建單時組 Uber {@code packages} 欄位 ＋ 承載檢查｜
 * 推走＝🔴 生命週期<b>掛在 quote 上</b>：quote 保留 90 天，本表的列隨該 quote 一同由既有的
 * 資料清理排程移除。列本身沒有狀態要推。
 */
@Data
@TableName("package_plan")
public class PackagePlanEntity {

    @TableId(type = IdType.INPUT)
    private Long id;

    /** 對應 {@code uber_quotes.quote_id}（{@code dqt_} 前綴的那個值，<b>不是</b>本服務的 TSID）。 */
    private String quoteId;

    /** 本 quote 內的箱號（1 起）。與 {@link #quoteId} 合成 UNIQUE，擋報價重試造出重複的箱子。 */
    private Integer seq;

    private Integer lengthCm;
    private Integer widthCm;
    private Integer heightCm;

    /** 本箱重量（公克，🔴 <b>含包材</b> {@code packing_policy.packaging_weight_gram}）。 */
    private Integer weightGram;

    /**
     * Uber 的 {@code size}（🔴 <b>string enum 不是向量</b>，U-K／66 N4）。
     * <p>⚠️ 66 未列出其值域 ⇒ DB 刻意不加 CHECK；值域由 W2 的 sandbox golden fixture 覆核後再決定要不要釘。
     */
    private String sizeClass;

    @TableField(fill = FieldFill.INSERT)
    private LocalDateTime createdAt;

    @TableField(fill = FieldFill.INSERT_UPDATE)
    private LocalDateTime updatedAt;
}
