package io.github.will413028.hawkr.shipping.provider.uber;

import io.github.will413028.hawkr.common.webhook.UberWebhookVerifier;
import com.baomidou.mybatisplus.core.toolkit.Wrappers;
import io.github.will413028.hawkr.common.obs.log.ObsLogger;
import io.github.will413028.hawkr.common.utils.code.GrpcResponseCode;
import io.github.will413028.hawkr.common.utils.code.ShippingMethod;
import io.github.will413028.hawkr.common.utils.code.ShippingStatus;
import io.github.will413028.hawkr.order.grpc.OrderShippingInfo;
import io.github.will413028.hawkr.shipping.config.UberConfig;
import io.github.will413028.hawkr.shipping.entity.SellerShipmentSettingEntity;
import io.github.will413028.hawkr.shipping.entity.ShipmentEntity;
import io.github.will413028.hawkr.shipping.entity.UberQuoteEntity;
import io.github.will413028.hawkr.shipping.entity.UberStoreMappingEntity;
import io.github.will413028.hawkr.shipping.exception.ShippingBusinessException;
import io.github.will413028.hawkr.shipping.mapper.UberQuoteMapper;
import io.github.will413028.hawkr.shipping.provider.FulfillmentContext;
import io.github.will413028.hawkr.shipping.provider.FulfillmentProviderAdapter;
import io.github.will413028.hawkr.shipping.provider.capability.CancelCapability;
import io.github.will413028.hawkr.shipping.provider.capability.CostCapability;
import io.github.will413028.hawkr.shipping.provider.capability.CreateCapability;
import io.github.will413028.hawkr.shipping.provider.capability.DispatchCapability;
import io.github.will413028.hawkr.shipping.provider.capability.InboundCapability;
import io.github.will413028.hawkr.shipping.provider.capability.InboundPayload;
import io.github.will413028.hawkr.shipping.provider.capability.LabelCapability;
import io.github.will413028.hawkr.shipping.provider.capability.OnboardCapability;
import io.github.will413028.hawkr.shipping.provider.capability.PodCapability;
import io.github.will413028.hawkr.shipping.provider.capability.ReturnCapability;
import io.github.will413028.hawkr.shipping.provider.capability.StoreLocatorCapability;
import io.github.will413028.hawkr.shipping.provider.capability.TrackCapability;
import io.github.will413028.hawkr.shipping.provider.create.UberShipmentStrategy;
import io.github.will413028.hawkr.shipping.provider.uber.UberApiModels.CreateDeliveryRequest;
import io.github.will413028.hawkr.shipping.provider.uber.UberApiModels.DeliveryResult;
import io.github.will413028.hawkr.shipping.provider.uber.UberApiModels.PodDocument;
import com.fasterxml.jackson.databind.JsonNode;
import com.fasterxml.jackson.databind.ObjectMapper;
import org.springframework.stereotype.Component;

import java.io.IOException;
import java.nio.charset.StandardCharsets;
import java.security.MessageDigest;
import java.security.NoSuchAlgorithmException;
import java.time.Clock;
import java.time.Instant;
import java.time.LocalDateTime;
import java.time.ZoneOffset;
import java.util.HexFormat;
import java.util.Optional;
import java.util.Set;

/**
 * Uber Direct 即時配送（{@code 3003}）的 provider adapter。
 *
 * <h2>capability 狀態（66 §4.5.0，值域權威在 65 §4.4.1b）</h2>
 * <table border="1">
 *   <caption>3003</caption>
 *   <tr><th>capability</th><th>狀態</th><th>端點／說明</th></tr>
 *   <tr><td>{@code create}</td><td>✅</td><td>E2 Create Delivery（消費結帳已接受的報價）</td></tr>
 *   <tr><td>{@code dispatch}</td><td>✅</td><td>E2；Uber 的「派車」就是建單本身，同一支呼叫不同觸發者</td></tr>
 *   <tr><td>{@code cancel}</td><td>✅ <b>四個 provider 裡唯一</b></td><td>E6 Cancel Delivery</td></tr>
 *   <tr><td>{@code track}</td><td>✅</td><td>E4 Get Delivery</td></tr>
 *   <tr><td>{@code pod}</td><td>✅</td><td>E7 ＋ 轉存 media {@code secure/}</td></tr>
 *   <tr><td>{@code onboard}</td><td>✅</td><td>{@code uber_store_mapping} 的狀態；<b>無 provider 端點</b>（見下）</td></tr>
 *   <tr><td>{@code inbound}</td><td>✅</td><td>webhook 驗簽 ＋ 狀態映射</td></tr>
 *   <tr><td>{@code label}</td><td>❌ <b>永久</b></td><td>即時配送無標籤概念——<b>這不是缺口</b></td></tr>
 *   <tr><td>{@code returns}</td><td>❌ <b>永久</b></td><td>逆向 leg 是另開一筆配送記錄，不是同一單的逆物流</td></tr>
 *   <tr><td>{@code storeLocator}</td><td>❌ <b>永久</b></td><td>送到府，沒有門市</td></tr>
 * </table>
 *
 * <h2>🔴 這個 adapter 只做翻譯（硬規則 1）</h2>
 * 「這個狀態要不要推進訂單」「要不要建 case」「要不要退款」全部留在呼叫端。
 * 本類別唯一碰業務語意的地方是<b>建單失敗的 {@code 990}／{@code 991} 分流</b>
 * ——那是 66 §4.5.4 明訂由 provider 回應決定的一格，也是陷阱 A8 的防線本身：
 * 把它留給呼叫端，等於讓每個呼叫端各自重新決定一次「5xx 算不算失敗」。
 *
 * <h2>⚠️ capability 的 present 與「現在能不能用」是兩件事</h2>
 * capability 回答的是「這個 provider 的 API 有沒有這個能力」——那是<b>靜態</b>的。
 * {@code uber.enabled=false} 或憑證未掛時，本 adapter 的 capability <b>仍然 present</b>，
 * 而呼叫在執行時 fail-closed（{@link #requireUsable}）。
 * 反過來做（不可用就回 empty）會讓 {@code allowedActions} 對停機中的 Uber 回
 * 「此配送服務尚未開放（9503）」——那句話是假的，而且它會讓下一個人以為 Uber 從來沒接。
 */
@Component
public class UberDirectAdapter implements FulfillmentProviderAdapter {

    private static final ObsLogger log = ObsLogger.forClass(UberDirectAdapter.class);

    /** POD 抓取窗口（66 T-12：取 U-F 照片 7 天與 U-G 簽名 30 天的<b>最短</b>者，不取 30 天）。 */
    static final int POD_RETENTION_HOURS = 7 * 24;


    /** 我方 pre-commit 鍵前綴（65 {@code ADR-014}：{@code uber_create_delivery:{order_no}:{leg}}）。 */
    static final String PRECOMMIT_KEY_PREFIX = "uber_create_delivery:";

    private final UberShipmentStrategy createStrategy;
    private final UberApiClient apiClient;
    private final UberStoreResolver storeResolver;
    private final UberQuoteMapper quoteMapper;
    private final UberPayloadCodec payloadCodec;
    private final UberPodStorage podStorage;
    private final UberWebhookVerifier webhookVerifier;
    private final UberConfig.UberAvailability availability;
    private final ObjectMapper objectMapper;
    private final Clock clock;

    public UberDirectAdapter(UberShipmentStrategy createStrategy,
                             UberApiClient apiClient,
                             UberStoreResolver storeResolver,
                             UberQuoteMapper quoteMapper,
                             UberPayloadCodec payloadCodec,
                             UberPodStorage podStorage,
                             UberWebhookVerifier webhookVerifier,
                             UberConfig.UberAvailability availability,
                             ObjectMapper objectMapper,
                             Clock clock) {
        this.createStrategy = createStrategy;
        this.apiClient = apiClient;
        this.storeResolver = storeResolver;
        this.quoteMapper = quoteMapper;
        this.payloadCodec = payloadCodec;
        this.podStorage = podStorage;
        this.webhookVerifier = webhookVerifier;
        this.availability = availability;
        this.objectMapper = objectMapper;
        this.clock = clock;
    }

    @Override
    public ShippingMethod supportedMethod() {
        return ShippingMethod.UBER_INSTANT;
    }

    /**
     * 🔴 空集合：本波（W2-1）之後 3003 沒有「provider 有能力但我們還沒搬」的 capability。
     *
     * <p>{@code label}／{@code returns}／{@code storeLocator} 三個 empty 是<b>永久</b>的
     * （provider 不支援），依 {@link FulfillmentProviderAdapter#pendingMigration()} 的規約不列入。
     */
    @Override
    public Set<String> pendingMigration() {
        return Set.of();
    }

    // ==================================================================
    // create：E1 報價前置（消費結帳已接受的報價）＋ E2 Create Delivery
    // ==================================================================

    @Override
    public Optional<CreateCapability> create(FulfillmentContext ctx) {
        return Optional.of(new CreateCapability() {
            @Override
            public ShipmentEntity createFromOrder(String orderNo, OrderShippingInfo orderInfo,
                                                  SellerShipmentSettingEntity onboardingSetting) {
                ShipmentEntity entity = createStrategy.createShipmentFromOrder(orderNo, orderInfo, onboardingSetting);
                if (entity == null) {
                    // strategy 的契約：回 null 代表它自己沒組出東西，這裡不代它決定。
                    return null;
                }
                submitDelivery(entity);
                return entity;
            }

            /**
             * 🔴 Uber 不需要建店——賣家在 Uber 沒有帳號，平台才是唯一的 API 客戶。
             *
             * <p>⚠️ 這與 {@link #onboard} 為 present <b>不矛盾</b>：
             * 「建單前要不要先有 {@code seller_shipment_settings}」與
             * 「這個 provider 有沒有開通概念」是兩個問題。Uber 的開通是
             * {@code uber_store_mapping}，它由賣家設定出貨門市那條路徑寫入，不走建店槽位。
             */
            @Override
            public boolean requiresOnboarding() {
                return false;
            }
        });
    }

    /**
     * 打 E2 建單，並把結果寫進 entity（尚未持久化，由呼叫端 insert）。
     *
     * <h2>🔴 失敗分流（66 §4.5.4／§7.6，陷阱 A8）</h2>
     * <table border="1">
     *   <caption>建單失敗 → shipments.status</caption>
     *   <tr><th>失敗</th><th>狀態</th><th>為什麼不是別的</th></tr>
     *   <tr><td>其他 400（確定性）</td><td>{@code 990 CREATE_FAILED}</td><td>重試同一 payload 只會再錯一次</td></tr>
     *   <tr><td>{@code customer_blocked} 403</td><td>{@code 990}</td><td>＋呼叫端寫 {@code blocked=true}＋通知賣家（N6 事後反推）</td></tr>
     *   <tr><td>404</td><td>{@code 990}</td><td>對方明確說沒有這個資源 ⇒ 確定沒建出單</td></tr>
     *   <tr><td><b>5xx／timeout</b></td><td><b>{@code 991 CREATE_UNKNOWN}</b></td>
     *       <td>🔴 <b>不得當 990</b>——可能已建成功，重建＝兩趟車、重複計費</td></tr>
     *   <tr><td>429</td><td>{@code 991}</td><td>退避重試耗盡後仍不確定對方收了沒</td></tr>
     * </table>
     *
     * <h2>⚠️ 「請求根本沒送出去」的那幾種<b>拋例外</b>，不寫 990／991</h2>
     * 憑證缺席、token 取不到、總開關關著——這幾種<b>確定沒有</b>在 Uber 側產生任何東西。
     * <ul>
     *   <li>寫 {@code 990} ＝ 因為一次暫時性認證失敗就<b>把買家的錢退掉、訂單作廢</b></li>
     *   <li>寫 {@code 991} ＝ 讓回查排程去找一個<b>本來就不存在</b>的單，白跑三天才落 990</li>
     * </ul>
     * ⇒ 拋例外讓 {@code order.paid} 的 consumer 走既有的 retry／DLQ，而重試是安全的
     * （沒有任何 Uber 側副作用要去重）。⚠️ 這一格 66 沒有明寫，是本波的判斷，已登記在交付報告。
     */
    void submitDelivery(ShipmentEntity entity) {
        requireUsable();
        // 🔴 門市對應缺席寫 990 而不是拋例外。
        //    兩者的差別是「誰會發現」：拋例外 ⇒ order.paid 的 consumer 重試耗盡進 DLQ
        //    ⇒ 買家付了錢、連一列 shipment 都沒有、訂單卡住，而 DLQ 沒有人盯；
        //    寫 990 ⇒ 有一列帶著 provider_error_code 的單，Admin 看得到，
        //    且 990 有既有的出口（Admin 重建／Admin 取消＋退款鏈）。
        //    ⚠️ 它是確定性的（重試一百次都一樣）——結帳時的承載檢查道 1 本來就該擋掉它，
        //    走到這裡代表門市在「結帳 → 付款」之間被停用，那需要人看一眼。
        Optional<UberStoreMappingEntity> mappingOpt =
                storeResolver.activeMapping(entity.getStoreId() == null ? 0L : entity.getStoreId());
        if (mappingOpt.isEmpty()) {
            markCreateFailed(entity, "UBER_STORE_NOT_MAPPED",
                    "查無可用的 Uber 門市對應（未設定、已關閉，或被 Uber 封鎖）");
            return;
        }
        UberStoreMappingEntity mapping = mappingOpt.get();
        UberQuoteEntity quote = acceptedQuote(entity.getOrderNo());
        if (quote == null) {
            // 確定性：沒有已被結帳確認的報價就建不出單，重試一百次都一樣。
            // （uber_quotes 的 CONSUMED allowed-from 只允許 CHECKOUT_ACCEPTED，66 §5.1.4。）
            markCreateFailed(entity, "UBER_QUOTE_NOT_ACCEPTED",
                    "找不到已被結帳確認的 Uber 報價（order_no=" + entity.getOrderNo() + "）");
            return;
        }
        CreateDeliveryRequest request = new CreateDeliveryRequest(
                quote.getQuoteId(),
                precommitKey(entity),
                mapping.getExternalStoreId(),
                payloadCodec.waypointFromJson(quote.getPickupJson()),
                payloadCodec.waypointFromJson(quote.getDropoffJson()),
                payloadCodec.itemsFromJson(quote.getItemsJson()),
                entity.getShipmentNo(),
                pickupReadyDt(mapping));
        try {
            DeliveryResult result = apiClient.createDelivery(mapping.getCustomerId(), request);
            entity.setUberDeliveryId(result.id());
            entity.setStatus(ShippingStatus.UNPROCESSED.value());
            // 🔴 **刻意不在這裡寫 provider_status**（V12 的 C4 判準，static gate 擋）。
            //
            //    `shipments.provider_status` 只有一條寫入路徑：mapper 的
            //    `updateProviderStatus`，它是唯一不要求狀態轉移合法性的寫入點，
            //    也是「未知 provider 原生值」統一 fail-closed 的落點
            //    （ShippingService.persistUnknownProviderStatus）。
            //    在建單成功這一格用 entity setter 塞一份，等於開第二條寫入路徑 ——
            //    而第二條路徑**不會**經過未知值處置，下一個 provider 進來時就會在
            //    這裡靜默寫進一個沒有人認得的原生值。
            //
            //    ⚠️ 不寫並沒有遺失資訊：建單當下 Uber 回的是 `pending`，
            //    它映射到的正是上面那一行已經寫下的 91 UNPROCESSED；
            //    真正的原生值會由 webhook 或 T-8 回查經 canonical ingress 補上。
            //    result.status() 仍保留在回應物件上供 log 與測試斷言使用。
            log.info("UberDirectAdapter.create.providerStatus",
                    "Uber 建單回應原生狀態（不落 provider_status，由 canonical ingress 寫入）",
                    "providerStatus", result.status());
            log.info("UberDirectAdapter.create.success",
                    "Uber 建單成功", "orderNo", entity.getOrderNo());
        } catch (UberApiException e) {
            applyCreateFailure(entity, e);
        }
    }

    private void applyCreateFailure(ShipmentEntity entity, UberApiException e) {
        switch (e.kind()) {
            case AUTH -> throw e;                       // 請求沒送出去（見 submitDelivery javadoc）
            case NON_DETERMINISTIC, RATE_LIMITED -> {
                // 🔴 991 不是 990：可能已經建成功了。收斂交給 T-8 回查（66 §7.7）。
                entity.setStatus(ShippingStatus.CREATE_UNKNOWN.value());
                setProviderError(entity, e);
                log.warn("UberDirectAdapter.create.unknown",
                        "Uber 建單非確定性失敗，寫 991 交回查收斂（不得重建）",
                        "orderNo", entity.getOrderNo());
            }
            default -> {
                entity.setStatus(ShippingStatus.CREATE_FAILED.value());
                setProviderError(entity, e);
                log.warn("UberDirectAdapter.create.failed",
                        "Uber 建單確定性失敗，寫 990", "orderNo", entity.getOrderNo());
            }
        }
    }

    private void markCreateFailed(ShipmentEntity entity, String code, String message) {
        entity.setStatus(ShippingStatus.CREATE_FAILED.value());
        entity.setProviderErrorCode(code);
        entity.setProviderErrorMessage(message);
        entity.setProviderErrorAt(nowUtc());
    }

    private void setProviderError(ShipmentEntity entity, UberApiException e) {
        entity.setProviderErrorCode(e.providerErrorCode());
        entity.setProviderErrorMessage(e.getMessage());
        entity.setProviderErrorAt(nowUtc());
    }

    // ==================================================================
    // dispatch：E2（Uber 的「派車」就是建單本身）
    // ==================================================================

    /**
     * 🔴 <b>present</b>：66 §4.5.0 E2 第二列——「由 {@code order.paid} 自動觸發」與賣家手動重試
     * 走的是<b>同一支</b> provider 呼叫，差別只在觸發者。
     *
     * <p>⚠️ 這一格的<b>唯一</b>合法呼叫端是 65 §6.1.1 的 provider-neutral
     * {@code S-10 POST /app/shipments/:order_no/dispatch}；
     * 既有的黑貓專用 {@code callTcat} 不是（它會寫 {@code 210 TCAT_CALLED}，而 3003 不經過該值）。
     * 狀態面的防線已補在 {@code FulfillmentActionPolicy}：{@code dispatch@3003} 的 allowed-from
     * <b>只有 {@code 990 CREATE_FAILED}</b>。
     */
    @Override
    public Optional<DispatchCapability> dispatch(FulfillmentContext ctx) {
        return Optional.of((shipment, sellerSetting, options) -> {
            // ⚠️ sellerSetting 對 Uber 恆為 null（requiresOnboarding()=false，沒有建店槽位）。
            //    取件人資訊來自 uber_store_mapping 的快照，不是 seller_shipment_settings。
            submitDelivery(shipment);
            if (ShippingStatus.CREATE_FAILED.value().equals(shipment.getStatus())
                    || ShippingStatus.CREATE_UNKNOWN.value().equals(shipment.getStatus())) {
                // 🔴 DispatchCapability 的契約逐字：「失敗以例外表達，不回 outcome」——
                //    呼叫端的 ProviderCallExecutor 正是靠例外分辨 markSucceeded／markFailed，
                //    回一個「失敗的 outcome」會讓搶佔狀態機把失敗記成成功。
                throw new ShippingBusinessException(GrpcResponseCode.SHIPMENT_PROVIDER_CALL_FAILED,
                        shipment.getProviderErrorMessage() == null
                                ? "Uber 建單未成功" : shipment.getProviderErrorMessage());
            }
            return new DispatchCapability.DispatchOutcome(
                    shipment.getUberDeliveryId(), shipment.getProviderStatus(), null);
        });
    }

    // ==================================================================
    // cancel：E6（四個 provider 裡唯一真的打 provider 的取消）
    // ==================================================================

    @Override
    public Optional<CancelCapability> cancel(FulfillmentContext ctx) {
        return Optional.of((shipment, reason) -> {
            requireUsable();
            UberStoreMappingEntity mapping = requireMapping(shipment);
            String deliveryId = requireDeliveryId(shipment);
            try {
                DeliveryResult result = apiClient.cancelDelivery(mapping.getCustomerId(), deliveryId);
                // provider 已給出終局答案 ⇒ 呼叫端可直接落終態 41（66 §4.5.0 E6：寫 992 → 打 E6 → 成功 41）。
                return new CancelCapability.CancelOutcome(true, true, result.status());
            } catch (UberApiException e) {
                if (e.kind() == UberFailureKind.NONCANCELABLE) {
                    // 🔴 取消失敗、**留原狀態**（不寫 992 → 41）、回 9522（66 §4.5.4／§7.6）。
                    //
                    //    ⚠️ 為什麼是拋例外而不是回 CancelOutcome：那個 record 只有
                    //    (accepted, terminal) 兩個布林，而它們表達不出「被拒絕，請還原成原本的狀態」——
                    //    terminal=true 的註解逐字是「可直接落終態」（會對買家宣告取消而貨照送），
                    //    terminal=false 是「落 992 等回調」（會卡在一個等不到的狀態）。
                    //    兩個都是錯的答案，所以不能用那個通道。已登記在交付報告的「與設計書不符」節。
                    throw new ShippingBusinessException(GrpcResponseCode.SHIPMENT_CANCEL_NOT_CANCELLABLE);
                }
                throw e;
            }
        });
    }

    // ==================================================================
    // track：E4
    // ==================================================================

    @Override
    public Optional<TrackCapability> track(FulfillmentContext ctx) {
        return Optional.of(shipment -> {
            requireUsable();
            String deliveryId = shipment.getUberDeliveryId();
            if (deliveryId == null || deliveryId.isBlank()) {
                // 🔴 陷阱 B2：991 沒有 delivery_id，用 E4 回查會每次 404、永不收斂。
                //    那條路徑的正解是 E3 ＋ 我方 external_id 反查（66 §7.7），不是這裡。
                return new TrackCapability.TrackResult(null, null,
                        "尚無 Uber delivery_id，不以 E4 回查（991 的收斂走 E3 反查）", null);
            }
            UberStoreMappingEntity mapping = requireMapping(shipment);
            DeliveryResult result = apiClient.getDelivery(mapping.getCustomerId(), deliveryId);
            return new TrackCapability.TrackResult(
                    UberStatusMapper.normalize(result.status()).map(ShippingStatus::value).orElse(null),
                    result.status(),
                    // 外送員電話一律遮罩後才進任何我方欄位／log（66 §8.6）
                    UberFieldFormat.maskPhone(result.courierPhone()),
                    toLocalDateTime(result.statusAt()));
        });
    }

    // ==================================================================
    // pod：E7 ＋ 轉存 media secure/
    // ==================================================================

    @Override
    public Optional<PodCapability> pod(FulfillmentContext ctx) {
        return Optional.of(new PodCapability() {
            @Override
            public Optional<PodCapability.PodResult> fetchPod(ShipmentEntity shipment) {
                requireUsable();
                String deliveryId = shipment.getUberDeliveryId();
                if (deliveryId == null || deliveryId.isBlank()) {
                    return Optional.empty();
                }
                UberStoreMappingEntity mapping = requireMapping(shipment);
                try {
                    PodDocument doc = apiClient.fetchProofOfDelivery(
                            mapping.getCustomerId(), deliveryId, "dropoff", "picture");
                    if (doc.content() == null || doc.content().length == 0) {
                        return Optional.empty();
                    }
                    // 🔴 走 secure/ 前綴（陷阱 A6：public 上傳沒有任何訊號，
                    //    而 POD 的內容是收件地址與門口實景）。
                    String objectKey = podStorage.store(deliveryId, doc.content(), doc.contentType());
                    return Optional.of(new PodCapability.PodResult(
                            objectKey, null, toLocalDateTime(Instant.now(clock))));
                } catch (UberApiException e) {
                    if (e.kind() == UberFailureKind.NOT_FOUND) {
                        // 窗口未到或該筆沒有這一種 POD（leave_at_door 只有 picture、需簽收才有 signature）。
                        // ⚠️ INV-U5 因此是「≥1 筆」而不是「三種都要有」（66 §7.8）。
                        return Optional.empty();
                    }
                    throw e;
                }
            }

            @Override
            public int retentionWindowHours() {
                return POD_RETENTION_HOURS;
            }
        });
    }

    // ==================================================================
    // onboard：uber_store_mapping 的狀態（🔴 沒有 provider 端點）
    // ==================================================================

    /**
     * 🔴 <b>present，但它不打任何 Uber API</b>。
     *
     * <p>實查兩份權威：66 §4.5.2 的端點表只有 E1~E7，
     * {@code docs/第三方整合/Uber Direct API 總覽.md} 也<b>沒有</b>任何建店／組織端點。
     * Uber 的「開通」在我方是 {@code uber_store_mapping} 這一列
     * （賣家設定出貨門市 ⇒ S-U1／ADM ⇒ A-U4 ⇒ 建單 403 的 N6 事後反推）。
     *
     * <p>⇒ 本 capability 的職責是讓「這個賣家在 Uber 那邊開通了沒」有一個歸屬，
     * 而不是憑空造一支不存在的 provider 呼叫。
     */
    @Override
    public Optional<OnboardCapability> onboard(FulfillmentContext ctx) {
        return Optional.of(new OnboardCapability() {
            @Override
            public OnboardOutcome submit(SellerShipmentSettingEntity setting) {
                return mappingStatus(setting);
            }

            @Override
            public OnboardOutcome resubmit(SellerShipmentSettingEntity setting) {
                return mappingStatus(setting);
            }

            /**
             * 🔴 回 {@code null}：Uber <b>沒有建店槽位</b>。
             *
             * <p>{@code ShippingType} 的值域是 {@code CVS_C2C}／{@code HOME_DELIVERY}／
             * {@code MERCHANT_DELIVERY}，沒有 Uber 的格子，
             * 而 {@code OnboardingService.mapShippingMethodToType} 對 null 的處理
             * （{@code Optional.map} ⇒ empty ⇒ {@code orElse(null)}）正是我們要的：
             * 建單不去查 {@code seller_shipment_settings}。
             *
             * <p>⚠️ 硬塞一個既有值進來會讓 Uber 的建單去查別條線的建店設定，
             * 而 {@code buildSlotKey} 的鍵一撞就是靜默取到別人的設定。
             */
            @Override
            public String shippingTypeValue() {
                return null;
            }
        });
    }

    private OnboardCapability.OnboardOutcome mappingStatus(SellerShipmentSettingEntity setting) {
        long storeId = setting == null || setting.getStoreId() == null ? 0L : setting.getStoreId();
        return storeResolver.activeMapping(storeId)
                .map(m -> new OnboardCapability.OnboardOutcome(
                        // 🔴 external_store_id 是憑證等級的值（66 §8.6）⇒ 回傳遮罩後的形式。
                        UberFieldFormat.maskStoreId(m.getExternalStoreId()),
                        m.getHealthStatus(),
                        "Uber 門市對應已啟用"))
                .orElseGet(() -> new OnboardCapability.OnboardOutcome(
                        null, "UNKNOWN",
                        "尚未建立可用的 Uber 門市對應（未設定、已關閉，或被 Uber 封鎖）"));
    }

    // ==================================================================
    // inbound：webhook 驗簽 ＋ 狀態映射
    // ==================================================================

    @Override
    public Optional<InboundCapability> inbound(FulfillmentContext ctx) {
        return Optional.of(new InboundCapability() {

            @Override
            public InboundStatusEvent parseStatusEvent(InboundPayload raw) {
                JsonNode body = verifiedBody(raw);
                String rawStatus = firstText(body, "status", "order_status");
                if (rawStatus == null && body.path("data").isObject()) {
                    rawStatus = firstText(body.path("data"), "status");
                }
                LocalDateTime at = toLocalDateTime(parseEpochOrIso(
                        firstText(body, "created", "event_time")));
                String message = firstText(body, "kind", "event_type");

                if (rawStatus == null || rawStatus.isBlank()) {
                    // 沒有 status 的事件（例：courier_update 的位置更新）不是「未知貨態」。
                    // raw 回 null 讓呼叫端分辨得出「沒有東西可保存」與「有值但看不懂」
                    // ——把它寫成未知貨態會用「保存證據」的名義把 provider_status 覆蓋成 null。
                    return InboundStatusEvent.unknown(null, message, at);
                }
                Optional<ShippingStatus> normalized = UberStatusMapper.normalize(rawStatus);
                if (normalized.isEmpty()) {
                    // 🔴 fail-closed：raw 值**一律保存**，不推進狀態機（INV-U7）。
                    //    ⚠️ 容忍值（shopping_completed）與真未知值在這裡長得一樣，
                    //    呼叫端用 UberStatusMapper.isTolerated 分辨：容忍值只計 counter、**不建 case**。
                    return InboundStatusEvent.unknown(rawStatus, message, at);
                }
                return new InboundStatusEvent(
                        Optional.of(normalized.get().value()), rawStatus, message, at);
            }

            /** 🔴 恆 false：即時配送沒有標籤／取號概念（{@code label()} 是永久 empty）。 */
            @Override
            public boolean isLabelEvent(InboundPayload raw) {
                return false;
            }

            @Override
            public Optional<InboundLabelEvent> parseLabelEvent(InboundPayload raw) {
                return Optional.empty();
            }

            /**
             * 去重鍵（66 §6.3.3）。
             *
             * <p>級 1（首選）：provider event id ⇒ {@code event_key = <event_id>}。
             * <p>級 2（fallback）：{@code {delivery_id}:{event_type}:{provider_timestamp}:{sha256(raw_body)[0:16]}}。
             *
             * <p>⚠️ 級 2 的失效條件（66 §6.3.3 逐字）：同一秒內同 delivery 的同型事件且內容完全相同
             * ⇒ 被誤判重複而丟棄。{@code delivery_status} 不會（狀態變了 body 就變）；
             * {@code courier_update} 會（外送員沒動時 body 相同）⇒ <b>可接受</b>，
             * 因為它的語意是 level（丟一則不影響最終值）。
             */
            @Override
            public String dedupKey(InboundPayload raw) {
                JsonNode body = verifiedBody(raw);
                String eventId = firstText(body, "id", "event_id");
                if (eventId != null && !eventId.isBlank()) {
                    return eventId;
                }
                return String.join(":",
                        nullToEmpty(firstText(body, "delivery_id", "order_id")),
                        nullToEmpty(firstText(body, "kind", "event_type")),
                        nullToEmpty(firstText(body, "created", "event_time")),
                        sha256Prefix(raw.rawBody()));
            }
        });
    }

    /**
     * 🔴 驗簽 → 解析，順序不可換（陷阱 A2）。
     *
     * <p>先 parse 再 re-serialize 會因欄位順序不同而<b>偶發</b>失敗——失敗率取決於資料內容，
     * 測不出來、上線才炸。本方法是本 adapter 內<b>唯一</b>把 raw bytes 變成 JSON 的地方，
     * 而它一定先驗簽。
     */
    private JsonNode verifiedBody(InboundPayload raw) {
        if (!webhookVerifier.verify(raw.rawBody(), raw.headers())) {
            throw new UberWebhookSignatureException(
                    "Uber webhook 驗簽失敗（端點必須回 401，且不得寫入 webhook_inbox）");
        }
        try {
            return objectMapper.readTree(raw.rawBody());
        } catch (IOException e) {
            // 驗簽過了但內容不是 JSON ⇒ 對方送了我們看不懂的東西。
            // 🔴 不得回 4xx（Uber 不重試 4xx ⇒ 事件靜默丟失，66 §0.3），讓它往上成為 5xx。
            throw new IllegalStateException("Uber webhook 驗簽通過但 body 不是合法 JSON", e);
        }
    }

    // ==================================================================
    // 永久 empty 的三個 capability —— 🔴 這不是缺口
    // ==================================================================

    /** ❌ 永久 empty：即時配送無標籤概念。 */
    @Override
    public Optional<LabelCapability> label(FulfillmentContext ctx) {
        return Optional.empty();
    }

    /**
     * ❌ 永久 empty：逆向 leg 是<b>另開一筆</b> shipment ＋ 另一筆 {@code uber_deliveries}
     * （{@code is_return=true}），不是在同一單上加狀態（66 §7.3.3）。
     *
     * <p>⚠️ 66 §4.5.0 逐字：「{@code empty()} 是誠實標註不是設計完成」——
     * 逆向 leg 的 {@code canceled} 無對應 {@code AfterSaleCaseStatus}（open item UP-8），
     * 未關前逆向 leg 的處置仍是人工。
     */
    @Override
    public Optional<ReturnCapability> returns(FulfillmentContext ctx) {
        return Optional.empty();
    }

    /** ❌ 永久 empty：Uber 無門市概念（宅配到點）。 */
    @Override
    public Optional<StoreLocatorCapability> storeLocator(FulfillmentContext ctx) {
        return Optional.empty();
    }

    // ==================================================================
    // helpers
    // ==================================================================

    /**
     * 🔴 fail-closed（66 §8.5／§4.5.1）：總開關關著或憑證缺席時<b>不送出任何位元組</b>。
     *
     * <p>帶空字串上路的後果是一連串 401，而 401 對外看起來像<b>我們被停權</b>。
     */
    private void requireUsable() {
        if (!availability.usable()) {
            throw new ShippingBusinessException(GrpcResponseCode.FEATURE_SHIPPING_PROVIDER_DISABLED);
        }
    }

    /**
     * 取門市對應；查不到就拋。
     *
     * <p>⚠️ 僅供 {@code cancel}／{@code track}／{@code pod} 使用——那三條路徑沒有
     * 「把失敗寫進 entity 狀態」這個通道，例外才是它們唯一表達得出失敗的方式。
     * <b>建單路徑刻意不走這一支</b>（見 {@link #submitDelivery}）。
     */
    private UberStoreMappingEntity requireMapping(ShipmentEntity shipment) {
        long storeId = shipment.getStoreId() == null ? 0L : shipment.getStoreId();
        return storeResolver.activeMapping(storeId)
                .orElseThrow(() -> new ShippingBusinessException(
                        GrpcResponseCode.FULFILLMENT_PROVIDER_NOT_ENABLED));
    }

    private String requireDeliveryId(ShipmentEntity shipment) {
        String id = shipment.getUberDeliveryId();
        if (id == null || id.isBlank()) {
            throw new ShippingBusinessException(GrpcResponseCode.SHIPMENT_STATE_TRANSITION_REJECTED);
        }
        return id;
    }

    /**
     * Uber 的 provider 成本 ＝ <b>checkout 當下被接受的那一筆報價</b>的 {@code fee_amount}。
     *
     * <h3>為什麼取 quote 而不是向 Uber 回查</h3>
     * 官方 delivery 回應雖然有 {@code fee} 欄，但我方 {@code DeliveryResult} <b>刻意不解析它</b>；
     * 而取消費／等待費<b>根本不在 API 契約內</b>（只出現在月結帳單）。
     * ⇒ 今天唯一可靠、且與買家被收的運費同源的金額，就是被接受的那筆報價。
     *
     * <p>🔴 <b>這不是「猜」</b>：`INV-U1` 保證買家配送費在 capture 當下凍結、永不補收，
     * 而該值今天是 pass-through（repo 內查無 markup）⇒ 它就是我方付給 Uber 的預估金額。
     * 真實差額由 provider 月結帳單走 {@code actual_cost} true-up（70 §6.2），
     * 那是 estimated → actual 的正常路徑，不是缺陷。
     *
     * <p>⚠️ <b>單位已在 adapter 入站邊界換算成「元」</b>
     * （{@code UberApiClient} 解析 E1 回應時走 {@code UberFieldFormat.fromMinorUnits}）
     * ⇒ {@code uber_quotes.fee_amount} 存的是元整數，此處直接使用、<b>不得再換算一次</b>。
     *
     * <p>⚠️ 查無 {@code CHECKOUT_ACCEPTED} 報價 ⇒ 回 empty（<b>不是 0</b>）。
     * 那代表這條 leg 走的不是正常 checkout 路徑（例如人工補單），成本要由對帳補回，
     * 而不是用一個編出來的金額向賣家收錢。
     */
    @Override
    public Optional<CostCapability> cost(FulfillmentContext ctx) {
        return Optional.of(shipment -> {
            UberQuoteEntity quote = acceptedQuote(shipment.getOrderNo());
            if (quote == null || quote.getFeeAmount() == null) {
                return Optional.empty();
            }
            return Optional.of(quote.getFeeAmount());
        });
    }

    private UberQuoteEntity acceptedQuote(String orderNo) {
        if (orderNo == null || orderNo.isBlank()) {
            return null;
        }
        return quoteMapper.selectList(Wrappers.<UberQuoteEntity>lambdaQuery()
                        .eq(UberQuoteEntity::getOrderNo, orderNo)
                        .eq(UberQuoteEntity::getStatus, UberQuoteStatus.CHECKOUT_ACCEPTED)
                        .orderByDesc(UberQuoteEntity::getId))
                .stream()
                .findFirst()
                .orElse(null);
    }

    /**
     * 我方 pre-commit 鍵（65 {@code ADR-014}）：{@code uber_create_delivery:{order_no}:{leg}}。
     *
     * <p>🔴 <b>leg 取自我方的 {@code is_return}，不是 Uber 的 id 前綴</b>：
     * 前綴是 provider 的實作細節，它改了我們不會知道（66 §4.5.3／陷阱 A7）。
     */
    static String precommitKey(ShipmentEntity entity) {
        String leg = Boolean.TRUE.equals(entity.getIsReturn()) ? "return" : "forward";
        return PRECOMMIT_KEY_PREFIX + entity.getOrderNo() + ":" + leg;
    }

    /**
     * 取件時間。🔴 委派給 {@link UberFieldFormat#pickupReadyDt}——報價路徑用的是同一支，
     * 兩邊各算各的會讓報的價與實際建單的取件時間不一致。
     */
    private String pickupReadyDt(UberStoreMappingEntity mapping) {
        return UberFieldFormat.pickupReadyDt(Instant.now(clock), mapping.getPrepSlaMinutes());
    }

    private LocalDateTime nowUtc() {
        return LocalDateTime.ofInstant(Instant.now(clock), ZoneOffset.UTC);
    }

    /** 🔴 值語意一律 UTC（10 檔 §4）；null 不補現在時間（那會編一個假的 provider 時間）。 */
    private static LocalDateTime toLocalDateTime(Instant instant) {
        return instant == null ? null : LocalDateTime.ofInstant(instant, ZoneOffset.UTC);
    }

    private static String firstText(JsonNode node, String... fields) {
        for (String f : fields) {
            JsonNode v = node.path(f);
            if (!v.isMissingNode() && !v.isNull()) {
                String s = v.asText();
                if (s != null && !s.isBlank()) {
                    return s;
                }
            }
        }
        return null;
    }

    private static Instant parseEpochOrIso(String raw) {
        if (raw == null || raw.isBlank()) {
            return null;
        }
        try {
            return Instant.parse(raw);
        } catch (RuntimeException ignored) {
            // DaaS 的 created 與 dapi 的 event_time 都是 Unix 時間戳（總覽 §6.2／§7.2）。
        }
        try {
            long epoch = Long.parseLong(raw.trim());
            // 秒 vs 毫秒：總覽 §3.1 的 created_at 逐字是「Unix 毫秒」，而 webhook 的
            // event_time 是秒 ⇒ 以位數判別，不猜。
            return epoch > 99_999_999_999L ? Instant.ofEpochMilli(epoch) : Instant.ofEpochSecond(epoch);
        } catch (NumberFormatException ignored) {
            return null;
        }
    }

    private static String nullToEmpty(String v) {
        return v == null ? "" : v;
    }

    private static String sha256Prefix(byte[] body) {
        try {
            byte[] digest = MessageDigest.getInstance("SHA-256")
                    .digest(body == null ? new byte[0] : body);
            return HexFormat.of().formatHex(digest).substring(0, 16);
        } catch (NoSuchAlgorithmException e) {
            throw new IllegalStateException("SHA-256 不可用", e);
        }
    }

    /** 供測試與呼叫端組 UTF-8 raw body。 */
    static byte[] utf8(String s) {
        return s.getBytes(StandardCharsets.UTF_8);
    }
}
