package io.github.will413028.hawkr.common.utils.shipping;

import org.junit.jupiter.api.Test;

import java.util.List;
import java.util.Set;

import static org.junit.jupiter.api.Assertions.*;

class SellerShippingFilterTest {

    @Test
    void toMethodKey_handlesNullGoodsType() {
        assertEquals("3001:", SellerShippingFilter.toMethodKey("3001", null));
        assertEquals("3001:", SellerShippingFilter.toMethodKey("3001", ""));
        assertEquals("3002:3201", SellerShippingFilter.toMethodKey("3002", "3201"));
    }

    @Test
    void isAvailable_emptySet_returnsFalse_failClose() {
        // 對應 03 §4.3：empty set 不再 fail-open
        assertFalse(SellerShippingFilter.isAvailable("3001", "3201", Set.of()));
        assertFalse(SellerShippingFilter.isAvailable("3002", "3201", null));
    }

    @Test
    void isAvailable_homeDelivery_matchesEmptyGoodsType() {
        // TAQBIN 宅配 — config goodsType="" → key="3001:"
        Set<String> available = Set.of("3001:");
        assertTrue(SellerShippingFilter.isAvailable("3001", "3201", available));
        assertTrue(SellerShippingFilter.isAvailable("3001", "3203", available));
        assertFalse(SellerShippingFilter.isAvailable("3002", "3201", available));
    }

    @Test
    void isAvailable_cvsStorePickup_matchesByTemperature() {
        // 7-11 — goodsType=temperature
        Set<String> available = Set.of("3002:3201"); // 只啟用常溫
        assertTrue(SellerShippingFilter.isAvailable("3002", "3201", available));
        assertFalse(SellerShippingFilter.isAvailable("3002", "3203", available)); // 冷凍未啟用
    }

    @Test
    void isAvailable_disabledShippingMethod_returnsFalse() {
        // 賣家停用 7-11 (3002) 後，3002 不在 available set 中
        Set<String> available = Set.of("3001:"); // 只開宅配
        assertFalse(SellerShippingFilter.isAvailable("3002", "3201", available));
        assertTrue(SellerShippingFilter.isAvailable("3001", "3201", available));
    }

    @Test
    void onboardingSig_deterministicAndStable() {
        long sellerId = 2683382637L;
        Set<String> set1 = Set.of("3001:", "3002:3201");
        Set<String> set2 = Set.of("3002:3201", "3001:"); // 不同順序

        String sig1 = SellerShippingFilter.onboardingSig(sellerId, set1);
        String sig2 = SellerShippingFilter.onboardingSig(sellerId, set2);
        assertEquals(sig1, sig2, "set 順序不影響 sig");
        assertEquals(8, sig1.length(), "sig 為 8 字元 hex");
    }

    @Test
    void onboardingSig_changesWhenToggled() {
        long sellerId = 2683382637L;
        Set<String> beforeToggle = Set.of("3001:", "3002:3201"); // 兩個都開
        Set<String> afterToggle = Set.of("3001:");               // 停用 7-11

        String before = SellerShippingFilter.onboardingSig(sellerId, beforeToggle);
        String after = SellerShippingFilter.onboardingSig(sellerId, afterToggle);
        assertNotEquals(before, after, "toggle 後 sig 必須變動");
    }

    @Test
    void onboardingSig_emptySet() {
        String sig = SellerShippingFilter.onboardingSig(123L, Set.of());
        assertEquals(8, sig.length());
        assertNotNull(sig);
    }

    @Test
    void listOnboardingSig_aggregatesPerSellerSigs() {
        String sigA = SellerShippingFilter.onboardingSig(100L, Set.of("3001:"));
        String sigB = SellerShippingFilter.onboardingSig(200L, Set.of("3002:3201"));

        String listSig1 = SellerShippingFilter.listOnboardingSig(List.of(sigA, sigB));
        String listSig2 = SellerShippingFilter.listOnboardingSig(List.of(sigB, sigA));
        assertEquals(listSig1, listSig2, "order 不影響 list sig");
        assertEquals(8, listSig1.length());
    }

    @Test
    void listOnboardingSig_changesWhenAnySellerToggles() {
        String sigA_before = SellerShippingFilter.onboardingSig(100L, Set.of("3001:", "3002:3201"));
        String sigA_after = SellerShippingFilter.onboardingSig(100L, Set.of("3001:"));
        String sigB = SellerShippingFilter.onboardingSig(200L, Set.of("3002:3201"));

        String before = SellerShippingFilter.listOnboardingSig(List.of(sigA_before, sigB));
        String after = SellerShippingFilter.listOnboardingSig(List.of(sigA_after, sigB));
        assertNotEquals(before, after, "列表內任一 seller toggle 必須讓 list sig 變動");
    }
}
