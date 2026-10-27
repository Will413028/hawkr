package io.github.will413028.hawkr.aftersales.grpc.client;

import io.github.will413028.hawkr.buyer.grpc.BatchGetBuyerDisplayByBuyerIDsRequest;
import io.github.will413028.hawkr.buyer.grpc.BatchGetBuyerDisplayByBuyerIDsResponse;
import io.github.will413028.hawkr.buyer.grpc.BuyerDisplay;
import io.github.will413028.hawkr.buyer.grpc.BuyerServiceGrpc;
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
 * 提供批量查詢買家輕量資訊（暱稱+頭像）。
 * 內部呼叫，x-internal-service 由 GrpcStubFactory 自動注入。
 */
@Component
public class BuyerGrpcClient {

    private static final ObsLogger log = ObsLogger.forClass(BuyerGrpcClient.class);

    /**
     * 身分反解（{@code buyer_id → notify account_id}）的 gRPC deadline（秒）——issue #2294 P1-C。
     *
     * <p>與 {@link SellerGrpcClient#PRINCIPAL_RESOLVE_DEADLINE_SEC} 是<b>同一個推導、同一個值</b>：
     * 兩支合起來才是「一次身分反解」，跑在同一條（單一）排程執行緒上，逾時預算必須一起算。
     * 完整的上下界推導寫在 seller 側那個常數的 javadoc，此處不重複。
     * <b>刻意直接引用而非各寫一份 {@code 5L}</b>：兩個數字若能各自被調整，那個「合起來算」的推導
     * 就會在下一次改動時靜默失效（{@code 02-judgment.md} §2.1 的同一形狀）。
     *
     * <p>⚠️ 同樣<b>必須在每次呼叫時</b>套用（本類的 stub 也是建構子時建立的欄位）。
     */
    public static final long PRINCIPAL_RESOLVE_DEADLINE_SEC =
            SellerGrpcClient.PRINCIPAL_RESOLVE_DEADLINE_SEC;

    /**
     * {@code batchGetBuyerDisplayByBuyerIDs} 的 gRPC deadline（秒）——issue #3332。
     *
     * <p>與 {@link #PRINCIPAL_RESOLVE_DEADLINE_SEC} 分開：那個 5 秒是「身分反解」與 seller 側
     * <b>合起來算</b>的預算（見該常數 javadoc），本支是獨立的買家顯示資訊批次讀取（純 DB），
     * 不參與那個推導 ⇒ 取內部呼叫慣例 10 秒。
     *
     * <p>⚠️ 同樣<b>必須在每次呼叫時</b>套用：本類的 stub 是<b>建構子時建立的欄位</b>，
     * 若把 deadline 綁在建構子，絕對時間會從服務啟動那一刻起算 ⇒ 啟動 10 秒後全部
     * {@code DEADLINE_EXCEEDED}。
     */
    public static final long INTERNAL_READ_DEADLINE_SEC = 10L;

    private final BuyerServiceGrpc.BuyerServiceBlockingStub buyerStub;

    public BuyerGrpcClient(GrpcStubFactory grpcStubFactory) {
        this.buyerStub = grpcStubFactory.blocking("buyer", BuyerServiceGrpc::newBlockingStub);
    }

    /**
     * 批量查詢買家輕量資訊（暱稱+頭像）。
     *
     * @param buyerIds 售後案件上持久化的買家 buyer_id 列表
     * @return Map<buyerId, BuyerDisplay>
     */
    public Map<Long, BuyerDisplay> batchGetBuyerDisplayByBuyerIds(List<Long> buyerIds) {
        log.info("BuyerGrpcClient.batchGetBuyerDisplayByBuyerIds.start",
                "批量查詢買家資訊(PK)", "count", String.valueOf(buyerIds.size()));
        BatchGetBuyerDisplayByBuyerIDsRequest request = BatchGetBuyerDisplayByBuyerIDsRequest.newBuilder()
                .addAllBuyerIds(buyerIds)
                .build();
        // 不能挪到建構子——理由見 INTERNAL_READ_DEADLINE_SEC 的最後一段。
        BatchGetBuyerDisplayByBuyerIDsResponse response = buyerStub
                .withDeadlineAfter(INTERNAL_READ_DEADLINE_SEC, TimeUnit.SECONDS)
                .batchGetBuyerDisplayByBuyerIDs(request);
        log.info("BuyerGrpcClient.batchGetBuyerDisplayByBuyerIds.done",
                "買家資訊回應 code=" + response.getCode(),
                "resultCount", String.valueOf(response.getBuyersCount()));
        return response.getBuyersMap();
    }

    /**
     *
     * @param buyerIds 待解析的 buyer_id 列表
     * @return {@link PrincipalResolution}（common-core 共用型別，與 seller 軸同形）：
     *         principals（每個輸入 id 都有 entry，未解析者 echo 回自己）+ unresolvedIds（真正查無對應
     *         買家列的 id 子集，其在 principals 裡的值是未經驗證的 echo-back，不是真實解析結果）。
     *         <b>issue #2789 階段 B2 收斂</b>：原本是本 class 的 nested record（買家軸前身），
     *         與 common-core 的 {@code PrincipalResolution} 形狀與語意完全相同，
     *         留兩份只會讓「閘門必須用 requireResolved」這條規則在買家軸這邊選不到。
     * @throws io.grpc.StatusRuntimeException transport 失敗或逾時
     *         （issue #2294 P1-C 起最壞 {@link #PRINCIPAL_RESOLVE_DEADLINE_SEC} 秒即以
     *         {@code DEADLINE_EXCEEDED} 返回，不再無限等待）。
     *         與 seller 側不同，本方法<b>會拋</b>，降級與否由呼叫端決定
     *         （strict 轉 {@code PrincipalResolutionFailedException}、best-effort 退回 {@code 0L}）
     */
    public PrincipalResolution resolvePrincipals(List<Long> buyerIds) {
        if (buyerIds.isEmpty()) {
            return PrincipalResolution.empty();
        }
        ResolvePrincipalsRequest request = ResolvePrincipalsRequest.newBuilder()
                .addAllIds(buyerIds)
                .build();
        // issue #2294 P1-C：deadline 必須在**每次呼叫時**套用（絕對時間以此刻起算），
        // 不能挪到建構子——理由見 SellerGrpcClient.PRINCIPAL_RESOLVE_DEADLINE_SEC 的最後一段。
        ResolvePrincipalsResponse response = buyerStub
                .withDeadlineAfter(PRINCIPAL_RESOLVE_DEADLINE_SEC, TimeUnit.SECONDS)
                .resolvePrincipals(request);
        if (response.getCode() != 0) {
            throw Status.INTERNAL.withDescription(response.getMessage()).asRuntimeException();
        }
        return PrincipalResolution.of(response.getPrincipalsMap(), response.getUnresolvedIdsList());
    }
}
