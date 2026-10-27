package io.github.will413028.hawkr.shipping.service.dto;

import org.junit.jupiter.api.Test;
import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.junit.jupiter.api.Assertions.assertNull;

class ShippingEventPayloadTest {

    @Test
    void compatConstructor9Args_buyerIdIsNull() {
        ShippingEventPayload payload = new ShippingEventPayload(
                "O123", "S456", "TCAT",
                "IN_TRANSIT", "31", "運送中",
                "https://example.com/label.pdf", "T789", "2026-05-01"
        );
        assertNull(payload.buyerId());
        assertNull(payload.sellerName());
        assertNull(payload.sellerAvatarKey());
    }

    @Test
    void compatConstructor10Args_buyerIdIsNull() {
        ShippingEventPayload payload = new ShippingEventPayload(
                "O123", "S456", "TCAT",
                "IN_TRANSIT", "31", "運送中",
                "https://example.com/label.pdf", "T789", "2026-05-01",
                false
        );
        assertNull(payload.buyerId());
    }

    @Test
    void fullConstructor_carriesBuyerId() {
        ShippingEventPayload payload = new ShippingEventPayload(
                "O123", "S456", "TCAT",
                "IN_TRANSIT", "31", "運送中",
                "https://example.com/label.pdf", "T789", "2026-05-01",
                false, "TestSeller", "https://oss/avatar.jpg",
                999L, 888L
        );
        assertEquals(999L, payload.buyerId());
        assertEquals(888L, payload.notifyBuyerAccountId());
    }
}
