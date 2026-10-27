package io.github.will413028.hawkr.product.service.validator;

import io.github.will413028.hawkr.common.utils.code.CertificationType;
import io.github.will413028.hawkr.product.grpc.Certification;
import io.github.will413028.hawkr.product.grpc.ProductDraft;
import io.github.will413028.hawkr.product.grpc.ValidateProductDraftRequest.ValidationContext;
import io.github.will413028.hawkr.product.grpc.client.ShippingGrpcClient;
import io.github.will413028.hawkr.product.service.CategoryCacheService;
import io.github.will413028.hawkr.product.service.CategoryService;
import io.github.will413028.hawkr.product.service.GovCertificationService;
import io.github.will413028.hawkr.product.service.GovCertificationService.GovVerifyResult;
import io.github.will413028.hawkr.product.service.GovCertificationService.Validity;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.DisplayName;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.api.extension.ExtendWith;
import org.mockito.Mock;
import org.mockito.junit.jupiter.MockitoExtension;
import org.mockito.junit.jupiter.MockitoSettings;
import org.mockito.quality.Strictness;

import java.util.List;
import java.util.Optional;

import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.junit.jupiter.api.Assertions.assertFalse;
import static org.junit.jupiter.api.Assertions.assertTrue;
import static org.mockito.ArgumentMatchers.any;
import static org.mockito.ArgumentMatchers.anyString;
import static org.mockito.Mockito.never;
import static org.mockito.Mockito.verify;
import static org.mockito.Mockito.when;

/**
 * {@code ProductPublishValidator.validateCertifications}（設計書 19 §6.5.3）。
 *
 * <p><b>這裡守的是什麼</b>：本方法跑在 IMPORT_COMMIT／PUBLISH 等<b>同步寫入路徑</b>上。
 * 舊版在這裡直接打政府 API，一旦逾時就打開 per-type、TTL 60 秒、Redis 全域共享的 circuit
 * ⇒ 緊接著排出去的背景查驗落在那 60 秒窗口內被短路 ⇒ 背景查驗機制被同步階段自己封死。
 *
 * <p>因此第一支測試斷言的不是「結果對不對」，而是<b>「一次外部呼叫都沒有發生」</b>——
 * 那才是 §8.2「零外部呼叫是結構保證」真正的鎖。
 */
@ExtendWith(MockitoExtension.class)
@MockitoSettings(strictness = Strictness.LENIENT)
@DisplayName("ProductPublishValidator: 認證驗證只讀快取、拆碼、全部降為 warning")
class ProductPublishValidatorCertificationTest {

    private static final long STORE_ID = 2685914682L;
    private static final String ORGANIC_NO = "1-009-300022";

    @Mock private CategoryService categoryService;
    @Mock private CategoryCacheService categoryCacheService;
    @Mock private GovCertificationService govCertificationService;
    @Mock private ShippingGrpcClient shippingGrpcClient;

    private ProductPublishValidator validator;

    @BeforeEach
    void setUp() {
        when(categoryCacheService.findById(any())).thenReturn(Optional.empty());
        validator = new ProductPublishValidator(
                categoryService, categoryCacheService, govCertificationService, shippingGrpcClient);
    }

    private ProductDraft draftWith(String typeValue, String certNo) {
        return ProductDraft.newBuilder()
                .setName("測試商品")
                .addCertifications(Certification.newBuilder()
                        .setName(typeValue)
                        .setCertNo(certNo)
                        .build())
                .build();
    }

    private List<ProductPublishValidator.Violation> certViolations(ProductDraft draft) {
        return validator.validate(STORE_ID, draft, ValidationContext.IMPORT_COMMIT)
                .violations().stream()
                .filter(v -> v.field().startsWith("certifications["))
                .toList();
    }

    private static GovVerifyResult cached(Validity validity, boolean found) {
        return new GovVerifyResult("3701", ORGANIC_NO, "2026-08-26T04:12:09Z",
                found, validity, null, null, null, null, null, null, found ? 1 : 0, null);
    }

    // ==================== (2) 移除同步外部呼叫 ====================

    /**
     * 這一支是本檔的核心：舊版這裡是 {@code govCertificationService.verify(...)}。
     * 只要那行回來，這支就紅。
     */
    @Test
    @DisplayName("同步路徑一次外部呼叫都不得發生（只准讀結構化快取）")
    void neverCallsGovApiOnSyncPath() {
        when(govCertificationService.readCachedResult(anyString(), any())).thenReturn(null);

        certViolations(draftWith(CertificationType.ORGANIC.value(), ORGANIC_NO));

        verify(govCertificationService, never()).verify(anyString(), any());
        verify(govCertificationService, never()).verifyDetailed(anyString(), any());
        verify(govCertificationService, never()).verifyDetailedForBackground(anyString(), any());
        verify(govCertificationService).readCachedResult(ORGANIC_NO, CertificationType.ORGANIC);
    }

    @Test
    @DisplayName("快取未命中 → 不產生任何違規（交給背景查驗，§8.2）")
    void cacheMissProducesNoViolation() {
        when(govCertificationService.readCachedResult(anyString(), any())).thenReturn(null);

        assertTrue(certViolations(draftWith(CertificationType.ORGANIC.value(), ORGANIC_NO)).isEmpty());
    }

    @Test
    @DisplayName("快取命中且有效 → 不產生任何違規")
    void cachedValidProducesNoViolation() {
        when(govCertificationService.readCachedResult(anyString(), any()))
                .thenReturn(cached(Validity.VALID, true));

        assertTrue(certViolations(draftWith(CertificationType.ORGANIC.value(), ORGANIC_NO)).isEmpty());
    }

    // ==================== (1) 拆碼 + 都降為 warning ====================

    /**
     * 改前：類型不合法與「查無」共用 {@code CERT_VERIFY_FAILED} 且都是 {@code error()}。
     * 兩者共用一碼就只能有一份文案，而文案是<b>碼層級的對照表</b>
     * （{@code ImportErrorMessageCatalog} 與 website {@code products-import/i18n.ts} 都以碼為 key）
     * ⇒ 必有一邊的文案是錯的。
     */
    @Test
    @DisplayName("認證類型不合法 → CERT_ENUM_INVALID + WARNING（不再與查無共用一碼、不再阻擋）")
    void invalidTypeUsesEnumInvalidWarning() {
        List<ProductPublishValidator.Violation> v =
                certViolations(draftWith("產銷履歷", "58589605051")); // CSV 標籤而非碼值

        assertEquals(1, v.size(), "實際=" + v);
        assertEquals("CERT_ENUM_INVALID", v.get(0).code());
        assertEquals("WARNING", v.get(0).severity());
        assertTrue(v.get(0).field().endsWith(".name"));
        verify(govCertificationService, never()).readCachedResult(anyString(), any());
    }

    @Test
    @DisplayName("快取命中「查無」→ 維持 CERT_VERIFY_FAILED 但降為 WARNING（語意：已轉送人工審核）")
    void cachedNotFoundIsWarningNotError() {
        when(govCertificationService.readCachedResult(anyString(), any()))
                .thenReturn(cached(Validity.NOT_FOUND, false));

        List<ProductPublishValidator.Violation> v =
                certViolations(draftWith(CertificationType.ORGANIC.value(), ORGANIC_NO));

        assertEquals(1, v.size(), "實際=" + v);
        assertEquals("CERT_VERIFY_FAILED", v.get(0).code());
        assertEquals("WARNING", v.get(0).severity());
        assertTrue(v.get(0).message().contains("人工審核"),
                "文案語意必須是「已轉送人工審核」而不是「驗證失敗」，實際=" + v.get(0).message());
    }

    @Test
    @DisplayName("快取命中「已失效」→ 同樣是 WARNING，且訊息帶得出 EXPIRED 與 NOT_FOUND 的差別")
    void cachedExpiredIsWarning() {
        when(govCertificationService.readCachedResult(anyString(), any()))
                .thenReturn(cached(Validity.EXPIRED, true));

        List<ProductPublishValidator.Violation> v =
                certViolations(draftWith(CertificationType.ORGANIC.value(), ORGANIC_NO));

        assertEquals(1, v.size(), "實際=" + v);
        assertEquals("WARNING", v.get(0).severity());
        assertTrue(v.get(0).message().contains("EXPIRED"), "實際=" + v.get(0).message());
    }

    /**
     * 「降為 warning」的真正後果在這裡：{@code hasBlockingError()}。
     * 只改 severity 字串而 caller 照樣被擋，等於什麼都沒改。
     */
    @Test
    @DisplayName("認證問題一律不得阻擋 commit（不再有 ERROR 級認證違規）")
    void certificationIssuesNeverBlockCommit() {
        when(govCertificationService.readCachedResult(anyString(), any()))
                .thenReturn(cached(Validity.NOT_FOUND, false));

        ProductDraft draft = ProductDraft.newBuilder()
                .setName("測試商品")
                .addCertifications(Certification.newBuilder()
                        .setName(CertificationType.ORGANIC.value()).setCertNo(ORGANIC_NO).build())
                .addCertifications(Certification.newBuilder()
                        .setName("產銷履歷").setCertNo("58589605051").build())
                .build();

        List<ProductPublishValidator.Violation> certs = certViolations(draft);
        assertEquals(2, certs.size(), "實際=" + certs);
        assertTrue(certs.stream().noneMatch(v -> "ERROR".equals(v.severity())),
                "認證違規不得再有 ERROR：" + certs);
    }

    /**
     * {@code CERT_VERIFY_PENDING} 的 catch 分支隨外部呼叫一起移除（不再有例外可 catch）。
     * 碼值本身保留在 CodeMeta 不得刪除：{@code product_import_item_error} 既有列可能仍存著它。
     */
    @Test
    @DisplayName("CERT_VERIFY_PENDING 不再由本路徑發出（該分支已不可達）")
    void pendingCodeIsUnreachableHere() {
        when(govCertificationService.readCachedResult(anyString(), any()))
                .thenReturn(cached(Validity.UNKNOWN, false));

        List<ProductPublishValidator.Violation> v =
                certViolations(draftWith(CertificationType.ORGANIC.value(), ORGANIC_NO));

        assertFalse(v.stream().anyMatch(x -> "CERT_VERIFY_PENDING".equals(x.code())),
                "PENDING 已不可達：" + v);
    }
}
