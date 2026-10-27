package io.github.will413028.hawkr.payment.logistics.domain;

import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.junit.jupiter.api.Assertions.assertFalse;
import static org.junit.jupiter.api.Assertions.assertNotNull;
import static org.junit.jupiter.api.Assertions.assertThrows;
import static org.junit.jupiter.api.Assertions.assertTrue;

import org.junit.jupiter.api.Test;

import io.github.will413028.hawkr.common.utils.code.ResponsibilityCause;

/** 驗 ADR-004 的責任歸屬表。期望值硬寫，不從被測物反推。 */
class ResponsibilityRulesTest {

    @Test
    void sellerCausedIsBorneBySeller() {
        assertEquals(Bearer.SELLER, ResponsibilityRules.bearerOf(ResponsibilityCause.SELLER_CAUSED));
    }

    @Test
    void buyerCausedIsAlsoBorneBySeller() {
        // 🔴 這一格最容易被直覺讀反：買家已經完成一次付款，平台**不得**向買家二次收款
        //    （ADR-004）⇒ 買家原因造成的物流額外成本仍歸賣家的物流成本。
        assertEquals(Bearer.SELLER, ResponsibilityRules.bearerOf(ResponsibilityCause.BUYER_CAUSED));
    }

    @Test
    void platformCausedIsBorneByPlatform() {
        // 🔴 不得轉嫁賣家。
        assertEquals(Bearer.PLATFORM, ResponsibilityRules.bearerOf(ResponsibilityCause.PLATFORM_CAUSED));
    }

    @Test
    void providerCausedGoesToDispute() {
        // 🔴 判定原料不足時的 fail-closed 落點也是這一格；**不得自動向賣家收**。
        assertEquals(Bearer.DISPUTE, ResponsibilityRules.bearerOf(ResponsibilityCause.PROVIDER_CAUSED));
    }

    @Test
    void onlySellerBorneCausesAreChargeableToSeller() {
        // 陰性驗證才是這條規則的實質內容：「先進爭議」如果沒有人擋住自動收款，
        // 那條 fail-closed 等於不存在。
        assertTrue(ResponsibilityRules.chargeableToSeller(ResponsibilityCause.SELLER_CAUSED));
        assertTrue(ResponsibilityRules.chargeableToSeller(ResponsibilityCause.BUYER_CAUSED));
        assertFalse(ResponsibilityRules.chargeableToSeller(ResponsibilityCause.PLATFORM_CAUSED));
        assertFalse(ResponsibilityRules.chargeableToSeller(ResponsibilityCause.PROVIDER_CAUSED));
    }

    @Test
    void tableIsExhaustive() {
        // 🔴 新增成因而未指派負擔方 ⇒ 編譯期就失敗（沒有 default 的 switch expression）。
        //    本測試守的是另一半：現有的每個成因都真的有答案，不會回 null。
        for (ResponsibilityCause c : ResponsibilityCause.values()) {
            assertNotNull(ResponsibilityRules.bearerOf(c), c.value() + " 沒有指派負擔方");
        }
        assertEquals(4, ResponsibilityCause.values().length);
    }

    @Test
    void nullFailsLoud() {
        assertThrows(IllegalArgumentException.class, () -> ResponsibilityRules.bearerOf(null));
    }

    @Test
    void causeValueDomainMatchesTheDbCheckConstraint() {
        // 對照 V9 的 chk_logadj_cause。
        assertEquals(
                java.util.List.of("SELLER_CAUSED", "BUYER_CAUSED", "PLATFORM_CAUSED", "PROVIDER_CAUSED"),
                java.util.Arrays.stream(ResponsibilityCause.values())
                        .map(ResponsibilityCause::value)
                        .toList());
    }
}
