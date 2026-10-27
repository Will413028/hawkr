package io.github.will413028.hawkr.shipping.provider.uber;

import io.github.will413028.hawkr.shipping.entity.PackingPolicyEntity;
import org.junit.jupiter.api.DisplayName;
import org.junit.jupiter.api.Nested;
import org.junit.jupiter.api.Test;

import java.math.BigDecimal;
import java.util.List;

import static org.assertj.core.api.Assertions.assertThat;

/**
 * 分箱估計（66 §5.1.8）。
 *
 * <h2>🔴 它是<b>下界估計</b>，測試也只驗得起「估計的性質」</h2>
 * 不要在這裡斷言「這幾件東西剛好裝進這個尺寸的箱子」——那是三維裝箱，
 * 而本類別刻意不做（{@code void_factor} 是估計不是事實）。
 * 能驗的是：分箱規則有沒有被套用、上限有沒有擋住、包材重量有沒有算進去。
 */
@DisplayName("Uber 分箱估計")
class UberPackingPlannerTest {

    /** 不限任何上限（{@code 0} 在 {@code shipping_method_configs} 的慣例是「不限」）。 */
    private static final UberPackingPlanner.Limits UNLIMITED =
            new UberPackingPlanner.Limits(0, 0, 0, 0);

    private static PackingPolicyEntity policy(int maxItems, int packagingWeight, String voidFactor) {
        PackingPolicyEntity p = new PackingPolicyEntity();
        p.setVersion(1);
        p.setMaxItemsPerPackage(maxItems);
        p.setPackagingWeightGram(packagingWeight);
        p.setVoidFactor(new BigDecimal(voidFactor));
        return p;
    }

    private static UberPackingPlanner.Item item(String sku, int qty, int side, int weight) {
        return new UberPackingPlanner.Item(sku, "商品" + sku, qty, side, side, side, weight, 100L);
    }

    @Nested
    @DisplayName("分箱規則")
    class Splitting {

        @Test
        @DisplayName("件數超過上限 ⇒ 開新箱")
        void splitsByItemCount() {
            var plan = UberPackingPlanner.estimate(
                    List.of(item("A", 5, 10, 100)), policy(2, 150, "1.25"), UNLIMITED);
            assertThat(plan.usable()).isTrue();
            assertThat(plan.packages()).hasSize(3);   // 2 + 2 + 1
            assertThat(plan.packages().get(0).getSeq()).isEqualTo(1);
            assertThat(plan.packages().get(2).getSeq()).isEqualTo(3);
        }

        @Test
        @DisplayName("重量超過上限 ⇒ 開新箱（含包材）")
        void splitsByWeight() {
            // 包材 150g ＋ 每件 500g；上限 1200g ⇒ 一箱最多兩件（150+500+500=1150 ≤ 1200）
            var limits = new UberPackingPlanner.Limits(0, 0, 1200, 0);
            var plan = UberPackingPlanner.estimate(
                    List.of(item("A", 4, 10, 500)), policy(99, 150, "1.25"), limits);
            assertThat(plan.usable()).isTrue();
            assertThat(plan.packages()).hasSize(2);
            assertThat(plan.packages().get(0).getWeightGram()).isEqualTo(1150);
        }

        @Test
        @DisplayName("🔴 包材重量計入每一箱，不是只算第一箱")
        void packagingWeightCountedPerBox() {
            var plan = UberPackingPlanner.estimate(
                    List.of(item("A", 2, 10, 100)), policy(1, 150, "1.25"), UNLIMITED);
            assertThat(plan.packages()).hasSize(2);
            assertThat(plan.packages()).allSatisfy(
                    p -> assertThat(p.getWeightGram()).isEqualTo(250));
        }

        @Test
        @DisplayName("🔴 體積**不**參與分箱（估計值不該決定對外可見的箱數）")
        void volumeDoesNotSplit() {
            // 體積上限設成一個很小的值：若它參與分箱，箱數會變多。
            var limits = new UberPackingPlanner.Limits(0, 0, 0, 1);
            var plan = UberPackingPlanner.estimate(
                    List.of(item("A", 3, 10, 100)), policy(99, 150, "1.25"), limits);
            assertThat(plan.packages()).hasSize(1);
        }
    }

    @Nested
    @DisplayName("🔴 單件就超過上限 ⇒ 整個不可用（分箱救不了）")
    class SingleItemRejection {

        @Test
        @DisplayName("材積總和超標")
        void overDimensionSum() {
            var limits = new UberPackingPlanner.Limits(20, 0, 0, 0);
            var plan = UberPackingPlanner.estimate(
                    List.of(item("A", 1, 10, 100)), policy(99, 150, "1.25"), limits);   // 10+10+10=30 > 20
            assertThat(plan.usable()).isFalse();
            assertThat(plan.rejectReason()).isEqualTo("ITEM_OVER_DIMENSION_SUM");
        }

        @Test
        @DisplayName("最長邊超標")
        void overLongestSide() {
            var limits = new UberPackingPlanner.Limits(0, 5, 0, 0);
            var plan = UberPackingPlanner.estimate(
                    List.of(item("A", 1, 10, 100)), policy(99, 150, "1.25"), limits);
            assertThat(plan.rejectReason()).isEqualTo("ITEM_OVER_LONGEST_SIDE");
        }

        @Test
        @DisplayName("單件＋包材就超重")
        void overWeight() {
            var limits = new UberPackingPlanner.Limits(0, 0, 200, 0);
            var plan = UberPackingPlanner.estimate(
                    List.of(item("A", 1, 10, 100)), policy(99, 150, "1.25"), limits);   // 100+150=250 > 200
            assertThat(plan.rejectReason()).isEqualTo("ITEM_OVER_WEIGHT");
        }
    }

    @Nested
    @DisplayName("fail-closed 與邊界")
    class Guards {

        @Test
        @DisplayName("空購物車 ⇒ 不可用")
        void emptyCart() {
            var plan = UberPackingPlanner.estimate(List.of(), policy(99, 150, "1.25"), UNLIMITED);
            assertThat(plan.rejectReason()).isEqualTo("EMPTY_CART");
        }

        @Test
        @DisplayName("🔴 政策的 maxItemsPerPackage <= 0 ⇒ 不可用（否則迴圈不會前進）")
        void invalidPolicyFailsClosed() {
            var plan = UberPackingPlanner.estimate(
                    List.of(item("A", 1, 10, 100)), policy(0, 150, "1.25"), UNLIMITED);
            assertThat(plan.rejectReason()).isEqualTo("PACKING_POLICY_INVALID");
        }

        @Test
        @DisplayName("數量 <= 0 ⇒ 不可用（上游算錯時不要靜默產出零箱）")
        void nonPositiveQuantity() {
            var plan = UberPackingPlanner.estimate(
                    List.of(item("A", 0, 10, 100)), policy(99, 150, "1.25"), UNLIMITED);
            assertThat(plan.rejectReason()).isEqualTo("ITEM_QUANTITY_INVALID");
        }

        @Test
        @DisplayName("🔴 上限為 0 ＝「不限」，不是「全部擋掉」")
        void zeroMeansUnlimited() {
            var plan = UberPackingPlanner.estimate(
                    List.of(item("A", 1, 999, 999_000)), policy(99, 150, "1.25"), UNLIMITED);
            assertThat(plan.usable()).isTrue();
        }
    }

    @Nested
    @DisplayName("箱子的三邊")
    class BoxDimensions {

        @Test
        @DisplayName("🔴 最長邊不小於箱內最長的那一件（那一格是事實不是估計）")
        void longestSideIsFactual() {
            var plan = UberPackingPlanner.estimate(
                    List.of(item("A", 1, 40, 100)), policy(99, 150, "1.00"), UNLIMITED);
            var box = plan.packages().get(0);
            assertThat(Math.max(box.getLengthCm(), Math.max(box.getWidthCm(), box.getHeightCm())))
                    .isGreaterThanOrEqualTo(40);
        }

        @Test
        @DisplayName("void_factor 放大體積（1.00 與 2.00 算出的等效邊長不同）")
        void voidFactorInflates() {
            var tight = UberPackingPlanner.estimate(
                    List.of(item("A", 1, 10, 100)), policy(99, 150, "1.00"), UNLIMITED);
            var loose = UberPackingPlanner.estimate(
                    List.of(item("A", 1, 10, 100)), policy(99, 150, "2.00"), UNLIMITED);
            assertThat(loose.packages().get(0).getWidthCm())
                    .isGreaterThan(tight.packages().get(0).getWidthCm());
        }

        @Test
        @DisplayName("🔴 sizeClass 留 null 是決定不是遺漏（Uber size 值域未查證，送編出來的值會被判 400）")
        void sizeClassIsDeliberatelyNull() {
            var plan = UberPackingPlanner.estimate(
                    List.of(item("A", 1, 10, 100)), policy(99, 150, "1.25"), UNLIMITED);
            assertThat(plan.packages().get(0).getSizeClass()).isNull();
        }
    }
}
