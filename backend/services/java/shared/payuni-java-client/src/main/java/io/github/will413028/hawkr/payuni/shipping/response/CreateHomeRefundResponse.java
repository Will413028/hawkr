package io.github.will413028.hawkr.payuni.shipping.response;

import io.github.will413028.hawkr.payuni.http.PayUniApiResponse;

/**
 * 宅配退貨單建立回應
 *
 * @param apiResponse  PayUni 原始回應
 * @param shipTradeNo  UNi 物流序號
 * @param obtNumber    退貨編號（黑貓退貨追蹤用）
 * @param merTradeNo   商店訂單編號
 */
public record CreateHomeRefundResponse(
        PayUniApiResponse apiResponse,
        String shipTradeNo,
        String obtNumber,
        String merTradeNo
) {
    public boolean isSuccess() {
        return apiResponse != null && apiResponse.isSuccess();
    }

    public static CreateHomeRefundResponse from(PayUniApiResponse resp) {
        return new CreateHomeRefundResponse(
                resp,
                resp.decryptedData().getOrDefault("ShipTradeNo", ""),
                resp.decryptedData().getOrDefault("OBTNumber", ""),
                resp.decryptedData().getOrDefault("MerTradeNo", "")
        );
    }
}
