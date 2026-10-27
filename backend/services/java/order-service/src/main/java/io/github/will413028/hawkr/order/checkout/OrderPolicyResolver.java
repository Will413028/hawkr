package io.github.will413028.hawkr.order.checkout;

import io.github.will413028.hawkr.common.utils.code.CollectionModel;
import io.github.will413028.hawkr.common.utils.code.CommerceModel;
import io.github.will413028.hawkr.common.utils.code.FulfillmentMode;
import io.github.will413028.hawkr.common.utils.code.InvoiceModel;
import io.github.will413028.hawkr.common.utils.code.GrpcResponseCode;
import io.github.will413028.hawkr.common.utils.code.ShippingMethod;
import io.github.will413028.hawkr.order.exception.OrderBusinessException;
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
     * 平台履約的配送方式白名單——<b>deny-by-default 的具名集合</b>。
     *
     * <p>🔴 <b>為什麼是白名單而不是「非 MERCHANT_DELIVERY 即平台」</b>：後者對「未來新增的配送方式」
     * 與「打錯的字串」給出同一個答案（平台履約），而那兩者需要相反的處置。
     * 新增配送方式時<b>必須</b>回到這一行把它歸類——這是刻意的摩擦，不是遺漏。
     */
    private static final java.util.Set<String> PLATFORM_LOGISTICS_METHODS = java.util.Set.of(
            ShippingMethod.TAQBIN.value(),
            ShippingMethod.SEVEN_ELEVEN_STORE_PICKUP.value(),
            ShippingMethod.UBER_INSTANT.value(),
            ShippingMethod.SEVEN_ELEVEN_BULK_WAREHOUSE.value());

    /**
     * 推導本次結帳的政策快照。
     *
     * @param shippingMethodValue 買家選定的配送方式（{@code ShippingMethod.value()}）
     */
    public OrderPolicySnapshot resolve(String shippingMethodValue) {
        return new OrderPolicySnapshot(
                CommerceModel.SELLER_DIRECT.value(),
                CollectionModel.SELLER_MERCHANT.value(),
                fulfillmentModeFor(shippingMethodValue),
                InvoiceModel.SELLER_TO_BUYER.value(),
                POLICY_VERSION);
    }

    /**
     * 履約模式由配送方式決定。
     *
     * <p>🔴 這一格與 DB 的 {@code chk_orders_fulfillment_mode_consistent} 是<b>同一條規則的兩個位置</b>：
     * {@code MERCHANT_DELIVERY}（商家自行配送）必然是 {@code SELLER_MANAGED}。
     * 兩處不一致的話，結帳會在建單當下被 CHECK 擋掉——那是好的失敗（立刻、明確），
     * 但更好的是這裡先算對。
     *
     * <p>🔴 <b>未知的配送方式 fail-closed（拋 9504），不再回 {@code PLATFORM_LOGISTICS}。</b>
     * 舊註解的論證是「配送方式的合法性由結帳的其他驗證守，這裡多擋一次只會讓同一個錯誤有兩種訊息」——
     * 那個論證只在「其他驗證一定先跑到」成立時才對，而它是<b>呼叫端保證</b>不是決策點保證。
     * 這一格的輸出會被寫進訂單快照並成為撥款／開票的法源，
     * 猜錯的代價是「一個我們看不懂的配送方式被記成平台履約」——
     * 那正是撥款資格 {@code PayoutEligibility} 對未知值一律 BLOCKED 的同一個理由。
     * ⚠️ 2026-09-12 架構合併：該判準原本在獨立的 settlement 服務，現落 payment-service。
     */
    private static String fulfillmentModeFor(String shippingMethodValue) {
        // null 要自己擋：Set.of(...).contains(null) 在 Java 上是 NPE 不是 false，
        // 而 NPE 會被上層翻成 500，維運看到的碼指不到真正的原因。
        if (shippingMethodValue == null) {
            throw new OrderBusinessException(GrpcResponseCode.FULFILLMENT_COMMERCE_MODEL_UNKNOWN);
        }
        if (ShippingMethod.MERCHANT_DELIVERY.value().equals(shippingMethodValue)) {
            return FulfillmentMode.SELLER_MANAGED.value();
        }
        if (PLATFORM_LOGISTICS_METHODS.contains(shippingMethodValue)) {
            return FulfillmentMode.PLATFORM_LOGISTICS.value();
        }
        throw new OrderBusinessException(GrpcResponseCode.FULFILLMENT_COMMERCE_MODEL_UNKNOWN);
    }
}
