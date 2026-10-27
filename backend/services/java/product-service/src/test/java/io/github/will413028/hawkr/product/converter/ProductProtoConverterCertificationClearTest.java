package io.github.will413028.hawkr.product.converter;

import io.github.will413028.hawkr.product.dto.request.UpdateProductDTO;
import io.github.will413028.hawkr.product.grpc.Certification;
import io.github.will413028.hawkr.product.grpc.UpdateProductRequest;
import org.junit.jupiter.api.DisplayName;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.api.extension.ExtendWith;
import org.mockito.junit.jupiter.MockitoExtension;

import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.junit.jupiter.api.Assertions.assertNotNull;
import static org.junit.jupiter.api.Assertions.assertNull;
import static org.junit.jupiter.api.Assertions.assertTrue;

/**
 * proto3 的 repeated 分不出「空清單」與「未提供」，所以只看 count>0 等於宣告認證只增不減。
 *
 * 實測坐實的症狀（dev，2026-08-18）：移除唯一一張認證 → UI 顯示移除了 → PATCH 回 200 →
 * 頁面重抓後標章又回來；DB 原封不動，稽核列卻是 result=SUCCEEDED 而 changed_fields 為空。
 * 認證是賣家對買家的法規宣稱，「掛得上拿不下來」比一般欄位嚴重，而且賣家自助端與代行端
 * 走同一支 RPC ⇒ 影響的是所有賣家，不只代行者。
 *
 * 這幾條守的是旗標語意本身。沒有它們，把 converter 的 {@code ||} 條件改回單看 count
 * 不會讓任何東西轉紅 —— 那正是這個 bug 活到今天的原因。
 */
@ExtendWith(MockitoExtension.class)
class ProductProtoConverterCertificationClearTest {

    private final ProductProtoConverter converter = new ProductProtoConverter();

    private static UpdateProductRequest.Builder base() {
        return UpdateProductRequest.newBuilder()
                .setProductId("866542646616428481")
                .setStoreId(2685914682L);
    }

    @Test
    @DisplayName("update_certifications=true ＋ 空清單 → DTO 給空 list（可清空），不是 null")
    void clearsWhenFlagSetAndListEmpty() {
        UpdateProductDTO dto = converter.toUpdateDTO(
                base().setUpdateCertifications(true).build());

        assertNotNull(dto.getCertifications(),
                "null 會讓 ProductEntityConverter 整段跳過 ⇒ 認證清不掉");
        assertTrue(dto.getCertifications().isEmpty());
    }

    @Test
    @DisplayName("未帶旗標 ＋ 空清單 → 維持 null（不動認證），這是舊 client 的相容行為")
    void keepsCurrentWhenNoFlagAndListEmpty() {
        UpdateProductDTO dto = converter.toUpdateDTO(base().build());

        assertNull(dto.getCertifications(),
                "沒有任何一方表達要動認證時就不該動它");
    }

    @Test
    @DisplayName("未帶旗標 ＋ 非空清單 → 仍然更新（product-service 與 gateway 分開部署的窗口）")
    void stillUpdatesWhenNoFlagButListNonEmpty() {
        UpdateProductDTO dto = converter.toUpdateDTO(
                base().addCertifications(
                        Certification.newBuilder()
                                .setName("3701")
                                .setCertNo("1-1-0987654321")
                                .build())
                        .build());

        assertNotNull(dto.getCertifications(),
                "少了這個相容分支，先上的那個服務會讓認證更新在部署窗口內全部失效");
        assertEquals(1, dto.getCertifications().size());
        assertEquals("3701", dto.getCertifications().get(0).getName());
    }

    @Test
    @DisplayName("旗標 ＋ 非空清單 → 全量取代")
    void replacesWhenFlagSetAndListNonEmpty() {
        UpdateProductDTO dto = converter.toUpdateDTO(
                base().setUpdateCertifications(true)
                        .addCertifications(
                                Certification.newBuilder()
                                        .setName("3702")
                                        .setCertNo("A-2-1234567890")
                                        .build())
                        .build());

        assertEquals(1, dto.getCertifications().size());
        assertEquals("3702", dto.getCertifications().get(0).getName());
        assertEquals("A-2-1234567890", dto.getCertifications().get(0).getCertNo());
    }
}
