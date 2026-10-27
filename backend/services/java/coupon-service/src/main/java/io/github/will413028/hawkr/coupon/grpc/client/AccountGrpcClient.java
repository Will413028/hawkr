package io.github.will413028.hawkr.coupon.grpc.client;

import io.github.will413028.hawkr.account.grpc.*;
import io.github.will413028.hawkr.common.obs.grpc.GrpcStubFactory;
import io.github.will413028.hawkr.common.obs.log.ObsLogger;
import org.springframework.stereotype.Component;

import java.util.Collections;
import java.util.List;
import java.util.Map;
import java.util.concurrent.TimeUnit;
import java.util.stream.Collectors;

/**
 * Account Service gRPC Client
 *
 * 用於 ALL target_type 資格判定（取 created_at）及 DESIGNATED 詳情帳號名稱聚合
 */
@Component
public class AccountGrpcClient {

    private static final ObsLogger log = ObsLogger.forClass(AccountGrpcClient.class);

    /**
     * issue #3332：內部讀取型 RPC 的 gRPC deadline（秒）。
     *
     * <p><b>為什麼非有不可</b>：共用元件 {@code GrpcStubFactory} 全域<b>零</b> deadline，
     * Java {@code ManagedChannelBuilder} 預設無 keepalive ⇒ 下游掛住時 blocking stub 永不返回。
     * 本 client 三支全部走「失敗即降級」（-1／空 Map），而<b>永不返回連降級都到不了</b>——
     * {@code getAccountCreatedAt} 位於 ALL target_type 的領券資格判定同步路徑上。
     *
     * <p><b>10 秒怎麼來的</b>：account-service（Go）側是<b>本地 DB 讀取</b>，p99 毫秒級；
     * 取值與 {@code shipping-service} / {@code order-service} 同名常數一致。
     *
     * <p>⚠️ {@code withDeadlineAfter} 換算<b>絕對</b>時間 ⇒ 必須<b>每次呼叫時</b>套用；
     * 也<b>不得</b>改共用的 {@code GrpcStubFactory}（會一次改掉全 repo 的逾時語意）。
     */
    public static final long INTERNAL_READ_DEADLINE_SEC = 10L;
    private final GrpcStubFactory grpcStubFactory;

    public AccountGrpcClient(GrpcStubFactory grpcStubFactory) {
        this.grpcStubFactory = grpcStubFactory;
    }

    /**
     * 取得單一帳號的建立時間（Unix timestamp, ms）
     *
     * @return created_at timestamp，失敗時回傳 -1（呼叫端應視為不符資格）
     */
    public long getAccountCreatedAt(Long accountId) {
        try {
            var stub = grpcStubFactory.blocking("account", AccountServiceGrpc::newBlockingStub);
            var request = GetAccountInfoRequest.newBuilder()
                    .setAccountId(accountId)
                    .build();

            AccountInfoResponse response = stub
                    .withDeadlineAfter(INTERNAL_READ_DEADLINE_SEC, TimeUnit.SECONDS)
                    .getAccountInfo(request);
            // 失敗時 Account Service 會拋 StatusRuntimeException，由下方 catch 處理；
            // 此處只需確認 data 存在
            if (!response.hasData()) {
                return -1;
            }
            return response.getData().getCreatedAt();
        } catch (Exception e) {
            log.error("AccountGrpcClient.getAccountCreatedAt.fail", "Account Service 呼叫失敗", e);
            return -1;
        }
    }

    /**
     * 批次取得帳號資訊（account_id → nickname）
     *
     * @return Map<accountId, nickname>，失敗時回傳空 Map
     */
    public Map<Long, String> batchGetNicknames(List<Long> accountIds) {
        if (accountIds == null || accountIds.isEmpty()) {
            return Collections.emptyMap();
        }

        try {
            var stub = grpcStubFactory.blocking("account", AccountServiceGrpc::newBlockingStub);
            var request = GetAccountInfoBatchRequest.newBuilder()
                    .addAllAccountIds(accountIds)
                    .build();

            AccountInfoBatchResponse response = stub
                    .withDeadlineAfter(INTERNAL_READ_DEADLINE_SEC, TimeUnit.SECONDS)
                    .getAccountInfoBatch(request);
            return response.getDataList().stream()
                    .collect(Collectors.toMap(
                            AccountInfo::getAccountId,
                            AccountInfo::getNickname,
                            (a, b) -> a
                    ));
        } catch (Exception e) {
            log.error("AccountGrpcClient.batchGetNicknames.fail", "Account Service 批次呼叫失敗，降級為空", e);
            return Collections.emptyMap();
        }
    }

    /**
     * 批次取得帳號資訊（account_id → AccountInfo）
     *
     * @return Map<accountId, AccountInfo>，失敗時回傳空 Map
     */
    public Map<Long, AccountInfo> batchGetAccountInfo(List<Long> accountIds) {
        if (accountIds == null || accountIds.isEmpty()) {
            return Collections.emptyMap();
        }

        try {
            var stub = grpcStubFactory.blocking("account", AccountServiceGrpc::newBlockingStub);
            var request = GetAccountInfoBatchRequest.newBuilder()
                    .addAllAccountIds(accountIds)
                    .build();

            AccountInfoBatchResponse response = stub
                    .withDeadlineAfter(INTERNAL_READ_DEADLINE_SEC, TimeUnit.SECONDS)
                    .getAccountInfoBatch(request);
            return response.getDataList().stream()
                    .collect(Collectors.toMap(
                            AccountInfo::getAccountId,
                            info -> info,
                            (a, b) -> a
                    ));
        } catch (Exception e) {
            log.error("AccountGrpcClient.batchGetAccountInfo.fail", "Account Service 批次呼叫失敗，降級為空", e);
            return Collections.emptyMap();
        }
    }
}
