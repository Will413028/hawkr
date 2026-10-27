package io.github.will413028.hawkr.order.repository;

import org.apache.ibatis.annotations.Select;
import org.apache.ibatis.annotations.SelectProvider;
import org.junit.jupiter.api.DisplayName;
import org.junit.jupiter.api.Test;

import java.io.IOException;
import java.lang.reflect.Method;
import java.net.URISyntaxException;
import java.nio.file.Files;
import java.nio.file.Path;
import java.util.ArrayList;
import java.util.LinkedHashMap;
import java.util.List;
import java.util.Map;
import java.util.TreeMap;
import java.util.stream.Stream;

import static org.assertj.core.api.Assertions.assertThat;
import static org.assertj.core.api.Assertions.fail;

/**
 * {@code io.github.will413028.hawkr.order.repository} 全 package 取多列 SQL 的**排序決定性**名冊（issue #2547）。
 *
 * <p>PostgreSQL 對沒有全序的 {@code SELECT} 不保證回傳順序：任何一次 {@code UPDATE} 讓 MVCC 換掉
 * tuple 位置，heap 掃描順序就跟著變。三種傷害各不相同，但根因是同一個：
 * <ul>
 *   <li><b>{@code LIMIT/OFFSET} 分頁</b>——相鄰兩頁是兩次獨立執行，各自對同鍵值的列挑一種順序
 *       ⇒ <b>同一筆資料在兩頁重複出現、或一頁都不出現</b>。賣家訂單列表少一筆不會報錯、
 *       不會轉紅，只是那張單從此沒人看見。</li>
 *   <li><b>{@code LIMIT N}（取前 N）</b>——邊界上的同分列誰進榜任意，榜單自己換人。</li>
 *   <li><b>整段渲染的列表</b>——退款歷程、資金流水這種時序資料，重新整理就換順序。</li>
 * </ul>
 *
 * <p>ORDER BY 在非唯一欄上一樣不決定性：{@code created_at} 取 {@code NOW()}＝<b>交易開始時刻</b>，
 * 同一交易內寫入的多列時間戳完全相同。所以判準是「<b>排序鍵必須以唯一鍵收尾</b>」，
 * 而不是「有沒有 ORDER BY」。
 *
 * <p>本測試從 <b>code source 目錄</b>走訪整個 package（不是逐支列舉 mapper），新增的 mapper 檔
 * 會自動納管——沒有登記收尾鍵就紅。逐支的語意錨（哪個欄該當首鍵、兩支查詢的排序鍵要不要一致）
 * 留在各自的 {@code *OrderingSqlTest}，本檔只管全序這一件事。
 *
 * <p>⚠️ 覆蓋範圍限於 mapper 上的 {@code @Select} 字面 SQL。MyBatis-Plus {@code BaseMapper}
 * 繼承來的 {@code selectList} 系列不在此列（沒有 repo 內的 SQL 可讀），它們的排序責任在呼叫端。
 */
class RepositoryOrderingSqlTest {

    /**
     * 每個 mapper 的「收尾鍵」＝該表的主鍵欄（{@code V1__init_order_tables.sql} 逐表確認過，
     * 各表不同，不能假設都叫 {@code id}）。
     */
    private static final Map<Class<?>, String> DETERMINISM_KEY = new LinkedHashMap<>();

    static {
        DETERMINISM_KEY.put(OrderRepository.class, "id");
        DETERMINISM_KEY.put(OrderItemRepository.class, "id");
        DETERMINISM_KEY.put(OrderOutboxMapper.class, "id");
        DETERMINISM_KEY.put(RefundRequestRepository.class, "request_no");
        DETERMINISM_KEY.put(FinancialLogRepository.class, "log_id");
        DETERMINISM_KEY.put(PaymentEventDedupRepository.class, "transaction_id");
        DETERMINISM_KEY.put(AutoRefundTaskRepository.class, "task_id");
        DETERMINISM_KEY.put(CartCleanupRetryTaskRepository.class, "task_id");
        DETERMINISM_KEY.put(CouponConsumptionTaskRepository.class, "task_id");
        DETERMINISM_KEY.put(GrantPaymentTaskRepository.class, "task_id");
        // 複合主鍵 (stock_tx_id, phase)；兩支候選查詢都把 phase 釘成單一值，
        // 該欄在結果集內為常數 ⇒ stock_tx_id 已足以構成全序。
        DETERMINISM_KEY.put(StockTxRecordRepository.class, "stock_tx_id");
        // 唯一鍵 (order_no, snapshot_version)；唯一一支查詢把 order_no 釘成單一值
        // （WHERE order_no = #{orderNo}）⇒ 該欄在結果集內為常數，snapshot_version
        // 已足以構成全序。與 StockTxRecordRepository 同一條理由。
        DETERMINISM_KEY.put(OrderInvoiceSnapshotRepository.class, "snapshot_version");
    }

    /**
     * {@code GROUP BY} 的查詢取不到主鍵（分組後主鍵已不存在），改以<b>分組鍵</b>收尾——
     * 分組鍵在結果集內天然唯一，全序一樣成立。
     */
    private static final Map<String, String> KEY_OVERRIDE = Map.of(
            "OrderRepository#findBuyerMerchantSummaryByStatuses", "store_id",
            "OrderRepository#findTopSellersByRevenue", "store_id",
            "OrderRepository#findStoreOrderStats", "store_id",
            "OrderRepository#findOrderAggregates", "buyer_id");

    /**
     * 判定為「順序無語意」而不加排序的查詢。<b>兩個條件必須同時成立</b>才可以列在這裡：
     * ① 沒有 {@code LIMIT}（不會有列因為排序而被排除在結果之外）
     * ② 消費端把結果當<b>集合</b>用，不依賴任何一列的位置。
     *
     * <p>只滿足其中一個不夠：有 {@code LIMIT} 就會漏列；被當序列用就會顯示錯的順序。
     */
    private static final Map<String, String> EXEMPT = Map.of(
            "OrderRepository#countMyOrdersGroupByStatus",
            "GROUP BY status 的筆數表，無 LIMIT；消費端（OrderGrpcService.getMyOrderStatusCounts）"
                    + "逐列 addCounts 並加總 total，前端依 status 取用對應筆數，位置無語意",

            "OrderRepository#findPendingPaymentOrderNosBySeller",
            "賣家停權批次取消的候選全集，無 LIMIT；SellerSuspendedConsumer 逐筆走 CAS 取消整個集合，"
                    + "每一筆都會被處理，先後不影響結果");

    // ==================== 錨 ====================

    @Test
    @DisplayName("錨 1：package 內每個 mapper 都登記了收尾鍵（新增 mapper 不得靜默逃過）")
    void everyMapperInPackageHasARegisteredDeterminismKey() {
        List<Class<?>> discovered = mapperInterfacesInPackage();

        assertThat(discovered)
                .as("走訪 code source 目錄一個 mapper 都沒掃到 ⇒ 掃描壞了，本檔全部的綠燈都是假的")
                .contains(OrderRepository.class);
        assertThat(discovered)
                .as("有 mapper 未登記收尾鍵：本檔無從判斷它的排序是否構成全序，等於不納管")
                .allMatch(DETERMINISM_KEY::containsKey);
        assertThat(DETERMINISM_KEY.keySet())
                .as("登記表有已不存在的 mapper ⇒ 名冊腐化")
                .allMatch(discovered::contains);
    }

    @Test
    @DisplayName("錨 2：取多列／帶 LIMIT 的 @Select 一律要有以該表唯一鍵收尾的 ORDER BY")
    void everyMultiRowSelectIsTotallyOrdered() {
        Map<String, String> selects = multiRowSelects();

        assertThat(selects)
                .as("一個取多列的 @Select 都沒掃到 ⇒ 解析壞了，綠燈是假的")
                .containsKey("OrderRepository#findSellerOrders");

        List<String> violations = new ArrayList<>();
        selects.forEach((key, sql) -> {
            if (EXEMPT.containsKey(key)) {
                return;
            }
            String orderBy = MapperSql.orderByOf(sql);
            if (orderBy == null) {
                violations.add(key + " 取多列卻沒有 ORDER BY（PostgreSQL 不保證順序）\n    SQL: " + sql);
                return;
            }
            String uniqueTail = uniqueTailOf(key);
            if (!MapperSql.endsWith(uniqueTail).matcher(orderBy).find()) {
                violations.add(key + " 的排序鍵沒有以唯一鍵 " + uniqueTail
                        + " 收尾 ⇒ 同鍵值的列之間仍然沒有順序\n    ORDER BY " + orderBy);
            }
        });

        if (!violations.isEmpty()) {
            fail("以下取多列的查詢沒有全序（分頁會漏列／重複列，列表會自己換順序）：%n%s%n%n"
                    + "判定為順序無語意時，請登記到本檔的 EXEMPT 並寫出理由（條件見該欄位註解）。",
                    String.join("\n", violations));
        }
    }

    @Test
    @DisplayName("錨 3：EXEMPT 名冊不得有殘留（豁免的方法必須還在、且真的是取多列的查詢）")
    void exemptionsStillPointAtRealMultiRowQueries() {
        Map<String, String> selects = multiRowSelects();

        assertThat(EXEMPT.keySet())
                .as("EXEMPT 列了已不存在或已不再取多列的方法 ⇒ 名冊腐化，"
                        + "下一支同名方法會直接繼承一個沒人重新判斷過的豁免")
                .allMatch(selects::containsKey);
    }

    @Test
    @DisplayName("錨 4：本 package 不得出現 @SelectProvider 或 XML mapper（本檔讀不到它們的 SQL）")
    void noSqlSourceEscapesThisRoster() {
        List<String> providers = new ArrayList<>();
        for (Class<?> mapper : mapperInterfacesInPackage()) {
            for (Method method : mapper.getDeclaredMethods()) {
                if (method.getAnnotation(SelectProvider.class) != null) {
                    providers.add(key(mapper, method));
                }
            }
        }
        assertThat(providers)
                .as("@SelectProvider 的 SQL 由 Java 方法動態組出，本檔的字面值解析看不到它 ⇒ 盲區。"
                        + "要用它就得同時擴充本檔的取 SQL 方式")
                .isEmpty();

        assertThat(getClass().getResource("/mapper"))
                .as("出現 XML mapper（application.yml 的 mapper-locations 指向 classpath:/mapper/**/*.xml）"
                        + "⇒ 那些 SQL 不在本檔的掃描範圍內")
                .isNull();
    }

    // ==================== helpers ====================

    /**
     * 走訪 {@link OrderRepository} 所在的 code source 目錄，取出本 package 下所有 mapper 介面。
     * <p>
     * 刻意不逐支列舉：列舉式的名冊會在有人新增 mapper 時安靜地漏掉那一支，而漏掉的那支正是
     * 沒有人想過排序問題的那支。
     */
    private static List<Class<?>> mapperInterfacesInPackage() {
        String packageName = OrderRepository.class.getPackageName();
        Path root;
        try {
            root = Path.of(OrderRepository.class.getProtectionDomain()
                    .getCodeSource().getLocation().toURI());
        } catch (URISyntaxException e) {
            throw new AssertionError("取不到 mapper 的 code source 位置", e);
        }
        Path packageDir = root.resolve(packageName.replace('.', '/'));
        assertThat(packageDir)
                .as("mapper package 目錄不存在，掃描不到任何東西")
                .exists();

        try (Stream<Path> files = Files.list(packageDir)) {
            List<Class<?>> result = new ArrayList<>();
            files.map(path -> path.getFileName().toString())
                    .filter(name -> name.endsWith(".class") && !name.contains("$"))
                    .map(name -> name.substring(0, name.length() - ".class".length()))
                    .sorted()
                    .forEach(simpleName -> {
                        try {
                            Class<?> type = Class.forName(packageName + "." + simpleName);
                            if (type.isInterface()) {
                                result.add(type);
                            }
                        } catch (ClassNotFoundException e) {
                            throw new AssertionError("class 檔存在但載不進來：" + simpleName, e);
                        }
                    });
            return result;
        } catch (IOException e) {
            throw new AssertionError("走訪 mapper package 目錄失敗：" + packageDir, e);
        }
    }

    /**
     * 取所有「可能回傳多列」的 {@code @Select}：回傳 {@link List} 者，或 SQL 帶 {@code LIMIT} 者
     * （{@code LIMIT} 沒有全序同樣是從不確定的順序裡挑一列）。
     * 依主鍵／唯一鍵取單列者與回 {@code int}/{@code long} 的 count 系列不在此列。
     */
    private static Map<String, String> multiRowSelects() {
        Map<String, String> result = new TreeMap<>();
        for (Class<?> mapper : mapperInterfacesInPackage()) {
            for (Method method : mapper.getDeclaredMethods()) {
                Select select = method.getAnnotation(Select.class);
                if (select == null) {
                    continue;
                }
                String sql = MapperSql.flatten(select.value());
                if (List.class.isAssignableFrom(method.getReturnType())
                        || sql.toUpperCase().contains(" LIMIT ")) {
                    result.put(key(mapper, method), sql);
                }
            }
        }
        return result;
    }

    private static String key(Class<?> mapper, Method method) {
        return mapper.getSimpleName() + "#" + method.getName();
    }

    private static String uniqueTailOf(String key) {
        String override = KEY_OVERRIDE.get(key);
        if (override != null) {
            return override;
        }
        String mapperName = key.substring(0, key.indexOf('#'));
        return DETERMINISM_KEY.entrySet().stream()
                .filter(e -> e.getKey().getSimpleName().equals(mapperName))
                .map(Map.Entry::getValue)
                .findFirst()
                .orElseThrow(() -> new AssertionError("未登記收尾鍵的 mapper：" + mapperName));
    }

}
