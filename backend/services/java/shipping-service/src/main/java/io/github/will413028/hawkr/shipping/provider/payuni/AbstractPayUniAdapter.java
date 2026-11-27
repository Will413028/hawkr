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

    private final TrackCapability trackCapability = new TrackCapability() {
        @Override
        public TrackResult query(ShipmentEntity shipment) {
            QueryShipmentResponse resp = queryProvider(shipment);

            if (!resp.isSuccess()) {
                log.warn("PayUniAdapter.query.failed",
                        "PayUni 查詢失敗", "message", String.valueOf(resp.apiResponse().message()));
                return new TrackResult(null, null, resp.apiResponse().message(), null);
            }

            String raw = resp.shipStatus();
            ShippingStatus normalized = statusMapper.mapPayUniStatus(raw);
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

    private final InboundCapability inboundCapability = new InboundCapability() {
        @Override
        public InboundStatusEvent parseStatusEvent(InboundPayload payload) {
            Map<String, String> rawPayload = payload.formParams();
            String raw = rawPayload.get("ShipStatus");
            String message = rawPayload.get("ShipStatusDesc");
            LocalDateTime at = parseProviderTime(rawPayload.get("ShipStatusTime"));
            if (raw == null || raw.isBlank()) {
                return InboundStatusEvent.unknown(null, message, at);
            }
            ShippingStatus normalized = statusMapper.mapPayUniStatus(raw);
            if (normalized == null) {
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
            String merId = rawPayload.getOrDefault("MerID", "");
            String tradeNo = rawPayload.getOrDefault("ShipTradeNo", "");
            if (isPrintEvent(rawPayload)) {
                return String.join(":", merId, tradeNo, "PRINT",
                        rawPayload.getOrDefault("Odno", ""));
            }
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
