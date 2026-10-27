package io.github.will413028.hawkr.payment.adapter.webhook.handler;

import io.github.will413028.hawkr.common.obs.log.ObsLogger;
import io.github.will413028.hawkr.common.utils.code.PaymentMethod;
import io.github.will413028.hawkr.common.utils.code.PaymentTransactionStatus;
import io.github.will413028.hawkr.common.utils.code.WebhookProvider;
import io.github.will413028.hawkr.payment.application.service.DuplicatePaymentService;
import io.github.will413028.hawkr.payment.constant.PayUniConstants;
import io.github.will413028.hawkr.payment.domain.PaymentStatusRules;
import io.github.will413028.hawkr.payment.domain.event.PaymentDomainEvent;
import io.github.will413028.hawkr.payment.domain.model.PaymentTransaction;
import io.github.will413028.hawkr.payment.domain.repository.PaymentTransactionRepository;
import io.github.will413028.hawkr.payment.domain.service.PaymentDomainService;
import io.github.will413028.hawkr.payment.infrastructure.mq.PaymentOutboxWriter;
import org.springframework.stereotype.Component;

import io.github.will413028.hawkr.common.utils.DateTimeUtils;

import java.time.LocalDateTime;
import java.util.List;
import java.util.Map;
import java.util.Optional;

/**
 * 付款結果通知處理器
 *
 * 處理 PayUni 付款結果 Webhook，更新交易狀態並產生領域事件
 */
@Component
public class PaymentNotifyHandler implements WebhookNotifyHandler {

    private static final ObsLogger log = ObsLogger.forClass(PaymentNotifyHandler.class);

    private final PaymentTransactionRepository transactionRepository;
    private final PaymentDomainService domainService;
    private final DuplicatePaymentService duplicatePaymentService;
    private final PaymentOutboxWriter outboxWriter;

    public PaymentNotifyHandler(
            PaymentTransactionRepository transactionRepository,
            PaymentDomainService domainService,
            DuplicatePaymentService duplicatePaymentService,
            PaymentOutboxWriter outboxWriter
    ) {
        this.transactionRepository = transactionRepository;
        this.domainService = domainService;
        this.duplicatePaymentService = duplicatePaymentService;
        this.outboxWriter = outboxWriter;
    }

    @Override
    public WebhookProvider webhookType() {
        return WebhookProvider.PAYUNI;
    }

    @Override
    public String extractIdempotencyKey(Map<String, String> decryptedData) {
        String tradeNo = decryptedData.get(PayUniConstants.FIELD_MER_TRADE_NO);
        String providerTradeNo = decryptedData.get(PayUniConstants.FIELD_TRADE_NO);
        if (tradeNo == null || providerTradeNo == null) {
            return null;
        }
        return tradeNo + ":" + providerTradeNo;
    }

    @Override
    public WebhookHandleResult handle(Map<String, String> decryptedData) {
        String tradeNo = decryptedData.get(PayUniConstants.FIELD_MER_TRADE_NO);
        String providerTradeNo = decryptedData.get(PayUniConstants.FIELD_TRADE_NO);

        // §10.5 D7：解析 PayUni 真實時間欄位（UTC+8 → UTC LocalDateTime）
        // 信用卡 webhook：AuthDay (yyyyMMdd) + AuthTime (HHmmss)
        // ATM/愛金卡：PaymentDay (yyyy-MM-dd HH:mm:ss)，且 ATM webhook 通常無時間欄位 → null
        LocalDateTime providerPaidAt = DateTimeUtils.parsePayUniAuthDayTimeToUtc(
                decryptedData.get(PayUniConstants.FIELD_AUTH_DAY),
                decryptedData.get(PayUniConstants.FIELD_AUTH_TIME));
        if (providerPaidAt == null) {
            providerPaidAt = DateTimeUtils.parsePayUniDateTimeToUtc(
                    decryptedData.get(PayUniConstants.FIELD_PAYMENT_DAY));
        }
        LocalDateTime providerCreateAt = DateTimeUtils.parsePayUniDateTimeToUtc(
                decryptedData.get(PayUniConstants.FIELD_CREATE_DAY));

        // §2A.3.6 段 1：依 provider_trade_no 精準查找
        Optional<PaymentTransaction> optTransaction = transactionRepository.findByProviderTradeNo(providerTradeNo);

        // §2A.3.6 段 1.5：依 trade_no（= MerTradeNo，含序號）精準查找（信用卡 UPP 首次 webhook）
        if (optTransaction.isEmpty()) {
            optTransaction = transactionRepository.findCanonicalByTradeNo(tradeNo);
        }

        // §2A.3.6 段 2/3：MerTradeNo 序列化後，需 extractOrderId 從 trade_no 提取 order_id
        if (optTransaction.isEmpty()) {
            String orderId = extractOrderId(tradeNo);
            // 段 2：依 order_id 查活躍 row（兜底）
            optTransaction = transactionRepository.findActiveByOrderId(orderId);

            // 段 3：重複付款偵測
            if (optTransaction.isEmpty()) {
                List<PaymentTransaction> history = transactionRepository.findHistoryByOrderId(orderId);
                boolean hasPaid = history.stream().anyMatch(t -> t.status() == PaymentTransactionStatus.PAID);
                if (hasPaid) {
                    return handleDuplicateDetected(decryptedData, tradeNo, providerTradeNo, providerPaidAt);
                }
                log.warn("PaymentNotifyHandler.handle.notFound",
                        "全段查找皆無 trade_no=" + tradeNo + " order_id=" + orderId + " provider_trade_no=" + providerTradeNo);
                return WebhookHandleResult.failed("交易不存在: " + tradeNo);
            }
        }

        PaymentTransaction transaction = optTransaction.get();

        // §10.5 D7：寫入 PayUni 真實時間軸（無論後續走哪個分支都先寫入，COALESCE 保護既有值）
        if (providerPaidAt != null || providerCreateAt != null) {
            transactionRepository.updateProviderTimes(transaction.id(), providerCreateAt, providerPaidAt);
        }

        // 2. 如果交易已是終態：
        //    (a) Apple/Google Pay → 衝突檢查
        //    (b) 段 1 找到 CLOSED_BY_SWITCH/EXPIRED/CLOSED + webhook 是 PAID → §2A.6 重複付款情境
        //        （使用者切換後又用舊管道完成付款，PayUni C2 約束導致舊交易仍可被付）
        //    (c) 其他終態 → 跳過
        if (PaymentStatusRules.isTerminal(transaction.status())) {
            if (transaction.isDirectPayment()) {
                String terminalProviderStatus = parseTradeStatus(decryptedData.get(PayUniConstants.FIELD_TRADE_STATUS));
                Optional<PaymentTransactionStatus> webhookStatusOpt = domainService.parseProviderStatus(terminalProviderStatus);

                if (webhookStatusOpt.isPresent()
                        && domainService.isConflict(transaction.status(), webhookStatusOpt.get())) {
                    return handleConflict(transaction, terminalProviderStatus,
                            webhookStatusOpt.get(), decryptedData.get(PayUniConstants.FIELD_MESSAGE));
                }
            }

            // §2A.3.6 段 1.b: 找到非 PAID 終態 row + webhook 帶 PAID
            // 此情境是「同一筆 PayUni 交易（同 provider_trade_no）在我方被切換掉後，使用者用舊管道完成付款」
            // → 不是真重複付款（不能 INSERT 新 row 撞 UNIQUE）
            // → reactivate 段 1 row + 觸發 §2A.6 退款流程（因為切換後另一筆已 PAID）
            String webhookStatus = parseTradeStatus(decryptedData.get(PayUniConstants.FIELD_TRADE_STATUS));
            Optional<PaymentTransactionStatus> webhookStatusOpt = domainService.parseProviderStatus(webhookStatus);
            if (webhookStatusOpt.isPresent()
                    && webhookStatusOpt.get() == PaymentTransactionStatus.PAID
                    && transaction.status() != PaymentTransactionStatus.PAID) {
                log.warn("PaymentNotifyHandler.handle.switchBackToPaid",
                        "段 1.b 切換後舊管道完成 trade_no=" + tradeNo
                                + " row_status=" + transaction.status() + " row_id=" + transaction.id());
                return handleSwitchBackToPaid(transaction, providerPaidAt);
            }

            // §2A.3.6 段 1.c: 找到 PAID row + webhook 帶 PAID + provider_trade_no 不同
            // → 真實「同 paymentType 重複付款」場景（同 order 兩筆獨立 PayUni 交易都成功）
            // → row 已是 PAID 但 webhook 帶的是另一筆獨立 TradeNo
            // → 走段 3 真重複付款處理：INSERT 新 row（不同 provider_trade_no，UNIQUE 不擋）
            if (webhookStatusOpt.isPresent()
                    && webhookStatusOpt.get() == PaymentTransactionStatus.PAID
                    && transaction.status() == PaymentTransactionStatus.PAID
                    && transaction.providerTradeNo() != null
                    && !transaction.providerTradeNo().equals(providerTradeNo)) {
                log.warn("PaymentNotifyHandler.handle.duplicatePaid",
                        "段 1.c 真重複付款偵測 trade_no=" + tradeNo
                                + " existing_provider=" + transaction.providerTradeNo()
                                + " new_provider=" + providerTradeNo);
                return handleDuplicateDetected(decryptedData, tradeNo, providerTradeNo, providerPaidAt);
            }

            log.info("PaymentNotifyHandler.handle.terminal", "交易已是終態", "trade_no", tradeNo);
            return WebhookHandleResult.skipped("交易已是終態: " + transaction.status());
        }

        // 3. 解析 PayUni 狀態
        String providerStatus = parseTradeStatus(decryptedData.get(PayUniConstants.FIELD_TRADE_STATUS));
        Optional<PaymentTransactionStatus> statusOpt = domainService.parseProviderStatus(providerStatus);
        if (statusOpt.isEmpty()) {
            log.warn("PaymentNotifyHandler.handle.unknownStatus", "無法解析 PayUni 狀態", "trade_no", tradeNo);
            return WebhookHandleResult.failed("無法解析 PayUni 狀態: " + decryptedData.get(PayUniConstants.FIELD_TRADE_STATUS));
        }

        PaymentTransactionStatus newStatus = statusOpt.get();
        String providerMessage = decryptedData.get(PayUniConstants.FIELD_MESSAGE);

        // 4. 根據狀態更新交易並產生事件
        return switch (newStatus) {
            case PAID -> handlePaid(transaction, providerTradeNo, providerStatus, providerMessage);
            case FAILED -> handleFailed(transaction, providerTradeNo, providerStatus, providerMessage);
            case INIT -> {
                log.info("PaymentNotifyHandler.handle.processing", "狀態仍為處理中", "trade_no", tradeNo);
                yield WebhookHandleResult.skipped("狀態仍為處理中");
            }
            default -> {
                log.warn("PaymentNotifyHandler.handle.unexpected", "非預期狀態: " + newStatus, "trade_no", tradeNo);
                yield WebhookHandleResult.skipped("非預期狀態: " + newStatus);
            }
        };
    }

    /**
     * 處理付款成功。
     * <p>
     * §2A.6 webhook handler atomicity：版本衝突時 reload + retry 一次。仍失敗時
     * 拋 {@link WebhookRetryableException}，由外層 transaction 回滾、consumer 端
     * MqErrorHandler 接管 retry/DLQ（避免「webhook 被誤標 processed 但業務狀態
     * 未收斂」的死局）。
     * <p>
     * idempotent fast path：reload 後若 row 已是 PAID 則直接回 success 不重試。
     */
    private WebhookHandleResult handlePaid(
            PaymentTransaction transaction,
            String providerTradeNo,
            String providerStatus,
            String providerMessage
    ) {
        LocalDateTime payTime = DateTimeUtils.nowUtc();
        int updated = transactionRepository.updateToPaid(
                transaction.tradeNo(), providerTradeNo, providerStatus, providerMessage, payTime, transaction.version()
        );

        // 第一次更新失敗 → reload + retry 一次
        PaymentTransaction effective = transaction;
        if (updated == 0) {
            String tradeNo = transaction.tradeNo();
            log.warn("PaymentNotifyHandler.handle.versionConflict.reload",
                    "版本衝突，reload 後重試 trade_no=" + tradeNo);
            // 用 id 精準 reload 同一 row（§2A.6 後同 trade_no 可能多 row，
            // findByTradeNo 會炸 TooManyResultsException）
            PaymentTransaction reloaded = transactionRepository.findById(transaction.id())
                    .orElseThrow(() -> new WebhookRetryableException(
                            "reload 找不到 row trade_no=" + tradeNo + " id=" + transaction.id()));

            // 已是 PAID → idempotent fast path
            if (reloaded.status() == PaymentTransactionStatus.PAID) {
                log.info("PaymentNotifyHandler.handle.idempotent",
                        "row 已為 PAID，視為冪等成功 trade_no=" + tradeNo);
                return WebhookHandleResult.success("付款成功（冪等）");
            }

            // 仍是 INIT → 用新 version 重試一次
            updated = transactionRepository.updateToPaid(
                    reloaded.tradeNo(), providerTradeNo, providerStatus, providerMessage, payTime, reloaded.version());

            if (updated == 0) {
                log.error("PaymentNotifyHandler.handle.versionConflict.exhausted",
                        "reload 重試後仍失敗 trade_no=" + tradeNo);
                throw new WebhookRetryableException(
                        "version conflict reload retry exhausted trade_no=" + tradeNo);
            }
            effective = reloaded;
        }

        // §5.10.1 收斂點 A：事務內 INSERT outbox（與 updateToPaid / markAsProcessed 原子）
        // + 註冊 afterCommit 即時投遞
        PaymentDomainEvent.PaymentSucceeded event = domainService.createPaymentSucceededEvent(effective, payTime);
        outboxWriter.enqueueAndDispatch(event);
        log.info("PaymentNotifyHandler.handle.paid", "付款成功", "trade_no", transaction.tradeNo());
        return WebhookHandleResult.success("付款成功");
    }

    /**
     * 處理付款失敗
     */
    private WebhookHandleResult handleFailed(
            PaymentTransaction transaction,
            String webhookProviderTradeNo,
            String providerStatus,
            String providerMessage
    ) {
        // §11.4 補強：webhook 帶來的 PayUni TradeNo 也寫回 FAILED row 供對帳
        int updated = transactionRepository.updateToFailed(
                transaction.tradeNo(), providerStatus, providerMessage, webhookProviderTradeNo, transaction.version()
        );

        // 第一次更新失敗（version 衝突）→ reload 判斷後重試 / 冪等 / 拋重投，
        // 禁止靜默回 success 把失敗結果丟掉（否則 payment.failed 不發、order 卡 PAYMENT_PENDING）
        PaymentTransaction effective = transaction;
        if (updated == 0) {
            String tradeNo = transaction.tradeNo();
            PaymentTransaction reloaded = transactionRepository.findById(transaction.id())
                    .orElseThrow(() -> new WebhookRetryableException(
                            "reload 找不到 row trade_no=" + tradeNo + " id=" + transaction.id()));

            if (reloaded.status() != PaymentTransactionStatus.INIT) {
                // 已被其他寫入推進到終態：不覆蓋（含已 PAID 的衝突），冪等 ack 不丟結果
                if (reloaded.status() == PaymentTransactionStatus.FAILED) {
                    log.info("PaymentNotifyHandler.handle.failed.idempotent",
                            "row 已為 FAILED，視為冪等 trade_no=" + tradeNo);
                    return WebhookHandleResult.success("付款失敗（冪等）");
                }
                log.error("PaymentNotifyHandler.handle.failed.stateConflict",
                        "webhook 回失敗但 row 已為終態 " + reloaded.status().name()
                                + "，不覆蓋 trade_no=" + tradeNo);
                return WebhookHandleResult.success("付款失敗結果與現況終態不一致，不覆蓋");
            }

            // 仍是 INIT → 用新 version 重試一次；再失敗則拋出重投（NACK），不吞掉
            updated = transactionRepository.updateToFailed(
                    reloaded.tradeNo(), providerStatus, providerMessage, webhookProviderTradeNo, reloaded.version());
            if (updated == 0) {
                log.error("PaymentNotifyHandler.handle.failed.versionConflict.exhausted",
                        "reload 重試後仍失敗 trade_no=" + tradeNo);
                throw new WebhookRetryableException(
                        "version conflict reload retry exhausted trade_no=" + tradeNo);
            }
            effective = reloaded;
        }

        // §5.10.1 收斂點 A：事務內 INSERT outbox（與 updateToFailed / markAsProcessed 原子）
        PaymentDomainEvent.PaymentFailed event = domainService.createPaymentFailedEvent(effective, providerMessage);
        outboxWriter.enqueueAndDispatch(event);
        log.info("PaymentNotifyHandler.handle.failed", "付款失敗", "trade_no", transaction.tradeNo());
        return WebhookHandleResult.success("付款失敗");
    }

    /**
     * 處理狀態衝突（Apple Pay / Google Pay 同步終態與 Webhook 結果矛盾）
     */
    private WebhookHandleResult handleConflict(
            PaymentTransaction transaction,
            String providerStatus,
            PaymentTransactionStatus webhookStatus,
            String providerMessage
    ) {
        int updated = transactionRepository.updateToConflict(
                transaction.tradeNo(), providerStatus, providerMessage, transaction.version()
        );

        // §2A.6.13 對稱 handlePaid/handleFailed：version 衝突（updated==0）時 reload 判斷後
        // 重試 / 冪等 / 拋重投，禁止靜默回 success 把衝突結果丟掉（否則 payment.conflict 不發、
        // 狀態不收斂，需手動刪 payment_webhooks 才能重投）。
        PaymentTransaction effective = transaction;
        if (updated == 0) {
            String tradeNo = transaction.tradeNo();
            PaymentTransaction reloaded = transactionRepository.findById(transaction.id())
                    .orElseThrow(() -> new WebhookRetryableException(
                            "reload 找不到 row trade_no=" + tradeNo + " id=" + transaction.id()));

            if (reloaded.status() == PaymentTransactionStatus.CONFLICT) {
                // 已被其他寫入標記 CONFLICT：冪等 ack 不重發
                log.info("PaymentNotifyHandler.handle.conflict.idempotent",
                        "row 已為 CONFLICT，視為冪等 trade_no=" + tradeNo);
                return WebhookHandleResult.success("狀態衝突（冪等）");
            }

            if (reloaded.status() == PaymentTransactionStatus.PAID
                    || reloaded.status() == PaymentTransactionStatus.FAILED) {
                // 仍為 updateToConflict 適用狀態（PAID/FAILED）→ 用新 version 重試一次；再失敗拋重投
                updated = transactionRepository.updateToConflict(
                        reloaded.tradeNo(), providerStatus, providerMessage, reloaded.version());
                if (updated == 0) {
                    log.error("PaymentNotifyHandler.handle.conflict.versionConflict.exhausted",
                            "reload 重試後仍失敗 trade_no=" + tradeNo);
                    throw new WebhookRetryableException(
                            "version conflict reload retry exhausted trade_no=" + tradeNo);
                }
                effective = reloaded;
            } else {
                // reload 後 row 已被推進到其他非衝突終態（EXPIRED/CLOSED 等）：不覆蓋，冪等 ack 不丟結果
                log.error("PaymentNotifyHandler.handle.conflict.stateConflict",
                        "webhook 回衝突但 row 已為終態 " + reloaded.status().name()
                                + "，不覆蓋 trade_no=" + tradeNo);
                return WebhookHandleResult.success("狀態衝突結果與現況終態不一致，不覆蓋");
            }
        }

        // §5.10.1 收斂點 A：事務內 INSERT outbox（與 updateToConflict / markAsProcessed 原子）
        PaymentDomainEvent.PaymentConflict event = domainService.createPaymentConflictEvent(
                effective,
                effective.status().label(),
                webhookStatus.label()
        );
        outboxWriter.enqueueAndDispatch(event);
        log.warn("PaymentNotifyHandler.handle.conflict",
                "狀態衝突: DB=" + effective.status() + " Webhook=" + webhookStatus,
                "trade_no", effective.tradeNo());
        return WebhookHandleResult.success(
                "狀態衝突: DB=" + effective.status() + " Webhook=" + webhookStatus);
    }

    /**
     * §2A.3.6 段 1.b：webhook provider_trade_no 找到 row 但 row 是 CLOSED_BY_SWITCH 終態 + webhook PAID。
     * <p>
     * 此情境是「同一筆 PayUni 交易在我方被切換掉後使用者用舊管道完成」，
     * row 與 webhook 是同一筆 PayUni 交易（同 provider_trade_no），不可 INSERT 新 row（撞 UNIQUE）。
     * <p>
     * 處理：呼叫 DuplicatePaymentService.handleSwitchBackToPaid → reactivate row + 翻轉 + 退舊主
     */
    private WebhookHandleResult handleSwitchBackToPaid(PaymentTransaction reactivatedRow,
                                                       LocalDateTime providerPaidAt) {
        try {
            DuplicatePaymentService.DuplicateResult result =
                    duplicatePaymentService.handleSwitchBackToPaid(reactivatedRow, providerPaidAt);
            log.warn("PaymentNotifyHandler.handle.switchBackToPaid.success",
                    "段 1.b 處理完成 trade_no=" + reactivatedRow.tradeNo()
                            + " refund_target_id=" + result.refundTarget().id()
                            + " flipped=" + result.flipped());
            return WebhookHandleResult.success("切換後舊管道完成已 reactivate + 觸發 rescue 退款 trade_no="
                    + reactivatedRow.tradeNo());
        } catch (Exception e) {
            log.error("PaymentNotifyHandler.handle.switchBackToPaid.failed",
                    "段 1.b 處理失敗 trade_no=" + reactivatedRow.tradeNo() + " err=" + e.getMessage());
            return WebhookHandleResult.failed("段 1.b 處理失敗: " + e.getMessage());
        }
    }

    /**
     * §2A.6 段 3：webhook 帶的 trade_no 對應到歷史 PAID row 但沒有對應的活躍/精準 row
     * → 觸發重複付款偵測流程
     */
    private WebhookHandleResult handleDuplicateDetected(
            Map<String, String> decryptedData,
            String tradeNo,
            String providerTradeNo,
            LocalDateTime providerPaidAt) {
        try {
            String webhookPaymentTypeRaw = decryptedData.get(PayUniConstants.FIELD_PAYMENT_TYPE);
            PaymentMethod webhookPaymentType = mapPayUniPaymentTypeCode(webhookPaymentTypeRaw);
            long amount = parseLongOrZero(decryptedData.get("TradeAmt"));
            String merId = decryptedData.get("MerID");
            String providerStatus = parseTradeStatus(decryptedData.get(PayUniConstants.FIELD_TRADE_STATUS));
            String providerMessage = decryptedData.get(PayUniConstants.FIELD_MESSAGE);

            DuplicatePaymentService.DuplicateResult result = duplicatePaymentService.handleDuplicateFromWebhook(
                    tradeNo, providerTradeNo, webhookPaymentType, amount,
                    merId, providerPaidAt, providerStatus, providerMessage);

            log.warn("PaymentNotifyHandler.handle.duplicate",
                    "重複付款偵測 trade_no=" + tradeNo
                            + " new_row_id=" + result.newRow().id()
                            + " refund_target_id=" + result.refundTarget().id()
                            + " flipped=" + result.flipped());
            return WebhookHandleResult.success("重複付款偵測，已發起 rescue 退款 trade_no=" + tradeNo);
        } catch (Exception e) {
            log.error("PaymentNotifyHandler.handle.duplicate.failed",
                    "重複付款處理失敗 trade_no=" + tradeNo + " err=" + e.getMessage());
            return WebhookHandleResult.failed("重複付款處理失敗: " + e.getMessage());
        }
    }

    /**
     * PayUni webhook PaymentType code (1/2/3/4) → 我方 PaymentMethod。
     */
    private PaymentMethod mapPayUniPaymentTypeCode(String code) {
        if (code == null) return null;
        return switch (code.trim()) {
            case "1" -> PaymentMethod.CREDIT_CARD;
            case "2" -> PaymentMethod.ATM;
            case "3", "4" -> PaymentMethod.CREDIT_CARD;  // PayUni 把 ApplePay/GooglePay 歸在信用卡子類
            default -> null;
        };
    }

    private long parseLongOrZero(String s) {
        if (s == null || s.isBlank()) return 0L;
        try { return Long.parseLong(s.trim()); } catch (NumberFormatException e) { return 0L; }
    }

    /**
     * 解析 TradeStatus（直接回傳字串值）
     */
    private String parseTradeStatus(String statusStr) {
        if (statusStr == null || statusStr.isEmpty()) {
            return null;
        }
        return statusStr;
    }

    /**
     * §2A.3.6 從 MerTradeNo（trade_no，含序號）提取 order_id。
     * orderNo 格式固定為 O{9位數字}（10 字元），trade_no = orderNo + 序號（3位以上）。
     */
    private static String extractOrderId(String merTradeNo) {
        if (merTradeNo == null || merTradeNo.length() < 10) {
            throw new IllegalArgumentException("Invalid MerTradeNo: " + merTradeNo);
        }
        return merTradeNo.substring(0, 10);
    }
}
