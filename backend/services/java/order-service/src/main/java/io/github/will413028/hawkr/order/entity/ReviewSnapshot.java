package io.github.will413028.hawkr.order.entity;

import lombok.AllArgsConstructor;
import lombok.Data;
import lombok.NoArgsConstructor;

import java.util.List;

/**
 * orders.review JSONB 結構：訂單評論快照（訂單層級單一）。
 *
 * 來源 review-service review.created MQ 事件，取該訂單**最近一筆**商品評論。
 * 平台評論不可改/刪 → 凍結快照（03 §8.9）。key 為 camelCase，與 JSONB 內容一致（§11.1）。
 */
@Data
@NoArgsConstructor
@AllArgsConstructor
public class ReviewSnapshot {

    /** 評分 1-5 */
    private Integer rating;

    /** 評論文字（可空） */
    private String content;

    /** 評論附圖 objectKey（無圖為空陣列；顯示 URL 由 Gateway 組，49 §5.3/§6.2） */
    private List<String> imageKeys;

    /** 評論建立時間（ISO 8601 UTC） */
    private String createdAt;
}
