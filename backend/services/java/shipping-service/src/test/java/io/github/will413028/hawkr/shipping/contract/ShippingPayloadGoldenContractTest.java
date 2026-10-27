package io.github.will413028.hawkr.shipping.contract;

import io.github.will413028.hawkr.common.mq.payload.ShipmentAnomalyDetectedPayload;
import io.github.will413028.hawkr.shipping.service.dto.ShipmentCreatedPayload;
import io.github.will413028.hawkr.shipping.service.dto.ShippingEventPayload;
import com.fasterxml.jackson.databind.JsonNode;
import com.fasterxml.jackson.databind.ObjectMapper;
import org.junit.jupiter.api.Test;

import java.io.File;
import java.util.ArrayList;
import java.util.HashSet;
import java.util.Iterator;
import java.util.LinkedHashMap;
import java.util.List;
import java.util.Map;
import java.util.Set;

import static org.junit.jupiter.api.Assertions.assertTrue;

/**
 * publisher-side 反向契約測試（issue #1529）。
 *
 * <p>notification-service 的 template 契約 gate 只驗「template 讀的 key ⊆ golden emitted_keys」，
 * 不驗「golden emitted_keys ⊆ publisher 實發 key」——golden 手寫錯（宣稱發某 key 但 payload 根本沒發）
 * 不會被抓到，但 {@link ShippingEventPayload} 從未有該欄位 → notification 靜默 0 投遞）。
 *
 * <p>本測試補上反向斷言：對 shipping-service 三個 payload 承載的 golden 條目，
 * 驗證 golden {@code emitted_keys} 全部落在 payload 序列化後的實發 key 集內。
 * 用生產同款 Jackson 序列化（@JsonNaming(SnakeCase) 由 annotation 自帶，plain ObjectMapper 即生效）。
 */
class ShippingPayloadGoldenContractTest {

    private static final ObjectMapper MAPPER = new ObjectMapper();
    private static final String PUBLISHER = "shipping-service";

    /**
     * golden event_type → 對應 publisher payload 樣本實例（所有欄位填非 null，確保全欄位序列化出來）。
     *
     * <p>只涵蓋本三 payload 承載的 event_type。曾另有 {@code order.shipped.merchant} 與
     * {@code return.shipped} 兩個 shipping-service golden 條目不在此 map（被 sample == null
     * 跳過），2026-08-16 已連同條目一起刪除：兩者的 template 早在
     * {@code 20260703130001_1176_ghost_template_cleanup.sql} 就因語意被取代而刪掉，
     * golden 留著只會讓人以為它們仍受本測試保護。
     */
    private static Map<String, Object> samplePayloads() {
        Map<String, Object> m = new LinkedHashMap<>();
        ShippingEventPayload shipping = new ShippingEventPayload(
                "evt-1", "O-1", "SHP-1", "3001", "31", "IN_TRANSIT", "運送中",
                "https://x/label.pdf", "T-1", "2026-01-01T00:00:00Z", false,
                "賣家", "https://x/avatar.jpg", 2001L, 2002L, 1001L, "PROV", "provider-msg", 4001L);
        m.put("shipment.delivered", shipping);
        m.put("shipment.delivered.seller", shipping);
        m.put("shipment.status.changed", shipping);
        m.put("shipment.created", new ShipmentCreatedPayload(
                "O-1", "SHP-1", "T-1", "3001", "91", "https://x/label.pdf",
                "2026-01-01T00:00:00Z", "1001", "2001", "1200", "小明", "https://x/buyer.jpg", "4001"));
        m.put("shipment.anomaly.detected.seller", new ShipmentAnomalyDetectedPayload(
                "evt-2", "O-1", "100", 1001L, 2001L, "TCAT_CALL_STUCK",
                "黑貓叫件未完成", "HIGH", "2026-01-01T00:00:00Z", 4001L));
        return m;
    }

    @Test
    void goldenEmittedKeys_areSubsetOfActualPublisherKeys() throws Exception {
        JsonNode golden = MAPPER.readTree(locateGolden());
        Map<String, Object> samples = samplePayloads();

        List<String> violations = new ArrayList<>();
        Iterator<Map.Entry<String, JsonNode>> it = golden.fields();
        while (it.hasNext()) {
            Map.Entry<String, JsonNode> e = it.next();
            String eventType = e.getKey();
            JsonNode entry = e.getValue();
            if (!PUBLISHER.equals(entry.path("publisher").asText())) {
                continue;
            }
            Object sample = samples.get(eventType);
            if (sample == null) {
                continue;
            }
            Set<String> actual = actualKeys(sample);
            for (JsonNode k : entry.path("emitted_keys")) {
                if (!actual.contains(k.asText())) {
                    violations.add(eventType + ": golden 宣稱 emit '" + k.asText()
                            + "' 但 " + sample.getClass().getSimpleName()
                            + " 實發 key=" + actual);
                }
            }
        }

        // 防呆：三個 payload 對應的 event_type 必須真的存在於 golden，避免 golden 改名後靜默漏驗
        for (String eventType : samples.keySet()) {
            assertTrue(golden.has(eventType),
                    "golden 缺少本測試預期覆蓋的 event_type: " + eventType);
        }

        assertTrue(violations.isEmpty(),
                "契約違反（golden emitted_keys ⊄ publisher 實發 key）:\n" + String.join("\n", violations));
    }

    private static Set<String> actualKeys(Object payload) {
        JsonNode node = MAPPER.valueToTree(payload);
        Set<String> keys = new HashSet<>();
        node.fieldNames().forEachRemaining(keys::add);
        return keys;
    }

    private static File locateGolden() {
        File dir = new File("").getAbsoluteFile();
        while (dir != null) {
            File f = new File(dir, "tests/contract/golden/event_payload_keys.json");
            if (f.isFile()) {
                return f;
            }
            dir = dir.getParentFile();
        }
        throw new IllegalStateException(
                "找不到 tests/contract/golden/event_payload_keys.json（從 " + new File("").getAbsolutePath() + " 向上找）");
    }
}
