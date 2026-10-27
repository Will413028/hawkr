package io.github.will413028.hawkr.product.converter;

import io.github.will413028.hawkr.common.utils.code.ProductPublicationState;
import io.github.will413028.hawkr.common.utils.code.ProductStatus;
import io.github.will413028.hawkr.product.entity.ProductCertificationApplicationEntity;
import io.github.will413028.hawkr.product.entity.projection.ProductPublicationBriefProjection;
import io.github.will413028.hawkr.product.grpc.ProductCertificationApplication;
import org.junit.jupiter.api.DisplayName;
import org.junit.jupiter.api.Test;

import java.time.LocalDateTime;

import static org.junit.jupiter.api.Assertions.assertEquals;

/**
 * ADM 認證審核回應的商品欄位（#2818）。
 *
 * <p>症狀是 ADM 六支端點的 {@code productName} 對已軟刪商品一律空字串，
 * 成因是取名字用了帶 {@code deleted_at IS NULL} 的公開視角查詢——而認證申請
 * <b>結構上就會指向已軟刪商品</b>（§15.3 migration 不濾、賣家送件後可刪商品、
 * 而 {@code updateCertificationProjection} 刻意不濾以便 ADM 對已下架商品撤銷標章）。
 *
 * <p>🔴 這幾條同時守兩件事，缺一都會讓修復悄悄退化：
 * <ul>
 *   <li><b>陽性</b>：已軟刪商品仍取得到名字（症狀本身）</li>
 *   <li><b>陰性</b>：live 商品**不得**被標成已移除——否則「永遠標已移除」也會讓陽性斷言通過</li>
 * </ul>
 * 另外守住「狀態值由 {@code ProductPublicationPolicy} 產生」這個約束：任何人把
 * converter 改成自己寫 {@code deletedAt != null || sellerDeletedAt != null}，
 * {@code seller_deleted_at} 那一條會轉紅。
 */
class ProductCertificationApplicationConverterAdminProductStateTest {

    private static final LocalDateTime T = LocalDateTime.of(2026, 8, 27, 4, 12, 5);

    private final ProductCertificationApplicationConverter converter =
            new ProductCertificationApplicationConverter();

    private static ProductCertificationApplicationEntity application() {
        ProductCertificationApplicationEntity e = new ProductCertificationApplicationEntity();
        e.setId(843856384063471761L);
        e.setProductId(843856384063471755L);
        e.setStoreId(2685914682L);
        e.setCertificationType(3703);
        e.setCertificateNo("010105");
        e.setStatus("SUBMITTED");
        e.setSource("LEGACY_MIGRATION");
        e.setRequiresManualReview(false);
        e.setGovVerifyState("PENDING");
        e.setVersion(0L);
        e.setSubmittedAt(T);
        return e;
    }

    private static ProductPublicationBriefProjection brief(
            LocalDateTime firstPublishedAt, String status,
            LocalDateTime deletedAt, LocalDateTime sellerDeletedAt) {
        return new ProductPublicationBriefProjection(
                843856384063471755L, "有機小松菜", status, firstPublishedAt, deletedAt, sellerDeletedAt, true);
    }

    @Test
    @DisplayName("商品已軟刪：名字仍取得到，且標成 REMOVED —— 這是 #2818 的症狀本身")
    void softDeletedProductStillResolvesNameAndIsMarkedRemoved() {
        ProductCertificationApplication proto = converter.toAdminProto(
                application(),
                brief(T, ProductStatus.ACTIVE.value(), T, null),
                null);

        assertEquals("有機小松菜", proto.getProductName(),
                "已軟刪商品的名字必須取得到——空字串正是 #2818，ADM 會看不出這是哪個商品");
        assertEquals(ProductPublicationState.REMOVED.value(), proto.getProductPublicationState(),
                "已軟刪必須標 REMOVED，否則補回名字只是把可見缺陷換成靜默缺陷");
    }

    @Test
    @DisplayName("賣家帳號已刪（seller_deleted_at）：同樣算已移除 —— 商品有兩個軟刪戳")
    void sellerDeletedProductIsAlsoRemoved() {
        ProductCertificationApplication proto = converter.toAdminProto(
                application(),
                brief(T, ProductStatus.ACTIVE.value(), null, T),
                null);

        assertEquals(ProductPublicationState.REMOVED.value(), proto.getProductPublicationState(),
                "只看 deleted_at 會漏掉『賣家被刪、商品連坐』那一整類（V7）");
    }

    @Test
    @DisplayName("陰性：live 且上架中的商品標 PUBLIC，不得被標成已移除")
    void livePublishedProductIsPublic() {
        ProductCertificationApplication proto = converter.toAdminProto(
                application(),
                brief(T, ProductStatus.ACTIVE.value(), null, null),
                null);

        assertEquals("有機小松菜", proto.getProductName());
        assertEquals(ProductPublicationState.PUBLIC.value(), proto.getProductPublicationState(),
                "沒有這一條，『永遠回 REMOVED』的實作也能讓上面兩條通過");
    }

    @Test
    @DisplayName("陰性：下架中（曾公開）是 TEMPORARILY_UNAVAILABLE，不是已移除 —— 這種申請照常審")
    void inactiveButPublishedIsTemporarilyUnavailable() {
        ProductCertificationApplication proto = converter.toAdminProto(
                application(),
                brief(T, ProductStatus.INACTIVE.value(), null, null),
                null);

        assertEquals(ProductPublicationState.TEMPORARILY_UNAVAILABLE.value(),
                proto.getProductPublicationState(),
                "下架 ≠ 刪除；把它也標成已移除會讓審核者誤以為不用審");
    }

    @Test
    @DisplayName("商品列查不到（實體刪除）：名字空字串、狀態不設，但 productId 仍在")
    void missingProductRowLeavesNameEmptyButKeepsProductId() {
        ProductCertificationApplication proto = converter.toAdminProto(application(), null, null);

        assertEquals("", proto.getProductName());
        assertEquals("", proto.getProductPublicationState(),
                "查不到就不設——不得猜成 REMOVED，那是把『沒有資料』講成『有資料且已移除』");
        assertEquals("843856384063471755", proto.getProductId(),
                "至少要留 productId 讓 ADM 追得回去");
    }
}
