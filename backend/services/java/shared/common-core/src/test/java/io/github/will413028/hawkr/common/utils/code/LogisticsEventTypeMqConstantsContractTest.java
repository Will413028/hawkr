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
 * {@link LogisticsEventType} ↔ {@code MqConstants} 的雙向契約（70 §7.1、36 §3.3）。
 *
 * <p><b>本檔為什麼必須獨立於另外兩支同類測試</b>：三個 family 的值域邊界逐字錨在
 * <b>routing key 前綴</b>——{@link ShippingEventType} ＝ {@code shipment.}、
 * {@link FulfillmentEventType} ＝ {@code fulfillment.}、本家族 ＝ {@code logistics.}。
 * 把 {@code logistics.*} 塞進任一既有家族，會造成 65 附錄 C.4 逐字記載的那個失效模式：
 * <b>值域被撐開、而守門的測試看不到新成員</b>——漏登記常數不會轉紅。
 *
 * <p>⚠️ <b>同 exchange 不是同 family 的理由</b>：三者同掛 {@code hawkr.shipping}
 * （36 §1.4 禁止的是「同一條 routing key 多個 publisher」，不是「一個 exchange 一個 publisher」）。
 * 碼表的邊界錨在 <b>owner domain ＋ routing key 前綴</b>，不是 exchange。
 *
 * <p>🔴 本家族的 producer 是 <b>shipping-service</b>，而 {@code fulfillment.*} 的 producer 是
 * payment-service——<b>publisher 服務不是 family 的判準</b>，兩者剛好相反正好說明這一點。
 *
 * <p>五個軸：① 掃描器鑑別力 ② 常數→值域 ③ 值域→常數（幽靈值）④ Code 軸
 * ⑤ <b>三個</b> family 的值域兩兩互斥。
 */
class LogisticsEventTypeMqConstantsContractTest {

    /** 本領域 routing key 的唯一辨識條件——錨在<b>值</b>不錨在<b>欄位名</b>。 */
    private static final String LOGISTICS_ROUTING_KEY_PREFIX = "logistics.";

    /** 掃描器的對照組：這一條確知存在，讀不到它就代表掃描器壞了而不是「沒有東西」。 */
    private static final String ANCHOR_ROUTING_KEY = "logistics.cost.incurred";
    private static final String ANCHOR_FIELD = "ROUTING_KEY_LOGISTICS_COST_INCURRED";

    private static final String ROUTING_KEY_PREFIX = "ROUTING_KEY_";
    private static final String EVENT_TYPE_PREFIX = "EVENT_TYPE_";

    /**
     * 明確<b>不</b>屬本領域的 key。
     *
     * <p>🔴 {@code fulfillment.responsibility.finalized} 是最重要的一條：它與本家族的事件
     * 在業務上直接相關（兩個事實要 durable join，70 §3.1.4），正因為相關才最容易被
     * 「順手統一」成同一個碼表——而那會讓兩邊的守門測試各自失明。
     */
    private static final List<String> FOREIGN_DOMAIN_KEYS = List.of(
            "fulfillment.responsibility.finalized",
            "fulfillment.responsibility.unresolved",
            "shipment.delivered");

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

    /**
     * 值以 {@code logistics.} 起頭的 <b>routing key</b> 常數：欄位名 → 值。
     *
     * <p>⚠️ <b>必須排除 queue 名</b>：本領域的 consumer queue 叫
     * {@code payment.logistics.cost_incurred}，它不以 {@code logistics.} 起頭所以不會誤入；
     * 但若日後有人取了 {@code logistics.*} 開頭的 queue 名，這裡會把它當成 routing key。
     * 因此額外要求欄位名以 {@code ROUTING_KEY_} 起頭——<b>兩個條件都成立才算</b>。
     */
    private static Map<String, String> logisticsRoutingKeyConstants() {
        return mqStringConstants().entrySet().stream()
                .filter(e -> e.getValue().startsWith(LOGISTICS_ROUTING_KEY_PREFIX))
                .filter(e -> e.getKey().startsWith(ROUTING_KEY_PREFIX))
                .collect(Collectors.toMap(Map.Entry::getKey, Map.Entry::getValue,
                        (a, b) -> a, TreeMap::new));
    }

    private static Set<String> enumValues() {
        return Arrays.stream(LogisticsEventType.values())
                .map(LogisticsEventType::value)
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

        Map<String, String> logistics = logisticsRoutingKeyConstants();
        assertFalse(logistics.isEmpty(),
                "MqConstants 掃不到任何以 " + LOGISTICS_ROUTING_KEY_PREFIX + " 起頭的 routing key ⇒ 掃描器壞了"
                        + "（或 prefix 常數被寫錯——那會讓本檔每一條斷言都自動通過）");
        assertEquals(ANCHOR_ROUTING_KEY, logistics.get(ANCHOR_FIELD),
                "對照組 " + ANCHOR_FIELD + " 讀不到預期值 ⇒ 掃描條件已與實際常數脫節");

        assertFalse(enumValues().isEmpty(), "LogisticsEventType 值域為空 ⇒ 生成檔異常");
    }

    // ==================== ② 正向：常數 → enum 值域 ====================

    /**
     * 失敗的修法：改 {@code docs/代碼管理（CodeMeta）/內部技術碼.md} 的 LogisticsEventType 表格，
     * 再跑 {@code generate.sh --all}（禁止手改生成檔）。
     */
    @Test
    @DisplayName("MqConstants 每一條 logistics.* routing key 都在 LogisticsEventType 值域內")
    void everyLogisticsRoutingKeyConstantIsInTheEnumValueDomain() {
        Map<String, String> missing = new TreeMap<>();
        logisticsRoutingKeyConstants().forEach((field, key) -> {
            if (!LogisticsEventType.isValidValue(key)) {
                missing.put(field, key);
            }
        });
        assertTrue(missing.isEmpty(),
                "以下 MqConstants 常數不在 LogisticsEventType 值域內 ⇒ 又一條字面值：" + missing
                        + "。修法：改 內部技術碼.md 的 LogisticsEventType 表格後跑 generate.sh --all。"
                        + "🔴 不得改塞進 ShippingEventType／FulfillmentEventType"
                        + "——那兩個 enum 的守門測試看不到 logistics. 前綴");
    }

    // ==================== ③ 反向：enum 值域 → 常數（幽靈值） ====================

    /**
     * 幽靈值 ＝ 碼表裡有、卻沒有人發得出這條事件。
     *
     * <p>⚠️ 這一條守住碼表「值域邊界第 2 條」那個決定：線 B 共三條 routing key，
     * 但只有 {@code logistics.cost.incurred} 真的落了碼。若有人為了「文件說有三條」
     * 而把 {@code receivable.settled}／{@code receivable.unconfirmed} 先加進 enum，本測試立刻轉紅。
     */
    @Test
    @DisplayName("LogisticsEventType 沒有幽靈值：每個值都有對應的 MqConstants 常數")
    void everyEnumValueHasABackingMqConstant() {
        Set<String> constants = new LinkedHashSet<>(logisticsRoutingKeyConstants().values());
        Set<String> ghosts = new TreeSet<>(enumValues());
        ghosts.removeAll(constants);
        assertTrue(ghosts.isEmpty(),
                "以下 LogisticsEventType 值在 MqConstants 找不到對應常數 ⇒ 幽靈值"
                        + "（碼表裡有，但沒有人發得出這條事件）：" + ghosts);
    }

    // ==================== ④ Code 軸：event_type header 常數 ====================

    @Test
    @DisplayName("每個 Code 都等於同名兄弟常數 EVENT_TYPE_* 的值")
    void everyEnumCodeMatchesItsEventTypeSiblingConstant() {
        Map<String, String> all = mqStringConstants();
        Map<String, String> problems = new TreeMap<>();

        logisticsRoutingKeyConstants().forEach((field, key) -> {
            if (!LogisticsEventType.isValidValue(key)) {
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
            String enumCode = LogisticsEventType.fromValue(key).code();
            if (!eventType.equals(enumCode)) {
                problems.put(field, "MqConstants." + sibling + "=\"" + eventType
                        + "\" 與 LogisticsEventType.code()=\"" + enumCode + "\" 不一致");
            }
        });

        assertTrue(problems.isEmpty(),
                "Code 軸（event_type header）與 MqConstants 不一致：" + problems);
    }

    // ==================== ⑤ 三個 family 的值域兩兩互斥 ====================

    /**
     * 🔴 <b>本檔最重要的一條</b>：三個 family 同掛 {@code hawkr.shipping}、同屬履約／物流鏈，
     * 而它們的值域<b>必須互斥</b>。
     *
     * <p>⚠️ 本家族與 {@code fulfillment.*} 的關係特別危險：兩者在業務上直接相關
     * （成本事實與責任終局要 durable join，70 §3.1.4），<b>正因為相關才最容易被「順手統一」</b>
     * ——而症狀是守門測試看不到新成員，不會有任何東西出聲。
     */
    @Test
    @DisplayName("三個 EventType family 的值域兩兩互斥（不得有人把它們合併）")
    void theThreeFamiliesDoNotOverlap() {
        Set<String> shipping = Arrays.stream(ShippingEventType.values())
                .map(ShippingEventType::value).collect(Collectors.toCollection(TreeSet::new));
        Set<String> fulfillment = Arrays.stream(FulfillmentEventType.values())
                .map(FulfillmentEventType::value).collect(Collectors.toCollection(TreeSet::new));
        Set<String> logistics = new TreeSet<>(enumValues());

        // 對照組：三邊都非空，否則「沒有交集」是空集合造成的假通過
        assertFalse(shipping.isEmpty(), "ShippingEventType 值域為空 ⇒ 本斷言零鑑別力");
        assertFalse(fulfillment.isEmpty(), "FulfillmentEventType 值域為空 ⇒ 本斷言零鑑別力");
        assertFalse(logistics.isEmpty(), "LogisticsEventType 值域為空 ⇒ 本斷言零鑑別力");

        assertNoOverlap("ShippingEventType", shipping, "LogisticsEventType", logistics);
        assertNoOverlap("FulfillmentEventType", fulfillment, "LogisticsEventType", logistics);
        assertNoOverlap("ShippingEventType", shipping, "FulfillmentEventType", fulfillment);

        // 方向性：本家族只收 logistics.*，別的家族不得出現 logistics.*
        assertTrue(logistics.stream().allMatch(v -> v.startsWith(LOGISTICS_ROUTING_KEY_PREFIX)),
                "LogisticsEventType 出現了非 logistics.* 的值");
        assertTrue(shipping.stream().noneMatch(v -> v.startsWith(LOGISTICS_ROUTING_KEY_PREFIX)),
                "🔴 ShippingEventType 出現了 logistics.* 值 ⇒ 它的守門測試（只掃 shipment.）看不到那些新成員");
        assertTrue(fulfillment.stream().noneMatch(v -> v.startsWith(LOGISTICS_ROUTING_KEY_PREFIX)),
                "🔴 FulfillmentEventType 出現了 logistics.* 值 ⇒ 它的守門測試（只掃 fulfillment.）看不到那些新成員");
    }

    private static void assertNoOverlap(String nameA, Set<String> a, String nameB, Set<String> b) {
        Set<String> overlap = new TreeSet<>(a);
        overlap.retainAll(b);
        assertTrue(overlap.isEmpty(),
                nameA + " 與 " + nameB + " 的值域重疊了：" + overlap
                        + "。65 附錄 C.4 逐字「不讓一個 enum 同時服務三個 owner」");
    }

    /**
     * 別的領域的 key 不得混進本 family——特別是 {@code fulfillment.responsibility.finalized}
     * （業務上直接相關，最容易被合併）。
     */
    @Test
    @DisplayName("fulfillment／shipping 領域的 key 不得出現在 LogisticsEventType 值域")
    void keysOwnedByOtherDomainsStayOutOfTheValueDomain() {
        for (String key : FOREIGN_DOMAIN_KEYS) {
            assertFalse(LogisticsEventType.isValidValue(key),
                    key + " 不屬物流財務 family（見本檔 javadoc 的值域邊界）");
            assertFalse(key.startsWith(LOGISTICS_ROUTING_KEY_PREFIX),
                    key + " 的值以 " + LOGISTICS_ROUTING_KEY_PREFIX
                            + " 起頭 ⇒ 本檔的掃描條件會把它拉進來，值域邊界必須改寫");
        }
    }
}
