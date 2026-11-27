package io.github.will413028.hawkr.payuni.shipping.request;

public record OnboardHomeRequest(
    String lgsType,
    String shipType,
    String senderName,
    String senderMobile,
    String senderAddress,
    String productTypeId,
    String notifyUrl
) {}
