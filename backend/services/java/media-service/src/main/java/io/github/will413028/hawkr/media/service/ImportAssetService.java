package io.github.will413028.hawkr.media.service;

import io.github.will413028.hawkr.common.config.MediaUrlProperties;
import io.github.will413028.hawkr.common.mq.MqConstants;
import io.github.will413028.hawkr.common.obs.log.ObsLogger;
import io.github.will413028.hawkr.common.obs.mq.ObsRabbitPublisher;
import io.github.will413028.hawkr.common.safefetch.SafeFetchException;
import io.github.will413028.hawkr.common.safefetch.SafeFetchRequest;
import io.github.will413028.hawkr.common.safefetch.SafeFetchResponse;
import io.github.will413028.hawkr.common.safefetch.SafeFetcher;
import io.github.will413028.hawkr.common.utils.code.CodeMetaValidator;
import io.github.will413028.hawkr.common.grpc.PrincipalResolution;
import io.github.will413028.hawkr.common.grpc.UnresolvedPrincipalException;
import io.github.will413028.hawkr.common.utils.DateTimeUtils;
import io.github.will413028.hawkr.common.utils.TsidUtil;
import io.github.will413028.hawkr.common.utils.code.AssetReferenceType;
import io.github.will413028.hawkr.common.utils.code.ImportAssetFailureReason;
import io.github.will413028.hawkr.common.utils.code.ImportAssetMediaStatus;
import io.github.will413028.hawkr.common.utils.code.MediaEventType;
import io.github.will413028.hawkr.common.utils.code.MediaUploadSessionStatus;
import io.github.will413028.hawkr.media.client.SellerGrpcClient;
import io.github.will413028.hawkr.media.dto.StagedTokenClaims;
import io.github.will413028.hawkr.media.entity.ImportAssetEntity;
import io.github.will413028.hawkr.media.entity.MediaAssetEntity;
import io.github.will413028.hawkr.media.entity.UploadSessionEntity;
import io.github.will413028.hawkr.media.exception.MediaServiceException;
import io.github.will413028.hawkr.common.mq.events.MediaImportAssetDownloadedEvent;
import io.github.will413028.hawkr.media.mq.event.InternalDownloadTaskEvent;
import io.github.will413028.hawkr.media.repository.ImportAssetRepository;
import io.github.will413028.hawkr.media.repository.MediaAssetRepository;
import io.github.will413028.hawkr.media.repository.UploadSessionRepository;
import io.github.will413028.hawkr.media.storage.StorageService;
import io.github.will413028.hawkr.media.util.AssetFilename;
import com.fasterxml.jackson.core.JsonProcessingException;
import com.fasterxml.jackson.databind.ObjectMapper;
import jakarta.annotation.PostConstruct;
import lombok.RequiredArgsConstructor;
import org.springframework.beans.factory.annotation.Value;
import org.springframework.dao.DuplicateKeyException;
import org.springframework.stereotype.Service;
import org.springframework.transaction.PlatformTransactionManager;
import org.springframework.transaction.annotation.Transactional;
import org.springframework.transaction.support.TransactionSynchronization;
import org.springframework.transaction.support.TransactionSynchronizationManager;
import org.springframework.transaction.support.TransactionTemplate;

import java.io.ByteArrayInputStream;
import java.nio.charset.StandardCharsets;
import java.security.MessageDigest;
import java.security.NoSuchAlgorithmException;
import java.time.Duration;
import java.time.LocalDateTime;
import java.time.format.DateTimeParseException;
import java.util.HexFormat;
import java.util.List;
import java.util.Map;
import java.util.Optional;
import java.util.Set;
import java.util.UUID;

/**
 * Product Import 委派下載 / 轉正 服務（08 號設計書 §7.2 / §7.3）。
 *
 * <p>本 Service 提供三個進入點：</p>
 * <ol>
 *   <li>{@link #importDownloadAsset}：同步 RPC 入口 — INSERT import_asset + afterCommit publish 內部派工事件</li>
 *   <li>{@link #finalizeImportAsset}：同步 RPC 入口 — OSS 搬遷 + 建 media_asset + UPDATE 轉正</li>
 *   <li>{@link #processInternalDownload}：MQ worker 端執行 SafeFetch + OSS tmp 上傳 + UPDATE + callback publish</li>
 * </ol>
 *
 * <p>對應 docs/開發規範/Java 微服務開發規範.md §8、docs/開發規範/後端服務責任邊界規範.md、docs/開發規範/MQ 與事件管理規範.md — 統一走 ObsRabbitPublisher、寫入路徑同事務、不引入補償任務。</p>
 */
@Service
@RequiredArgsConstructor
public class ImportAssetService {

    private static final ObsLogger log = ObsLogger.forClass(ImportAssetService.class);
    private static final ObjectMapper MAPPER = new ObjectMapper();
    private static final Set<String> ALLOWED_CONTENT_TYPES = Set.of(
        "image/jpeg",
        "image/png",
        "image/webp"
    );

    /**
     * 內部派工事件的 obs 標籤（純觀測用，不對外）。
     * <p>對外 callback 走 {@link MediaEventType#MEDIA_IMPORT_ASSET_DOWNLOADED}（CodeMeta 一條真實事件）；
     * 內部 worker queue 僅用此 string 作 trace 標籤，避免污染 CodeMeta（docs/代碼管理（CodeMeta）/README.md 核心原則 4：純內部值不納管）。</p>
     */
    private static final String INTERNAL_EVENT_TAG = "media.import.asset.download.internal";

    private final ImportAssetRepository importAssetRepository;
    private final MediaAssetRepository mediaAssetRepository;
    private final UploadSessionRepository uploadSessionRepository;
    private final StorageService storageService;
    private final SafeFetcher safeFetcher;
    private final StagedTokenService stagedTokenService;
    private final ObsRabbitPublisher obsRabbitPublisher;
    private final MediaUrlProperties mediaUrlProperties;
    private final PlatformTransactionManager transactionManager;
    private final SellerGrpcClient sellerGrpcClient;
    /** Worker / Finalize 內各階段自行控制獨立事務（避免內部 this.* 呼叫繞過 @Transactional proxy）。 */
    private TransactionTemplate txTemplate;
    private TransactionTemplate txTemplateNew;

    @Value("${media.import.download.max-file-size-bytes:10485760}")
    private long maxFileSizeBytes;

    @Value("${media.import.download.connect-timeout-seconds:5}")
    private int connectTimeoutSeconds;

    @Value("${media.import.download.read-timeout-seconds:30}")
    private int readTimeoutSeconds;

    @Value("${media.import.download.max-redirect-hops:3}")
    private int maxRedirectHops;

    @Value("${media.import.download.user-agent:HawkrImportBot/1.0 (+https://example.com/bot)}")
    private String userAgent;

    @Value("${media.oss.temp-prefix:tmp}")
    private String tempPrefix;

    @PostConstruct
    public void verifyConfig() {
        if (maxFileSizeBytes <= 0) {
            throw new IllegalStateException(
                "media.import.download.max-file-size-bytes 必須大於 0，當前 = " + maxFileSizeBytes);
        }
        this.txTemplate = new TransactionTemplate(transactionManager);
        this.txTemplateNew = new TransactionTemplate(transactionManager);
        this.txTemplateNew.setPropagationBehavior(
            org.springframework.transaction.TransactionDefinition.PROPAGATION_REQUIRES_NEW);
    }

    // ============================================================================
    // §7.2 ImportDownloadAsset（async 派工）
    // ============================================================================

    /**
     * 派工結果 DTO（RPC handler 用）。
     *
     * <p>dedup 命中且 Media 已 settled 時必須 echo 完整 settled metadata,讓 caller 同步 settle 自己 row,
     * 不依賴 callback(40 號 §5.4.1 step 4 同步 settle 分支 / 08 號 §7.2 dedup 命中 echo 規則)。
     *
     * @param importAssetId   import_asset.id（UUID）
     * @param status          當下 status value（{@link ImportAssetMediaStatus} value：DOWNLOADING(9401) / UPLOADED(9402) / FINALIZED(9403) / FAILED(9404) / EXPIRED(9405)）
     * @param reused          true = dedup 命中既有 row（未 INSERT、未 enqueue）；false = 新 INSERT 並已排程派工
     * @param objectKey       dedup 命中且 status ∈ {UPLOADED, FINALIZED} 時填；否則 null
     * @param previewUrl      同 objectKey 條件填
     * @param failureReason   dedup 命中且 status = FAILED 時填；否則 null
     * @param failureDetail   dedup 命中且 status = FAILED 時填
     */
    public record DispatchResult(
        String importAssetId,
        String status,
        boolean reused,
        String objectKey,
        String previewUrl,
        String failureReason,
        String failureDetail
    ) {
        /** 首次 INSERT 派工成功 — 一律 DOWNLOADING,無 settled metadata。 */
        public static DispatchResult freshDispatch(String importAssetId) {
            return new DispatchResult(importAssetId,
                ImportAssetMediaStatus.DOWNLOADING.value(), false,
                null, null, null, null);
        }

        /** dedup 命中 — 依 row 當下 status 帶上對應 settled metadata。 */
        public static DispatchResult fromDedupHit(ImportAssetEntity row) {
            String status = row.getStatus();
            boolean settledOk = ImportAssetMediaStatus.UPLOADED.value().equals(status)
                || ImportAssetMediaStatus.FINALIZED.value().equals(status);
            boolean settledFail = ImportAssetMediaStatus.FAILED.value().equals(status);
            return new DispatchResult(
                row.getId(),
                status,
                true,
                settledOk ? row.getObjectKey() : null,
                settledOk ? row.getPreviewUrl() : null,
                settledFail ? row.getFailureReason() : null,
                settledFail ? row.getFailureDetail() : null
            );
        }
    }

    /**
     * 派工流程（08 號設計書 §7.2 step 1~5）。同步 RPC 階段，毫秒級完成。
     * <p>步驟 4 publish 內部派工 event 走 afterCommit hook，確保 DB commit 才發。</p>
     *
     * @param refType        AssetReferenceType value（PRODUCT_IMPORT_PREVIEW / PRODUCT_IMPORT_PREVIEW_DESC）
     * @param sourceUrl      賣家提供的外部 URL（已 normalize）
     * @param storeId        product-import-service 傳入的賣家 store_id
     * @param importJobId    import_job.id
     * @param importItemId   import_item.id
     * @param expiresAtIso   ISO 8601 UTC，asset 生命週期終點（通常 ImportJob.expires_at + 1d）
     * @param callbackCorrelationId  Import 端 product_import_item_asset.id（callback echo 用）
     */
    @Transactional
    public DispatchResult importDownloadAsset(
        String refType,
        String sourceUrl,
        Long storeId,
        Long importJobId,
        Long importItemId,
        String expiresAtIso,
        Long callbackCorrelationId
    ) {
        // 入參校驗
        if (sourceUrl == null || sourceUrl.isBlank()) {
            throw MediaServiceException.invalidArgument("source_url 不可為空");
        }
        if (storeId == null || storeId <= 0) {
            throw MediaServiceException.invalidArgument("store_id 必填且大於 0");
        }
        if (importJobId == null || importJobId <= 0) {
            throw MediaServiceException.invalidArgument("import_job_id 必填且大於 0");
        }
        if (importItemId == null || importItemId <= 0) {
            throw MediaServiceException.invalidArgument("import_item_id 必填且大於 0");
        }
        if (callbackCorrelationId == null || callbackCorrelationId <= 0) {
            throw MediaServiceException.invalidArgument("callback_correlation_id 必填且大於 0");
        }
        // refType 必須是 import preview 系列；CodeMeta 合法值驗證
        try {
            CodeMetaValidator.requireValidValue(AssetReferenceType.class, refType, "ref_type");
        } catch (IllegalArgumentException ex) {
            throw MediaServiceException.invalidArgument(ex.getMessage());
        }
        if (!AssetReferenceType.PRODUCT_IMPORT_PREVIEW.value().equals(refType)
            && !AssetReferenceType.PRODUCT_IMPORT_PREVIEW_DESC.value().equals(refType)) {
            throw MediaServiceException.invalidArgument(
                "ref_type 必須是 PRODUCT_IMPORT_PREVIEW 或 PRODUCT_IMPORT_PREVIEW_DESC，當前 = " + refType);
        }
        LocalDateTime expiresAt = parseIso8601OrThrow(expiresAtIso, "expires_at");

        String sourceUrlHash = sha256Hex(sourceUrl);

        // Step 1：dedup 檢查（同 store + job + url hash）
        // Settled (UPLOADED/FINALIZED/FAILED) 時 echo metadata,讓 caller 同步 settle 自己 row,
        // 不依賴 callback (40 號 §5.4.1 step 4)。
        Optional<ImportAssetEntity> existing = importAssetRepository.findByDedupKey(
            storeId, importJobId, sourceUrlHash);
        if (existing.isPresent()) {
            ImportAssetEntity row = existing.get();
            log.info("ImportAssetService.dispatch.dedup",
                "命中既有 import_asset，回傳當下 status + settled metadata",
                "import_asset_id",
                "import_asset_id=" + row.getId() + ", status=" + row.getStatus());
            return DispatchResult.fromDedupHit(row);
        }

        // Step 2-3：INSERT；以 try-catch DuplicateKeyException 處理並發 race（UNIQUE constraint 兜底）
        ImportAssetEntity entity = new ImportAssetEntity();
        entity.setId(UUID.randomUUID().toString());
        entity.setStoreId(storeId);
        entity.setImportJobId(importJobId);
        entity.setImportItemId(importItemId);
        entity.setCallbackCorrelationId(callbackCorrelationId);
        entity.setSourceType(ImportAssetEntity.SOURCE_TYPE_URL);
        entity.setSourceUrl(sourceUrl);
        entity.setSourceUrlHash(sourceUrlHash);
        entity.setRefType(refType);
        entity.setStatus(ImportAssetMediaStatus.DOWNLOADING.value());
        entity.setExpiresAt(expiresAt);

        try {
            importAssetRepository.insert(entity);
        } catch (DuplicateKeyException ex) {
            // 並發另一執行緒搶先 INSERT → re-fetch 回現有 row
            log.info("ImportAssetService.dispatch.raceWin",
                "INSERT 撞 UNIQUE，re-fetch 既有 row",
                "import_job_id",
                "import_job_id=" + importJobId + ", source_url_hash=" + sourceUrlHash);
            ImportAssetEntity row = importAssetRepository.findByDedupKey(
                storeId, importJobId, sourceUrlHash)
                .orElseThrow(() -> MediaServiceException.internal(
                    "DuplicateKeyException 後 dedup re-fetch 失敗，import_job_id="
                        + importJobId));
            return DispatchResult.fromDedupHit(row);
        }

        // Step 4：afterCommit hook — DB commit 之後再 publish 內部派工事件
        final String importAssetId = entity.getId();
        registerAfterCommitPublish(importAssetId);

        log.info("ImportAssetService.dispatch.scheduled",
            "派工成功，等待 commit 後 enqueue worker",
            "import_asset_id",
            "import_asset_id=" + importAssetId + ", import_job_id=" + importJobId);

        return DispatchResult.freshDispatch(importAssetId);
    }

    /**
     * 註冊 afterCommit hook 將內部派工事件發到 worker queue。
     * <p>commit 之後才發，避免「MQ 已 publish 但 DB rollback」的孤兒事件。</p>
     */
    private void registerAfterCommitPublish(String importAssetId) {
        if (!TransactionSynchronizationManager.isSynchronizationActive()) {
            // 罕見：呼叫端忘記掛事務，fallback 立刻 publish + 警告
            log.warn("ImportAssetService.dispatch.noTransaction",
                "import_asset INSERT 未在事務內，立即 publish（不保證 commit 順序）",
                "import_asset_id", importAssetId);
            publishInternalDownloadTask(importAssetId);
            return;
        }
        TransactionSynchronizationManager.registerSynchronization(new TransactionSynchronization() {
            @Override
            public void afterCommit() {
                publishInternalDownloadTask(importAssetId);
            }
        });
    }

    private void publishInternalDownloadTask(String importAssetId) {
        try {
            byte[] body = MAPPER.writeValueAsBytes(new InternalDownloadTaskEvent(importAssetId));
            obsRabbitPublisher.publish(
                MqConstants.EXCHANGE_MEDIA,
                MqConstants.RK_MEDIA_IMPORT_ASSET_DOWNLOAD_INTERNAL,
                body,
                INTERNAL_EVENT_TAG
            );
            log.info("ImportAssetService.dispatch.enqueued",
                "內部派工事件已 publish 到 worker queue",
                "import_asset_id", importAssetId);
        } catch (JsonProcessingException e) {
            // JSON 序列化失敗（理論上不可能）— 留 EXPIRED 由 GC 兜底
            log.error("ImportAssetService.dispatch.serializeFailed",
                "InternalDownloadTaskEvent 序列化失敗 import_asset_id=" + importAssetId, e);
        } catch (RuntimeException e) {
            // MQ 不可用：worker 不會收到事件 → 由 GC 兜底（過期掃描）
            log.error("ImportAssetService.dispatch.publishFailed",
                "內部派工事件 publish 失敗 import_asset_id=" + importAssetId, e);
        }
    }

    // ============================================================================
    // §7.2b BindStagedAsset（sync，賣家本地上傳路徑）
    // ============================================================================

    /**
     * 綁定結果 DTO。
     */
    public record BindResult(
        String importAssetId,
        String objectKey,
        String previewUrl,
        boolean reused
    ) {
    }

    /**
     * 將賣家本地上傳取得的 staged_token 同步綁定為 import_asset（40 號設計書 §3.4 / §4.3）。
     * <p>流程：parse JWT → 校驗 session COMPLETED + 屬該賣家 → dedup → OSS copy →
     * INSERT import_asset (status=UPLOADED)。</p>
     *
     * <p>同一 (store_id, import_job_id, staged_session_id) 重複呼叫回傳既有 row（idempotent）。</p>
     *
     * @param stagedToken     CompleteUploadSession 回傳的 JWT
     * @param storeId         product-import-service 傳入的賣家 store_id
     * @param importJobId     import_job.id
     * @param importItemId    import_item.id
     * @param refType         AssetReferenceType value（PRODUCT_IMPORT_PREVIEW / PRODUCT_IMPORT_PREVIEW_DESC）
     */
    public BindResult bindStagedAsset(
        String stagedToken,
        Long storeId,
        Long importJobId,
        Long importItemId,
        String refType
    ) {
        if (stagedToken == null || stagedToken.isBlank()) {
            throw MediaServiceException.invalidArgument("staged_token 不可為空");
        }
        if (storeId == null || storeId <= 0) {
            throw MediaServiceException.invalidArgument("store_id 必填且大於 0");
        }
        if (importJobId == null || importJobId <= 0) {
            throw MediaServiceException.invalidArgument("import_job_id 必填且大於 0");
        }
        if (importItemId == null || importItemId <= 0) {
            throw MediaServiceException.invalidArgument("import_item_id 必填且大於 0");
        }
        try {
            CodeMetaValidator.requireValidValue(AssetReferenceType.class, refType, "ref_type");
        } catch (IllegalArgumentException ex) {
            throw MediaServiceException.invalidArgument(ex.getMessage());
        }
        if (!AssetReferenceType.PRODUCT_IMPORT_PREVIEW.value().equals(refType)
            && !AssetReferenceType.PRODUCT_IMPORT_PREVIEW_DESC.value().equals(refType)) {
            throw MediaServiceException.invalidArgument(
                "ref_type 必須是 PRODUCT_IMPORT_PREVIEW 或 PRODUCT_IMPORT_PREVIEW_DESC，當前 = " + refType);
        }

        // 階段 1：parse token + 權限校驗
        StagedTokenClaims claims = stagedTokenService.parseAndValidate(stagedToken);
        // storeId 是 product-import-service 傳入的賣家 store_id（見 ItemMutationService.update:
        // mediaClient.bindStagedAsset(..., storeId, ...)）；staged token 的 claims.accountId
        // 則是上傳當下 GrpcAuthContext.requireAccountId() 取得的 raw account_id。分岔賣家
        // （store_id != account_id）直接相等比對會誤判越權，須先經 seller-service 解析回
        // raw account_id 才能正確比對。
        PrincipalResolution ownerResolution = sellerGrpcClient.resolvePrincipals(List.of(storeId));
        if (ownerResolution.isUnresolved(storeId)) {
            // 查無存活賣家（已關閉或不存在）：沒有擁有者可以比對，下面的比對一律拒絕（fail-closed）。單體的 store_id 與
            // account_id 是兩個序列，不能拿 store_id 冒充 account_id 去比對 staged token 的帳號。
            log.warn("ImportAssetService.bindStagedAsset.storeUnresolved",
                "store_id 查無存活賣家，沒有擁有者可比對，拒絕",
                "accounts", "request=" + storeId + ",token=" + claims.getAccountId());
        }
        // fail-closed：沒有解析到存活的賣家就沒有擁有者可以比對，一律當「不存在」拒絕（不拿 store_id 冒充 account_id 比對）。
        Long resolvedOwnerAccountId = ownerResolution.isUnresolved(storeId) ? null : ownerResolution.principals().get(storeId);
        if (resolvedOwnerAccountId == null || !resolvedOwnerAccountId.equals(claims.getAccountId())) {
            // 越權防枚舉（API 設計規範 §14.1）：回應與「不存在」一致，不洩漏 account_id 線索
            log.warn("ImportAssetService.bindStagedAsset.accountMismatch", "staged_token 帳號不匹配",
                "accounts", "token=" + claims.getAccountId() + ",request=" + storeId
                    + ",resolvedOwner=" + resolvedOwnerAccountId);
            throw MediaServiceException.notFound("staged_token 無效或不存在");
        }
        if (Boolean.TRUE.equals(claims.getIsSecure())) {
            throw MediaServiceException.invalidArgument(
                "機敏檔案不支援 BindStagedAsset（import 路徑限公開上傳）");
        }
        UUID sessionId = claims.getSessionId();

        // 階段 2：dedup 檢查 — 同 (store, job, session_id) 已有 import_asset 直接回
        // 這裡刻意用 storeId 而非 principal：import_asset.store_id 全表存的就是 store_id
        // （URL 路徑見 importDownloadAsset 收 product-import-service 的 storeId、STAGED
        // 路徑見下方 entity.setStoreId(storeId)），dedup 鍵必須與被查的欄位同一個 id 空間。
        // 分岔賣家在這裡最多是 dedup miss 多建一列 import_asset（多一次 OSS copy，不影響
        // 正確性）；擁有者比對那一關由 prepareFinalize 的 isSameSellerPrincipal 負責，
        // 不靠這個鍵把關。
        Optional<ImportAssetEntity> existing = importAssetRepository.findByStagedDedupKey(
            storeId, importJobId, sessionId.toString());
        if (existing.isPresent()) {
            ImportAssetEntity row = existing.get();
            log.info("ImportAssetService.bindStaged.dedup",
                "命中既有 STAGED import_asset，直接回傳",
                "import_asset_id",
                "import_asset_id=" + row.getId() + ", session_id=" + sessionId);
            return new BindResult(row.getId(), row.getObjectKey(), row.getPreviewUrl(), true);
        }

        // 階段 3：校驗 upload_session 狀態
        // 查詢鍵用 resolvedOwnerAccountId（principal）而非 storeId：
        // upload_session.account_id 是上傳當下由 GrpcAuthContext.requireAccountId()
        // 寫入的 raw account_id（UploadSessionService.createSession），拿 store_id 去查
        // 對分岔賣家永遠零筆 → 剛通過上面的越權檢查、卻立刻在這裡收到「upload_session
        // 不存在或無權存取」。此處不必再打一次 seller-service：上面的檢查已經確立
        // resolvedOwnerAccountId == claims.getAccountId()，而 staged token 的 accountId
        // 正是由 session.getAccountId() 簽發的（UploadSessionService.completeSession →
        // stagedTokenService.generateToken(session.getId(), session.getAccountId(), ...)），
        // 兩者同一個 id 空間。
        UploadSessionEntity session =
            uploadSessionRepository.selectByIdAndAccountId(sessionId, resolvedOwnerAccountId);
        if (session == null) {
            throw MediaServiceException.notFound(
                "upload_session 不存在或無權存取 session_id=" + sessionId);
        }
        if (!MediaUploadSessionStatus.COMPLETED.value().equals(session.getStatus())
            && !MediaUploadSessionStatus.INIT.value().equals(session.getStatus())) {
            throw MediaServiceException.failedPrecondition(
                "upload_session 狀態不可綁定 status=" + session.getStatus()
                    + "（預期 COMPLETED）");
        }
        if (session.getObjectKey() == null || session.getObjectKey().isBlank()) {
            throw MediaServiceException.internal(
                "upload_session 缺 object_key session_id=" + sessionId);
        }

        // 階段 4：OSS 搬遷（事務外）— upload session OSS path → import_asset tmp path
        // 這裡刻意用 storeId：它只是 tmp/{date}/{id}/import/ 路徑裡的一段分桶字串，不是
        // 查詢鍵也不是授權依據，與 importDownloadAsset 的 tmp 路徑規則一致以共用 GC。
        // 分岔賣家換屆後新舊路徑不同，但物件位置一律以 object_key 落庫，不靠路徑反推擁有者。
        String newObjectKey = buildStagedTmpObjectKey(storeId, session.getObjectKey());
        try {
            storageService.copyObject(session.getObjectKey(), newObjectKey);
        } catch (MediaServiceException e) {
            log.error("ImportAssetService.bindStaged.ossCopyFailed",
                "OSS copy 失敗 session_id=" + sessionId + " src=" + session.getObjectKey()
                    + " dst=" + newObjectKey, e);
            throw e;
        }

        // 階段 5：INSERT import_asset（DOWNLOADING → UPLOADED 一步到位，無 worker 介入）
        ImportAssetEntity entity = new ImportAssetEntity();
        entity.setId(UUID.randomUUID().toString());
        entity.setStoreId(storeId);
        entity.setImportJobId(importJobId);
        entity.setImportItemId(importItemId);
        entity.setCallbackCorrelationId(0L); // STAGED 路徑無需 callback；填 0 表 N/A
        entity.setSourceType(ImportAssetEntity.SOURCE_TYPE_STAGED);
        entity.setStagedSessionId(sessionId.toString());
        entity.setRefType(refType);
        entity.setStatus(ImportAssetMediaStatus.UPLOADED.value());
        entity.setObjectKey(newObjectKey);
        entity.setContentType(session.getContentType());
        entity.setFileSize(session.getFileSize());
        // expires_at 同 ImportDownloadAsset 設計：通常 ImportJob.expires_at + 1d，這裡 STAGED 不從 caller 收
        // — 走 session.expires_at 為基準（不會超過 session 本身）；若需更長，UpdateImportItem 端可重綁
        entity.setExpiresAt(session.getExpiresAt() != null
            ? session.getExpiresAt()
            : DateTimeUtils.nowUtc().plusDays(8));

        // 預覽 URL（失敗不阻擋）
        try {
            entity.setPreviewUrl(
                storageService.generatePresignedDownloadUrl(newObjectKey, 24 * 3600));
        } catch (RuntimeException e) {
            log.warn("ImportAssetService.bindStaged.presignFailed",
                "預覽 URL 簽名失敗（不影響落 DB） session_id=" + sessionId
                    + ", error=" + e.getClass().getSimpleName() + ": " + e.getMessage());
        }

        try {
            importAssetRepository.insert(entity);
        } catch (DuplicateKeyException ex) {
            // 並發 race：另一執行緒搶先 INSERT → 回 dedup 結果
            log.info("ImportAssetService.bindStaged.raceWin",
                "INSERT 撞 UNIQUE，re-fetch session_id=" + sessionId);
            // 清掉剛搬的 OSS 物件（race 失敗者）
            try {
                storageService.deleteObject(newObjectKey);
            } catch (RuntimeException dx) {
                log.warn("ImportAssetService.bindStaged.cleanupFailed",
                    "race 失敗者清 OSS 失敗（容忍）: " + dx.getMessage(),
                    "object_key", newObjectKey);
            }
            ImportAssetEntity row = importAssetRepository.findByStagedDedupKey(
                storeId, importJobId, sessionId.toString())
                .orElseThrow(() -> MediaServiceException.internal(
                    "DuplicateKeyException 後 STAGED dedup re-fetch 失敗 session_id=" + sessionId));
            return new BindResult(row.getId(), row.getObjectKey(), row.getPreviewUrl(), true);
        }

        log.info("ImportAssetService.bindStaged.success",
            "STAGED import_asset 綁定完成",
            "import_asset_id",
            "import_asset_id=" + entity.getId() + ", session_id=" + sessionId
                + ", object_key=" + newObjectKey);

        return new BindResult(entity.getId(), newObjectKey, entity.getPreviewUrl(), false);
    }

    /**
     * 由 upload_session.object_key 推導 import_asset tmp 路徑。
     * <p>格式：{tempPrefix}/{yyyyMMdd}/{storeId}/import/{uuid}{ext}，與 ImportDownloadAsset 一致以共用 GC。</p>
     */
    private String buildStagedTmpObjectKey(Long storeId, String sourceKey) {
        String dateStr = java.time.LocalDate.now().format(
            java.time.format.DateTimeFormatter.ofPattern("yyyyMMdd"));
        String ext = extractExt(sourceKey);
        return String.format("%s/%s/%d/import/%s%s",
            tempPrefix, dateStr, storeId, UUID.randomUUID(), ext);
    }

    private static String extractExt(String key) {
        if (key == null) return "";
        int slash = key.lastIndexOf('/');
        int dot = key.lastIndexOf('.');
        if (dot < 0 || dot < slash) return "";
        return key.substring(dot);
    }

    // ============================================================================
    // §7.2a Worker：處理內部派工事件（SafeFetch + OSS tmp + UPDATE + callback publish）
    // ============================================================================

    /**
     * Worker 入口（由 {@link io.github.will413028.hawkr.media.mq.MediaImportAssetDownloadListener} 呼叫）。
     *
     * <p>本方法吞例外回 false，由 listener 端決定 ACK / NACK；
     * publishCallback 走 afterCommit hook 確保 DB commit 之後才發出對外事件。</p>
     *
     * @return true = 已正確處理（不論成功 / 失敗結果都已落 DB + publish callback），listener 可以 ACK；
     *         false = 處理過程中出現預期外例外（DB / 系統錯誤），listener 應 NACK 進 DLQ
     */
    public boolean processInternalDownload(String importAssetId) {
        if (importAssetId == null || importAssetId.isBlank()) {
            log.warn("ImportAssetService.worker.invalidEvent", "import_asset_id 為空，跳過");
            return true;
        }
        try {
            // 走 REQUIRES_NEW 確保 worker 每次都拿到獨立事務（避免跟 listener 外層 tx 混淆）
            txTemplateNew.executeWithoutResult(status -> executeInternalDownloadInTx(importAssetId));
            return true;
        } catch (RuntimeException ex) {
            // DB 寫入失敗 / OSS upload 失敗 / 其他系統錯誤 → 進 DLQ 由維運處理
            log.error("ImportAssetService.worker.unhandled",
                "worker 處理 import_asset 例外，將 NACK 進 DLQ import_asset_id=" + importAssetId, ex);
            return false;
        }
    }

    /**
     * Worker 同事務內：SELECT FOR UPDATE → SafeFetch → OSS tmp → UPDATE → afterCommit publish callback。
     * <p>由 {@link #processInternalDownload} 透過 {@link #txTemplateNew} 包裹執行，
     * 不掛 {@code @Transactional}（避免 self this.* 繞過 proxy）。</p>
     */
    private void executeInternalDownloadInTx(String importAssetId) {
        Optional<ImportAssetEntity> opt = importAssetRepository.findByIdForUpdate(importAssetId);
        if (opt.isEmpty()) {
            log.warn("ImportAssetService.worker.notFound",
                "import_asset 不存在，可能已被 GC 清除，跳過",
                "import_asset_id", importAssetId);
            return;
        }
        ImportAssetEntity entity = opt.get();

        // 只有 DOWNLOADING 才處理；其餘狀態（已被 EXPIRED / 已 UPLOADED / FAILED / FINALIZED）視為重入，跳過
        if (!ImportAssetMediaStatus.DOWNLOADING.value().equals(entity.getStatus())) {
            log.info("ImportAssetService.worker.skip",
                "import_asset 已非 DOWNLOADING，視為重入跳過",
                "import_asset_id",
                "import_asset_id=" + importAssetId + ", status=" + entity.getStatus());
            return;
        }

        // 過期保護：若 expires_at 已過，不浪費資源下載
        LocalDateTime now = DateTimeUtils.nowUtc();
        if (entity.getExpiresAt() != null && entity.getExpiresAt().isBefore(now)) {
            log.info("ImportAssetService.worker.expired",
                "import_asset 在處理前已過期，標 EXPIRED 跳過",
                "import_asset_id", importAssetId);
            entity.setStatus(ImportAssetMediaStatus.EXPIRED.value());
            entity.setExpiredAt(now);
            importAssetRepository.updateById(entity);
            return;
        }

        // === SafeFetch ===
        SafeFetchResponse fetchResponse;
        try {
            SafeFetchRequest req = new SafeFetchRequest(
                entity.getSourceUrl(),
                ALLOWED_CONTENT_TYPES,
                maxFileSizeBytes,
                Duration.ofSeconds(connectTimeoutSeconds),
                Duration.ofSeconds(readTimeoutSeconds),
                userAgent,
                maxRedirectHops
            );
            fetchResponse = safeFetcher.fetch(req);
        } catch (SafeFetchException ex) {
            handleDownloadFailure(entity, ex, now);
            return;
        }

        // === OSS tmp 上傳 ===
        String objectKey;
        try {
            objectKey = buildTmpObjectKey(entity, fetchResponse.contentType());
            storageService.uploadStream(
                objectKey,
                new ByteArrayInputStream(fetchResponse.content()),
                fetchResponse.contentType(),
                fetchResponse.actualSizeBytes()
            );
        } catch (RuntimeException ex) {
            // OSS 失敗對齊設計書：標 FAILED + FETCH_ERROR
            log.error("ImportAssetService.worker.ossUploadFailed",
                "OSS 上傳失敗 import_asset_id=" + importAssetId, ex);
            entity.setStatus(ImportAssetMediaStatus.FAILED.value());
            entity.setFailureReason(ImportAssetFailureReason.FETCH_ERROR.value());
            entity.setFailureDetail(truncate(ex.getMessage(), 1000));
            importAssetRepository.updateById(entity);
            registerAfterCommitCallback(entity, ImportAssetMediaStatus.UPLOADED.value(), now, /*success*/ false);
            return;
        }

        // === UPDATE 為 UPLOADED ===
        entity.setStatus(ImportAssetMediaStatus.UPLOADED.value());
        entity.setObjectKey(objectKey);
        entity.setContentType(fetchResponse.contentType());
        entity.setFileSize(fetchResponse.actualSizeBytes());
        // preview_url 走 OSS 簽名（24h）；下載失敗不阻擋主流程
        try {
            entity.setPreviewUrl(storageService.generatePresignedDownloadUrl(objectKey, 24 * 3600));
        } catch (RuntimeException ex) {
            log.warn("ImportAssetService.worker.presignFailed",
                "預覽 URL 簽名失敗（不影響落 DB） import_asset_id=" + importAssetId
                    + ", error=" + ex.getClass().getSimpleName() + ": " + ex.getMessage());
        }
        try {
            importAssetRepository.updateById(entity);

            // === afterCommit publish callback ===
            registerAfterCommitCallback(entity, /*priorStatus*/ ImportAssetMediaStatus.UPLOADED.value(), now, true);
        } catch (RuntimeException ex) {
            // 本 method 由 txTemplateNew 包覆：這裡拋出 = 整個事務 rollback，object_key 不會落庫。
            // 不補償就留下一個沒有任何一張表認得的 tmp 物件（GC 走 DB 列，掃不到它）。
            try {
                storageService.deleteObject(objectKey);
                log.info("ImportAssetService.worker.compensateDeleted",
                    "落庫失敗，已刪除剛上傳的暫存物件", "object_key", objectKey);
            } catch (RuntimeException dx) {
                log.error("ImportAssetService.worker.compensateFailed",
                    "補償刪除失敗，OSS 將留下無主物件 object_key=" + objectKey, dx);
            }
            throw ex;
        }

        log.info("ImportAssetService.worker.uploaded",
            "import_asset 下載 + 上 OSS tmp 完成",
            "import_asset_id",
            "import_asset_id=" + importAssetId + ", object_key=" + objectKey
                + ", size=" + fetchResponse.actualSizeBytes());
    }

    private void handleDownloadFailure(ImportAssetEntity entity, SafeFetchException ex, LocalDateTime now) {
        ImportAssetFailureReason reason = mapSafeFetchReason(ex.reason());
        entity.setStatus(ImportAssetMediaStatus.FAILED.value());
        entity.setFailureReason(reason.value());
        entity.setFailureDetail(truncate(ex.getMessage(), 1000));
        importAssetRepository.updateById(entity);
        registerAfterCommitCallback(entity, ImportAssetMediaStatus.UPLOADED.value(), now, false);

        log.warn("ImportAssetService.worker.fetchFailed",
            "SafeFetch 失敗：" + reason.value() + " (" + ex.reason() + ")",
            "import_asset_id",
            "import_asset_id=" + entity.getId() + ", source_url_hash=" + entity.getSourceUrlHash());
    }

    /**
     * SafeFetchException.Reason → ImportAssetFailureReason 對應（08 號設計書 §7.7）。
     */
    private static ImportAssetFailureReason mapSafeFetchReason(SafeFetchException.Reason r) {
        return switch (r) {
            case PROTOCOL_NOT_ALLOWED,
                 IP_BLOCKED_PRIVATE,
                 IP_BLOCKED_LOOPBACK,
                 IP_BLOCKED_LINK_LOCAL,
                 IP_BLOCKED_MULTICAST,
                 DOMAIN_BLOCKED_INTERNAL,
                 REDIRECT_LIMIT_EXCEEDED,
                 REDIRECT_CROSS_DOMAIN -> ImportAssetFailureReason.SSRF_BLOCKED;
            case SIZE_EXCEEDED -> ImportAssetFailureReason.SIZE_EXCEEDED;
            case TIMEOUT_CONNECT, TIMEOUT_READ -> ImportAssetFailureReason.TIMEOUT;
            case CONTENT_TYPE_NOT_ALLOWED -> ImportAssetFailureReason.UNSUPPORTED_TYPE;
            case NOT_FOUND -> ImportAssetFailureReason.NOT_FOUND;
            case TLS_VERIFY_FAILED, FETCH_ERROR -> ImportAssetFailureReason.FETCH_ERROR;
        };
    }

    /**
     * 在 commit 之後 publish callback event 給 Import Service。
     *
     * @param entity        最新狀態 entity（已 set status / failure 等欄位）
     * @param priorStatus   觸發 publish 的「期望 publish 內容狀態」— UPLOADED / FAILED 文字 only for log（實際看 entity.getStatus()）
     * @param now           完成時間
     * @param success       true = UPLOADED；false = FAILED
     */
    private void registerAfterCommitCallback(ImportAssetEntity entity, String priorStatus,
                                             LocalDateTime now, boolean success) {
        // 取出 publish 需要的 immutable 快照（entity 是 mutable，afterCommit 時可能已被改動）
        MediaImportAssetDownloadedEvent event = new MediaImportAssetDownloadedEvent(
            UUID.randomUUID().toString(),
            entity.getId(),
            entity.getImportJobId(),
            entity.getImportItemId(),
            entity.getCallbackCorrelationId(),
            success
                ? ImportAssetMediaStatus.UPLOADED.value()
                : ImportAssetMediaStatus.FAILED.value(),
            success ? entity.getObjectKey() : null,
            success ? entity.getPreviewUrl() : null,
            success ? entity.getContentType() : null,
            success ? entity.getFileSize() : null,
            success ? null : entity.getFailureReason(),
            success ? null : entity.getFailureDetail(),
            DateTimeUtils.formatIso8601(now)
        );

        if (!TransactionSynchronizationManager.isSynchronizationActive()) {
            publishCallback(event);
            return;
        }
        TransactionSynchronizationManager.registerSynchronization(new TransactionSynchronization() {
            @Override
            public void afterCommit() {
                publishCallback(event);
            }
        });
    }

    private void publishCallback(MediaImportAssetDownloadedEvent event) {
        try {
            byte[] body = MAPPER.writeValueAsBytes(event);
            obsRabbitPublisher.publish(
                MqConstants.EXCHANGE_MEDIA,
                MqConstants.RK_MEDIA_IMPORT_ASSET_DOWNLOADED,
                body,
                MediaEventType.MEDIA_IMPORT_ASSET_DOWNLOADED.value()
            );
            log.info("ImportAssetService.callback.published",
                "MediaImportAssetDownloadedEvent 已 publish",
                "import_asset_id",
                "import_asset_id=" + event.importAssetId() + ", result_status=" + event.resultStatus());
        } catch (JsonProcessingException e) {
            log.error("ImportAssetService.callback.serializeFailed",
                "callback event 序列化失敗 import_asset_id=" + event.importAssetId(), e);
        } catch (RuntimeException e) {
            // callback publish 失敗：DB 已落 UPLOADED/FAILED，Import 端會在 callback 30min timeout 後由 ProductImportGcJob 補強
            log.error("ImportAssetService.callback.publishFailed",
                "callback event publish 失敗 import_asset_id=" + event.importAssetId(), e);
        }
    }

    // ============================================================================
    // §7.3 FinalizeImportAsset（Product 端 commit 階段呼叫）
    // ============================================================================

    /**
     * Finalize 結果 DTO。
     */
    public record FinalizeResult(
        Long assetId,
        String objectKey,
        String url
    ) {
    }

    /**
     * 把 import_asset 搬到正式 refType 路徑 + 建立 media_asset（08 號設計書 §7.3）。
     * <p>OSS copy/delete 在事務外執行（OSS 不可回滾），DB 寫入同事務。</p>
     */
    public FinalizeResult finalizeImportAsset(
        String importAssetId,
        String targetRefType,
        Long callerStoreId,
        Long productId
    ) {
        if (importAssetId == null || importAssetId.isBlank()) {
            throw MediaServiceException.invalidArgument("import_asset_id 不可為空");
        }
        if (callerStoreId == null || callerStoreId <= 0) {
            throw MediaServiceException.invalidArgument("store_id 必填且大於 0");
        }
        if (productId == null || productId <= 0) {
            throw MediaServiceException.invalidArgument("product_id 必填且大於 0");
        }
        // targetRefType 必須是 PRODUCT 或 PRODUCT_DESC
        try {
            CodeMetaValidator.requireValidValue(AssetReferenceType.class, targetRefType, "target_ref_type");
        } catch (IllegalArgumentException ex) {
            throw MediaServiceException.invalidArgument(ex.getMessage());
        }
        if (!AssetReferenceType.PRODUCT.value().equals(targetRefType)
            && !AssetReferenceType.PRODUCT_DESC.value().equals(targetRefType)) {
            throw MediaServiceException.invalidArgument(
                "target_ref_type 必須是 PRODUCT 或 PRODUCT_DESC，當前 = " + targetRefType);
        }

        // 階段 1：DB 內檢查（同事務 + FOR UPDATE）— 走 txTemplate 避免 self this.* 繞過 proxy
        final FinalizeContext ctx = txTemplate.execute(status ->
            prepareFinalize(importAssetId, targetRefType, callerStoreId));
        if (ctx == null) {
            throw MediaServiceException.internal("prepareFinalize 回傳 null import_asset_id=" + importAssetId);
        }
        if (ctx.idempotent) {
            // 已 FINALIZED → 直接回現有 result（idempotent）
            return ctx.result;
        }

        // 階段 2：OSS 操作（事務外）
        try {
            storageService.copyObject(ctx.originalKey, ctx.finalObjectKey);
            storageService.deleteObject(ctx.originalKey);
        } catch (MediaServiceException e) {
            log.error("ImportAssetService.finalize.ossFailed",
                "OSS 搬移失敗 import_asset_id=" + importAssetId, e);
            throw e;
        }

        // 階段 3：DB 寫入 media_asset + UPDATE import_asset（同事務）
        return txTemplate.execute(status -> commitFinalize(ctx));
    }

    /**
     * Step 1: 拿 import_asset FOR UPDATE，做狀態 / 權限校驗 + 生成 finalObjectKey；
     * 已 FINALIZED 則直接拼出 result idempotent 回傳。
     * <p>由 caller 透過 {@link #txTemplate} 包裹執行。</p>
     */
    private FinalizeContext prepareFinalize(String importAssetId, String targetRefType, Long callerStoreId) {
        ImportAssetEntity entity = importAssetRepository.findByIdForUpdate(importAssetId)
            .orElseThrow(() -> MediaServiceException.notFound(
                "import_asset 不存在: " + importAssetId));

        if (!isSameSellerPrincipal(entity.getStoreId(), callerStoreId)) {
            // 越權防枚舉（API 設計規範 §14.1）：回應與「import_asset 不存在」一致
            log.warn("ImportAssetService.prepareFinalize.accountMismatch", "import_asset 帳號不匹配",
                "import_asset_id", importAssetId);
            throw MediaServiceException.notFound("import_asset 不存在: " + importAssetId);
        }

        // 已 FINALIZED → idempotent
        if (ImportAssetMediaStatus.FINALIZED.value().equals(entity.getStatus())) {
            log.info("ImportAssetService.finalize.idempotent",
                "import_asset 已 FINALIZED，重入回傳既有 asset",
                "import_asset_id",
                "import_asset_id=" + importAssetId + ", finalized_asset_id=" + entity.getFinalizedAssetId());
            String url = entity.getFinalizedObjectKey() != null
                ? mediaUrlProperties.buildUrl(entity.getFinalizedObjectKey())
                : "";
            return new FinalizeContext(
                entity, targetRefType,
                /*originalKey*/ null,
                /*finalObjectKey*/ entity.getFinalizedObjectKey(),
                true,
                new FinalizeResult(entity.getFinalizedAssetId(), entity.getFinalizedObjectKey(), url),
                // idempotent 分支不再建列，檔名不會被用到
                /*originalFilename*/ null
            );
        }

        // 否則必須是 UPLOADED 才能 finalize
        if (!ImportAssetMediaStatus.UPLOADED.value().equals(entity.getStatus())) {
            throw MediaServiceException.failedPrecondition(
                "import_asset 尚未下載完成或已失敗，目前 status=" + entity.getStatus()
                    + "（預期 UPLOADED）");
        }
        if (entity.getObjectKey() == null || entity.getObjectKey().isBlank()) {
            throw MediaServiceException.internal(
                "import_asset UPLOADED 但 object_key 為空 import_asset_id=" + importAssetId);
        }

        // generateFinalObjectKey 重用既有 OSS service 邏輯（isSecure 一律 false — import 入口沒有 secure 上傳）
        // 該方法的 ownerId 參數是軸中立的路徑分桶值：這裡傳 store_id，另一呼叫端
        // MediaAssetService 傳真正的登入 account_id，兩者都只影響 object key 的一段字串。
        String finalObjectKey = storageService.generateFinalObjectKey(
            entity.getObjectKey(), /*isSecure*/ false, targetRefType, callerStoreId);

        return new FinalizeContext(entity, targetRefType, entity.getObjectKey(), finalObjectKey,
            false, null, resolveImportDisplayName(entity));
    }

    /**
     * 匯入資產的顯示檔名（#3149，設計書 08 §4.9.2／§4.9.3a）。
     *
     * <p>🔴 <b>先分支判 {@code source_type}，不要靠 catch 兜</b>：STAGED 來源的 CHECK 約束
     * 強制 {@code source_url} 為 NULL，而 {@code URI.create(null)} 拋的是
     * {@link NullPointerException} 而不是 {@link IllegalArgumentException}。
     *
     * <ul>
     *   <li><b>STAGED</b> → 查 {@code upload_session.original_filename}（建立會話時已消毒）。
     *       GC 只把 session 標 EXPIRED、不刪列，所以這裡查得到。</li>
     *   <li><b>URL</b> → 解析 {@code source_url}（規則見 {@link AssetFilename#fromSourceUrl}）。</li>
     * </ul>
     *
     * @return 解不出來即 {@code null}，讀取端回退到 object_key 尾段
     */
    private String resolveImportDisplayName(ImportAssetEntity entity) {
        if (ImportAssetEntity.SOURCE_TYPE_STAGED.equals(entity.getSourceType())) {
            String sessionId = entity.getStagedSessionId();
            if (sessionId == null || sessionId.isBlank()) {
                return null;
            }
            UUID uuid;
            try {
                uuid = UUID.fromString(sessionId);
            } catch (IllegalArgumentException e) {
                log.warn("ImportAssetService.finalize.stagedSessionIdMalformed",
                    "staged_session_id 非合法 UUID，檔名走回退 import_asset_id=" + entity.getId());
                return null;
            }
            UploadSessionEntity session = uploadSessionRepository.selectById(uuid);
            return session == null ? null : session.getOriginalFilename();
        }
        // URL 來源（含其他未知 source_type：source_url 為 NULL 時 fromSourceUrl 回 null）
        return AssetFilename.fromSourceUrl(entity.getSourceUrl());
    }

    /**
     * Finalize 的擁有者比對：兩個 id 是否屬於同一個登入 principal。
     *
     * <p>{@code import_asset.store_id} 存的是「下載/綁定當下」的賣家 store_id
     * （{@link #importDownloadAsset} 收到 product-import-service 傳入的 store_id 後
     * 直接落庫），{@code callerStoreId} 則是 Finalize 當下 product-service 傳入的
     * <b>現任</b> store_id。賣家在 import job 存活期（預設 7 天）內刪店重開時兩者會
     * 分岔（store_id 換新、account_id 不變，44 號設計書買賣家身分分離），直接相等
     * 比對會把合法的同一賣家誤判成越權，回應與「import_asset 不存在」一致而卡死
     * 匯入最終提交。比照同檔 {@link #bindStagedAsset}：先各自解析回登入 principal
     * account_id 再比對。</p>
     *
     * <p>快路徑：兩值相同即同一個 store stint，不必打 seller-service（絕大多數呼叫
     * 走這條，維持每張圖一次 Finalize 的既有成本）。慢路徑硬依賴 seller-service，
     * fail-closed 兩種：RPC 失敗時 {@link SellerGrpcClient#resolvePrincipals} 拋出；
     * 任一側 store_id 查無在架賣家（{@link io.github.will413028.hawkr.common.grpc.PrincipalResolution}
     * 的 unresolved）時本方法回 {@code false} 拒絕——兩種都不會悄悄放行。</p>
     */
    private boolean isSameSellerPrincipal(Long ownerStoreId, Long callerStoreId) {
        if (ownerStoreId == null || callerStoreId == null) {
            return false;
        }
        if (ownerStoreId.equals(callerStoreId)) {
            return true;
        }
        PrincipalResolution resolution =
            sellerGrpcClient.resolvePrincipals(List.of(ownerStoreId, callerStoreId));
        long ownerPrincipal;
        long callerPrincipal;
        try {
            // fail-closed（issue #2789 階段 B1）：任一側查無在架賣家就不比對。
            // seller-service 對查無在架列的 store_id 會把輸入原樣回吐，舊寫法
            // （getOrDefault echo-back）於是拿一個 **store_id 空間的值** 去跟另一側
            // **account_id 空間的值** 比相等——那已經不是身分比對，結果是構造上未定義的，
            // 而這一關放行的後果是拿得到別人的 import_asset。
            // 未解析 ⇔ 該 stint 已軟刪除（sellers.deleted_at 只由帳號刪除寫入）或 id 不存在，
            // 兩種都不該通過擁有權比對；賣家「刪店重開」的舊 stint 只是 is_current=false、
            // deleted_at 仍為 NULL，照樣解析得到，本方法原本要放行的分岔情境不受影響。
            ownerPrincipal = resolution.requireResolved(ownerStoreId, "store_id");
            callerPrincipal = resolution.requireResolved(callerStoreId, "store_id");
        } catch (UnresolvedPrincipalException e) {
            log.warn("ImportAssetService.prepareFinalize.principalUnresolved",
                "store_id 查無在架賣家，無法確認 import_asset 擁有權，fail-closed 拒絕 Finalize: "
                    + e.getMessage(),
                "accounts", "owner=" + ownerStoreId + ",caller=" + callerStoreId);
            return false;
        }
        boolean same = ownerPrincipal == callerPrincipal;
        if (same) {
            log.info("ImportAssetService.prepareFinalize.divergedStint",
                "賣家 store_id 已換屆，解析回同一 principal 後放行",
                "accounts", "owner=" + ownerStoreId + ",caller=" + callerStoreId
                    + ",principal=" + callerPrincipal);
        }
        return same;
    }

    /**
     * Step 3: OSS 已搬移完成 → 同事務 INSERT media_asset + UPDATE import_asset FINALIZED。
     * <p>由 caller 透過 {@link #txTemplate} 包裹執行。</p>
     */
    private FinalizeResult commitFinalize(FinalizeContext ctx) {
        ImportAssetEntity entity = ctx.entity;

        // INSERT media_asset
        // media_asset.account_id 原是刻意保留的多義欄位（一般上傳路徑存真正登入
        // account_id，此 import 路徑存 store_id），2026-08-03 account_id 命名債 Phase 3
        // Task 6 已拆成 uploaderAccountId / importStoreId 兩個各自單義的欄位；此路徑寫的是
        // import_asset 自己的 store_id，故落 importStoreId。
        MediaAssetEntity asset = new MediaAssetEntity();
        asset.setId(TsidUtil.generate());
        asset.setImportStoreId(entity.getStoreId());
        asset.setObjectKey(ctx.finalObjectKey);
        asset.setRefType(ctx.targetRefType);
        asset.setIsSecure(false);
        asset.setMimeType(entity.getContentType() != null ? entity.getContentType() : "application/octet-stream");
        asset.setSizeBytes(entity.getFileSize() != null ? entity.getFileSize() : 0L);
        // 原始檔名（#3149）：prepareFinalize 依 source_type 解出、經 ctx 帶過來
        asset.setOriginalFilename(ctx.originalFilename);
        mediaAssetRepository.insert(asset);

        // UPDATE import_asset → FINALIZED
        LocalDateTime now = DateTimeUtils.nowUtc();
        entity.setStatus(ImportAssetMediaStatus.FINALIZED.value());
        entity.setFinalizedAssetId(asset.getId());
        entity.setFinalizedObjectKey(ctx.finalObjectKey);
        entity.setFinalizedRefType(ctx.targetRefType);
        entity.setFinalizedAt(now);
        importAssetRepository.updateById(entity);

        log.info("ImportAssetService.finalize.success",
            "import_asset 轉正完成",
            "import_asset_id",
            "import_asset_id=" + entity.getId() + ", asset_id=" + asset.getId()
                + ", object_key=" + ctx.finalObjectKey);

        return new FinalizeResult(asset.getId(), ctx.finalObjectKey,
            mediaUrlProperties.buildUrl(ctx.finalObjectKey));
    }

    /**
     * Finalize 跨階段需要傳遞的內部 context（mutable 是有意的）。
     */
    private static final class FinalizeContext {
        final ImportAssetEntity entity;
        final String targetRefType;
        final String originalKey;
        final String finalObjectKey;
        final boolean idempotent;
        final FinalizeResult result;
        /**
         * 顯示用的原始檔名（#3149）。在 {@link #prepareFinalize} 解出來（那裡才有 entity
         * 與 upload_session），帶到 {@link #commitFinalize} 寫進 media_asset。
         * <p>解不出來即 {@code null}——匯入圖片沒有檔名不是錯誤，讀取端會回退到 object_key。
         * 該路徑的 final key 末段是 {@code {uuid}{ext}}（不含檔名），所以這一條與兩支後端
         * 直傳不同，<b>不是</b>零行為變化。
         */
        final String originalFilename;

        FinalizeContext(ImportAssetEntity entity, String targetRefType,
                        String originalKey, String finalObjectKey,
                        boolean idempotent, FinalizeResult result,
                        String originalFilename) {
            this.entity = entity;
            this.targetRefType = targetRefType;
            this.originalKey = originalKey;
            this.finalObjectKey = finalObjectKey;
            this.idempotent = idempotent;
            this.result = result;
            this.originalFilename = originalFilename;
        }
    }

    // ============================================================================
    // §7.5 gc.expire_job consumer
    // ============================================================================

    /**
     * 處理 ImportJob 過期事件：將同 job 下所有非終態 import_asset 標 EXPIRED 並刪 OSS tmp。
     * <p>idempotent（UPDATE 條件 {@code status NOT IN (FINALIZED, EXPIRED)}）。</p>
     */
    @Transactional
    public void expireByJobId(Long importJobId) {
        if (importJobId == null || importJobId <= 0) {
            log.warn("ImportAssetService.gc.invalidJob", "import_job_id 為空，跳過");
            return;
        }
        // 先取「即將被標 EXPIRED」的 row 拿到 object_key（之後 OSS delete 用）
        java.util.List<ImportAssetEntity> targets = importAssetRepository.selectActiveByJobId(importJobId);
        if (targets.isEmpty()) {
            log.info("ImportAssetService.gc.empty",
                "ImportJob 沒有非終態 import_asset，跳過",
                "import_job_id", String.valueOf(importJobId));
            return;
        }

        LocalDateTime now = DateTimeUtils.nowUtc();
        int marked = importAssetRepository.markExpiredByJobId(importJobId, now);

        // 刪 OSS tmp（事務外執行的 OSS 動作即使失敗也只造成 tmp 殘留，由既有 GC 第 4 層兜底）
        for (ImportAssetEntity row : targets) {
            if (row.getObjectKey() != null && !row.getObjectKey().isBlank()) {
                try {
                    storageService.deleteObject(row.getObjectKey());
                } catch (RuntimeException e) {
                    log.warn("ImportAssetService.gc.ossDeleteFailed",
                        "OSS tmp 刪除失敗（容忍）: " + e.getMessage(),
                        "import_asset_id",
                        "import_asset_id=" + row.getId() + ", object_key=" + row.getObjectKey());
                }
            }
        }

        log.info("ImportAssetService.gc.done",
            "ImportJob 過期清理完成",
            "import_job_id",
            "import_job_id=" + importJobId + ", marked_expired=" + marked);
    }

    // ============================================================================
    // §7.6 GC 第 4 層：過期掃描 + 保留期刪除（被 MediaGarbageCollectionJob 呼叫）
    // ============================================================================

    /**
     * 過期未 Finalize：DOWNLOADING / UPLOADED / FAILED 且 expires_at < now → EXPIRED + 刪 OSS tmp。
     *
     * @return 標 EXPIRED 的筆數
     */
    @Transactional
    public int cleanupExpiredCandidates(int batchSize) {
        LocalDateTime now = DateTimeUtils.nowUtc();
        java.util.List<ImportAssetEntity> candidates =
            importAssetRepository.selectExpiredCandidates(now, batchSize);
        if (candidates.isEmpty()) {
            return 0;
        }
        int count = 0;
        for (ImportAssetEntity row : candidates) {
            row.setStatus(ImportAssetMediaStatus.EXPIRED.value());
            row.setExpiredAt(now);
            importAssetRepository.updateById(row);
            if (row.getObjectKey() != null && !row.getObjectKey().isBlank()) {
                try {
                    storageService.deleteObject(row.getObjectKey());
                } catch (RuntimeException e) {
                    log.warn("ImportAssetService.gc.expiredOssFailed",
                        "GC 刪除 OSS tmp 失敗（容忍）: " + e.getMessage(),
                        "import_asset_id", row.getId());
                }
            }
            count++;
        }
        return count;
    }

    /**
     * 已 EXPIRED 滿 retentionDays 的 row → DELETE（保留期過後審計也不再需要）。
     *
     * @return 刪除筆數
     */
    @Transactional
    public int cleanupExpiredRetention(int retentionDays, int batchSize) {
        LocalDateTime threshold = DateTimeUtils.nowUtc().minusDays(retentionDays);
        java.util.List<ImportAssetEntity> candidates =
            importAssetRepository.selectExpiredRetentionCandidates(threshold, batchSize);
        if (candidates.isEmpty()) {
            return 0;
        }
        java.util.List<String> ids = candidates.stream()
            .map(ImportAssetEntity::getId)
            .toList();
        return importAssetRepository.deleteBatchIds(ids);
    }

    // ============================================================================
    // helpers
    // ============================================================================

    private String buildTmpObjectKey(ImportAssetEntity entity, String contentType) {
        // tmp/{yyyyMMdd}/{storeId}/import/{uuid}{ext}
        String dateStr = java.time.LocalDate.now().format(
            java.time.format.DateTimeFormatter.ofPattern("yyyyMMdd"));
        String uuid = UUID.randomUUID().toString();
        String ext = guessExt(contentType);
        return String.format("%s/%s/%d/import/%s%s",
            tempPrefix, dateStr, entity.getStoreId(), uuid, ext);
    }

    private static String guessExt(String contentType) {
        if (contentType == null) return "";
        return switch (contentType.toLowerCase()) {
            case "image/jpeg", "image/jpg" -> ".jpg";
            case "image/png" -> ".png";
            case "image/webp" -> ".webp";
            default -> "";
        };
    }

    private static String sha256Hex(String s) {
        try {
            MessageDigest md = MessageDigest.getInstance("SHA-256");
            byte[] digest = md.digest(s.getBytes(StandardCharsets.UTF_8));
            return HexFormat.of().formatHex(digest);
        } catch (NoSuchAlgorithmException e) {
            throw MediaServiceException.internal("SHA-256 not available", e);
        }
    }

    private static LocalDateTime parseIso8601OrThrow(String iso, String fieldName) {
        if (iso == null || iso.isBlank()) {
            throw MediaServiceException.invalidArgument(fieldName + " 不可為空");
        }
        try {
            return DateTimeUtils.parseIso8601(iso);
        } catch (DateTimeParseException e) {
            throw MediaServiceException.invalidArgument(
                fieldName + " 必須是 ISO 8601 UTC 格式: " + iso);
        }
    }

    private static String truncate(String s, int max) {
        if (s == null) return null;
        return s.length() <= max ? s : s.substring(0, max);
    }
}
