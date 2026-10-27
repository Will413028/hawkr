package io.github.will413028.hawkr.order.grpc.client;

import io.github.will413028.hawkr.common.obs.grpc.GrpcStubFactory;
import io.github.will413028.hawkr.common.obs.log.ObsLogger;
import io.github.will413028.hawkr.media.grpc.*;
import org.springframework.stereotype.Component;

import java.util.ArrayList;
import java.util.List;
import java.util.concurrent.TimeUnit;

/**
 * Media Service gRPC Client
 *
 * 提供買家確認收貨上傳照片所需的兩個操作：
 * - finalizeImages：將前端 stagedToken 轉正為正式 asset
 * - linkImages：建立 ORDER_RECEIPT 類型的引用追蹤
 *
 * 內部呼叫，x-internal-service 自動注入
 */
@Component
public class MediaGrpcClient {

    private static final ObsLogger log = ObsLogger.forClass(MediaGrpcClient.class);

    /**
     * media-service 呼叫的 deadline。
     *
     * <p><b>20 秒不是 10 秒</b>：這兩支（{@code finalizeImages} / {@code linkImages}）是
     * <b>寫入</b>路徑——圖片轉正要搬 object storage、連結要寫 DB，工作量與「下游本地 DB 讀取」
     * 不是同一個量級。沿用 {@code ProductGrpcClient.INTERNAL_READ_DEADLINE_SEC}（10 秒，名字
     * 裡的 READ 就是它的適用範圍）會把一個原本慢但會成功的請求變成 DEADLINE_EXCEEDED。
     *
     * <p>取 20 秒是為了與**同一個下游**的既有值一致，不是自造數字：
     * {@code aftersales-service} 與 {@code shipping-service} 的 MediaGrpcClient 都是
     * {@code MEDIA_DEADLINE_SEC = 20L}，理由逐字寫著「同一個下游、同一種工作量」。
     *
     * <p>⚠️ 本值在 #3230 之前是<b>無上界</b>（完全沒有 deadline）⇒ 加上它引入了一個新的
     * 失敗模式：超過 20 秒的轉正會從「慢但成功」變成拋出。這是 CLAUDE.md §12 要求的取捨，
     * 但值得在 dev 用多張大圖實測一次 p99。
     */
    public static final long MEDIA_DEADLINE_SEC = 20L;

    private final GrpcStubFactory grpcStubFactory;

    public MediaGrpcClient(GrpcStubFactory grpcStubFactory) {
        this.grpcStubFactory = grpcStubFactory;
    }

    /**
     * 將前端 stagedToken 轉正為正式 asset。
     * 失敗時拋 RuntimeException，呼叫端需轉成 INTERNAL gRPC 錯誤回給買家。
     */
    public List<FinalizedAsset> finalizeImages(List<String> stagedTokens, String refType) {
        if (stagedTokens == null || stagedTokens.isEmpty()) {
            return new ArrayList<>();
        }
        FinalizeImagesRequest request = FinalizeImagesRequest.newBuilder()
                .addAllStagedTokens(stagedTokens)
                .setRefType(refType)
                .build();
        // media-service 失敗時會拋 StatusRuntimeException，由上層 RPC catch-all 處理
        FinalizeImagesResponse response = getStub()
                .withDeadlineAfter(MEDIA_DEADLINE_SEC, TimeUnit.SECONDS)
                .finalizeImages(request);
        log.info("MediaGrpcClient.finalizeImages.success",
                "圖片轉正成功", "count", String.valueOf(response.getAssetsCount()));
        return response.getAssetsList();
    }

    /**
     * 綁定引用（buyer receipt photo 用）。失敗會拋出，呼叫端**必須**中止並在寫入業務列之前呼叫
     * ——順序契約見 08 號設計書 §4.4 / backend#2222。
     */
    public void linkImages(String refType, String refId, List<String> assetIds) {
        if (assetIds == null || assetIds.isEmpty()) {
            return;
        }
        LinkImagesRequest request = LinkImagesRequest.newBuilder()
                .setRefType(refType)
                .setRefId(refId)
                .addAllAssetIds(assetIds)
                .build();
        LinkImagesResponse response = getStub()
                .withDeadlineAfter(MEDIA_DEADLINE_SEC, TimeUnit.SECONDS)
                .linkImages(request);
        log.info("MediaGrpcClient.linkImages.success",
                "資產引用建立成功，linkedCount=" + response.getLinkedCount(), "refId", refId);
    }

    private MediaServiceGrpc.MediaServiceBlockingStub getStub() {
        return grpcStubFactory.blocking("media", MediaServiceGrpc::newBlockingStub);
    }
}
