package io.github.will413028.hawkr.order.grpc.converter;

import io.github.will413028.hawkr.order.entity.OrderEntity;
import io.github.will413028.hawkr.order.entity.OrderItemEntity;
import io.github.will413028.hawkr.order.grpc.OrderAfterSalesView;
import io.github.will413028.hawkr.order.grpc.OrderDetailView;
import io.github.will413028.hawkr.order.grpc.OrderSummaryView;
import io.github.will413028.hawkr.seller.grpc.SellerMeta;
import io.github.will413028.hawkr.shipping.grpc.ShipmentDetailView;
import org.junit.jupiter.api.Test;

import java.time.LocalDateTime;
import java.util.List;

import static org.assertj.core.api.Assertions.assertThat;

class OrderProtoConverterTest {

    @Test
    void summaryViewUsesLiveShipmentLabelCodesForSellerView() {
        OrderEntity order = baseOrder();
        ShipmentDetailView liveShipment = ShipmentDetailView.newBuilder()
                .setLabelKey("https://cdn.example/label.pdf")
                .setPartnerId("777")
                .setOdno("ODNO1234")
                .setValidationNo("9876")
                .build();

        OrderSummaryView summary = OrderProtoConverter.toSummaryView(
                order, List.of(), null, null, "", true, liveShipment);

        assertThat(summary.getLabelKey()).isEqualTo("https://cdn.example/label.pdf");
        assertThat(summary.getPartnerId()).isEqualTo("777");
        assertThat(summary.getOdno()).isEqualTo("ODNO1234");
        assertThat(summary.getValidationNo()).isEqualTo("9876");
    }

    @Test
    void summaryViewFallsBackToOrderSnapshotWhenLiveShipmentCodesAreBlank() {
        OrderEntity order = baseOrder();
        order.setLabelUrl("https://cdn.example/snapshot-label.pdf");
        order.setPartnerId("123");
        order.setOdno("SNAP5678");
        order.setValidationNo("4321");

        OrderSummaryView summary = OrderProtoConverter.toSummaryView(
                order, List.of(), null, null, "", true, ShipmentDetailView.getDefaultInstance());

        assertThat(summary.getLabelKey()).isEqualTo("https://cdn.example/snapshot-label.pdf");
        assertThat(summary.getPartnerId()).isEqualTo("123");
        assertThat(summary.getOdno()).isEqualTo("SNAP5678");
        assertThat(summary.getValidationNo()).isEqualTo("4321");
    }

    /**
     * #2137：訂單列表／詳情的賣家連結先前只有 store_id（內部代理鍵），前端連
     * /sellers/{store_id} 每次都要多吃一次 middleware 308。賣家頁 canonical 網址
     * 走 store_code，而這個值本來就在同一份 SellerMeta 裡（店名與 logo 就是從那裡
     * 抄的），只是組 proto 時被丟掉——補齊零額外 RPC。
     *
     * store_code 刻意設成與 store_id 不同：兩者相等的賣家（由已綁定的 PayUni
     * merchant_code 回填）驗不出有沒有真的讀對欄位。
     */
    @Test
    void summaryAndDetailViewsCarrySellerStoreCode() {
        OrderEntity order = baseOrder();
        SellerMeta sellerMeta = SellerMeta.newBuilder()
                .setSellerName("Seller X")
                .setLogoKey("logo.jpg")
                .setStoreCode("2649605557")
                .build();

        OrderSummaryView summary = OrderProtoConverter.toSummaryView(
                order, List.of(), sellerMeta, null, "", false);
        assertThat(summary.getStoreCode()).isEqualTo("2649605557");

        OrderDetailView detail = OrderProtoConverter.toDetailView(
                order, List.of(), List.of(), null, sellerMeta, null, "", false);
        assertThat(detail.getStoreCode()).isEqualTo("2649605557");
    }

    /** 賣家聚合失敗時整個賣家區塊留空，前端據此不渲染連結（而不是連 store_id）。 */
    @Test
    void viewsLeaveStoreCodeBlankWhenSellerLookupDegrades() {
        OrderEntity order = baseOrder();

        OrderSummaryView summary = OrderProtoConverter.toSummaryView(
                order, List.of(), null, null, "", false);
        assertThat(summary.getStoreCode()).isEmpty();
        assertThat(summary.getSellerName()).isEmpty();
    }

    /**
     * non-catalog line（RFQ 成交的訂單明細）：{@code order_items.sku_id} 為 NULL。
     * 三條讀取路徑都必須原樣通過 —— summary／detail 是訂單查詢，afterSales 是
     * aftersales-service 建案時撈的那一份。
     *
     * <p>🔴 {@code ProtoConversionUtils.formatLong} 對 null 回空字串（:85-87），而 gateway 的
     * 訂單明細 DTO 是 {@code SkuID string `json:"skuId,omitempty"`} ⇒ 缺值自然不出現在 JSON。
     * <p>⚠️ 品名不受影響：{@code sku_name} 維持 NOT NULL —— doc 64 §6.4.9 禁止 sentinel 的是
     * <b>品名</b>，那條禁令不約束 {@code sku_id} 指向誰。
     */
    @Test
    void nonCatalogLineWithNullSkuIdPassesAllReadPaths() {
        OrderEntity order = baseOrder();
        OrderItemEntity item = nonCatalogItem();

        OrderSummaryView summary = OrderProtoConverter.toSummaryView(
                order, List.of(item), null, null, "", false, null);
        assertThat(summary.getItems(0).getSkuId()).isEmpty();
        assertThat(summary.getItems(0).getSkuName()).isEqualTo("文旦柚 200 公斤");

        OrderDetailView detail = OrderProtoConverter.toDetailView(
                order, List.of(item), List.of(), null, null, null, "", false);
        assertThat(detail.getItems(0).getSkuId()).isEmpty();

        OrderAfterSalesView afterSales = OrderProtoConverter.toAfterSalesView(
                order, List.of(item), "");
        assertThat(afterSales.getItems(0).getSkuId()).isEmpty();
        assertThat(afterSales.getItems(0).getSkuName()).isEqualTo("文旦柚 200 公斤");
    }

    /** catalog line 不受影響：有 SKU 就照原樣輸出，放寬只針對缺值。 */
    @Test
    void catalogLineStillCarriesSkuId() {
        OrderEntity order = baseOrder();
        OrderItemEntity item = nonCatalogItem();
        item.setSkuId(5150L);

        OrderSummaryView summary = OrderProtoConverter.toSummaryView(
                order, List.of(item), null, null, "", false, null);
        assertThat(summary.getItems(0).getSkuId()).isEqualTo("5150");
    }

    private static OrderItemEntity nonCatalogItem() {
        OrderItemEntity item = new OrderItemEntity();
        item.setId(7001L);
        item.setOrderNo("O_TEST_001");
        item.setSkuId(null);
        item.setSkuName("文旦柚 200 公斤");
        item.setProductName("文旦柚");
        item.setQuantity(1);
        item.setUnitPrice(30000L);
        item.setListPrice(30000L);
        item.setShareDiscount(0L);
        item.setActualPayAmount(30000L);
        // DB 是 NOT NULL DEFAULT '0'；記憶體 entity 要自己補，protobuf 的 setter 不吃 null。
        item.setRefundStatus("0");
        item.setImageObjectKey("");
        return item;
    }

    private static OrderEntity baseOrder() {
        OrderEntity order = new OrderEntity();
        order.setOrderNo("O_TEST_001");
        order.setCreatedAt(LocalDateTime.of(2026, 5, 3, 1, 2, 3));
        order.setUpdatedAt(LocalDateTime.of(2026, 5, 3, 1, 2, 3));
        order.setStatus("10");
        order.setTotalActualAmount(100L);
        order.setStoreId(1001L);
        order.setBuyerId(2001L);
        order.setShippingMethod("3002");
        order.setDeliveryTemp("3201");
        return order;
    }
}
