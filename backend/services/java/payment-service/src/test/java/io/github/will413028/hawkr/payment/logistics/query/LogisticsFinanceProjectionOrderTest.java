package io.github.will413028.hawkr.payment.logistics.query;

import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.junit.jupiter.api.Assertions.assertTrue;

import io.github.will413028.hawkr.payment.logistics.repository.mapper.LogisticsFinanceQueryMapper;
import io.github.will413028.hawkr.payment.logistics.repository.projection.LogisticsFinanceProjections.CollectionAggregate;
import io.github.will413028.hawkr.payment.logistics.repository.projection.LogisticsFinanceProjections.ReceivableAggregate;
import io.github.will413028.hawkr.payment.logistics.repository.projection.LogisticsFinanceProjections.StatementAggregate;
import java.lang.reflect.Method;
import java.lang.reflect.RecordComponent;
import java.util.ArrayList;
import java.util.List;
import java.util.Locale;
import java.util.regex.Matcher;
import java.util.regex.Pattern;
import org.apache.ibatis.annotations.Select;
import org.junit.jupiter.api.DisplayName;
import org.junit.jupiter.api.Test;

/**
 * record 投影的<b>欄位順序</b>契約。
 *
 * <h2>它防的是什麼</h2>
 * MyBatis 對 record 走 constructor automapping。本專案<b>沒有</b>開 {@code -parameters}
 * 編譯旗標（實測：{@code services/java/pom.xml} 與 payment-service 的 pom 對 {@code parameters}
 * 零命中）⇒ 參數名在 class 檔裡不存在 ⇒ 對映只能<b>依序</b>。
 *
 * <p>於是「{@code SELECT} 的兩個同型別欄位互換」是<b>完全靜默</b>的：編譯過、啟動過、
 * 查詢回來的數字互換了，而沒有任何東西會叫。症狀是「賣家的補貼金額變成了成本」這種
 * 看起來像業務邏輯錯誤的東西。
 *
 * <h2>它不證明什麼</h2>
 * 它比對的是 SQL 裡的 {@code AS 別名} 與 record component 名，不是 PostgreSQL 真正回傳的欄名。
 * 沒寫 {@code AS} 的欄位（例如 {@code SELECT *}）不在本測試範圍內——那些回的是 Entity
 * 不是 record，走的是 {@code map-underscore-to-camel-case} 的具名對映。
 */
@DisplayName("物流財務投影：SELECT 欄位順序 ↔ record component 順序")
class LogisticsFinanceProjectionOrderTest {

    /** {@code AS xxx_yyy} 的別名擷取（只認明確寫了 AS 的）。 */
    private static final Pattern ALIAS = Pattern.compile("(?i)\\bAS\\s+([a-z_][a-z0-9_]*)");

    private static String snake(String camel) {
        StringBuilder sb = new StringBuilder();
        for (char c : camel.toCharArray()) {
            if (Character.isUpperCase(c)) {
                sb.append('_').append(Character.toLowerCase(c));
            } else {
                sb.append(c);
            }
        }
        return sb.toString();
    }

    private static List<String> aliasesOf(String methodName) {
        for (Method m : LogisticsFinanceQueryMapper.class.getDeclaredMethods()) {
            if (!m.getName().equals(methodName)) {
                continue;
            }
            Select select = m.getAnnotation(Select.class);
            assertTrue(select != null, methodName + " 必須有 @Select");
            String sql = select.value()[0];
            // 只取第一段 SELECT（LATERAL 子查詢裡的別名不是外層投影欄）。
            int from = sql.toUpperCase(Locale.ROOT).indexOf("\nFROM ");
            String head = from > 0 ? sql.substring(0, from) : sql;
            List<String> out = new ArrayList<>();
            Matcher matcher = ALIAS.matcher(head);
            while (matcher.find()) {
                out.add(matcher.group(1));
            }
            return out;
        }
        throw new AssertionError("找不到 mapper 方法 " + methodName
                + " —— 方法改名了就要一起改本測試，不要讓它靜默地變成零斷言");
    }

    private static List<String> componentsOf(Class<?> recordClass) {
        List<String> out = new ArrayList<>();
        for (RecordComponent rc : recordClass.getRecordComponents()) {
            out.add(snake(rc.getName()));
        }
        return out;
    }

    private static void assertOrderMatches(String mapperMethod, Class<?> recordClass) {
        List<String> aliases = aliasesOf(mapperMethod);
        List<String> components = componentsOf(recordClass);
        // 對照組：抽不到別名 ⇒ 本斷言等於恆真，那比沒有斷言更糟。
        assertTrue(aliases.size() >= components.size(),
                mapperMethod + " 只抽到 " + aliases.size() + " 個別名，少於 record 的 "
                        + components.size() + " 個 component —— 正則沒抓到東西，斷言等於恆真");
        assertEquals(components, aliases.subList(0, components.size()),
                mapperMethod + " 的 SELECT 欄位順序必須與 " + recordClass.getSimpleName()
                        + " 的 component 順序逐一對齊（MyBatis 對 record 依序對映，"
                        + "順序錯了是靜默的數字互換）");
    }

    @Test
    @DisplayName("ReceivableAggregate")
    void receivableAggregate() {
        assertOrderMatches("sellerReceivableAggregate", ReceivableAggregate.class);
    }

    @Test
    @DisplayName("CollectionAggregate")
    void collectionAggregate() {
        assertOrderMatches("sellerCollectionAggregate", CollectionAggregate.class);
    }

    @Test
    @DisplayName("StatementAggregate（列表與詳情兩句都要對齊同一個 record）")
    void statementAggregate() {
        assertOrderMatches("adminListStatements", StatementAggregate.class);
        assertOrderMatches("adminGetStatement", StatementAggregate.class);
    }
}
