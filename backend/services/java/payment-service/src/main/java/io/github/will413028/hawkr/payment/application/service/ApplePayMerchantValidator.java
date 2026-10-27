package io.github.will413028.hawkr.payment.application.service;

import io.github.will413028.hawkr.common.obs.log.ObsLogger;
import io.github.will413028.hawkr.common.utils.code.GrpcResponseCode;
import io.github.will413028.hawkr.payment.config.ApplePayProperties;
import io.github.will413028.hawkr.payment.exception.PaymentBusinessException;
import com.fasterxml.jackson.core.JsonProcessingException;
import com.fasterxml.jackson.databind.ObjectMapper;
import org.springframework.stereotype.Service;

import javax.net.ssl.KeyManagerFactory;
import javax.net.ssl.SSLContext;
import javax.net.ssl.TrustManagerFactory;
import java.io.ByteArrayInputStream;
import java.io.IOException;
import java.net.URI;
import java.net.URISyntaxException;
import java.net.http.HttpClient;
import java.net.http.HttpRequest;
import java.net.http.HttpResponse;
import java.nio.file.Files;
import java.nio.file.NoSuchFileException;
import java.nio.file.Path;
import java.security.KeyFactory;
import java.security.KeyManagementException;
import java.security.KeyStore;
import java.security.KeyStoreException;
import java.security.NoSuchAlgorithmException;
import java.security.PrivateKey;
import java.security.UnrecoverableKeyException;
import java.security.cert.CertificateException;
import java.security.cert.CertificateFactory;
import java.security.cert.X509Certificate;
import java.security.spec.InvalidKeySpecException;
import java.security.spec.PKCS8EncodedKeySpec;
import java.time.Duration;
import java.util.Base64;
import java.util.LinkedHashMap;
import java.util.List;
import java.util.Map;
import java.util.concurrent.atomic.AtomicReference;

/**
 * Apple Pay on Web Merchant Validation usecase。
 *
 * 設計參考：docs/詳細設計書/05-支付服務（Payment Service）詳細設計書.md §5.11
 *
 * 流程：SSRF 檢查 → domain allowlist → 載入 cert+key 建 SSLContext（lazy + cache）
 *      → mTLS POST Apple validationURL → 原樣回傳 merchantSession JSON
 *
 * Log event 集中於 validate() 主方法：
 *   validate.started / validate.rejected / validate.error / validate.success
 * cert 載入失敗 log 在 loadSslContext.cert-error（因需帶具體 exception 類型）。
 */
@Service
public class ApplePayMerchantValidator {

    private static final ObsLogger log = ObsLogger.forClass(ApplePayMerchantValidator.class);

    /** SSRF：path 必須以此開頭。 */
    private static final String APPLE_PATH_PREFIX = "/paymentservices/";

    /** SSRF：host 必須以此結尾。 */
    private static final String APPLE_HOST_SUFFIX = ".apple.com";

    /** SSRF：validationURL 長度上限（reviewer §8，避免大 payload 攻擊 / log 汙染）。 */
    private static final int MAX_VALIDATION_URL_LENGTH = 2048;

    /** SSRF：domainName 長度上限（FQDN RFC 1035 最大 253 chars）。 */
    private static final int MAX_DOMAIN_NAME_LENGTH = 253;

    /** PKCS12 in-memory keystore alias + 密碼（不離開 process）。 */
    private static final String KEYSTORE_ALIAS = "apple-pay-merchant";
    private static final char[] KEYSTORE_PASSWORD = "changeit".toCharArray();

    /** Apple POST body 固定欄位 initiative。 */
    private static final String INITIATIVE_WEB = "web";

    /** non-2xx response body 截斷長度（log 用，避免 Apple 錯誤回應汙染 log）。 */
    private static final int RESPONSE_BODY_LOG_LIMIT = 300;

    private final ApplePayProperties properties;
    private final ObjectMapper objectMapper;
    private final AtomicReference<HttpClient> cachedClient = new AtomicReference<>();

    public ApplePayMerchantValidator(ApplePayProperties properties, ObjectMapper objectMapper) {
        this.properties = properties;
        this.objectMapper = objectMapper;
    }

    /**
     * 對 Apple validationURL 取得 merchantSession，原樣回傳 JSON 字串。
     *
     * @throws PaymentBusinessException 以 APPLE_PAY_* 業務碼包裝失敗情境
     */
    public String validate(String validationUrl, String domainName, String displayName) {
        log.info("started", "Apple Pay merchant validation start",
                "domain", String.valueOf(domainName));

        URI uri;
        try {
            uri = parseAndCheckUrl(validationUrl, domainName);
            ensureDomainAllowed(domainName);
        } catch (PaymentBusinessException e) {
            log.warn("rejected", "validation request rejected",
                    "domain", String.valueOf(domainName));
            throw e;
        }

        HttpClient client = getOrInitHttpClient();
        String body = buildRequestBody(domainName, displayName);

        HttpRequest request = HttpRequest.newBuilder()
                .uri(uri)
                .timeout(Duration.ofSeconds(properties.getRequestTimeoutSeconds()))
                .header("Content-Type", "application/json")
                .POST(HttpRequest.BodyPublishers.ofString(body))
                .build();

        HttpResponse<String> response;
        try {
            response = client.send(request, HttpResponse.BodyHandlers.ofString());
        } catch (IOException | InterruptedException e) {
            if (e instanceof InterruptedException) {
                Thread.currentThread().interrupt();
            }
            log.error("error", "Apple Pay mTLS POST failed: "
                            + e.getClass().getSimpleName() + " - " + e.getMessage(),
                    "domain", String.valueOf(domainName));
            throw new PaymentBusinessException(GrpcResponseCode.APPLE_PAY_MERCHANT_VALIDATION_FAILED);
        }

        if (response.statusCode() / 100 != 2) {
            log.error("error", "Apple Pay non-2xx: status=" + response.statusCode()
                            + " domain=" + domainName
                            + " body=" + truncate(response.body(), RESPONSE_BODY_LOG_LIMIT),
                    "status", String.valueOf(response.statusCode()));
            throw new PaymentBusinessException(GrpcResponseCode.APPLE_PAY_MERCHANT_VALIDATION_FAILED);
        }

        log.info("success", "Apple Pay merchant validation success",
                "domain", String.valueOf(domainName));
        return response.body();
    }

    /* -------------------- SSRF / Input validation -------------------- */

    private URI parseAndCheckUrl(String validationUrl, String domainName) {
        // 1. null / blank
        if (validationUrl == null || validationUrl.isBlank()
                || domainName == null || domainName.isBlank()) {
            throw new PaymentBusinessException(GrpcResponseCode.APPLE_PAY_VALIDATION_URL_REJECTED);
        }
        // 2. 長度上限（防大 payload 與 log 汙染）
        if (validationUrl.length() > MAX_VALIDATION_URL_LENGTH
                || domainName.length() > MAX_DOMAIN_NAME_LENGTH) {
            throw new PaymentBusinessException(GrpcResponseCode.APPLE_PAY_VALIDATION_URL_REJECTED);
        }
        // 3. parse + normalize（防 /paymentservices/../admin path traversal）
        URI uri;
        try {
            uri = new URI(validationUrl).normalize();
        } catch (URISyntaxException e) {
            throw new PaymentBusinessException(GrpcResponseCode.APPLE_PAY_VALIDATION_URL_REJECTED);
        }
        // 4. defensive reject userInfo（Apple 不會給含 user:pass@host 的 URL）
        if (uri.getRawUserInfo() != null) {
            throw new PaymentBusinessException(GrpcResponseCode.APPLE_PAY_VALIDATION_URL_REJECTED);
        }
        // 5. 三層白名單：scheme + host + path
        String scheme = uri.getScheme();
        String host = uri.getHost();
        String path = uri.getPath();
        if (scheme == null || !scheme.equalsIgnoreCase("https")
                || host == null || !host.toLowerCase().endsWith(APPLE_HOST_SUFFIX)
                || path == null || !path.startsWith(APPLE_PATH_PREFIX)) {
            throw new PaymentBusinessException(GrpcResponseCode.APPLE_PAY_VALIDATION_URL_REJECTED);
        }
        // 6. port 必須為 -1（未指定）或 443
        int port = uri.getPort();
        if (port != -1 && port != 443) {
            throw new PaymentBusinessException(GrpcResponseCode.APPLE_PAY_VALIDATION_URL_REJECTED);
        }
        return uri;
    }

    private void ensureDomainAllowed(String domainName) {
        List<String> allowlist = properties.getDomainAllowlist();
        if (allowlist == null || allowlist.isEmpty()) {
            throw new PaymentBusinessException(GrpcResponseCode.APPLE_PAY_VALIDATION_URL_REJECTED);
        }
        for (String allowed : allowlist) {
            if (allowed != null && allowed.equalsIgnoreCase(domainName)) {
                return;
            }
        }
        throw new PaymentBusinessException(GrpcResponseCode.APPLE_PAY_VALIDATION_URL_REJECTED);
    }

    /* -------------------- mTLS HttpClient（首次 lazy + cache） -------------------- */

    private HttpClient getOrInitHttpClient() {
        HttpClient existing = cachedClient.get();
        if (existing != null) {
            return existing;
        }
        synchronized (cachedClient) {
            HttpClient again = cachedClient.get();
            if (again != null) {
                return again;
            }
            HttpClient created = buildMtlsHttpClient();
            cachedClient.set(created);
            return created;
        }
    }

    private HttpClient buildMtlsHttpClient() {
        SSLContext sslContext = loadSslContext();
        return HttpClient.newBuilder()
                .sslContext(sslContext)
                .connectTimeout(Duration.ofSeconds(properties.getConnectTimeoutSeconds()))
                .build();
    }

    private SSLContext loadSslContext() {
        String certPath = properties.getMerchantCertPath();
        String keyPath = properties.getMerchantKeyPath();
        if (certPath == null || certPath.isBlank() || keyPath == null || keyPath.isBlank()) {
            log.error("cert-error", "Apple Pay cert/key path not configured");
            throw new PaymentBusinessException(GrpcResponseCode.APPLE_PAY_CERT_LOAD_FAILED);
        }
        try {
            X509Certificate certificate = readCertificate(Path.of(certPath));
            PrivateKey privateKey = readPrivateKey(Path.of(keyPath));

            KeyStore keyStore = KeyStore.getInstance("PKCS12");
            keyStore.load(null, KEYSTORE_PASSWORD);
            keyStore.setKeyEntry(KEYSTORE_ALIAS, privateKey, KEYSTORE_PASSWORD,
                    new X509Certificate[]{certificate});

            KeyManagerFactory kmf = KeyManagerFactory.getInstance(KeyManagerFactory.getDefaultAlgorithm());
            kmf.init(keyStore, KEYSTORE_PASSWORD);

            TrustManagerFactory tmf = TrustManagerFactory.getInstance(TrustManagerFactory.getDefaultAlgorithm());
            tmf.init((KeyStore) null);

            SSLContext sslContext = SSLContext.getInstance("TLSv1.2");
            sslContext.init(kmf.getKeyManagers(), tmf.getTrustManagers(), null);
            return sslContext;
        } catch (NoSuchFileException e) {
            log.error("cert-error", "Apple Pay cert/key file not found: " + e.getMessage());
            throw new PaymentBusinessException(GrpcResponseCode.APPLE_PAY_CERT_LOAD_FAILED);
        } catch (KeyStoreException | NoSuchAlgorithmException | CertificateException
                 | UnrecoverableKeyException | InvalidKeySpecException
                 | KeyManagementException | IOException | IllegalStateException e) {
            // IllegalStateException 涵蓋 PKCS#1 friendly fail；
            // 其他未列出 RuntimeException（NPE 等）讓 GrpcErrorHandler catch-all 抓 → 500 INTERNAL_ERROR
            log.error("cert-error", "Apple Pay cert/key load failed: "
                    + e.getClass().getSimpleName() + " - " + e.getMessage());
            throw new PaymentBusinessException(GrpcResponseCode.APPLE_PAY_CERT_LOAD_FAILED);
        }
    }

    private X509Certificate readCertificate(Path path) throws IOException, CertificateException {
        byte[] pem = Files.readAllBytes(path);
        try (var in = new ByteArrayInputStream(pem)) {
            CertificateFactory cf = CertificateFactory.getInstance("X.509");
            return (X509Certificate) cf.generateCertificate(in);
        }
    }

    private PrivateKey readPrivateKey(Path path) throws IOException,
            NoSuchAlgorithmException, InvalidKeySpecException {
        String pem = Files.readString(path);
        // PKCS#1 friendly fail：openssl 預設輸出格式之一，但 JDK 不直接支援
        if (pem.contains("-----BEGIN RSA PRIVATE KEY-----")) { // gitleaks:allow（PEM 檔頭字面，不是金鑰）
            throw new IllegalStateException("PKCS#1 RSA key not supported; "
                    + "convert with: openssl pkcs8 -topk8 -nocrypt -in <pkcs1.key> -out <pkcs8.key>");
        }
        String base64 = pem
                .replace("-----BEGIN PRIVATE KEY-----REDACTED-----END PRIVATE KEY-----", "")
                .replaceAll("\\s", "");
        byte[] der = Base64.getDecoder().decode(base64);
        PKCS8EncodedKeySpec spec = new PKCS8EncodedKeySpec(der);
        KeyFactory kf = KeyFactory.getInstance("RSA");
        return kf.generatePrivate(spec);
    }

    /* -------------------- Apple POST body -------------------- */

    private String buildRequestBody(String domainName, String displayName) {
        String resolvedDisplayName = (displayName == null || displayName.isBlank())
                ? properties.getDisplayName() : displayName;
        Map<String, String> body = new LinkedHashMap<>();
        body.put("merchantIdentifier", properties.getMerchantId());
        body.put("displayName", resolvedDisplayName);
        body.put("initiative", INITIATIVE_WEB);
        body.put("initiativeContext", domainName);
        try {
            return objectMapper.writeValueAsString(body);
        } catch (JsonProcessingException e) {
            // 邊角：Map<String,String> 都是 string 不該失敗；發生即視為 cert-error path
            log.error("cert-error", "Apple POST body json serialization failed: " + e.getMessage());
            throw new PaymentBusinessException(GrpcResponseCode.APPLE_PAY_CERT_LOAD_FAILED);
        }
    }

    private static String truncate(String value, int limit) {
        if (value == null) return "";
        return value.length() > limit ? value.substring(0, limit) + "...[truncated]" : value;
    }
}
