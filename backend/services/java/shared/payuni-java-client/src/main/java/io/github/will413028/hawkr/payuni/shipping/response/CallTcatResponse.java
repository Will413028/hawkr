package io.github.will413028.hawkr.payuni.shipping.response;

import io.github.will413028.hawkr.payuni.http.PayUniApiResponse;

/**
 * 呼叫黑貓預約取件回應
 *
 * @param apiResponse 原始 API 回應
 * @param callTime    呼叫時間
 */
public record CallTcatResponse(
        PayUniApiResponse apiResponse,
        String callTime
) {

    public static CallTcatResponse from(PayUniApiResponse resp) {
        return new CallTcatResponse(
                resp,
                resp.getData("CallTime")
        );
    }

    public boolean isSuccess() {
        return apiResponse.isSuccess();
    }
}
