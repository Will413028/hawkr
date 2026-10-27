package io.github.will413028.hawkr.shipping.provider.capability;

import io.github.will413028.hawkr.shipping.entity.SellerShipmentSettingEntity;
import io.github.will413028.hawkr.shipping.entity.ShipmentEntity;

/**
 * 逆物流建單能力。
 *
 * <p>⚠️ 這一支在遷移前是<b>完全繞過既有抽象</b>的——兩條逆物流路徑
 * （{@code ShippingService.createReturnCvsShipment} / {@code createReturnHomeShipment}）
 * 就地組裝 provider request，從未經過 strategy。因此本 capability 是「從零建」而不是「搬既有實作」。
 *
 * <p>🔴 <b>它是 active production flow，不是為未來預留</b>：
 * {@code aftersales-service} 的 {@code AfterSaleResolutionService} 經
 * {@code ShippingGrpcClient.createReturnCvsShipment} 打進來，每一筆退貨都會走。
 */
public interface ReturnCapability {

    /**
     * 建立逆向配送單（<b>尚未持久化</b>，由呼叫端統一 insert）。
     *
     * <p>🔴 <b>成功與失敗都回一個 {@link ReturnOutcome}，而不是失敗就丟例外</b>：
     * provider 拒絕時我方必須落一列 {@code CREATE_FAILED} 殼（收斂規範 §8.4，否則
     * Admin 異常清單永遠查不到這筆），而那列殼的欄位組成與成功列<b>幾乎相同</b>。
     * 用例外表達的話，殼的欄位組裝只能在呼叫端再寫一份——那正是遷移前的現況
     * （成功路徑與失敗殼各有一份約 15 個 setter 的副本，沒有任何東西保證它們同步）。
     *
     * @param forwardShipment 對應的正向配送單，用於取寄件／收件資訊與規格；查無正向單時可為 null
     * @param sellerSetting   賣家的退貨收件設定（呼叫端已驗證存在）
     */
    ReturnOutcome createReturnShipment(ShipmentEntity forwardShipment,
                                       SellerShipmentSettingEntity sellerSetting,
                                       ReturnRequest request);

    /**
     * 逆向建單的中性輸入。
     *
     * <p>⚠️ 買家地址四欄只有宅配逆物流用得到（超商退貨是「買家拿去門市寄」，沒有到府取件）；
     * 對超商 adapter 它們是被忽略的，<b>不是漏填</b>。
     *
     * @param orderNo           原訂單編號
     * @param storeId           賣家 store surrogate
     * @param merId             PayUni 商家編號（{@code HAWK<store_id>}）
     * @param returnShipmentNo  我方逆向單號（由呼叫端產生，adapter 不決定平台單號規則）
     * @param temperature       溫層（{@code ShippingTemperature.value()}）；provider 的 GoodsType 由 adapter 換算
     * @param tradeAmt          商品價值（遺失認賠金額）；宅配逆向不代收，恆 0
     */
    record ReturnRequest(String orderNo, long storeId, String merId, String returnShipmentNo,
                         String buyerName, String buyerPhone,
                         String buyerCity, String buyerDistrict, String buyerAddress,
                         String buyerPostalCode,
                         String temperature, long tradeAmt) {}

    /**
     * @param accepted        provider 是否受理
     * @param shipment        <b>恆非 null</b>：受理時是狀態 {@code UNPROCESSED} 的正式列，
     *                        拒絕時是狀態 {@code CREATE_FAILED} 的殼（已帶 provider 狀態與訊息）。
     *                        呼叫端只負責補主鍵、持久化與狀態歷程
     * @param providerStatus  provider 原生狀態碼（拒絕時才有值）
     * @param providerMessage provider 原生訊息
     */
    record ReturnOutcome(boolean accepted, ShipmentEntity shipment,
                         String providerStatus, String providerMessage) {

        public ReturnOutcome {
            if (shipment == null) {
                // 🔴 契約而非防禦：允許 null 就等於允許「provider 拒絕了、但我方沒有任何一列可落地」，
                //    而那正是收斂規範 §8.4 要消滅的靜默失敗。
                throw new IllegalArgumentException(
                        "ReturnOutcome.shipment 不得為 null——受理與拒絕都必須交得出一列可落地的配送單");
            }
        }

        public static ReturnOutcome accepted(ShipmentEntity shipment) {
            return new ReturnOutcome(true, shipment, null, null);
        }

        public static ReturnOutcome rejected(ShipmentEntity failedShell,
                                             String providerStatus, String providerMessage) {
            return new ReturnOutcome(false, failedShell, providerStatus, providerMessage);
        }
    }
}
