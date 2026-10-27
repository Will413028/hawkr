package io.github.will413028.hawkr.product.entity;

import com.baomidou.mybatisplus.annotation.FieldFill;
import com.baomidou.mybatisplus.annotation.FieldStrategy;
import com.baomidou.mybatisplus.annotation.TableField;
import com.baomidou.mybatisplus.annotation.TableName;
import lombok.Data;

import java.io.Serializable;
import java.time.LocalDateTime;

/**
 * product 與 hashtag 關聯表 Entity（02 設計書 §3.8）
 *
 * <p>採複合主鍵 (product_id, hashtag_id)；display_name 保留賣家輸入時的原大小寫（per-binding，與 IG 一致）。
 */
@Data
@TableName("product_hashtag")
public class ProductHashtagEntity implements Serializable {
    private static final long serialVersionUID = 1L;

    /** 商品 ID（複合主鍵） */
    private Long productId;

    /** Hashtag ID（複合主鍵） */
    private Long hashtagId;

    /** 賣家輸入時保留原大小寫的版本（trim + strip # + NFC，**不** lower） */
    private String displayName;

    /** 賣家編輯時順序，0 起算；商品詳情頁 hashtag list 依此升序 */
    private Short position;

    @TableField(fill = FieldFill.INSERT, updateStrategy = FieldStrategy.NEVER)
    private LocalDateTime createdAt;
}
