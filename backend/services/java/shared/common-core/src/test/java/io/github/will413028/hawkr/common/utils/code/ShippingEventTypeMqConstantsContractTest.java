package io.github.will413028.hawkr.common.utils.code;

import io.github.will413028.hawkr.common.mq.MqConstants;
import org.junit.jupiter.api.DisplayName;
import org.junit.jupiter.api.Test;

import java.lang.reflect.Field;
import java.lang.reflect.Modifier;
import java.util.LinkedHashSet;
import java.util.List;
import java.util.Map;
import java.util.Set;
import java.util.TreeMap;
import java.util.TreeSet;
import java.util.stream.Collectors;

import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.junit.jupiter.api.Assertions.assertFalse;
import static org.junit.jupiter.api.Assertions.assertTrue;

/**
 * {@code ShippingEventType} ↔ {@code MqConstants} 的機械檢查。
 *
 * <p><b>它擋什麼</b>：CodeMeta「內部技術碼.md § ShippingEventType」的「機械檢查」欄逐字要求——
 * 「必須有一支斷言『{@code MqConstants} 中所有以 {@code shipment.} 起頭的 routing key 常數，
 * 都在本 enum 的值域內』，缺一即紅」。沒有它，下一個人加常數時 enum 不會跟上，
 * 而症狀是<b>又一條字面值</b>——不會編譯錯、不會測試紅（禁硬編碼的規範靜默失效）。
 *
 * <p><b>兩邊的來源互相獨立，所以這不是自己跟自己對</b>：
 * {@code MqConstants} 是手工維護的 Java 常數；{@code ShippingEventType} 由
 * {@code docs/代碼管理（CodeMeta）/內部技術碼.md} 經 {@code generate.sh} 生成。
 * 兩份清單都以<b>反射</b>就地讀出，本檔<b>不手寫任何一條 routing key 當期望值</b>——
 * 手寫的話，新增常數時這支測試不會轉紅，等於沒有。
 *
 * <p><b>值域邊界（CodeMeta 該 enum 的「值域邊界」欄，三條）</b>：值域錨在<b>契約</b>
 * （routing key ＋ 所屬 exchange），不錨在 topology binding。據此有三條 key <b>刻意不在</b>本 enum：
 * <ul>
 *   <li>{@code order.shipment.create_requested}／{@code order.shipment.creation_failed}
 *       —— publisher 是 order-service、掛 {@code hawkr.order}，屬<b>訂單軸</b>（見 {@code OrderEventType}）。</li>
 *   <li>{@code payuni.shipping.status} —— publisher 是 payuni-webhook-service、掛 {@code payuni.events}，
 *       屬 <b>webhook 領域</b>（見 {@code PayUniEventType}）。</li>
 * </ul>
 * 三者的值都不以 {@code shipment.} 起頭，因此本檔的掃描條件天然排除它們；
 * {@link #keysOwnedByOtherDomainsStayOutOfTheValueDomain()} 另外把這個結論釘住，
 * 免得哪天有人「順手」把它們加進 enum。
 *
 * <p><b>反向也驗</b>：enum 裡有而 {@code MqConstants} 沒有 ⇒ 幽靈值（發不出去的事件）。
 *
 * <p><b>兩軸</b>：{@code ObsRabbitPublisher.publish(exchange, routingKey, body, eventType)} 的
 * routing key 與 event type 是兩個獨立參數，所以 enum 的 {@code value()} 對應
 * {@code ROUTING_KEY_SHIPMENT_*}、{@code code()} 對應 {@code EVENT_TYPE_SHIPMENT_*}，兩軸各驗一次。
 *
 * @see <a href="../../../../../../../../../../docs/詳細設計書/36-MQ 與事件總覽（MQ &amp; Event Catalog）.md">36 號 §3.3／§8.1</a>
 */
class ShippingEventTypeMqConstantsContractTest {

    /**
     * 本領域 routing key 的唯一辨識條件——錨在<b>值</b>不錨在<b>欄位名</b>（欄位名可以亂取，值是契約）。
     *
     * <p>🔴 <b>刻意只有 {@code shipment.} 一個前綴，不得擴成集合。</b>
     * {@code fulfillment.*}（65 的 C 群）屬<b>另一個 owner domain</b>，由
     * {@link FulfillmentEventType} 與 {@code FulfillmentEventTypeMqConstantsContractTest} 負責
     * ——65 附錄 C.4 逐字「不讓一個 enum 同時服務三個 owner」。
     *
     * <p>⚠️ <b>2026-09-16（WP2A）曾把這裡擴成 {@code {shipment., fulfillment.}} 並把 C 群兩條
     * 塞進 {@link ShippingEventType}，已於同波 R1 撤回。</b> 當時的理由是「值域錨在 routing key ＋
     * exchange，而 {@code fulfillment.*} 也掛 {@code hawkr.shipping} ⇒ 該收」——
     * 那個推論只讀了本檔與 CodeMeta 的值域邊界，<b>沒有讀 65 附錄 C.4 對同一件事的定版</b>。
     * ⇒ 同一個問題有兩份 SSOT 時，兩邊都要查。
     */
    private static final String SHIPPING_ROUTING_KEY_PREFIX = "shipment.";

    /** 掃描器的對照組：這一條確知存在，讀不到它就代表掃描器壞了而不是「沒有東西」。 */
    private static final String ANCHOR_ROUTING_KEY = "shipment.status.changed";
    private static final String ANCHOR_FIELD = "ROUTING_KEY_SHIPMENT_STATUS_CHANGED";

    private static final String ROUTING_KEY_PREFIX = "ROUTING_KEY_";
    private static final String EVENT_TYPE_PREFIX = "EVENT_TYPE_";

    /** 明確不屬本領域的三條（理由見類別 javadoc）。 */
    private static final List<String> FOREIGN_DOMAIN_KEYS = List.of(
            "order.shipment.create_requested",
            "order.shipment.creation_failed",
            "payuni.shipping.status");

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

    /** 值以 {@code shipment.} 起頭的常數：欄位名 → 值。 */
    private static Map<String, String> shippingRoutingKeyConstants() {
        return mqStringConstants().entrySet().stream()
                .filter(e -> e.getValue().startsWith(SHIPPING_ROUTING_KEY_PREFIX))
                .collect(Collectors.toMap(Map.Entry::getKey, Map.Entry::getValue,
                        (a, b) -> a, TreeMap::new));
    }

    private static Set<String> enumValues() {
        return java.util.Arrays.stream(ShippingEventType.values())
                .map(ShippingEventType::value)
                .collect(Collectors.toCollection(TreeSet::new));
    }

    // ==================== ① 掃描器自身的鑑別力（fail-closed） ====================

    /**
     * 觀測窗不得為空。反射掃不到東西時，底下每一支斷言都會「空集合 ⊆ 空集合」而全綠——
     * 那正是 {@code .claude/rules/06-no-small-tails.md} §3.10 說的「看不到就放行」。
     * 這一支讓掃描器壞掉時<b>報錯而不是放行</b>。
     */
    @Test
    @DisplayName("掃描器有鑑別力：MqConstants 反射得到常數，且抓得到已知錨點")
    void reflectionScannerHasDiscriminatingPower() {
        Map<String, String> all = mqStringConstants();
        assertFalse(all.isEmpty(), "反射掃不到任何 MqConstants 常數 ⇒ 掃描器壞了，不是『沒有常數』");

        Map<String, String> shipping = shippingRoutingKeyConstants();
        assertFalse(shipping.isEmpty(),
                "MqConstants 掃不到任何以 " + SHIPPING_ROUTING_KEY_PREFIX + " 起頭的值 ⇒ 掃描器壞了");
        assertEquals(ANCHOR_ROUTING_KEY, shipping.get(ANCHOR_FIELD),
                "對照組 " + ANCHOR_FIELD + " 讀不到預期值 ⇒ 掃描條件已與實際常數脫節");

        assertFalse(enumValues().isEmpty(), "ShippingEventType 值域為空 ⇒ 生成檔異常");
    }

    // ==================== ② 正向：常數 → enum 值域 ====================

    /**
     * CodeMeta「機械檢查」欄逐字要求的那一條。
     * <p>失敗的修法：改 {@code docs/代碼管理（CodeMeta）/內部技術碼.md} 的 ShippingEventType 表格，
     * 再跑 {@code docs/代碼管理（CodeMeta）/scripts/generate.sh --all}（禁止手改生成檔）。
     */
    @Test
    @DisplayName("MqConstants 每一條 shipment.* routing key 都在 ShippingEventType 值域內")
    void everyShipmentRoutingKeyConstantIsInTheEnumValueDomain() {
        Map<String, String> missing = new TreeMap<>();
        shippingRoutingKeyConstants().forEach((field, key) -> {
            if (!ShippingEventType.isValidValue(key)) {
                missing.put(field, key);
            }
        });
        assertTrue(missing.isEmpty(),
                "以下 MqConstants 常數不在 ShippingEventType 值域內 ⇒ 又一條字面值："
                        + missing
                        + "。修法：改 內部技術碼.md 的 ShippingEventType 表格後跑 generate.sh --all，"
                        + "禁止手改生成檔。若該 key 真的不屬物流領域，理由要寫進本檔 javadoc 的值域邊界。");
    }

    // ==================== ③ 反向：enum 值域 → 常數（幽靈值） ====================

    @Test
    @DisplayName("ShippingEventType 沒有幽靈值：每個值都有對應的 MqConstants 常數")
    void everyEnumValueHasABackingMqConstant() {
        Set<String> constants = new LinkedHashSet<>(shippingRoutingKeyConstants().values());
        Set<String> ghosts = new TreeSet<>(enumValues());
        ghosts.removeAll(constants);
        assertTrue(ghosts.isEmpty(),
                "以下 ShippingEventType 值在 MqConstants 找不到對應常數 ⇒ 幽靈值（沒有人發得出這條事件）："
                        + ghosts);
    }

    // ==================== ④ Code 軸：event_type header 常數 ====================

    /**
     * {@code publish(exchange, routingKey, body, eventType)} 的第 4 個參數走
     * {@code EVENT_TYPE_*} 家族。enum 的 {@code code()} 若與該常數漂開，
     * consumer 端以 {@code event_type} header 分流的邏輯會靜默錯配。
     * <p>順帶釘住命名慣例：值以 {@code shipment.} 起頭的常數一律叫 {@code ROUTING_KEY_*}，
     * 否則下面的「同名兄弟」推導不成立。
     */
    @Test
    @DisplayName("每個 Code 都等於同名兄弟常數 EVENT_TYPE_* 的值")
    void everyEnumCodeMatchesItsEventTypeSiblingConstant() {
        Map<String, String> all = mqStringConstants();
        Map<String, String> problems = new TreeMap<>();

        shippingRoutingKeyConstants().forEach((field, key) -> {
            if (!field.startsWith(ROUTING_KEY_PREFIX)) {
                problems.put(field, "命名不符慣例：值以 " + SHIPPING_ROUTING_KEY_PREFIX
                        + " 起頭的常數必須叫 " + ROUTING_KEY_PREFIX + "*");
                return;
            }
            if (!ShippingEventType.isValidValue(key)) {
                // 「常數不在 enum 值域」由 everyShipmentRoutingKeyConstantIsInTheEnumValueDomain 負責，
                // 這裡略過，免得同一個缺陷在兩支測試各報一次、還多一個 fromValue 的例外堆疊。
                return;
            }
            String sibling = EVENT_TYPE_PREFIX + field.substring(ROUTING_KEY_PREFIX.length());
            String eventType = all.get(sibling);
            if (eventType == null) {
                problems.put(field, "缺同名兄弟常數 " + sibling
                        + "（publish() 的第 4 個參數不得是行內字面值）");
                return;
            }
            String enumCode = ShippingEventType.fromValue(key).code();
            if (!eventType.equals(enumCode)) {
                problems.put(field, "MqConstants." + sibling + "=\"" + eventType
                        + "\" 與 ShippingEventType.code()=\"" + enumCode + "\" 不一致");
            }
        });

        assertTrue(problems.isEmpty(),
                "Code 軸（event_type header）與 MqConstants 不一致：" + problems);
    }

    // ==================== ⑤ 值域邊界：別的領域的 key 不得混進來 ====================

    /**
     * 這三條長得很像物流，但 producer 與 exchange 都屬別的領域（理由見類別 javadoc）。
     * 它們的值不以 {@code shipment.} 起頭，所以②不會要求它們進 enum；
     * 本支釘住反面：也不准有人主動把它們加進 enum。
     */
    @Test
    @DisplayName("訂單軸／webhook 領域的三條 key 不得出現在 ShippingEventType 值域")
    void keysOwnedByOtherDomainsStayOutOfTheValueDomain() {
        for (String key : FOREIGN_DOMAIN_KEYS) {
            assertFalse(ShippingEventType.isValidValue(key),
                    key + " 不屬物流領域（見本檔 javadoc 的值域邊界）：前兩條 publisher 是 order-service"
                            + "、掛 hawkr.order；payuni.shipping.status 的 publisher 是"
                            + " payuni-webhook-service、掛 payuni.events");
            assertFalse(key.startsWith(SHIPPING_ROUTING_KEY_PREFIX),
                    key + " 的值以 " + SHIPPING_ROUTING_KEY_PREFIX
                            + " 起頭 ⇒ 本檔的掃描條件會把它拉進來，值域邊界必須改寫");
        }
    }
}
