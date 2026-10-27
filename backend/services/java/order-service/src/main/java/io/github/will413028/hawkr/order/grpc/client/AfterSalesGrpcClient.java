package io.github.will413028.hawkr.order.grpc.client;

import io.github.will413028.hawkr.aftersales.grpc.AfterSalesServiceGrpc;
import io.github.will413028.hawkr.aftersales.grpc.CancelActiveCaseOnConfirmReceiptRequest;
import io.github.will413028.hawkr.aftersales.grpc.CancelActiveCaseOnConfirmReceiptResponse;
import io.github.will413028.hawkr.aftersales.grpc.GetOrderPayoutHoldRequest;
import io.github.will413028.hawkr.aftersales.grpc.GetOrderPayoutHoldResponse;
import io.github.will413028.hawkr.aftersales.grpc.GetSellerCaseClassificationRequest;
import io.github.will413028.hawkr.aftersales.grpc.GetSellerCaseClassificationResponse;
import io.github.will413028.hawkr.aftersales.grpc.HasActiveCaseRequest;
import io.github.will413028.hawkr.aftersales.grpc.HasActiveCaseResponse;
import io.github.will413028.hawkr.common.obs.grpc.GrpcStubFactory;
import io.github.will413028.hawkr.common.obs.log.ObsLogger;
import io.github.will413028.hawkr.common.utils.proto.ProtoConversionUtils;
import org.springframework.stereotype.Component;

/**
 * AfterSales Service gRPC Client（內部呼叫）
 *
 * 用於：
 * 1. 訂單自動完成倒數恢復前反查活躍 case（03 §5.8 / 09 §4.4.1）。
 * 2. 賣家工作台 Dashboard 四桶 count 的售後分類（03 §2.1 SSOT）。
 */
@Component
public class AfterSalesGrpcClient {

    private static final ObsLogger log = ObsLogger.forClass(AfterSalesGrpcClient.class);

    /**
     * backend#3332：aftersales 內部讀取／狀態轉換 RPC 的 gRPC deadline（秒）。
     *
     * <p><b>為什麼非有不可</b>：{@code GrpcStubFactory} / {@code GrpcChannelRegistry}
     * （observability-common）全域<b>零</b> deadline，Java {@code ManagedChannelBuilder} 也沒有
     * 預設 keepalive ⇒ 下游若是「掛住」而非「失敗」，blocking stub <b>永不返回</b>。
     * {@link #hasActiveCase} 由 {@code AutoCompleteScheduler} 逐筆呼叫，而 Spring Boot 的
     * {@code spring.task.scheduling.pool.size} 預設是 <b>1</b> ⇒ 一次掛住就讓 order-service 的
     * <b>所有</b>排程停擺且永不恢復。
     *
     * <p><b>10 秒怎麼來的</b>：這三支在 aftersales 端都<b>不打第三方</b>——
     * {@code hasActiveCase} 是 {@code order_no} 上的一次索引 count、
     * {@code getSellerCaseClassification} 是 store 範圍查詢＋記憶體分類、
     * {@code cancelActiveCaseOnConfirmReceipt} 是 CAS UPDATE ＋ audit ＋ MQ publish。
     * p99 皆為毫秒級，10 秒是兩個數量級以上的餘裕，只用來把「永遠不回」轉成「10 秒後失敗」。
     * 與同 package 的 {@code BuyerGrpcClient} / {@code ProductGrpcClient} 同名常數同值。
     *
     * <p>取 10 秒而非 {@link #PAYOUT_HOLD_DEADLINE_SEC} 的 5 秒：後者的值被
     * {@code GrantPaymentWorkerPayoutHoldTest.leaseCoversBothExternalCallDeadlines} 綁進租約
     * 下界的不等式，語意專屬於撥款資格反查，不應被無關呼叫點共用。這三支都不在任何租約窗口內
     * （{@code AutoCompleteScheduler} 無 claim／lease；另兩支走使用者同步路徑）。
     *
     * <p>deadline 加在<b>呼叫點</b>而非 {@code GrpcStubFactory}（共用元件，動它會一次改掉全 repo
     * 所有 Java gRPC 呼叫的逾時語意）。⚠️ {@code withDeadlineAfter} 換算<b>絕對</b>時間，
     * 必須每次呼叫時套用；挪進建構子＝啟動 10 秒後全部逾時。
     */
    public static final long INTERNAL_READ_DEADLINE_SEC = 10L;

    private final AfterSalesServiceGrpc.AfterSalesServiceBlockingStub stub;

    public AfterSalesGrpcClient(GrpcStubFactory grpcStubFactory) {
        this.stub = grpcStubFactory.blocking("aftersales", AfterSalesServiceGrpc::newBlockingStub);
    }

    /**
     * 訂單是否有活躍 case。
     * 失敗時回傳 true（保守：不恢復倒數，等下一次事件 / 對帳 worker 再試）。
     */
    public boolean hasActiveCase(String orderNo) {
        try {
            HasActiveCaseResponse resp = stub
                    .withDeadlineAfter(INTERNAL_READ_DEADLINE_SEC, java.util.concurrent.TimeUnit.SECONDS)
                    .hasActiveCase(
                    HasActiveCaseRequest.newBuilder().setOrderNo(orderNo).build());
            return resp.getHasActive();
        } catch (Exception e) {
            log.warn("AfterSalesGrpcClient.hasActiveCase.error",
                    "反查活躍 case 失敗，保守視為仍有活躍: " + e.getMessage(), "orderNo", orderNo);
            return true;
        }
    }

    /**
     * backend#3262 撥款資格反查的 deadline（秒）。
     *
     * <p><b>為什麼一定要有</b>（《多副本排程搶佔規範》§三 第 5 步）：本呼叫發生在
     * {@code GrantPaymentWorker} 的<b>租約窗口內</b>，而租約下界的定義是
     * 「> 單筆最壞耗時，含該筆會打的每一個外部呼叫的 deadline」。
     * 沒有 deadline ⇒ 最壞耗時無上界 ⇒ {@code CLAIM_LEASE_SEC} 的下界推導直接失效，
     * 兩個 pod 有機會同時處理同一筆撥款。
     *
     * <p><b>為什麼是 5 秒</b>：server 端只做一次 {@code order_no} 上的索引查詢
     * ＋ 記憶體分類（{@code AfterSalePayoutHoldService}），無外部呼叫、無交易。
     * 5 秒對這個工作量有數量級餘裕，而它進入租約下界的算式：
     * {@code 5（本呼叫）＋ 100（PaymentGrpcClient.PAYUNI_BACKED_DEADLINE_SEC）= 105 < 120（CLAIM_LEASE_SEC）}。
     * 這條不等式由 {@code GrantPaymentWorkerPayoutHoldTest.leaseCoversBothExternalCallDeadlines} 機械斷言。
     *
     * <p>逾時走 fail-closed（視為 hold），不是失敗——見 {@link #getPayoutHold}。
     *
     * <p>⚠️ {@code withDeadlineAfter} 換算<b>絕對</b>時間，必須每次呼叫時套用，
     * 不可在建構子把 stub 存成帶 deadline 的欄位（那個 deadline 會在第一次呼叫後就過期）。
     */
    public static final long PAYOUT_HOLD_DEADLINE_SEC = 5L;

    /** backend#3262 撥款資格反查結果。 */
    public enum PayoutHoldReason { NONE, ACTIVE_CASE, APPEAL_WINDOW_OPEN, APPEAL_IN_FLIGHT, LOOKUP_FAILED }

    /**
     * @param hold      true = 該訂單仍可能生出退款，禁止撥款
     * @param holdUntil hold 且<b>有時間界</b>時帶；無時間界（申訴審理中 / 反查失敗）為 null
     */
    public record PayoutHold(boolean hold, java.time.LocalDateTime holdUntil, PayoutHoldReason reason, String caseNo) {}

    /**
     * 撥款資格反查（backend#3262）：這張訂單還會不會再生出退款。
     *
     * <p><b>fail-closed</b>：RPC 失敗 / 逾時 / 回非 0 code → 一律回 hold（無時間界），
     * 呼叫端把撥款任務推遲後重問。理由不對稱——<b>撥錯款追不回來</b>（全 repo 無賣家扣回機制），
     * <b>撥晚了只是等</b>。與 {@link #hasActiveCase}（失敗保守回 true）同一個哲學。
     *
     * <p>刻意<b>不</b>拋例外：拋出去會走 {@code GrantPaymentWorker} 的 retry 分支而累加
     * {@code retry_count}，一次 aftersales 短暫不可用就會把撥款任務推進 MANUAL_INTERVENTION，
     * 那是把依賴故障誤報成撥款失敗。回 hold 讓它單純延後、不消耗重試額度。
     */
    public PayoutHold getPayoutHold(String orderNo) {
        try {
            GetOrderPayoutHoldResponse resp = stub
                    .withDeadlineAfter(PAYOUT_HOLD_DEADLINE_SEC, java.util.concurrent.TimeUnit.SECONDS)
                    .getOrderPayoutHold(
                            GetOrderPayoutHoldRequest.newBuilder().setOrderNo(orderNo).build());
            if (resp.getCode() != 0) {
                log.warn("AfterSalesGrpcClient.getPayoutHold.bizError",
                        "撥款資格反查回非 0（fail-closed 視為 hold）code=" + resp.getCode()
                                + ", message=" + resp.getMessage(), "orderNo", orderNo);
                return new PayoutHold(true, null, PayoutHoldReason.LOOKUP_FAILED, null);
            }
            if (!resp.getHold()) {
                return new PayoutHold(false, null, PayoutHoldReason.NONE, null);
            }
            java.time.LocalDateTime holdUntil = resp.getHoldUntil().isEmpty()
                    ? null
                    : ProtoConversionUtils.parseIso8601Required(resp.getHoldUntil(), "holdUntil");
            return new PayoutHold(true, holdUntil, toReason(resp.getReason()), resp.getCaseNo());
        } catch (Exception e) {
            log.warn("AfterSalesGrpcClient.getPayoutHold.error",
                    "撥款資格反查失敗（fail-closed 視為 hold）: " + e.getMessage(), "orderNo", orderNo);
            return new PayoutHold(true, null, PayoutHoldReason.LOOKUP_FAILED, null);
        }
    }

    /** proto enum → client enum；未知值（含 REASON_UNSPECIFIED）保守歸為 ACTIVE_CASE（無時間界 hold）。 */
    private static PayoutHoldReason toReason(GetOrderPayoutHoldResponse.Reason reason) {
        return switch (reason) {
            case NONE -> PayoutHoldReason.NONE;
            case APPEAL_WINDOW_OPEN -> PayoutHoldReason.APPEAL_WINDOW_OPEN;
            case APPEAL_IN_FLIGHT -> PayoutHoldReason.APPEAL_IN_FLIGHT;
            default -> PayoutHoldReason.ACTIVE_CASE;
        };
    }

    /**
     * 賣家工作台 Dashboard 售後分類（03 §2.1 SSOT）。
     * <p>
     * 失敗時 fail-loud 拋出 RuntimeException：dashboard count 必須準確（issue #1188 的核心），
     * 寧可整頁明確報錯讓賣家重整，不靜默回不準的數字（不做售後覆蓋會讓有售後的訂單重複計）。
     */
    public SellerCaseClassification getSellerCaseClassification(long storeId) {
        try {
            GetSellerCaseClassificationResponse resp = stub
                    .withDeadlineAfter(INTERNAL_READ_DEADLINE_SEC, java.util.concurrent.TimeUnit.SECONDS)
                    .getSellerCaseClassification(
                    GetSellerCaseClassificationRequest.newBuilder().setStoreId(storeId).build());
            return new SellerCaseClassification(
                    ProtoConversionUtils.parseLongOrDefault(resp.getActiveCaseCount(), 0L),
                    resp.getActiveOrderNosList(),
                    resp.getCompletedOverrideOrderNosList());
        } catch (Exception e) {
            log.warn("AfterSalesGrpcClient.getSellerCaseClassification.error",
                    "賣家售後分類失敗: " + e.getMessage(), "storeId", String.valueOf(storeId));
            throw new RuntimeException("getSellerCaseClassification failed for seller " + storeId, e);
        }
    }

    /** §2.1.1 / §8.2.1（#1225）確認收貨同步處置進行中售後 case 的結果。 */
    public enum ConfirmReceiptOutcome { NO_ACTIVE_CASE, TERMINATED, BLOCKED }

    /** outcome + caseNo + blockingStatus（BLOCKED 時帶擋住的 case status value，供組裝訊息）。 */
    public record ConfirmReceiptCaseResult(ConfirmReceiptOutcome outcome, String caseNo, String blockingStatus) {}

    /**
     * 確認收貨同步處置進行中售後 case（03 §5.4 階段 0.5，#1225）。
     * <p>
     * <b>fail-closed</b>：RPC 失敗 / 逾時 → 拋 RuntimeException，由 OrderService 阻擋確認收貨讓買家重試。
     * 不可 fail-open，否則撥款後才發現有退款中 case（對齊 {@link #hasActiveCase} 保守哲學）。
     */
    public ConfirmReceiptCaseResult cancelActiveCaseOnConfirmReceipt(String orderNo, long buyerId) {
        try {
            CancelActiveCaseOnConfirmReceiptResponse resp = stub
                    .withDeadlineAfter(INTERNAL_READ_DEADLINE_SEC, java.util.concurrent.TimeUnit.SECONDS)
                    .cancelActiveCaseOnConfirmReceipt(
                    CancelActiveCaseOnConfirmReceiptRequest.newBuilder()
                            .setOrderNo(orderNo)
                            .setBuyerId(buyerId)
                            .build());
            ConfirmReceiptOutcome outcome = switch (resp.getResult()) {
                case NO_ACTIVE_CASE -> ConfirmReceiptOutcome.NO_ACTIVE_CASE;
                case TERMINATED -> ConfirmReceiptOutcome.TERMINATED;
                case BLOCKED -> ConfirmReceiptOutcome.BLOCKED;
                default -> throw new IllegalStateException("unexpected result: " + resp.getResult());
            };
            return new ConfirmReceiptCaseResult(outcome, resp.getCaseNo(), resp.getBlockingStatus());
        } catch (Exception e) {
            log.warn("AfterSalesGrpcClient.cancelActiveCaseOnConfirmReceipt.error",
                    "確認收貨同步處置售後失敗(fail-closed，擋住確認收貨): " + e.getMessage(), "orderNo", orderNo);
            throw new RuntimeException("cancelActiveCaseOnConfirmReceipt failed for order " + orderNo, e);
        }
    }
}
