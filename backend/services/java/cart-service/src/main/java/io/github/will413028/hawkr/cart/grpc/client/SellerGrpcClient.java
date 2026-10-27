package io.github.will413028.hawkr.cart.grpc.client;

import io.github.will413028.hawkr.common.grpc.PrincipalResolution;
import io.github.will413028.hawkr.common.obs.grpc.GrpcStubFactory;
import io.github.will413028.hawkr.common.obs.log.ObsLogger;
import io.github.will413028.hawkr.common.utils.code.GrpcResponseCode;
import io.github.will413028.hawkr.seller.grpc.BatchGetSellerMetaRequest;
import io.github.will413028.hawkr.seller.grpc.BatchGetSellerMetaResponse;
import io.github.will413028.hawkr.seller.grpc.ResolvePrincipalsRequest;
import io.github.will413028.hawkr.seller.grpc.ResolvePrincipalsResponse;
import io.github.will413028.hawkr.seller.grpc.SellerServiceGrpc;
import io.github.will413028.hawkr.seller.grpc.SellerMeta;
import io.grpc.Status;
import org.springframework.stereotype.Component;

import java.util.Collections;
import java.util.List;
import java.util.Map;
import java.util.concurrent.TimeUnit;

/**
 * Seller Service gRPC Client（設計書 §4.2）
 *
 * 僅在 GetCart 時呼叫 BatchGetSellerMeta，取得賣家資訊（可降級）
 */
@Component
public class SellerGrpcClient {

    private static final ObsLogger log = ObsLogger.forClass(SellerGrpcClient.class);

    /** 一次重試。對側硬掛時第二次會即刻 fail（無額外等待），DEADLINE_EXCEEDED 時才真的多花一個 deadline。 */
    private static final int MAX_ATTEMPTS = 2;
    /** 對齊 buyer-service seller client 的 3s 預算，兩次合計仍在使用者可忍受範圍。 */
    private static final long CALL_DEADLINE_SECONDS = 2L;

    private final GrpcStubFactory grpcStubFactory;

    public SellerGrpcClient(GrpcStubFactory grpcStubFactory) {
        this.grpcStubFactory = grpcStubFactory;
    }

    /**
     * 批次取得賣家資訊
     *
     * <p>降級（回空 Map）是最後手段，不是第一反應：賣家 meta 裡的 store_code 是店面連結的
     * canonical key，缺了它前端會渲染出指回目前頁面的死連結（#2098）。因此這裡先把兩個
     * 「安靜地失敗」補起來再談降級：
     * <ol>
     *   <li>非 0 業務碼原本被無視（{@code getSellersMap()} 對失敗回應就是空 Map），
     *       與成功但沒有任何賣家完全無法區分 —— 比照本類別 resolvePrincipals 視為失敗；</li>
     *   <li>單次 transport 失敗即降級，連一次重試都沒有。</li>
     * </ol>
     * 另外補上 deadline：channel 本身沒有預設 deadline（GrpcChannelRegistry），
     * 原本這個呼叫在對側 hang 住時是無上限等待。加上 deadline 後最壞情況反而從
     * 「無上限」收斂成 {@code CALL_DEADLINE_SECONDS × MAX_ATTEMPTS}。
     *
     * @param storeIds 賣家分組 key（store 代理鍵）列表
     * @return Map<storeId, SellerMeta>，重試後仍失敗才回傳空 Map（降級）
     */
    public Map<Long, SellerMeta> batchGetSellerMeta(List<Long> storeIds) {
        if (storeIds == null || storeIds.isEmpty()) {
            return Collections.emptyMap();
        }

        var request = BatchGetSellerMetaRequest.newBuilder()
                .addAllStoreIds(storeIds)
                .build();

        Exception lastFailure = null;
        for (int attempt = 1; attempt <= MAX_ATTEMPTS; attempt++) {
            try {
                var stub = grpcStubFactory.blocking("seller", SellerServiceGrpc::newBlockingStub)
                        .withDeadlineAfter(CALL_DEADLINE_SECONDS, TimeUnit.SECONDS);

                log.info("SellerGrpcClient.batchGetSellerMeta.call",
                        "呼叫 Seller Service attempt=" + attempt, "count", String.valueOf(storeIds.size()));

                BatchGetSellerMetaResponse response = stub.batchGetSellerMeta(request);
                if (response.getCode() != GrpcResponseCode.SUCCESS.code()) {
                    // 對側把業務錯誤放在 code 欄位、gRPC status 仍是 OK，直接讀 sellersMap
                    // 會拿到空 Map 並當成「這些賣家都不存在」。
                    throw Status.INTERNAL
                            .withDescription("BatchGetSellerMeta code=" + response.getCode()
                                    + " message=" + response.getMessage())
                            .asRuntimeException();
                }
                return response.getSellersMap();
            } catch (Exception e) {
                lastFailure = e;
                log.error("SellerGrpcClient.batchGetSellerMeta.attemptFail",
                        "Seller Service 呼叫失敗 attempt=" + attempt + "/" + MAX_ATTEMPTS, e);
            }
        }

        // 降級：Seller Service 失敗不影響 GetCart（購物車仍可結帳），但賣家欄位會缺值，
        // 必須留下可查的訊號，不然它會退化成「靜默吐空字串」。
        log.error("SellerGrpcClient.batchGetSellerMeta.degraded",
                "重試後仍失敗，降級為空；store_code 等賣家欄位將缺值 storeIds=" + storeIds, lastFailure);
        return Collections.emptyMap();
    }

    /**
     * 解析 store_id → 登入 principal account_id（Plan 2b 自買防護用）。
     * 硬依賴（與本類別 batchGetSellerMeta 的降級策略不同）：自買防護是防詐骨科，
     * Seller Service 故障時必須 fail-closed，不能悄悄退化成錯誤比對。
     *
     * <p>回傳 {@link PrincipalResolution} 而非裸 map：查無在架賣家的 store_id 會被
     * server 原樣回吐成合法非零值，只拿 map 的呼叫端分不出它是不是真的解析結果
     * （issue #2789）。自買防護必須看得到這個差別，否則「無法確認」會被當成「不是自己」。</p>
     */
    /**
     * backend#3332：{@code ResolvePrincipals} 的 gRPC deadline（秒）。
     *
     * <p><b>為什麼非有不可</b>：{@code GrpcStubFactory} / {@code GrpcChannelRegistry}
     * （observability-common）全域<b>零</b> deadline，Java {@code ManagedChannelBuilder} 也沒有
     * 預設 keepalive ⇒ 下游掛住時 blocking stub <b>永不返回</b>。本呼叫是自買防護的
     * <b>硬依賴（fail-closed）</b>且位在<b>加入購物車／結帳的同步路徑</b>上，
     * 沒有 deadline ＝ 使用者那一側是無限等待。
     *
     * <p><b>為什麼是 5 秒</b>：{@code seller-service} 端只做一次
     * {@code sellerRepo.GetPrincipalsByStoreIDs} 的索引查詢（Go，無第三方、無交易、無扇出），p99 毫秒級。
     * 5 秒有三個數量級的餘裕，只用來把「永遠不回」轉成「5 秒後失敗」。
     * 取值與 aftersales 側同名 RPC 的 {@code PRINCIPAL_RESOLVE_DEADLINE_SEC} 一致。
     *
     * <p><b>為什麼不共用本類別的 {@code CALL_DEADLINE_SECONDS}（2 秒）</b>：那個值是為
     * {@code batchGetSellerMeta} 調的——該呼叫<b>可降級</b>且帶 {@code MAX_ATTEMPTS} 次重試，
     * 2 秒太短只是少一次補值。本呼叫相反：fail-closed 單發，逾時就是把合法的加入購物車擋下來，
     * 因此取同 RPC 的 repo 慣例值 5 秒而非更激進的 2 秒。
     *
     * <p>deadline 加在<b>呼叫點</b>而非 {@code GrpcStubFactory}（共用元件，動它會一次改掉全 repo
     * 所有 Java gRPC 呼叫的逾時語意）。⚠️ {@code withDeadlineAfter} 換算<b>絕對</b>時間，
     * 必須每次呼叫時套用。
     */
    public static final long PRINCIPAL_RESOLVE_DEADLINE_SEC = 5L;

    public PrincipalResolution resolvePrincipals(List<Long> storeIds) {
        if (storeIds == null || storeIds.isEmpty()) {
            return PrincipalResolution.empty();
        }
        var stub = grpcStubFactory.blocking("seller", SellerServiceGrpc::newBlockingStub);
        var request = ResolvePrincipalsRequest.newBuilder()
                .addAllIds(storeIds)
                .build();
        ResolvePrincipalsResponse response = stub
                .withDeadlineAfter(PRINCIPAL_RESOLVE_DEADLINE_SEC, TimeUnit.SECONDS)
                .resolvePrincipals(request);
        if (response.getCode() != 0) {
            // 非 0 code（業務碼錯誤，非 transport 失敗）同樣視為失敗、fail-closed，
            // 避免呼叫端拿到空 map 後 getOrDefault 悄悄退化成錯誤比對（final review follow-up）
            throw Status.INTERNAL.withDescription(response.getMessage()).asRuntimeException();
        }
        return PrincipalResolution.of(response.getPrincipalsMap(), response.getUnresolvedIdsList());
    }
}
