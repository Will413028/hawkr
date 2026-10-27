package io.github.will413028.hawkr.product.domain.hashtag;

/**
 * Hashtag 規範化結果（02 設計書 §3.9）。
 *
 * <p>每筆對應賣家輸入序中的單一 hashtag：
 * <ul>
 *     <li>{@code canonical}：trim + strip # + Unicode NFC + ASCII lower（中文不變），用於 hashtag 主表 name 與比對基準</li>
 *     <li>{@code display}：trim + strip # + Unicode NFC（保留原大小寫），用於 product_hashtag.display_name</li>
 * </ul>
 */
public record NormalizedHashtag(String canonical, String display) {
}
