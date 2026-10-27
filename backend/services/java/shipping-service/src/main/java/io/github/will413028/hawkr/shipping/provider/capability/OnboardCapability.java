package io.github.will413028.hawkr.shipping.provider.capability;

import io.github.will413028.hawkr.shipping.entity.SellerShipmentSettingEntity;

/**
 * 建店／開通能力。
 *
 * <h2>🔴 {@code submit} / {@code resubmit} 目前**沒有呼叫端**，而那是設計，不是漏接線</h2>
 * 設計明訂：<b>動作端點一律 provider-neutral，只有 onboarding 端點保留 provider 名</b>
 * （{@code /app/shipping_onboardings/{cvs|home_delivery|uber}}）。
 * 理由是各 provider 的建店欄位<b>本質不同</b>——實查：超商 7 個參數
 * （goodsType／退貨門市三欄／寄件人兩欄），宅配 13+ 個（商品分類／寄件地址四欄／
 * 退貨人與退貨地址各四欄）。硬塞進同一個 neutral 端點只會做出一個每個 provider
 * 都只用一半欄位的請求體，而「哪些欄位這次要填」變成前端要猜的事。
 *
 * <p>⚠️ 因此<b>不要</b>為了讓 gRPC 入口「走 adapter」而在呼叫端組一個假的
 * {@link SellerShipmentSettingEntity} 只為了傳參數——那比直接呼叫更差：
 * 它讓一個尚不存在的持久化物件變成參數載體，而下一個人會以為那是一筆真的設定。
 *
 * <p>那 {@code submit}／{@code resubmit} 存在的意義是什麼？
 * <b>讓 provider 側的建店呼叫有一個歸屬。</b> 新 provider 接入時，
 * 「怎麼向這個 provider 送審」有地方放，而不是又在某個 service 上長出一組具名方法。
 * {@link #shippingTypeValue()} 則是<b>現在就在用</b>的——它取代了
 * {@code mapShippingMethodToType} 的字面分支（那個方法對未登記的配送方式回 null，
 * 而下游兩處都是 {@code if (shippingType == null) return null} ⇒ 建單永遠取不到設定，且不報錯）。
 *
 * <p>依「每個新狀態的四問」，這一格的「誰把它推進來」目前的答案是 <b>尚無</b>，本註解即為登記。
 */
public interface OnboardCapability {

    /** 向 provider 送出建店申請。 */
    OnboardOutcome submit(SellerShipmentSettingEntity setting);

    /** 重送（provider 退件後修正再送）。 */
    OnboardOutcome resubmit(SellerShipmentSettingEntity setting);

    /**
     * 本配送方式對應的 {@code ShippingType.value()}（建店槽位的分類鍵）。
     *
     * <p>🔴 吸收 {@code OnboardingService.mapShippingMethodToType} 的字面分支。
     * 該方法對未登記的配送方式回 {@code null}，而下游兩處都是
     * {@code if (shippingType == null) return null} ⇒ <b>建單流程永遠取不到設定</b>，
     * 且 {@code buildSlotKey(null, null)} 在 Java 字串串接下產出字面 {@code "null:"}
     * ——不會 NPE，所以不會有人發現。
     */
    String shippingTypeValue();

    /**
     * @param providerReference provider 端的商店識別
     * @param submitStatus      我方的送審狀態值（{@code PayUniSubmitStatus} 之類），已正規化
     */
    record OnboardOutcome(String providerReference, String submitStatus, String providerMessage) {}
}
