package io.github.will413028.hawkr.shipping.provider.capability;

import io.github.will413028.hawkr.shipping.entity.SellerShipmentSettingEntity;
import io.github.will413028.hawkr.shipping.entity.ShipmentEntity;

/** 派車／叫件／交運確認能力。 */
public interface DispatchCapability {

    /**
     * 向 provider 發動取件或派車。
     *
     * <p>🔴 <b>只做 provider 翻譯，不碰平台流程</b>：搶佔（{@code ProviderCallExecutor}）、
     * 狀態轉移、狀態歷程、事件發布與預計到貨日估算全部留在呼叫端。本成員負責的是
     * 「把一筆配送單 ＋ 賣家寄件設定 ＋ 中性選項，翻譯成這個 provider 的請求並打出去」。
     *
     * <p>🔴 <b>為什麼要收 {@code sellerSetting}</b>（與 {@link ReturnCapability} 同一個理由）：
     * provider 的取件請求要的是<b>寄件人</b>資訊（姓名／電話／地址），
     * 而那些欄位在 {@code seller_shipment_settings} 上、不在 {@code ShipmentEntity} 上。
     * 不收的話 adapter 只能自己去查一次——那是把平台的「設定是否仍有效」判斷
     * （{@code OnboardingService.requireActiveSetting}）複製進 adapter。
     *
     * <p>⚠️ <b>失敗以例外表達，不回 outcome</b>：與 {@link ReturnCapability} 相反，
     * 這裡沒有「失敗也要落一列」的需求，而呼叫端的 {@code ProviderCallExecutor}
     * 正是靠例外分辨 markSucceeded／markFailed。回一個「失敗的 outcome」會讓
     * 搶佔狀態機把失敗記成成功。
     *
     * @param sellerSetting 賣家寄件設定（寄件人姓名／電話／地址來源），由呼叫端驗證有效性後傳入
     * @param options       中性選項；provider 專屬語意由 adapter 自行翻譯，<b>不得讓 provider 參數型別越過本介面</b>
     * @throws UnsupportedOperationException provider 沒有派車／叫件動作時（見各 adapter 的說明）
     */
    DispatchOutcome dispatch(ShipmentEntity shipment,
                             SellerShipmentSettingEntity sellerSetting,
                             DispatchOptions options);

    /**
     * 中性派車選項。
     *
     * <p>🔴 型別是 {@code Boolean} 而不是 provider 的 {@code "Y"}／{@code "N"} 字串：
     * 那兩個字面值是 PayUni 的參數形狀，翻譯是 adapter 的事。呼叫端送 Boolean 進來，
     * 換 provider 時不必回頭改呼叫端。
     *
     * @param contactBeforeDelivery 送達前是否需要電聯
     * @param needTrolley           是否需要推車
     * @param memo                  給司機的備註
     */
    record DispatchOptions(Boolean contactBeforeDelivery, Boolean needTrolley, String memo) {
        public static DispatchOptions none() {
            return new DispatchOptions(null, null, null);
        }
    }

    /**
     * 派車結果。
     *
     * <p>⚠️ {@code dispatchedAt} 是本波新增的欄位：黑貓叫件的回應帶一個 provider 側的
     * 受理時間，而它是這支 gRPC 回給賣家的東西。少了它，provider 呼叫就搬不進來
     * ——那正是前一波把 {@code dispatch} 留在 {@code pendingMigration()} 的原因之一，
     * 修法是讓簽章承認那個事實（與 {@code LabelCapability.LabelRequest} 同一手法）。
     *
     * @param providerReference provider 端的派車識別（如取件單號）；無則為 null
     * @param providerMessage   provider 回應訊息（稽核用）
     * @param dispatchedAt      provider 回報的受理時間原樣字串；格式由 provider 決定，呼叫端只透傳不解析
     */
    record DispatchOutcome(String providerReference, String providerMessage, String dispatchedAt) {}
}
