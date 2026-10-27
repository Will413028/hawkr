package io.github.will413028.hawkr.common.utils.code;

import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.junit.jupiter.api.Assertions.assertFalse;
import static org.junit.jupiter.api.Assertions.assertTrue;

import io.github.will413028.hawkr.common.mq.MqConstants;
import java.lang.reflect.Field;
import java.lang.reflect.Modifier;
import java.util.Arrays;
import java.util.LinkedHashSet;
import java.util.List;
import java.util.Map;
import java.util.Set;
import java.util.TreeMap;
import java.util.TreeSet;
import java.util.stream.Collectors;
import org.junit.jupiter.api.DisplayName;
import org.junit.jupiter.api.Test;

/**
 * {@link FulfillmentEventType} ↔ {@code MqConstants} 的雙向契約（65 附錄 C.3／C.4、36 §3.3）。
 *
 * <p><b>本檔為什麼必須獨立於 {@code ShippingEventTypeMqConstantsContractTest}</b>：
 * 65 附錄 C.4 把 {@link ShippingEventType} 的值域定為「routing key 以 {@code shipment.} 起頭」的 9 條，
 * 理由逐字是「<b>不讓一個 enum 同時服務三個 owner</b>」。{@code fulfillment.*} 是另一個 owner domain。
 *
 * <p>⚠️ <b>更關鍵的是守門測試的鑑別力</b>：那一支的掃描面逐字是「值以 {@code shipment.} 起頭的常數」
 * ⇒ 把 {@code fulfillment.*} 硬塞進 {@code ShippingEventType}，會造成
 * 「<b>值域被撐開、守門的測試卻看不到新成員</b>」——漏登記常數不會轉紅。
 * 65 §6A.1.2 對這一點有逐字記載。
 *
 * <p>⚠️ <b>同 exchange 不是同 family 的理由</b>：{@code hawkr.shipping} 上可以有多個 publisher
 * （36 §1.4 禁止的是「同一條 routing key 多個 publisher」，不是「一個 exchange 一個 publisher」）。
 * 碼表的邊界錨在 <b>owner domain ＋ routing key 前綴</b>，不是 exchange。
 *
 * <p>五個軸，與 shipping 那一支同構：① 掃描器鑑別力 ② 常數→值域 ③ 值域→常數（幽靈值）
 * ④ Code 軸（{@code EVENT_TYPE_*} 同名兄弟）⑤ 兩個 family 的值域<b>不得重疊</b>。
 */
class FulfillmentEventTypeMqConstantsContractTest {

    /** 本領域 routing key 的唯一辨識條件——錨在<b>值</b>不錨在<b>欄位名</b>。 */
    private static final String FULFILLMENT_ROUTING_KEY_PREFIX = "fulfillment.";

    /** 掃描器的對照組：這一條確知存在，讀不到它就代表掃描器壞了而不是「沒有東西」。 */
    private static final String ANCHOR_ROUTING_KEY = "fulfillment.responsibility.finalized";
    private static final String ANCHOR_FIELD = "ROUTING_KEY_FULFILLMENT_RESPONSIBILITY_FINALIZED";

    private static final String ROUTING_KEY_PREFIX = "ROUTING_KEY_";
    private static final String EVENT_TYPE_PREFIX = "EVENT_TYPE_";

    /**
     * 明確<b>不</b>屬本領域的 key。
     *
     * <p>🔴 {@code shipment.*} 是最重要的一組：它們與本領域同 exchange、同樣是履約鏈的一部分，
     * 但屬 {@link ShippingEventType} 的 owner domain。少了這一條反向釘樁，
     * 哪天有人「順手統一」把兩邊合併，不會有任何東西出聲。
     */
    private static final List<String> FOREIGN_DOMAIN_KEYS = List.of(
            "shipment.status.changed",
            "shipment.delivered",
            "order.shipment.create_requested");

    /** {@code MqConstants} 全部 {@code public static final String} 常數：欄位名 → 值。 */
    private static Map<String, String> mqStringConstants() {
        Map<String, String> found = new TreeMap<>();
        for (Field f : MqConstants.class.getDeclaredFields()) {
            if (!Modifier.isStatic(f.getModifiers()) || !Modifier.isFinal(f.getModifiers())) {
                continue;
            }
            if (f.getType() != String.class) {
                continue;
            }
            try {
                Object v = f.get(null);
                if (v != null) {
                    found.put(f.getName(), (String) v);
                }
            } catch (IllegalAccessException e) {
                throw new AssertionError("讀不到 MqConstants." + f.getName(), e);
            }
        }
        return found;
    }

    /** 值以 {@code fulfillment.} 起頭的常數：欄位名 → 值。 */
    private static Map<String, String> fulfillmentRoutingKeyConstants() {
        return mqStringConstants().entrySet().stream()
                .filter(e -> e.getValue().startsWith(FULFILLMENT_ROUTING_KEY_PREFIX))
                .collect(Collectors.toMap(Map.Entry::getKey, Map.Entry::getValue,
                        (a, b) -> a, TreeMap::new));
    }

    private static Set<String> enumValues() {
        return Arrays.stream(FulfillmentEventType.values())
                .map(FulfillmentEventType::value)
                .collect(Collectors.toCollection(LinkedHashSet::new));
    }

    // ==================== ① 掃描器自身的鑑別力 ====================

    /**
     * 🔴 <b>先證明量測有效，再談它量到什麼</b>：反射掃不到東西、或 prefix 寫錯，
     * 下面每一條「都在值域內」的斷言都會<b>自動通過</b>，而那看起來與真的通過一模一樣。
     */
    @Test
    @DisplayName("掃描器有鑑別力：MqConstants 反射得到常數，且抓得到已知錨點")
    void scannerHasDiscriminatingPower() {
        Map<String, String> all = mqStringConstants();
        assertFalse(all.isEmpty(), "反射掃不到任何 MqConstants 常數 ⇒ 掃描器壞了，不是『沒有常數』");

        Map<String, String> fulfillment = fulfillmentRoutingKeyConstants();
        assertFalse(fulfillment.isEmpty(),
                "MqConstants 掃不到任何以 " + FULFILLMENT_ROUTING_KEY_PREFIX + " 起頭的值 ⇒ 掃描器壞了"
                        + "（或 prefix 常數被寫錯——那會讓本檔每一條斷言都自動通過）");
        assertEquals(ANCHOR_ROUTING_KEY, fulfillment.get(ANCHOR_FIELD),
                "對照組 " + ANCHOR_FIELD + " 讀不到預期值 ⇒ 掃描條件已與實際常數脫節");

        assertFalse(enumValues().isEmpty(), "FulfillmentEventType 值域為空 ⇒ 生成檔異常");
    }

    // ==================== ② 正向：常數 → enum 值域 ====================

    /**
     * 失敗的修法：改 {@code docs/代碼管理（CodeMeta）/內部技術碼.md} 的 FulfillmentEventType 表格，
     * 再跑 {@code generate.sh --all}（禁止手改生成檔）。
     */
    @Test
    @DisplayName("MqConstants 每一條 fulfillment.* routing key 都在 FulfillmentEventType 值域內")
    void everyFulfillmentRoutingKeyConstantIsInTheEnumValueDomain() {
        Map<String, String> missing = new TreeMap<>();
        fulfillmentRoutingKeyConstants().forEach((field, key) -> {
            if (!FulfillmentEventType.isValidValue(key)) {
                missing.put(field, key);
            }
        });
        assertTrue(missing.isEmpty(),
                "以下 MqConstants 常數不在 FulfillmentEventType 值域內 ⇒ 又一條字面值：" + missing
                        + "。修法：改 內部技術碼.md 的 FulfillmentEventType 表格後跑 generate.sh --all。"
                        + "🔴 不得改塞進 ShippingEventType——那個 enum 的守門測試看不到 fulfillment. 前綴");
    }

    // ==================== ③ 反向：enum 值域 → 常數（幽靈值） ====================

    /**
     * 幽靈值 ＝ 碼表裡有、卻沒有人發得出這條事件。
     *
     * <p>⚠️ 這一條同時守住 65 §6A.1.2 C 群「七條只收已落碼的兩條」那個決定：
     * 若有人為了「文件說有七條」而把 {@code N11}～{@code N15} 先加進 enum，本測試會立刻轉紅。
     */
    @Test
    @DisplayName("FulfillmentEventType 沒有幽靈值：每個值都有對應的 MqConstants 常數")
    void everyEnumValueHasABackingMqConstant() {
        Set<String> constants = new LinkedHashSet<>(fulfillmentRoutingKeyConstants().values());
        Set<String> ghosts = new TreeSet<>(enumValues());
        ghosts.removeAll(constants);
        assertTrue(ghosts.isEmpty(),
                "以下 FulfillmentEventType 值在 MqConstants 找不到對應常數 ⇒ 幽靈值"
                        + "（碼表裡有，但沒有人發得出這條事件）：" + ghosts);
    }

    // ==================== ④ Code 軸：event_type header 常數 ====================

    @Test
    @DisplayName("每個 Code 都等於同名兄弟常數 EVENT_TYPE_* 的值")
    void everyEnumCodeMatchesItsEventTypeSiblingConstant() {
        Map<String, String> all = mqStringConstants();
        Map<String, String> problems = new TreeMap<>();

        fulfillmentRoutingKeyConstants().forEach((field, key) -> {
            if (!field.startsWith(ROUTING_KEY_PREFIX)) {
                problems.put(field, "命名不符慣例：值以 " + FULFILLMENT_ROUTING_KEY_PREFIX
                        + " 起頭的常數必須叫 " + ROUTING_KEY_PREFIX + "*");
                return;
            }
            if (!FulfillmentEventType.isValidValue(key)) {
                // 「常數不在 enum 值域」由 ② 負責，這裡略過免得同一個缺陷報兩次。
                return;
            }
            String sibling = EVENT_TYPE_PREFIX + field.substring(ROUTING_KEY_PREFIX.length());
            String eventType = all.get(sibling);
            if (eventType == null) {
                problems.put(field, "缺同名兄弟常數 " + sibling
                        + "（event_type header 不得是行內字面值）");
                return;
            }
            String enumCode = FulfillmentEventType.fromValue(key).code();
            if (!eventType.equals(enumCode)) {
                problems.put(field, "MqConstants." + sibling + "=\"" + eventType
                        + "\" 與 FulfillmentEventType.code()=\"" + enumCode + "\" 不一致");
            }
        });

        assertTrue(problems.isEmpty(),
                "Code 軸（event_type header）與 MqConstants 不一致：" + problems);
    }

    // ==================== ⑤ 兩個 family 的值域不得重疊 ====================

    /**
     * 🔴 <b>本檔最重要的一條</b>：{@link ShippingEventType} 與 {@link FulfillmentEventType}
     * 同掛 {@code hawkr.shipping}、同屬履約鏈，而它們的值域<b>必須互斥</b>。
     *
     * <p>2026-09-16（WP2A）曾把 C 群兩條塞進 {@code ShippingEventType} 並把該處的掃描器擴成
     * 兩個前綴，已於同波 R1 撤回。少了這一條釘樁，同樣的「順手統一」會再發生一次，
     * 而症狀是<b>守門測試看不到新成員</b>——不會有任何東西出聲。
     */
    @Test
    @DisplayName("ShippingEventType 與 FulfillmentEventType 的值域互斥（不得有人再把兩者合併）")
    void theTwoFamiliesDoNotOverlap() {
        Set<String> shipping = Arrays.stream(ShippingEventType.values())
                .map(ShippingEventType::value).collect(Collectors.toCollection(TreeSet::new));
        Set<String> fulfillment = new TreeSet<>(enumValues());

        // 對照組：兩邊都非空，否則「沒有交集」是空集合造成的假通過
        assertFalse(shipping.isEmpty(), "ShippingEventType 值域為空 ⇒ 本斷言零鑑別力");
        assertFalse(fulfillment.isEmpty(), "FulfillmentEventType 值域為空 ⇒ 本斷言零鑑別力");

        Set<String> overlap = new TreeSet<>(shipping);
        overlap.retainAll(fulfillment);
        assertTrue(overlap.isEmpty(),
                "兩個 EventType family 的值域重疊了：" + overlap
                        + "。65 附錄 C.4 逐字「不讓一個 enum 同時服務三個 owner」");

        // 方向性：shipping 不得含 fulfillment. 前綴，反之亦然
        assertTrue(shipping.stream().noneMatch(v -> v.startsWith(FULFILLMENT_ROUTING_KEY_PREFIX)),
                "🔴 ShippingEventType 出現了 fulfillment.* 值 ⇒ 它的守門測試（只掃 shipment.）看不到那些新成員");
        assertTrue(fulfillment.stream().allMatch(v -> v.startsWith(FULFILLMENT_ROUTING_KEY_PREFIX)),
                "FulfillmentEventType 出現了非 fulfillment.* 的值");
    }

    /**
     * 別的領域的 key 不得混進本 family——特別是 {@code shipment.*}（同 exchange、同履約鏈）。
     */
    @Test
    @DisplayName("shipping／order 領域的 key 不得出現在 FulfillmentEventType 值域")
    void keysOwnedByOtherDomainsStayOutOfTheValueDomain() {
        for (String key : FOREIGN_DOMAIN_KEYS) {
            assertFalse(FulfillmentEventType.isValidValue(key),
                    key + " 不屬履約 family（見本檔 javadoc 的值域邊界）");
            assertFalse(key.startsWith(FULFILLMENT_ROUTING_KEY_PREFIX),
                    key + " 的值以 " + FULFILLMENT_ROUTING_KEY_PREFIX
                            + " 起頭 ⇒ 本檔的掃描條件會把它拉進來，值域邊界必須改寫");
        }
    }
}
