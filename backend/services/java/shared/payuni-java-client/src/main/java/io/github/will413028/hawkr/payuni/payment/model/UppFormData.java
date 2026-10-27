package io.github.will413028.hawkr.payuni.payment.model;

/**
 * PAYUNi UPP 整合式支付頁 Form POST 資料
 * <p>
 * 前端收到後建立 hidden form，POST 至 actionUrl。
 *
 * @param actionUrl   PAYUNi UPP API URL
 * @param merId       MerID（子商家代號）
 * @param version     API 版本，固定 "2.0"
 * @param encryptInfo AES 加密字串
 * @param hashInfo    SHA256 簽章
 * @param isPlatform  代理商模式標記，固定 "1"
 */
public record UppFormData(
        String actionUrl,
        String merId,
        String version,
        String encryptInfo,
        String hashInfo,
        String isPlatform
) {

    /**
     * 建立代理商模式的 UPP 表單資料
     */
    public static UppFormData forPlatform(String baseUrl, String merId,
                                          String encryptInfo, String hashInfo) {
        return new UppFormData(
                baseUrl + "/api/upp",
                merId,
                "2.0",
                encryptInfo,
                hashInfo,
                "1"
        );
    }
}
