package io.github.will413028.hawkr.payment.logistics.query;

import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.junit.jupiter.api.Assertions.assertFalse;
import static org.junit.jupiter.api.Assertions.assertTrue;

import io.github.will413028.hawkr.payment.logistics.repository.mapper.LogisticsFinanceQueryMapper;
import java.lang.reflect.Method;
import java.util.ArrayList;
import java.util.List;
import java.util.Locale;
import java.util.TreeSet;
import org.apache.ibatis.annotations.Insert;
import org.apache.ibatis.annotations.Select;
import org.apache.ibatis.annotations.Update;
import org.junit.jupiter.api.DisplayName;
import org.junit.jupiter.api.Test;

/**
 * 物流財務查詢的<b>範圍</b>與<b>唯讀</b>契約。
 *
 * <h2>它證明什麼</h2>
 * <ol>
 *   <li>每一支 {@code seller*} 查詢的 SQL 都帶著 {@code store_id = #{storeId}} 述詞
 *       —— 那是「賣家看不到別人的應收」的<b>唯一</b>機械保證。</li>
 *   <li>本 mapper <b>一句寫入都沒有</b>（零 {@code @Insert}／{@code @Update}），
 *       且 SQL 文字裡沒有 {@code INSERT}／{@code UPDATE}／{@code DELETE}。</li>
 * </ol>
 *
 * <h2>為什麼非釘不可</h2>
 * 拿掉那個述詞，回應的<b>形狀完全不變</b>：仍然是 200、仍然是一頁應收，只是裡面混進了
 * 別人的資料。沒有任何測試、型別或 CI 會因此轉紅——它只在有人真的比對兩家店的資料時
 * 才會被發現。
 *
 * <h2>它不證明什麼（誠實標註）</h2>
 * 這是對 SQL <b>字面</b>的斷言，不是對 PostgreSQL 執行結果的斷言。
 * 它擋得住「有人把述詞刪掉／改成別的欄位」；擋不住「述詞在、但綁錯參數」
 * （例如 {@code store_id = #{receivableId}} —— 那會編譯過、這裡也會過）。
 * 後者由 {@code LogisticsFinanceQueryServiceTest} 的收斂點守衛從行為面接住。
 */
@DisplayName("物流財務查詢：賣家範圍與唯讀契約")
class LogisticsFinanceQueryScopeContractTest {

    /**
     * 🔴 期望值硬寫字面值，<b>不從被測物反推</b>：從 SQL 抽出來再拿去比對 SQL，
     * 那是自己跟自己對，必然全綠。
     */
    private static final String STORE_SCOPE_DIRECT = "store_id = #{storeId}";
    private static final String STORE_SCOPE_JOINED = "r.store_id = #{storeId}";

    private static List<Method> sellerMethods() {
        List<Method> out = new ArrayList<>();
        for (Method m : LogisticsFinanceQueryMapper.class.getDeclaredMethods()) {
            if (m.getName().startsWith("seller")) {
                out.add(m);
            }
        }
        out.sort((a, b) -> a.getName().compareTo(b.getName()));
        return out;
    }

    @Test
    @DisplayName("每一支 seller* 查詢都帶 store_id 述詞")
    void everySellerQueryIsStoreScoped() {
        List<Method> methods = sellerMethods();

        // 🔴 對照組：先證明「我確實抓到了東西」。抓到 0 支方法而斷言全過，
        //    是本測試最可能的假陰性來源（改名、搬檔、反射抓錯類別都長這樣）。
        assertEquals(8, methods.size(),
                "seller* 查詢的支數變了。新增一支就要在這裡對帳，不是把數字改大："
                        + new TreeSet<>(methods.stream().map(Method::getName).toList()));

        List<String> missing = new ArrayList<>();
        for (Method m : methods) {
            Select select = m.getAnnotation(Select.class);
            assertTrue(select != null && select.value().length == 1,
                    m.getName() + " 必須是單一 @Select 查詢");
            String sql = select.value()[0];
            if (!sql.contains(STORE_SCOPE_DIRECT) && !sql.contains(STORE_SCOPE_JOINED)) {
                missing.add(m.getName());
            }
        }
        assertTrue(missing.isEmpty(),
                "以下 seller* 查詢沒有 store 範圍述詞 —— 賣家會看到別人的物流財務資料："
                        + missing);
    }

    @Test
    @DisplayName("查詢 mapper 沒有任何寫入入口")
    void queryMapperIsReadOnly() {
        List<String> writes = new ArrayList<>();
        for (Method m : LogisticsFinanceQueryMapper.class.getDeclaredMethods()) {
            if (m.getAnnotation(Insert.class) != null || m.getAnnotation(Update.class) != null) {
                writes.add(m.getName() + "（@Insert/@Update）");
                continue;
            }
            Select select = m.getAnnotation(Select.class);
            if (select == null) {
                continue;
            }
            String sql = select.value()[0].toUpperCase(Locale.ROOT);
            // ⚠️ 只看語句開頭的動詞不夠：本 repo 有 `@Select` 執行 `UPDATE … RETURNING` 的
            //    既有寫法（claim 的租約搶佔就是），所以這裡掃的是整段 SQL 文字。
            for (String verb : List.of("INSERT ", "UPDATE ", "DELETE ")) {
                if (sql.contains(verb)) {
                    writes.add(m.getName() + "（SQL 內含 " + verb.trim() + "）");
                }
            }
        }
        assertTrue(writes.isEmpty(),
                "物流財務查詢層不得有任何寫入路徑（CLAUDE.md §22：那等於繞過 idempotency key "
                        + "與 lease 的第二條路）：" + writes);
    }

    @Test
    @DisplayName("Admin 查詢不得被誤加 store 範圍（那會讓 ADM 看不到任何資料）")
    void adminQueriesAreNotHardScoped() {
        for (Method m : LogisticsFinanceQueryMapper.class.getDeclaredMethods()) {
            if (!m.getName().startsWith("admin")) {
                continue;
            }
            Select select = m.getAnnotation(Select.class);
            if (select == null) {
                continue;
            }
            String sql = select.value()[0];
            // Admin 的 store 篩選是**可選**的，必須包在 <if> 裡；無條件的述詞代表寫錯了。
            int direct = sql.indexOf(STORE_SCOPE_DIRECT);
            if (direct >= 0) {
                assertTrue(sql.substring(0, direct).contains("<if test=\"storeId != null\">"),
                        m.getName() + " 的 store 篩選必須是可選的（包在 <if> 內），"
                                + "否則 ADM 的總表會永遠是空的");
            }
            assertFalse(sql.contains("WHERE store_id = #{storeId}\n"),
                    m.getName() + " 不得無條件以 store 收斂");
        }
    }
}
