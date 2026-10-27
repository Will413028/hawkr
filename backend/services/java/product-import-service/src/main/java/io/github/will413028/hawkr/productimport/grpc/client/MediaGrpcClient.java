package io.github.will413028.hawkr.productimport.grpc.client;

import io.github.will413028.hawkr.common.obs.grpc.GrpcStubFactory;
import io.github.will413028.hawkr.common.obs.log.ObsLogger;
import io.github.will413028.hawkr.media.grpc.BindStagedAssetRequest;
import io.github.will413028.hawkr.media.grpc.BindStagedAssetResponse;
import io.github.will413028.hawkr.media.grpc.ImportDownloadAssetRequest;
import io.github.will413028.hawkr.media.grpc.ImportDownloadAssetResponse;
import io.github.will413028.hawkr.media.grpc.MediaServiceGrpc;
import io.grpc.StatusRuntimeException;
import org.springframework.stereotype.Component;

import java.util.concurrent.TimeUnit;

@Component
public class MediaGrpcClient {

    private static final ObsLogger log = ObsLogger.forClass(MediaGrpcClient.class);
    private static final String TARGET = "media";

    /**
     * issue #3332：media-service 呼叫的 gRPC deadline（秒）。
     *
     * <p><b>為什麼非有不可</b>：共用元件 {@code GrpcStubFactory} 全域<b>零</b> deadline，
     * Java {@code ManagedChannelBuilder} 預設無 keepalive ⇒ 下游掛住時 blocking stub 永不返回。
     * 本 client 由匯入 pipeline（背景 worker）呼叫，沒有上界等於把該條 worker 永久吃掉。
     *
     * <p><b>20 秒是為了與同一個下游的既有值一致</b>，不是自造數字：{@code order-service} /
     * {@code aftersales-service} / {@code shipping-service} / {@code product-service} 的
     * MediaGrpcClient 都是 {@code MEDIA_DEADLINE_SEC = 20L}，理由逐字寫著「同一個下游、同一種工作量」。
     *
     * <p>實查兩支的 media 側都<b>不是</b>同步下載：{@code ImportDownloadAsset} 只做
     * INSERT {@code import_asset} ＋ afterCommit publish 內部派工事件（真正的下載在 worker），
     * {@code BindStagedAsset} 只綁既有 staged 資產。理論上 10 秒也夠——取 20 秒是因為
     * <b>同一個下游</b>的一致性比逐支微調更重要，且過長的代價遠小於過短。
     *
     * <p>⚠️ {@code withDeadlineAfter} 換算<b>絕對</b>時間 ⇒ 必須<b>每次呼叫時</b>套用；
     * 也<b>不得</b>改共用的 {@code GrpcStubFactory}（會一次改掉全 repo 的逾時語意）。
     */
    public static final long MEDIA_DEADLINE_SEC = 20L;

    private final GrpcStubFactory grpcStubFactory;

    public MediaGrpcClient(GrpcStubFactory grpcStubFactory) {
        this.grpcStubFactory = grpcStubFactory;
    }

    private MediaServiceGrpc.MediaServiceBlockingStub stub() {
        return grpcStubFactory.blocking(TARGET, MediaServiceGrpc::newBlockingStub);
    }

    public ImportDownloadAssetResponse importDownloadAsset(String refType,
                                                           String sourceUrl,
                                                           long storeId,
                                                           long importJobId,
                                                           long importItemId,
                                                           String idempotencyKey,
                                                           String expiresAt,
                                                           long callbackCorrelationId) {
        ImportDownloadAssetRequest req = ImportDownloadAssetRequest.newBuilder()
                .setRefType(refType)
                .setSourceUrl(sourceUrl)
                .setStoreId(storeId)
                .setImportJobId(importJobId)
                .setImportItemId(importItemId)
                .setIdempotencyKey(idempotencyKey)
                .setExpiresAt(expiresAt)
                .setCallbackCorrelationId(callbackCorrelationId)
                .build();
        try {
            return stub()
                    .withDeadlineAfter(MEDIA_DEADLINE_SEC, TimeUnit.SECONDS)
                    .importDownloadAsset(req);
        } catch (StatusRuntimeException e) {
            log.error("MediaGrpcClient.importDownloadAsset.error",
                    "ImportDownloadAsset failed: " + e.getStatus(), e);
            throw e;
        }
    }

    public BindStagedAssetResponse bindStagedAsset(String stagedToken,
                                                   long storeId,
                                                   long importJobId,
                                                   long importItemId,
                                                   String refType) {
        BindStagedAssetRequest req = BindStagedAssetRequest.newBuilder()
                .setStagedToken(stagedToken)
                .setStoreId(storeId)
                .setImportJobId(importJobId)
                .setImportItemId(importItemId)
                .setRefType(refType)
                .build();
        try {
            return stub()
                    .withDeadlineAfter(MEDIA_DEADLINE_SEC, TimeUnit.SECONDS)
                    .bindStagedAsset(req);
        } catch (StatusRuntimeException e) {
            log.error("MediaGrpcClient.bindStagedAsset.error",
                    "BindStagedAsset failed: " + e.getStatus(), e);
            throw e;
        }
    }
}
