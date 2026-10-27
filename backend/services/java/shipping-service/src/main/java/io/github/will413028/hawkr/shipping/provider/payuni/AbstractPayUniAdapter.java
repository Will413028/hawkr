package io.github.will413028.hawkr.shipping.provider.payuni;

import io.github.will413028.hawkr.common.obs.log.ObsLogger;
import io.github.will413028.hawkr.common.utils.DateTimeUtils;
import io.github.will413028.hawkr.common.utils.code.ShippingStatus;
import io.github.will413028.hawkr.payuni.http.PayUniApiResponse;
import io.github.will413028.hawkr.payuni.shipping.LgsType;
import io.github.will413028.hawkr.payuni.shipping.ShippingClient;
import io.github.will413028.hawkr.payuni.shipping.request.QueryShipmentRequest;
import io.github.will413028.hawkr.payuni.shipping.response.QueryShipmentResponse;
import io.github.will413028.hawkr.shipping.entity.ShipmentEntity;
import io.github.will413028.hawkr.shipping.provider.FulfillmentProviderAdapter;
import io.github.will413028.hawkr.shipping.provider.capability.InboundCapability;
import io.github.will413028.hawkr.shipping.provider.capability.InboundPayload;
import io.github.will413028.hawkr.shipping.provider.capability.LabelCapability;
import io.github.will413028.hawkr.shipping.provider.capability.TrackCapability;
import io.github.will413028.hawkr.shipping.service.ShipmentLabelService;
import io.github.will413028.hawkr.shipping.service.ShipmentStatusMapper;

import java.time.LocalDateTime;
import java.util.Map;
import java.util.Optional;

/**
 * 兩條 PayUni 產品線（黑貓宅配 3001、超商 C2C 3002）共用的 provider 行為。
 *
 * <h2>🔴 這個抽象類存在的理由，以及它刻意不做的事</h2>
 * 兩條線走的是**同一組 PayUni 端點**，差別在 {@code LgsType}／{@code ShipType} 等參數。
 * 把「怎麼呼叫 PayUni」放這裡、「用哪些參數」留給子類，是為了讓
 * {@code deriveLgsType} 這種<b>只有 PayUni 才需要的推導</b>不再由平台的維度欄位即時算出來
 * ——那是最典型的 provider 洩漏，而且它在本 repo 曾有<b>兩份副本</b>（service 一份、排程器一份），
 * 沒有任何東西保證它們同步。<b>兩份都已收斂到 {@link #lgsTypeCode()}</b>：
 * 排程器那一份靠 {@link LabelCapability#fetchLabelIdentifiers} 把 provider 呼叫整段搬進來後消滅。
 *
 * <p>⚠️ 本類別<b>不</b>持有憑證：{@link ShippingClient} 是已經解析好憑證的 client
 * （硬規則 3）。也<b>不</b>做業務判斷：狀態要不要推進是狀態機的事，本類別只回報「provider 說了什麼」。
 */
public abstract class AbstractPayUniAdapter implements FulfillmentProviderAdapter {

    private static final ObsLogger log = ObsLogger.forClass(AbstractPayUniAdapter.class);

    protected final ShippingClient shippingClient;
    protected final ShipmentStatusMapper statusMapper;

    /**
     * 標籤的實際 provider 呼叫。
     *
     * <p>⚠️ adapter 委派它而不是把 code 搬進來：{@code ShipmentLabelService} 同時做了
     * 「打 PayUni」與「下載檔案並轉存我方儲存」，後者是平台的媒體流程、不是 provider 能力。
     * 整段搬進 adapter 會把媒體轉存也變成 provider 知識。
     */
    protected final ShipmentLabelService labelService;

    protected AbstractPayUniAdapter(ShippingClient shippingClient,
                                    ShipmentStatusMapper statusMapper,
                                    ShipmentLabelService labelService) {
        this.shippingClient = shippingClient;
        this.statusMapper = statusMapper;
        this.labelService = labelService;
    }

    /**
     * 本產品線在 PayUni 的物流型態代碼。
     *
     * <p>🔴 這是 {@code deriveLgsType} 的正確歸宿：它是 <b>provider 的常數</b>，
     * 不是從平台的 {@code shipping_method} 即時推導出來的東西。
     * 由子類各自宣告之後，那個推導函式（及其第二份副本）就沒有存在理由了。
     */
    protected abstract String lgsTypeCode();

    /**
     * 逆向建單的 provider 訂單編號。
     *
     * <p>PayUni 的 {@code MerTradeNo} 同一商店下 10 分鐘內不允許重複，而正向出貨已經用掉 {@code orderNo}
     * ⇒ 逆向必須加後綴避開衝突。這是 <b>provider 的約束</b>，兩條 PayUni 線共用。
     */
    protected static String returnMerTradeNo(String orderNo) {
        return orderNo + "-Refund";
    }

    // ── track：主動回查 ────────────────────────────────────────────────
    private final TrackCapability trackCapability = new TrackCapability() {
        @Override
        public TrackResult query(ShipmentEntity shipment) {
            QueryShipmentResponse resp = queryProvider(shipment);

            if (!resp.isSuccess()) {
                // provider 明確回失敗：不是「未知狀態」，是這次查詢沒成功。
                // 兩者要分開——把查詢失敗當成未知狀態，會讓 provider 短暫故障變成資料污染。
                log.warn("PayUniAdapter.query.failed",
                        "PayUni 查詢失敗", "message", String.valueOf(resp.apiResponse().message()));
                return new TrackResult(null, null, resp.apiResponse().message(), null);
            }

            String raw = resp.shipStatus();
            ShippingStatus normalized = statusMapper.mapPayUniStatus(raw);
            // 🔴 normalized 為 null ＝ provider 回了一個映射表不認得的值。
            //    這裡**照樣把 raw 帶回去**——呼叫端要保存它、要告警、但不推進狀態機。
            //    舊實作在這一格是 `log.warn(...); return entity;`：raw 值沒有落庫、沒有指標，
            //    於是「PayUni 新增了一個狀態碼」這件事在系統裡完全沒有痕跡。
            return new TrackResult(
                    normalized == null ? null : normalized.value(),
                    raw,
                    resp.apiResponse().message(),
                    parseProviderTime(resp.shipStatusTime()));
        }
    };

    @Override
    public Optional<TrackCapability> track(io.github.will413028.hawkr.shipping.provider.FulfillmentContext ctx) {
        return Optional.of(trackCapability);
    }

    /**
     * PayUni {@code /api/logistics/query} 的唯一呼叫點。
     *
     * <p>🔴 <b>{@code LgsType} 只在這裡被填入</b>。回查（{@link TrackCapability}）與標籤補收
     * （{@link LabelCapability#fetchLabelIdentifiers}）是同一支 provider 查詢的兩個觸發者；
     * 讓它們各自組請求，就是讓 {@code LgsType} 有兩份推導——那正是遷移前的現況
     * （{@code ShipmentLabelReconcileScheduler.deriveLgsType} 是第二份）。
     */
    protected QueryShipmentResponse queryProvider(ShipmentEntity shipment) {
        return shippingClient.queryShipment(
                shipment.getMerId(),
                new QueryShipmentRequest(shipment.getShipTradeNo(), null, lgsTypeCode()));
    }

    /**
     * 標籤識別回查的共用骨架：查一次 provider，把三態結果組出來。
     *
     * <p>「什麼算完整的標籤識別」交給 {@link #labelIdentifiersOf}（子類覆寫）——那是 provider 知識。
     */
    protected LabelCapability.LabelIdentifierLookup lookupLabelIdentifiers(ShipmentEntity shipment) {
        QueryShipmentResponse resp = queryProvider(shipment);
        if (!resp.isSuccess()) {
            return LabelCapability.LabelIdentifierLookup.queryFailed(resp.apiResponse().message());
        }
        Map<String, String> identifiers = labelIdentifiersOf(resp.apiResponse());
        if (identifiers.isEmpty()) {
            return LabelCapability.LabelIdentifierLookup.notReady(resp.apiResponse().message());
        }
        return LabelCapability.LabelIdentifierLookup.ready(shipment.getShipTradeNo(), identifiers);
    }

    /**
     * 從查詢回應取出<b>完整</b>的標籤識別；尚未產生或本產品線沒有這個概念時回空 map。
     *
     * <p>預設空 map（宅配走同步 PDF，PayUni 對 {@code LgsType=HOME} 的查詢回應 by-design 不帶三段碼）。
     */
    protected Map<String, String> labelIdentifiersOf(PayUniApiResponse api) {
        return Map.of();
    }

    // ── inbound：貨態 Notify 的解析與正規化 ──────────────────────────────
    private final InboundCapability inboundCapability = new InboundCapability() {
        @Override
        public InboundStatusEvent parseStatusEvent(InboundPayload payload) {
            // 🔴 PayUni 是 form-encoded provider ⇒ 只讀 formParams()，rawBody 為 null（ADR-012）。
            //    ⚠️ 參數名不叫 raw：本方法內的 `String raw` 是 PayUni 的貨態原生值，兩者不同東西。
            Map<String, String> rawPayload = payload.formParams();
            // 🔴 欄名取自 PayUni 官方文件的貨態 Notify 欄位表
            //    （docs/第三方整合/PayUni 物流官方 API 總覽.md：ShipStatus／ShipStatusDesc／ShipStatusTime）。
            //    ⚠️ 2026-09-10 修正：原本讀的是 "Message" 與 "StatusTime"，**PayUni 沒有這兩個欄位**。
            //    在 inbound() 還是死碼的期間，這個錯誤完全看不出來——沒有任何呼叫端，
            //    也沒有任何測試拿真實形狀的 payload 餵過它。接上 consumer 的那一刻，
            //    症狀會是 provider_message 與 provider_status_time **靜默全空**。
            String raw = rawPayload.get("ShipStatus");
            String message = rawPayload.get("ShipStatusDesc");
            LocalDateTime at = parseProviderTime(rawPayload.get("ShipStatusTime"));
            if (raw == null || raw.isBlank()) {
                // 🔴 **沒有 ShipStatus 不等於「未知貨態」**——那是另一種事件。
                //    上游 EventKeyBuilder 的註解逐字寫著：
                //    「建單回傳無 ShipStatus（有 TradeStatus），貨態回傳有 ShipStatus」
                //    ⇒ 帶 TradeStatus 的是**建單結果回報**，它根本不是貨態轉移事件。
                //
                //    2026-09-10 dev 實測到的後果：把它當成未知貨態，會讓呼叫端把
                //    `provider_status` 寫成 null ——**覆蓋掉原本存著的 provider 證據**。
                //    （eventKey=HAWK…:S0011789042138064014:STATUS:0，payload 只有 TradeStatus。）
                //    raw 回 null 讓呼叫端分辨得出「沒有東西可保存」與「有值但看不懂」。
                return InboundStatusEvent.unknown(null, message, at);
            }
            ShippingStatus normalized = statusMapper.mapPayUniStatus(raw);
            if (normalized == null) {
                // fail-closed：可查詢的未知，不是例外也不是靜默放行
                return InboundStatusEvent.unknown(raw, message, at);
            }
            return new InboundStatusEvent(Optional.of(normalized.value()), raw, message, at);
        }

        @Override
        public boolean isLabelEvent(InboundPayload raw) {
            return isPrintEvent(raw.formParams());
        }

        @Override
        public Optional<InboundLabelEvent> parseLabelEvent(InboundPayload raw) {
            Map<String, String> rawPayload = raw.formParams();
            String tradeNo = rawPayload.get("ShipTradeNo");
            if (tradeNo == null || tradeNo.isBlank()) {
                return Optional.empty();
            }
            return Optional.of(new InboundLabelEvent(tradeNo, labelSegmentsOf(rawPayload)));
        }

        @Override
        public String dedupKey(InboundPayload raw) {
            Map<String, String> rawPayload = raw.formParams();
            // PayUni 的去重鍵：商家代號 + 物流單號 + **事件種類** + 該種類的鑑別欄位。
            //
            // 🔴 2026-09-10（BLOCKER 4）修正：舊版一律讀 `ShipStatus`，
            //    而 Print 回調的 payload **沒有這個欄位** ⇒ `String.valueOf(null)` 產出
            //    字面字串 "null" ⇒ 同一張單的每一則 Print 回調共用同一個鍵
            //    （而且會與「ShipStatus 缺失的貨態事件」撞鍵）。
            //    直接把 consumer 換成呼叫本方法，會把「Print 只處理得到第一則」變成新的 bug。
            //
            // ⚠️ 這裡刻意與上游 payuni-webhook-service 的 EventKeyBuilder 對齊
            //    （STATUS/PRINT 前綴分離）——兩邊算出來的鍵形狀必須一致，
            //    否則同一則事件在 inbox 與 shipping 的去重表上會被當成兩件事。
            // 🔴 缺欄一律回空字串，**不是** String.valueOf(null) 的字面 "null"。
            //    上游 EventKeyBuilder 逐字用 payload.getOrDefault(欄名, "")；
            //    兩邊對「缺欄」給出不同字面值 ⇒ 同一則事件在 inbox 與 shipping 算出不同的鍵，
            //    而症狀是「有時會重複處理」這種難重現的形狀。
            String merId = rawPayload.getOrDefault("MerID", "");
            String tradeNo = rawPayload.getOrDefault("ShipTradeNo", "");
            if (isPrintEvent(rawPayload)) {
                return String.join(":", merId, tradeNo, "PRINT",
                        rawPayload.getOrDefault("Odno", ""));
            }
            // 🔴 與上游 EventKeyBuilder 逐字對齊：ShipStatus 缺就退 TradeStatus。
            //    不對齊的話，同一則事件在 inbox（上游）與 shipping（本地）算出不同的鍵
            //    ⇒ 去重各算各的，而症狀是「有時會重複處理」這種難重現的形狀。
            String statusValue = rawPayload.getOrDefault("ShipStatus", "");
            if (statusValue == null || statusValue.isBlank()) {
                statusValue = rawPayload.getOrDefault("TradeStatus", "");
            }
            return String.join(":", merId, tradeNo, "STATUS", statusValue);
        }
    };

    /**
     * 這一則回調是不是標籤／取號事件。
     *
     * <p>⚠️ {@code ApiType} 的值域是 PayUni 專屬知識，留在 adapter 內；
     * consumer 讀到的只有「這是 status 還是 label 事件」這個中性結論。
     */
    protected boolean isPrintEvent(Map<String, String> rawPayload) {
        return "Print".equals(rawPayload.get("ApiType"));
    }

    @Override
    public Optional<InboundCapability> inbound(io.github.will413028.hawkr.shipping.provider.FulfillmentContext ctx) {
        return Optional.of(inboundCapability);
    }

    /**
     * PayUni 軸<b>不透過 API 提供 provider 成本</b> ⇒ 一律 {@link Optional#empty()}。
     *
     * <p>🔴 <b>empty 不是 0</b>：0 是合法金額、會讓賣家應收變成 0 而沒有任何訊號；
     * empty 的語意是「本 provider 這條管道沒有成本事實」⇒ 呼叫端不發
     * {@code logistics.cost.incurred}。
     *
     * <p>⚠️ <b>這不代表超商／宅配沒有物流成本</b>——它們的成本走月結對帳
     * （70 §6.1 provider 帳單匯入），與本 capability 是兩條路。
     * 硬要在這裡回一個金額，等於把「對帳才知道的數字」偽裝成「即時已知的事實」。
     */
    @Override
    public Optional<io.github.will413028.hawkr.shipping.provider.capability.CostCapability> cost(
            io.github.will413028.hawkr.shipping.provider.FulfillmentContext ctx) {
        return Optional.empty();
    }

    /** 標籤回調帶回的 provider 專屬識別；子類覆寫（只有超商軸有三段碼）。 */
    protected Map<String, String> labelSegmentsOf(Map<String, String> rawPayload) {
        return Map.of();
    }

    /**
     * PayUni 的時間字串 → UTC {@code LocalDateTime}；無法解析時回 null（不猜時間）。
     *
     * <p>🔴 <b>必須走 {@link DateTimeUtils#parsePayUniDateTimeToUtc}，不得自己 parse。</b>
     * PayUni 回的是 {@code "yyyy-MM-dd HH:mm:ss"} 而且是 <b>UTC+8</b>；
     * 本方法原本寫成 {@code LocalDateTime.parse(raw.replace(' ', 'T'))}——
     * 格式對得上、不會拋例外，但<b>沒有換算時區</b> ⇒ 落庫的 {@code provider_status_time}
     * 整整快 8 小時，而且因為值看起來完全正常，沒有任何東西會發現。
     * （同一個坑在 {@code ShippingService.updateStatus} 的 §10.4 D7 已經修過一次，
     * 這裡是第二份副本；本波把兩者收斂到同一支 parser。）
     */
    protected static LocalDateTime parseProviderTime(String raw) {
        if (raw == null || raw.isBlank()) {
            return null;
        }
        LocalDateTime parsed = DateTimeUtils.parsePayUniDateTimeToUtc(raw);
        if (parsed == null) {
            log.warn("PayUniAdapter.parseProviderTime.failed",
                    "無法解析 provider 回報的時間字串，落 null 而不是猜一個", "raw", raw);
        }
        return parsed;
    }

    /** 供子類宣告 {@code LgsType} 常數時使用，避免各自打字串。 */
    protected static String lgsHome() {
        return LgsType.HOME.code();
    }

    protected static String lgsC2c() {
        return LgsType.C2C.code();
    }
}
