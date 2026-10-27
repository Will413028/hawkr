package io.github.will413028.hawkr.media.repository;

import org.apache.ibatis.annotations.Select;
import org.apache.ibatis.annotations.Update;
import org.junit.jupiter.api.DisplayName;
import org.junit.jupiter.api.Test;

import java.lang.reflect.Method;
import java.time.LocalDateTime;

import static org.assertj.core.api.Assertions.assertThat;

/**
 * GC 的法定文件例外（設計書 58 §0「憑證與稽核保存 1826 天」、G6「媒體 GC 7 天必須例外化」）。
 *
 * <p>🔴 <b>為什麼用反射讀 annotation 而不是 mock repository</b>：本模組既有的兩支 GC 測試
 * （{@code MediaGarbageCollectionJobTest} / {@code ...ClaimTest}）都是 Mockito，repository 是替身
 * ⇒ 它們對「SQL 述詞長什麼樣」<b>零鑑別力</b>。而這個例外<b>整個</b>活在 SQL 裡，
 * 用 mock 寫出來的測試會在述詞被刪掉之後照樣全綠。
 *
 * <p>反射讀的是<b>真正會送進 DB 的那個字串</b>，不是原始碼檔案的文字——差別在於
 * 它跟著常數串接的結果走，改壞常數一樣抓得到。
 *
 * <p>⚠️ 它仍然驗不到「SQL 語意正確」（那需要真的資料庫，而本模組沒有 Testcontainers）。
 * 語意面於 2026-09-02 以 dev 實測補上。
 */
class MediaAssetGcExclusionTest {

    @Test
    @DisplayName("候選查詢必須排除法定 ref_type —— 少了它，發票 PDF 失去引用 7 天後被實體刪除")
    void selectOrphanedAssetsExcludesStatutoryRefTypes() throws Exception {
        Method m = MediaAssetRepository.class.getMethod(
            "selectOrphanedAssets", LocalDateTime.class, LocalDateTime.class, int.class);
        String sql = m.getAnnotation(Select.class).value()[0];
        assertThat(normalize(sql))
            .as("selectOrphanedAssets 的述詞遺漏法定文件例外")
            .contains(normalize(MediaAssetRepository.STATUTORY_GC_EXCLUSION));
    }

    @Test
    @DisplayName("claim 也必須排除 —— 它是刪 R2 物件前最後一個能重新驗證候選條件的位置")
    void claimAssetForGcExcludesStatutoryRefTypes() throws Exception {
        Method m = MediaAssetRepository.class.getMethod(
            "claimAssetForGc", Long.class, LocalDateTime.class, LocalDateTime.class, LocalDateTime.class);
        String sql = m.getAnnotation(Update.class).value()[0];
        assertThat(normalize(sql))
            .as("claimAssetForGc 遺漏法定文件例外；候選查詢擋住了而 claim 沒擋，"
                + "等於 TOCTOU 期間仍可能刪掉法定憑證")
            .contains(normalize(MediaAssetRepository.STATUTORY_GC_EXCLUSION));
    }

    @Test
    @DisplayName("例外清單必須含發票 PDF 與認證文件，且放行 ref_type IS NULL")
    void exclusionCoversStatutoryTypes() {
        String s = MediaAssetRepository.STATUTORY_GC_EXCLUSION;
        // 7120 = INVOICE_DOCUMENT（電子發票／收據／折讓單 PDF）
        // 7111 = CERTIFICATION（商家認證與稅籍證明）
        assertThat(s).contains("'7120'").contains("'7111'");
        // NULL 要放行：ref_type 為空的舊列不該因為這個例外變成永不回收
        // ——那會把「保護法定憑證」擴張成「什麼都不回收」。
        assertThat(s).contains("IS NULL");
    }

    private static String normalize(String s) {
        return s.replaceAll("\\s+", " ").trim();
    }
}
