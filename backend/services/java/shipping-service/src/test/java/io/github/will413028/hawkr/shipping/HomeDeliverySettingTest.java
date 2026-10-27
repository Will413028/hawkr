package io.github.will413028.hawkr.shipping;

import io.github.will413028.hawkr.payuni.crypto.PayUniCryptoService;
import io.github.will413028.hawkr.payuni.http.PayUniHttpClient;
import io.github.will413028.hawkr.payuni.http.PayUniApiResponse;

import java.util.LinkedHashMap;
import java.util.Map;

/**
 * 宅配建店 API 快速測試
 * <p>
 * 直接執行 main 即可呼叫 PayUni sandbox 開通黑貓宅配物流。
 * 替換 MER_ID 為目標商家代號。
 */
public class HomeDeliverySettingTest {

    // ===== 可替換變數 =====
    private static final String MER_ID = "HAWK9";

    // ===== PayUni 代理商金鑰（sandbox） =====
    private static final String HASH_KEY = "TestOnlyFakeHashKey0123456789AB1";
    private static final String HASH_IV = "TestOnlyFakeIV00";
    private static final String BASE_URL = "https://sandbox-api.payuni.com.tw";
    private static final String API_PATH = "/api/home_delivery/setting";

    public static void main(String[] args) {
        PayUniCryptoService cryptoService = new PayUniCryptoService(HASH_KEY, HASH_IV);
        PayUniHttpClient httpClient = new PayUniHttpClient(cryptoService, BASE_URL);

        // 內層加密參數
        Map<String, String> params = new LinkedHashMap<>();
        params.put("MerID", MER_ID);
        params.put("Timestamp", String.valueOf(System.currentTimeMillis() / 1000));
        params.put("LgsType", "HOME");
        params.put("ShipType", "2");
        params.put("SenderName", "Hawkr 測試商店");
        params.put("SenderMobile", "0912345678");
        params.put("SenderAddress", "台北市中山區南京東路一段1號");
        params.put("ProductTypeId", "0004");
        String notifyUrl = System.getenv().getOrDefault("PAYUNI_NOTIFY_URL", "");
        if (!notifyUrl.isEmpty()) {
            params.put("NotifyURL", notifyUrl);
        }

        System.out.println("===== 宅配建店 API 測試 =====");
        System.out.println("MerID: " + MER_ID);
        System.out.println("API: " + BASE_URL + API_PATH);
        System.out.println("參數: " + params);
        System.out.println();

        try {
            PayUniApiResponse response = httpClient.callApi(MER_ID, API_PATH, params, "1.0");

            System.out.println("===== 回應結果 =====");
            System.out.println("Status: " + response.status());
            System.out.println("Message: " + response.message());

            if (response.decryptedData() != null) {
                System.out.println("解密資料:");
                response.decryptedData().forEach((k, v) ->
                        System.out.println("  " + k + " = " + v));
            }

            if (response.isSuccess()) {
                System.out.println("\n開通成功！");
            } else {
                System.out.println("\n開通失敗，請檢查回應。");
            }

        } catch (Exception e) {
            System.err.println("呼叫失敗: " + e.getMessage());
            e.printStackTrace();
        }
    }
}
