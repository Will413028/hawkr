package io.github.will413028.hawkr.product.mq;

import io.github.will413028.hawkr.common.utils.code.ProductEventType;
import io.github.will413028.hawkr.product.mq.dto.ProductCertificationDecidedPayload;
import io.github.will413028.hawkr.product.mq.dto.ProductCertificationVerifyRequestedPayload;
import com.fasterxml.jackson.databind.JsonNode;
import com.fasterxml.jackson.databind.ObjectMapper;
import org.junit.jupiter.api.DisplayName;
import org.junit.jupiter.api.Test;

import java.util.Iterator;
import java.util.TreeSet;

import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.junit.jupiter.api.Assertions.assertFalse;
import static org.junit.jupiter.api.Assertions.assertTrue;

/**
 * 認證三個決定事件與 verify.requested 的 wire 形狀：snake_case key、ID 為 JSON 字串、不外洩 internal_note。
 */
class ProductCertificationEventWireTest {

    private static final ObjectMapper MAPPER = new ObjectMapper();

    @Test
    @DisplayName("裁決 payload 的 wire key 皆為 snake_case，seller_account_id 為 JSON 字串")
    void decidedPayloadWireShape() {
        for (ProductEventType type : new ProductEventType[]{
                ProductEventType.PRODUCT_CERTIFICATION_APPROVED,
                ProductEventType.PRODUCT_CERTIFICATION_REJECTED,
                ProductEventType.PRODUCT_CERTIFICATION_REVOKED}) {
            String json = serialize(decided(type, "OTHER", "原因"));
            for (String key : keysOf(json)) {
                assertTrue(key.equals(key.toLowerCase()) && !key.contains("-"), type + " 的 key " + key + " 不是 snake_case");
            }
            assertTrue(readTree(json).get("seller_account_id").isTextual(), type + " 的 seller_account_id 必須是 JSON 字串");
        }
    }

    /**
     * {@code internal_note} 絕不能被序列化出來。
     */
    @Test
    @DisplayName("裁決 payload 永遠不含 internal_note")
    void neverEmitsInternalNote() {
        for (ProductEventType type : new ProductEventType[]{
                ProductEventType.PRODUCT_CERTIFICATION_APPROVED,
                ProductEventType.PRODUCT_CERTIFICATION_REJECTED,
                ProductEventType.PRODUCT_CERTIFICATION_REVOKED}) {
            String json = serialize(decided(type, "OTHER", "原因"));
            assertFalse(json.contains("internal"), type + " payload 出現 internal 欄位: " + json);
        }
    }

    /**
     * {@code product.certification.verify.requested} 由 product-service 自己消費，它的 wire 形狀在這裡守。
     */
    @Test
    @DisplayName("verify.requested 的 wire key 為 snake_case 且 id 皆為 JSON 字串")
    void verifyRequestedWireShape() {
        ProductCertificationVerifyRequestedPayload payload = new ProductCertificationVerifyRequestedPayload(
                "3f2b1c00-0000-4000-8000-000000000000",
                ProductEventType.PRODUCT_CERTIFICATION_VERIFY_REQUESTED.value(),
                "843900000000000001", "843856384063471755", "2685914682",
                "3701", "1-009-300022", true, "2026-08-26T00:00:00Z");

        assertEquals(new TreeSet<>(java.util.Set.of(
                        "event_id", "event_type", "application_id", "product_id", "store_id",
                        "certification_type", "certificate_no", "requires_manual_review", "occurred_at")),
                new TreeSet<>(keysOf(serialize(payload))));

        JsonNode node = readTree(serialize(payload));
        assertTrue(node.get("application_id").isTextual(), "TSID 必須是 JSON 字串，否則消費端 float64 截斷");
        assertTrue(node.get("store_id").isTextual(), "TSID 必須是 JSON 字串");
        assertTrue(node.get("requires_manual_review").isBoolean());
    }

    // ==================== helper ====================

    private ProductCertificationDecidedPayload decided(ProductEventType type,
                                                       String reasonCode, String publicReason) {
        boolean withReason = type != ProductEventType.PRODUCT_CERTIFICATION_APPROVED;
        return new ProductCertificationDecidedPayload(
                "3f2b1c00-0000-4000-8000-000000000001",
                type.value(),
                "843900000000000001",
                "843856384063471755",
                "有機小松菜",
                "2685914682",
                "9002685914682",
                "3701",
                "1-009-300022",
                ProductCertificationDecidedPayload.DECIDED_BY_ADMIN,
                withReason ? reasonCode : null,
                withReason ? publicReason : null,
                "2026-08-26T00:00:00Z");
    }

    private static String serialize(Object payload) {
        try {
            return MAPPER.writeValueAsString(payload);
        } catch (Exception e) {
            throw new IllegalStateException(e);
        }
    }

    private static JsonNode readTree(String json) {
        try {
            return MAPPER.readTree(json);
        } catch (Exception e) {
            throw new IllegalStateException(e);
        }
    }

    private static java.util.Set<String> keysOf(String json) {
        java.util.Set<String> keys = new TreeSet<>();
        for (Iterator<String> it = readTree(json).fieldNames(); it.hasNext(); ) {
            keys.add(it.next());
        }
        return keys;
    }
}
