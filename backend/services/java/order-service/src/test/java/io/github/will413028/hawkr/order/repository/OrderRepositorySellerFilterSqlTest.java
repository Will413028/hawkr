package io.github.will413028.hawkr.order.repository;

import io.github.will413028.hawkr.order.entity.SellerOrderQuery;
import org.apache.ibatis.annotations.Select;
import org.apache.ibatis.mapping.SqlSource;
import org.apache.ibatis.scripting.xmltags.XMLLanguageDriver;
import org.apache.ibatis.session.Configuration;
import org.junit.jupiter.api.DisplayName;
import org.junit.jupiter.api.Test;

import java.time.LocalDateTime;
import java.util.HashMap;
import java.util.List;
import java.util.Map;

import static org.assertj.core.api.Assertions.assertThat;

/**
 * 賣家訂單列表／筆數兩句 SQL 的**渲染結果**契約（backend#2478）。
 *
 * <p>分頁是手寫 LIMIT/OFFSET ＋ 獨立 COUNT，兩句的 WHERE 一旦分岔，total 會與 records 對不上，
 * 而且不會拋任何例外——前端只會看到「第 3 頁是空的」。這支測試把 {@code @Select} 註解裡的
 * 動態 SQL 真的用 MyBatis 渲染出來，逐字比對兩者的 WHERE，讓
 * {@link OrderRepository#SELLER_ORDER_FILTER} 的共用不只是「現在剛好共用」——就算之後有人把
 * 片段就地展開回其中一句，只要條件不同這裡就紅。
 *
 * <p>另一半責任是釘住 keyword 的比對範圍：{@code recipient_phone} 與完整 {@code shipping_address}
 * 一旦被「順手」加進來，就是一支不觸發 ORDER_PII_REVEAL 稽核的存在性 oracle（56 號設計書
 * §8.1.5／A16）。理由寫在 {@link OrderRepository#SELLER_ORDER_FILTER} 的註解，這裡是機械保證。
 */
class OrderRepositorySellerFilterSqlTest {

    private static final long STORE_ID = 2685914682L;

    @Test
    @DisplayName("列表與筆數的 WHERE 條件逐字一致（四種篩選組合）")
    void listAndCountShareIdenticalWhereClause() {
        for (SellerOrderQuery query : List.of(
                query(0L, List.of(), null, null, null),
                query(0L, List.of(), "%蘋果%", null, null),
                query(2623900520L, List.of("20", "30"), null, from(), to()),
                query(2623900520L, List.of("20"), "%O123%", from(), to()))) {
            assertThat(whereOf(renderList(query)))
                    .as("篩選組合 buyerId=%d statuses=%s keyword=%s dateFrom=%s dateTo=%s",
                            query.getBuyerId(), query.getStatusList(), query.getKeywordPattern(),
                            query.getDateFrom(), query.getDateTo())
                    .isEqualTo(whereOf(renderCount(query)));
        }
    }

    @Test
    @DisplayName("keyword 的比對範圍恰為 order_no／recipient_name／品名，不含電話與完整地址")
    void keywordMatchesOnlyListVisibleColumns() {
        String sql = whereOf(renderList(query(0L, List.of(), "%林%", null, null)));

        assertThat(sql)
                .contains("order_no ILIKE ?")
                .contains("recipient_name ILIKE ?")
                .contains("oi.product_name ILIKE ?")
                .contains("oi.sku_name ILIKE ?")
                // 半連接：一單多品時不得讓訂單重複出現在列表
                .contains("EXISTS")
                .contains("oi.order_no = orders.order_no");
        assertThat(sql)
                .as("拿列表看不到的欄位當搜尋條件＝不留稽核痕跡的存在性 oracle（56 號 §8.1.5／A16）")
                .doesNotContain("recipient_phone")
                .doesNotContain("shipping_address");
    }

    @Test
    @DisplayName("keyword 為 null 時完全不產生 ILIKE 條件（空字串＝不篩選）")
    void noKeywordMeansNoIlikePredicate() {
        assertThat(whereOf(renderList(query(0L, List.of(), null, null, null))))
                .doesNotContain("ILIKE")
                .doesNotContain("EXISTS");
    }

    @Test
    @DisplayName("日期邊界兩端都是閉區間，且未給值時不產生條件")
    void dateBoundsAreInclusiveAndOptional() {
        String bounded = whereOf(renderList(query(0L, List.of(), null, from(), to())));
        assertThat(bounded).contains("created_at >= ?").contains("created_at <= ?");

        String unbounded = whereOf(renderList(query(0L, List.of(), null, null, null)));
        assertThat(unbounded).doesNotContain("created_at >=").doesNotContain("created_at <=");

        String onlyFrom = whereOf(renderList(query(0L, List.of(), null, from(), null)));
        assertThat(onlyFrom).contains("created_at >= ?").doesNotContain("created_at <=");
    }

    // ==================== helpers ====================

    private static SellerOrderQuery query(long buyerId, List<String> statusList, String keywordPattern,
                                          LocalDateTime dateFrom, LocalDateTime dateTo) {
        return new SellerOrderQuery(STORE_ID, buyerId, statusList, keywordPattern, dateFrom, dateTo);
    }

    private static LocalDateTime from() {
        return LocalDateTime.of(2026, 8, 1, 0, 0);
    }

    private static LocalDateTime to() {
        return LocalDateTime.of(2026, 8, 13, 23, 59, 59);
    }

    private static String renderList(SellerOrderQuery query) {
        Map<String, Object> params = new HashMap<>();
        params.put("q", query);
        params.put("size", 20);
        params.put("offset", 0);
        return render(scriptOf("findSellerOrders", SellerOrderQuery.class, int.class, int.class), params);
    }

    private static String renderCount(SellerOrderQuery query) {
        Map<String, Object> params = new HashMap<>();
        params.put("q", query);
        return render(scriptOf("countSellerOrders", SellerOrderQuery.class), params);
    }

    /** 直接讀 mapper 上的 {@code @Select} 字面值，避免測試自己抄一份 SQL 而與正式碼各自演化。 */
    private static String scriptOf(String method, Class<?>... paramTypes) {
        try {
            String[] value = OrderRepository.class.getMethod(method, paramTypes)
                    .getAnnotation(Select.class).value();
            return String.join(" ", value);
        } catch (NoSuchMethodException e) {
            throw new AssertionError("mapper 方法簽章已變更：" + method, e);
        }
    }

    private static String render(String script, Map<String, Object> params) {
        SqlSource sqlSource = new XMLLanguageDriver()
                .createSqlSource(new Configuration(), script, Object.class);
        return sqlSource.getBoundSql(params).getSql();
    }

    /** 取 WHERE 起至 ORDER BY 前（count 無 ORDER BY 則取到結尾），並壓平空白後比對。 */
    private static String whereOf(String sql) {
        String normalized = sql.replaceAll("\\s+", " ").trim();
        int where = normalized.indexOf("WHERE ");
        int orderBy = normalized.lastIndexOf("ORDER BY ");
        return (orderBy > where ? normalized.substring(where, orderBy) : normalized.substring(where)).trim();
    }
}
