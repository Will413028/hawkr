package io.github.will413028.hawkr.shipping.service;

import io.github.will413028.hawkr.common.utils.code.ShippingStatus;
import org.junit.jupiter.api.Test;

import static org.junit.jupiter.api.Assertions.assertFalse;
import static org.junit.jupiter.api.Assertions.assertNull;
import static org.junit.jupiter.api.Assertions.assertSame;
import static org.junit.jupiter.api.Assertions.assertTrue;

class ShipmentStatusMapperTest {

    private final ShipmentStatusMapper mapper = new ShipmentStatusMapper();

    @Test
    void mapPayUniStatus_unknownOrBlank_returnsNull() {
        assertNull(mapper.mapPayUniStatus(null));
        assertNull(mapper.mapPayUniStatus(""));
        assertNull(mapper.mapPayUniStatus("-"));
    }

    @Test
    void mapPayUniStatus_knownValue_returnsShippingStatus() {
        assertSame(ShippingStatus.PROCESSING, mapper.mapPayUniStatus("92"));
        assertSame(ShippingStatus.PICKED_UP, mapper.mapPayUniStatus("11"));
    }

    @Test
    void payUniPreShipment_canProgressToLaterStatus() {
        assertTrue(mapper.isValidTransition(ShippingStatus.PROCESSING, ShippingStatus.IN_TRANSIT));
        assertTrue(mapper.isValidTransition(ShippingStatus.PROCESSING, ShippingStatus.PICKED_UP));
        assertTrue(mapper.isValidTransition(ShippingStatus.PROCESSING, ShippingStatus.ABNORMAL));
    }

    @Test
    void localProcessingDone_rejectsPayUniPreShipmentRollback() {
        assertFalse(mapper.isValidTransition(ShippingStatus.CVS_CODE_RECEIVED, ShippingStatus.UNPROCESSED));
        assertFalse(mapper.isValidTransition(ShippingStatus.CVS_CODE_RECEIVED, ShippingStatus.PROCESSING));
        assertFalse(mapper.isValidTransition(ShippingStatus.SELLER_CONFIRMED, ShippingStatus.PROCESSING));
        assertFalse(mapper.isValidTransition(ShippingStatus.SELLER_CONFIRMED, ShippingStatus.PENDING_SHIPMENT));
        assertFalse(mapper.isValidTransition(ShippingStatus.TCAT_CALLED, ShippingStatus.PENDING_SHIPMENT));
    }

    @Test
    void localProcessingDone_acceptsRealShippingProgress() {
        assertTrue(mapper.isValidTransition(ShippingStatus.CVS_CODE_RECEIVED, ShippingStatus.IN_TRANSIT));
        assertTrue(mapper.isValidTransition(ShippingStatus.SELLER_CONFIRMED, ShippingStatus.WAITING_PICKUP));
        assertTrue(mapper.isValidTransition(ShippingStatus.SELLER_CONFIRMED, ShippingStatus.PICKED_UP));
        assertTrue(mapper.isValidTransition(ShippingStatus.TCAT_CALLED, ShippingStatus.IN_TRANSIT));
        assertTrue(mapper.isValidTransition(ShippingStatus.TCAT_CALLED, ShippingStatus.ABNORMAL));
    }
}
