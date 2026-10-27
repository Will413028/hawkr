package io.github.will413028.hawkr.shipping.provider.payuni;

import io.github.will413028.hawkr.common.utils.DateTimeUtils;
import io.github.will413028.hawkr.common.utils.code.ShippingMethod;
import io.github.will413028.hawkr.common.utils.code.ShippingStatus;
import io.github.will413028.hawkr.common.utils.code.ShippingType;
import io.github.will413028.hawkr.common.obs.log.ObsLogger;
import io.github.will413028.hawkr.payuni.http.PayUniApiException;
import io.github.will413028.hawkr.payuni.shipping.DeliveryTimeTag;
import io.github.will413028.hawkr.payuni.shipping.GoodsType;
import io.github.will413028.hawkr.payuni.shipping.ServiceType;
import io.github.will413028.hawkr.payuni.shipping.ShipType;
import io.github.will413028.hawkr.payuni.shipping.ShippingClient;
import io.github.will413028.hawkr.payuni.shipping.request.CallTcatRequest;
import io.github.will413028.hawkr.payuni.shipping.request.CreateHomeRefundRequest;
import io.github.will413028.hawkr.payuni.shipping.response.CallTcatResponse;
import io.github.will413028.hawkr.payuni.shipping.response.CreateHomeRefundResponse;
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
import io.github.will413028.hawkr.shipping.provider.capability.BatchConstraints;
import io.github.will413028.hawkr.shipping.service.OnboardingService;
import io.github.will413028.hawkr.shipping.service.PayUniParamMapper;
import io.github.will413028.hawkr.shipping.service.ShipmentLabelService;
import io.github.will413028.hawkr.shipping.service.ShipmentStatusMapper;
import io.github.will413028.hawkr.shipping.provider.create.HomeShipmentStrategy;
import io.github.will413028.hawkr.shipping.util.AddressNormalizer;
import io.github.will413028.hawkr.order.grpc.OrderShippingInfo;
import org.springframework.beans.factory.ObjectProvider;
import org.springframework.stereotype.Component;

import java.time.DayOfWeek;
import java.time.LocalDate;
import java.time.format.DateTimeFormatter;
import java.util.List;
import java.util.Optional;
import java.util.Set;

/**
 * 黑貓宅急便（{@code 3001}）的 provider adapter。
 *
 * <h2>🔴 capability 集合：三欄，不是兩欄</h2>
 * 「回 {@code Optional.empty()}」有<b>兩種完全不同的原因</b>，混在一起會讓下一個人做出錯誤推論。
 * 因此本表把它們分開，且 {@link #pendingMigration()} 讓它可被機械檢查：
 *
 * <table border="1">
 *   <caption>3001 的 capability 狀態</caption>
 *   <tr><th>capability</th><th>狀態</th><th>說明</th></tr>
 *   <tr><td>{@code create}</td><td>✅ 已實作</td><td>委派 {@code HomeShipmentStrategy}（既有、已驗證的建單邏輯）</td></tr>
 *   <tr><td>{@code track}</td><td>✅ 已實作</td><td>PayUni 查詢已搬進 {@link AbstractPayUniAdapter}，含未知狀態的 raw 值回傳</td></tr>
 *   <tr><td>{@code inbound}</td><td>✅ 已實作</td><td>貨態 Notify 解析與去重鍵已搬進 base</td></tr>
 *   <tr><td>{@code label}</td><td>✅ <b>已實作（本波搬入）</b></td>
 *       <td>{@code requestLabel} 委派 {@code ShipmentLabelService.scheduleAndUploadLabel}（同步取得 PDF）；
 *           {@code usesThreeSegmentCode()} 回 <b>false</b>——宅配走同步 PDF 路徑，by-design 不寫三段碼</td></tr>
 *   <tr><td>{@code returns}</td><td>✅ <b>已實作（本波從零建）</b></td>
 *       <td>宅配逆物流建單（{@code /api/home_delivery/refund}）。它是 active production flow，
 *           遷移前完全繞過任何抽象</td></tr>
 *   <tr><td>{@code cancel}</td><td>❌ <b>provider 不支援</b></td>
 *       <td>PayUni 的 11 個端點常數實查無 cancel。這是<b>永久</b>的 empty</td></tr>
 *   <tr><td>{@code pod}</td><td>❌ <b>provider 不支援</b></td><td>黑貓無送達證明概念。永久 empty</td></tr>
 *   <tr><td>{@code storeLocator}</td><td>❌ <b>provider 不支援</b></td><td>宅配沒有門市。永久 empty</td></tr>
 *   <tr><td>{@code onboard}</td><td>✅ 已實作</td>
 *       <td>委派 {@code OnboardingService.onboardHomeDelivery}；{@code shippingTypeValue()} 取代
 *           {@code OnboardingService.mapShippingMethodToType} 的字面分支</td></tr>
 *   <tr><td>{@code dispatch}</td><td>✅ <b>已實作（本波搬入）</b></td>
 *       <td>黑貓叫件（{@code /api/home_delivery/call_cat}，見 {@code PayUniShippingEndpoint.PATH_CALL_TCAT}）。搬進來的是<b>參數翻譯＋呼叫</b>
 *           （溫層→件數、寄件人地址組裝、Y/N 翻譯、同步 retry）；搶佔、狀態轉移、事件發布與
 *           預計到貨日估算是平台流程，留在 {@code ShippingService.callTcat}</td></tr>
 * </table>
 *
 * <p>🔴 {@code cancel()} 回 empty <b>不代表黑貓單不能取消</b>：取消是
 * {@code requiresProviderCall = false} 的 domain action，寫 {@code 992 CANCEL_PENDING}
 * 或本地終態，provider 側由回調／人工補上。
 */
@Component
public class PayUniHomeAdapter extends AbstractPayUniAdapter {

    private static final ObsLogger log = ObsLogger.forClass(PayUniHomeAdapter.class);

    /** PayUni 的日期參數格式（{@code yyyyMMdd}）。 */
    private static final DateTimeFormatter PROVIDER_DATE = DateTimeFormatter.BASIC_ISO_DATE;

    /** 宅配逆向的預設規格（60cm）；正向單有存 spec 時以正向單為準。 */
    private static final String DEFAULT_SPEC = "1";

    private final HomeShipmentStrategy createStrategy;

    /**
     * 🔴 {@code ObjectProvider} 斷開 {@code OnboardingService → registry → adapter → OnboardingService}
     * 這個環（{@code mapShippingMethodToType} 改成問 adapter 之後才出現）。
     * 用 {@code @Lazy} 硬繞過只是把啟動期的問題推到執行期。
     */
    private final ObjectProvider<OnboardingService> onboardingServiceProvider;

    public PayUniHomeAdapter(ShippingClient shippingClient,
                             ShipmentStatusMapper statusMapper,
                             ShipmentLabelService labelService,
                             HomeShipmentStrategy createStrategy,
                             ObjectProvider<OnboardingService> onboardingServiceProvider) {
        super(shippingClient, statusMapper, labelService);
        this.createStrategy = createStrategy;
        this.onboardingServiceProvider = onboardingServiceProvider;
    }

    @Override
    public ShippingMethod supportedMethod() {
        return ShippingMethod.TAQBIN;
    }

    /**
     * 🔴 這裡是 {@code deriveLgsType} 的歸宿：宅配在 PayUni 就是 {@code HOME}，**常數不是推導**。
     * 舊實作把它寫成「{@code TAQBIN} 就回 HOME、否則回 C2C」的 if，而且有<b>兩份副本</b>
     * （service 一份、對帳排程器一份），沒有任何東西保證它們同步。
     */
    @Override
    protected String lgsTypeCode() {
        return lgsHome();
    }

    /**
     * ✅ <b>空集合</b>：黑貓（3001）在 production 運作的 capability 已全數搬入本 adapter。
     *
     * <p>⚠️ 空集合<b>不代表</b>本 adapter 每一格都 present——{@code cancel}／{@code pod}／
     * {@code storeLocator} 仍是 {@code Optional.empty()}，但那是「provider 真的做不到」，
     * 不是「我們還沒搬」。這個方法存在的理由正是把後者標出來：
     * {@code Optional.empty()} 對兩者在型別上一模一樣，分不出來的話，下一個人會把
     * 「我們還沒搬」讀成「黑貓做不到」，然後在別處重新實作一份。
     *
     * <p>🔴 這是本方法的<b>目標終態</b>。把任何一項加回去之前先問：那個 capability 今天
     * 在 production 有沒有在跑？有 ⇒ 它不該被標成 pending 而擱置，該搬。
     */
    @Override
    public Set<String> pendingMigration() {
        return Set.of();
    }

    @Override
    public Optional<CreateCapability> create(FulfillmentContext ctx) {
        return Optional.of(new CreateCapability() {
            @Override
            public ShipmentEntity createFromOrder(String orderNo, OrderShippingInfo orderInfo,
                                                  SellerShipmentSettingEntity onboardingSetting) {
                return createStrategy.createShipmentFromOrder(orderNo, orderInfo, onboardingSetting);
            }

            /** 黑貓必須先完成建店才建得出單（舊實作寫成「不是 Uber 就要建店」的反向判斷）。 */
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
        /**
         * 宅配取號：PayUni 回 HTML → 二次 POST 下載 PDF → 轉存我方儲存。
         *
         * <p>🔴 回 {@code synchronous}：與超商不同，宅配的標籤在這一支呼叫內就拿到了，
         * 沒有等待 provider 回調的階段。呼叫端據此決定要不要立刻寫 {@code SUCCEEDED}。
         */
        @Override
        public LabelRequestOutcome requestLabel(ShipmentEntity shipment, LabelRequest request) {
            requireDates(request);
            ShipmentLabelService.LabelUploadResult result = labelService.scheduleAndUploadLabel(
                    shipment.getMerId(),
                    shipment.getShipTradeNo(),
                    shipment.getShipmentNo(),
                    shipment.getGoodsType(),
                    lgsTypeCode(),
                    ShipType.TAQBIN.code(),
                    request.shipDate(),
                    request.deliveryDate(),
                    shipment.getSpec());
            return LabelRequestOutcome.synchronous(result.assetId(), result.objectKey());
        }

        /**
         * 🔴 宅配<b>沒有</b>批次取號端點：{@code BatchGenerateHomeLabelPdf} 是我方
         * {@code BatchHomeLabelService} 用 virtual thread 逐筆呼叫單筆路徑組出來的，
         * provider 端一次只收一張。⇒ 這裡不是「還沒搬」，是 provider 真的做不到。
         */
        @Override
        public LabelRequestOutcome requestLabelBatch(List<ShipmentEntity> shipments, LabelRequest request) {
            throw new IllegalArgumentException(
                    "黑貓宅配沒有批次取號端點（provider 一次只收一張）——"
                            + "批次由呼叫端逐筆走 requestLabel，見 batchConstraints()=SINGLE_ONLY");
        }

        @Override
        public BatchConstraints batchConstraints() {
            return BatchConstraints.SINGLE_ONLY;
        }

        /**
         * 🔴 回 {@code false} 是這一格的重點：舊實作用「{@code 是不是 3002}」來決定要不要偵測三段碼，
         * 而那句話的<b>意思</b>一直都是「這個 provider 的標籤帶不帶三段碼」。
         * 宅配走同步 PDF，PayUni 對 {@code LgsType=HOME} 的回應 by-design 不帶三段碼。
         */
        @Override
        public boolean usesThreeSegmentCode() {
            return false;
        }

        /**
         * 宅配的查詢回應 by-design 不帶標籤識別 ⇒ 恆為 {@code identifiersReady=false}
         * （{@code labelIdentifiersOf} 未覆寫，用 base 的空 map）。
         *
         * <p>⚠️ 仍然要<b>真的打一次查詢</b>而不是直接回 notReady：補收排程靠
         * {@code querySucceeded} 分辨「provider 故障」與「還沒好」，直接回 notReady
         * 會讓宅配單的 provider 故障永遠看不見。
         */
        @Override
        public LabelIdentifierLookup fetchLabelIdentifiers(ShipmentEntity shipment) {
            return lookupLabelIdentifiers(shipment);
        }
    };

    /**
     * 🔴 兩個日期缺一律拋，<b>不得代填</b>：PayUni 用它們決定出貨日與配達日，
     * 猜錯就是把錯的資料送進 provider，而回應是 200。
     */
    private static void requireDates(LabelCapability.LabelRequest request) {
        if (request == null
                || request.shipDate() == null || request.shipDate().isBlank()
                || request.deliveryDate() == null || request.deliveryDate().isBlank()) {
            throw new IllegalArgumentException(
                    "宅配取號必填 shipDate 與 deliveryDate（PayUni 請求參數）——adapter 不代填");
        }
    }

    /**
     * ✅ 黑貓叫件（{@code callTcat}）已搬入本 adapter。
     *
     * <p>🔴 <b>為什麼不是 {@code empty}</b>：黑貓確實有派車能力，而且它在 production
     * <b>今天就在運作</b>。回 {@code empty} 的話，{@code allowedActions} 會算出「叫件不可用」
     * 並附上 {@code FULFILLMENT_PROVIDER_NOT_ENABLED}，賣家看到的是一顆按不下去的叫件按鈕，
     * 而那句話是假的。
     */
    @Override
    public Optional<DispatchCapability> dispatch(FulfillmentContext ctx) {
        return Optional.of(dispatchCapability);
    }

    private final DispatchCapability dispatchCapability = new DispatchCapability() {
        /**
         * 黑貓預約取件：把配送單 ＋ 賣家寄件設定 ＋ 中性選項翻譯成 {@code LgsCallTcat} 請求。
         *
         * <h3>🔴 這裡做的是 provider 翻譯，<b>只有</b>翻譯</h3>
         * <ul>
         *   <li><b>溫層 → 件數欄位</b>：PayUni 不收「溫層」這個維度，它收三個件數欄位
         *       （{@code NormalQuantity}／{@code ColdQuantity}／{@code FreezeQuantity}），
         *       一筆單只會落在其中一格。這個對照是 PayUni 的請求形狀，遷移前它是寫在
         *       {@code ShippingService} 裡的 {@code if/else if} 鏈——那正是 provider 洩漏</li>
         *   <li><b>寄件人地址組裝</b>：{@code city + district + 正規化後的 detail}
         *       串成一個字串，與逆向建單共用 {@link PayUniHomeAdapter#fullAddress}。
         *       遷移前 service 有一份、adapter 有一份，沒有東西保證它們同步</li>
         *   <li><b>Boolean → {@code "Y"}／{@code "N"}</b>：那兩個字面值是 PayUni 的參數形狀</li>
         *   <li><b>同步 retry 最多一次</b>：收斂規範 §8.1，與 {@code createHomeRefund} 一致</li>
         * </ul>
         *
         * <h3>⚠️ 這裡<b>不</b>做的事（全部留在 {@code ShippingService.callTcat}）</h3>
         * 搶佔（{@code ProviderCallExecutor} 的 acquire／markSucceeded／markFailed）、
         * 狀態轉移與狀態歷程、事件發布、{@code expected_delivery_date} 估算、
         * 以及「賣家設定是否仍有效」的判斷。那些是平台流程，不是 provider 知識。
         *
         * <p>🔴 <b>失敗一律拋 {@link PayUniApiException}</b>（含 retry 後仍失敗）：呼叫端的
         * {@code ProviderCallExecutor} 靠例外走 markFailed，回一個「失敗的 outcome」
         * 會讓搶佔狀態機把失敗記成成功。錯誤訊息優先取 provider 的 {@code ErrorMsg}
         * 明細，只有它是空或哨兵值 {@code "-"} 時才退回泛用 message——反過來會讓賣家
         * 看到「交易失敗」這種說了等於沒說的字。
         */
        @Override
        public DispatchOutcome dispatch(ShipmentEntity shipment,
                                        SellerShipmentSettingEntity sellerSetting,
                                        DispatchOptions options) {
            CallTcatRequest req = new CallTcatRequest(
                    sellerSetting.getSenderName(),
                    sellerSetting.getSenderPhone(),
                    fullAddress(sellerSetting.getSenderCity(),
                            sellerSetting.getSenderDistrict(),
                            sellerSetting.getSenderAddressDetail()),
                    quantityFor(GoodsType.NORMAL, shipment.getGoodsType()),
                    quantityFor(GoodsType.REFRIGERATED, shipment.getGoodsType()),
                    quantityFor(GoodsType.FROZEN, shipment.getGoodsType()),
                    yesNo(options.contactBeforeDelivery()),
                    yesNo(options.needTrolley()),
                    options.memo());

            // 收斂規範 §8.1：同步 retry 最多一次
            CallTcatResponse resp = shippingClient.callTcat(shipment.getMerId(), req);
            if (!resp.isSuccess()) {
                log.warn("PayUniHomeAdapter.dispatch.retry",
                        "黑貓呼叫首次失敗，retry: " + resp.apiResponse().message(),
                        "orderNo", String.valueOf(shipment.getOrderNo()));
                resp = shippingClient.callTcat(shipment.getMerId(), req);
            }
            if (!resp.isSuccess()) {
                String errStatus = resp.apiResponse().status();
                String errMsg = resp.apiResponse().getData("ErrorMsg");
                if (errMsg == null || "-".equals(errMsg) || errMsg.isBlank()) {
                    errMsg = resp.apiResponse().message();
                }
                throw new PayUniApiException(
                        errStatus != null ? errStatus : "TCAT_FAILED", errMsg, 0);
            }

            // providerReference = null：LgsCallTcat 的回應不含取件單號，只有受理時間。
            // 硬湊一個值（例如回填 shipTradeNo）會讓「provider 給了識別」變成假的。
            return new DispatchOutcome(null, resp.apiResponse().message(), resp.callTime());
        }

    };

    /**
     * 溫層 → PayUni 的件數欄位。
     *
     * <p>本筆單的溫層等於 {@code slot} 時回 {@code "1"}，否則回 {@code null}
     * （PayUni 對不適用的溫層欄位收 null，不收 {@code "0"}）。
     *
     * <p>⚠️ 未知或空的 {@code goodsType} 三格都會是 null ——與遷移前的 if/else if 鏈逐字相同
     * （三個條件都不成立 ⇒ 三個變數都留在 null）。這裡刻意不改成「預設常溫」：
     * 那會把一筆資料有問題的單靜默送成常溫件，而 provider 會回 200。
     */
    private static String quantityFor(GoodsType slot, String goodsType) {
        return slot.code().equals(goodsType) ? "1" : null;
    }

    /**
     * Boolean → PayUni 的 {@code "Y"}／{@code "N"}。
     *
     * <p>⚠️ {@code null} 回 {@code null}（不代填）：預設值是平台的產品決定，而且要落我方的
     * {@code shipments.is_contact}／{@code shipments.is_trolley} 欄位 ⇒ 由呼叫端在搶佔那一步決定，
     * adapter 代填會讓 DB 記的與實際送出的不一致。
     */
    private static String yesNo(Boolean flag) {
        return flag == null ? null : (flag ? "Y" : "N");
    }

    /** ❌ 永久 empty：PayUni 的 11 個端點常數實查無 cancel。 */
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
         * 宅配逆物流：買家是退貨人（司機到府取件），賣家的退貨聯絡人是收件人。
         *
         * <p>⚠️ 刻意<b>不帶</b> {@code ShipTradeNo}：帶了 PayUni 會檢查原單狀態，
         * 而退貨常發生在原單已終結之後。
         */
        @Override
        public ReturnOutcome createReturnShipment(ShipmentEntity forwardShipment,
                                                  SellerShipmentSettingEntity sellerSetting,
                                                  ReturnRequest request) {
            String goodsType = PayUniParamMapper.toGoodsType(request.temperature());

            String returnName = firstNonBlank(sellerSetting.getReturnName(), sellerSetting.getSenderName());
            String returnPhone = firstNonBlank(sellerSetting.getReturnPhone(), sellerSetting.getSenderPhone());
            String returnCity = firstNonBlank(sellerSetting.getReturnCity(), sellerSetting.getSenderCity());
            String returnDistrict = firstNonBlank(sellerSetting.getReturnDistrict(), sellerSetting.getSenderDistrict());
            String returnDetail = firstNonBlank(sellerSetting.getReturnAddressDetail(),
                    sellerSetting.getSenderAddressDetail());
            String consigneeAddress = fullAddress(returnCity, returnDistrict, returnDetail);
            String consignorAddress = fullAddress(request.buyerCity(), request.buyerDistrict(),
                    request.buyerAddress());

            String spec = forwardShipment != null && forwardShipment.getSpec() != null
                    && !forwardShipment.getSpec().isBlank()
                    ? forwardShipment.getSpec() : DEFAULT_SPEC;

            CreateHomeRefundRequest payUniReq = new CreateHomeRefundRequest(
                    returnMerTradeNo(request.orderNo()),
                    null,                      // 不帶 ShipTradeNo，避免 PayUni 檢查原單狀態限制
                    goodsType,
                    returnName,                // 收件人 = 賣家退貨聯絡人
                    returnPhone,
                    consigneeAddress,
                    request.buyerName(),       // 退貨人 = 買家
                    request.buyerPhone(),
                    consignorAddress,
                    DeliveryTimeTag.ANY.code(),
                    spec,
                    pickupDate(),
                    "退貨商品",
                    null);

            // 收斂規範 §8.1：同步 retry 最多一次
            CreateHomeRefundResponse resp = shippingClient.createHomeRefund(request.merId(), payUniReq);
            if (!resp.isSuccess()) {
                resp = shippingClient.createHomeRefund(request.merId(), payUniReq);
            }

            ShipmentEntity entity = baseReturnEntity(request, goodsType, consignorAddress,
                    returnName, returnPhone, consigneeAddress);
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
         * <p>🔴 遷移前兩者各有一份副本，而且<b>已經漂移了</b>：成功列的 {@code senderAddress} 是
         * 正規化後的地址，失敗殼卻是 {@code city + district + 原始 detail} 直接串接
         * ——同一筆退貨在成功與失敗時，Admin 看到的地址格式不同。共用之後只有一種。
         */
        private ShipmentEntity baseReturnEntity(ReturnRequest request, String goodsType,
                                                String consignorAddress, String returnName,
                                                String returnPhone, String consigneeAddress) {
            ShipmentEntity entity = new ShipmentEntity();
            entity.setShipmentNo(request.returnShipmentNo());
            entity.setOrderNo(request.orderNo());
            entity.setStoreId(request.storeId());
            entity.setShippingMethod(supportedMethod().value());
            entity.setMerId(request.merId());
            entity.setGoodsType(goodsType);
            entity.setServiceType(ServiceType.NO_CASH_ON_DELIVERY.code());
            entity.setTradeAmt(0L);   // 宅配逆向不代收
            entity.setSenderName(request.buyerName());
            entity.setSenderPhone(request.buyerPhone());
            entity.setSenderCity(request.buyerCity());
            entity.setSenderDistrict(request.buyerDistrict());
            entity.setSenderAddress(consignorAddress);
            entity.setSenderZipCode(request.buyerPostalCode());
            entity.setRecipientName(returnName);
            entity.setRecipientPhone(returnPhone);
            entity.setRecipientAddress(consigneeAddress);
            entity.setIsReturn(true);
            return entity;
        }
    };

    private static String firstNonBlank(String preferred, String fallback) {
        return preferred != null && !preferred.isBlank() ? preferred : fallback;
    }

    private static String fullAddress(String city, String district, String detail) {
        return city + district + AddressNormalizer.normalizeAddressDetail(city, district, detail);
    }

    /**
     * 黑貓退貨取件日。
     *
     * <p>PayUni 規則：16:25 後建單須 D+2 ⇒ 統一取 D+2 避免時區邊界誤判；週日不可取件，順延至週一。
     * 🔴 <b>用台北時區</b>：容器跑 UTC，用 UTC 日期會在台灣時間 08:00 前算出前一天。
     */
    private static String pickupDate() {
        LocalDate date = DateTimeUtils.taipeiPlusDays(2);
        if (date.getDayOfWeek() == DayOfWeek.SUNDAY) {
            date = date.plusDays(1);
        }
        return date.format(PROVIDER_DATE);
    }

    /** ❌ 永久 empty：黑貓無送達證明概念。 */
    @Override
    public Optional<PodCapability> pod(FulfillmentContext ctx) {
        return Optional.empty();
    }

    /**
     * ✅ 已搬移：送審委派 {@code OnboardingService.onboardHomeDelivery}，
     * {@code shippingTypeValue()} 取代 {@code mapShippingMethodToType} 的字面分支。
     */
    @Override
    public Optional<OnboardCapability> onboard(FulfillmentContext ctx) {
        return Optional.of(onboardCapability);
    }

    private final OnboardCapability onboardCapability = new OnboardCapability() {
        /**
         * ⚠️ {@code submit} 與 {@code resubmit} 委派同一個入口：
         * {@code onboardHomeDelivery} 內部先查既有設定，有就 resubmit、沒有就首次送審。
         * 由 adapter 自己判斷走哪一條，等於在 adapter 裡複製一份業務判斷（違反硬規則 1）。
         */
        @Override
        public OnboardOutcome submit(SellerShipmentSettingEntity setting) {
            SellerShipmentSettingEntity saved = onboardingServiceProvider.getObject().onboardHomeDelivery(
                    setting.getStoreId(),
                    setting.getProductCategoryCode(),
                    setting.getSenderName(),
                    setting.getSenderPhone(),
                    setting.getSenderCity(),
                    setting.getSenderDistrict(),
                    setting.getSenderAddressDetail(),
                    setting.getSenderPostalCode(),
                    setting.getReturnName(),
                    setting.getReturnPhone(),
                    setting.getReturnCity(),
                    setting.getReturnDistrict(),
                    setting.getReturnAddressDetail(),
                    setting.getReturnPostalCode());
            // provider 側識別：宅配建店回的是 eshipId（超商是 partnerId）
            return new OnboardOutcome(
                    saved.getEshipId(), saved.getPayuniSubmitStatus(), saved.getProviderMessage());
        }

        @Override
        public OnboardOutcome resubmit(SellerShipmentSettingEntity setting) {
            return submit(setting);
        }

        @Override
        public String shippingTypeValue() {
            return ShippingType.HOME_DELIVERY.value();
        }
    };

    /** ❌ 永久 empty：宅配沒有門市。 */
    @Override
    public Optional<StoreLocatorCapability> storeLocator(FulfillmentContext ctx) {
        return Optional.empty();
    }
}
