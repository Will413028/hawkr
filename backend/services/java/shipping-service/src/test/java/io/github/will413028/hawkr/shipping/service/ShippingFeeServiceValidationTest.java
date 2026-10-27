package io.github.will413028.hawkr.shipping.service;

import io.github.will413028.hawkr.shipping.entity.ShippingMethodConfigEntity;
import io.github.will413028.hawkr.shipping.mapper.ShippingFeeTierMapper;
import io.github.will413028.hawkr.shipping.mapper.ShippingMethodConfigMapper;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.Test;

import java.lang.reflect.Field;
import java.util.HashMap;
import java.util.List;
import java.util.Map;
import java.util.Set;

import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.junit.jupiter.api.Assertions.assertFalse;
import static org.junit.jupiter.api.Assertions.assertTrue;
import static org.mockito.Mockito.mock;

/**
 * ShippingFeeService.validateProductShipping 拒絕原因粒度測試
 *
 * 對應 06 號設計書 §5.17.3 拒絕原因總表：
 *  - METHOD_NOT_ONBOARDED:該 method 完全未開通
 *  - TEMPERATURE_NOT_ONBOARDED:該 method 有開通其他溫層,但本溫層未開通
 *  - METHOD_NOT_SUPPORTED:(method,temperature) 組合本身不被系統支援(如 7-11 冷藏)
 *  - 通過 happy path
 */
class ShippingFeeServiceValidationTest {

    private static final String TAQBIN = "3001";              // 黑貓宅急便
    private static final String SEVEN_ELEVEN = "3002";        // 7-11 店到店
    private static final String UBER = "3003";                // Uber 即時配送 — Phase 1 預留,全系統暫停(無 config row)
    private static final String AMBIENT = "3201";             // 常溫
    private static final String CHILLED = "3202";             // 冷藏 — 7-11 不支援
    private static final String FROZEN = "3203";              // 冷凍

    private ShippingFeeService service;

    @BeforeEach
    void setUp() throws Exception {
        ShippingMethodConfigMapper configMapper = mock(ShippingMethodConfigMapper.class);
        ShippingFeeTierMapper tierMapper = mock(ShippingFeeTierMapper.class);
        service = new ShippingFeeService(configMapper, tierMapper);

        // 直接灌 configCache:7-11 常溫 / 7-11 冷凍 / TAQBIN 全溫層 — 不灌 7-11 冷藏(模擬業務不支援)
        Map<String, ShippingMethodConfigEntity> cache = new HashMap<>();
        cache.put(SEVEN_ELEVEN + ":" + AMBIENT, makeConfig(105, 45, 5000));
        cache.put(SEVEN_ELEVEN + ":" + FROZEN, makeConfig(105, 45, 5000));
        cache.put(TAQBIN + ":" + AMBIENT, makeConfig(150, 100, 20000));
        cache.put(TAQBIN + ":" + CHILLED, makeConfig(150, 100, 20000));
        cache.put(TAQBIN + ":" + FROZEN, makeConfig(150, 100, 20000));
        Field cacheField = ShippingFeeService.class.getDeclaredField("configCache");
        cacheField.setAccessible(true);
        cacheField.set(service, cache);
    }

    @Test
    void temperatureNotOnboarded_messageMentionsBothMethodAndTemperatureLabels() {
        // 賣家只開通 7-11 常溫 → 試上架冷凍 + 7-11
        Set<String> sellerAvailable = Set.of(SEVEN_ELEVEN + ":" + AMBIENT);
        ShippingFeeService.ValidationResult result = service.validateProductShipping(
                List.of(item("冷凍商品", SEVEN_ELEVEN, FROZEN)), sellerAvailable);

        assertFalse(result.allValid());
        ShippingFeeService.ValidationResultItem r = result.results().get(0);
        assertEquals("TEMPERATURE_NOT_ONBOARDED", r.rejectionReason());
        assertTrue(r.rejectionMessage().contains("7-11店到店"), "訊息需含 method label,實際=" + r.rejectionMessage());
        assertTrue(r.rejectionMessage().contains("冷凍"), "訊息需含 temperature label,實際=" + r.rejectionMessage());
        assertFalse(r.rejectionMessage().contains(SEVEN_ELEVEN), "訊息禁止露出 enum value");
        assertFalse(r.rejectionMessage().contains(FROZEN), "訊息禁止露出 enum value");
    }

    @Test
    void methodNotOnboarded_friendlyMessage() {
        // 賣家完全沒開通任何 method
        ShippingFeeService.ValidationResult result = service.validateProductShipping(
                List.of(item("常溫商品", SEVEN_ELEVEN, AMBIENT)), Set.of());

        assertFalse(result.allValid());
        ShippingFeeService.ValidationResultItem r = result.results().get(0);
        assertEquals("METHOD_NOT_ONBOARDED", r.rejectionReason());
        assertTrue(r.rejectionMessage().contains("7-11店到店"));
        assertTrue(r.rejectionMessage().contains("尚未開通"));
        assertTrue(r.rejectionMessage().contains("物流設定"));
    }

    @Test
    void methodNotSupported_combinationIsBusinessInvalid() {
        // 7-11 + 冷藏 業務不支援 — 賣家假設都已開通(雖然開不到),走到 step 2 config check 噴 METHOD_NOT_SUPPORTED
        // 注意 7-11 + 冷藏 sellerAvailable 不會有此項(SLOT_DEFINITIONS 沒列),所以實務上 step 1 已擋
        // 這裡用 sellerAvailableMethods=null 略過 step 1 直接驗 step 2
        ShippingFeeService.ValidationResult result = service.validateProductShipping(
                List.of(item("冷藏商品", SEVEN_ELEVEN, CHILLED)), null);

        assertFalse(result.allValid());
        ShippingFeeService.ValidationResultItem r = result.results().get(0);
        assertEquals("METHOD_NOT_SUPPORTED", r.rejectionReason());
        assertTrue(r.rejectionMessage().contains("7-11店到店"));
        assertTrue(r.rejectionMessage().contains("冷藏"));
    }

    @Test
    void happyPath_allOnboardedAndWithinLimits() {
        Set<String> sellerAvailable = Set.of(SEVEN_ELEVEN + ":" + AMBIENT, TAQBIN + ":");
        ShippingFeeService.ValidationResult result = service.validateProductShipping(
                List.of(item("常溫商品", SEVEN_ELEVEN, AMBIENT)), sellerAvailable);

        assertTrue(result.allValid());
        assertEquals("", result.results().get(0).rejectionReason());
    }

    @Test
    void methodNotAvailable_whenMethodHasNoEnabledConfig() {
        // Uber Phase 1 預留 — configCache 無 3003 任何 row → hasAnyEnabledConfig(UBER)=false
        // 即使賣家假裝開通了,系統層級 step 0 就應該擋下
        Set<String> sellerAvailable = Set.of(UBER + ":" + AMBIENT);  // 故意設「賣家有開通」
        ShippingFeeService.ValidationResult result = service.validateProductShipping(
                List.of(item("Uber商品", UBER, AMBIENT)), sellerAvailable);

        assertFalse(result.allValid());
        ShippingFeeService.ValidationResultItem r = result.results().get(0);
        assertEquals("METHOD_NOT_AVAILABLE", r.rejectionReason());
        assertTrue(r.rejectionMessage().contains("Uber 即時配送"), "訊息需用 method label,實際=" + r.rejectionMessage());
        assertTrue(r.rejectionMessage().contains("暫停"), "訊息需明示『暫停使用』,實際=" + r.rejectionMessage());
        assertFalse(r.rejectionMessage().contains("尚未開通"), "不可顯示『尚未開通』(那是賣家責任語意),實際=" + r.rejectionMessage());
    }

    @Test
    void methodNotAvailable_takesPrecedenceOverOnboardingChecks() {
        // 系統層級暫停的訊息應該優先於賣家 onboarding 訊息
        ShippingFeeService.ValidationResult result = service.validateProductShipping(
                List.of(item("Uber商品", UBER, FROZEN)), null);  // sellerAvailableMethods=null 也擋
        assertFalse(result.allValid());
        assertEquals("METHOD_NOT_AVAILABLE", result.results().get(0).rejectionReason());
    }

    @Test
    void taqbinUsesEmptyTempInKey_andStillFriendlyMessage() {
        // 黑貓 key 固定 "TAQBIN:" — 不分溫層
        // 賣家有 TAQBIN: 則任何溫層都通過 step 1
        Set<String> sellerAvailable = Set.of(TAQBIN + ":");
        ShippingFeeService.ValidationResult result = service.validateProductShipping(
                List.of(item("冷凍商品", TAQBIN, FROZEN)), sellerAvailable);

        assertTrue(result.allValid());

        // 反向:賣家沒開通 TAQBIN → METHOD_NOT_ONBOARDED + 黑貓 label
        ShippingFeeService.ValidationResult result2 = service.validateProductShipping(
                List.of(item("常溫商品", TAQBIN, AMBIENT)), Set.of(SEVEN_ELEVEN + ":" + AMBIENT));
        assertFalse(result2.allValid());
        assertEquals("METHOD_NOT_ONBOARDED", result2.results().get(0).rejectionReason());
        assertTrue(result2.results().get(0).rejectionMessage().contains("黑貓宅急便"));
    }

    private ShippingFeeService.ValidationItem item(String name, String method, String temp) {
        return new ShippingFeeService.ValidationItem(name, method, temp, 30, 20, 10, 1000);
    }

    private ShippingMethodConfigEntity makeConfig(int maxDim, int maxSide, long maxWeight) {
        ShippingMethodConfigEntity c = new ShippingMethodConfigEntity();
        c.setMaxDimensionSumCm(maxDim);
        c.setMaxSingleSideCm(maxSide);
        c.setMaxWeightGrams(maxWeight);
        c.setEstimatedDays(3);
        c.setSettlementDays(7);
        return c;
    }
}
