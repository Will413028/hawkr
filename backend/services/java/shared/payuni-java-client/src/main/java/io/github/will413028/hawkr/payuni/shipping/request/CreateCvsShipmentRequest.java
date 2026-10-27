package io.github.will413028.hawkr.payuni.shipping.request;

/**
 * 超商物流單建立請求
 *
 * @param merTradeNo    商店訂單編號
 * @param shipType      物流商類型（1=7-ELEVEN, 2=大嘴鳥）
 * @param lgsType       通路類型（B2C, C2C）
 * @param goodsType     商品溫層（1=常溫, 2=冷凍, 3=冷藏）
 * @param serviceType   服務類型（1=取貨付款, 3=取貨不付款）
 * @param senderName    寄件人姓名
 * @param senderPhone   寄件人電話
 * @param receiverName  收件人姓名
 * @param receiverPhone 收件人電話
 * @param storeId       門市代號
 * @param tradeAmt      交易金額
 * @param goodsName     商品名稱
 * @param refundStoreId C2C 退貨門市代碼（選填）
 */
public record CreateCvsShipmentRequest(
        String merTradeNo,
        String shipType,
        String lgsType,
        String goodsType,
        String serviceType,
        String senderName,
        String senderPhone,
        String receiverName,
        String receiverPhone,
        String storeId,
        String tradeAmt,
        String goodsName,
        String refundStoreId
) {
}
