package io.github.will413028.hawkr.payuni.shipping.request;

/**
 * 呼叫黑貓預約取件請求（v1.0）
 *
 * @param contactName     聯絡人姓名
 * @param contactMobile   聯絡人手機
 * @param contactAddress  聯絡人地址
 * @param normalQuantity  常溫件數
 * @param coldQuantity    冷藏件數
 * @param freezeQuantity  冷凍件數
 * @param isContact       是否需要事先電聯（Y/N，選填）
 * @param isTrolley       是否需要推車（Y/N，選填）
 * @param memo            給宅配人員備註（選填）
 */
public record CallTcatRequest(
        String contactName,
        String contactMobile,
        String contactAddress,
        String normalQuantity,
        String coldQuantity,
        String freezeQuantity,
        String isContact,
        String isTrolley,
        String memo
) {
}
