package io.github.will413028.hawkr.product.grpc.client;

import io.github.will413028.hawkr.common.obs.grpc.GrpcStubFactory;
import io.github.will413028.hawkr.common.obs.log.ObsLogger;
import io.github.will413028.hawkr.common.utils.code.GrpcResponseCode;
import io.github.will413028.hawkr.common.utils.grpc.GrpcClientErrorTranslator;
import io.github.will413028.hawkr.media.grpc.*;
import io.github.will413028.hawkr.product.exception.ProductBusinessException;
import io.grpc.StatusRuntimeException;
import org.springframework.stereotype.Component;

import java.util.ArrayList;
import java.util.List;
import java.util.concurrent.TimeUnit;
import java.util.stream.Collectors;

/**
 * Media Service gRPC 客戶端
 * 遵循設計書規範：3 次重試，每次間隔 500ms
 */
@Component
public class MediaGrpcClient {

    private static final ObsLogger log = ObsLogger.forClass(MediaGrpcClient.class);

    private static final int MAX_RETRIES = 3;
    private static final long RETRY_DELAY_MS = 500;
    private static final String MEDIA_TARGET = "media";

    /**
     * issue #3332：media-service 呼叫的 gRPC deadline（秒）。
     *
     * <p><b>為什麼非有不可</b>：共用元件 {@code GrpcStubFactory} 全域<b>零</b> deadline，
     * Java {@code ManagedChannelBuilder} 預設無 keepalive ⇒ 下游掛住時 blocking stub 永不返回。
     *
     * <p><b>20 秒不是 10 秒</b>：這四支是<b>寫入</b>路徑——轉正要搬 object storage、
     * link/unlink/release 要寫 DB，工作量與「下游本地 DB 讀取」不是同一個量級。
     * 取 20 秒是為了與<b>同一個下游</b>的既有值一致，不是自造數字：
     * {@code order-service} / {@code aftersales-service} / {@code shipping-service} 的
     * MediaGrpcClient 都是 {@code MEDIA_DEADLINE_SEC = 20L}，理由逐字寫著「同一個下游、同一種工作量」。
     *
     * <p>⚠️ 本 client 外層還有 {@code executeWithRetry}（{@code MAX_RETRIES}=3、間隔 500ms）
     * ⇒ 單一業務方法的最壞耗時 ≈ 3 × 20s ＋ 2 × 500ms ≈ 61 秒。
     *
     * <p>⚠️ {@code withDeadlineAfter} 換算的是<b>絕對</b>時間 ⇒ 必須<b>每次呼叫時</b>套用
     * （每次重試都會重新執行 lambda，因此每次重試也各自拿到新的 20 秒）。
     * 不得挪進 {@link #getStub()} 以外的建構期，也<b>不得</b>改共用的 {@code GrpcStubFactory}。
     */
    public static final long MEDIA_DEADLINE_SEC = 20L;

    private final GrpcStubFactory grpcStubFactory;

    public MediaGrpcClient(GrpcStubFactory grpcStubFactory) {
        this.grpcStubFactory = grpcStubFactory;
    }

    /**
     * 將暫存圖片轉正為正式資產（僅轉正，不建立引用）
     * <p>
     * 呼叫端必須在業務資料落盤後，自行呼叫 {@link #linkImages} 建立引用關係。
     * 正確順序：FinalizeImages → 業務 DB 寫入 → LinkImages
     * <p>
     * 原因：若先 Link 後寫 DB，DB 回滾時 asset_reference 已存在，
     * GC 無法清理該資產（有引用），造成永久孤兒與持續的儲存成本。
     *
     * @param stagedTokens 暫存 token 列表
     * @return 已轉正的資產列表
     */
    public List<FinalizedImage> finalizeImages(List<String> stagedTokens, String refType) {
        if (stagedTokens == null || stagedTokens.isEmpty()) {
            return new ArrayList<>();
        }

        FinalizeImagesRequest request = FinalizeImagesRequest.newBuilder()
                .addAllStagedTokens(stagedTokens)
                .setRefType(refType)
                .build();

        FinalizeImagesResponse response = executeWithRetry(
                "FinalizeImages",
                () -> getStub()
                        .withDeadlineAfter(MEDIA_DEADLINE_SEC, TimeUnit.SECONDS)
                        .finalizeImages(request)
        );

        // media 的業務拒絕是 onNext(code≠0) 而不是 StatusRuntimeException，executeWithRetry
        // 看不到它。原本這裡只讀 assets 清單，於是「一張都沒轉正」與「全部轉正」在呼叫端
        // 完全同形——本方法照樣回傳、log 照樣記 success，圖片靜默消失（56 P0-1：代行上傳的
        // 圖永遠掛不上去，且因為 images 是整份替換，既有圖片一併被清空，而請求回 code: 0）。
        if (response.getCode() != GrpcResponseCode.SUCCESS.code()) {
            throw new ProductBusinessException(finalizeFailureCode(response.getCode()), response.getMessage());
        }
        // 張數不符即失敗：媒體轉正是 all-or-nothing（media 對任一 token 不合格就整批拋），
        // 少一張代表回傳與請求對不上，此時繼續往下組 gallery 只會沉默地少掛一張。
        if (response.getAssetsCount() != stagedTokens.size()) {
            throw new ProductBusinessException(GrpcResponseCode.INTERNAL_ERROR,
                    "圖片轉正結果與請求不符，requested=" + stagedTokens.size()
                            + "，finalized=" + response.getAssetsCount());
        }

        List<FinalizedImage> results = response.getAssetsList().stream()
                .map(asset -> new FinalizedImage(
                        asset.getStagedToken(),
                        asset.getAssetId(),
                        asset.getObjectKey()
                ))
                .collect(Collectors.toList());

        log.info("success", "圖片轉正成功，count=" + results.size());

        return results;
    }

    /**
     * 建立資產與業務實體的引用關係
     */
    public void linkImages(List<String> assetIds, String refType, String refId) {
        if (assetIds == null || assetIds.isEmpty()) {
            return;
        }

        LinkImagesRequest request = LinkImagesRequest.newBuilder()
                .setRefType(refType)
                .setRefId(refId)
                .addAllAssetIds(assetIds)
                .build();

        LinkImagesResponse response = executeWithRetry(
                "LinkImages",
                () -> getStub()
                        .withDeadlineAfter(MEDIA_DEADLINE_SEC, TimeUnit.SECONDS)
                        .linkImages(request)
        );

        log.info("success", "資產引用建立成功，linked_count=" + response.getLinkedCount() + "，ref_type=" + refType, "ref_id", refId);
    }

    /**
     * 解除資產與業務實體的引用關係
     */
    public void unlinkImages(List<String> assetIds, String refType, String refId) {
        if (assetIds == null || assetIds.isEmpty()) {
            return;
        }

        UnlinkImagesRequest request = UnlinkImagesRequest.newBuilder()
                .setRefType(refType)
                .setRefId(refId)
                .addAllAssetIds(assetIds)
                .build();

        UnlinkImagesResponse response = executeWithRetry(
                "UnlinkImages",
                () -> getStub()
                        .withDeadlineAfter(MEDIA_DEADLINE_SEC, TimeUnit.SECONDS)
                        .unlinkImages(request)
        );

        log.info("success", "資產引用解除成功，unlinked_count=" + response.getUnlinkedCount() + "，ref_type=" + refType, "ref_id", refId);
    }

    /**
     * 釋放引用：依 (ref_type, ref_id) 批次移除所有關聯，不需 asset_ids。
     * <p>
     * 用於刪除業務實體時一次釋放該實體的全部媒體引用（如商品軟刪除釋放 PRODUCT / PRODUCT_DESC）。
     * 釋放後資產若無其他引用即成孤兒，由 media-service GC 回收 OSS。
     */
    public void releaseReferences(String refType, List<String> refIds) {
        if (refIds == null || refIds.isEmpty()) {
            return;
        }

        ReleaseReferencesRequest request = ReleaseReferencesRequest.newBuilder()
                .setRefType(refType)
                .addAllRefIds(refIds)
                .build();

        ReleaseReferencesResponse response = executeWithRetry(
                "ReleaseReferences",
                () -> getStub()
                        .withDeadlineAfter(MEDIA_DEADLINE_SEC, TimeUnit.SECONDS)
                        .releaseReferences(request)
        );

        log.info("success", "資產引用釋放成功，released_count=" + response.getReleasedCount() + "，ref_type=" + refType);
    }

    // ==================== 私有輔助方法 ====================

    /**
     * 取得 gRPC stub（使用 GrpcStubFactory，自動注入 trace context）
     */
    private MediaServiceGrpc.MediaServiceBlockingStub getStub() {
        return grpcStubFactory.blocking(MEDIA_TARGET, MediaServiceGrpc::newBlockingStub);
    }

    /**
     * 執行 gRPC 呼叫，包含重試邏輯
     * 3 次重試，每次間隔 500ms
     */
    private <T> T executeWithRetry(String operation, GrpcCall<T> call) {
        int attempt = 0;
        StatusRuntimeException lastException = null;

        while (attempt < MAX_RETRIES) {
            attempt++;
            try {
                return call.execute();
            } catch (StatusRuntimeException e) {
                lastException = e;
                log.warn("retry", "gRPC 呼叫失敗，準備重試 attempt=" + attempt + "/" + MAX_RETRIES + "，" + e.getMessage(), "operation", operation);

                if (attempt < MAX_RETRIES) {
                    try {
                        Thread.sleep(RETRY_DELAY_MS);
                    } catch (InterruptedException ie) {
                        Thread.currentThread().interrupt();
                        throw new ProductBusinessException(GrpcResponseCode.INTERNAL_ERROR,
                                operation + " 重試被中斷");
                    }
                }
            }
        }

        log.error("exhausted", "重試次數已用盡，operation=" + operation + "，max_retries=" + MAX_RETRIES);

        // 重試耗盡 → 轉換為本服務業務異常（依 Status 對應 GrpcResponseCode）
        throw GrpcClientErrorTranslator.translate(lastException, ProductBusinessException::new);
    }

    /**
     * media 的業務碼原樣沿用，讓「這張圖不是你的」與「media 掛了」在前端分得開；
     * 未登錄的碼退回 INTERNAL_ERROR，不猜。
     */
    private GrpcResponseCode finalizeFailureCode(int mediaCode) {
        try {
            return GrpcResponseCode.fromCode(mediaCode);
        } catch (IllegalArgumentException e) {
            return GrpcResponseCode.INTERNAL_ERROR;
        }
    }

    /**
     * 生成冪等鍵
     */
    private String generateIdempotencyKey(String operation, String refType, String refId) {
        return String.format("%s:%s:%s:%d", operation, refType, refId, System.currentTimeMillis());
    }

    /**
     * gRPC 呼叫函數介面
     */
    @FunctionalInterface
    private interface GrpcCall<T> {
        T execute();
    }

    /**
     * 已轉正的圖片資訊
     */
    public static class FinalizedImage {
        private final String stagedToken;
        private final String assetId;
        private final String objectKey;

        public FinalizedImage(String stagedToken, String assetId, String objectKey) {
            this.stagedToken = stagedToken;
            this.assetId = assetId;
            this.objectKey = objectKey;
        }

        public String getStagedToken() {
            return stagedToken;
        }

        public String getAssetId() {
            return assetId;
        }

        public String getObjectKey() {
            return objectKey;
        }
    }
}
