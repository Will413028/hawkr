package io.github.will413028.hawkr.media.grpc;

import io.github.will413028.hawkr.common.grpc.GrpcAuthContext;
import io.github.will413028.hawkr.common.obs.grpc.GrpcErrorHandler;
import io.github.will413028.hawkr.common.grpc.GrpcServiceHelper;
import io.github.will413028.hawkr.common.obs.log.ObsLogger;
import io.github.will413028.hawkr.common.utils.code.AssetReferenceType;
import io.github.will413028.hawkr.common.utils.code.GrpcResponseCode;
import io.github.will413028.hawkr.common.utils.code.AccountType;
import io.github.will413028.hawkr.common.utils.proto.ProtoConversionUtils;
import io.github.will413028.hawkr.media.converter.MediaAssetConverter;
import io.github.will413028.hawkr.media.entity.MediaAssetEntity;
import io.github.will413028.hawkr.media.dto.AssetMetadataDTO;
import io.github.will413028.hawkr.media.dto.FinalizedAssetDTO;
import io.github.will413028.hawkr.media.dto.SecureAssetContentDTO;
import io.github.will413028.hawkr.media.exception.MediaServiceException;
import io.github.will413028.hawkr.media.service.AssetReferenceService;
import io.github.will413028.hawkr.media.service.ImportAssetService;
import io.github.will413028.hawkr.media.service.MediaAssetService;
import io.github.will413028.hawkr.media.service.UploadSessionService;
import io.grpc.Status;
import io.grpc.StatusRuntimeException;

import java.util.UUID;
import io.grpc.stub.StreamObserver;
import lombok.RequiredArgsConstructor;
import net.devh.boot.grpc.server.service.GrpcService;

import java.util.List;

/**
 * Media gRPC 服務實作
 * <p>
 * 依設計書 3.2 定義的內部 gRPC 介面
 */
@GrpcService
@RequiredArgsConstructor
public class MediaGrpcService extends MediaServiceGrpc.MediaServiceImplBase {

    private static final ObsLogger log = ObsLogger.forClass(MediaGrpcService.class);

    private final MediaAssetService mediaAssetService;
    private final AssetReferenceService assetReferenceService;
    private final MediaAssetConverter mediaAssetConverter;
    private final UploadSessionService uploadSessionService;
    private final ImportAssetService importAssetService;

    /**
     * 轉正圖片
     * <p>
     * 依設計書 3.2.2 驗證規則：
     * - 解析 staged_token，取得 session_id 與 account_id
     * - upload_session 必須存在、未過期，且狀態為 COMPLETED
     * - 帳號驗證：token.account_id 必須等於 caller_subject.account_id
     * <p>
     * 依設計書 3.2.4 重試語意：
     * - 若同一 staged_token 已完成轉正（FINALIZED），不報錯，回傳既有 asset
     */
    @Override
    public void finalizeImages(
        FinalizeImagesRequest request,
        StreamObserver<FinalizeImagesResponse> responseObserver
    ) {
        log.info("MediaGrpcService.finalize.start", "收到 FinalizeImages 請求",
            "token_count", String.valueOf(request.getStagedTokensCount()));

        try {
            // 授權：BUYER / SELLER / ADMIN 可轉正圖片
            GrpcAuthContext.requireAccountType(AccountType.BUYER, AccountType.SELLER, AccountType.ADMIN);
            long callerAccountId = GrpcAuthContext.requireAccountId();

            // 驗證參數
            if (request.getStagedTokensList().isEmpty()) {
                responseObserver.onNext(FinalizeImagesResponse.newBuilder()
                    .setCode(GrpcResponseCode.INVALID_PARAM.code())
                    .setMessage("staged_tokens 不可為空")
                    .build());
                responseObserver.onCompleted();
                return;
            }

            // ref_type 驗證與 fallback
            String refType = request.getRefType();
            if (refType.isEmpty()) {
                refType = AssetReferenceType.OTHER.value();
            }
            if (!AssetReferenceType.isValidValue(refType)) {
                log.warn("MediaGrpcService.finalize.invalidRefType",
                    "收到無效的 ref_type", "ref_type", refType);
                responseObserver.onNext(FinalizeImagesResponse.newBuilder()
                    .setCode(GrpcResponseCode.INVALID_PARAM.code())
                    .setMessage("無效的 ref_type: " + refType)
                    .build());
                responseObserver.onCompleted();
                return;
            }

            // 呼叫 Service
            List<FinalizedAssetDTO> results = mediaAssetService.finalizeImages(
                request.getStagedTokensList(),
                callerAccountId,
                refType
            );

            // 轉換為 Proto 回應
            FinalizeImagesResponse response = FinalizeImagesResponse.newBuilder()
                .setCode(GrpcResponseCode.SUCCESS.code())
                .setMessage("轉正完成")
                .addAllAssets(mediaAssetConverter.toProtoList(results))
                .build();

            log.info("MediaGrpcService.finalize.success", "FinalizeImages 完成",
                "asset_count", String.valueOf(results.size()));

            responseObserver.onNext(response);
            responseObserver.onCompleted();

        } catch (StatusRuntimeException e) {
            responseObserver.onNext(FinalizeImagesResponse.newBuilder()
                .setCode(e.getStatus().getCode() == Status.Code.UNAUTHENTICATED
                        ? GrpcResponseCode.UNAUTHORIZED.code()
                        : GrpcResponseCode.PERMISSION_DENIED.code())
                .setMessage(e.getStatus().getDescription())
                .build());
            responseObserver.onCompleted();
        } catch (MediaServiceException e) {
            log.warn("MediaGrpcService.finalize.error", e.getMessage());
            responseObserver.onNext(FinalizeImagesResponse.newBuilder()
                .setCode(mapToResponseCode(e.getGrpcStatusCode()))
                .setMessage(e.getMessage())
                .build());
            responseObserver.onCompleted();
        } catch (Exception e) {
            GrpcErrorHandler.handleException(responseObserver, FinalizeImagesResponse.newBuilder(), e, "MediaGrpcService", "finalizeImages");
        }
    }

    /**
     * 建立資產引用
     * <p>
     * 依設計書 3.2.4：
     * - 使用 ON CONFLICT DO NOTHING，多次呼叫視為冪等
     */
    @Override
    public void linkImages(
        LinkImagesRequest request,
        StreamObserver<LinkImagesResponse> responseObserver
    ) {
        log.info("MediaGrpcService.link.start",
            "收到 LinkImages 請求, ref_type=" + request.getRefType() + ", ref_id=" + request.getRefId());

        try {
            // 驗證參數
            if (request.getRefType().isEmpty()) {
                responseObserver.onNext(LinkImagesResponse.newBuilder()
                    .setCode(GrpcResponseCode.INVALID_PARAM.code())
                    .setMessage("ref_type 不可為空")
                    .build());
                responseObserver.onCompleted();
                return;
            }
            if (request.getRefId().isEmpty()) {
                responseObserver.onNext(LinkImagesResponse.newBuilder()
                    .setCode(GrpcResponseCode.INVALID_PARAM.code())
                    .setMessage("ref_id 不可為空")
                    .build());
                responseObserver.onCompleted();
                return;
            }
            if (request.getAssetIdsList().isEmpty()) {
                responseObserver.onNext(LinkImagesResponse.newBuilder()
                    .setCode(GrpcResponseCode.INVALID_PARAM.code())
                    .setMessage("asset_ids 不可為空")
                    .build());
                responseObserver.onCompleted();
                return;
            }
            if (!AssetReferenceType.isValidValue(request.getRefType())) {
                log.warn("MediaGrpcService.link.invalidRefType",
                    "收到無效的 ref_type", "ref_type", request.getRefType());
                responseObserver.onNext(LinkImagesResponse.newBuilder()
                    .setCode(GrpcResponseCode.INVALID_PARAM.code())
                    .setMessage("無效的 ref_type: " + request.getRefType())
                    .build());
                responseObserver.onCompleted();
                return;
            }

            // 呼叫 Service
            int linkedCount = assetReferenceService.linkImages(
                request.getRefType(),
                request.getRefId(),
                request.getAssetIdsList()
            );

            // 建立回應
            LinkImagesResponse response = LinkImagesResponse.newBuilder()
                .setCode(GrpcResponseCode.SUCCESS.code())
                .setMessage("連結完成")
                .setLinkedCount(String.valueOf(linkedCount))
                .build();

            log.info("MediaGrpcService.link.success", "LinkImages 完成",
                "linked_count", String.valueOf(linkedCount));

            responseObserver.onNext(response);
            responseObserver.onCompleted();

        } catch (MediaServiceException e) {
            log.warn("MediaGrpcService.link.error", e.getMessage());
            responseObserver.onNext(LinkImagesResponse.newBuilder()
                .setCode(mapToResponseCode(e.getGrpcStatusCode()))
                .setMessage(e.getMessage())
                .build());
            responseObserver.onCompleted();
        } catch (Exception e) {
            GrpcErrorHandler.handleException(responseObserver, LinkImagesResponse.newBuilder(), e, "MediaGrpcService", "linkImages");
        }
    }

    /**
     * 解除資產引用
     */
    @Override
    public void unlinkImages(
        UnlinkImagesRequest request,
        StreamObserver<UnlinkImagesResponse> responseObserver
    ) {
        log.info("MediaGrpcService.unlink.start",
            "收到 UnlinkImages 請求, ref_type=" + request.getRefType() + ", ref_id=" + request.getRefId());

        try {
            // 驗證參數
            if (request.getRefType().isEmpty()) {
                responseObserver.onNext(UnlinkImagesResponse.newBuilder()
                    .setCode(GrpcResponseCode.INVALID_PARAM.code())
                    .setMessage("ref_type 不可為空")
                    .build());
                responseObserver.onCompleted();
                return;
            }
            if (request.getRefId().isEmpty()) {
                responseObserver.onNext(UnlinkImagesResponse.newBuilder()
                    .setCode(GrpcResponseCode.INVALID_PARAM.code())
                    .setMessage("ref_id 不可為空")
                    .build());
                responseObserver.onCompleted();
                return;
            }
            if (request.getAssetIdsList().isEmpty()) {
                responseObserver.onNext(UnlinkImagesResponse.newBuilder()
                    .setCode(GrpcResponseCode.INVALID_PARAM.code())
                    .setMessage("asset_ids 不可為空")
                    .build());
                responseObserver.onCompleted();
                return;
            }
            if (!AssetReferenceType.isValidValue(request.getRefType())) {
                log.warn("MediaGrpcService.unlink.invalidRefType",
                    "收到無效的 ref_type", "ref_type", request.getRefType());
                responseObserver.onNext(UnlinkImagesResponse.newBuilder()
                    .setCode(GrpcResponseCode.INVALID_PARAM.code())
                    .setMessage("無效的 ref_type: " + request.getRefType())
                    .build());
                responseObserver.onCompleted();
                return;
            }

            // 呼叫 Service
            int unlinkedCount = assetReferenceService.unlinkImages(
                request.getRefType(),
                request.getRefId(),
                request.getAssetIdsList()
            );

            // 建立回應
            UnlinkImagesResponse response = UnlinkImagesResponse.newBuilder()
                .setCode(GrpcResponseCode.SUCCESS.code())
                .setMessage("解除連結完成")
                .setUnlinkedCount(String.valueOf(unlinkedCount))
                .build();

            log.info("MediaGrpcService.unlink.success", "UnlinkImages 完成",
                "unlinked_count", String.valueOf(unlinkedCount));

            responseObserver.onNext(response);
            responseObserver.onCompleted();

        } catch (MediaServiceException e) {
            log.warn("MediaGrpcService.unlink.error", e.getMessage());
            responseObserver.onNext(UnlinkImagesResponse.newBuilder()
                .setCode(mapToResponseCode(e.getGrpcStatusCode()))
                .setMessage(e.getMessage())
                .build());
            responseObserver.onCompleted();
        } catch (Exception e) {
            GrpcErrorHandler.handleException(responseObserver, UnlinkImagesResponse.newBuilder(), e, "MediaGrpcService", "unlinkImages");
        }
    }

    /**
     * 釋放引用：依 (ref_type, ref_id) 批次移除關聯（不需 asset_ids）
     */
    @Override
    public void releaseReferences(
        ReleaseReferencesRequest request,
        StreamObserver<ReleaseReferencesResponse> responseObserver
    ) {
        log.info("MediaGrpcService.release.start",
            "收到 ReleaseReferences 請求, ref_type=" + request.getRefType()
                + ", ref_ids_count=" + request.getRefIdsCount());

        try {
            if (request.getRefType().isEmpty()) {
                responseObserver.onNext(ReleaseReferencesResponse.newBuilder()
                    .setCode(GrpcResponseCode.INVALID_PARAM.code())
                    .setMessage("ref_type 不可為空")
                    .build());
                responseObserver.onCompleted();
                return;
            }
            if (request.getRefIdsList().isEmpty()) {
                responseObserver.onNext(ReleaseReferencesResponse.newBuilder()
                    .setCode(GrpcResponseCode.INVALID_PARAM.code())
                    .setMessage("ref_ids 不可為空")
                    .build());
                responseObserver.onCompleted();
                return;
            }
            if (!AssetReferenceType.isValidValue(request.getRefType())) {
                log.warn("MediaGrpcService.release.invalidRefType",
                    "收到無效的 ref_type", "ref_type", request.getRefType());
                responseObserver.onNext(ReleaseReferencesResponse.newBuilder()
                    .setCode(GrpcResponseCode.INVALID_PARAM.code())
                    .setMessage("無效的 ref_type: " + request.getRefType())
                    .build());
                responseObserver.onCompleted();
                return;
            }

            int releasedCount = assetReferenceService.releaseReferences(
                request.getRefType(),
                request.getRefIdsList(),
                request.getKeepAssetIdsList()
            );

            ReleaseReferencesResponse response = ReleaseReferencesResponse.newBuilder()
                .setCode(GrpcResponseCode.SUCCESS.code())
                .setMessage("引用釋放完成")
                .setReleasedCount(String.valueOf(releasedCount))
                .build();

            log.info("MediaGrpcService.release.success", "ReleaseReferences 完成",
                "released_count", String.valueOf(releasedCount));

            responseObserver.onNext(response);
            responseObserver.onCompleted();

        } catch (MediaServiceException e) {
            log.warn("MediaGrpcService.release.error", e.getMessage());
            responseObserver.onNext(ReleaseReferencesResponse.newBuilder()
                .setCode(mapToResponseCode(e.getGrpcStatusCode()))
                .setMessage(e.getMessage())
                .build());
            responseObserver.onCompleted();
        } catch (Exception e) {
            GrpcErrorHandler.handleException(responseObserver,
                ReleaseReferencesResponse.newBuilder(), e, "MediaGrpcService", "releaseReferences");
        }
    }

    /**
     * 建立上傳會話
     * <p>
     * Gateway 將 HTTP 請求轉換為 gRPC 呼叫
     */
    @Override
    public void createUploadSession(
        CreateUploadSessionRequest request,
        StreamObserver<CreateUploadSessionResponse> responseObserver
    ) {
        log.info("MediaGrpcService.session.create.start", "收到 CreateUploadSession 請求");

        try {
            // 授權：BUYER / SELLER / ADMIN 可建立上傳會話
            GrpcAuthContext.requireAccountType(AccountType.BUYER, AccountType.SELLER, AccountType.ADMIN);
            long accountId = GrpcAuthContext.requireAccountId();

            // 驗證參數
            if (request.getFilename().isEmpty()) {
                responseObserver.onNext(CreateUploadSessionResponse.newBuilder()
                    .setCode(GrpcResponseCode.INVALID_PARAM.code())
                    .setMessage("filename 不可為空")
                    .build());
                responseObserver.onCompleted();
                return;
            }
            if (request.getContentType().isEmpty()) {
                responseObserver.onNext(CreateUploadSessionResponse.newBuilder()
                    .setCode(GrpcResponseCode.INVALID_PARAM.code())
                    .setMessage("content_type 不可為空")
                    .build());
                responseObserver.onCompleted();
                return;
            }

            // file_size: Proto string → Java Long
            long fileSize = parseLong(request.getFileSize(), "file_size");
            if (fileSize <= 0) {
                responseObserver.onNext(CreateUploadSessionResponse.newBuilder()
                    .setCode(GrpcResponseCode.INVALID_PARAM.code())
                    .setMessage("file_size 必須大於 0")
                    .build());
                responseObserver.onCompleted();
                return;
            }

            // 轉換為內部 DTO
            io.github.will413028.hawkr.media.dto.request.CreateUploadSessionRequest internalRequest =
                io.github.will413028.hawkr.media.dto.request.CreateUploadSessionRequest.builder()
                    .filename(request.getFilename())
                    .contentType(request.getContentType())
                    .fileSize(fileSize)
                    .isSecure(request.getIsSecure())
                    .build();

            // 呼叫 Service
            io.github.will413028.hawkr.media.dto.response.CreateUploadSessionResponse serviceResponse =
                uploadSessionService.createSession(accountId, internalRequest);

            // 轉換為 Proto 回應
            CreateUploadSessionResponse response = CreateUploadSessionResponse.newBuilder()
                .setCode(GrpcResponseCode.SUCCESS.code())
                .setMessage("上傳會話建立成功")
                .setSessionId(serviceResponse.getSessionId())
                .setUploadUrl(serviceResponse.getUploadUrl() != null ? serviceResponse.getUploadUrl() : "")
                .setExpiresIn(serviceResponse.getExpiresIn() != null ? String.valueOf(serviceResponse.getExpiresIn()) : "0")
                .setIsSecure(Boolean.TRUE.equals(serviceResponse.getIsSecure()))
                .build();

            log.info("MediaGrpcService.session.create.success", "CreateUploadSession 完成",
                "session_id", serviceResponse.getSessionId());

            responseObserver.onNext(response);
            responseObserver.onCompleted();

        } catch (StatusRuntimeException e) {
            responseObserver.onNext(CreateUploadSessionResponse.newBuilder()
                .setCode(e.getStatus().getCode() == Status.Code.UNAUTHENTICATED
                        ? GrpcResponseCode.UNAUTHORIZED.code()
                        : GrpcResponseCode.PERMISSION_DENIED.code())
                .setMessage(e.getStatus().getDescription())
                .build());
            responseObserver.onCompleted();
        } catch (MediaServiceException e) {
            log.warn("MediaGrpcService.session.create.error", e.getMessage());
            responseObserver.onNext(CreateUploadSessionResponse.newBuilder()
                .setCode(mapToResponseCode(e.getGrpcStatusCode()))
                .setMessage(e.getMessage())
                .build());
            responseObserver.onCompleted();
        } catch (Exception e) {
            GrpcErrorHandler.handleException(responseObserver, CreateUploadSessionResponse.newBuilder(), e, "MediaGrpcService", "createUploadSession");
        }
    }

    /**
     * 完成上傳會話
     * <p>
     * 前端上傳 OSS 完畢後呼叫，回傳 staged_token
     */
    @Override
    public void completeUploadSession(
        CompleteUploadSessionRequest request,
        StreamObserver<CompleteUploadSessionResponse> responseObserver
    ) {
        log.info("MediaGrpcService.session.complete.start", "收到 CompleteUploadSession 請求",
            "session_id", request.getSessionId());

        try {
            // 授權：BUYER / SELLER / ADMIN 可完成上傳會話
            GrpcAuthContext.requireAccountType(AccountType.BUYER, AccountType.SELLER, AccountType.ADMIN);
            long accountId = GrpcAuthContext.requireAccountId();

            // 驗證參數
            if (request.getSessionId().isEmpty()) {
                responseObserver.onNext(CompleteUploadSessionResponse.newBuilder()
                    .setCode(GrpcResponseCode.INVALID_PARAM.code())
                    .setMessage("session_id 不可為空")
                    .build());
                responseObserver.onCompleted();
                return;
            }

            UUID sessionId;
            try {
                sessionId = UUID.fromString(request.getSessionId());
            } catch (IllegalArgumentException e) {
                log.warn("MediaGrpcService.session.complete.invalid", "無效的 session_id 格式: " + request.getSessionId());
                responseObserver.onNext(CompleteUploadSessionResponse.newBuilder()
                    .setCode(GrpcResponseCode.INVALID_PARAM.code())
                    .setMessage("無效的 session_id 格式")
                    .build());
                responseObserver.onCompleted();
                return;
            }

            // 呼叫 Service
            io.github.will413028.hawkr.media.dto.response.CompleteUploadSessionResponse serviceResponse =
                uploadSessionService.completeSession(accountId, sessionId, null);

            // 轉換為 Proto 回應
            CompleteUploadSessionResponse response = CompleteUploadSessionResponse.newBuilder()
                .setCode(GrpcResponseCode.SUCCESS.code())
                .setMessage("上傳會話完成")
                .setStagedToken(serviceResponse.getStagedToken())
                .setPreviewUrl(serviceResponse.getPreviewUrl() != null ? serviceResponse.getPreviewUrl() : "")
                .build();

            log.info("MediaGrpcService.session.complete.success", "CompleteUploadSession 完成",
                "session_id", request.getSessionId());

            responseObserver.onNext(response);
            responseObserver.onCompleted();

        } catch (StatusRuntimeException e) {
            responseObserver.onNext(CompleteUploadSessionResponse.newBuilder()
                .setCode(e.getStatus().getCode() == Status.Code.UNAUTHENTICATED
                        ? GrpcResponseCode.UNAUTHORIZED.code()
                        : GrpcResponseCode.PERMISSION_DENIED.code())
                .setMessage(e.getStatus().getDescription())
                .build());
            responseObserver.onCompleted();
        } catch (MediaServiceException e) {
            log.warn("MediaGrpcService.session.complete.error", e.getMessage());
            responseObserver.onNext(CompleteUploadSessionResponse.newBuilder()
                .setCode(mapToResponseCode(e.getGrpcStatusCode()))
                .setMessage(e.getMessage())
                .build());
            responseObserver.onCompleted();
        } catch (Exception e) {
            GrpcErrorHandler.handleException(responseObserver, CompleteUploadSessionResponse.newBuilder(), e, "MediaGrpcService", "completeUploadSession");
        }
    }

    /**
     * 後端直傳並轉正（服務間呼叫）
     * <p>
     * Shipping Service 用於上傳宅配 PDF 託運單。
     * 此 RPC 為內部呼叫（x-internal-service），不需前端身分驗證。
     */
    @Override
    public void serverUploadAndFinalize(
        ServerUploadAndFinalizeRequest request,
        StreamObserver<ServerUploadAndFinalizeResponse> responseObserver
    ) {
        log.info("MediaGrpcService.serverUpload.start", "收到 ServerUploadAndFinalize 請求",
            "filename", request.getFilename());

        try {
            // 驗證參數
            if (request.getFileData().isEmpty()) {
                responseObserver.onNext(ServerUploadAndFinalizeResponse.newBuilder()
                    .setCode(GrpcResponseCode.INVALID_PARAM.code())
                    .setMessage("file_data 不可為空")
                    .build());
                responseObserver.onCompleted();
                return;
            }
            if (request.getFilename().isEmpty()) {
                responseObserver.onNext(ServerUploadAndFinalizeResponse.newBuilder()
                    .setCode(GrpcResponseCode.INVALID_PARAM.code())
                    .setMessage("filename 不可為空")
                    .build());
                responseObserver.onCompleted();
                return;
            }
            if (request.getContentType().isEmpty()) {
                responseObserver.onNext(ServerUploadAndFinalizeResponse.newBuilder()
                    .setCode(GrpcResponseCode.INVALID_PARAM.code())
                    .setMessage("content_type 不可為空")
                    .build());
                responseObserver.onCompleted();
                return;
            }

            // ref_type 驗證與 fallback
            String refType = request.getRefType();
            if (refType.isEmpty()) {
                refType = AssetReferenceType.OTHER.value();
            }
            if (!AssetReferenceType.isValidValue(refType)) {
                log.warn("MediaGrpcService.serverUpload.invalidRefType",
                    "收到無效的 ref_type", "ref_type", refType);
                responseObserver.onNext(ServerUploadAndFinalizeResponse.newBuilder()
                    .setCode(GrpcResponseCode.INVALID_PARAM.code())
                    .setMessage("無效的 ref_type: " + refType)
                    .build());
                responseObserver.onCompleted();
                return;
            }

            // 呼叫 Service
            byte[] fileData = request.getFileData().toByteArray();
            io.github.will413028.hawkr.media.entity.MediaAssetEntity asset =
                mediaAssetService.serverUploadAndFinalize(fileData, request.getFilename(), request.getContentType(), refType);

            // 建立回應
            ServerUploadAndFinalizeResponse response = ServerUploadAndFinalizeResponse.newBuilder()
                .setCode(GrpcResponseCode.SUCCESS.code())
                .setMessage("上傳並轉正完成")
                .setAssetId(String.valueOf(asset.getId()))
                .setObjectKey(asset.getObjectKey())
                .build();

            log.info("MediaGrpcService.serverUpload.success", "ServerUploadAndFinalize 完成",
                "asset_id", String.valueOf(asset.getId()));

            responseObserver.onNext(response);
            responseObserver.onCompleted();

        } catch (MediaServiceException e) {
            log.warn("MediaGrpcService.serverUpload.error", e.getMessage());
            responseObserver.onNext(ServerUploadAndFinalizeResponse.newBuilder()
                .setCode(mapToResponseCode(e.getGrpcStatusCode()))
                .setMessage(e.getMessage())
                .build());
            responseObserver.onCompleted();
        } catch (Exception e) {
            GrpcErrorHandler.handleException(responseObserver, ServerUploadAndFinalizeResponse.newBuilder(), e, "MediaGrpcService", "serverUploadAndFinalize");
        }
    }

    /**
     * 機敏檔後端直傳並轉正（58 §8.1.5）。發票 PDF 走這一支。
     *
     * <p>與 {@link #serverUploadAndFinalize} 的差別見
     * {@link MediaAssetService#serverUploadSecureAndFinalize}。這裡多兩道驗證：
     * refType 與 refId <b>都必填且不 fallback</b>——那支對空 refType 會 fallback 成
     * {@code OTHER}，而機敏檔用 OTHER 等於沒有引用歸屬，{@code releaseReferences}
     * 之後無從辨識、GC 也無從保護。
     */
    @Override
    public void serverUploadSecureAndFinalize(
        ServerUploadSecureAndFinalizeRequest request,
        StreamObserver<ServerUploadSecureAndFinalizeResponse> responseObserver
    ) {
        log.info("MediaGrpcService.serverUploadSecure.start", "收到 ServerUploadSecureAndFinalize 請求",
            "filename", request.getFilename());

        try {
            String bad = null;
            if (request.getFileData().isEmpty()) {
                bad = "file_data 不可為空";
            } else if (request.getFilename().isEmpty()) {
                bad = "filename 不可為空";
            } else if (request.getContentType().isEmpty()) {
                bad = "content_type 不可為空";
            } else if (request.getRefType().isEmpty()) {
                // 🔴 不 fallback 成 OTHER：機敏檔沒有引用歸屬 ⇒ GC 無從保護、釋放無從辨識。
                bad = "ref_type 不可為空（機敏檔必須有引用歸屬）";
            } else if (request.getRefId().isEmpty()) {
                bad = "ref_id 不可為空（機敏檔必須有引用歸屬）";
            } else if (!AssetReferenceType.isValidValue(request.getRefType())) {
                bad = "無效的 ref_type: " + request.getRefType();
            }
            if (bad != null) {
                responseObserver.onNext(ServerUploadSecureAndFinalizeResponse.newBuilder()
                    .setCode(GrpcResponseCode.INVALID_PARAM.code())
                    .setMessage(bad)
                    .build());
                responseObserver.onCompleted();
                return;
            }

            MediaAssetEntity asset = mediaAssetService.serverUploadSecureAndFinalize(
                request.getFileData().toByteArray(),
                request.getFilename(),
                request.getContentType(),
                request.getRefType(),
                request.getRefId());

            responseObserver.onNext(ServerUploadSecureAndFinalizeResponse.newBuilder()
                .setCode(GrpcResponseCode.SUCCESS.code())
                .setMessage("上傳並轉正完成")
                .setAssetId(String.valueOf(asset.getId()))
                .setObjectKey(asset.getObjectKey())
                .build());
            responseObserver.onCompleted();
        } catch (MediaServiceException e) {
            log.warn("MediaGrpcService.serverUploadSecure.error", e.getMessage());
            responseObserver.onNext(ServerUploadSecureAndFinalizeResponse.newBuilder()
                .setCode(mapToResponseCode(e.getGrpcStatusCode()))
                .setMessage(e.getMessage())
                .build());
            responseObserver.onCompleted();
        } catch (Exception e) {
            // 與既有那支同一條路徑：錯誤碼映射與 log 形狀集中在 GrpcErrorHandler，
            // 自己寫一份 catch 會漂移出不同的碼與不同的欄位名。
            GrpcErrorHandler.handleException(responseObserver, ServerUploadSecureAndFinalizeResponse.newBuilder(), e, "MediaGrpcService", "serverUploadSecureAndFinalize");
        }
    }

    /**
     * 服務端讀取機敏資產（58 §8.1.6）。發票 PDF 的買家／賣家／ADM 三條下載路徑都經由
     * invoice-service 轉呼叫這一支取得位元組。
     *
     * <p><b>internal-only</b>：它把一份機敏檔的明文整個吐出來，而且<b>沒有任何使用者授權檢查</b>
     * ——因為呼叫端不是使用者。少了 {@code isInternalCall()} 這道閘門，任何持有 JWT 的人只要
     * 猜得到 object_key 與 (ref_type, ref_id) 就能讀走別人的發票。簽章由 server interceptor 驗
     * （52 號 HMAC 四 header），這裡只讀 interceptor 的判定結果。
     *
     * <p>「誰能看這份發票」的判斷留在 invoice-service：它才知道這張單的買家、賣家與代行關係。
     * media 只驗「呼叫端說得出的三元組是否真的指向這份資產」。
     */
    @Override
    public void serverReadSecureAsset(
        ServerReadSecureAssetRequest request,
        StreamObserver<ServerReadSecureAssetResponse> responseObserver
    ) {
        log.info("MediaGrpcService.serverReadSecure.start", "收到 ServerReadSecureAsset 請求",
            "object_key", request.getObjectKey());

        try {
            if (!GrpcAuthContext.isInternalCall()) {
                throw Status.PERMISSION_DENIED
                    .withDescription("ServerReadSecureAsset 僅接受內部服務呼叫")
                    .asRuntimeException();
            }

            // 三個欄位合起來就是授權述詞，缺一即無述詞可驗——不 fallback、不放行。
            String bad = null;
            if (request.getObjectKey().isEmpty()) {
                bad = "object_key 不可為空";
            } else if (request.getRefType().isEmpty()) {
                bad = "ref_type 不可為空（三元組是本 RPC 的授權述詞）";
            } else if (request.getRefId().isEmpty()) {
                bad = "ref_id 不可為空（三元組是本 RPC 的授權述詞）";
            }
            if (bad != null) {
                responseObserver.onNext(ServerReadSecureAssetResponse.newBuilder()
                    .setCode(GrpcResponseCode.INVALID_PARAM.code())
                    .setMessage(bad)
                    .build());
                responseObserver.onCompleted();
                return;
            }

            SecureAssetContentDTO content = mediaAssetService.serverReadSecureAsset(
                request.getObjectKey(), request.getRefType(), request.getRefId());

            responseObserver.onNext(ServerReadSecureAssetResponse.newBuilder()
                .setCode(GrpcResponseCode.SUCCESS.code())
                .setMessage("讀取成功")
                .setFileData(com.google.protobuf.ByteString.copyFrom(content.fileData()))
                .setContentType(content.contentType())
                .setFilename(content.filename())
                .build());
            responseObserver.onCompleted();
        } catch (MediaServiceException e) {
            log.warn("MediaGrpcService.serverReadSecure.error", e.getMessage());
            responseObserver.onNext(ServerReadSecureAssetResponse.newBuilder()
                .setCode(mapToResponseCode(e.getGrpcStatusCode()))
                .setMessage(e.getMessage())
                .build());
            responseObserver.onCompleted();
        } catch (Exception e) {
            GrpcErrorHandler.handleException(responseObserver, ServerReadSecureAssetResponse.newBuilder(), e, "MediaGrpcService", "serverReadSecureAsset");
        }
    }

    /**
     * 資產中介資料批次查詢（#3031）。回 asset_id／檔名／大小／上傳時間，<b>不回 bytes</b>。
     *
     * <p><b>internal-only ＋ 三元組全等</b>，與 {@link #serverReadSecureAsset} 逐字相同的
     * 授權述詞。它回的是中介資料不是內容，但「這個 object_key 存不存在」本身就是資訊
     * ⇒ 查無與越權必須無法分辨（通不過的 key 就是不出現在結果裡）。
     */
    @Override
    public void batchGetAssetMetadata(
        BatchGetAssetMetadataRequest request,
        StreamObserver<BatchGetAssetMetadataResponse> responseObserver
    ) {
        try {
            if (!GrpcAuthContext.isInternalCall()) {
                throw Status.PERMISSION_DENIED
                    .withDescription("BatchGetAssetMetadata 僅接受內部服務呼叫")
                    .asRuntimeException();
            }

            String bad = null;
            if (request.getRefType().isEmpty()) {
                bad = "ref_type 不可為空（三元組是本 RPC 的授權述詞）";
            } else if (request.getRefId().isEmpty()) {
                bad = "ref_id 不可為空（三元組是本 RPC 的授權述詞）";
            } else if (request.getObjectKeysCount() > MAX_METADATA_KEYS) {
                // 上限而不是靜默截斷：截斷會讓被丟掉的那幾份附件在畫面上「消失」，
                // 而畫面看起來完全正常。
                bad = "object_keys 一次最多 " + MAX_METADATA_KEYS + " 筆";
            }
            if (bad != null) {
                responseObserver.onNext(BatchGetAssetMetadataResponse.newBuilder()
                    .setCode(GrpcResponseCode.INVALID_PARAM.code())
                    .setMessage(bad)
                    .build());
                responseObserver.onCompleted();
                return;
            }

            BatchGetAssetMetadataResponse.Builder resp = BatchGetAssetMetadataResponse.newBuilder()
                .setCode(GrpcResponseCode.SUCCESS.code())
                .setMessage("查詢成功");
            for (AssetMetadataDTO m : mediaAssetService.batchGetAssetMetadata(
                    request.getObjectKeysList(), request.getRefType(), request.getRefId())) {
                resp.addData(AssetMetadataItem.newBuilder()
                    .setObjectKey(m.objectKey())
                    .setAssetId(m.assetId())
                    .setFileName(m.fileName())
                    .setSizeBytes(m.sizeBytes())
                    .setUploadedAt(m.uploadedAt())
                    .setMimeType(m.mimeType())
                    .build());
            }
            responseObserver.onNext(resp.build());
            responseObserver.onCompleted();
        } catch (MediaServiceException e) {
            log.warn("MediaGrpcService.batchGetAssetMetadata.error", e.getMessage());
            responseObserver.onNext(BatchGetAssetMetadataResponse.newBuilder()
                .setCode(mapToResponseCode(e.getGrpcStatusCode()))
                .setMessage(e.getMessage())
                .build());
            responseObserver.onCompleted();
        } catch (Exception e) {
            GrpcErrorHandler.handleException(responseObserver, BatchGetAssetMetadataResponse.newBuilder(), e, "MediaGrpcService", "batchGetAssetMetadata");
        }
    }

    /** object_keys 的單次上限，對齊 invoice 端稅籍佐證的實務上限。 */
    private static final int MAX_METADATA_KEYS = 100;

    /**
     * Proto string → Java long 轉換
     * 委派至 ProtoConversionUtils，錯誤時轉為 MediaServiceException
     */
    private long parseLong(String value, String fieldName) {
        try {
            return ProtoConversionUtils.parseLongRequired(value, fieldName);
        } catch (IllegalArgumentException e) {
            throw MediaServiceException.invalidArgument(e.getMessage());
        }
    }

    /**
     * 映射 gRPC Status.Code 到 GrpcResponseCode 的 int code
     * 委派至 GrpcServiceHelper
     */
    private int mapToResponseCode(Status.Code code) {
        if (code == null) {
            return GrpcResponseCode.INTERNAL_ERROR.code();
        }
        return GrpcServiceHelper.mapStatusCode(
                code.toStatus().asRuntimeException());
    }

    // ============================================================================
    // Product Import 整合（08 號設計書 §7）
    //
    // 純內部 RPC：由 x-internal-service metadata 識別免身份驗證（docs/詳細設計書/52-微服務間內部授權（Internal Service Authorization）詳細設計書.md）。
    // ============================================================================

    /**
     * 派工外部 URL 下載（async）。詳見 08 號設計書 §7.2 / §7.2a。
     *
     * <p>RPC 同步階段：dedup → INSERT import_asset → afterCommit publish 內部派工事件。
     * 實際 SafeFetch + OSS 上傳由 {@code MediaImportAssetDownloadListener} 執行；
     * 結果透過 {@code hawkr.media} exchange + {@code import.asset.downloaded} routing key 通知 Import Service。</p>
     */
    @Override
    public void importDownloadAsset(
        ImportDownloadAssetRequest request,
        StreamObserver<ImportDownloadAssetResponse> responseObserver
    ) {
        log.info("MediaGrpcService.importDownloadAsset.start",
            "收到 ImportDownloadAsset 請求",
            "import_job_id",
            "import_job_id=" + request.getImportJobId() + ", import_item_id=" + request.getImportItemId());

        try {
            ImportAssetService.DispatchResult result = importAssetService.importDownloadAsset(
                request.getRefType(),
                request.getSourceUrl(),
                request.getStoreId(),
                request.getImportJobId(),
                request.getImportItemId(),
                request.getExpiresAt(),
                request.getCallbackCorrelationId()
            );

            ImportDownloadAssetResponse.Builder builder = ImportDownloadAssetResponse.newBuilder()
                .setCode(GrpcResponseCode.SUCCESS.code())
                .setMessage(result.reused() ? "dedup 命中既有 import_asset" : "派工成功")
                .setImportAssetId(result.importAssetId())
                .setStatus(result.status());
            // dedup 命中且 settled 時 echo metadata,讓 caller 同步 settle 自己 row (08 號 §7.2)
            if (result.objectKey() != null) builder.setObjectKey(result.objectKey());
            if (result.previewUrl() != null) builder.setPreviewUrl(result.previewUrl());
            if (result.failureReason() != null) builder.setFailureReason(result.failureReason());
            if (result.failureDetail() != null) builder.setFailureDetail(result.failureDetail());
            ImportDownloadAssetResponse response = builder.build();

            log.info("MediaGrpcService.importDownloadAsset.success",
                "派工流程完成",
                "import_asset_id",
                "import_asset_id=" + result.importAssetId() + ", status=" + result.status() + ", reused=" + result.reused());

            responseObserver.onNext(response);
            responseObserver.onCompleted();

        } catch (MediaServiceException e) {
            log.warn("MediaGrpcService.importDownloadAsset.bizError", e.getMessage());
            responseObserver.onNext(ImportDownloadAssetResponse.newBuilder()
                .setCode(mapToResponseCode(e.getGrpcStatusCode()))
                .setMessage(e.getMessage())
                .setStatus("")
                .setFailureReason(e.getMessage())
                .build());
            responseObserver.onCompleted();
        } catch (Exception e) {
            GrpcErrorHandler.handleException(
                responseObserver, ImportDownloadAssetResponse.newBuilder(), e,
                "MediaGrpcService", "importDownloadAsset");
        }
    }

    /**
     * Import asset 轉正為 PRODUCT / PRODUCT_DESC media_asset。詳見 08 號設計書 §7.3。
     *
     * <p>由 Product Service 在 {@code CreateProductFromImportInternal} commit 階段呼叫。
     * 流程：取 import_asset FOR UPDATE → OSS copy/delete → INSERT media_asset + UPDATE FINALIZED。</p>
     */
    @Override
    public void finalizeImportAsset(
        FinalizeImportAssetRequest request,
        StreamObserver<FinalizeImportAssetResponse> responseObserver
    ) {
        log.info("MediaGrpcService.finalizeImportAsset.start",
            "收到 FinalizeImportAsset 請求",
            "import_asset_id",
            "import_asset_id=" + request.getImportAssetId() + ", target_ref_type=" + request.getTargetRefType());

        try {
            ImportAssetService.FinalizeResult result = importAssetService.finalizeImportAsset(
                request.getImportAssetId(),
                request.getTargetRefType(),
                request.getStoreId(),
                request.getProductId()
            );

            FinalizeImportAssetResponse response = FinalizeImportAssetResponse.newBuilder()
                .setCode(GrpcResponseCode.SUCCESS.code())
                .setMessage("轉正完成")
                .setAssetId(ProtoConversionUtils.formatLong(result.assetId()))
                .setObjectKey(result.objectKey() != null ? result.objectKey() : "")
                .setUrl(result.url() != null ? result.url() : "")
                .build();

            log.info("MediaGrpcService.finalizeImportAsset.success",
                "import_asset 轉正完成",
                "import_asset_id",
                "import_asset_id=" + request.getImportAssetId() + ", asset_id=" + result.assetId());

            responseObserver.onNext(response);
            responseObserver.onCompleted();

        } catch (MediaServiceException e) {
            log.warn("MediaGrpcService.finalizeImportAsset.bizError", e.getMessage());
            responseObserver.onNext(FinalizeImportAssetResponse.newBuilder()
                .setCode(mapToResponseCode(e.getGrpcStatusCode()))
                .setMessage(e.getMessage())
                .build());
            responseObserver.onCompleted();
        } catch (Exception e) {
            GrpcErrorHandler.handleException(
                responseObserver, FinalizeImportAssetResponse.newBuilder(), e,
                "MediaGrpcService", "finalizeImportAsset");
        }
    }

    /**
     * 將賣家本地上傳的 staged_token 同步綁定為 import_asset（40 號設計書 §3.4 / §4.3）。
     * <p>由 product-import-service 在 UpdateImportItem 階段呼叫；同步回傳 import_asset_id / object_key / preview_url。</p>
     */
    @Override
    public void bindStagedAsset(
        BindStagedAssetRequest request,
        StreamObserver<BindStagedAssetResponse> responseObserver
    ) {
        log.info("MediaGrpcService.bindStagedAsset.start",
            "收到 BindStagedAsset 請求",
            "import_job_id",
            "import_job_id=" + request.getImportJobId() + ", import_item_id=" + request.getImportItemId());

        try {
            ImportAssetService.BindResult result = importAssetService.bindStagedAsset(
                request.getStagedToken(),
                request.getStoreId(),
                request.getImportJobId(),
                request.getImportItemId(),
                request.getRefType()
            );

            BindStagedAssetResponse response = BindStagedAssetResponse.newBuilder()
                .setCode(GrpcResponseCode.SUCCESS.code())
                .setMessage(result.reused() ? "dedup 命中既有 STAGED import_asset" : "綁定成功")
                .setImportAssetId(result.importAssetId())
                .setObjectKey(result.objectKey() != null ? result.objectKey() : "")
                .setPreviewUrl(result.previewUrl() != null ? result.previewUrl() : "")
                .build();

            log.info("MediaGrpcService.bindStagedAsset.success",
                "BindStagedAsset 完成",
                "import_asset_id",
                "import_asset_id=" + result.importAssetId() + ", reused=" + result.reused());

            responseObserver.onNext(response);
            responseObserver.onCompleted();

        } catch (MediaServiceException e) {
            log.warn("MediaGrpcService.bindStagedAsset.bizError", e.getMessage());
            responseObserver.onNext(BindStagedAssetResponse.newBuilder()
                .setCode(mapToResponseCode(e.getGrpcStatusCode()))
                .setMessage(e.getMessage())
                .build());
            responseObserver.onCompleted();
        } catch (Exception e) {
            GrpcErrorHandler.handleException(
                responseObserver, BindStagedAssetResponse.newBuilder(), e,
                "MediaGrpcService", "bindStagedAsset");
        }
    }
}
