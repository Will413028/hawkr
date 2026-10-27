package io.github.will413028.hawkr.payuni.shipping.response;

import io.github.will413028.hawkr.payuni.http.PayUniApiResponse;

/**
 * 預約宅配取號回應
 * <p>
 * 含標籤下載 URL 連結（非 PDF byte[]），前端或後端可透過 URL 下載標籤。
 *
 * @param apiResponse 原始 API 回應
 * @param labelUrl    標籤下載 URL
 * @param shipTradeNo 物流交易編號
 */
public record ScheduleHomeLabelResponse(
        PayUniApiResponse apiResponse,
        String labelUrl,
        String shipTradeNo
) {

    public static ScheduleHomeLabelResponse from(PayUniApiResponse resp) {
        return new ScheduleHomeLabelResponse(
                resp,
                resp.getData("LabelUrl"),
                resp.getData("ShipTradeNo")
        );
    }

    public boolean isSuccess() {
        return apiResponse.isSuccess();
    }
}
