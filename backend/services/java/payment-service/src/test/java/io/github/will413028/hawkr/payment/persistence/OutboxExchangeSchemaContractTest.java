package io.github.will413028.hawkr.payment.persistence;

import io.github.will413028.hawkr.payment.infrastructure.persistence.entity.OutboxEventEntity;
import org.junit.jupiter.api.DisplayName;
import org.junit.jupiter.api.Test;

import java.io.IOException;
import java.io.InputStream;
import java.lang.reflect.Field;
import java.nio.charset.StandardCharsets;
import java.nio.file.Files;
import java.nio.file.Path;
import java.util.Arrays;
import java.util.Set;
import java.util.stream.Collectors;

import static org.junit.jupiter.api.Assertions.assertFalse;
import static org.junit.jupiter.api.Assertions.assertTrue;

/**
 * {@code V11__add_outbox_events_exchange.sql} ↔ {@link OutboxEventEntity} 的 schema 契約釘樁
 * （36 §3.3.1 / 65 測試項 ③，形狀照抄 {@code LogisticsSchemaContractTest}）。
 *
 * <p><b>它證明什麼</b>：DDL 上真的有 {@code exchange} 欄、寬度與 DEFAULT 與契約一致、
 * DB 層擋得住 whitespace-only，而且 entity 側有對應欄位。
 *
 * <p><b>它證明不了什麼（誠實標註觀測窗，§3.10）</b>：
 * <ul>
 *   <li>不驗 migration 真的套得起來、也不驗既有列回填——那要真 DB。
 *       本輪以 {@code postgres:17-alpine} 依序套 V1..V11 實測（回填、滾動更新相容、
 *       7 個約束變異全部正確），證據留在 WP1 報告。</li>
 *   <li>{@link #stripLineComments} 只處理 {@code --} 行註解，不認得字串字面值裡的 {@code --}。
 *       本檔的斷言都跑在剝乾淨的視圖上；V11 沒有含 {@code --} 的字串字面值。</li>
 * </ul>
 */
@DisplayName("V11 outbox exchange schema ↔ entity 契約")
class OutboxExchangeSchemaContractTest {

    private static final String V11_RESOURCE = "db/migration/V11__add_outbox_events_exchange.sql";
    private static final String V11_PATH = "src/main/resources/" + V11_RESOURCE;
    private static final String V1_RESOURCE = "db/migration/V1__init_payment_tables.sql";
    private static final String V1_PATH = "src/main/resources/" + V1_RESOURCE;

    /** 剝掉 {@code --} 行註解的純 DDL 視圖，空白壓成單一空格。 */
    private static final String V11 = flatten(stripLineComments(read(V11_PATH, V11_RESOURCE)));
    private static final String V1 = flatten(stripLineComments(read(V1_PATH, V1_RESOURCE)));

    // ─────────────────────────── 讀檔 ───────────────────────────

    private static String read(String fsPath, String resource) {
        Path p = Path.of(fsPath);
        if (Files.isRegularFile(p)) {
            try {
                return Files.readString(p, StandardCharsets.UTF_8);
            } catch (IOException e) {
                throw new IllegalStateException("讀不到 " + fsPath, e);
            }
        }
        try (InputStream in = OutboxExchangeSchemaContractTest.class.getClassLoader()
                .getResourceAsStream(resource)) {
            if (in == null) {
                // 🔴 對「目標不存在」必須 FAIL 而不是 SKIP：migration 被改名或搬走時，
                //    SKIP 會讓這支測試靜默轉綠，而那正是它該叫的時候。
                throw new IllegalStateException(
                        "找不到 migration（檔案系統 " + fsPath + " 與 classpath " + resource
                                + " 都沒有）。本測試拒絕在看不到目標時通過。");
            }
            return new String(in.readAllBytes(), StandardCharsets.UTF_8);
        } catch (IOException e) {
            throw new IllegalStateException("讀不到 " + resource, e);
        }
    }

    private static String stripLineComments(String raw) {
        return Arrays.stream(raw.split("\n"))
                .map(line -> {
                    int i = line.indexOf("--");
                    return i < 0 ? line : line.substring(0, i);
                })
                .collect(Collectors.joining("\n"));
    }

    private static String flatten(String s) {
        return s.replaceAll("\\s+", " ");
    }

    // ─────────────────────────── 斷言 ───────────────────────────

    @Test
    @DisplayName("V11 加的是 exchange VARCHAR(64) NOT NULL DEFAULT 'payment.events'")
    void addsExchangeColumnWithContractShape() {
        assertTrue(V11.contains("ALTER TABLE outbox_events ADD COLUMN exchange VARCHAR(64) NOT NULL DEFAULT 'payment.events'"),
                "欄位宣告與 36 §3.3.1 契約不符。實際（剝註解後）：" + V11);
    }

    /**
     * DEFAULT 是滾動更新相容所需（舊 pod 的 INSERT 不帶這一欄），
     * 不是給新 code 當後路——後者由 factory 的必填參數與 publisher 的 fail-closed 各自擋住。
     * 這一支釘住「DEFAULT 值必須正好是既有 code 唯一會發的那個 exchange」。
     */
    @Test
    @DisplayName("DEFAULT 必須是 payment.events（舊 pod 寫入的語意正確值）")
    void defaultIsPaymentEvents() {
        assertTrue(V11.contains("DEFAULT 'payment.events'"), "DEFAULT 值不是 payment.events");
        assertFalse(V11.contains("DEFAULT 'hawkr.shipping'"),
                "DEFAULT 不得是 shipping：舊 pod 的寫入會被填成錯的 exchange");
    }

    /**
     * 🔴 本輪實測發現的缺陷防回歸：{@code btrim()} 預設只剝<b>空格</b>，
     * 由 tab／換行組成的 exchange 會穿過 {@code btrim(exchange) <> ''}。
     * 改用 {@code exchange ~ '\S'}（至少一個非空白字元）才涵蓋 space/tab/CR/LF/FF/VT。
     */
    @Test
    @DisplayName("CHECK 用 ~ '\\S' 而非 btrim（btrim 版本讓 tab/換行穿過去）")
    void checkConstraintRejectsAllWhitespace() {
        assertTrue(V11.contains("ADD CONSTRAINT chk_payment_outbox_exchange CHECK (exchange ~ '\\S')"),
                "CHECK 述詞與契約不符。實際（剝註解後）：" + V11);
        assertFalse(V11.contains("btrim(exchange)"),
                "不得退回 btrim 版本：tab／換行組成的值會通過（本輪實測坐實）");
    }

    /**
     * 對照組：證明上面的查詢真的看得到東西，而不是在一個空字串上做「不包含」斷言。
     * V1 建表時沒有 exchange 欄 ⇒ 它確實是 V11 才加的。
     */
    @Test
    @DisplayName("對照組：V1 的建表沒有 exchange 欄（證明查詢有鑑別力）")
    void v1DidNotHaveExchange() {
        assertTrue(V1.contains("CREATE TABLE IF NOT EXISTS outbox_events")
                        || V1.contains("CREATE TABLE outbox_events"),
                "V1 裡找不到 outbox_events 建表——查詢本身壞了，後面的否定斷言全部不算數");
        assertFalse(V1.matches(".*CREATE TABLE[^;]*outbox_events[^;]*exchange.*"),
                "V1 的建表竟然已有 exchange 欄，與本次 migration 的前提矛盾");
    }

    @Test
    @DisplayName("entity 有對應的 exchange 欄位（String）")
    void entityHasExchangeField() {
        Set<String> fields = Arrays.stream(OutboxEventEntity.class.getDeclaredFields())
                .map(Field::getName).collect(Collectors.toSet());
        assertTrue(fields.contains("exchange"),
                "OutboxEventEntity 缺 exchange 欄位；findPendingEvents 是 SELECT * + autoResultMap，"
                        + "缺欄位不會有任何東西編譯錯，publisher 只會拿到 null 然後 fail-closed。實際欄位=" + fields);

        Field f = Arrays.stream(OutboxEventEntity.class.getDeclaredFields())
                .filter(x -> x.getName().equals("exchange")).findFirst().orElseThrow();
        assertTrue(f.getType() == String.class, "exchange 欄位型別應為 String，實際=" + f.getType());
    }
}
