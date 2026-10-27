package io.github.will413028.hawkr.order.checkout;

import io.github.will413028.hawkr.common.utils.code.GrpcResponseCode;
import io.github.will413028.hawkr.common.utils.code.ShippingMethod;
import io.github.will413028.hawkr.order.exception.OrderBusinessException;
import org.springframework.beans.factory.annotation.Autowired;
import org.springframework.stereotype.Component;

import java.util.ArrayList;
import java.util.Arrays;
import java.util.EnumMap;
import java.util.LinkedHashSet;
import java.util.List;
import java.util.Set;
import java.util.stream.Collectors;

/**
 * Uber Direct（{@code shipping_method = 3003}）的結帳資格閘門——<b>deny-by-default</b>。
 *
 * <h2>它回答的唯一問題</h2>
 * 「這張訂單<b>現在</b>可以用 Uber 建嗎？」——而不是「3003 是不是一個合法的碼值」
 * （後者是 {@link OrderPolicyResolver} 的事，它只把 3003 歸類成 {@code PLATFORM_LOGISTICS}）。
 *
 * <h2>🔴 為什麼必須在 Buyer Payment 之前</h2>
 * Uber 是<b>即時報價 ＋ 即時派車</b>：報價有壽命、地址不一定在服務範圍、賣家不一定有
 * {@code external_store_id}。這些條件若等到「付款成功後建 shipment」才發現不成立，
 * <b>錢已經收了而配送建不出來</b>——那正是 issue #3308 的形狀（7-11 無 StoreID 可以過 checkout、
 * 建 shipment 才失敗）。所以本閘門的落點在結帳鏈，不在出貨鏈。
 *
 * <h2>判定來自 shipping，本類別不重新判一次</h2>
 * 六道檢查<b>沒有任何一道在 order-service 內自行求值</b>。唯一的證據來源是
 * {@link QuoteAssessor}——它背後是 shipping 的 {@code GetUberQuoteForCheckout} ／
 * {@code ValidateUberQuoteForCheckout}，而那兩支的裁決者都是同一個
 * {@code UberQuoteService.quote()}。本類別做的事只有兩件：
 * <ol>
 *   <li>把 shipping 回的<b>單一理由碼</b>攤成六個具名檢查項（{@link Check#providerReasons()}
 *       是那張對照表本身，不是另一份抄本）；</li>
 *   <li>拿不到證據時<b>拒絕</b>。</li>
 * </ol>
 * 在 order 側複製一份「賣家有沒有開通／地址在不在範圍」的判準，會讓兩邊各自演化，
 * 而買家看到的與建單當下用的是兩個不同的答案。
 *
 * <h2>🔴 取不到證據 ＝ 拒絕（三種取不到，處置相同）</h2>
 * <ul>
 *   <li>{@link Context#probe()} 為 {@code null}——呼叫端沒有報價 context（見下方「誰供得起 probe」）</li>
 *   <li>沒有 {@link QuoteAssessor}——以無參建構子建出來的實例（測試／誤用）</li>
 *   <li>RPC 失敗、或 shipping 回了一個本類別不認得的理由碼</li>
 * </ul>
 * 最後一種特別重要：<b>不認得的理由碼一律六項全拒</b>，而不是挑一項來擋。shipping 之後新增
 * 一個理由而本檔沒跟上時，退化方向是「拒絕」而不是「放行」。
 *
 * <h2>誰供得起 probe</h2>
 * {@link Probe} 需要<b>結構化收件地址</b>（county／district／detail 三欄，見
 * {@code shipping.proto} 的 {@code UberQuoteDropoff}：攤平成字串再讓 shipping 反解＝
 * 自己製造一個解析率問題）與<b>逐件尺寸重量</b>。取得三欄地址的唯一方式是收
 * {@code address_id} 並向 buyer-service 取回——那是 {@code shipping_addresses} 的原欄位，
 * 不經任何解析。三個買家面入口都這樣做：
 * <ul>
 *   <li>{@code CheckoutService.uberQuote}（{@code POST /app/checkout/uber_quote}）：
 *       {@link Probe#forQuote} ——只報價，不落狀態。</li>
 *   <li>{@code CheckoutService.preview}：{@link Probe#forCheckout}（accept=false）
 *       ——帶著買家手上那筆 quote 重驗並定價。</li>
 *   <li>{@code CheckoutService.confirm}：先 {@link Probe#forCheckout} 定價，
 *       建單成功後再 {@link Probe#accepting}（accept=true）把同一筆 CAS 成
 *       {@code CHECKOUT_ACCEPTED}（66 §9）。</li>
 * </ul>
 *
 * <p>仍然拿不到 probe、因此仍然拒絕的入口（<b>是正確行為不是缺口</b>）：
 * <ul>
 *   <li>{@code CheckoutValidator.validate} 的選項迴圈：靜態費率選項列表不承載即時報價，
 *       Uber 本來就不從那裡出（它沒有級距表）。</li>
 *   <li>{@code CheckoutService.shippingFee}（{@code POST /app/checkout/shipping_fee}）：
 *       那支端點算的是<b>級距費率表</b>的運費。Uber 的價格只有
 *       {@code /app/checkout/uber_quote} 出得來（會過期、要重報），
 *       讓它從費率表出等於把「會過期」這件事弄丟。</li>
 *   <li>{@code OrderService.createOrder} 兩個建單入口：不是買家結帳鏈，沒有報價 context。</li>
 * </ul>
 *
 * <h2>🔴 這道閘門擋不住什麼（誠實標明邊界）</h2>
 * <ul>
 *   <li>它的觀測窗只有<b>一個字串</b>（{@code ShippingMethod.value()}）。呼叫端若拿別的表示法
 *       （enum 名、未經 {@code ShippingMethod.fromValue} 正規化的輸入）遞進來，它看不到。
 *       本 repo 全線以 {@code .value()} 傳遞配送方式，這是它成立的前提。</li>
 *   <li>它擋不住「<b>新增的建單入口沒有呼叫它</b>」。那一面靠
 *       {@code UberCheckoutWiringTest} ／ {@code UberOrderCreationGuardWiringTest} 逐一枚舉
 *       建單入口來守，不是靠本類別。</li>
 *   <li>{@code shippingMethodValue == null} 對本閘門是 {@link Status#NOT_APPLICABLE}（放行）。
 *       null 由 {@link OrderPolicyResolver} 擋（{@code 9504}）——本閘門只答 Uber 這一題，
 *       不兼任配送方式的合法性驗證，兩個決定點不重疊。</li>
 *   <li>它擋不住「報價當下可用、建單當下已失效」的時間差。那一段由 shipping 的
 *       {@code accept}（CAS 標 {@code CHECKOUT_ACCEPTED}）守，不是由本類別守。</li>
 * </ul>
 */
@Component
public class UberOrderEligibilityGuard {

    /**
     * shipping 回得出來的理由碼，逐字抄自對端原始碼（<b>不是</b>憑印象寫的）：
     * {@code UberQuoteService.quote()} 四個、{@code UberPackingPlanner} 五個、
     * {@code ShippingGrpcServiceImpl.validateUberQuoteForCheckout} 一個、
     * {@code UberQuoteService.Revalidation} javadoc 明載的 price-changed 一個。
     *
     * <p>🔴 這些是<b>跨服務 wire 契約</b>：shipping 改字串、本檔沒跟上 ⇒ 落進
     * 「不認得的理由」⇒ 六項全拒。方向是安全的，但那代表買家看到一個沒有理由的拒絕，
     * 所以 {@code UberOrderEligibilityGuardTest} 用窮舉把這張表釘住。
     */
    public static final class ProviderReason {
        /** 供應商層總開關關閉（{@code uber.enabled=false}）。 */
        public static final String UBER_DISABLED = "UBER_DISABLED";
        /** 3003 該溫層沒有 enabled 的 {@code shipping_method_configs} 列。 */
        public static final String UBER_METHOD_NOT_ENABLED = "UBER_METHOD_NOT_ENABLED";
        /** 賣家沒有生效的 {@code uber_store_mapping.external_store_id}。 */
        public static final String UBER_STORE_NOT_MAPPED = "UBER_STORE_NOT_MAPPED";
        /** 沒有生效的分箱政策版本。 */
        public static final String PACKING_POLICY_MISSING = "PACKING_POLICY_MISSING";
        /** 分箱政策內容不合法（箱型表空／欄位缺）。 */
        public static final String PACKING_POLICY_INVALID = "PACKING_POLICY_INVALID";
        /** 件數欄位不合法。 */
        public static final String ITEM_QUANTITY_INVALID = "ITEM_QUANTITY_INVALID";
        /** 三邊和超限。 */
        public static final String ITEM_OVER_DIMENSION_SUM = "ITEM_OVER_DIMENSION_SUM";
        /** 最長邊超限。 */
        public static final String ITEM_OVER_LONGEST_SIDE = "ITEM_OVER_LONGEST_SIDE";
        /** 重量超限。 */
        public static final String ITEM_OVER_WEIGHT = "ITEM_OVER_WEIGHT";
        /** 地址無法結構化／驗證不過／無經緯度。 */
        public static final String ADDRESS_UNPARSEABLE = "ADDRESS_UNPARSEABLE";
        /** Create Quote（E1）失敗。 */
        public static final String QUOTE_FAILED = "QUOTE_FAILED";
        /** {@code accept=true} 的 CAS 未命中：已被換手／過期／已接受過。 */
        public static final String QUOTE_NOT_ACCEPTABLE = "QUOTE_NOT_ACCEPTABLE";
        /** 重驗後換了一張報價且金額不同（INV-U1：不得 capture，要使用者重新確認）。 */
        public static final String UBER_QUOTE_PRICE_CHANGED = "UBER_QUOTE_PRICE_CHANGED";

        /**
         * 🔴 order 側自己產生的理由：問不到 shipping（RPC 掛了／逾時／回非零 code）。
         *
         * <p>它<b>刻意不進任何 {@link Check} 的 {@link Check#providerReasons()}</b>——
         * 「我問不到」與「它說賣家沒開通」是兩件事，把前者歸到某一項會讓 log 指向
         * 一個沒有人查得動的方向。它落在「不認得的理由」分支，六項全拒。
         */
        public static final String ASSESSOR_UNAVAILABLE = "UBER_QUOTE_ASSESSOR_UNAVAILABLE";

        private ProviderReason() {
        }
    }

    /**
     * 每一道檢查的判定結果。
     *
     * <p>🔴 {@code blocking} 由每個常數<b>自己宣告</b>，沒有預設值：
     * 未來新增狀態時必須當場回答「它擋不擋」，不會因為漏寫而靜默變成放行。
     */
    public enum Status {
        /** 已檢查並通過——shipping 的判定涵蓋這一項且沒有指向它。 */
        PASSED(false, "已檢查並通過"),
        /** shipping 明確指出這一項不成立。 */
        DENIED_BY_PROVIDER(true, "shipping 判定這一項不成立"),
        /** 取不到證據（無 probe／無 assessor／RPC 失敗／理由碼不認得）⇒ 拒絕。 */
        DENIED_NO_EVIDENCE(true, "取不到證據 ⇒ 拒絕"),
        /** 不適用——本次配送方式不是 3003，本閘門不管這張訂單。 */
        NOT_APPLICABLE(false, "不適用（本次配送方式不是 3003）");

        private final boolean blocking;
        private final String label;

        Status(boolean blocking, String label) {
            this.blocking = blocking;
            this.label = label;
        }

        /** 這個狀態是否阻擋結帳。 */
        public boolean blocking() {
            return blocking;
        }

        public String label() {
            return label;
        }
    }

    /**
     * 六道前置檢查。
     *
     * <p>每個常數帶兩樣東西：{@code evidenceSource} 說「這一項的答案是誰給的」（已接線，
     * 不是指向未來的待辦），{@code providerReasons} 說「shipping 的哪些理由碼屬於這一項」。
     * 後者就是對照表本身——{@link #checkFor(String)} 直接走訪它，沒有第二份 switch。
     */
    public enum Check {
        /**
         * 供應商層總開關與 3003 的方法級開通。
         * 來源：{@code UberQuoteService.quote()} 的 {@code enabled} 旗標與
         * {@code shipping_method_configs} 查找（66 §3.4 ② 程式層）。
         */
        PROVIDER_DISABLED(
                "shipping UberQuoteService.quote()：uber.enabled 旗標 ＋ shipping_method_configs（66 §3.4 ②）",
                ProviderReason.UBER_DISABLED,
                ProviderReason.UBER_METHOD_NOT_ENABLED),
        /**
         * 這個賣家有沒有 Uber 取件點。
         * 來源：{@code UberStoreResolver.activeMapping()} ⇒ {@code uber_store_mapping.external_store_id}
         * （66 §3.3 ①：Create Quote 與 Create Delivery 兩處都要帶，且是 payload hash 的組成）。
         */
        SELLER_NOT_ENABLED(
                "shipping UberStoreResolver.activeMapping()：uber_store_mapping.external_store_id（66 §3.3 ①）",
                ProviderReason.UBER_STORE_NOT_MAPPED),
        /**
         * 有沒有一份可用的報價。
         * 來源：Create Quote（E1）的結果（66 §7.6 第 1 列：失敗 ⇒ 選項不可用，不建訂單）。
         */
        QUOTE_MISSING(
                "shipping UberApiClient.createQuote()（E1）的結果（66 §7.6 第 1 列）",
                ProviderReason.QUOTE_FAILED),
        /**
         * 報價是否還活著、還是同一個價。
         * 來源：{@code UberQuoteService.revalidate()}／{@code accept()} 的 CAS
         * （66 §7.3 終態 CONSUMED／EXPIRED／SUPERSEDED；§7.6 INV-U1 金額不同要重新確認）。
         */
        QUOTE_EXPIRED(
                "shipping UberQuoteService.revalidate()／accept() 的 CAS（66 §7.3／§7.6 INV-U1）",
                ProviderReason.QUOTE_NOT_ACCEPTABLE,
                ProviderReason.UBER_QUOTE_PRICE_CHANGED),
        /**
         * 取件與送達地址是否在服務範圍。
         * 來源：{@code UberQuoteService.resolveDropoff()}——地址結構化 ＋ buyer-service／TGOS
         * 驗證 ＋ 經緯度（缺一即 fail-closed）。
         */
        ADDRESS_UNSERVICEABLE(
                "shipping UberQuoteService.resolveDropoff()：地址結構化 ＋ TGOS 驗證 ＋ 經緯度",
                ProviderReason.ADDRESS_UNPARSEABLE),
        /**
         * 承載檢查（66 §7.4 道 2～7：重量／體積／逐 item 尺寸／件數／分箱政策）。
         * 來源：{@code UberPackingPlanner.estimate()} 與生效政策查找。
         */
        DELIVERY_DATA_INCOMPLETE(
                "shipping UberPackingPlanner.estimate() ＋ packing_policies 生效版本（66 §7.4 道 2～7）",
                ProviderReason.PACKING_POLICY_MISSING,
                ProviderReason.PACKING_POLICY_INVALID,
                ProviderReason.ITEM_QUANTITY_INVALID,
                ProviderReason.ITEM_OVER_DIMENSION_SUM,
                ProviderReason.ITEM_OVER_LONGEST_SIDE,
                ProviderReason.ITEM_OVER_WEIGHT);

        private final String evidenceSource;
        private final Set<String> providerReasons;

        Check(String evidenceSource, String... providerReasons) {
            this.evidenceSource = evidenceSource;
            this.providerReasons = Set.copyOf(new LinkedHashSet<>(Arrays.asList(providerReasons)));
        }

        /** 這一項的答案由誰給（已接線的實際來源）。 */
        public String evidenceSource() {
            return evidenceSource;
        }

        /** shipping 的哪些理由碼歸這一項。 */
        public Set<String> providerReasons() {
            return providerReasons;
        }
    }

    /**
     * 理由碼 → 檢查項。找不到回 {@code null}（＝不認得 ⇒ 六項全拒，見 {@link #evaluate}）。
     *
     * <p>走訪 {@link Check#values()} 而不是維護第二份 map：多一份就會有漂移。
     */
    public static Check checkFor(String providerReason) {
        if (providerReason == null || providerReason.isBlank()) {
            return null;
        }
        for (Check check : Check.values()) {
            if (check.providerReasons().contains(providerReason)) {
                return check;
            }
        }
        return null;
    }

    /**
     * {@link Check} 常數存在、但 {@link #evaluate} 沒有為它 put 任何 outcome 時，
     * {@link #decide} 補上的哨兵 evidence。
     *
     * <p>它是 {@code public} 的唯一理由：{@code UberOrderEligibilityGuardTest} 要能斷言
     * 「沒有任何一項落在這個哨兵上」。否則「漏接一項」與「該項已接線且拒絕」會產生
     * 完全相同的外部行為（都是拒絕），測試對前者零鑑別力。
     */
    public static final String UNWIRED_EVIDENCE = "本檢查沒有接線，fail-closed 視為未通過";

    /** 一件待報價的商品。欄位逐一對齊 {@code shipping.proto} 的 {@code UberQuoteItem}。 */
    public record Item(long skuId,
                       String name,
                       int quantity,
                       int lengthCm,
                       int widthCm,
                       int heightCm,
                       long weightGram,
                       long unitPrice) {
    }

    /**
     * 問 shipping 所需要的一切。
     *
     * <p>🔴 收件地址是<b>三欄</b>不是一個字串：{@code shipping.proto} 的 {@code UberQuoteDropoff}
     * 逐字寫明攤平成字串再讓 shipping 反解會製造解析率問題，而每一筆解不出來的就是一個
     * 看不到選項的買家。組不出這三欄的呼叫端<b>不要硬湊</b>——傳 {@code null} probe，
     * 讓閘門以「取不到證據」拒絕，那是誠實的答案。
     *
     * @param quoteId 買家手上已有的報價 id；空字串 ⇒ 取一筆新的（或沿用同 payload hash 的既有那筆）
     * @param accept  capture 成功後才可為 true（把報價 CAS 成 {@code CHECKOUT_ACCEPTED}）；
     *                為 true 時 {@code orderNo} 必填
     */
    public record Probe(long buyerId,
                        long sellerStoreId,
                        String deliveryTemp,
                        String receiverName,
                        String receiverPhone,
                        String county,
                        String district,
                        String detailAddress,
                        List<Item> items,
                        String quoteId,
                        boolean accept,
                        String orderNo) {

        public Probe {
            items = items == null ? List.of() : List.copyOf(items);
            quoteId = quoteId == null ? "" : quoteId;
            orderNo = orderNo == null ? "" : orderNo;
        }

        /** 結帳頁報價用：沒有既有 quote，也不落任何狀態。 */
        public static Probe forQuote(long buyerId, long sellerStoreId, String deliveryTemp,
                                     String receiverName, String receiverPhone,
                                     String county, String district, String detailAddress,
                                     List<Item> items) {
            return new Probe(buyerId, sellerStoreId, deliveryTemp,
                    receiverName, receiverPhone, county, district, detailAddress,
                    items, "", false, "");
        }

        /**
         * preview／confirm 的<b>定價</b>用：帶著買家手上那筆 quote 重驗，但<b>不落狀態</b>。
         *
         * <p>{@code quoteId} 可以是空字串（買家還沒報過價，或報價已被 SUPERSEDED）——
         * 那時 shipping 會重新取一筆，回來的 {@link Assessment#quoteId()} 才是該採用的那筆。
         * <b>不要假設送進去的與回來的是同一個 id</b>：66 §7 的決策樹允許重報。
         */
        public static Probe forCheckout(long buyerId, long sellerStoreId, String deliveryTemp,
                                        String receiverName, String receiverPhone,
                                        String county, String district, String detailAddress,
                                        List<Item> items, String quoteId) {
            return new Probe(buyerId, sellerStoreId, deliveryTemp,
                    receiverName, receiverPhone, county, district, detailAddress,
                    items, quoteId, false, "");
        }

        /**
         * confirm 建單成功後的<b>鎖定</b>用：把 {@code quoteId} CAS 成 {@code CHECKOUT_ACCEPTED}
         * 並掛上 {@code orderNo}（66 §9「確認 → 重驗 → CHECKOUT_ACCEPTED → 收款」）。
         *
         * <p>🔴 <b>沿用同一個 {@code base}，只換 quoteId 與 orderNo</b>：地址／品項若在這兩步之間
         * 被重新組一次，鎖到的會是「另一個 payload 的報價」，而 payload_hash 的比對就失去意義。
         *
         * @param quoteId 必須是<b>上一步重驗回來的那個</b>（{@link Assessment#quoteId()}），
         *                不是買家送上來的那個——重驗允許重報，兩者可能不同
         */
        public static Probe accepting(Probe base, String quoteId, String orderNo) {
            return new Probe(base.buyerId(), base.sellerStoreId(), base.deliveryTemp(),
                    base.receiverName(), base.receiverPhone(),
                    base.county(), base.district(), base.detailAddress(),
                    base.items(), quoteId, true, orderNo);
        }
    }

    /**
     * shipping 的裁決，轉成 order 側的中性形狀。
     *
     * @param eligible          true ⇒ 可以用 Uber 建這張單
     * @param unavailableReason {@code eligible=false} 時必填；值域見 {@link ProviderReason}
     * @param quoteId           可用時的報價 id（結帳頁要回給前端）
     * @param feeAmount         運費（元整數字串）
     */
    public record Assessment(boolean eligible,
                             String unavailableReason,
                             String quoteId,
                             String feeAmount,
                             String currency,
                             String expiresAt) {

        public static Assessment eligible(String quoteId, String feeAmount,
                                          String currency, String expiresAt) {
            return new Assessment(true, "", quoteId, feeAmount, currency, expiresAt);
        }

        public static Assessment unavailable(String reason) {
            return new Assessment(false, reason == null ? "" : reason, "", "", "", "");
        }
    }

    /**
     * 證據來源。唯一實作是 {@code ShippingUberQuoteAssessor}（打 shipping 的兩支 RPC）；
     * 測試以 fake 取代，而 fake 的鑑別力來自它<b>拒絕非預期的 probe</b>。
     */
    public interface QuoteAssessor {
        /** 🔴 不得拋例外表達業務不可用——那是 {@link Assessment#unavailable} 的工作。 */
        Assessment assess(Probe probe);
    }

    /**
     * 一次判定的輸入。
     *
     * <p>🔴 <b>沒有只收字串的多載</b>：每個呼叫端都必須明寫自己有沒有 probe
     * （{@link #of} 或 {@link #methodOnly}）。少了這個強制，新入口會沿用最方便的那個多載，
     * 而「沒有證據」就會變成一件沒有人在呼叫端看得見的事。
     */
    public record Context(String shippingMethodValue, Probe probe) {

        /** 有完整報價 context。 */
        public static Context of(String shippingMethodValue, Probe probe) {
            return new Context(shippingMethodValue, probe);
        }

        /**
         * 只有配送方式、沒有報價 context ⇒ 3003 一律拒絕。
         *
         * <p>用在結構上供不起 probe 的入口（{@code CheckoutService.shippingFee}、兩個
         * {@code OrderService.createOrder}），理由見類別 javadoc「誰供得起 probe」。
         */
        public static Context methodOnly(String shippingMethodValue) {
            return new Context(shippingMethodValue, null);
        }
    }

    /** 單一檢查的結果。{@code evidence} 是「為什麼是這個狀態」，會進 log 與錯誤訊息。 */
    public record CheckOutcome(Check check, Status status, String evidence) {
    }

    /**
     * 閘門的判定。
     *
     * @param applicable 本次配送方式是不是 3003；false 時 {@code allowed} 恆為 true（本閘門不管它）
     * @param allowed    是否放行——<b>沒有任何一項是 blocking 狀態</b>才為 true
     * @param outcomes   六項逐項結果，順序固定為 {@link Check#values()}
     * @param assessment shipping 的原始裁決；取不到證據時為 {@code null}
     */
    public record Decision(boolean applicable, boolean allowed,
                           List<CheckOutcome> outcomes, Assessment assessment) {

        /** 造成拒絕的項目（blocking 的那些），依 {@link Check#values()} 的順序。 */
        public List<Check> blockingChecks() {
            return outcomes.stream()
                    .filter(o -> o.status().blocking())
                    .map(CheckOutcome::check)
                    .toList();
        }

        /** 人可讀的拒絕原因，逐項列出，供 log 與錯誤訊息使用。 */
        public String denyReason() {
            return outcomes.stream()
                    .filter(o -> o.status().blocking())
                    .map(o -> o.check().name() + "(" + o.evidence() + ")")
                    .collect(Collectors.joining("; "));
        }

        /**
         * 可寫進 UI 的理由碼。
         *
         * <p>放行或不適用時為空字串；取不到證據時是 order 側的
         * {@link ProviderReason#ASSESSOR_UNAVAILABLE} 之類，否則就是 shipping 的原始理由碼
         * ——<b>不翻譯、不美化</b>，前端與維運查的是同一個字串。
         */
        public String providerReason() {
            return assessment == null ? "" : assessment.unavailableReason();
        }
    }

    private final QuoteAssessor assessor;

    /**
     * 🔴 無證據來源 ⇒ 對 3003 一律拒絕（fail-closed）。
     *
     * <p>保留它是為了讓「只關心非 Uber 行為」的測試不必端一個 assessor 進來。
     * 生產環境走下面帶 {@link Autowired} 的那支——Spring 有兩個建構子時只認被標註的那一個。
     */
    public UberOrderEligibilityGuard() {
        this(null);
    }

    @Autowired
    public UberOrderEligibilityGuard(QuoteAssessor assessor) {
        this.assessor = assessor;
    }

    /**
     * 判定這個配送方式現在能不能用。
     *
     * @param context 配送方式（{@code ShippingMethod.value()}）＋ 報價 context；
     *                非 3003（含 null）一律 {@link Status#NOT_APPLICABLE}，本閘門不介入
     */
    public Decision evaluate(Context context) {
        String shippingMethodValue = context == null ? null : context.shippingMethodValue();
        if (!ShippingMethod.UBER_INSTANT.value().equals(shippingMethodValue)) {
            return notApplicable();
        }

        Probe probe = context.probe();
        if (probe == null) {
            return deniedWithoutEvidence(
                    "呼叫端未提供報價 context（結構化收件地址與逐件尺寸），無從向 shipping 求證",
                    null);
        }
        if (assessor == null) {
            return deniedWithoutEvidence(
                    "本實例沒有 QuoteAssessor（以無參建構子建出），無從向 shipping 求證",
                    null);
        }

        Assessment assessment;
        try {
            assessment = assessor.assess(probe);
        } catch (RuntimeException e) {
            // 🔴 問不到就是拒絕。把例外吞成「可用」是本檔唯一不能犯的錯。
            return deniedWithoutEvidence(
                    "向 shipping 求證時失敗：" + e.getClass().getSimpleName() + " " + e.getMessage(),
                    Assessment.unavailable(ProviderReason.ASSESSOR_UNAVAILABLE));
        }
        if (assessment == null) {
            return deniedWithoutEvidence("shipping 回了 null 裁決",
                    Assessment.unavailable(ProviderReason.ASSESSOR_UNAVAILABLE));
        }

        if (assessment.eligible()) {
            EnumMap<Check, CheckOutcome> wired = new EnumMap<>(Check.class);
            for (Check check : Check.values()) {
                wired.put(check, new CheckOutcome(check, Status.PASSED,
                        "shipping 回報價可用（quoteId=" + assessment.quoteId() + "）；來源："
                                + check.evidenceSource()));
            }
            return decide(wired, assessment);
        }

        String reason = assessment.unavailableReason();
        Check culprit = checkFor(reason);
        if (culprit == null) {
            // 不認得的理由碼 ⇒ 不知道是哪一項壞了 ⇒ 六項全拒（絕不挑一項來擋）。
            return deniedWithoutEvidence(
                    "shipping 回了本服務不認得的理由碼 " + safeReason(reason) + "，無法歸屬到任何一項檢查",
                    assessment);
        }

        EnumMap<Check, CheckOutcome> wired = new EnumMap<>(Check.class);
        for (Check check : Check.values()) {
            if (check == culprit) {
                wired.put(check, new CheckOutcome(check, Status.DENIED_BY_PROVIDER,
                        "shipping 判定不可用：" + reason + "；來源：" + check.evidenceSource()));
            } else {
                wired.put(check, new CheckOutcome(check, Status.PASSED,
                        "shipping 的裁決（" + reason + "）未指向本項；來源：" + check.evidenceSource()));
            }
        }
        return decide(wired, assessment);
    }

    /**
     * 不可用就拋 {@code 9503}。
     *
     * <p>用 {@code 9503 FULFILLMENT_PROVIDER_NOT_ENABLED}（403「此配送服務尚未開放」）而不是
     * {@code 9510 SHIPPING_OPTION_CONFIG_MISSING}：後者的語意是「配置缺陷，有人要去補」，
     * 會讓維運去建一張不該建的 config 列（66 §3.4 逐字區分了這兩類）。
     * 也不是 {@code 4008 SHIPPING_METHOD_UNAVAILABLE}——那個碼吐不出「為什麼」。
     */
    public Decision requireEligible(Context context) {
        Decision decision = evaluate(context);
        if (decision.allowed()) {
            return decision;
        }
        throw new OrderBusinessException(GrpcResponseCode.FULFILLMENT_PROVIDER_NOT_ENABLED);
    }

    private static String safeReason(String reason) {
        return reason == null || reason.isBlank() ? "(空字串)" : reason;
    }

    private static Decision notApplicable() {
        List<CheckOutcome> outcomes = new ArrayList<>(Check.values().length);
        for (Check check : Check.values()) {
            outcomes.add(new CheckOutcome(check, Status.NOT_APPLICABLE, "本次配送方式不是 3003"));
        }
        return new Decision(false, true, List.copyOf(outcomes), null);
    }

    /** 六項一起落在「取不到證據」。 */
    private static Decision deniedWithoutEvidence(String evidence, Assessment assessment) {
        EnumMap<Check, CheckOutcome> wired = new EnumMap<>(Check.class);
        for (Check check : Check.values()) {
            wired.put(check, new CheckOutcome(check, Status.DENIED_NO_EVIDENCE, evidence));
        }
        return decide(wired, assessment);
    }

    /**
     * 把逐項結果收斂成一個判定。
     *
     * <p>🔴 走訪的是 {@link Check#values()} 而不是 map 的 entrySet：
     * <b>新增 Check 常數卻忘了接線 ⇒ 取不到 outcome ⇒ 當成拒絕</b>，不是靜默通過。
     * 反過來寫（只看已接線的項目）會讓「漏接一項」與「該項通過」產生同一個輸出。
     */
    private static Decision decide(EnumMap<Check, CheckOutcome> wired, Assessment assessment) {
        List<CheckOutcome> outcomes = new ArrayList<>(Check.values().length);
        for (Check check : Check.values()) {
            CheckOutcome outcome = wired.get(check);
            if (outcome == null) {
                outcome = new CheckOutcome(check, Status.DENIED_NO_EVIDENCE, UNWIRED_EVIDENCE);
            }
            outcomes.add(outcome);
        }
        boolean allowed = outcomes.stream().noneMatch(o -> o.status().blocking());
        return new Decision(true, allowed, List.copyOf(outcomes), assessment);
    }
}
