package io.github.will413028.hawkr.media.controller;

import io.github.will413028.hawkr.common.grpc.GrpcAuthContext;
import io.github.will413028.hawkr.common.obs.log.ObsLogger;
import io.github.will413028.hawkr.common.utils.code.MediaUploadSessionStatus;
import io.github.will413028.hawkr.common.utils.code.AccountType;
import io.github.will413028.hawkr.common.utils.code.helper.AccountTypeRules;
import io.github.will413028.hawkr.media.dto.response.ApiResponse;
import io.github.will413028.hawkr.media.dto.response.CompleteUploadSessionResponse;
import io.github.will413028.hawkr.media.entity.MediaAssetEntity;
import io.github.will413028.hawkr.media.entity.UploadSessionEntity;
import io.github.will413028.hawkr.media.exception.MediaServiceException;
import io.github.will413028.hawkr.media.repository.AssetReferenceRepository;
import io.github.will413028.hawkr.media.repository.UploadSessionRepository;
import io.github.will413028.hawkr.media.service.MediaAssetService;
import io.github.will413028.hawkr.media.service.StagedTokenService;
import io.github.will413028.hawkr.media.crypto.AesStreamCipher;
import io.github.will413028.hawkr.media.storage.StorageObjectStat;
import io.github.will413028.hawkr.media.storage.StorageService;
import io.github.will413028.hawkr.media.util.AssetFilename;
import jakarta.servlet.http.HttpServletRequest;
import jakarta.servlet.http.HttpServletResponse;
import lombok.RequiredArgsConstructor;
import org.springframework.http.ContentDisposition;
import org.springframework.http.HttpHeaders;
import org.springframework.web.bind.annotation.*;

import io.github.will413028.hawkr.common.utils.validation.InputValidator;
import org.springframework.beans.factory.annotation.Value;

import javax.crypto.CipherInputStream;
import java.io.IOException;
import java.io.InputStream;
import java.io.OutputStream;
import java.nio.charset.StandardCharsets;
import io.github.will413028.hawkr.common.utils.DateTimeUtils;
import java.time.LocalDateTime;
import java.util.UUID;

/**
 * 機敏資源 Controller
 * <p>
 * 依設計書 3.1 定義的機敏檔案處理介面
 * <p>
 * 路徑規則：Gateway 會將 /api/v1/app/media/* 去除 /api/v1 前綴後轉發至此服務
 * 例如：前端 PUT /api/v1/app/media/secure_upload/{id} → 此服務 PUT /app/media/secure_upload/{id}
 */
@RestController
@RequestMapping("/app/media")
@RequiredArgsConstructor
public class SecureMediaController {

    private static final ObsLogger log = ObsLogger.forClass(SecureMediaController.class);

    private final UploadSessionRepository uploadSessionRepository;
    private final AssetReferenceRepository assetReferenceRepository;
    private final MediaAssetService mediaAssetService;
    private final StorageService storageService;
    private final StagedTokenService stagedTokenService;
    private final AesStreamCipher aesStreamCipher;

    @Value("${hawkr.media.preview.ttl:3600}")
    private int previewTtlSeconds;

    /**
     * 機敏檔案流式上傳（Binary Stream）
     * <p>
     * 前端呼叫：PUT /api/v1/app/media/secure_upload/{id}
     * 本服務：PUT /app/media/secure_upload/{id}
     * <p>
     * 與公開上傳統一使用 PUT + binary stream 方式，
     * 前端將檔案流傳至後端，後端採 AES 邊收邊加密邊傳至 OSS，回傳 staged_token
     * <p>
     * Headers:
     * - Content-Type: 檔案的 MIME 類型（如 image/jpeg）
     * - Content-Length: 檔案大小（位元組）
     */
    @PutMapping("/secure_upload/{id}")
    public ApiResponse<CompleteUploadSessionResponse> secureUpload(
            @RequestHeader(value = GrpcAuthContext.HEADER_ACCOUNT_ID) Long accountId,
            @RequestHeader(value = GrpcAuthContext.HEADER_ACCOUNT_TYPE) String accountType,
            @RequestHeader(value = "Content-Type", defaultValue = "application/octet-stream") String contentType,
            @RequestHeader(value = "Content-Length", required = false) Long contentLength,
            @PathVariable("id") String id,
            HttpServletRequest request
    ) throws IOException {
        // 授權：BUYER / SELLER / ADMIN 可上傳機敏檔案
        validateAccountType(accountType, AccountType.BUYER, AccountType.SELLER, AccountType.ADMIN);

        log.info("http.secure.upload.start", "收到機敏檔案上傳請求",
                "session_id", id);

        UUID sessionId = UUID.fromString(id);

        // 查詢會話
        UploadSessionEntity session = uploadSessionRepository.selectByIdAndAccountId(sessionId, accountId);
        if (session == null) {
            throw MediaServiceException.notFound("上傳會話不存在或無權限存取");
        }

        // 檢查是否為機敏上傳
        if (!Boolean.TRUE.equals(session.getIsSecure())) {
            throw MediaServiceException.invalidArgument(
                    "此會話不是機敏上傳，請使用標準上傳流程");
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

        // 取得檔案大小（優先使用 header，否則用 request 的 content length）
        long fileSize = contentLength != null ? contentLength : request.getContentLengthLong();

        // 生成 AES-GCM 加密金鑰（以主密鑰加密後存儲）
        String wrappedKey = aesStreamCipher.generateWrappedKey();

        // issue #2759：每次呼叫都用全新、per-attempt 專屬的 object key，不重用
        // session 建立時就固定的 session.getObjectKey()——兩個並發 attempt 天生寫到
        // 不同位置，R2 層面結構上不可能互相覆寫。只有下面 CAS 贏家的 key 會被記錄。
        String attemptObjectKey = session.getObjectKey() + ".attempt-" + UUID.randomUUID();

        // 執行流式加密上傳：邊收邊加密邊傳至 OSS
        // 密文大小 = 明文大小 + GCM auth tag (16 bytes)
        long ciphertextSize = fileSize + AesStreamCipher.GCM_TAG_BYTES;
        try (InputStream plainInput = request.getInputStream();
             CipherInputStream encryptedStream = aesStreamCipher.encryptingStream(plainInput, wrappedKey)) {
            storageService.uploadStream(
                    attemptObjectKey,
                    encryptedStream,
                    "application/octet-stream",
                    ciphertextSize
            );
        }

        // 更新狀態、延長 expires_at 為預覽有效期、存入加密金鑰與本次 attempt 的 object key
        LocalDateTime previewExpiresAt = DateTimeUtils.nowUtc().plusSeconds(previewTtlSeconds);
        int updated = uploadSessionRepository.updateStatusExpiresAtEncryptionKeyAndObjectKey(
                sessionId,
                MediaUploadSessionStatus.INIT.value(),
                MediaUploadSessionStatus.COMPLETED.value(),
                previewExpiresAt,
                wrappedKey,
                attemptObjectKey
        );

        if (updated == 0) {
            // 輸掉並發：另一個 attempt 已經先完成這個 session 的上傳。本次的 key
            // 天生唯一、沒有任何 DB 列會引用它，直接自我刪除，不留孤兒物件。
            try {
                storageService.deleteObject(attemptObjectKey);
            } catch (RuntimeException e) {
                log.error("http.secure.upload.compensate.failed",
                        "並發落敗後補償刪除失敗，R2 將留下無主物件 object_key=" + attemptObjectKey, e);
            }
            throw MediaServiceException.failedPrecondition("並發上傳衝突，另一個請求已完成此會話上傳，請重新查詢會話狀態");
        }

        // 生成 Staged Token（使用延長後的到期時間）
        String stagedToken = stagedTokenService.generateToken(
                session.getId(),
                session.getAccountId(),
                session.getIsSecure(),
                previewExpiresAt
        );

        // 組合預覽 URL（經由 Gateway 代理，前端使用 /api/v1/app 前綴）
        String previewUrl = "/api/v1/app/media/secure_preview/" + id;

        log.info("http.secure.upload.success", "機敏檔案上傳成功",
                "session_id", id);

        return ApiResponse.success(
                CompleteUploadSessionResponse.builder()
                        .stagedToken(stagedToken)
                        .previewUrl(previewUrl)
                        .build()
        );
    }

    /**
     * 機敏檔案預覽（Session-based，上傳完成後 Finalize 前使用）
     * <p>
     * 前端呼叫：GET /api/v1/app/media/secure_preview/{session_id}
     * 本服務：GET /app/media/secure_preview/{session_id}
     * <p>
     * 驗證邏輯：
     * - Session 狀態必須為 COMPLETED
     * - Session 未過期（expires_at > now，上傳完成時已延長為預覽 TTL）
     * - x-account-id 必須與 session.accountId 一致
     * <p>
     * 與 secure_download 的分工：
     * - secure_preview：Finalize 前的暫存預覽，基於 session_id
     * - secure_download：Finalize 後的正式資產讀取，基於 asset_id
     */
    @GetMapping("/secure_preview/{session_id}")
    public void securePreview(
            @RequestHeader(value = GrpcAuthContext.HEADER_ACCOUNT_ID) Long accountId,
            @RequestHeader(value = GrpcAuthContext.HEADER_ACCOUNT_TYPE) String accountType,
            @PathVariable("session_id") String sessionId,
            HttpServletResponse response
    ) throws IOException {
        // 授權：BUYER / SELLER / ADMIN 可預覽機敏檔案
        validateAccountType(accountType, AccountType.BUYER, AccountType.SELLER, AccountType.ADMIN);

        log.info("http.secure.preview.start", "收到機敏檔案預覽請求",
                "session_id", sessionId);

        UUID sessionUUID = UUID.fromString(sessionId);

        // 查詢會話（含 accountId 驗證）
        UploadSessionEntity session = uploadSessionRepository.selectByIdAndAccountId(sessionUUID, accountId);
        if (session == null) {
            throw MediaServiceException.notFound("上傳會話不存在或無權限存取");
        }

        // 檢查狀態：僅 COMPLETED 狀態可預覽
        if (!MediaUploadSessionStatus.COMPLETED.value().equals(session.getStatus())) {
            throw MediaServiceException.failedPrecondition(
                    "上傳會話狀態不正確，僅已完成的會話可預覽，目前狀態: " + session.getStatus());
        }

        // 檢查是否過期
        if (session.getExpiresAt().isBefore(DateTimeUtils.nowUtc())) {
            throw MediaServiceException.invalidArgument("預覽已過期");
        }

        // 從 OSS 串流讀取、AES-GCM 解密後回傳
        String encryptionKey = session.getEncryptionKey();
        if (encryptionKey == null || encryptionKey.isEmpty()) {
            throw MediaServiceException.internal("此會話缺少加密金鑰");
        }

        try (InputStream ossStream = storageService.getObjectStream(session.getObjectKey());
             CipherInputStream decryptedStream = aesStreamCipher.decryptingStream(ossStream, encryptionKey);
             OutputStream outputStream = response.getOutputStream()) {

            // 預覽發生在 Finalize **之前**，asset 還不存在 ⇒ 檔名取自 upload_session（#3149 §4.9.4）
            String filename = AssetFilename.resolveDisplayName(
                    session.getOriginalFilename(), session.getObjectKey());

            // 使用 session 記錄的 Content-Type，更準確
            String mimeType = session.getContentType();
            if (mimeType == null || mimeType.isEmpty()) {
                mimeType = inferContentType(filename);
            }
            response.setContentType(mimeType);

            // 🔴 disposition type 必須維持 inline（§4.9.5）：改成 attachment 會讓「預覽」變成
            // 下載檔案，而編譯與測試全綠。builder 負責 RFC 6266／5987 編碼——中文檔名直接
            // 拼進 filename="..." 會是亂碼或被截斷。
            response.setHeader(HttpHeaders.CONTENT_DISPOSITION,
                    ContentDisposition.inline().filename(filename, StandardCharsets.UTF_8).build().toString());

            setActualContentLength(response, session.getObjectKey());

            // 流式解密傳輸
            byte[] buffer = new byte[8192];
            int bytesRead;
            while ((bytesRead = decryptedStream.read(buffer)) != -1) {
                outputStream.write(buffer, 0, bytesRead);
            }

            log.info("http.secure.preview.success", "機敏檔案預覽成功",
                    "session_id", sessionId);
        }
    }

    /**
     * 機敏檔案流式讀取
     * <p>
     * 前端呼叫：GET /api/v1/app/media/secure_download/{asset_id}
     * 本服務：GET /app/media/secure_download/{asset_id}
     * <p>
     * 依設計書 4.2.3： - 必須帶使用者 access token - Media 驗證使用者身分後，必須確認此 asset_id
     * 與該使用者存在合法關聯 - 若無關聯 → 回 403
     */
    @GetMapping("/secure_download/{asset_id}")
    public void secureDownload(
            @RequestHeader(value = GrpcAuthContext.HEADER_ACCOUNT_ID) Long accountId,
            @RequestHeader(value = GrpcAuthContext.HEADER_ACCOUNT_TYPE) String accountType,
            @PathVariable("asset_id") String assetId,
            HttpServletResponse response
    ) throws IOException {
        // 授權：BUYER / SELLER / ADMIN 可下載機敏檔案
        validateAccountType(accountType, AccountType.BUYER, AccountType.SELLER, AccountType.ADMIN);

        log.info("http.secure.download.start", "收到機敏檔案下載請求",
                "asset_id", assetId);

        InputValidator.requireNotEmpty(assetId, "資產識別碼");
        Long assetIdLong = Long.parseLong(assetId);

        // 查詢資產
        MediaAssetEntity asset = mediaAssetService.getAsset(assetIdLong);
        if (asset == null) {
            throw MediaServiceException.notFound("資產不存在: " + assetId);
        }

        // 驗證是否為機敏檔案
        if (!Boolean.TRUE.equals(asset.getIsSecure())) {
            throw MediaServiceException.invalidArgument(
                    "此資產不是機敏檔案，請使用公開 URL 存取");
        }

        // 驗證使用者權限（依設計書 4.2.3）
        // 資產只能被本人或後台稽核角色讀取
        boolean hasAccess = accountId.equals(asset.getUploaderAccountId())
                || assetReferenceRepository.hasAccessByAccount(assetIdLong, accountId);

        if (!hasAccess) {
            // 越權防枚舉（API 設計規範 §14.1）：回應與「資產不存在」一致
            log.warn("denied", "無權限存取機敏資產, asset_id=" + assetId + ", account_id=" + accountId);
            throw MediaServiceException.notFound("資產不存在: " + assetId);
        }

        // 執行流式解密下載
        String encryptionKey = asset.getEncryptionKey();
        if (encryptionKey == null || encryptionKey.isEmpty()) {
            throw MediaServiceException.internal("此資產缺少加密金鑰");
        }

        try (InputStream ossStream = storageService.getObjectStream(asset.getObjectKey());
             CipherInputStream decryptedStream = aesStreamCipher.decryptingStream(ossStream, encryptionKey);
             OutputStream outputStream = response.getOutputStream()) {

            response.setContentType(asset.getMimeType());
            response.setHeader(HttpHeaders.CONTENT_DISPOSITION,
                    ContentDisposition.attachment()
                            .filename(AssetFilename.resolveDisplayName(
                                    asset.getOriginalFilename(), asset.getObjectKey()),
                                    StandardCharsets.UTF_8)
                            .build().toString());

            setActualContentLength(response, asset.getObjectKey());

            // 流式解密傳輸
            byte[] buffer = new byte[8192];
            int bytesRead;
            while ((bytesRead = decryptedStream.read(buffer)) != -1) {
                outputStream.write(buffer, 0, bytesRead);
            }

            log.info("http.secure.download.success", "機敏檔案下載成功",
                    "asset_id", assetId);
        }
    }

    /**
     * 驗證 user-type 是否為允許的角色
     */
    private void validateAccountType(String accountType, AccountType... allowed) {
        if (accountType == null || accountType.isBlank()) {
            throw MediaServiceException.permissionDenied("缺少使用者類型資訊");
        }
        try {
            AccountType actual = AccountTypeRules.parse(accountType);
            for (AccountType required : allowed) {
                if (AccountTypeRules.covers(actual, required)) return;
            }
            throw MediaServiceException.permissionDenied("無權操作：您的帳號類型無法執行此操作");
        } catch (IllegalArgumentException e) {
            throw MediaServiceException.permissionDenied("無效的帳號類型: " + accountType);
        }
    }

    /**
     * 設定回應的 Content-Length，來源是 R2 上實際存在的密文大小（HEAD 查得）減去
     * GCM auth tag，而不是任何「建立/轉正當下宣告」的 fileSize——那個值只是 client
     * 在還沒真正上傳前的估計，可能跟實際位元組數對不上（client 誤算、或行動裝置在
     * 宣告後又重新編碼過檔案）。宣告值不可信時，錯誤的 Content-Length 會讓呼叫端
     * 收到被截斷的內容（宣告 &lt; 實際）或永久卡住等永遠不會到的剩餘 bytes（宣告
     * &gt; 實際）——兩者都在 dev 上以受控實驗重現過。
     * <p>
     * 找不到物件（{@code statObject} 回 null）時不設 header，讓後面的
     * {@code getObjectStream} 自然拋出真正的錯誤。
     */
    private void setActualContentLength(HttpServletResponse response, String objectKey) {
        StorageObjectStat stat = storageService.statObject(objectKey);
        if (stat == null) {
            return;
        }
        long plainSize = stat.contentLength() - AesStreamCipher.GCM_TAG_BYTES;
        if (plainSize >= 0) {
            response.setContentLengthLong(plainSize);
        }
    }

    /**
     * 依副檔名推斷 Content-Type
     */
    private String inferContentType(String filename) {
        if (filename == null) {
            return "application/octet-stream";
        }
        String lower = filename.toLowerCase();
        if (lower.endsWith(".jpg") || lower.endsWith(".jpeg")) {
            return "image/jpeg";
        } else if (lower.endsWith(".png")) {
            return "image/png";
        } else if (lower.endsWith(".gif")) {
            return "image/gif";
        } else if (lower.endsWith(".webp")) {
            return "image/webp";
        } else if (lower.endsWith(".pdf")) {
            return "application/pdf";
        }
        return "application/octet-stream";
    }
}
