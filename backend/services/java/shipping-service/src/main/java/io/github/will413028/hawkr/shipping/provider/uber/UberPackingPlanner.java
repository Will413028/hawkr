package io.github.will413028.hawkr.shipping.provider.uber;

import io.github.will413028.hawkr.shipping.entity.PackagePlanEntity;
import io.github.will413028.hawkr.shipping.entity.PackingPolicyEntity;

import java.math.BigDecimal;
import java.math.RoundingMode;
import java.util.ArrayList;
import java.util.List;

/**
 * 分箱（66 §5.1.8 的 {@code package_plan} 產出者）。
 *
 * <h2>🔴 它算的是<b>下界估計</b>，不是真正的裝箱</h2>
 * 66 §5.1.8 逐字「{@code void_factor} 是<b>估計不是事實</b> ⇒ 體積檢查是『下界估計 × 係數』
 * ⇒ §7.4 道 3 的文案是『超過限制』而不是『箱子放不下』」。
 * 本類別忠實實作那句話：把品項體積加總後乘上係數，<b>不</b>做三維裝箱。
 *
 * <p>⚠️ 因此它的輸出不可以被當成「這些東西一定塞得進這個箱子」。
 * 命名刻意用 {@code estimate} 而不是 {@code pack}：呼叫端讀到的是估計。
 *
 * <h2>分箱規則（只有兩條，而且都來自政策欄位）</h2>
 * <ol>
 *   <li>件數：一箱最多 {@code max_items_per_package} 件</li>
 *   <li>重量：一箱（含包材）不得超過該配送方式的 {@code max_weight_grams}</li>
 * </ol>
 * 🔴 <b>沒有第三條「依體積分箱」</b>：體積是估計值，拿估計值決定要開幾個箱子，
 * 會讓「箱數」這個對外可見的數字建立在一個沒有事實基礎的計算上。
 * 體積只用在<b>拒絕</b>（超過上限就不可用），不用在分箱。
 *
 * <h2>🔴 單件就超過上限 ⇒ 直接不可用，不是硬塞進一箱</h2>
 * 一件商品自己就超重／超材積時，分箱再多也解決不了。
 * 這種情況回傳空的 plan 並由呼叫端把 Uber 選項標成不可用（66 §7.4 道 3／4／5）。
 */
public final class UberPackingPlanner {

    private UberPackingPlanner() {
    }

    /**
     * 一件待裝箱的商品。
     *
     * @param skuId 進 {@code manifest_items[].external_id}，也是 payload_hash 的排序鍵
     * @param quantity 件數（&gt; 0）
     */
    public record Item(String skuId,
                       String name,
                       int quantity,
                       int lengthCm,
                       int widthCm,
                       int heightCm,
                       int weightGram,
                       long unitPrice) {
    }

    /** 該配送方式的承載上限（來自 {@code shipping_method_configs} 的 3003 列）。 */
    public record Limits(long maxDimensionSumCm,
                         long maxSingleSideCm,
                         long maxWeightGram,
                         long maxVolumeCm3) {

        /** {@code 0} 在本表的慣例是「不限」（沿用 {@code shipping_method_configs} 既有慣例）。 */
        static boolean unlimited(long v) {
            return v <= 0L;
        }
    }

    /**
     * 分箱結果。
     *
     * @param packages 空 list ⇒ <b>不可用</b>，理由在 {@code rejectReason}
     * @param rejectReason {@code null} ⇒ 可用
     */
    public record Plan(List<PackagePlanEntity> packages, String rejectReason) {

        public boolean usable() {
            return rejectReason == null && !packages.isEmpty();
        }
    }

    /**
     * @param policy 生效中的分箱政策（{@code effective_from <= now} 中 version 最大者）
     * @return 🔴 回傳的 {@link PackagePlanEntity} <b>尚未填 {@code id} 與 {@code quoteId}</b>
     *         ——那兩個值要等 Uber 回了 {@code quote_id} 才知道，由寫入端補
     */
    public static Plan estimate(List<Item> items, PackingPolicyEntity policy, Limits limits) {
        if (items == null || items.isEmpty()) {
            return new Plan(List.of(), "EMPTY_CART");
        }
        int packagingWeight = policy.getPackagingWeightGram() == null ? 0 : policy.getPackagingWeightGram();
        int maxItems = policy.getMaxItemsPerPackage() == null ? 1 : policy.getMaxItemsPerPackage();
        if (maxItems <= 0) {
            // 政策值壞掉時 fail-closed：maxItems <= 0 會讓下面的迴圈永遠開新箱子（或不前進）。
            return new Plan(List.of(), "PACKING_POLICY_INVALID");
        }
        BigDecimal voidFactor = policy.getVoidFactor() == null ? BigDecimal.ONE : policy.getVoidFactor();

        // ① 單件檢查：一件自己就超過上限 ⇒ 分箱救不了
        for (Item item : items) {
            String single = rejectSingle(item, limits, packagingWeight);
            if (single != null) {
                return new Plan(List.of(), single);
            }
        }

        // ② 依件數與重量切箱
        List<PackagePlanEntity> out = new ArrayList<>();
        int seq = 1;
        int countInBox = 0;
        long weightInBox = packagingWeight;
        long volumeInBox = 0L;
        int longestSideInBox = 0;
        for (Item item : items) {
            for (int i = 0; i < item.quantity(); i++) {
                boolean overCount = countInBox + 1 > maxItems;
                boolean overWeight = !Limits.unlimited(limits.maxWeightGram())
                        && weightInBox + item.weightGram() > limits.maxWeightGram();
                if (countInBox > 0 && (overCount || overWeight)) {
                    out.add(box(seq++, volumeInBox, longestSideInBox, weightInBox, voidFactor));
                    countInBox = 0;
                    weightInBox = packagingWeight;
                    volumeInBox = 0L;
                    longestSideInBox = 0;
                }
                countInBox++;
                weightInBox += item.weightGram();
                volumeInBox += (long) item.lengthCm() * item.widthCm() * item.heightCm();
                longestSideInBox = Math.max(longestSideInBox, longestSide(item));
            }
        }
        if (countInBox > 0) {
            out.add(box(seq, volumeInBox, longestSideInBox, weightInBox, voidFactor));
        }
        return new Plan(out, null);
    }

    /** 單件就超過上限的三種情形。回 {@code null} 表示這一件沒問題。 */
    private static String rejectSingle(Item item, Limits limits, int packagingWeight) {
        if (item.quantity() <= 0) {
            return "ITEM_QUANTITY_INVALID";
        }
        long dimensionSum = (long) item.lengthCm() + item.widthCm() + item.heightCm();
        if (!Limits.unlimited(limits.maxDimensionSumCm()) && dimensionSum > limits.maxDimensionSumCm()) {
            return "ITEM_OVER_DIMENSION_SUM";
        }
        if (!Limits.unlimited(limits.maxSingleSideCm()) && longestSide(item) > limits.maxSingleSideCm()) {
            return "ITEM_OVER_LONGEST_SIDE";
        }
        if (!Limits.unlimited(limits.maxWeightGram())
                && (long) item.weightGram() + packagingWeight > limits.maxWeightGram()) {
            return "ITEM_OVER_WEIGHT";
        }
        return null;
    }

    private static int longestSide(Item item) {
        return Math.max(item.lengthCm(), Math.max(item.widthCm(), item.heightCm()));
    }

    /**
     * 由「品項體積加總 × void_factor」反推一個<b>等效立方體</b>的三邊。
     *
     * <p>🔴 這三個數字是<b>估計的表示法</b>，不是真的量出來的箱子。取立方根的理由是它是
     * 「給定體積下，三邊之和最小」的形狀 ⇒ 對材積上限最寬鬆 ⇒ 不會因為我方挑了一個
     * 細長的表示法而誤判成超材積。⚠️ 最長邊另外用<b>實際</b>最長的那一件覆蓋，
     * 因為那一格是事實：箱子再怎麼裝，也不可能比裡面最長的那件短。
     */
    private static PackagePlanEntity box(int seq, long rawVolume, int longestSide,
                                         long weightGram, BigDecimal voidFactor) {
        BigDecimal inflated = BigDecimal.valueOf(rawVolume).multiply(voidFactor);
        long volume = inflated.setScale(0, RoundingMode.CEILING).longValueExact();
        int side = (int) Math.max(1L, Math.round(Math.cbrt((double) volume)));
        PackagePlanEntity e = new PackagePlanEntity();
        e.setSeq(seq);
        e.setLengthCm(Math.max(side, longestSide));
        e.setWidthCm(side);
        e.setHeightCm(side);
        e.setWeightGram((int) weightGram);
        // 🔴 sizeClass 刻意留 null，而且這是**決定**不是遺漏。
        //
        // 66 N4／U-K 說 Uber 的 `size` 是 string enum，但**沒有列出它的值域**，
        // 本 repo 的官方總覽（docs/第三方整合/Uber Direct API 總覽.md §2.2）也只載
        // `manifest_items` 必填、未載其子欄位。
        //
        // ⇒ 在 sandbox 坐實值域之前，這裡沒有任何值是我「查到」的，只有我「想得出來」的。
        //   而 UberApiClient.manifestItems() 對 null 的處置是**不送這個欄位**——
        //   少送一個可能是選填的欄位，比送一個我編出來的 enum 值安全：
        //   後者會被 Uber 判成 400，而錯誤訊息會指向「品項格式不對」。
        //
        // ⚠️ 這一格登記為 Enablement Blocker。坐實之後要改的是**這一行**與
        //   PackagePlanEntity.sizeClass 的 DB CHECK，不是散在別處的判斷。
        e.setSizeClass(null);
        return e;
    }
}
