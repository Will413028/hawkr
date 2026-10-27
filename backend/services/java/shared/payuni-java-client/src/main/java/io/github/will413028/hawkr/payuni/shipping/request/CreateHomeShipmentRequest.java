package io.github.will413028.hawkr.payuni.shipping.request;

/**
 * 宅配單建立請求（背景 v1.2）
 *
 * @param merTradeNo      商店訂單編號
 * @param goodsType       商品溫層（1=常溫, 2=冷凍, 3=冷藏）
 * @param senderName      寄件人姓名
 * @param senderPhone     寄件人電話
 * @param senderZipCode   寄件人郵遞區號
 * @param senderAddress   寄件人地址
 * @param receiverName    收件人姓名
 * @param receiverPhone   收件人電話
 * @param receiverZipCode 收件人郵遞區號
 * @param receiverAddress 收件人地址
 * @param deliveryTimeTag 配送時段標記（01=13時前, 02=14-18時, 04=不指定）
 * @param tradeAmt        交易金額
 * @param goodsName       商品名稱
 */
public record CreateHomeShipmentRequest(
        String merTradeNo,
        String goodsType,
        String senderName,
        String senderPhone,
        String senderZipCode,
        String senderAddress,
        String receiverName,
        String receiverPhone,
        String receiverZipCode,
        String receiverAddress,
        String deliveryTimeTag,
        String tradeAmt,
        String goodsName
) {
}
