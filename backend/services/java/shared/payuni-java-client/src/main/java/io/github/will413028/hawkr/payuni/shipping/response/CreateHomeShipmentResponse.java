package io.github.will413028.hawkr.payuni.shipping.response;

import io.github.will413028.hawkr.payuni.http.PayUniApiResponse;

/**
 * 宅配單建立回應
 *
 * @param apiResponse  原始 API 回應
 * @param shipTradeNo  物流交易編號
 * @param shipStatus   物流狀態碼
 */
public record CreateHomeShipmentResponse(
        PayUniApiResponse apiResponse,
        String shipTradeNo,
        String shipStatus
) {

    public static CreateHomeShipmentResponse from(PayUniApiResponse resp) {
        return new CreateHomeShipmentResponse(
                resp,
                resp.getData("ShipTradeNo"),
                resp.getData("ShipStatus")
        );
    }

    public boolean isSuccess() {
        return apiResponse.isSuccess();
    }
}
