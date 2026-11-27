package io.github.will413028.hawkr.payuni.shipping.request;

public record OnboardCvsRequest(
    String goodsType,
    String lgsType,
    String shipType,
    String labelCompanyName,
    String contactName,
    String contactTel,
    String contactMail,
    String storeId,
    String notifyUrl
) {}
