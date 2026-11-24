package io.github.will413028.hawkr.order.checkout;

import io.github.will413028.hawkr.common.utils.code.CollectionModel;
import io.github.will413028.hawkr.common.utils.code.CommerceModel;
import io.github.will413028.hawkr.common.utils.code.FulfillmentMode;
import io.github.will413028.hawkr.common.utils.code.InvoiceModel;
import org.springframework.stereotype.Component;

/**
 * 訂單政策的伺服端推導——<b>唯一的決定點</b>。
 *
 * <h2>🔴 這個類別取代了什麼</h2>
 * 改造前，收款主體由 {@code CheckoutPreviewRequest.consolidated}（<b>買家 request 透傳的布林</b>）決定。
 * 需求書把「移除 consolidated 由 buyer request 透傳、改由後端以賣家契約／履約模式／政策版本推導」
 * 列為<b>不可妥協的工程邊界</b>之一——理由不需要多解釋：<b>收款主體不能由呼叫端指定</b>。
 *
 * <h2>本波（Shared Foundation）的推導結果：一律 {@code SELLER_DIRECT}</h2>
 * 受託代銷（{@code PLATFORM_CONSIGNMENT}）<b>刻意尚未啟用</b>，這不是「還沒寫」而是 fail-closed：
 * <ul>
 *   <li>撥款指示費的<b>費率</b>未經正式契約確認 ⇒ 撥款鏈整條不得啟用</li>
 *   <li>賣家→平台的進項發票軸尚未落地 ⇒ 撥款資格的其中一條恆 BLOCKED</li>
 * </ul>
 * ⇒ 現階段推導出 {@code PLATFORM_CONSIGNMENT} 會讓撥款鏈在費率與進項發票軸都未就緒時啟動。
 * <b>「值域已定義」與「已啟用」是兩件事</b>，本類別是後者的閘門。
 *
 * <p>⚠️ 啟用受託代銷時要改的是<b>這裡</b>（加上賣家契約查詢），不是在呼叫端塞一個布林。
 */
@Component
public class OrderPolicyResolver {

    /**
     * 本波的政策版本。
     *
     * <p>🔴 政策版本是<b>快照欄</b>：改政策不得回溯既有訂單，所以每一筆訂單都要帶著
     * 它成立當下的版本。事後爭議時，這是「用同一組輸入重算出同一個數字」的前提。
     */
    public static final String POLICY_VERSION = "v1-foundation";

    /**
     * 推導本次建單的政策快照。
     *
     * <p>履約一律 {@code SELLER_MANAGED}（賣家自寄）：平台沒有物流商，出貨歸 B8 的訂單出貨設計。
     * 結帳與 RFQ 建單共用這一支，不再有第二個決定點。
     */
    public OrderPolicySnapshot resolve() {
        return new OrderPolicySnapshot(
                CommerceModel.SELLER_DIRECT.value(),
                CollectionModel.SELLER_MERCHANT.value(),
                FulfillmentMode.SELLER_MANAGED.value(),
                InvoiceModel.SELLER_TO_BUYER.value(),
                POLICY_VERSION);
    }
}
