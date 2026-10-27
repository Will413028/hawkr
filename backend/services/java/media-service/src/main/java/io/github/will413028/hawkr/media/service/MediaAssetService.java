package io.github.will413028.hawkr.media.service;

import io.github.will413028.hawkr.common.obs.log.ObsLogger;
import io.github.will413028.hawkr.common.utils.code.AssetReferenceType;
import io.github.will413028.hawkr.common.utils.code.MediaUploadSessionStatus;
import io.github.will413028.hawkr.media.converter.MediaAssetConverter;
import io.github.will413028.hawkr.media.crypto.AesStreamCipher;
import io.github.will413028.hawkr.media.dto.FinalizedAssetDTO;
import io.github.will413028.hawkr.media.dto.SecureAssetContentDTO;
import io.github.will413028.hawkr.media.dto.StagedTokenClaims;
import io.github.will413028.hawkr.media.entity.MediaAssetEntity;
import io.github.will413028.hawkr.media.entity.UploadSessionEntity;
import io.github.will413028.hawkr.media.exception.MediaServiceException;
import io.github.will413028.hawkr.common.utils.code.StagedTokenScope;
import io.github.will413028.hawkr.media.repository.MediaAssetRepository;
import io.github.will413028.hawkr.media.repository.UploadSessionRepository;
import io.github.will413028.hawkr.media.storage.StorageObjectStat;
import io.github.will413028.hawkr.media.storage.StorageService;
import io.github.will413028.hawkr.media.util.AssetFilename;
import org.springframework.stereotype.Service;
import org.springframework.transaction.PlatformTransactionManager;
import org.springframework.transaction.support.TransactionTemplate;

import io.github.will413028.hawkr.common.utils.TsidUtil;

import io.github.will413028.hawkr.common.utils.DateTimeUtils;
import io.github.will413028.hawkr.media.dto.AssetMetadataDTO;

import java.util.List;

import javax.crypto.CipherInputStream;
import java.io.IOException;
import java.io.InputStream;
import java.util.*;

/**
 * 資源資產服務
 * <p>
 * 職責：處理 FinalizeImages，將 Staging 轉為正式資產
 *
 * <p><b>儲存端操作一律在事務外</b>（比照 {@link ImportAssetService#finalizeImportAsset}）：
 * R2 不參與 DB 事務，把 copy/upload 放進 {@code @Transactional} 方法裡，任何後續 rollback
 * 都會留下「R2 有物件、{@code media_asset} 無列」的孤兒。而 {@code MediaGarbageCollectionJob}
 * 掃的是 {@code media_asset LEFT JOIN asset_reference}（見
 * {@link MediaAssetRepository#selectOrphanedAssets}），DB 沒有列就永遠不在回收範圍 ——
 * 這種孤兒只能從生產端堵，回收端結構上救不了。</p>
 */
@Service
public class MediaAssetService {

    private static final ObsLogger log = ObsLogger.forClass(MediaAssetService.class);

    private final MediaAssetRepository mediaAssetRepository;
    private final UploadSessionRepository uploadSessionRepository;
    private final StagedTokenService stagedTokenService;
    private final StorageService storageService;
    private final MediaAssetConverter mediaAssetConverter;
    private final MediaUploadValidator uploadValidator;
    private final AesStreamCipher aesStreamCipher;

    /** 轉正各階段自行控制事務（避免同 class 內 this.* 呼叫繞過 @Transactional proxy）。 */
    private final AssetReferenceService assetReferenceService;
    private final TransactionTemplate txTemplate;

    public MediaAssetService(MediaAssetRepository mediaAssetRepository,
                             UploadSessionRepository uploadSessionRepository,
                             StagedTokenService stagedTokenService,
                             StorageService storageService,
                             MediaAssetConverter mediaAssetConverter,
                             MediaUploadValidator uploadValidator,
                             AesStreamCipher aesStreamCipher,
                             AssetReferenceService assetReferenceService,
                             PlatformTransactionManager transactionManager) {
        this.mediaAssetRepository = mediaAssetRepository;
        this.uploadSessionRepository = uploadSessionRepository;
        this.stagedTokenService = stagedTokenService;
        this.storageService = storageService;
        this.mediaAssetConverter = mediaAssetConverter;
        this.uploadValidator = uploadValidator;
        this.aesStreamCipher = aesStreamCipher;
        this.assetReferenceService = assetReferenceService;
        this.txTemplate = new TransactionTemplate(transactionManager);
    }

    /**
     * 轉正圖片
     * <p>
     * 依設計書 4.1 轉正流程： 1. 驗證 Token 2. 狀態轉換：COMPLETED → FINALIZED 3. OSS 搬移至分類路徑
     * 4. 建立 media_asset 5. 回填 session.asset_id 6. 回傳 mapping
     * <p>
     * 重入語意（設計書 3.2.4）： - 若 staged_token 已完成轉正（FINALIZED），不報錯，回傳既有 asset
     *
     * @param stagedTokens    Staged Token 列表
     * @param callerAccountId 呼叫者帳號 ID（用於權限驗證）
     * @param refType         資源分類，對應 AssetReferenceType（如 PRODUCT、SHIPPING、OTHER）
     * @return 已轉正資產列表
     */
    public List<FinalizedAssetDTO> finalizeImages(
            List<String> stagedTokens,
            Long callerAccountId,
            String refType
    ) {
        log.info("finalize.start", "開始轉正圖片",
                "token_count", String.valueOf(stagedTokens.size()));

        // 階段 1（事務內、唯讀）：逐個 token 驗證並算出正式路徑；已轉正者直接帶既有 asset 回來。
        List<FinalizePlan> plans = txTemplate.execute(status -> {
            List<FinalizePlan> prepared = new ArrayList<>(stagedTokens.size());
            for (String token : stagedTokens) {
                prepared.add(prepareFinalize(token, callerAccountId, refType));
            }
            return prepared;
        });

        // 階段 2（事務外）：HEAD 回查 + copy 到正式路徑。
        // 刻意「先不刪暫存」：暫存要留到 DB commit 之後才刪，否則中途失敗會把來源與副本一起弄丟，
        // session 停在 COMPLETED 卻再也無法重試。
        List<String> copiedKeys = new ArrayList<>();
        try {
            for (FinalizePlan plan : plans) {
                if (plan.reentrant()) {
                    continue;
                }
                verifyUploadedObject(plan);
                storageService.copyObject(plan.originalKey(), plan.finalObjectKey());
                copiedKeys.add(plan.finalObjectKey());
            }
        } catch (RuntimeException e) {
            deleteUnreferencedCopies(copiedKeys);
            throw e;
        }

        // 階段 3（事務內）：狀態 CAS + 建立 media_asset。整批 all-or-nothing，維持既有 API 語意。
        List<FinalizedAssetDTO> results;
        try {
            results = txTemplate.execute(status -> commitFinalize(plans));
        } catch (RuntimeException e) {
            deleteUnreferencedCopies(copiedKeys);
            throw e;
        }

        // 階段 4（commit 之後、best-effort）：清掉暫存物件。失敗只留下 tmp 物件，
        // 由 MediaGarbageCollectionJob 依 upload_session 過期規則回收，不影響正確性。
        for (FinalizePlan plan : plans) {
            if (plan.reentrant()) {
                continue;
            }
            try {
                storageService.deleteObject(plan.originalKey());
            } catch (RuntimeException e) {
                log.warn("finalize.tmp.cleanupFailed",
                        "暫存物件刪除失敗（不影響轉正結果）: " + e.getMessage(),
                        "object_key", plan.originalKey());
            }
        }

        log.info("finalize.success", "圖片轉正完成",
                "asset_count", String.valueOf(results.size()));

        return results;
    }

    /**
     * 階段 1：驗證單一 token 並算出正式路徑（不碰儲存端、不寫 DB）。
     */
    private FinalizePlan prepareFinalize(String stagedToken, Long callerAccountId, String refType) {
        // 1. 解析並驗證 Token
        StagedTokenClaims claims;
        try {
            claims = stagedTokenService.parseAndValidate(stagedToken);
        } catch (MediaServiceException e) {
            log.warn("finalize.token.invalid", "Token 驗證失敗: " + e.getMessage());
            throw e;
        }

        UUID sessionId = claims.getSessionId();

        // 2. 驗證帳號（設計書 3.2.2）
        if (!claims.getAccountId().equals(callerAccountId)) {
            // 越權防枚舉（API 設計規範 §14.1）：回應與「資產不存在」一致
            log.warn("finalize.account.mismatch", "帳號不匹配",
                    "session_id", sessionId.toString());
            throw MediaServiceException.notFound("資產不存在");
        }

        // 3. 查詢 upload_session
        UploadSessionEntity session = uploadSessionRepository.selectById(sessionId);
        if (session == null) {
            throw MediaServiceException.invalidArgument(
                    "上傳會話不存在: " + sessionId);
        }

        // 3.5 Token claims ↔ server-side session 交叉檢查（#1396）
        // token 完整性邊界已因共用 HMAC key 被削弱，故所有安全宣告一律與 session 再比對，
        // 不可只靠 token account id 與 caller 相符。任一不一致回應與「資產不存在」一致（防枚舉）。
        boolean secureMismatch =
                Boolean.TRUE.equals(claims.getIsSecure()) != Boolean.TRUE.equals(session.getIsSecure());
        boolean expectedScopeMismatch = !expectedScope(session.getIsSecure()).equals(claims.getScope());
        if (!session.getAccountId().equals(claims.getAccountId())
                || secureMismatch
                || expectedScopeMismatch) {
            log.warn("finalize.claims.mismatch", "token claims 與 session 不一致",
                    "session_id", sessionId.toString());
            throw MediaServiceException.notFound("資產不存在");
        }

        // 4. 檢查狀態 - 支援重入
        String currentStatus = session.getStatus();

        if (MediaUploadSessionStatus.FINALIZED.value().equals(currentStatus)) {
            // 已轉正，查找既有資產並回傳（重入語意）
            log.info("finalize.already.done", "會話已轉正，執行重入回傳",
                    "session_id", sessionId.toString());

            return FinalizePlan.reentrant(stagedToken, findExistingAsset(session, stagedToken));
        }

        if (!MediaUploadSessionStatus.COMPLETED.value().equals(currentStatus)) {
            throw MediaServiceException.invalidArgument(
                    "上傳會話狀態不正確，目前狀態: " + currentStatus
                    + "，預期: " + MediaUploadSessionStatus.COMPLETED.value());
        }

        // 5. 檢查是否過期
        if (session.getExpiresAt().isBefore(DateTimeUtils.nowUtc())) {
            throw MediaServiceException.invalidArgument("上傳會話已過期");
        }

        // 6. 生成最終路徑（實際搬移在階段 2、建列在階段 3）
        String originalKey = session.getObjectKey();
        boolean isSecure = Boolean.TRUE.equals(session.getIsSecure());
        String finalObjectKey = storageService.generateFinalObjectKey(
                originalKey, isSecure, refType, session.getAccountId());

        return new FinalizePlan(stagedToken, session, refType, originalKey, finalObjectKey, false, null);
    }

    /**
     * 階段 2：HEAD 回查（49 號 §8.1 強制點）。
     * <p>presigned PUT 繞過宣告，故實際物件的 size/Content-Type 一律在此回查；超限或與宣告不符即
     * 刪除暫存並拒絕。dev 未配置儲存時跳過（走 mock/skip fallback）。</p>
     */
    private void verifyUploadedObject(FinalizePlan plan) {
        if (!storageService.isEnabled()) {
            return;
        }
        UploadSessionEntity session = plan.session();
        StorageObjectStat stat = storageService.statObject(plan.originalKey());
        try {
            uploadValidator.validateFinalizedObject(
                    stat, session.getContentType(), session.getFileSize(),
                    Boolean.TRUE.equals(session.getIsSecure()));
        } catch (MediaServiceException e) {
            log.warn("finalize.head.reject", "HEAD 回查不通過，清理暫存物件",
                    "object_key", plan.originalKey());
            storageService.deleteObject(plan.originalKey());
            throw e;
        }
    }

    /**
     * 階段 3：狀態 CAS + 建立 media_asset（同事務）。
     */
    private List<FinalizedAssetDTO> commitFinalize(List<FinalizePlan> plans) {
        List<FinalizedAssetDTO> results = new ArrayList<>(plans.size());

        for (FinalizePlan plan : plans) {
            if (plan.reentrant()) {
                results.add(plan.reentrantResult());
                continue;
            }

            UUID sessionId = plan.session().getId();

            // 先 CAS 再 INSERT。舊版順序相反：輸掉並發的一方已經 INSERT 完才發現 CAS=0，
            // 那一列不會被回滾，於是留下兩列指向**同一個 object_key** 的 media_asset
            // （dev 實測到相隔 117ms 的一對）。GC 以 object_key 刪 R2 物件，任一列失去引用被
            // 回收時就會刪掉共用物件，另一列變成死指標。
            int updated = uploadSessionRepository.updateStatusWithCondition(
                    sessionId,
                    MediaUploadSessionStatus.COMPLETED.value(),
                    MediaUploadSessionStatus.FINALIZED.value()
            );

            if (updated == 0) {
                // 其他請求搶先轉正：本次不建列。階段 2 複製的正式物件與贏家指向同一個 key
                // （finalObjectKey 由來源 key 決定，同 session 恆等），故也不必刪。
                log.info("finalize.concurrent", "並發處理，嘗試重入",
                        "session_id", sessionId.toString());
                UploadSessionEntity latest = uploadSessionRepository.selectById(sessionId);
                results.add(findExistingAsset(latest != null ? latest : plan.session(), plan.stagedToken()));
                continue;
            }

            MediaAssetEntity asset = createMediaAsset(plan.session(), plan.refType(), plan.finalObjectKey());

            // 回填 asset_id 到 session（用於重入查詢）
            uploadSessionRepository.updateAssetId(sessionId, asset.getId());

            log.info("success", "單一圖片轉正成功, session_id=" + sessionId + ", asset_id=" + asset.getId());

            results.add(mediaAssetConverter.toFinalizedDTO(asset, plan.stagedToken()));
        }

        return results;
    }

    /**
     * 補償刪除：轉正中途失敗時，把已複製到正式路徑、但沒有任何 media_asset 列認得的物件刪掉。
     *
     * <p>刪除前先確認沒有其他已提交的 media_asset 列指向同一個 key —— 並發情境下贏家可能已經
     * 用同一個 finalObjectKey 建好列並提交，這時刪掉物件會把贏家變成死指標。</p>
     */
    private void deleteUnreferencedCopies(List<String> objectKeys) {
        for (String objectKey : objectKeys) {
            try {
                if (mediaAssetRepository.countByObjectKey(objectKey) > 0) {
                    log.info("finalize.compensate.skip",
                            "正式路徑物件已有 media_asset 列指向，略過補償刪除",
                            "object_key", objectKey);
                    continue;
                }
                storageService.deleteObject(objectKey);
                log.info("finalize.compensate.deleted",
                        "轉正失敗，已刪除正式路徑殘留物件",
                        "object_key", objectKey);
            } catch (RuntimeException e) {
                // 補償失敗不可覆蓋原始例外；留下的物件無 DB 列，只能靠對帳清理
                log.error("finalize.compensate.failed",
                        "補償刪除失敗，R2 將留下無主物件 object_key=" + objectKey, e);
            }
        }
    }

    /**
     * 轉正跨階段傳遞的計畫。{@code reentrant=true} 表示該 token 對應的 session 已轉正，
     * 不需要任何儲存端或 DB 寫入，直接回傳 {@code reentrantResult}。
     */
    private record FinalizePlan(
            String stagedToken,
            UploadSessionEntity session,
            String refType,
            String originalKey,
            String finalObjectKey,
            boolean reentrant,
            FinalizedAssetDTO reentrantResult
    ) {
        static FinalizePlan reentrant(String stagedToken, FinalizedAssetDTO result) {
            return new FinalizePlan(stagedToken, null, null, null, null, true, result);
        }
    }

    /**
     * 建立 media_asset（使用轉正後的最終路徑）
     *
     * @param session        上傳會話
     * @param refType        資源分類
     * @param finalObjectKey 轉正後的 OSS 最終路徑
     */
    private MediaAssetEntity createMediaAsset(UploadSessionEntity session, String refType, String finalObjectKey) {
        MediaAssetEntity asset = new MediaAssetEntity();
        asset.setId(TsidUtil.generate());
        asset.setUploaderAccountId(session.getAccountId());
        asset.setObjectKey(finalObjectKey);
        asset.setRefType(refType);
        asset.setIsSecure(session.getIsSecure());

        // 從 session 取得檔案資訊（建立會話時已儲存）
        asset.setMimeType(session.getContentType() != null ? session.getContentType() : "application/octet-stream");
        asset.setSizeBytes(session.getFileSize() != null ? session.getFileSize() : 0L);
        // 原始檔名（#3149）：session 裡的值建立會話時已消毒過，**不再 sanitize 第二次**。
        asset.setOriginalFilename(session.getOriginalFilename());

        // 機敏檔案：複製加密金鑰（公開檔案 URL 由讀取端以 objectKey 組裝，DB 不落 URL）
        if (Boolean.TRUE.equals(session.getIsSecure())) {
            asset.setEncryptionKey(session.getEncryptionKey());
        }

        mediaAssetRepository.insert(asset);
        return asset;
    }

    /**
     * 查找已存在的資產（重入用）
     * <p>
     * 優先使用 session.assetId 直接查找；若 assetId 為空則 fallback 到 accountId 查詢
     */
    private FinalizedAssetDTO findExistingAsset(UploadSessionEntity session, String stagedToken) {
        // 優先使用 asset_id 直接查找（Finalize 後回填的）
        if (session.getAssetId() != null) {
            MediaAssetEntity asset = mediaAssetRepository.selectById(session.getAssetId());
            if (asset != null) {
                return mediaAssetConverter.toFinalizedDTO(asset, stagedToken);
            }
        }

        // Fallback：依 accountId 查找最近的資產（相容舊資料）
        // 只用於一般上傳重入流程，恆為 bucket 1（真實登入 account_id），故查
        // uploaderAccountId 即可，不需考慮 importStoreId。
        List<MediaAssetEntity> assets = mediaAssetRepository.selectList(
                new com.baomidou.mybatisplus.core.conditions.query.LambdaQueryWrapper<MediaAssetEntity>()
                        .eq(MediaAssetEntity::getUploaderAccountId, session.getAccountId())
                        .orderByDesc(MediaAssetEntity::getCreatedAt)
                        .last("LIMIT 1")
        );

        if (assets.isEmpty()) {
            log.error("finalize.inconsistent", "狀態不一致：FINALIZED 但無對應資產",
                    "session_id", session.getId().toString());
            throw MediaServiceException.internal("狀態不一致，請聯繫管理員");
        }

        return mediaAssetConverter.toFinalizedDTO(assets.get(0), stagedToken);
    }


    /**
     * 後端直傳並轉正（服務間呼叫，不經前端上傳流程）
     *
     * 路徑格式：public/{refType}/{yyyyMMdd}/{uuid}/{filename}
     * 系統上傳：uploaderAccountId / importStoreId 皆不設（NULL）
     *
     * @param fileData    檔案二進位資料
     * @param filename    檔案名稱
     * @param contentType MIME 類型
     * @param refType     資產引用類型（如 SHIPPING）
     * @return 已建立的 media_asset
     */
    public MediaAssetEntity serverUploadAndFinalize(byte[] fileData, String filename, String contentType, String refType) {
        // 1. 生成 object key：public/{category}/{yyyyMMdd}/{uuid}/{filename}
        String dateStr = java.time.LocalDate.now().format(java.time.format.DateTimeFormatter.ofPattern("yyyyMMdd"));
        String uuid = UUID.randomUUID().toString();
        String category = AssetReferenceType.fromValue(refType).code().toLowerCase();
        String objectKey = String.format("public/%s/%s/%s/%s", category, dateStr, uuid, filename);
        // 原始檔名（#3149，設計書 08 §4.9.3〈作用範圍〉）：**另宣告變數，不覆寫 filename**。
        // filename 的原值上面剛拿去組 object_key 末段，那條路徑一個字都不能動。
        String displayName = AssetFilename.sanitize(filename);

        // 2. 上傳至儲存後端（事務外——R2 不隨 DB rollback）
        storageService.uploadStream(
                objectKey,
                new java.io.ByteArrayInputStream(fileData),
                contentType,
                fileData.length
        );

        // 3. 建立 media_asset 記錄；失敗即補償刪除，否則 R2 留下沒有任何一張表認得的物件
        MediaAssetEntity asset = new MediaAssetEntity();
        asset.setId(TsidUtil.generate());
        // bucket 2（系統直傳）：uploaderAccountId / importStoreId 皆不設，維持 NULL
        asset.setObjectKey(objectKey);
        asset.setRefType(refType);
        asset.setIsSecure(false);
        asset.setMimeType(contentType);
        asset.setSizeBytes((long) fileData.length);
        asset.setOriginalFilename(displayName);

        try {
            mediaAssetRepository.insert(asset);
        } catch (RuntimeException e) {
            try {
                storageService.deleteObject(objectKey);
                log.info("serverUpload.compensate.deleted",
                        "建列失敗，已刪除剛上傳的物件", "object_key", objectKey);
            } catch (RuntimeException dx) {
                log.error("serverUpload.compensate.failed",
                        "補償刪除失敗，R2 將留下無主物件 object_key=" + objectKey, dx);
            }
            throw e;
        }

        log.info("serverUpload.success", "後端直傳完成，objectKey=" + objectKey,
                "asset_id", String.valueOf(asset.getId()));

        return asset;
    }

    /**
     * 機敏檔後端直傳並轉正（58 §8.1.5）。發票 PDF 與稅務證明走這一支。
     *
     * <p>與 {@link #serverUploadAndFinalize} 的三個差別，缺任何一件發票 PDF 都不能用：
     * <ol>
     *   <li>objectKey 落 {@code secure/} 前綴（那支硬寫 {@code public/}）</li>
     *   <li>{@code is_secure = true}（那支硬寫 false）</li>
     *   <li>refType／refId <b>必填</b>，且在<b>同一次呼叫</b>內建立 asset_reference</li>
     * </ol>
     *
     * <p>🔴 第 3 點是 58 §8.6 T-03 的直接產物：先上傳資產、事後再另建 asset_reference
     * （或靠掃描補建）的話，media GC 第 2 階段會在第 8 天把 media_asset 連同 R2 物件
     * 刪除。發票文件列還在、{@code pdf_object_key} 還在，<b>下載時才 404</b>——而 AMEGO 的
     * {@code invoice_file} 只能查 180 天內，超過就再也抓不回來。所以「建資產」與「建引用」
     * 必須是一個原子動作。
     *
     * <p>🔴 refId 用 {@code invoice_documents.id} 不用 {@code order_no}（T-04）：
     * {@code releaseReferences} 依 {@code (ref_type, ref_id)} <b>批次</b>釋放，用 order_no
     * 當 refId 會在任一份文件要釋放時，把原發票＋重開票＋所有折讓的引用一起打掉。
     */
    public MediaAssetEntity serverUploadSecureAndFinalize(
            byte[] fileData, String filename, String contentType, String refType, String refId) {
        String dateStr = java.time.LocalDate.now().format(java.time.format.DateTimeFormatter.ofPattern("yyyyMMdd"));
        String uuid = UUID.randomUUID().toString();
        String category = AssetReferenceType.fromValue(refType).code().toLowerCase();
        String objectKey = String.format("secure/%s/%s/%s/%s", category, dateStr, uuid, filename);
        // 同 serverUploadAndFinalize：object_key 末段吃原值，只有顯示值走 sanitize。
        String displayName = AssetFilename.sanitize(filename);

        // 🔴 **AES-GCM 加密後才落 R2**（#3024）。在此之前這一支把明文直接送進 R2、
        // 而 asset 標成 is_secure=true 且 encryption_key 為 NULL —— 它唯一的使用者是
        // 發票／收據 PDF（ref_type=7120），也就是帶著完整買受人姓名、統編與金額的稅務憑證。
        //
        // 🔴 **主密鑰不可用時 fail loud，不得退回明文**：退回明文正是本缺陷本身，
        // 而且它是完全靜默的（上傳成功、asset 標著 is_secure，只有靜態儲存是裸的）。
        // 上傳失敗則是響的：INV.pdf 是帶重試與 DLQ 的排程 worker，看得見也救得回來。
        // dev 與 prod 的 media-service 都已注入 ENCRYPTION_MASTER_KEY（2026-09-02 兩端實查
        // 皆為非空，來源 secret hawkr-r2），所以這條分支在正常環境不會被走到。
        if (!aesStreamCipher.isAvailable()) {
            throw MediaServiceException.internal(
                    "機敏檔加密主密鑰未配置（ENCRYPTION_MASTER_KEY），拒絕以明文寫入機敏資產");
        }
        String wrappedKey = aesStreamCipher.generateWrappedKey();
        // 密文大小 = 明文大小 + GCM auth tag；uploadStream 需要精確長度。
        long ciphertextSize = (long) fileData.length + AesStreamCipher.GCM_TAG_BYTES;
        try (java.io.InputStream plainInput = new java.io.ByteArrayInputStream(fileData);
             CipherInputStream encryptedStream = aesStreamCipher.encryptingStream(plainInput, wrappedKey)) {
            storageService.uploadStream(
                    objectKey,
                    encryptedStream,
                    // 物件本身是密文 ⇒ 儲存端的 content-type 是 octet-stream；
                    // **邏輯型別仍記在 asset.mime_type**（下面），讀回時由那一欄決定回什麼。
                    "application/octet-stream",
                    ciphertextSize
            );
        } catch (IOException e) {
            throw MediaServiceException.internal("機敏檔加密上傳失敗", e);
        }

        MediaAssetEntity asset = new MediaAssetEntity();
        asset.setId(TsidUtil.generate());
        asset.setObjectKey(objectKey);
        asset.setRefType(refType);
        asset.setIsSecure(true);
        asset.setEncryptionKey(wrappedKey);
        // mime_type 記**明文**的型別（application/pdf），不是 R2 上那份密文的型別：
        // readAssetBytes 解密之後回的是明文，呼叫端要的是明文的型別。
        asset.setMimeType(contentType);
        // size_bytes 記**明文**長度，與 readAssetBytes 解密後的長度同一個口徑。
        asset.setSizeBytes((long) fileData.length);
        asset.setOriginalFilename(displayName);

        try {
            // 建列與建引用在同一個交易內：只建列而引用建失敗，會留下一個 GC 會在第 8 天
            // 刪掉的資產，而呼叫端已經把 objectKey 存進業務表了。
            txTemplate.executeWithoutResult(status -> {
                mediaAssetRepository.insert(asset);
                assetReferenceService.linkImages(refType, refId,
                        java.util.List.of(String.valueOf(asset.getId())));
            });
        } catch (RuntimeException e) {
            try {
                storageService.deleteObject(objectKey);
                log.info("serverUploadSecure.compensate.deleted",
                        "建列或建引用失敗，已刪除剛上傳的物件", "object_key", objectKey);
            } catch (RuntimeException dx) {
                log.error("serverUploadSecure.compensate.failed",
                        "補償刪除失敗，R2 將留下無主物件 object_key=" + objectKey, dx);
            }
            throw e;
        }

        log.info("serverUploadSecure.success", "機敏檔後端直傳完成，objectKey=" + objectKey,
                "asset_id", String.valueOf(asset.getId()));

        return asset;
    }

    /**
     * 資產不存在與越權共用的回應訊息。
     *
     * <p>🔴 兩條路徑必須回<b>同一個字串</b>：訊息不同就等於告訴呼叫端「這個 object_key 存在，
     * 只是你的 (ref_type, ref_id) 猜錯了」——那讓這支 RPC 變成一個可枚舉的 oracle。
     * 訊息裡也刻意不帶 object_key：它是呼叫端自己送進來的值，回聲沒有診斷價值，
     * 卻讓兩條路徑多一個可以漂移出差異的地方。
     */
    private static final String SECURE_ASSET_NOT_FOUND = "機敏資產不存在";

    /**
     * 服務端讀取機敏資產原始位元組（58 §8.1.6）。發票 PDF 的三條下載路徑走這一支。
     *
     * <p><b>為什麼需要它</b>：發票 PDF 由 invoice-service 以<b>服務身分</b>經
     * {@link #serverUploadSecureAndFinalize} 上傳，上傳者不是任何一個真人；而
     * {@code GET /app/media/secure_download/{asset_id}} 的授權述詞是「使用者是上傳者、
     * 或使用者持有該資產的 asset_reference」⇒ 三個合法讀者（買家／賣家／ADM）一個都不成立。
     * 加密金鑰又只有 media 拿得到（呼叫端直接讀 R2 只會拿到密文，且違反 25 號規範的
     * 「儲存 SDK 只准在 media／product-import」），所以必須有一支「把 bytes 還給上傳它的
     * 那個服務」的內部 RPC——誰能看是呼叫端的事，media 只驗資產歸屬。
     *
     * <p><b>授權述詞＝三元組全等</b>：{@code object_key} 會出現在呼叫端的 DB 與 log 裡，
     * 單獨拿它當憑證等於「知道路徑就能讀」。加上 {@code (ref_type, ref_id)} 之後，呼叫端
     * 必須同時說得出「這份資產綁在哪一筆業務上」，而那是它自己寫進 asset_reference 的值。
     *
     * @param objectKey 資產路徑（{@code secure/} 前綴）
     * @param refType   引用類型，須與上傳當時相同
     * @param refId     引用業務 ID，須與上傳當時相同
     * @return 解密後的原始內容
     * @throws MediaServiceException NOT_FOUND（資產不存在或三元組不符，兩者不可分辨）／
     *                               INVALID_ARGUMENT（非機敏檔）／INTERNAL（讀取或解密失敗）
     */
    public SecureAssetContentDTO serverReadSecureAsset(String objectKey, String refType, String refId) {
        MediaAssetEntity asset = mediaAssetRepository.selectByObjectKey(objectKey);
        if (asset == null) {
            log.warn("serverReadSecure.notFound", "機敏資產不存在，object_key=" + objectKey);
            throw MediaServiceException.notFound(SECURE_ASSET_NOT_FOUND);
        }

        // 非機敏檔不從這支出：公開資產本來就以 URL 對外，走這裡只會繞過 CDN。
        // 這一條刻意<b>不</b>與 not-found 合併——它不洩漏授權述詞，且合併會讓呼叫端
        // 把「用錯 API」誤讀成「資產被 GC 掉了」。
        if (!Boolean.TRUE.equals(asset.getIsSecure())) {
            throw MediaServiceException.invalidArgument("此資產不是機敏檔案，請使用公開 URL 存取");
        }

        if (!assetReferenceService.hasReference(asset.getId(), refType, refId)) {
            log.warn("serverReadSecure.denied",
                    "三元組不符，object_key=" + objectKey + ", ref_type=" + refType + ", ref_id=" + refId);
            throw MediaServiceException.notFound(SECURE_ASSET_NOT_FOUND);
        }

        byte[] fileData = readAssetBytes(asset);

        log.info("serverReadSecure.success", "機敏資產讀取成功，object_key=" + objectKey,
                "asset_id", String.valueOf(asset.getId()));

        String mimeType = asset.getMimeType() != null && !asset.getMimeType().isEmpty()
                ? asset.getMimeType()
                : "application/octet-stream";
        return new SecureAssetContentDTO(fileData, mimeType,
                AssetFilename.resolveDisplayName(asset.getOriginalFilename(), asset.getObjectKey()));
    }

    /**
     * 資產中介資料批次查詢（#3031）。回 asset_id／檔名／大小／上傳時間，<b>不回 bytes</b>。
     *
     * <p><b>為什麼需要它</b>：呼叫端手上只有 object_key（那是它自己存進業務表的東西），
     * 而畫面要的是「這份附件多大、什麼時候傳的、下載連結怎麼組」。asset_id 是 Gateway
     * 組下載路徑的唯一輸入，沒有它前端只能畫一個點不下去的檔名。
     *
     * <p><b>授權述詞與 {@link #serverReadSecureAsset} 逐字相同</b>：三元組全等。
     * 🔴 <b>查無與越權必須無法分辨</b>——通不過的 key 就是不出現在結果裡，不回錯誤、
     * 不回空物件占位。回一個「這個 key 存在但你沒權限」的訊號等於把這支 RPC 變成
     * 可枚舉的 oracle：攻擊者拿一批猜的 object_key 掃，靠回應差異就能問出哪些存在。
     *
     * <p>⚠️ <b>size_bytes 取 media_asset.size_bytes（明文長度），不是 R2 上的物件大小</b>：
     * 機敏檔在 R2 上是密文（明文 + 16 bytes GCM tag，#3024），拿物件大小去顯示會讓
     * 每一份附件都多 16 bytes——一個永遠對不上、也永遠不會有人發現的數字。
     */
    public List<AssetMetadataDTO> batchGetAssetMetadata(List<String> objectKeys, String refType, String refId) {
        if (objectKeys == null || objectKeys.isEmpty()) {
            return List.of();
        }
        List<AssetMetadataDTO> out = new java.util.ArrayList<>(objectKeys.size());
        for (String key : objectKeys) {
            if (key == null || key.isEmpty()) {
                continue;
            }
            MediaAssetEntity asset = mediaAssetRepository.selectByObjectKey(key);
            if (asset == null) {
                continue; // 查無：不出現在結果裡
            }
            if (!assetReferenceService.hasReference(asset.getId(), refType, refId)) {
                log.warn("batchGetAssetMetadata.denied",
                    "三元組不符，object_key=" + key + ", ref_type=" + refType + ", ref_id=" + refId);
                continue; // 越權：與查無**逐字相同**的處置
            }
            out.add(new AssetMetadataDTO(
                asset.getObjectKey(),
                String.valueOf(asset.getId()),
                AssetFilename.resolveDisplayName(asset.getOriginalFilename(), asset.getObjectKey()),
                asset.getSizeBytes() == null ? "0" : String.valueOf(asset.getSizeBytes()),
                // 🔴 **截到秒**：契約要求 ISO 8601 UTC **秒精度**（58 §6.2.0 硬慣例 3）。
                // DateTimeUtils.formatIso8601 走 ISO_OFFSET_DATE_TIME，有次秒就照印
                // ——dev 實測回出 "2026-09-01T16:10:13.039874Z"，與同一份 payload 上
                // 其他時間欄（Go 端 timeutil.FormatISO8601，恆秒精度）長得不一樣。
                // 不改共用 helper：它有其他消費者，改它的行為是另一件事。
                asset.getCreatedAt() == null ? ""
                        : DateTimeUtils.formatIso8601(asset.getCreatedAt().truncatedTo(java.time.temporal.ChronoUnit.SECONDS)),
                asset.getMimeType() == null ? "" : asset.getMimeType()));
        }
        return out;
    }

    /**
     * 從儲存端取回資產的明文位元組。
     *
     * <p><b>加密與否由 {@code encryption_key} 這一欄決定，不由 {@code is_secure} 決定。</b>
     * 兩條寫入路徑<b>現在都會加密</b>（#3024 之後）：使用者的
     * {@code PUT /app/media/secure_upload} 與 {@link #serverUploadSecureAndFinalize}
     * 都是 AES-GCM ＋ 把 wrapped key 寫進 {@code media_asset.encryption_key}。
     *
     * <p>🔴 <b>明文分支仍然必要，不得刪除</b>：#3024 修好之前寫進去的資產
     * （dev/prod 上既有的發票 PDF）{@code encryption_key} 為 NULL，刪掉這條分支
     * 等於把歷史發票鎖死。以<b>欄位值</b>判斷而不是以 {@code is_secure} 判斷，
     * 正是為了讓新舊兩種都讀得回來。
     *
     * <p>讀完一定驗長度：{@code CipherInputStream} 對 GCM 驗章失敗是<b>靜默</b>吐較少的
     * 位元組（JDK 行為），不驗長度就會把截斷的內容當成正常結果回給呼叫端，而呼叫端會把
     * 它當發票 PDF 存下去。長度基準取 R2 上實際的物件大小（{@code statObject}），不取
     * {@code media_asset.size_bytes}——後者來自 client 在上傳前宣告的估計值，本來就可能對不上
     * （見 {@code SecureMediaController.setActualContentLength} 的說明）。
     */
    private byte[] readAssetBytes(MediaAssetEntity asset) {
        String wrappedKey = asset.getEncryptionKey();
        boolean encrypted = wrappedKey != null && !wrappedKey.isEmpty();

        byte[] data;
        try (InputStream ossStream = storageService.getObjectStream(asset.getObjectKey())) {
            if (encrypted) {
                try (CipherInputStream decryptedStream = aesStreamCipher.decryptingStream(ossStream, wrappedKey)) {
                    data = decryptedStream.readAllBytes();
                }
            } else {
                data = ossStream.readAllBytes();
            }
        } catch (IOException e) {
            throw MediaServiceException.internal("讀取機敏資產失敗", e);
        }

        StorageObjectStat stat = storageService.statObject(asset.getObjectKey());
        if (stat != null) {
            long expected = encrypted
                    ? stat.contentLength() - AesStreamCipher.GCM_TAG_BYTES
                    : stat.contentLength();
            if (data.length != expected) {
                log.error("serverReadSecure.lengthMismatch",
                        "解密後長度與儲存端不符 object_key=" + asset.getObjectKey()
                                + ", expected=" + expected + ", actual=" + data.length);
                throw MediaServiceException.internal("機敏資產讀取不完整");
            }
        } else if (data.length == 0) {
            // statObject 查不到就沒有基準可比，至少擋住「回空 bytes 當成功」。
            throw MediaServiceException.internal("機敏資產讀取不完整");
        }

        return data;
    }

    /**
     * 依 isSecure 推導 staged token 應有的 scope（#1396 交叉檢查用）。
     */
    private static String expectedScope(Boolean isSecure) {
        return Boolean.TRUE.equals(isSecure)
                ? StagedTokenScope.MEDIA_SECURE_UPLOAD.value()
                : StagedTokenScope.MEDIA_UPLOAD_SESSION.value();
    }

    /**
     * 依 ID 查詢資產
     */
    public MediaAssetEntity getAsset(Long assetId) {
        return mediaAssetRepository.selectById(assetId);
    }

    /**
     * 批次查詢資產
     */
    public List<MediaAssetEntity> getAssets(List<Long> assetIds) {
        if (assetIds == null || assetIds.isEmpty()) {
            return Collections.emptyList();
        }
        return mediaAssetRepository.selectByIds(assetIds);
    }
}
