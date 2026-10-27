package io.github.will413028.hawkr.shipping.provider.payuni;

import io.github.will413028.hawkr.common.utils.code.ShippingMethod;
import io.github.will413028.hawkr.common.utils.code.ShippingStatus;
import io.github.will413028.hawkr.common.utils.code.ShippingType;
import io.github.will413028.hawkr.payuni.http.PayUniApiResponse;
import io.github.will413028.hawkr.payuni.shipping.ServiceType;
import io.github.will413028.hawkr.payuni.shipping.ShipType;
import io.github.will413028.hawkr.payuni.shipping.ShippingClient;
import io.github.will413028.hawkr.payuni.shipping.request.CreateCvsShipmentRequest;
import io.github.will413028.hawkr.payuni.shipping.response.CreateCvsShipmentResponse;
import io.github.will413028.hawkr.shipping.entity.SellerShipmentSettingEntity;
import io.github.will413028.hawkr.shipping.entity.ShipmentEntity;
import io.github.will413028.hawkr.shipping.provider.FulfillmentContext;
import io.github.will413028.hawkr.shipping.provider.capability.CancelCapability;
import io.github.will413028.hawkr.shipping.provider.capability.CreateCapability;
import io.github.will413028.hawkr.shipping.provider.capability.DispatchCapability;
import io.github.will413028.hawkr.shipping.provider.capability.LabelCapability;
import io.github.will413028.hawkr.shipping.provider.capability.OnboardCapability;
import io.github.will413028.hawkr.shipping.provider.capability.PodCapability;
import io.github.will413028.hawkr.shipping.provider.capability.ReturnCapability;
import io.github.will413028.hawkr.shipping.provider.capability.StoreLocatorCapability;
import io.github.will413028.hawkr.shipping.config.ShippingBatchProperties;
import io.github.will413028.hawkr.shipping.provider.capability.BatchConstraints;
import io.github.will413028.hawkr.shipping.service.OnboardingService;
import io.github.will413028.hawkr.shipping.service.PayUniParamMapper;
import io.github.will413028.hawkr.shipping.service.ShipmentLabelService;
import io.github.will413028.hawkr.shipping.service.ShipmentStatusMapper;
import io.github.will413028.hawkr.shipping.service.StoreMapService;
import io.github.will413028.hawkr.shipping.provider.create.CvsShipmentStrategy;
import io.github.will413028.hawkr.order.grpc.OrderShippingInfo;
import org.springframework.beans.factory.ObjectProvider;
import org.springframework.stereotype.Component;

import java.util.LinkedHashMap;
import java.util.List;
import java.util.Map;
import java.util.Optional;
import java.util.Set;

/**
 * 7-ELEVEN 交貨便 C2C（{@code 3002}）的 provider adapter。
 *
 * <table border="1">
 *   <caption>3002 的 capability 狀態</caption>
 *   <tr><th>capability</th><th>狀態</th><th>說明</th></tr>
 *   <tr><td>{@code create}</td><td>✅ 已實作</td><td>委派 {@code CvsShipmentStrategy}</td></tr>
 *   <tr><td>{@code track}</td><td>✅ 已實作</td><td>PayUni 查詢（{@code LgsType=C2C}）</td></tr>
 *   <tr><td>{@code inbound}</td><td>✅ 已實作</td><td>貨態 Notify ＋ 取號 Notify 的三段碼</td></tr>
 *   <tr><td>{@code onboard}</td><td>✅ 已實作</td>
 *       <td>委派 {@code OnboardingService.onboardCvsShipping}；{@code shippingTypeValue()} 取代
 *           {@code OnboardingService.mapShippingMethodToType} 的字面分支</td></tr>
 *   <tr><td>{@code dispatch}</td><td>⚠️ <b>present，但只有 {@code requiresSellerConfirmation()} 有實作</b></td>
 *       <td>🔴 這一格若寫成 {@code empty}，
 *           {@link DispatchCapability#requiresSellerConfirmation()} 就<b>永遠問不到</b>，
 *           而那個布林正是 {@code ShippingService.confirmCvsShipment} 用來取代
 *           「{@code !SEVEN_ELEVEN} → 硬擋」的唯一依據。{@code dispatch(...)} 這個<b>操作</b>
 *           對 7-11 C2C 永遠不存在，呼叫即拋 {@link UnsupportedOperationException}（不是靜默 no-op：
 *           靜默 no-op 會讓「以為叫了車」變成使用者的問題）</td></tr>
 *   <tr><td>{@code label}</td><td>✅ <b>已實作（本波搬入）</b></td>
 *       <td>{@code requestLabel}／{@code requestLabelBatch} 委派 {@code ShipmentLabelService}；
 *           {@code fetchLabelIdentifiers} 消滅了 {@code ShipmentLabelReconcileScheduler.deriveLgsType}
 *           這第二份 {@code LgsType} 推導</td></tr>
 *   <tr><td>{@code returns}</td><td>✅ <b>已實作（本波從零建）</b></td>
 *       <td>超商逆物流建單。它是 active production flow（aftersales 每筆退貨都會走），
 *           遷移前完全繞過任何抽象、就地組 PayUni request</td></tr>
 *   <tr><td>{@code storeLocator}</td><td>✅ <b>已實作（本波搬入）</b></td>
 *       <td>委派 {@code StoreMapService}；解析不出門市代碼即丟例外（fail-closed）</td></tr>
 *   <tr><td>{@code cancel}</td><td>❌ <b>provider 不支援</b></td><td>PayUni 無 cancel 端點</td></tr>
 *   <tr><td>{@code pod}</td><td>❌ <b>provider 不支援</b></td><td>超商取貨以「買家到店取貨」代替簽收</td></tr>
 * </table>
 */
@Component
public class PayUniCvsAdapter extends AbstractPayUniAdapter {

    private final CvsShipmentStrategy createStrategy;
    private final ShippingBatchProperties batchProperties;
    private final StoreMapService storeMapService;

    /**
     * 🔴 {@code ObjectProvider} 不是為了省事，是為了斷開一個真實的環：
     * {@code OnboardingService → FulfillmentProviderRegistry → PayUniCvsAdapter → OnboardingService}。
     * 環的來源是 {@code OnboardingService.mapShippingMethodToType} 改成問 adapter 之後才出現的。
     * 用 {@code @Lazy} 硬繞過只是把啟動期的問題推到執行期（第一次呼叫才炸），
     * {@code ObjectProvider} 則是明說「這個依賴在建構時不解析」。
     */
    private final ObjectProvider<OnboardingService> onboardingServiceProvider;

    public PayUniCvsAdapter(ShippingClient shippingClient,
                            ShipmentStatusMapper statusMapper,
                            ShipmentLabelService labelService,
                            CvsShipmentStrategy createStrategy,
                            ShippingBatchProperties batchProperties,
                            StoreMapService storeMapService,
                            ObjectProvider<OnboardingService> onboardingServiceProvider) {
        super(shippingClient, statusMapper, labelService);
        this.createStrategy = createStrategy;
        this.batchProperties = batchProperties;
        this.storeMapService = storeMapService;
        this.onboardingServiceProvider = onboardingServiceProvider;
    }

    @Override
    public ShippingMethod supportedMethod() {
        return ShippingMethod.SEVEN_ELEVEN_STORE_PICKUP;
    }

    /** 超商店到店在 PayUni 是 {@code C2C}——常數，不是從平台維度推導。 */
    @Override
    protected String lgsTypeCode() {
        return lgsC2c();
    }

    /**
     * 本波之後為<b>空集合</b>：{@code label}／{@code returns}／{@code storeLocator} 三項的實作都已搬進來。
     *
     * <p>⚠️ 空集合是這個方法的<b>目標終態</b>（見 {@code FulfillmentProviderAdapter#pendingMigration}），
     * 不是「忘了填」。要往回加東西之前先問：那個 capability 是「provider 不支援」（永久 empty，
     * 不進本集合）還是「我們還沒搬」（才進本集合）。
     */
    @Override
    public Set<String> pendingMigration() {
        return Set.of();
    }

    /**
     * 🔴 三段碼是超商軸專屬的 provider 概念。
     * 這一格取代 {@code ShippingService.processWebhookPrint} 內
     * 「只有超商才偵測三段碼」的字面分支——判斷依據從「配送方式是不是 3002」
     * 變成「這個 provider 的標籤回調帶不帶三段碼」。
     */
    @Override
    protected Map<String, String> labelSegmentsOf(Map<String, String> rawPayload) {
        Map<String, String> segments = new LinkedHashMap<>();
        putIfPresent(segments, rawPayload, LabelCapability.SEGMENT_PARTNER_ID);
        putIfPresent(segments, rawPayload, LabelCapability.SEGMENT_ODNO);
        putIfPresent(segments, rawPayload, LabelCapability.SEGMENT_VALIDATION_NO);
        return Map.copyOf(segments);
    }

    /**
     * 從 {@code /api/logistics/query} 的 raw response 取三段碼。
     *
     * <p>🔴 <b>「完整」的定義在這裡，不在呼叫端</b>：三段缺一即回空 map ⇒ 上層讀到
     * {@code identifiersReady=false}。遷移前這條規則寫在排程器裡（含 {@code "-"} 是 sandbox
     * 佔位符這件 provider 知識），而排程器同時還自己算了一份 {@code LgsType}。
     */
    @Override
    protected Map<String, String> labelIdentifiersOf(PayUniApiResponse api) {
        String partnerId = api.getData(LabelCapability.SEGMENT_PARTNER_ID);
        String odno = api.getData(LabelCapability.SEGMENT_ODNO);
        String validationNo = api.getData(LabelCapability.SEGMENT_VALIDATION_NO);
        if (blankOrPlaceholder(partnerId) || blankOrPlaceholder(odno) || blankOrPlaceholder(validationNo)) {
            return Map.of();
        }
        return Map.of(LabelCapability.SEGMENT_PARTNER_ID, partnerId,
                LabelCapability.SEGMENT_ODNO, odno,
                LabelCapability.SEGMENT_VALIDATION_NO, validationNo);
    }

    /** {@code "-"} 是 PayUni sandbox 的「尚未產生」佔位符，與空值同義。 */
    private static boolean blankOrPlaceholder(String s) {
        return s == null || s.isBlank() || "-".equals(s);
    }

    private static void putIfPresent(Map<String, String> target, Map<String, String> source, String key) {
        String v = source.get(key);
        if (v != null && !v.isBlank()) {
            target.put(key, v);
        }
    }

    @Override
    public Optional<CreateCapability> create(FulfillmentContext ctx) {
        return Optional.of(new CreateCapability() {
            @Override
            public ShipmentEntity createFromOrder(String orderNo, OrderShippingInfo orderInfo,
                                                  SellerShipmentSettingEntity onboardingSetting) {
                return createStrategy.createShipmentFromOrder(orderNo, orderInfo, onboardingSetting);
            }

            @Override
            public boolean requiresOnboarding() {
                return true;
            }
        });
    }

    @Override
    public Optional<LabelCapability> label(FulfillmentContext ctx) {
        return Optional.of(labelCapability);
    }

    private final LabelCapability labelCapability = new LabelCapability() {
        @Override
        public LabelRequestOutcome requestLabel(ShipmentEntity shipment, LabelRequest request) {
            labelService.triggerCvsPrint(shipment.getMerId(), shipment.getShipTradeNo(),
                    shipment.getGoodsType(), lgsTypeCode(), shipment.getShipType(),
                    requireShipDate(request));
            // 🔴 超商列印的成功訊號是 provider 的 Print Notify，不是這支 API 的回應。
            //    回 synchronous 會讓呼叫端誤以為可以立刻寫 SUCCEEDED。
            return LabelRequestOutcome.async();
        }

        @Override
        public LabelRequestOutcome requestLabelBatch(List<ShipmentEntity> shipments, LabelRequest request) {
            requireBatchWithinConstraints(shipments);
            String batchShipTradeNos = shipments.stream()
                    .map(ShipmentEntity::getShipTradeNo)
                    .collect(java.util.stream.Collectors.joining(","));
            ShipmentEntity first = shipments.get(0);
            labelService.triggerBatchCvsPrint(first.getMerId(), batchShipTradeNos,
                    first.getGoodsType(), lgsTypeCode(), first.getShipType(),
                    requireShipDate(request));
            return LabelRequestOutcome.async();
        }

        /**
         * PayUni 超商批次列印的硬約束：同一批必須同 {@code GoodsType}／{@code LgsType}／
         * {@code ShipType}／{@code ShipDate}（PayUni 把整批當成一次列印請求）。
         *
         * <p>單批上限取 {@code hawkr.shipping.batch.max-size}——它今天寫在
         * 平台的批次設定裡，而它<b>限制的是 provider</b>；放在這裡至少讓呼叫端只有一個地方讀得到。
         */
        @Override
        public BatchConstraints batchConstraints() {
            return new BatchConstraints(batchProperties.getMaxSize(),
                    Set.of("goodsType", "lgsType", "shipType", "shipDate"));
        }

        @Override
        public boolean usesThreeSegmentCode() {
            return true;
        }

        @Override
        public LabelIdentifierLookup fetchLabelIdentifiers(ShipmentEntity shipment) {
            return lookupLabelIdentifiers(shipment);
        }
    };

    /**
     * 🔴 {@code shipDate} 缺席一律拋，<b>不得代填今天</b>：
     * PayUni 用它決定出貨日，猜錯就是把錯的資料送進 provider，而回應是 200。
     */
    private static String requireShipDate(LabelCapability.LabelRequest request) {
        if (request == null || request.shipDate() == null || request.shipDate().isBlank()) {
            throw new IllegalArgumentException(
                    "超商列印必填 shipDate（PayUni 請求參數）——adapter 不代填，代填等於送錯資料給 provider");
        }
        return request.shipDate();
    }

    /**
     * 批次約束的<b>強制執行點</b>。約束物件光是存在不會擋住任何東西——
     * {@code batchConstraints()} 沒有人執行的話，它只是一份被閱讀的文件。
     */
    private void requireBatchWithinConstraints(List<ShipmentEntity> shipments) {
        if (shipments == null || shipments.isEmpty()) {
            throw new IllegalArgumentException("批次列印不得為空批");
        }
        BatchConstraints constraints = labelCapability.batchConstraints();
        if (shipments.size() > constraints.maxBatchSize()) {
            throw new IllegalArgumentException("超過 provider 單批上限 " + constraints.maxBatchSize()
                    + "，實得 " + shipments.size());
        }
        ShipmentEntity first = shipments.get(0);
        for (ShipmentEntity e : shipments) {
            if (!java.util.Objects.equals(first.getGoodsType(), e.getGoodsType())
                    || !java.util.Objects.equals(first.getShipType(), e.getShipType())
                    || !java.util.Objects.equals(first.getMerId(), e.getMerId())) {
                throw new IllegalArgumentException(
                        "同一批的 goodsType／shipType／merId 必須一致（PayUni 把整批當成一次列印請求）");
            }
        }
    }

    /**
     * 🔴 {@code empty}——7-ELEVEN 交貨便 C2C <b>沒有派車／叫件 API</b>，這是事實陳述。
     *
     * <p><b>2026-09-10 BLOCKER 2 修復前這裡回的是 {@code Optional.of(一個會丟例外的 stub)}</b>，
     * 理由是「{@code requiresSellerConfirmation()} 需要問得出布林」。
     * 那個理由讓 capability 說了謊：上游 {@code AllowedActionResolver.hasCapabilityFor}
     * 只看 {@code isPresent()} ⇒ 超商單的 {@code dispatch} 被算成 <b>enabled</b>，
     * 而 Gateway 給的 href 指向黑貓的 {@code /call_tcat}。
     * 那個布林已搬到 {@link FulfillmentProviderAdapter#requiresSellerConfirmation()}
     * ——它本來就不是 dispatch 能力的一部分。
     */
    @Override
    public Optional<DispatchCapability> dispatch(FulfillmentContext ctx) {
        return Optional.empty();
    }

    /** 7-11 C2C 的交運＝賣家在我方確認出貨後拿去門市交寄 ⇒ 有「賣家確認出貨」這一步。 */
    @Override
    public boolean requiresSellerConfirmation() {
        return true;
    }


    /** ❌ 永久 empty：PayUni 無 cancel 端點。取消走 992 CANCEL_PENDING。 */
    @Override
    public Optional<CancelCapability> cancel(FulfillmentContext ctx) {
        return Optional.empty();
    }

    @Override
    public Optional<ReturnCapability> returns(FulfillmentContext ctx) {
        return Optional.of(returnCapability);
    }

    private final ReturnCapability returnCapability = new ReturnCapability() {
        /**
         * 超商 C2C 逆物流：<b>角色對調</b>——買家是寄件人，賣家的退貨門市是取件門市。
         *
         * <p>⚠️ {@code forwardShipment} 對本條線沒有用途（超商退貨不沿用原單的任何 provider 參數），
         * 傳 null 是合法的。
         */
        @Override
        public ReturnOutcome createReturnShipment(ShipmentEntity forwardShipment,
                                                  SellerShipmentSettingEntity sellerSetting,
                                                  ReturnRequest request) {
            String goodsType = PayUniParamMapper.toGoodsType(request.temperature());

            CreateCvsShipmentRequest payUniReq = new CreateCvsShipmentRequest(
                    returnMerTradeNo(request.orderNo()),
                    ShipType.SEVEN.code(),
                    lgsTypeCode(),
                    goodsType,
                    ServiceType.NO_CASH_ON_DELIVERY.code(),
                    request.buyerName(),                   // 寄件人 = 買家
                    request.buyerPhone(),
                    sellerSetting.getSenderName(),         // 收件人 = 賣家退貨聯絡人
                    sellerSetting.getSenderPhone(),
                    sellerSetting.getReturnStoreCode(),    // 取貨門市 = 退貨門市
                    String.valueOf(request.tradeAmt()),    // 商品價值（遺失認賠金額）
                    "退貨商品",
                    null                                   // 這本身就是退貨單，不需再指定退貨門市
            );

            // 收斂規範 §8.1：同步 retry 最多一次
            CreateCvsShipmentResponse resp = shippingClient.createCvsShipment(request.merId(), payUniReq);
            if (!resp.isSuccess()) {
                resp = shippingClient.createCvsShipment(request.merId(), payUniReq);
            }

            ShipmentEntity entity = baseReturnEntity(request, goodsType, sellerSetting);
            if (!resp.isSuccess()) {
                String providerStatus = resp.apiResponse().status();
                String providerMessage = resp.apiResponse().message();
                entity.setStatus(ShippingStatus.CREATE_FAILED.value());
                // V12：建單失敗是 provider 層失敗，寫 provider_error_*。
                // provider_status 是「正常貨態證據」欄，把失敗代碼寫進去會讓 ADM 的
                // 異常判準只能寫成「有值即異常」，於是每張正常單都被撈進異常清單。
                entity.setProviderErrorCode(providerStatus);
                entity.setProviderErrorMessage(providerMessage);
                entity.setProviderErrorAt(io.github.will413028.hawkr.common.utils.DateTimeUtils.nowUtc());
                return ReturnOutcome.rejected(entity, providerStatus, providerMessage);
            }

            entity.setStatus(ShippingStatus.UNPROCESSED.value());
            entity.setShipTradeNo(resp.shipTradeNo());
            return ReturnOutcome.accepted(entity);
        }

        /**
         * 受理與拒絕共用的欄位組裝。
         *
         * <p>🔴 <b>共用不是為了省行數</b>：遷移前成功路徑與失敗殼各有一份約 15 個 setter 的副本，
         * 而它們必須永遠一致（Admin 的異常清單是靠殼上的欄位查的）。兩份副本沒有任何東西保證同步。
         */
        private ShipmentEntity baseReturnEntity(ReturnRequest request, String goodsType,
                                                SellerShipmentSettingEntity sellerSetting) {
            ShipmentEntity entity = new ShipmentEntity();
            entity.setShipmentNo(request.returnShipmentNo());
            entity.setOrderNo(request.orderNo());
            entity.setStoreId(request.storeId());
            entity.setShippingMethod(supportedMethod().value());
            entity.setMerId(request.merId());
            entity.setShipType(ShipType.SEVEN.code());
            entity.setLgsType(lgsTypeCode());
            entity.setGoodsType(goodsType);
            entity.setServiceType(ServiceType.NO_CASH_ON_DELIVERY.code());
            entity.setTradeAmt(request.tradeAmt());
            entity.setSenderName(request.buyerName());
            entity.setSenderPhone(request.buyerPhone());
            entity.setRecipientName(sellerSetting.getSenderName());
            entity.setRecipientPhone(sellerSetting.getSenderPhone());
            entity.setCvsStoreCode(sellerSetting.getReturnStoreCode());
            entity.setStoreName(sellerSetting.getReturnStoreName());
            entity.setIsReturn(true);
            return entity;
        }
    };

    /** ❌ 永久 empty：超商取貨以「買家到店取貨」代替簽收，無 POD 概念。 */
    @Override
    public Optional<PodCapability> pod(FulfillmentContext ctx) {
        return Optional.empty();
    }

    /**
     * ✅ 已搬移：送審委派 {@code OnboardingService.onboardCvsShipping}，
     * {@code shippingTypeValue()} 取代 {@code mapShippingMethodToType} 的字面分支。
     */
    @Override
    public Optional<OnboardCapability> onboard(FulfillmentContext ctx) {
        return Optional.of(onboardCapability);
    }

    private final OnboardCapability onboardCapability = new OnboardCapability() {
        /**
         * ⚠️ {@code submit} 與 {@code resubmit} 委派同一個入口<b>不是偷懶</b>：
         * {@code onboardCvsShipping} 內部先查既有設定，有就走 resubmit、沒有就走首次送審。
         * 讓 adapter 自己決定走哪一條，等於在 adapter 裡複製一份「什麼算既有設定」的業務判斷
         * （硬規則 1：adapter 只做翻譯）。
         */
        @Override
        public OnboardOutcome submit(SellerShipmentSettingEntity setting) {
            return toOutcome(onboardingServiceProvider.getObject().onboardCvsShipping(
                    setting.getStoreId(),
                    setting.getGoodsType(),
                    setting.getReturnStoreCode(),
                    setting.getReturnStoreName(),
                    setting.getReturnStoreAddress(),
                    setting.getSenderName(),
                    setting.getSenderPhone()));
        }

        @Override
        public OnboardOutcome resubmit(SellerShipmentSettingEntity setting) {
            return submit(setting);
        }

        @Override
        public String shippingTypeValue() {
            return ShippingType.CVS_C2C.value();
        }
    };

    /** provider 側識別：超商建店回的是 {@code PartnerId}（宅配是 {@code eshipId}）。 */
    private static OnboardCapability.OnboardOutcome toOutcome(SellerShipmentSettingEntity saved) {
        return new OnboardCapability.OnboardOutcome(
                saved.getPartnerId(), saved.getPayuniSubmitStatus(), saved.getProviderMessage());
    }

    @Override
    public Optional<StoreLocatorCapability> storeLocator(FulfillmentContext ctx) {
        return Optional.of(storeLocatorCapability);
    }

    private final StoreLocatorCapability storeLocatorCapability = new StoreLocatorCapability() {
        @Override
        public Map<String, String> buildRedirectParams(StoreMapRequest request) {
            StoreMapService.StoreMapFormResult form = storeMapService.buildStoreMapParams(
                    request.principalId(), request.clientKind(), request.mobile() ? "Y" : "N");
            Map<String, String> params = new LinkedHashMap<>();
            params.put(PARAM_FORM_ACTION, nullToEmpty(form.formAction()));
            params.put(PARAM_MERCHANT_ID, nullToEmpty(form.merId()));
            params.put(PARAM_VERSION, nullToEmpty(form.version()));
            params.put(PARAM_PAYLOAD, nullToEmpty(form.encryptInfo()));
            params.put(PARAM_SIGNATURE, nullToEmpty(form.hashInfo()));
            params.put(PARAM_REDIRECT_URL, nullToEmpty(form.redirectUrl()));
            return Map.copyOf(params);
        }

        /**
         * 🔴 <b>fail-closed</b>：解不出門市代碼就丟例外。
         *
         * <p>遷移前這一格是 fail-open——{@code MapJson} 解析失敗只記一行 error log，
         * 然後帶著<b>空的 storeCode</b> 組出導回網址。買家看到的是「選店完成」，
         * 而錯誤要到出貨當天才會現形。
         */
        @Override
        public StoreSelection parseResult(Map<String, String> rawResult) {
            StoreMapService.StoreMapDecryptResult decrypted = storeMapService.decryptStoreMapResult(
                    rawResult.get(RESULT_PAYLOAD),
                    rawResult.get(RESULT_SIGNATURE),
                    rawResult.get(RESULT_CLIENT_KIND));
            if (decrypted.storeId() == null || decrypted.storeId().isBlank()) {
                throw new IllegalArgumentException(
                        "門市地圖回傳中解不出門市代碼——不猜門市（帶空代碼往下走，錯誤會在出貨當天才現形）");
            }
            return new StoreSelection(decrypted.storeId(), decrypted.storeName(), decrypted.address(),
                    decrypted.insularArea(), decrypted.storeType(), decrypted.redirectUrl());
        }
    };

    private static String nullToEmpty(String s) {
        return s == null ? "" : s;
    }
}
