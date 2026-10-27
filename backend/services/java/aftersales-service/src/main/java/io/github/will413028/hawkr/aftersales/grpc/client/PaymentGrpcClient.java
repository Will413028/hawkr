package io.github.will413028.hawkr.aftersales.grpc.client;

import io.github.will413028.hawkr.common.obs.grpc.GrpcStubFactory;
import io.github.will413028.hawkr.common.obs.log.ObsLogger;
import io.github.will413028.hawkr.payment.grpc.*;
import org.springframework.stereotype.Component;

import java.util.concurrent.TimeUnit;

@Component
public class PaymentGrpcClient {

    private static final ObsLogger log = ObsLogger.forClass(PaymentGrpcClient.class);

    /**
     * {@code CreateRefund} 的 gRPC deadline（秒）——issue #2294 P1-1。
     *
     * <p><b>為什麼非有不可</b>：{@code GrpcStubFactory} / {@code GrpcChannelRegistry}
     * （observability-common）全域<b>零</b> deadline 設定，而 {@code CreateRefund} 是同步打 PayUni 的
     * RPC ⇒ 沒有 deadline 就是「單筆最壞耗時無上界」。{@code RefundTaskWorker} 的租約下界
     * （《多副本排程搶佔規範》§三第 5 步：<b>租約長度必須 &gt; 單筆工作的最壞耗時</b>）因此無從建立。
     *
     * <p><b>100 秒是怎麼推出來的（下界推導，取最大者）</b>：
     * <ol>
     *   <li>payment-service 的 {@code CreateRefund} 同步呼叫 PayUni
     *       （{@code PaymentGrpcService.createRefund} → {@code refundService.createRefund}）。</li>
     *   <li>PayUni 往返上界由共用 HTTP client 決定：{@code PayUniHttpClient} 的 OkHttp 設定為
     *       connect 30s ＋ write 30s ＋ read 30s（{@code payuni-java-client/.../PayUniHttpClient.java:42-44}），
     *       三段各自獨立計時 ⇒ 單次交換上界 <b>90 秒</b>。</li>
     *   <li>再加 payment 端本地 DB 寫入與 gRPC 傳輸的餘裕 10 秒 ⇒ <b>100 秒</b>。</li>
     * </ol>
     *
     * <p><b>⚠️ 這個值刻意取大，不可「保守地」調小</b>——「調小比較安全」在這條路徑上是反的。
     * deadline 若小於 PayUni 的最壞往返，一筆<b>其實已在 PayUni 端成功</b>的退款會在我方變成
     * {@code DEADLINE_EXCEEDED} ⇒ 走 {@code RefundTaskWorker.handleRefundFailure}
     * ⇒ <b>{@code retry_count++}</b> ⇒ 下一輪的 {@code refund_no = case_no-retryCount}
     * 是<b>另一把冪等鍵</b>（payment 側 {@code idx_payment_refunds_idempotency_key} 認的是
     * {@code refund_no}）⇒ 冪等鍵這道防線<b>當場失效</b>。
     *
     * <p><b>失效之後真正的後果分兩種情形</b>（issue #2294 P1-D：先前只寫了第二種，
     * 那個敘述對全額退款是<b>錯的</b>；結論不變，但理由要準確，否則下一個人會拿一個被證偽的前提
     * 來調這個數字）。換鍵之後擋在前面的是 payment 側<b>與冪等鍵無關</b>的兩道防線：
     * {@code RefundApplicationService} 的 {@code hasProcessingRefund}（status IN {@code '0','1'}）
     * 與 {@code sumRefundedAmountByTradeNo}（status IN {@code '0','1','2'}）→
     * {@code RefundDomainService.validateRefundAmount}：
     * <ol>
     *   <li><b>全額退款（本服務的主要形態）</b>：第一筆已把可退額度佔滿 ⇒ 重試被
     *       「退款金額超過可退款餘額」擋下 ⇒ <b>不會退第二次錢</b>。
     *       但壞法換成另一種、且同樣嚴重：PayUni 已實際退款，我方 {@code refund_tasks} 卻走到終態
     *       {@code FAILED}、case 停在 {@code 40} <b>永不 CLOSED</b>、
     *       {@code aftersales.refund.completed} <b>永不發出</b>（券／庫存／訂單狀態全部不同步），
     *       而 {@code last_error} 記的是<b>誤導性的金額錯誤</b>，讓事後查修的人往錯的方向找。</li>
     *   <li><b>部分退款</b>（驗貨扣款等，交易剩餘可退額度仍足夠）：兩道防線都放行
     *       ⇒ <b>真的退第二次錢</b>。</li>
     * </ol>
     * （第一筆若當下尚未終結，{@code hasProcessingRefund} 會先以
     * {@code REFUND_ALREADY_PROCESSING} 擋下，結局同第 1 種。）
     *
     * <p>對照：租約到期被第二個 pod 重搶時 {@code retry_count} <b>未變</b> ⇒ 同一把冪等鍵
     * ⇒ 命中冪等索引、回傳既有退款，兩種情形都不會發生。
     * 兩者的差別全在 {@code retry_count} 有沒有被推進。
     */
    public static final long CREATE_REFUND_DEADLINE_SEC = 100L;

    /**
     * 本 client <b>其餘</b> RPC 的 gRPC deadline（秒）——issue #3332。
     *
     * <p><b>為什麼不是 {@link #CREATE_REFUND_DEADLINE_SEC} 的 100 秒</b>：這幾支的名字
     * （{@code decryptPayUniResult} / {@code getRefundPageUrl} /
     * {@code ackAtmRefundAccepted} / {@code batchGetRefundProviderStatusByCaseIds}）
     * 看起來像是背後會打 PayUni，實際<b>逐支追進 payment-service 後確認都不會</b>：
     * <ul>
     *   <li>{@code decryptPayUniResult} → {@code cryptoService.verifyHash/decrypt}，純本地 AES／SHA 運算。</li>
     *   <li>{@code getRefundPageUrl} → DB 查原交易 ＋ {@code PaymentClient.offlineRefundPage}
     *       （只做 {@code crypto.encrypt} 組 {@code UppFormData}，<b>不</b>呼叫 {@code httpClient.callApi}；
     *       實際 POST 給 PayUni 的是買家瀏覽器）＋ Redis 存表單換 opaque id。</li>
     *   <li>{@code ackAtmRefundAccepted} → {@code refundRepository.markAtmRefundAccepted}，單句 DB update。</li>
     *   <li>{@code batchGetRefundProviderStatusByCaseIds} / {@code getRefund} → 純 DB 讀
     *       （{@code provider_status} 是既有欄位，不是即時向 PayUni 查詢）。</li>
     * </ul>
     * ⇒ 最壞耗時是本地 DB／Redis 等級，不受 PayUni 的 90 秒往返上界拘束，故取內部呼叫慣例 10 秒。
     *
     * <p>⚠️ 與 {@link #CREATE_REFUND_DEADLINE_SEC} 同樣<b>必須在每次呼叫時</b>套用
     * （{@code withDeadlineAfter} 換算的是絕對時間），且<b>不得</b>改共用的 {@code GrpcStubFactory}。
     */
    public static final long INTERNAL_DEADLINE_SEC = 10L;

    private final GrpcStubFactory grpcStubFactory;

    public PaymentGrpcClient(GrpcStubFactory grpcStubFactory) {
        this.grpcStubFactory = grpcStubFactory;
    }

    public CreateRefundResponse createRefund(String tradeNo, String amount, String reason, String refundNo) {
        log.info("PaymentGrpcClient.createRefund.start", "執行退款", "tradeNo", tradeNo);
        // deadline 加在**呼叫點**而非 GrpcStubFactory：後者是 observability-common 的共用元件，
        // 動它會一次改掉全 repo 所有 Java gRPC 呼叫的逾時語意（遠超本 issue 範圍）。
        var stub = grpcStubFactory.blocking("payment", PaymentServiceGrpc::newBlockingStub)
                .withDeadlineAfter(CREATE_REFUND_DEADLINE_SEC, TimeUnit.SECONDS);
        var resp = stub.createRefund(
            CreateRefundRequest.newBuilder()
                .setTradeNo(tradeNo)
                .setAmount(amount)
                .setReason(reason)
                .setRefundNo(refundNo)
                .build()
        );
        log.info("PaymentGrpcClient.createRefund.done", "退款完成, code=" + resp.getCode(), "tradeNo", tradeNo);
        return resp;
    }

    public GetRefundPageUrlResponse getRefundPageUrl(String tradeNo, String amount, String returnUrl) {
        log.info("PaymentGrpcClient.getRefundPageUrl.start", "取得退款頁面 URL", "tradeNo", tradeNo);
        var stub = grpcStubFactory.blocking("payment", PaymentServiceGrpc::newBlockingStub)
                .withDeadlineAfter(INTERNAL_DEADLINE_SEC, TimeUnit.SECONDS);
        var resp = stub.getRefundPageUrl(
            GetRefundPageUrlRequest.newBuilder()
                .setTradeNo(tradeNo)
                .setAmount(amount)
                .setReturnUrl(returnUrl)
                .build()
        );
        log.info("PaymentGrpcClient.getRefundPageUrl.done", "取得退款頁面 URL 完成, code=" + resp.getCode(), "tradeNo", tradeNo);
        return resp;
    }

    public DecryptPayUniResultResponse decryptPayUniResult(String encryptInfo, String hashInfo) {
        log.info("PaymentGrpcClient.decryptPayUniResult.start", "解密 PayUni 加密資料");
        var stub = grpcStubFactory.blocking("payment", PaymentServiceGrpc::newBlockingStub)
                .withDeadlineAfter(INTERNAL_DEADLINE_SEC, TimeUnit.SECONDS);
        var resp = stub.decryptPayUniResult(
            DecryptPayUniResultRequest.newBuilder()
                .setEncryptInfo(encryptInfo)
                .setHashInfo(hashInfo)
                .build()
        );
        log.info("PaymentGrpcClient.decryptPayUniResult.done", "解密完成, code=" + resp.getCode());
        return resp;
    }

    public GetRefundResponse getRefund(String refundNo) {
        var stub = grpcStubFactory.blocking("payment", PaymentServiceGrpc::newBlockingStub)
                .withDeadlineAfter(INTERNAL_DEADLINE_SEC, TimeUnit.SECONDS);
        return stub.getRefund(GetRefundRequest.newBuilder().setRefundNo(refundNo).build());
    }

    /** 批次查詢退款 provider 狀態 by case_ids（§29 AfterSales Admin list 用）。 */
    public BatchGetRefundProviderStatusResponse batchGetRefundProviderStatusByCaseIds(java.util.List<String> caseIds) {
        var stub = grpcStubFactory.blocking("payment", PaymentServiceGrpc::newBlockingStub)
                .withDeadlineAfter(INTERNAL_DEADLINE_SEC, TimeUnit.SECONDS);
        return stub.batchGetRefundProviderStatusByCaseIds(
                BatchGetRefundProviderStatusByCaseIdsRequest.newBuilder()
                        .addAllCaseIds(caseIds)
                        .build());
    }

    /** ATM 退款頁面受理成功（sync 階段，收斂規範 §4.4）：Payment 側 PROCESSING + estimatedRefundDate 落地。 */
    public AckAtmRefundAcceptedResponse ackAtmRefundAccepted(String refundNo, String providerStatus,
                                                              String providerMessage,
                                                              String estimatedRefundDate) {
        log.info("PaymentGrpcClient.ackAtmRefundAccepted.start",
                "通知 Payment ATM 退款受理", "refundNo", refundNo);
        var stub = grpcStubFactory.blocking("payment", PaymentServiceGrpc::newBlockingStub)
                .withDeadlineAfter(INTERNAL_DEADLINE_SEC, TimeUnit.SECONDS);
        var resp = stub.ackAtmRefundAccepted(
            AckAtmRefundAcceptedRequest.newBuilder()
                .setRefundNo(refundNo == null ? "" : refundNo)
                .setProviderStatus(providerStatus == null ? "" : providerStatus)
                .setProviderMessage(providerMessage == null ? "" : providerMessage)
                .setEstimatedRefundDate(estimatedRefundDate == null ? "" : estimatedRefundDate)
                .build()
        );
        log.info("PaymentGrpcClient.ackAtmRefundAccepted.done",
                "通知完成 code=" + resp.getCode(), "refundNo", refundNo);
        return resp;
    }
}
