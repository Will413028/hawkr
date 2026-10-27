package io.github.will413028.hawkr.shipping.provider.capability;

import java.time.LocalDateTime;
import java.util.Map;
import java.util.Optional;

/**
 * provider → 我方回調的解析與正規化（webhook／Notify／檔案交換）。
 *
 * <p>🔴 <b>本介面是硬規則 2「未知的 provider 原生值一律 fail-closed」唯一的落點。</b>
 * 沒有它，每個 provider 的映射表會各自寫在自己的 consumer 裡，那條硬規則就沒有任何東西承載。
 *
 * <h2>fail-closed 的具體形式：不是拋例外，是「可查詢的未知」</h2>
 * 拋例外會讓整條 webhook 進 DLQ——provider 新增一個狀態碼不該讓鏈路停擺。
 * 但靜默當成已知更糟。因此 {@link InboundStatusEvent#normalizedStatus()} 是 {@code Optional}：
 * <ul>
 *   <li><b>present</b> → 照常推進狀態機</li>
 *   <li><b>empty</b> → 呼叫端<b>必須</b>：保存 raw 值到 {@code provider_status}、發出告警、
 *       <b>不推進狀態機</b>（不猜、不預設放行），並讓讀取側顯示「物流商回報了目前無法辨識的狀態」</li>
 * </ul>
 * ⚠️ 呼叫端把 empty 當成「沒有變化」而跳過，就等於把 fail-closed 寫成 fail-open。
 *
 * <h2>入參型別是 {@link InboundPayload}，不是 {@code Map<String,String>}</h2>
 * 後者是 <b>PayUni form-encoded 的形狀</b>——一個 provider 的 wire format 洩漏進
 * provider-neutral 介面，而那正是本介面存在要消滅的東西（65 附錄 {@code ADR-012}）。
 * 兩個具體後果：① 巢狀 JSON（Uber 的 {@code courier}／{@code manifest} 物件、
 * {@code related_deliveries} 陣列）攤平會產生「攤平規則」這個新的 provider 知識；
 * ② <b>raw bytes 在介面層就遺失</b>，而 Uber 驗簽必須 over raw body。
 */
public interface InboundCapability {

    /** 解析貨態回調。 */
    InboundStatusEvent parseStatusEvent(InboundPayload raw);

    /**
     * 這一則回調是不是標籤／取號事件。
     *
     * <p>🔴 <b>「怎麼分辨」是 provider 專屬知識</b>（PayUni 看 {@code ApiType}），
     * 留在 consumer 裡就會變成第二個 provider 進來時的第一個 {@code if}——
     * 而 adapter 內部的去重鍵**已經**在做同一個判定，兩份必須保持同步而它們分屬兩層。
     * ⚠️ 這正是 BLOCKER 4 要消滅的形狀，只是它藏在「主路徑」而不是 fallback 分支裡。
     */
    boolean isLabelEvent(InboundPayload raw);

    /** 解析標籤／取號回調。provider 沒有這類回調時回 empty。 */
    Optional<InboundLabelEvent> parseLabelEvent(InboundPayload raw);

    /**
     * 這一則回調的去重鍵。
     *
     * <p>⚠️ 去重鍵是 provider 專屬知識（例：PayUni 是「商家代號 + 物流單號 + 貨態」三段），
     * 留在 consumer 裡就會變成第二個 provider 進來時的第一個 if。
     */
    String dedupKey(InboundPayload raw);

    /**
     * @param normalizedStatus  正規化後的 {@code ShippingStatus.value()}；
     *                          <b>empty ＝ 未知 provider 狀態，呼叫端必須 fail-closed</b>
     * @param providerRawStatus provider 原生值，一律保存（即使無法正規化）
     */
    record InboundStatusEvent(Optional<String> normalizedStatus,
                              String providerRawStatus,
                              String providerMessage,
                              LocalDateTime providerStatusAt) {

        public boolean isKnown() {
            return normalizedStatus.isPresent();
        }

        public static InboundStatusEvent unknown(String rawStatus, String message, LocalDateTime at) {
            return new InboundStatusEvent(Optional.empty(), rawStatus, message, at);
        }
    }

    /**
     * @param segmentCodes 標籤三段碼等 provider 專屬識別，以中性 key→value 表達；
     *                     呼叫端只做透傳與持久化，<b>不得依欄位名做業務判斷</b>
     */
    record InboundLabelEvent(String providerTradeNo, Map<String, String> segmentCodes) {}
}
