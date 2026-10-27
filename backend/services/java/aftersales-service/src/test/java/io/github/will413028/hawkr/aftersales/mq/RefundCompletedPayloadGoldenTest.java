package io.github.will413028.hawkr.aftersales.mq;

import io.github.will413028.hawkr.aftersales.mq.payload.RefundCompletedPayload;
import com.fasterxml.jackson.databind.ObjectMapper;
import org.junit.jupiter.api.Test;

import java.io.IOException;
import java.util.Map;
import java.util.Set;

import static org.junit.jupiter.api.Assertions.assertTrue;

/**
 * issue #1626 golden guardrail：驗證 tests/contract/golden/event_payload_keys.json
 * 裡 aftersales.refund.completed.{seller,buyer} 宣告的 emitted_keys 都是這個 payload
 * class 真的會序列化出來的 key——防止 golden 手寫出不存在的欄位（本 issue 的根因，
 * 13 號設計書 §14.3「golden 禁手寫，須由真實 serializer 產生」原則的落地）。
 *
 * <p>期望值由 {@link GoldenPayloadKeys} 直接讀 golden 檔取得，不在此手抄——手抄的
 * 常數只會複製 golden 的內容，golden 多寫一個不存在的欄位時測試仍會綠。
 */
class RefundCompletedPayloadGoldenTest {

    private static final String SELLER_EVENT_TYPE = "aftersales.refund.completed.seller";
    private static final String BUYER_EVENT_TYPE = "aftersales.refund.completed.buyer";

    @Test
    void serializedKeys_containAllGoldenSellerKeys() throws IOException {
        Set<String> goldenKeys = GoldenPayloadKeys.emittedKeys(SELLER_EVENT_TYPE);
        Set<String> actualKeys = serializedKeys();
        assertTrue(actualKeys.containsAll(goldenKeys),
                "RefundCompletedPayload 缺少 golden 宣告的 seller 欄位, golden=" + goldenKeys
                        + ", actual=" + actualKeys);
    }

    @Test
    void serializedKeys_containAllGoldenBuyerKeys() throws IOException {
        Set<String> goldenKeys = GoldenPayloadKeys.emittedKeys(BUYER_EVENT_TYPE);
        Set<String> actualKeys = serializedKeys();
        assertTrue(actualKeys.containsAll(goldenKeys),
                "RefundCompletedPayload 缺少 golden 宣告的 buyer 欄位, golden=" + goldenKeys
                        + ", actual=" + actualKeys);
    }

    @SuppressWarnings("unchecked")
    private Set<String> serializedKeys() throws IOException {
        RefundCompletedPayload payload = new RefundCompletedPayload(
                "CASE1", "ORD1", "FULL_REFUND", "1000", "CREDIT_CARD", "EXT1",
                "2026-07-25T00:00:00", "system", "7001", "5001", "9001", "42");
        byte[] json = new ObjectMapper().writeValueAsBytes(payload);
        Map<String, Object> map = new ObjectMapper().readValue(json, Map.class);
        return map.keySet();
    }
}
