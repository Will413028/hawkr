package io.github.will413028.hawkr.media.storage;

import io.github.will413028.hawkr.common.utils.code.AssetReferenceType;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.DisplayName;
import org.junit.jupiter.api.Test;
import org.springframework.test.util.ReflectionTestUtils;

import java.time.LocalDate;
import java.time.format.DateTimeFormatter;

import io.github.will413028.hawkr.media.exception.MediaServiceException;

import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.junit.jupiter.api.Assertions.assertFalse;
import static org.junit.jupiter.api.Assertions.assertThrows;
import static org.junit.jupiter.api.Assertions.assertTrue;

/**
 * {@link AbstractStorageService} 的 objectKey 生成邏輯測試（與 backend 無關，
 * 以 {@link R2StorageService} 具體實例驅動）。
 *
 * <p>#1203 回歸守門：import 暫存 key 為 5 段（{@code tmp/{date}/{storeId}/import/{uuid}}），
 * 不可把字面 {@code import} 當成日期段，必須以 {@code yyyyMMdd} 樣式定位日期段，
 * 產出符合 08 §2.2 格式的 {@code public/{category}/{ownerId}/{date}/{uuid}}。
 */
class AbstractStorageServiceTest {

    private static final DateTimeFormatter DATE_FMT = DateTimeFormatter.ofPattern("yyyyMMdd");
    private static final String REF_PRODUCT = AssetReferenceType.PRODUCT.value();
    private static final long ACCOUNT_ID = 2683382637L;

    private AbstractStorageService service;

    @BeforeEach
    void setUp() {
        service = new R2StorageService();
        ReflectionTestUtils.setField(service, "publicPrefix", "public");
        ReflectionTestUtils.setField(service, "securePrefix", "secure");
        ReflectionTestUtils.setField(service, "tempPrefix", "tmp");
    }

    @Test
    @DisplayName("generateObjectKey：正常副檔名產出 tmp/{date}/{uuid}.jpg")
    void generateObjectKey_normalExtension() {
        String key = service.generateObjectKey("photo.jpg", false);
        assertTrue(key.startsWith("tmp/"), "公開暫存走 tmp 前綴");
        assertTrue(key.endsWith(".jpg"));
    }

    @Test
    @DisplayName("消毒（49 §8.1）：副檔名含 Windows 路徑分隔符 \\ → 拒絕")
    void generateObjectKey_rejectsBackslashInExtension() {
        assertThrows(MediaServiceException.class,
            () -> service.generateObjectKey("evil.jp\\g", false));
    }

    @Test
    @DisplayName("消毒（49 §8.1）：副檔名含路徑分隔符 / → 拒絕")
    void generateObjectKey_rejectsSlashInExtension() {
        assertThrows(MediaServiceException.class,
            () -> service.generateObjectKey("evil.jp/g", false));
    }

    @Test
    @DisplayName("儲存前綴取自 CodeMeta Code：SELLER_LOGO → public/seller_logo/（#2199）")
    void finalKeyPrefixTracksCodeMetaCode() {
        // 前綴唯一定義點就是這一行：AssetReferenceType.fromValue(refType).code().toLowerCase()。
        // 賣家 logo 的 Code 原為 USER_AVATAR，物件因此落在 public/user_avatar/ —— 路由與欄位
        // 早已收斂成 logo/logoKey，只剩儲存層名不符實（#2199）。改 CodeMeta 的 Code 欄即改前綴，
        // 這條測試把「哪個 refType 落哪個前綴」釘死，避免日後改 CodeMeta 時無聲改動儲存版面。
        String original = "tmp/20260805/8c2f2f2e-0000-4000-8000-000000000001.png";

        String result = service.generateFinalObjectKey(
                original, false, AssetReferenceType.SELLER_LOGO.value(), ACCOUNT_ID);

        assertEquals(
                "public/seller_logo/2683382637/20260805/8c2f2f2e-0000-4000-8000-000000000001.png",
                result);
        assertFalse(result.contains("user_avatar"),
                "賣家 logo 不得再落在 user_avatar/ 前綴（#2199）");
    }

    @Test
    @DisplayName("買家頭像維持 public/buyer_avatar/：#2199 只改賣家 logo，不動其他 refType")
    void buyerAvatarPrefixUnchanged() {
        String original = "tmp/20260805/8c2f2f2e-0000-4000-8000-000000000002.png";

        String result = service.generateFinalObjectKey(
                original, false, AssetReferenceType.BUYER_AVATAR.value(), ACCOUNT_ID);

        assertEquals(
                "public/buyer_avatar/2683382637/20260805/8c2f2f2e-0000-4000-8000-000000000002.png",
                result);
    }

    @Test
    @DisplayName("import 5 段暫存 key：取 yyyyMMdd 日期段，不可把 import 當日期（#1203）")
    void importFiveSegmentKey_usesDateNotImportLiteral() {
        String original = "tmp/20260628/2683382637/import/35a54767-78a4-412e-b124-8b205ce77f90.jpg";

        String result = service.generateFinalObjectKey(original, false, REF_PRODUCT, ACCOUNT_ID);

        assertEquals(
                "public/product/2683382637/20260628/35a54767-78a4-412e-b124-8b205ce77f90.jpg",
                result);
        assertFalse(result.contains("/import/"),
                "finalized 路徑不可含 import 段（會被 Media 孤兒 GC 當暫存物件清掉）");
    }

    @Test
    @DisplayName("一般上傳 3 段暫存 key：行為不變")
    void normalThreeSegmentKey_unchanged() {
        String original = "tmp/20260628/abcd1234.jpg";

        String result = service.generateFinalObjectKey(original, false, REF_PRODUCT, ACCOUNT_ID);

        assertEquals("public/product/2683382637/20260628/abcd1234.jpg", result);
    }

    @Test
    @DisplayName("secure 上傳 3 段暫存 key：visibility 走 securePrefix")
    void secureThreeSegmentKey_usesSecurePrefix() {
        String original = "secure/20260628/abcd1234.pdf";

        String result = service.generateFinalObjectKey(original, true, REF_PRODUCT, ACCOUNT_ID);

        assertEquals("secure/product/2683382637/20260628/abcd1234.pdf", result);
    }

    @Test
    @DisplayName("無可辨識日期段：fallback 為今天日期，末段仍為 filename")
    void noDateSegment_fallsBackToToday() {
        String today = LocalDate.now().format(DATE_FMT);
        String original = "tmp/import/abcd1234.jpg";

        String result = service.generateFinalObjectKey(original, false, REF_PRODUCT, ACCOUNT_ID);

        assertEquals("public/product/2683382637/" + today + "/abcd1234.jpg", result);
        assertTrue(result.endsWith("/abcd1234.jpg"));
    }
}
