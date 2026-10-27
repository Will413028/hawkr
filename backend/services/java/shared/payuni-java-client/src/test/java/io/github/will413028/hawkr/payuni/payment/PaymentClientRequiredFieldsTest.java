package io.github.will413028.hawkr.payuni.payment;

import io.github.will413028.hawkr.payuni.crypto.PayUniCryptoService;
import io.github.will413028.hawkr.payuni.http.PayUniApiResponse;
import io.github.will413028.hawkr.payuni.http.PayUniHttpClient;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.DisplayName;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.api.extension.ExtendWith;
import org.mockito.ArgumentCaptor;
import org.mockito.Mock;
import org.mockito.junit.jupiter.MockitoExtension;

import java.util.Map;

import static org.junit.jupiter.api.Assertions.*;
import static org.mockito.ArgumentMatchers.*;
import static org.mockito.Mockito.*;

/**
 * PaymentClient 必填欄位注入測試
 *
 * 對應審計報告 §3 B-shared-1：
 * shared.PaymentClient.queryTransaction 之前漏塞 MerID + Timestamp 兩個必填欄位
 * 導致 PayUni 回 "未有商店代號"。
 *
 * 此測試鎖死所有需要 MerID + Timestamp 的 PaymentClient method，
 * 任何未來修改如果漏塞必填欄位會立刻 fail，避免回歸。
 *
 * 依 PayUni 文件，以下 method 的 EncryptInfo 內層必填 MerID + Timestamp：
 * - queryTransaction
 * - queryTransactionByProviderTradeNo
 * - refund
 * - cancelAuthorization
 * - createAtmPayment / createApplePayPayment / createGooglePayPayment / createCreditCardPayment
 */
@ExtendWith(MockitoExtension.class)
class PaymentClientRequiredFieldsTest {

    @Mock private PayUniHttpClient httpClient;

    private PaymentClient client;
    private static final String MER_ID = "HAWK9";
    private static final String TRADE_NO = "TRD_001";
    private static final String PROVIDER_TRADE_NO = "UNI_99999";

    @BeforeEach
    void setUp() {
        client = new PaymentClient(
                httpClient,
                "https://sandbox-api.payuni.com.tw",
                "https://example.com/notify",
                "https://example.com/return"
        );
        // 預設 callApi / callAgentApi 回成功（lenient：部分 test 不走任一條路徑）
        lenient().when(httpClient.callApi(anyString(), anyString(), anyMap(), anyString()))
                .thenReturn(new PayUniApiResponse("SUCCESS", "ok", null));
        lenient().when(httpClient.callAgentApi(anyString(), anyString(), anyMap(), anyString()))
                .thenReturn(new PayUniApiResponse("SUCCESS", "ok", null));
    }

    @SuppressWarnings("unchecked")
    private Map<String, String> captureParams() {
        ArgumentCaptor<Map<String, String>> cap = ArgumentCaptor.forClass(Map.class);
        verify(httpClient).callApi(eq(MER_ID), anyString(), cap.capture(), anyString());
        return cap.getValue();
    }

    private void assertHasRequiredCommonFields(Map<String, String> params) {
        assertEquals(MER_ID, params.get("MerID"), "EncryptInfo 必填 MerID");
        assertNotNull(params.get("Timestamp"), "EncryptInfo 必填 Timestamp");
        assertFalse(params.get("Timestamp").isEmpty(), "Timestamp 不可為空");
        // Timestamp 應該是合理的 unix 秒數（10 位以上）
        assertTrue(params.get("Timestamp").length() >= 10);
    }

    @Test
    @DisplayName("queryTransaction 必填 MerID + Timestamp + MerTradeNo")
    void queryTransaction_requiredFields() {
        client.queryTransaction(MER_ID, TRADE_NO);
        Map<String, String> params = captureParams();
        assertHasRequiredCommonFields(params);
        assertEquals(TRADE_NO, params.get("MerTradeNo"));
    }

    @Test
    @DisplayName("queryTransactionByProviderTradeNo 必填 MerID + Timestamp + TradeNo")
    void queryByProviderTradeNo_requiredFields() {
        client.queryTransactionByProviderTradeNo(MER_ID, PROVIDER_TRADE_NO);
        Map<String, String> params = captureParams();
        assertHasRequiredCommonFields(params);
        assertEquals(PROVIDER_TRADE_NO, params.get("TradeNo"));
    }

    @Test
    @DisplayName("refund 必填 MerID + Timestamp + TradeNo + CloseType + TradeAmt")
    void refund_requiredFields() {
        client.refund(MER_ID, PROVIDER_TRADE_NO, "RFD_001", 500L);
        Map<String, String> params = captureParams();
        assertHasRequiredCommonFields(params);
        assertEquals(PROVIDER_TRADE_NO, params.get("TradeNo"));
        assertEquals("2", params.get("CloseType"));
        assertEquals("500", params.get("TradeAmt"));
    }

    @Test
    @DisplayName("cancelAuthorization 必填 MerID + Timestamp + TradeNo")
    void cancelAuthorization_requiredFields() {
        client.cancelAuthorization(MER_ID, PROVIDER_TRADE_NO);
        Map<String, String> params = captureParams();
        assertHasRequiredCommonFields(params);
        assertEquals(PROVIDER_TRADE_NO, params.get("TradeNo"));
    }

    @Test
    @DisplayName("createAtmPayment 必填 MerID + Timestamp + GrantExport=1")
    void createAtmPayment_requiredFields() {
        client.createAtmPayment(MER_ID, TRADE_NO, 500L, "商品", "test@example.com", 3, "822");
        Map<String, String> params = captureParams();
        assertHasRequiredCommonFields(params);
        assertEquals("1", params.get("GrantExport"), "ATM 建單必帶 GrantExport=1（設計書 05）");
    }

    @Test
    @DisplayName("createApplePayPayment 必填 MerID + Timestamp + GrantExport=1")
    void createApplePayPayment_requiredFields() {
        client.createApplePayPayment(MER_ID, TRADE_NO, 500L, "商品", "test@example.com", "token", PaymentClient.MODE_IN_APP);
        Map<String, String> params = captureParams();
        assertHasRequiredCommonFields(params);
        assertEquals("1", params.get("GrantExport"), "Apple Pay 建單必帶 GrantExport=1（設計書 05）");
    }

    @Test
    @DisplayName("createApplePayPayment mode=1 (OnWeb)：AppleObject = Base64(token)；AppleMode=1")
    void createApplePayPayment_onWebMode() {
        String rawToken = "{\"data\":\"abc\",\"signature\":\"sig\",\"header\":{},\"version\":\"EC_v1\"}";
        client.createApplePayPayment(MER_ID, TRADE_NO, 500L, "商品", "test@example.com",
                rawToken, PaymentClient.MODE_ON_WEB);
        Map<String, String> params = captureParams();
        String expectedBase64 = java.util.Base64.getEncoder()
                .encodeToString(rawToken.getBytes(java.nio.charset.StandardCharsets.UTF_8));
        assertEquals(expectedBase64, params.get("AppleObject"),
                "OnWeb 模式 AppleObject 應為 Base64(paymentData JSON)（2026-06-06 實證確認，與 InApp 統一）");
        assertEquals("1", params.get("AppleMode"), "OnWeb 模式 AppleMode 應為 1");
    }

    @Test
    @DisplayName("createApplePayPayment mode=2 (InApp)：AppleObject = Base64(token)；AppleMode=2")
    void createApplePayPayment_inAppMode() {
        String rawToken = "{\"data\":\"abc\",\"signature\":\"sig\",\"header\":{},\"version\":\"EC_v1\"}";
        client.createApplePayPayment(MER_ID, TRADE_NO, 500L, "商品", "test@example.com",
                rawToken, PaymentClient.MODE_IN_APP);
        Map<String, String> params = captureParams();
        String expectedBase64 = java.util.Base64.getEncoder()
                .encodeToString(rawToken.getBytes(java.nio.charset.StandardCharsets.UTF_8));
        assertEquals(expectedBase64, params.get("AppleObject"), "InApp 模式 AppleObject 應 base64 編碼");
        assertEquals("2", params.get("AppleMode"), "InApp 模式 AppleMode 應為 2");
    }

    @Test
    @DisplayName("createApplePayPayment mode 不合法應拋 IllegalArgumentException")
    void createApplePayPayment_invalidMode() {
        assertThrows(IllegalArgumentException.class, () ->
                client.createApplePayPayment(MER_ID, TRADE_NO, 500L, "商品", "test@example.com", "token", "3"));
        assertThrows(IllegalArgumentException.class, () ->
                client.createApplePayPayment(MER_ID, TRADE_NO, 500L, "商品", "test@example.com", "token", null));
        assertThrows(IllegalArgumentException.class, () ->
                client.createApplePayPayment(MER_ID, TRADE_NO, 500L, "商品", "test@example.com", "token", ""));
    }

    @Test
    @DisplayName("createGooglePayPayment 必填 MerID + Timestamp + GrantExport=1")
    void createGooglePayPayment_requiredFields() {
        client.createGooglePayPayment(MER_ID, TRADE_NO, 500L, "商品", "test@example.com", "token", PaymentClient.MODE_IN_APP);
        Map<String, String> params = captureParams();
        assertHasRequiredCommonFields(params);
        assertEquals("1", params.get("GrantExport"), "Google Pay 建單必帶 GrantExport=1（設計書 05）");
    }

    @Test
    @DisplayName("createGooglePayPayment mode=1 (OnWeb)：GoogleObject Base64Encode；GoogleMode=1")
    void createGooglePayPayment_onWebMode() {
        String rawToken = "{\"protocolVersion\":\"ECv2\"}";
        client.createGooglePayPayment(MER_ID, TRADE_NO, 500L, "商品", "test@example.com",
                rawToken, PaymentClient.MODE_ON_WEB);
        Map<String, String> params = captureParams();
        String expectedBase64 = java.util.Base64.getEncoder()
                .encodeToString(rawToken.getBytes(java.nio.charset.StandardCharsets.UTF_8));
        assertEquals(expectedBase64, params.get("GoogleObject"), "Google Pay 統一 base64");
        assertEquals("1", params.get("GoogleMode"), "OnWeb 模式 GoogleMode 應為 1");
    }

    @Test
    @DisplayName("createGooglePayPayment mode=2 (InApp)：GoogleObject Base64Encode；GoogleMode=2")
    void createGooglePayPayment_inAppMode() {
        String rawToken = "{\"protocolVersion\":\"ECv2\"}";
        client.createGooglePayPayment(MER_ID, TRADE_NO, 500L, "商品", "test@example.com",
                rawToken, PaymentClient.MODE_IN_APP);
        Map<String, String> params = captureParams();
        String expectedBase64 = java.util.Base64.getEncoder()
                .encodeToString(rawToken.getBytes(java.nio.charset.StandardCharsets.UTF_8));
        assertEquals(expectedBase64, params.get("GoogleObject"), "Google Pay 統一 base64");
        assertEquals("2", params.get("GoogleMode"), "InApp 模式 GoogleMode 應為 2");
    }

    @Test
    @DisplayName("createGooglePayPayment mode 不合法應拋 IllegalArgumentException")
    void createGooglePayPayment_invalidMode() {
        assertThrows(IllegalArgumentException.class, () ->
                client.createGooglePayPayment(MER_ID, TRADE_NO, 500L, "商品", "test@example.com", "token", "x"));
        assertThrows(IllegalArgumentException.class, () ->
                client.createGooglePayPayment(MER_ID, TRADE_NO, 500L, "商品", "test@example.com", "token", null));
    }

    @Test
    @DisplayName("呼叫前 merId 為 null 應拋例外（代理商模式禁止 fallback）")
    void requireMerId_nullThrows() {
        assertThrows(IllegalArgumentException.class,
                () -> client.queryTransaction(null, TRADE_NO));
        assertThrows(IllegalArgumentException.class,
                () -> client.queryTransaction("", TRADE_NO));
    }

    @Test
    @DisplayName("queryTransaction 必須使用 PATH_QUERY 與 VERSION_QUERY 常數（鎖死路徑與版本）")
    void queryTransaction_pathAndVersion() {
        client.queryTransaction(MER_ID, TRADE_NO);
        verify(httpClient).callApi(
                eq(MER_ID),
                eq(PayUniPaymentEndpoint.PATH_QUERY),
                anyMap(),
                eq(PayUniPaymentEndpoint.VERSION_QUERY)
        );
    }

    @Test
    @DisplayName("PATH_QUERY 必須是 /api/trade/query，VERSION_QUERY 必須是 2.0（PayUni §2.2）")
    void payUniEndpointConstants_lockedDown() {
        assertEquals("/api/trade/query", PayUniPaymentEndpoint.PATH_QUERY);
        assertEquals("2.0", PayUniPaymentEndpoint.VERSION_QUERY);
    }

    @Test
    @DisplayName("grant 走 callAgentApi（不是 callApi），外層用 AgentID")
    void grant_usesAgentApi() {
        client.grant("HAWK", MER_ID, PROVIDER_TRADE_NO, 500L);

        // 不應呼叫 callApi
        verify(httpClient, never()).callApi(anyString(), anyString(), anyMap(), anyString());
        // 應呼叫 callAgentApi
        ArgumentCaptor<Map<String, String>> cap = ArgumentCaptor.forClass(Map.class);
        verify(httpClient).callAgentApi(eq("HAWK"), eq(PayUniPaymentEndpoint.PATH_GRANT),
                cap.capture(), eq(PayUniPaymentEndpoint.VERSION_GRANT));

        Map<String, String> params = cap.getValue();
        assertEquals(MER_ID, params.get("MerID"));
        assertEquals(PROVIDER_TRADE_NO, params.get("TradeNo"));
        assertEquals("500", params.get("Amt"));
    }

    // ---- 設計書 05：建單一律帶 GrantExport=1（撥款改由平台指示控管） ----

    /** 信用卡建單走 UppFormData（crypto.encrypt）而非 callApi，需從 encrypt 捕捉 params。 */
    @SuppressWarnings("unchecked")
    private Map<String, String> captureCreditCardParams() {
        PayUniCryptoService crypto = mock(PayUniCryptoService.class);
        when(httpClient.getCryptoService()).thenReturn(crypto);
        when(crypto.encrypt(anyMap())).thenReturn("ENC");
        when(crypto.calculateHash(anyString())).thenReturn("HASH");
        client.createCreditCardPayment(MER_ID, TRADE_NO, 500L, "商品", "acct1");
        ArgumentCaptor<Map<String, String>> cap = ArgumentCaptor.forClass(Map.class);
        verify(crypto).encrypt(cap.capture());
        return cap.getValue();
    }

    @SuppressWarnings("unchecked")
    private Map<String, String> captureAgentParams() {
        ArgumentCaptor<Map<String, String>> cap = ArgumentCaptor.forClass(Map.class);
        verify(httpClient).callAgentApi(anyString(), anyString(), cap.capture(), anyString());
        return cap.getValue();
    }

    @Test
    @DisplayName("createCreditCardPayment 建單必帶 GrantExport=1（設計書 05）")
    void createCreditCardPayment_hasGrantExport() {
        Map<String, String> params = captureCreditCardParams();
        assertHasRequiredCommonFields(params);
        assertEquals("1", params.get("GrantExport"), "信用卡建單必帶 GrantExport=1（設計書 05）");
    }

    @Test
    @DisplayName("refund 不得帶 GrantExport（非建單，帶了語意錯）")
    void refund_noGrantExport() {
        client.refund(MER_ID, PROVIDER_TRADE_NO, "RFD_001", 500L);
        assertNull(captureParams().get("GrantExport"), "refund 不得帶 GrantExport");
    }

    @Test
    @DisplayName("queryTransaction 不得帶 GrantExport（非建單）")
    void queryTransaction_noGrantExport() {
        client.queryTransaction(MER_ID, TRADE_NO);
        assertNull(captureParams().get("GrantExport"), "queryTransaction 不得帶 GrantExport");
    }

    @Test
    @DisplayName("grantQuery 不得帶 GrantExport（非建單）")
    void grantQuery_noGrantExport() {
        client.grantQuery(MER_ID, "2026-07-08", 1);
        assertNull(captureParams().get("GrantExport"), "grantQuery 不得帶 GrantExport");
    }

    @Test
    @DisplayName("grant 不得帶 GrantExport（撥款指示，非建單）")
    void grant_noGrantExport() {
        client.grant("HAWK", MER_ID, PROVIDER_TRADE_NO, 500L);
        assertNull(captureAgentParams().get("GrantExport"), "grant 不得帶 GrantExport");
    }
}
