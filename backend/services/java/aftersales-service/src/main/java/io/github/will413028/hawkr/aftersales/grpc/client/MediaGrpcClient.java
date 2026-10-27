package io.github.will413028.hawkr.aftersales.grpc.client;

import io.github.will413028.hawkr.common.obs.grpc.GrpcStubFactory;
import io.github.will413028.hawkr.common.obs.log.ObsLogger;
import io.github.will413028.hawkr.common.utils.code.AssetReferenceType;
import io.github.will413028.hawkr.media.grpc.*;
import org.springframework.stereotype.Component;

import java.util.List;
import java.util.concurrent.TimeUnit;

@Component
public class MediaGrpcClient {

    private static final ObsLogger log = ObsLogger.forClass(MediaGrpcClient.class);

    /**
     * backend#3262：media RPC 的 gRPC deadline（秒）。共用元件 {@code GrpcStubFactory}
     * 全域<b>零 deadline</b>（{@code GrpcChannelRegistry.buildChannel} 只有 forAddress /
     * intercept / usePlaintext，無 keepAlive、無 idleTimeout）⇒ 下游掛住時 blocking stub 永不返回。
     *
     * <p><b>為什麼這一支特別要有</b>：兩個方法都被
     * {@link io.github.will413028.hawkr.aftersales.service.EvidenceService#finalizeAndInsert} 呼叫，
     * 而那支跑在 {@code submitAppeal} 的 {@code @Transactional} 內，且位於
     * 「申訴資格已確立（A3 通過、case row 已持鎖）→ commit」之間。無 deadline ⇒
     * <b>該 case 的 row lock 持有時間無上界</b> ⇒ 撥款會被無限期推遲（每次重問都探到鎖）。
     *
     * <p>⚠️ 資金安全<b>不</b>靠這個值：那是 {@code AfterSalePayoutHoldService} 的持鎖順序保證
     * （fail-closed）。本 deadline 只把「撥款被推遲多久」變成有界，是 defense-in-depth，
     * 不是 correctness 的依據——把它調大不會產生資損，只會讓賣家多等。
     *
     * <p>取 20 秒，與 {@code shipping-service} 的
     * {@code MediaGrpcClient.MEDIA_DEADLINE_SEC} 同值（同一個下游、同一種工作量）。
     */
    public static final long MEDIA_DEADLINE_SEC = 20L;

    private final GrpcStubFactory grpcStubFactory;

    public MediaGrpcClient(GrpcStubFactory grpcStubFactory) {
        this.grpcStubFactory = grpcStubFactory;
    }

    /**
     * ⚠️ deadline 必須在<b>每次呼叫時</b>套用而不是建構子：{@code withDeadlineAfter} 換算的是
     * <b>絕對</b>時間，存進欄位的話那個 deadline 會在第一次呼叫後就永久過期。
     */
    private MediaServiceGrpc.MediaServiceBlockingStub stub() {
        return grpcStubFactory.blocking("media", MediaServiceGrpc::newBlockingStub)
                .withDeadlineAfter(MEDIA_DEADLINE_SEC, TimeUnit.SECONDS);
    }

    public FinalizeImagesResponse finalizeImages(List<String> stagedTokens) {
        return finalizeImages(stagedTokens, AssetReferenceType.AFTERSALE.value());
    }

    public FinalizeImagesResponse finalizeImages(List<String> stagedTokens, String refType) {
        log.info("MediaGrpcClient.finalizeImages.start", "圖片轉正, count=" + stagedTokens.size() + ", refType=" + refType);
        var resp = stub().finalizeImages(
                FinalizeImagesRequest.newBuilder()
                        .addAllStagedTokens(stagedTokens)
                        .setRefType(refType)
                        .build()
        );
        log.info("MediaGrpcClient.finalizeImages.done", "圖片轉正完成, code=" + resp.getCode());
        return resp;
    }

    public LinkImagesResponse linkImages(String refType, String refId, List<Long> assetIds) {
        log.info("MediaGrpcClient.linkImages.start", "連結圖片, refType=" + refType, "refId", refId);
        // LinkImagesRequest.asset_ids 為 repeated string（Proto 數字欄位一律 string）
        List<String> assetIdStrings = assetIds.stream()
                .map(String::valueOf)
                .toList();
        var resp = stub().linkImages(
                LinkImagesRequest.newBuilder()
                        .setRefType(refType)
                        .setRefId(refId)
                        .addAllAssetIds(assetIdStrings)
                        .build()
        );
        log.info("MediaGrpcClient.linkImages.done", "連結圖片完成, code=" + resp.getCode(), "refId", refId);
        return resp;
    }
}
