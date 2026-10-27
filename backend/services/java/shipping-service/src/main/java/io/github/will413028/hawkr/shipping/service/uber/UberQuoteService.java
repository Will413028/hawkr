package io.github.will413028.hawkr.shipping.service.uber;

import com.baomidou.mybatisplus.core.toolkit.Wrappers;
import io.github.will413028.hawkr.common.obs.log.ObsLogger;
import io.github.will413028.hawkr.common.utils.TsidUtil;
import io.github.will413028.hawkr.shipping.client.BuyerGrpcClient;
import io.github.will413028.hawkr.shipping.entity.PackingPolicyEntity;
import io.github.will413028.hawkr.shipping.entity.ShippingMethodConfigEntity;
import io.github.will413028.hawkr.shipping.entity.UberQuoteEntity;
import io.github.will413028.hawkr.shipping.entity.UberStoreMappingEntity;
import io.github.will413028.hawkr.shipping.mapper.PackingPolicyMapper;
import io.github.will413028.hawkr.shipping.mapper.UberQuoteMapper;
import io.github.will413028.hawkr.shipping.provider.uber.UberAddressStructure;
import io.github.will413028.hawkr.shipping.provider.uber.UberAddressUnparseableException;
import io.github.will413028.hawkr.shipping.provider.uber.UberApiClient;
import io.github.will413028.hawkr.shipping.provider.uber.UberApiException;
import io.github.will413028.hawkr.shipping.provider.uber.UberApiModels.ManifestItem;
import io.github.will413028.hawkr.shipping.provider.uber.UberApiModels.QuoteRequest;
import io.github.will413028.hawkr.shipping.provider.uber.UberApiModels.QuoteResult;
import io.github.will413028.hawkr.shipping.provider.uber.UberApiModels.UberWaypoint;
import io.github.will413028.hawkr.shipping.provider.uber.UberFieldFormat;
import io.github.will413028.hawkr.shipping.provider.uber.UberPackingPlanner;
import io.github.will413028.hawkr.shipping.provider.uber.UberPayloadCodec;
import io.github.will413028.hawkr.shipping.provider.uber.UberQuotePayloadHash;
import io.github.will413028.hawkr.shipping.provider.uber.UberQuoteStatus;
import io.github.will413028.hawkr.shipping.provider.uber.UberStoreResolver;
import io.github.will413028.hawkr.shipping.service.ShippingFeeService;
import com.fasterxml.jackson.databind.ObjectMapper;

import java.time.Clock;
import java.time.Instant;
import java.time.LocalDateTime;
import java.time.ZoneOffset;
import java.util.ArrayList;
import java.util.List;
import java.util.Optional;

/**
 * Uber 即時報價（66 §6.1.3／§7.1；提示詞 Stage 8B W2-2）。
 *
 * <h2>🔴 這一支是 W2 缺的那一段</h2>
 * 在它落地之前，{@code uber_quotes} 這張表<b>只有讀取端</b>
 * （{@code UberDirectAdapter.acceptedQuote()}），零寫入端
 * ⇒ 建單必然走到 {@code markCreateFailed("UBER_QUOTE_NOT_ACCEPTED")}。
 * 那不是 bug（它 fail-closed 且留痕），是這一步還沒做。
 *
 * <h2>決策樹（66 §6.1.3 逐字）</h2>
 * <pre>
 * 進結帳頁 / 改地址 / 改數量 → 伺服端算 payload_hash
 *   ├ 已有 CREATED 且 hash 相同且剩餘 &gt;180s → 直接回既有 quote（不打 Uber）
 *   ├ 已有但 hash 不同                     → 舊的標 SUPERSEDED，重報
 *   └ 沒有 / 過期 / 剩餘 ≤180s              → 重報
 * </pre>
 * ⚠️「剩餘 ≤180s 就重報」在<b>報價階段</b>做，不是等到 capture——
 * capture 當下才發現過期，使用者已經按下付款了。
 *
 * <h2>🔴 每一條失敗路徑都回「不可用 ＋ 理由」，不是拋例外</h2>
 * 報價失敗<b>不是</b>結帳失敗：Uber 不可用時買家還有黑貓與超商可選。
 * 讓它拋例外會把「一個選項算不出價」升級成「整個結帳頁掛掉」。
 * ⇒ 回 {@link Outcome}，由呼叫端把該選項標成 {@code available=false} 並附上理由。
 */
public class UberQuoteService {

    private static final ObsLogger log = ObsLogger.forClass(UberQuoteService.class);

    /** Uber 即時配送的 {@code shipping_method} 碼（CodeMeta {@code UBER_INSTANT}）。 */
    public static final String METHOD_UBER = "3003";

    /**
     * 🔴 重報門檻：剩餘 &le; 180 秒即視為過期（66 T-2）。
     *
     * <p>它同時是<b>報價</b>與<b>confirm 前重驗</b>的門檻，而且必須是同一個數字：
     * 報價用 200 秒、confirm 用 180 秒的話，中間那 20 秒會產生
     * 「報價階段說沒過期、confirm 說過期」的組合，而使用者已經在付款頁了。
     */
    public static final long MIN_REMAINING_SECONDS = 180L;

    private final UberStoreResolver storeResolver;
    private final UberApiClient apiClient;
    private final UberPayloadCodec payloadCodec;
    private final UberQuoteMapper quoteMapper;
    private final PackingPolicyMapper packingPolicyMapper;
    private final UberQuoteWriter writer;
    private final BuyerGrpcClient buyerGrpcClient;
    private final ShippingFeeService shippingFeeService;
    private final ObjectMapper objectMapper;
    private final Clock clock;
    private final boolean enabled;

    public UberQuoteService(UberStoreResolver storeResolver,
                            UberApiClient apiClient,
                            UberPayloadCodec payloadCodec,
                            UberQuoteMapper quoteMapper,
                            PackingPolicyMapper packingPolicyMapper,
                            UberQuoteWriter writer,
                            BuyerGrpcClient buyerGrpcClient,
                            ShippingFeeService shippingFeeService,
                            ObjectMapper objectMapper,
                            Clock clock,
                            boolean enabled) {
        this.storeResolver = storeResolver;
        this.apiClient = apiClient;
        this.payloadCodec = payloadCodec;
        this.quoteMapper = quoteMapper;
        this.packingPolicyMapper = packingPolicyMapper;
        this.writer = writer;
        this.buyerGrpcClient = buyerGrpcClient;
        this.shippingFeeService = shippingFeeService;
        this.objectMapper = objectMapper;
        this.clock = clock;
        this.enabled = enabled;
    }

    /** 結帳頁要報價時給的東西。🔴 {@code payloadHash} 不在裡面——它由伺服端算（66 §5.1.4）。 */
    public record Ask(long buyerId,
                      long sellerStoreId,
                      String deliveryTemp,
                      String receiverName,
                      String receiverPhone,
                      String county,
                      String district,
                      String detailAddress,
                      List<UberPackingPlanner.Item> items) {
    }

    /**
     * @param quote {@code null} ⇒ 不可用
     * @param unavailableReason {@code null} ⇒ 可用。非 null 時是<b>可寫進 UI 的理由碼</b>
     */
    public record Outcome(UberQuoteEntity quote, String unavailableReason) {

        public boolean available() {
            return quote != null && unavailableReason == null;
        }

        static Outcome unavailable(String reason) {
            return new Outcome(null, reason);
        }
    }

    /** 取得（或沿用）一筆可用的報價。 */
    public Outcome quote(Ask ask) {
        if (!enabled) {
            // 🔴 總開關關閉時連 store mapping 都不查：那會在 log 上製造
            //    「有人在用 Uber」的假象，而實際上一筆都不會成立。
            return Outcome.unavailable("UBER_DISABLED");
        }
        ShippingMethodConfigEntity config = shippingFeeService.getConfig(METHOD_UBER, ask.deliveryTemp());
        if (config == null) {
            // getConfig 只載 enabled=true 的列 ⇒ 這一格同時涵蓋「3003 該溫層未開通」
            // 與「整個 3003 還沒 enabled」。66 §12.2 步 9 之前這是<b>預期</b>結果。
            return Outcome.unavailable("UBER_METHOD_NOT_ENABLED");
        }
        Optional<UberStoreMappingEntity> mappingOpt = storeResolver.activeMapping(ask.sellerStoreId());
        if (mappingOpt.isEmpty()) {
            return Outcome.unavailable("UBER_STORE_NOT_MAPPED");
        }
        UberStoreMappingEntity mapping = mappingOpt.get();

        PackingPolicyEntity policy = activePolicy();
        if (policy == null) {
            // 🔴 沒有生效政策 ⇒ 分箱沒有依據 ⇒ 不可用。
            //    用一組寫死的預設值繼續算，會讓 uber_quotes.packing_policy_version 指向不存在的版本，
            //    而那一欄的用途正是「這張訂單當時用什麼規則算的」。
            return Outcome.unavailable("PACKING_POLICY_MISSING");
        }

        UberPackingPlanner.Plan plan = UberPackingPlanner.estimate(
                ask.items(), policy, limitsOf(config));
        if (!plan.usable()) {
            return Outcome.unavailable(plan.rejectReason());
        }

        UberWaypoint dropoff;
        try {
            dropoff = resolveDropoff(ask);
        } catch (UberAddressUnparseableException e) {
            log.warn("UberQuoteService.quote.addressUnparseable", e.getMessage());
            return Outcome.unavailable("ADDRESS_UNPARSEABLE");
        }

        List<ManifestItem> manifestItems = manifestItems(ask.items(), plan);
        String payloadHash = UberQuotePayloadHash.of(
                mapping.getExternalStoreId(), dropoff, manifestItems, policy.getVersion());

        UberQuoteEntity reusable = reusableQuote(ask.buyerId(), ask.sellerStoreId(), payloadHash);
        if (reusable != null) {
            // 🔴 不打 Uber：E1 的 token 有 100 req/hr 上限，而結帳頁每次重算都會走到這裡。
            return new Outcome(reusable, null);
        }

        // hash 不同或沒有可用的 ⇒ 舊的換手再重報。
        // ⚠️ 換手必須在打 Uber **之前**：打完才換手的話，E1 失敗會留下一筆
        //    hash 已經不符當下購物車、狀態卻仍是 CREATED 的報價。
        writer.supersedeCreated(ask.buyerId(), ask.sellerStoreId());

        QuoteResult result;
        try {
            result = apiClient.createQuote(mapping.getCustomerId(), new QuoteRequest(
                    mapping.getExternalStoreId(),
                    payloadCodec.waypointFromJson(mapping.getPickupSnapshot()),
                    dropoff,
                    UberFieldFormat.pickupReadyDt(Instant.now(clock), mapping.getPrepSlaMinutes())));
        } catch (UberApiException e) {
            // E1 無副作用（66 §4.5.0）⇒ 失敗就是「這個選項這次算不出價」，不是錯誤狀態。
            log.warn("UberQuoteService.quote.e1Failed", "Uber 報價失敗：" + e.getMessage());
            return Outcome.unavailable("QUOTE_FAILED");
        }

        UberQuoteEntity entity = new UberQuoteEntity();
        entity.setId(TsidUtil.generate());
        entity.setQuoteId(result.id());
        entity.setBuyerId(ask.buyerId());
        entity.setSellerStoreId(ask.sellerStoreId());
        entity.setExternalStoreId(mapping.getExternalStoreId());
        entity.setFeeAmount(result.fee());
        entity.setCurrency(result.currency());
        entity.setExpiresAt(toLocalDateTime(result.expires()));
        entity.setPickupJson(payloadCodec.waypointToJson(
                payloadCodec.waypointFromJson(mapping.getPickupSnapshot())));
        entity.setDropoffJson(payloadCodec.waypointToJson(dropoff));
        entity.setItemsJson(payloadCodec.itemsToJson(manifestItems));
        entity.setPayloadHash(payloadHash);
        entity.setPackingPolicyVersion(policy.getVersion());
        entity.setStatus(UberQuoteStatus.CREATED);

        writer.insertQuoteWithPackages(entity, plan.packages());
        return new Outcome(entity, null);
    }

    /**
     * confirm（capture 前）重驗（66 §6.1.4）。
     *
     * <p>🔴 兩件都要：① 剩餘 &gt; {@value #MIN_REMAINING_SECONDS} 秒 ② {@code payload_hash} 與當下購物車一致。
     * 任一不成立 ⇒ 重報一次；<b>金額不同 ⇒ 不 capture</b>，要求使用者重新確認
     * （自動接受新價＝補收＝違約；自動用舊價＝我方吃差額且無上限，INV-U1）。
     *
     * @return 重驗結果。{@link Revalidation#priceChanged()} 為 true 時呼叫端<b>不得</b> capture
     */
    public Revalidation revalidate(String quoteId, Ask ask) {
        UberQuoteEntity current = quoteMapper.selectOne(Wrappers.<UberQuoteEntity>lambdaQuery()
                .eq(UberQuoteEntity::getQuoteId, quoteId));
        Outcome fresh = quote(ask);
        if (!fresh.available()) {
            return new Revalidation(null, false, fresh.unavailableReason());
        }
        boolean sameQuote = current != null && current.getQuoteId().equals(fresh.quote().getQuoteId());
        if (sameQuote) {
            return new Revalidation(fresh.quote(), false, null);
        }
        // 換了一張報價：只有在金額也不同時才需要使用者重新確認。
        // 🔴 金額相同就不打擾使用者——INV-U1 要守的是「金額」不是「quote_id」。
        boolean priceChanged = current == null
                || !fresh.quote().getFeeAmount().equals(current.getFeeAmount());
        return new Revalidation(fresh.quote(), priceChanged, null);
    }

    /**
     * @param quote 重驗後<b>應該採用</b>的那一筆
     * @param priceChanged true ⇒ 回 {@code UBER_QUOTE_PRICE_CHANGED} ＋ {@code data.newFee}，不 capture
     * @param unavailableReason 非 null ⇒ Uber 這次整個不可用
     */
    public record Revalidation(UberQuoteEntity quote, boolean priceChanged, String unavailableReason) {
    }

    /**
     * capture 成功後把報價標成已確認（66 §6.1.4）。
     *
     * @return false ⇒ CAS 未命中（已被換手／過期／已確認過）⇒ 🔴 呼叫端必須當成失敗處理
     */
    public boolean accept(String quoteId, String orderNo) {
        return writer.markCheckoutAccepted(quoteId, orderNo) == 1;
    }

    // ==================== internals ====================

    /** 生效版本＝{@code effective_from <= now} 中 version 最大者（66 §5.1.8）。 */
    private PackingPolicyEntity activePolicy() {
        List<PackingPolicyEntity> rows = packingPolicyMapper.selectList(
                Wrappers.<PackingPolicyEntity>lambdaQuery()
                        .le(PackingPolicyEntity::getEffectiveFrom, nowUtc())
                        .orderByDesc(PackingPolicyEntity::getVersion)
                        .last("LIMIT 1"));
        return rows.isEmpty() ? null : rows.get(0);
    }

    /**
     * 可沿用的報價：狀態 {@code CREATED} ＋ hash 相同 ＋ 剩餘 &gt; 180 秒。
     *
     * <p>🔴 三個條件缺一不可。少了 hash ⇒ 改過地址還用舊價；少了剩餘秒數 ⇒ 把一筆
     * 即將過期的報價推給付款頁；少了狀態 ⇒ 可能沿用到已被 {@code SUPERSEDED} 的那筆。
     */
    private UberQuoteEntity reusableQuote(long buyerId, long sellerStoreId, String payloadHash) {
        LocalDateTime threshold = nowUtc().plusSeconds(MIN_REMAINING_SECONDS);
        List<UberQuoteEntity> rows = quoteMapper.selectList(Wrappers.<UberQuoteEntity>lambdaQuery()
                .eq(UberQuoteEntity::getBuyerId, buyerId)
                .eq(UberQuoteEntity::getSellerStoreId, sellerStoreId)
                .eq(UberQuoteEntity::getStatus, UberQuoteStatus.CREATED)
                .eq(UberQuoteEntity::getPayloadHash, payloadHash)
                .gt(UberQuoteEntity::getExpiresAt, threshold)
                .orderByDesc(UberQuoteEntity::getId)
                .last("LIMIT 1"));
        return rows.isEmpty() ? null : rows.get(0);
    }

    /** 收件 waypoint：結構化地址（我方三欄）＋ 地理編碼（buyer-service／TGOS）。 */
    private UberWaypoint resolveDropoff(Ask ask) {
        UberAddressStructure.Structured structured = UberAddressStructure.of(
                objectMapper, ask.county(), ask.district(), ask.detailAddress());
        String full = nullSafe(ask.county()) + nullSafe(ask.district()) + nullSafe(ask.detailAddress());
        var validation = buyerGrpcClient.validateAddress(full);
        if (validation.getCode() != 0 || !validation.getData().getValid()) {
            throw new UberAddressUnparseableException(
                    "地址驗證未通過（code=" + validation.getCode() + "）");
        }
        Double lat = parseCoordinate(validation.getData().getLat());
        Double lng = parseCoordinate(validation.getData().getLng());
        if (lat == null || lng == null) {
            // 🔴 fail-closed：Uber 建單的 dropoff_latitude／longitude 是必填（官方總覽 §2.2）。
            //    沒有座標就寫進 quote 的話，炸掉的時點是買家付完錢之後的建單。
            throw new UberAddressUnparseableException("地址驗證沒有回經緯度");
        }
        return new UberWaypoint(
                structured.addressJson(),
                ask.receiverName(),
                ask.receiverPhone(),
                lat,
                lng,
                structured.notes());
    }

    /** 分箱結果 → Uber {@code manifest_items}。{@code size} 取該箱的 {@code size_class}。 */
    private List<ManifestItem> manifestItems(List<UberPackingPlanner.Item> items,
                                             UberPackingPlanner.Plan plan) {
        // 🔴 size 目前一律取第一箱的 size_class：66 未列出 Uber size 的值域（U-K／N4），
        //    在 sandbox 坐實之前不發明「哪一件放在哪一箱」的對應。
        //    ⚠️ 這一格是 Enablement Blocker，不是可以靜默放過的簡化。
        String size = plan.packages().isEmpty() ? null : plan.packages().get(0).getSizeClass();
        List<ManifestItem> out = new ArrayList<>();
        for (UberPackingPlanner.Item item : items) {
            out.add(new ManifestItem(
                    item.name(),
                    item.quantity(),
                    size,
                    UberFieldFormat.toMinorUnits(item.unitPrice(), "TWD"),
                    item.skuId()));
        }
        return out;
    }

    private static UberPackingPlanner.Limits limitsOf(ShippingMethodConfigEntity config) {
        return new UberPackingPlanner.Limits(
                config.getMaxDimensionSumCm() == null ? 0L : config.getMaxDimensionSumCm(),
                config.getMaxSingleSideCm() == null ? 0L : config.getMaxSingleSideCm(),
                config.getMaxWeightGrams() == null ? 0L : config.getMaxWeightGrams(),
                config.getMaxVolumeCm3() == null ? 0L : config.getMaxVolumeCm3());
    }

    private static Double parseCoordinate(String raw) {
        if (raw == null || raw.isBlank()) {
            return null;
        }
        try {
            return Double.valueOf(raw.trim());
        } catch (NumberFormatException e) {
            // 🔴 不猜、不回 0：0,0 是幾內亞灣，而那會變成一筆看起來合法的報價。
            return null;
        }
    }

    private LocalDateTime nowUtc() {
        return LocalDateTime.ofInstant(Instant.now(clock), ZoneOffset.UTC);
    }

    private static LocalDateTime toLocalDateTime(Instant instant) {
        return instant == null ? null : LocalDateTime.ofInstant(instant, ZoneOffset.UTC);
    }

    private static String nullSafe(String s) {
        return s == null ? "" : s;
    }
}
