package io.github.will413028.hawkr.aftersales.mq;

import io.github.will413028.hawkr.aftersales.mq.payload.AwaitingBuyerRefundPagePayload;
import com.fasterxml.jackson.databind.ObjectMapper;
import org.junit.jupiter.api.Test;

import java.io.IOException;
import java.util.Map;
import java.util.Set;

import static org.junit.jupiter.api.Assertions.assertTrue;

/**
 * issue #1626 golden guardrail：同 RefundCompletedPayloadGoldenTest，
 * 驗證 aftersales.refund.awaiting_buyer_page.buyer 宣告的 emitted_keys 為真。
 *
 * <p>期望值由 {@link GoldenPayloadKeys} 直接讀 tests/contract/golden/event_payload_keys.json
 * 取得，不在此手抄成常數。
 */
class AwaitingBuyerRefundPagePayloadGoldenTest {

    private static final String EVENT_TYPE = "aftersales.refund.awaiting_buyer_page.buyer";

    @Test
    void serializedKeys_containAllGoldenKeys() throws IOException {
        Set<String> goldenKeys = GoldenPayloadKeys.emittedKeys(EVENT_TYPE);

        AwaitingBuyerRefundPagePayload payload = new AwaitingBuyerRefundPagePayload(
                "ORD1", "CASE1", "9001", "2026-07-25T00:00:00", "8001", "42");
        byte[] json = new ObjectMapper().writeValueAsBytes(payload);
        @SuppressWarnings("unchecked")
        Map<String, Object> map = new ObjectMapper().readValue(json, Map.class);

        assertTrue(map.keySet().containsAll(goldenKeys),
                "AwaitingBuyerRefundPagePayload 缺少 golden 宣告的欄位, golden=" + goldenKeys
                        + ", actual=" + map.keySet());
    }
}
