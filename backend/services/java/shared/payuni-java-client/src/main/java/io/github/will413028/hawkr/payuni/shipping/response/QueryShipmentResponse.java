package io.github.will413028.hawkr.payuni.shipping.response;

import io.github.will413028.hawkr.payuni.http.PayUniApiResponse;

/**
 * 物流單查詢回應
 *
 * @param apiResponse    原始 API 回應
 * @param shipTradeNo    物流交易編號
 * @param shipStatus     物流狀態碼
 * @param shipStatusTime 狀態變更時間
 * @param merTradeNo     商店訂單編號
 */
public record QueryShipmentResponse(
        PayUniApiResponse apiResponse,
        String shipTradeNo,
        String shipStatus,
        String shipStatusTime,
        String merTradeNo
) {

    public static QueryShipmentResponse from(PayUniApiResponse resp) {
        return new QueryShipmentResponse(
                resp,
                resp.getData("ShipTradeNo"),
                resp.getData("ShipStatus"),
                resp.getData("ShipStatusTime"),
                resp.getData("MerTradeNo")
        );
    }

    public boolean isSuccess() {
        return apiResponse.isSuccess();
    }
}
