package io.github.will413028.hawkr.order.checkout;

import io.github.will413028.hawkr.common.obs.log.ObsLogger;
import io.github.will413028.hawkr.common.utils.code.GrpcResponseCode;
import io.github.will413028.hawkr.common.utils.code.ShippingFeePayer;
import io.github.will413028.hawkr.common.utils.code.ShippingMethod;
import io.github.will413028.hawkr.common.utils.code.ShippingRoute;
import io.github.will413028.hawkr.common.utils.code.ShippingTemperature;
import io.github.will413028.hawkr.order.exception.OrderBusinessException;
import org.springframework.stereotype.Component;

import java.util.*;
import java.util.stream.Collectors;

/**
 * 結帳驗證器
 *
 * 驗證：賣家一致性、溫層交集、材積/重量限制
 * 計算：可用配送選項（method × temp 組合）
 * 運費費率從 Shipping Service 快取取得（ShippingFeeCacheClient）
 */
@Component
public class CheckoutValidator {

    private static final ObsLogger log = ObsLogger.forClass(CheckoutValidator.class);

    private final ShippingFeeCacheClient shippingFeeCache;

    /**
     * Uber Direct 資格閘門（明定落點就是本類別）。
     *
     * <p>🔴 兩個接點各擋不同的東西，<b>缺一都會漏</b>：
     * <ul>
     *   <li>{@link #validate}：Uber <b>不進 {@code availableOptions}</b> ⇒ 買家在選項列表就看不到它。
     *       這一層不擋任何東西，它只保證「不把一個按下去會失敗的選項端給使用者」。</li>
     *   <li>{@link #lookupSingleOption}：買家<b>指名</b> 3003 時直接拋 {@code 9503}。
     *       shippingFee／preview／confirm 三支都走這裡而<b>不經過</b> {@code validate} 的選項迴圈
     *       （它們自己帶著使用者選定的 method），所以只做上面那一層等於沒擋。</li>
     * </ul>
     */
    private final UberOrderEligibilityGuard uberGuard;

    public CheckoutValidator(ShippingFeeCacheClient shippingFeeCache,
                             UberOrderEligibilityGuard uberGuard) {
        this.shippingFeeCache = shippingFeeCache;
        this.uberGuard = uberGuard;
    }

    /**
     * 執行結帳驗證
     *
     * @param items 已解析的結帳商品列表
     * @param route 運費路線（Validate 階段用 DOMESTIC，其他階段用精確路線）
     * @return 驗證結果
     */
    public ValidationResult validate(List<ResolvedItem> items, ShippingRoute route) {
        List<ValidationError> errors = new ArrayList<>();

        if (items == null || items.isEmpty()) {
            errors.add(new ValidationError("EMPTY_ITEMS", "結帳商品不可為空"));
            return new ValidationResult(false, Set.of(), List.of(), errors, 0);
        }

        // 1. 賣家一致性
        long firstSeller = items.get(0).storeId();
        boolean sameSeller = items.stream().allMatch(i -> i.storeId() == firstSeller);
        if (!sameSeller) {
            errors.add(new ValidationError("DIFFERENT_SELLER", "結帳商品必須來自同一賣家"));
            return new ValidationResult(false, Set.of(), List.of(), errors, 0);
        }

        // 庫存 / 下架 / lookup 失敗檢查移到 caller（CheckoutService）以 ProductPurchasabilityRules
        // 統一收集進 unavailable_items[]（20 §4.14 / §4.15）。Validator 內不再收 row 級
        // PRODUCT_UNAVAILABLE。

        // 2. 溫層交集（每個 SKU 支援的配送溫層取交集）
        Set<String> tempIntersection = null;
        for (ResolvedItem item : items) {
            Set<String> itemTemps = item.shippingOptions().stream()
                    .map(SkuShippingOption::temperatureCode)
                    .collect(Collectors.toSet());
            if (tempIntersection == null) {
                tempIntersection = new HashSet<>(itemTemps);
            } else {
                tempIntersection.retainAll(itemTemps);
            }
        }
        if (tempIntersection == null || tempIntersection.isEmpty()) {
            errors.add(new ValidationError("TEMP_ZONE_INCOMPATIBLE", "商品的配送溫層不相容，無法合併配送"));
            return new ValidationResult(false, Set.of(), List.of(), errors, 0);
        }

        // 4. (method, temp) 對的交集：所有 SKU 都支援的配送組合
        Set<String> methodTempIntersection = null;
        for (ResolvedItem item : items) {
            Set<String> itemPairs = item.shippingOptions().stream()
                    .map(opt -> opt.methodCode() + ":" + opt.temperatureCode())
                    .collect(Collectors.toSet());
            if (methodTempIntersection == null) {
                methodTempIntersection = new HashSet<>(itemPairs);
            } else {
                methodTempIntersection.retainAll(itemPairs);
            }
        }
        if (methodTempIntersection == null || methodTempIntersection.isEmpty()) {
            errors.add(new ValidationError("ALL_SHIPPING_EXCEEDED", "所有配送方式均不相容"));
            return new ValidationResult(false, tempIntersection, List.of(), errors, 0);
        }

        // 5. 材積/重量計算（最短邊堆疊法）
        PackageDimension pkg = calculatePackageDimension(items);

        // 6. 逐一檢查每個 (method, temp) 組合
        List<AvailableOption> availableOptions = new ArrayList<>();
        for (String pair : methodTempIntersection) {
            String[] parts = pair.split(":");
            ShippingMethod method = ShippingMethod.fromValue(parts[0]);
            ShippingTemperature temp = ShippingTemperature.fromValue(parts[1]);

            // Uber Direct 資格閘門（顯示層 fail-closed 第 ③ 層）。
            // 🔴 放在 config／級距查找**之前**：Uber 今天「關閉」的機制是資料面缺 config 列，
            //    那條路徑走的是下一行的 `if (config == null) continue;`——靜默跳過、零 log 零指標，
            //    而且有人補上那一列的當天就自動變成放行。這裡先用程式面的具名規則排除它，
            //    讓「為什麼看不到 Uber」有一行可查的 log，而不是一個 continue。
            // 🔴 methodOnly（沒有 probe）：選項列表走的是**靜態費率表**，而 Uber 沒有級距表——
            //    它的價格只有即時報價算得出來，所以本來就不從這個迴圈出（前端走
            //    POST /app/checkout/uber_quote）。這裡的排除是正確行為，不是「還沒接線」。
            UberOrderEligibilityGuard.Decision uberDecision =
                    uberGuard.evaluate(UberOrderEligibilityGuard.Context.methodOnly(method.value()));
            if (!uberDecision.allowed()) {
                log.info("CheckoutValidator.validate.optionExcluded",
                        "配送選項因資格檢查未通過而排除：" + uberDecision.denyReason(),
                        "shippingMethodTemp", method.value() + ":" + temp.value());
                continue;
            }

            // 物流限制檢查（從 Shipping Service 快取取得）
            ShippingFeeCacheClient.ConfigEntry config = shippingFeeCache.getConfig(method.value(), temp.value());
            if (config == null) continue;
            if (!shippingFeeCache.isWithinLimit(config, pkg.totalDimSum(), pkg.maxSingleSide(), pkg.totalWeightGram())) continue;

            // 運費級距查找（ShippingRoute → delivery_zone 轉換）
            String deliveryZone = ShippingFeeCacheClient.toDeliveryZone(route);
            Long fee = shippingFeeCache.lookupFee(method.value(), temp.value(), deliveryZone, pkg.totalDimSum());
            if (fee == null) continue;

            // 暫定免運判定（CheckoutShippingFee 階段；尚未套用優惠券）
            FreeShippingResult fsr = checkFreeShipping(items, parts[0], parts[1], null);
            long actualFee = fsr.eligible() ? 0L : fee;

            String label = method.label() + "（" + temp.label() + "）";

            availableOptions.add(new AvailableOption(
                    method, temp, label, actualFee, fee, fsr.feePayer(), fsr.lowestThreshold(), fsr.eligible()));
        }

        if (availableOptions.isEmpty()) {
            errors.add(new ValidationError("ALL_SHIPPING_EXCEEDED", "所有配送方式均超過材積或重量限制"));
            return new ValidationResult(false, tempIntersection, List.of(), errors, pkg.totalDimSum());
        }

        return new ValidationResult(true, tempIntersection, availableOptions, List.of(), pkg.totalDimSum());
    }

    /**
     * 針對已選定的 method × temp 查找運費。
     *
     * <p>🔴 <b>3003 走另一條路</b>：Uber 在 {@code shipping_fee_config} 沒有任何一列
     * （它沒有級距表），運費是一筆<b>會過期的即時報價</b>。所以本方法對 Uber 不查費率表，
     * 直接採用 {@code uberContext} 這一次判定回來的
     * {@link UberOrderEligibilityGuard.Assessment#feeAmount()}——
     * 「買家看到的選項」與「按下去的價格」必須來自同一次求值（INV-U1）。
     *
     * @param uberContext 本次判定的報價 context。<b>沒有多載</b>：每個呼叫端都必須明寫
     *                    自己有沒有 probe（{@link UberOrderEligibilityGuard.Context#of} ／
     *                    {@link UberOrderEligibilityGuard.Context#methodOnly}），
     *                    理由見 {@code Context} 的 javadoc
     * @return 運費資訊，若選項不可用則回傳 null
     */
    public AvailableOption lookupSingleOption(List<ResolvedItem> items, ShippingMethod method,
                                               ShippingTemperature temp, ShippingRoute route,
                                               UberOrderEligibilityGuard.Context uberContext) {
        // 🔴 呼叫端手上的 method 與 context 裡的必須是同一個，否則會出現
        //    「拿 3001 的 context 放行 3003」這種只在特定呼叫順序下成立的破口。
        if (uberContext == null || !method.value().equals(uberContext.shippingMethodValue())) {
            throw new IllegalArgumentException(
                    "uberContext 的配送方式與本次查找不一致：context="
                            + (uberContext == null ? "null" : uberContext.shippingMethodValue())
                            + " method=" + method.value());
        }

        // Uber Direct 資格閘門（結帳鏈，買家已指名 3003）。
        // 🔴 這裡**拋**而不是 return null：return null 會被三支 caller 收斂成
        //    4008 SHIPPING_METHOD_UNAVAILABLE「所選配送方式不可用」，那個碼吐不出「為什麼」，
        //    也與「材積超限」「無費率級距」混在同一個出口 ⇒ 前端與維運都分不出是哪一種。
        UberOrderEligibilityGuard.Decision uberDecision = uberGuard.requireEligible(uberContext);
        if (uberDecision.applicable()) {
            return uberOption(method, temp, uberDecision.assessment());
        }

        PackageDimension pkg = calculatePackageDimension(items);

        ShippingFeeCacheClient.ConfigEntry config = shippingFeeCache.getConfig(method.value(), temp.value());
        if (config == null) return null;
        if (!shippingFeeCache.isWithinLimit(config, pkg.totalDimSum(), pkg.maxSingleSide(), pkg.totalWeightGram())) return null;

        String deliveryZone = ShippingFeeCacheClient.toDeliveryZone(route);
        Long fee = shippingFeeCache.lookupFee(method.value(), temp.value(), deliveryZone, pkg.totalDimSum());
        if (fee == null) return null;

        // 暫定免運判定（lookupSingleOption 用於 ShippingFee 階段，尚未套用優惠券）
        FreeShippingResult fsr = checkFreeShipping(items, method.value(), temp.value(), null);
        long actualFee = fsr.eligible() ? 0L : fee;
        String label = method.label() + "（" + temp.label() + "）";

        return new AvailableOption(method, temp, label, actualFee, fee, fsr.feePayer(), fsr.lowestThreshold(), fsr.eligible());
    }

    /**
     * 把 Uber 的報價轉成一筆 {@link AvailableOption}。
     *
     * <p>🔴 <b>不套免運門檻</b>：免運的機制是「賣家為某個級距費率配送方式設一個門檻」，
     * 而 Uber 的價格是逐趟報價。把門檻套上去＝在沒有任何人做過決定的情況下，
     * 讓某一方吃掉一筆隨距離浮動、無上限的外送費。
     * 提示詞 W4-4 逐字：「Buyer Shipping Charge = 0 不代表 Platform 負擔」——
     * 誰補貼是 subsidy 那條軸的決定（{@code seller_subsidy}／{@code platform_subsidy}
     * 各自保存），不是這裡的副作用。
     *
     * <p>⚠️ {@code rawFee == actualFee}：confirm 的最終金額走的是 {@code rawFee}
     * （見 {@code CheckoutService.confirm} 的 {@code finalFsr}），兩者不相等會讓
     * 訂單運費與報價脫鉤，直接違反 INV-U1。
     */
    private AvailableOption uberOption(ShippingMethod method, ShippingTemperature temp,
                                       UberOrderEligibilityGuard.Assessment assessment) {
        long fee;
        try {
            fee = Long.parseLong(assessment.feeAmount());
        } catch (NumberFormatException | NullPointerException e) {
            // 走到這裡代表 shipping 回了 eligible=true 卻給不出金額——那是契約被破壞，
            // 不是業務上的「不可用」。吞成 0 元會讓買家免費叫車，所以一律拒絕。
            log.error("CheckoutValidator.uberOption.feeUnparseable",
                    "Uber 判定可用但運費解析不出來：" + assessment.feeAmount(), e);
            throw new OrderBusinessException(GrpcResponseCode.FULFILLMENT_PROVIDER_NOT_ENABLED);
        }
        if (fee < 0) {
            log.error("CheckoutValidator.uberOption.feeNegative",
                    "Uber 回了負運費：" + assessment.feeAmount(), "quoteId", assessment.quoteId());
            throw new OrderBusinessException(GrpcResponseCode.FULFILLMENT_PROVIDER_NOT_ENABLED);
        }
        String label = method.label() + "（" + temp.label() + "）";
        return new AvailableOption(method, temp, label, fee, fee,
                ShippingFeePayer.BUYER.value(), 0L, false, assessment.quoteId());
    }

    /**
     * 建單成功後把報價鎖定成 {@code CHECKOUT_ACCEPTED}。
     *
     * <p>🔴 <b>它是 {@code UberDirectAdapter.acceptedQuote(orderNo)} 的唯一寫入端。</b>
     * 沒有這一步，建單會走到 {@code markCreateFailed("UBER_QUOTE_NOT_ACCEPTED")}
     * ——買家付了錢而 shipment 建不出來。
     *
     * <p>失敗（含 {@code price_changed}）一律由呼叫端補償（取消訂單），<b>不得</b>自動接受新價：
     * 自動接受＝對買家補收（違約）；自動沿用舊價＝我方吃差額且無上限。
     *
     * @return shipping 的裁決；{@code allowed()==false} 代表沒鎖到
     */
    public UberOrderEligibilityGuard.Decision acceptUberQuote(UberOrderEligibilityGuard.Probe probe) {
        return uberGuard.evaluate(UberOrderEligibilityGuard.Context.of(
                ShippingMethod.UBER_INSTANT.value(), probe));
    }

    /**
     * Uber 即時報價的資格判定——<b>唯一一個拿得到完整報價 context 的接點</b>。
     *
     * <p>呼叫端是 {@code CheckoutService.uberQuote}（{@code POST /app/checkout/uber_quote}）：
     * 它收 {@code address_id} 並向 buyer-service 取回 city／district／detail_address 三欄，
     * 是整條結帳鏈上唯一組得出 {@link UberOrderEligibilityGuard.Probe} 的地方。
     *
     * <p>🔴 <b>為什麼報價端點要走閘門，而不是自己打 shipping</b>：買家「看得到的選項」與
     * 「按下去會不會過」必須來自同一次判定。兩邊各打各的 RPC 會製造
     * 「報價說可以、建單說不行」——正是本閘門存在要消滅的那個形狀。
     * 回傳的 {@link UberOrderEligibilityGuard.Decision#assessment()} 已帶 quote_id 與運費，
     * 呼叫端不需要（也不得）再打第二次。
     */
    public UberOrderEligibilityGuard.Decision assessUberQuote(UberOrderEligibilityGuard.Probe probe) {
        return uberGuard.evaluate(UberOrderEligibilityGuard.Context.of(
                ShippingMethod.UBER_INSTANT.value(), probe));
    }

    // ==================== 材積 & 免運計算 ====================

    /**
     * 最短邊堆疊法計算包裹材積
     *
     * 同一 SKU 多件沿最短邊堆疊；不同 SKU 間取最大中/長邊，最短邊加總。
     * totalDimSum = Σ(shortSide × qty) + max(midSide) + max(longSide)
     */
    private PackageDimension calculatePackageDimension(List<ResolvedItem> items) {
        int stackedShort = 0;
        int maxMid = 0;
        int maxLong = 0;
        long totalWeightGram = 0;

        for (ResolvedItem item : items) {
            int[] dims = {item.lengthCm(), item.widthCm(), item.heightCm()};
            Arrays.sort(dims);

            stackedShort += dims[0] * item.quantity();
            maxMid = Math.max(maxMid, dims[1]);
            maxLong = Math.max(maxLong, dims[2]);

            totalWeightGram += item.weightGram() * item.quantity();
        }

        int totalDimSum = stackedShort + maxMid + maxLong;
        int maxSingleSide = Math.max(stackedShort, Math.max(maxMid, maxLong));

        return new PackageDimension(totalDimSum, maxSingleSide, totalWeightGram);
    }

    /**
     * 逐商品免運判定（20 設計書 §4.7 / §4.7.2）。
     *
     * 規則：
     * 1. feePayer=SELLER 是前提條件，feePayer=BUYER 的商品不具備免運資格
     * 2. feePayer=SELLER 且 freeThreshold &lt;= 0（未設門檻）時，無條件免運（§4.7.0-A）：
     *    該商品直接達標，不受 freeThreshold / 優惠券 / 折扣後小計影響
     * 3. feePayer=SELLER 且 freeThreshold &gt; 0 時，折扣後判定（§4.7.0-B）：用該商品實際成交小計 − 分攤折扣判定
     * 4. 每個商品獨立計算，不同商品金額不可合併
     * 5. 任一商品達標 → 整筆配送免運
     *
     * 兩階段呼叫：
     * - **暫定免運判定（tentative）**：shareDiscountBySkuId = null 或空 Map
     *   → 以 unit_price × quantity 對門檻判定（CheckoutShippingFee 階段）
     * - **最終免運判定（final）**：shareDiscountBySkuId 帶優惠券分攤
     *   → 以 unit_price × quantity − share_discount 對門檻判定（CheckoutPreview / Confirm 階段）
     *
     * @param items 結帳商品
     * @param methodCode 配送方式 value
     * @param tempCode 配送溫層 value
     * @param shareDiscountBySkuId 各品項分攤的優惠券折扣（key = skuId，value = 折扣金額）；
     *                             null 或空 = 不扣折扣（暫定判定）
     */
    public FreeShippingResult checkFreeShipping(List<ResolvedItem> items,
                                                  String methodCode, String tempCode,
                                                  Map<Long, Long> shareDiscountBySkuId) {
        boolean anyEligible = false;
        String feePayer = ShippingFeePayer.BUYER.value();
        long lowestThreshold = 0;
        Map<Long, Long> discountMap = shareDiscountBySkuId != null ? shareDiscountBySkuId : Map.of();

        for (ResolvedItem item : items) {
            for (SkuShippingOption opt : item.shippingOptions()) {
                if (opt.methodCode().equals(methodCode) && opt.temperatureCode().equals(tempCode)) {
                    // 取第一個匹配的 feePayer
                    if (ShippingFeePayer.BUYER.value().equals(feePayer)) {
                        feePayer = opt.feePayer();
                    }

                    // 僅 feePayer=SELLER 才具備免運資格
                    if (ShippingFeePayer.SELLER.value().equals(opt.feePayer())) {
                        if (opt.freeThreshold() <= 0) {
                            // 無條件免運（20 §4.7.0-A）：賣家負擔運費且未設門檻 → 該商品直接達標，
                            // 買家運費固定 0，不受 freeThreshold / 優惠券 / 折扣後小計影響（issue #1147 產品負責人裁定）。
                            // lowestThreshold 保持 0 表「無門檻」。
                            anyEligible = true;
                        } else {
                            // 門檻免運（20 §4.7.0-B）：折扣後判定（扣掉該品項分攤的優惠券折扣），規範性決策維持不變。
                            // 追蹤最低免運門檻（供前端顯示）
                            if (lowestThreshold == 0 || opt.freeThreshold() < lowestThreshold) {
                                lowestThreshold = opt.freeThreshold();
                            }

                            // 逐商品獨立判定（折扣後判定）
                            long itemSubtotal = item.unitPrice() * item.quantity();
                            long itemDiscount = discountMap.getOrDefault(item.skuId(), 0L);
                            long itemActual = itemSubtotal - itemDiscount;
                            if (itemActual >= opt.freeThreshold()) {
                                anyEligible = true;
                            }
                        }
                    }

                    break; // 此商品已找到對應配送選項
                }
            }
        }

        return new FreeShippingResult(anyEligible, feePayer, lowestThreshold);
    }

    // ==================== 內部資料結構 ====================

    /**
     * 已解析的結帳商品（由 CheckoutService 從 Product Service 回應轉換而來）
     */
    public record ResolvedItem(
            long skuId,
            String skuName,
            int quantity,
            long unitPrice,        // 實際成交單價（無特價=listPrice，有特價=當時 sale_price）
            long listPrice,        // 原價快照（永遠 = SKU.price）
            long stockQty,
            int lengthCm,
            int widthCm,
            int heightCm,
            long weightGram,
            long storeId,
            String productId,
            String productName,
            List<SkuShippingOption> shippingOptions,
            String imageObjectKey,
            boolean isActive        // product.status == ACTIVE 且 deleted_at IS NULL（02 §5.5.2）。
                                    // 即使 isActive=false 仍會被 validator 用於跨賣家 / 溫層 / 材積判定，
                                    // 「不可購買」攔截由 caller 在 ProductPurchasabilityRules 階段收進 unavailable_items。
    ) {
        public int dimensionSum() {
            return lengthCm + widthCm + heightCm;
        }

        public int maxSingleSide() {
            return Math.max(lengthCm, Math.max(widthCm, heightCm));
        }

        /**
         * 推斷存儲溫層（供 Preview 顯示）
         * AMBIENT 配送 → 產品為常溫（只有常溫產品可用常溫配送）
         * 僅 CHILLED → 冷藏產品
         * 僅 FROZEN → 冷凍產品
         */
        public String inferredTemperature() {
            boolean hasAmbient = shippingOptions.stream()
                    .anyMatch(o -> ShippingTemperature.AMBIENT.value().equals(o.temperatureCode()));
            if (hasAmbient) return ShippingTemperature.AMBIENT.value();
            boolean hasChilled = shippingOptions.stream()
                    .anyMatch(o -> ShippingTemperature.CHILLED.value().equals(o.temperatureCode()));
            if (hasChilled) return ShippingTemperature.CHILLED.value();
            return ShippingTemperature.FROZEN.value();
        }
    }

    public record SkuShippingOption(
            String methodCode,
            String temperatureCode,
            long freeThreshold,
            String feePayer
    ) {}

    public record ValidationResult(
            boolean valid,
            Set<String> deliveryTempIntersection,
            List<AvailableOption> availableOptions,
            List<ValidationError> errors,
            int dimensionSum
    ) {}

    public record AvailableOption(
            ShippingMethod method,
            ShippingTemperature temp,
            String label,
            long fee,            // 暫定運費（已套用 tentative 免運判定，免運時為 0）
            long rawFee,         // 級距原始運費（未套免運），供 Preview 在折扣後免運翻轉時重算
            String feePayer,
            long freeThreshold,
            boolean freeShippingEligible,
            // 🔴 只有 3003 非空：本次定價實際採用的 uber_quotes.quote_id。
            //    confirm 要拿它去 CAS 成 CHECKOUT_ACCEPTED，而**重驗允許重報**
            //    ⇒ 它不一定等於買家送上來的那個。
            //    讓它跟著價格一起回來，是為了不讓「定價」與「鎖定」變成兩次求值。
            String uberQuoteId
    ) {
        /** 級距費率選項：沒有報價 id。 */
        public AvailableOption(ShippingMethod method, ShippingTemperature temp, String label,
                               long fee, long rawFee, String feePayer, long freeThreshold,
                               boolean freeShippingEligible) {
            this(method, temp, label, fee, rawFee, feePayer, freeThreshold, freeShippingEligible, "");
        }
    }

    public record ValidationError(
            String errorCode,
            String errorMessage
    ) {}

    /**
     * row 級不可購買項目（CartItemUnavailableReason）。
     * 對應 order.proto UnavailableItem，由 caller 在 resolveItems 階段透過
     * {@link io.github.will413028.hawkr.common.utils.code.helper.ProductPurchasabilityRules} 推導。
     * 20 設計書 §3.2 / §3.4 / §3.5。
     */
    public record UnavailableItemInfo(
            String skuId,
            String skuName,
            String reason,        // CartItemUnavailableReason value
            String requestedQty,
            String availableQty   // INSUFFICIENT_STOCK 時 0<available<requested；OUT_OF_STOCK 時 "0"；其他 ""
    ) {}

    /**
     * resolveItems 的回傳：含可組裝 validator 的 resolved items 與 row 級 unavailable items。
     * LOOKUP_FAILED 的 item 不進 resolved（缺 weight/dimension/shipping_methods，無法跑 validator），
     * 純 unavailable_items 表達；其餘（DELISTED / OUT_OF_STOCK / INSUFFICIENT_STOCK）的 item
     * **仍進 resolved**，validator 跑跨賣家 / 溫層 / 材積判定仍有效。
     */
    public record ResolveResult(
            List<ResolvedItem> resolved,
            List<UnavailableItemInfo> unavailable
    ) {}

    /**
     * 包裹材積計算結果
     */
    record PackageDimension(int totalDimSum, int maxSingleSide, long totalWeightGram) {}

    /**
     * 免運判定結果
     */
    record FreeShippingResult(boolean eligible, String feePayer, long lowestThreshold) {}
}
