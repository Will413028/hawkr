package io.github.will413028.hawkr.shipping.client;

import io.github.will413028.hawkr.buyer.grpc.BatchGetBuyerDisplayByBuyerIDsRequest;
import io.github.will413028.hawkr.buyer.grpc.BatchGetBuyerDisplayByBuyerIDsResponse;
import io.github.will413028.hawkr.buyer.grpc.BuyerDisplay;
import io.github.will413028.hawkr.buyer.grpc.BuyerServiceGrpc;
import io.github.will413028.hawkr.buyer.grpc.ResolvePrincipalsRequest;
import io.github.will413028.hawkr.buyer.grpc.ResolvePrincipalsResponse;
import io.github.will413028.hawkr.buyer.grpc.ValidateAddressRequest;
import io.github.will413028.hawkr.buyer.grpc.ValidateAddressResponse;
import io.github.will413028.hawkr.common.grpc.PrincipalResolution;
import io.github.will413028.hawkr.common.obs.grpc.GrpcStubFactory;
import io.github.will413028.hawkr.common.obs.log.ObsLogger;
import io.grpc.Status;
import org.springframework.stereotype.Component;

import java.util.List;
import java.util.Map;
import java.util.concurrent.TimeUnit;

/**
 * Buyer Service gRPC Client（shipping-service 用）
 *
 * <p>提供批量查詢買家輕量資訊（暱稱+頭像）。內部呼叫，{@code x-internal-service}
 * metadata 由 {@link GrpcStubFactory} / GrpcAuthClientInterceptor 自動注入。
 *
 * <p>{@code BuyerDisplay.avatar_key} 是**裸 objectKey**（49 §6.3/S3），不是絕對 URL；
 * MQ payload 只帶 objectKey，由消費端 worker 組 URL（見 {@code ShipmentCreatedPayload}）。
 * （2026-08-29 更正：原文寫「avatar_url 已轉為絕對 URL」——那個欄位不存在，
 *  且與同批 {@code ShippingService} 內「只帶 objectKey」的註解直接矛盾。）
 *
 * <p>僅提供 buyer_id PK lookup 與 principal 解析；不保留 account-axis 的
 * 同名 overload，避免把持久化 buyer_id 誤送進 account query 後靜默查空。
 */
@Component
public class BuyerGrpcClient {

    private static final ObsLogger log = ObsLogger.forClass(BuyerGrpcClient.class);

    /**
     * issue #2294：內部讀取型 RPC 的 gRPC deadline（秒）。
     *
     * <p><b>為什麼非有不可</b>：{@code GrpcStubFactory} / {@code GrpcChannelRegistry}
     * （observability-common）全域<b>零</b> deadline 設定 ⇒ 下游若是「掛住」而非「失敗」，
     * blocking stub <b>永不返回</b>。本 client 被 {@code @Scheduled} 排程（§5.19 / §5.20）
     * 間接使用，而 Spring Boot 的 {@code spring.task.scheduling.pool.size} 預設是 <b>1</b>
     * ⇒ 一次掛住就讓本服務的<b>所有</b>排程停擺且永不恢復；同時使排程搶佔的租約下界
     * （《多副本排程搶佔規範》§三「租約必須 &gt; 單筆工作的最壞耗時」）無從建立。
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

    private final BuyerServiceGrpc.BuyerServiceBlockingStub buyerStub;

    public BuyerGrpcClient(GrpcStubFactory grpcStubFactory) {
        this.buyerStub = grpcStubFactory.blocking("buyer", BuyerServiceGrpc::newBlockingStub);
    }

    /**
     * PK（buyer_id）版本的批量查詢買家資訊（暱稱、頭像）。出貨資訊上持久化的買家欄位是 PK
     * （出貨當下的買家身分），不是 account_id——兩者設計上永不相等，該欄位下游
     * 查詢一律走這支，不能用 account_id/current-stint 版本（會對已重新註冊過
     * 的帳號靜默查空，見 2026-07-26 事故）。
     *
     * @param buyerIds 出貨資訊上持久化的買家 buyer_id 列表
     * @return Map&lt;buyerId, BuyerDisplay&gt;；空列表或對方無資料時回傳空 map
     */
    public Map<Long, BuyerDisplay> batchGetBuyerDisplayByBuyerIds(List<Long> buyerIds) {
        log.info("BuyerGrpcClient.batchGetBuyerDisplayByBuyerIds.start",
                "批量查詢買家資訊(PK)", "count", String.valueOf(buyerIds.size()));
        BatchGetBuyerDisplayByBuyerIDsRequest request = BatchGetBuyerDisplayByBuyerIDsRequest.newBuilder()
                .addAllBuyerIds(buyerIds)
                .build();
        BatchGetBuyerDisplayByBuyerIDsResponse response = buyerStub
                .withDeadlineAfter(INTERNAL_READ_DEADLINE_SEC, TimeUnit.SECONDS)
                .batchGetBuyerDisplayByBuyerIDs(request);
        log.info("BuyerGrpcClient.batchGetBuyerDisplayByBuyerIds.done",
                "買家資訊回應 code=" + response.getCode(),
                "resultCount", String.valueOf(response.getBuyersCount()));
        return response.getBuyersMap();
    }

    /**
     * 解析 buyer_id → 登入 principal account_id（notification routing 用）。
     * 硬依賴：Buyer Service 失敗時拋出例外，由呼叫端 fail-closed/fail-soft 決定
     * （鏡像 aftersales-service {@code BuyerGrpcClient.resolvePrincipals} 與 order-service 同名檔案）。
     * shipment 買家推播身分 rekey 補完新增。
     *
     * <p>回傳 {@link PrincipalResolution} 而非裸 map（issue #2789 階段 B2）：查無在架買家的
     * buyer_id 會被 server <b>原樣回吐進 map</b>（{@code buyer.proto} 的 {@code unresolved_ids}
     * 逐字寫明那是 UNVERIFIED echo-back），所以 {@code getOrDefault(id, fallback)} 的 fallback
     * 分支是死碼、拿到的 echo 值與真實解析結果逐位元無法區分。呼叫端一律用
     * {@link PrincipalResolution#isUnresolved(Long)} 判定，拿不到「只有 map」的版本。
     *
     * @param buyerIds 待解析的 buyer_id 列表
     * @return principals（每個輸入 id 都有 entry，未解析者是 echo-back）＋ unresolvedIds
     */
    public PrincipalResolution resolvePrincipals(List<Long> buyerIds) {
        if (buyerIds.isEmpty()) {
            return PrincipalResolution.empty();
        }
        ResolvePrincipalsRequest request = ResolvePrincipalsRequest.newBuilder()
                .addAllIds(buyerIds)
                .build();
        ResolvePrincipalsResponse response = buyerStub
                .withDeadlineAfter(INTERNAL_READ_DEADLINE_SEC, TimeUnit.SECONDS)
                .resolvePrincipals(request);
        if (response.getCode() != 0) {
            throw Status.INTERNAL.withDescription(response.getMessage()).asRuntimeException();
        }
        return PrincipalResolution.of(response.getPrincipalsMap(), response.getUnresolvedIdsList());
    }

    /**
     * 地址驗證與地理編碼（Uber 報價用）。
     *
     * <h2>🔴 為什麼是打 buyer-service 而不是自己接 TGOS</h2>
     * 這支底下是 TGOS，而 TGOS 是<b>來源 IP 白名單</b>的第三方
     * ⇒ 直連必須走 egress-proxy（10 檔 §26）。shipping-service 目前沒有那條出口，
     * 而 buyer-service 已經有、且已經在用。再接一條等於把同一個第三方的憑證、
     * 斷路器與白名單複製第二份（10 檔 §15／§26 ③ 逐字禁止為單一第三方另開 proxy）。
     *
     * <p>🔴 <b>本呼叫是報價路徑的硬依賴，失敗時 fail-closed</b>：拿不到經緯度就報不了價
     * （Uber 建單 {@code dropoff_latitude}／{@code dropoff_longitude} 為必填，
     * 官方總覽 §2.2）。fail-open 的後果是寫進一筆沒有座標的報價，
     * 而那筆報價在<b>買家付完錢之後</b>才會在建單時炸掉。
     *
     * <p>⚠️ 網路面本來就通：{@code k8s/values/buyer-service.yaml} 的
     * {@code networkPolicy.grpcAllowedCallers} 實讀已含 {@code shipping-service}
     * ⇒ 本方法<b>不需要</b>新增放行（10 檔 §14.5.0c 的三件已滿足其一其二，
     * 第三件「policy 註解有沒有寫只有 X 會呼叫」不適用：該清單本來就是多呼叫端）。
     *
     * @param fullAddress 完整地址字串（縣市＋鄉鎮市區＋門牌）
     * @return 驗證結果；{@code valid=false} 時呼叫端必須把 Uber 選項標成不可用
     * @throws io.grpc.StatusRuntimeException 下游錯誤或逾時——🔴 不吞
     */
    public ValidateAddressResponse validateAddress(String fullAddress) {
        ValidateAddressRequest request = ValidateAddressRequest.newBuilder()
                .setAddress(fullAddress == null ? "" : fullAddress)
                .build();
        return buyerStub
                .withDeadlineAfter(INTERNAL_READ_DEADLINE_SEC, TimeUnit.SECONDS)
                .validateAddress(request);
    }
}
