package io.github.will413028.hawkr.order.checkout.client;

import io.github.will413028.hawkr.common.grpc.PrincipalResolution;
import io.github.will413028.hawkr.common.obs.grpc.GrpcStubFactory;
import io.github.will413028.hawkr.common.obs.log.ObsLogger;
import io.github.will413028.hawkr.common.utils.code.GrpcResponseCode;
import io.github.will413028.hawkr.order.exception.OrderBusinessException;
import io.github.will413028.hawkr.seller.grpc.BatchGetSellerMetaRequest;
import io.github.will413028.hawkr.seller.grpc.BatchGetSellerMetaResponse;
import io.github.will413028.hawkr.seller.grpc.ResolvePrincipalsRequest;
import io.github.will413028.hawkr.seller.grpc.ResolvePrincipalsResponse;
import io.github.will413028.hawkr.seller.grpc.SellerMeta;
import io.github.will413028.hawkr.seller.grpc.SellerServiceGrpc;
import io.grpc.Status;
import org.springframework.stereotype.Component;

import java.util.List;
import java.util.Map;
import java.util.concurrent.TimeUnit;

/**
 * Seller Service gRPC Client
 *
 * 提供：
 * - getSellerMeta：單一賣家查詢（結帳用，硬依賴）
 * - batchGetSellerMeta：批量查詢（訂單列表/詳情聚合用）
 *
 * 內部呼叫，x-internal-service 自動注入
 */
@Component
public class SellerGrpcClient {

    private static final ObsLogger log = ObsLogger.forClass(SellerGrpcClient.class);

    /**
     * issue #2294：內部讀取型 RPC 的 gRPC deadline（秒）。
     *
     * <p><b>為什麼非有不可</b>：{@code GrpcStubFactory} / {@code GrpcChannelRegistry}
     * （observability-common）全域<b>零</b> deadline 設定 ⇒ 下游若是「掛住」而非「失敗」，
     * blocking stub <b>永不返回</b>。本 client 被 {@code @Scheduled} worker 使用，而 Spring Boot
     * 的 {@code spring.task.scheduling.pool.size} 預設是 <b>1</b> ⇒ 一次掛住就讓該服務的
     * <b>所有</b>排程停擺且永不恢復；同時使排程搶佔的租約下界
     * （《多副本排程搶佔規範》§三第 5 步「租約必須 &gt; 單筆工作的最壞耗時」）無從建立。
     *
     * <p><b>10 秒怎麼來的</b>：這幾支都是下游服務的<b>本地 DB 讀取</b>（無第三方、無跨服務串接），
     * p99 為毫秒級；10 秒是兩個數量級以上的餘裕，只用來把「永遠不回」轉成「10 秒後失敗」。
     *
     * <p>deadline 加在<b>呼叫點</b>而非 {@code GrpcStubFactory}：後者是 observability-common
     * 的共用元件，動它會一次改掉全 repo 所有 Java gRPC 呼叫的逾時語意。
     * ⚠️ {@code withDeadlineAfter} 換算的是<b>絕對</b>時間，因此必須<b>每次呼叫時</b>套用；
     * 挪進建構子 ＝ 啟動 10 秒後全部逾時。
     */
    public static final long INTERNAL_READ_DEADLINE_SEC = 10L;

    private final SellerServiceGrpc.SellerServiceBlockingStub sellerStub;

    public SellerGrpcClient(GrpcStubFactory grpcStubFactory) {
        this.sellerStub = grpcStubFactory.blocking("seller", SellerServiceGrpc::newBlockingStub);
    }

    /**
     * 取得單一賣家元資料（硬依賴，失敗直接拋出例外）
     *
     * @param storeId 賣家 store_id（店家代理鍵，非登入 account_id）
     * @return SellerMeta（含 zip_code, payuni_mer_id）
     * @throws RuntimeException Seller Service 不可用或查無結果
     */
    public SellerMeta getSellerMeta(long storeId) {
        log.info("SellerGrpcClient.getSellerMeta.start", "取得賣家元資料",
                "storeId", String.valueOf(storeId));

        BatchGetSellerMetaRequest request = BatchGetSellerMetaRequest.newBuilder()
                .addAllStoreIds(List.of(storeId))
                .build();

        // seller-service 失敗時會拋 StatusRuntimeException，由上層 RPC catch-all 處理
        BatchGetSellerMetaResponse response = sellerStub
                .withDeadlineAfter(INTERNAL_READ_DEADLINE_SEC, TimeUnit.SECONDS)
                .batchGetSellerMeta(request);

        SellerMeta meta = response.getSellersMap().get(storeId);
        if (meta == null) {
            throw new OrderBusinessException(GrpcResponseCode.NOT_FOUND,
                    "查無賣家元資料，storeId=" + storeId);
        }

        log.info("SellerGrpcClient.getSellerMeta.done", "取得賣家元資料完成",
                "storeId", String.valueOf(storeId));
        return meta;
    }

    /**
     * 批量查詢賣家資訊（店名、頭像、城市、地區）
     *
     * @param storeIds 賣家 store_id 列表（店家代理鍵，非登入 account_id）
     * @return Map<storeId, SellerMeta>，查詢失敗時回傳空 Map
     */
    public Map<Long, SellerMeta> batchGetSellerMeta(List<Long> storeIds) {
        log.info("SellerGrpcClient.batchGetSellerMeta.start",
                "批量查詢賣家資訊", "count", String.valueOf(storeIds.size()));
        BatchGetSellerMetaRequest request = BatchGetSellerMetaRequest.newBuilder()
                .addAllStoreIds(storeIds)
                .build();
        BatchGetSellerMetaResponse response = sellerStub
                .withDeadlineAfter(INTERNAL_READ_DEADLINE_SEC, TimeUnit.SECONDS)
                .batchGetSellerMeta(request);
        log.info("SellerGrpcClient.batchGetSellerMeta.done",
                "賣家資訊回應 code=" + response.getCode(), "resultCount", String.valueOf(response.getSellersCount()));
        return response.getSellersMap();
    }

    /**
     * 解析 store_id → 登入 principal account_id（Plan 2b 自買防護用）。
     * 硬依賴（與本類別 getSellerMeta 相同、與 batchGetSellerMeta 的降級策略不同）：
     * Seller Service 失敗時拋出例外，由呼叫端 fail-closed 處理 —
     * 自買防護是防詐骨科，不能在 Seller Service 故障時悄悄退化成錯誤比對。
     *
     * <p>回傳 {@link PrincipalResolution} 而非裸 map：查無在架賣家的 store_id 會被 server
     * 原樣回吐成合法非零值，只拿 map 的呼叫端分不出它是不是真的解析結果（issue #2789）。
     * 閘門類（自買防護）必須用 require 系列 fail-closed；通知類讀 {@code principals()}
     * 即維持既有語意。</p>
     *
     * @param storeIds 待解析的 id 列表
     * @return principals（每個輸入 id 都有 entry）＋ unresolvedIds（其值是未驗證的 echo-back）
     */
    public PrincipalResolution resolvePrincipals(List<Long> storeIds) {
        if (storeIds.isEmpty()) {
            return PrincipalResolution.empty();
        }
        ResolvePrincipalsRequest request = ResolvePrincipalsRequest.newBuilder()
                .addAllIds(storeIds)
                .build();
        // seller-service 失敗時會拋 StatusRuntimeException，由呼叫端決定 fail-closed 處理
        ResolvePrincipalsResponse response = sellerStub
                .withDeadlineAfter(INTERNAL_READ_DEADLINE_SEC, TimeUnit.SECONDS)
                .resolvePrincipals(request);
        if (response.getCode() != 0) {
            // 非 0 code（業務碼錯誤，非 transport 失敗）同樣視為失敗、fail-closed，
            // 避免呼叫端拿到空 map 後 getOrDefault 悄悄退化成錯誤比對（final review follow-up）
            throw Status.INTERNAL.withDescription(response.getMessage()).asRuntimeException();
        }
        return PrincipalResolution.of(response.getPrincipalsMap(), response.getUnresolvedIdsList());
    }
}
