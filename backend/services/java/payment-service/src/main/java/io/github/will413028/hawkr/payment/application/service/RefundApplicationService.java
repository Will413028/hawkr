package io.github.will413028.hawkr.payment.application.service;

import io.github.will413028.hawkr.common.obs.log.ObsLogger;
import io.github.will413028.hawkr.common.utils.code.GrpcResponseCode;
import io.github.will413028.hawkr.common.utils.code.RefundStatus;
import io.github.will413028.hawkr.payment.exception.PaymentBusinessException;
import io.github.will413028.hawkr.payment.application.dto.CreateRefundRequest;
import io.github.will413028.hawkr.payment.application.dto.RefundDetailResponse;
import io.github.will413028.hawkr.payment.constant.PayUniConstants;
import io.github.will413028.hawkr.payment.domain.event.PaymentDomainEvent;
import io.github.will413028.hawkr.payment.domain.model.PaymentTransaction;
import io.github.will413028.hawkr.payment.domain.model.RefundTransaction;
import io.github.will413028.hawkr.payment.domain.repository.PaymentTransactionRepository;
import io.github.will413028.hawkr.payment.domain.repository.RefundTransactionRepository;
import io.github.will413028.hawkr.payment.domain.service.RefundDomainService;
import io.github.will413028.hawkr.payment.infrastructure.mq.PaymentEventPublisher;
import io.github.will413028.hawkr.payuni.http.PayUniApiResponse;
import io.github.will413028.hawkr.payuni.payment.PaymentClient;
import org.springframework.stereotype.Service;
import org.springframework.transaction.annotation.Transactional;

import io.github.will413028.hawkr.common.utils.DateTimeUtils;

import java.time.LocalDate;
import java.time.LocalDateTime;
import java.util.List;
import java.util.Optional;

/**
 * 退款應用服務
 */
@Service
public class RefundApplicationService {

    private static final ObsLogger log = ObsLogger.forClass(RefundApplicationService.class);

    private final PaymentTransactionRepository transactionRepository;
    private final RefundTransactionRepository refundRepository;
    private final PaymentEventPublisher eventPublisher;
    private final RefundDomainService domainService;
    private final PaymentClient paymentClient;

    public RefundApplicationService(
            PaymentTransactionRepository transactionRepository,
            RefundTransactionRepository refundRepository,
            PaymentEventPublisher eventPublisher,
            RefundDomainService domainService,
            PaymentClient paymentClient
    ) {
        this.transactionRepository = transactionRepository;
        this.refundRepository = refundRepository;
        this.eventPublisher = eventPublisher;
        this.domainService = domainService;
        this.paymentClient = paymentClient;
    }

    /**
     * 建立退款申請
     */
    @Transactional
    public RefundDetailResponse createRefund(CreateRefundRequest request) {
        String tradeNo = request.tradeNo();

        // 0. 冪等檢查：若有傳入 idempotencyKey，先查詢是否已存在
        if (request.hasIdempotencyKey()) {
            Optional<RefundTransaction> existing = refundRepository.findByIdempotencyKey(request.idempotencyKey());
            if (existing.isPresent()) {
                RefundTransaction existingRefund = existing.get();
                log.info("RefundApplicationService.create.idempotent",
                        "冪等命中，返回既有退款: idempotencyKey=" + request.idempotencyKey()
                                + ", refundNo=" + existingRefund.refundNo()
                                + ", status=" + existingRefund.status().name());
                return RefundDetailResponse.from(existingRefund);
            }
        }

        // 1. 查詢並鎖定原交易（§5.5.5 L1：canonical 主 row + FOR UPDATE，序列化同交易併發退款建立）。
        //    本方法已 @Transactional，行鎖持有至事務結束；後到的併發 createRefund 阻塞至前一筆
        //    commit 釋鎖後才重讀 L2 已退款累計，避免兩請求同時讀到過期累計而超額退款。
        PaymentTransaction transaction = transactionRepository.findCanonicalByTradeNoForUpdate(tradeNo)
                .orElseThrow(() -> new IllegalArgumentException("交易不存在: " + tradeNo));

        // 1A. §2A.1.4：零元訂單沒有退款概念（沒收錢何來退款）。任何上游若誤打 refund 進來，
        // 一律拒絕；正確路徑是由 aftersales / order-cancel 走訂單退貨/取消流程，不進 payment refund。
        if (transaction.paymentType() == io.github.will413028.hawkr.common.utils.code.PaymentMethod.ZERO_AMOUNT) {
            throw new PaymentBusinessException(GrpcResponseCode.PAYMENT_ZERO_AMOUNT_NOT_REFUNDABLE);
        }

        // 2. 檢查是否可以退款
        if (!domainService.canRequestRefund(transaction)) {
            throw new PaymentBusinessException(GrpcResponseCode.PAYMENT_STATUS_INVALID, transaction.status().label());
        }

        // 2.5 檢查是否有進行中的退款（防止重複申請）
        if (refundRepository.hasProcessingRefund(tradeNo)) {
            throw new PaymentBusinessException(GrpcResponseCode.REFUND_ALREADY_PROCESSING);
        }

        // 3. 檢查退款金額
        long alreadyRefunded = refundRepository.sumRefundedAmountByTradeNo(tradeNo);
        Optional<String> validationError = domainService.validateRefundAmount(
                request.amount(),
                transaction.amount(),
                alreadyRefunded
        );
        if (validationError.isPresent()) {
            throw new IllegalArgumentException(validationError.get());
        }

        // 4. 建立退款記錄（含冪等鍵）
        RefundTransaction refund = RefundTransaction.create(
                transaction.id(),
                tradeNo,
                transaction.orderId(),
                request.amount(),
                request.reason(),
                request.hasIdempotencyKey() ? request.idempotencyKey() : null
        );

        refundRepository.save(refund);
        log.info("RefundApplicationService.create.success", "退款申請建立成功", "refund_no", refund.refundNo());

        // 5. 根據原交易支付方式，呼叫對應的 PayUni 退款 API
        PayUniApiResponse response;
        if (transaction.isVoucherPayment()) {
            // ATM 退款改用 PayUni 頁面版，平台不接觸銀行機敏資訊
            // ATM 退款記錄已建立（REQUESTED 狀態），等待 AfterSales 透過 GetRefundPageUrl 取得頁面版 URL
            log.info("RefundApplicationService.create.atmPending",
                    "ATM 退款記錄已建立，等待頁面版退款", "refund_no", refund.refundNo());
            return refundRepository.findByRefundNo(refund.refundNo())
                    .map(RefundDetailResponse::from)
                    .orElseThrow();
        } else {
            // 信用卡/Apple Pay/Google Pay：先查詢請款狀態（CloseStatus），再決定取消授權或退款
            response = handleCreditCardRefund(transaction, refund, request.amount());
        }

        // 6. 處理退款結果
        if (response.isSuccess()) {
            handleRefundSuccess(refund, response);
        } else {
            handleRefundFailure(refund, response);
        }

        // 7. 重新查詢並返回
        return refundRepository.findByRefundNo(refund.refundNo())
                .map(RefundDetailResponse::from)
                .orElseThrow();
    }

    /**
     * 查詢退款詳情
     */
    public Optional<RefundDetailResponse> getRefundDetail(String refundNo) {
        return refundRepository.findByRefundNo(refundNo)
                .map(RefundDetailResponse::from);
    }

    /**
     * Rescue 退款（L2 補救路徑專用）：
     * 由 PendingPaymentReconcileScheduler 在偵測到「本地 EXPIRED 但 PayUni 已成功」的衝突時呼叫。
     *
     * 與 createRefund 的差異：
     * - **跳過 canRequestRefund 狀態檢查**（呼叫時交易為 EXPIRED，正常路徑會被擋）
     * - **保留 hasProcessingRefund 防雙重退款檢查**
     * - **保留 idempotency 冪等檢查**（idempotencyKey 由呼叫方傳入，固定樣式 "rescue:{tradeNo}"）
     * - 信用卡 / Apple Pay / Google Pay：同步完成退款
     * - ATM：建立 REQUESTED 記錄等待人工頁面版完成
     *
     * 不污染既有 createRefund 流程；不反向修改交易狀態。
     */
    @Transactional
    public RescueRefundResult createRescueRefund(String tradeNo, long amount, String reason, String idempotencyKey) {
        // 0. 冪等檢查
        Optional<RefundTransaction> existing = refundRepository.findByIdempotencyKey(idempotencyKey);
        if (existing.isPresent()) {
            RefundTransaction existingRefund = existing.get();
            log.info("RefundApplicationService.rescue.idempotent",
                    "rescue 冪等命中，返回既有退款: idempotencyKey=" + idempotencyKey
                            + ", refundNo=" + existingRefund.refundNo()
                            + ", status=" + existingRefund.status().name());
            return new RescueRefundResult(
                    existingRefund.refundNo(),
                    isVoucherTransaction(tradeNo),
                    true,
                    existingRefund.status().name()
            );
        }

        // 1. 取原交易（注意：此處的 transaction 必為 EXPIRED；§2A.6 後走 canonical）
        PaymentTransaction transaction = transactionRepository.findCanonicalByTradeNo(tradeNo)
                .orElseThrow(() -> new IllegalArgumentException("rescue 失敗，交易不存在: " + tradeNo));

        // 2. 防雙重退款
        if (refundRepository.hasProcessingRefund(tradeNo)) {
            throw new PaymentBusinessException(GrpcResponseCode.REFUND_ALREADY_PROCESSING);
        }

        // 2.5 金額上限檢查（縱深防禦，與 createRefund L2 對齊）：
        //     rescue 本筆此時尚未 save → 用 sumRefundedAmountByTradeNo（含在途 '0','1','2'，不排除本筆），
        //     與 createRefund L2 同一累計語意。應用層更早拒絕 + 明確錯誤，不必等 #1328 的 L3 DB trigger RAISE。
        long alreadyRefunded = refundRepository.sumRefundedAmountByTradeNo(tradeNo);
        Optional<String> validationError = domainService.validateRefundAmount(
                amount,
                transaction.amount(),
                alreadyRefunded
        );
        if (validationError.isPresent()) {
            throw new IllegalArgumentException(validationError.get());
        }

        // 3. 建立退款記錄（含 idempotency key）
        RefundTransaction refund = RefundTransaction.create(
                transaction.id(),
                tradeNo,
                transaction.orderId(),
                amount,
                reason,
                idempotencyKey
        );
        refundRepository.save(refund);
        log.info("RefundApplicationService.rescue.create",
                "rescue 退款記錄建立 refund_no=" + refund.refundNo() + " trade_no=" + tradeNo);

        // 4. 依支付方式分流
        if (transaction.isVoucherPayment()) {
            // ATM：無法全自動，記錄停在 REQUESTED 等待人工頁面版完成
            log.warn("RefundApplicationService.rescue.atmPending",
                    "ATM rescue 退款記錄已建立，需人工頁面版完成", "refund_no", refund.refundNo());
            return new RescueRefundResult(refund.refundNo(), true, false, RefundStatus.REQUESTED.value());
        }

        // 5. 信用卡 / Apple Pay / Google Pay：同步完成
        PayUniApiResponse response;
        try {
            response = handleCreditCardRefund(transaction, refund, amount);
        } catch (Exception e) {
            log.error("RefundApplicationService.rescue.creditCard.failed",
                    "rescue 信用卡退款執行失敗: " + e.getMessage(), e);
            // PayUni 呼叫未真正完成（拋例外）→ 沒有真實 PayUni provider_status 可寫，
            // 只有我方技術錯誤碼，必須塞 provider_message（VARCHAR(500)）而非 provider_status（VARCHAR(4) 只存 PayUni 單字元狀態碼）
            refundRepository.updateStatusToFailed(
                    refund.refundNo(),
                    null,
                    "RESCUE_EXEC_FAIL: " + e.getMessage(),
                    refund.version()
            );
            return new RescueRefundResult(refund.refundNo(), false, false, RefundStatus.FAILED.value());
        }

        if (response.isSuccess()) {
            handleRefundSuccess(refund, response);
            return new RescueRefundResult(refund.refundNo(), false, true, RefundStatus.SUCCEEDED.value());
        } else {
            handleRefundFailure(refund, response);
            return new RescueRefundResult(refund.refundNo(), false, false, RefundStatus.FAILED.value());
        }
    }

    /**
     * 內部用：判斷是否為 ATM/Voucher 類型交易（不重新查 DB，僅供 idempotent 早退路徑使用）
     */
    private boolean isVoucherTransaction(String tradeNo) {
        return transactionRepository.findCanonicalByTradeNo(tradeNo)
                .map(PaymentTransaction::isVoucherPayment)
                .orElse(false);
    }

    /**
     * Rescue 退款結果
     *
     * @param refundNo       退款編號（建立成功必有）
     * @param requiresManual true 表示 ATM 等待人工頁面版完成
     * @param succeeded      true 表示信用卡退款已同步成功（ATM 永遠 false 直到人工完成）
     * @param finalStatus    最終 refund.status 值（REQUESTED / SUCCEEDED / FAILED）
     */
    public record RescueRefundResult(
            String refundNo,
            boolean requiresManual,
            boolean succeeded,
            String finalStatus
    ) {}

    /**
     * 查詢交易的所有退款
     */
    public List<RefundDetailResponse> getRefundsByTradeNo(String tradeNo) {
        return refundRepository.findByTradeNo(tradeNo).stream()
                .map(RefundDetailResponse::from)
                .toList();
    }

    // ==================== 內部方法 ====================

    /**
     * 信用卡退款：先查詢 CloseStatus（請款狀態），再決定取消授權或退款
     *
     * CloseStatus 分流：
     * - APPLYING（請款申請中）→ cancelAuth（/api/trade/cancel，取消授權，更快、無手續費）
     * - SUCCESS（請款成功）  → refund（/api/trade/close CloseType=2，退款）
     * - PROCESSING（請款處理中）→ refund（退款）
     * - CANCELED（請款取消）  → 拋異常
     * - NOT_APPLIED（未申請）  → 拋異常
     */
    private PayUniApiResponse handleCreditCardRefund(PaymentTransaction transaction,
                                                    RefundTransaction refund, long amount) {
        // 查詢交易請款狀態
        PayUniApiResponse queryResp = paymentClient.queryTransactionByProviderTradeNo(
                transaction.merId(), transaction.providerTradeNo());

        if (!queryResp.isSuccess()) {
            throw new PaymentBusinessException(GrpcResponseCode.QUERY_TRANSACTION_FAILED, queryResp.getErrorMessage());
        }

        String closeStatus = queryResp.getCloseStatus();
        if (closeStatus == null || closeStatus.isEmpty()) {
            throw new PaymentBusinessException(GrpcResponseCode.CLOSE_STATUS_UNAVAILABLE);
        }

        log.info("RefundApplicationService.creditCard.closeStatus",
                "查詢請款狀態: CloseStatus=" + closeStatus + ", providerTradeNo=" + transaction.providerTradeNo());

        return switch (closeStatus) {
            case PayUniConstants.CLOSE_STATUS_APPLYING -> {
                // 請款申請中 → 取消授權（/api/trade/cancel）
                log.info("RefundApplicationService.creditCard.cancelAuth",
                        "請款申請中，執行取消授權", "providerTradeNo", transaction.providerTradeNo());
                yield paymentClient.cancelAuthorization(transaction.merId(), transaction.providerTradeNo());
            }
            case PayUniConstants.CLOSE_STATUS_SUCCESS, PayUniConstants.CLOSE_STATUS_PROCESSING -> {
                // 請款成功 / 請款處理中 → 退款（/api/trade/close CloseType=2）
                log.info("RefundApplicationService.creditCard.refund",
                        "請款已完成，執行退款", "providerTradeNo", transaction.providerTradeNo());
                yield paymentClient.refund(transaction.merId(), transaction.providerTradeNo(),
                        refund.refundNo(), amount);
            }
            case PayUniConstants.CLOSE_STATUS_CANCELED ->
                    throw new PaymentBusinessException(GrpcResponseCode.CLOSE_STATUS_CANCELED);
            case PayUniConstants.CLOSE_STATUS_NOT_APPLIED ->
                    throw new PaymentBusinessException(GrpcResponseCode.CLOSE_STATUS_NOT_APPLIED);
            default -> throw new PaymentBusinessException(GrpcResponseCode.CLOSE_STATUS_UNKNOWN,
                    closeStatusLabel(closeStatus));
        };
    }

    private String closeStatusLabel(String closeStatus) {
        return switch (closeStatus) {
            case PayUniConstants.CLOSE_STATUS_APPLYING -> "請款申請中";
            case PayUniConstants.CLOSE_STATUS_SUCCESS -> "請款成功";
            case PayUniConstants.CLOSE_STATUS_CANCELED -> "請款取消";
            case PayUniConstants.CLOSE_STATUS_PROCESSING -> "請款處理中";
            case PayUniConstants.CLOSE_STATUS_NOT_APPLIED -> "未申請";
            default -> "未知狀態";
        };
    }

    /**
     * 處理退款成功（信用卡 sync 路徑）
     */
    private void handleRefundSuccess(RefundTransaction refund, PayUniApiResponse response) {
        LocalDateTime completedAt = DateTimeUtils.nowUtc();

        int updated = refundRepository.updateStatusToSucceeded(
                refund.refundNo(),
                response.getProviderTradeNo(),
                response.getTradeStatus(),
                response.getErrorMessage(),
                null,   // 信用卡 sync：無 bankDate
                completedAt,
                refund.version()
        );

        if (updated > 0) {
            // 信用卡同步成功：無 estimatedRefundDate / bankDate
            PaymentDomainEvent.RefundSucceeded event = domainService.createRefundSucceededEvent(
                    refund, response.getTradeStatus(), response.getErrorMessage(), null, null, completedAt);
            publishEvent(event);
            log.info("RefundApplicationService.refund.succeeded", "退款成功", "refund_no", refund.refundNo());
        }
    }

    /**
     * 處理退款失敗
     */
    private void handleRefundFailure(RefundTransaction refund, PayUniApiResponse response) {
        int updated = refundRepository.updateStatusToFailed(
                refund.refundNo(),
                response.getTradeStatus(),
                response.getErrorMessage(),
                refund.version()
        );

        if (updated > 0) {
            PaymentDomainEvent.RefundFailed event = domainService.createRefundFailedEvent(
                    refund, response.getTradeStatus(), response.getErrorMessage());
            publishEvent(event);
            log.warn("RefundApplicationService.refund.failed", "退款失敗: " + response.getErrorMessage(), "refund_no", refund.refundNo());
        }
    }

    // ==================== ATM 退款 webhook 處理（收斂規範 §5.5 / §9.5）====================

    /**
     * 處理 ATM 退款 async webhook（由 PayUniRefundNotifyConsumer 呼叫）。
     * <p>
     * webhook 帶回的 inner RefundStatus（1 成功 / 2 失敗）是該退款是否真正到帳的唯一權威訊號，
     * 任何競態 / 重送都不得使此結果遺失（05 §5.5.4.1）。
     * <ul>
     *   <li>找到 active（REQUESTED/PROCESSING）：RefundStatus=1 → finalizeAtmRefundSucceeded + 發 RefundSucceeded；
     *       RefundStatus=2 → finalizeAtmRefundFailed + 發 RefundFailed</li>
     *   <li>找不到 active（★ 不得靜默當成功）：查最新一筆退款判定終態
     *       —— SUCCEEDED 冪等略過；FAILED 且 RefundStatus=1 為狀態衝突，校正為 SUCCEEDED 並補發；
     *       FAILED 且 RefundStatus=2 冪等；查無 row 為孤兒告警</li>
     * </ul>
     * finalize 用狀態機冪等（WHERE status IN(0,1)，無 version），只要退款尚未終態必套用。
     */
    @Transactional
    public void handleAtmRefundWebhook(String tradeNo, String refundStatus,
                                       String bankDateRaw, String message) {
        Optional<RefundTransaction> active = refundRepository.findActiveByTradeNo(tradeNo);
        if (active.isPresent()) {
            RefundTransaction refund = active.get();
            if ("1".equals(refundStatus)) {
                finalizeSucceededAndPublish(refund, refundStatus, bankDateRaw, message);
            } else if ("2".equals(refundStatus)) {
                finalizeFailedAndPublish(refund, refundStatus, message);
            } else {
                log.warn("RefundApplicationService.atmWebhook.unknownStatus",
                        "未知 RefundStatus 值，已忽略 refundStatus=" + refundStatus
                                + " tradeNo=" + tradeNo);
            }
            return;
        }

        // ★ 找不到 active，不得靜默當成功：查最新一筆退款（不限狀態）判定終態
        Optional<RefundTransaction> latestOpt = refundRepository.findLatestByTradeNo(tradeNo);
        if (latestOpt.isEmpty()) {
            log.error("RefundApplicationService.atmWebhook.orphan",
                    "找不到任何退款 row（孤兒結果，需人工核對）tradeNo=" + tradeNo
                            + " refundStatus=" + refundStatus);
            return;
        }
        RefundTransaction latest = latestOpt.get();

        if (latest.status() == RefundStatus.SUCCEEDED) {
            log.info("RefundApplicationService.atmWebhook.idempotentSucceeded",
                    "退款已 SUCCEEDED（成功已落地，冪等不重發）refund_no=" + latest.refundNo()
                            + " refundStatus=" + refundStatus);
        } else if (latest.status() == RefundStatus.FAILED) {
            if ("1".equals(refundStatus)) {
                // 狀態衝突：我方誤標 FAILED、PayUni 實際已退款 → 以 PayUni 為準校正為 SUCCEEDED。
                // 走專屬 correctFailedToSucceeded（WHERE status='3'）；finalizeAtmRefundSucceeded 的
                // WHERE status IN('0','1') 不含 '3'，對 FAILED row 會 0 rows 靜默 no-op。
                log.error("RefundApplicationService.atmWebhook.conflictCorrect",
                        "狀態衝突：本地 FAILED 但 PayUni 回成功，校正為 SUCCEEDED refund_no="
                                + latest.refundNo());
                correctFailedAndPublish(latest, refundStatus, bankDateRaw, message);
            } else {
                log.info("RefundApplicationService.atmWebhook.idempotentFailed",
                        "退款已 FAILED 且 PayUni 亦失敗（冪等不動作）refund_no=" + latest.refundNo());
            }
        } else {
            // latest 仍為 REQUESTED/PROCESSING 但 findActiveByTradeNo 卻回空：理論上不會發生（同一 row）
            log.error("RefundApplicationService.atmWebhook.unexpectedActive",
                    "最新退款非終態卻找不到 active，需人工核對 refund_no=" + latest.refundNo()
                            + " status=" + latest.status().name());
        }
    }

    /** finalize → SUCCEEDED（狀態機冪等）+ 補發 RefundSucceeded event。 */
    private void finalizeSucceededAndPublish(RefundTransaction refund, String refundStatus,
                                             String bankDateRaw, String message) {
        LocalDate bankDate = parsePayUniDateOrNull(bankDateRaw);
        LocalDateTime completedAt = DateTimeUtils.nowUtc();
        int updated = refundRepository.finalizeAtmRefundSucceeded(
                refund.refundNo(),
                null,          // provider_refund_no 不變更
                refundStatus,  // 單一欄位雙階段寫入（收斂規範 §4.4）：async → inner RefundStatus
                message,
                bankDate,
                completedAt
        );
        if (updated > 0) {
            PaymentDomainEvent.RefundSucceeded event = domainService.createRefundSucceededEvent(
                    refund, refundStatus, message, null, bankDate, completedAt);
            publishEvent(event);
            log.info("RefundApplicationService.atmWebhook.succeeded",
                    "ATM 退款成功", "refund_no", refund.refundNo());
        }
    }

    /**
     * 狀態衝突校正 → 從 FAILED 校正為 SUCCEEDED（WHERE status='3' 狀態機冪等）+ 補發 RefundSucceeded event。
     * 僅在 correctFailedToSucceeded 真正校正到 row（updated>0）時才補發；重入（status 已 '2'）為 0 rows 不重發。
     */
    private void correctFailedAndPublish(RefundTransaction refund, String refundStatus,
                                         String bankDateRaw, String message) {
        LocalDate bankDate = parsePayUniDateOrNull(bankDateRaw);
        LocalDateTime completedAt = DateTimeUtils.nowUtc();
        int updated = refundRepository.correctFailedToSucceeded(
                refund.refundNo(),
                null,          // provider_refund_no 不變更
                refundStatus,  // 單一欄位雙階段寫入（收斂規範 §4.4）：async → inner RefundStatus
                message,
                bankDate,
                completedAt
        );
        if (updated > 0) {
            PaymentDomainEvent.RefundSucceeded event = domainService.createRefundSucceededEvent(
                    refund, refundStatus, message, null, bankDate, completedAt);
            publishEvent(event);
            log.info("RefundApplicationService.atmWebhook.conflictCorrected",
                    "ATM 退款狀態衝突已校正為 SUCCEEDED 並補發", "refund_no", refund.refundNo());
        }
    }

    /** finalize → FAILED（狀態機冪等）+ 發 RefundFailed event。 */
    private void finalizeFailedAndPublish(RefundTransaction refund, String refundStatus, String message) {
        int updated = refundRepository.finalizeAtmRefundFailed(
                refund.refundNo(),
                refundStatus,
                message
        );
        if (updated > 0) {
            PaymentDomainEvent.RefundFailed event = domainService.createRefundFailedEvent(
                    refund, refundStatus, message);
            publishEvent(event);
            log.warn("RefundApplicationService.atmWebhook.failed",
                    "ATM 退款失敗: " + message, "refund_no", refund.refundNo());
        }
    }

    /**
     * ATM async 受理通知（§6.6 async 副本 webhook）。
     * 由 {@code PayUniRefundAcceptedConsumer} 消費 {@code payuni.refund.accepted} routing key 的訊息後呼叫。
     *
     * <p>用 {@code tradeNo} lookup active refund（findActiveByTradeNo）→ 取 refund_no →
     * 呼叫 {@link #ackAtmRefundAccepted} 冪等推進 REQUESTED → PROCESSING + 寫 estimated_refund_date。</p>
     *
     * <p>與 sync return 路徑（{@code AfterSales.handleRefundPageReturn} 走 {@link #ackAtmRefundAccepted}）
     * 彼此冪等：markAtmRefundAccepted 的 WHERE 有 {@code status = '0'} 條件，先到的寫、後到的 noop。</p>
     */
    @Transactional
    public void handleAtmRefundAccepted(String tradeNo, String estimatedRefundDateRaw, String message) {
        Optional<RefundTransaction> opt = refundRepository.findActiveByTradeNo(tradeNo);
        if (opt.isEmpty()) {
            // ★ 回空不得靜默當成功（05 §5.5.4.3）。受理是非終態里程碑，回空的終態情境是正確 no-op，
            // 但需以 findLatest 判定並修正 log 語意，補齊與結果 Notify（§5.5.4.1）對稱的孤兒告警。
            // 皆不 throw：受理里程碑不可重試，requeue 只會 infinite-loop。
            resolveAtmAcceptedNoActive(tradeNo);
            return;
        }
        RefundTransaction refund = opt.get();
        LocalDate estimated = parsePayUniDateOrNull(estimatedRefundDateRaw);
        int updated = refundRepository.markAtmRefundAccepted(
                refund.refundNo(), "SUCCESS", message, estimated);
        if (updated == 0) {
            log.info("RefundApplicationService.atmAccepted.noop",
                    "refund 非 REQUESTED 狀態（sync 已先寫）或不存在，冪等忽略 refund_no=" + refund.refundNo());
            return;
        }
        log.info("RefundApplicationService.atmAccepted.success",
                "ATM 退款受理（async），狀態推進為 PROCESSING refund_no=" + refund.refundNo());
    }

    /**
     * ATM 受理 Notify 找不到 active 退款的處置（05 §5.5.4.3）。
     *
     * <p>受理 Notify（§6.6）與結果 Notify（§6.7）是兩條獨立 queue、無跨訊息順序保證；結果 Notify
     * 可能先收斂使退款進入終態，之後受理 Notify 抵達時 {@code findActiveByTradeNo}（僅 REQUESTED/PROCESSING）
     * 自然回空。此時受理里程碑（PROCESSING + estimated_refund_date）已無意義，冪等略過為正確行為，
     * 僅需修正 log 語意並補齊孤兒告警。「row 未落地」race 不成立（row 於 createRefund 同事務先建、commit
     * 後才發退款頁 URL），故此處不處理該情境，亦不 throw（受理不可重試）。</p>
     */
    private void resolveAtmAcceptedNoActive(String tradeNo) {
        Optional<RefundTransaction> latestOpt = refundRepository.findLatestByTradeNo(tradeNo);
        if (latestOpt.isEmpty()) {
            log.error("RefundApplicationService.atmAccepted.orphan",
                    "受理 Notify 找不到任何退款 row（孤兒受理，需人工核對）tradeNo=" + tradeNo);
            return;
        }
        RefundTransaction latest = latestOpt.get();
        RefundStatus status = latest.status();
        if (status == RefundStatus.SUCCEEDED || status == RefundStatus.FAILED) {
            log.info("RefundApplicationService.atmAccepted.idempotentTerminal",
                    "退款已終態（結果 Notify 已先收斂），受理里程碑冪等略過 refund_no="
                            + latest.refundNo() + " status=" + status.name());
        } else {
            // REQUESTED/PROCESSING 卻找不到 active：理論上不會發生（同一 row 同時滿足兩查詢矛盾）
            log.error("RefundApplicationService.atmAccepted.unexpectedActive",
                    "最新退款非終態卻找不到 active，需人工核對 refund_no="
                            + latest.refundNo() + " status=" + status.name());
        }
    }

    /**
     * ATM sync（頁面版受理）：REQUESTED → PROCESSING + 寫入 estimated_refund_date。
     * 由 AfterSales.handleRefundPageReturn 解密 PayUni URL scheme 後呼叫。
     */
    @Transactional
    public void ackAtmRefundAccepted(String refundNo, String providerStatus,
                                     String providerMessage, String estimatedRefundDateRaw) {
        LocalDate estimated = parsePayUniDateOrNull(estimatedRefundDateRaw);
        int updated = refundRepository.markAtmRefundAccepted(refundNo, providerStatus, providerMessage, estimated);
        if (updated == 0) {
            log.info("RefundApplicationService.atmAccept.noop",
                    "refund 非 REQUESTED 狀態或不存在，冪等忽略 refund_no=" + refundNo);
            return;
        }
        log.info("RefundApplicationService.atmAccept.success",
                "ATM 退款受理，狀態推進為 PROCESSING refund_no=" + refundNo);
    }

    /**
     * 解析 PayUni 日期字串（"YYYY-MM-DD"）。
     * "0000-00-00" / "-" / null / 空字串皆回 null（非合法日期）。
     */
    private static LocalDate parsePayUniDateOrNull(String raw) {
        if (raw == null || raw.isBlank()) return null;
        String trimmed = raw.trim();
        if ("-".equals(trimmed) || "0000-00-00".equals(trimmed)) return null;
        try {
            return LocalDate.parse(trimmed);
        } catch (Exception e) {
            return null;
        }
    }

    /**
     * 發布領域事件到 MQ
     */
    private void publishEvent(PaymentDomainEvent event) {
        eventPublisher.publish(event);
    }
}
