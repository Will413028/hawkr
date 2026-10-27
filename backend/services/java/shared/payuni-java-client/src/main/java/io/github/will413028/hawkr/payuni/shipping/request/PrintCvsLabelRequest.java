package io.github.will413028.hawkr.payuni.shipping.request;

/**
 * 超商出貨單列印請求（v1.0，後端代理）
 *
 * @param shipTradeNo 物流交易編號
 * @param goodsType   溫層代碼（1=常溫, 2=冷凍, 3=冷藏）
 * @param lgsType     通路類型（C2C）
 * @param shipType    物流商類型（1=7-ELEVEN）
 * @param shipDate    出貨日期（YYYYMMDD）
 */
public record PrintCvsLabelRequest(
        String shipTradeNo,
        String goodsType,
        String lgsType,
        String shipType,
        String shipDate
) {
}
