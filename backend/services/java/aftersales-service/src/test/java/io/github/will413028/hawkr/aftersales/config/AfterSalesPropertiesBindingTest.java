package io.github.will413028.hawkr.aftersales.config;

import org.junit.jupiter.api.DisplayName;
import org.junit.jupiter.api.Test;
import org.yaml.snakeyaml.Yaml;

import java.io.InputStream;
import java.lang.reflect.Field;
import java.util.ArrayList;
import java.util.HashMap;
import java.util.List;
import java.util.Map;

import static org.junit.jupiter.api.Assertions.assertTrue;

/**
 * 守「application.yml 的 aftersales.* 每個 key 都真的綁得上 {@link AfterSalesProperties} 的欄位」。
 *
 * <p>🔴 為什麼需要這支：Spring 的 relaxed binding 對綁不上的 key <b>完全沉默</b>——
 * 不警告、不失敗，該設定就靜靜地走 Java 欄位預設。2026-09-08 實查發現兩個實例：
 * <ul>
 *   <li>{@code case:} 綁不上 {@code caseConfig}（只接受 case-config／caseConfig／case_config）</li>
 *   <li>{@code return:} 綁不上 {@code returnConfig}</li>
 * </ul>
 * 兩者自 {@code b2695b1c4} 寫下起從未生效。dev 實證：11 筆自動駁回的間隔正好 72.00 小時
 * （Java 預設），而 yml／k8s values／.env.template 三處都寫著 24。
 *
 * <p>⚠️ <b>只比對第一層是不夠的</b>：真正被讀的值在第二層，只守一半等於沒守。
 * 本測試遞迴到第二層（inner class 的欄位）。
 *
 * <p>⚠️ 它守不住什麼（誠實標明）：<b>值</b>對不對它一概不管，只管「這個 key 有沒有接收者」。
 * 值的正確性由各自的業務測試與設計書負責。
 */
class AfterSalesPropertiesBindingTest {

    /** relaxed binding 的正規化：忽略連字號、底線與大小寫。 */
    private static String norm(String s) {
        return s.replace("-", "").replace("_", "").toLowerCase();
    }

    private static Map<String, Field> fieldsOf(Class<?> type) {
        Map<String, Field> m = new HashMap<>();
        for (Field f : type.getDeclaredFields()) {
            if (!f.isSynthetic()) {
                m.put(norm(f.getName()), f);
            }
        }
        return m;
    }

    @Test
    @DisplayName("application.yml 的 aftersales.* 每個 key（含第二層）都必須綁得上欄位")
    @SuppressWarnings("unchecked")
    void everyYamlKeyBindsToAField() throws Exception {
        Map<String, Object> aftersales = new HashMap<>();
        try (InputStream in = getClass().getResourceAsStream("/application.yml")) {
            assertTrue(in != null, "讀不到 application.yml——測試本身失效了，不是設定沒問題");
            for (Object doc : new Yaml().loadAll(in)) {
                if (doc instanceof Map<?, ?> m && m.get("aftersales") instanceof Map<?, ?> a) {
                    aftersales.putAll((Map<String, Object>) a);
                }
            }
        }
        // 對照組：讀不到任何 key 的話，下面的迴圈會空轉而「通過」——那是假綠。
        assertTrue(aftersales.size() >= 3,
                "aftersales.* 只讀到 " + aftersales.size() + " 個 key，查詢本身可疑");

        Map<String, Field> top = fieldsOf(AfterSalesProperties.class);
        List<String> unbound = new ArrayList<>();
        for (Map.Entry<String, Object> e : aftersales.entrySet()) {
            Field f = top.get(norm(e.getKey()));
            if (f == null) {
                unbound.add("aftersales." + e.getKey());
                continue;
            }
            if (e.getValue() instanceof Map<?, ?> sub) {
                Map<String, Field> inner = fieldsOf(f.getType());
                for (Object k : sub.keySet()) {
                    if (inner.get(norm(String.valueOf(k))) == null) {
                        unbound.add("aftersales." + e.getKey() + "." + k);
                    }
                }
            }
        }
        assertTrue(unbound.isEmpty(),
                "這些 key 綁不上任何欄位，會靜默走 Java 預設：" + unbound
                        + "（Spring relaxed binding 只接受 kebab-case／camelCase／snake_case 的同名變體）");
    }
}
