package io.github.will413028.hawkr.payuni.shipping.request;

public record OnboardCvsRequest(
    String goodsType,       // 1=常溫, 2=冷凍
    String lgsType,         // C2C
    String shipType,        // 1 (7-ELEVEN)
    String labelCompanyName,
    String contactName,
    String contactTel,
    String contactMail,
    String storeId,         // 退貨門市代碼
    String notifyUrl
) {}
