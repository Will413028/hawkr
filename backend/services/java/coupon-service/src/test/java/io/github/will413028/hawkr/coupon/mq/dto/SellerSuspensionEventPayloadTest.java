package io.github.will413028.hawkr.coupon.mq.dto;

import com.fasterxml.jackson.databind.JsonNode;
import com.fasterxml.jackson.databind.ObjectMapper;
import org.junit.jupiter.api.Test;

import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.junit.jupiter.api.Assertions.assertNotEquals;
import static org.junit.jupiter.api.Assertions.assertNotNull;
import static org.junit.jupiter.api.Assertions.assertNull;

/**
 * Plan C Phase 1 Task 2：SellerSuspensionEventPayload 新增 storeId 欄位
 * （Phase 0 擴充的 store_id wire event，snake_case key，見
 * pkg/mq/events.go SellerSuspendedEvent/SellerUnsuspendedEvent）。
 */
class SellerSuspensionEventPayloadTest {

    private static final ObjectMapper MAPPER = new ObjectMapper();

    @Test
    void fromJson_parsesStoreIdSeparatelyFromAccountId() throws Exception {
        JsonNode node = MAPPER.readTree(
                "{\"event_type\":\"SELLER_SUSPENDED\",\"suspensionId\":\"7482084104842517848\",\"accountId\":\"100\",\"reasonCode\":\"9601\",\"store_id\":\"555\"}");

        SellerSuspensionEventPayload payload = SellerSuspensionEventPayload.fromJson(node);

        assertNotNull(payload);
        assertEquals(100L, payload.accountId());
        assertEquals(555L, payload.storeId());
        assertNotEquals(payload.accountId(), payload.storeId());
    }

    @Test
    void fromJson_missingStoreId_returnsNull() throws Exception {
        JsonNode node = MAPPER.readTree(
                "{\"event_type\":\"SELLER_SUSPENDED\",\"suspensionId\":\"7482084104842517848\",\"accountId\":\"100\",\"reasonCode\":\"9601\"}");

        assertNull(SellerSuspensionEventPayload.fromJson(node));
    }
}
