package io.github.will413028.hawkr.payuni.shipping.response;

import io.github.will413028.hawkr.payuni.http.PayUniApiResponse;

public record OnboardShippingResponse(
    PayUniApiResponse apiResponse,
    String partnerId,
    String eshipId,
    String setStatus
) {
    public boolean isSuccess() {
        return apiResponse != null && apiResponse.isSuccess();
    }

    public static OnboardShippingResponse from(PayUniApiResponse resp) {
        if (resp == null || !resp.isSuccess()) {
            return new OnboardShippingResponse(resp, null, null, null);
        }
        var data = resp.decryptedData();
        return new OnboardShippingResponse(
            resp,
            data.getOrDefault("PartnerID", ""),
            data.getOrDefault("EshipID", ""),
            data.getOrDefault("SetStatus", "")
        );
    }
}
