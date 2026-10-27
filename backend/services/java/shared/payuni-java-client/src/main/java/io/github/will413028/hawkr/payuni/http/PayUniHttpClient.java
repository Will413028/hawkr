package io.github.will413028.hawkr.payuni.http;

import io.github.will413028.hawkr.payuni.crypto.PayUniCryptoService;
import com.fasterxml.jackson.core.type.TypeReference;
import com.fasterxml.jackson.databind.ObjectMapper;
import okhttp3.*;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;

import java.io.IOException;
import java.util.Map;
import java.util.concurrent.TimeUnit;

/**
 * PayUni HTTP 底層客戶端
 * <p>
 * 負責：加密參數 → 組裝 Form POST → 發送 HTTP → 解析回應 → 驗簽解密。
 * 代理商模式下所有請求自動帶 IsPlatForm=1。
 * <p>
 * 稽核：每個入口（callApi / callAgentApi / callApiHtml）以「可變 context 累加器 + finally 單次觸發」
 * 保證每次 exchange 恰好觸發一次 {@link PayUniRequestLogger#onExchange}，成功與各類失敗路徑皆不遺漏、不重複。
 */
public class PayUniHttpClient {

    private static final Logger log = LoggerFactory.getLogger(PayUniHttpClient.class);

    private final PayUniCryptoService cryptoService;
    private final ObjectMapper objectMapper;
    private final OkHttpClient httpClient;
    private final String baseUrl;
    private volatile PayUniRequestLogger requestLogger = PayUniRequestLogger.NOOP;

    public PayUniHttpClient(PayUniCryptoService cryptoService, String baseUrl) {
        this(cryptoService, baseUrl, new ObjectMapper());
    }

    public PayUniHttpClient(PayUniCryptoService cryptoService, String baseUrl, ObjectMapper objectMapper) {
        this.cryptoService = cryptoService;
        this.baseUrl = baseUrl;
        this.objectMapper = objectMapper;
        this.httpClient = new OkHttpClient.Builder()
                .connectTimeout(30, TimeUnit.SECONDS)
                .readTimeout(30, TimeUnit.SECONDS)
                .writeTimeout(30, TimeUnit.SECONDS)
                .build();
    }

    /**
     * 注入 audit logger（服務啟動時透過 PayUniConfig 設定）。
     * 預設為 NOOP，無 logger 時不影響行為。
     */
    public void setRequestLogger(PayUniRequestLogger logger) {
        this.requestLogger = logger != null ? logger : PayUniRequestLogger.NOOP;
    }

    /** 於 finally 觸發一次；吞掉實作端例外，不阻斷 PayUni 主流程。 */
    private void safeOnExchange(PayUniExchangeContext ctx) {
        try {
            requestLogger.onExchange(ctx);
        } catch (Throwable t) {
            log.warn("PayUniRequestLogger.onExchange failed: {}", t.getMessage());
        }
    }

    /**
     * 呼叫 PayUni API（代理商模式，Form POST）
     *
     * @param merId   子商家 ID
     * @param path    API 路徑（如 "/api/atm"）
     * @param params  內層業務參數
     * @param version API 版本
     * @return 解析後的回應
     */
    public PayUniApiResponse callApi(String merId, String path, Map<String, String> params, String version) {
        log.info("PayUni API 請求: path={}, merId={}, version={}, params={}", path, merId, version, params);
        long t0 = System.currentTimeMillis();
        PayUniExchangeContext ctx = new PayUniExchangeContext(merId, path, params);
        try {
            String encryptStr = cryptoService.encrypt(params);
            String hashStr = cryptoService.calculateHash(encryptStr);

            FormBody formBody = new FormBody.Builder()
                    .add("MerID", merId)
                    .add("Version", version)
                    .add("EncryptInfo", encryptStr)
                    .add("HashInfo", hashStr)
                    .add("IsPlatForm", "1")
                    .build();

            Request request = new Request.Builder()
                    .url(baseUrl + path)
                    .header("User-Agent", "payuni")
                    .header("Content-Type", "application/x-www-form-urlencoded")
                    .post(formBody)
                    .build();

            try (Response response = httpClient.newCall(request).execute()) {
                ctx.setHttpStatus(response.code());
                if (!response.isSuccessful()) {
                    ctx.setErrorCode("HTTP_ERROR");
                    ctx.setErrorMessage("HTTP " + response.code());
                    throw new PayUniApiException("HTTP_ERROR", "HTTP " + response.code(), response.code());
                }

                ResponseBody body = response.body();
                if (body == null) {
                    ctx.setErrorCode("EMPTY_RESPONSE");
                    ctx.setErrorMessage("回應為空");
                    throw new PayUniApiException("EMPTY_RESPONSE", "回應為空", response.code());
                }

                String responseBody = body.string();
                log.info("PayUni API 回應: path={}, body={}", path, responseBody);
                ctx.setRawResponseBody(responseBody);
                PayUniApiResponse parsed = parseResponse(responseBody);
                captureParsed(ctx, parsed);
                return parsed;
            }
        } catch (PayUniApiException e) {
            throw e;
        } catch (IOException e) {
            ctx.setErrorCode("IO_EXCEPTION");
            ctx.setErrorMessage(e.getMessage());
            if (ctx.getHttpStatus() == null) {
                ctx.setHttpStatus(0); // 連線層失敗（未取得 HTTP 狀態）；200 後讀 body 失敗保留原狀態碼
            }
            throw new PayUniApiException("PayUni API 呼叫失敗: " + path, e);
        } finally {
            ctx.setDurationMs(System.currentTimeMillis() - t0);
            safeOnExchange(ctx);
        }
    }

    /**
     * 呼叫 PayUni 代理商級別 API（外層使用 AgentID 而非 MerID）
     *
     * 與 callApi() 不同：
     * - 外層使用 AgentID 而非 MerID
     * - 不帶 IsPlatForm
     * 適用於撥款指示等代理商專用 API（/api/trade/grant 等）。
     */
    public PayUniApiResponse callAgentApi(String agentId, String path, Map<String, String> params, String version) {
        log.info("PayUni Agent API 請求: path={}, agentId={}, version={}, params={}", path, agentId, version, params);
        long t0 = System.currentTimeMillis();
        PayUniExchangeContext ctx = new PayUniExchangeContext(agentId, path, params);
        try {
            String encryptStr = cryptoService.encrypt(params);
            String hashStr = cryptoService.calculateHash(encryptStr);

            FormBody formBody = new FormBody.Builder()
                    .add("AgentID", agentId)
                    .add("Version", version)
                    .add("EncryptInfo", encryptStr)
                    .add("HashInfo", hashStr)
                    .build();

            Request request = new Request.Builder()
                    .url(baseUrl + path)
                    .header("User-Agent", "payuni")
                    .header("Content-Type", "application/x-www-form-urlencoded")
                    .post(formBody)
                    .build();

            try (Response response = httpClient.newCall(request).execute()) {
                ctx.setHttpStatus(response.code());
                if (!response.isSuccessful()) {
                    ctx.setErrorCode("HTTP_ERROR");
                    ctx.setErrorMessage("HTTP " + response.code());
                    throw new PayUniApiException("HTTP_ERROR", "HTTP " + response.code(), response.code());
                }

                ResponseBody body = response.body();
                if (body == null) {
                    ctx.setErrorCode("EMPTY_RESPONSE");
                    ctx.setErrorMessage("回應為空");
                    throw new PayUniApiException("EMPTY_RESPONSE", "回應為空", response.code());
                }

                String responseBody = body.string();
                log.info("PayUni Agent API 回應: path={}, body={}", path, responseBody);
                ctx.setRawResponseBody(responseBody);
                PayUniApiResponse parsed = parseResponse(responseBody);
                captureParsed(ctx, parsed);
                return parsed;
            }
        } catch (PayUniApiException e) {
            throw e;
        } catch (IOException e) {
            ctx.setErrorCode("IO_EXCEPTION");
            ctx.setErrorMessage(e.getMessage());
            if (ctx.getHttpStatus() == null) {
                ctx.setHttpStatus(0); // 連線層失敗（未取得 HTTP 狀態）；200 後讀 body 失敗保留原狀態碼
            }
            throw new PayUniApiException("PayUni Agent API 呼叫失敗: " + path, e);
        } finally {
            ctx.setDurationMs(System.currentTimeMillis() - t0);
            safeOnExchange(ctx);
        }
    }

    /**
     * 呼叫 PayUni API，回傳原始 HTML 字串（適用於 print_label 等非 JSON 回應的 API）
     *
     * @param merId   子商家 ID
     * @param path    API 路徑
     * @param params  內層業務參數
     * @param version API 版本
     * @return 原始 HTML 回應字串
     */
    public String callApiHtml(String merId, String path, Map<String, String> params, String version) {
        log.info("PayUni API 請求 (HTML): path={}, merId={}, version={}, params={}", path, merId, version, params);
        long t0 = System.currentTimeMillis();
        PayUniExchangeContext ctx = new PayUniExchangeContext(merId, path, params);
        try {
            String encryptStr = cryptoService.encrypt(params);
            String hashStr = cryptoService.calculateHash(encryptStr);

            FormBody formBody = new FormBody.Builder()
                    .add("MerID", merId)
                    .add("Version", version)
                    .add("EncryptInfo", encryptStr)
                    .add("HashInfo", hashStr)
                    .add("IsPlatForm", "1")
                    .build();

            Request request = new Request.Builder()
                    .url(baseUrl + path)
                    .header("User-Agent", "payuni")
                    .header("Content-Type", "application/x-www-form-urlencoded")
                    .post(formBody)
                    .build();

            try (Response response = httpClient.newCall(request).execute()) {
                ctx.setHttpStatus(response.code());
                if (!response.isSuccessful()) {
                    ctx.setErrorCode("HTTP_ERROR");
                    ctx.setErrorMessage("HTTP " + response.code());
                    throw new PayUniApiException("HTTP_ERROR", "HTTP " + response.code(), response.code());
                }

                ResponseBody body = response.body();
                if (body == null) {
                    ctx.setErrorCode("EMPTY_RESPONSE");
                    ctx.setErrorMessage("回應為空");
                    throw new PayUniApiException("EMPTY_RESPONSE", "回應為空", response.code());
                }

                String html = body.string();
                log.info("PayUni API 回應 (HTML): path={}, length={}", path, html.length());
                // HTML 回應無 parseResponse（無解密內層）；response_raw 記 HTML 全文（不截斷）。
                ctx.setRawResponseBody(html);
                return html;
            }
        } catch (PayUniApiException e) {
            throw e;
        } catch (IOException e) {
            ctx.setErrorCode("IO_EXCEPTION");
            ctx.setErrorMessage(e.getMessage());
            if (ctx.getHttpStatus() == null) {
                ctx.setHttpStatus(0); // 連線層失敗（未取得 HTTP 狀態）；200 後讀 body 失敗保留原狀態碼
            }
            throw new PayUniApiException("PayUni API 呼叫失敗: " + path, e);
        } finally {
            ctx.setDurationMs(System.currentTimeMillis() - t0);
            safeOnExchange(ctx);
        }
    }

    /**
     * 取得 PayUni base URL（供組合圖片完整 URL 使用）
     */
    public String getBaseUrl() {
        return baseUrl;
    }

    /**
     * 取得加密服務（供建立 UPP 表單資料等場景使用）
     */
    public PayUniCryptoService getCryptoService() {
        return cryptoService;
    }

    /** 把 parseResponse 的外層 status/message 與解密內層帶入稽核 context（不改變 parseResponse 對外行為）。 */
    private void captureParsed(PayUniExchangeContext ctx, PayUniApiResponse parsed) {
        ctx.setOuterStatus(parsed.status());
        ctx.setOuterMessage(parsed.message());
        ctx.setDecryptedPayload(parsed.decryptedData());
    }

    /**
     * 解析 PayUni API 回應（JSON → 驗簽 → 解密）
     */
    private PayUniApiResponse parseResponse(String responseBody) {
        try {
            Map<String, Object> raw = objectMapper.readValue(responseBody, new TypeReference<>() {});

            String status = (String) raw.get("Status");
            String message = (String) raw.get("Message");

            Map<String, String> decryptedData = null;

            Object encryptInfo = raw.get("EncryptInfo");
            Object hashInfo = raw.get("HashInfo");

            if (encryptInfo != null && hashInfo != null) {
                String encryptStr = encryptInfo.toString();
                String hash = hashInfo.toString();

                if (!encryptStr.isEmpty() && !hash.isEmpty()) {
                    if (!cryptoService.verifyHash(encryptStr, hash)) {
                        return new PayUniApiResponse("HASH_INVALID", "簽章驗證失敗", null);
                    }
                    decryptedData = cryptoService.decrypt(encryptStr);
                    log.info("PayUni 解密後資料: {}", decryptedData);
                }
            }

            // 物流 API 外層無 Message，從解密資料補充
            if (message == null && decryptedData != null && decryptedData.containsKey("Message")) {
                message = decryptedData.get("Message");
            }

            return new PayUniApiResponse(status, message, decryptedData);

        } catch (Exception e) {
            log.error("解析 PayUni 回應失敗", e);
            return new PayUniApiResponse("PARSE_ERROR", e.getMessage(), null);
        }
    }
}
