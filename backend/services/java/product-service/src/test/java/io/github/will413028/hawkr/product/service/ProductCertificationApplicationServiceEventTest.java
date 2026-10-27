package io.github.will413028.hawkr.product.service;

import io.github.will413028.hawkr.common.utils.TsidUtil;
import io.github.will413028.hawkr.common.utils.code.ProductCertificationGovVerifyState;
import io.github.will413028.hawkr.common.utils.code.ProductCertificationSource;
import io.github.will413028.hawkr.common.utils.code.ProductCertificationStatus;
import io.github.will413028.hawkr.common.utils.code.ProductEventType;
import io.github.will413028.hawkr.product.entity.ProductCertificationApplicationEntity;
import io.github.will413028.hawkr.product.entity.ProductEntity;
import io.github.will413028.hawkr.product.grpc.client.SellerGrpcClient;
import io.github.will413028.hawkr.product.mq.ProductEventPublisher;
import io.github.will413028.hawkr.product.mq.dto.ProductCertificationDecidedPayload;
import io.github.will413028.hawkr.product.mq.dto.ProductCertificationVerifyRequestedPayload;
import io.github.will413028.hawkr.product.repository.ProductCertificationApplicationRepository;
import io.github.will413028.hawkr.product.repository.ProductCertificationReviewEventRepository;
import io.github.will413028.hawkr.product.repository.ProductRepository;
import io.github.will413028.hawkr.seller.grpc.SellerMeta;
import com.fasterxml.jackson.databind.ObjectMapper;
import org.junit.jupiter.api.BeforeAll;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.DisplayName;
import org.junit.jupiter.api.Nested;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.api.extension.ExtendWith;
import org.mockito.ArgumentCaptor;
import org.mockito.Mock;
import org.mockito.junit.jupiter.MockitoExtension;
import org.mockito.junit.jupiter.MockitoSettings;
import org.mockito.quality.Strictness;

import java.time.LocalDateTime;
import java.util.List;
import java.util.Map;
import java.util.Optional;

import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.junit.jupiter.api.Assertions.assertFalse;
import static org.junit.jupiter.api.Assertions.assertThrows;
import static org.junit.jupiter.api.Assertions.assertTrue;
import static org.mockito.ArgumentMatchers.any;
import static org.mockito.ArgumentMatchers.anyBoolean;
import static org.mockito.ArgumentMatchers.anyInt;
import static org.mockito.ArgumentMatchers.anyLong;
import static org.mockito.ArgumentMatchers.anyString;
import static org.mockito.Mockito.never;
import static org.mockito.Mockito.verify;
import static org.mockito.Mockito.verifyNoInteractions;
import static org.mockito.Mockito.when;

/**
 * {@link ProductCertificationApplicationService} 的 <b>MQ 事件契約</b>
 * （19 號設計書《農業認證驗證》§12、36 §3.6h.1，#2755）。
 *
 * <p>本檔只管四件事，每一件都是「零訊號失效」——不發／發錯收件人／多發，
 * 系統其他部分全部正常，只有賣家收不到（或收到不該收的）通知：
 * <ol>
 *   <li><b>推播策略三情境</b>（§12.3）：同步快取命中的即刻核准<b>不發</b>、
 *       背景補課核准<b>要發</b>、人工裁決<b>要發</b>。</li>
 *   <li><b>{@code sellerAccountId} fail-closed</b>：解析失敗時<b>絕不發出缺收件人的 payload</b>。</li>
 *   <li><b>收件人是 account_id 不是 store_id</b>：首任店家兩者相等，頂替在 fixture 全綠。</li>
 *   <li><b>{@code internalNote} 絕不進 payload</b>（僅 ADM 可見）。</li>
 * </ol>
 */
@ExtendWith(MockitoExtension.class)
@MockitoSettings(strictness = Strictness.LENIENT)
class ProductCertificationApplicationServiceEventTest {

    private static final Long PRODUCT_ID = 843856384063471755L;
    private static final String PRODUCT_NAME = "有機小松菜";
    private static final Long STORE_ID = 2685914682L;
    /** 賣家登入 principal，<b>刻意與 {@link #STORE_ID} 不同</b>（re-key 過的店家）：拿 storeId 頂替會被抓到。 */
    private static final Long SELLER_ACCOUNT_ID = 9002685914682L;
    private static final Long ADMIN_ACCOUNT_ID = 7001L;
    private static final Long APPLICATION_ID = 843900000000000001L;
    private static final int ORGANIC = 3701;
    private static final int TRACEABLE = 3704;
    private static final String CERT_A = "1-009-300022";
    /** 政府查驗回的業者名稱；fixture 一律讓它匹配本賣家（§8.5.1 的第五個資格條件）。 */
    private static final String HOLDER_NAME = "測試農場";
    private static final String IDEM_KEY = "idem-key-1";

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
    @Mock org.springframework.data.redis.core.ValueOperations<String, String> redisValueOps;

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
        // 重新查驗租約：預設「拿得到」，個別測試再覆寫成 false 模擬併發
        when(redisTemplate.opsForValue()).thenReturn(redisValueOps);
        when(redisValueOps.setIfAbsent(anyString(), anyString(), anyLong(), any())).thenReturn(true);

        when(applicationRepository.findByProductTypeCertNoAndStatuses(anyLong(), anyInt(), anyString(), any()))
                .thenReturn(List.of());
        when(applicationRepository.lockSubmittedByProductAndTypeExceptCertNo(anyLong(), anyInt(), anyString()))
                .thenReturn(List.of());
        when(applicationRepository.lockApprovedByProductExcludingTypes(anyLong(), any()))
                .thenReturn(List.of());
        when(applicationRepository.findApprovedByProductId(anyLong())).thenReturn(List.of());
        when(applicationRepository.findApprovedByProductAndType(anyLong(), anyInt())).thenReturn(Optional.empty());
        when(applicationRepository.insertSubmittedIfAbsent(any(), any(), any(), any(), any(), any(),
                any(), anyBoolean(), any(), any(), any(), any())).thenReturn(1);
        when(applicationRepository.casTransitionStatus(anyLong(), anyLong(), anyString(), anyString(),
                any(), any(), any(), any(), any())).thenReturn(1);
        when(applicationRepository.casUpdateGovVerifyResult(anyLong(), anyLong(), anyString(), any(), any()))
                .thenReturn(1);
        when(productRepository.findByIdIncludingDeleted(anyLong()))
                .thenReturn(Optional.of(product()));
        when(productRepository.updateCertificationProjection(anyLong(), any())).thenReturn(1);
        when(idempotencyService.acquireLock(anyString(), any(), anyString(), any(), any()))
                .thenReturn(Optional.empty());
        when(govCertificationService.readCachedResult(anyString(), any())).thenReturn(null);
        when(sellerGrpcClient.getSellerMetaOrThrow(anyLong())).thenReturn(sellerMeta());
        // §8.5.1 第五個自動核准資格條件：政府回的業者名稱必須屬於這個賣家。
        // 本檔測的是「事件發不發／發給誰」，不是歸屬比對本身 ⇒ fixture 一律表達
        // 「歸屬成立的正常賣家」，否則每個自動核准案例都會落進 NO_HOLDER_NAME 轉人工而不發事件。
        // 歸屬比對的各個分支由 ProductCertificationApplicationServiceTest.HolderAttribution 覆蓋。
        when(sellerGrpcClient.matchCertificateHolder(STORE_ID, HOLDER_NAME))
                .thenReturn(new SellerGrpcClient.HolderMatch(true, true, "REAL_NAME", true));
    }

    // ==================== ① §12.3 推播策略三情境 ====================

    @Nested
    @DisplayName("§12.3 推播策略：即刻核准不發／背景補課發／人工裁決發")
    class PushPolicy {

        /**
         * 同步快取命中「有效」⇒ 在同一交易內即刻核准。賣家當下畫面就看到標章，
         * <b>不得</b>再推一則「已核准」。多發不會壞資料，所以沒有任何 gate 抓得到。
         */
        @Test
        @DisplayName("同步快取命中的即刻核准 → 不發 approved、不發 verify.requested")
        void syncCacheHitApproval_publishesNothing() {
            when(govCertificationService.readCachedResult(anyString(), any())).thenReturn(validGovResult());
            stubFreshApplication(ProductCertificationStatus.SUBMITTED.value(),
                    ProductCertificationGovVerifyState.VERIFIED.value(), false, 0L);

            service.syncFromProductWrite(syncCommand(ORGANIC, CERT_A));

            verify(eventPublisher, never()).publishCertificationApproved(any());
            verify(eventPublisher, never()).publishCertificationVerifyRequested(any());
        }

        /**
         * 背景補課核准（consumer 走 {@code approveBySystem} 這個 bean 外入口）⇒ <b>要發</b>。
         * 賣家的畫面停在「審核中」，不通知他永遠不會知道已經通過了。
         */
        @Test
        @DisplayName("背景補課核准（approveBySystem）→ 發 approved，decidedBy=SYSTEM")
        void backgroundApproval_publishesApprovedAsSystem() {
            stubLockedApplication(ProductCertificationStatus.SUBMITTED.value(),
                    ProductCertificationGovVerifyState.VERIFIED.value(), false, 3L);

            assertTrue(service.approveBySystem(APPLICATION_ID));

            ProductCertificationDecidedPayload payload = captureApproved();
            assertEquals(ProductEventType.PRODUCT_CERTIFICATION_APPROVED.value(), payload.eventType());
            assertEquals(ProductCertificationDecidedPayload.DECIDED_BY_SYSTEM, payload.decidedBy());
            assertEquals(String.valueOf(APPLICATION_ID), payload.applicationId());
            assertEquals(String.valueOf(PRODUCT_ID), payload.productId());
            assertEquals(PRODUCT_NAME, payload.productName());
            assertEquals(String.valueOf(ORGANIC), payload.certificationType());
            assertEquals(CERT_A, payload.certificateNo());
        }

        /** 自動核准資格不符（CAS 前的三條件之一不成立）⇒ 沒核准就<b>不得</b>發通知。 */
        @Test
        @DisplayName("背景補課但資格不符（requiresManualReview）→ 不發任何事件")
        void backgroundApprovalIneligible_publishesNothing() {
            stubLockedApplication(ProductCertificationStatus.SUBMITTED.value(),
                    ProductCertificationGovVerifyState.VERIFIED.value(), true, 3L);

            assertFalse(service.approveBySystem(APPLICATION_ID));

            verifyNoInteractions(eventPublisher);
        }

        @Test
        @DisplayName("ADM 人工核准 → 發 approved，decidedBy=ADMIN")
        void adminApproval_publishesApprovedAsAdmin() {
            stubLockedApplication(ProductCertificationStatus.SUBMITTED.value(),
                    ProductCertificationGovVerifyState.VERIFIED.value(), true, 5L);

            service.approveForAdmin(APPLICATION_ID, 5L, ADMIN_ACCOUNT_ID, "查驗通過", "內部備註", IDEM_KEY);

            ProductCertificationDecidedPayload payload = captureApproved();
            assertEquals(ProductCertificationDecidedPayload.DECIDED_BY_ADMIN, payload.decidedBy());
        }

        @Test
        @DisplayName("ADM 駁回 → 發 rejected，帶 reasonCode／publicReason")
        void adminRejection_publishesRejected() {
            stubLockedApplication(ProductCertificationStatus.SUBMITTED.value(),
                    ProductCertificationGovVerifyState.NOT_FOUND.value(), false, 5L);

            service.rejectForAdmin(APPLICATION_ID, 5L, ADMIN_ACCOUNT_ID,
                    "CERT_NOT_FOUND", "政府端查無此證號", "內部備註", IDEM_KEY);

            ArgumentCaptor<ProductCertificationDecidedPayload> captor =
                    ArgumentCaptor.forClass(ProductCertificationDecidedPayload.class);
            verify(eventPublisher).publishCertificationRejected(captor.capture());
            ProductCertificationDecidedPayload payload = captor.getValue();
            assertEquals(ProductEventType.PRODUCT_CERTIFICATION_REJECTED.value(), payload.eventType());
            assertEquals("CERT_NOT_FOUND", payload.reasonCode());
            assertEquals("政府端查無此證號", payload.publicReason());
        }

        @Test
        @DisplayName("ADM 撤銷 → 發 revoked，帶 reasonCode／publicReason")
        void adminRevocation_publishesRevoked() {
            stubLockedApplication(ProductCertificationStatus.APPROVED.value(),
                    ProductCertificationGovVerifyState.VERIFIED.value(), false, 6L);

            service.revokeForAdmin(APPLICATION_ID, 6L, ADMIN_ACCOUNT_ID,
                    "CERT_HOLDER_MISMATCH", "證號歸屬不符", "內部備註", IDEM_KEY);

            ArgumentCaptor<ProductCertificationDecidedPayload> captor =
                    ArgumentCaptor.forClass(ProductCertificationDecidedPayload.class);
            verify(eventPublisher).publishCertificationRevoked(captor.capture());
            assertEquals(ProductEventType.PRODUCT_CERTIFICATION_REVOKED.value(),
                    captor.getValue().eventType());
        }

        /** 賣家撤回是「自己停止宣稱」不是平台裁決，§12.3 的表沒有這一列 ⇒ 不推播。 */
        @Test
        @DisplayName("賣家撤回 → 不發任何裁決事件")
        void sellerWithdraw_publishesNothing() {
            ProductCertificationApplicationEntity app = application(
                    ProductCertificationStatus.APPROVED.value(),
                    ProductCertificationGovVerifyState.VERIFIED.value(), false, 2L);
            when(applicationRepository.findByIdForUpdate(APPLICATION_ID)).thenReturn(Optional.of(app));
            when(applicationRepository.findById(APPLICATION_ID)).thenReturn(Optional.of(app));

            service.withdrawForSeller(PRODUCT_ID, APPLICATION_ID, STORE_ID, SELLER_ACCOUNT_ID, 2L, IDEM_KEY);

            verifyNoInteractions(eventPublisher);
        }
    }

    // ==================== ② §12.1 verify.requested ====================

    @Nested
    @DisplayName("§12.1 verify.requested：只有 PENDING 才排背景查驗")
    class VerifyRequested {

        @Test
        @DisplayName("同步快取未命中（PENDING）→ 發 verify.requested，id 欄位全為 string")
        void cacheMiss_publishesVerifyRequested() {
            stubFreshApplication(ProductCertificationStatus.SUBMITTED.value(),
                    ProductCertificationGovVerifyState.PENDING.value(), false, 0L);

            service.syncFromProductWrite(syncCommand(ORGANIC, CERT_A));

            ProductCertificationVerifyRequestedPayload payload = captureVerifyRequested();
            assertEquals(ProductEventType.PRODUCT_CERTIFICATION_VERIFY_REQUESTED.value(), payload.eventType());
            assertEquals(String.valueOf(APPLICATION_ID), payload.applicationId());
            assertEquals(String.valueOf(PRODUCT_ID), payload.productId());
            assertEquals(String.valueOf(STORE_ID), payload.storeId());
            assertEquals(String.valueOf(ORGANIC), payload.certificationType());
            assertEquals(CERT_A, payload.certificateNo());
            assertFalse(payload.requiresManualReview());
            verify(eventPublisher, never()).publishCertificationApproved(any());
        }

        /**
         * §6.3 Step 3：曾被 REJECTED／REVOKED 的號碼重送 ⇒ 永不自動核准，
         * <b>但查驗照排</b>——ADM 重審需要政府資料當依據。旗標帶在 payload 裡給 consumer 判斷。
         */
        @Test
        @DisplayName("requiresManualReview=true 仍要發 verify.requested，且旗標帶進 payload")
        void manualReviewRequired_stillPublishesVerifyRequested() {
            stubFreshApplication(ProductCertificationStatus.SUBMITTED.value(),
                    ProductCertificationGovVerifyState.PENDING.value(), true, 0L);

            service.syncFromProductWrite(syncCommand(ORGANIC, CERT_A));

            assertTrue(captureVerifyRequested().requiresManualReview());
        }

        /** 3704–3706 沒有政府開放資料介面 ⇒ 落 UNSUPPORTED，發出去只會讓 consumer 空跑。 */
        @Test
        @DisplayName("不支援線上查驗的類型（UNSUPPORTED）→ 不發 verify.requested")
        void unsupportedType_publishesNothing() {
            stubFreshApplication(ProductCertificationStatus.SUBMITTED.value(),
                    ProductCertificationGovVerifyState.UNSUPPORTED.value(), false, 0L);

            service.syncFromProductWrite(syncCommand(TRACEABLE, "1234567890"));

            verify(eventPublisher, never()).publishCertificationVerifyRequested(any());
        }

        @Test
        @DisplayName("賣家單筆申請（PENDING）→ 也發 verify.requested")
        void sellerSubmit_publishesVerifyRequested() {
            stubFreshApplication(ProductCertificationStatus.SUBMITTED.value(),
                    ProductCertificationGovVerifyState.PENDING.value(), false, 0L);
            when(productRepository.findByIdAndStoreId(PRODUCT_ID, STORE_ID)).thenReturn(Optional.of(product()));

            service.submitForSeller(PRODUCT_ID, STORE_ID, SELLER_ACCOUNT_ID,
                    String.valueOf(ORGANIC), CERT_A, IDEM_KEY);

            assertEquals(String.valueOf(APPLICATION_ID), captureVerifyRequested().applicationId());
        }
    }

    // ==================== ③ §12.4 收件人解析 fail-closed ====================

    @Nested
    @DisplayName("§12.4 sellerAccountId：fail-closed，且必須是 account_id")
    class RecipientResolution {

        /**
         * 🔴 這一格擋的是「發出缺收件人的 payload」：事件已「成功」發出、通知靜默蒸發、
         * 重送救不回來。fail-closed 的表現＝<b>整個裁決交易 rollback</b>，
         * ADM 收到錯誤可以重試，沒有東西被靜默吞掉。
         */
        @Test
        @DisplayName("收件人解析失敗 → 不得發出任何 payload，且例外向外傳播讓交易 rollback")
        void recipientResolutionFails_publishesNothingAndPropagates() {
            stubLockedApplication(ProductCertificationStatus.SUBMITTED.value(),
                    ProductCertificationGovVerifyState.VERIFIED.value(), true, 5L);
            when(sellerGrpcClient.getSellerMetaOrThrow(anyLong()))
                    .thenThrow(new IllegalStateException("查無賣家元資料，storeId=" + STORE_ID));

            assertThrows(IllegalStateException.class, () ->
                    service.approveForAdmin(APPLICATION_ID, 5L, ADMIN_ACCOUNT_ID, null, null, IDEM_KEY));

            verifyNoInteractions(eventPublisher);
        }

        /**
         * 首任店家 {@code store_id == account_id}，拿 storeId 頂替在 dev 與 fixture 全綠，
         * 只有 re-key 過的店家收不到通知而且沒有任何告警。本測試的兩個常數刻意不相等。
         */
        @Test
        @DisplayName("收件人是登入 principal（account_id），不是 store_id")
        void recipientIsAccountIdNotStoreId() {
            stubLockedApplication(ProductCertificationStatus.SUBMITTED.value(),
                    ProductCertificationGovVerifyState.VERIFIED.value(), false, 3L);

            service.approveBySystem(APPLICATION_ID);

            ProductCertificationDecidedPayload payload = captureApproved();
            assertEquals(String.valueOf(SELLER_ACCOUNT_ID), payload.sellerAccountId());
            assertEquals(String.valueOf(STORE_ID), payload.storeId());
            verify(sellerGrpcClient).getSellerMetaOrThrow(STORE_ID);
        }
    }

    // ==================== ④ wire 形狀 ====================

    @Nested
    @DisplayName("§12.4 wire 形狀：snake_case、id 為字串、internalNote 不外洩")
    class WireShape {

        /** 🔴 {@code internalNote} 僅 ADM 可見，絕不進 payload——序列化後也不能有任何殘影。 */
        @Test
        @DisplayName("駁回 payload 不含 internalNote，且 wire key 為 snake_case、id 為 JSON 字串")
        void rejectedPayloadWireShape() throws Exception {
            stubLockedApplication(ProductCertificationStatus.SUBMITTED.value(),
                    ProductCertificationGovVerifyState.NOT_FOUND.value(), false, 5L);

            service.rejectForAdmin(APPLICATION_ID, 5L, ADMIN_ACCOUNT_ID,
                    "CERT_NOT_FOUND", "政府端查無此證號", "只有 ADM 看得到的內部備註", IDEM_KEY);

            ArgumentCaptor<ProductCertificationDecidedPayload> captor =
                    ArgumentCaptor.forClass(ProductCertificationDecidedPayload.class);
            verify(eventPublisher).publishCertificationRejected(captor.capture());

            String json = new ObjectMapper().writeValueAsString(captor.getValue());
            assertFalse(json.contains("內部備註"), "internalNote 洩漏到 payload: " + json);
            assertFalse(json.contains("internal"), "payload 出現 internal 欄位: " + json);
            // 沒帶 @JsonNaming 就會發出 camelCase key，而 contract gate 只驗
            // 「template 讀的 key ⊆ golden」、不驗「golden ⊆ publisher 實發」⇒ 全程無訊號
            assertFalse(json.contains("sellerAccountId"), "wire key 是 camelCase: " + json);
            assertFalse(json.contains("applicationId"), "wire key 是 camelCase: " + json);

            Map<?, ?> wire = new ObjectMapper().readValue(json, Map.class);
            assertTrue(wire.containsKey("application_id"), "wire key 不是 snake_case: " + wire.keySet());
            assertTrue(wire.containsKey("seller_account_id"), "wire key 不是 snake_case: " + wire.keySet());
            assertTrue(wire.containsKey("reason_code"));
            assertTrue(wire.containsKey("public_reason"));
            // TSID 以 JSON number 傳出去會被 generic-map 消費端的 float64 截斷
            assertEquals(String.class, wire.get("application_id").getClass());
            assertEquals(String.class, wire.get("seller_account_id").getClass());
        }

        /** approved 沒有駁回原因可帶 ⇒ 那兩個 key 直接不出現（NON_NULL），不是 {@code null}。 */
        @Test
        @DisplayName("核准 payload 不帶 reason_code／public_reason")
        void approvedPayloadOmitsReasonKeys() throws Exception {
            stubLockedApplication(ProductCertificationStatus.SUBMITTED.value(),
                    ProductCertificationGovVerifyState.VERIFIED.value(), false, 3L);

            service.approveBySystem(APPLICATION_ID);

            String json = new ObjectMapper().writeValueAsString(captureApproved());
            Map<?, ?> wire = new ObjectMapper().readValue(json, Map.class);
            assertFalse(wire.containsKey("reason_code"), wire.keySet().toString());
            assertFalse(wire.containsKey("public_reason"), wire.keySet().toString());
        }

        @Test
        @DisplayName("verify.requested wire key 為 snake_case、id 為 JSON 字串")
        void verifyRequestedWireShape() throws Exception {
            stubFreshApplication(ProductCertificationStatus.SUBMITTED.value(),
                    ProductCertificationGovVerifyState.PENDING.value(), false, 0L);

            service.syncFromProductWrite(syncCommand(ORGANIC, CERT_A));

            String json = new ObjectMapper().writeValueAsString(captureVerifyRequested());
            Map<?, ?> wire = new ObjectMapper().readValue(json, Map.class);
            assertTrue(wire.containsKey("application_id"), wire.keySet().toString());
            assertTrue(wire.containsKey("requires_manual_review"), wire.keySet().toString());
            assertEquals(String.class, wire.get("application_id").getClass());
            assertEquals(String.class, wire.get("store_id").getClass());
        }
    }

    // ==================== fixtures ====================

    private ProductCertificationDecidedPayload captureApproved() {
        ArgumentCaptor<ProductCertificationDecidedPayload> captor =
                ArgumentCaptor.forClass(ProductCertificationDecidedPayload.class);
        verify(eventPublisher).publishCertificationApproved(captor.capture());
        return captor.getValue();
    }

    private ProductCertificationVerifyRequestedPayload captureVerifyRequested() {
        ArgumentCaptor<ProductCertificationVerifyRequestedPayload> captor =
                ArgumentCaptor.forClass(ProductCertificationVerifyRequestedPayload.class);
        verify(eventPublisher).publishCertificationVerifyRequested(captor.capture());
        return captor.getValue();
    }

    /** 收斂點插入後會 {@code findById} 讀回來；同時餵給 {@code approveBySystemInternal} 的 FOR UPDATE。 */
    private void stubFreshApplication(String status, String govState, boolean manualReview, long version) {
        ProductCertificationApplicationEntity app = application(status, govState, manualReview, version);
        when(applicationRepository.findById(anyLong())).thenReturn(Optional.of(app));
        when(applicationRepository.findByIdForUpdate(anyLong())).thenReturn(Optional.of(app));
    }

    private void stubLockedApplication(String status, String govState, boolean manualReview, long version) {
        ProductCertificationApplicationEntity app = application(status, govState, manualReview, version);
        when(applicationRepository.findByIdForUpdate(APPLICATION_ID)).thenReturn(Optional.of(app));
        when(applicationRepository.findById(APPLICATION_ID)).thenReturn(Optional.of(app));
    }

    private ProductCertificationApplicationEntity application(String status, String govState,
                                                              boolean manualReview, long version) {
        ProductCertificationApplicationEntity app = new ProductCertificationApplicationEntity();
        app.setId(APPLICATION_ID);
        app.setProductId(PRODUCT_ID);
        app.setStoreId(STORE_ID);
        app.setSubmittedByAccountId(SELLER_ACCOUNT_ID);
        app.setCertificationType(ORGANIC);
        app.setCertificateNo(CERT_A);
        app.setStatus(status);
        app.setSource(ProductCertificationSource.SELLER.value());
        app.setRequiresManualReview(manualReview);
        app.setGovVerifyState(govState);
        app.setVersion(version);
        app.setSubmittedAt(LocalDateTime.now());
        // 查驗通過的申請一定帶著政府回的 payload；少了 holderName 就走不到自動核准（§8.5.1）。
        ProductCertificationApplicationEntity.GovVerifyPayload payload =
                new ProductCertificationApplicationEntity.GovVerifyPayload();
        payload.setCertType(String.valueOf(ORGANIC));
        payload.setCertNo(CERT_A);
        payload.setValidity("VALID");
        payload.setHolderName(HOLDER_NAME);
        app.setGovVerifyPayload(payload);
        return app;
    }

    private ProductEntity product() {
        ProductEntity p = new ProductEntity();
        p.setId(PRODUCT_ID);
        p.setStoreId(STORE_ID);
        p.setName(PRODUCT_NAME);
        return p;
    }

    private SellerMeta sellerMeta() {
        return SellerMeta.newBuilder()
                .setAccountId(SELLER_ACCOUNT_ID)
                .setStoreId(String.valueOf(STORE_ID))
                .build();
    }

    private ProductCertificationApplicationService.CertificationSyncCommand syncCommand(int type, String certNo) {
        return new ProductCertificationApplicationService.CertificationSyncCommand(
                PRODUCT_ID, STORE_ID, SELLER_ACCOUNT_ID,
                ProductCertificationSource.SELLER.value(), true,
                List.of(new ProductCertificationApplicationService.CertificationInput(
                        String.valueOf(type), certNo)));
    }

    private GovCertificationService.GovVerifyResult validGovResult() {
        return new GovCertificationService.GovVerifyResult(
                String.valueOf(ORGANIC), CERT_A, "2026-08-26T00:00:00Z", true,
                GovCertificationService.Validity.VALID, "通過", "2027-01-01",
                "測試農場", "台北市大安區", "驗證機構", "小松菜", 1, null);
    }
}
