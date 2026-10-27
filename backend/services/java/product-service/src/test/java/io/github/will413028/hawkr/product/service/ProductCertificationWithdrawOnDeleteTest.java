package io.github.will413028.hawkr.product.service;

import io.github.will413028.hawkr.common.utils.TsidUtil;
import io.github.will413028.hawkr.common.utils.code.ProductCertificationActorRole;
import io.github.will413028.hawkr.common.utils.code.ProductCertificationReviewAction;
import io.github.will413028.hawkr.common.utils.code.ProductCertificationStatus;
import io.github.will413028.hawkr.product.entity.ProductCertificationApplicationEntity;
import io.github.will413028.hawkr.product.entity.ProductCertificationReviewEventEntity;
import io.github.will413028.hawkr.product.grpc.client.SellerGrpcClient;
import io.github.will413028.hawkr.product.mq.ProductEventPublisher;
import io.github.will413028.hawkr.product.repository.ProductCertificationApplicationRepository;
import io.github.will413028.hawkr.product.repository.ProductCertificationReviewEventRepository;
import io.github.will413028.hawkr.product.repository.ProductRepository;
import com.fasterxml.jackson.databind.ObjectMapper;
import org.junit.jupiter.api.BeforeAll;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.DisplayName;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.api.extension.ExtendWith;
import org.mockito.ArgumentCaptor;
import org.mockito.Mock;
import org.mockito.junit.jupiter.MockitoExtension;
import org.mockito.junit.jupiter.MockitoSettings;
import org.mockito.quality.Strictness;

import java.util.List;

import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.junit.jupiter.api.Assertions.assertNull;
import static org.mockito.ArgumentMatchers.any;
import static org.mockito.ArgumentMatchers.anyLong;
import static org.mockito.ArgumentMatchers.anyString;
import static org.mockito.Mockito.never;
import static org.mockito.Mockito.verify;
import static org.mockito.Mockito.when;

/**
 * #2820：商品軟刪時連帶撤回待審的認證申請 —— 三個裁決各自釘住。
 *
 * <p>1. **只收 SUBMITTED，不動 APPROVED**：{@code REVOKED} 是平台對假標章的唯一治理手段，
 *    自動清掉已核准的申請等於給賣家一條「刪商品即抹除盜用證號紀錄」的路。
 * <p>2. **終態是 WITHDRAWN 不是新狀態**：碼表的 WITHDRAWN 已涵蓋隱性停止宣稱。
 * <p>3. **actor_role 是 SYSTEM 不是 SELLER**：賣家刪的是商品、不是撤回申請；
 *    且 SELLER 的契約要求 actor_account_id 是登入 principal，而該路徑拿不到。
 */
@ExtendWith(MockitoExtension.class)
@MockitoSettings(strictness = Strictness.LENIENT)
class ProductCertificationWithdrawOnDeleteTest {

    private static final Long PRODUCT_ID = 876667630441496665L;
    private static final Long APP_ID = 843900000000000011L;

    @Mock ProductCertificationApplicationRepository applicationRepository;
    @Mock ProductCertificationReviewEventRepository reviewEventRepository;
    @Mock ProductRepository productRepository;
    @Mock GovCertificationService govCertificationService;
    @Mock ProductCacheService cacheService;
    @Mock ProductRevalidationService revalidationService;
    @Mock IdempotencyService idempotencyService;
    @Mock ProductEventPublisher eventPublisher;
    @Mock SellerGrpcClient sellerGrpcClient;
    @Mock org.springframework.data.redis.core.StringRedisTemplate redisTemplate;

    ProductCertificationApplicationService service;

    @BeforeAll
    static void initTsid() {
        try {
            TsidUtil.init(200, 0);
        } catch (IllegalStateException ignored) {
            // 同一個 JVM 內的其他測試已初始化
        }
    }

    @BeforeEach
    void setUp() {
        service = new ProductCertificationApplicationService(
                applicationRepository, reviewEventRepository, productRepository,
                govCertificationService, cacheService, revalidationService, idempotencyService,
                new ObjectMapper(), eventPublisher, sellerGrpcClient, redisTemplate);
    }

    private ProductCertificationApplicationEntity submitted() {
        ProductCertificationApplicationEntity e = new ProductCertificationApplicationEntity();
        e.setId(APP_ID);
        e.setProductId(PRODUCT_ID);
        e.setStoreId(2639226113L);
        e.setCertificationType(3703);
        e.setCertificateNo("010105");
        e.setStatus(ProductCertificationStatus.SUBMITTED.value());
        e.setVersion(0L);
        return e;
    }

    @Test
    @DisplayName("待審申請轉 WITHDRAWN，且來源狀態明寫 SUBMITTED（讓轉換對重複執行冪等）")
    void withdrawsSubmittedApplications() {
        when(applicationRepository.lockSubmittedByProductId(PRODUCT_ID))
                .thenReturn(List.of(submitted()));
        when(applicationRepository.transitionToTerminalStatus(any(), anyString(), anyString(), any()))
                .thenReturn(1);

        int withdrawn = service.withdrawSubmittedOnProductDeleted(PRODUCT_ID);

        assertEquals(1, withdrawn);
        verify(applicationRepository).transitionToTerminalStatus(
                org.mockito.ArgumentMatchers.eq(List.of(APP_ID)),
                org.mockito.ArgumentMatchers.eq(ProductCertificationStatus.SUBMITTED.value()),
                org.mockito.ArgumentMatchers.eq(ProductCertificationStatus.WITHDRAWN.value()),
                any());
    }

    @Test
    @DisplayName("🔴 不得碰 APPROVED：只能走 SUBMITTED-only 的查詢，不得改用會撈到已核准的那支")
    void neverTouchesApprovedApplications() {
        when(applicationRepository.lockSubmittedByProductId(PRODUCT_ID)).thenReturn(List.of());

        service.withdrawSubmittedOnProductDeleted(PRODUCT_ID);

        verify(applicationRepository).lockSubmittedByProductId(PRODUCT_ID);
        // 這兩支會撈到 APPROVED —— 用它們就會把已核准的標章一起撤掉，
        // 等於讓賣家用「刪商品」抹除盜用證號的治理紀錄。
        verify(applicationRepository, never()).lockApprovedByProductExcludingTypes(anyLong(), any());
        verify(applicationRepository, never()).findApprovedByProductId(anyLong());
    }

    @Test
    @DisplayName("🔴 稽核事件的 actor 是 SYSTEM＋null，不是 SELLER（賣家刪的是商品，不是撤回申請）")
    void writesSystemActorNotSeller() {
        when(applicationRepository.lockSubmittedByProductId(PRODUCT_ID))
                .thenReturn(List.of(submitted()));
        when(applicationRepository.transitionToTerminalStatus(any(), anyString(), anyString(), any()))
                .thenReturn(1);

        service.withdrawSubmittedOnProductDeleted(PRODUCT_ID);

        ArgumentCaptor<ProductCertificationReviewEventEntity> captor =
                ArgumentCaptor.forClass(ProductCertificationReviewEventEntity.class);
        verify(reviewEventRepository).insert(captor.capture());
        ProductCertificationReviewEventEntity ev = captor.getValue();

        assertEquals(ProductCertificationReviewAction.WITHDRAW.value(), ev.getAction());
        assertEquals(ProductCertificationStatus.SUBMITTED.value(), ev.getFromStatus());
        assertEquals(ProductCertificationStatus.WITHDRAWN.value(), ev.getToStatus());
        assertEquals(ProductCertificationActorRole.SYSTEM.value(), ev.getActorRole(),
                "賣家刪的是商品不是撤回申請；且 SELLER 的契約要求 actor_account_id 是登入 principal");
        assertNull(ev.getActorAccountId(),
                "SYSTEM 的契約明寫 actor_account_id 必為 null，不得拿 store_id 頂替");
    }

    @Test
    @DisplayName("沒有待審申請時直接返回，不寫任何事件")
    void noPendingApplicationsWritesNothing() {
        when(applicationRepository.lockSubmittedByProductId(PRODUCT_ID)).thenReturn(List.of());

        assertEquals(0, service.withdrawSubmittedOnProductDeleted(PRODUCT_ID));

        verify(applicationRepository, never()).transitionToTerminalStatus(any(), anyString(), anyString(), any());
        verify(reviewEventRepository, never()).insert(any());
    }

    @Test
    @DisplayName("轉換回 0 列（被其他路徑搶先）時不寫事件 —— 事件必須對應真正發生的轉換")
    void doesNotWriteEventWhenTransitionAffectedNoRows() {
        when(applicationRepository.lockSubmittedByProductId(PRODUCT_ID))
                .thenReturn(List.of(submitted()));
        when(applicationRepository.transitionToTerminalStatus(any(), anyString(), anyString(), any()))
                .thenReturn(0);

        assertEquals(0, service.withdrawSubmittedOnProductDeleted(PRODUCT_ID));

        verify(reviewEventRepository, never()).insert(any());
    }
}
