package io.github.will413028.hawkr.payuni.shipping.response;

import io.github.will413028.hawkr.payuni.http.PayUniApiResponse;

/**
 * 超商出貨單列印回應（前景跳轉模式）
 * <p>
 * 此 API 為前景跳轉，回傳加密參數供前端 Form POST 提交至 PayUni URL。
 *
 * @param apiResponse   原始 API 回應（此場景不使用）
 * @param formActionUrl PayUni 表單提交 URL
 * @param encryptInfo   AES 加密字串
 * @param hashInfo      SHA256 雜湊
 */
public record PrintCvsLabelResponse(
        PayUniApiResponse apiResponse,
        String formActionUrl,
        String encryptInfo,
        String hashInfo
) {
}
