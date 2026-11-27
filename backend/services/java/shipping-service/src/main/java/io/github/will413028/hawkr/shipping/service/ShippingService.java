package io.github.will413028.hawkr.shipping.service;

import com.baomidou.mybatisplus.core.conditions.query.LambdaQueryWrapper;
import io.github.will413028.hawkr.common.obs.log.ObsLogger;
import io.github.will413028.hawkr.common.providercall.ProviderCallContext;
import io.github.will413028.hawkr.common.providercall.ProviderCallExecutor;
import io.github.will413028.hawkr.common.utils.DateTimeUtils;
import io.github.will413028.hawkr.common.utils.code.AssetReferenceType;
import io.github.will413028.hawkr.common.utils.code.GrpcResponseCode;
import io.github.will413028.hawkr.common.utils.code.LabelPrintStatus;
import io.github.will413028.hawkr.common.utils.code.ShippingMethod;
import io.github.will413028.hawkr.common.utils.code.ShippingStatus;
import io.github.will413028.hawkr.common.utils.code.TcatCallStatus;
import io.github.will413028.hawkr.common.utils.code.helper.ShippingStatusRules;
import io.github.will413028.hawkr.shipping.exception.ProviderCreateFailedException;
import io.github.will413028.hawkr.shipping.exception.ShippingBusinessException;
import io.github.will413028.hawkr.order.grpc.GetOrderForShippingResponse;
import io.github.will413028.hawkr.order.grpc.OrderShippingInfo;
import io.github.will413028.hawkr.payuni.shipping.DeliveryTimeTag;
import io.github.will413028.hawkr.payuni.shipping.LgsType;
import io.github.will413028.hawkr.payuni.shipping.ServiceType;
import io.github.will413028.hawkr.payuni.shipping.ShipType;
import io.github.will413028.hawkr.shipping.client.BuyerGrpcClient;
import io.github.will413028.hawkr.shipping.client.MediaGrpcClient;
import io.github.will413028.hawkr.shipping.client.OrderGrpcClient;
import io.github.will413028.hawkr.shipping.client.SellerGrpcClient;
import io.github.will413028.hawkr.seller.grpc.BatchGetSellerMetaResponse;
import io.github.will413028.hawkr.seller.grpc.SellerMeta;
import io.github.will413028.hawkr.shipping.entity.SellerShipmentSettingEntity;
import io.github.will413028.hawkr.shipping.entity.ShipmentEntity;
import io.github.will413028.hawkr.shipping.entity.ShipmentStatusLogEntity;
import io.github.will413028.hawkr.shipping.mapper.ShipmentMapper;
import io.github.will413028.hawkr.shipping.mapper.ShipmentStatusLogMapper;
import io.github.will413028.hawkr.common.utils.code.ShipmentAnomalyType;
import io.github.will413028.hawkr.shipping.mq.ShipmentAnomalyEventPublisher;
import io.github.will413028.hawkr.shipping.mq.ShippingEventPublisher;
import io.github.will413028.hawkr.shipping.provider.FulfillmentContext;
import io.github.will413028.hawkr.common.utils.code.FulfillmentAction;
import io.github.will413028.hawkr.shipping.provider.FulfillmentProviderAdapter;
import io.github.will413028.hawkr.shipping.provider.capability.CreateCapability;
import io.github.will413028.hawkr.shipping.provider.capability.InboundCapability;
import io.github.will413028.hawkr.shipping.provider.capability.InboundPayload;
import io.github.will413028.hawkr.shipping.provider.action.AllowedActionResolver;
import io.github.will413028.hawkr.shipping.provider.action.FulfillmentActionPolicy;
import io.github.will413028.hawkr.shipping.provider.FulfillmentProviderRegistry;
import io.github.will413028.hawkr.shipping.provider.capability.CreateCapability;
import io.github.will413028.hawkr.shipping.provider.capability.DispatchCapability;
import io.github.will413028.hawkr.shipping.provider.capability.LabelCapability;
import io.github.will413028.hawkr.shipping.provider.capability.ReturnCapability;
import io.github.will413028.hawkr.shipping.provider.capability.TrackCapability;
import io.github.will413028.hawkr.shipping.service.dto.ShipmentPhotoPayload;
import io.github.will413028.hawkr.shipping.service.dto.ShippingEventPayload;
import org.springframework.stereotype.Service;
import org.springframework.transaction.PlatformTransactionManager;
import org.springframework.transaction.TransactionDefinition;
import org.springframework.transaction.annotation.Transactional;
import org.springframework.transaction.support.TransactionTemplate;

import java.time.LocalDate;
import java.time.LocalDateTime;
import java.time.format.DateTimeFormatter;
import java.util.List;
import java.util.Map;
import java.util.Optional;
import java.util.function.Function;
import java.util.stream.Collectors;

@Service
public class ShippingService {

    private static final ObsLogger log = ObsLogger.forClass(ShippingService.class);
    private static final String REF_TYPE_SHIPPING = AssetReferenceType.SHIPPING.value();
    private static final DateTimeFormatter ISO_FMT = DateTimeFormatter.ISO_LOCAL_DATE_TIME;
    private static final DateTimeFormatter DATE_FMT = DateTimeFormatter.ofPattern("yyyyMMdd");
    /** shipDate/deliveryDate 相容格式：yyyy-MM-dd（business_days 回傳格式 / 前端日曆元件慣用） */
    private static final DateTimeFormatter ISO_DATE_FMT = DateTimeFormatter.ISO_LOCAL_DATE;

    private final ShipmentMapper shipmentMapper;
    private final ShipmentStatusLogMapper statusLogMapper;
    private final OrderGrpcClient orderGrpcClient;
    private final BuyerGrpcClient buyerGrpcClient;
    private final MediaGrpcClient mediaGrpcClient;
    private final SellerGrpcClient sellerGrpcClient;
    private final ShipmentStatusMapper statusMapper;
    /**
     * 🔴 BLOCKER 3：執行面與契約面共用的判定（{@code allowedActions} 也是它算的）。
     * 注入它而不是複製一份判定邏輯，是這次修復的核心——兩份規則必然漂移，
     * 而漂移的症狀是「契約說不能按、端點照做」與「契約說能按、端點拒絕」各出現一次。
     */
    private final AllowedActionResolver allowedActionResolver;
    private final ShipmentLabelService labelService;
    private final ShippingEventPublisher eventPublisher;
    private final ShipmentAnomalyEventPublisher anomalyEventPublisher;
    private final OnboardingService onboardingService;
    private final HolidayService holidayService;
    private final CvsPrintWaitingRegistry cvsPrintWaitingRegistry;
    private final ProviderCallExecutor providerCallExecutor;

    /**
     * provider 能力的唯一問答點（65）。
     *
     * <p>🔴 本類別內<b>不得</b>再出現 {@code ShippingMethod.XXX.equals(...)} 形式的
     * provider 分支——那些判斷一律改成問這裡拿到的 adapter「這個 provider 有沒有這個能力」。
     * 差別不是風格：字面分支在新增第四個 provider 時要記得回來加一格，而漏加的症狀是
     * <b>靜默走進 PayUni 分支</b>，不是報錯。
     */
    private final FulfillmentProviderRegistry fulfillmentProviders;

    /**
     * 獨立事務模板 (PROPAGATION_REQUIRES_NEW)。
     * Why: 外層 @Transactional 在 PayUni 建單/呼叫失敗後會拋例外 rollback；failed shell / provider_status
     * 落地必須脫離外層事務，否則被一併 rollback 導致 Admin 異常清單永遠查不到。
     */
    private final TransactionTemplate requiresNewTx;

    /**
     * 「物流商回了看不懂的貨態」的計數器名稱 —— 告警 ShippingProviderStatusUnknown 的生產者。
     *
     * <p>Micrometer 會在 Prometheus 上補 {@code _total} 後綴；命名對齊本 repo 既有的
     * {@code hawkr_*} 命名空間（{@code hawkr_orders_created} 等）。
     */
    static final String METRIC_PROVIDER_STATUS_UNKNOWN = "hawkr_shipping_provider_status_unknown";

    /**
     * 「回調不帶貨態欄位」的計數器 —— <b>與上面那支刻意分開，不是同一支加 tag</b>。
     *
     * <p>🔴 分開的理由是 dev 實測出來的：PayUni 的建單結果回報只有 {@code TradeStatus}
     * 沒有 {@code ShipStatus}，它會走進同一個處置分支，但那是<b>正常流量</b>而非缺陷。
     * 兩者若共用一支 counter，告警的 {@code sum by (env, shipping_method)} 會跨 outcome
     * 加總 ⇒ <b>每次正常建單都讓告警觸發</b>。一條會在正常流量上響的告警，第二次就會被
     * 當成狼來了，那比沒有告警更糟。
     */
    static final String METRIC_NOT_A_STATUS_EVENT = "hawkr_shipping_provider_not_status_event";

    /**
     * PayUni 的「尚無資料」哨兵。
     *
     * <p>⚠️ 這是全 repo 第五處各自處理它（{@code DateTimeUtils} 的 javadoc 逐字列了前四處：
     * {@code PayUniCvsAdapter}／{@code PayUniHomeAdapter}／{@code RefundApplicationService}／
     * {@code RefundResultConsumer}，而該檔的常數是 private）。留在這裡是為了把本次修復的
     * 部署面收斂在 shipping-service 一支 image；DRY 的正解是把 DateTimeUtils 的那個常數
     * 開放出來，但那會讓 10 支 Java image 全部重建。
     */
    private static final String PAYUNI_DASH_PLACEHOLDER = "-";

    /** raw 值有內容、已落庫。 */
    static final String OUTCOME_PERSISTED = "persisted";
    /** raw 值有內容，但 UPDATE 影響 0 列（單子已被別的路徑改掉／id 對不上）。 */
    static final String OUTCOME_PERSIST_MISSED = "persist_missed";

    /**
     * 告警用 counter 的 shipping_method 值域 —— 啟動時預先展開成 0。
     *
     * <p>🔴 <b>不預先展開的話，第一次真的未知貨態不會告警</b>（dev 實測坐實）：
     * Micrometer 的 counter 在第一次觀測前不出現在 {@code /metrics} 上，序列從「不存在」
     * 跳到 1 <b>不算 increase</b>——{@code increase(...[1h])} 對只有一個定值取樣的序列恆為 0。
     * 於是每個 pod 生命週期內的第一筆會靜默，而「第一筆」正是最需要被看到的那一筆。
     * 展開 0 之後，0→1 是真的 increase。
     *
     * <p>基數：4 個配送方式 × 1 = 4 條恆存在的序列，遠低於 invoice 先例刻意避開的「數百條」。
     * {@code source} 與 {@code outcome} 刻意<b>不</b>放進 label——它們在 WARN log 裡，
     * 而告警需要的只是「哪條產品線開始出現看不懂的貨態」。
     */
    private static final java.util.List<String> ALERTABLE_SHIPPING_METHODS = java.util.List.of(
            ShippingMethod.TAQBIN.value(),
            ShippingMethod.SEVEN_ELEVEN_STORE_PICKUP.value(),
            ShippingMethod.UBER_INSTANT.value(),
            ShippingMethod.MERCHANT_DELIVERY.value());

    private final io.micrometer.core.instrument.MeterRegistry meterRegistry;

    public ShippingService(ShipmentMapper shipmentMapper,
                           ShipmentStatusLogMapper statusLogMapper,
                           OrderGrpcClient orderGrpcClient,
                           BuyerGrpcClient buyerGrpcClient,
                           MediaGrpcClient mediaGrpcClient,
                           SellerGrpcClient sellerGrpcClient,
                           ShipmentStatusMapper statusMapper,
                           ShipmentLabelService labelService,
                           ShippingEventPublisher eventPublisher,
                           ShipmentAnomalyEventPublisher anomalyEventPublisher,
                           OnboardingService onboardingService,
                           HolidayService holidayService,
                           CvsPrintWaitingRegistry cvsPrintWaitingRegistry,
                           ProviderCallExecutor providerCallExecutor,
                           PlatformTransactionManager transactionManager,
                           FulfillmentProviderRegistry fulfillmentProviders,
                           AllowedActionResolver allowedActionResolver,
                           io.micrometer.core.instrument.MeterRegistry meterRegistry) {
        this.shipmentMapper = shipmentMapper;
        this.statusLogMapper = statusLogMapper;
        this.orderGrpcClient = orderGrpcClient;
        this.buyerGrpcClient = buyerGrpcClient;
        this.mediaGrpcClient = mediaGrpcClient;
        this.sellerGrpcClient = sellerGrpcClient;
        this.statusMapper = statusMapper;
        this.allowedActionResolver = allowedActionResolver;
        this.labelService = labelService;
        this.eventPublisher = eventPublisher;
        this.anomalyEventPublisher = anomalyEventPublisher;
        this.onboardingService = onboardingService;
        this.holidayService = holidayService;
        this.cvsPrintWaitingRegistry = cvsPrintWaitingRegistry;
        this.providerCallExecutor = providerCallExecutor;
        this.fulfillmentProviders = fulfillmentProviders;
        this.meterRegistry = meterRegistry;
        for (String m : ALERTABLE_SHIPPING_METHODS) {
            io.micrometer.core.instrument.Counter.builder(METRIC_PROVIDER_STATUS_UNKNOWN)
                    .description("物流商回報無法辨識貨態的次數（R2；raw 值另落 provider_status，此處只計趨勢）")
                    .tag("shipping_method", m)
                    .register(meterRegistry)
                    .increment(0.0);
        }
        this.requiresNewTx = new TransactionTemplate(transactionManager);
        this.requiresNewTx.setPropagationBehavior(TransactionDefinition.PROPAGATION_REQUIRES_NEW);
    }


    /**
     * 解析 shipDate / deliveryDate，同時接受 {@code yyyy-MM-dd}（前端日曆元件 / business_days 回傳格式）
     * 與 {@code yyyyMMdd}（PayUni 慣用格式、舊版 App）。兩格式在字元長度與是否含連字號上無歧義，
     * 依外觀確定性判別，不做寬鬆猜測。
     * <p>
     * parse 失敗（含空值）一律包裝為 {@link GrpcResponseCode#SHIPMENT_DATE_INVALID_FORMAT} 業務碼，
     * 不得任由 {@link java.time.format.DateTimeParseException} 裸拋（issue #1491：generate_home_label
     * 的 shipDate 曾直接洩漏未包裝的 parse 例外訊息）。
     *
     * @param rawDate    原始輸入
     * @param fieldLabel 錯誤訊息用的欄位中文名（如「出貨日期」「配達日期」）
     * @return 解析後的 LocalDate
     */
    private LocalDate parseFlexibleDate(String rawDate, String fieldLabel) {
        if (rawDate == null || rawDate.isBlank()) {
            throw new ShippingBusinessException(GrpcResponseCode.SHIPMENT_DATE_INVALID_FORMAT, fieldLabel, "(空)");
        }
        try {
            if (rawDate.length() == 8 && rawDate.chars().allMatch(Character::isDigit)) {
                return LocalDate.parse(rawDate, DATE_FMT);
            }
            if (rawDate.length() == 10 && rawDate.charAt(4) == '-' && rawDate.charAt(7) == '-') {
                return LocalDate.parse(rawDate, ISO_DATE_FMT);
            }
            throw new java.time.format.DateTimeParseException("不支援的日期格式", rawDate, 0);
        } catch (java.time.format.DateTimeParseException e) {
            throw new ShippingBusinessException(GrpcResponseCode.SHIPMENT_DATE_INVALID_FORMAT, fieldLabel, rawDate);
        }
    }

    /** normalize 為 PayUni 邊界所需的 {@code yyyyMMdd}；內部 / 對外送出一律走此格式，不受前端輸入格式影響 */
    private String toPayUniDateFormat(LocalDate date) {
        return date.format(DATE_FMT);
    }


    /**
     * 背景自動建立物流單（order.paid MQ 事件觸發）
     *
     * @param orderNo 訂單編號
     * @return 已持久化的物流單
     */
    @Transactional
    public ShipmentEntity createShipmentFromOrder(String orderNo) {
        ShipmentEntity existing = getShipmentByOrderNo(orderNo);
        if (existing != null) {
            log.info("ShippingService.createShipmentFromOrder.idempotent",
                    "物流單已存在，跳過建單", "orderNo", orderNo);
            return existing;
        }

        GetOrderForShippingResponse orderResp = orderGrpcClient.getOrderForShipping(orderNo);
        if (!orderResp.hasData()) {
            throw new ShippingBusinessException(GrpcResponseCode.SHIPMENT_ORDER_QUERY_FAILED, "訂單資料缺失");
        }
        OrderShippingInfo orderInfo = orderResp.getData();

        String storeIdStr = orderInfo.getStoreId();
        if (storeIdStr == null || storeIdStr.isEmpty()) {
            throw new ShippingBusinessException(GrpcResponseCode.SHIPMENT_DATA_INCOMPLETE, "缺少賣家 store_id");
        }
        String buyerIdStr = orderInfo.getBuyerId();
        if (buyerIdStr == null || buyerIdStr.isEmpty()) {
            throw new ShippingBusinessException(GrpcResponseCode.SHIPMENT_DATA_INCOMPLETE, "缺少買家帳號");
        }
        String totalAmountStr = orderInfo.getTotalActualAmount();
        if (totalAmountStr == null || totalAmountStr.isEmpty()) {
            throw new ShippingBusinessException(GrpcResponseCode.SHIPMENT_DATA_INCOMPLETE, "缺少實付金額");
        }

        long storeId = Long.parseLong(storeIdStr);
        SellerShipmentSettingEntity onboardingSetting =
                onboardingService.findActiveSetting(storeId, orderInfo.getShippingMethod());

        String shippingMethod = orderInfo.getShippingMethod();
        if (requiresOnboarding(storeId, shippingMethod) && onboardingSetting == null) {
            log.error("noOnboarding",
                    "賣家尚未開通此配送方式，storeId=" + storeId,
                    "shippingMethod", shippingMethod);
            throw new ShippingBusinessException(GrpcResponseCode.SHIPMENT_SELLER_NOT_ONBOARDED, ShippingMethod.fromValue(shippingMethod).label());
        }

        CreateCapability createCapability = fulfillmentProviders.find(shippingMethod)
                .flatMap(adapter -> adapter.create(
                        FulfillmentContext.forStore(storeId, adapter.supportedMethod())))
                .orElseThrow(() -> new ShippingBusinessException(
                        GrpcResponseCode.SHIPMENT_UNSUPPORTED_METHOD,
                        safeShippingMethodLabel(shippingMethod)));

        ShipmentEntity entity;
        try {
            entity = createCapability.createFromOrder(orderNo, orderInfo, onboardingSetting);
        } catch (ProviderCreateFailedException e) {
            persistFailedShipmentShell(orderNo, orderInfo, shippingMethod, onboardingSetting, e);
            throw e;
        }

        entity.setSenderSnapshot(buildSenderSnapshot(onboardingSetting, orderInfo.getMerId()));
        entity.setReceiverSnapshot(buildReceiverSnapshot(orderInfo));

        shipmentMapper.insert(entity);

        insertStatusLog(entity.getId(), null, ShippingStatus.UNPROCESSED, "MQ");

        publishShipmentCreatedEvent(entity, orderInfo);

        log.info("ShippingService.createShipmentFromOrder.success",
                "自動建單完成，shipmentNo=" + entity.getShipmentNo(), "orderNo", orderNo);

        return entity;
    }


    /** 超商列印結果（含同步等待 Webhook 回填） */
    public record CvsLabelResult(String shipmentNo, String partnerId, String odno,
                                  String validationNo, boolean webhookReceived) {}

    private static final long CVS_PRINT_WAIT_SECONDS = 55;

    /**
     * 超商出貨單列印（觸發 PayUni 列印 + 同步等待 Webhook 最多 55 秒）
     * <p>
     * 首次列印：呼叫 PayUni 列印 API → 等待 Webhook → 推進訂單狀態為 SHIPPED(30)
     * 重複列印：僅更新出貨照片，不再呼叫 PayUni，直接回傳已有的交貨便代碼
     * <p>
     * 事務拆分：DB 更新先 commit，再進入等待階段，避免長時間佔用 DB 連線。
     */
    public CvsLabelResult printCvsLabel(long storeId, String orderNo, String shipDate) {
        onboardingService.requireActiveSetting(storeId, ShippingMethod.SEVEN_ELEVEN_STORE_PICKUP);

        ShipmentEntity entity = getAndVerifyShipment(orderNo, storeId);
        requireEndpointProvider(entity, ShippingMethod.SEVEN_ELEVEN_STORE_PICKUP);
        allowedActionResolver.require(FulfillmentAction.PRINT_LABEL, entity);

        if (entity.getShipTradeNo() == null || entity.getShipTradeNo().isBlank()) {
            throw new ShippingBusinessException(GrpcResponseCode.SHIPMENT_NO_TRADE_NO);
        }

        if (LabelPrintStatus.SUCCEEDED.value().equals(entity.getLabelPrintStatus())) {
            return handleReprintCvsLabel(entity, orderNo);
        }

        String normalizedShipDate = toPayUniDateFormat(parseFlexibleDate(shipDate, "出貨日期"));
        return handleFirstPrintCvsLabel(entity, orderNo, normalizedShipDate);
    }

    /**
     * 首次/重試超商列印：透過 ProviderCallExecutor 走「acquireRequested → invoke → markFailed」骨架。
     * markSucceeded 不在此處 — 由 PayUni Print Webhook（§5.6 processWebhookPrint）transition 4102→4103。
     */
    private CvsLabelResult handleFirstPrintCvsLabel(ShipmentEntity entity, String orderNo,
                                                      String shipDate) {
        var future = cvsPrintWaitingRegistry.register(entity.getShipTradeNo());
        try {
            providerCallExecutor.execute(
                    ProviderCallContext.of("printCvsLabel", entity.getId()),
                    () -> requiresNewTx.execute(s -> shipmentMapper.acquireLabelPrintRequested(entity.getId())) > 0,
                    () -> {
                        requireLabelCapability(entity).requestLabel(
                                entity, LabelCapability.LabelRequest.onShipDate(shipDate));
                        return null;
                    },
                    result -> {},
                    ex -> {
                        if (isTransientPayUniError(ex)) {
                            log.info("ShippingService.printCvsLabel.transientErrorKept",
                                    "PayUni 暫時性錯誤，保留 REQUESTED 等 §5.19.B reconciliation 補拉: " + ex.getMessage(),
                                    "orderNo", entity.getOrderNo());
                            return;
                        }
                        requiresNewTx.executeWithoutResult(s ->
                                shipmentMapper.markLabelPrintFailed(
                                        entity.getId(),
                                        extractProviderStatus(ex),
                                        extractProviderMessage(ex)));
                    }
            );
        } catch (io.github.will413028.hawkr.common.providercall.ProviderCallBusyException busy) {
            cvsPrintWaitingRegistry.waitAndRemove(entity.getShipTradeNo(), future, 0);
            throw new ShippingBusinessException(GrpcResponseCode.SHIPMENT_PROVIDER_CALL_IN_PROGRESS);
        } catch (io.github.will413028.hawkr.payuni.http.PayUniApiException papi) {
            cvsPrintWaitingRegistry.waitAndRemove(entity.getShipTradeNo(), future, 0);
            if (isTransientPayUniError(papi)) {
                throw new ShippingBusinessException(GrpcResponseCode.SHIPMENT_PROVIDER_CALL_IN_PROGRESS);
            }
            throw new ShippingBusinessException(GrpcResponseCode.SHIPMENT_PROVIDER_CALL_FAILED, papi.getMessage());
        } catch (RuntimeException e) {
            cvsPrintWaitingRegistry.waitAndRemove(entity.getShipTradeNo(), future, 0);
            throw e;
        }

        CvsPrintWaitingRegistry.CvsPrintResult result =
                cvsPrintWaitingRegistry.waitAndRemove(entity.getShipTradeNo(), future, CVS_PRINT_WAIT_SECONDS);

        if (result != null) {
            log.info("ShippingService.printCvsLabel.webhookReceived",
                    "Webhook 在等待期內到達", "orderNo", orderNo);
            return new CvsLabelResult(entity.getShipmentNo(),
                    result.partnerId(), result.odno(), result.validationNo(), true);
        } else {
            log.info("ShippingService.printCvsLabel.timeout",
                    "等待超時，交貨便代碼稍後由 Webhook 或 §5.19 reconciliation 回填", "orderNo", orderNo);
            return new CvsLabelResult(entity.getShipmentNo(), "", "", "", false);
        }
    }

    /**
     * 重複超商列印：直接回傳已有代碼
     */
    private CvsLabelResult handleReprintCvsLabel(ShipmentEntity entity, String orderNo) {
        log.info("ShippingService.printCvsLabel.reprint",
                "已列印過，直接回傳已有代碼", "orderNo", orderNo);

        String partnerId = entity.getPartnerId() != null ? entity.getPartnerId() : "";
        String odno = entity.getOdno() != null ? entity.getOdno() : "";
        String validationNo = entity.getValidationNo() != null ? entity.getValidationNo() : "";

        return new CvsLabelResult(entity.getShipmentNo(), partnerId, odno, validationNo, true);
    }

    /**
     * 從 PayUni 例外中萃取錯誤碼（無法萃取時回 ""）
     */
    private static String extractProviderStatus(Throwable ex) {
        if (ex instanceof io.github.will413028.hawkr.payuni.http.PayUniApiException papi) {
            return papi.getErrorCode() != null ? papi.getErrorCode() : "";
        }
        return "";
    }

    /**
     * 從 PayUni 例外中萃取人類可讀錯誤訊息
     */
    private static String extractProviderMessage(Throwable ex) {
        return ex.getMessage() != null ? ex.getMessage() : ex.toString();
    }

    /**
     * §2.4.4 transient vs permanent 判斷（純判定 helper，不決定 caller 處理方式）。
     * <p>傳回 true 代表此例外屬於 PayUni 後端 race / 網路抖動類暫時性錯誤
     * （PayUni 5xx / IO_ERROR / 「處理中」訊息 / busy / timeout）。
     * <p>caller 對 transient 的處理取決於該套用點補拉性質（§2.4.4 A/B/C 分類）：
     * - A 自動補拉（超商列印）→ keep REQUESTED 等 §5.19.3.B reconcile
     * - B 人工補拉（黑貓叫車 / 建店送 PayUni）→ keep REQUESTED 等 §5.19.3.C/D ops 告警
     * - C 無補拉（宅配 PDF）→ markFailed (4104) 讓賣家重打（O873490339 incident 教訓）
     * 與 OnboardingService 同步維護。
     */
    private static boolean isTransientPayUniError(Throwable ex) {
        if (!(ex instanceof io.github.will413028.hawkr.payuni.http.PayUniApiException papi)) return false;
        if ("IO_ERROR".equals(papi.getErrorCode())) return true;
        if (papi.getHttpStatus() >= 500) return true;
        String msg = papi.getMessage();
        if (msg == null) return false;
        String low = msg.toLowerCase();
        return msg.contains("處理中")
            || msg.contains("尚在處理")
            || low.contains("trying to access array offset")
            || low.contains("busy")
            || low.contains("timeout");
    }


    /**
     * 超商賣家確認出貨：更新狀態 921 + 處理出貨照片（選填）+ 發布事件
     * 前置條件：超商物流、label_printed=true、非終態
     */
    @Transactional
    public void confirmCvsShipment(long storeId, String orderNo, List<String> stagedTokens) {
        ShipmentEntity entity = getAndVerifyShipment(orderNo, storeId);

        if (ShippingStatus.SELLER_CONFIRMED.value().equals(entity.getStatus())) {
            log.info("ShippingService.confirmCvsShipment.idempotent",
                    "已確認出貨（冪等跳過）", "orderNo", orderNo);
            return;
        }

        allowedActionResolver.require(FulfillmentAction.HANDOVER, entity);

        if (!FulfillmentActionPolicy.isDomainApplicable(
                FulfillmentAction.HANDOVER, ShippingMethod.fromValue(entity.getShippingMethod()))) {
            throw new ShippingBusinessException(GrpcResponseCode.SHIPMENT_PROVIDER_MISMATCH);
        }
        if (!LabelPrintStatus.SUCCEEDED.value().equals(entity.getLabelPrintStatus())) {
            throw new ShippingBusinessException(GrpcResponseCode.SHIPMENT_HANDOVER_NOT_ALLOWED);
        }
        ShippingStatus currentStatus = ShippingStatus.fromValue(entity.getStatus());
        if (ShippingStatusRules.isTerminal(currentStatus)) {
            throw new ShippingBusinessException(GrpcResponseCode.SHIPMENT_STATE_TRANSITION_REJECTED);
        }

        processShipmentPhotos(entity, stagedTokens);

        int rows = shipmentMapper.updateShipmentStatus(
                entity.getId(),
                entity.getStatus(),
                ShippingStatus.SELLER_CONFIRMED.value(),
                null, null, null);

        if (rows == 0) {
            throw new ShippingBusinessException(GrpcResponseCode.SHIPMENT_CONCURRENT_CONFLICT);
        }

        if (entity.getShipmentPhotos() != null && !entity.getShipmentPhotos().isEmpty()) {
            shipmentMapper.updateShipmentPhotos(entity.getId(), entity.getShipmentPhotos());
        }

        insertStatusLog(entity.getId(), currentStatus, ShippingStatus.SELLER_CONFIRMED, "CONFIRM_CVS");

        log.info("ShippingService.confirmCvsShipment.success",
                "超商確認出貨完成，status=921(SELLER_CONFIRMED)", "orderNo", orderNo);

        publishShipmentPhotoEvent(entity);

        eventPublisher.publishShipmentStatusChanged(new ShippingEventPayload(
                entity.getOrderNo(), entity.getShipmentNo(), entity.getShippingMethod(),
                ShippingStatus.SELLER_CONFIRMED.value(), ShippingStatus.SELLER_CONFIRMED.label(),
                ShippingStatus.SELLER_CONFIRMED.label(),
                "", entity.getShipTradeNo(), "",
                false, null, null, entity.getBuyerId(), 0L
        ));
    }


    public record BatchPrintResult(int totalCount, int successCount, List<BatchPrintItemResult> results) {}
    public record BatchPrintItemResult(String orderNo, String shipmentNo, boolean success, String message) {}

    /**
     * 批次超商出貨單列印（部分成功模式）
     * <p>
     * 流程：批次查詢 → 逐筆驗證 → 跳過失敗的 → 依 (goodsType, lgsType, shipType) 分組 → 每組呼叫 PayUni → 批次更新 DB
     *
     * <p><b>刻意無 {@code @Transactional}</b>（與單筆路徑 {@link #generateHomeLabelPdf} 對齊）：
     * 本方法在 claim 之後要跨一次 PayUni 網路往返，包在單一交易裡等於<b>同時持有 N 列
     * {@code shipments} 的 row lock 橫跨第三方 HTTP</b>（《多副本排程搶佔規範》§一 ②
     * 反對的正是這個交易形狀）。單筆路徑早就用 {@code requiresNewTx} 把 claim／markFailed
     * 拆成各自的短交易，本方法過去是唯一的例外。
     *
     * <p>行為差異（刻意）：claim 現在逐筆立即提交，方法後段若拋例外不會把已 claim 的列
     * 撤回 4101——它們停在 REQUESTED(4102)，由 §5.19.B reconciliation 補拉，
     * 與 transient 分支本來就依賴的機制相同。
     */
    public BatchPrintResult batchPrintCvsLabel(long storeId, List<String> orderNos, String shipDate) {
        onboardingService.requireActiveSetting(storeId, ShippingMethod.SEVEN_ELEVEN_STORE_PICKUP);

        shipDate = toPayUniDateFormat(parseFlexibleDate(shipDate, "出貨日期"));

        List<String> uniqueOrderNos = orderNos.stream().distinct().toList();
        int totalCount = uniqueOrderNos.size();

        List<ShipmentEntity> entities = shipmentMapper.selectList(
                new LambdaQueryWrapper<ShipmentEntity>()
                        .in(ShipmentEntity::getOrderNo, uniqueOrderNos));
        Map<String, ShipmentEntity> entityMap = entities.stream()
                .collect(Collectors.toMap(ShipmentEntity::getOrderNo, Function.identity()));

        List<BatchPrintItemResult> results = new java.util.ArrayList<>();
        List<ShipmentEntity> validEntities = new java.util.ArrayList<>();

        for (String orderNo : uniqueOrderNos) {
            ShipmentEntity entity = entityMap.get(orderNo);
            if (entity == null) {
                results.add(new BatchPrintItemResult(orderNo, "", false, "物流單不存在"));
                continue;
            }
            if (!entity.getStoreId().equals(storeId)) {
                results.add(new BatchPrintItemResult(orderNo, "", false, "賣家帳號不符，無權操作此物流單"));
                continue;
            }
            try {
                allowedActionResolver.require(FulfillmentAction.PRINT_LABEL, entity);
            } catch (ShippingBusinessException e) {
                log.warn("ShippingService.batchPrintCvsLabel.rejected",
                        "本筆不符合列印條件，跳過（code=" + e.code() + "）",
                        "orderNo", entity.getOrderNo());
                continue;
            }
            if (entity.getShipTradeNo() == null || entity.getShipTradeNo().isBlank()) {
                results.add(new BatchPrintItemResult(orderNo, "", false, "物流單尚未取得 PayUni 序號，無法列印"));
                continue;
            }
            if (LabelPrintStatus.SUCCEEDED.value().equals(entity.getLabelPrintStatus())) {
                results.add(new BatchPrintItemResult(orderNo, entity.getShipmentNo(), false, "出貨單已列印，不可重複列印"));
                continue;
            }
            if (LabelPrintStatus.REQUESTED.value().equals(entity.getLabelPrintStatus())) {
                results.add(new BatchPrintItemResult(orderNo, entity.getShipmentNo(), false, "列印中，請稍候"));
                continue;
            }
            validEntities.add(entity);
        }

        if (validEntities.isEmpty()) {
            return new BatchPrintResult(totalCount, 0, results);
        }

        record PrintGroupKey(String goodsType, String lgsType, String shipType) {}
        Map<PrintGroupKey, List<ShipmentEntity>> groups = validEntities.stream()
                .collect(Collectors.groupingBy(e -> new PrintGroupKey(
                        e.getGoodsType(), e.getLgsType(), e.getShipType())));

        List<ShipmentEntity> acquiredEntities = new java.util.ArrayList<>();
        for (ShipmentEntity entity : validEntities) {
            if (requiresNewTx.execute(s -> shipmentMapper.acquireLabelPrintRequested(entity.getId())) > 0) {
                acquiredEntities.add(entity);
            } else {
                results.add(new BatchPrintItemResult(
                        entity.getOrderNo(), entity.getShipmentNo(), false, "列印中或已成功，跳過"));
            }
        }

        if (acquiredEntities.isEmpty()) {
            return new BatchPrintResult(totalCount, 0, results);
        }

        Map<PrintGroupKey, List<ShipmentEntity>> acquiredGroups = acquiredEntities.stream()
                .collect(Collectors.groupingBy(e -> new PrintGroupKey(
                        e.getGoodsType(), e.getLgsType(), e.getShipType())));

        int successCount = 0;
        for (Map.Entry<PrintGroupKey, List<ShipmentEntity>> entry : acquiredGroups.entrySet()) {
            PrintGroupKey key = entry.getKey();
            List<ShipmentEntity> groupEntities = entry.getValue();

            String batchShipTradeNos = groupEntities.stream()
                    .map(ShipmentEntity::getShipTradeNo)
                    .collect(Collectors.joining(","));

            try {
                requireLabelCapability(groupEntities.get(0)).requestLabelBatch(
                        groupEntities, LabelCapability.LabelRequest.onShipDate(shipDate));
                for (ShipmentEntity ge : groupEntities) {
                    results.add(new BatchPrintItemResult(ge.getOrderNo(), ge.getShipmentNo(), true, ""));
                    successCount++;
                }
            } catch (Exception e) {
                log.error("ShippingService.batchPrint.groupFailed",
                        "批次列印分組失敗: " + e.getMessage(), "shipTradeNos", batchShipTradeNos);
                String providerStatus = extractProviderStatus(e);
                String providerMessage = extractProviderMessage(e);
                boolean transient_ = isTransientPayUniError(e);
                if (transient_) {
                    log.info("ShippingService.batchPrint.transientErrorKept",
                            "批次列印 PayUni 暫時性錯誤，保留 REQUESTED 等 reconciliation: " + providerMessage,
                            "shipTradeNos", batchShipTradeNos);
                    for (ShipmentEntity ge : groupEntities) {
                        results.add(new BatchPrintItemResult(
                                ge.getOrderNo(), ge.getShipmentNo(), false, "處理中，請稍候刷新"));
                    }
                } else {
                    for (ShipmentEntity ge : groupEntities) {
                        requiresNewTx.executeWithoutResult(s ->
                                shipmentMapper.markLabelPrintFailed(ge.getId(), providerStatus, providerMessage));
                        results.add(new BatchPrintItemResult(
                                ge.getOrderNo(), ge.getShipmentNo(), false, "PayUni 呼叫失敗: " + providerMessage));
                    }
                }
            }
        }

        log.info("ShippingService.batchPrintCvsLabel.done",
                "批次列印完成 total=" + totalCount + " success=" + successCount);

        return new BatchPrintResult(totalCount, successCount, results);
    }

    /**
     * 宅配取號 + 下載 PDF + 上傳 OSS
     */
    @Transactional
    public String generateHomeLabelPdf(long storeId, String orderNo,
                                        String shipDate, String deliveryDate,
                                        List<String> stagedTokens) {
        onboardingService.requireActiveSetting(storeId, ShippingMethod.TAQBIN);

        ShipmentEntity entity = getAndVerifyShipment(orderNo, storeId);
        requireEndpointProvider(entity, ShippingMethod.TAQBIN);
        allowedActionResolver.require(FulfillmentAction.PRINT_LABEL, entity);

        if (entity.getShipTradeNo() == null || entity.getShipTradeNo().isBlank()) {
            throw new ShippingBusinessException(GrpcResponseCode.SHIPMENT_NO_TRADE_NO);
        }
        if (LabelPrintStatus.SUCCEEDED.value().equals(entity.getLabelPrintStatus())) {
            throw new ShippingBusinessException(GrpcResponseCode.SHIPMENT_ALREADY_LABELED);
        }


        LocalDate shipDateParsed = parseFlexibleDate(shipDate, "出貨日期");
        shipDate = toPayUniDateFormat(shipDateParsed);
        if (!shipDateParsed.isAfter(DateTimeUtils.todayInTaipei())) {
            throw new ShippingBusinessException(GrpcResponseCode.SHIPMENT_DATE_MUST_FUTURE);
        }
        String shipHolidayName = holidayService.getHolidayName(shipDateParsed);
        if (shipHolidayName != null) {
            throw new ShippingBusinessException(GrpcResponseCode.SHIPMENT_DATE_HOLIDAY, "出貨日期", shipDate, shipHolidayName);
        }
        List<LocalDate> allowedDays = holidayService.getNextBusinessDays(7);
        if (!allowedDays.isEmpty() && shipDateParsed.isAfter(allowedDays.get(allowedDays.size() - 1))) {
            throw new ShippingBusinessException(GrpcResponseCode.SHIPMENT_DATE_OUT_OF_RANGE, allowedDays.get(allowedDays.size() - 1).format(DATE_FMT));
        }

        LocalDate deliveryDateParsed;
        if (deliveryDate == null || deliveryDate.isBlank()) {
            deliveryDateParsed = holidayService.nextBusinessDay(shipDateParsed, 1);
            deliveryDate = deliveryDateParsed.format(DATE_FMT);
        } else {
            deliveryDateParsed = parseFlexibleDate(deliveryDate, "配達日期");
            deliveryDate = toPayUniDateFormat(deliveryDateParsed);
            if (!deliveryDateParsed.isAfter(shipDateParsed)) {
                throw new ShippingBusinessException(GrpcResponseCode.SHIPMENT_DELIVERY_BEFORE_SHIP);
            }
            String deliveryHolidayName = holidayService.getHolidayName(deliveryDateParsed);
            if (deliveryHolidayName != null) {
                throw new ShippingBusinessException(GrpcResponseCode.SHIPMENT_DATE_HOLIDAY, "配達日期", deliveryDate, deliveryHolidayName);
            }
        }

        final String shipDateFinal = shipDate;
        final String deliveryDateFinal = deliveryDate;
        LabelCapability.LabelRequestOutcome result;
        try {
            result = providerCallExecutor.execute(
                    ProviderCallContext.of("generateHomeLabelPdf", entity.getId()),
                    () -> requiresNewTx.execute(s -> shipmentMapper.acquireLabelPrintRequested(entity.getId())) > 0,
                    () -> requireLabelCapability(entity).requestLabel(
                            entity,
                            new LabelCapability.LabelRequest(shipDateFinal, deliveryDateFinal)),
                    r -> {
                        mediaGrpcClient.linkImages(REF_TYPE_SHIPPING,
                                String.valueOf(entity.getId()), List.of(String.valueOf(r.labelAssetId())));
                        requiresNewTx.executeWithoutResult(s -> {
                            int rows = shipmentMapper.markLabelPrintSucceededHome(
                                    entity.getId(), r.labelObjectKey(), r.labelAssetId());
                            if (rows == 0) {
                                throw new ShippingBusinessException(GrpcResponseCode.SHIPMENT_CONCURRENT_CONFLICT);
                            }
                        });
                    },
                    ex -> requiresNewTx.executeWithoutResult(s ->
                            shipmentMapper.markLabelPrintFailed(
                                    entity.getId(),
                                    extractProviderStatus(ex),
                                    extractProviderMessage(ex)))
            );
        } catch (io.github.will413028.hawkr.common.providercall.ProviderCallBusyException busy) {
            throw new ShippingBusinessException(GrpcResponseCode.SHIPMENT_PROVIDER_CALL_IN_PROGRESS);
        } catch (io.github.will413028.hawkr.payuni.http.PayUniApiException papi) {
            throw new ShippingBusinessException(GrpcResponseCode.SHIPMENT_PROVIDER_CALL_FAILED, papi.getMessage());
        }

        processShipmentPhotos(entity, stagedTokens);

        if (entity.getShipmentPhotos() != null && !entity.getShipmentPhotos().isEmpty()) {
            shipmentMapper.updateShipmentPhotos(entity.getId(), entity.getShipmentPhotos());
        }

        publishShipmentPhotoEvent(entity);

        LocalDateTime labelExpireAt = DateTimeUtils.nowUtc().plusHours(24);
        eventPublisher.publishShipmentStatusChanged(new ShippingEventPayload(
                entity.getOrderNo(), entity.getShipmentNo(), entity.getShippingMethod(),
                ShippingStatus.UNPROCESSED.value(), ShippingStatus.UNPROCESSED.label(),
                ShippingStatus.UNPROCESSED.label(),
                result.labelObjectKey(), entity.getShipTradeNo(),
                labelExpireAt.toString(),
                false, null, null, entity.getBuyerId(), 0L
        ));

        log.info("ShippingService.generateHomeLabelPdf.success",
                "宅配取號 PDF 完成", "orderNo", orderNo);

        return result.labelObjectKey();
    }

    /**
     * 呼叫黑貓預約取件。
     *
     * <p>走 §2.4 ProviderCallExecutor：acquire (4201/4204→4202) → invoke → markSucceeded / markFailed。
     *
     * <h3>🔴 provider 呼叫已搬進 adapter（{@code DispatchCapability.dispatch}）</h3>
     * 本方法<b>不再</b>組裝 PayUni 請求，也不再持有 {@code ShippingClient}。留在這裡的是平台流程：
     * <ul>
     *   <li>前置守衛（標籤已取號、未重複叫件）與賣家設定有效性</li>
     *   <li>{@code ProviderCallExecutor} 搶佔與 4202／4204 的狀態記帳</li>
     *   <li>狀態轉移（→ 210 {@code TCAT_CALLED}）、狀態歷程、事件發布</li>
     *   <li>{@code expected_delivery_date} 估算（§5.21.8，平台的業務規則）</li>
     * </ul>
     *
     * <p>⚠️ 預設值（電聯 Y／推車 N）留在這裡而不是 adapter：它們會被寫進我方的
     * {@code shipments.is_contact}／{@code shipments.is_trolley} 欄位（acquire 那一步），
     * adapter 代填會讓 DB 記的與實際送出的不一致。
     */
    public String callTcat(long storeId, String orderNo,
                            String isContact, String isTrolley, String memo) {
        ShipmentEntity entity = getAndVerifyShipment(orderNo, storeId);

        if (TcatCallStatus.SUCCEEDED.value().equals(entity.getTcatCallStatus())) {
            throw new ShippingBusinessException(GrpcResponseCode.SHIPMENT_TCAT_ALREADY_CALLED);
        }
        allowedActionResolver.require(FulfillmentAction.DISPATCH, entity);

        if (!LabelPrintStatus.SUCCEEDED.value().equals(entity.getLabelPrintStatus())) {
            throw new ShippingBusinessException(GrpcResponseCode.SHIPMENT_LABEL_REQUIRED);
        }

        final String resolvedIsContact = isContact != null && !isContact.isBlank() ? isContact : "Y";
        final String resolvedIsTrolley = isTrolley != null && !isTrolley.isBlank() ? isTrolley : "N";

        SellerShipmentSettingEntity setting =
                onboardingService.requireActiveSetting(storeId, ShippingMethod.TAQBIN);

        final DispatchCapability dispatchCapability = requireDispatchCapability(entity);
        final DispatchCapability.DispatchOptions dispatchOptions =
                new DispatchCapability.DispatchOptions(
                        "Y".equals(resolvedIsContact), "Y".equals(resolvedIsTrolley), memo);

        DispatchCapability.DispatchOutcome resp;
        try {
            resp = providerCallExecutor.execute(
                    ProviderCallContext.of("callTcat", entity.getId()),
                    () -> requiresNewTx.execute(s -> shipmentMapper.acquireTcatCallRequested(
                            entity.getId(), resolvedIsContact, resolvedIsTrolley, memo)) > 0,
                    () -> dispatchCapability.dispatch(entity, setting, dispatchOptions),
                    r -> requiresNewTx.executeWithoutResult(s -> {
                        LocalDateTime callTimeUtc = DateTimeUtils.nowUtc();
                        int rows = shipmentMapper.markTcatCallSucceeded(
                                entity.getId(), callTimeUtc, ShippingStatus.TCAT_CALLED.value());
                        if (rows == 0) {
                            throw new ShippingBusinessException(GrpcResponseCode.SHIPMENT_CONCURRENT_CONFLICT);
                        }
                    }),
                    ex -> {
                        if (isTransientPayUniError(ex)) {
                            log.info("ShippingService.callTcat.transientErrorKept",
                                    "PayUni 暫時性錯誤，保留 REQUESTED 給 §5.19.3.C ops 告警: " + ex.getMessage(),
                                    "orderNo", entity.getOrderNo());
                            return;
                        }
                        requiresNewTx.executeWithoutResult(s ->
                                shipmentMapper.markTcatCallFailed(
                                        entity.getId(),
                                        extractProviderStatus(ex),
                                        extractProviderMessage(ex)));
                    }
            );
        } catch (io.github.will413028.hawkr.common.providercall.ProviderCallBusyException busy) {
            throw new ShippingBusinessException(GrpcResponseCode.SHIPMENT_PROVIDER_CALL_IN_PROGRESS);
        } catch (io.github.will413028.hawkr.payuni.http.PayUniApiException papi) {
            if (isTransientPayUniError(papi)) {
                throw new ShippingBusinessException(GrpcResponseCode.SHIPMENT_PROVIDER_CALL_IN_PROGRESS);
            }
            throw new ShippingBusinessException(GrpcResponseCode.SHIPMENT_TCAT_CALL_FAILED, papi.getMessage());
        }

        ShippingStatus fromStatus = ShippingStatus.fromValue(entity.getStatus());
        insertStatusLog(entity.getId(), fromStatus, ShippingStatus.TCAT_CALLED, "CALL_TCAT");
        String labelKey = labelKeyOf(entity);
        eventPublisher.publishShipmentStatusChanged(new ShippingEventPayload(
                entity.getOrderNo(), entity.getShipmentNo(), entity.getShippingMethod(),
                ShippingStatus.TCAT_CALLED.value(), ShippingStatus.TCAT_CALLED.label(),
                ShippingStatus.TCAT_CALLED.label(),
                labelKey, entity.getShipTradeNo(),
                entity.getLabelExpireAt() != null ? entity.getLabelExpireAt().toString() : "",
                false, null, null, entity.getBuyerId(), 0L
        ));

        eventPublisher.publishProcessingCleared(entity.getOrderNo());

        try {
            java.time.LocalDate baseDate = DateTimeUtils.nowUtc().toLocalDate();
            java.time.LocalDate expectedDate = holidayService.nextBusinessDay(baseDate, 3);
            shipmentMapper.setExpectedDeliveryDate(entity.getId(), expectedDate);
        } catch (Exception e) {
            log.warn("ShippingService.callTcat.expectedDeliveryDate.failed",
                    "寫入 expected_delivery_date 失敗 orderNo=" + orderNo + " err=" + e.getMessage());
        }

        log.info("ShippingService.callTcat.success",
                "呼叫黑貓預約取件成功，status=210(TCAT_CALLED)", "orderNo", orderNo);

        return resp.dispatchedAt();
    }



    /** 退貨物流建單結果（shipTradeNo 供 AfterSales 快照使用，§5.3.6） */
    public record ReturnShipmentResult(String shipmentNo, String trackingCode,
                                        String returnStoreCode, String returnStoreName,
                                        String shipTradeNo) {}

    /**
     * 建立超商 C2C 退貨物流單。
     *
     * <p>角色對調：買家為寄件人，賣家退貨門市為取貨門市。
     *
     * <h2>🔴 provider 呼叫與 entity 組裝都在 {@link ReturnCapability} 內</h2>
     * 這一支是 active production flow（aftersales 每筆超商退貨都會走），而它遷移前
     * <b>完全繞過任何抽象</b>：就地 new PayUni request、就地填 {@code LgsType}／{@code ShipType}，
     * 而且成功列與失敗殼各有一份約 15 個 setter 的副本。
     * 本方法現在只負責<b>平台側</b>的事：查設定、產單號、落地、狀態歷程、排非同步列印。
     */
    @Transactional
    public ReturnShipmentResult createReturnCvsShipment(String orderNo, long storeId,
                                                          String merId, String buyerName,
                                                          String buyerPhone, String temperature,
                                                          long tradeAmt) {
        SellerShipmentSettingEntity setting =
                onboardingService.findSettingByMethod(storeId, ShippingMethod.SEVEN_ELEVEN_STORE_PICKUP.value());
        if (setting == null) {
            throw new ShippingBusinessException(GrpcResponseCode.SHIPMENT_DATA_INCOMPLETE, "找不到賣家 7-11 退貨設定");
        }
        if (setting.getReturnStoreCode() == null || setting.getReturnStoreCode().isBlank()) {
            throw new IllegalArgumentException("賣家尚未設定退貨門市");
        }

        ReturnCapability.ReturnRequest request = new ReturnCapability.ReturnRequest(
                orderNo, storeId, merId, generateReturnShipmentNo(),
                buyerName, buyerPhone,
                null, null, null, null,
                temperature, tradeAmt);

        ReturnCapability.ReturnOutcome outcome = requireReturnCapability(
                ShippingMethod.SEVEN_ELEVEN_STORE_PICKUP, storeId)
                .createReturnShipment(null, setting, request);

        ShipmentEntity entity = outcome.shipment();
        entity.setId(io.github.will413028.hawkr.common.utils.TsidUtil.generate());

        if (!outcome.accepted()) {
            log.error("ShippingService.createReturnCvsShipment.error",
                    "PayUni 退貨超商建單 retry 後仍失敗: " + outcome.providerMessage());
            persistFailedReturnShell(entity, "RETURN_CVS_FAILED", orderNo);
            throw new ShippingBusinessException(GrpcResponseCode.RETURN_SHIPMENT_CREATE_FAILED,
                    outcome.providerMessage());
        }

        shipmentMapper.insert(entity);
        insertStatusLog(entity.getId(), null, ShippingStatus.UNPROCESSED, "RETURN_CVS");

        log.info("ShippingService.createReturnCvsShipment.success",
                "退貨超商物流單建立完成 orderNo=" + orderNo + " shipmentNo=" + entity.getShipmentNo());

        scheduleAsyncCvsPrint(entity);

        return new ReturnShipmentResult(entity.getShipmentNo(), entity.getShipTradeNo(),
                setting.getReturnStoreCode(), setting.getReturnStoreName(), entity.getShipTradeNo());
    }

    /**
     * 非同步觸發 PayUni 超商列印（§5.3.6）。事務 commit 後才執行。
     * 失敗僅記 error log，不中斷主流程（Worker 會在 5 分鐘後自動重試一次，見 §5.3.7）。
     */
    private void scheduleAsyncCvsPrint(ShipmentEntity printTarget) {
        final String orderNo = printTarget.getOrderNo();
        final String shipTradeNo = printTarget.getShipTradeNo();
        String shipDate = java.time.LocalDate.now().format(DATE_FMT);
        org.springframework.transaction.support.TransactionSynchronizationManager.registerSynchronization(
                new org.springframework.transaction.support.TransactionSynchronization() {
                    @Override
                    public void afterCommit() {
                        java.util.concurrent.CompletableFuture.runAsync(() -> {
                            try {
                                requireLabelCapability(printTarget).requestLabel(
                                        printTarget, LabelCapability.LabelRequest.onShipDate(shipDate));
                                log.info("ShippingService.asyncReturnPrint.triggered",
                                        "退貨列印非同步觸發成功 orderNo=" + orderNo
                                                + " shipTradeNo=" + shipTradeNo);
                            } catch (Exception e) {
                                log.error("ShippingService.asyncReturnPrint.failed",
                                        "退貨列印非同步觸發失敗 orderNo=" + orderNo
                                                + " shipTradeNo=" + shipTradeNo
                                                + " 5 分鐘後 Worker 將自動重試", e);
                            }
                        });
                    }
                });
    }

    /**
     * 退貨出貨便列印重試（§5.3.7，供 AfterSales Worker / ops retry API 呼叫）。
     * 以 shipTradeNo 查 shipments，驗證為退貨單後非同步觸發 PayUni print_label，不等 webhook。
     */
    public void retryReturnPrintLabel(String shipTradeNo) {
        ShipmentEntity entity = shipmentMapper.selectOne(
                new LambdaQueryWrapper<ShipmentEntity>()
                        .eq(ShipmentEntity::getShipTradeNo, shipTradeNo));
        if (entity == null) {
            throw new ShippingBusinessException(GrpcResponseCode.SHIPMENT_NOT_FOUND);
        }
        if (!Boolean.TRUE.equals(entity.getIsReturn())) {
            throw new ShippingBusinessException(GrpcResponseCode.SHIPMENT_NOT_A_RETURN);
        }

        String shipDate = java.time.LocalDate.now().format(DATE_FMT);
        java.util.concurrent.CompletableFuture.runAsync(() -> {
            try {
                requireLabelCapability(entity).requestLabel(
                        entity, LabelCapability.LabelRequest.onShipDate(shipDate));
                log.info("ShippingService.retryReturnPrintLabel.triggered",
                        "退貨列印重試觸發成功 shipTradeNo=" + shipTradeNo);
            } catch (Exception e) {
                log.error("ShippingService.retryReturnPrintLabel.failed",
                        "退貨列印重試觸發失敗 shipTradeNo=" + shipTradeNo, e);
            }
        });
    }

    /**
     * 建立宅配退貨物流單（PayUni {@code /api/home_delivery/refund}）。
     *
     * <p>退貨人＝買家（司機到府取件），收件人＝賣家退貨聯絡人。
     *
     * <h2>🔴 provider 呼叫與 entity 組裝都在 {@link ReturnCapability} 內</h2>
     * 取件日推算（D+2、週日順延）、地址正規化、規格沿用原單——全部是 provider 側的參數組裝，
     * 遷移前就地寫在本方法裡。本方法現在只負責查設定、查原單、落地、狀態歷程。
     */
    @Transactional
    public ReturnShipmentResult createReturnHomeShipment(String orderNo, long storeId,
                                                           String merId, String buyerName,
                                                           String buyerPhone, String buyerCity,
                                                           String buyerDistrict, String buyerAddress,
                                                           String buyerPostalCode, String temperature) {
        SellerShipmentSettingEntity setting =
                onboardingService.findSettingByMethod(storeId, ShippingMethod.TAQBIN.value());
        if (setting == null) {
            throw new ShippingBusinessException(GrpcResponseCode.SHIPMENT_DATA_INCOMPLETE, "找不到賣家宅配退貨設定");
        }

        ShipmentEntity forwardShipment = shipmentMapper.selectOne(
                new LambdaQueryWrapper<ShipmentEntity>()
                        .eq(ShipmentEntity::getOrderNo, orderNo)
                        .eq(ShipmentEntity::getIsReturn, false)
                        .last("LIMIT 1"));

        ReturnCapability.ReturnRequest request = new ReturnCapability.ReturnRequest(
                orderNo, storeId, merId, generateReturnShipmentNo(),
                buyerName, buyerPhone,
                buyerCity, buyerDistrict, buyerAddress, buyerPostalCode,
                temperature, 0L);

        ReturnCapability.ReturnOutcome outcome =
                requireReturnCapability(ShippingMethod.TAQBIN, storeId)
                        .createReturnShipment(forwardShipment, setting, request);

        ShipmentEntity entity = outcome.shipment();
        entity.setId(io.github.will413028.hawkr.common.utils.TsidUtil.generate());

        if (!outcome.accepted()) {
            log.error("ShippingService.createReturnHomeShipment.error",
                    "PayUni 退貨宅配建單 retry 後仍失敗: " + outcome.providerMessage());
            persistFailedReturnShell(entity, "RETURN_HOME_FAILED", orderNo);
            throw new ShippingBusinessException(GrpcResponseCode.RETURN_SHIPMENT_CREATE_FAILED,
                    outcome.providerMessage());
        }

        shipmentMapper.insert(entity);
        insertStatusLog(entity.getId(), null, ShippingStatus.UNPROCESSED, "RETURN_HOME");

        log.info("ShippingService.createReturnHomeShipment.success",
                "退貨宅配物流單建立完成 orderNo=" + orderNo + " shipmentNo=" + entity.getShipmentNo());

        return new ReturnShipmentResult(entity.getShipmentNo(), entity.getShipTradeNo(), null, null,
                entity.getShipTradeNo());
    }

    /**
     * 查詢賣家退貨設定（讀路徑語意，不過濾 onboarding_status / enabled — 見 06 設計書 §2.3.1）
     */
    public SellerShipmentSettingEntity getSellerReturnConfig(long storeId, String shippingType) {
        return onboardingService.findSettingByMethod(
                storeId, methodOfShippingType(storeId, shippingType));
    }

    /**
     * 建店槽位分類 → 配送方式：{@link io.github.will413028.hawkr.shipping.provider.capability.OnboardCapability#shippingTypeValue()}
     * 的<b>反向查表</b>。
     *
     * <p>🔴 舊實作是一段寫死的 {@code CVS_C2C→3002 / HOME_DELIVERY→3001} if-else，
     * 那是 {@code shippingTypeValue()} 的<b>第二份真實來源</b>：新增 provider 時兩邊都要改，
     * 而漏改這一邊的症狀是 {@code IllegalArgumentException}（500），不是可讀的業務錯誤。
     * 改成問 registry 之後，唯一的來源是各 adapter 自己宣告的那一格。
     *
     * <p>⚠️ 找不到仍然拋 {@code IllegalArgumentException} ＋ 同一句訊息（行為不變）。
     */
    private String methodOfShippingType(long storeId, String shippingType) {
        for (String methodValue : fulfillmentProviders.registeredMethodValues()) {
            FulfillmentProviderAdapter adapter = fulfillmentProviders.require(methodValue);
            String candidate = adapter
                    .onboard(FulfillmentContext.forStore(storeId, adapter.supportedMethod()))
                    .map(io.github.will413028.hawkr.shipping.provider.capability.OnboardCapability::shippingTypeValue)
                    .orElse(null);
            if (candidate != null && candidate.equals(shippingType)) {
                return methodValue;
            }
        }
        throw new IllegalArgumentException("不支援的 shipping_type: " + shippingType);
    }

    private String generateReturnShipmentNo() {
        long seq = io.github.will413028.hawkr.common.utils.TsidUtil.generate() % 1_000_000_000L;
        return "RS" + String.format("%09d", seq);
    }

    /**
     * 按訂單號查詢物流單
     */
    public ShipmentEntity getShipmentByOrderNo(String orderNo) {
        return shipmentMapper.selectOne(
                new LambdaQueryWrapper<ShipmentEntity>()
                        .eq(ShipmentEntity::getOrderNo, orderNo));
    }

    /**
     * 批次按訂單號查詢物流單（Order Service 賣家列表頁用，避免 N+1）
     */
    public java.util.List<ShipmentEntity> getShipmentsByOrderNos(java.util.List<String> orderNos) {
        if (orderNos == null || orderNos.isEmpty()) {
            return java.util.Collections.emptyList();
        }
        return shipmentMapper.selectList(
                new LambdaQueryWrapper<ShipmentEntity>()
                        .in(ShipmentEntity::getOrderNo, orderNos));
    }

    /**
     * 按訂單號 + 正向/退貨查詢物流單（Webhook 模擬器用）
     */
    public ShipmentEntity lookupShipment(String orderNo, boolean isReturn) {
        return shipmentMapper.selectOne(
                new LambdaQueryWrapper<ShipmentEntity>()
                        .eq(ShipmentEntity::getOrderNo, orderNo)
                        .eq(ShipmentEntity::getIsReturn, isReturn)
                        .isNotNull(ShipmentEntity::getShipTradeNo)
                        .orderByDesc(ShipmentEntity::getCreatedAt)
                        .last("LIMIT 1"));
    }

    /**
     * 主動查詢 PayUni 物流狀態並同步
     *
     * 無 @Transactional：與 §5.5 processWebhookStatusUpdate 對齊，依賴 updateShipmentStatus 的 SQL WHERE 樂觀鎖保護。
     * 不能加 @Transactional，否則 PayUni 查詢例外會 rollback finally 內的 touchLastReconciledAt，導致退避失效。
     */
    public ShipmentEntity queryAndSyncStatus(String shipmentNo) {
        ShipmentEntity entity = shipmentMapper.selectOne(
                new LambdaQueryWrapper<ShipmentEntity>()
                        .eq(ShipmentEntity::getShipmentNo, shipmentNo));

        if (entity == null) {
            throw new ShippingBusinessException(GrpcResponseCode.SHIPMENT_NOT_FOUND);
        }

        return syncAndTouch(entity);
    }

    /**
     * 買家查看訂單物流追蹤（§5.7.1）
     *
     * 以 order_no 為入口（買家資源），驗 buyer ownership 後即時同步 PayUni 當前狀態。
     * 與 {@link #queryAndSyncStatus(String)} 共用 {@link #syncAndTouch(ShipmentEntity)} 核心，差別僅在入口 key 與 ownership。
     *
     * @param orderNo 訂單編號
     * @param buyerId Gateway 已解析的現任買家 buyer_id
     */
    public ShipmentEntity getBuyerTrackingByOrderNo(String orderNo, Long buyerId) {
        ShipmentEntity entity = shipmentMapper.selectOne(
                new LambdaQueryWrapper<ShipmentEntity>()
                        .eq(ShipmentEntity::getOrderNo, orderNo)
                        .eq(ShipmentEntity::getIsReturn, false));

        if (entity == null) {
            throw new ShippingBusinessException(GrpcResponseCode.SHIPMENT_NOT_FOUND);
        }

        if (buyerId == null || !buyerId.equals(entity.getBuyerId())) {
            throw new ShippingBusinessException(GrpcResponseCode.SHIPMENT_PERMISSION_DENIED);
        }

        return syncAndTouch(entity);
    }

    /**
     * 賣家查看訂單物流追蹤（§5.7.2）
     *
     * 與 {@link #getBuyerTrackingByOrderNo(String, Long)} 同構，差別僅 ownership 欄位（seller）與 is_return 可變。
     *
     * @param orderNo         訂單編號
     * @param storeId         Gateway JWT 注入的賣家帳號
     * @param isReturn        false=正向單 / true=退貨單
     */
    public ShipmentEntity getSellerTrackingByOrderNo(String orderNo, Long storeId, boolean isReturn) {
        ShipmentEntity entity = shipmentMapper.selectOne(
                new LambdaQueryWrapper<ShipmentEntity>()
                        .eq(ShipmentEntity::getOrderNo, orderNo)
                        .eq(ShipmentEntity::getIsReturn, isReturn));

        if (entity == null) {
            throw new ShippingBusinessException(GrpcResponseCode.SHIPMENT_NOT_FOUND);
        }

        if (storeId == null || !storeId.equals(entity.getStoreId())) {
            throw new ShippingBusinessException(GrpcResponseCode.SHIPMENT_PERMISSION_DENIED);
        }

        return syncAndTouch(entity);
    }

    /**
     * 同步核心：即時向 PayUni 查詢當前狀態並同步 DB（§5.7）。
     *
     * 無 @Transactional：與 §5.5 processWebhookStatusUpdate 對齊，依賴 updateShipmentStatus 的 SQL WHERE 樂觀鎖保護。
     * 不能加 @Transactional，否則 PayUni 查詢例外會 rollback finally 內的 touchLastReconciledAt，導致退避失效。
     *
     * §5.7 永不被退避擋下：按需查詢必須永遠強制 fresh（補救/即時展示皆不能回 cached）。
     * last_reconciled_at 由 finally 區塊更新，唯一作用是讓 §5.19 scheduler 在 backoff 內不重複工作。
     * PayUni 配額保護由 Gateway rate limit 處理，不在 service 層做 freshness gate。
     */
    private ShipmentEntity syncAndTouch(ShipmentEntity entity) {
        Optional<TrackCapability> track = fulfillmentProviders.find(entity.getShippingMethod())
                .flatMap(adapter -> adapter.track(FulfillmentContext.forShipment(entity)));
        if (track.isEmpty()) {
            return entity;
        }

        if (entity.getShipTradeNo() == null || entity.getShipTradeNo().isBlank()) {
            return entity;
        }

        try {
            TrackCapability.TrackResult result = track.get().query(entity);

            if (result.normalizedStatus() == null) {
                if (result.providerRawStatus() == null) {
                    log.warn("ShippingService.queryStatus.error",
                            "PayUni 查詢失敗: " + result.providerMessage());
                } else {
                    persistUnknownProviderStatus(entity, result.providerRawStatus(),
                            result.providerMessage(), "QUERY");
                }
                return entity;
            }

            ShippingStatus newStatus = ShippingStatus.fromValue(result.normalizedStatus());

            ShippingStatus currentStatus = ShippingStatus.fromValue(entity.getStatus());
            if (statusMapper.isValidTransition(currentStatus, newStatus)) {
                updateStatusWithParsedTime(entity, currentStatus, newStatus,
                        result.providerRawStatus(), null, result.providerStatusAt(), "QUERY");
            }

            return entity;
        } finally {
            shipmentMapper.touchLastReconciledAt(entity.getId(), DateTimeUtils.nowUtc());
        }
    }

    /**
     * <b>canonical provider callback ingress</b>——回調的解析與去重鍵的唯一來源。
     *
     * <h2>🔴 BLOCKER 4：這支方法存在的理由</h2>
     * 修復前，{@code ShippingWebhookConsumer} 自己讀 PayUni 欄名（{@code ShipTradeNo}／
     * {@code ApiType}／{@code ShipStatus}／{@code PartnerId}…）、自己組去重鍵，
     * 而 adapter 的 {@code InboundCapability} 是<b>零呼叫端的死碼</b>
     * ⇒ 同一份 provider 知識存在兩處，且兩處已經不等價
     * （consumer 的 Print 去重鍵是 {@code MerID:ShipTradeNo:Print}，
     *   adapter 的是讀 {@code ShipStatus} 而 Print payload 沒有那個欄位）。
     *
     * <p>設計書逐字說 {@code inbound()} 是「未知 provider 原生值 fail-closed」的<b>唯一落點</b>。
     * 兩份 SSOT 的實際代價是：第二個 provider 進來時，consumer 裡會長出第一個 {@code if}。
     *
     * <p>⚠️ legacy 相容端點（{@code processWebhookStatusUpdate} 等）保留，
     * 但它們進來之後匯流到同一條路徑，不另立第二份 status processing。
     *
     * @return 該筆配送單的 provider inbound 能力；查無配送單或該 provider 沒有回調能力時 empty
     */
    public Optional<InboundCapability> inboundFor(String shipTradeNo) {
        ShipmentEntity entity = findByShipTradeNo(shipTradeNo);
        if (entity == null) {
            return Optional.empty();
        }
        return fulfillmentProviders.find(entity.getShippingMethod())
                .flatMap(adapter -> adapter.inbound(FulfillmentContext.forShipment(entity)));
    }

    /** 依 provider 單號取配送單；查無回 null（呼叫端各自決定 ACK 或告警）。 */
    public ShipmentEntity findByShipTradeNo(String shipTradeNo) {
        return shipmentMapper.selectOne(
                new LambdaQueryWrapper<ShipmentEntity>()
                        .eq(ShipmentEntity::getShipTradeNo, shipTradeNo));
    }

    /**
     * 套用<b>已由 adapter 正規化過</b>的貨態事件。
     *
     * <p>known ⇒ 走既有的轉移路徑；unknown ⇒ {@link #persistUnknownProviderStatus}。
     * 呼叫端不得自己判斷 {@code normalizedStatus} 的內容——那是 adapter 的職責。
     */
    @Transactional
    public void applyInboundStatus(String shipTradeNo, InboundCapability.InboundStatusEvent event) {
        ShipmentEntity entity = findByShipTradeNo(shipTradeNo);
        if (entity == null) {
            log.warn("ShippingService.inbound.notFound",
                    "找不到物流單，shipTradeNo=" + shipTradeNo);
            return;
        }
        if (!event.isKnown()) {
            persistUnknownProviderStatus(entity, event.providerRawStatus(),
                    event.providerMessage(), "WEBHOOK");
            return;
        }
        ShippingStatus newStatus = ShippingStatus.fromValue(event.normalizedStatus().orElseThrow());
        ShippingStatus currentStatus = ShippingStatus.fromValue(entity.getStatus());
        if (currentStatus == newStatus) {
            return;
        }
        if (!statusMapper.isValidTransition(currentStatus, newStatus)) {
            log.warn("ShippingService.inbound.invalidTransition",
                    "非法狀態轉移，忽略（" + currentStatus.value() + " → " + newStatus.value() + "）",
                    "orderNo", entity.getOrderNo());
            return;
        }
        updateStatusWithParsedTime(entity, currentStatus, newStatus,
                event.providerRawStatus(), event.providerMessage(), event.providerStatusAt(), "WEBHOOK");
    }

    /**
     * 未知 provider 貨態的統一處置：<b>raw 值落庫、狀態機不動、留下可查詢的痕跡</b>。
     *
     * <h2>為什麼不是拋例外</h2>
     * 拋例外會讓整條 webhook 進 DLQ——provider 新增一個狀態碼不該讓鏈路停擺。
     * 但靜默當成「沒有變化」更糟：那是把 fail-closed 寫成 fail-open
     * （{@code InboundCapability} 的 javadoc 逐字要求呼叫端不得這樣做）。
     *
     * <h2>落在哪一欄</h2>
     * {@code shipments.provider_status}。V11 的註解逐字寫明「🔴 不新增 provider_raw_status：
     * 既有 provider_status 已經就是那一欄」⇒ 修法不是加欄位，是讓未知分支寫得進去。
     * {@code ShipmentMapper.updateProviderStatus} 是唯一不要求狀態轉移的寫入點，
     * 而它在此之前<b>零呼叫端</b>。
     *
     * <p>⚠️ 呼叫端仍會 ACK：訊息已經被正確接收與記錄，重投不會產生新資訊。
     * 「不 silent success」指的是<b>留下痕跡</b>，不是把 broker 的重投當成告警機制。
     */
    private void persistUnknownProviderStatus(ShipmentEntity entity, String rawStatus,
                                              String providerMessage, String source) {
        if (rawStatus == null || rawStatus.isBlank() || PAYUNI_DASH_PLACEHOLDER.equals(rawStatus.trim())) {
            log.warn("ShippingService.providerStatus.notAStatusEvent",
                    "回調不帶貨態欄位，非貨態事件，不寫入也不推進狀態機"
                            + "（source=" + source + ", shipmentNo=" + entity.getShipmentNo() + "）",
                    "orderNo", entity.getOrderNo());
            countNotAStatusEvent(entity, source);
            return;
        }
        int rows = shipmentMapper.updateProviderStatus(entity.getId(), rawStatus, providerMessage);
        log.warn("ShippingService.providerStatus.unknown",
                "物流商回報無法辨識的狀態，已保存原始值但不推進狀態機"
                        + "（source=" + source + ", raw=" + rawStatus
                        + ", shipmentNo=" + entity.getShipmentNo()
                        + ", persisted=" + rows + "）",
                "orderNo", entity.getOrderNo());
        countProviderStatusUnknown(entity, rows > 0 ? OUTCOME_PERSISTED : OUTCOME_PERSIST_MISSED);
    }

    /**
     * 「物流商回了我們看不懂的東西」的可觀測性。
     *
     * <p>🔴 為什麼光有 WARN log 不夠：Independent QA 的原話是「unknown status 被保存只是
     * 第一步；如果沒有任何監控，實務上仍是 silent failure」。log 沒有人訂閱，
     * 而這件事的意義正是「provider 可能改了碼表 / adapter 漏 map」——它需要的是趨勢，不是單筆。
     *
     * <p><b>label 的基數紀律</b>：只放有界維度。
     * <ul>
     *   <li>{@code source}：3 值（webhook／無 adapter 的 webhook／回查），呼叫端硬編字面值</li>
     *   <li>{@code shipping_method}：5 值（3001–3005，CodeMeta 值域）</li>
     *   <li>{@code outcome}：3 值（見下方常數）</li>
     * </ul>
     * 🔴 <b>raw 狀態值不得當 label</b>——它的定義就是「我方不認得的值」，母體由第三方決定，
     * 基數無上界。raw 值已經落在 {@code shipments.provider_status} 欄與 WARN log 裡，
     * 告警需要的只是「有沒有在發生、發生在哪個 provider」。
     * （同形先例：invoice-service 的 {@code invoice_amego_response_code_total} 把未知碼收斂成 "other"。）
     *
     * <p>⚠️ {@code notAStatusEvent} 這個出口也要計數：它是同一個函式的第二條未知路徑，
     * 漏掉就少一半觀測——而那正是 invoice M3/M4/M14 三條告警十個月從未觸發的成因形狀
     * （宣告與觀測點分屬不同 package）。
     */
    private void countProviderStatusUnknown(ShipmentEntity entity, String outcome) {
        io.micrometer.core.instrument.Counter.builder(METRIC_PROVIDER_STATUS_UNKNOWN)
                .description("物流商回報無法辨識貨態的次數（R2；raw 值另落 provider_status，此處只計趨勢）")
                .tag("shipping_method", methodTag(entity))
                .register(meterRegistry)
                .increment();
        log.warn("ShippingService.providerStatus.unknown.outcome", "未知貨態處置結果=" + outcome,
                "orderNo", entity.getOrderNo());
    }

    /**
     * 「回調不帶貨態欄位」的計數 —— 獨立指標，<b>不進告警</b>。
     *
     * <p>它是正常流量（PayUni 建單結果回報只有 TradeStatus），保留計數是為了讓
     * 「這條路徑突然變多」看得見，但它本身不代表缺陷，因此不與真未知貨態共用 counter。
     */
    private void countNotAStatusEvent(ShipmentEntity entity, String source) {
        io.micrometer.core.instrument.Counter.builder(METRIC_NOT_A_STATUS_EVENT)
                .description("回調不帶貨態欄位的次數（正常流量，例：PayUni 建單結果回報只有 TradeStatus）")
                .tag("shipping_method", methodTag(entity))
                .tag("source", source == null || source.isBlank() ? "unknown" : source)
                .register(meterRegistry)
                .increment();
    }

    private static String methodTag(ShipmentEntity entity) {
        return entity.getShippingMethod() == null ? "unknown" : entity.getShippingMethod();
    }


    /**
     * <b>legacy 相容入口</b>：以 PayUni 欄位為參數的貨態處理。
     *
     * <h2>🔴 它現在只做一件事：把參數還原成 raw payload，然後匯流到 canonical ingress</h2>
     * BLOCKER 4 的驗收條件之一是「不得存在兩份 status processing SSOT」。
     * 這支方法原本自己做**整套**：{@code mapPayUniStatus} 正規化、未知值處置、
     * 同狀態忽略、轉移合法性、更新——與 {@code InboundCapability} ＋ {@link #applyInboundStatus}
     * 完全平行的第二份。保留簽章（既有測試與任何未來的相容呼叫端用得到），
     * 但**行為由 canonical path 提供**。
     *
     * <p>⚠️ 欄名用的是 PayUni 官方文件的貨態欄位（{@code ShipStatus}／{@code ShipStatusDesc}／
     * {@code ShipStatusTime}），由 {@code PayUniInboundWireContractTest} 對著文件盯住。
     *
     * <p>production 的 webhook 已直接走 {@code ShippingWebhookConsumer} → {@link #inboundFor}
     * → {@link #applyInboundStatus}，不經過這裡。
     */
    @Transactional
    public void processWebhookStatusUpdate(String shipTradeNo, String payUniStatus,
                                            String statusDesc, String statusTime) {
        Map<String, String> rawPayload = new java.util.LinkedHashMap<>();
        rawPayload.put("ShipTradeNo", shipTradeNo);
        rawPayload.put("ShipStatus", payUniStatus);
        rawPayload.put("ShipStatusDesc", statusDesc);
        rawPayload.put("ShipStatusTime", statusTime);

        Optional<InboundCapability> inbound = inboundFor(shipTradeNo);
        if (inbound.isEmpty()) {
            ShipmentEntity entity = findByShipTradeNo(shipTradeNo);
            if (entity == null) {
                log.warn("ShippingService.webhook.notFound",
                        "找不到物流單，shipTradeNo=" + shipTradeNo);
                return;
            }
            persistUnknownProviderStatus(entity, payUniStatus, statusDesc, "WEBHOOK_NO_ADAPTER");
            return;
        }
        applyInboundStatus(shipTradeNo,
                inbound.get().parseStatusEvent(InboundPayload.ofFormParams(rawPayload)));
    }

    /**
     * 標籤補收路徑的入口：只收<b>中性的 key→value</b>。
     *
     * <p>🔴 存在的理由是不讓呼叫端碰 provider 欄名。補收排程從
     * {@link LabelCapability#fetchLabelIdentifiers} 拿到 map，直接原樣送進來；
     * 鍵名由 {@link LabelCapability} 的常數界定，<b>產生者與消費者引用同一份</b>
     * ——兩邊各自打字串就是兩份真實來源，而漏改一邊的症狀是三段碼靜默全空。
     *
     * <p>🔴 <b>{@code @Transactional} 必須掛在這一支</b>（2026-09-10 修）：
     * consumer 改走 canonical ingress 之後呼叫的是<b>本多載</b>，而它內部呼叫
     * 四參數多載屬 <b>self-invocation</b> ⇒ Spring proxy 不攔截 ⇒ 那一支上的
     * {@code @Transactional} 靜默失效（CLAUDE.md §11.1 明列的陷阱）。
     * 症狀不是報錯，是「三段碼與狀態的寫入不再原子」——各語句 auto-commit。
     */
    @Transactional
    public void processWebhookPrint(String shipTradeNo, Map<String, String> segmentCodes) {
        processWebhookPrint(shipTradeNo,
                segmentCodes.get(LabelCapability.SEGMENT_PARTNER_ID),
                segmentCodes.get(LabelCapability.SEGMENT_ODNO),
                segmentCodes.get(LabelCapability.SEGMENT_VALIDATION_NO));
    }

    /**
     * 處理 ApiType=Print Webhook（更新超商出貨單列印結果）
     */
    @Transactional
    public void processWebhookPrint(String shipTradeNo, String partnerId, String odno, String validationNo) {
        ShipmentEntity entity = shipmentMapper.selectOne(
                new LambdaQueryWrapper<ShipmentEntity>()
                        .eq(ShipmentEntity::getShipTradeNo, shipTradeNo));

        if (entity == null) {
            log.warn("ShippingService.webhookPrint.notFound",
                    "找不到物流單，shipTradeNo=" + shipTradeNo);
            return;
        }

        if (usesThreeSegmentCode(entity)
                && (partnerId == null || partnerId.isBlank()
                || odno == null || odno.isBlank()
                || validationNo == null || validationNo.isBlank())) {
            log.warn("ShippingService.webhookPrint.b143",
                    "B-143 三段碼缺失 shipTradeNo=" + shipTradeNo
                            + " partnerId=" + partnerId + " odno=" + odno + " validationNo=" + validationNo);
            anomalyEventPublisher.publishAnomalyDetected(
                    ShipmentAnomalyType.PAYUNI_LGS_NO_CODE,
                    entity.getOrderNo(),
                    entity.getId() == null ? "" : String.valueOf(entity.getId()),
                    entity.getStoreId(),
                    entity.getBuyerId()
            );
        }

        String cvsStatus = ShippingStatus.CVS_CODE_RECEIVED.value();
        int rows = shipmentMapper.markLabelPrintSucceededByWebhook(entity.getId(), partnerId, odno, validationNo, cvsStatus);

        if (rows == 0) {
            log.info("ShippingService.webhookPrint.skipped",
                    "列印結果已回填（冪等跳過）", "shipTradeNo", shipTradeNo);
            return;
        }

        ShippingStatus fromStatus = ShippingStatus.fromValue(entity.getStatus());
        insertStatusLog(entity.getId(), fromStatus, ShippingStatus.CVS_CODE_RECEIVED, "WEBHOOK_PRINT");

        log.info("ShippingService.webhookPrint.success",
                "超商列印結果已更新，status=920(CVS_CODE_RECEIVED)", "shipTradeNo", shipTradeNo);

        if (entity.getOrderNo() != null && !entity.getOrderNo().isBlank()) {
            boolean isReturn = Boolean.TRUE.equals(entity.getIsReturn());
            eventPublisher.publishLabelUpdated(
                    new io.github.will413028.hawkr.shipping.service.dto.LabelUpdatedPayload(
                            entity.getOrderNo(),
                            entity.getShipmentNo(),
                            partnerId, odno, validationNo,
                            isReturn));

            eventPublisher.publishShipmentStatusChanged(new ShippingEventPayload(
                    entity.getOrderNo(), entity.getShipmentNo(), entity.getShippingMethod(),
                    ShippingStatus.CVS_CODE_RECEIVED.value(), ShippingStatus.CVS_CODE_RECEIVED.label(),
                    ShippingStatus.CVS_CODE_RECEIVED.label(),
                    "", entity.getShipTradeNo(), "",
                    false, null, null, entity.getBuyerId(), 0L
            ));

            if (!isReturn) {
                eventPublisher.publishProcessingCleared(entity.getOrderNo());
            }
        }
    }


    /**
     * 處理出貨照片：FinalizeImages → **LinkImages** → 才寫入 {@code entity.shipmentPhotos}（#2222）。
     * <p>
     * 出貨照片是選填功能，整段對主流程容錯：任一步失敗就當作「這次沒有照片」，
     * entity.shipmentPhotos 保持空，因此**不會**有任何業務列指向剛轉正的資產，
     * 那些資產無人引用、由 media GC 依孤兒規則回收。
     * <p>
     * 順序不可調換：舊版是 finalize → 寫 shipment_photos → 事後 best-effort link，
     * link 一失敗就留下「shipments.shipment_photos 指著、卻沒有 asset_reference」的資產，
     * 7 天後被 GC 連同 R2 物件實體刪除。
     */
    private void processShipmentPhotos(ShipmentEntity entity, List<String> stagedTokens) {
        if (stagedTokens == null || stagedTokens.isEmpty()) {
            return;
        }
        try {
            var finalized = mediaGrpcClient.finalizeImages(stagedTokens, REF_TYPE_SHIPPING);
            List<ShipmentEntity.ShipmentPhoto> photos = finalized.stream()
                    .map(asset -> {
                        ShipmentEntity.ShipmentPhoto photo = new ShipmentEntity.ShipmentPhoto();
                        photo.setAssetId(asset.getAssetId());
                        photo.setObjectKey(asset.getObjectKey());
                        return photo;
                    })
                    .collect(Collectors.toList());
            if (photos.isEmpty()) {
                return;
            }
            List<String> assetIds = photos.stream()
                    .map(ShipmentEntity.ShipmentPhoto::getAssetId)
                    .collect(Collectors.toList());
            mediaGrpcClient.linkImages(REF_TYPE_SHIPPING, String.valueOf(entity.getId()), assetIds);
            entity.setShipmentPhotos(photos);
        } catch (Exception e) {
            log.error("ShippingService.processShipmentPhotos.error",
                    "出貨照片處理失敗（不影響主流程，本次視為未附照片）: " + e.getMessage(), e);
        }
    }

    /**
     * 發布出貨照片更新事件至 Order Service（best-effort，失敗不影響主流程）
     */
    private void publishShipmentPhotoEvent(ShipmentEntity entity) {
        if (entity.getShipmentPhotos() == null || entity.getShipmentPhotos().isEmpty()) {
            return;
        }
        try {
            List<String> photoKeys = entity.getShipmentPhotos().stream()
                    .map(ShipmentEntity.ShipmentPhoto::getObjectKey)
                    .filter(java.util.Objects::nonNull)
                    .toList();
            eventPublisher.publishShipmentPhotoUpdated(
                    new ShipmentPhotoPayload(entity.getOrderNo(), photoKeys));
        } catch (Exception e) {
            log.error("ShippingService.publishShipmentPhotoEvent.error",
                    "出貨照片事件發布失敗（不影響主流程）: " + e.getMessage(), e);
        }
    }

    /**
     * 批次查詢：依 order_no + is_return 取每個 order 的 provider_status / provider_message。
     * 收斂規範 §6.3 批次聚合用（Order / AfterSales 聚合 RPC 呼叫）。
     */
    public List<io.github.will413028.hawkr.shipping.dto.ProviderStatusRow> findProviderStatusByOrderNos(
            List<String> orderNos, boolean isReturn) {
        if (orderNos == null || orderNos.isEmpty()) {
            return List.of();
        }
        return shipmentMapper.findProviderStatusByOrderNos(orderNos, isReturn);
    }


    /**
     * PayUni 建單失敗時落地 shipments 表 shell row（status=CREATE_FAILED）。
     * 寫入 provider_status / provider_message，供 Admin 異常清單 + 後續閉環使用。
     * 不發 shipment.created 事件（建單失敗，未進 UNPROCESSED 正常流程）。
     */
    private void persistFailedShipmentShell(String orderNo, OrderShippingInfo orderInfo,
                                             String shippingMethod,
                                             SellerShipmentSettingEntity onboardingSetting,
                                             ProviderCreateFailedException failure) {
        try {
            requiresNewTx.executeWithoutResult(status -> {
                ShipmentEntity shell = new ShipmentEntity();
                shell.setId(io.github.will413028.hawkr.common.utils.TsidUtil.generate());
                shell.setShipmentNo(generateShipmentNo());
                shell.setOrderNo(orderNo);
                String storeIdStr = orderInfo.getStoreId();
                String buyerIdStr = orderInfo.getBuyerId();
                if (storeIdStr != null && !storeIdStr.isEmpty()) {
                    shell.setStoreId(Long.parseLong(storeIdStr));
                }
                if (buyerIdStr != null && !buyerIdStr.isEmpty()) {
                    shell.setBuyerId(Long.parseLong(buyerIdStr));
                }
                shell.setShippingMethod(shippingMethod);
                shell.setMerId(orderInfo.getMerId());
                shell.setStatus(ShippingStatus.CREATE_FAILED.value());
                shell.setProviderErrorCode(failure.providerStatus());
                shell.setProviderErrorMessage(failure.providerMessage());
                shell.setProviderErrorAt(DateTimeUtils.nowUtc());
                shell.setRecipientName(orderInfo.getRecipientName());
                shell.setRecipientPhone(orderInfo.getRecipientPhone());
                shell.setCvsStoreCode(orderInfo.getCvsStoreId());
                shell.setStoreName(orderInfo.getCvsStoreName());
                shell.setProdDesc(orderInfo.getProductName());
                shell.setServiceType(io.github.will413028.hawkr.payuni.shipping.ServiceType.NO_CASH_ON_DELIVERY.code());
                long tradeAmt = 0L;
                String tradeAmtStr = orderInfo.getTotalActualAmount();
                if (tradeAmtStr != null && !tradeAmtStr.isEmpty()) {
                    try { tradeAmt = Long.parseLong(tradeAmtStr); } catch (NumberFormatException ignore) {}
                }
                shell.setTradeAmt(tradeAmt);
                if (onboardingSetting != null) {
                    String snName = onboardingSetting.getSenderName();
                    String snPhone = onboardingSetting.getSenderPhone();
                    shell.setSenderName(snName != null ? snName : "");
                    shell.setSenderPhone(snPhone != null ? snPhone : "");
                } else {
                    shell.setSenderName("");
                    shell.setSenderPhone("");
                }
                shipmentMapper.insert(shell);
                insertStatusLog(shell.getId(), null, ShippingStatus.CREATE_FAILED, "PAYUNI_CREATE_FAILED");
            });
            log.warn("ShippingService.shipmentShell.persisted",
                    "建單失敗 shell 已落地 orderNo=" + orderNo
                            + " providerStatus=" + failure.providerStatus(),
                    "orderNo", orderNo);
        } catch (Exception persistEx) {
            log.error("ShippingService.shipmentShell.persistFailed",
                    "落地 failed shipment 失敗 orderNo=" + orderNo + " err=" + persistEx.getMessage(), persistEx);
        }
    }

    private static String generateShipmentNo() {
        long seq = io.github.will413028.hawkr.common.utils.TsidUtil.generate() % 1_000_000_000L;
        return "S" + String.format("%09d", seq);
    }

    /**
     * 退貨建單失敗殼的落地（收斂規範 §8.4）。
     *
     * <p>🔴 <b>必須脫離外層事務</b>：外層 {@code @Transactional} 會因為接下來拋出的
     * {@code RETURN_SHIPMENT_CREATE_FAILED} 而 rollback，殼被一起回滾的話，
     * Admin 的異常清單永遠查不到這筆——症狀是「退貨憑空消失」而不是「有一筆失敗的退貨」。
     *
     * <p>⚠️ 殼的<b>欄位組成由 adapter 決定</b>（與成功列同一份組裝）。遷移前這裡各有一份副本，
     * 而且已經漂移：宅配殼的 senderAddress 沒有走地址正規化，成功列有。
     */
    private void persistFailedReturnShell(ShipmentEntity shell, String logReason, String orderNo) {
        try {
            requiresNewTx.executeWithoutResult(status -> {
                shipmentMapper.insert(shell);
                insertStatusLog(shell.getId(), null, ShippingStatus.CREATE_FAILED, logReason);
            });
        } catch (Exception persistEx) {
            log.error("ShippingService.returnShipmentShell.persistFailed",
                    "落地 failed return shipment 失敗 orderNo=" + orderNo
                            + " err=" + persistEx.getMessage(), persistEx);
        }
    }


    /**
     * 查詢物流單並驗證賣家一致
     */
    private ShipmentEntity getAndVerifyShipment(String orderNo, long storeId) {
        ShipmentEntity entity = getShipmentByOrderNo(orderNo);
        if (entity == null) {
            throw new ShippingBusinessException(GrpcResponseCode.SHIPMENT_NOT_FOUND);
        }
        if (!entity.getStoreId().equals(storeId)) {
            throw new ShippingBusinessException(GrpcResponseCode.SHIPMENT_PERMISSION_DENIED);
        }
        return entity;
    }

    private void publishShipmentCreatedEvent(ShipmentEntity entity, OrderShippingInfo orderInfo) {
        String labelKey = labelKeyOf(entity);
        String trackingNo = (entity.getShipTradeNo() != null && !entity.getShipTradeNo().isBlank())
                ? entity.getShipTradeNo()
                : "";
        String processingDeadline = entity.getProcessingDeadline() != null
                ? DateTimeUtils.formatIso8601(entity.getProcessingDeadline())
                : "";

        String buyerNickname = null;
        String buyerAvatarKey = null;
        try {
            long buyerId = Long.parseLong(orderInfo.getBuyerId());
            Map<Long, io.github.will413028.hawkr.buyer.grpc.BuyerDisplay> metas =
                    buyerGrpcClient.batchGetBuyerDisplayByBuyerIds(List.of(buyerId));
            io.github.will413028.hawkr.buyer.grpc.BuyerDisplay bm = metas.get(buyerId);
            if (bm != null) {
                buyerNickname = bm.getDisplayName();
                buyerAvatarKey = bm.getAvatarKey();
            }
        } catch (Exception e) {
            log.error("ShippingService.publishShipmentCreatedEvent.enrichBuyerFailed",
                    "取買家資訊失敗，buyerId=" + orderInfo.getBuyerId()
                            + "，賣家推播將 fallback 至通用「買家」字樣", e);
        }

        Long resolvedSellerAccountId = resolveSellerAccountIdConservative(entity.getStoreId());
        io.github.will413028.hawkr.shipping.service.dto.ShipmentCreatedPayload eventPayload =
                new io.github.will413028.hawkr.shipping.service.dto.ShipmentCreatedPayload(
                        entity.getOrderNo(),
                        entity.getShipmentNo(),
                        trackingNo,
                        entity.getShippingMethod(),
                        ShippingStatus.UNPROCESSED.value(),
                        labelKey,
                        processingDeadline,
                        String.valueOf(resolvedSellerAccountId),
                        orderInfo.getBuyerId(),
                        orderInfo.getTotalActualAmount(),
                        buyerNickname,
                        buyerAvatarKey,
                        String.valueOf(entity.getStoreId())
                );
        eventPublisher.publishShipmentCreated(eventPayload);
    }

    private void updateStatus(ShipmentEntity entity, ShippingStatus fromStatus,
                               ShippingStatus toStatus, String providerStatus,
                               String providerMessage, String statusTime,
                               String triggerSource) {
        LocalDateTime providerStatusTime = DateTimeUtils.parsePayUniDateTimeToUtc(statusTime);
        if (providerStatusTime == null && statusTime != null && !statusTime.isBlank()) {
            log.warn("ShippingService.parseStatusTime.error",
                    "狀態時間解析失敗: " + statusTime);
        }
        updateStatusWithParsedTime(entity, fromStatus, toStatus, providerStatus,
                providerMessage, providerStatusTime, triggerSource);
    }

    /**
     * 與 {@link #updateStatus} 同一條路徑，差別只在 provider 時間<b>已經解析過</b>。
     *
     * <p>存在的理由：走 adapter 的回查路徑拿到的是 {@code LocalDateTime}
     * （時區換算已由 {@code AbstractPayUniAdapter.parseProviderTime} 做掉），
     * 讓它再格式化回字串只是為了重新 parse 一次，中間任何格式差異都會變成靜默的時間錯誤。
     */
    private void updateStatusWithParsedTime(ShipmentEntity entity, ShippingStatus fromStatus,
                                             ShippingStatus toStatus, String providerStatus,
                                             String providerMessage, LocalDateTime providerStatusTime,
                                             String triggerSource) {
        int rows = shipmentMapper.updateShipmentStatus(
                entity.getId(),
                fromStatus.value(),
                toStatus.value(),
                providerStatus,
                providerMessage,
                providerStatusTime
        );

        if (rows == 0) {
            log.info("ShippingService.statusUpdate.skipped",
                    "狀態未更新（已被其他執行緒搶先或已是更新的狀態）: shipmentNo=" + entity.getShipmentNo() + ", targetStatus=" + toStatus.label());
            return;
        }

        insertStatusLog(entity.getId(), fromStatus, toStatus, triggerSource);

        String labelKey = labelKeyOf(entity);
        String expireAt = entity.getLabelExpireAt() != null ? entity.getLabelExpireAt().toString() : "";
        String desc = (providerMessage != null && !providerMessage.isBlank())
                ? providerMessage : toStatus.label();
        boolean isReturnShipment = Boolean.TRUE.equals(entity.getIsReturn());
        boolean sellerTemplateReadsThisField = !isReturnShipment && toStatus == ShippingStatus.PICKED_UP;
        Long resolvedSellerAccountId = sellerTemplateReadsThisField
                ? resolveSellerAccountIdConservative(entity.getStoreId())
                : resolveSellerAccountIdOrZero(entity.getStoreId());
        Long notifyBuyerAccountId = (toStatus == ShippingStatus.IN_TRANSIT || toStatus == ShippingStatus.PICKED_UP)
                ? resolveNotifyBuyerAccountId(entity.getBuyerId())
                : 0L;
        ShippingEventPayload payload = new ShippingEventPayload(
                entity.getOrderNo(),
                entity.getShipmentNo(),
                entity.getShippingMethod(),
                toStatus.value(),
                toStatus.label(),
                desc,
                labelKey,
                entity.getShipTradeNo(),
                expireAt,
                isReturnShipment,
                null,
                null,
                entity.getBuyerId(),
                notifyBuyerAccountId,
                resolvedSellerAccountId,
                providerStatus,
                providerMessage,
                entity.getStoreId()
        );

        if (Boolean.TRUE.equals(entity.getIsReturn())) {
            eventPublisher.publishReturnShipmentStatusChanged(payload);
        } else if (toStatus == ShippingStatus.PICKED_UP) {
            eventPublisher.publishShipmentDelivered(payload);
            if (labelService.unlinkLabel(entity.getId(), entity.getLabelAssetId())) {
                shipmentMapper.clearLabelAfterDelivery(entity.getId());
                entity.setLabelObjectKey(null);
                entity.setLabelAssetId(null);
                entity.setLabelExpireAt(null);
            }
        } else {
            eventPublisher.publishShipmentStatusChanged(payload);
        }

        log.info("ShippingService.statusUpdate.success",
                "狀態更新: " + fromStatus.label() + " → " + toStatus.label(),
                "shipmentNo", entity.getShipmentNo());
    }


    /**
     * 真正的查詢：解析成功回 account_id，<b>解析不到或 gRPC 失敗一律回 {@code null}</b>。
     *
     * <p>兩個公開入口（{@link #resolveSellerAccountIdConservative} /
     * {@link #resolveSellerAccountIdOrZero}）共用它，差別只在「拿到 null 之後怎麼降級」。
     * 把降級決策留在入口，是因為那個決策<b>逐事件不同</b>，不是這支查詢的性質。
     *
     * <p>⚠️ {@code batchGetSellerMeta} 走 {@code GetSellerMetaByStoreIDs}（PK lookup，
     * 刻意不濾 {@code deleted_at}／{@code is_current}，且<b>不做 COALESCE echo-back</b>），
     * 所以 map-miss 的意義明確就是「查無此 store_id」。
     */
    private Long lookupSellerAccountId(Long storeId) {
        try {
            BatchGetSellerMetaResponse resp = sellerGrpcClient.batchGetSellerMeta(List.of(storeId));
            SellerMeta seller = resp.getSellersOrDefault(storeId, null);
            if (seller != null) {
                return seller.getAccountId();
            }
            log.warn("ShippingService.resolveSellerAccountId.notFound",
                    "seller-service 沒有回傳這個 store_id 的 meta（可能是不存在的 store_id）",
                    "storeId", String.valueOf(storeId));
        } catch (Exception e) {
            log.warn("resolveSellerAccountId.error",
                    "解析賣家 accountId 失敗，storeId=" + storeId + ": " + e.getMessage());
        }
        return null;
    }

    /**
     * <b>保守版</b>：解析不到時仍退回原始 store_id（fail-open），但一定留一行 WARN。
     *
     * <p>只給「這個事件有賣家向 notification template」的路徑用：
     * <ul>
     *   <li>{@code shipment.created}（template id=2，{@code push_mode=visible}「買家已下單」）——
     *       賣家<b>唯一</b>的新訂單推播；掉了會連鎖到出貨逾時、系統取消、退款</li>
     *   <li>{@code shipment.delivered}（賣家向 template {@code shipment.delivered.seller}）——
     *       賣家的「訂單已送達」收件匣通知</li>
     * </ul>
     *
     * <p>⚠️ 這條路徑帶出去的值<b>可能不是解析結果</b>：grandfather 期間 store_id == account_id
     * 讓它多半仍送對人，re-key 之後就會<b>送給錯的收件人或送不到</b>。
     * 之所以還留著它，是因為「有機會送到」大於「保證跳過」——這是對這兩個事件的取捨，
     * 不是對回吐值的信任。要拿掉這個取捨，正解是讓 seller 軸的
     * {@code batchGetSellerMeta} 也回報 unresolved（現行 {@code ResolvePrincipals} 已有
     * {@code unresolved_ids}），不是把這裡改成靜默 echo-back。
     */
    private Long resolveSellerAccountIdConservative(Long storeId) {
        if (storeId == null) {
            return null;
        }
        Long resolved = lookupSellerAccountId(storeId);
        if (resolved != null) {
            return resolved;
        }
        log.warn("ShippingService.resolveSellerAccountId.degradedEchoBack",
                "賣家身分未解析，本則事件降級帶原始 store_id ——「通知可能送錯對象或送不到」，"
                        + "此值不是解析結果，不得被下游當成 account_id 落庫",
                "storeId", String.valueOf(storeId));
        return storeId;
    }

    /**
     * <b>安全版</b>：解析不到就帶 {@code 0L}，不回吐 store_id。
     *
     * <p>給「沒有任何賣家向 template 會讀這個欄位」的路徑用：
     * {@code shipment.status.changed}（baseline 只有買家向 template id=3）與
     * {@code return.shipment.status.changed}（baseline <b>無任何</b> template）。
     * 這兩條回吐 store_id 的投遞收益是零，代價卻是讓一個別的身分空間的合法非零值
     * 繼續在事件裡流動；帶 0 之後 notification-service 的
     * {@code extractUserID}（{@code id <= 0} 回 error）會跳過並留 WARN。
     */
    private Long resolveSellerAccountIdOrZero(Long storeId) {
        if (storeId == null) {
            return 0L;
        }
        Long resolved = lookupSellerAccountId(storeId);
        if (resolved != null) {
            return resolved;
        }
        log.warn("ShippingService.resolveSellerAccountId.zeroed",
                "賣家身分未解析，本則事件的 sellerAccountId 帶 0（此事件無賣家向 template，"
                        + "不回吐 store_id 以免寫進錯的身分空間）",
                "storeId", String.valueOf(storeId));
        return 0L;
    }

    private Long resolveNotifyBuyerAccountId(Long buyerId) {
        if (buyerId == null) {
            return 0L;
        }
        try {
            io.github.will413028.hawkr.common.grpc.PrincipalResolution resolution =
                    buyerGrpcClient.resolvePrincipals(List.of(buyerId));
            if (!resolution.isUnresolved(buyerId)) {
                return resolution.principals().get(buyerId);
            }
            log.warn("ShippingService.resolveNotifyBuyerAccountId.unresolved",
                    "buyer-service 回報查無在架買家（unresolved_ids），買家通知帶 0，"
                            + "不回吐未驗證的 buyer_id",
                    "buyerId", String.valueOf(buyerId));
            return 0L;
        } catch (Exception e) {
            log.error("ShippingService.resolveNotifyBuyerAccountId.error",
                    "buyer_id -> account_id 解析失敗，買家通知將帶 0，buyerId=" + buyerId, e);
            return 0L;
        }
    }

    private void insertStatusLog(Long shipmentId, ShippingStatus fromStatus,
                                  ShippingStatus toStatus, String triggerSource) {
        ShipmentStatusLogEntity logEntity = new ShipmentStatusLogEntity();
        logEntity.setShipmentId(shipmentId);
        logEntity.setFromStatus(fromStatus != null ? fromStatus.value() : null);
        logEntity.setToStatus(toStatus.value());
        logEntity.setTriggerSource(triggerSource);
        statusLogMapper.insert(logEntity);
    }


    /**
     * 建單前是否必須先完成建店。
     *
     * <p>🔴 取代 {@code !UBER_INSTANT → 必須已建店} 的反向字面判斷：判斷依據從
     * 「配送方式是不是 Uber」變成「這個 provider 的建單需不需要建店」
     * （{@link CreateCapability#requiresOnboarding()}）。
     *
     * <p>⚠️ 查無 adapter 時回 {@code true}（保守：仍要求已建店），與改動前對未註冊配送方式的
     * 行為逐字相同——真正的「不支援此配送方式」由下游的 {@code CreateCapability} 查找回報，
     * 這裡不搶著換一個錯誤碼。
     * <p>⚠️ 本句原寫「由下游的 {@code strategyMap} 查找回報」，而 {@code strategyMap}
     * 已於 BLOCKER 5 移除——描述一個不存在的機制會讓下一個人往錯的地方找。
     */
    private boolean requiresOnboarding(long storeId, String shippingMethodValue) {
        FulfillmentProviderAdapter adapter = fulfillmentProviders.find(shippingMethodValue).orElse(null);
        if (adapter == null) {
            return true;
        }
        return adapter.create(FulfillmentContext.forStore(storeId, adapter.supportedMethod()))
                .map(CreateCapability::requiresOnboarding)
                .orElse(true);
    }

    /**
     * <b>provider-specific 端點的守衛</b>：這張配送單的配送方式必須就是本端點服務的那一種。
     *
     * <h2>🔴 BLOCKER 2：為什麼非有不可</h2>
     * {@code printCvsLabel} 與 {@code generateHomeLabelPdf} 是<b>兩支不同 provider 的端點</b>，
     * 而它們原本<b>一處都沒有讀 {@code entity.getShippingMethod()}</b>——
     * 唯一的守衛 {@code requireActiveSetting(storeId, ...)} 檢查的是「賣家有沒有開通這條線」，
     * 不是「這張訂單是不是這條線」。同時開通兩條線的賣家（dev 上就有）因此可以拿黑貓單
     * 打超商端點：QA 實測 {@code O290960523} 被推進 {@code label_print_status=4104 列印失敗}，
     * 而該狀態落在 {@code idx_shipments_label_reconcile} 述詞內 ⇒ 排程反覆重試、每次都失敗。
     *
     * <p>⚠️ <b>這是「前端隱藏按鈕不是安全邊界」的具體落實</b>：Gateway 這一波已改成
     * 按 (action, provider) 給 href，不會再產出跨 provider 的連結；但 write path 必須自己再驗一次，
     * 因為端點是公開的、而正確性不能建立在「沒有人會直接打它」上。
     *
     * <p>REMOVE-WHEN：65 的 provider-neutral 端點（S-10 {@code /dispatch}、
     * S-11 {@code /cancel}、S-12 {@code /pod}）落地並取代這些 provider-specific 端點之後，
     * 本守衛連同它守的端點一起移除。
     */
    private void requireEndpointProvider(ShipmentEntity entity, ShippingMethod endpointMethod) {
        if (!endpointMethod.value().equals(entity.getShippingMethod())) {
            throw new ShippingBusinessException(GrpcResponseCode.SHIPMENT_PROVIDER_MISMATCH);
        }
    }

    /**
     * 這個 provider 的履約流程裡有沒有「賣家確認出貨」這一步。
     *
     * <p>capability 為 empty ⇒ {@code false} ⇒ 照樣硬擋，與改動前逐字相同。
     */
    private boolean requiresSellerConfirmation(ShipmentEntity entity) {
        return fulfillmentProviders.find(entity.getShippingMethod())
                .map(FulfillmentProviderAdapter::requiresSellerConfirmation)
                .orElse(false);
    }

    /**
     * 取得這一筆的派車／叫件能力；provider 沒有這個動作時拋業務例外。
     *
     * <p>🔴 <b>fail-closed</b>：與 {@link #requireLabelCapability} 同一個理由——呼叫點是
     * 「賣家按了叫件」，靜默跳過會讓按鈕看起來成功而司機永遠不會來。
     *
     * <p>⚠️ 這裡回的是 capability 物件而不是布林：{@code requiresSellerConfirmation()}
     * 那條路徑問的是「有沒有這一步」（capability 為 empty ⇒ false），
     * 而這裡要真的把呼叫打出去 ⇒ 拿不到就必須擋。
     */
    private DispatchCapability requireDispatchCapability(ShipmentEntity entity) {
        return fulfillmentProviders.require(entity.getShippingMethod())
                .dispatch(FulfillmentContext.forShipment(entity))
                .orElseThrow(() -> new ShippingBusinessException(
                        GrpcResponseCode.FULFILLMENT_PROVIDER_NOT_ENABLED));
    }

    /**
     * 取得這一筆的標籤能力；provider 沒有標籤概念時拋業務例外。
     *
     * <p>🔴 <b>不做成「查不到就跳過」</b>：這個 helper 的呼叫點都是「賣家按了列印」，
     * 靜默跳過會讓按鈕看起來成功而什麼都沒發生，而那是最貴的一種失效。
     */
    private LabelCapability requireLabelCapability(ShipmentEntity entity) {
        return fulfillmentProviders.require(entity.getShippingMethod())
                .label(FulfillmentContext.forShipment(entity))
                .orElseThrow(() -> new ShippingBusinessException(
                        GrpcResponseCode.FULFILLMENT_PROVIDER_NOT_ENABLED));
    }

    /**
     * 取得逆物流建單能力；provider 不支援時拋業務例外。
     *
     * <p>⚠️ {@code FulfillmentContext.forStore}：逆向建單發生在<b>還沒有逆向配送單</b>的時候，
     * 硬湊一個假 shipment 只會讓「這個欄位是真的還是湊的」變成下一個人的問題。
     */
    private ReturnCapability requireReturnCapability(ShippingMethod method, long storeId) {
        return fulfillmentProviders.require(method.value())
                .returns(FulfillmentContext.forStore(storeId, method))
                .orElseThrow(() -> new ShippingBusinessException(
                        GrpcResponseCode.FULFILLMENT_PROVIDER_NOT_ENABLED));
    }

    /**
     * 這個 provider 的標籤是否以「三段碼」形式回傳（{@code PartnerId}／{@code Odno}／{@code ValidationNo}）。
     *
     * <p>capability 為 empty（例：Uber 沒有標籤概念）⇒ {@code false} ⇒ 不做 B-143 偵測。
     */
    private boolean usesThreeSegmentCode(ShipmentEntity entity) {
        return fulfillmentProviders.find(entity.getShippingMethod())
                .flatMap(adapter -> adapter.label(FulfillmentContext.forShipment(entity)))
                .map(LabelCapability::usesThreeSegmentCode)
                .orElse(false);
    }

    private String safeShippingMethodLabel(String shippingMethod) {
        try {
            return ShippingMethod.fromValue(shippingMethod).label();
        } catch (IllegalArgumentException e) {
            return "未知配送方式";
        }
    }

    /**
     * 組裝寄件人快照（JSONB）— 寄件人資訊來自 seller_shipment_settings
     */
    private String buildSenderSnapshot(SellerShipmentSettingEntity setting, String merId) {
        StringBuilder sb = new StringBuilder("{");
        sb.append("\"senderName\":\"").append(escape(setting != null ? setting.getSenderName() : "")).append("\"");
        sb.append(",\"senderPhone\":\"").append(escape(setting != null ? setting.getSenderPhone() : "")).append("\"");
        sb.append(",\"city\":\"").append(escape(setting != null ? setting.getSenderCity() : "")).append("\"");
        sb.append(",\"district\":\"").append(escape(setting != null ? setting.getSenderDistrict() : "")).append("\"");
        sb.append(",\"addressDetail\":\"").append(escape(setting != null ? setting.getSenderAddressDetail() : "")).append("\"");
        sb.append(",\"zipCode\":\"").append(escape(setting != null ? setting.getSenderPostalCode() : "")).append("\"");
        sb.append(",\"merId\":\"").append(escape(merId != null ? merId : "")).append("\"");
        if (setting != null) {
            sb.append(",\"onboardingId\":\"").append(setting.getId()).append("\"");
            if (setting.getReturnStoreCode() != null) {
                sb.append(",\"returnStoreCode\":\"").append(escape(setting.getReturnStoreCode())).append("\"");
            }
        }
        sb.append(",\"snapshotAt\":\"").append(DateTimeUtils.nowUtc().format(ISO_FMT)).append("\"");
        sb.append("}");
        return sb.toString();
    }

    /**
     * 組裝收件人快照（JSONB，寫入 {@code shipments.receiver_snapshot}）。
     *
     * <p><b>2026-08-05 key 正名</b>：{@code buyerAccountId} → {@code buyerId}
     * （裝的是 {@code orders.buyer_id} 裸 PK，從未解析成登入 account_id）；
     * {@code storeId} → {@code cvsStoreCode}（裝的是超商取貨門市代號，與賣家
     * store_id 無關——同款碰撞 V4 migration 已在 DB 欄位層解過一次，
     * 見 {@code V4__rename_shipping_seller_account_id_to_store_id.sql} 與
     * {@code ShipmentEntity.cvsStoreCode}）。本欄目前是 write-only 稽核快照
     * （全 repo 與 4 個前端 repo 皆無 reader），歷史 row 維持舊 key 不做 backfill；
     * 第一個要讀這欄的人必須同時處理新舊兩種形狀。
     */
    private String buildReceiverSnapshot(OrderShippingInfo orderInfo) {
        StringBuilder sb = new StringBuilder("{");
        sb.append("\"recipientName\":\"").append(escape(orderInfo.getRecipientName())).append("\"");
        sb.append(",\"recipientPhone\":\"").append(escape(orderInfo.getRecipientPhone())).append("\"");
        sb.append(",\"buyerId\":\"").append(orderInfo.getBuyerId()).append("\"");
        if (!orderInfo.getShippingAddress().isEmpty()) {
            sb.append(",\"shippingAddress\":\"").append(escape(orderInfo.getShippingAddress())).append("\"");
        }
        if (!orderInfo.getCvsStoreId().isEmpty()) {
            sb.append(",\"cvsStoreCode\":\"").append(escape(orderInfo.getCvsStoreId())).append("\"");
            sb.append(",\"storeName\":\"").append(escape(orderInfo.getCvsStoreName())).append("\"");
        }
        sb.append(",\"snapshotAt\":\"").append(DateTimeUtils.nowUtc().format(ISO_FMT)).append("\"");
        sb.append("}");
        return sb.toString();
    }

    private String escape(String value) {
        if (value == null) return "";
        return value.replace("\\", "\\\\").replace("\"", "\\\"");
    }

    private String labelKeyOf(ShipmentEntity entity) {
        if (entity.getLabelObjectKey() == null || entity.getLabelObjectKey().isBlank()) {
            return "";
        }
        return entity.getLabelObjectKey();
    }

}
