package io.github.will413028.hawkr.product.converter;

import io.github.will413028.hawkr.product.dto.request.ProductSkuDTO;
import io.github.will413028.hawkr.product.dto.request.UpdateProductDTO;
import io.github.will413028.hawkr.product.grpc.ProductSku;
import io.github.will413028.hawkr.product.grpc.UpdateProductRequest;
import org.junit.jupiter.api.DisplayName;
import org.junit.jupiter.api.Test;

import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.junit.jupiter.api.Assertions.assertNull;

/**
 * 庫存的 0 是一個**真實的值**（售完），不是「沒填」。
 *
 * 失效形狀（與 #2480 認證標章清不掉同一個形狀、同一支檔案）：本類原本用 {@code parseInt}
 * 把 "0" 當成「未提供」回 null，而 {@code ProductSkuConverter.updateEntity} 對 null 的語意
 * 是「跳過不更新」⇒ **賣家把 SKU 庫存改成 0（售完）永遠不會生效**：HTTP 200、code 0、
 * 稽核記 SUCCEEDED、gateway 的 diff 甚至會謊報「庫存數量 30 件 → 0 件」，而 DB 原封不動。
 *
 * <p>影響的是所有賣家而不只代行者：賣家自助端與代行端走同一支 RPC。而「下架賣不掉的東西」
 * 是賣家對買家最基本的承諾之一 —— 賣光了還能被下單，後果落在買家與出貨端。
 *
 * <p>這幾條守的是「0 要能穿過轉換層」這件事本身。沒有它們，把 {@code stockQty} 改回走
 * {@code parseIntTreatingZeroAsAbsent} 不會讓任何東西轉紅 —— 那正是這個 bug 活到今天的原因。
 */
class ProductProtoConverterStockZeroTest {

    private final ProductProtoConverter converter = new ProductProtoConverter();

    private static UpdateProductRequest requestWithSkuStock(String stockQty) {
        return UpdateProductRequest.newBuilder()
                .setProductId("866542646616428481")
                .setStoreId(2685914682L)
                .addSkus(ProductSku.newBuilder()
                        .setId("310")
                        .setSkuName("一箱")
                        .setUnit("3301")
                        .setPrice("500")
                        .setStockQty(stockQty)
                        .build())
                .build();
    }

    private static ProductSkuDTO onlySku(UpdateProductDTO dto) {
        assertEquals(1, dto.getSkus().size(), "測試前提：只有一個 SKU");
        return dto.getSkus().get(0);
    }

    @Test
    @DisplayName("stockQty=\"0\" → DTO 給 0（售完真的送得下去），不是 null")
    void zeroStockSurvivesConversion() {
        ProductSkuDTO sku = onlySku(converter.toUpdateDTO(requestWithSkuStock("0")));
        assertEquals(
                0,
                sku.getStockQty(),
                "0 被轉成 null 的話，updateEntity 會跳過 ⇒ 賣家無法把 SKU 設為售完");
    }

    @Test
    @DisplayName("stockQty=\"\"（未提供）→ null，維持 partial-update 語意")
    void absentStockStaysNull() {
        // 這一條與上一條是一組：沒有它，把 "0"→0 修好的同時可能把「沒填」也變成 0，
        // 那會讓每一次不帶庫存的部分更新都把庫存洗成 0 —— 比原本的 bug 更糟。
        assertNull(onlySku(converter.toUpdateDTO(requestWithSkuStock(""))).getStockQty());
    }

    @Test
    @DisplayName("一般正值照常轉換")
    void positiveStockConverts() {
        assertEquals(30, onlySku(converter.toUpdateDTO(requestWithSkuStock("30"))).getStockQty());
    }

    @Test
    @DisplayName("非數字字串 → null，不拋例外（沿用 parseIntOrNull 既有約定）")
    void garbageStockBecomesNull() {
        assertNull(onlySku(converter.toUpdateDTO(requestWithSkuStock("abc"))).getStockQty());
    }
}
