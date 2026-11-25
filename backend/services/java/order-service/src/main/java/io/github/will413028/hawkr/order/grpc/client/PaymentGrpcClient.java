package io.github.will413028.hawkr.order.grpc.client;

import io.github.will413028.hawkr.common.obs.grpc.GrpcStubFactory;
import io.github.will413028.hawkr.common.obs.log.ObsLogger;
import io.github.will413028.hawkr.common.utils.code.GrpcResponseCode;
import io.github.will413028.hawkr.payment.grpc.*;
import org.springframework.stereotype.Component;
import java.util.concurrent.TimeUnit;

/**
 * Payment Service gRPC Client
 *
 * 提供：
 * - GrantPayment：撥款指示（確認收貨後呼叫）
 * - ClosePaymentByOrderNo：關閉付款交易（訂單取消時呼叫）
 * 內部呼叫，x-internal-service 自動注入
 */
@Component
public class PaymentGrpcClient {

    private static final ObsLogger log = ObsLogger.forClass(PaymentGrpcClient.class);

    /**
     * 搬錢 RPC（{@code CreateRefund}、{@code GrantPayment}）的 gRPC deadline（秒）。
     *
     * <p><b>為什麼非有不可</b>：{@code GrpcStubFactory} / {@code GrpcChannelRegistry}
     * 全域<b>零</b> deadline ⇒ 沒有 deadline 就是「單筆最壞耗時無上界」；
     * {@code AutoRefundWorker} 與 {@code GrantPaymentWorker} 的租約下界依賴它，
     * 且 Spring Boot 排程執行緒池預設 1，一次掛住即讓 order-service 全部排程停擺。
     *
     * <p><b>為什麼取大</b>：單體的 payment 在請求內只寫退款／轉帳列並排 job，外部呼叫在 job 裡，
     * 正常耗時遠小於此值；但 order 側 {@code AutoRefundWorker} 呼叫 {@code createRefund}
     * <b>不帶冪等鍵</b>（見下方多載），逾時後重試就是另一筆退款請求，所以寧可等久也不誤判逾時。
     * 調小前要先讓呼叫帶冪等鍵，並重算兩個 worker 的租約不等式（測試守住）。
     *
     * <p>deadline 加在<b>呼叫點</b>而非 {@code GrpcStubFactory}（共用元件）；
     * ⚠️ {@code withDeadlineAfter} 換算<b>絕對</b>時間，必須每次呼叫時套用。
     */
    public static final long MONEY_MOVEMENT_DEADLINE_SEC = 100L;

    /**
     * backend#3332：payment 端<b>不</b>夾 PayUni 的內部 RPC 的 gRPC deadline（秒）。
     *
     * <p><b>為什麼非有不可</b>：{@code GrpcStubFactory} / {@code GrpcChannelRegistry}
     * （observability-common）全域<b>零</b> deadline，Java {@code ManagedChannelBuilder} 也沒有
     * 預設 keepalive ⇒ 下游掛住時 blocking stub <b>永不返回</b>。
     * {@link #getActivePaymentStatusByOrderNo} 由 {@code PaymentDeadlineScheduler}
     * （經 {@code OrderExpiryCancellationService}）逐筆呼叫，Spring Boot 排程執行緒池預設 1，
     * 一次掛住即讓 order-service 全部排程停擺。
     *
     * <p><b>為什麼不是 {@link #MONEY_MOVEMENT_DEADLINE_SEC} 的 100 秒</b>：這四支都是讀取或本地狀態變更——
     * {@code getActivePaymentStatusByOrderNo} 與兩支 {@code batchGet*ProviderStatus*} 讀付款列，
     * {@code closePaymentByOrderNo} 寫付款列並排取消 job。100 秒是為不帶冪等鍵的搬錢呼叫取大的，
     * 套在這些操作上只會讓「掛住」多撐 90 秒才被發現。
     *
     * <p><b>短 deadline 在這四支上為何安全</b>：它們都<b>不是</b>那條「逾時＝重打一次外部交易」
     * 的路徑（那是 {@link #createRefund} / {@link #grantPayment}，見
     * {@link #MONEY_MOVEMENT_DEADLINE_SEC} 的刻意取大說明）。這四支要嘛唯讀、要嘛冪等：
     * {@code closePaymentByOrderNo} 對「無活躍交易」與「已非 INIT」都回 true，
     * 且呼叫端是 best-effort；{@code getActivePaymentStatusByOrderNo} 逾時走
     * {@code OrderExpiryCancellationService} 的 fail-safe（付款真相未知 ⇒ 保守<b>不</b>取消，
     * 下輪重掃），因此逾時不會誤取消已付款訂單。
     *
     * <p>deadline 加在<b>呼叫點</b>而非 {@code GrpcStubFactory}（共用元件）；
     * ⚠️ {@code withDeadlineAfter} 換算<b>絕對</b>時間，必須每次呼叫時套用，
     * 挪進建構子＝啟動 10 秒後全部逾時。
     */
    public static final long INTERNAL_READ_DEADLINE_SEC = 10L;

    private final PaymentServiceGrpc.PaymentServiceBlockingStub paymentStub;

    public PaymentGrpcClient(GrpcStubFactory grpcStubFactory) {
        this.paymentStub = grpcStubFactory.blocking("payment", PaymentServiceGrpc::newBlockingStub);
    }

    /**
     * 依訂單編號反查 canonical 交易的付款狀態（逾期取消前反查 SoT，internal-only）
     *
     * <p>供 {@code PaymentDeadlineScheduler} 在逾期取消前反查付款狀態（見 03 §8.5 / 05 §5.12）：
     * PAID 則收斂不取消。查無交易時 payment 側回 {@code code=0} 且 {@code data} 為空（訂單未建立付款交易屬正常情況）。
     *
     * @param orderNo 訂單編號
     * @return GetActivePaymentStatusByOrderNoResponse（code=0 成功；data.status 為 PaymentTransactionStatus.value()）
     */
    public GetActivePaymentStatusByOrderNoResponse getActivePaymentStatusByOrderNo(String orderNo) {
        log.info("PaymentGrpcClient.getActivePaymentStatusByOrderNo.start", "反查訂單付款狀態", "orderNo", orderNo);
        GetActivePaymentStatusByOrderNoRequest request = GetActivePaymentStatusByOrderNoRequest.newBuilder()
                .setOrderNo(orderNo)
                .build();
        GetActivePaymentStatusByOrderNoResponse response = paymentStub
                .withDeadlineAfter(INTERNAL_READ_DEADLINE_SEC, TimeUnit.SECONDS)
                .getActivePaymentStatusByOrderNo(request);
        log.info("PaymentGrpcClient.getActivePaymentStatusByOrderNo.done",
                "反查付款狀態回應 code=" + response.getCode(), "orderNo", orderNo);
        return response;
    }

    /**
     * 撥款指示（確認收貨後呼叫）
     *
     * @param orderNo 訂單編號
     * @return GrantPaymentResponse
     */
    public GrantPaymentResponse grantPayment(String orderNo) {
        log.info("PaymentGrpcClient.grantPayment.start", "發起撥款指示", "orderNo", orderNo);
        GrantPaymentRequest request = GrantPaymentRequest.newBuilder()
                .setOrderNo(orderNo)
                .build();
        GrantPaymentResponse response = paymentStub
                .withDeadlineAfter(MONEY_MOVEMENT_DEADLINE_SEC, TimeUnit.SECONDS)
                .grantPayment(request);
        log.info("PaymentGrpcClient.grantPayment.done", "撥款指示回應 code=" + response.getCode(), "orderNo", orderNo);
        return response;
    }

    /**
     * 建立退款（原路退款，信用卡/Apple Pay/Google Pay）
     *
     * @param tradeNo 原交易編號（orders.transaction_id）
     * @param amount  退款金額（新台幣元，數字字串）
     * @param reason  退款原因
     * @return refundNo 退款編號
     */
    public String createRefund(String tradeNo, long amount, String reason) {
        return createRefund(tradeNo, amount, reason, null);
    }

    /**
     * 建立退款（帶冪等鍵版本）。
     *
     * @param idempotencyKey 冪等鍵（對應 proto {@code refund_no} 欄位）；
     *                        同 key 重複呼叫 Payment 側 {@code findByIdempotencyKey} 會回既有 row，避免 orphan。
     * @return refundNo Payment 側產生或既有的退款編號
     */
    public String createRefund(String tradeNo, long amount, String reason, String idempotencyKey) {
        log.info("PaymentGrpcClient.createRefund.start",
                "建立退款 tradeNo=" + tradeNo + " amount=" + amount + " idempotencyKey=" + idempotencyKey);
        CreateRefundRequest.Builder builder = CreateRefundRequest.newBuilder()
                .setTradeNo(tradeNo)
                .setAmount(String.valueOf(amount))
                .setReason(reason);
        if (idempotencyKey != null && !idempotencyKey.isBlank()) {
            builder.setRefundNo(idempotencyKey);
        }
        // envelope-code 檢查（依《Java gRPC 錯誤處理規範》§7.0 現行約定）：
        // payment 業務失敗（PayUni 拒退 RefundStatus.FAILED 等）以 envelope code≠0 回傳（onNext，非
        // StatusRuntimeException），必須檢查。失敗拋 IllegalStateException，由 caller 既有 catch 處理：
        // AutoRefundWorker → updateFailed → retry → MANUAL_INTERVENTION（不標 DONE）；
        // PaymentEventHandler ATM 預建 → log + 不中斷主流程（#1150 同類：原本不檢查 code 直接取
        // getData().getRefundNo()，FAILED 退款被當成功標 task DONE，買家錢未退）。
        CreateRefundResponse response = paymentStub
                .withDeadlineAfter(MONEY_MOVEMENT_DEADLINE_SEC, TimeUnit.SECONDS)
                .createRefund(builder.build());
        if (response.getCode() != GrpcResponseCode.SUCCESS.code()) {
            throw new IllegalStateException("createRefund 失敗: code=" + response.getCode()
                    + ", message=" + response.getMessage() + ", tradeNo=" + tradeNo);
        }
        log.info("PaymentGrpcClient.createRefund.done", "退款回應 ok",
                "tradeNo", tradeNo);
        return response.getData().getRefundNo();
    }

    /**
     * 依訂單編號關閉付款交易（訂單取消時呼叫，best-effort）
     */
    public void closePaymentByOrderNo(String orderNo) {
        log.info("PaymentGrpcClient.closePaymentByOrderNo.start", "關閉付款交易", "orderNo", orderNo);
        ClosePaymentByOrderNoRequest request = ClosePaymentByOrderNoRequest.newBuilder()
                .setOrderNo(orderNo)
                .build();
        ClosePaymentByOrderNoResponse response = paymentStub
                .withDeadlineAfter(INTERNAL_READ_DEADLINE_SEC, TimeUnit.SECONDS)
                .closePaymentByOrderNo(request);
        log.info("PaymentGrpcClient.closePaymentByOrderNo.done", "關閉付款交易回應 code=" + response.getCode(), "orderNo", orderNo);
    }

    /** 批次查 付款 provider_status / provider_message（收斂規範 §6.3） */
    public BatchGetProviderStatusByOrderIdsResponse batchGetProviderStatusByOrderIds(java.util.List<String> orderIds) {
        return paymentStub
                .withDeadlineAfter(INTERNAL_READ_DEADLINE_SEC, TimeUnit.SECONDS)
                .batchGetProviderStatusByOrderIds(
                BatchGetProviderStatusByOrderIdsRequest.newBuilder()
                        .addAllOrderIds(orderIds)
                        .build());
    }

    /** 批次查 退款 provider_status / provider_message by order ids（收斂規範 §6.3） */
    public BatchGetRefundProviderStatusResponse batchGetRefundProviderStatusByOrderIds(java.util.List<String> orderIds) {
        return paymentStub
                .withDeadlineAfter(INTERNAL_READ_DEADLINE_SEC, TimeUnit.SECONDS)
                .batchGetRefundProviderStatusByOrderIds(
                BatchGetRefundProviderStatusByOrderIdsRequest.newBuilder()
                        .addAllOrderIds(orderIds)
                        .build());
    }
}
