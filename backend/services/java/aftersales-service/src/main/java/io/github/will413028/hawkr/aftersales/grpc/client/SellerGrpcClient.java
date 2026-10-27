package io.github.will413028.hawkr.aftersales.grpc.client;

import io.github.will413028.hawkr.common.grpc.PrincipalResolution;
import io.github.will413028.hawkr.common.obs.grpc.GrpcStubFactory;
import io.github.will413028.hawkr.common.obs.log.ObsLogger;
import io.github.will413028.hawkr.seller.grpc.GetSellerBatchRequest;
import io.github.will413028.hawkr.seller.grpc.ResolvePrincipalsRequest;
import io.github.will413028.hawkr.seller.grpc.ResolvePrincipalsResponse;
import io.github.will413028.hawkr.seller.grpc.SellerBatchResponse;
import io.github.will413028.hawkr.seller.grpc.SellerInfo;
import io.github.will413028.hawkr.seller.grpc.SellerServiceGrpc;
import org.springframework.stereotype.Component;

import java.util.Collections;
import java.util.HashMap;
import java.util.List;
import java.util.Map;
import java.util.concurrent.TimeUnit;

/**
 * Seller Service gRPC Client.
 *
 * <p>提供批量查詢賣家輕量資訊（名稱 + logo）。內部呼叫，x-internal-service 由
 * GrpcStubFactory 自動注入。§10.6.6 Admin 列表 / 詳情 API 使用。</p>
 *
 * <p>降級策略：查詢失敗回空 map，不 throw，由呼叫端以空字串填欄位。</p>
 */
@Component
public class SellerGrpcClient {

    private static final ObsLogger log = ObsLogger.forClass(SellerGrpcClient.class);

    /**
     * 身分反解（{@code store_id → account_id}）的 gRPC deadline（秒）——issue #2294 P1-C。
     *
     * <p><b>為什麼非有不可</b>：{@code GrpcStubFactory} / {@code GrpcChannelRegistry}
     * （observability-common）全域<b>零</b> deadline。而 issue #2294 P1-2 把身分反解從 tx 內
     * <b>搬到了 tx 之外的排程執行緒上</b>——row lock 是解掉了，「無界呼叫」本身沒有。
     * aftersales <b>沒有設定</b> {@code spring.task.scheduling.pool.size}
     * （{@code application.yml} 只設 Hikari），Spring Boot 預設是<b>單一排程執行緒</b>
     * ⇒ 對側「掛住」（不是拒絕，是不回應）時，這一支就會把
     * auto-reject／auto-confirm／store-pickup／refund worker／refund sync／24h 預警／stuck alert
     * <b>全部</b>一起停住，而且<b>永遠不會自己恢復</b>。
     *
     * <p><b>5 秒是怎麼推出來的</b>：
     * <ol>
     *   <li><b>下界</b>——這是 seller-service 的一次 PK 查表（{@code ResolvePrincipals}），
     *       不像 {@link io.github.will413028.hawkr.aftersales.grpc.client.PaymentGrpcClient#CREATE_REFUND_DEADLINE_SEC}
     *       那樣把第三方（PayUni 90 秒）夾在中間，兩者<b>不同量級</b>。本 repo 既有唯一先例是
     *       {@code cart-service} 的 {@code SellerGrpcClient.CALL_DEADLINE_SECONDS = 2s}
     *       （買家同步路徑，配 {@code MAX_ATTEMPTS=2} ⇒ 實質 4 秒預算）。本呼叫點<b>沒有</b>重試，
     *       故一次給到接近的總預算 5 秒，涵蓋 channel 重建（TCP+HTTP/2 handshake）＋一次 GC 停頓
     *       ＋一次慢查詢，而不至於製造假的 {@code DEADLINE_EXCEEDED}。</li>
     *   <li><b>上界</b>——單一排程執行緒的佔用要有界。最壞情形是
     *       {@code AutoConfirmReceiptScheduler}／{@code StorePickupTimeoutScheduler} 的
     *       {@code BATCH_LIMIT=200} 逐筆全部逾時：200 × 2 支（seller＋buyer）× 5 秒 = 2000 秒。
     *       取 10 秒則為 4000 秒，故取建議區間（5–10 秒）的<b>下緣</b>。</li>
     * </ol>
     *
     * <p>⚠️ <b>誠實的殘餘</b>：2000 秒仍<b>大於</b>那兩支排程自己的 cron 週期（10 分鐘 = 600 秒）。
     * 也就是說本 deadline 買到的是「<b>執行緒不會被永久吃掉、對側恢復後排程自己活回來</b>」，
     * <b>不是</b>「一個 tick 一定跑得完」。要讓後者成立需要批次級熔斷或獨立排程執行緒池，
     * 那是另一個題目（本批不做）。
     *
     * <p>⚠️ <b>必須加在呼叫點、每次呼叫時加</b>：{@code withDeadlineAfter} 是以「呼叫這個方法的當下」
     * 為起點換算成<b>絕對</b> deadline。若寫在建構子（本類的 stub 是建構子時建立的欄位），
     * 那個 deadline 會從<b>服務啟動那一刻</b>起算 ⇒ 啟動 5 秒後所有呼叫一律 {@code DEADLINE_EXCEEDED}。
     * 同時<b>不得</b>改共用的 {@code GrpcStubFactory}——那會一次改掉全 repo 所有 Java gRPC 呼叫的逾時語意。
     */
    public static final long PRINCIPAL_RESOLVE_DEADLINE_SEC = 5L;

    /**
     * {@code getSellerBatch} 的 gRPC deadline（秒）——issue #3332。
     *
     * <p>與 {@link #PRINCIPAL_RESOLVE_DEADLINE_SEC} 分開：那個 5 秒是「身分反解」與 buyer 側
     * <b>合起來算</b>的排程預算（見該常數 javadoc），本支是賣家 profile 批次讀取（純 DB）、
     * 失敗即降級回空 map，不參與那個推導 ⇒ 取內部呼叫慣例 10 秒。
     *
     * <p>⚠️ 同樣<b>必須在每次呼叫時</b>套用（本類的 stub 是建構子時建立的欄位）。
     */
    public static final long INTERNAL_READ_DEADLINE_SEC = 10L;

    private final SellerServiceGrpc.SellerServiceBlockingStub sellerStub;

    public SellerGrpcClient(GrpcStubFactory grpcStubFactory) {
        this.sellerStub = grpcStubFactory.blocking("seller", SellerServiceGrpc::newBlockingStub);
    }

    /**
     * 批量查詢賣家 profile。
     *
     * @return key=storeId, value=SellerInfo；查詢失敗回空 map（降級）
     */
    public Map<Long, SellerInfo> getSellerProfiles(List<Long> storeIds) {
        if (storeIds == null || storeIds.isEmpty()) {
            return Collections.emptyMap();
        }
        try {
            GetSellerBatchRequest request = GetSellerBatchRequest.newBuilder()
                    .addAllStoreIds(storeIds)
                    .build();
            // 不能挪到建構子——理由見 PRINCIPAL_RESOLVE_DEADLINE_SEC 的最後一段。
            SellerBatchResponse response = sellerStub
                    .withDeadlineAfter(INTERNAL_READ_DEADLINE_SEC, TimeUnit.SECONDS)
                    .getSellerBatch(request);
            Map<Long, SellerInfo> result = new HashMap<>();
            for (SellerInfo info : response.getDataList()) {
                result.put(Long.parseLong(info.getStoreId()), info);
            }
            return result;
        } catch (Exception e) {
            log.warn("SellerGrpcClient.getSellerProfiles.error",
                    "賣家資訊查詢失敗，降級回空 map: " + e.getMessage(),
                    "count", String.valueOf(storeIds.size()));
            return Collections.emptyMap();
        }
    }

    /**
     * 將 store_id 反解回目前持有該 store 的登入 account_id（seller-service
     * {@code ResolvePrincipals} RPC）。
     * <p>
     * 用途是 <b>domain event payload 的 {@code seller_account_id}</b>（消費端 notification-service
     * 以登入 account_id 定址），以及排程／consumer 的通知路徑。case 只存 store_id，所以要反解一次。
     * <p>
     * ⚠️ <b>賣家工作區 realtime（{@code seller.after_sale.updated}）不再經過這裡。</b>
     * 本 javadoc 曾寫「SSE topic 是登入 account_id」——那個前提在 2026-08-30（commit
     * {@code 57cefff9f}）就失效了：gateway 的訂閱端改成解析 current store_id，
     * 而發布端仍反解成 account_id ⇒ 對 {@code store_id != account_id} 的賣家 100% 送不到。
     * 已於 2026-09-03 修正為 store_id 直送（{@code AfterSalesEventPublisherImpl#emitSellerAfterSaleUpdated}）。
     * 新增呼叫端前先問：這個值的<b>消費端</b>用哪個身分空間定址？
     *
     * @return 反解後的 account_id；查無存活店家（已關閉或不存在，{@code unresolved_ids}）回 {@code 0L}（沒有收件人，
     *         與買家側 best-effort 的 {@code 0L} 一致，不回吐未驗證的 store_id）；查詢失敗（含 issue #2294 P1-C 加上的
     *         {@link #PRINCIPAL_RESOLVE_DEADLINE_SEC} 逾時）降級回傳 {@code null}
     *         ——本方法<b>永不拋</b>，failure mode 只有這一種。呼叫端據此分流：
     *         strict（排程）把 {@code null} 轉成 {@code PrincipalResolutionFailedException} 跳過本筆、
     *         best-effort（API）維持跳過本次 realtime 推播（與既有降級策略一致）
     */
    public Long resolveAccountIdForStore(Long storeId) {
        if (storeId == null) {
            return null;
        }
        try {
            ResolvePrincipalsRequest request = ResolvePrincipalsRequest.newBuilder()
                    .addAllIds(List.of(storeId))
                    .build();
            // issue #2294 P1-C：deadline 必須在**每次呼叫時**套用（絕對時間以此刻起算），
            // 不能挪到建構子——理由見 PRINCIPAL_RESOLVE_DEADLINE_SEC 的最後一段。
            ResolvePrincipalsResponse response = sellerStub
                    .withDeadlineAfter(PRINCIPAL_RESOLVE_DEADLINE_SEC, TimeUnit.SECONDS)
                    .resolvePrincipals(request);
            if (response.getCode() != 0) {
                throw new IllegalStateException("ResolvePrincipals code=" + response.getCode());
            }
            PrincipalResolution resolution =
                    PrincipalResolution.of(response.getPrincipalsMap(), response.getUnresolvedIdsList());
            if (resolution.isUnresolved(storeId)) {
                // 單體的 store_id 與 account_id 是兩個序列：沒有解析到就沒有收件人，不能拿 store_id 冒充 account_id。
                log.warn("SellerGrpcClient.resolveAccountIdForStore.unresolved",
                        "store_id 查無存活店家（已關閉或不存在），該筆通知的 seller_account_id 帶 0",
                        "storeId", String.valueOf(storeId));
                return 0L;
            }
            return resolution.principals().get(storeId);
        } catch (Exception e) {
            log.warn("SellerGrpcClient.resolveAccountIdForStore.error",
                    "store_id 反解 account_id 失敗，該筆通知的 seller_account_id 將降級: " + e.getMessage(),
                    "storeId", String.valueOf(storeId));
            return null;
        }
    }
}
