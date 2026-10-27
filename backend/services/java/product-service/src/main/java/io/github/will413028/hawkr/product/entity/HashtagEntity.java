package io.github.will413028.hawkr.product.entity;

import com.baomidou.mybatisplus.annotation.FieldFill;
import com.baomidou.mybatisplus.annotation.FieldStrategy;
import com.baomidou.mybatisplus.annotation.IdType;
import com.baomidou.mybatisplus.annotation.TableField;
import com.baomidou.mybatisplus.annotation.TableId;
import com.baomidou.mybatisplus.annotation.TableName;
import lombok.Data;

import java.io.Serializable;
import java.time.LocalDateTime;

/**
 * Hashtag 主表 Entity（02 設計書 §3.7）
 *
 * <p>每個 hashtag 以 canonical name 唯一識別；公開商品數不儲存，讀取時計算。
 */
@Data
@TableName("hashtag")
public class HashtagEntity implements Serializable {
    private static final long serialVersionUID = 1L;

    /** Hashtag ID（TSID，與 product 共用 generator） */
    @TableId(type = IdType.INPUT)
    private Long id;

    /** Canonical 名稱：trim + strip # + Unicode NFC + ASCII lower 後唯一識別（02 §3.9） */
    private String name;

    @TableField(fill = FieldFill.INSERT, updateStrategy = FieldStrategy.NEVER)
    private LocalDateTime createdAt;

    @TableField(fill = FieldFill.INSERT_UPDATE)
    private LocalDateTime updatedAt;
}
