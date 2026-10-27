package io.github.will413028.hawkr.payuni.shipping.request;

public record OnboardHomeRequest(
    String lgsType,         // HOME
    String shipType,        // 2 (黑貓)
    String senderName,
    String senderMobile,
    String senderAddress,
    String productTypeId,
    String notifyUrl
) {}
