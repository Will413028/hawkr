package io.github.will413028.hawkr.shipping;

import io.github.will413028.hawkr.payuni.crypto.PayUniCryptoService;
import io.github.will413028.hawkr.payuni.http.PayUniHttpClient;
import io.github.will413028.hawkr.payuni.http.PayUniApiResponse;

import java.util.LinkedHashMap;
import java.util.Map;

/**
 * 超商門市地圖 API 快速測試（前景 API）
 * <p>
 * 注意：此為前景導頁 API，回傳可能是 HTML 而非 JSON。
 */
public class CvsShipMapTest {

    // ===== 可替換變數 =====
    private static final String MER_ID = "HAWK9";

    // ===== PayUni 代理商金鑰（sandbox） =====
    private static final String HASH_KEY = "TestOnlyFakeHashKey0123456789AB1";
    private static final String HASH_IV = "TestOnlyFakeIV00";
    private static final String BASE_URL = "https://sandbox-api.payuni.com.tw";
    private static final String API_PATH = "/api/logistics/ship_map";

    public static void main(String[] args) {
        PayUniCryptoService cryptoService = new PayUniCryptoService(HASH_KEY, HASH_IV);
        PayUniHttpClient httpClient = new PayUniHttpClient(cryptoService, BASE_URL);

        Map<String, String> params = new LinkedHashMap<>();
        params.put("MerID", MER_ID);
        params.put("Timestamp", String.valueOf(System.currentTimeMillis() / 1000));
        params.put("MerKeyNo", "TEST-MAP-001");
        params.put("GoodsType", "1");
        params.put("LgsType", "C2C");
        params.put("ShipType", "1");
        params.put("MapType", "1");
        params.put("Tag", "2");
        params.put("MobileTag", "N");
        // 前景導回 URL（測試時由環境變數填入）
        String mapReturnUrl = System.getenv().getOrDefault("PAYUNI_MAP_RETURN_URL", "");
        if (!mapReturnUrl.isEmpty()) {
            params.put("MapReturnURL", mapReturnUrl);
        }

        System.out.println("===== 超商門市地圖 API 測試 =====");
        System.out.println("MerID: " + MER_ID);
        System.out.println("API: " + BASE_URL + API_PATH);
        System.out.println("參數: " + params);
        System.out.println("（注意：此為前景 API，可能回傳 HTML）");
        System.out.println();

        try {
            PayUniApiResponse response = httpClient.callApi(MER_ID, API_PATH, params, "1.1");

            System.out.println("===== 回應結果 =====");
            System.out.println("Status: " + response.status());
            System.out.println("Message: " + response.message());

            if (response.decryptedData() != null) {
                System.out.println("解密資料:");
                response.decryptedData().forEach((k, v) ->
                        System.out.println("  " + k + " = " + v));
            } else {
                System.out.println("（無法解密 — 可能是 HTML 回應）");
            }

        } catch (Exception e) {
            System.err.println("呼叫失敗（前景 API 預期行為）: " + e.getMessage());
            e.printStackTrace();
        }
    }
}
