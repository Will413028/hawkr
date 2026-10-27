package io.github.will413028.hawkr.product.service;

import io.github.will413028.hawkr.common.obs.context.ObsKeys;
import io.github.will413028.hawkr.common.obs.util.IdGenerator;
import io.github.will413028.hawkr.common.obs.log.ObsLogger;
import io.github.will413028.hawkr.common.utils.DateTimeUtils;
import io.github.will413028.hawkr.common.utils.TsidUtil;
import io.github.will413028.hawkr.common.utils.code.CertificationType;
import io.github.will413028.hawkr.common.utils.code.CodeMetaValidator;
import io.github.will413028.hawkr.common.utils.code.GrpcResponseCode;
import io.github.will413028.hawkr.common.utils.code.ProductCertificationGovVerifyState;
import io.github.will413028.hawkr.common.utils.code.ProductCertificationRejectReason;
import io.github.will413028.hawkr.common.utils.code.ProductCertificationReviewAction;
import io.github.will413028.hawkr.common.utils.code.ProductCertificationSource;
import io.github.will413028.hawkr.common.utils.code.ProductCertificationStatus;
import io.github.will413028.hawkr.common.utils.code.ProductEventType;
import io.github.will413028.hawkr.product.domain.CertificationTypeRules;
import io.github.will413028.hawkr.product.domain.ProductCertificationRules;
import io.github.will413028.hawkr.product.dto.response.CertificationApplicationRefDTO;
import io.github.will413028.hawkr.product.dto.response.CertificationWarningDTO;
import io.github.will413028.hawkr.product.entity.ProductCertificationApplicationEntity;
import io.github.will413028.hawkr.product.entity.ProductCertificationApplicationEntity.GovVerifyPayload;
import io.github.will413028.hawkr.product.entity.ProductCertificationReviewEventEntity;
import io.github.will413028.hawkr.product.entity.ProductEntity;
import io.github.will413028.hawkr.product.exception.ProductBusinessException;
import io.github.will413028.hawkr.product.grpc.client.SellerGrpcClient;
import io.github.will413028.hawkr.product.mq.ProductEventPublisher;
import io.github.will413028.hawkr.product.mq.dto.ProductCertificationDecidedPayload;
import io.github.will413028.hawkr.product.mq.dto.ProductCertificationVerifyRequestedPayload;
import io.github.will413028.hawkr.product.repository.ProductCertificationApplicationRepository;
import io.github.will413028.hawkr.product.repository.ProductCertificationReviewEventRepository;
import io.github.will413028.hawkr.product.repository.ProductRepository;
import com.fasterxml.jackson.databind.ObjectMapper;
import org.slf4j.MDC;
import org.springframework.dao.DataAccessException;
import org.springframework.data.redis.core.StringRedisTemplate;
import org.springframework.stereotype.Service;
import org.springframework.transaction.annotation.Propagation;
import org.springframework.transaction.annotation.Transactional;
import org.springframework.transaction.support.TransactionSynchronization;
import org.springframework.transaction.support.TransactionSynchronizationManager;

import java.time.LocalDateTime;
import java.time.temporal.ChronoUnit;
import java.util.concurrent.TimeUnit;
import java.util.ArrayList;
import java.util.LinkedHashSet;
import java.util.List;
import java.util.Objects;
import java.util.Optional;
import java.util.Set;
import java.util.function.Supplier;

/**
 * 商品認證申請與審核的收斂點（19 號設計書《農業認證驗證》§6／§7）。
 *
 * <p>🔴 <b>必須是獨立 {@code @Service} bean</b>（§6.7）。寫成 {@code ProductService} 的私有方法
 * 並標 {@code @Transactional}，Spring proxy 不攔截、<b>事務完全失效</b>，而編譯與測試都不會報錯。
 * 本 repo 已有前例——四處帶 {@code InTx} 之名卻從未有事務（#1370），命名讓人誤信。
 *
 * <p><b>本類別承擔的三件事</b>：
 * <ol>
 *   <li>{@link #syncFromProductWrite}：五條商品寫入路徑（賣家建立／更新、ADM 代行建立／更新、匯入 commit）
 *       的<b>唯一</b>認證收斂點。收斂後任何路徑都不得再直寫 {@code product.certifications} 投影。</li>
 *   <li>{@link #rebuildProjection}：由當前所有 {@code APPROVED} 申請重算公開投影。
 *       <b>四個改變 approved 集合的觸發點一個都不能漏</b>（APPROVE／SUPERSEDE／WITHDRAW／REVOKE）
 *       ——漏了，已撤銷的標章會繼續公開，而 §13 的清快取只是讓它重讀同一份 stale JSONB。</li>
 *   <li>核准 transaction（§6.6）：人工（ADM）與系統（同步快取命中／背景查驗補課）
 *       <b>共用同一段實作</b>，差別只在 CAS 來源與失敗處置。</li>
 * </ol>
 */
@Service
public class ProductCertificationApplicationService {

    private static final ObsLogger log = ObsLogger.forClass(ProductCertificationApplicationService.class);

    private final ProductCertificationApplicationRepository applicationRepository;
    private final ProductCertificationReviewEventRepository reviewEventRepository;
    private final ProductRepository productRepository;
    private final GovCertificationService govCertificationService;
    private final ProductCacheService cacheService;
    private final ProductRevalidationService revalidationService;
    private final IdempotencyService idempotencyService;
    private final ObjectMapper objectMapper;
    private final ProductEventPublisher eventPublisher;
    private final SellerGrpcClient sellerGrpcClient;
    private final StringRedisTemplate redisTemplate;

    public ProductCertificationApplicationService(
            ProductCertificationApplicationRepository applicationRepository,
            ProductCertificationReviewEventRepository reviewEventRepository,
            ProductRepository productRepository,
            GovCertificationService govCertificationService,
            ProductCacheService cacheService,
            ProductRevalidationService revalidationService,
            IdempotencyService idempotencyService,
            ObjectMapper objectMapper,
            ProductEventPublisher eventPublisher,
            SellerGrpcClient sellerGrpcClient,
            StringRedisTemplate redisTemplate) {
        this.applicationRepository = applicationRepository;
        this.reviewEventRepository = reviewEventRepository;
        this.productRepository = productRepository;
        this.govCertificationService = govCertificationService;
        this.cacheService = cacheService;
        this.revalidationService = revalidationService;
        this.idempotencyService = idempotencyService;
        this.objectMapper = objectMapper;
        this.eventPublisher = eventPublisher;
        this.sellerGrpcClient = sellerGrpcClient;
        this.redisTemplate = redisTemplate;
    }

    // ==================== 對外型別 ====================

    /** 一筆認證輸入（賣家送來的原始值，可能非法）。 */
    public record CertificationInput(String certificationType, String certificateNo) {
    }

    /**
     * §6.3 收斂點的輸入。
     *
     * @param updateCertifications proto {@code UpdateProductRequest.update_certifications} 旗標原值。
     *                             🔴 Step 0 的條件是<b>「旗標 OR 認證非空」</b>，不是只看旗標，見
     *                             {@link #syncFromProductWrite}
     * @param certifications       本次送出的認證清單；{@code null} 與空清單在 Step 0 的意義不同
     */
    public record CertificationSyncCommand(Long productId,
                                           Long storeId,
                                           Long submittedByAccountId,
                                           String source,
                                           boolean updateCertifications,
                                           List<CertificationInput> certifications) {
    }

    /** 分頁查詢結果。 */
    public record Paged<T>(List<T> records, long total) {
    }

    // ==================== §6.3 寫入收斂 ====================

    /**
     * 五條商品寫入路徑的唯一認證收斂點（§6.3 Step 0–6 ＋ §6.4 全量取代）。
     *
     * <p>🔴 <b>Step 0（前置閘門，先於一切）</b>：
     * <pre>effectiveUpdate = update_certifications || !certifications.isEmpty()</pre>
     * 兩個方向都是零訊號失效，缺一不可：
     * <ul>
     *   <li><b>漏掉整個閘門</b> ⇒ 任何部分更新（舊 App 只改名稱、代行 PATCH 單一欄位）都會讓
     *       「本次送出的 type 集合」變成空集合 ⇒ §6.4 把該商品<b>全部 APPROVED 一次轉 WITHDRAWN</b>
     *       ⇒ 法規標章消失。而 HTTP 200、稽核表有列（且記成語意完全正確的「賣家停止宣稱」）、
     *       {@code WITHDRAWN} 不發推播 ⇒ 賣家不會收到任何通知。</li>
     *   <li><b>條件寫成只看旗標</b>（{@code if (!updateCertifications) return;}）⇒ 舊 App 送了
     *       認證但沒設旗標的請求直接 return ⇒ <b>賣家送出的認證靜默蒸發</b>。
     *       {@code api/proto/product/product.proto} 該欄註解明訂「相容期：未設但 certifications
     *       非空仍視為要更新」，而 {@code api/proto/} 的契約優先序<b>高於</b>詳細設計書。</li>
     * </ul>
     *
     * <p><b>交易傳播</b>：{@code REQUIRED} —— 投影重建必須與商品寫入原子。跳過某筆認證不拋例外，
     * 故不會拖垮外層交易（§16.2）。
     *
     * @return 本次未被採用的認證（§6.3.1 {@code certificationWarnings[]}）；無警告時為空清單
     */
    @Transactional(propagation = Propagation.REQUIRED)
    public List<CertificationWarningDTO> syncFromProductWrite(CertificationSyncCommand cmd) {
        List<CertificationInput> raw = cmd.certifications() == null ? List.of() : cmd.certifications();

        // ---- Step 0：前置閘門 ----
        boolean effectiveUpdate = cmd.updateCertifications() || !raw.isEmpty();
        if (!effectiveUpdate) {
            return List.of();
        }

        List<CertificationWarningDTO> warnings = new ArrayList<>();

        // ---- 第二道防線：非法輸入剔除（§6.2.1 層 ②）----
        // Gateway handler 的 filter 是第一道；這裡接住 gRPC 直呼與未來新增的第五條路徑。
        // 一律「剔除該筆 ＋ warn log ＋ 落進 warnings」，不拋例外、不回 4xx（§6.2.2）。
        List<NormalizedCertification> accepted = new ArrayList<>();
        Set<String> seen = new LinkedHashSet<>();
        for (CertificationInput input : raw) {
            String rawType = input == null ? null : input.certificationType();
            String rawCertNo = input == null ? null : input.certificateNo();
            String type = trimToNull(rawType);

            if (type == null || !CodeMetaValidator.isValidValue(CertificationType.class, type)) {
                log.warn("syncFromProductWrite.invalidType",
                        "認證類型不合法，已剔除該筆",
                        "product_id", String.valueOf(cmd.productId()));
                warnings.add(CertificationWarningDTO.invalidType(rawType, rawCertNo));
                continue;
            }

            // §6.2.9：certNo 的合法性「依類型而異」。對六型一律套嚴格白名單，
            // ISO 22000:2018（含冒號）與 HACCP／GMP（含空格、斜線）的真實編號會被剔除
            // ⇒ 賣家送出成功、不建申請、不回錯 ⇒ 零訊號。
            // 🔴 驗什麼就存什麼：底下用的是回傳值（放寬類已 trim），不是原值。
            CertificationType certType = CertificationType.fromValue(type);
            String certNo = ProductCertificationRules.normalizeCertNo(certType, rawCertNo);
            if (certNo == null) {
                log.warn("syncFromProductWrite.invalidCertNo",
                        "認證編號不合法，已剔除該筆",
                        "product_id", String.valueOf(cmd.productId()));
                warnings.add(CertificationWarningDTO.invalidCertNo(type, rawCertNo));
                continue;
            }
            // 同一次請求送了兩筆完全相同的 (type, certNo)：後者是純重複，不是衝突，不發 warning。
            // 去重鍵用正規化後的值，否則「同一個編號、尾端多一個空白」會被當成兩筆。
            if (seen.add(type + "|" + certNo)) {
                accepted.add(new NormalizedCertification(certType, certNo));
            }
        }

        // ---- Step 1–6：逐組 (type, certNo) ----
        // Step 6 的自動核准會自己重建投影（approveInternal 內），這裡不重複計數 ——
        // 否則同一次寫入會對 product 下兩次 UPDATE，白白多 bump 一次 version。
        for (NormalizedCertification cert : accepted) {
            CertificationWarningDTO warning = syncOne(cmd, cert);
            if (warning != null) {
                warnings.add(warning);
            }
        }

        // ---- §6.4 全量取代：該 type 完全沒送 ----
        boolean approvedSetChanged = false;
        // 粒度是 certification_type 而非 (type, certNo)：換號時舊號碼必須保持公開到新號碼核准為止。
        // SUBMITTED 刻意不在撤回範圍內（待審是進行中的請求，只能由明確 withdraw 撤銷）。
        List<Integer> submittedTypes = accepted.stream()
                .map(c -> Integer.valueOf(c.type().value()))
                .distinct()
                .toList();
        String actorRole = ProductCertificationRules.actorRoleForSource(cmd.source());
        Long actorAccountId = ProductCertificationRules.ACTOR_SYSTEM.equals(actorRole)
                ? null : cmd.submittedByAccountId();
        LocalDateTime now = DateTimeUtils.nowUtc();
        for (ProductCertificationApplicationEntity stale
                : applicationRepository.lockApprovedByProductExcludingTypes(cmd.productId(), submittedTypes)) {
            int rows = applicationRepository.transitionToTerminalStatus(
                    List.of(stale.getId()),
                    ProductCertificationStatus.APPROVED.value(),
                    ProductCertificationStatus.WITHDRAWN.value(),
                    now);
            if (rows > 0) {
                writeEvent(stale, ProductCertificationReviewAction.WITHDRAW.value(),
                        ProductCertificationStatus.APPROVED.value(),
                        ProductCertificationStatus.WITHDRAWN.value(),
                        actorRole, actorAccountId, null, null, null,
                        stale.getVersion() + 1);
                approvedSetChanged = true;
            }
        }

        // ---- §7.2 投影重建（觸發點：APPROVE 已由 approveInternal 觸發，此處補 WITHDRAW）----
        if (approvedSetChanged) {
            rebuildProjection(cmd.productId());
        }
        return warnings;
    }

    /**
     * 單一 (type, certNo) 的 Step 1–6。
     *
     * @return 該筆的警告；{@code null} = 沒有警告。投影重建由 Step 6 的核准實作自行負責。
     */
    private CertificationWarningDTO syncOne(CertificationSyncCommand cmd, NormalizedCertification cert) {
        Long productId = cmd.productId();
        Integer typeValue = Integer.valueOf(cert.type().value());
        String certNo = cert.certNo();

        // Step 1：同 (product, type, certNo) 已 APPROVED ⇒ no-op。
        //
        // 🔴 必須在最前面。新列一律以 SUBMITTED 插入，同 (type, certNo) 已 APPROVED 時
        // 不觸發任何唯一索引衝突（uk_pca_pending 的謂詞是 status='SUBMITTED'）
        // ⇒ 而全量取代語意下每次商品編輯都會把整份 approved 投影原樣送回
        // ⇒ 少了這一步，每存一次商品就替每個已核准認證多建一筆 SUBMITTED，ADM 佇列被灌爆。
        // 命中即跳過也連帶不執行 Step 4 ⇒ 不會誤殺賣家正在等待的換號申請。
        if (!applicationRepository.findByProductTypeCertNoAndStatuses(productId, typeValue, certNo,
                List.of(ProductCertificationStatus.APPROVED.value())).isEmpty()) {
            return null;
        }

        // Step 2：同組已在待審 ⇒ no-op（冪等：賣家重送同號碼）
        if (!applicationRepository.findByProductTypeCertNoAndStatuses(productId, typeValue, certNo,
                List.of(ProductCertificationStatus.SUBMITTED.value())).isEmpty()) {
            return null;
        }

        // Step 3：曾被 REJECTED／REVOKED ⇒ 建 SUBMITTED 但標 requiresManualReview
        // （§5.3：不得讓重送繞過 ADM 的裁決；但仍要排背景查驗，ADM 重審需要政府資料當依據）
        boolean requiresManualReview = !applicationRepository.findByProductTypeCertNoAndStatuses(
                productId, typeValue, certNo,
                List.of(ProductCertificationStatus.REJECTED.value(),
                        ProductCertificationStatus.REVOKED.value())).isEmpty();

        // Step 4：同 type 的異號待審 ⇒ SUPERSEDED（每一列都要寫 review_event）
        supersedeOtherPendingOfSameType(productId, typeValue, certNo);

        // §8.2 同步階段：只讀結構化快取，一律不發外部呼叫
        GovSnapshot snapshot = resolveGovSnapshotFromCache(cert);

        // Step 5：一律以 status='SUBMITTED' 插入 ＋ ON CONFLICT DO NOTHING
        //
        // 🔴 絕不可直接以 APPROVED 插入：uk_pca_active 的謂詞是 status='APPROVED'，
        // 同 (product, type) 已有別的號碼是 APPROVED 時會撞唯一索引，而 DO NOTHING
        // 會靜默丟棄新申請（無列、無錯）。
        Long applicationId = TsidUtil.generate();
        LocalDateTime now = DateTimeUtils.nowUtc();
        int inserted = applicationRepository.insertSubmittedIfAbsent(
                applicationId, productId, cmd.storeId(), cmd.submittedByAccountId(),
                typeValue, certNo, cmd.source(), requiresManualReview,
                snapshot.state(), snapshot.payload(), snapshot.verifiedAt(), now);

        if (inserted == 0) {
            // 序列情境插不出衝突（Step 4 已清掉同 type 的異號待審）⇒ 只有併發才會到這裡。
            // 商品路徑不 rollback、不回 4xx（舊 App 不得收到 4xx，且認證層級的競態不該讓
            // 整次商品儲存失敗），但必須明列在 certificationWarnings[]，否則賣家永遠不知道。
            log.warn("syncFromProductWrite.concurrentConflict",
                    "ON CONFLICT DO NOTHING 未插入任何列，已跳過該筆認證",
                    "product_id", String.valueOf(productId));
            return CertificationWarningDTO.concurrentConflict(cert.type().value(), certNo);
        }

        ProductCertificationApplicationEntity created =
                applicationRepository.findById(applicationId).orElseThrow(
                        () -> new IllegalStateException("剛插入的認證申請讀不回來，id=" + applicationId));
        writeEvent(created, ProductCertificationReviewAction.SUBMIT.value(),
                null, ProductCertificationStatus.SUBMITTED.value(),
                ProductCertificationRules.actorRoleForSource(cmd.source()),
                cmd.submittedByAccountId(), null, null, null, created.getVersion());

        // Step 6：同步快取命中「有效」⇒ 在同一交易內接著走 §6.6 的核准實作。
        // 走 private 實作而非 approveBySystem()：同 bean 內以 this. 呼叫自家 @Transactional
        // 方法，Spring proxy 不攔截（#1370 的形狀）。這裡本來就在外層交易內，行為相同，
        // 但不留下一個「看起來有交易語意、實際不會生效」的呼叫。
        // 核准成功時 approveInternal 已重建投影，此處不再回報 approvedSetChanged。
        // notifySeller=false：即刻核准的賣家當下畫面就看到標章，不再推播（§12.3）。
        approveBySystemInternal(applicationId, false);

        // §12.1：同步快取未命中（PENDING）⇒ 於 afterCommit 排背景查驗。
        publishVerifyRequestedIfPending(created, snapshot.state());
        return null;
    }

    /**
     * §6.3 Step 4：把同 (product, type) 但號碼不同的待審申請轉成 {@code SUPERSEDED}。
     *
     * <p>🔴 <b>每一列都要寫 review_event</b>：若 T1 已 commit {@code (3701, B, SUBMITTED)}，
     * T2 隨後執行本步驟（謂詞 {@code certificate_no <> C}）會把<b>併發贏家 T1 剛建立的申請 B</b>
     * 轉成 SUPERSEDED。這不是換號、是競態的副產物，但對 B 而言後果一樣（退場）
     * ——沒有事件，賣家的申請就是憑空消失且查不到是誰動的。
     */
    private void supersedeOtherPendingOfSameType(Long productId, Integer typeValue, String certNo) {
        List<ProductCertificationApplicationEntity> others =
                applicationRepository.lockSubmittedByProductAndTypeExceptCertNo(productId, typeValue, certNo);
        if (others.isEmpty()) {
            return;
        }
        LocalDateTime now = DateTimeUtils.nowUtc();
        for (ProductCertificationApplicationEntity other : others) {
            int rows = applicationRepository.transitionToTerminalStatus(
                    List.of(other.getId()),
                    ProductCertificationStatus.SUBMITTED.value(),
                    ProductCertificationStatus.SUPERSEDED.value(),
                    now);
            if (rows > 0) {
                writeEvent(other, ProductCertificationReviewAction.SUPERSEDE.value(),
                        ProductCertificationStatus.SUBMITTED.value(),
                        ProductCertificationStatus.SUPERSEDED.value(),
                        ProductCertificationRules.ACTOR_SYSTEM, null, null, null, null,
                        other.getVersion() + 1);
            }
        }
    }

    // ==================== §7.2 投影重建 ====================

    /**
     * 由該商品當前所有 {@code APPROVED} 申請重算 {@code product.certifications} 公開投影（§7.2）。
     *
     * <p>🔴 <b>四個改變 approved 集合的觸發點都必須在同一 transaction 內呼叫它</b>：
     * {@code APPROVE}（人工＋系統）／{@code SUPERSEDE}／{@code WITHDRAW}（由 APPROVED 轉出時）／
     * {@code REVOKE}。漏了任一個 ⇒ 已撤銷／已盜用的標章繼續公開，而 §13 的清快取只是讓它重讀
     * <b>同一份未更新的 JSONB</b>（清了等於沒清）。{@code REJECT} 不改 approved 集合，
     * 但為統一亦呼叫（本方法對「集合沒變」是 no-op，冪等）。
     *
     * <p>投影沒變就不寫 —— 避免每次商品編輯都白白 bump 一次 {@code product.version}
     * （那會讓代行者手上的 {@code expectedVersion} 平白失效）。
     *
     * @return true = 實際覆寫了投影
     */
    public boolean rebuildProjection(Long productId) {
        List<ProductCertificationApplicationEntity> approved =
                applicationRepository.findApprovedByProductId(productId);

        List<ProductEntity.CertificationItem> projection = new ArrayList<>(approved.size());
        for (ProductCertificationApplicationEntity app : approved) {
            ProductEntity.CertificationItem item = new ProductEntity.CertificationItem();
            // 投影的 name 欄位是 CertificationType 的 value（"3701"…），不是中文標籤
            item.setName(String.valueOf(app.getCertificationType()));
            item.setCertNo(app.getCertificateNo());
            projection.add(item);
        }

        Optional<ProductEntity> current = productRepository.findByIdIncludingDeleted(productId);
        if (current.isEmpty()) {
            log.warn("rebuildProjection.productMissing", "商品不存在，投影重建略過",
                    "product_id", String.valueOf(productId));
            return false;
        }
        if (sameProjection(current.get().getCertifications(), projection)) {
            return false;
        }
        int rows = productRepository.updateCertificationProjection(productId, projection);
        if (rows == 0) {
            log.warn("rebuildProjection.noRows", "投影覆寫影響 0 列",
                    "product_id", String.valueOf(productId));
            return false;
        }
        log.info("rebuildProjection.success", "認證公開投影已重建，approved=" + projection.size(),
                "product_id", String.valueOf(productId));
        return true;
    }

    private boolean sameProjection(List<ProductEntity.CertificationItem> a,
                                   List<ProductEntity.CertificationItem> b) {
        List<ProductEntity.CertificationItem> left = a == null ? List.of() : a;
        if (left.size() != b.size()) {
            return false;
        }
        for (int i = 0; i < b.size(); i++) {
            if (!Objects.equals(left.get(i).getName(), b.get(i).getName())
                    || !Objects.equals(left.get(i).getCertNo(), b.get(i).getCertNo())) {
                return false;
            }
        }
        return true;
    }

    // ==================== §6.6 核准 transaction ====================

    /**
     * 系統自動核准入口（§6.3 Step 6 同步快取命中／§8.3 背景查驗補課共用）。
     *
     * <p>資格三條缺一不可：{@code gov_verify_state = VERIFIED}、{@code requires_manual_review = false}、
     * 該 type 允許自動核准（§8.5，實測只有 3701／3702）。
     *
     * <p>CAS 用<b>讀取當下的 version</b>；失敗（期間已被人工裁決）⇒ <b>放棄，不重試、不報錯</b>
     * ——人工裁決優先，而且系統路徑拋例外會分別造成「MQ 無限重投」與「商品儲存整批失敗」。
     *
     * @return true = 已核准
     */
    @Transactional(propagation = Propagation.REQUIRED)
    public boolean approveBySystem(Long applicationId) {
        // notifySeller=true：這是「背景補課核准」的入口（唯一的 bean 外呼叫端是
        // CertificationVerifyConsumer）。賣家的畫面停在「審核中」，不通知他永遠不會知道
        // 已經通過了（§12.3）——與 syncOne／doSubmitForSeller 的即刻核准剛好相反。
        return approveBySystemInternal(applicationId, true);
    }

    /**
     * 政府查驗回的業者名稱是否屬於這個賣家（§8.5.1）。
     *
     * <p><b>不通過時把這一筆標成 {@code requires_manual_review}</b>，而不是只回 false：
     * 只回 false 會讓它維持 SUBMITTED，下一次背景查驗或賣家重送時**又會走一次自動核准**，
     * 同一個判斷反覆做、同一個漏洞反覆有機會被撞開。標記之後這筆永久只能由人裁決（§6.3 Step 3 同語意）。
     *
     * <p>⚠️ 四種不通過的成因（確定不符／賣家無已通過 KYC／RPC 失敗／payload 無業者名稱）
     * <b>一律導向人工審核，不得導向拒絕</b>——拒絕會退回本設計書要取代的舊行為（§0.2）。
     */
    private boolean holderBelongsToSeller(ProductCertificationApplicationEntity app) {
        String holderName = app.getGovVerifyPayload() == null
                ? null
                : app.getGovVerifyPayload().getHolderName();
        String reason;
        if (holderName == null || holderName.isBlank()) {
            // 查驗通過卻沒帶業者名稱 ⇒ 無從歸屬，同樣不放行（3702 就沒有這個欄位）。
            reason = "NO_HOLDER_NAME";
        } else {
            SellerGrpcClient.HolderMatch match =
                    sellerGrpcClient.matchCertificateHolder(app.getStoreId(), holderName);
            if (match.allowsAutoApprove()) {
                return true;
            }
            reason = !match.available() ? "LOOKUP_UNAVAILABLE"
                    : !match.hasReference() ? "SELLER_HAS_NO_VERIFIED_NAME"
                    : "HOLDER_MISMATCH";
        }
        markRequiresManualReview(app, reason);
        return false;
    }

    /** 標記這筆只能人工裁決，並留下原因（不記名稱本身——那是 PII）。 */
    private void markRequiresManualReview(ProductCertificationApplicationEntity app, String reason) {
        int rows = applicationRepository.markRequiresManualReview(app.getId());
        log.info("certification.autoApprove.blockedByHolderMismatch",
                "業者名稱歸屬未通過，轉人工審核: reason=" + reason
                        + " store_id=" + app.getStoreId() + " marked_rows=" + rows,
                "application_id", String.valueOf(app.getId()));
    }

    /**
     * {@link #approveBySystem} 的實作；同 bean 內的呼叫端一律走這支，避免 self-invocation 假交易。
     *
     * @param notifySeller 是否發 {@code product.certification.approved}。
     *                     🔴 這個旗標就是 §12.3「同步快取命中的即刻核准<b>不發</b>、背景補課核准<b>要發</b>」
     *                     的唯一實作點：兩條路徑共用同一段核准實作，差別只在這裡。
     */
    private boolean approveBySystemInternal(Long applicationId, boolean notifySeller) {
        Optional<ProductCertificationApplicationEntity> locked =
                applicationRepository.findByIdForUpdate(applicationId);
        if (locked.isEmpty()) {
            return false;
        }
        ProductCertificationApplicationEntity app = locked.get();

        if (!ProductCertificationStatus.SUBMITTED.value().equals(app.getStatus())
                || Boolean.TRUE.equals(app.getRequiresManualReview())
                || !ProductCertificationGovVerifyState.VERIFIED.value().equals(app.getGovVerifyState())
                || !ProductCertificationRules.isAutoApprovable(
                        CertificationType.fromValue(String.valueOf(app.getCertificationType())))) {
            return false;
        }

        // 🔴 第五個條件（§8.5.1）：這張證書必須是**這個賣家的**。
        //    前四條只證明「這個編號存在且有效」——政府 API 不回答「屬於誰」。
        //    少了這一問，任何賣家填入一個查得到的他人編號就會被自動核准並公開標章
        //    （2026-08-27 實測：1-009-300022／持有人 Acme Road LLC 被自動核准到別家的文旦禮盒上）。
        if (!holderBelongsToSeller(app)) {
            return false;
        }

        boolean ok = approveInternal(app, app.getVersion(),
                ProductCertificationReviewAction.AUTO_APPROVE.value(),
                ProductCertificationRules.ACTOR_SYSTEM, null, null, null, null);
        if (!ok) {
            // 持有 FOR UPDATE 的情況下 CAS 結構上不會失敗；真的失敗只可能是同交易內的重入，
            // 一律跳過該筆、不拋例外（不得讓商品儲存整批失敗）。
            log.warn("approveBySystem.casFailed", "系統自動核准 CAS 失敗，已放棄",
                    "application_id", String.valueOf(applicationId));
            return false;
        }
        if (notifySeller) {
            publishDecidedAfterCommit(app, ProductEventType.PRODUCT_CERTIFICATION_APPROVED,
                    ProductCertificationDecidedPayload.DECIDED_BY_SYSTEM, null, null);
        }
        return true;
    }

    /**
     * ADM 人工核准（§9.2）。
     *
     * <p>CAS 用<b>使用者帶的 {@code expectedVersion}</b>；失敗 ⇒ <b>拋 {@code 3026} 讓交易 rollback</b>。
     * 🔴 這裡不得照本服務「business error 走 onNext + code 不拋例外」的既有慣例：
     * Step 4 已做的 SUPERSEDE 會被 commit ⇒ 賣家原本的待審申請被吞掉而沒有新申請。
     */
    @Transactional
    public ProductCertificationApplicationEntity approveForAdmin(Long applicationId,
                                                                 Long expectedVersion,
                                                                 Long adminAccountId,
                                                                 String publicReason,
                                                                 String internalNote,
                                                                 String idempotencyKey) {
        return runIdempotent(idempotencyKey, adminAccountId, "ApproveProductCertificationForAdmin",
                List.of(String.valueOf(applicationId), String.valueOf(expectedVersion)),
                () -> doApproveForAdmin(applicationId, expectedVersion, adminAccountId,
                        publicReason, internalNote));
    }

    private ProductCertificationApplicationEntity doApproveForAdmin(Long applicationId,
                                                                    Long expectedVersion,
                                                                    Long adminAccountId,
                                                                    String publicReason,
                                                                    String internalNote) {
        ProductCertificationApplicationEntity app = lockForReview(applicationId, expectedVersion,
                ProductCertificationStatus.APPROVED.value());

        boolean ok = approveInternal(app, expectedVersion,
                ProductCertificationReviewAction.APPROVE.value(),
                ProductCertificationRules.ACTOR_ADMIN, adminAccountId,
                null, publicReason, internalNote);
        if (!ok) {
            throw new ProductBusinessException(GrpcResponseCode.CERT_APPLICATION_VERSION_CONFLICT);
        }
        publishDecidedAfterCommit(app, ProductEventType.PRODUCT_CERTIFICATION_APPROVED,
                ProductCertificationDecidedPayload.DECIDED_BY_ADMIN, null, publicReason);
        evictAfterCommit(app.getProductId(), app.getStoreId());
        return requireFresh(applicationId);
    }

    /**
     * 核准的共用實作（人工／系統兩個入口，§6.6）。呼叫端必須已持有該列的 {@code FOR UPDATE} 鎖。
     *
     * <p>🔴 <b>①② 的順序不可交換</b>：{@code uk_pca_active} 是 partial unique <b>index</b>，
     * PostgreSQL 無法 deferrable、<b>逐句即時檢查</b>。先做 ② 再做 ①，那一瞬間同
     * {@code (product_id, type)} 有兩列 APPROVED ⇒ <b>23505 ⇒ 整個交易 abort</b>。
     * 而 §6.4 的 type 粒度就是為了讓舊 APPROVED 與新 SUBMITTED 並存
     * ⇒ <b>換號核准是主路徑，不是邊角情境</b>。
     *
     * <p>⚠️ ① 在 ② 之前執行，所以 ② 失敗會留下「舊的被撤了、新的沒核准」。呼叫端必須先
     * 持有 {@code FOR UPDATE} 並確認 status／version 相符（{@link #lockForReview}）——
     * 鎖住之後沒有第三方能改這一列，② 的 CAS 結構上不會失敗。系統路徑不拋例外，靠的就是這個保證。
     */
    private boolean approveInternal(ProductCertificationApplicationEntity app,
                                    Long expectedVersion,
                                    String action,
                                    String actorRole,
                                    Long actorAccountId,
                                    String reasonCode,
                                    String publicReason,
                                    String internalNote) {
        LocalDateTime now = DateTimeUtils.nowUtc();

        // ① 同 type 舊 APPROVED → SUPERSEDED（必須在前）
        Optional<ProductCertificationApplicationEntity> old = applicationRepository
                .findApprovedByProductAndType(app.getProductId(), app.getCertificationType());
        if (old.isPresent() && !old.get().getId().equals(app.getId())) {
            ProductCertificationApplicationEntity superseded = old.get();
            int rows = applicationRepository.transitionToTerminalStatus(
                    List.of(superseded.getId()),
                    ProductCertificationStatus.APPROVED.value(),
                    ProductCertificationStatus.SUPERSEDED.value(),
                    now);
            if (rows > 0) {
                writeEvent(superseded, ProductCertificationReviewAction.SUPERSEDE.value(),
                        ProductCertificationStatus.APPROVED.value(),
                        ProductCertificationStatus.SUPERSEDED.value(),
                        ProductCertificationRules.ACTOR_SYSTEM, null, null, null, null,
                        superseded.getVersion() + 1);
            }
        }

        // ② 本筆 SUBMITTED → APPROVED（必須在後）
        // reviewed_by_account_id：自動核准為 null（actor 由 review_event 的 SYSTEM 標示，§4.1）
        int rows = applicationRepository.casTransitionStatus(
                app.getId(), expectedVersion,
                ProductCertificationStatus.SUBMITTED.value(),
                ProductCertificationStatus.APPROVED.value(),
                now, actorAccountId, reasonCode, publicReason, internalNote);
        if (rows == 0) {
            return false;
        }

        writeEvent(app, action,
                ProductCertificationStatus.SUBMITTED.value(),
                ProductCertificationStatus.APPROVED.value(),
                actorRole, actorAccountId, reasonCode, publicReason, internalNote,
                expectedVersion + 1);

        // §7.2 觸發點：APPROVE（人工＋系統）＋ 連帶的 SUPERSEDE
        rebuildProjection(app.getProductId());
        return true;
    }

    /** ADM 駁回（§9.2）：{@code SUBMITTED → REJECTED}，{@code reasonCode} 必填。 */
    @Transactional
    public ProductCertificationApplicationEntity rejectForAdmin(Long applicationId,
                                                                Long expectedVersion,
                                                                Long adminAccountId,
                                                                String reasonCode,
                                                                String publicReason,
                                                                String internalNote,
                                                                String idempotencyKey) {
        return runIdempotent(idempotencyKey, adminAccountId, "RejectProductCertificationForAdmin",
                List.of(String.valueOf(applicationId), String.valueOf(expectedVersion)),
                () -> doRejectForAdmin(applicationId, expectedVersion, adminAccountId,
                        reasonCode, publicReason, internalNote));
    }

    private ProductCertificationApplicationEntity doRejectForAdmin(Long applicationId,
                                                                   Long expectedVersion,
                                                                   Long adminAccountId,
                                                                   String reasonCode,
                                                                   String publicReason,
                                                                   String internalNote) {
        requireReasonCode(reasonCode);
        ProductCertificationApplicationEntity app = lockForReview(applicationId, expectedVersion,
                ProductCertificationStatus.REJECTED.value());

        int rows = applicationRepository.casTransitionStatus(
                applicationId, expectedVersion,
                ProductCertificationStatus.SUBMITTED.value(),
                ProductCertificationStatus.REJECTED.value(),
                DateTimeUtils.nowUtc(), adminAccountId, reasonCode, publicReason, internalNote);
        if (rows == 0) {
            throw new ProductBusinessException(GrpcResponseCode.CERT_APPLICATION_VERSION_CONFLICT);
        }
        writeEvent(app, ProductCertificationReviewAction.REJECT.value(),
                ProductCertificationStatus.SUBMITTED.value(),
                ProductCertificationStatus.REJECTED.value(),
                ProductCertificationRules.ACTOR_ADMIN, adminAccountId,
                reasonCode, publicReason, internalNote, expectedVersion + 1);

        // REJECT 不改 approved 集合，但為統一亦呼叫（§7.2；集合沒變即 no-op）
        rebuildProjection(app.getProductId());
        publishDecidedAfterCommit(app, ProductEventType.PRODUCT_CERTIFICATION_REJECTED,
                ProductCertificationDecidedPayload.DECIDED_BY_ADMIN, reasonCode, publicReason);
        evictAfterCommit(app.getProductId(), app.getStoreId());
        return requireFresh(applicationId);
    }

    /**
     * ADM 撤銷（§9.2）：{@code APPROVED → REVOKED}，{@code reasonCode} 必填。
     *
     * <p><b>唯一的治理手段</b>——政府 API 只驗「號碼存在」不驗「歸屬」⇒ 盜用他人證號會自動
     * 通過查驗；缺了 REVOKED 平台對已公開的假標章毫無辦法。
     */
    @Transactional
    public ProductCertificationApplicationEntity revokeForAdmin(Long applicationId,
                                                                Long expectedVersion,
                                                                Long adminAccountId,
                                                                String reasonCode,
                                                                String publicReason,
                                                                String internalNote,
                                                                String idempotencyKey) {
        return runIdempotent(idempotencyKey, adminAccountId, "RevokeProductCertificationForAdmin",
                List.of(String.valueOf(applicationId), String.valueOf(expectedVersion)),
                () -> doRevokeForAdmin(applicationId, expectedVersion, adminAccountId,
                        reasonCode, publicReason, internalNote));
    }

    private ProductCertificationApplicationEntity doRevokeForAdmin(Long applicationId,
                                                                   Long expectedVersion,
                                                                   Long adminAccountId,
                                                                   String reasonCode,
                                                                   String publicReason,
                                                                   String internalNote) {
        requireReasonCode(reasonCode);
        ProductCertificationApplicationEntity app = lockForReview(applicationId, expectedVersion,
                ProductCertificationStatus.REVOKED.value());

        int rows = applicationRepository.casTransitionStatus(
                applicationId, expectedVersion,
                ProductCertificationStatus.APPROVED.value(),
                ProductCertificationStatus.REVOKED.value(),
                DateTimeUtils.nowUtc(), adminAccountId, reasonCode, publicReason, internalNote);
        if (rows == 0) {
            throw new ProductBusinessException(GrpcResponseCode.CERT_APPLICATION_VERSION_CONFLICT);
        }
        writeEvent(app, ProductCertificationReviewAction.REVOKE.value(),
                ProductCertificationStatus.APPROVED.value(),
                ProductCertificationStatus.REVOKED.value(),
                ProductCertificationRules.ACTOR_ADMIN, adminAccountId,
                reasonCode, publicReason, internalNote, expectedVersion + 1);

        // §7.2 觸發點：REVOKE（標章必須立刻從公開投影消失）
        rebuildProjection(app.getProductId());
        publishDecidedAfterCommit(app, ProductEventType.PRODUCT_CERTIFICATION_REVOKED,
                ProductCertificationDecidedPayload.DECIDED_BY_ADMIN, reasonCode, publicReason);
        evictAfterCommit(app.getProductId(), app.getStoreId());
        return requireFresh(applicationId);
    }

    // ==================== §9.1 賣家 ====================

    /**
     * 賣家專用申請端點（§9.1 {@code POST .../certification_applications}）。
     *
     * <p>與商品寫入路徑的差別：<b>單筆明確動作 ⇒ 一律回 typed 業務碼</b>（賣家看得懂該重試什麼），
     * 而不是靜默跳過落進 warnings。併發衝突回 {@code 3029}、值域不合法回 {@code 3002}／{@code 3001}。
     *
     * <p>🔴 <b>Gateway 對這支端點刻意不套剔除</b>（§6.2.9 末段），值域交給這裡以業務碼回答：
     * 單筆申請被靜默剔掉，賣家看到的是「送出成功但列表沒有這筆」——比回一個 typed 錯誤更糟。
     * 剔除只適用於批次路徑（商品 create／update 內嵌的認證）。
     */
    @Transactional
    public ProductCertificationApplicationEntity submitForSeller(Long productId,
                                                                 Long storeId,
                                                                 Long accountId,
                                                                 String certificationTypeValue,
                                                                 String certificateNo,
                                                                 String idempotencyKey) {
        return runIdempotent(idempotencyKey, storeId, "SubmitProductCertification",
                List.of(String.valueOf(productId), String.valueOf(certificationTypeValue),
                        String.valueOf(certificateNo)),
                () -> doSubmitForSeller(productId, storeId, accountId,
                        certificationTypeValue, certificateNo));
    }

    private ProductCertificationApplicationEntity doSubmitForSeller(Long productId,
                                                                    Long storeId,
                                                                    Long accountId,
                                                                    String certificationTypeValue,
                                                                    String certificateNo) {
        requireOwnedProduct(productId, storeId);

        String type = trimToNull(certificationTypeValue);
        if (type == null || !CodeMetaValidator.isValidValue(CertificationType.class, type)) {
            throw new ProductBusinessException(GrpcResponseCode.CERT_TYPE_INVALID);
        }
        CertificationType certType = CertificationType.fromValue(type);
        if (certificateNo == null || certificateNo.isBlank()) {
            throw new ProductBusinessException(GrpcResponseCode.CERT_NO_REQUIRED);
        }
        // 與收斂點同一支規則（§6.2.9）：3701–3703 嚴格且對原值判、3704–3706 放寬並回傳 trim 後的值。
        // 🔴 驗什麼就存什麼——底下寫進 DB 的是這個回傳值。
        String certNo = ProductCertificationRules.normalizeCertNo(certType, certificateNo);
        if (certNo == null) {
            throw new ProductBusinessException(GrpcResponseCode.INVALID_PARAM, "認證編號格式不正確");
        }

        Integer typeValue = Integer.valueOf(type);

        // Step 1／Step 2：已核准或已在待審 ⇒ 直接回既有那筆（冪等）
        List<ProductCertificationApplicationEntity> existing =
                applicationRepository.findByProductTypeCertNoAndStatuses(productId, typeValue, certNo,
                        List.of(ProductCertificationStatus.APPROVED.value(),
                                ProductCertificationStatus.SUBMITTED.value()));
        if (!existing.isEmpty()) {
            return existing.get(0);
        }

        boolean requiresManualReview = !applicationRepository.findByProductTypeCertNoAndStatuses(
                productId, typeValue, certNo,
                List.of(ProductCertificationStatus.REJECTED.value(),
                        ProductCertificationStatus.REVOKED.value())).isEmpty();

        supersedeOtherPendingOfSameType(productId, typeValue, certNo);

        GovSnapshot snapshot = resolveGovSnapshotFromCache(new NormalizedCertification(certType, certNo));
        Long applicationId = TsidUtil.generate();
        LocalDateTime now = DateTimeUtils.nowUtc();
        int inserted = applicationRepository.insertSubmittedIfAbsent(
                applicationId, productId, storeId, accountId, typeValue, certNo,
                ProductCertificationSource.SELLER.value(), requiresManualReview,
                snapshot.state(), snapshot.payload(), snapshot.verifiedAt(), now);
        if (inserted == 0) {
            throw new ProductBusinessException(GrpcResponseCode.CERT_APPLICATION_DUPLICATE_PENDING);
        }

        ProductCertificationApplicationEntity created = requireFresh(applicationId);
        writeEvent(created, ProductCertificationReviewAction.SUBMIT.value(),
                null, ProductCertificationStatus.SUBMITTED.value(),
                ProductCertificationRules.ACTOR_SELLER, accountId, null, null, null,
                created.getVersion());

        // notifySeller=false：同 syncOne，即刻核准不推播（§12.3）
        approveBySystemInternal(applicationId, false);
        publishVerifyRequestedIfPending(created, snapshot.state());
        evictAfterCommit(productId, storeId);
        return requireFresh(applicationId);
    }

    /**
     * 賣家撤回（§9.1 {@code POST .../{application_id}/withdraw}）。
     *
     * <p>{@code SUBMITTED}（停止一個進行中的請求）與 {@code APPROVED}（取消勾選、停止宣稱）
     * 都可撤回，兩者都是「賣家自己停止宣稱」而非平台駁回——歸到 REJECTED 會誣指平台駁回，
     * audit 與事實不符（§5.2）。
     */
    @Transactional
    public ProductCertificationApplicationEntity withdrawForSeller(Long productId,
                                                                   Long applicationId,
                                                                   Long storeId,
                                                                   Long accountId,
                                                                   Long expectedVersion,
                                                                   String idempotencyKey) {
        return runIdempotent(idempotencyKey, storeId, "WithdrawProductCertification",
                List.of(String.valueOf(productId), String.valueOf(applicationId),
                        String.valueOf(expectedVersion)),
                () -> doWithdrawForSeller(productId, applicationId, storeId, accountId, expectedVersion));
    }

    private ProductCertificationApplicationEntity doWithdrawForSeller(Long productId,
                                                                      Long applicationId,
                                                                      Long storeId,
                                                                      Long accountId,
                                                                      Long expectedVersion) {
        ProductCertificationApplicationEntity app = applicationRepository.findByIdForUpdate(applicationId)
                .orElseThrow(() -> new ProductBusinessException(GrpcResponseCode.CERT_APPLICATION_NOT_FOUND));
        // 非本人商品 / 對不上的 productId 一律當成不存在（防 IDOR 枚舉）
        if (!Objects.equals(app.getStoreId(), storeId) || !Objects.equals(app.getProductId(), productId)) {
            throw new ProductBusinessException(GrpcResponseCode.CERT_APPLICATION_NOT_FOUND);
        }
        if (!ProductCertificationRules.isLegalTransition(
                app.getStatus(), ProductCertificationStatus.WITHDRAWN.value())) {
            throw new ProductBusinessException(GrpcResponseCode.CERT_APPLICATION_STATE_INVALID);
        }
        if (expectedVersion != null && !expectedVersion.equals(app.getVersion())) {
            throw new ProductBusinessException(GrpcResponseCode.CERT_APPLICATION_VERSION_CONFLICT);
        }

        String fromStatus = app.getStatus();
        long casVersion = expectedVersion != null ? expectedVersion : app.getVersion();
        int rows = applicationRepository.casTransitionStatus(
                applicationId, casVersion, fromStatus,
                ProductCertificationStatus.WITHDRAWN.value(),
                DateTimeUtils.nowUtc(), null, null, null, null);
        if (rows == 0) {
            throw new ProductBusinessException(GrpcResponseCode.CERT_APPLICATION_VERSION_CONFLICT);
        }
        writeEvent(app, ProductCertificationReviewAction.WITHDRAW.value(),
                fromStatus, ProductCertificationStatus.WITHDRAWN.value(),
                ProductCertificationRules.ACTOR_SELLER, accountId, null, null, null,
                casVersion + 1);

        // §7.2 觸發點：WITHDRAW —— 只有「由 APPROVED 轉出」才改變 approved 集合
        if (ProductCertificationStatus.APPROVED.value().equals(fromStatus)) {
            rebuildProjection(app.getProductId());
        }
        evictAfterCommit(app.getProductId(), app.getStoreId());
        return requireFresh(applicationId);
    }

    /** 賣家查自己商品的申請（§9.1）。非本人商品一律 404（防 IDOR 枚舉）。 */
    public Paged<ProductCertificationApplicationEntity> listForSeller(Long productId, Long storeId,
                                                                      String status, int page, int pageSize) {
        requireOwnedProduct(productId, storeId);
        int offset = (Math.max(page, 1) - 1) * pageSize;
        return new Paged<>(
                applicationRepository.findByProductAndStore(productId, storeId, status, pageSize, offset),
                applicationRepository.countByProductAndStore(productId, storeId, status));
    }

    // ==================== §9.2 Admin 讀取 ====================

    public Paged<ProductCertificationApplicationEntity> listForAdmin(String status,
                                                                     Integer certificationType,
                                                                     Long storeId,
                                                                     String keyword,
                                                                     LocalDateTime submittedFrom,
                                                                     LocalDateTime submittedTo,
                                                                     int page, int pageSize) {
        int offset = (Math.max(page, 1) - 1) * pageSize;
        return new Paged<>(
                applicationRepository.findForAdmin(status, certificationType, storeId, keyword,
                        submittedFrom, submittedTo, pageSize, offset),
                applicationRepository.countForAdmin(status, certificationType, storeId, keyword,
                        submittedFrom, submittedTo));
    }

    public ProductCertificationApplicationEntity getForAdmin(Long applicationId) {
        return requireFresh(applicationId);
    }

    /**
     * 同 type 現行有效那筆（§9.2 的 {@code currentApproved}）。
     *
     * <p>🔴 不是裝飾：核准這一筆會讓它被 SUPERSEDE（§6.6 ①），ADM 必須先看得到
     * 「我核准之後，哪一張標章會被換掉」。
     */
    public Optional<ProductCertificationApplicationEntity> findCurrentApproved(
            ProductCertificationApplicationEntity app) {
        Optional<ProductCertificationApplicationEntity> approved = applicationRepository
                .findApprovedByProductAndType(app.getProductId(), app.getCertificationType());
        return approved.filter(a -> !a.getId().equals(app.getId()));
    }

    public Paged<ProductCertificationReviewEventEntity> listAuditLogs(Long applicationId,
                                                                      int page, int pageSize) {
        int offset = (Math.max(page, 1) - 1) * pageSize;
        return new Paged<>(
                reviewEventRepository.findByApplicationId(applicationId, pageSize, offset),
                reviewEventRepository.countByApplicationId(applicationId));
    }

    // ==================== §9.3 ADM 重新查驗 ====================

    /**
     * ADM「重新查驗」（§9.3）：同步跑一次背景查驗邏輯（90 秒 timeout、繞過同步用斷路器），
     * 回寫 {@code gov_verify_state} / {@code gov_verify_payload} / {@code gov_verified_at}。
     *
     * <p>⚠️ <b>本方法刻意不標 {@code @Transactional}</b>：它會發出一個最長 90 秒的外部 HTTP 呼叫，
     * 包在交易裡等於讓連線池的一條連線被外部服務綁住 90 秒。回寫是<b>單一 CAS 語句</b>，
     * 本身即為原子操作，不需要交易邊界。
     *
     * <p>⚠️ <b>只重新查驗、不改 {@code status}</b>：即使結果變成 {@code VERIFIED} 也不自動核准
     * ——ADM 已經在看這一筆了，自動放行會讓人工裁決與系統裁決在同一秒打架。要核准仍走 approve。
     *
     * <p>它也是 §15.3 migration 產生的 legacy 列<b>唯一的查驗觸發方式</b>（那些列沒有 MQ 事件，
     * 而 §8.3 明令禁止掃 {@code PENDING} 的排程）。
     *
     * <p>🔴 <b>繞過結果快取</b>（{@link GovCertificationService#verifyDetailedForceRefresh}）。
     * 讀快取的話這顆按鈕在最需要它的情境下會靜默失效：結果快取對 {@code VALID} 的 TTL 是
     * <b>7 天</b>，而它存在的第一個理由正是「證書可能在申請<b>之後</b>才登記上政府資料庫」
     * ⇒ 賣家補登完，ADM 七天內按幾次都拿到同一個舊答案，
     * 而 {@code version} 照樣 bump、{@code updated_at} 照樣動、{@code gov_verified_at}
     * 卻紋風不動——<b>畫面上沒有任何線索說那是快取</b>。
     * 代價是每按一次都真的消耗政府 API 每日配額，ADM 端已在按鈕旁常駐說明。
     */
    public ProductCertificationApplicationEntity reverifyForAdmin(Long applicationId,
                                                                  Long adminAccountId,
                                                                  String idempotencyKey) {
        return runIdempotent(idempotencyKey, adminAccountId, "ReverifyProductCertificationForAdmin",
                List.of(String.valueOf(applicationId)),
                () -> doReverifyForAdmin(applicationId));
    }

    /**
     * 冷卻窗（§9.3）：同一筆申請在這段時間內只會真的向農業部查詢一次。
     *
     * <p>60 秒的取捨：它要擋的是「連點」與「兩個管理員同時看同一筆」，不是要限制
     * 一天能查幾次——證書可能在申請之後才登記，同一筆本來就會被查很多次（§9.3 理由 1）。
     */
    static final long REVERIFY_COOLDOWN_SECONDS = 60;

    /**
     * 查驗租約的 TTL。**必須 &gt; 單次查驗的最壞耗時**，否則呼叫還在飛租約就先過期，
     * 剛好在最慢的那些請求上失去保護（3702 實測 33–82 秒、背景 read-timeout 90 秒）。
     * 它只是崩潰時的安全網——正常路徑在 finally 主動釋放。
     */
    private static final long REVERIFY_LEASE_TTL_SECONDS = 120;

    private static final String REVERIFY_LEASE_PREFIX = "cert:reverify:lease:";

    private ProductCertificationApplicationEntity doReverifyForAdmin(Long applicationId) {
        requireCooldownElapsed(requireFresh(applicationId));

        // 🔴 租約必須在外部呼叫「之前」取得。只靠 gov_verified_at 的時間差擋不住
        //    「呼叫還在飛」的那 90 秒：第二個管理員在這段期間讀到的仍是舊時間戳
        //    ⇒ 照樣放行 ⇒ 同一筆同時打兩次農業部。那正是本護欄要防的兩個情境。
        String leaseKey = REVERIFY_LEASE_PREFIX + applicationId;
        if (!tryAcquireReverifyLease(leaseKey)) {
            throw cooldownException("另一個查驗正在進行中");
        }
        try {
            return runVerify(applicationId, true);
        } finally {
            // 用完即還（§29：lease 不是永久旗標）。查完之後由 gov_verified_at 的
            // 時間差接手擋剩下的冷卻窗——兩段窗口相接，中間沒有空隙。
            releaseReverifyLease(leaseKey);
        }
    }

    /**
     * 上次查驗未滿冷卻窗 ⇒ 拒絕，<b>不打外部</b>。
     *
     * <p>⚠️ <b>{@code gov_verified_at IS NULL} 一律放行</b>：§15.3 migration 產生的 legacy 列
     * 從未查驗過，而「重新查驗」是它們<b>唯一</b>的查驗觸發方式（§8.3 明令禁止掃
     * {@code PENDING} 的排程）。把首次查驗擋掉等於讓那批列永遠停在待查驗。
     */
    private void requireCooldownElapsed(ProductCertificationApplicationEntity app) {
        LocalDateTime lastVerifiedAt = app.getGovVerifiedAt();
        if (lastVerifiedAt == null) {
            return;
        }
        // 時鐘往回跳（跨 pod skew）時 elapsed 會是負的。夾到 0 而不是放行：
        // 負值代表那個時間戳「剛被寫下」，正是最該擋的時候。
        long elapsed = Math.max(0, ChronoUnit.SECONDS.between(lastVerifiedAt, DateTimeUtils.nowUtc()));
        if (elapsed >= REVERIFY_COOLDOWN_SECONDS) {
            return;
        }
        // elapsed 為 0 是「連按兩下」的常態，而「剛在 0 秒前查驗過」讀起來像壞掉的字串。
        // 最常撞到冷卻的那一次，訊息不能是最難讀的那一句。
        String when = elapsed == 0
                ? "剛剛查驗過"
                : String.format("剛在 %d 秒前查驗過", elapsed);
        throw cooldownException(String.format("%s，約 %d 秒後可再試",
                when, REVERIFY_COOLDOWN_SECONDS - elapsed));
    }

    private ProductBusinessException cooldownException(String reason) {
        return new ProductBusinessException(GrpcResponseCode.CERT_REVERIFY_COOLDOWN,
                String.format(GrpcResponseCode.CERT_REVERIFY_COOLDOWN.message(),
                        reason, REVERIFY_COOLDOWN_SECONDS));
    }

    /**
     * {@code SET key value NX EX ttl}——跨 pod 原子。
     *
     * <p>⚠️ <b>Redis 失聯時 fail-open</b>（沿用 02 §2.7.3 的既有降級契約）：這是配額護欄，
     * 不是正確性不變式。Redis 掛掉時讓管理員完全按不了「重新查驗」，代價比多打幾次
     * 政府 API 大得多——而 {@code gov_verified_at} 的時間差那一關不依賴 Redis，仍然有效。
     */
    private boolean tryAcquireReverifyLease(String leaseKey) {
        try {
            return Boolean.TRUE.equals(redisTemplate.opsForValue().setIfAbsent(
                    leaseKey, "1", REVERIFY_LEASE_TTL_SECONDS, TimeUnit.SECONDS));
        } catch (DataAccessException e) {
            log.warn("reverifyLease.unavailable",
                    "Redis 失聯，重新查驗租約略過（gov_verified_at 冷卻仍生效）: " + e.getMessage());
            return true;
        }
    }

    private void releaseReverifyLease(String leaseKey) {
        try {
            redisTemplate.delete(leaseKey);
        } catch (DataAccessException e) {
            // 沒還成也不會卡死：TTL 到期就自動釋放。
            log.warn("reverifyLease.releaseFailed", "重新查驗租約釋放失敗: " + e.getMessage());
        }
    }

    /**
     * 背景查驗回寫（§8.3）——{@code CertificationVerifyConsumer} 與 ADM「重新查驗」（§9.3）
     * <b>共用同一段實作</b>：跑一次政府查驗（90 秒 timeout、不讀不寫斷路器），
     * 以「讀取當下的 version」CAS 回寫 {@code gov_verify_state} / {@code gov_verify_payload} /
     * {@code gov_verified_at}。
     *
     * <p>⚠️ <b>刻意不標 {@code @Transactional}</b>：內含一個最長 90 秒的外部 HTTP 呼叫，
     * 包進交易等於讓連線池的一條連線被政府 API 綁住 90 秒；回寫本身是單一 CAS 語句，原子性已足。
     *
     * <p>⚠️ <b>只回寫查驗欄位、不改 {@code status}</b>。是否自動核准由呼叫端決定
     * （consumer 接著呼叫 {@link #approveBySystem}；ADM 路徑則刻意不自動核准——
     * ADM 已經在看這一筆了，自動放行會讓人工與系統裁決在同一秒打架）。
     *
     * <p>{@code rowsAffected = 0} ⇒ 期間已被人工裁決 ⇒ <b>放棄回寫、不重試、不報錯</b>（人工裁決優先）。
     *
     * @return 回寫後（或放棄回寫後）的最新申請列
     */
    public ProductCertificationApplicationEntity runBackgroundVerify(Long applicationId) {
        return runVerify(applicationId, false);
    }

    /**
     * 共用實作。{@code forceRefresh} 是<b>兩條路徑唯一的差異</b>：
     * ADM 手動「重新查驗」繞過結果快取（§9.3），MQ 背景查驗照常讀快取
     * （它是「第一次補課」，命中快取正是我們要的省呼叫）。
     */
    private ProductCertificationApplicationEntity runVerify(Long applicationId, boolean forceRefresh) {
        ProductCertificationApplicationEntity app = requireFresh(applicationId);
        CertificationType certType = CertificationType.fromValue(String.valueOf(app.getCertificationType()));

        GovSnapshot snapshot;
        if (!CertificationTypeRules.isVerifiable(certType)) {
            // 3704／3705／3706 無政府開放資料介面 ⇒ 落 UNSUPPORTED，不打任何外部 API
            snapshot = new GovSnapshot(ProductCertificationGovVerifyState.UNSUPPORTED.value(),
                    null, DateTimeUtils.nowUtc());
        } else {
            snapshot = runGovVerify(certType, app.getCertificateNo(), forceRefresh);
        }

        int rows = applicationRepository.casUpdateGovVerifyResult(
                applicationId, app.getVersion(), snapshot.state(), snapshot.payload(), snapshot.verifiedAt());
        if (rows == 0) {
            // 期間已被人工裁決（version 已變）⇒ 放棄回寫，回傳最新狀態讓呼叫端自己看
            log.warn("runVerify.casFailed", "查驗結果回寫時版本已變，略過回寫",
                    "application_id", String.valueOf(applicationId));
        }
        return requireFresh(applicationId);
    }

    /** 實際發出政府查驗呼叫並把三種失敗全部收斂成 {@code UNAVAILABLE}。 */
    private GovSnapshot runGovVerify(CertificationType certType, String certNo, boolean forceRefresh) {
        LocalDateTime now = DateTimeUtils.nowUtc();
        try {
            GovCertificationService.GovVerifyResult result = forceRefresh
                    ? govCertificationService.verifyDetailedForceRefresh(certNo, certType)
                    : govCertificationService.verifyDetailedForBackground(certNo, certType);
            return toSnapshot(result);
        } catch (GovCertificationService.GovApiUnavailableException
                 | GovCertificationService.GovApiResponseException e) {
            // 🔴 上游故障絕不得落成 NOT_FOUND（§8.4 防線 1）：金鑰失效／配額用盡時政府 API
            // 回 HTTP 200 且完全沒有 Data 欄，判成「查無」等於全站認證靜默變成假的。
            log.warn("runGovVerify.govUnavailable", "政府查驗服務不可用: " + e.getMessage());
            return new GovSnapshot(ProductCertificationGovVerifyState.UNAVAILABLE.value(), null, now);
        } catch (RuntimeException e) {
            log.error("runGovVerify.govFailed", "政府查驗呼叫失敗: " + e.getMessage(), e);
            return new GovSnapshot(ProductCertificationGovVerifyState.UNAVAILABLE.value(), null, now);
        }
    }

    // ==================== 內部 helper ====================

    /**
     * §8.2 同步階段：<b>只查結構化快取，一律不發外部呼叫</b>。
     *
     * <p>🔴 同步打政府 API 有致命副作用：{@code GovCertificationService.openCircuit} 在任何
     * timeout／IO 例外時就寫 {@code gov:cert:circuit:{type}}（TTL 60s、per-type、Redis 全域共享）
     * ⇒ 緊接著觸發的背景查驗落在 60 秒窗口內被短路成失敗、根本沒打政府 API
     * ⇒ §8.3 整套機制被同步階段自己封死。零外部呼叫是<b>結構保證</b>，不是約定。
     */
    private GovSnapshot resolveGovSnapshotFromCache(NormalizedCertification cert) {
        LocalDateTime now = DateTimeUtils.nowUtc();
        if (!CertificationTypeRules.isVerifiable(cert.type())) {
            // 3704／3705／3706：直接 UNSUPPORTED，不排背景查驗
            return new GovSnapshot(ProductCertificationGovVerifyState.UNSUPPORTED.value(), null, now);
        }
        GovCertificationService.GovVerifyResult cached =
                govCertificationService.readCachedResult(cert.certNo(), cert.type());
        if (cached == null) {
            // 未命中 ⇒ PENDING ＋ 排入背景查驗（事件發布由背景查驗那一批接線）
            return new GovSnapshot(ProductCertificationGovVerifyState.PENDING.value(), null, null);
        }
        return toSnapshot(cached);
    }

    /** {@code GovVerifyResult}（服務層 record）→ 申請列的三個查驗欄位。 */
    private GovSnapshot toSnapshot(GovCertificationService.GovVerifyResult result) {
        String state = switch (result.validity()) {
            case VALID -> ProductCertificationGovVerifyState.VERIFIED.value();
            case NOT_FOUND -> ProductCertificationGovVerifyState.NOT_FOUND.value();
            case EXPIRED, TERMINATED, SUSPENDED -> ProductCertificationGovVerifyState.EXPIRED.value();
            // UNKNOWN ＝ 我方或上游故障／回應無法判讀，不是查驗結論（§8.3.1）
            case UNKNOWN -> ProductCertificationGovVerifyState.UNAVAILABLE.value();
        };
        LocalDateTime verifiedAt;
        try {
            verifiedAt = result.queriedAt() == null || result.queriedAt().isBlank()
                    ? DateTimeUtils.nowUtc()
                    : DateTimeUtils.parseIso8601(result.queriedAt());
        } catch (RuntimeException e) {
            verifiedAt = DateTimeUtils.nowUtc();
        }
        return new GovSnapshot(state, toPayload(result), verifiedAt);
    }

    private GovVerifyPayload toPayload(GovCertificationService.GovVerifyResult result) {
        GovVerifyPayload payload = new GovVerifyPayload();
        payload.setCertType(result.certType());
        payload.setCertNo(result.certNo());
        payload.setQueriedAt(result.queriedAt());
        payload.setFound(result.found());
        payload.setValidity(result.validity() == null ? null : result.validity().name());
        payload.setStatusRaw(result.statusRaw());
        payload.setExpiresOn(result.expiresOn());
        payload.setHolderName(result.holderName());
        payload.setHolderAddress(result.holderAddress());
        payload.setIssuerName(result.issuerName());
        payload.setProductSummary(result.productSummary());
        payload.setMatchedCount(result.matchedCount());
        if (result.rawExcerpt() != null) {
            try {
                payload.setRawExcerptJson(objectMapper.writeValueAsString(result.rawExcerpt()));
            } catch (RuntimeException | com.fasterxml.jackson.core.JsonProcessingException e) {
                log.warn("toPayload.excerptSerializeFailed",
                        "政府查驗 rawExcerpt 序列化失敗，僅略過該欄: " + e.getMessage());
            }
        }
        return payload;
    }

    /**
     * 冪等外殼（§16.3）：ADM 裁決重送 ＋ 賣家重複送出共用。
     *
     * <p><b>為什麼六支端點都要</b>：{@code idempotency_key} 在 Gateway 是
     * {@code binding:"required"}，收了卻不消費就是「欄位存在但沒人用」——
     * 契約說會冪等，實際不會，而失效只在使用者連點兩次時才現形。
     *
     * <p>各自防的東西不同：
     * <ul>
     *   <li><b>reverify</b>：每一次執行都真的打政府 API 並消耗每日配額——它<b>繞過結果快取</b>
     *       （§9.3），所以第二次不會被快取默默吸收掉。它也<b>沒有</b> {@code expectedVersion}
     *       可以擋，冪等鍵是它唯一的保護。
     *       <br>⚠️ 但要看清楚那把鍵擋的是什麼：ADM 端刻意<b>每次點擊都換一把新鍵</b>
     *       （{@code newIdempotencyKey("cert-reverify")}），因為「證書可能在申請之後才登記」
     *       ⇒ 同一筆本來就會被查很多次。所以冪等鍵收斂的是<b>同一個請求被重送</b>
     *       （連線重試、瀏覽器重送），<b>不是</b>使用者刻意按第二次；後者由 UI 的
     *       {@code disabled={reverifyMutation.isPending}} 擋住送出中的重複點擊。</li>
     *   <li><b>approve／reject／revoke</b>：{@code expectedVersion} CAS 已讓第二次拿到 3026，
     *       冪等鍵讓它改拿到「與第一次相同的成功結果」，這才是使用者按兩次時該有的行為。</li>
     *   <li><b>submit／withdraw</b>：Step 1／2 的 no-op 已提供業務層冪等，冪等鍵補上
     *       「同一把 key 但內容不同」的偵測（{@code requestHash} 比對）。</li>
     * </ul>
     *
     * <p>⚠️ <b>重放時重讀當前狀態</b>而不是回放快照：冪等記錄存的是申請 ID
     * （{@link CertificationApplicationRefDTO}），申請在那之後可能已被裁決／被換號取代。
     *
     * <p>⚠️ {@code idempotencyKey} 為空時直接執行：Gateway 已強制必填，這裡不再多立一道
     * 會讓 gRPC 直呼（縱深防禦路徑）整個不可用的門檻。
     */
    private ProductCertificationApplicationEntity runIdempotent(
            String idempotencyKey, Long subjectId, String endpoint, Object requestSignature,
            Supplier<ProductCertificationApplicationEntity> action) {
        if (idempotencyKey == null || idempotencyKey.isBlank()) {
            log.warn("runIdempotent.missingKey", "未帶 idempotency_key，直接執行", "endpoint", endpoint);
            return action.get();
        }
        String requestHash = idempotencyService.calculateHash(requestSignature);
        Optional<CertificationApplicationRefDTO> cached = idempotencyService.acquireLock(
                idempotencyKey, subjectId, endpoint, requestHash, CertificationApplicationRefDTO.class);
        if (cached.isPresent() && cached.get().applicationId() != null) {
            log.info("runIdempotent.replay", "冪等重放，重讀當前狀態", "endpoint", endpoint);
            return requireFresh(Long.parseLong(cached.get().applicationId()));
        }
        try {
            ProductCertificationApplicationEntity result = action.get();
            idempotencyService.markSucceeded(idempotencyKey, subjectId,
                    new CertificationApplicationRefDTO(String.valueOf(result.getId())));
            return result;
        } catch (RuntimeException e) {
            // markFailed 走 REQUIRES_NEW，外層 rollback 也留得下失敗標記
            idempotencyService.markFailed(idempotencyKey, subjectId);
            throw e;
        }
    }

    /** 鎖定申請列並套狀態機守門 ＋ 版本檢查；三種失敗各有自己的業務碼。 */
    private ProductCertificationApplicationEntity lockForReview(Long applicationId,
                                                                Long expectedVersion,
                                                                String toStatus) {
        ProductCertificationApplicationEntity app = applicationRepository.findByIdForUpdate(applicationId)
                .orElseThrow(() -> new ProductBusinessException(GrpcResponseCode.CERT_APPLICATION_NOT_FOUND));
        if (!ProductCertificationRules.isLegalTransition(app.getStatus(), toStatus)) {
            throw new ProductBusinessException(GrpcResponseCode.CERT_APPLICATION_STATE_INVALID);
        }
        if (expectedVersion == null || !expectedVersion.equals(app.getVersion())) {
            throw new ProductBusinessException(GrpcResponseCode.CERT_APPLICATION_VERSION_CONFLICT);
        }
        return app;
    }

    private void requireReasonCode(String reasonCode) {
        if (reasonCode == null || reasonCode.isBlank()) {
            throw new ProductBusinessException(GrpcResponseCode.CERT_REVIEW_REASON_REQUIRED);
        }
        CodeMetaValidator.requireValidValue(
                ProductCertificationRejectReason.class, reasonCode, "reasonCode");
    }

    private void requireOwnedProduct(Long productId, Long storeId) {
        if (productRepository.findByIdAndStoreId(productId, storeId).isEmpty()) {
            throw new ProductBusinessException(GrpcResponseCode.PRODUCT_NOT_FOUND);
        }
    }

    private ProductCertificationApplicationEntity requireFresh(Long applicationId) {
        return applicationRepository.findById(applicationId)
                .orElseThrow(() -> new ProductBusinessException(GrpcResponseCode.CERT_APPLICATION_NOT_FOUND));
    }

    /**
     * 寫一筆 append-only 稽核事件（§4.2）。
     *
     * <p>🔴 <b>自動核准也必須寫</b>（{@code actorRole = SYSTEM}、{@code action = AUTO_APPROVE}）。
     * 缺了它，事後查不出「這張標章是誰放行的」，而自動核准正是最需要事後可稽核的路徑。
     */
    /**
     * 商品軟刪時，把該商品**全部** {@code SUBMITTED} 的認證申請轉成 {@code WITHDRAWN}（#2820）。
     *
     * <p><b>為什麼是 WITHDRAWN 而不是新狀態</b>：碼表把四個終態的差別寫得很清楚——
     * {@code WITHDRAWN} ＝「賣家自己停止宣稱」，且已涵蓋「該 type 在全量取代中完全沒送」
     * 這種**隱性**停止。賣家刪掉商品是更強的隱性停止，語意完全落在既有值域內。
     * 發明新狀態要動 CodeMeta ＋ {@code ck_pca_status} 的 migration ＋ FSM 三處，是純技術債。
     *
     * <p><b>為什麼不動 APPROVED</b>：{@code REVOKED} 是平台對假標章的**唯一治理手段**
     * （政府 API 只驗號碼存在、不驗歸屬，盜用他人證號會自動通過查驗）。自動清掉已核准的
     * 申請等於給賣家一條「刪商品即抹除治理紀錄」的路。已軟刪商品沒有公開投影，標章不會
     * 顯示給任何人，留著只影響 ADM 的稽核視圖 —— 那正是要留的。
     *
     * <p>不重建投影：已軟刪商品的 {@code product.certifications} 不會被任何公開路徑讀到，
     * 而 {@code SUBMITTED} 本來就不在投影內（投影的唯一輸入是 {@code APPROVED}）⇒ 投影未變。
     *
     * <p>呼叫端必須在**同一個事務**內（{@code ProductService.softDeleteProduct} 是
     * {@code @Transactional}，跨 bean 呼叫故 proxy 生效）。
     *
     * <p><b>為什麼 actor_role 是 SYSTEM 而不是 SELLER</b>：賣家刪的是**商品**，不是撤回申請——
     * 撤回是平台施加的**連帶結果**，與 {@code SUPERSEDE}（核准新號碼時連帶退場舊號碼）同構，
     * 那一支也是 {@code SYSTEM}。稽核問「賣家撤回了這個宣稱嗎」時答 SELLER 是誤導。
     * 且 {@code DeleteProductRequest} 只帶 {@code store_id}，而 {@code SELLER} 的契約要求
     * {@code actor_account_id} 是登入 principal、{@code SYSTEM} 的契約明寫「**不得拿
     * store_id 頂替**」⇒ 記成 SELLER 無論填什麼都違約。
     *
     * <p>⚠️ 與 §6.4「SUBMITTED 刻意不在撤回範圍內（待審是進行中的請求，只能由明確 withdraw
     * 撤銷）」不衝突：那裡的觸發是「賣家送了新的認證清單、剛好沒含這個 type」，語意曖昧所以
     * 保留待審；這裡的觸發是商品本身消失，待審的宣稱已無標的可依附。
     *
     * @param productId 被軟刪的商品
     * @return 實際轉成 WITHDRAWN 的筆數
     */
    public int withdrawSubmittedOnProductDeleted(Long productId) {
        List<ProductCertificationApplicationEntity> pending =
                applicationRepository.lockSubmittedByProductId(productId);
        if (pending.isEmpty()) {
            return 0;
        }
        LocalDateTime now = DateTimeUtils.nowUtc();
        int withdrawn = 0;
        for (ProductCertificationApplicationEntity app : pending) {
            int rows = applicationRepository.transitionToTerminalStatus(
                    List.of(app.getId()),
                    ProductCertificationStatus.SUBMITTED.value(),
                    ProductCertificationStatus.WITHDRAWN.value(),
                    now);
            if (rows > 0) {
                writeEvent(app, ProductCertificationReviewAction.WITHDRAW.value(),
                        ProductCertificationStatus.SUBMITTED.value(),
                        ProductCertificationStatus.WITHDRAWN.value(),
                        ProductCertificationRules.ACTOR_SYSTEM, null,
                        null, null, "商品已軟刪除，連帶停止該商品的認證宣稱（#2820）",
                        app.getVersion() + 1);
                withdrawn++;
            }
        }
        log.info("ProductCertificationApplicationService.withdrawSubmittedOnProductDeleted.success",
                "商品軟刪連帶撤回待審申請，product_id=" + productId + "，withdrawn=" + withdrawn);
        return withdrawn;
    }

    private void writeEvent(ProductCertificationApplicationEntity app,
                            String action, String fromStatus, String toStatus,
                            String actorRole, Long actorAccountId,
                            String reasonCode, String publicReason, String internalNote,
                            Long applicationVersion) {
        ProductCertificationReviewEventEntity event = new ProductCertificationReviewEventEntity();
        event.setApplicationId(app.getId());
        event.setAction(action);
        event.setFromStatus(fromStatus);
        event.setToStatus(toStatus);
        event.setActorRole(actorRole);
        event.setActorAccountId(actorAccountId);
        event.setCertificationType(app.getCertificationType());
        event.setCertificateNo(app.getCertificateNo());
        event.setReasonCode(reasonCode);
        event.setPublicReason(publicReason);
        event.setInternalNote(internalNote);
        event.setApplicationVersion(applicationVersion);
        event.setTraceId(currentTraceId());
        reviewEventRepository.insert(event);
    }

    private String currentTraceId() {
        String traceId = MDC.get(ObsKeys.TRACE_ID);
        return traceId == null || traceId.isBlank() ? null : traceId;
    }

    // ==================== §12 MQ 事件 ====================

    /**
     * §12.1：新建的 {@code SUBMITTED} 申請若停在 {@code PENDING}（同步快取未命中），
     * 於 afterCommit 發 {@code product.certification.verify.requested} 排背景查驗。
     *
     * <p>只有 {@code PENDING} 才發：{@code VERIFIED} 已在同一交易內即刻核准、
     * {@code UNSUPPORTED}（3704–3706）沒有政府開放資料介面可查，兩者發出去只會讓 consumer 空跑。
     *
     * <p>⚠️ {@code requiresManualReview = true} 的列<b>仍然要發</b>（§6.3 Step 3）：
     * 不得讓重送繞過 ADM 裁決是「不自動核准」，不是「不查驗」——ADM 重審需要政府資料當依據。
     * 那個旗標帶在 payload 裡讓 consumer 自己判斷要不要自動核准。
     */
    private void publishVerifyRequestedIfPending(ProductCertificationApplicationEntity app,
                                                 String govVerifyState) {
        if (!ProductCertificationGovVerifyState.PENDING.value().equals(govVerifyState)) {
            return;
        }
        eventPublisher.publishCertificationVerifyRequested(new ProductCertificationVerifyRequestedPayload(
                IdGenerator.uuidV4Lower(),
                ProductEventType.PRODUCT_CERTIFICATION_VERIFY_REQUESTED.value(),
                String.valueOf(app.getId()),
                String.valueOf(app.getProductId()),
                String.valueOf(app.getStoreId()),
                String.valueOf(app.getCertificationType()),
                app.getCertificateNo(),
                Boolean.TRUE.equals(app.getRequiresManualReview()),
                DateTimeUtils.formatIso8601(DateTimeUtils.nowUtc())));
    }

    /**
     * §12.3／§12.4：核准／駁回／撤銷後通知賣家（於 afterCommit 送出）。
     *
     * <p>🔴 <b>{@code sellerAccountId} 走 fail-closed 解析</b>（{@link SellerGrpcClient#getSellerMetaOrThrow}），
     * 而且<b>刻意在交易 commit 之前</b>解析：
     * <ul>
     *   <li>用失敗回空 Map 的 {@code batchGetSellerMetas} ⇒ 發出<b>缺收件人</b>的 payload ⇒
     *       通知靜默蒸發，而事件已「成功」發出，重送救不回來。</li>
     *   <li>把解析放進 afterCommit ⇒ 解析失敗時裁決已 commit、事件卻永遠不會發出，
     *       同樣是無聲失敗。放在 commit 前，解析失敗 ⇒ 整個裁決交易 rollback ⇒
     *       ADM 收到錯誤（或 consumer 進 MQ retry）⇒ 重試整個動作，沒有東西被靜默吞掉。</li>
     * </ul>
     *
     * <p>⚠️ 收件人是<b>登入 principal</b> {@code account_id}，不是 {@code store_id}。
     * 首任店家兩者恰好相等 ⇒ 拿 storeId 頂替在 dev 與 fixture 全綠，
     * 只有 re-key 過的店家收不到通知，而且沒有任何告警（CLAUDE.md §6）。
     *
     * <p>🔴 {@code internalNote} 絕不進 payload（僅 ADM 可見，§4.1）——本方法連參數都不收。
     */
    private void publishDecidedAfterCommit(ProductCertificationApplicationEntity app,
                                           ProductEventType eventType,
                                           String decidedBy,
                                           String reasonCode,
                                           String publicReason) {
        long sellerAccountId = sellerGrpcClient.getSellerMetaOrThrow(app.getStoreId()).getAccountId();
        // 商品不存在（或名稱為空）時回空字串而非 null：payload 是 NON_NULL 序列化，
        // null 會讓 product_name 這個 key 直接消失，notification 端的 #{productName} 就渲染不出來。
        String productName = productRepository.findByIdIncludingDeleted(app.getProductId())
                .map(ProductEntity::getName)
                .orElse("");
        if (productName == null) {
            productName = "";
        }

        ProductCertificationDecidedPayload payload = new ProductCertificationDecidedPayload(
                IdGenerator.uuidV4Lower(),
                eventType.value(),
                String.valueOf(app.getId()),
                String.valueOf(app.getProductId()),
                productName,
                String.valueOf(app.getStoreId()),
                String.valueOf(sellerAccountId),
                String.valueOf(app.getCertificationType()),
                app.getCertificateNo(),
                decidedBy,
                reasonCode,
                publicReason,
                DateTimeUtils.formatIso8601(DateTimeUtils.nowUtc()));

        switch (eventType) {
            case PRODUCT_CERTIFICATION_APPROVED -> eventPublisher.publishCertificationApproved(payload);
            case PRODUCT_CERTIFICATION_REJECTED -> eventPublisher.publishCertificationRejected(payload);
            case PRODUCT_CERTIFICATION_REVOKED -> eventPublisher.publishCertificationRevoked(payload);
            default -> throw new IllegalArgumentException("非認證裁決事件: " + eventType);
        }
    }

    /**
     * §13 快取失效：核准／撤銷／撤回<b>不經</b> {@code ProductService.updateProduct}，
     * evict 不會自動觸發，必須顯式清，而且要<b>在 commit 之後</b>（避免 rollback 後清了個寂寞、
     * 或清完又被舊值填回）。
     *
     * <p>🔴 商品列表必須走 {@code evictProductListForWrite(storeId)} 而不是
     * {@code evictProductList(storeId)}：公開列表 cache key 的 ownerId <b>恆為 0</b>，
     * 與賣家桶不同桶；只清賣家桶會讓
     * {@code GET /public/products?certification_types=3702} stale 到 TTL 到期。
     *
     * <p>前提是 §7.2 的投影已重建 —— 否則清快取只是讓它重讀<b>同一份 stale JSONB</b>。
     */
    private void evictAfterCommit(Long productId, Long storeId) {
        if (!TransactionSynchronizationManager.isSynchronizationActive()) {
            doEvict(productId, storeId);
            return;
        }
        TransactionSynchronizationManager.registerSynchronization(new TransactionSynchronization() {
            @Override
            public void afterCommit() {
                doEvict(productId, storeId);
            }
        });
    }

    private void doEvict(Long productId, Long storeId) {
        try {
            cacheService.evictProductDetail(String.valueOf(productId));
            cacheService.evictProductListForWrite(storeId);
            revalidationService.revalidateProduct(productId);
        } catch (RuntimeException e) {
            // 失敗不阻斷業務；TTL 自然收斂（§16.2）
            log.warn("evictAfterCommit.failed", "認證裁決後清快取失敗: " + e.getMessage(),
                    "product_id", String.valueOf(productId));
        }
    }

    private static String trimToNull(String value) {
        if (value == null) {
            return null;
        }
        String trimmed = value.trim();
        return trimmed.isEmpty() ? null : trimmed;
    }

    // ==================== 內部型別 ====================

    /** 通過第二道防線的一筆認證（type 已解析成 enum、certNo 已 trim）。 */
    private record NormalizedCertification(CertificationType type, String certNo) {
    }

    /** 申請列的三個查驗欄位（{@code gov_verify_state} / {@code gov_verify_payload} / {@code gov_verified_at}）。 */
    private record GovSnapshot(String state, GovVerifyPayload payload, LocalDateTime verifiedAt) {
    }

}
