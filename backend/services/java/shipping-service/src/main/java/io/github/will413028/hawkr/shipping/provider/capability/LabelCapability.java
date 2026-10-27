package io.github.will413028.hawkr.shipping.provider.capability;

import io.github.will413028.hawkr.shipping.entity.ShipmentEntity;

import java.util.List;
import java.util.Map;

/** 標籤能力（取號／列印／重印／批次）。 */
public interface LabelCapability {

    /**
     * 三段碼在 {@link LabelIdentifierLookup#segmentCodes()} 與
     * {@link InboundCapability.InboundLabelEvent#segmentCodes()} 內的鍵名。
     *
     * <p>🔴 <b>產生者與消費者共用同一份常數</b>：這個 map 由 adapter 產生、由 service 消費，
     * 兩邊各自打字串就是兩份真實來源（改一邊不會有任何東西轉紅，症狀是三段碼靜默全空）。
     *
     * <p>⚠️ 值沿用 provider 欄名只是為了讓稽核 log 對得上；<b>消費端一律引用本常數</b>。
     */
    String SEGMENT_PARTNER_ID = "PartnerId";
    String SEGMENT_ODNO = "Odno";
    String SEGMENT_VALIDATION_NO = "ValidationNo";

    /**
     * 觸發 provider 產生標籤或配號。
     *
     * <p>🔴 <b>為什麼要有 {@link LabelRequest} 而不是只收 {@code shipment}</b>：
     * PayUni 取號／列印必填 {@code ShipDate}（宅配還要 {@code DeliveryDate}），
     * 而它們是<b>請求參數</b>、{@code ShipmentEntity} 上沒有這兩個欄位。
     * 只收 entity 的話 adapter 拿不到就只能猜一個日期——那會直接送錯資料給 provider。
     * 這正是本成員在前一波「搬不過來」的原因，修法是讓簽章承認那個事實。
     *
     * <p>⚠️ 日期的<b>業務規則</b>（假日、工作天窗口、必須未來日）留在呼叫端：
     * 那是平台對賣家的約束，不是 provider 的約束。adapter 只負責翻譯成 provider 參數。
     */
    LabelRequestOutcome requestLabel(ShipmentEntity shipment, LabelRequest request);

    /**
     * 批次觸發。
     *
     * @throws IllegalArgumentException 當 shipments 違反 {@link #batchConstraints()}（超量或分組鍵不一致）
     */
    LabelRequestOutcome requestLabelBatch(List<ShipmentEntity> shipments, LabelRequest request);

    /** provider 對批次的硬約束。呼叫端據此切組，不得自行寫死數字。 */
    BatchConstraints batchConstraints();

    /**
     * 本 provider 的標籤是否以「三段碼」形式回傳（超商軸專屬）。
     *
     * <p>🔴 吸收 {@code ShippingService.processWebhookPrint} 內「三段碼偵測只對超商」的字面分支。
     */
    boolean usesThreeSegmentCode();

    /**
     * 向 provider 取回這一筆的標籤識別（補收路徑用）。
     *
     * <p>🔴 <b>本成員存在的理由是消滅第二份 LgsType 真實來源。</b>
     * {@code ShipmentLabelReconcileScheduler.reconcileOne} 原本自己打 provider 查詢，
     * 因而自己算一次 {@code LgsType}——與 adapter 的 {@code lgsTypeCode()} 形成兩份 provider
     * 參數推導，沒有任何東西保證它們同步。把那段 provider 呼叫收進來之後，
     * 排程器只讀中性的 key→value，provider 參數推導只剩 adapter 這一份。
     *
     * <p>⚠️ 回傳<b>三態</b>而不是 {@code Optional}：「查詢失敗」與「查詢成功但尚未產生標籤」
     * 的處置完全不同（前者是 provider 短暫故障、後者是正常等待），
     * 用 {@code Optional.empty()} 表達會把兩者壓成同一個訊號。
     */
    LabelIdentifierLookup fetchLabelIdentifiers(ShipmentEntity shipment);

    /**
     * 標籤請求的中性參數。
     *
     * @param shipDate     出貨日（{@code yyyyMMdd}）；provider 專屬格式由 adapter 轉換
     * @param deliveryDate 配達日（{@code yyyyMMdd}）；只有宅配取號需要，超商列印為 null
     */
    record LabelRequest(String shipDate, String deliveryDate) {
        public static LabelRequest onShipDate(String shipDate) {
            return new LabelRequest(shipDate, null);
        }
    }

    /**
     * @param synchronousResult 標籤是否已同步取得；false 表示要等 provider 的回調（inbound）補齊
     * @param providerMessage   provider 回覆的原始訊息（僅供稽核與排錯，不得拿來做業務判斷）
     * @param labelAssetId      同步取得時的標籤資產 id（已轉存我方儲存）；非同步為 null
     * @param labelObjectKey    同步取得時的標籤 objectKey。🔴 只存 objectKey，禁止存完整 URL
     */
    record LabelRequestOutcome(boolean synchronousResult, String providerMessage,
                               Long labelAssetId, String labelObjectKey) {
        public static LabelRequestOutcome async() {
            return new LabelRequestOutcome(false, null, null, null);
        }

        public static LabelRequestOutcome synchronous(Long labelAssetId, String labelObjectKey) {
            return new LabelRequestOutcome(true, null, labelAssetId, labelObjectKey);
        }
    }

    /**
     * 標籤識別回查的三態結果。
     *
     * @param querySucceeded   provider 查詢本身有沒有成功（false ＝ provider 故障／查無此件）
     * @param identifiersReady provider 是否已產生完整的標籤識別（查詢成功才有意義）
     * @param providerTradeNo  provider 端的物流序號
     * @param segmentCodes     中性 key→value（鍵名見 {@link #SEGMENT_PARTNER_ID} 等常數）；
     *                         未 ready 時為空 map，<b>不得為 null</b>
     * @param providerMessage  provider 說明，僅供稽核
     */
    record LabelIdentifierLookup(boolean querySucceeded, boolean identifiersReady,
                                 String providerTradeNo, Map<String, String> segmentCodes,
                                 String providerMessage) {

        public LabelIdentifierLookup {
            segmentCodes = segmentCodes == null ? Map.of() : Map.copyOf(segmentCodes);
            if (identifiersReady && segmentCodes.isEmpty()) {
                // 🔴 契約：宣告「識別已就緒」卻交不出任何識別，會讓呼叫端把空值往下寫。
                throw new IllegalArgumentException(
                        "identifiersReady=true 但 segmentCodes 為空——呼叫端會把空的三段碼寫進 shipments");
            }
        }

        public static LabelIdentifierLookup queryFailed(String providerMessage) {
            return new LabelIdentifierLookup(false, false, null, Map.of(), providerMessage);
        }

        public static LabelIdentifierLookup notReady(String providerMessage) {
            return new LabelIdentifierLookup(true, false, null, Map.of(), providerMessage);
        }

        public static LabelIdentifierLookup ready(String providerTradeNo, Map<String, String> segmentCodes) {
            return new LabelIdentifierLookup(true, true, providerTradeNo, segmentCodes, null);
        }
    }
}
