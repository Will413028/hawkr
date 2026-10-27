package io.github.will413028.hawkr.payment.adapter.grpc;

import io.github.will413028.hawkr.common.grpc.GrpcAuthContext;
import io.github.will413028.hawkr.common.obs.grpc.GrpcErrorHandler;
import io.github.will413028.hawkr.common.grpc.GrpcServiceHelper;
import io.github.will413028.hawkr.common.obs.log.ObsLogger;
import io.github.will413028.hawkr.common.utils.code.AppType;
import io.github.will413028.hawkr.common.utils.code.ClientType;
import io.github.will413028.hawkr.common.utils.code.GrpcResponseCode;
import io.github.will413028.hawkr.common.utils.proto.ProtoConversionUtils;
import io.github.will413028.hawkr.common.utils.code.PaymentMethod;
import io.github.will413028.hawkr.common.utils.code.PaymentTransactionStatus;
import io.github.will413028.hawkr.common.utils.code.RefundStatus;
import io.github.will413028.hawkr.common.utils.code.AccountType;
import io.github.will413028.hawkr.common.utils.code.helper.AccountTypeRules;
import io.github.will413028.hawkr.payment.domain.model.PaymentTransaction;
import io.github.will413028.hawkr.payment.domain.model.RefundTransaction;
import io.github.will413028.hawkr.payment.infrastructure.persistence.entity.PaymentLogEntity;
import io.github.will413028.hawkr.payment.infrastructure.persistence.mapper.PaymentLogMapper;
import io.github.will413028.hawkr.payment.constant.PayUniConstants;
import io.github.will413028.hawkr.payment.exception.PaymentBusinessException;
import io.github.will413028.hawkr.payment.application.dto.GrantPaymentResult;
import io.github.will413028.hawkr.payment.application.dto.PaymentDetailResponse;
import io.github.will413028.hawkr.payment.application.dto.RefundDetailResponse;
import io.github.will413028.hawkr.payment.application.service.PaymentApplicationService;
import io.github.will413028.hawkr.payment.application.service.PaymentCloseService;
import io.github.will413028.hawkr.payment.application.service.PaymentPageService;
import io.github.will413028.hawkr.payment.application.service.PaymentRedirectFormStore;
import io.github.will413028.hawkr.payment.application.service.PaymentResultDisplay;
import io.github.will413028.hawkr.payment.application.service.RefundApplicationService;
import io.github.will413028.hawkr.payment.grpc.ClosePaymentByOrderNoRequest;
import io.github.will413028.hawkr.payment.grpc.ClosePaymentByOrderNoResponse;
import io.github.will413028.hawkr.payment.grpc.GetActivePaymentStatusByOrderNoRequest;
import io.github.will413028.hawkr.payment.grpc.GetActivePaymentStatusByOrderNoResponse;
import io.github.will413028.hawkr.payment.grpc.GetPaymentFactsInternalRequest;
import io.github.will413028.hawkr.payment.grpc.GetPaymentFactsInternalResponse;
import io.github.will413028.hawkr.payment.grpc.PaymentFactsData;
import io.github.will413028.hawkr.payment.grpc.ClosePaymentRequest;
import io.github.will413028.hawkr.payment.grpc.ClosePaymentResponse;
import io.github.will413028.hawkr.payment.grpc.CreatePaymentRequest;
import io.github.will413028.hawkr.payment.grpc.CreatePaymentResponse;
import io.github.will413028.hawkr.payment.grpc.GetPaymentHistoryRequest;
import io.github.will413028.hawkr.payment.grpc.GetPaymentHistoryResponse;
import io.github.will413028.hawkr.payment.grpc.GetSellerPaymentHistoryRequest;
import io.github.will413028.hawkr.payment.grpc.GetSellerPaymentHistoryResponse;
import io.github.will413028.hawkr.payment.grpc.GetPaymentRequest;
import io.github.will413028.hawkr.payment.grpc.GetPaymentResponse;
import io.github.will413028.hawkr.payment.grpc.GetPaymentRedirectFormRequest;
import io.github.will413028.hawkr.payment.grpc.GetPaymentRedirectFormResponse;
import io.github.will413028.hawkr.payment.grpc.GetPaymentReturnPageRequest;
import io.github.will413028.hawkr.payment.grpc.GetPaymentReturnPageResponse;
import io.github.will413028.hawkr.payment.grpc.GetPublicPaymentStatusRequest;
import io.github.will413028.hawkr.payment.grpc.GetPublicPaymentStatusResponse;
import io.github.will413028.hawkr.payment.grpc.PublicPaymentStatus;
import io.github.will413028.hawkr.payment.grpc.GetRefundRequest;
import io.github.will413028.hawkr.payment.grpc.GetRefundResponse;
import io.github.will413028.hawkr.payment.grpc.GetRefundsByTradeNoRequest;
import io.github.will413028.hawkr.payment.grpc.GetRefundsByTradeNoResponse;
import io.github.will413028.hawkr.payment.grpc.PaymentDetail;
import io.github.will413028.hawkr.payment.grpc.PaymentResult;
import io.github.will413028.hawkr.payment.grpc.PaymentServiceGrpc;

import io.github.will413028.hawkr.payment.grpc.GetRefundPageUrlRequest;
import io.github.will413028.hawkr.payment.grpc.GetRefundPageUrlResponse;
import io.github.will413028.hawkr.payment.grpc.RefundDetail;
import io.github.will413028.hawkr.payment.grpc.RefundPageData;
import io.github.will413028.hawkr.payment.grpc.SyncPaymentStatusRequest;
import io.github.will413028.hawkr.payment.grpc.SyncPaymentStatusResponse;
import io.github.will413028.hawkr.payment.grpc.ValidateApplePayMerchantRequest;
import io.github.will413028.hawkr.payment.grpc.ValidateApplePayMerchantResponse;
import io.github.will413028.hawkr.payment.application.service.ApplePayMerchantValidator;

import io.github.will413028.hawkr.common.utils.code.WebhookProcessStatus;
import io.github.will413028.hawkr.common.utils.code.WebhookProvider;
import io.github.will413028.hawkr.payment.adapter.webhook.handler.PaymentNotifyHandler;
import io.github.will413028.hawkr.payment.adapter.webhook.handler.WebhookHandleResult;
import io.github.will413028.hawkr.payment.grpc.AckAtmRefundAcceptedRequest;
import io.github.will413028.hawkr.payment.grpc.AckAtmRefundAcceptedResponse;
import io.github.will413028.hawkr.payment.grpc.DecryptPayUniResultRequest;
import io.github.will413028.hawkr.payment.grpc.DecryptPayUniResultResponse;
import io.github.will413028.hawkr.payment.grpc.DecryptedPayUniData;
import io.github.will413028.hawkr.payment.grpc.HandlePaymentPageReturnRequest;
import io.github.will413028.hawkr.payment.grpc.HandlePaymentPageReturnResponse;
import io.github.will413028.hawkr.payment.grpc.BatchGetProviderStatusByOrderIdsRequest;
import io.github.will413028.hawkr.payment.grpc.BatchGetProviderStatusByOrderIdsResponse;
import io.github.will413028.hawkr.payment.grpc.BatchGetRefundProviderStatusByCaseIdsRequest;
import io.github.will413028.hawkr.payment.grpc.BatchGetRefundProviderStatusByOrderIdsRequest;
import io.github.will413028.hawkr.payment.grpc.BatchGetRefundProviderStatusResponse;
import io.github.will413028.hawkr.payment.grpc.ProviderStatusEntry;
import io.github.will413028.hawkr.payment.domain.repository.PaymentTransactionRepository;
import io.github.will413028.hawkr.payment.domain.repository.RefundTransactionRepository;
import io.github.will413028.hawkr.payuni.crypto.PayUniCryptoService;
import io.github.will413028.hawkr.payment.infrastructure.persistence.entity.PaymentWebhookEntity;
import io.github.will413028.hawkr.payment.infrastructure.persistence.mapper.PaymentWebhookMapper;
import com.fasterxml.jackson.databind.ObjectMapper;
import io.grpc.Status;
import io.grpc.StatusRuntimeException;
import io.grpc.stub.StreamObserver;
import net.devh.boot.grpc.server.service.GrpcService;
import org.slf4j.MDC;
import org.springframework.beans.factory.annotation.Value;
import org.springframework.transaction.support.TransactionTemplate;

import io.github.will413028.hawkr.common.utils.DateTimeUtils;

import java.net.URLEncoder;
import java.nio.charset.StandardCharsets;
import java.time.LocalDate;
import java.time.format.DateTimeFormatter;
import java.util.List;
import java.util.Map;
import java.util.Optional;

/**
 * 支付 gRPC 服務實作
 */
@GrpcService
public class PaymentGrpcService extends PaymentServiceGrpc.PaymentServiceImplBase {

    private static final ObsLogger log = ObsLogger.forClass(PaymentGrpcService.class);
    private static final String SOURCE_IP_GRPC_RETURN = "GRPC_RETURN";

    private final PaymentApplicationService paymentService;
    private final PaymentCloseService paymentCloseService;
    private final PaymentPageService pageService;
    private final RefundApplicationService refundService;
    private final PayUniCryptoService cryptoService;
    private final PaymentNotifyHandler notifyHandler;
    private final PaymentWebhookMapper webhookMapper;
    private final TransactionTemplate transactionTemplate;
    private final ObjectMapper objectMapper;
    private final io.github.will413028.hawkr.payment.infrastructure.audit.PayUniAuditLogger auditLogger;
    private final PaymentTransactionRepository transactionRepository;
    private final RefundTransactionRepository refundRepository;
    private final io.github.will413028.hawkr.payment.application.service.PaymentAdminQueryService adminQueryService;
    private final PaymentLogMapper paymentLogMapper;
    private final ApplePayMerchantValidator applePayMerchantValidator;
    private final io.github.will413028.hawkr.payment.agentfee.AgentFeeService agentFeeService;

    @Value("${hawkr.web.frontend-url}")
    private String webFrontendUrl;

    public PaymentGrpcService(
            PaymentApplicationService paymentService,
            PaymentCloseService paymentCloseService,
            PaymentPageService pageService,
            RefundApplicationService refundService,
            PayUniCryptoService cryptoService,
            PaymentNotifyHandler notifyHandler,
            PaymentWebhookMapper webhookMapper,
            TransactionTemplate transactionTemplate,
            ObjectMapper objectMapper,
            io.github.will413028.hawkr.payment.infrastructure.audit.PayUniAuditLogger auditLogger,
            PaymentTransactionRepository transactionRepository,
            RefundTransactionRepository refundRepository,
            io.github.will413028.hawkr.payment.application.service.PaymentAdminQueryService adminQueryService,
            PaymentLogMapper paymentLogMapper,
            ApplePayMerchantValidator applePayMerchantValidator,
            io.github.will413028.hawkr.payment.agentfee.AgentFeeService agentFeeService
    ) {
        this.paymentService = paymentService;
        this.paymentCloseService = paymentCloseService;
        this.pageService = pageService;
        this.refundService = refundService;
        this.cryptoService = cryptoService;
        this.notifyHandler = notifyHandler;
        this.webhookMapper = webhookMapper;
        this.transactionTemplate = transactionTemplate;
        this.objectMapper = objectMapper;
        this.auditLogger = auditLogger;
        this.transactionRepository = transactionRepository;
        this.refundRepository = refundRepository;
        this.adminQueryService = adminQueryService;
        this.paymentLogMapper = paymentLogMapper;
        this.applePayMerchantValidator = applePayMerchantValidator;
        this.agentFeeService = agentFeeService;
    }

    // ==================== 授權輔助方法 ====================

    /**
     * 取得呼叫端在「買家角色」下的主體 ID。
     *
     * <p><b>這個值不是 JWT 的 raw account_id。</b>payment-service 的買家歸屬檢查比對的是
     * {@code payment_transactions.buyer_id}——一個每段買家 stint 重新以 TSID 產生、
     * 與 account_id 永遠沒有數值關係的代理鍵（buyer-service 一律 {@code m.BuyerID = u.tsid()}，
     * 從不複製 account_id）。因此在這條路徑上，x-account-id metadata 承載的必須是
     * <b>呼叫端已解析的現任 buyer_id</b>：
     * <ul>
     *   <li>Gateway：{@code gateway/internal/handler/payment.go} 的
     *       {@code resolvePaymentBuyerID} 解析後，由
     *       {@code gateway/internal/grpc/payment_client.go} 的 {@code ctxWithAccount}
     *       覆寫 metadata（該檔註解另有完整說明）。</li>
     *   <li>Java 服務轉呼叫：必須用
     *       {@code io.grpc.Context.withValue(GrpcAuthContext.ACCOUNT_ID, buyerId)}
     *       覆寫 ambient context（見 order-service 的
     *       {@code CartGrpcClient.callAsAccount}）。
     *       直接用 stub 呼叫會讓全域掛載的 {@code GrpcAuthClientInterceptor} 把該服務
     *       自己收到的 raw account_id 原封傳下來。</li>
     * </ul>
     *
     * <p>違反契約的後果是<b>靜默降級</b>而非爆炸：比對必然不相等，回應是與「交易不存在」
     * 刻意無法區分的 NOT_FOUND（防枚舉，見 {@link #verifyPaymentOwnership}），而且只有
     * buyer_id 已分岔的帳號會中招——所以只會在 production 被發現。cart-service 的
     * {@code CHECKOUT_LOCK_FAILED} (6005) 事故就是同一個形狀。下面兩個 verify 方法的
     * {@code buyerMismatch} WARN log 是目前唯一的偵測手段。
     *
     * <p>刻意保留為一行 delegate 而非包成 wrapper type：產生這個值的
     * {@code GrpcAuthServerInterceptor} 在 parse header 的當下無從得知它承載的是
     * buyer_id、store_id 還是 raw account_id（cart-service 在同一個 channel 上裝的就是
     * buyer_id），包型別只會得到一個不誠實的標籤。這個具名 accessor 的作用是讓
     * 呼叫端讀到的是「buyer id」而不是「account id」，契約寫在它的 javadoc 裡。
     *
     * @return 呼叫端的現任 buyer_id；內部呼叫且未傳播身分時為 0
     */
    private static long requireCallerBuyerId() {
        return GrpcAuthContext.requireAccountId();
    }

    /**
     * 驗證支付交易擁有權（buyer-only）。
     * - BUYER：txn.buyerId 必須匹配呼叫者的已解析 buyer_id（見 {@link #requireCallerBuyerId()}）
     * - SELLER：#2501 起所有呼叫本方法的 RPC（getPayment / getRefund / getRefundsByTradeNo /
     *   syncPaymentStatus / closePayment）都已收斂為 requireAccountType(BUYER)，賣家無法到達這裡；
     *   賣家的付款可見度改由 {@link #getSellerPaymentHistory} 以 store-scope + 遮蔽提供。
     *
     * @return 查詢到的交易詳情
     */
    private PaymentDetailResponse verifyPaymentOwnership(String tradeNo) {
        long callerBuyerId = requireCallerBuyerId();
        AccountType callerType = GrpcAuthContext.getAccountType();
        boolean buyerScoped = AccountTypeRules.covers(callerType, AccountType.BUYER);

        // 契約違反（不是越權）：呼叫端宣告自己是買家角色，卻沒有傳播已解析的 buyer_id。
        // 不能沿用防枚舉的 NOT_FOUND——那會讓「呼叫端接錯線」與「交易真的不存在」永遠
        // 無法區分，正是本 bug class 只在 production 才被發現的原因。這裡刻意 fail loud。
        if (buyerScoped && callerBuyerId <= 0) {
            log.warn("PaymentGrpcService.verifyPaymentOwnership.missingBuyerId",
                    "買家角色呼叫但未傳播 buyer_id（x-account-id 缺漏或為 0），無法執行歸屬檢查",
                    "tradeNo", tradeNo);
            throw Status.UNAUTHENTICATED.withDescription("請先登入").asRuntimeException();
        }

        Optional<PaymentDetailResponse> opt = paymentService.getPaymentDetail(tradeNo);
        if (opt.isEmpty()) {
            throw Status.NOT_FOUND.withDescription("交易不存在").asRuntimeException();
        }

        PaymentDetailResponse txn = opt.get();
        if (buyerScoped) {
            String callerBuyerIdStr = String.valueOf(callerBuyerId);
            if (!callerBuyerIdStr.equals(txn.buyerId())) {
                // 越權防枚舉（API 設計規範 §14.1）：回應與「交易不存在」一致。
                // 回應不能透露差異，但 server-side log 可以——這是分辨「真的越權」與
                // 「呼叫端傳了 raw account_id」的唯一線索。
                log.warn("PaymentGrpcService.verifyPaymentOwnership.buyerMismatch",
                        "買家歸屬不匹配（越權，或呼叫端傳了 raw account_id 而非已解析 buyer_id）"
                                + " callerBuyerId=" + callerBuyerIdStr + " txnBuyerId=" + txn.buyerId(),
                        "tradeNo", tradeNo);
                throw Status.NOT_FOUND.withDescription("交易不存在").asRuntimeException();
            }
        }
        // #2501：SELLER 已無法到達此方法（所有呼叫端收斂為 buyer-only）；此 fall-through 為縱深防禦。
        return txn;
    }

    /**
     * 依 order_id 驗證歸屬（用於 GetPaymentHistory，查該訂單任一筆 row 來判定）。
     * 買家側比對語意與 {@link #verifyPaymentOwnership} 相同，見 {@link #requireCallerBuyerId()} 的契約。
     */
    private void verifyPaymentOwnershipByOrderId(String orderId) {
        long callerBuyerId = requireCallerBuyerId();
        AccountType callerType = GrpcAuthContext.getAccountType();
        boolean buyerScoped = AccountTypeRules.covers(callerType, AccountType.BUYER);

        if (buyerScoped && callerBuyerId <= 0) {
            log.warn("PaymentGrpcService.verifyPaymentOwnershipByOrderId.missingBuyerId",
                    "買家角色呼叫但未傳播 buyer_id（x-account-id 缺漏或為 0），無法執行歸屬檢查",
                    "orderId", orderId);
            throw Status.UNAUTHENTICATED.withDescription("請先登入").asRuntimeException();
        }

        java.util.List<PaymentDetailResponse> history = paymentService.getPaymentHistory(orderId);
        if (history.isEmpty()) {
            throw Status.NOT_FOUND.withDescription("訂單無付款記錄").asRuntimeException();
        }

        if (buyerScoped) {
            String callerBuyerIdStr = String.valueOf(callerBuyerId);
            boolean owned = history.stream().anyMatch(r -> callerBuyerIdStr.equals(r.buyerId()));
            if (!owned) {
                // 越權防枚舉（API 設計規範 §14.1）：回應與「訂單無付款記錄」一致
                log.warn("PaymentGrpcService.verifyPaymentOwnershipByOrderId.buyerMismatch",
                        "買家歸屬不匹配（越權，或呼叫端傳了 raw account_id 而非已解析 buyer_id）"
                                + " callerBuyerId=" + callerBuyerIdStr
                                + " txnBuyerIds=" + history.stream()
                                        .map(PaymentDetailResponse::buyerId).distinct().toList(),
                        "orderId", orderId);
                throw Status.NOT_FOUND.withDescription("訂單無付款記錄").asRuntimeException();
            }
        }
    }

    // ==================== gRPC 方法實作 ====================

    @Override
    public void createPayment(CreatePaymentRequest request, StreamObserver<CreatePaymentResponse> responseObserver) {
        // #1871 OBS-09：order_no 進 MDC，讓這個方法內所有 log 行都能被 Loki structured
        // metadata 查到（見 k8s/monitoring/promtail-daemonset.yaml）。
        MDC.put("order_no", request.getOrderNo());
        try {
            // 授權：僅 BUYER 可建立支付
            GrpcAuthContext.requireAccountType(AccountType.BUYER);

            io.github.will413028.hawkr.payment.application.dto.CreatePaymentResponse dtoResponse = paymentService.createPayment(
                    new io.github.will413028.hawkr.payment.application.dto.CreatePaymentRequest(
                            request.getOrderNo(),
                            request.getBuyerId(),  // int64，Gateway 已解析為現任 buyer_id 後注入
                            toPaymentMethod(request.getPaymentType()),
                            parseExpireDays(request.getExpireDays()),
                            request.getApplePayToken().isEmpty() ? null : request.getApplePayToken(),
                            request.getGooglePayToken().isEmpty() ? null : request.getGooglePayToken(),
                            request.getBankType().isEmpty() ? null : request.getBankType(),
                            request.getUserEmail().isEmpty() ? null : request.getUserEmail(),
                            request.getClientType().isEmpty() ? null : request.getClientType(),
                            request.getAppType().isEmpty() ? null : request.getAppType()
                    )
            );

            // 根據實際交易狀態決定返回結果
            // 幕後即時付（Apple Pay / Google Pay）會同步返回支付結果
            boolean isSuccess = dtoResponse.status() != PaymentTransactionStatus.FAILED;
            String message = buildResponseMessage(dtoResponse);

            responseObserver.onNext(CreatePaymentResponse.newBuilder()
                    .setCode(isSuccess ? GrpcResponseCode.SUCCESS.code() : GrpcResponseCode.INTERNAL_ERROR.code())
                    .setMessage(message)
                    .setData(toPaymentResult(dtoResponse))
                    .build());
            responseObserver.onCompleted();

            if (isSuccess) {
                log.info("PaymentGrpcService.createPayment.success", "建立支付成功", "trade_no", dtoResponse.tradeNo());
            } else {
                log.warn("PaymentGrpcService.createPayment.paymentFailed", "支付失敗", "trade_no", dtoResponse.tradeNo());
            }

        } catch (StatusRuntimeException e) {
            responseObserver.onNext(CreatePaymentResponse.newBuilder()
                    .setCode(mapStatusCode(e))
                    .setMessage(e.getStatus().getDescription())
                    .build());
            responseObserver.onCompleted();
        } catch (IllegalStateException e) {
            // 重複付款或訂單狀態不允許
            String msg = e.getMessage();
            boolean isDuplicate = msg != null && msg.contains("已付款");
            int code = isDuplicate ? GrpcResponseCode.CONFLICT.code() : GrpcResponseCode.INVALID_PARAM.code();
            String responseMsg = isDuplicate ? "重複付款" : msg;
            log.warn("PaymentGrpcService.createPayment.rejected", responseMsg + ": " + msg);
            responseObserver.onNext(CreatePaymentResponse.newBuilder()
                    .setCode(code)
                    .setMessage(responseMsg)
                    .build());
            responseObserver.onCompleted();
        } catch (org.springframework.dao.DataIntegrityViolationException e) {
            // §2A.3.6 併發安全 L3：trade_no UNIQUE 或 partial unique index 攔截
            // transaction 已 abort，在 transaction 外重新查詢回冪等結果
            log.warn("PaymentGrpcService.createPayment.concurrentRetry",
                    "併發 INSERT 衝突，回退冪等查詢 order_no=" + request.getOrderNo());
            try {
                io.github.will413028.hawkr.payment.application.dto.CreatePaymentResponse retryResp =
                        paymentService.createPayment(
                                new io.github.will413028.hawkr.payment.application.dto.CreatePaymentRequest(
                                        request.getOrderNo(),
                                        request.getBuyerId(),
                                        toPaymentMethod(request.getPaymentType()),
                                        parseExpireDays(request.getExpireDays()),
                                        request.getApplePayToken().isEmpty() ? null : request.getApplePayToken(),
                                        request.getGooglePayToken().isEmpty() ? null : request.getGooglePayToken(),
                                        request.getBankType().isEmpty() ? null : request.getBankType(),
                                        request.getUserEmail().isEmpty() ? null : request.getUserEmail(),
                                        request.getClientType().isEmpty() ? null : request.getClientType(),
                                        request.getAppType().isEmpty() ? null : request.getAppType()
                                )
                        );
                responseObserver.onNext(CreatePaymentResponse.newBuilder()
                        .setCode(GrpcResponseCode.SUCCESS.code())
                        .setMessage("建立成功")
                        .setData(toPaymentResult(retryResp))
                        .build());
                responseObserver.onCompleted();
            } catch (Exception retryEx) {
                log.error("PaymentGrpcService.createPayment.concurrentRetryFailed",
                        "併發重試也失敗: " + retryEx.getMessage());
                responseObserver.onNext(CreatePaymentResponse.newBuilder()
                        .setCode(GrpcResponseCode.CONFLICT.code())
                        .setMessage(GrpcResponseCode.PAYMENT_ALREADY_EXISTS.message())
                        .build());
                responseObserver.onCompleted();
            }
        } catch (Exception e) {
            GrpcErrorHandler.handleException(responseObserver, CreatePaymentResponse.newBuilder(), e, "PaymentGrpcService", "createPayment");
        } finally {
            MDC.remove("order_no");
        }
    }

    @Override
    public void getPayment(GetPaymentRequest request, StreamObserver<GetPaymentResponse> responseObserver) {
        try {
            // 授權：buyer-only（#2501 收斂）。單筆付款查詢含買家 PII，賣家一律不可讀；
            // 賣家的付款可見度只保留 getSellerPaymentHistory（store-scoped + 遮蔽）。
            GrpcAuthContext.requireAccountType(AccountType.BUYER);
            PaymentDetailResponse detail = verifyPaymentOwnership(request.getTradeNo());

            responseObserver.onNext(GetPaymentResponse.newBuilder()
                    .setCode(GrpcResponseCode.SUCCESS.code())
                    .setMessage(GrpcResponseCode.SUCCESS.message())
                    .setData(toPaymentDetail(detail))
                    .build());
            responseObserver.onCompleted();

        } catch (StatusRuntimeException e) {
            responseObserver.onNext(GetPaymentResponse.newBuilder()
                    .setCode(mapStatusCode(e))
                    .setMessage(e.getStatus().getDescription())
                    .build());
            responseObserver.onCompleted();
        } catch (Exception e) {
            GrpcErrorHandler.handleException(responseObserver, GetPaymentResponse.newBuilder(), e, "PaymentGrpcService", "getPayment");
        }
    }

    @Override
    public void getPaymentHistory(GetPaymentHistoryRequest request,
                                   StreamObserver<GetPaymentHistoryResponse> responseObserver) {
        try {
            // 授權：buyer-only（#2501 收斂）。賣家改走 getSellerPaymentHistory（store-scoped）。
            GrpcAuthContext.requireAccountType(AccountType.BUYER);
            // ownership check 用 order_id 的任一筆 row 驗證
            verifyPaymentOwnershipByOrderId(request.getOrderId());

            java.util.List<PaymentDetailResponse> history =
                    paymentService.getPaymentHistory(request.getOrderId());

            GetPaymentHistoryResponse.Builder builder = GetPaymentHistoryResponse.newBuilder()
                    .setCode(GrpcResponseCode.SUCCESS.code())
                    .setMessage(GrpcResponseCode.SUCCESS.message());
            for (PaymentDetailResponse row : history) {
                builder.addRecords(toPaymentDetail(row));
            }
            responseObserver.onNext(builder.build());
            responseObserver.onCompleted();

        } catch (StatusRuntimeException e) {
            responseObserver.onNext(GetPaymentHistoryResponse.newBuilder()
                    .setCode(mapStatusCode(e))
                    .setMessage(e.getStatus().getDescription())
                    .build());
            responseObserver.onCompleted();
        } catch (Exception e) {
            GrpcErrorHandler.handleException(responseObserver, GetPaymentHistoryResponse.newBuilder(), e, "PaymentGrpcService", "getPaymentHistory");
        }
    }

    /**
     * #2501 賣家視角付款歷程：賣家查詢自己店家訂單的付款歷程。
     *
     * <p><b>與買家版的差異在授權軸與可見欄位，不是「換個欄位比對」：</b>
     * <ul>
     *   <li>身分：{@code request.getStoreId()} 是 Gateway 由賣家 JWT 的 account_id 解出的現任
     *       store_id（信任 actor context），不是 client 自填。這裡仍以 canonical order 反解的
     *       store_id 再次比對，不把 request 的值當授權終點。</li>
     *   <li>歸屬：payment_transactions 不存 store_id，店家歸屬只能由 order-service 反解
     *       （{@link PaymentApplicationService#resolveOrderStoreId}）。跨店與查無訂單一律回同形
     *       NOT_FOUND「訂單無付款記錄」（防枚舉，比照買家版 verifyPaymentOwnershipByOrderId）。</li>
     *   <li>可見欄位：賣家版遮蔽 buyer_id / atm_virtual_account / provider_trade_no 等買家 PII
     *       （{@link #toSellerPaymentDetail}）；賣家只需付款狀態/方式/金額/時間來對帳。</li>
     * </ul>
     */
    @Override
    public void getSellerPaymentHistory(GetSellerPaymentHistoryRequest request,
                                        StreamObserver<GetSellerPaymentHistoryResponse> responseObserver) {
        try {
            // 授權：SELLER（涵蓋 SELLER_AND_BUYER）。純買家改走 getPaymentHistory。
            GrpcAuthContext.requireAccountType(AccountType.SELLER);

            long callerStoreId = request.getStoreId();
            // 契約違反（非越權）：賣家視角呼叫卻沒帶 Gateway 解出的 store_id。比照買家版
            // missingBuyerId，刻意 fail loud，不塌成防枚舉的 NOT_FOUND——否則 Gateway 接線
            // 錯誤只能在 production 才被發現。
            if (callerStoreId <= 0) {
                log.warn("PaymentGrpcService.getSellerPaymentHistory.missingStoreId",
                        "賣家視角呼叫但未帶已解析的 store_id（Gateway 契約違反）",
                        "orderId", request.getOrderId());
                throw Status.UNAUTHENTICATED.withDescription("請先登入").asRuntimeException();
            }

            long orderStoreId = paymentService.resolveOrderStoreId(request.getOrderId());
            if (orderStoreId <= 0 || orderStoreId != callerStoreId) {
                // 越權防枚舉（API 設計規範 §14.1）：跨店與「訂單不存在」回應必須一致，
                // 否則賣家可用 order_id 枚舉他店訂單是否存在。server-side WARN 是唯一線索。
                log.warn("PaymentGrpcService.getSellerPaymentHistory.storeMismatch",
                        "賣家店家歸屬不匹配（越權，或訂單不存在）callerStoreId=" + callerStoreId
                                + " orderStoreId=" + orderStoreId,
                        "orderId", request.getOrderId());
                throw Status.NOT_FOUND.withDescription("訂單無付款記錄").asRuntimeException();
            }

            java.util.List<PaymentDetailResponse> history =
                    paymentService.getPaymentHistory(request.getOrderId());

            GetSellerPaymentHistoryResponse.Builder builder = GetSellerPaymentHistoryResponse.newBuilder()
                    .setCode(GrpcResponseCode.SUCCESS.code())
                    .setMessage(GrpcResponseCode.SUCCESS.message());
            for (PaymentDetailResponse row : history) {
                builder.addRecords(toSellerPaymentDetail(row));
            }
            responseObserver.onNext(builder.build());
            responseObserver.onCompleted();

        } catch (StatusRuntimeException e) {
            responseObserver.onNext(GetSellerPaymentHistoryResponse.newBuilder()
                    .setCode(mapStatusCode(e))
                    .setMessage(e.getStatus().getDescription())
                    .build());
            responseObserver.onCompleted();
        } catch (Exception e) {
            GrpcErrorHandler.handleException(responseObserver, GetSellerPaymentHistoryResponse.newBuilder(), e, "PaymentGrpcService", "getSellerPaymentHistory");
        }
    }

    @Override
    public void getPublicPaymentStatus(GetPublicPaymentStatusRequest request,
                                        StreamObserver<GetPublicPaymentStatusResponse> responseObserver) {
        try {
            // 公開端點，免驗證（供付款結果回調頁面使用）
            String tradeNo = request.getTradeNo();
            if (tradeNo == null || tradeNo.isEmpty()) {
                responseObserver.onNext(GetPublicPaymentStatusResponse.newBuilder()
                        .setCode(GrpcResponseCode.INVALID_PARAM.code())
                        .setMessage(GrpcResponseCode.INVALID_PARAM.message())
                        .build());
                responseObserver.onCompleted();
                return;
            }

            Optional<PaymentDetailResponse> opt = paymentService.getPaymentDetail(tradeNo);
            if (opt.isEmpty()) {
                responseObserver.onNext(GetPublicPaymentStatusResponse.newBuilder()
                        .setCode(GrpcResponseCode.NOT_FOUND.code())
                        .setMessage(GrpcResponseCode.NOT_FOUND.message())
                        .build());
                responseObserver.onCompleted();
                return;
            }

            PaymentDetailResponse detail = opt.get();
            responseObserver.onNext(GetPublicPaymentStatusResponse.newBuilder()
                    .setCode(GrpcResponseCode.SUCCESS.code())
                    .setMessage(GrpcResponseCode.SUCCESS.message())
                    .setData(PublicPaymentStatus.newBuilder()
                            .setTradeNo(detail.tradeNo())
                            .setOrderId(detail.orderId())
                            .setAmount(formatAmount(detail.amount()))
                            .setStatus(detail.status().value())
                            .build())
                    .build());
            responseObserver.onCompleted();

        } catch (Exception e) {
            GrpcErrorHandler.handleException(responseObserver, GetPublicPaymentStatusResponse.newBuilder(), e, "PaymentGrpcService", "getPublicPaymentStatus");
        }
    }

    @Override
    public void getPaymentReturnPage(GetPaymentReturnPageRequest request,
                                      StreamObserver<GetPaymentReturnPageResponse> responseObserver) {
        try {
            // 公開端點，免驗證（PayUni 付款完成後瀏覽器導回此頁面）
            String tradeNo = request.getTradeNo();
            if (tradeNo == null || tradeNo.isEmpty()) {
                responseObserver.onNext(GetPaymentReturnPageResponse.newBuilder()
                        .setHtmlContent(pageService.renderErrorHtml("缺少交易編號"))
                        .build());
                responseObserver.onCompleted();
                return;
            }

            Optional<PaymentDetailResponse> opt = paymentService.getPaymentDetail(tradeNo);
            String htmlContent = pageService.renderPaymentReturnPage(opt.orElse(null));

            responseObserver.onNext(GetPaymentReturnPageResponse.newBuilder()
                    .setHtmlContent(htmlContent)
                    .build());
            responseObserver.onCompleted();

        } catch (Exception e) {
            log.error("PaymentGrpcService.getPaymentReturnPage.failed", "取得付款結果頁面失敗: " + e.getMessage(), e);
            responseObserver.onNext(GetPaymentReturnPageResponse.newBuilder()
                    .setHtmlContent(pageService.renderErrorHtml("系統異常，請稍後重試"))
                    .build());
            responseObserver.onCompleted();
        }
    }

    @Override
    public void getPaymentRedirectForm(GetPaymentRedirectFormRequest request,
                                        StreamObserver<GetPaymentRedirectFormResponse> responseObserver) {
        // 公開端點：付款中轉頁以 opaque id 一次性取回 canonical 表單（不可猜 + 短效 + 一次性；#1394）
        Optional<PaymentRedirectFormStore.RedirectForm> opt =
                pageService.consumeRedirectForm(request.getRedirectId());
        if (opt.isEmpty()) {
            responseObserver.onError(Status.NOT_FOUND
                    .withDescription("redirect form not found or expired")
                    .asRuntimeException());
            return;
        }

        PaymentRedirectFormStore.RedirectForm form = opt.get();
        responseObserver.onNext(GetPaymentRedirectFormResponse.newBuilder()
                .setActionUrl(nullSafe(form.actionUrl()))
                .setMerId(nullSafe(form.merId()))
                .setVersion(nullSafe(form.version()))
                .setEncryptInfo(nullSafe(form.encryptInfo()))
                .setHashInfo(nullSafe(form.hashInfo()))
                .setIsPlatform("1".equals(form.isPlatform()))
                .build());
        responseObserver.onCompleted();
    }

    @Override
    public void syncPaymentStatus(SyncPaymentStatusRequest request, StreamObserver<SyncPaymentStatusResponse> responseObserver) {
        try {
            // 授權：僅 BUYER 可同步狀態 + ownership 檢查（同步有副作用，需先驗證）
            GrpcAuthContext.requireAccountType(AccountType.BUYER);
            verifyPaymentOwnership(request.getTradeNo());

            Optional<PaymentDetailResponse> optDetail = paymentService.syncPaymentStatus(request.getTradeNo());

            if (optDetail.isPresent()) {
                responseObserver.onNext(SyncPaymentStatusResponse.newBuilder()
                        .setCode(GrpcResponseCode.SUCCESS.code())
                        .setMessage(GrpcResponseCode.SUCCESS.message())
                        .setData(toPaymentDetail(optDetail.get()))
                        .build());
            } else {
                responseObserver.onNext(SyncPaymentStatusResponse.newBuilder()
                        .setCode(GrpcResponseCode.NOT_FOUND.code())
                        .setMessage(GrpcResponseCode.NOT_FOUND.message())
                        .build());
            }
            responseObserver.onCompleted();

        } catch (StatusRuntimeException e) {
            responseObserver.onNext(SyncPaymentStatusResponse.newBuilder()
                    .setCode(mapStatusCode(e))
                    .setMessage(e.getStatus().getDescription())
                    .build());
            responseObserver.onCompleted();
        } catch (Exception e) {
            GrpcErrorHandler.handleException(responseObserver, SyncPaymentStatusResponse.newBuilder(), e, "PaymentGrpcService", "syncPaymentStatus");
        }
    }

    @Override
    public void closePayment(ClosePaymentRequest request, StreamObserver<ClosePaymentResponse> responseObserver) {
        try {
            // 授權：僅 BUYER 可關閉交易 + ownership 檢查
            GrpcAuthContext.requireAccountType(AccountType.BUYER);
            verifyPaymentOwnership(request.getTradeNo());

            boolean closed = paymentCloseService.closePayment(request.getTradeNo());

            responseObserver.onNext(ClosePaymentResponse.newBuilder()
                    .setCode(closed ? GrpcResponseCode.SUCCESS.code() : GrpcResponseCode.INTERNAL_ERROR.code())
                    .setMessage(closed ? "關閉成功" : "關閉失敗")
                    .build());
            responseObserver.onCompleted();

        } catch (StatusRuntimeException e) {
            responseObserver.onNext(ClosePaymentResponse.newBuilder()
                    .setCode(mapStatusCode(e))
                    .setMessage(e.getStatus().getDescription())
                    .build());
            responseObserver.onCompleted();
        } catch (Exception e) {
            GrpcErrorHandler.handleException(responseObserver, ClosePaymentResponse.newBuilder(), e, "PaymentGrpcService", "closePayment");
        }
    }

    @Override
    public void grantPayment(io.github.will413028.hawkr.payment.grpc.GrantPaymentRequest request,
                              StreamObserver<io.github.will413028.hawkr.payment.grpc.GrantPaymentResponse> responseObserver) {
        try {
            // 內部呼叫（Order Service 發起），免權限檢查（x-internal-service 自動處理）

            GrantPaymentResult result = paymentService.grantPayment(request.getOrderNo());

            // §5.6 P0-3a：內層 grantPayment 採 write-then-return，失敗終態隨交易 commit 存活、正常 return（不 throw）。
            // adapter 在 @Transactional 外，依內層回傳的結構化 grantOutcome 分流（§5.6.1）：
            //   GRANTED / PROVIDER_TOOK_OVER → 正常回應（帶 grant_outcome），PROVIDER_TOOK_OVER 為預期收斂結果非錯誤；
            //   null（PENDING_CONFIRM / PayUni 錯誤碼）→ throw gRPC error（維持「Order task 收 gRPC error → 重試」語意）。
            // 此 throw 不會回滾內層已 commit 的哨兵值（交易邊界已在 grantPayment 內結束）。
            if (result.grantOutcome() == null) {
                throw new PaymentBusinessException(GrpcResponseCode.GRANT_FAILED,
                        result.grantMessage() != null ? result.grantMessage() : result.grantStatus());
            }

            responseObserver.onNext(io.github.will413028.hawkr.payment.grpc.GrantPaymentResponse.newBuilder()
                    .setCode(GrpcResponseCode.SUCCESS.code())
                    .setMessage(GrpcResponseCode.SUCCESS.message())
                    .setGrantOutcome(result.grantOutcome())
                    .setData(io.github.will413028.hawkr.payment.grpc.GrantPaymentResult.newBuilder()
                            .setTradeNo(result.tradeNo())
                            .setOrderId(result.orderId())
                            .setProviderTradeNo(result.providerTradeNo())
                            .setAmount(formatAmount(result.amount()))
                            // PROVIDER_TOOK_OVER 無撥款指示時間（granted_at 刻意 NULL）→ 空字串
                            .setGrantedAt(result.grantedAt() != null
                                    ? String.valueOf(DateTimeUtils.toEpochMillis(result.grantedAt())) : "")
                            .build())
                    .build());
            responseObserver.onCompleted();

            log.info("PaymentGrpcService.grantPayment.success",
                    "撥款回應 grant_outcome=" + result.grantOutcome(), "order_no", request.getOrderNo());

        } catch (Exception e) {
            GrpcErrorHandler.handleException(responseObserver, io.github.will413028.hawkr.payment.grpc.GrantPaymentResponse.newBuilder(), e, "PaymentGrpcService", "grantPayment");
        }
    }

    // ==================== 多功能收付（agent_fee，線 B 收款通道）====================

    /**
     * 送出多功能收付（{@code ADR-009} 的「收款執行」那一格）。
     *
     * <h2>🔴 三件事必須分清楚，否則呼叫端會做出相反的處置</h2>
     * <ul>
     *   <li><b>明確受理</b>（{@code SUCCESS}）⇒ 應收進 SUBMITTED，等次日</li>
     *   <li><b>明確拒絕</b>（有回應、非 SUCCESS）⇒ 沒有受理、不存在「可能已經扣了」的窗口
     *       ⇒ 可以自動退避重送</li>
     *   <li><b>未取得回應</b>（逾時／連線失敗，{@code providerResponded=false}）
     *       ⇒ 🔴 <b>不得視為失敗、不得自動重送</b>——PayUni 可能已經受理了，
     *       而自動重送的期望值是「把一次不確定變成一次確定的重複扣款」</li>
     * </ul>
     *
     * <p>🔴 <b>本 RPC 不負責去重</b>（70 §14.2 逐字）：官方標 {@code TradeNo} 可重複，
     * 去重只能由呼叫端的 DB UNIQUE 保證，且必須<b>在呼叫本 RPC 之前</b>落地。
     */
    @Override
    public void submitAgentFee(io.github.will413028.hawkr.payment.grpc.SubmitAgentFeeRequest request,
                                StreamObserver<io.github.will413028.hawkr.payment.grpc.SubmitAgentFeeResponse> responseObserver) {
        try {
            long targetNet = Long.parseLong(request.getTargetNetAmount());
            int feeType = Integer.parseInt(request.getFeeType());
            int balType = Integer.parseInt(request.getBalType());

            io.github.will413028.hawkr.payment.agentfee.AgentFeeService.SubmitOutcome outcome =
                    agentFeeService.submit(request.getMerId(), request.getTradeNo(), targetNet, feeType, balType);

            responseObserver.onNext(io.github.will413028.hawkr.payment.grpc.SubmitAgentFeeResponse.newBuilder()
                    .setCode(GrpcResponseCode.SUCCESS.code())
                    .setMessage(GrpcResponseCode.SUCCESS.message())
                    .setData(io.github.will413028.hawkr.payment.grpc.SubmitAgentFeeData.newBuilder()
                            .setProviderStatus(outcome.providerStatus())
                            .setProviderMessage(outcome.providerMessage())
                            .setGrossAmount(String.valueOf(outcome.grossAmount()))
                            .setEstimatedFee(String.valueOf(outcome.estimatedFee()))
                            .setFeePolicyVersion(outcome.feePolicyVersion())
                            .setProviderResponded(outcome.providerResponded())
                            .build())
                    .build());
            responseObserver.onCompleted();

            log.info("PaymentGrpcService.submitAgentFee.done",
                    "agent_fee 送出 responded=" + outcome.providerResponded()
                            + " status=" + outcome.providerStatus(),
                    "tradeNo", request.getTradeNo());

        } catch (Exception e) {
            GrpcErrorHandler.handleException(responseObserver,
                    io.github.will413028.hawkr.payment.grpc.SubmitAgentFeeResponse.newBuilder(), e,
                    "PaymentGrpcService", "submitAgentFee");
        }
    }

    /**
     * 查詢多功能收付的撥款結果。
     *
     * <p>🔴 回應把 PayUni 的兩個哨兵值翻譯掉，因為它們在 wire 上長得像正常值：
     * <ul>
     *   <li>{@code FundDate = "0000-00-00"} ⇒ {@code fund_date} <b>不設值</b>（尚未撥款）</li>
     *   <li>查無此筆 ⇒ {@code found = false}，與「尚未撥款」<b>是兩件事</b>——
     *       混在一起的話，呼叫端會把「沒受理」誤讀成「還沒到次日」而一直等下去</li>
     * </ul>
     */
    @Override
    public void queryAgentFee(io.github.will413028.hawkr.payment.grpc.QueryAgentFeeRequest request,
                               StreamObserver<io.github.will413028.hawkr.payment.grpc.QueryAgentFeeResponse> responseObserver) {
        try {
            long grossAmount = Long.parseLong(request.getGrossAmount());

            io.github.will413028.hawkr.payment.agentfee.AgentFeeService.QueryOutcome outcome =
                    agentFeeService.query(request.getMerId(), request.getTradeNo(), grossAmount);

            io.github.will413028.hawkr.payment.grpc.QueryAgentFeeData.Builder data =
                    io.github.will413028.hawkr.payment.grpc.QueryAgentFeeData.newBuilder()
                            .setFound(outcome.found())
                            .setMerId(outcome.merId())
                            .setTradeNo(outcome.tradeNo())
                            .setGrossAmount(String.valueOf(outcome.grossAmount()))
                            .setFeeType(outcome.feeType())
                            .setBalType(outcome.balType())
                            .setTradeDate(outcome.tradeDate());
            outcome.fundDate().ifPresent(data::setFundDate);
            outcome.actualFee().ifPresent(fee -> data.setActualFee(String.valueOf(fee)));
            outcome.agentMerId().ifPresent(data::setAgentMerId);

            responseObserver.onNext(io.github.will413028.hawkr.payment.grpc.QueryAgentFeeResponse.newBuilder()
                    .setCode(GrpcResponseCode.SUCCESS.code())
                    .setMessage(GrpcResponseCode.SUCCESS.message())
                    .setData(data.build())
                    .build());
            responseObserver.onCompleted();

        } catch (Exception e) {
            GrpcErrorHandler.handleException(responseObserver,
                    io.github.will413028.hawkr.payment.grpc.QueryAgentFeeResponse.newBuilder(), e,
                    "PaymentGrpcService", "queryAgentFee");
        }
    }

    @Override
    public void closePaymentByOrderNo(ClosePaymentByOrderNoRequest request,
                                       StreamObserver<ClosePaymentByOrderNoResponse> responseObserver) {
        // #1871 OBS-09：order_no 進 MDC，讓這個方法內所有 log 行都能被 Loki structured
        // metadata 查到（見 k8s/monitoring/promtail-daemonset.yaml）。
        MDC.put("order_no", request.getOrderNo());
        try {
            // 內部呼叫（Order Service 取消訂單時發起），免權限檢查
            boolean closed = paymentService.closePaymentByOrderNo(request.getOrderNo());

            responseObserver.onNext(ClosePaymentByOrderNoResponse.newBuilder()
                    .setCode(GrpcResponseCode.SUCCESS.code())
                    .setMessage(closed ? "關閉成功" : "無需關閉")
                    .build());
            responseObserver.onCompleted();

        } catch (Exception e) {
            GrpcErrorHandler.handleException(responseObserver, ClosePaymentByOrderNoResponse.newBuilder(), e, "PaymentGrpcService", "closePaymentByOrderNo");
        } finally {
            MDC.remove("order_no");
        }
    }

    /**
     * §5.12 依訂單編號反查付款狀態（internal-only，issue #1349 L3）。
     * <p>
     * 供 Order Service {@code PaymentDeadlineScheduler} 逾期取消訂單前同步反查付款 SoT，
     * 避免「付款已成功但事件尚未送達」時誤取消造成資損。以 {@code order_no} 聚合該訂單
     * 的所有交易 row：命中 PAID row 即回該筆（呼叫端據此收斂訂單為 PAID、不取消）；
     * 否則回 canonical（最新）row；查無交易則 {@code code=0} 且 {@code data} 空
     * （訂單未建立付款交易屬正常情況）。
     */
    @Override
    public void getActivePaymentStatusByOrderNo(GetActivePaymentStatusByOrderNoRequest request,
                                                 StreamObserver<GetActivePaymentStatusByOrderNoResponse> responseObserver) {
        try {
            // §5.2 internal-only：排程器無使用者情境，僅接受內部服務呼叫（x-internal-service）
            if (!GrpcAuthContext.isInternalCall()) {
                throw Status.PERMISSION_DENIED
                        .withDescription("GetActivePaymentStatusByOrderNo 僅接受內部服務呼叫")
                        .asRuntimeException();
            }

            String orderNo = request.getOrderNo();
            // §2A.6 後同 order 可能多 row，canonical 未必即 PAID row → 以 order_no 聚合判定
            // （history ORDER BY created_at DESC，最新在前）
            java.util.List<PaymentDetailResponse> history = paymentService.getPaymentHistory(orderNo);

            GetActivePaymentStatusByOrderNoResponse.Builder builder =
                    GetActivePaymentStatusByOrderNoResponse.newBuilder()
                            .setCode(GrpcResponseCode.SUCCESS.code())
                            .setMessage(GrpcResponseCode.SUCCESS.message());

            if (history.isEmpty()) {
                // 訂單未建立付款交易屬正常情況：code=0 且 data 空
                log.info("PaymentGrpcService.getActivePaymentStatusByOrderNo.noTransaction",
                        "訂單無付款交易", "order_no", orderNo);
            } else {
                // 有 PAID row 回該筆（呼叫端據此收斂），否則回 canonical（最新）row
                PaymentDetailResponse selected = history.stream()
                        .filter(d -> d.status() == PaymentTransactionStatus.PAID)
                        .findFirst()
                        .orElse(history.get(0));
                builder.setData(toPaymentDetail(selected));
                log.info("PaymentGrpcService.getActivePaymentStatusByOrderNo.found",
                        "order_no=" + orderNo + " status=" + selected.status().value());
            }

            responseObserver.onNext(builder.build());
            responseObserver.onCompleted();

        } catch (Exception e) {
            GrpcErrorHandler.handleException(responseObserver, GetActivePaymentStatusByOrderNoResponse.newBuilder(), e, "PaymentGrpcService", "getActivePaymentStatusByOrderNo");
        }
    }

    /**
     * 58 號 §4.1「05 支付」：發票側唯讀取「訂單的付款事實」。
     *
     * <p><b>只讀不寫</b>——本 RPC 不觸碰支付任何狀態機、不寫任何表；發票是憑證編排，
     * 付款狀態的唯一寫入者仍是 webhook / reconcile 路徑。
     *
     * <p>回的是 canonical 交易：{@code status = PAID} 且 {@code is_duplicate = FALSE}。
     * 重複付款副 row 不開票也不折讓（58 號 §7.1.2 註 3），那筆錢由 payment 側 rescue 退還。
     *
     * <p>查無 canonical 交易 ⇒ 回<b>非 0 業務碼</b>（{@code TRANSACTION_NOT_FOUND_FOR_ORDER}）
     * 且不帶 {@code data}，不得回 {@code code=0} ＋ 空 data：呼叫端唯一安全的行為是 fail-loud
     * （{@code order.paid} 重試三次進 DLQ），空成功回應會讓發票日期靜默退回本地時鐘。
     *
     * <p>🔴 {@code provider_paid_at} 是三態欄位：DB 為 NULL 時<b>完全不 set</b>
     * （{@code hasProviderPaidAt() == false}），絕不 set 成空字串。ATM webhook 常無此欄，
     * nil 是設計上的合法值；若「拿不到」與「本來就沒有」在 wire 上分不開，
     * invoice-service 的 {@code PaidAt()} 會靜默改用我方 webhook 收單時間，而那沒有任何訊號。
     */
    @Override
    public void getPaymentFactsInternal(GetPaymentFactsInternalRequest request,
                                        StreamObserver<GetPaymentFactsInternalResponse> responseObserver) {
        try {
            // §5.2 internal-only：發票側服務間呼叫，無使用者情境。簽章由 server interceptor 驗，
            // 這裡只讀 interceptor 的判定結果，不自行解析 x-internal-service。
            if (!GrpcAuthContext.isInternalCall()) {
                throw Status.PERMISSION_DENIED
                        .withDescription("GetPaymentFactsInternal 僅接受內部服務呼叫")
                        .asRuntimeException();
            }

            String orderNo = request.getOrderNo();
            if (orderNo == null || orderNo.isBlank()) {
                throw new IllegalArgumentException("order_no 不可為空");
            }

            PaymentTransaction txn = transactionRepository.findCanonicalPaidByOrderId(orderNo)
                    .orElseThrow(() -> new PaymentBusinessException(
                            GrpcResponseCode.TRANSACTION_NOT_FOUND_FOR_ORDER));

            PaymentFactsData.Builder data = PaymentFactsData.newBuilder()
                    .setOrderNo(txn.orderId())
                    .setTradeNo(txn.tradeNo())
                    // 我方 webhook 寫入當下的 nowUtc（不是 PayUni 回報的時間，兩者語意不同）
                    .setPayTime(ProtoConversionUtils.formatIso8601(txn.payTime()))
                    .setTradeAmt(ProtoConversionUtils.formatAmount(txn.amount()))
                    .setPaymentType(txn.paymentType().value())
                    .setIsDuplicate(txn.isDuplicate());

            // 🔴 三態：NULL ⇒ 不 set（見方法註解）。這一行改成無條件 set 會靜默毀掉 invoice_date。
            if (txn.providerPaidAt() != null) {
                // PayUni 回報的真實付款時間（卡片 AuthDay+AuthTime／ATM PaymentDay）
                data.setProviderPaidAt(ProtoConversionUtils.formatIso8601(txn.providerPaidAt()));
            }

            if (txn.payTime() == null) {
                // PAID 交易照契約必有 pay_time；缺了代表寫入路徑有洞，留線索但不改回應形狀
                log.warn("PaymentGrpcService.getPaymentFactsInternal.payTimeNull",
                        "PAID 交易缺 pay_time，trade_no=" + txn.tradeNo(), "order_no", orderNo);
            }

            responseObserver.onNext(GetPaymentFactsInternalResponse.newBuilder()
                    .setCode(GrpcResponseCode.SUCCESS.code())
                    .setMessage(GrpcResponseCode.SUCCESS.message())
                    .setData(data.build())
                    .build());
            responseObserver.onCompleted();

        } catch (Exception e) {
            GrpcErrorHandler.handleException(responseObserver,
                    GetPaymentFactsInternalResponse.newBuilder(),
                    e, "PaymentGrpcService", "getPaymentFactsInternal");
        }
    }

    @Override
    public void createRefund(io.github.will413028.hawkr.payment.grpc.CreateRefundRequest request,
                             StreamObserver<io.github.will413028.hawkr.payment.grpc.CreateRefundResponse> responseObserver) {
        try {
            // 授權：退款僅接受內部服務呼叫（x-internal-service）。唯一合法入口為 AfterSales
            // 審核核准後由 RefundTaskWorker 以 internal call 發起（05 §5.5.3）。外部帶 JWT 的
            // 前端一律 PERMISSION_DENIED，杜絕買家繞過售後審核自助退款造成資損。
            if (!GrpcAuthContext.isInternalCall()) {
                throw Status.PERMISSION_DENIED
                        .withDescription("退款僅接受內部服務呼叫")
                        .asRuntimeException();
            }

            // 組裝 DTO（含冪等鍵）
            String idempotencyKey = request.getRefundNo();  // Proto refund_no 作為冪等鍵

            RefundDetailResponse response = refundService.createRefund(
                    new io.github.will413028.hawkr.payment.application.dto.CreateRefundRequest(
                            request.getTradeNo(),
                            parseAmount(request.getAmount()),  // Proto string → Java long
                            request.getReason(),
                            idempotencyKey.isEmpty() ? null : idempotencyKey
                    )
            );

            // 檢查退款實際結果：PayUni 可能拒絕退款但 Application Service 不拋例外
            if (response.status() == RefundStatus.FAILED) {
                String failMessage = response.providerMessage() != null
                        ? "退款失敗: " + response.providerMessage()
                        : "退款失敗";
                log.warn("PaymentGrpcService.createRefund.payuniFailed", failMessage,
                        "refund_no", response.refundNo());
                responseObserver.onNext(io.github.will413028.hawkr.payment.grpc.CreateRefundResponse.newBuilder()
                        .setCode(GrpcResponseCode.INTERNAL_ERROR.code())
                        .setMessage(failMessage)
                        .setData(toRefundDetail(response))
                        .build());
                responseObserver.onCompleted();
                return;
            }

            responseObserver.onNext(io.github.will413028.hawkr.payment.grpc.CreateRefundResponse.newBuilder()
                    .setCode(GrpcResponseCode.SUCCESS.code())
                    .setMessage(GrpcResponseCode.SUCCESS.message())
                    .setData(toRefundDetail(response))
                    .build());
            responseObserver.onCompleted();

            log.info("PaymentGrpcService.createRefund.success", "建立退款成功", "refund_no", response.refundNo());

        } catch (StatusRuntimeException e) {
            responseObserver.onNext(io.github.will413028.hawkr.payment.grpc.CreateRefundResponse.newBuilder()
                    .setCode(mapStatusCode(e))
                    .setMessage(e.getStatus().getDescription())
                    .build());
            responseObserver.onCompleted();
        } catch (Exception e) {
            GrpcErrorHandler.handleException(responseObserver, io.github.will413028.hawkr.payment.grpc.CreateRefundResponse.newBuilder(), e, "PaymentGrpcService", "createRefund");
        }
    }

    @Override
    public void getRefund(GetRefundRequest request, StreamObserver<GetRefundResponse> responseObserver) {
        try {
            // 授權：buyer-only（#2501 收斂）。退款明細含買家退款金額/理由，賣家不可經此讀他店退款。
            GrpcAuthContext.requireAccountType(AccountType.BUYER);

            Optional<RefundDetailResponse> optDetail = refundService.getRefundDetail(request.getRefundNo());
            if (optDetail.isEmpty()) {
                // 越權防枚舉（API 設計規範 §14.1）：退款不存在與「父交易越權」回應必須一致，
                // 否則可用 refund_no 枚舉「此退款是否存在」。統一走 verifyPaymentOwnership 的 NOT_FOUND/"交易不存在"。
                throw Status.NOT_FOUND.withDescription("交易不存在").asRuntimeException();
            }

            // ownership 檢查：透過退款的 tradeNo 驗證父交易歸屬
            verifyPaymentOwnership(optDetail.get().tradeNo());

            responseObserver.onNext(GetRefundResponse.newBuilder()
                    .setCode(GrpcResponseCode.SUCCESS.code())
                    .setMessage(GrpcResponseCode.SUCCESS.message())
                    .setData(toRefundDetail(optDetail.get()))
                    .build());
            responseObserver.onCompleted();

        } catch (StatusRuntimeException e) {
            responseObserver.onNext(GetRefundResponse.newBuilder()
                    .setCode(mapStatusCode(e))
                    .setMessage(e.getStatus().getDescription())
                    .build());
            responseObserver.onCompleted();
        } catch (Exception e) {
            GrpcErrorHandler.handleException(responseObserver, GetRefundResponse.newBuilder(), e, "PaymentGrpcService", "getRefund");
        }
    }

    @Override
    public void getRefundsByTradeNo(GetRefundsByTradeNoRequest request,
                                    StreamObserver<GetRefundsByTradeNoResponse> responseObserver) {
        try {
            // 授權：buyer-only（#2501 收斂）。trade_no 退款清單同屬買家 PII，賣家不可經此讀。
            GrpcAuthContext.requireAccountType(AccountType.BUYER);
            verifyPaymentOwnership(request.getTradeNo());

            List<RefundDetailResponse> refunds = refundService.getRefundsByTradeNo(request.getTradeNo());

            GetRefundsByTradeNoResponse.Builder builder = GetRefundsByTradeNoResponse.newBuilder()
                    .setCode(GrpcResponseCode.SUCCESS.code())
                    .setMessage(GrpcResponseCode.SUCCESS.message());

            for (RefundDetailResponse refund : refunds) {
                builder.addRefunds(toRefundDetail(refund));
            }

            responseObserver.onNext(builder.build());
            responseObserver.onCompleted();

        } catch (StatusRuntimeException e) {
            responseObserver.onNext(GetRefundsByTradeNoResponse.newBuilder()
                    .setCode(mapStatusCode(e))
                    .setMessage(e.getStatus().getDescription())
                    .build());
            responseObserver.onCompleted();
        } catch (Exception e) {
            GrpcErrorHandler.handleException(responseObserver, GetRefundsByTradeNoResponse.newBuilder(), e, "PaymentGrpcService", "getRefundsByTradeNo");
        }
    }

    // ==================== 批次 provider 狀態聚合（收斂規範 §6.3）====================

    @Override
    public void batchGetProviderStatusByOrderIds(BatchGetProviderStatusByOrderIdsRequest request,
                                                  StreamObserver<BatchGetProviderStatusByOrderIdsResponse> responseObserver) {
        try {
            List<String> orderIds = request.getOrderIdsList();
            java.util.Map<String, ProviderStatusEntry> entries = buildProviderEntries(
                    transactionRepository.findProviderStatusByOrderIds(orderIds));

            responseObserver.onNext(BatchGetProviderStatusByOrderIdsResponse.newBuilder()
                    .setCode(GrpcResponseCode.SUCCESS.code())
                    .setMessage(GrpcResponseCode.SUCCESS.message())
                    .putAllEntries(entries)
                    .build());
            responseObserver.onCompleted();
        } catch (Exception e) {
            GrpcErrorHandler.handleException(responseObserver,
                    BatchGetProviderStatusByOrderIdsResponse.newBuilder(),
                    e, "PaymentGrpcService", "batchGetProviderStatusByOrderIds");
        }
    }

    @Override
    public void batchGetRefundProviderStatusByCaseIds(BatchGetRefundProviderStatusByCaseIdsRequest request,
                                                       StreamObserver<BatchGetRefundProviderStatusResponse> responseObserver) {
        try {
            List<String> caseIds = request.getCaseIdsList();
            java.util.Map<String, ProviderStatusEntry> entries = buildProviderEntries(
                    refundRepository.findProviderStatusByCaseIds(caseIds));

            responseObserver.onNext(BatchGetRefundProviderStatusResponse.newBuilder()
                    .setCode(GrpcResponseCode.SUCCESS.code())
                    .setMessage(GrpcResponseCode.SUCCESS.message())
                    .putAllEntries(entries)
                    .build());
            responseObserver.onCompleted();
        } catch (Exception e) {
            GrpcErrorHandler.handleException(responseObserver,
                    BatchGetRefundProviderStatusResponse.newBuilder(),
                    e, "PaymentGrpcService", "batchGetRefundProviderStatusByCaseIds");
        }
    }

    @Override
    public void batchGetRefundProviderStatusByOrderIds(BatchGetRefundProviderStatusByOrderIdsRequest request,
                                                        StreamObserver<BatchGetRefundProviderStatusResponse> responseObserver) {
        try {
            List<String> orderIds = request.getOrderIdsList();
            java.util.Map<String, ProviderStatusEntry> entries = buildProviderEntries(
                    refundRepository.findProviderStatusByOrderIds(orderIds));

            responseObserver.onNext(BatchGetRefundProviderStatusResponse.newBuilder()
                    .setCode(GrpcResponseCode.SUCCESS.code())
                    .setMessage(GrpcResponseCode.SUCCESS.message())
                    .putAllEntries(entries)
                    .build());
            responseObserver.onCompleted();
        } catch (Exception e) {
            GrpcErrorHandler.handleException(responseObserver,
                    BatchGetRefundProviderStatusResponse.newBuilder(),
                    e, "PaymentGrpcService", "batchGetRefundProviderStatusByOrderIds");
        }
    }

    /**
     * 將 Repository 回傳的 key/provider_status/provider_message 列表轉為 proto map。
     * Repository 統一以 column alias `key` 作為主鍵（order_id / case_id 都對齊）。
     */
    private java.util.Map<String, ProviderStatusEntry> buildProviderEntries(
            List<io.github.will413028.hawkr.payment.infrastructure.persistence.dto.ProviderStatusRow> rows) {
        if (rows == null || rows.isEmpty()) {
            return java.util.Collections.emptyMap();
        }
        java.util.Map<String, ProviderStatusEntry> out = new java.util.HashMap<>(rows.size());
        for (var row : rows) {
            if (row.getKey() == null) continue;
            ProviderStatusEntry.Builder eb = ProviderStatusEntry.newBuilder();
            if (row.getProviderStatus() != null) eb.setProviderStatus(row.getProviderStatus());
            if (row.getProviderMessage() != null) eb.setProviderMessage(row.getProviderMessage());
            out.put(row.getKey(), eb.build());
        }
        return out;
    }

    @Override
    public void getRefundPageUrl(GetRefundPageUrlRequest request,
                                  StreamObserver<GetRefundPageUrlResponse> responseObserver) {
        try {
            // 內部服務呼叫（AfterSales Service 發起），免權限檢查

            String tradeNo = request.getTradeNo();
            long amount = parseAmount(request.getAmount());
            String returnUrl = request.getReturnUrl();

            if (tradeNo.isEmpty()) {
                throw new IllegalArgumentException("trade_no 為必填");
            }
            if (returnUrl.isEmpty()) {
                throw new IllegalArgumentException("return_url 為必填");
            }

            // 查詢原交易取得 mer_id 和 provider_trade_no
            PaymentDetailResponse detail = paymentService.getPaymentDetail(tradeNo)
                    .orElseThrow(() -> new IllegalArgumentException("交易不存在: " + tradeNo));

            // Defense in depth (收斂規範 §8.4)：校驗退款金額 <= 原交易可退餘額。
            // Why: ATM 頁面版不經 createRefund 校驗；若上游誤算金額超過 PayUni 實收，
            // PayUni 會回「網址無效」而平台無紀錄。提前在 RPC 層擋下並回明確訊息。
            //
            // ★ 排除本筆（§5.5.5 L2）：ATM 流程先預建一筆 REQUESTED('0') refund row，買家取退款頁時本筆
            // 已存在於表中。sumRefundedAmountByTradeNo 含在途('0','1','2') 會把本筆自己算進已退累計，
            // 使全額 / 大額 ATM 退款 remaining=0 而誤報 REFUND_AMOUNT_EXCEEDED（買家永遠拿不到退款頁）。
            // 故改用「排除本筆活躍退款」的累計 = 其他退款已用額度；ATM 同交易同時僅一筆活躍退款（由
            // createRefund 的 hasProcessingRefund 保證），findActiveByTradeNo 取到的即本筆。
            String activeRefundNo = refundRepository.findActiveByTradeNo(tradeNo)
                    .map(RefundTransaction::refundNo)
                    .orElse(null);
            long alreadyRefunded = refundRepository.sumRefundedAmountByTradeNoExcluding(tradeNo, activeRefundNo);
            long remaining = detail.amount() - alreadyRefunded;
            if (amount <= 0 || amount > remaining) {
                throw new PaymentBusinessException(GrpcResponseCode.REFUND_AMOUNT_EXCEEDED,
                        "amount=" + amount + " 超過可退餘額=" + remaining
                                + "（原交易 " + detail.amount() + " 已退 " + alreadyRefunded + "）");
            }

            // 組裝 ATM 退款頁面版 UppFormData → 轉為 redirect URL
            var uppFormData = paymentService.buildRefundPageFormData(
                    detail.merId(), detail.providerTradeNo(), amount, returnUrl);
            String redirectUrl = pageService.buildRedirectUrl(uppFormData);

            responseObserver.onNext(GetRefundPageUrlResponse.newBuilder()
                    .setCode(GrpcResponseCode.SUCCESS.code())
                    .setMessage(GrpcResponseCode.SUCCESS.message())
                    .setData(RefundPageData.newBuilder()
                            .setRedirectUrl(redirectUrl)
                            .build())
                    .build());
            responseObserver.onCompleted();

            log.info("PaymentGrpcService.getRefundPageUrl.success",
                    "取得退款頁面 URL", "trade_no", tradeNo);

        } catch (Exception e) {
            GrpcErrorHandler.handleException(responseObserver, GetRefundPageUrlResponse.newBuilder(), e, "PaymentGrpcService", "getRefundPageUrl");
        }
    }

    // ==================== 信用卡付款 Return 處理（統一為 gRPC） ====================

    /**
     * 信用卡付款完成回調處理。
     * 從 PayUni Form POST 的 EncryptInfo + HashInfo 解密交易結果，
     * 更新 DB 狀態，發布 MQ 事件，依 §5.9 矩陣組 redirect_url。
     */
    @Override
    public void handlePaymentPageReturn(HandlePaymentPageReturnRequest request,
                                         StreamObserver<HandlePaymentPageReturnResponse> responseObserver) {
        try {
            String encryptInfo = request.getEncryptInfo();
            String hashInfo = request.getHashInfo();

            // 1. 驗簽
            if (!cryptoService.verifyHash(encryptInfo, hashInfo)) {
                log.warn("PaymentGrpcService.handlePaymentPageReturn.verifyFailed", "簽章驗證失敗");
                responseObserver.onNext(HandlePaymentPageReturnResponse.newBuilder()
                        .setCode(GrpcResponseCode.SUCCESS.code()).setMessage(GrpcResponseCode.SUCCESS.message())
                        .setRedirectUrl(buildRedirectUrl(null, "", PaymentResultDisplay.FAILED, ""))
                        .build());
                responseObserver.onCompleted();
                return;
            }

            // 2. 解密
            Map<String, String> decryptedData = cryptoService.decrypt(encryptInfo);

            String tradeNo = decryptedData.getOrDefault("MerTradeNo", "");
            String tradeStatus = decryptedData.getOrDefault("TradeStatus", "");

            log.info("PaymentGrpcService.handlePaymentPageReturn.decrypted",
                    "解密成功, tradeStatus=" + tradeStatus, "tradeNo", tradeNo);

            // §11.4 入向稽核：解密成功後 + dedup check 之前寫 payment_logs
            try {
                auditLogger.recordInboundReturn(
                        tradeNo,
                        decryptedData.get("TradeNo"),
                        decryptedData);
            } catch (Exception auditEx) {
                log.warn("PaymentGrpcService.handlePaymentPageReturn.audit.failed",
                        "入向稽核寫入失敗 err=" + auditEx.getMessage());
            }

            // §5.9 SoT 在 DB：lookup payment_transactions 取 client_type / app_type / order_id
            PaymentTransaction transaction = transactionRepository.findByTradeNo(tradeNo).orElse(null);

            // 3. 判斷付款結果
            boolean isSuccess = "1".equals(tradeStatus);
            PaymentResultDisplay displayStatus = isSuccess ? PaymentResultDisplay.SUCCESS : PaymentResultDisplay.FAILED;

            // 4. 冪等檢查
            String idempotencyKey = notifyHandler.extractIdempotencyKey(decryptedData);
            if (idempotencyKey != null
                    && webhookMapper.existsProcessed(WebhookProvider.PAYUNI.value(), idempotencyKey)) {
                log.info("PaymentGrpcService.handlePaymentPageReturn.idempotent",
                        "已處理過（冪等跳過）", "key", idempotencyKey);
                responseObserver.onNext(HandlePaymentPageReturnResponse.newBuilder()
                        .setCode(GrpcResponseCode.SUCCESS.code()).setMessage(GrpcResponseCode.SUCCESS.message())
                        .setRedirectUrl(buildRedirectUrl(transaction, tradeNo, displayStatus, tradeStatus))
                        .build());
                responseObserver.onCompleted();
                return;
            }

            // 5. 儲存 Inbox 記錄
            Long webhookId = saveReturnInbox(encryptInfo, decryptedData, idempotencyKey);

            // 6. Transaction 內執行業務邏輯（更新交易狀態 + outbox 事件寫入 + 標記已處理，三者原子）
            //    §5.10.1 L1'：付款結果事件由 PaymentNotifyHandler 經 PaymentOutboxWriter 在此事務內
            //    寫入 outbox 並註冊 afterCommit 即時投遞
            transactionTemplate.execute(txStatus -> {
                WebhookHandleResult r = notifyHandler.handle(decryptedData);
                webhookMapper.markAsProcessed(webhookId, r.processStatus(),
                        r.message(), DateTimeUtils.nowUtc());
                return r;
            });

            log.info("PaymentGrpcService.handlePaymentPageReturn.done",
                    "處理完成, status=" + displayStatus, "tradeNo", tradeNo);

            responseObserver.onNext(HandlePaymentPageReturnResponse.newBuilder()
                    .setCode(GrpcResponseCode.SUCCESS.code()).setMessage(GrpcResponseCode.SUCCESS.message())
                    .setRedirectUrl(buildRedirectUrl(transaction, tradeNo, displayStatus, tradeStatus))
                    .build());
            responseObserver.onCompleted();

        } catch (Exception e) {
            GrpcErrorHandler.handleException(responseObserver, HandlePaymentPageReturnResponse.newBuilder(),
                    e, "PaymentGrpcService", "handlePaymentPageReturn");
        }
    }

    // ==================== PayUni 加密資料通用解密 ====================

    /**
     * 通用解密 PayUni EncryptInfo。
     * 供其他微服務（如 AfterSales）呼叫，取得解密後的 key-value 資料。
     */
    @Override
    public void decryptPayUniResult(DecryptPayUniResultRequest request,
                                     StreamObserver<DecryptPayUniResultResponse> responseObserver) {
        try {
            String encryptInfo = request.getEncryptInfo();
            String hashInfo = request.getHashInfo();

            // 1. 驗簽
            if (!cryptoService.verifyHash(encryptInfo, hashInfo)) {
                log.warn("PaymentGrpcService.decryptPayUniResult.verifyFailed", "簽章驗證失敗");
                responseObserver.onNext(DecryptPayUniResultResponse.newBuilder()
                        .setCode(GrpcResponseCode.INVALID_PARAM.code())
                        .setMessage("簽章驗證失敗")
                        .build());
                responseObserver.onCompleted();
                return;
            }

            // 2. 解密
            Map<String, String> decryptedData = cryptoService.decrypt(encryptInfo);

            log.info("PaymentGrpcService.decryptPayUniResult.success", "解密成功");

            // 3. 組裝回應
            DecryptedPayUniData.Builder dataBuilder = DecryptedPayUniData.newBuilder();
            putIfPresent(dataBuilder, decryptedData, "Status", "status");
            putIfPresent(dataBuilder, decryptedData, "MerTradeNo", "merTradeNo");
            putIfPresent(dataBuilder, decryptedData, "TradeNo", "tradeNo");
            putIfPresent(dataBuilder, decryptedData, "RefundAmt", "refundAmt");
            putIfPresent(dataBuilder, decryptedData, "PaymentType", "paymentType");
            putIfPresent(dataBuilder, decryptedData, "EstimatedRefundDate", "estimatedRefundDate");
            putIfPresent(dataBuilder, decryptedData, "Message", "errorMessage");

            responseObserver.onNext(DecryptPayUniResultResponse.newBuilder()
                    .setCode(GrpcResponseCode.SUCCESS.code())
                    .setMessage(GrpcResponseCode.SUCCESS.message())
                    .setData(dataBuilder.build())
                    .build());
            responseObserver.onCompleted();

        } catch (Exception e) {
            GrpcErrorHandler.handleException(responseObserver, DecryptPayUniResultResponse.newBuilder(),
                    e, "PaymentGrpcService", "decryptPayUniResult");
        }
    }

    // ==================== ATM 退款頁面受理 ====================

    /**
     * ATM 退款頁面受理確認（收斂規範 §4.4 sync 階段）。
     * AfterSales 解密 PayUni 頁面回傳 payload 後呼叫；Payment 將 payment_refunds 推進為 PROCESSING
     * 並寫入 estimated_refund_date / provider_status / provider_message。
     */
    @Override
    public void ackAtmRefundAccepted(AckAtmRefundAcceptedRequest request,
                                      StreamObserver<AckAtmRefundAcceptedResponse> responseObserver) {
        try {
            String refundNo = request.getRefundNo();
            if (refundNo == null || refundNo.isBlank()) {
                responseObserver.onNext(AckAtmRefundAcceptedResponse.newBuilder()
                        .setCode(GrpcResponseCode.INVALID_PARAM.code())
                        .setMessage("refund_no 不可為空")
                        .build());
                responseObserver.onCompleted();
                return;
            }
            refundService.ackAtmRefundAccepted(
                    refundNo,
                    request.getProviderStatus(),
                    request.getProviderMessage(),
                    request.getEstimatedRefundDate()
            );
            responseObserver.onNext(AckAtmRefundAcceptedResponse.newBuilder()
                    .setCode(GrpcResponseCode.SUCCESS.code())
                    .setMessage(GrpcResponseCode.SUCCESS.message())
                    .build());
            responseObserver.onCompleted();
        } catch (Exception e) {
            GrpcErrorHandler.handleException(responseObserver, AckAtmRefundAcceptedResponse.newBuilder(),
                    e, "PaymentGrpcService", "ackAtmRefundAccepted");
        }
    }

    // ==================== Return 處理共用方法 ====================

    /**
     * §5.9 依 client_type × app_type 矩陣組 redirect_url。
     * <p>
     * transaction 為 null 時（驗簽失敗、找不到 trade_no）退回 MOBILE_APP+BUYER deep link，
     * 因為此時無法從 DB 知道使用者環境，且 MOBILE_APP 路徑對 web 使用者也只是 scheme 失敗（不會誤導）。
     */
    private String buildRedirectUrl(PaymentTransaction transaction, String tradeNo,
                                    PaymentResultDisplay display, String code) {
        String safeTradeNo = tradeNo != null ? tradeNo : "";
        String safeCode = code != null ? code : "";

        String clientTypeValue = transaction != null ? transaction.clientType() : null;
        String appTypeValue = transaction != null ? transaction.appType() : null;
        if (clientTypeValue == null || clientTypeValue.isBlank()) {
            clientTypeValue = ClientType.MOBILE_APP.value();
        }
        if (appTypeValue == null || appTypeValue.isBlank()) {
            appTypeValue = AppType.BUYER.value();
        }
        ClientType clientType = ClientType.fromValue(clientTypeValue);
        AppType appType = AppType.fromValue(appTypeValue);

        if (clientType == ClientType.MOBILE_APP) {
            String scheme = appType == AppType.SELLER ? "hawkrseller" : "hawkrbuyer";
            return scheme + "://payment/result?tradeNo=" + urlEncode(safeTradeNo) + "&status=" + display.code();
        }

        // WEB_BROWSER：Phase 1 SELLER 不支援，fallback 到 BUYER 路徑並寫 WARN 留痕
        if (appType == AppType.SELLER) {
            insertWebSellerFallbackWarn(safeTradeNo);
        }
        String orderNo = transaction != null && transaction.orderId() != null ? transaction.orderId() : "";
        String base = webFrontendUrl != null ? webFrontendUrl : "";
        if (base.endsWith("/")) {
            base = base.substring(0, base.length() - 1);
        }
        return base + "/orders/" + urlEncode(orderNo)
                + "?status=" + display.code()
                + "&trade_no=" + urlEncode(safeTradeNo)
                + "&code=" + urlEncode(safeCode);
    }

    private void insertWebSellerFallbackWarn(String tradeNo) {
        try {
            PaymentLogEntity entity = new PaymentLogEntity();
            entity.setTradeNo(tradeNo);
            entity.setLogType("WEB_SELLER_FALLBACK");
            entity.setLogLevel("WARNING");
            entity.setMessage("WEB_BROWSER+SELLER 組合 Phase 1 不支援，fallback 為 BUYER 路徑");
            entity.setTriggerSource("PAYUNI_RETURN");
            paymentLogMapper.insert(entity);
        } catch (Exception ex) {
            log.warn("PaymentGrpcService.handlePaymentPageReturn.fallbackLog.failed",
                    "WEB+SELLER fallback log 寫入失敗 err=" + ex.getMessage());
        }
    }

    /**
     * 儲存 Return Inbox 記錄。
     */
    private Long saveReturnInbox(String rawEncryptInfo, Map<String, String> decryptedData,
                                  String idempotencyKey) {
        PaymentWebhookEntity entity = new PaymentWebhookEntity();
        entity.setWebhookType(WebhookProvider.PAYUNI.value());
        entity.setTradeNo(decryptedData.get("MerTradeNo"));
        entity.setProviderTradeNo(decryptedData.get("TradeNo"));
        entity.setIdempotencyKey(idempotencyKey);
        entity.setRawRequest(rawEncryptInfo);
        entity.setDecryptedPayload(toJson(decryptedData));
        entity.setIsValid(true);
        entity.setIsProcessed(false);
        entity.setProcessStatus(WebhookProcessStatus.PENDING.value());
        entity.setReceivedAt(DateTimeUtils.nowUtc());
        entity.setSourceIp(SOURCE_IP_GRPC_RETURN);

        webhookMapper.insert(entity);
        return entity.getId();
    }

    private void putIfPresent(DecryptedPayUniData.Builder builder, Map<String, String> data,
                               String payuniKey, String fieldName) {
        String value = data.getOrDefault(payuniKey, "");
        switch (fieldName) {
            case "status" -> builder.setStatus(value);
            case "merTradeNo" -> builder.setMerTradeNo(value);
            case "tradeNo" -> builder.setTradeNo(value);
            case "refundAmt" -> builder.setRefundAmt(value);
            case "paymentType" -> builder.setPaymentType(value);
            case "estimatedRefundDate" -> builder.setEstimatedRefundDate(value);
            case "errorMessage" -> builder.setErrorMessage(value);
        }
    }

    private String toJson(Object obj) {
        try {
            return objectMapper.writeValueAsString(obj);
        } catch (Exception e) {
            return "{}";
        }
    }

    private static String urlEncode(String value) {
        return URLEncoder.encode(value, StandardCharsets.UTF_8);
    }

    /** proto3 string 欄位不接受 null，統一 null → 空字串 */
    private static String nullSafe(String value) {
        return value == null ? "" : value;
    }

    // ==================== 轉換方法 ====================

    /**
     * Proto string → Java long（接收外部金額資料）
     * 委派至 ProtoConversionUtils
     */
    private long parseAmount(String amountStr) {
        return ProtoConversionUtils.parseAmountRequired(amountStr, "金額");
    }

    /**
     * Proto string → Java Integer（解析有效天數，選填欄位）
     * 空字串或 ≤0 回傳 null（使用預設值）
     */
    private Integer parseExpireDays(String expireDaysStr) {
        Integer days = ProtoConversionUtils.parseIntOrNull(expireDaysStr);
        return (days != null && days > 0) ? days : null;
    }

    /**
     * Java long → Proto string（回傳外部金額資料）
     * 委派至 ProtoConversionUtils
     */
    private String formatAmount(long amount) {
        return ProtoConversionUtils.formatAmount(amount);
    }

    /**
     * Proto payment_type value → Domain PaymentMethod
     */
    private PaymentMethod toPaymentMethod(String paymentTypeValue) {
        if (paymentTypeValue == null || paymentTypeValue.isBlank()) {
            throw new PaymentBusinessException(GrpcResponseCode.PAYMENT_TYPE_REQUIRED);
        }
        try {
            return PaymentMethod.fromValue(paymentTypeValue);
        } catch (IllegalArgumentException e) {
            throw new PaymentBusinessException(GrpcResponseCode.INVALID_PAYMENT_TYPE, "未知支付方式");
        }
    }

    /**
     * 轉換為 Proto PaymentResult
     */
    private PaymentResult toPaymentResult(io.github.will413028.hawkr.payment.application.dto.CreatePaymentResponse response) {
        PaymentResult.Builder builder = PaymentResult.newBuilder()
                .setTradeNo(response.tradeNo())
                .setOrderId(response.orderId())
                .setPaymentType(response.paymentType().value())
                .setAmount(formatAmount(response.amount()))  // Java long → Proto string
                .setStatus(response.status().value());

        // UPP 表單資料（信用卡支付）+ redirectUrl
        if (response.uppFormData() != null) {
            builder.setUppFormData(toProtoUppFormData(response.uppFormData()));
            String redirectUrl = pageService.buildRedirectUrl(response.uppFormData());
            if (!redirectUrl.isEmpty()) {
                builder.setRedirectUrl(redirectUrl);
            }
        }
        if (response.atmBankCode() != null) {
            builder.setAtmBankCode(response.atmBankCode());
        }
        if (response.atmVirtualAccount() != null) {
            builder.setAtmVirtualAccount(response.atmVirtualAccount());
        }
        if (response.atmExpireDate() != null) {
            builder.setAtmExpireDate(formatDate(response.atmExpireDate()));
        }
        if (response.expireAt() != null) {
            builder.setExpireAt(String.valueOf(DateTimeUtils.toEpochMillis(response.expireAt())));
        }

        return builder.build();
    }

    /**
     * 轉換為 Proto UppFormData
     */
    private io.github.will413028.hawkr.payment.grpc.UppFormData toProtoUppFormData(
            io.github.will413028.hawkr.payuni.payment.model.UppFormData uppFormData) {
        return io.github.will413028.hawkr.payment.grpc.UppFormData.newBuilder()
                .setActionUrl(uppFormData.actionUrl())
                .setMerId(uppFormData.merId())
                .setVersion(uppFormData.version())
                .setEncryptInfo(uppFormData.encryptInfo())
                .setHashInfo(uppFormData.hashInfo())
                .setIsPlatform(uppFormData.isPlatform())
                .build();
    }

    /**
     * 轉換為 Proto PaymentDetail
     */
    private PaymentDetail toPaymentDetail(PaymentDetailResponse response) {
        PaymentDetail.Builder builder = PaymentDetail.newBuilder()
                .setTradeNo(response.tradeNo())
                .setOrderId(response.orderId())
                .setPaymentType(response.paymentType().value())
                .setAmount(formatAmount(response.amount()))  // Java long → Proto string
                .setStatus(response.status().value());

        if (response.merId() != null) {
            builder.setMerId(response.merId());
        }
        if (response.buyerId() != null) {
            builder.setBuyerId(response.buyerId());
        }
        if (response.productName() != null) {
            builder.setProductName(response.productName());
        }
        if (response.providerTradeNo() != null) {
            builder.setProviderTradeNo(response.providerTradeNo());
        }
        if (response.providerStatus() != null) {
            builder.setProviderStatus(response.providerStatus());
        }
        if (response.providerMessage() != null) {
            builder.setProviderMessage(response.providerMessage());
        }
        if (response.atmBankCode() != null) {
            builder.setAtmBankCode(response.atmBankCode());
        }
        if (response.atmVirtualAccount() != null) {
            builder.setAtmVirtualAccount(response.atmVirtualAccount());
        }
        if (response.atmExpireDate() != null) {
            builder.setAtmExpireDate(formatDate(response.atmExpireDate()));
        }
        if (response.payTime() != null) {
            builder.setPayTime(String.valueOf(DateTimeUtils.toEpochMillis(response.payTime())));
        }
        if (response.closedAt() != null) {
            builder.setClosedAt(String.valueOf(DateTimeUtils.toEpochMillis(response.closedAt())));
        }
        if (response.expireAt() != null) {
            builder.setExpireAt(String.valueOf(DateTimeUtils.toEpochMillis(response.expireAt())));
        }
        if (response.createdAt() != null) {
            builder.setCreatedAt(String.valueOf(DateTimeUtils.toEpochMillis(response.createdAt())));
        }
        if (response.updatedAt() != null) {
            builder.setUpdatedAt(String.valueOf(DateTimeUtils.toEpochMillis(response.updatedAt())));
        }
        // §2A.6 重複付款顯示欄位
        builder.setIsDuplicate(response.isDuplicate());
        if (response.duplicateSequence() != null) {
            builder.setDuplicateSequence(String.valueOf(response.duplicateSequence()));
        }
        // 撥款欄位（收斂規範 §6.2 封閉規則：僅 Admin API 暴露；Gateway DTO 層刻意過濾）
        if (response.grantedAt() != null) {
            builder.setGrantedAt(DateTimeUtils.formatIso8601(response.grantedAt()));
        }
        if (response.grantStatus() != null) {
            builder.setGrantStatus(response.grantStatus());
        }
        if (response.grantMessage() != null) {
            builder.setGrantMessage(response.grantMessage());
        }

        return builder.build();
    }

    /**
     * #2501 賣家視角的 PaymentDetail：在買家版基礎上遮蔽買家 PII。
     *
     * <p>賣家看自己店家的訂單只需付款狀態/方式/金額/時間對帳；買家的 ATM 虛擬帳號、buyer_id、
     * provider trade reference 屬買家 PII，即使是自家訂單也不應暴露給賣家。清空這三個欄位後，
     * 其餘欄位（狀態、金額、付款方式、時間、mer_id=賣家自己的商家編號）維持不變。
     */
    private PaymentDetail toSellerPaymentDetail(PaymentDetailResponse response) {
        return toPaymentDetail(response).toBuilder()
                .clearBuyerId()
                .clearAtmVirtualAccount()
                .clearProviderTradeNo()
                .build();
    }

    /**
     * 轉換為 Proto RefundDetail
     */
    private io.github.will413028.hawkr.payment.grpc.RefundDetail toRefundDetail(RefundDetailResponse response) {
        io.github.will413028.hawkr.payment.grpc.RefundDetail.Builder builder = io.github.will413028.hawkr.payment.grpc.RefundDetail.newBuilder()
                .setRefundNo(response.refundNo())
                .setTradeNo(response.tradeNo())
                .setOrderId(response.orderId())
                .setAmount(formatAmount(response.amount()))  // Java long → Proto string
                .setStatus(response.status().value());

        if (response.reason() != null) {
            builder.setReason(response.reason());
        }
        if (response.providerRefundNo() != null) {
            builder.setProviderRefundNo(response.providerRefundNo());
        }
        if (response.providerStatus() != null) {
            builder.setProviderStatus(response.providerStatus());
        }
        if (response.providerMessage() != null) {
            builder.setProviderMessage(response.providerMessage());
        }
        if (response.estimatedRefundDate() != null) {
            builder.setEstimatedRefundDate(response.estimatedRefundDate().toString());
        }
        if (response.bankDate() != null) {
            builder.setBankDate(response.bankDate().toString());
        }
        if (response.completedAt() != null) {
            builder.setCompletedAt(String.valueOf(DateTimeUtils.toEpochMillis(response.completedAt())));
        }
        if (response.createdAt() != null) {
            builder.setCreatedAt(String.valueOf(DateTimeUtils.toEpochMillis(response.createdAt())));
        }

        return builder.build();
    }

    /**
     * 格式化日期
     */
    private String formatDate(LocalDate date) {
        return date.format(DateTimeFormatter.ofPattern("yyyyMMdd"));
    }


    /**
     * 將 gRPC StatusRuntimeException 映射為對應的回應碼
     * 委派至 GrpcServiceHelper
     */
    private int mapStatusCode(StatusRuntimeException e) {
        return GrpcServiceHelper.mapStatusCode(e);
    }

    /**
     * 安全取得異常訊息（避免 null）
     * 委派至 GrpcServiceHelper
     */
    private String getErrorMessage(Exception e) {
        return GrpcServiceHelper.getErrorMessage(e);
    }

    /**
     * 根據支付結果建立回應訊息
     *
     * 幕後即時付（Apple Pay / Google Pay）會同步返回支付結果，
     * 需要根據實際狀態返回適當的訊息。
     */
    private String buildResponseMessage(io.github.will413028.hawkr.payment.application.dto.CreatePaymentResponse response) {
        if (response.status() == PaymentTransactionStatus.FAILED) {
            // 優先使用 PayUni 返回的錯誤訊息
            if (response.errorMessage() != null && !response.errorMessage().isEmpty()) {
                return response.errorMessage();
            }
            return "支付失敗";
        }

        if (response.status() == PaymentTransactionStatus.PAID) {
            return "支付成功";
        }

        // INIT 狀態（信用卡、ATM 等非即時付款）
        return "建立成功";
    }

    // ==================== Admin 異常查詢 API（§29）====================

    @Override
    public void listPaymentTransactionsForAdmin(
            io.github.will413028.hawkr.payment.grpc.ListPaymentTransactionsForAdminRequest request,
            StreamObserver<io.github.will413028.hawkr.payment.grpc.ListPaymentTransactionsForAdminResponse> observer) {
        try {
            GrpcAuthContext.requireAccountType(AccountType.ADMIN);
            var data = adminQueryService.listPayments(request);
            observer.onNext(io.github.will413028.hawkr.payment.grpc.ListPaymentTransactionsForAdminResponse.newBuilder()
                    .setCode(GrpcResponseCode.SUCCESS.code())
                    .setMessage(GrpcResponseCode.SUCCESS.message())
                    .setData(data)
                    .build());
            observer.onCompleted();
        } catch (Exception e) {
            GrpcErrorHandler.handleException(observer,
                    io.github.will413028.hawkr.payment.grpc.ListPaymentTransactionsForAdminResponse.newBuilder(),
                    e, "PaymentGrpcService", "listPaymentTransactionsForAdmin");
        }
    }

    @Override
    public void getPaymentTransactionForAdmin(
            io.github.will413028.hawkr.payment.grpc.GetPaymentTransactionForAdminRequest request,
            StreamObserver<io.github.will413028.hawkr.payment.grpc.GetPaymentTransactionForAdminResponse> observer) {
        try {
            GrpcAuthContext.requireAccountType(AccountType.ADMIN);
            var detailOpt = adminQueryService.getPaymentDetail(request.getTradeNo());
            if (detailOpt.isEmpty()) {
                observer.onNext(io.github.will413028.hawkr.payment.grpc.GetPaymentTransactionForAdminResponse.newBuilder()
                        .setCode(GrpcResponseCode.NOT_FOUND.code())
                        .setMessage("payment transaction not found")
                        .build());
                observer.onCompleted();
                return;
            }
            observer.onNext(io.github.will413028.hawkr.payment.grpc.GetPaymentTransactionForAdminResponse.newBuilder()
                    .setCode(GrpcResponseCode.SUCCESS.code())
                    .setMessage(GrpcResponseCode.SUCCESS.message())
                    .setData(detailOpt.get())
                    .build());
            observer.onCompleted();
        } catch (Exception e) {
            GrpcErrorHandler.handleException(observer,
                    io.github.will413028.hawkr.payment.grpc.GetPaymentTransactionForAdminResponse.newBuilder(),
                    e, "PaymentGrpcService", "getPaymentTransactionForAdmin");
        }
    }

    @Override
    public void listRefundsForAdmin(
            io.github.will413028.hawkr.payment.grpc.ListRefundsForAdminRequest request,
            StreamObserver<io.github.will413028.hawkr.payment.grpc.ListRefundsForAdminResponse> observer) {
        try {
            GrpcAuthContext.requireAccountType(AccountType.ADMIN);
            var data = adminQueryService.listRefunds(request);
            observer.onNext(io.github.will413028.hawkr.payment.grpc.ListRefundsForAdminResponse.newBuilder()
                    .setCode(GrpcResponseCode.SUCCESS.code())
                    .setMessage(GrpcResponseCode.SUCCESS.message())
                    .setData(data)
                    .build());
            observer.onCompleted();
        } catch (Exception e) {
            GrpcErrorHandler.handleException(observer,
                    io.github.will413028.hawkr.payment.grpc.ListRefundsForAdminResponse.newBuilder(),
                    e, "PaymentGrpcService", "listRefundsForAdmin");
        }
    }

    @Override
    public void getRefundForAdmin(
            io.github.will413028.hawkr.payment.grpc.GetRefundForAdminRequest request,
            StreamObserver<io.github.will413028.hawkr.payment.grpc.GetRefundForAdminResponse> observer) {
        try {
            GrpcAuthContext.requireAccountType(AccountType.ADMIN);
            var detailOpt = adminQueryService.getRefundDetail(request.getRefundNo());
            if (detailOpt.isEmpty()) {
                observer.onNext(io.github.will413028.hawkr.payment.grpc.GetRefundForAdminResponse.newBuilder()
                        .setCode(GrpcResponseCode.NOT_FOUND.code())
                        .setMessage("refund not found")
                        .build());
                observer.onCompleted();
                return;
            }
            observer.onNext(io.github.will413028.hawkr.payment.grpc.GetRefundForAdminResponse.newBuilder()
                    .setCode(GrpcResponseCode.SUCCESS.code())
                    .setMessage(GrpcResponseCode.SUCCESS.message())
                    .setData(detailOpt.get())
                    .build());
            observer.onCompleted();
        } catch (Exception e) {
            GrpcErrorHandler.handleException(observer,
                    io.github.will413028.hawkr.payment.grpc.GetRefundForAdminResponse.newBuilder(),
                    e, "PaymentGrpcService", "getRefundForAdmin");
        }
    }

    @Override
    public void listGrantsForAdmin(
            io.github.will413028.hawkr.payment.grpc.ListGrantsForAdminRequest request,
            StreamObserver<io.github.will413028.hawkr.payment.grpc.ListGrantsForAdminResponse> observer) {
        try {
            GrpcAuthContext.requireAccountType(AccountType.ADMIN);
            var data = adminQueryService.listGrants(request);
            observer.onNext(io.github.will413028.hawkr.payment.grpc.ListGrantsForAdminResponse.newBuilder()
                    .setCode(GrpcResponseCode.SUCCESS.code())
                    .setMessage(GrpcResponseCode.SUCCESS.message())
                    .setData(data)
                    .build());
            observer.onCompleted();
        } catch (Exception e) {
            GrpcErrorHandler.handleException(observer,
                    io.github.will413028.hawkr.payment.grpc.ListGrantsForAdminResponse.newBuilder(),
                    e, "PaymentGrpcService", "listGrantsForAdmin");
        }
    }

    /**
     * Apple Pay on Web Merchant Validation：對 Apple validationURL 取 merchantSession。
     * 詳見 05 號設計書 §5.11。
     */
    @Override
    public void validateApplePayMerchant(
            ValidateApplePayMerchantRequest request,
            StreamObserver<ValidateApplePayMerchantResponse> observer) {
        try {
            String merchantSessionJson = applePayMerchantValidator.validate(
                    request.getValidationUrl(),
                    request.getDomainName(),
                    request.getDisplayName());
            observer.onNext(ValidateApplePayMerchantResponse.newBuilder()
                    .setCode(GrpcResponseCode.SUCCESS.code())
                    .setMessage(GrpcResponseCode.SUCCESS.message())
                    .setMerchantSessionJson(merchantSessionJson)
                    .build());
            observer.onCompleted();
        } catch (Exception e) {
            GrpcErrorHandler.handleException(observer,
                    ValidateApplePayMerchantResponse.newBuilder(),
                    e, "PaymentGrpcService", "validateApplePayMerchant");
        }
    }
}
