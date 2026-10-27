package io.github.will413028.hawkr.shipping.migration;

import org.junit.jupiter.api.DisplayName;
import org.junit.jupiter.api.Test;

import java.io.IOException;
import java.io.InputStream;
import java.nio.charset.StandardCharsets;
import java.nio.file.Files;
import java.nio.file.Path;
import java.util.List;
import java.util.regex.Matcher;
import java.util.regex.Pattern;

import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.junit.jupiter.api.Assertions.assertFalse;
import static org.junit.jupiter.api.Assertions.assertTrue;
import static org.junit.jupiter.api.Assertions.fail;

/**
 * {@code V13__add_uber_direct_tables.sql} 的 schema 契約釘樁（66 號詳細設計書 §5）。
 *
 * <p><b>它證明什麼</b>：66 §5 逐字指定的表、欄、值域、UNIQUE 與 partial index 述詞
 * 確實出現在 migration 的 <b>DDL</b> 裡，且 66 明令<b>不要做</b>的幾件事沒有被做
 * （不建 {@code shipping_fee_tiers} 的 3003 列、不回填租約欄、3003 設定列的 {@code enabled} 是 {@code false}）。
 *
 * <p><b>為什麼這一支非釘不可</b>：本波是純資料層，<b>零程式碼消費者</b>——
 * 沒有任何 service／mapper 會因為欄位打錯、值域少一個、partial index 述詞對不上而編譯失敗或測試轉紅。
 * 在 W2 接上來之前，這支測試是這份 DDL 唯一的自動化守門。
 * 尤其 partial index：述詞與查詢不完全相符時 PostgreSQL <b>不會用它、而且不會報錯</b>——只會慢
 * （66 §5.2 紅字），所以它壞掉的時候沒有任何訊號。
 *
 * <p>🔴 <b>所有斷言都跑在「剝掉註解」的 DDL 上（{@link #CODE}）</b>：那份 migration 的註解裡
 * <b>刻意</b>提到了它不該做的事（「不建 uber_webhook_inbox」「client_secret 不落任何一張表」
 * 「容忍值 shopping_completed 不寫入」）。直接對全文比對的話，這些「否定斷言」會被自己的說明文字
 * 打成假紅；而「肯定斷言」則可能被一段描述性註解餵飽而假綠——兩個方向都會壞。
 *
 * <p><b>它不證明什麼（誠實標註）</b>：這是對 SQL <b>字面</b>的斷言，不是對 PostgreSQL 執行結果的斷言
 * ——shipping-service 沒有 DB 測試基礎設施（testcontainers 只在 cart／aftersales／coupon）。
 * 它擋得住「有人把某個欄位／約束刪掉或改掉」這個回歸；擋不住「這段 DDL 在真的 PostgreSQL 上跑不起來」。
 * 後者在本波以一次性 dry-run（V1→V13 對全新資料庫依序套用 ＋ INV-U6 的拒絕／接受對照）人工驗過，
 * 證據附在交付回報裡。
 *
 * <p>🔴 所有期望值一律<b>硬寫字面值</b>：期望值若來自被測檔案自己（或它引用的常數），
 * 這支測試就只是在自己跟自己對，必然全綠。
 */
@DisplayName("V13 Uber Direct schema 契約")
class UberDirectSchemaContractTest {

    /** 66 §5.1 逐字指定的八張表（硬寫字面值，不從 migration 反推）。 */
    private static final List<String> EXPECTED_TABLES = List.of(
            "uber_account",
            "uber_organization",
            "uber_store_mapping",
            "uber_quotes",
            "uber_deliveries",
            "uber_pod_artifacts",
            "packing_policy",
            "package_plan");

    private static final String MIGRATION_RESOURCE = "db/migration/V13__add_uber_direct_tables.sql";
    private static final String MIGRATION_PATH = "src/main/resources/" + MIGRATION_RESOURCE;

    /** 原始檔內容。 */
    private static final String RAW = readMigration();

    /** 🔴 剝掉 {@code --} 註解、再把空白壓成單一空格的「純 DDL」視圖。所有斷言都跑在它上面。 */
    private static final String CODE = stripLineComments(RAW).replaceAll("\\s+", " ");

    private static String readMigration() {
        Path p = Path.of(MIGRATION_PATH);
        if (Files.isRegularFile(p)) {
            try {
                return Files.readString(p, StandardCharsets.UTF_8);
            } catch (IOException e) {
                throw new IllegalStateException("讀不到 " + MIGRATION_PATH, e);
            }
        }
        // surefire 的工作目錄若不是 module basedir，退回 classpath（main/resources 在測試 classpath 上）。
        try (InputStream in = UberDirectSchemaContractTest.class.getClassLoader()
                .getResourceAsStream(MIGRATION_RESOURCE)) {
            if (in == null) {
                // 🔴 對「目標不存在」必須 FAIL 而不是 SKIP：檔案被改名或搬走時，
                //    SKIP 會讓這支測試靜默轉綠，而那正是它該叫的時候。
                throw new IllegalStateException(
                        "找不到 migration（檔案系統 " + MIGRATION_PATH + " 與 classpath " + MIGRATION_RESOURCE
                                + " 都沒有）。檔案被改名／搬走了嗎？本測試拒絕在看不到目標時通過。");
            }
            return new String(in.readAllBytes(), StandardCharsets.UTF_8);
        } catch (IOException e) {
            throw new IllegalStateException("讀不到 " + MIGRATION_RESOURCE, e);
        }
    }

    /**
     * 逐行剝掉 {@code --} 行註解，並以 in-string 狀態機避開字串字面值裡的 {@code --}。
     * <p>⚠️ 這支剝除器本身的鑑別力由 {@link #stripperKeepsDdlAndDropsComments()} 守——
     * 一支「把整份檔案都吃掉」的剝除器會讓每一條否定斷言都自動通過。
     */
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

    /** 取出某張表的 {@code CREATE TABLE …( … );} 內文（純 DDL）；找不到就 fail（不是回空字串）。 */
    private static String tableBody(String table) {
        Matcher m = Pattern.compile(
                "CREATE TABLE IF NOT EXISTS " + Pattern.quote(table) + " \\((.*?)\\); ").matcher(CODE);
        if (!m.find()) {
            fail("找不到 CREATE TABLE " + table + " —— 表被改名或刪掉了？");
        }
        return m.group(1);
    }

    /** 斷言某欄位以指定型別出現在該表內文（同時釘欄名與型別）。 */
    private static void assertColumn(String body, String table, String column, String type) {
        Pattern p = Pattern.compile("(?<![0-9a-zA-Z_])" + Pattern.quote(column) + " " + Pattern.quote(type)
                + "(?![0-9a-zA-Z_(])");
        assertTrue(p.matcher(body).find(),
                table + " 缺少欄位或型別不符：" + column + " " + type);
    }

    private static void assertDdlContains(String needle, String why) {
        assertTrue(CODE.contains(needle), why + "\nDDL 裡找不到片段：" + needle);
    }

    private static void assertDdlAbsent(String needle, String why) {
        assertFalse(CODE.contains(needle), why + "\nDDL 裡不該出現的片段：" + needle);
    }

    // ─────────────────────── 剝除器自身的鑑別力 ───────────────────────

    @Test
    @DisplayName("註解剝除器：留得住 DDL、吃得掉註解（它自己的鑑別力）")
    void stripperKeepsDdlAndDropsComments() {
        // 留得住：這三個片段只可能來自真正的 DDL。
        assertTrue(CODE.contains("CREATE TABLE IF NOT EXISTS uber_quotes ("), "剝除器把 DDL 也吃掉了");
        assertTrue(CODE.contains("ALTER TABLE shipments ADD COLUMN"), "剝除器把 ALTER TABLE 也吃掉了");
        assertTrue(CODE.contains("COMMENT ON TABLE uber_quotes IS"), "剝除器把 COMMENT ON 也吃掉了");
        // 吃得掉：這兩個片段只出現在 -- 註解裡（原始檔有、剝完後不該有）。
        assertTrue(RAW.contains("刻意不做的事"), "原始檔應含這段註解——它是本斷言的對照組");
        assertFalse(CODE.contains("刻意不做的事"), "剝除器沒有真的剝掉 -- 註解");
        assertTrue(RAW.contains("uber_webhook_inbox"), "原始檔的註解應提到 uber_webhook_inbox——對照組");
        assertFalse(CODE.contains("uber_webhook_inbox"),
                "uber_webhook_inbox 只出現在註解裡，剝完後不該還在（否則否定斷言全是假紅）");
    }

    // ─────────────────────────── 表存在性 ───────────────────────────

    @Test
    @DisplayName("66 §5.1 指定的八張表全部建立，且沒有多建")
    void createsExactlyTheEightTables() {
        for (String t : EXPECTED_TABLES) {
            assertDdlContains("CREATE TABLE IF NOT EXISTS " + t + " (", "66 §5.1 要求建立 " + t);
        }
        Matcher m = Pattern.compile("CREATE TABLE IF NOT EXISTS ").matcher(CODE);
        int count = 0;
        while (m.find()) {
            count++;
        }
        assertEquals(8, count, "建立的表數量與 66 §5.1 不符（多建或少建）");
    }

    @Test
    @DisplayName("🔴 不建 provider 專屬 webhook inbox —— 共用既有 webhook_inbox（66 §5.1.6）")
    void doesNotCreateUberWebhookInbox() {
        assertDdlAbsent("uber_webhook_inbox",
                "66 §5.1.6 明令共用 payuni-webhook-service 既有的 webhook_inbox"
                        + "（它的 uq_inbox_event UNIQUE 本來就有 provider 維度），不另建 provider 專屬 inbox");
    }

    @Test
    @DisplayName("🔴 client_secret 不落任何一張表（66 §5.1.1／§8.5）")
    void neverPersistsClientSecret() {
        // ⚠️ 這裡只掃 CREATE TABLE 的欄位定義，不掃整份 DDL——COMMENT ON 的字串裡
        //    刻意寫著「client_secret 不在本表」，掃全文會被自己的說明文字打成假紅。
        //    要守的不變式是「沒有一個叫 client_secret 的欄位」，不是「這個字串不准出現」。
        for (String table : EXPECTED_TABLES) {
            assertFalse(tableBody(table).contains("client_secret"),
                    table + " 不得有 client_secret 欄位——它只能放 k8s secret "
                            + "uber-direct-credentials 的 client-secret（66 §8.5）");
        }
    }

    // ─────────────────────────── 逐表欄位 ───────────────────────────

    @Test
    @DisplayName("uber_account 欄位（66 §5.1.1）")
    void uberAccountColumns() {
        String b = tableBody("uber_account");
        assertColumn(b, "uber_account", "id", "BIGINT PRIMARY KEY");
        assertColumn(b, "uber_account", "env", "VARCHAR(20)");
        assertColumn(b, "uber_account", "client_id", "VARCHAR(128)");
        assertColumn(b, "uber_account", "organization_id", "VARCHAR(128)");
        assertColumn(b, "uber_account", "enabled", "BOOLEAN");
        // 停用走 enabled=false，不刪列 ⇒ 預設必須是關的（開啟是一次明確的動作）。
        assertTrue(b.contains("enabled BOOLEAN NOT NULL DEFAULT false"),
                "uber_account.enabled 預設必須是 false");
    }

    @Test
    @DisplayName("uber_organization 欄位與 status 值域（66 §5.1.2）")
    void uberOrganizationColumnsAndStatusDomain() {
        String b = tableBody("uber_organization");
        assertColumn(b, "uber_organization", "id", "BIGINT PRIMARY KEY");
        assertColumn(b, "uber_organization", "uber_account_id", "BIGINT");
        // 🔴 賣家業務鍵是 store_id，不是 account_id（身分分離，設計書 44）
        assertColumn(b, "uber_organization", "store_id", "BIGINT");
        assertFalse(b.contains("seller_account_id"),
                "66 §5.1.2 逐字要求 store_id（賣家業務鍵），不得用 account_id");
        assertColumn(b, "uber_organization", "organization_id", "VARCHAR(128)");
        assertColumn(b, "uber_organization", "status", "VARCHAR(20)");
        assertTrue(b.contains("CHECK (status IN ('PENDING', 'ACTIVE', 'SUSPENDED'))"),
                "uber_organization.status 值域必須逐字是 PENDING｜ACTIVE｜SUSPENDED");
    }

    @Test
    @DisplayName("uber_store_mapping 欄位、預設值、UNIQUE 與 INV-U6（66 §5.1.3／§5.5）")
    void uberStoreMappingContract() {
        String b = tableBody("uber_store_mapping");
        assertColumn(b, "uber_store_mapping", "id", "BIGINT PRIMARY KEY");
        assertColumn(b, "uber_store_mapping", "seller_store_id", "BIGINT");
        assertColumn(b, "uber_store_mapping", "external_store_id", "VARCHAR(64)");
        assertColumn(b, "uber_store_mapping", "customer_id", "VARCHAR(64)");
        assertColumn(b, "uber_store_mapping", "pickup_snapshot", "JSONB");
        assertColumn(b, "uber_store_mapping", "supported_temps", "JSONB");
        assertColumn(b, "uber_store_mapping", "max_packages", "INTEGER");
        assertColumn(b, "uber_store_mapping", "prep_sla_minutes", "INTEGER");
        assertColumn(b, "uber_store_mapping", "active", "BOOLEAN");
        assertColumn(b, "uber_store_mapping", "blocked", "BOOLEAN");
        assertColumn(b, "uber_store_mapping", "tax_form_required", "BOOLEAN");
        assertColumn(b, "uber_store_mapping", "health_status", "VARCHAR(20)");

        // 66 §5.1.3 逐字的預設值
        assertTrue(b.contains("supported_temps JSONB NOT NULL DEFAULT '[\"3201\"]'"),
                "supported_temps 預設必須只有常溫 [\"3201\"]（Uber API 完全沒有溫層欄位，檢查 100% 在我方）");
        assertTrue(b.contains("max_packages INTEGER NOT NULL DEFAULT 30"), "max_packages 預設 30（M-4）");
        assertTrue(b.contains("prep_sla_minutes INTEGER NOT NULL DEFAULT 15"), "prep_sla_minutes 預設 15");
        assertTrue(b.contains("active BOOLEAN NOT NULL DEFAULT false"), "active 預設 false");

        assertTrue(b.contains("UNIQUE (seller_store_id, external_store_id)"),
                "66 §5.1.3 逐字：UNIQUE (seller_store_id, external_store_id)");
        assertTrue(b.contains("CHECK (health_status IN ('HEALTHY', 'DEGRADED', 'BLOCKED', 'UNKNOWN'))"),
                "health_status 值域必須逐字是 HEALTHY｜DEGRADED｜BLOCKED｜UNKNOWN");
        // INV-U6：active=true ⇒ blocked=false
        assertTrue(b.contains("CHECK ((NOT active) OR (NOT blocked))"),
                "INV-U6 必須由 DB CHECK 守：違反的症狀是對已封鎖門市持續建單、每筆 403");
    }

    @Test
    @DisplayName("uber_quotes 欄位、金額型別、狀態值域與 UNIQUE（66 §5.1.4）")
    void uberQuotesContract() {
        String b = tableBody("uber_quotes");
        assertColumn(b, "uber_quotes", "id", "BIGINT PRIMARY KEY");
        assertColumn(b, "uber_quotes", "quote_id", "VARCHAR(64)");
        assertColumn(b, "uber_quotes", "order_no", "VARCHAR(64)");
        assertColumn(b, "uber_quotes", "buyer_id", "BIGINT");
        assertColumn(b, "uber_quotes", "seller_store_id", "BIGINT");
        assertColumn(b, "uber_quotes", "external_store_id", "VARCHAR(64)");
        // 🔴 金額一律 BIGINT（新台幣元整數）
        assertColumn(b, "uber_quotes", "fee_amount", "BIGINT");
        assertColumn(b, "uber_quotes", "currency", "VARCHAR(8)");
        assertColumn(b, "uber_quotes", "expires_at", "TIMESTAMPTZ");
        assertColumn(b, "uber_quotes", "pickup_json", "JSONB");
        assertColumn(b, "uber_quotes", "dropoff_json", "JSONB");
        assertColumn(b, "uber_quotes", "items_json", "JSONB");
        // 🔴 payload_hash：66 A9 的核心。少了它，「換了取件門市但 hash 相同 ⇒ 用舊報價建新門市的單」
        //    就完全無從偵測，而且是靜默的（運費對得起來、門市是錯的）。
        assertColumn(b, "uber_quotes", "payload_hash", "CHAR(64)");
        assertColumn(b, "uber_quotes", "packing_policy_version", "INTEGER");
        assertColumn(b, "uber_quotes", "status", "VARCHAR(24)");

        assertTrue(b.contains("UNIQUE (quote_id)"), "quote_id 必須 UNIQUE（66 §5.1.4）");
        assertTrue(b.contains(
                        "CHECK (status IN ('CREATED', 'CHECKOUT_ACCEPTED', 'EXPIRED', 'CONSUMED', 'SUPERSEDED'))"),
                "uber_quotes.status 值域必須逐字是 CREATED｜CHECKOUT_ACCEPTED｜EXPIRED｜CONSUMED｜SUPERSEDED");
        assertTrue(b.contains("CHECK (fee_amount >= 0)"), "運費不得為負");
    }

    @Test
    @DisplayName("uber_deliveries 欄位、三個 UNIQUE 與兩個值域（66 §5.1.5／§5.5／C.2 ④）")
    void uberDeliveriesContract() {
        String b = tableBody("uber_deliveries");
        assertColumn(b, "uber_deliveries", "id", "BIGINT PRIMARY KEY");
        assertColumn(b, "uber_deliveries", "delivery_id", "VARCHAR(128)");
        assertColumn(b, "uber_deliveries", "shipment_id", "BIGINT");
        assertColumn(b, "uber_deliveries", "quote_id", "VARCHAR(64)");
        // 🔴 leg 的真相在 is_return，不在 id 前綴（前綴是 provider 的實作細節）
        assertColumn(b, "uber_deliveries", "is_return", "BOOLEAN");
        assertColumn(b, "uber_deliveries", "related_deliveries", "JSONB");
        assertColumn(b, "uber_deliveries", "provider_status", "VARCHAR(32)");
        // 🔴 courier_imminent 是 boolean 欄位，不是第 8 個狀態
        assertColumn(b, "uber_deliveries", "courier_imminent", "BOOLEAN");
        assertColumn(b, "uber_deliveries", "courier_name", "VARCHAR(128)");
        assertColumn(b, "uber_deliveries", "courier_phone_masked", "VARCHAR(32)");
        assertColumn(b, "uber_deliveries", "courier_location", "JSONB");
        assertColumn(b, "uber_deliveries", "eta_dropoff_at", "TIMESTAMPTZ");
        assertColumn(b, "uber_deliveries", "undeliverable_reason", "VARCHAR(64)");
        assertColumn(b, "uber_deliveries", "cancelation_reason", "VARCHAR(64)");
        assertColumn(b, "uber_deliveries", "outcome", "VARCHAR(32)");

        // 🔴 未遮罩的外送員電話不得落庫（66 §5.1.5／§8.6）：Uber 給的是可撥打的臨時號碼。
        assertFalse(Pattern.compile("(?<![0-9a-zA-Z_])courier_phone(?!_masked)").matcher(b).find(),
                "只准存遮罩後的 courier_phone_masked，原值不落庫也不落 log");

        assertTrue(b.contains("UNIQUE (quote_id)"), "INV-U2：quote_id UNIQUE（違反＝兩張單、重複計費）");
        assertTrue(b.contains("UNIQUE (shipment_id)"), "INV-U3：shipment_id UNIQUE（違反＝狀態寫不回去）");
        assertTrue(b.contains("UNIQUE (delivery_id)"), "66 §5.3：delivery_id UNIQUE 是建單重試的最後一道");

        // Uber 原生 7 值（U-I）。
        assertTrue(b.contains("provider_status IN ('pending', 'pickup', 'pickup_complete', 'dropoff',"
                        + " 'delivered', 'canceled', 'returned')"),
                "provider_status 值域必須逐字是 Uber 原生 7 值（66 §7.3.1）");
        // 容忍值：counter ＋ 不寫入狀態 ⇒ 它不該進值域，否則等於默許把它寫成一個狀態。
        assertFalse(b.contains("shopping_completed"),
                "shopping_completed 是容忍值：counter ＋ 不寫入狀態（66 §7.3.1），不得進值域");

        // UberDeliveryOutcome 五值（66 C.2 ④）——「41 這一格的錢與責任歸誰」的唯一判準
        assertTrue(b.contains("outcome IN ('PROVIDER_UNDELIVERABLE', 'PROVIDER_CANCELED', 'SELLER_CANCELED',"
                        + " 'ADMIN_CANCELED', 'CREATE_FAILED')"),
                "outcome 值域必須逐字是 UberDeliveryOutcome 五值");
    }

    @Test
    @DisplayName("uber_pod_artifacts 欄位、UNIQUE 與 artifact_type 值域（66 §5.1.7）")
    void uberPodArtifactsContract() {
        String b = tableBody("uber_pod_artifacts");
        assertColumn(b, "uber_pod_artifacts", "id", "BIGINT PRIMARY KEY");
        assertColumn(b, "uber_pod_artifacts", "delivery_id", "VARCHAR(128)");
        assertColumn(b, "uber_pod_artifacts", "artifact_type", "VARCHAR(20)");
        assertColumn(b, "uber_pod_artifacts", "asset_id", "BIGINT");
        // 🔴 欄名叫 _key、存的是 key（不是 URL）——多媒體規範的命名誠實要求
        assertColumn(b, "uber_pod_artifacts", "object_key", "VARCHAR(512)");
        assertFalse(b.contains("object_url"), "存 key 就叫 _key，禁止 *_url 名存 key 值");
        assertColumn(b, "uber_pod_artifacts", "pincode_value", "VARCHAR(32)");
        assertColumn(b, "uber_pod_artifacts", "fetched_at", "TIMESTAMPTZ");
        assertColumn(b, "uber_pod_artifacts", "fetch_attempts", "INTEGER");
        // 🔴 租約欄不是「已抓過」旗標
        assertColumn(b, "uber_pod_artifacts", "fetch_lease_until", "TIMESTAMPTZ");

        assertTrue(b.contains("UNIQUE (delivery_id, artifact_type)"),
                "66 §5.1.7／§7.9：UNIQUE (delivery_id, artifact_type) 擋 POD 重複上傳");
        assertTrue(b.contains("CHECK (artifact_type IN ('picture', 'signature', 'pincode'))"),
                "artifact_type 值域必須逐字是 Uber 原生三值 picture｜signature｜pincode");
    }

    @Test
    @DisplayName("packing_policy／package_plan 欄位與預設值（66 §5.1.8）")
    void packingTablesContract() {
        String policy = tableBody("packing_policy");
        assertColumn(policy, "packing_policy", "id", "BIGINT PRIMARY KEY");
        assertColumn(policy, "packing_policy", "version", "INTEGER");
        // 🔴 void_factor 是「係數不是金額」——本 migration 唯一允許的非整數型別
        assertColumn(policy, "packing_policy", "void_factor", "NUMERIC(4,2)");
        assertColumn(policy, "packing_policy", "packaging_weight_gram", "INTEGER");
        assertColumn(policy, "packing_policy", "max_items_per_package", "INTEGER");
        assertColumn(policy, "packing_policy", "effective_from", "TIMESTAMPTZ");
        assertTrue(policy.contains("void_factor NUMERIC(4,2) NOT NULL DEFAULT 1.25"), "void_factor 預設 1.25");
        assertTrue(policy.contains("packaging_weight_gram INTEGER NOT NULL DEFAULT 150"),
                "packaging_weight_gram 預設 150");
        assertTrue(policy.contains("max_items_per_package INTEGER NOT NULL DEFAULT 12"),
                "max_items_per_package 預設 12");
        assertTrue(policy.contains("UNIQUE (version)"),
                "version 是 quote 快照指向的業務鍵，必須 UNIQUE");
        // fail-closed：係數 <= 0 會讓「下界估計 × 係數」恆為 0 ⇒ 體積檢查恆過，而且完全無聲
        assertTrue(policy.contains("CHECK (void_factor > 0)"), "void_factor 必須 > 0（否則體積檢查恆過）");

        String plan = tableBody("package_plan");
        assertColumn(plan, "package_plan", "id", "BIGINT PRIMARY KEY");
        assertColumn(plan, "package_plan", "quote_id", "VARCHAR(64)");
        assertColumn(plan, "package_plan", "seq", "INTEGER");
        assertColumn(plan, "package_plan", "length_cm", "INTEGER");
        assertColumn(plan, "package_plan", "width_cm", "INTEGER");
        assertColumn(plan, "package_plan", "height_cm", "INTEGER");
        assertColumn(plan, "package_plan", "weight_gram", "INTEGER");
        assertColumn(plan, "package_plan", "size_class", "VARCHAR(20)");
        assertTrue(plan.contains("UNIQUE (quote_id, seq)"), "同一 quote 的箱號不得重複");
        assertTrue(plan.contains("CHECK (length_cm > 0 AND width_cm > 0 AND height_cm > 0)"),
                "任一邊為 0 會讓體積恆為 0 ⇒ 承載檢查恆過，而且完全無聲");
    }

    @Test
    @DisplayName("packing_policy 有一筆初始版本，否則 quote 的 packing_policy_version 沒有可指向的版本")
    void seedsInitialPackingPolicyVersion() {
        assertDdlContains("INSERT INTO packing_policy", "66 §5.1.8 的三個預設值需要一筆初始版本落地");
        Matcher m = Pattern.compile("INSERT INTO packing_policy \\((.*?)\\) VALUES \\((.*?)\\)").matcher(CODE);
        assertTrue(m.find(), "找不到 packing_policy 的初始版本 INSERT");
        assertTrue(m.group(2).contains("1.25"), "初始 void_factor 必須是 66 §5.1.8 的 1.25");
        assertTrue(m.group(2).contains("150"), "初始 packaging_weight_gram 必須是 66 §5.1.8 的 150");
        assertTrue(m.group(2).contains("12"), "初始 max_items_per_package 必須是 66 §5.1.8 的 12");
    }

    // ─────────────────────────── 金額型別紅線 ───────────────────────────

    @Test
    @DisplayName("🔴 金額欄一律 BIGINT；NUMERIC 只准出現在 void_factor（係數不是金額）")
    void moneyIsAlwaysBigint() {
        Matcher m = Pattern.compile("(?<![0-9a-zA-Z_])([a-z_]+) (NUMERIC|DECIMAL|REAL|DOUBLE PRECISION|FLOAT)")
                .matcher(CODE);
        while (m.find()) {
            String column = m.group(1);
            assertEquals("void_factor", column,
                    "金額／數量一律 BIGINT（新台幣元整數）。唯一允許的非整數欄是 packing_policy.void_factor"
                            + "（係數不是金額），但這裡出現的是：" + column + " " + m.group(2));
        }
    }

    // ─────────────────────────── partial index 述詞 ───────────────────────────

    @Test
    @DisplayName("🔴 三條 partial index 的 WHERE 述詞與 66 §5.2 逐字相符")
    void partialIndexPredicatesMatchDesignVerbatim() {
        // ⚠️ 述詞若與 mapper 的查詢不完全相符，PostgreSQL 不會用這個索引，而且不會報錯——只會慢。
        assertDdlContains(
                "CREATE INDEX IF NOT EXISTS idx_uber_quotes_expiry ON uber_quotes (expires_at) "
                        + "WHERE status = 'CREATED';",
                "66 §5.2：quote TTL 掃描的 partial index");
        assertDdlContains(
                "CREATE INDEX IF NOT EXISTS idx_pod_fetch_lease ON uber_pod_artifacts (fetch_lease_until) "
                        + "WHERE fetched_at IS NULL;",
                "66 §5.2：POD 抓取候選的 partial index");
        assertDdlContains(
                "CREATE INDEX IF NOT EXISTS idx_shipments_uber_reconcile_lease ON shipments "
                        + "(uber_status_reconcile_next_at) "
                        + "WHERE shipping_method = '3003' AND status NOT IN ('11', '41');",
                "66 §5.2：未終態回查候選的 partial index");
    }

    // ─────────────────────────── 既有表擴充 ───────────────────────────

    @Test
    @DisplayName("shipments 新增租約欄，且 🔴 不回填（66 §5.4）")
    void shipmentsLeaseColumnAddedAndNotBackfilled() {
        assertDdlContains(
                "ALTER TABLE shipments ADD COLUMN IF NOT EXISTS uber_status_reconcile_next_at TIMESTAMPTZ;",
                "66 §5.1.10：shipments 新增回查租約欄");
        // 租約欄 NULL 的正確語意就是「從未被搶佔、立即可搶」；回填成 NOW() 會讓既有列沉默一個租約週期。
        assertDdlAbsent("UPDATE shipments SET uber_status_reconcile_next_at",
                "🔴 66 §5.4 逐字：租約欄不得回填");
        // DEFAULT NOW() 等同於對每一列回填。
        assertDdlAbsent("uber_status_reconcile_next_at TIMESTAMPTZ DEFAULT",
                "🔴 租約欄不得有 DEFAULT（等同回填）");
    }

    @Test
    @DisplayName("shipping_method_configs 新增 max_volume_cm3，且 3003／3201 列以 enabled = false 建立")
    void shippingMethodConfigsExtendedAndRowDisabled() {
        assertDdlContains(
                "ALTER TABLE shipping_method_configs ADD COLUMN IF NOT EXISTS max_volume_cm3 "
                        + "BIGINT NOT NULL DEFAULT 0;",
                "66 §5.1.10：max_volume_cm3（0 表不限，沿用同表既有慣例）");

        Matcher m = Pattern.compile("INSERT INTO shipping_method_configs \\((.*?)\\) VALUES \\((.*?)\\)")
                .matcher(CODE);
        assertTrue(m.find(), "66 §5.1.10 要求建立 3003／3201 設定列");
        String columns = m.group(1);
        String values = m.group(2);

        // 🔴 enabled = false 是本列存在的全部理由（66 §3.4 ①／陷阱 A1′）：
        //    建列是「要求」，開啟才是那一步不可逆的切換（§12.2 步 9）。
        assertTrue(columns.contains("enabled"), "INSERT 必須顯式指定 enabled（該欄 DB 預設是 true）");
        assertTrue(values.contains("false"),
                "🔴 3003 列必須以 enabled = false 建立。切成 true 而程式還在 Phase 1 ＝ 陷阱 A1："
                        + "買家付錢、建出沒人送的單、零告警");
        assertFalse(values.contains("true"), "3003 列的 INSERT 不得有任何 true");

        // 值來自 66 §0.1 的 M-1／M-2／M-3
        assertTrue(values.contains("'3003', '3201'"), "配送方式 3003（Uber）／溫層 3201（常溫）");
        assertTrue(values.contains("20000"), "M-1：我方重量上限 20,000 g");
        assertTrue(values.contains("125000"), "M-2：我方體積上限 125,000 cm³");
        assertTrue(values.contains("60"), "M-3：我方最長邊 60 cm");
    }

    @Test
    @DisplayName("🔴 不建 shipping_fee_tiers 的 3003 列（66 §5.1.10 明令）")
    void neverSeedsUberFeeTier() {
        assertDdlAbsent("INSERT INTO shipping_fee_tiers",
                "🔴 Uber 運費是 quote 即時價，不走級距查表（66 §5.1.10）。"
                        + "建了這一列等於讓 CheckoutValidator 拿查表價當 Uber 運費。");
    }

    @Test
    @DisplayName("🔴 expand-only：只擴充 66 §5.1.10 授權的兩張既有表，且不 DROP 任何東西")
    void expandOnlyAndOnlyAuthorisedTables() {
        Matcher m = Pattern.compile("ALTER TABLE ([a-z_]+) ").matcher(CODE);
        while (m.find()) {
            String table = m.group(1);
            assertTrue(List.of("shipments", "shipping_method_configs").contains(table),
                    "66 §5.1.10 只授權擴充 shipments 與 shipping_method_configs，但本檔動了：" + table);
        }
        assertDdlAbsent("DROP TABLE", "本波是 expand-only（66 §5.4），不得 DROP TABLE");
        assertDdlAbsent("DROP COLUMN", "本波是 expand-only（66 §5.4），不得 DROP COLUMN");
    }
}
