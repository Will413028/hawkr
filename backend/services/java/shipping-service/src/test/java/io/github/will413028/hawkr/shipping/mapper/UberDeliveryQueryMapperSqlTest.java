package io.github.will413028.hawkr.shipping.mapper;

import io.github.will413028.hawkr.common.utils.code.ShippingMethod;
import io.github.will413028.hawkr.common.utils.code.ShippingStatus;
import org.apache.ibatis.annotations.Select;
import org.junit.jupiter.api.Test;

import javax.xml.parsers.DocumentBuilderFactory;
import java.io.ByteArrayInputStream;
import java.lang.reflect.Method;
import java.nio.charset.StandardCharsets;
import java.util.ArrayList;
import java.util.List;

import static org.junit.jupiter.api.Assertions.assertFalse;
import static org.junit.jupiter.api.Assertions.assertNotNull;
import static org.junit.jupiter.api.Assertions.assertTrue;
import static org.junit.jupiter.api.Assertions.fail;

/**
 * {@link UberDeliveryQueryMapper} 的 SQL 契約釘樁（66；Stage 8B W6-3／W6-4）。
 *
 * <p><b>它證明什麼</b>：
 * <ol>
 *   <li>賣家面<b>每一支</b>查詢都帶 {@code store_id} 歸屬述詞——清單由介面自己推導
 *       （名字含 {@code Seller} 者全數納入），不是寫死三個方法名。
 *       寫死的話，第四支賣家查詢加進來時這個 gate 看不到它，而那正是最可能漏掉的時刻。</li>
 *   <li>驅動表是 {@code shipments}，不是 {@code uber_deliveries}。</li>
 *   <li>SQL 裡的碼值字面量與 CodeMeta enum 的實值相符。</li>
 *   <li>{@code failure_only} 的判準不碰 {@code provider_status}。</li>
 * </ol>
 *
 * <p><b>它不證明什麼（誠實標註）</b>：這是對 {@code @Select} <b>註解值</b>的斷言，
 * 不是對 PostgreSQL 執行結果的斷言——shipping-service 沒有 DB 測試基礎設施
 * （testcontainers 只在 cart / aftersales / coupon，且被預設 {@code mvn test} 排除）。
 * 它擋得住「有人把述詞拿掉／改錯表／改錯碼值」這類回歸；擋不住「述詞寫對了但索引沒吃到」。
 * 讀註解而不是讀原始碼字串是刻意的：註解值就是<b>實際會被執行的那份 SQL</b>，
 * 而原始碼裡的同名常數可能根本沒被任何查詢用到。
 */
class UberDeliveryQueryMapperSqlTest {

    /** 取某方法 {@code @Select} 的實際 SQL（多段陣列以空白接起、壓平空白）。 */
    private static String sqlOf(String methodName) {
        for (Method m : UberDeliveryQueryMapper.class.getDeclaredMethods()) {
            if (!m.getName().equals(methodName)) {
                continue;
            }
            Select select = m.getAnnotation(Select.class);
            assertNotNull(select, methodName + " 沒有 @Select——本測試指著的東西已經不存在了");
            return String.join(" ", select.value()).replaceAll("\\s+", " ");
        }
        fail("找不到方法 " + methodName + "——mapper 被改動過，先確認本測試還指著對的東西");
        return "";
    }

    /** 賣家面查詢＝方法名含 {@code Seller} 者。由介面推導而非寫死清單（見類別說明 ①）。 */
    private static List<String> sellerFacingMethods() {
        List<String> names = new ArrayList<>();
        for (Method m : UberDeliveryQueryMapper.class.getDeclaredMethods()) {
            if (m.getName().contains("Seller")) {
                names.add(m.getName());
            }
        }
        return names;
    }

    @Test
    void everySellerFacingQueryCarriesTheStoreOwnershipPredicate() {
        List<String> methods = sellerFacingMethods();
        assertTrue(methods.size() >= 3,
                "賣家面查詢應有 list／count／detail 三支，實際只找到 " + methods
                        + "——若真的減少了，先確認少掉的那支去哪了");

        String predicate = UberDeliveryQueryMapper.SELLER_OWNERSHIP_PREDICATE.replaceAll("\\s+", " ");
        for (String method : methods) {
            assertTrue(sqlOf(method).contains(predicate),
                    method + " 少了歸屬述詞 " + predicate + "——後果不是 403 而是"
                            + "「賣家看到全平台的 Uber 配送」，而列表照樣回 200、分頁照樣正常");
        }
    }

    @Test
    void sellerOwnershipPredicateBindsStoreIdParameter() {
        // 防「述詞還在但綁到別的參數」——例如被改成 #{sellerId} 之後靜默恆不命中。
        assertTrue(UberDeliveryQueryMapper.SELLER_OWNERSHIP_PREDICATE.contains("#{storeId}"),
                "歸屬述詞必須綁 storeId 參數，實際：" + UberDeliveryQueryMapper.SELLER_OWNERSHIP_PREDICATE);
        assertTrue(UberDeliveryQueryMapper.SELLER_OWNERSHIP_PREDICATE.contains("s.store_id"),
                "歸屬來源只有 shipments.store_id（uber_deliveries 自己沒有 store_id），實際："
                        + UberDeliveryQueryMapper.SELLER_OWNERSHIP_PREDICATE);
    }

    @Test
    void drivingTableIsShipmentsSoCreateFailedAndUnknownRowsStayVisible() {
        String join = UberDeliveryQueryMapper.FROM_JOIN.replaceAll("\\s+", " ");
        assertTrue(join.startsWith("FROM shipments s"),
                "驅動表必須是 shipments：990／991 的單沒有 uber_deliveries 列，"
                        + "以 uber_deliveries 驅動會讓 W6-4 的 failure／unknown 兩個篩選"
                        + "永遠查不到東西，而且完全無聲。實際：" + join);
        assertTrue(join.contains("LEFT JOIN uber_deliveries d"),
                "必須 LEFT JOIN（不是 INNER）否則同樣會濾掉沒有 delivery 列的單。實際：" + join);
    }

    @Test
    void uberMethodLiteralMatchesCodeMetaValue() {
        assertTrue(UberDeliveryQueryMapper.UBER_METHOD_PREDICATE
                        .contains("'" + ShippingMethod.UBER_INSTANT.value() + "'"),
                "SQL 字面量與 ShippingMethod.UBER_INSTANT 脫節（@Select 需編譯期常數，"
                        + "無法呼叫 enum，只能靠本測試對帳）。實際："
                        + UberDeliveryQueryMapper.UBER_METHOD_PREDICATE);
    }

    @Test
    void unknownOnlyLiteralMatchesCreateUnknownValue() {
        String expected = "s.status = '" + ShippingStatus.CREATE_UNKNOWN.value() + "'";
        for (String method : new String[]{"selectAdminDeliveries", "countAdminDeliveries"}) {
            assertTrue(sqlOf(method).contains(expected),
                    method + " 的 unknown_only 述詞與 ShippingStatus.CREATE_UNKNOWN 脫節，"
                            + "應含：" + expected);
        }
    }

    @Test
    void failureOnlyDoesNotUseProviderStatusAsTheCriterion() {
        // provider_status 保存的是**正常軌跡**；以「有值」判失敗會把每張正常單撈進來
        // （V12 對 shipments 修過同一個 regression）。
        for (String method : new String[]{"selectAdminDeliveries", "countAdminDeliveries"}) {
            String branch = failureOnlyBranch(sqlOf(method), method);
            assertFalse(branch.contains("provider_status"),
                    method + " 的 failure_only 用了 provider_status 當判準。實際分支：" + branch);
            assertTrue(branch.contains("d.outcome IS NOT NULL"),
                    method + " 的 failure_only 必須以 outcome 為主判準。實際分支：" + branch);
            assertTrue(branch.contains("provider_error_code"),
                    method + " 的 failure_only 必須同時涵蓋 provider_error_code（建單失敗那一類"
                            + "根本沒有 outcome）。實際分支：" + branch);
        }
    }

    /**
     * 每一支的 {@code <script>} 都必須是合法 XML。
     *
     * <p>🔴 沒有這條的話，一個少打的結束標籤要等到<b>服務啟動</b>（MyBatis 註冊 mapper 時）
     * 才會炸——dev 無所謂，prod 就是那一顆 pod 起不來，而觸發者只是「加了一個篩選條件」的人。
     * 把訊號從「部署時」拉回「寫 code 時」是這條的全部價值（§3.9 第 6 種外殼）。
     */
    @Test
    void everyScriptIsWellFormedXml() {
        int checked = 0;
        for (Method m : UberDeliveryQueryMapper.class.getDeclaredMethods()) {
            Select select = m.getAnnotation(Select.class);
            if (select == null) {
                continue;
            }
            String sql = String.join(" ", select.value());
            assertTrue(sql.trim().startsWith("<script>"),
                    m.getName() + " 的 SQL 沒有以 <script> 開頭——動態標籤不會被解析，"
                            + "<if>/<foreach> 會原樣送進 PostgreSQL");
            try {
                DocumentBuilderFactory factory = DocumentBuilderFactory.newInstance();
                factory.setNamespaceAware(false);
                factory.newDocumentBuilder().parse(
                        new ByteArrayInputStream(sql.getBytes(StandardCharsets.UTF_8)));
            } catch (Exception e) {
                fail(m.getName() + " 的 <script> 不是合法 XML：" + e.getMessage());
            }
            checked++;
        }
        // 不印計數的話，「沒有失敗」與「迴圈根本沒跑」長得一模一樣。
        assertTrue(checked >= 5, "只檢查了 " + checked + " 支查詢，預期至少 5 支（3 賣家 + 3 Admin）");
    }

    /** 只取 {@code <if test='failureOnly'>} 那一段，避免斷言誤命中別處的述詞。 */
    private static String failureOnlyBranch(String sql, String method) {
        String open = "<if test='failureOnly'>";
        int start = sql.indexOf(open);
        assertTrue(start >= 0, method + " 找不到 failureOnly 分支——先確認本測試還指著對的東西");
        int end = sql.indexOf("</if>", start);
        assertTrue(end > start, method + " 的 failureOnly 分支沒有結尾");
        return sql.substring(start + open.length(), end);
    }
}
