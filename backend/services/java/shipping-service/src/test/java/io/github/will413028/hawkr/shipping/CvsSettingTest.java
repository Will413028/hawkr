package io.github.will413028.hawkr.shipping;

import io.github.will413028.hawkr.payuni.crypto.PayUniCryptoService;
import io.github.will413028.hawkr.payuni.http.PayUniHttpClient;
import io.github.will413028.hawkr.payuni.http.PayUniApiResponse;

import java.util.LinkedHashMap;
import java.util.Map;

/**
 * 超商物流建店 API 快速測試
 * <p>
 * 開通 7-ELEVEN C2C 店到店 / B2C 大宗寄倉物流。
 */
public class CvsSettingTest {

    // ===== 可替換變數 =====
    private static final String MER_ID = "HAWK9";
    // C2C 或 B2C
    private static final String LGS_TYPE = "B2C";

    // ===== PayUni 代理商金鑰（sandbox） =====
    private static final String HASH_KEY = "TestOnlyFakeHashKey0123456789AB1";
    private static final String HASH_IV = "TestOnlyFakeIV00";
    private static final String BASE_URL = "https://sandbox-api.payuni.com.tw";
    private static final String API_PATH = "/api/logistics/setting";

    public static void main(String[] args) {
        PayUniCryptoService cryptoService = new PayUniCryptoService(HASH_KEY, HASH_IV);
        PayUniHttpClient httpClient = new PayUniHttpClient(cryptoService, BASE_URL);

        Map<String, String> params = new LinkedHashMap<>();
        params.put("MerID", MER_ID);
        params.put("Timestamp", String.valueOf(System.currentTimeMillis() / 1000));
        params.put("GoodsType", "1");
        params.put("LgsType", LGS_TYPE);
        params.put("ShipType", "1");
        params.put("LabelCompanyName", "Hawkr");

        // 共用必填欄位
        params.put("ContactName", "王小明");
        params.put("ContactTel", "0912345678");
        params.put("ContactMail", "test@example.com");

        if ("C2C".equals(LGS_TYPE)) {
            // C2C 需要退貨門市
            params.put("StoreID", "991182");
        } else {
            // B2C 專用必填
            params.put("ServiceTel", "0223456789");
            params.put("MerURL", "https://www.example.com");
            params.put("ReturnCycle", "2");
            params.put("ReturnCycleType", "1");
            params.put("ReturnAddress", "台北市中山區南京東路一段1號");
        }

        System.out.println("===== 超商物流建店 API 測試 =====");
        System.out.println("MerID: " + MER_ID);
        System.out.println("LgsType: " + LGS_TYPE);
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

            System.out.println(response.isSuccess() ? "\n開通成功！" : "\n開通失敗，請檢查回應。");

        } catch (Exception e) {
            System.err.println("呼叫失敗: " + e.getMessage());
            e.printStackTrace();
        }
    }
}
