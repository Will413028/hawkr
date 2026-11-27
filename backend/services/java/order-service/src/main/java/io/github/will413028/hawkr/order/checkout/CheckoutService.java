package io.github.will413028.hawkr.order.checkout;

import io.github.will413028.hawkr.common.utils.code.CollectionModel;
import java.util.UUID;
import io.github.will413028.hawkr.common.utils.code.GrpcResponseCode;
import io.github.will413028.hawkr.order.exception.OrderBusinessException;
import io.github.will413028.hawkr.common.obs.log.ObsLogger;
import io.github.will413028.hawkr.common.utils.code.ShippingMethod;
import io.github.will413028.hawkr.common.utils.code.ShippingRoute;
import io.github.will413028.hawkr.common.utils.code.ShippingTemperature;
import io.github.will413028.hawkr.common.utils.code.WeightUnit;
import io.github.will413028.hawkr.common.utils.code.CartItemUnavailableReason;
import io.github.will413028.hawkr.common.utils.code.helper.ProductPurchasabilityRules;
import io.github.will413028.hawkr.common.utils.proto.ProtoConversionUtils;
import io.github.will413028.hawkr.common.utils.shipping.SellerShippingFilter;
import io.github.will413028.hawkr.common.utils.shipping.ShippingAreaResolver;
import io.github.will413028.hawkr.common.utils.shipping.WeightConverter;
import io.github.will413028.hawkr.common.utils.validation.InputValidator;
import io.github.will413028.hawkr.common.utils.validation.PayUniShippingConstraints;
import io.github.will413028.hawkr.coupon.grpc.CouponValidateResult;
import io.grpc.StatusRuntimeException;
import io.github.will413028.hawkr.order.checkout.CheckoutValidator.*;
import io.github.will413028.hawkr.order.checkout.client.CheckoutCouponGrpcClient;
import io.github.will413028.hawkr.order.service.CouponReleaseService;
import io.github.will413028.hawkr.common.grpc.PrincipalResolution;
import io.github.will413028.hawkr.common.grpc.UnresolvedPrincipalException;
import io.github.will413028.hawkr.order.checkout.client.SellerGrpcClient;
import io.github.will413028.hawkr.order.grpc.*;
import io.github.will413028.hawkr.order.grpc.client.BuyerGrpcClient;
import io.github.will413028.hawkr.common.utils.code.InvoiceCarrierType;
import io.github.will413028.hawkr.invoice.grpc.InvoiceAmountBreakdown;
import io.github.will413028.hawkr.invoice.grpc.ResolveOrderInvoiceModeInternalData;
import io.github.will413028.hawkr.invoice.grpc.ResolveOrderInvoiceModeInternalRequest;
import io.github.will413028.hawkr.invoice.grpc.ResolveOrderInvoiceModeInternalResponse;
import io.github.will413028.hawkr.invoice.grpc.InvoiceInputSpec;
import io.github.will413028.hawkr.invoice.grpc.ValidateOrderInvoiceInputInternalRequest;
import io.github.will413028.hawkr.invoice.grpc.ValidateOrderInvoiceInputInternalResponse;
import io.github.will413028.hawkr.order.grpc.client.InvoiceGrpcClient;
import io.github.will413028.hawkr.order.grpc.client.ProductGrpcClient;
import io.github.will413028.hawkr.order.grpc.client.ShippingGrpcClient;
import io.github.will413028.hawkr.buyer.grpc.AddressInfo;
import io.github.will413028.hawkr.shipping.grpc.GetSellerShippingConfigResponse;
import io.github.will413028.hawkr.shipping.grpc.SellerShippingConfigItem;
import io.github.will413028.hawkr.order.exception.CheckoutExpiredException;
import io.github.will413028.hawkr.order.exception.CheckoutPriceChangedException;
import io.github.will413028.hawkr.order.exception.UnavailableItemsException;
import io.github.will413028.hawkr.order.mq.PaymentEventHandler;
import io.github.will413028.hawkr.order.service.OrderService;
import io.github.will413028.hawkr.product.grpc.BatchGetProductForCheckoutResponse;
import io.github.will413028.hawkr.product.grpc.GetProductResponse;
import io.github.will413028.hawkr.product.grpc.ProductForCheckoutInfo;
import io.github.will413028.hawkr.product.grpc.ProductDetail;
import io.github.will413028.hawkr.product.grpc.ProductSku;
import io.github.will413028.hawkr.seller.grpc.SellerMeta;
import com.fasterxml.jackson.databind.JsonNode;
import com.fasterxml.jackson.databind.ObjectMapper;
import org.springframework.beans.factory.annotation.Value;
import org.springframework.data.redis.core.StringRedisTemplate;
import org.springframework.stereotype.Service;

import java.time.Duration;
import java.util.*;
import java.util.stream.Collectors;

/**
 * 結帳模組主服務
 *
 * 提供 4 支結帳 API：validate / shippingFee / preview / confirm
 */
@Service
public class CheckoutService {

    private static final ObsLogger log = ObsLogger.forClass(CheckoutService.class);

    private static final String CHECKOUT_KEY_PREFIX = "checkout:";
    private static final Duration CHECKOUT_TTL = Duration.ofMinutes(30);
    private static final ObjectMapper objectMapper = new ObjectMapper();

    private final ProductGrpcClient productGrpcClient;
    private final SellerGrpcClient sellerGrpcClient;
    private final BuyerGrpcClient buyerGrpcClient;
    private final ShippingGrpcClient shippingGrpcClient;
    private final CheckoutCouponGrpcClient couponGrpcClient;
    private final CouponReleaseService couponReleaseService;
    private final PaymentEventHandler paymentEventHandler;
    private final InvoiceGrpcClient invoiceGrpcClient;
    private final CheckoutValidator checkoutValidator;
    private final OrderService orderService;
    private final OrderPolicyResolver orderPolicyResolver;
    private final StringRedisTemplate redisTemplate;

    public CheckoutService(ProductGrpcClient productGrpcClient,
                           SellerGrpcClient sellerGrpcClient,
                           BuyerGrpcClient buyerGrpcClient,
                           ShippingGrpcClient shippingGrpcClient,
                           CheckoutCouponGrpcClient couponGrpcClient,
                           InvoiceGrpcClient invoiceGrpcClient,
                           CheckoutValidator checkoutValidator,
                           OrderService orderService,
                           OrderPolicyResolver orderPolicyResolver,
                           StringRedisTemplate redisTemplate,
                           CouponReleaseService couponReleaseService,
                           PaymentEventHandler paymentEventHandler) {
        this.productGrpcClient = productGrpcClient;
        this.sellerGrpcClient = sellerGrpcClient;
        this.buyerGrpcClient = buyerGrpcClient;
        this.shippingGrpcClient = shippingGrpcClient;
        this.couponGrpcClient = couponGrpcClient;
        this.invoiceGrpcClient = invoiceGrpcClient;
        this.checkoutValidator = checkoutValidator;
        this.orderService = orderService;
        this.orderPolicyResolver = orderPolicyResolver;
        this.redisTemplate = redisTemplate;
        this.couponReleaseService = couponReleaseService;
        this.paymentEventHandler = paymentEventHandler;
    }


    /**
     * 自買防護（共用 helper，§4.1/§4.3/§4.4）
     * resolve-to-principal（Plan 2b）：任一 item 的 storeId 解析出的 principal
     * == 買家 principal 即拋 SELF_PURCHASE_NOT_ALLOWED (4015)。
     *
     * <p><b>兩側都必須經過解析</b>（issue #2789 階段 B2）：本方法收到的 {@code buyerId} 是
     * <b>buyer_id 代理鍵</b>（44 號設計書的營運鍵雙分離），而 seller 軸解出來的是
     * <b>account_id 登入 principal</b>。舊寫法直接拿 raw {@code buyerId} 去比 account_id，
     * 是<b>拿兩個不同身分空間的值做等價比較</b> —— 今天只因為 grandfathered
     * {@code buyer_id == account_id} 而碰巧成立，<b>買家 re-key 之後這道防護就靜默失效</b>
     * （比不中 ⇒ 永遠放行）。這與 echo-back 是兩個獨立的 root cause：那個是回吐了未驗證的值，
     * 這個是<b>少做了一次解析</b>。
     */
    private void enforceNoSelfPurchase(long buyerId, List<ResolvedItem> resolved) {
        enforceNoSelfPurchaseForTest(buyerId, resolved);
    }

    void enforceNoSelfPurchaseForTest(long buyerId, List<ResolvedItem> resolved) {
        List<Long> storeIds = resolved.stream()
                .map(ResolvedItem::storeId)
                .distinct()
                .toList();
        PrincipalResolution sellerResolution;
        PrincipalResolution buyerResolution;
        try {
            sellerResolution = sellerGrpcClient.resolvePrincipals(storeIds);
            buyerResolution = buyerGrpcClient.resolvePrincipals(List.of(buyerId));
        } catch (StatusRuntimeException e) {
            log.error("CheckoutService.enforceNoSelfPurchase.resolvePrincipalsFail",
                    "Seller / Buyer Service principal 解析失敗", e);
            throw new OrderBusinessException(GrpcResponseCode.SERVICE_UNAVAILABLE);
        }
        Map<Long, Long> storePrincipals;
        long buyerPrincipal;
        try {
            storePrincipals = sellerResolution.requireAllResolved(storeIds, "store_id");
            buyerPrincipal = buyerResolution.requireResolved(buyerId, "buyer_id");
        } catch (UnresolvedPrincipalException e) {
            log.warn("CheckoutService.enforceNoSelfPurchase.principalUnresolved",
                    "store_id / buyer_id 查無在架身分，自買防護無法評估，fail-closed 拒絕結帳："
                            + e.getMessage());
            throw new OrderBusinessException(GrpcResponseCode.SKU_NOT_FOUND);
        }
        boolean hasSelfOwned = resolved.stream()
                .anyMatch(item -> buyerPrincipal == storePrincipals.get(item.storeId()));
        if (hasSelfOwned) {
            throw new OrderBusinessException(GrpcResponseCode.SELF_PURCHASE_NOT_ALLOWED);
        }
    }


    /**
     * 即時驗證（購物車勾選時呼叫，輕量級，預設 DOMESTIC 運費）
     * 不呼叫 Seller Service（無需精確路線）
     */
    public CheckoutValidateData validate(long buyerId, List<CheckoutItem> items) {
        log.info("CheckoutService.validate.start", "即時驗證");

        ResolveResult resolveResult;
        try {
            resolveResult = resolveItems(items);
        } catch (Exception e) {
            log.warn("CheckoutService.validate.resolveItems.failed", "商品解析失敗: " + e.getMessage());
            return CheckoutValidateData.newBuilder()
                    .setValid(false)
                    .addErrors(CheckoutValidationError.newBuilder()
                            .setErrorCode("INVALID_ITEM")
                            .setErrorMessage(e.getMessage())
                            .build())
                    .build();
        }
        List<ResolvedItem> resolved = resolveResult.resolved();
        List<UnavailableItemInfo> unavailable = resolveResult.unavailable();

        if (resolved.isEmpty()) {
            return CheckoutValidateData.newBuilder()
                    .setValid(false)
                    .addAllUnavailableItems(toProtoUnavailable(unavailable))
                    .build();
        }

        enforceNoSelfPurchase(buyerId, resolved);

        ValidationResult result = checkoutValidator.validate(resolved, ShippingRoute.DOMESTIC);

        long storeId = resolved.get(0).storeId();
        Set<String> sellerAvailableMethods = resolveSellerAvailableMethods(storeId);

        requireSellerAcceptingOrders(sellerGrpcClient.getSellerMeta(storeId));

        boolean valid = result.valid() && unavailable.isEmpty();
        CheckoutValidateData.Builder builder = CheckoutValidateData.newBuilder()
                .setValid(valid)
                .addAllDeliveryTempIntersection(result.deliveryTempIntersection())
                .addAllUnavailableItems(toProtoUnavailable(unavailable));

        for (AvailableOption opt : result.availableOptions()) {
            if (!SellerShippingFilter.isAvailable(opt.method().value(), opt.temp().value(), sellerAvailableMethods)) {
                continue;
            }
            builder.addAvailableShippingOptions(AvailableShippingOption.newBuilder()
                    .setShippingMethod(opt.method().value())
                    .setDeliveryTemp(opt.temp().value())
                    .setLabel(opt.label())
                    .setFee(String.valueOf(opt.fee()))
                    .setFeePayer(opt.feePayer())
                    .setFreeShippingThreshold(String.valueOf(opt.freeThreshold()))
                    .setTentativeFreeShippingEligible(opt.freeShippingEligible())
                    .build());
        }

        for (ValidationError err : result.errors()) {
            builder.addErrors(CheckoutValidationError.newBuilder()
                    .setErrorCode(err.errorCode())
                    .setErrorMessage(err.errorMessage())
                    .build());
        }

        log.info("CheckoutService.validate.done",
                "即時驗證完成 valid=" + valid + " unavailableCount=" + unavailable.size());
        return builder.build();
    }

    /** UnavailableItemInfo (domain record) → UnavailableItem (proto)，供三 caller 統一組裝 */
    private static List<UnavailableItem> toProtoUnavailable(List<UnavailableItemInfo> infos) {
        List<UnavailableItem> out = new ArrayList<>(infos.size());
        for (UnavailableItemInfo u : infos) {
            out.add(UnavailableItem.newBuilder()
                    .setSkuId(u.skuId())
                    .setSkuName(u.skuName())
                    .setReason(u.reason())
                    .setRequestedQty(u.requestedQty())
                    .setAvailableQty(u.availableQty())
                    .build());
        }
        return out;
    }


    /**
     * 取一筆 Uber 即時報價（Stage 8B W2-2）。
     *
     * <h2>🔴 業務失敗一律回 {@code available=false} ＋ 理由碼，不拋例外</h2>
     * 呼叫端是結帳頁的配送選項區。「Uber 算不出價」不是結帳失敗——買家還有黑貓與超商可選，
     * 拋例外會讓整個選項計算一起垮掉、<b>三個配送方式同時消失</b>。
     * 這條規則與 {@code ShippingGrpcServiceImpl.getUberQuoteForCheckout} 的同一條是配套的：
     * 下游已經把業務失敗轉成 {@code available=false}，這一層若把它轉回例外等於白做。
     *
     * <p>⚠️ <b>2026-09-12 起 shipping 端的基礎設施失敗也不再往上拋</b>（原本寫的是「仍往上拋」）。
     * 理由：資格判定收斂到 {@link UberOrderEligibilityGuard} 之後，「問不到 shipping」必須
     * <b>fail-closed 成拒絕</b>——讓例外穿過閘門，等於讓一次逾時決定一張 Uber 訂單能不能建。
     * 本端點因此回 {@code available=false} ＋
     * {@link UberOrderEligibilityGuard.ProviderReason#ASSESSOR_UNAVAILABLE}。
     * 🔴 那個理由碼<b>與任何業務理由都不同字串</b>，且 {@code CheckoutService.uberQuote.unavailable}
     * 這一行 log 會帶著它——「shipping 掛了」與「賣家沒開通」在可觀測性上仍然分得開，
     * 變的只是 HTTP 狀態（500 → 200＋具名理由）。
     * ⚠️ {@code resolveItems} 的下游例外（product-service 掛了）<b>仍然往上拋</b>，未受此變更影響。
     *
     * <h2>賣家與溫層都從 SKU 推導，不由呼叫端指定</h2>
     * Uber 是<b>單店取件</b>：一趟車只去一個 {@code external_store_id}，所以跨店購物車
     * 叫不了同一趟車（{@code MULTI_SELLER_CART}）。溫層則決定分箱政策，混溫層無法用同一個
     * 保溫規格送（{@code MIXED_TEMPERATURE}）。
     * 兩項判定都委給 {@link CheckoutValidator#validate}——它是「賣家一致性、溫層交集」的既有
     * 唯一實作，在這裡重寫一份會讓兩邊的規則各自演化。
     *
     * @param buyerId   買家業務鍵（buyers PK）
     * @param addressId 買家收件地址 id（{@code shipping_addresses.id}）
     * @param items     當前已勾選的商品
     */
    public OrderUberQuoteData uberQuote(long buyerId, String addressId, List<CheckoutItem> items) {
        log.info("CheckoutService.uberQuote.start", "Uber 即時報價",
                "itemCount", String.valueOf(items == null ? 0 : items.size()));

        if (items == null || items.isEmpty()) {
            return uberUnavailable("EMPTY_ITEMS");
        }

        ResolveResult resolveResult;
        try {
            resolveResult = resolveItems(items);
        } catch (StatusRuntimeException e) {
            throw e;
        } catch (Exception e) {
            log.warn("CheckoutService.uberQuote.resolveItems.failed", "商品解析失敗: " + e.getMessage());
            return uberUnavailable("INVALID_ITEM");
        }
        List<ResolvedItem> resolved = resolveResult.resolved();
        if (resolved.isEmpty() || !resolveResult.unavailable().isEmpty()) {
            return uberUnavailable("ITEMS_UNAVAILABLE");
        }

        ValidationResult validation = checkoutValidator.validate(resolved, ShippingRoute.DOMESTIC);
        for (ValidationError err : validation.errors()) {
            if ("DIFFERENT_SELLER".equals(err.errorCode())) {
                return uberUnavailable("MULTI_SELLER_CART");
            }
            if ("TEMP_ZONE_INCOMPATIBLE".equals(err.errorCode())) {
                return uberUnavailable("MIXED_TEMPERATURE");
            }
        }

        String deliveryTemp = coldestTemperature(resolved);
        if (!validation.deliveryTempIntersection().contains(deliveryTemp)) {
            return uberUnavailable("MIXED_TEMPERATURE");
        }

        long addressIdValue;
        try {
            addressIdValue = Long.parseLong(addressId);
        } catch (NumberFormatException e) {
            return uberUnavailable("ADDRESS_NOT_FOUND");
        }
        AddressInfo address = buyerGrpcClient.getAddress(buyerId, addressIdValue);
        if (address == null) {
            return uberUnavailable("ADDRESS_NOT_FOUND");
        }

        long storeId = resolved.get(0).storeId();

        UberOrderEligibilityGuard.Decision decision = checkoutValidator.assessUberQuote(
                UberOrderEligibilityGuard.Probe.forQuote(
                        buyerId, storeId, deliveryTemp,
                        address.getReceiverName(), address.getReceiverPhone(),
                        address.getCity(), address.getDistrict(), address.getDetailAddress(),
                        toProbeItems(resolved)));

        if (!decision.allowed()) {
            log.info("CheckoutService.uberQuote.unavailable",
                    "Uber 不可用：" + decision.denyReason(),
                    "reason", decision.providerReason());
            String reason = decision.providerReason().isEmpty()
                    ? UberOrderEligibilityGuard.ProviderReason.ASSESSOR_UNAVAILABLE
                    : decision.providerReason();
            return uberUnavailable(reason);
        }

        UberOrderEligibilityGuard.Assessment assessment = decision.assessment();
        log.info("CheckoutService.uberQuote.done",
                "Uber 即時報價完成 fee=" + assessment.feeAmount(),
                "quoteId", assessment.quoteId());
        return OrderUberQuoteData.newBuilder()
                .setAvailable(true)
                .setQuoteId(assessment.quoteId())
                .setFeeAmount(assessment.feeAmount())
                .setCurrency(assessment.currency())
                .setExpiresAt(assessment.expiresAt())
                .build();
    }

    /**
     * 不可用的 {@link OrderUberQuoteData}。
     *
     * <p>🔴 {@code quoteId} / {@code feeAmount} 一律留空：用「fee=0」表示不可用，會與
     * 「0 元運費」（平台補貼）逐位元無法區分（order.proto 與 shipping.proto 兩側都逐字寫了這一條）。
     */
    private static OrderUberQuoteData uberUnavailable(String reason) {
        return OrderUberQuoteData.newBuilder()
                .setAvailable(false)
                .setUnavailableReason(reason == null ? "" : reason)
                .build();
    }

    /** 整車溫層 ＝ 各品項儲存溫層中最冷的那一個（FROZEN &gt; CHILLED &gt; AMBIENT）。 */
    private static String coldestTemperature(List<ResolvedItem> resolved) {
        boolean hasFrozen = false;
        boolean hasChilled = false;
        for (ResolvedItem item : resolved) {
            String temp = item.inferredTemperature();
            if (ShippingTemperature.FROZEN.value().equals(temp)) {
                hasFrozen = true;
            } else if (ShippingTemperature.CHILLED.value().equals(temp)) {
                hasChilled = true;
            }
        }
        if (hasFrozen) return ShippingTemperature.FROZEN.value();
        if (hasChilled) return ShippingTemperature.CHILLED.value();
        return ShippingTemperature.AMBIENT.value();
    }

    /**
     * {@link ResolvedItem} → {@link UberOrderEligibilityGuard.Item}。
     *
     * <p>重量取 {@link ResolvedItem#weightGram()}——它在 {@code resolveItems} 已經由
     * common-core 的 {@code WeightConverter} 換算過（§9：運費相關的單位換算不得各自實作），
     * 這裡再換一次就是第二份實作。
     *
     * <p>⚠️ 轉成的是<b>閘門的中性型別</b>而不是 proto：proto → wire 的那一步在
     * {@code ShippingUberQuoteAssessor}，只有一份。這裡若直接組 proto，會讓
     * 「怎麼填 UberQuoteItem」在兩個檔案各有一份，而它們遲早會不一樣。
     */
    private static List<UberOrderEligibilityGuard.Item> toProbeItems(List<ResolvedItem> resolved) {
        List<UberOrderEligibilityGuard.Item> out = new ArrayList<>(resolved.size());
        for (ResolvedItem item : resolved) {
            out.add(new UberOrderEligibilityGuard.Item(
                    item.skuId(),
                    item.skuName(),
                    item.quantity(),
                    item.lengthCm(),
                    item.widthCm(),
                    item.heightCm(),
                    item.weightGram(),
                    item.unitPrice()));
        }
        return out;
    }

    /**
     * 結帳鏈（preview／confirm）的 Uber 報價 probe。
     *
     * <p>非 3003 一律回 {@code null}——本閘門不介入其他配送方式。
     * 3003 但組不出 probe（沒帶 {@code address_id}、id 不是數字、地址查不到）也回 {@code null}，
     * 由 {@link #uberContext} 轉成 {@code methodOnly} ⇒ 依 deny-by-default 拒絕。
     *
     * <p>🔴 <b>地址走 buyer-service 取三欄，不從 {@code shipping_address} 反解。</b>
     * {@code CheckoutConfirmRequest.shipping_address} 是一整串字串，把它塞進
     * {@code detail_address} 會讓 shipping 去猜縣市與鄉鎮市區——而每一筆猜不出來的，
     * 就是一個看不到選項的買家（{@code shipping.proto} 的 {@code UberQuoteDropoff} 逐字禁止）。
     * 這裡用的是與 {@code POST /app/checkout/uber_quote} <b>同一個來源</b>，
     * 所以「報價頁說可以」與「結帳說可以」不會是兩個答案。
     */
    private UberOrderEligibilityGuard.Probe uberProbeOrNull(
            ShippingMethod method, long buyerId, long storeId, String deliveryTemp,
            String addressId, String uberQuoteId, List<ResolvedItem> resolved) {
        if (method != ShippingMethod.UBER_INSTANT) {
            return null;
        }
        if (addressId == null || addressId.isBlank()) {
            log.warn("CheckoutService.uberProbe.noAddressId",
                    "買家指名 3003 但未帶 address_id，無從組出 dropoff", "buyerId", String.valueOf(buyerId));
            return null;
        }
        long addressIdValue;
        try {
            addressIdValue = Long.parseLong(addressId);
        } catch (NumberFormatException e) {
            log.warn("CheckoutService.uberProbe.addressIdNotNumeric",
                    "address_id 不是數字字串：" + addressId, "buyerId", String.valueOf(buyerId));
            return null;
        }
        AddressInfo address = buyerGrpcClient.getAddress(buyerId, addressIdValue);
        if (address == null) {
            log.warn("CheckoutService.uberProbe.addressNotFound",
                    "查無此收件地址 addressId=" + addressId, "buyerId", String.valueOf(buyerId));
            return null;
        }
        return UberOrderEligibilityGuard.Probe.forCheckout(
                buyerId, storeId, deliveryTemp,
                address.getReceiverName(), address.getReceiverPhone(),
                address.getCity(), address.getDistrict(), address.getDetailAddress(),
                toProbeItems(resolved),
                uberQuoteId == null ? "" : uberQuoteId);
    }

    /** probe 為 {@code null} ⇒ {@code methodOnly}（沒有證據就拒絕），否則帶著它問。 */
    private static UberOrderEligibilityGuard.Context uberContext(
            ShippingMethod method, UberOrderEligibilityGuard.Probe probe) {
        return probe == null
                ? UberOrderEligibilityGuard.Context.methodOnly(method.value())
                : UberOrderEligibilityGuard.Context.of(method.value(), probe);
    }

    /**
     * 建單成功後把報價鎖定成 {@code CHECKOUT_ACCEPTED}；鎖不到就取消這張單。
     *
     * <p>🔴 <b>為什麼失敗要取消而不是放行</b>：{@code UberDirectAdapter} 建單時只認
     * {@code acceptedQuote(orderNo)}，鎖不到就會走 {@code markCreateFailed("UBER_QUOTE_NOT_ACCEPTED")}
     * ——買家付了錢而 shipment 建不出來。與其留一張注定失敗的單，不如當場失敗讓買家重報價。
     *
     * <p>🔴 <b>{@code price_changed} 也走這條</b>（shipping 對它回 valid=false）：
     * 自動接受新價＝對買家補收（違約，INV-U1「永不補收」）；自動沿用舊價＝我方吃差額且無上限。
     * 兩個都不能默默做。
     *
     * <p>⚠️ 這裡有一個誠實標明的窗口：取消訂單若也失敗，會留下一張 PENDING 的單
     * 與一筆沒鎖到的報價。它不會靜默——兩行 error log 都有 orderNo，
     * 且訂單逾期取消排程會把它收掉。
     */
    private void acceptUberQuoteOrCancel(UberOrderEligibilityGuard.Probe base,
                                         String quoteId, String orderNo, long buyerId) {
        UberOrderEligibilityGuard.Decision decision = checkoutValidator.acceptUberQuote(
                UberOrderEligibilityGuard.Probe.accepting(base, quoteId, orderNo));
        if (decision.allowed()) {
            log.info("CheckoutService.confirm.uberAccepted",
                    "Uber 報價已鎖定 quoteId=" + decision.assessment().quoteId(), "orderNo", orderNo);
            return;
        }
        log.error("CheckoutService.confirm.uberAccept.failed",
                "Uber 報價鎖定失敗，取消訂單：" + decision.denyReason(), "orderNo", orderNo);
        try {
            orderService.cancelOrder(buyerId, orderNo);
            orderService.releaseStockAfterCancel(orderNo);
        } catch (Exception compensateErr) {
            log.error("CheckoutService.confirm.uberAccept.compensateFailed",
                    "取消訂單失敗（orderNo=" + orderNo + "）", compensateErr);
        }
        throw new OrderBusinessException(GrpcResponseCode.FULFILLMENT_PROVIDER_NOT_ENABLED);
    }


    /**
     * 運費精算（選配送方式 + 填地址/選門市後呼叫）
     * 硬依賴 Shipping Service（取賣家寄件郵遞區號推導路線）
     */
    public CheckoutShippingFeeData shippingFee(List<CheckoutItem> items,
                                                String shippingMethodCode, String deliveryTempCode,
                                                String receiverZipCode, String cvsStoreId) {
        log.info("CheckoutService.shippingFee.start", "運費精算");
        ResolveResult shippingResolve = resolveItems(items);
        if (shippingResolve.resolved().isEmpty()) {
            throw new UnavailableItemsException(shippingResolve.unavailable());
        }
        List<ResolvedItem> resolved = shippingResolve.resolved();

        if (shippingMethodCode == null || shippingMethodCode.isEmpty()) {
            throw new IllegalArgumentException("配送方式不可為空");
        }
        if (deliveryTempCode == null || deliveryTempCode.isEmpty()) {
            throw new IllegalArgumentException("配送溫層不可為空");
        }
        ShippingMethod method = ShippingMethod.fromValue(shippingMethodCode);
        ShippingTemperature temp = ShippingTemperature.fromValue(deliveryTempCode);

        long storeId = resolved.get(0).storeId();
        requireSellerOnboarded(storeId, shippingMethodCode);

        String senderPostalCode = getSenderPostalCode(storeId, shippingMethodCode, deliveryTempCode);
        ShippingRoute route = resolveRoute(method, senderPostalCode, receiverZipCode);

        AvailableOption option = checkoutValidator.lookupSingleOption(
                resolved, method, temp, route,
                UberOrderEligibilityGuard.Context.methodOnly(method.value()));
        if (option == null) {
            throw new OrderBusinessException(GrpcResponseCode.SHIPPING_METHOD_UNAVAILABLE);
        }

        long orderSubtotal = calcSubtotal(resolved);
        long estimatedTotal = orderSubtotal + option.fee();

        log.info("CheckoutService.shippingFee.done", "運費精算完成",
                "fee", String.valueOf(option.fee()));
        return CheckoutShippingFeeData.newBuilder()
                .setShippingMethod(shippingMethodCode)
                .setDeliveryTemp(deliveryTempCode)
                .setShippingRoute(route.value())
                .setShippingFee(String.valueOf(option.fee()))
                .setFeePayer(option.feePayer())
                .setTentativeFreeShippingEligible(option.freeShippingEligible())
                .setFreeShippingThreshold(String.valueOf(option.freeThreshold()))
                .setOrderSubtotal(String.valueOf(orderSubtotal))
                .setEstimatedTotal(String.valueOf(estimatedTotal))
                .build();
    }


    /**
     * 結帳預覽（完整驗證 + 精算總額 + 優惠券驗證，不建單）
     * 生成 checkout_id 供 Confirm 使用
     */
    public CheckoutPreviewData preview(List<CheckoutItem> items,
                                        String shippingMethodCode, String deliveryTempCode,
                                        String receiverZipCode, String cvsStoreId,
                                        String couponTemplateId, long buyerId,
                                        boolean consolidated,
                                        String addressId, String uberQuoteId) {
        log.info("CheckoutService.preview.start", "結帳預覽",
                "consolidated", String.valueOf(consolidated));
        ResolveResult resolveResult = resolveItems(items);
        List<ResolvedItem> resolved = resolveResult.resolved();
        List<UnavailableItemInfo> unavailable = resolveResult.unavailable();

        if (resolved.isEmpty()) {
            throw new UnavailableItemsException(unavailable);
        }
        enforceNoSelfPurchase(buyerId, resolved);

        if (!unavailable.isEmpty()) {
            throw new UnavailableItemsException(unavailable);
        }

        long storeId = resolved.get(0).storeId();
        requireSellerOnboarded(storeId, shippingMethodCode);

        String senderPostalCode = getSenderPostalCode(storeId, shippingMethodCode, deliveryTempCode);

        SellerMeta sellerMeta = sellerGrpcClient.getSellerMeta(storeId);

        requireSellerAcceptingOrders(sellerMeta);

        if (shippingMethodCode == null || shippingMethodCode.isEmpty()) {
            throw new IllegalArgumentException("配送方式不可為空");
        }
        if (deliveryTempCode == null || deliveryTempCode.isEmpty()) {
            throw new IllegalArgumentException("配送溫層不可為空");
        }
        ShippingMethod method = ShippingMethod.fromValue(shippingMethodCode);
        ShippingTemperature temp = ShippingTemperature.fromValue(deliveryTempCode);
        ShippingRoute route = resolveRoute(method, senderPostalCode, receiverZipCode);

        ValidationResult validationResult = checkoutValidator.validate(resolved, route);
        if (!validationResult.valid()) {
            String msg = validationResult.errors().stream()
                    .map(ValidationError::errorMessage)
                    .collect(Collectors.joining("; "));
            throw new OrderBusinessException(GrpcResponseCode.CHECKOUT_VALIDATION_FAILED, msg);
        }

        AvailableOption option = checkoutValidator.lookupSingleOption(
                resolved, method, temp, route,
                uberContext(method, uberProbeOrNull(
                        method, buyerId, storeId, temp.value(), addressId, uberQuoteId, resolved)));
        if (option == null) {
            throw new OrderBusinessException(GrpcResponseCode.SHIPPING_METHOD_UNAVAILABLE);
        }

        long orderSubtotal = calcSubtotal(resolved);
        long discountAmount = 0;
        if (couponTemplateId != null && !couponTemplateId.isEmpty()) {
            CouponValidateResult couponResult = couponGrpcClient.validateCoupon(
                    couponTemplateId, buyerId, storeId, orderSubtotal);
            discountAmount = InputValidator.requireNonNegativeAmount(couponResult.getDiscountAmount(), "折扣金額");
        }

        String checkoutId = UUID.randomUUID().toString();

        Map<UUID, Long> shareDiscountBySku = allocateShareDiscount(resolved, discountAmount);

        var finalFsr = checkoutValidator.checkFreeShipping(
                resolved, shippingMethodCode, deliveryTempCode, shareDiscountBySku);
        long finalShippingFee = finalFsr.eligible() ? 0L : option.rawFee();

        List<CheckoutItemPreview> itemPreviews = resolved.stream()
                .map(r -> {
                    long subtotal = r.unitPrice() * r.quantity();
                    long itemDiscount = shareDiscountBySku.getOrDefault(r.skuId(), 0L);
                    String salePriceStr = r.unitPrice() < r.listPrice()
                            ? String.valueOf(r.unitPrice()) : "";
                    return CheckoutItemPreview.newBuilder()
                            .setSkuId(String.valueOf(r.skuId()))
                            .setProductId(r.productId().toString())
                            .setSkuName(r.skuName())
                            .setQuantity(String.valueOf(r.quantity()))
                            .setPrice(String.valueOf(r.unitPrice()))
                            .setSubtotal(String.valueOf(subtotal))
                            .setStockQty(String.valueOf(r.stockQty()))
                            .setTemperature(r.inferredTemperature())
                            .setListPrice(String.valueOf(r.listPrice()))
                            .setSalePrice(salePriceStr)
                            .setShareDiscount(String.valueOf(itemDiscount))
                            .setActualPayAmount(String.valueOf(subtotal - itemDiscount))
                            .build();
                })
                .toList();

        OrderGroupPreview group = OrderGroupPreview.newBuilder()
                .setGroupIndex("0")
                .setStoreId(String.valueOf(storeId))
                .setSellerName(sellerMeta.getSellerName())
                .setStoreCode(sellerMeta.getStoreCode())
                .setShippingMethod(shippingMethodCode)
                .setDeliveryTemp(deliveryTempCode)
                .setShippingLabel(option.label())
                .setShippingFee(String.valueOf(finalShippingFee))
                .setShippingFeePayer(option.feePayer())
                .setFreeShippingEligible(finalFsr.eligible())
                .setTentativeFreeShippingEligible(option.freeShippingEligible())
                .addAllItems(itemPreviews)
                .setSubtotalOriginalAmount(String.valueOf(orderSubtotal))
                .setDiscountAmount(String.valueOf(discountAmount))
                .build();

        long totalActualAmount = Math.max(0, orderSubtotal + finalShippingFee - discountAmount);

        OrderPolicySnapshot policySnapshot = orderPolicyResolver.resolve(method.value());
        requireSupportedCollectionModel(policySnapshot);

        String snapshotJson = buildPriceSnapshot(orderSubtotal, finalShippingFee, discountAmount,
                totalActualAmount, finalFsr.eligible(), option.freeShippingEligible(),
                policySnapshot);
        redisTemplate.opsForValue().set(CHECKOUT_KEY_PREFIX + checkoutId, snapshotJson, CHECKOUT_TTL);

        log.info("CheckoutService.preview.done", "結帳預覽完成",
                "checkoutId", checkoutId);
        CheckoutPreviewData.Builder out = CheckoutPreviewData.newBuilder()
                .setCheckoutId(checkoutId)
                .addOrderGroups(group)
                .setTotalOriginalAmount(String.valueOf(orderSubtotal))
                .setTotalShippingFee(String.valueOf(finalShippingFee))
                .setTotalDiscount(String.valueOf(discountAmount))
                .setTotalActualAmount(String.valueOf(totalActualAmount));

        CheckoutInvoicePreview invoicePreview = resolveInvoicePreview(
                storeId, buyerId, checkoutId, orderSubtotal, finalShippingFee, discountAmount);
        if (invoicePreview != null) {
            out.setInvoice(invoicePreview);
        }
        return out.build();
    }

    /**
     * 收款模式（{@code collection_model}）fail-closed 守門：只有賣家收款（SELLER_MERCHANT）放行。
     *
     * <p>🔴 由政策快照的 {@code collection_model} 驅動，不讀 buyer request 的 {@code consolidated}
     * ——收款主體不能由呼叫端指定，推導在 {@link OrderPolicyResolver}。
     * 窮舉式判定、沒有 fall-through：null／空白／PROVIDER_COLLECTED／EXTERNAL_PREPAID／未知字串
     * 一律拒絕，因為對看不懂的收款模式做出「照賣家收」的結論是猜測，不是推導
     * （與撥款資格 PayoutEligibility 的 default⇒BLOCKED 同源）。
     * 賣家能不能接單由 requireSellerAcceptingOrders（can_accept_orders）擋下。
     */
    private void requireSupportedCollectionModel(OrderPolicySnapshot policy) {
        String model = policy.collectionModel();
        if (model == null || model.isBlank()) {
            throw new OrderBusinessException(GrpcResponseCode.FULFILLMENT_COMMERCE_MODEL_UNKNOWN);
        }
        if (CollectionModel.SELLER_MERCHANT.value().equals(model)) {
            return;
        }
        if (CollectionModel.PLATFORM_AGENT_MERCHANT.value().equals(model)) {
            throw new OrderBusinessException(GrpcResponseCode.PLATFORM_COLLECTION_NOT_CONFIGURED);
        }
        throw new OrderBusinessException(GrpcResponseCode.FULFILLMENT_COMMERCE_MODEL_UNKNOWN);
    }


    /**
     * 確認結帳（建單 + 回傳付款資訊）
     * 重新驗證所有資料，建單後消耗優惠券
     */
    public CheckoutConfirmData confirm(CheckoutConfirmRequest request) {
        log.info("CheckoutService.confirm.start", "確認結帳",
                "checkoutId", request.getCheckoutId());

        String snapshotJson = redisTemplate.opsForValue().getAndDelete(CHECKOUT_KEY_PREFIX + request.getCheckoutId());
        if (snapshotJson == null || snapshotJson.isEmpty()) {
            throw new CheckoutExpiredException();
        }

        OrderPolicySnapshot policySnapshot = extractPolicySnapshot(snapshotJson);

        long buyerId = request.getBuyerId();
        List<CheckoutItem> items = request.getItemsList();
        ResolveResult resolveResult = resolveItems(items);
        List<ResolvedItem> resolved = resolveResult.resolved();
        List<UnavailableItemInfo> unavailable = resolveResult.unavailable();

        if (resolved.isEmpty()) {
            throw new UnavailableItemsException(unavailable);
        }
        enforceNoSelfPurchase(buyerId, resolved);
        if (!unavailable.isEmpty()) {
            throw new UnavailableItemsException(unavailable);
        }

        long storeId = resolved.get(0).storeId();

        String shippingMethodCode = request.getShippingMethod();
        String deliveryTempCode = request.getDeliveryTemp();
        InputValidator.requireNotEmpty(shippingMethodCode, "配送方式");
        InputValidator.requireNotEmpty(deliveryTempCode, "配送溫層");
        ShippingMethod methodForAddrCheck = ShippingMethod.fromValue(shippingMethodCode);
        if (methodForAddrCheck == ShippingMethod.SEVEN_ELEVEN_STORE_PICKUP) {
            InputValidator.requireDisplayLengthBetween(request.getRecipientName(),
                    PayUniShippingConstraints.CVS_RECIPIENT_NAME_MIN,
                    PayUniShippingConstraints.CVS_RECIPIENT_NAME_MAX, "收件人姓名");
        } else {
            InputValidator.requireDisplayLengthBetween(request.getRecipientName(),
                    1, PayUniShippingConstraints.HOME_RECIPIENT_NAME_MAX, "收件人姓名");
        }
        InputValidator.requireMobile(request.getRecipientPhone(), "收件人電話");
        if (methodForAddrCheck != ShippingMethod.SEVEN_ELEVEN_STORE_PICKUP) {
            InputValidator.requireAddress(request.getShippingAddress(), "收件地址");
            InputValidator.requireDisplayLengthBetween(request.getShippingAddress(),
                    1, PayUniShippingConstraints.RECIPIENT_ADDRESS_MAX, "收件地址");
        }

        requireSellerOnboarded(storeId, shippingMethodCode);

        String senderPostalCode = getSenderPostalCode(storeId, shippingMethodCode, deliveryTempCode);
        SellerMeta sellerMeta = sellerGrpcClient.getSellerMeta(storeId);

        requireSellerAcceptingOrders(sellerMeta);

        ShippingMethod method = ShippingMethod.fromValue(shippingMethodCode);
        ShippingTemperature temp = ShippingTemperature.fromValue(deliveryTempCode);
        ShippingRoute route = resolveRoute(method, senderPostalCode, request.getReceiverZipCode());

        OrderPolicySnapshot expectedPolicy = orderPolicyResolver.resolve(method.value());
        if (!expectedPolicy.equals(policySnapshot)) {
            log.warn("CheckoutService.confirm.policyDrift",
                    "快照政策與本次配送方式不一致，結帳中止（snapshotFulfillmentMode="
                            + policySnapshot.fulfillmentMode()
                            + ", requestShippingMethod=" + method.value() + "）",
                    "checkoutId", request.getCheckoutId());
            throw new OrderBusinessException(GrpcResponseCode.FULFILLMENT_COMMERCE_MODEL_UNKNOWN);
        }

        ValidationResult validationResult = checkoutValidator.validate(resolved, route);
        if (!validationResult.valid()) {
            String msg = validationResult.errors().stream()
                    .map(ValidationError::errorMessage)
                    .collect(Collectors.joining("; "));
            throw new OrderBusinessException(GrpcResponseCode.CHECKOUT_VALIDATION_FAILED, msg);
        }

        UberOrderEligibilityGuard.Probe uberProbe = uberProbeOrNull(
                method, buyerId, storeId, temp.value(),
                request.getAddressId(), request.getUberQuoteId(), resolved);
        AvailableOption option = checkoutValidator.lookupSingleOption(
                resolved, method, temp, route, uberContext(method, uberProbe));
        if (option == null) {
            throw new OrderBusinessException(GrpcResponseCode.SHIPPING_METHOD_UNAVAILABLE);
        }

        long orderSubtotal = calcSubtotal(resolved);
        long discountAmount = 0;
        String couponTemplateId = request.getCouponTemplateId();
        if (couponTemplateId != null && !couponTemplateId.isEmpty()) {
            CouponValidateResult couponResult = couponGrpcClient.validateCoupon(
                    couponTemplateId, buyerId, storeId, orderSubtotal);
            discountAmount = InputValidator.requireNonNegativeAmount(couponResult.getDiscountAmount(), "折扣金額");
        }

        Map<UUID, Long> shareDiscountBySku = allocateShareDiscount(resolved, discountAmount);
        var finalFsr = checkoutValidator.checkFreeShipping(
                resolved, shippingMethodCode, deliveryTempCode, shareDiscountBySku);
        long finalShippingFee = finalFsr.eligible() ? 0L : option.rawFee();

        long totalActualAmount = Math.max(0, orderSubtotal + finalShippingFee - discountAmount);

        if (totalActualAmount > OrderAmountLimit.ORDER_MAX_AMOUNT) {
            log.warn("CheckoutService.confirm.amountExceedsLimit",
                    "訂單金額超出上限 amount=" + totalActualAmount
                            + " limit=" + OrderAmountLimit.ORDER_MAX_AMOUNT
                            + " buyer=" + buyerId);
            throw new OrderBusinessException(
                    GrpcResponseCode.ORDER_AMOUNT_EXCEEDS_LIMIT,
                    totalActualAmount,
                    OrderAmountLimit.ORDER_MAX_AMOUNT);
        }

        comparePriceSnapshot(snapshotJson, orderSubtotal, finalShippingFee, discountAmount,
                totalActualAmount, finalFsr.eligible());

        String shippingArea = ShippingAreaResolver.resolve(request.getReceiverZipCode()).value();

        String senderCity = "";
        String senderDistrict = "";
        GetSellerShippingConfigResponse configResp =
                shippingGrpcClient.getSellerShippingConfig(storeId);
        if (configResp.getCode() == 0) {
            for (SellerShippingConfigItem cfg : configResp.getConfigsList()) {
                if (cfg.getShippingMethod().equals(shippingMethodCode) && cfg.getAvailable()) {
                    senderCity = cfg.getSenderCity();
                    senderDistrict = cfg.getSenderDistrict();
                    break;
                }
            }
        }

        List<OrderService.CreateOrderItemParam> orderItems = resolved.stream()
                .map(r -> OrderService.CreateOrderItemParam.catalog(
                        r.skuId(), r.skuName(), r.quantity(), r.unitPrice(), r.listPrice(),
                        shareDiscountBySku.getOrDefault(r.skuId(), 0L),
                        r.imageObjectKey(),
                        r.productId(), r.productName()))
                .toList();

        requireSupportedCollectionModel(policySnapshot);
        String orderMerId = "";

        validateInvoiceInputOrThrow(request, buyerId, storeId,
                orderSubtotal, finalShippingFee, discountAmount, totalActualAmount);

        InvoiceIntakeResolver.Snapshot invoiceSnapshot =
                InvoiceIntakeResolver.resolve(request.getInvoice(), request.hasInvoice());

        OrderService.CreateOrderParams params = new OrderService.CreateOrderParams(
                buyerId, storeId, orderItems,
                orderSubtotal, totalActualAmount,
                request.getCheckoutId(),
                temp.value(), method.value(), finalShippingFee, option.feePayer(),
                request.getRecipientName(), request.getRecipientPhone(),
                request.getShippingAddress(), request.getShippingNote(),
                request.getOrderRemark(), request.getCvsStoreId(), request.getCvsStoreName(),
                shippingArea, senderCity, senderDistrict,
                validationResult.dimensionSum(),
                orderMerId,
                discountAmount,
                invoiceSnapshot,
                policySnapshot
        );

        String orderNo = orderService.createOrderFromCheckout(params);

        if (uberProbe != null) {
            acceptUberQuoteOrCancel(uberProbe, option.uberQuoteId(), orderNo, buyerId);
        }

        if (couponTemplateId != null && !couponTemplateId.isEmpty()) {
            try {
                String instanceId = couponGrpcClient.reserveCoupon(couponTemplateId, buyerId, orderNo, storeId, orderSubtotal);
                orderService.updateCouponInstanceId(orderNo, instanceId);
            } catch (Exception e) {
                log.error("CheckoutService.confirm.reserveCoupon.failed", "優惠券鎖定失敗，取消訂單", e);
                try {
                    orderService.cancelOrder(buyerId, orderNo);
                    orderService.releaseStockAfterCancel(orderNo);
                } catch (Exception compensateErr) {
                    log.error("CheckoutService.confirm.compensate.failed", "補償操作失敗", compensateErr);
                }
                couponReleaseService.releaseWithLateLockRecheck(orderNo);
                throw new OrderBusinessException(GrpcResponseCode.COUPON_LOCK_FAILED, e.getMessage());
            }
        }

        if (totalActualAmount == 0) {
            paymentEventHandler.completeZeroAmountOrder(orderNo);
        }

        log.info("CheckoutService.confirm.done", "確認結帳完成",
                "orderNo", orderNo);
        return CheckoutConfirmData.newBuilder()
                .setCheckoutId(request.getCheckoutId())
                .addOrderNos(orderNo)
                .setTotalPayAmount(String.valueOf(totalActualAmount))
                .build();
    }


    /**
     * 解析結帳商品（02 §5.5.2 + 20 §4.14）：
     * 走 BatchGetProductForCheckout 一次批次取所有 SKU（接受 inactive），
     * 對每個 input item 透過 {@link ProductPurchasabilityRules} 推導 reason：
     * <ul>
     *   <li>response data map 缺 sku key → unavailable[LOOKUP_FAILED]，**不**進 resolved
     *       （缺 weight / dimension / shipping_methods 無法跑 validator）</li>
     *   <li>!isActive → unavailable[DELISTED]，仍進 resolved（跨賣家 / 溫層判定仍需要）</li>
     *   <li>stockQty&lt;=0 → unavailable[OUT_OF_STOCK]，仍進 resolved</li>
     *   <li>stockQty&lt;quantity → unavailable[INSUFFICIENT_STOCK]，仍進 resolved</li>
     *   <li>else → 可購買，進 resolved</li>
     * </ul>
     */
    private ResolveResult resolveItems(List<CheckoutItem> checkoutItems) {
        List<String> skuIds = checkoutItems.stream().map(CheckoutItem::getSkuId).toList();
        BatchGetProductForCheckoutResponse batchResp = productGrpcClient.batchGetProductForCheckout(skuIds);
        Map<String, ProductForCheckoutInfo> infoMap = batchResp.getDataMap();

        List<ResolvedItem> resolved = new ArrayList<>();
        List<UnavailableItemInfo> unavailable = new ArrayList<>();

        for (CheckoutItem item : checkoutItems) {
            UUID skuId = ProtoConversionUtils.parseUuidOrNull(item.getSkuId());
            ProductForCheckoutInfo info = skuId == null ? null : infoMap.get(skuId.toString());

            Integer requestedQtyOrNull = parseIntPositive(item.getQuantity());
            if (requestedQtyOrNull == null) {
                unavailable.add(new UnavailableItemInfo(
                        item.getSkuId(),
                        info != null && info.getSkuName() != null ? info.getSkuName() : "",
                        CartItemUnavailableReason.LOOKUP_FAILED.value(),
                        item.getQuantity() != null ? item.getQuantity() : "",
                        ""));
                continue;
            }
            int requestedQty = requestedQtyOrNull;

            if (info == null) {
                ProductPurchasabilityRules.Purchasability lf = ProductPurchasabilityRules.lookupFailed();
                unavailable.add(new UnavailableItemInfo(
                        item.getSkuId(), "", lf.reason(),
                        String.valueOf(requestedQty), ""));
                continue;
            }

            long stockQty = info.getStockQty().isEmpty() ? 0L : Long.parseLong(info.getStockQty());
            ProductPurchasabilityRules.Purchasability p =
                    ProductPurchasabilityRules.evaluate(info.getIsActive(), stockQty, requestedQty);
            if (!p.purchasable()) {
                unavailable.add(new UnavailableItemInfo(
                        item.getSkuId(),
                        info.getSkuName() != null ? info.getSkuName() : "",
                        p.reason(),
                        String.valueOf(requestedQty),
                        switch (p.reason()) {
                            case "INSUFFICIENT_STOCK" -> String.valueOf(stockQty);
                            case "OUT_OF_STOCK" -> "0";
                            default -> "";
                        }));
                continue;
            }

            long weightGram = 0;
            String weightStr = info.getWeight();
            String unitStr = info.getUnit();
            if (weightStr != null && !weightStr.isEmpty() && !"0".equals(weightStr)
                    && unitStr != null && !unitStr.isEmpty()) {
                WeightUnit unit = WeightUnit.fromValue(unitStr.toUpperCase());
                weightGram = WeightConverter.convertToGrams(weightStr, unit);
            }

            List<SkuShippingOption> options = info.getShippingMethodsList().stream()
                    .map(sm -> new SkuShippingOption(
                            sm.getType(),
                            sm.getTemperature(),
                            sm.getFreeThreshold().isEmpty() ? 0L : Long.parseLong(sm.getFreeThreshold()),
                            sm.getFeePayer()))
                    .toList();

            long listPrice = InputValidator.requirePositiveAmount(info.getListPrice(), "商品原價");
            long unitPrice = info.getSalePrice().isEmpty()
                    ? listPrice
                    : InputValidator.requirePositiveAmount(info.getSalePrice(), "商品特價");

            resolved.add(new ResolvedItem(
                    skuId,
                    info.getSkuName() != null ? info.getSkuName() : "",
                    requestedQty,
                    unitPrice,
                    listPrice,
                    stockQty,
                    parseIntSafe(info.getLengthCm()),
                    parseIntSafe(info.getWidthCm()),
                    parseIntSafe(info.getHeightCm()),
                    weightGram,
                    Long.parseLong(info.getStoreId()),
                    ProtoConversionUtils.parseUuidRequired(info.getProductId(), "product_id"),
                    info.getProductName() != null ? info.getProductName() : "",
                    options,
                    info.getImageObjectKey() != null ? info.getImageObjectKey() : "",
                    info.getIsActive()
            ));
        }
        return new ResolveResult(resolved, unavailable);
    }

    /**
     * 推導運費路線（店到店統一 DOMESTIC）
     */
    private ShippingRoute resolveRoute(ShippingMethod method, String sellerZipCode, String receiverZipCode) {
        if (method == ShippingMethod.SEVEN_ELEVEN_STORE_PICKUP) {
            return ShippingRoute.DOMESTIC;
        }
        return ShippingAreaResolver.resolveRoute(sellerZipCode, receiverZipCode);
    }

    /**
     * 從 Shipping Service 取得賣家對應配送方式的寄件郵遞區號（純取地址，不含 enabled 檢查）。
     *
     * <p>03 §4.3：本方法**只負責取 postal code**，enabled-check 已拆出 {@link #requireSellerOnboarded}。
     * caller（shippingFee / preview / confirm）必須先 call {@code requireSellerOnboarded} 再 call 本方法。
     *
     * <p>超商固定回傳空字串（resolveRoute 會走 DOMESTIC，超商不需要寄件郵遞區號）。
     */
    private String getSenderPostalCode(long storeId, String shippingMethodCode, String deliveryTempCode) {
        if (ShippingMethod.SEVEN_ELEVEN_STORE_PICKUP.value().equals(shippingMethodCode)) {
            return "";
        }

        GetSellerShippingConfigResponse configResp =
                shippingGrpcClient.getSellerShippingConfig(storeId);

        for (SellerShippingConfigItem config : configResp.getConfigsList()) {
            if (config.getShippingMethod().equals(shippingMethodCode)) {
                String postalCode = config.getSenderPostalCode();
                if (postalCode == null || postalCode.isEmpty()) {
                    throw new OrderBusinessException(GrpcResponseCode.SELLER_SHIPPING_INCOMPLETE);
                }
                return postalCode;
            }
        }

        throw new OrderBusinessException(GrpcResponseCode.SELLER_NOT_ONBOARDED);
    }

    /**
     * 取得賣家當下已啟用的配送方式 key set。
     * 對應 03 §4.3 fail-close：RPC 失敗 / code!=0 → 拋 SERVICE_UNAVAILABLE（不 fail-open）。
     *
     * <p>key 格式：{@code "{shippingMethod}:{goodsType}"}（goodsType 為空時用空字串）。
     */
    private Set<String> resolveSellerAvailableMethods(long storeId) {
        GetSellerShippingConfigResponse configResp;
        try {
            configResp = shippingGrpcClient.getSellerShippingConfig(storeId);
        } catch (Exception e) {
            log.warn("CheckoutService.resolveSellerAvailableMethods.failed",
                    "shipping-service getSellerShippingConfig 失敗，fail-close: " + e.getMessage(),
                    "storeId", String.valueOf(storeId));
            throw new OrderBusinessException(GrpcResponseCode.SERVICE_UNAVAILABLE);
        }
        if (configResp.getCode() != 0) {
            log.warn("CheckoutService.resolveSellerAvailableMethods.nonZeroCode",
                    "shipping-service getSellerShippingConfig 回應非零碼，fail-close: code="
                            + configResp.getCode() + " msg=" + configResp.getMessage(),
                    "storeId", String.valueOf(storeId));
            throw new OrderBusinessException(GrpcResponseCode.SERVICE_UNAVAILABLE);
        }
        Set<String> available = new HashSet<>();
        for (SellerShippingConfigItem cfg : configResp.getConfigsList()) {
            if (cfg.getAvailable()) {
                available.add(SellerShippingFilter.toMethodKey(cfg.getShippingMethod(), cfg.getGoodsType()));
            }
        }
        return available;
    }

    /**
     * 03 §4.3 共用入口：驗證賣家對指定 {@code shippingMethodCode} 已開通且 enabled=true。
     *
     * <p>**無視 shippingMethodCode 是否為超商** — 修復 7-11 短路 return 跳過 enabled-check 的 bug
     * （見 issue #1114 C3 額外發現：7-11 disabled 但 preview 仍 200 success）。
     *
     * <p>caller：shippingFee / preview / confirm 進入 {@link #getSenderPostalCode} 前必須先 call。
     *
     * @throws OrderBusinessException {@code SELLER_NOT_ONBOARDED} 若該方式未啟用 / 無對應 config entry
     */
    private void requireSellerOnboarded(long storeId, String shippingMethodCode) {
        Set<String> available = resolveSellerAvailableMethods(storeId);
        boolean ok = available.stream()
                .anyMatch(k -> k.startsWith(shippingMethodCode + ":"));
        if (!ok) {
            log.warn("CheckoutService.requireSellerOnboarded.notOnboarded",
                    "賣家未開通指定配送方式 storeId=" + storeId + " shippingMethodCode=" + shippingMethodCode);
            throw new OrderBusinessException(GrpcResponseCode.SELLER_NOT_ONBOARDED);
        }
    }

    /**
     * 賣家停權收斂點 backstop（03 §5.1a.1 / 46 §6.2 §11，首次接線 A5）。
     * {@code can_accept_orders=false}（店家不可公開曝光：停權、等待刪除、已關閉、KYC 或 PayUni 未核准、缺店名／logo／所在地）→
     * 整組商品擋下，回業務碼 9241。
     * fail-closed：SellerMeta 查詢本身走既有硬依賴 {@link SellerGrpcClient#getSellerMeta}
     * （查無資料拋 NOT_FOUND、gRPC 失敗拋 StatusRuntimeException 由上層 catch-all 轉 SERVICE_UNAVAILABLE），
     * 兩者皆不放行，與 Gateway 能力層 guard 的 fail-open 形成非對稱互補。
     */
    private void requireSellerAcceptingOrders(SellerMeta sellerMeta) {
        if (!sellerMeta.getCanAcceptOrders()) {
            throw new OrderBusinessException(GrpcResponseCode.SELLER_SUSPENDED_PRODUCT_UNAVAILABLE);
        }
    }


    /** 直接驗證業務規則本身（can_accept_orders=false → 拋 9241）。 */
    void requireSellerAcceptingOrdersForTest(SellerMeta sellerMeta) {
        requireSellerAcceptingOrders(sellerMeta);
    }

    /** 鏡射 validate() 內實際呼叫序（sellerGrpcClient.getSellerMeta → 規則檢查），
     * 用於驗證 SellerMeta 查詢失敗時例外會原樣往外拋（fail-closed，未被靜默吞掉）。 */
    void requireSellerAcceptingOrdersByStoreIdForTest(long storeId) {
        requireSellerAcceptingOrders(sellerGrpcClient.getSellerMeta(storeId));
    }

    private long calcSubtotal(List<ResolvedItem> items) {
        return items.stream().mapToLong(i -> i.unitPrice() * i.quantity()).sum();
    }

    /** 防護式解析正整數；null / 空 / 非數字 / ≤ 0 一律回 null（呼叫端可視為 INVALID_ITEM）。 */
    private static Integer parseIntPositive(String value) {
        if (value == null || value.isEmpty()) return null;
        try {
            int n = Integer.parseInt(value);
            return n > 0 ? n : null;
        } catch (NumberFormatException e) {
            return null;
        }
    }

    private int parseIntSafe(String value) {
        if (value == null || value.isEmpty()) return 0;
        try {
            return (int) Math.round(Double.parseDouble(value));
        } catch (NumberFormatException e) {
            return 0;
        }
    }

    /**
     * 從 Redis snapshot JSON 取回 preview 階段凍結的政策五欄——<b>fail-closed</b>。
     *
     * <p>🔴 <b>為什麼缺欄位要拋而不是退回預設值</b>：這五欄是撥款資格與開票法源的輸入
     * （{@code collection_model} 決定錢經過誰的手、{@code commerce_model} 決定誰開票）。
     * 「讀不到就用 DB DEFAULT」＝把「我不知道」記成「賣家直營、賣家收款」，
     * 而那個結論看起來完全正常：訂單成功、付款成功、帳本平衡。
     * 讓買家重新 preview 的成本是一次往返；猜錯的成本是錢與發票對不上而沒有人會發現。
     *
     * <p>⚠️ 舊版 snapshot（本次改動部署前寫入、TTL 內仍在 Redis 的那些）沒有 policy 欄
     * ⇒ 會走到這裡拋 9504。那是<b>刻意的</b>：in-flight 的結帳重新 preview 即可，
     * 而放行它們就等於在部署當天留一個「合法產生錯誤政策」的窗口。
     */
    private OrderPolicySnapshot extractPolicySnapshot(String snapshotJson) {
        if (snapshotJson == null || snapshotJson.isEmpty() || "1".equals(snapshotJson)) {
            throw new OrderBusinessException(GrpcResponseCode.FULFILLMENT_COMMERCE_MODEL_UNKNOWN);
        }
        JsonNode policy;
        try {
            policy = objectMapper.readTree(snapshotJson).path("policy");
        } catch (Exception e) {
            log.warn("CheckoutService.extractPolicySnapshot.failed",
                    "政策快照解析失敗，結帳中止", "error", e.getMessage());
            throw new OrderBusinessException(GrpcResponseCode.FULFILLMENT_COMMERCE_MODEL_UNKNOWN);
        }
        if (policy.isMissingNode() || !policy.isObject()) {
            log.warn("CheckoutService.extractPolicySnapshot.missing",
                    "快照缺少政策欄位，結帳中止（請重新結帳）");
            throw new OrderBusinessException(GrpcResponseCode.FULFILLMENT_COMMERCE_MODEL_UNKNOWN);
        }
        String commerceModel = requiredText(policy, "commerceModel");
        String collectionModel = requiredText(policy, "collectionModel");
        String fulfillmentMode = requiredText(policy, "fulfillmentMode");
        String invoiceModel = requiredText(policy, "invoiceModel");
        String policyVersion = requiredText(policy, "policyVersion");
        return new OrderPolicySnapshot(commerceModel, collectionModel, fulfillmentMode,
                invoiceModel, policyVersion);
    }

    private String requiredText(JsonNode node, String field) {
        String v = node.path(field).asText("");
        if (v.isBlank()) {
            log.warn("CheckoutService.extractPolicySnapshot.blankField",
                    "政策快照欄位為空，結帳中止", "field", field);
            throw new OrderBusinessException(GrpcResponseCode.FULFILLMENT_COMMERCE_MODEL_UNKNOWN);
        }
        return v;
    }

    private static String emptyToNull(String v) {
        return (v == null || v.isBlank()) ? null : v;
    }

    /**
     * 組裝金額快照 JSON（Preview 存入 Redis）。20 設計書 §4.11 + §4.13.4。
     */
    String buildPriceSnapshot(long subtotal, long shippingFee, long discount, long totalAmount,
                                       boolean freeShippingEligible, boolean tentativeFreeShippingEligible,
                                       OrderPolicySnapshot policy) {
        try {
            Map<String, Object> snapshot = new LinkedHashMap<>();
            snapshot.put("subtotal", subtotal);
            snapshot.put("shippingFee", shippingFee);
            snapshot.put("discount", discount);
            snapshot.put("totalAmount", totalAmount);
            snapshot.put("freeShippingEligible", freeShippingEligible);
            snapshot.put("tentativeFreeShippingEligible", tentativeFreeShippingEligible);
            Map<String, Object> policyMap = new LinkedHashMap<>();
            policyMap.put("commerceModel", policy.commerceModel());
            policyMap.put("collectionModel", policy.collectionModel());
            policyMap.put("fulfillmentMode", policy.fulfillmentMode());
            policyMap.put("invoiceModel", policy.invoiceModel());
            policyMap.put("policyVersion", policy.policyVersion());
            snapshot.put("policy", policyMap);
            return objectMapper.writeValueAsString(snapshot);
        } catch (Exception e) {
            log.error("CheckoutService.buildPriceSnapshot.failed", "金額快照序列化失敗", e);
            throw new OrderBusinessException(GrpcResponseCode.FULFILLMENT_COMMERCE_MODEL_UNKNOWN);
        }
    }

    /**
     * 折扣分攤（20 設計書 §4.10）：
     * item.share_discount = round(discount × (item.unitPrice × qty / total_subtotal))
     * 尾差歸入最後一筆，避免分攤總和 ≠ discount。
     */
    /**
     * §6.2.4 order 的第 ③ 項：建單前把買家的發票輸入送 invoice-service 驗。
     *
     * <p><b>回 code ≠ 0 就擋下建單</b>——那些碼是買家當場改得掉的輸入錯誤
     * （載具格式、三者互斥）或賣家側的接單上限（9326）。等建完單才在 DB CHECK
     * 上炸，買家看到的是 500 而不是一句能照著改的話。
     *
     * <p>🔴 <b>invoice-service 不可用時放行</b>（client 回 null）。理由見
     * {@link InvoiceGrpcClient#validateOrderInvoiceInput}：發票正確性的保護在
     * {@code order.paid} 那一側，這裡 fail-closed 會讓 invoice-service 變成整條
     * 金流的硬依賴。
     *
     * <p>⚠️ <b>不用它回傳的 snapshot</b>：{@code order_invoice_snapshots} 的內容由
     * {@link InvoiceIntakeResolver}（純函式）產生，那是 §5.4.3 的既有設計。這裡只取
     * 「過或不過」。兩份 snapshot 各自落庫會立刻漂移，而漂移的那一份沒有任何東西在對。
     */
    private void validateInvoiceInputOrThrow(CheckoutConfirmRequest request,
                                             long buyerId, long storeId,
                                             long orderSubtotal, long shippingFee,
                                             long discountAmount, long invoiceAmount) {
        ValidateOrderInvoiceInputInternalRequest.Builder req =
                ValidateOrderInvoiceInputInternalRequest.newBuilder()
                        .setStoreId(String.valueOf(storeId))
                        .setBuyerId(String.valueOf(buyerId))
                        .setOrderNo("")
                        .setAmount(InvoiceAmountBreakdown.newBuilder()
                                .setProductAmount(String.valueOf(orderSubtotal))
                                .setShippingAmount(String.valueOf(shippingFee))
                                .setDiscountAmount(String.valueOf(discountAmount))
                                .setInvoiceAmount(String.valueOf(invoiceAmount))
                                .build());
        if (request.hasInvoice()) {
            req.setInvoice(toInvoiceInputSpec(request.getInvoice()));
        }

        ValidateOrderInvoiceInputInternalResponse resp =
                invoiceGrpcClient.validateOrderInvoiceInput(req.build());
        if (resp == null) {
            return;
        }
        if (resp.getCode() == 0) {
            return;
        }
        GrpcResponseCode code = GrpcResponseCode.fromCode(resp.getCode());
        if (code == null) {
            throw new OrderBusinessException(GrpcResponseCode.SERVICE_UNAVAILABLE);
        }
        throw new OrderBusinessException(code);
    }

    /**
     * order 的 {@code CheckoutInvoiceRequest} → invoice 的 {@code InvoiceInputSpec}。
     *
     * <p>兩邊是<b>不同的 message</b>，這是刻意的：order 的契約不承載發票服務的內部欄位
     * （{@code order.proto} 的註解逐字列了「刻意不放進 order 契約」的那幾欄）。
     *
     * <p>🔴 <b>order 側沒有 {@code carrier_type}／{@code value} 是刻意的</b>，不是缺口。
     * 58 號設計書 §5821 第 4 項已完整推導：{@code order_invoice_snapshots} 是<b>零 PII 表</b>，
     * 只有 {@code buyer_invoice_preference_id} 一個指標欄，而 {@code chk_ois_mode} 的四值裡
     * 沒有一個代表「這一單自己填的載具」——落 {@code USE_BUYER_DEFAULT} 會把買家剛輸入的
     * 載具靜默吞掉，落 {@code EXPLICIT_PREFERENCE} ＋ 空 id 則因 {@code chk_ois_pref_pair}
     * 是雙向等式而直接 500。⇒ 只剩 fail-closed 一條路。
     *
     * <p>所以手機條碼／自然人憑證走的是<b>先存成 profile → 送 {@code profile_id}</b>
     * （58A §B02 的 Sheet 底下那個「儲存為常用發票資料」checkbox 就是這條路）；
     * 純手動輸入由 invoice-service 回 9305 ＋ 專屬訊息擋下。
     * {@code TAX_ID}／{@code DONATION} 則完全支援，因為 {@code NONE}／{@code DONATION}
     * 兩個 mode 就能完整表達 —— 這正是本 message 只有這幾個欄位的原因。
     *
     * <p>要正式支援手動載具是一個<b>業務決定</b>（會打破零 PII，或讓 confirm 多一次跨服務
     * 寫入），已記在 58 §5821 第 4 項的「需求方要決定的事」，不要在這裡自行擴欄。
     *
     * <p>捐贈碼在 order 側是獨立欄位、在 invoice 側是
     * {@code carrier_type = DONATION} ＋ {@code value}，所以要在這裡折一次。
     * 值取自 CodeMeta 生成的 enum，不寫字面值。
     */
    private static InvoiceInputSpec toInvoiceInputSpec(CheckoutInvoiceRequest in) {
        InvoiceInputSpec.Builder out = InvoiceInputSpec.newBuilder()
                .setProfileId(in.getProfileId())
                .setBuyerTaxId(in.getBuyerTaxId())
                .setBuyerName(in.getBuyerTitle())
                .setEmail(in.getNotifyEmail());
        if (!in.getDonationCode().isBlank()) {
            out.setCarrierType(InvoiceCarrierType.DONATION.value())
               .setValue(in.getDonationCode());
        } else if (!in.getBuyerTaxId().isBlank()) {
            out.setCarrierType(InvoiceCarrierType.TAX_ID.value());
        }
        return out.build();
    }

    /**
     * §6.2.4 order ②：向 invoice-service 解析本單的發票模式，折成 order 的對外契約。
     *
     * <p>🔴 <b>snapshot_version 由 order 覆蓋</b>：I-50 明訂它的唯一寫入者是
     * order-service，而 invoice 那支 RPC 回<b>空字串</b>（它沒有可回聲的值，設計書
     * §5821 第 1 項逐字記載）。不覆蓋的話前端拿到空字串，而那不會有任何錯誤訊號。
     * Phase 1 的快照版本恆為 1（confirm 建立第一版），所以這裡直接給 "1"。
     *
     * <p>回 null ⇒ 呼叫端不設該欄（preview 不擋結帳，見 client 的 javadoc）。
     */
    private CheckoutInvoicePreview resolveInvoicePreview(long storeId, long buyerId, String checkoutId,
                                                         long productAmount, long shippingAmount,
                                                         long discountAmount) {
        ResolveOrderInvoiceModeInternalResponse resp = invoiceGrpcClient.resolveOrderInvoiceMode(
                ResolveOrderInvoiceModeInternalRequest.newBuilder()
                        .setStoreId(String.valueOf(storeId))
                        .setBuyerId(String.valueOf(buyerId))
                        .setProductAmount(String.valueOf(productAmount))
                        .setShippingAmount(String.valueOf(shippingAmount))
                        .setDiscountAmount(String.valueOf(discountAmount))
                        .setCheckoutId(checkoutId)
                        .build());
        if (resp == null || resp.getCode() != 0 || !resp.hasData()) {
            return null;
        }
        ResolveOrderInvoiceModeInternalData d = resp.getData();
        CheckoutInvoicePreview.Builder b = CheckoutInvoicePreview.newBuilder()
                .setInvoiceMode(d.getInvoiceMode())
                .setSnapshotVersion("1")
                .setPreviewExpiresAt(d.getPreviewExpiresAt());
        if (d.hasAmount()) {
            b.setAmount(CheckoutInvoiceAmount.newBuilder()
                    .setProductAmount(d.getAmount().getProductAmount())
                    .setShippingAmount(d.getAmount().getShippingAmount())
                    .setDiscountAmount(d.getAmount().getDiscountAmount())
                    .setInvoiceAmount(d.getAmount().getInvoiceAmount())
                    .setTaxCategory(d.getAmount().getTaxCategory()));
        }
        if (d.hasDisclosure()) {
            b.setDisclosure(CheckoutInvoiceDisclosure.newBuilder()
                    .setRequired(d.getDisclosure().getRequired())
                    .setTermsVersion(d.getDisclosure().getTermsVersion())
                    .setTitle(d.getDisclosure().getTitle())
                    .setBody(d.getDisclosure().getBody()));
        }
        if (d.hasDefaultProfile() && !d.getDefaultProfile().getProfileId().isBlank()) {
            b.setDefaultProfile(CheckoutInvoiceProfile.newBuilder()
                    .setProfileId(d.getDefaultProfile().getProfileId())
                    .setCarrierType(d.getDefaultProfile().getCarrierType())
                    .setDisplayValue(d.getDefaultProfile().getDisplayValue())
                    .setBuyerTaxId(d.getDefaultProfile().getBuyerTaxId())
                    .setBuyerName(d.getDefaultProfile().getBuyerName())
                    .setIsDefault(d.getDefaultProfile().getIsDefault())
                    .setLabel(d.getDefaultProfile().getLabel()));
        }
        if (d.hasIntakeLimit()) {
            b.setIntakeLimit(CheckoutInvoiceIntakeLimit.newBuilder()
                    .setOrderCountUsed(d.getIntakeLimit().getOrderCountUsed())
                    .setOrderCountLimit(d.getIntakeLimit().getOrderCountLimit())
                    .setAmountUsed(d.getIntakeLimit().getAmountUsed())
                    .setAmountLimit(d.getIntakeLimit().getAmountLimit())
                    .setBlocked(d.getIntakeLimit().getBlocked()));
        }
        return b.build();
    }

    private Map<UUID, Long> allocateShareDiscount(List<ResolvedItem> items, long discountAmount) {
        Map<UUID, Long> result = new LinkedHashMap<>();
        if (discountAmount <= 0 || items.isEmpty()) {
            for (ResolvedItem r : items) result.put(r.skuId(), 0L);
            return result;
        }
        long totalSubtotal = items.stream().mapToLong(i -> i.unitPrice() * i.quantity()).sum();
        if (totalSubtotal <= 0) {
            for (ResolvedItem r : items) result.put(r.skuId(), 0L);
            return result;
        }
        long allocated = 0;
        for (int i = 0; i < items.size(); i++) {
            ResolvedItem r = items.get(i);
            long share;
            if (i == items.size() - 1) {
                share = discountAmount - allocated;
            } else {
                long itemSubtotal = r.unitPrice() * r.quantity();
                share = Math.round((double) discountAmount * itemSubtotal / totalSubtotal);
                allocated += share;
            }
            result.put(r.skuId(), Math.max(0, share));
        }
        return result;
    }

    /**
     * 比對 Preview 金額快照與 Confirm 重算結果
     *
     * 快照為 "1" 時（舊格式或序列化失敗）跳過比對，保持向下相容。
     */
    private void comparePriceSnapshot(String snapshotJson, long newSubtotal, long newShippingFee,
                                       long newDiscount, long newTotalAmount,
                                       boolean newFreeShippingEligible) {
        if ("1".equals(snapshotJson)) {
            return;
        }

        JsonNode snapshot;
        try {
            snapshot = objectMapper.readTree(snapshotJson);
        } catch (Exception e) {
            log.warn("CheckoutService.comparePriceSnapshot.parseFailed", "金額快照解析失敗，跳過比對: " + e.getMessage());
            return;
        }

        long oldSubtotal = snapshot.path("subtotal").asLong(0);
        long oldShippingFee = snapshot.path("shippingFee").asLong(0);
        long oldDiscount = snapshot.path("discount").asLong(0);
        long oldTotalAmount = snapshot.path("totalAmount").asLong(0);
        boolean oldFreeShippingEligible = snapshot.path("freeShippingEligible").asBoolean(false);

        List<String> reasons = new ArrayList<>();
        List<String> reasonLabels = new ArrayList<>();

        if (oldSubtotal != newSubtotal) {
            reasons.add("PRODUCT_PRICE");
            reasonLabels.add("商品價格");
        }
        if (oldShippingFee != newShippingFee) {
            reasons.add("SHIPPING_FEE");
            reasonLabels.add("運費");
        }
        if (oldDiscount != newDiscount) {
            if (newDiscount == 0 && oldDiscount > 0) {
                reasons.add("COUPON_INVALID");
                reasonLabels.add("優惠券已失效");
            } else {
                reasons.add("COUPON_DISCOUNT");
                reasonLabels.add("優惠券折扣");
            }
        }
        if (oldFreeShippingEligible != newFreeShippingEligible) {
            reasons.add("FREE_SHIPPING_FLIPPED");
            reasonLabels.add("免運資格");
        }

        if (reasons.isEmpty()) {
            if (oldTotalAmount != newTotalAmount) {
                reasons.add("AMOUNT_CALCULATION");
                reasonLabels.add("金額計算");
            }
        }

        if (!reasons.isEmpty()) {
            String message;
            if (reasons.size() == 1) {
                message = switch (reasons.get(0)) {
                    case "PRODUCT_PRICE" -> "商品價格已變動，請重新確認訂單";
                    case "SHIPPING_FEE" -> "運費已調整，請重新確認訂單";
                    case "COUPON_INVALID" -> "優惠券已失效，請重新確認訂單";
                    case "COUPON_DISCOUNT" -> "優惠券折扣金額已變更，請重新確認訂單";
                    default -> "訂單金額計算異常，請重新確認訂單";
                };
            } else {
                message = "訂單金額已變動（" + String.join("、", reasonLabels) + "），請重新確認訂單";
            }

            log.warn("CheckoutService.comparePriceSnapshot.mismatch",
                    "金額變動: " + message
                            + ", old=" + oldTotalAmount + ", new=" + newTotalAmount
                            + ", reasons=" + reasons);

            throw new CheckoutPriceChangedException(
                    message, oldTotalAmount, newTotalAmount,
                    newSubtotal, newShippingFee, newDiscount, reasons);
        }
    }
}
