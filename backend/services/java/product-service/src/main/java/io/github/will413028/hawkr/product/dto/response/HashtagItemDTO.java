package io.github.will413028.hawkr.product.dto.response;

import lombok.AllArgsConstructor;
import lombok.Builder;
import lombok.Data;
import lombok.NoArgsConstructor;

/**
 * Hashtag 條目 DTO（33-商品 Hashtag 模組詳細設計書 §5.4）。
 *
 * <ul>
 *   <li>{@code name}：canonical 名稱（lower），查詢與比對基準</li>
 *   <li>{@code displayName}：顯示名稱（保留原大小寫），auto-complete 場景填空字串</li>
 *   <li>{@code productCount}：引用數量字串型別（讀取時計算的公開商品數，遵守 docs/開發規範/Java 微服務開發規範.md §2.1）</li>
 * </ul>
 */
@Data
@Builder
@NoArgsConstructor
@AllArgsConstructor
public class HashtagItemDTO {
    private String name;
    private String displayName;
    private String productCount;
}
