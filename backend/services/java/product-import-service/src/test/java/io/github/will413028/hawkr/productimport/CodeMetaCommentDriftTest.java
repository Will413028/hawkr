package io.github.will413028.hawkr.productimport;

import io.github.will413028.hawkr.common.utils.code.CodeMeta;
import io.github.will413028.hawkr.common.utils.code.ImportAssetFailureReason;
import io.github.will413028.hawkr.common.utils.code.ImportAssetLocalStatus;
import io.github.will413028.hawkr.common.utils.code.ImportAssetMediaStatus;
import io.github.will413028.hawkr.common.utils.code.ImportAssetUsage;
import io.github.will413028.hawkr.common.utils.code.ImportItemErrorType;
import io.github.will413028.hawkr.common.utils.code.ImportItemStatus;
import io.github.will413028.hawkr.common.utils.code.ImportJobStatus;
import org.junit.jupiter.api.DisplayName;
import org.junit.jupiter.api.Test;

import java.io.IOException;
import java.nio.file.Files;
import java.nio.file.Path;
import java.nio.file.Paths;
import java.util.ArrayList;
import java.util.HashMap;
import java.util.List;
import java.util.Map;
import java.util.regex.Matcher;
import java.util.regex.Pattern;
import java.util.stream.Stream;

import static org.junit.jupiter.api.Assertions.assertTrue;

/**
 * 註解／migration 裡寫的 CodeMeta 值必須與 Enum 實際的 value 相符。
 *
 * <p><b>為什麼需要機械保證</b>：本 repo 守禁硬編碼與 CodeMeta SSOT
 *（docs/開發規範/Java 微服務開發規範.md §6、docs/代碼管理（CodeMeta）/README.md），狀態值一律用
 * {@code XxxEnum.XXX.value()} 注入，於是 SQL 與 Java 的<b>字面上看不到值</b>——
 * 唯一寫著值的地方就是給人看的註解與 migration 說明。那些地方寫錯不會有任何編譯錯誤、
 * 不會有任何測試轉紅，但下一個人會照它施工：把 {@code status = '9303'} 直接寫進一支新查詢，
 * 得到的是「零命中、排程靜默停擺」——沒有錯誤，只是每輪撈到 0 筆。
 *
 * <p><b>實例（issue #2294 L9）</b>：{@code V10__add_product_import_gc_lease.sql} 與
 * {@code ProductImportItemAssetMapper.claimStaleDownloadingAsset} 的註解都寫著
 * {@code status='9303'(DOWNLOADING)}，但 {@code ImportAssetLocalStatus.DOWNLOADING} 是
 * {@code '9302'}，{@code '9303'} 是 {@code UPLOADED} —— 意思正好相反
 *（「已上傳」而不是「下載中」）。
 *
 * <p>本測試掃 {@code src/main} 下所有 {@code .java} / {@code .sql}，把
 * {@code '<四位數>'(<ENUM_NAME>)} 這個書寫慣例當成一則可驗證的宣稱來檢查。
 */
class CodeMetaCommentDriftTest {

    /** 形如 {@code '9302'(DOWNLOADING)} 或 {@code '9302' (DOWNLOADING)}。 */
    private static final Pattern CLAIM = Pattern.compile("'(\\d{3,4})'\\s*\\(([A-Z][A-Z0-9_]{2,})\\)");

    private static final List<Class<? extends CodeMeta>> ENUMS = List.of(
            ImportAssetLocalStatus.class,
            ImportAssetMediaStatus.class,
            ImportAssetFailureReason.class,
            ImportAssetUsage.class,
            ImportItemStatus.class,
            ImportItemErrorType.class,
            ImportJobStatus.class);

    /** enum 常數名 → 該名字在上列所有 enum 裡出現過的合法 value（同名可能跨 enum）。 */
    private static Map<String, List<String>> valuesByName() {
        Map<String, List<String>> out = new HashMap<>();
        for (Class<? extends CodeMeta> c : ENUMS) {
            for (Object constant : c.getEnumConstants()) {
                Enum<?> e = (Enum<?>) constant;
                out.computeIfAbsent(e.name(), k -> new ArrayList<>()).add(((CodeMeta) constant).value());
            }
        }
        return out;
    }

    @Test
    @DisplayName("註解／migration 裡的 '<value>'(<ENUM_NAME>) 必須與 CodeMeta enum 相符（寫錯即轉紅）")
    void documentedCodeMetaValuesMatchEnums() throws IOException {
        Path mainDir = Paths.get("src", "main");
        assertTrue(Files.isDirectory(mainDir),
                "找不到 " + mainDir.toAbsolutePath() + "；本測試依賴 surefire 的 working dir = module 根目錄");

        Map<String, List<String>> valuesByName = valuesByName();
        List<String> drift = new ArrayList<>();
        int checked = 0;

        try (Stream<Path> files = Files.walk(mainDir)) {
            for (Path p : files.filter(Files::isRegularFile)
                    .filter(f -> f.toString().endsWith(".java") || f.toString().endsWith(".sql"))
                    .toList()) {
                String content = Files.readString(p);
                Matcher m = CLAIM.matcher(content);
                while (m.find()) {
                    String documentedValue = m.group(1);
                    String enumName = m.group(2);
                    List<String> legal = valuesByName.get(enumName);
                    if (legal == null) {
                        // 不是本服務 CodeMeta enum 的名字（例如 SQLSTATE 縮寫、其他領域的碼），跳過。
                        continue;
                    }
                    checked++;
                    if (!legal.contains(documentedValue)) {
                        drift.add(p + " 寫著 '" + documentedValue + "'(" + enumName
                                + ")，但 " + enumName + " 的合法 value 是 " + legal);
                    }
                }
            }
        }

        // 鑑別力自檢：掃不到任何一則宣稱時，這個測試恆綠而毫無價值（memory
        // feedback_no_blind_measurement：量測本身要先證明它答得出來）。
        assertTrue(checked > 0,
                "整個 src/main 掃不到任何 '<value>'(<ENUM_NAME>) 宣稱——"
                        + "書寫慣例改了就要同步改本測試的 pattern，否則它會靜默恆綠");
        assertTrue(drift.isEmpty(), "CodeMeta 註解漂移（共 " + drift.size() + " 處，已檢查 "
                + checked + " 則宣稱）：\n  " + String.join("\n  ", drift));
    }
}
