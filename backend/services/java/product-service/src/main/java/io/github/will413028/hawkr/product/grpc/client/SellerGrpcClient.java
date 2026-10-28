package io.github.will413028.hawkr.product.grpc.client;

import io.github.will413028.hawkr.common.obs.grpc.GrpcStubFactory;
import io.github.will413028.hawkr.common.obs.log.ObsLogger;
import io.github.will413028.hawkr.seller.grpc.BatchGetSellerMetaRequest;
import io.github.will413028.hawkr.seller.grpc.BatchGetSellerMetaResponse;
import io.github.will413028.hawkr.seller.grpc.ListPublicStoreIDsByCityRequest;
import io.github.will413028.hawkr.seller.grpc.MatchCertificateHolderRequest;
import io.github.will413028.hawkr.seller.grpc.MatchCertificateHolderResponse;
import io.github.will413028.hawkr.seller.grpc.ListPublicStoresByCityRequest;
import io.github.will413028.hawkr.seller.grpc.PublicCityStores;
import io.github.will413028.hawkr.seller.grpc.SellerMeta;
import io.github.will413028.hawkr.seller.grpc.SellerServiceGrpc;
import org.springframework.stereotype.Component;

import java.util.ArrayList;
import java.util.List;
import java.util.Map;
import java.util.Set;
import java.util.concurrent.TimeUnit;

/**
 * Seller Service gRPC Client
 *
 * 用於運費計算時取得賣家元資料（zip_code → 出貨地區判定）
 * 失敗時降級為 null（呼叫端預設為本島）
 */
@Component
public class SellerGrpcClient {

    private static final ObsLogger log = ObsLogger.forClass(SellerGrpcClient.class);
    private static final String SELLER_TARGET = "seller";

    /**
     * issue #3332：內部讀取型 RPC 的 gRPC deadline（秒）。
     *
     * <p><b>為什麼非有不可</b>：共用元件 {@code GrpcStubFactory} 全域<b>零</b> deadline，
     * Java {@code ManagedChannelBuilder} 預設無 keepalive ⇒ 下游掛住時 blocking stub 永不返回。
     * 本 client 的 {@code batchGetSellerMeta} 走在運費計算與商品 ACTIVE transition 的
     * 同步路徑上（02 §5.10 / 46 §6.2），沒有上界等於把上游 worker 一起吃掉；
     * 而降級回空 Map 的那幾支更危險——永不返回連降級都到不了。
     *
     * <p><b>10 秒怎麼來的</b>：這幾支都是 seller-service 的<b>本地 DB 讀取</b>，p99 毫秒級；
     * 取值與 {@code shipping-service} / {@code order-service} 同名常數一致。
     * 比 {@link #HOLDER_MATCH_DEADLINE_SEC}（3 秒）寬，是因為那一支刻意要快失敗轉人工審核。
     *
     * <p>⚠️ {@code withDeadlineAfter} 換算<b>絕對</b>時間 ⇒ 必須<b>每次呼叫時</b>套用；
     * 也<b>不得</b>改共用的 {@code GrpcStubFactory}（會一次改掉全 repo 的逾時語意）。
     */
    public static final long INTERNAL_READ_DEADLINE_SEC = 10L;

    private final GrpcStubFactory grpcStubFactory;

    public SellerGrpcClient(GrpcStubFactory grpcStubFactory) {
        this.grpcStubFactory = grpcStubFactory;
    }

    /**
     * 查驗到的「業者名稱」是不是這個賣家的（19 號設計書 §8.5.1）。
     *
     * <p>政府 API 只回答「這個編號存在且有效」，<b>不回答「這張證書屬於誰」</b>。少了這一問，
     * 任何賣家填入一個查得到的<b>他人</b>編號就會被自動核准並公開標章
     * （2026-08-27 實測：真實編號 {@code 1-009-300022}／持有人 {@code Acme Road LLC}
     * 被自動核准到另一家的文旦禮盒上）。
     *
     * <p>🔴 <b>比對在 seller-service 內部完成</b>，這裡只拿得到布林與命中欄位名——
     * {@code real_name} 是 PII（`docs/modules/seller.md` §13 列為明文保留欄），不跨服務傳輸。
     *
     * <p>🔴 <b>失敗一律 fail-closed</b>：回 {@link HolderMatch#unavailable()}，
     * 讓呼叫端轉人工審核。這與本類其他方法「降級回空值繼續跑」的慣例<b>相反</b>——
     * 那些方法失敗只是少一點資訊（運費算保守值），這一支失敗而放行等於把漏洞打開。
     *
     * @param storeId    賣家 store 代理鍵
     * @param holderName 政府查驗回的業者名稱
     */
    public HolderMatch matchCertificateHolder(long storeId, String holderName) {
        try {
            var stub = grpcStubFactory.blocking(SELLER_TARGET, SellerServiceGrpc::newBlockingStub)
                    // ⚠️ deadline 必須加在**呼叫點**：withDeadlineAfter 以「呼叫當下」換算絕對時間，
                    //    寫在建構子會從服務啟動起算 ⇒ 啟動幾秒後全部 DEADLINE_EXCEEDED。
                    .withDeadlineAfter(HOLDER_MATCH_DEADLINE_SEC, TimeUnit.SECONDS);
            MatchCertificateHolderResponse response = stub.matchCertificateHolder(
                    MatchCertificateHolderRequest.newBuilder()
                            .setStoreId(storeId)
                            .setHolderName(holderName == null ? "" : holderName)
                            .build());
            return new HolderMatch(response.getMatched(), response.getHasReference(),
                    response.getMatchedField(), true);
        } catch (Exception e) {
            log.warn("SellerGrpcClient.matchCertificateHolder.fail",
                    "業者名稱歸屬比對失敗，轉人工審核: " + e.getClass().getName() + ": " + e.getMessage(),
                    "store_id", String.valueOf(storeId));
            return HolderMatch.unavailable();
        }
    }

    /** 比對逾時。名稱比對是單列查詢＋純字串運算，不該慢；慢了就轉人工，不要拖住核准路徑。 */
    static final long HOLDER_MATCH_DEADLINE_SEC = 3L;

    /**
     * 比對結果。三種「不放行」的成因必須分得出來，否則稽核查不出當初為什麼轉人工：
     * 確定不符（{@code hasReference=true, matched=false}）、
     * 無從判斷（{@code hasReference=false}，賣家沒有已通過的 KYC）、
     * 查不到答案（{@code available=false}，RPC 失敗或逾時）。
     */
    public record HolderMatch(boolean matched, boolean hasReference, String matchedField, boolean available) {
        public static HolderMatch unavailable() {
            return new HolderMatch(false, false, "NONE", false);
        }

        /** 唯一可以放行自動核准的情況。 */
        public boolean allowsAutoApprove() {
            return available && hasReference && matched;
        }
    }

    /**
     * 批次取得賣家元資料
     *
     * @param storeIds 賣家 store 代理鍵（store_id）集合
     * @return Map<storeId, SellerMeta>，失敗時回傳空 Map
     */
    public Map<Long, SellerMeta> batchGetSellerMetas(Set<Long> storeIds) {
        if (storeIds == null || storeIds.isEmpty()) {
            return Map.of();
        }
        try {
            var stub = grpcStubFactory.blocking(SELLER_TARGET, SellerServiceGrpc::newBlockingStub);
            var request = BatchGetSellerMetaRequest.newBuilder()
                    .addAllStoreIds(storeIds)
                    .build();

            BatchGetSellerMetaResponse response = stub
                    .withDeadlineAfter(INTERNAL_READ_DEADLINE_SEC, TimeUnit.SECONDS)
                    .batchGetSellerMeta(request);
            // 失敗時 seller-service 會拋 StatusRuntimeException，由下方 catch 降級為空 Map
            return response.getSellersMap();
        } catch (Exception e) {
            log.error("SellerGrpcClient.batchGetSellerMetas.fail",
                    "Seller Service 批次呼叫失敗: " + e.getClass().getName() + ": " + e.getMessage(),
                    e);
            return Map.of();
        }
    }

    /**
     * 取得單一賣家元資料（硬依賴，失敗直接拋出例外）。
     *
     * <p>與 {@link #getSellerMeta(long)} 降級語意相反：本方法供賣家停權 ACTIVE transition
     * backstop 使用（02 §5.10 步驟 5b / 46 §6.2），fail-closed 場景不可靜默降級——
     * gRPC 失敗或查無資料一律拋出，交由呼叫端擋下轉換。
     *
     * @param storeId 賣家 store 代理鍵（store_id），非登入 account_id
     * @return SellerMeta（含 can_manage_products）
     * @throws RuntimeException Seller Service 不可用或查無結果
     */
    public SellerMeta getSellerMetaOrThrow(long storeId) {
        var stub = grpcStubFactory.blocking(SELLER_TARGET, SellerServiceGrpc::newBlockingStub);
        var request = BatchGetSellerMetaRequest.newBuilder()
                .addAllStoreIds(List.of(storeId))
                .build();
        // 失敗時 seller-service 會拋 StatusRuntimeException，不 catch，交由呼叫端 fail-closed 處理
        BatchGetSellerMetaResponse response = stub
                .withDeadlineAfter(INTERNAL_READ_DEADLINE_SEC, TimeUnit.SECONDS)
                .batchGetSellerMeta(request);
        SellerMeta meta = response.getSellersMap().get(storeId);
        if (meta == null) {
            throw new IllegalStateException("查無賣家元資料，storeId=" + storeId);
        }
        return meta;
    }

    /**
     * 取得單一賣家元資料
     *
     * @param storeId 賣家 store 代理鍵（store_id），非登入 account_id
     * @return SellerMeta，失敗或查無結果時回傳 null（降級）
     */
    public SellerMeta getSellerMeta(long storeId) {
        try {
            var stub = grpcStubFactory.blocking(SELLER_TARGET, SellerServiceGrpc::newBlockingStub);
            var request = BatchGetSellerMetaRequest.newBuilder()
                    .addAllStoreIds(List.of(storeId))
                    .build();

            BatchGetSellerMetaResponse response = stub
                    .withDeadlineAfter(INTERNAL_READ_DEADLINE_SEC, TimeUnit.SECONDS)
                    .batchGetSellerMeta(request);
            // 失敗時 seller-service 會拋 StatusRuntimeException，由下方 catch 降級為 null
            return response.getSellersMap().get(storeId);
        } catch (Exception e) {
            // 降級：Seller Service 失敗不阻斷運費計算，預設為本島
            log.error("SellerGrpcClient.getSellerMeta.fail",
                    "Seller Service 呼叫失敗: " + e.getClass().getName() + ": " + e.getMessage(),
                    e);
            return null;
        }
    }

    /** Hard dependency used by the public seller-city product filter. */
    public List<Long> listPublicStoreIdsByCityCode(String cityCode) {
        var stub = grpcStubFactory.blocking(SELLER_TARGET, SellerServiceGrpc::newBlockingStub);
        var response = stub
                .withDeadlineAfter(INTERNAL_READ_DEADLINE_SEC, TimeUnit.SECONDS)
                .listPublicStoreIDsByCity(ListPublicStoreIDsByCityRequest.newBuilder()
                .setCity(cityCode)
                .build());
        if (response.getCode() != 0) {
            throw new IllegalArgumentException(response.getMessage());
        }
        List<Long> result = new ArrayList<>(response.getStoreIdsCount());
        for (String raw : response.getStoreIdsList()) {
            result.add(Long.parseLong(raw));
        }
        return result;
    }

    /** Hard dependency used by the complete 22-city summary. */
    public List<PublicCityStores> listPublicStoresByCity() {
        var stub = grpcStubFactory.blocking(SELLER_TARGET, SellerServiceGrpc::newBlockingStub);
        var response = stub
                .withDeadlineAfter(INTERNAL_READ_DEADLINE_SEC, TimeUnit.SECONDS)
                .listPublicStoresByCity(ListPublicStoresByCityRequest.getDefaultInstance());
        if (response.getCode() != 0) {
            throw new IllegalStateException(response.getMessage());
        }
        return response.getCitiesList();
    }
}
