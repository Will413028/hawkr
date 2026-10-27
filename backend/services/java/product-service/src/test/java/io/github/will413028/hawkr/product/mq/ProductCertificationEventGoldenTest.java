package io.github.will413028.hawkr.product.mq;

import io.github.will413028.hawkr.common.utils.code.ProductEventType;
import io.github.will413028.hawkr.product.mq.dto.ProductCertificationDecidedPayload;
import io.github.will413028.hawkr.product.mq.dto.ProductCertificationVerifyRequestedPayload;
import com.fasterxml.jackson.databind.JsonNode;
import com.fasterxml.jackson.databind.ObjectMapper;
import org.junit.jupiter.api.DisplayName;
import org.junit.jupiter.api.Test;

import java.nio.file.Files;
import java.nio.file.Path;
import java.nio.file.Paths;
import java.util.Iterator;
import java.util.TreeSet;

import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.junit.jupiter.api.Assertions.assertFalse;
import static org.junit.jupiter.api.Assertions.assertNotNull;
import static org.junit.jupiter.api.Assertions.assertTrue;

/**
 * publisher 端 golden 契約：<b>認證三個決定事件實際序列化出來的 wire key
 * ＝ {@code tests/contract/golden/event_payload_keys.json} 的 {@code emitted_keys}</b>
 * （36 §3.6h.1，#2755；golden README「維護規則」最後一條所說的 {@code *GoldenTest}）。
 *
 * <p>🔴 <b>這支測試補的是一個沒有任何東西守著的方向。</b>
 * notification-service 的 {@code TestTemplateContract} 只驗「template 讀的 key ⊆ golden」，
 * <b>不驗「golden ⊆ publisher 實發」</b>；而 golden 與 template 是同一批寫的、彼此自洽
 * ⇒ publisher 少發一個 key、或忘了 {@code @JsonNaming(SnakeCaseStrategy)} 而發成 camelCase，
 * <b>兩邊都不會紅</b>。症狀是通知靜默 0 投遞：事件發出、consumer 收到、找不到 key、
 * 渲染成空字串或直接丟棄，沒有錯誤、沒有 DLQ、沒有告警。本 repo 有前例。
 *
 * <p>⚠️ 36 §3.6h.1 列的是 <b>Java record component 名稱</b>（camelCase），
 * golden 與 wire 是 snake_case，兩者差一個命名策略——不要拿設計書的清單直接當 wire key 對。
 */
class ProductCertificationEventGoldenTest {

    private static final ObjectMapper MAPPER = new ObjectMapper();
    private static final String GOLDEN_REL = "tests/contract/golden/event_payload_keys.json";

    @Test
    @DisplayName("approved payload 的 wire key 與 golden 完全一致（不多不少）")
    void approvedMatchesGolden() {
        assertKeysMatchGolden("product.certification.approved.seller",
                decided(ProductEventType.PRODUCT_CERTIFICATION_APPROVED, null, null));
    }

    @Test
    @DisplayName("rejected payload 的 wire key 與 golden 完全一致（含 reason_code／public_reason）")
    void rejectedMatchesGolden() {
        assertKeysMatchGolden("product.certification.rejected.seller",
                decided(ProductEventType.PRODUCT_CERTIFICATION_REJECTED, "CERT_NOT_FOUND", "政府端查無此證號"));
    }

    @Test
    @DisplayName("revoked payload 的 wire key 與 golden 完全一致（含 reason_code／public_reason）")
    void revokedMatchesGolden() {
        assertKeysMatchGolden("product.certification.revoked.seller",
                decided(ProductEventType.PRODUCT_CERTIFICATION_REVOKED, "CERT_HOLDER_MISMATCH", "證號歸屬不符"));
    }

    /**
     * {@code internal_note} 不在任何一個 golden 條目裡，也絕不能被序列化出來
     * ——上面三支「不多不少」的斷言已經涵蓋，這支是為了讓失敗訊息直接指出原因。
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
     * {@code product.certification.verify.requested} 由 product-service 自己消費、沒有 notification
     * template ⇒ <b>刻意不進 golden</b>（golden 的反向斷言要求每個條目都要找得到 template）。
     * 它的 wire 形狀只能在這裡守。
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

    private void assertKeysMatchGolden(String eventType, ProductCertificationDecidedPayload payload) {
        JsonNode golden = goldenEntry(eventType);
        TreeSet<String> expected = new TreeSet<>();
        golden.get("emitted_keys").forEach(k -> expected.add(k.asText()));

        String json = serialize(payload);
        TreeSet<String> actual = new TreeSet<>(keysOf(json));

        assertEquals(expected, actual,
                eventType + " 的 wire key 與 golden 不一致（golden ⊆ publisher 這個方向沒有其他東西在守）："
                        + json);

        // golden 宣告 seller_account_id 屬 account_id identity space；發成 store_id 會讓
        // re-key 過的店家收不到通知，而且測試環境（首任店家兩者相等）全綠
        assertEquals("account_id", golden.get("identity_spaces").get("seller_account_id").asText(),
                "golden 的 identity space 被改動，請確認收件人語意");
        assertTrue(readTree(json).get("seller_account_id").isTextual());
    }

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

    private static JsonNode goldenEntry(String eventType) {
        Path golden = repoRoot().resolve(GOLDEN_REL);
        try {
            JsonNode entry = MAPPER.readTree(Files.readAllBytes(golden)).get(eventType);
            assertNotNull(entry, "golden 找不到條目 " + eventType + "（檔案：" + golden + "）");
            return entry;
        } catch (java.io.IOException e) {
            throw new IllegalStateException("讀不到 golden：" + golden, e);
        }
    }

    /** 由 surefire 的 cwd（module 目錄）往上找到含 golden 檔的 repo root。 */
    private static Path repoRoot() {
        Path dir = Paths.get(System.getProperty("user.dir")).toAbsolutePath();
        for (int i = 0; i < 8 && dir != null; i++, dir = dir.getParent()) {
            if (Files.isRegularFile(dir.resolve(GOLDEN_REL))) {
                return dir;
            }
        }
        throw new IllegalStateException("由 " + System.getProperty("user.dir") + " 往上八層找不到 " + GOLDEN_REL);
    }
}
