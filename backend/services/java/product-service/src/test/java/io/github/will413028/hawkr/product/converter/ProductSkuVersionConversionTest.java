package io.github.will413028.hawkr.product.converter;

import io.github.will413028.hawkr.product.dto.request.ProductSkuDTO;
import io.github.will413028.hawkr.product.dto.request.UpdateProductDTO;
import io.github.will413028.hawkr.product.dto.response.ProductDetailDTO;
import io.github.will413028.hawkr.product.entity.ProductSkuEntity;
import io.github.will413028.hawkr.product.grpc.ProductDetail;
import io.github.will413028.hawkr.product.grpc.ProductSku;
import io.github.will413028.hawkr.product.grpc.UpdateProductRequest;
import org.junit.jupiter.api.DisplayName;
import org.junit.jupiter.api.Test;

import java.util.List;

import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.junit.jupiter.api.Assertions.assertNull;

/**
 * SKU 樂觀鎖 token 的往返（#2545）。
 *
 * <p>樂觀鎖是一個**往返協定**：server 下發 {@code version}、client 下次寫回時帶
 * {@code expectedVersion}。轉換層漏掉任一個方向，DB 欄位與 CAS 邏輯都還在，鎖卻完全不生效，
 * 而且**沒有任何東西會轉紅**——那正是同一支檔案裡 {@code stockQty="0"}（賣家設售完不生效）
 * 與 {@code certifications}（認證清不掉）兩個 bug 活到被實測抓出來為止的原因。
 *
 * <p>失效形狀具體是：下發方向漏了 ⇒ client 手上永遠是空字串 ⇒ 一律不比對；
 * 上行方向漏了 ⇒ client 送了也到不了 service 層。兩者都是 HTTP 200、行為與沒有鎖時一模一樣。
 */
class ProductSkuVersionConversionTest {

    private final ProductProtoConverter protoConverter = new ProductProtoConverter();
    private final ProductSkuConverter skuConverter = new ProductSkuConverter();

    // ── Entity → DTO（下發方向的第一段）────────────────────────────────────

    @Test
    @DisplayName("Entity.version → DTO.version：不下發，client 就沒有東西可以在下次寫回時回送")
    void entityVersionFlowsIntoDto() {
        ProductSkuEntity entity = new ProductSkuEntity();
        entity.setId(310L);
        entity.setSkuName("一箱");
        entity.setVersion(7L);

        assertEquals("7", skuConverter.toDTO(entity).getVersion());
    }

    @Test
    @DisplayName("Entity.version 為 null（migration 之前的舊物件）→ DTO 給 null，不得變成字面 \"null\"")
    void nullEntityVersionStaysNull() {
        ProductSkuEntity entity = new ProductSkuEntity();
        entity.setId(310L);
        entity.setSkuName("一箱");

        assertNull(skuConverter.toDTO(entity).getVersion());
    }

    // ── DTO → Proto（下發方向的第二段）────────────────────────────────────

    @Test
    @DisplayName("DTO.version → proto version：Gateway 要靠它才有 skus[].version 可以吐給前端")
    void dtoVersionFlowsIntoProto() {
        ProductDetailDTO detail = new ProductDetailDTO();
        detail.setId("866542646616428481");
        detail.setSkus(List.of(ProductSkuDTO.builder().id("310").skuName("一箱").version("7").build()));

        ProductDetail proto = protoConverter.toProductDetailProto(detail);
        assertEquals(1, proto.getSkusCount(), "測試前提：只有一個 SKU");
        assertEquals("7", proto.getSkus(0).getVersion());
    }

    // ── Proto → DTO（上行方向）────────────────────────────────────────────

    @Test
    @DisplayName("proto expected_version → DTO.expectedVersion：漏了的話 client 送了也到不了 CAS")
    void protoExpectedVersionFlowsIntoDto() {
        UpdateProductDTO dto = protoConverter.toUpdateDTO(requestWithSkuExpectedVersion("5"));
        assertEquals("5", onlySku(dto).getExpectedVersion());
    }

    @Test
    @DisplayName("proto expected_version 為空字串（舊 client）→ DTO 給 null，不得讓 parseLong 炸成 400")
    void emptyExpectedVersionBecomesNull() {
        UpdateProductDTO dto = protoConverter.toUpdateDTO(requestWithSkuExpectedVersion(""));
        assertNull(onlySku(dto).getExpectedVersion(),
                "空字串 = 沒帶 = 不比對；轉成空字串會讓 applySkuExpectedVersion 走進 parseLong");
    }

    private static UpdateProductRequest requestWithSkuExpectedVersion(String expectedVersion) {
        return UpdateProductRequest.newBuilder()
                .setProductId("866542646616428481")
                .setStoreId(2685914682L)
                .addSkus(ProductSku.newBuilder()
                        .setId("310")
                        .setSkuName("一箱")
                        .setUnit("3301")
                        .setPrice("500")
                        .setStockQty("30")
                        .setExpectedVersion(expectedVersion)
                        .build())
                .build();
    }

    private static ProductSkuDTO onlySku(UpdateProductDTO dto) {
        assertEquals(1, dto.getSkus().size(), "測試前提：只有一個 SKU");
        return dto.getSkus().get(0);
    }
}
