package io.github.will413028.hawkr.order.checkout;

/**
 * 訂單政策快照五欄——在 <b>preview 階段</b>決定，confirm 時隨訂單一起凍結。
 *
 * <p>🔴 <b>為什麼必須在 preview 就決定</b>：{@code collection_model} 決定收款主體，
 * 而收款主體必須在<b>建立付款交易之前</b>就定案。等到 confirm 才算，付款用的 MerID
 * 與快照欄可能來自兩次不同的求值 ⇒ 快照說「平台收款」而錢進了賣家的子商店，
 * 且<b>沒有任何東西會發現</b>（訂單成功、付款成功、帳本平衡、對帳也查得到——只是查到的是別人的交易）。
 *
 * @param commerceModel                商流模式：這筆錢誰收、誰開票
 * @param collectionModel              收款模式：款項實際經過誰的手
 * @param fulfillmentMode              履約模式：誰負責把貨送到
 * @param invoiceModel                 買家發票由誰開
 * @param policyVersion                事後重算的依據
 */
public record OrderPolicySnapshot(
        String commerceModel,
        String collectionModel,
        String fulfillmentMode,
        String invoiceModel,
        String policyVersion
) {
}
