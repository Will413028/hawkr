package io.github.will413028.hawkr.shipping;

import io.github.will413028.hawkr.payuni.crypto.PayUniCryptoService;
import io.github.will413028.hawkr.payuni.http.PayUniHttpClient;
import io.github.will413028.hawkr.payuni.shipping.ShippingClient;
import io.github.will413028.hawkr.payuni.shipping.request.CreateCvsShipmentRequest;
import io.github.will413028.hawkr.payuni.shipping.request.PrintCvsLabelRequest;
import io.github.will413028.hawkr.payuni.shipping.response.CreateCvsShipmentResponse;
import io.github.will413028.hawkr.payuni.shipping.response.PrintCvsLabelDirectResponse;

import java.time.LocalDate;
import java.time.format.DateTimeFormatter;

/**
 * 超商出貨單建立 + 列印測試（驗證 Webhook 回呼）
 *
 * 測試前提：
 * 1. ngrok 已啟動：ngrok http 9946
 * 2. webhook-service 已啟動（port 9946）
 * 3. 環境變數 PAYUNI_SHIPPING_NOTIFY_URL 設為 ngrok URL + /webhooks/payuni/notify
 *
 * 執行後檢查：
 * - ngrok dashboard (http://127.0.0.1:4040) 是否收到 PayUni POST 請求
 * - webhook-service log 是否出現 "收到 PayUni Webhook"
 */
public class CvsShipmentWebhookTest {

    private static final String MER_ID = "HAWK9";
    private static final String HASH_KEY = "TestOnlyFakeHashKey0123456789AB1";
    private static final String HASH_IV = "TestOnlyFakeIV00";
    private static final String BASE_URL = "https://sandbox-api.payuni.com.tw";

    public static void main(String[] args) throws Exception {
        String notifyUrl = System.getenv().getOrDefault("PAYUNI_SHIPPING_NOTIFY_URL", "");

        PayUniCryptoService cryptoService = new PayUniCryptoService(HASH_KEY, HASH_IV);
        PayUniHttpClient httpClient = new PayUniHttpClient(cryptoService, BASE_URL);
        ShippingClient shippingClient = new ShippingClient(httpClient,
                notifyUrl.isEmpty() ? null : notifyUrl);

        String tradeNo = "TEST-WH-" + System.currentTimeMillis();
        String shipDate = LocalDate.now().format(DateTimeFormatter.ofPattern("yyyyMMdd"));

        // ===== Step 1: 建立超商物流單 =====
        System.out.println("===== Step 1: 建立超商物流單 =====");
        System.out.println("MerID: " + MER_ID);
        System.out.println("MerTradeNo: " + tradeNo);
        System.out.println("NotifyURL: " + (notifyUrl.isEmpty() ? "(未設定)" : notifyUrl));

        CreateCvsShipmentRequest createReq = new CreateCvsShipmentRequest(
                tradeNo,
                "1",        // ShipType: 7-ELEVEN
                "C2C",      // LgsType
                "1",        // GoodsType: 常溫
                "3",        // ServiceType: 取貨不付款
                "測試寄件人",
                "0912345678",
                "測試收件人",
                "0987654321",
                "131386",   // 7-ELEVEN 門市代碼（測試用）
                "100",      // TradeAmt
                "測試商品",
                null        // RefundStoreID
        );

        CreateCvsShipmentResponse createResp = shippingClient.createCvsShipment(MER_ID, createReq);

        System.out.println("Status: " + createResp.apiResponse().status());
        System.out.println("Message: " + createResp.apiResponse().message());

        if (!createResp.isSuccess()) {
            System.err.println("建單失敗，中止測試");
            if (createResp.apiResponse().decryptedData() != null) {
                createResp.apiResponse().decryptedData().forEach((k, v) ->
                        System.out.println("  " + k + " = " + v));
            }
            return;
        }

        String shipTradeNo = createResp.shipTradeNo();
        System.out.println("ShipTradeNo: " + shipTradeNo);
        System.out.println("建單成功！\n");

        // ===== Step 2: 列印超商出貨單 =====
        System.out.println("===== Step 2: 列印超商出貨單 =====");
        System.out.println("ShipTradeNo: " + shipTradeNo);
        System.out.println("ShipDate: " + shipDate);

        try {
            PrintCvsLabelDirectResponse labelResp = shippingClient.printCvsLabel(
                    MER_ID,
                    new PrintCvsLabelRequest(shipTradeNo, "1", "C2C", "1", shipDate));

            System.out.println("標籤圖片 URL: " + labelResp.imageUrl());
            System.out.println("列印成功！");
        } catch (Exception e) {
            System.out.println("列印結果: " + e.getMessage());
            System.out.println("（sandbox 環境列印可能回傳處理中，這是正常的）");
        }

        System.out.println("\n===== 測試完成 =====");
        System.out.println("請檢查：");
        System.out.println("  1. ngrok dashboard (http://127.0.0.1:4040) 是否收到 POST 請求");
        System.out.println("  2. webhook-service log 是否出現 Webhook 通知");
    }
}
