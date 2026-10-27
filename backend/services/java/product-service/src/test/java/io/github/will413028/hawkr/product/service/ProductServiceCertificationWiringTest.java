package io.github.will413028.hawkr.product.service;

import io.github.will413028.hawkr.common.utils.code.ProductCertificationSource;
import io.github.will413028.hawkr.common.utils.code.ProductStatus;
import io.github.will413028.hawkr.product.converter.ProductEntityConverter;
import io.github.will413028.hawkr.product.converter.ProductSkuConverter;
import io.github.will413028.hawkr.product.dto.request.CertificationDTO;
import io.github.will413028.hawkr.product.dto.request.UpdateProductDTO;
import io.github.will413028.hawkr.product.dto.response.CertificationWarningDTO;
import io.github.will413028.hawkr.product.dto.response.ProductDetailDTO;
import io.github.will413028.hawkr.product.dto.response.ProductWriteOutcome;
import io.github.will413028.hawkr.product.entity.ProductEntity;
import io.github.will413028.hawkr.product.repository.ProductRepository;
import io.github.will413028.hawkr.product.repository.ProductSkuRepository;
import io.github.will413028.hawkr.product.service.helper.ProductMediaReferenceLinker;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.DisplayName;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.api.extension.ExtendWith;
import org.mockito.ArgumentCaptor;
import org.mockito.InjectMocks;
import org.mockito.Mock;
import org.mockito.junit.jupiter.MockitoExtension;
import org.mockito.junit.jupiter.MockitoSettings;
import org.mockito.quality.Strictness;

import java.util.ArrayList;
import java.util.HashMap;
import java.util.List;
import java.util.Map;
import java.util.Optional;

import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.junit.jupiter.api.Assertions.assertFalse;
import static org.junit.jupiter.api.Assertions.assertNull;
import static org.junit.jupiter.api.Assertions.assertSame;
import static org.mockito.ArgumentMatchers.any;
import static org.mockito.ArgumentMatchers.anyBoolean;
import static org.mockito.ArgumentMatchers.anyLong;
import static org.mockito.Mockito.verify;
import static org.mockito.Mockito.when;

/**
 * {@code ProductService} → {@code ProductCertificationApplicationService} 的<b>接線</b>契約
 * （19 號設計書 §6.1 W1–W4）。
 *
 * <p>收斂點自己的行為由 {@code ProductCertificationApplicationServiceTest} 守；本檔只守
 * 「{@code ProductService} 到底把什麼餵給了它」——這一段沒有測試的話，
 * 收斂點做得再對也可能收到錯的 {@code source} 或被頂替的送出者，而且兩者都是零訊號。
 */
@ExtendWith(MockitoExtension.class)
@MockitoSettings(strictness = Strictness.LENIENT)
class ProductServiceCertificationWiringTest {

    private static final long PID = 1001L;
    private static final long STORE = 2685914682L;

    @Mock ProductRepository productRepository;
    @Mock io.github.will413028.hawkr.product.repository.StoreExposureRepository storeExposureRepository;
    @Mock ProductSkuRepository productSkuRepository;
    @Mock ProductEntityConverter productEntityConverter;
    @Mock ProductSkuConverter productSkuConverter;
    @Mock IdempotencyService idempotencyService;
    @Mock ProductCacheService cacheService;
    @Mock CategoryService categoryService;
    @Mock HashtagService hashtagService;
    @Mock ProductRevalidationService productRevalidationService;
    @Mock ProductMediaReferenceLinker mediaReferenceLinker;
    @Mock ProductCertificationApplicationService certificationApplicationService;
    @InjectMocks ProductService productService;

    @BeforeEach
    void wireHappyPath() {
        ProductEntity persisted = new ProductEntity();
        persisted.setId(PID);
        persisted.setStoreId(STORE);
        persisted.setStatus(ProductStatus.INACTIVE.value());
        persisted.setName("有機小松菜");
        persisted.setCategoryLv1(11L);
        persisted.setCategoryLv2(12L);
        persisted.setCategory(13L);
        persisted.setStockQty(30L);
        persisted.setVersion(3L);

        when(productRepository.findByIdAndStoreId(PID, STORE)).thenReturn(Optional.of(persisted));
        when(productSkuRepository.findByProductId(PID)).thenReturn(new ArrayList<>());
        when(productRepository.updateById(any())).thenReturn(1);
        when(productRepository.findByIdIncludingDeleted(anyLong())).thenReturn(Optional.of(persisted));
        when(productEntityConverter.toDetailDTO(any(), any(), anyBoolean())).thenReturn(new ProductDetailDTO());
        Map<Long, String> names = new HashMap<>(Map.of(11L, "蔬菜", 12L, "葉菜", 13L, "小松菜"));
        when(categoryService.getCategoryNames(any())).thenReturn(names);
        when(certificationApplicationService.syncFromProductWrite(any())).thenReturn(List.of());
    }

    private UpdateProductDTO dto(boolean delegated, List<CertificationDTO> certs, boolean updateCerts) {
        UpdateProductDTO dto = new UpdateProductDTO();
        dto.setIdempotencyKey("idem-" + System.nanoTime());
        dto.setProductId(String.valueOf(PID));
        dto.setStoreId(STORE);
        dto.setName("有機小松菜（改名）");
        dto.setCertifications(certs);
        dto.setUpdateCertifications(updateCerts);
        dto.setDelegated(delegated);
        return dto;
    }

    private ProductCertificationApplicationService.CertificationSyncCommand captureCommand() {
        ArgumentCaptor<ProductCertificationApplicationService.CertificationSyncCommand> captor =
                ArgumentCaptor.forClass(ProductCertificationApplicationService.CertificationSyncCommand.class);
        verify(certificationApplicationService).syncFromProductWrite(captor.capture());
        return captor.getValue();
    }

    /**
     * 賣家自助與 ADM 代行是<b>不同的人</b>做同一件事。都記成 {@code SELLER}，
     * 稽核就永遠分不出「這張標章是店主自己掛的，還是平台代行掛上去的」。
     */
    @Test
    @DisplayName("代行更新 ⇒ source = DELEGATED_ADMIN")
    void delegatedUpdateUsesDelegatedAdminSource() {
        productService.updateProduct(dto(true,
                List.of(CertificationDTO.builder().name("3701").certNo("1-009-300022").build()), true));

        assertEquals(ProductCertificationSource.DELEGATED_ADMIN.value(), captureCommand().source());
    }

    @Test
    @DisplayName("賣家自助更新 ⇒ source = SELLER")
    void sellerUpdateUsesSellerSource() {
        productService.updateProduct(dto(false,
                List.of(CertificationDTO.builder().name("3701").certNo("1-009-300022").build()), true));

        assertEquals(ProductCertificationSource.SELLER.value(), captureCommand().source());
    }

    /**
     * 🔴 {@code store_id} 是營運鍵、{@code account_id} 是登入 principal，兩者只在首任
     * grandfather 時恰好相等 ⇒ 頂替會讓 dev 與所有 fixture 全綠，<b>只有 re-key 過的店家錯
     * 而且沒有任何告警</b>（CLAUDE.md §6）。UpdateProductRequest 沒有 account_id 欄位，
     * 正解是留 NULL，不是編一個假的 principal 出來。
     */
    @Test
    @DisplayName("submittedByAccountId 必須是 null，不得被 storeId 頂替")
    void submitterIsNullNotStoreId() {
        productService.updateProduct(dto(false,
                List.of(CertificationDTO.builder().name("3701").certNo("1-009-300022").build()), true));

        ProductCertificationApplicationService.CertificationSyncCommand cmd = captureCommand();
        assertNull(cmd.submittedByAccountId(),
                "沒有 account_id 可用時要留 NULL；拿 storeId 頂替是零訊號的身分錯置");
        assertEquals(STORE, cmd.storeId(), "storeId 本身仍要正確帶入（它是擁有權欄位）");
    }

    /**
     * 🔴 Step 0 閘門的兩個輸入都必須<b>原樣</b>往下傳，由收斂點判定。
     * {@code ProductService} 若自行提前 return，收斂點那一整套判定就形同虛設。
     */
    @Test
    @DisplayName("旗標與清單原樣傳給收斂點，ProductService 不得自行提前 return")
    void gateInputsArePassedThroughUntouched() {
        productService.updateProduct(dto(false, null, false));

        ProductCertificationApplicationService.CertificationSyncCommand cmd = captureCommand();
        assertFalse(cmd.updateCertifications());
        assertNull(cmd.certifications(),
                "null 與空清單在 Step 0 的意義不同，不得在這一層被抹平成空清單");
    }

    /** 收斂點回的警告必須原樣出現在寫入結果裡，否則欄位存在卻恆空（§9.5）。 */
    @Test
    @DisplayName("收斂點的 certificationWarnings 要出現在 ProductWriteOutcome")
    void warningsAreSurfacedInOutcome() {
        CertificationWarningDTO warning =
                CertificationWarningDTO.concurrentConflict("3701", "1-009-300022");
        when(certificationApplicationService.syncFromProductWrite(any())).thenReturn(List.of(warning));

        ProductWriteOutcome outcome = productService.updateProductWithWarnings(dto(false,
                List.of(CertificationDTO.builder().name("3701").certNo("1-009-300022").build()), true));

        assertEquals(1, outcome.certificationWarnings().size());
        assertSame(warning, outcome.certificationWarnings().get(0));
    }
}
