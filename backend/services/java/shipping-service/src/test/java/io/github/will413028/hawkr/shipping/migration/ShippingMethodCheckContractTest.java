package io.github.will413028.hawkr.shipping.migration;

import io.github.will413028.hawkr.common.utils.code.ShippingMethod;
import org.junit.jupiter.api.DisplayName;
import org.junit.jupiter.api.Test;

import java.io.IOException;
import java.io.InputStream;
import java.nio.charset.StandardCharsets;
import java.util.Arrays;
import java.util.Set;
import java.util.TreeSet;
import java.util.regex.Matcher;
import java.util.regex.Pattern;

import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.junit.jupiter.api.Assertions.assertTrue;

/**
 * {@code shipments.chk_shipping_method} 的值域必須等於 {@link ShippingMethod} 的 value。
 *
 * <p>檢查的是現行約束的最後一次定義（{@code V15__drop_shipping_method_3004.sql}）；
 * 之後若再改這條 CHECK，要把本測試指向新的 migration。
 * 這是對 SQL 字面的斷言，不是對 PostgreSQL 執行結果的斷言（本模組沒有 DB 測試基礎設施）。
 */
@DisplayName("shipments.chk_shipping_method 契約")
class ShippingMethodCheckContractTest {

    private static final String MIGRATION = "db/migration/V15__drop_shipping_method_3004.sql";

    private static final Pattern CHECK = Pattern.compile(
            "ADD\\s+CONSTRAINT\\s+chk_shipping_method\\s+CHECK\\s*\\(\\s*shipping_method\\s+IN\\s*\\(([^)]*)\\)",
            Pattern.CASE_INSENSITIVE);

    @Test
    @DisplayName("CHECK 的值域恰為 3001／3002／3003／3005，且等於 ShippingMethod 全部 value")
    void checkValuesEqualShippingMethodValues() throws IOException {
        Set<String> inCheck = checkValues();

        // 硬寫字面值：期望值若來自被測檔案或 enum 自己，這條就只是在跟自己對。
        assertEquals(Set.of("3001", "3002", "3003", "3005"), inCheck);

        Set<String> inEnum = new TreeSet<>();
        for (ShippingMethod m : ShippingMethod.values()) {
            inEnum.add(m.value());
        }
        assertEquals(inEnum, inCheck, "ShippingMethod 與 DB CHECK 值域漂移");
        assertTrue(!inCheck.contains("3004"), "3004 已刪除，不得出現在 CHECK");
    }

    private static Set<String> checkValues() throws IOException {
        try (InputStream in = ShippingMethodCheckContractTest.class.getClassLoader().getResourceAsStream(MIGRATION)) {
            assertTrue(in != null, MIGRATION + " 不存在");
            String sql = stripComments(new String(in.readAllBytes(), StandardCharsets.UTF_8));
            Matcher m = CHECK.matcher(sql);
            assertTrue(m.find(), "找不到 chk_shipping_method 的 CHECK 定義");
            Set<String> values = new TreeSet<>();
            Arrays.stream(m.group(1).split(",")).map(String::trim)
                    .map(v -> v.replace("'", "")).forEach(values::add);
            return values;
        }
    }

    private static String stripComments(String sql) {
        StringBuilder out = new StringBuilder();
        for (String line : sql.split("\n")) {
            int i = line.indexOf("--");
            out.append(i >= 0 ? line.substring(0, i) : line).append('\n');
        }
        return out.toString();
    }
}
