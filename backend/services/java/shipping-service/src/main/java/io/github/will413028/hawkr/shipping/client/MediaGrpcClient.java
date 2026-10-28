package io.github.will413028.hawkr.shipping.client;

import io.github.will413028.hawkr.common.obs.grpc.GrpcStubFactory;
import io.github.will413028.hawkr.common.obs.log.ObsLogger;
import io.github.will413028.hawkr.common.utils.code.GrpcResponseCode;
import io.github.will413028.hawkr.common.utils.grpc.GrpcClientErrorTranslator;
import io.github.will413028.hawkr.media.grpc.*;
import io.github.will413028.hawkr.shipping.exception.ShippingBusinessException;
import io.grpc.StatusRuntimeException;
import org.springframework.stereotype.Component;

import java.util.ArrayList;
import java.util.List;
import java.util.concurrent.TimeUnit;
import java.util.stream.Collectors;

@Component
public class MediaGrpcClient {

    private static final ObsLogger log = ObsLogger.forClass(MediaGrpcClient.class);
    private static final int MAX_RETRIES = 3;
    private static final long RETRY_DELAY_MS = 500;

    private final GrpcStubFactory grpcStubFactory;

    public MediaGrpcClient(GrpcStubFactory grpcStubFactory) {
        this.grpcStubFactory = grpcStubFactory;
    }

    /**
     * 後端直傳上傳 + 轉正（宅配 PDF 標籤用）
     */
    public ServerUploadAndFinalizeResponse serverUploadAndFinalize(
            byte[] fileData, String filename, String contentType, String refType) {
        var request = ServerUploadAndFinalizeRequest.newBuilder()
                .setFileData(com.google.protobuf.ByteString.copyFrom(fileData))
                .setFilename(filename)
                .setContentType(contentType)
                .setRefType(refType)
                .build();

        ServerUploadAndFinalizeResponse response = executeWithRetry("ServerUploadAndFinalize",
                () -> getStub().serverUploadAndFinalize(request));

        log.info("MediaGrpcClient.serverUploadAndFinalize.success",
                "PDF 上傳成功，assetId=" + response.getAssetId() + "，objectKey=" + response.getObjectKey());
        return response;
    }

    /**
     * 機敏檔後端直傳 + 轉正（{@code secure/} 前綴）。
     *
     * <h2>🔴 為什麼不能沿用 {@link #serverUploadAndFinalize}</h2>
     * 那一支硬寫 {@code public/} 前綴 ⇒ 任何拿到 objectKey 的人都取得到。
     * 而 Uber 的送達證明是<b>收件地址與門口實景</b>（66 陷阱 A6），用 public 上傳
     * <b>沒有任何訊號</b>：上傳成功、圖片看得到、一切正常，只是全世界都看得到。
     * ⇒ 唯一的防線是「斷言 objectKey 以 {@code secure/} 開頭」。
     *
     * <p>先例：invoice-service 的發票 PDF 走同一支 RPC
     * （{@code AssetReferenceType.INVOICE_DOCUMENT} 的 description 逐字記載）。
     *
     * @param refType {@code AssetReferenceType.value()}。<b>必填</b>——media 端以它決定 secure 子前綴
     * @param refId   業務主鍵。<b>必填</b>——它同時是日後 {@code GetSecureAssetInternal} 的授權述詞，
     *                填錯的症狀是「資產存在但取不回來」
     * @return 回應內的 {@code objectKey} 是 {@code secure/} 前綴；🔴 <b>只存 objectKey</b>，
     *         禁止存完整 URL 或 presigned URL（10 檔 §25）
     */
    public ServerUploadSecureAndFinalizeResponse serverUploadSecureAndFinalize(
            byte[] fileData, String filename, String contentType, String refType, String refId) {
        var request = ServerUploadSecureAndFinalizeRequest.newBuilder()
                .setFileData(com.google.protobuf.ByteString.copyFrom(fileData))
                .setFilename(filename)
                .setContentType(contentType)
                .setRefType(refType)
                .setRefId(refId)
                .build();

        ServerUploadSecureAndFinalizeResponse response = executeWithRetry("ServerUploadSecureAndFinalize",
                () -> getStub().serverUploadSecureAndFinalize(request));

        // ⚠️ objectKey 本身是**憑證等級**的值（：拿到它就取得到那張圖）
        //    ⇒ 這裡只記 assetId 與長度，不記 objectKey 本身。
        //    （對照上面那支 public 的：它記了 objectKey，而 public 前綴本來就沒有秘密。）
        log.info("MediaGrpcClient.serverUploadSecureAndFinalize.success",
                "機敏檔上傳成功，assetId=" + response.getAssetId(), "refId", refId);
        return response;
    }

    /**
     * 前端 stagedToken 轉正（出貨照片用）
     */
    public List<FinalizedAsset> finalizeImages(List<String> stagedTokens, String refType) {
        if (stagedTokens == null || stagedTokens.isEmpty()) {
            return new ArrayList<>();
        }

        var request = FinalizeImagesRequest.newBuilder()
                .addAllStagedTokens(stagedTokens)
                .setRefType(refType)
                .build();

        FinalizeImagesResponse response = executeWithRetry("FinalizeImages",
                () -> getStub().finalizeImages(request));

        log.info("MediaGrpcClient.finalizeImages.success",
                "圖片轉正成功", "count", String.valueOf(response.getAssetsCount()));
        return response.getAssetsList();
    }

    /**
     * 綁定引用（PDF 標籤 + 出貨照片共用）
     */
    public void linkImages(String refType, String refId, List<String> assetIds) {
        if (assetIds == null || assetIds.isEmpty()) {
            return;
        }

        var request = LinkImagesRequest.newBuilder()
                .setRefType(refType)
                .setRefId(refId)
                .addAllAssetIds(assetIds)
                .build();

        LinkImagesResponse response = executeWithRetry("LinkImages",
                () -> getStub().linkImages(request));

        log.info("MediaGrpcClient.linkImages.success",
                "資產引用建立成功，linkedCount=" + response.getLinkedCount(), "refId", refId);
    }

    /**
     * 解除引用（配送完成後清理 PDF）
     */
    public void unlinkImages(String refType, String refId, List<String> assetIds) {
        if (assetIds == null || assetIds.isEmpty()) {
            return;
        }

        var request = UnlinkImagesRequest.newBuilder()
                .setRefType(refType)
                .setRefId(refId)
                .addAllAssetIds(assetIds)
                .build();

        UnlinkImagesResponse response = executeWithRetry("UnlinkImages",
                () -> getStub().unlinkImages(request));

        log.info("MediaGrpcClient.unlinkImages.success",
                "資產引用解除成功，unlinkedCount=" + response.getUnlinkedCount(), "refId", refId);
    }

    /**
     * issue #2294：媒體 RPC 的 gRPC deadline（秒）。理由同
     * {@link BuyerGrpcClient#INTERNAL_READ_DEADLINE_SEC}（共用元件 {@code GrpcStubFactory}
     * 全域零 deadline ⇒ 下游掛住時 blocking stub 永不返回）。
     *
     * <p>{@code unlinkImages} 位於 §5.19 貨態補收排程的<b>租約區間內</b>
     * （{@code ShippingService.updateStatus} 於 PICKED_UP 時解除 PDF 引用），
     * 無 deadline 時「單筆最壞耗時 &lt; 租約」這條不變式無從建立。
     *
     * <p>取 20 秒（比純 DB 讀的 10 秒寬）：{@code serverUploadAndFinalize} 會夾帶 PDF 位元組，
     * 走的是上傳而非查詢。最壞耗時 = {@code MAX_RETRIES}(3) × 20s ＋ 2 × {@code RETRY_DELAY_MS}
     * ≈ 61 秒，仍遠小於本服務所有排程的租約（最短 5 分鐘）。
     */
    public static final long MEDIA_DEADLINE_SEC = 20L;

    /**
     * ⚠️ deadline 在<b>這裡</b>套用而不是建構子：{@code withDeadlineAfter} 換算的是<b>絕對</b>
     * 時間，而本方法在每次呼叫（含每次重試）時都會被重新執行 ⇒ 每次都拿到新的相對逾時。
     */
    private MediaServiceGrpc.MediaServiceBlockingStub getStub() {
        return grpcStubFactory.blocking("media", MediaServiceGrpc::newBlockingStub)
                .withDeadlineAfter(MEDIA_DEADLINE_SEC, TimeUnit.SECONDS);
    }

    private <T> T executeWithRetry(String operation, GrpcCall<T> call) {
        int attempt = 0;
        StatusRuntimeException lastException = null;

        while (attempt < MAX_RETRIES) {
            attempt++;
            try {
                return call.execute();
            } catch (StatusRuntimeException e) {
                lastException = e;
                log.warn("MediaGrpcClient.retry",
                        "gRPC 呼叫失敗，準備重試 " + operation + " attempt=" + attempt + "/" + MAX_RETRIES);
                if (attempt < MAX_RETRIES) {
                    try {
                        Thread.sleep(RETRY_DELAY_MS);
                    } catch (InterruptedException ie) {
                        Thread.currentThread().interrupt();
                        throw new ShippingBusinessException(GrpcResponseCode.INTERNAL_ERROR,
                                operation + " 重試被中斷");
                    }
                }
            }
        }

        // 重試耗盡 → 轉換為本服務業務異常（依 Status 對應 GrpcResponseCode）
        throw GrpcClientErrorTranslator.translate(lastException, ShippingBusinessException::new);
    }

    @FunctionalInterface
    private interface GrpcCall<T> {
        T execute();
    }
}
