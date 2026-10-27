package io.github.will413028.hawkr.product.service;

import io.github.will413028.hawkr.common.utils.TsidUtil;
import io.github.will413028.hawkr.common.utils.DateTimeUtils;
import io.github.will413028.hawkr.common.utils.code.CertificationType;
import io.github.will413028.hawkr.common.utils.code.GrpcResponseCode;
import io.github.will413028.hawkr.common.utils.code.ProductCertificationGovVerifyState;
import io.github.will413028.hawkr.common.utils.code.ProductCertificationReviewAction;
import io.github.will413028.hawkr.common.utils.code.ProductCertificationSource;
import io.github.will413028.hawkr.common.utils.code.ProductCertificationStatus;
import io.github.will413028.hawkr.product.dto.response.CertificationApplicationRefDTO;
import io.github.will413028.hawkr.product.dto.response.CertificationWarningDTO;
import io.github.will413028.hawkr.product.entity.ProductCertificationApplicationEntity;
import io.github.will413028.hawkr.product.entity.ProductCertificationReviewEventEntity;
import io.github.will413028.hawkr.product.entity.ProductEntity;
import io.github.will413028.hawkr.product.exception.ProductBusinessException;
import io.github.will413028.hawkr.product.grpc.client.SellerGrpcClient;
import io.github.will413028.hawkr.product.repository.ProductCertificationApplicationRepository;
import io.github.will413028.hawkr.product.repository.ProductCertificationReviewEventRepository;
import io.github.will413028.hawkr.product.repository.ProductRepository;
import com.fasterxml.jackson.databind.ObjectMapper;
import org.junit.jupiter.api.BeforeAll;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.DisplayName;
import org.junit.jupiter.api.Nested;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.api.extension.ExtendWith;
import org.junit.jupiter.params.ParameterizedTest;
import org.junit.jupiter.params.provider.Arguments;
import org.junit.jupiter.params.provider.MethodSource;
import org.mockito.ArgumentCaptor;
import org.mockito.InOrder;
import org.mockito.Mock;
import org.mockito.junit.jupiter.MockitoExtension;
import org.mockito.junit.jupiter.MockitoSettings;
import org.mockito.quality.Strictness;

import java.time.LocalDateTime;
import java.util.ArrayList;
import java.util.List;
import java.util.Optional;
import java.util.stream.Stream;

import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.junit.jupiter.api.Assertions.assertFalse;
import static org.junit.jupiter.api.Assertions.assertNotNull;
import static org.junit.jupiter.api.Assertions.assertThrows;
import static org.junit.jupiter.api.Assertions.assertTrue;
import static org.mockito.ArgumentMatchers.any;
import static org.mockito.ArgumentMatchers.anyInt;
import static org.mockito.ArgumentMatchers.anyLong;
import static org.mockito.ArgumentMatchers.anyString;
import static org.mockito.ArgumentMatchers.eq;
import static org.mockito.Mockito.never;
import static org.mockito.Mockito.times;
import static org.mockito.Mockito.verify;
import static org.mockito.Mockito.when;

/**
 * {@link ProductCertificationApplicationService} 的行為契約（19 號設計書《農業認證驗證》§6／§7）。
 *
 * <p>每一組測試對應設計書裡一個「失效訊號為零」的缺口 —— 這些缺口的共同特徵是
 * <b>HTTP 200、資料形狀正常、稽核表有列</b>，只有語意是錯的，任何 schema 檢查與型別測試都抓不到。
 * 所以斷言全部下在「哪一句 SQL 被呼叫、帶什麼參數、以什麼順序」。
 */
@ExtendWith(MockitoExtension.class)
@MockitoSettings(strictness = Strictness.LENIENT)
class ProductCertificationApplicationServiceTest {

    private static final Long PRODUCT_ID = 843856384063471755L;
    private static final Long STORE_ID = 2685914682L;
    private static final Long ACCOUNT_ID = 2685914682L;
    /** 賣家登入 principal；刻意與 {@link #STORE_ID} 不同（re-key 過的店家），頂替會被抓到 */
    private static final Long SELLER_ACCOUNT_ID = 9002685914682L;
    private static final int ORGANIC = 3701;
    private static final String CERT_A = "1-009-300022";
    private static final String CERT_B = "1-009-300099";
    /** §8.5.1 那一組共用的申請 id 與「政府查驗回的業者名稱」（與 {@link #validResult} 同值） */
    private static final Long HOLDER_APP_ID = 55L;
    private static final String HOLDER_NAME = "○○農場";

    @Mock ProductCertificationApplicationRepository applicationRepository;
    @Mock ProductCertificationReviewEventRepository reviewEventRepository;
    @Mock ProductRepository productRepository;
    @Mock GovCertificationService govCertificationService;
    @Mock ProductCacheService cacheService;
    @Mock ProductRevalidationService revalidationService;
    @Mock IdempotencyService idempotencyService;
    @Mock io.github.will413028.hawkr.product.mq.ProductEventPublisher eventPublisher;
    @Mock io.github.will413028.hawkr.product.grpc.client.SellerGrpcClient sellerGrpcClient;
    @Mock org.springframework.data.redis.core.StringRedisTemplate redisTemplate;
    @Mock org.springframework.data.redis.core.ValueOperations<String, String> redisValueOps;

    ProductCertificationApplicationService service;

    @BeforeAll
    static void initTsid() {
        // product-service 的 node-id-base（§7.2）；重複初始化會拋，已初始化就沿用
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
        // 重新查驗租約：預設「拿得到」，個別測試再覆寫成 false 模擬併發
        when(redisTemplate.opsForValue()).thenReturn(redisValueOps);
        when(redisValueOps.setIfAbsent(anyString(), anyString(), anyLong(), any())).thenReturn(true);
        stubEmptyRepository();
        // 同步階段一律只讀快取；預設 miss
        when(govCertificationService.readCachedResult(anyString(), any())).thenReturn(null);
        // 決定事件的收件人解析（fail-closed）；預設解析得到，個別測試再覆寫成拋錯
        when(sellerGrpcClient.getSellerMetaOrThrow(anyLong()))
                .thenReturn(io.github.will413028.hawkr.seller.grpc.SellerMeta.newBuilder()
                        .setAccountId(SELLER_ACCOUNT_ID)
                        .setStoreId(String.valueOf(STORE_ID))
                        .build());
    }

    /** 預設：查不到任何既有申請、商品存在且投影為空。{@code reset(mock)} 之後要重新套用。 */
    private void stubEmptyRepository() {
        when(applicationRepository.findByProductTypeCertNoAndStatuses(anyLong(), anyInt(), anyString(), any()))
                .thenReturn(List.of());
        when(applicationRepository.lockSubmittedByProductAndTypeExceptCertNo(anyLong(), anyInt(), anyString()))
                .thenReturn(List.of());
        when(applicationRepository.lockApprovedByProductExcludingTypes(anyLong(), any()))
                .thenReturn(List.of());
        when(applicationRepository.findApprovedByProductId(anyLong())).thenReturn(List.of());
        when(applicationRepository.findApprovedByProductAndType(anyLong(), anyInt())).thenReturn(Optional.empty());
        when(applicationRepository.insertSubmittedIfAbsent(any(), any(), any(), any(), any(), any(),
                any(), anyBooleanArg(), any(), any(), any(), any())).thenReturn(1);
        when(productRepository.findByIdIncludingDeleted(anyLong()))
                .thenReturn(Optional.of(productWithProjection()));
        when(productRepository.updateCertificationProjection(anyLong(), any())).thenReturn(1);
        // 冪等：預設不是重放
        when(idempotencyService.acquireLock(anyString(), any(), anyString(), any(), any()))
                .thenReturn(Optional.empty());
        // 收斂點插完會把該列讀回來寫 SUBMIT 事件；預設回一筆 PENDING（不符自動核准資格）
        when(applicationRepository.findById(anyLong())).thenAnswer(inv ->
                Optional.of(application(inv.getArgument(0), ORGANIC, CERT_A,
                        ProductCertificationStatus.SUBMITTED.value(), 0L)));
    }

    private static boolean anyBooleanArg() {
        return org.mockito.ArgumentMatchers.anyBoolean();
    }

    // ==================== ① Step 0 前置閘門（§6.3.0） ====================

    @Nested
    @DisplayName("§6.3 Step 0：effectiveUpdate 閘門")
    class Step0Gate {

        /**
         * 這一格就是設計書點名「本設計書最危險的一個缺口」：任何部分更新（舊 App 只改名稱、
         * 代行只改單一欄位）都會讓「本次送出的 type 集合」變成空集合，
         * §6.4 於是把該商品<b>全部 APPROVED 一次轉 WITHDRAWN</b> ⇒ 法規標章消失，
         * 而 HTTP 200、稽核有列、WITHDRAWN 不發推播 ⇒ 零訊號。
         */
        @Test
        @DisplayName("旗標 false ＋ 未帶認證 ⇒ 一筆都不能動（既有 APPROVED 不得被撤回）")
        void partialUpdateMustNotTouchAnyCertification() {
            List<CertificationWarningDTO> warnings = service.syncFromProductWrite(
                    syncCommand(false, null));

            assertTrue(warnings.isEmpty());
            // 連「有哪些 type 沒送」都不准去查——查了就代表閘門在它後面
            verify(applicationRepository, never()).lockApprovedByProductExcludingTypes(anyLong(), any());
            verify(applicationRepository, never()).transitionToTerminalStatus(any(), anyString(), anyString(), any());
            verify(applicationRepository, never()).insertSubmittedIfAbsent(any(), any(), any(), any(), any(),
                    any(), any(), anyBooleanArg(), any(), any(), any(), any());
            verify(productRepository, never()).updateCertificationProjection(anyLong(), any());
        }

        @Test
        @DisplayName("旗標 false ＋ 空清單（新 client 送空但沒設旗標）⇒ 同樣一筆都不能動")
        void emptyListWithoutFlagMustNotTouchAnyCertification() {
            service.syncFromProductWrite(syncCommand(false, List.of()));

            verify(applicationRepository, never()).lockApprovedByProductExcludingTypes(anyLong(), any());
        }

        /**
         * 反方向的零訊號失效：條件若寫成 {@code if (!updateCertifications) return;}，
         * 舊 App 送了認證但沒設旗標的請求會直接 return ⇒ 賣家送出的認證靜默蒸發。
         * proto 該欄註解明訂「相容期：未設但 certifications 非空仍視為要更新」，
         * 而 api/proto 的契約優先序高於設計書。
         */
        @Test
        @DisplayName("旗標 false ＋ 認證非空（舊 App）⇒ 仍然要建立申請")
        void legacyClientWithoutFlagStillCreatesApplication() {
            service.syncFromProductWrite(syncCommand(false, List.of(input(ORGANIC, CERT_A))));

            verify(applicationRepository).insertSubmittedIfAbsent(
                    any(), eq(PRODUCT_ID), eq(STORE_ID), eq(null), eq(ORGANIC), eq(CERT_A),
                    eq(ProductCertificationSource.SELLER.value()), eq(false),
                    eq(ProductCertificationGovVerifyState.PENDING.value()), any(), any(), any());
        }

        @Test
        @DisplayName("旗標 true ＋ 空陣列 ⇒ 這是唯一合法的『清空』，既有 APPROVED 轉 WITHDRAWN ＋ 重建投影")
        void explicitClearWithdrawsApprovedAndRebuildsProjection() {
            ProductCertificationApplicationEntity approved = application(11L, ORGANIC, CERT_A,
                    ProductCertificationStatus.APPROVED.value(), 3L);
            when(productRepository.findByIdIncludingDeleted(PRODUCT_ID))
                    .thenReturn(Optional.of(productWithProjection(ORGANIC, CERT_A)));
            when(applicationRepository.lockApprovedByProductExcludingTypes(eq(PRODUCT_ID), eq(List.of())))
                    .thenReturn(List.of(approved));
            when(applicationRepository.transitionToTerminalStatus(eq(List.of(11L)),
                    eq(ProductCertificationStatus.APPROVED.value()),
                    eq(ProductCertificationStatus.WITHDRAWN.value()), any())).thenReturn(1);

            service.syncFromProductWrite(syncCommand(true, List.of()));

            verify(applicationRepository).transitionToTerminalStatus(eq(List.of(11L)),
                    eq(ProductCertificationStatus.APPROVED.value()),
                    eq(ProductCertificationStatus.WITHDRAWN.value()), any());
            assertEquals(ProductCertificationReviewAction.WITHDRAW.value(),
                    capturedEvents().get(0).getAction());
            verify(productRepository).updateCertificationProjection(eq(PRODUCT_ID), any());
        }
    }

    // ==================== ② 換號情境（§6.4 粒度 ＋ §3.5） ====================

    @Nested
    @DisplayName("§6.4 換號：舊號碼保持公開到新號碼核准為止")
    class CertificateNumberSwap {

        /**
         * 粒度若寫成 (type, certNo)，賣家把 A 改成 B 時 payload 只有 B ⇒ A 落入「沒送」
         * ⇒ 送出當下就 WITHDRAWN、標章立即消失，而 B 還在待審。
         */
        @Test
        @DisplayName("送出同 type 的新號碼 B ⇒ 舊的 A 仍是 APPROVED、投影不動")
        void submittingNewNumberKeepsOldApprovedPublic() {
            service.syncFromProductWrite(syncCommand(true, List.of(input(ORGANIC, CERT_B))));

            // 該 type 有送任一號碼 ⇒ 不在「沒送」集合內
            verify(applicationRepository).lockApprovedByProductExcludingTypes(PRODUCT_ID, List.of(ORGANIC));
            // 沒有任何 APPROVED 被轉成 WITHDRAWN，投影也不該被改寫
            verify(applicationRepository, never()).transitionToTerminalStatus(any(),
                    eq(ProductCertificationStatus.APPROVED.value()), anyString(), any());
            verify(productRepository, never()).updateCertificationProjection(anyLong(), any());
        }

        @Test
        @DisplayName("B 核准後 ⇒ 投影只剩 B，A 轉 SUPERSEDED 且留下稽核事件")
        void approvingNewNumberSupersedesOldAndProjectsOnlyNew() {
            ProductCertificationApplicationEntity oldApproved = application(11L, ORGANIC, CERT_A,
                    ProductCertificationStatus.APPROVED.value(), 3L);
            ProductCertificationApplicationEntity pending = application(22L, ORGANIC, CERT_B,
                    ProductCertificationStatus.SUBMITTED.value(), 0L);

            when(applicationRepository.findByIdForUpdate(22L)).thenReturn(Optional.of(pending));
            when(applicationRepository.findApprovedByProductAndType(PRODUCT_ID, ORGANIC))
                    .thenReturn(Optional.of(oldApproved));
            when(applicationRepository.transitionToTerminalStatus(eq(List.of(11L)), anyString(), anyString(), any()))
                    .thenReturn(1);
            when(applicationRepository.casTransitionStatus(eq(22L), eq(0L), anyString(), anyString(),
                    any(), any(), any(), any(), any())).thenReturn(1);
            // 核准後 approved 集合只剩 B
            when(applicationRepository.findApprovedByProductId(PRODUCT_ID)).thenReturn(List.of(
                    application(22L, ORGANIC, CERT_B, ProductCertificationStatus.APPROVED.value(), 1L)));
            when(applicationRepository.findById(22L)).thenReturn(Optional.of(
                    application(22L, ORGANIC, CERT_B, ProductCertificationStatus.APPROVED.value(), 1L)));

            service.approveForAdmin(22L, 0L, 999L, null, null, "idem-approve");

            ArgumentCaptor<List<ProductEntity.CertificationItem>> captor = projectionCaptor();
            verify(productRepository).updateCertificationProjection(eq(PRODUCT_ID), captor.capture());
            assertEquals(1, captor.getValue().size());
            assertEquals(CERT_B, captor.getValue().get(0).getCertNo(),
                    "投影必須只剩新號碼；舊號碼已被 SUPERSEDE");

            List<ProductCertificationReviewEventEntity> events = capturedEvents();
            assertTrue(events.stream().anyMatch(e ->
                            ProductCertificationReviewAction.SUPERSEDE.value().equals(e.getAction())),
                    "被換掉的那一筆必須留下 SUPERSEDE 事件，否則賣家的申請憑空消失且查不到是誰動的");
            assertTrue(events.stream().anyMatch(e ->
                    ProductCertificationReviewAction.APPROVE.value().equals(e.getAction())));
        }
    }

    // ==================== ③ 核准順序（§6.6 ①→②） ====================

    @Nested
    @DisplayName("§6.6 核准交易：① SUPERSEDE 必須在 ② APPROVE 之前")
    class ApproveOrdering {

        /**
         * {@code uk_pca_active} 是 partial unique <b>index</b>，PostgreSQL 無法 deferrable、
         * 逐句即時檢查。順序顛倒 ⇒ 那一瞬間同 (product, type) 有兩列 APPROVED
         * ⇒ 23505 ⇒ 整個交易 abort。而換號核准是主路徑不是邊角情境。
         *
         * <p>本測試的鑑別力：把實作裡 ①② 兩段對調，{@code InOrder} 會直接紅。
         */
        @Test
        @DisplayName("先把舊 APPROVED 轉 SUPERSEDED，再把本筆轉 APPROVED")
        void supersedeMustHappenBeforeApprove() {
            ProductCertificationApplicationEntity oldApproved = application(11L, ORGANIC, CERT_A,
                    ProductCertificationStatus.APPROVED.value(), 3L);
            ProductCertificationApplicationEntity pending = application(22L, ORGANIC, CERT_B,
                    ProductCertificationStatus.SUBMITTED.value(), 0L);

            when(applicationRepository.findByIdForUpdate(22L)).thenReturn(Optional.of(pending));
            when(applicationRepository.findApprovedByProductAndType(PRODUCT_ID, ORGANIC))
                    .thenReturn(Optional.of(oldApproved));
            when(applicationRepository.transitionToTerminalStatus(eq(List.of(11L)), anyString(), anyString(), any()))
                    .thenReturn(1);
            when(applicationRepository.casTransitionStatus(eq(22L), eq(0L), anyString(), anyString(),
                    any(), any(), any(), any(), any())).thenReturn(1);
            when(applicationRepository.findById(22L)).thenReturn(Optional.of(pending));

            service.approveForAdmin(22L, 0L, 999L, null, null, "idem-approve");

            InOrder inOrder = org.mockito.Mockito.inOrder(applicationRepository);
            inOrder.verify(applicationRepository).transitionToTerminalStatus(
                    eq(List.of(11L)),
                    eq(ProductCertificationStatus.APPROVED.value()),
                    eq(ProductCertificationStatus.SUPERSEDED.value()),
                    any());
            inOrder.verify(applicationRepository).casTransitionStatus(eq(22L), eq(0L),
                    eq(ProductCertificationStatus.SUBMITTED.value()),
                    eq(ProductCertificationStatus.APPROVED.value()),
                    any(), any(), any(), any(), any());
        }

        @Test
        @DisplayName("expectedVersion 不符 ⇒ 拋 3026 讓交易 rollback（不得走 onNext + code 的既有慣例）")
        void staleExpectedVersionThrowsSoTransactionRollsBack() {
            ProductCertificationApplicationEntity pending = application(22L, ORGANIC, CERT_B,
                    ProductCertificationStatus.SUBMITTED.value(), 5L);
            when(applicationRepository.findByIdForUpdate(22L)).thenReturn(Optional.of(pending));

            ProductBusinessException e = assertThrows(ProductBusinessException.class,
                    () -> service.approveForAdmin(22L, 2L, 999L, null, null, "idem-approve"));
            assertEquals(GrpcResponseCode.CERT_APPLICATION_VERSION_CONFLICT.code(), e.code());
            // 版本不符時，① 都不該發生 —— 否則賣家原本的待審申請被吞掉而沒有新申請
            verify(applicationRepository, never()).transitionToTerminalStatus(any(), anyString(), anyString(), any());
        }

        @Test
        @DisplayName("終態不得再被核准 ⇒ 3027")
        void terminalStatusCannotBeApproved() {
            when(applicationRepository.findByIdForUpdate(22L)).thenReturn(Optional.of(
                    application(22L, ORGANIC, CERT_B, ProductCertificationStatus.REJECTED.value(), 1L)));

            ProductBusinessException e = assertThrows(ProductBusinessException.class,
                    () -> service.approveForAdmin(22L, 1L, 999L, null, null, "idem-approve"));
            assertEquals(GrpcResponseCode.CERT_APPLICATION_STATE_INVALID.code(), e.code());
        }
    }

    // ==================== ④ rebuildProjection 的四個觸發點（§7.2） ====================

    @Nested
    @DisplayName("§7.2 投影重建的四個觸發點")
    class ProjectionTriggers {

        @Test
        @DisplayName("APPROVE（人工）")
        void approveRebuildsProjection() {
            ProductCertificationApplicationEntity pending = application(22L, ORGANIC, CERT_B,
                    ProductCertificationStatus.SUBMITTED.value(), 0L);
            when(applicationRepository.findByIdForUpdate(22L)).thenReturn(Optional.of(pending));
            when(applicationRepository.casTransitionStatus(eq(22L), eq(0L), anyString(), anyString(),
                    any(), any(), any(), any(), any())).thenReturn(1);
            when(applicationRepository.findApprovedByProductId(PRODUCT_ID)).thenReturn(List.of(
                    application(22L, ORGANIC, CERT_B, ProductCertificationStatus.APPROVED.value(), 1L)));
            when(applicationRepository.findById(22L)).thenReturn(Optional.of(pending));

            service.approveForAdmin(22L, 0L, 999L, null, null, "idem-approve");

            verify(productRepository).updateCertificationProjection(eq(PRODUCT_ID), any());
        }

        /**
         * APPROVE（系統自動）—— 同步快取命中「有效」且 type 允許自動核准（§8.5 只有 3701／3702）。
         * 這條路徑同時驗證了「Step 5 一律以 SUBMITTED 插入、再走 §6.6 兩步」，
         * 而不是直接以 APPROVED 插入（那會撞 uk_pca_active 被 DO NOTHING 靜默丟棄）。
         */
        @Test
        @DisplayName("APPROVE（系統）：同步快取命中有效 ⇒ 插 SUBMITTED 後在同一交易內核准並重建投影")
        void systemAutoApproveRebuildsProjection() {
            when(govCertificationService.readCachedResult(eq(CERT_A), eq(CertificationType.ORGANIC)))
                    .thenReturn(validResult(CERT_A));
            // 剛插入的列：VERIFIED、非人工複審、version = 0
            ProductCertificationApplicationEntity inserted = application(33L, ORGANIC, CERT_A,
                    ProductCertificationStatus.SUBMITTED.value(), 0L);
            inserted.setGovVerifyState(ProductCertificationGovVerifyState.VERIFIED.value());
            // §8.5.1 第五個條件：payload 的業者名稱必須屬於這個賣家。真實路徑的 payload 由
            // toSnapshot(validResult) 產生，holderName 就是 HOLDER_NAME；mock 走 findById 繞過了
            // 那段轉換，所以在這裡補上，否則本案例會停在 NO_HOLDER_NAME 而非驗到投影重建。
            ProductCertificationApplicationEntity.GovVerifyPayload payload =
                    new ProductCertificationApplicationEntity.GovVerifyPayload();
            payload.setCertNo(CERT_A);
            payload.setHolderName(HOLDER_NAME);
            inserted.setGovVerifyPayload(payload);
            when(sellerGrpcClient.matchCertificateHolder(STORE_ID, HOLDER_NAME))
                    .thenReturn(new SellerGrpcClient.HolderMatch(true, true, "REAL_NAME", true));
            when(applicationRepository.findById(anyLong())).thenReturn(Optional.of(inserted));
            when(applicationRepository.findByIdForUpdate(anyLong())).thenReturn(Optional.of(inserted));
            when(applicationRepository.casTransitionStatus(anyLong(), eq(0L), anyString(), anyString(),
                    any(), any(), any(), any(), any())).thenReturn(1);
            when(applicationRepository.findApprovedByProductId(PRODUCT_ID)).thenReturn(List.of(
                    application(33L, ORGANIC, CERT_A, ProductCertificationStatus.APPROVED.value(), 1L)));

            service.syncFromProductWrite(syncCommand(true, List.of(input(ORGANIC, CERT_A))));

            // 一律以 SUBMITTED 插入（insertSubmittedIfAbsent 的 SQL 寫死 'SUBMITTED'）
            verify(applicationRepository).insertSubmittedIfAbsent(any(), eq(PRODUCT_ID), any(), any(),
                    eq(ORGANIC), eq(CERT_A), any(), eq(false),
                    eq(ProductCertificationGovVerifyState.VERIFIED.value()), any(), any(), any());
            verify(productRepository).updateCertificationProjection(eq(PRODUCT_ID), any());
            assertTrue(capturedEvents().stream().anyMatch(e ->
                            ProductCertificationReviewAction.AUTO_APPROVE.value().equals(e.getAction())
                                    && "SYSTEM".equals(e.getActorRole())),
                    "自動核准也必須寫事件，否則事後查不出這張標章是誰放行的");
        }

        @Test
        @DisplayName("SUPERSEDE（§6.3 Step 4 的併發副產物也要寫事件）")
        void supersedeInStep4WritesEvent() {
            ProductCertificationApplicationEntity otherPending = application(44L, ORGANIC, CERT_A,
                    ProductCertificationStatus.SUBMITTED.value(), 0L);
            when(applicationRepository.lockSubmittedByProductAndTypeExceptCertNo(PRODUCT_ID, ORGANIC, CERT_B))
                    .thenReturn(List.of(otherPending));
            when(applicationRepository.transitionToTerminalStatus(eq(List.of(44L)),
                    eq(ProductCertificationStatus.SUBMITTED.value()),
                    eq(ProductCertificationStatus.SUPERSEDED.value()), any())).thenReturn(1);

            service.syncFromProductWrite(syncCommand(true, List.of(input(ORGANIC, CERT_B))));

            assertTrue(capturedEvents().stream().anyMatch(e ->
                            ProductCertificationReviewAction.SUPERSEDE.value().equals(e.getAction())
                                    && Long.valueOf(44L).equals(e.getApplicationId())),
                    "每一列被 Step 4 SUPERSEDE 都要寫事件（含併發競態造成的退場）");
        }

        @Test
        @DisplayName("WITHDRAW：由 APPROVED 轉出才重建投影；由 SUBMITTED 轉出不動投影")
        void withdrawRebuildsProjectionOnlyFromApproved() {
            ProductCertificationApplicationEntity approved = application(55L, ORGANIC, CERT_A,
                    ProductCertificationStatus.APPROVED.value(), 2L);
            // 起始投影必須含 A：起始就是空的話「撤回後也是空」⇒ rebuildProjection 判定沒變、
            // 本測試會在實作正確時照樣紅，等於量到的是 fixture 不是行為
            when(productRepository.findByIdIncludingDeleted(PRODUCT_ID))
                    .thenReturn(Optional.of(productWithProjection(ORGANIC, CERT_A)));
            when(applicationRepository.findByIdForUpdate(55L)).thenReturn(Optional.of(approved));
            when(applicationRepository.casTransitionStatus(eq(55L), eq(2L), anyString(), anyString(),
                    any(), any(), any(), any(), any())).thenReturn(1);
            when(applicationRepository.findById(55L)).thenReturn(Optional.of(approved));

            service.withdrawForSeller(PRODUCT_ID, 55L, STORE_ID, ACCOUNT_ID, 2L, "idem-wd");
            verify(productRepository).updateCertificationProjection(eq(PRODUCT_ID), any());

            org.mockito.Mockito.reset(productRepository);
            when(productRepository.findByIdIncludingDeleted(anyLong()))
                    .thenReturn(Optional.of(productWithProjection(ORGANIC, CERT_A)));
            ProductCertificationApplicationEntity submitted = application(66L, ORGANIC, CERT_B,
                    ProductCertificationStatus.SUBMITTED.value(), 0L);
            when(applicationRepository.findByIdForUpdate(66L)).thenReturn(Optional.of(submitted));
            when(applicationRepository.casTransitionStatus(eq(66L), eq(0L), anyString(), anyString(),
                    any(), any(), any(), any(), any())).thenReturn(1);
            when(applicationRepository.findById(66L)).thenReturn(Optional.of(submitted));

            service.withdrawForSeller(PRODUCT_ID, 66L, STORE_ID, ACCOUNT_ID, 0L, "idem-wd2");
            verify(productRepository, never()).updateCertificationProjection(anyLong(), any());
        }

        @Test
        @DisplayName("REVOKE：標章必須立刻從公開投影消失")
        void revokeRebuildsProjection() {
            ProductCertificationApplicationEntity approved = application(77L, ORGANIC, CERT_A,
                    ProductCertificationStatus.APPROVED.value(), 4L);
            when(productRepository.findByIdIncludingDeleted(PRODUCT_ID))
                    .thenReturn(Optional.of(productWithProjection(ORGANIC, CERT_A)));
            when(applicationRepository.findByIdForUpdate(77L)).thenReturn(Optional.of(approved));
            when(applicationRepository.casTransitionStatus(eq(77L), eq(4L), anyString(), anyString(),
                    any(), any(), any(), any(), any())).thenReturn(1);
            when(applicationRepository.findById(77L)).thenReturn(Optional.of(approved));
            when(applicationRepository.findApprovedByProductId(PRODUCT_ID)).thenReturn(List.of());

            service.revokeForAdmin(77L, 4L, 999L, "CERT_HOLDER_MISMATCH", "非本店所有", null, "idem-revoke");

            ArgumentCaptor<List<ProductEntity.CertificationItem>> captor = projectionCaptor();
            verify(productRepository).updateCertificationProjection(eq(PRODUCT_ID), captor.capture());
            assertTrue(captor.getValue().isEmpty(), "撤銷後 approved 集合為空，投影必須跟著清空");
        }

        @Test
        @DisplayName("撤銷缺 reasonCode ⇒ 3028，且不得動到任何列")
        void revokeWithoutReasonCodeIsRejected() {
            ProductBusinessException e = assertThrows(ProductBusinessException.class,
                    () -> service.revokeForAdmin(77L, 4L, 999L, "  ", null, null, "idem-revoke"));
            assertEquals(GrpcResponseCode.CERT_REVIEW_REASON_REQUIRED.code(), e.code());
            verify(applicationRepository, never()).casTransitionStatus(anyLong(), anyLong(), anyString(),
                    anyString(), any(), any(), any(), any(), any());
        }

        /** 投影沒變就不寫 —— 否則每次商品編輯都白白 bump 一次 product.version，代行的 expectedVersion 平白失效。 */
        @Test
        @DisplayName("approved 集合沒變 ⇒ 不覆寫投影、不 bump version")
        void unchangedProjectionIsNotRewritten() {
            ProductEntity product = productWithProjection();
            ProductEntity.CertificationItem item = new ProductEntity.CertificationItem();
            item.setName(String.valueOf(ORGANIC));
            item.setCertNo(CERT_A);
            product.setCertifications(new ArrayList<>(List.of(item)));
            when(productRepository.findByIdIncludingDeleted(PRODUCT_ID)).thenReturn(Optional.of(product));
            when(applicationRepository.findApprovedByProductId(PRODUCT_ID)).thenReturn(List.of(
                    application(11L, ORGANIC, CERT_A, ProductCertificationStatus.APPROVED.value(), 1L)));

            assertFalse(service.rebuildProjection(PRODUCT_ID));
            verify(productRepository, never()).updateCertificationProjection(anyLong(), any());
        }
    }

    // ==================== ⑤ 非法輸入剔除（§6.2） ====================

    @Nested
    @DisplayName("§6.2 非法輸入：剔除該筆，不拋例外、不回 4xx")
    class InvalidInput {

        /**
         * certNo 的非法值在舊版 App 上可達（AI 上架助手 patch 的值會繞過畫面驗證直接送出）。
         * 改成 400 ⇒ 已上架 App 的 AI 上架流程當場壞掉。
         */
        @Test
        @DisplayName("非法 type 與非法 certNo 各自被剔除，其他合法筆數照常進申請表")
        void illegalEntriesAreDroppedWithoutThrowing() {
            List<CertificationWarningDTO> warnings = service.syncFromProductWrite(syncCommand(true, List.of(
                    input("產銷履歷", CERT_A),        // 中文標籤，不是 CodeMeta value
                    input(ORGANIC, "abc def"),        // 空白不在白名單字元集內
                    input(ORGANIC, CERT_B))));        // 合法

            assertEquals(2, warnings.size());
            assertEquals(CertificationWarningDTO.REASON_INVALID_TYPE, warnings.get(0).reason());
            assertEquals("產銷履歷", warnings.get(0).certificationType(),
                    "警告要原樣回送賣家送來的值，前端不得假設它是合法碼");
            assertEquals(CertificationWarningDTO.REASON_INVALID_CERT_NO, warnings.get(1).reason());

            verify(applicationRepository, times(1)).insertSubmittedIfAbsent(any(), any(), any(), any(),
                    any(), any(), any(), anyBooleanArg(), any(), any(), any(), any());
            verify(applicationRepository).insertSubmittedIfAbsent(any(), eq(PRODUCT_ID), any(), any(),
                    eq(ORGANIC), eq(CERT_B), any(), anyBooleanArg(), any(), any(), any(), any());
        }

        /**
         * 序列情境插不出衝突（Step 4 已先清掉同 type 的異號待審）⇒ 只有併發才會走到這裡。
         * 不顯式檢查 rowsAffected 就是靜默資料遺失：無列、無錯。
         */
        @Test
        @DisplayName("ON CONFLICT DO NOTHING 沒插進去 ⇒ 落進 certificationWarnings，不 rollback、不回 4xx")
        void concurrentConflictBecomesWarningNotError() {
            when(applicationRepository.insertSubmittedIfAbsent(any(), any(), any(), any(), any(), any(),
                    any(), anyBooleanArg(), any(), any(), any(), any())).thenReturn(0);

            List<CertificationWarningDTO> warnings = service.syncFromProductWrite(
                    syncCommand(true, List.of(input(ORGANIC, CERT_A))));

            assertEquals(1, warnings.size());
            assertEquals(CertificationWarningDTO.REASON_CONCURRENT_CONFLICT, warnings.get(0).reason());
        }

        @Test
        @DisplayName("同一次請求送兩筆完全相同的 (type, certNo) ⇒ 只建一筆，也不發警告")
        void duplicateInputIsDeduped() {
            List<CertificationWarningDTO> warnings = service.syncFromProductWrite(syncCommand(true, List.of(
                    input(ORGANIC, CERT_A), input(ORGANIC, CERT_A))));

            assertTrue(warnings.isEmpty());
            verify(applicationRepository, times(1)).insertSubmittedIfAbsent(any(), any(), any(), any(),
                    any(), any(), any(), anyBooleanArg(), any(), any(), any(), any());
        }
    }


    // ==================== §6.2.9 certNo 依類型分流 ====================

    @Nested
    @DisplayName("§6.2.9：certNo 的合法性依類型而異")
    class CertNoRuleByType {

        /**
         * <b>本組最鋒利的一支</b>：同一個字面值、兩個 type、相反結果。
         * 分流不存在（六型都套嚴格）⇒ 3705 那格會紅；分流反了 ⇒ 兩格都紅。
         */
        @Test
        @DisplayName("同一個 ISO 22000:2018：3705 保留、3701 剔除")
        void sameValueOppositeOutcomeAcrossTypes() {
            String isoNo = "ISO 22000:2018";

            List<CertificationWarningDTO> relaxed = service.syncFromProductWrite(
                    syncCommand(true, List.of(input(3705, isoNo))));
            assertTrue(relaxed.isEmpty(), "3705 僅人工審核，含冒號與空格的真實編號必須保留");
            verify(applicationRepository).insertSubmittedIfAbsent(any(), any(), any(), any(),
                    eq(3705), eq(isoNo), any(), anyBooleanArg(), any(), any(), any(), any());

            org.mockito.Mockito.reset(applicationRepository);
            stubEmptyRepository();

            List<CertificationWarningDTO> strict = service.syncFromProductWrite(
                    syncCommand(true, List.of(input(3701, isoNo))));
            assertEquals(1, strict.size());
            assertEquals(CertificationWarningDTO.REASON_INVALID_CERT_NO, strict.get(0).reason(),
                    "3701 的值會被組進政府 API 查詢字串，必須維持嚴格白名單");
            verify(applicationRepository, never()).insertSubmittedIfAbsent(any(), any(), any(), any(),
                    any(), any(), any(), anyBooleanArg(), any(), any(), any(), any());
        }

        @Test
        @DisplayName("HACCP／GMP 的空格與斜線編號必須保留（放寬類不是不驗，是換一組規則）")
        void relaxedTypesAcceptRealWorldFormats() {
            service.syncFromProductWrite(syncCommand(true, List.of(
                    input(3704, "HACCP 2026 DP 0047"),
                    input(3706, "GMP/2026/0031"))));

            verify(applicationRepository).insertSubmittedIfAbsent(any(), any(), any(), any(),
                    eq(3704), eq("HACCP 2026 DP 0047"), any(), anyBooleanArg(), any(), any(), any(), any());
            verify(applicationRepository).insertSubmittedIfAbsent(any(), any(), any(), any(),
                    eq(3706), eq("GMP/2026/0031"), any(), anyBooleanArg(), any(), any(), any(), any());
        }

        /** 放寬 ≠ 不驗：控制字元與角括號仍要擋（後者是 ADM 詳情頁的注入面）。 */
        @Test
        @DisplayName("放寬類仍擋控制字元與角括號")
        void relaxedTypesStillRejectControlCharsAndAngleBrackets() {
            List<CertificationWarningDTO> warnings = service.syncFromProductWrite(syncCommand(true, List.of(
                    input(3704, "HACCP2026"),
                    input(3705, "<script>ISO</script>"))));

            assertEquals(2, warnings.size());
            assertTrue(warnings.stream().allMatch(w ->
                    CertificationWarningDTO.REASON_INVALID_CERT_NO.equals(w.reason())));
            verify(applicationRepository, never()).insertSubmittedIfAbsent(any(), any(), any(), any(),
                    any(), any(), any(), anyBooleanArg(), any(), any(), any(), any());
        }

        /**
         * 長度上限用<b>字元數</b>不是位元組：「食品GMP認證字第○○號」這類編號在 UTF-8 下
         * 一個中文字佔 3 bytes，用 byte 長度會讓 22 個中文字就爆掉。
         */
        @Test
        @DisplayName("長度用字元數不是位元組：64 個中文字通過、65 個被擋")
        void relaxedLengthIsCountedInCharactersNotBytes() {
            String sixtyFour = "食".repeat(64);
            String sixtyFive = "食".repeat(65);

            List<CertificationWarningDTO> warnings = service.syncFromProductWrite(syncCommand(true, List.of(
                    input(3706, sixtyFour))));
            assertTrue(warnings.isEmpty(), "64 個中文字 = 192 bytes，用 byte 長度判會被誤擋");
            verify(applicationRepository).insertSubmittedIfAbsent(any(), any(), any(), any(),
                    eq(3706), eq(sixtyFour), any(), anyBooleanArg(), any(), any(), any(), any());

            org.mockito.Mockito.reset(applicationRepository);
            stubEmptyRepository();

            warnings = service.syncFromProductWrite(syncCommand(true, List.of(input(3706, sixtyFive))));
            assertEquals(1, warnings.size());
            assertEquals(CertificationWarningDTO.REASON_INVALID_CERT_NO, warnings.get(0).reason());
        }

        /**
         * 🔴 驗什麼就存什麼。若「驗 trim 後的值、存原值」，從試算表貼進來的尾端換行會通過驗證
         * 然後原樣落進申請表——驗證看起來有做，被擋的東西卻進去了。
         */
        @Test
        @DisplayName("放寬類：存進 DB 的必須是正規化（trim）後的值，不是原值")
        void relaxedTypesPersistNormalizedValue() {
            service.syncFromProductWrite(syncCommand(true, List.of(
                    input(3704, "  HACCP-2026-DP-0047\n"))));

            verify(applicationRepository).insertSubmittedIfAbsent(any(), any(), any(), any(),
                    eq(3704), eq("HACCP-2026-DP-0047"), any(), anyBooleanArg(), any(), any(), any(), any());
        }

        /**
         * 嚴格類刻意<b>不 trim</b>：它與 {@code GovCertificationService.isValidCertNo} 是同一條規則的
         * 拷貝，多做一次 trim 會讓「兩邊逐字一致」變得不精確。
         */
        @Test
        @DisplayName("嚴格類對原值判：前後空白直接判不合法，不先 trim 再放行")
        void strictTypesJudgeRawValueWithoutTrimming() {
            List<CertificationWarningDTO> warnings = service.syncFromProductWrite(syncCommand(true, List.of(
                    input(ORGANIC, " " + CERT_A + " "))));

            assertEquals(1, warnings.size());
            assertEquals(CertificationWarningDTO.REASON_INVALID_CERT_NO, warnings.get(0).reason());
            verify(applicationRepository, never()).insertSubmittedIfAbsent(any(), any(), any(), any(),
                    any(), any(), any(), anyBooleanArg(), any(), any(), any(), any());
        }

        /**
         * 單筆端點刻意不走剔除（§6.2.9 末段）：被靜默剔掉的話賣家看到的是
         * 「送出成功但列表沒有這筆」，比回一個 typed 錯誤更糟。
         */
        @Test
        @DisplayName("單筆 submit：非法 certNo 回業務碼，不走剔除")
        void singleSubmitReturnsBusinessCodeInsteadOfDropping() {
            when(productRepository.findByIdAndStoreId(PRODUCT_ID, STORE_ID))
                    .thenReturn(Optional.of(productWithProjection()));

            ProductBusinessException e = assertThrows(ProductBusinessException.class,
                    () -> service.submitForSeller(PRODUCT_ID, STORE_ID, ACCOUNT_ID, "3701", "ISO 22000:2018", "idem-submit"));
            assertEquals(GrpcResponseCode.INVALID_PARAM.code(), e.code());
            verify(applicationRepository, never()).insertSubmittedIfAbsent(any(), any(), any(), any(),
                    any(), any(), any(), anyBooleanArg(), any(), any(), any(), any());
        }

        @Test
        @DisplayName("單筆 submit：放寬類的合法編號要能送出，且存正規化後的值")
        void singleSubmitAcceptsRelaxedTypeAndPersistsNormalized() {
            when(productRepository.findByIdAndStoreId(PRODUCT_ID, STORE_ID))
                    .thenReturn(Optional.of(productWithProjection()));
            ProductCertificationApplicationEntity created = application(91L, 3705, "ISO 22000:2018",
                    ProductCertificationStatus.SUBMITTED.value(), 0L);
            when(applicationRepository.findById(anyLong())).thenReturn(Optional.of(created));
            when(applicationRepository.findByIdForUpdate(anyLong())).thenReturn(Optional.of(created));

            service.submitForSeller(PRODUCT_ID, STORE_ID, ACCOUNT_ID, "3705", "  ISO 22000:2018  ", "idem-submit");

            verify(applicationRepository).insertSubmittedIfAbsent(any(), eq(PRODUCT_ID), any(), any(),
                    eq(3705), eq("ISO 22000:2018"), any(), anyBooleanArg(),
                    eq(ProductCertificationGovVerifyState.UNSUPPORTED.value()), any(), any(), any());
        }
    }

    // ==================== §6.3 Step 1/2/3 ====================

    @Nested
    @DisplayName("§6.3 Step 1–3：冪等與裁決不可被重送繞過")
    class IdempotencyAndManualReview {

        /**
         * Step 1 若不在最前面：全量取代語意下每次商品編輯都會把整份 approved 投影原樣送回，
         * 而同 (type, certNo) 已 APPROVED 時不觸發任何唯一索引衝突
         * ⇒ 每存一次商品就替每個已核准認證多建一筆 SUBMITTED，ADM 佇列被灌爆。
         */
        @Test
        @DisplayName("Step 1：同組已 APPROVED ⇒ no-op，不再建申請、也不執行 Step 4")
        void alreadyApprovedIsNoop() {
            when(applicationRepository.findByProductTypeCertNoAndStatuses(PRODUCT_ID, ORGANIC, CERT_A,
                    List.of(ProductCertificationStatus.APPROVED.value())))
                    .thenReturn(List.of(application(11L, ORGANIC, CERT_A,
                            ProductCertificationStatus.APPROVED.value(), 1L)));

            service.syncFromProductWrite(syncCommand(true, List.of(input(ORGANIC, CERT_A))));

            verify(applicationRepository, never()).insertSubmittedIfAbsent(any(), any(), any(), any(),
                    any(), any(), any(), anyBooleanArg(), any(), any(), any(), any());
            verify(applicationRepository, never())
                    .lockSubmittedByProductAndTypeExceptCertNo(anyLong(), anyInt(), anyString());
        }

        @Test
        @DisplayName("Step 3：曾被 REJECTED 的號碼重送 ⇒ requiresManualReview=true（永不自動核准）")
        void resubmittingRejectedNumberRequiresManualReview() {
            when(applicationRepository.findByProductTypeCertNoAndStatuses(PRODUCT_ID, ORGANIC, CERT_A,
                    List.of(ProductCertificationStatus.REJECTED.value(),
                            ProductCertificationStatus.REVOKED.value())))
                    .thenReturn(List.of(application(11L, ORGANIC, CERT_A,
                            ProductCertificationStatus.REJECTED.value(), 2L)));
            // 政府查驗剛好通過也不得放行
            when(govCertificationService.readCachedResult(eq(CERT_A), any())).thenReturn(validResult(CERT_A));
            ProductCertificationApplicationEntity inserted = application(33L, ORGANIC, CERT_A,
                    ProductCertificationStatus.SUBMITTED.value(), 0L);
            inserted.setGovVerifyState(ProductCertificationGovVerifyState.VERIFIED.value());
            inserted.setRequiresManualReview(true);
            when(applicationRepository.findById(anyLong())).thenReturn(Optional.of(inserted));
            when(applicationRepository.findByIdForUpdate(anyLong())).thenReturn(Optional.of(inserted));

            service.syncFromProductWrite(syncCommand(true, List.of(input(ORGANIC, CERT_A))));

            verify(applicationRepository).insertSubmittedIfAbsent(any(), any(), any(), any(),
                    eq(ORGANIC), eq(CERT_A), any(), eq(true), any(), any(), any(), any());
            verify(applicationRepository, never()).casTransitionStatus(anyLong(), anyLong(), anyString(),
                    anyString(), any(), any(), any(), any(), any());
        }

        /**
         * §8.5：3703 CAS 的 {@code Emblem_ID} 是工廠標誌編號，實測一個編號對到 173 個產品
         * ⇒ 查得到只證明該工廠有 CAS 資格，自動核准等於讓賣家用工廠編號給任意商品掛 CAS 標章。
         */
        @Test
        @DisplayName("3703 CAS 即使查驗通過也不得自動核准")
        void casTypeIsNeverAutoApproved() {
            int cas = 3703;
            when(govCertificationService.readCachedResult(eq("010105"), eq(CertificationType.CAS)))
                    .thenReturn(new GovCertificationService.GovVerifyResult(
                            "3703", "010105", "2026-08-26T04:12:09Z", true,
                            GovCertificationService.Validity.VALID, null, null,
                            "○○食品股份有限公司", "台南市", null, "冷凍蔬菜", 173, null));
            ProductCertificationApplicationEntity inserted = application(88L, cas, "010105",
                    ProductCertificationStatus.SUBMITTED.value(), 0L);
            inserted.setGovVerifyState(ProductCertificationGovVerifyState.VERIFIED.value());
            when(applicationRepository.findById(anyLong())).thenReturn(Optional.of(inserted));
            when(applicationRepository.findByIdForUpdate(anyLong())).thenReturn(Optional.of(inserted));

            service.syncFromProductWrite(syncCommand(true, List.of(input(cas, "010105"))));

            verify(applicationRepository, never()).casTransitionStatus(anyLong(), anyLong(), anyString(),
                    anyString(), any(), any(), any(), any(), any());
            verify(productRepository, never()).updateCertificationProjection(anyLong(), any());
        }

        /** 3704／3705／3706 無政府開放資料介面 ⇒ 直接 UNSUPPORTED，不排背景查驗、不打任何外部呼叫。 */
        @Test
        @DisplayName("3704 HACCP ⇒ UNSUPPORTED，且不讀查驗快取")
        void unsupportedTypeSkipsVerification() {
            service.syncFromProductWrite(syncCommand(true, List.of(input(3704, "HACCP-2026-DP-0047"))));

            verify(applicationRepository).insertSubmittedIfAbsent(any(), any(), any(), any(),
                    eq(3704), eq("HACCP-2026-DP-0047"), any(), anyBooleanArg(),
                    eq(ProductCertificationGovVerifyState.UNSUPPORTED.value()), any(), any(), any());
            verify(govCertificationService, never()).readCachedResult(anyString(), any());
        }

        /**
         * §8.2：同步階段一律不發外部呼叫。同步打政府 API 會在 timeout／IO 例外時打開
         * per-type、TTL 60 秒、Redis 全域共享的 circuit，把緊接著排出去的背景查驗一起封死。
         */
        @Test
        @DisplayName("同步階段只讀快取，絕不呼叫政府 API")
        void syncPathNeverCallsGovApi() {
            service.syncFromProductWrite(syncCommand(true, List.of(input(ORGANIC, CERT_A))));

            verify(govCertificationService, never()).verify(anyString(), any());
            verify(govCertificationService, never()).verifyDetailed(anyString(), any());
            verify(govCertificationService, never()).verifyDetailedForBackground(anyString(), any());
        }
    }

    // ==================== §8.5.1 持有人歸屬（自動核准的第五個條件） ====================

    /**
     * §8.5.1：政府 API 只回答「這個編號存在且有效」，<b>不回答「這張證書屬於誰」</b>。
     *
     * <p>少了這一問的後果 2026-08-27 已實測坐實：真實編號 {@code 1-009-300022}
     * （持有人 {@code Acme Road LLC}、涵蓋松子與堅果）被自動核准到<b>另一家</b>的「文旦禮盒」上
     * 並公開標章 ⇒ 任何賣家只要填一個查得到的他人編號就能取得標章。前四個條件
     * （SUBMITTED／非人工複審／VERIFIED／type 允許自動核准）在那次實測中<b>全部成立</b>，
     * 所以本組每個案例都刻意讓前四條件成立，只讓第五條件變動。
     *
     * <p>🔴 四種不放行的成因一律導向<b>人工審核</b>、不得導向<b>拒絕</b>——拒絕會退回本設計書
     * 要取代的舊行為（§0.2），因此另有 {@link #neverRejectsForAnyReason} 逐一釘住這條不變式。
     */
    @Nested
    @DisplayName("§8.5.1 持有人歸屬比對（自動核准的第五個條件）")
    class HolderAttribution {

        /**
         * 布置一筆「前四條件全部成立」的待審申請，只留第五條件待判。
         *
         * @param holderName 政府查驗回的業者名稱；null／空字串模擬 payload 沒帶名稱
         */
        private ProductCertificationApplicationEntity arrangeEligible(String holderName) {
            ProductCertificationApplicationEntity app = application(HOLDER_APP_ID, ORGANIC, CERT_A,
                    ProductCertificationStatus.SUBMITTED.value(), 0L);
            app.setRequiresManualReview(false);
            app.setGovVerifyState(ProductCertificationGovVerifyState.VERIFIED.value());
            ProductCertificationApplicationEntity.GovVerifyPayload payload =
                    new ProductCertificationApplicationEntity.GovVerifyPayload();
            payload.setCertType(String.valueOf(ORGANIC));
            payload.setCertNo(CERT_A);
            payload.setValidity("VALID");
            payload.setHolderName(holderName);
            app.setGovVerifyPayload(payload);

            when(applicationRepository.findByIdForUpdate(HOLDER_APP_ID)).thenReturn(Optional.of(app));
            when(applicationRepository.casTransitionStatus(eq(HOLDER_APP_ID), eq(0L), anyString(),
                    anyString(), any(), any(), any(), any(), any())).thenReturn(1);
            when(applicationRepository.findApprovedByProductId(PRODUCT_ID)).thenReturn(List.of(
                    application(HOLDER_APP_ID, ORGANIC, CERT_A,
                            ProductCertificationStatus.APPROVED.value(), 1L)));
            when(applicationRepository.markRequiresManualReview(HOLDER_APP_ID)).thenReturn(1);
            return app;
        }

        /** 不放行的共同結果：沒核准、沒重建投影、而且這一筆被標成只能人工裁決。 */
        private void assertParkedForManualReview(boolean approved) {
            assertFalse(approved, "業者名稱歸屬未通過時不得自動核准");
            verify(applicationRepository).markRequiresManualReview(HOLDER_APP_ID);
            verify(applicationRepository, never()).casTransitionStatus(anyLong(), anyLong(), anyString(),
                    anyString(), any(), any(), any(), any(), any());
            verify(productRepository, never()).updateCertificationProjection(anyLong(), any());
        }

        /**
         * 唯一可以放行的情況：{@code available && hasReference && matched}。
         *
         * <p>這條同時是本組的鑑別力保險——若第五個條件被整條拿掉，其他案例會轉紅，
         * 但只有這一條能證明「新加的條件沒有把正常賣家一起擋掉」。
         */
        @Test
        @DisplayName("業者名稱屬於本賣家 ⇒ 照常自動核准（新條件不得誤傷正常賣家）")
        void matchedHolderIsAutoApproved() {
            arrangeEligible(HOLDER_NAME);
            when(sellerGrpcClient.matchCertificateHolder(STORE_ID, HOLDER_NAME))
                    .thenReturn(new SellerGrpcClient.HolderMatch(true, true, "REAL_NAME", true));

            assertTrue(service.approveBySystem(HOLDER_APP_ID), "五個條件全部成立就該核准");

            verify(applicationRepository).casTransitionStatus(eq(HOLDER_APP_ID), eq(0L),
                    eq(ProductCertificationStatus.SUBMITTED.value()),
                    eq(ProductCertificationStatus.APPROVED.value()),
                    any(), any(), any(), any(), any());
            verify(applicationRepository, never()).markRequiresManualReview(anyLong());
            verify(productRepository).updateCertificationProjection(eq(PRODUCT_ID), any());
            assertTrue(capturedEvents().stream().anyMatch(e ->
                            ProductCertificationReviewAction.AUTO_APPROVE.value().equals(e.getAction())),
                    "自動核准仍須寫 AUTO_APPROVE 事件");
        }

        /**
         * 確定不符（{@code hasReference=true, matched=false}）—— 就是 2026-08-27 那次實測的形狀：
         * 證書真實有效，只是持有人是別人。
         */
        @Test
        @DisplayName("HOLDER_MISMATCH：賣家有已通過的名稱但對不上 ⇒ 不核准並轉人工")
        void mismatchedHolderIsParkedForManualReview() {
            arrangeEligible(HOLDER_NAME);
            when(sellerGrpcClient.matchCertificateHolder(STORE_ID, HOLDER_NAME))
                    .thenReturn(new SellerGrpcClient.HolderMatch(false, true, "NONE", true));

            assertParkedForManualReview(service.approveBySystem(HOLDER_APP_ID));
        }

        /**
         * 賣家沒有任何「已通過實名認證」的名稱（{@code hasReference=false}）⇒ 無從判斷。
         * 🔴 未通過審核的 KYC 名稱是賣家自己填的、平台沒驗過，拿它當基準等於讓賣家
         * 填一個假名字去匹配任意一張他人的證書 —— 所以「無從判斷 ≠ 通過」。
         */
        @Test
        @DisplayName("SELLER_HAS_NO_VERIFIED_NAME：賣家無已通過 KYC ⇒ 不核准並轉人工")
        void sellerWithoutVerifiedNameIsParkedForManualReview() {
            arrangeEligible(HOLDER_NAME);
            when(sellerGrpcClient.matchCertificateHolder(STORE_ID, HOLDER_NAME))
                    .thenReturn(new SellerGrpcClient.HolderMatch(false, false, "NONE", true));

            assertParkedForManualReview(service.approveBySystem(HOLDER_APP_ID));
        }

        /**
         * RPC 失敗／逾時 ⇒ <b>fail-closed</b>。這與 {@code SellerGrpcClient} 其他方法
         * 「降級繼續跑」的慣例相反：那些失敗只是少一點資訊，這一支失敗而放行等於把漏洞打開。
         */
        @Test
        @DisplayName("LOOKUP_UNAVAILABLE：比對 RPC 失敗 ⇒ fail-closed，不核准並轉人工")
        void lookupFailureFailsClosed() {
            arrangeEligible(HOLDER_NAME);
            when(sellerGrpcClient.matchCertificateHolder(STORE_ID, HOLDER_NAME))
                    .thenReturn(SellerGrpcClient.HolderMatch.unavailable());

            assertParkedForManualReview(service.approveBySystem(HOLDER_APP_ID));
        }

        /** payload 沒帶業者名稱（3702 就沒有這個欄位）⇒ 無從歸屬，且不該白花一次 RPC。 */
        @Test
        @DisplayName("NO_HOLDER_NAME（null）：不核准並轉人工，且完全不呼叫 seller-service")
        void nullHolderNameSkipsRpcAndParks() {
            arrangeEligible(null);

            assertParkedForManualReview(service.approveBySystem(HOLDER_APP_ID));
            verify(sellerGrpcClient, never()).matchCertificateHolder(anyLong(), any());
        }

        @Test
        @DisplayName("NO_HOLDER_NAME（空字串）：不核准並轉人工，且完全不呼叫 seller-service")
        void blankHolderNameSkipsRpcAndParks() {
            arrangeEligible("   ");

            assertParkedForManualReview(service.approveBySystem(HOLDER_APP_ID));
            verify(sellerGrpcClient, never()).matchCertificateHolder(anyLong(), any());
        }

        static Stream<Arguments> blockingReasons() {
            return Stream.of(
                    Arguments.of("HOLDER_MISMATCH", HOLDER_NAME,
                            new SellerGrpcClient.HolderMatch(false, true, "NONE", true)),
                    Arguments.of("SELLER_HAS_NO_VERIFIED_NAME", HOLDER_NAME,
                            new SellerGrpcClient.HolderMatch(false, false, "NONE", true)),
                    Arguments.of("LOOKUP_UNAVAILABLE", HOLDER_NAME,
                            SellerGrpcClient.HolderMatch.unavailable()),
                    Arguments.of("NO_HOLDER_NAME", null, null));
        }

        /**
         * 🔴 <b>不得退回舊行為</b>（§8.5.1 末段 ／ §0.2）。
         *
         * <p>把歸屬未通過的申請改成 {@code REJECTED} 同樣能堵住漏洞，所以這是最容易被「順手改一下」
         * 改壞的地方；但那正是本設計書要取代的舊行為 —— 賣家一旦被駁回就只能重送，
         * 而重送會再被 §6.3 Step 3 標成人工複審，等於把一個可救的案子變成死結。
         *
         * <p>斷言下在「完全沒有狀態轉換」：既沒有 CAS 到 APPROVED，也沒有任何終態轉換，
         * 連 review_event 都不該有一筆 —— 這一筆自始至終停在 SUBMITTED 等人來看。
         */
        @ParameterizedTest(name = "{0}")
        @MethodSource("blockingReasons")
        @DisplayName("四種不放行的成因，沒有一種可以導向 REJECTED")
        void neverRejectsForAnyReason(String reason, String holderName,
                                      SellerGrpcClient.HolderMatch match) {
            ProductCertificationApplicationEntity app = arrangeEligible(holderName);
            if (match != null) {
                when(sellerGrpcClient.matchCertificateHolder(STORE_ID, holderName)).thenReturn(match);
            }

            assertFalse(service.approveBySystem(HOLDER_APP_ID), reason + " 不得自動核准");

            verify(applicationRepository, never()).casTransitionStatus(anyLong(), anyLong(), anyString(),
                    anyString(), any(), any(), any(), any(), any());
            verify(applicationRepository, never()).transitionToTerminalStatus(any(), anyString(),
                    anyString(), any());
            verify(reviewEventRepository, never()).insert(any());
            assertEquals(ProductCertificationStatus.SUBMITTED.value(), app.getStatus(),
                    reason + " 必須維持 SUBMITTED；改成 REJECTED 就退回設計書要取代的舊行為（§0.2）");
        }
    }

    // ==================== 賣家專用端點（§9.1） ====================

    @Nested
    @DisplayName("§9.1 賣家專用端點")
    class SellerEndpoints {

        @Test
        @DisplayName("非本人商品一律當成不存在（防 IDOR 枚舉）")
        void foreignProductLooksMissing() {
            when(productRepository.findByIdAndStoreId(PRODUCT_ID, STORE_ID)).thenReturn(Optional.empty());

            ProductBusinessException e = assertThrows(ProductBusinessException.class,
                    () -> service.submitForSeller(PRODUCT_ID, STORE_ID, ACCOUNT_ID, "3701", CERT_A, "idem-submit"));
            assertEquals(GrpcResponseCode.PRODUCT_NOT_FOUND.code(), e.code());
        }

        @Test
        @DisplayName("專用端點的併發衝突回 typed 409（3029），不是靜默跳過")
        void dedicatedEndpointReturnsTypedConflict() {
            when(productRepository.findByIdAndStoreId(PRODUCT_ID, STORE_ID))
                    .thenReturn(Optional.of(productWithProjection()));
            when(applicationRepository.insertSubmittedIfAbsent(any(), any(), any(), any(), any(), any(),
                    any(), anyBooleanArg(), any(), any(), any(), any())).thenReturn(0);

            ProductBusinessException e = assertThrows(ProductBusinessException.class,
                    () -> service.submitForSeller(PRODUCT_ID, STORE_ID, ACCOUNT_ID, "3701", CERT_A, "idem-submit"));
            assertEquals(GrpcResponseCode.CERT_APPLICATION_DUPLICATE_PENDING.code(), e.code());
        }

        @Test
        @DisplayName("撤回別人的申請 ⇒ 當成不存在（3025），不得洩漏它存在")
        void withdrawingForeignApplicationLooksMissing() {
            ProductCertificationApplicationEntity other = application(55L, ORGANIC, CERT_A,
                    ProductCertificationStatus.APPROVED.value(), 1L);
            other.setStoreId(999999L);
            when(applicationRepository.findByIdForUpdate(55L)).thenReturn(Optional.of(other));

            ProductBusinessException e = assertThrows(ProductBusinessException.class,
                    () -> service.withdrawForSeller(PRODUCT_ID, 55L, STORE_ID, ACCOUNT_ID, 1L, "idem-wd"));
            assertEquals(GrpcResponseCode.CERT_APPLICATION_NOT_FOUND.code(), e.code());
        }
    }

    // ==================== §9.3 重新查驗 ====================

    @Nested
    @DisplayName("§9.3 ADM 重新查驗：只查驗、不改 status")
    class Reverify {

        @Test
        @DisplayName("查驗通過也不自動核准，只回寫查驗欄位")
        void reverifyNeverChangesStatus() {
            ProductCertificationApplicationEntity app = application(99L, ORGANIC, CERT_A,
                    ProductCertificationStatus.SUBMITTED.value(), 1L);
            when(applicationRepository.findById(99L)).thenReturn(Optional.of(app));
            when(govCertificationService.verifyDetailedForceRefresh(CERT_A, CertificationType.ORGANIC))
                    .thenReturn(validResult(CERT_A));
            when(applicationRepository.casUpdateGovVerifyResult(eq(99L), eq(1L), anyString(), any(), any()))
                    .thenReturn(1);

            service.reverifyForAdmin(99L, 999L, "idem-reverify");

            verify(applicationRepository).casUpdateGovVerifyResult(eq(99L), eq(1L),
                    eq(ProductCertificationGovVerifyState.VERIFIED.value()), any(), any());
            verify(applicationRepository, never()).casTransitionStatus(anyLong(), anyLong(), anyString(),
                    anyString(), any(), any(), any(), any(), any());
        }

        /**
         * §8.4 防線 1：金鑰失效／配額用盡時政府 API 回 HTTP 200 且完全沒有 Data 欄。
         * 落成 NOT_FOUND ⇒ 全站所有認證編號都會被判成假的，而且沒有任何錯誤訊號。
         */
        @Test
        @DisplayName("政府 API 不可用 ⇒ 落 UNAVAILABLE，絕不得落成 NOT_FOUND")
        void govUnavailableNeverBecomesNotFound() {
            ProductCertificationApplicationEntity app = application(99L, ORGANIC, CERT_A,
                    ProductCertificationStatus.SUBMITTED.value(), 1L);
            when(applicationRepository.findById(99L)).thenReturn(Optional.of(app));
            when(govCertificationService.verifyDetailedForceRefresh(CERT_A, CertificationType.ORGANIC))
                    .thenThrow(new GovCertificationService.GovApiUnavailableException("API_ID&API_KEY不符"));
            when(applicationRepository.casUpdateGovVerifyResult(anyLong(), anyLong(), anyString(), any(), any()))
                    .thenReturn(1);

            service.reverifyForAdmin(99L, 999L, "idem-reverify");

            verify(applicationRepository).casUpdateGovVerifyResult(eq(99L), eq(1L),
                    eq(ProductCertificationGovVerifyState.UNAVAILABLE.value()), any(), any());
        }

        @Test
        @DisplayName("3704 等不支援線上驗證的類型 ⇒ 落 UNSUPPORTED，不打外部 API")
        void unsupportedTypeSkipsExternalCall() {
            ProductCertificationApplicationEntity app = application(99L, 3704, "HACCP-2026-DP-0047",
                    ProductCertificationStatus.SUBMITTED.value(), 1L);
            when(applicationRepository.findById(99L)).thenReturn(Optional.of(app));
            when(applicationRepository.casUpdateGovVerifyResult(anyLong(), anyLong(), anyString(), any(), any()))
                    .thenReturn(1);

            service.reverifyForAdmin(99L, 999L, "idem-reverify");

            verify(govCertificationService, never()).verifyDetailedForBackground(anyString(), any());
            verify(govCertificationService, never()).verifyDetailedForceRefresh(anyString(), any());
            verify(applicationRepository).casUpdateGovVerifyResult(eq(99L), eq(1L),
                    eq(ProductCertificationGovVerifyState.UNSUPPORTED.value()), any(), any());
        }

        /**
         * 🔴 本節的重點測試：<b>ADM「重新查驗」不得讀結果快取</b>（§9.3）。
         *
         * <p>dev 實測的病徵：對一筆已查驗過的申請按「重新查驗」，{@code version} 有 bump、
         * {@code updated_at} 有動，但 {@code gov_verified_at} 與 payload 的 {@code queriedAt}
         * 紋風不動——因為它走的是會讀結果快取的背景路徑，而 {@code VALID} 的 TTL 是
         * <b>7 天</b>。⇒ 賣家在申請之後才補登證書時（這正是這顆按鈕存在的第一個理由），
         * ADM 七天內按幾次都拿不到新答案，<b>畫面上也沒有任何線索</b>。
         *
         * <p>把 {@code runVerify(applicationId, true)} 改回 {@code false}、或讓
         * {@code runGovVerify} 忽略 {@code forceRefresh}，這支就會轉紅。
         */
        @Test
        @DisplayName("🔴 reverify 走繞過結果快取的路徑，不得走會讀快取的背景路徑")
        void reverifyBypassesResultCache() {
            ProductCertificationApplicationEntity app = application(99L, ORGANIC, CERT_A,
                    ProductCertificationStatus.SUBMITTED.value(), 1L);
            when(applicationRepository.findById(99L)).thenReturn(Optional.of(app));
            when(govCertificationService.verifyDetailedForceRefresh(CERT_A, CertificationType.ORGANIC))
                    .thenReturn(validResult(CERT_A));
            when(applicationRepository.casUpdateGovVerifyResult(anyLong(), anyLong(), anyString(), any(), any()))
                    .thenReturn(1);

            service.reverifyForAdmin(99L, 999L, "idem-reverify");

            verify(govCertificationService).verifyDetailedForceRefresh(CERT_A, CertificationType.ORGANIC);
            verify(govCertificationService, never()).verifyDetailedForBackground(anyString(), any());
        }

        /**
         * 反向：MQ 背景查驗<b>仍然讀快取</b>，不得跟著一起繞。
         *
         * <p>背景查驗是「第一次補課」，命中快取正是我們要的省呼叫；把它也改成繞過，
         * 等於每一筆商品建立都必打一次政府 API，會把每日配額燒在不需要新鮮度的路徑上。
         * 這支守的是「修 A 不要順手把 B 也改掉」。
         */
        @Test
        @DisplayName("反向：MQ 背景查驗仍走讀快取的路徑（配額只花在 ADM 手動觸發上）")
        void backgroundVerifyStillUsesCache() {
            ProductCertificationApplicationEntity app = application(99L, ORGANIC, CERT_A,
                    ProductCertificationStatus.SUBMITTED.value(), 1L);
            when(applicationRepository.findById(99L)).thenReturn(Optional.of(app));
            when(govCertificationService.verifyDetailedForBackground(CERT_A, CertificationType.ORGANIC))
                    .thenReturn(validResult(CERT_A));
            when(applicationRepository.casUpdateGovVerifyResult(anyLong(), anyLong(), anyString(), any(), any()))
                    .thenReturn(1);

            service.runBackgroundVerify(99L);

            verify(govCertificationService).verifyDetailedForBackground(CERT_A, CertificationType.ORGANIC);
            verify(govCertificationService, never()).verifyDetailedForceRefresh(anyString(), any());
        }

        // ---------- 冷卻護欄（配額保護） ----------

        /**
         * 🔴 繞過結果快取之後，每按一次「重新查驗」都真的打農業部並消耗每日配額，
         * 而這支端點<b>沒有</b> {@code expectedVersion} 可以擋、冪等鍵又刻意每次點擊都換新
         * ⇒ 沒有任何東西擋得住連點。冷卻窗是最低限度的護欄。
         *
         * <p>斷言「<b>完全沒有打外部</b>」而不只是「有拋例外」：拋了例外但已經打過，
         * 配額照樣花掉，那等於沒守到。
         */
        @Test
        @DisplayName("🔴 冷卻窗內重複觸發 ⇒ 回 3030，且完全不打政府 API")
        void cooldownBlocksRepeatedReverify() {
            ProductCertificationApplicationEntity app = application(99L, ORGANIC, CERT_A,
                    ProductCertificationStatus.SUBMITTED.value(), 1L);
            app.setGovVerifiedAt(DateTimeUtils.nowUtc().minusSeconds(10));
            when(applicationRepository.findById(99L)).thenReturn(Optional.of(app));

            ProductBusinessException e = assertThrows(ProductBusinessException.class,
                    () -> service.reverifyForAdmin(99L, 999L, "idem-reverify"));

            assertEquals(GrpcResponseCode.CERT_REVERIFY_COOLDOWN.code(), e.code());
            verify(govCertificationService, never()).verifyDetailedForceRefresh(anyString(), any());
            verify(govCertificationService, never()).verifyDetailedForBackground(anyString(), any());
            verify(applicationRepository, never())
                    .casUpdateGovVerifyResult(anyLong(), anyLong(), anyString(), any(), any());
        }

        /**
         * ADM 必須看得到<b>為什麼</b>被擋，不是靜默無反應。訊息要同時帶「多久以前查的」
         * 與「還要等多久」——只說「請稍後再試」等於叫人盲目重按。
         */
        @Test
        @DisplayName("被擋時訊息講得出「幾秒前查的」與「還要等多久」")
        void cooldownMessageTellsAdminWhy() {
            ProductCertificationApplicationEntity app = application(99L, ORGANIC, CERT_A,
                    ProductCertificationStatus.SUBMITTED.value(), 1L);
            app.setGovVerifiedAt(DateTimeUtils.nowUtc().minusSeconds(10));
            when(applicationRepository.findById(99L)).thenReturn(Optional.of(app));

            ProductBusinessException e = assertThrows(ProductBusinessException.class,
                    () -> service.reverifyForAdmin(99L, 999L, "idem-reverify"));

            assertTrue(e.getMessage().contains("10 秒前"), "實際=" + e.getMessage());
            assertTrue(e.getMessage().contains("50 秒後"), "實際=" + e.getMessage());
            assertTrue(e.getMessage().contains("配額"),
                    "要講出為什麼有這個限制，否則管理員只會覺得系統壞了：" + e.getMessage());
        }

        /**
         * <b>elapsed 為 0 是連按兩下的常態</b>，不是罕見邊界——上一句測試涵蓋的 10 秒
         * 反而少見。最容易撞到的那一次，訊息不該是「剛在 0 秒前查驗過」這種讀起來
         * 像字串壞掉的句子。
         */
        @Test
        @DisplayName("剛查完就再按（elapsed=0）⇒ 訊息說「剛剛」，不說「0 秒前」")
        void cooldownMessageReadsNaturallyAtZeroElapsed() {
            ProductCertificationApplicationEntity app = application(99L, ORGANIC, CERT_A,
                    ProductCertificationStatus.SUBMITTED.value(), 1L);
            app.setGovVerifiedAt(DateTimeUtils.nowUtc());
            when(applicationRepository.findById(99L)).thenReturn(Optional.of(app));

            ProductBusinessException e = assertThrows(ProductBusinessException.class,
                    () -> service.reverifyForAdmin(99L, 999L, "idem-reverify-zero"));

            assertTrue(e.getMessage().contains("剛剛查驗過"), "實際=" + e.getMessage());
            assertFalse(e.getMessage().contains("0 秒前"),
                    "「0 秒前」是壞掉的字串感，不該出現：" + e.getMessage());
            assertTrue(e.getMessage().contains("60 秒後"), "還要等多久仍要講出來：" + e.getMessage());
        }

        /**
         * 🔴 <b>legacy 列的首次查驗不得被擋</b>（§15.3）。那些 migration 產生的列沒有 MQ 事件，
         * §8.3 又明令禁止掃 {@code PENDING} 的排程 ⇒「重新查驗」是它們<b>唯一</b>的觸發方式。
         * 擋掉首次查驗＝那批列永遠停在待查驗，而且沒有任何人會發現。
         */
        @Test
        @DisplayName("🔴 gov_verified_at 為 NULL（legacy 列首次查驗）不套用冷卻")
        void cooldownDoesNotBlockLegacyFirstVerify() {
            ProductCertificationApplicationEntity app = application(99L, ORGANIC, CERT_A,
                    ProductCertificationStatus.SUBMITTED.value(), 1L);
            app.setGovVerifiedAt(null);
            when(applicationRepository.findById(99L)).thenReturn(Optional.of(app));
            when(govCertificationService.verifyDetailedForceRefresh(CERT_A, CertificationType.ORGANIC))
                    .thenReturn(validResult(CERT_A));
            when(applicationRepository.casUpdateGovVerifyResult(anyLong(), anyLong(), anyString(), any(), any()))
                    .thenReturn(1);

            service.reverifyForAdmin(99L, 999L, "idem-reverify");

            verify(govCertificationService).verifyDetailedForceRefresh(CERT_A, CertificationType.ORGANIC);
        }

        @Test
        @DisplayName("冷卻窗過了就放行（不是永久鎖住）")
        void cooldownExpiresAndAllowsAgain() {
            ProductCertificationApplicationEntity app = application(99L, ORGANIC, CERT_A,
                    ProductCertificationStatus.SUBMITTED.value(), 1L);
            app.setGovVerifiedAt(DateTimeUtils.nowUtc().minusSeconds(
                    ProductCertificationApplicationService.REVERIFY_COOLDOWN_SECONDS + 1));
            when(applicationRepository.findById(99L)).thenReturn(Optional.of(app));
            when(govCertificationService.verifyDetailedForceRefresh(CERT_A, CertificationType.ORGANIC))
                    .thenReturn(validResult(CERT_A));
            when(applicationRepository.casUpdateGovVerifyResult(anyLong(), anyLong(), anyString(), any(), any()))
                    .thenReturn(1);

            service.reverifyForAdmin(99L, 999L, "idem-reverify");

            verify(govCertificationService).verifyDetailedForceRefresh(CERT_A, CertificationType.ORGANIC);
        }

        /**
         * 🔴 「多位管理員同時看同一筆」＝ 第一個呼叫<b>還在飛</b>的時候第二個進來。
         * 這段窗口最長 90 秒（背景 read-timeout），而在這段期間 {@code gov_verified_at}
         * <b>還沒被更新</b> ⇒ 光靠時間差那一關會直接放行。租約是唯一擋得住它的東西。
         */
        @Test
        @DisplayName("🔴 租約被別人持有（另一個查驗還在飛）⇒ 擋下，且不打政府 API")
        void leaseHeldByConcurrentAdminBlocks() {
            ProductCertificationApplicationEntity app = application(99L, ORGANIC, CERT_A,
                    ProductCertificationStatus.SUBMITTED.value(), 1L);
            // 上一次查驗是很久以前 ⇒ 時間差那一關會放行，只剩租約擋得住
            app.setGovVerifiedAt(DateTimeUtils.nowUtc().minusDays(3));
            when(applicationRepository.findById(99L)).thenReturn(Optional.of(app));
            when(redisValueOps.setIfAbsent(anyString(), anyString(), anyLong(), any())).thenReturn(false);

            ProductBusinessException e = assertThrows(ProductBusinessException.class,
                    () -> service.reverifyForAdmin(99L, 999L, "idem-reverify"));

            assertEquals(GrpcResponseCode.CERT_REVERIFY_COOLDOWN.code(), e.code());
            assertTrue(e.getMessage().contains("正在進行中"), "實際=" + e.getMessage());
            verify(govCertificationService, never()).verifyDetailedForceRefresh(anyString(), any());
        }

        /**
         * 租約用完必須還（§29：lease 不是永久旗標）。不還的話下一次要等 TTL（120 秒）
         * 才解得開，而使用者看到的訊息會說「60 秒後可再試」——說一套做一套。
         */
        @Test
        @DisplayName("查驗結束會釋放租約；即使查驗拋例外也要還")
        void leaseIsAlwaysReleased() {
            ProductCertificationApplicationEntity app = application(99L, ORGANIC, CERT_A,
                    ProductCertificationStatus.SUBMITTED.value(), 1L);
            app.setGovVerifiedAt(null);
            when(applicationRepository.findById(99L)).thenReturn(Optional.of(app));
            when(govCertificationService.verifyDetailedForceRefresh(CERT_A, CertificationType.ORGANIC))
                    .thenReturn(validResult(CERT_A));
            when(applicationRepository.casUpdateGovVerifyResult(anyLong(), anyLong(), anyString(), any(), any()))
                    .thenReturn(1);

            service.reverifyForAdmin(99L, 999L, "idem-reverify");
            verify(redisTemplate).delete("cert:reverify:lease:99");

            // 例外路徑。⚠️ 不能讓 verifyDetailedForceRefresh 拋——runGovVerify 會把
            // RuntimeException 收斂成 UNAVAILABLE（§8.4 防線 1 要求的行為），根本傳不出來。
            // 改讓回寫那一句炸，它在 runVerify 裡沒有 catch，才真的會穿出 try 區塊。
            org.mockito.Mockito.clearInvocations(redisTemplate);
            when(applicationRepository.casUpdateGovVerifyResult(anyLong(), anyLong(), anyString(), any(), any()))
                    .thenThrow(new IllegalStateException("boom"));
            assertThrows(IllegalStateException.class,
                    () -> service.reverifyForAdmin(99L, 999L, "idem-reverify-2"));
            verify(redisTemplate).delete("cert:reverify:lease:99");
        }

        /**
         * Redis 失聯不得讓「重新查驗」整個不能按：這是配額護欄不是正確性不變式，
         * 而 {@code gov_verified_at} 那一關不依賴 Redis、仍然有效（02 §2.7.3 降級契約）。
         */
        @Test
        @DisplayName("Redis 失聯 ⇒ 租約 fail-open，功能不中斷")
        void redisOutageFailsOpen() {
            ProductCertificationApplicationEntity app = application(99L, ORGANIC, CERT_A,
                    ProductCertificationStatus.SUBMITTED.value(), 1L);
            app.setGovVerifiedAt(null);
            when(applicationRepository.findById(99L)).thenReturn(Optional.of(app));
            when(redisValueOps.setIfAbsent(anyString(), anyString(), anyLong(), any()))
                    .thenThrow(new org.springframework.dao.QueryTimeoutException("redis down"));
            when(govCertificationService.verifyDetailedForceRefresh(CERT_A, CertificationType.ORGANIC))
                    .thenReturn(validResult(CERT_A));
            when(applicationRepository.casUpdateGovVerifyResult(anyLong(), anyLong(), anyString(), any(), any()))
                    .thenReturn(1);

            service.reverifyForAdmin(99L, 999L, "idem-reverify");

            verify(govCertificationService).verifyDetailedForceRefresh(CERT_A, CertificationType.ORGANIC);
        }

        /**
         * 反向：MQ 背景查驗<b>不受冷卻窗拘束</b>。它不是使用者觸發的、也讀快取，
         * 把冷卻套上去會讓「商品建立後的第一次補課」在重試時被自己擋掉。
         */
        @Test
        @DisplayName("反向：MQ 背景查驗不套用冷卻（它讀快取，本來就不燒配額）")
        void backgroundVerifyNotSubjectToCooldown() {
            ProductCertificationApplicationEntity app = application(99L, ORGANIC, CERT_A,
                    ProductCertificationStatus.SUBMITTED.value(), 1L);
            app.setGovVerifiedAt(DateTimeUtils.nowUtc().minusSeconds(1));
            when(applicationRepository.findById(99L)).thenReturn(Optional.of(app));
            when(govCertificationService.verifyDetailedForBackground(CERT_A, CertificationType.ORGANIC))
                    .thenReturn(validResult(CERT_A));
            when(applicationRepository.casUpdateGovVerifyResult(anyLong(), anyLong(), anyString(), any(), any()))
                    .thenReturn(1);

            service.runBackgroundVerify(99L);

            verify(govCertificationService).verifyDetailedForBackground(CERT_A, CertificationType.ORGANIC);
        }

        /**
         * {@code gov_verified_at} 是「這個答案是什麼時候問到的」。回寫時若沿用快取裡的舊
         * {@code queriedAt}，畫面上的查驗時間就會停在七天前——那正是 dev 實測看到的現象。
         * 繞過快取之後，回寫的時間必須跟著這次查詢走。
         */
        @Test
        @DisplayName("回寫的 gov_verified_at 來自這一次查詢的 queriedAt，不是舊快取的時間")
        void reverifyWritesFreshVerifiedAt() {
            ProductCertificationApplicationEntity app = application(99L, ORGANIC, CERT_A,
                    ProductCertificationStatus.SUBMITTED.value(), 1L);
            when(applicationRepository.findById(99L)).thenReturn(Optional.of(app));
            when(govCertificationService.verifyDetailedForceRefresh(CERT_A, CertificationType.ORGANIC))
                    .thenReturn(new GovCertificationService.GovVerifyResult(
                            "3701", CERT_A, "2026-08-27T09:30:00Z", true,
                            GovCertificationService.Validity.VALID, "通過", "2028-10-14",
                            "○○農場", "台南市", "○○驗證公司", "小松菜", 1, null));
            when(applicationRepository.casUpdateGovVerifyResult(anyLong(), anyLong(), anyString(), any(), any()))
                    .thenReturn(1);

            service.reverifyForAdmin(99L, 999L, "idem-reverify");

            ArgumentCaptor<java.time.LocalDateTime> verifiedAt =
                    ArgumentCaptor.forClass(java.time.LocalDateTime.class);
            verify(applicationRepository).casUpdateGovVerifyResult(eq(99L), eq(1L),
                    eq(ProductCertificationGovVerifyState.VERIFIED.value()), any(), verifiedAt.capture());
            assertEquals(java.time.LocalDateTime.of(2026, 8, 27, 9, 30, 0), verifiedAt.getValue());
        }
    }


    // ==================== §4.1 來源與送出者 ====================

    @Nested
    @DisplayName("§4.1 source 與 submitted_by_account_id")
    class WriteSourceAndSubmitter {

        /**
         * 賣家自助與 ADM 代行是<b>不同的人</b>做同一件事。記成同一個 source，
         * 稽核就永遠分不出「這張標章是店主自己掛的，還是平台代行掛上去的」。
         */
        @Test
        @DisplayName("代行寫入 ⇒ source = DELEGATED_ADMIN，稽核 actor_role = ADMIN")
        void delegatedWriteIsRecordedAsDelegatedAdmin() {
            service.syncFromProductWrite(
                    new ProductCertificationApplicationService.CertificationSyncCommand(
                            PRODUCT_ID, STORE_ID, null,
                            ProductCertificationSource.DELEGATED_ADMIN.value(), true,
                            List.of(input(ORGANIC, CERT_A))));

            verify(applicationRepository).insertSubmittedIfAbsent(any(), any(), any(), any(),
                    eq(ORGANIC), eq(CERT_A),
                    eq(ProductCertificationSource.DELEGATED_ADMIN.value()),
                    anyBooleanArg(), any(), any(), any(), any());
            assertEquals("ADMIN", capturedEvents().get(0).getActorRole());
        }

        /**
         * 🔴 {@code store_id} 是<b>營運鍵</b>、{@code account_id} 是<b>登入 principal</b>，
         * 兩者只在「首任 grandfather」時恰好相等 ⇒ 拿 storeId 頂替會讓 dev 與所有 fixture 全綠，
         * <b>只有 re-key 過的店家錯，而且沒有任何告警</b>（CLAUDE.md §6）。
         */
        @Test
        @DisplayName("submitted_by_account_id 絕不可被 storeId 頂替")
        void submitterIsNeverSubstitutedByStoreId() {
            service.syncFromProductWrite(syncCommand(true, List.of(input(ORGANIC, CERT_A))));

            ArgumentCaptor<Long> submitter = ArgumentCaptor.forClass(Long.class);
            verify(applicationRepository).insertSubmittedIfAbsent(any(), any(), any(),
                    submitter.capture(), any(), any(), any(), anyBooleanArg(),
                    any(), any(), any(), any());
            assertFalse(STORE_ID.equals(submitter.getValue()),
                    "storeId 是營運鍵，不是登入 principal —— 頂替只有 re-key 過的店家會錯且無告警");
        }

        /** 匯入路徑的來源必須是 PRODUCT_IMPORT，稽核 actor 記 SYSTEM（沒有具體的人在按按鈕）。 */
        @Test
        @DisplayName("匯入寫入 ⇒ source = PRODUCT_IMPORT，actor_role = SYSTEM")
        void importWriteIsRecordedAsProductImport() {
            service.syncFromProductWrite(
                    new ProductCertificationApplicationService.CertificationSyncCommand(
                            PRODUCT_ID, STORE_ID, null,
                            ProductCertificationSource.PRODUCT_IMPORT.value(), false,
                            List.of(input(ORGANIC, CERT_A))));

            verify(applicationRepository).insertSubmittedIfAbsent(any(), any(), any(), any(),
                    eq(ORGANIC), eq(CERT_A),
                    eq(ProductCertificationSource.PRODUCT_IMPORT.value()),
                    anyBooleanArg(), any(), any(), any(), any());
            assertEquals("SYSTEM", capturedEvents().get(0).getActorRole());
        }
    }

    // ==================== §16.3 冪等 ====================

    @Nested
    @DisplayName("§16.3 冪等：idempotencyKey 必須真的被消費")
    class Idempotency {

        /**
         * reverify <b>沒有</b> {@code expectedVersion} 可以擋重送，冪等鍵是它唯一的保護。
         * 不消費這個欄位 ⇒ ADM 連點兩次就打兩次政府 API，白白消耗每日配額。
         */
        @Test
        @DisplayName("reverify 重放 ⇒ 不再打政府 API")
        void reverifyReplayDoesNotCallGovApiAgain() {
            when(idempotencyService.acquireLock(eq("idem-reverify"), any(), anyString(), any(), any()))
                    .thenReturn(Optional.of(new CertificationApplicationRefDTO("99")));
            when(applicationRepository.findById(99L)).thenReturn(Optional.of(
                    application(99L, ORGANIC, CERT_A, ProductCertificationStatus.SUBMITTED.value(), 1L)));

            service.reverifyForAdmin(99L, 999L, "idem-reverify");

            verify(govCertificationService, never()).verifyDetailedForBackground(anyString(), any());
            verify(applicationRepository, never()).casUpdateGovVerifyResult(anyLong(), anyLong(),
                    anyString(), any(), any());
        }

        /**
         * 🔴 重放要<b>重讀當前狀態</b>，不是回放寫入當下的快照。
         * 冪等記錄存的是申請 ID，而申請在那之後可能已被裁決／被換號 SUPERSEDE
         * ——回放快照等於回一個過時狀態而呼叫端無從分辨。
         */
        @Test
        @DisplayName("核准重放 ⇒ 重讀當前狀態，且不重複執行狀態轉移")
        void approveReplayRereadsCurrentStateAndDoesNotRerun() {
            when(idempotencyService.acquireLock(eq("idem-approve"), any(), anyString(), any(), any()))
                    .thenReturn(Optional.of(new CertificationApplicationRefDTO("22")));
            // 第一次核准之後，這筆已經被同 type 的更新號碼 SUPERSEDE 掉了
            when(applicationRepository.findById(22L)).thenReturn(Optional.of(
                    application(22L, ORGANIC, CERT_B, ProductCertificationStatus.SUPERSEDED.value(), 4L)));

            ProductCertificationApplicationEntity replayed =
                    service.approveForAdmin(22L, 0L, 999L, null, null, "idem-approve");

            assertEquals(ProductCertificationStatus.SUPERSEDED.value(), replayed.getStatus(),
                    "重放必須回當前狀態；回快照會讓呼叫端以為它還是 APPROVED");
            verify(applicationRepository, never()).findByIdForUpdate(anyLong());
            verify(applicationRepository, never()).casTransitionStatus(anyLong(), anyLong(), anyString(),
                    anyString(), any(), any(), any(), any(), any());
        }

        @Test
        @DisplayName("成功後要標記冪等成功，存的是申請 ID 不是整份快照")
        void successMarksIdempotencyWithApplicationId() {
            ProductCertificationApplicationEntity pending = application(22L, ORGANIC, CERT_B,
                    ProductCertificationStatus.SUBMITTED.value(), 0L);
            when(applicationRepository.findByIdForUpdate(22L)).thenReturn(Optional.of(pending));
            when(applicationRepository.casTransitionStatus(eq(22L), eq(0L), anyString(), anyString(),
                    any(), any(), any(), any(), any())).thenReturn(1);
            when(applicationRepository.findById(22L)).thenReturn(Optional.of(pending));

            service.approveForAdmin(22L, 0L, 999L, null, null, "idem-approve");

            verify(idempotencyService).markSucceeded(eq("idem-approve"), eq(999L),
                    eq(new CertificationApplicationRefDTO("22")));
        }

        @Test
        @DisplayName("失敗要標記冪等失敗（否則同一把 key 永遠卡在 IN_PROGRESS）")
        void failureMarksIdempotencyFailed() {
            when(applicationRepository.findByIdForUpdate(22L)).thenReturn(Optional.of(
                    application(22L, ORGANIC, CERT_B, ProductCertificationStatus.SUBMITTED.value(), 5L)));

            assertThrows(ProductBusinessException.class,
                    () -> service.approveForAdmin(22L, 2L, 999L, null, null, "idem-approve"));

            verify(idempotencyService).markFailed("idem-approve", 999L);
        }
    }

    // ==================== fixtures ====================

    private ProductCertificationApplicationService.CertificationSyncCommand syncCommand(
            boolean updateCertifications,
            List<ProductCertificationApplicationService.CertificationInput> certs) {
        // 🔴 submittedByAccountId 傳 null：商品寫入路徑沒有 account_id 可用，而 storeId 是營運鍵
        //    ——ProductService 也是這樣傳的，fixture 若傳 ACCOUNT_ID 會讓測試對頂替行為失明
        return new ProductCertificationApplicationService.CertificationSyncCommand(
                PRODUCT_ID, STORE_ID, null,
                ProductCertificationSource.SELLER.value(), updateCertifications, certs);
    }

    private static ProductCertificationApplicationService.CertificationInput input(int type, String certNo) {
        return new ProductCertificationApplicationService.CertificationInput(String.valueOf(type), certNo);
    }

    private static ProductCertificationApplicationService.CertificationInput input(String type, String certNo) {
        return new ProductCertificationApplicationService.CertificationInput(type, certNo);
    }

    private static ProductCertificationApplicationEntity application(Long id, int type, String certNo,
                                                                      String status, Long version) {
        ProductCertificationApplicationEntity entity = new ProductCertificationApplicationEntity();
        entity.setId(id);
        entity.setProductId(PRODUCT_ID);
        entity.setStoreId(STORE_ID);
        entity.setSubmittedByAccountId(ACCOUNT_ID);
        entity.setCertificationType(type);
        entity.setCertificateNo(certNo);
        entity.setStatus(status);
        entity.setSource(ProductCertificationSource.SELLER.value());
        entity.setRequiresManualReview(false);
        entity.setGovVerifyState(ProductCertificationGovVerifyState.PENDING.value());
        entity.setVersion(version);
        entity.setSubmittedAt(LocalDateTime.now());
        return entity;
    }

    private static ProductEntity productWithProjection() {
        ProductEntity product = new ProductEntity();
        product.setId(PRODUCT_ID);
        product.setStoreId(STORE_ID);
        product.setVersion(1L);
        product.setCertifications(new ArrayList<>());
        return product;
    }

    /** 起始投影已含一筆已核准認證的商品。 */
    private static ProductEntity productWithProjection(int type, String certNo) {
        ProductEntity product = productWithProjection();
        ProductEntity.CertificationItem item = new ProductEntity.CertificationItem();
        item.setName(String.valueOf(type));
        item.setCertNo(certNo);
        product.setCertifications(new ArrayList<>(List.of(item)));
        return product;
    }

    private static GovCertificationService.GovVerifyResult validResult(String certNo) {
        return new GovCertificationService.GovVerifyResult(
                "3701", certNo, "2026-08-26T04:12:07Z", true,
                GovCertificationService.Validity.VALID, "通過", "2028-10-14",
                "○○農場", "台南市", "○○驗證公司", "小松菜", 1, null);
    }

    private List<ProductCertificationReviewEventEntity> capturedEvents() {
        ArgumentCaptor<ProductCertificationReviewEventEntity> captor =
                ArgumentCaptor.forClass(ProductCertificationReviewEventEntity.class);
        verify(reviewEventRepository, org.mockito.Mockito.atLeastOnce()).insert(captor.capture());
        List<ProductCertificationReviewEventEntity> events = captor.getAllValues();
        assertNotNull(events);
        return events;
    }

    @SuppressWarnings("unchecked")
    private static ArgumentCaptor<List<ProductEntity.CertificationItem>> projectionCaptor() {
        return ArgumentCaptor.forClass((Class<List<ProductEntity.CertificationItem>>) (Class<?>) List.class);
    }

}
