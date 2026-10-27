package io.github.will413028.hawkr.product.dto.response;

import lombok.AllArgsConstructor;
import lombok.Builder;
import lombok.Data;
import lombok.NoArgsConstructor;

import java.io.Serializable;
import java.time.LocalDateTime;

/**
 * 評論預覽 DTO（公開商品詳情用，最多 2 筆）
 */
@Data
@Builder
@NoArgsConstructor
@AllArgsConstructor
public class ReviewPreviewDTO implements Serializable {
    private static final long serialVersionUID = 1L;

    /** 評論 ID */
    private String reviewId;

    /** 評論者 buyer ID（目前無 caller 填充此 DTO，stub） */
    private Long buyerId;

    /** 評分 */
    private String rating;

    /** 評論內容 */
    private String content;

    /** 建立時間 */
    private LocalDateTime createdAt;
}
