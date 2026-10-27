package io.github.will413028.hawkr.shipping.provider.uber;

import io.github.will413028.hawkr.shipping.provider.uber.UberApiModels.ManifestItem;
import io.github.will413028.hawkr.shipping.provider.uber.UberApiModels.UberWaypoint;
import org.junit.jupiter.api.DisplayName;
import org.junit.jupiter.api.Test;

import java.util.List;

import static org.assertj.core.api.Assertions.assertThat;

/**
 * {@code payload_hash}（66 §5.1.4）。
 *
 * <h2>🔴 這份測試的形狀：每一條都是「改了 X，hash 該不該變」</h2>
 * 一個 hash 函式的錯誤只有兩種方向，而它們的後果相反：
 * <ul>
 *   <li><b>該變而沒變</b> ⇒ 用舊價建新購物車的單（66 A9）——這是會賠錢的那一種</li>
 *   <li><b>不該變而變了</b> ⇒ 每次微調都重打 E1，而 E1 的 token 有 100 req/hr 上限</li>
 * </ul>
 * ⇒ 兩個方向都要有測試，只驗其中一邊會讓另一邊完全無人看守。
 */
@DisplayName("Uber 報價 payload_hash")
class UberQuotePayloadHashTest {

    private static final UberWaypoint DROPOFF = new UberWaypoint(
            "{\"street\":\"忠孝東路四段1號\",\"city\":\"大安區\",\"state\":\"台北市\",\"country\":\"TW\"}",
            "王小明", "0912345678", 25.0418, 121.5654, "5樓");

    private static final List<ManifestItem> ITEMS = List.of(
            new ManifestItem("蘋果", 2, null, 100L, "sku-A"),
            new ManifestItem("香蕉", 1, null, 50L, "sku-B"));

    private static String base() {
        return UberQuotePayloadHash.of("store-1", DROPOFF, ITEMS, 1);
    }

    @Test
    @DisplayName("輸出是 64 個 hex 字元（對應 payload_hash CHAR(64)）")
    void shapeIs64Hex() {
        assertThat(base()).hasSize(64).matches("[0-9a-f]{64}");
    }

    @Test
    @DisplayName("同樣的輸入 ⇒ 同樣的輸出")
    void deterministic() {
        assertThat(base()).isEqualTo(base());
    }

    // ==================== 該變 ====================

    @Test
    @DisplayName("🔴 換取件門市 ⇒ hash 必須變（66 A9：否則用舊報價建新門市的單）")
    void storeChangeMustChangeHash() {
        assertThat(UberQuotePayloadHash.of("store-2", DROPOFF, ITEMS, 1)).isNotEqualTo(base());
    }

    @Test
    @DisplayName("換收件地址 ⇒ hash 變")
    void addressChangeMustChangeHash() {
        UberWaypoint other = new UberWaypoint(
                "{\"street\":\"信義路五段1號\",\"city\":\"信義區\",\"state\":\"台北市\",\"country\":\"TW\"}",
                "王小明", "0912345678", 25.0418, 121.5654, "5樓");
        assertThat(UberQuotePayloadHash.of("store-1", other, ITEMS, 1)).isNotEqualTo(base());
    }

    @Test
    @DisplayName("換經緯度 ⇒ hash 變（同一個地址字串也可能地理編碼到不同點）")
    void coordinateChangeMustChangeHash() {
        UberWaypoint moved = new UberWaypoint(DROPOFF.structuredAddressJson(), DROPOFF.name(),
                DROPOFF.phone(), 24.0, 121.5654, DROPOFF.notes());
        assertThat(UberQuotePayloadHash.of("store-1", moved, ITEMS, 1)).isNotEqualTo(base());
    }

    @Test
    @DisplayName("改數量 ⇒ hash 變")
    void quantityChangeMustChangeHash() {
        List<ManifestItem> more = List.of(
                new ManifestItem("蘋果", 3, null, 100L, "sku-A"),
                new ManifestItem("香蕉", 1, null, 50L, "sku-B"));
        assertThat(UberQuotePayloadHash.of("store-1", DROPOFF, more, 1)).isNotEqualTo(base());
    }

    @Test
    @DisplayName("換 SKU ⇒ hash 變")
    void skuChangeMustChangeHash() {
        List<ManifestItem> other = List.of(
                new ManifestItem("蘋果", 2, null, 100L, "sku-A"),
                new ManifestItem("芭樂", 1, null, 50L, "sku-C"));
        assertThat(UberQuotePayloadHash.of("store-1", DROPOFF, other, 1)).isNotEqualTo(base());
    }

    @Test
    @DisplayName("換分箱政策版本 ⇒ hash 變（同一車在新政策下可能分成不同箱數）")
    void policyVersionChangeMustChangeHash() {
        assertThat(UberQuotePayloadHash.of("store-1", DROPOFF, ITEMS, 2)).isNotEqualTo(base());
    }

    // ==================== 不該變 ====================

    @Test
    @DisplayName("🔴 品項順序不影響（購物車重排不是換了一車）")
    void itemOrderDoesNotMatter() {
        List<ManifestItem> reversed = List.of(ITEMS.get(1), ITEMS.get(0));
        assertThat(UberQuotePayloadHash.of("store-1", DROPOFF, reversed, 1)).isEqualTo(base());
    }

    @Test
    @DisplayName("🔴 改收件人姓名／電話／樓層備註 ⇒ hash 不變（它們不影響運費，變了就是白重報）")
    void contactFieldsDoNotChangeHash() {
        UberWaypoint renamed = new UberWaypoint(DROPOFF.structuredAddressJson(), "李大華",
                "0987654321", DROPOFF.latitude(), DROPOFF.longitude(), "12樓");
        assertThat(UberQuotePayloadHash.of("store-1", renamed, ITEMS, 1)).isEqualTo(base());
    }

    @Test
    @DisplayName("null 輸入不拋例外（報價前置失敗時呼叫端已經攔下，這裡只要不炸）")
    void nullsAreTolerated() {
        assertThat(UberQuotePayloadHash.of(null, null, null, null)).hasSize(64);
    }
}
