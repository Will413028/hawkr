package io.github.will413028.hawkr.shipping.service;

import io.github.will413028.hawkr.payuni.crypto.PayUniCryptoService;
import io.github.will413028.hawkr.shipping.config.PayUniConfig;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.Test;
import org.springframework.test.util.ReflectionTestUtils;

import java.util.HashMap;
import java.util.Map;

import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.junit.jupiter.api.Assertions.assertTrue;
import static org.mockito.Mockito.mock;
import static org.mockito.Mockito.when;

/**
 * 單元測試：驗證 StoreMapService 在三種 appType（buyer / seller / web）下
 * 正確組裝 PayUni 門市選擇完成後的 redirect URL。
 * <p>
 * 對應設計書 06 §5.14.2「最終出口分支矩陣」與「錯誤分支」。
 */
class StoreMapServiceTest {

    private static final String WEB_BASE = "https://web-test.example.com";

    private PayUniCryptoService cryptoService;
    private PayUniConfig payUniConfig;
    private StoreMapService service;

    @BeforeEach
    void setUp() {
        cryptoService = mock(PayUniCryptoService.class);
        payUniConfig = mock(PayUniConfig.class);
        service = new StoreMapService(cryptoService, payUniConfig, mock(StoreMapRedirectFormStore.class));
        ReflectionTestUtils.setField(service, "webFrontendUrl", WEB_BASE);
    }

    // ==================== buildErrorDeepLink ====================

    @Test
    void buildErrorDeepLink_buyer_returnsBuyerScheme() {
        String url = service.buildErrorDeepLink("buyer", "簽章驗證失敗");
        assertTrue(url.startsWith("hawkrbuyer://store_map/result?error="),
                "buyer 應走 hawkrbuyer scheme, got: " + url);
        assertTrue(url.contains("%E7%B0%BD%E7%AB%A0"), "錯誤訊息應 URL encoded, got: " + url);
    }

    @Test
    void buildErrorDeepLink_seller_returnsSellerScheme() {
        String url = service.buildErrorDeepLink("seller", "x");
        assertEquals("hawkrseller://store_map/result?error=x", url);
    }

    @Test
    void buildErrorDeepLink_web_returnsWebFrontendUrl() {
        String url = service.buildErrorDeepLink("web", "x");
        assertEquals(WEB_BASE + "/store-map/return?error=x", url);
    }

    @Test
    void buildErrorDeepLink_web_trailingSlashStripped() {
        ReflectionTestUtils.setField(service, "webFrontendUrl", WEB_BASE + "/");
        String url = service.buildErrorDeepLink("web", "x");
        assertEquals(WEB_BASE + "/store-map/return?error=x", url,
                "尾端斜線應被剝除避免雙斜線");
    }

    @Test
    void buildErrorDeepLink_unknownAppType_fallsBackToBuyer() {
        String url = service.buildErrorDeepLink("unknown", "x");
        assertTrue(url.startsWith("hawkrbuyer://"),
                "未知 appType 兜底為 buyer scheme（與既有行為一致）, got: " + url);
    }

    // ==================== decryptStoreMapResult (含 buildDeepLink private) ====================

    private static final String FAKE_ENC = "ENCRYPTED";
    private static final String FAKE_HASH = "HASH";

    private void stubCryptoSuccess() {
        when(cryptoService.verifyHash(FAKE_ENC, FAKE_HASH)).thenReturn(true);
        Map<String, String> decrypted = new HashMap<>();
        decrypted.put("MapJson", "{\"StoreID\":\"916712\",\"StoreName\":\"敦安門市\","
                + "\"Address\":\"台北市大安區\",\"InsularArea\":\"I\",\"StoreType\":\"1\"}");
        when(cryptoService.decrypt(FAKE_ENC)).thenReturn(decrypted);
    }

    @Test
    void decryptStoreMapResult_buyer_returnsBuyerDeepLink() {
        stubCryptoSuccess();
        StoreMapService.StoreMapDecryptResult r =
                service.decryptStoreMapResult(FAKE_ENC, FAKE_HASH, "buyer");

        assertEquals("916712", r.storeId());
        assertEquals("敦安門市", r.storeName());
        assertTrue(r.redirectUrl().startsWith("hawkrbuyer://store_map/result?"),
                "buyer 應走 hawkrbuyer scheme, got: " + r.redirectUrl());
        assertTrue(r.redirectUrl().contains("storeId=916712"), r.redirectUrl());
        assertTrue(r.redirectUrl().contains("insularArea=I"), r.redirectUrl());
        assertTrue(r.redirectUrl().contains("storeType=1"), r.redirectUrl());
    }

    @Test
    void decryptStoreMapResult_seller_returnsSellerDeepLink() {
        stubCryptoSuccess();
        StoreMapService.StoreMapDecryptResult r =
                service.decryptStoreMapResult(FAKE_ENC, FAKE_HASH, "seller");

        assertTrue(r.redirectUrl().startsWith("hawkrseller://store_map/result?"),
                "seller 應走 hawkrseller scheme, got: " + r.redirectUrl());
    }

    @Test
    void decryptStoreMapResult_web_returnsWebFrontendUrl() {
        stubCryptoSuccess();
        StoreMapService.StoreMapDecryptResult r =
                service.decryptStoreMapResult(FAKE_ENC, FAKE_HASH, "web");

        assertTrue(r.redirectUrl().startsWith(WEB_BASE + "/store-map/return?"),
                "web 應走 {WEB_FRONTEND_URL}/store-map/return, got: " + r.redirectUrl());
        // 三向 query naming 對齊既有 buyer/seller deep link（設計書 §5.14.2 註記）
        assertTrue(r.redirectUrl().contains("storeId=916712"), r.redirectUrl());
        assertTrue(r.redirectUrl().contains("storeName="), r.redirectUrl());
        assertTrue(r.redirectUrl().contains("address="), r.redirectUrl());
        assertTrue(r.redirectUrl().contains("insularArea=I"), r.redirectUrl());
        assertTrue(r.redirectUrl().contains("storeType=1"), r.redirectUrl());
        // 中文必須 URL encoded
        assertTrue(r.redirectUrl().contains("%E6%95%A6%E5%AE%89"),
                "StoreName 中文應 URL encoded, got: " + r.redirectUrl());
    }
}
