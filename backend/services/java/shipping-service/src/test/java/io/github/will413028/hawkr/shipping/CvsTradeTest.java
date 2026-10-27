package io.github.will413028.hawkr.shipping;

import io.github.will413028.hawkr.payuni.crypto.PayUniCryptoService;
import io.github.will413028.hawkr.payuni.http.PayUniHttpClient;
import io.github.will413028.hawkr.payuni.http.PayUniApiResponse;

import java.util.LinkedHashMap;
import java.util.Map;

/**
 * 超商物流建單 API 快速測試（C2C 店到店）
 * <p>
 * 建立 7-ELEVEN C2C 物流單，取得 ShipTradeNo。
 * 前提：已透過 CvsSettingTest 成功開通 C2C。
 */
public class CvsTradeTest {

    // ===== 可替換變數 =====
    private static final String MER_ID = "HAWK9";
    // 門市代碼（透過 ship_map 取得，或直接指定已知門市）
    private static final String STORE_ID = "991182";

    // ===== PayUni 代理商金鑰（sandbox） =====
    private static final String HASH_KEY = "TestOnlyFakeHashKey0123456789AB1";
    private static final String HASH_IV = "TestOnlyFakeIV00";
    private static final String BASE_URL = "https://sandbox-api.payuni.com.tw";
    private static final String API_PATH = "/api/logistics/trade";

    public static void main(String[] args) {
        PayUniCryptoService cryptoService = new PayUniCryptoService(HASH_KEY, HASH_IV);
        PayUniHttpClient httpClient = new PayUniHttpClient(cryptoService, BASE_URL);

        String merTradeNo = "CVS-TEST-" + System.currentTimeMillis() / 1000;

        Map<String, String> params = new LinkedHashMap<>();
        params.put("MerID", MER_ID);
        params.put("Timestamp", String.valueOf(System.currentTimeMillis() / 1000));
        params.put("MerTradeNo", merTradeNo);
        params.put("GoodsType", "1");
        params.put("LgsType", "C2C");
        params.put("ShipType", "1");
        params.put("TradeAmt", "500");
        params.put("ServiceType", "3");
        params.put("StoreID", STORE_ID);
        params.put("Consignee", "測試買家");
        params.put("ConsigneeMobile", "0911222333");
        params.put("ProdDesc", "Hawkr 測試商品");

        System.out.println("===== 超商物流建單 API 測試（C2C） =====");
        System.out.println("MerID: " + MER_ID);
        System.out.println("MerTradeNo: " + merTradeNo);
        System.out.println("StoreID: " + STORE_ID);
        System.out.println("API: " + BASE_URL + API_PATH);
        System.out.println("參數: " + params);
        System.out.println();

        try {
            PayUniApiResponse response = httpClient.callApi(MER_ID, API_PATH, params, "1.3");

            System.out.println("===== 回應結果 =====");
            System.out.println("Status: " + response.status());
            System.out.println("Message: " + response.message());

            if (response.decryptedData() != null) {
                System.out.println("解密資料:");
                response.decryptedData().forEach((k, v) ->
                        System.out.println("  " + k + " = " + v));
            }

            System.out.println(response.isSuccess() ? "\n建單成功！" : "\n建單失敗，請檢查回應。");

        } catch (Exception e) {
            System.err.println("呼叫失敗: " + e.getMessage());
            e.printStackTrace();
        }
    }
}
