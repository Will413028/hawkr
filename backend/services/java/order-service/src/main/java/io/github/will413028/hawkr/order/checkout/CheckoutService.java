package io.github.will413028.hawkr.order.checkout;

import io.github.will413028.hawkr.common.utils.code.CollectionModel;
import io.github.will413028.hawkr.common.utils.code.GrpcResponseCode;
import io.github.will413028.hawkr.order.exception.OrderBusinessException;
import io.github.will413028.hawkr.common.obs.log.ObsLogger;
import io.github.will413028.hawkr.common.utils.code.ShippingTemperature;
import io.github.will413028.hawkr.common.utils.code.CartItemUnavailableReason;
import io.github.will413028.hawkr.common.utils.code.helper.PaymentMethodLimitRules;
import io.github.will413028.hawkr.common.utils.code.helper.ProductPurchasabilityRules;
import io.github.will413028.hawkr.common.utils.validation.InputValidator;
import io.github.will413028.hawkr.coupon.grpc.CouponValidateResult;
import io.grpc.StatusRuntimeException;
import io.github.will413028.hawkr.order.checkout.CheckoutValidator.*;
import io.github.will413028.hawkr.order.checkout.client.CheckoutCouponGrpcClient;
import io.github.will413028.hawkr.order.service.CouponReleaseService;
import io.github.will413028.hawkr.common.grpc.PrincipalResolution;
import io.github.will413028.hawkr.common.grpc.UnresolvedPrincipalException;
import io.github.will413028.hawkr.order.checkout.client.SellerGrpcClient;
import io.github.will413028.hawkr.order.grpc.*;
import io.github.will413028.hawkr.order.grpc.converter.PaymentMethodAvailabilityConverter;
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
import io.github.will413028.hawkr.order.exception.CheckoutExpiredException;
import io.github.will413028.hawkr.order.exception.CheckoutPriceChangedException;
import io.github.will413028.hawkr.order.exception.UnavailableItemsException;
import io.github.will413028.hawkr.order.constant.OrderConstants;
import io.github.will413028.hawkr.order.service.OrderService;
import io.github.will413028.hawkr.order.util.ColumnWidth;
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
 * 提供 3 支結帳 API：validate / preview / confirm
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
    private final CheckoutCouponGrpcClient couponGrpcClient;
    private final CouponReleaseService couponReleaseService;
    private final InvoiceGrpcClient invoiceGrpcClient;
    private final CheckoutValidator checkoutValidator;
    private final OrderService orderService;
    private final OrderPolicyResolver orderPolicyResolver;
    private final StringRedisTemplate redisTemplate;

    /**
     * 代理商 PayUni MerID（多賣家和單代收）。consolidated=true 時取此值。
     * 空字串時不允許 consolidated=true（preview 階段拒絕）。
     * 詳見 20 設計書 §4.13 和單與 paymentMerId 決策。
     */
    @Value("${payuni.agent.mer-id:}")
    private String agentMerId;

    public CheckoutService(ProductGrpcClient productGrpcClient,
                           SellerGrpcClient sellerGrpcClient,
                           BuyerGrpcClient buyerGrpcClient,
                           CheckoutCouponGrpcClient couponGrpcClient,
                           InvoiceGrpcClient invoiceGrpcClient,
                           CheckoutValidator checkoutValidator,
                           OrderService orderService,
                           OrderPolicyResolver orderPolicyResolver,
                           StringRedisTemplate redisTemplate,
                           CouponReleaseService couponReleaseService) {
        this.productGrpcClient = productGrpcClient;
        this.sellerGrpcClient = sellerGrpcClient;
        this.buyerGrpcClient = buyerGrpcClient;
        this.couponGrpcClient = couponGrpcClient;
        this.invoiceGrpcClient = invoiceGrpcClient;
        this.checkoutValidator = checkoutValidator;
        this.orderService = orderService;
        this.orderPolicyResolver = orderPolicyResolver;
        this.redisTemplate = redisTemplate;
        this.couponReleaseService = couponReleaseService;
    }

    // ==================== 自買防護 ====================

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

    // package-visible test seam — logic lives here so tests don't need a full
    // validate()/preview()/confirm() fixture just to exercise the guard.
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
        // fail-closed（issue #2789 階段 B1/B2）：查無在架賣家／買家的 id 會被 server 原樣回吐，
        // 舊寫法 getOrDefault(id, id) 拿那個未驗證的值來比對 ⇒「無法確認」被當成
        // 「不是自己」⇒ 自買防護靜默失效。未解析代表該身分已不在架，這筆本來就不該成單。
        // 兩軸必須同批 fail-closed：只修一半就是「兩個各自正確的改動組合起來互相抵銷」。
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

    // ==================== 即時驗證 ====================

    /**
     * 即時驗證（購物車勾選時呼叫，輕量級）
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

        // 全部 LOOKUP_FAILED → resolved 空，僅回 unavailable_items（20 §4.14 / §4.15）
        if (resolved.isEmpty()) {
            return CheckoutValidateData.newBuilder()
                    .setValid(false)
                    .addAllUnavailableItems(toProtoUnavailable(unavailable))
                    .build();
        }

        // 自買防護（short-circuit，§4.1 步驟 3）
        enforceNoSelfPurchase(buyerId, resolved);

        ValidationResult result = checkoutValidator.validate(resolved);

        long storeId = resolved.get(0).storeId();

        // 03 §5.1a.1 / 46 §6.2：賣家停權收斂點 backstop（首次接線 A5，fail-closed）。
        // 此為結帳鏈最前的驗證收斂點，主防線放這裡（Confirm 為第二道，見 confirm()）。
        requireSellerAcceptingOrders(sellerGrpcClient.getSellerMeta(storeId));

        // 20 §4.14：valid = unavailable.empty && errors.empty
        boolean valid = result.valid() && unavailable.isEmpty();
        CheckoutValidateData.Builder builder = CheckoutValidateData.newBuilder()
                .setValid(valid)
                .addAllDeliveryTempIntersection(result.deliveryTempIntersection())
                .addAllUnavailableItems(toProtoUnavailable(unavailable));

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

    // ==================== 結帳預覽 ====================

    /**
     * 結帳預覽（完整驗證 + 精算總額 + 優惠券驗證，不建單）
     * 生成 checkout_id 供 Confirm 使用
     */
    public CheckoutPreviewData preview(List<CheckoutItem> items,
                                        String deliveryTempCode,
                                        String couponTemplateId, long buyerId,
                                        boolean consolidated) {
        log.info("CheckoutService.preview.start", "結帳預覽",
                "consolidated", String.valueOf(consolidated));
        ResolveResult resolveResult = resolveItems(items);
        List<ResolvedItem> resolved = resolveResult.resolved();
        List<UnavailableItemInfo> unavailable = resolveResult.unavailable();

        // 自買防護（§4.3 步驟 2.5）
        // 全部 LOOKUP_FAILED → resolved 空，自買防護不適用，直接拋 unavailable
        if (resolved.isEmpty()) {
            throw new UnavailableItemsException(unavailable);
        }
        enforceNoSelfPurchase(buyerId, resolved);

        // §4.3 步驟 2.6：可購買性收集非空 → 不繼續，由 OrderGrpcService 包成 code=4001 + unavailable_items
        if (!unavailable.isEmpty()) {
            throw new UnavailableItemsException(unavailable);
        }

        long storeId = resolved.get(0).storeId();

        // 賣家元資料（僅用於 sellerName）
        SellerMeta sellerMeta = sellerGrpcClient.getSellerMeta(storeId);

        // 03 §5.1a.1 / 46 §6.2：賣家停權收斂點 backstop（與 validate() 對稱，避免 preview 通過但 confirm 才擋）
        requireSellerAcceptingOrders(sellerMeta);

        // 配送溫層（防護空值）
        if (deliveryTempCode == null || deliveryTempCode.isEmpty()) {
            throw new IllegalArgumentException("配送溫層不可為空");
        }
        ShippingTemperature temp = ShippingTemperature.fromValue(deliveryTempCode);

        // 完整驗證
        ValidationResult validationResult = checkoutValidator.validate(resolved);
        if (!validationResult.valid()) {
            String msg = validationResult.errors().stream()
                    .map(ValidationError::errorMessage)
                    .collect(Collectors.joining("; "));
            throw new OrderBusinessException(GrpcResponseCode.CHECKOUT_VALIDATION_FAILED, msg);
        }

        // 優惠券驗證（不消耗）— couponTemplateId 為使用者選擇的模板 ID
        long orderSubtotal = calcSubtotal(resolved);
        long discountAmount = 0;
        if (couponTemplateId != null && !couponTemplateId.isEmpty()) {
            CouponValidateResult couponResult = couponGrpcClient.validateCoupon(
                    couponTemplateId, buyerId, storeId, orderSubtotal);
            discountAmount = InputValidator.requireNonNegativeAmount(couponResult.getDiscountAmount(), "折扣金額");
        }

        // 生成結帳批次 ID，存入 Redis（TTL 30 分鐘，含金額快照供 Confirm 比對）
        String checkoutId = UUID.randomUUID().toString();

        // 折扣分攤：依 unit_price × qty 比例分攤 discountAmount，尾差歸最後一筆（20 設計書 §4.10）
        Map<Long, Long> shareDiscountBySku = allocateShareDiscount(resolved, discountAmount);

        // 賣家自寄，B8 出貨設計落地前運費固定 0，沒有免運判定
        long shippingFee = OrderConstants.CHECKOUT_SHIPPING_FEE;

        // 商品明細（每品項回傳 list_price / sale_price / share_discount / actual_pay_amount）
        List<CheckoutItemPreview> itemPreviews = resolved.stream()
                .map(r -> {
                    long subtotal = r.unitPrice() * r.quantity();
                    long itemDiscount = shareDiscountBySku.getOrDefault(r.skuId(), 0L);
                    String salePriceStr = r.unitPrice() < r.listPrice()
                            ? String.valueOf(r.unitPrice()) : "";
                    return CheckoutItemPreview.newBuilder()
                            .setSkuId(String.valueOf(r.skuId()))
                            .setProductId(r.productId())
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
                // 賣家頁 canonical 網址走 store_code；值已在上面同一次 getSellerMeta 回應裡
                .setStoreCode(sellerMeta.getStoreCode())
                .setDeliveryTemp(temp.value())
                .setShippingFee(String.valueOf(shippingFee))
                .addAllItems(itemPreviews)
                .setSubtotalOriginalAmount(String.valueOf(orderSubtotal))
                .setDiscountAmount(String.valueOf(discountAmount))
                .build();

        // 計算最終實付總額
        long totalActualAmount = Math.max(0, orderSubtotal + shippingFee - discountAmount);

        // 政策快照：伺服端推導，**不讀** request 的 consolidated（65）。
        // ⚠️ consolidated 參數仍留在簽章與 proto 上：移除它是跨 repo 的破壞性契約變更，
        //    要走擴張—收縮（後端先不讀 → 前端停送 → 再移欄）。**本波只做「不讀」這一半**，
        //    留著不讀比留著讀更安全，但它是一個尚未收縮的欄位，登記在此以免被當成 target design。
        OrderPolicySnapshot policySnapshot = orderPolicyResolver.resolve();
        String paymentMerId = resolvePaymentMerId(policySnapshot, sellerMeta);

        // 寫入 Redis 金額快照（含 paymentMerId 給 confirm 讀回）
        String snapshotJson = buildPriceSnapshot(orderSubtotal, shippingFee, discountAmount,
                totalActualAmount, paymentMerId, policySnapshot);
        redisTemplate.opsForValue().set(CHECKOUT_KEY_PREFIX + checkoutId, snapshotJson, CHECKOUT_TTL);

        log.info("CheckoutService.preview.done", "結帳預覽完成",
                "checkoutId", checkoutId);
        CheckoutPreviewData.Builder out = CheckoutPreviewData.newBuilder()
                .setCheckoutId(checkoutId)
                .addOrderGroups(group)
                .setTotalOriginalAmount(String.valueOf(orderSubtotal))
                .setTotalShippingFee(String.valueOf(shippingFee))
                .setTotalDiscount(String.valueOf(discountAmount))
                .setTotalActualAmount(String.valueOf(totalActualAmount))
                .setPaymentMerId(paymentMerId)
                // §3.4.1 付款方式可用性（依 total_actual_amount 算；零元 → 空清單）
                .addAllAvailablePaymentMethods(PaymentMethodAvailabilityConverter.resolve(totalActualAmount));

        // §6.2.4 order ②：發票區（58A §B01）。拿不到就不設這一欄，前端不渲染發票區。
        CheckoutInvoicePreview invoicePreview = resolveInvoicePreview(
                storeId, buyerId, checkoutId, orderSubtotal, shippingFee, discountAmount);
        if (invoicePreview != null) {
            out.setInvoice(invoicePreview);
        }
        return out.build();
    }

    /**
     * 決策本次交易的付款 PayUni MerID。
     *
     * <p>🔴 <b>由 {@code collection_model} 驅動，不再讀 buyer request 透傳的 {@code consolidated}。</b>
     * 需求書把「移除 consolidated 由 buyer request 透傳、改由後端推導」列為不可妥協的工程邊界之一
     * ——收款主體不能由呼叫端指定。推導在 {@link OrderPolicyResolver}。
     *
     * <p>🔴 <b>不補這一段的話，會是本波最安靜的一個失效</b>：
     * {@code orders.collection_model} 被正確寫成平台代收（快照欄、有 DB CHECK 守著），
     * 帳本也據此記「平台代理商戶收到買家款」——<b>而錢實際上還在賣家的 MerID</b>。
     * 快照欄與現金流向不一致，沒有任何東西會發現：訂單成功、付款成功、帳本平衡，
     * 對帳查詢也查得到（只是查到的是賣家子商店的交易）。
     */
    private String resolvePaymentMerId(OrderPolicySnapshot policy, SellerMeta sellerMeta) {
        String model = policy.collectionModel();
        // 🔴 窮舉式判定，**沒有 fall-through**。改成 switch 之前，這裡是
        //    「等於 PLATFORM_AGENT_MERCHANT 才特殊，其餘一律賣家 MerID」——
        //    於是 null／`PROVIDER_COLLECTED`／`EXTERNAL_PREPAID`／任何打錯的字串
        //    都被靜默解析成賣家收款。那兩個值是 DB CHECK 的合法值，
        //    只是今天還沒有寫入路徑；它們一旦出現，錢會進錯人的口袋而快照欄說得很正確。
        if (model == null || model.isBlank()) {
            throw new OrderBusinessException(GrpcResponseCode.FULFILLMENT_COMMERCE_MODEL_UNKNOWN);
        }
        if (CollectionModel.SELLER_MERCHANT.value().equals(model)) {
            return sellerMeta.getPayuniMerId();
        }
        if (CollectionModel.PLATFORM_AGENT_MERCHANT.value().equals(model)) {
            if (agentMerId == null || agentMerId.isBlank()) {
                // fail-closed 在**結帳當下**，不是建單後才發現。
                // ⚠️ 這條在受託代銷啟用前一定會觸發——那是正確行為，不是要繞過的障礙。
                throw new OrderBusinessException(GrpcResponseCode.PLATFORM_COLLECTION_NOT_CONFIGURED);
            }
            return agentMerId;
        }
        // PROVIDER_COLLECTED／EXTERNAL_PREPAID／未來新增值／未知字串一律擋下。
        // 判準與撥款資格 PayoutEligibility 的 default⇒BLOCKED 同源：
        // ⚠️ 2026-09-12 架構合併：該判準原本在獨立的 settlement 服務，現落 payment-service。
        // 對一個我們看不懂的收款模式做出「照賣家收」的結論，是猜測不是推導。
        throw new OrderBusinessException(GrpcResponseCode.FULFILLMENT_COMMERCE_MODEL_UNKNOWN);
    }

    // ==================== 確認結帳 ====================

    /**
     * 確認結帳（建單 + 回傳付款資訊）
     * 重新驗證所有資料，建單後消耗優惠券
     */
    public CheckoutConfirmData confirm(CheckoutConfirmRequest request) {
        log.info("CheckoutService.confirm.start", "確認結帳",
                "checkoutId", request.getCheckoutId());

        // 驗證 checkoutId 有效性（原子 GET+DEL，同時取出金額快照）
        String snapshotJson = redisTemplate.opsForValue().getAndDelete(CHECKOUT_KEY_PREFIX + request.getCheckoutId());
        if (snapshotJson == null || snapshotJson.isEmpty()) {
            throw new CheckoutExpiredException();
        }

        // 從 snapshot 讀回 preview 決策的 paymentMerId（20 設計書 §4.13.4）
        // confirm 不重新決策，避免 preview/confirm drift
        String paymentMerIdFromSnapshot = extractPaymentMerIdFromSnapshot(snapshotJson);

        // 🔴 政策快照同樣只從 preview 讀回，**不在 confirm 重算**（65）。
        //    重算會讓「preview 顯示的收款主體」與「訂單實際寫入的政策」來自兩次求值。
        //    缺欄位一律 fail-closed：讓買家重新 preview，而不是靜默吃 DB DEFAULT。
        OrderPolicySnapshot policySnapshot = extractPolicySnapshot(snapshotJson);

        long buyerId = request.getBuyerId();
        List<CheckoutItem> items = request.getItemsList();
        ResolveResult resolveResult = resolveItems(items);
        List<ResolvedItem> resolved = resolveResult.resolved();
        List<UnavailableItemInfo> unavailable = resolveResult.unavailable();

        // §4.4 步驟 2.5 自買防護 + 步驟 2.6 可購買性最終攔截
        if (resolved.isEmpty()) {
            throw new UnavailableItemsException(unavailable);
        }
        enforceNoSelfPurchase(buyerId, resolved);
        if (!unavailable.isEmpty()) {
            throw new UnavailableItemsException(unavailable);
        }

        long storeId = resolved.get(0).storeId();

        String deliveryTempCode = request.getDeliveryTemp();
        InputValidator.requireNotEmpty(deliveryTempCode, "配送溫層");
        // 收件人資訊：長度上限為 orders 欄寬（recipient_name 50、shipping_address 255），地址一律必填
        InputValidator.requireNotEmpty(request.getRecipientName(), "收件人姓名");
        ColumnWidth.requireWithin(request.getRecipientName(), ColumnWidth.RECIPIENT_NAME, "收件人姓名");
        InputValidator.requireMobile(request.getRecipientPhone(), "收件人電話");
        InputValidator.requireAddress(request.getShippingAddress(), "收件地址");
        ColumnWidth.requireWithin(request.getShippingAddress(), ColumnWidth.SHIPPING_ADDRESS, "收件地址");

        SellerMeta sellerMeta = sellerGrpcClient.getSellerMeta(storeId);

        // 03 §5.1a.1 / 46 §6.2：賣家停權收斂點 backstop 第二道（建單前最後防線；race：驗證通過後
        // 到本行之間才被停權的極窄窗口仍可能建單，屬既有 race 容忍範圍，見 03 §5.1a.1 race 說明）
        requireSellerAcceptingOrders(sellerMeta);

        ShippingTemperature temp = ShippingTemperature.fromValue(deliveryTempCode);

        // 完整驗證
        ValidationResult validationResult = checkoutValidator.validate(resolved);
        if (!validationResult.valid()) {
            String msg = validationResult.errors().stream()
                    .map(ValidationError::errorMessage)
                    .collect(Collectors.joining("; "));
            throw new OrderBusinessException(GrpcResponseCode.CHECKOUT_VALIDATION_FAILED, msg);
        }

        // 優惠券驗證 — couponTemplateId 為使用者選擇的模板 ID
        long orderSubtotal = calcSubtotal(resolved);
        long discountAmount = 0;
        String couponTemplateId = request.getCouponTemplateId();
        if (couponTemplateId != null && !couponTemplateId.isEmpty()) {
            CouponValidateResult couponResult = couponGrpcClient.validateCoupon(
                    couponTemplateId, buyerId, storeId, orderSubtotal);
            discountAmount = InputValidator.requireNonNegativeAmount(couponResult.getDiscountAmount(), "折扣金額");
        }

        // 折扣分攤（與 preview 一致）
        Map<Long, Long> shareDiscountBySku = allocateShareDiscount(resolved, discountAmount);
        // 賣家自寄，B8 出貨設計落地前運費固定 0（與 preview 一致）
        long shippingFee = OrderConstants.CHECKOUT_SHIPPING_FEE;

        long totalActualAmount = Math.max(0, orderSubtotal + shippingFee - discountAmount);

        // §2A.2 L1 上限攔截：訂單建立時尚未選 paymentMethod，使用最寬鬆上限提早攔截不可能成立的訂單。
        // 下限不在 L1 檢查（信用卡 1 元、ATM 15 元），留給 payment-service L2 按 paymentMethod 精準檢核。
        // 0 元由 §2A.1 零元路徑處理，不進入本檢查。
        if (totalActualAmount > PaymentMethodLimitRules.ORDER_MAX_AMOUNT) {
            log.warn("CheckoutService.confirm.amountExceedsLimit",
                    "訂單金額超出上限 amount=" + totalActualAmount
                            + " limit=" + PaymentMethodLimitRules.ORDER_MAX_AMOUNT
                            + " buyer=" + buyerId);
            throw new OrderBusinessException(
                    GrpcResponseCode.ORDER_AMOUNT_EXCEEDS_LIMIT,
                    totalActualAmount,
                    PaymentMethodLimitRules.ORDER_MAX_AMOUNT);
        }

        // 金額比對（Preview → Confirm 防護）
        comparePriceSnapshot(snapshotJson, orderSubtotal, discountAmount, totalActualAmount);

        // 建立訂單（傳入 unit_price + list_price 雙快照 + share_discount 分攤）
        List<OrderService.CreateOrderItemParam> orderItems = resolved.stream()
                // catalog 建單：走 CreateOrderItemParam.catalog（primitive long），
                // RFQ 放寬 sku_id 之後這是 catalog 路徑剩下的型別層守門（64 號 §6.4.15）。
                .map(r -> OrderService.CreateOrderItemParam.catalog(
                        r.skuId(), r.skuName(), r.quantity(), r.unitPrice(), r.listPrice(),
                        shareDiscountBySku.getOrDefault(r.skuId(), 0L),  // 03 §3.2.1：建單時寫入分攤折扣
                        r.imageObjectKey(),
                        Long.parseLong(r.productId()), r.productName()))
                .toList();

        // OrderEntity.mer_id 寫入來源 = preview 階段決策並存入 snapshot 的 paymentMerId
        // （取代原本「直接用 sellerMeta.getPayuniMerId()」邏輯，20 設計書 §4.13.4）
        // 🔴 fallback 改為「用同一份政策重新解析」，不再無條件退回 sellerMeta。
        //    舊寫法在 collection_model = PLATFORM_AGENT_MERCHANT 而快照缺 paymentMerId 時，
        //    會把款項導到賣家子商店，**同時**把 collection_model 寫成平台代收——
        //    快照欄與現金流向不一致，而兩邊各自看起來都正常。
        String orderMerId = (paymentMerIdFromSnapshot != null && !paymentMerIdFromSnapshot.isBlank())
                ? paymentMerIdFromSnapshot
                : resolvePaymentMerId(policySnapshot, sellerMeta);

        // 58 號 §7.1.2 步驟 1：採集買家發票選擇。**選填**——未帶時走 USE_BUYER_DEFAULT，
        // 由 invoice-service 在建立文件時解析（見 InvoiceIntakeResolver 的類別註解）。
        // 驗證在建單**之前**做：格式錯要讓買家當場改，而不是建完單才在 DB CHECK 上炸。
        //
        // §6.2.4 order 的第 ③ 項：呼叫 invoice-service 的 ValidateOrderInvoiceInputInternal。
        //
        // 2026-09-01 接線（原本因法定告知而擱置，見 #2900）。解除的是**兩個**前提，缺一都不能接：
        //   ① DEC-20260831-210934 把法定告知改成「只顯示不強制勾選」⇒ 那支 RPC 的第一格
        //      不再是擋門，`disclosure_accepted` 兩個欄位已從 proto 移除（欄號 reserved）。
        //      在此之前接線＝凡賣家非 AMEGO_AUTO 一律 9314，而沒有任何客戶端送得出那個欄位。
        //   ② 待審接單額度的累加補上了（BumpPendingOrderCounters 先前零呼叫端）。
        //      在此之前接線＝五格檢查全部不會拒絕任何東西，裝一個永遠不作用的檢查。
        //
        // ⚠️ 今天真正會拒絕的只有 **9326（待審接單雙上限）**：其餘四格都要買家送出
        //    `invoice` 區塊才觸發，而已上架 App 不帶、網頁 B02 尚未實作。B02 上線後
        //    9302／9305／9306 才會開始發揮作用。
        validateInvoiceInputOrThrow(request, buyerId, storeId,
                orderSubtotal, shippingFee, discountAmount, totalActualAmount);

        // 58 號 §7.1.2 步驟 1：採集買家發票選擇。**選填**——未帶時走 USE_BUYER_DEFAULT，
        // 由 invoice-service 在建立文件時解析（見 InvoiceIntakeResolver 的類別註解）。
        InvoiceIntakeResolver.Snapshot invoiceSnapshot =
                InvoiceIntakeResolver.resolve(request.getInvoice(), request.hasInvoice());

        OrderService.CreateOrderParams params = new OrderService.CreateOrderParams(
                buyerId, storeId, orderItems,
                orderSubtotal, totalActualAmount,
                request.getCheckoutId(),
                temp.value(), shippingFee,
                request.getRecipientName(), request.getRecipientPhone(),
                request.getShippingAddress(), request.getShippingNote(),
                request.getOrderRemark(),
                orderMerId,
                discountAmount,
                invoiceSnapshot,
                policySnapshot
        );

        String orderNo = orderService.createOrderFromCheckout(params);

        // 建單成功後鎖定優惠券（硬依賴：失敗需補償）
        // reserveCoupon 接受 templateId，回傳 instanceId 寫入 orders.coupon_instance_id
        if (couponTemplateId != null && !couponTemplateId.isEmpty()) {
            try {
                String instanceId = couponGrpcClient.reserveCoupon(couponTemplateId, buyerId, orderNo, storeId, orderSubtotal);
                orderService.updateCouponInstanceId(orderNo, instanceId);
            } catch (Exception e) {
                log.error("CheckoutService.confirm.reserveCoupon.failed", "優惠券鎖定失敗，取消訂單", e);
                try {
                    orderService.cancelOrder(buyerId, orderNo);
                    orderService.releaseStockAfterCancel(orderNo);
                    // 購物車已在建單時清理（建單即清策略），無需解鎖
                } catch (Exception compensateErr) {
                    log.error("CheckoutService.confirm.compensate.failed", "補償操作失敗", compensateErr);
                }
                // reserve 可能在 coupon 端已成功、只是回應遺失或回寫 instance id 失敗：訂單沒有記下 instance id，
                // 其他釋放路徑都找不到這張券；reserve 逾時時 Lock 也可能還在途中、晚於當場釋放才提交。
                // 以 order_no 釋放（冪等；coupon 端沒鎖時是空操作）並排一筆延後到 reserve deadline 之後的任務（D3）。
                couponReleaseService.releaseWithLateLockRecheck(orderNo);
                throw new OrderBusinessException(GrpcResponseCode.COUPON_LOCK_FAILED, e.getMessage());
            }
        }

        log.info("CheckoutService.confirm.done", "確認結帳完成",
                "orderNo", orderNo);
        return CheckoutConfirmData.newBuilder()
                .setCheckoutId(request.getCheckoutId())
                .addOrderNos(orderNo)
                .setTotalPayAmount(String.valueOf(totalActualAmount))
                .build();
    }

    // ==================== 工具方法 ====================

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
        // 批次取所有 SKU（接受 inactive）— 取代既有 N+1 getPublicProduct（後者對 inactive throw）
        List<String> skuIds = checkoutItems.stream().map(CheckoutItem::getSkuId).toList();
        BatchGetProductForCheckoutResponse batchResp = productGrpcClient.batchGetProductForCheckout(skuIds);
        Map<String, ProductForCheckoutInfo> infoMap = batchResp.getDataMap();

        List<ResolvedItem> resolved = new ArrayList<>();
        List<UnavailableItemInfo> unavailable = new ArrayList<>();

        for (CheckoutItem item : checkoutItems) {
            ProductForCheckoutInfo info = infoMap.get(item.getSkuId());

            // quantity 防護式解析（code-reviewer #7）：若 quantity 不合法（"0" / 負數 / 非數字）
            // 不 throw，視為 INVALID_ITEM 進 unavailable 並 continue。
            // 確保「下架商品 + 異常 quantity」混合場景仍能回 structured unavailable_items。
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
                // LOOKUP_FAILED — 不進 resolved，純 unavailable_items 表達
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
                        // INSUFFICIENT_STOCK 帶 stock；OUT_OF_STOCK 帶 "0"；DELISTED / LOOKUP_FAILED 空字串
                        switch (p.reason()) {
                            case "INSUFFICIENT_STOCK" -> String.valueOf(stockQty);
                            case "OUT_OF_STOCK" -> "0";
                            default -> "";
                        }));
                // **不可購買 item 不進 resolved**（code-reviewer #1 / #3）：
                // 若進 resolved，後續 InputValidator.requirePositiveAmount 對 price=0/null 的 inactive
                // SKU 會 throw IllegalArgumentException → 整批 INVALID_PARAM(400)，unavailable_items 永遠拿不到。
                // 且 inactive item 的 weight / dimension 會污染 validator 材積判定 → 運費失真。
                // Validator 仍能對 purchasable 子集跑跨賣家 / 溫層 errors，§4.15「一次看完整錯誤」由
                // unavailable_items + errors 並列回傳達成。
                continue;
            }

            // 商品支援的配送溫層
            List<SkuShippingOption> options = info.getShippingMethodsList().stream()
                    .map(sm -> new SkuShippingOption(sm.getTemperature()))
                    .toList();

            // 02 / 20 §2.8 / §4.3：實際成交單價 = sale_price ?? list_price
            long listPrice = InputValidator.requirePositiveAmount(info.getListPrice(), "商品原價");
            long unitPrice = info.getSalePrice().isEmpty()
                    ? listPrice
                    : InputValidator.requirePositiveAmount(info.getSalePrice(), "商品特價");

            resolved.add(new ResolvedItem(
                    Long.parseLong(item.getSkuId()),
                    info.getSkuName() != null ? info.getSkuName() : "",
                    requestedQty,
                    unitPrice,
                    listPrice,
                    stockQty,
                    Long.parseLong(info.getStoreId()),
                    item.getProductId(),
                    info.getProductName() != null ? info.getProductName() : "",
                    options,
                    info.getImageObjectKey() != null ? info.getImageObjectKey() : "",
                    info.getIsActive()
            ));
        }
        return new ResolveResult(resolved, unavailable);
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

    // package-visible test seams（mirrors CheckoutServiceSelfPurchaseTest 的 xxxForTest 慣例）：
    // 讓測試不需要 bootstrap 整條 validate()/preview()/confirm() pipeline 就能直接驗業務規則。

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
        // 實際成交小計：使用 unit_price（特價優先）× quantity
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

    /**
     * 從 Redis snapshot JSON 取出 paymentMerId。
     * preview 階段已決策並寫入，confirm 階段直接讀回，避免 drift。
     */
    private String extractPaymentMerIdFromSnapshot(String snapshotJson) {
        if (snapshotJson == null || snapshotJson.isEmpty() || "1".equals(snapshotJson)) {
            return null;
        }
        try {
            JsonNode node = objectMapper.readTree(snapshotJson);
            String v = node.path("paymentMerId").asText("");
            return v.isBlank() ? null : v;
        } catch (Exception e) {
            log.warn("CheckoutService.extractPaymentMerIdFromSnapshot.failed",
                    "snapshot 解析失敗，退回 sellerMeta 取 merId", "error", e.getMessage());
            return null;
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
     * paymentMerId 為 preview 階段決策結果，confirm 直接讀回避免 drift。
     */
    // package-private（非 private）：讓同 package 的測試用**產線這一支**組快照，
    // 而不是各自手寫一份 JSON——手寫 fixture 等於自己跟自己對，欄名改了不會有人發現。
    String buildPriceSnapshot(long subtotal, long shippingFee, long discount, long totalAmount,
                                       String paymentMerId, OrderPolicySnapshot policy) {
        try {
            Map<String, Object> snapshot = new LinkedHashMap<>();
            snapshot.put("subtotal", subtotal);
            snapshot.put("shippingFee", shippingFee);
            snapshot.put("discount", discount);
            snapshot.put("totalAmount", totalAmount);
            snapshot.put("paymentMerId", paymentMerId);
            // 🔴 政策五欄與 paymentMerId 是**同一次求值的兩個投影**，必須一起進快照。
            //    分開求值（preview 算收款主體、confirm 重算政策）會讓兩者在
            //    「快照說平台代收、錢進賣家子商店」這種組合上不一致，而那是靜默的。
            Map<String, Object> policyMap = new LinkedHashMap<>();
            policyMap.put("commerceModel", policy.commerceModel());
            policyMap.put("collectionModel", policy.collectionModel());
            policyMap.put("fulfillmentMode", policy.fulfillmentMode());
            policyMap.put("invoiceModel", policy.invoiceModel());
            policyMap.put("policyVersion", policy.policyVersion());
            snapshot.put("policy", policyMap);
            return objectMapper.writeValueAsString(snapshot);
        } catch (Exception e) {
            // 🔴 不再退化成 "1"。退化版本沒有政策欄，而 confirm 對缺政策一律 fail-closed
            //    ⇒ 留著這條路只會把「序列化壞了」變成 confirm 階段一個看不懂的錯誤。
            //    在 preview 就炸，錯誤訊息才指得到真正的原因。
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
                        .setOrderNo("")   // 建單前還沒有訂單編號；本 RPC 只拿它做 log 關聯
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
            return;   // 服務不可用 ⇒ 這一輪不驗（見 javadoc）
        }
        if (resp.getCode() == 0) {
            return;
        }
        GrpcResponseCode code = GrpcResponseCode.fromCode(resp.getCode());
        if (code == null) {
            // 值域長胖而 order 這側還沒同步 ⇒ 不得靜默放行，也不得回一個猜的碼。
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
        // 🔴 兩個 carrierType 都必須由這裡明說，**不可以只設 DONATION**。
        //
        // invoice-service 的 identityFromManualInput 第一件事就是
        // `InvoiceCarrierTypeFromValue(spec.CarrierType)`，值域外（含空字串）一律回
        // 9305；它的原地註解逐字寫著「只帶 buyerTaxId 而不帶 carrierType 也走這裡：
        // 統編必須明說 carrierType=TAX_ID，由欄位有沒有值去推語意，就是在憑證上猜
        // 買受人身分」。⇒ 少設這一行，**打統編這條路在 gateway 上結構性不可能成功**。
        //
        // ⚠️ 這個缺口在單元測試裡看不到（order 側自己的 mapper 測試怎麼寫都會過），
        // 也不會讓 CI 紅——它只在真的打一次 dev 的 confirm 時才現形（2026-09-01 實測）。
        // 對應的契約測試在 CheckoutServiceInvoiceInputSpecTest。
        if (!in.getDonationCode().isBlank()) {
            out.setCarrierType(InvoiceCarrierType.DONATION.value())
               .setValue(in.getDonationCode());
        } else if (!in.getBuyerTaxId().isBlank()) {
            // TAX_ID 的 value 必須留空（§6.1.1）：統編走 buyer_tax_id 那一格，
            // 把它同時塞進 value 會被 validateIdentityShape 判成「載具塞進統編欄」。
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
                .setSnapshotVersion("1")   // ← 見 javadoc 紅字：不得用 d.getSnapshotVersion()
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
        // default_profile 可為空 message（買家無 profile）⇒ 只有 profile_id 非空才帶。
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

    private Map<Long, Long> allocateShareDiscount(List<ResolvedItem> items, long discountAmount) {
        Map<Long, Long> result = new LinkedHashMap<>();
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
                share = discountAmount - allocated;            // 尾差歸最後一筆
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
    private void comparePriceSnapshot(String snapshotJson, long newSubtotal,
                                       long newDiscount, long newTotalAmount) {
        if ("1".equals(snapshotJson)) {
            // 向下相容：舊格式快照，跳過比對
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
        long oldDiscount = snapshot.path("discount").asLong(0);
        long oldTotalAmount = snapshot.path("totalAmount").asLong(0);

        // 逐項比對，收集變動原因
        List<String> reasons = new ArrayList<>();
        List<String> reasonLabels = new ArrayList<>();

        if (oldSubtotal != newSubtotal) {
            reasons.add("PRODUCT_PRICE");
            reasonLabels.add("商品價格");
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

        if (reasons.isEmpty()) {
            // 個別項目都一致但總額不同（不應發生，防禦性檢查）
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
                    newSubtotal, OrderConstants.CHECKOUT_SHIPPING_FEE, newDiscount, reasons);
        }
    }
}
