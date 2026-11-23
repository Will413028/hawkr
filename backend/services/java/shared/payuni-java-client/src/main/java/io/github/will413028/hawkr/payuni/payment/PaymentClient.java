package io.github.will413028.hawkr.payuni.payment;

import io.github.will413028.hawkr.payuni.crypto.PayUniCryptoService;
import io.github.will413028.hawkr.payuni.http.PayUniApiResponse;
import io.github.will413028.hawkr.payuni.http.PayUniHttpClient;
import io.github.will413028.hawkr.payuni.payment.model.UppFormData;

import java.nio.charset.StandardCharsets;
import java.time.LocalDate;
import java.time.ZoneId;
import java.time.format.DateTimeFormatter;
import java.util.Base64;
import java.util.HashMap;
import java.util.Map;

/**
 * PayUni 金流 API 客戶端（代理商模式，完整 7 方法）
 * <p>
 * 從 payment-service PayUniClient 完整搬出，保留所有業務邏輯。
 * <p>
 * 代理商模式說明：
 * - 使用代理商的 Key/IV 進行加解密
 * - MerID 傳子商家的商家編號
 * - 所有請求加上 IsPlatForm=1 參數
 */
public class PaymentClient {

    /** PayUni 系統時區（台灣），ATM ExpireDate 必須以此計算避免 UTC 容器跨日邊界 */
    private static final ZoneId PAYUNI_ZONE = ZoneId.of("Asia/Taipei");
    private static final DateTimeFormatter PAYUNI_DATE_FMT = DateTimeFormatter.ofPattern("yyyy-MM-dd");

    private final PayUniHttpClient httpClient;
    private final String baseUrl;
    private final String notifyUrl;
    private final String returnUrl;

    public PaymentClient(PayUniHttpClient httpClient, String baseUrl, String notifyUrl, String returnUrl) {
        this.httpClient = httpClient;
        this.baseUrl = baseUrl;
        this.notifyUrl = notifyUrl;
        this.returnUrl = returnUrl;
    }

    /**
     * 建立信用卡支付（UPP 整合支付頁面）
     * <p>
     * 前端收到 UppFormData 後，需建立 hidden form POST 至 PAYUNi。
     */
    public UppFormData createCreditCardPayment(String merId, String tradeNo, long amount,
                                                String productName, String buyerId) {
        String effectiveMerId = requireMerId(merId);
        Map<String, String> params = buildBaseParams(effectiveMerId, tradeNo, amount, productName, buyerId);
        params.put("Credit", "1");
        params.put("TradeLExpireSec", "600");  // §2A.4：信用卡頁面 600 秒過期，與我方 expire_at 同步

        PayUniCryptoService crypto = httpClient.getCryptoService();
        String encryptStr = crypto.encrypt(params);
        String hashStr = crypto.calculateHash(encryptStr);

        return UppFormData.forPlatform(baseUrl, effectiveMerId, encryptStr, hashStr);
    }

    /** Apple Pay / Google Pay mode：OnWeb（網頁瀏覽器） */
    public static final String MODE_ON_WEB = "1";
    /** Apple Pay / Google Pay mode：InApp（行動原生 App） */
    public static final String MODE_IN_APP = "2";

    /**
     * 建立 Apple Pay 支付。
     * <p>
     * 依 PayUni §4.6 + 2026-06-06 Sandbox 實證確認：
     * - OnWeb (mode=1) 與 InApp (mode=2) AppleObject 均為 Base64Encode(paymentData JSON)
     * - mode 參數僅作為 PayUni 端 AppleMode 來源標記，不影響編碼方式
     * <p>
     * caller 永遠傳「原始 paymentData JSON 字串」（不含 paymentMethod /
     * transactionIdentifier），base64 由 client 統一處理。
     * <p>
     * 詳見 11 號設計書 §7.4.1.1 文件文字與實證的差異說明。
     */
    public PayUniApiResponse createApplePayPayment(String merId, String tradeNo, long amount,
                                                    String productName, String userEmail,
                                                    String applePayToken, String mode) {
        String effectiveMerId = requireMerId(merId);
        String normalizedMode = requireMode(mode);
        String appleObject = Base64.getEncoder()
                .encodeToString(applePayToken.getBytes(StandardCharsets.UTF_8));

        Map<String, String> params = new HashMap<>();
        params.put("MerID", effectiveMerId);
        params.put("MerTradeNo", tradeNo);
        params.put("TradeAmt", String.valueOf(amount));
        params.put("Timestamp", String.valueOf(System.currentTimeMillis() / 1000));
        params.put("AppleObject", appleObject);
        params.put("AppleMode", normalizedMode);
        params.put("NotifyURL", notifyUrl);
        params.put("ProdDesc", productName != null ? productName : "商品");
        params.put("GrantExport", "1");  // 設計書 05：建單一律帶 GrantExport=1，撥款改由平台指示控管
        putIfNotNull(params, "UsrMail", userEmail);

        return httpClient.callApi(effectiveMerId, PayUniPaymentEndpoint.PATH_APPLE_PAY,
                params, PayUniPaymentEndpoint.VERSION_APPLE_PAY);
    }

    /**
     * 建立 Google Pay 支付。
     * <p>
     * 依 PayUni §5.6：mode=1 / mode=2 均要求 GoogleObject = Base64Encode(token)，
     * 故 client 一律 base64。mode 僅作為來源標記（GoogleMode 參數）。
     * <p>
     * caller 永遠傳「原始 token 字串」。
     */
    public PayUniApiResponse createGooglePayPayment(String merId, String tradeNo, long amount,
                                                     String productName, String userEmail,
                                                     String googlePayToken, String mode) {
        String effectiveMerId = requireMerId(merId);
        String normalizedMode = requireMode(mode);
        String googleObject = Base64.getEncoder().encodeToString(googlePayToken.getBytes(StandardCharsets.UTF_8));

        Map<String, String> params = new HashMap<>();
        params.put("MerID", effectiveMerId);
        params.put("MerTradeNo", tradeNo);
        params.put("TradeAmt", String.valueOf(amount));
        params.put("Timestamp", String.valueOf(System.currentTimeMillis() / 1000));
        params.put("GoogleObject", googleObject);
        params.put("GoogleMode", normalizedMode);
        params.put("NotifyURL", notifyUrl);
        params.put("ProdDesc", productName != null ? productName : "商品");
        params.put("GrantExport", "1");  // 設計書 05：建單一律帶 GrantExport=1，撥款改由平台指示控管
        putIfNotNull(params, "UsrMail", userEmail);

        return httpClient.callApi(effectiveMerId, PayUniPaymentEndpoint.PATH_GOOGLE_PAY,
                params, PayUniPaymentEndpoint.VERSION_GOOGLE_PAY);
    }

    private static String requireMode(String mode) {
        if (MODE_ON_WEB.equals(mode) || MODE_IN_APP.equals(mode)) {
            return mode;
        }
        throw new IllegalArgumentException("mode 必須為 \"1\" (OnWeb) 或 \"2\" (InApp)，實際: " + mode);
    }

    /**
     * 建立 ATM 虛擬帳號
     */
    public PayUniApiResponse createAtmPayment(String merId, String tradeNo, long amount,
                                               String productName, String userEmail, int expireDays, String bankType) {
        String effectiveMerId = requireMerId(merId);

        Map<String, String> params = new HashMap<>();
        params.put("MerID", effectiveMerId);
        params.put("MerTradeNo", tradeNo);
        params.put("TradeAmt", String.valueOf(amount));
        params.put("Timestamp", String.valueOf(System.currentTimeMillis() / 1000));
        params.put("ProdDesc", productName != null ? productName : "商品");
        params.put("PaySet", "1");
        params.put("NotifyURL", notifyUrl);
        params.put("GrantExport", "1");  // 設計書 05：建單一律帶 GrantExport=1，撥款改由平台指示控管
        putIfNotNull(params, "BankType", bankType);

        // §10.4 D1 修正：必須以台北時區計算，避免 UTC 容器跨日邊界錯一天
        LocalDate expireDate = LocalDate.now(PAYUNI_ZONE).plusDays(expireDays);
        params.put("ExpireDate", expireDate.format(PAYUNI_DATE_FMT));
        putIfNotNull(params, "UsrMail", userEmail);

        return httpClient.callApi(effectiveMerId, PayUniPaymentEndpoint.PATH_ATM,
                params, PayUniPaymentEndpoint.VERSION_ATM);
    }

    /**
     * 查詢交易狀態（依 MerTradeNo / 商店訂單編號）
     *
     * 依 PayUni 逆向金流 API §2.3，EncryptInfo 內層必填：
     * - MerID
     * - MerTradeNo（與 TradeNo 擇一）
     * - Timestamp
     */
    public PayUniApiResponse queryTransaction(String merId, String tradeNo) {
        String effectiveMerId = requireMerId(merId);

        Map<String, String> params = new HashMap<>();
        params.put("MerID", effectiveMerId);
        params.put("MerTradeNo", tradeNo);
        params.put("Timestamp", String.valueOf(System.currentTimeMillis() / 1000));

        return httpClient.callApi(effectiveMerId, PayUniPaymentEndpoint.PATH_QUERY,
                params, PayUniPaymentEndpoint.VERSION_QUERY);
    }

    /**
     * 查詢交易狀態（依 PayUni UNi 序號 / TradeNo）
     *
     * 用於退款前查詢信用卡 CloseStatus（請款狀態），決定走取消授權或退款。
     * PayUni 交易查詢 API v2.0 回傳 Result 陣列，信用卡交易包含 CloseStatus 欄位。
     *
     * 依 PayUni 逆向金流 API §2.3，EncryptInfo 內層必填：MerID、TradeNo、Timestamp
     */
    public PayUniApiResponse queryTransactionByProviderTradeNo(String merId, String providerTradeNo) {
        String effectiveMerId = requireMerId(merId);

        Map<String, String> params = new HashMap<>();
        params.put("MerID", effectiveMerId);
        params.put("TradeNo", providerTradeNo);
        params.put("Timestamp", String.valueOf(System.currentTimeMillis() / 1000));

        return httpClient.callApi(effectiveMerId, PayUniPaymentEndpoint.PATH_QUERY,
                params, PayUniPaymentEndpoint.VERSION_QUERY);
    }

    /**
     * 申請退款（信用卡/Apple Pay/Google Pay）
     *
     * 依照 PayUni 文件 /api/trade/close Ver 1.0：
     * - EncryptInfo 內層必填：MerID、TradeNo（UNi序號）、Timestamp、CloseType
     * - EncryptInfo 內層選填：TradeAmt（部分退款時必填）
     *
     * @param merId           子商家 MerID
     * @param providerTradeNo PayUni UNi序號（payment_transactions.provider_trade_no）
     * @param refundNo        退款編號（保留供日誌追蹤）
     * @param amount          退款金額（新台幣元）
     */
    public PayUniApiResponse refund(String merId, String providerTradeNo, String refundNo, long amount) {
        String effectiveMerId = requireMerId(merId);

        Map<String, String> params = new HashMap<>();
        params.put("MerID", effectiveMerId);                                    // 必填：商店代號
        params.put("TradeNo", providerTradeNo);                                 // 必填：PayUni UNi序號
        params.put("Timestamp", String.valueOf(System.currentTimeMillis() / 1000)); // 必填：時間戳記
        params.put("CloseType", "2");                                           // 必填：退款
        params.put("TradeAmt", String.valueOf(amount));                         // 選填：退款金額

        return httpClient.callApi(effectiveMerId, PayUniPaymentEndpoint.PATH_TRADE_CLOSE,
                params, PayUniPaymentEndpoint.VERSION_TRADE_CLOSE);
    }

    /**
     * 取消信用卡授權（已授權未請款）
     * <p>
     * 依照 PayUni 逆向金流 API §3：/api/trade/cancel Ver 1.0
     * EncryptInfo 內層必填：MerID、TradeNo（UNi序號）、Timestamp
     * 不需要金額參數（整筆授權取消）。
     *
     * @param merId           子商家 MerID
     * @param providerTradeNo PayUni UNi序號（payment_transactions.provider_trade_no）
     */
    public PayUniApiResponse cancelAuthorization(String merId, String providerTradeNo) {
        String effectiveMerId = requireMerId(merId);

        Map<String, String> params = new HashMap<>();
        params.put("MerID", effectiveMerId);
        params.put("TradeNo", providerTradeNo);
        params.put("Timestamp", String.valueOf(System.currentTimeMillis() / 1000));

        return httpClient.callApi(effectiveMerId, PayUniPaymentEndpoint.PATH_TRADE_CANCEL,
                params, PayUniPaymentEndpoint.VERSION_TRADE_CANCEL);
    }

    /**
     * ATM 退款轉匯（非信用卡退款）
     * <p>
     * PayUni 非即時處理：API 回應為「申請受理」，隔日扣款 + 匯款（2~3 工作天到帳）。
     * 退款結果透過 Notify 至 NotifyURL 回傳（RefundStatus: 1=成功, 2=失敗）。
     *
     * @param merId         商店代號（子商家 MerID）
     * @param providerTradeNo PayUni UNi 序號（provider_trade_no）
     * @param amount        退款金額（新台幣元）
     * @param bankCode      銀行代碼（3碼）
     * @param bankSubCode   分行代碼（4碼）
     * @param bankAccountNo 銀行帳戶號碼
     * @param uid           身分證字號或統編（需與銀行帳號持有人相同）
     */
    public PayUniApiResponse offlineRefund(String merId, String providerTradeNo, long amount,
                                            String bankCode, String bankSubCode,
                                            String bankAccountNo, String uid) {
        String effectiveMerId = requireMerId(merId);

        Map<String, String> params = new HashMap<>();
        params.put("TradeNo", providerTradeNo);
        params.put("Timestamp", String.valueOf(System.currentTimeMillis() / 1000));
        params.put("PaymentType", "2");
        params.put("RefundAmt", String.valueOf(amount));
        params.put("BankCode", bankCode);
        params.put("BankSubCode", bankSubCode);
        params.put("BankAccountNo", bankAccountNo);
        params.put("UID", uid);

        return httpClient.callApi(effectiveMerId, PayUniPaymentEndpoint.PATH_OFFLINE_REFUND,
                params, PayUniPaymentEndpoint.VERSION_OFFLINE_REFUND);
    }

    /**
     * ATM 退款轉匯（頁面版）— 由買家自行填寫銀行資訊
     * <p>
     * 與 {@link #offlineRefund} 的差異：API 版需後端提供銀行資訊，頁面版由 PayUni 頁面收集。
     * 前端收到 UppFormData 後，建立 hidden form POST 至 PayUni 頁面。
     * 買家填寫完成後，PayUni 導回 ReturnURL；退款結果透過 Webhook 通知。
     *
     * @param merId           子商家 MerID
     * @param providerTradeNo PayUni UNi 序號
     * @param amount          退款金額（新台幣元）
     * @param returnUrl       完成後返回 URL（PayUni 頁面操作完成後導向此 URL）
     */
    public UppFormData offlineRefundPage(String merId, String providerTradeNo, long amount, String returnUrl) {
        String effectiveMerId = requireMerId(merId);

        Map<String, String> params = new HashMap<>();
        params.put("MerID", effectiveMerId);   // §6.5 必要欄位；遺漏導致 PayUni 回 REF02001「未有商店代號」
        params.put("TradeNo", providerTradeNo);
        params.put("Timestamp", String.valueOf(System.currentTimeMillis() / 1000));
        params.put("PaymentType", "2");
        params.put("RefundAmt", String.valueOf(amount));
        putIfNotNull(params, "ReturnURL", returnUrl);

        PayUniCryptoService crypto = httpClient.getCryptoService();
        String encryptStr = crypto.encrypt(params);
        String hashStr = crypto.calculateHash(encryptStr);

        return new UppFormData(
                baseUrl + PayUniPaymentEndpoint.PATH_OFFLINE_REFUND_PAGE,
                effectiveMerId,
                PayUniPaymentEndpoint.VERSION_OFFLINE_REFUND_PAGE,
                encryptStr,
                hashStr,
                "1"
        );
    }

    /**
     * 取消 ATM 退款轉匯（僅限發動退款當日可取消，隔日則不可）
     *
     * @param merId         商店代號（子商家 MerID）
     * @param providerTradeNo PayUni UNi 序號
     */
    public PayUniApiResponse cancelOfflineRefund(String merId, String providerTradeNo) {
        String effectiveMerId = requireMerId(merId);

        Map<String, String> params = new HashMap<>();
        params.put("TradeNo", providerTradeNo);
        params.put("Timestamp", String.valueOf(System.currentTimeMillis() / 1000));

        return httpClient.callApi(effectiveMerId, PayUniPaymentEndpoint.PATH_OFFLINE_CANCEL_REFUND,
                params, PayUniPaymentEndpoint.VERSION_OFFLINE_CANCEL_REFUND);
    }

    /**
     * 發動撥款指示（代理商專用 API）
     *
     * 與其他 API 不同：
     * - 外層使用 AgentID 而非 MerID
     * - 不帶 IsPlatForm
     * - EncryptInfo 內層帶 MerID + TradeNo（PayUni UNi 序號）+ Amt（金額）
     *
     * @param agentId         代理商前綴字（4 碼大寫）
     * @param merId           子商家 MerID
     * @param providerTradeNo PayUni UNi 序號
     * @param amount          撥款金額（新台幣元）
     */
    public PayUniApiResponse grant(String agentId, String merId, String providerTradeNo, long amount) {
        String effectiveMerId = requireMerId(merId);

        Map<String, String> params = new HashMap<>();
        params.put("MerID", effectiveMerId);
        params.put("TradeNo", providerTradeNo);
        params.put("Amt", String.valueOf(amount));

        return httpClient.callAgentApi(agentId, PayUniPaymentEndpoint.PATH_GRANT,
                params, PayUniPaymentEndpoint.VERSION_GRANT);
    }

    /**
     * 撥款查詢（grant_query，代理商專用 API 總覽 §6）—— §5.6 去重硬化 B 重試前對帳。
     * <p>
     * 與撥款指示 {@link #grant} 的外層識別不同：本 API 外層使用 <b>MerID + IsPlatForm=1</b>
     * （走 {@code httpClient.callApi}，callApi 自動帶 IsPlatForm=1），而非 AgentID。
     * <p>
     * EncryptInfo 內層（§6.3）：MerID + QueryDate（{@code YYYY-MM-DD}，限查 30 天內）+ Page
     * （每頁固定 500 筆，預設第 1 頁）。回傳撥款入帳明細 {@code Result[]}（含 TradeNo / ExecuteAmt /
     * FundsAmt / Type）+ {@code PageInfo}（Current / Total）。
     *
     * @param merId     子商家 MerID
     * @param queryDate 查詢日期（{@code YYYY-MM-DD}，PayUni 台北時區日曆日）
     * @param page      頁碼（每頁 500 筆，從 1 起）
     */
    public PayUniApiResponse grantQuery(String merId, String queryDate, int page) {
        String effectiveMerId = requireMerId(merId);

        Map<String, String> params = new HashMap<>();
        params.put("MerID", effectiveMerId);
        params.put("QueryDate", queryDate);
        params.put("Page", String.valueOf(page));

        return httpClient.callApi(effectiveMerId, PayUniPaymentEndpoint.PATH_GRANT_QUERY,
                params, PayUniPaymentEndpoint.VERSION_GRANT_QUERY);
    }

    /**
     * 解析 Webhook 通知（verify + decrypt）
     *
     * @return 解密後的資料，驗證失敗返回 null
     */
    public Map<String, String> parseWebhook(String encryptInfo, String hashInfo) {
        PayUniCryptoService crypto = httpClient.getCryptoService();
        if (!crypto.verifyHash(encryptInfo, hashInfo)) {
            return null;
        }
        return crypto.decrypt(encryptInfo);
    }

    private Map<String, String> buildBaseParams(String merId, String tradeNo, long amount,
                                                 String productName, String buyerId) {
        Map<String, String> params = new HashMap<>();
        params.put("MerID", merId);
        params.put("MerTradeNo", tradeNo);
        params.put("TradeAmt", String.valueOf(amount));
        params.put("Timestamp", String.valueOf(System.currentTimeMillis() / 1000));
        params.put("ProdDesc", productName != null ? productName : "商品");
        params.put("NotifyURL", notifyUrl);
        params.put("ReturnURL", returnUrl);
        params.put("GrantExport", "1");  // 設計書 05：建單一律帶 GrantExport=1，撥款改由平台指示控管
        putIfNotNull(params, "UserId", buyerId);
        return params;
    }

    private String requireMerId(String merId) {
        if (merId == null || merId.isEmpty()) {
            throw new IllegalArgumentException("代理商模式下必須提供子商家 MerID");
        }
        return merId;
    }

    private void putIfNotNull(Map<String, String> map, String key, String value) {
        if (value != null && !value.isEmpty()) {
            map.put(key, value);
        }
    }
}
