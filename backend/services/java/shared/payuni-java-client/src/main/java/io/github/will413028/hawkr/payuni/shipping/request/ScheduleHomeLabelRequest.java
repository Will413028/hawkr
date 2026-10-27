package io.github.will413028.hawkr.payuni.shipping.request;

/**
 * 產宅配編號並下載託運單 PDF 請求（v1.0）
 *
 * @param shipTradeNo  物流交易編號（多筆以半形逗號隔開）
 * @param goodsType    溫層代碼（1=常溫, 2=冷凍, 3=冷藏）
 * @param lgsType      物流型態（HOME）
 * @param shipType     物流商類型（2=黑貓）
 * @param shipDate     出貨日期（YYYYMMDD）
 * @param deliveryDate 配達日期（YYYYMMDD）
 * @param spec         規格代碼（1=60cm, 2=90cm, 3=120cm, 4=150cm）
 * @param postType     前景/背景（1=前景, 選填）
 * @param printType    列印類型（1=A4, 選填）
 */
public record ScheduleHomeLabelRequest(
        String shipTradeNo,
        String goodsType,
        String lgsType,
        String shipType,
        String shipDate,
        String deliveryDate,
        String spec,
        String postType,
        String printType
) {
}
