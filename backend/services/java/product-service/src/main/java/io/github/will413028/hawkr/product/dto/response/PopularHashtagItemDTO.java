package io.github.will413028.hawkr.product.dto.response;

import lombok.AllArgsConstructor;
import lombok.Builder;
import lombok.Data;
import lombok.NoArgsConstructor;

/**
 * 區間熱門 Hashtag 條目（33-商品 Hashtag 模組詳細設計書 §5.2.5）。
 *
 * <ul>
 *   <li>{@code hashtag}：區間內最多 ACTIVE 商品使用的原始大小寫版本</li>
 *   <li>{@code count}：區間內掛上此 hashtag 的 distinct ACTIVE 商品數（字串型別，docs/開發規範/Java 微服務開發規範.md §2.1）</li>
 * </ul>
 */
@Data
@Builder
@NoArgsConstructor
@AllArgsConstructor
public class PopularHashtagItemDTO {
    private String hashtag;
    private String count;
}
