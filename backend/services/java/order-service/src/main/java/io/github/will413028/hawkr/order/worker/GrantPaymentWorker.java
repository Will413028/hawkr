package io.github.will413028.hawkr.order.worker;

import io.github.will413028.hawkr.common.obs.log.ObsLogger;
import io.github.will413028.hawkr.order.constant.OrderConstants;
import io.github.will413028.hawkr.order.entity.GrantPaymentTaskEntity;
import io.github.will413028.hawkr.order.config.OrderConfig;
import io.github.will413028.hawkr.order.grpc.client.AfterSalesGrpcClient;
import io.github.will413028.hawkr.order.grpc.client.PaymentGrpcClient;
import io.github.will413028.hawkr.order.repository.GrantPaymentTaskRepository;
import io.github.will413028.hawkr.order.repository.OrderRepository;
import io.github.will413028.hawkr.order.mq.OrderEventPublisher;
import io.github.will413028.hawkr.order.mq.dto.PayoutCompletedPayload;
import io.github.will413028.hawkr.payment.grpc.GrantPaymentResponse;
import org.springframework.scheduling.annotation.Scheduled;
import org.springframework.stereotype.Component;

import io.github.will413028.hawkr.common.utils.DateTimeUtils;
import io.micrometer.core.instrument.Counter;
import io.micrometer.core.instrument.MeterRegistry;

import java.time.LocalDateTime;
import java.util.List;

/**
 * 撥款指示 Worker（03 §8.8）
 *
 * 定期掃描 grant_payment_tasks 表（僅 PENDING / FAILED 重試排程；PROVIDER_AUTO 為終態純對帳
 * 記錄不撈取），呼叫 Payment Service GrantPayment RPC，指數退避重試，超過上限轉人工介入。
 * PayUni 接管收斂見 processTask 內註解（03 §3.11.2）。
 *
 * <p><b>搶佔語意（issue #2294 C4）</b>：claim 是 <b>lease on {@code next_retry_time}</b>
 * （{@link #CLAIM_LEASE_SEC} 秒），不是 {@code SET status=PROCESSING} 的永久旗標——
 * 舊版卡在 {@code PROCESSING} 的撥款任務連 admin 的 {@code resetForRetry} 都收不到
 * （那支救濟 SQL 原本只認 FAILED／MANUAL_INTERVENTION），而 order-service prod
 * {@code replicas=2}。本次同時把 {@code PROCESSING} 納入候選述詞與救濟允許集合。零 migration。
 * 詳見 {@code GrantPaymentTaskRepository.claimTask}。
 *
 * <p><b>撥款資格閘門（backend#3262）</b>：claim 之後、打 payment 之前，先向 aftersales 反查
 * 「這張訂單還會不會再生出退款」。售後 case 被駁回(60)／待寄回逾時關案(33)之後買家仍有
 * {@code appeal.deadline-hours}（72h）可申訴，而訂單此時已完成、撥款任務已建立、30 秒內就撥出去
 * ——申訴成立後 Admin 裁決退款，錢已經在賣家那裡且<b>全 repo 無扣回機制</b>。
 *
 * <p>閘門放在 <b>worker 而不是建立點</b>，三個理由：
 * <ul>
 *   <li>判定必須是<b>撥款當下</b>的狀態：建立時算好的資格會被「之後才提交的申訴」推翻</li>
 *   <li>兩條完成路徑（買家確認收貨 / AutoComplete）匯流於此，一處覆蓋兩條</li>
 *   <li>任何把任務叫醒的路徑（admin {@code resetForRetry}、租約到期、pod 重啟）都會重新過閘</li>
 * </ul>
 *
 * <p>hold 時<b>不算失敗</b>：走 {@code deferForPayoutHold} 推遲 {@code next_retry_time}，
 * 不動 {@code retry_count} ⇒ 不會因為等待而被推進 MANUAL_INTERVENTION。
 * 有時間界的 hold 推到 {@code hold_until}（到期自動放行，<b>不會永久卡住賣家款項</b>）；
 * 無時間界的（申訴審理中 / 反查失敗）推 {@code payout-hold-recheck-seconds}。
 */
@Component
public class GrantPaymentWorker {

    private static final ObsLogger log = ObsLogger.forClass(GrantPaymentWorker.class);

    /** 每批最多處理筆數 */
    private static final int BATCH_SIZE = 20;

    /**
     * issue #2294 C4：claim 租約長度（秒）。
     *
     * <p><b>上界</b>：候選述詞（{@code GrantPaymentTaskRepository.findPendingTasksByStatus}）
     * 無時間上界 ⇒ 窗口寬度 <b>∞</b> ⇒ 任何租約都合格；tick 30 秒遠小於窗口。
     *
     * <p><b>下界</b>（《多副本排程搶佔規範》§三第 5 步）：claim 之後有<b>兩個</b>外部呼叫，
     * 兩者的 deadline 都要進算式——
     * <ul>
     *   <li>{@link AfterSalesGrpcClient#getPayoutHold}（backend#3262 撥款資格閘門）：
     *       {@link AfterSalesGrpcClient#PAYOUT_HOLD_DEADLINE_SEC} = <b>5 秒</b></li>
     *   <li>{@link PaymentGrpcClient#grantPayment}（payment 端同步打 PayUni）：
     *       {@link PaymentGrpcClient#PAYUNI_BACKED_DEADLINE_SEC} = <b>100 秒</b></li>
     * </ul>
     * ⇒ 單筆最壞耗時 <b>105 秒</b> < <b>120 秒</b>租約，餘裕 15 秒。
     * ⚠️ 加閘門時<b>必須</b>連帶檢查這條不等式：新增一個沒有 deadline 的呼叫進租約窗口，
     * 會讓下界變成無上界（規範 §三：「無 deadline ⇒ 最壞耗時無上界 ⇒ 下界根本無從建立」），
     * 兩個 pod 就有機會同時撥同一筆款。本不等式由
     * {@code GrantPaymentWorkerPayoutHoldTest.leaseCoversBothExternalCallDeadlines} 機械斷言。
     *
     * <p>⚠️ 加長租約不會拖慢正常重試：{@code markGrantTaskRetryScheduled} 寫的指數退避值與租約
     * <b>寫同一欄</b>且在 claim 之後執行，天然覆蓋。
     */
    static final long CLAIM_LEASE_SEC = 120L;

    private final GrantPaymentTaskRepository grantPaymentTaskRepository;
    private final PaymentGrpcClient paymentGrpcClient;
    private final OrderRepository orderRepository;
    private final OrderEventPublisher orderEventPublisher;
    private final io.github.will413028.hawkr.order.checkout.client.SellerGrpcClient sellerGrpcClient;
    private final AfterSalesGrpcClient afterSalesGrpcClient;
    private final OrderConfig orderConfig;
    /** backend#3262 業務健康 metric：撥款被 hold 的次數（Prometheus 對 Counter 自動補 _total）。 */
    private final MeterRegistry meterRegistry;

    public GrantPaymentWorker(GrantPaymentTaskRepository grantPaymentTaskRepository,
                              PaymentGrpcClient paymentGrpcClient,
                              OrderRepository orderRepository,
                              OrderEventPublisher orderEventPublisher,
                              io.github.will413028.hawkr.order.checkout.client.SellerGrpcClient sellerGrpcClient,
                              AfterSalesGrpcClient afterSalesGrpcClient,
                              OrderConfig orderConfig,
                              MeterRegistry meterRegistry) {
        this.grantPaymentTaskRepository = grantPaymentTaskRepository;
        this.paymentGrpcClient = paymentGrpcClient;
        this.orderRepository = orderRepository;
        this.orderEventPublisher = orderEventPublisher;
        this.sellerGrpcClient = sellerGrpcClient;
        this.afterSalesGrpcClient = afterSalesGrpcClient;
        this.orderConfig = orderConfig;
        this.meterRegistry = meterRegistry;
    }

    @Scheduled(fixedDelay = 30000)
    public void execute() {
        List<GrantPaymentTaskEntity> tasks = grantPaymentTaskRepository.findPendingTasks(BATCH_SIZE);
        if (tasks.isEmpty()) {
            return;
        }

        log.info("GrantPaymentWorker.execute.start",
                "掃描到待處理撥款任務，count=" + tasks.size());

        for (GrantPaymentTaskEntity task : tasks) {
            processTask(task);
        }
    }

    private void processTask(GrantPaymentTaskEntity task) {
        // lease CAS 搶佔（推 next_retry_time；status 一併寫 PROCESSING 供運維觀測）。
        // claim 回 0 ⇒ 另一個 pod 持有租約 ⇒ 一個副作用都不做。
        int claimed = grantPaymentTaskRepository.claimTask(
                task.getTaskId(), DateTimeUtils.nowUtc().plusSeconds(CLAIM_LEASE_SEC));
        if (claimed == 0) {
            return;
        }

        String orderNo = task.getOrderNo();
        Long taskId = task.getTaskId();

        // backend#3262 撥款資格閘門：這張訂單還會不會再生出退款？會 ⇒ 不撥，推遲後重問。
        // 反查失敗一律視為 hold（client 端 fail-closed）——撥錯款追不回來，撥晚了只是等。
        AfterSalesGrpcClient.PayoutHold hold = afterSalesGrpcClient.getPayoutHold(orderNo);
        if (hold.hold()) {
            deferForPayoutHold(task, hold);
            return;
        }

        try {
            GrantPaymentResponse response = paymentGrpcClient.grantPayment(orderNo);

            if (response.getCode() == 0) {
                // PayUni 接管收斂（03 §3.11.2）：payment-service 以正常回應帶
                // grant_outcome=PROVIDER_TOOK_OVER（該交易存在已完成退款，PayUni 主動撥款、拒絕補發）
                // → task 收斂 PROVIDER_AUTO（終態純對帳記錄），不重試、不進 MANUAL_INTERVENTION、不發 payout 通知
                if (OrderConstants.GRANT_OUTCOME_PROVIDER_TOOK_OVER.equals(response.getGrantOutcome())) {
                    String lastError = "grant_outcome=PROVIDER_TOOK_OVER, message=" + response.getMessage();
                    int converged = grantPaymentTaskRepository.markGrantTaskProviderAuto(taskId, lastError);
                    if (converged == 0) {
                        log.info("GrantPaymentWorker.processTask.providerAutoSkip",
                                "收斂 PROVIDER_AUTO 時 task 已被他路徑變更，冪等 skip, orderNo=" + orderNo);
                        return;
                    }
                    log.info("GrantPaymentWorker.processTask.providerAuto",
                            "PayUni 已接管撥款（退款交易），task 收斂 PROVIDER_AUTO，" + lastError,
                            "orderNo", orderNo);
                    return;
                }

                // 撥款成功（PROCESSING → DONE）
                int updated = grantPaymentTaskRepository.markGrantTaskDone(taskId);
                if (updated == 0) {
                    log.info("GrantPaymentWorker.processTask.doneSkip",
                            "標記成功時 task 已被他路徑變更，冪等 skip, orderNo=" + orderNo);
                    return;
                }
                log.info("GrantPaymentWorker.processTask.success",
                        "撥款指示成功", "orderNo", orderNo);

                // #1286：撥款成功發 payout.completed 通知賣家（走既有通知管線 → FCM + web SSE 雙通道）。
                // 通知失敗只 log，不影響撥款 task 已 DONE 的事實（best-effort，notification 冪等靠 reference_id）。
                try {
                    Long storeId = orderRepository.findStoreIdByOrderNo(orderNo);
                    Long notifySellerAccountId = 0L;
                    if (storeId != null) {
                        try {
                            notifySellerAccountId = sellerGrpcClient.resolvePrincipals(java.util.List.of(storeId))
                                    .principalOrZero(storeId, "store_id");
                        } catch (Exception resolveEx) {
                            log.error("GrantPaymentWorker.processTask.resolvePrincipalsForNotifyFailed",
                                    "store_id -> account_id 解析失敗，payout.completed 通知將帶 0，storeId=" + storeId, resolveEx);
                        }
                    }
                    var data = response.getData();
                    orderEventPublisher.publishPayoutCompleted(new PayoutCompletedPayload(
                            orderNo,
                            storeId != null ? String.valueOf(storeId) : "",
                            String.valueOf(notifySellerAccountId),
                            data != null ? data.getAmount() : "",
                            data != null ? data.getTradeNo() : "",
                            data != null ? data.getGrantedAt() : ""));
                } catch (Exception ex) {
                    log.error("GrantPaymentWorker.processTask.payoutNotifyFailed",
                            "撥款成功但 payout.completed 通知發送失敗, orderNo=" + orderNo, ex);
                }
            } else {
                // 業務錯誤（非暫時性，PROCESSING → MANUAL_INTERVENTION）
                String lastError = "code=" + response.getCode() + ", message=" + response.getMessage();
                int updated = grantPaymentTaskRepository.markGrantTaskManualIntervention(taskId, lastError);
                if (updated == 0) {
                    log.info("GrantPaymentWorker.processTask.bizErrorSkip",
                            "標記人工介入時 task 已被他路徑變更，冪等 skip, orderNo=" + orderNo);
                    return;
                }
                log.error("GrantPaymentWorker.processTask.bizError",
                        "撥款指示業務錯誤，轉人工介入，code=" + response.getCode()
                                + ", orderNo=" + orderNo);
            }
        } catch (Exception e) {
            handleRetry(task, e);
        }
    }

    /**
     * 撥款資格未成立 → 推遲重問（backend#3262）。<b>不是失敗</b>，不動 {@code retry_count}。
     *
     * <p>下次可執行時點：
     * <ul>
     *   <li><b>有時間界</b>（申訴窗未關）→ 推到 {@code hold_until}（＝申訴截止 + grace）。
     *       到期後 worker 自動重問，屆時窗已關 ⇒ 正常撥款。<b>這是「不會永久卡住賣家款項」的機制本體。</b></li>
     *   <li><b>無時間界</b>（售後進行中 / 申訴審理中 / 反查失敗）→ 推
     *       {@code hawkr.order.payout-hold-recheck-seconds}。解除由 Admin 裁決或退款管線決定，
     *       沒有可預期時點，只能定期重問。</li>
     * </ul>
     *
     * <p>CAS 回 0 ＝ 租約已易主或任務已被他路徑（admin 重發）改變 ⇒ 冪等 skip，不重複記帳。
     */
    private void deferForPayoutHold(GrantPaymentTaskEntity task, AfterSalesGrpcClient.PayoutHold hold) {
        LocalDateTime now = DateTimeUtils.nowUtc();
        boolean bounded = hold.holdUntil() != null;
        LocalDateTime nextEligibleAt = bounded
                ? hold.holdUntil()
                : now.plusSeconds(orderConfig.getPayoutHoldRecheckSeconds());
        String reason = "payout hold: reason=" + hold.reason()
                + ", until=" + (bounded ? hold.holdUntil() : "unbounded")
                + ", caseNo=" + (hold.caseNo() == null || hold.caseNo().isEmpty() ? "-" : hold.caseNo());

        int deferred = grantPaymentTaskRepository.deferForPayoutHold(
                task.getTaskId(), nextEligibleAt, reason);
        if (deferred == 0) {
            log.info("GrantPaymentWorker.payoutHold.deferSkip",
                    "推遲撥款時 task 已被他路徑變更，冪等 skip, orderNo=" + task.getOrderNo());
            return;
        }

        Counter.builder("hawkr_grant_payout_held")
                .description("撥款因售後/申訴資格未成立而被推遲的次數（backend#3262）")
                .tag("reason", hold.reason().name())
                .register(meterRegistry)
                .increment();

        if (bounded) {
            log.info("GrantPaymentWorker.payoutHold.appealWindow",
                    "申訴窗未關，推遲撥款至 " + nextEligibleAt + "，" + reason,
                    "orderNo", task.getOrderNo());
        } else {
            // 無時間界 ⇒ 需要有人看見：申訴長期未裁決、或 aftersales 反查持續失敗，都會停在這裡
            log.warn("GrantPaymentWorker.payoutHold.unbounded",
                    "撥款被無時間界 hold，推遲至 " + nextEligibleAt + " 重問，" + reason,
                    "orderNo", task.getOrderNo());
        }
    }

    /**
     * 重試處理：指數退避，超過上限轉人工介入。
     */
    private void handleRetry(GrantPaymentTaskEntity task, Exception e) {
        Long taskId = task.getTaskId();
        int retryCount = (task.getRetryCount() != null ? task.getRetryCount() : 0) + 1;
        int maxRetry = task.getMaxRetryCount() != null ? task.getMaxRetryCount() : OrderConstants.DEFAULT_MAX_RETRY_COUNT;

        if (retryCount >= maxRetry) {
            int updated = grantPaymentTaskRepository.markGrantTaskMaxRetryReached(
                    taskId, retryCount, "超過最大重試次數: " + e.getMessage());
            if (updated == 0) {
                log.info("GrantPaymentWorker.handleRetry.maxReachedSkip",
                        "排程人工介入時 task 已被他路徑變更，冪等 skip, orderNo=" + task.getOrderNo());
                return;
            }
            log.error("GrantPaymentWorker.handleRetry.maxReached",
                    "撥款任務超過最大重試次數，轉人工介入，orderNo=" + task.getOrderNo());
        } else {
            // 指數退避：30s, 60s, 120s, 240s...
            long delaySeconds = 30L * (1L << Math.min(retryCount - 1, 10));
            int updated = grantPaymentTaskRepository.markGrantTaskRetryScheduled(
                    taskId, retryCount,
                    DateTimeUtils.nowUtc().plusSeconds(delaySeconds),
                    e.getMessage());
            if (updated == 0) {
                log.info("GrantPaymentWorker.handleRetry.scheduledSkip",
                        "排程重試時 task 已被他路徑變更，冪等 skip, orderNo=" + task.getOrderNo());
                return;
            }
            log.error("GrantPaymentWorker.handleRetry.scheduled",
                    "撥款任務失敗，排程重試，retryCount=" + retryCount
                            + ", nextRetry=" + delaySeconds + "s, orderNo=" + task.getOrderNo());
        }
    }
}
