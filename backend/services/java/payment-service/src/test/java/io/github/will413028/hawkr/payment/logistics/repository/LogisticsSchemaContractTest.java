package io.github.will413028.hawkr.payment.logistics.repository;

import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.junit.jupiter.api.Assertions.assertFalse;
import static org.junit.jupiter.api.Assertions.assertNotNull;
import static org.junit.jupiter.api.Assertions.assertTrue;
import static org.junit.jupiter.api.Assertions.fail;

import com.baomidou.mybatisplus.annotation.IdType;
import com.baomidou.mybatisplus.annotation.TableId;
import com.baomidou.mybatisplus.annotation.TableName;
import io.github.will413028.hawkr.common.utils.code.LedgerAccountCode;
import io.github.will413028.hawkr.common.utils.code.LedgerOwnerType;
import io.github.will413028.hawkr.common.utils.code.LedgerTxnType;
import io.github.will413028.hawkr.common.utils.code.LogisticsReceivableStatus;
import io.github.will413028.hawkr.common.utils.code.ProviderFeeType;
import io.github.will413028.hawkr.common.utils.code.FulfillmentCaseResolutionType;
import io.github.will413028.hawkr.common.utils.code.FulfillmentExceptionCategory;
import io.github.will413028.hawkr.common.utils.code.ResponsibilityCause;
import io.github.will413028.hawkr.payment.logistics.domain.ResponsibilityCaseRules;
import io.github.will413028.hawkr.payment.logistics.repository.mapper.FulfillmentExceptionCaseMapper;
import io.github.will413028.hawkr.payment.logistics.repository.entity.FulfillmentCaseResolutionEntity;
import io.github.will413028.hawkr.payment.logistics.repository.entity.FulfillmentExceptionCaseEntity;
import io.github.will413028.hawkr.payment.logistics.repository.entity.LedgerEntryEntity;
import io.github.will413028.hawkr.payment.logistics.repository.entity.LedgerTransactionEntity;
import io.github.will413028.hawkr.payment.logistics.repository.entity.LogisticsAdjustmentEntity;
import io.github.will413028.hawkr.payment.logistics.repository.entity.LogisticsCollectionAttemptEntity;
import io.github.will413028.hawkr.payment.logistics.repository.entity.LogisticsReceivableEntity;
import io.github.will413028.hawkr.payment.logistics.repository.entity.ProviderSettlementLineEntity;
import io.github.will413028.hawkr.payment.logistics.repository.entity.SellerLogisticsAccountEntity;
import java.io.IOException;
import java.io.InputStream;
import java.lang.reflect.Field;
import java.lang.reflect.Method;
import java.lang.reflect.Modifier;
import java.nio.charset.StandardCharsets;
import java.nio.file.Files;
import java.nio.file.Path;
import java.util.ArrayList;
import java.util.LinkedHashMap;
import java.util.LinkedHashSet;
import java.util.List;
import java.util.Map;
import java.util.Set;
import java.util.TreeSet;
import java.util.regex.Matcher;
import java.util.regex.Pattern;
import org.apache.ibatis.annotations.Select;
import org.junit.jupiter.api.DisplayName;
import org.junit.jupiter.api.Test;

/**
 * {@code V9__add_logistics_finance_tables.sql} ↔ logistics entity 的 schema 契約釘樁。
 *
 * <p><b>它證明什麼</b>：9 個 entity 的欄位集合與 V9 的 {@code CREATE TABLE} 欄位集合<b>逐欄一致</b>
 * （雙向：entity 不多、DDL 不少），主鍵策略與 DDL 的 {@code bigint} / {@code bigserial} 對得上，
 * 以及兩支 mapper 裡寫死的 {@code ON CONFLICT … WHERE} 述詞與 V9 的 partial unique index 述詞一字不差。
 *
 * <p><b>為什麼這一支非釘不可</b>：本波是純資料層，<b>零程式碼消費者</b>——沒有任何 service 會因為
 * 欄名打錯、少一欄、主鍵策略選錯而編譯失敗或測試轉紅。在使用端接上來之前，這支測試是這批
 * entity 唯一的自動化守門（與 {@code UberDirectSchemaContractTest} 同一個處境）。
 *
 * <p>🔴 <b>期望值一律硬寫字面值</b>（表名、欄數、主鍵策略、索引述詞），不從被測物反推：
 * 期望值若來自 entity 自己或 V9 自己，這支測試就只是在自己跟自己對，必然全綠。
 * 唯一的例外是「entity 欄位集合 ↔ DDL 欄位集合」那一組——它<b>本來就是</b>兩個獨立來源的對拍，
 * 那正是它的鑑別力所在（改任一邊而不改另一邊就轉紅）。
 *
 * <p><b>它不證明什麼（誠實標註）</b>：這是對 SQL <b>字面</b>與 Java <b>反射</b>的斷言，不是對
 * PostgreSQL 執行結果的斷言。它擋得住「有人把某個欄位改名／刪掉」「有人把 bigserial 的主鍵標成
 * TSID」「有人把 ON CONFLICT 的述詞改壞」；擋不住「這段 DDL 在真的 PostgreSQL 上跑不起來」，
 * 也擋不住「mapper 的 SQL 語意錯了」。後者本波以一次性 DB（V1..V9 依序套用 ＋ CAS／claim 的
 * 拒絕／接受對照）人工驗過，證據附在交付回報裡。
 */
@DisplayName("V9 + V12 logistics schema ↔ entity 契約")
class LogisticsSchemaContractTest {

    private static final String MIGRATION_RESOURCE = "db/migration/V9__add_logistics_finance_tables.sql";
    private static final String MIGRATION_PATH = "src/main/resources/" + MIGRATION_RESOURCE;

    /**
     * V12（WP2A）在既有兩張表上<b>疊加</b>不變式與一個投影欄。
     *
     * <p>🔴 本測試必須讀它，否則「entity 欄位集合 ↔ DDL 欄位集合」那組對拍會在 V12 之後
     * 永遠紅（entity 多一欄而 V9 沒有）——而<b>把那組斷言放寬才是真正的損失</b>：
     * 它是這批 entity 唯一擋得住「欄名打錯／少一欄」的東西。正解是讓 DDL 那一側也看到 V12。
     */
    private static final String MIGRATION_V12_RESOURCE =
            "db/migration/V12__enforce_responsibility_core_finality.sql";
    private static final String MIGRATION_V12_PATH = "src/main/resources/" + MIGRATION_V12_RESOURCE;

    /**
     * V13（WP2A-R2）把責任成因前移到 maker 的提案列。
     *
     * <p>🔴 它在 {@code fulfillment_case_resolutions} 上<b>再加一欄</b>，
     * 且內嵌了 {@code ResponsibilityCause} 四值的<b>第二份複製</b>
     * （第一份在 V12 的 {@code chk_exc_responsibility_resolved_cause}）
     * ⇒ 必須納入逐字對拍，否則 CodeMeta 改值時它會靜默漂移。
     */
    private static final String MIGRATION_V13_RESOURCE =
            "db/migration/V13__bind_responsibility_approval_to_proposal.sql";
    private static final String MIGRATION_V13_PATH = "src/main/resources/" + MIGRATION_V13_RESOURCE;

    /**
     * V14（WP2A-R2 PM Decision）把 N16／N17 的 event context 變成案件自己的欄位。
     *
     * <p>🔴 它在 {@code fulfillment_exception_cases} 上加三欄 ⇒ 不讀它，
     * 「entity 欄位集合 ↔ DDL 欄位集合」那組對拍會永遠紅。
     */
    /**
     * V15：把 receivable 的 identity 錨回 provider fulfillment leg，並補責任成因快照欄
     * （70 §3.1／§3.1.2／§3.6.2）。
     *
     * <p>⚠️ 70 號把這批標成「`V11`，實作階段必辦」，而 `V11` 已被 outbox `exchange` 欄佔用
     * ⇒ 本批改以 `V15` 取號。號碼不同不代表內容不同。
     */
    private static final String MIGRATION_V15_RESOURCE =
            "db/migration/V15__anchor_receivable_identity_on_provider_leg.sql";
    private static final String MIGRATION_V15_PATH = "src/main/resources/" + MIGRATION_V15_RESOURCE;

    private static final String MIGRATION_V14_RESOURCE =
            "db/migration/V14__add_responsibility_case_immutable_context.sql";
    private static final String MIGRATION_V14_PATH = "src/main/resources/" + MIGRATION_V14_RESOURCE;

    private static final String RAW = readMigration(MIGRATION_PATH, MIGRATION_RESOURCE);
    private static final String RAW_V12 = readMigration(MIGRATION_V12_PATH, MIGRATION_V12_RESOURCE);
    private static final String RAW_V13 = readMigration(MIGRATION_V13_PATH, MIGRATION_V13_RESOURCE);
    private static final String RAW_V14 = readMigration(MIGRATION_V14_PATH, MIGRATION_V14_RESOURCE);
    private static final String RAW_V15 = readMigration(MIGRATION_V15_PATH, MIGRATION_V15_RESOURCE);

    /** V12 剝註解後的純 DDL 視圖（與 {@link #CODE} 同樣處理）。 */
    private static final String CODE_V12 = stripLineComments(RAW_V12);

    /** V12 的單行視圖，給「片段包含」類斷言用。 */
    private static final String FLAT_V12 = CODE_V12.replaceAll("\\s+", " ");

    /** V13 剝註解後的純 DDL 視圖。 */
    private static final String CODE_V13 = stripLineComments(RAW_V13);

    /** V13 的單行視圖。 */
    private static final String FLAT_V13 = CODE_V13.replaceAll("\\s+", " ");

    /**
     * V9 之後所有 migration 的單行視圖，供「當前 schema ＝ 建表 ＋ 增量」那組對拍使用。
     *
     * <p>🔴 <b>為什麼是清單而不是只讀最新那一支</b>：每加一支會動欄位的 migration，
     * 就要有人記得回來改這裡，而忘記的症狀是 {@code entityColumnsMatchDdlColumnsExactly}
     * 永遠紅 ⇒ 下一個人最省事的處置是<b>把那組斷言放寬</b>，而那是這批 entity
     * 唯一擋得住「欄名打錯／少一欄」的東西。列成清單讓新增一支只要加一行。
     */
    /** V14 剝註解後的純 DDL 視圖。 */
    private static final String CODE_V14 = stripLineComments(RAW_V14);

    /** V14 的單行視圖。 */
    private static final String FLAT_V14 = CODE_V14.replaceAll("\\s+", " ");

    /** V15 剝註解後的純 DDL 視圖。 */
    private static final String CODE_V15 = stripLineComments(RAW_V15);

    /** V15 的單行視圖。 */
    private static final String FLAT_V15 = CODE_V15.replaceAll("\\s+", " ");

    private static final List<String> POST_V9_MIGRATIONS =
            List.of(FLAT_V12, FLAT_V13, FLAT_V14, FLAT_V15);

    /** 🔴 剝掉 {@code --} 註解後的「純 DDL」視圖。所有斷言都跑在它上面。 */
    private static final String CODE = stripLineComments(RAW);

    /** 空白壓成單一空格的單行視圖，給「片段包含」類斷言用。 */
    private static final String FLAT = CODE.replaceAll("\\s+", " ");

    /**
     * 🔴 硬寫的期望：9 張表 → entity class ＋ 欄位數 ＋ 主鍵策略。
     * 欄位數逐張從 V9 數出來後寫死——它是「有人偷加／偷刪一欄」的最直接偵測。
     */
    private static final List<TableSpec> SPECS = List.of(
            // ⚠️ 20 ＝ V9 的 19 欄 ＋ V15 的 responsibility_cause（resolved responsibility audit snapshot）。
            new TableSpec("logistics_receivables", LogisticsReceivableEntity.class, 20, IdType.INPUT, "id"),
            new TableSpec("logistics_collection_attempts", LogisticsCollectionAttemptEntity.class, 15, IdType.INPUT, "id"),
            new TableSpec("seller_logistics_accounts", SellerLogisticsAccountEntity.class, 6, IdType.INPUT, "store_id"),
            new TableSpec("logistics_adjustments", LogisticsAdjustmentEntity.class, 11, IdType.INPUT, "id"),
            new TableSpec("ledger_transactions", LedgerTransactionEntity.class, 12, IdType.INPUT, "id"),
            // 🔴 唯一一張 bigserial：逐表實查 DDL 決定，不是照抄兄弟表。
            new TableSpec("ledger_entries", LedgerEntryEntity.class, 10, IdType.AUTO, "id"),
            // ⚠️ 25 ＝ V9 的 22 欄 ＋ V14 的 order_no／shipment_id／shipping_method。
            new TableSpec("fulfillment_exception_cases", FulfillmentExceptionCaseEntity.class, 25, IdType.INPUT, "id"),
            // ⚠️ 17 ＝ V9 的 15 欄 ＋ V12 的 case_category 投影欄 ＋ V13 的 proposed_responsibility_cause。
            new TableSpec("fulfillment_case_resolutions", FulfillmentCaseResolutionEntity.class, 17, IdType.INPUT, "id"),
            new TableSpec("provider_settlement_lines", ProviderSettlementLineEntity.class, 13, IdType.INPUT, "id"));

    private record TableSpec(String table, Class<?> entity, int expectedColumns, IdType idType, String idColumn) {
    }

    // ─────────────────────────── 讀檔與剝註解 ───────────────────────────

    private static String readMigration(String path, String resource) {
        Path p = Path.of(path);
        if (Files.isRegularFile(p)) {
            try {
                return Files.readString(p, StandardCharsets.UTF_8);
            } catch (IOException e) {
                throw new IllegalStateException("讀不到 " + path, e);
            }
        }
        try (InputStream in = LogisticsSchemaContractTest.class.getClassLoader()
                .getResourceAsStream(resource)) {
            if (in == null) {
                // 🔴 對「目標不存在」必須 FAIL 而不是 SKIP：檔案被改名或搬走時，
                //    SKIP 會讓這支測試靜默轉綠，而那正是它該叫的時候。
                throw new IllegalStateException(
                        "找不到 migration（檔案系統 " + path + " 與 classpath "
                                + resource + " 都沒有）。本測試拒絕在看不到目標時通過。");
            }
            return new String(in.readAllBytes(), StandardCharsets.UTF_8);
        } catch (IOException e) {
            throw new IllegalStateException("讀不到 " + resource, e);
        }
    }

    /** 逐行剝 {@code --} 行註解，以 in-string 狀態機避開字串字面值裡的 {@code --}。 */
    private static String stripLineComments(String sql) {
        StringBuilder out = new StringBuilder(sql.length());
        boolean inString = false;
        for (String line : sql.split("\n", -1)) {
            int cut = line.length();
            for (int i = 0; i < line.length(); i++) {
                char c = line.charAt(i);
                if (c == '\'') {
                    inString = !inString;
                } else if (!inString && c == '-' && i + 1 < line.length() && line.charAt(i + 1) == '-') {
                    cut = i;
                    break;
                }
            }
            out.append(line, 0, cut).append('\n');
        }
        return out.toString();
    }

    // ─────────────────────────── helpers ───────────────────────────

    /** 取某張表的 {@code CREATE TABLE "x" ( … );} 內文；找不到就 fail（不是回空字串）。 */
    private static String tableBody(String table) {
        Matcher m = Pattern.compile(
                "CREATE TABLE \"" + Pattern.quote(table) + "\" \\((.*?)\\n\\);", Pattern.DOTALL).matcher(CODE);
        if (!m.find()) {
            fail("找不到 CREATE TABLE \"" + table + "\" —— 表被改名或刪掉了？");
        }
        return m.group(1);
    }

    /**
     * 從表內文取欄位名（依 DDL 出現順序）；{@code CONSTRAINT …} 行不算欄位。
     *
     * <p>🔴 <b>疊加 V12 的 {@code ALTER TABLE … ADD COLUMN}</b>：這一組對拍問的是
     * 「entity 與<b>當前</b> schema 一不一致」，而當前 schema ＝ V9 建表 ＋ 後續 migration 的增量。
     * 只讀 V9 會讓 V12 之後的每一個新欄位都被誤報成「entity 多出來的」。
     */
    private static List<String> ddlColumns(String table) {
        List<String> cols = new ArrayList<>();
        Pattern col = Pattern.compile("^\\s*\"([a-z_]+)\"\\s+[a-z]");
        for (String line : tableBody(table).split("\n")) {
            if (line.trim().toUpperCase().startsWith("CONSTRAINT")) {
                continue;
            }
            Matcher m = col.matcher(line);
            if (m.find()) {
                cols.add(m.group(1));
            }
        }
        cols.addAll(addedColumnsAfterV9(table));
        return cols;
    }

    /**
     * V9 之後各 migration 對某張表新增的欄位（{@code ALTER TABLE "t" ADD COLUMN "c" type}）。
     *
     * <p>⚠️ <b>本 helper 擋不住什麼</b>（誠實標明邊界，不要把它當成完整的 schema 重放）：
     * 它只看 {@code ADD COLUMN}，<b>不處理</b> {@code DROP COLUMN} 與 {@code RENAME COLUMN}。
     * 日後真的出現那兩種變更時，這組對拍會紅——那是正確的訊號（去補這裡），
     * 不是放寬斷言的理由。
     */
    private static List<String> addedColumnsAfterV9(String table) {
        List<String> added = new ArrayList<>();
        // 🔴 先切出屬於這張表的整句 ALTER TABLE（到分號為止），再在句內找**每一個** ADD COLUMN。
        //    ⚠️ 舊版寫成 `ALTER TABLE "t" ADD COLUMN "x"` 一次匹配，只抓得到<b>第一個</b>子句
        //    ⇒ 一句 ALTER 加三欄時會靜默少算兩欄（V14 就是這個形狀，被
        //    columnCountsMatchHardcodedExpectation 抓出來）。
        Pattern stmt = Pattern.compile(
                "ALTER TABLE \"" + Pattern.quote(table) + "\"(.*?);", Pattern.DOTALL);
        Pattern col = Pattern.compile("ADD COLUMN \"([a-z_]+)\"");
        for (String migration : POST_V9_MIGRATIONS) {
            Matcher s = stmt.matcher(migration);
            while (s.find()) {
                Matcher c = col.matcher(s.group(1));
                while (c.find()) {
                    added.add(c.group(1));
                }
            }
        }
        return added;
    }

    /** entity 的持久化欄位名（camelCase → snake_case），跳過 static／synthetic（Lombok 產物）。 */
    private static Set<String> entityColumns(Class<?> entity) {
        Set<String> cols = new LinkedHashSet<>();
        for (Field f : entity.getDeclaredFields()) {
            if (Modifier.isStatic(f.getModifiers()) || f.isSynthetic()) {
                continue;
            }
            TableId id = f.getAnnotation(TableId.class);
            if (id != null && !id.value().isEmpty()) {
                cols.add(id.value());
                continue;
            }
            cols.add(toSnake(f.getName()));
        }
        return cols;
    }

    private static String toSnake(String camel) {
        StringBuilder sb = new StringBuilder(camel.length() + 4);
        for (char c : camel.toCharArray()) {
            if (Character.isUpperCase(c)) {
                sb.append('_').append(Character.toLowerCase(c));
            } else {
                sb.append(c);
            }
        }
        return sb.toString();
    }

    // ─────────────────── 剝除器自身的鑑別力（它壞掉會讓每條否定斷言自動通過） ───────────────────

    @Test
    @DisplayName("註解剝除器：留得住 DDL、吃得掉註解")
    void stripperKeepsDdlAndDropsComments() {
        assertTrue(CODE.contains("CREATE TABLE \"logistics_receivables\" ("), "剝除器把 CREATE TABLE 吃掉了");
        assertTrue(CODE.contains("CREATE CONSTRAINT TRIGGER trg_ledger_balanced"), "剝除器把 TRIGGER 吃掉了");
        assertTrue(CODE.contains("CREATE SEQUENCE IF NOT EXISTS \"ledger_txn_no_seq\""), "剝除器把 SEQUENCE 吃掉了");

        // V9 的註解裡刻意提到「沒有搬進來的表」。剝乾淨了，這些字串就不該出現。
        assertFalse(CODE.contains("payout_instructions"), "剝除器沒有真的剝掉 -- 註解（註解裡才有 payout_instructions）");
        assertFalse(CODE.contains("settlement_candidates"), "剝除器沒有真的剝掉 -- 註解");
        assertFalse(CODE.contains("架構裁決"), "剝除器沒有真的剝掉 -- 註解");
    }

    // ─────────────────────────── 主契約 ───────────────────────────

    @Test
    @DisplayName("V9 恰好建立這 9 張表，一張不多一張不少")
    void migrationCreatesExactlyTheNineTables() {
        Set<String> found = new TreeSet<>();
        Matcher m = Pattern.compile("CREATE TABLE \"([a-z_]+)\" \\(").matcher(CODE);
        while (m.find()) {
            found.add(m.group(1));
        }
        Set<String> expected = new TreeSet<>(SPECS.stream().map(TableSpec::table).toList());
        assertEquals(expected, found, "V9 建立的表與契約不符（多了或少了一張）");
    }

    @Test
    @DisplayName("每個 entity 的欄位集合 = 該表 DDL 的欄位集合（雙向逐欄）")
    void entityColumnsMatchDdlColumnsExactly() {
        Map<String, String> failures = new LinkedHashMap<>();
        for (TableSpec spec : SPECS) {
            List<String> ddl = ddlColumns(spec.table());
            Set<String> ddlSet = new TreeSet<>(ddl);
            Set<String> entitySet = new TreeSet<>(entityColumns(spec.entity()));

            Set<String> missingInEntity = new TreeSet<>(ddlSet);
            missingInEntity.removeAll(entitySet);
            Set<String> extraInEntity = new TreeSet<>(entitySet);
            extraInEntity.removeAll(ddlSet);

            if (!missingInEntity.isEmpty() || !extraInEntity.isEmpty()) {
                failures.put(spec.table(),
                        "entity 缺少 " + missingInEntity + "；entity 多出 " + extraInEntity);
            }
        }
        assertTrue(failures.isEmpty(), "entity 與 V9 DDL 的欄位集合不一致：\n" + failures);
    }

    @Test
    @DisplayName("每張表的欄位數與硬寫的期望相符（偷加／偷刪一欄的直接偵測）")
    void columnCountsMatchHardcodedExpectation() {
        Map<String, String> failures = new LinkedHashMap<>();
        for (TableSpec spec : SPECS) {
            int actual = ddlColumns(spec.table()).size();
            if (actual != spec.expectedColumns()) {
                failures.put(spec.table(), "期望 " + spec.expectedColumns() + " 欄，DDL 實得 " + actual);
            }
        }
        assertTrue(failures.isEmpty(), "欄位數與契約不符：\n" + failures);
    }

    @Test
    @DisplayName("@TableName 指向正確的表，且 JSONB entity 一律 autoResultMap = true")
    void tableNameAnnotationsAreCorrect() {
        for (TableSpec spec : SPECS) {
            TableName ann = spec.entity().getAnnotation(TableName.class);
            assertNotNull(ann, spec.entity().getSimpleName() + " 缺 @TableName");
            assertEquals(spec.table(), ann.value(),
                    spec.entity().getSimpleName() + " 的 @TableName 指錯表");
            // 🔴 有 TypeHandler 欄位的 entity 少了 autoResultMap，JSONB 取回時不走 handler。
            assertTrue(ann.autoResultMap(),
                    spec.entity().getSimpleName() + " 必須 autoResultMap = true");
        }
    }

    @Test
    @DisplayName("主鍵策略與 DDL 的 bigint / bigserial 對得上（ledger_entries 是唯一的 AUTO）")
    void primaryKeyStrategyMatchesDdl() {
        for (TableSpec spec : SPECS) {
            Field idField = null;
            for (Field f : spec.entity().getDeclaredFields()) {
                if (f.getAnnotation(TableId.class) != null) {
                    idField = f;
                    break;
                }
            }
            assertNotNull(idField, spec.entity().getSimpleName() + " 缺 @TableId");
            TableId id = idField.getAnnotation(TableId.class);
            assertEquals(spec.idType(), id.type(),
                    spec.entity().getSimpleName() + " 的主鍵策略與 DDL 不符");

            // 反向：DDL 裡該欄的型別必須與策略一致。
            String body = tableBody(spec.table());
            boolean isSerial = Pattern.compile("\"" + Pattern.quote(spec.idColumn()) + "\"\\s+bigserial")
                    .matcher(body).find();
            boolean isBigint = Pattern.compile("\"" + Pattern.quote(spec.idColumn()) + "\"\\s+bigint")
                    .matcher(body).find();
            assertTrue(isSerial || isBigint,
                    spec.table() + "." + spec.idColumn() + " 在 DDL 裡既不是 bigint 也不是 bigserial");
            if (spec.idType() == IdType.AUTO) {
                assertTrue(isSerial, spec.table() + " 標了 IdType.AUTO，但 DDL 的主鍵不是 bigserial");
            } else {
                assertTrue(isBigint, spec.table() + " 標了 IdType.INPUT，但 DDL 的主鍵不是 bigint");
            }
        }
    }

    @Test
    @DisplayName("append-only 的兩張表沒有 updated_at（entity 也不得憑空長出來）")
    void appendOnlyTablesHaveNoUpdatedAt() {
        // ledger_transactions：更正一律開反向 MANUAL_ADJUSTMENT，不改原列。
        assertFalse(ddlColumns("ledger_transactions").contains("updated_at"),
                "ledger_transactions 是 append-only，不該有 updated_at");
        assertFalse(entityColumns(LedgerTransactionEntity.class).contains("updated_at"),
                "LedgerTransactionEntity 不該有 updatedAt");
        // logistics_adjustments：調整單是憑據，寫錯開反向的。
        assertFalse(ddlColumns("logistics_adjustments").contains("updated_at"),
                "logistics_adjustments 不該有 updated_at");
        // provider_settlement_lines：時間軸是 imported_at。
        assertFalse(ddlColumns("provider_settlement_lines").contains("created_at"),
                "provider_settlement_lines 的時間軸是 imported_at，不該有 created_at");
        assertTrue(ddlColumns("provider_settlement_lines").contains("imported_at"),
                "provider_settlement_lines 少了 imported_at");
    }

    @Test
    @DisplayName("租約欄存在，且沒有任何一張表用『已送出／已查過』式的永久旗標")
    void leaseColumnsExistAndNoPermanentFlags() {
        assertTrue(ddlColumns("logistics_receivables").contains("submit_lease_until"),
                "logistics_receivables 少了租約欄 submit_lease_until（10 檔 §29）");
        assertTrue(ddlColumns("logistics_collection_attempts").contains("query_lease_until"),
                "logistics_collection_attempts 少了租約欄 query_lease_until");

        // 🔴 永久旗標的形狀：布林／時間戳以外的「做過了」欄位。這兩個名字是最容易被加回來的。
        assertFalse(ddlColumns("logistics_receivables").contains("submitted"),
                "logistics_receivables 出現永久旗標 submitted —— 租約被換成旗標了");
        assertFalse(ddlColumns("logistics_collection_attempts").contains("queried"),
                "logistics_collection_attempts 出現永久旗標 queried");

        // 租約欄必須有支援它的 partial index，否則 claim 會全表掃。
        assertTrue(FLAT.contains("CREATE INDEX \"idx_logrecv_submit_lease\" ON \"logistics_receivables\" (\"submit_lease_until\")"),
                "submit_lease_until 的索引不見了");
        assertTrue(FLAT.contains("CREATE INDEX \"idx_logcoll_query_lease\" ON \"logistics_collection_attempts\" (\"query_lease_until\")"),
                "query_lease_until 的索引不見了");
    }

    /**
     * 🔴 mapper 裡寫死的 {@code ON CONFLICT … WHERE} 述詞 ↔ V9 的 partial unique index 述詞。
     *
     * <p>那兩個字面值<b>不得改成 {@code #{}} 參數</b>：PostgreSQL 17.9 實測，參數化的述詞只在前
     * 5 次 EXECUTE（custom plan）成立，第 6 次切成 generic plan 後永久 42P10；pgjdbc 的
     * {@code prepareThreshold} 預設同樣是 5 ⇒ 本機與 CI 全綠、上線第 6 次才炸。
     * 既然只能寫死，就必須有東西盯著它與 DDL 一致——就是這一支。
     */
    @Test
    @DisplayName("mapper 的 ON CONFLICT 述詞與 V9 的 partial unique index 述詞一字不差")
    void onConflictPredicatesMatchPartialIndexPredicates() throws IOException {
        // V9 側：兩個 partial unique index 的述詞。
        assertTrue(FLAT.contains("CREATE UNIQUE INDEX \"uq_fcr_open_per_case\" ON \"fulfillment_case_resolutions\" (\"case_id\") WHERE \"status\" = 'AWAITING_REVIEW'"),
                "uq_fcr_open_per_case 的述詞變了（或索引不見了）——mapper 的 ON CONFLICT 要一起改");
        assertTrue(FLAT.contains("CREATE UNIQUE INDEX \"uq_exc_open_dedup\" ON \"fulfillment_exception_cases\" (\"dedup_key\")"),
                "uq_exc_open_dedup 不見了");
        assertTrue(FLAT.contains("'OPEN'::character varying, 'IN_PROGRESS'::character varying"),
                "uq_exc_open_dedup 的狀態述詞變了——mapper 的 ON CONFLICT 要一起改");

        // mapper 側：字面值述詞必須在，且不得退化成參數。
        String excMapper = readMapper("FulfillmentExceptionCaseMapper.java");
        assertTrue(excMapper.contains("ON CONFLICT (dedup_key) WHERE status IN ('OPEN', 'IN_PROGRESS')"),
                "FulfillmentExceptionCaseMapper 的 ON CONFLICT 述詞與 V9 的索引述詞對不上");
        assertFalse(excMapper.contains("ON CONFLICT (dedup_key) WHERE status IN (#{"),
                "ON CONFLICT 的述詞被改成參數了 —— generic plan 下會永久 42P10（實測）");

        String fcrMapper = readMapper("FulfillmentCaseResolutionMapper.java");
        assertTrue(fcrMapper.contains("ON CONFLICT (case_id) WHERE status = 'AWAITING_REVIEW'"),
                "FulfillmentCaseResolutionMapper 的 ON CONFLICT 述詞與 V9 的索引述詞對不上");
        assertFalse(fcrMapper.contains("ON CONFLICT (case_id) WHERE status = #{"),
                "ON CONFLICT 的述詞被改成參數了 —— generic plan 下會永久 42P10（實測）");
    }

    /**
     * 🔴 CAS 的 allowed-from 不得在 SQL 裡另有一份：{@code LogisticsReceivableMapper} 的每一句
     * {@code UPDATE … SET status} 都必須把來源狀態當<b>參數</b>收，自己不持有任何狀態字面值。
     */
    @Test
    @DisplayName("LogisticsReceivableMapper 的 SQL 不含任何應收狀態字面值（allowed-from 只有 Rules 一份）")
    void receivableMapperSqlHoldsNoStatusLiterals() throws IOException {
        String src = readMapper("LogisticsReceivableMapper.java");
        // 只看 text block 裡的 SQL：javadoc 提到狀態名是說明，不是碼值的第二份來源。
        StringBuilder sql = new StringBuilder();
        Matcher m = Pattern.compile("\"\"\"(.*?)\"\"\"", Pattern.DOTALL).matcher(src);
        while (m.find()) {
            sql.append(m.group(1)).append('\n');
        }
        assertFalse(sql.isEmpty(), "抽不到任何 SQL text block —— 抽取器壞了（不是 mapper 沒 SQL）");

        for (String status : List.of("PENDING", "SUBMITTED", "SETTLED", "UNCONFIRMED",
                "FAILED", "CLOSED", "WRITTEN_OFF")) {
            assertFalse(sql.toString().contains("'" + status + "'"),
                    "LogisticsReceivableMapper 的 SQL 出現狀態字面值 '" + status
                            + "' —— allowed-from 必須只從 ReceivableStatusRules 取，SQL 不得另有一份");
        }
        // 正向：CAS 與 claim 的關鍵片段確實在（否則上面的否定斷言可能只是因為 SQL 被刪光了）。
        assertTrue(sql.toString().contains("FOR UPDATE SKIP LOCKED"),
                "claim 少了 FOR UPDATE SKIP LOCKED");
        assertTrue(sql.toString().contains("make_interval(secs => #{leaseSeconds})"),
                "租約基準不是在 claim 當下逐筆取（10 檔 §29）");
        assertTrue(sql.toString().contains("<foreach collection=\"allowedFrom\""),
                "CAS 的 allowed-from 不是由呼叫端傳入");
    }

    /**
     * 🔴 <b>CodeMeta 碼值 ↔ V9 欄寬</b>：每個會落進 {@code ledger_entries.account_code} 的
     * {@code LedgerAccountCode} 都必須塞得進 DDL 宣告的寬度。
     *
     * <p><b>為什麼需要這一支</b>：碼值住在 CodeMeta（{@code 內部技術碼.md}），欄寬住在 V9，
     * 兩者<b>沒有任何東西在對拍</b>。碼值加長一個字元，症狀是執行期
     * {@code ERROR: value too long for type character varying(32)}（SQLSTATE 22001），
     * 而編譯、單元測試、CI 全部是綠的——本波零消費者更是連執行期都碰不到。
     *
     * <p>🔴🔴 <b>本輪實測發現的既存缺陷（V9 待修，不是本測試的誤報）</b>：
     * {@code PLATFORM_LOGISTICS_SERVICE_INCOME} 是 <b>33</b> 字元，而
     * {@code ledger_entries.account_code} 是 {@code varchar(32)}。
     * PostgreSQL 17.9 一次性 DB（V1..V9）實測：
     * <pre>
     * INSERT … account_code = 'SELLER_LOGISTICS_RECEIVABLE'        (27 字元) → INSERT 0 1   ← 對照組
     * INSERT … account_code = 'PLATFORM_LOGISTICS_SERVICE_INCOME'  (33 字元) → ERROR 22001
     * </pre>
     * 它是 {@code LOGISTICS_RECEIVABLE_ACCRUAL} 的<b>貸方科目</b>，而
     * {@code LogisticsLedgerBuilders} 有 <b>4 處</b>會產生它
     * （{@code logisticsReceivableAccrual}／{@code logisticsTrueUp} 兩個方向／{@code logisticsCreditIssue}）
     * ⇒ <b>線 B 的第一筆帳本交易就寫不進去</b>。
     * <p><b>修法</b>：新增 {@code V10} 做
     * {@code ALTER TABLE ledger_entries ALTER COLUMN account_code TYPE varchar(64)}
     * （禁止改已 commit 的 V9，CLAUDE.md §11）。<b>不是</b>把碼值改短——那是跨 repo 的破壞性變更。
     *
     * <p>⚠️ {@link #KNOWN_TOO_LONG} 是 <b>REMOVE-WHEN 式的自我移除例外</b>，不是白名單：
     * 本測試要求它<b>剛好等於</b>實際超長的集合 ⇒ ① 再有第二個碼值超長 → 紅
     * ② V10 修好之後它變空而清單沒清掉 → 也紅，逼人回來刪掉這段。
     */
    private static final Set<String> KNOWN_TOO_LONG = Set.of("PLATFORM_LOGISTICS_SERVICE_INCOME");

    @Test
    @DisplayName("LedgerAccountCode 全部塞得進 account_code 的欄寬（既存缺陷以自我移除例外釘住）")
    void ledgerAccountCodesFitDeclaredColumnWidth() {
        String body = tableBody("ledger_entries");
        Matcher m = Pattern.compile("\"account_code\"\\s+varchar\\((\\d+)\\)").matcher(body);
        assertTrue(m.find(), "讀不到 ledger_entries.account_code 的宣告寬度");
        int width = Integer.parseInt(m.group(1));
        assertEquals(32, width, "account_code 的欄寬變了 —— 請同步檢討下面的 KNOWN_TOO_LONG");

        Set<String> tooLong = new TreeSet<>();
        for (LedgerAccountCode c : LedgerAccountCode.values()) {
            if (c.code().length() > width) {
                tooLong.add(c.code());
            }
        }
        assertEquals(new TreeSet<>(KNOWN_TOO_LONG), tooLong,
                "LedgerAccountCode 與 account_code varchar(" + width + ") 的相符情形改變了。\n"
                        + "  實際超長：" + tooLong + "\n"
                        + "  已知超長：" + new TreeSet<>(KNOWN_TOO_LONG) + "\n"
                        + "  ① 多出來的 → 新的 22001 執行期缺陷，請加寬欄位（新增 V10 ALTER，不得改 V9）\n"
                        + "  ② 少掉的   → 缺陷已修，請把它從 KNOWN_TOO_LONG 移除並刪掉這段說明");
    }

    /**
     * 🔴 同一形狀的第二個面：其餘落進固定寬度欄位的 CodeMeta 碼值必須全部塞得下。
     * 這一支<b>沒有</b>例外清單——它現在就該全綠，任何一個超長都是新缺陷。
     */
    @Test
    @DisplayName("其餘 CodeMeta 碼值都塞得進 V9 宣告的欄寬（無例外清單）")
    void otherCodeMetaValuesFitDeclaredWidths() {
        assertCodesFit("ledger_transactions", "txn_type",
                java.util.Arrays.stream(LedgerTxnType.values()).map(LedgerTxnType::code).toList());
        assertCodesFit("ledger_entries", "owner_type",
                java.util.Arrays.stream(LedgerOwnerType.values()).map(LedgerOwnerType::code).toList());
        assertCodesFit("logistics_receivables", "status",
                java.util.Arrays.stream(LogisticsReceivableStatus.values())
                        .map(LogisticsReceivableStatus::value).toList());
        assertCodesFit("logistics_adjustments", "responsibility_cause",
                java.util.Arrays.stream(ResponsibilityCause.values())
                        .map(ResponsibilityCause::value).toList());
        assertCodesFit("provider_settlement_lines", "fee_type",
                java.util.Arrays.stream(ProviderFeeType.values()).map(ProviderFeeType::code).toList());
    }

    private static void assertCodesFit(String table, String column, List<String> codes) {
        String body = tableBody(table);
        Matcher m = Pattern.compile("\"" + Pattern.quote(column) + "\"\\s+varchar\\((\\d+)\\)").matcher(body);
        assertTrue(m.find(), "讀不到 " + table + "." + column + " 的宣告寬度");
        int width = Integer.parseInt(m.group(1));
        // 🔴 對照組：抓得到寬度且寬度合理，證明這個斷言不是因為 regex 落空而空轉。
        assertTrue(width > 0 && !codes.isEmpty(),
                "量測本身失效：" + table + "." + column + " width=" + width + " codes=" + codes.size());
        for (String c : codes) {
            assertTrue(c.length() <= width,
                    table + "." + column + " 是 varchar(" + width + ")，但碼值 '" + c
                            + "' 有 " + c.length() + " 字元 ⇒ 寫入時會 22001。"
                            + "修法是新增 V{N+1} ALTER 加寬欄位，不是改短碼值（跨 repo 破壞性變更）");
        }
    }

    // ═══════════════════ V12：責任判定核心不變式（WP2A）═══════════════════

    /**
     * 🔴 四道不變式的 DDL 逐字釘樁。
     *
     * <p>它們是「買家拿不拿得到退款」這件事的機械形式，而<b>每一道失效時的症狀都是靜默的</b>：
     * 沒有 CAS 失敗、沒有例外、沒有錯誤碼，只有 Order 停在 35 而 aging 指標一路長上去。
     * ⇒ 只有這支測試會在有人改動或移除它們時出聲。
     */
    @Test
    @DisplayName("V12 的四道責任判定不變式逐字存在")
    void v12ResponsibilityInvariantsPinned() {
        // ① lifetime uniqueness：🔴 述詞裡**不得**出現 status
        assertTrue(FLAT_V12.contains(
                        "CREATE UNIQUE INDEX \"uq_exc_responsibility_subject\" "
                                + "ON \"fulfillment_exception_cases\" (\"subject_type\", \"subject_ref\") "
                                + "WHERE \"category\" = 'RESPONSIBILITY_UNRESOLVED';"),
                "uq_exc_responsibility_subject 的述詞變了（或索引不見了）——"
                        + "它是「同一 leg 終其一生只有一張責任案件」的唯一機械形式");
        assertFalse(responsibilityIndexPredicate().contains("status"),
                "🔴 uq_exc_responsibility_subject 的述詞出現了 status ⇒ 它退化成「只有未結案時唯一」，"
                        + "案件一旦結案就能再開第二張，而 finality 條件 ③ 是靠 lifetime 唯一撐住的");

        // ② DISMISSED 禁令
        assertTrue(FLAT_V12.contains(
                        "CONSTRAINT \"chk_exc_responsibility_no_dismiss\" CHECK ( "
                                + "NOT (\"category\" = 'RESPONSIBILITY_UNRESOLVED' AND \"status\" = 'DISMISSED'))"),
                "chk_exc_responsibility_no_dismiss 變了或不見了");

        // ③ 結案碼值域（四個 resolved cause）
        assertTrue(FLAT_V12.contains("CONSTRAINT \"chk_exc_responsibility_resolved_cause\""),
                "chk_exc_responsibility_resolved_cause 不見了");

        // ④ type ↔ category 雙向綁定（跨表，靠複合 FK 把 category 投影進來）
        assertTrue(FLAT_V12.contains(
                        "CONSTRAINT \"fk_fcr_case_category\" FOREIGN KEY (\"case_id\", \"case_category\") "
                                + "REFERENCES \"fulfillment_exception_cases\" (\"id\", \"category\")"),
                "fk_fcr_case_category 不見了 ⇒ case_category 變成「寫入者說了算」的欄位，"
                        + "那比留在應用層更糟——它看起來像 DB 守住了");
        assertTrue(FLAT_V12.contains(
                        "CONSTRAINT \"chk_fcr_responsibility_type_binding\" CHECK ( "
                                + "(\"case_category\" = 'RESPONSIBILITY_UNRESOLVED') "
                                + "= (\"resolution_type\" = 'RESPONSIBILITY_ATTRIBUTE'))"),
                "chk_fcr_responsibility_type_binding 變了或不見了。🔴 它必須是**雙向等式**："
                        + "單向會讓其中一個方向靜默通過，而兩個方向的後果都是 finality 永遠不成立");
        // 複合 FK 的被參照側必須有恰好匹配的 UNIQUE，否則 FK 根本建不起來
        assertTrue(FLAT_V12.contains("CONSTRAINT \"uq_exc_id_category\" UNIQUE (\"id\", \"category\")"),
                "uq_exc_id_category 不見了 ⇒ fk_fcr_case_category 指不到被參照側");
    }

    /**
     * 🔴 V12 內嵌的<b>碼表複製</b>必須與 CodeMeta 一致。
     *
     * <p>V12 有兩處複製：{@code category} 的單一字面值（低風險，與 V9 既有作法同源）與
     * <b>四個 {@code ResponsibilityCause} 值</b>（高風險，那是一整份碼表）。
     * 後者沒有這支對拍的話，CodeMeta 改值時 DB CHECK 會靜默漂移——而症狀是
     * 「某個成因突然結不了案」或「一個不該能結案的值通過了」。
     *
     * <p>⚠️ 期望值<b>從 CodeMeta 推導</b>、實際值<b>從 DDL 文字抓</b>：兩個獨立來源的對拍，
     * 改任一邊而不改另一邊就轉紅。這正是它的鑑別力所在。
     */
    @Test
    @DisplayName("V12 內嵌的責任成因值域 = CodeMeta 的四個 resolved 值（一個不多一個不少）")
    void v12ResolvedCauseCheckMatchesCodeMeta() {
        Matcher m = Pattern.compile(
                "chk_exc_responsibility_resolved_cause.*?\"resolution_code\" NOT IN \\(([^)]*)\\)",
                Pattern.DOTALL).matcher(FLAT_V12);
        assertTrue(m.find(), "抓不到 chk_exc_responsibility_resolved_cause 的值域清單");
        Set<String> inDdl = new TreeSet<>();
        Matcher lit = Pattern.compile("'([A-Z_]+)'").matcher(m.group(1));
        while (lit.find()) {
            inDdl.add(lit.group(1));
        }

        // 期望＝CodeMeta 的四個 resolved 值。🔴 刻意不硬寫四個字串：
        //    ResponsibilityCause 日後新增第 5 值（已登記 RESPONSIBILITY_UNRESOLVED）時，
        //    這裡會把它算進來而 DDL 沒有 ⇒ 轉紅 ⇒ 逼人回來決定那個值該不該能結案。
        Set<String> expected = new TreeSet<>();
        for (ResponsibilityCause c : ResponsibilityCause.values()) {
            expected.add(c.value());
        }
        // 對照組：抓到的清單非空，證明這個斷言不是因為 regex 落空而空轉。
        assertFalse(inDdl.isEmpty(), "量測本身失效：從 DDL 抓到 0 個碼值");

        assertEquals(expected, inDdl,
                "V12 的結案碼值域與 CodeMeta ResponsibilityCause 不一致。"
                        + "若 CodeMeta 新增了值，必須先決定它能不能作為結案碼："
                        + "能 ⇒ 同步 V{N+1} 加進 CHECK；不能 ⇒ 本斷言要改成排除清單並寫下理由。"
                        + "🔴 不得為了讓測試變綠而直接改期望值");
        assertFalse(inDdl.contains("SYSTEM_CAUSED"),
                "SYSTEM_CAUSED 不存在於 CodeMeta（平台／系統原因一律歸 PLATFORM_CAUSED）");
    }

    /**
     * 🔴 <b>V13 的四值是 {@code ResponsibilityCause} 在 schema 裡的第二份複製</b>
     * （第一份在 V12 的 {@code chk_exc_responsibility_resolved_cause}）。
     *
     * <p>V12 該節自己寫明：碼表複製必須納入逐字對拍，否則 CodeMeta 改值時它會靜默漂移。
     * 本測試就是 V13 那一份的對拍。
     *
     * <p>⚠️ 與 V12 那支<b>不是</b>重複：兩支釘的是<b>不同的欄位</b>
     * （{@code fulfillment_exception_cases.resolution_code} vs
     * {@code fulfillment_case_resolutions.proposed_responsibility_cause}）。
     * 只釘其中一份，另一份就會在改值那天靜默走偏——而它們必須永遠相等，
     * 因為 approve 會把後者原封不動寫進前者。
     */
    @Test
    @DisplayName("V13 內嵌的提案成因值域 = CodeMeta 的四個 resolved 值（一個不多一個不少）")
    void v13ProposedCauseCheckMatchesCodeMeta() {
        Matcher m = Pattern.compile(
                "chk_fcr_proposed_cause_domain.*?IN \\(([^)]*)\\)",
                Pattern.DOTALL).matcher(FLAT_V13);
        assertTrue(m.find(), "抓不到 chk_fcr_proposed_cause_domain 的值域清單");
        Set<String> inDdl = new TreeSet<>();
        Matcher lit = Pattern.compile("'([A-Z_]+)'").matcher(m.group(1));
        while (lit.find()) {
            inDdl.add(lit.group(1));
        }

        Set<String> expected = new TreeSet<>();
        for (ResponsibilityCause c : ResponsibilityCause.values()) {
            expected.add(c.value());
        }
        // 對照組：抓到的清單非空，證明這個斷言不是因為 regex 落空而空轉。
        assertFalse(inDdl.isEmpty(), "量測本身失效：從 V13 DDL 抓到 0 個碼值");

        assertEquals(expected, inDdl,
                "V13 的提案成因值域與 CodeMeta ResponsibilityCause 不一致。"
                        + "🔴 不得為了讓測試變綠而直接改期望值");
        assertFalse(inDdl.contains("RESPONSIBILITY_UNRESOLVED"),
                "RESPONSIBILITY_UNRESOLVED 是未結案期間的 category，不是可提出的成因——"
                        + "讓它進來會在 approve 時撞 chk_exc_responsibility_resolved_cause（500）");
    }

    /**
     * 🔴 <b>V15 的四值是 {@code ResponsibilityCause} 在 schema 裡的第三份複製</b>
     * （前兩份在 V12 的 {@code chk_exc_responsibility_resolved_cause} 與 V13 的
     * {@code chk_fcr_proposed_cause_domain}）。
     *
     * <p>V15 自己的檔頭寫著「必須納入逐字對拍」——本測試就是那句話的兌現。
     * ⚠️ <b>一句寫在 migration 註解裡、卻沒有人兌現的承諾，比沒寫更糟</b>：
     * 下一個人讀到它會以為已經有東西在看（`.claude/rules/02-judgment.md` §2.1.2）。
     *
     * <p>⚠️ 與前兩支<b>不是</b>重複：三支釘的是三張<b>不同表</b>的不同欄位
     * （`fulfillment_exception_cases.resolution_code` ／
     * `fulfillment_case_resolutions.proposed_responsibility_cause` ／
     * `logistics_receivables.responsibility_cause`）。少釘任何一份，
     * 它就會在 CodeMeta 改值那天靜默走偏。
     */
    @Test
    @DisplayName("V15 內嵌的應收成因快照值域 = CodeMeta 的四個 resolved 值（一個不多一個不少）")
    void v15ReceivableCauseCheckMatchesCodeMeta() {
        Matcher m = Pattern.compile(
                "chk_logrecv_responsibility_cause.*?IN \\(([^)]*)\\)",
                Pattern.DOTALL).matcher(FLAT_V15);
        assertTrue(m.find(), "抓不到 chk_logrecv_responsibility_cause 的值域清單");
        Set<String> inDdl = new TreeSet<>();
        Matcher lit = Pattern.compile("'([A-Z_]+)'").matcher(m.group(1));
        while (lit.find()) {
            inDdl.add(lit.group(1));
        }

        Set<String> expected = new TreeSet<>();
        for (ResponsibilityCause c : ResponsibilityCause.values()) {
            expected.add(c.value());
        }
        // 對照組：抓到的清單非空，證明這個斷言不是因為 regex 落空而空轉。
        assertFalse(inDdl.isEmpty(), "量測本身失效：從 V15 DDL 抓到 0 個碼值");

        assertEquals(expected, inDdl,
                "V15 的應收成因快照值域與 CodeMeta ResponsibilityCause 不一致。"
                        + "🔴 不得為了讓測試變綠而直接改期望值——這一欄決定 ADM 對帳時"
                        + "看到的是誰的責任");
        assertFalse(inDdl.contains("SYSTEM_CAUSED"),
                "SYSTEM_CAUSED 不存在於 CodeMeta（平台／系統原因一律歸 PLATFORM_CAUSED）");
        assertFalse(inDdl.contains("RESPONSIBILITY_UNRESOLVED"),
                "應收只在 resolved 之後才建立 ⇒ 本欄的值域只有 4 個 resolved 值。"
                        + "classification 本身是 5 值——那是分工不是矛盾（70 §3.6.2）");
    }

    /**
     * 🔴 <b>兩份複製必須相等</b>：approve 把 {@code proposed_responsibility_cause} 原封不動寫進
     * {@code resolution_code}。兩邊值域若不同，就會存在「提得出來但結不了案」的成因
     * ⇒ 案件永久卡在 {@code IN_PROGRESS}，而且每一步都「照規則成功」。
     *
     * <p>⚠️ 這支與上面兩支<b>不是</b>重複：那兩支各自對拍「DDL ↔ CodeMeta」，
     * 這支對拍「DDL ↔ DDL」。前兩支同時被改成同一個錯值時，只有這支會出聲。
     */
    @Test
    @DisplayName("V13 提案成因值域 = V12 結案成因值域（兩份複製必須逐字相等）")
    void v13ProposedCauseDomainEqualsV12ResolvedCauseDomain() {
        Set<String> proposed = literalsIn(FLAT_V13, "chk_fcr_proposed_cause_domain.*?IN \\(([^)]*)\\)");
        Set<String> resolved = literalsIn(FLAT_V12,
                "chk_exc_responsibility_resolved_cause.*?\"resolution_code\" NOT IN \\(([^)]*)\\)");
        assertFalse(proposed.isEmpty(), "量測本身失效：V13 抓到 0 個碼值");
        assertFalse(resolved.isEmpty(), "量測本身失效：V12 抓到 0 個碼值");
        assertEquals(resolved, proposed,
                "提案成因值域與結案成因值域不相等 ⇒ 會出現「提得出來但結不了案」的成因，"
                        + "案件永久卡在 IN_PROGRESS");
    }

    private static Set<String> literalsIn(String flatDdl, String regex) {
        Matcher m = Pattern.compile(regex, Pattern.DOTALL).matcher(flatDdl);
        Set<String> out = new TreeSet<>();
        if (m.find()) {
            Matcher lit = Pattern.compile("'([A-Z_]+)'").matcher(m.group(1));
            while (lit.find()) {
                out.add(lit.group(1));
            }
        }
        return out;
    }

    /**
     * V13 的兩道 CHECK 必須存在，且 binding 是<b>雙向等式</b>。
     *
     * <p>🔴 單向（只寫 {@code resolution_type = … IMPLIES cause IS NOT NULL}）會讓反向靜默通過：
     * 一般處置夾帶一個沒有人會讀的責任成因 ⇒ 稽核看到「有責任主張」的理賠處置。
     */
    @Test
    @DisplayName("V13 兩道 CHECK 存在，且 binding 是雙向等式")
    void v13ProposedCauseConstraintsPinned() {
        assertTrue(FLAT_V13.contains("ADD COLUMN \"proposed_responsibility_cause\" varchar(48)"),
                "V13 沒有新增 proposed_responsibility_cause 欄，或型別／寬度與 resolution_code 不一致");
        assertTrue(FLAT_V13.contains("CONSTRAINT \"chk_fcr_proposed_cause_binding\""),
                "缺 chk_fcr_proposed_cause_binding ⇒ 責任提案可以不帶成因，缺陷原樣復活");
        assertTrue(FLAT_V13.contains("CONSTRAINT \"chk_fcr_proposed_cause_domain\""),
                "缺 chk_fcr_proposed_cause_domain ⇒ 任意字串都能當責任成因");
        assertTrue(FLAT_V13.replaceAll("\\s", "").contains(
                        "(\"resolution_type\"='RESPONSIBILITY_ATTRIBUTE')=(\"proposed_responsibility_cause\"ISNOTNULL)"),
                "chk_fcr_proposed_cause_binding 不是雙向等式——單向會讓其中一個方向靜默通過");
        assertTrue(FLAT_V13.contains("ALTER TABLE \"fulfillment_case_resolutions\""),
                "V13 動的不是 fulfillment_case_resolutions？");
    }

    /** 索引述詞與兩道 CHECK 內嵌的 category 字面值，必須等於 CodeMeta 的值。 */
    @Test
    @DisplayName("V12 內嵌的 category 字面值 = CodeMeta FulfillmentExceptionCategory")
    void v12CategoryLiteralMatchesCodeMeta() {
        String expected = FulfillmentExceptionCategory.RESPONSIBILITY_UNRESOLVED.value();
        long occurrences = Pattern.compile("'" + Pattern.quote(expected) + "'")
                .matcher(FLAT_V12).results().count();
        // 三處：索引述詞、no_dismiss、resolved_cause；第四處在 resolutions 的 type binding。
        assertTrue(occurrences >= 4,
                "V12 內嵌的 category 字面值出現 " + occurrences + " 次，少於預期的 4 處"
                        + "（索引述詞／no_dismiss／resolved_cause／type_binding）——"
                        + "有人改了其中一處而沒改其他處？");
        assertTrue(FulfillmentCaseResolutionType.RESPONSIBILITY_ATTRIBUTE.value()
                        .equals("RESPONSIBILITY_ATTRIBUTE"),
                "CodeMeta 的責任判定 resolution_type 值變了，V12 的 chk_fcr_type 與 type_binding 要一起改");
        assertTrue(FLAT_V12.contains("'RESPONSIBILITY_ATTRIBUTE'"),
                "V12 沒有內嵌 RESPONSIBILITY_ATTRIBUTE ⇒ chk_fcr_type 沒擴充，責任提案根本建不出來");
    }

    /**
     * V14 的三個 context 欄位與責任軸必填 CHECK 必須存在，且 {@code shipment_id} <b>不在必填內</b>。
     *
     * <p>🔴 {@code shipment_id} 若被寫進必填：責任判定案件在 shipment 建立之前
     * （建單失敗 {@code CREATE_FAILED(990)}）就建不出來，而那正是最需要判責任的情形之一。
     *
     * <p>🔴 述詞必須是「非責任 category 一律放行」而<b>不是</b>雙向等式：
     * generic exception case 將來也可能想記這三欄，反向封死會讓它們永遠加不進去。
     */
    @Test
    @DisplayName("V14 三個 context 欄位 ＋ 責任軸必填（shipment_id 可為 null、generic 不受影響）")
    void v14ImmutableContextPinned() {
        for (String col : List.of("order_no", "shipment_id", "shipping_method")) {
            assertTrue(FLAT_V14.contains("ADD COLUMN \"" + col + "\""),
                    "V14 沒有新增 " + col + " ⇒ N16／N17 的 context 仍然由呼叫端各傳一次");
        }
        assertTrue(FLAT_V14.contains("\"order_no\" varchar(32)"),
                "order_no 型別／寬度應跟齊 V9 的 logistics_receivables.order_no");
        assertTrue(FLAT_V14.contains("\"shipment_id\" bigint"),
                "shipment_id 應是 bigint（TSID 內部原生型別）");
        assertTrue(FLAT_V14.contains("CONSTRAINT \"chk_exc_responsibility_context\""),
                "缺 chk_exc_responsibility_context ⇒ 責任案件可以沒有 order_no，"
                        + "而它是 order-service 35→54 CAS 的錨");

        String flat = FLAT_V14.replaceAll("\\s", "");
        assertTrue(flat.contains("\"order_no\"ISNOTNULLAND\"shipping_method\"ISNOTNULL"),
                "責任軸必須同時要求 order_no 與 shipping_method 非空");
        // 🔴 反向對照：shipment_id 不得出現在必填述詞裡。
        int chk = FLAT_V14.indexOf("chk_exc_responsibility_context");
        assertTrue(chk >= 0, "量測本身失效：抓不到 chk_exc_responsibility_context");
        String predicate = FLAT_V14.substring(chk, Math.min(FLAT_V14.length(), chk + 260));
        assertFalse(predicate.contains("\"shipment_id\" IS NOT NULL"),
                "🔴 shipment_id 被寫進必填 ⇒ shipment 建立前的責任案件（建單失敗）建不出來");
        // 🔴 generic 不受影響：述詞必須以 category <> 開頭放行，而不是雙向等式。
        assertTrue(predicate.contains("<> 'RESPONSIBILITY_UNRESOLVED'"),
                "述詞應為「非責任 category 一律放行」，雙向等式會把 generic 案件封死");
    }

    /**
     * 🔴 <b>G4</b>：責任判定案件的建立<b>不得</b>沿用指名 conflict target 的那一支。
     *
     * <p>V12 之後責任案件同時受兩條唯一約束拘束。指名 {@code (dedup_key) WHERE …} 的 INSERT
     * 只認得 {@code uq_exc_open_dedup}，撞到 lifetime UNIQUE 時<b>不會</b>走 {@code DO NOTHING}
     * 而是拋 23505 ⇒ 交易 aborted（25P02）⇒ 整個建案交易 500。
     * 症狀不是「重複案件被安靜擋下」，而是 500——而那會被讀成「系統壞了」而不是「已經有案件了」。
     */
    @Test
    @DisplayName("責任案件的 INSERT 不指名 conflict target（G4）")
    void responsibilityInsertDoesNotNameConflictTarget() throws IOException {
        String mapper = readMapper("FulfillmentExceptionCaseMapper.java");
        int idx = mapper.indexOf("int insertResponsibilityCaseIfAbsent");
        assertTrue(idx > 0, "找不到 insertResponsibilityCaseIfAbsent —— 責任建案路徑被改名或移除了？");
        // 取該方法之前最近的一段 SQL（@Insert 到方法簽章之間）
        int stmtStart = mapper.lastIndexOf("@Insert", idx);
        assertTrue(stmtStart > 0, "找不到 insertResponsibilityCaseIfAbsent 的 @Insert");
        String stmt = mapper.substring(stmtStart, idx);

        assertTrue(stmt.contains("ON CONFLICT DO NOTHING"),
                "責任建案必須用**不指名 conflict target** 的 ON CONFLICT DO NOTHING（G4）");
        assertFalse(stmt.contains("ON CONFLICT (dedup_key)"),
                "🔴 責任建案沿用了指名 dedup_key 的 ON CONFLICT ⇒ 撞到 uq_exc_responsibility_subject 時"
                        + "會拋 23505 並讓整個交易 aborted，建案變成 500（G4）");

        // 對照組：一般建案那一支**仍然**指名 target（它只受一條唯一約束拘束，指名才推斷得到 partial index）
        assertTrue(mapper.contains("ON CONFLICT (dedup_key) WHERE status IN ('OPEN', 'IN_PROGRESS')"),
                "一般建案那一支的 ON CONFLICT 述詞不見了 —— 兩支的差異正是 G4 的內容，"
                        + "不得為了「統一」而把一般建案也改成不指名");
    }

    /**
     * 責任判定的覆核 CAS 必須帶 {@code resolution_type} 述詞（36 §3.3 Atomic Boundary ① 逐字）。
     *
     * <p>少了它，非責任型別的提案也會被這條路徑推成 {@code APPROVED}，而呼叫端接著會依
     * 「resolution 已 APPROVED」把案件結掉並發 N16 ⇒ 「唯一產生點」就不再唯一。
     */
    @Test
    @DisplayName("責任覆核 CAS 的述詞含 case_id ＋ resolution_type ＋ two-person")
    void responsibilityDecidePredicateIsComplete() throws IOException {
        String mapper = readMapper("FulfillmentCaseResolutionMapper.java");
        int idx = mapper.indexOf("int decideResponsibility");
        assertTrue(idx > 0, "找不到 decideResponsibility");
        int stmtStart = mapper.lastIndexOf("@Update", idx);
        String stmt = mapper.substring(stmtStart, idx);

        assertTrue(stmt.contains("AND status = #{fromStatus}"), "缺 allowed-from 述詞");
        assertTrue(stmt.contains("AND case_id = #{caseId}"),
                "缺 case_id 述詞 ⇒ 一個提案可以被拿去結另一張案件");
        assertTrue(stmt.contains("AND resolution_type = #{resolutionType}"),
                "🔴 缺 resolution_type 述詞（36 §3.3 Atomic Boundary ① 逐字要求）");
        assertTrue(stmt.contains("AND proposed_by_account_id <> #{reviewedByAccountId}"),
                "缺 two-person 述詞 ⇒ APPROVED 不再保證有兩個人，"
                        + "而「不向買家退款」是動買家的錢的決定");
    }

    private static String responsibilityIndexPredicate() {
        Matcher m = Pattern.compile(
                "CREATE UNIQUE INDEX \"uq_exc_responsibility_subject\"(.*?);", Pattern.DOTALL)
                .matcher(FLAT_V12);
        assertTrue(m.find(), "抓不到 uq_exc_responsibility_subject 的定義");
        return m.group(1);
    }

    /**
     * {@code ResponsibilityCaseRules.SOURCE_SERVICES} ↔ V9 的 {@code chk_exc_source} 值域逐字一致。
     *
     * <p>應用層的 fail-closed guard 會拿這個集合擋掉非法輸入 ⇒ 它比 DB 那道<b>先</b>執行。
     * 兩邊漂移的後果分兩種，都不會有人發現：
     * <ul>
     *   <li>應用層<b>少</b>一個值 ⇒ 合法的來源服務被擋掉（症狀是「某條路徑突然建不了案」）</li>
     *   <li>應用層<b>多</b>一個值 ⇒ 該值通過應用層、落到 DB 才被 23514 擋 ⇒ 交易 abort、呼叫端拿到 500</li>
     * </ul>
     *
     * <p>⚠️ 期望值從 <b>DDL 文字</b>抓、實際值從 <b>Java 常數</b>讀——兩個獨立來源的對拍。
     */
    @Test
    @DisplayName("ResponsibilityCaseRules.SOURCE_SERVICES = V9 的 chk_exc_source 值域")
    void sourceServiceDomainMatchesDdl() {
        Matcher m = Pattern.compile(
                "CONSTRAINT \"chk_exc_source\"\\s+CHECK \\(\"source_service\" IN \\(([^)]*)\\)")
                .matcher(FLAT);
        assertTrue(m.find(), "抓不到 chk_exc_source 的值域清單 ⇒ 約束被改名或移除了？");

        Set<String> inDdl = new TreeSet<>();
        Matcher lit = Pattern.compile("'([a-z_]+)'").matcher(m.group(1));
        while (lit.find()) {
            inDdl.add(lit.group(1));
        }
        // 對照組：抓到的清單非空，證明這個斷言不是因為 regex 落空而空轉
        assertFalse(inDdl.isEmpty(), "量測本身失效：從 DDL 抓到 0 個來源服務");

        assertEquals(inDdl, new TreeSet<>(ResponsibilityCaseRules.SOURCE_SERVICES),
                "應用層 guard 的值域與 chk_exc_source 不一致。"
                        + "🔴 不得為了讓測試變綠而單邊改——先決定哪一邊才對："
                        + "新增來源服務要同時改 V{N+1} 的 CHECK 與這個集合");
    }

    /**
     * 🔴 <b>{@code findByIdForUpdate} 的 SQL 必須含 {@code FOR UPDATE}——綁在<b>那一支 method 本身</b>。</b>
     *
     * <p><b>這一支為什麼非有不可</b>：`FOR UPDATE` 是 65 §5.5.2 E0.1 ⓪ 與 36 §3.3 Atomic Boundary ⓪
     * 的逐字要求，但 WP2A 實測發現——把它拿掉之後，責任判定的並發測試組<b>連跑 5 次全綠</b>
     * （每條競態都先被 resolution 的 CAS 與 {@code uq_fcr_open_per_case} 接住了）。
     * ⇒ <b>在本測試存在之前，沒有任何東西會在這條 lock 被刪掉時出聲。</b>
     *
     * <p>🔴 <b>用 reflection 讀該 method 的 {@code @Select}，不是搜整個檔案</b>：
     * 檔案層的 {@code contains("FOR UPDATE")} 會被<b>任何一支</b>帶鎖的 SQL 滿足
     * ——包括別的 method、甚至註解裡的字。那樣的檢查在 method 被改壞時仍然是綠的。
     *
     * <p>⚠️ 對「拿不到 annotation」必須 FAIL 而不是 SKIP：若有人把它改成 XML mapper 或
     * {@code @SelectProvider}，reflection 會讀不到 {@code @Select} ⇒ 那正是本測試該叫的時候。
     */
    @Test
    @DisplayName("findByIdForUpdate 的 @Select 含 FOR UPDATE（method-specific，不是檔案層）")
    void findByIdForUpdatePinsRowLockOnTheMethodItself() throws Exception {
        Method locking = FulfillmentExceptionCaseMapper.class.getMethod("findByIdForUpdate", Long.class);
        Select lockingSelect = locking.getAnnotation(Select.class);
        assertNotNull(lockingSelect,
                "findByIdForUpdate 沒有 @Select —— 它被改成 XML／@SelectProvider 了嗎？"
                        + "本測試拒絕在看不到 SQL 時通過");
        String lockingSql = String.join(" ", lockingSelect.value()).toUpperCase();
        assertTrue(lockingSql.contains("FOR UPDATE"),
                "🔴 findByIdForUpdate 的 SQL 不含 FOR UPDATE ⇒ 提案與定案不再序列化，"
                        + "而每一步單獨看都會成功、沒有任何錯誤訊息。實際 SQL：" + lockingSql);

        // 🔴 對照組：同一個 mapper 的兄弟 method 必須**不含** FOR UPDATE。
        //    少了它，本測試無法分辨「我真的綁在 findByIdForUpdate 上」與
        //    「我只是碰巧在這個 class 裡找到了 FOR UPDATE」。
        Method plain = FulfillmentExceptionCaseMapper.class.getMethod("findById", Long.class);
        Select plainSelect = plain.getAnnotation(Select.class);
        assertNotNull(plainSelect, "findById 沒有 @Select");
        String plainSql = String.join(" ", plainSelect.value()).toUpperCase();
        assertFalse(plainSql.contains("FOR UPDATE"),
                "findById（非鎖定版）竟然含 FOR UPDATE ⇒ 本測試的 method 綁定失效，"
                        + "或兩支 method 的職責混了。實際 SQL：" + plainSql);

        // 🔴 命名與行為一致：叫 ForUpdate 的就要鎖，不叫的就不要鎖。
        //    防的是「有人加了第三支 findXxxForUpdate 卻忘了寫 FOR UPDATE」。
        for (Method m : FulfillmentExceptionCaseMapper.class.getMethods()) {
            Select s = m.getAnnotation(Select.class);
            if (s == null) {
                continue;
            }
            boolean namedForUpdate = m.getName().endsWith("ForUpdate");
            boolean sqlLocks = String.join(" ", s.value()).toUpperCase().contains("FOR UPDATE");
            assertEquals(namedForUpdate, sqlLocks,
                    m.getName() + " 的命名與 SQL 是否加鎖不一致"
                            + "（叫 *ForUpdate 就必須鎖、不叫就不得鎖）");
        }
    }

    private static String readMapper(String fileName) throws IOException {
        Path p = Path.of("src/main/java/io/github/will413028/hawkr/payment/logistics/repository/mapper", fileName);
        if (!Files.isRegularFile(p)) {
            // 🔴 對「目標不存在」FAIL 而不是 SKIP。
            throw new IllegalStateException("找不到 " + p + " —— 檔案被改名／搬走了嗎？本測試拒絕在看不到目標時通過。");
        }
        return Files.readString(p, StandardCharsets.UTF_8);
    }
}
