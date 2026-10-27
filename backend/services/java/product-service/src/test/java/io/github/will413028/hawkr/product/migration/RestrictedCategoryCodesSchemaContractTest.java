package io.github.will413028.hawkr.product.migration;

import com.baomidou.mybatisplus.annotation.TableField;
import io.github.will413028.hawkr.product.config.typehandler.RestrictedCategoryCodeListTypeHandler;
import io.github.will413028.hawkr.product.entity.ProductSkuEntity;
import org.apache.ibatis.type.MappedTypes;
import org.junit.jupiter.api.DisplayName;
import org.junit.jupiter.api.Test;

import java.io.IOException;
import java.io.InputStream;
import java.lang.reflect.Field;
import java.nio.charset.StandardCharsets;
import java.nio.file.Files;
import java.nio.file.Path;
import java.util.List;

import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.junit.jupiter.api.Assertions.assertFalse;
import static org.junit.jupiter.api.Assertions.assertNotNull;
import static org.junit.jupiter.api.Assertions.assertTrue;

/**
 * {@code V19__add_product_sku_restricted_category_codes.sql} 的 schema 契約釘樁
 * （66 號詳細設計書 §5.1.9 ②：禁運品／品類守門的 SKU 標記欄）。
 *
 * <p><b>它證明什麼</b>：那一欄、它的 {@code NOT NULL DEFAULT '[]'}、以及把「值域尚未定案」
 * 翻譯成 fail-closed 的那條 CHECK，確實出現在 migration 的 <b>DDL</b> 裡；
 * 而且 entity 側真的接上了對應的 TypeHandler。
 *
 * <p><b>為什麼這一支非釘不可</b>：本波這一欄<b>零程式碼消費者</b>
 * （寫入者是 66 §5.1.9 ③ 的賣家上架 UI、讀取者是 §7.4 道 6 的結帳檢查，都在後續波次）
 * ⇒ 沒有任何 service／mapper 會因為欄位打錯、CHECK 被拿掉而編譯失敗或測試轉紅。
 * 在那兩件接上來之前，這支測試是這份 DDL 唯一的自動化守門。
 *
 * <p>🔴 <b>斷言跑在「剝掉 {@code --} 註解」的 DDL 上（{@link #CODE}）</b>：
 * 那份 migration 的註解<b>刻意</b>寫出了它現在不該做的事
 * （UP-4 定案後才該出現的 {@code jsonb_array_elements_text} 逐值列舉範例）。
 * 直接對全文比對的話，否定斷言會被自己的說明文字打成假紅，
 * 肯定斷言則可能被一段描述性註解餵飽而假綠——兩個方向都會壞。
 * {@link #註解剝除器本身有效()} 就是那個剝除器的對照組。
 *
 * <p><b>它不證明什麼（誠實標註）</b>：這是對 SQL <b>字面</b>的斷言，不是對 PostgreSQL
 * 執行結果的斷言——product-service 沒有 DB 測試基礎設施（testcontainers 只在
 * cart／aftersales／coupon）。它擋得住「有人把欄位或 CHECK 刪掉或改掉」這個回歸；
 * 擋不住「這段 DDL 在真的 PostgreSQL 上跑不起來」。
 *
 * <p>🔴 所有期望值一律<b>硬寫字面值</b>：期望值若來自被測檔案自己（或它引用的常數），
 * 這支測試就只是在自己跟自己對，必然全綠。
 */
@DisplayName("V19 product_sku.restricted_category_codes schema 契約")
class RestrictedCategoryCodesSchemaContractTest {

    private static final String MIGRATION_RESOURCE =
            "db/migration/V19__add_product_sku_restricted_category_codes.sql";
    private static final String MIGRATION_PATH = "src/main/resources/" + MIGRATION_RESOURCE;

    /** 原始檔內容。 */
    private static final String RAW = readMigration();

    /** 🔴 剝掉 {@code --} 註解、再把空白壓成單一空格的「純 DDL」視圖。所有 DDL 斷言都跑在它上面。 */
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
        try (InputStream in = RestrictedCategoryCodesSchemaContractTest.class.getClassLoader()
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
            throw new IllegalStateException("讀不到 classpath:" + MIGRATION_RESOURCE, e);
        }
    }

    /** 逐行剝掉 {@code --} 之後的內容；字串字面值內的 {@code --} 不算註解。 */
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

    @Test
    @DisplayName("剝除器本身有效：只出現在註解裡的字串不得進入 CODE")
    void 註解剝除器本身有效() {
        // jsonb_array_elements_text 只出現在「UP-4 定案後要怎麼改」那段 -- 註解裡。
        // 它若出現在 CODE 裡，代表剝除器沒生效 ⇒ 底下所有否定斷言都會失去鑑別力。
        assertTrue(RAW.contains("jsonb_array_elements_text"),
                "對照組失效：原始檔裡找不到那段註解，本測試無法判定剝除器是否有效——"
                        + "請改用另一個只出現在註解裡的字串當對照組。");
        assertFalse(CODE.contains("jsonb_array_elements_text"),
                "剝除器沒生效：註解內容進了 CODE。此時所有否定斷言都是假綠。");
    }

    @Test
    @DisplayName("欄位存在且型別／預設值正確（66 §5.1.9 ② 逐字）")
    void 欄位存在() {
        assertTrue(CODE.contains("ALTER TABLE product_sku ADD COLUMN restricted_category_codes JSONB NOT NULL DEFAULT '[]'::jsonb;"),
                "缺少 product_sku.restricted_category_codes 欄位，或型別／NOT NULL／DEFAULT 被改掉。\n"
                        + "🔴 66 §5.1.9：只做碼表與結帳檢查、不做這個欄位，是最糟的組合——"
                        + "檢查會跑但每一筆都是空陣列 ⇒ 恆為通過，而且看起來完全正常。\n實際 DDL：" + CODE);
    }

    @Test
    @DisplayName("值域未定案 ⇒ CHECK 把它鎖成 fail-closed（沒有碼表 = 寫不進去）")
    void 值域未定案時只允許空陣列() {
        assertTrue(CODE.contains("ADD CONSTRAINT chk_product_sku_restricted_category_codes CHECK ("),
                "CHECK 約束不見了。沒有它，『沒有碼表』就等於『什麼都寫得進去』——"
                        + "等 UP-4 定案時 DB 裡已經躺著一批沒有人認得的值。");
        assertTrue(CODE.contains("jsonb_typeof(restricted_category_codes) = 'array'"),
                "CHECK 少了型別不變式（它必須是陣列，不是物件也不是純量）。");
        assertTrue(CODE.contains("jsonb_array_length(restricted_category_codes) = 0"),
                "CHECK 少了值域不變式。RestrictedGoodsCategory 目前是空集合 ⇒ 唯一合法內容是空陣列；"
                        + "放寬它之前必須先建碼表（66 附錄 C.2 ⑤／UP-4）。");
    }

    @Test
    @DisplayName("不回填既有列（66 §5.4 逐字：空陣列即為正確初值）")
    void 不得有回填UPDATE() {
        assertFalse(CODE.toUpperCase().contains("UPDATE PRODUCT_SKU"),
                "出現了對既有列的回填 UPDATE。66 §5.4 逐條判定本欄不需要回填："
                        + "既有 SKU 從未被標記過禁運品類，空陣列即為正確初值。");
    }

    @Test
    @DisplayName("欄位有 COMMENT，把『值域待 UP-4』留在 DB 裡而不只留在 repo 裡")
    void 欄位有註解() {
        assertTrue(CODE.contains("COMMENT ON COLUMN product_sku.restricted_category_codes IS"),
                "缺少 COMMENT ON COLUMN。直接看 DB 的人（維運 SQL、psql \\d+）拿不到"
                        + "『這一欄現在只收空陣列、值域待 UP-4』這個關鍵前提。");
        assertTrue(CODE.contains("UP-4"),
                "COMMENT 裡沒有提到 UP-4——放寬 CHECK 的前置條件必須寫在 DB 自己身上。");
    }

    @Test
    @DisplayName("entity 側真的接上了：欄位存在且指名對應的 TypeHandler")
    void entity已接線() throws NoSuchFieldException {
        // 🔴 這一格擋的是「migration 加了欄位但沒人把 entity 接上」——
        //    那種情況 DDL 測試全綠、編譯全綠，而讀寫路徑永遠看不到這一欄。
        Field f = ProductSkuEntity.class.getDeclaredField("restrictedCategoryCodes");
        assertEquals(List.class, f.getType(), "欄位型別應為 List<String>（JSONB 字串陣列）。");

        TableField tf = f.getAnnotation(TableField.class);
        assertNotNull(tf, "缺少 @TableField ⇒ MyBatis-Plus 不會套用 TypeHandler，JSONB 會以字串原樣塞進來。");
        assertEquals(RestrictedCategoryCodeListTypeHandler.class, tf.typeHandler(),
                "TypeHandler 指錯了。JSONB 欄位少了對應 handler 時，寫入會變成帶引號的字串字面值。");
    }

    @Test
    @DisplayName("TypeHandler 不得標 @MappedTypes(List.class)（Java 微服務開發規範 §9.1）")
    void typeHandler未污染全域註冊表() {
        assertFalse(RestrictedCategoryCodeListTypeHandler.class.isAnnotationPresent(MappedTypes.class),
                "本 handler 由 @TableField(typeHandler = ...) 顯式指名使用，不需要也不得宣告 "
                        + "@MappedTypes（規範 §9.1 明令禁止 @MappedTypes(List.class)）——"
                        + "本服務已有兩個 List 的 JSONB handler，再宣告一次只是再覆寫一次同一格。");
    }
}
