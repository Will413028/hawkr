package io.github.will413028.hawkr.payuni.model;

/**
 * PayUni 外層傳輸信封
 * <p>
 * 所有 PayUni API 請求/回應及 Webhook 通知的外層格式。
 *
 * @param merId       商家編號
 * @param version     API 版本
 * @param encryptInfo 加密後的業務資料
 * @param hashInfo    SHA256 簽章
 */
public record PayUniEnvelope(String merId, String version, String encryptInfo, String hashInfo) {
}
