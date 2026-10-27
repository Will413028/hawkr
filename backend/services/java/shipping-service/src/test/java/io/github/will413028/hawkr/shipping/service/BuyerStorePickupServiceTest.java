package io.github.will413028.hawkr.shipping.service;

import io.github.will413028.hawkr.common.utils.TsidUtil;
import io.github.will413028.hawkr.common.utils.code.GrpcResponseCode;
import io.github.will413028.hawkr.shipping.entity.BuyerStorePickupEntity;
import io.github.will413028.hawkr.shipping.exception.ShippingBusinessException;
import io.github.will413028.hawkr.shipping.mapper.BuyerStorePickupMapper;
import org.junit.jupiter.api.BeforeAll;
import org.junit.jupiter.api.Test;
import org.springframework.test.util.ReflectionTestUtils;

import static org.junit.jupiter.api.Assertions.*;
import static org.mockito.ArgumentMatchers.any;
import static org.mockito.Mockito.*;

class BuyerStorePickupServiceTest {

    @BeforeAll
    static void initTsid() {
        try { TsidUtil.init(400, 0); } catch (IllegalStateException ignored) { }
    }

    @Test
    void updateStorePickup_BelongsToDifferentBuyer_ThrowsNotFound() {
        BuyerStorePickupMapper mapper = mock(BuyerStorePickupMapper.class);
        BuyerStorePickupService service = new BuyerStorePickupService(mapper);
        ReflectionTestUtils.setField(service, "maxPerBuyer", 20);

        BuyerStorePickupEntity existing = new BuyerStorePickupEntity();
        existing.setId(1L);
        existing.setBuyerId(999L); // 屬於另一個買家（分岔後的舊 buyer_id 或他人）
        when(mapper.selectById(1L)).thenReturn(existing);

        ShippingBusinessException ex = assertThrows(ShippingBusinessException.class, () ->
                service.updateStorePickup(100L, 1L, "S001", "門市A", "地址A", "王小明", "0912345678", false));
        assertEquals(GrpcResponseCode.BUYER_STORE_PICKUP_NOT_FOUND, ex.responseCode());
        verify(mapper, never()).updateById(any());
    }

    @Test
    void deleteStorePickup_BelongsToDifferentBuyer_ThrowsNotFound() {
        BuyerStorePickupMapper mapper = mock(BuyerStorePickupMapper.class);
        BuyerStorePickupService service = new BuyerStorePickupService(mapper);

        BuyerStorePickupEntity existing = new BuyerStorePickupEntity();
        existing.setId(1L);
        existing.setBuyerId(999L);
        when(mapper.selectById(1L)).thenReturn(existing);

        assertThrows(ShippingBusinessException.class, () -> service.deleteStorePickup(100L, 1L));
        verify(mapper, never()).deleteById(anyLong());
    }

    @Test
    void addStorePickup_StoresUnderGivenBuyerId() {
        BuyerStorePickupMapper mapper = mock(BuyerStorePickupMapper.class);
        BuyerStorePickupService service = new BuyerStorePickupService(mapper);
        ReflectionTestUtils.setField(service, "maxPerBuyer", 20);
        when(mapper.selectCount(any())).thenReturn(0L);
        when(mapper.selectList(any())).thenReturn(java.util.Collections.emptyList());

        BuyerStorePickupEntity result = service.addStorePickup(
                777L, "S001", "門市A", "地址A", "王小明", "0912345678", false);

        assertEquals(777L, result.getBuyerId());
        verify(mapper).insert(argThat(e -> e.getBuyerId().equals(777L)));
    }
}
