package io.github.will413028.hawkr.payment.application.service;

import io.github.will413028.hawkr.common.utils.code.GrpcResponseCode;
import io.github.will413028.hawkr.payment.config.ApplePayProperties;
import io.github.will413028.hawkr.payment.exception.PaymentBusinessException;
import com.fasterxml.jackson.databind.ObjectMapper;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.DisplayName;
import org.junit.jupiter.api.Test;

import java.util.List;

import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.junit.jupiter.api.Assertions.assertThrows;

/**
 * SSRF / domain allowlist 邊界測試。
 *
 * mTLS POST 真打 Apple 不在單測範圍，那部分在 §7 整合驗證中以 host 服務手動測試。
 *
 * Cases 涵蓋：
 *  - scheme（http/file）/ host（非 apple、偽冒、大小寫、IPv4、IPv6）/ path（非 paymentservices、traversal）/ port（非 443）
 *  - null/blank（validation_url、domain_name）/ 超長
 *  - userInfo defensive reject
 *  - allowlist 為空 / domain 不在 allowlist
 *  - SSRF 通過但 cert path 未配置（8029）
 */
class ApplePayMerchantValidatorTest {

    private static final String VALID_URL = "https://apple-pay-gateway-cert.apple.com/paymentservices/startSession";
    private static final String VALID_DOMAIN = "web-dev.example.com";

    private ApplePayProperties properties;
    private ApplePayMerchantValidator validator;

    @BeforeEach
    void setUp() {
        properties = new ApplePayProperties();
        properties.setMerchantId("merchant.io.github.will413028.hawkr.sandbox");
        properties.setDisplayName("範例生鮮");
        properties.setDomainAllowlist(List.of(VALID_DOMAIN));
        // 不設 cert / key path — 任何到 mTLS 階段的 case 都會回 8029
        validator = new ApplePayMerchantValidator(properties, new ObjectMapper());
    }

    private void assertRejected(String validationUrl, String domainName) {
        var ex = assertThrows(PaymentBusinessException.class, () ->
                validator.validate(validationUrl, domainName, "test"));
        assertEquals(GrpcResponseCode.APPLE_PAY_VALIDATION_URL_REJECTED.code(), ex.code());
    }

    /* ============ scheme ============ */

    @Test
    @DisplayName("scheme=http 拒絕 8028")
    void rejectsHttpScheme() {
        assertRejected("http://apple-pay-gateway.apple.com/paymentservices/startSession", VALID_DOMAIN);
    }

    @Test
    @DisplayName("scheme=file 拒絕 8028")
    void rejectsFileScheme() {
        assertRejected("file:///etc/passwd", VALID_DOMAIN);
    }

    /* ============ host ============ */

    @Test
    @DisplayName("host 非 *.apple.com 拒絕 8028")
    void rejectsNonAppleHost() {
        assertRejected("https://evil.com/paymentservices/startSession", VALID_DOMAIN);
    }

    @Test
    @DisplayName("host 偽冒 apple.com.evil.com 拒絕 8028")
    void rejectsSpoofedAppleHost() {
        assertRejected("https://apple.com.evil.com/paymentservices/startSession", VALID_DOMAIN);
    }

    @Test
    @DisplayName("host 全大寫 APPLE-PAY-GATEWAY.APPLE.COM 通過 host 檢查（lowercased endsWith）")
    void acceptsUppercaseHost() {
        // 大小寫應通過 SSRF 檢查（lowercase 後 endsWith），最終回 8029（cert 未配置）證明走到 mTLS 階段
        var ex = assertThrows(PaymentBusinessException.class, () ->
                validator.validate("https://APPLE-PAY-GATEWAY-CERT.APPLE.COM/paymentservices/startSession",
                        VALID_DOMAIN, "test"));
        assertEquals(GrpcResponseCode.APPLE_PAY_CERT_LOAD_FAILED.code(), ex.code());
    }

    @Test
    @DisplayName("IPv4 literal host 拒絕 8028（不會 endsWith .apple.com）")
    void rejectsIpv4LiteralHost() {
        assertRejected("https://17.253.144.10/paymentservices/startSession", VALID_DOMAIN);
    }

    @Test
    @DisplayName("IPv6 literal host 拒絕 8028（[]bracketed host 不會 endsWith .apple.com）")
    void rejectsIpv6LiteralHost() {
        assertRejected("https://[2620:149:af0:9::10]/paymentservices/startSession", VALID_DOMAIN);
    }

    /* ============ path ============ */

    @Test
    @DisplayName("path 不在 /paymentservices/* 拒絕 8028")
    void rejectsNonPaymentServicesPath() {
        assertRejected("https://apple-pay-gateway.apple.com/admin/startSession", VALID_DOMAIN);
    }

    @Test
    @DisplayName("path traversal /paymentservices/../admin 經 normalize 後拒絕 8028")
    void rejectsPathTraversal() {
        assertRejected("https://apple-pay-gateway.apple.com/paymentservices/../admin/secret", VALID_DOMAIN);
    }

    /* ============ userInfo ============ */

    @Test
    @DisplayName("URL 含 userInfo 拒絕 8028（defensive reject，Apple 不會給 user:pass@host）")
    void rejectsUserInfo() {
        assertRejected("https://attacker:pass@apple-pay-gateway-cert.apple.com/paymentservices/startSession",
                VALID_DOMAIN);
    }

    /* ============ port ============ */

    @Test
    @DisplayName("port != 443 (8080) 拒絕 8028")
    void rejectsNonHttpsPort() {
        assertRejected("https://apple-pay-gateway-cert.apple.com:8080/paymentservices/startSession",
                VALID_DOMAIN);
    }

    /* ============ length / null / blank ============ */

    @Test
    @DisplayName("validation_url 空字串拒絕 8028")
    void rejectsEmptyValidationUrl() {
        assertRejected("", VALID_DOMAIN);
    }

    @Test
    @DisplayName("domain_name = null 拒絕 8028")
    void rejectsNullDomain() {
        assertRejected(VALID_URL, null);
    }

    @Test
    @DisplayName("domain_name 空字串拒絕 8028")
    void rejectsBlankDomain() {
        assertRejected(VALID_URL, "");
    }

    @Test
    @DisplayName("超長 validation_url（> 2048）拒絕 8028")
    void rejectsOverlongUrl() {
        String pad = "/" + "a".repeat(2100);
        assertRejected("https://apple-pay-gateway-cert.apple.com/paymentservices" + pad, VALID_DOMAIN);
    }

    @Test
    @DisplayName("超長 domain_name（> 253）拒絕 8028")
    void rejectsOverlongDomain() {
        String overlongDomain = "a".repeat(260) + ".example.com";
        assertRejected(VALID_URL, overlongDomain);
    }

    /* ============ allowlist ============ */

    @Test
    @DisplayName("domain_name 不在 allowlist 拒絕 8028")
    void rejectsDomainNotInAllowlist() {
        assertRejected("https://apple-pay-gateway.apple.com/paymentservices/startSession",
                "evil.example.com");
    }

    @Test
    @DisplayName("allowlist 為空（未配置）拒絕 8028")
    void rejectsWhenAllowlistEmpty() {
        properties.setDomainAllowlist(List.of());
        assertRejected("https://apple-pay-gateway.apple.com/paymentservices/startSession", VALID_DOMAIN);
    }

    /* ============ URL syntax ============ */

    @Test
    @DisplayName("URL 語法錯誤拒絕 8028")
    void rejectsMalformedUrl() {
        assertRejected("https://apple-pay-gateway.apple.com:abc/paymentservices/startSession", VALID_DOMAIN);
    }

    /* ============ SSRF 通過 → cert 階段 ============ */

    @Test
    @DisplayName("SSRF 全通過但 cert/key path 未設定 → 8029 APPLE_PAY_CERT_LOAD_FAILED")
    void certPathNotConfiguredFailsLoad() {
        var ex = assertThrows(PaymentBusinessException.class, () ->
                validator.validate(VALID_URL, VALID_DOMAIN, "test"));
        assertEquals(GrpcResponseCode.APPLE_PAY_CERT_LOAD_FAILED.code(), ex.code());
    }
}
