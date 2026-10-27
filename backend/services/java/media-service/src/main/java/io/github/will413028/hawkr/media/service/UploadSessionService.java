package io.github.will413028.hawkr.media.service;

import io.github.will413028.hawkr.common.obs.log.ObsLogger;
import io.github.will413028.hawkr.common.utils.code.MediaUploadSessionStatus;
import io.github.will413028.hawkr.media.dto.request.CompleteUploadSessionRequest;
import io.github.will413028.hawkr.media.dto.request.CreateUploadSessionRequest;
import io.github.will413028.hawkr.media.dto.response.CompleteUploadSessionResponse;
import io.github.will413028.hawkr.media.dto.response.CreateUploadSessionResponse;
import io.github.will413028.hawkr.media.entity.UploadSessionEntity;
import io.github.will413028.hawkr.media.exception.MediaServiceException;
import io.github.will413028.hawkr.media.repository.UploadSessionRepository;
import io.github.will413028.hawkr.media.storage.StorageService;
import io.github.will413028.hawkr.media.util.AssetFilename;
import lombok.RequiredArgsConstructor;
import org.springframework.beans.factory.annotation.Value;
import org.springframework.stereotype.Service;
import org.springframework.transaction.annotation.Transactional;

import io.github.will413028.hawkr.common.utils.DateTimeUtils;
import java.time.LocalDateTime;
import java.util.UUID;

/**
 * 上傳會話服務
 * <p>
 * 職責：管理上傳會話的生命週期
 */
@Service
@RequiredArgsConstructor
public class UploadSessionService {

    private static final ObsLogger log = ObsLogger.forClass(UploadSessionService.class);

    private final UploadSessionRepository uploadSessionRepository;
    private final StorageService storageService;
    private final StagedTokenService stagedTokenService;
    private final MediaUploadValidator uploadValidator;

    @Value("${media.upload.session-ttl-minutes:30}")
    private int sessionTtlMinutes;

    @Value("${hawkr.media.preview.ttl:86400}")
    private int previewTtlSeconds;

    /**
     * 建立上傳會話
     * <p>
     * 公開上傳：回傳 OSS 簽名 URL 機敏上傳：不回傳 OSS URL，需透過 secure_upload 接口
     */
    @Transactional
    public CreateUploadSessionResponse createSession(Long accountId, CreateUploadSessionRequest request) {
        log.info("session.create.start", "開始建立上傳會話", "account_id", accountId.toString());

        boolean isSecure = Boolean.TRUE.equals(request.getIsSecure());

        // 上傳宣告驗證（49 號 §8.1）：MIME 白名單 + fileSize 上限
        uploadValidator.validateDeclared(request.getContentType(), request.getFileSize(), isSecure);

        LocalDateTime now = DateTimeUtils.nowUtc();
        LocalDateTime expiresAt = now.plusMinutes(sessionTtlMinutes);

        // 生成 object key
        String objectKey = storageService.generateObjectKey(
                request.getFilename(),
                isSecure
        );

        // 建立會話 Entity（IdType.INPUT 需手動生成 UUID）
        UploadSessionEntity session = new UploadSessionEntity();
        session.setId(UUID.randomUUID());
        session.setAccountId(accountId);
        session.setObjectKey(objectKey);
        session.setIsSecure(isSecure);
        session.setContentType(request.getContentType());
        session.setFileSize(request.getFileSize());
        // 原始檔名（#3149，設計書 08 §4.9.3）。
        // 🔴 **必須排在 generateObjectKey 之後，順序不可對調**：sanitize 的第一步是取
        // basename，會把 `photo.jp/g` 洗成合法的 `g`，而 AbstractStorageService.sanitizedExtension
        // 對含 `/`、`\`、`..` 的副檔名是丟 INVALID_PARAM 的既有路徑穿越拒絕。先消毒再生成 key
        // ⇒ 那道拒絕被靜默關掉，而編譯、測試、CI 全綠。key 一律吃 request.getFilename() 的原值。
        session.setOriginalFilename(AssetFilename.sanitize(request.getFilename()));
        session.setStatus(MediaUploadSessionStatus.INIT.value());
        session.setExpiresAt(expiresAt);
        session.setCreatedAt(now);

        uploadSessionRepository.insert(session);

        log.info("session.create.success", "上傳會話建立成功",
                "session_id", session.getId().toString());

        // 建立回應
        CreateUploadSessionResponse.CreateUploadSessionResponseBuilder responseBuilder
                = CreateUploadSessionResponse.builder()
                        .sessionId(session.getId().toString())
                        .expiresIn(sessionTtlMinutes * 60)
                        .isSecure(isSecure);

        // 僅公開上傳提供簽名 URL
        if (!isSecure) {
            String uploadUrl = storageService.generatePresignedUploadUrl(
                    objectKey,
                    request.getContentType(),
                    sessionTtlMinutes * 60
            );
            responseBuilder.uploadUrl(uploadUrl);
        }

        return responseBuilder.build();
    }

    /**
     * 完成上傳會話（公開上傳用）
     * <p>
     * 前端上傳 OSS 完畢後呼叫此接口 回傳 staged_token 供後續 Finalize 使用
     */
    @Transactional
    public CompleteUploadSessionResponse completeSession(
            Long accountId,
            UUID sessionId,
            CompleteUploadSessionRequest request
    ) {
        log.info("session.complete.start", "開始完成上傳會話",
                "session_id", sessionId.toString());

        // 查詢會話
        UploadSessionEntity session = uploadSessionRepository.selectByIdAndAccountId(sessionId, accountId);
        if (session == null) {
            throw MediaServiceException.notFound("上傳會話不存在或無權限存取");
        }

        // 檢查狀態
        if (!MediaUploadSessionStatus.INIT.value().equals(session.getStatus())) {
            throw MediaServiceException.failedPrecondition(
                    "上傳會話狀態不正確，目前狀態: " + session.getStatus());
        }

        // 檢查是否過期
        if (session.getExpiresAt().isBefore(DateTimeUtils.nowUtc())) {
            throw MediaServiceException.invalidArgument("上傳會話已過期");
        }

        // 機敏上傳不走這個流程
        if (Boolean.TRUE.equals(session.getIsSecure())) {
            throw MediaServiceException.invalidArgument(
                    "機敏檔案不支援此完成方式，請使用 secure_upload 接口");
        }

        // 原子更新狀態
        int updated = uploadSessionRepository.updateStatusWithCondition(
                sessionId,
                MediaUploadSessionStatus.INIT.value(),
                MediaUploadSessionStatus.COMPLETED.value()
        );

        if (updated == 0) {
            throw MediaServiceException.failedPrecondition("更新會話狀態失敗，可能已被其他請求處理");
        }

        // 生成 Staged Token
        String stagedToken = stagedTokenService.generateToken(
                session.getId(),
                session.getAccountId(),
                session.getIsSecure(),
                session.getExpiresAt()
        );

        // 生成預覽 URL（OSS 簽名 URL）
        String previewUrl = storageService.generatePresignedDownloadUrl(
                session.getObjectKey(),
                previewTtlSeconds
        );

        log.info("session.complete.success", "上傳會話完成",
                "session_id", sessionId.toString());

        return CompleteUploadSessionResponse.builder()
                .stagedToken(stagedToken)
                .previewUrl(previewUrl)
                .build();
    }

    /**
     * 依 ID 查詢會話
     */
    public UploadSessionEntity getSession(UUID sessionId) {
        return uploadSessionRepository.selectById(sessionId);
    }

    /**
     * 依 ID 與帳號查詢會話
     */
    public UploadSessionEntity getSessionByIdAndAccount(UUID sessionId, Long accountId) {
        return uploadSessionRepository.selectByIdAndAccountId(sessionId, accountId);
    }
}
