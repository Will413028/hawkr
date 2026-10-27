package io.github.will413028.hawkr.payuni.shipping.request;

/**
 * 宅配退貨單建立請求（黑貓退貨 v1.0）
 *
 * 對應 PayUni API: /api/home_delivery/refund
 * 用途：建立黑貓退貨單，黑貓司機將於指定取件日至退貨地址收取退件。
 *
 * @param merTradeNo       商店訂單編號（退貨用 RO 前綴，如 RO269180302）
 * @param shipTradeNo      原宅配單 UNi 物流序號（針對原宅配單退貨時必填，否則傳 null）
 * @param goodsType        寄件類型（1=常溫, 2=冷凍, 3=冷藏）
 * @param consigneeName    收件人姓名（賣家退貨聯絡人）
 * @param consigneePhone   收件人手機號碼
 * @param consigneeAddress 收件人地址（賣家退貨地址）
 * @param consignorName    退貨人姓名（買家）
 * @param consignorPhone   退貨人手機號碼（買家）
 * @param consignorAddress 退貨人地址（買家地址）
 * @param deliveryTimeTag  希望配達時間（01=13時前, 02=14-18時, 04=不指定）
 * @param spec             規格代碼（1=60cm, 2=90cm, 3=120cm, 4=150cm；冷凍/冷藏不支援 4）
 * @param shipDate         取件日期（YYYYMMDD，D+1 ~ D+7，16:25 後須 D+2）
 * @param goodsName        商品名稱（最長 20 位）
 * @param memo             備註給黑貓（最長 100 位，可為 null）
 */
public record CreateHomeRefundRequest(
        String merTradeNo,
        String shipTradeNo,
        String goodsType,
        String consigneeName,
        String consigneePhone,
        String consigneeAddress,
        String consignorName,
        String consignorPhone,
        String consignorAddress,
        String deliveryTimeTag,
        String spec,
        String shipDate,
        String goodsName,
        String memo
) {
}
