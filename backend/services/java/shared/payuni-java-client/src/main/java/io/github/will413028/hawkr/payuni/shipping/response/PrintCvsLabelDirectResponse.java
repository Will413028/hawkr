package io.github.will413028.hawkr.payuni.shipping.response;

/**
 * 超商出貨單列印回應（後端代理模式）
 * <p>
 * 後端呼叫 PayUni print_label API 後解析 HTML，提取標籤圖片 URL。
 *
 * @param imageUrl 標籤圖片完整 URL（PayUni 回傳 HTML 中 img src 拼接 baseUrl）
 */
public record PrintCvsLabelDirectResponse(String imageUrl) {
}
