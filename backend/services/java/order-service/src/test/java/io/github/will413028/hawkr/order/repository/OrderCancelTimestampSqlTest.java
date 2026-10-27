package io.github.will413028.hawkr.order.repository;

import org.apache.ibatis.annotations.Update;
import org.junit.jupiter.api.DisplayName;
import org.junit.jupiter.api.Test;

import java.io.IOException;
import java.io.UncheckedIOException;
import java.lang.reflect.Method;
import java.nio.charset.StandardCharsets;
import java.nio.file.Files;
import java.nio.file.Path;
import java.util.LinkedHashMap;
import java.util.List;
import java.util.Map;
import java.util.TreeMap;
import java.util.regex.Matcher;
import java.util.regex.Pattern;
import java.util.stream.Stream;

import static org.assertj.core.api.Assertions.assertThat;
import static org.assertj.core.api.Assertions.fail;

/**
 * 取消時間戳 {@code cancelled_at} 的**窮舉守護**（issue #2471）。
 *
 * <p>漏掉一條取消路徑的後果是靜默的：那條路徑產生的訂單永遠沒有結案時間，不報錯、不轉紅，
 * 前端只是少畫一行。所以光有「三支各驗一次」不夠——真正要守的是「**沒有第四條**」。
 *
 * <p><b>為什麼錨不能下在狀態值</b>：本 repo 禁硬編碼，狀態一律 {@code #{param}} 注入，
 * 取消 SQL 的字面上根本看不到 {@code '80'}（CANCELLED）。以字面值為錨的掃描對這個 repo
 * 系統性失明。因此本測試改用三個彼此獨立、都不依賴狀態值的錨：
 * <ol>
 *   <li><b>SET 子句寫了取消稽核欄</b>（{@code cancel_reason／cancel_remark／cancel_by_role}）
 *       ⇒ 必須同句寫 {@code cancelled_at}。語意錨，不需知道目標狀態是什麼。</li>
 *   <li><b>SET 子句寫了 {@code status}</b> 的 {@code @Update} 方法集合 ＝ 下方名冊，
 *       且每個都標了「是不是取消」。新增任何狀態寫入者都會讓這裡紅，逼人分類——
 *       這是唯一擋得住「第四條取消路徑」的機制。</li>
 *   <li><b>{@code UPDATE orders} 只准出現在 {@link OrderRepository}</b>，且 src/main 不得出現
 *       {@code updateById(}／{@code LambdaUpdateWrapper}。否則狀態可以在沒有任何 SQL 字面值的
 *       情況下被改掉（全欄覆寫），前兩條錨一起失明。</li>
 * </ol>
 *
 * <p>錨 2 會在新增**非取消**的狀態寫入者時也轉紅。那是刻意的：代價是往名冊加一行，
 * 換到的是「新的取消路徑不可能靜悄悄地溜過去」。
 */
class OrderCancelTimestampSqlTest {

    /** SET 子句裡的取消時間戳寫入（三支取消 CAS 一律這樣寫）。 */
    private static final String CANCELLED_AT_STAMP = "cancelled_at = NOW()";

    /** 取消稽核欄：寫了其中任一，這句 SQL 在語意上就是一次取消。 */
    private static final Pattern CANCEL_AUDIT_COLUMN =
            Pattern.compile("cancel_(reason|remark|by_role)\\s*=");

    /**
     * SET 子句裡的 {@code status} 寫入。負向後查排除 {@code shipping_status}、
     * {@code shipping_status_desc}（後者另因要求緊接 {@code =} 而排除）；
     * {@code #{newStatus}} 等參數名是大寫 S，本來就不會命中。
     */
    private static final Pattern STATUS_WRITE =
            Pattern.compile("(?<![_a-zA-Z])status\\s*=");

    /**
     * {@link OrderRepository} 中所有會寫 {@code status} 的 {@code @Update}，逐一標明是不是取消。
     * <p>
     * 這份名冊本身就是斷言：實際掃出來的集合與它不一致就紅。新增狀態寫入者時，
     * 加進來的那一刻就必須回答「這是取消嗎」，答 {@code CANCEL} 就得補 {@code cancelled_at}。
     */
    private static final Map<String, Boolean> STATUS_WRITERS = new LinkedHashMap<>();

    static {
        // —— 非取消：推進 / 退款 / 完成，各有自己的業務時間欄（confirmed_at、delivered_at…）
        STATUS_WRITERS.put("updatePaymentConfirmed", false);          // → PAID，寫 pay_time
        STATUS_WRITERS.put("updateStatusIfMatchAny", false);          // 物流推進 20/30/32/35 與全額退款 70
        STATUS_WRITERS.put("applyPartialRefundByStatus", false);      // 50 → 60（CASE），不是終態取消
        STATUS_WRITERS.put("updateOrderDelivered", false);            // → DELIVERED，寫 delivered_at
        STATUS_WRITERS.put("completeDeliveredOrderByStatus", false);  // → COMPLETED，寫 confirmed_at
        // 🔴 35 → DELIVERY_FAILED_NO_REFUND(54)：**不是取消**（03 §2.1）。
        //    它是「配送未成功、但不退款、且**可撥款**」的財務結案終態——語意與 CANCELLED(80) 相反：
        //    取消是錢回買家，這一格是錢照撥給賣家。標成取消會讓它去寫 cancelled_at，
        //    而那個欄位的讀者（對帳、客服、報表）會把一筆已撥款的訂單讀成已取消。
        // ⚠️ 它也**不寫任何業務時間欄**：54 不經過 DELIVERED(40)，買家從未收貨
        //    ⇒ confirmed_at／delivered_at 一個都不該有值（03 §2.1 下游讀者表第 7 項）。
        STATUS_WRITERS.put("markDeliveryFailedNoRefund", false);
        // —— 取消：三條入口（賣家／售後、買家自願、系統逾時／停權），全部必須寫 cancelled_at
        STATUS_WRITERS.put("cancelOrderInternalCas", true);
        STATUS_WRITERS.put("cancelOrderByBuyerCas", true);
        STATUS_WRITERS.put("cancelOrderByPaymentExpireCas", true);
    }

    @Test
    @DisplayName("錨 1：任何寫取消稽核欄的 SQL 都必須同句寫 cancelled_at")
    void everyCancelAuditWriteAlsoStampsCancelledAt() {
        Map<String, String> setClauses = updateSetClauses();

        List<String> cancelWrites = setClauses.entrySet().stream()
                .filter(e -> CANCEL_AUDIT_COLUMN.matcher(e.getValue()).find())
                .map(Map.Entry::getKey)
                .toList();

        assertThat(cancelWrites)
                .as("取消稽核欄的寫入點消失＝這個錨失效了，不是通過")
                .isNotEmpty();

        for (String method : cancelWrites) {
            assertThat(setClauses.get(method))
                    .as("%s 寫了取消稽核欄卻沒寫 %s ⇒ 該路徑的訂單永遠沒有結案時間（靜默）",
                            method, CANCELLED_AT_STAMP)
                    .contains(CANCELLED_AT_STAMP);
        }
    }

    @Test
    @DisplayName("錨 2：寫 status 的 @Update 集合＝名冊，且取消者才寫 cancelled_at")
    void statusWritersMatchRosterAndOnlyCancelStamps() {
        Map<String, String> setClauses = updateSetClauses();

        Map<String, String> actual = new TreeMap<>();
        setClauses.forEach((method, set) -> {
            if (STATUS_WRITE.matcher(set).find()) {
                actual.put(method, set);
            }
        });

        assertThat(actual.keySet())
                .as("名冊與實際不符：新增了狀態寫入者就必須在 STATUS_WRITERS 標明是不是取消；"
                        + "是取消就要補 %s", CANCELLED_AT_STAMP)
                .containsExactlyInAnyOrderElementsOf(STATUS_WRITERS.keySet());

        actual.forEach((method, set) -> {
            if (STATUS_WRITERS.get(method)) {
                assertThat(set)
                        .as("%s 標為取消路徑，必須寫 %s", method, CANCELLED_AT_STAMP)
                        .contains(CANCELLED_AT_STAMP);
            } else {
                assertThat(set)
                        .as("%s 不是取消路徑，不得寫 cancelled_at（會讓未取消的訂單帶取消時間）", method)
                        .doesNotContain("cancelled_at");
            }
        });
    }

    @Test
    @DisplayName("錨 3：orders 的寫入只准經 OrderRepository 的具名 SQL，不得全欄覆寫")
    void ordersAreOnlyWrittenThroughNamedSqlInOrderRepository() {
        Path mainSources = Path.of("src", "main", "java");
        assertThat(mainSources)
                .as("找不到 src/main/java ⇒ 這支測試什麼都沒掃到，綠燈是假的")
                .isDirectory();

        Pattern updateOrders = Pattern.compile("UPDATE\\s+orders\\b");
        Pattern fullRowUpdate = Pattern.compile("updateById\\s*\\(|LambdaUpdateWrapper|UpdateWrapper");

        int scanned = 0;
        try (Stream<Path> files = Files.walk(mainSources)) {
            for (Path file : files.filter(p -> p.toString().endsWith(".java")).toList()) {
                String code = stripComments(Files.readString(file, StandardCharsets.UTF_8));
                scanned++;

                if (!file.endsWith(Path.of("repository", "OrderRepository.java"))
                        && updateOrders.matcher(code).find()) {
                    fail("%s 直接寫 orders 表 ⇒ 取消路徑可能繞過 OrderRepository，"
                            + "錨 1／2 掃不到它。請把 SQL 收回 OrderRepository", file);
                }
                Matcher fullRow = fullRowUpdate.matcher(code);
                if (fullRow.find()) {
                    fail("%s 使用 %s ⇒ 可在沒有任何 SQL 字面值的情況下改掉 status（全欄覆寫），"
                            + "本 repo 一律用具名 UPDATE", file, fullRow.group());
                }
            }
        } catch (IOException e) {
            throw new UncheckedIOException(e);
        }

        assertThat(scanned)
                .as("掃到的檔案數異常偏低 ⇒ 路徑錯了，綠燈是假的")
                .isGreaterThan(50);
    }

    /**
     * 🔴 <b>錨 4：{@code total_actual_amount} 建單之後不得再被寫入。</b>
     *
     * <p><b>為什麼 order-service 要為別的服務守這條</b>：payment-service 的責任判定
     * 影響預覽（{@code ResponsibilityImpactPreview}，WP2A-R2）採「<b>交易外</b>讀權威金額
     * → <b>交易內</b>重算比對」兩段式，而那個設計<b>只有在這個值不可變時才沒有 TOCTOU 窗口</b>。
     * 同一個假設今天也撐著 {@code SellerCancelRefundTxHandler}（真實退款金額來源）
     * 與 58 號發票的 I1 不變式（{@code Σ 明細 == invoice.total_amount == orders.total_actual_amount}，
     * 而 AMEGO 發票開出去<b>不可撤回</b>）。
     *
     * <p>⇒ 這條不變式今天是「窮舉之後發現沒有人寫它」，本測試把它變成<b>機械保證</b>：
     * 日後有人在 {@code OrderRepository} 新增一句把它放進 {@code SET} 的 UPDATE，這裡會紅。
     *
     * <p>⚠️ <b>本測試擋不住什麼</b>（誠實標明邊界）：它只看 {@code OrderRepository} 的
     * {@code @Update} 字面 SQL。錨 3 已經保證「{@code UPDATE orders} 只能出現在這個檔案」
     * 且全 repo 無 {@code updateById}／{@code UpdateWrapper}，兩者合起來才是完整的寫入面；
     * 單獨看本測試會以為它掃了全部。
     */
    @Test
    @DisplayName("錨 4：total_actual_amount 建單後 write-once（責任判定預覽的架構前提）")
    void totalActualAmountIsNeverUpdatedAfterInsert() {
        Path repository = Path.of("src", "main", "java", "io", "github", "will413028",
                "hawkr", "order", "repository", "OrderRepository.java");
        assertThat(repository)
                .as("找不到 OrderRepository.java ⇒ 這支測試什麼都沒掃到，綠燈是假的")
                .isRegularFile();

        String code;
        try {
            code = stripComments(Files.readString(repository, StandardCharsets.UTF_8));
        } catch (IOException e) {
            throw new UncheckedIOException(e);
        }

        // 取出每一段 UPDATE orders … 的 SET 子句（到 WHERE 為止）
        Matcher m = Pattern.compile("UPDATE\\s+orders\\b(.*?)\\bWHERE\\b",
                Pattern.DOTALL | Pattern.CASE_INSENSITIVE).matcher(code);
        int updates = 0;
        while (m.find()) {
            updates++;
            String setClause = m.group(1);
            assertThat(setClause)
                    .as("🔴 第 %d 句 UPDATE orders 把 total_actual_amount 放進 SET。"
                            + "它是責任判定影響預覽（payment-service）、賣家取消退款金額、"
                            + "以及 58 號發票 I1 不變式的共同權威——"
                            + "一旦它會變，那三處的「交易外讀→交易內重算」全部出現 TOCTOU 窗口，"
                            + "而已開出的 AMEGO 發票無法撤回。"
                            + "真要新增可變金額，請先回到 03 §2.5 與 58 §I1 收斂契約。", updates)
                    .doesNotContain("total_actual_amount");
        }

        // 🔴 對照組：真的抓到了 UPDATE 句，而且抓得到「確實會被改的欄位」。
        //    少了它，regex 落空時本測試會 0 次迴圈然後安靜地通過。
        assertThat(updates)
                .as("抓到的 UPDATE orders 句數異常偏低 ⇒ regex 落空，綠燈是假的")
                .isGreaterThan(10);
        assertThat(code)
                .as("量測本身失效：連 refund_total_amount 都抓不到 ⇒ 掃描範圍或剝註解壞了")
                .contains("refund_total_amount");
    }

    // ==================== helpers ====================

    /**
     * 讀 {@link OrderRepository} 上每個 {@code @Update} 的字面值，取 SET 到 WHERE 之間那段。
     * <p>
     * 只看 SET 子句是關鍵：WHERE 裡的 {@code status = #{expectedOldStatus}} 是 CAS 前置條件
     * 不是寫入，混進來會把每一支 CAS 都誤判成狀態寫入者。
     */
    private static Map<String, String> updateSetClauses() {
        Map<String, String> result = new TreeMap<>();
        for (Method method : OrderRepository.class.getDeclaredMethods()) {
            Update update = method.getAnnotation(Update.class);
            if (update == null) {
                continue;
            }
            String sql = String.join(" ", update.value()).replaceAll("\\s+", " ").trim();
            int set = sql.indexOf("SET ");
            if (set < 0) {
                fail("%s 的 @Update 沒有 SET 子句，解析假設已不成立", method.getName());
            }
            int where = sql.indexOf(" WHERE ", set);
            result.put(method.getName(),
                    where > set ? sql.substring(set, where) : sql.substring(set));
        }
        assertThat(result)
                .as("OrderRepository 一個 @Update 都沒掃到 ⇒ 解析壞了，綠燈是假的")
                .isNotEmpty();
        return result;
    }

    /** 註解裡提到 {@code updateById} 之類的字眼很常見（多半在說「不要用」），掃描前先去掉。 */
    private static String stripComments(String code) {
        return code.replaceAll("(?s)/\\*.*?\\*/", " ").replaceAll("(?m)//.*$", " ");
    }
}
