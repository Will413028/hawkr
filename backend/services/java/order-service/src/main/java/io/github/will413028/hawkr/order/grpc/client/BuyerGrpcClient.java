package io.github.will413028.hawkr.order.grpc.client;

import io.github.will413028.hawkr.buyer.grpc.AddressInfo;
import io.github.will413028.hawkr.buyer.grpc.BatchGetBuyerDisplayByBuyerIDsRequest;
import io.github.will413028.hawkr.buyer.grpc.BatchGetBuyerDisplayByBuyerIDsResponse;
import io.github.will413028.hawkr.buyer.grpc.BuyerDisplay;
import io.github.will413028.hawkr.buyer.grpc.BuyerServiceGrpc;
import io.github.will413028.hawkr.buyer.grpc.GetAddressRequest;
import io.github.will413028.hawkr.buyer.grpc.GetAddressResponse;
import io.github.will413028.hawkr.buyer.grpc.ResolvePrincipalsRequest;
import io.github.will413028.hawkr.buyer.grpc.ResolvePrincipalsResponse;
import io.github.will413028.hawkr.common.grpc.PrincipalResolution;
import io.github.will413028.hawkr.common.obs.grpc.GrpcStubFactory;
import io.github.will413028.hawkr.common.obs.log.ObsLogger;
import io.grpc.Status;
import org.springframework.stereotype.Component;

import java.util.List;
import java.util.Map;
import java.util.concurrent.TimeUnit;

/**
 * Buyer Service gRPC Client
 *
 * 提供批量查詢買家輕量資訊（暱稱+頭像）
 * 內部呼叫，x-internal-service 自動注入
 */
@Component
public class BuyerGrpcClient {

    private static final ObsLogger log = ObsLogger.forClass(BuyerGrpcClient.class);

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

    private final BuyerServiceGrpc.BuyerServiceBlockingStub buyerStub;

    public BuyerGrpcClient(GrpcStubFactory grpcStubFactory) {
        this.buyerStub = grpcStubFactory.blocking("buyer", BuyerServiceGrpc::newBlockingStub);
    }

    /**
     * PK（buyer_id）版本的 batchGetBuyerDisplay。orders.buyer_id 是 PK（下單當下
     * 的買家身分），不是 account_id——兩者設計上永不相等，該欄位下游查詢一律
     * 走這支，不能用 account_id/current-stint 版本的 batchGetBuyerMeta（那支已註銷者查不到）（會對
     * 已重新註冊過的帳號靜默查空，見 2026-07-26 事故）。
     *
     * @param buyerIds 訂單上持久化的買家 buyer_id 列表
     * @return Map<buyerId, BuyerDisplay>（已註銷者回墓碑字串，可直接顯示）
     */
    public Map<Long, BuyerDisplay> batchGetBuyerDisplayByBuyerIds(List<Long> buyerIds) {
        log.info("BuyerGrpcClient.batchGetBuyerDisplayByBuyerIds.start",
                "批量查詢買家資訊(PK)", "count", String.valueOf(buyerIds.size()));
        // #2830：改打 Display 版。Meta 版底層帶 `deleted_at IS NULL`（刻意的 PII 邊界），
        // 買家註銷後解不出來 ⇒ 賣家訂單列表／詳情的 buyerNickname 變成空字串，而
        // gateway 帶 omitempty ⇒ 該欄位整個從 JSON 消失。Display 版讓已註銷的 stint
        // 也解得出來，但只回墓碑字串「已刪除帳號」、頭像清空，真名不離開 buyer-service。
        BatchGetBuyerDisplayByBuyerIDsRequest request = BatchGetBuyerDisplayByBuyerIDsRequest.newBuilder()
                .addAllBuyerIds(buyerIds)
                .build();
        BatchGetBuyerDisplayByBuyerIDsResponse response = buyerStub
                .withDeadlineAfter(INTERNAL_READ_DEADLINE_SEC, TimeUnit.SECONDS)
                .batchGetBuyerDisplayByBuyerIDs(request);
        log.info("BuyerGrpcClient.batchGetBuyerDisplayByBuyerIds.done",
                "買家資訊回應 code=" + response.getCode(), "resultCount", String.valueOf(response.getBuyersCount()));
        return response.getBuyersMap();
    }

    /**
     * 取單一收件地址（：Uber 報價的 dropoff 來源）。
     *
     * <p>🔴 <b>歸屬由 buyer-service 保證</b>：{@code GetAddress} 的底層是
     * {@code addressUC.GetByID(ctx, buyerId, addressId)}，兩個鍵一起進 where ⇒
     * 拿別人的 addressId 來查會落在「查無」而不是「查到別人的地址」。
     * 呼叫端<b>不要</b>自己再比一次 {@code AddressInfo.buyer_id}：那個欄位是回程資料，
     * 用回程資料驗歸屬等於沒驗。
     *
     * @param buyerId   買家業務鍵（buyers PK），不是 account_id
     * @param addressId {@code shipping_addresses.id}
     * @return 地址；<b>查無或無權限（回應 code != 0）時回 {@code null}</b>——那是預期中的
     *         業務狀態（買家把地址刪了、前端帶了舊 id），不是例外。真正的例外
     *         （buyer-service 掛了、逾時）仍以 {@link io.grpc.StatusRuntimeException} 拋出。
     */
    public AddressInfo getAddress(long buyerId, long addressId) {
        GetAddressRequest request = GetAddressRequest.newBuilder()
                .setBuyerId(buyerId)
                .setAddressId(addressId)
                .build();
        GetAddressResponse response = buyerStub
                .withDeadlineAfter(INTERNAL_READ_DEADLINE_SEC, TimeUnit.SECONDS)
                .getAddress(request);
        if (response.getCode() != 0) {
            log.warn("BuyerGrpcClient.getAddress.notFound",
                    "查無收件地址 code=" + response.getCode() + " message=" + response.getMessage(),
                    "addressId", String.valueOf(addressId));
            return null;
        }
        return response.getData();
    }

    /**
     * 解析 buyer_id → 登入 principal account_id。
     * 硬依賴：Buyer Service 失敗時拋出例外，由呼叫端 fail-closed/fail-soft 決定
     * （鏡像 {@code checkout.client.SellerGrpcClient.resolvePrincipals}）。
     *
     * <p>回傳 {@link PrincipalResolution} 而非裸 map（issue #2789 階段 B2）：查無在架買家的
     * buyer_id 會被 server <b>原樣回吐進 map</b>（{@code buyer.proto} 的 {@code unresolved_ids}
     * 逐字寫明那是 UNVERIFIED echo-back），所以 {@code getOrDefault(id, id)} 拿到的值與真實
     * 解析結果<b>逐位元無法區分</b>。閘門類呼叫端（自買防護）必須用
     * {@link PrincipalResolution#requireAllResolved} fail-closed；通知類呼叫端讀
     * {@link PrincipalResolution#principals()}，取值與改動前完全一致。
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
}
