package io.github.will413028.hawkr.aftersales.service;

import io.github.will413028.hawkr.aftersales.entity.RefundTaskEntity;
import io.github.will413028.hawkr.aftersales.mapper.AfterSaleCaseMapper;
import io.github.will413028.hawkr.aftersales.mapper.RefundTaskMapper;
import io.github.will413028.hawkr.aftersales.mapper.SellerCancelRequestMapper;
import io.github.will413028.hawkr.aftersales.mq.AfterSalesEventPublisherImpl;
import io.github.will413028.hawkr.aftersales.grpc.client.PaymentGrpcClient;
import io.github.will413028.hawkr.common.obs.log.ObsLogger;
import io.github.will413028.hawkr.common.utils.DateTimeUtils;
import io.github.will413028.hawkr.common.utils.code.AfterSaleAuditAction;
import io.github.will413028.hawkr.common.utils.code.AfterSaleAuditActor;
import io.github.will413028.hawkr.common.utils.code.AfterSalePlatformDecisionOutcome;
import io.github.will413028.hawkr.common.utils.code.GrpcResponseCode;
import io.github.will413028.hawkr.common.utils.code.RefundStatus;
import io.github.will413028.hawkr.payment.grpc.GetRefundResponse;
import org.springframework.stereotype.Component;

import java.util.HashMap;
import java.util.Map;

/**
 * ATM 退款「與 payment 端對帳」共用邏輯（issue #1288 / 09 §5.4.3.1）。
 *
 * <p>PayUni 已成功退款為單一真實來源。三條 revert 路徑（sync return / {@code refund.failed} /
 * UNKNOWN fallback）在 revert 前，以及 REF02037 分支，都必須先查 payment 端該退款實際狀態，
 * 避免把已成功退款打回買家重填或標失敗造成兩端狀態分裂。對帳於單一寫入路徑內即時完成，
 * 不引入補償 job（見 {@code docs/開發規範/後端服務責任邊界規範.md}）。</p>
 *
 * <p><b>gRPC 查詢與 DB 寫入分離（連線耗盡防護）</b>：{@link #queryPaymentRefundState} 含同步 gRPC
 * getRefund，caller 必須在 <b>tx 外</b>先查得狀態，再把結果帶入 {@link #advanceIfPaymentSucceeded}
 * / {@link #reconcileNoBalance} 於 tx 內做 CAS 寫入。CAS 條件 SQL（WHERE status IN...）已能處理
 * 查詢與寫入之間的 TOCTOU，無需在 tx 內重查。</p>
 */
@Component
public class AtmRefundReconciler {

    private static final ObsLogger log = ObsLogger.forClass(AtmRefundReconciler.class);

    private final PaymentGrpcClient paymentGrpcClient;
    private final RefundTaskMapper refundTaskMapper;
    private final AfterSaleCaseMapper caseMapper;
    private final SellerCancelRequestMapper cancelMapper;
    private final AfterSaleAuditService auditService;
    private final AfterSalesEventPublisherImpl eventPublisher;

    public AtmRefundReconciler(PaymentGrpcClient paymentGrpcClient,
                               RefundTaskMapper refundTaskMapper,
                               AfterSaleCaseMapper caseMapper,
                               SellerCancelRequestMapper cancelMapper,
                               AfterSaleAuditService auditService,
                               AfterSalesEventPublisherImpl eventPublisher) {
        this.paymentGrpcClient = paymentGrpcClient;
        this.refundTaskMapper = refundTaskMapper;
        this.caseMapper = caseMapper;
        this.cancelMapper = cancelMapper;
        this.auditService = auditService;
        this.eventPublisher = eventPublisher;
    }

    /** 對帳結果分類：payment 端該退款實際狀態。 */
    public enum PaymentRefundState {
        /** payment 已 SUCCEEDED（終態成功）。 */
        SUCCEEDED,
        /** payment 已受理待 webhook（PROCESSING）。 */
        PROCESSING,
        /** 無法判定已退（查詢失敗 / 無 refund_no / REQUESTED / FAILED / 未知）。 */
        UNDETERMINED
    }

    /**
     * 缺陷 B 守衛：任何失敗 revert 前先確認 payment 端未終態成功。
     * <p>{@code state} 由 caller <b>於 tx 外</b>先透過 {@link #queryPaymentRefundState} 查得後帶入
     * （避免同步 gRPC 佔用 DB tx 連線）。若 payment 已 SUCCEEDED → 推進 task 往完成
     * （DONE + close case + 發 aftersales.refund.completed），回傳 true 表示 caller 不應再 revert；
     * 否則回傳 false，caller 維持既有 revert 行為。</p>
     */
    public boolean advanceIfPaymentSucceeded(PaymentRefundState state, RefundTaskEntity task,
                                             String payuniStatus, String payuniMessage) {
        if (state == PaymentRefundState.SUCCEEDED) {
            advanceToDone(task, payuniStatus, payuniMessage);
            log.warn("AtmRefundReconciler.revertGuardHit",
                    "payment 端退款已 SUCCEEDED，取消 revert 改推進完成 refundTaskId=" + task.getId());
            return true;
        }
        return false;
    }

    /**
     * 缺陷 C：REF02037（無足夠金額可退款 = 已退過）處置。
     * <p>不 revert、不 attempt++、不提示買家重填。對帳 payment 端狀態後：
     * SUCCEEDED → 推進完成 / PROCESSING → AWAITING_WEBHOOK / 無法判定 → MANUAL_INTERVENTION 打斷迴圈。</p>
     * <p>{@code state} 由 caller <b>於 tx 外</b>先透過 {@link #queryPaymentRefundState} 查得後帶入。</p>
     *
     * @return 對帳判定的 payment 狀態（供 caller 決定 redirect 顯示，等同傳入的 {@code state}）
     */
    public PaymentRefundState reconcileNoBalance(PaymentRefundState state, RefundTaskEntity task,
                                                 String payuniStatus, String payuniMessage) {
        switch (state) {
            case SUCCEEDED -> {
                advanceToDone(task, payuniStatus, payuniMessage);
                log.info("AtmRefundReconciler.noBalance.succeeded",
                        "REF02037 對帳 payment 已 SUCCEEDED，推進完成 refundTaskId=" + task.getId());
            }
            case PROCESSING -> {
                refundTaskMapper.advanceAwaitingPageToAwaitingWebhook(task.getId(), payuniStatus, payuniMessage);
                log.info("AtmRefundReconciler.noBalance.processing",
                        "REF02037 對帳 payment PROCESSING，推進 AWAITING_WEBHOOK refundTaskId=" + task.getId());
            }
            case UNDETERMINED -> {
                int rows = refundTaskMapper.markManualIntervention(task.getId(), payuniStatus, payuniMessage);
                if (rows > 0) {
                    Map<String, Object> detail = new HashMap<>();
                    detail.put("refundTaskId", task.getId());
                    detail.put("payuniStatus", payuniStatus);
                    detail.put("payuniMessage", payuniMessage);
                    detail.put("previousStatus", task.getStatus());
                    auditService.log(task.getCaseNo(), task.getCancelId(), null,
                            AfterSaleAuditActor.SYSTEM.value(),
                            AfterSaleAuditAction.REFUND_MANUAL_INTERVENTION.value(),
                            detail);
                }
                log.error("AtmRefundReconciler.noBalance.manual",
                        "REF02037 對帳無法判定已退，轉 MANUAL_INTERVENTION 打斷迴圈 refundTaskId=" + task.getId());
            }
        }
        return state;
    }

    /**
     * 查 payment 端該退款實際狀態。external_ref_id（= payment refund_no / case_no 冪等鍵）為空、
     * gRPC 非成功、或狀態非 SUCCEEDED/PROCESSING → UNDETERMINED（保守，不臆造已退）。
     */
    public PaymentRefundState queryPaymentRefundState(RefundTaskEntity task) {
        String refundNo = task.getExternalRefId();
        if (refundNo == null || refundNo.isBlank()) {
            log.warn("AtmRefundReconciler.query.noRefundNo",
                    "refund_tasks.external_ref_id 為空，無法對帳 payment refundTaskId=" + task.getId());
            return PaymentRefundState.UNDETERMINED;
        }
        try {
            GetRefundResponse resp = paymentGrpcClient.getRefund(refundNo);
            if (resp.getCode() != GrpcResponseCode.SUCCESS.code() || !resp.hasData()) {
                log.warn("AtmRefundReconciler.query.notFound",
                        "payment getRefund 非成功或無資料 code=" + resp.getCode() + " refundNo=" + refundNo);
                return PaymentRefundState.UNDETERMINED;
            }
            String status = resp.getData().getStatus();
            if (RefundStatus.SUCCEEDED.value().equals(status)) {
                return PaymentRefundState.SUCCEEDED;
            }
            if (RefundStatus.PROCESSING.value().equals(status)) {
                return PaymentRefundState.PROCESSING;
            }
            return PaymentRefundState.UNDETERMINED;
        } catch (Exception e) {
            log.warn("AtmRefundReconciler.query.failed",
                    "payment getRefund 對帳查詢失敗 refundNo=" + refundNo + " err=" + e.getMessage());
            return PaymentRefundState.UNDETERMINED;
        }
    }

    /**
     * 推進 task 往完成：DONE(20) + close case / complete cancel + 發 aftersales.refund.completed。
     * CAS（WHERE status IN 5,15 / 前置終態）rows=0 視為冪等 skip，不重複 audit / event。
     * 對齊 RefundResultConsumer.closeParentAndPublish 的收斂邏輯。
     */
    private void advanceToDone(RefundTaskEntity task, String payuniStatus, String payuniMessage) {
        int rows = refundTaskMapper.advanceAnyAwaitingToDone(task.getId(), payuniStatus, payuniMessage);
        if (rows == 0) {
            log.info("AtmRefundReconciler.advanceToDone.noop",
                    "task 已不在 5/15 狀態，跳過推進 refundTaskId=" + task.getId());
            return;
        }
        String refundNo = task.getExternalRefId();
        if (task.getCaseNo() != null) {
            int updated = caseMapper.closeIfNotClosed(task.getCaseNo(), DateTimeUtils.nowUtc());
            if (updated > 0) {
                auditService.log(task.getCaseNo(), null, null, AfterSaleAuditActor.SYSTEM.value(),
                        AfterSaleAuditAction.REFUND_SUCCEEDED.value(),
                        Map.of("refundNo", nullToEmpty(refundNo), "refundTaskId", task.getId(),
                                "source", "RECONCILE"));

                AfterSalePlatformDecisionOutcome outcome =
                        AfterSalePlatformDecisionOutcome.fromValue(task.getRefundOutcome());
                Long storeId = caseMapper.findStoreIdByCaseNo(task.getCaseNo());
                Long buyerId = caseMapper.findBuyerIdByCaseNo(task.getCaseNo());
                eventPublisher.publishRefundCompleted(
                        task.getCaseNo(), task.getOrderNo(), storeId, buyerId,
                        outcome, task.getRefundAmount(),
                        task.getPaymentMethod(), refundNo);
            }
        } else if (task.getCancelId() != null) {
            int updated = cancelMapper.completeIfNotCompleted(task.getCancelId());
            if (updated > 0) {
                auditService.log(null, task.getCancelId(), null, AfterSaleAuditActor.SYSTEM.value(),
                        AfterSaleAuditAction.CANCEL_REFUND_SUCCEEDED.value(),
                        Map.of("refundNo", nullToEmpty(refundNo), "refundTaskId", task.getId(),
                                "source", "RECONCILE"));

                eventPublisher.publishCancelCompleted(task.getOrderNo(), task.getCancelId());
            }
        }
    }

    private static String nullToEmpty(String s) {
        return s == null ? "" : s;
    }
}
